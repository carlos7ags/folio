// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestTextAnnotation(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddText("Hello", font.Helvetica, 12, 72, 700)
	p.AddTextAnnotation([4]float64{72, 700, 92, 720}, "This is a sticky note", "Comment")

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Subtype /Text") {
		t.Error("expected /Subtype /Text for sticky note annotation")
	}
	if !strings.Contains(pdf, "/Name /Comment") {
		t.Error("expected /Name /Comment")
	}
	if !strings.Contains(pdf, "/Contents") {
		t.Error("expected /Contents with note text")
	}
}

func TestTextAnnotationOpen(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddTextAnnotationOpen([4]float64{100, 700, 120, 720}, "Open note", "Note")

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Open true") {
		t.Error("expected /Open true for initially open annotation")
	}
}

func TestTextAnnotationDefaultIcon(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddTextAnnotation([4]float64{100, 700, 120, 720}, "Default icon", "")

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Name /Note") {
		t.Error("empty icon should default to /Note")
	}
}

func TestHighlightAnnotation(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddText("Highlighted text", font.Helvetica, 12, 72, 700)
	p.AddHighlight(
		[4]float64{72, 695, 200, 715},
		[3]float64{1, 1, 0}, // yellow
		nil,                 // auto quad from rect
	)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Subtype /Highlight") {
		t.Error("expected /Subtype /Highlight")
	}
	if !strings.Contains(pdf, "/C") {
		t.Error("expected /C color array")
	}
	if !strings.Contains(pdf, "/QuadPoints") {
		t.Error("expected /QuadPoints for text markup")
	}
}

func TestUnderlineAnnotation(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddText("Underlined text", font.Helvetica, 12, 72, 700)
	p.AddUnderline(
		[4]float64{72, 695, 200, 715},
		[3]float64{1, 0, 0}, // red
		nil,
	)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Subtype /Underline") {
		t.Error("expected /Subtype /Underline")
	}
}

func TestSquigglyAnnotation(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddSquiggly(
		[4]float64{72, 695, 200, 715},
		[3]float64{0, 0, 1}, // blue
		nil,
	)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	if !strings.Contains(buf.String(), "/Subtype /Squiggly") {
		t.Error("expected /Subtype /Squiggly")
	}
}

func TestStrikeOutAnnotation(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddStrikeOut(
		[4]float64{72, 695, 200, 715},
		[3]float64{1, 0, 0},
		nil,
	)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	if !strings.Contains(buf.String(), "/Subtype /StrikeOut") {
		t.Error("expected /Subtype /StrikeOut")
	}
}

func TestTextMarkupGeneric(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddTextMarkup(MarkupStrikeOut,
		[4]float64{72, 695, 200, 715},
		[3]float64{1, 0, 0},
		nil,
	)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	if !strings.Contains(buf.String(), "/Subtype /StrikeOut") {
		t.Error("expected /Subtype /StrikeOut from generic AddTextMarkup")
	}
}

func TestHighlightWithCustomQuadPoints(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()

	quads := [][8]float64{
		{72, 700, 200, 700, 72, 715, 200, 715},
		{72, 680, 150, 680, 72, 695, 150, 695},
	}
	p.AddHighlight(
		[4]float64{72, 680, 200, 715},
		[3]float64{1, 1, 0},
		quads,
	)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/QuadPoints") {
		t.Error("expected /QuadPoints")
	}
}

func TestAnnotationColorOnLink(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddLink([4]float64{72, 700, 200, 715}, "https://example.com")

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// Links should still work with the updated serialization.
	pdf := buf.String()
	if !strings.Contains(pdf, "/Subtype /Link") {
		t.Error("expected /Subtype /Link")
	}
	if !strings.Contains(pdf, "/URI") {
		t.Error("expected /URI action")
	}
}

func TestMultipleAnnotationTypes(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.AddText("Some text", font.Helvetica, 12, 72, 700)
	p.AddLink([4]float64{72, 700, 200, 715}, "https://example.com")
	p.AddTextAnnotation([4]float64{210, 700, 230, 720}, "A note", "Comment")
	p.AddHighlight([4]float64{72, 695, 200, 715}, [3]float64{1, 1, 0}, nil)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Subtype /Link") {
		t.Error("missing Link annotation")
	}
	if !strings.Contains(pdf, "/Subtype /Text") {
		t.Error("missing Text annotation")
	}
	if !strings.Contains(pdf, "/Subtype /Highlight") {
		t.Error("missing Highlight annotation")
	}
}
