// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"math"
	"testing"
)

// prefixesCorpus reuses characterizationCorpus plus a few extra strings
// with long runs, so the cumulative-walk path in MeasureStringPrefixes is
// exercised over more than a handful of rune boundaries.
var prefixesCorpus = append(append([]string{}, characterizationCorpus...),
	"abcdefghijabcdefghij",
	"AVAWAToToAVAWAToTo",
	"中华人民共和国是一个历史悠久的文明古国",
)

var prefixesSizes = []float64{8, 12, 12.5}

// checkPrefixes asserts prefixes has one entry per rune boundary and that
// every entry is bit-identical to measuring that prefix independently.
func checkPrefixes(t *testing.T, label string, s string, prefixes []float64, measure func(string) float64) {
	t.Helper()
	runes := []rune(s)
	if len(prefixes) != len(runes)+1 {
		t.Fatalf("%s: len(prefixes) = %d, want %d", label, len(prefixes), len(runes)+1)
	}
	for i := 0; i <= len(runes); i++ {
		want := measure(string(runes[:i]))
		if math.Float64bits(prefixes[i]) != math.Float64bits(want) {
			t.Errorf("%s: prefixes[%d] = %v (0x%x), want bit-identical %v (0x%x)",
				label, i, prefixes[i], math.Float64bits(prefixes[i]), want, math.Float64bits(want))
		}
	}
}

func TestMeasureStringPrefixesStandard(t *testing.T) {
	fonts := []*Standard{Helvetica, Symbol}
	for _, f := range fonts {
		for _, s := range prefixesCorpus {
			for _, size := range prefixesSizes {
				prefixes := f.MeasureStringPrefixes(s, size)
				checkPrefixes(t, f.name, s, prefixes, func(prefix string) float64 {
					return f.MeasureString(prefix, size)
				})
			}
		}
	}
}

// TestMeasureStringPrefixesStandardNilWidths exercises the fallback path
// (no width table for the font name) via an unexported Standard built with
// an unknown name — standardWidths covers every exported standard font, so
// this is the only way to reach the byte-length fallback formula.
func TestMeasureStringPrefixesStandardNilWidths(t *testing.T) {
	f := &Standard{name: "Unknown"}
	for _, s := range prefixesCorpus {
		for _, size := range prefixesSizes {
			prefixes := f.MeasureStringPrefixes(s, size)
			checkPrefixes(t, "Unknown", s, prefixes, func(prefix string) float64 {
				return f.MeasureString(prefix, size)
			})
		}
	}
}

func TestMeasureStringPrefixesEmbedded(t *testing.T) {
	face, err := LoadFont("testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("LoadFont: %v", err)
	}
	ef := NewEmbeddedFont(face)

	for _, s := range prefixesCorpus {
		for _, size := range prefixesSizes {
			prefixes := ef.MeasureStringPrefixes(s, size)
			checkPrefixes(t, "embedded", s, prefixes, func(prefix string) float64 {
				return ef.MeasureString(prefix, size)
			})
		}
	}
}

// refPrefixFit derives the expected (byteLen, runeLen, width) of PrefixFit
// from the full prefix widths, mirroring the first-overflow scan that word
// chunking runs: start with one rune, extend while the next rune boundary
// stays within maxWidth, stop at the first overflow.
func refPrefixFit(s string, prefixes []float64, maxWidth float64) (byteLen, runeLen int, width float64) {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return 0, 0, 0
	}
	end := 1
	for end < n {
		if prefixes[end+1] > maxWidth {
			break
		}
		end++
	}
	return len(string(runes[:end])), end, prefixes[end]
}

// checkPrefixFit compares PrefixFit against the reference over a spread of
// maxWidth values, including exact rune-boundary widths (boundary equality).
func checkPrefixFit(t *testing.T, label, s string, prefixes []float64, fit func(maxWidth float64) (int, int, float64)) {
	t.Helper()
	total := prefixes[len(prefixes)-1]
	widths := []float64{0, total * 0.1, total * 0.25, total * 0.5, total * 0.9, total, total * 2}
	widths = append(widths, prefixes...) // exact rune-boundary widths
	for _, mw := range widths {
		wantBytes, wantRunes, wantWidth := refPrefixFit(s, prefixes, mw)
		gotBytes, gotRunes, gotWidth := fit(mw)
		if gotBytes != wantBytes || gotRunes != wantRunes {
			t.Errorf("%s PrefixFit(%q, maxWidth=%v) = (bytes=%d, runes=%d), want (bytes=%d, runes=%d)",
				label, s, mw, gotBytes, gotRunes, wantBytes, wantRunes)
		}
		if math.Float64bits(gotWidth) != math.Float64bits(wantWidth) {
			t.Errorf("%s PrefixFit(%q, maxWidth=%v) width = %v (0x%x), want bit-identical %v (0x%x)",
				label, s, mw, gotWidth, math.Float64bits(gotWidth), wantWidth, math.Float64bits(wantWidth))
		}
	}
}

func TestPrefixFitStandard(t *testing.T) {
	fonts := []*Standard{Helvetica, Symbol, {name: "Unknown"}}
	for _, f := range fonts {
		for _, s := range prefixesCorpus {
			if s == "" {
				continue
			}
			for _, size := range prefixesSizes {
				prefixes := f.MeasureStringPrefixes(s, size)
				checkPrefixFit(t, f.name, s, prefixes, func(mw float64) (int, int, float64) {
					return f.PrefixFit(s, size, mw)
				})
			}
		}
	}
}

func TestPrefixFitEmbedded(t *testing.T) {
	face, err := LoadFont("testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("LoadFont: %v", err)
	}
	ef := NewEmbeddedFont(face)
	for _, s := range prefixesCorpus {
		if s == "" {
			continue
		}
		for _, size := range prefixesSizes {
			prefixes := ef.MeasureStringPrefixes(s, size)
			checkPrefixFit(t, "embedded", s, prefixes, func(mw float64) (int, int, float64) {
				return ef.PrefixFit(s, size, mw)
			})
		}
	}
}

// zeroUpemFace wraps fakeFace to force the UnitsPerEm() == 0 path, where
// MeasureString returns 0 for every input.
type zeroUpemFace struct{ *fakeFace }

func (zeroUpemFace) UnitsPerEm() int { return 0 }

// TestMeasureStringPrefixesEmbeddedZeroUpem exercises the upem==0 path,
// where MeasureString returns 0 for every input.
func TestMeasureStringPrefixesEmbeddedZeroUpem(t *testing.T) {
	face := zeroUpemFace{newFakeFace()}
	ef := NewEmbeddedFont(face)

	for _, s := range prefixesCorpus {
		prefixes := ef.MeasureStringPrefixes(s, 12)
		checkPrefixes(t, "zero-upem", s, prefixes, func(prefix string) float64 {
			return ef.MeasureString(prefix, 12)
		})
	}
}
