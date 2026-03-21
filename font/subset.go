// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// Subset produces a minimal TrueType font containing only the glyphs
// referenced in usedGlyphs. Original glyph IDs are preserved (Option A):
// unused glyph slots are zeroed out so /CIDToGIDMap /Identity remains valid.
//
// The subset includes tables: head, hhea, maxp, OS/2, name, cmap, post,
// loca, glyf, hmtx. All other tables are omitted.
func Subset(raw []byte, usedGlyphs map[uint16]rune) ([]byte, error) {
	tables, err := parseTTFTables(raw)
	if err != nil {
		return nil, err
	}

	// Always include GID 0 (.notdef).
	glyphSet := make(map[uint16]bool)
	glyphSet[0] = true
	for gid := range usedGlyphs {
		glyphSet[gid] = true
	}

	// Read numGlyphs from maxp.
	maxpData, ok := tables["maxp"]
	if !ok {
		return nil, fmt.Errorf("subset: missing maxp table")
	}
	if len(maxpData) < 6 {
		return nil, fmt.Errorf("subset: maxp table too short")
	}
	numGlyphs := int(binary.BigEndian.Uint16(maxpData[4:6]))

	// Read head to get loca format.
	headData, ok := tables["head"]
	if !ok {
		return nil, fmt.Errorf("subset: missing head table")
	}
	if len(headData) < 54 {
		return nil, fmt.Errorf("subset: head table too short")
	}
	locaFormat := int16(binary.BigEndian.Uint16(headData[50:52]))

	// Read loca table.
	locaData, ok := tables["loca"]
	if !ok {
		return nil, fmt.Errorf("subset: missing loca table")
	}

	// Read glyf table.
	glyfData, ok := tables["glyf"]
	if !ok {
		return nil, fmt.Errorf("subset: missing glyf table")
	}

	// Parse loca to get glyph offsets.
	offsets, err := parseLoca(locaData, locaFormat, numGlyphs)
	if err != nil {
		return nil, err
	}

	// Resolve composite glyphs — add any component GIDs.
	resolveComposites(glyfData, offsets, glyphSet, numGlyphs)

	// Rebuild glyf and loca (zero unused slots).
	newGlyf, newLoca := rebuildGlyfLoca(glyfData, offsets, glyphSet, numGlyphs)

	// Rebuild hmtx (zero unused entries).
	hheaData, ok := tables["hhea"]
	if !ok {
		return nil, fmt.Errorf("subset: missing hhea table")
	}
	hmtxData, ok := tables["hmtx"]
	if !ok {
		return nil, fmt.Errorf("subset: missing hmtx table")
	}
	numHMetrics := int(binary.BigEndian.Uint16(hheaData[34:36]))
	newHmtx := rebuildHmtx(hmtxData, glyphSet, numGlyphs, numHMetrics)

	// Rebuild cmap with only used mappings.
	newCmap := buildSubsetCmap(usedGlyphs)

	// Minimal post table (format 3.0 — no glyph names).
	newPost := buildMinimalPost()

	// Update head: use long loca format (we always write uint32 offsets).
	newHead := make([]byte, len(headData))
	copy(newHead, headData)
	binary.BigEndian.PutUint16(newHead[50:52], 1) // indexToLocFormat = 1 (long)
	// Zero out checkSumAdjustment for now; will fix after assembly.
	binary.BigEndian.PutUint32(newHead[8:12], 0)

	// Assemble the subset font.
	outTables := map[string][]byte{
		"head": newHead,
		"hhea": copyTable(hheaData),
		"maxp": copyTable(maxpData),
		"cmap": newCmap,
		"glyf": newGlyf,
		"loca": newLoca,
		"hmtx": newHmtx,
		"post": newPost,
	}
	// Include OS/2 and name if present (copy as-is).
	if d, ok := tables["OS/2"]; ok {
		outTables["OS/2"] = copyTable(d)
	}
	if d, ok := tables["name"]; ok {
		outTables["name"] = copyTable(d)
	}

	result := assembleTTF(outTables)

	// Fix head.checkSumAdjustment.
	fixHeadChecksum(result, outTables)

	return result, nil
}

// --- TTF parsing ---

// parseTTFTables extracts table data from a raw TTF file.
func parseTTFTables(raw []byte) (map[string][]byte, error) {
	if len(raw) < 12 {
		return nil, fmt.Errorf("subset: file too short for TTF header")
	}

	numTables := int(binary.BigEndian.Uint16(raw[4:6]))
	if len(raw) < 12+numTables*16 {
		return nil, fmt.Errorf("subset: file too short for table directory")
	}

	tables := make(map[string][]byte, numTables)
	for i := range numTables {
		offset := 12 + i*16
		tag := string(raw[offset : offset+4])
		tblOffset := int(binary.BigEndian.Uint32(raw[offset+8 : offset+12]))
		tblLength := int(binary.BigEndian.Uint32(raw[offset+12 : offset+16]))
		if tblOffset+tblLength > len(raw) {
			return nil, fmt.Errorf("subset: table %q extends beyond file", tag)
		}
		tables[tag] = raw[tblOffset : tblOffset+tblLength]
	}

	return tables, nil
}

// parseLoca reads the loca table and returns glyph offsets (numGlyphs+1 entries).
func parseLoca(data []byte, format int16, numGlyphs int) ([]uint32, error) {
	offsets := make([]uint32, numGlyphs+1)
	if format == 0 {
		// Short format: uint16, multiply by 2.
		need := (numGlyphs + 1) * 2
		if len(data) < need {
			return nil, fmt.Errorf("subset: loca table too short (short format)")
		}
		for i := range numGlyphs + 1 {
			offsets[i] = uint32(binary.BigEndian.Uint16(data[i*2:])) * 2
		}
	} else {
		// Long format: uint32.
		need := (numGlyphs + 1) * 4
		if len(data) < need {
			return nil, fmt.Errorf("subset: loca table too short (long format)")
		}
		for i := range numGlyphs + 1 {
			offsets[i] = binary.BigEndian.Uint32(data[i*4:])
		}
	}
	return offsets, nil
}

// --- Composite glyph resolution ---

// resolveComposites scans included glyphs for composite references
// and adds their component GIDs to the set. Repeats until stable.
func resolveComposites(glyfData []byte, offsets []uint32, glyphSet map[uint16]bool, numGlyphs int) {
	for {
		added := false
		for gid := range glyphSet {
			if int(gid) >= numGlyphs {
				continue
			}
			start := offsets[gid]
			end := offsets[gid+1]
			if start >= end || int(end) > len(glyfData) {
				continue
			}
			data := glyfData[start:end]
			if len(data) < 2 {
				continue
			}
			numContours := int16(binary.BigEndian.Uint16(data[0:2]))
			if numContours >= 0 {
				continue // simple glyph
			}
			// Composite glyph — parse components.
			components := parseCompositeComponents(data)
			for _, cid := range components {
				if !glyphSet[cid] {
					glyphSet[cid] = true
					added = true
				}
			}
		}
		if !added {
			break
		}
	}
}

// parseCompositeComponents extracts referenced glyph IDs from a composite glyph.
func parseCompositeComponents(data []byte) []uint16 {
	if len(data) < 12 {
		return nil
	}
	// Skip header: numberOfContours(2) + xMin(2) + yMin(2) + xMax(2) + yMax(2) = 10 bytes
	pos := 10
	var components []uint16

	for pos+4 <= len(data) {
		flags := binary.BigEndian.Uint16(data[pos:])
		glyphIdx := binary.BigEndian.Uint16(data[pos+2:])
		components = append(components, glyphIdx)
		pos += 4

		// Skip transform data based on flags, checking bounds at each step.
		if flags&0x0001 != 0 { // ARG_1_AND_2_ARE_WORDS
			pos += 4
		} else {
			pos += 2
		}
		if pos > len(data) {
			break
		}
		if flags&0x0008 != 0 { // WE_HAVE_A_SCALE
			pos += 2
		} else if flags&0x0040 != 0 { // WE_HAVE_AN_X_AND_Y_SCALE
			pos += 4
		} else if flags&0x0080 != 0 { // WE_HAVE_A_TWO_BY_TWO
			pos += 8
		}
		if pos > len(data) {
			break
		}

		if flags&0x0020 == 0 { // MORE_COMPONENTS
			break
		}
	}

	return components
}

// --- Rebuild tables ---

// rebuildGlyfLoca creates new glyf and loca tables.
// Used glyphs keep their data; unused glyphs get zero-length entries.
// Always outputs long loca format (uint32).
func rebuildGlyfLoca(oldGlyf []byte, offsets []uint32, glyphSet map[uint16]bool, numGlyphs int) ([]byte, []byte) {
	var newGlyf []byte
	newLoca := make([]byte, (numGlyphs+1)*4)

	runningOffset := uint32(0)
	for gid := range numGlyphs {
		binary.BigEndian.PutUint32(newLoca[gid*4:], runningOffset)

		start := offsets[gid]
		end := offsets[gid+1]
		if glyphSet[uint16(gid)] && start < end && int(end) <= len(oldGlyf) {
			glyphData := oldGlyf[start:end]
			newGlyf = append(newGlyf, glyphData...)
			// Pad to even boundary (glyf entries should be word-aligned for loca).
			if len(glyphData)%2 != 0 {
				newGlyf = append(newGlyf, 0)
			}
			runningOffset = uint32(len(newGlyf))
		}
		// If not used, runningOffset stays the same → zero-length entry.
	}
	// Final loca entry.
	binary.BigEndian.PutUint32(newLoca[numGlyphs*4:], runningOffset)

	return newGlyf, newLoca
}

// rebuildHmtx zeroes out advance widths for unused glyphs.
func rebuildHmtx(oldHmtx []byte, glyphSet map[uint16]bool, numGlyphs, numHMetrics int) []byte {
	// hmtx has numHMetrics long entries (4 bytes: advanceWidth + lsb)
	// followed by (numGlyphs - numHMetrics) short entries (2 bytes: lsb only).
	newHmtx := make([]byte, len(oldHmtx))
	copy(newHmtx, oldHmtx)

	for gid := range numGlyphs {
		if glyphSet[uint16(gid)] {
			continue
		}
		if gid < numHMetrics {
			// Zero out the 4-byte long entry.
			off := gid * 4
			if off+4 <= len(newHmtx) {
				binary.BigEndian.PutUint16(newHmtx[off:], 0)   // advanceWidth
				binary.BigEndian.PutUint16(newHmtx[off+2:], 0) // lsb
			}
		} else {
			// Zero out the 2-byte short entry.
			off := numHMetrics*4 + (gid-numHMetrics)*2
			if off+2 <= len(newHmtx) {
				binary.BigEndian.PutUint16(newHmtx[off:], 0)
			}
		}
	}

	return newHmtx
}

// buildSubsetCmap builds a minimal cmap table with a format 4 subtable
// containing only the used codepoint→GID mappings.
func buildSubsetCmap(usedGlyphs map[uint16]rune) []byte {
	// Collect mappings and sort by codepoint.
	type cpGID struct {
		cp  uint16
		gid uint16
	}
	var mappings []cpGID
	for gid, r := range usedGlyphs {
		if gid == 0 || r > 0xFFFF {
			continue // skip .notdef and non-BMP
		}
		mappings = append(mappings, cpGID{cp: uint16(r), gid: gid})
	}
	sort.Slice(mappings, func(i, j int) bool { return mappings[i].cp < mappings[j].cp })

	// Build segments for format 4.
	type segment struct {
		startCode uint16
		endCode   uint16
		idDelta   int16
	}

	var segs []segment
	for _, m := range mappings {
		delta := int16(m.gid) - int16(m.cp)
		if len(segs) > 0 {
			last := &segs[len(segs)-1]
			if m.cp == last.endCode+1 && delta == last.idDelta {
				last.endCode = m.cp
				continue
			}
		}
		segs = append(segs, segment{startCode: m.cp, endCode: m.cp, idDelta: delta})
	}
	// Add the required terminating segment.
	segs = append(segs, segment{startCode: 0xFFFF, endCode: 0xFFFF, idDelta: 1})

	segCount := len(segs)
	searchRange := 1
	entrySelector := 0
	for searchRange*2 <= segCount {
		searchRange *= 2
		entrySelector++
	}
	searchRange *= 2
	rangeShift := segCount*2 - searchRange

	// Format 4 subtable size:
	// 14 bytes header + 4 arrays of segCount*2 bytes + 2 bytes reservedPad
	subtableLen := 14 + segCount*2*4 + 2

	// cmap header: version(2) + numTables(2) + platformID(2) + encodingID(2) + offset(4) = 12
	cmapHeaderLen := 12
	totalLen := cmapHeaderLen + subtableLen

	buf := make([]byte, totalLen)
	// cmap header
	binary.BigEndian.PutUint16(buf[0:], 0)  // version
	binary.BigEndian.PutUint16(buf[2:], 1)  // numTables
	binary.BigEndian.PutUint16(buf[4:], 3)  // platformID = Windows
	binary.BigEndian.PutUint16(buf[6:], 1)  // encodingID = Unicode BMP
	binary.BigEndian.PutUint32(buf[8:], 12) // offset to subtable

	// Format 4 subtable
	off := 12
	binary.BigEndian.PutUint16(buf[off:], 4)                     // format
	binary.BigEndian.PutUint16(buf[off+2:], uint16(subtableLen)) // length
	binary.BigEndian.PutUint16(buf[off+4:], 0)                   // language
	binary.BigEndian.PutUint16(buf[off+6:], uint16(segCount*2))  // segCountX2
	binary.BigEndian.PutUint16(buf[off+8:], uint16(searchRange))
	binary.BigEndian.PutUint16(buf[off+10:], uint16(entrySelector))
	binary.BigEndian.PutUint16(buf[off+12:], uint16(rangeShift))
	off += 14

	// endCode array
	for _, s := range segs {
		binary.BigEndian.PutUint16(buf[off:], s.endCode)
		off += 2
	}
	// reservedPad
	binary.BigEndian.PutUint16(buf[off:], 0)
	off += 2
	// startCode array
	for _, s := range segs {
		binary.BigEndian.PutUint16(buf[off:], s.startCode)
		off += 2
	}
	// idDelta array
	for _, s := range segs {
		binary.BigEndian.PutUint16(buf[off:], uint16(s.idDelta))
		off += 2
	}
	// idRangeOffset array (all zeros — we use idDelta)
	for range segs {
		binary.BigEndian.PutUint16(buf[off:], 0)
		off += 2
	}

	return buf
}

// buildMinimalPost creates a format 3.0 post table (no glyph names).
func buildMinimalPost() []byte {
	post := make([]byte, 32)
	// Format 3.0 (fixed-point: 0x00030000)
	binary.BigEndian.PutUint32(post[0:], 0x00030000)
	// italicAngle, underlinePosition, underlineThickness, isFixedPitch — all zero
	return post
}

// --- TTF assembly ---

// assembleTTF writes a complete TTF file from the given tables.
func assembleTTF(tables map[string][]byte) []byte {
	// Sort table tags alphabetically.
	tags := make([]string, 0, len(tables))
	for tag := range tables {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	numTables := len(tags)
	searchRange := 1
	entrySelector := 0
	for searchRange*2 <= numTables {
		searchRange *= 2
		entrySelector++
	}
	searchRange *= 16
	rangeShift := numTables*16 - searchRange

	// Calculate data offset (after header + directory).
	headerSize := 12 + numTables*16
	dataOffset := headerSize
	// Pad to 4-byte boundary.
	if dataOffset%4 != 0 {
		dataOffset += 4 - dataOffset%4
	}

	// Calculate table offsets.
	type tableInfo struct {
		tag    string
		offset int
		length int
	}
	var infos []tableInfo
	curOffset := dataOffset
	for _, tag := range tags {
		data := tables[tag]
		infos = append(infos, tableInfo{tag: tag, offset: curOffset, length: len(data)})
		curOffset += len(data)
		// Pad to 4-byte boundary.
		if curOffset%4 != 0 {
			curOffset += 4 - curOffset%4
		}
	}

	totalSize := curOffset
	buf := make([]byte, totalSize)

	// Write offset table header.
	binary.BigEndian.PutUint32(buf[0:], 0x00010000) // sfVersion (TrueType)
	binary.BigEndian.PutUint16(buf[4:], uint16(numTables))
	binary.BigEndian.PutUint16(buf[6:], uint16(searchRange))
	binary.BigEndian.PutUint16(buf[8:], uint16(entrySelector))
	binary.BigEndian.PutUint16(buf[10:], uint16(rangeShift))

	// Write table directory.
	for i, info := range infos {
		dirOff := 12 + i*16
		copy(buf[dirOff:], info.tag)
		data := tables[info.tag]
		cs := tableChecksum(data)
		binary.BigEndian.PutUint32(buf[dirOff+4:], cs)
		binary.BigEndian.PutUint32(buf[dirOff+8:], uint32(info.offset))
		binary.BigEndian.PutUint32(buf[dirOff+12:], uint32(info.length))
	}

	// Write table data.
	for _, info := range infos {
		copy(buf[info.offset:], tables[info.tag])
	}

	return buf
}

// tableChecksum computes the checksum of a table.
func tableChecksum(data []byte) uint32 {
	// Pad to 4 bytes.
	padded := data
	if len(padded)%4 != 0 {
		padded = make([]byte, len(data)+4-len(data)%4)
		copy(padded, data)
	}
	var sum uint32
	for i := 0; i < len(padded); i += 4 {
		sum += binary.BigEndian.Uint32(padded[i:])
	}
	return sum
}

// fixHeadChecksum sets head.checkSumAdjustment so the entire file checksums to 0xB1B0AFBA.
func fixHeadChecksum(data []byte, tables map[string][]byte) {
	// Find head table offset in the file.
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	for i := range numTables {
		dirOff := 12 + i*16
		tag := string(data[dirOff : dirOff+4])
		if tag == "head" {
			tblOffset := int(binary.BigEndian.Uint32(data[dirOff+8 : dirOff+12]))
			// Compute whole-file checksum.
			wholeSum := tableChecksum(data)
			adj := uint32(0xB1B0AFBA) - wholeSum
			binary.BigEndian.PutUint32(data[tblOffset+8:tblOffset+12], adj)

			// Also update the directory checksum for head.
			headData := tables["head"]
			// Recompute since we modified the data in-place.
			binary.BigEndian.PutUint32(headData[8:12], adj)
			cs := tableChecksum(headData)
			binary.BigEndian.PutUint32(data[dirOff+4:dirOff+8], cs)
			return
		}
	}
}

// copyTable creates a copy of table data.
func copyTable(data []byte) []byte {
	c := make([]byte, len(data))
	copy(c, data)
	return c
}
