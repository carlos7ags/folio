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

	"github.com/carlos7ags/folio/layout"
)

// folio_heading_new creates a heading element with the given text and level.
//
//export folio_heading_new
func folio_heading_new(text *C.char, level C.int32_t) C.uint64_t {
	h := layout.NewHeading(C.GoString(text), layout.HeadingLevel(level))
	return C.uint64_t(ht.store(h))
}

// folio_heading_new_with_font creates a heading with a specific standard font and size.
//
//export folio_heading_new_with_font
func folio_heading_new_with_font(text *C.char, level C.int32_t, fontH C.uint64_t, fontSize C.double) C.uint64_t {
	f, errCode := loadStandardFont(fontH)
	if errCode != errOK {
		return 0
	}
	h := layout.NewHeadingWithFont(C.GoString(text), layout.HeadingLevel(level), f, float64(fontSize))
	return C.uint64_t(ht.store(h))
}

// folio_heading_new_embedded creates a heading using an embedded TrueType font.
//
//export folio_heading_new_embedded
func folio_heading_new_embedded(text *C.char, level C.int32_t, fontH C.uint64_t) C.uint64_t {
	ef, errCode := loadEmbeddedFont(fontH)
	if errCode != errOK {
		return 0
	}
	h := layout.NewHeadingEmbedded(C.GoString(text), layout.HeadingLevel(level), ef)
	return C.uint64_t(ht.store(h))
}

// folio_heading_set_align sets the text alignment of a heading.
//
//export folio_heading_set_align
func folio_heading_set_align(hH C.uint64_t, align C.int32_t) C.int32_t {
	h, errCode := loadHeading(hH)
	if errCode != errOK {
		return errCode
	}
	h.SetAlign(layout.Align(align))
	return errOK
}

// folio_heading_free removes a heading handle from the handle table.
//
//export folio_heading_free
func folio_heading_free(hH C.uint64_t) {
	ht.delete(uint64(hH))
}

// loadHeading retrieves a *layout.Heading from the handle table.
func loadHeading(h C.uint64_t) (*layout.Heading, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid heading handle")
		return nil, errInvalidHandle
	}
	heading, ok := v.(*layout.Heading)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a heading (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return heading, errOK
}
