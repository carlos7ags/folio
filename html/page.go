// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"

	"github.com/carlos7ags/folio/layout"
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
//
// Margin cascade: each pseudo target (:first/:left/:right) is seeded from the
// RESOLVED base @page {} margins BEFORE its own declarations are applied, so a
// side the pseudo does not specify inherits the base value (CSS cascade). For
// example, `@page { margin: 2cm } @page :first { margin-top: 4cm }` yields a
// first page with top=4cm and right/bottom/left=2cm — not 0.
//
// Two passes are required because a pseudo rule may appear in source before the
// base @page {} rule it inherits from: pass 1 resolves the base, pass 2 seeds
// and applies the pseudos.
func parsePageConfig(rules []pageRule, defaultFontSize float64) *PageConfig {
	pc := &PageConfig{fontSize: defaultFontSize}
	hasAny := false

	// Pass 1: resolve the base @page {} rule (selector == "") — size,
	// margins, and margin boxes. Named pages and unrecognised pseudos are
	// already dropped upstream in css.go so they cannot pollute the default.
	for _, rule := range rules {
		if rule.selector != "" {
			continue
		}
		for _, d := range rule.declarations {
			prop := strings.TrimSpace(strings.ToLower(d.property))
			val := strings.TrimSpace(d.value)
			switch prop {
			case "size":
				parsePageSize(val, pc, defaultFontSize)
				hasAny = true
			case "margin":
				// Parse both the eager floats (correct for absolute units)
				// and the unresolved length trees so percent / calc can be
				// resolved against the page box at apply time (B2/N1).
				t, r, b, l := parseMarginShorthand(val, defaultFontSize)
				pc.MarginTop, pc.MarginRight, pc.MarginBottom, pc.MarginLeft = t, r, b, l
				pc.marginTopLen, pc.marginRightLen, pc.marginBottomLen, pc.marginLeftLen = parseMarginShorthandLengths(val, defaultFontSize)
				pc.HasMargins = true
				hasAny = true
			case "margin-top":
				// Route through the calc-aware parser (N1) and keep the
				// length for page-box percent resolution (B2).
				pc.MarginTop = parseSingleLength(val, defaultFontSize)
				pc.marginTopLen = parseBoxSideLength(val, defaultFontSize)
				pc.HasMargins = true
				hasAny = true
			case "margin-right":
				pc.MarginRight = parseSingleLength(val, defaultFontSize)
				pc.marginRightLen = parseBoxSideLength(val, defaultFontSize)
				pc.HasMargins = true
				hasAny = true
			case "margin-bottom":
				pc.MarginBottom = parseSingleLength(val, defaultFontSize)
				pc.marginBottomLen = parseBoxSideLength(val, defaultFontSize)
				pc.HasMargins = true
				hasAny = true
			case "margin-left":
				pc.MarginLeft = parseSingleLength(val, defaultFontSize)
				pc.marginLeftLen = parseBoxSideLength(val, defaultFontSize)
				pc.HasMargins = true
				hasAny = true
			}
		}
		if len(rule.marginBoxes) > 0 {
			hasAny = true
			for boxName, boxDecls := range rule.marginBoxes {
				mbc := parseMarginBoxDecls(boxDecls, defaultFontSize)
				if mbc.Content == "" {
					continue
				}
				if pc.MarginBoxes == nil {
					pc.MarginBoxes = make(map[string]MarginBoxContent)
				}
				pc.MarginBoxes[boxName] = mbc
			}
		}
	}

	// Pass 2: pseudo selectors (:first/:left/:right). Each target inherits
	// the resolved base margins, then overlays its own declarations.
	for _, rule := range rules {
		var target *PageMargins
		switch rule.selector {
		case "first":
			if pc.First == nil {
				pc.First = newSeededMargins(pc)
			}
			target = pc.First
		case "left":
			if pc.Left == nil {
				pc.Left = newSeededMargins(pc)
			}
			target = pc.Left
		case "right":
			if pc.Right == nil {
				pc.Right = newSeededMargins(pc)
			}
			target = pc.Right
		default:
			continue
		}

		for _, d := range rule.declarations {
			prop := strings.TrimSpace(strings.ToLower(d.property))
			val := strings.TrimSpace(d.value)
			switch prop {
			case "margin":
				t, r, b, l := parseMarginShorthand(val, defaultFontSize)
				target.Top, target.Right, target.Bottom, target.Left = t, r, b, l
				target.topLen, target.rightLen, target.bottomLen, target.leftLen = parseMarginShorthandLengths(val, defaultFontSize)
				target.HasMargins = true
				hasAny = true
			case "margin-top":
				target.Top = parseSingleLength(val, defaultFontSize)
				target.topLen = parseBoxSideLength(val, defaultFontSize)
				target.HasMargins = true
				hasAny = true
			case "margin-right":
				target.Right = parseSingleLength(val, defaultFontSize)
				target.rightLen = parseBoxSideLength(val, defaultFontSize)
				target.HasMargins = true
				hasAny = true
			case "margin-bottom":
				target.Bottom = parseSingleLength(val, defaultFontSize)
				target.bottomLen = parseBoxSideLength(val, defaultFontSize)
				target.HasMargins = true
				hasAny = true
			case "margin-left":
				target.Left = parseSingleLength(val, defaultFontSize)
				target.leftLen = parseBoxSideLength(val, defaultFontSize)
				target.HasMargins = true
				hasAny = true
			}
		}

		// Extract margin box content for this pseudo target.
		//
		// Unlike the base @page rule, a pseudo box with empty resolved content
		// is PRESERVED when the `content` property was explicitly declared
		// (HasContent). Such a box (e.g. `@page :first { @bottom-center {
		// content: "" } }`) must enter the pseudo set so the per-slot merge in
		// the renderer lets it OVERRIDE — and thus blank — the inherited
		// default box for that page/slot. A box with no content declaration at
		// all (only font-size/color, say) and empty content is still dropped.
		if len(rule.marginBoxes) > 0 {
			hasAny = true
			for boxName, boxDecls := range rule.marginBoxes {
				mbc := parseMarginBoxDecls(boxDecls, defaultFontSize)
				if mbc.Content == "" && !mbc.HasContent {
					continue
				}
				if target.MarginBoxes == nil {
					target.MarginBoxes = make(map[string]MarginBoxContent)
				}
				target.MarginBoxes[boxName] = mbc
			}
		}
	}

	if !hasAny {
		return nil
	}
	return pc
}

// newSeededMargins creates a PageMargins for a pseudo target (:first/:left/
// :right) seeded with the resolved base @page {} margins. If the base set any
// margins, the seeded set is marked HasMargins so unspecified sides inherit the
// base instead of defaulting to 0 (CSS cascade — Defect B fix).
func newSeededMargins(pc *PageConfig) *PageMargins {
	pm := &PageMargins{fontSize: pc.fontSize}
	if pc.HasMargins {
		pm.Top = pc.MarginTop
		pm.Right = pc.MarginRight
		pm.Bottom = pc.MarginBottom
		pm.Left = pc.MarginLeft
		// Inherit the deferred length trees too, so a pseudo that does not
		// override a side resolves percent/calc base margins against the
		// page box (B2) rather than the basis-0 eager float.
		pm.topLen = pc.marginTopLen
		pm.rightLen = pc.marginRightLen
		pm.bottomLen = pc.marginBottomLen
		pm.leftLen = pc.marginLeftLen
		pm.HasMargins = true
	}
	return pm
}

// parseMarginBoxDecls extracts content, font-size, color, and font-style/
// font-weight/font-family from margin box declarations.
func parseMarginBoxDecls(decls []cssDecl, defaultFontSize float64) MarginBoxContent {
	var mbc MarginBoxContent
	for _, d := range decls {
		prop := strings.TrimSpace(strings.ToLower(d.property))
		val := strings.TrimSpace(d.value)
		switch prop {
		case "content":
			mbc.Content = parseContentValue(val)
			mbc.HasContent = true
		case "font-size":
			if l := parseCSSLengthWithUnit(val); l != nil {
				mbc.FontSize = l.toPoints(0, defaultFontSize)
			}
		case "color":
			if c, ok := parseColor(val); ok {
				mbc.Color = [3]float64{c.R, c.G, c.B}
				mbc.HasColor = true
			}
		case "font-style":
			mbc.FontStyle = parseFontStyle(val)
		case "font-weight":
			mbc.FontWeight = parseFontWeight(val, 400)
		case "font-family":
			mbc.FontFamily = parseFontFamily(val)
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
		// counter() function — stored as a placeholder, resolved at
		// render time. Supports an optional second argument naming a
		// list-style-type, e.g. counter(page, upper-roman). The style is
		// threaded through the placeholder so the renderer can format the
		// resolved page number accordingly (layout.formatCounter). Only
		// the reserved `page` / `pages` counters are deferred; any other
		// counter name is dropped (its value is not tracked here) rather
		// than leaking the literal call text into the PDF.
		if strings.HasPrefix(remaining, "counter(") {
			closeIdx := strings.IndexByte(remaining, ')')
			if closeIdx >= 0 {
				inner := remaining[len("counter("):closeIdx]
				name, style := inner, ""
				if comma := strings.IndexByte(inner, ','); comma >= 0 {
					name = inner[:comma]
					style = strings.TrimSpace(inner[comma+1:])
				}
				name = strings.TrimSpace(strings.ToLower(name))
				if name == "page" || name == "pages" {
					result.WriteString(layout.CounterPlaceholder(name, style))
				}
				remaining = remaining[closeIdx+1:]
				continue
			}
		}
		// string() function — references a CSS string-set value.
		// Stored as {string(name)} placeholder, resolved by renderer.
		if strings.HasPrefix(remaining, "string(") {
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
// Supports: "a4", "letter", "a4 landscape", "8.5in 11in", "210mm 297mm",
// and functional values like "calc(8in + 0.5in) 11in".
//
// fontSize is used to resolve em/rem inside calc() and is passed through
// to parseLengthPt (which routes through the main parseLength).
func parsePageSize(val string, pc *PageConfig, fontSize float64) {
	val = strings.ToLower(strings.TrimSpace(val))
	// splitTopLevelFields keeps calc()/min()/max()/clamp() values as a
	// single token even when they contain internal whitespace.
	parts := splitTopLevelFields(val)

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

	// Orientation only: "landscape" or "portrait". No explicit
	// dimensions are given, so the orientation must be applied to the
	// document default page size (done in document/html.go, where the
	// default is known). Flag it so that layer can swap width/height.
	if parts[0] == "landscape" || parts[0] == "portrait" {
		pc.OrientationOnly = true
		return
	}

	// Explicit dimensions: "8.5in 11in" or "210mm 297mm" or
	// "calc(8in + 0.5in) 11in". parseLengthPt routes through the
	// calc-aware parseLength in properties.go (NOT the page-local
	// parseSingleLength/parseCSSLengthWithUnit, which only strip unit
	// suffixes and don't understand functional values).
	// Special case: height of "0" means auto-height (size page to content).
	if len(parts) >= 2 {
		w := parseLengthPt(parts[0], fontSize)
		h := parseLengthPt(parts[1], fontSize)
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
		s := parseLengthPt(parts[0], fontSize)
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
