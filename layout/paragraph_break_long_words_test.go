// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/carlos7ags/folio/font"
)

// TestBreakLongWordsPreservesIndicOriginalTextAndGIDs is the
// regression test for #255. A Devanagari word that exceeds the line
// width — say, 30 "ka" syllables glued together — must split into
// chunks where every chunk carries:
//
//   - a per-chunk OriginalText slice (pre-shape Unicode) for the
//     renderer's /ActualText marked-content wrapper, so copy/paste
//     recovers the original Devanagari instead of post-shape glyph
//     codepoints;
//   - a re-shaped GIDs stream, so the Identity-H emission path the
//     draw layer relies on has the right glyph indices at chunk
//     boundaries.
//
// Pre-fix breakLongWords copied most Word fields when constructing
// chunks but dropped both OriginalText and GIDs, so chunk N>0 lost
// its accessibility / round-trip text and its glyph stream. The
// rendered PDF showed correct first chunk and silently-broken tails.
func TestBreakLongWordsPreservesIndicOriginalTextAndGIDs(t *testing.T) {
	face := newMockDevaFace()
	face.substitutions = &font.GSUBSubstitutions{
		Single: map[font.GSUBFeature]map[uint16]uint16{},
	}
	ef := font.NewEmbeddedFont(face)

	// 30 ka syllables, no whitespace — a single word the layout engine
	// would route into breakLongWords once the line width is narrower
	// than the whole-word measure.
	const fontSize = 12
	source := strings.Repeat("क", 30)

	// Replicate what shapeAndMeasureWord does to produce a Word as it
	// would appear immediately before breakLongWords runs.
	gids, ok := ShapeIndicWithEmbedded(source, ef, ScriptDevanagari)
	if !ok {
		t.Fatal("ShapeIndicWithEmbedded returned (_, false) for the test source")
	}
	wholeWidth := ef.MeasureGIDs(gids, fontSize)
	if wholeWidth == 0 {
		t.Fatal("mock face produced zero-width word; test cannot drive breakLongWords")
	}

	w := Word{
		Text:         source,
		OriginalText: source,
		GIDs:         gids,
		Width:        wholeWidth,
		Embedded:     ef,
		FontSize:     fontSize,
	}

	// Pick maxWidth = 1/4 of the whole word so the function is forced
	// to produce multiple chunks.
	maxWidth := wholeWidth / 4
	chunks := breakLongWords([]Word{w}, maxWidth)

	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d (whole=%v, max=%v)",
			len(chunks), wholeWidth, maxWidth)
	}

	var rejoined strings.Builder
	for i, c := range chunks {
		if c.OriginalText == "" {
			t.Errorf("chunk %d: OriginalText empty — ActualText recovery would emit shaped codepoints", i)
		}
		if len(c.GIDs) == 0 {
			t.Errorf("chunk %d: GIDs empty — Devanagari Identity-H emission would fall back to .notdef glyphs", i)
		}
		if c.Width > maxWidth+0.001 {
			t.Errorf("chunk %d: width %v exceeds maxWidth %v", i, c.Width, maxWidth)
		}
		rejoined.WriteString(c.OriginalText)
	}
	if rejoined.String() != source {
		t.Errorf("chunk OriginalTexts do not rejoin to the source word.\n  source: %q\n  rejoined: %q",
			source, rejoined.String())
	}
}

// TestBreakLongWordsPreservesArabicOriginalText covers the parallel
// Arabic path. ShapeArabic substitutes Presentation Forms-B codepoints
// in place — the post-shape Text differs from the pre-shape original.
// Pre-fix breakLongWords dropped OriginalText on chunks N>0, so the
// /ActualText wrapper for tail chunks emitted the FE-range glyph-form
// codepoints instead of the original Arabic; copy/paste recovered
// disconnected isolated forms.
//
// The test uses font.Helvetica as a standard-font carrier so the
// Arabic-shaped Text is measured via MeasureString. Helvetica has no
// Arabic glyphs in WinAnsi, but the measurement path treats every
// rune as having a fallback width — enough to drive breakLongWords
// deterministically.
func TestBreakLongWordsPreservesArabicOriginalText(t *testing.T) {
	const fontSize = 12
	// 30 lam-alef pairs (U+0644 U+0627) — no whitespace, so the
	// resulting Word is one long Arabic word post-shape.
	source := strings.Repeat("لا", 30)
	shaped := ShapeArabic(source)
	if shaped == source {
		t.Skip("ShapeArabic did not substitute any Presentation Forms; test cannot demonstrate OriginalText preservation")
	}
	wholeWidth := font.Helvetica.MeasureString(shaped, fontSize)
	if wholeWidth == 0 {
		t.Fatal("Helvetica measured zero width; test cannot drive breakLongWords")
	}

	w := Word{
		Text:         shaped,
		OriginalText: source,
		Width:        wholeWidth,
		Font:         font.Helvetica,
		FontSize:     fontSize,
	}

	maxWidth := wholeWidth / 4
	chunks := breakLongWords([]Word{w}, maxWidth)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d (whole=%v, max=%v)",
			len(chunks), wholeWidth, maxWidth)
	}

	var rejoined strings.Builder
	for i, c := range chunks {
		if c.OriginalText == "" {
			t.Errorf("chunk %d: OriginalText empty — ActualText recovery would emit Presentation Forms-B", i)
		}
		// Each chunk's OriginalText must be a non-empty slice of
		// the source codepoints. Validate count matches Text by
		// rune count if no ligature collapsed the chunk.
		if utf8.RuneCountInString(c.OriginalText) == 0 {
			t.Errorf("chunk %d: OriginalText has zero runes", i)
		}
		rejoined.WriteString(c.OriginalText)
	}
	if rejoined.String() != source {
		t.Errorf("chunk OriginalTexts do not rejoin to the source word.\n  source: %q\n  rejoined: %q",
			source, rejoined.String())
	}
}

// TestBreakLongWordsUnshapedPathUnchanged guards against the
// re-shaping path firing for words that were never shaped to begin
// with. A plain Latin word with no OriginalText should still chunk
// by Text runes and emit chunks whose Text is a substring of the
// source — same behaviour as the function had before #255.
func TestBreakLongWordsUnshapedPathUnchanged(t *testing.T) {
	const fontSize = 12
	source := strings.Repeat("a", 30)
	wholeWidth := font.Helvetica.MeasureString(source, fontSize)
	if wholeWidth == 0 {
		t.Fatal("Helvetica measured zero width; test cannot drive breakLongWords")
	}
	w := Word{
		Text:     source,
		Width:    wholeWidth,
		Font:     font.Helvetica,
		FontSize: fontSize,
	}
	chunks := breakLongWords([]Word{w}, wholeWidth/4)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	var rejoined strings.Builder
	for i, c := range chunks {
		if c.OriginalText != "" {
			t.Errorf("chunk %d: unshaped word should not gain OriginalText, got %q", i, c.OriginalText)
		}
		if c.GIDs != nil {
			t.Errorf("chunk %d: unshaped word should not gain GIDs, got %v", i, c.GIDs)
		}
		rejoined.WriteString(c.Text)
	}
	if rejoined.String() != source {
		t.Errorf("chunk Texts do not rejoin to the source.\n  source: %q\n  rejoined: %q",
			source, rejoined.String())
	}
}

// TestBreakLongWordsAccountsForLetterSpacing is the regression test for the
// letter-spacing gap: chunks must be sized (and fit-tested) including the
// (n-1) inter-character gaps the renderer's Tc operator will draw, matching
// the intrinsic-width formula in paragraph_measure.go.
func TestBreakLongWordsAccountsForLetterSpacing(t *testing.T) {
	const fontSize = 12
	const ls = 5.0
	source := strings.Repeat("a", 30)
	wholeWidth := font.Helvetica.MeasureString(source, fontSize)
	if wholeWidth == 0 {
		t.Fatal("Helvetica measured zero width; test cannot drive breakLongWords")
	}
	maxWidth := wholeWidth / 4
	w := Word{
		Text:          source,
		Width:         wholeWidth + ls*float64(utf8.RuneCountInString(source)-1),
		Font:          font.Helvetica,
		FontSize:      fontSize,
		LetterSpacing: ls,
	}
	chunks := breakLongWords([]Word{w}, maxWidth)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	var rejoined strings.Builder
	for i, c := range chunks {
		if c.Width > maxWidth+0.01 {
			t.Errorf("chunk %d: width %v exceeds maxWidth %v (letter-spacing not accounted for)", i, c.Width, maxWidth)
		}
		n := utf8.RuneCountInString(c.Text)
		if n > 1 {
			plain := font.Helvetica.MeasureString(c.Text, fontSize)
			wantWidth := plain + ls*float64(n-1)
			if diff := c.Width - wantWidth; diff > 0.01 || diff < -0.01 {
				t.Errorf("chunk %d: width %v, want %v (plain %v + %d*ls)", i, c.Width, wantWidth, plain, n-1)
			}
		}
		rejoined.WriteString(c.Text)
	}
	if rejoined.String() != source {
		t.Errorf("chunk Texts do not rejoin to the source.\n  source: %q\n  rejoined: %q",
			source, rejoined.String())
	}
}

// TestBreakLongWordsEmptyAndNegativeMaxWidth is the regression test for a
// latent panic: a zero-width word (e.g. a blank LineBreak marker) reaching
// the split path when maxWidth is negative (a Div whose padding+borders
// exceed its width) must be passed through, not index runes[0:1] on an
// empty slice.
func TestBreakLongWordsEmptyAndNegativeMaxWidth(t *testing.T) {
	words := []Word{
		{LineBreak: true}, // empty Text, zero Width
		{Text: "x", Width: 5, Font: font.Helvetica, FontSize: 12},
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("breakLongWords panicked: %v", r)
		}
	}()
	result := breakLongWords(words, -10)
	if len(result) == 0 {
		t.Fatal("expected words to be passed through, got none")
	}
}
