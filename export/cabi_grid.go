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

//export folio_grid_new
func folio_grid_new() C.uint64_t {
	return C.uint64_t(ht.store(layout.NewGrid()))
}

//export folio_grid_add_child
func folio_grid_add_child(gridH C.uint64_t, elemH C.uint64_t) C.int32_t {
	g, errCode := loadGrid(gridH)
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
	g.AddChild(elem)
	return errOK
}

// folio_grid_set_template_columns sets column tracks.
// types and values are parallel arrays of length count.
// Types: 0=px, 1=percent, 2=fr, 3=auto.
//
//export folio_grid_set_template_columns
func folio_grid_set_template_columns(gridH C.uint64_t, types *C.int32_t, values *C.double, count C.int32_t) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	n := int(count)
	if !checkCArray(unsafe.Pointer(types), n) || !checkCArray(unsafe.Pointer(values), n) {
		return errInvalidArg
	}
	tracks := parseGridTracks(types, values, n)
	g.SetTemplateColumns(tracks)
	return errOK
}

//export folio_grid_set_template_rows
func folio_grid_set_template_rows(gridH C.uint64_t, types *C.int32_t, values *C.double, count C.int32_t) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	n := int(count)
	if !checkCArray(unsafe.Pointer(types), n) || !checkCArray(unsafe.Pointer(values), n) {
		return errInvalidArg
	}
	tracks := parseGridTracks(types, values, n)
	g.SetTemplateRows(tracks)
	return errOK
}

//export folio_grid_set_auto_rows
func folio_grid_set_auto_rows(gridH C.uint64_t, types *C.int32_t, values *C.double, count C.int32_t) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	n := int(count)
	if !checkCArray(unsafe.Pointer(types), n) || !checkCArray(unsafe.Pointer(values), n) {
		return errInvalidArg
	}
	tracks := parseGridTracks(types, values, n)
	g.SetAutoRows(tracks)
	return errOK
}

//export folio_grid_set_gap
func folio_grid_set_gap(gridH C.uint64_t, rowGap, colGap C.double) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	g.SetGap(float64(rowGap), float64(colGap))
	return errOK
}

//export folio_grid_set_placement
func folio_grid_set_placement(gridH C.uint64_t, childIndex C.int32_t,
	colStart, colEnd, rowStart, rowEnd C.int32_t) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	g.SetPlacement(int(childIndex), layout.GridPlacement{
		ColStart: int(colStart), ColEnd: int(colEnd),
		RowStart: int(rowStart), RowEnd: int(rowEnd),
	})
	return errOK
}

//export folio_grid_set_padding
func folio_grid_set_padding(gridH C.uint64_t, padding C.double) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	g.SetPadding(float64(padding))
	return errOK
}

//export folio_grid_set_background
func folio_grid_set_background(gridH C.uint64_t, r, g2, b C.double) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	g.SetBackground(layout.RGB(float64(r), float64(g2), float64(b)))
	return errOK
}

//export folio_grid_set_justify_items
func folio_grid_set_justify_items(gridH C.uint64_t, align C.int32_t) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	g.SetJustifyItems(layout.AlignItems(align))
	return errOK
}

//export folio_grid_set_align_items
func folio_grid_set_align_items(gridH C.uint64_t, align C.int32_t) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	g.SetAlignItems(layout.AlignItems(align))
	return errOK
}

//export folio_grid_set_justify_content
func folio_grid_set_justify_content(gridH C.uint64_t, justify C.int32_t) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	g.SetJustifyContent(layout.JustifyContent(justify))
	return errOK
}

//export folio_grid_set_align_content
func folio_grid_set_align_content(gridH C.uint64_t, align C.int32_t) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	g.SetAlignContent(layout.JustifyContent(align))
	return errOK
}

//export folio_grid_set_space_before
func folio_grid_set_space_before(gridH C.uint64_t, pts C.double) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	g.SetSpaceBefore(float64(pts))
	return errOK
}

//export folio_grid_set_space_after
func folio_grid_set_space_after(gridH C.uint64_t, pts C.double) C.int32_t {
	g, errCode := loadGrid(gridH)
	if errCode != errOK {
		return errCode
	}
	g.SetSpaceAfter(float64(pts))
	return errOK
}

//export folio_grid_free
func folio_grid_free(gridH C.uint64_t) {
	ht.delete(uint64(gridH))
}

func parseGridTracks(types *C.int32_t, values *C.double, n int) []layout.GridTrack {
	tracks := make([]layout.GridTrack, n)
	cTypes := unsafe.Slice(types, n)
	cValues := unsafe.Slice(values, n)
	for i := 0; i < n; i++ {
		tracks[i] = layout.GridTrack{
			Type:  layout.GridTrackType(cTypes[i]),
			Value: float64(cValues[i]),
		}
	}
	return tracks
}

func loadGrid(h C.uint64_t) (*layout.Grid, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid grid handle")
		return nil, errInvalidHandle
	}
	g, ok := v.(*layout.Grid)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a grid (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return g, errOK
}
