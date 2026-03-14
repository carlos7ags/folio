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
	"Helvetica-Bold":        helveticaKernPairs,
	"Helvetica-Oblique":     helveticaKernPairs,
	"Helvetica-BoldOblique": helveticaKernPairs,
	"Times-Roman":           timesKernPairs,
	"Times-Bold":            timesKernPairs,
	"Times-Italic":          timesKernPairs,
	"Times-BoldItalic":      timesKernPairs,
}

// helveticaKernPairs — most impactful kern pairs from the Helvetica AFM.
var helveticaKernPairs = map[kernKey]int{
	{'A', 'V'}: -80, {'A', 'W'}: -60, {'A', 'Y'}: -110, {'A', 'v'}: -40,
	{'A', 'w'}: -40, {'A', 'y'}: -40, {'A', 'T'}: -90,
	{'F', 'A'}: -80, {'F', 'a'}: -20, {'F', 'o'}: -30,
	{'L', 'T'}: -90, {'L', 'V'}: -110, {'L', 'W'}: -80, {'L', 'Y'}: -120,
	{'P', 'A'}: -100, {'P', 'a'}: -30, {'P', 'o'}: -40,
	{'T', 'A'}: -90, {'T', 'a'}: -80, {'T', 'e'}: -60, {'T', 'o'}: -80,
	{'T', 'r'}: -40, {'T', 'y'}: -60,
	{'V', 'A'}: -80, {'V', 'a'}: -60, {'V', 'e'}: -50, {'V', 'o'}: -50,
	{'W', 'A'}: -60, {'W', 'a'}: -40, {'W', 'e'}: -35, {'W', 'o'}: -35,
	{'Y', 'A'}: -110, {'Y', 'a'}: -90, {'Y', 'e'}: -80, {'Y', 'o'}: -80,
	{'Y', 'p'}: -50, {'Y', 'u'}: -60,
	{'r', '.'}: -40, {'r', ','}: -40,
	{'f', '.'}: -80, {'f', ','}: -80,
}

// timesKernPairs — most impactful kern pairs from the Times Roman AFM.
var timesKernPairs = map[kernKey]int{
	{'A', 'V'}: -135, {'A', 'W'}: -90, {'A', 'Y'}: -105, {'A', 'v'}: -55,
	{'A', 'w'}: -55, {'A', 'y'}: -55, {'A', 'T'}: -95,
	{'F', 'A'}: -115, {'F', 'a'}: -75, {'F', 'o'}: -105,
	{'L', 'T'}: -92, {'L', 'V'}: -100, {'L', 'W'}: -74, {'L', 'Y'}: -100,
	{'P', 'A'}: -92, {'P', 'a'}: -15, {'P', 'o'}: -35,
	{'T', 'A'}: -95, {'T', 'a'}: -92, {'T', 'e'}: -92, {'T', 'o'}: -95,
	{'T', 'r'}: -37, {'T', 'y'}: -37,
	{'V', 'A'}: -135, {'V', 'a'}: -92, {'V', 'e'}: -100, {'V', 'o'}: -100,
	{'W', 'A'}: -120, {'W', 'a'}: -65, {'W', 'e'}: -65, {'W', 'o'}: -75,
	{'Y', 'A'}: -120, {'Y', 'a'}: -92, {'Y', 'e'}: -92, {'Y', 'o'}: -92,
	{'Y', 'p'}: -55, {'Y', 'u'}: -92,
	{'r', '.'}: -65, {'r', ','}: -65,
	{'f', '.'}: -80, {'f', ','}: -80,
}

// standardWidths maps font name → (rune → width in 1/1000 units).
// These are the standard widths from the PDF spec Appendix D.
// Only the most common Latin characters are included; missing chars
// fall back to the default width.
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
	"Symbol":                nil, // not used for text layout
	"ZapfDingbats":          nil,
}
