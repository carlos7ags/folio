// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestToRoman(t *testing.T) {
	tests := []struct {
		n     int
		upper bool
		want  string
	}{
		{1, false, "i"},
		{2, false, "ii"},
		{3, false, "iii"},
		{4, false, "iv"},
		{5, false, "v"},
		{9, false, "ix"},
		{10, false, "x"},
		{14, false, "xiv"},
		{40, false, "xl"},
		{50, false, "l"},
		{90, false, "xc"},
		{100, false, "c"},
		{400, false, "cd"},
		{500, false, "d"},
		{900, false, "cm"},
		{1000, false, "m"},
		{1994, false, "mcmxciv"},
		{3999, false, "mmmcmxcix"},
		{1, true, "I"},
		{4, true, "IV"},
		{14, true, "XIV"},
		{0, false, "0"},       // out of range fallback
		{4000, false, "4000"}, // out of range fallback
	}

	for _, tt := range tests {
		got := toRoman(tt.n, tt.upper)
		if got != tt.want {
			t.Errorf("toRoman(%d, %v) = %q, want %q", tt.n, tt.upper, got, tt.want)
		}
	}
}

func TestToAlpha(t *testing.T) {
	tests := []struct {
		n    int
		base byte
		want string
	}{
		{1, 'a', "a"},
		{2, 'a', "b"},
		{26, 'a', "z"},
		{27, 'a', "aa"},
		{28, 'a', "ab"},
		{52, 'a', "az"},
		{53, 'a', "ba"},
		{1, 'A', "A"},
		{26, 'A', "Z"},
		{27, 'A', "AA"},
	}

	for _, tt := range tests {
		got := toAlpha(tt.n, tt.base)
		if got != tt.want {
			t.Errorf("toAlpha(%d, %q) = %q, want %q", tt.n, string(tt.base), got, tt.want)
		}
	}
}

func TestListRomanMarkers(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		SetStyle(ListOrderedRoman).
		AddItem("First").
		AddItem("Second").
		AddItem("Third").
		AddItem("Fourth")

	lines := l.Layout(400)
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines, got %d", len(lines))
	}

	// Check that markers are roman numerals.
	markers := []string{"i.", "ii.", "iii.", "iv."}
	for i, want := range markers {
		if lines[i].listRef == nil || len(lines[i].listRef.markerWords) == 0 {
			t.Fatalf("line %d: missing marker", i)
		}
		got := lines[i].listRef.markerWords[0].Text
		if got != want {
			t.Errorf("line %d marker = %q, want %q", i, got, want)
		}
	}
}

func TestListUpperRomanMarkers(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		SetStyle(ListOrderedRomanUp).
		AddItem("First")

	lines := l.Layout(400)
	if len(lines) == 0 || lines[0].listRef == nil || len(lines[0].listRef.markerWords) == 0 {
		t.Fatal("expected marker")
	}
	got := lines[0].listRef.markerWords[0].Text
	if got != "I." {
		t.Errorf("marker = %q, want %q", got, "I.")
	}
}

func TestListAlphaMarkers(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		SetStyle(ListOrderedAlpha).
		AddItem("First").
		AddItem("Second").
		AddItem("Third")

	lines := l.Layout(400)
	markers := []string{"a.", "b.", "c."}
	for i, want := range markers {
		if lines[i].listRef == nil || len(lines[i].listRef.markerWords) == 0 {
			t.Fatalf("line %d: missing marker", i)
		}
		got := lines[i].listRef.markerWords[0].Text
		if got != want {
			t.Errorf("line %d marker = %q, want %q", i, got, want)
		}
	}
}

func TestListUpperAlphaMarkers(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		SetStyle(ListOrderedAlphaUp).
		AddItem("First").
		AddItem("Second")

	lines := l.Layout(400)
	markers := []string{"A.", "B."}
	for i, want := range markers {
		if lines[i].listRef == nil || len(lines[i].listRef.markerWords) == 0 {
			t.Fatalf("line %d: missing marker", i)
		}
		got := lines[i].listRef.markerWords[0].Text
		if got != want {
			t.Errorf("line %d marker = %q, want %q", i, got, want)
		}
	}
}
