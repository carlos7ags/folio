// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// RTL demonstrates Right-To-Left text support: Hebrew paragraphs,
// Arabic contextual shaping, mixed LTR/RTL content, and bracket
// mirroring.
//
// The example tries to load a system font with Arabic/Hebrew support.
// On macOS it uses Arial Hebrew; on Linux it looks for DejaVu Sans or
// Noto Sans Arabic. If no suitable font is found, the example falls
// back to Helvetica (which cannot render Arabic/Hebrew glyphs — the
// layout API calls still work but the glyphs appear as .notdef boxes).
//
// Usage:
//
//	go run ./examples/rtl
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
	doc.Info.Title = "Right-To-Left Text"
	doc.Info.Author = "Folio"

	doc.Add(layout.NewHeading("Right-To-Left Text Support", layout.H1))

	// --- Section 1: Hebrew (bidi only, no shaping needed) ---

	doc.Add(layout.NewHeading("1. Hebrew", layout.H2))

	doc.Add(makeParagraph(
		"Hebrew text uses the Unicode Bidirectional Algorithm for correct "+
			"word ordering. The paragraph auto-detects RTL from the first "+
			"strong character and defaults to right-alignment.",
		nil, font.Helvetica, 11,
	))

	ef := loadArabicFont()

	if ef != nil {
		// Pure Hebrew
		p := layout.NewParagraphEmbedded(
			"\u05E9\u05DC\u05D5\u05DD \u05E2\u05D5\u05DC\u05DD", // שלום עולם
			ef, 14,
		)
		doc.Add(p)

		// Mixed Hebrew + English
		p2 := layout.NewStyledParagraph(
			layout.NewRunEmbedded("Folio ", ef, 12),
			layout.NewRunEmbedded("\u05EA\u05D5\u05DE\u05DA \u05D1\u05E2\u05D1\u05E8\u05D9\u05EA", ef, 12), // תומך בעברית
			layout.NewRunEmbedded(" and ", ef, 12),
			layout.NewRunEmbedded("\u05E2\u05E8\u05D1\u05D9\u05EA", ef, 12), // ערבית
		)
		doc.Add(p2)
	} else {
		doc.Add(makeParagraph(
			"(No Arabic/Hebrew font found on this system. "+
				"Install Arial Hebrew, Noto Sans Arabic, or DejaVu Sans "+
				"to see rendered RTL text.)",
			nil, font.Helvetica, 11,
		))
	}

	// --- Section 2: Arabic (bidi + contextual shaping) ---

	doc.Add(layout.NewHeading("2. Arabic Contextual Shaping", layout.H2))

	doc.Add(makeParagraph(
		"Arabic letters have four positional forms (isolated, initial, "+
			"medial, final) that are selected based on each letter's neighbors. "+
			"Folio applies Presentation Forms-B substitution automatically, "+
			"including the lam-alef ligature.",
		nil, font.Helvetica, 11,
	))

	if ef != nil {
		// "bismillah" = بسم الله
		p := layout.NewParagraphEmbedded(
			"\u0628\u0633\u0645 \u0627\u0644\u0644\u0647",
			ef, 16,
		)
		doc.Add(p)

		// Farsi: "salam donya" = سلام دنیا
		p2 := layout.NewParagraphEmbedded(
			"\u0633\u0644\u0627\u0645 \u062F\u0646\u06CC\u0627",
			ef, 16,
		)
		doc.Add(p2)
	}

	// --- Section 3: Mixed bidi with numbers ---

	doc.Add(layout.NewHeading("3. Mixed Bidirectional Text", layout.H2))

	doc.Add(makeParagraph(
		"Numbers in RTL text stay left-to-right per the Unicode bidi "+
			"algorithm. Brackets are mirrored in RTL runs: ( becomes ) "+
			"and vice versa.",
		nil, font.Helvetica, 11,
	))

	if ef != nil {
		// Hebrew with numbers: "סעיף 42 בחוק" (Section 42 of the law)
		p := layout.NewParagraphEmbedded(
			"\u05E1\u05E2\u05D9\u05E3 42 \u05D1\u05D7\u05D5\u05E7",
			ef, 14,
		)
		doc.Add(p)

		// Brackets in RTL: "(שלום)"
		p2 := layout.NewParagraphEmbedded(
			"(\u05E9\u05DC\u05D5\u05DD)",
			ef, 14,
		)
		doc.Add(p2)
	}

	// --- Section 4: Explicit direction control ---

	doc.Add(layout.NewHeading("4. Explicit Direction Control", layout.H2))

	doc.Add(makeParagraph(
		"Paragraph.SetDirection(DirectionRTL) forces RTL alignment "+
			"even for text with no strong directional characters. "+
			"SetAlign(AlignLeft) overrides the RTL default right-alignment.",
		nil, font.Helvetica, 11,
	))

	// Punctuation-only with RTL direction → right-aligned
	dots := layout.NewParagraph("......", font.Helvetica, 12)
	dots.SetDirection(layout.DirectionRTL)
	doc.Add(dots)

	// RTL paragraph with explicit left override
	if ef != nil {
		left := layout.NewParagraphEmbedded(
			"\u05E9\u05DC\u05D5\u05DD \u05E2\u05D5\u05DC\u05DD",
			ef, 14,
		)
		left.SetAlign(layout.AlignLeft)
		doc.Add(left)
	}

	// --- Save ---

	if err := doc.Save("rtl.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created rtl.pdf")
}

// makeParagraph creates a paragraph with either an embedded font or a
// standard font, depending on what's available.
func makeParagraph(text string, ef *font.EmbeddedFont, std *font.Standard, size float64) *layout.Paragraph {
	if ef != nil {
		return layout.NewParagraphEmbedded(text, ef, size)
	}
	return layout.NewParagraph(text, std, size)
}

// loadArabicFont tries common system font paths for a font with Arabic
// and Hebrew support. Returns nil if none found.
func loadArabicFont() *font.EmbeddedFont {
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/System/Library/Fonts/ArialHB.ttc",
			"/System/Library/Fonts/SFArabic.ttf",
			"/Library/Fonts/Arial Unicode.ttf",
		}
	case "linux":
		paths = []string{
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/noto/NotoSansArabic-Regular.ttf",
			"/usr/share/fonts/opentype/noto/NotoSansArabic-Regular.ttf",
		}
	case "windows":
		paths = []string{
			`C:\Windows\Fonts\arial.ttf`,
			`C:\Windows\Fonts\tahoma.ttf`,
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
