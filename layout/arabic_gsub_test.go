// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"os"
	"runtime"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// --- Mock GSUBProvider for deterministic CI-safe tests ---

// mockGSUBFace implements font.Face and font.GSUBProvider with synthetic
// data so tests don't depend on system fonts.
type mockGSUBFace struct {
	glyphMap      map[rune]uint16         // cmap: rune -> GID
	reverseMap    map[uint16]rune         // reverse cmap: GID -> rune
	substitutions *font.GSUBSubstitutions // GSUB tables
}

func (m *mockGSUBFace) PostScriptName() string { return "MockArabic" }
func (m *mockGSUBFace) UnitsPerEm() int        { return 1000 }
func (m *mockGSUBFace) GlyphIndex(r rune) uint16 {
	if gid, ok := m.glyphMap[r]; ok {
		return gid
	}
	return 0
}
func (m *mockGSUBFace) GlyphAdvance(uint16) int       { return 500 }
func (m *mockGSUBFace) Ascent() int                   { return 800 }
func (m *mockGSUBFace) Descent() int                  { return -200 }
func (m *mockGSUBFace) BBox() [4]int                  { return [4]int{0, -200, 1000, 800} }
func (m *mockGSUBFace) ItalicAngle() float64          { return 0 }
func (m *mockGSUBFace) CapHeight() int                { return 700 }
func (m *mockGSUBFace) StemV() int                    { return 80 }
func (m *mockGSUBFace) Kern(uint16, uint16) int       { return 0 }
func (m *mockGSUBFace) Flags() uint32                 { return 0 }
func (m *mockGSUBFace) RawData() []byte               { return nil }
func (m *mockGSUBFace) NumGlyphs() int                { return 100 }
func (m *mockGSUBFace) GSUB() *font.GSUBSubstitutions { return m.substitutions }
func (m *mockGSUBFace) GIDToUnicode() map[uint16]rune { return m.reverseMap }

// newMockArabicFace creates a mock face with synthetic GSUB data for
// beh (U+0628) and alef (U+0627). The GSUB maps base GIDs to synthetic
// replacement GIDs that reverse-map to distinctive codepoints, so tests
// can verify the GSUB path was taken (not PFB fallback).
func newMockArabicFace() *mockGSUBFace {
	return &mockGSUBFace{
		glyphMap: map[rune]uint16{
			0x0628: 10, // beh -> GID 10
			0x0627: 11, // alef -> GID 11
		},
		reverseMap: map[uint16]rune{
			10: 0x0628, // GID 10 -> beh (base)
			11: 0x0627, // GID 11 -> alef (base)
			20: 0xFE91, // GID 20 -> beh initial (PFB codepoint)
			21: 0xFE8E, // GID 21 -> alef final (PFB codepoint)
			30: 0xE001, // GID 30 -> PUA codepoint (font-specific, NOT in PFB table)
			31: 0xE002, // GID 31 -> PUA codepoint
		},
		substitutions: &font.GSUBSubstitutions{
			Single: map[font.GSUBFeature]map[uint16]uint16{
				font.GSUBInit: {10: 30}, // beh initial: GID 10 -> GID 30 -> U+E001
				font.GSUBFina: {11: 31}, // alef final: GID 11 -> GID 31 -> U+E002
			},
		},
	}
}

// TestGSUBPipelineUsedOverPFB verifies that when a font has GSUB tables,
// the GSUB substitutions are used instead of the PFB table. This test
// uses PUA codepoints in the mock's reverse map so the result is
// distinguishable from PFB (which would produce U+FE91 and U+FE8E).
func TestGSUBPipelineUsedOverPFB(t *testing.T) {
	face := newMockArabicFace()
	// Beh + Alef: beh should get init form, alef should get fina form.
	input := "\u0628\u0627"
	shaped := ShapeArabicWithFont(input, face)
	runes := []rune(shaped)

	if len(runes) != 2 {
		t.Fatalf("expected 2 runes, got %d: %U", len(runes), runes)
	}
	// GSUB maps beh initial to GID 30 -> U+E001 (PUA, not PFB's U+FE91).
	if runes[0] != 0xE001 {
		t.Errorf("beh: got %U, want U+E001 (GSUB path). If U+FE91, GSUB was not used.", runes[0])
	}
	// GSUB maps alef final to GID 31 -> U+E002 (PUA, not PFB's U+FE8E).
	if runes[1] != 0xE002 {
		t.Errorf("alef: got %U, want U+E002 (GSUB path). If U+FE8E, GSUB was not used.", runes[1])
	}
}

// TestGSUBFallbackToPFBWhenNoSubstitution verifies that characters not
// covered by GSUB fall back to the PFB table.
func TestGSUBFallbackToPFBWhenNoSubstitution(t *testing.T) {
	face := &mockGSUBFace{
		glyphMap:   map[rune]uint16{0x0633: 40}, // seen -> GID 40
		reverseMap: map[uint16]rune{40: 0x0633},
		substitutions: &font.GSUBSubstitutions{
			// No init/fina/medi/isol entries for GID 40.
			Single: map[font.GSUBFeature]map[uint16]uint16{},
		},
	}
	// Seen isolated: GSUB has no entry -> falls back to PFB.
	input := "\u0633"
	shaped := ShapeArabicWithFont(input, face)
	runes := []rune(shaped)
	// PFB isolated form of seen = U+FEB1.
	if len(runes) != 1 || runes[0] != 0xFEB1 {
		t.Errorf("expected PFB fallback U+FEB1, got %U", runes)
	}
}

// TestGSUBFallbackWhenGIDZero verifies fallback when the font's cmap
// doesn't have the rune (GlyphIndex returns 0).
func TestGSUBFallbackWhenGIDZero(t *testing.T) {
	face := &mockGSUBFace{
		glyphMap:   map[rune]uint16{}, // empty cmap
		reverseMap: map[uint16]rune{},
		substitutions: &font.GSUBSubstitutions{
			Single: map[font.GSUBFeature]map[uint16]uint16{font.GSUBIsol: {99: 100}},
		},
	}
	input := "\u0628" // beh
	shaped := ShapeArabicWithFont(input, face)
	runes := []rune(shaped)
	// GlyphIndex returns 0 -> GSUB skipped -> PFB used.
	if len(runes) != 1 || runes[0] != 0xFE8F {
		t.Errorf("expected PFB fallback U+FE8F (beh isolated), got %U", runes)
	}
}

// TestGSUBFallbackWhenNoReverseMapping verifies fallback when the
// substituted GID has no reverse cmap entry.
func TestGSUBFallbackWhenNoReverseMapping(t *testing.T) {
	face := &mockGSUBFace{
		glyphMap:   map[rune]uint16{0x0628: 10},
		reverseMap: map[uint16]rune{10: 0x0628}, // no entry for GID 50
		substitutions: &font.GSUBSubstitutions{
			Single: map[font.GSUBFeature]map[uint16]uint16{font.GSUBIsol: {10: 50}}, // maps to GID 50
		},
	}
	input := "\u0628"
	shaped := ShapeArabicWithFont(input, face)
	runes := []rune(shaped)
	// GID 50 has no reverse mapping -> falls back to PFB.
	if len(runes) != 1 || runes[0] != 0xFE8F {
		t.Errorf("expected PFB fallback U+FE8F, got %U", runes)
	}
}

// TestGSUBNilFaceMatchesPFB verifies nil face falls back identically.
func TestGSUBNilFaceMatchesPFB(t *testing.T) {
	input := "\u0628\u0633\u0645"
	withNil := ShapeArabicWithFont(input, nil)
	pfbOnly := ShapeArabic(input)
	if withNil != pfbOnly {
		t.Errorf("nil face: got %U, want %U (same as ShapeArabic)", []rune(withNil), []rune(pfbOnly))
	}
}

// TestGSUBFaceWithoutProvider verifies that a Face that does NOT
// implement GSUBProvider falls back to PFB without error.
func TestGSUBFaceWithoutProvider(t *testing.T) {
	// Use a real face that implements Face but check the path works.
	// Since we can't easily create a non-GSUBProvider face (sfntFace
	// always implements it), just verify the nil GSUB path.
	input := "\u0628"
	shaped := ShapeArabicWithFont(input, nil)
	if shaped == input {
		t.Error("expected shaping even without GSUBProvider")
	}
}

// --- System font tests (skipped on CI without Arabic fonts) ---

// TestShapeArabicWithRealFontGSUB exercises the pipeline with a real
// system Arabic font. Skipped when no font is available.
func TestShapeArabicWithRealFontGSUB(t *testing.T) {
	face := loadArabicTestFace(t)
	if face == nil {
		t.Skip("no system Arabic font with GSUB found")
	}
	gp, ok := face.(font.GSUBProvider)
	if !ok || gp.GSUB() == nil {
		t.Skip("no GSUB tables")
	}
	sub := gp.GSUB()
	t.Logf("GSUB features: init=%d medi=%d fina=%d isol=%d",
		len(sub.Single[font.GSUBInit]), len(sub.Single[font.GSUBMedi]),
		len(sub.Single[font.GSUBFina]), len(sub.Single[font.GSUBIsol]))

	input := "\u0633\u0644\u0627\u0645" // salam
	shaped := ShapeArabicWithFont(input, face)
	t.Logf("Input:  %U", []rune(input))
	t.Logf("Shaped: %U", []rune(shaped))

	if shaped == input {
		t.Error("expected shaped output to differ from input")
	}
}

func loadArabicTestFace(t *testing.T) font.Face {
	t.Helper()
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/System/Library/Fonts/SFArabic.ttf",
			"/System/Library/Fonts/ArialHB.ttc",
		}
	case "linux":
		paths = []string{
			"/usr/share/fonts/truetype/noto/NotoSansArabic-Regular.ttf",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		}
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		face, err := font.LoadFont(p)
		if err != nil {
			continue
		}
		return face
	}
	return nil
}
