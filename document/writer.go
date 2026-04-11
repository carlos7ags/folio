// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package document provides the top-level API for creating and
// writing PDF documents.
package document

import (
	"crypto/rand"
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
	fileID     []byte                     // 16-byte file identifier for /ID in trailer
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

// SetFileID sets a 16-byte file identifier written to the trailer /ID array.
// PDF/A (all parts) requires /ID unconditionally (ISO 19005 §6.1.3).
// If not called, no /ID is written unless encryption is active.
func (w *Writer) SetFileID(id []byte) {
	w.fileID = id
}

// GenerateFileID creates a random 16-byte file identifier and sets it.
// Returns an error if the random source fails.
func (w *Writer) GenerateFileID() error {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return fmt.Errorf("writer: generate file ID: %w", err)
	}
	w.fileID = id
	return nil
}

// SetEncryption configures encryption for the PDF output.
// The encrypt dictionary is added as an indirect object; its object number
// is recorded so the encryption walk skips it.
func (w *Writer) SetEncryption(enc *core.Encryptor) {
	w.encryptor = enc
	encDict := enc.BuildEncryptDict()
	w.encryptRef = w.AddObject(encDict)
	enc.SetEncryptDictObjNum(w.encryptRef.Num())
}

// WriteTo writes the complete PDF file to the given writer using the
// historical default options: a traditional cross-reference table
// (ISO 32000-1 §7.5.4) and a separate trailer dictionary (§7.5.5).
//
// To opt into the optional cross-reference stream format (§7.5.8) or
// future writer behavior, use [Writer.WriteToWithOptions].
func (w *Writer) WriteTo(out io.Writer) (int64, error) {
	return w.WriteToWithOptions(out, WriteOptions{})
}

// countingWriter wraps an io.Writer and tracks the total bytes written.
type countingWriter struct {
	w io.Writer
	n int64
}

// Write writes p to the underlying writer and adds the number of bytes written to the total.
func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	return n, err
}
