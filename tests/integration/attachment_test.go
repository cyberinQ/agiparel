package integration

import (
	"testing"

	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attachment"
)

func TestDynamicAttachmentDAGValidation(t *testing.T) {
	engine := attachment.NewAttachmentEngine()

	// Register valid edge: Part 101 (LEFT_POCKET) -> Part 102 (FRONT_PANEL) for Style KK524867
	engine.RegisterValidEdge("KK524867", 101, 102)

	// Bundle A: Left Pocket (Pant Position 1)
	engine.RegisterBundle("NFC-BUNDLE-A", attachment.SubBundleNode{
		SequentialID: 1,
		CutNo:        1,
		PantPosition: 1,
		SubBundleID:  101,
		PartName:     "Left Pocket",
		PlyStart:     1,
		PlyEnd:       20,
	})

	// Bundle B: Right Front Panel (Pant Position 2 - Shade Mix Violation)
	engine.RegisterBundle("NFC-BUNDLE-B", attachment.SubBundleNode{
		SequentialID: 2,
		CutNo:        1,
		PantPosition: 2,
		SubBundleID:  102,
		PartName:     "Front Panel",
		PlyStart:     1,
		PlyEnd:       20,
	})

	// Bundle C: Valid Front Panel (Pant Position 1)
	engine.RegisterBundle("NFC-BUNDLE-C", attachment.SubBundleNode{
		SequentialID: 3,
		CutNo:        1,
		PantPosition: 1,
		SubBundleID:  102,
		PartName:     "Front Panel",
		PlyStart:     1,
		PlyEnd:       20,
	})

	// Test 1: Cross-Pant Shade-Mix Interlock
	err := engine.ValidatePair("KK524867", "NFC-BUNDLE-A", "NFC-BUNDLE-B")
	if err == nil {
		t.Fatalf("Expected shade mismatch error, got nil")
	}

	// Test 2: Valid Marriage
	err = engine.ValidatePair("KK524867", "NFC-BUNDLE-A", "NFC-BUNDLE-C")
	if err != nil {
		t.Fatalf("Expected valid attachment, got: %v", err)
	}
}