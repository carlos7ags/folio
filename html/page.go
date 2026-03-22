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
// Supports pseudo-selectors: @page :first, @page :left, @page :right.
func parsePageConfig(rules []pageRule, defaultFontSize float64) *PageConfig {
	pc := &PageConfig{}
	hasAny := false

	for _, rule := range rules {
		// Determine which margin target to apply to based on selector.
		var target *PageMargins
		switch rule.selector {
		case "first":
			if pc.First == nil {
				pc.First = &PageMargins{}
			}
			target = pc.First
		case "left":
			if pc.Left == nil {
				pc.Left = &PageMargins{}
			}
			target = pc.Left
		case "right":
			if pc.Right == nil {
				pc.Right = &PageMargins{}
			}
			target = pc.Right
		}

		for _, d := range rule.declarations {
			prop := strings.TrimSpace(strings.ToLower(d.property))
			val := strings.TrimSpace(d.value)

			switch prop {
			case "size":
				parsePageSize(val, pc)
				hasAny = true
			case "margin":
				t, r, b, l := parseMarginShorthand(val, defaultFontSize)
				if target != nil {
					target.Top, target.Right, target.Bottom, target.Left = t, r, b, l
					target.HasMargins = true
				} else {
					pc.MarginTop, pc.MarginRight, pc.MarginBottom, pc.MarginLeft = t, r, b, l
					pc.HasMargins = true
				}
				hasAny = true
			case "margin-top":
				v := parseSingleLength(val, defaultFontSize)
				if target != nil {
					target.Top = v
					target.HasMargins = true
				} else {
					pc.MarginTop = v
					pc.HasMargins = true
				}
				hasAny = true
			case "margin-right":
				v := parseSingleLength(val, defaultFontSize)
				if target != nil {
					target.Right = v
					target.HasMargins = true
				} else {
					pc.MarginRight = v
					pc.HasMargins = true
				}
				hasAny = true
			case "margin-bottom":
				v := parseSingleLength(val, defaultFontSize)
				if target != nil {
					target.Bottom = v
					target.HasMargins = true
				} else {
					pc.MarginBottom = v
					pc.HasMargins = true
				}
				hasAny = true
			case "margin-left":
				v := parseSingleLength(val, defaultFontSize)
				if target != nil {
					target.Left = v
					target.HasMargins = true
				} else {
					pc.MarginLeft = v
					pc.HasMargins = true
				}
				hasAny = true
			}
		}

		// Extract margin box content.
		if len(rule.marginBoxes) > 0 {
			hasAny = true
			for boxName, boxDecls := range rule.marginBoxes {
				mbc := parseMarginBoxDecls(boxDecls, defaultFontSize)
				if mbc.Content == "" {
					continue
				}
				if target != nil {
					if target.MarginBoxes == nil {
						target.MarginBoxes = make(map[string]MarginBoxContent)
					}
					target.MarginBoxes[boxName] = mbc
				} else {
					if pc.MarginBoxes == nil {
						pc.MarginBoxes = make(map[string]MarginBoxContent)
					}
					pc.MarginBoxes[boxName] = mbc
				}
			}
		}
	}

	if !hasAny {
		return nil
	}
	return pc
}

// parseMarginBoxDecls extracts content, font-size, and color from margin box declarations.
func parseMarginBoxDecls(decls []cssDecl, defaultFontSize float64) MarginBoxContent {
	var mbc MarginBoxContent
	for _, d := range decls {
		prop := strings.TrimSpace(strings.ToLower(d.property))
		val := strings.TrimSpace(d.value)
		switch prop {
		case "content":
			mbc.Content = parseContentValue(val)
		case "font-size":
			if l := parseCSSLengthWithUnit(val); l != nil {
				mbc.FontSize = l.toPoints(0, defaultFontSize)
			}
		case "color":
			if c, ok := parseColor(val); ok {
				mbc.Color = [3]float64{c.R, c.G, c.B}
			}
		}
	}
	return mbc
}

// parseContentValue parses a CSS content value, supporting:
//   - quoted strings: "Page "
//   - counter(page), counter(pages)
//   - concatenation of the above
func parseContentValue(val string) string {
	val = strings.TrimSpace(val)
	if val == "none" || val == "normal" || val == "" {
		return ""
	}

	var result strings.Builder
	remaining := val
	for len(remaining) > 0 {
		remaining = strings.TrimSpace(remaining)
		if len(remaining) == 0 {
			break
		}
		// Quoted string.
		if remaining[0] == '"' || remaining[0] == '\'' {
			quote := remaining[0]
			end := strings.IndexByte(remaining[1:], quote)
			if end >= 0 {
				result.WriteString(remaining[1 : end+1])
				remaining = remaining[end+2:]
				continue
			}
		}
		// counter() function — stored as placeholder, resolved at render time.
		if strings.HasPrefix(remaining, "counter(") {
			closeIdx := strings.IndexByte(remaining, ')')
			if closeIdx >= 0 {
				fnCall := remaining[:closeIdx+1]
				result.WriteString("{" + fnCall + "}")
				remaining = remaining[closeIdx+1:]
				continue
			}
		}
		// Skip unknown tokens.
		spIdx := strings.IndexByte(remaining, ' ')
		if spIdx >= 0 {
			remaining = remaining[spIdx+1:]
		} else {
			break
		}
	}
	return result.String()
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
	// Special case: height of "0" means auto-height (size page to content).
	if len(parts) >= 2 {
		w := parseCSSLength(parts[0])
		h := parseCSSLength(parts[1])
		explicitZeroH := parts[1] == "0"
		if w > 0 && (h > 0 || explicitZeroH) {
			pc.Width = w
			pc.Height = h
			if explicitZeroH {
				pc.AutoHeight = true
			}
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

// parseFloat extracts a float64 from the numeric prefix of s.
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	var v float64
	for i, ch := range s {
		if ch == '.' || (i == 0 && ch == '-') {
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
