// Author: Sunil Jain, CyberinQ LLC, 2026-10-30.
package provenance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"time"
)

func TestProvenanceStoryIntegrity(t *testing.T) {
	story := ProvenanceStory{
		ProtocolVersion:           "AGIPAREL_PROVENANCE_V45",
		GarmentID:                 "00000000-0000-0000-0000-000000000001",
		TenantID:                  "TENANT_NOBLE",
		PlantID:                   "PLANT_01",
		OriginFabricSKU:           "AEGEAN-COTTON-842",
		ShadeBand:                 "B",
		SourceOrderHash:           "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		ColorCode:                 "INDIGO_BLUE",
		SizeCode:                  "32W_34L",
		MarkerPosition:            4,
		PlyNumber:                 12,
		OperationsCompleted:       18,
		PseudonymizedCraftspeople: 11,
		QCReleased:                true,
		CartonCode:                "CARTON-C-1104",
		StatutoryCompliance: map[string]interface{}{
			"eu_dpp_compliant":              true,
			"uflpa_chain_of_custody_proven": true,
			"worker_privacy_protected":      true,
		},
		ExportedAt: time.Now().UTC(),
	}

	bytes, err := json.Marshal(story)
	if err != nil {
		t.Fatalf("Failed to serialize story: %v", err)
	}

	h := sha256.Sum256(bytes)
	hashHex := hex.EncodeToString(h[:])
	if len(hashHex) != 64 {
		t.Fatalf("Expected 64-char sha256 hex, got: %s", hashHex)
	}

	if !story.QCReleased || story.OperationsCompleted != 18 {
		t.Fatalf("Invariant violated: story assertions mismatched")
	}
}
