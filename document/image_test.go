// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	goimage "image"
	"image/color"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
	"github.com/carlos7ags/folio/layout"
)

func createTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := goimage.NewRGBA(goimage.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	return buf.Bytes()
}

func createTestPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := goimage.NewRGBA(goimage.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 0, G: 128, B: 255, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestPageAddImageJPEG(t *testing.T) {
	jpegData := createTestJPEG(t, 100, 50)
	img, err := folioimage.NewJPEG(jpegData)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}

	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddImage(img, 72, 600, 200, 100)

	var buf bytes.Buffer
	_, err = doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Subtype /Image") {
		t.Error("missing /Subtype /Image")
	}
	if !strings.Contains(pdf, "/Filter /DCTDecode") {
		t.Error("missing /Filter /DCTDecode")
	}
	if !strings.Contains(pdf, "/XObject") {
		t.Error("missing /XObject in resources")
	}
	if !strings.Contains(pdf, "Do") {
		t.Error("missing Do operator in content stream")
	}
}

func TestPageAddImagePNG(t *testing.T) {
	pngData := createTestPNG(t, 80, 60)
	img, err := folioimage.NewPNG(pngData)
	if err != nil {
		t.Fatalf("NewPNG: %v", err)
	}

	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddImage(img, 72, 600, 200, 0) // auto height

	var buf bytes.Buffer
	_, err = doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Subtype /Image") {
		t.Error("missing /Subtype /Image")
	}
	if !strings.Contains(pdf, "/XObject") {
		t.Error("missing /XObject in resources")
	}
}

func TestPageAddImageAutoSize(t *testing.T) {
	jpegData := createTestJPEG(t, 200, 100)
	img, err := folioimage.NewJPEG(jpegData)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}

	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddImage(img, 72, 600, 0, 0) // natural size

	var buf bytes.Buffer
	_, err = doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	// Should have Do operator with the image dimensions.
	pdf := buf.String()
	if !strings.Contains(pdf, "200 0 0 100") {
		t.Error("expected natural size 200x100 in cm operator")
	}
}

func TestImageWithTextQpdfCheck(t *testing.T) {
	jpegData := createTestJPEG(t, 100, 50)
	img, err := folioimage.NewJPEG(jpegData)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}

	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddText("Hello with image", font.Helvetica, 18, 72, 720)
	p.AddImage(img, 72, 600, 200, 100)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestPNGImageQpdfCheck(t *testing.T) {
	pngData := createTestPNG(t, 80, 60)
	img, err := folioimage.NewPNG(pngData)
	if err != nil {
		t.Fatalf("NewPNG: %v", err)
	}

	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddImage(img, 72, 600, 300, 0)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestImageLayoutElementQpdfCheck(t *testing.T) {
	jpegData := createTestJPEG(t, 200, 100)
	img, err := folioimage.NewJPEG(jpegData)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}

	doc := NewDocument(PageSizeLetter)
	doc.Add(layout.NewParagraph("Image in layout flow:", font.Helvetica, 14))
	doc.Add(layout.NewImageElement(img))
	doc.Add(layout.NewParagraph("Text after the image.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}

func TestMixedContentQpdfCheck(t *testing.T) {
	jpegData := createTestJPEG(t, 150, 100)
	jpegImg, err := folioimage.NewJPEG(jpegData)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}

	pngData := createTestPNG(t, 80, 80)
	pngImg, err := folioimage.NewPNG(pngData)
	if err != nil {
		t.Fatalf("NewPNG: %v", err)
	}

	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Mixed Content Test"

	doc.Add(layout.NewParagraph("Report with Images", font.HelveticaBold, 18).SetAlign(layout.AlignCenter))
	doc.Add(layout.NewImageElement(jpegImg).SetSize(300, 0))

	tbl := layout.NewTable()
	h := tbl.AddHeaderRow()
	h.AddCell("Item", font.HelveticaBold, 10)
	h.AddCell("Price", font.HelveticaBold, 10)
	r := tbl.AddRow()
	r.AddCell("Widget", font.Helvetica, 10)
	r.AddCell("$9.99", font.Helvetica, 10)
	doc.Add(tbl)

	doc.Add(layout.NewImageElement(pngImg).SetSize(200, 200))
	doc.Add(layout.NewParagraph("End of report.", font.Helvetica, 11))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}
