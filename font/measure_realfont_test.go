// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"os"
	"testing"
)

// fixtureAdvance mirrors advanceFor in testdata/build_metrics_fixture.go:
// the advance width (font units) of rune r in the synthetic_metrics_*.ttf
// fixtures.
func fixtureAdvance(r rune) int {
	if r == ' ' {
		return 250
	}
	return 500 + 10*int(r-0x20)
}

// TestMeasureStringMatchesHmtxAdvances is a regression guard for an embedded
// TrueType font: it asserts MeasureString's per-glyph sum is bit-identical to
// the font's own hmtx advance-width table for every rune in the test string,
// with no kerning applied (the fixture ships no kern/GPOS data). A change
// that introduces a systematic advance-width drift (wrong cmap subtable,
// wrong scale, double-counted kerning, etc.) would break it.
func TestMeasureStringMatchesHmtxAdvances(t *testing.T) {
	data, err := os.ReadFile("testdata/synthetic_metrics_hhea.ttf")
	if err != nil {
		t.Fatalf("read font: %v", err)
	}
	face, err := ParseTTF(data)
	if err != nil {
		t.Fatalf("parse font: %v", err)
	}
	ef := NewEmbeddedFont(face)

	text := "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 abc"
	wantUnits := 0
	for _, r := range text {
		wantUnits += fixtureAdvance(r)
	}
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
