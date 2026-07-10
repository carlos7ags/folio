// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

// Package export provides a C ABI for the Folio PDF library.
// Every Go object exposed to C is stored in an opaque handle table.
// C callers receive uint64 handle IDs and never see raw Go pointers.
//
// Error convention: all functions return int32 (0 = success, negative = error).
// Call folio_last_error() to retrieve the error message string.
package main

/*
#include <stdint.h>
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"sync"
	"unsafe"
)

// version is the library version, injected at build time via:
//
//	-ldflags "-X main.version=v1.2.3"
//
// Falls back to "dev" for local development builds.
var version = "dev"

// versionCStr is a persistent C string for folio_version().
// Allocated once, never freed — avoids per-call memory leaks.
var versionCStr *C.char

func init() {
	versionCStr = C.CString(version)
}

// Error codes returned by C ABI functions.
const (
	errOK            = 0
	errInvalidHandle = -1
	errInvalidArg    = -2
	errIO            = -3
	errPDF           = -4
	errTypeMismatch  = -5
	errInternalError = -6
)

// handleTable stores Go objects keyed by uint64 handle IDs.
// Handle 0 is reserved as the null/invalid handle.
type handleTable struct {
	mu      sync.Mutex
	handles map[uint64]any
	next    uint64
}

// ht is the global handle table shared by all C ABI functions.
var ht = &handleTable{
	handles: make(map[uint64]any),
	next:    1,
}

// store adds a value to the handle table and returns its handle ID.
func (t *handleTable) store(v any) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	id := t.next
	t.next++
	t.handles[id] = v
	return id
}

// load retrieves a value by handle ID. Returns nil if not found.
func (t *handleTable) load(id uint64) any {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.handles[id]
}

// delete removes a handle. Returns true if it existed.
func (t *handleTable) delete(id uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.handles[id]
	if ok {
		delete(t.handles, id)
	}
	return ok
}

// count returns the number of live handles (for testing).
func (t *handleTable) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.handles)
}

// lastError stores the most recent error message per-thread (approximated
// via a single global since cgo serializes calls through a single OS thread
// by default). The returned C string is valid until the next C ABI call.
var (
	lastErrorMu  sync.Mutex
	lastErrorMsg *C.char
)

// setLastError stores an error message retrievable via folio_last_error.
func setLastError(msg string) {
	lastErrorMu.Lock()
	defer lastErrorMu.Unlock()
	if lastErrorMsg != nil {
		C.free(unsafe.Pointer(lastErrorMsg))
	}
	lastErrorMsg = C.CString(msg)
}

// clearLastError clears any previous error.
func clearLastError() {
	lastErrorMu.Lock()
	defer lastErrorMu.Unlock()
	if lastErrorMsg != nil {
		C.free(unsafe.Pointer(lastErrorMsg))
		lastErrorMsg = nil
	}
}

// setErr sets the last error from an error value and returns the error code.
func setErr(code C.int32_t, err error) C.int32_t {
	if err != nil {
		setLastError(err.Error())
	}
	return code
}

// maxCArrayCount caps (pointer, count) array arguments accepted across the
// C ABI. Counts above this are rejected instead of risking a crash.
const maxCArrayCount = 1 << 20

// checkCArray validates a (pointer, count) argument pair. On failure it sets
// the last error and returns false.
func checkCArray(ptr unsafe.Pointer, n int) bool {
	if n < 0 || n > maxCArrayCount {
		setLastError(fmt.Sprintf("array count %d out of range [0, %d]", n, maxCArrayCount))
		return false
	}
	if n > 0 && ptr == nil {
		setLastError("nil array pointer with non-zero count")
		return false
	}
	return true
}

// folio_version returns the library version as a C string.
//
//export folio_version
func folio_version() *C.char {
	return versionCStr
}

// folio_last_error returns the most recent error message, or nil if none.
//
//export folio_last_error
func folio_last_error() *C.char {
	lastErrorMu.Lock()
	defer lastErrorMu.Unlock()
	return lastErrorMsg
}

func main() {}
