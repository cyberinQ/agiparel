// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.14B: authenticated, two-witness admission of a new trim lot.
package main

import (
	"errors"
	"net/http"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/stores"
)

type trimAdmissionParams struct {
	StockID          string `json:"stock_id"`
	TrimCode         string `json:"trim_code"`
	Description      string `json:"description"`
	SupplierLot      string `json:"supplier_lot"`
	ColorCode        string `json:"color_code"`
	SizeCode         string `json:"size_code"`
	UOM              string `json:"uom"`
	QuantityMilli    int64  `json:"quantity_milli"`
	BinLocation      string `json:"bin_location"`
	SourceDocumentID string `json:"source_document_id"`
	SourceSHA256     string `json:"source_sha256"`
	CommandID        string `json:"command_id"`
}
type trimAdmissionBody struct {
	trimAdmissionParams
	ManagerAttestation   string `json:"manager_attestation"`
	InspectorAttestation string `json:"inspector_attestation"`
}

func admissionTrimCommand(p trimAdmissionParams, tenant, plant string) stores.TrimAdmissionCommand {
	return stores.TrimAdmissionCommand{TenantID: tenant, PlantID: plant, StockID: p.StockID, TrimCode: p.TrimCode,
		Description: p.Description, SupplierLot: p.SupplierLot, ColorCode: p.ColorCode, SizeCode: p.SizeCode, UOM: p.UOM,
		QuantityMilli: p.QuantityMilli, BinLocation: p.BinLocation, SourceDocumentID: p.SourceDocumentID,
		SourceSHA256: p.SourceSHA256, CommandID: p.CommandID}
}
func (a *materialAPI) trimAdmissionChallenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !a.ready(w) {
		return
	}
	actor, err := a.authorize(r, "material.trims.admit.prepare", "stores_manager", true)
	if err != nil {
		materialError(w, err)
		return
	}
	var p trimAdmissionParams
	if err = decodeMaterialJSON(w, r, &p); err != nil {
		respondMaterial(w, 400, map[string]string{"error": "ERR_F314B_JSON"})
		return
	}
	c := admissionTrimCommand(p, actor.TenantID, actor.PlantID)
	if !stores.ValidTrimAdmission(c) {
		materialError(w, errors.New("ERR_F314B_FIELDS"))
		return
	}
	respondMaterial(w, 200, map[string]any{"challenge": stores.TrimAdmissionChallenge(c), "action": "material.trims.admit.witness",
		"manager_role": "stores_manager", "inspector_role": "stores_inspector",
		"authority": "CHALLENGE_ONLY_NO_STOCK_CREATED", "quantity_unit": "milli_of_stock_uom"})
}
func (a *materialAPI) trimAdmission(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !a.ready(w) {
		return
	}
	actor, err := a.authorize(r, "material.trims.admit.submit", "stores_manager", true)
	if err != nil {
		materialError(w, err)
		return
	}
	var p trimAdmissionBody
	if err = decodeMaterialJSON(w, r, &p); err != nil {
		respondMaterial(w, 400, map[string]string{"error": "ERR_F314B_JSON"})
		return
	}
	c := admissionTrimCommand(p.trimAdmissionParams, actor.TenantID, actor.PlantID)
	if !stores.ValidTrimAdmission(c) {
		materialError(w, errors.New("ERR_F314B_FIELDS"))
		return
	}
	challenge := stores.TrimAdmissionChallenge(c)
	mgr, err := a.verifier.Verify(p.ManagerAttestation, "material.trims.admit.witness", challenge, "stores_manager", true)
	if err != nil {
		materialError(w, errors.New("ERR_F314B_MANAGER_PROOF"))
		return
	}
	ins, err := a.verifier.Verify(p.InspectorAttestation, "material.trims.admit.witness", challenge, "stores_inspector", true)
	if err != nil {
		materialError(w, errors.New("ERR_F314B_INSPECTOR_PROOF"))
		return
	}
	if mgr.Subject != actor.Subject || mgr.TenantID != actor.TenantID || ins.TenantID != actor.TenantID ||
		mgr.PlantID != actor.PlantID || ins.PlantID != actor.PlantID || mgr.Subject == ins.Subject || mgr.JWTID == ins.JWTID {
		materialError(w, errors.New("ERR_F314B_INDEPENDENT_WITNESSES"))
		return
	}
	c.Manager = stores.TrimSignedProof(mgr.TenantID, mgr.PlantID, mgr.Issuer, mgr.Subject, mgr.JWTID, p.ManagerAttestation, "stores_manager")
	c.Inspector = stores.TrimSignedProof(ins.TenantID, ins.PlantID, ins.Issuer, ins.Subject, ins.JWTID, p.InspectorAttestation, "stores_inspector")
	out, err := a.repository.AdmitTrim(r.Context(), c)
	if err != nil {
		materialError(w, err)
		return
	}
	status := http.StatusCreated
	if out.Replayed {
		status = http.StatusOK
	}
	respondMaterial(w, status, out)
}
