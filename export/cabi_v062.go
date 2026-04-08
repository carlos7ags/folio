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

	foliohtml "github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/sign"
)

// ── Document convenience ──────────────────────────────────────────

// folio_document_to_bytes returns the document as a PDF buffer.
//
//export folio_document_to_bytes
func folio_document_to_bytes(docH C.uint64_t) C.uint64_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return 0
	}
	data, err := doc.ToBytes()
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(newCBuffer(data)))
}

// folio_document_validate_pdfa validates the document against PDF/A requirements.
//
//export folio_document_validate_pdfa
func folio_document_validate_pdfa(docH C.uint64_t) C.int32_t {
	doc, errCode := loadDoc(docH)
	if errCode != errOK {
		return errCode
	}
	if err := doc.ValidatePdfA(); err != nil {
		return setErr(errPDF, err)
	}
	return errOK
}

// ── Div extensions ────────────────────────────────────────────────

//export folio_div_set_aspect_ratio
func folio_div_set_aspect_ratio(divH C.uint64_t, ratio C.double) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetAspectRatio(float64(ratio))
	return errOK
}

//export folio_div_set_keep_together
func folio_div_set_keep_together(divH C.uint64_t, enabled C.int32_t) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetKeepTogether(enabled != 0)
	return errOK
}

//export folio_div_set_border_radius_per_corner
func folio_div_set_border_radius_per_corner(divH C.uint64_t, tl, tr, br, bl C.double) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetBorderRadiusPerCorner(float64(tl), float64(tr), float64(br), float64(bl))
	return errOK
}

//export folio_div_set_width_percent
func folio_div_set_width_percent(divH C.uint64_t, pct C.double) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetWidthPercent(float64(pct))
	return errOK
}

//export folio_div_set_hcenter
func folio_div_set_hcenter(divH C.uint64_t, enabled C.int32_t) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetHCenter(enabled != 0)
	return errOK
}

//export folio_div_set_hright
func folio_div_set_hright(divH C.uint64_t, enabled C.int32_t) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetHRight(enabled != 0)
	return errOK
}

//export folio_div_set_clear
func folio_div_set_clear(divH C.uint64_t, value *C.char) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetClear(C.GoString(value))
	return errOK
}

//export folio_div_set_outline
func folio_div_set_outline(divH C.uint64_t, width C.double, style *C.char,
	r, g, b, offset C.double) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.SetOutline(
		float64(width),
		C.GoString(style),
		layout.RGB(float64(r), float64(g), float64(b)),
		float64(offset),
	)
	return errOK
}

//export folio_div_add_box_shadow
func folio_div_add_box_shadow(divH C.uint64_t,
	offsetX, offsetY, blur, spread, r, g, b C.double) C.int32_t {
	div, errCode := loadDiv(divH)
	if errCode != errOK {
		return errCode
	}
	div.AddBoxShadow(layout.BoxShadow{
		OffsetX: float64(offsetX),
		OffsetY: float64(offsetY),
		Blur:    float64(blur),
		Spread:  float64(spread),
		Color:   layout.RGB(float64(r), float64(g), float64(b)),
	})
	return errOK
}

// ── Cell extensions ───────────────────────────────────────────────

//export folio_cell_set_border_radius
func folio_cell_set_border_radius(cellH C.uint64_t, radius C.double) C.int32_t {
	cell, errCode := loadCell(cellH)
	if errCode != errOK {
		return errCode
	}
	cell.SetBorderRadius(float64(radius))
	return errOK
}

//export folio_cell_set_border_radius_per_corner
func folio_cell_set_border_radius_per_corner(cellH C.uint64_t, tl, tr, br, bl C.double) C.int32_t {
	cell, errCode := loadCell(cellH)
	if errCode != errOK {
		return errCode
	}
	cell.SetBorderRadiusPerCorner(float64(tl), float64(tr), float64(br), float64(bl))
	return errOK
}

// ── Grid extensions ───────────────────────────────────────────────

//export folio_grid_set_border
func folio_grid_set_border(gridH C.uint64_t, width, cr, cg, cb C.double) C.int32_t {
	gr, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	gr.SetBorder(layout.Border{
		Width: float64(width),
		Color: layout.RGB(float64(cr), float64(cg), float64(cb)),
	})
	return errOK
}

//export folio_grid_set_borders
func folio_grid_set_borders(gridH C.uint64_t,
	topW, topR, topG, topB C.double,
	rightW, rightR, rightG, rightB C.double,
	bottomW, bottomR, bottomG, bottomB C.double,
	leftW, leftR, leftG, leftB C.double) C.int32_t {
	gr, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	gr.SetBorders(layout.CellBorders{
		Top:    layout.Border{Width: float64(topW), Color: layout.RGB(float64(topR), float64(topG), float64(topB))},
		Right:  layout.Border{Width: float64(rightW), Color: layout.RGB(float64(rightR), float64(rightG), float64(rightB))},
		Bottom: layout.Border{Width: float64(bottomW), Color: layout.RGB(float64(bottomR), float64(bottomG), float64(bottomB))},
		Left:   layout.Border{Width: float64(leftW), Color: layout.RGB(float64(leftR), float64(leftG), float64(leftB))},
	})
	return errOK
}

//export folio_grid_set_template_areas
func folio_grid_set_template_areas(gridH C.uint64_t, rows **C.char,
	cols *C.int32_t, rowCount C.int32_t) C.int32_t {
	gr, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	n := int(rowCount)
	if n == 0 || rows == nil {
		gr.SetTemplateAreas(nil)
		return errOK
	}
	cRows := (*[1 << 20]*C.char)(unsafe.Pointer(rows))[:n:n]
	cCols := (*[1 << 20]C.int32_t)(unsafe.Pointer(cols))[:n:n]
	areas := make([][]string, n)
	for i := 0; i < n; i++ {
		// Each row is a space-separated string; split into cells.
		rowStr := C.GoString(cRows[i])
		colCount := int(cCols[i])
		areas[i] = splitAreaRow(rowStr, colCount)
	}
	gr.SetTemplateAreas(areas)
	return errOK
}

// splitAreaRow splits a template area row into cells by whitespace.
func splitAreaRow(s string, colCount int) []string {
	result := make([]string, 0, colCount)
	var current []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' {
			if len(current) > 0 {
				result = append(result, string(current))
				current = current[:0]
			}
			continue
		}
		current = append(current, c)
	}
	if len(current) > 0 {
		result = append(result, string(current))
	}
	return result
}

// ── Flex extensions ───────────────────────────────────────────────

//export folio_flex_set_borders
func folio_flex_set_borders(flexH C.uint64_t,
	topW, topR, topG, topB C.double,
	rightW, rightR, rightG, rightB C.double,
	bottomW, bottomR, bottomG, bottomB C.double,
	leftW, leftR, leftG, leftB C.double) C.int32_t {
	fl, errCode := loadFlex(flexH)
	if errCode != errOK {
		return errCode
	}
	fl.SetBorders(layout.CellBorders{
		Top:    layout.Border{Width: float64(topW), Color: layout.RGB(float64(topR), float64(topG), float64(topB))},
		Right:  layout.Border{Width: float64(rightW), Color: layout.RGB(float64(rightR), float64(rightG), float64(rightB))},
		Bottom: layout.Border{Width: float64(bottomW), Color: layout.RGB(float64(bottomR), float64(bottomG), float64(bottomB))},
		Left:   layout.Border{Width: float64(leftW), Color: layout.RGB(float64(leftR), float64(leftG), float64(leftB))},
	})
	return errOK
}

//export folio_flex_set_align_content
func folio_flex_set_align_content(flexH C.uint64_t, align C.int32_t) C.int32_t {
	fl, errCode := loadFlex(flexH)
	if errCode != errOK {
		return errCode
	}
	fl.SetAlignContent(layout.JustifyContent(align))
	return errOK
}

// ── Paragraph extensions ──────────────────────────────────────────

//export folio_paragraph_set_text_align_last
func folio_paragraph_set_text_align_last(pH C.uint64_t, align C.int32_t) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetTextAlignLast(layout.Align(align))
	return errOK
}

// ── Image element extensions ──────────────────────────────────────

//export folio_image_element_set_object_fit
func folio_image_element_set_object_fit(ieH C.uint64_t, fit *C.char) C.int32_t {
	v := ht.load(uint64(ieH))
	if v == nil {
		setLastError("invalid image element handle")
		return errInvalidHandle
	}
	ie, ok := v.(*layout.ImageElement)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not an image element", uint64(ieH)))
		return errTypeMismatch
	}
	ie.SetObjectFit(C.GoString(fit))
	return errOK
}

//export folio_image_element_set_object_position
func folio_image_element_set_object_position(ieH C.uint64_t, pos *C.char) C.int32_t {
	v := ht.load(uint64(ieH))
	if v == nil {
		setLastError("invalid image element handle")
		return errInvalidHandle
	}
	ie, ok := v.(*layout.ImageElement)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not an image element", uint64(ieH)))
		return errTypeMismatch
	}
	ie.SetObjectPosition(C.GoString(pos))
	return errOK
}

// ── TextRun background color (text highlight) ─────────────────────

//export folio_run_list_last_set_background_color
func folio_run_list_last_set_background_color(rlH C.uint64_t, r, g, b C.double) C.int32_t {
	rl, errCode := loadRunList(rlH)
	if errCode != errOK {
		return errCode
	}
	if len(rl.runs) == 0 {
		setLastError("run list is empty")
		return errInvalidArg
	}
	idx := len(rl.runs) - 1
	rl.runs[idx] = rl.runs[idx].WithBackgroundColor(layout.RGB(float64(r), float64(g), float64(b)))
	return errOK
}

// ── Signing: PKCS#12 support ──────────────────────────────────────

//export folio_signer_new_pkcs12
func folio_signer_new_pkcs12(data unsafe.Pointer, length C.int32_t, password *C.char) C.uint64_t {
	if data == nil || length <= 0 {
		setLastError("invalid PKCS#12 data")
		return 0
	}
	bytes := C.GoBytes(data, C.int(length))
	signer, err := sign.ParsePKCS12(bytes, C.GoString(password))
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(signer))
}

// ── HTML utility ──────────────────────────────────────────────────

//export folio_html_parse_css_length
func folio_html_parse_css_length(s *C.char, fontSize, relativeTo C.double) C.double {
	return C.double(foliohtml.ParseCSSLength(C.GoString(s), float64(fontSize), float64(relativeTo)))
}
