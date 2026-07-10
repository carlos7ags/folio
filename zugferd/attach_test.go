// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package zugferd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/reader"
)

// buildAndParse runs Attach on a fresh document, writes it, and
// parses the bytes back with reader.Parse — the write-then-parse round
// trip the package's PDF/A-3 output must survive.
func buildAndParse(t *testing.T, inv *Invoice, p Profile) (*reader.PdfReader, []byte) {
	t.Helper()
	doc := document.NewDocument(document.PageSizeA4)
	doc.Info.Title = "Factur-X Attach Test"
	if err := inv.Attach(doc, p); err != nil {
		t.Fatalf("Attach: %v", err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	return r, buf.Bytes()
}

// findEmbeddedFile walks the catalog's /AF array to the first
// Filespec's embedded /EF /F stream, decoding it the same way
// reader.PageInfo.ContentStream decodes page content streams
// (ResolveObject follows the indirect reference and decompresses).
// Returns the filespec dictionary and the decoded file bytes.
func findEmbeddedFile(t *testing.T, r *reader.PdfReader) (*core.PdfDictionary, []byte) {
	t.Helper()
	catalog := r.Catalog()
	afObj := catalog.Get("AF")
	if afObj == nil {
		t.Fatal("catalog has no /AF entry")
	}
	resolved, err := r.ResolveObject(afObj)
	if err != nil {
		t.Fatalf("resolve /AF: %v", err)
	}
	afArray, ok := resolved.(*core.PdfArray)
	if !ok || afArray.Len() == 0 {
		t.Fatalf("/AF is not a non-empty array: %T", resolved)
	}

	fsObj, err := r.ResolveObject(afArray.At(0))
	if err != nil {
		t.Fatalf("resolve filespec: %v", err)
	}
	fsDict, ok := fsObj.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("filespec is not a dictionary: %T", fsObj)
	}

	efHolderObj, err := r.ResolveObject(fsDict.Get("EF"))
	if err != nil {
		t.Fatalf("resolve /EF: %v", err)
	}
	efHolder, ok := efHolderObj.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("/EF is not a dictionary: %T", efHolderObj)
	}

	streamObj, err := r.ResolveObject(efHolder.Get("F"))
	if err != nil {
		t.Fatalf("resolve embedded file stream: %v", err)
	}
	stream, ok := streamObj.(*core.PdfStream)
	if !ok {
		t.Fatalf("/EF /F is not a stream: %T", streamObj)
	}

	return fsDict, stream.Data
}

// TestAttachEmbedsExactXML is the round-trip acceptance test: it
// writes a document via Attach, re-parses the PDF bytes, extracts and
// decompresses the embedded file stream through the same path
// reader.PageInfo.ContentStream uses for page content, and asserts
// the decoded bytes are byte-identical to what Invoice.XML produces
// independently — proving the attachment carries the exact Factur-X
// XML, not merely a filename that says so.
func TestAttachEmbedsExactXML(t *testing.T) {
	inv := sampleInvoice()
	wantXML, err := inv.XML(ProfileBasic)
	if err != nil {
		t.Fatalf("XML: %v", err)
	}

	r, _ := buildAndParse(t, inv, ProfileBasic)
	fsDict, gotXML := findEmbeddedFile(t, r)

	if !bytes.Equal(gotXML, wantXML) {
		t.Errorf("embedded file bytes differ from Invoice.XML output:\ngot:\n%s\nwant:\n%s", gotXML, wantXML)
	}

	if name, ok := fsDict.Get("F").(*core.PdfString); !ok || name.Text() != attachmentFileName {
		t.Errorf("filespec /F = %v, want %q", fsDict.Get("F"), attachmentFileName)
	}
	if name, ok := fsDict.Get("UF").(*core.PdfString); !ok || name.Text() != attachmentFileName {
		t.Errorf("filespec /UF = %v, want %q", fsDict.Get("UF"), attachmentFileName)
	}
	if rel, ok := fsDict.Get("AFRelationship").(*core.PdfName); !ok || rel.Value != "Alternative" {
		t.Errorf("filespec /AFRelationship = %v, want /Alternative", fsDict.Get("AFRelationship"))
	}
}

// TestAttachSetsPdfA3BConformance asserts Attach configures PDF/A-3B
// (the only PDF/A level permitting file attachments) with the
// Factur-X XMP extension schema and a conformance level matching the
// profile.
func TestAttachSetsPdfA3BConformance(t *testing.T) {
	_, pdfBytes := buildAndParse(t, sampleInvoice(), ProfileBasic)
	pdf := string(pdfBytes)

	if !strings.Contains(pdf, "<pdfaid:part>3</pdfaid:part>") {
		t.Error("expected PDF/A part 3 (PDF/A-3) in XMP")
	}
	if !strings.Contains(pdf, "<pdfaid:conformance>B</pdfaid:conformance>") {
		t.Error("expected PDF/A conformance B in XMP")
	}
	if !strings.Contains(pdf, "Factur-X PDFA Extension Schema") {
		t.Error("expected Factur-X XMP extension schema declaration")
	}
	if !strings.Contains(pdf, "<fx:ConformanceLevel>BASIC</fx:ConformanceLevel>") {
		t.Error("expected fx:ConformanceLevel BASIC")
	}
	if !strings.Contains(pdf, "<fx:DocumentFileName>"+attachmentFileName+"</fx:DocumentFileName>") {
		t.Error("expected fx:DocumentFileName factur-x.xml")
	}
}

// TestAttachMinimumConformanceLevel asserts the XMP fx:ConformanceLevel
// tracks the profile passed to Attach.
func TestAttachMinimumConformanceLevel(t *testing.T) {
	inv := sampleInvoice()
	inv.Lines = nil
	inv.TaxTotals = nil
	_, pdfBytes := buildAndParse(t, inv, ProfileMinimum)
	if !strings.Contains(string(pdfBytes), "<fx:ConformanceLevel>MINIMUM</fx:ConformanceLevel>") {
		t.Error("expected fx:ConformanceLevel MINIMUM")
	}
}

// TestAttachRejectsInvalidInvoice asserts Attach surfaces validation
// errors instead of embedding a broken invoice.
func TestAttachRejectsInvalidInvoice(t *testing.T) {
	inv := sampleInvoice()
	inv.Number = ""
	doc := document.NewDocument(document.PageSizeA4)
	doc.Info.Title = "Invalid Invoice"
	if err := inv.Attach(doc, ProfileBasic); err == nil {
		t.Error("Attach with missing Number = nil error, want error")
	}
}

// TestAttachPageCountAndTitlePreserved asserts Attach only adds PDF/A
// and attachment metadata — it does not touch page content or Info
// beyond what the caller already set.
func TestAttachPageCountAndTitlePreserved(t *testing.T) {
	r, _ := buildAndParse(t, sampleInvoice(), ProfileBasic)
	if got := r.PageCount(); got != 0 {
		t.Errorf("PageCount = %d, want 0 (test builds no page content; Attach must not add any)", got)
	}
	title, _, _, _, _ := r.Info()
	if title != "Factur-X Attach Test" {
		t.Errorf("/Info Title = %q, want %q", title, "Factur-X Attach Test")
	}
}
