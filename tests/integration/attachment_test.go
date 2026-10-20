package integration

import (
	"testing"
	"github.com/cyberinq/agiparel/services/edge-daemon/pkg/attachment"
)

func TestAttachmentDAGValidation(t *testing.T) {
	engine := attachment.NewAttachmentEngine()

	// Register valid edge: Part 1 (Left Pocket) -> Part 2 (Front Panel) for Style KK524867
	engine.RegisterValidEdge("KK524867", 1, 2)

	// Bundle A: Left Pocket (Pant Position 1)
	engine.RegisterBundle("NFC-BUNDLE-A", attachment.SubBundleNode{
		SequentialID: 1,
		CutNo:        1,
		PantPosition: 1,
		SubBundleID:  1,
		PartName:     "Left Pocket",
		PlyStart:     1,
		PlyEnd:     20,
	})

	// Bundle B: Right Front Panel (Pant Position 2 - Shade Mix Violation)
	engine.RegisterBundle("NFC-BUNDLE-B", attachment.SubBundleNode{
		SequentialID: 2,
		CutNo:        1,
		PantPosition: 2,
		SubBundleID:  2,
		PartName:     "Front Panel",
		PlyStart:     1,
		PlyEnd:     20,
	})

	// Test Shade-Mix Interlock
	err := engine.ValidatePair("KK524867", "NFC-BUNDLE-A", "NFC-BUNDLE-B")
	if err == nil {
		t.Fatalf("Expected shade mismatch error, got nil")
	}
}