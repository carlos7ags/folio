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

func TestTaggedPDFBasic(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetTagged(true)

	doc.Add(layout.NewHeading("Title", layout.H1))
	doc.Add(layout.NewParagraph("Body text.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// Should have MarkInfo.
	if !strings.Contains(pdf, "/Marked true") {
		t.Error("expected /MarkInfo << /Marked true >>")
	}

	// Should have StructTreeRoot.
	if !strings.Contains(pdf, "/StructTreeRoot") {
		t.Error("expected /StructTreeRoot in catalog")
	}

	// Should have structure elements.
	if !strings.Contains(pdf, "/S /H1") {
		t.Error("expected /S /H1 structure element for heading")
	}
	if !strings.Contains(pdf, "/S /P") {
		t.Error("expected /S /P structure element for paragraph")
	}

	// Should have marked content operators in the stream.
	if !strings.Contains(pdf, "BDC") {
		t.Error("expected BDC marked content operator")
	}
	if !strings.Contains(pdf, "EMC") {
		t.Error("expected EMC marked content operator")
	}

	// Should have MCID in BDC.
	if !strings.Contains(pdf, "/MCID") {
		t.Error("expected /MCID in BDC operator")
	}

	// Should have StructParents on the page.
	if !strings.Contains(pdf, "/StructParents") {
		t.Error("expected /StructParents on page dictionary")
	}
}

func TestTaggedPDFMultipleElements(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetTagged(true)

	doc.Add(layout.NewHeading("Chapter 1", layout.H1))
	doc.Add(layout.NewParagraph("First paragraph.", font.Helvetica, 12))
	doc.Add(layout.NewHeading("Section 1.1", layout.H2))
	doc.Add(layout.NewParagraph("Second paragraph.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/S /H1") {
		t.Error("expected H1")
	}
	if !strings.Contains(pdf, "/S /H2") {
		t.Error("expected H2")
	}
	// Should have multiple P elements.
	if strings.Count(pdf, "/S /P") < 2 {
		t.Error("expected at least 2 paragraph structure elements")
	}
}

func TestTaggedPDFDisabledByDefault(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Add(layout.NewParagraph("Not tagged.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if strings.Contains(pdf, "/StructTreeRoot") {
		t.Error("untagged document should not have StructTreeRoot")
	}
	if strings.Contains(pdf, "BDC") {
		t.Error("untagged document should not have BDC operators")
	}
}

func TestTaggedPDFWithTable(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetTagged(true)

	tbl := layout.NewTable()
	r := tbl.AddRow()
	r.AddCell("A", font.Helvetica, 10)
	r.AddCell("B", font.Helvetica, 10)
	doc.Add(tbl)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/S /TR") {
		t.Error("expected /S /TR structure element for table row")
	}
}

func TestTaggedPDFQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetTagged(true)
	doc.Info.Title = "Tagged PDF Test"

	doc.Add(layout.NewHeading("Tagged Document", layout.H1))
	doc.Add(layout.NewParagraph(
		"This document includes a structure tree for accessibility.",
		font.Helvetica, 12,
	))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	runQpdfCheck(t, buf.Bytes())
}

func TestTaggedPDFParentTree(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetTagged(true)

	doc.Add(layout.NewParagraph("Paragraph one.", font.Helvetica, 12))
	doc.Add(layout.NewParagraph("Paragraph two.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	// ParentTree should be present with /Nums array.
	if !strings.Contains(pdf, "/Nums") {
		t.Error("expected /Nums in ParentTree")
	}
}

func TestTaggedPDFDocumentRoot(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetTagged(true)

	doc.Add(layout.NewParagraph("Content.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	// The StructTreeRoot should have the Document structure element.
	if !strings.Contains(pdf, "/S /Document") {
		t.Error("expected /S /Document as root structure element")
	}
}
