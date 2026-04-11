// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/carlos7ags/folio/core"
)

func loadTestFont(t *testing.T) []byte {
	t.Helper()
	path := testFontPath(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return data
}

func TestSubsetProducesValidTTF(t *testing.T) {
	raw := loadTestFont(t)
	face := loadTestFace(t)

	glyphs := map[uint16]rune{
		0:                    0,
		face.GlyphIndex('A'): 'A',
		face.GlyphIndex('B'): 'B',
		face.GlyphIndex(' '): ' ',
	}

	subset, err := Subset(raw, glyphs)
	if err != nil {
		t.Fatalf("Subset: %v", err)
	}

	if len(subset) < 12 {
		t.Fatal("subset too small")
	}
	scalar := binary.BigEndian.Uint32(subset[:4])
	if scalar != 0x00010000 && scalar != 0x74727565 {
		t.Errorf("unexpected scalar type: 0x%08X", scalar)
	}
}

func TestSubsetSmallerThanOriginal(t *testing.T) {
	raw := loadTestFont(t)
	face := loadTestFace(t)

	glyphs := map[uint16]rune{
		0:                    0,
		face.GlyphIndex('X'): 'X',
	}

	subset, err := Subset(raw, glyphs)
	if err != nil {
		t.Fatalf("Subset: %v", err)
	}

	if len(subset) >= len(raw) {
		t.Errorf("subset (%d bytes) should be smaller than original (%d bytes)", len(subset), len(raw))
	}
}

func TestSubsetPreservesUsedGlyphs(t *testing.T) {
	raw := loadTestFont(t)
	face := loadTestFace(t)

	gidA := face.GlyphIndex('A')
	glyphs := map[uint16]rune{
		0:    0,
		gidA: 'A',
	}

	subset, err := Subset(raw, glyphs)
	if err != nil {
		t.Fatalf("Subset: %v", err)
	}

	tables, err := parseTTFTables(subset)
	if err != nil {
		t.Fatalf("parseTTFTables on subset: %v", err)
	}

	head := tables["head"]
	loca := tables["loca"]

	indexToLocFormat := int16(binary.BigEndian.Uint16(head[50:52]))
	offsets, err := parseLoca(loca, indexToLocFormat, int(gidA)+2)
	if err != nil {
		t.Fatalf("parseLoca: %v", err)
	}

	start := offsets[gidA]
	end := offsets[gidA+1]
	if end <= start {
		t.Errorf("glyph A (GID %d) has zero-length data in subset", gidA)
	}
}

func TestSubsetZerosUnusedGlyphs(t *testing.T) {
	raw := loadTestFont(t)
	face := loadTestFace(t)

	gidA := face.GlyphIndex('A')
	gidB := face.GlyphIndex('B')
	glyphs := map[uint16]rune{
		0:    0,
		gidA: 'A',
	}

	subset, err := Subset(raw, glyphs)
	if err != nil {
		t.Fatalf("Subset: %v", err)
	}

	tables, err := parseTTFTables(subset)
	if err != nil {
		t.Fatalf("parseTTFTables: %v", err)
	}

	head := tables["head"]
	loca := tables["loca"]

	indexToLocFormat := int16(binary.BigEndian.Uint16(head[50:52]))
	maxGID := max(gidA, gidB)
	offsets, err := parseLoca(loca, indexToLocFormat, int(maxGID)+2)
	if err != nil {
		t.Fatalf("parseLoca: %v", err)
	}

	if gidB < uint16(len(offsets)-1) {
		start := offsets[gidB]
		end := offsets[gidB+1]
		if end != start {
			t.Errorf("unused glyph B (GID %d) should be zeroed, but has length %d", gidB, end-start)
		}
	}
}

func TestSubsetRequiredTables(t *testing.T) {
	raw := loadTestFont(t)
	face := loadTestFace(t)

	glyphs := map[uint16]rune{
		0:                    0,
		face.GlyphIndex('H'): 'H',
	}

	subset, err := Subset(raw, glyphs)
	if err != nil {
		t.Fatalf("Subset: %v", err)
	}

	tables, err := parseTTFTables(subset)
	if err != nil {
		t.Fatalf("parseTTFTables: %v", err)
	}

	required := []string{"head", "hhea", "maxp", "OS/2", "name", "cmap", "post", "loca", "glyf", "hmtx"}
	for _, name := range required {
		if _, ok := tables[name]; !ok {
			t.Errorf("subset missing required table: %s", name)
		}
	}
}

func TestSubsetEmptyGlyphs(t *testing.T) {
	raw := loadTestFont(t)
	glyphs := map[uint16]rune{0: 0}

	subset, err := Subset(raw, glyphs)
	if err != nil {
		t.Fatalf("Subset: %v", err)
	}
	if len(subset) == 0 {
		t.Error("subset should not be empty")
	}
}

func TestSubsetInvalidData(t *testing.T) {
	_, err := Subset([]byte("not a font"), map[uint16]rune{0: 0})
	if err == nil {
		t.Error("expected error for invalid font data")
	}
}

func TestSubsetIntegrationBuildObjects(t *testing.T) {
	face := loadTestFace(t)
	ef := NewEmbeddedFont(face)
	ef.EncodeString("Hello")

	var objects []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		ref := &core.PdfIndirectReference{ObjectNumber: len(objects) + 1, GenerationNumber: 0}
		objects = append(objects, obj)
		return ref
	}

	type0 := ef.BuildObjects(addObject)

	// Verify the font name has a subset tag (6 uppercase letters + "+")
	baseFontObj := type0.Get("BaseFont")
	if baseFontObj == nil {
		t.Fatal("missing BaseFont in Type0 dict")
	}
	baseFontName := baseFontObj.(*core.PdfName).Value
	if !strings.Contains(baseFontName, "+") {
		t.Errorf("expected subset tag in font name, got %q", baseFontName)
	}
	parts := strings.SplitN(baseFontName, "+", 2)
	if len(parts[0]) != 6 {
		t.Errorf("subset tag should be 6 chars, got %q", parts[0])
	}
	for _, c := range parts[0] {
		if c < 'A' || c > 'Z' {
			t.Errorf("subset tag should be uppercase, got %q", parts[0])
			break
		}
	}
}

func TestSubsetFontStreamSmaller(t *testing.T) {
	face := loadTestFace(t)
	ef := NewEmbeddedFont(face)
	ef.EncodeString("AB")

	var fontStreamData []byte
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		if stream, ok := obj.(*core.PdfStream); ok {
			if stream.Dict.Get("Length1") != nil {
				fontStreamData = stream.Data
			}
		}
		return &core.PdfIndirectReference{ObjectNumber: 1, GenerationNumber: 0}
	}
	ef.BuildObjects(addObject)

	// The font stream (compressed subset) should be much smaller than the raw font
	rawSize := len(face.RawData())
	if len(fontStreamData) == 0 {
		t.Fatal("did not capture font stream data")
	}
	t.Logf("raw font: %d bytes, subset stream: %d bytes", rawSize, len(fontStreamData))
	if len(fontStreamData) >= rawSize {
		t.Errorf("subset stream (%d) should be smaller than raw font (%d)", len(fontStreamData), rawSize)
	}
}

func TestSubsetTag(t *testing.T) {
	glyphs := map[uint16]rune{0: 0, 72: 'H'}
	tag := subsetTag(glyphs)
	if len(tag) != 6 {
		t.Errorf("expected 6-char tag, got %q", tag)
	}
	for _, c := range tag {
		if c < 'A' || c > 'Z' {
			t.Errorf("tag should be all uppercase, got %q", tag)
			break
		}
	}

	// Same input should produce same tag
	tag2 := subsetTag(glyphs)
	if tag != tag2 {
		t.Errorf("tag not deterministic: %q vs %q", tag, tag2)
	}
}

// --- parseCompositeComponents ---
//
// These tests cover the composite-glyph record walker directly with
// hand-built bytes, so we can exercise each flag combination in the
// transform encoding and confirm the bounds checks terminate safely
// on malformed input.
//
// Composite glyph layout (OpenType spec, glyf table):
//
//	int16 numberOfContours (negative for composite)
//	int16 xMin, yMin, xMax, yMax
//	repeat until MORE_COMPONENTS (0x0020) is clear:
//	    uint16 flags
//	    uint16 glyphIndex
//	    int8/int16 argument1, argument2   (words if 0x0001 set)
//	    optional transform:
//	        WE_HAVE_A_SCALE          (0x0008): F2Dot14       (2 bytes)
//	        WE_HAVE_AN_X_AND_Y_SCALE (0x0040): 2 × F2Dot14   (4 bytes)
//	        WE_HAVE_A_TWO_BY_TWO     (0x0080): 4 × F2Dot14   (8 bytes)

const (
	compArgsAreWords = 0x0001
	compMoreComps    = 0x0020
	compScale        = 0x0008
	compXYScale      = 0x0040
	compTwoByTwo     = 0x0080
)

// compositeHeader is the 10-byte header shared by every composite glyph
// record in the tests below.
var compositeHeader = []byte{
	0xFF, 0xFF, // numberOfContours = -1 (composite)
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // bbox (zeros)
}

func appendComponent(buf []byte, flags, gid uint16, argSize, transformSize int) []byte {
	buf = binary.BigEndian.AppendUint16(buf, flags)
	buf = binary.BigEndian.AppendUint16(buf, gid)
	buf = append(buf, make([]byte, argSize)...)
	buf = append(buf, make([]byte, transformSize)...)
	return buf
}

func equalUint16(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestParseCompositeComponentsSingle(t *testing.T) {
	data := slices.Clone(compositeHeader)
	// Single component, no MORE_COMPONENTS, 1-byte args, no transform.
	data = appendComponent(data, 0, 42, 2, 0)

	got := parseCompositeComponents(data)
	want := []uint16{42}
	if !equalUint16(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseCompositeComponentsMultiple(t *testing.T) {
	data := slices.Clone(compositeHeader)
	data = appendComponent(data, compMoreComps, 7, 2, 0)
	data = appendComponent(data, compMoreComps, 19, 2, 0)
	data = appendComponent(data, 0, 123, 2, 0) // last, no MORE_COMPONENTS

	got := parseCompositeComponents(data)
	want := []uint16{7, 19, 123}
	if !equalUint16(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseCompositeComponentsArgsAsWords(t *testing.T) {
	data := slices.Clone(compositeHeader)
	// ARG_1_AND_2_ARE_WORDS -> args take 4 bytes instead of 2.
	data = appendComponent(data, compArgsAreWords, 55, 4, 0)

	got := parseCompositeComponents(data)
	if !equalUint16(got, []uint16{55}) {
		t.Errorf("got %v, want [55]", got)
	}
}

func TestParseCompositeComponentsTransformVariants(t *testing.T) {
	cases := []struct {
		name      string
		flags     uint16
		transform int
	}{
		{"scale", compScale, 2},
		{"xy_scale", compXYScale, 4},
		{"two_by_two", compTwoByTwo, 8},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Two components: first has the transform flag + MORE_COMPONENTS,
			// second has no transform and no MORE_COMPONENTS. This also
			// verifies that pos is advanced past the transform block.
			data := slices.Clone(compositeHeader)
			data = appendComponent(data, tc.flags|compMoreComps, 100, 2, tc.transform)
			data = appendComponent(data, 0, 200, 2, 0)

			got := parseCompositeComponents(data)
			want := []uint16{100, 200}
			if !equalUint16(got, want) {
				t.Errorf("got %v, want %v", got, want)
			}
		})
	}
}

func TestParseCompositeComponentsTruncatedHeader(t *testing.T) {
	// Anything shorter than the 10-byte header must return nil.
	for n := range 12 {
		got := parseCompositeComponents(make([]byte, n))
		if got != nil {
			t.Errorf("len=%d: expected nil, got %v", n, got)
		}
	}
}

func TestParseCompositeComponentsTruncatedTransform(t *testing.T) {
	// Declare WE_HAVE_A_TWO_BY_TWO (needs 8 transform bytes) but only
	// provide 3. The walker must stop without reading out of bounds.
	data := slices.Clone(compositeHeader)
	data = appendComponent(data, compTwoByTwo, 42, 2, 3) // short matrix

	got := parseCompositeComponents(data)
	// The first component header was read successfully, so GID 42 is
	// reported before the walker notices the truncated transform.
	if !equalUint16(got, []uint16{42}) {
		t.Errorf("got %v, want [42]", got)
	}
}

func TestParseCompositeComponentsTruncatedRecord(t *testing.T) {
	// Header + only 3 bytes of the next component record. The loop
	// condition (pos+4 <= len) must reject the short record.
	data := slices.Clone(compositeHeader)
	data = append(data, 0x00, 0x20, 0x00) // partial flags+gid

	got := parseCompositeComponents(data)
	if got != nil {
		t.Errorf("expected nil on truncated record, got %v", got)
	}
}

// --- resolveComposites ---

// buildGlyfEntries concatenates glyph records and returns the glyf bytes
// and the loca offsets array (length numGlyphs+1).
func buildGlyfEntries(records [][]byte) ([]byte, []uint32) {
	var glyf []byte
	offsets := make([]uint32, len(records)+1)
	for i, rec := range records {
		offsets[i] = uint32(len(glyf))
		glyf = append(glyf, rec...)
	}
	offsets[len(records)] = uint32(len(glyf))
	return glyf, offsets
}

func TestResolveCompositesTransitiveClosure(t *testing.T) {
	// Glyph 0: simple (.notdef), dummy contents.
	// Glyph 1: simple.
	// Glyph 2: composite referencing glyph 3 (not yet in the set).
	// Glyph 3: composite referencing glyph 1 (already in the set via closure).
	simple := []byte{0x00, 0x01, 0, 0, 0, 0, 0, 0, 0, 0} // numContours=1
	comp := func(gid uint16) []byte {
		b := slices.Clone(compositeHeader)
		return appendComponent(b, 0, gid, 2, 0)
	}
	glyf, offsets := buildGlyfEntries([][]byte{
		simple,      // gid 0
		simple,      // gid 1
		comp(3),     // gid 2 -> 3
		comp(1),     // gid 3 -> 1
	})

	glyphSet := map[uint16]bool{0: true, 2: true}
	resolveComposites(glyf, offsets, glyphSet, 4)

	for _, gid := range []uint16{0, 1, 2, 3} {
		if !glyphSet[gid] {
			t.Errorf("gid %d missing from set after closure", gid)
		}
	}
}

func TestResolveCompositesCycleTerminates(t *testing.T) {
	// Glyph 0 simple; glyph 1 and glyph 2 reference each other.
	// The fixed-point loop must terminate because the set stops growing.
	simple := []byte{0x00, 0x01, 0, 0, 0, 0, 0, 0, 0, 0}
	comp := func(gid uint16) []byte {
		b := slices.Clone(compositeHeader)
		return appendComponent(b, 0, gid, 2, 0)
	}
	glyf, offsets := buildGlyfEntries([][]byte{
		simple,  // gid 0
		comp(2), // gid 1 -> 2
		comp(1), // gid 2 -> 1
	})

	glyphSet := map[uint16]bool{0: true, 1: true}
	done := make(chan struct{})
	go func() {
		resolveComposites(glyf, offsets, glyphSet, 3)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("resolveComposites did not terminate on cyclic references")
	}
	if !glyphSet[2] {
		t.Error("cyclic target gid 2 should have been added to set")
	}
}

func TestParseCompositeComponentsCombinedFlags(t *testing.T) {
	// A single component can carry multiple flags simultaneously:
	// ARG_1_AND_2_ARE_WORDS (4-byte args) together with WE_HAVE_A_TWO_BY_TWO
	// (8-byte matrix) produces the longest-possible component record. The
	// walker must advance past both blocks before it reads the next
	// component header.
	data := slices.Clone(compositeHeader)
	data = appendComponent(data, compArgsAreWords|compTwoByTwo|compMoreComps, 77, 4, 8)
	data = appendComponent(data, compArgsAreWords, 88, 4, 0)

	got := parseCompositeComponents(data)
	want := []uint16{77, 88}
	if !equalUint16(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseCompositeComponentsInstructionsFlagIgnored(t *testing.T) {
	// WE_HAVE_INSTRUCTIONS (0x0100) means a uint16 instruction-length
	// field and its payload follow the last component. parseCompositeComponents
	// has no business walking past the last record, so any trailing
	// instruction bytes must be ignored without affecting the result.
	const weHaveInstructions uint16 = 0x0100
	data := slices.Clone(compositeHeader)
	data = appendComponent(data, weHaveInstructions, 33, 2, 0)
	// Trailing bytes that a naive reader might interpret as another
	// component header — they must not pollute the returned list.
	data = append(data, 0x00, 0x03, 0xFF, 0xFF, 0xFF)

	got := parseCompositeComponents(data)
	if !equalUint16(got, []uint16{33}) {
		t.Errorf("got %v, want [33]", got)
	}
}

func TestResolveCompositesWithRealFont(t *testing.T) {
	raw := loadTestFont(t)
	face := loadTestFace(t)

	tables, err := parseTTFTables(raw)
	if err != nil {
		t.Fatalf("parseTTFTables: %v", err)
	}
	maxpData := tables["maxp"]
	numGlyphs := int(binary.BigEndian.Uint16(maxpData[4:6]))
	headData := tables["head"]
	locaFormat := int16(binary.BigEndian.Uint16(headData[50:52]))
	offsets, err := parseLoca(tables["loca"], locaFormat, numGlyphs)
	if err != nil {
		t.Fatalf("parseLoca: %v", err)
	}
	glyfData := tables["glyf"]

	// Accented Latin letters are usually stored as composite glyphs
	// (base + accent). Scan candidates until we find one that is
	// actually composite in the installed font; skip if none are.
	candidates := []rune{'Á', 'É', 'Í', 'Ó', 'Ú', 'Ñ', 'Ö', 'Ü', 'Ç', 'Å'}
	var compositeGID uint16
	var compositeRune rune
	for _, r := range candidates {
		gid := face.GlyphIndex(r)
		if gid == 0 || int(gid) >= numGlyphs {
			continue
		}
		start := offsets[gid]
		end := offsets[gid+1]
		if start >= end || int(end) > len(glyfData) || end-start < 2 {
			continue
		}
		numContours := int16(binary.BigEndian.Uint16(glyfData[start : start+2]))
		if numContours < 0 {
			compositeGID = gid
			compositeRune = r
			break
		}
	}
	if compositeGID == 0 {
		t.Skip("no composite glyphs among candidates in installed test font")
	}

	glyphSet := map[uint16]bool{0: true, compositeGID: true}
	resolveComposites(glyfData, offsets, glyphSet, numGlyphs)

	if len(glyphSet) < 3 {
		t.Errorf("closure for %q (gid %d) produced %d glyphs, expected at least 3 (notdef + composite + >=1 component)",
			compositeRune, compositeGID, len(glyphSet))
	}
	// Every glyph in the closed set must be a valid GID.
	for gid := range glyphSet {
		if int(gid) >= numGlyphs {
			t.Errorf("closed set contains out-of-range gid %d (numGlyphs=%d)", gid, numGlyphs)
		}
	}
	// Subsetting the font with just the accented rune must succeed and
	// must preserve the composite glyph's data (non-zero-length glyf entry).
	subset, err := Subset(raw, map[uint16]rune{0: 0, compositeGID: compositeRune})
	if err != nil {
		t.Fatalf("Subset: %v", err)
	}
	subTables, err := parseTTFTables(subset)
	if err != nil {
		t.Fatalf("parseTTFTables(subset): %v", err)
	}
	subOffsets, err := parseLoca(subTables["loca"],
		int16(binary.BigEndian.Uint16(subTables["head"][50:52])),
		numGlyphs)
	if err != nil {
		t.Fatalf("parseLoca(subset): %v", err)
	}
	if subOffsets[compositeGID] >= subOffsets[compositeGID+1] {
		t.Errorf("composite glyph %d was zeroed out of the subset", compositeGID)
	}
}

func TestResolveCompositesIgnoresOutOfRangeComponents(t *testing.T) {
	// Glyph 1 references gid 999, which is past numGlyphs. The outer
	// loop (resolveComposites) filters gids >= numGlyphs, so the call
	// must return without adding bogus entries or panicking.
	simple := []byte{0x00, 0x01, 0, 0, 0, 0, 0, 0, 0, 0}
	b := slices.Clone(compositeHeader)
	b = appendComponent(b, 0, 999, 2, 0)
	glyf, offsets := buildGlyfEntries([][]byte{simple, b})

	glyphSet := map[uint16]bool{0: true, 1: true}
	resolveComposites(glyf, offsets, glyphSet, 2)

	if _, ok := glyphSet[999]; ok {
		t.Error("out-of-range component gid 999 should not be in set")
	}
	if len(glyphSet) != 2 {
		t.Errorf("glyphSet size = %d, want 2", len(glyphSet))
	}
}
