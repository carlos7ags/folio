// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package document provides the top-level API for creating and
// writing PDF documents.
package document

import (
	"fmt"
	"io"

	"github.com/carlos7ags/folio/core"
)

// IndirectObject pairs a PdfObject with its object/generation numbers
// for writing as an indirect object definition.
type IndirectObject struct {
	ObjectNumber     int
	GenerationNumber int
	Object           core.PdfObject
}

// Writer serializes a collection of indirect objects into a valid PDF file.
// It handles the PDF file structure: header, object definitions,
// cross-reference table, trailer, and EOF marker.
type Writer struct {
	version    string // e.g. "1.7"
	objects    []IndirectObject
	root       *core.PdfIndirectReference // /Root entry in trailer
	info       *core.PdfIndirectReference // /Info entry in trailer (optional)
	encryptor  *core.Encryptor            // nil if no encryption
	encryptRef *core.PdfIndirectReference // /Encrypt entry in trailer
}

// NewWriter creates a Writer targeting the given PDF version.
func NewWriter(version string) *Writer {
	return &Writer{version: version}
}

// AddObject registers an object to be written. Returns the indirect
// reference that can be used to refer to this object elsewhere.
func (w *Writer) AddObject(obj core.PdfObject) *core.PdfIndirectReference {
	objNum := len(w.objects) + 1 // object numbers start at 1
	w.objects = append(w.objects, IndirectObject{
		ObjectNumber:     objNum,
		GenerationNumber: 0,
		Object:           obj,
	})
	return core.NewPdfIndirectReference(objNum, 0)
}

// SetRoot sets the document catalog reference for the trailer /Root entry.
func (w *Writer) SetRoot(ref *core.PdfIndirectReference) {
	w.root = ref
}

// SetInfo sets the document info reference for the trailer /Info entry.
func (w *Writer) SetInfo(ref *core.PdfIndirectReference) {
	w.info = ref
}

// SetEncryption configures encryption for the PDF output.
// The encrypt dictionary is added as an indirect object; its object number
// is recorded so the encryption walk skips it.
func (w *Writer) SetEncryption(enc *core.Encryptor) {
	w.encryptor = enc
	encDict := enc.BuildEncryptDict()
	w.encryptRef = w.AddObject(encDict)
	enc.SetEncryptDictObjNum(w.encryptRef.ObjectNumber)
}

// WriteTo writes the complete PDF file to the given writer.
func (w *Writer) WriteTo(out io.Writer) (int64, error) {
	// Encrypt all objects in place (except the /Encrypt dict itself).
	if w.encryptor != nil {
		for _, obj := range w.objects {
			if err := w.encryptor.EncryptObject(obj.Object, obj.ObjectNumber, obj.GenerationNumber); err != nil {
				return 0, fmt.Errorf("encrypt object %d: %w", obj.ObjectNumber, err)
			}
		}
	}

	cw := &countingWriter{w: out}

	// 1. Header
	if _, err := fmt.Fprintf(cw, "%%PDF-%s\n", w.version); err != nil {
		return cw.n, err
	}

	// 2. Binary comment (recommended by spec to signal binary content
	//    to file-type detectors). Four bytes with high bit set.
	if _, err := fmt.Fprintf(cw, "%%\xe2\xe3\xcf\xd3\n"); err != nil {
		return cw.n, err
	}

	// 3. Object definitions — track byte offsets for xref table
	offsets := make([]int64, len(w.objects))
	for i, obj := range w.objects {
		offsets[i] = cw.n
		if _, err := fmt.Fprintf(cw, "%d %d obj\n", obj.ObjectNumber, obj.GenerationNumber); err != nil {
			return cw.n, err
		}
		if _, err := obj.Object.WriteTo(cw); err != nil {
			return cw.n, err
		}
		if _, err := fmt.Fprint(cw, "\nendobj\n"); err != nil {
			return cw.n, err
		}
	}

	// 4. Cross-reference table
	xrefOffset := cw.n
	if _, err := fmt.Fprint(cw, "xref\n"); err != nil {
		return cw.n, err
	}
	// One section covering object 0 through N
	if _, err := fmt.Fprintf(cw, "0 %d\n", len(w.objects)+1); err != nil {
		return cw.n, err
	}
	// Entry for object 0 (free object, head of free list)
	if _, err := fmt.Fprint(cw, "0000000000 65535 f \n"); err != nil {
		return cw.n, err
	}
	// Entries for each indirect object
	for _, offset := range offsets {
		if _, err := fmt.Fprintf(cw, "%010d 00000 n \n", offset); err != nil {
			return cw.n, err
		}
	}

	// 5. Trailer
	trailer := core.NewPdfDictionary()
	trailer.Set("Size", core.NewPdfInteger(len(w.objects)+1))
	if w.root != nil {
		trailer.Set("Root", w.root)
	}
	if w.info != nil {
		trailer.Set("Info", w.info)
	}
	if w.encryptor != nil {
		trailer.Set("Encrypt", w.encryptRef)
		// /ID array is required for encrypted documents.
		id := core.NewPdfHexString(string(w.encryptor.FileID))
		trailer.Set("ID", core.NewPdfArray(id, id))
	}

	if _, err := fmt.Fprint(cw, "trailer\n"); err != nil {
		return cw.n, err
	}
	if _, err := trailer.WriteTo(cw); err != nil {
		return cw.n, err
	}
	if _, err := fmt.Fprint(cw, "\n"); err != nil {
		return cw.n, err
	}

	// 6. startxref + EOF
	if _, err := fmt.Fprintf(cw, "startxref\n%d\n%%%%EOF\n", xrefOffset); err != nil {
		return cw.n, err
	}

	return cw.n, nil
}

// countingWriter wraps an io.Writer and tracks the total bytes written.
type countingWriter struct {
	w io.Writer
	n int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	return n, err
}
