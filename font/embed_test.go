// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

func TestEncodeString(t *testing.T) {
	face := loadTestFace(t)
	ef := NewEmbeddedFont(face)

	encoded := ef.EncodeString("AB")
	// Each character should produce 2 bytes (big-endian glyph ID)
	if len(encoded) != 4 {
		t.Fatalf("expected 4 bytes for 2 chars, got %d", len(encoded))
	}

	// Glyph IDs should be non-zero for 'A' and 'B'
	gidA := uint16(encoded[0])<<8 | uint16(encoded[1])
	gidB := uint16(encoded[2])<<8 | uint16(encoded[3])
	if gidA == 0 {
		t.Error("glyph ID for 'A' should not be 0")
	}
	if gidB == 0 {
		t.Error("glyph ID for 'B' should not be 0")
	}
	if gidA == gidB {
		t.Error("'A' and 'B' should have different glyph IDs")
	}
}

func TestEncodeStringTracksGlyphs(t *testing.T) {
	face := loadTestFace(t)
	ef := NewEmbeddedFont(face)

	ef.EncodeString("Hello")
	// Should have tracked unique glyphs: H, e, l, o (4 unique)
	if len(ef.usedGlyphs) != 4 {
		t.Errorf("expected 4 unique glyphs, got %d", len(ef.usedGlyphs))
	}
}

func TestBuildObjects(t *testing.T) {
	face := loadTestFace(t)
	ef := NewEmbeddedFont(face)
	ef.EncodeString("Hello World")

	var objects []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		n := len(objects) + 1
		objects = append(objects, obj)
		return core.NewPdfIndirectReference(n, 0)
	}

	type0 := ef.BuildObjects(addObject)

	// Should have created 4 additional objects:
	// font stream, descriptor, CIDFont, ToUnicode
	if len(objects) != 4 {
		t.Fatalf("expected 4 objects, got %d", len(objects))
	}

	// Verify Type0 dict
	var buf bytes.Buffer
	type0.WriteTo(&buf)
	s := buf.String()

	if !strings.Contains(s, "/Subtype /Type0") {
		t.Error("Type0 dict missing /Subtype /Type0")
	}
	if !strings.Contains(s, "/Encoding /Identity-H") {
		t.Error("Type0 dict missing /Encoding /Identity-H")
	}
	if !strings.Contains(s, "/DescendantFonts") {
		t.Error("Type0 dict missing /DescendantFonts")
	}
	if !strings.Contains(s, "/ToUnicode") {
		t.Error("Type0 dict missing /ToUnicode")
	}
}

func TestBuildObjectsFontDescriptor(t *testing.T) {
	face := loadTestFace(t)
	ef := NewEmbeddedFont(face)
	ef.EncodeString("Test")

	var objects []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		n := len(objects) + 1
		objects = append(objects, obj)
		return core.NewPdfIndirectReference(n, 0)
	}

	ef.BuildObjects(addObject)

	// Object 0: font stream, Object 1: descriptor
	descriptor := objects[1]
	var buf bytes.Buffer
	descriptor.WriteTo(&buf)
	s := buf.String()

	if !strings.Contains(s, "/Type /FontDescriptor") {
		t.Error("descriptor missing /Type /FontDescriptor")
	}
	if !strings.Contains(s, "/FontName") {
		t.Error("descriptor missing /FontName")
	}
	if !strings.Contains(s, "/FontBBox") {
		t.Error("descriptor missing /FontBBox")
	}
	if !strings.Contains(s, "/Ascent") {
		t.Error("descriptor missing /Ascent")
	}
	if !strings.Contains(s, "/Descent") {
		t.Error("descriptor missing /Descent")
	}
	if !strings.Contains(s, "/FontFile2") {
		t.Error("descriptor missing /FontFile2")
	}
}

func TestBuildObjectsCIDFont(t *testing.T) {
	face := loadTestFace(t)
	ef := NewEmbeddedFont(face)
	ef.EncodeString("Hi")

	var objects []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		n := len(objects) + 1
		objects = append(objects, obj)
		return core.NewPdfIndirectReference(n, 0)
	}

	ef.BuildObjects(addObject)

	// Object 2: CIDFont
	cidFont := objects[2]
	var buf bytes.Buffer
	cidFont.WriteTo(&buf)
	s := buf.String()

	if !strings.Contains(s, "/Subtype /CIDFontType2") {
		t.Error("CIDFont missing /Subtype /CIDFontType2")
	}
	if !strings.Contains(s, "/CIDSystemInfo") {
		t.Error("CIDFont missing /CIDSystemInfo")
	}
	if !strings.Contains(s, "/CIDToGIDMap /Identity") {
		t.Error("CIDFont missing /CIDToGIDMap /Identity")
	}
	if !strings.Contains(s, "/W") {
		t.Error("CIDFont missing /W (width array)")
	}
}

func TestToUnicodeCMap(t *testing.T) {
	face := loadTestFace(t)
	ef := NewEmbeddedFont(face)
	ef.EncodeString("AB")

	cmap := ef.buildToUnicodeCMap()

	if !strings.Contains(cmap, "beginbfchar") {
		t.Error("CMap missing beginbfchar")
	}
	if !strings.Contains(cmap, "endbfchar") {
		t.Error("CMap missing endbfchar")
	}
	if !strings.Contains(cmap, "begincodespacerange") {
		t.Error("CMap missing begincodespacerange")
	}
	// Should map glyph IDs for A (0x0041) and B (0x0042)
	if !strings.Contains(cmap, "<0041>") {
		t.Error("CMap missing Unicode mapping for 'A'")
	}
	if !strings.Contains(cmap, "<0042>") {
		t.Error("CMap missing Unicode mapping for 'B'")
	}
}

func TestBuildToUnicodeCMapFiltersNonBMP(t *testing.T) {
	face := loadTestFace(t)
	ef := NewEmbeddedFont(face)

	// Encode a normal BMP character so usedGlyphs is not empty.
	ef.EncodeString("A")

	// Manually insert a mapping with a non-BMP rune (> 0xFFFF).
	// Use a fake glyph ID that won't collide with 'A'.
	nonBMPGID := uint16(9999)
	ef.usedGlyphs[nonBMPGID] = 0x1F600 // U+1F600 (emoji, outside BMP)

	cmap := ef.buildToUnicodeCMap()

	// The CMap should NOT contain an entry for the non-BMP glyph ID.
	needle := fmt.Sprintf("<%04X>", nonBMPGID)
	if strings.Contains(cmap, needle) {
		t.Errorf("CMap should not contain entry for non-BMP glyph %s, but it does", needle)
	}

	// The CMap should still contain the BMP entry for 'A'.
	if !strings.Contains(cmap, "<0041>") {
		t.Error("CMap missing expected BMP mapping for 'A'")
	}
}

func TestSubsetTagFormat(t *testing.T) {
	glyphs := map[uint16]rune{
		0:  0,
		36: 'A',
		37: 'B',
		72: 'Z',
	}

	tag := subsetTag(glyphs)

	if len(tag) != 6 {
		t.Fatalf("subsetTag should produce 6 characters, got %d: %q", len(tag), tag)
	}
	for i, c := range tag {
		if c < 'A' || c > 'Z' {
			t.Errorf("subsetTag char %d = %q, want uppercase A-Z", i, string(c))
		}
	}
}

func TestSanitizePSName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ArialMT", "ArialMT"},
		{"Times New Roman", "Times-New-Roman"},
		{"Font(Bold)", "FontBold"},
		{"Name/With/Slashes", "NameWithSlashes"},
	}
	for _, tc := range tests {
		got := sanitizePSName(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizePSName(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}
