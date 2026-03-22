// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

/*
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
*/
import "C"
import "unsafe"

// cBuffer holds C-allocated memory for returning byte data to callers.
// The data pointer is allocated via C.malloc and must be freed with C.free.
type cBuffer struct {
	ptr unsafe.Pointer
	len int
}

// newCBuffer copies Go bytes into C-allocated memory.
func newCBuffer(data []byte) *cBuffer {
	n := len(data)
	if n == 0 {
		return &cBuffer{ptr: nil, len: 0}
	}
	ptr := C.malloc(C.size_t(n))
	C.memcpy(ptr, unsafe.Pointer(&data[0]), C.size_t(n))
	return &cBuffer{ptr: ptr, len: n}
}

// folio_buffer_data returns a pointer to the raw bytes of a buffer handle.
//
//export folio_buffer_data
func folio_buffer_data(buf C.uint64_t) unsafe.Pointer {
	v := ht.load(uint64(buf))
	if v == nil {
		return nil
	}
	b, ok := v.(*cBuffer)
	if !ok {
		return nil
	}
	return b.ptr
}

// folio_buffer_len returns the byte length of a buffer handle.
//
//export folio_buffer_len
func folio_buffer_len(buf C.uint64_t) C.int32_t {
	v := ht.load(uint64(buf))
	if v == nil {
		return 0
	}
	b, ok := v.(*cBuffer)
	if !ok {
		return 0
	}
	return C.int32_t(b.len)
}

// folio_buffer_free releases the C-allocated memory and removes the buffer handle.
//
//export folio_buffer_free
func folio_buffer_free(buf C.uint64_t) {
	v := ht.load(uint64(buf))
	if v != nil {
		if b, ok := v.(*cBuffer); ok && b.ptr != nil {
			C.free(b.ptr)
		}
	}
	ht.delete(uint64(buf))
}
