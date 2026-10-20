package attachment

import (
	"errors"
	"fmt"
	"sync"
)

type SubBundleNode struct {
	SequentialID int    `json:"sequential_id"`
	CutNo        int    `json:"cut_no"`
	PantPosition int    `json:"pant_position"`
	SubBundleID  int    `json:"sub_bundle_id"` // 16-part component ID
	PartName     string `json:"part_name"`
	PlyStart     int    `json:"ply_start"`
	PlyEnd       int    `json:"ply_end"`
}

type AttachmentEngine struct {
	sync.RWMutex
	validEdges   map[string]map[int]map[int]bool // Style -> SourcePart -> TargetPart -> true
	bundleLookup map[string]SubBundleNode        // NFC_UID -> Node
}

func NewAttachmentEngine() *AttachmentEngine {
	return &AttachmentEngine{
		validEdges:   make(map[string]map[int]map[int]bool),
		bundleLookup: make(map[string]SubBundleNode),
	}
}

func (e *AttachmentEngine) RegisterBundle(nfcUID string, node SubBundleNode) {
	e.Lock()
	defer e.Unlock()
	e.bundleLookup[nfcUID] = node
}

func (e *AttachmentEngine) RegisterValidEdge(styleID string, srcPart, tgtPart int) {
	e.Lock()
	defer e.Unlock()
	if e.validEdges[styleID] == nil {
		e.validEdges[styleID] = make(map[int]map[int]bool)
	}
	if e.validEdges[styleID][srcPart] == nil {
		e.validEdges[styleID][srcPart] = make(map[int]bool)
	}
	e.validEdges[styleID][srcPart][tgtPart] = true
}

func (e *AttachmentEngine) ValidatePair(styleID, sourceNFC, targetNFC string) error {
	e.RLock()
	defer e.RUnlock()

	src, okSrc := e.bundleLookup[sourceNFC]
	tgt, okTgt := e.bundleLookup[targetNFC]
	if !okSrc || !okTgt {
		return errors.New("ERR_UNREGISTERED_TAG: Tag missing from active floor pool")
	}

	// Invariant: Cross-cut assembly is prohibited
	if src.CutNo != tgt.CutNo {
		return fmt.Errorf("ERR_CROSS_CUT: Cannot attach Cut %d to Cut %d", src.CutNo, tgt.CutNo)
	}

	// Invariant: Cross-pant assembly (shade-mix) is prohibited
	if src.PantPosition != tgt.PantPosition {
		return fmt.Errorf("ERR_SHADE_MISMATCH: Bundle %d (Pant %d) cannot marry Bundle %d (Pant %d)",
			src.SequentialID, src.PantPosition, tgt.SequentialID, tgt.PantPosition)
	}

	// Invariant: Exact ply-group alignment required
	if src.PlyStart != tgt.PlyStart || src.PlyEnd != tgt.PlyEnd {
		return fmt.Errorf("ERR_PLY_MISMATCH: Ply group mismatch [%d-%d] vs [%d-%d]",
			src.PlyStart, src.PlyEnd, tgt.PlyStart, tgt.PlyEnd)
	}

	// Invariant: Directed edge validation
	if !e.validEdges[styleID][src.SubBundleID][tgt.SubBundleID] {
		return fmt.Errorf("ERR_INVALID_ATTACHMENT: Part %s cannot attach to %s", src.PartName, tgt.PartName)
	}

	return nil
}