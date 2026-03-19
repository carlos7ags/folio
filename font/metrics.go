// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

// TextMeasurer measures the width of text for layout purposes.
type TextMeasurer interface {
	// MeasureString returns the width of the given text in PDF points
	// at the specified font size.
	MeasureString(text string, fontSize float64) float64
}

// MeasureString implements TextMeasurer for standard fonts.
// Uses hardcoded width tables from the PDF spec (Appendix D).
func (f *Standard) MeasureString(text string, fontSize float64) float64 {
	widths := standardWidths[f.name]
	if widths == nil {
		// Fallback: assume 600 units per char (Courier-like)
		return float64(len(text)) * 600.0 / 1000.0 * fontSize
	}

	var total float64
	for _, r := range text {
		w, ok := widths[r]
		if !ok {
			w = widths[0] // .notdef / default width
			if w == 0 {
				w = 500 // reasonable default
			}
		}
		total += float64(w)
	}
	// Widths are in units of 1/1000 of text space. Multiply by fontSize/1000.
	return total / 1000.0 * fontSize
}

// MeasureString implements TextMeasurer for embedded fonts.
func (ef *EmbeddedFont) MeasureString(text string, fontSize float64) float64 {
	face := ef.face
	upem := face.UnitsPerEm()
	var total float64
	for _, r := range text {
		gid := face.GlyphIndex(r)
		adv := face.GlyphAdvance(gid)
		total += float64(adv)
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
