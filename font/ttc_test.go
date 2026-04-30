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

	"golang.org/x/image/font/sfnt"
)

// TestExtractTTCFontFromSyntheticTTC builds a TTC at test time by wrapping a
// system TTF in TTC headers, extracts face 0, and verifies the result parses
// under sfnt.Parse. This pins regression coverage on every host (no silent
// skip on Linux CI without Noto-CJK), independent of which TTC fonts the
// host happens to ship.
func TestExtractTTCFontFromSyntheticTTC(t *testing.T) {
	ttfBytes := loadAnySystemTTF(t)
	ttc := buildSyntheticTTC(t, ttfBytes, 1, 0x00010000)

	out, err := extractTTCFont(ttc, 0)
	if err != nil {
		t.Fatalf("extractTTCFont: %v", err)
	}
	if bytes.Equal(out[:4], []byte("ttcf")) {
		t.Fatal("extracted output still has ttcf magic")
	}
	if _, err := sfnt.Parse(out); err != nil {
		t.Fatalf("sfnt.Parse on extracted single font: %v", err)
	}
}

// TestExtractTTCFontVersion2Header verifies that a v2 TTC (which adds 12
// trailing DSIG bytes after the offset-table array) extracts identically to
// v1 — the version field is not consulted, and the DSIG fields sit after
// the data the extractor reads. Locks the current behavior so a future
// "skip past DSIG block" change doesn't silently drift.
func TestExtractTTCFontVersion2Header(t *testing.T) {
	ttfBytes := loadAnySystemTTF(t)

	v1 := buildSyntheticTTC(t, ttfBytes, 1, 0x00010000)
	v2 := buildSyntheticTTC(t, ttfBytes, 1, 0x00020000)

	v1out, err := extractTTCFont(v1, 0)
	if err != nil {
		t.Fatalf("v1 extract: %v", err)
	}
	v2out, err := extractTTCFont(v2, 0)
	if err != nil {
		t.Fatalf("v2 extract: %v", err)
	}
	if !bytes.Equal(v1out, v2out) {
		t.Errorf("v1 and v2 TTCs wrapping identical TTF produced different output (len v1=%d v2=%d)", len(v1out), len(v2out))
	}
}

// TestExtractTTCFontMultiFaceIndex verifies the offset arithmetic for a
// non-zero face index. Builds a 2-face TTC where each face wraps a copy of
// the same TTF at a distinct absolute offset, then extracts both faces and
// asserts they parse independently with consistent metrics.
func TestExtractTTCFontMultiFaceIndex(t *testing.T) {
	ttfBytes := loadAnySystemTTF(t)
	ttc := buildSyntheticTTCDistinctBodies(t, ttfBytes, 2, 0x00010000)

	face0, err := extractTTCFont(ttc, 0)
	if err != nil {
		t.Fatalf("face 0 extract: %v", err)
	}
	face1, err := extractTTCFont(ttc, 1)
	if err != nil {
		t.Fatalf("face 1 extract: %v", err)
	}
	f0, err := sfnt.Parse(face0)
	if err != nil {
		t.Fatalf("sfnt.Parse face 0: %v", err)
	}
	f1, err := sfnt.Parse(face1)
	if err != nil {
		t.Fatalf("sfnt.Parse face 1: %v", err)
	}
	if f0.UnitsPerEm() != f1.UnitsPerEm() {
		t.Errorf("UnitsPerEm differ across identical faces: f0=%d f1=%d", f0.UnitsPerEm(), f1.UnitsPerEm())
	}
	// Out-of-range index after 2 faces.
	if _, err := extractTTCFont(ttc, 2); !errors.Is(err, ErrCorruptTable) {
		t.Errorf("face index 2 on 2-face TTC: err = %v, want errors.Is ErrCorruptTable", err)
	}
}

// TestExtractTTCFontHandlesHostileUint32Offsets confirms that hostile uint32
// values are rejected with a wrapped sentinel error rather than panicking
// from slice-out-of-bounds. On 32-bit hosts (where Go's `int` is 32 bits),
// casting a uint32 ≥ 0x80000000 to int produces a negative value, so a
// pre-fix bounds check `int(off)+12 > len(data)` evaluates true on
// signed-comparison and skips the truncation guard, then panics on the
// later `data[off:off+length]` slice. The uint64-arithmetic fix is sound
// on both 32-bit and 64-bit, but to keep this test arch-independent we
// also assert that a uint32-MAX-class field never produces a panic, only
// an error. ARCHITECTURE.md §error-handling: "never panic on malformed
// PDF input."
func TestExtractTTCFontHandlesHostileUint32Offsets(t *testing.T) {
	cases := []struct {
		name string
		mut  func(buf []byte)
	}{
		{
			name: "font offset MaxUint32",
			mut: func(buf []byte) {
				binary.BigEndian.PutUint32(buf[12:16], 0xFFFFFFFF)
			},
		},
		{
			name: "font offset high-bit set",
			mut: func(buf []byte) {
				binary.BigEndian.PutUint32(buf[12:16], 0x80000000)
			},
		},
		{
			name: "numFonts MaxUint32",
			mut: func(buf []byte) {
				binary.BigEndian.PutUint32(buf[8:12], 0xFFFFFFFF)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := make([]byte, 16)
			copy(buf[0:4], "ttcf")
			binary.BigEndian.PutUint32(buf[4:8], 0x00010000)
			binary.BigEndian.PutUint32(buf[8:12], 1)
			binary.BigEndian.PutUint32(buf[12:16], 0)
			tc.mut(buf)

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("extractTTCFont panicked on hostile input: %v", r)
				}
			}()
			if _, err := extractTTCFont(buf, 0); err == nil {
				t.Error("expected error on hostile input, got nil")
			}
		})
	}
}

// TestExtractTTCFontFromRealSystemTTC opportunistically validates against a
// real system TTC if one is present and parseable. Unlike the synthetic
// test, this catches integration regressions (real-world DSIG, real-world
// shared tables) but skips on hosts without a parseable TTC.
func TestExtractTTCFontFromRealSystemTTC(t *testing.T) {
	candidates := systemTTCCandidates()
	if len(candidates) == 0 {
		t.Skip("no system .ttc font found on this host")
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			t.Logf("%s: read err: %v", p, err)
			continue
		}
		out, err := extractTTCFont(data, 0)
		if err != nil {
			t.Errorf("%s: extractTTCFont: %v", p, err)
			continue
		}
		if len(out) < 12 || bytes.Equal(out[:4], []byte("ttcf")) {
			t.Errorf("%s: extracted output is not a single-font sfnt (len=%d)", p, len(out))
			continue
		}
		if _, err := sfnt.Parse(out); err != nil {
			t.Logf("%s: sfnt.Parse: %v (orthogonal sfnt limit, not a TTC issue)", p, err)
			continue
		}
		return
	}
	t.Fatal("none of the candidate TTCs produced a sfnt-parseable single-font extract")
}

// TestParseFontAcceptsTTC verifies that the ttcf branch in ParseFont is
// wired up — without the fix, ParseFont routes TTC bytes to sfnt.Parse,
// which returns "invalid single font (data is a font collection)".
func TestParseFontAcceptsTTC(t *testing.T) {
	ttfBytes := loadAnySystemTTF(t)
	ttc := buildSyntheticTTC(t, ttfBytes, 1, 0x00010000)

	face, err := ParseFont(ttc)
	if err != nil {
		t.Fatalf("ParseFont on TTC: %v", err)
	}
	if face.UnitsPerEm() <= 0 {
		t.Errorf("UnitsPerEm = %d, want > 0", face.UnitsPerEm())
	}
	if face.PostScriptName() == "" {
		t.Error("PostScriptName is empty for extracted TTC face")
	}
	if rd := face.RawData(); len(rd) == 0 || bytes.Equal(rd[:4], []byte("ttcf")) {
		t.Error("RawData should be a single-font TTF, not the original TTC")
	}
}

// TestLoadFontAcceptsTTC verifies the full LoadFont -> ParseFont -> ParseTTF
// chain works for a TTC file on disk.
func TestLoadFontAcceptsTTC(t *testing.T) {
	ttfBytes := loadAnySystemTTF(t)
	ttc := buildSyntheticTTC(t, ttfBytes, 1, 0x00010000)

	dir := t.TempDir()
	path := dir + "/synthetic.ttc"
	if err := os.WriteFile(path, ttc, 0o600); err != nil {
		t.Fatal(err)
	}
	face, err := LoadFont(path)
	if err != nil {
		t.Fatalf("LoadFont(%q): %v", path, err)
	}
	if face == nil {
		t.Fatal("expected non-nil face")
	}
}

// TestExtractTTCFontRejectsBadInput exercises the validation paths.
func TestExtractTTCFontRejectsBadInput(t *testing.T) {
	cases := []struct {
		name    string
		data    []byte
		index   int
		wantErr error
	}{
		{"too short", []byte{0x74, 0x74, 0x63, 0x66, 0, 1, 0, 0}, 0, ErrTruncated},
		{"wrong magic", []byte("\x00\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"), 0, ErrUnknownFormat},
		{"index out of range", buildEmptyTTC(1), 5, ErrCorruptTable},
		{"negative index", buildEmptyTTC(1), -1, ErrCorruptTable},
		{"empty collection", buildEmptyTTC(0), 0, ErrCorruptTable},
		{"font offset truncated", offsetTableTruncatedTTC(t), 0, ErrTruncated},
		{"directory truncated", directoryTruncatedTTC(t), 0, ErrTruncated},
		{"table beyond data", tableBeyondDataTTC(t), 0, ErrTruncated},
		{"length wraps uint32", lengthWrapsTTC(t), 0, ErrTruncated},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := extractTTCFont(tc.data, tc.index)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, tc.wantErr) {
				t.Errorf("err = %v, want errors.Is %v", err, tc.wantErr)
			}
		})
	}
}

// buildEmptyTTC returns a minimal-but-valid TTC header with numFonts entries
// pointing at offset 0 (which is invalid for a real font but exercises the
// header-parsing code paths). Only the header layout is needed to test the
// pre-payload validation in extractTTCFont.
func buildEmptyTTC(numFonts int) []byte {
	buf := make([]byte, 12+numFonts*4)
	copy(buf[0:4], "ttcf")
	binary.BigEndian.PutUint32(buf[4:8], 0x00010000)
	binary.BigEndian.PutUint32(buf[8:12], uint32(numFonts))
	return buf
}

// buildSyntheticTTC wraps a single TTF in a TTC envelope. faces ≥ 1 share
// one TTF body — they all index the same offset table — which is enough
// for tests that only assert face 0 round-trips correctly. version is
// 0x00010000 (v1) or 0x00020000 (v2); the latter appends 12 zero bytes of
// DSIG fields after the font-offset array, matching the v2 spec layout.
func buildSyntheticTTC(t *testing.T, ttfBytes []byte, faces int, version uint32) []byte {
	t.Helper()
	if faces < 1 {
		t.Fatal("faces must be >= 1")
	}
	headerSize := 12 + faces*4
	if version == 0x00020000 {
		headerSize += 12 // ulDsigTag + ulDsigLength + ulDsigOffset
	}
	out := make([]byte, headerSize+len(ttfBytes))
	copy(out[0:4], "ttcf")
	binary.BigEndian.PutUint32(out[4:8], version)
	binary.BigEndian.PutUint32(out[8:12], uint32(faces))
	for i := range faces {
		binary.BigEndian.PutUint32(out[12+i*4:], uint32(headerSize))
	}
	copy(out[headerSize:], ttfBytes)

	// Rewrite the embedded TTF's table directory offsets so they're
	// absolute within the TTC (per the TTC spec: directory offsets are
	// absolute file offsets, not TTF-relative).
	if len(out) < headerSize+12 {
		t.Fatalf("ttf bytes too short to wrap (len=%d)", len(ttfBytes))
	}
	numTables := int(binary.BigEndian.Uint16(out[headerSize+4:]))
	for i := range numTables {
		entryBase := headerSize + 12 + i*16
		oldOff := binary.BigEndian.Uint32(out[entryBase+8:])
		binary.BigEndian.PutUint32(out[entryBase+8:], oldOff+uint32(headerSize))
	}
	return out
}

// buildSyntheticTTCDistinctBodies wraps `faces` independent copies of ttfBytes
// in a TTC envelope. Unlike buildSyntheticTTC (which has every face share a
// single body), this places each TTF at a distinct absolute offset so that
// extracting face N exercises the non-zero-index arithmetic.
func buildSyntheticTTCDistinctBodies(t *testing.T, ttfBytes []byte, faces int, version uint32) []byte {
	t.Helper()
	if faces < 1 {
		t.Fatal("faces must be >= 1")
	}
	headerSize := 12 + faces*4
	if version == 0x00020000 {
		headerSize += 12
	}
	out := make([]byte, headerSize+faces*len(ttfBytes))
	copy(out[0:4], "ttcf")
	binary.BigEndian.PutUint32(out[4:8], version)
	binary.BigEndian.PutUint32(out[8:12], uint32(faces))
	for i := range faces {
		bodyOff := headerSize + i*len(ttfBytes)
		binary.BigEndian.PutUint32(out[12+i*4:], uint32(bodyOff))
		copy(out[bodyOff:], ttfBytes)
		// Rewrite this face's directory entry offsets to be absolute.
		numTables := int(binary.BigEndian.Uint16(out[bodyOff+4:]))
		for j := range numTables {
			entryBase := bodyOff + 12 + j*16
			oldOff := binary.BigEndian.Uint32(out[entryBase+8:])
			binary.BigEndian.PutUint32(out[entryBase+8:], oldOff+uint32(bodyOff))
		}
	}
	return out
}

// offsetTableTruncatedTTC declares numFonts=4 but provides space for only 1.
func offsetTableTruncatedTTC(t *testing.T) []byte {
	t.Helper()
	buf := make([]byte, 16)
	copy(buf[0:4], "ttcf")
	binary.BigEndian.PutUint32(buf[4:8], 0x00010000)
	binary.BigEndian.PutUint32(buf[8:12], 4)          // numFonts says 4
	binary.BigEndian.PutUint32(buf[12:16], 16)        // only one offset stored
	return buf
}

// directoryTruncatedTTC declares numTables=2 but the directory is cut off.
func directoryTruncatedTTC(t *testing.T) []byte {
	t.Helper()
	const fontOff = 16
	const numTables = 2
	buf := make([]byte, fontOff+12+numTables*16-4) // -4 to truncate last entry
	copy(buf[0:4], "ttcf")
	binary.BigEndian.PutUint32(buf[4:8], 0x00010000)
	binary.BigEndian.PutUint32(buf[8:12], 1)
	binary.BigEndian.PutUint32(buf[12:16], fontOff)
	binary.BigEndian.PutUint32(buf[fontOff:fontOff+4], 0x00010000) // sfntVersion
	binary.BigEndian.PutUint16(buf[fontOff+4:fontOff+6], numTables)
	return buf
}

// tableBeyondDataTTC has a complete header + directory but the directory
// declares a table whose end exceeds the buffer.
func tableBeyondDataTTC(t *testing.T) []byte {
	t.Helper()
	const fontOff = 16
	const numTables = 1
	buf := make([]byte, fontOff+12+numTables*16+8)
	copy(buf[0:4], "ttcf")
	binary.BigEndian.PutUint32(buf[4:8], 0x00010000)
	binary.BigEndian.PutUint32(buf[8:12], 1)
	binary.BigEndian.PutUint32(buf[12:16], fontOff)
	binary.BigEndian.PutUint32(buf[fontOff:fontOff+4], 0x00010000)
	binary.BigEndian.PutUint16(buf[fontOff+4:fontOff+6], numTables)
	entry := fontOff + 12
	copy(buf[entry:entry+4], "head")
	binary.BigEndian.PutUint32(buf[entry+4:], 0)
	binary.BigEndian.PutUint32(buf[entry+8:], uint32(fontOff+12+16)) // table starts here
	binary.BigEndian.PutUint32(buf[entry+12:], 1024)                  // way past buffer end
	return buf
}

// lengthWrapsTTC declares a uint32 length close to MaxUint32 such that
// srcOff+length would wrap on int32 but must be detected via uint64.
func lengthWrapsTTC(t *testing.T) []byte {
	t.Helper()
	const fontOff = 16
	const numTables = 1
	buf := make([]byte, fontOff+12+numTables*16)
	copy(buf[0:4], "ttcf")
	binary.BigEndian.PutUint32(buf[4:8], 0x00010000)
	binary.BigEndian.PutUint32(buf[8:12], 1)
	binary.BigEndian.PutUint32(buf[12:16], fontOff)
	binary.BigEndian.PutUint32(buf[fontOff:fontOff+4], 0x00010000)
	binary.BigEndian.PutUint16(buf[fontOff+4:fontOff+6], numTables)
	entry := fontOff + 12
	copy(buf[entry:entry+4], "head")
	binary.BigEndian.PutUint32(buf[entry+4:], 0)
	binary.BigEndian.PutUint32(buf[entry+8:], 0)
	binary.BigEndian.PutUint32(buf[entry+12:], 0xFFFFFF00) // huge length
	return buf
}

// loadAnySystemTTF locates any TTF on the host (Latin or CJK) — the
// universal candidate set is broader than for TTCs, so this rarely skips
// even on minimal Linux CIs.
func loadAnySystemTTF(t *testing.T) []byte {
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
	t.Skip("no system TTF found to build a synthetic TTC")
	return nil
}

// systemTTCCandidates returns TTC paths that exist on the host. The list is
// stat-filtered, not parse-filtered, so callers can distinguish "no TTC
// available" (skip) from "TTC available but parse failed" (real failure).
// Very large CJK fonts (STHeiti, some Noto CJK builds) exceed sfnt's
// hardcoded maxCmapSegments — orthogonal to TTC dispatch. The candidate
// list is ordered to prefer fonts known to parse cleanly.
func systemTTCCandidates() []string {
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/System/Library/Fonts/Helvetica.ttc",
			"/System/Library/Fonts/Courier.ttc",
			"/System/Library/Fonts/Hiragino Sans GB.ttc",
		}
	case "linux":
		candidates = []string{
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		}
	case "windows":
		candidates = []string{
			`C:\Windows\Fonts\cambria.ttc`,
			`C:\Windows\Fonts\msyh.ttc`,
			`C:\Windows\Fonts\msgothic.ttc`,
		}
	}
	out := candidates[:0]
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}
