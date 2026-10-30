// Author: Sunil Jain, CyberinQ LLC, 2026-10-30.
package provenance

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type ProvenanceStory struct {
	ProtocolVersion           string                 `json:"protocol_version"`
	GarmentID                 string                 `json:"garment_id"`
	TenantID                  string                 `json:"tenant_id"`
	PlantID                   string                 `json:"plant_id"`
	OriginFabricSKU           string                 `json:"origin_fabric_sku"`
	ShadeBand                 string                 `json:"shade_band"`
	SourceOrderHash           string                 `json:"source_order_hash"`
	ColorCode                 string                 `json:"color_code"`
	SizeCode                  string                 `json:"size_code"`
	MarkerPosition            int                    `json:"marker_position"`
	PlyNumber                 int                    `json:"ply_number"`
	OperationsCompleted       int                    `json:"operations_completed"`
	PseudonymizedCraftspeople int                    `json:"pseudonymized_craftspeople_count"`
	QCReleased                bool                   `json:"qc_released"`
	CartonCode                string                 `json:"carton_code"`
	StatutoryCompliance       map[string]interface{} `json:"statutory_compliance"`
	ExportedAt                time.Time              `json:"exported_at"`
}

type StoryService struct {
	db *sql.DB
}

func NewStoryService(db *sql.DB) *StoryService {
	return &StoryService{db: db}
}

func (s *StoryService) GenerateAndSignStory(ctx context.Context, tenantID, plantID, garmentID, epcHex, signer string) (*ProvenanceStory, string, error) {
	var rawJSON []byte
	query := `SELECT export_garment_story_provenance_v45($1, $2, $3::uuid)`
	if err := s.db.QueryRowContext(ctx, query, tenantID, plantID, garmentID).Scan(&rawJSON); err != nil {
		return nil, "", fmt.Errorf("ERR_EXPORT_PROVENANCE: %w", err)
	}

	var story ProvenanceStory
	if err := json.Unmarshal(rawJSON, &story); err != nil {
		return nil, "", fmt.Errorf("ERR_UNMARSHAL_STORY: %w", err)
	}

	h := sha256.Sum256(rawJSON)
	hashHex := hex.EncodeToString(h[:])
	mockSigHex := fmt.Sprintf("sig_ed25519_%s_%s", signer, hashHex[:16])

	insertQuery := `
		INSERT INTO public.garment_provenance_stories_v45
		(tenant_id, plant_id, garment_id, epc_hex, story_payload, payload_sha256, signed_by, signature_hex)
		VALUES ($1, $2, $3::uuid, $4, $5, $6, $7, $8)
		ON CONFLICT (tenant_id, garment_id) DO UPDATE SET
			story_payload = EXCLUDED.story_payload,
			payload_sha256 = EXCLUDED.payload_sha256,
			signed_by = EXCLUDED.signed_by,
			signature_hex = EXCLUDED.signature_hex,
			generated_at = clock_timestamp()`

	if _, err := s.db.ExecContext(ctx, insertQuery, tenantID, plantID, garmentID, epcHex, rawJSON, hashHex, signer, mockSigHex); err != nil {
		return nil, "", fmt.Errorf("ERR_PERSIST_STORY: %w", err)
	}

	return &story, hashHex, nil
}
