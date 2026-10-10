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
	SubBundleID  int    `json:"sub_bundle_id"`
	PartName     string `json:"part_name"`
	PlyStart     int    `json:"ply_start"`
	PlyEnd       int    `json:"ply_end"`
}

type AttachmentEngine struct {
	sync.RWMutex
	validEdges   map[string]map[int]map[int]bool
	bundleLookup map[string]SubBundleNode
}

func NewAttachmentEngine() *AttachmentEngine {
	return &AttachmentEngine{
		validEdges:   make(map[string]map[int]map[int]bool),
		bundleLookup: make(map[string]SubBundleNode),
	}
}

func (e *AttachmentEngine) RegisterRule(styleID string, sourcePartID, targetPartID int) {
	e.Lock()
	defer e.Unlock()

	if _, exists := e.validEdges[styleID]; !exists {
		e.validEdges[styleID] = make(map[int]map[int]bool)
	}
	if _, exists := e.validEdges[styleID][sourcePartID]; !exists {
		e.validEdges[styleID][sourcePartID] = make(map[int]bool)
	}
	e.validEdges[styleID][sourcePartID][targetPartID] = true
}

func (e *AttachmentEngine) BindTag(nfcUID string, node SubBundleNode) {
	e.Lock()
	defer e.Unlock()
	e.bundleLookup[nfcUID] = node
}

func (e *AttachmentEngine) UnbindTag(nfcUID string) {
	e.Lock()
	defer e.Unlock()
	delete(e.bundleLookup, nfcUID)
}

func (e *AttachmentEngine) ResetTags() {
	e.Lock()
	defer e.Unlock()
	e.bundleLookup = make(map[string]SubBundleNode)
}

func (e *AttachmentEngine) ValidatePair(styleID, sourceNFC, targetNFC string) error {
	e.RLock()
	defer e.RUnlock()

	src, okSrc := e.bundleLookup[sourceNFC]
	tgt, okTgt := e.bundleLookup[targetNFC]

	if !okSrc || !okTgt {
		return errors.New("ERR_UNREGISTERED_TAG: Tag missing from active floor pool")
	}

	if src.CutNo != tgt.CutNo {
		return fmt.Errorf("ERR_CROSS_CUT: Cannot attach Cut %d to Cut %d", src.CutNo, tgt.CutNo)
	}

	if src.PantPosition != tgt.PantPosition {
		return fmt.Errorf("ERR_SHADE_MISMATCH: Bundle %d (Pant %d) cannot marry Bundle %d (Pant %d)",
			src.SequentialID, src.PantPosition, tgt.SequentialID, tgt.PantPosition)
	}

	if src.PlyStart != tgt.PlyStart || src.PlyEnd != tgt.PlyEnd {
		return fmt.Errorf("ERR_PLY_MISMATCH: Ply group mismatch [%d-%d] vs [%d-%d]",
			src.PlyStart, src.PlyEnd, tgt.PlyStart, tgt.PlyEnd)
	}

	styleEdges, styleExists := e.validEdges[styleID]
	if !styleExists {
		return fmt.Errorf("ERR_INVALID_STYLE: Style %s has no registered attachment rules", styleID)
	}

	targets, sourceExists := styleEdges[src.SubBundleID]
	if !sourceExists || !targets[tgt.SubBundleID] {
		return fmt.Errorf("ERR_INVALID_ATTACHMENT: Part %s (ID %d) cannot attach to Part %s (ID %d)",
			src.PartName, src.SubBundleID, tgt.PartName, tgt.SubBundleID)
	}

	return nil
}