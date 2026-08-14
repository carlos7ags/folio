// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// englishText is roughly 100 words / 700 bytes, repeated to give the
// wrap loop enough lines to work through per call.
var englishText = strings.Repeat(
	"Lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod "+
		"tempor incididunt ut labore et dolore magna aliqua ut enim ad minim "+
		"veniam quis nostrud exercitation ullamco laboris nisi ut aliquip ex "+
		"ea commodo consequat ",
	4,
)

// arabicText is ~30 repetitions of a short Arabic phrase, routed through
// ShapeArabic during word measurement.
var arabicText = strings.Repeat("السلام عليكم ", 30)

// BenchmarkParagraphWrapPlain covers Paragraph.Layout's greedy word-wrap
// loop with plain left-aligned English text (no hyphenation attempts).
func BenchmarkParagraphWrapPlain(b *testing.B) {
	p := NewParagraph(englishText, font.Helvetica, 12)
	b.ResetTimer()
	for range b.N {
		lines := p.Layout(200)
		if len(lines) == 0 {
			b.Fatal("Layout returned no lines")
		}
	}
}

// BenchmarkParagraphWrapJustifyHyphens covers justified wrapping at a
// narrow width, which drives hyphenateWord on most line breaks.
func BenchmarkParagraphWrapJustifyHyphens(b *testing.B) {
	p := NewParagraph(englishText, font.Helvetica, 12).
		SetAlign(AlignJustify).
		SetHyphens("auto")
	b.ResetTimer()
	for range b.N {
		lines := p.Layout(150)
		if len(lines) == 0 {
			b.Fatal("Layout returned no lines")
		}
	}
}

// BenchmarkParagraphWrapArabic covers ShapeArabic plus Standard.MeasureString
// per word, exercised through the normal paragraph wrap path.
func BenchmarkParagraphWrapArabic(b *testing.B) {
	p := NewParagraph(arabicText, font.Helvetica, 12)
	b.ResetTimer()
	for range b.N {
		lines := p.Layout(200)
		if len(lines) == 0 {
			b.Fatal("Layout returned no lines")
		}
	}
}

// BenchmarkTableMultiPagePlan covers Table.PlanLayout for a table that
// spans many pages: ~200 rows x 4 columns of auto-sized text, walked
// through the full overflow chain the way the renderer consumes
// LayoutPlan (PlanLayout, then re-PlanLayout the Overflow table on the
// next page, etc). Before column-width memoization, each continuation
// table re-measured every remaining row's cell text via resolveColWidths,
// giving O(rows^2) text measurement overall for a table this size.
func BenchmarkTableMultiPagePlan(b *testing.B) {
	buildTable := func() *Table {
		t := NewTable()
		t.SetAutoColumnWidths()
		for range 200 {
			row := t.AddRow()
			row.AddCell("Widget Name Example", font.Helvetica, 10)
			row.AddCell("Quantity 42", font.Helvetica, 10)
			row.AddCell("Unit Price $19.99", font.Helvetica, 10)
			row.AddCell("Extended Total $839.58", font.Helvetica, 10)
		}
		return t
	}

	b.ResetTimer()
	for range b.N {
		tbl := buildTable()
		area := LayoutArea{Width: 500, Height: 60}
		pages := 0
		for {
			plan := tbl.PlanLayout(area)
			pages++
			if plan.Status != LayoutPartial || plan.Overflow == nil {
				break
			}
			var ok bool
			tbl, ok = plan.Overflow.(*Table)
			if !ok {
				b.Fatalf("overflow is %T, want *Table", plan.Overflow)
			}
			if pages > 500 {
				b.Fatal("too many pages — overflow chain not terminating")
			}
		}
	}
}

// BenchmarkBreakLongWordsUnshaped drives the prefix re-measurement loop in
// breakLongWords on a single long unbreakable ASCII token.
func BenchmarkBreakLongWordsUnshaped(b *testing.B) {
	text := strings.Repeat("abcdefghij", 40)
	w := Word{
		Text:     text,
		Font:     font.Helvetica,
		FontSize: 12,
		Width:    font.Helvetica.MeasureString(text, 12),
	}
	b.ResetTimer()
	for range b.N {
		words := breakLongWords([]Word{w}, 100)
		if len(words) == 0 {
			b.Fatal("breakLongWords returned no words")
		}
	}
}

// BenchmarkBreakLongWordsIndic drives breakLongWords on a shaped Devanagari
// word, where each chunk must carry re-shaped GIDs (see
// TestBreakLongWordsPreservesIndicOriginalTextAndGIDs).
func BenchmarkBreakLongWordsIndic(b *testing.B) {
	face := newMockDevaFace()
	face.substitutions = &font.GSUBSubstitutions{
		Single: map[font.GSUBFeature]map[uint16]uint16{},
	}
	ef := font.NewEmbeddedFont(face)

	const fontSize = 12
	source := strings.Repeat("क", 30)

	gids, ok := ShapeIndicWithEmbedded(source, ef, ScriptDevanagari)
	if !ok {
		b.Fatal("ShapeIndicWithEmbedded returned (_, false) for the benchmark source")
	}
	wholeWidth := ef.MeasureGIDs(gids, fontSize)
	if wholeWidth == 0 {
		b.Fatal("mock face produced zero-width word; benchmark cannot drive breakLongWords")
	}

	w := Word{
		Text:         source,
		OriginalText: source,
		GIDs:         gids,
		Width:        wholeWidth,
		Embedded:     ef,
		FontSize:     fontSize,
	}
	maxWidth := wholeWidth / 4

	b.ResetTimer()
	for range b.N {
		chunks := breakLongWords([]Word{w}, maxWidth)
		if len(chunks) == 0 {
			b.Fatal("breakLongWords returned no chunks")
		}
	}
}
