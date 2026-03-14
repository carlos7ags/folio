// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
)

func makePDF(t *testing.T, title string, pages int) []byte {
	t.Helper()
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = title
	for i := range pages {
		p := doc.AddPage()
		p.AddText(title+" page "+string(rune('1'+i)), font.Helvetica, 12, 72, 700)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestMergeTwoPDFs(t *testing.T) {
	pdf1 := makePDF(t, "Doc One", 2)
	pdf2 := makePDF(t, "Doc Two", 3)

	r1, _ := Parse(pdf1)
	r2, _ := Parse(pdf2)

	m, err := Merge(r1, r2)
	if err != nil {
		t.Fatalf("Merge failed: %v", err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// Read back and verify.
	merged, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("Parse merged failed: %v", err)
	}

	if merged.PageCount() != 5 {
		t.Errorf("merged PageCount = %d, want 5 (2+3)", merged.PageCount())
	}
}

func TestMergePreservesContent(t *testing.T) {
	pdf1 := makePDF(t, "Alpha", 1)
	pdf2 := makePDF(t, "Beta", 1)

	r1, _ := Parse(pdf1)
	r2, _ := Parse(pdf2)

	m, _ := Merge(r1, r2)
	var buf bytes.Buffer
	m.WriteTo(&buf)

	merged, _ := Parse(buf.Bytes())

	// Both pages should have content streams.
	for i := range merged.PageCount() {
		page, _ := merged.Page(i)
		content, err := page.ContentStream()
		if err != nil {
			t.Fatalf("page %d ContentStream: %v", i, err)
		}
		if len(content) == 0 {
			t.Errorf("page %d has empty content stream", i)
		}
	}
}

func TestMergeSinglePDF(t *testing.T) {
	pdf := makePDF(t, "Solo", 3)
	r, _ := Parse(pdf)

	m, _ := Merge(r)
	var buf bytes.Buffer
	m.WriteTo(&buf)

	result, _ := Parse(buf.Bytes())
	if result.PageCount() != 3 {
		t.Errorf("PageCount = %d, want 3", result.PageCount())
	}
}

func TestMergeWithNewPage(t *testing.T) {
	pdf := makePDF(t, "Original", 1)
	r, _ := Parse(pdf)

	m, _ := Merge(r)
	m.AddPageWithText(612, 792, "New page content", font.Helvetica, 12, 72, 700)

	var buf bytes.Buffer
	m.WriteTo(&buf)

	result, _ := Parse(buf.Bytes())
	if result.PageCount() != 2 {
		t.Errorf("PageCount = %d, want 2", result.PageCount())
	}
}

func TestMergeWithBlankPage(t *testing.T) {
	pdf := makePDF(t, "WithBlank", 1)
	r, _ := Parse(pdf)

	m, _ := Merge(r)
	m.AddBlankPage(612, 792)

	var buf bytes.Buffer
	m.WriteTo(&buf)

	result, _ := Parse(buf.Bytes())
	if result.PageCount() != 2 {
		t.Errorf("PageCount = %d, want 2", result.PageCount())
	}
}

func TestMergeSetInfo(t *testing.T) {
	pdf := makePDF(t, "Old Title", 1)
	r, _ := Parse(pdf)

	m, _ := Merge(r)
	m.SetInfo("New Title", "New Author")

	var buf bytes.Buffer
	m.WriteTo(&buf)

	result, _ := Parse(buf.Bytes())
	title, author, _, _, _ := result.Info()
	if title != "New Title" {
		t.Errorf("Title = %q, want %q", title, "New Title")
	}
	if author != "New Author" {
		t.Errorf("Author = %q, want %q", author, "New Author")
	}
}

func TestMergeQpdfCheck(t *testing.T) {
	qpdfPath, err := exec.LookPath("qpdf")
	if err != nil {
		t.Skip("qpdf not installed")
	}

	pdf1 := makePDF(t, "QpdfCheck1", 2)
	pdf2 := makePDF(t, "QpdfCheck2", 1)

	r1, _ := Parse(pdf1)
	r2, _ := Parse(pdf2)

	m, _ := Merge(r1, r2)
	m.SetInfo("Merged PDF", "Folio")

	var buf bytes.Buffer
	m.WriteTo(&buf)

	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/merged.pdf"
	if err := writeFile(tmpFile, buf.Bytes()); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(qpdfPath, "--check", tmpFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("qpdf --check failed: %v\n%s", err, out)
	}
}

func TestMergeThreePDFs(t *testing.T) {
	pdf1 := makePDF(t, "First", 1)
	pdf2 := makePDF(t, "Second", 2)
	pdf3 := makePDF(t, "Third", 3)

	r1, _ := Parse(pdf1)
	r2, _ := Parse(pdf2)
	r3, _ := Parse(pdf3)

	m, _ := Merge(r1, r2, r3)
	var buf bytes.Buffer
	m.WriteTo(&buf)

	result, _ := Parse(buf.Bytes())
	if result.PageCount() != 6 {
		t.Errorf("PageCount = %d, want 6 (1+2+3)", result.PageCount())
	}
}

func TestMergePreservesPageSize(t *testing.T) {
	// Create two PDFs with different page sizes.
	doc1 := document.NewDocument(document.PageSizeLetter)
	doc1.Info.Title = "Letter"
	doc1.AddPage()
	var buf1 bytes.Buffer
	doc1.WriteTo(&buf1)

	doc2 := document.NewDocument(document.PageSizeA4)
	doc2.Info.Title = "A4"
	doc2.AddPage()
	var buf2 bytes.Buffer
	doc2.WriteTo(&buf2)

	r1, _ := Parse(buf1.Bytes())
	r2, _ := Parse(buf2.Bytes())

	m, _ := Merge(r1, r2)
	var out bytes.Buffer
	m.WriteTo(&out)

	result, _ := Parse(out.Bytes())
	p1, _ := result.Page(0)
	p2, _ := result.Page(1)

	if p1.Width != 612 || p1.Height != 792 {
		t.Errorf("page 1: %.0fx%.0f, want 612x792 (Letter)", p1.Width, p1.Height)
	}
	if p2.Width < 595 || p2.Width > 596 {
		t.Errorf("page 2 width: %.2f, want ~595.28 (A4)", p2.Width)
	}
}

func TestMergePageDimensions(t *testing.T) {
	pdf := makePDF(t, "Dimensions", 1)
	r, _ := Parse(pdf)
	m, _ := Merge(r)

	var buf bytes.Buffer
	m.WriteTo(&buf)

	result, _ := Parse(buf.Bytes())
	page, _ := result.Page(0)
	if page.Width != 612 || page.Height != 792 {
		t.Errorf("page size = %.0fx%.0f, want 612x792", page.Width, page.Height)
	}
}

func TestMergeExtractText(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "TextExtract"
	p := doc.AddPage()
	p.AddText("Extractable text", font.Helvetica, 12, 72, 700)
	var buf bytes.Buffer
	doc.WriteTo(&buf)

	r, _ := Parse(buf.Bytes())
	m, _ := Merge(r)
	var out bytes.Buffer
	m.WriteTo(&out)

	result, _ := Parse(out.Bytes())
	page, _ := result.Page(0)
	text, _ := page.ExtractText()
	if !strings.Contains(text, "Extractable text") {
		t.Errorf("text = %q, want to contain 'Extractable text'", text)
	}
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}
