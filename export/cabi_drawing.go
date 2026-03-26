// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
*/
import "C"

// folio_page_add_line draws a line from (x1,y1) to (x2,y2) with the given
// stroke width and RGB color.
//
//export folio_page_add_line
func folio_page_add_line(pageH C.uint64_t, x1, y1, x2, y2, width, r, g, b C.double) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	page.AddLine(float64(x1), float64(y1), float64(x2), float64(y2),
		float64(width), [3]float64{float64(r), float64(g), float64(b)})
	return errOK
}

// folio_page_add_rect draws a rectangle outline at (x,y) with given dimensions,
// stroke width, and RGB color.
//
//export folio_page_add_rect
func folio_page_add_rect(pageH C.uint64_t, x, y, w, h, strokeWidth, r, g, b C.double) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	page.AddRect(float64(x), float64(y), float64(w), float64(h),
		float64(strokeWidth), [3]float64{float64(r), float64(g), float64(b)})
	return errOK
}

// folio_page_add_rect_filled draws a filled rectangle at (x,y) with given
// dimensions and RGB fill color.
//
//export folio_page_add_rect_filled
func folio_page_add_rect_filled(pageH C.uint64_t, x, y, w, h, r, g, b C.double) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	page.AddRectFilled(float64(x), float64(y), float64(w), float64(h),
		[3]float64{float64(r), float64(g), float64(b)})
	return errOK
}
