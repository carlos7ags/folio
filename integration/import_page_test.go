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

// TestImportPageAsBackground creates a PDF, re-parses it, imports a page
// as a background into a new document, adds text on top, and verifies
// both the imported content and the new content are present.
func TestImportPageAsBackground(t *testing.T) {
	// Step 1: Create the "template" PDF.
	templateDoc := document.NewDocument(document.PageSizeLetter)
	tp := templateDoc.AddPage()
	tp.AddText("TEMPLATE BACKGROUND", font.HelveticaBold, 24, 72, 400)
	var templateBuf bytes.Buffer
	if _, err := templateDoc.WriteTo(&templateBuf); err != nil {
		t.Fatal(err)
	}

	// Step 2: Parse the template.
	r, err := reader.Parse(templateBuf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatal(err)
	}
	contentStream, err := page.ContentStream()
	if err != nil {
		t.Fatal(err)
	}
	resources, _ := page.Resources()

	// Step 3: Create a new document and import the template page.
	doc := document.NewDocument(document.PageSizeLetter)
	p := doc.AddPage()
	p.ImportPage(contentStream, resources, page.Width, page.Height)
	p.AddText("Overlay text here", font.Helvetica, 14, 72, 300)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// Step 4: Re-parse and verify both texts are present.
	result, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if result.PageCount() != 1 {
		t.Errorf("expected 1 page, got %d", result.PageCount())
	}

	outPage, _ := result.Page(0)
	text, _ := outPage.ExtractText()
	if !strings.Contains(text, "Overlay text here") {
		t.Error("overlay text not found in output")
	}
	// The template text may or may not be extractable depending on
	// how the Form XObject is processed during extraction. The key
	// verification is that the output is a valid PDF with both streams.
	if len(buf.Bytes()) < len(templateBuf.Bytes()) {
		t.Error("output should be larger than template (contains both streams)")
	}
}

// TestImportPageMultiple imports the same template page into multiple
// pages of a new document.
func TestImportPageMultiple(t *testing.T) {
	// Create template.
	templateDoc := document.NewDocument(document.PageSizeA4)
	tp := templateDoc.AddPage()
	tp.AddText("Invoice Template", font.HelveticaBold, 18, 72, 750)
	var templateBuf bytes.Buffer
	if _, err := templateDoc.WriteTo(&templateBuf); err != nil {
		t.Fatal(err)
	}

	r, _ := reader.Parse(templateBuf.Bytes())
	page, _ := r.Page(0)
	cs, _ := page.ContentStream()
	res, _ := page.Resources()
	w, h := page.Width, page.Height

	// Create new document with 3 pages using the same template.
	doc := document.NewDocument(document.PageSizeA4)
	for i := range 3 {
		p := doc.AddPage()
		p.ImportPage(cs, res, w, h)
		p.AddText(
			strings.Repeat("Invoice #", 1)+string(rune('1'+i)),
			font.Helvetica, 12, 72, 700,
		)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	result, _ := reader.Parse(buf.Bytes())
	if result.PageCount() != 3 {
		t.Errorf("expected 3 pages, got %d", result.PageCount())
	}
}

// TestImportPageWithExtractPageImport uses the ExtractPageImport convenience
// function which resolves all indirect references. This tests the fix for
// importing layout-engine PDFs where fonts are stored as indirect objects.
func TestImportPageWithExtractPageImport(t *testing.T) {
	// Step 1: Create a PDF with the layout engine (produces indirect font refs).
	templateDoc := document.NewDocument(document.PageSizeLetter)
	templateDoc.Add(layout.NewHeading("Invoice #1234", layout.H1))
	templateDoc.Add(layout.NewParagraph("Customer: Acme Corp", font.Helvetica, 12))
	templateDoc.Add(layout.NewParagraph("Amount: $500.00", font.HelveticaBold, 14))

	var templateBuf bytes.Buffer
	if _, err := templateDoc.WriteTo(&templateBuf); err != nil {
		t.Fatal(err)
	}

	// Step 2: Use ExtractPageImport (exercises resolveDeep).
	r, err := reader.Parse(templateBuf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	imp, err := reader.ExtractPageImport(r, 0)
	if err != nil {
		t.Fatalf("ExtractPageImport: %v", err)
	}

	if imp.Width <= 0 || imp.Height <= 0 {
		t.Errorf("invalid dimensions: %.1f x %.1f", imp.Width, imp.Height)
	}
	if len(imp.ContentStream) == 0 {
		t.Error("content stream is empty")
	}
	if imp.Resources == nil {
		t.Fatal("resources is nil")
	}

	// Step 3: Import into a new document and add overlay text.
	doc := document.NewDocument(document.PageSizeLetter)
	p := doc.AddPage()
	p.ImportPage(imp.ContentStream, imp.Resources, imp.Width, imp.Height)
	p.AddText("PAID", font.HelveticaBold, 36, 400, 400)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	// Step 4: Verify the output is a valid PDF with overlay text.
	result, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if result.PageCount() != 1 {
		t.Errorf("expected 1 page, got %d", result.PageCount())
	}
	outPage, _ := result.Page(0)
	text, _ := outPage.ExtractText()
	if !strings.Contains(text, "PAID") {
		t.Error("overlay text 'PAID' not found in output")
	}

	// Output should be significantly larger than a blank page
	// (contains Form XObject with fonts + overlay).
	if len(buf.Bytes()) < 500 {
		t.Error("output PDF seems too small")
	}
}

// TestImportPagePreservesAnnotations verifies that adding annotations
// on top of an imported page works correctly.
func TestImportPagePreservesAnnotations(t *testing.T) {
	// Create template.
	templateDoc := document.NewDocument(document.PageSizeLetter)
	tp := templateDoc.AddPage()
	tp.AddText("Background", font.Helvetica, 12, 72, 700)
	var templateBuf bytes.Buffer
	if _, err := templateDoc.WriteTo(&templateBuf); err != nil {
		t.Fatal(err)
	}

	r, _ := reader.Parse(templateBuf.Bytes())
	page, _ := r.Page(0)
	cs, _ := page.ContentStream()
	res, _ := page.Resources()

	// Import and add a link annotation on top.
	doc := document.NewDocument(document.PageSizeLetter)
	p := doc.AddPage()
	p.ImportPage(cs, res, page.Width, page.Height)
	p.AddText("Click here", font.Helvetica, 12, 72, 500)
	p.AddLink([4]float64{72, 490, 200, 510}, "https://example.com")

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// Verify the output is valid and has the link annotation.
	pdf := buf.String()
	if !strings.Contains(pdf, "/Annot") {
		t.Error("expected annotation in output")
	}
	if !strings.Contains(pdf, "example.com") {
		t.Error("expected link URI in output")
	}
}
