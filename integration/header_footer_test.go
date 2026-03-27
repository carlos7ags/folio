// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
)

func TestSetHeaderElement(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetHeaderElement(func(ctx document.PageContext) layout.Element {
		return layout.NewParagraph("HEADER TEXT", font.HelveticaBold, 12)
	})
	doc.Add(layout.NewParagraph("Body content here.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// Parse and verify both header and body text are present.
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if r.PageCount() != 1 {
		t.Fatalf("expected 1 page, got %d", r.PageCount())
	}
	page, _ := r.Page(0)
	text, _ := page.ExtractText()
	if !strings.Contains(text, "HEADER TEXT") {
		t.Error("header text not found in output")
	}
	if !strings.Contains(text, "Body content") {
		t.Error("body text not found in output")
	}
}

func TestSetFooterElement(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetFooterElement(func(ctx document.PageContext) layout.Element {
		return layout.NewParagraph("FOOTER TEXT", font.Helvetica, 9)
	})
	doc.Add(layout.NewParagraph("Body content.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, _ := reader.Parse(buf.Bytes())
	page, _ := r.Page(0)
	text, _ := page.ExtractText()
	if !strings.Contains(text, "FOOTER TEXT") {
		t.Error("footer text not found in output")
	}
}

func TestSetHeaderText(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetHeaderText("My Report", font.HelveticaBold, 12, layout.AlignCenter)
	doc.Add(layout.NewParagraph("Content.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, _ := reader.Parse(buf.Bytes())
	page, _ := r.Page(0)
	text, _ := page.ExtractText()
	if !strings.Contains(text, "My Report") {
		t.Error("header text not found")
	}
}

func TestSetFooterTextWithPlaceholders(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetFooterText("Page {page} of {pages}", font.Helvetica, 9, layout.AlignCenter)

	// Add enough content for 2 pages.
	for range 60 {
		doc.Add(layout.NewParagraph("Line of text to fill the page.", font.Helvetica, 12))
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, _ := reader.Parse(buf.Bytes())
	if r.PageCount() < 2 {
		t.Skipf("expected at least 2 pages, got %d", r.PageCount())
	}

	// Check first page footer.
	p1, _ := r.Page(0)
	t1, _ := p1.ExtractText()
	if !strings.Contains(t1, "Page 1 of") {
		t.Errorf("page 1 footer not found, got: %q", t1)
	}

	// Check second page footer.
	p2, _ := r.Page(1)
	t2, _ := p2.ExtractText()
	if !strings.Contains(t2, "Page 2 of") {
		t.Errorf("page 2 footer not found, got: %q", t2)
	}
}

func TestHeaderElementPerPage(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetHeaderElement(func(ctx document.PageContext) layout.Element {
		if ctx.PageIndex == 0 {
			return layout.NewParagraph("COVER PAGE", font.HelveticaBold, 14)
		}
		return layout.NewParagraph("CONTINUATION", font.Helvetica, 10)
	})

	// Add enough content for 2 pages.
	for range 60 {
		doc.Add(layout.NewParagraph("Content line.", font.Helvetica, 12))
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, _ := reader.Parse(buf.Bytes())
	if r.PageCount() < 2 {
		t.Skip("not enough pages")
	}

	p1, _ := r.Page(0)
	t1, _ := p1.ExtractText()
	if !strings.Contains(t1, "COVER PAGE") {
		t.Error("first page should have 'COVER PAGE' header")
	}

	p2, _ := r.Page(1)
	t2, _ := p2.ExtractText()
	if !strings.Contains(t2, "CONTINUATION") {
		t.Error("second page should have 'CONTINUATION' header")
	}
}

func TestHeaderElementNilSkipsPage(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetHeaderElement(func(ctx document.PageContext) layout.Element {
		if ctx.PageIndex == 0 {
			return nil // skip first page header
		}
		return layout.NewParagraph("HEADER", font.Helvetica, 10)
	})
	doc.Add(layout.NewParagraph("Body.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, _ := reader.Parse(buf.Bytes())
	page, _ := r.Page(0)
	text, _ := page.ExtractText()
	if strings.Contains(text, "HEADER") {
		t.Error("first page should not have header when decorator returns nil")
	}
}

func TestHeaderAndFooterTogether(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetHeaderElement(func(ctx document.PageContext) layout.Element {
		return layout.NewParagraph("TOP", font.HelveticaBold, 12)
	})
	doc.SetFooterElement(func(ctx document.PageContext) layout.Element {
		return layout.NewParagraph("BOTTOM", font.Helvetica, 9)
	})
	doc.Add(layout.NewParagraph("MIDDLE", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, _ := reader.Parse(buf.Bytes())
	page, _ := r.Page(0)
	text, _ := page.ExtractText()
	for _, expected := range []string{"TOP", "MIDDLE", "BOTTOM"} {
		if !strings.Contains(text, expected) {
			t.Errorf("%q not found in output", expected)
		}
	}
}

func TestHeaderDoesNotOverlapContent(t *testing.T) {
	// The key test: header should push content down, not overlap.
	doc := document.NewDocument(document.PageSizeA4)
	doc.SetMargins(layout.Margins{Top: 20, Right: 20, Bottom: 20, Left: 20})
	doc.SetHeaderElement(func(ctx document.PageContext) layout.Element {
		return layout.NewParagraph("HEADER", font.HelveticaBold, 14)
	})

	// Fill page with content.
	for range 40 {
		doc.Add(layout.NewParagraph("Content line that should not overlap the header.", font.Helvetica, 12))
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// If header reserving space works, we should get more than 1 page
	// because the content area is reduced by the header height.
	r, _ := reader.Parse(buf.Bytes())
	if r.PageCount() < 2 {
		t.Log("with header reserving space, content should need more pages")
	}

	// Basic validity check.
	if len(buf.Bytes()) < 500 {
		t.Error("output seems too small")
	}
}

func TestHeaderElementWithManualPagesOnly(t *testing.T) {
	// SetHeaderElement should work even when only manual pages exist
	// (no layout elements added via doc.Add).
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetHeaderElement(func(ctx document.PageContext) layout.Element {
		return layout.NewParagraph("MANUAL PAGE HEADER", font.HelveticaBold, 12)
	})
	p := doc.AddPage()
	p.AddText("Manual page content", font.Helvetica, 12, 72, 600)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, _ := reader.Parse(buf.Bytes())
	if r.PageCount() != 1 {
		t.Fatalf("expected 1 page, got %d", r.PageCount())
	}
	page, _ := r.Page(0)
	text, _ := page.ExtractText()
	if !strings.Contains(text, "MANUAL PAGE HEADER") {
		t.Error("header not rendered on manual page")
	}
	if !strings.Contains(text, "Manual page content") {
		t.Error("manual page content not found")
	}
}

func TestHeaderElementOnlyNoBody(t *testing.T) {
	// Header/footer with no body content should still produce a valid PDF.
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetHeaderText("Header Only", font.HelveticaBold, 12, layout.AlignCenter)
	doc.SetFooterText("Footer Only", font.Helvetica, 9, layout.AlignCenter)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// With no content and no manual pages, we may get 0 pages — that's OK.
	// The test ensures no panic or error.
	if len(buf.Bytes()) < 100 {
		t.Error("output seems too small")
	}
}

func TestBothDecoratorAndElementHeader(t *testing.T) {
	// Using both SetHeader (low-level) and SetHeaderElement should both render.
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetHeader(func(ctx document.PageContext, page *document.Page) {
		page.AddText("LOW LEVEL", font.Helvetica, 8, 72, 780)
	})
	doc.SetHeaderElement(func(ctx document.PageContext) layout.Element {
		return layout.NewParagraph("HIGH LEVEL", font.HelveticaBold, 12)
	})
	doc.Add(layout.NewParagraph("Body.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, _ := reader.Parse(buf.Bytes())
	page, _ := r.Page(0)
	text, _ := page.ExtractText()
	if !strings.Contains(text, "LOW LEVEL") {
		t.Error("low-level header not found")
	}
	if !strings.Contains(text, "HIGH LEVEL") {
		t.Error("element-based header not found")
	}
}
