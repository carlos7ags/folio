// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"math"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestParagraphSingleLine(t *testing.T) {
	p := NewParagraph("Hello World", font.Helvetica, 12)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if len(lines[0].Words) != 2 {
		t.Errorf("expected 2 words, got %d", len(lines[0].Words))
	}
	if lines[0].Words[0].Text != "Hello" {
		t.Errorf("expected 'Hello', got %q", lines[0].Words[0].Text)
	}
	if lines[0].Words[1].Text != "World" {
		t.Errorf("expected 'World', got %q", lines[0].Words[1].Text)
	}
	if !lines[0].IsLast {
		t.Error("single line should be marked as last")
	}
}

func TestParagraphWordWrap(t *testing.T) {
	// Use a narrow width to force wrapping.
	// "Hello" in Helvetica at 12pt = H(722)+e(556)+l(222)+l(222)+o(556) = 2278/1000*12 ≈ 27.3
	// "World" ≈ W(944)+o(556)+r(333)+l(222)+d(556) = 2611/1000*12 ≈ 31.3
	// Space = 278/1000*12 ≈ 3.3
	// Together ≈ 61.9; force wrap at 40pt
	p := NewParagraph("Hello World", font.Helvetica, 12)
	lines := p.Layout(40)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0].Words[0].Text != "Hello" {
		t.Errorf("first line should be 'Hello', got %q", lines[0].Words[0].Text)
	}
	if lines[1].Words[0].Text != "World" {
		t.Errorf("second line should be 'World', got %q", lines[1].Words[0].Text)
	}
	if lines[0].IsLast {
		t.Error("first line should not be last")
	}
	if !lines[1].IsLast {
		t.Error("second line should be last")
	}
}

func TestParagraphEmptyText(t *testing.T) {
	p := NewParagraph("", font.Helvetica, 12)
	lines := p.Layout(500)
	if len(lines) != 0 {
		t.Errorf("expected 0 lines for empty text, got %d", len(lines))
	}
}

func TestParagraphWhitespaceOnly(t *testing.T) {
	p := NewParagraph("   \t\n  ", font.Helvetica, 12)
	lines := p.Layout(500)
	if len(lines) != 0 {
		t.Errorf("expected 0 lines for whitespace-only text, got %d", len(lines))
	}
}

func TestParagraphLeading(t *testing.T) {
	p := NewParagraph("Hello", font.Helvetica, 10).SetLeading(1.5)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	expected := 15.0 // 10 * 1.5
	if math.Abs(lines[0].Height-expected) > 0.001 {
		t.Errorf("expected height %.1f, got %.3f", expected, lines[0].Height)
	}
}

func TestParagraphAlignCenter(t *testing.T) {
	p := NewParagraph("Hello", font.Helvetica, 12).SetAlign(AlignCenter)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Align != AlignCenter {
		t.Error("line should have center alignment")
	}
}

func TestParagraphAlignRight(t *testing.T) {
	p := NewParagraph("Hello", font.Helvetica, 12).SetAlign(AlignRight)
	lines := p.Layout(500)
	if lines[0].Align != AlignRight {
		t.Error("line should have right alignment")
	}
}

func TestParagraphAlignJustify(t *testing.T) {
	p := NewParagraph("Hello World Test", font.Helvetica, 12).SetAlign(AlignJustify)
	// Force wrapping so we get a non-last line.
	lines := p.Layout(80)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	if lines[0].Align != AlignJustify {
		t.Error("first line should have justify alignment")
	}
	// Last line of justified paragraph should still be marked as last.
	last := lines[len(lines)-1]
	if !last.IsLast {
		t.Error("last line should be marked as last")
	}
}

func TestParagraphMultipleWords(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog"
	p := NewParagraph(text, font.Helvetica, 12)
	lines := p.Layout(200)
	// Just verify wrapping happened and all words are present.
	totalWords := 0
	for _, line := range lines {
		totalWords += len(line.Words)
	}
	if totalWords != 9 {
		t.Errorf("expected 9 words total, got %d", totalWords)
	}
}

func TestParagraphLongWord(t *testing.T) {
	// A word wider than maxWidth should be broken into character-level chunks.
	p := NewParagraph("Supercalifragilisticexpialidocious", font.Helvetica, 12)
	lines := p.Layout(50) // very narrow
	if len(lines) < 2 {
		t.Fatalf("expected multiple lines (long word should be broken), got %d", len(lines))
	}
	// All characters should be preserved across lines.
	var allText string
	for _, line := range lines {
		for _, w := range line.Words {
			allText += w.Text
		}
	}
	if allText != "Supercalifragilisticexpialidocious" {
		t.Errorf("characters lost during word break: got %q", allText)
	}
}

func TestParagraphLineHeight(t *testing.T) {
	p := NewParagraph("Hello", font.Helvetica, 12)
	lines := p.Layout(500)
	expected := 14.4 // 12 * 1.2 (default leading)
	if math.Abs(lines[0].Height-expected) > 0.001 {
		t.Errorf("expected height %.1f, got %.3f", expected, lines[0].Height)
	}
}

func TestParagraphWordWidths(t *testing.T) {
	p := NewParagraph("AB", font.Helvetica, 10)
	lines := p.Layout(500)
	// A=667, B=667 → 1334/1000*10 = 13.34
	expected := 13.34
	if math.Abs(lines[0].Words[0].Width-expected) > 0.001 {
		t.Errorf("expected word width %.2f, got %.3f", expected, lines[0].Words[0].Width)
	}
}

func TestParagraphSetLeadingChaining(t *testing.T) {
	p := NewParagraph("Hello", font.Helvetica, 12).SetLeading(2.0).SetAlign(AlignCenter)
	lines := p.Layout(500)
	if lines[0].Height != 24.0 {
		t.Errorf("expected height 24.0, got %.3f", lines[0].Height)
	}
	if lines[0].Align != AlignCenter {
		t.Error("expected center alignment")
	}
}

func TestParagraphPanicsOnNilFont(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewParagraph with nil font should panic")
		}
	}()
	NewParagraph("text", nil, 12)
}

func TestParagraphPanicsOnZeroFontSize(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewParagraph with zero fontSize should panic")
		}
	}()
	NewParagraph("text", font.Helvetica, 0)
}

func TestParagraphPanicsOnNegativeFontSize(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewParagraph with negative fontSize should panic")
		}
	}()
	NewParagraph("text", font.Helvetica, -5)
}

func TestEmptyParagraphPreservesSpacing(t *testing.T) {
	p := NewParagraph("", font.Helvetica, 12).SetSpaceBefore(10).SetSpaceAfter(8)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line for empty paragraph with spacing, got %d", len(lines))
	}
	if lines[0].Height != 0 {
		t.Errorf("expected Height=0, got %f", lines[0].Height)
	}
	if lines[0].SpaceBefore != 10 {
		t.Errorf("expected SpaceBefore=10, got %f", lines[0].SpaceBefore)
	}
	if lines[0].SpaceAfterV != 8 {
		t.Errorf("expected SpaceAfterV=8, got %f", lines[0].SpaceAfterV)
	}
	if !lines[0].IsLast {
		t.Error("expected IsLast=true")
	}
}

// --- Sprint B: Box model tests ---

func TestParagraphSpaceBefore(t *testing.T) {
	p := NewParagraph("Hello", font.Helvetica, 12).SetSpaceBefore(10)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatal("expected 1 line")
	}
	if lines[0].SpaceBefore != 10 {
		t.Errorf("expected SpaceBefore=10, got %f", lines[0].SpaceBefore)
	}
}

func TestParagraphSpaceAfter(t *testing.T) {
	p := NewParagraph("Hello", font.Helvetica, 12).SetSpaceAfter(8)
	lines := p.Layout(500)
	last := lines[len(lines)-1]
	if last.SpaceAfterV != 8 {
		t.Errorf("expected SpaceAfterV=8, got %f", last.SpaceAfterV)
	}
}

func TestParagraphSpaceBeforeMultiLine(t *testing.T) {
	// SpaceBefore only on first line, SpaceAfter only on last line.
	// Make it wrap by using a very narrow width.
	longText := ""
	for range 50 {
		longText += "word "
	}
	p2 := NewParagraph(longText, font.Helvetica, 12).SetSpaceBefore(6).SetSpaceAfter(4)
	lines := p2.Layout(100)
	if len(lines) < 2 {
		t.Skip("not enough lines to test multi-line spacing")
	}
	if lines[0].SpaceBefore != 6 {
		t.Errorf("first line SpaceBefore: expected 6, got %f", lines[0].SpaceBefore)
	}
	if lines[0].SpaceAfterV != 0 {
		t.Errorf("first line SpaceAfterV should be 0, got %f", lines[0].SpaceAfterV)
	}
	last := lines[len(lines)-1]
	if last.SpaceAfterV != 4 {
		t.Errorf("last line SpaceAfterV: expected 4, got %f", last.SpaceAfterV)
	}
	if last.SpaceBefore != 0 {
		t.Errorf("last line SpaceBefore should be 0, got %f", last.SpaceBefore)
	}
}

func TestParagraphBackground(t *testing.T) {
	bg := RGB(0.9, 0.9, 0.9)
	p := NewParagraph("Hello", font.Helvetica, 12).SetBackground(bg)
	lines := p.Layout(500)
	if lines[0].Background == nil {
		t.Fatal("expected Background to be set")
	}
	if *lines[0].Background != bg {
		t.Errorf("expected background %+v, got %+v", bg, *lines[0].Background)
	}
}

func TestParagraphBackgroundAllLines(t *testing.T) {
	bg := RGB(1, 1, 0.8)
	longText := ""
	for range 50 {
		longText += "word "
	}
	p := NewParagraph(longText, font.Helvetica, 12).SetBackground(bg)
	lines := p.Layout(100)
	for i, l := range lines {
		if l.Background == nil {
			t.Errorf("line %d: expected Background to be set", i)
		}
	}
}

func TestDecorationUnderline(t *testing.T) {
	r := Run("underlined", font.Helvetica, 12).WithUnderline()
	if r.Decoration&DecorationUnderline == 0 {
		t.Error("expected underline decoration")
	}
	p := NewStyledParagraph(r)
	lines := p.Layout(500)
	if lines[0].Words[0].Decoration&DecorationUnderline == 0 {
		t.Error("word should have underline decoration")
	}
}

func TestDecorationStrikethrough(t *testing.T) {
	r := Run("struck", font.Helvetica, 12).WithStrikethrough()
	if r.Decoration&DecorationStrikethrough == 0 {
		t.Error("expected strikethrough decoration")
	}
}

func TestDecorationBoth(t *testing.T) {
	r := Run("both", font.Helvetica, 12).WithUnderline().WithStrikethrough()
	if r.Decoration&DecorationUnderline == 0 {
		t.Error("expected underline")
	}
	if r.Decoration&DecorationStrikethrough == 0 {
		t.Error("expected strikethrough")
	}
}

func TestStyledParagraphPanicsOnNilFontRun(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewStyledParagraph with nil Font and nil Embedded should panic")
		}
	}()
	NewStyledParagraph(TextRun{Text: "bad", FontSize: 12})
}

func TestHeadingKeepWithNext(t *testing.T) {
	h := NewHeading("Title", H1)
	lines := h.Layout(500)
	last := lines[len(lines)-1]
	if !last.KeepWithNext {
		t.Error("heading should have KeepWithNext on last line")
	}
}

func TestHeadingSpaceBefore(t *testing.T) {
	h := NewHeading("Title", H2)
	lines := h.Layout(500)
	// H2 size is 24, spacing is 24 * 0.5 = 12
	expected := 24.0 * 0.5
	diff := lines[0].SpaceBefore - expected
	if diff > 0.01 || diff < -0.01 {
		t.Errorf("expected SpaceBefore=%.1f, got %.1f", expected, lines[0].SpaceBefore)
	}
}

func TestParagraphEmptyTextPlanLayout(t *testing.T) {
	// Empty paragraph should produce LayoutFull with zero consumed height
	// (plus any spacing).
	p := NewParagraph("", font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull for empty paragraph, got %d", plan.Status)
	}
}

func TestParagraphZeroWidthLayout(t *testing.T) {
	// Layout with zero max width should not panic.
	p := NewParagraph("Hello World", font.Helvetica, 12)
	lines := p.Layout(0)
	// All text gets broken into character-level chunks.
	// Should not panic.
	if len(lines) == 0 {
		t.Error("expected at least 1 line even with 0 width")
	}
}

// TestParagraphNewlineBreak verifies that \n in paragraph text creates
// a forced line break, producing separate lines in the output.
func TestParagraphNewlineBreak(t *testing.T) {
	p := NewParagraph("Line one\nLine two\nLine three", font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	// Should produce 3 lines (one per \n-separated segment).
	if len(plan.Blocks) != 3 {
		t.Errorf("expected 3 lines, got %d", len(plan.Blocks))
	}
}

// TestParagraphNewlineInTable verifies the use case from issue #61:
// multi-line address text in a table cell using \n.
func TestParagraphNewlineInTable(t *testing.T) {
	tbl := NewTable().SetColumnUnitWidths([]UnitValue{Pct(50), Pct(50)})
	r := tbl.AddRow()
	r.AddCell("Postcode", font.Helvetica, 10)

	addr := NewParagraph("123 Main St\nSuite 456\nNew York, NY 10001", font.Helvetica, 10)
	r.AddCellElement(addr)

	plan := tbl.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	// The address cell should be taller than a single-line cell because
	// it contains 3 lines.
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

// TestParagraphNewlineEmpty verifies that consecutive \n\n produces
// a visual blank line (empty line between content lines).
func TestParagraphNewlineEmpty(t *testing.T) {
	p := NewParagraph("Before\n\nAfter", font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	// "Before" on line 1, empty line 2 (no words), "After" on line 3.
	// Empty lines between \n\n may collapse since there are no words.
	// At minimum we should get 2 lines (Before and After).
	if len(plan.Blocks) < 2 {
		t.Errorf("expected at least 2 lines, got %d", len(plan.Blocks))
	}
}

// TestParagraphNewlineTrailing verifies that trailing \n doesn't
// produce extra empty content.
func TestParagraphNewlineTrailing(t *testing.T) {
	p := NewParagraph("Hello\n", font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	// Should have 1 line ("Hello"), trailing \n doesn't add a line.
	if len(plan.Blocks) != 1 {
		t.Errorf("expected 1 line, got %d", len(plan.Blocks))
	}
}

// --- Text highlight / background color ---

func TestWithBackgroundColor(t *testing.T) {
	bg := RGB(1, 1, 0)
	run := Run("highlight", font.Helvetica, 12).WithBackgroundColor(bg)
	if run.BackgroundColor == nil {
		t.Fatal("expected BackgroundColor to be set")
	}
	if *run.BackgroundColor != bg {
		t.Errorf("BackgroundColor = %v, want %v", *run.BackgroundColor, bg)
	}
}

func TestParagraphBackgroundColorPropagates(t *testing.T) {
	bg := RGB(1, 1, 0)
	p := NewStyledParagraph(
		Run("Hello ", font.Helvetica, 12),
		Run("World", font.Helvetica, 12).WithBackgroundColor(bg),
	)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "World" should carry the background color.
	found := false
	for _, w := range lines[0].Words {
		if w.BackgroundColor != nil && *w.BackgroundColor == bg {
			found = true
		}
	}
	if !found {
		t.Error("expected a word with BackgroundColor set")
	}
}

func TestParagraphHighlightRendersWithPlanLayout(t *testing.T) {
	bg := RGB(1, 1, 0)
	p := NewStyledParagraph(
		Run("Normal ", font.Helvetica, 12),
		Run("highlighted", font.Helvetica, 12).WithBackgroundColor(bg),
		Run(" normal", font.Helvetica, 12),
	)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}
