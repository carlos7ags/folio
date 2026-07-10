// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// TestParseBorderFullStyleKeywords pins that the shorthand parser
// accepts the full CSS Backgrounds L3 §4.1 set of border-style
// keywords. Pre-fix `groove`, `ridge`, `inset`, `outset` were
// rejected (kept the default "solid" string).
func TestParseBorderFullStyleKeywords(t *testing.T) {
	keywords := []string{"solid", "dashed", "dotted", "double", "none", "hidden",
		"groove", "ridge", "inset", "outset"}
	for _, kw := range keywords {
		t.Run(kw, func(t *testing.T) {
			_, st, _ := parseBorderFull("2px "+kw+" red", 12)
			if st != kw {
				t.Errorf("parseBorderFull style = %q, want %q", st, kw)
			}
		})
	}
}

// TestBeveledColorMatrix locks in the per-side, per-style color
// modulation table for the 3D border styles. groove/inset behave
// identically here (top/left dark, bottom/right light); same for
// ridge/outset (the inverse).
func TestBeveledColorMatrix(t *testing.T) {
	base := layout.RGB(0.5, 0.5, 0.5) // mid-gray so light/dark are obviously different

	tests := []struct {
		name      string
		side      borderSide
		sunken    bool // true for groove/inset, false for ridge/outset
		wantDark  bool // expected to be darker than base
		wantLight bool // expected to be lighter than base
	}{
		{"groove top → dark", borderTop, true, true, false},
		{"groove left → dark", borderLeft, true, true, false},
		{"groove bottom → light", borderBottom, true, false, true},
		{"groove right → light", borderRight, true, false, true},
		{"ridge top → light", borderTop, false, false, true},
		{"ridge left → light", borderLeft, false, false, true},
		{"ridge bottom → dark", borderBottom, false, true, false},
		{"ridge right → dark", borderRight, false, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := beveledColor(tc.side, base, tc.sunken)
			if tc.wantDark && got.R >= base.R {
				t.Errorf("expected darker than base, got R=%v vs base R=%v", got.R, base.R)
			}
			if tc.wantLight && got.R <= base.R {
				t.Errorf("expected lighter than base, got R=%v vs base R=%v", got.R, base.R)
			}
		})
	}
}

// TestBuildBorderForSide3DStyles verifies the per-side dispatch
// produces a SolidBorder with a modulated color (rather than the
// pre-fix "fall through to solid" behaviour that lost the modulation
// entirely).
func TestBuildBorderForSide3DStyles(t *testing.T) {
	base := layout.RGB(0.5, 0.5, 0.5)
	width := 4.0

	for _, st := range []string{"groove", "ridge", "inset", "outset"} {
		t.Run(st, func(t *testing.T) {
			top := buildBorderForSide(borderTop, width, st, base)
			bottom := buildBorderForSide(borderBottom, width, st, base)
			if top.Style != layout.BorderSolid {
				t.Errorf("3D border %s top.Style = %v, want BorderSolid", st, top.Style)
			}
			if top.Width != width || bottom.Width != width {
				t.Errorf("3D border %s widths = (%v, %v), want both %v", st, top.Width, bottom.Width, width)
			}
			// Top and bottom must differ — that's the whole point of
			// the bevel. Pre-fix both got the same source color.
			if math.Abs(top.Color.R-bottom.Color.R) < 0.01 {
				t.Errorf("3D border %s top.R (%v) and bottom.R (%v) should differ — modulation not applied",
					st, top.Color.R, bottom.Color.R)
			}
		})
	}
}

// TestBuildBorderForSidePassthroughStyles guards against regression
// in the existing styles: solid / dashed / dotted / double / unknown
// should produce identical output to pre-fix, with Color untouched (no
// light/dark modulation) and the Style enum mapping to the existing
// layout primitives. hidden is a distinct case: it now maps to
// layout.BorderHidden (a border-collapse conflict must be able to tell
// "hidden" apart from "undeclared") and Color is irrelevant since a
// hidden border never draws.
func TestBuildBorderForSidePassthroughStyles(t *testing.T) {
	base := layout.RGB(0.2, 0.4, 0.6)
	w := 2.0

	tests := []struct {
		input string
		want  layout.BorderStyle
	}{
		{"solid", layout.BorderSolid},
		{"dashed", layout.BorderDashed},
		{"dotted", layout.BorderDotted},
		{"double", layout.BorderDouble},
		{"banana", layout.BorderSolid},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			b := buildBorderForSide(borderTop, w, tc.input, base)
			if b.Style != tc.want {
				t.Errorf("style %q produced layout.Style %v, want %v", tc.input, b.Style, tc.want)
			}
			if b.Color != base {
				t.Errorf("style %q modulated the color (%v vs base %v); should pass through", tc.input, b.Color, base)
			}
		})
	}

	t.Run("hidden", func(t *testing.T) {
		b := buildBorderForSide(borderTop, w, "hidden", base)
		if b.Style != layout.BorderHidden {
			t.Errorf("style %q produced layout.Style %v, want %v", "hidden", b.Style, layout.BorderHidden)
		}
	})
}

// TestBorderStyleEndToEndThroughConvert verifies the new keywords
// survive the full HTML → buildCellBorders → render pipeline. The
// assertion is bounded: we can confirm CellBorders exists and each
// side has a Border with the expected Width and Style — but the
// post-modulation Color is internal to the resolver. The unit tests
// above cover the modulation table.
func TestBorderStyleEndToEndThroughConvert(t *testing.T) {
	html := `<div style="border: 4px ridge silver; padding: 8px">x</div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("no elements")
	}
	// A bordered div emits a layout.Div with non-empty CellBorders.
	// We can't easily inspect Div.borders without an accessor, but
	// the test exists to lock in the contract that Convert() does
	// not error and that the new keyword survives the parser.
	// If a future change drops "ridge" from the parser whitelist,
	// this test would still pass — it's the unit tests that lock in
	// the visual semantic. This integration test is a smoke check
	// that the pipeline accepts the input without error.
	_ = elems
}
