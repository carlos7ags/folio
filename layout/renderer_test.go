// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"bytes"
	"fmt"
	goimage "image"
	"image/jpeg"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
)

func TestRendererSingleParagraph(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Hello World", font.Helvetica, 12))
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if len(pages[0].Fonts) != 1 {
		t.Errorf("expected 1 font, got %d", len(pages[0].Fonts))
	}
	if pages[0].Fonts[0].Standard.Name() != "Helvetica" {
		t.Errorf("expected Helvetica, got %s", pages[0].Fonts[0].Standard.Name())
	}
}

func TestRendererPageBreak(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	// Usable height = 792 - 72 - 72 = 648pt
	// Each line at 12pt * 1.2 leading = 14.4pt
	// 648 / 14.4 = 45 lines per page
	// Generate enough text to exceed one page.
	longText := ""
	for range 200 {
		longText += "This is a test sentence that takes up some horizontal and vertical space on the page. "
	}
	r.Add(NewParagraph(longText, font.Helvetica, 12))
	pages := r.Render()
	if len(pages) < 2 {
		t.Errorf("expected at least 2 pages for long text, got %d", len(pages))
	}
}

func TestRendererMultipleElements(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("First paragraph.", font.Helvetica, 12))
	r.Add(NewParagraph("Second paragraph.", font.HelveticaBold, 14))
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	// Should have two fonts registered.
	if len(pages[0].Fonts) != 2 {
		t.Errorf("expected 2 fonts, got %d", len(pages[0].Fonts))
	}
}

// TestAutoHeightWithFirstMargins verifies that an auto-height page (page
// height 0) which also has @page :first margins is sized using the :first
// margins — the same margins used to position its content — rather than the
// default margin set. Content is positioned at marginsForPage(0).Top from the
// top, so the computed page height must include that (larger) top margin to
// avoid clipping.
//
// Fail-before/pass-after: before the fix the auto-height height used
// r.margins.Top (default 10) instead of the :first top (200), so the page was
// ~190pt too short and the content's top would fall outside the page box.
func TestAutoHeightWithFirstMargins(t *testing.T) {
	defaultMargins := Margins{Top: 10, Right: 10, Bottom: 10, Left: 10}
	r := NewRenderer(612, 0, defaultMargins) // height 0 => auto-height
	firstTop := 200.0
	r.SetFirstMargins(Margins{Top: firstTop, Right: 10, Bottom: 10, Left: 10})
	r.Add(NewParagraph("Hello", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	got := pages[0].PageHeight
	// The page must be at least as tall as the :first top margin plus the
	// content. With the default-margin bug the height would be < firstTop.
	if got < firstTop {
		t.Errorf("auto-height page too short: got %.1f, want >= %.1f (:first top margin)", got, firstTop)
	}
	// Sanity: height should account for the larger :first top, not the
	// default top (10). It must exceed default top + bottom + content.
	if got <= defaultMargins.Top+defaultMargins.Bottom {
		t.Errorf("auto-height page ignored :first top margin: got %.1f", got)
	}
}

// firstTextY returns the y-coordinate of the first text-positioning (Td)
// operator in a content stream. Text is drawn from a bottom-left origin, so a
// larger top margin pushes content lower and yields a SMALLER y value.
// Returns -1 if no Td operator is present.
func firstTextY(t *testing.T, stream string) float64 {
	t.Helper()
	for _, line := range strings.Split(stream, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 3 && fields[2] == "Td" {
			var y float64
			if _, err := fmt.Sscanf(fields[1], "%g", &y); err == nil {
				return y
			}
		}
	}
	return -1
}

// TestFirstMarginsWithExplicitPageBreaks is the regression guard for the bug
// where @page :first margins leaked onto EVERY page of a document that uses
// explicit page breaks (AreaBreak). Natural-overflow pagination correctly
// increments the cumulative page index, so :first applied only to page 0; the
// explicit-break path failed to recompute per-page margins after advancing the
// index, so every break-delimited page kept page 0's (:first) margins.
//
// Fail-before/pass-after: before the fix all three pages' first-line Y matched
// (all used the 144pt :first top). After the fix page 0 sits lower (smaller Y,
// from the 144pt top) while pages 1 and 2 sit higher (larger Y, from the 36pt
// base top).
func TestFirstMarginsWithExplicitPageBreaks(t *testing.T) {
	base := Margins{Top: 36, Right: 36, Bottom: 36, Left: 36}
	r := NewRenderer(612, 792, base)
	first := base
	first.Top = 144
	r.SetFirstMargins(first)

	r.Add(NewParagraph("Page one", font.Helvetica, 12))
	r.Add(NewAreaBreak())
	r.Add(NewParagraph("Page two", font.Helvetica, 12))
	r.Add(NewAreaBreak())
	r.Add(NewParagraph("Page three", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(pages))
	}

	y0 := firstTextY(t, string(pages[0].Stream.Bytes()))
	y1 := firstTextY(t, string(pages[1].Stream.Bytes()))
	y2 := firstTextY(t, string(pages[2].Stream.Bytes()))
	for i, y := range []float64{y0, y1, y2} {
		if y < 0 {
			t.Fatalf("page %d has no text", i)
		}
	}

	// Page 0 (:first, 144pt top) must sit lower on the page than pages 1 and 2
	// (base, 36pt top): a larger top margin means a smaller y.
	if !(y0 < y1) {
		t.Errorf(":first top margin did not apply to page 0 only: y0=%.1f y1=%.1f (want y0 < y1)", y0, y1)
	}
	// Pages 1 and 2 (both base margins) must share the same first-line y.
	if y1 != y2 {
		t.Errorf("pages 1 and 2 should use the same base margin: y1=%.1f y2=%.1f", y1, y2)
	}
	// The gap between page 0 and the base pages must equal the margin delta
	// (144 - 36 = 108pt), confirming page 0 alone got the :first top.
	if delta := y1 - y0; delta < 107 || delta > 109 {
		t.Errorf("page 0 top-margin delta = %.1f, want ~108 (144pt :first - 36pt base)", delta)
	}
}

// TestEmptyFirstMarginBoxSuppressesDefault is the regression guard for the bug
// where @page :first { @bottom-center { content: "" } } failed to blank the
// first-page footer. An empty-content :first margin box must OVERRIDE the
// inherited default @bottom-center for page 0 (drawing nothing), while pages 1+
// keep the default footer.
//
// Fail-before/pass-after: before the fix the empty :first box was dropped at
// parse time, so marginBoxesForPage(0) fell back to the default and page 0
// showed the footer like every other page.
func TestEmptyFirstMarginBoxSuppressesDefault(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 36, Right: 36, Bottom: 36, Left: 36})
	r.SetMarginBoxes(map[string]MarginBox{
		"bottom-center": {Content: "FOOTER"},
	})
	// Empty :first box overrides (blanks) the default for page 0 only.
	r.SetFirstMarginBoxes(map[string]MarginBox{
		"bottom-center": {Content: ""},
	})

	r.Add(NewParagraph("Page one", font.Helvetica, 12))
	r.Add(NewAreaBreak())
	r.Add(NewParagraph("Page two", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}

	p0 := string(pages[0].Stream.Bytes())
	p1 := string(pages[1].Stream.Bytes())
	if strings.Contains(p0, "FOOTER") {
		t.Errorf("page 0 footer should be suppressed by empty :first box, but FOOTER is present")
	}
	if !strings.Contains(p1, "FOOTER") {
		t.Errorf("page 1 should keep the default FOOTER, but it is missing")
	}

	// Per-slot merge sanity: the empty :first box must replace the default box
	// for page 0, while page 1 retains the default content.
	if box := r.marginBoxesForPage(0)["bottom-center"]; box.Content != "" {
		t.Errorf("page 0 bottom-center content = %q, want empty (suppressed)", box.Content)
	}
	if box := r.marginBoxesForPage(1)["bottom-center"]; box.Content != "FOOTER" {
		t.Errorf("page 1 bottom-center content = %q, want \"FOOTER\"", box.Content)
	}
}

func TestRendererNoElements(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	pages := r.Render()
	// Even with no elements, renderer creates one blank page.
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestRendererEmptyParagraph(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("", font.Helvetica, 12))
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestRendererAlignCenter(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Hi", font.Helvetica, 12).SetAlign(AlignCenter))
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	// Just verify it doesn't panic and produces content.
	if pages[0].Stream == nil {
		t.Error("expected non-nil stream")
	}
}

func TestRendererCustomMargins(t *testing.T) {
	// Tiny margins = more usable space.
	r := NewRenderer(612, 792, Margins{Top: 10, Right: 10, Bottom: 10, Left: 10})
	longText := ""
	for range 50 {
		longText += "Test sentence for margin verification. "
	}
	pages := r.Render()
	// With no elements, 1 page.
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestRendererFontReuse(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	// Two paragraphs with the same font should reuse the font resource.
	r.Add(NewParagraph("First.", font.Helvetica, 12))
	r.Add(NewParagraph("Second.", font.Helvetica, 12))
	pages := r.Render()
	if len(pages[0].Fonts) != 1 {
		t.Errorf("expected 1 font (reused), got %d", len(pages[0].Fonts))
	}
}

func rendererTestImage(t *testing.T) *folioimage.Image {
	t.Helper()
	img := goimage.NewRGBA(goimage.Rect(0, 0, 200, 100))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("failed to encode test JPEG: %v", err)
	}
	fimg, err := folioimage.NewJPEG(buf.Bytes())
	if err != nil {
		t.Fatalf("failed to create folio Image: %v", err)
	}
	return fimg
}

func TestRendererWithImage(t *testing.T) {
	fimg := rendererTestImage(t)
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewImageElement(fimg))
	pages := r.Render()

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if len(pages[0].Images) != 1 {
		t.Fatalf("expected 1 image registered, got %d", len(pages[0].Images))
	}
	if pages[0].Images[0].Name != "Im1" {
		t.Errorf("expected image name Im1, got %s", pages[0].Images[0].Name)
	}
	if pages[0].Images[0].Image != fimg {
		t.Error("expected the registered image to match the input image")
	}
	// Content stream should contain Do operator for the image
	streamBytes := string(pages[0].Stream.Bytes())
	if !strings.Contains(streamBytes, "/Im1 Do") {
		t.Error("content stream should contain /Im1 Do operator")
	}
	// Should contain cm (concat matrix) for image placement
	if !strings.Contains(streamBytes, "cm") {
		t.Error("content stream should contain cm operator for image placement")
	}
}

func TestRendererWithImageReuse(t *testing.T) {
	fimg := rendererTestImage(t)
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	// Add the same image twice
	r.Add(NewImageElement(fimg).SetSize(100, 50))
	r.Add(NewImageElement(fimg).SetSize(200, 100))
	pages := r.Render()

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	// Same image object should be reused (only 1 entry)
	if len(pages[0].Images) != 1 {
		t.Errorf("expected 1 image (reused), got %d", len(pages[0].Images))
	}
}

func TestRendererWithList(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	list := NewList(font.Helvetica, 12).
		AddItem("First item").
		AddItem("Second item").
		AddItem("Third item")
	r.Add(list)
	pages := r.Render()

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if len(pages[0].Fonts) == 0 {
		t.Error("expected at least 1 font registered")
	}
	// Content stream should contain the bullet character (WinAnsi byte 149).
	streamBytes := pages[0].Stream.Bytes()
	hasBullet := false
	for _, b := range streamBytes {
		if b == 149 { // WinAnsi encoding of U+2022 BULLET
			hasBullet = true
			break
		}
	}
	if !hasBullet {
		t.Error("content stream should contain bullet character")
	}
	// Should contain the item text
	if !strings.Contains(string(streamBytes), "First") {
		t.Error("content stream should contain list item text")
	}
}

func TestRendererRightAlignment(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	p := NewParagraph("Right aligned", font.Helvetica, 12).SetAlign(AlignRight)
	r.Add(p)
	pages := r.Render()

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	// The stream should have content (the text was rendered)
	streamBytes := string(pages[0].Stream.Bytes())
	if !strings.Contains(streamBytes, "Right") {
		t.Error("content stream should contain the word 'Right'")
	}
	// For right alignment, the x position should be greater than the left margin (72)
	// We can verify the Td operator has an x value > 72
	if !strings.Contains(streamBytes, "Td") {
		t.Error("content stream should contain Td operator")
	}
}

func TestRendererWithImageCenterAlign(t *testing.T) {
	fimg := rendererTestImage(t)
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewImageElement(fimg).SetSize(100, 50).SetAlign(AlignCenter))
	pages := r.Render()

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	streamBytes := string(pages[0].Stream.Bytes())
	if !strings.Contains(streamBytes, "/Im1 Do") {
		t.Error("content stream should contain /Im1 Do operator")
	}
}

func TestRendererWithImageRightAlign(t *testing.T) {
	fimg := rendererTestImage(t)
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewImageElement(fimg).SetSize(100, 50).SetAlign(AlignRight))
	pages := r.Render()

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	streamBytes := string(pages[0].Stream.Bytes())
	if !strings.Contains(streamBytes, "/Im1 Do") {
		t.Error("content stream should contain /Im1 Do operator")
	}
}

func TestRendererWithOrderedList(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	list := NewList(font.Helvetica, 12).
		SetStyle(ListOrdered).
		AddItem("Alpha").
		AddItem("Beta")
	r.Add(list)
	pages := r.Render()

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	streamBytes := string(pages[0].Stream.Bytes())
	if !strings.Contains(streamBytes, "1.") {
		t.Error("content stream should contain ordered marker '1.'")
	}
}

func TestRendererJustifiedText(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	// Use enough text to force wrapping so we get a non-last justified line
	text := "This is a test of justified text that should wrap across multiple lines for proper testing."
	p := NewParagraph(text, font.Helvetica, 12).SetAlign(AlignJustify)
	r.Add(p)
	pages := r.Render()

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	if pages[0].Stream == nil {
		t.Error("expected non-nil stream")
	}
}

func TestRendererWithNestedList(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	l := NewList(font.Helvetica, 12)
	sub := l.AddItemWithSubList("Parent item")
	sub.AddItem("Child A")
	sub.AddItem("Child B")
	l.AddItem("Sibling item")

	r.Add(l)
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	// Nested list should render without errors and produce content.
	if pages[0].Stream == nil {
		t.Error("expected non-nil stream")
	}
	if len(pages[0].Fonts) == 0 {
		t.Error("expected at least one font registered")
	}
}

// --- Absolute positioning ---

func TestRendererAddAbsolute(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Flow content", font.Helvetica, 12))

	// Place a paragraph at an absolute position.
	r.AddAbsolute(NewParagraph("Absolute", font.Helvetica, 10), 200, 400, 100)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	stream := string(pages[0].Stream.Bytes())
	// With kerning, text may be split across TJ array elements (e.g. [(Fl) -20 (ow)] TJ),
	// so check for individual fragments rather than the full word.
	if !strings.Contains(stream, "Flo") && !strings.Contains(stream, "Flow") && !strings.Contains(stream, "ow") {
		t.Errorf("stream should contain flow text fragments, got: %s", stream[:min(200, len(stream))])
	}
	if !strings.Contains(stream, "Abs") && !strings.Contains(stream, "Absolute") && !strings.Contains(stream, "olute") {
		t.Errorf("stream should contain absolute text fragments, got: %s", stream[:min(200, len(stream))])
	}
}

func TestRendererAddAbsoluteOnPage(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	// Fill page 1.
	r.Add(NewParagraph("Page 1", font.Helvetica, 12))

	// Force page 2 by adding many lines.
	for range 50 {
		r.Add(NewParagraph("Line of text that fills the page", font.Helvetica, 12))
	}

	// Place absolute text on page 0 and page 1.
	r.AddAbsoluteOnPage(NewParagraph("Stamp0", font.Helvetica, 10), 100, 100, 200, 0)
	r.AddAbsoluteOnPage(NewParagraph("Stamp1", font.Helvetica, 10), 100, 100, 200, 1)

	pages := r.Render()
	if len(pages) < 2 {
		t.Fatalf("expected at least 2 pages, got %d", len(pages))
	}

	s0 := string(pages[0].Stream.Bytes())
	if !strings.Contains(s0, "Stamp0") {
		t.Error("page 0 should contain Stamp0")
	}
	s1 := string(pages[1].Stream.Bytes())
	if !strings.Contains(s1, "Stamp1") {
		t.Error("page 1 should contain Stamp1")
	}
}

func TestRendererAddAbsoluteFixedEveryPage(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	// Fill enough flow content to span at least three pages.
	for range 120 {
		r.Add(NewParagraph("Body line of text that fills the page", font.Helvetica, 12))
	}

	// A fixed masthead must appear on every page.
	r.AddAbsoluteWithOpts(NewParagraph("MASTHEAD", font.Helvetica, 14), 72, 740, 200, AbsoluteOpts{Fixed: true})

	pages := r.Render()
	if len(pages) < 3 {
		t.Fatalf("expected at least 3 pages, got %d", len(pages))
	}
	for i := range pages {
		s := string(pages[i].Stream.Bytes())
		if !strings.Contains(s, "MASTHEAD") {
			t.Errorf("fixed element missing from page %d (should render on every page)", i)
		}
	}
}

func TestRendererAddAbsoluteNonFixedLastPageOnly(t *testing.T) {
	// A non-fixed absolute with pageIndex -1 must remain on the last page only.
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	for range 120 {
		r.Add(NewParagraph("Body line of text that fills the page", font.Helvetica, 12))
	}
	r.AddAbsolute(NewParagraph("ENDMARK", font.Helvetica, 14), 72, 740, 200)

	pages := r.Render()
	if len(pages) < 3 {
		t.Fatalf("expected at least 3 pages, got %d", len(pages))
	}
	last := len(pages) - 1
	for i := range pages {
		s := string(pages[i].Stream.Bytes())
		has := strings.Contains(s, "ENDMARK")
		if i == last && !has {
			t.Errorf("non-fixed absolute should appear on last page %d", i)
		}
		if i != last && has {
			t.Errorf("non-fixed absolute leaked onto non-last page %d", i)
		}
	}
}

func TestRendererAddAbsoluteDefaultWidth(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Flow", font.Helvetica, 12))

	// width=0 should use full content width.
	r.AddAbsolute(NewParagraph("Full width absolute text", font.Helvetica, 10), 72, 600, 0)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	stream := string(pages[0].Stream.Bytes())
	if !strings.Contains(stream, "Full") {
		t.Error("stream should contain absolute text")
	}
}

func TestRendererAddAbsoluteNoFlowImpact(t *testing.T) {
	// Absolute elements should not affect flow layout.
	r1 := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r1.Add(NewParagraph("Flow text", font.Helvetica, 12))
	pagesWithout := r1.Render()

	r2 := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r2.Add(NewParagraph("Flow text", font.Helvetica, 12))
	r2.AddAbsolute(NewParagraph("Overlay", font.Helvetica, 10), 50, 50, 100)
	pagesWith := r2.Render()

	// Same number of pages — absolute doesn't cause page breaks.
	if len(pagesWithout) != len(pagesWith) {
		t.Errorf("absolute elements should not change page count: %d vs %d",
			len(pagesWithout), len(pagesWith))
	}
}

func TestRendererAddAbsoluteInvalidPage(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Only page", font.Helvetica, 12))

	// Page 99 doesn't exist — should be silently ignored.
	r.AddAbsoluteOnPage(NewParagraph("Ghost", font.Helvetica, 10), 100, 100, 100, 99)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	stream := string(pages[0].Stream.Bytes())
	if strings.Contains(stream, "Ghost") {
		t.Error("ghost text should not appear (invalid page index)")
	}
}

func TestRendererAddAbsoluteWithTable(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Main content", font.Helvetica, 12))

	// Absolutely position a table.
	tbl := NewTable()
	row := tbl.AddRow()
	row.AddCell("A", font.Helvetica, 10)
	row.AddCell("B", font.Helvetica, 10)
	r.AddAbsolute(tbl, 100, 300, 200)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	stream := string(pages[0].Stream.Bytes())
	if !strings.Contains(stream, "Main") {
		t.Error("should contain flow text")
	}
}

func TestRendererAddAbsoluteWithList(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Flow", font.Helvetica, 12))

	l := NewList(font.Helvetica, 10).AddItem("Absolute item")
	r.AddAbsolute(l, 100, 400, 200)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestRendererAbsoluteOnly(t *testing.T) {
	// No flow elements at all — only absolute.
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.AddAbsolute(NewParagraph("Standalone", font.Helvetica, 12), 100, 500, 200)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	stream := string(pages[0].Stream.Bytes())
	if !strings.Contains(stream, "Standalone") {
		t.Error("absolute-only content should appear on the page")
	}
}

func TestRendererAddAbsoluteWithNestedList(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Flow content", font.Helvetica, 12))

	l := NewList(font.Helvetica, 10)
	sub := l.AddItemWithSubList("Parent")
	sub.AddItem("Child")
	r.AddAbsolute(l, 72, 300, 200)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

// TestMarginBoxesDeterministicOutput guards against map iteration order
// leaking into the content stream: drawMarginBoxes must draw boxes (and
// register their fonts) in a fixed order, not map order.
func TestMarginBoxesDeterministicOutput(t *testing.T) {
	render := func() []byte {
		r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
		r.SetMarginBoxes(map[string]MarginBox{
			"top-left":      {Content: "Left header"},
			"top-center":    {Content: "Doc title"},
			"top-right":     {Content: "Page {counter(page)}"},
			"bottom-center": {Content: "Footer"},
		})
		r.Add(NewParagraph("body text", font.Helvetica, 12))
		pages := r.Render()
		var buf bytes.Buffer
		for _, p := range pages {
			buf.Write(p.Stream.Bytes())
			for _, fe := range p.Fonts {
				buf.WriteString(fe.Name)
			}
		}
		return buf.Bytes()
	}

	want := render()
	for i := 0; i < 20; i++ {
		got := render()
		if !bytes.Equal(want, got) {
			t.Fatalf("iteration %d: output not deterministic (want %d bytes, got %d bytes)", i, len(want), len(got))
		}
	}
}

// TestMarginBoxesFixedOrderMatchesSwitch ensures marginBoxOrder stays in
// sync with the position names drawMarginBoxes' switch recognizes; adding a
// new position requires updating both.
func TestMarginBoxesFixedOrderMatchesSwitch(t *testing.T) {
	want := []string{
		"top-left", "top-center", "top-right",
		"bottom-left", "bottom-center", "bottom-right",
	}
	if len(marginBoxOrder) != len(want) {
		t.Fatalf("marginBoxOrder has %d entries, want %d", len(marginBoxOrder), len(want))
	}
	for i, name := range want {
		if marginBoxOrder[i] != name {
			t.Errorf("marginBoxOrder[%d] = %q, want %q", i, marginBoxOrder[i], name)
		}
	}
}
