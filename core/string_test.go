// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"strings"
	"testing"
	"unicode/utf8"
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

func TestDecodeTextStringPDFDocEncoding(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"caf\xE9", "café"},                     // Latin-1 range
		{"\x80 \x84 \x8D\x8E \x92", "• — “” ™"}, // 0x80–0x9F differ from Latin-1
		{"\xA0", "€"},
		{"\x18\x19\x1A\x1B\x1C\x1D\x1E\x1F", "˘ˇˆ˙˝˛˚˜"},
		{"\x93\x94\x95\x96\x97\x98\x99\x9A", "ﬁﬂŁŒŠŸŽı"},
		{"\x7F\x9F\xAD", "\uFFFD\uFFFD\uFFFD"}, // undefined codes
	}
	for _, tt := range tests {
		got := DecodeTextString(tt.in)
		if got != tt.want {
			t.Errorf("DecodeTextString(%q) = %q, want %q", tt.in, got, tt.want)
		}
		if !utf8.ValidString(got) {
			t.Errorf("DecodeTextString(%q) is not valid UTF-8", tt.in)
		}
	}
	// Raw UTF-8 written by older folio releases is kept as-is.
	if got := DecodeTextString("Información"); got != "Información" {
		t.Errorf("legacy UTF-8: got %q", got)
	}
}

func TestDecodeTextStringUTF16LanguageEscape(t *testing.T) {
	// §7.9.2.2: ESC (00 1B), a 2-byte language code, an optional 2-byte
	// country code, ESC. The escape sequences are dropped.
	raw := "\xFE\xFF" + "\x00\x1Ben" + "US\x00\x1B" + "\x00H\x00i\x00 " +
		"\x00\x1Bja\x00\x1B" + "\x65\xE5\x67\x2C" // 日本
	if got := DecodeTextString(raw); got != "Hi 日本" {
		t.Errorf("got %q, want %q", got, "Hi 日本")
	}
}

func TestIsPDFDocASCII(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want bool
	}{
		{"plain (text)\n", true},
		{"", true},
		{"tilde\x1F", false},
		{"breve\x18", false},
		{"del\x7F", false},
		{"é", false},
	} {
		if got := IsPDFDocASCII(tt.in); got != tt.want {
			t.Errorf("IsPDFDocASCII(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
	// Text with PDFDocEncoding accent codes must round-trip via UTF-16BE.
	in := "x\x1Fy\x7F"
	s := NewPdfTextString(in)
	if !s.IsHex() {
		t.Error("text with 0x1F/0x7F should be encoded as UTF-16BE")
	}
	if got := s.TextString(); got != in {
		t.Errorf("round trip: got %q, want %q", got, in)
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

// ESC is reserved as the language-escape marker in UTF-16 text strings, so
// encoding drops it rather than letting a reader hide the text after it.
func TestTextStringDropsEscape(t *testing.T) {
	if got := NewPdfTextString("before\x1Bafter").TextString(); got != "beforeafter" {
		t.Errorf("round trip = %q, want %q", got, "beforeafter")
	}
}
