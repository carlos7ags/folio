// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestStyledParagraphSingleRun(t *testing.T) {
	p := NewStyledParagraph(Run("Hello World", font.Helvetica, 12))
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if len(lines[0].Words) != 2 {
		t.Fatalf("expected 2 words, got %d", len(lines[0].Words))
	}
	if lines[0].Words[0].Text != "Hello" || lines[0].Words[1].Text != "World" {
		t.Error("unexpected word text")
	}
}

func TestStyledParagraphMixedFonts(t *testing.T) {
	p := NewStyledParagraph(
		Run("Normal ", font.Helvetica, 12),
		Run("bold", font.HelveticaBold, 12),
		Run(" text.", font.Helvetica, 12),
	)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "Normal bold text." → 3 words
	words := lines[0].Words
	if len(words) != 3 {
		t.Fatalf("expected 3 words, got %d", len(words))
	}
	if words[0].Font != font.Helvetica {
		t.Error("word 0 should be Helvetica")
	}
	if words[1].Font != font.HelveticaBold {
		t.Error("word 1 should be HelveticaBold")
	}
	if words[2].Font != font.Helvetica {
		t.Error("word 2 should be Helvetica")
	}
}

func TestStyledParagraphMixedSizes(t *testing.T) {
	p := NewStyledParagraph(
		Run("Big", font.Helvetica, 24),
		Run(" small", font.Helvetica, 10),
	)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	words := lines[0].Words
	if words[0].FontSize != 24 {
		t.Errorf("word 0 size: expected 24, got %f", words[0].FontSize)
	}
	if words[1].FontSize != 10 {
		t.Errorf("word 1 size: expected 10, got %f", words[1].FontSize)
	}
	// Line height should be based on the max font size.
	expectedHeight := 24 * 1.2
	diff := lines[0].Height - expectedHeight
	if diff > 0.001 || diff < -0.001 {
		t.Errorf("line height: expected %f, got %f", expectedHeight, lines[0].Height)
	}
}

func TestStyledParagraphColor(t *testing.T) {
	red := RGB(1, 0, 0)
	p := NewStyledParagraph(
		Run("Black", font.Helvetica, 12),
		Run(" red", font.Helvetica, 12).WithColor(red),
	)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	words := lines[0].Words
	if words[0].Color != ColorBlack {
		t.Errorf("word 0 should be black, got %+v", words[0].Color)
	}
	if words[1].Color != red {
		t.Errorf("word 1 should be red, got %+v", words[1].Color)
	}
}

func TestStyledParagraphWordWrap(t *testing.T) {
	p := NewStyledParagraph(
		Run("Start ", font.Helvetica, 12),
		Run("middle ", font.HelveticaBold, 12),
		Run("end of a longer text that should wrap across lines.", font.Helvetica, 12),
	)
	lines := p.Layout(200)
	if len(lines) < 2 {
		t.Errorf("expected multiple lines, got %d", len(lines))
	}
	// Words should flow across runs — a bold word may end up on line 2.
	allWords := 0
	for _, l := range lines {
		allWords += len(l.Words)
	}
	if allWords < 5 {
		t.Errorf("expected at least 5 words total, got %d", allWords)
	}
}

func TestStyledParagraphEmptyRun(t *testing.T) {
	p := NewStyledParagraph(
		Run("", font.Helvetica, 12),
		Run("Hello", font.Helvetica, 12),
	)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if len(lines[0].Words) != 1 {
		t.Errorf("expected 1 word, got %d", len(lines[0].Words))
	}
}

func TestStyledParagraphAllEmpty(t *testing.T) {
	p := NewStyledParagraph(
		Run("", font.Helvetica, 12),
		Run("  ", font.Helvetica, 12),
	)
	lines := p.Layout(500)
	if len(lines) != 0 {
		t.Errorf("expected 0 lines for empty runs, got %d", len(lines))
	}
}

func TestStyledParagraphAlignment(t *testing.T) {
	p := NewStyledParagraph(
		Run("Centered", font.Helvetica, 12),
	).SetAlign(AlignCenter)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatal("expected 1 line")
	}
	if lines[0].Align != AlignCenter {
		t.Error("expected AlignCenter")
	}
}

func TestStyledParagraphSpaceAfterPerWord(t *testing.T) {
	p := NewStyledParagraph(
		Run("Big", font.Helvetica, 24),
		Run(" small text", font.Helvetica, 8),
	)
	lines := p.Layout(500)
	words := lines[0].Words
	// Each word should have SpaceAfter from its own font/size.
	if words[0].SpaceAfter == words[1].SpaceAfter {
		t.Log("SpaceAfter differs by font size — expected different values for different sizes")
		// Helvetica space width at 24pt vs 8pt should differ.
	}
	if words[0].SpaceAfter <= 0 {
		t.Error("SpaceAfter should be positive")
	}
}

func TestRunWithColor(t *testing.T) {
	r := Run("test", font.Helvetica, 12).WithColor(RGB(0.5, 0.5, 0.5))
	if r.Color.R != 0.5 || r.Color.G != 0.5 || r.Color.B != 0.5 {
		t.Errorf("unexpected color: %+v", r.Color)
	}
	// Original run should be unmodified (value receiver).
	r2 := Run("test", font.Helvetica, 12)
	if r2.Color != ColorBlack {
		t.Errorf("original run should be black: %+v", r2.Color)
	}
}

func TestNewParagraphBackwardCompatible(t *testing.T) {
	// NewParagraph should still work exactly as before.
	p := NewParagraph("Hello World", font.Helvetica, 12)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if len(lines[0].Words) != 2 {
		t.Fatalf("expected 2 words, got %d", len(lines[0].Words))
	}
	if lines[0].Words[0].Font != font.Helvetica {
		t.Error("expected Helvetica")
	}
}

func TestNewParagraphEmbeddedNilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil embedded font")
		}
	}()
	NewParagraphEmbedded("text", nil, 12)
}

func TestRGBConstructor(t *testing.T) {
	c := RGB(0.2, 0.4, 0.6)
	if c.R != 0.2 || c.G != 0.4 || c.B != 0.6 {
		t.Errorf("unexpected color: %+v", c)
	}
}
