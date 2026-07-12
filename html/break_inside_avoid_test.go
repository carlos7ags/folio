// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// breakInsideMinY returns the lowest y-coordinate (absolute PDF page
// coordinates, origin at bottom-left) touched by any text baseline (Td) or
// vector op (re/m/l) in a page content stream — i.e. how far content reaches
// toward the page bottom. found=false for a blank page.
func breakInsideMinY(stream string) (minY float64, found bool) {
	minY = 1e18
	consider := func(y float64) {
		if y < minY {
			minY = y
		}
		found = true
	}
	for _, line := range strings.Split(stream, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		switch f[len(f)-1] {
		case "Td", "m", "l":
			var y float64
			if _, err := fmt.Sscanf(f[len(f)-2], "%g", &y); err == nil {
				consider(y)
			}
		case "re":
			if len(f) >= 5 {
				var y float64
				fmt.Sscanf(f[len(f)-4], "%g", &y)
				consider(y)
			}
		}
	}
	return
}

// countTextSegments counts text-showing operators (Tj / TJ) across a stream.
// folio emits one per whitespace-separated word, so summing across all pages
// counts the words actually rendered — a content-loss probe. Using single-word
// paragraphs makes this equal the number of paragraphs placed.
func countTextSegments(stream string) int {
	n := 0
	for _, line := range strings.Split(stream, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		switch f[len(f)-1] {
		case "Tj", "TJ":
			n++
		}
	}
	return n
}

// firstKeepTogetherDiv reports whether any element in the (recursively walked)
// tree is a Div with KeepTogether set.
func hasKeepTogetherDiv(elems []layout.Element) bool {
	for _, e := range elems {
		if d, ok := e.(*layout.Div); ok {
			if d.KeepTogether() {
				return true
			}
			if hasKeepTogetherDiv(d.Children()) {
				return true
			}
		}
	}
	return false
}

const (
	biPageW, biPageH = 595.28, 841.89
	biTop, biBottom  = 28.35, 28.35 // 10mm A4 margins
	biTol            = 2.0          // baselines sit a few pt above the glyph bottom
	biUsable         = biPageH - biTop - biBottom
)

func biRender(t *testing.T, htmlStr string) (elems []layout.Element, pages []layout.PageResult) {
	t.Helper()
	var err error
	elems, err = Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := layout.NewRenderer(biPageW, biPageH, layout.Margins{Top: biTop, Right: biBottom, Bottom: biBottom, Left: biBottom})
	for _, e := range elems {
		r.Add(e)
	}
	pages = r.Render()
	return
}

func biAssertNoOverflow(t *testing.T, pages []layout.PageResult) {
	t.Helper()
	for i, pg := range pages {
		minY, ok := breakInsideMinY(string(pg.Stream.Bytes()))
		if !ok {
			continue
		}
		if minY < biBottom-biTol {
			t.Errorf("page %d: content reaches y=%.1f, below the bottom margin (%.1f) — content overran the margin", i, minY, biBottom)
		}
	}
}

// TestBreakInsideAvoidBareBlockMovesWhole is the regression test for the
// reported bug: a bare block (no border/background/padding/size) carrying only
// `break-inside: avoid` was flattened into its children, discarding the
// keep-together grouping. Placed near a page bottom, its leading inline marker
// was stranded in the bottom margin while the following table moved to the next
// page. The block must instead move to the next page as a single unit.
func TestBreakInsideAvoidBareBlockMovesWhole(t *testing.T) {
	// Spacer height is in px; folio scales px->pt (~0.75), so size in pt/0.75.
	// Leave ~40pt at the bottom — smaller than the marker + first table row —
	// so the block cannot fit in the remaining space.
	spacerPx := (biUsable - 40) / 0.75
	htmlStr := fmt.Sprintf(`
<div style="height:%.0fpx">spacer</div>
<div style="position:relative; break-inside:avoid">
  <span style="display:inline-block; width:20px; height:20px; background:#333">4</span>
  <table>
    <tr><td>Row one cell content</td></tr>
    <tr><td>Row two cell content</td></tr>
    <tr><td>Row three cell content</td></tr>
  </table>
</div>`, spacerPx)

	elems, pages := biRender(t, htmlStr)

	if !hasKeepTogetherDiv(elems) {
		t.Fatal("break-inside:avoid block was flattened — no keep-together Div in the tree")
	}
	if len(pages) < 2 {
		t.Fatalf("expected the block to move to a second page, got %d page(s)", len(pages))
	}
	biAssertNoOverflow(t, pages)

	// The whole block (marker + all three rows) must land together on page 2:
	// page 1 shows only the spacer, page 2 carries the rest.
	if n := countTextSegments(string(pages[0].Stream.Bytes())); n != 1 {
		t.Errorf("page 1 should show only the spacer (1 word), got %d", n)
	}
}

// TestBreakInsideAvoidNestedInWrapper is the regression test for the common
// real-world shape: the keep-together block sits inside an outer container (a
// styled page wrapper). The top-level renderer only consults KeepTogether on
// the elements it paginates directly, so without Div.PlanLayout also honoring a
// child's KeepTogether the block was fragmented inside the wrapper — the marker
// stranded on the first page while the table moved to the next. The whole block
// must move together even when nested.
func TestBreakInsideAvoidNestedInWrapper(t *testing.T) {
	spacerPx := (biUsable - 40) / 0.75
	htmlStr := fmt.Sprintf(`
<div style="padding:1px">
  <div style="height:%.0fpx">spacer</div>
  <div style="position:relative; break-inside:avoid">
    <span style="display:inline-block; width:20px; height:20px; background:#333">4</span>
    <table>
      <tr><td>Row one cell content</td></tr>
      <tr><td>Row two cell content</td></tr>
      <tr><td>Row three cell content</td></tr>
    </table>
  </div>
</div>`, spacerPx)

	_, pages := biRender(t, htmlStr)
	if len(pages) < 2 {
		t.Fatalf("expected the nested block to move to a second page, got %d page(s)", len(pages))
	}
	biAssertNoOverflow(t, pages)
	// Page 1 must hold only the spacer word; the block (marker + rows) follows.
	if n := countTextSegments(string(pages[0].Stream.Bytes())); n != 1 {
		t.Errorf("nested block was split: page 1 should show only the spacer (1 word), got %d", n)
	}
}

// TestBreakInsideAvoidNestedOversizedStillFragments guards against a pagination
// loop: a nested keep-together block taller than a page must still fragment (it
// arrives first in the overflow wrapper next page, where the defer guard does
// not fire) rather than deferring forever.
func TestBreakInsideAvoidNestedOversizedStillFragments(t *testing.T) {
	const rows = 80
	var b strings.Builder
	b.WriteString(`<div style="padding:1px"><p>lead</p><div style="break-inside:avoid">`)
	for i := 0; i < rows; i++ {
		fmt.Fprintf(&b, `<p>row%04d</p>`, i)
	}
	b.WriteString(`</div></div>`)

	_, pages := biRender(t, b.String())
	if len(pages) < 2 {
		t.Fatalf("an oversized nested block must fragment, got %d page(s)", len(pages))
	}
	biAssertNoOverflow(t, pages)
	total := 0
	for _, pg := range pages {
		total += countTextSegments(string(pg.Stream.Bytes()))
	}
	if total != rows+1 { // +1 for the "lead" paragraph
		t.Errorf("content loss: expected %d rendered words across pages, got %d", rows+1, total)
	}
}

// TestBreakInsideAvoidLegacySpelling confirms the legacy `page-break-inside`
// spelling behaves identically to the modern `break-inside`.
func TestBreakInsideAvoidLegacySpelling(t *testing.T) {
	spacerPx := (biUsable - 40) / 0.75
	tmpl := `
<div style="height:%.0fpx">spacer</div>
<div style="position:relative; %s:avoid">
  <span style="display:inline-block; width:20px; height:20px; background:#333">4</span>
  <table><tr><td>Row one</td></tr><tr><td>Row two</td></tr><tr><td>Row three</td></tr></table>
</div>`

	modernElems, modernPages := biRender(t, fmt.Sprintf(tmpl, spacerPx, "break-inside"))
	legacyElems, legacyPages := biRender(t, fmt.Sprintf(tmpl, spacerPx, "page-break-inside"))

	if !hasKeepTogetherDiv(legacyElems) {
		t.Fatal("page-break-inside:avoid block was flattened — legacy spelling not honored")
	}
	if !hasKeepTogetherDiv(modernElems) {
		t.Fatal("break-inside:avoid block was flattened")
	}
	if len(modernPages) != len(legacyPages) {
		t.Fatalf("legacy and modern spellings paginate differently: modern=%d legacy=%d pages",
			len(modernPages), len(legacyPages))
	}
	// Per-page content must match exactly, not just the page count.
	for i := range modernPages {
		m := countTextSegments(string(modernPages[i].Stream.Bytes()))
		l := countTextSegments(string(legacyPages[i].Stream.Bytes()))
		if m != l {
			t.Errorf("page %d: legacy spelling placed %d words, modern placed %d", i, l, m)
		}
	}
	biAssertNoOverflow(t, legacyPages)
}

// TestBreakInsideAvoidOversizedStillFragments confirms the best-effort nature of
// the hint: a break-inside:avoid block TALLER than a page must still fragment
// normally — no content dropped and nothing pushed past the bottom margin.
func TestBreakInsideAvoidOversizedStillFragments(t *testing.T) {
	// Single-word paragraphs: each renders as exactly one text segment, so the
	// total segment count across pages must equal the paragraph count.
	const rows = 80
	var b strings.Builder
	b.WriteString(`<div style="break-inside:avoid">`)
	for i := 0; i < rows; i++ {
		fmt.Fprintf(&b, `<p>row%04d</p>`, i)
	}
	b.WriteString(`</div>`)

	elems, pages := biRender(t, b.String())

	if !hasKeepTogetherDiv(elems) {
		t.Fatal("break-inside:avoid block was flattened")
	}
	if len(pages) < 2 {
		t.Fatalf("an oversized block must fragment across pages, got %d page(s)", len(pages))
	}
	biAssertNoOverflow(t, pages)

	// No content loss: every one of the `rows` paragraphs must be rendered.
	total := 0
	for _, pg := range pages {
		total += countTextSegments(string(pg.Stream.Bytes()))
	}
	if total != rows {
		t.Errorf("content loss: expected %d rendered paragraphs across pages, got %d", rows, total)
	}
}

// TestBreakInsideAvoidAbsentUnchanged confirms that without the property a bare
// block is still flattened (no keep-together grouping) — the fix does not change
// pagination for ordinary content.
func TestBreakInsideAvoidAbsentUnchanged(t *testing.T) {
	spacerPx := (biUsable - 40) / 0.75
	htmlStr := fmt.Sprintf(`
<div style="height:%.0fpx">spacer</div>
<div style="position:relative">
  <span style="display:inline-block; width:20px; height:20px; background:#333">4</span>
  <table><tr><td>Row one</td></tr><tr><td>Row two</td></tr><tr><td>Row three</td></tr></table>
</div>`, spacerPx)

	elems, _ := biRender(t, htmlStr)
	if hasKeepTogetherDiv(elems) {
		t.Error("a bare block without break-inside:avoid should not produce a keep-together Div")
	}
}
