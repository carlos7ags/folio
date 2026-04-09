// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"
)

// TestShapeArabicIsolated verifies that a single Arabic letter gets its
// isolated presentation form.
func TestShapeArabicIsolated(t *testing.T) {
	// Beh (U+0628) isolated → U+FE8F
	shaped := ShapeArabic("\u0628")
	if shaped != "\uFE8F" {
		t.Errorf("single beh: got %U, want U+FE8F", []rune(shaped))
	}
}

// TestShapeArabicTwoLetterWord verifies initial+final forms for a
// two-letter connected word.
func TestShapeArabicTwoLetterWord(t *testing.T) {
	// Beh (D) + Alef (R) → initial beh + final alef
	// Beh initial = U+FE91, Alef final = U+FE8E
	shaped := ShapeArabic("\u0628\u0627")
	runes := []rune(shaped)
	if len(runes) != 2 {
		t.Fatalf("expected 2 runes, got %d: %U", len(runes), runes)
	}
	if runes[0] != 0xFE91 {
		t.Errorf("beh: got %U, want U+FE91 (initial)", runes[0])
	}
	if runes[1] != 0xFE8E {
		t.Errorf("alef: got %U, want U+FE8E (final)", runes[1])
	}
}

// TestShapeArabicMedialForm verifies that a letter between two joining
// neighbors gets its medial form.
func TestShapeArabicMedialForm(t *testing.T) {
	// Beh (D) + Seen (D) + Meem (D) → initial beh + medial seen + final meem
	shaped := ShapeArabic("\u0628\u0633\u0645")
	runes := []rune(shaped)
	if len(runes) != 3 {
		t.Fatalf("expected 3 runes, got %d: %U", len(runes), runes)
	}
	// Beh initial = FE91, Seen medial = FEB4, Meem final = FEE2
	if runes[0] != 0xFE91 {
		t.Errorf("beh: got %U, want U+FE91 (initial)", runes[0])
	}
	if runes[1] != 0xFEB4 {
		t.Errorf("seen: got %U, want U+FEB4 (medial)", runes[1])
	}
	if runes[2] != 0xFEE2 {
		t.Errorf("meem: got %U, want U+FEE2 (final)", runes[2])
	}
}

// TestShapeArabicRightJoiningBreaksChain verifies that a right-joining
// character (alef) breaks the forward joining chain: the character
// after alef starts a new initial form.
func TestShapeArabicRightJoiningBreaksChain(t *testing.T) {
	// Beh (D) + Alef (R) + Beh (D) → initial beh + final alef + isolated beh
	// Alef is R (joins right only), so it can't pass joining to the left.
	// The second beh starts a new context with no right neighbor → isolated.
	shaped := ShapeArabic("\u0628\u0627\u0628")
	runes := []rune(shaped)
	if len(runes) != 3 {
		t.Fatalf("expected 3 runes, got %d: %U", len(runes), runes)
	}
	if runes[0] != 0xFE91 {
		t.Errorf("first beh: got %U, want U+FE91 (initial)", runes[0])
	}
	if runes[1] != 0xFE8E {
		t.Errorf("alef: got %U, want U+FE8E (final)", runes[1])
	}
	if runes[2] != 0xFE8F {
		t.Errorf("second beh: got %U, want U+FE8F (isolated)", runes[2])
	}
}

// TestShapeArabicLamAlef verifies the lam-alef ligature formation.
func TestShapeArabicLamAlef(t *testing.T) {
	// Lam (U+0644) + Alef (U+0627) → ligature U+FEFB (isolated lam-alef)
	shaped := ShapeArabic("\u0644\u0627")
	runes := []rune(shaped)
	if len(runes) != 1 {
		t.Fatalf("lam-alef should produce 1 rune, got %d: %U", len(runes), runes)
	}
	if runes[0] != 0xFEFB {
		t.Errorf("lam-alef: got %U, want U+FEFB", runes[0])
	}
}

// TestShapeArabicLamAlefFinal verifies the final form of lam-alef
// ligature when lam joins to its right neighbor.
func TestShapeArabicLamAlefFinal(t *testing.T) {
	// Beh + Lam + Alef → initial beh + final lam-alef ligature (U+FEFC)
	shaped := ShapeArabic("\u0628\u0644\u0627")
	runes := []rune(shaped)
	if len(runes) != 2 {
		t.Fatalf("expected 2 runes (beh + lam-alef lig), got %d: %U", len(runes), runes)
	}
	if runes[0] != 0xFE91 {
		t.Errorf("beh: got %U, want U+FE91 (initial)", runes[0])
	}
	if runes[1] != 0xFEFC {
		t.Errorf("lam-alef: got %U, want U+FEFC (final)", runes[1])
	}
}

// TestShapeArabicFarsiPeh verifies Farsi peh (U+067E) shaping.
func TestShapeArabicFarsiPeh(t *testing.T) {
	// Peh isolated = U+FB56
	shaped := ShapeArabic("\u067E")
	if shaped != "\uFB56" {
		t.Errorf("peh isolated: got %U, want U+FB56", []rune(shaped))
	}
}

// TestShapeArabicSalam verifies the full word "سلام" (salam = peace).
func TestShapeArabicSalam(t *testing.T) {
	// سلام = Seen + Lam + Alef + Meem
	// Lam + Alef → lam-alef ligature
	// Seen: joins left (to lam-alef lig) → initial
	// Lam-alef: joins right (from seen), joins left (to meem)? No — lam-alef
	// is a ligature that replaces lam+alef; it functions as a right-joining
	// character (like alef). So:
	// Seen initial + final lam-alef + isolated meem? No...
	//
	// Actually: Seen(D)+Lam(D)+Alef(R)+Meem(D)
	// Lam-alef merges lam+alef into one glyph. After merge: Seen + LamAlef + Meem
	// Seen joins left → initial. LamAlef lig: in the forms table? No, the
	// ligature is a single codepoint that doesn't participate in further
	// joining. Meem has no right neighbor that joins left → isolated.
	//
	// Wait — after lam-alef merge, the joining context is:
	// Seen(D) | LamAlefLig | Meem(D)
	// The lig is not in getJoiningType → jtNone → breaks chain.
	// So: Seen initial, Meem isolated. That's correct Arabic rendering.
	shaped := ShapeArabic("\u0633\u0644\u0627\u0645")
	runes := []rune(shaped)
	if len(runes) != 3 {
		t.Fatalf("expected 3 runes after lam-alef merge, got %d: %U", len(runes), runes)
	}
	// Seen initial = FEB3
	if runes[0] != 0xFEB3 {
		t.Errorf("seen: got %U, want U+FEB3 (initial)", runes[0])
	}
	// Lam-alef final (because seen joins left) = FEFC
	if runes[1] != 0xFEFC {
		t.Errorf("lam-alef: got %U, want U+FEFC (final)", runes[1])
	}
}

// TestShapeArabicPassesNonArabic verifies that non-Arabic text passes
// through unchanged.
func TestShapeArabicPassesNonArabic(t *testing.T) {
	tests := []string{
		"Hello world",
		"12345",
		"",
		"Hello \u05E9\u05DC\u05D5\u05DD world", // Hebrew is not shaped
	}
	for _, s := range tests {
		if got := ShapeArabic(s); got != s {
			t.Errorf("ShapeArabic(%q) = %q, want unchanged", s, got)
		}
	}
}

// TestShapeArabicWithDiacritics verifies that transparent diacritics
// (tashkeel) don't break the joining chain.
func TestShapeArabicWithDiacritics(t *testing.T) {
	// Beh + Fathah (U+064E, transparent) + Alef
	// The diacritic should not affect joining: beh joins to alef.
	shaped := ShapeArabic("\u0628\u064E\u0627")
	runes := []rune(shaped)
	if len(runes) != 3 {
		t.Fatalf("expected 3 runes (beh + fathah + alef), got %d: %U", len(runes), runes)
	}
	// Beh should be initial (joining to alef through transparent fathah)
	if runes[0] != 0xFE91 {
		t.Errorf("beh: got %U, want U+FE91 (initial, joining through diacritic)", runes[0])
	}
	// Fathah passes through as-is
	if runes[1] != 0x064E {
		t.Errorf("fathah: got %U, want U+064E (unchanged)", runes[1])
	}
	// Alef should be final (joining from beh through transparent fathah)
	if runes[2] != 0xFE8E {
		t.Errorf("alef: got %U, want U+FE8E (final)", runes[2])
	}
}
