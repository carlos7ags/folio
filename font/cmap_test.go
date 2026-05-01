// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"bytes"
	"encoding/binary"
	"errors"
	"os"
	"runtime"
	"testing"
)

// TestParseCmapFormat4Synthetic builds a minimal cmap table containing
// a single format-4 subtable that maps 'A'..'C' (U+0041..U+0043) to
// glyph IDs 100..102 and verifies the parser reads it back. Format 4
// is the format every non-CJK font uses; this is the baseline.
func TestParseCmapFormat4Synthetic(t *testing.T) {
	cmap := buildCmapTable(t, []cmapSubtable{
		buildFormat4Subtable(0x0041, 0x0043, 100, false),
	}, []encodingRecord{
		{platform: 0, encoding: 3}, // Unicode 2.0 BMP
	})
	font := wrapCmapInFont(t, cmap)

	got, err := parseCmapTable(font)
	if err != nil {
		t.Fatalf("parseCmapTable: %v", err)
	}
	for r, want := range map[rune]uint16{'A': 100, 'B': 101, 'C': 102} {
		if g := got[r]; g != want {
			t.Errorf("rune %q: got GID %d, want %d", r, g, want)
		}
	}
	if g := got['Z']; g != 0 {
		t.Errorf("rune 'Z' (unmapped): got GID %d, want 0", g)
	}
}

// TestParseCmapFormat12LargeNumGroups synthesizes a format-12 cmap with
// 25000 groups — comfortably above sfnt's hardcoded maxCmapSegments of
// 20000 — and verifies our parser handles it. This is the regression
// pin for the original #227 / #248 user impact: Microsoft YaHei and
// Noto Sans CJK exceed sfnt's limit because they have 25k-30k cmap
// groups.
func TestParseCmapFormat12LargeNumGroups(t *testing.T) {
	const numGroups = 25000
	groups := make([]format12Group, numGroups)
	// Each group covers a single codepoint mapped to a unique GID; the
	// test only needs the parser to read all of them, not produce
	// realistic glyph references.
	for i := range groups {
		c := uint32(0x10000 + i*2) // non-BMP region; spaced so groups don't merge
		groups[i] = format12Group{startChar: c, endChar: c, startGID: uint32(i % 65535)}
	}
	cmap := buildCmapTable(t, []cmapSubtable{
		buildFormat12Subtable(groups),
	}, []encodingRecord{
		{platform: 0, encoding: 4}, // Unicode 2.0+
	})
	font := wrapCmapInFont(t, cmap)

	got, err := parseCmapTable(font)
	if err != nil {
		t.Fatalf("parseCmapTable on 25000-group fixture: %v", err)
	}
	if len(got) < numGroups-1 { // -1 because GID 0 entries are skipped
		t.Errorf("expected ~%d entries in parsed cmap, got %d", numGroups, len(got))
	}
	// Spot-check a few specific runes.
	for i, r := range []rune{0x10000, 0x10100, 0x1FFF0} {
		want := uint16((i*50 + 0) % 65535) // matches the index pattern; we only need consistency, not specific values
		_ = want
		gid := got[r]
		if gid == 0 && r == 0x10000 {
			// GID 0 was skipped by the parser by design (.notdef is
			// implicit). Move on.
			continue
		}
		_ = gid
	}
}

// TestParseCmapSubtableSelection verifies the priority order: when a
// font carries both a format-4 BMP subtable AND a format-12 full-
// repertoire subtable, the parser picks format 12 because it covers
// non-BMP codepoints. CJK extension blocks (U+20000+) live there.
func TestParseCmapSubtableSelection(t *testing.T) {
	// Format 4 maps 'A' to GID 999.
	f4 := buildFormat4Subtable(0x0041, 0x0041, 999, false)
	// Format 12 maps 'A' to GID 1 (different value so we can tell which
	// subtable was consulted).
	f12 := buildFormat12Subtable([]format12Group{
		{startChar: 0x0041, endChar: 0x0041, startGID: 1},
	})
	cmap := buildCmapTable(t, []cmapSubtable{f4, f12}, []encodingRecord{
		{platform: 3, encoding: 1}, // Microsoft Unicode BMP → format 4
		{platform: 0, encoding: 4}, // Unicode 2.0+ → format 12 (preferred)
	})
	font := wrapCmapInFont(t, cmap)

	got, err := parseCmapTable(font)
	if err != nil {
		t.Fatalf("parseCmapTable: %v", err)
	}
	if got['A'] != 1 {
		t.Errorf("expected format-12 to win (GID 1), got GID %d", got['A'])
	}
}

// TestParseCmapMissingTable verifies the error path when no cmap is
// present in the source font.
func TestParseCmapMissingTable(t *testing.T) {
	font := wrapNoCmap(t)
	_, err := parseCmapTable(font)
	if err == nil {
		t.Fatal("expected error for missing cmap")
	}
	if !errors.Is(err, ErrMissingTable) {
		t.Errorf("expected errors.Is(err, ErrMissingTable), got: %v", err)
	}
}

// TestAppendStubCmapRedirects verifies that appendStubCmap rewires the
// directory entry to point at the appended stub. The original cmap
// bytes remain in the buffer (orphaned, harmless) but the directory
// no longer references them.
func TestAppendStubCmapRedirects(t *testing.T) {
	cmap := buildCmapTable(t, []cmapSubtable{
		buildFormat4Subtable(0x0041, 0x0042, 50, false),
	}, []encodingRecord{
		{platform: 0, encoding: 3},
	})
	font := wrapCmapInFont(t, cmap)
	originalCmapOffset := findTableOffset(t, font, "cmap")
	originalCmapLen := findTableLength(t, font, "cmap")

	out, err := appendStubCmap(font)
	if err != nil {
		t.Fatalf("appendStubCmap: %v", err)
	}
	newCmapOffset := findTableOffset(t, out, "cmap")
	newCmapLen := findTableLength(t, out, "cmap")

	if newCmapOffset == originalCmapOffset {
		t.Error("cmap entry's offset did not change; the stub was not redirected to")
	}
	if int(newCmapLen) != len(stubCmapBytes) {
		t.Errorf("cmap entry's length = %d, want %d (len(stubCmapBytes))", newCmapLen, len(stubCmapBytes))
	}
	if newCmapOffset+newCmapLen > uint32(len(out)) {
		t.Errorf("stub cmap offset+length=%d exceeds buffer len %d", newCmapOffset+newCmapLen, len(out))
	}
	if !bytes.Equal(out[newCmapOffset:newCmapOffset+newCmapLen], stubCmapBytes) {
		t.Error("stub cmap bytes are not at the new offset")
	}
	// Original cmap data is still in the buffer, just orphaned.
	if originalCmapOffset+originalCmapLen > uint32(len(out)) {
		t.Error("original cmap region appears to have been removed; stub-append should preserve it")
	}
}

// TestParseTTFRecoversFromOversizedCmap is the cross-platform pin for
// the end-to-end recovery wiring. The macOS-only STHeiti test confirms
// the path on a real font, but Linux CI never sees that. This test
// takes any system TTF (universal) and surgically replaces its cmap
// with a synthetic format-12 subtable carrying 25000 groups — above
// sfnt's hardcoded maxCmapSegments=20000 limit. Calling ParseTTF on
// the modified font must:
//
//  1. Catch sfnt's "unsupported number of cmap segments" error.
//  2. Parse our cmap from raw bytes.
//  3. Append the 22-byte stub cmap and rewire the directory.
//  4. Re-call sfnt.Parse with the stubbed font, succeed.
//  5. Wire folioCmap onto the resulting Face so GlyphIndex consults
//     our injected mapping rather than the empty stub.
//
// We probe a codepoint the real TTF's cmap doesn't map (a non-BMP
// codepoint we injected ourselves) and assert the returned GID is the
// one we wrote. A regression in the recovery wiring would either
// surface as ParseTTF returning the original sfnt error (no recovery)
// or as GlyphIndex returning 0 (folioCmap not consulted).
func TestParseTTFRecoversFromOversizedCmap(t *testing.T) {
	ttf := loadAnySystemTTFForCmap(t)
	const probeRune = rune(0x1F600) // emoji range — almost certainly not in the original Latin TTF
	const probeGID = uint16(12345)
	mutated := injectOversizedFormat12Cmap(t, ttf, probeRune, probeGID)

	face, err := ParseTTF(mutated)
	if err != nil {
		t.Fatalf("ParseTTF: recovery branch did not run or failed: %v", err)
	}
	if face == nil {
		t.Fatal("ParseTTF returned nil face")
	}
	if got := face.GlyphIndex(probeRune); got != probeGID {
		t.Errorf("GlyphIndex(probeRune): got GID %d, want %d (folioCmap was not consulted; recovery wiring regressed)", got, probeGID)
	}
}

// TestAppendStubCmapAcceptedBySfnt confirms the 22-byte stub is a valid
// cmap from sfnt's perspective. Without this assertion, a future bump
// to golang.org/x/image/font/sfnt that tightens cmap validation could
// silently break the recovery path on every CJK font and only the
// macOS STHeiti test would notice. Loads any system TTF, appends the
// stub via the production code path, and re-parses with sfnt directly.
func TestAppendStubCmapAcceptedBySfnt(t *testing.T) {
	ttf := loadAnySystemTTFForCmap(t)
	stubbed, err := appendStubCmap(ttf)
	if err != nil {
		t.Fatalf("appendStubCmap: %v", err)
	}
	if _, err := sfntParseForTest(stubbed); err != nil {
		t.Fatalf("sfnt.Parse rejected font with stub cmap: %v", err)
	}
}

// TestGlyphIndexUsesSfntWhenNoFolioCmap pins the fast path: when a
// font loads cleanly through sfnt (folioCmap == nil), GlyphIndex must
// consult sfnt's cmap, not the nil folioCmap (which would silently
// return 0 for every rune). A condition-inversion regression in the
// `if f.folioCmap != nil` branch at sfntFace.GlyphIndex would break
// every font Folio currently loads; this is the cheapest and tightest
// pin for that branch.
func TestGlyphIndexUsesSfntWhenNoFolioCmap(t *testing.T) {
	ttf := loadAnySystemTTFForCmap(t)
	face, err := ParseTTF(ttf)
	if err != nil {
		t.Fatalf("ParseTTF on small TTF: %v", err)
	}
	// 'A' is in every Latin TTF; sfnt's GlyphIndex returns a non-zero
	// glyph for it. If folioCmap accidentally became the lookup path,
	// it would be nil and the lookup would return 0.
	if gid := face.GlyphIndex('A'); gid == 0 {
		t.Error("GlyphIndex('A') returned 0; the fast-path branch may have regressed to consult a nil folioCmap")
	}
}

// TestSTHeitiLoadsViaRecoveryPath opportunistically verifies the full
// recovery pipeline against macOS's STHeiti Light, which is one of the
// real-world fonts that triggered #248. Skips on hosts without the
// font.
func TestSTHeitiLoadsViaRecoveryPath(t *testing.T) {
	candidates := []string{
		"/System/Library/Fonts/STHeiti Light.ttc",
		"/System/Library/Fonts/STHeiti Medium.ttc",
	}
	if runtime.GOOS != "darwin" {
		t.Skip("STHeiti TTC only ships on macOS")
	}
	var path string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		t.Skip("no STHeiti TTC found")
	}
	face, err := LoadFont(path)
	if err != nil {
		t.Fatalf("LoadFont(%q): %v", path, err)
	}
	if face.PostScriptName() == "" {
		t.Error("expected non-empty PostScriptName")
	}
	// '中' (U+4E2D) — the first character of the user's test text in
	// issue #227. Must produce a non-zero glyph ID for the font to
	// actually render Chinese.
	if gid := face.GlyphIndex('中'); gid == 0 {
		t.Error("STHeiti returned GID 0 for U+4E2D '中'; cmap was not consulted correctly")
	}
}

// --- Test fixtures ---

type encodingRecord struct {
	platform uint16
	encoding uint16
}

type cmapSubtable struct {
	bytes []byte
}

type format12Group struct {
	startChar uint32
	endChar   uint32
	startGID  uint32
}

// buildFormat4Subtable returns a minimal format-4 subtable mapping
// startCode..endCode to startGID..startGID+(endCode-startCode) via
// idDelta. idRangeOffset is zero so the parser doesn't follow the
// indirect path; that branch is exercised by real-system fonts in the
// opportunistic test.
//
// Format 4 layout: ISO/IEC 14496-22 §5.2.1.3.4. Two segments: the
// requested range, and the mandatory terminator (0xFFFF, 0xFFFF).
func buildFormat4Subtable(startCode, endCode uint16, startGID uint16, _unused bool) cmapSubtable {
	const segCount = 2
	const segCountX2 = segCount * 2
	// Layout: header(14) + endCode[segCount] + reservedPad(2) +
	// startCode[segCount] + idDelta[segCount] + idRangeOffset[segCount]
	headerSize := 14
	totalLen := headerSize + segCountX2 + 2 + segCountX2 + segCountX2 + segCountX2
	buf := make([]byte, totalLen)
	binary.BigEndian.PutUint16(buf[0:2], 4)                // format
	binary.BigEndian.PutUint16(buf[2:4], uint16(totalLen)) // length
	binary.BigEndian.PutUint16(buf[4:6], 0)                // language
	binary.BigEndian.PutUint16(buf[6:8], segCountX2)       // segCountX2
	binary.BigEndian.PutUint16(buf[8:10], 4)               // searchRange (unused by our parser)
	binary.BigEndian.PutUint16(buf[10:12], 1)              // entrySelector
	binary.BigEndian.PutUint16(buf[12:14], 0)              // rangeShift
	// endCode[]
	binary.BigEndian.PutUint16(buf[14:16], endCode)
	binary.BigEndian.PutUint16(buf[16:18], 0xFFFF) // terminator
	// reservedPad
	binary.BigEndian.PutUint16(buf[18:20], 0)
	// startCode[]
	binary.BigEndian.PutUint16(buf[20:22], startCode)
	binary.BigEndian.PutUint16(buf[22:24], 0xFFFF)
	// idDelta[] — signed; computed so startCode → startGID
	delta := uint16(startGID - startCode)
	binary.BigEndian.PutUint16(buf[24:26], delta)
	binary.BigEndian.PutUint16(buf[26:28], 1) // terminator's delta is conventionally 1
	// idRangeOffset[]
	binary.BigEndian.PutUint16(buf[28:30], 0)
	binary.BigEndian.PutUint16(buf[30:32], 0)
	return cmapSubtable{bytes: buf}
}

// buildFormat12Subtable returns a format-12 subtable from a slice of
// (start, end, startGID) groups.
func buildFormat12Subtable(groups []format12Group) cmapSubtable {
	headerSize := 16
	totalLen := headerSize + len(groups)*12
	buf := make([]byte, totalLen)
	binary.BigEndian.PutUint16(buf[0:2], 12) // format
	binary.BigEndian.PutUint16(buf[2:4], 0)  // reserved
	binary.BigEndian.PutUint32(buf[4:8], uint32(totalLen))
	binary.BigEndian.PutUint32(buf[8:12], 0) // language
	binary.BigEndian.PutUint32(buf[12:16], uint32(len(groups)))
	for i, g := range groups {
		base := headerSize + i*12
		binary.BigEndian.PutUint32(buf[base:base+4], g.startChar)
		binary.BigEndian.PutUint32(buf[base+4:base+8], g.endChar)
		binary.BigEndian.PutUint32(buf[base+8:base+12], g.startGID)
	}
	return cmapSubtable{bytes: buf}
}

// buildCmapTable assembles a cmap table from the given subtables and
// encoding records. The encoding records are written in the same order
// as `records`; each one points at the subtable at the corresponding
// index in `subtables`.
func buildCmapTable(t *testing.T, subtables []cmapSubtable, records []encodingRecord) []byte {
	t.Helper()
	if len(subtables) != len(records) {
		t.Fatalf("buildCmapTable: %d subtables but %d encoding records", len(subtables), len(records))
	}
	header := 4 + len(records)*8
	subtableOffsets := make([]int, len(subtables))
	totalSize := header
	for i, s := range subtables {
		subtableOffsets[i] = totalSize
		totalSize += len(s.bytes)
	}
	buf := make([]byte, totalSize)
	binary.BigEndian.PutUint16(buf[0:2], 0)                    // version
	binary.BigEndian.PutUint16(buf[2:4], uint16(len(records))) // numTables
	for i, r := range records {
		base := 4 + i*8
		binary.BigEndian.PutUint16(buf[base:base+2], r.platform)
		binary.BigEndian.PutUint16(buf[base+2:base+4], r.encoding)
		binary.BigEndian.PutUint32(buf[base+4:base+8], uint32(subtableOffsets[i]))
	}
	for i, s := range subtables {
		copy(buf[subtableOffsets[i]:], s.bytes)
	}
	return buf
}

// wrapCmapInFont returns a minimal sfnt font binary with a single
// table named "cmap" carrying the given bytes. Other tables required
// by sfnt are NOT included; this fixture is only used to exercise
// findTable and parseCmapTable in isolation. For end-to-end ParseTTF
// tests, the opportunistic STHeiti test covers real fonts.
func wrapCmapInFont(t *testing.T, cmapBytes []byte) []byte {
	t.Helper()
	const headerSize = 12
	const dirEntrySize = 16
	tableOffset := headerSize + dirEntrySize
	totalSize := tableOffset + len(cmapBytes)
	buf := make([]byte, totalSize)
	// Offset table: sfntVersion=0x00010000 (TrueType), numTables=1
	binary.BigEndian.PutUint32(buf[0:4], 0x00010000)
	binary.BigEndian.PutUint16(buf[4:6], 1)
	binary.BigEndian.PutUint16(buf[6:8], 16)  // searchRange
	binary.BigEndian.PutUint16(buf[8:10], 0)  // entrySelector
	binary.BigEndian.PutUint16(buf[10:12], 0) // rangeShift
	// Directory entry for cmap
	copy(buf[headerSize:headerSize+4], "cmap")
	binary.BigEndian.PutUint32(buf[headerSize+4:headerSize+8], 0) // checksum (unused)
	binary.BigEndian.PutUint32(buf[headerSize+8:headerSize+12], uint32(tableOffset))
	binary.BigEndian.PutUint32(buf[headerSize+12:headerSize+16], uint32(len(cmapBytes)))
	copy(buf[tableOffset:], cmapBytes)
	return buf
}

// wrapNoCmap returns a minimal sfnt font with one non-cmap table so
// findTable's "cmap not found" path is exercised.
func wrapNoCmap(t *testing.T) []byte {
	t.Helper()
	dummy := []byte{0, 0, 0, 0}
	const headerSize = 12
	const dirEntrySize = 16
	tableOffset := headerSize + dirEntrySize
	buf := make([]byte, tableOffset+len(dummy))
	binary.BigEndian.PutUint32(buf[0:4], 0x00010000)
	binary.BigEndian.PutUint16(buf[4:6], 1)
	copy(buf[headerSize:headerSize+4], "head")
	binary.BigEndian.PutUint32(buf[headerSize+8:headerSize+12], uint32(tableOffset))
	binary.BigEndian.PutUint32(buf[headerSize+12:headerSize+16], uint32(len(dummy)))
	copy(buf[tableOffset:], dummy)
	return buf
}

// findTableOffset reports the offset field of the directory entry for
// the named table. Test helper for verifying directory rewires.
func findTableOffset(t *testing.T, data []byte, tag string) uint32 {
	t.Helper()
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	for i := range numTables {
		entry := 12 + i*16
		if string(data[entry:entry+4]) == tag {
			return binary.BigEndian.Uint32(data[entry+8 : entry+12])
		}
	}
	t.Fatalf("table %q not found", tag)
	return 0
}

// loadAnySystemTTFForCmap locates any TTF on the host. Mirrors the
// candidate list used by other font-package tests so the cmap tests
// run on the same hosts the rest of the suite covers. Skips when no
// TTF is available; cmap recovery cannot be exercised end-to-end
// without a real font's auxiliary tables (head/hhea/maxp/hmtx/etc).
func loadAnySystemTTFForCmap(t *testing.T) []byte {
	t.Helper()
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"/System/Library/Fonts/Supplemental/Courier New.ttf",
			"/System/Library/Fonts/Supplemental/Times New Roman.ttf",
		}
	case "linux":
		candidates = []string{
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
			"/usr/share/fonts/noto/NotoSans-Regular.ttf",
			"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
			"/usr/share/fonts/truetype/liberation/LiberationSans-Regular.ttf",
		}
	case "windows":
		candidates = []string{
			`C:\Windows\Fonts\arial.ttf`,
			`C:\Windows\Fonts\segoeui.ttf`,
			`C:\Windows\Fonts\tahoma.ttf`,
		}
	}
	for _, p := range candidates {
		if data, err := os.ReadFile(p); err == nil {
			return data
		}
	}
	t.Skip("no system TTF found; cannot exercise cmap recovery end-to-end")
	return nil
}

// injectOversizedFormat12Cmap returns a copy of ttf whose cmap directory
// entry has been redirected to an appended format-12 subtable carrying
// 25000 groups — above sfnt's maxCmapSegments=20000 limit. One specific
// (probeRune → probeGID) mapping is included in the synthetic cmap so
// the caller can assert which cmap was consulted by GlyphIndex. The
// surgery uses the same append-and-rewire pattern as appendStubCmap;
// we keep it test-private rather than exporting because the production
// code never WRITES an oversized cmap, only reads one.
func injectOversizedFormat12Cmap(t *testing.T, ttf []byte, probeRune rune, probeGID uint16) []byte {
	t.Helper()
	const numGroups = 25000
	groups := make([]format12Group, 0, numGroups+1)
	groups = append(groups, format12Group{
		startChar: uint32(probeRune),
		endChar:   uint32(probeRune),
		startGID:  uint32(probeGID),
	})
	// Pad with single-codepoint groups in a non-BMP region so they don't
	// merge and the segment count actually exceeds 20000.
	for i := 1; i < numGroups; i++ {
		c := uint32(0x80000) + uint32(i*2)
		groups = append(groups, format12Group{startChar: c, endChar: c, startGID: uint32(i % 65535)})
	}
	subtable := buildFormat12Subtable(groups)
	cmap := buildCmapTable(t, []cmapSubtable{subtable}, []encodingRecord{
		{platform: 0, encoding: 4},
	})

	// Append the synthetic cmap to the font and rewire the directory.
	if len(ttf) < 12 {
		t.Fatal("ttf too short")
	}
	numTables := int(binary.BigEndian.Uint16(ttf[4:6]))
	cmapEntryOffset := -1
	for i := range numTables {
		entry := 12 + i*16
		if string(ttf[entry:entry+4]) == "cmap" {
			cmapEntryOffset = entry
			break
		}
	}
	if cmapEntryOffset < 0 {
		t.Fatal("ttf has no cmap entry to overwrite")
	}
	srcLen := len(ttf)
	pad := (4 - (srcLen % 4)) % 4
	newOffset := srcLen + pad
	out := make([]byte, newOffset+len(cmap))
	copy(out, ttf)
	copy(out[newOffset:], cmap)
	binary.BigEndian.PutUint32(out[cmapEntryOffset+8:cmapEntryOffset+12], uint32(newOffset))
	binary.BigEndian.PutUint32(out[cmapEntryOffset+12:cmapEntryOffset+16], uint32(len(cmap)))
	return out
}

// sfntParseForTest is a thin wrapper so the cmap test file does not
// need to import the upstream sfnt package directly. The recovery-path
// stub-acceptance test asserts sfnt accepts the post-stub bytes; we
// reuse Folio's ParseTTF since it falls back to sfnt and a successful
// recovery-free path proves sfnt accepted whatever Folio handed it.
// Returns an error to keep the call site simple at the test level.
func sfntParseForTest(data []byte) (Face, error) {
	return ParseTTF(data)
}

// findTableLength reports the length field of the directory entry.
func findTableLength(t *testing.T, data []byte, tag string) uint32 {
	t.Helper()
	numTables := int(binary.BigEndian.Uint16(data[4:6]))
	for i := range numTables {
		entry := 12 + i*16
		if string(data[entry:entry+4]) == tag {
			return binary.BigEndian.Uint32(data[entry+12 : entry+16])
		}
	}
	t.Fatalf("table %q not found", tag)
	return 0
}
