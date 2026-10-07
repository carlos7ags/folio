// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
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
// returned as-is. See [DecodeTextString].
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
// shows as "Ã³". This constructor keeps pure-ASCII text as a plain
// literal string, which is valid PDFDocEncoding and stays readable in
// the file, and encodes anything else as UTF-16BE with the BOM, written
// in hexadecimal notation so the embedded NUL and delimiter bytes need
// no escaping.
//
// Use [NewPdfLiteralString] for byte strings that are not text strings,
// such as /URI, dates, named destinations and file specifications (/F).
func NewPdfTextString(v string) *PdfString {
	if isASCII(v) {
		return NewPdfLiteralString(v)
	}
	return NewPdfHexString(EncodeTextStringUTF16BE(v))
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

// EncodeTextStringUTF16BE returns the UTF-16BE byte representation of s
// prefixed with the UTF-16 byte-order mark (\xFE\xFF). The result is
// suitable as the value of a PDF text string per ISO 32000-1 §7.9.2.2.
// Code points outside the Basic Multilingual Plane are emitted as a
// UTF-16 surrogate pair (high surrogate + low surrogate).
func EncodeTextStringUTF16BE(s string) string {
	var b strings.Builder
	b.Grow(2 + 2*len(s))
	b.WriteByte(0xFE)
	b.WriteByte(0xFF)
	for _, r := range s {
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
// U+FFFD). A value starting with the UTF-8 BOM EF BB BF (allowed by
// ISO 32000-2) has the BOM stripped. Anything else is PDFDocEncoding and
// is returned unchanged; its ASCII range is identical to UTF-8.
func DecodeTextString(raw string) string {
	switch {
	case len(raw) >= 2 && raw[0] == 0xFE && raw[1] == 0xFF:
		body := raw[2:]
		units := make([]uint16, 0, len(body)/2)
		for i := 0; i+1 < len(body); i += 2 {
			units = append(units, uint16(body[i])<<8|uint16(body[i+1]))
		}
		return string(utf16.Decode(units))
	case len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF:
		return raw[3:]
	default:
		return raw
	}
}
