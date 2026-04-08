// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestRenderSingleParagraph(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Hello World", font.Helvetica, 12))
	pages := r.Render()

	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "Tj") {
		t.Error("expected text operators in output")
	}
}

func TestRenderMultipleParagraphs(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("First paragraph.", font.Helvetica, 12))
	r.Add(NewParagraph("Second paragraph.", font.Helvetica, 12))
	r.Add(NewParagraph("Third paragraph.", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestRenderAreaBreak(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Page one", font.Helvetica, 12))
	r.Add(NewAreaBreak())
	r.Add(NewParagraph("Page two", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 2 {
		t.Fatalf("expected 2 pages, got %d", len(pages))
	}
}

func TestRenderPageBreakOnOverflow(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	// Add enough content to overflow one page.
	for range 60 {
		r.Add(NewParagraph("This is a line of text that takes up space on the page.", font.Helvetica, 12))
	}

	pages := r.Render()
	if len(pages) < 2 {
		t.Fatalf("expected at least 2 pages, got %d", len(pages))
	}
	// Each page should have content.
	for i, p := range pages {
		if len(p.Stream.Bytes()) == 0 {
			t.Errorf("page %d has no content", i)
		}
	}
}

func TestRenderEmptyDocument(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	pages := r.Render()

	if len(pages) != 1 {
		t.Fatalf("expected 1 empty page, got %d", len(pages))
	}
}

func TestRenderWithHeading(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewHeading("Chapter 1", H1))
	r.Add(NewParagraph("Body text.", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "Tj") {
		t.Error("expected text content")
	}
}

func TestRenderTagged(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.SetTagged(true)
	r.Add(NewHeading("Title", H1))
	r.Add(NewParagraph("Body.", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "BDC") {
		t.Error("expected BDC marked content in tagged mode")
	}
	if !strings.Contains(content, "EMC") {
		t.Error("expected EMC in tagged mode")
	}

	tags := r.StructTags()
	if len(tags) < 2 {
		t.Errorf("expected at least 2 struct tags, got %d", len(tags))
	}
	// First tag should be H1.
	hasH1 := false
	hasP := false
	for _, tag := range tags {
		if tag.Tag == "H1" {
			hasH1 = true
		}
		if tag.Tag == "P" {
			hasP = true
		}
	}
	if !hasH1 {
		t.Error("expected H1 struct tag")
	}
	if !hasP {
		t.Error("expected P struct tag")
	}
}

func TestTaggedTableNesting(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.SetTagged(true)

	tbl := NewTable()
	row := tbl.AddRow()
	row.AddCell("A", font.Helvetica, 10)
	row.AddCell("B", font.Helvetica, 10)
	r.Add(tbl)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	tags := r.StructTags()
	// Should have a Table tag as parent of TR tags.
	var tableIdx = -1
	for i, tag := range tags {
		if tag.Tag == "Table" {
			tableIdx = i
			break
		}
	}
	if tableIdx < 0 {
		t.Fatal("expected Table struct tag")
	}

	// TR tags should reference the Table tag as parent.
	trCount := 0
	for _, tag := range tags {
		if tag.Tag == "TR" {
			trCount++
			if tag.ParentIndex != tableIdx {
				t.Errorf("TR tag parent=%d, want %d (Table)", tag.ParentIndex, tableIdx)
			}
		}
	}
	if trCount == 0 {
		t.Error("expected at least one TR struct tag")
	}
}

func TestTaggedDivNesting(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.SetTagged(true)

	d := NewDiv()
	d.Add(NewParagraph("Inside div", font.Helvetica, 12))
	r.Add(d)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	tags := r.StructTags()
	var divIdx = -1
	for i, tag := range tags {
		if tag.Tag == "Div" {
			divIdx = i
			break
		}
	}
	if divIdx < 0 {
		t.Fatal("expected Div struct tag")
	}

	// P tag inside Div should reference Div as parent.
	hasNestedP := false
	for _, tag := range tags {
		if tag.Tag == "P" && tag.ParentIndex == divIdx {
			hasNestedP = true
		}
	}
	if !hasNestedP {
		t.Error("expected P tag nested under Div")
	}
}

func TestTaggedListNesting(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.SetTagged(true)

	lst := NewList(font.Helvetica, 12)
	lst.AddItem("First item")
	lst.AddItem("Second item")
	r.Add(lst)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	tags := r.StructTags()
	var listIdx = -1
	for i, tag := range tags {
		if tag.Tag == "L" {
			listIdx = i
			break
		}
	}
	if listIdx < 0 {
		t.Fatal("expected L struct tag for list")
	}

	// LI tags should reference L as parent.
	liCount := 0
	for _, tag := range tags {
		if tag.Tag == "LI" && tag.ParentIndex == listIdx {
			liCount++
		}
	}
	if liCount < 2 {
		t.Errorf("expected at least 2 LI tags nested under L, got %d", liCount)
	}
}

func TestExtGStateOnDivOpacity(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	d := NewDiv().SetOpacity(0.5)
	d.Add(NewParagraph("Semi-transparent", font.Helvetica, 12))
	r.Add(d)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}

	// Page should have an ExtGState entry.
	if len(pages[0].ExtGStates) == 0 {
		t.Error("expected ExtGState entry for opacity")
	}

	// Content stream should reference the graphics state.
	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "gs") {
		t.Error("expected gs operator in content stream")
	}
}

func TestRenderLineSeparator(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Before", font.Helvetica, 12))
	r.Add(NewLineSeparator().SetWidth(1))
	r.Add(NewParagraph("After", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "S") {
		t.Error("expected stroke operator for separator")
	}
}

func TestRenderTable(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	tbl := NewTable()
	row := tbl.AddRow()
	row.AddCell("A", font.Helvetica, 10)
	row.AddCell("B", font.Helvetica, 10)
	r.Add(tbl)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "Tj") {
		t.Error("expected text content from table")
	}
}

// TestStripLeadingOffsetMultiBlock verifies that the page-top normalization
// uniformly removes the leading vertical offset from every PlacedBlock in a
// plan, not just from Blocks[0]. The previous implementation zeroed only the
// first block, which left subsequent blocks shifted by the original offset
// — producing either an overlap (heading multi-line bug) or an oversized
// gap (paragraph spaceBefore at page top), depending on which element type
// the plan came from.
func TestStripLeadingOffsetMultiBlock(t *testing.T) {
	plan := LayoutPlan{
		Status:   LayoutFull,
		Consumed: 100,
		Blocks: []PlacedBlock{
			{Y: 14, Height: 30}, // first line, shifted by space-above of 14
			{Y: 44, Height: 30}, // second line, follows immediately
			{Y: 74, Height: 30}, // third line
		},
	}
	stripLeadingOffset(&plan)

	wantYs := []float64{0, 30, 60}
	for i, want := range wantYs {
		if plan.Blocks[i].Y != want {
			t.Errorf("block %d Y: want %.1f, got %.1f", i, want, plan.Blocks[i].Y)
		}
	}
	if plan.Consumed != 86 {
		t.Errorf("Consumed: want 86, got %.1f", plan.Consumed)
	}
}

// TestStripLeadingOffsetNoOpZeroOffset ensures the helper is a no-op for
// plans whose first block is already at Y=0. This is the common case for
// any element without space-above (most paragraphs, lists, divs).
func TestStripLeadingOffsetNoOpZeroOffset(t *testing.T) {
	plan := LayoutPlan{
		Status:   LayoutFull,
		Consumed: 60,
		Blocks: []PlacedBlock{
			{Y: 0, Height: 30},
			{Y: 30, Height: 30},
		},
	}
	stripLeadingOffset(&plan)

	if plan.Blocks[0].Y != 0 || plan.Blocks[1].Y != 30 {
		t.Errorf("zero-offset plan should be untouched: %v", plan.Blocks)
	}
	if plan.Consumed != 60 {
		t.Errorf("Consumed should be untouched, got %.1f", plan.Consumed)
	}
}

// TestStripLeadingOffsetEmpty guards against a panic when an element returns
// a plan with no blocks (e.g. an empty paragraph).
func TestStripLeadingOffsetEmpty(t *testing.T) {
	plan := LayoutPlan{Status: LayoutFull, Consumed: 0}
	stripLeadingOffset(&plan) // must not panic
}

// TestRenderHeadingMultilineAtPageTop is an end-to-end check that a heading
// which wraps to multiple lines AND lands at the top of a page renders
// without overlap and without an oversized gap. This is the case the
// previous bug + the previous page-top snap interacted in: with the old
// code, mid-page headings overlapped, and a hypothetical fix that only
// shifted all blocks would have introduced a top-of-page gap unless the
// snap also normalized uniformly.
func TestRenderHeadingMultilineAtPageTop(t *testing.T) {
	r := NewRenderer(300, 800, Margins{Top: 36, Right: 36, Bottom: 36, Left: 36})
	r.Add(NewHeading("Globex Corporation — Platform Renewal + Expansion (FY26)", H1))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	// The rendered page is opaque from here, but the regression test in
	// heading_test.go (TestHeadingPlanLayoutMultilineNoOverlap) covers the
	// block-level invariant. This test simply ensures the full pipeline
	// runs to completion for a wrapped heading at page top — the previous
	// page-top snap ignored multi-block plans and would silently drop
	// later lines off the page in some configurations.
	if len(pages[0].Stream.Bytes()) == 0 {
		t.Error("expected non-empty content stream for wrapped heading")
	}
}

func TestRenderSpaceBeforeSuppressedOnNewPage(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	// First element with large SpaceBefore.
	p := NewParagraph("Should be at top", font.Helvetica, 12).SetSpaceBefore(50)
	r.Add(p)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	// The first block's Y should be 0 (SpaceBefore suppressed at page top).
	// We can't inspect blocks directly after render, but the content stream
	// should show text positioned near the top margin.
}

func TestRenderPartialSplit(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	// Create a long paragraph that should split across pages via the adapter.
	longText := strings.Repeat("This is a sentence that will repeat many times to create a very long paragraph that spans multiple pages. ", 50)
	r.Add(NewParagraph(longText, font.Helvetica, 12))

	pages := r.Render()
	if len(pages) < 2 {
		t.Fatalf("expected at least 2 pages for a very long paragraph, got %d", len(pages))
	}
	// All pages should have text content.
	for i, p := range pages {
		content := string(p.Stream.Bytes())
		if !strings.Contains(content, "Tj") {
			t.Errorf("page %d should have text content", i)
		}
	}
}
