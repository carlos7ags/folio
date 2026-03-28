// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"testing"
)

func TestWinAnsiEncodingASCII(t *testing.T) {
	got := winAnsiEncoding.Decode([]byte("Hello, World!"))
	if got != "Hello, World!" {
		t.Errorf("WinAnsi ASCII = %q, want %q", got, "Hello, World!")
	}
}

func TestWinAnsiEncodingSpecial(t *testing.T) {
	// Byte 0x93 = left double quote, 0x94 = right double quote in Windows-1252.
	got := winAnsiEncoding.Decode([]byte{0x93, 'H', 'i', 0x94})
	if got != "\u201CHi\u201D" {
		t.Errorf("WinAnsi special = %q (%U), want \\u201CHi\\u201D", got, []rune(got))
	}
}

func TestWinAnsiEncodingEuro(t *testing.T) {
	got := winAnsiEncoding.Decode([]byte{0x80})
	if got != "€" {
		t.Errorf("WinAnsi Euro = %q, want €", got)
	}
}

func TestMacRomanEncodingASCII(t *testing.T) {
	got := macRomanEncoding.Decode([]byte("Test"))
	if got != "Test" {
		t.Errorf("MacRoman ASCII = %q", got)
	}
}

func TestMacRomanEncodingSpecial(t *testing.T) {
	// Mac Roman 0x8A = ä (Adieresis), 0xC4 = ƒ (florin)
	got := macRomanEncoding.Decode([]byte{0x8A})
	if got != "ä" {
		t.Errorf("MacRoman 0x8A = %q, want ä", got)
	}
}

func TestStandardEncodingBasic(t *testing.T) {
	got := standardEncoding.Decode([]byte("ABC"))
	if got != "ABC" {
		t.Errorf("Standard = %q", got)
	}
}

func TestGlyphToRune(t *testing.T) {
	tests := []struct {
		name string
		want rune
	}{
		{"A", 'A'},
		{"space", ' '},
		{"endash", 0x2013},
		{"fi", 0xFB01},
		{"Euro", 0x20AC},
		{"uni0041", 'A'},
		{"uni20AC", 0x20AC},
		{"nonexistent", 0},
	}
	for _, tc := range tests {
		got := glyphToRune(tc.name)
		if got != tc.want {
			t.Errorf("glyphToRune(%q) = %U, want %U", tc.name, got, tc.want)
		}
	}
}

func TestEncodingDecodeZero(t *testing.T) {
	// Byte 0 should not produce output (maps to rune 0 = null).
	got := winAnsiEncoding.Decode([]byte{0, 'A', 0})
	if got != "A" {
		t.Errorf("zero bytes = %q, want %q", got, "A")
	}
}
