// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package optimize implements a first rewrite pass for existing PDF
// files: parse an input document with [reader.Parse] and re-serialize
// it through the writer's lossless optimization passes (cross-reference
// streams, object streams, orphan sweep, stream recompression, and
// object deduplication — see [document.WriteOptions]).
//
// The pipeline only carries pages and their resources across the
// rewrite: it walks the page tree via [reader.NewCopier], not the
// document catalog, so document-level structure the copier does not
// already handle — outlines, AcroForm fields, embedded file
// attachments, named destinations, page labels, and the structure tree
// — is dropped from the output. This makes the package a measurement
// vehicle for how much a lossless rewrite saves, not a general-purpose
// PDF optimizer.
//
// Encrypted inputs are refused, even ones that decrypt with an empty
// password: the standard security handler derives per-object keys from
// object numbers, and the rewrite renumbers every surviving object.
// Signed inputs are not rejected but rewriting one invalidates its
// signature, since the signed byte range no longer exists in the output.
package optimize

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/reader"
)

// ErrEncrypted is returned by [Bytes] when the input PDF is encrypted.
// Decryption is out of scope; the caller must supply an already
// decrypted document.
var ErrEncrypted = errors.New("optimize: input PDF is encrypted; decryption is not supported")

// Stats reports the size outcome of an optimize pass.
type Stats struct {
	BytesIn  int // size of the input PDF
	BytesOut int // size of the rewritten PDF
}

// SavedBytes returns how many bytes smaller the output is than the
// input. Never negative: [Bytes] falls back to the original input
// whenever the rewrite would grow it.
func (s Stats) SavedBytes() int {
	return s.BytesIn - s.BytesOut
}

// SavedPercent returns SavedBytes as a percentage of BytesIn, or 0 when
// BytesIn is 0.
func (s Stats) SavedPercent() float64 {
	if s.BytesIn == 0 {
		return 0
	}
	return 100 * float64(s.SavedBytes()) / float64(s.BytesIn)
}

// Bytes parses data as a PDF, rebuilds it through the writer's lossless
// optimization passes, and returns the result. The output always
// parses back into an equivalent page sequence and is never larger
// than the input — if the rewrite does not shrink the file, Bytes
// returns the original bytes unchanged.
//
// Bytes returns ErrEncrypted for encrypted input, including a document
// that decrypts successfully with an empty password.
func Bytes(data []byte) ([]byte, Stats, error) {
	r, err := reader.Parse(data)
	if err != nil {
		if isEncryptedErr(err) {
			return nil, Stats{}, ErrEncrypted
		}
		return nil, Stats{}, fmt.Errorf("optimize: parse: %w", err)
	}
	if r.Access() != reader.AccessNone {
		return nil, Stats{}, ErrEncrypted
	}

	out, err := rewrite(r)
	if err != nil {
		return nil, Stats{}, err
	}

	if len(out) >= len(data) {
		// The rewrite did not pay off — keep the original bytes rather
		// than hand back a larger, differently-structured file.
		return data, Stats{BytesIn: len(data), BytesOut: len(data)}, nil
	}
	return out, Stats{BytesIn: len(data), BytesOut: len(out)}, nil
}

// rewrite copies every page and the document info dictionary from r
// into a fresh writer, then serializes it with every lossless
// optimization pass enabled.
func rewrite(r *reader.PdfReader) ([]byte, error) {
	version := r.Version()
	if version == "" {
		version = "1.7"
	}
	w := document.NewWriter(version)
	copier := reader.NewCopier(r, w.AddObject)

	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))

	pagesDict := core.NewPdfDictionary()
	pagesDict.Set("Type", core.NewPdfName("Pages"))
	pagesRef := w.AddObject(pagesDict)
	catalog.Set("Pages", pagesRef)

	kids := core.NewPdfArray()
	pageCount := 0
	for i := range r.PageCount() {
		page, err := r.Page(i)
		if err != nil {
			return nil, fmt.Errorf("optimize: read page %d: %w", i, err)
		}

		copied, err := copier.CopyObject(page.Dict())
		if err != nil {
			return nil, fmt.Errorf("optimize: copy page %d: %w", i, err)
		}
		copiedDict, ok := copied.(*core.PdfDictionary)
		if !ok {
			return nil, fmt.Errorf("optimize: copy page %d: copied object is not a dictionary", i)
		}
		copiedDict.Remove("Parent")
		copiedDict.Set("Parent", pagesRef)

		pageRef := w.AddObject(copiedDict)
		kids.Add(pageRef)
		pageCount++
	}
	pagesDict.Set("Kids", kids)
	pagesDict.Set("Count", core.NewPdfInteger(pageCount))

	catalogRef := w.AddObject(catalog)
	w.SetRoot(catalogRef)

	if trailer := r.Trailer(); trailer != nil {
		if infoRef := trailer.Get("Info"); infoRef != nil {
			if copiedInfo, err := copier.CopyObject(infoRef); err == nil {
				if infoDict, ok := copiedInfo.(*core.PdfDictionary); ok {
					w.SetInfo(w.AddObject(infoDict))
				}
			}
		}
	}

	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, document.WriteOptions{
		UseXRefStream:      true,
		UseObjectStreams:   true,
		OrphanSweep:        true,
		RecompressStreams:  true,
		DeduplicateObjects: true,
	}); err != nil {
		return nil, fmt.Errorf("optimize: write: %w", err)
	}
	return buf.Bytes(), nil
}

// isEncryptedErr reports whether err is one of the sentinels
// reader.Parse returns for an encrypted document it could not open —
// a wrong password, or a security-handler configuration this library
// does not implement.
func isEncryptedErr(err error) bool {
	return errors.Is(err, reader.ErrInvalidPassword) || errors.Is(err, reader.ErrUnsupportedEncryption)
}
