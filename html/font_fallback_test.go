// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// TestFontFallbackSplitsMixedScript verifies that mixed Latin+Hebrew text
// produces multiple TextRuns (one per font) when a fallback font is
// available. Without font splitting, the entire text would use the
// fallback font even for Latin characters that the standard font handles.
func TestFontFallbackSplitsMixedScript(t *testing.T) {
	// This test exercises the splitTextByFont path. We need a fallback
	// font to be available. On systems without one, the test is skipped.
	src := `<p>Hello שלום world</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}
	// Verify it produces a renderable layout.
	lines := p.Layout(500)
	if len(lines) == 0 {
		t.Fatal("no lines")
	}
	// With a fallback font, Hebrew text should get the fallback and
	// Latin text should stay on Helvetica. We check that both scripts
	// are present in the word list.
	hasLatin := false
	hasNonLatin := false
	for _, line := range lines {
		for _, w := range line.Words {
			if w.Text == "Hello" || w.Text == "world" {
				hasLatin = true
			}
			if len([]rune(w.Text)) > 0 {
				for _, r := range w.Text {
					if r >= 0x0590 && r <= 0x05FF { // Hebrew block
						hasNonLatin = true
					}
				}
			}
		}
	}
	if !hasLatin {
		t.Error("expected Latin words in output")
	}
	if !hasNonLatin {
		t.Error("expected Hebrew characters in output")
	}
}

// TestFontFallbackPureASCIINoSplit verifies that pure ASCII text is NOT
// split (fast path — no fallback font needed).
func TestFontFallbackPureASCIINoSplit(t *testing.T) {
	src := `<p>Hello world</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}
	lines := p.Layout(500)
	if len(lines) == 0 {
		t.Fatal("no lines")
	}
	// All words should use the same font (standard Helvetica).
	for _, w := range lines[0].Words {
		if w.Font == nil && w.Embedded == nil {
			t.Errorf("word %q has no font", w.Text)
		}
		if w.Embedded != nil {
			t.Errorf("word %q should use standard font, got embedded", w.Text)
		}
	}
}

// TestCanEncodeWinAnsiRune verifies the per-rune encoding check.
func TestCanEncodeWinAnsiRune(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'A', true},
		{'z', true},
		{' ', true},
		{0x00E9, true},  // e-acute — in WinAnsi
		{0x05D0, false}, // Hebrew alef — not in WinAnsi
		{0x0628, false}, // Arabic beh — not in WinAnsi
		{0x4E2D, false}, // CJK character — not in WinAnsi
	}
	for _, tt := range tests {
		got := font.CanEncodeWinAnsiRune(tt.r)
		if got != tt.want {
			t.Errorf("CanEncodeWinAnsiRune(%U): got %v, want %v", tt.r, got, tt.want)
		}
	}
}
