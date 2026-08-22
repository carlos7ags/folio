// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"os"
	"testing"
)

// TestMeasureStringMatchesHmtxAdvances is a regression guard for a real
// embedded font: it asserts MeasureString's per-glyph sum is bit-identical
// to the font's own hmtx advance-width table (verified independently with
// fontTools) for every rune in the test string, with no kerning applied
// (this font ships no pair-kerning data). A prior investigation confirmed
// this equality already holds; a future change that introduces a systematic
// advance-width drift (wrong cmap subtable, wrong scale, double-counted
// kerning, etc.) would break it.
func TestMeasureStringMatchesHmtxAdvances(t *testing.T) {
	data, err := os.ReadFile("testdata/Poppins-Bold.ttf")
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	face, err := ParseTTF(data)
	if err != nil {
		t.Fatalf("parse font: %v", err)
	}
	ef := NewEmbeddedFont(face)

	text := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	const wantUnits = 23674 // fontTools-verified sum of hmtx advances, no kerning
	upem := face.UnitsPerEm()
	if upem != 1000 {
		t.Fatalf("unexpected unitsPerEm=%d (fixture changed?)", upem)
	}

	var gotUnits int
	for _, r := range text {
		gotUnits += face.GlyphAdvance(face.GlyphIndex(r))
	}
	if gotUnits != wantUnits {
		t.Errorf("sum of glyph advances = %d units, want %d units (hmtx ground truth)", gotUnits, wantUnits)
	}

	const fontSize = 16.0
	want := float64(wantUnits) / float64(upem) * fontSize
	got := ef.MeasureString(text, fontSize)
	if got != want {
		t.Errorf("MeasureString(%q, %v) = %v, want %v (bit-identical to hmtx sum)", text, fontSize, got, want)
	}
}
