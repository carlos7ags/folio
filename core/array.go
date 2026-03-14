// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"io"
)

// PdfArray represents a PDF array object (ISO 32000 §7.3.6).
// An array is a one-dimensional collection of objects.
type PdfArray struct {
	Elements []PdfObject
}

// NewPdfArray creates a new PdfArray containing the given elements.
func NewPdfArray(elements ...PdfObject) *PdfArray {
	return &PdfArray{Elements: elements}
}

// Type returns ObjectTypeArray.
func (a *PdfArray) Type() ObjectType { return ObjectTypeArray }

// Add appends an object to the array. Panics if obj is nil.
func (a *PdfArray) Add(obj PdfObject) {
	if obj == nil {
		panic("core.PdfArray.Add: nil object")
	}
	a.Elements = append(a.Elements, obj)
}

// Len returns the number of elements.
func (a *PdfArray) Len() int {
	return len(a.Elements)
}

// WriteTo serializes the array in PDF syntax to w.
func (a *PdfArray) WriteTo(w io.Writer) (int64, error) {
	cw := &countingWriter{w: w}

	if _, err := cw.WriteString("["); err != nil {
		return cw.n, err
	}

	for i, elem := range a.Elements {
		if i > 0 {
			if _, err := cw.WriteString(" "); err != nil {
				return cw.n, err
			}
		}
		if _, err := elem.WriteTo(cw); err != nil {
			return cw.n, err
		}
	}

	if _, err := cw.WriteString("]"); err != nil {
		return cw.n, err
	}

	return cw.n, nil
}
