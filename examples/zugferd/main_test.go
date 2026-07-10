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

// exampleZUGFeRDPDF runs the example's font-discovery, HTML, and
// PDF/A attachment pipeline once per test process and caches the
// result. Returns nil when buildDocument reports no embeddable system
// font (the same condition findSystemFont guards); callers skip in
// that case via examplePDFOrSkip rather than treating it as a
// failure. A genuine build failure with a font present still panics.
var exampleZUGFeRDPDF = sync.OnceValue(func() []byte {
	doc, err := buildDocument()
	if err != nil {
		if findSystemFont() == "" {
			return nil // no system font on this host; caller skips
		}
		panic("buildDocument: " + err.Error())
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		panic("WriteTo: " + err.Error())
	}
	return buf.Bytes()
})

// examplePDFOrSkip returns the cached PDF bytes, skipping the test
// when findSystemFont locates no font to embed (PDF/A requires one).
// This is the same candidate-list check the example itself uses, so
// the test runs wherever the example would succeed — and skips
// cleanly on CI hosts (e.g. Linux) that lack the macOS system-font
// paths, rather than hard-failing.
func examplePDFOrSkip(t *testing.T) []byte {
	t.Helper()
	if findSystemFont() == "" {
		t.Skip("no system font found for PDF/A embedding; skipping")
	}
	pdf := exampleZUGFeRDPDF()
	if pdf == nil {
		t.Fatal("buildDocument failed despite findSystemFont() returning a path")
	}
	return pdf
}

// TestZUGFeRDExampleProducesValidPDF asserts the example produces a
// well-formed PDF of plausible size. The size floor catches the
// regression where the HTML conversion or attachment step silently
// no-ops.
func TestZUGFeRDExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFOrSkip(t)
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 4000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >4 KB", len(pdf))
	}
}

// TestZUGFeRDExampleParsesAsValidPDF verifies the PDF actually parses
// and carries the expected page count and title.
func TestZUGFeRDExampleParsesAsValidPDF(t *testing.T) {
	pdf := examplePDFOrSkip(t)
	r, err := reader.Parse(pdf)
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	if got := r.PageCount(); got != 1 {
		t.Errorf("PageCount = %d, want 1", got)
	}
	title, _, _, _, _ := r.Info()
	if title != "Invoice 2024-001" {
		t.Errorf("/Info Title = %q, want %q", title, "Invoice 2024-001")
	}
}

// TestZUGFeRDExampleHasEmbeddedFileAttachment pins the two structural
// markers that make this a hybrid Factur-X document: the
// /EmbeddedFiles name tree (or /AF associated-files array) and the
// literal attachment filename the example sets via FileAttachment.
// Losing either would silently degrade the PDF back into a plain
// invoice with no machine-readable data.
func TestZUGFeRDExampleHasEmbeddedFileAttachment(t *testing.T) {
	pdf := string(examplePDFOrSkip(t))
	if !strings.Contains(pdf, "/EmbeddedFiles") && !strings.Contains(pdf, "/AF") {
		t.Error("no /EmbeddedFiles or /AF entry; doc.AttachFile should register the Factur-X XML in the file-attachment tree")
	}
	if !strings.Contains(pdf, "factur-x.xml") {
		t.Error(`attachment filename "factur-x.xml" not found in PDF`)
	}
}

// TestZUGFeRDExamplePdfAMarkerPresent asserts the PDF/A conformance
// marker (XMP pdfaid schema, set via doc.SetPdfA) is present. PDF/A-3B
// is the only PDF/A level permitting file attachments (ISO 19005-3
// §6.4); losing this marker would mean the output no longer declares
// PDF/A conformance at all, even though it still carries an
// attachment.
func TestZUGFeRDExamplePdfAMarkerPresent(t *testing.T) {
	pdf := string(examplePDFOrSkip(t))
	if !strings.Contains(pdf, "pdfaid") {
		t.Error("no pdfaid XMP schema found; doc.SetPdfA(PdfA3B) should emit a PDF/A conformance marker")
	}
	if !strings.Contains(pdf, "Factur-X PDFA Extension Schema") {
		t.Error("Factur-X XMP extension schema not found; doc.SetPdfA's XMPSchemas should be embedded")
	}
}
