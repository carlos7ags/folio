// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"os"
	"testing"
)

// testFontPath returns a path to a TTF font available on the system.
// Falls back and skips if not found.
func testFontPath(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/System/Library/Fonts/Supplemental/Courier New.ttf",
		"/System/Library/Fonts/Supplemental/Times New Roman.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", // Linux
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no suitable TTF font found on this system")
	return ""
}

func loadTestFace(t *testing.T) Face {
	t.Helper()
	path := testFontPath(t)
	face, err := LoadTTF(path)
	if err != nil {
		t.Fatalf("LoadTTF(%s) failed: %v", path, err)
	}
	return face
}

func TestLoadTTF(t *testing.T) {
	face := loadTestFace(t)
	if face == nil {
		t.Fatal("LoadTTF returned nil")
	}
}

func TestPostScriptName(t *testing.T) {
	face := loadTestFace(t)
	name := face.PostScriptName()
	if name == "" {
		t.Error("PostScriptName should not be empty")
	}
	t.Logf("PostScriptName: %s", name)
}

func TestUnitsPerEm(t *testing.T) {
	face := loadTestFace(t)
	upem := face.UnitsPerEm()
	// Most fonts use 1000 or 2048
	if upem != 1000 && upem != 2048 {
		t.Logf("unusual UnitsPerEm: %d (expected 1000 or 2048)", upem)
	}
	if upem <= 0 {
		t.Errorf("UnitsPerEm should be positive, got %d", upem)
	}
	t.Logf("UnitsPerEm: %d", upem)
}

func TestGlyphIndex(t *testing.T) {
	face := loadTestFace(t)

	// 'A' should have a non-zero glyph ID in any Latin font
	gid := face.GlyphIndex('A')
	if gid == 0 {
		t.Error("GlyphIndex('A') returned 0 (notdef)")
	}

	// Space should also exist
	gidSpace := face.GlyphIndex(' ')
	if gidSpace == 0 {
		t.Error("GlyphIndex(' ') returned 0 (notdef)")
	}

	// Different characters should (usually) have different glyph IDs
	gidB := face.GlyphIndex('B')
	if gidB == gid {
		t.Error("'A' and 'B' should have different glyph IDs")
	}
}

func TestGlyphAdvance(t *testing.T) {
	face := loadTestFace(t)

	gid := face.GlyphIndex('A')
	adv := face.GlyphAdvance(gid)
	if adv <= 0 {
		t.Errorf("GlyphAdvance('A') should be positive, got %d", adv)
	}

	// Space should be narrower than 'M' in most fonts
	gidM := face.GlyphIndex('M')
	gidSpace := face.GlyphIndex(' ')
	advM := face.GlyphAdvance(gidM)
	advSpace := face.GlyphAdvance(gidSpace)
	t.Logf("Advance: M=%d, space=%d", advM, advSpace)
	if advSpace >= advM {
		t.Log("space advance >= M advance (unusual but not necessarily wrong)")
	}
}

func TestAscent(t *testing.T) {
	face := loadTestFace(t)
	asc := face.Ascent()
	if asc <= 0 {
		t.Errorf("Ascent should be positive, got %d", asc)
	}
	t.Logf("Ascent: %d", asc)
}

func TestDescent(t *testing.T) {
	face := loadTestFace(t)
	desc := face.Descent()
	if desc >= 0 {
		t.Errorf("Descent should be negative (PDF convention), got %d", desc)
	}
	t.Logf("Descent: %d", desc)
}

func TestBBox(t *testing.T) {
	face := loadTestFace(t)
	bbox := face.BBox()
	// BBox should have non-zero extent
	width := bbox[2] - bbox[0]
	height := bbox[3] - bbox[1]
	if width <= 0 || height <= 0 {
		t.Errorf("BBox should have positive extent, got %v (w=%d, h=%d)", bbox, width, height)
	}
	t.Logf("BBox: %v", bbox)
}

func TestFlags(t *testing.T) {
	face := loadTestFace(t)
	flags := face.Flags()
	// Should have Nonsymbolic bit set (32)
	if flags&32 == 0 {
		t.Error("expected Nonsymbolic flag (bit 6) to be set")
	}
}

func TestRawData(t *testing.T) {
	face := loadTestFace(t)
	data := face.RawData()
	if len(data) == 0 {
		t.Error("RawData should not be empty")
	}
	// TTF files start with 0x00010000 or "OTTO" (for OTF)
	if len(data) >= 4 {
		if data[0] == 0 && data[1] == 1 && data[2] == 0 && data[3] == 0 {
			t.Log("Detected TrueType font")
		} else if string(data[:4]) == "OTTO" {
			t.Log("Detected OpenType font")
		} else {
			t.Logf("Unknown font header: %x", data[:4])
		}
	}
}

func TestNumGlyphs(t *testing.T) {
	face := loadTestFace(t)
	n := face.NumGlyphs()
	if n <= 0 {
		t.Errorf("NumGlyphs should be positive, got %d", n)
	}
	t.Logf("NumGlyphs: %d", n)
}

func TestParseTTFInvalidData(t *testing.T) {
	_, err := ParseTTF([]byte("not a font"))
	if err == nil {
		t.Error("ParseTTF should fail on invalid data")
	}
}

func TestLoadTTFMissingFile(t *testing.T) {
	_, err := LoadTTF("/nonexistent/path/font.ttf")
	if err == nil {
		t.Error("LoadTTF should fail on missing file")
	}
}

func TestItalicAngle(t *testing.T) {
	face := loadTestFace(t)
	angle := face.ItalicAngle()
	// Arial is upright, so italic angle should be 0.
	// Other test fonts may differ, but the value should be parseable.
	t.Logf("ItalicAngle: %f", angle)
	if angle > 0 || angle < -45 {
		t.Errorf("ItalicAngle out of expected range [-45, 0], got %f", angle)
	}
}

func TestCapHeight(t *testing.T) {
	face := loadTestFace(t)
	ch := face.CapHeight()
	t.Logf("CapHeight: %d", ch)
	// Most fonts have CapHeight between 600–800 for upem 2048,
	// or 600–750 for upem 1000. Should be positive if OS/2 v2+.
	if ch <= 0 {
		t.Log("CapHeight is 0 — OS/2 table may be missing or version < 2")
	}
	if ch > 0 && ch > face.UnitsPerEm() {
		t.Errorf("CapHeight %d exceeds UnitsPerEm %d", ch, face.UnitsPerEm())
	}
}

func TestStemV(t *testing.T) {
	face := loadTestFace(t)
	sv := face.StemV()
	t.Logf("StemV: %d", sv)
	if sv <= 0 {
		t.Errorf("StemV should be positive, got %d", sv)
	}
	if sv > 500 {
		t.Errorf("StemV seems too large: %d", sv)
	}
}

func TestFaceInterface(t *testing.T) {
	// Verify sfntFace implements Face at compile time
	face := loadTestFace(t)
	var _ Face = face //nolint:staticcheck // compile-time interface check
}

func loadFontFace(t *testing.T, path string) Face {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("font not available: %s", path)
	}
	face, err := ParseTTF(data)
	if err != nil {
		t.Fatalf("ParseTTF(%s): %v", path, err)
	}
	return face
}

func TestFlagsNonsymbolic(t *testing.T) {
	// Arial is a sans-serif, non-symbolic, non-italic, proportional font.
	face := loadFontFace(t, "/System/Library/Fonts/Supplemental/Arial.ttf")
	flags := face.Flags()
	if flags&32 == 0 {
		t.Error("Arial should be Nonsymbolic (bit 5)")
	}
	if flags&4 != 0 {
		t.Error("Arial should NOT be Symbolic (bit 2)")
	}
	if flags&1 != 0 {
		t.Error("Arial should NOT be FixedPitch (bit 0)")
	}
	if flags&64 != 0 {
		t.Error("Arial should NOT be Italic (bit 6)")
	}
}

func TestFlagsFixedPitch(t *testing.T) {
	face := loadFontFace(t, "/System/Library/Fonts/Supplemental/Courier New.ttf")
	flags := face.Flags()
	if flags&1 == 0 {
		t.Error("Courier New should be FixedPitch (bit 0)")
	}
	if flags&32 == 0 {
		t.Error("Courier New should be Nonsymbolic (bit 5)")
	}
}

func TestFlagsSerif(t *testing.T) {
	face := loadFontFace(t, "/System/Library/Fonts/Supplemental/Times New Roman.ttf")
	flags := face.Flags()
	if flags&2 == 0 {
		t.Error("Times New Roman should be Serif (bit 1)")
	}
	if flags&32 == 0 {
		t.Error("Times New Roman should be Nonsymbolic (bit 5)")
	}
}

func TestLookupKernFormat0BoundsCheck(t *testing.T) {
	// Craft a kern format 0 subtable with inflated nPairs but only 1 real pair.
	// nPairs=9999, searchRange=0, entrySelector=0, rangeShift=0 (8 bytes header)
	// + 1 real pair: left=0x0041 ('A'), right=0x0056 ('V'), value=-80 (6 bytes)
	// Total: 14 bytes, but nPairs claims 9999 entries.
	data := []byte{
		0x27, 0x0F, // nPairs = 9999
		0x00, 0x00, // searchRange
		0x00, 0x00, // entrySelector
		0x00, 0x00, // rangeShift
		// pair: left=0x0041, right=0x0056, value=-80 (0xFFB0)
		0x00, 0x41, 0x00, 0x56, 0xFF, 0xB0,
	}

	// Should not panic despite inflated nPairs.
	val := lookupKernFormat0(data, 0x0041, 0x0056)
	if val != -80 {
		t.Errorf("expected -80, got %d", val)
	}

	// Non-existent pair should return 0.
	val = lookupKernFormat0(data, 0x0041, 0x0042)
	if val != 0 {
		t.Errorf("expected 0 for missing pair, got %d", val)
	}
}

func TestFlagsItalic(t *testing.T) {
	face := loadFontFace(t, "/System/Library/Fonts/Supplemental/Courier New Italic.ttf")
	flags := face.Flags()
	if flags&64 == 0 {
		t.Error("Courier New Italic should be Italic (bit 6)")
	}
	if flags&1 == 0 {
		t.Error("Courier New Italic should be FixedPitch (bit 0)")
	}
}
