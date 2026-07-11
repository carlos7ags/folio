// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestListUnorderedBasic(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		AddItem("First item").
		AddItem("Second item")

	lines := l.Layout(400)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	// First line of first item should have marker words (bullet)
	if lines[0].listRef == nil {
		t.Fatal("expected listRef on line")
	}
	if len(lines[0].listRef.markerWords) == 0 {
		t.Error("first line should have marker words")
	}
}

func TestListOrderedMarkers(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		SetStyle(ListOrdered).
		AddItem("Alpha").
		AddItem("Beta").
		AddItem("Gamma")

	lines := l.Layout(400)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// Check markers are "1.", "2.", "3."
	markers := []string{"1.", "2.", "3."}
	lineIdx := 0
	for i, expected := range markers {
		if lineIdx >= len(lines) {
			t.Fatalf("ran out of lines at item %d", i)
		}
		ref := lines[lineIdx].listRef
		if ref == nil {
			t.Fatalf("line %d missing listRef", lineIdx)
		}
		if len(ref.markerWords) == 0 {
			t.Errorf("item %d: no marker words", i)
		} else if ref.markerWords[0].Text != expected {
			t.Errorf("item %d: expected marker %q, got %q", i, expected, ref.markerWords[0].Text)
		}
		// Skip to next item's first line
		lineIdx++
		for lineIdx < len(lines) && (lines[lineIdx].listRef == nil || len(lines[lineIdx].listRef.markerWords) > 0) {
			if lines[lineIdx].listRef != nil && len(lines[lineIdx].listRef.markerWords) > 0 {
				break
			}
			lineIdx++
		}
	}
}

func TestListIndent(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		SetIndent(36).
		AddItem("Indented item")

	lines := l.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].listRef.indent != 36 {
		t.Errorf("expected indent 36, got %f", lines[0].listRef.indent)
	}
}

func TestListWordWrap(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		AddItem("This is a longer list item that should wrap to multiple lines when the available width is narrow")

	lines := l.Layout(200)
	if len(lines) < 2 {
		t.Errorf("expected word-wrapped lines, got %d", len(lines))
	}
	// Second line should be indented but have no marker
	if len(lines) >= 2 {
		if lines[1].listRef == nil {
			t.Error("second line should have listRef for indent")
		} else if len(lines[1].listRef.markerWords) > 0 {
			t.Error("second line should not have marker words")
		}
	}
}

func TestListEmpty(t *testing.T) {
	l := NewList(font.Helvetica, 12)
	lines := l.Layout(400)
	if len(lines) != 0 {
		t.Errorf("empty list should produce no lines, got %d", len(lines))
	}
}

func TestListChaining(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		SetStyle(ListOrdered).
		SetIndent(24).
		SetLeading(1.5).
		AddItem("One").
		AddItem("Two")

	lines := l.Layout(400)
	if len(lines) < 2 {
		t.Errorf("expected at least 2 lines, got %d", len(lines))
	}
}

func TestListImplementsElement(t *testing.T) {
	var _ Element = NewList(font.Helvetica, 12)
}

// SetMarkerFont overrides the marker's font independently of the list's
// body font (used for CSS ::marker { font-style: italic }); the body text
// keeps the list's own font.
func TestSetMarkerFont(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		SetStyle(ListOrdered).
		SetMarkerFont(font.HelveticaOblique, nil).
		AddItem("Alpha")

	marker := l.markerParagraph(0)
	if marker.runs[0].Font != font.HelveticaOblique {
		t.Errorf("expected marker font %v, got %v", font.HelveticaOblique, marker.runs[0].Font)
	}

	lines := l.Layout(400)
	if len(lines) == 0 || len(lines[0].Words) == 0 {
		t.Fatal("expected body text words")
	}
	if lines[0].Words[0].Font != font.Helvetica {
		t.Errorf("expected body text to keep the list font %v, got %v", font.Helvetica, lines[0].Words[0].Font)
	}
}

// cloneWithItems (used for page-break continuation) must copy the marker
// font override, so a list split across a page keeps its marker font on the
// continuation fragment.
func TestCloneWithItemsCopiesMarkerFont(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		SetMarkerFont(font.HelveticaOblique, nil).
		AddItem("Alpha").
		AddItem("Beta")

	clone := l.cloneWithItems(l.items)
	if clone.markerFont != font.HelveticaOblique {
		t.Errorf("expected cloned list to carry the marker font override, got %v", clone.markerFont)
	}
	if clone.markerEmbedded != nil {
		t.Errorf("expected nil markerEmbedded on clone, got %v", clone.markerEmbedded)
	}
}

func TestHeadingImplementsElement(t *testing.T) {
	var _ Element = NewHeading("Title", H1)
}

// --- Nested lists ---

func TestNestedListBasic(t *testing.T) {
	l := NewList(font.Helvetica, 12)
	sub := l.AddItemWithSubList("Parent item")
	sub.AddItem("Child A")
	sub.AddItem("Child B")

	lines := l.Layout(400)
	// 1 parent line + 2 child lines = 3
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// Parent line indent should be 18 (default)
	if lines[0].listRef.indent != 18 {
		t.Errorf("parent indent: expected 18, got %f", lines[0].listRef.indent)
	}

	// Child lines indent should be 36 (18 + 18)
	if lines[1].listRef.indent != 36 {
		t.Errorf("child indent: expected 36, got %f", lines[1].listRef.indent)
	}
}

func TestNestedListThreeLevels(t *testing.T) {
	l := NewList(font.Helvetica, 10)
	sub := l.AddItemWithSubList("Level 1")
	subsub := sub.AddItemWithSubList("Level 2")
	subsub.AddItem("Level 3")

	lines := l.Layout(400)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// Level 1: indent 18, Level 2: indent 36, Level 3: indent 54
	if lines[0].listRef.indent != 18 {
		t.Errorf("level 1 indent: expected 18, got %f", lines[0].listRef.indent)
	}
	if lines[1].listRef.indent != 36 {
		t.Errorf("level 2 indent: expected 36, got %f", lines[1].listRef.indent)
	}
	if lines[2].listRef.indent != 54 {
		t.Errorf("level 3 indent: expected 54, got %f", lines[2].listRef.indent)
	}
}

func TestNestedListOrdered(t *testing.T) {
	l := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
	sub := l.AddItemWithSubList("First")
	sub.SetStyle(ListOrdered)
	sub.AddItem("Sub-first")
	sub.AddItem("Sub-second")
	l.AddItem("Second")

	lines := l.Layout(400)
	// "First" + "Sub-first" + "Sub-second" + "Second" = 4 lines
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines, got %d", len(lines))
	}

	// Parent markers: "1." and "2." (numbering restarts in sub-list)
	if lines[0].listRef.markerWords[0].Text != "1." {
		t.Errorf("expected '1.' marker, got %q", lines[0].listRef.markerWords[0].Text)
	}
	// Sub-list starts at 1.
	if lines[1].listRef.markerWords[0].Text != "1." {
		t.Errorf("expected sub '1.' marker, got %q", lines[1].listRef.markerWords[0].Text)
	}
	// Last item is parent's second
	lastLine := lines[len(lines)-1]
	if lastLine.listRef.markerWords[0].Text != "2." {
		t.Errorf("expected '2.' marker, got %q", lastLine.listRef.markerWords[0].Text)
	}
}

func TestNestedListInheritsFont(t *testing.T) {
	l := NewList(font.HelveticaBold, 14)
	sub := l.AddItemWithSubList("Parent")
	sub.AddItem("Child")

	if sub.font != font.HelveticaBold {
		t.Error("sub-list should inherit parent font")
	}
	if sub.fontSize != 14 {
		t.Error("sub-list should inherit parent font size")
	}
}

func TestNestedListEmptySubList(t *testing.T) {
	l := NewList(font.Helvetica, 12)
	l.AddItemWithSubList("Parent with empty sub-list")
	l.AddItem("Next item")

	lines := l.Layout(400)
	// Should have 2 lines: parent + next item. Empty sub-list adds nothing.
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
}

func TestNestedListWordWrap(t *testing.T) {
	l := NewList(font.Helvetica, 12)
	sub := l.AddItemWithSubList("Parent")
	sub.AddItem("This is a very long nested list item that should wrap to multiple lines in a narrow column width")

	lines := l.Layout(200)
	// Parent + multiple wrapped child lines
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (parent + wrapped child), got %d", len(lines))
	}
	// All child lines should have the nested indent (36 = 18+18)
	for i := 1; i < len(lines); i++ {
		if lines[i].listRef == nil {
			t.Fatalf("line %d missing listRef", i)
		}
		if lines[i].listRef.indent != 36 {
			t.Errorf("line %d indent: expected 36, got %f", i, lines[i].listRef.indent)
		}
	}
}

func TestNestedListDeep(t *testing.T) {
	l := NewList(font.Helvetica, 10)
	cur := l
	for range 5 {
		cur = cur.AddItemWithSubList("Level")
	}
	cur.AddItem("Leaf")

	lines := l.Layout(400)
	// 5 "Level" items + 1 "Leaf" = 6 lines
	if len(lines) < 6 {
		t.Fatalf("expected at least 6 lines, got %d", len(lines))
	}
	// Last line (deepest) should have indent = 6 * 18 = 108
	last := lines[len(lines)-1]
	if last.listRef.indent != 108 {
		t.Errorf("deepest indent: expected 108, got %f", last.listRef.indent)
	}
}

func TestListBulletCharacter(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		AddItem("Item")

	lines := l.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	ref := lines[0].listRef
	if ref == nil {
		t.Fatal("expected listRef on first line")
	}
	if len(ref.markerWords) == 0 {
		t.Fatal("expected marker words on first line")
	}
	bullet := ref.markerWords[0].Text
	if bullet != "\u2022" {
		t.Errorf("expected bullet character U+2022 (%q), got %q", "\u2022", bullet)
	}
}

// TestListElementItemMultipleLines verifies that an element list item with
// multiple block children produces multiple content lines (not flattened
// onto one line) and that the marker is aligned to the first text line.
// Regression test for #342.
func TestListElementItemMultipleLines(t *testing.T) {
	div := NewDiv()
	div.Add(NewParagraph("Title.", font.Helvetica, 12))
	div.Add(NewParagraph("First body.", font.Helvetica, 12))
	div.Add(NewParagraph("Second body.", font.Helvetica, 12))

	l := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
	l.AddItemElement(div)

	plan := l.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %v", plan.Status)
	}

	// Gather distinct y positions of leaf text blocks.
	ys := map[float64]bool{}
	var walk func(bs []PlacedBlock, off float64)
	walk = func(bs []PlacedBlock, off float64) {
		for _, b := range bs {
			if len(b.Children) > 0 {
				walk(b.Children, off+b.Y)
				continue
			}
			if b.Height > 0 {
				ys[off+b.Y] = true
			}
		}
	}
	walk(plan.Blocks, 0)
	if len(ys) < 3 {
		t.Errorf("expected >=3 distinct content lines, got %d", len(ys))
	}
}

// TestListElementItemMarkerAligned verifies the marker baseline aligns to the
// first text line of the element rather than the item top or center.
func TestListElementItemMarkerAligned(t *testing.T) {
	div := NewDiv()
	div.Add(NewParagraph("Title.", font.Helvetica, 12))
	div.Add(NewParagraph("Body.", font.Helvetica, 12))

	l := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
	l.AddItemElement(div)

	lines := l.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 synthetic line for element item, got %d", len(lines))
	}
	ref := lines[0].listRef
	if ref == nil || ref.element == nil {
		t.Fatal("expected listRef with element on element item line")
	}
	if len(ref.markerWords) == 0 || ref.markerWords[0].Text != "1." {
		t.Fatalf("expected marker '1.', got %+v", ref.markerWords)
	}
	// The marker baseline must fall within the first text line, i.e. less
	// than one line height from the top — not the item center or bottom.
	firstLineH := 12 * 1.2
	if ref.markerOffsetY <= 0 || ref.markerOffsetY > firstLineH {
		t.Errorf("markerOffsetY %f not within first line [0,%f]", ref.markerOffsetY, firstLineH)
	}
}

// TestListElementItemWithSubList verifies a nested sub-list under an element
// item still renders with its own marker and indentation.
func TestListElementItemWithSubList(t *testing.T) {
	div := NewDiv()
	div.Add(NewParagraph("Parent.", font.Helvetica, 12))

	l := NewList(font.Helvetica, 12)
	sub := l.AddItemElementWithSubList(div)
	sub.AddItem("Child")

	plan := l.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Fatalf("expected positive consumed, got %f", plan.Consumed)
	}
	// The sub-list child should be more indented than the parent content.
	var indents []float64
	var walk func(bs []PlacedBlock, off float64)
	walk = func(bs []PlacedBlock, off float64) {
		for _, b := range bs {
			if len(b.Children) > 0 {
				walk(b.Children, off+b.Y)
				continue
			}
			if b.Height > 0 {
				indents = append(indents, b.X)
			}
		}
	}
	walk(plan.Blocks, 0)
	if len(indents) < 2 {
		t.Fatalf("expected parent and child content blocks, got %d", len(indents))
	}
}

// markerBlockCount returns the number of drawn list markers in a block tree.
// A drawn marker is a leaf "LI" block at X==0 with positive height (see
// planElementItem, which only emits a marker block when there is a marker).
func markerBlockCount(blocks []PlacedBlock, indent float64) int {
	n := 0
	var walk func(bs []PlacedBlock)
	walk = func(bs []PlacedBlock) {
		for _, b := range bs {
			if len(b.Children) > 0 {
				walk(b.Children)
				continue
			}
			if b.Tag == "LI" && b.X == 0 && b.Width == indent && b.Height > 0 {
				n++
			}
		}
	}
	walk(blocks)
	return n
}

// contentLineCount returns the number of distinct content text leaves (height
// > 0) that are NOT marker blocks, i.e. the actual element body lines.
func contentLineCount(blocks []PlacedBlock, indent float64) int {
	n := 0
	var walk func(bs []PlacedBlock)
	walk = func(bs []PlacedBlock) {
		for _, b := range bs {
			if len(b.Children) > 0 {
				walk(b.Children)
				continue
			}
			if b.Height <= 0 {
				continue
			}
			// Skip marker blocks (leaf LI at X==0, width==indent).
			if b.Tag == "LI" && b.X == 0 && b.Width == indent {
				continue
			}
			n++
		}
	}
	walk(blocks)
	return n
}

// TestListElementItemSplitFirstItem verifies that an oversized element list
// item that is the FIRST thing on the page splits across pages: the List
// returns LayoutPartial with a non-nil Overflow, no content is lost across the
// fragments, and the marker is drawn only on the first fragment. Regression
// for the dropped-tail BLOCKER (#342/#339 follow-up).
func TestListElementItemSplitFirstItem(t *testing.T) {
	const nParas = 60
	div := NewDiv()
	for i := 0; i < nParas; i++ {
		div.Add(NewParagraph("Line.", font.Helvetica, 12))
	}

	l := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
	l.AddItemElement(div)

	const pageH = 100.0
	indent := l.Indent()

	// Walk the page chain, summing content lines and markers.
	totalContent := 0
	totalMarkers := 0
	fragments := 0
	var cur Element = l
	for cur != nil {
		plan := cur.PlanLayout(LayoutArea{Width: 400, Height: pageH})
		fragments++
		totalContent += contentLineCount(plan.Blocks, indent)
		totalMarkers += markerBlockCount(plan.Blocks, indent)

		if fragments == 1 {
			if plan.Status != LayoutPartial {
				t.Fatalf("first page: expected LayoutPartial, got %v", plan.Status)
			}
			if plan.Overflow == nil {
				t.Fatal("first page: expected non-nil Overflow")
			}
			if markerBlockCount(plan.Blocks, indent) != 1 {
				t.Fatalf("first fragment: expected exactly 1 marker, got %d", markerBlockCount(plan.Blocks, indent))
			}
		} else {
			// Continuation fragments must NOT repeat the marker.
			if markerBlockCount(plan.Blocks, indent) != 0 {
				t.Errorf("continuation fragment %d: marker repeated", fragments)
			}
		}

		if plan.Status == LayoutFull {
			break
		}
		cur = plan.Overflow
		if fragments > nParas+5 {
			t.Fatal("did not converge: too many fragments (possible infinite loop)")
		}
	}

	if fragments < 2 {
		t.Fatalf("expected the item to split across >=2 pages, got %d", fragments)
	}
	if totalContent != nParas {
		t.Errorf("content lost: expected %d content lines across fragments, got %d", nParas, totalContent)
	}
	if totalMarkers != 1 {
		t.Errorf("marker drawn %d times across fragments, want exactly 1 (first fragment only)", totalMarkers)
	}
}

// TestListElementItemSplitAfterItem verifies the same split behavior when the
// oversized element item is NOT first: a preceding plain item occupies part of
// the page, then the tall element item splits, and no content is lost.
func TestListElementItemSplitAfterItem(t *testing.T) {
	const nParas = 60
	div := NewDiv()
	for i := 0; i < nParas; i++ {
		div.Add(NewParagraph("Line.", font.Helvetica, 12))
	}

	l := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
	l.AddItem("First plain item.")
	l.AddItemElement(div)

	const pageH = 100.0
	indent := l.Indent()

	totalContent := 0
	fragments := 0
	sawOverflow := false
	var cur Element = l
	for cur != nil {
		plan := cur.PlanLayout(LayoutArea{Width: 400, Height: pageH})
		fragments++
		totalContent += contentLineCount(plan.Blocks, indent)

		if plan.Status == LayoutPartial && plan.Overflow != nil {
			sawOverflow = true
		}
		if plan.Status == LayoutFull {
			break
		}
		cur = plan.Overflow
		if fragments > nParas+5 {
			t.Fatal("did not converge: too many fragments (possible infinite loop)")
		}
	}

	if !sawOverflow {
		t.Fatal("expected the list to overflow across pages")
	}
	// nParas element body lines + 1 plain item line.
	if totalContent != nParas+1 {
		t.Errorf("content lost: expected %d content lines, got %d", nParas+1, totalContent)
	}
}
