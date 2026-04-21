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
	// Deprecated: since v0.7.0, scheduled for removal at v1.0. Use
	// [PdfIndirectReference.Num] for reads and [PdfIndirectReference.SetNum]
	// for writes.
	ObjectNumber int

	// GenerationNumber is the generation number of the referenced object.
	//
	// Deprecated: since v0.7.0, scheduled for removal at v1.0. Use
	// [PdfIndirectReference.Gen] in new code.
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

// SetNum updates the object number this reference points to. Writer-side
// passes that renumber objects (e.g., orphan sweep, deduplication) use
// this in place of mutating the deprecated ObjectNumber field directly.
func (r *PdfIndirectReference) SetNum(objNum int) { r.ObjectNumber = objNum }

// WriteTo serializes the indirect reference as "objNum genNum R" to w.
func (r *PdfIndirectReference) WriteTo(w io.Writer) (int64, error) {
	written, err := fmt.Fprintf(w, "%d %d R", r.ObjectNumber, r.GenerationNumber)
	return int64(written), err
}
