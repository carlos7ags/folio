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
