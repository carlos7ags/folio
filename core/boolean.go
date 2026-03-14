// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"io"
)

// PdfBoolean represents a PDF boolean value (ISO 32000 §7.3.2).
type PdfBoolean struct {
	Value bool
}

// NewPdfBoolean creates a new PdfBoolean with the given value.
func NewPdfBoolean(v bool) *PdfBoolean {
	return &PdfBoolean{Value: v}
}

// Type returns ObjectTypeBoolean.
func (b *PdfBoolean) Type() ObjectType { return ObjectTypeBoolean }

// WriteTo serializes the boolean as "true" or "false" to w.
func (b *PdfBoolean) WriteTo(w io.Writer) (int64, error) {
	s := "false"
	if b.Value {
		s = "true"
	}
	n, err := fmt.Fprint(w, s)
	return int64(n), err
}
