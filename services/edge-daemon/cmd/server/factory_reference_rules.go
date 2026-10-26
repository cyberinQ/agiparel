// Gate 06.5-B4: strict typed validation of evidence-based reference data.
// Pure functions keep the rules independently testable from PostgreSQL and HTTP.
package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var referenceIDPattern = regexp.MustCompile(`^[A-Za-z0-9_.:/-]{1,64}$`)
var referenceDecimalPattern = regexp.MustCompile(`^(?:0|[1-9][0-9]{0,8})(?:\.[0-9]{1,4})?$`)
var referenceCurrencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
var validRateBases = map[string]bool{"HOUR": true, "PIECE": true, "METER": true, "YARD": true, "KG": true, "DAY": true, "UNIT": true}

type referenceFacts struct {
	Type       string            `json:"entity_type"`
	Key        string            `json:"entity_key"`
	Attributes map[string]string `json:"attributes"`
	RateBasis  string            `json:"rate_basis,omitempty"`
}

func parseReferenceFacts(typ, key string, raw []byte, basis string) (referenceFacts, error) {
	out := referenceFacts{Type: typ, Key: key, RateBasis: basis}
	if len(raw) == 0 || len(raw) > 4096 {
		return out, fmt.Errorf("missing or oversized source attributes")
	}
	if err := json.Unmarshal(raw, &out.Attributes); err != nil || out.Attributes == nil {
		return out, fmt.Errorf("source attributes must be text-valued fields")
	}
	for k, v := range out.Attributes {
		if len(k) > 100 || len(v) > 300 || strings.ContainsRune(v, 0) {
			return out, fmt.Errorf("invalid source attribute")
		}
	}
	field := func(k string) string { return strings.TrimSpace(out.Attributes[k]) }
	switch typ {
	case "MACHINE":
		if !referenceIDPattern.MatchString(field("machine_id")) || key != field("machine_id") || field("machine_type") == "" || len(field("machine_type")) > 180 {
			return out, fmt.Errorf("machine identity and type must match the reviewed source")
		}
		if basis != "" {
			return out, fmt.Errorf("machine cannot have a rate basis")
		}
	case "SKILL":
		if !referenceIDPattern.MatchString(field("worker_id")) || field("skill") == "" || len(field("skill")) > 180 || key != field("worker_id")+":"+field("skill") {
			return out, fmt.Errorf("worker identifier and skill must match the reviewed source")
		}
		if basis != "" {
			return out, fmt.Errorf("skill cannot have a rate basis")
		}
	case "RATE":
		if field("cost_type") == "" || len(field("cost_type")) > 100 || !referenceCurrencyPattern.MatchString(field("currency")) || key != field("cost_type")+":"+field("currency") || !referenceDecimalPattern.MatchString(field("rate")) {
			return out, fmt.Errorf("rate requires exact amount, currency and cost type")
		}
		if !validRateBases[basis] {
			return out, fmt.Errorf("choose the rate basis explicitly (hour, piece, etc.)")
		}
	default:
		return out, fmt.Errorf("only machines, skills and rates can enter the reference catalog; inventory must use verified stores")
	}
	return out, nil
}

func validReferenceAction(action, typ, key, candidate string, expectedRev, draftRev int, note, basis string) error {
	if action != "PUBLISH" && action != "WITHDRAW" {
		return fmt.Errorf("unknown reference action")
	}
	if typ != "MACHINE" && typ != "SKILL" && typ != "RATE" {
		return fmt.Errorf("inventory and other unverified evidence cannot be published")
	}
	if key == "" || len(key) > 256 || strings.TrimSpace(key) != key || strings.ContainsRune(key, 0) {
		return fmt.Errorf("invalid reference key")
	}
	if len(candidate) != 35 || !strings.HasPrefix(candidate, "FC-") {
		return fmt.Errorf("invalid source candidate")
	}
	for _, c := range candidate[3:] {
		if !strings.ContainsRune("0123456789abcdef", c) {
			return fmt.Errorf("invalid source candidate")
		}
	}
	if expectedRev < 0 || draftRev < 1 {
		return fmt.Errorf("invalid expected or draft revision")
	}
	if len(strings.TrimSpace(note)) < 12 || len(note) > 1000 || strings.ContainsRune(note, 0) {
		return fmt.Errorf("explain your verification in at least 12 characters")
	}
	if typ == "RATE" && !validRateBases[basis] {
		return fmt.Errorf("choose a confirmed rate basis")
	}
	if typ != "RATE" && basis != "" {
		return fmt.Errorf("rate basis only applies to rates")
	}
	return nil
}
