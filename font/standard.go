// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package font handles font loading, parsing, subsetting, and PDF embedding.
//
// It supports the 14 standard PDF fonts (which require no embedding),
// TrueType/OpenType fonts parsed via golang.org/x/image/font/sfnt,
// and WOFF1 web fonts decoded to TTF. Embedded fonts are subset to
// include only the glyphs actually used, reducing file size.
//
// Text measurement and kerning are available for both standard and
// embedded fonts through the [TextMeasurer] interface and the Kern
// methods.
package font

import "github.com/carlos7ags/folio/core"

// Standard represents one of the 14 standard PDF fonts that every
// conforming viewer must support (ISO 32000 §9.6.2.2).
// These fonts require no embedding — only a reference by name.
type Standard struct {
	name string // PDF BaseFont name (e.g. "Helvetica")
}

// Name returns the PDF BaseFont name.
func (f *Standard) Name() string {
	return f.name
}

// Dict returns the PDF font dictionary for this standard font.
//
//	<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>
func (f *Standard) Dict() *core.PdfDictionary {
	d := core.NewPdfDictionary()
	d.Set("Type", core.NewPdfName("Font"))
	d.Set("Subtype", core.NewPdfName("Type1"))
	d.Set("BaseFont", core.NewPdfName(f.name))
	d.Set("Encoding", core.NewPdfName("WinAnsiEncoding"))
	return d
}

// The 14 standard PDF fonts.
var (
	Helvetica            = &Standard{"Helvetica"}
	HelveticaBold        = &Standard{"Helvetica-Bold"}
	HelveticaOblique     = &Standard{"Helvetica-Oblique"}
	HelveticaBoldOblique = &Standard{"Helvetica-BoldOblique"}

	TimesRoman      = &Standard{"Times-Roman"}
	TimesBold       = &Standard{"Times-Bold"}
	TimesItalic     = &Standard{"Times-Italic"}
	TimesBoldItalic = &Standard{"Times-BoldItalic"}

	Courier            = &Standard{"Courier"}
	CourierBold        = &Standard{"Courier-Bold"}
	CourierOblique     = &Standard{"Courier-Oblique"}
	CourierBoldOblique = &Standard{"Courier-BoldOblique"}

	Symbol       = &Standard{"Symbol"}
	ZapfDingbats = &Standard{"ZapfDingbats"}
)

// CanEncodeWinAnsi reports whether all characters in s can be encoded
// in WinAnsiEncoding without loss. Returns false if any character would
// be replaced with '?' during encoding.
func CanEncodeWinAnsi(s string) bool {
	for _, r := range s {
		if r >= 128 {
			if _, ok := unicodeToWinAnsi[r]; !ok {
				return false
			}
		}
	}
	return true
}

// WinAnsiEncode converts a Unicode string to a byte string using
// WinAnsiEncoding (Windows-1252). Characters not in the encoding
// are replaced with a fallback (e.g. '?' or a close equivalent).
func WinAnsiEncode(s string) string {
	var buf []byte
	for _, r := range s {
		if b, ok := unicodeToWinAnsi[r]; ok {
			buf = append(buf, b)
		} else if r < 128 {
			buf = append(buf, byte(r))
		} else {
			buf = append(buf, '?')
		}
	}
	return string(buf)
}

// unicodeToWinAnsi maps Unicode code points (above 127) to their
// WinAnsiEncoding (Windows-1252) byte values.
var unicodeToWinAnsi = map[rune]byte{
	'\u20AC': 128, // €
	'\u201A': 130, // ‚
	'\u0192': 131, // ƒ
	'\u201E': 132, // „
	'\u2026': 133, // …
	'\u2020': 134, // †
	'\u2021': 135, // ‡
	'\u02C6': 136, // ˆ
	'\u2030': 137, // ‰
	'\u0160': 138, // Š
	'\u2039': 139, // ‹
	'\u0152': 140, // Œ
	'\u017D': 142, // Ž
	'\u2018': 145, // '
	'\u2019': 146, // '
	'\u201C': 147, // "
	'\u201D': 148, // "
	'\u2022': 149, // •
	'\u2013': 150, // –
	'\u2014': 151, // —
	'\u02DC': 152, // ˜
	'\u2122': 153, // ™
	'\u0161': 154, // š
	'\u203A': 155, // ›
	'\u0153': 156, // œ
	'\u017E': 158, // ž
	'\u0178': 159, // Ÿ
	'\u00A0': 160, // non-breaking space
	'\u00A1': 161, // ¡
	'\u00A2': 162, // ¢
	'\u00A3': 163, // £
	'\u00A4': 164, // ¤
	'\u00A5': 165, // ¥
	'\u00A6': 166, // ¦
	'\u00A7': 167, // §
	'\u00A8': 168, // ¨
	'\u00A9': 169, // ©
	'\u00AA': 170, // ª
	'\u00AB': 171, // «
	'\u00AC': 172, // ¬
	'\u00AD': 173, // soft hyphen
	'\u00AE': 174, // ®
	'\u00AF': 175, // ¯
	'\u00B0': 176, // °
	'\u00B1': 177, // ±
	'\u00B2': 178, // ²
	'\u00B3': 179, // ³
	'\u00B4': 180, // ´
	'\u00B5': 181, // µ
	'\u00B6': 182, // ¶
	'\u00B7': 183, // ·
	'\u00B8': 184, // ¸
	'\u00B9': 185, // ¹
	'\u00BA': 186, // º
	'\u00BB': 187, // »
	'\u00BC': 188, // ¼
	'\u00BD': 189, // ½
	'\u00BE': 190, // ¾
	'\u00BF': 191, // ¿
	'\u00C0': 192, // À
	'\u00C1': 193, // Á
	'\u00C2': 194, // Â
	'\u00C3': 195, // Ã
	'\u00C4': 196, // Ä
	'\u00C5': 197, // Å
	'\u00C6': 198, // Æ
	'\u00C7': 199, // Ç
	'\u00C8': 200, // È
	'\u00C9': 201, // É
	'\u00CA': 202, // Ê
	'\u00CB': 203, // Ë
	'\u00CC': 204, // Ì
	'\u00CD': 205, // Í
	'\u00CE': 206, // Î
	'\u00CF': 207, // Ï
	'\u00D0': 208, // Ð
	'\u00D1': 209, // Ñ
	'\u00D2': 210, // Ò
	'\u00D3': 211, // Ó
	'\u00D4': 212, // Ô
	'\u00D5': 213, // Õ
	'\u00D6': 214, // Ö
	'\u00D7': 215, // ×
	'\u00D8': 216, // Ø
	'\u00D9': 217, // Ù
	'\u00DA': 218, // Ú
	'\u00DB': 219, // Û
	'\u00DC': 220, // Ü
	'\u00DD': 221, // Ý
	'\u00DE': 222, // Þ
	'\u00DF': 223, // ß
	'\u00E0': 224, // à
	'\u00E1': 225, // á
	'\u00E2': 226, // â
	'\u00E3': 227, // ã
	'\u00E4': 228, // ä
	'\u00E5': 229, // å
	'\u00E6': 230, // æ
	'\u00E7': 231, // ç
	'\u00E8': 232, // è
	'\u00E9': 233, // é
	'\u00EA': 234, // ê
	'\u00EB': 235, // ë
	'\u00EC': 236, // ì
	'\u00ED': 237, // í
	'\u00EE': 238, // î
	'\u00EF': 239, // ï
	'\u00F0': 240, // ð
	'\u00F1': 241, // ñ
	'\u00F2': 242, // ò
	'\u00F3': 243, // ó
	'\u00F4': 244, // ô
	'\u00F5': 245, // õ
	'\u00F6': 246, // ö
	'\u00F7': 247, // ÷
	'\u00F8': 248, // ø
	'\u00F9': 249, // ù
	'\u00FA': 250, // ú
	'\u00FB': 251, // û
	'\u00FC': 252, // ü
	'\u00FD': 253, // ý
	'\u00FE': 254, // þ
	'\u00FF': 255, // ÿ
	'\u2212': '-', // − (minus sign → hyphen-minus, closest ASCII match)
}

// WinAnsiDecode converts a WinAnsiEncoding byte string back to Unicode.
func WinAnsiDecode(s string) string {
	var buf []rune
	for i := 0; i < len(s); i++ {
		b := s[i]
		if r, ok := winAnsiToUnicode[b]; ok {
			buf = append(buf, r)
		} else {
			buf = append(buf, rune(b))
		}
	}
	return string(buf)
}

// winAnsiToUnicode maps WinAnsiEncoding bytes (128-159) to Unicode.
var winAnsiToUnicode = map[byte]rune{
	128: '\u20AC', 130: '\u201A', 131: '\u0192', 132: '\u201E',
	133: '\u2026', 134: '\u2020', 135: '\u2021', 136: '\u02C6',
	137: '\u2030', 138: '\u0160', 139: '\u2039', 140: '\u0152',
	142: '\u017D', 145: '\u2018', 146: '\u2019', 147: '\u201C',
	148: '\u201D', 149: '\u2022', 150: '\u2013', 151: '\u2014',
	152: '\u02DC', 153: '\u2122', 154: '\u0161', 155: '\u203A',
	156: '\u0153', 158: '\u017E', 159: '\u0178',
}

// IsStandardFont returns true if name is one of the 14 standard PDF fonts.
func IsStandardFont(name string) bool {
	_, ok := standardWidths[name]
	return ok
}

// StandardFontByteWidths returns a 256-element width array for the named
// standard font, indexed by WinAnsiEncoding byte value. Each entry is in
// units of 1/1000 of text space. Returns nil if name is not a standard font
// or has no width data (Symbol, ZapfDingbats).
//
// This is designed for readers that need to compute text width from raw
// content stream bytes without the font's /Widths array (which standard
// fonts omit from the PDF since viewers are required to know them).
func StandardFontByteWidths(name string) []int {
	runeWidths, ok := standardWidths[name]
	if !ok || runeWidths == nil {
		return nil
	}

	widths := make([]int, 256)
	defaultW := runeWidths[0]

	// Map each WinAnsi byte → Unicode rune → width.
	for b := 0; b < 256; b++ {
		// Determine the Unicode rune for this byte.
		var r rune
		if ur, ok := winAnsiToUnicode[byte(b)]; ok {
			r = ur
		} else {
			r = rune(b)
		}

		if w, ok := runeWidths[r]; ok {
			widths[b] = w
		} else {
			widths[b] = defaultW
		}
	}
	return widths
}

// StandardFonts returns all 14 standard fonts.
func StandardFonts() []*Standard {
	return []*Standard{
		Helvetica, HelveticaBold, HelveticaOblique, HelveticaBoldOblique,
		TimesRoman, TimesBold, TimesItalic, TimesBoldItalic,
		Courier, CourierBold, CourierOblique, CourierBoldOblique,
		Symbol, ZapfDingbats,
	}
}
