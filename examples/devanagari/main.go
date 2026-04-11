// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Devanagari demonstrates OpenType shaping for the Devanagari script,
// which is used to write Hindi, Sanskrit, Marathi, Nepali and several
// other languages. The layout pipeline routes Devanagari words through
// the Indic shaper automatically, so the example just loads a
// Devanagari-capable font and adds paragraphs — the reph, pre-base
// matra reordering, and conjunct formation happen under the hood.
//
// The example tries to load a Devanagari font from common system
// locations. If none is found, it prints a message and exits cleanly
// with status 0, mirroring the rtl example.
//
// Usage:
//
//	go run ./examples/devanagari
package main

import (
	"fmt"
	"os"
	"runtime"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

func main() {
	ef := loadDevanagariFont()
	if ef == nil {
		fmt.Println("no Devanagari font found on this system; see source for checked paths")
		return
	}

	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Devanagari Text Shaping"
	doc.Info.Author = "Folio"

	doc.Add(layout.NewHeading("Devanagari Text Shaping", layout.H1))
	doc.Add(layout.NewParagraph(
		"Devanagari is used to write Hindi, Sanskrit, Marathi, Nepali, "+
			"and other South Asian languages. Folio runs the Indic "+
			"shaper on each Devanagari word, handling reph, pre-base "+
			"matra reordering, half forms, and conjuncts.",
		font.Helvetica, 11,
	))

	// Greeting: "namaste duniya" — simple consonant + vowel sequence.
	doc.Add(layout.NewHeading("1. Greeting", layout.H2))
	doc.Add(layout.NewParagraphEmbedded("\u0928\u092E\u0938\u094D\u0924\u0947 \u0926\u0941\u0928\u093F\u092F\u093E", ef, 18))

	// Conjunct: "kshatriya" — the kṣa conjunct exercises the akhn
	// (akhand) ligature, formed from ka + virama + ssa.
	doc.Add(layout.NewHeading("2. Conjunct (akhn)", layout.H2))
	doc.Add(layout.NewParagraphEmbedded("\u0915\u094D\u0937\u0924\u094D\u0930\u093F\u092F", ef, 18))

	// Pre-base matra: "kitna" — the i-vowel sign U+093F is typed
	// after ka but renders visually before it.
	doc.Add(layout.NewHeading("3. Pre-base matra reorder", layout.H2))
	doc.Add(layout.NewParagraphEmbedded("\u0915\u093F\u0924\u0928\u093E", ef, 18))

	// Reph: "karma" — ra + virama at the start of a cluster
	// becomes a superscript reph over the following base.
	doc.Add(layout.NewHeading("4. Reph", layout.H2))
	doc.Add(layout.NewParagraphEmbedded("\u0915\u0930\u094D\u092E", ef, 18))

	if err := doc.Save("devanagari.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created devanagari.pdf")
}

// loadDevanagariFont tries common system font paths for a font with
// Devanagari coverage. Returns nil if none is available.
func loadDevanagariFont() *font.EmbeddedFont {
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/System/Library/Fonts/Supplemental/Devanagari Sangam MN.ttc",
			"/System/Library/Fonts/Supplemental/ITFDevanagari.ttc",
			"/System/Library/Fonts/Supplemental/DevanagariMT.ttc",
			"/Library/Fonts/Arial Unicode.ttf",
		}
	case "linux":
		paths = []string{
			"/usr/share/fonts/truetype/noto/NotoSansDevanagari-Regular.ttf",
			"/usr/share/fonts/opentype/noto/NotoSansDevanagari-Regular.ttf",
			"/usr/share/fonts/noto/NotoSansDevanagari-Regular.ttf",
			"/usr/share/fonts/TTF/NotoSansDevanagari-Regular.ttf",
		}
	case "windows":
		paths = []string{
			`C:\Windows\Fonts\mangal.ttf`,
			`C:\Windows\Fonts\Nirmala.ttf`,
		}
	}
	for _, p := range paths {
		face, err := font.LoadFont(p)
		if err != nil {
			continue
		}
		return font.NewEmbeddedFont(face)
	}
	return nil
}
