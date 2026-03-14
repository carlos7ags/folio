// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"strconv"
	"strings"
)

// Encoding maps byte values (0-255) to Unicode runes for simple (non-CID) fonts.
type Encoding struct {
	table [256]rune
}

// Decode converts raw bytes through the encoding table to a Unicode string.
func (e *Encoding) Decode(raw []byte) string {
	runes := make([]rune, 0, len(raw))
	for _, b := range raw {
		r := e.table[b]
		if r != 0 {
			runes = append(runes, r)
		}
	}
	return string(runes)
}

// WinAnsiEncoding is the Windows-1252 encoding used by most PDF simple fonts.
var WinAnsiEncoding = makeWinAnsiEncoding()

// MacRomanEncoding is the Mac OS Roman encoding.
var MacRomanEncoding = makeMacRomanEncoding()

// StandardEncoding is Adobe's standard encoding for Type1 fonts.
var StandardEncoding = makeStandardEncoding()

func makeWinAnsiEncoding() *Encoding {
	var e Encoding
	// ASCII range maps 1:1.
	for i := range 128 {
		e.table[i] = rune(i)
	}
	// Windows-1252 upper half.
	cp1252 := map[byte]rune{
		128: 0x20AC, 130: 0x201A, 131: 0x0192, 132: 0x201E, 133: 0x2026,
		134: 0x2020, 135: 0x2021, 136: 0x02C6, 137: 0x2030, 138: 0x0160,
		139: 0x2039, 140: 0x0152, 142: 0x017D, 145: 0x2018, 146: 0x2019,
		147: 0x201C, 148: 0x201D, 149: 0x2022, 150: 0x2013, 151: 0x2014,
		152: 0x02DC, 153: 0x2122, 154: 0x0161, 155: 0x203A, 156: 0x0153,
		158: 0x017E, 159: 0x0178,
	}
	for i := 128; i < 256; i++ {
		if r, ok := cp1252[byte(i)]; ok {
			e.table[i] = r
		} else {
			e.table[i] = rune(i) // Latin-1 supplement
		}
	}
	return &e
}

func makeMacRomanEncoding() *Encoding {
	var e Encoding
	for i := range 128 {
		e.table[i] = rune(i)
	}
	// Mac Roman upper half mapping.
	macRomanHigh := [128]rune{
		0x00C4, 0x00C5, 0x00C7, 0x00C9, 0x00D1, 0x00D6, 0x00DC, 0x00E1, // 80-87
		0x00E0, 0x00E2, 0x00E4, 0x00E3, 0x00E5, 0x00E7, 0x00E9, 0x00E8, // 88-8F
		0x00EA, 0x00EB, 0x00ED, 0x00EC, 0x00EE, 0x00EF, 0x00F1, 0x00F3, // 90-97
		0x00F2, 0x00F4, 0x00F6, 0x00F5, 0x00FA, 0x00F9, 0x00FB, 0x00FC, // 98-9F
		0x2020, 0x00B0, 0x00A2, 0x00A3, 0x00A7, 0x2022, 0x00B6, 0x00DF, // A0-A7
		0x00AE, 0x00A9, 0x2122, 0x00B4, 0x00A8, 0x2260, 0x00C6, 0x00D8, // A8-AF
		0x221E, 0x00B1, 0x2264, 0x2265, 0x00A5, 0x00B5, 0x2202, 0x2211, // B0-B7
		0x220F, 0x03C0, 0x222B, 0x00AA, 0x00BA, 0x2126, 0x00E6, 0x00F8, // B8-BF
		0x00BF, 0x00A1, 0x00AC, 0x221A, 0x0192, 0x2248, 0x2206, 0x00AB, // C0-C7
		0x00BB, 0x2026, 0x00A0, 0x00C0, 0x00C3, 0x00D5, 0x0152, 0x0153, // C8-CF
		0x2013, 0x2014, 0x201C, 0x201D, 0x2018, 0x2019, 0x00F7, 0x25CA, // D0-D7
		0x00FF, 0x0178, 0x2044, 0x20AC, 0x2039, 0x203A, 0xFB01, 0xFB02, // D8-DF
		0x2021, 0x00B7, 0x201A, 0x201E, 0x2030, 0x00C2, 0x00CA, 0x00C1, // E0-E7
		0x00CB, 0x00C8, 0x00CD, 0x00CE, 0x00CF, 0x00CC, 0x00D3, 0x00D4, // E8-EF
		0xF8FF, 0x00D2, 0x00DA, 0x00DB, 0x00D9, 0x0131, 0x02C6, 0x02DC, // F0-F7
		0x00AF, 0x02D8, 0x02D9, 0x02DA, 0x00B8, 0x02DD, 0x02DB, 0x02C7, // F8-FF
	}
	for i, r := range macRomanHigh {
		e.table[128+i] = r
	}
	return &e
}

func makeStandardEncoding() *Encoding {
	var e Encoding
	// ASCII printable range is mostly the same.
	for i := 0x20; i < 0x7F; i++ {
		e.table[i] = rune(i)
	}
	// Key differences from ASCII in StandardEncoding.
	overrides := map[byte]rune{
		0x27: 0x2019, // quoteright
		0x60: 0x2018, // quoteleft
		0xA1: 0x00A1, // exclamdown
		0xA2: 0x00A2, // cent
		0xA3: 0x00A3, // sterling
		0xA4: 0x2044, // fraction
		0xA5: 0x00A5, // yen
		0xA6: 0x0192, // florin
		0xA7: 0x00A7, // section
		0xA8: 0x00A4, // currency
		0xAB: 0x00AB, // guillemotleft
		0xAC: 0x2039, // guilsinglleft
		0xAD: 0x203A, // guilsinglright
		0xAE: 0xFB01, // fi
		0xAF: 0xFB02, // fl
		0xB1: 0x2013, // endash
		0xB2: 0x2020, // dagger
		0xB3: 0x2021, // daggerdbl
		0xB4: 0x00B7, // periodcentered
		0xB7: 0x2022, // bullet
		0xB8: 0x201A, // quotesinglbase
		0xB9: 0x201E, // quotedblbase
		0xBA: 0x201D, // quotedblright
		0xBB: 0x00BB, // guillemotright
		0xBC: 0x2026, // ellipsis
		0xBD: 0x2030, // perthousand
		0xC1: 0x0060, // grave
		0xC2: 0x00B4, // acute
		0xC3: 0x02C6, // circumflex
		0xC4: 0x02DC, // tilde
		0xC5: 0x00AF, // macron
		0xC6: 0x02D8, // breve
		0xC7: 0x02D9, // dotaccent
		0xC8: 0x00A8, // dieresis
		0xCA: 0x02DA, // ring
		0xCB: 0x00B8, // cedilla
		0xCD: 0x02DD, // hungarumlaut
		0xCE: 0x02DB, // ogonek
		0xCF: 0x02C7, // caron
		0xD0: 0x2014, // emdash
		0xE1: 0x00C6, // AE
		0xE3: 0x00AA, // ordfeminine
		0xE8: 0x0141, // Lslash
		0xE9: 0x00D8, // Oslash
		0xEA: 0x0152, // OE
		0xEB: 0x00BA, // ordmasculine
		0xF1: 0x00E6, // ae
		0xF5: 0x0131, // dotlessi
		0xF8: 0x0142, // lslash
		0xF9: 0x00F8, // oslash
		0xFA: 0x0153, // oe
		0xFB: 0x00DF, // germandbls
	}
	for b, r := range overrides {
		e.table[b] = r
	}
	return &e
}

// glyphNameToRune maps common Adobe glyph names to Unicode code points.
// Used for /Differences arrays in font encoding dictionaries.
var glyphNameToRune = map[string]rune{
	"space": ' ', "exclam": '!', "quotedbl": '"', "numbersign": '#',
	"dollar": '$', "percent": '%', "ampersand": '&', "quotesingle": '\'',
	"parenleft": '(', "parenright": ')', "asterisk": '*', "plus": '+',
	"comma": ',', "hyphen": '-', "period": '.', "slash": '/',
	"zero": '0', "one": '1', "two": '2', "three": '3',
	"four": '4', "five": '5', "six": '6', "seven": '7',
	"eight": '8', "nine": '9', "colon": ':', "semicolon": ';',
	"less": '<', "equal": '=', "greater": '>', "question": '?',
	"at": '@',
	"A":  'A', "B": 'B', "C": 'C', "D": 'D', "E": 'E', "F": 'F',
	"G": 'G', "H": 'H', "I": 'I', "J": 'J', "K": 'K', "L": 'L',
	"M": 'M', "N": 'N', "O": 'O', "P": 'P', "Q": 'Q', "R": 'R',
	"S": 'S', "T": 'T', "U": 'U', "V": 'V', "W": 'W', "X": 'X',
	"Y": 'Y', "Z": 'Z',
	"bracketleft": '[', "backslash": '\\', "bracketright": ']',
	"asciicircum": '^', "underscore": '_', "grave": '`',
	"a": 'a', "b": 'b', "c": 'c', "d": 'd', "e": 'e', "f": 'f',
	"g": 'g', "h": 'h', "i": 'i', "j": 'j', "k": 'k', "l": 'l',
	"m": 'm', "n": 'n', "o": 'o', "p": 'p', "q": 'q', "r": 'r',
	"s": 's', "t": 't', "u": 'u', "v": 'v', "w": 'w', "x": 'x',
	"y": 'y', "z": 'z',
	"braceleft": '{', "bar": '|', "braceright": '}', "asciitilde": '~',
	// Accented / special
	"Agrave": 0xC0, "Aacute": 0xC1, "Acircumflex": 0xC2, "Atilde": 0xC3,
	"Adieresis": 0xC4, "Aring": 0xC5, "AE": 0xC6, "Ccedilla": 0xC7,
	"Egrave": 0xC8, "Eacute": 0xC9, "Ecircumflex": 0xCA, "Edieresis": 0xCB,
	"Igrave": 0xCC, "Iacute": 0xCD, "Icircumflex": 0xCE, "Idieresis": 0xCF,
	"Eth": 0xD0, "Ntilde": 0xD1, "Ograve": 0xD2, "Oacute": 0xD3,
	"Ocircumflex": 0xD4, "Otilde": 0xD5, "Odieresis": 0xD6, "multiply": 0xD7,
	"Oslash": 0xD8, "Ugrave": 0xD9, "Uacute": 0xDA, "Ucircumflex": 0xDB,
	"Udieresis": 0xDC, "Yacute": 0xDD, "Thorn": 0xDE, "germandbls": 0xDF,
	"agrave": 0xE0, "aacute": 0xE1, "acircumflex": 0xE2, "atilde": 0xE3,
	"adieresis": 0xE4, "aring": 0xE5, "ae": 0xE6, "ccedilla": 0xE7,
	"egrave": 0xE8, "eacute": 0xE9, "ecircumflex": 0xEA, "edieresis": 0xEB,
	"igrave": 0xEC, "iacute": 0xED, "icircumflex": 0xEE, "idieresis": 0xEF,
	"eth": 0xF0, "ntilde": 0xF1, "ograve": 0xF2, "oacute": 0xF3,
	"ocircumflex": 0xF4, "otilde": 0xF5, "odieresis": 0xF6, "divide": 0xF7,
	"oslash": 0xF8, "ugrave": 0xF9, "uacute": 0xFA, "ucircumflex": 0xFB,
	"udieresis": 0xFC, "yacute": 0xFD, "thorn": 0xFE, "ydieresis": 0xFF,
	// Typographic
	"endash": 0x2013, "emdash": 0x2014, "bullet": 0x2022,
	"quotedblleft": 0x201C, "quotedblright": 0x201D,
	"quoteleft": 0x2018, "quoteright": 0x2019,
	"quotesinglbase": 0x201A, "quotedblbase": 0x201E,
	"dagger": 0x2020, "daggerdbl": 0x2021,
	"ellipsis": 0x2026, "perthousand": 0x2030,
	"guillemotleft": 0x00AB, "guillemotright": 0x00BB,
	"guilsinglleft": 0x2039, "guilsinglright": 0x203A,
	"trademark": 0x2122, "copyright": 0x00A9, "registered": 0x00AE,
	"degree": 0x00B0, "plusminus": 0x00B1, "minus": 0x2212,
	"fraction": 0x2044, "fi": 0xFB01, "fl": 0xFB02,
	"OE": 0x0152, "oe": 0x0153, "Lslash": 0x0141, "lslash": 0x0142,
	"Scaron": 0x0160, "scaron": 0x0161, "Zcaron": 0x017D, "zcaron": 0x017E,
	"Ydieresis": 0x0178, "florin": 0x0192, "circumflex": 0x02C6,
	"tilde": 0x02DC, "dotlessi": 0x0131, "ring": 0x02DA,
	"breve": 0x02D8, "dotaccent": 0x02D9, "hungarumlaut": 0x02DD,
	"ogonek": 0x02DB, "caron": 0x02C7, "cedilla": 0x00B8,
	"macron": 0x00AF, "acute": 0x00B4, "dieresis": 0x00A8,
	"section": 0x00A7, "paragraph": 0x00B6,
	"periodcentered": 0x00B7, "cent": 0x00A2, "sterling": 0x00A3,
	"yen": 0x00A5, "currency": 0x00A4, "brokenbar": 0x00A6,
	"logicalnot": 0x00AC, "mu": 0x00B5, "Euro": 0x20AC,
	"exclamdown": 0x00A1, "questiondown": 0x00BF,
	"ordfeminine": 0x00AA, "ordmasculine": 0x00BA,
	"onehalf": 0x00BD, "onequarter": 0x00BC, "threequarters": 0x00BE,
	"onesuperior": 0x00B9, "twosuperior": 0x00B2, "threesuperior": 0x00B3,
}

// GlyphToRune converts an Adobe glyph name to a Unicode rune.
// Returns 0 if unknown.
func GlyphToRune(name string) rune {
	if r, ok := glyphNameToRune[name]; ok {
		return r
	}
	// Try "uniXXXX" convention.
	if len(name) == 7 && strings.HasPrefix(name, "uni") {
		if code, err := strconv.ParseUint(name[3:], 16, 32); err == nil {
			return rune(code)
		}
	}
	return 0
}
