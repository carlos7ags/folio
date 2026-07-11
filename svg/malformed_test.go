// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"math"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// parsePathData: malformed grammar
// ---------------------------------------------------------------------------

func TestParsePathData_Malformed(t *testing.T) {
	tests := []struct {
		name    string
		d       string
		wantErr bool
		wantLen int // checked only when wantErr is false
	}{
		{"truncated arc args", "M0 0 A 5 5", true, 0},
		{"arc cut mid-flag", "M0 0 A 5 5 0 1", true, 0},
		{"bare minus token", "M 0 0 L - 5", true, 0},
		{"bare dot token", "M . . L 1 1", true, 0},
		{"trailing bare exponent", "M 1e 0", true, 0},
		{"huge exponent", "M 1e999 0", true, 0},
		{"number where command expected", "5 5 M 0 0", true, 0},
		{"command with zero args then EOF", "L", false, 0},
		{"unexpected character", "M 0 0 # 5 5", true, 0},
		{"wrong arity on implicit repeat", "M0 0 C 1 2 3 4 5 6 7", true, 0},
		{"empty string", "", false, 0},
		{"whitespace only", "   \t\n", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmds, err := parsePathData(tt.d)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parsePathData(%q) error = %v, wantErr %v", tt.d, err, tt.wantErr)
			}
			if !tt.wantErr && len(cmds) != tt.wantLen {
				t.Errorf("parsePathData(%q) = %d commands, want %d", tt.d, len(cmds), tt.wantLen)
			}
		})
	}
}

// TestParsePathData_GiantRepeat pins that a large-but-valid path completes
// without superlinear blowup: 50,000 implicit-repeat L commands after one M.
func TestParsePathData_GiantRepeat(t *testing.T) {
	d := "M0 0 " + strings.Repeat("L1 1 ", 50_000)
	start := time.Now()
	cmds, err := parsePathData(d)
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("parsePathData giant repeat: unexpected error: %v", err)
	}
	if len(cmds) != 50_001 {
		t.Errorf("parsePathData giant repeat: got %d commands, want 50001", len(cmds))
	}
	if elapsed > 30*time.Second {
		t.Errorf("parsePathData giant repeat took %v, want well under 30s (possible superlinear behavior)", elapsed)
	}
}

func TestArcToCubics_Extremes(t *testing.T) {
	tests := []struct {
		name                                  string
		startX, startY, rx, ry, xAxisRotation float64
		largeArc, sweep                       bool
		endX, endY                            float64
	}{
		{"infinite rx, NaN rotation", 0, 0, math.Inf(1), 5, math.NaN(), false, true, 10, 10},
		{"NaN rx and ry", 0, 0, math.NaN(), math.NaN(), 0, false, true, 10, 0},
		{"infinite start point", math.Inf(1), math.Inf(-1), 5, 5, 0, true, false, 10, 10},
		{"infinite end point", 0, 0, 5, 5, 0, false, true, math.Inf(1), math.Inf(1)},
		{"NaN endpoint", 0, 0, 5, 5, 0, false, true, math.NaN(), 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Must not panic; result values are unconstrained for degenerate input.
			_ = arcToCubics(tt.startX, tt.startY, tt.rx, tt.ry, tt.xAxisRotation, tt.largeArc, tt.sweep, tt.endX, tt.endY)
		})
	}
}

// ---------------------------------------------------------------------------
// parseTransform: malformed grammar (parseTransform never errors, so every
// case pins the resulting Matrix)
// ---------------------------------------------------------------------------

func matrixNear(a, b Matrix) bool {
	const eps = 1e-9
	return math.Abs(a.A-b.A) < eps && math.Abs(a.B-b.B) < eps &&
		math.Abs(a.C-b.C) < eps && math.Abs(a.D-b.D) < eps &&
		math.Abs(a.E-b.E) < eps && math.Abs(a.F-b.F) < eps
}

func TestParseTransform_Malformed(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want Matrix
	}{
		{"unclosed paren", "translate(10", identity()},
		{"unclosed paren after valid", "scale(2) translate(10", Scale(2, 2)},
		{"garbage args", "scale(abc)", Scale(1, 1)},
		{"mixed garbage", "translate(10,abc)", Translate(10, 0)},
		{"unknown function", "frobnicate(1,2) translate(5,5)", Translate(5, 5)},
		{"rotate with 2 args", "rotate(90, 10)", Rotate(90)},
		{"matrix with 5 args", "matrix(1,2,3,4,5)", identity()},
		{"empty parens", "translate()", Translate(0, 0)},
		{"no parens at all", "translate", identity()},
		{"comma soup", ",,, ,", identity()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTransform(tt.in)
			if !matrixNear(got, tt.want) {
				t.Errorf("parseTransform(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseGradientCoord: malformed grammar
// ---------------------------------------------------------------------------

func TestParseGradientCoord(t *testing.T) {
	const def = 0.5
	tests := []struct {
		name string
		in   string
		want float64
	}{
		{"empty", "", def},
		{"fifty percent", "50%", 0.5},
		{"over 100 percent not clamped", "120%", 1.2},
		{"bad percent", "bad%", def},
		{"px suffix", "10px", 10},
		{"garbage", "abc", def},
		{"padded number", " 0.25 ", 0.25},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGradientCoord(tt.in, def)
			if got != tt.want {
				t.Errorf("parseGradientCoord(%q, %v) = %v, want %v", tt.in, def, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseStops: malformed grammar
// ---------------------------------------------------------------------------

// findGradientNode does a depth-first search for the first node with the given tag.
func findGradientNode(n *Node, tag string) *Node {
	if n == nil {
		return nil
	}
	if n.Tag == tag {
		return n
	}
	for _, c := range n.Children {
		if found := findGradientNode(c, tag); found != nil {
			return found
		}
	}
	return nil
}

func TestParseStops_Malformed(t *testing.T) {
	t.Run("zero stops", func(t *testing.T) {
		svgXML := `<svg><defs><linearGradient id="g"></linearGradient></defs></svg>`
		s, err := Parse(svgXML)
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		g := findGradientNode(s.Root(), "linearGradient")
		if g == nil {
			t.Fatal("linearGradient node not found")
		}
		if stops := parseStops(g); stops != nil {
			t.Errorf("parseStops with no <stop> children = %v, want nil", stops)
		}
	})

	t.Run("unparsable stop-color is skipped", func(t *testing.T) {
		svgXML := `<svg><defs><linearGradient id="g">
			<stop offset="0" stop-color="nope"/>
			<stop offset="1" stop-color="blue"/>
		</linearGradient></defs></svg>`
		s, err := Parse(svgXML)
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		g := findGradientNode(s.Root(), "linearGradient")
		if g == nil {
			t.Fatal("linearGradient node not found")
		}
		stops := parseStops(g)
		if len(stops) != 1 {
			t.Fatalf("parseStops = %d stops, want 1 (unparsable stop-color skipped)", len(stops))
		}
		if stops[0].Offset != 1 {
			t.Errorf("remaining stop offset = %v, want 1", stops[0].Offset)
		}
	})

	t.Run("unparsable stop-opacity falls back to opaque", func(t *testing.T) {
		svgXML := `<svg><defs><linearGradient id="g">
			<stop offset="0" stop-color="red" stop-opacity="abc"/>
			<stop offset="1" stop-color="blue"/>
		</linearGradient></defs></svg>`
		s, err := Parse(svgXML)
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		g := findGradientNode(s.Root(), "linearGradient")
		if g == nil {
			t.Fatal("linearGradient node not found")
		}
		stops := parseStops(g)
		if len(stops) != 2 {
			t.Fatalf("parseStops = %d stops, want 2", len(stops))
		}
		if stops[0].Color.A != 1 {
			t.Errorf("stop-opacity=%q alpha = %v, want 1 (fallback)", "abc", stops[0].Color.A)
		}
	})
}
