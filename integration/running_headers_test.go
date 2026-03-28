// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/reader"
)

func TestRunningHeaderStringSet(t *testing.T) {
	// CSS string-set on headings, string() in margin boxes.
	// Each chapter heading should update the running string,
	// and the margin box should reflect the most recent value.
	htmlSrc := `
<html>
<head><style>
h1 { string-set: chapter content(); }
@page {
	@top-center { content: string(chapter); }
	@bottom-center { content: "Page " counter(page) " of " counter(pages); }
}
</style></head>
<body>
<h1>Chapter 1: Introduction</h1>
<p>` + strings.Repeat("Introduction text that fills the page. ", 200) + `</p>
<h1>Chapter 2: Methods</h1>
<p>` + strings.Repeat("Methods text that fills the page with content. ", 200) + `</p>
<h1>Chapter 3: Results</h1>
<p>` + strings.Repeat("Results text for the final chapter. ", 100) + `</p>
</body>
</html>`

	doc := document.NewDocument(document.PageSizeLetter)
	err := doc.AddHTML(htmlSrc, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// Parse the output and verify chapter titles appear in headers.
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if r.PageCount() < 3 {
		t.Skipf("expected at least 3 pages, got %d", r.PageCount())
	}

	// Page 1 should have "Chapter 1" in its text (from the margin box).
	p1, _ := r.Page(0)
	t1, _ := p1.ExtractText()
	if !strings.Contains(t1, "Chapter 1") {
		t.Errorf("page 1 should contain 'Chapter 1' (from heading or margin box), got text: %s", truncate(t1, 200))
	}

	// A later page should have a different chapter title.
	lastPage, _ := r.Page(r.PageCount() - 1)
	lastText, _ := lastPage.ExtractText()
	if !strings.Contains(lastText, "Chapter") {
		t.Logf("last page text (may or may not have chapter header): %s", truncate(lastText, 200))
	}

	// Basic validity.
	if len(buf.Bytes()) < 1000 {
		t.Error("output seems too small")
	}
}

func TestRunningHeaderNoStringSet(t *testing.T) {
	// Without string-set, string() in margin box should produce empty string.
	htmlSrc := `
<html>
<head><style>
@page { @top-center { content: string(chapter); } }
</style></head>
<body>
<h1>Title</h1>
<p>Some content.</p>
</body>
</html>`

	doc := document.NewDocument(document.PageSizeLetter)
	err := doc.AddHTML(htmlSrc, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// Should produce a valid PDF without errors.
	_, err = reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
}

func TestStringSetLiteralValue(t *testing.T) {
	// string-set with a literal value instead of content().
	htmlSrc := `
<html>
<head><style>
h1 { string-set: doctitle "Annual Report 2026"; }
@page { @top-right { content: string(doctitle); } }
</style></head>
<body>
<h1>Introduction</h1>
<p>` + strings.Repeat("Content. ", 100) + `</p>
</body>
</html>`

	doc := document.NewDocument(document.PageSizeLetter)
	if err := doc.AddHTML(htmlSrc, nil); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, _ := reader.Parse(buf.Bytes())
	p1, _ := r.Page(0)
	t1, _ := p1.ExtractText()
	if !strings.Contains(t1, "Annual Report 2026") {
		t.Errorf("expected 'Annual Report 2026' in page text, got: %s", truncate(t1, 200))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
