// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"
)

func TestParseCSSLength(t *testing.T) {
	tests := []struct {
		input      string
		fontSize   float64
		relativeTo float64
		want       float64
	}{
		// Absolute units.
		{"72pt", 12, 0, 72},
		{"1in", 12, 0, 72},
		{"25.4mm", 12, 0, 72},
		{"2.54cm", 12, 0, 72},

		// Pixels (96dpi screen → 72dpi print: 1px = 0.75pt).
		{"16px", 12, 0, 12},
		{"96px", 12, 0, 72},
		{"1px", 12, 0, 0.75},

		// Em (relative to fontSize).
		{"1em", 12, 0, 12},
		{"2em", 12, 0, 24},
		{"0.5em", 20, 0, 10},

		// Percentage (relative to relativeTo).
		{"50%", 12, 200, 100},
		{"100%", 12, 500, 500},
		{"25%", 12, 80, 20},

		// Zero.
		{"0", 12, 0, 0},
		{"0px", 12, 0, 0},

		// Empty / invalid.
		{"", 12, 0, 0},
		{"auto", 12, 0, 0},
		{"inherit", 12, 0, 0},
		{"notaunit", 12, 0, 0},

		// Rem (assumes 16px = 12pt root).
		{"1rem", 12, 0, 12},
		{"2rem", 12, 0, 24},

		// Negative values.
		{"-10px", 12, 0, -7.5},
		{"-1in", 12, 0, -72},

		// Decimal without leading zero.
		{".5em", 20, 0, 10},

		// Whitespace.
		{" 16px ", 12, 0, 12},
		{" ", 12, 0, 0},

		// Zero percent with nonzero relativeTo.
		{"0%", 12, 500, 0},

		// calc().
		{"calc(100% - 40px)", 12, 200, 170}, // 200 - 30pt

		// min/max.
		{"min(100px, 200px)", 12, 0, 75},  // min(75pt, 150pt) = 75pt
		{"max(100px, 200px)", 12, 0, 150}, // max(75pt, 150pt) = 150pt
	}
	for _, tt := range tests {
		got := ParseCSSLength(tt.input, tt.fontSize, tt.relativeTo)
		if math.Abs(got-tt.want) > 0.1 {
			t.Errorf("ParseCSSLength(%q, %g, %g) = %g, want %g", tt.input, tt.fontSize, tt.relativeTo, got, tt.want)
		}
	}
}
