// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
)

// writeAndParse serializes d and parses the result back.
func writeAndParse(t *testing.T, d *document.Document) *reader.PdfReader {
	t.Helper()
	var buf bytes.Buffer
	if _, err := d.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	return r
}

// pageText extracts the text of page i of r.
func pageText(t *testing.T, r *reader.PdfReader, i int) string {
	t.Helper()
	p, err := r.Page(i)
	if err != nil {
		t.Fatalf("Page(%d): %v", i, err)
	}
	text, err := p.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText(%d): %v", i, err)
	}
	return text
}

// importablePDF builds a one-page PDF to be stamped onto another document.
func importablePDF(t *testing.T) *reader.PageImport {
	t.Helper()
	src := document.NewDocument(document.PageSizeA4)
	src.AddPage().AddText("Template body", font.Helvetica, 12, 72, 700)
	r := writeAndParse(t, src)
	imp, err := reader.ExtractPageImport(r, 0)
	if err != nil {
		t.Fatalf("ExtractPageImport: %v", err)
	}
	return imp
}

// TestIssue448AbsoluteAfterImportedPage reproduces issue #448: a manual page
// with an imported page stamped on it, followed by AddAbsolute, must draw the
// absolute element on that page instead of appending a new blank page.
func TestIssue448AbsoluteAfterImportedPage(t *testing.T) {
	imp := importablePDF(t)

	d := document.NewDocument(document.PageSizeA4)
	p := d.AddPage()
	p.ImportPage(imp.ContentStream, imp.Resources, imp.Width, imp.Height)
	d.AddAbsolute(layout.NewParagraph("Barcode one", font.HelveticaBold, 10), 400, 100, 150)
	d.AddAbsoluteRight(layout.NewParagraph("Barcode two", font.Courier, 10), 36, 60, 150)

	r := writeAndParse(t, d)
	if got := r.PageCount(); got != 1 {
		t.Fatalf("page count = %d, want 1 (absolutes must land on the existing page)", got)
	}
	text := pageText(t, r, 0)
	for _, want := range []string{"Barcode one", "Barcode two"} {
		if !strings.Contains(text, want) {
			t.Errorf("page 1 text %q missing %q", text, want)
		}
	}

	// The imported template must still be drawn underneath.
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	cs, err := page.ContentStream()
	if err != nil {
		t.Fatalf("ContentStream: %v", err)
	}
	if !bytes.Contains(cs, []byte("/Pg1 Do")) {
		t.Errorf("imported page no longer drawn; content stream:\n%s", cs)
	}
}

// TestIssue448AbsoluteTargetsLastManualPage checks that with several manual
// pages and no flow content, an absolute goes on the last page and a page
// that already uses fonts keeps both its own and the absolute's text.
func TestIssue448AbsoluteTargetsLastManualPage(t *testing.T) {
	d := document.NewDocument(document.PageSizeA4)
	d.AddPage().AddText("First page", font.Helvetica, 12, 72, 700)
	d.AddPage().AddText("Second page", font.Helvetica, 12, 72, 700)
	d.AddAbsolute(layout.NewParagraph("Stamped", font.TimesRoman, 12), 72, 100, 0)

	r := writeAndParse(t, d)
	if got := r.PageCount(); got != 2 {
		t.Fatalf("page count = %d, want 2", got)
	}
	if text := pageText(t, r, 0); strings.Contains(text, "Stamped") {
		t.Errorf("absolute drawn on page 1: %q", text)
	}
	text := pageText(t, r, 1)
	for _, want := range []string{"Second page", "Stamped"} {
		if !strings.Contains(text, want) {
			t.Errorf("page 2 text %q missing %q", text, want)
		}
	}
}

// TestIssue448AbsoluteWithFlowContentUnchanged checks that when flow content
// exists the absolute still lands on the last layout page, which follows the
// manual pages and is therefore the document's last page.
func TestIssue448AbsoluteWithFlowContentUnchanged(t *testing.T) {
	d := document.NewDocument(document.PageSizeA4)
	d.AddPage().AddText("Manual page", font.Helvetica, 12, 72, 700)
	d.Add(layout.NewParagraph("Flow content", font.Helvetica, 12))
	d.AddAbsolute(layout.NewParagraph("Stamped", font.Helvetica, 12), 72, 100, 0)

	r := writeAndParse(t, d)
	if got := r.PageCount(); got != 2 {
		t.Fatalf("page count = %d, want 2", got)
	}
	if text := pageText(t, r, 0); strings.Contains(text, "Stamped") {
		t.Errorf("absolute drawn on the manual page: %q", text)
	}
	text := pageText(t, r, 1)
	for _, want := range []string{"Flow content", "Stamped"} {
		if !strings.Contains(text, want) {
			t.Errorf("page 2 text %q missing %q", text, want)
		}
	}
}

// TestIssue448AbsoluteWithoutPagesCreatesPage checks that a document with only
// an absolute element (no manual pages, no flow) still produces one page.
func TestIssue448AbsoluteWithoutPagesCreatesPage(t *testing.T) {
	d := document.NewDocument(document.PageSizeA4)
	d.AddAbsolute(layout.NewParagraph("Alone", font.Helvetica, 12), 72, 100, 0)

	r := writeAndParse(t, d)
	if got := r.PageCount(); got != 1 {
		t.Fatalf("page count = %d, want 1", got)
	}
	if text := pageText(t, r, 0); !strings.Contains(text, "Alone") {
		t.Errorf("page text %q missing %q", text, "Alone")
	}
}

// TestIssue448AbsoluteOnManualPageTagged checks that a tagged document keeps
// the absolute's structure element when it is drawn on a manual page.
func TestIssue448AbsoluteOnManualPageTagged(t *testing.T) {
	d := document.NewDocument(document.PageSizeA4)
	d.SetTagged(true)
	d.AddPage()
	d.AddAbsolute(layout.NewParagraph("Tagged stamp", font.Helvetica, 12), 72, 100, 0)

	r := writeAndParse(t, d)
	if got := r.PageCount(); got != 1 {
		t.Fatalf("page count = %d, want 1", got)
	}
	if text := pageText(t, r, 0); !strings.Contains(text, "Tagged stamp") {
		t.Errorf("page text %q missing %q", text, "Tagged stamp")
	}
	if r.Catalog().Get("StructTreeRoot") == nil {
		t.Error("tagged document has no StructTreeRoot")
	}
}
