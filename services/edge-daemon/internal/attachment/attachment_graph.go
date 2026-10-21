package attachment

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

var (
	ErrShadeMismatch   = errors.New("ERR_SHADE_MISMATCH: Cross-pant attachment violation")
	ErrCutMismatch     = errors.New("ERR_CUT_MISMATCH: Cross-cut attachment violation")
	ErrInvalidSequence = errors.New("ERR_INVALID_SEQUENCE: Operation pairing not in DAG")
)

type SubBundleNode struct {
	SequentialID int    `json:"sequential_id"`
	CutNo        int    `json:"cut_no"`
	PantPosition int    `json:"pant_position"`
	PartCode     string `json:"part_code"`
	PartName     string `json:"part_name"`
	PlyStart     int    `json:"ply_start"`
	PlyEnd       int    `json:"ply_end"`
}

type AttachmentEngine struct {
	sync.RWMutex
	// Style -> ParentPartCode -> ChildPartCode -> Allowed
	validRules   map[string]map[string]map[string]bool
	bundleLookup map[string]SubBundleNode
	modbusAddr   string
}

func NewAttachmentEngine(modbusAddr ...string) *AttachmentEngine {
	addr := ""
	if len(modbusAddr) > 0 {
		addr = modbusAddr[0]
	}
	return &AttachmentEngine{
		validRules:   make(map[string]map[string]map[string]bool),
		bundleLookup: make(map[string]SubBundleNode),
		modbusAddr:   addr,
	}
}

func (e *AttachmentEngine) BindTag(nfcUID string, node SubBundleNode) {
	e.Lock()
	defer e.Unlock()
	e.bundleLookup[nfcUID] = node
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
	e.RLock()
	src, okSrc := e.bundleLookup[sourceNFC]
	tgt, okTgt := e.bundleLookup[targetNFC]
	allowed := false
	if e.validRules[styleID] != nil && e.validRules[styleID][src.PartCode] != nil {
		allowed = e.validRules[styleID][src.PartCode][tgt.PartCode]
	}
	e.RUnlock()

	if !okSrc || !okTgt {
		return errors.New("ERR_UNREGISTERED_TAG: Tag missing from active floor pool")
	}

	// Invariant: Cross-cut assembly prohibited
	if src.CutNo != tgt.CutNo {
		e.TripModbus()
		return fmt.Errorf("%w: Cut %d cannot marry Cut %d", ErrCutMismatch, src.CutNo, tgt.CutNo)
	}

	// Invariant: Cross-pant assembly (shade-mix) prohibited
	if src.PantPosition != tgt.PantPosition {
		e.TripModbus()
		return fmt.Errorf("%w: Pant %d cannot marry Pant %d", ErrShadeMismatch, src.PantPosition, tgt.PantPosition)
	}

	// Invariant: Ply-group boundary alignment
	if src.PlyStart != tgt.PlyStart || src.PlyEnd != tgt.PlyEnd {
		e.TripModbus()
		return fmt.Errorf("ERR_PLY_MISMATCH: Ply group mismatch [%d-%d] vs [%d-%d]",
			src.PlyStart, src.PlyEnd, tgt.PlyStart, tgt.PlyEnd)
	}

	// Invariant: Directed DAG edge validation
	if !allowed {
		e.TripModbus()
		return fmt.Errorf("%w: Part %s cannot attach to Part %s for style %s",
			ErrInvalidSequence, src.PartCode, tgt.PartCode, styleID)
	}

	return nil
}

func (e *AttachmentEngine) TripModbus() {
	if e.modbusAddr == "" {
		return
	}
	go func() {
		conn, err := net.DialTimeout("tcp", e.modbusAddr, 25*time.Millisecond)
		if err != nil {
			return
		}
		defer conn.Close()

		req := []byte{
			0x00, 0x01, // Transaction ID
			0x00, 0x00, // Protocol ID
			0x00, 0x06, // Length
			0x01,       // Unit ID
			0x05,       // Write Single Coil
			0x00, 0x01, // Coil Address 1
			0xFF, 0x00, // Force Coil ON (Trip Relay)
		}
		_, _ = conn.Write(req)
	}()
}