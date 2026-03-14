// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"math"
	"os"
	"testing"
)

func TestMeasureStringHelvetica(t *testing.T) {
	// "Hello" in Helvetica: H=722 e=556 l=222 l=222 o=556 = 2278
	// At 12pt: 2278/1000 * 12 = 27.336
	got := Helvetica.MeasureString("Hello", 12)
	expected := 27.336
	if math.Abs(got-expected) > 0.001 {
		t.Errorf("expected %.3f, got %.3f", expected, got)
	}
}

func TestMeasureStringHelveticaBold(t *testing.T) {
	// "AB" in Helvetica-Bold: A=722 B=722 = 1444
	// At 10pt: 1444/1000 * 10 = 14.44
	got := HelveticaBold.MeasureString("AB", 10)
	expected := 14.44
	if math.Abs(got-expected) > 0.001 {
		t.Errorf("expected %.3f, got %.3f", expected, got)
	}
}

func TestMeasureStringTimesRoman(t *testing.T) {
	// "Hi" in Times-Roman: H=722 i=278 = 1000
	// At 10pt: 1000/1000 * 10 = 10.0
	got := TimesRoman.MeasureString("Hi", 10)
	expected := 10.0
	if math.Abs(got-expected) > 0.001 {
		t.Errorf("expected %.3f, got %.3f", expected, got)
	}
}

func TestMeasureStringCourier(t *testing.T) {
	// Courier is monospaced: each char 600 units
	// "test" = 4 chars → 2400/1000 * 12 = 28.8
	got := Courier.MeasureString("test", 12)
	expected := 28.8
	if math.Abs(got-expected) > 0.001 {
		t.Errorf("expected %.3f, got %.3f", expected, got)
	}
}

func TestMeasureStringCourierBold(t *testing.T) {
	// Courier-Bold also monospaced at 600
	got := CourierBold.MeasureString("abc", 10)
	expected := 18.0 // 3 * 600 / 1000 * 10
	if math.Abs(got-expected) > 0.001 {
		t.Errorf("expected %.3f, got %.3f", expected, got)
	}
}

func TestMeasureStringEmpty(t *testing.T) {
	got := Helvetica.MeasureString("", 12)
	if got != 0 {
		t.Errorf("expected 0 for empty string, got %f", got)
	}
}

func TestMeasureStringSpace(t *testing.T) {
	// Space in Helvetica = 278 units
	got := Helvetica.MeasureString(" ", 10)
	expected := 2.78
	if math.Abs(got-expected) > 0.001 {
		t.Errorf("expected %.3f, got %.3f", expected, got)
	}
}

func TestMeasureStringUnknownCharFallback(t *testing.T) {
	// Characters not in the width table should use the default (key 0)
	// Helvetica default = 278
	got := Helvetica.MeasureString("\u00e9", 10) // é not in our table
	expected := 2.78                             // default width 278/1000 * 10
	if math.Abs(got-expected) > 0.001 {
		t.Errorf("expected %.3f, got %.3f", expected, got)
	}
}

func TestMeasureStringSymbolFallback(t *testing.T) {
	// Symbol has nil widths → fallback to 600 units/char (Courier-like)
	got := Symbol.MeasureString("abc", 10)
	expected := 18.0 // 3 * 600/1000 * 10
	if math.Abs(got-expected) > 0.001 {
		t.Errorf("expected %.3f, got %.3f", expected, got)
	}
}

func TestMeasureStringTextMeasurerInterface(t *testing.T) {
	// Verify *Standard satisfies TextMeasurer
	var m TextMeasurer = Helvetica
	got := m.MeasureString("A", 10)
	expected := 6.67 // 667/1000 * 10
	if math.Abs(got-expected) > 0.001 {
		t.Errorf("expected %.3f, got %.3f", expected, got)
	}
}

func TestMeasureStringEmbeddedFont(t *testing.T) {
	ttfPath := "/System/Library/Fonts/Supplemental/Arial.ttf"
	data, err := os.ReadFile(ttfPath)
	if err != nil {
		t.Skipf("Arial TTF not available: %v", err)
	}

	face, err := ParseTTF(data)
	if err != nil {
		t.Fatalf("ParseTrueType failed: %v", err)
	}

	ef := NewEmbeddedFont(face)

	// Verify EmbeddedFont satisfies TextMeasurer
	var m TextMeasurer = ef

	// MeasureString of empty string should be 0
	if m.MeasureString("", 12) != 0 {
		t.Error("expected 0 for empty string")
	}

	// MeasureString should return positive value for non-empty string
	w := m.MeasureString("Hello", 12)
	if w <= 0 {
		t.Errorf("expected positive width, got %f", w)
	}

	// Wider text should have larger width
	w1 := m.MeasureString("i", 12)
	w2 := m.MeasureString("W", 12)
	if w1 >= w2 {
		t.Errorf("'i' (%.3f) should be narrower than 'W' (%.3f)", w1, w2)
	}
}

func TestMeasureStringFontSize(t *testing.T) {
	// Width should scale linearly with font size
	w10 := Helvetica.MeasureString("Hello", 10)
	w20 := Helvetica.MeasureString("Hello", 20)
	ratio := w20 / w10
	if math.Abs(ratio-2.0) > 0.001 {
		t.Errorf("expected 2x ratio, got %.3f", ratio)
	}
}

// --- Kerning tests ---

func TestKernHelveticaAV(t *testing.T) {
	// A-V is a classic kern pair — should be negative (tighter).
	k := Helvetica.Kern('A', 'V')
	if k >= 0 {
		t.Errorf("expected negative kern for A-V, got %f", k)
	}
	if k != -80 {
		t.Errorf("expected -80 for Helvetica A-V, got %f", k)
	}
}

func TestKernHelveticaNoPair(t *testing.T) {
	// 'x' + 'z' has no kerning pair → should return 0.
	k := Helvetica.Kern('x', 'z')
	if k != 0 {
		t.Errorf("expected 0 for non-kerned pair, got %f", k)
	}
}

func TestKernTimesRoman(t *testing.T) {
	k := TimesRoman.Kern('A', 'V')
	if k >= 0 {
		t.Errorf("expected negative kern for Times A-V, got %f", k)
	}
}

func TestKernCourierNoKerning(t *testing.T) {
	// Courier (monospaced) has no kerning pairs.
	k := Courier.Kern('A', 'V')
	if k != 0 {
		t.Errorf("expected 0 for Courier (monospaced), got %f", k)
	}
}

func TestKernHelveticaBoldSameAsRegular(t *testing.T) {
	// Bold variant shares the same kern table.
	k1 := Helvetica.Kern('T', 'o')
	k2 := HelveticaBold.Kern('T', 'o')
	if k1 != k2 {
		t.Errorf("expected same kern for Helvetica and Bold: %f vs %f", k1, k2)
	}
}

func TestKernEmbeddedFont(t *testing.T) {
	ttfPath := "/System/Library/Fonts/Supplemental/Arial.ttf"
	data, err := os.ReadFile(ttfPath)
	if err != nil {
		t.Skipf("Arial TTF not available: %v", err)
	}

	face, err := ParseTTF(data)
	if err != nil {
		t.Fatalf("ParseTTF failed: %v", err)
	}

	ef := NewEmbeddedFont(face)
	// Just check it doesn't panic and returns a value.
	_ = ef.Kern('A', 'V')
	// No kern pair should return 0.
	k := ef.Kern('x', 'z')
	if k != 0 {
		t.Errorf("expected 0 for non-kerned pair, got %f", k)
	}
}
