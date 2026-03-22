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
	"fmt"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/layout"
)

// folio_document_new creates a new document with a custom page size in points.
//
//export folio_document_new
func folio_document_new(width, height C.double) C.uint64_t {
	doc := document.NewDocument(document.PageSize{
		Width:  float64(width),
		Height: float64(height),
	})
	return C.uint64_t(ht.store(doc))
}

// folio_document_new_letter creates a new document with US Letter page size.
//
//export folio_document_new_letter
func folio_document_new_letter() C.uint64_t {
	doc := document.NewDocument(document.PageSizeLetter)
	return C.uint64_t(ht.store(doc))
}

// folio_document_new_a4 creates a new document with A4 page size.
//
//export folio_document_new_a4
func folio_document_new_a4() C.uint64_t {
	doc := document.NewDocument(document.PageSizeA4)
	return C.uint64_t(ht.store(doc))
}

// folio_document_set_title sets the PDF document title metadata.
//
//export folio_document_set_title
func folio_document_set_title(docH C.uint64_t, title *C.char) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	doc.Info.Title = C.GoString(title)
	return errOK
}

// folio_document_set_author sets the PDF document author metadata.
//
//export folio_document_set_author
func folio_document_set_author(docH C.uint64_t, author *C.char) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	doc.Info.Author = C.GoString(author)
	return errOK
}

// folio_document_set_margins sets the page margins in points (top, right, bottom, left).
//
//export folio_document_set_margins
func folio_document_set_margins(docH C.uint64_t, top, right, bottom, left C.double) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	doc.SetMargins(layout.Margins{
		Top:    float64(top),
		Right:  float64(right),
		Bottom: float64(bottom),
		Left:   float64(left),
	})
	return errOK
}

// folio_document_add_page adds a new low-level page to the document and returns its handle.
//
//export folio_document_add_page
func folio_document_add_page(docH C.uint64_t) C.uint64_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return 0
	}
	page := doc.AddPage()
	return C.uint64_t(ht.store(page))
}

// folio_document_page_count returns the number of pages in the document.
//
//export folio_document_page_count
func folio_document_page_count(docH C.uint64_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return 0
	}
	return C.int32_t(doc.PageCount())
}

// folio_document_add appends a layout element to the document for automatic pagination.
//
//export folio_document_add
func folio_document_add(docH C.uint64_t, elemH C.uint64_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	v := ht.load(uint64(elemH))
	if v == nil {
		setLastError("invalid element handle")
		return errInvalidHandle
	}
	elem, ok := v.(layout.Element)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a layout element (type %T)", uint64(elemH), v))
		return errTypeMismatch
	}
	doc.Add(elem)
	return errOK
}

// folio_document_save writes the document to a PDF file at the given path.
//
//export folio_document_save
func folio_document_save(docH C.uint64_t, path *C.char) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	if err := doc.Save(C.GoString(path)); err != nil {
		return setErr(errIO, err)
	}
	return errOK
}

// folio_document_write_to_buffer renders the document to an in-memory buffer and returns its handle.
//
//export folio_document_write_to_buffer
func folio_document_write_to_buffer(docH C.uint64_t) C.uint64_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return 0
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(newCBuffer(buf.Bytes())))
}

// folio_document_free removes a document handle from the handle table.
//
//export folio_document_free
func folio_document_free(docH C.uint64_t) {
	ht.delete(uint64(docH))
}

// loadDoc retrieves a *document.Document from the handle table.
func loadDoc(h C.uint64_t) (*document.Document, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid document handle")
		return nil, errInvalidHandle
	}
	doc, ok := v.(*document.Document)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a document (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return doc, errOK
}
