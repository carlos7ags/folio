// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestStyledParagraphSingleRun(t *testing.T) {
	p := NewStyledParagraph(NewRun("Hello World", font.Helvetica, 12))
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
	if lines[0].Words[0].Font != font.Helvetica {
		t.Errorf("expected Helvetica font, got %v", lines[0].Words[0].Font)
	}
	if lines[0].Words[0].FontSize != 12 {
		t.Errorf("expected fontSize 12, got %.1f", lines[0].Words[0].FontSize)
	}
}

func TestStyledParagraphMixedFonts(t *testing.T) {
	p := NewStyledParagraph(
		NewRun("Normal ", font.Helvetica, 12),
		NewRun("bold", font.HelveticaBold, 12),
		NewRun(" text.", font.Helvetica, 12),
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
		NewRun("Big", font.Helvetica, 24),
		NewRun(" small", font.Helvetica, 10),
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
		NewRun("Black", font.Helvetica, 12),
		NewRun(" red", font.Helvetica, 12).WithColor(red),
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
		NewRun("Start ", font.Helvetica, 12),
		NewRun("middle ", font.HelveticaBold, 12),
		NewRun("end of a longer text that should wrap across lines.", font.Helvetica, 12),
	)
	lines := p.Layout(200)
	if len(lines) < 2 {
		t.Errorf("expected multiple lines, got %d", len(lines))
	}
	// Count all words across lines — input has 11 words, all must survive.
	allWords := 0
	for _, l := range lines {
		allWords += len(l.Words)
	}
	if allWords != 12 {
		t.Errorf("expected 12 words total, got %d", allWords)
	}
}

func TestStyledParagraphEmptyRun(t *testing.T) {
	p := NewStyledParagraph(
		NewRun("", font.Helvetica, 12),
		NewRun("Hello", font.Helvetica, 12),
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
		NewRun("", font.Helvetica, 12),
		NewRun("  ", font.Helvetica, 12),
	)
	lines := p.Layout(500)
	if len(lines) != 0 {
		t.Errorf("expected 0 lines for empty runs, got %d", len(lines))
	}
}

func TestStyledParagraphAlignment(t *testing.T) {
	p := NewStyledParagraph(
		NewRun("Centered", font.Helvetica, 12),
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
		NewRun("Big", font.Helvetica, 24),
		NewRun(" small text", font.Helvetica, 8),
	)
	lines := p.Layout(500)
	words := lines[0].Words
	// Each word should have SpaceAfter from its own font/size.
	// Helvetica space width at 24pt vs 8pt should differ.
	if words[0].SpaceAfter == words[1].SpaceAfter {
		t.Errorf("SpaceAfter should differ by font size: 24pt=%.2f, 8pt=%.2f", words[0].SpaceAfter, words[1].SpaceAfter)
	}
	if words[0].SpaceAfter <= 0 {
		t.Error("SpaceAfter should be positive")
	}
}

func TestRunWithColor(t *testing.T) {
	r := NewRun("test", font.Helvetica, 12).WithColor(RGB(0.5, 0.5, 0.5))
	if r.Color.R != 0.5 || r.Color.G != 0.5 || r.Color.B != 0.5 {
		t.Errorf("unexpected color: %+v", r.Color)
	}
	// Original run should be unmodified (value receiver).
	r2 := NewRun("test", font.Helvetica, 12)
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

// TestPunctuationNotMergedAcrossFontBoundary verifies that punctuation at
// a font boundary is NOT merged into the preceding word when the fonts
// differ. The period should keep its own (regular) font, not inherit bold.
// Regression test for #30, supersedes #25 behavior for cross-font cases.
func TestPunctuationNotMergedAcrossFontBoundary(t *testing.T) {
	p := NewStyledParagraph(
		NewRun("click here", font.HelveticaBold, 12),
		NewRun(". Then continue.", font.Helvetica, 12),
	)
	words, _ := p.measureWords(400)
	// "here" should be bold, "." should be regular (separate word).
	for _, w := range words {
		if w.Text == "here." {
			t.Error("period should NOT be merged into bold word across font boundary")
		}
	}
	// The period must exist as its own word with regular font.
	foundPeriod := false
	for _, w := range words {
		if w.Text == "." {
			foundPeriod = true
			if w.Font != font.Helvetica {
				t.Errorf("period should be Helvetica, got %v", w.Font)
			}
		}
	}
	if !foundPeriod {
		t.Error("expected standalone '.' word in output")
	}
}

// TestPunctuationMergeMatchesSingleRun verifies that cross-run punctuation
// merging produces identical words to a single-run paragraph with the same
// text. This ensures the merge is a true root fix, not a rendering patch.
func TestPunctuationMergeMatchesSingleRun(t *testing.T) {
	single := NewParagraph("click here. Then continue.", font.Helvetica, 12)
	multi := NewStyledParagraph(
		NewRun("click here", font.Helvetica, 12),
		NewRun(". Then continue.", font.Helvetica, 12),
	)
	singleWords, _ := single.measureWords(400)
	multiWords, _ := multi.measureWords(400)
	if len(singleWords) != len(multiWords) {
		t.Fatalf("word count differs: single=%d multi=%d", len(singleWords), len(multiWords))
	}
	for i := range singleWords {
		if singleWords[i].Text != multiWords[i].Text {
			t.Errorf("word %d: single=%q multi=%q", i, singleWords[i].Text, multiWords[i].Text)
		}
	}
}

// TestPunctuationCommaKeepsOwnFontAcrossBoundary verifies that a comma
// after a bold word keeps regular font when the fonts differ.
func TestPunctuationCommaKeepsOwnFontAcrossBoundary(t *testing.T) {
	p := NewStyledParagraph(
		NewRun("see ", font.Helvetica, 12),
		NewRun("this", font.HelveticaBold, 12),
		NewRun(", that.", font.Helvetica, 12),
	)
	words, _ := p.measureWords(400)
	// "this" should be bold, comma should NOT be merged into it.
	for _, w := range words {
		if w.Text == "this," {
			t.Error("comma should NOT be merged into bold word across font boundary")
		}
	}
	// Verify the comma exists as part of its own run's words with regular font.
	// The run ", that." starts with comma, so after splitWords it becomes [",", "that."].
	foundCommaWord := false
	for _, w := range words {
		if w.Text == "," || (len(w.Text) > 0 && w.Text[0] == ',') {
			foundCommaWord = true
			if w.Font == font.HelveticaBold {
				t.Errorf("comma should be regular font, got bold")
			}
		}
	}
	if !foundCommaWord {
		texts := make([]string, len(words))
		for i, w := range words {
			texts[i] = w.Text
		}
		t.Errorf("expected word starting with comma, got: %v", texts)
	}
}

// TestPunctuationLeadingSpaceNotMerged verifies that when a run starts
// with whitespace before punctuation (e.g. " . word"), the space acts as
// a word boundary and the period is NOT merged into the previous word.
func TestPunctuationLeadingSpaceNotMerged(t *testing.T) {
	p := NewStyledParagraph(
		NewRun("word", font.Helvetica, 12),
		NewRun(" . separate", font.Helvetica, 12),
	)
	words, _ := p.measureWords(400)
	// The "." should be a standalone word because the run starts with a space.
	foundStandaloneDot := false
	for _, w := range words {
		if w.Text == "." {
			foundStandaloneDot = true
		}
	}
	if !foundStandaloneDot {
		texts := make([]string, len(words))
		for i, w := range words {
			texts[i] = w.Text
		}
		t.Errorf("expected standalone '.' word (space prevents merge), got: %v", texts)
	}
}

// TestPunctuationMultipleChars verifies that multiple leading punctuation
// characters (e.g. ")." or "...") are all merged.
func TestPunctuationMultipleChars(t *testing.T) {
	p := NewStyledParagraph(
		NewRun("end", font.Helvetica, 12),
		NewRun(").", font.Helvetica, 12),
	)
	words, _ := p.measureWords(400)
	foundMerged := false
	for _, w := range words {
		if w.Text == "end)." {
			foundMerged = true
		}
	}
	if !foundMerged {
		texts := make([]string, len(words))
		for i, w := range words {
			texts[i] = w.Text
		}
		t.Errorf("expected 'end).' but got: %v", texts)
	}
}

// TestPunctuationFirstRunNotMerged verifies that punctuation at the very
// start of the paragraph (no preceding word) is not merged anywhere.
func TestPunctuationFirstRunNotMerged(t *testing.T) {
	p := NewStyledParagraph(
		NewRun("...start", font.Helvetica, 12),
	)
	words, _ := p.measureWords(400)
	if len(words) != 1 || words[0].Text != "...start" {
		texts := make([]string, len(words))
		for i, w := range words {
			texts[i] = w.Text
		}
		t.Errorf("expected ['...start'] but got: %v", texts)
	}
}
