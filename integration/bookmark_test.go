// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
)

// renderHTML is a small helper used across the bookmark integration
// tests. It renders the given HTML with auto-bookmarks enabled and
// returns the produced PDF bytes.
func renderHTML(t *testing.T, htmlSrc string) []byte {
	t.Helper()
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetAutoBookmarks(true)
	if err := doc.AddHTML(htmlSrc, nil); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestBookmarkTreeShape verifies that a multi-section document produces
// an /Outlines entry per heading and at the right nesting depth.
func TestBookmarkTreeShape(t *testing.T) {
	htmlSrc := `<html><body>
<h1>Chapter 1</h1><p>Intro.</p>
<h2>Section 1.1</h2><p>Body.</p>
<h2>Section 1.2</h2><p>Body.</p>
<h1>Chapter 2</h1><p>Body.</p>
</body></html>`

	pdf := renderHTML(t, htmlSrc)
	s := string(pdf)
	if !strings.Contains(s, "/Outlines") {
		t.Fatal("expected /Outlines reference in catalog")
	}
	for _, want := range []string{"Chapter 1", "Section 1.1", "Section 1.2", "Chapter 2"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing bookmark title %q", want)
		}
	}
	// Two top-level chapters, each /Count should be 2 (the children).
	// Sample by looking for "/Count 2" — appears at least twice (root has
	// 4 visible items, each chapter has 2 children).
	rootCount := regexp.MustCompile(`/Count 4`)
	if !rootCount.Match(pdf) {
		t.Error("expected root /Count 4 (4 top-level visible items including children)")
	}
}

// TestBookmarkLevelSkip verifies the documented level-skip behavior:
// h1 → h3 with no h2 between nests the h3 under the preceding h1.
func TestBookmarkLevelSkip(t *testing.T) {
	htmlSrc := `<html><body>
<h1>Top</h1>
<h3>Skipped Level</h3>
<h1>Next Top</h1>
</body></html>`

	pdf := renderHTML(t, htmlSrc)
	s := string(pdf)
	for _, want := range []string{"Top", "Skipped Level", "Next Top"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing bookmark title %q", want)
		}
	}
	// "Top" must have one child (the h3). Look for an outline item with
	// Title (Top) and a non-zero /Count entry near it.
	topPattern := regexp.MustCompile(`(?s)\(Top\).*?/Count 1`)
	if !topPattern.Match(pdf) {
		t.Error("expected 'Top' outline to have /Count 1 (one child for the skipped-level h3)")
	}
}

// TestBookmarkLevelNoneSkips verifies that bookmark-level: none removes
// an entry from the outline. The cover-page heading must not appear in
// the bookmark tree.
func TestBookmarkLevelNoneSkips(t *testing.T) {
	htmlSrc := `<html><head><style>
.cover { bookmark-level: none; }
</style></head><body>
<h1 class="cover">Cover Title</h1>
<h1>Chapter 1</h1>
<h1>Chapter 2</h1>
</body></html>`

	pdf := renderHTML(t, htmlSrc)
	s := string(pdf)
	// Cover heading text still appears in body (so substring match alone
	// could be misleading). What we care about is the outline /Title
	// entries — the cover should not be one of them.
	titleRe := regexp.MustCompile(`/Title \(([^)]+)\)`)
	titles := map[string]bool{}
	for _, m := range titleRe.FindAllStringSubmatch(s, -1) {
		titles[m[1]] = true
	}
	if titles["Cover Title"] {
		t.Error("cover heading should be excluded from outline (/Title)")
	}
	if !titles["Chapter 1"] || !titles["Chapter 2"] {
		t.Errorf("expected Chapter 1 and Chapter 2 in outline, got titles: %v", titles)
	}
}

// TestBookmarkStateClosedNegativeCount verifies that bookmark-state:
// closed emits a negative /Count for the outline node, telling viewers
// to render the subtree collapsed (ISO 32000 §12.3.3).
func TestBookmarkStateClosedNegativeCount(t *testing.T) {
	htmlSrc := `<html><head><style>
h1 { bookmark-state: closed; }
</style></head><body>
<h1>Closed Section</h1>
<h2>Child A</h2>
<h2>Child B</h2>
</body></html>`

	pdf := renderHTML(t, htmlSrc)
	// The "Closed Section" item should have a negative /Count.
	closedItem := regexp.MustCompile(`(?s)\(Closed Section\).*?/Count -2`)
	if !closedItem.Match(pdf) {
		t.Errorf("expected 'Closed Section' outline to have negative /Count -2, pdf snippet:\n%s",
			snippetAround(string(pdf), "Closed Section", 200))
	}
}

// TestBookmarkLabelLiteral verifies that a literal-string bookmark-label
// overrides the heading's text content.
func TestBookmarkLabelLiteral(t *testing.T) {
	htmlSrc := `<html><head><style>
h1 { bookmark-label: "Override Title"; }
</style></head><body>
<h1>Original heading text</h1>
</body></html>`

	pdf := renderHTML(t, htmlSrc)
	s := string(pdf)
	if !strings.Contains(s, "(Override Title)") {
		t.Error("expected outline title 'Override Title' in PDF")
	}
}

// TestBookmarkLabelAttr verifies that bookmark-label: attr(NAME) reads
// the named attribute from the element.
func TestBookmarkLabelAttr(t *testing.T) {
	htmlSrc := `<html><head><style>
h1 { bookmark-label: attr(data-bookmark); }
</style></head><body>
<h1 data-bookmark="From Attribute">Visible heading text</h1>
</body></html>`

	pdf := renderHTML(t, htmlSrc)
	s := string(pdf)
	if !strings.Contains(s, "(From Attribute)") {
		t.Error("expected outline title 'From Attribute' (from data-bookmark) in PDF")
	}
}

// TestBookmarkLabelContent verifies that the explicit content() value
// resolves to the heading's text — same as the empty default.
func TestBookmarkLabelContent(t *testing.T) {
	htmlSrc := `<html><head><style>
h1 { bookmark-label: content(); }
</style></head><body>
<h1>The Heading</h1>
</body></html>`

	pdf := renderHTML(t, htmlSrc)
	if !bytes.Contains(pdf, []byte("(The Heading)")) {
		t.Error("expected outline title 'The Heading' (resolved via content()) in PDF")
	}
}

// TestBookmarkNonHeadingTarget verifies that a non-heading element
// (figure) with an explicit bookmark-level participates in the outline.
func TestBookmarkNonHeadingTarget(t *testing.T) {
	htmlSrc := `<html><head><style>
figure { bookmark-level: 2; bookmark-label: attr(data-caption); }
</style></head><body>
<h1>Chapter</h1>
<figure data-caption="Figure 1: Diagram"><p>caption goes here</p></figure>
<h1>Next Chapter</h1>
</body></html>`

	pdf := renderHTML(t, htmlSrc)
	s := string(pdf)
	if !strings.Contains(s, "(Figure 1: Diagram)") {
		t.Errorf("expected non-heading figure to appear in outline as 'Figure 1: Diagram'")
	}
	// The figure (level 2) should nest under its preceding h1.
	chapterChild := regexp.MustCompile(`(?s)\(Chapter\).*?/Count 1`)
	if !chapterChild.Match(pdf) {
		t.Error("expected 'Chapter' to have /Count 1 (one figure child)")
	}
}

// renderHTMLSmallPage renders with a tiny page so multi-line headings
// are forced to split across pages. Used by the continuation regression.
func renderHTMLSmallPage(t *testing.T, htmlSrc string) []byte {
	t.Helper()
	doc := document.NewDocument(document.PageSize{Width: 200, Height: 80})
	doc.SetMargins(layout.Margins{Top: 10, Right: 10, Bottom: 10, Left: 10})
	doc.SetAutoBookmarks(true)
	if err := doc.AddHTML(htmlSrc, nil); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestBookmarkAnchorSplitAcrossPagesEmitsOnce verifies the regression:
// a non-heading bookmark target whose content splits across pages must
// produce exactly one /Title entry. The skip-anchor wrap on Overflow
// inside layout.bookmarkAnchor.PlanLayout enforces this.
func TestBookmarkAnchorSplitAcrossPagesEmitsOnce(t *testing.T) {
	htmlSrc := `<html><head><style>
figure { bookmark-level: 1; bookmark-label: "Wide Figure"; }
</style></head><body>
<figure>
<p>line one with enough words to force the paragraph to wrap onto multiple lines</p>
<p>line two with even more words to push the second half of the figure onto the next page reliably</p>
<p>line three keeps pushing</p>
<p>line four keeps pushing</p>
<p>line five keeps pushing</p>
<p>line six keeps pushing</p>
</figure>
</body></html>`

	pdf := renderHTMLSmallPage(t, htmlSrc)
	count := strings.Count(string(pdf), "(Wide Figure)")
	if count != 1 {
		t.Errorf("expected exactly one '(Wide Figure)' /Title in PDF, got %d — "+
			"a non-heading bookmark target that spans pages must emit one entry", count)
	}
}

// TestBookmarkClosedLeafHasNoNegativeCount documents the spec-driven
// behavior: bookmark-state: closed on a leaf outline node has no PDF
// representation (ISO 32000 §12.3.3 only defines /Count for items with
// descendants). The closed bit is recorded in document.Outline.Closed
// but no negative /Count is written, since there is nothing to collapse.
func TestBookmarkClosedLeafHasNoNegativeCount(t *testing.T) {
	htmlSrc := `<html><head><style>
h1 { bookmark-state: closed; }
</style></head><body>
<h1>Lonely Closed Leaf</h1>
</body></html>`

	pdf := renderHTML(t, htmlSrc)
	s := string(pdf)
	if !strings.Contains(s, "(Lonely Closed Leaf)") {
		t.Fatal("expected leaf bookmark in outline")
	}
	// No negative count near the leaf — leaves emit no /Count at all.
	if regexp.MustCompile(`(?s)\(Lonely Closed Leaf\).*?/Count -`).MatchString(s) {
		t.Error("closed leaf must not emit a negative /Count (spec: /Count omitted for leaves)")
	}
}

// TestBookmarkLevelOutOfRangeIgnored verifies that bookmark-level: 0
// (rejected by the parser) and bookmark-level: 7 (out of H1-H6 range)
// fall back to the natural heading level instead of producing a phantom
// or out-of-range outline entry.
func TestBookmarkLevelOutOfRangeIgnored(t *testing.T) {
	htmlSrc := `<html><head><style>
.zero { bookmark-level: 0; }
.seven { bookmark-level: 7; }
</style></head><body>
<h1 class="zero">Zero Level</h1>
<h2 class="seven">Seven Level</h2>
<h1>Plain</h1>
</body></html>`

	pdf := renderHTML(t, htmlSrc)
	s := string(pdf)
	for _, want := range []string{"Zero Level", "Seven Level", "Plain"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing /Title %q — invalid bookmark-level should fall back to natural heading", want)
		}
	}
	// "Seven Level" is an h2 with rejected override — should nest under
	// "Zero Level" (h1) as a normal h2 child.
	nest := regexp.MustCompile(`(?s)\(Zero Level\).*?/Count 1.*?\(Seven Level\)`)
	if !nest.Match(pdf) {
		t.Error("expected 'Seven Level' to nest under 'Zero Level' (out-of-range override is ignored)")
	}
}

// TestBookmarkAutoDisabledNoOutline verifies the gate: even when
// CSS bookmark-* properties are present, no /Outlines is emitted unless
// SetAutoBookmarks(true) was called.
func TestBookmarkAutoDisabledNoOutline(t *testing.T) {
	htmlSrc := `<html><head><style>
h1 { bookmark-level: 1; bookmark-label: "Should Not Appear"; }
figure { bookmark-level: 2; bookmark-label: "Also Hidden"; }
</style></head><body>
<h1>Heading</h1>
<figure><p>caption</p></figure>
</body></html>`

	doc := document.NewDocument(document.PageSizeLetter)
	// No SetAutoBookmarks call — outline generation must stay off.
	if err := doc.AddHTML(htmlSrc, nil); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	if strings.Contains(s, "/Outlines") {
		t.Error("did not expect /Outlines reference when SetAutoBookmarks is off")
	}
	if strings.Contains(s, "(Should Not Appear)") || strings.Contains(s, "(Also Hidden)") {
		t.Error("auto-bookmarks disabled — no bookmark titles should reach the PDF")
	}
}

// TestBookmarkNestedAnchors verifies that a bookmark target nested
// inside another bookmark target produces both outline entries and the
// inner nests correctly. This exercises BookmarkAnchor wrapping a
// container that itself contains a BookmarkAnchor.
func TestBookmarkNestedAnchors(t *testing.T) {
	htmlSrc := `<html><head><style>
.outer { bookmark-level: 1; bookmark-label: "Outer"; }
.inner { bookmark-level: 2; bookmark-label: "Inner"; }
</style></head><body>
<div class="outer">
  <p>some content</p>
  <figure class="inner"><p>caption</p></figure>
</div>
</body></html>`

	pdf := renderHTML(t, htmlSrc)
	s := string(pdf)
	if !strings.Contains(s, "(Outer)") {
		t.Error("expected outer bookmark in outline")
	}
	if !strings.Contains(s, "(Inner)") {
		t.Error("expected inner bookmark in outline")
	}
	nest := regexp.MustCompile(`(?s)\(Outer\).*?/Count 1.*?\(Inner\)`)
	if !nest.Match(pdf) {
		t.Error("expected 'Inner' to nest under 'Outer' (level 2 child of level 1)")
	}
}

// TestBookmarkTitlesNonASCII verifies that auto-bookmark titles taken
// from accented headings are written as PDF text strings and decode back
// exactly. Before the fix the heading's UTF-8 bytes went straight into a
// literal string and viewers showed "InformaciÃ³n del Perfil".
func TestBookmarkTitlesNonASCII(t *testing.T) {
	want := []string{
		"Reporte de consulta",
		"Información del Perfil",
		"Categorías de información",
		"Pontuação",
		"Résumé — naïve “quotes”",
	}
	htmlSrc := `<html><head><meta charset="utf-8"><style>
h2 { page-break-before: always; }
</style></head><body>
<h1>Reporte de consulta</h1>
<h2>Información del Perfil</h2>
<h2>Categorías de información</h2>
<h2>Pontuação</h2>
<h2>Résumé — naïve “quotes”</h2>
</body></html>`

	pdf := renderHTML(t, htmlSrc)
	r, err := reader.Parse(pdf)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	resolve := func(obj core.PdfObject) *core.PdfDictionary {
		t.Helper()
		res, err := r.ResolveObject(obj)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		d, ok := res.(*core.PdfDictionary)
		if !ok {
			t.Fatalf("expected dictionary, got %T", res)
		}
		return d
	}
	var got []string
	var walk func(first core.PdfObject)
	walk = func(first core.PdfObject) {
		for item := first; item != nil; {
			d := resolve(item)
			if s, ok := d.Get("Title").(*core.PdfString); ok {
				got = append(got, viewerDecode(s))
			}
			if c := d.Get("First"); c != nil {
				walk(c)
			}
			item = d.Get("Next")
		}
	}
	walk(resolve(r.Catalog().Get("Outlines")).Get("First"))
	if !slices.Equal(got, want) {
		t.Errorf("outline titles:\n got %q\nwant %q", got, want)
	}
}

// viewerDecode decodes a PDF text string the way a conforming viewer
// does: a value with a byte-order mark is Unicode, anything else is
// PDFDocEncoding, one byte per character. Latin-1 stands in for
// PDFDocEncoding here; they agree on every byte these tests produce, so
// raw UTF-8 decodes to mojibake ("InformaciÃ³n") exactly as in a viewer.
func viewerDecode(s *core.PdfString) string {
	raw := s.Text()
	if strings.HasPrefix(raw, "\xFE\xFF") || strings.HasPrefix(raw, "\xEF\xBB\xBF") {
		return s.TextString()
	}
	r := make([]rune, len(raw))
	for i := range len(raw) {
		r[i] = rune(raw[i])
	}
	return string(r)
}

// snippetAround returns a window of s around the first occurrence of
// substr, for diagnostic output.
func snippetAround(s, substr string, window int) string {
	i := strings.Index(s, substr)
	if i < 0 {
		return "(substring not found)"
	}
	lo := i - window
	if lo < 0 {
		lo = 0
	}
	hi := i + len(substr) + window
	if hi > len(s) {
		hi = len(s)
	}
	return s[lo:hi]
}
