// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"unicode/utf8"

	"github.com/carlos7ags/folio/unicode/grapheme"
)

// TextMeasurer measures the width of text for layout purposes.
type TextMeasurer interface {
	// MeasureString returns the width of the given text in PDF points
	// at the specified font size.
	MeasureString(text string, fontSize float64) float64
}

// Ascent returns the typographic ascent for the standard font, scaled to
// the given font size in points. Values are from the PDF spec (Appendix D).
func (f *Standard) Ascent(fontSize float64) float64 {
	a := standardAscent[f.name]
	if a == 0 {
		a = 718 // Helvetica default
	}
	return float64(a) / 1000 * fontSize
}

// Descent returns the typographic descent for the standard font, scaled to
// the given font size in points. The value is positive (distance below baseline).
func (f *Standard) Descent(fontSize float64) float64 {
	d := standardDescent[f.name]
	if d == 0 {
		d = 207 // Helvetica default
	}
	return float64(d) / 1000 * fontSize
}

// Standard font ascent/descent values from the PDF spec (Appendix D).
// Ascent is the distance above the baseline, descent is the distance below
// (stored as positive values here).
var standardAscent = map[string]int{
	"Helvetica":             718,
	"Helvetica-Bold":        718,
	"Helvetica-Oblique":     718,
	"Helvetica-BoldOblique": 718,
	"Times-Roman":           683,
	"Times-Bold":            683,
	"Times-Italic":          683,
	"Times-BoldItalic":      683,
	"Courier":               629,
	"Courier-Bold":          626,
	"Courier-Oblique":       629,
	"Courier-BoldOblique":   626,
	"Symbol":                673,
	"ZapfDingbats":          677,
}

var standardDescent = map[string]int{
	"Helvetica":             207,
	"Helvetica-Bold":        207,
	"Helvetica-Oblique":     207,
	"Helvetica-BoldOblique": 207,
	"Times-Roman":           217,
	"Times-Bold":            217,
	"Times-Italic":          217,
	"Times-BoldItalic":      217,
	"Courier":               157,
	"Courier-Bold":          142,
	"Courier-Oblique":       157,
	"Courier-BoldOblique":   142,
	"Symbol":                216,
	"ZapfDingbats":          143,
}

// MeasureString implements TextMeasurer for standard fonts. The returned
// width is in PDF points and accounts for any kerning pairs the font
// supplies via Kern(), so wrapping widths agree with the advances that
// drawWordStandard emits via TJ adjustments.
//
// Cluster awareness: the advance is summed over extended grapheme
// clusters (UAX #29 §3.1.1), not per rune. Within a cluster the base
// rune contributes its full advance. Subsequent runes contribute zero
// unless they are SpacingMark (Devanagari-style vowel signs that take
// horizontal space); Extend (combining accents such as U+0301) and
// ZWJ contribute no advance, matching how they draw at zero offset on
// top of the base glyph. Kerning is applied between cluster
// boundaries, never between a base and its marks.
//
// Uses the hardcoded width tables from the PDF spec (Appendix D) and
// the AFM-derived kern pairs in standardKernPairs.
func (f *Standard) MeasureString(text string, fontSize float64) float64 {
	widths := standardWidths[f.name]
	if widths == nil {
		// Fallback: assume 600 units per char (Courier-like). Fallback fonts
		// have no kern data, so this path ignores kerning. Fallback fonts
		// also don't care about cluster composition — every byte counts as
		// one fixed-width cell.
		return float64(len(text)) * 600.0 / 1000.0 * fontSize
	}

	lookupWidth := func(r rune) float64 {
		w, ok := widths[r]
		if !ok {
			w = widths[0] // .notdef / default width
			if w == 0 {
				w = 500 // reasonable default
			}
		}
		return float64(w)
	}

	var total float64
	var prevTail rune // last contributing rune of the previous cluster, for kerning
	havePrev := false

	breaks := grapheme.Breaks(text)
	for i := 0; i+1 < len(breaks); i++ {
		cluster := text[breaks[i]:breaks[i+1]]
		// First rune is the cluster base — always contributes advance.
		baseRune, baseSize := utf8.DecodeRuneInString(cluster)
		if havePrev {
			total += f.Kern(prevTail, baseRune)
		}
		total += lookupWidth(baseRune)
		tail := baseRune
		// Walk the remaining runes in the cluster. SpacingMarks take
		// advance (GB9a codepoints like Devanagari vowel signs). Extend
		// and ZWJ do not: they render at zero offset on the base.
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

	// Widths and kern values are in units of 1/1000 of text space.
	return total / 1000.0 * fontSize
}

// MeasureString implements TextMeasurer for embedded fonts. The returned
// width is in PDF points and accounts for any kerning pairs the font
// supplies via its kern table, so wrapping widths agree with the
// advances that drawWordEmbedded emits via TJ adjustments.
//
// Cluster awareness: the advance is summed over extended grapheme
// clusters (UAX #29 §3.1.1), not per glyph. Within a cluster the base
// glyph contributes its full advance. Subsequent codepoints contribute
// zero unless they are SpacingMark (Devanagari-style vowel signs that
// take horizontal space); Extend (combining accents such as U+0301)
// and ZWJ contribute no advance, matching how GPOS mark-attachment
// positions them as zero-width glyphs over the base. Kerning is
// applied between cluster boundaries, never between a base and its
// marks.
func (ef *EmbeddedFont) MeasureString(text string, fontSize float64) float64 {
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

// Kern returns the kerning adjustment between two characters in thousandths
// of a unit of text space. Standard PDF fonts have limited kerning data;
// this returns common kern pairs for Helvetica and Times families.
// Negative values mean the glyphs should be closer together.
func (f *Standard) Kern(left, right rune) float64 {
	pairs := standardKernPairs[f.name]
	if pairs == nil {
		return 0
	}
	key := kernKey{left, right}
	return float64(pairs[key])
}

// kernKey identifies a pair of characters for kern lookup.
type kernKey struct {
	left, right rune
}

// standardKernPairs provides common kerning pairs for standard fonts.
// Values are in 1/1000 of text space unit (negative = tighter).
// These are the most impactful pairs from the AFM (Adobe Font Metrics) files.
var standardKernPairs = map[string]map[kernKey]int{
	"Helvetica":             helveticaKernPairs,
	"Helvetica-Bold":        helveticaBoldKernPairs,
	"Helvetica-Oblique":     helveticaKernPairs,
	"Helvetica-BoldOblique": helveticaBoldKernPairs,
	"Times-Roman":           timesRomanKernPairs,
	"Times-Bold":            timesBoldKernPairs,
	"Times-Italic":          timesItalicKernPairs,
	"Times-BoldItalic":      timesBoldItalicKernPairs,
}

// standardWidths maps font name → (rune → width in 1/1000 units).
// Generated from Adobe AFM files — see cmd/gen-metrics.
// Kern pair data is in metrics_data.go (also generated).
var standardWidths = map[string]map[rune]int{
	"Helvetica":             helveticaWidths,
	"Helvetica-Bold":        helveticaBoldWidths,
	"Helvetica-Oblique":     helveticaWidths, // same metrics as Helvetica
	"Helvetica-BoldOblique": helveticaBoldWidths,
	"Times-Roman":           timesRomanWidths,
	"Times-Bold":            timesBoldWidths,
	"Times-Italic":          timesItalicWidths,
	"Times-BoldItalic":      timesBoldItalicWidths,
	"Courier":               courierWidths,
	"Courier-Bold":          courierWidths, // Courier is monospaced
	"Courier-Oblique":       courierWidths,
	"Courier-BoldOblique":   courierWidths,
	"Symbol":                symbolWidths,
	"ZapfDingbats":          zapfDingbatsWidths,
}
