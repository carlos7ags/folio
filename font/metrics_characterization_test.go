// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"math"
	"testing"
	"unicode/utf8"

	"github.com/carlos7ags/folio/unicode/grapheme"
)

// referenceMeasureStandard is a verbatim copy of the pre-optimization
// Standard.MeasureString loop body (grapheme.Breaks-based). It pins the
// exact floating-point result so a later change to the iteration
// scaffolding can be checked for bit-identical output.
func referenceMeasureStandard(f *Standard, text string, fontSize float64) float64 {
	widths := standardWidths[f.name]
	if widths == nil {
		return float64(len(text)) * 600.0 / 1000.0 * fontSize
	}

	lookupWidth := func(r rune) float64 {
		w, ok := widths[r]
		if !ok {
			w = widths[0]
			if w == 0 {
				w = 500
			}
		}
		return float64(w)
	}

	var total float64
	var prevTail rune
	havePrev := false

	breaks := grapheme.Breaks(text)
	for i := 0; i+1 < len(breaks); i++ {
		cluster := text[breaks[i]:breaks[i+1]]
		baseRune, baseSize := utf8.DecodeRuneInString(cluster)
		if havePrev {
			total += f.Kern(prevTail, baseRune)
		}
		total += lookupWidth(baseRune)
		tail := baseRune
		for off := baseSize; off < len(cluster); {
			r, size := utf8.DecodeRuneInString(cluster[off:])
			if grapheme.PropertyOf(r) == grapheme.PropSpacingMark {
				total += lookupWidth(r)
				tail = r
			}
			off += size
		}
		prevTail = tail
		havePrev = true
	}

	return total / 1000.0 * fontSize
}

// referenceMeasureEmbedded is a verbatim copy of the pre-optimization
// EmbeddedFont.MeasureString loop body (grapheme.Breaks-based).
func referenceMeasureEmbedded(ef *EmbeddedFont, text string, fontSize float64) float64 {
	face := ef.face
	upem := face.UnitsPerEm()
	if upem == 0 {
		return 0
	}

	var total float64
	var prevTail uint16
	havePrev := false

	breaks := grapheme.Breaks(text)
	for i := 0; i+1 < len(breaks); i++ {
		cluster := text[breaks[i]:breaks[i+1]]
		baseRune, baseSize := utf8.DecodeRuneInString(cluster)
		baseGID := face.GlyphIndex(baseRune)
		if havePrev {
			total += float64(face.Kern(prevTail, baseGID))
		}
		total += float64(face.GlyphAdvance(baseGID))
		tail := baseGID
		for off := baseSize; off < len(cluster); {
			r, size := utf8.DecodeRuneInString(cluster[off:])
			if grapheme.PropertyOf(r) == grapheme.PropSpacingMark {
				gid := face.GlyphIndex(r)
				total += float64(face.GlyphAdvance(gid))
				tail = gid
			}
			off += size
		}
		prevTail = tail
		havePrev = true
	}
	return total / float64(upem) * fontSize
}

// characterizationCorpus exercises the grapheme-cluster boundary rules
// MeasureString depends on: plain ASCII, combining marks, SpacingMark
// (Devanagari), regional-indicator flags, ZWJ emoji sequences, Prepend,
// and Arabic presentation forms.
var characterizationCorpus = []string{
	"",
	"a",
	"AVAILABLE To Watch",
	"éf",
	"काख",
	"\U0001F1FA\U0001F1F8\U0001F1E9\U0001F1EA",
	"\U0001F469‍\U0001F4BB",
	"؀٢",
	"ﺍﻠ",
}

var characterizationSizes = []float64{8, 12, 12.5, 72}

func TestMeasureStringCharacterizationStandard(t *testing.T) {
	fonts := []*Standard{Helvetica, Symbol}
	for _, f := range fonts {
		for _, s := range characterizationCorpus {
			for _, size := range characterizationSizes {
				got := f.MeasureString(s, size)
				want := referenceMeasureStandard(f, s, size)
				if math.Float64bits(got) != math.Float64bits(want) {
					t.Errorf("%s.MeasureString(%q, %v) = %v, want bit-identical %v", f.name, s, size, got, want)
				}
			}
		}
	}
}

func TestMeasureStringCharacterizationEmbedded(t *testing.T) {
	face, err := LoadFont("testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("LoadFont: %v", err)
	}
	ef := NewEmbeddedFont(face)

	corpus := append([]string{}, characterizationCorpus...)
	corpus = append(corpus, "中华人民共和国是一个历史悠久的文明古国")

	for _, s := range corpus {
		for _, size := range characterizationSizes {
			got := ef.MeasureString(s, size)
			want := referenceMeasureEmbedded(ef, s, size)
			if math.Float64bits(got) != math.Float64bits(want) {
				t.Errorf("EmbeddedFont.MeasureString(%q, %v) = %v, want bit-identical %v", s, size, got, want)
			}
		}
	}
}
