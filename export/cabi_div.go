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

// folio_div_new creates a new empty div container and returns its handle.
//
//export folio_div_new
func folio_div_new() C.uint64_t {
	return C.uint64_t(ht.store(layout.NewDiv()))
}

// folio_div_add appends a layout element to a div.
//
//export folio_div_add
func folio_div_add(divH C.uint64_t, elemH C.uint64_t) C.int32_t {
	div, errCode := loadDiv(divH)
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
		setLastError(fmt.Sprintf("handle %d is not a layout element", uint64(elemH)))
		return errTypeMismatch
	}
	div.Add(elem)
	return errOK
}

// folio_div_set_padding sets the padding on all four sides of a div in points.
//
//export folio_div_set_padding
func folio_div_set_padding(divH C.uint64_t, top, right, bottom, left C.double) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetPaddingAll(layout.Padding{
		Top: float64(top), Right: float64(right),
		Bottom: float64(bottom), Left: float64(left),
	})
	return errOK
}

// folio_div_set_background sets the background color of a div using RGB values in [0,1].
//
//export folio_div_set_background
func folio_div_set_background(divH C.uint64_t, r, g, b C.double) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetBackground(layout.RGB(float64(r), float64(g), float64(b)))
	return errOK
}

// folio_div_set_border sets a solid border on a div with the given width and RGB color.
//
//export folio_div_set_border
func folio_div_set_border(divH C.uint64_t, width C.double, r, g, b C.double) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetBorder(layout.SolidBorder(float64(width), layout.RGB(float64(r), float64(g), float64(b))))
	return errOK
}

// folio_div_set_width sets the fixed width of a div in points.
//
//export folio_div_set_width
func folio_div_set_width(divH C.uint64_t, pts C.double) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetWidth(float64(pts))
	return errOK
}

// folio_div_set_min_height sets the minimum height of a div in points.
//
//export folio_div_set_min_height
func folio_div_set_min_height(divH C.uint64_t, pts C.double) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetMinHeight(float64(pts))
	return errOK
}

// folio_div_set_space_before sets the vertical spacing before a div in points.
//
//export folio_div_set_space_before
func folio_div_set_space_before(divH C.uint64_t, pts C.double) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetSpaceBefore(float64(pts))
	return errOK
}

// folio_div_set_space_after sets the vertical spacing after a div in points.
//
//export folio_div_set_space_after
func folio_div_set_space_after(divH C.uint64_t, pts C.double) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetSpaceAfter(float64(pts))
	return errOK
}

// folio_div_free removes a div handle from the handle table.
//
//export folio_div_free
func folio_div_free(divH C.uint64_t) {
	ht.delete(uint64(divH))
}

// folio_line_separator_new creates a horizontal line separator element and returns its handle.
//
//export folio_line_separator_new
func folio_line_separator_new() C.uint64_t {
	return C.uint64_t(ht.store(layout.NewLineSeparator()))
}

// folio_area_break_new creates a page break element and returns its handle.
//
//export folio_area_break_new
func folio_area_break_new() C.uint64_t {
	return C.uint64_t(ht.store(layout.NewAreaBreak()))
}

// loadDiv retrieves a *layout.Div from the handle table.
func loadDiv(h C.uint64_t) (*layout.Div, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid div handle")
		return nil, errInvalidHandle
	}
	div, ok := v.(*layout.Div)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a div (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return div, errOK
}
