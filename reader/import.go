// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"fmt"

	"github.com/carlos7ags/folio/core"
)

// PageImport holds the data needed to import an existing PDF page into
// a new document via document.Page.ImportPage. Use ExtractPageImport to
// extract this from a parsed PDF.
//
// All indirect references in the Resources dictionary are fully resolved
// and deep-copied, so the PageImport is self-contained and independent
// of the source PdfReader.
type PageImport struct {
	// ContentStream is the decompressed page content stream bytes.
	ContentStream []byte

	// Resources is the page's /Resources dictionary with all indirect
	// references resolved. Fonts, images, color spaces, and other
	// objects are inlined as direct objects.
	Resources *core.PdfDictionary

	// Width is the page width in PDF points.
	Width float64

	// Height is the page height in PDF points.
	Height float64
}

// ExtractPageImport extracts the content stream and resources from a
// parsed PDF page, ready for importing into a new document.Page.
// All resource objects are deep-copied and resolved so the result is
// self-contained — it does not reference the source PdfReader.
//
// Usage:
//
//	r, _ := reader.Parse(pdfBytes)
//	imp, _ := reader.ExtractPageImport(r, 0)
//	p.ImportPage(imp.ContentStream, imp.Resources, imp.Width, imp.Height)
func ExtractPageImport(r *PdfReader, pageIndex int) (*PageImport, error) {
	page, err := r.Page(pageIndex)
	if err != nil {
		return nil, fmt.Errorf("import: page %d: %w", pageIndex, err)
	}

	data, err := page.ContentStream()
	if err != nil {
		return nil, fmt.Errorf("import: content stream: %w", err)
	}

	resources, err := page.Resources()
	if err != nil {
		return nil, fmt.Errorf("import: resources: %w", err)
	}

	// Deep-copy resources, resolving all indirect references so the
	// result is independent of the source reader. Without this, the
	// Resources dictionary contains indirect references (e.g. "5 0 R")
	// that are meaningless outside the source PDF.
	resolvedRes, err := resolveDeep(resources, r.resolver)
	if err != nil {
		return nil, fmt.Errorf("import: resolve resources: %w", err)
	}
	resDict, _ := resolvedRes.(*core.PdfDictionary)

	return &PageImport{
		ContentStream: data,
		Resources:     resDict,
		Width:         page.Width,
		Height:        page.Height,
	}, nil
}

// resolveDeep recursively resolves all indirect references in a PDF object
// tree, producing a fully self-contained object with no indirect refs.
// Streams are preserved with their data and dictionary entries resolved.
func resolveDeep(obj core.PdfObject, res *resolver) (core.PdfObject, error) {
	return resolveDeepVisited(obj, res, make(map[int]core.PdfObject))
}

func resolveDeepVisited(obj core.PdfObject, res *resolver, visited map[int]core.PdfObject) (core.PdfObject, error) {
	if obj == nil {
		return nil, nil
	}

	// Resolve indirect references.
	if ref, ok := obj.(*core.PdfIndirectReference); ok {
		// Check for cycles.
		if cached, ok := visited[ref.ObjectNumber]; ok {
			return cached, nil
		}
		resolved, err := res.Resolve(ref.ObjectNumber)
		if err != nil {
			return obj, nil // return unresolved on error
		}
		// Mark as visited before recursing (handles cycles).
		visited[ref.ObjectNumber] = nil
		result, err := resolveDeepVisited(resolved, res, visited)
		if err != nil {
			return resolved, nil
		}
		visited[ref.ObjectNumber] = result
		return result, nil
	}

	switch o := obj.(type) {
	case *core.PdfDictionary:
		newDict := core.NewPdfDictionary()
		for _, entry := range o.Entries {
			resolved, err := resolveDeepVisited(entry.Value, res, visited)
			if err != nil {
				return nil, err
			}
			newDict.Set(entry.Key.Value, resolved)
		}
		return newDict, nil

	case *core.PdfArray:
		newArr := core.NewPdfArray()
		for _, elem := range o.Elements {
			resolved, err := resolveDeepVisited(elem, res, visited)
			if err != nil {
				return nil, err
			}
			newArr.Add(resolved)
		}
		return newArr, nil

	case *core.PdfStream:
		// Copy the stream data and resolve dict entries.
		newStream := core.NewPdfStream(o.Data)
		for _, entry := range o.Dict.Entries {
			resolved, err := resolveDeepVisited(entry.Value, res, visited)
			if err != nil {
				return nil, err
			}
			newStream.Dict.Set(entry.Key.Value, resolved)
		}
		return newStream, nil

	default:
		// Primitives (number, string, name, bool, null) — return as-is.
		return obj, nil
	}
}
