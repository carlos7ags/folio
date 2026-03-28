// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

func TestParseCMYKPercentRed(t *testing.T) {
	// cmyk(0, 100%, 100%, 0) should produce CMYK color with m=1, y=1.
	c, ok := parseColor("cmyk(0, 100%, 100%, 0)")
	if !ok {
		t.Fatal("expected valid color")
	}
	if c.Space != layout.ColorSpaceCMYK {
		t.Errorf("expected CMYK color space, got %v", c.Space)
	}
	if math.Abs(c.C-0) > 0.01 {
		t.Errorf("C = %f, want 0", c.C)
	}
	if math.Abs(c.M-1) > 0.01 {
		t.Errorf("M = %f, want 1", c.M)
	}
	if math.Abs(c.Y-1) > 0.01 {
		t.Errorf("Y = %f, want 1", c.Y)
	}
	if math.Abs(c.K-0) > 0.01 {
		t.Errorf("K = %f, want 0", c.K)
	}
}

func TestParseCMYKBlack(t *testing.T) {
	// cmyk(0, 0, 0, 1) should be full black.
	c, ok := parseColor("cmyk(0, 0, 0, 1)")
	if !ok {
		t.Fatal("expected valid color")
	}
	if c.Space != layout.ColorSpaceCMYK {
		t.Errorf("expected CMYK color space, got %v", c.Space)
	}
	if math.Abs(c.K-1) > 0.01 {
		t.Errorf("K = %f, want 1", c.K)
	}
}

func TestParseDeviceCMYK(t *testing.T) {
	c, ok := parseColor("device-cmyk(0.5, 0.3, 0, 0)")
	if !ok {
		t.Fatal("expected valid color")
	}
	if c.Space != layout.ColorSpaceCMYK {
		t.Errorf("expected CMYK color space, got %v", c.Space)
	}
	if math.Abs(c.C-0.5) > 0.01 {
		t.Errorf("C = %f, want 0.5", c.C)
	}
	if math.Abs(c.M-0.3) > 0.01 {
		t.Errorf("M = %f, want 0.3", c.M)
	}
}

func TestParseCMYKInvalid(t *testing.T) {
	// Too few args.
	_, ok := parseColor("cmyk(0, 0, 0)")
	if ok {
		t.Error("expected invalid for 3-arg cmyk")
	}

	// Empty.
	_, ok = parseColor("cmyk()")
	if ok {
		t.Error("expected invalid for empty cmyk")
	}
}

func TestParseCMYKDecimalValues(t *testing.T) {
	c, ok := parseColor("cmyk(0.1, 0.2, 0.3, 0.4)")
	if !ok {
		t.Fatal("expected valid color")
	}
	if math.Abs(c.C-0.1) > 0.01 || math.Abs(c.M-0.2) > 0.01 ||
		math.Abs(c.Y-0.3) > 0.01 || math.Abs(c.K-0.4) > 0.01 {
		t.Errorf("CMYK = (%f,%f,%f,%f), want (0.1,0.2,0.3,0.4)", c.C, c.M, c.Y, c.K)
	}
}

func TestCMYKInHTMLConversion(t *testing.T) {
	src := `<style>p { color: cmyk(0, 100%, 100%, 0); }</style><p>Red text</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestCMYKMixedPercentsAndDecimals(t *testing.T) {
	// Mixing percent and decimal notation.
	c, ok := parseColor("cmyk(50%, 0.3, 0%, 0.1)")
	if !ok {
		t.Fatal("expected valid color")
	}
	if math.Abs(c.C-0.5) > 0.01 {
		t.Errorf("C = %f, want 0.5", c.C)
	}
	if math.Abs(c.M-0.3) > 0.01 {
		t.Errorf("M = %f, want 0.3", c.M)
	}
}
