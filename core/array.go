// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"io"
	"iter"
)

// PdfArray represents a PDF array object (ISO 32000 §7.3.6).
// An array is a one-dimensional collection of objects.
type PdfArray struct {
	// Elements is the underlying slice of array elements.
	//
	// Deprecated: direct field access is retained for backward compatibility.
	// Prefer [PdfArray.All], [PdfArray.At], [PdfArray.Len], and [PdfArray.Add]
	// for new code so that internal representation changes remain invisible
	// to callers.
	Elements []PdfObject
}

// NewPdfArray creates a new PdfArray containing the given elements.
// It panics if any element is nil, matching the contract of [PdfArray.Add].
func NewPdfArray(elements ...PdfObject) *PdfArray {
	for i, e := range elements {
		if e == nil {
			panic(fmt.Sprintf("core.NewPdfArray: nil element at index %d", i))
		}
	}
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

// At returns the element at index i. It panics if i is out of range.
func (a *PdfArray) At(i int) PdfObject {
	return a.Elements[i]
}

// Set replaces the element at index i with obj. It panics if i is out of
// range or if obj is nil, matching the contract of [PdfArray.Add].
func (a *PdfArray) Set(i int, obj PdfObject) {
	if obj == nil {
		panic("core.PdfArray.Set: nil object")
	}
	a.Elements[i] = obj
}

// RemoveAt removes and returns the element at index i, shifting later
// elements down. It panics if i is out of range.
func (a *PdfArray) RemoveAt(i int) PdfObject {
	removed := a.Elements[i]
	a.Elements = append(a.Elements[:i], a.Elements[i+1:]...)
	return removed
}

// Replace discards all current elements and sets the array content to the
// given sequence. It panics if any element is nil.
func (a *PdfArray) Replace(elements ...PdfObject) {
	for i, e := range elements {
		if e == nil {
			panic(fmt.Sprintf("core.PdfArray.Replace: nil element at index %d", i))
		}
	}
	a.Elements = elements
}

// All returns an iterator over (index, element) pairs in insertion order.
// Using this iterator insulates callers from the underlying representation
// of [PdfArray.Elements].
func (a *PdfArray) All() iter.Seq2[int, PdfObject] {
	return func(yield func(int, PdfObject) bool) {
		for i, e := range a.Elements {
			if !yield(i, e) {
				return
			}
		}
	}
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
