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
type PdfString struct {
	Value    string
	Encoding StringEncoding
}

// NewPdfLiteralString creates a literal string: (value).
func NewPdfLiteralString(v string) *PdfString {
	return &PdfString{Value: v, Encoding: StringLiteral}
}

// NewPdfHexString creates a hexadecimal string: <hex>.
func NewPdfHexString(v string) *PdfString {
	return &PdfString{Value: v, Encoding: StringHexadecimal}
}

// Type returns ObjectTypeString.
func (s *PdfString) Type() ObjectType { return ObjectTypeString }

// WriteTo serializes the string in literal or hexadecimal notation to w.
func (s *PdfString) WriteTo(w io.Writer) (int64, error) {
	var out string
	switch s.Encoding {
	case StringHexadecimal:
		out = "<" + fmt.Sprintf("%X", []byte(s.Value)) + ">"
	default:
		out = "(" + EscapeLiteralString(s.Value) + ")"
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
	for i := 0; i < len(s); i++ {
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
