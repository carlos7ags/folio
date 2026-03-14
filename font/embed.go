// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/carlos7ags/folio/core"
)

// EmbeddedFont holds the PDF objects needed to embed a TrueType font
// as a CIDFont (Type0 composite font) in a PDF document.
//
// Object tree:
//
//	Type0 font dict (/Subtype /Type0)
//	  ├── /DescendantFonts → [CIDFont dict]
//	  │     └── /FontDescriptor dict
//	  │           └── /FontFile2 → compressed font stream
//	  ├── /ToUnicode → CMap stream
//	  └── /Encoding /Identity-H
type EmbeddedFont struct {
	face Face

	// usedGlyphs tracks which glyph IDs have been referenced.
	// Maps glyph ID → rune (for ToUnicode CMap).
	usedGlyphs map[uint16]rune
}

// NewEmbeddedFont creates an embeddable font from a parsed Face.
func NewEmbeddedFont(face Face) *EmbeddedFont {
	return &EmbeddedFont{
		face:       face,
		usedGlyphs: make(map[uint16]rune),
	}
}

// Face returns the underlying Face.
func (ef *EmbeddedFont) Face() Face {
	return ef.face
}

// ResetGlyphs clears the used-glyph tracker so the font can be reused
// in a new document without carrying over glyphs from a previous one.
func (ef *EmbeddedFont) ResetGlyphs() {
	ef.usedGlyphs = make(map[uint16]rune)
}

// EncodeString converts a Unicode string to glyph IDs encoded as
// big-endian uint16 pairs (Identity-H encoding). It also records
// which glyphs are used for subsetting and ToUnicode generation.
func (ef *EmbeddedFont) EncodeString(s string) []byte {
	var buf []byte
	for _, r := range s {
		gid := ef.face.GlyphIndex(r)
		ef.usedGlyphs[gid] = r
		buf = append(buf, byte(gid>>8), byte(gid&0xFF))
	}
	return buf
}

// Kern returns the kerning value between two runes in PDF text space units
// (thousandths of a unit of text space). Negative values mean glyphs should
// be closer together. Returns 0 if no kerning data is available.
func (ef *EmbeddedFont) Kern(left, right rune) float64 {
	leftGID := ef.face.GlyphIndex(left)
	rightGID := ef.face.GlyphIndex(right)
	if leftGID == 0 || rightGID == 0 {
		return 0
	}
	kernUnits := ef.face.Kern(leftGID, rightGID)
	if kernUnits == 0 {
		return 0
	}
	// Convert from font design units to 1/1000 of text space unit.
	return float64(kernUnits) * 1000.0 / float64(ef.face.UnitsPerEm())
}

// PdfObjects builds the complete set of PDF objects for this embedded font.
// The caller is responsible for registering these as indirect objects.
//
// Returns:
//   - type0Dict: the top-level Type0 font dictionary (goes in page /Resources /Font)
//   - objects: additional objects that must be registered (CIDFont, descriptor,
//     font stream, ToUnicode CMap). The type0Dict references these via
//     indirect references that the caller must wire up.
//
// Because indirect references require object numbers (assigned by the Writer),
// this method returns a builder function that takes an "add object" callback.
func (ef *EmbeddedFont) BuildObjects(addObject func(core.PdfObject) *core.PdfIndirectReference) *core.PdfDictionary {
	face := ef.face
	psName := sanitizePSName(face.PostScriptName())
	upem := face.UnitsPerEm()

	// 1. Font stream (subset TTF, compressed)
	// Always include .notdef (GID 0).
	glyphs := make(map[uint16]rune, len(ef.usedGlyphs)+1)
	glyphs[0] = 0
	maps.Copy(glyphs, ef.usedGlyphs)
	fontData := face.RawData()
	if subsetData, err := Subset(fontData, glyphs); err == nil {
		fontData = subsetData
		// Add subset tag prefix (e.g. "ABCDEF+FontName") per PDF spec
		psName = subsetTag(glyphs) + "+" + psName
	}
	// If subsetting fails, fall back to embedding the full font.

	fontStream := core.NewPdfStreamCompressed(fontData)
	fontStream.Dict.Set("Length1", core.NewPdfInteger(len(fontData)))
	fontStreamRef := addObject(fontStream)

	// 2. Font descriptor
	bbox := face.BBox()
	descriptor := core.NewPdfDictionary()
	descriptor.Set("Type", core.NewPdfName("FontDescriptor"))
	descriptor.Set("FontName", core.NewPdfName(psName))
	descriptor.Set("Flags", core.NewPdfInteger(int(face.Flags())))
	descriptor.Set("FontBBox", core.NewPdfArray(
		core.NewPdfInteger(bbox[0]),
		core.NewPdfInteger(bbox[1]),
		core.NewPdfInteger(bbox[2]),
		core.NewPdfInteger(bbox[3]),
	))
	descriptor.Set("ItalicAngle", core.NewPdfReal(face.ItalicAngle()))
	descriptor.Set("Ascent", core.NewPdfInteger(face.Ascent()))
	descriptor.Set("Descent", core.NewPdfInteger(face.Descent()))
	capHeight := face.CapHeight()
	if capHeight == 0 {
		capHeight = face.Ascent() // fallback if OS/2 table missing
	}
	descriptor.Set("CapHeight", core.NewPdfInteger(capHeight))
	stemV := face.StemV()
	if stemV == 0 {
		stemV = 80 // fallback
	}
	descriptor.Set("StemV", core.NewPdfInteger(stemV))
	descriptor.Set("FontFile2", fontStreamRef)
	descriptorRef := addObject(descriptor)

	// 3. CIDFont dictionary (DescendantFont)
	cidFont := core.NewPdfDictionary()
	cidFont.Set("Type", core.NewPdfName("Font"))
	cidFont.Set("Subtype", core.NewPdfName("CIDFontType2"))
	cidFont.Set("BaseFont", core.NewPdfName(psName))
	cidFont.Set("CIDSystemInfo", buildCIDSystemInfo())
	cidFont.Set("FontDescriptor", descriptorRef)
	cidFont.Set("DW", core.NewPdfInteger(1000)) // default width
	cidFont.Set("W", buildWidthArray(ef, upem))
	// Identity mapping: CID = GID
	cidFont.Set("CIDToGIDMap", core.NewPdfName("Identity"))
	cidFontRef := addObject(cidFont)

	// 4. ToUnicode CMap
	toUnicode := core.NewPdfStreamCompressed([]byte(ef.buildToUnicodeCMap()))
	toUnicodeRef := addObject(toUnicode)

	// 5. Top-level Type0 font dictionary
	type0 := core.NewPdfDictionary()
	type0.Set("Type", core.NewPdfName("Font"))
	type0.Set("Subtype", core.NewPdfName("Type0"))
	type0.Set("BaseFont", core.NewPdfName(psName))
	type0.Set("Encoding", core.NewPdfName("Identity-H"))
	type0.Set("DescendantFonts", core.NewPdfArray(cidFontRef))
	type0.Set("ToUnicode", toUnicodeRef)

	return type0
}

// buildCIDSystemInfo returns the required CIDSystemInfo dictionary.
func buildCIDSystemInfo() *core.PdfDictionary {
	d := core.NewPdfDictionary()
	d.Set("Registry", core.NewPdfLiteralString("Adobe"))
	d.Set("Ordering", core.NewPdfLiteralString("Identity"))
	d.Set("Supplement", core.NewPdfInteger(0))
	return d
}

// buildWidthArray builds the /W array for the CIDFont.
// Format: [cid [w1 w2 ...] cid [w1 w2 ...] ...]
// We use consecutive ranges for efficiency.
func buildWidthArray(ef *EmbeddedFont, upem int) *core.PdfArray {
	w := core.NewPdfArray()

	// Collect used glyph IDs and sort them
	if len(ef.usedGlyphs) == 0 {
		return w
	}

	// Find min/max GID
	var minGID, maxGID uint16
	first := true
	for gid := range ef.usedGlyphs {
		if first || gid < minGID {
			minGID = gid
		}
		if first || gid > maxGID {
			maxGID = gid
		}
		first = false
	}

	// Build width entries for each used glyph
	// Simple approach: one entry per glyph [cid [width]]
	for gid := minGID; gid <= maxGID; gid++ {
		if _, used := ef.usedGlyphs[gid]; !used {
			continue
		}
		advDesignUnits := ef.face.GlyphAdvance(gid)
		// Convert from design units to 1/1000 of text space unit
		width := advDesignUnits * 1000 / upem
		w.Add(core.NewPdfInteger(int(gid)))
		w.Add(core.NewPdfArray(core.NewPdfInteger(width)))
	}

	return w
}

// buildToUnicodeCMap generates a ToUnicode CMap that maps glyph IDs
// back to Unicode codepoints, enabling text search and copy/paste.
func (ef *EmbeddedFont) buildToUnicodeCMap() string {
	var b strings.Builder

	b.WriteString("/CIDInit /ProcSet findresource begin\n")
	b.WriteString("12 dict begin\n")
	b.WriteString("begincmap\n")
	b.WriteString("/CIDSystemInfo\n")
	b.WriteString("<< /Registry (Adobe) /Ordering (UCS) /Supplement 0 >> def\n")
	b.WriteString("/CMapName /Adobe-Identity-UCS def\n")
	b.WriteString("/CMapType 2 def\n")
	b.WriteString("1 begincodespacerange\n")
	b.WriteString("<0000> <FFFF>\n")
	b.WriteString("endcodespacerange\n")

	// Write mappings in chunks of 100 (PDF limit per beginbfchar block).
	// Skip .notdef (GID 0) and non-BMP runes (> 0xFFFF) which require
	// surrogate pairs and a different CMap format.
	var mappings []glyphMapping
	for gid, r := range ef.usedGlyphs {
		if gid == 0 || r > 0xFFFF {
			continue
		}
		mappings = append(mappings, glyphMapping{gid: gid, r: r})
	}

	// Sort for deterministic output
	sortMappings(mappings)

	for i := 0; i < len(mappings); i += 100 {
		end := min(i+100, len(mappings))
		chunk := mappings[i:end]

		fmt.Fprintf(&b, "%d beginbfchar\n", len(chunk))
		for _, m := range chunk {
			fmt.Fprintf(&b, "<%04X> <%04X>\n", m.gid, m.r)
		}
		b.WriteString("endbfchar\n")
	}

	b.WriteString("endcmap\n")
	b.WriteString("CMapName currentdict /CMap defineresource pop\n")
	b.WriteString("end\n")
	b.WriteString("end\n")

	return b.String()
}

// glyphMapping pairs a glyph ID with its Unicode codepoint.
type glyphMapping struct {
	gid uint16
	r   rune
}

// sortMappings sorts by glyph ID for deterministic output.
func sortMappings(m []glyphMapping) {
	slices.SortFunc(m, func(a, b glyphMapping) int {
		return int(a.gid) - int(b.gid)
	})
}

// subsetTag generates a 6-letter uppercase tag from used glyph IDs.
// Per PDF spec, subset fonts use a tag like "ABCDEF+FontName".
func subsetTag(glyphs map[uint16]rune) string {
	// Order-independent hash: XOR and sum are commutative.
	var hash uint32
	for gid := range glyphs {
		hash ^= uint32(gid) * 2654435761 // Knuth multiplicative hash
	}
	var tag [6]byte
	for i := range tag {
		tag[i] = 'A' + byte(hash%26)
		hash /= 26
	}
	return string(tag[:])
}

// sanitizePSName ensures a PostScript name is valid for PDF.
// Replaces spaces with hyphens and removes invalid characters.
func sanitizePSName(name string) string {
	var b strings.Builder
	b.Grow(len(name))
	for _, r := range name {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r >= '!' && r <= '~' && r != '[' && r != ']' &&
			r != '(' && r != ')' && r != '{' && r != '}' &&
			r != '<' && r != '>' && r != '/' && r != '%':
			b.WriteRune(r)
		default:
			// Skip invalid characters
		}
	}
	return b.String()
}
