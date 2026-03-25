// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
*/
import "C"
import (
	"bytes"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

// folio_html_to_pdf converts an HTML string to a PDF file at the given output path.
//
//export folio_html_to_pdf
func folio_html_to_pdf(htmlStr *C.char, outputPath *C.char) C.int32_t {
	doc, err := htmlToDocument(C.GoString(htmlStr), 0, 0)
	if err != errOK {
		return err
	}
	if saveErr := doc.Save(C.GoString(outputPath)); saveErr != nil {
		return setErr(errIO, saveErr)
	}
	return errOK
}

// folio_html_to_buffer converts an HTML string to an in-memory PDF buffer with optional page dimensions.
//
//export folio_html_to_buffer
func folio_html_to_buffer(htmlStr *C.char, pageWidth, pageHeight C.double) C.uint64_t {
	doc, err := htmlToDocument(C.GoString(htmlStr), float64(pageWidth), float64(pageHeight))
	if err != errOK {
		return 0
	}
	var buf bytes.Buffer
	if _, writeErr := doc.WriteTo(&buf); writeErr != nil {
		setLastError(writeErr.Error())
		return 0
	}
	return C.uint64_t(ht.store(newCBuffer(buf.Bytes())))
}

// folio_html_convert converts an HTML string to a document handle for further manipulation.
//
//export folio_html_convert
func folio_html_convert(htmlStr *C.char, pageWidth, pageHeight C.double) C.uint64_t {
	doc, err := htmlToDocument(C.GoString(htmlStr), float64(pageWidth), float64(pageHeight))
	if err != errOK {
		return 0
	}
	return C.uint64_t(ht.store(doc))
}

// htmlToDocument converts HTML to a Document ready for save/write.
func htmlToDocument(htmlStr string, pageWidth, pageHeight float64) (*document.Document, C.int32_t) {
	opts := &html.Options{}
	if pageWidth > 0 {
		opts.PageWidth = pageWidth
	}
	if pageHeight > 0 {
		opts.PageHeight = pageHeight
	}

	result, err := html.ConvertFull(htmlStr, opts)
	if err != nil {
		setLastError(err.Error())
		return nil, errPDF
	}

	// Determine page size.
	ps := document.PageSizeLetter
	if pageWidth > 0 && pageHeight > 0 {
		ps = document.PageSize{Width: pageWidth, Height: pageHeight}
	}
	if pc := result.PageConfig; pc != nil {
		if pc.Width > 0 && pc.Height > 0 {
			ps = document.PageSize{Width: pc.Width, Height: pc.Height}
		}
	}

	doc := document.NewDocument(ps)

	// Apply @page margins.
	if pc := result.PageConfig; pc != nil && pc.HasMargins {
		doc.SetMargins(layout.Margins{
			Top: pc.MarginTop, Right: pc.MarginRight,
			Bottom: pc.MarginBottom, Left: pc.MarginLeft,
		})
	}

	// Apply @page margin boxes (page numbers, headers/footers).
	if result.MarginBoxes != nil {
		doc.SetMarginBoxes(result.MarginBoxes)
	}
	if result.FirstMarginBoxes != nil {
		doc.SetFirstMarginBoxes(result.FirstMarginBoxes)
	}

	// Apply metadata.
	if result.Metadata.Title != "" {
		doc.Info.Title = result.Metadata.Title
	}
	if result.Metadata.Author != "" {
		doc.Info.Author = result.Metadata.Author
	}

	for _, e := range result.Elements {
		doc.Add(e)
	}
	for _, abs := range result.Absolutes {
		doc.AddAbsoluteWithOpts(abs.Element, abs.X, abs.Y, abs.Width, layout.AbsoluteOpts{
			RightAligned: abs.RightAligned,
			ZIndex:       abs.ZIndex,
			PageIndex:    -1,
		})
	}

	return doc, errOK
}
