// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"math"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

const splitFloatEpsilon = 1e-9

func splitNearlyEqual(a, b float64) bool {
	return math.Abs(a-b) < splitFloatEpsilon
}

// --- Basic shape -----------------------------------------------------------

func TestSplitAfterLineNoOverflow(t *testing.T) {
	p := NewParagraph("Hello World", font.Helvetica, 12)
	head, tail := p.SplitAfterLine(5, 500)
	if head == nil {
		t.Fatal("head should be non-nil when n >= total lines")
	}
	if tail != nil {
		t.Error("tail should be nil when n >= total lines")
	}
	if got := head.MeasureLines(500); got != 1 {
		t.Errorf("head lines = %d, want 1", got)
	}
}

func TestSplitAfterLineZero(t *testing.T) {
	text := strings.Repeat("alpha beta gamma delta ", 6)
	p := NewParagraph(text, font.Helvetica, 12)
	total := p.MeasureLines(80)

	head, tail := p.SplitAfterLine(0, 80)
	if head != nil {
		t.Error("head should be nil when n <= 0")
	}
	if tail == nil {
		t.Fatal("tail should be non-nil when n <= 0")
	}
	if got := tail.MeasureLines(80); got != total {
		t.Errorf("tail lines = %d, want %d (entire paragraph)", got, total)
	}
}

func TestSplitAfterLineNegative(t *testing.T) {
	p := NewParagraph("Hello World", font.Helvetica, 12)
	head, tail := p.SplitAfterLine(-3, 500)
	if head != nil {
		t.Error("head should be nil for negative n")
	}
	if tail == nil {
		t.Fatal("tail should be non-nil for negative n")
	}
}

func TestSplitAfterLineExactBoundary(t *testing.T) {
	// Splitting exactly at the total line count returns full head, nil tail.
	text := strings.Repeat("alpha beta gamma delta ", 6)
	p := NewParagraph(text, font.Helvetica, 12)
	total := p.MeasureLines(80)

	head, tail := p.SplitAfterLine(total, 80)
	if head == nil {
		t.Fatal("head should be non-nil at exact boundary")
	}
	if tail != nil {
		t.Errorf("tail should be nil at exact boundary, got %d lines", tail.MeasureLines(80))
	}
	if got := head.MeasureLines(80); got != total {
		t.Errorf("head lines = %d, want %d", got, total)
	}
}

func TestSplitAfterLineDoesNotMutateReceiver(t *testing.T) {
	text := strings.Repeat("alpha beta gamma delta ", 6)
	p := NewParagraph(text, font.Helvetica, 12)
	before := p.MeasureLines(80)
	_, _ = p.SplitAfterLine(1, 80)
	after := p.MeasureLines(80)
	if before != after {
		t.Errorf("SplitAfterLine mutated receiver: before=%d, after=%d", before, after)
	}
}

func TestSplitAfterLineBasic(t *testing.T) {
	text := strings.Repeat("alpha beta gamma delta ", 8)
	p := NewParagraph(text, font.Helvetica, 12)
	total := p.MeasureLines(80)
	if total < 4 {
		t.Fatalf("test setup: expected ≥4 lines, got %d", total)
	}

	head, tail := p.SplitAfterLine(2, 80)
	if head == nil || tail == nil {
		t.Fatal("split mid-paragraph must return both head and tail")
	}
	if got := head.MeasureLines(80); got != 2 {
		t.Errorf("head lines = %d, want 2", got)
	}
	if head.MeasureLines(80)+tail.MeasureLines(80) != total {
		t.Errorf("split lost lines: head=%d + tail=%d != total=%d",
			head.MeasureLines(80), tail.MeasureLines(80), total)
	}
}

// --- Spacing ownership -----------------------------------------------------

func TestSplitAfterLineSpacingOwnership(t *testing.T) {
	text := strings.Repeat("alpha beta gamma delta ", 6)
	p := NewParagraph(text, font.Helvetica, 12).
		SetSpaceBefore(11).
		SetSpaceAfter(13)

	head, tail := p.SplitAfterLine(2, 80)
	if got := head.GetSpaceBefore(); got != 11 {
		t.Errorf("head spaceBefore = %v, want 11", got)
	}
	if got := head.GetSpaceAfter(); got != 0 {
		t.Errorf("head spaceAfter = %v, want 0 (belongs to tail)", got)
	}
	if got := tail.GetSpaceBefore(); got != 0 {
		t.Errorf("tail spaceBefore = %v, want 0 (belongs to head)", got)
	}
	if got := tail.GetSpaceAfter(); got != 13 {
		t.Errorf("tail spaceAfter = %v, want 13", got)
	}
}

// --- Re-lay at different width --------------------------------------------

func TestSplitAfterLineRelayDifferentWidth(t *testing.T) {
	// Re-laying head/tail at a narrower width should not panic; line
	// counts may differ (the contract is that words are real, not
	// pre-wrapped).
	text := strings.Repeat("alpha beta gamma delta ", 8)
	p := NewParagraph(text, font.Helvetica, 12)

	head, tail := p.SplitAfterLine(2, 240)
	if head == nil || tail == nil {
		t.Fatal("split should return both halves")
	}

	// Doesn't panic at the original width.
	_ = head.MeasureLines(240)
	_ = tail.MeasureLines(240)

	// Doesn't panic narrower or wider.
	if got := head.MeasureLines(60); got <= 0 {
		t.Errorf("head re-lay at 60 returned %d lines", got)
	}
	if got := head.MeasureLines(800); got <= 0 {
		t.Errorf("head re-lay at 800 returned %d lines", got)
	}
}

// --- Double split ---------------------------------------------------------

func TestSplitAfterLineTailSplitAgain(t *testing.T) {
	text := strings.Repeat("alpha beta gamma delta ", 12)
	p := NewParagraph(text, font.Helvetica, 12)
	total := p.MeasureLines(80)
	if total < 6 {
		t.Fatalf("test setup needs ≥6 lines, got %d", total)
	}

	_, tail1 := p.SplitAfterLine(2, 80)
	if tail1 == nil {
		t.Fatal("first split tail nil")
	}
	head2, tail2 := tail1.SplitAfterLine(2, 80)
	if head2 == nil || tail2 == nil {
		t.Fatal("second split should return both halves")
	}
	if got := head2.MeasureLines(80); got != 2 {
		t.Errorf("second head lines = %d, want 2", got)
	}
	// head2 is a head-of-tail: its parent (tail1) had spaceBefore=0,
	// so head2 must inherit 0 — not the original p.spaceBefore.
	if got := head2.GetSpaceBefore(); got != 0 {
		t.Errorf("head2 spaceBefore = %v, want 0 (parent tail1 had 0)", got)
	}
	// tail2 is also tail-of-tail.
	if got := tail2.GetSpaceBefore(); got != 0 {
		t.Errorf("tail2 spaceBefore = %v, want 0", got)
	}
}

// --- Style preservation ---------------------------------------------------

func TestSplitAfterLineStyledRunsSurvive(t *testing.T) {
	// Multiple runs with distinct styles. After splitting, the styles
	// must survive on whichever half each word landed in.
	style := func(s string, c Color) TextRun {
		return NewRun(s, font.Helvetica, 12).WithColor(c)
	}
	prefix := strings.Repeat("alpha beta gamma delta ", 8)
	p := NewStyledParagraph(
		style(prefix, ColorRed),
		style(strings.Repeat("rest ", 8), ColorBlack),
	)

	head, tail := p.SplitAfterLine(3, 80)
	if head == nil || tail == nil {
		t.Fatal("split should return both halves")
	}

	// Head should still contain at least one ColorRed run.
	hasRed := false
	for _, r := range head.Runs() {
		if r.Color == ColorRed {
			hasRed = true
			break
		}
	}
	if !hasRed {
		t.Error("head lost ColorRed styling")
	}
}

func TestSplitAfterLinePreservesLeading(t *testing.T) {
	text := strings.Repeat("alpha beta gamma delta ", 6)
	p := NewParagraph(text, font.Helvetica, 12).SetLeading(1.6)

	head, tail := p.SplitAfterLine(2, 80)
	if head == nil || tail == nil {
		t.Fatal("split should return both halves")
	}
	want := 12.0 * 1.6
	headLines := head.Layout(80)
	tailLines := tail.Layout(80)
	if len(headLines) > 0 && !splitNearlyEqual(headLines[0].Height, want) {
		t.Errorf("head line height = %v, want %v", headLines[0].Height, want)
	}
	if len(tailLines) > 0 && !splitNearlyEqual(tailLines[0].Height, want) {
		t.Errorf("tail line height = %v, want %v", tailLines[0].Height, want)
	}
}

// --- Forced line breaks ---------------------------------------------------

func TestSplitAfterLineForcedNewline(t *testing.T) {
	p := NewParagraph("alpha beta\ngamma delta\nepsilon zeta", font.Helvetica, 12)
	if total := p.MeasureLines(500); total != 3 {
		t.Fatalf("setup: expected 3 lines from forced \\n, got %d", total)
	}

	head, tail := p.SplitAfterLine(1, 500)
	if head == nil || tail == nil {
		t.Fatal("split should return both halves")
	}
	if got := head.MeasureLines(500); got != 1 {
		t.Errorf("head lines = %d, want 1", got)
	}
	if got := tail.MeasureLines(500); got != 2 {
		t.Errorf("tail lines = %d, want 2", got)
	}
}

func TestSplitAfterLineHTMLBr(t *testing.T) {
	// IsLineBreak runs (from <br>) follow a separate path through
	// Layout than \n in text. Verify SplitAfterLine handles them too.
	p := NewStyledParagraph(
		NewRun("alpha", font.Helvetica, 12),
		TextRun{IsLineBreak: true},
		NewRun("beta", font.Helvetica, 12),
		TextRun{IsLineBreak: true},
		NewRun("gamma", font.Helvetica, 12),
	)
	if total := p.MeasureLines(500); total != 3 {
		t.Fatalf("setup: expected 3 lines from <br>, got %d", total)
	}

	head, tail := p.SplitAfterLine(1, 500)
	if head == nil || tail == nil {
		t.Fatal("<br> split should return both halves")
	}
	if got := head.MeasureLines(500); got != 1 {
		t.Errorf("head lines = %d, want 1", got)
	}
	if got := tail.MeasureLines(500); got != 2 {
		t.Errorf("tail lines = %d, want 2", got)
	}
}

func TestSplitAfterLineBlankLineBoundary(t *testing.T) {
	// Paragraph with a blank line in the middle. Splitting at line 2
	// (the blank line) puts the blank in the head; tail starts with
	// the post-blank content.
	p := NewParagraph("alpha\n\nbeta", font.Helvetica, 12)
	total := p.MeasureLines(500)
	if total != 3 {
		t.Fatalf("setup: expected 3 lines (alpha, blank, beta), got %d", total)
	}

	head, tail := p.SplitAfterLine(1, 500)
	if head == nil || tail == nil {
		t.Fatal("blank-line split should return both halves")
	}
	// Head has just "alpha"; tail has the blank line + "beta".
	if got := head.MeasureLines(500); got != 1 {
		t.Errorf("head lines = %d, want 1", got)
	}
	if got := tail.MeasureLines(500); got != 2 {
		t.Errorf("tail lines = %d, want 2", got)
	}
}

// --- RTL ------------------------------------------------------------------

func TestSplitAfterLineRTLPreservesDirection(t *testing.T) {
	// Explicit RTL direction must round-trip through both halves.
	text := strings.Repeat("שלום עולם ", 8)
	p := NewParagraph(text, font.Helvetica, 12).SetDirection(DirectionRTL)
	total := p.MeasureLines(60)
	if total < 3 {
		t.Skipf("RTL test setup needs ≥3 lines; got %d", total)
	}

	head, tail := p.SplitAfterLine(2, 60)
	if head == nil || tail == nil {
		t.Fatal("RTL split should return both halves")
	}
	if head.Direction() != DirectionRTL {
		t.Errorf("head direction = %v, want DirectionRTL", head.Direction())
	}
	if tail.Direction() != DirectionRTL {
		t.Errorf("tail direction = %v, want DirectionRTL", tail.Direction())
	}
}

// --- Inline elements ------------------------------------------------------

func TestSplitAfterLinePreservesInlineImage(t *testing.T) {
	el := &fixedElement{width: 20, height: 16}
	prefix := strings.Repeat("alpha beta gamma delta ", 8)
	p := NewStyledParagraph(
		NewRun(prefix, font.Helvetica, 12),
		RunInline(el),
		NewRun(" tail tail tail", font.Helvetica, 12),
	)
	total := p.MeasureLines(80)
	if total < 3 {
		t.Fatalf("setup: needs ≥3 lines, got %d", total)
	}

	// Split such that the inline element falls in tail.
	head, tail := p.SplitAfterLine(1, 80)
	if head == nil || tail == nil {
		t.Fatal("split should return both halves")
	}
	// Find the inline element in either half. Where it lands depends
	// on the wrap, but it must survive *somewhere*.
	headHas, tailHas := false, false
	for _, run := range head.Runs() {
		if run.InlineElement == el {
			headHas = true
			break
		}
	}
	for _, run := range tail.Runs() {
		if run.InlineElement == el {
			tailHas = true
			break
		}
	}
	if !headHas && !tailHas {
		t.Errorf("inline element lost across SplitAfterLine: head runs=%d tail runs=%d",
			len(head.Runs()), len(tail.Runs()))
	}
}

// --- Shaped scripts -------------------------------------------------------

func TestSplitAfterLineArabicOriginalText(t *testing.T) {
	arabic := "العربية"
	text := strings.Repeat(arabic+" ", 16)
	p := NewParagraph(text, font.Helvetica, 12)
	total := p.MeasureLines(100)
	if total < 3 {
		t.Skipf("Arabic setup needs ≥3 lines, got %d", total)
	}

	head, tail := p.SplitAfterLine(1, 100)
	if head == nil || tail == nil {
		t.Fatal("Arabic split should return both halves")
	}

	// Re-lay the tail and verify OriginalText round-trips.
	for _, line := range tail.Layout(100) {
		for _, w := range line.Words {
			if w.OriginalText != "" && strings.Contains(w.OriginalText, arabic) {
				return // pass
			}
		}
	}
	t.Error("Arabic OriginalText not recovered after SplitAfterLine")
}

// --- CJK round-trip --------------------------------------------------------

func TestSplitAfterLineCJKNoSpuriousSpaces(t *testing.T) {
	// CJK round-trip exercises the SpaceAfter==0 join logic in
	// cloneWithWords from the SplitAfterLine path.
	text := strings.Repeat("中文文本", 20)
	p := NewParagraph(text, font.Helvetica, 12)
	total := p.MeasureLines(60)
	if total < 3 {
		t.Skipf("CJK setup needs ≥3 lines, got %d", total)
	}

	head, tail := p.SplitAfterLine(1, 60)
	if head == nil || tail == nil {
		t.Fatal("CJK split should return both halves")
	}

	// Neither half's run text should contain ASCII spaces.
	for label, halfRuns := range map[string][]TextRun{"head": head.Runs(), "tail": tail.Runs()} {
		for i, run := range halfRuns {
			if strings.Contains(run.Text, " ") {
				t.Errorf("%s run %d contains spurious space: %q", label, i, run.Text)
			}
		}
	}
}

// --- Link spans ------------------------------------------------------------

func TestSplitAfterLineLinkSpansBoundary(t *testing.T) {
	// A link that spans many words should remain functional on both
	// halves of a split — the LinkURI must round-trip.
	url := "https://example.com/long"
	link := NewRun(strings.Repeat("link-text ", 10), font.Helvetica, 12)
	link.LinkURI = url
	p := NewStyledParagraph(link)
	total := p.MeasureLines(80)
	if total < 3 {
		t.Skipf("link setup needs ≥3 lines, got %d", total)
	}

	head, tail := p.SplitAfterLine(1, 80)
	if head == nil || tail == nil {
		t.Fatal("link split should return both halves")
	}

	hasLinkInHead, hasLinkInTail := false, false
	for _, r := range head.Runs() {
		if r.LinkURI == url {
			hasLinkInHead = true
		}
	}
	for _, r := range tail.Runs() {
		if r.LinkURI == url {
			hasLinkInTail = true
		}
	}
	if !hasLinkInHead {
		t.Error("LinkURI lost on head")
	}
	if !hasLinkInTail {
		t.Error("LinkURI lost on tail")
	}
}

// --- Hyphenation interaction ----------------------------------------------
//
// SplitAfterLine uses Layout(), which CAN hyphenate. The CJK-driven
// SpaceAfter==0 join in cloneWithWords would naively produce
// "linguis-tic" when both halves of a hyphenated pair land in the same
// half. Today this passes silently; if it ever breaks visibly, the fix
// is a Word.HyphenatedBoundary flag (tracked as follow-up). The test
// below documents the current behavior so a regression is visible.

func TestSplitAfterLineHyphenationInternalToHead(t *testing.T) {
	// Long alpha word that hyphenates. Force a multi-line wrap with
	// hyphens enabled, then split AFTER the hyphenated pair so both
	// halves land in head. Today the head's run text contains a
	// recombined "-" artifact (e.g. "linguis-tic"). When this test
	// changes shape, the recombination logic in cloneWithWords will
	// have been improved.
	// Multi-line paragraph with hyphens enabled. "linguistic" fits within
	// lineWidth (70pt) but does not fit after "hello ", so it hyphenates
	// across lines to "linguis-" and "tic". When split after line 2, both
	// halves of the hyphenated pair land in head.
	//
	// Pre-fix, cloneWithWords left a literal hyphen artifact in the head's
	// run text ("linguis-tic"). With Word.HyphenatedBoundary tracking, the
	// original word "linguistic" is recovered cleanly without "-" artifact.
	source := "hello linguistic world of typography"
	p := NewParagraph(source, font.Helvetica, 12).SetHyphens("auto")
	total := p.MeasureLines(70)
	if total < 3 {
		t.Skipf("setup needs ≥3 lines, got %d", total)
	}

	// Setup precondition: at least one line must end with a hyphen.
	hyphenated := false
	for _, line := range p.Layout(70) {
		if len(line.Words) == 0 {
			continue
		}
		last := line.Words[len(line.Words)-1]
		if strings.HasSuffix(last.Text, "-") {
			hyphenated = true
			break
		}
	}
	if !hyphenated {
		t.Skip("hyphenation did not fire on this input — pattern dictionary may have changed")
	}

	// Split after the hyphenated pair (line 2) so both halves land in head.
	head, tail := p.SplitAfterLine(2, 70)
	if head == nil {
		t.Fatal("head should be non-nil")
	}
	if tail == nil {
		t.Fatal("tail should be non-nil")
	}

	// Assert the hyphen artifact is gone from the head's run text:
	// "linguistic" should appear whole, not "linguis-tic" or "linguis-".
	for i, r := range head.Runs() {
		if strings.Contains(r.Text, "linguis-") {
			t.Errorf("head run %d contains un-recombined hyphen artifact: %q", i, r.Text)
		}
		if !strings.Contains(r.Text, "linguistic") {
			t.Errorf("head run %d missing recombined original word: %q", i, r.Text)
		}
	}

	// Head re-laid at the same width should produce the same line count (2).
	if got := head.MeasureLines(70); got != 2 {
		t.Errorf("head re-lay line count drifted: was 2, now %d", got)
	}

	// Split across the hyphenation boundary (line 1): head ends with "linguis-",
	// tail starts with "tic". Neither recombines.
	head1, tail1 := p.SplitAfterLine(1, 70)
	if head1 == nil || tail1 == nil {
		t.Fatal("head1 and tail1 must be non-nil")
	}
	if !strings.Contains(head1.Runs()[0].Text, "linguis-") {
		t.Errorf("cross-split head should retain hyphenated prefix, got %q", head1.Runs()[0].Text)
	}
	if !strings.HasPrefix(strings.TrimSpace(tail1.Runs()[0].Text), "tic") {
		t.Errorf("cross-split tail should start with hyphenated suffix, got %q", tail1.Runs()[0].Text)
	}
}

// --- Inline element at line-head in tail ---------------------------------

// TestSplitAfterLineInlineAtLineHeadInTail covers the exact bug the
// new InlineBlock-aware blank-line guard fixes: an inline element
// landing at line.Words[0] of any line beyond the first gets
// LineBreak=true injected by flattenLineWords, which would otherwise
// match the blank-line guard in cloneWithWords and silently drop the
// inline. This test forces that geometry and asserts the inline
// survives in the tail.
func TestSplitAfterLineInlineAtLineHeadInTail(t *testing.T) {
	el := &fixedElement{width: 60, height: 16}
	// Long enough prefix to push the inline onto a non-first line.
	// Tight width forces wrap such that the inline lands at j=0 of
	// some line.
	prefix := strings.Repeat("alpha beta gamma delta epsilon zeta ", 6)
	p := NewStyledParagraph(
		NewRun(prefix, font.Helvetica, 12),
		RunInline(el),
		NewRun(" tail tail", font.Helvetica, 12),
	)

	// Verify setup: inline lands at line.Words[0] of some non-first
	// line. Otherwise the test isn't exercising the guard.
	lines := p.Layout(70)
	inlineAtLineHead := false
	for i, line := range lines {
		if i == 0 || len(line.Words) == 0 {
			continue
		}
		if line.Words[0].InlineBlock == el {
			inlineAtLineHead = true
			break
		}
	}
	if !inlineAtLineHead {
		t.Skipf("inline didn't land at line.Words[0] of a non-first line; setup needs tuning")
	}

	// Split such that the inline lands in the tail.
	head, tail := p.SplitAfterLine(1, 70)
	if head == nil || tail == nil {
		t.Fatal("split should return both halves")
	}
	tailHas := false
	for _, run := range tail.Runs() {
		if run.InlineElement == el {
			tailHas = true
			break
		}
	}
	if !tailHas {
		t.Error("inline at line-head in tail was lost — InlineBlock guard regressed")
	}
}

// --- FirstLineIndent ------------------------------------------------------

// TestSplitAfterLineDropsFirstIndent verifies the doc-comment claim
// that FirstLineIndent is dropped from both halves. cloneWithWords
// constructs a fresh Paragraph literal that omits firstIndent, so the
// drop happens transitively. Without this test, a future change that
// propagates firstIndent to clones would silently break the contract.
func TestSplitAfterLineDropsFirstIndent(t *testing.T) {
	// Geometry chosen so first-line indent forces an EXTRA line: at
	// width 80 with indent=40, the first line has 40pt usable; without
	// indent, it has 80pt. With small Helvetica 12pt content of ~10
	// short words, the indented form wraps to one more line than the
	// unindented form.
	text := "alpha beta gamma delta epsilon zeta eta theta iota kappa"
	pNoIndent := NewParagraph(text, font.Helvetica, 12)
	pIndent := NewParagraph(text, font.Helvetica, 12).SetFirstLineIndent(40)

	noIndentLines := pNoIndent.MeasureLines(80)
	indentLines := pIndent.MeasureLines(80)
	if indentLines <= noIndentLines {
		t.Skipf("setup: indent failed to force extra wrap (indent=%d, no-indent=%d)",
			indentLines, noIndentLines)
	}

	// SplitAfterLine the indented paragraph; head should re-lay
	// WITHOUT the indent (i.e. line count matches noIndent at the
	// same width).
	head, tail := pIndent.SplitAfterLine(indentLines, 80)
	if head == nil {
		t.Fatal("head should be non-nil")
	}
	if tail != nil {
		t.Fatalf("tail should be nil at exact-boundary, got %d lines", tail.MeasureLines(80))
	}

	headRelay := head.MeasureLines(80)
	if headRelay != noIndentLines {
		t.Errorf("head re-lay produced %d lines, expected %d (no-indent equivalent); "+
			"firstIndent was not dropped on head", headRelay, noIndentLines)
	}
}

// --- Empty paragraph with spacing ----------------------------------------

// TestSplitAfterLineEmptyWithSpacing verifies SplitAfterLine on an
// empty paragraph that carries SpaceBefore. cloneWithWords produces
// a runs=nil paragraph; SplitAfterLine then propagates spaceBefore to
// the head explicitly. The head's spacing must round-trip.
func TestSplitAfterLineEmptyWithSpacing(t *testing.T) {
	p := NewParagraph("", font.Helvetica, 12).
		SetSpaceBefore(10).
		SetSpaceAfter(15)

	head, tail := p.SplitAfterLine(1, 500)
	if tail != nil {
		t.Errorf("empty paragraph tail should be nil, got %d lines", tail.MeasureLines(500))
	}
	if head == nil {
		t.Skip("empty head returned as nil (allowed alternative)")
	}
	// No-overflow case: head IS the entire paragraph, so it owns
	// BOTH spacings (there is no tail to receive spaceAfter).
	if got := head.GetSpaceBefore(); got != 10 {
		t.Errorf("head spaceBefore = %v, want 10", got)
	}
	if got := head.GetSpaceAfter(); got != 15 {
		t.Errorf("head spaceAfter = %v, want 15 (no tail to inherit it)", got)
	}
}

// --- Fallback / cross-script ---------------------------------------------

// TestSplitAfterLineFallbackCrossScript verifies that a paragraph
// built via NewParagraphFallback (cross-script font dispatch) splits
// cleanly. Each half must preserve the script-dispatched fonts on
// the appropriate runs.
func TestSplitAfterLineFallbackCrossScript(t *testing.T) {
	latin := font.NewEmbeddedFont(newFakeFace('H', 'e', 'l', 'o', 'L', 'a', 'r', 'g', 'm', 'i', 'n', ' '))
	hebrew := font.NewEmbeddedFont(newFakeFace('ש', 'ל', 'ו', 'ם', ' '))
	fb := font.NewFallback(latin, hebrew)

	// Long enough to wrap, and Hebrew + Latin both present.
	text := strings.Repeat("Hello שלום ", 8)
	p := NewParagraphFallback(text, fb, 12)
	total := p.MeasureLines(80)
	if total < 3 {
		t.Skipf("fallback setup needs ≥3 lines, got %d", total)
	}

	head, tail := p.SplitAfterLine(1, 80)
	if head == nil || tail == nil {
		t.Fatal("fallback split should return both halves")
	}

	// Each half should still carry both Latin and Hebrew embedded
	// fonts on its runs (script segmentation should still produce
	// distinct runs after the split).
	hasLatin, hasHebrew := false, false
	for _, r := range tail.Runs() {
		if r.Embedded == latin {
			hasLatin = true
		}
		if r.Embedded == hebrew {
			hasHebrew = true
		}
	}
	if !hasLatin {
		t.Error("tail lost Latin font dispatch")
	}
	if !hasHebrew {
		t.Error("tail lost Hebrew font dispatch")
	}
}

// --- Empty / degenerate ---------------------------------------------------

func TestSplitAfterLineEmpty(t *testing.T) {
	// Empty paragraph: no lines, both halves nil-ish.
	p := NewParagraph("", font.Helvetica, 12)
	head, tail := p.SplitAfterLine(1, 500)
	// total = 0, n=1 >= 0 so head returns full paragraph (empty), tail nil.
	if tail != nil {
		t.Errorf("empty paragraph tail should be nil, got %v lines",
			tail.MeasureLines(500))
	}
	if head == nil {
		// Allowed alternative: empty head returned as nil. Either is fine.
		return
	}
	if got := head.MeasureLines(500); got != 0 {
		t.Errorf("empty head lines = %d, want 0", got)
	}
}
