// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

// TestMarginBoxCustomFontRegistered verifies the renderer registers and
// draws with a margin box's own resolved Font instead of the hard-coded
// standard Helvetica (issue #378 gap A).
func TestMarginBoxCustomFontRegistered(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Body", font.Helvetica, 12))
	r.SetMarginBoxes(map[string]MarginBox{
		"bottom-center": {Content: "X", FontSize: 9, Font: font.HelveticaOblique},
	})
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least one page")
	}
	found := false
	for _, fe := range pages[0].Fonts {
		if fe.Standard != nil && fe.Standard.Name() == "Helvetica-Oblique" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a registered Helvetica-Oblique font resource, got %+v", pages[0].Fonts)
	}
}

// TestMarginBoxNilFontDefaultsHelvetica verifies a nil Font (matching
// every pre-existing caller) still falls back to plain Helvetica.
func TestMarginBoxNilFontDefaultsHelvetica(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Body", font.Helvetica, 12))
	r.SetMarginBoxes(map[string]MarginBox{
		"bottom-center": {Content: "X", FontSize: 9},
	})
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least one page")
	}
	found := false
	for _, fe := range pages[0].Fonts {
		if fe.Standard != nil && fe.Standard.Name() == "Helvetica" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a registered plain Helvetica font resource, got %+v", pages[0].Fonts)
	}
}
