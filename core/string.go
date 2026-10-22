// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// StringEncoding controls how a PdfString is serialized.
type StringEncoding int

const (
	StringLiteral     StringEncoding = iota // (Hello World)
	StringHexadecimal                       // <48656C6C6F>
)

// PdfString represents a PDF string object (ISO 32000 §7.3.4).
// PDF supports two notations: literal strings in parentheses and
// hexadecimal strings in angle brackets.
//
// Use [NewPdfLiteralString] or [NewPdfHexString] to construct a byte
// string, [NewPdfTextString] for a human-readable *text string* (a title,
// author, bookmark label, annotation contents, ...), and
// [PdfString.Text] / [PdfString.TextString] / [PdfString.IsHex] to read.
// The underlying fields are unexported; encryption mutates them in-place
// from within the core package.
type PdfString struct {
	value    string
	encoding StringEncoding
}

// NewPdfLiteralString creates a literal string: (value).
func NewPdfLiteralString(v string) *PdfString {
	return &PdfString{value: v, encoding: StringLiteral}
}

// NewPdfHexString creates a hexadecimal string: <hex>.
func NewPdfHexString(v string) *PdfString {
	return &PdfString{value: v, encoding: StringHexadecimal}
}

// Type returns ObjectTypeString.
func (s *PdfString) Type() ObjectType { return ObjectTypeString }

// Text returns the raw string value, without PDF escaping.
// For literal strings this is the unescaped content; for hex strings
// it is the decoded bytes interpreted as a string. For a text string
// written by [NewPdfTextString] this may be UTF-16BE bytes; use
// [PdfString.TextString] to get the decoded Unicode text.
func (s *PdfString) Text() string { return s.value }

// TextString decodes the value as a PDF text string (ISO 32000-1
// §7.9.2.2): a UTF-16BE string with the byte-order mark FE FF is decoded
// to UTF-8, a UTF-8 BOM (ISO 32000-2) is stripped, and anything else is
// decoded as PDFDocEncoding. See [DecodeTextString].
func (s *PdfString) TextString() string { return DecodeTextString(s.value) }

// IsHex reports whether the string will be serialized in hexadecimal
// notation (<hex>) rather than literal notation ((text)).
func (s *PdfString) IsHex() bool { return s.encoding == StringHexadecimal }

// WriteTo serializes the string in literal or hexadecimal notation to w.
func (s *PdfString) WriteTo(w io.Writer) (int64, error) {
	var out string
	switch s.encoding {
	case StringHexadecimal:
		out = "<" + fmt.Sprintf("%X", []byte(s.value)) + ">"
	default:
		out = "(" + EscapeLiteralString(s.value) + ")"
	}
	n, err := fmt.Fprint(w, out)
	return int64(n), err
}

// EscapeLiteralString escapes special characters inside a PDF literal string.
// Per ISO 32000 §7.3.4.2, the characters \, (, and ) must be escaped.
// Control characters (0x00–0x1F except \n, \r, \t) are escaped as octal.
func EscapeLiteralString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := range len(s) {
		c := s[i]
		switch {
		case c == '\\':
			b.WriteString(`\\`)
		case c == '(':
			b.WriteString(`\(`)
		case c == ')':
			b.WriteString(`\)`)
		case c == '\n':
			b.WriteString(`\n`)
		case c == '\r':
			b.WriteString(`\r`)
		case c == '\t':
			b.WriteString(`\t`)
		case c <= 0x1F:
			// Control characters: escape as octal.
			fmt.Fprintf(&b, `\%03o`, c)
		default:
			// Write raw byte — preserves WinAnsiEncoding values (128-255).
			b.WriteByte(c)
		}
	}
	return b.String()
}

// NewPdfTextString creates a PDF *text string* (ISO 32000-1 §7.9.2.2)
// holding human-readable Unicode text: an outline title, an Info entry,
// annotation contents, alt text, a form field name, and so on.
//
// A text string must be either PDFDocEncoding or UTF-16BE prefixed with
// the byte-order mark FE FF. Writing a Go string's UTF-8 bytes directly
// is wrong: a reader decodes them as PDFDocEncoding and "ó" (C3 B3)
// shows as "Ã³". This constructor keeps ASCII text as a plain literal
// string (see [IsPDFDocASCII]), which is valid PDFDocEncoding and stays
// readable in the file, and encodes anything else as UTF-16BE with the BOM, written
// in hexadecimal notation so the embedded NUL and delimiter bytes need
// no escaping.
//
// Use [NewPdfLiteralString] for byte strings that are not text strings,
// such as /URI, dates, named destinations and file specifications (/F).
func NewPdfTextString(v string) *PdfString {
	if IsPDFDocASCII(v) {
		return NewPdfLiteralString(v)
	}
	return NewPdfHexString(EncodeTextStringUTF16BE(v))
}

// IsPDFDocASCII reports whether s can be written as a text string
// unchanged: every byte is 7-bit ASCII that means the same character in
// PDFDocEncoding. Bytes 0x18–0x1F (which PDFDocEncoding assigns to
// spacing accents such as ˘ and ˜) and 0x7F (undefined) are excluded, so
// text containing them is encoded as UTF-16BE instead.
func IsPDFDocASCII(s string) bool {
	for i := range len(s) {
		c := s[i]
		if c >= 0x7F || (c >= 0x18 && c <= 0x1F) {
			return false
		}
	}
	return true
}

// EncodeTextStringUTF16BE returns the UTF-16BE byte representation of s
// prefixed with the UTF-16 byte-order mark (\xFE\xFF). The result is
// suitable as the value of a PDF text string per ISO 32000-1 §7.9.2.2.
// Code points outside the Basic Multilingual Plane are emitted as a
// UTF-16 surrogate pair (high surrogate + low surrogate). U+001B is
// dropped: inside a UTF-16 text string it marks a language escape, so a
// reader would hide the text that follows it.
func EncodeTextStringUTF16BE(s string) string {
	var b strings.Builder
	b.Grow(2 + 2*len(s))
	b.WriteByte(0xFE)
	b.WriteByte(0xFF)
	for _, r := range s {
		if r == 0x1B {
			continue
		}
		if r <= 0xFFFF {
			b.WriteByte(byte(r >> 8))
			b.WriteByte(byte(r))
			continue
		}
		// Astral plane: encode as a surrogate pair (UTF-16, RFC 2781).
		v := uint32(r) - 0x10000
		hi := 0xD800 + (v >> 10)
		lo := 0xDC00 + (v & 0x3FF)
		b.WriteByte(byte(hi >> 8))
		b.WriteByte(byte(hi))
		b.WriteByte(byte(lo >> 8))
		b.WriteByte(byte(lo))
	}
	return b.String()
}

// DecodeTextString decodes the raw bytes of a PDF text string
// (ISO 32000-1 §7.9.2.2) to Go UTF-8 text.
//
// A value starting with the byte-order mark FE FF is UTF-16BE and is
// decoded (an odd trailing byte is dropped, unpaired surrogates become
// U+FFFD, and language escape sequences ESC … ESC are removed). A value
// starting with the UTF-8 BOM EF BB BF (allowed by ISO 32000-2) has the
// BOM stripped. Anything else is PDFDocEncoding and is mapped through
// Annex D.2, except that a value which is valid UTF-8 and contains a
// non-ASCII byte is returned unchanged: older folio releases wrote text
// strings as raw UTF-8, and those files stay readable.
func DecodeTextString(raw string) string {
	switch {
	case len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF:
		body := raw[2:]
		units := make([]uint16, 0, len(body)/2)
		inEscape := false
		for i := 0; i+1 < len(body); i += 2 {
			u := uint16(body[i])<<8 | uint16(body[i+1])
			if u == 0x1B {
				// §7.9.2.2: ESC <language> [<country>] ESC marks a
				// language change and is not part of the text.
				inEscape = !inEscape
				continue
			}
			if !inEscape {
				units = append(units, u)
			}
		}
		return string(utf16.Decode(units))
	case len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF:
		return raw[3:]
	case IsPDFDocASCII(raw):
		return raw
	case !isASCII(raw) && utf8.ValidString(raw):
		return raw
	default:
		return decodePDFDoc(raw)
	}
}

// isASCII reports whether every byte of s is 7-bit ASCII.
func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// decodePDFDoc maps each byte of s from PDFDocEncoding to its Unicode
// character.
func decodePDFDoc(s string) string {
	var b strings.Builder
	b.Grow(len(s) + len(s)/2)
	for i := range len(s) {
		b.WriteRune(pdfDocRune(s[i]))
	}
	return b.String()
}

// pdfDocRune returns the Unicode character for PDFDocEncoding byte c
// (ISO 32000-1 Annex D.2). PDFDocEncoding matches ISO Latin-1 except at
// 0x18–0x1F, 0x7F–0xA0 and 0xAD; the undefined codes 0x7F, 0x9F and
// 0xAD map to U+FFFD.
func pdfDocRune(c byte) rune {
	switch {
	case c >= 0x18 && c <= 0x1F:
		return pdfDocAccents[c-0x18]
	case c >= 0x80 && c <= 0x9F:
		return pdfDocHigh[c-0x80]
	case c == 0x7F, c == 0xAD:
		return utf8.RuneError
	case c == 0xA0:
		return '\u20AC' // €
	default:
		return rune(c)
	}
}

// pdfDocAccents holds PDFDocEncoding 0x18–0x1F.
var pdfDocAccents = [8]rune{
	'\u02D8', '\u02C7', '\u02C6', '\u02D9', '\u02DD', '\u02DB', '\u02DA', '\u02DC', // ˘ ˇ ˆ ˙ ˝ ˛ ˚ ˜
}

// pdfDocHigh holds PDFDocEncoding 0x80–0x9F; 0x9F is undefined.
var pdfDocHigh = [32]rune{
	'\u2022', '\u2020', '\u2021', '\u2026', '\u2014', '\u2013', '\u0192', '\u2044', // • † ‡ … — – ƒ ⁄
	'\u2039', '\u203A', '\u2212', '\u2030', '\u201E', '\u201C', '\u201D', '\u2018', // ‹ › − ‰ „ “ ” ‘
	'\u2019', '\u201A', '\u2122', '\uFB01', '\uFB02', '\u0141', '\u0152', '\u0160', // ’ ‚ ™ ﬁ ﬂ Ł Œ Š
	'\u0178', '\u017D', '\u0131', '\u0142', '\u0153', '\u0161', '\u017E', utf8.RuneError, // Ÿ Ž ı ł œ š ž
}
