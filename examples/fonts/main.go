// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Fonts demonstrates PDF generation with multiple fonts: the 14 standard
// PDF fonts, custom embedded fonts via @font-face, and Unicode text
// including Chinese, Russian, and Japanese.
//
// Custom fonts are loaded from common system paths. On macOS this uses
// fonts from /System/Library/Fonts/Supplemental; on Linux from
// /usr/share/fonts. If a font is not found, that section is skipped.
//
// Usage:
//
//	go run ./examples/fonts
package main

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
)

func main() {
	fonts := discoverFonts()

	// Build @font-face rules and matching CSS classes.
	// Class names must be lowercase because the CSS selector parser lowercases them.
	var css string
	for _, f := range fonts {
		cls := strings.ToLower(f.name)
		css += fmt.Sprintf(
			"@font-face { font-family: '%s'; src: url('%s'); }\n",
			f.name, f.path,
		)
		css += fmt.Sprintf(".cf-%s { font-family: '%s'; }\n", cls, f.name)
	}

	htmlStr := `<html><head><style>
` + css + `
body { margin: 30px; }
h1 { font-size: 22px; color: #1a1a2e; margin-bottom: 8px; }
h2 { font-size: 14px; color: #16213e; margin-top: 14px; margin-bottom: 4px; }
p { margin-bottom: 6px; font-size: 12px; }
.helvetica { font-family: Helvetica; }
.times { font-family: "Times New Roman", serif; }
.courier { font-family: "Courier New", monospace; }
hr { margin: 12px 0; }
</style></head><body>

<h1 class="helvetica">Folio Font Showcase</h1>
<p class="helvetica">This PDF demonstrates standard and custom fonts rendered by the Folio library.</p>

<hr/>
<h2 class="helvetica">Standard PDF Fonts</h2>

<p class="helvetica"><b>Helvetica:</b> The quick brown fox jumps over the lazy dog. 0123456789</p>
<p class="times"><b>Times:</b> The quick brown fox jumps over the lazy dog. 0123456789</p>
<p class="courier"><b>Courier:</b> The quick brown fox jumps over the lazy dog. 0123456789</p>
`

	// Add a section for each discovered custom font.
	for _, f := range fonts {
		cls := strings.ToLower(f.name)
		htmlStr += fmt.Sprintf(`
<hr/>
<h2 class="cf-%s">%s (custom @font-face)</h2>
<p class="cf-%s">The quick brown fox jumps over the lazy dog. 0123456789</p>
`, cls, f.label, cls)

		if f.unicode {
			htmlStr += fmt.Sprintf(`
<p class="cf-%s">Chinese: 你好世界！这是一个使用自定义字体的PDF测试。</p>
<p class="cf-%s">Russian: Привет мир! Быстрая коричневая лиса перепрыгнула через ленивую собаку.</p>
<p class="cf-%s">Japanese: こんにちは世界！カスタムフォントを使用したPDFテストです。</p>
<p class="cf-%s">Mixed: English meets 中文 meets Русский meets 日本語 — all in one paragraph!</p>
`, cls, cls, cls, cls)
		}
	}

	htmlStr += `</body></html>`

	result, err := html.ConvertFull(htmlStr, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "convert: %v\n", err)
		os.Exit(1)
	}

	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Folio Font Showcase"
	doc.Info.Author = "Folio"

	for _, e := range result.Elements {
		doc.Add(e)
	}

	if err := doc.Save("fonts.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created fonts.pdf")
}

type fontEntry struct {
	name    string // CSS font-family name (no spaces, used as class suffix)
	label   string // human-readable display label
	path    string // filesystem path to the font file
	unicode bool   // true if the font covers CJK and Cyrillic
}

// discoverFonts returns custom fonts available on the current system.
func discoverFonts() []fontEntry {
	var candidates []fontEntry

	switch runtime.GOOS {
	case "darwin":
		candidates = []fontEntry{
			{"Verdana", "Verdana", "/System/Library/Fonts/Supplemental/Verdana.ttf", false},
			{"Georgia", "Georgia", "/System/Library/Fonts/Supplemental/Georgia.ttf", false},
			{"Impact", "Impact", "/System/Library/Fonts/Supplemental/Impact.ttf", false},
			{"ComicSans", "Comic Sans MS", "/System/Library/Fonts/Supplemental/Comic Sans MS.ttf", false},
			{"Chalkduster", "Chalkduster", "/System/Library/Fonts/Supplemental/Chalkduster.ttf", false},
			{"BrushScript", "Brush Script", "/System/Library/Fonts/Supplemental/Brush Script.ttf", false},
			{"ArialUnicode", "Arial Unicode MS", "/Library/Fonts/Arial Unicode.ttf", true},
		}
	case "linux":
		candidates = []fontEntry{
			{"NotoSans", "Noto Sans", "/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf", false},
			{"DejaVu", "DejaVu Sans", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", false},
			{"NotoSansCJK", "Noto Sans CJK", "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc", true},
		}
	case "windows":
		candidates = []fontEntry{
			{"Verdana", "Verdana", `C:\Windows\Fonts\verdana.ttf`, false},
			{"Georgia", "Georgia", `C:\Windows\Fonts\georgia.ttf`, false},
			{"Impact", "Impact", `C:\Windows\Fonts\impact.ttf`, false},
			{"ComicSans", "Comic Sans MS", `C:\Windows\Fonts\comic.ttf`, false},
			{"Arial", "Arial", `C:\Windows\Fonts\arial.ttf`, true},
		}
	}

	var found []fontEntry
	for _, c := range candidates {
		if _, err := os.Stat(c.path); err == nil {
			found = append(found, c)
		}
	}
	return found
}
