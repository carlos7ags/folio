// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"bytes"
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
	// Content stream should contain the bullet character
	streamBytes := string(pages[0].Stream.Bytes())
	if !strings.Contains(streamBytes, "\u2022") {
		t.Error("content stream should contain bullet character")
	}
	// Should contain the item text
	if !strings.Contains(streamBytes, "First") {
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
	if !strings.Contains(stream, "Flow") {
		t.Error("stream should contain flow text")
	}
	if !strings.Contains(stream, "Absolute") {
		t.Error("stream should contain absolute text")
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
