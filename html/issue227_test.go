// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"os"
	"runtime"
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// TestFontFaceAbsolutePathLoadsTTC verifies that @font-face with an absolute
// filesystem path to a TrueType Collection (.ttc) is loaded from disk even
// when Options.BaseFS is nil. Before the fix, absolute paths were resolved
// only through BaseFS, so the @font-face load silently failed and CJK text
// fell through to the system fallback (which can be wrong, missing, or
// produce different glyphs than the user requested).
//
// Reproduces the Windows/Ubuntu failure mode in issue #227.
func TestFontFaceAbsolutePathLoadsTTC(t *testing.T) {
	ttcPath := systemParseableTTCPath()
	if ttcPath == "" {
		t.Skip("no system .ttc font that parses cleanly under sfnt found")
	}
	want := loadedPostScriptName(t, ttcPath)

	src := `<html><head><style>
		@font-face {
			font-family: 'SystemTTC';
			src: url('` + ttcPath + `');
		}
		p { font-family: 'SystemTTC'; font-size: 12pt; }
	</style></head><body>
		<p>Hello</p>
	</body></html>`

	got := embeddedPostScriptName(t, src)
	if got != want {
		t.Errorf("embedded font PostScriptName = %q, want %q (the @font-face TTC was not loaded; renderer used a fallback or standard font)", got, want)
	}
}

// TestFontFaceAbsolutePathLoadsTTF mirrors the TTC test for a plain .ttf so
// the absolute-path branch is verified independently of TTC handling.
func TestFontFaceAbsolutePathLoadsTTF(t *testing.T) {
	ttfPath := systemTTFPath()
	if ttfPath == "" {
		t.Skip("no system TTF font found")
	}
	want := loadedPostScriptName(t, ttfPath)

	src := `<html><head><style>
		@font-face {
			font-family: 'SystemTTF';
			src: url('` + ttfPath + `');
		}
		p { font-family: 'SystemTTF'; font-size: 12pt; }
	</style></head><body>
		<p>Hello</p>
	</body></html>`

	got := embeddedPostScriptName(t, src)
	if got != want {
		t.Errorf("embedded font PostScriptName = %q, want %q", got, want)
	}
}

// embeddedPostScriptName converts src and returns the PostScript name of the
// embedded font used for the first word of the first paragraph. Fails the
// test if there is no embedded font or no rendered word.
func embeddedPostScriptName(t *testing.T, src string) string {
	t.Helper()
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}
	lines := p.Layout(500)
	if len(lines) == 0 || len(lines[0].Words) == 0 {
		t.Fatal("no words rendered")
	}
	w := lines[0].Words[0]
	if w.Embedded == nil {
		t.Fatal("expected embedded font, got standard font")
	}
	return w.Embedded.Face().PostScriptName()
}

// loadedPostScriptName loads the font directly through font.LoadFont and
// returns its PostScript name — the value the @font-face path should match.
func loadedPostScriptName(t *testing.T, path string) string {
	t.Helper()
	face, err := font.LoadFont(path)
	if err != nil {
		t.Fatalf("font.LoadFont(%q): %v", path, err)
	}
	return face.PostScriptName()
}

// TestFontFaceCJKEndToEnd is the integration test that mirrors the exact
// flow the user reported: HTML with CJK text, @font-face url() pointing at
// a system path, Options nil. Asserts that the rendered word uses the
// requested embedded font (not a fallback) and that its advance is
// non-zero — proving cmap selection actually returned glyphs for the CJK
// codepoints rather than .notdef. Skips when no CJK-capable system font
// parses cleanly (e.g., a minimal Linux CI without Noto-CJK).
func TestFontFaceCJKEndToEnd(t *testing.T) {
	cjkPath := systemCJKFontPath()
	if cjkPath == "" {
		t.Skip("no CJK-capable system font that parses under sfnt found")
	}
	want := loadedPostScriptName(t, cjkPath)

	src := `<html><head><style>
		@font-face {
			font-family: 'CJK';
			src: url('` + cjkPath + `');
		}
		p { font-family: 'CJK'; font-size: 12pt; }
	</style></head><body>
		<p>中华人民共和国</p>
	</body></html>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}
	lines := p.Layout(500)
	if len(lines) == 0 || len(lines[0].Words) == 0 {
		t.Fatal("no words rendered")
	}
	w := lines[0].Words[0]
	if w.Embedded == nil {
		t.Fatal("expected embedded font, got standard font (CJK @font-face was not loaded)")
	}
	if got := w.Embedded.Face().PostScriptName(); got != want {
		t.Errorf("embedded font PostScriptName = %q, want %q", got, want)
	}
	if w.Width <= 0 {
		t.Errorf("rendered CJK word has zero advance (Width=%v); cmap returned .notdef for every codepoint", w.Width)
	}
}

// TestFontFaceWindowsAbsolutePath locks the Windows-path branch of the bug.
// Skips on non-Windows hosts because filepath.IsAbs(`C:\...`) is false on
// Unix; runs when CI eventually executes on Windows and msyh.ttc/cambria.ttc
// is present. Cheap insurance against regressing the OS-path bypass for
// drive-letter paths specifically.
func TestFontFaceWindowsAbsolutePath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skipf("Windows-only test (host is %s)", runtime.GOOS)
	}
	candidates := []string{
		`C:\Windows\Fonts\cambria.ttc`,
		`C:\Windows\Fonts\arial.ttf`,
		`C:\Windows\Fonts\segoeui.ttf`,
	}
	var path string
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if _, err := font.LoadFont(p); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		t.Skip("no parseable Windows system font found")
	}
	want := loadedPostScriptName(t, path)
	src := `<html><head><style>
		@font-face { font-family: 'Win'; src: url('` + path + `'); }
		p { font-family: 'Win'; font-size: 12pt; }
	</style></head><body><p>Hello</p></body></html>`
	got := embeddedPostScriptName(t, src)
	if got != want {
		t.Errorf("embedded font PostScriptName = %q, want %q", got, want)
	}
}

// systemCJKFontPath returns a CJK-capable system font path that parses
// cleanly under sfnt. Falls back across .ttc and .ttf because some hosts
// only ship CJK in collections, others only as standalone files.
func systemCJKFontPath() string {
	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Library/Fonts/Arial Unicode.ttf",
			"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
			"/System/Library/Fonts/Hiragino Sans GB.ttc",
			"/System/Library/Fonts/PingFang.ttc",
		}
	case "linux":
		candidates = []string{
			"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
			"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
		}
	case "windows":
		candidates = []string{
			`C:\Windows\Fonts\msyh.ttc`,
			`C:\Windows\Fonts\msgothic.ttc`,
			`C:\Windows\Fonts\malgun.ttf`,
		}
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if _, err := font.LoadFont(p); err == nil {
			return p
		}
	}
	return ""
}

func systemParseableTTCPath() string {
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
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if _, err := font.LoadFont(p); err == nil {
			return p
		}
	}
	return ""
}
