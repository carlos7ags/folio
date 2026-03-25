// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
*/
import "C"
import (
	"bytes"
	"fmt"
	"unsafe"

	"github.com/carlos7ags/folio/reader"
)

// folio_reader_merge merges multiple PDF reader handles into a single modifiable document.
// readers is an array of uint64_t reader handles, count is the number of handles.
//
//export folio_reader_merge
func folio_reader_merge(readers *C.uint64_t, count C.int32_t) C.uint64_t {
	n := int(count)
	if n <= 0 || readers == nil {
		setLastError("merge requires at least one reader")
		return 0
	}
	cHandles := (*[1 << 20]C.uint64_t)(unsafe.Pointer(readers))[:n:n]
	goReaders := make([]*reader.PdfReader, n)
	for i := 0; i < n; i++ {
		r, errCode := loadReader(C.uint64_t(cHandles[i]))
		if errCode != errOK {
			return 0
		}
		goReaders[i] = r
	}
	m, err := reader.Merge(goReaders...)
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(m))
}

// folio_merge_files merges PDF files by path into a single modifiable document.
// paths is an array of C strings, count is the number of paths.
//
//export folio_merge_files
func folio_merge_files(paths **C.char, count C.int32_t) C.uint64_t {
	n := int(count)
	if n <= 0 || paths == nil {
		setLastError("merge requires at least one path")
		return 0
	}
	cPaths := (*[1 << 20]*C.char)(unsafe.Pointer(paths))[:n:n]
	goPaths := make([]string, n)
	for i := 0; i < n; i++ {
		goPaths[i] = C.GoString(cPaths[i])
	}
	m, err := reader.MergeFiles(goPaths...)
	if err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(m))
}

// folio_merge_set_info sets the title and author metadata on a merged document.
//
//export folio_merge_set_info
func folio_merge_set_info(mergedH C.uint64_t, title, author *C.char) C.int32_t {
	m, errCode := loadModifier(mergedH)
	if errCode != errOK {
		return errCode
	}
	m.SetInfo(C.GoString(title), C.GoString(author))
	return errOK
}

// folio_merge_add_blank_page adds a blank page with the given dimensions.
//
//export folio_merge_add_blank_page
func folio_merge_add_blank_page(mergedH C.uint64_t, width, height C.double) C.int32_t {
	m, errCode := loadModifier(mergedH)
	if errCode != errOK {
		return errCode
	}
	m.AddBlankPage(float64(width), float64(height))
	return errOK
}

// folio_merge_add_page_with_text adds a page with simple text content.
//
//export folio_merge_add_page_with_text
func folio_merge_add_page_with_text(mergedH C.uint64_t, width, height C.double,
	text *C.char, fontH C.uint64_t, fontSize, x, y C.double) C.int32_t {
	m, errCode := loadModifier(mergedH)
	if errCode != errOK {
		return errCode
	}
	f, errCode := loadStandardFont(fontH)
	if errCode != errOK {
		return errCode
	}
	m.AddPageWithText(float64(width), float64(height), C.GoString(text), f, float64(fontSize), float64(x), float64(y))
	return errOK
}

// folio_merge_save writes the merged document to a file.
//
//export folio_merge_save
func folio_merge_save(mergedH C.uint64_t, path *C.char) C.int32_t {
	m, errCode := loadModifier(mergedH)
	if errCode != errOK {
		return errCode
	}
	if err := m.SaveTo(C.GoString(path)); err != nil {
		return setErr(errIO, err)
	}
	return errOK
}

// folio_merge_write_to_buffer writes the merged document to an in-memory buffer.
//
//export folio_merge_write_to_buffer
func folio_merge_write_to_buffer(mergedH C.uint64_t) C.uint64_t {
	m, errCode := loadModifier(mergedH)
	if errCode != errOK {
		return 0
	}
	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		setLastError(err.Error())
		return 0
	}
	return C.uint64_t(ht.store(newCBuffer(buf.Bytes())))
}

// folio_merge_free removes a merged document handle from the handle table.
//
//export folio_merge_free
func folio_merge_free(mergedH C.uint64_t) {
	ht.delete(uint64(mergedH))
}

func loadModifier(h C.uint64_t) (*reader.Modifier, C.int32_t) {
	v := ht.load(uint64(h))
	if v == nil {
		setLastError("invalid merge handle")
		return nil, errInvalidHandle
	}
	m, ok := v.(*reader.Modifier)
	if !ok {
		setLastError(fmt.Sprintf("handle %d is not a merged document (type %T)", uint64(h), v))
		return nil, errTypeMismatch
	}
	return m, errOK
}
