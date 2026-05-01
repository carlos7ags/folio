// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
)

// cmapTable maps a Unicode codepoint to a glyph ID. It is the in-memory
// form of the cmap table parsed by [parseCmapTable]. Lookups for missing
// runes return 0 (.notdef) by convention.
//
// Folio parses the cmap directly from raw font bytes rather than relying
// on golang.org/x/image/font/sfnt, which has a hardcoded
// `maxCmapSegments = 20000` limit (sfnt.go:69) that rejects fonts most
// CJK users have installed (Microsoft YaHei, Noto Sans CJK, STHeiti).
// The downstream sfnt face in [sfntFace] consults this map via
// [sfntFace.GlyphIndex] when the source font tripped the sfnt limit.
//
// Layout reference: ISO/IEC 14496-22 §5.2.1 (Open Font Format) and the
// Apple TrueType Reference, "The 'cmap' table".
type cmapTable map[rune]uint16

// parseCmapTable reads the cmap table from a raw single-font sfnt blob
// (TTF / OTF / extracted-from-TTC) and returns the best-available
// Unicode mapping. The picker prefers segmented coverage (format 12)
// over BMP-only formats (format 4) and Unicode platforms (0, 3) over
// legacy Mac platforms (1).
//
// Returns an error when the cmap table is missing, truncated, or has
// no Unicode subtable in a supported format. Format 14 (Unicode
// Variation Selectors) is recognized and skipped — the base mapping is
// returned unchanged, matching browser fallback when the user agent
// has no variation-sequence support.
func parseCmapTable(rawFont []byte) (cmapTable, error) {
	cmap := findTable(rawFont, "cmap")
	if cmap == nil {
		return nil, fmt.Errorf("cmap: table not found: %w", ErrMissingTable)
	}
	if len(cmap) < 4 {
		return nil, fmt.Errorf("cmap: table header truncated: %w", ErrTruncated)
	}
	numSubtables := int(binary.BigEndian.Uint16(cmap[2:4]))
	if len(cmap) < 4+numSubtables*8 {
		return nil, fmt.Errorf("cmap: encoding records truncated: %w", ErrTruncated)
	}

	// Subtable selection priority. Higher score wins. The scoring
	// captures the practical preference order browsers and font tools
	// use for url() references; the format-12 entries dominate because
	// they cover the full Unicode range, which is what CJK fonts need.
	type candidate struct {
		offset uint64
		format uint16
		score  int
	}
	var best candidate
	for i := range numSubtables {
		base := 4 + i*8
		platformID := binary.BigEndian.Uint16(cmap[base : base+2])
		encodingID := binary.BigEndian.Uint16(cmap[base+2 : base+4])
		offset := uint64(binary.BigEndian.Uint32(cmap[base+4 : base+8]))
		if offset+2 > uint64(len(cmap)) {
			continue
		}
		format := binary.BigEndian.Uint16(cmap[offset : offset+2])

		score := scoreSubtable(platformID, encodingID, format)
		if score > best.score {
			best = candidate{offset: offset, format: format, score: score}
		}
	}
	if best.score == 0 {
		return nil, fmt.Errorf("cmap: no supported Unicode subtable: %w", ErrCorruptTable)
	}

	switch best.format {
	case 4:
		return parseCmapFormat4(cmap, best.offset)
	case 12:
		return parseCmapFormat12(cmap, best.offset)
	}
	return nil, fmt.Errorf("cmap: subtable format %d not supported: %w", best.format, ErrCorruptTable)
}

// scoreSubtable returns a non-zero priority for cmap subtables we can
// read. Higher means preferred. Returns 0 for subtables we ignore
// (legacy Mac formats 0/2/6, format 14 variation selectors, or
// platform/encoding combinations outside the Unicode mappings we
// support).
//
// The numeric gaps are deliberate so future entries can slot between
// the existing tiers without re-ordering everything.
func scoreSubtable(platformID, encodingID, format uint16) int {
	// Unicode platform (0): all encodings under this platform are
	// Unicode by definition. Encoding 4 means "Unicode 2.0 and beyond,
	// non-BMP allowed" — the only complete-Unicode option.
	switch platformID {
	case 0:
		switch encodingID {
		case 4, 6: // Unicode full repertoire (4 = Unicode 2.0+, 6 = Unicode Variation Sequences but used as full mapping)
			if format == 12 {
				return 100
			}
			if format == 4 {
				return 60
			}
		case 3: // Unicode 2.0 BMP only
			if format == 4 {
				return 50
			}
		}
	case 3: // Microsoft platform
		switch encodingID {
		case 10: // Unicode UCS-4 — full repertoire
			if format == 12 {
				return 90
			}
		case 1: // Unicode UCS-2 — BMP only
			if format == 4 {
				return 40
			}
		}
	}
	return 0
}

// parseCmapFormat4 reads a segmented-mapping-to-delta-values subtable
// (Unicode BMP only, codepoints 0x0000..0xFFFF). Format 4 is the most
// common encoding for non-CJK fonts; CJK fonts include it as a
// fallback alongside format 12.
//
// Layout: ISO/IEC 14496-22 §5.2.1.3.4. The arrays endCode[],
// startCode[], idDelta[], idRangeOffset[] are each segCount entries
// long. A glyph ID for a codepoint is computed from the segment that
// contains it via either idDelta (when idRangeOffset is 0) or an
// indirect lookup into glyphIdArray[] (when idRangeOffset is non-zero).
func parseCmapFormat4(cmap []byte, off uint64) (cmapTable, error) {
	if off+14 > uint64(len(cmap)) {
		return nil, fmt.Errorf("cmap: format 4 header truncated: %w", ErrTruncated)
	}
	length := uint64(binary.BigEndian.Uint16(cmap[off+2 : off+4]))
	if off+length > uint64(len(cmap)) {
		return nil, fmt.Errorf("cmap: format 4 length exceeds table: %w", ErrTruncated)
	}
	segCountX2 := uint64(binary.BigEndian.Uint16(cmap[off+6 : off+8]))
	if segCountX2 == 0 || segCountX2%2 != 0 {
		return nil, fmt.Errorf("cmap: format 4 segCountX2 invalid: %w", ErrCorruptTable)
	}
	segCount := segCountX2 / 2

	// Layout offsets within the subtable.
	endCodeOff := off + 14
	// Skip endCode + reservedPad to reach startCode.
	startCodeOff := endCodeOff + segCountX2 + 2
	idDeltaOff := startCodeOff + segCountX2
	idRangeOffsetOff := idDeltaOff + segCountX2
	glyphIdArrayOff := idRangeOffsetOff + segCountX2

	if glyphIdArrayOff > uint64(len(cmap)) {
		return nil, fmt.Errorf("cmap: format 4 arrays extend past table: %w", ErrTruncated)
	}

	out := make(cmapTable, segCount*8) // rough capacity guess

	for i := uint64(0); i < segCount; i++ {
		endCode := uint32(binary.BigEndian.Uint16(cmap[endCodeOff+i*2 : endCodeOff+i*2+2]))
		startCode := uint32(binary.BigEndian.Uint16(cmap[startCodeOff+i*2 : startCodeOff+i*2+2]))
		idDelta := uint32(int32(int16(binary.BigEndian.Uint16(cmap[idDeltaOff+i*2 : idDeltaOff+i*2+2]))))
		idRangeOffset := uint64(binary.BigEndian.Uint16(cmap[idRangeOffsetOff+i*2 : idRangeOffsetOff+i*2+2]))

		// The terminating segment has startCode=endCode=0xFFFF; skip it
		// to avoid mapping U+FFFF (which is not a real character) to
		// glyph 0xFFFF unintentionally.
		if startCode == 0xFFFF && endCode == 0xFFFF {
			continue
		}

		for c := startCode; c <= endCode; c++ {
			var gid uint16
			if idRangeOffset == 0 {
				gid = uint16(c + idDelta)
			} else {
				// idRangeOffset is a byte offset from the START of the
				// idRangeOffset[i] entry to the glyphIdArray entry
				// holding our glyph id. The spec calls this the "obscure
				// indexing trick"; it works out to:
				//
				//   *(&idRangeOffset[i]) + idRangeOffset/2 + (c - startCode)
				//
				// expressed in words. In bytes:
				addr := idRangeOffsetOff + i*2 + idRangeOffset + (uint64(c-startCode) * 2)
				if addr+2 > uint64(len(cmap)) {
					continue
				}
				rawGID := binary.BigEndian.Uint16(cmap[addr : addr+2])
				if rawGID == 0 {
					continue // unmapped within this range
				}
				gid = uint16(uint32(rawGID) + idDelta)
			}
			if gid != 0 {
				out[rune(c)] = gid
			}
		}
	}
	return out, nil
}

// parseCmapFormat12 reads a segmented-coverage subtable. This is the
// format used by every modern CJK font for full Unicode coverage; it
// is also the format that trips sfnt's maxCmapSegments=20000 limit on
// fonts like Microsoft YaHei (~25k groups) and Noto Sans CJK
// (~30k groups).
//
// Layout: ISO/IEC 14496-22 §5.2.1.3.7. The header is 16 bytes; each
// of the numGroups records that follow is 12 bytes:
// (startCharCode, endCharCode, startGlyphID).
func parseCmapFormat12(cmap []byte, off uint64) (cmapTable, error) {
	if off+16 > uint64(len(cmap)) {
		return nil, fmt.Errorf("cmap: format 12 header truncated: %w", ErrTruncated)
	}
	length := uint64(binary.BigEndian.Uint32(cmap[off+4 : off+8]))
	if off+length > uint64(len(cmap)) {
		return nil, fmt.Errorf("cmap: format 12 length exceeds table: %w", ErrTruncated)
	}
	numGroups := uint64(binary.BigEndian.Uint32(cmap[off+12 : off+16]))
	groupsOff := off + 16
	if groupsOff+numGroups*12 > uint64(len(cmap)) {
		return nil, fmt.Errorf("cmap: format 12 groups truncated: %w", ErrTruncated)
	}

	// Pre-allocate based on a rough total-codepoints estimate. Format 12
	// fonts cover tens of thousands of codepoints; an under-allocation
	// just causes Go to grow the map and is harmless.
	out := make(cmapTable, numGroups*8)

	for i := uint64(0); i < numGroups; i++ {
		groupBase := groupsOff + i*12
		startCharCode := binary.BigEndian.Uint32(cmap[groupBase : groupBase+4])
		endCharCode := binary.BigEndian.Uint32(cmap[groupBase+4 : groupBase+8])
		startGlyphID := binary.BigEndian.Uint32(cmap[groupBase+8 : groupBase+12])

		// Spec doesn't bound endCharCode at 0x10FFFF but values above
		// it are not legal Unicode; skip them defensively.
		if startCharCode > 0x10FFFF {
			continue
		}
		if endCharCode > 0x10FFFF {
			endCharCode = 0x10FFFF
		}
		if endCharCode < startCharCode {
			continue
		}

		for c := startCharCode; c <= endCharCode; c++ {
			gidOffset := c - startCharCode
			gid := startGlyphID + gidOffset
			if gid > 0xFFFF {
				continue // sfnt glyph IDs are uint16; out-of-range entries are unmapped
			}
			if gid != 0 {
				out[rune(c)] = uint16(gid)
			}
		}
	}
	return out, nil
}
