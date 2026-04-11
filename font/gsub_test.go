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
		table, ok := subs[feat]
		if !ok || len(table) == 0 {
			t.Errorf("feature %q: not found or empty in %s", feat, path)
		}
	}
	t.Logf("GSUB from %s: init=%d medi=%d fina=%d isol=%d",
		path,
		len(subs[GSUBInit]), len(subs[GSUBMedi]),
		len(subs[GSUBFina]), len(subs[GSUBIsol]))
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
	if subs := ParseGSUB(ttf); len(subs) != 0 {
		t.Errorf("expected no substitutions from empty GSUB lists, got %d features", len(subs))
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
