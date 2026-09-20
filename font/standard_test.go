// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"bytes"
	"strings"
	"testing"
)

func TestStandardFontCount(t *testing.T) {
	fonts := StandardFonts()
	if len(fonts) != 14 {
		t.Errorf("expected 14 standard fonts, got %d", len(fonts))
	}
}

func TestStandardFontNames(t *testing.T) {
	expected := []string{
		"Helvetica", "Helvetica-Bold", "Helvetica-Oblique", "Helvetica-BoldOblique",
		"Times-Roman", "Times-Bold", "Times-Italic", "Times-BoldItalic",
		"Courier", "Courier-Bold", "Courier-Oblique", "Courier-BoldOblique",
		"Symbol", "ZapfDingbats",
	}
	fonts := StandardFonts()
	for i, f := range fonts {
		if f.Name() != expected[i] {
			t.Errorf("font %d: expected %q, got %q", i, expected[i], f.Name())
		}
	}
}

func TestStandardFontDict(t *testing.T) {
	tests := []struct {
		font     *Standard
		expected string
	}{
		{Helvetica, "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>"},
		{TimesBold, "<< /Type /Font /Subtype /Type1 /BaseFont /Times-Bold /Encoding /WinAnsiEncoding >>"},
		{CourierOblique, "<< /Type /Font /Subtype /Type1 /BaseFont /Courier-Oblique /Encoding /WinAnsiEncoding >>"},
		// Symbolic fonts should not specify an encoding.
		// See PDF 32000-1:2008 p265.
		{Symbol, "<< /Type /Font /Subtype /Type1 /BaseFont /Symbol >>"},
		{ZapfDingbats, "<< /Type /Font /Subtype /Type1 /BaseFont /ZapfDingbats >>"},
	}

	for _, tc := range tests {
		t.Run(tc.font.Name(), func(t *testing.T) {
			var buf bytes.Buffer
			_, err := tc.font.Dict().WriteTo(&buf)
			if err != nil {
				t.Fatalf("WriteTo failed: %v", err)
			}
			got := buf.String()
			if got != tc.expected {
				t.Errorf("expected:\n%s\ngot:\n%s", tc.expected, got)
			}
		})
	}
}

func TestCanEncodeWinAnsi(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"Hello", true},
		{"Hello World 123", true},
		{"€™©®", true},     // these are in WinAnsi (Windows-1252)
		{"世界", false},      // CJK not in WinAnsi
		{"Hello世界", false}, // mixed
		{"", true},         // empty string
	}
	for _, tt := range tests {
		got := CanEncodeWinAnsi(tt.input)
		if got != tt.want {
			t.Errorf("CanEncodeWinAnsi(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestWinAnsiEncode(t *testing.T) {
	// ASCII characters should pass through unchanged.
	got := WinAnsiEncode("Hello")
	if got != "Hello" {
		t.Errorf("expected 'Hello', got %q", got)
	}

	// Empty string.
	got = WinAnsiEncode("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestWinAnsiRoundTrip(t *testing.T) {
	// Characters in WinAnsi should round-trip through encode/decode.
	input := "Hello World!"
	encoded := WinAnsiEncode(input)
	decoded := WinAnsiDecode(encoded)
	if decoded != input {
		t.Errorf("round-trip failed: %q → %q → %q", input, encoded, decoded)
	}
}

func TestIsStandardFont(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"Helvetica", true},
		{"Times-Roman", true},
		{"Courier", true},
		{"Courier-Bold", true},
		{"Symbol", true},
		{"ZapfDingbats", true},
		{"Arial", false},
		{"NotAFont", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsStandardFont(tt.name)
		if got != tt.want {
			t.Errorf("IsStandardFont(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestStandardFontByteWidths(t *testing.T) {
	// Helvetica should return a 256-element width array.
	w := StandardFontByteWidths("Helvetica")
	if w == nil {
		t.Fatal("expected non-nil widths for Helvetica")
	}
	if len(w) != 256 {
		t.Errorf("expected 256 entries, got %d", len(w))
	}
	// 'H' is byte 72 in WinAnsi, should have width 722.
	if w[72] != 722 {
		t.Errorf("expected Helvetica 'H' width 722, got %d", w[72])
	}

	// Non-existent font → nil.
	if StandardFontByteWidths("NotAFont") != nil {
		t.Error("expected nil for unknown font")
	}

	// Symbol has its own width table (not WinAnsi but still byte-indexed).
	sw := StandardFontByteWidths("Symbol")
	if sw == nil {
		t.Log("Symbol returned nil byte widths (acceptable if no byte mapping)")
	}
}

func TestAllStandardFontDictsHaveRequiredKeys(t *testing.T) {
	for _, f := range StandardFonts() {
		t.Run(f.Name(), func(t *testing.T) {
			var buf bytes.Buffer
			_, err := f.Dict().WriteTo(&buf)
			if err != nil {
				t.Fatalf("WriteTo failed: %v", err)
			}
			s := buf.String()
			if !strings.Contains(s, "/Type /Font") {
				t.Error("missing /Type /Font")
			}
			if !strings.Contains(s, "/Subtype /Type1") {
				t.Error("missing /Subtype /Type1")
			}
			if !strings.Contains(s, "/BaseFont /"+f.Name()) {
				t.Errorf("missing /BaseFont /%s", f.Name())
			}
		})
	}
}
