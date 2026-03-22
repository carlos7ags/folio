// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
*/
import "C"
import (
	"fmt"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
)

// folio_page_add_text draws text on a low-level page using a standard font at the given position.
//
//export folio_page_add_text
func folio_page_add_text(pageH C.uint64_t, text *C.char, fontH C.uint64_t, size, x, y C.double) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	f, errCode := loadStandardFont(fontH)
	if errCode != errOK {
		return errCode
	}
	page.AddText(C.GoString(text), f, float64(size), float64(x), float64(y))
	return errOK
}

// folio_page_add_text_embedded draws text on a low-level page using an embedded font at the given position.
//
//export folio_page_add_text_embedded
func folio_page_add_text_embedded(pageH C.uint64_t, text *C.char, fontH C.uint64_t, size, x, y C.double) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	ef, errCode := loadEmbeddedFont(fontH)
	if errCode != errOK {
		return errCode
	}
	page.AddTextEmbedded(C.GoString(text), ef, float64(size), float64(x), float64(y))
	return errOK
}

// folio_page_add_link adds a URI link annotation to a page within the given rectangle.
//
//export folio_page_add_link
func folio_page_add_link(pageH C.uint64_t, x1, y1, x2, y2 C.double, uri *C.char) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	page.AddLink([4]float64{float64(x1), float64(y1), float64(x2), float64(y2)}, C.GoString(uri))
	return errOK
}

// folio_page_set_opacity sets the global opacity for subsequent drawing operations on a page.
//
//export folio_page_set_opacity
func folio_page_set_opacity(pageH C.uint64_t, alpha C.double) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	page.SetOpacity(float64(alpha))
	return errOK
}

// folio_page_set_rotate sets the page rotation in degrees (must be a multiple of 90).
//
//export folio_page_set_rotate
func folio_page_set_rotate(pageH C.uint64_t, degrees C.int32_t) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	page.SetRotate(int(degrees))
	return errOK
}

// loadPage retrieves a *document.Page from the handle table.
func loadPage(h C.uint64_t) (*document.Page, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid page handle")
		return nil, errInvalidHandle
	}
	page, ok := v.(*document.Page)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a page (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return page, errOK
}

// loadStandardFont retrieves a *font.Standard from the handle table.
func loadStandardFont(h C.uint64_t) (*font.Standard, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid font handle")
		return nil, errInvalidHandle
	}
	f, ok := v.(*font.Standard)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a standard font (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return f, errOK
}
