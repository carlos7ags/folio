// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"io"
)

// PdfBoolean represents a PDF boolean value (ISO 32000 §7.3.2).
// The underlying value is immutable after construction; use [NewPdfBoolean]
// to create new instances and [PdfBoolean.Bool] to read them.
type PdfBoolean struct {
	value bool
}

// Shared singletons for the two possible PdfBoolean values.
var (
	pdfBooleanTrue  = &PdfBoolean{value: true}
	pdfBooleanFalse = &PdfBoolean{value: false}
)

// NewPdfBoolean returns the shared PdfBoolean instance for the given value.
// Because PdfBoolean is immutable, singletons are safe and avoid allocation.
func NewPdfBoolean(v bool) *PdfBoolean {
	if v {
		return pdfBooleanTrue
	}
	return pdfBooleanFalse
}

// Bool returns the underlying boolean value.
func (b *PdfBoolean) Bool() bool { return b.value }

// Type returns ObjectTypeBoolean.
func (b *PdfBoolean) Type() ObjectType { return ObjectTypeBoolean }

// WriteTo serializes the boolean as "true" or "false" to w.
func (b *PdfBoolean) WriteTo(w io.Writer) (int64, error) {
	s := "false"
	if b.value {
		s = "true"
	}
	n, err := fmt.Fprint(w, s)
	return int64(n), err
}
