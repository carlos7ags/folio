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
	"unsafe"

	"github.com/carlos7ags/folio/document"
)

//export folio_page_set_art_box
func folio_page_set_art_box(pageH C.uint64_t, x1, y1, x2, y2 C.double) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	page.SetArtBox([4]float64{float64(x1), float64(y1), float64(x2), float64(y2)})
	return errOK
}

//export folio_page_set_size
func folio_page_set_size(pageH C.uint64_t, width, height C.double) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	page.SetSize(document.PageSize{Width: float64(width), Height: float64(height)})
	return errOK
}

//export folio_page_add_page_link
func folio_page_add_page_link(pageH C.uint64_t, x1, y1, x2, y2 C.double, targetPage C.int32_t) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	page.AddPageLink([4]float64{float64(x1), float64(y1), float64(x2), float64(y2)}, int(targetPage))
	return errOK
}

//export folio_page_set_opacity_fill_stroke
func folio_page_set_opacity_fill_stroke(pageH C.uint64_t, fillAlpha, strokeAlpha C.double) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	page.SetOpacityFillStroke(float64(fillAlpha), float64(strokeAlpha))
	return errOK
}

//export folio_page_add_highlight
func folio_page_add_highlight(pageH C.uint64_t, x1, y1, x2, y2 C.double,
	r, g, b C.double, quadPoints unsafe.Pointer, quadCount C.int32_t) C.int32_t {
	return pageTextMarkup(pageH, document.MarkupHighlight, x1, y1, x2, y2, r, g, b, quadPoints, quadCount)
}

//export folio_page_add_underline_annotation
func folio_page_add_underline_annotation(pageH C.uint64_t, x1, y1, x2, y2 C.double,
	r, g, b C.double, quadPoints unsafe.Pointer, quadCount C.int32_t) C.int32_t {
	return pageTextMarkup(pageH, document.MarkupUnderline, x1, y1, x2, y2, r, g, b, quadPoints, quadCount)
}

//export folio_page_add_squiggly
func folio_page_add_squiggly(pageH C.uint64_t, x1, y1, x2, y2 C.double,
	r, g, b C.double, quadPoints unsafe.Pointer, quadCount C.int32_t) C.int32_t {
	return pageTextMarkup(pageH, document.MarkupSquiggly, x1, y1, x2, y2, r, g, b, quadPoints, quadCount)
}

//export folio_page_add_strikeout
func folio_page_add_strikeout(pageH C.uint64_t, x1, y1, x2, y2 C.double,
	r, g, b C.double, quadPoints unsafe.Pointer, quadCount C.int32_t) C.int32_t {
	return pageTextMarkup(pageH, document.MarkupStrikeOut, x1, y1, x2, y2, r, g, b, quadPoints, quadCount)
}

// pageTextMarkup is a helper for all text markup annotations.
// quadPoints is a flat array of 8*quadCount doubles.
func pageTextMarkup(pageH C.uint64_t, markupType document.MarkupType,
	x1, y1, x2, y2, r, g, b C.double,
	quadPoints unsafe.Pointer, quadCount C.int32_t) C.int32_t {
	page, errCode := loadPage(pageH)
	if errCode != errOK {
		return errCode
	}
	rect := [4]float64{float64(x1), float64(y1), float64(x2), float64(y2)}
	color := [3]float64{float64(r), float64(g), float64(b)}

	n := int(quadCount)
	if n < 0 || n > maxCArrayCount {
		setLastError(fmt.Sprintf("array count %d out of range [0, %d]", n, maxCArrayCount))
		return errInvalidArg
	}
	var qp [][8]float64
	if n > 0 && quadPoints != nil {
		flat := unsafe.Slice((*C.double)(quadPoints), n*8)
		qp = make([][8]float64, n)
		for i := 0; i < n; i++ {
			for j := 0; j < 8; j++ {
				qp[i][j] = float64(flat[i*8+j])
			}
		}
	}
	page.AddTextMarkup(markupType, rect, color, qp)
	return errOK
}
