// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"strings"
	"testing"
)

func TestAddHTMLMetadata(t *testing.T) {
	doc := NewDocument(PageSizeLetter)

	err := doc.AddHTML(`<!DOCTYPE html>
<html>
<head>
  <title>Quarterly Report</title>
  <meta name="author" content="Jane Doe">
  <meta name="subject" content="Finance">
  <meta name="keywords" content="quarterly, revenue, 2026">
  <meta name="generator" content="ReportBuilder v2">
</head>
<body>
  <h1>Q1 2026 Revenue</h1>
  <p>Revenue grew 23% year over year.</p>
</body>
</html>`, nil)
	if err != nil {
		t.Fatalf("AddHTML: %v", err)
	}

	if doc.Info.Title != "Quarterly Report" {
		t.Errorf("Title = %q, want %q", doc.Info.Title, "Quarterly Report")
	}
	if doc.Info.Author != "Jane Doe" {
		t.Errorf("Author = %q, want %q", doc.Info.Author, "Jane Doe")
	}
	if doc.Info.Subject != "Finance" {
		t.Errorf("Subject = %q, want %q", doc.Info.Subject, "Finance")
	}
	if doc.Info.Keywords != "quarterly, revenue, 2026" {
		t.Errorf("Keywords = %q, want %q", doc.Info.Keywords, "quarterly, revenue, 2026")
	}
	if doc.Info.Creator != "ReportBuilder v2" {
		t.Errorf("Creator = %q, want %q", doc.Info.Creator, "ReportBuilder v2")
	}

	if len(doc.elements) == 0 {
		t.Error("expected layout elements from HTML body")
	}
}

func TestAddHTMLPreservesExistingInfo(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "My Custom Title"

	err := doc.AddHTML(`<html><head><title>HTML Title</title></head><body><p>Hi</p></body></html>`, nil)
	if err != nil {
		t.Fatalf("AddHTML: %v", err)
	}

	if doc.Info.Title != "My Custom Title" {
		t.Errorf("Title was overwritten: got %q, want %q", doc.Info.Title, "My Custom Title")
	}
}

func TestAddHTMLProducesPDF(t *testing.T) {
	doc := NewDocument(PageSizeLetter)

	err := doc.AddHTML(`<html>
<head><title>Test PDF</title><meta name="author" content="Folio"></head>
<body>
  <h1>Hello World</h1>
  <p>This PDF was generated from HTML with automatic metadata.</p>
</body>
</html>`, nil)
	if err != nil {
		t.Fatalf("AddHTML: %v", err)
	}

	var buf bytes.Buffer
	n, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if n == 0 {
		t.Fatal("WriteTo produced zero bytes")
	}

	pdf := buf.String()
	if !strings.HasPrefix(pdf, "%PDF-") {
		t.Error("output does not start with %PDF-")
	}
	if !strings.Contains(pdf, "Test PDF") {
		t.Error("PDF does not contain title metadata")
	}
}
