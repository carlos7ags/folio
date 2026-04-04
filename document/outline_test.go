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

func TestAddOutlineFit(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p1 := doc.AddPage()
	p1.AddText("Chapter 1", font.Helvetica, 24, 72, 720)
	p2 := doc.AddPage()
	p2.AddText("Chapter 2", font.Helvetica, 24, 72, 720)

	doc.AddOutline("Chapter 1", FitDest(0))
	doc.AddOutline("Chapter 2", FitDest(1))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Type /Outlines") {
		t.Error("missing /Type /Outlines")
	}
	if !strings.Contains(pdf, "/Title (Chapter 1)") {
		t.Error("missing outline title 'Chapter 1'")
	}
	if !strings.Contains(pdf, "/Title (Chapter 2)") {
		t.Error("missing outline title 'Chapter 2'")
	}
	if !strings.Contains(pdf, "/Fit") {
		t.Error("missing /Fit destination")
	}
	if !strings.Contains(pdf, "/Outlines") {
		t.Error("catalog missing /Outlines reference")
	}
}

func TestAddOutlineXYZ(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()

	doc.AddOutline("Section 1", XYZDest(0, 72, 500, 1.5))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/XYZ") {
		t.Error("missing /XYZ destination")
	}
}

func TestAddOutlineNested(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()
	doc.AddPage()
	doc.AddPage()

	ch1 := doc.AddOutline("Chapter 1", FitDest(0))
	ch1.AddChild("Section 1.1", FitDest(0))
	ch1.AddChild("Section 1.2", FitDest(1))
	doc.AddOutline("Chapter 2", FitDest(2))

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Title (Section 1.1)") {
		t.Error("missing nested outline 'Section 1.1'")
	}
	if !strings.Contains(pdf, "/Title (Section 1.2)") {
		t.Error("missing nested outline 'Section 1.2'")
	}
	// Count should be 4 (2 top-level + 2 children)
	if !strings.Contains(pdf, "/Count 4") {
		t.Error("outline count should be 4")
	}
}

func TestNoOutlinesOmitted(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if strings.Contains(pdf, "/Outlines") {
		t.Error("catalog should not have /Outlines when none are added")
	}
}

func TestPageRemove(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()
	doc.AddPage()
	doc.AddPage()

	if doc.PageCount() != 3 {
		t.Fatalf("expected 3 pages, got %d", doc.PageCount())
	}

	// Remove middle page
	if err := doc.RemovePage(1); err != nil {
		t.Fatalf("RemovePage failed: %v", err)
	}
	if doc.PageCount() != 2 {
		t.Errorf("expected 2 pages after removal, got %d", doc.PageCount())
	}

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	if !strings.Contains(buf.String(), "/Count 2") {
		t.Error("page count should be 2 after removal")
	}
}

func TestPageRemoveFirst(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()
	doc.AddPage()

	if err := doc.RemovePage(0); err != nil {
		t.Fatalf("RemovePage(0) failed: %v", err)
	}
	if doc.PageCount() != 1 {
		t.Errorf("expected 1 page, got %d", doc.PageCount())
	}
}

func TestPageRemoveLast(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()
	doc.AddPage()

	if err := doc.RemovePage(1); err != nil {
		t.Fatalf("RemovePage(1) failed: %v", err)
	}
	if doc.PageCount() != 1 {
		t.Errorf("expected 1 page, got %d", doc.PageCount())
	}
}

func TestPageRemoveOnly(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()

	if err := doc.RemovePage(0); err != nil {
		t.Fatalf("RemovePage(0) failed: %v", err)
	}
	if doc.PageCount() != 0 {
		t.Errorf("expected 0 pages, got %d", doc.PageCount())
	}
}

func TestPageRemoveOutOfRange(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()

	if err := doc.RemovePage(-1); err == nil {
		t.Error("RemovePage(-1) should fail")
	}
	if err := doc.RemovePage(1); err == nil {
		t.Error("RemovePage(1) should fail for 1-page doc")
	}
	if err := doc.RemovePage(100); err == nil {
		t.Error("RemovePage(100) should fail")
	}
}

func TestPageAccessor(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	got, err := doc.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	if got != p {
		t.Error("Page(0) should return the page we added")
	}
}

func TestPageAccessorOutOfRange(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()
	if _, err := doc.Page(-1); err == nil {
		t.Error("expected error for negative index")
	}
	if _, err := doc.Page(1); err == nil {
		t.Error("expected error for index >= page count")
	}
}

func TestToBytes(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Add(layout.NewParagraph("Hello", font.Helvetica, 12))

	pdf, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("expected non-empty bytes")
	}
	if string(pdf[:5]) != "%PDF-" {
		t.Errorf("expected PDF header, got %q", string(pdf[:5]))
	}
}

func TestToBytesEmpty(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	pdf, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes on empty doc: %v", err)
	}
	if len(pdf) == 0 {
		t.Fatal("expected non-empty bytes even for empty doc")
	}
	if string(pdf[:5]) != "%PDF-" {
		t.Errorf("empty doc should still have PDF header, got %q", string(pdf[:5]))
	}
}

func TestToBytesMatchesWriteTo(t *testing.T) {
	// ToBytes must produce identical output to WriteTo — this is the core contract.
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Equivalence Test"
	doc.Add(layout.NewParagraph("Hello", font.Helvetica, 12))

	toBytes, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	if !bytes.Equal(toBytes, buf.Bytes()) {
		t.Errorf("ToBytes and WriteTo produced different output: %d vs %d bytes", len(toBytes), buf.Len())
	}
}

func TestToBytesDeterministic(t *testing.T) {
	// Same document called twice — verifies no mutation on serialization.
	doc := NewDocument(PageSizeLetter)
	doc.Add(layout.NewParagraph("Deterministic", font.Helvetica, 12))

	pdf1, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("first ToBytes: %v", err)
	}
	pdf2, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("second ToBytes: %v", err)
	}
	if !bytes.Equal(pdf1, pdf2) {
		t.Errorf("ToBytes not idempotent: %d vs %d bytes", len(pdf1), len(pdf2))
	}
}

func TestToBytesMultiPage(t *testing.T) {
	doc := NewDocument(PageSize{Width: 612, Height: 100}) // very short pages
	for range 20 {
		doc.Add(layout.NewParagraph("Line of text on a very short page.", font.Helvetica, 12))
	}
	pdf, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes multi-page: %v", err)
	}
	if string(pdf[:5]) != "%PDF-" {
		t.Error("multi-page PDF missing header")
	}
}

func TestToBytesWithMetadata(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Test Invoice"
	doc.Info.Author = "Folio Test Author"
	doc.Add(layout.NewParagraph("Content", font.Helvetica, 12))

	pdf, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	s := string(pdf)
	if !strings.Contains(s, "Test Invoice") {
		t.Error("expected title in PDF metadata")
	}
	if !strings.Contains(s, "Folio Test Author") {
		t.Error("expected author in PDF metadata")
	}
}

func TestM3QpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Milestone 3 Test"
	doc.Info.Author = "Folio"
	doc.Info.Producer = "Folio PDF Library"

	p1 := doc.AddPage()
	p1.AddText("Chapter 1: Introduction", font.Helvetica, 24, 72, 720)
	p1.AddText("This is the first chapter.", font.Helvetica, 12, 72, 690)

	p2 := doc.AddPage()
	p2.AddText("Chapter 2: Details", font.Helvetica, 24, 72, 720)

	p3 := doc.AddPage()
	p3.AddText("Appendix", font.Helvetica, 24, 72, 720)

	ch1 := doc.AddOutline("Chapter 1: Introduction", FitDest(0))
	ch1.AddChild("Section 1.1", XYZDest(0, 72, 690, 0))
	doc.AddOutline("Chapter 2: Details", FitDest(1))
	doc.AddOutline("Appendix", FitDest(2))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}
