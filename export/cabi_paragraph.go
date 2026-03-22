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

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// folio_paragraph_new creates a paragraph with a standard font and returns its handle.
//
//export folio_paragraph_new
func folio_paragraph_new(text *C.char, fontH C.uint64_t, fontSize C.double) C.uint64_t {
	f, errCode := loadStandardFont(fontH)
	if errCode != errOK {
		return 0
	}
	p := layout.NewParagraph(C.GoString(text), f, float64(fontSize))
	return C.uint64_t(ht.store(p))
}

// folio_paragraph_new_embedded creates a paragraph with an embedded TrueType font.
//
//export folio_paragraph_new_embedded
func folio_paragraph_new_embedded(text *C.char, fontH C.uint64_t, fontSize C.double) C.uint64_t {
	ef, errCode := loadEmbeddedFont(fontH)
	if errCode != errOK {
		return 0
	}
	p := layout.NewParagraphEmbedded(C.GoString(text), ef, float64(fontSize))
	return C.uint64_t(ht.store(p))
}

// folio_paragraph_set_align sets the text alignment of a paragraph.
//
//export folio_paragraph_set_align
func folio_paragraph_set_align(pH C.uint64_t, align C.int32_t) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetAlign(layout.Align(align))
	return errOK
}

// folio_paragraph_set_leading sets the line spacing (leading) of a paragraph in points.
//
//export folio_paragraph_set_leading
func folio_paragraph_set_leading(pH C.uint64_t, leading C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetLeading(float64(leading))
	return errOK
}

// folio_paragraph_set_space_before sets the vertical spacing before a paragraph in points.
//
//export folio_paragraph_set_space_before
func folio_paragraph_set_space_before(pH C.uint64_t, pts C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetSpaceBefore(float64(pts))
	return errOK
}

// folio_paragraph_set_space_after sets the vertical spacing after a paragraph in points.
//
//export folio_paragraph_set_space_after
func folio_paragraph_set_space_after(pH C.uint64_t, pts C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetSpaceAfter(float64(pts))
	return errOK
}

// folio_paragraph_set_background sets the background color of a paragraph using RGB values in [0,1].
//
//export folio_paragraph_set_background
func folio_paragraph_set_background(pH C.uint64_t, r, g, b C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetBackground(layout.RGB(float64(r), float64(g), float64(b)))
	return errOK
}

// folio_paragraph_set_first_indent sets the first-line indentation of a paragraph in points.
//
//export folio_paragraph_set_first_indent
func folio_paragraph_set_first_indent(pH C.uint64_t, pts C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	p.SetFirstLineIndent(float64(pts))
	return errOK
}

// folio_paragraph_add_run appends a styled text run to a paragraph with the given font and color.
//
//export folio_paragraph_add_run
func folio_paragraph_add_run(pH C.uint64_t, text *C.char, fontH C.uint64_t, fontSize C.double, r, g, b C.double) C.int32_t {
	p, errCode := loadParagraph(pH)
	if errCode != errOK {
		return errCode
	}
	run := layout.TextRun{
		Text:     C.GoString(text),
		FontSize: float64(fontSize),
		Color:    layout.RGB(float64(r), float64(g), float64(b)),
	}
	// Determine font type from handle.
	v := ht.load(uint64(fontH))
	if v == nil {
		setLastError("invalid font handle")
		return errInvalidHandle
	}
	switch f := v.(type) {
	case *font.Standard:
		run.Font = f
	case *font.EmbeddedFont:
		run.Embedded = f
	default:
		setLastError(fmt.Sprintf("handle %d is not a font (type %T)", uint64(fontH), v))
		return errTypeMismatch
	}
	p.AddRun(run)
	return errOK
}

// folio_paragraph_free removes a paragraph handle from the handle table.
//
//export folio_paragraph_free
func folio_paragraph_free(pH C.uint64_t) {
	ht.delete(uint64(pH))
}

// loadParagraph retrieves a *layout.Paragraph from the handle table.
func loadParagraph(h C.uint64_t) (*layout.Paragraph, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid paragraph handle")
		return nil, errInvalidHandle
	}
	p, ok := v.(*layout.Paragraph)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a paragraph (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return p, errOK
}
