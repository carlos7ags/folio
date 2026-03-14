// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"io"
)

// PdfNull represents the PDF null object (ISO 32000 §7.3.9).
type PdfNull struct{}

// NewPdfNull creates a new PdfNull instance.
func NewPdfNull() *PdfNull {
	return &PdfNull{}
}

// Type returns ObjectTypeNull.
func (n *PdfNull) Type() ObjectType { return ObjectTypeNull }

// WriteTo serializes the null object as "null" to w.
func (n *PdfNull) WriteTo(w io.Writer) (int64, error) {
	written, err := fmt.Fprint(w, "null")
	return int64(written), err
}
