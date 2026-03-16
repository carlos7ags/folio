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

// sfntFace implements Face using golang.org/x/image/font/sfnt.
// This is an internal implementation — callers use the Face interface.
type sfntFace struct {
	font    *sfnt.Font
	rawData []byte
	buf     sfnt.Buffer // reusable buffer for sfnt operations
	ppem    fixed.Int26_6

	// Cached table data from raw TTF (parsed lazily).
	tables       map[string][]byte
	tablesParsed bool
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

func (f *sfntFace) PostScriptName() string {
	name, err := f.font.Name(&f.buf, sfnt.NameIDPostScript)
	if err != nil || name == "" {
		name, _ = f.font.Name(&f.buf, sfnt.NameIDFull)
	}
	return name
}

func (f *sfntFace) UnitsPerEm() int {
	return int(f.font.UnitsPerEm())
}

func (f *sfntFace) GlyphIndex(r rune) uint16 {
	idx, err := f.font.GlyphIndex(&f.buf, r)
	if err != nil {
		return 0
	}
	return uint16(idx)
}

func (f *sfntFace) GlyphAdvance(glyphID uint16) int {
	adv, err := f.font.GlyphAdvance(&f.buf, sfnt.GlyphIndex(glyphID), f.ppem, xfont.HintingNone)
	if err != nil {
		return 0
	}
	return fix26_6ToInt(adv)
}

func (f *sfntFace) Ascent() int {
	metrics, err := f.font.Metrics(&f.buf, f.ppem, xfont.HintingNone)
	if err != nil {
		return 0
	}
	return fix26_6ToInt(metrics.Ascent)
}

func (f *sfntFace) Descent() int {
	metrics, err := f.font.Metrics(&f.buf, f.ppem, xfont.HintingNone)
	if err != nil {
		return 0
	}
	// sfnt returns descent as a positive number; PDF expects negative
	return -fix26_6ToInt(metrics.Descent)
}

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

func (f *sfntFace) rawTables() map[string][]byte {
	if !f.tablesParsed {
		f.tables, _ = parseTTFTables(f.rawData)
		f.tablesParsed = true
	}
	return f.tables
}

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

func (f *sfntFace) Kern(left, right uint16) int {
	tables := f.rawTables()
	if tables == nil {
		return 0
	}
	kern, ok := tables["kern"]
	if !ok || len(kern) < 4 {
		return 0
	}
	return lookupKernPair(kern, left, right)
}

// lookupKernPair searches the kern table for a glyph pair.
// Supports format 0 subtables (the most common format).
func lookupKernPair(data []byte, left, right uint16) int {
	if len(data) < 4 {
		return 0
	}
	version := binary.BigEndian.Uint16(data[0:2])
	nTables := binary.BigEndian.Uint16(data[2:4])
	offset := 4

	// Version 0 kern table (Windows/TrueType style).
	if version == 0 {
		for range int(nTables) {
			if offset+6 > len(data) {
				break
			}
			// subtable header: version(2) + length(2) + coverage(2)
			subtableLen := int(binary.BigEndian.Uint16(data[offset+2 : offset+4]))
			coverage := binary.BigEndian.Uint16(data[offset+4 : offset+6])

			// Validate subtable bounds.
			if subtableLen < 6 || offset+subtableLen > len(data) {
				break
			}

			// coverage: bits 0-7 = format, bit 0 of high byte = horizontal
			format := coverage & 0xFF
			horizontal := (coverage & 0x0100) != 0

			if format == 0 && horizontal {
				val := lookupKernFormat0(data[offset+6:offset+subtableLen], left, right)
				if val != 0 {
					return val
				}
			}
			offset += subtableLen
		}
		return 0
	}

	// Version 1 kern table (macOS/AAT style) — less common but worth supporting.
	if version == 1 && len(data) >= 8 {
		nTables32 := binary.BigEndian.Uint32(data[4:8])
		offset = 8
		for range int(nTables32) {
			if offset+8 > len(data) {
				break
			}
			subtableLen := int(binary.BigEndian.Uint32(data[offset : offset+4]))
			coverage := binary.BigEndian.Uint16(data[offset+4 : offset+6])

			// Validate subtable bounds.
			if subtableLen < 8 || offset+subtableLen > len(data) {
				break
			}

			format := coverage & 0xFF
			if format == 0 {
				val := lookupKernFormat0(data[offset+8:offset+subtableLen], left, right)
				if val != 0 {
					return val
				}
			}
			offset += subtableLen
		}
	}

	return 0
}

// lookupKernFormat0 searches a format 0 kern subtable for the given pair.
// Format 0 has: nPairs(2), searchRange(2), entrySelector(2), rangeShift(2)
// followed by nPairs entries of: left(2) + right(2) + value(2).
func lookupKernFormat0(data []byte, left, right uint16) int {
	if len(data) < 8 {
		return 0
	}
	nPairs := int(binary.BigEndian.Uint16(data[0:2]))
	pairData := data[8:] // skip nPairs, searchRange, entrySelector, rangeShift

	// Binary search for the pair (pairs are sorted by (left, right)).
	key := uint32(left)<<16 | uint32(right)
	lo, hi := 0, nPairs-1
	for lo <= hi {
		mid := (lo + hi) / 2
		off := mid * 6
		if off+6 > len(pairData) {
			break
		}
		pairLeft := binary.BigEndian.Uint16(pairData[off : off+2])
		pairRight := binary.BigEndian.Uint16(pairData[off+2 : off+4])
		pairKey := uint32(pairLeft)<<16 | uint32(pairRight)

		if pairKey == key {
			return int(int16(binary.BigEndian.Uint16(pairData[off+4 : off+6])))
		} else if pairKey < key {
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return 0
}

func (f *sfntFace) Flags() uint32 {
	// PDF font flags (Table 123 in ISO 32000):
	// Bit 6 (value 32): Nonsymbolic — using standard Latin encoding.
	// This is the common case for TrueType fonts with Unicode cmap.
	return 32
}

func (f *sfntFace) RawData() []byte {
	return f.rawData
}

func (f *sfntFace) NumGlyphs() int {
	return f.font.NumGlyphs()
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
