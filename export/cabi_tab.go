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

	"github.com/carlos7ags/folio/layout"
)

// folio_tabbed_line_new creates a tabbed line with tab stops.
// positions, aligns, leaders are parallel arrays of length count.
// aligns: 0=left, 1=right, 2=center. leaders: rune value (0=none, '.'=dot).
//
//export folio_tabbed_line_new
func folio_tabbed_line_new(fontH C.uint64_t, fontSize C.double,
	positions *C.double, aligns *C.int32_t, leaders *C.int32_t, count C.int32_t) C.uint64_t {
	f, errCode := loadStandardFont(fontH)
	if errCode != errOK {
		return 0
	}
	n := int(count)
	if !checkCArray(unsafe.Pointer(positions), n) ||
		!checkCArray(unsafe.Pointer(aligns), n) ||
		!checkCArray(unsafe.Pointer(leaders), n) {
		return 0
	}
	stops := parseTabs(positions, aligns, leaders, n)
	tl := layout.NewTabbedLine(f, float64(fontSize), stops...)
	return C.uint64_t(ht.store(tl))
}

//export folio_tabbed_line_new_embedded
func folio_tabbed_line_new_embedded(fontH C.uint64_t, fontSize C.double,
	positions *C.double, aligns *C.int32_t, leaders *C.int32_t, count C.int32_t) C.uint64_t {
	ef, errCode := loadEmbeddedFont(fontH)
	if errCode != errOK {
		return 0
	}
	n := int(count)
	if !checkCArray(unsafe.Pointer(positions), n) ||
		!checkCArray(unsafe.Pointer(aligns), n) ||
		!checkCArray(unsafe.Pointer(leaders), n) {
		return 0
	}
	stops := parseTabs(positions, aligns, leaders, n)
	tl := layout.NewTabbedLineEmbedded(ef, float64(fontSize), stops...)
	return C.uint64_t(ht.store(tl))
}

// folio_tabbed_line_set_segments sets the text segments.
// segments is an array of C strings of length count.
//
//export folio_tabbed_line_set_segments
func folio_tabbed_line_set_segments(tlH C.uint64_t, segments **C.char, count C.int32_t) C.int32_t {
	tl, errCode := loadTabbedLine(tlH)
	if errCode != errOK {
		return errCode
	}
	n := int(count)
	if n < 0 || n > maxCArrayCount {
		setLastError(fmt.Sprintf("array count %d out of range [0, %d]", n, maxCArrayCount))
		return errInvalidArg
	}
	goSegs := make([]string, n)
	if n > 0 && segments != nil {
		cArray := unsafe.Slice(segments, n)
		for i := 0; i < n; i++ {
			goSegs[i] = C.GoString(cArray[i])
		}
	}
	tl.SetSegments(goSegs...)
	return errOK
}

//export folio_tabbed_line_set_color
func folio_tabbed_line_set_color(tlH C.uint64_t, r, g, b C.double) C.int32_t {
	tl, errCode := loadTabbedLine(tlH)
	if errCode != errOK {
		return errCode
	}
	tl.SetColor(layout.RGB(float64(r), float64(g), float64(b)))
	return errOK
}

//export folio_tabbed_line_set_leading
func folio_tabbed_line_set_leading(tlH C.uint64_t, leading C.double) C.int32_t {
	tl, errCode := loadTabbedLine(tlH)
	if errCode != errOK {
		return errCode
	}
	tl.SetLeading(float64(leading))
	return errOK
}

//export folio_tabbed_line_free
func folio_tabbed_line_free(tlH C.uint64_t) {
	ht.delete(uint64(tlH))
}

func parseTabs(positions *C.double, aligns *C.int32_t, leaders *C.int32_t, n int) []layout.TabStop {
	stops := make([]layout.TabStop, n)
	cPos := unsafe.Slice(positions, n)
	cAlign := unsafe.Slice(aligns, n)
	cLeader := unsafe.Slice(leaders, n)
	for i := 0; i < n; i++ {
		stops[i] = layout.TabStop{
			Position: float64(cPos[i]),
			Align:    layout.TabAlign(cAlign[i]),
			Leader:   rune(cLeader[i]),
		}
	}
	return stops
}

func loadTabbedLine(h C.uint64_t) (*layout.TabbedLine, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid tabbed line handle")
		return nil, errInvalidHandle
	}
	tl, ok := v.(*layout.TabbedLine)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a tabbed line (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return tl, errOK
}
