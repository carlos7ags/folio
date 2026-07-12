// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// TestParseAngleWithCalc closes #274 for parseAngle. Pre-fix the
// function used strconv.ParseFloat directly on the value and so failed
// on any calc/min/max/clamp wrapper, returning 0. Post-fix it walks the
// expression as an angle-native tree.
func TestParseAngleWithCalc(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want float64
	}{
		// Plain leaves still work — regression guard for pre-existing behaviour.
		{"plain deg", "45deg", 45},
		{"plain rad", "1.5708rad", 90},
		{"plain grad", "100grad", 90},
		{"plain turn", "0.25turn", 90},
		{"bare number is degrees", "30", 30},
		{"empty string is zero", "", 0},

		// calc with same-unit arithmetic.
		{"calc(deg + deg)", "calc(45deg + 45deg)", 90},
		{"calc(deg - deg)", "calc(180deg - 90deg)", 90},
		{"calc(deg * num)", "calc(45deg * 2)", 90},
		{"calc(num * deg)", "calc(2 * 45deg)", 90},
		{"calc(deg / num)", "calc(180deg / 2)", 90},

		// Mixed-unit calc — each leaf converts to degrees first.
		{"calc(deg + rad)", "calc(45deg + 0.7854rad)", 90},
		{"calc(turn - deg)", "calc(1turn - 270deg)", 90},
		{"calc(grad + deg)", "calc(50grad + 45deg)", 90},

		// Nested calc.
		{"nested calc", "calc((30deg + 30deg) * 2)", 120},
		{"calc with sub-expression", "calc(45deg * (1 + 1))", 90},

		// min / max.
		{"min picks smallest", "min(45deg, 90deg, 30deg)", 30},
		{"max picks largest", "max(45deg, 90deg, 30deg)", 90},
		{"min with calc inside", "min(calc(60deg + 30deg), 60deg)", 60},

		// clamp.
		{"clamp middle", "clamp(0deg, 45deg, 90deg)", 45},
		{"clamp clipped low", "clamp(50deg, 30deg, 90deg)", 50},
		{"clamp clipped high", "clamp(0deg, 100deg, 90deg)", 90},

		// Defensive cases — degrades to 0 rather than crashing.
		{"calc with malformed leaf", "calc(45xyz + 0)", 0},
		{"divide by zero", "calc(45deg / 0)", 0},
		{"clamp wrong arity", "clamp(0deg, 45deg)", 0},

		// Negative angles (CSS rotate accepts these for counter-rotation).
		{"plain negative deg", "-45deg", -45},
		{"calc producing negative", "calc(0deg - 90deg)", -90},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseAngle(tc.val)
			if math.Abs(got-tc.want) > 0.001 {
				t.Errorf("parseAngle(%q) = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}

// TestParseNumericValWithCalc closes #274 for parseNumericVal. Pre-fix
// the function only handled strconv.ParseFloat and returned 0 for any
// calc wrapper, silently making `scale(calc(0.5 + 0.5))` a degenerate
// zero-scale transform. Post-fix it delegates to parseLength for calc
// inputs whose leaves are dimensionless ("num").
func TestParseNumericValWithCalc(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want float64
	}{
		// Plain numbers still work.
		{"plain integer", "5", 5},
		{"plain float", "1.5", 1.5},
		{"negative", "-2.5", -2.5},
		{"zero", "0", 0},
		{"empty string is zero", "", 0},

		// calc with dimensionless leaves.
		{"calc add", "calc(0.5 + 0.5)", 1.0},
		{"calc subtract", "calc(2 - 0.5)", 1.5},
		{"calc multiply", "calc(0.5 * 3)", 1.5},
		{"calc divide", "calc(3 / 2)", 1.5},
		{"calc with parens on right", "calc(2 * (1 + 1))", 4},

		// min / max / clamp on dimensionless.
		{"min", "min(2, 1.5, 3)", 1.5},
		{"max", "max(2, 1.5, 3)", 3},
		{"clamp middle", "clamp(0, 1.5, 2)", 1.5},
		{"clamp clipped low", "clamp(2, 1, 3)", 2},

		// Defensive.
		{"calc divide by zero", "calc(5 / 0)", 0},
		{"calc with garbage", "calc(xyz)", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseNumericVal(tc.val)
			if math.Abs(got-tc.want) > 0.001 {
				t.Errorf("parseNumericVal(%q) = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}

// TestCSSLengthIsDimensionless covers the new helper used by
// parseLineHeight (#275) to distinguish a calc that resolves to a
// dimensionless multiplier (every leaf is "num") from one that
// resolves to a length.
func TestCSSLengthIsDimensionless(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		{"plain px is length", "16px", false},
		{"plain percent is length", "50%", false},
		{"plain em is length", "1.5em", false},
		{"plain bare number is length (parsePlainLength tags as px)", "1.5", false},
		{"calc all numeric", "calc(1.2 * 1.5)", true},
		{"calc one length", "calc(1em + 4px)", false},
		{"calc number plus number", "calc(2 + 3)", true},
		{"calc number times length is length", "calc(2 * 1em)", false},
		// parseMathFuncArgs goes through parseLength, which tags bare-number
		// math args as "px" (the right default for length contexts). So
		// min/max/clamp of bare numbers are NOT dimensionless from
		// isDimensionless's perspective. parseNumericVal routes around this
		// by handling min/max/clamp itself.
		{"min of bare numbers tagged px", "min(1, 2, 3)", false},
		{"min mixed", "min(1, 2px)", false},
		{"max of bare numbers tagged px", "max(1, 2, 3)", false},
		{"clamp of bare numbers tagged px", "clamp(1, 2, 3)", false},
		{"clamp with one length", "clamp(0, 1.5, 100%)", false},
		{"nil cssLength is not dimensionless", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := parseLength(tc.val)
			got := l.isDimensionless()
			if got != tc.want {
				t.Errorf("parseLength(%q).isDimensionless() = %v, want %v", tc.val, got, tc.want)
			}
		})
	}
}

// TestScaleAndSkewCalcResolveCorrectly is an end-to-end check on
// parseTransform that scale/scaleX/scaleY/skew/skewX/skewY now resolve
// calc arguments via the fixed parseAngle / parseNumericVal. Sibling
// to TestParseTransformWithCalc which covers translate/rotate.
func TestScaleAndSkewCalcResolveCorrectly(t *testing.T) {
	tests := []struct {
		name      string
		val       string
		opType    string
		valuesIdx int     // 0 or 1
		want      float64 // expected at valuesIdx
	}{
		{"scale(calc(0.5 + 0.5))", "scale(calc(0.5 + 0.5))", "scale", 0, 1.0},
		{"scaleX(calc(2 / 4))", "scaleX(calc(2 / 4))", "scale", 0, 0.5},
		{"scaleY(calc(0.25 + 0.25))", "scaleY(calc(0.25 + 0.25))", "scale", 1, 0.5},
		{"skew(calc(10deg * 2))", "skew(calc(10deg * 2))", "skewX", 0, 20},
		{"skewX(calc(45deg / 2))", "skewX(calc(45deg / 2))", "skewX", 0, 22.5},
		{"skewY(calc(0.5turn))", "skewY(calc(0.5turn))", "skewY", 0, 180},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ops := parseTransform(tc.val)
			if len(ops) == 0 {
				t.Fatalf("parseTransform(%q) returned no ops", tc.val)
			}
			if ops[0].Type != tc.opType {
				t.Fatalf("ops[0].Type = %q, want %q", ops[0].Type, tc.opType)
			}
			if math.Abs(ops[0].Values[tc.valuesIdx]-tc.want) > 0.01 {
				t.Errorf("ops[0].Values[%d] = %v, want %v", tc.valuesIdx, ops[0].Values[tc.valuesIdx], tc.want)
			}
		})
	}
}

// TestParseLineHeight pins parseLineHeight against each input form
// CSS Inline Layout Module Level 3 §4.3 accepts. The "dimensionless
// calc" cases are the central regression bar for #275 — pre-fix
// `calc(1.2 * 1.5)` resolved to 1.8 then was divided by fontSize=9,
// producing a 9× compression of line spacing. The length-form calc
// cases (`calc(1.5em)`, `calc(24px)`) must still divide by fontSize;
// without the explicit length-form coverage a regression that drops
// the divide-by-fontSize would mis-render existing documents.
func TestParseLineHeight(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		fontSize float64
		want     float64
	}{
		// Keyword + simple forms.
		{"normal keyword", "normal", 12, layout.LeadingNormal},
		{"empty string falls back to normal", "", 12, layout.LeadingNormal},
		{"bare unitless number", "1.5", 12, 1.5},
		{"zero collapses the line box", "0", 12, 0},

		// Percentage form — CSS Inline Layout L3 §4.3 lists this
		// alongside <number> / <length>. A percent is resolved against
		// fontSize, so the multiplier equals the percent / 100.
		{"percentage 150% (multiplier 1.5)", "150%", 12, 1.5},
		{"percentage 50% (multiplier 0.5)", "50%", 12, 0.5},

		// Dimensionless calc — every leaf is a bare number. The result
		// IS the multiplier; dividing by fontSize would mis-render
		// (the original #275 bug).
		{"dimensionless calc — multiplication", "calc(1.2 * 1.5)", 12, 1.8},
		{"dimensionless calc — addition", "calc(1 + 0.5)", 9, 1.5},
		{"dimensionless calc — single leaf", "calc(1.5)", 12, 1.5},
		{"dimensionless calc — zero factor", "calc(0 * 5)", 12, 0},
		// CSS-invalid (negative line-height) but the parser tolerates
		// it; pin the lenient behavior so a future strictness change
		// is intentional rather than an accident.
		{"dimensionless calc — negative result", "calc(0.5 - 1)", 12, -0.5},

		// Length-form calc — at least one leaf carries a length unit.
		// The result is in points; the multiplier is points / fontSize.
		// Using pt explicitly avoids coupling the assertion to folio's
		// px→pt conversion factor (1px = 0.75pt at 96dpi).
		{"length-form calc with em", "calc(1.5em)", 12, 1.5},
		{"length-form calc with pt", "calc(18pt)", 12, 1.5},

		// Documenting test: min/max args route through parseLength,
		// which tags bare numbers as "px" (folio's default-unit
		// convention). So min(1.2, 1.8) is a length min, not a
		// dimensionless min. Result: 1.2px = 0.9pt → 0.9/12 = 0.075.
		// CSS spec rejects bare numbers in min/max anyway; pinning
		// the lenient folio behavior keeps a future "treat bare
		// numbers as num inside min/max" refactor visible.
		{"min() of bare numbers — folio treats as px", "min(1.2, 1.8)", 12, 0.075},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseLineHeight(tc.value, tc.fontSize)
			if math.Abs(got-tc.want) > 0.001 {
				t.Errorf("parseLineHeight(%q, %v) = %v, want %v",
					tc.value, tc.fontSize, got, tc.want)
			}
		})
	}
}
