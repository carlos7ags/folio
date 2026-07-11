// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

// examplePDFBytes runs the example's default (no --tailwind) pipeline
// once per test process and caches the result. Passing false
// explicitly (rather than reading the package-level flag) keeps the
// test off the network regardless of how the test binary is invoked.
var examplePDFBytes = sync.OnceValue(func() []byte {
	doc, err := buildDocument(false)
	if err != nil {
		panic("buildDocument: " + err.Error())
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		panic("WriteTo: " + err.Error())
	}
	return buf.Bytes()
})

func examplePDFReader(t *testing.T) *reader.PdfReader {
	t.Helper()
	r, err := reader.Parse(examplePDFBytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	return r
}

// TestInvoiceExampleProducesValidPDF asserts the default (offline)
// path produces a well-formed PDF of plausible size. The size floor
// catches the regression where the CSS grid/flexbox layout silently
// collapses to near-empty output.
func TestInvoiceExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 3000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >3 KB", len(pdf))
	}
}

// TestInvoiceExampleOnePage asserts the invoice fits on a single
// page, matching its documented one-page layout. A regression that
// broke the flexbox/grid sizing could overflow content onto a
// spurious second page.
func TestInvoiceExampleOnePage(t *testing.T) {
	r := examplePDFReader(t)
	if got := r.PageCount(); got != 1 {
		t.Errorf("PageCount = %d, want 1", got)
	}
}

// TestInvoiceExampleMetadataFromDocument reads /Info directly rather
// than grepping serialized bytes, since "FolioPDF Inc." also appears
// in the rendered "From" card body text — a dropped /Info /Author
// would slip past a substring-only check.
func TestInvoiceExampleMetadataFromDocument(t *testing.T) {
	r := examplePDFReader(t)
	title, author, _, _, _ := r.Info()
	if title != "Invoice INV-2026-0042" {
		t.Errorf("/Info Title = %q, want %q", title, "Invoice INV-2026-0042")
	}
	if author != "FolioPDF Inc." {
		t.Errorf("/Info Author = %q, want %q", author, "FolioPDF Inc.")
	}
}

// TestInvoiceExampleContentPresent pins invoice-specific strings from
// the example's embedded HTML: the invoice number, the line-item
// table content, and the computed total. A regression in the HTML
// converter that dropped table rows or the totals block would surface
// here even though the PDF still "looks" valid structurally.
func TestInvoiceExampleContentPresent(t *testing.T) {
	page, err := examplePDFReader(t).Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	for _, want := range []string{"INV-2026-0042", "Growth Plan", "$413.10"} {
		if !strings.Contains(text, want) {
			t.Errorf("page text does not contain %q; extracted text:\n%s", want, text)
		}
	}
}

// TestInvoiceExampleTailwindFlagDefaultsFalse guards the flag.Bool
// default passed to useTailwind. If it ever flipped to true, a plain
// `go run ./examples/invoice` (and this test's own buildDocument(false)
// calls) would start fetching the Tailwind CDN stylesheet over the
// network on every run.
func TestInvoiceExampleTailwindFlagDefaultsFalse(t *testing.T) {
	if *useTailwind {
		t.Error("useTailwind flag defaults to true; the example's offline path must be the default")
	}
}
