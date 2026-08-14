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
//
// Input carrying document-level compliance data — PDF/A output
// intents, XMP metadata, embedded/associated files, tagging, or a
// document language — is refused with [ErrLossyInput] unless the
// caller opts in with [Options].AllowLossy, since the rewrite would
// silently strip it.
package optimize

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/reader"
)

// ErrEncrypted is returned by [Bytes] when the input PDF is encrypted.
// Decryption is out of scope; the caller must supply an already
// decrypted document.
var ErrEncrypted = errors.New("optimize: input PDF is encrypted; decryption is not supported")

// ErrLossyInput is returned by [Bytes] when the input carries
// document-level structure the rewrite would silently drop — PDF/A
// output intents, XMP metadata, embedded/associated files, tagging, or
// a document language. Opt in with [Options].AllowLossy to strip it
// anyway.
var ErrLossyInput = errors.New("optimize: input carries document-level data the rewrite would drop")

// lossyCatalogKeys are the catalog entries whose loss changes the
// document's meaning or compliance status, not just its navigation.
var lossyCatalogKeys = []string{
	"OutputIntents", "Metadata", "AF", "MarkInfo", "StructTreeRoot", "Lang",
}

// lossyKeys reports which compliance-bearing entries the source
// catalog carries. "Names" counts only when it holds /EmbeddedFiles.
func lossyKeys(r *reader.PdfReader) ([]string, error) {
	catalog := r.Catalog()
	if catalog == nil {
		return nil, nil
	}

	var keys []string
	for _, key := range lossyCatalogKeys {
		if catalog.Get(key) != nil {
			keys = append(keys, key)
		}
	}

	if namesObj := catalog.Get("Names"); namesObj != nil {
		resolved, err := r.ResolveObject(namesObj)
		if err != nil {
			return nil, fmt.Errorf("optimize: resolve Names: %w", err)
		}
		if namesDict, ok := resolved.(*core.PdfDictionary); ok {
			if namesDict.Get("EmbeddedFiles") != nil {
				keys = append(keys, "Names.EmbeddedFiles")
			}
		}
	}

	return keys, nil
}

// Options configures an optimize pass.
type Options struct {
	// AllowLossy permits optimizing a document that carries
	// document-level compliance data (see [ErrLossyInput]), accepting
	// that the rewrite strips it.
	AllowLossy bool
}

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
//
// Bytes returns ErrLossyInput when the input catalog carries
// document-level compliance data (PDF/A output intents, XMP metadata,
// embedded/associated files, tagging, or a document language) that the
// rewrite would silently drop, unless the caller passes
// Options{AllowLossy: true}.
func Bytes(data []byte, opts ...Options) ([]byte, Stats, error) {
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

	var o Options
	if len(opts) > 0 {
		o = opts[0]
	}
	if !o.AllowLossy {
		keys, err := lossyKeys(r)
		if err != nil {
			return nil, Stats{}, fmt.Errorf("optimize: inspect catalog: %w", err)
		}
		if len(keys) > 0 {
			return nil, Stats{}, fmt.Errorf("%w (would drop: %s)", ErrLossyInput, strings.Join(keys, ", "))
		}
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

		if page.Dict().Get("MediaBox") == nil && !page.MediaBox.IsZero() {
			copiedDict.Set("MediaBox", boxToArray(page.MediaBox))
		}
		if page.Dict().Get("CropBox") == nil && !page.CropBox.IsZero() {
			copiedDict.Set("CropBox", boxToArray(page.CropBox))
		}
		if page.Dict().Get("Rotate") == nil && page.Rotate != 0 {
			copiedDict.Set("Rotate", core.NewPdfInteger(page.Rotate))
		}
		if page.Dict().Get("Resources") == nil {
			res, err := page.Resources()
			if err != nil {
				return nil, fmt.Errorf("optimize: page %d resources: %w", i, err)
			}
			if res.Len() > 0 {
				copiedRes, err := copier.CopyObject(res)
				if err != nil {
					return nil, fmt.Errorf("optimize: copy page %d resources: %w", i, err)
				}
				copiedDict.Set("Resources", copiedRes)
			}
		}

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

// boxToArray converts a reader.Box back to a PDF rectangle array.
func boxToArray(b reader.Box) *core.PdfArray {
	return core.NewPdfArray(
		core.NewPdfReal(b.X1), core.NewPdfReal(b.Y1),
		core.NewPdfReal(b.X2), core.NewPdfReal(b.Y2),
	)
}

// isEncryptedErr reports whether err is one of the sentinels
// reader.Parse returns for an encrypted document it could not open —
// a wrong password, or a security-handler configuration this library
// does not implement.
func isEncryptedErr(err error) bool {
	return errors.Is(err, reader.ErrInvalidPassword) || errors.Is(err, reader.ErrUnsupportedEncryption)
}
