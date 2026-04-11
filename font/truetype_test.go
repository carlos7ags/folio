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

func TestFaceGSUBCaching(t *testing.T) {
	face := loadTestFace(t)
	provider, ok := face.(GSUBProvider)
	if !ok {
		t.Fatal("sfntFace should implement GSUBProvider")
	}
	first := provider.GSUB()
	second := provider.GSUB()

	// Both calls must agree on nil-ness.
	if (first == nil) != (second == nil) {
		t.Errorf("GSUB cache inconsistency: first nil=%v, second nil=%v",
			first == nil, second == nil)
	}
	if first == nil {
		t.Skip("font has no GSUB table; cannot verify cache identity")
	}

	// Identity check: the cached path must return the same pointer.
	if first != second {
		t.Errorf("GSUB cache returned different pointers (%p vs %p); cache is not taking effect", first, second)
	}
}

func TestFaceGIDToUnicode(t *testing.T) {
	face := loadTestFace(t)
	provider, ok := face.(GSUBProvider)
	if !ok {
		t.Fatal("sfntFace should implement GSUBProvider")
	}
	m := provider.GIDToUnicode()
	if len(m) == 0 {
		t.Fatal("GIDToUnicode returned empty map")
	}

	// Look up the GID for 'A' and confirm the reverse map contains it.
	gidA := face.GlyphIndex('A')
	if gidA == 0 {
		t.Skip("font has no glyph for 'A'")
	}
	r, ok := m[gidA]
	if !ok {
		t.Errorf("GIDToUnicode missing entry for GID %d ('A')", gidA)
	} else if r == 0 {
		t.Errorf("GIDToUnicode mapped GID %d to zero rune", gidA)
	}

	// Cache check: second call returns equivalent map.
	m2 := provider.GIDToUnicode()
	if len(m2) != len(m) {
		t.Errorf("GIDToUnicode map length changed: first=%d second=%d", len(m), len(m2))
	}
}

func TestBuildGIDToUnicodeDirect(t *testing.T) {
	path := testFontPath(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	m := BuildGIDToUnicode(data)
	if len(m) == 0 {
		t.Fatal("BuildGIDToUnicode returned empty map")
	}
	// A system TTF that covers Latin-1 must round-trip 'A'.
	found := false
	for _, r := range m {
		if r == 'A' {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'A' (0x41) to be reachable in GIDToUnicode reverse map")
	}
}

func TestBuildGIDToUnicodeInvalidData(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("BuildGIDToUnicode panicked on invalid data: %v", r)
		}
	}()
	m := BuildGIDToUnicode([]byte{1, 2, 3})
	if len(m) != 0 {
		t.Errorf("expected empty map for invalid data, got %d entries", len(m))
	}
}

// TestLookupKernPairTooShort exercises the short-data guard at the top
// of lookupKernPair.
func TestLookupKernPairTooShort(t *testing.T) {
	if v := lookupKernPair([]byte{}, 0, 0); v != 0 {
		t.Errorf("expected 0 for empty input, got %d", v)
	}
	if v := lookupKernPair([]byte{0, 0, 0}, 0, 0); v != 0 {
		t.Errorf("expected 0 for 3-byte input, got %d", v)
	}
}

// TestLookupKernPairWrongVersion exercises the branch where the version
// field is neither 0 nor 1.
func TestLookupKernPairWrongVersion(t *testing.T) {
	data := []byte{
		0x00, 0x02, // version = 2 (unsupported)
		0x00, 0x00, // nTables = 0
	}
	if v := lookupKernPair(data, 0, 0); v != 0 {
		t.Errorf("expected 0 for unsupported version, got %d", v)
	}
}

// TestLookupKernPairVersion1 exercises the Apple AAT version-1 kern table
// parsing branch with a single format-0 subtable.
func TestLookupKernPairVersion1(t *testing.T) {
	// Version 1 AAT header: version(uint32)=0x00010000, nTables(uint32).
	// Code only reads bytes 0-2 to check version==1 (sees 0x0001),
	// then reads bytes 4-8 as nTables32.
	//
	// Subtable v1 header: length(4), coverage(2), tupleIndex(2).
	// Coverage low byte = format. Set format=0.
	// Subtable body follows at offset+8.
	//
	// Format-0 body: nPairs(2), searchRange(2), entrySelector(2),
	// rangeShift(2), pairs... (6 bytes each).
	body := []byte{
		0x00, 0x01, // nPairs = 1
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // sr/es/rs
		0x00, 0x41, 0x00, 0x56, 0xFF, 0xB0, // 'A','V' = -80
	}
	subLen := 8 + len(body)
	sub := make([]byte, 0, subLen)
	// length (uint32 big endian)
	sub = append(sub,
		byte(subLen>>24), byte(subLen>>16), byte(subLen>>8), byte(subLen),
	)
	// coverage: format=0 in low byte
	sub = append(sub, 0x00, 0x00)
	// tupleIndex
	sub = append(sub, 0x00, 0x00)
	sub = append(sub, body...)

	// Kern header: version(uint32)=0x00010000, nTables(uint32)=1
	data := []byte{
		0x00, 0x01, 0x00, 0x00, // version
		0x00, 0x00, 0x00, 0x01, // nTables
	}
	data = append(data, sub...)

	if v := lookupKernPair(data, 0x0041, 0x0056); v != -80 {
		t.Errorf("expected -80 for v1 AAT kern table, got %d", v)
	}
}

// TestLookupKernPairFormat1Skipped builds a version-0 header with one
// subtable whose format is 1 (Apple AAT). The lookup should skip it and
// return 0 because only format 0 is supported.
func TestLookupKernPairFormat1Skipped(t *testing.T) {
	// Subtable header: subVersion(2)=0, length(2)=14, coverage(2).
	// coverage encoding: high byte bit 0 = horizontal, low byte = format.
	// For format=1, horizontal: coverage = 0x0101.
	// Subtable total length including header = 6 (header) + 8 (dummy data) = 14.
	data := []byte{
		0x00, 0x00, // version = 0
		0x00, 0x01, // nTables = 1
		0x00, 0x00, // subtable version
		0x00, 0x0E, // subtable length = 14
		0x01, 0x01, // coverage: format=1, horizontal
		// 8 bytes of filler to reach subtable length 14
		0, 0, 0, 0, 0, 0, 0, 0,
	}
	if v := lookupKernPair(data, 0x41, 0x42); v != 0 {
		t.Errorf("expected 0 for format-1 subtable, got %d", v)
	}
}

// TestLookupKernPairMultipleSubtables builds a version-0 header with two
// format-0 subtables and puts the target pair in the second subtable.
func TestLookupKernPairMultipleSubtables(t *testing.T) {
	// Helper: build a format-0 subtable with one pair.
	makeSubtable := func(left, right uint16, val int16) []byte {
		// format-0 body: nPairs(2) + searchRange(2) + entrySelector(2)
		// + rangeShift(2) + nPairs*6 bytes
		body := []byte{
			0x00, 0x01, // nPairs = 1
			0x00, 0x00, // searchRange
			0x00, 0x00, // entrySelector
			0x00, 0x00, // rangeShift
			byte(left >> 8), byte(left),
			byte(right >> 8), byte(right),
			byte(uint16(val) >> 8), byte(uint16(val)),
		}
		// subtable header: version(2)=0, length(2), coverage(2).
		// coverage: format=0, horizontal => 0x0100.
		subLen := 6 + len(body)
		hdr := []byte{
			0x00, 0x00, // subtable version
			byte(subLen >> 8), byte(subLen),
			0x01, 0x00, // coverage: format=0, horizontal
		}
		return append(hdr, body...)
	}

	sub1 := makeSubtable(0x0001, 0x0002, -10) // unrelated pair
	sub2 := makeSubtable(0x0041, 0x0056, -80) // target pair: 'A','V'

	// kern header: version(2)=0 + nTables(2)=2
	data := []byte{0x00, 0x00, 0x00, 0x02}
	data = append(data, sub1...)
	data = append(data, sub2...)

	if v := lookupKernPair(data, 0x0041, 0x0056); v != -80 {
		t.Errorf("expected -80 for target pair in second subtable, got %d", v)
	}
	// First subtable pair should still resolve.
	if v := lookupKernPair(data, 0x0001, 0x0002); v != -10 {
		t.Errorf("expected -10 for target pair in first subtable, got %d", v)
	}
	// Missing pair returns 0.
	if v := lookupKernPair(data, 0x00FF, 0x00FF); v != 0 {
		t.Errorf("expected 0 for missing pair, got %d", v)
	}
}
