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

// examplePDFBytes runs the example pipeline against the bundled
// reportHTML constant once per test process and caches the result.
// Each assertion below reads from the same byte slice, so the costly
// HTML → layout → PDF conversion only runs a single time even though
// the file declares several tests.
var examplePDFBytes = sync.OnceValue(func() []byte {
	doc, err := buildDocument(reportHTML)
	if err != nil {
		panic("buildDocument: " + err.Error())
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		panic("WriteTo: " + err.Error())
	}
	return buf.Bytes()
})

// examplePDFReader returns a parsed PdfReader over the cached bytes.
// Tests that need structural access (page count, /Info dict, etc.)
// go through here rather than grepping the serialized text, which
// has a documented false-positive: "Apex Capital Partners" appears
// in both the <meta name="author"> AND the rendered body text, so a
// regression dropping /Info /Author would slip past a substring scan.
func examplePDFReader(t *testing.T) *reader.PdfReader {
	t.Helper()
	r, err := reader.Parse(examplePDFBytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	return r
}

// TestHTMLToPDFExampleProducesValidPDF asserts the example produces
// a well-formed PDF of plausible size. The size floor catches the
// regression where conversion silently bails and writes a near-empty
// file; the header check catches the regression where the output
// isn't even a PDF.
func TestHTMLToPDFExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 5000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >5 KB", len(pdf))
	}
}

// TestHTMLToPDFExampleTwoPages asserts the example renders exactly
// two pages by parsing the cross-reference table. Counts the leaves
// of the page tree, so a regression that adds spurious page nodes or
// drops the CSS page-break would surface here as 1 or 3+.
func TestHTMLToPDFExampleTwoPages(t *testing.T) {
	r := examplePDFReader(t)
	if got := r.PageCount(); got != 2 {
		t.Errorf("PageCount = %d, want 2 (example has one CSS page-break)", got)
	}
}

// TestHTMLToPDFExampleMetadataFromHTML reads the /Info dictionary
// directly and verifies Title and Author come from the <title> and
// <meta name="author"> tags in the source HTML. The earlier substring
// check would have passed even if /Info /Author were dropped entirely,
// because "Apex Capital Partners" also appears in the rendered
// .header-band div. Parsing the Info dict eliminates that ambiguity.
func TestHTMLToPDFExampleMetadataFromHTML(t *testing.T) {
	r := examplePDFReader(t)
	title, author, _, _, _ := r.Info()
	if title != "Q4 2026 Quarterly Report" {
		t.Errorf("/Info Title = %q, want %q (from <title> tag)", title, "Q4 2026 Quarterly Report")
	}
	if author != "Apex Capital Partners" {
		t.Errorf("/Info Author = %q, want %q (from <meta name=\"author\">)", author, "Apex Capital Partners")
	}
}

// TestHTMLToPDFExampleFontDedupAcrossPages pins the document-level
// font-sharing fix landed in #300 (commit 61dc047). The example
// styles body text as Helvetica and headings as Helvetica-Bold across
// two pages; each variant must appear exactly once in the PDF, not
// twice (pre-fix per-page embedding). The trailing space narrows each
// match so /Helvetica does not falsely fire on /Helvetica-Bold, and
// /Helvetica-Bold does not falsely fire on /Helvetica-BoldOblique
// (the margin-box footer's font).
func TestHTMLToPDFExampleFontDedupAcrossPages(t *testing.T) {
	pdf := string(examplePDFBytes())
	for _, marker := range []string{"/BaseFont /Helvetica ", "/BaseFont /Helvetica-Bold "} {
		if got := strings.Count(pdf, marker); got != 1 {
			t.Errorf("count of %q = %d, want 1 (post-#300 single-embed-per-document)", marker, got)
		}
	}
}

// TestHTMLToPDFExampleLinkAnnotationTargetsRepo asserts not only that
// a /Link annotation exists but that its URI action points at the
// expected destination. A regression that emits links but mangles
// their target (e.g. always-empty URI, hardcoded localhost, dropped
// /A action) would pass the presence-only check but break end-user
// behavior — the link no longer goes anywhere useful. Pinning the
// URL specifically is what makes the assertion catch real failures.
func TestHTMLToPDFExampleLinkAnnotationTargetsRepo(t *testing.T) {
	pdf := string(examplePDFBytes())
	if !strings.Contains(pdf, "/Subtype /Link") {
		t.Fatal("no /Subtype /Link annotation; the <a href> in the footer should emit one")
	}
	const wantURL = "https://github.com/carlos7ags/folio"
	if !strings.Contains(pdf, wantURL) {
		t.Errorf("link URI %q not present in PDF; check that the <a href> destination survives layout/serialization", wantURL)
	}
}

// TestHTMLToPDFExampleMarginBoxFontStyleHonored pins issue #378 gap A: the
// bottom-right footer declares font-style: italic; font-weight: bold and
// must draw with Helvetica-BoldOblique, not plain Helvetica.
func TestHTMLToPDFExampleMarginBoxFontStyleHonored(t *testing.T) {
	pdf := string(examplePDFBytes())
	if !strings.Contains(pdf, "/BaseFont /Helvetica-BoldOblique") {
		t.Error("expected a Helvetica-BoldOblique font resource for the italic+bold margin box; font-style/font-weight were not honored")
	}
}
