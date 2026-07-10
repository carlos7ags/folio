// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package cssunit

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

const eps = 1e-9

func approxEq(a, b float64) bool {
	return math.Abs(a-b) < eps
}

func TestParse(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue float64
		wantUnit  string
		wantOK    bool
	}{
		{"px", "12px", 12, "px", true},
		{"pt", "10pt", 10, "pt", true},
		{"rem", "1rem", 1, "rem", true},
		{"em", "1.5em", 1.5, "em", true},
		{"mm", "5mm", 5, "mm", true},
		{"cm", "2cm", 2, "cm", true},
		{"in", "1in", 1, "in", true},
		{"percent", "50%", 50, "%", true},

		// rem vs em disambiguation: "rem" must not be mistaken for "em".
		{"rem-not-em", "2rem", 2, "rem", true},
		{"em-not-rem", "2em", 2, "em", true},

		// Bare number.
		{"bare-int", "100", 100, "", true},
		{"bare-float", "12.5", 12.5, "", true},

		// Negative and decimal values.
		{"negative", "-10px", -10, "px", true},
		{"negative-bare", "-5", -5, "", true},
		{"decimal", "0.5em", 0.5, "em", true},

		// Internal whitespace: parsePlainLength trimmed the numeric part
		// after stripping the suffix, so "12 px" parsed as 12px.
		{"internal-space", "12 px", 12, "px", true},
		{"leading-trailing-space", "  12px  ", 12, "px", true},

		// Empty and garbage.
		{"empty", "", 0, "", false},
		{"garbage", "abc", 0, "", false},
		{"unit-only", "px", 0, "", false},

		// Multi-suffix garbage is rejected — exactly one unit is consumed.
		{"multi-suffix", "10pxpx", 0, "", false},
		{"multi-suffix-mixed", "10empx", 0, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, unit, ok := Parse(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("Parse(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if !approxEq(v, tt.wantValue) || unit != tt.wantUnit {
				t.Errorf("Parse(%q) = (%v, %q), want (%v, %q)", tt.input, v, unit, tt.wantValue, tt.wantUnit)
			}
		})
	}
}

func TestPointsPerUnit(t *testing.T) {
	const fontSize = 12.0
	tests := []struct {
		unit       string
		wantFactor float64
		wantOK     bool
	}{
		{"pt", 1, true},
		{"px", 0.75, true},
		{"em", fontSize, true},
		{"rem", 16 * 0.75, true},
		{"mm", 72 / 25.4, true},
		{"cm", 72 / 2.54, true},
		{"in", 72, true},
		{"%", 0, false},
		{"", 0, false},
		{"bogus", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.unit, func(t *testing.T) {
			factor, ok := PointsPerUnit(tt.unit, fontSize)
			if ok != tt.wantOK {
				t.Fatalf("PointsPerUnit(%q) ok = %v, want %v", tt.unit, ok, tt.wantOK)
			}
			if ok && !approxEq(factor, tt.wantFactor) {
				t.Errorf("PointsPerUnit(%q) = %v, want %v", tt.unit, factor, tt.wantFactor)
			}
		})
	}
}

// oldParsePlainLength reproduces html's pre-migration parsePlainLength
// (html/properties.go) byte for byte, so TestParseMatchesOldHTMLParser can
// assert the shared tokenizer is a drop-in replacement.
func oldParsePlainLength(value string) (float64, string, bool) {
	value = strings.TrimSpace(value)
	for _, unit := range []string{"px", "pt", "rem", "em", "mm", "cm", "in", "%"} {
		if strings.HasSuffix(value, unit) {
			numStr := strings.TrimSpace(value[:len(value)-len(unit)])
			num, err := strconv.ParseFloat(numStr, 64)
			if err != nil {
				return 0, "", false
			}
			return num, unit, true
		}
	}
	if num, err := strconv.ParseFloat(value, 64); err == nil {
		return num, "px", true
	}
	return 0, "", false
}

func TestParseMatchesOldHTMLParser(t *testing.T) {
	inputs := []string{
		"12px", "10pt", "1rem", "1.5em", "5mm", "2cm", "1in", "50%",
		"100", "12.5", "-10px", "-5", "0.5em", "12 px", "  12px  ",
		"", "abc", "px", "10pxpx", "10empx", "0", "0px",
	}
	for _, in := range inputs {
		t.Run(in, func(t *testing.T) {
			wantVal, wantUnit, wantOK := oldParsePlainLength(in)
			gotVal, gotUnit, gotOK := Parse(in)
			// The old parser defaulted a bare number to unit "px"; Parse
			// returns unit "" for a bare number and lets the caller apply
			// its own default (html/properties.go parsePlainLength does).
			if gotOK && gotUnit == "" {
				gotUnit = "px"
			}
			if gotOK != wantOK {
				t.Fatalf("ok mismatch for %q: got %v, want %v", in, gotOK, wantOK)
			}
			if gotOK && (!approxEq(gotVal, wantVal) || gotUnit != wantUnit) {
				t.Errorf("mismatch for %q: got (%v, %q), want (%v, %q)", in, gotVal, gotUnit, wantVal, wantUnit)
			}
		})
	}
}

// oldParseDimension reproduces svg's pre-migration parseDimension
// (svg/parser.go), including its multi-suffix-stripping quirk, so
// TestParseDimensionBugFix can pin the intentional behavior change.
func oldParseDimension(s string) float64 {
	s = strings.TrimSpace(s)
	for _, suffix := range []string{"px", "pt", "em", "rem", "cm", "mm", "in", "%"} {
		s = strings.TrimSuffix(s, suffix)
	}
	s = strings.TrimSpace(s)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func TestParseDimensionBugFix(t *testing.T) {
	tests := []struct {
		input     string
		oldWant   float64
		newWant   float64
		regresses bool // old and new intentionally disagree
	}{
		{"100px", 100, 100, false},
		{"10cm", 10, 10, false},
		{"50%", 50, 50, false},
		{"100", 100, 100, false},
		{"", 0, 0, false},
		{"10pxpx", 0, 0, false}, // "px" strips once, leftover "10px" fails to parse — already 0 today
		{"10empx", 10, 0, true}, // "px" then "em" strip in sequence, leaving "10" — old parsed it, new rejects it
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := oldParseDimension(tt.input); !approxEq(got, tt.oldWant) {
				t.Fatalf("oldParseDimension(%q) = %v, want %v (old-parser pin is stale)", tt.input, got, tt.oldWant)
			}
			v, _, ok := Parse(tt.input)
			if !ok {
				v = 0
			}
			if !approxEq(v, tt.newWant) {
				t.Errorf("Parse(%q) value = %v, want %v", tt.input, v, tt.newWant)
			}
			if !tt.regresses && !approxEq(tt.oldWant, tt.newWant) {
				t.Errorf("%q: expected old/new agreement, old=%v new=%v", tt.input, tt.oldWant, tt.newWant)
			}
		})
	}
}
