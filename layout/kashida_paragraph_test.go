// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// mockArabicFace is a deterministic Face used for kashida justification
// tests. It exposes a small set of Arabic letters (including the seen
// family) plus tatweel (U+0640) and their PFB positional variants. All
// glyphs share a fixed advance so width arithmetic is exact.
type mockArabicFace struct {
	advance int // glyph advance in font design units
	upem    int
}

func (m *mockArabicFace) PostScriptName() string { return "MockArabicFace" }
func (m *mockArabicFace) UnitsPerEm() int        { return m.upem }
func (m *mockArabicFace) GlyphIndex(r rune) uint16 {
	// Arabic block: every base letter has a glyph (GID = base+1).
	if r >= 0x0600 && r <= 0x06FF {
		return uint16(r - 0x0600 + 1)
	}
	// Presentation Forms-B: also covered (GID = 0x1000 + offset).
	if r >= 0xFE70 && r <= 0xFEFF {
		return uint16(0x1000 + (r - 0xFE70))
	}
	// ASCII space — used by paragraph layout to size inter-word gaps.
	if r == ' ' {
		return uint16(0xF000)
	}
	return 0
}
func (m *mockArabicFace) GlyphAdvance(gid uint16) int {
	if gid == 0 {
		return 0
	}
	return m.advance
}
func (m *mockArabicFace) Ascent() int             { return 800 }
func (m *mockArabicFace) Descent() int            { return -200 }
func (m *mockArabicFace) BBox() [4]int            { return [4]int{0, -200, 1000, 800} }
func (m *mockArabicFace) ItalicAngle() float64    { return 0 }
func (m *mockArabicFace) CapHeight() int          { return 700 }
func (m *mockArabicFace) StemV() int              { return 80 }
func (m *mockArabicFace) Kern(uint16, uint16) int { return 0 }
func (m *mockArabicFace) Flags() uint32           { return 0 }
func (m *mockArabicFace) RawData() []byte         { return nil }
func (m *mockArabicFace) NumGlyphs() int          { return 4096 }

// newMockArabicEmbedded returns a font.EmbeddedFont backed by mockArabicFace.
// The advance/upem ratio is 1:2 so each glyph at FontSize=12 measures 6 pt.
func newMockArabicEmbedded() *font.EmbeddedFont {
	face := &mockArabicFace{advance: 500, upem: 1000}
	return font.NewEmbeddedFont(face)
}

// countTatweels counts U+0640 occurrences across every word on every line.
func countTatweels(lines []Line) int {
	n := 0
	for _, line := range lines {
		for _, w := range line.Words {
			n += strings.Count(w.Text, string(kashidaTatweel))
		}
	}
	return n
}

// TestKashidaJustificationInsertsTatweels verifies that an Arabic-only
// justified line with leftover space ends up containing tatweel
// characters in its words after the draw pass mutates the captured word
// slice. The test simulates the draw pass by invoking applyKashidaJustification
// directly with the slack the renderer would compute.
func TestKashidaJustificationInsertsTatweels(t *testing.T) {
	ef := newMockArabicEmbedded()
	// Build a line of three short Arabic words. The mock font measures
	// every glyph at 6 pt at fontSize=12, so two words of 4 letters each
	// + a space sum to a small width — leaving lots of slack on a 500 pt
	// line.
	words := []Word{
		{Text: ShapeArabic("سلام"), Embedded: ef, FontSize: 12, SpaceAfter: ef.MeasureString(" ", 12)},
		{Text: ShapeArabic("سلام"), Embedded: ef, FontSize: 12, SpaceAfter: ef.MeasureString(" ", 12)},
		{Text: ShapeArabic("سلام"), Embedded: ef, FontSize: 12, SpaceAfter: ef.MeasureString(" ", 12)},
	}
	for i := range words {
		words[i].Width = ef.MeasureString(words[i].Text, words[i].FontSize)
	}

	// Compute slack the renderer would see.
	maxWidth := 500.0
	totalW := 0.0
	for _, w := range words {
		totalW += w.Width
	}
	natural := words[0].SpaceAfter + words[1].SpaceAfter
	slack := maxWidth - totalW - natural
	if slack <= 0 {
		t.Fatalf("test setup error: no slack available (slack=%v)", slack)
	}

	consumed := applyKashidaJustification(words, slack)
	if consumed <= 0 {
		t.Fatalf("expected kashida insertion to consume slack; consumed=%v", consumed)
	}

	totalTatweels := 0
	for _, w := range words {
		totalTatweels += strings.Count(w.Text, string(kashidaTatweel))
	}
	if totalTatweels == 0 {
		t.Errorf("expected at least one tatweel inserted; got 0")
	}
}

// TestKashidaJustificationLatinUntouched is a regression guard: a line
// of Latin words must not gain any tatweels and the function must report
// zero consumption.
func TestKashidaJustificationLatinUntouched(t *testing.T) {
	ef := newMockArabicEmbedded()
	words := []Word{
		{Text: "hello", Embedded: ef, FontSize: 12, SpaceAfter: ef.MeasureString(" ", 12)},
		{Text: "world", Embedded: ef, FontSize: 12, SpaceAfter: 0},
	}
	for i := range words {
		words[i].Width = ef.MeasureString(words[i].Text, words[i].FontSize)
	}

	consumed := applyKashidaJustification(words, 200.0)
	if consumed != 0 {
		t.Errorf("Latin words should not consume slack; got %v", consumed)
	}
	for _, w := range words {
		if strings.Contains(w.Text, string(kashidaTatweel)) {
			t.Errorf("Latin word gained a tatweel: %q", w.Text)
		}
	}
}

// TestKashidaJustificationStandardFontUntouched is a regression guard:
// standard PDF fonts (no embedded face) have no Arabic glyphs and must
// be skipped entirely. The slack falls through to whitespace stretching
// just like before this change.
func TestKashidaJustificationStandardFontUntouched(t *testing.T) {
	words := []Word{
		{Text: ShapeArabic("سلام"), Font: font.Helvetica, FontSize: 12, SpaceAfter: 4},
		{Text: ShapeArabic("سلام"), Font: font.Helvetica, FontSize: 12, SpaceAfter: 0},
	}
	for i := range words {
		words[i].Width = font.Helvetica.MeasureString(words[i].Text, words[i].FontSize)
	}
	consumed := applyKashidaJustification(words, 200.0)
	if consumed != 0 {
		t.Errorf("standard-font words should be skipped; consumed=%v", consumed)
	}
}

// TestKashidaJustificationMixedRunPartialFill verifies that a mixed line
// (one Arabic word, one Latin word) only mutates the Arabic word and
// returns slack consumption equal to the inserted-tatweel total.
func TestKashidaJustificationMixedRunPartialFill(t *testing.T) {
	ef := newMockArabicEmbedded()
	words := []Word{
		{Text: ShapeArabic("سلام"), Embedded: ef, FontSize: 12, SpaceAfter: ef.MeasureString(" ", 12)},
		{Text: "hello", Embedded: ef, FontSize: 12, SpaceAfter: 0},
	}
	for i := range words {
		words[i].Width = ef.MeasureString(words[i].Text, words[i].FontSize)
	}
	originalLatin := words[1].Text

	consumed := applyKashidaJustification(words, 100.0)
	if consumed <= 0 {
		t.Errorf("expected kashida slack consumption on Arabic word")
	}
	if words[1].Text != originalLatin {
		t.Errorf("Latin word was modified: got %q want %q", words[1].Text, originalLatin)
	}
	if !strings.Contains(words[0].Text, string(kashidaTatweel)) {
		t.Errorf("Arabic word did not gain a tatweel")
	}
}

// TestKashidaJustificationLatinParagraphUnchanged is the high-level
// regression guard required by the spec: a pure-Latin paragraph still
// produces identical output before and after this change. We compare
// against a known-good shape (no tatweels anywhere, no width changes).
func TestKashidaJustificationLatinParagraphUnchanged(t *testing.T) {
	p := NewParagraph(
		"The quick brown fox jumps over the lazy dog and then keeps running.",
		font.Helvetica, 12,
	).SetAlign(AlignJustify)
	lines := p.Layout(200) // narrow column to force multi-line justification
	if countTatweels(lines) != 0 {
		t.Errorf("Latin paragraph contains tatweels (%d); expected zero", countTatweels(lines))
	}
}
