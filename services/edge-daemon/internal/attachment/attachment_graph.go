package attachment

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
)

type SubBundleNode struct {
	SequentialID       int    `json:"sequential_id"`
	CutNo              int    `json:"cut_no"`
	PantPosition       int    `json:"pant_position"`
	PartCode           string `json:"part_code"`
	PartName           string `json:"part_name"`
	GarmentSerialStart int    `json:"garment_serial_start"`
	GarmentSerialEnd   int    `json:"garment_serial_end"`
	PlyStart           int    `json:"ply_start"`
	PlyEnd             int    `json:"ply_end"`
}

type AssemblyEdge struct {
	ParentPartCode string
	ChildPartCode  string
	OperationID    string
	MachineType    string
}

type AttachmentEngine struct {
	sync.RWMutex
	validEdges   map[string]map[string]map[string]bool // Style -> ParentPart -> ChildPart -> true
	bundleLookup map[string]SubBundleNode              // NFC_UID -> Node
}

func NewAttachmentEngine() *AttachmentEngine {
	return &AttachmentEngine{
		validEdges:   make(map[string]map[string]map[string]bool),
		bundleLookup: make(map[string]SubBundleNode),
	}
}

// HydrateFromDB compiles dynamic garment assembly DAG edges from PostgreSQL.
func (e *AttachmentEngine) HydrateFromDB(ctx context.Context, db *sql.DB, tenantID, styleCode string) error {
	e.Lock()
	defer e.Unlock()

	query := `
		SELECT sp1.part_code, sp2.part_code
		FROM style_assembly_dag dag
		JOIN styles_master sm ON dag.style_id = sm.id
		JOIN style_parts sp1 ON dag.parent_part_id = sp1.id
		JOIN style_parts sp2 ON dag.child_part_id = sp2.id
		WHERE dag.tenant_id = $1 AND sm.style_code = $2;
	`
	rows, err := db.QueryContext(ctx, query, tenantID, styleCode)
	if err != nil {
		return fmt.Errorf("failed to load assembly DAG: %w", err)
	}
	defer rows.Close()

	if e.validEdges[styleCode] == nil {
		e.validEdges[styleCode] = make(map[string]map[string]bool)
	}

	for rows.Next() {
		var parent, child string
		if err := rows.Scan(&parent, &child); err != nil {
			return err
		}
		if e.validEdges[styleCode][parent] == nil {
			e.validEdges[styleCode][parent] = make(map[string]bool)
		}
		e.validEdges[styleCode][parent][child] = true
	}
	return nil
}

func (e *AttachmentEngine) RegisterRule(styleID, parentPartCode, childPartCode string) {
	e.Lock()
	defer e.Unlock()
	if _, exists := e.validEdges[styleID]; !exists {
		e.validEdges[styleID] = make(map[string]map[string]bool)
	}
	if _, exists := e.validEdges[styleID][parentPartCode]; !exists {
		e.validEdges[styleID][parentPartCode] = make(map[string]bool)
	}
	e.validEdges[styleID][parentPartCode][childPartCode] = true
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

	// Invariant 1: Cross-cut assembly is strictly prohibited
	if src.CutNo != tgt.CutNo {
		return fmt.Errorf("ERR_CROSS_CUT: Cannot attach Cut %d to Cut %d", src.CutNo, tgt.CutNo)
	}

	// Invariant 2: Cross-pant assembly (shade-mix) is strictly prohibited
	if src.PantPosition != tgt.PantPosition {
		return fmt.Errorf("ERR_SHADE_MISMATCH: Bundle %d (Pant %d) cannot marry Bundle %d (Pant %d)",
			src.SequentialID, src.PantPosition, tgt.SequentialID, tgt.PantPosition)
	}

	// Invariant 3: Exact ply-group alignment required
	if src.PlyStart != tgt.PlyStart || src.PlyEnd != tgt.PlyEnd {
		return fmt.Errorf("ERR_PLY_MISMATCH: Ply group mismatch [%d-%d] vs [%d-%d]",
			src.PlyStart, src.PlyEnd, tgt.PlyStart, tgt.PlyEnd)
	}

	// Invariant 4: Directed topological edge validation
	styleEdges, styleExists := e.validEdges[styleID]
	if !styleExists {
		return fmt.Errorf("ERR_INVALID_STYLE: Style %s has no registered attachment rules", styleID)
	}

	targets, sourceExists := styleEdges[src.PartCode]
	if !sourceExists || !targets[tgt.PartCode] {
		return fmt.Errorf("ERR_INVALID_ATTACHMENT: Part %s (%s) cannot attach to Part %s (%s)",
			src.PartName, src.PartCode, tgt.PartName, tgt.PartCode)
	}

	return nil
}