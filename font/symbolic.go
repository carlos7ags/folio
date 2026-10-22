// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

// symbolicEncoding is the built-in encoding of a symbolic standard font
// (Symbol or ZapfDingbats). These fonts are written without an /Encoding
// entry (ISO 32000-1 §9.6.6.1), so a viewer selects glyphs through the
// font's built-in encoding and text must be encoded with it rather than
// with WinAnsiEncoding: in Symbol, code 0x61 is α, not 'a'.
type symbolicEncoding struct {
	toRune   *[256]rune    // code → Unicode; 0 marks an unused code
	widths   *[256]int     // code → advance in 1/1000 text-space units
	fromRune map[rune]byte // Unicode → code, including aliases

	// replacement is the code emitted for a rune the font has no glyph
	// for: '?' when the font has a question mark (Symbol), otherwise a
	// space (ZapfDingbats), mirroring WinAnsiEncode's '?' substitution
	// without painting an unrelated dingbat.
	replacement byte
}

// newSymbolicEncoding builds the reverse map for a built-in encoding.
// aliases adds further Unicode code points that select an existing glyph;
// an alias never overrides a code point the table maps itself.
func newSymbolicEncoding(toRune *[256]rune, widths *[256]int, aliases map[rune]byte) *symbolicEncoding {
	e := &symbolicEncoding{
		toRune:   toRune,
		widths:   widths,
		fromRune: make(map[rune]byte, 256+len(aliases)),
	}
	for c, r := range toRune {
		if r != 0 {
			e.fromRune[r] = byte(c)
		}
	}
	for r, c := range aliases {
		if _, ok := e.fromRune[r]; !ok {
			e.fromRune[r] = c
		}
	}
	e.replacement = ' '
	if c, ok := e.fromRune['?']; ok {
		e.replacement = c
	}
	return e
}

// symbolBuiltin is the Symbol font's built-in encoding. The aliases cover
// the alternate Unicode values Adobe's Symbol mapping lists for the same
// glyphs (increment, ohm, micro, division slash, the private-use serif
// ®/©/™, the CJK and mathematical angle brackets), the middle dot the
// rune-keyed width table uses for dotmath, and the ASCII hyphen-minus and
// asterisk, whose Symbol glyphs are the minus sign and math asterisk.
var symbolBuiltin = newSymbolicEncoding(&symbolCodeToRune, &symbolCodeWidths, map[rune]byte{
	'\u00A0': 0x20, // no-break space → space
	'*':      0x2A, // asterisk → asteriskmath
	'-':      0x2D, // hyphen-minus → minus
	'\u2206': 0x44, // increment → Delta
	'\u2126': 0x57, // ohm sign → Omega
	'\u00B5': 0x6D, // micro sign → mu
	'\u2215': 0xA4, // division slash → fraction
	'\uF6DA': 0xD2, // registerserif (private use)
	'\uF6D9': 0xD3, // copyrightserif (private use)
	'\uF6DB': 0xD4, // trademarkserif (private use)
	'\u00B7': 0xD7, // middle dot → dotmath
	'\u3008': 0xE1, // left angle bracket → angleleft
	'\u27E8': 0xE1, // mathematical left angle bracket → angleleft
	'\u3009': 0xF1, // right angle bracket → angleright
	'\u27E9': 0xF1, // mathematical right angle bracket → angleright
})

// zapfDingbatsBuiltin is the ZapfDingbats font's built-in encoding.
var zapfDingbatsBuiltin = newSymbolicEncoding(&zapfDingbatsCodeToRune, &zapfDingbatsCodeWidths, map[rune]byte{
	'\u00A0': 0x20, // no-break space → space
})

// symbolicEncodingFor returns the built-in encoding of the named symbolic
// standard font, or nil for every other font.
func symbolicEncodingFor(name string) *symbolicEncoding {
	switch name {
	case "Symbol":
		return symbolBuiltin
	case "ZapfDingbats":
		return zapfDingbatsBuiltin
	}
	return nil
}

// symbolic returns f's built-in encoding when f is Symbol or ZapfDingbats,
// or nil when f uses WinAnsiEncoding (including a nil f).
func (f *Standard) symbolic() *symbolicEncoding {
	if f == nil {
		return nil
	}
	return symbolicEncodingFor(f.name)
}

// encodeRune returns the code for r, or the replacement code and false
// when the font has no glyph for r. A rune below 0x100 that names no
// glyph by its Unicode value but is a used code of the built-in encoding
// is passed through as that code, so callers that write the font's codes
// directly ("4" for the ZapfDingbats check mark, "a" for Symbol's α)
// keep drawing the glyph they always did.
func (e *symbolicEncoding) encodeRune(r rune) (byte, bool) {
	if c, ok := e.fromRune[r]; ok {
		return c, true
	}
	if r >= 0 && r < 0x100 && e.toRune[r] != 0 {
		return byte(r), true
	}
	return e.replacement, false
}

// runeWidth returns the advance of the glyph r encodes to, in 1/1000
// text-space units. An unencodable rune measures as the replacement
// glyph it is drawn with.
func (e *symbolicEncoding) runeWidth(r rune) int {
	c, _ := e.encodeRune(r)
	return e.widths[c]
}

// Encode converts a Unicode string to the byte string a content stream
// shows with this font. Symbol and ZapfDingbats use their built-in
// encodings, so "α" becomes 0x61 in Symbol and "✔" becomes 0x34 in
// ZapfDingbats; a raw built-in code is passed through unchanged (see
// encodeRune); any other character the font has no glyph for becomes '?' in
// Symbol and a space in ZapfDingbats (which has no question mark). Every
// other standard font uses [WinAnsiEncode].
func (f *Standard) Encode(s string) string {
	e := f.symbolic()
	if e == nil {
		return WinAnsiEncode(s)
	}
	buf := make([]byte, 0, len(s))
	for _, r := range s {
		c, _ := e.encodeRune(r)
		buf = append(buf, c)
	}
	return string(buf)
}

// CanEncode reports whether every character in s survives [Standard.Encode]
// without substitution. It is the font-aware counterpart of
// [CanEncodeWinAnsi], used to decide when text needs a fallback font.
func (f *Standard) CanEncode(s string) bool {
	e := f.symbolic()
	if e == nil {
		return CanEncodeWinAnsi(s)
	}
	for _, r := range s {
		if _, ok := e.encodeRune(r); !ok {
			return false
		}
	}
	return true
}

// CanEncodeRune reports whether r survives [Standard.Encode] without
// substitution. Used by the per-glyph font fallback splitter.
func (f *Standard) CanEncodeRune(r rune) bool {
	e := f.symbolic()
	if e == nil {
		return CanEncodeWinAnsiRune(r)
	}
	_, ok := e.encodeRune(r)
	return ok
}

// StandardFontBuiltinEncoding returns the code → Unicode table of the
// built-in encoding of the named symbolic standard font (Symbol or
// ZapfDingbats); unused codes map to 0. It returns nil for every other
// font, whose text is not encoded with a font-specific table. The result
// is a fresh copy the caller may modify.
//
// Readers use it to decode a symbolic standard font that has no /Encoding
// entry, or whose /Encoding dictionary has /Differences but no
// /BaseEncoding (ISO 32000-1 §9.6.6.1).
func StandardFontBuiltinEncoding(name string) []rune {
	e := symbolicEncodingFor(name)
	if e == nil {
		return nil
	}
	table := make([]rune, 256)
	copy(table, e.toRune[:])
	return table
}
