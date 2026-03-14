// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

func TestDocumentAddParagraph(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Add(layout.NewParagraph("Hello World from the layout engine.", font.Helvetica, 12))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "BT") {
		t.Error("missing BT operator")
	}
	if !strings.Contains(pdf, "/BaseFont /Helvetica") {
		t.Error("missing Helvetica font")
	}
}

func TestDocumentLayoutPageBreak(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	longText := ""
	for range 200 {
		longText += "This is a test sentence to fill up the page and trigger an automatic page break. "
	}
	doc.Add(layout.NewParagraph(longText, font.Helvetica, 12))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	// Should have multiple pages.
	if !strings.Contains(pdf, "/Count 2") && !strings.Contains(pdf, "/Count 3") &&
		!strings.Contains(pdf, "/Count 4") && !strings.Contains(pdf, "/Count 5") {
		t.Error("expected multiple pages from page break")
	}
}

func TestDocumentLayoutWithManualPages(t *testing.T) {
	doc := NewDocument(PageSizeLetter)

	// Add a manual page first.
	p := doc.AddPage()
	p.AddText("Manual page", font.Helvetica, 24, 72, 720)

	// Then add layout elements.
	doc.Add(layout.NewParagraph("Layout paragraph.", font.Helvetica, 12))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	// Should have at least 2 pages (1 manual + 1 from layout).
	if !strings.Contains(pdf, "/Count 2") {
		t.Error("expected 2 pages (manual + layout)")
	}
}

func TestDocumentLayoutAlignment(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Add(layout.NewParagraph("Centered text.", font.Helvetica, 14).SetAlign(layout.AlignCenter))
	doc.Add(layout.NewParagraph("Right-aligned text.", font.Helvetica, 14).SetAlign(layout.AlignRight))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "BT") {
		t.Error("missing text operators")
	}
}

func TestDocumentSetMargins(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetMargins(layout.Margins{Top: 36, Right: 36, Bottom: 36, Left: 36})
	doc.Add(layout.NewParagraph("Small margins.", font.Helvetica, 12))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	if !strings.Contains(buf.String(), "BT") {
		t.Error("missing text content")
	}
}

func TestDocumentLayoutQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Layout Engine Test"
	doc.Info.Author = "Folio"

	doc.Add(layout.NewParagraph(
		"This is the first paragraph using the layout engine. It should wrap nicely within the page margins and produce a valid PDF.",
		font.Helvetica, 12,
	))
	doc.Add(layout.NewParagraph(
		"This is the second paragraph with bold text. The layout engine handles word wrapping and page breaks automatically.",
		font.HelveticaBold, 12,
	).SetAlign(layout.AlignJustify))

	// Add enough text to trigger a page break.
	longText := ""
	for range 100 {
		longText += "Lorem ipsum dolor sit amet, consectetur adipiscing elit. "
	}
	doc.Add(layout.NewParagraph(longText, font.TimesRoman, 11).SetLeading(1.4))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestDocumentTableQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Table Test"

	tbl := layout.NewTable()
	tbl.SetColumnWidths([]float64{150, 200, 118})

	h := tbl.AddHeaderRow()
	h.AddCell("Name", font.HelveticaBold, 10)
	h.AddCell("Description", font.HelveticaBold, 10)
	h.AddCell("Value", font.HelveticaBold, 10).SetAlign(layout.AlignRight)

	for range 5 {
		r := tbl.AddRow()
		r.AddCell("Item", font.Helvetica, 10)
		r.AddCell("A sample item description", font.Helvetica, 10)
		r.AddCell("$99.99", font.Helvetica, 10).SetAlign(layout.AlignRight)
	}

	doc.Add(tbl)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestDocumentTableWithParagraphQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Mixed Layout Test"

	doc.Add(layout.NewParagraph("Quarterly Report", font.HelveticaBold, 18).SetAlign(layout.AlignCenter))

	tbl := layout.NewTable()
	h := tbl.AddHeaderRow()
	h.AddCell("Quarter", font.HelveticaBold, 10)
	h.AddCell("Revenue", font.HelveticaBold, 10)

	r1 := tbl.AddRow()
	r1.AddCell("Q1", font.Helvetica, 10)
	r1.AddCell("$1,200,000", font.Helvetica, 10)

	r2 := tbl.AddRow()
	r2.AddCell("Q2", font.Helvetica, 10)
	r2.AddCell("$1,500,000", font.Helvetica, 10)

	doc.Add(tbl)

	doc.Add(layout.NewParagraph("Revenue grew 25% quarter over quarter.", font.Helvetica, 11))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestDocumentTablePageBreakQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)

	tbl := layout.NewTable()
	h := tbl.AddHeaderRow()
	h.AddCell("ID", font.HelveticaBold, 10)
	h.AddCell("Name", font.HelveticaBold, 10)

	for range 50 {
		r := tbl.AddRow()
		r.AddCell(strings.Repeat("X", 3), font.Helvetica, 10)
		r.AddCell("Row data with enough text to verify rendering", font.Helvetica, 10)
	}

	doc.Add(tbl)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestDocumentHeaderFooter(t *testing.T) {
	doc := NewDocument(PageSizeLetter)

	doc.SetHeader(func(ctx PageContext, page *Page) {
		page.AddText("Header", font.Helvetica, 9, 72, 756)
	})
	doc.SetFooter(func(ctx PageContext, page *Page) {
		text := fmt.Sprintf("Page %d of %d", ctx.PageIndex+1, ctx.TotalPages)
		page.AddText(text, font.Helvetica, 9, 72, 36)
	})

	doc.Add(layout.NewParagraph("Body text on page 1.", font.Helvetica, 12))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "Header") {
		t.Error("missing header text")
	}
	if !strings.Contains(pdf, "Page 1 of 1") {
		t.Error("missing footer text with page numbers")
	}
}

func TestDocumentHeaderFooterMultiPage(t *testing.T) {
	doc := NewDocument(PageSizeLetter)

	pagesSeen := make(map[int]bool)
	doc.SetFooter(func(ctx PageContext, page *Page) {
		pagesSeen[ctx.PageIndex] = true
		text := fmt.Sprintf("Page %d of %d", ctx.PageIndex+1, ctx.TotalPages)
		page.AddText(text, font.Helvetica, 9, 72, 36)
	})

	// Generate enough text for multiple pages.
	var longText string
	for range 200 {
		longText += "This is a sentence that takes up some space on the page. "
	}
	doc.Add(layout.NewParagraph(longText, font.Helvetica, 12))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	if len(pagesSeen) < 2 {
		t.Errorf("expected footer on multiple pages, got %d", len(pagesSeen))
	}

	pdf := buf.String()
	// Should contain page numbers for all pages.
	totalPages := len(pagesSeen)
	for i := range totalPages {
		expected := fmt.Sprintf("Page %d of %d", i+1, totalPages)
		if !strings.Contains(pdf, expected) {
			t.Errorf("missing %q in PDF", expected)
		}
	}
}

func TestDocumentHeaderFooterQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Header/Footer Test"

	doc.SetHeader(func(ctx PageContext, page *Page) {
		page.AddText("Folio PDF Library", font.HelveticaBold, 10, 72, 760)
	})
	doc.SetFooter(func(ctx PageContext, page *Page) {
		text := fmt.Sprintf("Page %d of %d", ctx.PageIndex+1, ctx.TotalPages)
		page.AddText(text, font.Helvetica, 9, 500, 30)
	})

	doc.Add(layout.NewParagraph("Document body.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestDocumentRichTextQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Rich Text Test"

	// Mixed fonts within a single paragraph.
	doc.Add(layout.NewStyledParagraph(
		layout.Run("This is ", font.Helvetica, 12),
		layout.Run("bold", font.HelveticaBold, 12),
		layout.Run(" and ", font.Helvetica, 12),
		layout.Run("italic", font.HelveticaOblique, 12),
		layout.Run(" text in one paragraph.", font.Helvetica, 12),
	))

	// Mixed sizes.
	doc.Add(layout.NewStyledParagraph(
		layout.Run("BIG", font.HelveticaBold, 24),
		layout.Run(" then small", font.Helvetica, 10),
		layout.Run(" then medium.", font.Helvetica, 14),
	))

	// Colors.
	red := layout.RGB(1, 0, 0)
	blue := layout.RGB(0, 0, 1)
	doc.Add(layout.NewStyledParagraph(
		layout.Run("Black ", font.Helvetica, 12),
		layout.Run("red ", font.Helvetica, 12).WithColor(red),
		layout.Run("blue ", font.Helvetica, 12).WithColor(blue),
		layout.Run("black again.", font.Helvetica, 12),
	))

	// Regular paragraph still works.
	doc.Add(layout.NewParagraph("Plain paragraph after rich text.", font.Helvetica, 11))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestDocumentKerning(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	// "AVAYA" has several kern pairs (A-V, V-A, A-Y, Y-A).
	doc.Add(layout.NewParagraph("AVAYA", font.Helvetica, 24))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	// Should use TJ operator for kerned text.
	if !strings.Contains(pdf, "TJ") {
		t.Error("expected TJ operator for kerned text, got only Tj")
	}
}

func TestDocumentBoxModelQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Box Model Test"

	// Heading with automatic spacing and keep-with-next.
	doc.Add(layout.NewHeading("Section Title", layout.H1))

	// Paragraph with spacing and background.
	doc.Add(layout.NewParagraph("Body text with spacing and background.", font.Helvetica, 12).
		SetSpaceBefore(12).
		SetSpaceAfter(8).
		SetBackground(layout.RGB(0.95, 0.95, 0.95)))

	// Paragraph with underline and strikethrough.
	doc.Add(layout.NewStyledParagraph(
		layout.Run("Normal ", font.Helvetica, 12),
		layout.Run("underlined", font.Helvetica, 12).WithUnderline(),
		layout.Run(" and ", font.Helvetica, 12),
		layout.Run("struck through", font.Helvetica, 12).WithStrikethrough(),
		layout.Run(" text.", font.Helvetica, 12),
	))

	// Table with cell backgrounds and vertical alignment.
	tbl := layout.NewTable()
	tbl.SetColumnWidths([]float64{150, 150, 168})
	h := tbl.AddHeaderRow()
	h.AddCell("Name", font.HelveticaBold, 10).SetBackground(layout.RGB(0.8, 0.8, 0.9))
	h.AddCell("Value", font.HelveticaBold, 10).SetBackground(layout.RGB(0.8, 0.8, 0.9))
	h.AddCell("Notes", font.HelveticaBold, 10).SetBackground(layout.RGB(0.8, 0.8, 0.9))

	r := tbl.AddRow()
	r.AddCell("Item A", font.Helvetica, 10)
	r.AddCell("$100", font.Helvetica, 10).SetAlign(layout.AlignRight).SetVAlign(layout.VAlignMiddle)
	r.AddCell("This is a longer note that wraps", font.Helvetica, 10)

	doc.Add(tbl)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestDocumentUnderlineStrikethrough(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Add(layout.NewStyledParagraph(
		layout.Run("underlined", font.Helvetica, 12).WithUnderline(),
	))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	// Should contain line drawing operators for underline.
	if !strings.Contains(pdf, " l\n") && !strings.Contains(pdf, " l ") {
		t.Error("expected line operators for underline decoration")
	}
}

func TestDocumentGraphicsOperatorsQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Graphics Operators Test"

	p := doc.AddPage()

	// Text with character spacing, text rise, rendering mode.
	cs := p.ContentStream()
	if cs == nil {
		// Force stream creation by adding text first.
		p.AddText("Title", font.HelveticaBold, 18, 72, 720)
		cs = p.ContentStream()
	}
	cs.SaveState()

	// Dashed line.
	cs.SetLineWidth(1)
	cs.SetDashPattern([]float64{4, 2}, 0)
	cs.SetLineCap(1) // round
	cs.SetLineJoin(1)
	cs.MoveTo(72, 700)
	cs.LineTo(540, 700)
	cs.Stroke()

	// Circle.
	cs.SetDashPattern(nil, 0) // solid
	cs.SetLineWidth(2)
	cs.SetStrokeColorRGB(0, 0, 1)
	cs.Circle(300, 500, 50)
	cs.Stroke()

	// Filled rounded rectangle.
	cs.SetFillColorRGB(0.9, 0.9, 0.9)
	cs.SetStrokeColorRGB(0, 0, 0)
	cs.SetLineWidth(1)
	cs.RoundedRect(100, 300, 200, 80, 10)
	cs.FillAndStroke()

	// Text with rise (superscript).
	cs.BeginText()
	cs.SetFont("F1", 12)
	cs.MoveText(110, 330)
	cs.ShowText("E = mc")
	cs.SetTextRise(6)
	cs.SetFont("F1", 8)
	cs.ShowText("2")
	cs.SetTextRise(0)
	cs.EndText()

	cs.RestoreState()

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestDocumentKerningQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Kerning Test"

	// Text with known kern pairs.
	doc.Add(layout.NewParagraph("AVAYA Typography Test", font.Helvetica, 18))
	doc.Add(layout.NewParagraph("Lovely Weather Today", font.TimesRoman, 14))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestDocumentHeadingAndListQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Heading and List Test"

	doc.Add(layout.NewHeading("Project Overview", layout.H1))
	doc.Add(layout.NewParagraph("This document demonstrates headings and lists.", font.Helvetica, 12))

	doc.Add(layout.NewHeading("Features", layout.H2))
	ul := layout.NewList(font.Helvetica, 11).
		AddItem("Headings H1 through H6").
		AddItem("Ordered and unordered lists").
		AddItem("Word wrapping within list items")
	doc.Add(ul)

	doc.Add(layout.NewHeading("Steps", layout.H3))
	ol := layout.NewList(font.Helvetica, 11).
		SetStyle(layout.ListOrdered).
		AddItem("Create a document").
		AddItem("Add headings and lists").
		AddItem("Save to PDF")
	doc.Add(ol)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

// --- Absolute positioning ---

func TestDocumentAbsolutePositioning(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Add(layout.NewParagraph("Normal flow content.", font.Helvetica, 12))
	doc.AddAbsolute(
		layout.NewParagraph("Overlay", font.HelveticaBold, 24),
		200, 400, 200,
	)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	pdfStr := buf.String()
	// Both flow and absolute text should appear in the output.
	if !strings.Contains(pdfStr, "Normal") {
		t.Error("PDF should contain flow text")
	}
	if !strings.Contains(pdfStr, "Overlay") {
		t.Error("PDF should contain absolute text")
	}
}

func TestDocumentAbsoluteOnPageQPDF(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	for range 50 {
		doc.Add(layout.NewParagraph("Fill line for page content.", font.Helvetica, 12))
	}

	doc.AddAbsoluteOnPage(
		layout.NewParagraph("Page 1 stamp", font.HelveticaBold, 14),
		72, 50, 200, 0,
	)
	doc.AddAbsoluteOnPage(
		layout.NewParagraph("Page 2 stamp", font.HelveticaBold, 14),
		72, 50, 200, 1,
	)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}
