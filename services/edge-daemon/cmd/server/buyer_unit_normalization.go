// Author: Sunil Jain, 2026-10-27
// Normalize garment BOM unit labels from heterogeneous sources into stable stock-unit codes.
// Quantity is already expressed per garment, so a source denominator such as /pcs
// is metadata, not a second dimension. Never guess conversions or unknown ratios.
package main

import (
	"fmt"
	"regexp"
	"strings"
)

var buyerUnitPer = regexp.MustCompile(`(?i)\s+per\s+`)
var buyerUnitCode = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_./-]*$`)

func canonicalBuyerUOM(source string) (string, error) {
	original := strings.TrimSpace(source)
	if original == "" {
		return "", fmt.Errorf("unit is missing; enter metres, pieces, yards, or another unit")
	}
	// Normalize only superficial typography. Do not change physical quantities.
	unit := strings.ToLower(strings.Join(strings.Fields(strings.ReplaceAll(original, "\u00a0", " ")), " "))
	unit = strings.ReplaceAll(unit, "\u2044", "/") // Unicode fraction slash
	unit = buyerUnitPer.ReplaceAllString(unit, "/")
	if strings.Contains(unit, "/") {
		parts := strings.Split(unit, "/")
		if len(parts) != 2 {
			return "", fmt.Errorf("unit %q is unclear; confirm the unit per garment", original)
		}
		denominator := strings.TrimSpace(parts[1])
		switch denominator {
		case "pc", "pcs", "piece", "pieces", "garment", "garments":
			unit = strings.TrimSpace(parts[0])
		default:
			return "", fmt.Errorf("unit %q is ambiguous; confirm the consumption unit per garment", original)
		}
	}
	switch strings.TrimSpace(unit) {
	case "m", "meter", "meters", "metre", "metres", "mtr", "mtrs", "m_per_pc", "m_per_pcs", "m_per_garment":
		return "M", nil
	case "cm", "centimeter", "centimeters", "centimetre", "centimetres":
		return "CM", nil
	case "mm", "millimeter", "millimeters", "millimetre", "millimetres":
		return "MM", nil
	case "yd", "yds", "yard", "yards":
		return "YD", nil
	case "in", "inch", "inches":
		return "IN", nil
	case "ft", "feet", "foot":
		return "FT", nil
	case "kg", "kilogram", "kilograms":
		return "KG", nil
	case "g", "gram", "grams":
		return "G", nil
	case "pc", "pcs", "piece", "pieces", "each", "ea", "unit", "units":
		return "PCS", nil
	case "pair", "pairs", "pr":
		return "PAIR", nil
	case "set", "sets":
		return "SET", nil
	case "l", "liter", "liters", "litre", "litres":
		return "L", nil
	case "roll", "rolls":
		return "ROLL", nil
	case "cone", "cones":
		return "CONE", nil
	}
	// A new factory-specific code is allowed without a schema change, but
	// never convert unfamiliar punctuation or dimensional ratios silently.
	code := strings.ToUpper(strings.TrimSpace(unit))
	if len(code) == 0 || len(code) > 16 || !buyerUnitCode.MatchString(code) {
		return "", fmt.Errorf("unit %q is not recognized; use a short unit code or ask Aggie to clarify it", original)
	}
	return code, nil
}
