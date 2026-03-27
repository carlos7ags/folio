// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
*/
import "C"

import "github.com/carlos7ags/folio/layout"

// folio_document_set_header_text sets a simple text header that automatically
// reserves space so content doesn't overlap. The text may contain {page} and
// {pages} placeholders. align: 0=left, 1=center, 2=right.
//
//export folio_document_set_header_text
func folio_document_set_header_text(docH C.uint64_t, text *C.char, fontH C.uint64_t, size C.double, align C.int32_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	f, errCode := loadStandardFont(fontH)
	if errCode != errOK {
		return errCode
	}
	doc.SetHeaderText(C.GoString(text), f, float64(size), layout.Align(align))
	return errOK
}

// folio_document_set_footer_text sets a simple text footer that automatically
// reserves space. See folio_document_set_header_text for details.
//
//export folio_document_set_footer_text
func folio_document_set_footer_text(docH C.uint64_t, text *C.char, fontH C.uint64_t, size C.double, align C.int32_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	f, errCode := loadStandardFont(fontH)
	if errCode != errOK {
		return errCode
	}
	doc.SetFooterText(C.GoString(text), f, float64(size), layout.Align(align))
	return errOK
}
