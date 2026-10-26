// Author: Sunil Jain, 2026-10-25 — Repair planning API compilation and database readiness.
package main

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
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
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/ocr"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/packing"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/topology"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/wal"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/cutting"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/dbsmith"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/docscribe"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/logicweaver"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/onboarding"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/planning"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/sewing"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/stores"
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
	uploadMu       sync.Mutex
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
		port = "18530"
	}

	secret := os.Getenv("LEDGER_SECRET")
	if secret == "" {
		log.Fatal("FATAL: LEDGER_SECRET environment variable must be set")
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

	ledger := crypto.NewMerkleLedger([]byte(secret))
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
	// Root landing page links to the independently served embedded HMIs.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Agiparel Factory OS</title></head><body><main><h1>Agiparel Factory OS</h1><nav aria-label="Applications"><ul><li><a href="/plant">Plant dashboard</a></li><li><a href="/sew01">Sewing HMI</a></li><li><a href="/onboarding">Onboarding</a></li><li><a href="/planning">Planning</a></li><li><a href="/cutting">Cutting</a></li></ul></nav></main></body></html>`)
	})

	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/api/v1/modules", server.handleGetModules)
	mux.HandleFunc("/api/v1/ledger/append", server.handleLedgerAppend)
	mux.HandleFunc("/api/v1/ledger/latest", server.handleLedgerLatest)
	mux.HandleFunc("/api/v1/ledger/verify", server.handleLedgerVerify)
	mux.HandleFunc("/api/v1/agie/chat", server.handleAgieChat)
	mux.HandleFunc("/api/v1/agie/confirm", server.handleAgieConfirm)
	mux.HandleFunc("/api/v1/issues", server.handleGetIssues)
	mux.HandleFunc("/api/v1/config", server.handleGetConfig)
	mux.HandleFunc("/api/v1/onboarding/template.csv", server.handleOnboardingTemplate)
	mux.HandleFunc("/api/v1/onboarding/upload", server.handleOnboardingUpload)
	mux.HandleFunc("/api/v1/plant/overview", server.handlePlantOverview)
	mux.HandleFunc("/api/v1/issues/resolve", server.handleResolveIssue)
	mux.HandleFunc("/api/v1/planning/assessment", server.handlePlanningAssessment)
	mux.HandleFunc("/api/v1/planning/upload-docs", server.handlePlanningUploadDocs)
	mux.HandleFunc("/api/v1/planning/readiness-questions", server.handlePlanningReadinessQuestions)
	mux.HandleFunc("/planning", func(w http.ResponseWriter, r *http.Request) {
		data, err := web.GetOnboardingHTML()
		if err != nil {
			http.Error(w, "Planning view not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})
	mux.HandleFunc("/api/v1/planning/quick-fix", server.handlePlanningQuickFix)
	mux.HandleFunc("/api/v1/planning/approve", server.handlePlanningApprove)
	mux.HandleFunc("/api/v1/planning/email", server.handlePlanningEmail)
	mux.HandleFunc("/api/v1/cutting/optimize-geometry", server.handleCuttingOptimizeGeometry)
	mux.HandleFunc("/cutting", func(w http.ResponseWriter, r *http.Request) {
		data, err := web.GetCuttingViewHTML()
		if err != nil {
			http.Error(w, "Cutting view not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})

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

	mux.HandleFunc("/onboarding", func(w http.ResponseWriter, r *http.Request) {
		data, err := web.GetOnboardingHTML()
		if err != nil {
			http.Error(w, "Onboarding view not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	})

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 240 * time.Second,
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

// Author: Sunil Jain, CyberinQ LLC
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

	if req.EventType == "CYCLE_DONE" {
		var p struct {
			SAM         float64 `json:"sam"`
			ActualSec   float64 `json:"actual_sec"`
			OpActiveSec float64 `json:"op_active_sec"`
		}
		if err := json.Unmarshal([]byte(req.Payload), &p); err == nil && p.SAM > 0 {
			measuredSec := p.OpActiveSec
			if measuredSec <= 0 {
				measuredSec = p.ActualSec
			}
			now := time.Now().UTC()
			dwellRes, err := sewing.ValidateCycleDwell(sewing.CycleSubmission{
				StationID:      req.StationID,
				OperatorID:     req.OperatorID,
				OperationID:    "SEW_CYCLE",
				FactorySAMMin:  p.SAM / 60.0,
				CycleStartedAt: now.Add(-time.Duration(measuredSec * float64(time.Second))),
				SubmittedAt:    now,
			})
			if err != nil || !dwellRes.IsValid {
				http.Error(w, dwellRes.Reason, http.StatusConflict)
				return
			}
		}
	}

	event, err := s.ledger.AppendEvent(req.StationID, req.OperatorID, req.EventType, req.Payload)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	evBytes, _ := json.Marshal(event)
	_, _ = s.wal.Append("LEDGER_EVENT", evBytes)

	if s.db != nil && (req.EventType == "CYCLE_DONE" || req.EventType == "DEFECT_TAG" || req.EventType == "ANDON_CALL") {
		loc, err := time.LoadLocation("Asia/Kolkata")
		if err != nil {
			loc = time.FixedZone("IST", 5*3600+1800)
		}
		currHour := time.Now().In(loc).Hour()
		hourWindow := (currHour - 8) + 1
		if hourWindow < 1 {
			hourWindow = 1
		} else if hourWindow > 16 {
			hourWindow = 16
		}

		s.cfgMu.Lock()
		tenantID, _ := s.activeConfig["tenant_id"].(string)
		lineID, _ := s.activeConfig["pilot_line"].(string)
		s.cfgMu.Unlock()
		if tenantID == "" {
			tenantID = "TENANT_DEFAULT"
		}
		if lineID == "" {
			lineID = "Line 14"
		}

		initActual, initDefect, initAndon := 1, 0, 0
		colToIncrement := "actual_units = hourly_line_metrics.actual_units + 1"
		switch req.EventType {
		case "DEFECT_TAG":
			initActual, initDefect, initAndon = 0, 1, 0
			colToIncrement = "defect_units = hourly_line_metrics.defect_units + 1"
		case "ANDON_CALL":
			initActual, initDefect, initAndon = 0, 0, 5
			colToIncrement = "andon_downtime_minutes = hourly_line_metrics.andon_downtime_minutes + 5"
		}

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		tx, txErr := s.db.BeginTx(ctx, nil)
		if txErr == nil {
			_, _ = tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID)
			query := fmt.Sprintf(`
				INSERT INTO hourly_line_metrics (tenant_id, line_id, shift_date, hour_window, target_units, actual_units, defect_units, andon_downtime_minutes)
				VALUES ($1, $2, (CURRENT_TIMESTAMP AT TIME ZONE 'Asia/Kolkata')::date, $3, 75, $4, $5, $6)
				ON CONFLICT (tenant_id, line_id, shift_date, hour_window) DO UPDATE SET %s;
			`, colToIncrement)
			if _, dbErr := tx.ExecContext(ctx, query, tenantID, lineID, hourWindow, initActual, initDefect, initAndon); dbErr != nil {
				log.Printf("Warning: failed to update hourly_line_metrics: %v", dbErr)
				_ = tx.Rollback()
			} else {
				_ = tx.Commit()
			}
		}
		cancel()
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

	tempLedger := crypto.NewMerkleLedger([]byte(s.secretKey))
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

	chainValid, verifyErr := tempLedger.VerifyChain()
	isValid := chainValid && verifyErr == nil && (tempLedger.GetLatestHash() == s.ledger.GetLatestHash())

	resp := map[string]interface{}{
		"verified":     isValid,
		"total_events": tempLedger.GetCurrentIndex(),
		"current_hash": s.ledger.GetLatestHash(),
		"audited_hash": tempLedger.GetLatestHash(),
	}
	if verifyErr != nil {
		resp["error"] = verifyErr.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
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

	calc, _ := stores.EvaluateIndent(stores.IndentRequest{SKU: req.SKU, BOMRequiredQty: req.BOMRequiredQty, UnreservedQty: req.UnreservedQty})
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
	target, err := cutting.CalculateCutTarget(req.OrderID, req.SizeVariant, req.OrderQty)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
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

func ocrSessionKey(orderID string, cutNo int) string {
	if cutNo <= 0 {
		cutNo = 1
	}
	return fmt.Sprintf("%s:%d", orderID, cutNo)
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
	key := ocrSessionKey(req.OrderID, req.CutNo)
	s.ocrMu.Lock()
	s.activeOCRSess[key] = ocr.NewSignOffSession(report, s.secretKey)
	s.ocrMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(report)
}

type SignRequest struct {
	OrderID    string `json:"order_id"`
	CutNo      int    `json:"cut_no,omitempty"`
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
	key := ocrSessionKey(req.OrderID, req.CutNo)
	s.ocrMu.Lock()
	sess, exists := s.activeOCRSess[key]
	s.ocrMu.Unlock()
	if !exists {
		http.Error(w, "Reconciliation session not initialized for order/cut", http.StatusNotFound)
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
	CutNo   int    `json:"cut_no,omitempty"`
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
	key := ocrSessionKey(req.OrderID, req.CutNo)
	s.ocrMu.Lock()
	sess, exists := s.activeOCRSess[key]
	s.ocrMu.Unlock()
	if !exists {
		http.Error(w, "Reconciliation session not initialized for order/cut", http.StatusNotFound)
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
	Mode     string                   `json:"mode"`
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

	// Exact operational questions bypass the optional LLM: answer from tenant-scoped SQL.
	if intent := operationalIntent(req.Messages); intent != "" {
		reply, err := groundedOperationalReply(r.Context(), s.db, intent)
		if err != nil {
			log.Printf("[AGIE] Operational query failed: %v", err)
			http.Error(w, "Verified factory records are unavailable. Please check the database connection.", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"reply": reply, "source": "live_postgresql"})
		return
	}
	reply, err := s.interviewer.Chat(r.Context(), s.db, req.Messages)
	if err != nil {
		log.Printf("[AGIE] Chat backend failed: %v", err)
		http.Error(w, "AI service is unavailable; verify Ollama on the host. No answer was inferred.", http.StatusServiceUnavailable)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"reply": reply})
}

type ConfirmRequest struct {
	CandidateConfig interviewer.CandidateConfig `json:"candidate_config"`
	ConfirmedBy     string                      `json:"confirmed_by"`
	ConfirmationPIN string                      `json:"confirmation_pin"`
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
	if len(req.CandidateConfig.Lines) == 0 {
		http.Error(w, "ERR_INVALID_CONFIG: At least one line specification is required", http.StatusBadRequest)
		return
	}

	smith := dbsmith.NewDBSmithEngine(req.CandidateConfig.TenantID)
	migrationSQL, err := smith.GenerateSafeMigration(dbsmith.CustomFieldRequest{
		EntityName: "store_rolls",
		FieldName:  "custom_intake_batch",
		FieldType:  "VARCHAR(64)",
		IsNullable: true,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	weaver := logicweaver.NewLogicWeaverEngine(req.CandidateConfig.TenantID)
	rule, _ := weaver.CompileRule("RULE-M01", "DWELL", "Enforce 40% SAM dwell floor", "KK524867", 36, 14)

	scribe := docscribe.NewDocScribeEngine(req.CandidateConfig.TenantID)
	manual, _ := scribe.GeneratePlainEnglishManual("LINE14_ST01", "HEM_FOLD", "HI")

	cfgPath := "services/edge-daemon/internal/web/static/config.json"
	if cfgFile, err := os.Create(cfgPath); err == nil {
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

type FloorIssueResponse struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenant_id"`
	IssueType  string     `json:"issue_type"`
	Severity   string     `json:"severity"`
	PlantID    string     `json:"plant_id"`
	LineID     string     `json:"line_id"`
	StationID  string     `json:"station_id,omitempty"`
	MachineID  string     `json:"machine_id,omitempty"`
	OperatorID string     `json:"operator_id,omitempty"`
	Title      string     `json:"title"`
	StartedAt  time.Time  `json:"started_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
	OwnerID    string     `json:"owner_id"`
	Status     string     `json:"status"`
}

func (s *EdgeServer) handleGetIssues(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if s.db == nil {
		_ = json.NewEncoder(w).Encode([]FloorIssueResponse{})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to begin transaction: %v", err), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "SET LOCAL app.current_tenant_id = 'TENANT_DEFAULT'"); err != nil {
		http.Error(w, fmt.Sprintf("Failed to set tenant context: %v", err), http.StatusInternalServerError)
		return
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT id, tenant_id, issue_type, severity, plant_id, line_id,
		       COALESCE(station_id, ''), COALESCE(machine_id, ''), COALESCE(operator_id, ''),
		       title, started_at, resolved_at, owner_id, status
		FROM floor_issues
		WHERE status IN ('open', 'in_progress')
		ORDER BY started_at DESC;
	`)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to query floor issues: %v", err), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	issues := make([]FloorIssueResponse, 0)
	for rows.Next() {
		var iss FloorIssueResponse
		var resolvedAt sql.NullTime
		if err := rows.Scan(
			&iss.ID, &iss.TenantID, &iss.IssueType, &iss.Severity, &iss.PlantID, &iss.LineID,
			&iss.StationID, &iss.MachineID, &iss.OperatorID,
			&iss.Title, &iss.StartedAt, &resolvedAt, &iss.OwnerID, &iss.Status,
		); err == nil {
			if resolvedAt.Valid {
				iss.ResolvedAt = &resolvedAt.Time
			}
			issues = append(issues, iss)
		}
	}
	_ = json.NewEncoder(w).Encode(issues)
}
func (s *EdgeServer) handleOnboardingTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="agiparel_preboarding_template.csv"`)
	_, _ = w.Write([]byte(onboarding.GenerateTemplateCSV()))
}

func (s *EdgeServer) handleOnboardingUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var csvReader io.Reader = r.Body
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "Failed to parse multipart form: "+err.Error(), http.StatusBadRequest)
			return
		}
		file, _, err := r.FormFile("workbook")
		if err != nil {
			http.Error(w, "Missing 'workbook' file field: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()
		csvReader = file
	}

	wb, err := onboarding.ParseWorkbookCSV(csvReader)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := onboarding.ApplyToDatabase(r.Context(), s.db, wb); err != nil {
		http.Error(w, "Database provisioning failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Hydrate in-memory StoresEngine from workbook rolls
	for _, r := range wb.StoreRolls {
		s.stores.RegisterRoll(stores.RollStock{
			RollID:          r.RollID,
			OriginalLengthM: r.LengthM,
			RemainingLength: r.LengthM,
			ReservedOrderID: r.ReservedOrder,
			ReservedCutNo:   r.ReservedCutNo,
			Status:          r.Status,
		})
	}

	// Hydrate in-memory AttachmentEngine DAG rules from workbook operations
	for _, op := range wb.OperationsDAG {
		if op.SourcePartCode != "" && op.TargetPartCode != "" {
			s.attachment.RegisterRule(op.StyleCode, op.SourcePartCode, op.TargetPartCode)
		}
	}

	pilotLine := "Line 01"
	targetQty := 0
	if len(wb.Lines) > 0 {
		pilotLine = wb.Lines[0].LineID
		targetQty = wb.Lines[0].TargetDailyCapacity
	}
	styleCode := ""
	if len(wb.Styles) > 0 {
		styleCode = wb.Styles[0].StyleCode
	}
	poNumber := ""
	if len(wb.WorkOrders) > 0 {
		poNumber = wb.WorkOrders[0].PONumber
		if wb.WorkOrders[0].OrderedQty > 0 {
			targetQty = wb.WorkOrders[0].OrderedQty
		}
	}

	s.cfgMu.Lock()
	s.activeConfig = map[string]interface{}{
		"tenant_id":     wb.TenantID,
		"plant_name":    wb.FactoryName,
		"unit_code":     wb.UnitCode,
		"pilot_line":    pilotLine,
		"style_code":    styleCode,
		"po_number":     poNumber,
		"target_qty":    targetQty,
		"lines":         wb.Lines,
		"configured_at": time.Now().UTC().UnixMilli(),
	}
	s.cfgMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "ONBOARDED",
		"tenant_id":       wb.TenantID,
		"factory_name":    wb.FactoryName,
		"unit_code":       wb.UnitCode,
		"lines_loaded":    len(wb.Lines),
		"machines_loaded": len(wb.Machines),
		"workers_loaded":  len(wb.Workers),
		"styles_loaded":   len(wb.Styles),
		"parts_loaded":    len(wb.StyleParts),
		"dag_ops_loaded":  len(wb.OperationsDAG),
		"rolls_loaded":    len(wb.StoreRolls),
		"orders_loaded":   len(wb.WorkOrders),
	})
}

func (s *EdgeServer) handlePlantOverview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	s.cfgMu.Lock()
	cfg := s.activeConfig
	s.cfgMu.Unlock()

	type LineSummary struct {
		LineID   string  `json:"line_id"`
		Style    string  `json:"style"`
		Achieved int     `json:"achieved"`
		Target   int     `json:"target"`
		DHU      float64 `json:"dhu"`
		Status   string  `json:"status"`
	}

	var lines []LineSummary
	if s.db != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err == nil {
			defer tx.Rollback()
			tenantID, _ := cfg["tenant_id"].(string)
			if tenantID == "" {
				tenantID = "TENANT_DEFAULT"
			}
			_, _ = tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID)
			rows, qErr := tx.QueryContext(ctx, `
				SELECT dwo.line_id, sm.style_code, dwo.ordered_qty,
				       COALESCE((SELECT SUM(actual_units) FROM hourly_line_metrics h WHERE h.line_id = dwo.line_id AND h.shift_date = CURRENT_DATE), 0) AS achieved,
				       COALESCE((SELECT SUM(defect_units) FROM hourly_line_metrics h WHERE h.line_id = dwo.line_id AND h.shift_date = CURRENT_DATE), 0) AS defects,
				       (SELECT COUNT(*) FROM floor_issues fi WHERE fi.line_id = dwo.line_id AND fi.status IN ('open', 'in_progress')) AS open_issues
				FROM dynamic_work_orders dwo
				JOIN styles_master sm ON dwo.style_id = sm.id
				WHERE dwo.status IN ('IN_PROGRESS', 'PLANNED')
				ORDER BY dwo.line_id;
			`)
			if qErr == nil {
				for rows.Next() {
					var ls LineSummary
					var defects, openIssues int
					if err := rows.Scan(&ls.LineID, &ls.Style, &ls.Target, &ls.Achieved, &defects, &openIssues); err == nil {
						if ls.Achieved > 0 {
							ls.DHU = float64(defects) / float64(ls.Achieved) * 100.0
						}
						ls.Status = "ok"
						if openIssues > 0 || ls.DHU > 2.5 {
							ls.Status = "warn"
						}
						lines = append(lines, ls)
					}
				}
				rows.Close()
			}
		}
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"config": cfg,
		"lines":  lines,
	})
}

func (s *EdgeServer) loadCumulativeWorkbook() *onboarding.FactoryWorkbook {
	wb := &onboarding.FactoryWorkbook{TenantID: "TENANT_DEFAULT", ShiftStartHour: 8, BundleSize: 20}
	if data, err := os.ReadFile("./data/staging/output/factory_workbook.json"); err == nil {
		if err = json.Unmarshal(data, wb); err != nil {
			log.Printf("[ERROR] Malformed cumulative workbook: %v", err)
		}
	}
	return wb
}

// Upload raw documents, then run the same deterministic+AI intake executable as the CLI.
// Serialized re-ingestion prevents two managers from corrupting the manifest simultaneously.
func (s *EdgeServer) handlePlanningUploadDocs(w http.ResponseWriter, r *http.Request) {
	s.handlePlanningUploadJob(w, r)
}

// intakeFailureSummary avoids echoing credentials or full model replies to browsers.
func intakeFailureSummary(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, "[FATAL]") || strings.Contains(line, "[ERROR]") {
			// pgx errors may include a connection URL: do not expose the full diagnostics.
			if strings.Contains(strings.ToLower(line), "postgres") || strings.Contains(strings.ToLower(line), "database") {
				return "PostgreSQL import failed; inspect container logs for the exact constraint."
			}
			if strings.Contains(line, "documents failed extraction") {
				continue // Prefer the per-file error above the aggregate failure.
			}
			if n := strings.Index(line, "[ERROR]"); n >= 0 {
				line = line[n+len("[ERROR]"):]
			} else if n := strings.Index(line, "[FATAL]"); n >= 0 {
				line = line[n+len("[FATAL]"):]
			}
			if len(line) > 320 {
				line = line[:320] + "..."
			}
			return strings.TrimSpace(line)
		}
	}
	return "See container logs for the specific extraction error."
}

func (s *EdgeServer) handlePlanningAssessment(w http.ResponseWriter, r *http.Request) {
	po := r.URL.Query().Get("po")
	wb := s.loadCumulativeWorkbook()
	assessment := planning.AssessWorkbook(wb, po)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(assessment)
}

func (s *EdgeServer) handlePlanningQuickFix(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// A proposed worker substitution or new fabric receipt is not evidence that it happened.
	// In particular never invent rolls, mark absent workers present, or repair machines by changing a flag.
	http.Error(w, "Cannot automatically change physical inventory, attendance or machine status. Upload a verified update, then reassess.", http.StatusConflict)
}

func (s *EdgeServer) handlePlanningApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		PONumber   string `json:"po_number"`
		LineID     string `json:"line_id"`
		OrderedQty int    `json:"ordered_qty"`
		PIN        string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid plan", http.StatusBadRequest)
		return
	}
	pin := os.Getenv("MANAGER_PIN")
	if len(pin) != 4 {
		http.Error(w, "Set a 4-digit MANAGER_PIN secret on the server before approving plans", http.StatusServiceUnavailable)
		return
	}
	if len(req.PIN) != 4 || subtle.ConstantTimeCompare([]byte(req.PIN), []byte(pin)) != 1 {
		http.Error(w, "Invalid manager PIN", http.StatusUnauthorized)
		return
	}
	if s.db == nil {
		http.Error(w, "Database required to lock plans", http.StatusServiceUnavailable)
		return
	}
	wb := s.loadCumulativeWorkbook()
	assessment := planning.AssessWorkbook(wb, req.PONumber)
	if assessment.PONumber == "" || assessment.OverallStatus != planning.LightGreen {
		http.Error(w, "Plan is not ready; fix the highlighted problems", http.StatusConflict)
		return
	}
	if b, err := os.ReadFile("./data/staging/output/conflicts.json"); err == nil {
		var conflicts []json.RawMessage
		if json.Unmarshal(b, &conflicts) != nil || len(conflicts) > 0 {
			http.Error(w, "Review and resolve document conflicts before locking the plan", http.StatusConflict)
			return
		}
	}
	if req.OrderedQty <= 0 || req.OrderedQty != assessment.OrderedQty || strings.TrimSpace(req.PONumber) == "" {
		http.Error(w, "Plan quantity differs from intake; reload the assessment", http.StatusConflict)
		return
	}
	if req.LineID == "" {
		http.Error(w, "Line must be selected", http.StatusBadRequest)
		return
	}
	cutTarget, err := cutting.CalculateCutTarget(req.PONumber, "ALL", req.OrderedQty)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id',$1,true)", wb.TenantID); err != nil {
		http.Error(w, "Tenant scope failed", http.StatusInternalServerError)
		return
	}
	result, err := tx.ExecContext(ctx, `UPDATE dynamic_work_orders SET status='LOCKED', line_id=$1 WHERE tenant_id=$2 AND po_number=$3 AND ordered_qty=$4 AND status IN ('PROPOSED','PLANNED')`, req.LineID, wb.TenantID, req.PONumber, req.OrderedQty)
	if err != nil {
		http.Error(w, "Database plan-lock failed", http.StatusInternalServerError)
		return
	}
	affected, err := result.RowsAffected()
	if err != nil || affected != 1 {
		http.Error(w, "Expected one proposed plan to lock; none or several matched", http.StatusConflict)
		return
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Plan transaction failed", http.StatusInternalServerError)
		return
	}
	ev, ledgerErr := s.ledger.AppendEvent("PLANNING_DESK", "MANAGER_PIN", "PLAN_LOCKED", fmt.Sprintf(`{"po":%q,"cut_qty":%d}`, req.PONumber, cutTarget.TargetCutVolume))
	if ledgerErr != nil {
		log.Printf("[ERROR] Plan locked in DB, ledger event failed: %v", ledgerErr)
	} else if evBytes, err := json.Marshal(ev); err == nil {
		if _, err = s.wal.Append("LEDGER_EVENT", evBytes); err != nil {
			log.Printf("[ERROR] Ledger WAL write after DB lock: %v", err)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "LOCKED", "po_number": req.PONumber, "line_id": req.LineID, "target_cut_qty": cutTarget.TargetCutVolume, "audit_recorded": ledgerErr == nil})
}

func (s *EdgeServer) handlePlanningEmail(w http.ResponseWriter, r *http.Request) {
	// Never claim SENT before a real SMTP provider accepted the plan packet.
	http.Error(w, "Email dispatch is not configured; print the locked plan instead", http.StatusNotImplemented)
}

func (s *EdgeServer) handleCuttingOptimizeGeometry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		OrderID        string               `json:"order_id"`
		StyleCode      string               `json:"style_code"`
		RollLengthM    float64              `json:"roll_length_m"`
		UsableWidthCm  float64              `json:"usable_width_cm"`
		GarmentsPerPly int                  `json:"garments_per_ply"`
		Defects        []cutting.DefectSpot `json:"defects"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	plan := cutting.Optimize2DMarkerAndVision(
		req.OrderID, req.StyleCode, req.RollLengthM, req.UsableWidthCm, req.GarmentsPerPly, req.Defects,
	)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(plan)
}

// Author: Sunil Jain, CyberinQ LLC
func (s *EdgeServer) handleResolveIssue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.db == nil {
		http.Error(w, "Database not configured", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		IssueID string `json:"issue_id"`
		Action  string `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.IssueID) == "" {
		http.Error(w, "Invalid issue resolution payload", http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	s.cfgMu.Lock()
	tenantID, _ := s.activeConfig["tenant_id"].(string)
	s.cfgMu.Unlock()
	if tenantID == "" {
		tenantID = "TENANT_DEFAULT"
	}
	if _, err := tx.ExecContext(ctx, "SELECT set_config('app.current_tenant_id', $1, true)", tenantID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	res, err := tx.ExecContext(ctx, `
		UPDATE floor_issues
		SET status = 'resolved', resolved_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND tenant_id = $2 AND status IN ('open', 'in_progress');
	`, req.IssueID, tenantID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		http.Error(w, "Issue not found or already resolved", http.StatusNotFound)
		return
	}
	if err := tx.Commit(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "RESOLVED", "issue_id": req.IssueID})
}
