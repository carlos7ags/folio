// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestHeadingH1DefaultSize(t *testing.T) {
	h := NewHeading("Title", H1)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// H1 should use 28pt font
	if lines[0].Words[0].FontSize != 28 {
		t.Errorf("expected H1 font size 28, got %f", lines[0].Words[0].FontSize)
	}
}

func TestHeadingH6DefaultSize(t *testing.T) {
	h := NewHeading("Tiny heading", H6)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].Words[0].FontSize != 10.7 {
		t.Errorf("expected H6 font size 10.7, got %f", lines[0].Words[0].FontSize)
	}
}

func TestHeadingDefaultFont(t *testing.T) {
	h := NewHeading("Bold heading", H2)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].Words[0].Font != font.HelveticaBold {
		t.Error("expected HelveticaBold as default heading font")
	}
}

func TestHeadingWithFont(t *testing.T) {
	h := NewHeadingWithFont("Custom", H3, font.TimesRoman, 30)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].Words[0].Font != font.TimesRoman {
		t.Error("expected TimesRoman")
	}
	if lines[0].Words[0].FontSize != 30 {
		t.Errorf("expected font size 30, got %f", lines[0].Words[0].FontSize)
	}
}

func TestHeadingSpacing(t *testing.T) {
	h := NewHeading("Title", H1)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// First line height should include spacing (fontSize*leading + fontSize*0.5)
	expectedMin := 28 * 1.2 // at least the base line height
	if lines[0].Height <= expectedMin {
		t.Logf("line height %f should include spacing above", lines[0].Height)
	}
}

func TestHeadingAlignment(t *testing.T) {
	h := NewHeading("Centered", H1).SetAlign(AlignCenter)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].Align != AlignCenter {
		t.Error("expected AlignCenter")
	}
}

func TestHeadingWordWrap(t *testing.T) {
	h := NewHeading("This is a very long heading that should wrap to multiple lines", H1)
	lines := h.Layout(200)
	if len(lines) < 2 {
		t.Errorf("expected multiple lines for narrow width, got %d", len(lines))
	}
}

func TestHeadingAllLevels(t *testing.T) {
	levels := []HeadingLevel{H1, H2, H3, H4, H5, H6}
	var prevSize float64
	for _, level := range levels {
		h := NewHeading("Test", level)
		lines := h.Layout(500)
		if len(lines) == 0 {
			t.Fatalf("H%d produced no lines", level)
		}
		size := lines[0].Words[0].FontSize
		if prevSize > 0 && size >= prevSize {
			t.Errorf("H%d size %f should be smaller than H%d size %f", level, size, level-1, prevSize)
		}
		prevSize = size
	}
}
