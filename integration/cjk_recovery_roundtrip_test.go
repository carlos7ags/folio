// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"fmt"
	"os"
	"runtime"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/reader"
)

// TestCJKRecoveryRoundTripsThroughPDFExtraction is the end-to-end pin
// for the cmap recovery path's user-facing correctness claim:
// rendering Chinese text with a font that requires the recovery path
// produces a PDF whose embedded subset font and /ToUnicode CMap
// correctly round-trip the original codepoints back through Folio's
// own text extractor.
//
// The risk this test guards against is subtle: the recovery path
// installs a 22-byte stub cmap into the sfnt-parsed view of the font
// while keeping the original bytes on the Face for subsetting. If the
// downstream subset / ToUnicode generation read from the wrong source
// — sfnt's empty stub mapping rather than Folio's [cmapTable] — the
// PDF would render visually correct but text extraction would return
// garbage. Accessibility tools, copy-paste, and search would all be
// broken silently.
//
// This test mirrors the original #227 reporter's scenario as closely
// as possible: @font-face url() pointing at a system CJK TTC, a body
// containing the exact phrase from the bug report, BaseFS unset.
// macOS-only because STHeiti is the available proxy for the user's
// actually-reported msyh.ttc / NotoSansCJK-Regular.ttc fonts.
func TestCJKRecoveryRoundTripsThroughPDFExtraction(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("recovery-path round-trip pin requires STHeiti TTC (macOS-only proxy)")
	}
	const stHeiti = "/System/Library/Fonts/STHeiti Light.ttc"
	if _, err := os.Stat(stHeiti); err != nil {
		t.Skipf("STHeiti not present: %v", err)
	}
	const want = "中华人民共和国是一个历史悠久的文明古国"

	htmlStr := fmt.Sprintf(`<!DOCTYPE html>
<html><head><style>
@font-face { font-family: 'CJK'; src: url('%s'); }
body { font-family: 'CJK'; font-size: 14px; }
</style></head><body><p>%s。</p></body></html>`, stHeiti, want)

	result, err := html.ConvertFull(htmlStr, &html.Options{StrictAssets: true})
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	doc := document.NewDocument(document.PageSizeLetter)
	for _, e := range result.Elements {
		doc.Add(e)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	// Re-parse the PDF and extract text from page 0. This walks the
	// embedded subset font's ToUnicode CMap to convert glyph IDs back
	// to codepoints — the same path Acrobat / pdftotext / screen
	// readers take.
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	if r.PageCount() == 0 {
		t.Fatal("PDF has no pages")
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	got, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !bytes.Contains([]byte(got), []byte(want)) {
		t.Errorf("extracted text does not contain the original Chinese; ToUnicode CMap is broken.\n  want substring: %q\n  got: %q", want, got)
	}
}
