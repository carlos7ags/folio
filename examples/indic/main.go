// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Indic demonstrates OpenType shaping for Brahmic scripts. Devanagari
// is the first implementation; Bengali, Tamil, Telugu, Kannada,
// Malayalam, Gurmukhi, Odia, Assamese and Sinhala will slot into this
// example as their shapers land. The layout pipeline routes Indic
// words through the script-specific shaper automatically, so the
// example just loads a capable font and adds paragraphs — reph,
// pre-base matra reordering, half forms, conjuncts, nukta, below-base
// and post-base forms all happen under the hood.
//
// Required fonts (the Devanagari section is skipped if none resolve):
//
//	macOS   /System/Library/Fonts/Supplemental/Devanagari Sangam MN.ttc
//	        /System/Library/Fonts/Supplemental/ITFDevanagari.ttc
//	        /Library/Fonts/Arial Unicode.ttf
//	Linux   /usr/share/fonts/truetype/noto/NotoSansDevanagari-Regular.ttf
//	        /usr/share/fonts/opentype/noto/NotoSansDevanagari-Regular.ttf
//	Windows C:\Windows\Fonts\mangal.ttf
//	        C:\Windows\Fonts\Nirmala.ttf
//
// Usage:
//
//	go run ./examples/indic
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
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Indic Text Shaping"
	doc.Info.Author = "Folio"

	doc.Add(layout.NewHeading("Indic Text Shaping", layout.H1))
	doc.Add(layout.NewParagraph(
		"Brahmic scripts need per-script shaping engines that reorder "+
			"codepoints, form conjuncts, and apply position-dependent "+
			"glyph substitutions. Folio runs the OpenType Indic shaping "+
			"pipeline on each word so the calling code does not.",
		font.Helvetica, 11,
	))

	devanagariSection(doc)

	if err := doc.Save("indic.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created indic.pdf")
}

// devanagariSection renders Devanagari paragraphs that exercise
// every implemented OpenType Indic shaping feature.
func devanagariSection(doc *document.Document) {
	doc.Add(layout.NewHeading("Devanagari", layout.H2))

	ef := loadDevanagariFont()
	if ef == nil {
		doc.Add(layout.NewParagraph(
			"No Devanagari font found on this system; section skipped. "+
				"Install a font such as Noto Sans Devanagari to enable it.",
			font.Helvetica, 10,
		))
		fmt.Println("no Devanagari font found on this system; Devanagari section skipped")
		return
	}

	doc.Add(layout.NewParagraph(
		"Each paragraph below exercises a distinct phase of the OpenType "+
			"Indic shaping pipeline. Copy any paragraph out of the rendered "+
			"PDF and the original Unicode codepoints come back verbatim — "+
			"the shaper emits ActualText markers so copy/paste and "+
			"accessibility tools round-trip cleanly.",
		font.Helvetica, 10,
	))

	// Greeting: "namaste duniya" — simple consonant + vowel sequence.
	doc.Add(layout.NewHeading("Greeting", layout.H3))
	doc.Add(layout.NewParagraphEmbedded("\u0928\u092E\u0938\u094D\u0924\u0947 \u0926\u0941\u0928\u093F\u092F\u093E", ef, 18))

	// Conjunct: "kshatriya" — the kṣa conjunct exercises the akhn
	// (akhand) ligature, formed from ka + virama + ssa.
	doc.Add(layout.NewHeading("Conjunct (akhn)", layout.H3))
	doc.Add(layout.NewParagraphEmbedded("\u0915\u094D\u0937\u0924\u094D\u0930\u093F\u092F", ef, 18))

	// Pre-base matra: "kitna" — the i-vowel sign U+093F is typed
	// after ka but renders visually before it.
	doc.Add(layout.NewHeading("Pre-base matra reorder", layout.H3))
	doc.Add(layout.NewParagraphEmbedded("\u0915\u093F\u0924\u0928\u093E", ef, 18))

	// Reph: "karma" — ra + virama at the start of a cluster
	// becomes a superscript reph over the following base.
	doc.Add(layout.NewHeading("Reph (rphf)", layout.H3))
	doc.Add(layout.NewParagraphEmbedded("\u0915\u0930\u094D\u092E", ef, 18))

	// Half form: "kaccha" — the first ka in a ka+halant+cha cluster
	// takes its half form (truncated vertical stroke) before cha.
	doc.Add(layout.NewHeading("Half form (half)", layout.H3))
	doc.Add(layout.NewParagraphEmbedded("\u0915\u091A\u094D\u091A\u093E", ef, 18))

	// Below-base: "krama" — the ra in ka+halant+ra drops below the
	// base as a subscript ra-kara form.
	doc.Add(layout.NewHeading("Below-base form (blwf)", layout.H3))
	doc.Add(layout.NewParagraphEmbedded("\u0915\u094D\u0930\u092E", ef, 18))

	// Nukta: "qanun" — ka + nukta composes into the qa phoneme
	// used for loanwords from Persian and Arabic.
	doc.Add(layout.NewHeading("Nukta (nukt)", layout.H3))
	doc.Add(layout.NewParagraphEmbedded("\u0915\u093C\u093E\u0928\u0942\u0928", ef, 18))

	// Post-base: "hindi" — ndi cluster exercises post-base
	// consonant placement.
	doc.Add(layout.NewHeading("Post-base form (pstf)", layout.H3))
	doc.Add(layout.NewParagraphEmbedded("\u0939\u093F\u0928\u094D\u0926\u0940", ef, 18))
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
