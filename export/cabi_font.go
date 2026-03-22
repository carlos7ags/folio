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

	"github.com/carlos7ags/folio/font"
)

// standardFontHandles maps standard font names to pre-registered handles.
// Populated on first call to folio_font_standard.
var standardFontHandles map[string]uint64

func init() {
	standardFontHandles = make(map[string]uint64)
	// Pre-register all 14 standard fonts as persistent handles.
	for _, f := range []*font.Standard{
		font.Helvetica, font.HelveticaBold, font.HelveticaOblique, font.HelveticaBoldOblique,
		font.TimesRoman, font.TimesBold, font.TimesItalic, font.TimesBoldItalic,
		font.Courier, font.CourierBold, font.CourierOblique, font.CourierBoldOblique,
		font.Symbol, font.ZapfDingbats,
	} {
		id := ht.store(f)
		standardFontHandles[f.Name()] = id
	}
}

// folio_font_standard returns a handle for a PDF standard font looked up by name.
//
//export folio_font_standard
func folio_font_standard(name *C.char) C.uint64_t {
	goName := C.GoString(name)
	id, ok := standardFontHandles[goName]
	if !ok {
		setLastError("unknown standard font: " + goName)
		return 0
	}
	return C.uint64_t(id)
}

// folio_font_helvetica returns the handle for the Helvetica standard font.
//
//export folio_font_helvetica
func folio_font_helvetica() C.uint64_t {
	return C.uint64_t(standardFontHandles[font.Helvetica.Name()])
}

// folio_font_helvetica_bold returns the handle for the Helvetica-Bold standard font.
//
//export folio_font_helvetica_bold
func folio_font_helvetica_bold() C.uint64_t {
	return C.uint64_t(standardFontHandles[font.HelveticaBold.Name()])
}

// folio_font_times_roman returns the handle for the Times-Roman standard font.
//
//export folio_font_times_roman
func folio_font_times_roman() C.uint64_t {
	return C.uint64_t(standardFontHandles[font.TimesRoman.Name()])
}

// folio_font_times_bold returns the handle for the Times-Bold standard font.
//
//export folio_font_times_bold
func folio_font_times_bold() C.uint64_t {
	return C.uint64_t(standardFontHandles[font.TimesBold.Name()])
}

// folio_font_courier returns the handle for the Courier standard font.
//
//export folio_font_courier
func folio_font_courier() C.uint64_t {
	return C.uint64_t(standardFontHandles[font.Courier.Name()])
}

// folio_font_load_ttf loads a TrueType font from a file path and returns its handle.
//
//export folio_font_load_ttf
func folio_font_load_ttf(path *C.char) C.uint64_t {
	face, err := font.LoadTTF(C.GoString(path))
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	ef := font.NewEmbeddedFont(face)
	return C.uint64_t(ht.store(ef))
}

// folio_font_parse_ttf parses a TrueType font from in-memory bytes and returns its handle.
//
//export folio_font_parse_ttf
func folio_font_parse_ttf(data unsafe.Pointer, length C.int32_t) C.uint64_t {
	if data == nil || length <= 0 {
		setLastError("invalid font data")
		return 0
	}
	goData := C.GoBytes(data, C.int(length))
	face, err := font.ParseTTF(goData)
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	ef := font.NewEmbeddedFont(face)
	return C.uint64_t(ht.store(ef))
}

// folio_font_free releases an embedded font handle. Standard fonts are singletons and are not freed.
//
//export folio_font_free
func folio_font_free(fontH C.uint64_t) {
	// Standard fonts are singletons — don't delete them.
	v := ht.load(uint64(fontH))
	if v == nil {
		return
	}
	if _, isStd := v.(*font.Standard); isStd {
		return // no-op for standard fonts
	}
	ht.delete(uint64(fontH))
}

// loadEmbeddedFont retrieves a *font.EmbeddedFont from the handle table.
func loadEmbeddedFont(h C.uint64_t) (*font.EmbeddedFont, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid font handle")
		return nil, errInvalidHandle
	}
	ef, ok := v.(*font.EmbeddedFont)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not an embedded font (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return ef, errOK
}
