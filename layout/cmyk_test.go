// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestCMYKColor(t *testing.T) {
	c := CMYK(1, 0, 0, 0) // pure cyan
	if c.Space != ColorSpaceCMYK {
		t.Errorf("Space = %d, want ColorSpaceCMYK", c.Space)
	}
	if c.C != 1 || c.M != 0 || c.Y != 0 || c.K != 0 {
		t.Errorf("CMYK values wrong: %+v", c)
	}
}

func TestCMYKTextRendering(t *testing.T) {
	p := NewParagraph("CMYK text", font.Helvetica, 12)
	p.runs[0].Color = CMYK(0, 1, 1, 0) // red in CMYK

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(p)
	pages := r.Render()

	content := string(pages[0].Stream.Bytes())
	// Should emit "k" operator (CMYK fill), not "rg" (RGB fill).
	if !strings.Contains(content, " k") {
		t.Error("expected CMYK fill color operator 'k'")
	}
	if strings.Contains(content, " rg") {
		t.Error("should not emit RGB operator for CMYK color")
	}
}

func TestCMYKBackgroundRendering(t *testing.T) {
	p := NewParagraph("Background", font.Helvetica, 12).
		SetBackground(CMYK(0, 0, 0.2, 0)) // light yellow in CMYK

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(p)
	pages := r.Render()

	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, " k") {
		t.Error("expected CMYK fill operator for background")
	}
}

func TestCMYKDecorationRendering(t *testing.T) {
	run := Run("Underlined", font.Helvetica, 12).
		WithColor(CMYK(1, 0, 0, 0)).
		WithUnderline()
	p := NewStyledParagraph(run)

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(p)
	pages := r.Render()

	content := string(pages[0].Stream.Bytes())
	// Underline stroke should use CMYK operator "K".
	if !strings.Contains(content, " K") {
		t.Error("expected CMYK stroke operator for underline")
	}
}

func TestRGBDefault(t *testing.T) {
	c := RGB(1, 0, 0)
	if c.Space != ColorSpaceRGB {
		t.Errorf("RGB should default to ColorSpaceRGB, got %d", c.Space)
	}
}

func TestCMYKBlack(t *testing.T) {
	c := CMYK(0, 0, 0, 1)
	if c.K != 1 {
		t.Errorf("K = %.1f, want 1", c.K)
	}
}
