// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"io"
)

// PdfIndirectReference represents a PDF indirect reference (ISO 32000 §7.3.10).
// Written as "objNum genNum R" (e.g., "1 0 R").
// Generation numbers are almost always 0 in modern PDFs.
type PdfIndirectReference struct {
	// ObjectNumber is the object number of the referenced indirect object.
	//
	// Deprecated: direct field access is retained for backward compatibility.
	// Prefer [PdfIndirectReference.Num] in new code.
	ObjectNumber int

	// GenerationNumber is the generation number of the referenced object.
	//
	// Deprecated: direct field access is retained for backward compatibility.
	// Prefer [PdfIndirectReference.Gen] in new code.
	GenerationNumber int
}

// NewPdfIndirectReference creates a new indirect reference with the given object and generation numbers.
func NewPdfIndirectReference(objNum, genNum int) *PdfIndirectReference {
	return &PdfIndirectReference{
		ObjectNumber:     objNum,
		GenerationNumber: genNum,
	}
}

// Type returns ObjectTypeReference.
func (r *PdfIndirectReference) Type() ObjectType { return ObjectTypeReference }

// Num returns the object number of the referenced indirect object.
func (r *PdfIndirectReference) Num() int { return r.ObjectNumber }

// Gen returns the generation number of the referenced object.
func (r *PdfIndirectReference) Gen() int { return r.GenerationNumber }

// WriteTo serializes the indirect reference as "objNum genNum R" to w.
func (r *PdfIndirectReference) WriteTo(w io.Writer) (int64, error) {
	written, err := fmt.Fprintf(w, "%d %d R", r.ObjectNumber, r.GenerationNumber)
	return int64(written), err
}
