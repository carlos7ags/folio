// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"strings"
	"testing"
)

func TestEscapeLiteralStringControlCharNull(t *testing.T) {
	got := EscapeLiteralString(string([]byte{0x00}))
	expected := `\000`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestEscapeLiteralStringControlCharSOH(t *testing.T) {
	got := EscapeLiteralString(string([]byte{0x01}))
	expected := `\001`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestEscapeLiteralStringControlCharUS(t *testing.T) {
	// 0x1F is the last control character before space (0x20)
	got := EscapeLiteralString(string([]byte{0x1F}))
	expected := `\037`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestEscapeLiteralStringMixedControlChars(t *testing.T) {
	// Mix of a normal char, a control char, and a newline
	input := "A" + string([]byte{0x00}) + "\n" + "B"
	got := EscapeLiteralString(input)
	expected := `A\000\nB`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestNewPdfTextStringASCIIStaysLiteral(t *testing.T) {
	s := NewPdfTextString("Reporte de consulta (v2)")
	if s.IsHex() {
		t.Fatal("ASCII text string should stay a literal string")
	}
	if got := writeString(t, s); got != `(Reporte de consulta \(v2\))` {
		t.Errorf("got %q", got)
	}
	if got := s.TextString(); got != "Reporte de consulta (v2)" {
		t.Errorf("TextString = %q", got)
	}
}

func TestNewPdfTextStringNonASCIIIsUTF16BE(t *testing.T) {
	s := NewPdfTextString("Información")
	if !s.IsHex() {
		t.Fatal("non-ASCII text string should be written in hex notation")
	}
	want := "<FEFF0049006E0066006F0072006D00610063006900F3006E>"
	if got := writeString(t, s); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestTextStringRoundTrip(t *testing.T) {
	for _, in := range []string{
		"",
		"plain",
		"Información del Perfil",
		"Categorías de información",
		"Pontuação",
		"Résumé — naïve “quotes”",
		"emoji 😀 and (parens) \\ backslash",
		"日本語",
	} {
		s := NewPdfTextString(in)
		if got := s.TextString(); got != in {
			t.Errorf("round trip %q: got %q", in, got)
		}
		if got := DecodeTextString(s.Text()); got != in {
			t.Errorf("DecodeTextString %q: got %q", in, got)
		}
	}
}

func TestEncodeTextStringUTF16BESurrogatePair(t *testing.T) {
	got := EncodeTextStringUTF16BE("😀")
	want := "\xFE\xFF\xD8\x3D\xDE\x00"
	if got != want {
		t.Errorf("got % X, want % X", got, want)
	}
}

func TestDecodeTextStringPassthroughAndUTF8BOM(t *testing.T) {
	if got := DecodeTextString("abc"); got != "abc" {
		t.Errorf("no BOM: got %q", got)
	}
	if got := DecodeTextString("\xEF\xBB\xBFcañón"); got != "cañón" {
		t.Errorf("UTF-8 BOM: got %q", got)
	}
	// Odd trailing byte after the UTF-16 BOM is dropped.
	if got := DecodeTextString("\xFE\xFF\x00A\x00"); got != "A" {
		t.Errorf("odd length: got %q", got)
	}
}

func writeString(t *testing.T, s *PdfString) string {
	t.Helper()
	var b strings.Builder
	if _, err := s.WriteTo(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}
