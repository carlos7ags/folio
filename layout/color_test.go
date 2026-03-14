// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"math"
	"testing"
)

func TestGray(t *testing.T) {
	c := Gray(0.5)
	if c.R != 0.5 || c.G != 0.5 || c.B != 0.5 {
		t.Errorf("Gray(0.5) = %+v, want {0.5, 0.5, 0.5}", c)
	}

	black := Gray(0)
	if black != ColorBlack {
		t.Errorf("Gray(0) should equal ColorBlack")
	}

	white := Gray(1)
	if white != ColorWhite {
		t.Errorf("Gray(1) should equal ColorWhite")
	}
}

func TestHex(t *testing.T) {
	tests := []struct {
		hex  string
		want Color
	}{
		{"#FF0000", ColorRed},
		{"FF0000", ColorRed},
		{"#0000FF", ColorBlue},
		{"#000000", ColorBlack},
		{"#FFFFFF", ColorWhite},
		{"#808080", Color{R: 128.0 / 255, G: 128.0 / 255, B: 128.0 / 255}},
		{"", ColorBlack},     // invalid
		{"#FFF", ColorBlack}, // too short
	}

	for _, tt := range tests {
		got := Hex(tt.hex)
		if !colorClose(got, tt.want) {
			t.Errorf("Hex(%q) = %+v, want %+v", tt.hex, got, tt.want)
		}
	}
}

func TestColorConstants(t *testing.T) {
	// Verify a few constants are non-zero (not accidentally all black).
	if ColorWhite.R != 1 || ColorWhite.G != 1 || ColorWhite.B != 1 {
		t.Errorf("ColorWhite is wrong: %+v", ColorWhite)
	}
	if ColorRed.R != 1 || ColorRed.G != 0 || ColorRed.B != 0 {
		t.Errorf("ColorRed is wrong: %+v", ColorRed)
	}
	if ColorGreen.G == 0 {
		t.Error("ColorGreen should have non-zero G")
	}
	if ColorOrange.R != 1 || ColorOrange.G == 0 {
		t.Errorf("ColorOrange is wrong: %+v", ColorOrange)
	}
}

func colorClose(a, b Color) bool {
	const eps = 0.01
	return math.Abs(a.R-b.R) < eps &&
		math.Abs(a.G-b.G) < eps &&
		math.Abs(a.B-b.B) < eps
}
