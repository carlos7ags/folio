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
	got := Helvetica.MeasureString("\u4e16", 10) // 世 (CJK, not in our table)
	expected := 2.78                             // default width 278/1000 * 10
	if math.Abs(got-expected) > 0.001 {
		t.Errorf("expected %.3f, got %.3f", expected, got)
	}
}

func TestMeasureStringSymbolWidths(t *testing.T) {
	// Symbol font now has real width tables. ASCII 'a','b','c' are not in the
	// Symbol encoding, so they fall through to the default width (250).
	got := Symbol.MeasureString("abc", 10)
	expected := 7.5 // 3 * 250/1000 * 10
	if math.Abs(got-expected) > 0.001 {
		t.Errorf("expected %.3f, got %.3f", expected, got)
	}

	// Greek alpha (U+03B1) has width 631 in the Symbol font.
	got2 := Symbol.MeasureString("\u03B1", 10)
	expected2 := 6.31 // 631/1000 * 10
	if math.Abs(got2-expected2) > 0.001 {
		t.Errorf("expected %.3f, got %.3f", expected2, got2)
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
	if k != -70 {
		t.Errorf("expected -70 for Helvetica A-V (per AFM), got %f", k)
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

func TestKernHelveticaBoldHasOwnTable(t *testing.T) {
	// Bold variant now has its own kern table from AFM.
	k := HelveticaBold.Kern('A', 'V')
	if k >= 0 {
		t.Errorf("expected negative kern for Helvetica-Bold A-V, got %f", k)
	}
}

// --- Ascent/Descent tests ---

func TestStandardFontAscent(t *testing.T) {
	tests := []struct {
		font     *Standard
		fontSize float64
		want     float64
	}{
		{Helvetica, 12, 12 * 718.0 / 1000},
		{HelveticaBold, 12, 12 * 718.0 / 1000},
		{TimesRoman, 12, 12 * 683.0 / 1000},
		{Courier, 12, 12 * 629.0 / 1000},
		{Courier, 24, 24 * 629.0 / 1000},
		{Symbol, 10, 10 * 673.0 / 1000},
		{ZapfDingbats, 10, 10 * 677.0 / 1000},
	}
	for _, tt := range tests {
		got := tt.font.Ascent(tt.fontSize)
		if math.Abs(got-tt.want) > 0.001 {
			t.Errorf("%s.Ascent(%g) = %f, want %f", tt.font.Name(), tt.fontSize, got, tt.want)
		}
	}
}

func TestStandardFontDescent(t *testing.T) {
	tests := []struct {
		font     *Standard
		fontSize float64
		want     float64
	}{
		{Helvetica, 12, 12 * 207.0 / 1000},
		{TimesRoman, 12, 12 * 217.0 / 1000},
		{Courier, 12, 12 * 157.0 / 1000},
		{CourierBold, 10, 10 * 142.0 / 1000},
	}
	for _, tt := range tests {
		got := tt.font.Descent(tt.fontSize)
		if math.Abs(got-tt.want) > 0.001 {
			t.Errorf("%s.Descent(%g) = %f, want %f", tt.font.Name(), tt.fontSize, got, tt.want)
		}
	}
}

func TestAllStandardFontsHaveMetrics(t *testing.T) {
	fonts := []*Standard{
		Helvetica, HelveticaBold, HelveticaOblique, HelveticaBoldOblique,
		TimesRoman, TimesBold, TimesItalic, TimesBoldItalic,
		Courier, CourierBold, CourierOblique, CourierBoldOblique,
		Symbol, ZapfDingbats,
	}
	for _, f := range fonts {
		a := f.Ascent(12)
		d := f.Descent(12)
		if a <= 0 {
			t.Errorf("%s: Ascent should be > 0, got %f", f.Name(), a)
		}
		if d <= 0 {
			t.Errorf("%s: Descent should be > 0, got %f", f.Name(), d)
		}
		// Ascent + Descent should not exceed fontSize (physically impossible).
		if a+d > 12 {
			t.Errorf("%s: Ascent(%f) + Descent(%f) = %f > fontSize(12)", f.Name(), a, d, a+d)
		}
	}
}

func TestAscentDescentDifferBetweenFonts(t *testing.T) {
	hAsc := Helvetica.Ascent(100)
	cAsc := Courier.Ascent(100)
	tAsc := TimesRoman.Ascent(100)

	if hAsc == cAsc {
		t.Error("Helvetica and Courier should have different ascent values")
	}
	if hAsc == tAsc {
		t.Error("Helvetica and Times should have different ascent values")
	}

	hDes := Helvetica.Descent(100)
	cDes := Courier.Descent(100)
	if hDes == cDes {
		t.Error("Helvetica and Courier should have different descent values")
	}
}

func TestAscentDescentScaleLinearly(t *testing.T) {
	a10 := Helvetica.Ascent(10)
	a20 := Helvetica.Ascent(20)
	if math.Abs(a20/a10-2.0) > 0.001 {
		t.Errorf("Ascent should scale linearly: 10pt=%f, 20pt=%f, ratio=%f", a10, a20, a20/a10)
	}

	d10 := Helvetica.Descent(10)
	d20 := Helvetica.Descent(20)
	if math.Abs(d20/d10-2.0) > 0.001 {
		t.Errorf("Descent should scale linearly: 10pt=%f, 20pt=%f, ratio=%f", d10, d20, d20/d10)
	}
}

func TestAscentZeroFontSize(t *testing.T) {
	a := Helvetica.Ascent(0)
	d := Helvetica.Descent(0)
	if a != 0 {
		t.Errorf("Ascent at fontSize 0 should be 0, got %f", a)
	}
	if d != 0 {
		t.Errorf("Descent at fontSize 0 should be 0, got %f", d)
	}
}

func TestMeasureStringStandardFontAppliesKerning(t *testing.T) {
	// "AV" has a documented kern pair (-70) in Helvetica. The kerned
	// width must be smaller than the sum of the per-glyph advances.
	unKerned := (float64(helveticaWidths['A']) + float64(helveticaWidths['V'])) / 1000 * 12
	kerned := Helvetica.MeasureString("AV", 12)
	if kerned >= unKerned {
		t.Errorf("expected kerned width (%.4f) < unkerned (%.4f)", kerned, unKerned)
	}
	// Kern delta should equal the value from the kern table.
	expectedDelta := float64(Helvetica.Kern('A', 'V')) / 1000 * 12
	if gotDelta := kerned - unKerned; gotDelta < expectedDelta-0.001 || gotDelta > expectedDelta+0.001 {
		t.Errorf("kern delta = %.4f, want %.4f", gotDelta, expectedDelta)
	}

	// "AB" has no kern pair, so the measurement must equal the unkerned sum.
	unKernedAB := (float64(helveticaWidths['A']) + float64(helveticaWidths['B'])) / 1000 * 12
	kernedAB := Helvetica.MeasureString("AB", 12)
	if math.Abs(kernedAB-unKernedAB) > 0.001 {
		t.Errorf("AB: kerned %.4f != unkerned %.4f (no pair expected)", kernedAB, unKernedAB)
	}
}

func TestMeasureStringEmbeddedFontAppliesKerning(t *testing.T) {
	ttfPath := "/System/Library/Fonts/Supplemental/Arial.ttf"
	data, err := os.ReadFile(ttfPath)
	if err != nil {
		t.Skipf("Arial TTF not available: %v", err)
	}
	face, err := ParseTTF(data)
	if err != nil {
		t.Fatalf("ParseTTF: %v", err)
	}
	ef := NewEmbeddedFont(face)

	// Compute the unkerned advance sum for "AV" in PDF points.
	gidA := face.GlyphIndex('A')
	gidV := face.GlyphIndex('V')
	upem := float64(face.UnitsPerEm())
	unKerned := (float64(face.GlyphAdvance(gidA)) + float64(face.GlyphAdvance(gidV))) / upem * 12

	kerned := ef.MeasureString("AV", 12)
	kernRaw := face.Kern(gidA, gidV)
	if kernRaw == 0 {
		t.Skip("font has no kern entry for A-V; cannot verify kerning-aware measurement")
	}
	if kerned >= unKerned {
		t.Errorf("expected kerned width (%.4f) < unkerned (%.4f); raw kern = %d FUnits",
			kerned, unKerned, kernRaw)
	}
	// Kerned width must equal unkerned + kern contribution in FUnits→points.
	want := unKerned + float64(kernRaw)/upem*12
	if math.Abs(kerned-want) > 0.0001 {
		t.Errorf("kerned measurement %.6f != expected %.6f", kerned, want)
	}

	// Cross-check against the draw-time value used in drawWordEmbedded:
	// EmbeddedFont.Kern returns kern in thousandths of text space; the
	// draw pipeline applies -kern as a TJ adjustment, yielding an
	// effective advance of (unkerned - (-kern)/1000 * fontSize) =
	// unkerned + kern/1000*fontSize. Since Kern scales FUnits to
	// 1/1000 units, this matches the measurement.
	drawTimeKern := ef.Kern('A', 'V')
	drawTimeAdvance := unKerned + drawTimeKern/1000*12
	if math.Abs(kerned-drawTimeAdvance) > 0.0001 {
		t.Errorf("measure-time %.6f disagrees with draw-time advance %.6f",
			kerned, drawTimeAdvance)
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
