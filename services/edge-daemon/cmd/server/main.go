package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/internal/attachment"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/cutting"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/crypto"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/ocr"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/packing"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/stores"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/wal"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/web"
	"github.com/cyberinq/agiparel/services/edge-daemon/internal/topology"
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
	// Pre-seed ground-truth Roll #7 (59m) reserved for Cut 1 of PO MK6574J9
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
	server := &EdgeServer{
		modules:        modules,
		ledger:         ledger,
		attachment:     attachment.NewAttachmentEngine(),
		stores:         storesEng,
		spatialTwin:    twin,
		wal:            localWAL,
		db:             db,
		secretKey:      secret,
		activePackings: make(map[int]*packing.CartonPackingSession),
		activeOCRSess:  make(map[string]*ocr.SignOffSession),
	}

	mux := http.NewServeMux()

	// Base kernel routes
	mux.HandleFunc("/healthz", server.handleHealth)
	mux.HandleFunc("/api/v1/modules", server.handleGetModules)
	mux.HandleFunc("/api/v1/ledger/append", server.handleLedgerAppend)
	mux.HandleFunc("/api/v1/ledger/latest", server.handleLedgerLatest)

	// mod_stores routes
	if modules.IsEnabled("mod_stores") {
		mux.HandleFunc("/api/v1/stores/indent-check", server.handleStoresIndentCheck)
		mux.HandleFunc("/api/v1/stores/issue-double-witness", server.handleStoresIssueDoubleWitness)
		log.Println("Mounted endpoints: mod_stores")
	}

	// mod_cutting routes
	if modules.IsEnabled("mod_cutting") {
		mux.HandleFunc("/api/v1/cutting/plan-target", server.handleCuttingPlanTarget)
		mux.HandleFunc("/api/v1/cutting/lay-roll", server.handleCuttingLayRoll)
		log.Println("Mounted endpoints: mod_cutting")
	}

	// mod_sewing routes
	if modules.IsEnabled("mod_sewing") {
		mux.HandleFunc("/api/v1/sewing/bind-tag", server.handleSewingBindTag)
		mux.HandleFunc("/api/v1/sewing/validate-pair", server.handleSewingValidatePair)
		mux.HandleFunc("/api/v1/sewing/register-rule", server.handleSewingRegisterRule)
		log.Println("Mounted endpoints: mod_sewing")
	}

	// mod_packing routes
	if modules.IsEnabled("mod_packing") {
		mux.HandleFunc("/api/v1/packing/session/start", server.handlePackingStart)
		mux.HandleFunc("/api/v1/packing/session/scan", server.handlePackingScan)
		mux.HandleFunc("/api/v1/packing/session/seal", server.handlePackingSeal)
		log.Println("Mounted endpoints: mod_packing")
	}

	// mod_ocr routes
	if modules.IsEnabled("mod_ocr") {
		mux.HandleFunc("/api/v1/ocr/reconcile", server.handleOCRReconcile)
		mux.HandleFunc("/api/v1/ocr/sign", server.handleOCRSign)
		mux.HandleFunc("/api/v1/ocr/seal", server.handleOCRSeal)
		log.Println("Mounted endpoints: mod_ocr")
	}

	// Spatial Factory Twin View
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
	log.Println("Mounted endpoints: /plant and /api/v1/topology/devices")



	// Static HMI Files (Compiled into binary via go:embed)
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
	log.Println("Mounted endpoints: /sew01 and /static/ (embedded)")

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
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
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Forced shutdown: %v", err)
	}
	log.Println("plant-edge-os cleanly stopped.")
}

func (s *EdgeServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
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
	_ = s.wal.Append(wal.Record{
		Index:     event.Index,
		Timestamp: event.Timestamp,
		Type:      "LEDGER_EVENT",
		Payload:   evBytes,
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(event)
}

func (s *EdgeServer) handleLedgerLatest(w http.ResponseWriter, r *http.Request) {
	hash := s.ledger.GetLatestHash()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"latest_hash": hash,
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	record, err := s.stores.DoubleWitnessIssue(
		req.RollID, req.OrderID, req.CutNo, req.IssuedLengthM, req.IssuerID, req.RecipientID, s.secretKey,
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
	StyleID      string `json:"style_id"`
	SourcePartID int    `json:"source_part_id"`
	TargetPartID int    `json:"target_part_id"`
}

func (s *EdgeServer) handleSewingRegisterRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req RegisterRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.attachment.RegisterRule(req.StyleID, req.SourcePartID, req.TargetPartID)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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