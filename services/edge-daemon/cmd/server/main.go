package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/agie/interviewer"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/attachment"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/crypto"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/cutting"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/ocr"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/packing"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/stores"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/topology"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/wal"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/dbsmith"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/docscribe"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/logicweaver"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type ModuleRegistry struct {
	enabled map[string]bool
}

func NewModuleRegistry(raw string) *ModuleRegistry {
	m := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			m[trimmed] = true
		}
	}
	return &ModuleRegistry{enabled: m}
}

func (m *ModuleRegistry) IsEnabled(mod string) bool {
	return m.enabled[mod]
}

type EdgeServer struct {
	modules        *ModuleRegistry
	ledger         *crypto.MerkleLedger
	attachment     *attachment.AttachmentEngine
	stores         *stores.StoresEngine
	spatialTwin    *topology.SpatialTwin
	wal            *wal.LocalWAL
	db             *sql.DB
	secretKey      string
	packingMu      sync.Mutex
	activePackings map[int]*packing.CartonPackingSession
	ocrMu          sync.Mutex
	activeOCRSess  map[string]*ocr.SignOffSession
	interviewer    *interviewer.InterviewerEngine
	stagedCfg      *interviewer.CandidateConfig
	cfgMu          sync.Mutex
	activeConfig   map[string]interface{}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	secret := os.Getenv("LEDGER_SECRET")
	if secret == "" {
		secret = "DEV_INSECURE_SECRET_REPLACE_IN_PROD"
	}

	walPath := os.Getenv("WAL_PATH")
	if walPath == "" {
		walPath = "./data/wal/events.wal"
	}

	localWAL, err := wal.Open(walPath)
	if err != nil {
		log.Fatalf("Fatal: failed to initialize local WAL at %s: %v", walPath, err)
	}
	defer localWAL.Close()

	ledger := crypto.NewMerkleLedger(secret)
	records, err := localWAL.ReplayAll()
	if err != nil {
		log.Printf("Warning: failed to replay WAL: %v", err)
	} else {
		var auditEvents []crypto.AuditEvent
		for _, rec := range records {
			if rec.Type == "LEDGER_EVENT" {
				var ev crypto.AuditEvent
				if err := json.Unmarshal(rec.Payload, &ev); err == nil {
					auditEvents = append(auditEvents, ev)
				}
			}
		}
		ledger.Hydrate(auditEvents)
		log.Printf("Hydrated ledger to index %d (hash: %s)", ledger.GetCurrentIndex(), ledger.GetLatestHash())
	}

	enabledMods := os.Getenv("ENABLED_MODULES")
	if enabledMods == "" {
		enabledMods = "mod_stores,mod_cutting,mod_sewing,mod_packing,mod_ocr"
	}
	modules := NewModuleRegistry(enabledMods)
	log.Printf("Active modular engines: %s", enabledMods)

	storesEng := stores.NewStoresEngine()
	storesEng.RegisterRoll(stores.RollStock{
		RollID:          "ROLL-007",
		OriginalLengthM: 59.0,
		RemainingLength: 59.0,
		ReservedOrderID: "MK6574J9",
		ReservedCutNo:   1,
		Status:          "RESERVED",
	})

	dbURL := os.Getenv("DATABASE_URL")
	var db *sql.DB
	if dbURL != "" {
		db, err = sql.Open("pgx", dbURL)
		if err != nil {
			log.Printf("Notice: Core database unreachable (%v). Air-gapped fallback.", err)
		} else {
			db.SetMaxOpenConns(25)
			db.SetMaxIdleConns(5)
		}
	}

	twin := topology.NewSpatialTwin()

	defaultCfg := map[string]interface{}{
		"plant_name":    "Noble Garments",
		"unit_code":     "Unit 002/02",
		"pilot_line":    "Line 14",
		"style_code":    "KK524867",
		"po_number":     "MK6574J9",
		"target_qty":    600,
		"configured_at": time.Now().UTC().UnixMilli(),
	}

	server := &EdgeServer{
		modules:        modules,
		ledger:         ledger,
		attachment:     attachment.NewAttachmentEngine(),
		stores:         storesEng,
		spatialTwin:    twin,
		wal:            localWAL,
		db:             db,
		secretKey:      secret,
		interviewer:    interviewer.NewInterviewerEngine(),
		activeConfig:   defaultCfg,
		activePackings: make(map[int]*packing.CartonPackingSession),
		activeOCRSess:  make(map[string]*ocr.SignOffSession),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/api/v1/modules", server.handleGetModules)
	mux.HandleFunc("/api/v1/ledger/append", server.handleLedgerAppend)
	mux.HandleFunc("/api/v1/ledger/latest", server.handleLedgerLatest)
	mux.HandleFunc("/api/v1/ledger/verify", server.handleLedgerVerify)
	mux.HandleFunc("/api/v1/agie/chat", server.handleAgieChat)
	mux.HandleFunc("/api/v1/agie/confirm", server.handleAgieConfirm)
	mux.HandleFunc("/api/v1/config", server.handleGetConfig)

	if modules.IsEnabled("mod_stores") {
		mux.HandleFunc("/api/v1/stores/indent-check", server.handleStoresIndentCheck)
		mux.HandleFunc("/api/v1/stores/issue-double-witness", server.handleStoresIssueDoubleWitness)
	}
	if modules.IsEnabled("mod_cutting") {
		mux.HandleFunc("/api/v1/cutting/plan-target", server.handleCuttingPlanTarget)
		mux.HandleFunc("/api/v1/cutting/lay-roll", server.handleCuttingLayRoll)
	}
	if modules.IsEnabled("mod_sewing") {
		mux.HandleFunc("/api/v1/sewing/bind-tag", server.handleSewingBindTag)
		mux.HandleFunc("/api/v1/sewing/validate-pair", server.handleSewingValidatePair)
		mux.HandleFunc("/api/v1/sewing/register-rule", server.handleSewingRegisterRule)
	}
	if modules.IsEnabled("mod_packing") {
		mux.HandleFunc("/api/v1/packing/session/start", server.handlePackingStart)
		mux.HandleFunc("/api/v1/packing/session/scan", server.handlePackingScan)
		mux.HandleFunc("/api/v1/packing/session/seal", server.handlePackingSeal)
	}
	if modules.IsEnabled("mod_ocr") {
		mux.HandleFunc("/api/v1/ocr/reconcile", server.handleOCRReconcile)
		mux.HandleFunc("/api/v1/ocr/sign", server.handleOCRSign)
		mux.HandleFunc("/api/v1/ocr/seal", server.handleOCRSeal)
	}

	mux.HandleFunc("/api/v1/topology/devices", server.handleGetTopologyDevices)

	mux.HandleFunc("/plant", func(w http.ResponseWriter, r *http.Request) {
		data, err := web.GetPlantViewHTML()
		if err != nil {
			http.Error(w, "View not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})

	mux.Handle("/static/", http.StripPrefix("/static/", web.StaticHandler()))

	mux.HandleFunc("/sew01", func(w http.ResponseWriter, r *http.Request) {
		data, err := web.GetSEW01HTML()
		if err != nil {
			http.Error(w, "HMI not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("plant-edge-os kernel running on :%s", port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down plant-edge-os...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if db != nil {
		_ = db.Close()
	}
	_ = httpServer.Shutdown(ctx)
	log.Println("plant-edge-os cleanly stopped.")
}

func (s *EdgeServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "UP",
		"node":            "plant-edge-os",
		"wal_synced":      true,
		"enabled_modules": s.modules.enabled,
	})
}

func (s *EdgeServer) handleGetModules(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.modules.enabled)
}

type AppendRequest struct {
	StationID  string `json:"station_id"`
	OperatorID string `json:"operator_id"`
	EventType  string `json:"event_type"`
	Payload    string `json:"payload"`
}

func (s *EdgeServer) handleLedgerAppend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req AppendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	event, err := s.ledger.AppendEvent(req.StationID, req.OperatorID, req.EventType, req.Payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	evBytes, _ := json.Marshal(event)
	_, _ = s.wal.Append("LEDGER_EVENT", evBytes)

	if s.db != nil {
		currHour := time.Now().Hour()
		hourWindow := (currHour - 8) + 1
		if hourWindow < 1 {
			hourWindow = 1
		}

		colToIncrement := "actual_units = hourly_line_metrics.actual_units + 1"
		if req.EventType == "DEFECT_TAG" {
			colToIncrement = "defect_units = hourly_line_metrics.defect_units + 1"
		} else if req.EventType == "ANDON_CALL" {
			colToIncrement = "andon_downtime_minutes = hourly_line_metrics.andon_downtime_minutes + 5"
		}

		query := fmt.Sprintf(`
			INSERT INTO hourly_line_metrics (tenant_id, line_id, shift_date, hour_window, target_units, actual_units, defect_units, andon_downtime_minutes)
			VALUES ('TENANT_DEFAULT', 'Line 14', CURRENT_DATE, %d, 75, 1, 0, 0)
			ON CONFLICT (tenant_id, line_id, shift_date, hour_window)
			DO UPDATE SET %s;
		`, hourWindow, colToIncrement)

		if _, dbErr := s.db.Exec(query); dbErr != nil {
			log.Printf("Warning: failed to update hourly_line_metrics: %v", dbErr)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(event)
}

func (s *EdgeServer) handleLedgerVerify(w http.ResponseWriter, r *http.Request) {
	records, err := s.wal.ReplayAll()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to replay WAL: %v", err), http.StatusInternalServerError)
		return
	}

	tempLedger := crypto.NewMerkleLedger(s.secretKey)
	var auditEvents []crypto.AuditEvent
	for _, rec := range records {
		if rec.Type == "LEDGER_EVENT" {
			var ev crypto.AuditEvent
			if err := json.Unmarshal(rec.Payload, &ev); err == nil {
				auditEvents = append(auditEvents, ev)
			}
		}
	}
	tempLedger.Hydrate(auditEvents)

	isValid := tempLedger.GetLatestHash() == s.ledger.GetLatestHash()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"verified":      isValid,
		"total_events":  tempLedger.GetCurrentIndex(),
		"current_hash":  s.ledger.GetLatestHash(),
		"audited_hash":  tempLedger.GetLatestHash(),
	})
}

func (s *EdgeServer) handleLedgerLatest(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"latest_hash": s.ledger.GetLatestHash(),
	})
}

type IndentCheckRequest struct {
	SKU            string  `json:"sku"`
	BOMRequiredQty float64 `json:"bom_required_qty"`
	UnreservedQty  float64 `json:"unreserved_qty"`
}

func (s *EdgeServer) handleStoresIndentCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req IndentCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	calc := stores.CalculateNetIndent(req.SKU, req.BOMRequiredQty, req.UnreservedQty)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(calc)
}

type DoubleWitnessIssueRequest struct {
	RollID        string  `json:"roll_id"`
	OrderID       string  `json:"order_id"`
	CutNo         int     `json:"cut_no"`
	IssuedLengthM float64 `json:"issued_length_m"`
	IssuerID      string  `json:"issuer_id"`
	RecipientID   string  `json:"recipient_id"`
}

func (s *EdgeServer) handleStoresIssueDoubleWitness(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req DoubleWitnessIssueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	record, err := s.stores.DoubleWitnessIssue(
		req.RollID, req.OrderID, req.CutNo, req.IssuedLengthM,
		req.IssuerID, req.RecipientID, s.secretKey,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(record)
}

type CutPlanTargetRequest struct {
	OrderID     string `json:"order_id"`
	SizeVariant string `json:"size_variant"`
	OrderQty    int    `json:"order_qty"`
}

func (s *EdgeServer) handleCuttingPlanTarget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req CutPlanTargetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	target := cutting.CalculateCutTarget(req.OrderID, req.SizeVariant, req.OrderQty)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(target)
}

type LayRollRequest struct {
	RollID        string  `json:"roll_id"`
	OrderID       string  `json:"order_id"`
	CutNo         int     `json:"cut_no"`
	LayStart      int     `json:"lay_start"`
	LayEnd        int     `json:"lay_end"`
	EndBitLengthM float64 `json:"end_bit_length_m"`
}

func (s *EdgeServer) handleCuttingLayRoll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req LayRollRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	record, err := cutting.EvaluateEndBitDisposition(
		req.RollID, req.OrderID, req.CutNo, req.LayStart, req.LayEnd, req.EndBitLengthM,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(record)
}

type BindTagRequest struct {
	NFCUID string                   `json:"nfc_uid"`
	Node   attachment.SubBundleNode `json:"node"`
}

func (s *EdgeServer) handleSewingBindTag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req BindTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	s.attachment.BindTag(req.NFCUID, req.Node)
	w.WriteHeader(http.StatusCreated)
}

type ValidatePairRequest struct {
	StyleID   string `json:"style_id"`
	SourceNFC string `json:"source_nfc"`
	TargetNFC string `json:"target_nfc"`
}

func (s *EdgeServer) handleSewingValidatePair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ValidatePairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := s.attachment.ValidatePair(req.StyleID, req.SourceNFC, req.TargetNFC); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ACCEPT"})
}

type RegisterRuleRequest struct {
	StyleID        string `json:"style_id"`
	ParentPartCode string `json:"parent_part_code"`
	ChildPartCode  string `json:"child_part_code"`
}

func (s *EdgeServer) handleSewingRegisterRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RegisterRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	s.attachment.RegisterRule(req.StyleID, req.ParentPartCode, req.ChildPartCode)
	w.WriteHeader(http.StatusOK)
}

type StartPackingRequest struct {
	BoxID            int                  `json:"box_id"`
	PackType         string               `json:"pack_type"`
	Slots            []*packing.RatioSlot `json:"slots"`
	ExpectedWeightKg float64              `json:"expected_weight_kg"`
	ToleranceKg      float64              `json:"tolerance_kg"`
}

func (s *EdgeServer) handlePackingStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req StartPackingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	s.packingMu.Lock()
	s.activePackings[req.BoxID] = packing.NewCartonPackingSession(
		req.BoxID, req.PackType, req.Slots, req.ExpectedWeightKg, req.ToleranceKg,
	)
	s.packingMu.Unlock()

	w.WriteHeader(http.StatusCreated)
}

type ScanUPCRequest struct {
	BoxID int    `json:"box_id"`
	UPC   string `json:"upc"`
}

func (s *EdgeServer) handlePackingScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ScanUPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	s.packingMu.Lock()
	sess, exists := s.activePackings[req.BoxID]
	s.packingMu.Unlock()

	if !exists {
		http.Error(w, "Box session not found", http.StatusNotFound)
		return
	}

	if err := sess.ScanItemUPC(req.UPC); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type SealBoxRequest struct {
	BoxID          int     `json:"box_id"`
	MeasuredWeight float64 `json:"measured_weight_kg"`
}

func (s *EdgeServer) handlePackingSeal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req SealBoxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	s.packingMu.Lock()
	sess, exists := s.activePackings[req.BoxID]
	s.packingMu.Unlock()

	if !exists {
		http.Error(w, "Box session not found", http.StatusNotFound)
		return
	}

	if err := sess.VerifyAndSeal(req.MeasuredWeight); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "SEALED"})
}

type ReconcileRequest struct {
	OrderID         string             `json:"order_id"`
	CutNo           int                `json:"cut_no"`
	Breakdown       ocr.StageBreakdown `json:"breakdown"`
	VarianceComment string             `json:"variance_comment"`
}

func (s *EdgeServer) handleOCRReconcile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ReconcileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	report, err := ocr.EvaluateConservation(req.OrderID, req.CutNo, req.Breakdown, req.VarianceComment)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	key := req.OrderID
	s.ocrMu.Lock()
	s.activeOCRSess[key] = ocr.NewSignOffSession(report, s.secretKey)
	s.ocrMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

type SignRequest struct {
	OrderID    string `json:"order_id"`
	Department string `json:"department"`
	SignerID   string `json:"signer_id"`
}

func (s *EdgeServer) handleOCRSign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req SignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	s.ocrMu.Lock()
	sess, exists := s.activeOCRSess[req.OrderID]
	s.ocrMu.Unlock()

	if !exists {
		http.Error(w, "Reconciliation session not initialized for order", http.StatusNotFound)
		return
	}

	if err := sess.Sign(req.Department, req.SignerID); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ACCEPTED",
		"department": req.Department,
		"signature":  sess.Signatures[req.Department],
	})
}

type SealRequest struct {
	OrderID string `json:"order_id"`
}

func (s *EdgeServer) handleOCRSeal(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req SealRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	s.ocrMu.Lock()
	sess, exists := s.activeOCRSess[req.OrderID]
	s.ocrMu.Unlock()

	if !exists {
		http.Error(w, "Reconciliation session not initialized for order", http.StatusNotFound)
		return
	}

	cert, err := sess.Seal()
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cert)
}

func (s *EdgeServer) handleGetTopologyDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.spatialTwin.GetDevices())
}

type AgieChatRequest struct {
	Mode     string                    `json:"mode"`
	Messages []interviewer.LLMMessage `json:"messages"`
}

func (s *EdgeServer) handleAgieChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req AgieChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	reply, err := s.interviewer.Chat(r.Context(), s.db, req.Messages)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"reply": reply})
}

type ConfirmRequest struct {
	CandidateConfig interviewer.CandidateConfig `json:"candidate_config"`
	ConfirmedBy     string                     `json:"confirmed_by"`
	ConfirmationPIN string                     `json:"confirmation_pin"`
}

func (s *EdgeServer) handleAgieConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ConfirmRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if req.ConfirmationPIN == "" || req.ConfirmedBy == "" {
		http.Error(w, "ERR_UNCONFIRMED: Human approval signature required before code/schema updates", http.StatusBadRequest)
		return
	}

	smith := dbsmith.NewDBSmithEngine(req.CandidateConfig.TenantID)
	migrationSQL, _ := smith.GenerateSafeMigration(dbsmith.CustomFieldRequest{
		EntityName: "store_rolls",
		FieldName:  "custom_intake_batch",
		FieldType:  "VARCHAR(64)",
		IsNullable: true,
	})

	weaver := logicweaver.NewLogicWeaverEngine(req.CandidateConfig.TenantID)
	rule, _ := weaver.CompileRule("RULE-M01", "DWELL", "Enforce 40% SAM dwell floor", "KK524867", 36, 14)

	scribe := docscribe.NewDocScribeEngine(req.CandidateConfig.TenantID)
	manual, _ := scribe.GeneratePlainEnglishManual("LINE14_ST01", "HEM_FOLD", "HI")

	cfgPath := "services/edge-daemon/internal/web/static/config.json"
	cfgFile, err := os.Create(cfgPath)
	if err == nil {
		_ = json.NewEncoder(cfgFile).Encode(req.CandidateConfig)
		cfgFile.Close()
	}

	s.cfgMu.Lock()
	s.activeConfig = map[string]interface{}{
		"plant_name":    req.CandidateConfig.FactoryName,
		"unit_code":     req.CandidateConfig.UnitNumber,
		"pilot_line":    req.CandidateConfig.Lines[0].DisplayName,
		"style_code":    "KK524867",
		"po_number":     "MK6574J9",
		"target_qty":    req.CandidateConfig.Lines[0].TargetDailyCapacity,
		"configured_at": time.Now().UTC().UnixMilli(),
	}
	s.cfgMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "APPLIED",
		"migration_sql":   migrationSQL,
		"compiled_rule":   rule,
		"operator_manual": manual,
	})
}

func (s *EdgeServer) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	s.cfgMu.Lock()
	defer s.cfgMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(s.activeConfig)
}