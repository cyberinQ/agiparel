package logicweaver

import (
	"fmt"
	"strings"
)

type RuleDefinition struct {
	RuleID      string `json:"rule_id"`
	Category    string `json:"category"` // e.g., "ATTACHMENT", "DWELL", "TOLERANCE"
	Description string `json:"description"`
	StyleID     string `json:"style_id"`
	ParamA      int    `json:"param_a"`
	ParamB      int    `json:"param_b"`
	IsActive    bool   `json:"is_active"`
}

type LogicWeaverEngine struct {
	tenantID string
}

func NewLogicWeaverEngine(tenantID string) *LogicWeaverEngine {
	return &LogicWeaverEngine{tenantID: tenantID}
}

// CompileRule translates plain-language operational requirements into structured executable edge rules.
func (w *LogicWeaverEngine) CompileRule(ruleID, category, description, styleID string, paramA, paramB int) (RuleDefinition, error) {
	cat := strings.ToUpper(category)
	if cat != "ATTACHMENT" && cat != "DWELL" && cat != "TOLERANCE" {
		return RuleDefinition{}, fmt.Errorf("ERR_INVALID_RULE_CATEGORY: Category %s is not supported by edge core", category)
	}

	return RuleDefinition{
		RuleID:      ruleID,
		Category:    cat,
		Description: description,
		StyleID:     styleID,
		ParamA:      paramA,
		ParamB:      paramB,
		IsActive:    true,
	}, nil
}