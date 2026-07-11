// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	foliohtml "github.com/carlos7ags/folio/html"
)

// TestIssue328MarginBoxPdfAUsesEmbeddedFont reproduces issue #328: a document
// with an @font-face body font and an @page margin box (running footer with
// counter(page) / counter(pages)) rendered as PDF/A. Before the fix the
// margin box was drawn with the non-embedded standard Helvetica, so PDF/A
// validation failed with "non-embedded standard font Helvetica". After the
// fix the margin box is drawn with the document's embedded body font, so
// WriteTo (which validates PDF/A inline) succeeds.
func TestIssue328MarginBoxPdfAUsesEmbeddedFont(t *testing.T) {
	fontPath, err := filepath.Abs("../font/testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}

	// Body text uses codepoints the synthetic fixture covers; the margin-box
	// content ("Page N of M") is ASCII and falls back to .notdef in this
	// font, which is still embedded and therefore PDF/A-valid.
	htmlStr := fmt.Sprintf(`<!DOCTYPE html>
<html><head><style>
@font-face { font-family: 'CJK'; src: url('%s'); }
body { font-family: 'CJK'; font-size: 14px; }
@page { @bottom-center { content: "Page " counter(page) " of " counter(pages); } }
</style></head><body><p>中华人民共和国是一个历史悠久的文明古国。</p></body></html>`, fontPath)

	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Issue 328"
	doc.SetPdfA(PdfAConfig{Level: PdfA3B})
	if err := doc.AddHTML(htmlStr, &foliohtml.Options{AllowAbsolutePaths: true, StrictAssets: true}); err != nil {
		t.Fatalf("AddHTML: %v", err)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo (PDF/A validation) failed; margin box must use the "+
			"embedded body font, not Helvetica: %v", err)
	}
	if err := doc.ValidatePdfA(); err != nil {
		t.Fatalf("ValidatePdfA returned error: %v", err)
	}

	// Sanity: the standard Helvetica name must not appear as a font resource
	// (it would if the margin box still used the standard-font path).
	if strings.Contains(buf.String(), "/BaseFont /Helvetica") {
		t.Error("output references non-embedded /BaseFont /Helvetica; margin box font was not embedded")
	}
}

// TestIssue328MarginBoxFallsBackToHelveticaWithoutEmbeddedFont confirms a
// document with NO embedded fonts keeps the previous Helvetica behaviour for
// margin boxes (such a document is not PDF/A, so non-embedded is acceptable).
func TestIssue328MarginBoxFallsBackToHelveticaWithoutEmbeddedFont(t *testing.T) {
	htmlStr := `<!DOCTYPE html>
<html><head><style>
@page { @bottom-center { content: "Page " counter(page); } }
</style></head><body><p>Hello world.</p></body></html>`

	doc := NewDocument(PageSizeLetter)
	if err := doc.AddHTML(htmlStr, nil); err != nil {
		t.Fatalf("AddHTML: %v", err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	// Margin box should still render via the standard Helvetica path.
	if !strings.Contains(buf.String(), "Helvetica") {
		t.Error("expected Helvetica fallback for margin box in non-embedded document")
	}
}
