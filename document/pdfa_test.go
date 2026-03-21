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

func TestPdfA1bBasic(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "PDF/A-1b Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA1B})
	doc.AddPage()

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// Must use PDF 1.4 header.
	if !strings.HasPrefix(pdf, "%PDF-1.4") {
		t.Error("expected PDF 1.4 header for PDF/A-1b")
	}

	// Must have part 1 identification.
	if !strings.Contains(pdf, "<pdfaid:part>1</pdfaid:part>") {
		t.Error("expected PDF/A part 1")
	}
	if !strings.Contains(pdf, "<pdfaid:conformance>B</pdfaid:conformance>") {
		t.Error("expected PDF/A conformance B")
	}

	// Must have output intent and metadata.
	if !strings.Contains(pdf, "/OutputIntents") {
		t.Error("expected /OutputIntents in catalog")
	}
	if !strings.Contains(pdf, "/Metadata") {
		t.Error("expected /Metadata in catalog")
	}
}

func TestPdfA1aEnablesTagging(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Tagged PDF/A-1a"
	doc.SetPdfA(PdfAConfig{Level: PdfA1A})

	if !doc.tagged {
		t.Error("PDF/A-1a should enable tagged PDF automatically")
	}
}

func TestPdfA1bForbidsTransparency(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Transparency Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA1B})

	p := doc.AddPage()
	p.SetOpacity(0.5) // this adds an ExtGState

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err == nil {
		t.Error("expected validation error for transparency in PDF/A-1b")
	}
	if err != nil && !strings.Contains(err.Error(), "transparency") {
		t.Errorf("expected transparency error, got: %v", err)
	}
}

func TestPdfA2bAllowsTransparency(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Transparency OK"
	doc.SetPdfA(PdfAConfig{Level: PdfA2B})

	p := doc.AddPage()
	p.SetOpacity(0.5)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("PDF/A-2b should allow transparency, got: %v", err)
	}
}

func TestPdfA1bQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "PDF/A-1b qpdf Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA1B})
	doc.AddPage()

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	runQpdfCheck(t, buf.Bytes())
}

func TestPdfA3bAttachXML(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "PDF/A-3B Attachment Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA3B})

	doc.AttachFile(FileAttachment{
		FileName:       "invoice.xml",
		MIMEType:       "application/xml",
		Description:    "Test XML attachment",
		AFRelationship: "Alternative",
		Data:           []byte(`<?xml version="1.0"?><invoice><id>1</id></invoice>`),
	})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	if !strings.Contains(pdf, "/EmbeddedFiles") {
		t.Error("expected /EmbeddedFiles in output")
	}
	if !strings.Contains(pdf, "/AF ") {
		t.Error("expected /AF in catalog")
	}
	if !strings.Contains(pdf, "/AFRelationship") {
		t.Error("expected /AFRelationship in filespec")
	}
	if !strings.Contains(pdf, "/Alternative") {
		t.Error("expected /Alternative as AFRelationship value")
	}
	if !strings.Contains(pdf, "invoice.xml") {
		t.Error("expected filename in output")
	}
	if !strings.Contains(pdf, "/EmbeddedFile") {
		t.Error("expected /EmbeddedFile stream type")
	}
	if !strings.Contains(pdf, "/UF ") {
		t.Error("expected /UF (Unicode filename) in filespec")
	}
	if !strings.Contains(pdf, "pdfaExtension") {
		t.Error("expected pdfaExtension schema declaration in XMP")
	}
}

func TestPdfA3bAttachMultipleFiles(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Multiple Attachments Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA3B})

	doc.AttachFile(FileAttachment{
		FileName: "first.xml",
		MIMEType: "application/xml",
		Data:     []byte(`<first/>`),
	})
	doc.AttachFile(FileAttachment{
		FileName: "second.xml",
		MIMEType: "application/xml",
		Data:     []byte(`<second/>`),
	})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "first.xml") {
		t.Error("expected first.xml in output")
	}
	if !strings.Contains(pdf, "second.xml") {
		t.Error("expected second.xml in output")
	}
}

func TestPdfA2bRejectsAttachment(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Attachment Rejection Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA2B})

	doc.AttachFile(FileAttachment{
		FileName: "invoice.xml",
		MIMEType: "application/xml",
		Data:     []byte(`<invoice/>`),
	})

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err == nil {
		t.Error("expected error when attaching file to PDF/A-2B document")
	}
	if err != nil && !strings.Contains(err.Error(), "PDF/A-3B") {
		t.Errorf("expected error mentioning PDF/A-3B, got: %v", err)
	}
}

func TestPdfA3bDefaultAFRelationship(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Default AFRelationship Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA3B})

	doc.AttachFile(FileAttachment{
		FileName: "data.xml",
		MIMEType: "application/xml",
		Data:     []byte(`<data/>`),
		// AFRelationship intentionally left empty
	})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	if !strings.Contains(buf.String(), "/Unspecified") {
		t.Error("expected /Unspecified as default AFRelationship")
	}
}

func TestPdfA3bAttachNoDesc(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "No Description Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA3B})

	doc.AttachFile(FileAttachment{
		FileName: "nodesc.xml",
		MIMEType: "application/xml",
		Data:     []byte(`<nodesc/>`),
		// Description intentionally left empty
	})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
}

func TestPdfA3bAttachMIMETypeEncoding(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "MIME Encoding Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA3B})

	doc.AttachFile(FileAttachment{
		FileName: "invoice.xml",
		MIMEType: "application/xml",
		Data:     []byte(`<invoice/>`),
	})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// core.encodeName encodes '/' (a PDF delimiter) as '#2F' (uppercase hex).
	// So "application/xml" passed to NewPdfName must appear as
	// /application#2Fxml in the serialized output.
	if !strings.Contains(pdf, "/application#2Fxml") {
		t.Error("expected MIME type to be serialized as /application#2Fxml in PDF name")
	}
}

func TestPdfA3bXMPExtensionSchema(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "XMP Extension Schema Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA3B})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "http://www.aiim.org/pdfa/ns/extension/") {
		t.Error("expected pdfaExtension namespace in XMP")
	}
	if !strings.Contains(pdf, "http://www.aiim.org/pdfa/ns/f#") {
		t.Error("expected PDF/A-3 file association namespace in XMP")
	}
}

func TestPdfA2bNoXMPExtensionSchema(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "No XMP Extension for 2B"
	doc.SetPdfA(PdfAConfig{Level: PdfA2B})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// PDF/A-2B should not include the PDF/A-3 extension schema block.
	if strings.Contains(buf.String(), "pdfaExtension") {
		t.Error("PDF/A-2B should not contain pdfaExtension schema declaration")
	}
}

func TestSRGBICCProfileValid(t *testing.T) {
	profile := srgbICCProfile()

	// Profile must be larger than the old 128-byte stub.
	if len(profile) < 2000 {
		t.Errorf("expected full ICC profile > 2KB, got %d bytes", len(profile))
	}

	// Verify header fields.
	if string(profile[36:40]) != "acsp" {
		t.Error("missing 'acsp' signature in ICC header")
	}
	if string(profile[12:16]) != "mntr" {
		t.Error("expected 'mntr' device class")
	}
	if string(profile[16:20]) != "RGB " {
		t.Error("expected 'RGB ' color space")
	}

	// Verify tag count (should be 9).
	tagCount := int(profile[128])<<24 | int(profile[129])<<16 | int(profile[130])<<8 | int(profile[131])
	if tagCount != 9 {
		t.Errorf("expected 9 tags, got %d", tagCount)
	}
}

func TestPdfA2bUsesVersion17(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Version Test"
	doc.SetPdfA(PdfAConfig{Level: PdfA2B})
	doc.AddPage()

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	if !strings.HasPrefix(buf.String(), "%PDF-1.7") {
		t.Error("expected PDF 1.7 for PDF/A-2b")
	}
}
