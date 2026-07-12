// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"os"
	"testing"
)

// TestNormalLineHeightAcrossFonts is a fast (no Chrome/poppler needed)
// regression guard for FOLIO-GAP-01 across multiple font families and both
// TTF/glyf (Roboto, Open Sans, Noto Sans, Inter*) and OTF/CFF
// (NimbusSans, Inter) outlines, so a fix that only happens to work for
// Poppins's particular metrics doesn't regress another font. The
// authoritative cross-check against a real browser lives in the
// folio-repro parity harness (gap-01/gap-02 cases per font); this test
// just guards the arithmetic in isolation.
func TestNormalLineHeightAcrossFonts(t *testing.T) {
	files := []string{
		"Poppins-Bold.ttf",
		"NimbusSans-Regular.otf",
		"NimbusSans-Bold.otf",
		"Inter-Regular.otf",
		"Inter-Bold.otf",
		"NotoSans-Regular.ttf",
		"NotoSans-Bold.ttf",
		"OpenSans-Regular.ttf",
		"OpenSans-Bold.ttf",
		"Roboto-Regular.ttf",
		"Roboto-Bold.ttf",
	}

	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/" + name)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			face, err := ParseFont(data)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			ef := NewEmbeddedFont(face)

			const fontSize = 11.0
			nlh := ef.NormalLineHeight(fontSize)
			if nlh <= 0 {
				t.Fatalf("NormalLineHeight(%v) = %v, want > 0", fontSize, nlh)
			}

			// Sanity bound: normal leading is always somewhat larger than
			// the em size but not absurdly so. A wrong scale (e.g. missing
			// /UnitsPerEm division, or reading the wrong table) would blow
			// past this by an order of magnitude.
			ratio := nlh / fontSize
			if ratio < 0.8 || ratio > 3.0 {
				t.Errorf("NormalLineHeight/fontSize ratio = %.3f, want within [0.8, 3.0]", ratio)
			}

			// Must scale linearly with font size.
			doubled := ef.NormalLineHeight(fontSize * 2)
			if diff := doubled - nlh*2; diff > 0.01 || diff < -0.01 {
				t.Errorf("NormalLineHeight(2x) = %v, want ~%v (2x linear scaling)", doubled, nlh*2)
			}

			// MeasureString must be positive and monotonically
			// non-decreasing as text grows.
			short := ef.MeasureString("Hello", fontSize)
			long := ef.MeasureString("Hello World", fontSize)
			if short <= 0 {
				t.Errorf("MeasureString(%q) = %v, want > 0", "Hello", short)
			}
			if long <= short {
				t.Errorf("MeasureString(%q) = %v, want > MeasureString(%q) = %v", "Hello World", long, "Hello", short)
			}
		})
	}
}
