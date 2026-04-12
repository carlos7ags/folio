// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// RTL demonstrates Right-To-Left text support: Hebrew bidi, Arabic
// contextual shaping, the rlig lam-alef ligature, GPOS mark-to-base
// attachment for harakat, kashida justification, and ActualText
// round-trip for accessibility and copy/paste.
//
// Required fonts (the example picks one per script — if neither
// script has a font available that section is skipped):
//
//	Hebrew:
//	  macOS   /System/Library/Fonts/Supplemental/Arial.ttf
//	          /System/Library/Fonts/Supplemental/Tahoma.ttf
//	          /Library/Fonts/Arial Unicode.ttf
//	  Linux   /usr/share/fonts/truetype/dejavu/DejaVuSans.ttf
//	          /usr/share/fonts/truetype/noto/NotoSansHebrew-Regular.ttf
//	  Windows C:\Windows\Fonts\arial.ttf
//	          C:\Windows\Fonts\tahoma.ttf
//
//	Arabic:
//	  macOS   /System/Library/Fonts/SFArabic.ttf
//	          /System/Library/Fonts/Supplemental/Arial.ttf
//	          /System/Library/Fonts/Supplemental/Tahoma.ttf
//	          /Library/Fonts/Arial Unicode.ttf
//	  Linux   /usr/share/fonts/truetype/noto/NotoSansArabic-Regular.ttf
//	          /usr/share/fonts/opentype/noto/NotoSansArabic-Regular.ttf
//	          /usr/share/fonts/truetype/dejavu/DejaVuSans.ttf
//	  Windows C:\Windows\Fonts\arial.ttf
//	          C:\Windows\Fonts\tahoma.ttf
//
// The example prefers .ttf files over .ttc collections because
// collection parsing is not yet fully supported.
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

	// Load Hebrew and Arabic fonts independently. On most systems no
	// single file covers both scripts well, so the example picks the
	// best font per script and uses each in its own section.
	hebrewEF := loadHebrewFont()
	arabicEF := loadArabicFont()

	if hebrewEF == nil && arabicEF == nil {
		doc.Add(makeParagraph(
			"(No Hebrew or Arabic font found on this system. "+
				"Install Arial, Tahoma, or a Noto Sans variant to see "+
				"rendered RTL text.)",
			nil, font.Helvetica, 11,
		))
	}

	// --- Section 1: Hebrew (bidi only, no shaping needed) ---

	doc.Add(layout.NewHeading("1. Hebrew", layout.H2))

	doc.Add(makeParagraph(
		"Hebrew text uses the Unicode Bidirectional Algorithm for correct "+
			"word ordering. The paragraph auto-detects RTL from the first "+
			"strong character and defaults to right-alignment.",
		nil, font.Helvetica, 11,
	))

	if hebrewEF != nil {
		// Pure Hebrew
		p := layout.NewParagraphEmbedded(
			"\u05E9\u05DC\u05D5\u05DD \u05E2\u05D5\u05DC\u05DD", // שלום עולם
			hebrewEF, 14,
		)
		doc.Add(p)

		// Mixed Hebrew + English
		p2 := layout.NewStyledParagraph(
			layout.NewRunEmbedded("Folio ", hebrewEF, 12),
			layout.NewRunEmbedded("\u05EA\u05D5\u05DE\u05DA \u05D1\u05E2\u05D1\u05E8\u05D9\u05EA", hebrewEF, 12), // תומך בעברית
			layout.NewRunEmbedded(" and ", hebrewEF, 12),
			layout.NewRunEmbedded("\u05E2\u05E8\u05D1\u05D9\u05EA", hebrewEF, 12), // ערבית
		)
		doc.Add(p2)
	}

	// --- Section 2: Arabic (bidi + contextual shaping) ---

	doc.Add(layout.NewHeading("2. Arabic Contextual Shaping", layout.H2))

	doc.Add(makeParagraph(
		"Arabic letters have four positional forms (isolated, initial, "+
			"medial, final) that are selected based on each letter's neighbors. "+
			"Folio applies Presentation Forms-B substitution automatically.",
		nil, font.Helvetica, 11,
	))

	if arabicEF != nil {
		// "bismillah" = بسم الله
		p := layout.NewParagraphEmbedded(
			"\u0628\u0633\u0645 \u0627\u0644\u0644\u0647",
			arabicEF, 16,
		)
		doc.Add(p)

		// Farsi: "salam donya" = سلام دنیا
		p2 := layout.NewParagraphEmbedded(
			"\u0633\u0644\u0627\u0645 \u062F\u0646\u06CC\u0627",
			arabicEF, 16,
		)
		doc.Add(p2)
	}

	// --- Section 2a: Arabic ligatures ---

	doc.Add(layout.NewHeading("2a. Arabic Ligatures (rlig)", layout.H2))

	doc.Add(makeParagraph(
		"The lam-alef ligature is a required ligature in Arabic: whenever "+
			"lam is followed by alef, the pair composes into a single glyph. "+
			"Folio applies the rlig GSUB feature automatically after positional "+
			"shaping.",
		nil, font.Helvetica, 11,
	))

	if arabicEF != nil {
		// "la ilaha" = لا إله (explicitly exercises lam-alef ligature)
		p := layout.NewParagraphEmbedded(
			"\u0644\u0627 \u0625\u0644\u0647",
			arabicEF, 20,
		)
		doc.Add(p)
	}

	// --- Section 2b: Arabic with harakat (GPOS mark attachment) ---

	doc.Add(layout.NewHeading("2b. Arabic with Harakat (GPOS marks)", layout.H2))

	doc.Add(makeParagraph(
		"Vowel marks (harakat) are positioned on each base letter's anchor "+
			"via the OpenType GPOS mark-to-base feature. The combining marks "+
			"contribute zero advance and sit at the correct x/y offset "+
			"recorded in the font's mark anchor table.",
		nil, font.Helvetica, 11,
	))

	if arabicEF != nil {
		// Fully vocalized bismillah: بِسْمِ اللَّهِ الرَّحْمَٰنِ الرَّحِيمِ
		p := layout.NewParagraphEmbedded(
			"\u0628\u0650\u0633\u0652\u0645\u0650 \u0627\u0644\u0644\u0651\u064E\u0647\u0650",
			arabicEF, 20,
		)
		doc.Add(p)
	}

	// --- Section 2c: Kashida justification ---

	doc.Add(layout.NewHeading("2c. Kashida Justification", layout.H2))

	doc.Add(makeParagraph(
		"When Arabic text is justified, Folio inserts tatweel (U+0640, "+
			"also called kashida) between dual-joining letters to elongate "+
			"the connector instead of stretching whitespace. Watch the "+
			"connectors lengthen in the paragraph below.",
		nil, font.Helvetica, 11,
	))

	if arabicEF != nil {
		// A longer Arabic sentence that wraps and justifies.
		// "al-sha`b yurid al-adala wa-l-hurriyya wa-l-musawa lijamii al-muwatinin"
		// (The people want justice, freedom, and equality for all citizens.)
		justified := layout.NewParagraphEmbedded(
			"\u0627\u0644\u0634\u0639\u0628 \u064A\u0631\u064A\u062F "+
				"\u0627\u0644\u0639\u062F\u0627\u0644\u0629 \u0648\u0627\u0644\u062D\u0631\u064A\u0629 "+
				"\u0648\u0627\u0644\u0645\u0633\u0627\u0648\u0627\u0629 \u0644\u062C\u0645\u064A\u0639 "+
				"\u0627\u0644\u0645\u0648\u0627\u0637\u0646\u064A\u0646",
			arabicEF, 18,
		)
		justified.SetAlign(layout.AlignJustify)
		doc.Add(justified)
	}

	// --- Section 2d: Copy-paste round-trip ---

	doc.Add(layout.NewHeading("2d. ActualText Round-Trip", layout.H2))

	doc.Add(makeParagraph(
		"Every shaped Arabic word above is wrapped in an ISO 32000-2 "+
			"ActualText marker that carries the original Unicode. Copying "+
			"text out of this PDF or running pdftotext on it returns the "+
			"original codepoints, not the Presentation Forms-B glyph "+
			"substitutions that the shaper emitted.",
		nil, font.Helvetica, 11,
	))

	// --- Section 3: Mixed bidi with numbers ---

	doc.Add(layout.NewHeading("3. Mixed Bidirectional Text", layout.H2))

	doc.Add(makeParagraph(
		"Numbers in RTL text stay left-to-right per the Unicode bidi "+
			"algorithm. Brackets are mirrored in RTL runs: ( becomes ) "+
			"and vice versa.",
		nil, font.Helvetica, 11,
	))

	if hebrewEF != nil {
		// Hebrew with numbers: "סעיף 42 בחוק" (Section 42 of the law)
		p := layout.NewParagraphEmbedded(
			"\u05E1\u05E2\u05D9\u05E3 42 \u05D1\u05D7\u05D5\u05E7",
			hebrewEF, 14,
		)
		doc.Add(p)

		// Brackets in RTL: "(שלום)"
		p2 := layout.NewParagraphEmbedded(
			"(\u05E9\u05DC\u05D5\u05DD)",
			hebrewEF, 14,
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
	if hebrewEF != nil {
		left := layout.NewParagraphEmbedded(
			"\u05E9\u05DC\u05D5\u05DD \u05E2\u05D5\u05DC\u05DD",
			hebrewEF, 14,
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

// loadHebrewFont tries common system font paths for a font with
// Hebrew coverage. Returns nil if none found. .ttf files are preferred
// over .ttc collections because collection parsing is not yet fully
// supported.
func loadHebrewFont() *font.EmbeddedFont {
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"/System/Library/Fonts/Supplemental/Tahoma.ttf",
			"/Library/Fonts/Arial Unicode.ttf",
		}
	case "linux":
		paths = []string{
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
			"/usr/share/fonts/truetype/noto/NotoSansHebrew-Regular.ttf",
			"/usr/share/fonts/opentype/noto/NotoSansHebrew-Regular.ttf",
		}
	case "windows":
		paths = []string{
			`C:\Windows\Fonts\arial.ttf`,
			`C:\Windows\Fonts\tahoma.ttf`,
		}
	}
	return loadFirstFont(paths)
}

// loadArabicFont tries common system font paths for a font with
// Arabic coverage, including GPOS mark anchors for harakat. .ttf
// files are preferred over .ttc collections.
func loadArabicFont() *font.EmbeddedFont {
	var paths []string
	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/System/Library/Fonts/SFArabic.ttf",
			"/System/Library/Fonts/Supplemental/Arial.ttf",
			"/System/Library/Fonts/Supplemental/Tahoma.ttf",
			"/Library/Fonts/Arial Unicode.ttf",
		}
	case "linux":
		paths = []string{
			"/usr/share/fonts/truetype/noto/NotoSansArabic-Regular.ttf",
			"/usr/share/fonts/opentype/noto/NotoSansArabic-Regular.ttf",
			"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		}
	case "windows":
		paths = []string{
			`C:\Windows\Fonts\arial.ttf`,
			`C:\Windows\Fonts\tahoma.ttf`,
		}
	}
	return loadFirstFont(paths)
}

// loadFirstFont walks the given paths and returns the first one that
// parses successfully as an embedded font, or nil if none worked.
func loadFirstFont(paths []string) *font.EmbeddedFont {
	for _, p := range paths {
		face, err := font.LoadFont(p)
		if err != nil {
			continue
		}
		return font.NewEmbeddedFont(face)
	}
	return nil
}
