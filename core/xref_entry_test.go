// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"testing"
)

func TestEncodeXRefStreamEntryFreeHead(t *testing.T) {
	// Free list head: type 0, next free object 0, generation 65535.
	// Matches the traditional xref free head encoding.
	widths := [3]int{1, 4, 2}
	dst := make([]byte, 7)
	err := EncodeXRefStreamEntry(dst, XRefStreamEntry{
		Type:   XRefEntryFree,
		Field2: 0,
		Field3: 65535,
	}, widths)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0xFF, 0xFF}
	if !bytes.Equal(dst, want) {
		t.Errorf("got %x, want %x", dst, want)
	}
}

func TestEncodeXRefStreamEntryInUse(t *testing.T) {
	// In-use object at offset 0x12345 with generation 0.
	widths := [3]int{1, 3, 1}
	dst := make([]byte, 5)
	err := EncodeXRefStreamEntry(dst, XRefStreamEntry{
		Type:   XRefEntryInUse,
		Field2: 0x12345,
		Field3: 0,
	}, widths)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := []byte{0x01, 0x01, 0x23, 0x45, 0x00}
	if !bytes.Equal(dst, want) {
		t.Errorf("got %x, want %x", dst, want)
	}
}

func TestEncodeXRefStreamEntryCompressed(t *testing.T) {
	// Compressed object: in objstm 42 at index 7.
	widths := [3]int{1, 2, 2}
	dst := make([]byte, 5)
	err := EncodeXRefStreamEntry(dst, XRefStreamEntry{
		Type:   XRefEntryCompressed,
		Field2: 42,
		Field3: 7,
	}, widths)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := []byte{0x02, 0x00, 0x2A, 0x00, 0x07}
	if !bytes.Equal(dst, want) {
		t.Errorf("got %x, want %x", dst, want)
	}
}

func TestEncodeXRefStreamEntryRejectsWrongDstLength(t *testing.T) {
	widths := [3]int{1, 4, 2}
	if err := EncodeXRefStreamEntry(make([]byte, 6), XRefStreamEntry{}, widths); err == nil {
		t.Error("expected error for short dst, got nil")
	}
	if err := EncodeXRefStreamEntry(make([]byte, 8), XRefStreamEntry{}, widths); err == nil {
		t.Error("expected error for long dst, got nil")
	}
}

func TestEncodeXRefStreamEntryRejectsOverflow(t *testing.T) {
	// Field 2 width 1 byte, value 256 — must overflow.
	widths := [3]int{1, 1, 1}
	dst := make([]byte, 3)
	err := EncodeXRefStreamEntry(dst, XRefStreamEntry{
		Type:   XRefEntryInUse,
		Field2: 256,
	}, widths)
	if err == nil {
		t.Error("expected overflow error, got nil")
	}
}

func TestEncodeXRefStreamEntryAcceptsBoundary(t *testing.T) {
	// Field 2 width 1 byte, value 255 — must fit exactly.
	widths := [3]int{1, 1, 1}
	dst := make([]byte, 3)
	err := EncodeXRefStreamEntry(dst, XRefStreamEntry{
		Type:   XRefEntryInUse,
		Field2: 255,
		Field3: 0,
	}, widths)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if dst[1] != 0xFF {
		t.Errorf("field 2 byte = %02x, want ff", dst[1])
	}
}

func TestPutUintBEZeroWidth(t *testing.T) {
	// Width 0 only accepts value 0 per §7.5.8.2.
	if err := putUintBE(nil, 0); err != nil {
		t.Errorf("zero in zero width: %v", err)
	}
	if err := putUintBE(nil, 1); err == nil {
		t.Error("expected error for nonzero in zero width")
	}
}

func TestPutUintBEWideValues(t *testing.T) {
	// Make sure 8-byte values round-trip cleanly.
	dst := make([]byte, 8)
	if err := putUintBE(dst, 0x0123456789ABCDEF); err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xAB, 0xCD, 0xEF}
	if !bytes.Equal(dst, want) {
		t.Errorf("got %x, want %x", dst, want)
	}
}
