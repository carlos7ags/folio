// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/carlos7ags/folio/document"
	folioFont "github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/html"
)

// TestCJKExampleProducesValidPDF runs the example's HTML build → convert
// → save pipeline against a synthetic TTC fixture and verifies the
// resulting bytes start with the PDF header. This is the test that
// would have caught issue #227 — the example documented working
// `msyh.ttc` and `NotoSansCJK-Regular.ttc` paths on Windows / Linux,
// but neither was ever exercised end-to-end because nothing in CI
// compiled or ran the example. The reporter became the de-facto CI.
//
// The fixture is built at test time from any system TTF (universal
// across dev/CI hosts that have at least one TrueType font), wrapped
// in a TTC envelope so the TTC dispatch (font.ParseFont's `ttcMagic`
// branch added in #235) is exercised on the path the example takes.
// Skips when no TTF can be located rather than masking a genuine
// regression with a silent green.
func TestCJKExampleProducesValidPDF(t *testing.T) {
	ttfBytes := loadAnyTTF(t)
	ttcBytes := wrapTTFAsTTC(t, ttfBytes)
	dir := t.TempDir()
	ttcPath := filepath.Join(dir, "synthetic.ttc")
	if err := os.WriteFile(ttcPath, ttcBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	// Pre-parse the synthetic TTC so the test knows the PostScript name
	// the example should embed. Without this, a regression where the
	// example silently falls back to a system font (which also embeds
	// /BaseFont) would slip past a /BaseFont-presence-only assertion.
	wantFace, err := folioFont.LoadFont(ttcPath)
	if err != nil {
		t.Fatalf("synthetic TTC failed to parse: %v", err)
	}
	wantPS := wantFace.PostScriptName()
	if wantPS == "" {
		t.Fatal("synthetic TTC has empty PostScriptName; cannot pin embedding")
	}

	htmlStr := buildHTML(ttcPath)
	result, err := html.ConvertFull(htmlStr, &html.Options{AllowAbsolutePaths: true})
	if err != nil {
		t.Fatalf("html.ConvertFull: %v", err)
	}
	if len(result.Elements) == 0 {
		t.Fatal("expected paragraph elements from the example HTML")
	}

	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Folio CJK Text Layout"
	doc.Info.Author = "Folio"
	for _, e := range result.Elements {
		doc.Add(e)
	}

	pdfPath := filepath.Join(dir, "cjk.pdf")
	if err := doc.Save(pdfPath); err != nil {
		t.Fatalf("doc.Save: %v", err)
	}

	pdfBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(pdfBytes, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdfBytes[:min(8, len(pdfBytes))])
	}
	if len(pdfBytes) < 1024 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected real content", len(pdfBytes))
	}
	// The synthetic TTC's PostScript name must appear in the PDF — the
	// font is embedded with a 6-character subset prefix, so we look for
	// "+<PostScriptName>". Asserting on the requested font specifically
	// (not just /BaseFont presence) catches a regression where the
	// example silently falls back to a system font that also embeds.
	//
	// Note: font/embed.go runs the PostScript name through
	// sanitizePSName before the subset prefix is glued on. For all
	// candidate TTFs in loadAnyTTF (ArialMT, DejaVuSans, etc.) the
	// sanitizer is a no-op so the raw PostScriptName matches what
	// appears in the PDF; if a future candidate is added whose name
	// contains characters the sanitizer rewrites, this assertion will
	// need to mirror that transformation.
	needle := []byte("+" + wantPS)
	if !bytes.Contains(pdfBytes, needle) {
		t.Errorf("output PDF does not embed the requested font: looking for %q (subset of synthetic TTC's PostScriptName %q)", needle, wantPS)
	}
}

// TestCJKExampleQuotesFontURL is a unit-level guard for the CSS src
// format. The example's @font-face emits url('%s'); a regression that
// drops the quotes (url(%s)) can be subtly broken because some CSS
// parsers tolerate unquoted paths but the CJK example's tempdir paths
// often contain characters that strict parsing requires to be quoted.
// Without this test, a silent regression where the @font-face src is
// malformed could still produce a valid-looking PDF via the system
// fallback chain — the +<PostScriptName> assertion above catches the
// most likely cases, but not all.
//
// Asserting on the format directly is much cheaper than a full PDF
// round-trip and surfaces the regression at its actual source.
func TestCJKExampleQuotesFontURL(t *testing.T) {
	html := buildHTML("/some/path/font.ttc")
	if !bytes.Contains([]byte(html), []byte(`url('/some/path/font.ttc')`)) {
		t.Errorf("buildHTML did not emit single-quoted url('...') around the font path; CSS may misparse paths with spaces, parentheses, or commas. Generated HTML head excerpt: %q", html[:min(500, len(html))])
	}
}

// TestCJKExampleFindCJKFontReturnsExistingPath verifies that when
// findCJKFont returns non-empty, the path actually exists on disk.
// A drift where a candidate is renamed or removed (macOS major
// version, Linux distribution change) would be caught here rather
// than at user runtime.
func TestCJKExampleFindCJKFontReturnsExistingPath(t *testing.T) {
	path := findCJKFont()
	if path == "" {
		t.Skip("no CJK font found on this host; the empty-result branch is the documented fallback")
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("findCJKFont returned %q but it does not exist: %v", path, err)
	}
}

// loadAnyTTF locates any TTF on the host. The candidate list mirrors
// the universal TTF set used by the font package's test fixtures so
// the example test runs on the same hosts the unit tests do.
func loadAnyTTF(t *testing.T) []byte {
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
	t.Skip("no system TTF found; cannot build a synthetic TTC for the example test")
	return nil
}

// wrapTTFAsTTC builds a single-face TTC envelope around ttfBytes,
// rewriting the embedded TTF's table directory offsets to be absolute
// within the TTC. Mirrors font.buildSyntheticTTC (test-package-private)
// so the example test can exercise the TTC dispatch end-to-end without
// depending on a system .ttc being installed.
func wrapTTFAsTTC(t *testing.T, ttfBytes []byte) []byte {
	t.Helper()
	const headerSize = 12 + 4 // header + one uint32 face offset
	out := make([]byte, headerSize+len(ttfBytes))
	copy(out[0:4], "ttcf")
	binary.BigEndian.PutUint32(out[4:8], 0x00010000) // v1
	binary.BigEndian.PutUint32(out[8:12], 1)         // numFonts
	binary.BigEndian.PutUint32(out[12:16], headerSize)
	copy(out[headerSize:], ttfBytes)

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
