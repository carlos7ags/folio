// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"io"
	"strings"
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
// Use [NewPdfLiteralString] or [NewPdfHexString] to construct, and
// [PdfString.Text] / [PdfString.IsHex] to read. The underlying fields
// are unexported; encryption mutates them in-place from within the
// core package.
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
// it is the decoded bytes interpreted as a string.
func (s *PdfString) Text() string { return s.value }

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
