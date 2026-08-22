// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/internal/csscolor"
	"github.com/carlos7ags/folio/internal/cssunit"
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

	c, ok := csscolor.Parse(value)
	if !ok {
		return layout.Color{}, 0, false
	}
	if c.CMYK != nil {
		cmyk := *c.CMYK
		return layout.CMYK(cmyk[0], cmyk[1], cmyk[2], cmyk[3]), 1, true
	}
	return layout.RGB(c.R, c.G, c.B), c.A, true
}

// parseAspectRatio parses a CSS aspect-ratio value.
// Accepts: "16 / 9", "16/9", "1.778", "auto" (returns 0).
func parseAspectRatio(val string) float64 {
	val = strings.TrimSpace(val)
	if val == "" || val == "auto" || val == "none" {
		return 0
	}
	// Handle compound "auto <ratio>" form (CSS Sizing 4 §5.1.1):
	// use the ratio part, ignore auto keyword.
	val = strings.TrimPrefix(val, "auto ")
	val = strings.TrimSpace(val)
	if val == "" || val == "auto" {
		return 0
	}
	// Try "W / H" form.
	if slashIdx := strings.IndexByte(val, '/'); slashIdx >= 0 {
		wStr := strings.TrimSpace(val[:slashIdx])
		hStr := strings.TrimSpace(val[slashIdx+1:])
		w, errW := strconv.ParseFloat(wStr, 64)
		h, errH := strconv.ParseFloat(hStr, 64)
		if errW == nil && errH == nil && w > 0 && h > 0 {
			return w / h
		}
		return 0
	}
	// Try single number.
	if v, err := strconv.ParseFloat(val, 64); err == nil && v > 0 {
		return v
	}
	return 0
}

// parseColumnRule parses a CSS column-rule shorthand: "<width> <style> <color>".
func parseColumnRule(val string, fontSize float64) (float64, string, layout.Color) {
	// splitTopLevelFields keeps functional values intact (calc/min/max/
	// clamp for the width slot, rgb/rgba/hsl for the color slot) when
	// they contain internal whitespace.
	parts := splitTopLevelFields(strings.TrimSpace(strings.ToLower(val)))
	var width float64
	style := "solid"
	color := layout.ColorBlack
	for _, p := range parts {
		switch p {
		case "solid", "dashed", "dotted", "double", "none", "hidden",
			"groove", "ridge", "inset", "outset":
			style = p
		default:
			if c, ok := parseColor(p); ok {
				color = c
			} else if l := parseLength(p); l != nil {
				width = l.toPoints(0, fontSize)
			}
		}
	}
	// Per CSS Multi-column Layout L1, column-rule-style: none (or hidden)
	// computes column-rule-width to 0 — same rule as the border shorthand
	// (parseBorderFull below handles this identically).
	if style == "none" || style == "hidden" {
		width = 0
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

// parseRadiusComponent parses a single border-radius corner component and
// splits it into an absolute point length and a percentage fraction. A
// percentage value (e.g. "50%") returns (0, 0.5) so the caller can resolve it
// against the box width/height at layout time; any other length returns its
// resolved points and a 0 fraction. Invalid input returns (0, 0).
//
// Note: the CSS `border-radius: <h> / <v>` slash syntax (independent horizontal
// and vertical radii) is not handled here — that is deferred follow-up work.
func parseRadiusComponent(val string, fontSize float64) (abs float64, pct float64) {
	l := parseLength(val)
	if l == nil {
		return 0, 0
	}
	if l.Unit == "%" {
		return 0, l.Value / 100
	}
	return l.toPoints(0, fontSize), 0
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
	num, unit, ok := cssunit.Parse(value)
	if !ok {
		return nil
	}
	if unit == "" {
		unit = "px" // bare number — treat as px
	}
	return &cssLength{Value: num, Unit: unit}
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
	// Reset depth: the +/- scan above walks past every ')' and '(' in the
	// string, and the loop bound i > 0 skips index 0, so an opening paren
	// at s[0] is never matched and leaves depth at a stale non-zero value.
	// Without this reset, expressions like "(50% - 10%) * 2" hide their
	// top-level '*' behind a phantom paren depth and fail to parse.
	depth = 0
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

// percentFraction returns the [0..1] fraction value of a percent-only
// cssLength tree. Examples: 50% -> 0.5, calc(50% - 10%) -> 0.4,
// calc(50% * 2) -> 1.0, min(40%, 60%) -> 0.4.
//
// Returns (0, false) when the tree contains any leaf with a length unit
// (px, em, pt, rem, mm, cm, in) — mixed-dimension calc cannot be reduced
// to a fraction without knowing the resolution context (gradient line
// length or background box dimensions), which the position parsers do
// not have at parse time. The same restriction applies to plain length
// values such as 100px or 1em: the helper rejects them as not-a-fraction.
// True lazy resolution against gradient-line / background-box dimensions
// is deferred future work; see issues #265 and #266.
//
// Dimensionless leaves (Unit "num") are accepted so multipliers and
// divisors inside calc work, e.g. calc(50% * 2) and calc(60% / 2).
func percentFraction(l *cssLength) (float64, bool) {
	if l == nil {
		return 0, false
	}
	if !isPercentOnly(l) {
		return 0, false
	}
	// Resolve with relativeTo=100 so that a leaf "50%" evaluates to 50,
	// then divide by 100 to get the fraction. fontSize is irrelevant for
	// percent / num leaves and is passed as 0.
	return l.toPoints(100, 0) / 100, true
}

// isPercentOnly reports whether every leaf in the cssLength tree is
// either a percent ("%") or a dimensionless number ("num"). Used by
// percentFraction to gate length-aware reduction in contexts where the
// resolution dimension is unknown.
func isPercentOnly(l *cssLength) bool {
	if l == nil {
		return false
	}
	if l.calc != nil {
		return calcExprIsPercentOnly(l.calc)
	}
	if len(l.minArgs) > 0 {
		for _, a := range l.minArgs {
			if !isPercentOnly(a) {
				return false
			}
		}
		return true
	}
	if len(l.maxArgs) > 0 {
		for _, a := range l.maxArgs {
			if !isPercentOnly(a) {
				return false
			}
		}
		return true
	}
	return l.Unit == "%" || l.Unit == "num"
}

func calcExprIsPercentOnly(e *calcExpr) bool {
	if e == nil {
		return false
	}
	if e.leaf != nil {
		return isPercentOnly(e.leaf)
	}
	if e.left == nil || e.right == nil {
		return false
	}
	return calcExprIsPercentOnly(e.left) && calcExprIsPercentOnly(e.right)
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

// parseFontWeight resolves a CSS font-weight value to a position on the
// CSS Fonts L4 numeric ladder (100, 200, ..., 900). The keyword `normal`
// maps to 400, `bold` to 700. Numeric values are clamped to [100, 900].
// `bolder` and `lighter` resolve relative to the inherited weight per
// CSS Fonts L4 §3.1 — pass the parent's resolved weight as `inherited`
// (or 400 if there is no parent / it isn't yet computed). Unknown
// values fall back to `inherited`, matching the CSS spec's
// preserve-cascade semantics.
func parseFontWeight(value string, inherited int) int {
	value = strings.TrimSpace(strings.ToLower(value))
	if inherited == 0 {
		inherited = 400
	}
	switch value {
	case "normal":
		return 400
	case "bold":
		return 700
	case "bolder":
		return bolderWeight(inherited)
	case "lighter":
		return lighterWeight(inherited)
	}
	if n, err := strconv.Atoi(value); err == nil {
		if n < 1 {
			return inherited
		}
		if n < 100 {
			return 100
		}
		if n > 900 {
			return 900
		}
		return n
	}
	return inherited
}

// bolderWeight returns the resolved weight for `font-weight: bolder` per
// CSS Fonts L4 §3.1's bolder/lighter table.
func bolderWeight(inherited int) int {
	switch {
	case inherited < 350:
		return 400
	case inherited < 550:
		return 700
	case inherited < 750:
		return 900
	default:
		return 900
	}
}

// lighterWeight returns the resolved weight for `font-weight: lighter`
// per the same table.
func lighterWeight(inherited int) int {
	switch {
	case inherited < 350:
		return 100
	case inherited < 550:
		return 100
	case inherited < 750:
		return 400
	default:
		return 700
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
//
// `start` and `end` are direction-relative keywords (CSS Text L4 §7.1)
// — they resolve to `left`/`right` based on the cascaded `direction`
// property. Direction may not yet be applied at the time text-align is
// parsed (CSS declarations are processed in source order within a
// block), so this function returns an LTR-correct best guess
// (`start` → AlignLeft, `end` → AlignRight) and a non-empty `keyword`
// string. Consumers must call resolveTextAlign(style) at draw time
// to get the spec-correct value once `style.Direction` is known.
//
// For `left`/`right`/`center`/`justify` the returned `keyword` is
// empty and the returned Align is the final value — no late binding
// needed.
//
// Returns (align, keyword, ok). `ok` is false for unrecognized
// values, leaving the caller's TextAlign unchanged per the CSS
// invalid-value cascade rule.
func parseTextAlign(value string) (align layout.Align, keyword string, ok bool) {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "left":
		return layout.AlignLeft, "", true
	case "center":
		return layout.AlignCenter, "", true
	case "right":
		return layout.AlignRight, "", true
	case "justify":
		return layout.AlignJustify, "", true
	case "start":
		return layout.AlignLeft, "start", true
	case "end":
		return layout.AlignRight, "end", true
	default:
		return layout.AlignLeft, "", false
	}
}

// resolveTextAlign returns the spec-correct text-align value for a
// computed style, late-binding the direction-relative keywords
// `start` and `end` against `style.Direction`. Other keywords pass
// through `style.TextAlign` unchanged.
//
// `start` resolves to AlignLeft under LTR / Auto and AlignRight under
// RTL. `end` resolves to AlignRight under LTR / Auto and AlignLeft
// under RTL.
func resolveTextAlign(style computedStyle) layout.Align {
	return resolveDirectionRelativeAlign(style.TextAlignKeyword, style.TextAlign, style.Direction)
}

// resolveTextAlignLast does the same late binding for
// `text-align-last`, which has the same direction-relative keywords.
func resolveTextAlignLast(style computedStyle) layout.Align {
	return resolveDirectionRelativeAlign(style.TextAlignLastKeyword, style.TextAlignLast, style.Direction)
}

// resolveDirectionRelativeAlign maps the CSS direction-relative
// keywords (`start`/`end`) to a concrete left/right alignment given
// the computed direction. Any other keyword (including the empty
// string) returns `fallback` unchanged.
func resolveDirectionRelativeAlign(keyword string, fallback layout.Align, dir layout.Direction) layout.Align {
	switch keyword {
	case "start":
		if dir == layout.DirectionRTL {
			return layout.AlignRight
		}
		return layout.AlignLeft
	case "end":
		if dir == layout.DirectionRTL {
			return layout.AlignLeft
		}
		return layout.AlignRight
	default:
		return fallback
	}
}

// parseTextDecoration parses CSS text-decoration (the
// text-decoration-line subproperty in CSS Text Decoration L4) into
// layout.TextDecoration. Multiple keywords combine — `underline
// overline` produces a bitset with both flags set. The `blink`
// keyword is recognised as a no-op (PDFs are static); `none`
// returns DecorationNone.
func parseTextDecoration(value string) layout.TextDecoration {
	value = strings.TrimSpace(strings.ToLower(value))
	var dec layout.TextDecoration
	if strings.Contains(value, "underline") {
		dec |= layout.DecorationUnderline
	}
	if strings.Contains(value, "overline") {
		dec |= layout.DecorationOverline
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

// parseBoxSide parses a single length value into points, eagerly
// resolving percent against zero. Used by border-width / outline-
// width / column-gap / row-gap / etc. — Apply sites that store a
// float64 and don't need layout-time re-resolution. Margin / padding
// Apply sites use parseBoxSideLength so the *cssLength is preserved
// for resolution against the containing block at consumer time
// (#269).
func parseBoxSide(value string, fontSize float64) float64 {
	l := parseLength(value)
	if l == nil {
		return 0
	}
	return l.toPoints(0, fontSize)
}

// parseBoxSideLength parses a single side of margin/padding and
// returns the unresolved *cssLength so the percent / calc / min /
// max / clamp tree can be resolved against the containing block at
// consumer time (#269). A nil parseLength result (e.g. "auto" or
// unparseable input) yields nil.
//
// The fontSize parameter is accepted for symmetry with parseBoxSide
// (which resolves px / em eagerly) but is not currently consulted —
// parseLength only inspects the value's unit token. It's kept on the
// signature so a future change that needs fontSize at parse time
// doesn't require a call-site sweep.
func parseBoxSideLength(value string, fontSize float64) *cssLength {
	_ = fontSize
	return parseLength(value)
}

// parseMarginShorthandLengths parses the CSS margin/padding shorthand
// into four *cssLength values (top/right/bottom/left) using the
// standard 1/2/3/4-token expansion rules. The *cssLength preserves
// percent / calc / min / max / clamp trees for layout-time
// resolution against the containing block.
func parseMarginShorthandLengths(value string, fontSize float64) (*cssLength, *cssLength, *cssLength, *cssLength) {
	parts := splitTopLevelFields(value)
	switch len(parts) {
	case 1:
		v := parseBoxSideLength(parts[0], fontSize)
		return v, v, v, v
	case 2:
		tb := parseBoxSideLength(parts[0], fontSize)
		lr := parseBoxSideLength(parts[1], fontSize)
		return tb, lr, tb, lr
	case 3:
		tt := parseBoxSideLength(parts[0], fontSize)
		lr := parseBoxSideLength(parts[1], fontSize)
		bb := parseBoxSideLength(parts[2], fontSize)
		return tt, lr, bb, lr
	case 4:
		tt := parseBoxSideLength(parts[0], fontSize)
		rr := parseBoxSideLength(parts[1], fontSize)
		bb := parseBoxSideLength(parts[2], fontSize)
		ll := parseBoxSideLength(parts[3], fontSize)
		return tt, rr, bb, ll
	default:
		return nil, nil, nil, nil
	}
}

// parseMarginShorthand parses the CSS margin/padding shorthand.
// Returns top, right, bottom, left in points.
func parseMarginShorthand(value string, fontSize float64) (float64, float64, float64, float64) {
	// splitTopLevelFields keeps calc()/min()/max()/clamp() values as a
	// single token even when they contain internal whitespace.
	parts := splitTopLevelFields(value)
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
	// splitTopLevelFields keeps functional values (calc()/min()/max()/clamp()
	// for width, rgb()/rgba()/hsl() for color) intact when they contain
	// internal whitespace.
	parts := splitTopLevelFields(value)
	if len(parts) == 0 {
		return 0, "none", layout.ColorBlack
	}

	width := 0.75 // default thin
	style := "solid"
	color := layout.ColorBlack
	foundWidth := false

	for _, p := range parts {
		pl := strings.ToLower(p)
		// Check for style keywords. groove/ridge/inset/outset are
		// per-side beveled styles handled in buildBorderForSide; the
		// shorthand parser stores the keyword verbatim and the
		// per-side renderer dispatches to lighten/darken modulation.
		switch pl {
		case "solid", "dashed", "dotted", "double", "none", "hidden",
			"groove", "ridge", "inset", "outset":
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
// Returns style, weight, size, lineHeight, family. weight=0 means
// "unset" (caller keeps the inherited value). Other unset values return
// the zero value of their type (empty string or 0 float).
func parseFontShorthand(value string, parentSize float64) (style string, weight int, size, lineHeight float64, family string) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", 0, parentSize, 0, ""
	}

	// splitTopLevelFields keeps calc()/min()/max()/clamp() values as a
	// single token even when they contain internal whitespace. The
	// font-family tail is rejoined below with strings.Join, so multi-word
	// families like "Helvetica Neue" survive.
	parts := splitTopLevelFields(value)
	if len(parts) == 0 {
		return "", 0, parentSize, 0, ""
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

	// Optional font-weight. parseFontWeight needs an inherited weight
	// to resolve bolder/lighter; the shorthand parser doesn't have
	// access to the cascade context, so it passes 400. Inside the
	// `font:` shorthand bolder/lighter relative to 400 yield 700/100,
	// which matches what most callers want.
	if idx < len(parts) {
		switch strings.ToLower(parts[idx]) {
		case "bold", "bolder", "lighter", "100", "200", "300", "400", "500", "600", "700", "800", "900":
			weight = parseFontWeight(parts[idx], 400)
			idx++
		case "normal":
			idx++ // could be weight or style; skip
		}
	}

	// Required: font-size (possibly with /line-height).
	// indexByteAtTopLevel skips slashes inside parens, so a calc(2em / 2)
	// size or a 12px/calc(1.2 * 1.5) line-height does not get split
	// mid-calc.
	if idx < len(parts) {
		sizeStr := parts[idx]
		idx++
		if slashIdx := indexByteAtTopLevel(sizeStr, '/'); slashIdx >= 0 {
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

// parseLineHeight parses CSS line-height into a multiplier, or
// [layout.LeadingNormal] for the `normal` keyword (and any unparseable
// value, which browsers treat as if the property were never set).
//
// Per CSS Inline Layout Module Level 3 §4.3, line-height accepts:
//   - `normal` keyword (resolved from the font's own vertical metrics —
//     see [layout.LeadingNormal] — not a flat multiplier)
//   - a unitless `<number>` used directly as a multiplier
//   - a `<length>` whose multiplier is length/fontSize
//   - a `<percentage>` resolved against fontSize (so 150% → 1.5)
//
// calc() can produce either a length OR a unitless multiplier depending
// on its leaves; the two compose differently. A calc whose leaves are
// all dimensionless (e.g. `calc(1.2 * 1.5)`) is itself dimensionless and
// is used as a direct multiplier. A calc with a length leaf
// (e.g. `calc(1em + 4px)`) resolves to a length and is divided by
// fontSize. Without this distinction `calc(1.2 * 1.5)` would
// pass through `parseLength` as a dimensionless cssLength, and
// dividing the resolved value (1.8) by fontSize would produce
// a 9× compression of line spacing.
func parseLineHeight(value string, fontSize float64) float64 {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "normal" || value == "" {
		return layout.LeadingNormal
	}

	// Unitless number — direct multiplier.
	if num, err := strconv.ParseFloat(value, 64); err == nil {
		return num
	}

	// Length or calc value.
	l := parseLength(value)
	if l != nil {
		if l.isDimensionless() {
			// A dimensionless calc result is the multiplier directly;
			// fontSize/relativeTo are unused by Unit "num" leaves so
			// any positive value works.
			return l.toPoints(0, 0)
		}
		pts := l.toPoints(fontSize, fontSize)
		if fontSize > 0 {
			return pts / fontSize
		}
	}
	return layout.LeadingNormal
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

	// Split on top-level whitespace so functional values like
	// calc(50% - 8px) or min(10px, 5%) survive as a single token.
	parts := splitTopLevelFields(val)

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
