// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"path/filepath"
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// These tests exercise issue #328 through the production HTML →
// font-resolution path (ConvertFull → ConvertResult.MarginBoxes), which
// is exactly how document.AddHTML obtains the margin-box body font.
//
// Fixture limitation: the only embeddable font checked into the repo is
// font/testdata/synthetic_cjk.ttf, which covers a few CJK ideographs and
// NOTHING in ASCII. Its cmap maps every ASCII codepoint to GID 0
// (.notdef), so a real-world margin box of page-number digits ("Page 1")
// encodes to an all-zero GID stream and renders INVISIBLE — even though
// the font is embedded and the PDF passes PDF/A. To prove the margin box
// draws REAL glyphs (the regression the review asked to guard) we use
// margin-box content the fixture actually covers (CJK runes), so the
// resolved EmbeddedFont produces a provably non-.notdef GID stream. The
// ASCII case is pinned negatively below to document the fixture gap.

// marginFontHTML builds a document whose body uses the synthetic CJK
// @font-face and whose @page declares margin boxes with the given
// bottom-center / top-right content.
func marginFontHTML(fontPath, bottomCenter, topRight string) string {
	return `<!DOCTYPE html><html><head><style>
@font-face { font-family: 'Synth'; src: url('` + fontPath + `'); }
body { font-family: 'Synth'; font-size: 14px; }
@page { @bottom-center { content: "` + bottomCenter + `"; } @top-right { content: "` + topRight + `"; } }
</style></head><body><p>中华人民共和国</p></body></html>`
}

func resolveCJKFixture(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../font/testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	return abs
}

// bodyRunEmbedded returns the embedded font of the first paragraph run
// in the conversion result, or fails the test.
func bodyRunEmbedded(t *testing.T, elems []layout.Element) *font.EmbeddedFont {
	t.Helper()
	for _, e := range elems {
		p, ok := e.(*layout.Paragraph)
		if !ok {
			continue
		}
		runs := p.Runs()
		if len(runs) == 0 {
			t.Fatal("body paragraph has no runs")
		}
		if runs[0].Embedded == nil {
			t.Fatal("body paragraph run has no embedded font; @font-face body font not resolved")
		}
		return runs[0].Embedded
	}
	t.Fatal("no body Paragraph found in elements")
	return nil
}

// TestIssue328MarginBoxResolvedFontEncodesRealGlyphs is the core
// regression guard: the margin box's PRODUCTION-resolved EmbeddedFont
// must encode its (covered) content to a non-.notdef GID stream via the
// same EncodeString API drawWordEmbedded uses. A regression that routed
// the margin box to .notdef — or that failed to subset the drawn glyphs
// — would yield an all-zero stream and fail here.
func TestIssue328MarginBoxResolvedFontEncodesRealGlyphs(t *testing.T) {
	fontPath := resolveCJKFixture(t)
	// 中华国 are codepoints the fixture covers → GIDs 1, 2, 7.
	const content = "中华国"

	result, err := ConvertFull(marginFontHTML(fontPath, content, content), &Options{AllowAbsolutePaths: true, StrictAssets: true})
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}

	box, ok := result.MarginBoxes["bottom-center"]
	if !ok {
		t.Fatal("bottom-center margin box missing from conversion result")
	}
	if box.Embedded == nil {
		t.Fatal("margin box has no embedded font; body font was not inherited (issue #328)")
	}

	gids := box.Embedded.EncodeString(box.Content)
	if len(gids) == 0 {
		t.Fatal("EncodeString returned no bytes for margin-box content")
	}
	if allZero(gids) {
		t.Fatalf("margin-box content %q encoded to an all-.notdef GID stream % X; "+
			"covered glyphs were routed to .notdef (invisible text)", box.Content, gids)
	}
}

// TestIssue328MarginBoxAsciiIsNotdefInCJKFixture pins the fixture
// limitation that motivates the CJK-content choice above: ASCII
// page-number text resolves to ALL .notdef in the CJK-only font. This is
// why the positive guard cannot use "Page 1" — and it proves the
// all-zero failure mode the positive guard detects is real.
func TestIssue328MarginBoxAsciiIsNotdefInCJKFixture(t *testing.T) {
	fontPath := resolveCJKFixture(t)

	result, err := ConvertFull(marginFontHTML(fontPath, "Page 1", "Page 1"), &Options{AllowAbsolutePaths: true, StrictAssets: true})
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	box := result.MarginBoxes["bottom-center"]
	if box.Embedded == nil {
		t.Fatal("margin box has no embedded font")
	}
	gids := box.Embedded.EncodeString(box.Content)
	if !allZero(gids) {
		t.Fatalf("expected ASCII to map entirely to .notdef in the CJK-only fixture, "+
			"got % X; if this changes, the positive guard could use ASCII directly", gids)
	}
}

// TestIssue328MarginBoxInheritsBodyFontInstance proves the core of the
// feature: the margin box does not just have SOME embedded font, it
// shares the SAME *font.EmbeddedFont instance as the document body text.
// Inheritance is what makes the glyphs subset together and keeps the
// running footer PDF/A-valid. A regression that gave the margin box a
// fresh/independent font (or Helvetica) would break this identity.
func TestIssue328MarginBoxInheritsBodyFontInstance(t *testing.T) {
	fontPath := resolveCJKFixture(t)
	result, err := ConvertFull(marginFontHTML(fontPath, "中", "中"), &Options{AllowAbsolutePaths: true, StrictAssets: true})
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}

	bodyFont := bodyRunEmbedded(t, result.Elements)

	for name, box := range result.MarginBoxes {
		if box.Embedded == nil {
			t.Errorf("margin box %q has no embedded font", name)
			continue
		}
		if box.Embedded != bodyFont {
			t.Errorf("margin box %q embedded font is a different instance than the body "+
				"text font; the body font was not inherited (issue #328)", name)
		}
	}
}

// TestIssue328FirstPageMarginBoxCarriesEmbeddedFont covers the
// reviewer's extra: @page :first margin boxes must also carry the
// embedded body font (the first page is where a title-page footer lands).
func TestIssue328FirstPageMarginBoxCarriesEmbeddedFont(t *testing.T) {
	fontPath := resolveCJKFixture(t)
	htmlStr := `<!DOCTYPE html><html><head><style>
@font-face { font-family: 'Synth'; src: url('` + fontPath + `'); }
body { font-family: 'Synth'; font-size: 14px; }
@page :first { @bottom-center { content: "中"; } }
</style></head><body><p>中华人民共和国</p></body></html>`

	result, err := ConvertFull(htmlStr, &Options{AllowAbsolutePaths: true, StrictAssets: true})
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	if len(result.FirstMarginBoxes) == 0 {
		t.Fatal("no first-page margin boxes parsed from @page :first")
	}
	bodyFont := bodyRunEmbedded(t, result.Elements)
	for name, box := range result.FirstMarginBoxes {
		if box.Embedded == nil {
			t.Errorf("first-page margin box %q has no embedded font", name)
			continue
		}
		if box.Embedded != bodyFont {
			t.Errorf("first-page margin box %q does not share the body font instance", name)
		}
		// And the inherited font must encode the covered content to real
		// glyphs, not .notdef.
		if allZero(box.Embedded.EncodeString(box.Content)) {
			t.Errorf("first-page margin box %q encoded covered content to all .notdef", name)
		}
	}
}

// TestIssue328MarginBoxEmbeddedMeasureNonZero is the reviewer's
// right-align width extra: the embedded font must measure covered
// margin-box content to a positive width so right/center alignment math
// in drawMarginBoxes places the box correctly. A zero width would jam a
// right-aligned footer against the right margin regardless of length.
func TestIssue328MarginBoxEmbeddedMeasureNonZero(t *testing.T) {
	fontPath := resolveCJKFixture(t)
	result, err := ConvertFull(marginFontHTML(fontPath, "中", "中华国"), &Options{AllowAbsolutePaths: true, StrictAssets: true})
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	box := result.MarginBoxes["top-right"]
	if box.Embedded == nil {
		t.Fatal("top-right margin box has no embedded font")
	}
	const fontSize = 9.0
	w := box.Embedded.MeasureString(box.Content, fontSize)
	if w <= 0 {
		t.Fatalf("embedded MeasureString(%q) = %v; want > 0 for right-align placement", box.Content, w)
	}
	// A longer covered string must measure wider than a shorter one, so
	// right-edge placement (x = pageWidth - margin - width) actually
	// tracks content length.
	short := box.Embedded.MeasureString("中", fontSize)
	if w <= short {
		t.Fatalf("MeasureString(%q)=%v not greater than MeasureString(\"中\")=%v; "+
			"width does not scale with content length", box.Content, w, short)
	}
}

// allZero reports whether every byte in b is zero — i.e. an Identity-H
// GID stream that is entirely .notdef (GID 0).
func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}
