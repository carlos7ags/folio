// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

func TestDictionarySetPanicsOnNilValue(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when setting nil value in PdfDictionary")
		}
	}()
	d := NewPdfDictionary()
	d.Set("Key", nil)
}

func TestDictionaryRemoveExisting(t *testing.T) {
	d := NewPdfDictionary()
	d.Set("A", NewPdfInteger(1))
	d.Set("B", NewPdfInteger(2))
	d.Set("C", NewPdfInteger(3))
	d.Remove("B")
	if d.Get("B") != nil {
		t.Error("B should be gone after Remove")
	}
	if d.Get("A") == nil || d.Get("C") == nil {
		t.Error("A and C should still be present")
	}
	got := serialize(t, d)
	if got != "<< /A 1 /C 3 >>" {
		t.Errorf("expected %q, got %q", "<< /A 1 /C 3 >>", got)
	}
}

func TestDictionaryRemoveMissing(t *testing.T) {
	d := NewPdfDictionary()
	d.Set("A", NewPdfInteger(1))
	d.Remove("NotThere") // should be a no-op, not panic
	if d.Get("A") == nil {
		t.Error("existing entry should not be affected by removing missing key")
	}
}

func TestDictionaryRemoveOnly(t *testing.T) {
	d := NewPdfDictionary()
	d.Set("X", NewPdfInteger(42))
	d.Remove("X")
	if d.Len() != 0 {
		t.Errorf("expected empty dictionary, got %d entries", d.Len())
	}
	got := serialize(t, d)
	if got != "<< >>" {
		t.Errorf("expected %q, got %q", "<< >>", got)
	}
}

func TestDictionaryAllIterator(t *testing.T) {
	d := NewPdfDictionary()
	d.Set("B", NewPdfInteger(2))
	d.Set("A", NewPdfInteger(1))
	d.Set("C", NewPdfInteger(3))

	var keys []string
	for k, v := range d.All() {
		_ = v
		keys = append(keys, k)
	}
	// Must preserve insertion order.
	want := []string{"B", "A", "C"}
	if len(keys) != len(want) {
		t.Fatalf("expected %d keys, got %d", len(want), len(keys))
	}
	for i, k := range want {
		if keys[i] != k {
			t.Errorf("key[%d]: expected %q, got %q", i, k, keys[i])
		}
	}
}

func TestDictionaryIndexAfterRemove(t *testing.T) {
	d := NewPdfDictionary()
	d.Set("A", NewPdfInteger(1))
	d.Set("B", NewPdfInteger(2))
	d.Set("C", NewPdfInteger(3))
	d.Remove("A")
	// Both B and C should still be findable via the index after Remove
	// shifted their positions down.
	if b := d.Get("B"); b == nil || b.(*PdfNumber).IntValue() != 2 {
		t.Errorf("B lookup broken after Remove: %v", b)
	}
	if c := d.Get("C"); c == nil || c.(*PdfNumber).IntValue() != 3 {
		t.Errorf("C lookup broken after Remove: %v", c)
	}
}
