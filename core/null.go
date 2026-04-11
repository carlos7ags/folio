// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"io"
)

// PdfNull represents the PDF null object (ISO 32000 §7.3.9).
// PdfNull is stateless; all instances are equivalent.
type PdfNull struct{}

// pdfNullSingleton is the single shared PdfNull instance returned by NewPdfNull.
var pdfNullSingleton = &PdfNull{}

// NewPdfNull returns the shared PdfNull instance. Because PdfNull has no
// fields, a singleton is safe and avoids allocating on every call.
func NewPdfNull() *PdfNull {
	return pdfNullSingleton
}

// Type returns ObjectTypeNull.
func (n *PdfNull) Type() ObjectType { return ObjectTypeNull }

// WriteTo serializes the null object as "null" to w.
func (n *PdfNull) WriteTo(w io.Writer) (int64, error) {
	written, err := fmt.Fprint(w, "null")
	return int64(written), err
}
