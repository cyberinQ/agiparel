// Author: Sunil Jain, 2026-10-26 — Validate typed sewing input/output cardinality and assembly compatibility.
package sewing

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

type ExecutionMode string

const (
	SingleToSingle ExecutionMode = "SINGLE_TO_SINGLE"
	BundleToBundle ExecutionMode = "BUNDLE_TO_BUNDLE"
	BundleToSingle ExecutionMode = "BUNDLE_TO_SINGLE"
)

// OperationContract is derived from a signed, published OB revision.
// It is a DATA contract, not authentication or work-completion evidence.
type OperationContract struct {
	ID                   string        `json:"id"`
	StyleRevision        string        `json:"style_revision"`
	RouteRevision        string        `json:"route_revision"`
	MachineClass         string        `json:"machine_class"`
	Mode                 ExecutionMode `json:"mode"`
	RequiredParts        []string      `json:"required_parts"`
	RequiredPredecessors []string      `json:"required_predecessors"`
}

type SourcePart struct {
	CarrierID      string   `json:"carrier_id"`
	PartCode       string   `json:"part_code"`
	Kind           string   `json:"kind"` // PIECE or BUNDLE
	StyleRevision  string   `json:"style_revision"`
	Lot            string   `json:"lot"`
	Shade          string   `json:"shade"`
	GarmentSerials []string `json:"garment_serials"`
}

// OperationClaim is an untrusted proposal until separately authenticated
// against the actor, device, published route, RFID evidence, and authoritative
// DB transaction. This validator cannot establish that sewing occurred.
type OperationClaim struct {
	CommandID             string       `json:"command_id"`
	RouteRevision         string       `json:"route_revision"`
	MachineClass          string       `json:"machine_class"`
	Inputs                []SourcePart `json:"inputs"`
	OutputGarmentSerials  []string     `json:"output_garment_serials"`
	CompletedPredecessors []string     `json:"completed_predecessors"`
}

type OperationValidation struct {
	Eligible        bool `json:"eligible"`
	CreditableUnits int  `json:"creditable_units"`
}

func canonicalUnique(xs []string) ([]string, error) {
	if len(xs) == 0 {
		return nil, errors.New("ERR_EMPTY_SERIAL_SET")
	}
	sorted := append([]string{}, xs...)
	sort.Strings(sorted)
	for i, v := range sorted {
		if strings.TrimSpace(v) == "" || (i > 0 && sorted[i-1] == v) {
			return nil, errors.New("ERR_DUPLICATE_OR_BLANK_SERIAL")
		}
	}
	return sorted, nil
}
func sameSerialSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ValidateOperationClaim rejects cross-style, cross-shade, wrong-part,
// wrong-serial, duplicate-source and predecessor/cardinality violations.
// It NEVER produces a production credit itself; persistence must prevent
// double spending across concurrent claims and replayed commands.
func ValidateOperationClaim(c OperationContract, claim OperationClaim) (OperationValidation, error) {
	var invalid OperationValidation
	if strings.TrimSpace(c.ID) == "" || c.RouteRevision == "" || c.StyleRevision == "" || c.MachineClass == "" ||
		strings.TrimSpace(claim.CommandID) == "" || claim.RouteRevision != c.RouteRevision || claim.MachineClass != c.MachineClass {
		return invalid, errors.New("ERR_OPERATION_CONTEXT: Missing or inconsistent contract/revision/machine identity")
	}
	if len(claim.Inputs) == 0 {
		return invalid, errors.New("ERR_NO_INPUTS")
	}
	output, err := canonicalUnique(claim.OutputGarmentSerials)
	if err != nil {
		return invalid, err
	}
	seenPrev := make(map[string]bool, len(claim.CompletedPredecessors))
	for _, p := range claim.CompletedPredecessors {
		seenPrev[p] = true
	}
	for _, p := range c.RequiredPredecessors {
		if !seenPrev[p] {
			return invalid, fmt.Errorf("ERR_PREDECESSOR: %s not committed", p)
		}
	}
	if c.Mode == SingleToSingle || c.Mode == BundleToBundle {
		if len(claim.Inputs) != 1 || len(c.RequiredParts) != 1 {
			return invalid, errors.New("ERR_CARDINALITY: Expected exactly one source part type")
		}
	} else if c.Mode == BundleToSingle {
		if len(c.RequiredParts) < 2 || len(claim.Inputs) != len(c.RequiredParts) {
			return invalid, errors.New("ERR_CARDINALITY: Expected one source bundle per required part")
		}
	} else {
		return invalid, errors.New("ERR_UNSUPPORTED_MODE")
	}
	required := make(map[string]int)
	for _, p := range c.RequiredParts {
		if p == "" || required[p] != 0 {
			return invalid, errors.New("ERR_CONTRACT_PART: Duplicate or blank required part")
		}
		required[p] = 1
	}
	observed := make(map[string]bool)
	carriers := make(map[string]bool)
	var firstShade, firstLot string
	for i, input := range claim.Inputs {
		if strings.TrimSpace(input.CarrierID) == "" || carriers[input.CarrierID] {
			return invalid, errors.New("ERR_SOURCE_DOUBLE_USE")
		}
		carriers[input.CarrierID] = true
		if required[input.PartCode] != 1 || observed[input.PartCode] {
			return invalid, errors.New("ERR_PART_COMPATIBILITY")
		}
		observed[input.PartCode] = true
		if input.StyleRevision != c.StyleRevision || input.Shade == "" || input.Lot == "" {
			return invalid, errors.New("ERR_SOURCE_REVISION_OR_SHADE")
		}
		if i == 0 {
			firstShade, firstLot = input.Shade, input.Lot
		} else if input.Shade != firstShade || input.Lot != firstLot {
			return invalid, errors.New("ERR_CROSS_SHADE_OR_LOT")
		}
		if c.Mode == SingleToSingle && input.Kind != "PIECE" || c.Mode != SingleToSingle && input.Kind != "BUNDLE" {
			return invalid, errors.New("ERR_INPUT_KIND")
		}
		serials, e := canonicalUnique(input.GarmentSerials)
		if e != nil || !sameSerialSet(serials, output) {
			return invalid, errors.New("ERR_GARMENT_SERIAL_MISMATCH")
		}
	}
	if c.Mode == SingleToSingle && len(output) != 1 {
		return invalid, errors.New("ERR_SINGLE_CARDINALITY")
	}
	return OperationValidation{Eligible: true, CreditableUnits: len(output)}, nil
}
