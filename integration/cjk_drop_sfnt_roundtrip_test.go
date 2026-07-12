// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/reader"
)

// TestCJKDropSfntRoundTripsThroughPDFExtraction is the end-to-end
// pin for the user-facing correctness claim of issue #260: dropping
// sfnt from the metric path doesn't silently break the embedded
// font's /ToUnicode CMap. A regression where shaping correctly
// produces glyph IDs (so the rendered PDF looks visually correct)
// but the ToUnicode CMap is built against the wrong source would
// pass every other test in this package — accessibility tools,
// copy-paste, and PDF search would all be silently broken.
//
// The test mirrors the original #227 reporter's scenario: render
// Chinese text through the example flow with a CJK TTC that
// previously failed to load (sfnt's maxCmapSegments=20000 limit
// blocked it), parse the resulting PDF back, extract text via the
// same path Acrobat / pdftotext / screen readers use, and assert
// the original Chinese phrase round-trips byte-perfect.
//
// macOS-only because STHeiti is the universally-available
// recovery-path-triggering CJK font on dev hosts. Linux/Windows
// CI exercises the wiring synthetically via
// font/cmap_test.go::TestParseCmapFormat12LargeGroupCount; this
// test adds the integration-level user-facing claim on the platform
// where the real fixture exists.
func TestCJKDropSfntRoundTripsThroughPDFExtraction(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("STHeiti is the universally-available large-CJK TTC fixture; only on darwin")
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

	result, err := html.ConvertFull(htmlStr, &html.Options{AllowAbsolutePaths: true, StrictAssets: true})
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
	// embedded subset font's /ToUnicode CMap to convert glyph IDs back
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
	// Assert the full input phrase including the trailing 。 (U+3002).
	// Tighter than substring-contains: a CMap regression that drops
	// any trailing character (incl. the period) fails here. The text
	// extractor may surround the paragraph with whitespace; trim
	// before comparing.
	wantFull := want + "。"
	if strings.TrimSpace(got) != wantFull {
		t.Errorf("extracted text does not match input.\n  want: %q\n  got:  %q", wantFull, got)
	}
}

// TestCJKRoundTripWithSyntheticFixture is the platform-agnostic
// counterpart to TestCJKDropSfntRoundTripsThroughPDFExtraction above.
// It uses the synthetic CJK font under font/testdata/synthetic_cjk.ttf
// (issue #281), so the round-trip pin runs on every host — including
// CI runners that lack STHeiti / NotoSansCJK / MingLiU.
//
// The fixture covers exactly the 19 unique codepoints in the test
// phrase, one glyph per codepoint. It does NOT exercise the
// large-cmap recovery path that motivated #260; that branch stays
// covered by the macOS-gated test above and by the synthetic-cmap
// unit test at font/cmap_test.go::TestParseCmapFormat12LargeGroupCount.
// What this test pins is the user-facing claim: HTML containing a
// 19-character CJK phrase, rendered to PDF through @font-face, must
// extract back as the same phrase via the /ToUnicode CMap.
func TestCJKRoundTripWithSyntheticFixture(t *testing.T) {
	fixturePath, err := filepath.Abs("../font/testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}
	if _, err := os.Stat(fixturePath); err != nil {
		t.Fatalf("synthetic fixture missing at %s: %v (run go run ./font/testdata/build_cjk_fixture.go to regenerate)", fixturePath, err)
	}
	const want = "中华人民共和国是一个历史悠久的文明古国"

	htmlStr := fmt.Sprintf(`<!DOCTYPE html>
<html><head><style>
@font-face { font-family: 'CJK'; src: url('%s'); }
body { font-family: 'CJK'; font-size: 14px; }
</style></head><body><p>%s。</p></body></html>`, fixturePath, want)

	result, err := html.ConvertFull(htmlStr, &html.Options{AllowAbsolutePaths: true, StrictAssets: true})
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
	wantFull := want + "。"
	if strings.TrimSpace(got) != wantFull {
		t.Errorf("extracted text does not match input.\n  want: %q\n  got:  %q", wantFull, got)
	}
}
