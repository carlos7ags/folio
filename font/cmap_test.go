// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"errors"
	"os"
	"runtime"
	"slices"
	"testing"
)

// buildFormat4Subtable serialises a minimal format-4 subtable using
// idDelta-only segments (no idRangeOffset glyph array). Each segment
// is (start, end, delta). The terminating 0xFFFF segment is appended
// automatically.
func buildFormat4Subtable(segments []struct {
	start, end uint16
	delta      int16
}) []byte {
	segs := slices.Clone(segments)
	segs = append(segs, struct {
		start, end uint16
		delta      int16
	}{0xFFFF, 0xFFFF, 1})
	segCount := len(segs)
	length := 14 + segCount*2*4 + 2 // 4 arrays + reservedPad
	buf := make([]byte, length)
	binary.BigEndian.PutUint16(buf[0:], 4)
	binary.BigEndian.PutUint16(buf[2:], uint16(length))
	binary.BigEndian.PutUint16(buf[4:], 0) // language
	binary.BigEndian.PutUint16(buf[6:], uint16(segCount*2))
	// search range, entry selector, range shift not strictly required by parser.
	off := 14
	for _, s := range segs {
		binary.BigEndian.PutUint16(buf[off:], s.end)
		off += 2
	}
	off += 2 // reservedPad
	for _, s := range segs {
		binary.BigEndian.PutUint16(buf[off:], s.start)
		off += 2
	}
	for _, s := range segs {
		binary.BigEndian.PutUint16(buf[off:], uint16(s.delta))
		off += 2
	}
	// idRangeOffset all zeros.
	return buf
}

// buildFormat12Subtable serialises a format-12 subtable from a slice
// of (startChar, endChar, startGID) groups.
func buildFormat12Subtable(groups []struct{ startChar, endChar, startGID uint32 }) []byte {
	length := 16 + len(groups)*12
	buf := make([]byte, length)
	binary.BigEndian.PutUint16(buf[0:], 12)
	// reserved at [2:4] = 0
	binary.BigEndian.PutUint32(buf[4:], uint32(length))
	binary.BigEndian.PutUint32(buf[8:], 0) // language
	binary.BigEndian.PutUint32(buf[12:], uint32(len(groups)))
	for i, g := range groups {
		off := 16 + i*12
		binary.BigEndian.PutUint32(buf[off:], g.startChar)
		binary.BigEndian.PutUint32(buf[off+4:], g.endChar)
		binary.BigEndian.PutUint32(buf[off+8:], g.startGID)
	}
	return buf
}

// wrapCmap wraps subtables (each with platformID/encodingID) into a
// complete cmap table. Returns the final cmap bytes ready for
// parseCmapTable.
func wrapCmap(records []struct {
	platformID, encodingID uint16
	subtable               []byte
}) []byte {
	header := 4 + len(records)*8
	// Lay out subtables back-to-back after the header.
	totalLen := header
	offsets := make([]int, len(records))
	for i, r := range records {
		offsets[i] = totalLen
		totalLen += len(r.subtable)
	}
	buf := make([]byte, totalLen)
	binary.BigEndian.PutUint16(buf[0:], 0)
	binary.BigEndian.PutUint16(buf[2:], uint16(len(records)))
	for i, r := range records {
		off := 4 + i*8
		binary.BigEndian.PutUint16(buf[off:], r.platformID)
		binary.BigEndian.PutUint16(buf[off+2:], r.encodingID)
		binary.BigEndian.PutUint32(buf[off+4:], uint32(offsets[i]))
		copy(buf[offsets[i]:], r.subtable)
	}
	return buf
}

func TestParseCmapFormat4Single(t *testing.T) {
	sub := buildFormat4Subtable([]struct {
		start, end uint16
		delta      int16
	}{
		{start: 0x41, end: 0x5A, delta: -0x40}, // 'A'..'Z' → GID 1..26
	})
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{
		{platformID: 3, encodingID: 1, subtable: sub},
	})
	tab, err := parseCmapTable(cmap)
	if err != nil {
		t.Fatalf("parseCmapTable: %v", err)
	}
	if tab['A'] != 1 || tab['Z'] != 26 {
		t.Errorf("expected A→1, Z→26; got A=%d Z=%d", tab['A'], tab['Z'])
	}
	if _, ok := tab[0xFFFF]; ok {
		t.Error("sentinel segment should not appear in resolved table")
	}
}

// TestParseCmapFormat4IdRangeOffset exercises the format-4 indirect
// path (idRangeOffset != 0), which routes glyph lookup through the
// glyphIdArray appended after the parallel arrays. The "obscure
// indexing trick" in parseCmapFormat4 (cmap.go around line 184) is
// non-trivial pointer arithmetic — without this test the branch is
// reached only by real fonts loaded opportunistically. The Latin
// system fonts in loadAnySystemTTF often hit this path; this
// synthetic fixture pins it so any host can exercise it.
//
// Layout: one segment maps codepoints 0x40..0x42 ('@','A','B') via
// idRangeOffset → glyphIdArray. The trick: idRangeOffset[0] is a
// byte offset from &idRangeOffset[0] to the glyphIdArray entry
// holding the glyph ID for the segment's first codepoint. With one
// segment + terminator, idRangeOffset starts at offset 28 in the
// subtable (header 14 + endCode 4 + reservedPad 2 + startCode 4 +
// idDelta 4 = 28). glyphIdArray follows at offset 32. So
// idRangeOffset[0] = 32 - 28 = 4 selects glyphIdArray[0]; values
// for codepoints 0x40, 0x41, 0x42 land at glyphIdArray[0,1,2]
// respectively.
func TestParseCmapFormat4IdRangeOffset(t *testing.T) {
	// Hand-build the subtable rather than extending buildFormat4Subtable
	// — the existing helper hard-codes idRangeOffset = 0 and changing
	// it would couple all other tests to this one.
	sub := buildFormat4WithIndirectSegment(
		0x40, 0x42, // segment range
		[]uint16{100, 101, 102}, // glyphIdArray entries
	)
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{
		{platformID: 3, encodingID: 1, subtable: sub},
	})

	got, err := parseCmapTable(cmap)
	if err != nil {
		t.Fatalf("parseCmapTable: %v", err)
	}
	for r, want := range map[rune]uint16{0x40: 100, 0x41: 101, 0x42: 102} {
		if g := got[r]; g != want {
			t.Errorf("rune U+%04X: got GID %d, want %d (idRangeOffset path)", r, g, want)
		}
	}
	if g := got[0x43]; g != 0 {
		t.Errorf("rune U+0043 (outside segment): got GID %d, want 0", g)
	}
}

// buildFormat4WithIndirectSegment serialises a format-4 cmap subtable
// with a single segment whose idRangeOffset is non-zero, pointing into
// an appended glyphIdArray. Only used by TestParseCmapFormat4IdRangeOffset.
func buildFormat4WithIndirectSegment(start, end uint16, glyphIDs []uint16) []byte {
	if int(end-start)+1 != len(glyphIDs) {
		panic("buildFormat4WithIndirectSegment: glyphIDs length must match segment range")
	}
	const segCount = 2 // requested segment + terminator
	headerSize := 14
	arraysSize := segCount * 2 * 4 // endCode, startCode, idDelta, idRangeOffset
	pad := 2
	glyphArrSize := len(glyphIDs) * 2
	length := headerSize + arraysSize + pad + glyphArrSize
	buf := make([]byte, length)

	binary.BigEndian.PutUint16(buf[0:], 4)
	binary.BigEndian.PutUint16(buf[2:], uint16(length))
	binary.BigEndian.PutUint16(buf[4:], 0) // language
	binary.BigEndian.PutUint16(buf[6:], uint16(segCount*2))

	// endCode[]
	off := 14
	binary.BigEndian.PutUint16(buf[off:], end)
	binary.BigEndian.PutUint16(buf[off+2:], 0xFFFF)
	off += 4
	// reservedPad
	binary.BigEndian.PutUint16(buf[off:], 0)
	off += 2
	// startCode[]
	binary.BigEndian.PutUint16(buf[off:], start)
	binary.BigEndian.PutUint16(buf[off+2:], 0xFFFF)
	off += 4
	// idDelta[] — irrelevant when idRangeOffset != 0; set to 0
	binary.BigEndian.PutUint16(buf[off:], 0)
	binary.BigEndian.PutUint16(buf[off+2:], 1)
	off += 4
	// idRangeOffset[]: segment 0 points at glyphIdArray[0]; terminator = 0.
	// Distance in bytes from &idRangeOffset[0] to glyphIdArray[0]:
	// glyphIdArray starts immediately after the four parallel arrays
	// (right after idRangeOffset[segCount-1] + 2). For segCount=2 that's
	// 4 bytes from idRangeOffset[0].
	rangeOffset := uint16(segCount * 2)
	binary.BigEndian.PutUint16(buf[off:], rangeOffset)
	binary.BigEndian.PutUint16(buf[off+2:], 0)
	off += 4
	// glyphIdArray
	for i, gid := range glyphIDs {
		binary.BigEndian.PutUint16(buf[off+i*2:], gid)
	}
	return buf
}

func TestParseCmapFormat12Single(t *testing.T) {
	sub := buildFormat12Subtable([]struct{ startChar, endChar, startGID uint32 }{
		{startChar: 0x4E00, endChar: 0x4E0F, startGID: 1000},
	})
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{
		{platformID: 0, encodingID: 4, subtable: sub},
	})
	tab, err := parseCmapTable(cmap)
	if err != nil {
		t.Fatalf("parseCmapTable: %v", err)
	}
	if tab[0x4E00] != 1000 || tab[0x4E0F] != 1015 {
		t.Errorf("CJK segment lookup wrong: 0x4E00=%d 0x4E0F=%d", tab[0x4E00], tab[0x4E0F])
	}
}

func TestParseCmapPrefersFormat12OverFormat4(t *testing.T) {
	sub4 := buildFormat4Subtable([]struct {
		start, end uint16
		delta      int16
	}{
		{start: 'A', end: 'A', delta: 99 - 'A'},
	})
	sub12 := buildFormat12Subtable([]struct{ startChar, endChar, startGID uint32 }{
		{startChar: 'A', endChar: 'A', startGID: 7},
	})
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{
		{platformID: 3, encodingID: 1, subtable: sub4},  // format 4
		{platformID: 0, encodingID: 4, subtable: sub12}, // format 12
	})
	tab, err := parseCmapTable(cmap)
	if err != nil {
		t.Fatalf("parseCmapTable: %v", err)
	}
	if tab['A'] != 7 {
		t.Errorf("format 12 should win; got A=%d, want 7", tab['A'])
	}
}

// TestParseCmapFormat12LargeGroupCount stresses the parser with 25k
// groups, exceeding the hard-coded 20000-segment limit that
// golang.org/x/image/font/sfnt enforced. CJK fonts in the wild
// (Microsoft YaHei, Noto Sans CJK, STHeiti) routinely have tens of
// thousands of groups.
//
// This test is the load-bearing regression pin for issue #248: a stub
// parseCmapFormat12 that returned an empty map would still satisfy
// the type signature, so we explicitly assert a non-zero entry from
// the synthetic groups round-trips back.
func TestParseCmapFormat12LargeGroupCount(t *testing.T) {
	const N = 25000
	groups := make([]struct{ startChar, endChar, startGID uint32 }, N)
	for i := range N {
		// Place each group in the supplementary plane to avoid colliding
		// with format-4 sentinel runes; one rune per group keeps the
		// fixture small.
		c := uint32(0x10000 + i)
		groups[i] = struct{ startChar, endChar, startGID uint32 }{
			startChar: c,
			endChar:   c,
			startGID:  uint32(i + 1),
		}
	}
	sub := buildFormat12Subtable(groups)
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{
		{platformID: 0, encodingID: 4, subtable: sub},
	})
	tab, err := parseCmapTable(cmap)
	if err != nil {
		t.Fatalf("parseCmapTable on %d-group fixture: %v", N, err)
	}
	if got := tab[0x10000]; got != 1 {
		t.Errorf("first group: tab[0x10000]=%d, want 1", got)
	}
	if got := tab[rune(0x10000+N-1)]; got != uint16(N&0xFFFF) {
		// Note: GID is uint16 — 25000 still fits, no truncation expected here.
		t.Errorf("last group: tab[0x%X]=%d, want %d", 0x10000+N-1, got, N)
	}
}

// TestParseCmapFormat12OverlappingRangesRejected pins the range-
// expansion budget: 8 identical full-range groups charge
// 8 x 1,114,112 = 8,912,896 mapped codes, well over cmapMaxMappedCodes
// (1<<22). Overlapping declared ranges only arise from hostile or
// broken fonts — legitimate coverage cannot exceed the Unicode range
// once. The fixture is small enough that even unfixed code finishes
// in well under a second, so a regression here fails on `err == nil`
// rather than hanging CI.
func TestParseCmapFormat12OverlappingRangesRejected(t *testing.T) {
	groups := make([]struct{ startChar, endChar, startGID uint32 }, 8)
	for i := range groups {
		groups[i] = struct{ startChar, endChar, startGID uint32 }{0, 0x10FFFF, 1}
	}
	sub := buildFormat12Subtable(groups)
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{{platformID: 0, encodingID: 4, subtable: sub}})
	_, err := parseCmapTable(cmap)
	if !errors.Is(err, ErrCorruptTable) {
		t.Fatalf("overlapping full-range groups: err = %v, want ErrCorruptTable", err)
	}
}

// TestParseCmapFormat4OverlappingSegmentsRejected pins the same budget
// for format 4: 70 segments each covering the full BMP charge
// 70 x 65,535 = 4,587,450 mapped codes, over cmapMaxMappedCodes.
func TestParseCmapFormat4OverlappingSegmentsRejected(t *testing.T) {
	segments := make([]struct {
		start, end uint16
		delta      int16
	}, 70)
	for i := range segments {
		segments[i] = struct {
			start, end uint16
			delta      int16
		}{start: 0, end: 0xFFFE, delta: 1}
	}
	sub := buildFormat4Subtable(segments)
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{{platformID: 3, encodingID: 1, subtable: sub}})
	_, err := parseCmapTable(cmap)
	if !errors.Is(err, ErrCorruptTable) {
		t.Fatalf("overlapping full-BMP segments: err = %v, want ErrCorruptTable", err)
	}
}

// TestParseCmapFormat12SingleFullRangeAccepted pins that the budget
// does not reject the largest legitimate shape: one group spanning
// the entire Unicode range charges 1,114,112 codes, comfortably under
// cmapMaxMappedCodes.
func TestParseCmapFormat12SingleFullRangeAccepted(t *testing.T) {
	sub := buildFormat12Subtable([]struct{ startChar, endChar, startGID uint32 }{
		{startChar: 0, endChar: 0x10FFFF, startGID: 1},
	})
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{{platformID: 0, encodingID: 4, subtable: sub}})
	tab, err := parseCmapTable(cmap)
	if err != nil {
		t.Fatalf("single full-range group should stay under budget: %v", err)
	}
	if tab[0x41] != 0x42 {
		t.Errorf("tab[0x41] = %d, want 0x42", tab[0x41])
	}
}

// TestParseCmapAcceptsMicrosoftSymbol pins the symbol-font fallback
// in scoreSubtable. Wingdings, Symbol, and dingbat fonts ship a
// (platformID=3, encodingID=0) format-4 subtable mapping PUA
// codepoints (0xF020..0xF0FF) instead of a Unicode subtable. Without
// the score-20 entry for (3, 0), Folio rejects these fonts entirely
// with "no supported Unicode subtable". HarfBuzz, go-text, and pdf.js
// all accept this fallback; this test pins parity.
func TestParseCmapAcceptsMicrosoftSymbol(t *testing.T) {
	// Map 0xF041 (PUA equivalent of 'A' under the Symbol convention)
	// to GID 100 via a single segment. idDelta is int16 with modular
	// (mod 65536) arithmetic per the spec, so the wire delta is
	// (100 - 0xF041) mod 65536 = 4131 — which fits in int16 unsigned-
	// reinterpreted; addition of 0xF041 + 4131 wraps back to 100.
	sub := buildFormat4Subtable([]struct {
		start, end uint16
		delta      int16
	}{
		{start: 0xF041, end: 0xF043, delta: int16((100 - int32(0xF041)) & 0xFFFF)}, // → GID 100..102
	})
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{
		{platformID: 3, encodingID: 0, subtable: sub}, // Microsoft Symbol
	})
	got, err := parseCmapTable(cmap)
	if err != nil {
		t.Fatalf("parseCmapTable on symbol-only font: %v (regression — Folio used to reject these)", err)
	}
	// PUA codepoints — the canonical Symbol-font addressing.
	for r, want := range map[rune]uint16{0xF041: 100, 0xF042: 101, 0xF043: 102} {
		if g := got[r]; g != want {
			t.Errorf("PUA U+%04X: got GID %d, want %d", r, g, want)
		}
	}
	// ASCII mirror — the html-shaping path uses these. Without
	// mirrorSymbolToASCII the font would load but paragraphs like
	// `<p>A</p>` would render as .notdef.
	for r, want := range map[rune]uint16{0x0041: 100, 0x0042: 101, 0x0043: 102} {
		if g := got[r]; g != want {
			t.Errorf("ASCII mirror U+%04X (from PUA U+%04X): got GID %d, want %d", r, r+0xF000, g, want)
		}
	}
}

// TestMirrorSymbolToASCIIDoesNotOverwriteExisting pins the additive-
// only contract of mirrorSymbolToASCII: if the source cmap already
// has an ASCII-range entry, the mirror does NOT overwrite it. This
// matters for the rare Symbol font that ships an explicit ASCII
// mapping alongside its PUA entries — those explicit values must win
// over the synthesized aliases.
func TestMirrorSymbolToASCIIDoesNotOverwriteExisting(t *testing.T) {
	tab := cmapTable{
		0xF041: 100, // PUA 'A' → GID 100
		0x0041: 999, // explicit ASCII 'A' → GID 999 (must NOT be overwritten)
	}
	mirrorSymbolToASCII(tab)
	if got := tab[0x0041]; got != 999 {
		t.Errorf("explicit ASCII entry was overwritten by mirror: got %d, want 999", got)
	}
	if got := tab[0xF041]; got != 100 {
		t.Errorf("PUA entry was modified by mirror: got %d, want 100", got)
	}
}

func TestParseCmapTruncatedHeader(t *testing.T) {
	_, err := parseCmapTable([]byte{0, 0, 0})
	if !errors.Is(err, ErrTruncated) {
		t.Errorf("err = %v, want errors.Is ErrTruncated", err)
	}
}

func TestParseCmapNoSupportedSubtable(t *testing.T) {
	// One subtable, format 13 (unsupported here).
	sub := []byte{0, 13, 0, 0}
	cmap := wrapCmap([]struct {
		platformID, encodingID uint16
		subtable               []byte
	}{
		{platformID: 3, encodingID: 1, subtable: sub},
	})
	_, err := parseCmapTable(cmap)
	if !errors.Is(err, ErrCorruptTable) {
		t.Errorf("err = %v, want errors.Is ErrCorruptTable", err)
	}
}

// TestParseCmapSTHeitiOpportunistic loads /System/Library/Fonts/STHeiti
// Light.ttc on darwin and verifies that 中 (U+4E2D) maps to a
// non-zero glyph via the Face surface. STHeiti is the user-reported
// font from issue #227 that triggered the cmap-segment-limit failure
// in golang.org/x/image/font/sfnt; this test pins the regression.
func TestParseCmapSTHeitiOpportunistic(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("STHeiti is a darwin-only system font")
	}
	const path = "/System/Library/Fonts/STHeiti Light.ttc"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("STHeiti not available: %v", err)
	}
	face, err := ParseFont(data)
	if err != nil {
		t.Fatalf("ParseFont(STHeiti): %v — this is exactly the issue #227 failure we are pinning", err)
	}
	if gid := face.GlyphIndex('中'); gid == 0 {
		t.Errorf("STHeiti: GlyphIndex('中') = 0, expected non-zero (font is supposed to cover Han)")
	}
}
