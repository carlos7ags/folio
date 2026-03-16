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
		{Symbol, "<< /Type /Font /Subtype /Type1 /BaseFont /Symbol /Encoding /WinAnsiEncoding >>"},
		{ZapfDingbats, "<< /Type /Font /Subtype /Type1 /BaseFont /ZapfDingbats /Encoding /WinAnsiEncoding >>"},
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
