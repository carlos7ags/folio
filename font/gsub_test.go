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
		put16(coverage[0:2], 2)  // format
		put16(coverage[2:4], 1)  // rangeCount
		put16(coverage[4:6], 10) // startGlyphID
		put16(coverage[6:8], 10) // endGlyphID
		put16(coverage[8:10], 0) // startCoverageIndex
	default:
		coverage = make([]byte, 6)
		put16(coverage[0:2], 1)  // format
		put16(coverage[2:4], 1)  // glyphCount
		put16(coverage[4:6], 10) // GID 10
	}

	// LigatureSubstFormat1 subtable:
	//   format(2) + coverageOff(2) + ligSetCount(2) + setOffsets[1](2)
	//   then the coverage, then the ligSet.
	ligSubHdr := 8
	ligSub = make([]byte, ligSubHdr+len(coverage)+len(ligSet))
	put16(ligSub[0:2], 1)                               // format
	put16(ligSub[2:4], uint16(ligSubHdr))               // coverageOff
	put16(ligSub[4:6], 1)                               // ligSetCount
	put16(ligSub[6:8], uint16(ligSubHdr+len(coverage))) // setOffset[0]
	copy(ligSub[ligSubHdr:], coverage)
	copy(ligSub[ligSubHdr+len(coverage):], ligSet)

	// Optional Extension subtable: format(2) + extType(2) + extOff(4)
	// where extOff is relative to the extension subtable start and points
	// to the wrapped LigatureSubst bytes appended immediately after.
	if opt.Extension {
		extHdr := 8
		extSub = make([]byte, extHdr+len(ligSub))
		put16(extSub[0:2], 1)              // format = 1
		put16(extSub[2:4], 4)              // wrapped type = 4
		put32(extSub[4:8], uint32(extHdr)) // offset to wrapped subtable
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
	put16(script[2:4], 0)                 // langSysCount
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

// --- LookupType 6 (Chaining Contextual Substitution) unit tests ---

// TestParseClassDefFormat1 verifies that a Format 1 ClassDef (consecutive
// glyph range starting at startGlyphID) is decoded correctly.
func TestParseClassDefFormat1(t *testing.T) {
	// format(2)=1, startGID(2)=10, count(2)=4, classes=[1,2,0,1]
	data := []byte{
		0, 1,
		0, 10,
		0, 4,
		0, 1,
		0, 2,
		0, 0,
		0, 1,
	}
	m := parseClassDef(data, 0)
	if m[10] != 1 || m[11] != 2 || m[13] != 1 {
		t.Errorf("Format 1 ClassDef wrong: %v", m)
	}
	if _, ok := m[12]; ok {
		t.Errorf("class 0 entry should be absent, got %v", m[12])
	}
}

// TestParseClassDefFormat2 verifies that a Format 2 ClassDef (range
// records) is decoded correctly.
func TestParseClassDefFormat2(t *testing.T) {
	// format=2, rangeCount=2, {start=5,end=7,cls=3}, {start=20,end=20,cls=9}
	data := []byte{
		0, 2,
		0, 2,
		0, 5, 0, 7, 0, 3,
		0, 20, 0, 20, 0, 9,
	}
	m := parseClassDef(data, 0)
	if m[5] != 3 || m[6] != 3 || m[7] != 3 || m[20] != 9 {
		t.Errorf("Format 2 ClassDef wrong: %v", m)
	}
}

// chainOptions controls buildChainContextGSUB.
type chainOptions struct {
	// Format is 1, 2, or 3. Defaults to 1.
	Format int
	// Extension wraps the chain context subtable in a LookupType 7.
	Extension bool
}

// TestChainContextFormat1Basic builds a synthetic GSUB with a Format 1
// chain rule "if backtrack=[5], input=[10,20], lookahead=[30], substitute
// glyph at sequenceIndex 0 via a single-sub lookup [10->99]" and verifies
// that ApplyChainContext substitutes 10 to 99 only when the full chain
// matches.
func TestChainContextFormat1Basic(t *testing.T) {
	gsub := buildChainContextGSUB(chainOptions{Format: 1})
	ttf := buildTTFWithGSUB(gsub)
	subs := ParseGSUB(ttf)
	if subs == nil {
		t.Fatalf("ParseGSUB returned nil")
	}
	got := subs.ApplyChainContext([]uint16{5, 10, 20, 30}, GSUBCalt)
	if !uint16SliceEq(got, []uint16{5, 99, 20, 30}) {
		t.Errorf("matching chain: got %v, want [5 99 20 30]", got)
	}
	// Backtrack doesn't match (6 instead of 5).
	got2 := subs.ApplyChainContext([]uint16{6, 10, 20, 30}, GSUBCalt)
	if !uint16SliceEq(got2, []uint16{6, 10, 20, 30}) {
		t.Errorf("non-matching backtrack: got %v, want [6 10 20 30]", got2)
	}
}

// TestChainContextFormat1Extension wraps the Format 1 chain subtable
// inside a LookupType 7 and confirms ParseGSUB follows the extension.
func TestChainContextFormat1Extension(t *testing.T) {
	gsub := buildChainContextGSUB(chainOptions{Format: 1, Extension: true})
	ttf := buildTTFWithGSUB(gsub)
	subs := ParseGSUB(ttf)
	if subs == nil {
		t.Fatalf("ParseGSUB returned nil for extension-wrapped chain")
	}
	got := subs.ApplyChainContext([]uint16{5, 10, 20, 30}, GSUBCalt)
	if !uint16SliceEq(got, []uint16{5, 99, 20, 30}) {
		t.Errorf("extension-wrapped chain: got %v, want [5 99 20 30]", got)
	}
}

// TestChainContextFormat2ClassBased verifies a class-based chain rule
// fires correctly for two different GIDs in the same input class.
func TestChainContextFormat2ClassBased(t *testing.T) {
	gsub := buildChainContextGSUB(chainOptions{Format: 2})
	ttf := buildTTFWithGSUB(gsub)
	subs := ParseGSUB(ttf)
	if subs == nil {
		t.Fatalf("ParseGSUB returned nil for Format 2 chain")
	}
	// Input class 1 = {10, 11}. Both should trigger the same rule,
	// which substitutes via lookup [10->99, 11->98].
	got1 := subs.ApplyChainContext([]uint16{5, 10, 20, 30}, GSUBCalt)
	if !uint16SliceEq(got1, []uint16{5, 99, 20, 30}) {
		t.Errorf("Format 2 trigger 10: got %v, want [5 99 20 30]", got1)
	}
	got2 := subs.ApplyChainContext([]uint16{5, 11, 20, 30}, GSUBCalt)
	if !uint16SliceEq(got2, []uint16{5, 98, 20, 30}) {
		t.Errorf("Format 2 trigger 11: got %v, want [5 98 20 30]", got2)
	}
}

// TestChainContextFormat3CoverageBased verifies a rule whose backtrack
// uses a coverage of three GIDs accepts any of those three.
func TestChainContextFormat3CoverageBased(t *testing.T) {
	gsub := buildChainContextGSUB(chainOptions{Format: 3})
	ttf := buildTTFWithGSUB(gsub)
	subs := ParseGSUB(ttf)
	if subs == nil {
		t.Fatalf("ParseGSUB returned nil for Format 3 chain")
	}
	for _, back := range []uint16{4, 5, 6} {
		got := subs.ApplyChainContext([]uint16{back, 10, 20, 30}, GSUBCalt)
		want := []uint16{back, 99, 20, 30}
		if !uint16SliceEq(got, want) {
			t.Errorf("Format 3 backtrack %d: got %v, want %v", back, got, want)
		}
	}
	// A backtrack GID not in the coverage must not trigger.
	got := subs.ApplyChainContext([]uint16{7, 10, 20, 30}, GSUBCalt)
	if !uint16SliceEq(got, []uint16{7, 10, 20, 30}) {
		t.Errorf("Format 3 non-matching backtrack: got %v", got)
	}
}

// TestChainContextSequenceIndexOne confirms that when the action's
// SequenceIndex is 1, the second glyph (not the trigger) is substituted.
func TestChainContextSequenceIndexOne(t *testing.T) {
	// Input=[10,20], action targets sequenceIndex=1, lookup is [20->77].
	gsub := buildChainContextGSUBSeqOne()
	ttf := buildTTFWithGSUB(gsub)
	subs := ParseGSUB(ttf)
	if subs == nil {
		t.Fatalf("ParseGSUB returned nil")
	}
	got := subs.ApplyChainContext([]uint16{5, 10, 20, 30}, GSUBCalt)
	if !uint16SliceEq(got, []uint16{5, 10, 77, 30}) {
		t.Errorf("seqIndex 1: got %v, want [5 10 77 30]", got)
	}
}

// TestChainContextNoMatchPreserves verifies that glyph runs that don't
// match any rule pass through unchanged.
func TestChainContextNoMatchPreserves(t *testing.T) {
	gsub := buildChainContextGSUB(chainOptions{Format: 1})
	ttf := buildTTFWithGSUB(gsub)
	subs := ParseGSUB(ttf)
	if subs == nil {
		t.Fatalf("ParseGSUB returned nil")
	}
	// No trigger glyph (10) present.
	in := []uint16{1, 2, 3, 4, 5}
	got := subs.ApplyChainContext(in, GSUBCalt)
	if !uint16SliceEq(got, in) {
		t.Errorf("no-match: got %v, want %v", got, in)
	}
}

// TestApplyChainContextEmptyCases verifies nil receivers and missing
// features no-op cleanly.
func TestApplyChainContextEmptyCases(t *testing.T) {
	var g *GSUBSubstitutions
	if got := g.ApplyChainContext([]uint16{1, 2}, GSUBCalt); !uint16SliceEq(got, []uint16{1, 2}) {
		t.Errorf("nil receiver: got %v", got)
	}
	empty := &GSUBSubstitutions{}
	if got := empty.ApplyChainContext([]uint16{1, 2}, GSUBCalt); !uint16SliceEq(got, []uint16{1, 2}) {
		t.Errorf("empty chain map: got %v", got)
	}
}

// put16 is a tiny helper used by the chain-context GSUB builders.
func put16(buf []byte, v uint16) { buf[0] = byte(v >> 8); buf[1] = byte(v) }

// put32 is a tiny helper used by the chain-context GSUB builders.
func put32(buf []byte, v uint32) {
	buf[0] = byte(v >> 24)
	buf[1] = byte(v >> 16)
	buf[2] = byte(v >> 8)
	buf[3] = byte(v)
}

// buildChainContextGSUB constructs a synthetic GSUB blob for chain context
// testing. The layout uses a single "latn" script with a "calt" feature
// that references TWO lookups in the LookupList:
//
//	lookup 0: LookupType 1 Single [10->99, 11->98, 20->77]
//	lookup 1: LookupType 6 Chain Context (format per opt)
//
// The calt feature lists lookup 1 only (the ChainContext). The ChainContext
// rule's action dispatches to lookup 0 with sequenceIndex 0. The extra
// Single lookup is intentionally NOT directly wired into the feature so
// that we exercise the action-dispatch path (not just feature-level single
// substitution).
func buildChainContextGSUB(opt chainOptions) []byte {
	// ---- Single lookup (lookup index 0) ----
	// SingleSubstFormat2: format(2)=2, coverageOff(2), substCount(2),
	// substitute[substCount](2 each). Coverage is format 1 with GIDs
	// [10, 11, 20] in order; substitutes are [99, 98, 77].
	singleCoverage := make([]byte, 4+3*2)
	put16(singleCoverage[0:2], 1)
	put16(singleCoverage[2:4], 3)
	put16(singleCoverage[4:6], 10)
	put16(singleCoverage[6:8], 11)
	put16(singleCoverage[8:10], 20)
	singleSub := make([]byte, 6+3*2)
	put16(singleSub[0:2], 2) // format
	put16(singleSub[2:4], 6) // coverageOff relative to subtable
	put16(singleSub[4:6], 3) // substCount
	put16(singleSub[6:8], 99)
	put16(singleSub[8:10], 98)
	put16(singleSub[10:12], 77)
	singleSub = append(singleSub, singleCoverage...)
	// Correct the coverage offset to point AFTER the header.
	put16(singleSub[2:4], 12)

	// ---- Chain context subtable (lookup index 1) ----
	var chainSub []byte
	switch opt.Format {
	case 2:
		chainSub = buildChainContextFormat2Subtable()
	case 3:
		chainSub = buildChainContextFormat3Subtable()
	default:
		chainSub = buildChainContextFormat1Subtable()
	}

	// Optional LookupType 7 wrapping for the chain subtable.
	if opt.Extension {
		ext := make([]byte, 8+len(chainSub))
		put16(ext[0:2], 1)
		put16(ext[2:4], 6)
		put32(ext[4:8], 8)
		copy(ext[8:], chainSub)
		chainSub = ext
	}

	return assembleTwoLookupGSUB(singleSub, chainSub, opt.Extension)
}

// buildChainContextFormat1Subtable constructs a ChainContextSubstFormat1
// subtable: one covered trigger GID (10), one ChainSubRuleSet, one rule
// with backtrack=[5], input=[10,20] (on-disk omits the first), lookahead=[30],
// and a single SubstLookupRecord (sequenceIndex=0, lookupListIndex=0).
func buildChainContextFormat1Subtable() []byte {
	// Rule: backCount(2)=1, back[0](2)=5,
	//       inputCount(2)=2, input[0](2)=20 (input[0] in on-disk is second glyph),
	//       lookCount(2)=1, look[0](2)=30,
	//       substCount(2)=1, substLookupRecord(4): seqIndex=0, lookupIndex=0.
	rule := make([]byte, 0)
	rule = append(rule, 0, 1, 0, 5)
	rule = append(rule, 0, 2, 0, 20)
	rule = append(rule, 0, 1, 0, 30)
	rule = append(rule, 0, 1, 0, 0, 0, 0)

	// ChainSubRuleSet: ruleCount(2)=1, ruleOff(2)=4, then rule bytes.
	ruleSet := make([]byte, 4+len(rule))
	put16(ruleSet[0:2], 1)
	put16(ruleSet[2:4], 4)
	copy(ruleSet[4:], rule)

	// Coverage for trigger GID 10 (format 1).
	cov := make([]byte, 6)
	put16(cov[0:2], 1)
	put16(cov[2:4], 1)
	put16(cov[4:6], 10)

	// Subtable: format(2)=1, coverageOff(2), setCount(2)=1, setOff[0](2),
	// then coverage, then ruleSet.
	hdr := 8
	sub := make([]byte, hdr+len(cov)+len(ruleSet))
	put16(sub[0:2], 1)
	put16(sub[2:4], uint16(hdr))
	put16(sub[4:6], 1)
	put16(sub[6:8], uint16(hdr+len(cov)))
	copy(sub[hdr:], cov)
	copy(sub[hdr+len(cov):], ruleSet)
	return sub
}

// buildChainContextFormat2Subtable constructs a Format 2 subtable where:
//
//	coverage = {10, 11}
//	inputClassDef: 10->1, 11->1 (both in class 1)
//	backtrackClassDef: 5->1
//	lookaheadClassDef: 30->1
//	one rule under class 1 with back=[1], input=[1,?], look=[1]
//
// The second input position in a class-based rule is a class number,
// not a GID, so we use another class for GID 20. inputClass: 20->2.
// The rule's inputSequence (class 2) therefore matches only GID 20.
func buildChainContextFormat2Subtable() []byte {
	// Rule: backCount(2)=1, back[0](2)=1 (class 1),
	//       inputCount(2)=2, inputSeq[0](2)=2 (class 2),
	//       lookCount(2)=1, look[0](2)=1 (class 1),
	//       substCount(2)=1, substLookupRecord: seqIndex=0, lookupIndex=0
	rule := make([]byte, 0)
	rule = append(rule, 0, 1, 0, 1)
	rule = append(rule, 0, 2, 0, 2)
	rule = append(rule, 0, 1, 0, 1)
	rule = append(rule, 0, 1, 0, 0, 0, 0)
	set1 := make([]byte, 4+len(rule))
	put16(set1[0:2], 1)
	put16(set1[2:4], 4)
	copy(set1[4:], rule)

	// Coverage format 1 for {10, 11}.
	cov := make([]byte, 8)
	put16(cov[0:2], 1)
	put16(cov[2:4], 2)
	put16(cov[4:6], 10)
	put16(cov[6:8], 11)

	// Input ClassDef (format 1): startGID=10, count=11,
	// classes covering GIDs 10..20. 10->1, 11->1, 12..19->0, 20->2.
	inputClassDef := make([]byte, 6+11*2)
	put16(inputClassDef[0:2], 1)
	put16(inputClassDef[2:4], 10)
	put16(inputClassDef[4:6], 11)
	put16(inputClassDef[6:8], 1)  // 10
	put16(inputClassDef[8:10], 1) // 11
	// 12..19 already zero
	put16(inputClassDef[6+10*2:6+10*2+2], 2) // 20

	// Backtrack ClassDef (format 2): one range {5..5, class=1}.
	backClassDef := make([]byte, 4+6)
	put16(backClassDef[0:2], 2)
	put16(backClassDef[2:4], 1)
	put16(backClassDef[4:6], 5)
	put16(backClassDef[6:8], 5)
	put16(backClassDef[8:10], 1)

	// Lookahead ClassDef (format 2): one range {30..30, class=1}.
	lookClassDef := make([]byte, 4+6)
	put16(lookClassDef[0:2], 2)
	put16(lookClassDef[2:4], 1)
	put16(lookClassDef[4:6], 30)
	put16(lookClassDef[6:8], 30)
	put16(lookClassDef[8:10], 1)

	// Header layout: format(2)=2, covOff(2), backOff(2), inOff(2),
	// lookOff(2), setCount(2), setOffs[setCount](2).
	// setCount needs to cover class 1 — i.e. at least 2 entries (class 0, class 1).
	setCount := 2
	hdr := 12 + setCount*2

	// Place coverage, classDefs, and set1 after the header.
	body := make([]byte, 0)
	cOff := hdr
	body = append(body, cov...)
	backOff := cOff + len(cov)
	body = append(body, backClassDef...)
	inOff := backOff + len(backClassDef)
	body = append(body, inputClassDef...)
	lookOff := inOff + len(inputClassDef)
	body = append(body, lookClassDef...)
	set1Off := lookOff + len(lookClassDef)
	body = append(body, set1...)

	sub := make([]byte, hdr+len(body))
	put16(sub[0:2], 2)
	put16(sub[2:4], uint16(cOff))
	put16(sub[4:6], uint16(backOff))
	put16(sub[6:8], uint16(inOff))
	put16(sub[8:10], uint16(lookOff))
	put16(sub[10:12], uint16(setCount))
	put16(sub[12:14], 0)               // class 0 set offset = 0 (none)
	put16(sub[14:16], uint16(set1Off)) // class 1 set offset
	copy(sub[hdr:], body)
	return sub
}

// buildChainContextFormat3Subtable constructs a Format 3 subtable:
//
//	backtrack coverage = {4, 5, 6}  (a 3-glyph set)
//	input coverage     = {10}       (a 1-glyph set, single trigger)
//	input coverage 2   = {20}
//	lookahead coverage = {30}
//	action: seqIndex=0, lookupIndex=0
func buildChainContextFormat3Subtable() []byte {
	// Construct each coverage format 1 independently.
	back := make([]byte, 4+3*2)
	put16(back[0:2], 1)
	put16(back[2:4], 3)
	put16(back[4:6], 4)
	put16(back[6:8], 5)
	put16(back[8:10], 6)

	in1 := make([]byte, 6)
	put16(in1[0:2], 1)
	put16(in1[2:4], 1)
	put16(in1[4:6], 10)

	in2 := make([]byte, 6)
	put16(in2[0:2], 1)
	put16(in2[2:4], 1)
	put16(in2[4:6], 20)

	look := make([]byte, 6)
	put16(look[0:2], 1)
	put16(look[2:4], 1)
	put16(look[4:6], 30)

	// Header fields before the coverage blobs:
	// format(2)=3, backCount(2)=1, backCovOff[1](2),
	// inputCount(2)=2, inputCovOff[2](2 each),
	// lookCount(2)=1, lookCovOff[1](2),
	// substCount(2)=1, substLookupRecord(4).
	hdr := 2 + 2 + 1*2 + 2 + 2*2 + 2 + 1*2 + 2 + 4

	bodyOff := hdr
	body := make([]byte, 0)
	backOff := bodyOff
	body = append(body, back...)
	in1Off := bodyOff + len(back)
	body = append(body, in1...)
	in2Off := in1Off + len(in1)
	body = append(body, in2...)
	lookOff := in2Off + len(in2)
	body = append(body, look...)

	sub := make([]byte, hdr+len(body))
	p := 0
	put16(sub[p:p+2], 3)
	p += 2
	put16(sub[p:p+2], 1)
	p += 2
	put16(sub[p:p+2], uint16(backOff))
	p += 2
	put16(sub[p:p+2], 2)
	p += 2
	put16(sub[p:p+2], uint16(in1Off))
	p += 2
	put16(sub[p:p+2], uint16(in2Off))
	p += 2
	put16(sub[p:p+2], 1)
	p += 2
	put16(sub[p:p+2], uint16(lookOff))
	p += 2
	put16(sub[p:p+2], 1)
	p += 2
	put16(sub[p:p+2], 0)
	p += 2
	put16(sub[p:p+2], 0)
	copy(sub[hdr:], body)
	return sub
}

// buildChainContextGSUBSeqOne is a variant that builds a Format 1 chain
// context rule whose action targets sequenceIndex=1 (the second input
// glyph). The underlying Single lookup is [20->77].
func buildChainContextGSUBSeqOne() []byte {
	// Single lookup: format 2, coverage=[20], substitute=[77].
	cov := make([]byte, 6)
	put16(cov[0:2], 1)
	put16(cov[2:4], 1)
	put16(cov[4:6], 20)
	singleSub := make([]byte, 6)
	put16(singleSub[0:2], 2)
	put16(singleSub[2:4], 0) // placeholder, patched below
	put16(singleSub[4:6], 1) // substCount
	singleSub = append(singleSub, 0, 77)
	// Coverage lives after the substitute array: 6-byte header + 2 bytes
	// per substitute = offset 8.
	put16(singleSub[2:4], uint16(len(singleSub)))
	singleSub = append(singleSub, cov...)

	// Rule: back=[5], input=[10,20] (on-disk omits first), look=[30],
	// action seqIndex=1 targeting lookup 0.
	rule := make([]byte, 0)
	rule = append(rule, 0, 1, 0, 5)
	rule = append(rule, 0, 2, 0, 20)
	rule = append(rule, 0, 1, 0, 30)
	rule = append(rule, 0, 1, 0, 1, 0, 0)

	ruleSet := make([]byte, 4+len(rule))
	put16(ruleSet[0:2], 1)
	put16(ruleSet[2:4], 4)
	copy(ruleSet[4:], rule)

	triggerCov := make([]byte, 6)
	put16(triggerCov[0:2], 1)
	put16(triggerCov[2:4], 1)
	put16(triggerCov[4:6], 10)

	hdr := 8
	chainSub := make([]byte, hdr+len(triggerCov)+len(ruleSet))
	put16(chainSub[0:2], 1)
	put16(chainSub[2:4], uint16(hdr))
	put16(chainSub[4:6], 1)
	put16(chainSub[6:8], uint16(hdr+len(triggerCov)))
	copy(chainSub[hdr:], triggerCov)
	copy(chainSub[hdr+len(triggerCov):], ruleSet)

	return assembleTwoLookupGSUB(singleSub, chainSub, false)
}

// assembleTwoLookupGSUB wires together a script/feature/lookup list for
// a GSUB with two lookups: lookup 0 is a LookupType 1 single subtable,
// lookup 1 is a LookupType 6 chain context subtable. Only the chain
// lookup is referenced by the calt feature. If extension is true the
// chain lookup is tagged as LookupType 7 (the subtable is assumed to
// already carry its extension wrapper).
func assembleTwoLookupGSUB(singleSub, chainSub []byte, extension bool) []byte {
	// Lookup 0: type=1, flag=0, subCount=1, subOff[0]=8, then singleSub.
	lookup0 := make([]byte, 8+len(singleSub))
	put16(lookup0[0:2], 1)
	put16(lookup0[2:4], 0)
	put16(lookup0[4:6], 1)
	put16(lookup0[6:8], 8)
	copy(lookup0[8:], singleSub)

	// Lookup 1: type=6 (or 7 if extension), flag=0, subCount=1,
	// subOff[0]=8, then chainSub.
	lookupType := uint16(6)
	if extension {
		lookupType = 7
	}
	lookup1 := make([]byte, 8+len(chainSub))
	put16(lookup1[0:2], lookupType)
	put16(lookup1[2:4], 0)
	put16(lookup1[4:6], 1)
	put16(lookup1[6:8], 8)
	copy(lookup1[8:], chainSub)

	// LookupList: lookupCount(2)=2, lookupOffs[2](2 each), then lookups.
	lookupListHdr := 2 + 2*2
	lookup0Off := lookupListHdr
	lookup1Off := lookup0Off + len(lookup0)
	lookupList := make([]byte, lookup1Off+len(lookup1))
	put16(lookupList[0:2], 2)
	put16(lookupList[2:4], uint16(lookup0Off))
	put16(lookupList[4:6], uint16(lookup1Off))
	copy(lookupList[lookup0Off:], lookup0)
	copy(lookupList[lookup1Off:], lookup1)

	// Feature references lookup index 1 only.
	feature := make([]byte, 6)
	put16(feature[0:2], 0)
	put16(feature[2:4], 1)
	put16(feature[4:6], 1) // lookupListIndex = 1 (the chain lookup)

	featListHdr := 8
	featList := make([]byte, featListHdr+len(feature))
	put16(featList[0:2], 1)
	copy(featList[2:6], []byte("calt"))
	put16(featList[6:8], uint16(featListHdr))
	copy(featList[featListHdr:], feature)

	// LangSys: lookupOrder=0, reqFeature=0xFFFF, count=1, indices=[0].
	langSys := make([]byte, 8)
	put16(langSys[0:2], 0)
	put16(langSys[2:4], 0xFFFF)
	put16(langSys[4:6], 1)
	put16(langSys[6:8], 0)

	scriptHdr := 4
	script := make([]byte, scriptHdr+len(langSys))
	put16(script[0:2], uint16(scriptHdr))
	put16(script[2:4], 0)
	copy(script[scriptHdr:], langSys)

	scriptListHdr := 8
	scriptList := make([]byte, scriptListHdr+len(script))
	put16(scriptList[0:2], 1)
	copy(scriptList[2:6], []byte("latn"))
	put16(scriptList[6:8], uint16(scriptListHdr))
	copy(scriptList[scriptListHdr:], script)

	header := make([]byte, 10)
	scriptListOff := len(header)
	featListOff := scriptListOff + len(scriptList)
	lookupListOff := featListOff + len(featList)
	put16(header[0:2], 1)
	put16(header[2:4], 0)
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
