// Author: Sunil Jain, CyberinQ LLC
// v51: 4-Tier PO -> Styles -> Tech Pack & Operation Bulletin Hierarchy,
// Cross-Chapter PO Filtering, PO-Independent Store RFID Pool, and 17-Stage Factory Experiment.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/rfid"
)

var (
	v51EPCRE  = regexp.MustCompile(`^[0-9A-F]{24,96}$`)
	v51CodeRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./:-]{0,63}$`)
)

func registerRFIDAndDossierRoutesV51(mux *http.ServeMux, s *EdgeServer) {
	mux.HandleFunc("/api/v1/document-dossiers", s.handleDocumentDossiersV51)
	mux.HandleFunc("/api/v1/rfid/tag-pool", s.handleRFIDTagPoolV51)
	mux.HandleFunc("/api/v1/stores/roll-sheet", s.handleSupplierRollSheetV51)
	mux.HandleFunc("/api/v1/rfid/rtls-sightings", s.handleRTLSightingsV51)
	mux.HandleFunc("/api/v1/rfid/experiment/run", s.handleRFIDExperimentRunV51)
}

func (s *EdgeServer) ensureV51Schema(ctx context.Context, tx *sql.Tx) error {
	var hasStyleCol bool
	_ = tx.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema='public' AND table_name='rfid_live_tag_pool_v51' AND column_name='bulletin_code'
		)`).Scan(&hasStyleCol)
	if hasStyleCol {
		return nil
	}
	ddl := `
	DROP TABLE IF EXISTS public.document_po_dossier_links_v51 CASCADE;
	DROP TABLE IF EXISTS public.document_po_dossiers_v51 CASCADE;
	DROP TABLE IF EXISTS public.store_supplier_roll_sheets_v51 CASCADE;
	DROP TABLE IF EXISTS public.rfid_live_tag_pool_v51 CASCADE;
	DROP TABLE IF EXISTS public.rfid_live_tag_events_v51 CASCADE;
	DROP TABLE IF EXISTS public.rfid_rtls_sightings_v51 CASCADE;

	CREATE TABLE public.document_po_dossiers_v51 (
		id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
		tenant_id varchar(64) NOT NULL,
		po_number varchar(64) NOT NULL,
		style_code varchar(64) NOT NULL DEFAULT 'DEFAULT_STYLE',
		buyer_name varchar(128) NOT NULL DEFAULT '',
		anchor_document_id varchar(36),
		locked_machines jsonb NOT NULL DEFAULT '[]'::jsonb,
		locked_workers jsonb NOT NULL DEFAULT '[]'::jsonb,
		sub_assemblies jsonb NOT NULL DEFAULT '[]'::jsonb,
		rfid_stage_budget jsonb NOT NULL DEFAULT '{}'::jsonb,
		lock_days integer NOT NULL DEFAULT 0,
		notes text NOT NULL DEFAULT '',
		created_by varchar(64) NOT NULL DEFAULT 'dev',
		created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
		updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
		UNIQUE (tenant_id, po_number, style_code),
		UNIQUE (tenant_id, id)
	);
	CREATE TABLE public.document_po_dossier_links_v51 (
		id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
		tenant_id varchar(64) NOT NULL,
		dossier_id uuid NOT NULL,
		document_id varchar(36) NOT NULL,
		doc_role varchar(40) NOT NULL,
		sub_assembly varchar(128) NOT NULL DEFAULT '',
		linked_by varchar(64) NOT NULL DEFAULT 'dev',
		linked_at timestamptz NOT NULL DEFAULT clock_timestamp(),
		UNIQUE (tenant_id, dossier_id, document_id, doc_role),
		FOREIGN KEY (tenant_id, dossier_id) REFERENCES public.document_po_dossiers_v51(tenant_id, id) ON DELETE CASCADE
	);
	CREATE TABLE public.store_supplier_roll_sheets_v51 (
		id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
		tenant_id varchar(64) NOT NULL,
		plant_id varchar(64) NOT NULL DEFAULT 'PLANT_DEFAULT',
		po_number varchar(64) NOT NULL DEFAULT 'GENERAL_STOCK',
		allocated_po_number varchar(64) NOT NULL DEFAULT '',
		allocated_style_code varchar(64) NOT NULL DEFAULT '',
		allocated_bulletin_id varchar(64) NOT NULL DEFAULT '',
		serial_no varchar(64) NOT NULL,
		supplier_code varchar(64) NOT NULL,
		material_sku varchar(128) NOT NULL,
		fabric_description varchar(255) NOT NULL DEFAULT '',
		content varchar(255) NOT NULL DEFAULT '',
		gsm integer NOT NULL DEFAULT 0,
		construction varchar(128) NOT NULL DEFAULT '',
		color_code varchar(64) NOT NULL,
		declared_width_mm integer NOT NULL,
		declared_length_mm bigint NOT NULL,
		shade_band varchar(32) NOT NULL,
		actual_width_mm integer,
		actual_length_mm bigint,
		defect_points integer NOT NULL DEFAULT 0,
		inspection_status varchar(16) NOT NULL DEFAULT 'PENDING',
		mapped_epc_hex varchar(96),
		grn_number varchar(64),
		grn_posted_at timestamptz,
		zone_location varchar(64) NOT NULL DEFAULT 'RECEIVING_DOCK',
		balance_length_mm bigint NOT NULL,
		source_document_id varchar(36),
		recorded_at timestamptz NOT NULL DEFAULT clock_timestamp(),
		UNIQUE (tenant_id, serial_no)
	);
	CREATE TABLE public.rfid_live_tag_pool_v51 (
		tenant_id varchar(64) NOT NULL,
		epc_hex varchar(96) NOT NULL,
		nfc_uid varchar(64) NOT NULL DEFAULT '',
		human_code varchar(64) NOT NULL,
		tag_class varchar(24) NOT NULL DEFAULT 'DUAL_UHF_NFC',
		status varchar(24) NOT NULL DEFAULT 'UNUSED',
		battery_mv integer NOT NULL DEFAULT 3000,
		target_type varchar(24),
		target_id varchar(128),
		po_number varchar(64) NOT NULL DEFAULT '',
		style_code varchar(64) NOT NULL DEFAULT '',
		bulletin_code varchar(64) NOT NULL DEFAULT '',
		op_stage varchar(64) NOT NULL DEFAULT '',
		current_zone varchar(64) NOT NULL DEFAULT 'TAG_STOCK_BIN',
		current_station varchar(64) NOT NULL DEFAULT '',
		reuse_count integer NOT NULL DEFAULT 0,
		last_seen_at timestamptz NOT NULL DEFAULT clock_timestamp(),
		updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),
		PRIMARY KEY (tenant_id, epc_hex),
		UNIQUE (tenant_id, human_code)
	);
	CREATE TABLE public.rfid_live_tag_events_v51 (
		id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
		tenant_id varchar(64) NOT NULL,
		plant_id varchar(64) NOT NULL DEFAULT 'PLANT_DEFAULT',
		epc_hex varchar(96) NOT NULL,
		action varchar(24) NOT NULL,
		stage_no integer NOT NULL DEFAULT 0,
		stage_name varchar(64) NOT NULL DEFAULT '',
		target_type varchar(24) NOT NULL DEFAULT '',
		target_id varchar(128) NOT NULL DEFAULT '',
		po_number varchar(64) NOT NULL DEFAULT '',
		style_code varchar(64) NOT NULL DEFAULT '',
		bulletin_code varchar(64) NOT NULL DEFAULT '',
		gateway_id varchar(64) NOT NULL DEFAULT '',
		station_id varchar(64) NOT NULL DEFAULT '',
		zone_code varchar(64) NOT NULL DEFAULT '',
		rssi_dbm integer,
		actor_subject varchar(128) NOT NULL DEFAULT 'system',
		detail jsonb NOT NULL DEFAULT '{}'::jsonb,
		recorded_at timestamptz NOT NULL DEFAULT clock_timestamp()
	);
	CREATE TABLE public.rfid_rtls_sightings_v51 (
		id uuid PRIMARY KEY DEFAULT public.uuid_generate_v4(),
		tenant_id varchar(64) NOT NULL,
		plant_id varchar(64) NOT NULL DEFAULT 'PLANT_DEFAULT',
		gateway_id varchar(64) NOT NULL,
		reader_mode varchar(16) NOT NULL DEFAULT 'UHF_REMOTE',
		epc_hex varchar(96) NOT NULL,
		zone_code varchar(64) NOT NULL,
		rack_bin varchar(64) NOT NULL DEFAULT '',
		station_id varchar(64) NOT NULL DEFAULT '',
		rssi_dbm integer NOT NULL DEFAULT -55,
		battery_mv integer,
		observed_at timestamptz NOT NULL DEFAULT clock_timestamp()
	);`
	_, err := tx.ExecContext(ctx, ddl)
	return err
}

type poDossierLinkDTO struct {
	DocumentID  string    `json:"document_id"`
	Filename    string    `json:"filename"`
	Format      string    `json:"format"`
	Status      string    `json:"status"`
	DocRole     string    `json:"doc_role"`
	SubAssembly string    `json:"sub_assembly,omitempty"`
	LinkedAt    time.Time `json:"linked_at"`
}

type poStyleDossierDTO struct {
	ID               string             `json:"id"`
	PONumber         string             `json:"po_number"`
	StyleCode        string             `json:"style_code"`
	BuyerName        string             `json:"buyer_name"`
	AnchorDocumentID string             `json:"anchor_document_id,omitempty"`
	LockedMachines   []string           `json:"locked_machines"`
	LockedWorkers    []string           `json:"locked_workers"`
	SubAssemblies    []string           `json:"sub_assemblies"`
	RFIDStageBudget  map[string]int     `json:"rfid_stage_budget"`
	LockDays         int                `json:"lock_days"`
	Notes            string             `json:"notes"`
	UpdatedAt        time.Time          `json:"updated_at"`
	Documents        []poDossierLinkDTO `json:"documents"`
}

func (s *EdgeServer) handleDocumentDossiersV51(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Development tenant or PostgreSQL unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()
	if err = s.ensureV51Schema(ctx, tx); err != nil {
		http.Error(w, "Schema initialization failed: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		poFilter := strings.TrimSpace(r.URL.Query().Get("po_number"))
		styleFilter := strings.TrimSpace(r.URL.Query().Get("style_code"))
		rows, err := tx.QueryContext(ctx, `
			SELECT id::text, po_number, style_code, buyer_name, COALESCE(anchor_document_id,''),
			       locked_machines, locked_workers, sub_assemblies, rfid_stage_budget, lock_days, notes, updated_at
			FROM document_po_dossiers_v51
			WHERE tenant_id=$1 AND ($2='' OR po_number=$2) AND ($3='' OR style_code=$3)
			ORDER BY po_number ASC, style_code ASC LIMIT 150`, tenant, poFilter, styleFilter)
		if err != nil {
			http.Error(w, "Cannot read PO dossiers", http.StatusServiceUnavailable)
			return
		}
		dossiers := []poStyleDossierDTO{}
		byID := map[string]*poStyleDossierDTO{}
		for rows.Next() {
			var d poStyleDossierDTO
			var rawMach, rawWork, rawSub, rawBudget []byte
			if err = rows.Scan(&d.ID, &d.PONumber, &d.StyleCode, &d.BuyerName, &d.AnchorDocumentID,
				&rawMach, &rawWork, &rawSub, &rawBudget, &d.LockDays, &d.Notes, &d.UpdatedAt); err != nil {
				rows.Close()
				http.Error(w, "Cannot scan PO dossiers", http.StatusServiceUnavailable)
				return
			}
			_ = json.Unmarshal(rawMach, &d.LockedMachines)
			_ = json.Unmarshal(rawWork, &d.LockedWorkers)
			_ = json.Unmarshal(rawSub, &d.SubAssemblies)
			_ = json.Unmarshal(rawBudget, &d.RFIDStageBudget)
			if d.LockedMachines == nil {
				d.LockedMachines = []string{}
			}
			if d.LockedWorkers == nil {
				d.LockedWorkers = []string{}
			}
			if d.SubAssemblies == nil {
				d.SubAssemblies = []string{}
			}
			if d.RFIDStageBudget == nil {
				d.RFIDStageBudget = map[string]int{}
			}
			d.Documents = []poDossierLinkDTO{}
			dossiers = append(dossiers, d)
		}
		rows.Close()
		for i := range dossiers {
			byID[dossiers[i].ID] = &dossiers[i]
		}
		linkRows, err := tx.QueryContext(ctx, `
			SELECT l.dossier_id::text, l.document_id, COALESCE(s.filename, l.document_id), COALESCE(s.format,''), COALESCE(j.status,'EXTRACTED'), l.doc_role, l.sub_assembly, l.linked_at
			FROM document_po_dossier_links_v51 l
			LEFT JOIN document_sources_v65 s ON s.tenant_id=l.tenant_id AND s.id=l.document_id
			LEFT JOIN document_jobs_v65 j ON j.tenant_id=l.tenant_id AND j.document_id=l.document_id
			WHERE l.tenant_id=$1
			ORDER BY CASE l.doc_role
				WHEN 'ANCHOR_PO' THEN 1
				WHEN 'TECH_PACK' THEN 2
				WHEN 'OPERATION_BULLETIN' THEN 3
				WHEN 'MACHINE_ROSTER' THEN 4
				WHEN 'WORKER_SKILL_MATRIX' THEN 5
				WHEN 'MATERIAL_ROLL_SHEET' THEN 6
				WHEN 'PACKING_SHIPPING_BULLETIN' THEN 7
				ELSE 8 END, l.linked_at ASC`, tenant)
		if err == nil {
			for linkRows.Next() {
				var dID string
				var lk poDossierLinkDTO
				if linkRows.Scan(&dID, &lk.DocumentID, &lk.Filename, &lk.Format, &lk.Status, &lk.DocRole, &lk.SubAssembly, &lk.LinkedAt) == nil {
					if parent := byID[dID]; parent != nil {
						parent.Documents = append(parent.Documents, lk)
					}
				}
			}
			linkRows.Close()
		}
		_ = tx.Commit()
		docJSON(w, http.StatusOK, map[string]any{"dossiers": dossiers})
		return
	}

	var req struct {
		PONumber        string         `json:"po_number"`
		StyleCode       string         `json:"style_code"`
		BuyerName       string         `json:"buyer_name"`
		LockedMachines  []string       `json:"locked_machines"`
		LockedWorkers   []string       `json:"locked_workers"`
		SubAssemblies   []string       `json:"sub_assemblies"`
		RFIDStageBudget map[string]int `json:"rfid_stage_budget"`
		LockDays        int            `json:"lock_days"`
		Notes           string         `json:"notes"`
		DocumentIDs     []struct {
			DocumentID  string `json:"document_id"`
			DocRole     string `json:"doc_role"`
			SubAssembly string `json:"sub_assembly"`
		} `json:"documents"`
	}
	if !candidateJSONBody(w, r, 65536, &req) {
		return
	}
	poNum := strings.TrimSpace(req.PONumber)
	styleCode := nonEmptyOr(strings.TrimSpace(req.StyleCode), "DEFAULT_STYLE")
	if !v51CodeRE.MatchString(poNum) || !v51CodeRE.MatchString(styleCode) {
		http.Error(w, "Enter valid PO number and Style code (1–64 alphanumeric/dash characters)", http.StatusBadRequest)
		return
	}
	var anchorDocID *string
	for _, item := range req.DocumentIDs {
		if item.DocRole == "ANCHOR_PO" && validDocSourceID(item.DocumentID) {
			id := item.DocumentID
			anchorDocID = &id
			break
		}
	}
	if req.RFIDStageBudget == nil || len(req.RFIDStageBudget) == 0 {
		req.RFIDStageBudget = map[string]int{
			"store_roll_live_tags":      10,
			"cutting_bundle_dual_fobs":  24,
			"sewing_station_nfc_pucks":  14,
			"trolley_machine_rtls_tags": 4,
			"garment_sgtin96_inlays":    1200,
			"carton_sscc96_labels":      50,
		}
	}
	bMach, _ := json.Marshal(req.LockedMachines)
	bWork, _ := json.Marshal(req.LockedWorkers)
	bSub, _ := json.Marshal(req.SubAssemblies)
	bRfid, _ := json.Marshal(req.RFIDStageBudget)
	if req.LockDays <= 0 {
		req.LockDays = 14
	}

	var dossierID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO document_po_dossiers_v51 (
			tenant_id, po_number, style_code, buyer_name, anchor_document_id,
			locked_machines, locked_workers, sub_assemblies, rfid_stage_budget, lock_days, notes, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, $8::jsonb, $9::jsonb, $10, $11, clock_timestamp())
		ON CONFLICT (tenant_id, po_number, style_code) DO UPDATE SET
			buyer_name = CASE WHEN EXCLUDED.buyer_name <> '' THEN EXCLUDED.buyer_name ELSE document_po_dossiers_v51.buyer_name END,
			anchor_document_id = COALESCE(EXCLUDED.anchor_document_id, document_po_dossiers_v51.anchor_document_id),
			locked_machines = CASE WHEN jsonb_array_length(EXCLUDED.locked_machines) > 0 THEN EXCLUDED.locked_machines ELSE document_po_dossiers_v51.locked_machines END,
			locked_workers = CASE WHEN jsonb_array_length(EXCLUDED.locked_workers) > 0 THEN EXCLUDED.locked_workers ELSE document_po_dossiers_v51.locked_workers END,
			sub_assemblies = CASE WHEN jsonb_array_length(EXCLUDED.sub_assemblies) > 0 THEN EXCLUDED.sub_assemblies ELSE document_po_dossiers_v51.sub_assemblies END,
			rfid_stage_budget = EXCLUDED.rfid_stage_budget,
			lock_days = EXCLUDED.lock_days,
			notes = CASE WHEN EXCLUDED.notes <> '' THEN EXCLUDED.notes ELSE document_po_dossiers_v51.notes END,
			updated_at = clock_timestamp()
		RETURNING id::text`,
		tenant, poNum, styleCode, strings.TrimSpace(req.BuyerName), anchorDocID,
		string(bMach), string(bWork), string(bSub), string(bRfid), req.LockDays, strings.TrimSpace(req.Notes)).Scan(&dossierID)
	if err != nil {
		http.Error(w, "Failed to save PO+Style dossier: "+err.Error(), http.StatusServiceUnavailable)
		return
	}
	allowedRoles := map[string]bool{
		"ANCHOR_PO": true, "TECH_PACK": true, "OPERATION_BULLETIN": true,
		"MACHINE_ROSTER": true, "WORKER_SKILL_MATRIX": true, "MATERIAL_ROLL_SHEET": true,
		"PACKING_SHIPPING_BULLETIN": true, "SUB_ASSEMBLY_SPEC": true,
	}
	linked := 0
	for _, item := range req.DocumentIDs {
		role := strings.ToUpper(strings.TrimSpace(item.DocRole))
		if !validDocSourceID(item.DocumentID) || !allowedRoles[role] {
			continue
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO document_po_dossier_links_v51 (tenant_id, dossier_id, document_id, doc_role, sub_assembly)
			VALUES ($1, $2::uuid, $3, $4, $5)
			ON CONFLICT (tenant_id, dossier_id, document_id, doc_role) DO UPDATE SET
				sub_assembly = EXCLUDED.sub_assembly,
				linked_at = clock_timestamp()`,
			tenant, dossierID, item.DocumentID, role, strings.TrimSpace(item.SubAssembly)); err == nil {
			linked++
		}
	}
	if err = tx.Commit(); err != nil {
		http.Error(w, "Database commit failed", http.StatusServiceUnavailable)
		return
	}
	docJSON(w, http.StatusOK, map[string]any{"dossier_id": dossierID, "po_number": poNum, "style_code": styleCode, "linked_documents": linked})
}

func (s *EdgeServer) handleRFIDTagPoolV51(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Development tenant or PostgreSQL unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()
	if err = s.ensureV51Schema(ctx, tx); err != nil {
		http.Error(w, "Schema initialization failed", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		poFilter := strings.TrimSpace(r.URL.Query().Get("po_number"))
		styleFilter := strings.TrimSpace(r.URL.Query().Get("style_code"))
		rows, err := tx.QueryContext(ctx, `
			SELECT epc_hex, nfc_uid, human_code, tag_class, status, battery_mv,
			       COALESCE(target_type,''), COALESCE(target_id,''), po_number, style_code, bulletin_code, op_stage,
			       current_zone, current_station, reuse_count, last_seen_at
			FROM rfid_live_tag_pool_v51
			WHERE tenant_id=$1
			  AND ($2='' OR po_number=$2 OR po_number='GENERAL_STOCK' OR po_number='')
			  AND ($3='' OR style_code=$3 OR style_code='')
			ORDER BY status ASC, human_code ASC LIMIT 300`, tenant, poFilter, styleFilter)
		if err != nil {
			http.Error(w, "Cannot query RFID tag pool", http.StatusServiceUnavailable)
			return
		}
		defer rows.Close()
		tags := []map[string]any{}
		counts := map[string]int{"UNUSED": 0, "MAPPED": 0, "RELEASED": 0, "FAULTY_LOW_BATTERY": 0}
		for rows.Next() {
			var epc, nfc, code, tClass, status, tType, tID, po, st, bul, stg, zone, station string
			var batt, reuse int
			var seen time.Time
			if rows.Scan(&epc, &nfc, &code, &tClass, &status, &batt, &tType, &tID, &po, &st, &bul, &stg, &zone, &station, &reuse, &seen) == nil {
				counts[status]++
				tags = append(tags, map[string]any{
					"epc_hex": epc, "nfc_uid": nfc, "human_code": code, "tag_class": tClass,
					"status": status, "battery_mv": batt, "target_type": tType, "target_id": tID,
					"po_number": po, "style_code": st, "bulletin_code": bul, "op_stage": stg,
					"current_zone": zone, "current_station": station,
					"reuse_count": reuse, "last_seen_at": seen,
				})
			}
		}
		_ = tx.Commit()
		docJSON(w, http.StatusOK, map[string]any{"counts": counts, "tags": tags})
		return
	}

	var req struct {
		Action       string `json:"action"` // ENROLL, MAP, ALLOCATE_PO, RELEASE, RETURN_TO_UNUSED, MARK_FAULTY
		EPCHex       string `json:"epc_hex"`
		NFCUID       string `json:"nfc_uid"`
		HumanCode    string `json:"human_code"`
		TagClass     string `json:"tag_class"`
		TargetType   string `json:"target_type"`
		TargetID     string `json:"target_id"`
		PONumber     string `json:"po_number"`
		StyleCode    string `json:"style_code"`
		BulletinCode string `json:"bulletin_code"`
		OpStage      string `json:"op_stage"`
		ZoneCode     string `json:"zone_code"`
		StationID    string `json:"station_id"`
		BatteryMV    int    `json:"battery_mv"`
	}
	if !candidateJSONBody(w, r, 32768, &req) {
		return
	}
	epc := strings.ToUpper(strings.TrimSpace(req.EPCHex))
	if !v51EPCRE.MatchString(epc) || len(epc)%2 != 0 {
		http.Error(w, "epc_hex must be 24–96 even-length uppercase hex characters", http.StatusBadRequest)
		return
	}
	if req.BatteryMV <= 0 {
		req.BatteryMV = 3050
	}
	action := strings.ToUpper(strings.TrimSpace(req.Action))
	switch action {
	case "ENROLL":
		if req.TagClass == "" {
			req.TagClass = "DUAL_UHF_NFC"
		}
		if req.HumanCode == "" {
			req.HumanCode = "FOB-" + epc[len(epc)-6:]
		}
		_, err = tx.ExecContext(ctx, `
			INSERT INTO rfid_live_tag_pool_v51 (tenant_id, epc_hex, nfc_uid, human_code, tag_class, status, battery_mv, current_zone)
			VALUES ($1, $2, $3, $4, $5, 'UNUSED', $6, 'TAG_STOCK_BIN')
			ON CONFLICT (tenant_id, epc_hex) DO UPDATE SET
				nfc_uid = EXCLUDED.nfc_uid,
				battery_mv = EXCLUDED.battery_mv,
				updated_at = clock_timestamp()`,
			tenant, epc, strings.ToUpper(strings.TrimSpace(req.NFCUID)), req.HumanCode, req.TagClass, req.BatteryMV)
	case "MAP":
		var curStatus string
		err = tx.QueryRowContext(ctx, `SELECT status FROM rfid_live_tag_pool_v51 WHERE tenant_id=$1 AND epc_hex=$2 FOR UPDATE`, tenant, epc).Scan(&curStatus)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Tag not enrolled in pool", http.StatusNotFound)
			return
		}
		if curStatus != "UNUSED" {
			http.Error(w, fmt.Sprintf("Tag %s is currently %s; only UNUSED tags can be mapped", epc, curStatus), http.StatusConflict)
			return
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE rfid_live_tag_pool_v51
			SET status='MAPPED', target_type=$3, target_id=$4, po_number=$5, style_code=$6, bulletin_code=$7, op_stage=$8,
			    current_zone=$9, current_station=$10, updated_at=clock_timestamp(), last_seen_at=clock_timestamp()
			WHERE tenant_id=$1 AND epc_hex=$2`,
			tenant, epc, req.TargetType, req.TargetID, nonEmptyOr(req.PONumber, "GENERAL_STOCK"),
			req.StyleCode, req.BulletinCode, req.OpStage, nonEmptyOr(req.ZoneCode, "INSPECTION_MAPPING"), req.StationID)
	case "ALLOCATE_PO":
		// Re-assigns an already mapped General Stock roll tag to a specific PO, Style, and Operation Bulletin.
		_, err = tx.ExecContext(ctx, `
			UPDATE rfid_live_tag_pool_v51
			SET po_number=$3, style_code=$4, bulletin_code=$5, op_stage=$6, updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND epc_hex=$2`,
			tenant, epc, req.PONumber, req.StyleCode, req.BulletinCode, req.OpStage)
		_, _ = tx.ExecContext(ctx, `
			UPDATE store_supplier_roll_sheets_v51
			SET allocated_po_number=$3, allocated_style_code=$4, allocated_bulletin_id=$5
			WHERE tenant_id=$1 AND mapped_epc_hex=$2`,
			tenant, epc, req.PONumber, req.StyleCode, req.BulletinCode)
	case "RELEASE", "RETURN_TO_UNUSED":
		nextStatus := "RELEASED"
		nextZone := "CUTTING_RETURN_TRAY"
		if action == "RETURN_TO_UNUSED" {
			nextStatus = "UNUSED"
			nextZone = "TAG_STOCK_BIN"
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE rfid_live_tag_pool_v51
			SET status=$3,
			    target_type = CASE WHEN $3='UNUSED' THEN NULL ELSE target_type END,
			    target_id   = CASE WHEN $3='UNUSED' THEN NULL ELSE target_id END,
			    po_number   = CASE WHEN $3='UNUSED' THEN '' ELSE po_number END,
			    style_code  = CASE WHEN $3='UNUSED' THEN '' ELSE style_code END,
			    bulletin_code = CASE WHEN $3='UNUSED' THEN '' ELSE bulletin_code END,
			    op_stage    = CASE WHEN $3='UNUSED' THEN '' ELSE op_stage END,
			    current_zone=$4,
			    reuse_count = CASE WHEN $3='UNUSED' THEN reuse_count + 1 ELSE reuse_count END,
			    updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND epc_hex=$2`, tenant, epc, nextStatus, nextZone)
	case "MARK_FAULTY":
		_, err = tx.ExecContext(ctx, `
			UPDATE rfid_live_tag_pool_v51
			SET status='FAULTY_LOW_BATTERY', battery_mv=$3, current_zone='QUARANTINE_TRAY', updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND epc_hex=$2`, tenant, epc, req.BatteryMV)
	default:
		http.Error(w, "Unsupported action", http.StatusBadRequest)
		return
	}
	if err != nil {
		http.Error(w, "Tag pool update failed: "+err.Error(), http.StatusConflict)
		return
	}
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO rfid_live_tag_events_v51 (tenant_id, epc_hex, action, target_type, target_id, po_number, style_code, bulletin_code, zone_code, station_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		tenant, epc, action, req.TargetType, req.TargetID, req.PONumber, req.StyleCode, req.BulletinCode, req.ZoneCode, req.StationID)
	_ = tx.Commit()
	docJSON(w, http.StatusOK, map[string]any{"epc_hex": epc, "action": action, "status": "OK"})
}

func (s *EdgeServer) handleSupplierRollSheetV51(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Development tenant or PostgreSQL unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()
	if err = s.ensureV51Schema(ctx, tx); err != nil {
		http.Error(w, "Schema initialization failed", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		po := strings.TrimSpace(r.URL.Query().Get("po_number"))
		rows, err := tx.QueryContext(ctx, `
			SELECT po_number, allocated_po_number, allocated_style_code, allocated_bulletin_id,
			       serial_no, supplier_code, material_sku, fabric_description, content, gsm, construction,
			       color_code, declared_width_mm, declared_length_mm, shade_band,
			       COALESCE(actual_width_mm,0), COALESCE(actual_length_mm,0), defect_points,
			       inspection_status, COALESCE(mapped_epc_hex,''), COALESCE(grn_number,''), zone_location, balance_length_mm
			FROM store_supplier_roll_sheets_v51
			WHERE tenant_id=$1 AND ($2='' OR po_number=$2 OR allocated_po_number=$2 OR po_number='GENERAL_STOCK')
			ORDER BY serial_no ASC LIMIT 300`, tenant, po)
		if err != nil {
			http.Error(w, "Cannot read supplier roll sheets", http.StatusServiceUnavailable)
			return
		}
		defer rows.Close()
		rolls := []map[string]any{}
		mappedCount, shortCount, claimCount := 0, 0, 0
		for rows.Next() {
			var poNum, allocPO, allocStyle, allocBul, serial, sup, sku, desc, content, constr, color, shade, status, epc, grn, zone string
			var gsm, decW, actW, pts int
			var decL, actL, balL int64
			if rows.Scan(&poNum, &allocPO, &allocStyle, &allocBul, &serial, &sup, &sku, &desc, &content, &gsm, &constr, &color, &decW, &decL, &shade, &actW, &actL, &pts, &status, &epc, &grn, &zone, &balL) == nil {
				if epc != "" {
					mappedCount++
				} else {
					shortCount++
				}
				if status == "CLAIM" {
					claimCount++
				}
				rolls = append(rolls, map[string]any{
					"po_number": poNum, "allocated_po_number": allocPO, "allocated_style_code": allocStyle, "allocated_bulletin_id": allocBul,
					"serial_no": serial, "supplier_code": sup, "material_sku": sku,
					"fabric_description": desc, "content": content, "gsm": gsm, "construction": constr,
					"color_code": color, "declared_width_mm": decW, "declared_length_mm": decL,
					"shade_band": shade, "actual_width_mm": actW, "actual_length_mm": actL,
					"defect_points": pts, "inspection_status": status, "mapped_epc_hex": epc,
					"grn_number": grn, "zone_location": zone, "balance_length_mm": balL,
				})
			}
		}
		_ = tx.Commit()
		docJSON(w, http.StatusOK, map[string]any{
			"rolls": rolls,
			"reconciliation": map[string]int{
				"sheet_total":   len(rolls),
				"mapped_grn":    mappedCount,
				"short_missing": shortCount,
				"claims":        claimCount,
			},
		})
		return
	}
	var req struct {
		PONumber string `json:"po_number"`
		Rolls    []struct {
			SerialNo          string `json:"serial_no"`
			SupplierCode      string `json:"supplier_code"`
			MaterialSKU       string `json:"material_sku"`
			FabricDescription string `json:"fabric_description"`
			Content           string `json:"content"`
			GSM               int    `json:"gsm"`
			Construction      string `json:"construction"`
			ColorCode         string `json:"color_code"`
			DeclaredWidthMM   int    `json:"declared_width_mm"`
			DeclaredLengthMM  int64  `json:"declared_length_mm"`
			ShadeBand         string `json:"shade_band"`
		} `json:"rolls"`
	}
	if !candidateJSONBody(w, r, 131072, &req) {
		return
	}
	imported := 0
	for _, rl := range req.Rolls {
		if rl.SerialNo == "" || rl.DeclaredLengthMM <= 0 || rl.DeclaredWidthMM < 100 {
			continue
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO store_supplier_roll_sheets_v51 (
				tenant_id, po_number, serial_no, supplier_code, material_sku, fabric_description,
				content, gsm, construction, color_code, declared_width_mm, declared_length_mm,
				shade_band, balance_length_mm
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$12)
			ON CONFLICT (tenant_id, serial_no) DO NOTHING`,
			tenant, nonEmptyOr(req.PONumber, "GENERAL_STOCK"), rl.SerialNo, rl.SupplierCode, rl.MaterialSKU, rl.FabricDescription,
			rl.Content, rl.GSM, rl.Construction, rl.ColorCode, rl.DeclaredWidthMM, rl.DeclaredLengthMM, rl.ShadeBand); err == nil {
			imported++
		}
	}
	_ = tx.Commit()
	docJSON(w, http.StatusCreated, map[string]any{"po_number": nonEmptyOr(req.PONumber, "GENERAL_STOCK"), "imported_rolls": imported})
}

func (s *EdgeServer) handleRTLSightingsV51(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Development tenant or PostgreSQL unavailable", http.StatusServiceUnavailable)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()
	if err = s.ensureV51Schema(ctx, tx); err != nil {
		http.Error(w, "Schema initialization failed", http.StatusServiceUnavailable)
		return
	}
	if r.Method == http.MethodGet {
		poFilter := strings.TrimSpace(r.URL.Query().Get("po_number"))
		styleFilter := strings.TrimSpace(r.URL.Query().Get("style_code"))
		rows, err := tx.QueryContext(ctx, `
			SELECT stage_no, stage_name, epc_hex, action, target_type, target_id, po_number, style_code, bulletin_code, gateway_id, station_id, zone_code, COALESCE(rssi_dbm,-50), recorded_at
			FROM rfid_live_tag_events_v51
			WHERE tenant_id=$1
			  AND ($2='' OR po_number=$2 OR po_number='GENERAL_STOCK')
			  AND ($3='' OR style_code=$3 OR style_code='')
			ORDER BY recorded_at DESC LIMIT 200`, tenant, poFilter, styleFilter)
		if err != nil {
			http.Error(w, "Cannot read RTLS telemetry", http.StatusServiceUnavailable)
			return
		}
		defer rows.Close()
		events := []map[string]any{}
		for rows.Next() {
			var stg, rssi int
			var stgName, epc, act, tType, tID, po, st, bul, gw, stn, zn string
			var rec time.Time
			if rows.Scan(&stg, &stgName, &epc, &act, &tType, &tID, &po, &st, &bul, &gw, &stn, &zn, &rssi, &rec) == nil {
				events = append(events, map[string]any{
					"stage_no": stg, "stage_name": stgName, "epc_hex": epc, "action": act,
					"target_type": tType, "target_id": tID, "po_number": po, "style_code": st, "bulletin_code": bul,
					"gateway_id": gw, "station_id": stn, "zone_code": zn, "rssi_dbm": rssi, "recorded_at": rec,
				})
			}
		}
		_ = tx.Commit()
		docJSON(w, http.StatusOK, map[string]any{"events": events})
		return
	}
	var req struct {
		GatewayID  string `json:"gateway_id"`
		ReaderMode string `json:"reader_mode"`
		EPCHex     string `json:"epc_hex"`
		ZoneCode   string `json:"zone_code"`
		RackBin    string `json:"rack_bin"`
		StationID  string `json:"station_id"`
		RSSIDBm    int    `json:"rssi_dbm"`
		BatteryMV  int    `json:"battery_mv"`
	}
	if !candidateJSONBody(w, r, 16384, &req) {
		return
	}
	epc := strings.ToUpper(strings.TrimSpace(req.EPCHex))
	if !v51EPCRE.MatchString(epc) {
		http.Error(w, "Invalid epc_hex", http.StatusBadRequest)
		return
	}
	_, _ = tx.ExecContext(ctx, `
		INSERT INTO rfid_rtls_sightings_v51 (tenant_id, gateway_id, reader_mode, epc_hex, zone_code, rack_bin, station_id, rssi_dbm, battery_mv)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NULLIF($9,0))`,
		tenant, nonEmptyOr(req.GatewayID, "GW-STORE-01"), nonEmptyOr(req.ReaderMode, "UHF_REMOTE"), epc, req.ZoneCode, req.RackBin, req.StationID, req.RSSIDBm, req.BatteryMV)
	_, _ = tx.ExecContext(ctx, `
		UPDATE rfid_live_tag_pool_v51
		SET current_zone=$3, current_station=$4, last_seen_at=clock_timestamp(), updated_at=clock_timestamp()
		WHERE tenant_id=$1 AND epc_hex=$2`, tenant, epc, req.ZoneCode, req.StationID)
	_, _ = tx.ExecContext(ctx, `
		UPDATE store_supplier_roll_sheets_v51
		SET zone_location=$3
		WHERE tenant_id=$1 AND mapped_epc_hex=$2`, tenant, epc, nonEmptyOr(req.RackBin, req.ZoneCode))
	_ = tx.Commit()
	docJSON(w, http.StatusCreated, map[string]any{"status": "RECORDED", "epc_hex": epc, "zone": req.ZoneCode, "rack_bin": req.RackBin})
}

func (s *EdgeServer) handleRFIDExperimentRunV51(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tenant, err := intakeTenant()
	if err != nil || s.db == nil {
		http.Error(w, "Development tenant or PostgreSQL unavailable", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		PONumber     string `json:"po_number"`
		StyleCode    string `json:"style_code"`
		BulletinCode string `json:"bulletin_code"`
		ColorCode    string `json:"color_code"`
		RollCount    int    `json:"roll_count"`
		BundleCount  int    `json:"bundle_count"`
		CartonCount  int    `json:"carton_count"`
	}
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768)).Decode(&req)
	poNum := nonEmptyOr(strings.TrimSpace(req.PONumber), "REP3")
	styleCode := nonEmptyOr(strings.TrimSpace(req.StyleCode), "GM09Y094")
	bulletinCode := nonEmptyOr(strings.TrimSpace(req.BulletinCode), "OB-"+styleCode+"-L14")
	colorCode := nonEmptyOr(strings.TrimSpace(req.ColorCode), "SOFT_SILVER")
	rollCount := clampInt(req.RollCount, 6, 20, 10)
	bundleCount := clampInt(req.BundleCount, 8, 40, 14)
	cartonCount := clampInt(req.CartonCount, 2, 12, 4)

	ctx, cancel := context.WithTimeout(r.Context(), 25*time.Second)
	defer cancel()
	tx, err := s.docTransaction(ctx, tenant)
	if err != nil {
		http.Error(w, "Database unavailable", http.StatusServiceUnavailable)
		return
	}
	defer tx.Rollback()
	if err = s.ensureV51Schema(ctx, tx); err != nil {
		http.Error(w, "Schema initialization failed", http.StatusServiceUnavailable)
		return
	}

	makeEPC := func(prefix string, seq int) string {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%s:%d", tenant, poNum, styleCode, prefix, seq)))
		return strings.ToUpper(prefix + hex.EncodeToString(h[:])[:24-len(prefix)])
	}
	recordStage := func(stageNo int, stageName, epc, action, tType, tID, poScope, stScope, bulScope, gw, stn, zone string, rssi int) {
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO rfid_live_tag_events_v51 (
				tenant_id, epc_hex, action, stage_no, stage_name, target_type, target_id,
				po_number, style_code, bulletin_code, gateway_id, station_id, zone_code, rssi_dbm
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
			tenant, epc, action, stageNo, stageName, tType, tID, poScope, stScope, bulScope, gw, stn, zone, rssi)
	}

	// Stage 1, 2 & 2a: Store receives fabric rolls into PO-Independent GENERAL_STOCK first,
	// maps Live RFID tags from the UNUSED pool, and then allocates rolls to (PO, Style, Bulletin).
	for i := 1; i <= rollCount+1; i++ {
		serial := fmt.Sprintf("ROLL-%s-%04d", styleCode, i)
		decLen := int64(150000)
		decWid := 1480
		shade := "A"
		if i > (rollCount+1)/2 {
			shade = "B"
		}
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO store_supplier_roll_sheets_v51 (
				tenant_id, po_number, serial_no, supplier_code, material_sku, fabric_description,
				content, gsm, construction, color_code, declared_width_mm, declared_length_mm,
				shade_band, balance_length_mm
			) VALUES ($1,'GENERAL_STOCK',$2,'MEGCOU-8015553','60570-BISTRETCH-SS89','Bi-Stretch Twill Wicking','73% Poly 21% Rayon 6% Spandex',215,'Twill Woven',$3,$4,$5,$6,$5)
			ON CONFLICT (tenant_id, serial_no) DO UPDATE SET balance_length_mm=EXCLUDED.balance_length_mm`,
			tenant, serial, colorCode, decWid, decLen, shade)

		if i == rollCount+1 {
			continue
		}
		epc := makeEPC("F0B100", i)
		nfc := fmt.Sprintf("04A1B2%08X", i)
		human := fmt.Sprintf("LIVE-ROLL-%03d", i)
		// First mapped as PO-independent GENERAL_STOCK roll in Store Room
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO rfid_live_tag_pool_v51 (
				tenant_id, epc_hex, nfc_uid, human_code, tag_class, status, battery_mv,
				target_type, target_id, po_number, style_code, bulletin_code, op_stage, current_zone, current_station
			) VALUES ($1,$2,$3,$4,'LIVE_RTLS','MAPPED',3100,'ROLL',$5,'GENERAL_STOCK','','','STORE_INVENTORY','INSPECTION_BAY','INSP-TABLE-01')
			ON CONFLICT (tenant_id, epc_hex) DO UPDATE SET status='MAPPED', target_type='ROLL', target_id=$5, po_number='GENERAL_STOCK'`,
			tenant, epc, nfc, human, serial)
		recordStage(2, "Stage 2a · PO-Independent Store Roll Inspection & Live RFID Map", epc, "MAP", "ROLL", serial, "GENERAL_STOCK", "", "", "GW-INSP-01", "INSP-TABLE-01", "INSPECTION_BAY", -41)

		// Stage 3: Put-away in Store Rack, then dynamically allocated to (poNum, styleCode, bulletinCode)
		rackBin := fmt.Sprintf("RACK-A-BAY-%02d", (i%4)+1)
		actWid, actLen, pts, inspStatus := decWid, decLen, 4, "PASS"
		if i == rollCount {
			actWid = 1440
			actLen = 143000
			pts = 28
			inspStatus = "CLAIM"
		}
		grn := fmt.Sprintf("GRN-%s-%02d", styleCode, i)
		_, _ = tx.ExecContext(ctx, `
			UPDATE store_supplier_roll_sheets_v51
			SET actual_width_mm=$3, actual_length_mm=$4, defect_points=$5, inspection_status=$6,
			    mapped_epc_hex=$7, grn_number=$8, grn_posted_at=clock_timestamp(),
			    allocated_po_number=$9, allocated_style_code=$10, allocated_bulletin_id=$11,
			    zone_location=$12, balance_length_mm=$4
			WHERE tenant_id=$1 AND serial_no=$2`,
			tenant, serial, actWid, actLen, pts, inspStatus, epc, grn, poNum, styleCode, bulletinCode, rackBin)
		_, _ = tx.ExecContext(ctx, `
			UPDATE rfid_live_tag_pool_v51
			SET po_number=$3, style_code=$4, bulletin_code=$5, op_stage='STAGE_3_STORE_ALLOCATED', current_zone=$6
			WHERE tenant_id=$1 AND epc_hex=$2`,
			tenant, epc, poNum, styleCode, bulletinCode, rackBin)
		recordStage(3, "Stage 3 · Store Roll Allocated to PO/Style/Bulletin & Relaxed", epc, "ALLOCATE_PO", "ROLL", serial, poNum, styleCode, bulletinCode, "GW-STORE-RTLS-01", "STORE-AISLE-A", rackBin, -46)

		// Stage 4: Spreading & Cutting — consume roll, release Live RFID tag back to PO-independent UNUSED pool
		if i < rollCount-1 {
			_, _ = tx.ExecContext(ctx, `
				UPDATE store_supplier_roll_sheets_v51 SET balance_length_mm=0, zone_location='CUTTING_CONSUMED'
				WHERE tenant_id=$1 AND serial_no=$2`, tenant, serial)
			_, _ = tx.ExecContext(ctx, `
				UPDATE rfid_live_tag_pool_v51
				SET status='UNUSED', target_type=NULL, target_id=NULL, po_number='', style_code='', bulletin_code='', op_stage='',
				    current_zone='TAG_STOCK_BIN', current_station='CUT-TABLE-01', reuse_count=reuse_count+1, updated_at=clock_timestamp()
				WHERE tenant_id=$1 AND epc_hex=$2`, tenant, epc)
			recordStage(4, "Stage 4 · Roll Consumed & Live Tag Recycled to Global UNUSED Pool", epc, "RETURN_TO_UNUSED", "ROLL", serial, poNum, styleCode, bulletinCode, "GW-CUT-01", "CUT-TABLE-01", "TAG_STOCK_BIN", -38)
		} else {
			// End-bit returned to General Store keeps its tag for future POs
			_, _ = tx.ExecContext(ctx, `
				UPDATE store_supplier_roll_sheets_v51 SET balance_length_mm=18500, allocated_po_number='', zone_location='STORE_ENDBIT_RACK'
				WHERE tenant_id=$1 AND serial_no=$2`, tenant, serial)
			recordStage(4, "Stage 4 · Part-Roll End-Bit Returned to Store (Available for Next PO)", epc, "STAGE_DEPART", "ROLL", serial, "GENERAL_STOCK", styleCode, bulletinCode, "GW-CUT-01", "CUT-TABLE-01", "STORE_ENDBIT_RACK", -44)
		}
	}

	// Stage 5 to 13: Bundle Dual UHF+NFC Fobs mapped to (poNum, styleCode, bulletinCode)
	sizes := []string{"30", "32", "34", "36", "38", "40", "42"}
	for b := 1; b <= bundleCount; b++ {
		sz := sizes[(b-1)%len(sizes)]
		bundleID := fmt.Sprintf("B-%s-%s-%s-%03d", poNum, styleCode, sz, b)
		epc := makeEPC("CB08F0", b)
		nfc := fmt.Sprintf("04C5D6%08X", b)
		human := fmt.Sprintf("FOB-%s-%03d", styleCode, b)

		_, _ = tx.ExecContext(ctx, `
			INSERT INTO rfid_live_tag_pool_v51 (
				tenant_id, epc_hex, nfc_uid, human_code, tag_class, status, battery_mv,
				target_type, target_id, po_number, style_code, bulletin_code, op_stage, current_zone, current_station
			) VALUES ($1,$2,$3,$4,'DUAL_UHF_NFC','MAPPED',0,'BUNDLE',$5,$6,$7,$8,'SEWING_LINE','BUNDLING_DESK','BND-ENC-01')
			ON CONFLICT (tenant_id, epc_hex) DO UPDATE SET status='MAPPED', target_type='BUNDLE', target_id=$5, po_number=$6, style_code=$7, bulletin_code=$8`,
			tenant, epc, nfc, human, bundleID, poNum, styleCode, bulletinCode)

		recordStage(5, "Stage 5 · Bundling Desk RFID Encode ("+styleCode+")", epc, "MAP", "BUNDLE", bundleID, poNum, styleCode, bulletinCode, "GW-BND-01", "BND-ENC-01", "BUNDLING_DESK", -32)
		recordStage(8, "Stage 8 · Sewing Station NFC Puck Tap (Sub-Assembly: Fly & Pocket)", epc, "STAGE_ARRIVE", "BUNDLE", bundleID, poNum, styleCode, bulletinCode, "NFC-PUCK-L14-S03", "L14-ST03-SNLS", "SEWING_LINE_14", -28)
		recordStage(8, "Stage 8 · Sewing Station NFC Puck Tap (Sub-Assembly: Waistband)", epc, "STAGE_ARRIVE", "BUNDLE", bundleID, poNum, styleCode, bulletinCode, "NFC-PUCK-L14-S09", "L14-ST09-WB", "SEWING_LINE_14", -29)
		recordStage(9, "Stage 9 · Inline & End-Line QC Table Reader", epc, "SCAN_SIGHTING", "BUNDLE", bundleID, poNum, styleCode, bulletinCode, "NFC-PUCK-L14-QC", "L14-QC-01", "INLINE_QC", -31)
		recordStage(12, "Stage 12 · Finishing Trim & Steam Press Portal", epc, "STAGE_ARRIVE", "BUNDLE", bundleID, poNum, styleCode, bulletinCode, "GW-FINISH-01", "FIN-PRESS-02", "FINISHING_FLOOR", -45)
		recordStage(13, "Stage 13 · Final AQL Pass & Bundle Fob Returned to Global UNUSED Pool", epc, "RETURN_TO_UNUSED", "BUNDLE", bundleID, poNum, styleCode, bulletinCode, "NFC-PUCK-AQL-01", "AQL-DESK-01", "TAG_STOCK_BIN", -30)

		_, _ = tx.ExecContext(ctx, `
			UPDATE rfid_live_tag_pool_v51
			SET status='UNUSED', target_type=NULL, target_id=NULL, po_number='', style_code='', bulletin_code='', op_stage='',
			    current_zone='TAG_STOCK_BIN', current_station='AQL-DESK-01', reuse_count=reuse_count+1, updated_at=clock_timestamp()
			WHERE tenant_id=$1 AND epc_hex=$2`, tenant, epc)
	}

	// Stage 11: Movable Factory Assets locked by the Bulletin
	assets := []struct{ code, tType, zone string }{
		{"A-TROLLEY-01", "TROLLEY", "SEWING_TO_FINISHING_CORRIDOR"},
		{"A-SNLS-0311", "MACHINE", "SEWING_LINE_14_ST03"},
		{"A-WB-0902", "MACHINE", "SEWING_LINE_14_ST09"},
		{"A-FORKLIFT-01", "FORKLIFT", "DISPATCH_DOCK_BAY_1"},
	}
	for idx, a := range assets {
		epc := makeEPC("RT1500", idx+1)
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO rfid_live_tag_pool_v51 (
				tenant_id, epc_hex, human_code, tag_class, status, battery_mv,
				target_type, target_id, po_number, style_code, bulletin_code, op_stage, current_zone
			) VALUES ($1,$2,$3,'LIVE_RTLS','MAPPED',2980,$4,$3,$5,$6,$7,'ASSET_LOCKED',$8)
			ON CONFLICT (tenant_id, epc_hex) DO UPDATE SET po_number=$5, style_code=$6, bulletin_code=$7, current_zone=$8`,
			tenant, epc, a.code, a.tType, poNum, styleCode, bulletinCode, a.zone)
		recordStage(11, "Stage 11 · Bulletin-Locked Machine/Trolley RTLS Telemetry", epc, "SCAN_SIGHTING", a.tType, a.code, poNum, styleCode, bulletinCode, "GW-CEILING-RTLS", a.code, a.zone, -52)
	}

	// Stage 14–17: SGTIN-96 Garment Hangtag Encoding, Carton Tunnel, and Dock Dispatch
	sgtinSamples := []string{}
	for c := 1; c <= cartonCount; c++ {
		cartonCode := fmt.Sprintf("SSCC-%s-%04d", styleCode, c)
		cartonEPC := makeEPC("307400", c)
		for g := 1; g <= 4; g++ {
			serialNo := int64((c-1)*100 + g)
			sgtin, err := rfid.EncodeSGTIN96("0614141", 3001, serialNo)
			if err != nil {
				sgtin = makeEPC("303400", int(serialNo))
			}
			if len(sgtinSamples) < 6 {
				sgtinSamples = append(sgtinSamples, sgtin)
			}
			garmentSerial := fmt.Sprintf("%s-%s-G-%04d", poNum, styleCode, serialNo)
			recordStage(14, "Stage 14 · Garment SGTIN-96 Hangtag & Polybag Encode", sgtin, "MAP", "GARMENT", garmentSerial, poNum, styleCode, bulletinCode, "PRINTER-ENC-01", "PACK-TAG-01", "POLYBAG_STATION", -33)
		}
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO rfid_live_tag_pool_v51 (
				tenant_id, epc_hex, human_code, tag_class, status, battery_mv,
				target_type, target_id, po_number, style_code, bulletin_code, op_stage, current_zone, current_station
			) VALUES ($1,$2,$3,'SSCC96_CARTON','MAPPED',0,'CARTON',$3,$4,$5,$6,'DOCK_DISPATCH','DISPATCH_DOCK','DOCK-DOOR-01')
			ON CONFLICT (tenant_id, epc_hex) DO UPDATE SET current_zone='DISPATCH_DOCK'`,
			tenant, cartonEPC, cartonCode, poNum, styleCode, bulletinCode)
		recordStage(15, "Stage 15 · Carton UHF Tunnel Check (24 pcs)", cartonEPC, "STAGE_ARRIVE", "CARTON", cartonCode, poNum, styleCode, bulletinCode, "TUNNEL-PACK-01", "PACK-TUNNEL-01", "PACKING_LINE_1", -36)
		recordStage(17, "Stage 17 · Dispatch Dock Portal Scan & ASN Confirmation", cartonEPC, "STAGE_DEPART", "CARTON", cartonCode, poNum, styleCode, bulletinCode, "PORTAL-DOCK-01", "DOCK-DOOR-01", "CONTAINER_SEALED", -39)
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Experiment commit failed", http.StatusServiceUnavailable)
		return
	}
	docJSON(w, http.StatusOK, map[string]any{
		"status":                "COMPLETED_17_STAGES",
		"po_number":             poNum,
		"style_code":            styleCode,
		"bulletin_code":         bulletinCode,
		"supplier_sheet_rolls":  rollCount + 1,
		"mapped_live_roll_tags": rollCount,
		"short_unmapped_rolls":  1,
		"claim_variance_rolls":  1,
		"rolls_recycled_unused": rollCount - 2,
		"endbit_rolls_retained": 2,
		"bundles_cycled":        bundleCount,
		"rtls_movable_assets":   len(assets),
		"garments_sgtin96":      cartonCount * 4,
		"cartons_dispatched":    cartonCount,
		"sample_sgtin96_epcs":   sgtinSamples,
	})
}

func nonEmptyOr(val, fallback string) string {
	if strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return fallback
}

func clampInt(val, minVal, maxVal, defVal int) int {
	if val <= 0 {
		return defVal
	}
	if val < minVal {
		return minVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}
