// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"slices"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// hebrewWords returns n distinct two-letter Hebrew words so word order is
// observable after reordering.
func hebrewWords(n int) []string {
	out := make([]string, n)
	for i := range n {
		out[i] = string([]rune{0x05D0 + rune(i%22), 0x05D0 + rune((i/22)%22)})
	}
	return out
}

// paragraphRunText concatenates a paragraph's run texts with
// whitespace collapsed, so it can be compared against source text.
func paragraphRunText(p *Paragraph) string {
	var sb strings.Builder
	for _, r := range p.runs {
		sb.WriteString(r.Text)
		sb.WriteByte(' ')
	}
	return strings.Join(strings.Fields(sb.String()), " ")
}

// TestParagraphRTLOverflowKeepsLogicalOrder: the overflow paragraph of a
// page-split RTL paragraph must be built from logical-order words with
// logical text. Building it from the visual (reordered, glyph-reversed)
// words reversed the text and word order a second time on the next page.
func TestParagraphRTLOverflowKeepsLogicalOrder(t *testing.T) {
	words := hebrewWords(30)
	text := strings.Join(words, " ")
	const width = 120.0

	p := NewParagraph(text, font.Helvetica, 12)
	p.SetDirection(DirectionRTL)
	full := p.Layout(width)
	if len(full) < 3 {
		t.Fatalf("need a multi-line paragraph, got %d lines", len(full))
	}

	plan := p.PlanLayout(LayoutArea{Width: width, Height: 15})
	if plan.Status != LayoutPartial {
		t.Fatalf("status = %v, want LayoutPartial", plan.Status)
	}
	overflow := plan.Overflow.(*Paragraph)

	// Logical text of the overflow is a suffix of the source, in order.
	got := paragraphRunText(overflow)
	if !strings.HasSuffix(text, got) || got == "" {
		t.Fatalf("overflow text %q is not a logical suffix of %q", got, text)
	}

	// Re-laid continuation lines paint exactly like the original layout's
	// lines from the split point on.
	cont := overflow.Layout(width)
	for i := range cont {
		want := wordTexts(full[i+1].Words)
		if g := wordTexts(cont[i].Words); !slices.Equal(g, want) {
			t.Errorf("continuation line %d: got %q, want %q", i, g, want)
		}
	}

	// SplitAfterLine (used by list items) must round-trip the same way.
	_, tail := p.SplitAfterLine(1, width)
	if got := paragraphRunText(tail); !strings.HasSuffix(text, got) {
		t.Errorf("SplitAfterLine tail %q is not a logical suffix", got)
	}
	tl := tail.Layout(width)
	if g, want := wordTexts(tl[0].Words), wordTexts(full[1].Words); !slices.Equal(g, want) {
		t.Errorf("SplitAfterLine tail first line: got %q, want %q", g, want)
	}
}

// TestListRTLItemIsBidiReordered: list items run through the same UAX #9
// reordering as paragraphs. "abc שלום def" in an RTL list paints def at
// the left and abc at the right.
func TestListRTLItemIsBidiReordered(t *testing.T) {
	l := NewList(font.Helvetica, 12)
	l.SetDirection(DirectionRTL)
	l.AddItem("abc שלום def")
	plan := l.PlanLayout(LayoutArea{Width: 300, Height: 200})
	var abcX, defX float64 = -1, -1
	for _, line := range drawnLines(t, plan) {
		for _, d := range line {
			switch d.text {
			case "abc":
				abcX = d.x
			case "def":
				defX = d.x
			}
		}
	}
	if abcX < 0 || defX < 0 {
		t.Fatalf("abc/def not drawn (abc=%v def=%v)", abcX, defX)
	}
	if defX >= abcX {
		t.Errorf("RTL list item: def x=%v should be left of abc x=%v", defX, abcX)
	}
}

// TestListRTLItemSplitKeepsLogicalOrder: a split RTL list item continues
// on the next page with its logical text, not the visually reversed one.
func TestListRTLItemSplitKeepsLogicalOrder(t *testing.T) {
	text := strings.Join(hebrewWords(40), " ")
	l := NewList(font.Helvetica, 12)
	l.SetDirection(DirectionRTL)
	l.AddItem(text)
	plan := l.PlanLayout(LayoutArea{Width: 120, Height: 30})
	if plan.Status != LayoutPartial {
		t.Fatalf("status = %v, want LayoutPartial", plan.Status)
	}
	cont := plan.Overflow.(*List)
	if len(cont.items) == 0 || cont.items[0].contPara == nil {
		t.Fatal("expected a split continuation item")
	}
	got := paragraphRunText(cont.items[0].contPara)
	if got == "" || !strings.HasSuffix(text, got) {
		t.Errorf("continuation text %q is not a logical suffix of the item", got)
	}
}

// TestBidiMixedLevelWordSegments: a word whose characters sit at several
// bidi levels reverses (and mirrors) only its RTL segments.
func TestBidiMixedLevelWordSegments(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"ב-2026", "2026-ב"}, // ב-2026 → 2026-ב
		{"ص.ب123", "123ب.ص"}, // ص.ب123 → 123ب.ص
		{"2026م", "م2026"},   // 2026م → م2026
		{"(שלום)", "(םולש)"}, // uniform RTL unchanged rule
	}
	for _, tc := range tests {
		visual, _ := resolveLineBidi(makeWords(tc.in), DirectionRTL)
		if len(visual) != 1 || visual[0].Text != tc.want {
			t.Errorf("%q: got %q, want %q", tc.in, wordTexts(visual), tc.want)
			continue
		}
		if visual[0].OriginalText != tc.in {
			t.Errorf("%q: OriginalText = %q, want logical text", tc.in, visual[0].OriginalText)
		}
	}
}

// TestBidiTrailingPunctuationInLTRParagraph: in an LTR paragraph a comma
// after a Hebrew word resolves LTR and stays on the word's right.
func TestBidiTrailingPunctuationInLTRParagraph(t *testing.T) {
	visual, _ := resolveLineBidi(makeWords("abc", "שלום,", "def"), DirectionLTR)
	want := []string{"abc", "םולש,", "def"}
	if got := wordTexts(visual); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestBidiNumberInsideRTLRunInLTRParagraph: in an LTR paragraph, a number
// between two Hebrew words is embedded in the RTL run (level 2), so the
// three words reverse as one group.
func TestBidiNumberInsideRTLRunInLTRParagraph(t *testing.T) {
	shalom, olam := "שלום", "עולם"
	visual, dir := resolveLineBidi(makeWords("abc", shalom, "123", olam, "def"), DirectionLTR)
	if dir != DirectionLTR {
		t.Errorf("dir = %v, want LTR", dir)
	}
	want := []string{"abc", reverseTestRunes(olam), "123", reverseTestRunes(shalom), "def"}
	if got := wordTexts(visual); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestParagraphRTLSplitTokenGaps: "ב-2026" is pre-split into "ב-" and
// "2026" with no gap between them; after reordering the zero gap must
// still sit between those two pieces, not between "-ב" and the next word.
func TestParagraphRTLSplitTokenGaps(t *testing.T) {
	p := NewParagraph("שלום ב-2026 עולם", font.Helvetica, 12)
	p.SetDirection(DirectionRTL)
	lines := p.Layout(500)
	words := lines[0].Words
	want := []string{"םלוע", "2026", "-ב", "םולש"}
	if got := wordTexts(words); !slices.Equal(got, want) {
		t.Fatalf("visual words: got %q, want %q", got, want)
	}
	if words[1].SpaceAfter != 0 {
		t.Errorf("gap between 2026 and -ב = %v, want 0", words[1].SpaceAfter)
	}
	if words[0].SpaceAfter <= 0 || words[2].SpaceAfter <= 0 {
		t.Errorf("word gaps lost: %v / %v", words[0].SpaceAfter, words[2].SpaceAfter)
	}
}

// TestBidiExplicitRTLBaseIsEnforced: an explicit RTL base direction is a
// paragraph level, not just a fallback — "abc שלום def" paints right to
// left even though its first strong character is Latin.
func TestBidiExplicitRTLBaseIsEnforced(t *testing.T) {
	shalom := "שלום"
	visual, dir := resolveLineBidi(makeWords("abc", shalom, "def"), DirectionRTL)
	if dir != DirectionRTL {
		t.Errorf("dir = %v, want RTL", dir)
	}
	want := []string{"def", reverseTestRunes(shalom), "abc"}
	if got := wordTexts(visual); !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	// Auto direction resolves from the first strong character even when
	// the line starts with digits.
	_, dir = resolveLineBidi(makeWords("123", shalom), DirectionAuto)
	if dir != DirectionRTL {
		t.Errorf("auto dir for %q = %v, want RTL", "123 "+shalom, dir)
	}
}

// TestBidiReversedWordRecordsLogicalText: a word reversed for painting
// carries its logical text in OriginalText (for /ActualText and
// extraction) when no shaper set it, and leaves untouched words alone.
func TestBidiReversedWordRecordsLogicalText(t *testing.T) {
	shalom := "שלום"
	visual, _ := resolveLineBidi(makeWords(shalom, "abc"), DirectionRTL)
	for _, w := range visual {
		switch w.Text {
		case reverseTestRunes(shalom):
			if w.OriginalText != shalom {
				t.Errorf("reversed word OriginalText = %q, want %q", w.OriginalText, shalom)
			}
		case "abc":
			if w.OriginalText != "" {
				t.Errorf("unreversed word OriginalText = %q, want empty", w.OriginalText)
			}
		}
	}
}

// TestKashidaJustificationOnVisuallyReversedWords: bidi hands the
// justification pass visually reversed Arabic words; kashida must still
// be inserted, at logical sites, and the word kept in painting order.
func TestKashidaJustificationOnVisuallyReversedWords(t *testing.T) {
	ef := newMockArabicEmbedded()
	logical := []Word{
		{Text: ShapeArabic("سلام"), OriginalText: "سلام", Embedded: ef, FontSize: 12, SpaceAfter: ef.MeasureString(" ", 12)},
		{Text: ShapeArabic("سلام"), OriginalText: "سلام", Embedded: ef, FontSize: 12, SpaceAfter: ef.MeasureString(" ", 12)},
	}
	for i := range logical {
		logical[i].Width = ef.MeasureString(logical[i].Text, 12)
	}
	words, _ := resolveLineBidi(logical, DirectionRTL)
	if !words[0].visualRTL {
		t.Fatal("expected the Arabic word to be marked visually reversed")
	}
	slack := 500 - lineWidth(words)
	if consumed := applyKashidaJustification(words, slack); consumed <= 0 {
		t.Fatalf("no kashida inserted into reversed Arabic words (consumed=%v)", consumed)
	}
	for i, w := range words {
		if !strings.ContainsRune(w.Text, kashidaTatweel) {
			t.Errorf("word %d: no tatweel in %q", i, w.Text)
		}
		if want := reverseVisualRTL(w.logicalText); w.Text != want {
			t.Errorf("word %d: painted text %q is not the reversed logical text %q", i, w.Text, want)
		}
		if w.logicalText == ShapeArabic("سلام") {
			t.Errorf("word %d: logical text was not updated with the kashida", i)
		}
	}
}

// TestBidiInlineBlockBetweenRTLAndNumber pins UAX #9 N1 for an inline block
// (neutral): European/Arabic numbers count as R, so an image between Hebrew
// text and a number inside an RTL run in an LTR paragraph joins the RTL run.
// Comparing raw levels (1 vs 2) dropped it to the paragraph level and split
// the run. An EN that W7 turned into L (after Latin text) still counts as L.
func TestBidiInlineBlockBetweenRTLAndNumber(t *testing.T) {
	const (
		shalom = "שלום" // שלום
		olam   = "עולם" // עולם
		img    = "[img]"
	)
	label := func(ws []Word) string {
		parts := make([]string, len(ws))
		for i, w := range ws {
			if w.InlineBlock != nil {
				parts[i] = img
			} else {
				parts[i] = w.Text
			}
		}
		return strings.Join(parts, " ")
	}
	inline := Word{Text: "", Width: 12, InlineBlock: &Div{}}
	tests := []struct {
		name  string
		words []Word
		base  Direction
		want  string
	}{
		{
			"RTL text, image, number, RTL text in LTR paragraph",
			[]Word{{Text: "abc"}, {Text: shalom}, inline, {Text: "123"}, {Text: olam}},
			DirectionLTR,
			"abc " + reverseTestRunes(olam) + " 123 " + img + " " + reverseTestRunes(shalom),
		},
		{
			"number, image, RTL text in LTR paragraph after Latin (W7: EN is L)",
			[]Word{{Text: "abc"}, {Text: "123"}, inline, {Text: shalom}},
			DirectionLTR,
			"abc 123 " + img + " " + reverseTestRunes(shalom),
		},
		{
			"image between numbers inside RTL run",
			[]Word{{Text: shalom}, {Text: "123"}, inline, {Text: "456"}, {Text: olam}},
			DirectionLTR,
			reverseTestRunes(olam) + " 456 " + img + " 123 " + reverseTestRunes(shalom),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			visual, _ := resolveLineBidi(tc.words, tc.base)
			if got := label(visual); got != tc.want {
				t.Errorf("visual = %q, want %q", got, tc.want)
			}
		})
	}
}
