// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
*/
import "C"

import "github.com/carlos7ags/folio/reader"

// folio_extract_page_import extracts a page from a parsed PDF for importing
// into a new document. Returns a PageImport handle on success, 0 on failure.
// The returned handle holds the content stream, resolved resources, and dimensions.
//
//export folio_extract_page_import
func folio_extract_page_import(rH C.uint64_t, pageIndex C.int32_t) C.uint64_t {
	r, errCode := loadReader(rH)
	if errCode != errOK {
		return 0
	}
	imp, err := reader.ExtractPageImport(r, int(pageIndex))
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(imp))
}

// folio_page_import_free releases a PageImport handle.
//
//export folio_page_import_free
func folio_page_import_free(h C.uint64_t) {
	ht.delete(uint64(h))
}

// folio_page_import_width returns the page width in points.
//
//export folio_page_import_width
func folio_page_import_width(h C.uint64_t) C.double {
	imp, errCode := loadPageImport(h)
	if errCode != errOK {
		return 0
	}
	return C.double(imp.Width)
}

// folio_page_import_height returns the page height in points.
//
//export folio_page_import_height
func folio_page_import_height(h C.uint64_t) C.double {
	imp, errCode := loadPageImport(h)
	if errCode != errOK {
		return 0
	}
	return C.double(imp.Height)
}

// folio_page_import_apply imports the extracted page into a document page
// as a Form XObject background.
//
//export folio_page_import_apply
func folio_page_import_apply(pageH C.uint64_t, impH C.uint64_t) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	imp, errCode := loadPageImport(impH)
	if errCode != errOK {
		return errCode
	}
	page.ImportPage(imp.ContentStream, imp.Resources, imp.Width, imp.Height)
	return errOK
}

// loadPageImport retrieves a *reader.PageImport from the handle table.
func loadPageImport(h C.uint64_t) (*reader.PageImport, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid page import handle")
		return nil, errInvalidHandle
	}
	imp, ok := v.(*reader.PageImport)
	if !ok {
		setLastError("handle is not a page import")
		return nil, errTypeMismatch
	}
	return imp, errOK
}
