// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import "testing"

// BenchmarkMeasureStringStandard covers Standard.MeasureString, including
// grapheme cluster iteration and kern-pair lookups.
func BenchmarkMeasureStringStandard(b *testing.B) {
	s := "AVAILABLE Together Watch AVATAR Toward Wavelength quick fox"
	b.ResetTimer()
	for range b.N {
		Helvetica.MeasureString(s, 12)
	}
}

// BenchmarkMeasureStringEmbedded covers EmbeddedFont.MeasureString on a
// TrueType face, including glyph lookup and grapheme cluster iteration.
func BenchmarkMeasureStringEmbedded(b *testing.B) {
	face, err := LoadFont("testdata/synthetic_cjk.ttf")
	if err != nil {
		b.Fatalf("LoadFont: %v", err)
	}
	ef := NewEmbeddedFont(face)
	s := "中华人民共和国是一个历史悠久的文明古国"
	b.ResetTimer()
	for range b.N {
		ef.MeasureString(s, 12)
	}
}
