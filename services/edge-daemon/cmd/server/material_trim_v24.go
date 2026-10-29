// Author: Sunil Jain, CyberinQ LLC, 2026-10-29.
// F3.14: authenticated trim issue challenge and independent issuer/recipient attestations.
package main

import (
	"errors"
	"net/http"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/stores"
)

type trimIssueParams struct {
	TrimStockID   string `json:"trim_stock_id"`
	OrderID       string `json:"order_id"`
	LineID        string `json:"line_id"`
	QuantityMilli int64  `json:"quantity_milli"`
	CommandID     string `json:"command_id"`
}
type trimIssueBody struct {
	trimIssueParams
	IssuerAttestation    string `json:"issuer_attestation"`
	RecipientAttestation string `json:"recipient_attestation"`
}

func trimCommand(p trimIssueParams, tenant, plant string) stores.TrimIssueCommand {
	return stores.TrimIssueCommand{TenantID: tenant, PlantID: plant, TrimStockID: p.TrimStockID, OrderID: p.OrderID, LineID: p.LineID, QuantityMilli: p.QuantityMilli, CommandID: p.CommandID}
}
func (a *materialAPI) trimIssueChallenge(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !a.ready(w) {
		return
	}
	actor, err := a.authorize(r, "material.issue.prepare", "stores_issuer", true)
	if err != nil {
		materialError(w, err)
		return
	}
	var p trimIssueParams
	if err = decodeMaterialJSON(w, r, &p); err != nil {
		respondMaterial(w, 400, map[string]string{"error": "ERR_F314_JSON"})
		return
	}
	c := trimCommand(p, actor.TenantID, actor.PlantID)
	if !stores.ValidTrimIssue(c) {
		materialError(w, errors.New("ERR_F314_TRIM_FIELDS"))
		return
	}
	respondMaterial(w, 200, map[string]any{"challenge": stores.TrimIssueChallenge(c), "action": "material.issue.witness",
		"issuer_role": "stores_issuer", "recipient_role": "stores_recipient", "quantity_unit": "milli_of_stock_uom",
		"authority": "CHALLENGE_ONLY_NO_STOCK_CHANGE"})
}
func (a *materialAPI) trimIssue(w http.ResponseWriter, r *http.Request) {
	if !requirePOST(w, r) || !a.ready(w) {
		return
	}
	actor, err := a.authorize(r, "material.issue.submit", "stores_issuer", true)
	if err != nil {
		materialError(w, err)
		return
	}
	var p trimIssueBody
	if err = decodeMaterialJSON(w, r, &p); err != nil {
		respondMaterial(w, 400, map[string]string{"error": "ERR_F314_JSON"})
		return
	}
	c := trimCommand(p.trimIssueParams, actor.TenantID, actor.PlantID)
	if !stores.ValidTrimIssue(c) {
		materialError(w, errors.New("ERR_F314_TRIM_FIELDS"))
		return
	}
	ch := stores.TrimIssueChallenge(c)
	i, err := a.verifier.Verify(p.IssuerAttestation, "material.issue.witness", ch, "stores_issuer", true)
	if err != nil {
		materialError(w, errors.New("ERR_F314_ISSUER_PROOF"))
		return
	}
	rec, err := a.verifier.Verify(p.RecipientAttestation, "material.issue.witness", ch, "stores_recipient", true)
	if err != nil {
		materialError(w, errors.New("ERR_F314_RECIPIENT_PROOF"))
		return
	}
	if i.TenantID != actor.TenantID || rec.TenantID != actor.TenantID || i.PlantID != actor.PlantID || rec.PlantID != actor.PlantID ||
		i.Subject != actor.Subject || i.Subject == rec.Subject || i.JWTID == rec.JWTID {
		materialError(w, errors.New("ERR_F314_INDEPENDENT_WITNESSES"))
		return
	}
	c.Issuer = stores.TrimSignedProof(i.TenantID, i.PlantID, i.Issuer, i.Subject, i.JWTID, p.IssuerAttestation, "stores_issuer")
	c.Recipient = stores.TrimSignedProof(rec.TenantID, rec.PlantID, rec.Issuer, rec.Subject, rec.JWTID, p.RecipientAttestation, "stores_recipient")
	receipt, err := a.repository.IssueTrim(r.Context(), c)
	if err != nil {
		materialError(w, err)
		return
	}
	status := 201
	if receipt.Replayed {
		status = 200
	}
	respondMaterial(w, status, receipt)
}
