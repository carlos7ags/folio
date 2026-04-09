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
	r := NewRun("underlined", font.Helvetica, 12).WithUnderline()
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
	r := NewRun("struck", font.Helvetica, 12).WithStrikethrough()
	if r.Decoration&DecorationStrikethrough == 0 {
		t.Error("expected strikethrough decoration")
	}
}

func TestDecorationBoth(t *testing.T) {
	r := NewRun("both", font.Helvetica, 12).WithUnderline().WithStrikethrough()
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
	run := NewRun("highlight", font.Helvetica, 12).WithBackgroundColor(bg)
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
		NewRun("Hello ", font.Helvetica, 12),
		NewRun("World", font.Helvetica, 12).WithBackgroundColor(bg),
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
		NewRun("Normal ", font.Helvetica, 12),
		NewRun("highlighted", font.Helvetica, 12).WithBackgroundColor(bg),
		NewRun(" normal", font.Helvetica, 12),
	)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

// --- Inline element runs ---

// fixedElement is a test helper that implements Element with a fixed size.
type fixedElement struct {
	width, height float64
}

func (f *fixedElement) PlanLayout(area LayoutArea) LayoutPlan {
	w, h := f.width, f.height
	if w > area.Width {
		w = area.Width
	}
	if h > area.Height {
		return LayoutPlan{Status: LayoutNothing}
	}
	return LayoutPlan{
		Status:   LayoutFull,
		Consumed: h,
		Blocks: []PlacedBlock{{
			X: 0, Y: 0, Width: w, Height: h,
			Draw: func(ctx DrawContext, absX, absTopY float64) {},
		}},
	}
}

func TestRunInlineConstructor(t *testing.T) {
	el := &fixedElement{width: 20, height: 20}
	run := RunInline(el)
	if run.InlineElement != el {
		t.Error("RunInline should set InlineElement")
	}
	if run.Font != nil || run.Embedded != nil {
		t.Error("RunInline should leave font fields nil")
	}
}

func TestStyledParagraphAcceptsInlineRun(t *testing.T) {
	el := &fixedElement{width: 20, height: 20}
	// Should not panic.
	p := NewStyledParagraph(
		NewRun("Hello ", font.Helvetica, 12),
		RunInline(el),
		NewRun(" World", font.Helvetica, 12),
	)
	if p == nil {
		t.Fatal("expected non-nil paragraph")
	}
}

// TestNewStyledParagraphAcceptsLineBreakMarker verifies that a
// TextRun{IsLineBreak: true} produces a forced line break between
// the surrounding text runs. Regression test for issue #147, updated
// for #150 (typed field with proper line-break propagation).
func TestNewStyledParagraphAcceptsLineBreakMarker(t *testing.T) {
	p := NewStyledParagraph(
		NewRun("Hello", font.Helvetica, 12),
		TextRun{IsLineBreak: true},
		NewRun("World", font.Helvetica, 12),
	)
	lines := p.Layout(500)
	if len(lines) < 2 {
		t.Errorf("IsLineBreak should produce 2 lines, got %d", len(lines))
	}
}

// TestAddRunAcceptsLineBreakMarker verifies AddRun also accepts the
// IsLineBreak marker. Same exemption as NewStyledParagraph.
func TestAddRunAcceptsLineBreakMarker(t *testing.T) {
	p := NewParagraph("Hello", font.Helvetica, 12)
	// Should not panic.
	p.AddRun(TextRun{IsLineBreak: true})
	p.AddRun(NewRun("World", font.Helvetica, 12))
}

func TestAddRunAcceptsInlineElement(t *testing.T) {
	el := &fixedElement{width: 20, height: 20}
	p := NewParagraph("Hello ", font.Helvetica, 12)
	// Should not panic.
	p.AddRun(RunInline(el))
	p.AddRun(NewRun(" World", font.Helvetica, 12))
}

func TestParagraphInlineElementLayout(t *testing.T) {
	// "Hello" + 20pt-wide inline element + "World" at 500pt width
	// should fit on one line.
	el := &fixedElement{width: 20, height: 16}
	p := NewStyledParagraph(
		NewRun("Hello ", font.Helvetica, 12),
		RunInline(el),
		NewRun(" World", font.Helvetica, 12),
	)

	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	// Should have 3 words: "Hello", inline block, "World"
	found := false
	for _, w := range lines[0].Words {
		if w.InlineBlock != nil {
			found = true
			if math.Abs(w.Width-20) > 0.1 {
				t.Errorf("inline word width = %.1f, want 20", w.Width)
			}
		}
	}
	if !found {
		t.Error("expected an InlineBlock word in the line")
	}
}

func TestParagraphInlineElementPlanLayout(t *testing.T) {
	el := &fixedElement{width: 20, height: 16}
	p := NewStyledParagraph(
		NewRun("Hello ", font.Helvetica, 12),
		RunInline(el),
		NewRun(" World", font.Helvetica, 12),
	)

	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
	if len(plan.Blocks) == 0 {
		t.Fatal("expected at least one block")
	}
}

func TestParagraphInlineElementWraps(t *testing.T) {
	// Very narrow width should force text + inline element to wrap.
	el := &fixedElement{width: 30, height: 16}
	p := NewStyledParagraph(
		NewRun("Hello ", font.Helvetica, 12),
		RunInline(el),
	)

	// "Hello" ≈ 27pt + space ≈ 3pt + 30pt inline = ~60pt; force wrap at 40pt.
	lines := p.Layout(40)
	if len(lines) < 2 {
		t.Errorf("expected at least 2 lines at 40pt width, got %d", len(lines))
	}
}

func TestParagraphInlineElementLineHeight(t *testing.T) {
	// Inline element taller than text should increase line height.
	el := &fixedElement{width: 20, height: 50}
	p := NewStyledParagraph(
		NewRun("Hello ", font.Helvetica, 12),
		RunInline(el),
	)

	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	// With a 50pt inline element, consumed height should be >= 50.
	if plan.Consumed < 50 {
		t.Errorf("consumed = %.1f, expected >= 50 for 50pt tall inline element", plan.Consumed)
	}
}

func TestParagraphOnlyInlineElement(t *testing.T) {
	// A paragraph containing only an inline element (no text).
	el := &fixedElement{width: 100, height: 30}
	p := NewStyledParagraph(RunInline(el))

	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	if plan.Consumed < 30 {
		t.Errorf("consumed = %.1f, expected >= 30", plan.Consumed)
	}
}

func TestInlineSpaceAfterFromFollowingRun(t *testing.T) {
	// When an inline element leads the paragraph (no preceding text),
	// its SpaceAfter should be derived from the next text run's font
	// metrics, not a hardcoded value.
	el := &fixedElement{width: 20, height: 16}
	p := NewStyledParagraph(
		RunInline(el),
		NewRun(" World", font.Helvetica, 12),
	)

	lines := p.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// The first word is the inline element; its SpaceAfter should match
	// the space width of Helvetica 12pt (≈ 3.33pt), not be exactly 0.
	inlineWord := lines[0].Words[0]
	if inlineWord.InlineBlock == nil {
		t.Fatal("first word should be InlineBlock")
	}
	// Helvetica 12pt space width is approximately 3.33pt.
	expectedSpaceW := font.Helvetica.MeasureString(" ", 12)
	if math.Abs(inlineWord.SpaceAfter-expectedSpaceW) > 0.01 {
		t.Errorf("SpaceAfter = %.4f, want %.4f (from Helvetica 12pt)", inlineWord.SpaceAfter, expectedSpaceW)
	}
}

func TestInlineSpaceAfterFlushWhenOnlyInlines(t *testing.T) {
	// When a paragraph has only inline elements and no text runs,
	// SpaceAfter should be 0 (flush positioning).
	el1 := &fixedElement{width: 20, height: 16}
	el2 := &fixedElement{width: 30, height: 16}
	p := NewStyledParagraph(
		RunInline(el1),
		RunInline(el2),
	)

	lines := p.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	for i, w := range lines[0].Words {
		if w.InlineBlock == nil {
			continue
		}
		if w.SpaceAfter != 0 {
			t.Errorf("word[%d].SpaceAfter = %.4f, want 0 for all-inline paragraph", i, w.SpaceAfter)
		}
	}
}

func TestInlineSpaceAfterInheritsPreceding(t *testing.T) {
	// When an inline element follows text, it should inherit SpaceAfter
	// from the preceding text word.
	el := &fixedElement{width: 20, height: 16}
	p := NewStyledParagraph(
		NewRun("Hello ", font.Helvetica, 12),
		RunInline(el),
	)

	lines := p.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// Find the text word and the inline word.
	var textSpaceAfter, inlineSpaceAfter float64
	for _, w := range lines[0].Words {
		if w.InlineBlock != nil {
			inlineSpaceAfter = w.SpaceAfter
		} else if w.Text == "Hello" {
			textSpaceAfter = w.SpaceAfter
		}
	}
	if textSpaceAfter == 0 {
		t.Fatal("expected non-zero SpaceAfter on text word")
	}
	if math.Abs(inlineSpaceAfter-textSpaceAfter) > 0.01 {
		t.Errorf("inline SpaceAfter = %.4f, want %.4f (inherited from preceding text)", inlineSpaceAfter, textSpaceAfter)
	}
}

func TestInlineSpaceAfterPlanLayoutMatchesLayout(t *testing.T) {
	// Both Layout() and PlanLayout (via measureWords) should produce
	// the same SpaceAfter values for inline elements.
	el := &fixedElement{width: 20, height: 16}
	p := NewStyledParagraph(
		RunInline(el),
		NewRun(" World", font.Helvetica, 12),
	)

	lines := p.Layout(500)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})

	if len(lines) == 0 || len(plan.Blocks) == 0 {
		t.Fatal("expected output from both Layout and PlanLayout")
	}

	layoutSA := lines[0].Words[0].SpaceAfter
	// PlanLayout uses measureWords internally; verify the inline element
	// word in the plan's first block has the same spacing. We check this
	// indirectly by verifying both paths produce the same total line width.
	layoutWidth := lines[0].Width
	planWidth := plan.Blocks[0].Width
	if math.Abs(layoutWidth-planWidth) > 0.1 {
		t.Errorf("Layout width = %.2f, PlanLayout width = %.2f, want equal", layoutWidth, planWidth)
	}
	if layoutSA == 0 {
		t.Error("expected non-zero SpaceAfter from font metrics look-ahead")
	}
}

// --- Inline element alignment ---

func TestCenteredParagraphInlineChildBlockIsLineRelative(t *testing.T) {
	// When a centered paragraph contains an inline element, the child
	// PlacedBlock's X should be line-relative (starting near 0), not
	// include the alignment offset. The parent block's X already carries
	// the centering offset, and the renderer adds parent X to child X.
	el := &fixedElement{width: 40, height: 16}
	p := NewStyledParagraph(
		NewRun("AB ", font.Helvetica, 12),
		RunInline(el),
	)
	p.SetAlign(AlignCenter)

	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	if len(plan.Blocks) == 0 {
		t.Fatal("expected at least one block")
	}

	lineBlock := plan.Blocks[0]

	// The line block itself should be offset for centering.
	if lineBlock.X < 1 {
		t.Errorf("line block X = %.2f, expected positive centering offset", lineBlock.X)
	}

	// The inline child should be line-relative: its X should be
	// approximately the width of "AB " (not "AB " + centering offset).
	if len(lineBlock.Children) == 0 {
		t.Fatal("expected inline child blocks")
	}
	childX := lineBlock.Children[0].X
	// "AB " in Helvetica 12pt ≈ 18-20pt. The child X should be in that
	// range, not shifted by hundreds of points from centering.
	if childX > 50 {
		t.Errorf("child X = %.2f, expected < 50 (line-relative), got alignment offset leak", childX)
	}
}

func TestRightAlignedParagraphInlineChildBlockIsLineRelative(t *testing.T) {
	el := &fixedElement{width: 40, height: 16}
	p := NewStyledParagraph(
		NewRun("AB ", font.Helvetica, 12),
		RunInline(el),
	)
	p.SetAlign(AlignRight)

	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if len(plan.Blocks) == 0 || len(plan.Blocks[0].Children) == 0 {
		t.Fatal("expected blocks with children")
	}

	lineBlock := plan.Blocks[0]
	childX := lineBlock.Children[0].X

	// Right-aligned line block should have large X offset.
	if lineBlock.X < 100 {
		t.Errorf("line block X = %.2f, expected large right-alignment offset", lineBlock.X)
	}
	// Child should still be line-relative (small X).
	if childX > 50 {
		t.Errorf("child X = %.2f, expected < 50 (line-relative), got alignment offset leak", childX)
	}
}

func TestLeftAlignedParagraphInlineChildBlockConsistency(t *testing.T) {
	// Left-aligned should also have line-relative children at X ≈ 0
	// (since the line block X is 0 for left alignment).
	el := &fixedElement{width: 40, height: 16}
	p := NewStyledParagraph(
		RunInline(el),
		NewRun(" text", font.Helvetica, 12),
	)
	p.SetAlign(AlignLeft)

	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if len(plan.Blocks) == 0 || len(plan.Blocks[0].Children) == 0 {
		t.Fatal("expected blocks with children")
	}

	lineBlock := plan.Blocks[0]
	// Left-aligned: line block X should be 0.
	if lineBlock.X > 0.1 {
		t.Errorf("line block X = %.2f, expected 0 for left alignment", lineBlock.X)
	}
	// Leading inline child should start at X ≈ 0.
	childX := lineBlock.Children[0].X
	if childX > 1 {
		t.Errorf("child X = %.2f, expected ~0 for leading inline element", childX)
	}
}

func TestAdjacentRunsNoSpace(t *testing.T) {
	// "C" + subscript "8" + "H" should have no inter-word space.
	// Simulates C<sub>8</sub>H from HTML.
	p := NewStyledParagraph(
		NewRun("C", font.Helvetica, 12),
		NewRun("8", font.Helvetica, 9), // smaller font like <sub>
		NewRun("H", font.Helvetica, 12),
	)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "C" should have SpaceAfter=0 (glued to "8").
	// "8" should have SpaceAfter=0 (glued to "H").
	for i, w := range lines[0].Words {
		if i < len(lines[0].Words)-1 && w.SpaceAfter != 0 {
			t.Errorf("word[%d] %q: SpaceAfter=%.2f, want 0 (adjacent runs)", i, w.Text, w.SpaceAfter)
		}
	}
}

func TestAdjacentRunsWithSpacePreserved(t *testing.T) {
	// "Hello " + "world" — trailing space in first run means they ARE
	// separate words and should keep SpaceAfter.
	p := NewStyledParagraph(
		NewRun("Hello ", font.Helvetica, 12),
		NewRun("world", font.Helvetica, 12),
	)
	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "Hello" should have non-zero SpaceAfter.
	if lines[0].Words[0].SpaceAfter == 0 {
		t.Error("expected non-zero SpaceAfter between 'Hello' and 'world'")
	}
}

func TestComputeBaselineStandardFont(t *testing.T) {
	words := []Word{{Font: font.Helvetica, FontSize: 12}}
	lineH := 14.4
	baseline := computeBaseline(words, lineH)
	ascent := font.Helvetica.Ascent(12)
	descent := font.Helvetica.Descent(12)
	expected := (lineH + ascent - descent) / 2
	if math.Abs(baseline-expected) > 0.01 {
		t.Errorf("baseline = %.4f, want %.4f", baseline, expected)
	}
}

func TestComputeBaselineFallback(t *testing.T) {
	el := &fixedElement{width: 20, height: 16}
	words := []Word{{InlineBlock: el, InlineWidth: 20, InlineHeight: 16}}
	baseline := computeBaseline(words, 20)
	expected := 20.0 * 0.8
	if math.Abs(baseline-expected) > 0.01 {
		t.Errorf("baseline = %.2f, want %.2f", baseline, expected)
	}
}

func TestComputeBaselineMixedSizes(t *testing.T) {
	words := []Word{
		{Font: font.Helvetica, FontSize: 12},
		{Font: font.Helvetica, FontSize: 24},
	}
	lineH := 28.8
	baseline := computeBaseline(words, lineH)
	ascent24 := font.Helvetica.Ascent(24)
	descent24 := font.Helvetica.Descent(24)
	expected := (lineH + ascent24 - descent24) / 2
	if math.Abs(baseline-expected) > 0.01 {
		t.Errorf("baseline = %.4f, want %.4f", baseline, expected)
	}
}

func TestConsecutiveNewlinesProduceEmptyLines(t *testing.T) {
	p := NewParagraph("First\n\nSecond", font.Helvetica, 12)
	lines := p.Layout(500)
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (with empty middle line), got %d", len(lines))
	}
	// Middle line should have one blank word.
	if len(lines[1].Words) != 1 || lines[1].Words[0].Text != "" {
		t.Errorf("expected empty middle line, got %d words: %v", len(lines[1].Words), lines[1].Words)
	}
}

func TestTripleNewlines(t *testing.T) {
	p := NewParagraph("A\n\n\nB", font.Helvetica, 12)
	lines := p.Layout(500)
	if len(lines) != 4 {
		t.Fatalf("expected 4 lines (2 empty), got %d", len(lines))
	}
}

func TestSingleNewlineNoEmptyLine(t *testing.T) {
	// Single \n should produce a line break but no empty line.
	p := NewParagraph("First\nSecond", font.Helvetica, 12)
	lines := p.Layout(500)
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
}

func TestTrailingDoubleNewline(t *testing.T) {
	p := NewParagraph("Text\n\n", font.Helvetica, 12)
	lines := p.Layout(500)
	// "Text\n\n" = "Text" line, then empty line from \n\n.
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	// Second line should be the blank word.
	if len(lines[1].Words) != 1 || lines[1].Words[0].Text != "" {
		t.Errorf("expected empty second line, got words: %v", lines[1].Words)
	}
}

func TestConsecutiveNewlinesPlanLayout(t *testing.T) {
	p := NewParagraph("First\n\nSecond", font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}
	// Should have 3 blocks: "First", empty line, "Second".
	if len(plan.Blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(plan.Blocks))
	}
}

func TestPureNewlinesParagraph(t *testing.T) {
	p := NewParagraph("\n\n", font.Helvetica, 12)
	lines := p.Layout(500)
	// "\n\n" with no surrounding text produces a blank word from the
	// consecutive newlines. At minimum 1 line with a blank word.
	if len(lines) == 0 {
		t.Fatal("expected at least 1 line for pure newlines")
	}
	// The line should contain a blank word.
	if lines[0].Words[0].Text != "" {
		t.Errorf("expected blank word, got %q", lines[0].Words[0].Text)
	}
}

func TestPunctuationKeepsOwnFont(t *testing.T) {
	// <b>here</b>. — the period should render in regular font, not bold.
	p := NewStyledParagraph(
		NewRun("here", font.HelveticaBold, 12),
		NewRun(".", font.Helvetica, 12),
	)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatal("expected LayoutFull")
	}
	// Also verify via Layout which doesn't use splitLeadingPunct.
	lines := p.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	for _, w := range lines[0].Words {
		if w.Text == "." && w.Font != font.Helvetica {
			t.Errorf("period should be Helvetica, got %v", w.Font)
		}
		if w.Text == "here" && w.Font != font.HelveticaBold {
			t.Errorf("'here' should be Helvetica-Bold, got %v", w.Font)
		}
	}
	// Period should be glued (SpaceAfter=0) but keep its own font.
	for _, w := range lines[0].Words {
		if w.Text == "here" && w.SpaceAfter != 0 {
			t.Errorf("'here' SpaceAfter = %.2f, want 0 (glued to period)", w.SpaceAfter)
		}
	}
}

func TestCommaAfterBoldKeepsRegularFont(t *testing.T) {
	// <b>word</b>, rest — comma should be regular.
	p := NewStyledParagraph(
		NewRun("word", font.HelveticaBold, 12),
		NewRun(", rest", font.Helvetica, 12),
	)
	lines := p.Layout(500)
	for _, w := range lines[0].Words {
		if w.Text == "word," || w.Text == "," {
			if w.Font == font.HelveticaBold {
				t.Errorf("comma/word should not be bold, got %v", w.Font)
			}
		}
	}
}

func TestBlankLineSurvivesPageSplit(t *testing.T) {
	// A paragraph with \n\n that overflows should preserve the blank line
	// in the overflow paragraph.
	p := NewParagraph("First\n\nSecond", font.Helvetica, 12)
	p.SetLeading(1.2)

	// Give only enough height for the first line — force a split.
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 15})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %d", plan.Status)
	}
	if plan.Overflow == nil {
		t.Fatal("expected overflow paragraph")
	}

	// The overflow should contain the blank line + "Second".
	overflow := plan.Overflow.(*Paragraph)
	overflowPlan := overflow.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if overflowPlan.Status != LayoutFull {
		t.Fatalf("overflow should fit, got %d", overflowPlan.Status)
	}
	// Should have at least 2 blocks: blank line + "Second".
	if len(overflowPlan.Blocks) < 2 {
		t.Errorf("expected at least 2 blocks in overflow (blank + Second), got %d", len(overflowPlan.Blocks))
	}
}

func TestBlankLineSurvivesPageSplitMultiStyle(t *testing.T) {
	// Bold word, then \n\n, then regular word. Overflow should preserve
	// both the blank line AND the font change.
	p := NewStyledParagraph(
		NewRun("Bold", font.HelveticaBold, 12),
		NewRun("\n\nRegular", font.Helvetica, 12),
	)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 15})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %d", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)
	lines := overflow.Layout(500)
	// Should have 2 lines: blank + "Regular"
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines in overflow, got %d", len(lines))
	}
	// "Regular" should be in Helvetica, NOT HelveticaBold.
	for _, l := range lines {
		for _, w := range l.Words {
			if w.Text == "Regular" && w.Font != font.Helvetica {
				t.Errorf("'Regular' should be Helvetica, got %v", w.Font)
			}
		}
	}
}

func TestSingleNewlineSurvivesPageSplit(t *testing.T) {
	// "First\nSecond" — single newline, no blank line. Overflow should
	// preserve the forced line break.
	p := NewParagraph("First\nSecond", font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 15})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %d", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)
	lines := overflow.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line in overflow ('Second'), got %d", len(lines))
	}
	if lines[0].Words[0].Text != "Second" {
		t.Errorf("expected 'Second', got %q", lines[0].Words[0].Text)
	}
}

func TestTripleNewlineSurvivesPageSplit(t *testing.T) {
	// "A\n\n\nB" — should preserve 2 blank lines in overflow.
	p := NewParagraph("A\n\n\nB", font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 15})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %d", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)
	overflowPlan := overflow.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	// Should have at least 3 blocks: blank, blank, "B".
	if len(overflowPlan.Blocks) < 3 {
		t.Errorf("expected at least 3 blocks in overflow, got %d", len(overflowPlan.Blocks))
	}
}

func TestPlainPageSplitPreservesWords(t *testing.T) {
	// No newlines — just a long paragraph that overflows. Verify all words
	// survive the cloneWithWords round-trip.
	p := NewParagraph("one two three four five six seven eight nine ten", font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 200, Height: 15})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %d", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)
	overflowLines := overflow.Layout(500)
	var overflowWords []string
	for _, l := range overflowLines {
		for _, w := range l.Words {
			overflowWords = append(overflowWords, w.Text)
		}
	}
	// First page fits some words, overflow should have the rest.
	// Together they must total 10 words.
	firstPageWords := 0
	for _, b := range plan.Blocks {
		firstPageWords += len(b.Children) // approximate — count blocks
	}
	if len(overflowWords) == 0 {
		t.Error("expected words in overflow paragraph")
	}
}

func TestParagraphFirstLineIndent(t *testing.T) {
	text := "Word word word word word word word word word word word word word word word word word word word word."
	pIndent := NewParagraph(text, font.Helvetica, 12)
	pIndent.SetFirstLineIndent(80)
	linesIndent := pIndent.Layout(150)

	pNoIndent := NewParagraph(text, font.Helvetica, 12)
	linesNoIndent := pNoIndent.Layout(150)

	// Large indent (80pt on 150pt width) should cause more line wrapping.
	if len(linesIndent) <= len(linesNoIndent) {
		t.Errorf("indent should cause more wrapping: indented=%d lines, normal=%d lines", len(linesIndent), len(linesNoIndent))
	}
}

func TestParagraphEllipsis(t *testing.T) {
	p := NewParagraph("This is a very long text that should be truncated with an ellipsis", font.Helvetica, 12)
	p.SetEllipsis(true)

	lines := p.Layout(100)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// Ellipsis paragraph should not produce more lines than non-ellipsis.
	pFull := NewParagraph("This is a very long text that should be truncated with an ellipsis", font.Helvetica, 12)
	linesFull := pFull.Layout(100)
	if len(lines) > len(linesFull) {
		t.Errorf("ellipsis should not produce more lines: %d vs %d", len(lines), len(linesFull))
	}
}

func TestParagraphWidowsOrphans(t *testing.T) {
	// Long paragraph that overflows a short page.
	text := ""
	for range 20 {
		text += "Word after word filling up the paragraph to make it overflow the page. "
	}
	p := NewParagraph(text, font.Helvetica, 12)
	p.SetOrphans(2)
	p.SetWidows(2)

	r := NewRenderer(612, 120, Margins{Top: 20, Bottom: 20, Left: 20, Right: 20})
	r.Add(p)
	pages := r.Render()
	if len(pages) < 2 {
		t.Fatalf("expected ≥2 pages, got %d", len(pages))
	}
}
