// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

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
