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

//export folio_columns_new
func folio_columns_new(cols C.int32_t) C.uint64_t {
	if cols < 1 {
		setLastError("columns must be >= 1")
		return 0
	}
	return C.uint64_t(ht.store(layout.NewColumns(int(cols))))
}

//export folio_columns_set_gap
func folio_columns_set_gap(colsH C.uint64_t, gap C.double) C.int32_t {
	c, errCode := loadColumns(colsH)
	if errCode != errOK {
		return errCode
	}
	c.SetGap(float64(gap))
	return errOK
}

//export folio_columns_set_widths
func folio_columns_set_widths(colsH C.uint64_t, widths *C.double, count C.int32_t) C.int32_t {
	c, errCode := loadColumns(colsH)
	if errCode != errOK {
		return errCode
	}
	n := int(count)
	if !checkCArray(unsafe.Pointer(widths), n) {
		return errInvalidArg
	}
	goWidths := make([]float64, n)
	cWidths := unsafe.Slice(widths, n)
	for i := 0; i < n; i++ {
		goWidths[i] = float64(cWidths[i])
	}
	c.SetWidths(goWidths)
	return errOK
}

//export folio_columns_add
func folio_columns_add(colsH C.uint64_t, colIndex C.int32_t, elemH C.uint64_t) C.int32_t {
	c, errCode := loadColumns(colsH)
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
	c.Add(int(colIndex), elem)
	return errOK
}

// folio_columns_set_balanced controls height-balanced sequential
// packing across columns. Enabled by default per CSS Multi-column
// Layout (`column-fill: balance`); disabling produces round-robin
// distribution across columns at layout time. Any non-zero value
// enables balanced fill.
//
//export folio_columns_set_balanced
func folio_columns_set_balanced(colsH C.uint64_t, balanced C.int32_t) C.int32_t {
	c, errCode := loadColumns(colsH)
	if errCode != errOK {
		return errCode
	}
	c.SetBalanced(balanced != 0)
	return errOK
}

//export folio_columns_free
func folio_columns_free(colsH C.uint64_t) {
	ht.delete(uint64(colsH))
}

func loadColumns(h C.uint64_t) (*layout.Columns, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid columns handle")
		return nil, errInvalidHandle
	}
	c, ok := v.(*layout.Columns)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a columns layout (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return c, errOK
}
