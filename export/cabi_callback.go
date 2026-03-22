// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>

// C trampoline for page decorator callbacks.
// cgo cannot call C function pointers directly from Go, so we need
// this wrapper function that Go can call via C.call_page_decorator.
typedef void (*folio_page_decorator_fn)(int32_t page_index, int32_t total_pages, uint64_t page_handle, void* user_data);

static void call_page_decorator(folio_page_decorator_fn fn, int32_t page_index, int32_t total_pages, uint64_t page_handle, void* user_data) {
    fn(page_index, total_pages, page_handle, user_data);
}
*/
import "C"
import (
	"unsafe"

	"github.com/carlos7ags/folio/document"
)

// folio_document_set_header registers a C callback invoked on each page to render a header.
//
//export folio_document_set_header
func folio_document_set_header(docH C.uint64_t, fn C.folio_page_decorator_fn, userData unsafe.Pointer) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	if fn == nil {
		setLastError("header callback function must not be NULL")
		return errInvalidArg
	}
	doc.SetHeader(makeDecorator(fn, userData))
	return errOK
}

// folio_document_set_footer registers a C callback invoked on each page to render a footer.
//
//export folio_document_set_footer
func folio_document_set_footer(docH C.uint64_t, fn C.folio_page_decorator_fn, userData unsafe.Pointer) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	if fn == nil {
		setLastError("footer callback function must not be NULL")
		return errInvalidArg
	}
	doc.SetFooter(makeDecorator(fn, userData))
	return errOK
}

// makeDecorator wraps a C function pointer into a Go PageDecorator.
func makeDecorator(fn C.folio_page_decorator_fn, userData unsafe.Pointer) document.PageDecorator {
	return func(ctx document.PageContext, page *document.Page) {
		// Store the page in the handle table so the callback can use it.
		pageH := ht.store(page)
		C.call_page_decorator(fn,
			C.int32_t(ctx.PageIndex),
			C.int32_t(ctx.TotalPages),
			C.uint64_t(pageH),
			userData,
		)
		// Remove the temporary page handle after the callback returns.
		ht.delete(pageH)
	}
}
