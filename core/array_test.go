// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

func TestArrayAddPanicsOnNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when adding nil to PdfArray")
		}
	}()
	a := NewPdfArray()
	a.Add(nil)
}

func TestNewPdfArrayPanicsOnNilElement(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when constructing PdfArray with nil element")
		}
	}()
	NewPdfArray(NewPdfInteger(1), nil, NewPdfInteger(3))
}

func TestArrayAtAndAll(t *testing.T) {
	a := NewPdfArray(
		NewPdfInteger(10),
		NewPdfInteger(20),
		NewPdfInteger(30),
	)
	if a.At(1).(*PdfNumber).IntValue() != 20 {
		t.Errorf("At(1): expected 20")
	}

	var collected []int
	for i, e := range a.All() {
		_ = i
		collected = append(collected, e.(*PdfNumber).IntValue())
	}
	want := []int{10, 20, 30}
	for i, v := range want {
		if collected[i] != v {
			t.Errorf("All()[%d]: expected %d, got %d", i, v, collected[i])
		}
	}
}

func TestArraySet(t *testing.T) {
	a := NewPdfArray(
		NewPdfInteger(10),
		NewPdfInteger(20),
		NewPdfInteger(30),
	)
	a.Set(1, NewPdfInteger(99))
	if a.At(1).(*PdfNumber).IntValue() != 99 {
		t.Errorf("after Set(1, 99): expected 99 at index 1")
	}
	if a.At(0).(*PdfNumber).IntValue() != 10 {
		t.Errorf("Set should not affect other indices")
	}
	if a.Len() != 3 {
		t.Errorf("Set should not change length")
	}
}

func TestArraySetPanicsOnNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when Set receives nil")
		}
	}()
	a := NewPdfArray(NewPdfInteger(1))
	a.Set(0, nil)
}

func TestArraySetPanicsOutOfRange(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when Set index out of range")
		}
	}()
	a := NewPdfArray(NewPdfInteger(1))
	a.Set(5, NewPdfInteger(2))
}
