// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cgo && !js && !wasm

package main

import (
	"sync"
	"testing"
	"unsafe"
)

func newTestHandleTable() *handleTable {
	return &handleTable{handles: make(map[uint64]any), next: 1}
}

func TestHandleTableStoreLoad(t *testing.T) {
	ht := newTestHandleTable()
	id1 := ht.store("a")
	id2 := ht.store("b")

	if id1 == 0 || id2 == 0 {
		t.Fatalf("store returned a zero handle: id1=%d id2=%d", id1, id2)
	}
	if id1 == id2 {
		t.Fatalf("store returned duplicate handles: %d", id1)
	}
	if got := ht.load(id1); got != "a" {
		t.Errorf("load(id1) = %v, want %q", got, "a")
	}
	if got := ht.load(id2); got != "b" {
		t.Errorf("load(id2) = %v, want %q", got, "b")
	}
}

func TestHandleTableHandleZeroInvalid(t *testing.T) {
	ht := newTestHandleTable()
	if got := ht.load(0); got != nil {
		t.Errorf("load(0) = %v, want nil", got)
	}
	if ok := ht.delete(0); ok {
		t.Errorf("delete(0) = true, want false")
	}
}

func TestHandleTableUnknownID(t *testing.T) {
	ht := newTestHandleTable()
	if got := ht.load(12345); got != nil {
		t.Errorf("load(12345) = %v, want nil", got)
	}
	if ok := ht.delete(12345); ok {
		t.Errorf("delete(12345) = true, want false")
	}
}

func TestHandleTableDeleteTwice(t *testing.T) {
	ht := newTestHandleTable()
	id := ht.store("x")

	if ok := ht.delete(id); !ok {
		t.Fatalf("first delete(%d) = false, want true", id)
	}
	if ok := ht.delete(id); ok {
		t.Errorf("second delete(%d) = true, want false", id)
	}
	if got := ht.load(id); got != nil {
		t.Errorf("load after delete = %v, want nil", got)
	}
}

func TestHandleTableCount(t *testing.T) {
	ht := newTestHandleTable()
	if n := ht.count(); n != 0 {
		t.Fatalf("count() on empty table = %d, want 0", n)
	}

	id1 := ht.store("a")
	id2 := ht.store("b")
	if n := ht.count(); n != 2 {
		t.Errorf("count() after two stores = %d, want 2", n)
	}

	ht.delete(id1)
	if n := ht.count(); n != 1 {
		t.Errorf("count() after one delete = %d, want 1", n)
	}

	ht.delete(id2)
	if n := ht.count(); n != 0 {
		t.Errorf("count() after all deleted = %d, want 0", n)
	}
}

func TestHandleTableConcurrent(t *testing.T) {
	ht := newTestHandleTable()
	const goroutines = 8
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				id := ht.store(i)
				if got := ht.load(id); got != i {
					t.Errorf("load(%d) = %v, want %d", id, got, i)
				}
				if ok := ht.delete(id); !ok {
					t.Errorf("delete(%d) = false, want true", id)
				}
			}
		}()
	}
	wg.Wait()

	if n := ht.count(); n != 0 {
		t.Errorf("count() after concurrent store/load/delete = %d, want 0", n)
	}
}

func TestNewCBufferEmpty(t *testing.T) {
	for _, data := range [][]byte{nil, {}} {
		b := newCBuffer(data)
		if b.ptr != nil {
			t.Errorf("newCBuffer(%v).ptr = %v, want nil", data, b.ptr)
		}
		if b.len != 0 {
			t.Errorf("newCBuffer(%v).len = %d, want 0", data, b.len)
		}
	}
}

func TestNewCBufferNonEmpty(t *testing.T) {
	want := "hello"
	b := newCBuffer([]byte(want))
	// C.free is unreachable from _test.go; the 5-byte leak is confined to
	// the test binary's lifetime.
	if b.len != len(want) {
		t.Fatalf("len = %d, want %d", b.len, len(want))
	}
	if b.ptr == nil {
		t.Fatalf("ptr is nil for non-empty data")
	}
	got := unsafe.Slice((*byte)(b.ptr), b.len)
	if string(got) != want {
		t.Errorf("copied bytes = %q, want %q", got, want)
	}
}

func TestSetClearLastError(t *testing.T) {
	// No C-side readback is possible here (folio_last_error returns
	// *C.char); this only exercises the store/clear/overwrite paths
	// under -race. The C harness verifies the actual message contents.
	setLastError("boom")
	clearLastError()
	setLastError("x")
	setLastError("x")
}

func TestErrorCodes(t *testing.T) {
	// Pins the numeric contract FFI callers depend on. Renumbering any of
	// these is an ABI break.
	cases := []struct {
		name string
		code int32
		want int32
	}{
		{"errOK", errOK, 0},
		{"errInvalidHandle", errInvalidHandle, -1},
		{"errInvalidArg", errInvalidArg, -2},
		{"errIO", errIO, -3},
		{"errPDF", errPDF, -4},
		{"errTypeMismatch", errTypeMismatch, -5},
		{"errInternalError", errInternalError, -6},
	}
	for _, c := range cases {
		if c.code != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.code, c.want)
		}
	}
}
