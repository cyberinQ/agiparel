// Author: Sunil Jain, 2026-10-25 — Deterministic intake, conflict detection, and commit-aware manifests.
package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/onboarding"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// ============================================================================
// FLEXIBLE SCALAR TYPES (Tolerates LLM Type Drift on Unstructured Fallbacks)
// ============================================================================

type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	switch v := raw.(type) {
	case string:
		*f = FlexString(strings.TrimSpace(v))
	case float64:
		*f = FlexString(strconv.FormatInt(int64(v), 10))
	case int:
		*f = FlexString(strconv.Itoa(v))
	case bool:
		*f = FlexString(strconv.FormatBool(v))
	case nil:
		*f = ""
	default:
		*f = FlexString(fmt.Sprintf("%v", v))
	}
	return nil
}

func (f FlexString) String() string { return string(f) }

type FlexInt int

func (f *FlexInt) UnmarshalJSON(b []byte) error {
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	switch v := raw.(type) {
	case float64:
		*f = FlexInt(int(v))
	case int:
		*f = FlexInt(v)
	case string:
		s := strings.TrimSpace(v)
		if iv, err := strconv.Atoi(s); err == nil {
			*f = FlexInt(iv)
		} else if fv, err := strconv.ParseFloat(s, 64); err == nil {
			*f = FlexInt(int(fv))
		}
	}
	return nil
}

func (f FlexInt) Int() int { return int(f) }

type FlexFloat64 float64

func (f *FlexFloat64) UnmarshalJSON(b []byte) error {
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	switch v := raw.(type) {
	case float64:
		*f = FlexFloat64(v)
	case int:
		*f = FlexFloat64(float64(v))
	case string:
		if fv, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			*f = FlexFloat64(fv)
		}
	}
	return nil
}

func (f FlexFloat64) Float64() float64 { return float64(f) }

// ============================================================================
// EXTRACTION DTOs & MANIFEST TRACKING
// ============================================================================

type RawExtractionResult struct {
	FactoryProfile *struct {
		TenantID       FlexString `json:"tenant_id"`
		FactoryName    FlexString `json:"factory_name"`
		UnitCode       FlexString `json:"unit_code"`
		ShiftStartHour FlexInt    `json:"shift_start_hour"`
		BundleSize     FlexInt    `json:"bundle_size"`
	} `json:"factory_profile,omitempty"`
	Lines         []onboarding.LineRow         `json:"lines,omitempty"`
	Machines      []onboarding.MachineRow      `json:"machines,omitempty"`
	Workers       []onboarding.WorkerRow       `json:"workers,omitempty"`
	Styles        []onboarding.StyleRow        `json:"styles,omitempty"`
	StyleParts    []onboarding.StylePartRow    `json:"style_parts,omitempty"`
	OperationsDAG []onboarding.OperationDAGRow `json:"operations_dag,omitempty"`
	StoreRolls    []onboarding.StoreRollRow    `json:"store_rolls,omitempty"`
	WorkOrders    []onboarding.WorkOrderRow    `json:"work_orders,omitempty"`
}

type ProcessedManifest struct {
	Files map[string]FileRecord `json:"files"`
}

type FileRecord struct {
	SHA256      string `json:"sha256"`
	ProcessedAt int64  `json:"processed_at"`
	Mode        string `json:"mode"` // "DETERMINISTIC_PARSER" or "AI_LLM_FALLBACK"
	Status      string `json:"status"`
	DBApplied   bool   `json:"db_applied"`
	Conflicts   int    `json:"conflicts,omitempty"`
}

type OllamaChatRequest struct {
	Model    string              `json:"model"`
	Messages []OllamaChatMessage `json:"messages"`
	Format   string              `json:"format"`
	Stream   bool                `json:"stream"`
	Options  map[string]any      `json:"options"`
}

type OllamaChatMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type OllamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
	Error string `json:"error,omitempty"`
}

const IngestionSystemPrompt = `You are an expert garment factory data extraction engine. Extract manufacturing entities into valid JSON matching keys: factory_profile, lines, machines, workers, styles, style_parts, operations_dag, store_rolls, work_orders.`

// An extraction success is distinct from a database commit; offline artifacts may never report SUCCESS.
type IngestionConflict struct {
	File       string      `json:"file"`
	Entity     string      `json:"entity"`
	Field      string      `json:"field"`
	Existing   interface{} `json:"existing"`
	Incoming   interface{} `json:"incoming"`
	Resolution string      `json:"resolution"`
}

func main() {
	stagingDir := flag.String("staging-dir", "./data/staging/docs", "Path to raw customer documents")
	outputDir := flag.String("output-dir", "./data/staging/output", "Directory to write digitized artifacts")
	ollamaHost := flag.String("ollama-host", "http://localhost:11434", "Base URL of Ollama")
	modelName := flag.String("model", "qwen3-coder:30b", "Ollama model identifier")
	applyDB := flag.Bool("apply-db", true, "Require a successful PostgreSQL commit for SUCCESS status")
	forceReprocess := flag.Bool("force", false, "Rebuild cumulative state from all source files")
	dbURL := flag.String("db-url", "postgres://agiparel_app:app_secure_runtime_pass@localhost:5432/agiparel_dev?sslmode=disable", "PostgreSQL connection string")
	flag.Parse()

	// Fail before touching staging output if the requested database cannot be reached.
	var db *sql.DB
	if *applyDB {
		var err error
		db, err = sql.Open("pgx", *dbURL)
		if err != nil {
			log.Fatalf("[FATAL] Database initialization: %v", err)
		}
		defer db.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		err = db.PingContext(ctx)
		cancel()
		if err != nil {
			log.Fatalf("[FATAL] PostgreSQL unavailable: %v. Fix Compose first or explicitly run -apply-db=false for offline artifacts", err)
		}
	}
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatal(err)
	}
	manifestPath := filepath.Join(*outputDir, ".processed_manifest.json")
	manifest := loadManifest(manifestPath)
	masterWorkbook := loadExistingWorkbook(filepath.Join(*outputDir, "factory_workbook.json"))
	if *forceReprocess {
		manifest = &ProcessedManifest{Files: make(map[string]FileRecord)}
		masterWorkbook = newWorkbook()
	}
	files, err := os.ReadDir(*stagingDir)
	if err != nil {
		log.Fatalf("[FATAL] Staging docs unreadable: %v", err)
	}
	client := &http.Client{Timeout: 180 * time.Second}
	conflicts := []IngestionConflict{}
	conflictSeen := map[string]bool{}
	processed := []string{}
	failureCount := 0
	for _, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}
		path := filepath.Join(*stagingDir, file.Name())
		hash, hashErr := calculateSHA256(path)
		if hashErr != nil {
			log.Printf("[ERROR] Hashing %s: %v", file.Name(), hashErr)
			failureCount++
			continue
		}
		if !*forceReprocess {
			if rec, ok := manifest.Files[file.Name()]; ok && rec.SHA256 == hash && (rec.Status == "SUCCESS" && rec.DBApplied || rec.Status == "ARTIFACT_ONLY" && !*applyDB) {
				log.Printf("[SKIP] Unchanged %s", file.Name())
				processed = append(processed, file.Name())
				continue
			}
		}
		res, mode, extractErr := processFileHybrid(client, *ollamaHost, *modelName, path)
		if extractErr != nil {
			log.Printf("[ERROR] Extracting %s: %v", file.Name(), extractErr)
			manifest.Files[file.Name()] = FileRecord{SHA256: hash, ProcessedAt: time.Now().UnixMilli(), Mode: mode, Status: "FAILED"}
			failureCount++
			continue
		}
		these := mergeIntoWorkbook(masterWorkbook, res)
		for i := range these {
			these[i].File = file.Name()
			key := fmt.Sprintf("%s/%s/%s/%v", file.Name(), these[i].Entity, these[i].Field, these[i].Incoming)
			if !conflictSeen[key] {
				conflicts = append(conflicts, these[i])
				conflictSeen[key] = true
			}
		}
		manifest.Files[file.Name()] = FileRecord{SHA256: hash, ProcessedAt: time.Now().UnixMilli(), Mode: mode, Status: "STAGED", Conflicts: len(these)}
		processed = append(processed, file.Name())
		log.Printf("[PARSED] %s via %s: %d operation(s), %d machine(s), %d worker(s), %d roll(s), %d PO(s), %d conflict(s)", file.Name(), mode, len(res.OperationsDAG), len(res.Machines), len(res.Workers), len(res.StoreRolls), len(res.WorkOrders), len(these))
	}
	if failureCount > 0 {
		// Do not replace the last committed artifacts or success manifest on failed extraction.
		log.Fatalf("[FATAL] %d documents failed extraction; previously committed state retained", failureCount)
	}

	if db != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err = onboarding.ApplyToDatabase(ctx, db, masterWorkbook)
		cancel()
		if err != nil {
			for _, name := range processed {
				rec := manifest.Files[name]
				rec.Status = "DB_FAILED"
				rec.DBApplied = false
				manifest.Files[name] = rec
			}
			log.Fatalf("[FATAL] PostgreSQL transaction rolled back: %v", err)
		}
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
		err = persistDocuments(ctx, db, masterWorkbook.TenantID, manifest, processed, conflicts)
		cancel()
		if err != nil {
			for _, name := range processed {
				rec := manifest.Files[name]
				rec.Status = "DB_FAILED"
				rec.DBApplied = false
				manifest.Files[name] = rec
			}
			log.Fatalf("[FATAL] Document provenance could not be committed: %v", err)
		}
		log.Printf("[DB COMMIT] Successfully synchronized %d source document(s), tenant=%s", len(processed), masterWorkbook.TenantID)
	}
	// Publish derived artifacts only after successful extraction and database synchronization.
	if err := saveArtifacts(*outputDir, masterWorkbook); err != nil {
		log.Fatalf("[FATAL] Saving committed workbook: %v", err)
	}
	if err := writeJSON(filepath.Join(*outputDir, "conflicts.json"), conflicts); err != nil {
		log.Fatalf("[FATAL] Saving committed conflicts: %v", err)
	}
	for _, name := range processed {
		rec := manifest.Files[name]
		if db == nil {
			rec.Status = "ARTIFACT_ONLY"
			rec.DBApplied = false
		} else {
			rec.Status = "SUCCESS"
			rec.DBApplied = true
		}
		manifest.Files[name] = rec
	}
	saveManifest(manifestPath, manifest)
	log.Printf("[COMPLETE] %d files, %d operations, %d machines, %d workers, %d rolls, %d orders, %d conflicts, db_applied=%t", len(processed), len(masterWorkbook.OperationsDAG), len(masterWorkbook.Machines), len(masterWorkbook.Workers), len(masterWorkbook.StoreRolls), len(masterWorkbook.WorkOrders), len(conflicts), db != nil)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ============================================================================
// HYBRID ROUTER: DETERMINISTIC FIRST -> LLM FALLBACK SECOND
// ============================================================================

func processFileHybrid(client *http.Client, host, model, filePath string) (*RawExtractionResult, string, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".csv":
		res, err := parseDeterministicCSV(filePath)
		if err == nil && hasExtractedEntities(res) {
			return res, "DETERMINISTIC_CSV", nil
		}
	case ".xlsx":
		res, err := parseDeterministicXLSX(filePath)
		if err == nil && hasExtractedEntities(res) {
			return res, "DETERMINISTIC_XLSX", nil
		}
	}

	// Fallback to AI/LLM for unstructured PDFs, images, or unrecognized layouts
	res, err := processWithLLM(client, host, model, filePath)
	return res, "AI_LLM_FALLBACK", err
}

func hasExtractedEntities(r *RawExtractionResult) bool {
	if r == nil {
		return false
	}
	return r.FactoryProfile != nil ||
		len(r.Lines) > 0 ||
		len(r.Machines) > 0 ||
		len(r.Workers) > 0 ||
		len(r.Styles) > 0 ||
		len(r.StyleParts) > 0 ||
		len(r.OperationsDAG) > 0 ||
		len(r.StoreRolls) > 0 ||
		len(r.WorkOrders) > 0
}

// ============================================================================
// DETERMINISTIC CSV & MULTI-SHEET XLSX PARSERS (HANDLES ALL STAGING FILES)
// ============================================================================

func parseDeterministicCSV(path string) (*RawExtractionResult, error) {
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader(rawBytes))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	res := &RawExtractionResult{}
	parseTabularRows(rows, res)
	return res, nil
}

func parseDeterministicXLSX(path string) (*RawExtractionResult, error) {
	sheets, err := extractSheetsFromXLSX(path)
	if err != nil {
		return nil, err
	}
	res := &RawExtractionResult{}
	for _, rows := range sheets {
		parseTabularRows(rows, res)
	}
	return res, nil
}

// parseTabularRows inspects headers dynamically so multi-section CSVs (like 02_Buyer_Purchase_Order)
// and multi-tab Excel workbooks (like 01, 03, 05, and agiparel SAMPLE DATA) parse without LLM failure.
// Header-specific parsing: no fabricated workers, machines, operations, rolls or POs.
// In particular, operation bulletins use numeric sequence IDs, NOT OP-* IDs.
func parseTabularRows(rows [][]string, out *RawExtractionResult) {
	attendanceDate := ""
	header := map[string]int{}
	kind := ""
	style := ""
	po := ""
	legacyQty := 0
	colors := map[string]int{}
	sizes := map[string]int{}
	cartons := []map[string]string{}
	for _, raw := range rows {
		row := cleanRow(raw)
		if len(row) == 0 {
			continue
		}
		joined := strings.Join(row, " ")
		up := strings.ToUpper(joined)
		if idx := strings.Index(up, "STYLE CODE:"); idx >= 0 && style == "" {
			fields := strings.Fields(joined[idx+len("STYLE CODE:"):])
			if len(fields) > 0 {
				style = fields[0]
			}
		}
		if strings.Contains(up, "SHIFT DATE:") {
			attendanceDate = firstToken(valueAfterMarker(joined, "SHIFT DATE:"))
		}
		if strings.HasPrefix(strings.ToUpper(row[0]), "STYLE NO.") && style == "" {
			style = valueAfterColon(row[0])
		}
		if strings.Contains(up, "PO NUMBER:") {
			po = firstToken(valueAfterMarker(joined, "PO NUMBER:"))
		}
		if strings.HasPrefix(strings.ToUpper(row[0]), "PO NO") {
			po = valueAfterColon(row[0])
		}
		if strings.HasPrefix(strings.ToUpper(row[0]), "TOTAL ORDER QTY") {
			legacyQty, _ = strconv.Atoi(firstToken(valueAfterColon(row[0])))
		}
		normalized := map[string]int{}
		for i, k := range row {
			normalized[normalizeColumn(k)] = i
		}
		switch {
		case hasColumn(normalized, "SEQ") && hasColumn(normalized, "SAMMINUTES") && hasColumn(normalized, "OPERATIONNAME"):
			header = normalized
			kind = "operations"
			continue
		case hasColumn(normalized, "MACHINEID") && hasColumn(normalized, "CANONICALCLASS"):
			header = normalized
			kind = "machines"
			continue
		case hasColumn(normalized, "WORKERID") && hasColumn(normalized, "ATTENDANCESTATUS"):
			header = normalized
			kind = "workers"
			continue
		case hasColumn(normalized, "ROLLID") && hasColumn(normalized, "MEASUREDLM"):
			header = normalized
			kind = "rolls"
			continue
		case hasColumn(normalized, "PURCHASEORDER") && hasColumn(normalized, "TOTALORDERQTY"):
			header = normalized
			kind = "purchaseorder"
			continue
		case hasColumn(normalized, "PONUMBER") && hasColumn(normalized, "ORDERQTY"):
			header = normalized
			kind = "skus"
			continue
		case hasColumn(normalized, "CARTONRANGE") && hasColumn(normalized, "TOTALCARTONS"):
			header = normalized
			kind = "cartons"
			continue
		}
		// Empty or new commented section terminates the preceding table.
		if strings.HasPrefix(strings.TrimSpace(row[0]), "#") {
			kind = ""
			continue
		}
		get := func(key string) string {
			if i, ok := header[key]; ok && i < len(raw) {
				return strings.TrimSpace(raw[i])
			}
			return ""
		}
		switch kind {
		case "operations":
			seq, e := strconv.Atoi(get("SEQ"))
			if e != nil || seq <= 0 {
				continue
			}
			sam, e := strconv.ParseFloat(get("SAMMINUTES"), 64)
			if e != nil || sam <= 0 {
				continue
			}
			code := style
			if code == "" {
				continue
			} // missing identity must not be replaced by a guessed style
			srcCode, srcName := splitPart(get("SOURCEPART"))
			dstCode, dstName := splitPart(get("TARGETPART"))
			out.OperationsDAG = append(out.OperationsDAG, onboarding.OperationDAGRow{StyleCode: code, OperationID: fmt.Sprintf("%s-OP-%03d", code, seq), OperationName: get("OPERATIONNAME"), SAMMinutes: sam, MachineType: get("MACHINECLASSREQUIRED"), SourcePartCode: srcCode, TargetPartCode: dstCode})
			for _, part := range []struct{ code, name string }{{srcCode, srcName}, {dstCode, dstName}} {
				if part.code != "" {
					out.StyleParts = append(out.StyleParts, onboarding.StylePartRow{StyleCode: code, PartCode: part.code, PartName: defaultStr(part.name, "Part "+part.code), SubBundleOrder: 1})
				}
			}
		case "machines":
			id := get("MACHINEID")
			if !strings.HasPrefix(strings.ToUpper(id), "M-") {
				continue
			}
			capacity, _ := strconv.Atoi(get("RATEDUPH"))
			if capacity <= 0 {
				capacity = 1
			}
			out.Machines = append(out.Machines, onboarding.MachineRow{MachineID: id, LineID: get("LINEID"), StationID: get("STATIONID"), Name: get("MACHINENAMEMODEL"), CanonicalClass: get("CANONICALCLASS"), CapacityUPH: capacity, Status: get("HARDWAREASSETSTATUS")})
		case "workers":
			id := get("WORKERID")
			if !strings.HasPrefix(strings.ToUpper(id), "W-") {
				continue
			}
			rate, _ := strconv.ParseFloat(get("HOURLYRATEINR"), 64)
			out.Workers = append(out.Workers, onboarding.WorkerRow{WorkerID: id, EmployeeCode: get("EMPLOYEECODE"), Name: get("FULLNAME"), PrimarySkill: get("PRIMARYSKILL"), SkillGrade: get("SKILLGRADE"), HourlyRateINR: rate, AttendanceStatus: get("ATTENDANCESTATUS"), AttendanceDate: attendanceDate, AssignedLineID: get("ASSIGNEDLINE"), AssignedStationID: get("ASSIGNEDSTATION")})
		case "rolls":
			id := get("ROLLID")
			if !strings.HasPrefix(strings.ToUpper(id), "ROLL-") {
				continue
			}
			length, _ := strconv.ParseFloat(get("MEASUREDLM"), 64)
			width, _ := strconv.ParseFloat(get("USABLEWIDTHCM"), 64)
			cut, _ := strconv.Atoi(get("RESERVEDCUT"))
			sku := get("FACTORYSKUDESCRIPTION")
			sku = firstToken(sku)
			out.StoreRolls = append(out.StoreRolls, onboarding.StoreRollRow{RollID: id, FactorySKU: sku, LengthM: length, WidthCM: width, ShadeBand: get("SHADEBAND"), BinLocation: get("WAREHOUSEBIN"), Status: get("ALLOCATIONSTATUS"), ReservedOrder: get("RESERVEDPO"), ReservedCutNo: cut})
		case "purchaseorder":
			ref := get("PURCHASEORDER")
			qty, _ := strconv.Atoi(get("TOTALORDERQTY"))
			if ref == "" || qty <= 0 {
				continue
			}
			po = ref
			style = get("STYLECODE")
			out.WorkOrders = append(out.WorkOrders, onboarding.WorkOrderRow{WorkOrderID: "WO-" + po + "-01", PONumber: po, StyleCode: style, LineID: get("FACTORYLINE"), SupervisorID: get("SUPERVISORCODE"), OrderedQty: qty, Status: "PROPOSED"})
			target, _ := strconv.Atoi(get("TARGETDAILYOUTPUT"))
			if target > 0 {
				out.Lines = append(out.Lines, onboarding.LineRow{LineID: get("FACTORYLINE"), DisplayName: get("FACTORYLINE"), TargetDailyCapacity: target})
			}
			out.Styles = append(out.Styles, onboarding.StyleRow{StyleID: "STYLE_" + style, StyleCode: style, Category: get("CATEGORY"), FitProfile: "REGULAR_FIT", TargetSAMMin: 14.85})
		case "skus":
			if get("PONUMBER") == "" {
				continue
			}
			qty, _ := strconv.Atoi(get("ORDERQTY"))
			if qty <= 0 {
				continue
			}
			colors[get("COLORCODE")] += qty
			sizes[get("SIZECODE")] += qty
		case "cartons":
			if !strings.HasPrefix(strings.ToUpper(get("CARTONRANGE")), "CTN") {
				continue
			}
			entry := map[string]string{}
			for k := range header {
				entry[strings.ToLower(k)] = get(k)
			}
			cartons = append(cartons, entry)
		}
	}
	if len(colors) > 0 {
		for i := range out.WorkOrders {
			b, _ := json.Marshal(colors)
			out.WorkOrders[i].ColorJSON = string(b)
			b, _ = json.Marshal(sizes)
			out.WorkOrders[i].SizeJSON = string(b)
		}
	}
	if len(cartons) > 0 && po != "" {
		b, _ := json.Marshal(cartons)
		out.WorkOrders = append(out.WorkOrders, onboarding.WorkOrderRow{WorkOrderID: "WO-" + po + "-01", PONumber: po, PackagingJSON: string(b)})
	}
	if legacyQty > 0 && po != "" && style != "" {
		out.WorkOrders = append(out.WorkOrders, onboarding.WorkOrderRow{WorkOrderID: "WO-" + po + "-01", PONumber: po, StyleCode: style, OrderedQty: legacyQty, Status: "PROPOSED"})
	}
	if style != "" && len(out.OperationsDAG) > 0 && len(out.Styles) == 0 {
		out.Styles = append(out.Styles, onboarding.StyleRow{StyleID: "STYLE_" + style, StyleCode: style, Category: "WOVEN_TROUSER", FitProfile: "REGULAR_FIT", TargetSAMMin: 14.85})
	}
}

var partPattern = regexp.MustCompile(`^\s*([0-9A-Za-z_-]+)\s*(?:\(([^)]+)\))?`)

func splitPart(v string) (string, string) {
	m := partPattern.FindStringSubmatch(v)
	if len(m) < 3 {
		return "", ""
	}
	return m[1], m[2]
}
func valueAfterColon(s string) string {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(s[idx+1:])
}
func valueAfterMarker(s, marker string) string {
	idx := strings.Index(strings.ToUpper(s), strings.ToUpper(marker))
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(s[idx+len(marker):])
}
func firstToken(s string) string {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
func normalizeColumn(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' {
			return r - 32
		}
		if r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}
func hasColumn(m map[string]int, k string) bool { _, ok := m[k]; return ok }

func defaultHeaderForWidth(width int) []string {
	if width >= 8 {
		return []string{"ROLL_ID", "FACTORY_SKU", "LENGTH_M", "WIDTH_CM", "SHADE_BAND", "BIN_LOCATION", "STATUS", "RESERVED_ORDER"}
	}
	return []string{"COL_1", "COL_2", "COL_3", "COL_4", "COL_5", "COL_6", "COL_7", "COL_8"}
}

func extractSheetsFromXLSX(path string) ([][][]string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	var sharedStrings []string
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			var sst struct {
				SI []struct {
					T string `xml:"t"`
					R []struct {
						T string `xml:"t"`
					} `xml:"r"`
				} `xml:"si"`
			}
			_ = xml.NewDecoder(rc).Decode(&sst)
			rc.Close()
			for _, item := range sst.SI {
				if item.T != "" {
					sharedStrings = append(sharedStrings, item.T)
				} else {
					var sb strings.Builder
					for _, run := range item.R {
						sb.WriteString(run.T)
					}
					sharedStrings = append(sharedStrings, sb.String())
				}
			}
			break
		}
	}

	var allSheets [][][]string
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			var data struct {
				Row []struct {
					C []struct {
						T  string `xml:"t,attr"`
						R  string `xml:"r,attr"`
						V  string `xml:"v"`
						IS struct {
							T string `xml:"t"`
						} `xml:"is"`
					} `xml:"c"`
				} `xml:"sheetData>row"`
			}
			_ = xml.NewDecoder(rc).Decode(&data)
			rc.Close()

			var sheetRows [][]string
			for _, row := range data.Row {
				var vals []string
				for _, c := range row.C {
					pos := 0
					for _, ch := range c.R {
						if ch >= 'A' && ch <= 'Z' {
							pos = pos*26 + int(ch-'A') + 1
						} else {
							break
						}
					}
					if pos > 0 {
						for len(vals) < pos-1 {
							vals = append(vals, "")
						}
					}
					val := c.V
					if c.T == "s" {
						if idx, err := strconv.Atoi(c.V); err == nil && idx >= 0 && idx < len(sharedStrings) {
							val = sharedStrings[idx]
						}
					} else if c.T == "inlineStr" {
						val = c.IS.T
					}
					vals = append(vals, strings.TrimSpace(val))
				}
				sheetRows = append(sheetRows, vals)
			}
			allSheets = append(allSheets, sheetRows)
		}
	}
	return allSheets, nil
}

// ============================================================================
// CUMULATIVE WORKBOOK MERGER (IDEMPOTENT UPSERT BY ENTITY KEY)
// ============================================================================

func mergeIntoWorkbook(wb *onboarding.FactoryWorkbook, res *RawExtractionResult) []IngestionConflict {
	conflicts := []IngestionConflict{}
	if res == nil {
		return conflicts
	}
	if res.FactoryProfile != nil {
		if v := res.FactoryProfile.TenantID.String(); v != "" {
			wb.TenantID = v
		}
		if v := res.FactoryProfile.FactoryName.String(); v != "" {
			wb.FactoryName = v
		}
		if v := res.FactoryProfile.UnitCode.String(); v != "" {
			wb.UnitCode = v
		}
	}

	// Upsert Lines
	lineIdx := make(map[string]int)
	for i, l := range wb.Lines {
		lineIdx[l.LineID] = i
	}
	for _, l := range res.Lines {
		if l.LineID == "" {
			continue
		}
		if idx, exists := lineIdx[l.LineID]; exists {
			wb.Lines[idx] = l
		} else {
			lineIdx[l.LineID] = len(wb.Lines)
			wb.Lines = append(wb.Lines, l)
		}
	}

	// Upsert Machines
	machIdx := make(map[string]int)
	for i, m := range wb.Machines {
		machIdx[m.MachineID] = i
	}
	for _, m := range res.Machines {
		if m.MachineID == "" {
			continue
		}
		if idx, exists := machIdx[m.MachineID]; exists {
			wb.Machines[idx] = m
		} else {
			machIdx[m.MachineID] = len(wb.Machines)
			wb.Machines = append(wb.Machines, m)
		}
	}

	// Upsert Workers
	workerIdx := make(map[string]int)
	for i, w := range wb.Workers {
		workerIdx[w.WorkerID] = i
	}
	for _, w := range res.Workers {
		if w.WorkerID == "" {
			continue
		}
		if idx, exists := workerIdx[w.WorkerID]; exists {
			wb.Workers[idx] = w
		} else {
			workerIdx[w.WorkerID] = len(wb.Workers)
			wb.Workers = append(wb.Workers, w)
		}
	}

	// Upsert Styles
	styleIdx := make(map[string]int)
	for i, s := range wb.Styles {
		styleIdx[s.StyleCode] = i
	}
	for _, s := range res.Styles {
		if s.StyleCode == "" {
			continue
		}
		if idx, exists := styleIdx[s.StyleCode]; exists {
			wb.Styles[idx] = s
		} else {
			styleIdx[s.StyleCode] = len(wb.Styles)
			wb.Styles = append(wb.Styles, s)
		}
	}

	// Upsert Style Parts
	partIdx := make(map[string]int)
	for i, p := range wb.StyleParts {
		partIdx[p.StyleCode+":"+p.PartCode] = i
	}
	for _, p := range res.StyleParts {
		key := p.StyleCode + ":" + p.PartCode
		if p.PartCode == "" {
			continue
		}
		if idx, exists := partIdx[key]; exists {
			wb.StyleParts[idx] = p
		} else {
			partIdx[key] = len(wb.StyleParts)
			wb.StyleParts = append(wb.StyleParts, p)
		}
	}

	// Upsert Operations DAG
	dagIdx := make(map[string]int)
	for i, d := range wb.OperationsDAG {
		dagIdx[d.StyleCode+":"+d.OperationID] = i
	}
	for _, d := range res.OperationsDAG {
		key := d.StyleCode + ":" + d.OperationID
		if d.OperationID == "" {
			continue
		}
		if idx, exists := dagIdx[key]; exists {
			wb.OperationsDAG[idx] = d
		} else {
			dagIdx[key] = len(wb.OperationsDAG)
			wb.OperationsDAG = append(wb.OperationsDAG, d)
		}
	}

	// Upsert Store Rolls
	rollIdx := make(map[string]int)
	for i, r := range wb.StoreRolls {
		rollIdx[r.RollID] = i
	}
	for _, r := range res.StoreRolls {
		if r.RollID == "" {
			continue
		}
		if idx, exists := rollIdx[r.RollID]; exists {
			wb.StoreRolls[idx] = r
		} else {
			rollIdx[r.RollID] = len(wb.StoreRolls)
			wb.StoreRolls = append(wb.StoreRolls, r)
		}
	}

	// Field-level cumulative PO merge; conflicting quantities require human review.
	woIdx := make(map[string]int)
	for i, wo := range wb.WorkOrders {
		woIdx[wo.WorkOrderID] = i
	}
	for _, incoming := range res.WorkOrders {
		if incoming.WorkOrderID == "" {
			continue
		}
		if idx, ok := woIdx[incoming.WorkOrderID]; ok {
			old := wb.WorkOrders[idx]
			if incoming.OrderedQty > 0 && old.OrderedQty > 0 && incoming.OrderedQty != old.OrderedQty {
				conflicts = append(conflicts, IngestionConflict{Entity: incoming.WorkOrderID, Field: "ordered_qty", Existing: old.OrderedQty, Incoming: incoming.OrderedQty, Resolution: "REVIEW_REQUIRED: existing quantity preserved"})
			} else if incoming.OrderedQty > 0 {
				old.OrderedQty = incoming.OrderedQty
			}
			if incoming.PONumber != "" {
				old.PONumber = incoming.PONumber
			}
			if incoming.StyleCode != "" {
				old.StyleCode = incoming.StyleCode
			}
			if incoming.LineID != "" {
				old.LineID = incoming.LineID
			}
			if incoming.SupervisorID != "" {
				old.SupervisorID = incoming.SupervisorID
			}
			if incoming.ColorJSON != "" {
				old.ColorJSON = incoming.ColorJSON
			}
			if incoming.SizeJSON != "" {
				old.SizeJSON = incoming.SizeJSON
			}
			if incoming.PackagingJSON != "" {
				old.PackagingJSON = incoming.PackagingJSON
				var cartons []map[string]string
				if json.Unmarshal([]byte(incoming.PackagingJSON), &cartons) == nil && old.OrderedQty > 0 {
					packed := 0
					for _, c := range cartons {
						count, _ := strconv.Atoi(c["totalcartons"])
						per, _ := strconv.Atoi(c["pcspercarton"])
						packed += count * per
					}
					if packed != old.OrderedQty {
						conflicts = append(conflicts, IngestionConflict{Entity: old.WorkOrderID, Field: "packing_piece_total", Existing: old.OrderedQty, Incoming: packed, Resolution: "REVIEW_REQUIRED: packing specification does not conserve PO pieces"})
					}
				}
			}
			if incoming.ShippingJSON != "" {
				old.ShippingJSON = incoming.ShippingJSON
			}
			if incoming.Status != "" && old.Status != "LOCKED" {
				old.Status = incoming.Status
			}
			wb.WorkOrders[idx] = old
		} else {
			woIdx[incoming.WorkOrderID] = len(wb.WorkOrders)
			wb.WorkOrders = append(wb.WorkOrders, incoming)
		}
	}
	return conflicts
}

// ============================================================================
// HELPERS & LLM FALLBACK
// ============================================================================

func cleanRow(r []string) []string {
	var out []string
	nonEmpty := false
	for _, cell := range r {
		trimmed := strings.TrimSpace(cell)
		if trimmed != "" {
			nonEmpty = true
		}
		out = append(out, trimmed)
	}
	if !nonEmpty {
		return nil
	}
	return out
}

func isHeaderRow(row []string) bool {
	knownHeaders := map[string]bool{
		"WORKER_ID": true, "MACHINE_ID": true, "OPERATION_ID": true, "OP_CODE": true,
		"PURCHASE_ORDER": true, "PO_NUMBER": true, "ROLL_ID": true, "CARTON_RANGE": true,
	}
	for _, c := range row {
		norm := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(c), " ", "_"))
		if knownHeaders[norm] {
			return true
		}
	}
	return false
}

func normalizeHeaders(row []string) []string {
	out := make([]string, len(row))
	for i, c := range row {
		out[i] = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(c), " ", "_"))
	}
	return out
}

func mapRowByHeader(headers, row []string) map[string]string {
	m := make(map[string]string, len(headers))
	for i, h := range headers {
		if i < len(row) {
			m[h] = strings.TrimSpace(row[i])
		}
	}
	return m
}

func hasKeys(m map[string]string, keys ...string) bool {
	for _, k := range keys {
		if strings.TrimSpace(m[k]) == "" {
			return false
		}
	}
	return true
}

func defaultStr(val, fallback string) string {
	if strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return fallback
}

func processWithLLM(client *http.Client, host, model, filePath string) (*RawExtractionResult, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	var userMsg OllamaChatMessage
	userMsg.Role = "user"
	selectedModel := model

	if ext == ".png" || ext == ".jpg" || ext == ".jpeg" || ext == ".webp" {
		selectedModel = strings.TrimSpace(os.Getenv("OLLAMA_VISION_MODEL"))
		if selectedModel == "" {
			selectedModel = "qwen3-vl:8b"
		}
		rawBytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		userMsg.Content = "Extract manufacturing entities from this image."
		userMsg.Images = []string{base64.StdEncoding.EncodeToString(rawBytes)}
	} else {
		var documentText string
		if ext == ".pdf" {
			// Poppler is installed in the Docker runtime image. Never send raw PDF
			// bytes to a text-only language model as though they were plain text.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			output, err := exec.CommandContext(ctx, "pdftotext", "-layout", "-enc", "UTF-8", filePath, "-").Output()
			if err != nil {
				return nil, fmt.Errorf("PDF text extraction failed (scanned PDFs require OCR): %w", err)
			}
			documentText = strings.TrimSpace(string(output))
			if documentText == "" {
				return nil, fmt.Errorf("PDF contains no extractable text; scanned PDFs require OCR/vision processing")
			}
		} else if ext == ".xlsx" {
			// An Excel file is a ZIP archive. Passing its raw bytes as text
			// cannot work reliably, even if Ollama has the requested model.
			var err error
			documentText, err = xlsxAsWorksheetText(filePath)
			if err != nil {
				return nil, err
			}
		} else {
			rawBytes, err := os.ReadFile(filePath)
			if err != nil {
				return nil, err
			}
			documentText = string(rawBytes)
		}
		userMsg.Content = fmt.Sprintf("Extract manufacturing entities from the document below. Preserve source values; never invent missing data:\n\n%s", documentText)
	}

	reqPayload := OllamaChatRequest{
		Model: selectedModel,
		Messages: []OllamaChatMessage{
			{Role: "system", Content: IngestionSystemPrompt},
			userMsg,
		},
		Format:  "json",
		Stream:  false,
		Options: map[string]any{"temperature": 0.0},
	}

	body, _ := json.Marshal(reqPayload)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/chat", host), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var chatResp OllamaChatResponse
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if resp.StatusCode == http.StatusNotFound && strings.Contains(strings.ToLower(string(b)), "model") {
			return nil, fmt.Errorf("Ollama model %q is missing on the host; run 'ollama pull %s' on your Mac, or set AGIE_MODEL/OLLAMA_VISION_MODEL to an installed compatible model", selectedModel, selectedModel)
		}
		return nil, fmt.Errorf("Ollama %s model %s returned HTTP %d: %s", host, selectedModel, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("Ollama response could not be decoded: %w", err)
	}
	if chatResp.Error != "" {
		return nil, fmt.Errorf("Ollama rejected model %s: %s", selectedModel, chatResp.Error)
	}
	var res RawExtractionResult
	if err := json.Unmarshal([]byte(chatResp.Message.Content), &res); err != nil {
		return nil, fmt.Errorf("Ollama returned invalid extraction JSON: %w", err)
	}
	if !hasExtractedEntities(&res) {
		return nil, fmt.Errorf("document extraction produced no verifiable manufacturing entities")
	}
	return &res, nil
}

func calculateSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func loadManifest(path string) *ProcessedManifest {
	m := &ProcessedManifest{Files: make(map[string]FileRecord)}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, m)
	}
	return m
}

func saveManifest(path string, m *ProcessedManifest) {
	if data, err := json.MarshalIndent(m, "", "  "); err == nil {
		_ = os.WriteFile(path, data, 0644)
	}
}

func loadExistingWorkbook(path string) *onboarding.FactoryWorkbook {
	wb := newWorkbook()
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, wb)
	}
	return wb
}

func saveArtifacts(outputDir string, wb *onboarding.FactoryWorkbook) error {
	jsonPath := filepath.Join(outputDir, "factory_workbook.json")
	jsonData, err := json.MarshalIndent(wb, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(jsonPath, jsonData, 0644)
}
func newWorkbook() *onboarding.FactoryWorkbook {
	return &onboarding.FactoryWorkbook{TenantID: "TENANT_DEFAULT", FactoryName: "Noble Garments", UnitCode: "OKHLA Unit 002/02", ShiftStartHour: 8, BundleSize: 20}
}

func persistDocuments(ctx context.Context, db *sql.DB, tenant string, manifest *ProcessedManifest, files []string, conflicts []IngestionConflict) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id',$1,true)", tenant); err != nil {
		return err
	}
	for _, name := range files {
		rec, ok := manifest.Files[name]
		if !ok || rec.SHA256 == "" {
			return fmt.Errorf("missing manifest metadata for %s", name)
		}
		identity := sha256.Sum256([]byte(name + "\x00" + rec.SHA256))
		own := []IngestionConflict{}
		for _, c := range conflicts {
			if c.File == name {
				own = append(own, c)
			}
		}
		issues, err := json.Marshal(own)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO documents(id,tenant_id,filename,file_checksum,source_mode,status,conflicts_json)
           VALUES ($1,$2,$3,$4,$5,'PROPOSED',$6::jsonb)
           ON CONFLICT(id) DO UPDATE SET source_mode=EXCLUDED.source_mode,conflicts_json=EXCLUDED.conflicts_json,imported_at=CURRENT_TIMESTAMP`,
			hex.EncodeToString(identity[:]), tenant, name, rec.SHA256, rec.Mode, string(issues))
		if err != nil {
			return fmt.Errorf("document metadata %s: %w", name, err)
		}
	}
	return tx.Commit()
}

// xlsxAsWorksheetText preserves worksheet order, row numbers and cell boundaries
// when an unfamiliar XLSX must be sent to a local language model for interpretation.
// It rejects oversized prompts explicitly instead of silently discarding rows.
func xlsxAsWorksheetText(path string) (string, error) {
	sheets, err := extractSheetsFromXLSX(path)
	if err != nil {
		return "", fmt.Errorf("open XLSX worksheets: %w", err)
	}
	if len(sheets) == 0 {
		return "", fmt.Errorf("XLSX contains no readable worksheets")
	}
	const maxTextBytes = 128 * 1024
	var b strings.Builder
	for si, sheet := range sheets {
		fmt.Fprintf(&b, "\n--- Worksheet %d ---\n", si+1)
		for ri, row := range sheet {
			cells := make([]string, len(row))
			for i, cell := range row {
				cells[i] = strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(cell)
			}
			fmt.Fprintf(&b, "%d\t%s\n", ri+1, strings.Join(cells, "\t"))
			if b.Len() > maxTextBytes {
				return "", fmt.Errorf("XLSX worksheet text exceeds %d bytes; split the workbook into smaller files, then upload each (no data was silently truncated)", maxTextBytes)
			}
		}
	}
	result := strings.TrimSpace(b.String())
	if result == "" {
		return "", fmt.Errorf("XLSX has no extractable cells")
	}
	return result, nil
}
