// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
)

// cmapTable is the resolved Unicode → glyph-ID mapping built from the
// font's `cmap` table.
//
// The map representation is acceptable here because cmap is a sparse,
// random-access lookup; production callers iterate the input string,
// not the table. A future optimisation could replace this with a
// dense bitmap or an interval list, but the current shape matches
// what sfnt exposed and keeps the diff focused.
type cmapTable map[rune]uint16

// cmapMaxMappedCodes bounds the total number of code points expanded
// across all groups/segments of one subtable. Non-overlapping coverage
// cannot exceed 0x110000 (~1.1M) codes; overlapping ranges are only
// produced by hostile or broken fonts. 1<<22 (~4.2M) leaves ~4x
// headroom over the theoretical legitimate maximum.
const cmapMaxMappedCodes = 1 << 22

// parseCmapTable decodes a font's `cmap` table, picks the best Unicode
// subtable available, and returns the resolved mapping.
//
// Spec references: ISO/IEC 14496-22
//   - §5.2.1.3      cmap header layout
//   - §5.2.1.3.4    Format 4 — segment-mapping for BMP coverage
//   - §5.2.1.3.7    Format 12 — sequential map for full Unicode coverage
//
// Subtable selection prefers full Unicode (format 12, Platform 0
// Encoding 4) over BMP-only formats. CJK fonts in the wild commonly
// ship a format-12 subtable with tens of thousands of groups; the
// previously-used golang.org/x/image/font/sfnt parser hard-coded a
// 20000-segment limit that rejected those fonts (issue #248).
//
// The cmap header layout:
//   - version (uint16)            at 0
//   - numTables (uint16)          at 2
//   - encodingRecord[numTables]   at 4
//
// Each encodingRecord is 8 bytes:
//   - platformID (uint16)
//   - encodingID (uint16)
//   - subtableOffset (Offset32)
func parseCmapTable(data []byte) (cmapTable, error) {
	dataLen := uint64(len(data))
	if dataLen < 4 {
		return nil, fmt.Errorf("font: cmap: header truncated (%d < 4): %w", dataLen, ErrTruncated)
	}
	numTables := uint64(binary.BigEndian.Uint16(data[2:4]))
	recordsEnd := 4 + numTables*8
	if recordsEnd > dataLen {
		return nil, fmt.Errorf("font: cmap: encoding records truncated (need %d, have %d): %w", recordsEnd, dataLen, ErrTruncated)
	}

	type pick struct {
		score    int
		offset   uint64
		format   uint16
		isSymbol bool // true iff (3, 0) format-4 — Microsoft Symbol fallback
	}
	var best pick

	for i := uint64(0); i < numTables; i++ {
		recOff := 4 + i*8
		platformID := binary.BigEndian.Uint16(data[recOff : recOff+2])
		encodingID := binary.BigEndian.Uint16(data[recOff+2 : recOff+4])
		subOff := uint64(binary.BigEndian.Uint32(data[recOff+4 : recOff+8]))
		if subOff+2 > dataLen {
			continue
		}
		format := binary.BigEndian.Uint16(data[subOff : subOff+2])
		score := scoreSubtable(platformID, encodingID, format)
		if score > best.score {
			best = pick{
				score:    score,
				offset:   subOff,
				format:   format,
				isSymbol: platformID == 3 && encodingID == 0 && format == 4,
			}
		}
	}
	if best.score == 0 {
		return nil, fmt.Errorf("font: cmap: no supported Unicode subtable: %w", ErrCorruptTable)
	}
	var (
		out cmapTable
		err error
	)
	switch best.format {
	case 4:
		out, err = parseCmapFormat4(data, best.offset)
	case 12:
		out, err = parseCmapFormat12(data, best.offset)
	default:
		return nil, fmt.Errorf("font: cmap: subtable format %d not implemented: %w", best.format, ErrCorruptTable)
	}
	if err != nil {
		return nil, err
	}
	if best.isSymbol {
		mirrorSymbolToASCII(out)
	}
	return out, nil
}

// mirrorSymbolToASCII augments a Symbol-cmap mapping (Microsoft
// platform, encoding 0) with ASCII aliases for each PUA entry in the
// 0xF020..0xF0FF range. After the augmentation, both the canonical
// PUA codepoint AND the corresponding ASCII codepoint resolve to the
// same glyph ID — so a caller using the natural ASCII codepoint
// `'A'` (U+0041) gets the glyph the font intended at `0xF041`.
//
// Without this step, parseCmapTable could load Wingdings successfully
// but a paragraph containing `<p>A</p>` would render as .notdef
// because the html-shaping path uses U+0041, not U+F041. HarfBuzz
// applies the same alias automatically; without it Symbol fonts work
// in HarfBuzz but appear blank in any consumer that doesn't know to
// pre-translate.
//
// The mirror is one-directional and additive: existing PUA entries
// are preserved, and ASCII-range entries that already exist in the
// source cmap (rare for Symbol fonts but legal) are NOT overwritten.
func mirrorSymbolToASCII(t cmapTable) {
	for r, gid := range t {
		if r < 0xF020 || r > 0xF0FF {
			continue
		}
		ascii := r - 0xF000
		if _, exists := t[ascii]; exists {
			continue
		}
		t[ascii] = gid
	}
}

// scoreSubtable returns a priority score for a (platform, encoding,
// format) triple. Higher wins; 0 means we won't use it. The ordering
// reflects coverage breadth: format 12 (full Unicode) outranks
// format 4 (BMP only), and within each format the Unicode-platform
// records outrank the Windows-platform fallbacks.
func scoreSubtable(platformID, encodingID, format uint16) int {
	switch format {
	case 12:
		switch {
		case platformID == 0 && encodingID == 4:
			return 100 // Unicode 2.0+ full repertoire
		case platformID == 3 && encodingID == 10:
			return 90 // Windows UCS-4
		case platformID == 0:
			return 80 // any Unicode-platform format-12
		}
	case 4:
		switch {
		case platformID == 0 && encodingID == 3:
			return 50 // Unicode 2.0 BMP
		case platformID == 3 && encodingID == 1:
			return 40 // Windows Unicode BMP
		case platformID == 0:
			return 30
		case platformID == 3 && encodingID == 0:
			// Microsoft Symbol — used by Wingdings, Symbol, dingbat
			// fonts that do not ship a Unicode subtable. The mapping
			// uses PUA-style codepoints (0xF020..0xF0FF) per the
			// OpenType spec; callers that target these fonts know to
			// use the PUA codepoints. HarfBuzz, go-text, and pdf.js
			// all accept this fallback. Without it Folio rejects the
			// entire font with "no supported Unicode subtable".
			return 20
		}
	}
	return 0
}

// parseCmapFormat4 decodes a format-4 subtable starting at offset
// subOff within data.
//
// Spec: ISO/IEC 14496-22 §5.2.1.3.4. Format 4 maps BMP runes only and
// is the most commonly seen subtable in Latin/European fonts.
//
// Layout (offsets relative to subtable start):
//   - format (uint16)              at 0  — 4
//   - length (uint16)              at 2
//   - language (uint16)            at 4
//   - segCountX2 (uint16)          at 6
//   - searchRange/entrySelector/rangeShift (uint16 each) at 8..14
//   - endCode[segCount] (uint16)   at 14
//   - reservedPad (uint16)
//   - startCode[segCount] (uint16)
//   - idDelta[segCount] (int16)
//   - idRangeOffset[segCount] (uint16)
//   - glyphIdArray[] (uint16, variable length)
//
// Per the spec, the final segment is always (startCode=0xFFFF,
// endCode=0xFFFF, idDelta=1) — we still decode it so the caller can
// tell genuinely-mapped 0xFFFF runes from the sentinel; sfnt did the
// same.
func parseCmapFormat4(data []byte, subOff uint64) (cmapTable, error) {
	dataLen := uint64(len(data))
	if subOff+14 > dataLen {
		return nil, fmt.Errorf("font: cmap fmt4: header truncated: %w", ErrTruncated)
	}
	length := uint64(binary.BigEndian.Uint16(data[subOff+2 : subOff+4]))
	if subOff+length > dataLen {
		return nil, fmt.Errorf("font: cmap fmt4: declared length %d exceeds table: %w", length, ErrTruncated)
	}
	segCountX2 := uint64(binary.BigEndian.Uint16(data[subOff+6 : subOff+8]))
	if segCountX2%2 != 0 || segCountX2 == 0 {
		return nil, fmt.Errorf("font: cmap fmt4: invalid segCountX2 %d: %w", segCountX2, ErrCorruptTable)
	}
	segCount := segCountX2 / 2

	endOff := subOff + 14
	startOff := endOff + segCountX2 + 2 // +2 for reservedPad
	deltaOff := startOff + segCountX2
	rangeOff := deltaOff + segCountX2
	glyphArrOff := rangeOff + segCountX2

	if glyphArrOff > subOff+length {
		return nil, fmt.Errorf("font: cmap fmt4: arrays exceed declared length: %w", ErrTruncated)
	}

	out := make(cmapTable)
	var totalCodes uint64
	for i := uint64(0); i < segCount; i++ {
		end := binary.BigEndian.Uint16(data[endOff+i*2 : endOff+i*2+2])
		start := binary.BigEndian.Uint16(data[startOff+i*2 : startOff+i*2+2])
		delta := int16(binary.BigEndian.Uint16(data[deltaOff+i*2 : deltaOff+i*2+2]))
		idRangeOffset := binary.BigEndian.Uint16(data[rangeOff+i*2 : rangeOff+i*2+2])
		if start > end {
			continue
		}
		totalCodes += uint64(end) - uint64(start) + 1
		if totalCodes > cmapMaxMappedCodes {
			return nil, fmt.Errorf("font: cmap fmt4: total mapped codes exceed %d: %w", cmapMaxMappedCodes, ErrCorruptTable)
		}
		// The terminating sentinel segment (0xFFFF, 0xFFFF, idDelta=1)
		// processes naturally: the resulting glyph ID is 0 and the
		// gid==0 filter at the end of the loop drops it without
		// pinning U+FFFF to anything. Mirrors the approach taken by
		// HarfBuzz, FreeType, fontTools, and pdf.js — and avoids
		// masking the rare-but-legal case of a font that maps U+FFFF
		// to a real glyph (which an explicit sentinel-skip would have
		// silently dropped).
		for c := uint32(start); c <= uint32(end); c++ {
			var gid uint16
			if idRangeOffset == 0 {
				gid = uint16(int32(c) + int32(delta))
			} else {
				// idRangeOffset is a byte offset from the location of
				// itself into glyphIdArray. Resolved via the spec's
				// pointer arithmetic:
				//   &glyphIdArray[idRangeOffset/2 + (c - start) - (segCount - i)]
				// which equals (in raw byte offsets):
				//   rangeOff + i*2 + idRangeOffset + 2*(c - start).
				// uint64 arithmetic prevents overflow on hostile input.
				readOff := rangeOff + i*2 + uint64(idRangeOffset) + 2*uint64(c-uint32(start))
				if readOff+2 > subOff+length {
					continue
				}
				raw := binary.BigEndian.Uint16(data[readOff : readOff+2])
				if raw == 0 {
					continue
				}
				gid = uint16(int32(raw) + int32(delta))
			}
			if gid == 0 {
				continue
			}
			r := rune(c)
			if _, exists := out[r]; !exists {
				out[r] = gid
			}
		}
	}
	return out, nil
}

// parseCmapFormat12 decodes a format-12 subtable starting at offset
// subOff within data.
//
// Spec: ISO/IEC 14496-22 §5.2.1.3.7. Format 12 covers the full
// Unicode range (0..0x10FFFF) via sequential groups, the canonical
// shape for CJK fonts.
//
// Layout (offsets relative to subtable start):
//   - format (uint16)         at 0  — 12
//   - reserved (uint16)       at 2
//   - length (uint32)         at 4
//   - language (uint32)       at 8
//   - numGroups (uint32)      at 12
//   - groups[numGroups]:      at 16
//     startCharCode (uint32)
//     endCharCode (uint32)
//     startGlyphID (uint32)
//
// Each group contributes (endCharCode - startCharCode + 1) entries
// to the resolved table. CJK fonts in the wild produce 25k+ groups;
// the parser bounds-checks every group against the declared length
// rather than assuming a hard cap.
func parseCmapFormat12(data []byte, subOff uint64) (cmapTable, error) {
	dataLen := uint64(len(data))
	if subOff+16 > dataLen {
		return nil, fmt.Errorf("font: cmap fmt12: header truncated: %w", ErrTruncated)
	}
	length := uint64(binary.BigEndian.Uint32(data[subOff+4 : subOff+8]))
	if length < 16 {
		return nil, fmt.Errorf("font: cmap fmt12: declared length %d < header size: %w", length, ErrCorruptTable)
	}
	if subOff+length > dataLen {
		return nil, fmt.Errorf("font: cmap fmt12: declared length %d exceeds table: %w", length, ErrTruncated)
	}
	numGroups := uint64(binary.BigEndian.Uint32(data[subOff+12 : subOff+16]))
	groupsEnd := subOff + 16 + numGroups*12
	if groupsEnd > subOff+length {
		return nil, fmt.Errorf("font: cmap fmt12: groups (%d) exceed declared length: %w", numGroups, ErrTruncated)
	}

	out := make(cmapTable)
	var totalCodes uint64
	for i := uint64(0); i < numGroups; i++ {
		off := subOff + 16 + i*12
		startChar := binary.BigEndian.Uint32(data[off : off+4])
		endChar := binary.BigEndian.Uint32(data[off+4 : off+8])
		startGID := binary.BigEndian.Uint32(data[off+8 : off+12])
		if startChar > endChar {
			continue
		}
		// Cap the rune range to Unicode's maximum scalar value; some
		// hostile fonts declare endChar past 0x10FFFF.
		if endChar > 0x10FFFF {
			endChar = 0x10FFFF
		}
		totalCodes += uint64(endChar) - uint64(startChar) + 1
		if totalCodes > cmapMaxMappedCodes {
			return nil, fmt.Errorf("font: cmap fmt12: total mapped codes exceed %d: %w", cmapMaxMappedCodes, ErrCorruptTable)
		}
		for c := startChar; c <= endChar; c++ {
			gid32 := startGID + (c - startChar)
			if gid32 > 0xFFFF {
				continue // outside uint16 GID range
			}
			gid := uint16(gid32)
			if gid == 0 {
				continue
			}
			r := rune(c)
			if _, exists := out[r]; !exists {
				out[r] = gid
			}
		}
	}
	return out, nil
}
