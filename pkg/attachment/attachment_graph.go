package attachment

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
)

var (
	ErrShadeMismatch   = errors.New("ERR_SHADE_MISMATCH: Cross-pant attachment violation")
	ErrCutMismatch     = errors.New("ERR_CROSS_CUT: Cross-cut attachment violation")
	ErrInvalidSequence = errors.New("ERR_INVALID_ATTACHMENT: Operation pairing not in DAG")
)

type SubBundleNode struct {
	SequentialID int    `json:"sequential_id"`
	CutNo        int    `json:"cut_no"`
	PantPosition int    `json:"pant_position"`
	SubBundleID  int    `json:"sub_bundle_id"`
	PartCode     string `json:"part_code"`
	PartName     string `json:"part_name"`
	PlyStart     int    `json:"ply_start"`
	PlyEnd       int    `json:"ply_end"`
}

type AttachmentEngine struct {
	sync.RWMutex
	validRules   map[string]map[string]map[string]bool
	bundleLookup map[string]SubBundleNode
}

func NewAttachmentEngine() *AttachmentEngine {
	return &AttachmentEngine{
		validRules:   make(map[string]map[string]map[string]bool),
		bundleLookup: make(map[string]SubBundleNode),
	}
}

func partKey(node SubBundleNode) string {
	if node.PartCode != "" {
		return node.PartCode
	}
	return strconv.Itoa(node.SubBundleID)
}

func (e *AttachmentEngine) RegisterBundle(nfcUID string, node SubBundleNode) {
	e.Lock()
	defer e.Unlock()
	e.bundleLookup[nfcUID] = node
}

func (e *AttachmentEngine) BindTag(nfcUID string, node SubBundleNode) {
	e.RegisterBundle(nfcUID, node)
}

func (e *AttachmentEngine) RegisterRule(styleID, parentPartCode, childPartCode string) {
	e.Lock()
	defer e.Unlock()
	if e.validRules[styleID] == nil {
		e.validRules[styleID] = make(map[string]map[string]bool)
	}
	if e.validRules[styleID][parentPartCode] == nil {
		e.validRules[styleID][parentPartCode] = make(map[string]bool)
	}
	e.validRules[styleID][parentPartCode][childPartCode] = true
}

func (e *AttachmentEngine) ValidatePair(styleID, sourceNFC, targetNFC string) error {
	e.Lock()
	defer e.Unlock()

	src, okSrc := e.bundleLookup[sourceNFC]
	tgt, okTgt := e.bundleLookup[targetNFC]

	if !okSrc || !okTgt {
		return errors.New("ERR_UNREGISTERED_TAG: Tag missing from active floor pool")
	}

	if src.CutNo != tgt.CutNo {
		return fmt.Errorf("%w: Cannot attach Cut %d to Cut %d", ErrCutMismatch, src.CutNo, tgt.CutNo)
	}
	if src.PantPosition != tgt.PantPosition {
		return fmt.Errorf("%w: Bundle %d (Pant %d) cannot marry Bundle %d (Pant %d)", ErrShadeMismatch, src.SequentialID, src.PantPosition, tgt.SequentialID, tgt.PantPosition)
	}
	if src.PlyStart != tgt.PlyStart || src.PlyEnd != tgt.PlyEnd {
		return fmt.Errorf("ERR_PLY_MISMATCH: Ply group mismatch [%d-%d] vs [%d-%d]", src.PlyStart, src.PlyEnd, tgt.PlyStart, tgt.PlyEnd)
	}

	srcKey := partKey(src)
	tgtKey := partKey(tgt)

	if e.validRules[styleID] == nil || e.validRules[styleID][srcKey] == nil || !e.validRules[styleID][srcKey][tgtKey] {
		return fmt.Errorf("%w: Part %s cannot attach to %s", ErrInvalidSequence, src.PartName, tgt.PartName)
	}

	return nil
}

func (e *AttachmentEngine) RetireSecondaryTag(sourceNFC string) {
	e.Lock()
	defer e.Unlock()
	delete(e.bundleLookup, sourceNFC)
}
