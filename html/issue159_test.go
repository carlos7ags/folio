// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"encoding/base64"
	"os"
	"runtime"
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// TestFontFaceDataURILoadsFont verifies that @font-face with a
// base64-encoded data URI loads the font and uses it for text rendering.
// This is the exact use case from issue #159.
func TestFontFaceDataURILoadsFont(t *testing.T) {
	// Load a real TTF from the system to encode as base64.
	ttfPath := systemTTFPath()
	if ttfPath == "" {
		t.Skip("no system TTF font found for data URI test")
	}
	ttfData, err := os.ReadFile(ttfPath)
	if err != nil {
		t.Fatalf("read %s: %v", ttfPath, err)
	}
	b64 := base64.StdEncoding.EncodeToString(ttfData)

	src := `<html><head><style>
		@font-face {
			font-family: 'TestFont';
			src: url(data:font/truetype;base64,` + b64 + `) format('truetype');
		}
		p { font-family: 'TestFont'; font-size: 12pt; }
	</style></head><body>
		<p>Hello from a data URI font</p>
	</body></html>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}

	// The paragraph should use an embedded font (not a standard font).
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}
	lines := p.Layout(500)
	if len(lines) == 0 || len(lines[0].Words) == 0 {
		t.Fatal("no words")
	}
	w := lines[0].Words[0]
	if w.Embedded == nil {
		t.Error("expected embedded font from data URI, got standard font (data URI was not loaded)")
	}
}

// TestFontFaceDataURIInvalidBase64DoesNotPanic verifies that a malformed
// base64 font data URI is silently skipped without crashing.
func TestFontFaceDataURIInvalidBase64DoesNotPanic(t *testing.T) {
	src := `<html><head><style>
		@font-face {
			font-family: 'BadFont';
			src: url(data:font/truetype;base64,NOT_VALID_BASE64!!!) format('truetype');
		}
		p { font-family: 'BadFont'; font-size: 12pt; }
	</style></head><body>
		<p>Should fall back to standard font</p>
	</body></html>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

// TestFontFaceDataURIOpenType verifies that data:application/x-font-ttf
// media type also works (common variant).
func TestFontFaceDataURIOpenType(t *testing.T) {
	ttfPath := systemTTFPath()
	if ttfPath == "" {
		t.Skip("no system TTF font found")
	}
	ttfData, err := os.ReadFile(ttfPath)
	if err != nil {
		t.Fatalf("read %s: %v", ttfPath, err)
	}
	b64 := base64.StdEncoding.EncodeToString(ttfData)

	src := `<html><head><style>
		@font-face {
			font-family: 'OTFont';
			src: url(data:application/x-font-ttf;base64,` + b64 + `) format('truetype');
		}
		p { font-family: 'OTFont'; }
	</style></head><body>
		<p>OpenType media type</p>
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
		t.Fatal("no words")
	}
	if lines[0].Words[0].Embedded == nil {
		t.Error("expected embedded font from data URI with application/x-font-ttf media type")
	}
}

// TestDecodeFontDataURIUnit tests the decoder directly.
func TestDecodeFontDataURIUnit(t *testing.T) {
	// Invalid: no comma
	if _, err := decodeFontDataURI("data:font/truetype;base64"); err == nil {
		t.Error("expected error for missing comma")
	}
	// Invalid: not base64
	if _, err := decodeFontDataURI("data:font/truetype,raw-data"); err == nil {
		t.Error("expected error for non-base64 data")
	}
	// Invalid: bad base64
	if _, err := decodeFontDataURI("data:font/truetype;base64,!!!"); err == nil {
		t.Error("expected error for invalid base64")
	}
	// Valid base64 but not a font: should fail at ParseTTF
	b64 := base64.StdEncoding.EncodeToString([]byte("not a font"))
	if _, err := decodeFontDataURI("data:font/truetype;base64," + b64); err == nil {
		t.Error("expected error for non-font data")
	}
}

// TestFontFaceDataURIWithWhitespaceInBase64 verifies that line-wrapped
// base64 (common in real templates) is handled correctly.
func TestFontFaceDataURIWithWhitespaceInBase64(t *testing.T) {
	ttfPath := systemTTFPath()
	if ttfPath == "" {
		t.Skip("no system TTF font found")
	}
	ttfData, err := os.ReadFile(ttfPath)
	if err != nil {
		t.Fatal(err)
	}
	// Encode and inject newlines every 76 chars (MIME-style wrapping).
	raw := base64.StdEncoding.EncodeToString(ttfData)
	var wrapped string
	for i := 0; i < len(raw); i += 76 {
		end := i + 76
		if end > len(raw) {
			end = len(raw)
		}
		wrapped += raw[i:end] + "\n"
	}

	src := `<html><head><style>
		@font-face {
			font-family: 'WrappedFont';
			src: url(data:font/truetype;base64,` + wrapped + `) format('truetype');
		}
		p { font-family: 'WrappedFont'; }
	</style></head><body>
		<p>Wrapped base64</p>
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
		t.Fatal("no words")
	}
	if lines[0].Words[0].Embedded == nil {
		t.Error("expected embedded font from line-wrapped base64 data URI")
	}
}

// TestFontFaceLocalPlusURLFallback verifies that src with both local()
// and url() correctly picks up the url() data URI.
func TestFontFaceLocalPlusURLFallback(t *testing.T) {
	ttfPath := systemTTFPath()
	if ttfPath == "" {
		t.Skip("no system TTF font found")
	}
	ttfData, err := os.ReadFile(ttfPath)
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(ttfData)

	src := `<html><head><style>
		@font-face {
			font-family: 'FallbackFont';
			src: local('NonExistentFont'), url(data:font/truetype;base64,` + b64 + `) format('truetype');
		}
		p { font-family: 'FallbackFont'; }
	</style></head><body>
		<p>Local plus URL fallback</p>
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
		t.Fatal("no words")
	}
	if lines[0].Words[0].Embedded == nil {
		t.Error("expected embedded font from url() fallback after local()")
	}
}

// TestSplitDeclarationsCSS exercises the semicolon splitter directly.
func TestSplitDeclarationsCSS(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // expected number of parts
	}{
		{"simple", "color: red; font-size: 12px", 2},
		{"trailing semicolon", "color: red;", 1}, // trailing empty part not emitted
		{"no semicolon", "color: red", 1},
		{"empty", "", 0},
		{"semicolon in url", "src: url(data:font/truetype;base64,AAA)", 1},
		{"semicolon in single quotes", "content: 'hello; world'", 1},
		{"semicolon in double quotes", `content: "hello; world"`, 1},
		{"escaped quote in string", `content: "hello \" ; world"`, 1},
		{"mixed", "color: red; src: url(data:x;base64,Y); font-size: 12px", 3},
		{"nested parens", "width: calc(100% - var(--x; 0)); color: red", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitDeclarationsCSS(tt.input)
			if len(got) != tt.want {
				t.Errorf("splitDeclarationsCSS(%q) = %d parts %v, want %d", tt.input, len(got), got, tt.want)
			}
		})
	}
}

func systemTTFPath() string {
	switch runtime.GOOS {
	case "darwin":
		candidates := []string{
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"/System/Library/Fonts/Supplemental/Courier New.ttf",
			"/System/Library/Fonts/Supplemental/Times New Roman.ttf",
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	case "linux":
		candidates := []string{
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
		}
		for _, p := range candidates {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// Ensure font package is used (for embedded font check).
var _ = font.Helvetica
