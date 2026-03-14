// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

func TestWriterMinimalPDF(t *testing.T) {
	w := NewWriter("1.7")

	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))

	pages := core.NewPdfDictionary()
	pages.Set("Type", core.NewPdfName("Pages"))
	pages.Set("Kids", core.NewPdfArray())
	pages.Set("Count", core.NewPdfInteger(0))

	catalogRef := w.AddObject(catalog)
	pagesRef := w.AddObject(pages)
	catalog.Set("Pages", pagesRef)
	w.SetRoot(catalogRef)

	var buf bytes.Buffer
	n, err := w.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	if n != int64(buf.Len()) {
		t.Fatalf("WriteTo returned n=%d but buffer has %d bytes", n, buf.Len())
	}

	pdf := buf.String()

	// Verify PDF structure
	if !strings.HasPrefix(pdf, "%PDF-1.7\n") {
		t.Error("missing PDF header")
	}
	if !strings.HasSuffix(pdf, "%%EOF\n") {
		t.Error("missing EOF marker")
	}
	if !strings.Contains(pdf, "xref") {
		t.Error("missing xref table")
	}
	if !strings.Contains(pdf, "trailer") {
		t.Error("missing trailer")
	}
	if !strings.Contains(pdf, "startxref") {
		t.Error("missing startxref")
	}
	if !strings.Contains(pdf, "/Type /Catalog") {
		t.Error("missing catalog")
	}
	if !strings.Contains(pdf, "/Type /Pages") {
		t.Error("missing pages")
	}
}

func TestWriterXrefOffsets(t *testing.T) {
	w := NewWriter("1.7")

	dict := core.NewPdfDictionary()
	dict.Set("Type", core.NewPdfName("Catalog"))
	w.AddObject(dict)
	w.SetRoot(core.NewPdfIndirectReference(1, 0))

	var buf bytes.Buffer
	_, err := w.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// The xref entry for object 1 should point to the byte offset
	// where "1 0 obj" begins. Verify the offset is correct.
	objStart := strings.Index(pdf, "1 0 obj")
	if objStart < 0 {
		t.Fatal("could not find '1 0 obj' in output")
	}

	// Find the xref entry — it's the line after "0000000000 65535 f "
	xrefStart := strings.Index(pdf, "xref\n")
	xrefSection := pdf[xrefStart:]
	lines := strings.Split(xrefSection, "\n")
	// lines[0] = "xref"
	// lines[1] = "0 2"
	// lines[2] = "0000000000 65535 f "  (free entry)
	// lines[3] = offset for object 1
	if len(lines) < 4 {
		t.Fatal("xref section too short")
	}
	entry := lines[3]
	// Parse the 10-digit offset
	if len(entry) < 10 {
		t.Fatalf("xref entry too short: %q", entry)
	}
	offsetStr := entry[:10]
	// Compare with actual position
	expected := strings.Repeat("0", 10-len(strings.TrimLeft(offsetStr, "0"))) +
		strings.TrimLeft(offsetStr, "0")
	_ = expected

	// Simpler check: the offset in xref matches where we find "1 0 obj"
	var parsedOffset int
	for _, c := range offsetStr {
		parsedOffset = parsedOffset*10 + int(c-'0')
	}
	if parsedOffset != objStart {
		t.Errorf("xref offset %d does not match actual object position %d", parsedOffset, objStart)
	}
}

func TestDocumentBlankPage(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()

	// Structural checks
	if !strings.HasPrefix(pdf, "%PDF-1.7\n") {
		t.Error("missing PDF header")
	}
	if !strings.Contains(pdf, "/Type /Catalog") {
		t.Error("missing catalog")
	}
	if !strings.Contains(pdf, "/Type /Pages") {
		t.Error("missing pages dict")
	}
	if !strings.Contains(pdf, "/Type /Page") {
		t.Error("missing page")
	}
	if !strings.Contains(pdf, "/MediaBox [0 0 612.0 792.0]") {
		t.Errorf("missing or wrong MediaBox, pdf:\n%s", pdf)
	}
	if !strings.Contains(pdf, "/Count 1") {
		t.Error("page count should be 1")
	}
}

func TestDocumentMultiplePages(t *testing.T) {
	doc := NewDocument(PageSizeA4)
	doc.AddPage()
	doc.AddPage()
	doc.AddPage()

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Count 3") {
		t.Error("page count should be 3")
	}
	// Should have 3 page objects (obj 3, 4, 5)
	if strings.Count(pdf, "/Type /Page /") < 3 {
		t.Error("expected 3 page objects")
	}
}

func TestDocumentSaveFile(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()

	tmpFile := t.TempDir() + "/blank.pdf"
	err := doc.Save(tmpFile)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Read it back and verify header
	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.HasPrefix(string(data), "%PDF-1.7") {
		t.Error("saved file missing PDF header")
	}
}

// TestDocumentQpdfCheck validates the output with qpdf if available.
// This is the gold standard validation from CLAUDE.md.
func TestDocumentQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	runQpdfCheck(t, buf.Bytes())
}
