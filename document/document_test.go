// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"math"
	"os"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
)

// testFontPath returns a TTF font path available on this system.
func testFontPath(t *testing.T) string {
	t.Helper()
	candidates := []string{
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/System/Library/Fonts/Supplemental/Courier New.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	t.Skip("no suitable TTF font found")
	return ""
}

func TestAddTextSingleLine(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.AddText("Hello World", font.Helvetica, 12, 100, 700)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// Should have a content stream with text operators
	if !strings.Contains(pdf, "BT") {
		t.Error("missing BT operator")
	}
	if !strings.Contains(pdf, "ET") {
		t.Error("missing ET operator")
	}
	if !strings.Contains(pdf, "/F1 12 Tf") {
		t.Error("missing Tf operator")
	}
	if !strings.Contains(pdf, "100 700 Td") {
		t.Error("missing Td operator")
	}
	if !strings.Contains(pdf, "(Hello World) Tj") {
		t.Error("missing Tj operator")
	}

	// Should have font resource
	if !strings.Contains(pdf, "/BaseFont /Helvetica") {
		t.Error("missing Helvetica font object")
	}
	if !strings.Contains(pdf, "/Type /Font") {
		t.Error("missing font type")
	}

	// Page should reference resources and contents
	if !strings.Contains(pdf, "/Resources") {
		t.Error("missing /Resources on page")
	}
	if !strings.Contains(pdf, "/Contents") {
		t.Error("missing /Contents on page")
	}
}

func TestAddTextMultipleFonts(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.AddText("Normal text", font.Helvetica, 12, 72, 720)
	page.AddText("Bold text", font.HelveticaBold, 14, 72, 700)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// Should have both fonts
	if !strings.Contains(pdf, "/BaseFont /Helvetica ") || !strings.Contains(pdf, "/BaseFont /Helvetica-Bold") {
		// Check without trailing space too
		if !strings.Contains(pdf, "/BaseFont /Helvetica") {
			t.Error("missing Helvetica font")
		}
		if !strings.Contains(pdf, "/BaseFont /Helvetica-Bold") {
			t.Error("missing Helvetica-Bold font")
		}
	}

	// Should have two BT/ET blocks
	if strings.Count(pdf, "BT") != 2 {
		t.Errorf("expected 2 BT operators, got %d", strings.Count(pdf, "BT"))
	}
}

func TestAddTextSameFontReusesResource(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.AddText("Line 1", font.Helvetica, 12, 72, 720)
	page.AddText("Line 2", font.Helvetica, 12, 72, 700)

	// Should only register Helvetica once (two AddText calls, same font)
	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	pdf := buf.String()
	// Should have only one /BaseFont /Helvetica font object
	if strings.Count(pdf, "/BaseFont /Helvetica") != 1 {
		t.Errorf("expected 1 Helvetica font object, got %d", strings.Count(pdf, "/BaseFont /Helvetica"))
	}
}

func TestAddTextMultiplePages(t *testing.T) {
	doc := NewDocument(PageSizeLetter)

	p1 := doc.AddPage()
	p1.AddText("Page 1", font.Helvetica, 12, 72, 720)

	p2 := doc.AddPage()
	p2.AddText("Page 2", font.TimesBold, 16, 72, 720)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	if !strings.Contains(pdf, "(Page 1) Tj") {
		t.Error("missing page 1 text")
	}
	if !strings.Contains(pdf, "(Page 2) Tj") {
		t.Error("missing page 2 text")
	}
	if !strings.Contains(pdf, "/BaseFont /Helvetica") {
		t.Error("missing Helvetica")
	}
	if !strings.Contains(pdf, "/BaseFont /Times-Bold") {
		t.Error("missing Times-Bold")
	}
	if !strings.Contains(pdf, "/Count 2") {
		t.Error("page count should be 2")
	}
}

func TestBlankPageStillWorks(t *testing.T) {
	// A page with no text should have no /Contents but should
	// have an empty /Resources (required by spec, qpdf warns otherwise).
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if strings.Contains(pdf, "/Contents") {
		t.Error("blank page should not have /Contents")
	}
	if !strings.Contains(pdf, "/Resources") {
		t.Error("blank page should have /Resources (even if empty)")
	}
}

func TestTextEscapingInDocument(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.AddText("Price: $100 (net)", font.Helvetica, 12, 72, 720)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, `(Price: $100 \(net\)) Tj`) {
		t.Errorf("text not properly escaped in PDF output:\n%s", pdf)
	}
}

func TestSaveTextPDF(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.AddText("Hello World", font.Helvetica, 24, 72, 720)

	tmpFile := t.TempDir() + "/text.pdf"
	err := doc.Save(tmpFile)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.HasPrefix(string(data), "%PDF-1.7") {
		t.Error("saved file missing PDF header")
	}
	if !strings.Contains(string(data), "(Hello World) Tj") {
		t.Error("saved file missing text content")
	}
}

func TestTextPDFQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.AddText("Hello World", font.Helvetica, 24, 72, 720)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

// --- Embedded font tests ---

func TestAddTextEmbedded(t *testing.T) {
	path := testFontPath(t)
	face, err := font.LoadTTF(path)
	if err != nil {
		t.Fatalf("LoadTTF failed: %v", err)
	}
	ef := font.NewEmbeddedFont(face)

	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.AddTextEmbedded("Hello World", ef, 24, 72, 720)

	var buf bytes.Buffer
	_, err = doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// Should have Type0 composite font
	if !strings.Contains(pdf, "/Subtype /Type0") {
		t.Error("missing /Subtype /Type0")
	}
	if !strings.Contains(pdf, "/Encoding /Identity-H") {
		t.Error("missing /Encoding /Identity-H")
	}
	// Should have CIDFont descendant
	if !strings.Contains(pdf, "/Subtype /CIDFontType2") {
		t.Error("missing CIDFontType2")
	}
	// Should have font descriptor
	if !strings.Contains(pdf, "/Type /FontDescriptor") {
		t.Error("missing FontDescriptor")
	}
	// Should have embedded font stream
	if !strings.Contains(pdf, "/FontFile2") {
		t.Error("missing /FontFile2")
	}
	// Should have ToUnicode reference on the Type0 font
	if !strings.Contains(pdf, "/ToUnicode") {
		t.Error("missing /ToUnicode reference")
	}
	// Content stream should use hex encoding
	if !strings.Contains(pdf, "> Tj") {
		t.Error("missing hex-encoded Tj operator")
	}
}

func TestEmbeddedFontMixedWithStandard(t *testing.T) {
	path := testFontPath(t)
	face, err := font.LoadTTF(path)
	if err != nil {
		t.Fatalf("LoadTTF failed: %v", err)
	}
	ef := font.NewEmbeddedFont(face)

	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.AddText("Standard font text", font.Helvetica, 12, 72, 750)
	page.AddTextEmbedded("Embedded font text", ef, 12, 72, 720)

	var buf bytes.Buffer
	_, err = doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// Should have both standard and embedded fonts
	if !strings.Contains(pdf, "/BaseFont /Helvetica") {
		t.Error("missing standard Helvetica font")
	}
	if !strings.Contains(pdf, "/Subtype /Type0") {
		t.Error("missing embedded Type0 font")
	}
}

func TestEmbeddedFontSpecialChars(t *testing.T) {
	path := testFontPath(t)
	face, err := font.LoadTTF(path)
	if err != nil {
		t.Fatalf("LoadTTF failed: %v", err)
	}
	ef := font.NewEmbeddedFont(face)

	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.AddTextEmbedded("Price: $100 (net) — 50% off!", ef, 12, 72, 720)

	var buf bytes.Buffer
	_, err = doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// Should not crash and should produce valid PDF structure
	pdf := buf.String()
	if !strings.HasPrefix(pdf, "%PDF-1.7") {
		t.Error("missing PDF header")
	}
}

func TestEmbeddedFontQpdfCheck(t *testing.T) {
	path := testFontPath(t)
	face, err := font.LoadTTF(path)
	if err != nil {
		t.Fatalf("LoadTTF failed: %v", err)
	}
	ef := font.NewEmbeddedFont(face)

	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.AddTextEmbedded("Hello World — Embedded Font Test", ef, 24, 72, 720)
	page.AddTextEmbedded("Second line with more text.", ef, 12, 72, 690)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestEmbeddedFontReusesResource(t *testing.T) {
	path := testFontPath(t)
	face, err := font.LoadTTF(path)
	if err != nil {
		t.Fatalf("LoadTTF failed: %v", err)
	}
	ef := font.NewEmbeddedFont(face)

	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.AddTextEmbedded("Line 1", ef, 12, 72, 720)
	page.AddTextEmbedded("Line 2", ef, 12, 72, 700)

	var buf bytes.Buffer
	_, err = doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// Should only have one Type0 font (reused for both lines)
	pdf := buf.String()
	count := strings.Count(pdf, "/Subtype /Type0")
	if count != 1 {
		t.Errorf("expected 1 Type0 font, got %d", count)
	}
}

func TestEmbeddedFontSubsetTag(t *testing.T) {
	path := testFontPath(t)
	face, err := font.LoadTTF(path)
	if err != nil {
		t.Fatalf("LoadTTF failed: %v", err)
	}
	ef := font.NewEmbeddedFont(face)

	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.AddTextEmbedded("Subset", ef, 12, 72, 720)

	var buf bytes.Buffer
	_, err = doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// The PDF should contain a subset tag like "ABCDEF+FontName"
	pdf := buf.String()
	// Look for the pattern: 6 uppercase letters followed by "+"
	found := false
	for i := 0; i+7 < len(pdf); i++ {
		if pdf[i+6] == '+' {
			allUpper := true
			for j := 0; j < 6; j++ {
				if pdf[i+j] < 'A' || pdf[i+j] > 'Z' {
					allUpper = false
					break
				}
			}
			if allUpper {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected subset tag (ABCDEF+FontName) in PDF output")
	}
}

// --- Input validation tests ---

func TestAddTextNilFontPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic for nil font")
		}
	}()
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddText("test", nil, 12, 72, 720)
}

func TestAddTextNegativeSizePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic for negative size")
		}
	}()
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddText("test", font.Helvetica, -1, 72, 720)
}

func TestAddTextEmbeddedNilFontPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic for nil embedded font")
		}
	}()
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddTextEmbedded("test", nil, 12, 72, 720)
}

func TestAddImageNilPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic for nil image")
		}
	}()
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddImage(nil, 72, 600, 200, 100)
}

func TestAddImageNegativeDimensionsPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("expected panic for negative dimensions")
		}
	}()
	img := createTestImageForValidation(t)
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddImage(img, 72, 600, -100, 100)
}

func createTestImageForValidation(t *testing.T) *folioimage.Image {
	t.Helper()
	data := createTestJPEG(t, 10, 10)
	img, err := folioimage.NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}
	return img
}

func TestAddTextZeroSizeAllowed(t *testing.T) {
	// Size 0 is technically valid (invisible text, used for search-only text).
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddText("invisible", font.Helvetica, 0, 72, 720)
}

// --- Sprint C: Annotations, links, named destinations, page rotation ---

func TestPageAddLink(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddText("Click here", font.Helvetica, 12, 72, 720)
	p.AddLink([4]float64{72, 710, 200, 730}, "https://example.com")

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Subtype /Link") {
		t.Error("missing link annotation subtype")
	}
	if !strings.Contains(pdf, "/URI") {
		t.Error("missing URI action")
	}
	if !strings.Contains(pdf, "example.com") {
		t.Error("missing URI value")
	}
	if !strings.Contains(pdf, "/Annots") {
		t.Error("missing Annots entry on page")
	}
}

func TestPageAddPageLink(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p1 := doc.AddPage()
	p1.AddText("Go to page 2", font.Helvetica, 12, 72, 720)
	p1.AddPageLink([4]float64{72, 710, 200, 730}, 1)

	p2 := doc.AddPage()
	p2.AddText("Page 2", font.Helvetica, 12, 72, 720)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Dest") {
		t.Error("missing Dest entry for page link")
	}
	if !strings.Contains(pdf, "/Fit") {
		t.Error("missing Fit in destination")
	}
}

func TestPageAddInternalLink(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddText("Jump to section", font.Helvetica, 12, 72, 720)
	p.AddInternalLink([4]float64{72, 710, 200, 730}, "section1")

	doc.AddNamedDest(NamedDest{
		Name:      "section1",
		PageIndex: 0,
		FitType:   "FitH",
		Top:       700,
	})

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	// Internal links with resolvable named destinations use a direct
	// /Dest [pageRef /Fit] array for maximum viewer compatibility,
	// rather than a /GoTo action with a string destination name.
	if !strings.Contains(pdf, "/Dest") {
		t.Error("missing /Dest entry for internal link")
	}
	if !strings.Contains(pdf, "/Dests") {
		t.Error("missing Dests in catalog")
	}
}

func TestNamedDestFit(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage().AddText("Page 1", font.Helvetica, 12, 72, 720)
	doc.AddNamedDest(NamedDest{Name: "top", PageIndex: 0})

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	if !strings.Contains(buf.String(), "/Fit") {
		t.Error("default fit type should be Fit")
	}
}

func TestNamedDestXYZ(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage().AddText("Page 1", font.Helvetica, 12, 72, 720)
	doc.AddNamedDest(NamedDest{
		Name: "pos", PageIndex: 0,
		FitType: "XYZ", Left: 72, Top: 700, Zoom: 1.5,
	})

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	pdf := buf.String()
	if !strings.Contains(pdf, "/XYZ") {
		t.Error("missing XYZ fit type")
	}
}

func TestNamedDestOutOfRange(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage().AddText("Page 1", font.Helvetica, 12, 72, 720)
	doc.AddNamedDest(NamedDest{Name: "bad", PageIndex: 99})

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	// Should not crash; invalid page index is silently skipped.
}

func TestPageRotation(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddText("Rotated", font.Helvetica, 12, 72, 720)
	p.SetRotate(90)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Rotate 90") {
		t.Error("missing /Rotate 90 in PDF")
	}
}

func TestPageRotationZero(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage().AddText("Normal", font.Helvetica, 12, 72, 720)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// Rotation 0 should not add /Rotate entry.
	if strings.Contains(buf.String(), "/Rotate") {
		t.Error("should not include /Rotate for 0 degrees")
	}
}

func TestAnnotationsQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Annotations Test"

	p1 := doc.AddPage()
	p1.AddText("External link", font.Helvetica, 12, 72, 720)
	p1.AddLink([4]float64{72, 710, 200, 730}, "https://example.com")

	p1.AddText("Internal link", font.Helvetica, 12, 72, 690)
	p1.AddInternalLink([4]float64{72, 680, 200, 700}, "page2top")

	p1.AddText("Page link", font.Helvetica, 12, 72, 660)
	p1.AddPageLink([4]float64{72, 650, 200, 670}, 1)

	p2 := doc.AddPage()
	p2.AddText("Page 2", font.HelveticaBold, 18, 72, 720)
	p2.SetRotate(90)

	doc.AddNamedDest(NamedDest{
		Name: "page2top", PageIndex: 1, FitType: "FitH", Top: 792,
	})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

// --- Sprint D: Transparency/opacity ---

func TestSetOpacity(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	name := page.SetOpacity(0.5)
	if name != "GS1" {
		t.Errorf("expected GS1, got %s", name)
	}
	if len(page.extGStates) != 1 {
		t.Fatalf("expected 1 ExtGState, got %d", len(page.extGStates))
	}
	stream := string(page.ContentStream().Bytes())
	if !strings.Contains(stream, "/GS1 gs") {
		t.Errorf("stream should contain '/GS1 gs', got: %s", stream)
	}
}

func TestSetOpacityFillStroke(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	name := page.SetOpacityFillStroke(0.3, 0.7)
	if name != "GS1" {
		t.Errorf("expected GS1, got %s", name)
	}

	name2 := page.SetOpacity(1.0)
	if name2 != "GS2" {
		t.Errorf("expected GS2, got %s", name2)
	}
}

func TestOpacitySave(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	page.SetOpacity(0.5)
	page.AddText("Semi-transparent", font.Helvetica, 12, 72, 720)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	pdfOutput := buf.String()
	if !strings.Contains(pdfOutput, "/ExtGState") {
		t.Error("PDF output should contain /ExtGState in Resources")
	}
	if !strings.Contains(pdfOutput, "/GS1") {
		t.Error("PDF output should contain /GS1 reference")
	}
}

func TestOpacityQPDF(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()

	page.SetOpacity(0.5)
	page.AddText("Semi-transparent text", font.Helvetica, 14, 72, 700)

	page.SetOpacity(1.0)
	page.AddText("Opaque text", font.Helvetica, 14, 72, 680)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestOpacityClampOutOfRange(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()

	// Negative alpha should clamp to 0.
	page.SetOpacity(-0.5)
	page.AddText("Invisible", font.Helvetica, 12, 72, 700)

	// Alpha > 1 should clamp to 1.
	page.SetOpacity(1.5)
	page.AddText("Opaque", font.Helvetica, 12, 72, 680)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	// Should produce valid PDF without negative or >1 values.
	pdfStr := buf.String()
	if strings.Contains(pdfStr, "-0.5") {
		t.Error("negative alpha should be clamped, but -0.5 found in output")
	}
	if strings.Contains(pdfStr, "1.5") {
		t.Error("alpha > 1 should be clamped, but 1.5 found in output")
	}
}

func TestOpacityNaN(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()
	// NaN should be treated as 1.0 (fully opaque).
	page.SetOpacityFillStroke(math.NaN(), math.Inf(1))
	page.AddText("Safe text", font.Helvetica, 12, 72, 700)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
}

func TestOpacityEdgeValues(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	page := doc.AddPage()

	// Exact 0 and 1 should work.
	page.SetOpacity(0)
	page.AddText("Fully transparent", font.Helvetica, 12, 72, 700)
	page.SetOpacity(1)
	page.AddText("Fully opaque", font.Helvetica, 12, 72, 680)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
}

// --- Per-page size override tests ---

func TestPageCustomSize(t *testing.T) {
	// Create a Letter-sized document, then add an A4-sized page.
	doc := NewDocument(PageSizeLetter)

	// Page 1: default Letter size (612 x 792).
	p1 := doc.AddPage()
	p1.AddText("Letter page", font.Helvetica, 12, 72, 720)

	// Page 2: override to A4 size (595.28 x 841.89).
	p2 := doc.AddPage()
	p2.SetSize(PageSizeA4)
	p2.AddText("A4 page", font.Helvetica, 12, 72, 720)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// Should contain Letter MediaBox dimensions (612, 792).
	if !strings.Contains(pdf, "612") || !strings.Contains(pdf, "792") {
		t.Error("missing Letter page dimensions (612 x 792) in PDF output")
	}

	// Should contain A4 MediaBox dimensions (595.28, 841.89).
	if !strings.Contains(pdf, "595.28") || !strings.Contains(pdf, "841.89") {
		t.Error("missing A4 page dimensions (595.28 x 841.89) in PDF output")
	}

	// Both pages should exist.
	if !strings.Contains(pdf, "/Count 2") {
		t.Error("expected 2 pages")
	}
}

// --- Watermark tests ---

func TestWatermarkBasic(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetWatermark("DRAFT")

	page := doc.AddPage()
	page.AddText("Hello World", font.Helvetica, 12, 72, 720)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// The watermark text should appear in the PDF content stream.
	if !strings.Contains(pdf, "(DRAFT) Tj") {
		t.Error("missing watermark text '(DRAFT) Tj' in PDF output")
	}

	// Should have ExtGState for opacity.
	if !strings.Contains(pdf, "/ExtGState") {
		t.Error("missing /ExtGState in Resources for watermark opacity")
	}

	// Page content should also be present.
	if !strings.Contains(pdf, "(Hello World) Tj") {
		t.Error("missing page content text")
	}
}

func TestWatermarkCustomConfig(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetWatermarkConfig(WatermarkConfig{
		Text:     "CONFIDENTIAL",
		FontSize: 80,
		ColorR:   1.0,
		ColorG:   0.0,
		ColorB:   0.0,
		Angle:    30,
		Opacity:  0.5,
	})

	page := doc.AddPage()
	page.AddText("Secret stuff", font.Helvetica, 12, 72, 720)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// The custom watermark text should appear.
	if !strings.Contains(pdf, "(CONFIDENTIAL) Tj") {
		t.Error("missing watermark text '(CONFIDENTIAL) Tj' in PDF output")
	}

	// Should have custom font size (80).
	if !strings.Contains(pdf, "80 Tf") {
		t.Error("missing custom font size '80 Tf' in PDF output")
	}

	// Should have custom red color (1 0 0 rg).
	if !strings.Contains(pdf, "1 0 0 rg") {
		t.Error("missing custom red color '1 0 0 rg' in PDF output")
	}

	// Page content should also be present.
	if !strings.Contains(pdf, "(Secret stuff) Tj") {
		t.Error("missing page content text")
	}
}

func TestWatermarkMultiplePages(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetWatermark("SAMPLE")

	doc.AddPage().AddText("Page 1", font.Helvetica, 12, 72, 720)
	doc.AddPage().AddText("Page 2", font.Helvetica, 12, 72, 720)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// Watermark should appear twice (once per page).
	count := strings.Count(pdf, "(SAMPLE) Tj")
	if count != 2 {
		t.Errorf("expected watermark on 2 pages, got %d occurrences", count)
	}
}

func TestPageDefaultSize(t *testing.T) {
	// Verify that a page without SetSize uses the document's page size.
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddText("Default size", font.Helvetica, 12, 72, 720)

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// Should contain Letter MediaBox dimensions.
	if !strings.Contains(pdf, "612") {
		t.Error("missing Letter width (612) in PDF output")
	}
	if !strings.Contains(pdf, "792") {
		t.Error("missing Letter height (792) in PDF output")
	}

	// Should NOT contain A4-specific dimensions.
	if strings.Contains(pdf, "595.28") {
		t.Error("should not contain A4 width (595.28) when using default Letter size")
	}
	if strings.Contains(pdf, "841.89") {
		t.Error("should not contain A4 height (841.89) when using default Letter size")
	}
}
