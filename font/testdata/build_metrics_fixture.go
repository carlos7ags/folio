// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

//go:build ignore

// Build script for font/testdata/synthetic_metrics_hhea.ttf and
// font/testdata/synthetic_metrics_typo.ttf — two tiny, purpose-built
// TrueType fonts whose vertical metrics are chosen so tests can assert
// exact CSS `line-height: normal` values and which metric table
// (hhea vs OS/2 typo) Folio consults. They replace third-party font
// binaries, so the fixtures carry no license obligations.
//
// Run from the repository root:
//
//	go run ./font/testdata/build_metrics_fixture.go
//
// Both fonts share (upem 1000):
//
//   - hhea:  ascender 1050, descender -350, lineGap 100  (sum 1500 → 1.5em)
//   - OS/2:  sTypoAscender 900, sTypoDescender -250, sTypoLineGap 150
//     (sum 1300 → 1.3em); usWinAscent 1200, usWinDescent 400 (never
//     used for line-height; distinct so a wrong-table read is caught)
//
// and differ only in the OS/2 fsSelection USE_TYPO_METRICS bit (0x80):
// clear in synthetic_metrics_hhea.ttf (hhea wins → normal = 1.5em), set in
// synthetic_metrics_typo.ttf (typo wins → normal = 1.3em).
//
// The cmap covers printable ASCII U+0020..U+007E. Glyph advances follow
// [advanceFor] — space is 250 units, every other character is
// 500 + 10*(r-0x20) — so tests can derive exact expected widths without
// reading the font. Glyphs are empty (numberOfContours == 0); the fixtures
// exist for metrics, not rendering.
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
)

const (
	unitsPerEm = 1000
	firstChar  = 0x20
	lastChar   = 0x7E

	hheaAscender  = 1050
	hheaDescender = -350
	hheaLineGap   = 100

	typoAscender  = 900
	typoDescender = -250
	typoLineGap   = 150

	winAscent  = 1200
	winDescent = 400

	maxAdvance = 500 + 10*(lastChar-firstChar)
)

// advanceFor is the advance width (font units) of the glyph for r. Keep in
// sync with the tests that assert widths against these fixtures.
func advanceFor(r rune) int {
	if r == ' ' {
		return 250
	}
	return 500 + 10*int(r-firstChar)
}

func main() {
	for _, f := range []struct {
		out     string
		family  string
		psName  string
		useTypo bool
	}{
		{"font/testdata/synthetic_metrics_hhea.ttf", "Synthetic Metrics Hhea", "SyntheticMetricsHhea", false},
		{"font/testdata/synthetic_metrics_typo.ttf", "Synthetic Metrics Typo", "SyntheticMetricsTypo", true},
	} {
		numGlyphs := lastChar - firstChar + 2 // +1 for .notdef
		tables := map[string][]byte{
			"head": buildHead(),
			"hhea": buildHhea(numGlyphs),
			"maxp": buildMaxp(numGlyphs),
			"OS/2": buildOS2(f.useTypo),
			"cmap": buildCmap(),
			"name": buildName(f.family, f.psName),
			"hmtx": buildHmtx(),
			"glyf": buildGlyf(numGlyphs),
			"loca": buildLoca(numGlyphs),
			"post": buildPost(),
		}
		ttf := assembleTTF(tables)
		if err := os.WriteFile(f.out, ttf, 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s (%d bytes, %d glyphs)\n", f.out, len(ttf), numGlyphs)
	}
}

func buildHead() []byte {
	buf := new(bytes.Buffer)
	write32(buf, 0x00010000)         // version 1.0
	write32(buf, 0x00010000)         // fontRevision 1.0
	write32(buf, 0)                  // checkSumAdjustment (ignored)
	write32(buf, 0x5F0F3CF5)         // magicNumber
	write16(buf, 0)                  // flags
	write16(buf, unitsPerEm)         // unitsPerEm
	write64(buf, 0)                  // created
	write64(buf, 0)                  // modified
	writeI16(buf, 0)                 // xMin
	writeI16(buf, hheaDescender)     // yMin
	writeI16(buf, int16(maxAdvance)) // xMax
	writeI16(buf, hheaAscender)      // yMax
	write16(buf, 0)                  // macStyle
	write16(buf, 8)                  // lowestRecPPEM
	writeI16(buf, 2)                 // fontDirectionHint
	writeI16(buf, 1)                 // indexToLocFormat (1 = long offsets)
	writeI16(buf, 0)                 // glyphDataFormat
	return buf.Bytes()
}

func buildHhea(numGlyphs int) []byte {
	buf := new(bytes.Buffer)
	write32(buf, 0x00010000)         // version 1.0
	writeI16(buf, hheaAscender)      // ascender
	writeI16(buf, hheaDescender)     // descender
	writeI16(buf, hheaLineGap)       // lineGap
	write16(buf, uint16(maxAdvance)) // advanceWidthMax
	writeI16(buf, 0)                 // minLeftSideBearing
	writeI16(buf, 0)                 // minRightSideBearing
	writeI16(buf, int16(maxAdvance)) // xMaxExtent
	writeI16(buf, 1)                 // caretSlopeRise
	writeI16(buf, 0)                 // caretSlopeRun
	writeI16(buf, 0)                 // caretOffset
	for range 4 {
		writeI16(buf, 0) // reserved
	}
	writeI16(buf, 0)                // metricDataFormat
	write16(buf, uint16(numGlyphs)) // numberOfHMetrics
	return buf.Bytes()
}

func buildMaxp(numGlyphs int) []byte {
	buf := new(bytes.Buffer)
	write32(buf, 0x00010000) // version 1.0
	write16(buf, uint16(numGlyphs))
	write16(buf, 0) // maxPoints
	write16(buf, 0) // maxContours
	write16(buf, 0) // maxCompositePoints
	write16(buf, 0) // maxCompositeContours
	write16(buf, 2) // maxZones
	for range 8 {
		write16(buf, 0) // maxTwilightPoints..maxComponentDepth
	}
	return buf.Bytes()
}

// buildOS2 emits a 96-byte version-4 OS/2 table. fsSelection carries
// REGULAR (0x40) plus USE_TYPO_METRICS (0x80) when useTypo is set.
func buildOS2(useTypo bool) []byte {
	buf := new(bytes.Buffer)
	write16(buf, 4)    // version
	writeI16(buf, 600) // xAvgCharWidth
	write16(buf, 400)  // usWeightClass
	write16(buf, 5)    // usWidthClass (medium)
	write16(buf, 0)    // fsType (installable embedding)
	for range 10 {
		writeI16(buf, 0) // ySubscriptXSize..yStrikeoutPosition
	}
	writeI16(buf, 0)            // sFamilyClass
	buf.Write(make([]byte, 10)) // panose
	for range 4 {
		write32(buf, 0) // ulUnicodeRange1..4
	}
	buf.WriteString("NONE") // achVendID
	fsSelection := uint16(0x40)
	if useTypo {
		fsSelection |= 0x80
	}
	write16(buf, fsSelection)
	write16(buf, firstChar)      // usFirstCharIndex
	write16(buf, lastChar)       // usLastCharIndex
	writeI16(buf, typoAscender)  // sTypoAscender
	writeI16(buf, typoDescender) // sTypoDescender
	writeI16(buf, typoLineGap)   // sTypoLineGap
	write16(buf, winAscent)      // usWinAscent
	write16(buf, winDescent)     // usWinDescent
	write32(buf, 1)              // ulCodePageRange1 (Latin 1)
	write32(buf, 0)              // ulCodePageRange2
	writeI16(buf, 500)           // sxHeight
	writeI16(buf, 700)           // sCapHeight
	write16(buf, 0)              // usDefaultChar
	write16(buf, ' ')            // usBreakChar
	write16(buf, 0)              // usMaxContext
	return buf.Bytes()
}

// buildCmap emits a format-4 cmap with one contiguous segment mapping
// U+0020..U+007E to GIDs 1..95, plus the mandatory 0xFFFF terminator.
func buildCmap() []byte {
	type seg struct {
		end, start uint16
		delta      int16
	}
	segs := []seg{
		{end: lastChar, start: firstChar, delta: int16(1 - firstChar)},
		{end: 0xFFFF, start: 0xFFFF, delta: 1},
	}
	segCount := uint16(len(segs))
	segCountX2 := segCount * 2
	searchRange := uint16(1)
	for searchRange*2 <= segCount {
		searchRange *= 2
	}
	searchRange *= 2
	entrySelector := uint16(0)
	for (1 << entrySelector) < searchRange/2 {
		entrySelector++
	}
	rangeShift := segCountX2 - searchRange

	st := new(bytes.Buffer)
	write16(st, 4) // format
	write16(st, 0) // length (patched below)
	write16(st, 0) // language
	write16(st, segCountX2)
	write16(st, searchRange)
	write16(st, entrySelector)
	write16(st, rangeShift)
	for _, s := range segs {
		write16(st, s.end)
	}
	write16(st, 0) // reservedPad
	for _, s := range segs {
		write16(st, s.start)
	}
	for _, s := range segs {
		writeI16(st, s.delta)
	}
	for range segs {
		write16(st, 0) // idRangeOffset
	}
	subtable := st.Bytes()
	binary.BigEndian.PutUint16(subtable[2:4], uint16(len(subtable)))

	buf := new(bytes.Buffer)
	write16(buf, 0) // version
	write16(buf, 1) // numTables
	write16(buf, 3) // platform Windows
	write16(buf, 1) // encoding Unicode BMP
	write32(buf, 4+8)
	buf.Write(subtable)
	return buf.Bytes()
}

// buildName emits FontFamily (1), Subfamily (2), and PostScriptName (6)
// records as Windows Unicode BMP strings.
func buildName(family, psName string) []byte {
	type record struct {
		nameID uint16
		value  string
	}
	records := []record{{1, family}, {2, "Regular"}, {6, psName}}
	type encoded struct {
		nameID uint16
		bytes  []byte
	}
	enc := make([]encoded, len(records))
	stringData := new(bytes.Buffer)
	for i, r := range records {
		var u16 bytes.Buffer
		for _, c := range r.value {
			write16(&u16, uint16(c))
		}
		enc[i] = encoded{r.nameID, u16.Bytes()}
		stringData.Write(u16.Bytes())
	}
	headerLen := 6 + len(records)*12
	buf := new(bytes.Buffer)
	write16(buf, 0)
	write16(buf, uint16(len(records)))
	write16(buf, uint16(headerLen))
	offset := uint16(0)
	for _, e := range enc {
		write16(buf, 3)
		write16(buf, 1)
		write16(buf, 0x0409)
		write16(buf, e.nameID)
		write16(buf, uint16(len(e.bytes)))
		write16(buf, offset)
		offset += uint16(len(e.bytes))
	}
	buf.Write(stringData.Bytes())
	return buf.Bytes()
}

// buildHmtx emits one longHorMetric per glyph: .notdef gets 500, then
// advanceFor(r) for each mapped character in GID order.
func buildHmtx() []byte {
	buf := new(bytes.Buffer)
	write16(buf, 500) // .notdef
	writeI16(buf, 0)
	for r := rune(firstChar); r <= lastChar; r++ {
		write16(buf, uint16(advanceFor(r)))
		writeI16(buf, 0)
	}
	return buf.Bytes()
}

// buildGlyf emits one empty glyph (numberOfContours == 0) per slot.
func buildGlyf(numGlyphs int) []byte {
	buf := new(bytes.Buffer)
	for range numGlyphs {
		writeI16(buf, 0)
	}
	return buf.Bytes()
}

// buildLoca emits long-format offsets; each empty glyph is 2 bytes.
func buildLoca(numGlyphs int) []byte {
	buf := new(bytes.Buffer)
	for i := 0; i <= numGlyphs; i++ {
		write32(buf, uint32(i*2))
	}
	return buf.Bytes()
}

// buildPost emits a format 3.0 post table (no glyph names).
func buildPost() []byte {
	buf := new(bytes.Buffer)
	write32(buf, 0x00030000)
	write32(buf, 0)
	writeI16(buf, -100)
	writeI16(buf, 50)
	for range 5 {
		write32(buf, 0)
	}
	return buf.Bytes()
}

// assembleTTF combines the tables into a TrueType binary. Tables in
// the directory must be sorted alphabetically by tag (per spec). Each
// table is 4-byte aligned in the file; the directory records absolute
// offsets and unpadded lengths.
func assembleTTF(tables map[string][]byte) []byte {
	// Sort tags.
	tags := make([]string, 0, len(tables))
	for t := range tables {
		tags = append(tags, t)
	}
	sort.Strings(tags)

	numTables := len(tags)
	searchRange := uint16(1)
	for searchRange*2 <= uint16(numTables) {
		searchRange *= 2
	}
	searchRange *= 16
	entrySelector := uint16(0)
	for (1 << entrySelector) < searchRange/16 {
		entrySelector++
	}
	rangeShift := uint16(numTables)*16 - searchRange

	headerLen := 12 + numTables*16
	offset := headerLen

	// First pass: compute aligned offsets.
	type entry struct {
		tag    string
		offset uint32
		length uint32
		data   []byte
	}
	entries := make([]entry, numTables)
	for i, t := range tags {
		data := tables[t]
		entries[i] = entry{
			tag:    t,
			offset: uint32(offset),
			length: uint32(len(data)),
			data:   data,
		}
		padded := (len(data) + 3) &^ 3
		offset += padded
	}
	totalSize := offset

	// Build the binary.
	out := make([]byte, totalSize)
	// Offset table.
	binary.BigEndian.PutUint32(out[0:4], 0x00010000) // sfntVersion = TrueType
	binary.BigEndian.PutUint16(out[4:6], uint16(numTables))
	binary.BigEndian.PutUint16(out[6:8], searchRange)
	binary.BigEndian.PutUint16(out[8:10], entrySelector)
	binary.BigEndian.PutUint16(out[10:12], rangeShift)

	// Directory entries.
	for i, e := range entries {
		base := 12 + i*16
		copy(out[base:base+4], padTag(e.tag))
		binary.BigEndian.PutUint32(out[base+4:base+8], tableChecksum(e.data))
		binary.BigEndian.PutUint32(out[base+8:base+12], e.offset)
		binary.BigEndian.PutUint32(out[base+12:base+16], e.length)
	}

	// Payload.
	for _, e := range entries {
		copy(out[e.offset:e.offset+e.length], e.data)
	}
	return out
}

// padTag right-pads a 1-3 character tag to 4 bytes with spaces.
func padTag(tag string) []byte {
	out := []byte{' ', ' ', ' ', ' '}
	copy(out, tag)
	return out
}

// tableChecksum computes the OpenType per-table sum: the sum of all
// 32-bit big-endian words modulo 2^32. Padding bytes are treated as
// zero per spec.
func tableChecksum(data []byte) uint32 {
	var sum uint32
	for i := 0; i < len(data); i += 4 {
		var word uint32
		end := i + 4
		if end > len(data) {
			end = len(data)
		}
		for j := i; j < end; j++ {
			word |= uint32(data[j]) << (8 * (3 - (j - i)))
		}
		sum += word
	}
	return sum
}

// --- small byte-writer helpers ---

func write16(buf *bytes.Buffer, v uint16) {
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v))
}

func writeI16(buf *bytes.Buffer, v int16) {
	write16(buf, uint16(v))
}

func write32(buf *bytes.Buffer, v uint32) {
	buf.WriteByte(byte(v >> 24))
	buf.WriteByte(byte(v >> 16))
	buf.WriteByte(byte(v >> 8))
	buf.WriteByte(byte(v))
}

func write64(buf *bytes.Buffer, v uint64) {
	for i := 7; i >= 0; i-- {
		buf.WriteByte(byte(v >> (8 * i)))
	}
}
