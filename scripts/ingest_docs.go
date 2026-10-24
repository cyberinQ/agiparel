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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/onboarding"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// ============================================================================
// FLEXIBLE SCALAR TYPES (Tolerates LLM Type Drift)
// ============================================================================

type FlexString string

func (f *FlexString) UnmarshalJSON(b []byte) error {
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	switch v := raw.(type) {
	case string:
		*f = FlexString(v)
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

func (f FlexString) String() string {
	return string(f)
}

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
		if s == "" {
			*f = 0
			return nil
		}
		if iv, err := strconv.Atoi(s); err == nil {
			*f = FlexInt(iv)
		} else if fv, err := strconv.ParseFloat(s, 64); err == nil {
			*f = FlexInt(int(fv))
		}
	case nil:
		*f = 0
	}
	return nil
}

func (f FlexInt) Int() int {
	return int(f)
}

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
		s := strings.TrimSpace(v)
		if s == "" {
			*f = 0.0
			return nil
		}
		if fv, err := strconv.ParseFloat(s, 64); err == nil {
			*f = FlexFloat64(fv)
		}
	case nil:
		*f = 0.0
	}
	return nil
}

func (f FlexFloat64) Float64() float64 {
	return float64(f)
}

// ============================================================================
// FLEXIBLE EXTRACTION DTOS
// ============================================================================

type RawExtractionResult struct {
	FactoryProfile *struct {
		TenantID       FlexString `json:"tenant_id"`
		FactoryName    FlexString `json:"factory_name"`
		UnitCode       FlexString `json:"unit_code"`
		ShiftStartHour FlexInt    `json:"shift_start_hour"`
		BundleSize     FlexInt    `json:"bundle_size"`
	} `json:"factory_profile"`
	Lines []struct {
		LineID              FlexString  `json:"line_id"`
		DisplayName         FlexString  `json:"display_name"`
		StationsCount       FlexInt     `json:"stations_count"`
		TargetDailyCapacity FlexInt     `json:"target_daily_capacity"`
		DwellFloorPct       FlexFloat64 `json:"dwell_floor_pct"`
	} `json:"lines"`
	Machines []struct {
		MachineID      FlexString `json:"machine_id"`
		LineID         FlexString `json:"line_id"`
		StationID      FlexString `json:"station_id"`
		Name           FlexString `json:"name"`
		CanonicalClass FlexString `json:"canonical_class"`
		CapacityUPH    FlexInt    `json:"capacity_uph"`
		Status         FlexString `json:"status"`
	} `json:"machines"`
	Workers []struct {
		WorkerID          FlexString  `json:"worker_id"`
		EmployeeCode      FlexString  `json:"employee_code"`
		Name              FlexString  `json:"name"`
		PrimarySkill      FlexString  `json:"primary_skill"`
		SkillGrade        FlexString  `json:"skill_grade"`
		HourlyRateINR     FlexFloat64 `json:"hourly_rate_inr"`
		AttendanceStatus  FlexString  `json:"attendance_status"`
		AssignedLineID    FlexString  `json:"assigned_line_id"`
		AssignedStationID FlexString  `json:"assigned_station_id"`
	} `json:"workers"`
	Styles []struct {
		StyleID      FlexString  `json:"style_id"`
		StyleCode    FlexString  `json:"style_code"`
		Category     FlexString  `json:"category"`
		FitProfile   FlexString  `json:"fit_profile"`
		TargetSAMMin FlexFloat64 `json:"target_sam_min"`
		FabricSKU    FlexString  `json:"fabric_sku"`
	} `json:"styles"`
	StyleParts []struct {
		StyleCode      FlexString `json:"style_code"`
		PartCode       FlexString `json:"part_code"`
		PartName       FlexString `json:"part_name"`
		SubBundleOrder FlexInt    `json:"sub_bundle_order"`
		MaterialSKU    FlexString `json:"material_sku"`
	} `json:"style_parts"`
	OperationsDAG []struct {
		StyleCode      FlexString  `json:"style_code"`
		OperationID    FlexString  `json:"operation_id"`
		OperationName  FlexString  `json:"operation_name"`
		SAMMinutes     FlexFloat64 `json:"sam_minutes"`
		MachineType    FlexString  `json:"machine_type"`
		SourcePartCode FlexString  `json:"source_part_code"`
		TargetPartCode FlexString  `json:"target_part_code"`
	} `json:"operations_dag"`
	StoreRolls []struct {
		RollID        FlexString  `json:"roll_id"`
		FactorySKU    FlexString  `json:"factory_sku"`
		LengthM       FlexFloat64 `json:"length_m"`
		WidthCM       FlexFloat64 `json:"width_cm"`
		ShadeBand     FlexString  `json:"shade_band"`
		BinLocation   FlexString  `json:"bin_location"`
		Status        FlexString  `json:"status"`
		ReservedOrder FlexString  `json:"reserved_order"`
		ReservedCutNo FlexInt     `json:"reserved_cut_no"`
	} `json:"store_rolls"`
	WorkOrders []struct {
		WorkOrderID  FlexString `json:"work_order_id"`
		PONumber     FlexString `json:"po_number"`
		StyleCode    FlexString `json:"style_code"`
		LineID       FlexString `json:"line_id"`
		SupervisorID FlexString `json:"supervisor_id"`
		OrderedQty   FlexInt    `json:"ordered_qty"`
		ColorJSON    FlexString `json:"color_json"`
		SizeJSON     FlexString `json:"size_json"`
		Status       FlexString `json:"status"`
	} `json:"work_orders"`
}

const IngestionSystemPrompt = `You are an expert apparel factory data extraction engine.
Extract manufacturing ground truth from the document into valid JSON.
IMPORTANT INSTRUCTIONS:
1. All identifiers and names MUST be strings enclosed in quotes (e.g. "M-01", "Line 14", "KK524867").
2. ONLY return keys that actually exist in the source document. Do NOT output empty arrays for missing categories.

Valid JSON Schema Format:
{
  "factory_profile": {"tenant_id":"TENANT_DEFAULT","factory_name":"Noble Garments","unit_code":"Unit 002/02","shift_start_hour":8,"bundle_size":20},
  "lines": [{"line_id":"Line 14","display_name":"Line 14 - Assembly","stations_count":14,"target_daily_capacity":600,"dwell_floor_pct":0.40}],
  "machines": [{"machine_id":"M-14-01","line_id":"Line 14","station_id":"ST-01","name":"Pegasus Overlock","canonical_class":"4-OL_SERGING","capacity_uph":130,"status":"ACTIVE"}],
  "workers": [{"worker_id":"W-01","employee_code":"EMP_01","name":"Ram Kumar","primary_skill":"LINE_SUPERVISOR","skill_grade":"A","hourly_rate_inr":180.0,"attendance_status":"PRESENT","assigned_line_id":"Line 14","assigned_station_id":"SUPERVISOR_DESK"}],
  "styles": [{"style_id":"STYLE_KK524867","style_code":"KK524867","category":"WOVEN_TROUSER","fit_profile":"REGULAR_FIT","target_sam_min":0.60,"fabric_sku":"DENIM-STRETCH-12OZ"}],
  "style_parts": [{"style_code":"KK524867","part_code":"101","part_name":"Left Pocket","sub_bundle_order":1,"material_sku":"POCKETING-TWILL-WHITE"}],
  "operations_dag": [{"style_code":"KK524867","operation_id":"OP-101","operation_name":"Attach Pocket","sam_minutes":0.50,"machine_type":"SNLS_LOCKSTITCH","source_part_code":"101","target_part_code":"102"}],
  "store_rolls": [{"roll_id":"ROLL-007","factory_sku":"DENIM-STRETCH-12OZ","length_m":59.0,"width_cm":148.0,"shade_band":"A","bin_location":"BIN-A01","status":"RESERVED","reserved_order":"MK6574J9","reserved_cut_no":1}],
  "work_orders": [{"work_order_id":"WO-01","po_number":"MK6574J9","style_code":"KK524867","line_id":"Line 14","supervisor_id":"W-01","ordered_qty":600,"color_json":"{}","size_json":"{}","status":"IN_PROGRESS"}]
}`

type ProcessedManifest struct {
	Files map[string]FileRecord `json:"files"`
}

type FileRecord struct {
	SHA256      string `json:"sha256"`
	ProcessedAt int64  `json:"processed_at"`
	Status      string `json:"status"`
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
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"message"`
	Error string `json:"error,omitempty"`
}

func main() {
	stagingDir := flag.String("staging-dir", "./data/staging/docs", "Path to raw customer documents")
	outputDir := flag.String("output-dir", "./data/staging/output", "Directory to write digitized artifacts")
	ollamaHost := flag.String("ollama-host", "http://localhost:11434", "Base URL of Ollama")
	modelName := flag.String("model", "qwen2.5-coder:7b", "Ollama model identifier")
	applyDB := flag.Bool("apply-db", false, "Commit extracted entities into PostgreSQL after each file")
	dbURL := flag.String("db-url", "postgres://agiparel_app:app_secure_runtime_pass@localhost:5432/agiparel_dev?sslmode=disable", "PostgreSQL connection string")
	flag.Parse()

	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Fatal: failed to create output directory: %v", err)
	}

	manifestPath := filepath.Join(*outputDir, ".processed_manifest.json")
	manifest := loadManifest(manifestPath)
	masterWorkbook := loadExistingWorkbook(filepath.Join(*outputDir, "factory_workbook.json"))

	files, err := os.ReadDir(*stagingDir)
	if err != nil {
		log.Fatalf("Fatal: failed to read staging directory: %v", err)
	}

	transport := &http.Transport{
		MaxIdleConns:       10,
		IdleConnTimeout:    60 * time.Second,
		DisableCompression: true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   300 * time.Second,
	}

	var db *sql.DB
	if *applyDB {
		var dbErr error
		db, dbErr = sql.Open("pgx", *dbURL)
		if dbErr != nil {
			log.Fatalf("Fatal: DB connection failed: %v", dbErr)
		}
		defer db.Close()
	}

	for idx, file := range files {
		if file.IsDir() || strings.HasPrefix(file.Name(), ".") {
			continue
		}

		fullPath := filepath.Join(*stagingDir, file.Name())
		hash, err := calculateSHA256(fullPath)
		if err != nil {
			log.Printf("[WARN] Skipping %s: unable to compute hash: %v", file.Name(), err)
			continue
		}

		if rec, exists := manifest.Files[file.Name()]; exists && rec.SHA256 == hash && rec.Status == "SUCCESS" {
			log.Printf("[SKIP %d/%d] %s already processed (SHA: %s)", idx+1, len(files), file.Name(), hash[:8])
			continue
		}

		log.Printf("[INGEST %d/%d] Processing artifact: %s (SHA: %s)", idx+1, len(files), file.Name(), hash[:8])

		rawResult, err := processFile(client, *ollamaHost, *modelName, fullPath)
		if err != nil {
			log.Printf("[ERROR] Extraction failed for %s: %v", file.Name(), err)
			manifest.Files[file.Name()] = FileRecord{
				SHA256:      hash,
				ProcessedAt: time.Now().UTC().UnixMilli(),
				Status:      "FAILED",
			}
			saveManifest(manifestPath, manifest)
			continue
		}

		mergeIntoWorkbook(masterWorkbook, rawResult)

		if err := saveArtifacts(*outputDir, masterWorkbook); err != nil {
			log.Printf("[ERROR] Failed writing artifacts after %s: %v", file.Name(), err)
		} else {
			log.Printf("[CHECKPOINT] Incremental state flushed to disk after %s", file.Name())
		}

		if db != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			if err := onboarding.ApplyToDatabase(ctx, db, masterWorkbook); err != nil {
				log.Printf("[WARN] Incremental DB sync failed: %v", err)
			} else {
				log.Printf("[DB] Synchronized incremental state to tenant: %s", masterWorkbook.TenantID)
			}
			cancel()
		}

		manifest.Files[file.Name()] = FileRecord{
			SHA256:      hash,
			ProcessedAt: time.Now().UTC().UnixMilli(),
			Status:      "SUCCESS",
		}
		saveManifest(manifestPath, manifest)
	}

	log.Println("[INGEST] Incremental batch run complete.")
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
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, m)
	}
	return m
}

func saveManifest(path string, m *ProcessedManifest) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, data, 0644)
	}
}

func loadExistingWorkbook(path string) *onboarding.FactoryWorkbook {
	wb := &onboarding.FactoryWorkbook{
		TenantID:       "TENANT_DEFAULT",
		FactoryName:    "Unconfigured Factory",
		UnitCode:       "Unit 001",
		ShiftStartHour: 8,
		BundleSize:     20,
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, wb); err == nil {
			log.Printf("[RESUME] Hydrated existing state: %d lines, %d machines, %d workers, %d ops",
				len(wb.Lines), len(wb.Machines), len(wb.Workers), len(wb.OperationsDAG))
		}
	}
	return wb
}

func saveArtifacts(outputDir string, wb *onboarding.FactoryWorkbook) error {
	jsonPath := filepath.Join(outputDir, "factory_workbook.json")
	jsonData, err := json.MarshalIndent(wb, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return err
	}

	csvPath := filepath.Join(outputDir, "factory_onboarding.csv")
	csvData := serializeWorkbookToCSV(wb)
	return os.WriteFile(csvPath, []byte(csvData), 0644)
}

func processFile(client *http.Client, host, model, filePath string) (*RawExtractionResult, error) {
	ext := strings.ToLower(filepath.Ext(filePath))
	var userMsg OllamaChatMessage
	userMsg.Role = "user"

	selectedModel := model
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp":
		selectedModel = "qwen3-vl:8b"
		rawBytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		userMsg.Content = "Extract manufacturing operations, machine layouts, worker rosters, or POs from this image."
		userMsg.Images = []string{base64.StdEncoding.EncodeToString(rawBytes)}

	case ".csv", ".tsv", ".txt", ".json":
		textBytes, err := os.ReadFile(filePath)
		if err != nil {
			return nil, err
		}
		content := string(textBytes)
		if len(content) > 16000 {
			content = content[:16000]
		}
		userMsg.Content = fmt.Sprintf("Extract manufacturing entities from this file:\n\n%s", content)

	case ".xlsx":
		extractedText, err := extractTextFromXLSX(filePath)
		if err != nil {
			return nil, err
		}
		if len(extractedText) > 16000 {
			extractedText = extractedText[:16000]
		}
		userMsg.Content = fmt.Sprintf("Extract manufacturing entities from this spreadsheet:\n\n%s", extractedText)

	case ".pdf":
		extractedText, err := extractTextFromPDF(filePath)
		if err != nil {
			return nil, err
		}
		if len(extractedText) > 16000 {
			extractedText = extractedText[:16000]
		}
		userMsg.Content = fmt.Sprintf("Extract manufacturing entities from this PDF document:\n\n%s", extractedText)

	default:
		return nil, fmt.Errorf("unsupported file extension: %s", ext)
	}

	reqPayload := OllamaChatRequest{
		Model: selectedModel,
		Messages: []OllamaChatMessage{
			{Role: "system", Content: IngestionSystemPrompt},
			userMsg,
		},
		Format: "json",
		Stream: false,
		Options: map[string]any{
			"temperature": 0.0,
			"num_predict": 4096,
			"num_ctx":     16384,
		},
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/chat", host), bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp OllamaChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&chatResp); err != nil {
		return nil, fmt.Errorf("failed decoding response: %w", err)
	}
	if chatResp.Error != "" {
		return nil, fmt.Errorf("ollama runtime error: %s", chatResp.Error)
	}

	var rawResult RawExtractionResult
	if err := json.Unmarshal([]byte(chatResp.Message.Content), &rawResult); err != nil {
		return nil, fmt.Errorf("invalid extracted JSON (%s): %w", chatResp.Message.Content, err)
	}

	return &rawResult, nil
}

func mergeIntoWorkbook(wb *onboarding.FactoryWorkbook, res *RawExtractionResult) {
	if res == nil {
		return
	}
	if res.FactoryProfile != nil {
		if res.FactoryProfile.TenantID.String() != "" {
			wb.TenantID = res.FactoryProfile.TenantID.String()
		}
		if res.FactoryProfile.FactoryName.String() != "" {
			wb.FactoryName = res.FactoryProfile.FactoryName.String()
		}
		if res.FactoryProfile.UnitCode.String() != "" {
			wb.UnitCode = res.FactoryProfile.UnitCode.String()
		}
		if res.FactoryProfile.ShiftStartHour.Int() > 0 {
			wb.ShiftStartHour = res.FactoryProfile.ShiftStartHour.Int()
		}
		if res.FactoryProfile.BundleSize.Int() > 0 {
			wb.BundleSize = res.FactoryProfile.BundleSize.Int()
		}
	}

	lineSeen := make(map[string]bool)
	for _, l := range wb.Lines {
		lineSeen[l.LineID] = true
	}
	for _, l := range res.Lines {
		lid := l.LineID.String()
		if lid != "" && !lineSeen[lid] {
			dwell := l.DwellFloorPct.Float64()
			if dwell <= 0 {
				dwell = 0.40
			}
			wb.Lines = append(wb.Lines, onboarding.LineRow{
				LineID:              lid,
				DisplayName:         l.DisplayName.String(),
				StationsCount:       l.StationsCount.Int(),
				TargetDailyCapacity: l.TargetDailyCapacity.Int(),
				DwellFloorPct:       dwell,
			})
			lineSeen[lid] = true
		}
	}

	machineSeen := make(map[string]bool)
	for _, m := range wb.Machines {
		machineSeen[m.MachineID] = true
	}
	for _, m := range res.Machines {
		mid := m.MachineID.String()
		if mid != "" && !machineSeen[mid] {
			status := strings.ToUpper(m.Status.String())
			if status == "" {
				status = "ACTIVE"
			}
			uph := m.CapacityUPH.Int()
			if uph <= 0 {
				uph = 100
			}
			wb.Machines = append(wb.Machines, onboarding.MachineRow{
				MachineID:      mid,
				LineID:         m.LineID.String(),
				StationID:      m.StationID.String(),
				Name:           m.Name.String(),
				CanonicalClass: m.CanonicalClass.String(),
				CapacityUPH:    uph,
				Status:         status,
			})
			machineSeen[mid] = true
		}
	}

	workerSeen := make(map[string]bool)
	for _, w := range wb.Workers {
		workerSeen[w.WorkerID] = true
	}
	for _, w := range res.Workers {
		wid := w.WorkerID.String()
		if wid != "" && !workerSeen[wid] {
			grade := strings.ToUpper(w.SkillGrade.String())
			if grade == "" {
				grade = "A"
			}
			att := strings.ToUpper(w.AttendanceStatus.String())
			if att == "" {
				att = "PRESENT"
			}
			wb.Workers = append(wb.Workers, onboarding.WorkerRow{
				WorkerID:          wid,
				EmployeeCode:      w.EmployeeCode.String(),
				Name:              w.Name.String(),
				PrimarySkill:      w.PrimarySkill.String(),
				SkillGrade:        grade,
				HourlyRateINR:     w.HourlyRateINR.Float64(),
				AttendanceStatus:  att,
				AssignedLineID:    w.AssignedLineID.String(),
				AssignedStationID: w.AssignedStationID.String(),
			})
			workerSeen[wid] = true
		}
	}

	styleSeen := make(map[string]bool)
	for _, s := range wb.Styles {
		styleSeen[s.StyleCode] = true
	}
	for _, s := range res.Styles {
		code := s.StyleCode.String()
		if code != "" && !styleSeen[code] {
			sid := s.StyleID.String()
			if sid == "" {
				sid = "STYLE_" + code
			}
			wb.Styles = append(wb.Styles, onboarding.StyleRow{
				StyleID:      sid,
				StyleCode:    code,
				Category:     s.Category.String(),
				FitProfile:   s.FitProfile.String(),
				TargetSAMMin: s.TargetSAMMin.Float64(),
				FabricSKU:    s.FabricSKU.String(),
			})
			styleSeen[code] = true
		}
	}

	partSeen := make(map[string]bool)
	for _, p := range wb.StyleParts {
		partSeen[p.StyleCode+":"+p.PartCode] = true
	}
	for _, p := range res.StyleParts {
		key := p.StyleCode.String() + ":" + p.PartCode.String()
		if p.PartCode.String() != "" && !partSeen[key] {
			order := p.SubBundleOrder.Int()
			if order <= 0 {
				order = 1
			}
			wb.StyleParts = append(wb.StyleParts, onboarding.StylePartRow{
				StyleCode:      p.StyleCode.String(),
				PartCode:       p.PartCode.String(),
				PartName:       p.PartName.String(),
				SubBundleOrder: order,
				MaterialSKU:    p.MaterialSKU.String(),
			})
			partSeen[key] = true
		}
	}

	dagSeen := make(map[string]bool)
	for _, d := range wb.OperationsDAG {
		dagSeen[d.StyleCode+":"+d.OperationID] = true
	}
	for _, d := range res.OperationsDAG {
		key := d.StyleCode.String() + ":" + d.OperationID.String()
		if d.OperationID.String() != "" && !dagSeen[key] {
			sam := d.SAMMinutes.Float64()
			if sam <= 0 {
				sam = 0.50
			}
			wb.OperationsDAG = append(wb.OperationsDAG, onboarding.OperationDAGRow{
				StyleCode:      d.StyleCode.String(),
				OperationID:    d.OperationID.String(),
				OperationName:  d.OperationName.String(),
				SAMMinutes:     sam,
				MachineType:    d.MachineType.String(),
				SourcePartCode: d.SourcePartCode.String(),
				TargetPartCode: d.TargetPartCode.String(),
			})
			dagSeen[key] = true
		}
	}

	rollSeen := make(map[string]bool)
	for _, r := range wb.StoreRolls {
		rollSeen[r.RollID] = true
	}
	for _, r := range res.StoreRolls {
		rid := r.RollID.String()
		if rid != "" && !rollSeen[rid] {
			status := strings.ToUpper(r.Status.String())
			if status == "" {
				status = "AVAILABLE"
			}
			wb.StoreRolls = append(wb.StoreRolls, onboarding.StoreRollRow{
				RollID:        rid,
				FactorySKU:    r.FactorySKU.String(),
				LengthM:       r.LengthM.Float64(),
				WidthCM:       r.WidthCM.Float64(),
				ShadeBand:     r.ShadeBand.String(),
				BinLocation:   r.BinLocation.String(),
				Status:        status,
				ReservedOrder: r.ReservedOrder.String(),
				ReservedCutNo: r.ReservedCutNo.Int(),
			})
			rollSeen[rid] = true
		}
	}

	woSeen := make(map[string]bool)
	for _, wo := range wb.WorkOrders {
		woSeen[wo.WorkOrderID] = true
	}
	for _, wo := range res.WorkOrders {
		woid := wo.WorkOrderID.String()
		if woid != "" && !woSeen[woid] {
			status := strings.ToUpper(wo.Status.String())
			if status == "" {
				status = "IN_PROGRESS"
			}
			cJSON := wo.ColorJSON.String()
			if cJSON == "" {
				cJSON = "{}"
			}
			sJSON := wo.SizeJSON.String()
			if sJSON == "" {
				sJSON = "{}"
			}
			wb.WorkOrders = append(wb.WorkOrders, onboarding.WorkOrderRow{
				WorkOrderID:  woid,
				PONumber:     wo.PONumber.String(),
				StyleCode:    wo.StyleCode.String(),
				LineID:       wo.LineID.String(),
				SupervisorID: wo.SupervisorID.String(),
				OrderedQty:   wo.OrderedQty.Int(),
				ColorJSON:    cJSON,
				SizeJSON:     sJSON,
				Status:       status,
			})
			woSeen[woid] = true
		}
	}
}

func extractTextFromXLSX(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", err
	}
	defer r.Close()

	var sharedStrings []string
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			type SST struct {
				SI []struct {
					T string `xml:"t"`
				} `xml:"si"`
			}
			var sst SST
			_ = xml.NewDecoder(rc).Decode(&sst)
			rc.Close()
			for _, item := range sst.SI {
				sharedStrings = append(sharedStrings, item.T)
			}
			break
		}
	}

	var sb strings.Builder
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			type SheetData struct {
				Row []struct {
					C []struct {
						T string `xml:"t,attr"`
						V string `xml:"v"`
					} `xml:"c"`
				} `xml:"sheetData>row"`
			}
			var data SheetData
			_ = xml.NewDecoder(rc).Decode(&data)
			rc.Close()

			sb.WriteString(fmt.Sprintf("--- Sheet: %s ---\n", f.Name))
			for _, row := range data.Row {
				var cellVals []string
				for _, c := range row.C {
					val := c.V
					if c.T == "s" {
						idx, err := strconv.Atoi(c.V)
						if err == nil && idx >= 0 && idx < len(sharedStrings) {
							val = sharedStrings[idx]
						}
					}
					cellVals = append(cellVals, val)
				}
				sb.WriteString(strings.Join(cellVals, ", ") + "\n")
			}
		}
	}
	return sb.String(), nil
}

func extractTextFromPDF(path string) (string, error) {
	bytesData, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	lines := strings.Split(string(bytesData), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
			sb.WriteString(strings.Trim(trimmed, "()") + " ")
		}
	}
	text := sb.String()
	if len(strings.TrimSpace(text)) < 50 {
		return fmt.Sprintf("RAW_BINARY_PDF_SIZE_%d", len(bytesData)), nil
	}
	return text, nil
}

func serializeWorkbookToCSV(wb *onboarding.FactoryWorkbook) string {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	_ = writer.Write([]string{"RECORD_TYPE", "COL_1", "COL_2", "COL_3", "COL_4", "COL_5", "COL_6", "COL_7", "COL_8", "COL_9"})
	_ = writer.Write([]string{"FACTORY_PROFILE", wb.TenantID, wb.FactoryName, wb.UnitCode, strconv.Itoa(wb.ShiftStartHour), strconv.Itoa(wb.BundleSize)})

	for _, l := range wb.Lines {
		_ = writer.Write([]string{"LINE_ROSTER", l.LineID, l.DisplayName, strconv.Itoa(l.StationsCount), strconv.Itoa(l.TargetDailyCapacity), fmt.Sprintf("%.2f", l.DwellFloorPct)})
	}
	for _, m := range wb.Machines {
		_ = writer.Write([]string{"MACHINE_ROSTER", m.MachineID, m.LineID, m.StationID, m.Name, m.CanonicalClass, strconv.Itoa(m.CapacityUPH), m.Status})
	}
	for _, w := range wb.Workers {
		_ = writer.Write([]string{"WORKER_ROSTER", w.WorkerID, w.EmployeeCode, w.Name, w.PrimarySkill, w.SkillGrade, fmt.Sprintf("%.2f", w.HourlyRateINR), w.AttendanceStatus, w.AssignedLineID, w.AssignedStationID})
	}
	for _, s := range wb.Styles {
		_ = writer.Write([]string{"STYLE_MASTER", s.StyleID, s.StyleCode, s.Category, s.FitProfile, fmt.Sprintf("%.2f", s.TargetSAMMin), s.FabricSKU})
	}
	for _, p := range wb.StyleParts {
		_ = writer.Write([]string{"STYLE_PARTS", p.StyleCode, p.PartCode, p.PartName, strconv.Itoa(p.SubBundleOrder), p.MaterialSKU})
	}
	for _, d := range wb.OperationsDAG {
		_ = writer.Write([]string{"OPERATION_DAG", d.StyleCode, d.OperationID, d.OperationName, fmt.Sprintf("%.2f", d.SAMMinutes), d.MachineType, d.SourcePartCode, d.TargetPartCode})
	}
	for _, r := range wb.StoreRolls {
		_ = writer.Write([]string{"STORE_ROLLS", r.RollID, r.FactorySKU, fmt.Sprintf("%.2f", r.LengthM), fmt.Sprintf("%.2f", r.WidthCM), r.ShadeBand, r.BinLocation, r.Status, r.ReservedOrder, strconv.Itoa(r.ReservedCutNo)})
	}
	for _, wo := range wb.WorkOrders {
		_ = writer.Write([]string{"WORK_ORDERS", wo.WorkOrderID, wo.PONumber, wo.StyleCode, wo.LineID, wo.SupervisorID, strconv.Itoa(wo.OrderedQty), wo.ColorJSON, wo.SizeJSON, wo.Status})
	}

	writer.Flush()
	return buf.String()
}