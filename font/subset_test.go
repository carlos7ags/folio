// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"os"
	"strings"
	"testing"

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
