// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package cssunit tokenizes CSS/SVG length values into a numeric value
// and a unit suffix. It has no opinion on unit conversion — the html
// package converts to PDF points with its own policy, and the svg
// package treats every unit as an SVG user unit — so callers apply
// their own conversion after tokenizing.
package cssunit

import (
	"strconv"
	"strings"
)

// units is checked in order; "rem" must precede "em" since "1rem" also
// ends in the two characters "em".
var units = []string{"px", "pt", "rem", "em", "mm", "cm", "in", "%"}

// Parse splits a CSS/SVG length into numeric value and unit.
// Recognized units: "px", "pt", "rem", "em", "mm", "cm", "in", "%".
// A bare number returns unit "". Exactly one unit suffix is consumed;
// leftover characters make the parse fail (ok=false).
func Parse(s string) (value float64, unit string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, "", false
	}

	for _, u := range units {
		if strings.HasSuffix(s, u) {
			numStr := strings.TrimSpace(s[:len(s)-len(u)])
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, "", false
			}
			return num, u, true
		}
	}

	num, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, "", false
	}
	return num, "", true
}

// PointsPerUnit returns the multiplier converting one <unit> to PDF points
// at 96dpi CSS pixel density. fontSize is used for "em".
// Returns ok=false for "%" and "" (caller-specific policies).
func PointsPerUnit(unit string, fontSize float64) (factor float64, ok bool) {
	switch unit {
	case "pt":
		return 1, true
	case "px":
		return 0.75, true
	case "em":
		return fontSize, true
	case "rem":
		return 16 * 0.75, true
	case "mm":
		return 72 / 25.4, true
	case "cm":
		return 72 / 2.54, true
	case "in":
		return 72, true
	default:
		return 0, false
	}
}
