// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html_test

import (
	"bytes"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
)

// TestEndToEndHTMLToPDF verifies the full pipeline:
// HTML string → Convert() → layout elements → Document → valid PDF bytes.
func TestEndToEndHTMLToPDF(t *testing.T) {
	src := `<!DOCTYPE html>
<html>
<head><title>Invoice</title></head>
<body>
  <h1>Invoice #1042</h1>
  <p>Date: 2026-03-14</p>
  <p>Bill to: <strong>Acme Corp</strong></p>

  <table border="1">
    <thead>
      <tr><th>Item</th><th>Qty</th><th>Price</th></tr>
    </thead>
    <tbody>
      <tr><td>Widget A</td><td>10</td><td>$50.00</td></tr>
      <tr><td>Widget B</td><td>5</td><td>$30.00</td></tr>
      <tr><td colspan="2"><strong>Total</strong></td><td>$650.00</td></tr>
    </tbody>
  </table>

  <p>Thank you for your business.</p>
  <p>Visit <a href="https://example.com">example.com</a> for details.</p>

  <h2>Terms</h2>
  <ul>
    <li>Payment due within 30 days</li>
    <li>Late fee: 1.5% per month</li>
  </ul>
</body>
</html>`

	elems, err := html.Convert(src, nil)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(elems) == 0 {
		t.Fatal("Convert returned zero elements")
	}

	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Invoice #1042"
	doc.Info.Author = "Folio HTML Test"

	for _, e := range elems {
		doc.Add(e)
	}

	var buf bytes.Buffer
	n, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if n == 0 {
		t.Fatal("WriteTo produced zero bytes")
	}

	pdf := buf.Bytes()

	// Verify PDF header.
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Error("output does not start with %PDF-")
	}

	// Verify PDF trailer.
	if !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Error("output missing EOF marker")
	}

	t.Logf("Generated PDF: %d bytes, %d layout elements", len(pdf), len(elems))
}

// TestEndToEndStyledHTML tests CSS styling flows through to PDF.
func TestEndToEndStyledHTML(t *testing.T) {
	src := `<div style="padding: 20px; background-color: #f5f5f5">
  <h1 style="color: navy; text-align: center">Styled Report</h1>
  <p style="font-size: 14px; line-height: 1.6">
    This paragraph has custom font size and line height.
    It contains <em>italic</em> and <strong>bold</strong> text.
  </p>
  <ol>
    <li>First item</li>
    <li>Second item</li>
    <li>Third item</li>
  </ol>
</div>`

	elems, err := html.Convert(src, nil)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}

	doc := document.NewDocument(document.PageSizeA4)
	for _, e := range elems {
		doc.Add(e)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	if buf.Len() < 100 {
		t.Errorf("PDF suspiciously small: %d bytes", buf.Len())
	}

	t.Logf("Styled PDF: %d bytes", buf.Len())
}

// TestEndToEndMinimal tests the simplest possible HTML→PDF conversion.
func TestEndToEndMinimal(t *testing.T) {
	elems, err := html.Convert("<p>Hello, World!</p>", nil)
	if err != nil {
		t.Fatal(err)
	}

	doc := document.NewDocument(document.PageSizeLetter)
	for _, e := range elems {
		doc.Add(e)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Error("not a valid PDF")
	}
}
