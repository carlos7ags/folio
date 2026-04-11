// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"

	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// sfntFace is the Face implementation backed by golang.org/x/image/font/sfnt.
// Lazy caches (tables, gsubResult, gidToUnicodeMap, kernPairs) are
// unsynchronized; callers must not share a single sfntFace across
// goroutines. This is an internal implementation — callers use the
// Face interface.
type sfntFace struct {
	font    *sfnt.Font
	rawData []byte
	buf     sfnt.Buffer // reusable buffer for sfnt operations
	ppem    fixed.Int26_6

	// Cached table data from raw TTF (parsed lazily).
	tables       map[string][]byte
	tablesParsed bool

	// Cached GSUB substitution tables. gsubParsed distinguishes "not
	// yet parsed" (false) from "parsed and empty" (true, gsubResult nil).
	gsubResult *GSUBSubstitutions
	gsubParsed bool

	// Cached GID→Unicode reverse map (nil = not yet built).
	gidToUnicodeMap   map[uint16]rune
	gidToUnicodeBuilt bool

	// Cached kern pairs: (leftGID, rightGID) → FUnit value. Populated on
	// the first Kern() call. A nil map after parsing means the font has
	// no kern table or no supported subtables; kernPairsParsed then
	// guards re-parsing.
	kernPairs       map[[2]uint16]int16
	kernPairsParsed bool
}

// ParseTTF parses a TrueType (.ttf) or OpenType (.otf) font from raw bytes.
// Returns a Face that can be used for PDF embedding.
func ParseTTF(data []byte) (Face, error) {
	f, err := sfnt.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	// Set ppem to UnitsPerEm so that all metrics are returned in
	// font design units (as 26.6 fixed-point).
	ppem := fixed.I(int(f.UnitsPerEm()))
	return &sfntFace{
		font:    f,
		rawData: data,
		ppem:    ppem,
	}, nil
}

// LoadTTF reads and parses a TrueType font file from disk.
func LoadTTF(path string) (Face, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read font file: %w", err)
	}
	return ParseTTF(data)
}

// PostScriptName returns the PostScript name, falling back to the full name
// if the PostScript name entry is missing or empty.
func (f *sfntFace) PostScriptName() string {
	name, err := f.font.Name(&f.buf, sfnt.NameIDPostScript)
	if err != nil || name == "" {
		name, _ = f.font.Name(&f.buf, sfnt.NameIDFull)
	}
	return name
}

// UnitsPerEm returns the font's design units per em.
func (f *sfntFace) UnitsPerEm() int {
	return int(f.font.UnitsPerEm())
}

// GlyphIndex returns the glyph ID for r, or 0 if the rune is not in the font.
func (f *sfntFace) GlyphIndex(r rune) uint16 {
	idx, err := f.font.GlyphIndex(&f.buf, r)
	if err != nil {
		return 0
	}
	return uint16(idx)
}

// GlyphAdvance returns the advance width in font design units, or 0 on error.
func (f *sfntFace) GlyphAdvance(glyphID uint16) int {
	adv, err := f.font.GlyphAdvance(&f.buf, sfnt.GlyphIndex(glyphID), f.ppem, xfont.HintingNone)
	if err != nil {
		return 0
	}
	return fix26_6ToInt(adv)
}

// Ascent returns the typographic ascent in font design units.
func (f *sfntFace) Ascent() int {
	metrics, err := f.font.Metrics(&f.buf, f.ppem, xfont.HintingNone)
	if err != nil {
		return 0
	}
	return fix26_6ToInt(metrics.Ascent)
}

// Descent returns the typographic descent as a negative value in font design
// units. The sfnt library returns descent as positive, so this method negates it.
func (f *sfntFace) Descent() int {
	metrics, err := f.font.Metrics(&f.buf, f.ppem, xfont.HintingNone)
	if err != nil {
		return 0
	}
	// sfnt returns descent as a positive number; PDF expects negative
	return -fix26_6ToInt(metrics.Descent)
}

// BBox returns the font bounding box as [xMin, yMin, xMax, yMax] in font
// design units, converted from sfnt's Y-down coordinates to PDF's Y-up system.
func (f *sfntFace) BBox() [4]int {
	bounds, err := f.font.Bounds(&f.buf, f.ppem, xfont.HintingNone)
	if err != nil {
		return [4]int{}
	}
	// sfnt uses Y-increasing-downward; PDF uses Y-increasing-upward.
	// Negate and swap Y values for PDF coordinate system.
	return [4]int{
		fix26_6ToInt(bounds.Min.X),  // xMin
		-fix26_6ToInt(bounds.Max.Y), // yMin (was yMax in sfnt coords)
		fix26_6ToInt(bounds.Max.X),  // xMax
		-fix26_6ToInt(bounds.Min.Y), // yMax (was yMin in sfnt coords)
	}
}

// rawTables lazily parses the raw TTF table directory and caches the result.
func (f *sfntFace) rawTables() map[string][]byte {
	if !f.tablesParsed {
		f.tables, _ = parseTTFTables(f.rawData)
		f.tablesParsed = true
	}
	return f.tables
}

// ItalicAngle returns the italic angle by parsing the post table's Fixed 16.16
// field at offset 4. Returns 0 if the post table is missing or too short.
func (f *sfntFace) ItalicAngle() float64 {
	// Parse italic angle from the post table (offset 4, Fixed 16.16).
	tables := f.rawTables()
	if tables == nil {
		return 0
	}
	post, ok := tables["post"]
	if !ok || len(post) < 8 {
		return 0
	}
	// italicAngle is a Fixed 16.16 at offset 4.
	raw := binary.BigEndian.Uint32(post[4:8])
	intPart := int16(raw >> 16)
	fracPart := float64(raw&0xFFFF) / 65536.0
	return float64(intPart) + fracPart
}

// CapHeight returns the cap height from the OS/2 table (sCapHeight at offset
// 88). Requires OS/2 version >= 2. Returns 0 if unavailable.
func (f *sfntFace) CapHeight() int {
	// OS/2 table, sCapHeight at offset 88 (requires version >= 2).
	tables := f.rawTables()
	if tables == nil {
		return 0
	}
	os2, ok := tables["OS/2"]
	if !ok || len(os2) < 90 {
		return 0
	}
	// Check version >= 2 (offset 0).
	version := binary.BigEndian.Uint16(os2[0:2])
	if version < 2 {
		return 0
	}
	return int(int16(binary.BigEndian.Uint16(os2[88:90])))
}

// StemV derives the dominant vertical stem width from the OS/2 usWeightClass
// using the formula: 10 + 220*(weightClass-50)/900, clamped to a minimum of 10.
// Returns 80 as a fallback if the OS/2 table is missing.
func (f *sfntFace) StemV() int {
	// Derive from OS/2 usWeightClass (offset 4).
	// Formula: StemV = 10 + 220 * (weightClass - 50) / 900
	// Clamp to reasonable range.
	tables := f.rawTables()
	if tables == nil {
		return 80
	}
	os2, ok := tables["OS/2"]
	if !ok || len(os2) < 6 {
		return 80
	}
	weightClass := int(binary.BigEndian.Uint16(os2[4:6]))
	stemV := int(math.Round(10 + 220*float64(weightClass-50)/900))
	return max(stemV, 10)
}

// Kern returns the kerning adjustment between two glyphs by consulting
// the cached parsed kern table. Returns 0 if the font has no kern table,
// no supported subtables, or no entry for the pair. The cache is built
// on the first call.
func (f *sfntFace) Kern(left, right uint16) int {
	if !f.kernPairsParsed {
		if tables := f.rawTables(); tables != nil {
			if kern, ok := tables["kern"]; ok {
				f.kernPairs = ParseKern(kern)
			}
		}
		f.kernPairsParsed = true
	}
	if f.kernPairs == nil {
		return 0
	}
	return int(f.kernPairs[[2]uint16{left, right}])
}

// Flags returns the PDF font descriptor flags per ISO 32000 Table 123.
// Bits are computed from font metadata: FixedPitch (bit 0), Serif (bit 1),
// Symbolic (bit 2), Nonsymbolic (bit 5), Italic (bit 6).
func (f *sfntFace) Flags() uint32 {
	var flags uint32

	// Bit 0 (1): FixedPitch — check post table isFixedPitch field.
	if f.isFixedPitch() {
		flags |= 1
	}

	// Bit 1 (2): Serif — check OS/2 sFamilyClass.
	if f.isSerif() {
		flags |= 2
	}

	// Bit 2 (4) vs Bit 5 (32): Symbolic vs Nonsymbolic (mutually exclusive).
	// A font with a Unicode cmap that can map 'A' is Nonsymbolic.
	if f.GlyphIndex('A') != 0 {
		flags |= 32 // Nonsymbolic
	} else {
		flags |= 4 // Symbolic
	}

	// Bit 6 (64): Italic — check italic angle or OS/2 fsSelection.
	if f.ItalicAngle() != 0 || f.isItalicFromOS2() {
		flags |= 64
	}

	return flags
}

// isFixedPitch checks the post table isFixedPitch field (offset 12).
func (f *sfntFace) isFixedPitch() bool {
	tables := f.rawTables()
	if tables == nil {
		return false
	}
	post, ok := tables["post"]
	if !ok || len(post) < 16 {
		return false
	}
	return binary.BigEndian.Uint32(post[12:16]) != 0
}

// isSerif checks the OS/2 sFamilyClass field (offset 30-31).
// Family classes 1-5 and 7 indicate serif fonts.
func (f *sfntFace) isSerif() bool {
	tables := f.rawTables()
	if tables == nil {
		return false
	}
	os2, ok := tables["OS/2"]
	if !ok || len(os2) < 32 {
		return false
	}
	class := int(int16(binary.BigEndian.Uint16(os2[30:32]))) >> 8 // high byte is class ID
	return class >= 1 && class <= 5 || class == 7
}

// isItalicFromOS2 checks OS/2 fsSelection bit 0 (Italic).
func (f *sfntFace) isItalicFromOS2() bool {
	tables := f.rawTables()
	if tables == nil {
		return false
	}
	os2, ok := tables["OS/2"]
	if !ok || len(os2) < 64 {
		return false
	}
	fsSelection := binary.BigEndian.Uint16(os2[62:64])
	return fsSelection&1 != 0
}

// RawData returns the complete, unmodified font file bytes.
func (f *sfntFace) RawData() []byte {
	return f.rawData
}

// NumGlyphs returns the total number of glyphs in the font.
func (f *sfntFace) NumGlyphs() int {
	return f.font.NumGlyphs()
}

// GSUB returns the parsed GSUB substitution tables. The result is cached
// after the first call; a nil return means the font has no GSUB tables
// for any of the recognized features.
func (f *sfntFace) GSUB() *GSUBSubstitutions {
	if f.gsubParsed {
		return f.gsubResult
	}
	f.gsubResult = ParseGSUB(f.rawData)
	f.gsubParsed = true
	return f.gsubResult
}

// GIDToUnicode returns a reverse mapping from glyph ID to Unicode codepoint.
// Built lazily from the font's cmap table. Used to convert GSUB-substituted
// GIDs back to codepoints for the text rendering pipeline.
func (f *sfntFace) GIDToUnicode() map[uint16]rune {
	if f.gidToUnicodeBuilt {
		return f.gidToUnicodeMap
	}
	f.gidToUnicodeBuilt = true
	f.gidToUnicodeMap = BuildGIDToUnicode(f.rawData)
	return f.gidToUnicodeMap
}

// BuildGIDToUnicode parses a TrueType/OpenType font and builds a map
// from glyph ID to Unicode code point by scanning the font's cmap table.
// This is used as a fallback for CIDFont text extraction when no
// ToUnicode CMap is provided.
//
// The approach scans the Unicode BMP range (U+0000 to U+FFFF) and queries
// the font for each rune's glyph index, then builds the reverse mapping.
// First rune wins if multiple runes map to the same GID.
// Returns nil if parsing fails.
func BuildGIDToUnicode(fontData []byte) map[uint16]rune {
	f, err := sfnt.Parse(fontData)
	if err != nil {
		return nil
	}

	var buf sfnt.Buffer
	gidMap := make(map[uint16]rune)

	// Scan the full Unicode BMP (U+0000 to U+FFFF).
	for r := rune(0); r <= 0xFFFF; r++ {
		gid, err := f.GlyphIndex(&buf, r)
		if err != nil || gid == 0 {
			continue
		}
		g := uint16(gid)
		// First rune wins — don't overwrite if already mapped.
		if _, exists := gidMap[g]; !exists {
			gidMap[g] = r
		}
	}

	if len(gidMap) == 0 {
		return nil
	}
	return gidMap
}

// fix26_6ToInt converts a fixed.Int26_6 to a rounded integer.
func fix26_6ToInt(v fixed.Int26_6) int {
	return int((v + 32) >> 6)
}
