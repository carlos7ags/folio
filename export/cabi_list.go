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

//export folio_list_new
func folio_list_new(fontH C.uint64_t, fontSize C.double) C.uint64_t {
	f, errCode := loadStandardFont(fontH)
	if errCode != errOK {
		return 0
	}
	return C.uint64_t(ht.store(layout.NewList(f, float64(fontSize))))
}

//export folio_list_new_embedded
func folio_list_new_embedded(fontH C.uint64_t, fontSize C.double) C.uint64_t {
	ef, errCode := loadEmbeddedFont(fontH)
	if errCode != errOK {
		return 0
	}
	return C.uint64_t(ht.store(layout.NewListEmbedded(ef, float64(fontSize))))
}

//export folio_list_set_style
func folio_list_set_style(listH C.uint64_t, style C.int32_t) C.int32_t {
	l, errCode := loadList(listH)
	if errCode != errOK {
		return errCode
	}
	l.SetStyle(layout.ListStyle(style))
	return errOK
}

//export folio_list_set_indent
func folio_list_set_indent(listH C.uint64_t, indent C.double) C.int32_t {
	l, errCode := loadList(listH)
	if errCode != errOK {
		return errCode
	}
	l.SetIndent(float64(indent))
	return errOK
}

//export folio_list_add_item
func folio_list_add_item(listH C.uint64_t, text *C.char) C.int32_t {
	l, errCode := loadList(listH)
	if errCode != errOK {
		return errCode
	}
	l.AddItem(C.GoString(text))
	return errOK
}

//export folio_list_free
func folio_list_free(listH C.uint64_t) {
	ht.delete(uint64(listH))
}

func loadList(h C.uint64_t) (*layout.List, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid list handle")
		return nil, errInvalidHandle
	}
	l, ok := v.(*layout.List)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a list (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return l, errOK
}
