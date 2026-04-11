// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"os"
	"runtime"
	"testing"
)

// TestParseGSUBFindsArabicFeatures loads a system font known to have
// Arabic GSUB features and verifies that ParseGSUB extracts at least
// one substitution for the init/medi/fina/isol features.
func TestParseGSUBFindsArabicFeatures(t *testing.T) {
	path := arabicFontPath()
	if path == "" {
		t.Skip("no system Arabic font found; skipping GSUB test")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	subs := ParseGSUB(data)
	if subs == nil {
		t.Fatalf("ParseGSUB returned nil for %s", path)
	}
	// At minimum, a good Arabic font should have at least init and fina.
	for _, feat := range []GSUBFeature{GSUBInit, GSUBFina} {
		table, ok := subs.Single[feat]
		if !ok || len(table) == 0 {
			t.Errorf("feature %q: not found or empty in %s", feat, path)
		}
	}
	t.Logf("GSUB from %s: init=%d medi=%d fina=%d isol=%d",
		path,
		len(subs.Single[GSUBInit]), len(subs.Single[GSUBMedi]),
		len(subs.Single[GSUBFina]), len(subs.Single[GSUBIsol]))
}

// TestParseGSUBNilOnStandardFont verifies that ParseGSUB returns nil
// for a font without GSUB tables (e.g. Helvetica standard font bytes
// are not available, so we use an empty slice).
func TestParseGSUBNilOnEmpty(t *testing.T) {
	if subs := ParseGSUB(nil); subs != nil {
		t.Error("expected nil for nil data")
	}
	if subs := ParseGSUB([]byte{}); subs != nil {
		t.Error("expected nil for empty data")
	}
}

// TestFindTableReturnsNilForMissing verifies findTable returns nil
// for a nonexistent table tag.
func TestFindTableReturnsNilForMissing(t *testing.T) {
	if tbl := findTable([]byte("not a font"), "GSUB"); tbl != nil {
		t.Error("expected nil for invalid data")
	}
}

// buildTTFWithGSUB builds a minimal TTF file containing a GSUB table
// with the given bytes. Returns raw font data that findTable can locate
// the GSUB entry in. Only head+GSUB tables are included.
func buildTTFWithGSUB(gsubData []byte) []byte {
	// Minimal TTF layout:
	// offset table (12) + 1 directory entry (16) + padded gsub data.
	numTables := 1
	headerLen := 12 + numTables*16
	// Pad GSUB to 4 bytes.
	padded := gsubData
	if rem := len(padded) % 4; rem != 0 {
		padded = append(padded, make([]byte, 4-rem)...)
	}
	total := headerLen + len(padded)
	buf := make([]byte, total)
	// sfntVersion = 0x00010000 (TrueType)
	buf[0] = 0x00
	buf[1] = 0x01
	buf[2] = 0x00
	buf[3] = 0x00
	// numTables
	buf[4] = 0
	buf[5] = byte(numTables)
	// searchRange, entrySelector, rangeShift — leave zero.
	// Directory entry for GSUB.
	copy(buf[12:16], []byte("GSUB"))
	// checksum (bytes 16-20) = 0.
	// offset (bytes 20-24)
	off := uint32(headerLen)
	buf[20] = byte(off >> 24)
	buf[21] = byte(off >> 16)
	buf[22] = byte(off >> 8)
	buf[23] = byte(off)
	// length (bytes 24-28)
	l := uint32(len(gsubData))
	buf[24] = byte(l >> 24)
	buf[25] = byte(l >> 16)
	buf[26] = byte(l >> 8)
	buf[27] = byte(l)
	// Copy GSUB data in.
	copy(buf[headerLen:], gsubData)
	return buf
}

// TestParseGSUBTruncatedHeader verifies that a GSUB table smaller than
// the 10-byte minimum returns nil.
func TestParseGSUBTruncatedHeader(t *testing.T) {
	cases := [][]byte{
		{},
		{0x00},
		make([]byte, 9), // just below the 10-byte header
	}
	for i, gsubData := range cases {
		ttf := buildTTFWithGSUB(gsubData)
		if subs := ParseGSUB(ttf); subs != nil {
			t.Errorf("case %d: expected nil for truncated GSUB, got %v", i, subs)
		}
	}
}

// TestParseGSUBEmptyLists verifies that ParseGSUB handles a GSUB
// table whose script/feature/lookup offsets point to zero-count lists
// without panicking and produces no substitutions.
func TestParseGSUBEmptyLists(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("ParseGSUB panicked on empty lists: %v", r)
		}
	}()
	// 16-byte GSUB: majorVersion(2), minorVersion(2), scriptListOff(2),
	// featureListOff(2), lookupListOff(2), then 6 zero bytes.
	// Each offset points at two zero bytes inside the buffer, so the
	// downstream parsers read a uint16 count of 0 and walk empty lists.
	gsubData := make([]byte, 16)
	gsubData[4] = 0x00
	gsubData[5] = 0x0A // scriptListOff = 10
	gsubData[6] = 0x00
	gsubData[7] = 0x0C // featureListOff = 12
	gsubData[8] = 0x00
	gsubData[9] = 0x0E // lookupListOff = 14
	ttf := buildTTFWithGSUB(gsubData)
	if subs := ParseGSUB(ttf); subs != nil {
		t.Errorf("expected nil from empty GSUB lists, got %+v", subs)
	}
}

// TestParseGSUBOutOfRangeOffsets builds a GSUB where the three offsets
// point past the end of the buffer. ParseGSUB should return nil.
func TestParseGSUBOutOfRangeOffsets(t *testing.T) {
	gsubData := make([]byte, 32)
	// scriptListOff = 9999 (far past end)
	gsubData[4] = 0x27
	gsubData[5] = 0x0F
	gsubData[6] = 0x27
	gsubData[7] = 0x0F
	gsubData[8] = 0x27
	gsubData[9] = 0x0F
	ttf := buildTTFWithGSUB(gsubData)
	if subs := ParseGSUB(ttf); subs != nil {
		t.Errorf("expected nil for out-of-range offsets, got %v", subs)
	}
}

// --- LookupType 4 (Ligature Substitution) unit tests ---

// TestApplyLigatureBasic exercises the simplest ligature: one two-component
// ligature replaces the matching GID pair with the ligature glyph.
func TestApplyLigatureBasic(t *testing.T) {
	g := &GSUBSubstitutions{
		Ligature: map[GSUBFeature]map[uint16][]LigatureSubst{
			GSUBLiga: {
				10: []LigatureSubst{
					{Components: []uint16{20}, LigatureGID: 99},
				},
			},
		},
	}
	got := g.ApplyLigature([]uint16{10, 20, 30}, GSUBLiga)
	want := []uint16{99, 30}
	if !uint16SliceEq(got, want) {
		t.Errorf("ApplyLigature: got %v, want %v", got, want)
	}
}

// TestApplyLigatureGreedyLongest confirms that when multiple ligatures
// share a prefix, the longest matching sequence wins. Input [10,20,30,40]
// with ligatures [10,20]->99 and [10,20,30]->100 must produce [100, 40].
func TestApplyLigatureGreedyLongest(t *testing.T) {
	g := &GSUBSubstitutions{
		Ligature: map[GSUBFeature]map[uint16][]LigatureSubst{
			GSUBLiga: {
				10: []LigatureSubst{
					// Intentionally not in length order; ParseGSUB sorts,
					// but callers may construct this directly — ApplyLigature
					// should still pick the longest via its candidate scan
					// when the slice is pre-sorted by ParseGSUB. Sort here
					// to match the documented invariant.
					{Components: []uint16{20, 30}, LigatureGID: 100},
					{Components: []uint16{20}, LigatureGID: 99},
				},
			},
		},
	}
	got := g.ApplyLigature([]uint16{10, 20, 30, 40}, GSUBLiga)
	want := []uint16{100, 40}
	if !uint16SliceEq(got, want) {
		t.Errorf("ApplyLigature: got %v, want %v", got, want)
	}
}

// TestApplyLigatureNoMatch verifies that unmatched glyph runs are left
// untouched and that matching and non-matching runs can interleave.
func TestApplyLigatureNoMatch(t *testing.T) {
	g := &GSUBSubstitutions{
		Ligature: map[GSUBFeature]map[uint16][]LigatureSubst{
			GSUBLiga: {
				10: []LigatureSubst{
					{Components: []uint16{20}, LigatureGID: 99},
				},
			},
		},
	}
	// No [10,20] pair; prefix starts with 10 but 2nd glyph doesn't match.
	got := g.ApplyLigature([]uint16{10, 21, 10, 20}, GSUBLiga)
	want := []uint16{10, 21, 99}
	if !uint16SliceEq(got, want) {
		t.Errorf("ApplyLigature: got %v, want %v", got, want)
	}
}

// TestApplyLigatureNilReceiver and missing-feature cases should no-op.
func TestApplyLigatureEmptyCases(t *testing.T) {
	var g *GSUBSubstitutions
	if got := g.ApplyLigature([]uint16{1, 2}, GSUBLiga); !uint16SliceEq(got, []uint16{1, 2}) {
		t.Errorf("nil receiver: got %v, want [1 2]", got)
	}
	empty := &GSUBSubstitutions{}
	if got := empty.ApplyLigature([]uint16{1, 2}, GSUBLiga); !uint16SliceEq(got, []uint16{1, 2}) {
		t.Errorf("empty ligature map: got %v, want [1 2]", got)
	}
	only := &GSUBSubstitutions{
		Ligature: map[GSUBFeature]map[uint16][]LigatureSubst{
			GSUBRlig: {1: []LigatureSubst{{Components: []uint16{2}, LigatureGID: 9}}},
		},
	}
	if got := only.ApplyLigature([]uint16{1, 2}, GSUBLiga); !uint16SliceEq(got, []uint16{1, 2}) {
		t.Errorf("wrong feature: got %v, want [1 2]", got)
	}
}

// TestParseGSUBLigatureEndToEnd builds a minimal synthetic GSUB table
// wired via ScriptList/FeatureList/LookupList to a LigatureSubstFormat1
// subtable and verifies that ParseGSUB surfaces the ligature.
func TestParseGSUBLigatureEndToEnd(t *testing.T) {
	gsub := buildLigatureGSUB(ligOptions{})
	ttf := buildTTFWithGSUB(gsub)
	subs := ParseGSUB(ttf)
	if subs == nil {
		t.Fatalf("ParseGSUB returned nil")
	}
	table, ok := subs.Ligature[GSUBLiga]
	if !ok {
		t.Fatalf("liga feature missing; have %v", subs.Ligature)
	}
	bucket := table[10]
	if len(bucket) != 1 {
		t.Fatalf("expected 1 ligature for key 10, got %d", len(bucket))
	}
	if bucket[0].LigatureGID != 99 || len(bucket[0].Components) != 1 || bucket[0].Components[0] != 20 {
		t.Errorf("unexpected ligature entry: %+v", bucket[0])
	}
	got := subs.ApplyLigature([]uint16{10, 20, 30}, GSUBLiga)
	if !uint16SliceEq(got, []uint16{99, 30}) {
		t.Errorf("ApplyLigature on parsed table: got %v, want [99 30]", got)
	}
}

// TestParseGSUBLigatureExtension wraps the LigatureSubst subtable inside
// a LookupType 7 (Extension) subtable, as large fonts commonly do, and
// confirms ParseGSUB follows the 32-bit extension offset.
func TestParseGSUBLigatureExtension(t *testing.T) {
	gsub := buildLigatureGSUB(ligOptions{Extension: true})
	ttf := buildTTFWithGSUB(gsub)
	subs := ParseGSUB(ttf)
	if subs == nil {
		t.Fatalf("ParseGSUB returned nil for extension-wrapped ligature")
	}
	bucket := subs.Ligature[GSUBLiga][10]
	if len(bucket) != 1 || bucket[0].LigatureGID != 99 {
		t.Fatalf("extension unwrap failed: %+v", bucket)
	}
}

// TestParseGSUBLigatureCoverageFormat2 builds the same ligature subtable
// but uses a Coverage Format 2 range to enumerate the single covered GID.
func TestParseGSUBLigatureCoverageFormat2(t *testing.T) {
	gsub := buildLigatureGSUB(ligOptions{CoverageFormat: 2})
	ttf := buildTTFWithGSUB(gsub)
	subs := ParseGSUB(ttf)
	if subs == nil {
		t.Fatalf("ParseGSUB returned nil for Coverage Format 2 ligature")
	}
	bucket := subs.Ligature[GSUBLiga][10]
	if len(bucket) != 1 || bucket[0].LigatureGID != 99 {
		t.Fatalf("Coverage Format 2 parsing failed: %+v", bucket)
	}
}

// uint16SliceEq compares two uint16 slices for equality. Only used in tests.
func uint16SliceEq(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ligOptions controls the synthetic GSUB blob built by buildLigatureGSUB.
type ligOptions struct {
	Extension      bool // wrap the LookupType 4 subtable in a LookupType 7
	CoverageFormat int  // 0 or 1 = format 1; 2 = format 2 (range)
}

// buildLigatureGSUB constructs a minimal GSUB byte blob containing a
// single "latn" script, a single "liga" feature, and a LookupType 4
// (optionally wrapped in LookupType 7) subtable that maps the two-glyph
// sequence [10, 20] to ligature glyph 99. All internal offsets are
// computed from the actual layout produced below so the blob is a
// faithful ISO 14496-22 §6.2 GSUB table.
func buildLigatureGSUB(opt ligOptions) []byte {
	put16 := func(buf []byte, v uint16) { buf[0] = byte(v >> 8); buf[1] = byte(v) }
	put32 := func(buf []byte, v uint32) {
		buf[0] = byte(v >> 24)
		buf[1] = byte(v >> 16)
		buf[2] = byte(v >> 8)
		buf[3] = byte(v)
	}

	// Build each section into its own buffer at a known position; final
	// offsets are computed by concatenation.
	var (
		header     = make([]byte, 10)
		scriptList []byte
		script     []byte
		langSys    []byte
		featList   []byte
		feature    []byte
		lookupList []byte
		lookup     []byte
		extSub     []byte // only used when opt.Extension
		ligSub     []byte
		coverage   []byte
		ligSet     []byte
		ligature   []byte
	)

	// --- inner-most first so sizes are known going outward ---

	// Ligature: ligGlyph=99, componentCount=2, components=[20]
	ligature = make([]byte, 6)
	put16(ligature[0:2], 99)
	put16(ligature[2:4], 2)
	put16(ligature[4:6], 20)

	// LigatureSet: ligCount=1, offsets=[ligatureOffset]
	// Layout inside set: [hdr(2) + offsets(2)] then the ligature.
	ligSet = make([]byte, 4+len(ligature))
	put16(ligSet[0:2], 1) // ligCount
	put16(ligSet[2:4], 4) // ligature offset (relative to ligSet start)
	copy(ligSet[4:], ligature)

	// Coverage: either format 1 or format 2 covering GID 10.
	switch opt.CoverageFormat {
	case 2:
		coverage = make([]byte, 10)
		put16(coverage[0:2], 2) // format
		put16(coverage[2:4], 1) // rangeCount
		put16(coverage[4:6], 10) // startGlyphID
		put16(coverage[6:8], 10) // endGlyphID
		put16(coverage[8:10], 0) // startCoverageIndex
	default:
		coverage = make([]byte, 6)
		put16(coverage[0:2], 1) // format
		put16(coverage[2:4], 1) // glyphCount
		put16(coverage[4:6], 10) // GID 10
	}

	// LigatureSubstFormat1 subtable:
	//   format(2) + coverageOff(2) + ligSetCount(2) + setOffsets[1](2)
	//   then the coverage, then the ligSet.
	ligSubHdr := 8
	ligSub = make([]byte, ligSubHdr+len(coverage)+len(ligSet))
	put16(ligSub[0:2], 1)                                  // format
	put16(ligSub[2:4], uint16(ligSubHdr))                  // coverageOff
	put16(ligSub[4:6], 1)                                  // ligSetCount
	put16(ligSub[6:8], uint16(ligSubHdr+len(coverage)))    // setOffset[0]
	copy(ligSub[ligSubHdr:], coverage)
	copy(ligSub[ligSubHdr+len(coverage):], ligSet)

	// Optional Extension subtable: format(2) + extType(2) + extOff(4)
	// where extOff is relative to the extension subtable start and points
	// to the wrapped LigatureSubst bytes appended immediately after.
	if opt.Extension {
		extHdr := 8
		extSub = make([]byte, extHdr+len(ligSub))
		put16(extSub[0:2], 1)                    // format = 1
		put16(extSub[2:4], 4)                    // wrapped type = 4
		put32(extSub[4:8], uint32(extHdr))       // offset to wrapped subtable
		copy(extSub[extHdr:], ligSub)
	}

	// Lookup: type(2) + flag(2) + subTableCount(2) + subTableOffsets[1](2)
	// Followed by the subtable itself (either extSub or ligSub).
	lookupHdr := 8
	inner := ligSub
	lookupType := uint16(4)
	if opt.Extension {
		inner = extSub
		lookupType = 7
	}
	lookup = make([]byte, lookupHdr+len(inner))
	put16(lookup[0:2], lookupType)        // lookupType
	put16(lookup[2:4], 0)                 // lookupFlag
	put16(lookup[4:6], 1)                 // subTableCount
	put16(lookup[6:8], uint16(lookupHdr)) // subTableOffset[0]
	copy(lookup[lookupHdr:], inner)

	// LookupList: lookupCount(2) + lookupOffsets[1](2) + lookup
	lookupListHdr := 4
	lookupList = make([]byte, lookupListHdr+len(lookup))
	put16(lookupList[0:2], 1)                     // lookupCount
	put16(lookupList[2:4], uint16(lookupListHdr)) // offset to lookup
	copy(lookupList[lookupListHdr:], lookup)

	// Feature: featureParamsOff(2) + lookupCount(2) + lookupIndex[0](2)
	feature = make([]byte, 6)
	put16(feature[0:2], 0) // featureParams
	put16(feature[2:4], 1) // lookupCount
	put16(feature[4:6], 0) // lookupListIndex

	// FeatureList: featureCount(2) + FeatureRecord[1](tag(4)+offset(2))
	featListHdr := 8
	featList = make([]byte, featListHdr+len(feature))
	put16(featList[0:2], 1)                   // featureCount
	copy(featList[2:6], []byte("liga"))       // tag
	put16(featList[6:8], uint16(featListHdr)) // featureOff
	copy(featList[featListHdr:], feature)

	// LangSys: lookupOrder(2)=0 + reqFeatureIndex(2)=0xFFFF +
	//          featureIndexCount(2)=1 + featureIndices[0](2)=0
	langSys = make([]byte, 8)
	put16(langSys[0:2], 0)
	put16(langSys[2:4], 0xFFFF)
	put16(langSys[4:6], 1)
	put16(langSys[6:8], 0)

	// Script: defaultLangSysOff(2) + langSysCount(2) + langSys
	scriptHdr := 4
	script = make([]byte, scriptHdr+len(langSys))
	put16(script[0:2], uint16(scriptHdr)) // defaultLangSysOff
	put16(script[2:4], 0)                  // langSysCount
	copy(script[scriptHdr:], langSys)

	// ScriptList: scriptCount(2) + ScriptRecord[1](tag(4)+offset(2)) + script
	scriptListHdr := 8
	scriptList = make([]byte, scriptListHdr+len(script))
	put16(scriptList[0:2], 1)                     // scriptCount
	copy(scriptList[2:6], []byte("latn"))         // tag
	put16(scriptList[6:8], uint16(scriptListHdr)) // scriptOff
	copy(scriptList[scriptListHdr:], script)

	// Header: GSUB version 1.0 + offsets to ScriptList/FeatureList/LookupList.
	scriptListOff := len(header)
	featListOff := scriptListOff + len(scriptList)
	lookupListOff := featListOff + len(featList)
	put16(header[0:2], 1) // majorVersion
	put16(header[2:4], 0) // minorVersion
	put16(header[4:6], uint16(scriptListOff))
	put16(header[6:8], uint16(featListOff))
	put16(header[8:10], uint16(lookupListOff))

	out := make([]byte, 0, lookupListOff+len(lookupList))
	out = append(out, header...)
	out = append(out, scriptList...)
	out = append(out, featList...)
	out = append(out, lookupList...)
	return out
}

func arabicFontPath() string {
	switch runtime.GOOS {
	case "darwin":
		if _, err := os.Stat("/System/Library/Fonts/SFArabic.ttf"); err == nil {
			return "/System/Library/Fonts/SFArabic.ttf"
		}
	case "linux":
		paths := []string{
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/noto/NotoSansArabic-Regular.ttf",
		}
		for _, p := range paths {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}
