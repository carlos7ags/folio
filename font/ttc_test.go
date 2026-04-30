// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"bytes"
	"errors"
	"os"
	"runtime"
	"testing"

	"golang.org/x/image/font/sfnt"
)

// TestExtractTTCFontRoundTripsThroughSfnt verifies that a real system TTC
// can be extracted into a standalone TTF and parsed by golang.org/x/image/sfnt
// without the "invalid single font (data is a font collection)" error that
// blocks issue #227.
func TestExtractTTCFontRoundTripsThroughSfnt(t *testing.T) {
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
			// Some TTCs fail sfnt's orthogonal cmap-segment limit; that
			// is not a TTC-dispatch failure. Log and continue so the test
			// still validates whichever candidates parse cleanly.
			t.Logf("%s: sfnt.Parse: %v (orthogonal sfnt limit, not a TTC issue)", p, err)
			continue
		}
		// At least one candidate parsed cleanly; the dispatch works.
		return
	}
	t.Fatal("none of the candidate TTCs produced a sfnt-parseable single-font extract")
}

// TestParseFontAcceptsTTC verifies that the ttcf branch in ParseFont is
// wired up — without the fix, ParseFont routes TTC bytes to sfnt.Parse,
// which returns "invalid single font (data is a font collection)".
func TestParseFontAcceptsTTC(t *testing.T) {
	candidates := systemTTCCandidates()
	if len(candidates) == 0 {
		t.Skip("no system .ttc font found")
	}
	var lastErr error
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			lastErr = err
			continue
		}
		face, err := ParseFont(data)
		if err != nil {
			lastErr = err
			// Could be the orthogonal cmap limit; try the next.
			t.Logf("%s: ParseFont: %v", p, err)
			continue
		}
		if face.UnitsPerEm() <= 0 {
			t.Errorf("%s: UnitsPerEm = %d, want > 0", p, face.UnitsPerEm())
		}
		if face.PostScriptName() == "" {
			t.Errorf("%s: PostScriptName is empty", p)
		}
		if rd := face.RawData(); len(rd) == 0 || bytes.Equal(rd[:4], []byte("ttcf")) {
			t.Errorf("%s: RawData should be a single-font TTF, not the original TTC", p)
		}
		return
	}
	t.Fatalf("ParseFont rejected every system TTC; last error: %v", lastErr)
}

// TestLoadFontAcceptsTTC verifies the full LoadFont -> ParseFont -> ParseTTF
// chain works for an absolute system TTC path.
func TestLoadFontAcceptsTTC(t *testing.T) {
	candidates := systemTTCCandidates()
	if len(candidates) == 0 {
		t.Skip("no system .ttc font found")
	}
	var lastErr error
	for _, p := range candidates {
		if face, err := LoadFont(p); err == nil && face != nil {
			return
		} else {
			lastErr = err
		}
	}
	t.Fatalf("LoadFont rejected every system TTC; last error: %v", lastErr)
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
	buf[4], buf[5], buf[6], buf[7] = 0, 1, 0, 0 // version 1.0
	buf[8], buf[9], buf[10], buf[11] = byte(numFonts>>24), byte(numFonts>>16), byte(numFonts>>8), byte(numFonts)
	return buf
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
