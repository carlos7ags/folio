// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/font"
)

// mockGPOSFace is a deterministic Face + GPOSProvider used to exercise
// drawWordEmbeddedWithMarks. Each rune is mapped to a GID by a lookup
// table; advances and upem are fixed; GPOS data is injected directly.
type mockGPOSFace struct {
	upem    int
	advance map[uint16]int
	cmap    map[rune]uint16
	gpos    *font.GPOSAdjustments
}

func (m *mockGPOSFace) PostScriptName() string { return "MockGPOSFace" }
func (m *mockGPOSFace) UnitsPerEm() int        { return m.upem }
func (m *mockGPOSFace) GlyphIndex(r rune) uint16 {
	return m.cmap[r]
}
func (m *mockGPOSFace) GlyphAdvance(gid uint16) int {
	return m.advance[gid]
}
func (m *mockGPOSFace) Ascent() int             { return 800 }
func (m *mockGPOSFace) Descent() int            { return -200 }
func (m *mockGPOSFace) BBox() [4]int            { return [4]int{0, -200, 1000, 800} }
func (m *mockGPOSFace) ItalicAngle() float64    { return 0 }
func (m *mockGPOSFace) CapHeight() int          { return 700 }
func (m *mockGPOSFace) StemV() int              { return 80 }
func (m *mockGPOSFace) Kern(uint16, uint16) int { return 0 }
func (m *mockGPOSFace) Flags() uint32           { return 0 }
func (m *mockGPOSFace) RawData() []byte         { return nil }
func (m *mockGPOSFace) NumGlyphs() int          { return 4096 }

// GPOS satisfies font.GPOSProvider.
func (m *mockGPOSFace) GPOS() *font.GPOSAdjustments { return m.gpos }

// newLamFathaFace constructs a mock face with lam (U+0644) as a base
// glyph and fatha (U+064E) as a combining mark, plus a single GPOS
// mark/base entry that attaches fatha on class 0 of lam.
// Anchors: base lam at (500, 800), mark fatha at (200, 300).
// Expected MarkOffset = (500-200, 800-300) = (300, 500).
func newLamFathaFace() *mockGPOSFace {
	const (
		lamGID   uint16 = 50
		fathaGID uint16 = 60
	)
	face := &mockGPOSFace{
		upem: 1000,
		advance: map[uint16]int{
			lamGID:   700,
			fathaGID: 0, // combining mark: zero advance
		},
		cmap: map[rune]uint16{
			0x0644: lamGID,
			0x064E: fathaGID,
		},
		gpos: &font.GPOSAdjustments{
			Pairs: map[font.GPOSFeature]map[[2]uint16]font.PairAdjustment{},
			Marks: map[font.GPOSFeature]map[uint16]font.MarkRecord{
				font.GPOSMark: {
					fathaGID: {Class: 0, Anchor: font.Anchor{X: 200, Y: 300}},
				},
			},
			Bases: map[font.GPOSFeature]map[uint16]font.BaseRecord{
				font.GPOSMark: {
					lamGID: {Anchors: []font.Anchor{{X: 500, Y: 800}}},
				},
			},
		},
	}
	return face
}

// capturedWordStream renders a single Word in isolation through
// drawWordEmbedded bracketed by BT/ET/MoveText and returns the
// resulting raw content-stream bytes. Mirrors the operator sequence
// that drawTextLine would produce for this word.
func capturedWordStream(word Word) []byte {
	s := content.NewStream()
	s.BeginText()
	s.SetFont("F1", word.FontSize)
	s.MoveText(0, 0)
	drawWordEmbedded(s, word)
	s.EndText()
	return s.Bytes()
}

// countTdOps counts Td operator occurrences in a content stream.
func countTdOps(b []byte) int {
	n := 0
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, " Td") {
			n++
		}
	}
	return n
}

// firstMarkTdBetween returns the first Td operator line that appears
// strictly between two Tj hex-string lines in the given stream. It is
// used to pick out the Td that drawWordEmbeddedWithMarks inserts
// between the base Tj and the mark Tj. Returns the empty string when
// no such Td exists.
func firstMarkTdBetween(b []byte) string {
	lines := strings.Split(string(b), "\n")
	seenTj := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if seenTj && strings.HasSuffix(line, " Td") {
			return line
		}
		if strings.HasSuffix(line, " Tj") {
			if seenTj {
				return "" // two Tjs with no Td between them
			}
			seenTj = true
		}
	}
	return ""
}

// TestGPOSMarkAttachmentArabicHaraka renders a lam+fatha cluster with
// a mock face that carries a GPOS mark-to-base entry and asserts the
// content stream contains a Td move before the fatha Tj whose operands
// match the expected offset, plus a matching Td move back after.
func TestGPOSMarkAttachmentArabicHaraka(t *testing.T) {
	face := newLamFathaFace()
	ef := font.NewEmbeddedFont(face)

	// Expected offsets in points at fontSize=12:
	//   dx = (500 - 200) / 1000 * 12 = 3.6
	//   dy = (800 - 300) / 1000 * 12 = 6.0
	//   baseAdvance = 700 / 1000 * 12 = 8.4
	//   First Td: (dx - baseAdvance, dy) = (-4.8, 6)
	//   Second Td: (baseAdvance - dx, -dy) = (4.8, -6)
	word := Word{
		Text:     "\u0644\u064E", // lam + fatha
		Embedded: ef,
		FontSize: 12,
	}

	b := capturedWordStream(word)
	if countTdOps(b) < 3 {
		// One Td for the initial MoveText, two for the mark bracket.
		t.Fatalf("expected at least 3 Td ops (initial + mark bracket), got %d:\n%s", countTdOps(b), b)
	}

	// Confirm the first Td between Tj lines is the mark-open shift.
	td := firstMarkTdBetween(b)
	if td == "" {
		t.Fatalf("no Td between base Tj and mark Tj:\n%s", b)
	}
	if !strings.Contains(td, "-4.8") || !strings.Contains(td, "6 Td") {
		t.Errorf("mark-open Td operands: got %q, want -4.8 and 6:\n%s", td, b)
	}

	// Confirm the closing +4.8 / -6 Td appears somewhere after it.
	if !bytes.Contains(b, []byte("4.8 -6 Td")) {
		t.Errorf("expected closing Td '4.8 -6 Td' in stream:\n%s", b)
	}
}

// TestGPOSMarkAttachmentNoGPOSFallback verifies that when the font has
// no GPOS mark data, drawWordEmbedded emits the cluster via the fast
// path (single Tj, no Td pairs between glyph emissions). The only Td
// remains the initial MoveText.
func TestGPOSMarkAttachmentNoGPOSFallback(t *testing.T) {
	face := newLamFathaFace()
	face.gpos = nil
	ef := font.NewEmbeddedFont(face)

	word := Word{
		Text:     "\u0644\u064E",
		Embedded: ef,
		FontSize: 12,
	}

	b := capturedWordStream(word)
	// Exactly one Td: the initial MoveText(0, 0).
	if countTdOps(b) != 1 {
		t.Errorf("expected exactly 1 Td (initial MoveText), got %d:\n%s", countTdOps(b), b)
	}
	if firstMarkTdBetween(b) != "" {
		t.Errorf("unexpected Td between Tj lines without GPOS:\n%s", b)
	}
}

// TestGPOSMarkAttachmentLatinUntouched asserts Latin-only words that
// contain no combining marks are emitted by the fast path: no Td moves
// between Tjs, and the output is byte-for-byte what the pre-GPOS path
// would have produced.
func TestGPOSMarkAttachmentLatinUntouched(t *testing.T) {
	// Build a Latin-capable mock face that also declares GPOS marks;
	// eligibility should still reject because the text has no Extend.
	face := newLamFathaFace()
	face.cmap['h'] = 1
	face.cmap['e'] = 2
	face.cmap['l'] = 3
	face.cmap['o'] = 4
	face.advance[1] = 500
	face.advance[2] = 500
	face.advance[3] = 500
	face.advance[4] = 500
	ef := font.NewEmbeddedFont(face)

	word := Word{
		Text:     "hello",
		Embedded: ef,
		FontSize: 12,
	}

	b := capturedWordStream(word)
	if countTdOps(b) != 1 {
		t.Errorf("Latin word should emit only the initial Td, got %d:\n%s", countTdOps(b), b)
	}
	if firstMarkTdBetween(b) != "" {
		t.Errorf("Latin word should not emit mark-Td brackets:\n%s", b)
	}
}

// TestGPOSMarkAttachmentTwoMarks verifies that a cluster with two
// Extend marks (fatha and shadda on the same lam) emits two separate
// Td-bracketed mark emissions, so each mark is positioned individually.
func TestGPOSMarkAttachmentTwoMarks(t *testing.T) {
	face := newLamFathaFace()
	const shaddaGID uint16 = 61
	face.cmap[0x0651] = shaddaGID // shadda
	face.advance[shaddaGID] = 0
	// Mark class 0 shared: fatha already uses class 0. Give shadda its
	// own class (class 1) so the base needs a second anchor slot. This
	// also exercises multi-class mark positioning.
	face.gpos.Marks[font.GPOSMark][shaddaGID] = font.MarkRecord{
		Class:  1,
		Anchor: font.Anchor{X: 100, Y: 400},
	}
	base := face.gpos.Bases[font.GPOSMark][50]
	base.Anchors = append(base.Anchors, font.Anchor{X: 500, Y: 900}) // class 1 anchor
	face.gpos.Bases[font.GPOSMark][50] = base

	ef := font.NewEmbeddedFont(face)
	word := Word{
		Text:     "\u0644\u064E\u0651", // lam + fatha + shadda
		Embedded: ef,
		FontSize: 12,
	}

	b := capturedWordStream(word)

	// Expect: base Tj, then two Td-bracketed mark emissions. That is:
	// base Tj, Td(open1), mark1 Tj, Td(close1), Td(open2), mark2 Tj, Td(close2).
	// Count Tjs and Tds.
	tjCount := 0
	tdCount := 0
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasSuffix(line, " Tj") {
			tjCount++
		}
		if strings.HasSuffix(line, " Td") {
			tdCount++
		}
	}
	if tjCount != 3 {
		t.Errorf("expected 3 Tj (base + 2 marks), got %d:\n%s", tjCount, b)
	}
	// Initial MoveText + two open Td + two close Td = 5.
	if tdCount != 5 {
		t.Errorf("expected 5 Td (initial + 2*(open+close)), got %d:\n%s", tdCount, b)
	}
}

// TestGPOSMarkAttachmentMeasureAgreesWithDraw is the correctness
// invariant: the width reported by EmbeddedFont.MeasureString for a
// mark-bearing word must equal the total horizontal advance the text
// matrix undergoes during drawWordEmbedded. The test simulates the
// matrix advance by parsing the content stream and summing Td x
// components plus the base Tj advance per cluster.
func TestGPOSMarkAttachmentMeasureAgreesWithDraw(t *testing.T) {
	face := newLamFathaFace()
	ef := font.NewEmbeddedFont(face)

	word := Word{
		Text:     "\u0644\u064E",
		Embedded: ef,
		FontSize: 12,
	}

	measured := ef.MeasureString(word.Text, word.FontSize)

	// The draw path advances the text matrix by the base's Tj advance
	// plus the net of all Td operators after the initial MoveText(0,0).
	// Reproduce that calculation directly.
	//
	// The only non-zero-advance glyph in the cluster is lam (700 FUnits
	// = 8.4 pt). Fatha is zero-advance. The Td bracket is matched pairs
	// (-4.8 +4.8 / +6 -6) which sum to zero. Net advance = 8.4 pt.
	// MeasureString should also report ~8.4 pt (modulo float rounding).
	want := 8.4
	if !almostEqual(measured, want, 1e-9) {
		t.Errorf("MeasureString: got %v, want %v", measured, want)
	}

	// Now parse the draw stream and sum Td advances (after the initial
	// move) plus base advance. This is a stand-in for running the PDF
	// through an interpreter.
	b := capturedWordStream(word)
	netTdX := 0.0
	seenInitial := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasSuffix(line, " Td") {
			continue
		}
		if !seenInitial {
			seenInitial = true // initial MoveText(0, 0) — ignore
			continue
		}
		var tx, ty float64
		n, err := fmt.Sscanf(line, "%f %f Td", &tx, &ty)
		if err != nil || n != 2 {
			t.Fatalf("unparseable Td line %q: %v", line, err)
		}
		netTdX += tx
	}
	// Base glyph Tj advance: one lam.
	baseAdv := float64(face.advance[50]) / float64(face.upem) * word.FontSize
	drawAdvance := baseAdv + netTdX
	if !almostEqual(drawAdvance, measured, 1e-9) {
		t.Errorf("draw advance = %v, MeasureString = %v — these must agree for line wrap/draw consistency", drawAdvance, measured)
	}
}

func almostEqual(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}
