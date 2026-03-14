// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

func TestPdfA2bBasic(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "PDF/A Test Document"
	doc.Info.Author = "Folio"

	// PDF/A requires embedded fonts — use the layout engine with embedded font
	// or add content via manual page (which uses standard fonts — will fail validation).
	// For this test, use layout-only (no manual pages with standard fonts).
	doc.SetPdfA(PdfAConfig{Level: PdfA2B})

	// No pages with standard fonts — just a blank document with metadata.
	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// Must have XMP metadata.
	if !strings.Contains(pdf, "/Metadata") {
		t.Error("expected /Metadata in catalog")
	}
	if !strings.Contains(pdf, "pdfaid:part") {
		t.Error("expected PDF/A identification in XMP")
	}
	if !strings.Contains(pdf, "<pdfaid:part>2</pdfaid:part>") {
		t.Error("expected PDF/A part 2")
	}
	if !strings.Contains(pdf, "<pdfaid:conformance>B</pdfaid:conformance>") {
		t.Error("expected PDF/A conformance B")
	}

	// Must have output intent.
	if !strings.Contains(pdf, "/OutputIntents") {
		t.Error("expected /OutputIntents in catalog")
	}
	if !strings.Contains(pdf, "GTS_PDFA1") {
		t.Error("expected GTS_PDFA1 output intent subtype")
	}
}

func TestPdfA2bXMPMetadata(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "XMP Test"
	doc.Info.Author = "Test Author"
	doc.Info.Creator = "Test Creator"

	doc.SetPdfA(PdfAConfig{Level: PdfA2B})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "XMP Test") {
		t.Error("XMP should contain title")
	}
	if !strings.Contains(pdf, "Test Author") {
		t.Error("XMP should contain author")
	}
	if !strings.Contains(pdf, "Test Creator") {
		t.Error("XMP should contain creator tool")
	}
	if !strings.Contains(pdf, "/Subtype /XML") {
		t.Error("XMP stream should have /Subtype /XML")
	}
}

func TestPdfA2aEnablesTagging(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Tagged PDF/A"
	doc.SetPdfA(PdfAConfig{Level: PdfA2A})

	if !doc.tagged {
		t.Error("PDF/A-2a should enable tagged PDF automatically")
	}
}

func TestPdfAValidationNoTitle(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	// No title set.
	doc.SetPdfA(PdfAConfig{Level: PdfA2B})

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err == nil {
		t.Error("expected validation error for missing title")
	}
	if err != nil && !strings.Contains(err.Error(), "Title") {
		t.Errorf("expected title error, got: %v", err)
	}
}

func TestPdfAValidationStandardFont(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Font Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA2B})

	// Add a page with a non-embedded standard font — should fail validation.
	p := doc.AddPage()
	p.AddText("Hello", font.Helvetica, 12, 72, 700)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err == nil {
		t.Error("expected validation error for non-embedded font")
	}
	if err != nil && !strings.Contains(err.Error(), "font") {
		t.Errorf("expected font embedding error, got: %v", err)
	}
}

func TestPdfADisablesEncryption(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "No Encryption"
	doc.encryption = &EncryptionConfig{} // simulate encryption being set
	doc.SetPdfA(PdfAConfig{Level: PdfA2B})

	if doc.encryption != nil {
		t.Error("SetPdfA should clear encryption")
	}
}

func TestPdfA3b(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "PDF/A-3 Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA3B})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "<pdfaid:part>3</pdfaid:part>") {
		t.Error("expected PDF/A part 3")
	}
}

func TestPdfA2bWithLayoutContent(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Layout PDF/A"
	doc.SetPdfA(PdfAConfig{Level: PdfA2B})

	// Layout content with standard fonts goes through the layout engine,
	// which registers fonts on rendered pages.
	// Standard fonts used via layout are registered as fontResource with
	// standard != nil, which triggers the PDF/A validation check.
	// This test verifies the validation catches layout-rendered standard fonts.
	doc.Add(layout.NewParagraph("Hello PDF/A", font.Helvetica, 12))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	// Should fail because Helvetica is a standard font (not embedded).
	if err == nil {
		t.Error("expected validation error for standard font in layout")
	}
}

func TestPdfA2bQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "PDF/A qpdf Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA2B})

	// Add a blank page (no fonts needed).
	doc.AddPage()

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	runQpdfCheck(t, buf.Bytes())
}

func TestPdfAOutputCondition(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Custom Output"
	doc.SetPdfA(PdfAConfig{
		Level:           PdfA2B,
		OutputCondition: "Custom Profile",
	})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "Custom Profile") {
		t.Error("expected custom output condition identifier")
	}
}
