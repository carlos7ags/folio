// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"io"
	"strings"
)

// PdfName represents a PDF name object (ISO 32000 §7.3.5).
// Names are written with a leading solidus: /Type, /Pages, etc.
type PdfName struct {
	Value string // the name without the leading /
}

// NewPdfName creates a new PdfName with the given value (without the leading solidus).
func NewPdfName(v string) *PdfName {
	return &PdfName{Value: v}
}

// Type returns ObjectTypeName.
func (n *PdfName) Type() ObjectType { return ObjectTypeName }

// WriteTo serializes the name with a leading solidus to w.
func (n *PdfName) WriteTo(w io.Writer) (int64, error) {
	written, err := fmt.Fprint(w, "/"+encodeName(n.Value))
	return int64(written), err
}

// encodeName encodes a PDF name according to ISO 32000 §7.3.5.
// Characters outside the regular printable ASCII range (0x21–0x7E)
// and the special character '#' are encoded as #XX where XX is the
// two-digit hex code.
func encodeName(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := range len(s) {
		c := s[i]
		// Characters that must be encoded: outside 0x21-0x7E range,
		// '#' itself, and PDF delimiter characters
		if c < 0x21 || c > 0x7E || c == '#' || isDelimiter(c) {
			fmt.Fprintf(&b, "#%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// isDelimiter reports whether c is a PDF delimiter character (§7.2.2).
func isDelimiter(c byte) bool {
	switch c {
	case '(', ')', '<', '>', '[', ']', '{', '}', '/', '%':
		return true
	}
	return false
}
