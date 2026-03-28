// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/layout"
)

// parseColor parses a CSS color value into a layout.Color.
// Supports: named colors, #RGB, #RRGGBB, #RGBA, #RRGGBBAA,
// rgb(r,g,b), rgba(r,g,b,a), hsl(h,s%,l%), hsla(h,s%,l%,a).
// Alpha is discarded — use parseColorAlpha when alpha is needed.
func parseColor(value string) (layout.Color, bool) {
	c, _, ok := parseColorAlpha(value)
	return c, ok
}

// parseColorAlpha parses a CSS color and returns the alpha component (0-1).
// Alpha defaults to 1.0 for formats that don't include it.
func parseColorAlpha(value string) (layout.Color, float64, bool) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "inherit" || value == "initial" || value == "transparent" {
		return layout.Color{}, 0, false
	}

	// Named color.
	if c, ok := cssColorNames[value]; ok {
		return c, 1, true
	}

	// Hex color: #RGB, #RGBA, #RRGGBB, #RRGGBBAA.
	if strings.HasPrefix(value, "#") {
		hex := value[1:]
		switch len(hex) {
		case 3:
			hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
			return layout.Hex(hex), 1, true
		case 4:
			// #RGBA
			a := hexVal(hex[3])*16 + hexVal(hex[3])
			hex = string([]byte{hex[0], hex[0], hex[1], hex[1], hex[2], hex[2]})
			return layout.Hex(hex), float64(a) / 255, true
		case 6:
			return layout.Hex(hex), 1, true
		case 8:
			// #RRGGBBAA
			a := hexVal(hex[6])*16 + hexVal(hex[7])
			return layout.Hex(hex[:6]), float64(a) / 255, true
		}
		return layout.Color{}, 0, false
	}

	// rgb(r, g, b) / rgba(r, g, b, a)
	if strings.HasPrefix(value, "rgb") {
		inner, ok := extractFuncArgs(value, "rgba(")
		if !ok {
			inner, ok = extractFuncArgs(value, "rgb(")
		}
		if ok {
			parts := strings.Split(inner, ",")
			if len(parts) >= 3 {
				r := parseColorComponent(strings.TrimSpace(parts[0]))
				g := parseColorComponent(strings.TrimSpace(parts[1]))
				b := parseColorComponent(strings.TrimSpace(parts[2]))
				a := 1.0
				if len(parts) >= 4 {
					if v, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err == nil {
						a = v
					}
				}
				return layout.RGB(r, g, b), a, true
			}
		}
		return layout.Color{}, 0, false
	}

	// cmyk(c, m, y, k) / device-cmyk(c, m, y, k)
	if strings.HasPrefix(value, "cmyk(") || strings.HasPrefix(value, "device-cmyk(") {
		prefix := "cmyk("
		if strings.HasPrefix(value, "device-cmyk(") {
			prefix = "device-cmyk("
		}
		inner, ok := extractFuncArgs(value, prefix)
		if ok {
			parts := strings.Split(inner, ",")
			if len(parts) >= 4 {
				c := parseCMYKComponent(strings.TrimSpace(parts[0]))
				m := parseCMYKComponent(strings.TrimSpace(parts[1]))
				y := parseCMYKComponent(strings.TrimSpace(parts[2]))
				k := parseCMYKComponent(strings.TrimSpace(parts[3]))
				return layout.CMYK(c, m, y, k), 1, true
			}
		}
		return layout.Color{}, 0, false
	}

	// hsl(h, s%, l%) / hsla(h, s%, l%, a)
	if strings.HasPrefix(value, "hsl") {
		inner, ok := extractFuncArgs(value, "hsla(")
		if !ok {
			inner, ok = extractFuncArgs(value, "hsl(")
		}
		if ok {
			parts := strings.Split(inner, ",")
			if len(parts) >= 3 {
				h := parseHue(strings.TrimSpace(parts[0]))
				s := parsePercent(strings.TrimSpace(parts[1]))
				l := parsePercent(strings.TrimSpace(parts[2]))
				a := 1.0
				if len(parts) >= 4 {
					if v, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64); err == nil {
						a = v
					}
				}
				r, g, b := hslToRGB(h, s, l)
				return layout.RGB(r, g, b), a, true
			}
		}
		return layout.Color{}, 0, false
	}

	return layout.Color{}, 0, false
}

// extractFuncArgs extracts the content inside a CSS function like "rgb(...)" or "rgba(...)".
func extractFuncArgs(value, prefix string) (string, bool) {
	if strings.HasPrefix(value, prefix) && strings.HasSuffix(value, ")") {
		return value[len(prefix) : len(value)-1], true
	}
	return "", false
}

// parseColorComponent parses an RGB color component (0-255 or percentage).
func parseColorComponent(s string) float64 {
	if strings.HasSuffix(s, "%") {
		v, _ := strconv.ParseFloat(s[:len(s)-1], 64)
		return v / 100
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v / 255
}

// parseHue parses a CSS hue value (degrees, 0-360).
func parseHue(s string) float64 {
	s = strings.TrimSuffix(s, "deg")
	v, _ := strconv.ParseFloat(s, 64)
	// Normalize to 0-360.
	v = v - float64(int(v/360))*360
	if v < 0 {
		v += 360
	}
	return v / 360 // return as 0-1
}

// parsePercent parses a percentage value like "50%".
func parsePercent(s string) float64 {
	s = strings.TrimSuffix(s, "%")
	v, _ := strconv.ParseFloat(s, 64)
	return v / 100
}

// hexVal returns the numeric value of a hex digit.
func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

// parseCMYKComponent parses a CMYK color component (0-1 or percentage).
func parseCMYKComponent(s string) float64 {
	if strings.HasSuffix(s, "%") {
		v, _ := strconv.ParseFloat(s[:len(s)-1], 64)
		return v / 100
	}
	v, _ := strconv.ParseFloat(s, 64)
	// Clamp to 0-1 range.
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// parseColumnRule parses a CSS column-rule shorthand: "<width> <style> <color>".
func parseColumnRule(val string, fontSize float64) (float64, string, layout.Color) {
	parts := strings.Fields(strings.TrimSpace(strings.ToLower(val)))
	var width float64
	style := "solid"
	color := layout.ColorBlack
	for _, p := range parts {
		switch p {
		case "solid", "dashed", "dotted", "double", "none":
			style = p
		default:
			if c, ok := parseColor(p); ok {
				color = c
			} else if l := parseLength(p); l != nil {
				width = l.toPoints(0, fontSize)
			}
		}
	}
	return width, style, color
}

// parseMathFuncArgs parses comma-separated arguments to min()/max()/clamp().
// Each argument can be a plain length or a calc() expression.
func parseMathFuncArgs(inner string) []*cssLength {
	parts := splitTopLevelCommas(inner)
	var args []*cssLength
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if l := parseLength(p); l != nil {
			args = append(args, l)
		}
	}
	return args
}

// parseLengthPt parses a CSS length value and returns points, or 0 if invalid.
func parseLengthPt(val string, fontSize float64) float64 {
	if l := parseLength(val); l != nil {
		return l.toPoints(0, fontSize)
	}
	return 0
}

// hslToRGB converts HSL values (each 0-1) to RGB values (each 0-1).
func hslToRGB(h, s, l float64) (r, g, b float64) {
	if s == 0 {
		return l, l, l
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	r = hueToRGB(p, q, h+1.0/3)
	g = hueToRGB(p, q, h)
	b = hueToRGB(p, q, h-1.0/3)
	return
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t++
	}
	if t > 1 {
		t--
	}
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 1.0/2:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	default:
		return p
	}
}

// parseLength parses a CSS length value like "12px", "1.5em", "50%", "10pt",
// or "calc(100% - 40px)". Returns nil if the value cannot be parsed.
func parseLength(value string) *cssLength {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" || value == "auto" || value == "inherit" || value == "initial" {
		return nil
	}

	// Handle calc() expressions.
	if strings.HasPrefix(value, "calc(") && strings.HasSuffix(value, ")") {
		inner := value[5 : len(value)-1]
		expr := parseCalcExpr(inner)
		if expr != nil {
			return &cssLength{calc: expr}
		}
		return nil
	}

	// Handle min(), max(), clamp() math functions.
	if strings.HasPrefix(value, "min(") && strings.HasSuffix(value, ")") {
		inner := value[4 : len(value)-1]
		args := parseMathFuncArgs(inner)
		if len(args) >= 2 {
			return &cssLength{minArgs: args}
		}
		return nil
	}
	if strings.HasPrefix(value, "max(") && strings.HasSuffix(value, ")") {
		inner := value[4 : len(value)-1]
		args := parseMathFuncArgs(inner)
		if len(args) >= 2 {
			return &cssLength{maxArgs: args}
		}
		return nil
	}
	if strings.HasPrefix(value, "clamp(") && strings.HasSuffix(value, ")") {
		inner := value[6 : len(value)-1]
		args := parseMathFuncArgs(inner)
		if len(args) == 3 {
			// clamp(min, preferred, max) = max(min, min(preferred, max))
			return &cssLength{maxArgs: []*cssLength{
				args[0],
				{minArgs: []*cssLength{args[1], args[2]}},
			}}
		}
		return nil
	}

	return parsePlainLength(value)
}

// parsePlainLength parses a simple CSS length (no calc).
func parsePlainLength(value string) *cssLength {
	value = strings.TrimSpace(value)
	for _, unit := range []string{"px", "pt", "em", "rem", "mm", "cm", "in", "%"} {
		if strings.HasSuffix(value, unit) {
			numStr := strings.TrimSpace(value[:len(value)-len(unit)])
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return nil
			}
			return &cssLength{Value: num, Unit: unit}
		}
	}

	// Bare number — treat as px.
	if num, err := strconv.ParseFloat(value, 64); err == nil {
		return &cssLength{Value: num, Unit: "px"}
	}

	return nil
}

// parseCalcExpr parses the inside of a calc() expression.
// Supports: lengths, +, -, *, / with correct precedence.
// Examples: "100% - 40px", "50% + 20px", "100% / 3"
func parseCalcExpr(s string) *calcExpr {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	// Find the last top-level + or - (lowest precedence, left-to-right).
	// CSS calc requires spaces around + and - operators.
	splitIdx := -1
	var splitOp calcOp
	depth := 0
	for i := len(s) - 1; i > 0; i-- {
		ch := s[i]
		switch ch {
		case ')':
			depth++
		case '(':
			depth--
		}
		if depth != 0 {
			continue
		}
		if (ch == '+' || ch == '-') && i > 0 && s[i-1] == ' ' {
			splitIdx = i
			if ch == '+' {
				splitOp = calcAdd
			} else {
				splitOp = calcSub
			}
			break
		}
	}

	if splitIdx > 0 {
		left := parseCalcExpr(s[:splitIdx-1])
		right := parseCalcExpr(s[splitIdx+1:])
		if left != nil && right != nil {
			return &calcExpr{left: left, op: splitOp, right: right}
		}
	}

	// Try * and / (higher precedence).
	for i := len(s) - 1; i > 0; i-- {
		ch := s[i]
		switch ch {
		case ')':
			depth++
		case '(':
			depth--
		}
		if depth != 0 {
			continue
		}
		if (ch == '*' || ch == '/') && i > 0 && s[i-1] == ' ' {
			left := parseCalcExpr(s[:i-1])
			right := parseCalcExpr(s[i+1:])
			if left != nil && right != nil {
				op := calcMul
				if ch == '/' {
					op = calcDiv
				}
				return &calcExpr{left: left, op: op, right: right}
			}
		}
	}

	// Nested parens.
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		return parseCalcExpr(s[1 : len(s)-1])
	}

	// Leaf: a length with units first, then bare number as dimensionless.
	l := parseCalcLeaf(s)
	if l != nil {
		return &calcExpr{leaf: l}
	}

	return nil
}

// parseCalcLeaf parses a leaf value inside calc().
// Unlike parsePlainLength, bare numbers are treated as dimensionless ("num")
// rather than defaulting to px. This is correct for calc() where bare numbers
// are used as multipliers/divisors.
func parseCalcLeaf(s string) *cssLength {
	s = strings.TrimSpace(s)

	// Try units first (px, pt, em, rem, %).
	for _, unit := range []string{"px", "pt", "em", "rem", "mm", "cm", "in", "%"} {
		if strings.HasSuffix(s, unit) {
			numStr := strings.TrimSpace(s[:len(s)-len(unit)])
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return nil
			}
			return &cssLength{Value: num, Unit: unit}
		}
	}

	// Bare number → dimensionless.
	if num, err := strconv.ParseFloat(s, 64); err == nil {
		return &cssLength{Value: num, Unit: "num"}
	}

	return nil
}

// parseFontSize parses a CSS font-size into points.
// Handles absolute keywords, lengths, and percentages.
func parseFontSize(value string, parentSize float64) float64 {
	value = strings.TrimSpace(strings.ToLower(value))

	// Absolute keywords.
	switch value {
	case "xx-small":
		return 7.5 // 10px * 0.75
	case "x-small":
		return 8.25 // 11px * 0.75
	case "small":
		return 9.75 // 13px * 0.75
	case "medium":
		return 12 // 16px * 0.75
	case "large":
		return 13.5 // 18px * 0.75
	case "x-large":
		return 18 // 24px * 0.75
	case "xx-large":
		return 24 // 32px * 0.75
	case "smaller":
		return parentSize * 0.833
	case "larger":
		return parentSize * 1.2
	}

	l := parseLength(value)
	if l == nil {
		return parentSize
	}
	return l.toPoints(parentSize, parentSize)
}

// parseFontWeight normalizes a CSS font-weight to "normal" or "bold".
func parseFontWeight(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "bold", "bolder":
		return "bold"
	case "700", "800", "900":
		return "bold"
	default:
		return "normal"
	}
}

// parseFontStyle normalizes a CSS font-style to "normal" or "italic".
func parseFontStyle(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "italic", "oblique":
		return "italic"
	default:
		return "normal"
	}
}

// parseTextAlign parses CSS text-align into layout.Align.
func parseTextAlign(value string) (layout.Align, bool) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "left":
		return layout.AlignLeft, true
	case "center":
		return layout.AlignCenter, true
	case "right":
		return layout.AlignRight, true
	case "justify":
		return layout.AlignJustify, true
	default:
		return layout.AlignLeft, false
	}
}

// parseTextDecoration parses CSS text-decoration into layout.TextDecoration.
func parseTextDecoration(value string) layout.TextDecoration {
	value = strings.TrimSpace(strings.ToLower(value))
	var dec layout.TextDecoration
	if strings.Contains(value, "underline") {
		dec |= layout.DecorationUnderline
	}
	if strings.Contains(value, "line-through") {
		dec |= layout.DecorationStrikethrough
	}
	return dec
}

// parseDisplay normalizes a CSS display value.
func parseDisplay(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "block", "inline", "flex", "grid", "none", "table", "table-row",
		"table-cell", "inline-block", "list-item":
		return value
	default:
		return "block"
	}
}

// parseBoxSide parses a single side of margin/padding (e.g. "10px").
func parseBoxSide(value string, fontSize float64) float64 {
	l := parseLength(value)
	if l == nil {
		return 0
	}
	return l.toPoints(0, fontSize)
}

// parseMarginShorthand parses the CSS margin/padding shorthand.
// Returns top, right, bottom, left in points.
func parseMarginShorthand(value string, fontSize float64) (float64, float64, float64, float64) {
	parts := strings.Fields(value)
	switch len(parts) {
	case 1:
		v := parseBoxSide(parts[0], fontSize)
		return v, v, v, v
	case 2:
		tb := parseBoxSide(parts[0], fontSize)
		lr := parseBoxSide(parts[1], fontSize)
		return tb, lr, tb, lr
	case 3:
		t := parseBoxSide(parts[0], fontSize)
		lr := parseBoxSide(parts[1], fontSize)
		b := parseBoxSide(parts[2], fontSize)
		return t, lr, b, lr
	case 4:
		t := parseBoxSide(parts[0], fontSize)
		r := parseBoxSide(parts[1], fontSize)
		b := parseBoxSide(parts[2], fontSize)
		l := parseBoxSide(parts[3], fontSize)
		return t, r, b, l
	default:
		return 0, 0, 0, 0
	}
}

// parseBorderShorthand extracts the width from a CSS border shorthand like "1px solid black".
func parseBorderShorthand(value string, fontSize float64) float64 {
	w, _, _ := parseBorderFull(value, fontSize)
	return w
}

// parseBorderFull parses a CSS border shorthand into width, style, and color.
func parseBorderFull(value string, fontSize float64) (float64, string, layout.Color) {
	parts := strings.Fields(value)
	if len(parts) == 0 {
		return 0, "none", layout.ColorBlack
	}

	width := 0.75 // default thin
	style := "solid"
	color := layout.ColorBlack
	foundWidth := false

	for _, p := range parts {
		pl := strings.ToLower(p)
		// Check for style keywords.
		switch pl {
		case "solid", "dashed", "dotted", "double", "none", "hidden":
			style = pl
			continue
		case "thin":
			width = 0.75
			foundWidth = true
			continue
		case "medium":
			width = 2.25
			foundWidth = true
			continue
		case "thick":
			width = 3.75
			foundWidth = true
			continue
		}
		// Check for length.
		if !foundWidth {
			if l := parseLength(p); l != nil {
				width = l.toPoints(0, fontSize)
				foundWidth = true
				continue
			}
		}
		// Check for color.
		if c, ok := parseColor(p); ok {
			color = c
		}
	}

	if style == "none" || style == "hidden" {
		width = 0
	}

	return width, style, color
}

// parseFontFamily normalizes a CSS font-family value by lowercasing,
// stripping quotes, and selecting the first family from a comma-separated
// list. The raw family name is preserved so that custom @font-face names
// are not lost. Standard font mapping happens later in resolveFont.
func parseFontFamily(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	// Strip quotes.
	value = strings.Trim(value, `"'`)
	// Select the first family in the list.
	if idx := strings.IndexByte(value, ','); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
		value = strings.Trim(value, `"'`)
	}
	return value
}

// mapToStandardFamily maps a CSS font-family name to one of the three
// standard PDF font families: "courier", "times", or "helvetica".
// This is used as the final fallback when no @font-face match is found.
func mapToStandardFamily(family string) string {
	switch {
	case strings.Contains(family, "courier") || strings.Contains(family, "monospace") || family == "mono":
		return "courier"
	case strings.Contains(family, "times") || strings.Contains(family, "serif") && !strings.Contains(family, "sans"):
		return "times"
	default:
		return "helvetica"
	}
}

// parseFontShorthand parses the CSS font shorthand property.
// Format: [style] [weight] size[/line-height] family
// Returns style, weight, size, lineHeight, family. Unset values return "".
func parseFontShorthand(value string, parentSize float64) (style, weight string, size, lineHeight float64, family string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", parentSize, 0, ""
	}

	parts := strings.Fields(value)
	if len(parts) == 0 {
		return "", "", parentSize, 0, ""
	}

	idx := 0

	// Optional font-style.
	if idx < len(parts) {
		switch strings.ToLower(parts[idx]) {
		case "italic", "oblique":
			style = parseFontStyle(parts[idx])
			idx++
		case "normal":
			idx++ // skip explicit normal
		}
	}

	// Optional font-weight.
	if idx < len(parts) {
		switch strings.ToLower(parts[idx]) {
		case "bold", "bolder", "lighter", "100", "200", "300", "400", "500", "600", "700", "800", "900":
			weight = parseFontWeight(parts[idx])
			idx++
		case "normal":
			idx++ // could be weight or style; skip
		}
	}

	// Required: font-size (possibly with /line-height).
	if idx < len(parts) {
		sizeStr := parts[idx]
		idx++
		if slashIdx := strings.IndexByte(sizeStr, '/'); slashIdx >= 0 {
			size = parseFontSize(sizeStr[:slashIdx], parentSize)
			lineHeight = parseLineHeight(sizeStr[slashIdx+1:], size)
		} else {
			size = parseFontSize(sizeStr, parentSize)
		}
	} else {
		size = parentSize
	}

	// Remaining parts are font-family.
	if idx < len(parts) {
		family = parseFontFamily(strings.Join(parts[idx:], " "))
	}

	return style, weight, size, lineHeight, family
}

// parseLineHeight parses CSS line-height into a multiplier.
func parseLineHeight(value string, fontSize float64) float64 {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "normal" || value == "" {
		return 1.2
	}

	// Unitless number — direct multiplier.
	if num, err := strconv.ParseFloat(value, 64); err == nil {
		return num
	}

	// Length value.
	l := parseLength(value)
	if l != nil {
		pts := l.toPoints(fontSize, fontSize)
		if fontSize > 0 {
			return pts / fontSize
		}
	}
	return 1.2
}

// parseFlexShorthand parses the CSS flex shorthand property.
// Syntax: flex: none | [ <flex-grow> <flex-shrink>? || <flex-basis> ]
// Common values: flex: 1, flex: none, flex: 0 1 auto, flex: 1 0 0
func parseFlexShorthand(val string, style *computedStyle) {
	val = strings.TrimSpace(strings.ToLower(val))

	switch val {
	case "none":
		// flex: none → flex: 0 0 auto
		style.FlexGrow = 0
		style.FlexShrink = 0
		return
	case "auto":
		// flex: auto → flex: 1 1 auto
		style.FlexGrow = 1
		style.FlexShrink = 1
		return
	case "initial":
		// flex: initial → flex: 0 1 auto
		style.FlexGrow = 0
		style.FlexShrink = 1
		return
	}

	parts := strings.Fields(val)

	switch len(parts) {
	case 1:
		// Single value: if numeric, it's flex-grow (with shrink=1, basis=0).
		if v, err := strconv.ParseFloat(parts[0], 64); err == nil {
			style.FlexGrow = v
			style.FlexShrink = 1
			style.FlexBasis = &cssLength{Value: 0, Unit: "px"}
		} else {
			// Must be flex-basis.
			style.FlexBasis = parseLength(parts[0])
		}
	case 2:
		// Two values: <flex-grow> <flex-shrink> or <flex-grow> <flex-basis>
		if grow, err := strconv.ParseFloat(parts[0], 64); err == nil {
			style.FlexGrow = grow
			if shrink, err2 := strconv.ParseFloat(parts[1], 64); err2 == nil {
				style.FlexShrink = shrink
			} else {
				style.FlexBasis = parseLength(parts[1])
			}
		}
	case 3:
		// Three values: <flex-grow> <flex-shrink> <flex-basis>
		if grow, err := strconv.ParseFloat(parts[0], 64); err == nil {
			style.FlexGrow = grow
		}
		if shrink, err := strconv.ParseFloat(parts[1], 64); err == nil {
			style.FlexShrink = shrink
		}
		style.FlexBasis = parseLength(parts[2])
	}
}

// parseFlexFlowShorthand parses the CSS flex-flow shorthand.
// Syntax: flex-flow: <flex-direction> || <flex-wrap>
func parseFlexFlowShorthand(val string, style *computedStyle) {
	parts := strings.Fields(strings.TrimSpace(strings.ToLower(val)))
	for _, p := range parts {
		switch p {
		case "row", "row-reverse", "column", "column-reverse":
			style.FlexDirection = p
		case "nowrap", "wrap", "wrap-reverse":
			style.FlexWrap = p
		}
	}
}
