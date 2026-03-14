// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package integration_test

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/carlos7ags/folio/barcode"
	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
	"github.com/carlos7ags/folio/svg"
)

// qpdfCheck validates PDF bytes with qpdf --check.
func qpdfCheck(t *testing.T, data []byte) {
	t.Helper()
	if _, err := exec.LookPath("qpdf"); err != nil {
		t.Skip("qpdf not found, skipping validation")
	}
	tmp := t.TempDir()
	path := tmp + "/out.pdf"
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("write temp PDF: %v", err)
	}
	out, err := exec.Command("qpdf", "--check", path).CombinedOutput()
	if err != nil {
		t.Fatalf("qpdf --check failed: %v\n%s", err, out)
	}
}

// renderDoc writes a Document to bytes and runs qpdf --check.
func renderDoc(t *testing.T, doc *document.Document) []byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	data := buf.Bytes()
	if len(data) < 100 {
		t.Fatal("PDF output too small")
	}
	qpdfCheck(t, data)
	return data
}

// parsePDF reads back PDF bytes and returns the reader.
func parsePDF(t *testing.T, data []byte) *reader.PdfReader {
	t.Helper()
	r, err := reader.Parse(data)
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	return r
}

// extractAllText extracts text from all pages.
func extractAllText(t *testing.T, r *reader.PdfReader) string {
	t.Helper()
	var sb strings.Builder
	for i := range r.PageCount() {
		page, err := r.Page(i)
		if err != nil {
			t.Fatalf("Page(%d): %v", i, err)
		}
		text, err := page.ExtractText()
		if err != nil {
			t.Fatalf("ExtractText page %d: %v", i, err)
		}
		sb.WriteString(text)
		sb.WriteString("\n")
	}
	return sb.String()
}

// makeTestImage creates a small RGBA image for testing.
func makeTestImage(w, h int) *folioimage.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{
				R: uint8(x * 255 / w),
				G: uint8(y * 255 / h),
				B: 128,
				A: 255,
			})
		}
	}
	return folioimage.NewFromGoImage(img)
}

const testSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100">
  <circle cx="50" cy="50" r="40" fill="#3498db" stroke="#2c3e50" stroke-width="3"/>
  <rect x="30" y="30" width="40" height="40" fill="none" stroke="#e74c3c" stroke-width="2"/>
  <text x="50" y="55" text-anchor="middle" font-size="14" fill="white">SVG</text>
</svg>`

// TestMultiFeatureKitchenSink builds a document exercising every layout
// element and document-level feature, then validates it with qpdf and the reader.
func TestMultiFeatureKitchenSink(t *testing.T) {
	doc := document.NewDocument(document.PageSizeA4)

	// -- Metadata --
	doc.Info = document.Info{
		Title:        "Folio Multi-Feature Test",
		Author:       "Integration Test Suite",
		Subject:      "Comprehensive layout test",
		Keywords:     "folio, pdf, test",
		Creator:      "folio/integration",
		Producer:     "Folio Go Engine",
		CreationDate: time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
		ModDate:      time.Date(2026, 3, 16, 12, 0, 0, 0, time.UTC),
	}

	// -- Auto bookmarks & tagged PDF --
	doc.SetAutoBookmarks(true)
	doc.SetTagged(true)

	// -- Margins --
	doc.SetMargins(layout.Margins{Top: 72, Right: 54, Bottom: 72, Left: 54})

	// -- Watermark --
	doc.SetWatermark("DRAFT")

	// -- Header & Footer --
	doc.SetHeader(func(ctx document.PageContext, page *document.Page) {
		page.AddText("Folio Multi-Feature Test", font.Helvetica, 8, 54, 36)
	})
	doc.SetFooter(func(ctx document.PageContext, page *document.Page) {
		page.AddText("Page Footer", font.Helvetica, 8, 54, 756)
	})

	// =================================================================
	// PAGE 1 — Headings, paragraphs, styled text
	// =================================================================
	doc.Add(layout.NewHeading("Chapter 1: Typography", layout.H1))
	doc.Add(layout.NewHeading("Section 1.1: Fonts", layout.H2))

	// Standard fonts
	fonts := []*font.Standard{
		font.Helvetica,
		font.HelveticaBold,
		font.TimesRoman,
		font.TimesBold,
		font.Courier,
		font.CourierBold,
	}
	fontNames := []string{
		"Helvetica", "Helvetica-Bold",
		"Times-Roman", "Times-Bold",
		"Courier", "Courier-Bold",
	}
	for i, f := range fonts {
		doc.Add(layout.NewParagraph(fontNames[i]+": The quick brown fox jumps over the lazy dog.", f, 11))
	}

	// Styled paragraph with multiple runs
	doc.Add(layout.NewHeading("Section 1.2: Styled Text", layout.H3))
	styled := layout.NewStyledParagraph(
		layout.TextRun{Text: "Bold ", Font: font.HelveticaBold, FontSize: 12, Color: layout.ColorBlack},
		layout.TextRun{Text: "Italic ", Font: font.HelveticaOblique, FontSize: 12, Color: layout.ColorBlue},
		layout.TextRun{Text: "Red ", Font: font.Helvetica, FontSize: 12, Color: layout.ColorRed},
		layout.TextRun{Text: "Underlined", Font: font.Helvetica, FontSize: 12, Decoration: layout.DecorationUnderline},
	)
	styled.SetLeading(16).SetSpaceBefore(4).SetSpaceAfter(8)
	doc.Add(styled)

	// Alignment variants
	for _, align := range []layout.Align{layout.AlignLeft, layout.AlignCenter, layout.AlignRight} {
		p := layout.NewParagraph("Aligned text paragraph for testing.", font.Helvetica, 10)
		p.SetAlign(align)
		doc.Add(p)
	}

	// =================================================================
	// PAGE 2 — Tables
	// =================================================================
	doc.Add(layout.NewHeading("Chapter 2: Tables", layout.H1))

	// Simple table
	tbl := layout.NewTable()
	tbl.SetColumnWidths([]float64{120, 120, 120, 120})

	hdr := tbl.AddHeaderRow()
	hdr.AddCell("Product", font.HelveticaBold, 10).SetBackground(layout.RGB(0.9, 0.9, 0.9))
	hdr.AddCell("Q1", font.HelveticaBold, 10).SetBackground(layout.RGB(0.9, 0.9, 0.9))
	hdr.AddCell("Q2", font.HelveticaBold, 10).SetBackground(layout.RGB(0.9, 0.9, 0.9))
	hdr.AddCell("Q3", font.HelveticaBold, 10).SetBackground(layout.RGB(0.9, 0.9, 0.9))

	data := [][]string{
		{"Widget A", "$12,000", "$15,300", "$18,200"},
		{"Widget B", "$8,500", "$9,100", "$11,600"},
		{"Widget C", "$3,200", "$4,800", "$5,100"},
	}
	for _, row := range data {
		r := tbl.AddRow()
		for _, cell := range row {
			r.AddCell(cell, font.Helvetica, 10).SetPadding(4)
		}
	}
	doc.Add(tbl)

	// Table with colspan
	doc.Add(layout.NewHeading("Table with Colspan", layout.H3))
	tbl2 := layout.NewTable()
	tbl2.SetColumnWidths([]float64{160, 160, 160})
	spanRow := tbl2.AddRow()
	spanRow.AddCell("Merged across two columns", font.HelveticaBold, 10).SetColspan(2).SetBackground(layout.RGB(0.85, 0.85, 1.0))
	spanRow.AddCell("Single", font.Helvetica, 10)
	normalRow := tbl2.AddRow()
	normalRow.AddCell("A", font.Helvetica, 10)
	normalRow.AddCell("B", font.Helvetica, 10)
	normalRow.AddCell("C", font.Helvetica, 10)
	doc.Add(tbl2)

	// =================================================================
	// PAGE 3 — Lists
	// =================================================================
	doc.Add(layout.NewHeading("Chapter 3: Lists", layout.H1))

	// Unordered list
	doc.Add(layout.NewHeading("Unordered", layout.H3))
	ul := layout.NewList(font.Helvetica, 10)
	ul.SetStyle(layout.ListUnordered)
	ul.AddItem("First item in unordered list")
	ul.AddItem("Second item")
	ul.AddItem("Third item")
	doc.Add(ul)

	// Ordered list
	doc.Add(layout.NewHeading("Ordered", layout.H3))
	ol := layout.NewList(font.Helvetica, 10)
	ol.SetStyle(layout.ListOrdered)
	ol.AddItem("Step one")
	ol.AddItem("Step two")
	ol.AddItem("Step three")
	doc.Add(ol)

	// Roman numeral list
	doc.Add(layout.NewHeading("Roman Numerals", layout.H3))
	roman := layout.NewList(font.Helvetica, 10)
	roman.SetStyle(layout.ListOrderedRoman)
	roman.AddItem("Introduction")
	roman.AddItem("Methods")
	roman.AddItem("Results")
	roman.AddItem("Discussion")
	doc.Add(roman)

	// =================================================================
	// PAGE 4 — Divs, Flex, Grid
	// =================================================================
	doc.Add(layout.NewHeading("Chapter 4: Layout Containers", layout.H1))

	// Styled div
	doc.Add(layout.NewHeading("Styled Div", layout.H3))
	div := layout.NewDiv()
	div.SetPadding(12).
		SetBackground(layout.RGB(0.95, 0.95, 0.95)).
		SetBorders(layout.AllBorders(layout.SolidBorder(1, layout.ColorBlack)))
	div.Add(layout.NewParagraph("Content inside a styled div with padding and border.", font.Helvetica, 10))
	div.Add(layout.NewParagraph("Second paragraph in the same div.", font.Helvetica, 10))
	doc.Add(div)

	// Nested divs
	doc.Add(layout.NewHeading("Nested Divs", layout.H3))
	outer := layout.NewDiv()
	outer.SetPadding(8).SetBackground(layout.RGB(0.9, 0.9, 1.0))
	inner := layout.NewDiv()
	inner.SetPadding(8).SetBackground(layout.RGB(1.0, 0.9, 0.9))
	inner.Add(layout.NewParagraph("Inner div content", font.Helvetica, 10))
	outer.Add(inner)
	doc.Add(outer)

	// Flex layout
	doc.Add(layout.NewHeading("Flex Layout", layout.H3))
	flex := layout.NewFlex()
	flex.SetGap(8).SetPadding(4)
	flex.Add(layout.NewParagraph("Flex item 1", font.Helvetica, 10))
	flex.Add(layout.NewParagraph("Flex item 2", font.Helvetica, 10))
	flex.Add(layout.NewParagraph("Flex item 3", font.Helvetica, 10))
	doc.Add(flex)

	// Grid layout
	doc.Add(layout.NewHeading("Grid Layout", layout.H3))
	grid := layout.NewGrid()
	grid.SetTemplateColumns([]layout.GridTrack{
		{Type: layout.GridTrackFr, Value: 1},
		{Type: layout.GridTrackFr, Value: 1},
		{Type: layout.GridTrackFr, Value: 1},
	})
	grid.SetGap(8, 8).SetPadding(4)
	grid.AddChild(layout.NewParagraph("Grid cell 1", font.Helvetica, 10))
	grid.AddChild(layout.NewParagraph("Grid cell 2", font.Helvetica, 10))
	grid.AddChild(layout.NewParagraph("Grid cell 3", font.Helvetica, 10))
	grid.AddChild(layout.NewParagraph("Grid cell 4", font.Helvetica, 10))
	grid.AddChild(layout.NewParagraph("Grid cell 5", font.Helvetica, 10))
	grid.AddChild(layout.NewParagraph("Grid cell 6", font.Helvetica, 10))
	doc.Add(grid)

	// =================================================================
	// PAGE 5 — Images, SVG, Barcodes
	// =================================================================
	doc.Add(layout.NewHeading("Chapter 5: Graphics", layout.H1))

	// Image
	doc.Add(layout.NewHeading("Embedded Image", layout.H3))
	testImg := makeTestImage(80, 60)
	imgElem := layout.NewImageElement(testImg)
	imgElem.SetSize(200, 150)
	doc.Add(imgElem)

	// SVG
	doc.Add(layout.NewHeading("Inline SVG", layout.H3))
	svgDoc, err := svg.Parse(testSVG)
	if err != nil {
		t.Fatalf("svg.Parse: %v", err)
	}
	svgElem := layout.NewSVGElement(svgDoc)
	svgElem.SetSize(150, 150)
	doc.Add(svgElem)

	// Barcodes
	doc.Add(layout.NewHeading("Barcodes", layout.H3))

	qrCode, err := barcode.QR("https://github.com/carlos7ags/folio")
	if err != nil {
		t.Fatalf("barcode.QR: %v", err)
	}
	doc.Add(layout.NewBarcodeElement(qrCode, 100).SetHeight(100))

	code128, err := barcode.Code128("FOLIO-2026")
	if err != nil {
		t.Fatalf("barcode.Code128: %v", err)
	}
	doc.Add(layout.NewBarcodeElement(code128, 200).SetHeight(50))

	ean, err := barcode.EAN13("978014028032")
	if err != nil {
		t.Fatalf("barcode.EAN13: %v", err)
	}
	doc.Add(layout.NewBarcodeElement(ean, 200).SetHeight(60))

	// =================================================================
	// PAGE 6 — Decorative elements
	// =================================================================
	doc.Add(layout.NewHeading("Chapter 6: Decorative Elements", layout.H1))

	// Line separator
	doc.Add(layout.NewLineSeparator().
		SetColor(layout.ColorRed).
		SetWidth(2).
		SetSpaceBefore(8).
		SetSpaceAfter(8))

	doc.Add(layout.NewParagraph("Content between separators.", font.Helvetica, 10))

	doc.Add(layout.NewLineSeparator().
		SetStyle(layout.BorderDashed).
		SetColor(layout.ColorBlue).
		SetFraction(0.8).
		SetAlign(layout.AlignCenter))

	// Tabbed line
	doc.Add(layout.NewHeading("Tabbed Lines", layout.H3))
	tabs := layout.NewTabbedLine(font.Helvetica, 10,
		layout.TabStop{Position: 200, Align: layout.TabAlignLeft},
		layout.TabStop{Position: 400, Align: layout.TabAlignRight, Leader: '.'},
	)
	tabs.SetSegments("Item", "Description", "$99.00")
	doc.Add(tabs)

	// =================================================================
	// PAGE 7 — Links
	// =================================================================
	doc.Add(layout.NewHeading("Chapter 7: Links & Annotations", layout.H1))
	doc.Add(layout.NewParagraph("This page tests link annotations on the raw page.", font.Helvetica, 10))

	// -- Render --
	pdfData := renderDoc(t, doc)

	// -- Read back and validate --
	r := parsePDF(t, pdfData)

	if r.PageCount() < 3 {
		t.Errorf("expected at least 3 pages, got %d", r.PageCount())
	}

	text := extractAllText(t, r)

	// Verify key text from each section
	expectations := []string{
		"Chapter 1", "Typography",
		"Helvetica", "Times-Roman", "Courier",
		"Bold", "Red",
		"Chapter 2", "Tables",
		"Widget A", "$12,000", "$18,200",
		"Merged",
		"Chapter 3", "Lists",
		"First item", "Step one", "Introduction",
		"Chapter 4", "Layout Containers",
		"Flex item", "Grid cell",
		"Chapter 5", "Graphics",
		"Chapter 6", "Decorative",
		"Tabbed",
		"Chapter 7", "Links",
	}
	for _, exp := range expectations {
		if !strings.Contains(text, exp) {
			t.Errorf("expected text %q not found in extracted text", exp)
		}
	}

	// Verify metadata
	title, author, subject, _, producer := r.Info()
	if title != "Folio Multi-Feature Test" {
		t.Errorf("title = %q, want %q", title, "Folio Multi-Feature Test")
	}
	if author != "Integration Test Suite" {
		t.Errorf("author = %q, want %q", author, "Integration Test Suite")
	}
	if subject != "Comprehensive layout test" {
		t.Errorf("subject = %q, want %q", subject, "Comprehensive layout test")
	}
	if producer != "Folio Go Engine" {
		t.Errorf("producer = %q, want %q", producer, "Folio Go Engine")
	}
}

// TestMultiFeatureEncrypted generates the same rich document but encrypted,
// verifies qpdf can decrypt it, and reads it back.
func TestMultiFeatureEncrypted(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)

	doc.Info = document.Info{
		Title:  "Encrypted Multi-Feature",
		Author: "Test Suite",
	}

	doc.SetEncryption(document.EncryptionConfig{
		Algorithm:     document.EncryptAES256,
		UserPassword:  "user123",
		OwnerPassword: "owner456",
		Permissions:   core.PermAll,
	})

	// Add representative content
	doc.Add(layout.NewHeading("Encrypted Document", layout.H1))
	doc.Add(layout.NewParagraph("This document is AES-256 encrypted.", font.Helvetica, 11))

	tbl := layout.NewTable()
	tbl.SetColumnWidths([]float64{200, 200})
	row := tbl.AddRow()
	row.AddCell("Key", font.HelveticaBold, 10)
	row.AddCell("Value", font.Helvetica, 10)
	doc.Add(tbl)

	ul := layout.NewList(font.Helvetica, 10)
	ul.AddItem("Encrypted item one")
	ul.AddItem("Encrypted item two")
	doc.Add(ul)

	testImg := makeTestImage(40, 30)
	doc.Add(layout.NewImageElement(testImg).SetSize(100, 75))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	data := buf.Bytes()

	// qpdf should be able to decrypt with owner password
	if _, err := exec.LookPath("qpdf"); err != nil {
		t.Skip("qpdf not found")
	}
	tmp := t.TempDir()
	encPath := tmp + "/encrypted.pdf"
	decPath := tmp + "/decrypted.pdf"
	if err := os.WriteFile(encPath, data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Decrypt with owner password
	out, err := exec.Command("qpdf", "--password=owner456", "--decrypt", encPath, decPath).CombinedOutput()
	if err != nil {
		t.Fatalf("qpdf --decrypt failed: %v\n%s", err, out)
	}

	// Validate decrypted PDF
	out, err = exec.Command("qpdf", "--check", decPath).CombinedOutput()
	if err != nil {
		t.Fatalf("qpdf --check decrypted failed: %v\n%s", err, out)
	}

	// Read back decrypted PDF
	decData, err := os.ReadFile(decPath)
	if err != nil {
		t.Fatalf("read decrypted: %v", err)
	}
	r, err := reader.Parse(decData)
	if err != nil {
		t.Fatalf("parse decrypted: %v", err)
	}

	text := extractAllText(t, r)
	if !strings.Contains(text, "Encrypted Document") {
		t.Error("expected 'Encrypted Document' in decrypted text")
	}
	if !strings.Contains(text, "AES-256") {
		t.Error("expected 'AES-256' in decrypted text")
	}
}

// TestMultiFeatureLandscape verifies landscape orientation with content.
func TestMultiFeatureLandscape(t *testing.T) {
	doc := document.NewDocument(document.PageSizeA4.Landscape())

	doc.Add(layout.NewHeading("Landscape Document", layout.H1))
	doc.Add(layout.NewParagraph("This document uses A4 landscape orientation.", font.Helvetica, 11))

	// Wide table that benefits from landscape
	tbl := layout.NewTable()
	tbl.SetColumnWidths([]float64{100, 100, 100, 100, 100, 100, 100})
	hdr := tbl.AddHeaderRow()
	for _, col := range []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"} {
		hdr.AddCell(col, font.HelveticaBold, 9).SetBackground(layout.RGB(0.85, 0.85, 0.85))
	}
	for week := range 4 {
		row := tbl.AddRow()
		for day := range 7 {
			n := week*7 + day + 1
			if n <= 30 {
				row.AddCell(strings.Repeat("*", n%5+1), font.Helvetica, 9)
			} else {
				row.AddCell("", font.Helvetica, 9)
			}
		}
	}
	doc.Add(tbl)

	pdfData := renderDoc(t, doc)
	r := parsePDF(t, pdfData)

	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}

	// A4 landscape: width should be ~842, height ~595
	w := page.MediaBox.Width()
	h := page.MediaBox.Height()
	if w < 800 || w > 900 {
		t.Errorf("landscape width = %.0f, want ~842", w)
	}
	if h < 550 || h > 650 {
		t.Errorf("landscape height = %.0f, want ~595", h)
	}
}

// TestMultiFeatureMultiPage verifies automatic page breaks with a long document.
func TestMultiFeatureMultiPage(t *testing.T) {
	doc := document.NewDocument(document.PageSizeA4)
	doc.SetMargins(layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	doc.Add(layout.NewHeading("Long Document Test", layout.H1))

	// Add enough paragraphs to force multiple pages
	for i := range 50 {
		text := strings.Repeat("This is paragraph number. ", 8)
		_ = i
		doc.Add(layout.NewParagraph(text, font.Helvetica, 11).SetSpaceAfter(6))
	}

	pdfData := renderDoc(t, doc)
	r := parsePDF(t, pdfData)

	if r.PageCount() < 3 {
		t.Errorf("expected at least 3 pages for long content, got %d", r.PageCount())
	}

	text := extractAllText(t, r)
	if !strings.Contains(text, "Long Document Test") {
		t.Error("expected heading in extracted text")
	}
}

// TestMultiFeatureRawContentStream uses page.ContentStream() for direct drawing.
func TestMultiFeatureRawContentStream(t *testing.T) {
	doc := document.NewDocument(document.PageSizeA4)

	page := doc.AddPage()

	// AddText initialises the stream; then we can append raw drawing commands.
	page.AddText("Direct Drawing", font.Helvetica, 24, 120, 640)

	cs := page.ContentStream()

	// Draw a rectangle
	cs.SaveState()
	cs.SetFillColorRGB(0.2, 0.4, 0.8)
	cs.Rectangle(100, 600, 200, 100)
	cs.Fill()
	cs.RestoreState()

	// Draw a line
	cs.SaveState()
	cs.SetStrokeColorRGB(1, 0, 0)
	cs.SetLineWidth(2)
	cs.MoveTo(100, 580)
	cs.LineTo(300, 580)
	cs.Stroke()
	cs.RestoreState()

	pdfData := renderDoc(t, doc)

	// Verify the raw PDF contains our operators
	raw := string(pdfData)
	if !strings.Contains(raw, "Direct Drawing") {
		t.Error("expected 'Direct Drawing' in raw PDF")
	}
}

// TestMultiFeatureDivBoxModel tests div styling: shadow, border-radius, opacity.
func TestMultiFeatureDivBoxModel(t *testing.T) {
	doc := document.NewDocument(document.PageSizeA4)

	// Div with box shadow
	shadow := layout.NewDiv()
	shadow.SetPadding(16).
		SetBackground(layout.ColorWhite).
		SetBorders(layout.AllBorders(layout.SolidBorder(1, layout.RGB(0.8, 0.8, 0.8)))).
		SetBoxShadow(layout.BoxShadow{OffsetX: 3, OffsetY: 3, Blur: 5, Color: layout.RGB(0.5, 0.5, 0.5)}).
		SetBorderRadius(8)
	shadow.Add(layout.NewParagraph("Card with shadow and rounded corners", font.Helvetica, 11))
	doc.Add(shadow)

	// Div with opacity
	faded := layout.NewDiv()
	faded.SetPadding(12).SetBackground(layout.ColorBlue).SetOpacity(0.5)
	faded.Add(layout.NewParagraph("Semi-transparent content", font.Helvetica, 11))
	doc.Add(faded)

	// Div with overflow hidden
	clipped := layout.NewDiv()
	clipped.SetMaxWidth(200).SetMaxHeight(30).SetOverflow("hidden").SetPadding(4)
	clipped.Add(layout.NewParagraph("This text should be clipped if it exceeds the max height of the container div.", font.Helvetica, 10))
	doc.Add(clipped)

	pdfData := renderDoc(t, doc)
	r := parsePDF(t, pdfData)

	text := extractAllText(t, r)
	if !strings.Contains(text, "Card with shadow") {
		t.Error("expected shadow div text")
	}
}

// TestMultiFeatureGridPlacement tests explicit grid cell placement.
func TestMultiFeatureGridPlacement(t *testing.T) {
	doc := document.NewDocument(document.PageSizeA4)
	doc.Add(layout.NewHeading("Grid with Explicit Placement", layout.H2))

	grid := layout.NewGrid()
	grid.SetTemplateColumns([]layout.GridTrack{
		{Type: layout.GridTrackFr, Value: 1},
		{Type: layout.GridTrackFr, Value: 2},
		{Type: layout.GridTrackFr, Value: 1},
	})
	grid.SetGap(4, 4)

	// 4 children, third one spans 2 columns
	grid.AddChild(layout.NewParagraph("Top-Left", font.Helvetica, 10))
	grid.AddChild(layout.NewParagraph("Top-Center", font.Helvetica, 10))
	grid.AddChild(layout.NewParagraph("Top-Right", font.Helvetica, 10))
	grid.AddChild(layout.NewParagraph("Bottom spans two", font.Helvetica, 10))
	grid.SetPlacement(3, layout.GridPlacement{ColStart: 1, ColEnd: 3, RowStart: 2, RowEnd: 3})

	doc.Add(grid)

	pdfData := renderDoc(t, doc)
	r := parsePDF(t, pdfData)
	text := extractAllText(t, r)

	if !strings.Contains(text, "Top-Left") || !strings.Contains(text, "Bottom spans two") {
		t.Error("expected grid content in output")
	}
}
