// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"
)

// Standard page sizes in points (width x height, portrait).
var pageSizes = map[string][2]float64{
	"a3":      {841.89, 1190.55},
	"a4":      {595.28, 841.89},
	"a5":      {419.53, 595.28},
	"b4":      {708.66, 1000.63},
	"b5":      {498.90, 708.66},
	"letter":  {612, 792},
	"legal":   {612, 1008},
	"tabloid": {792, 1224},
	"ledger":  {1224, 792},
}

// parsePageConfig extracts page dimensions and margins from @page rules.
func parsePageConfig(rules []pageRule, defaultFontSize float64) *PageConfig {
	pc := &PageConfig{}
	hasAny := false

	for _, rule := range rules {
		for _, d := range rule.declarations {
			prop := strings.TrimSpace(strings.ToLower(d.property))
			val := strings.TrimSpace(d.value)

			switch prop {
			case "size":
				parsePageSize(val, pc)
				hasAny = true
			case "margin":
				t, r, b, l := parseMarginShorthand(val, defaultFontSize)
				pc.MarginTop = t
				pc.MarginRight = r
				pc.MarginBottom = b
				pc.MarginLeft = l
				hasAny = true
			case "margin-top":
				pc.MarginTop = parseSingleLength(val, defaultFontSize)
				hasAny = true
			case "margin-right":
				pc.MarginRight = parseSingleLength(val, defaultFontSize)
				hasAny = true
			case "margin-bottom":
				pc.MarginBottom = parseSingleLength(val, defaultFontSize)
				hasAny = true
			case "margin-left":
				pc.MarginLeft = parseSingleLength(val, defaultFontSize)
				hasAny = true
			}
		}
	}

	if !hasAny {
		return nil
	}
	return pc
}

// parsePageSize parses the CSS @page size property.
// Supports: "a4", "letter", "a4 landscape", "8.5in 11in", "210mm 297mm"
func parsePageSize(val string, pc *PageConfig) {
	val = strings.ToLower(strings.TrimSpace(val))
	parts := strings.Fields(val)

	if len(parts) == 0 {
		return
	}

	// Check for orientation keywords.
	for _, p := range parts {
		if p == "landscape" {
			pc.Landscape = true
		}
	}

	// Named size: "a4", "letter", etc.
	if size, ok := pageSizes[parts[0]]; ok {
		pc.Width = size[0]
		pc.Height = size[1]
		if pc.Landscape {
			pc.Width, pc.Height = pc.Height, pc.Width
		}
		return
	}

	// Orientation only: "landscape" or "portrait"
	if parts[0] == "landscape" || parts[0] == "portrait" {
		return // no dimensions, just orientation
	}

	// Explicit dimensions: "8.5in 11in" or "210mm 297mm"
	if len(parts) >= 2 {
		w := parseCSSLength(parts[0])
		h := parseCSSLength(parts[1])
		if w > 0 && h > 0 {
			pc.Width = w
			pc.Height = h
			if pc.Landscape {
				pc.Width, pc.Height = pc.Height, pc.Width
			}
		}
	} else if len(parts) == 1 {
		// Single dimension → square page
		s := parseCSSLength(parts[0])
		if s > 0 {
			pc.Width = s
			pc.Height = s
		}
	}
}

// parseSingleLength parses a CSS length value to points.
func parseSingleLength(val string, fontSize float64) float64 {
	l := parseCSSLengthWithUnit(val)
	if l == nil {
		return 0
	}
	return l.toPoints(0, fontSize)
}

// parseCSSLength parses a CSS length string (e.g. "8.5in", "210mm") to points.
func parseCSSLength(val string) float64 {
	val = strings.TrimSpace(strings.ToLower(val))

	if strings.HasSuffix(val, "in") {
		return parseFloat(strings.TrimSuffix(val, "in")) * 72
	}
	if strings.HasSuffix(val, "mm") {
		return parseFloat(strings.TrimSuffix(val, "mm")) * 72 / 25.4
	}
	if strings.HasSuffix(val, "cm") {
		return parseFloat(strings.TrimSuffix(val, "cm")) * 72 / 2.54
	}
	if strings.HasSuffix(val, "pt") {
		return parseFloat(strings.TrimSuffix(val, "pt"))
	}
	if strings.HasSuffix(val, "px") {
		return parseFloat(strings.TrimSuffix(val, "px")) * 0.75
	}

	// Bare number → assume px
	return parseFloat(val) * 0.75
}

// parseCSSLengthWithUnit parses a CSS length into a cssLength struct.
func parseCSSLengthWithUnit(val string) *cssLength {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "0" {
		return &cssLength{Value: 0, Unit: "pt"}
	}

	for _, unit := range []string{"rem", "em", "px", "pt", "mm", "cm", "in", "%"} {
		if strings.HasSuffix(val, unit) {
			num := parseFloat(strings.TrimSuffix(val, unit))
			switch unit {
			case "mm":
				return &cssLength{Value: num * 72 / 25.4, Unit: "pt"}
			case "cm":
				return &cssLength{Value: num * 72 / 2.54, Unit: "pt"}
			case "in":
				return &cssLength{Value: num * 72, Unit: "pt"}
			default:
				return &cssLength{Value: num, Unit: unit}
			}
		}
	}

	return nil
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	var v float64
	for i, ch := range s {
		if ch == '.' {
			continue
		}
		if ch < '0' || ch > '9' {
			s = s[:i]
			break
		}
	}
	fmt_Sscanf(s, &v)
	return v
}

// fmt_Sscanf is a minimal float parser to avoid importing fmt.
func fmt_Sscanf(s string, v *float64) {
	if s == "" {
		return
	}
	result := 0.0
	decimal := false
	divisor := 1.0
	negative := false
	for i, ch := range s {
		if i == 0 && ch == '-' {
			negative = true
			continue
		}
		if ch == '.' {
			decimal = true
			continue
		}
		if ch < '0' || ch > '9' {
			break
		}
		if decimal {
			divisor *= 10
			result += float64(ch-'0') / divisor
		} else {
			result = result*10 + float64(ch-'0')
		}
	}
	if negative {
		result = -result
	}
	*v = result
}
