// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestBalancedColumns(t *testing.T) {
	elements := make([]Element, 6)
	for i := range elements {
		elements[i] = NewParagraph("Item text", font.Helvetica, 12)
	}

	cols := BalancedColumns(3, 12, elements...)
	plan := cols.PlanLayout(LayoutArea{Width: 468, Height: 500})

	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

func TestBalancedColumnsEqualizesHeight(t *testing.T) {
	// Create elements with varying line counts so that round-robin
	// would produce visibly unequal columns. The redistribution
	// algorithm should pack them to roughly equal heights.
	short := NewParagraph("Short.", font.Helvetica, 12)
	medium := NewParagraph("Medium text that wraps to two lines in a narrow column for testing.", font.Helvetica, 12)
	long := NewParagraph("Long paragraph that will definitely wrap to several lines when laid "+
		"out in a narrow column to simulate realistic unbalanced content.", font.Helvetica, 12)

	cols := BalancedColumns(2, 12, short, medium, long)
	plan := cols.PlanLayout(LayoutArea{Width: 300, Height: 1000})

	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}

	// Measure what each column actually consumed by laying out the
	// distributed columns independently.
	colWidths := cols.resolveWidths(300)
	_, colHeights := cols.layoutColumns(colWidths, 1e9)

	if len(colHeights) != 2 {
		t.Fatalf("expected 2 column heights, got %d", len(colHeights))
	}

	// The taller column should be at most 2x the shorter. With this
	// test input the optimal split is 1.5:1 because the long paragraph
	// alone is taller than the other two combined; the threshold allows
	// headroom for font-metric variation while still catching the old
	// round-robin behavior that could leave a column empty entirely.
	taller := max(colHeights[0], colHeights[1])
	shorter := min(colHeights[0], colHeights[1])
	if shorter == 0 {
		t.Fatal("shorter column has zero height; redistribution left a column empty")
	}
	ratio := taller / shorter
	if ratio > 2.0 {
		t.Errorf("columns not balanced: heights %.1f and %.1f (ratio %.2f, want <= 2.0)",
			colHeights[0], colHeights[1], ratio)
	}
}

func TestBalancedColumnsSingleElement(t *testing.T) {
	elem := NewParagraph("Only one element", font.Helvetica, 12)
	cols := BalancedColumns(3, 12, elem)
	plan := cols.PlanLayout(LayoutArea{Width: 468, Height: 500})

	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

func TestBalancedColumnsEmpty(t *testing.T) {
	cols := BalancedColumns(2, 12)
	plan := cols.PlanLayout(LayoutArea{Width: 468, Height: 500})

	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
}

func TestBalancedColumnsNonUniform(t *testing.T) {
	// 5 elements across 3 columns: redistribution should not leave
	// any column empty when there are enough elements.
	elements := make([]Element, 5)
	for i := range elements {
		elements[i] = NewParagraph("Line of text", font.Helvetica, 12)
	}

	cols := BalancedColumns(3, 12, elements...)
	plan := cols.PlanLayout(LayoutArea{Width: 468, Height: 500})

	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %d", plan.Status)
	}

	// All 3 columns should have at least one element.
	for i, elems := range cols.elements {
		if len(elems) == 0 {
			t.Errorf("column %d is empty after redistribution (5 elements across 3 cols)", i)
		}
	}
}

func TestBalancedColumnsPreservesDocumentOrder(t *testing.T) {
	// The critical property from #145: elements must appear in their
	// original document order across columns, not interleaved.
	// With 6 equal paragraphs across 3 columns, balanced distribution
	// should produce col0=[0,1], col1=[2,3], col2=[4,5]. Round-robin
	// would have produced col0=[0,3], col1=[1,4], col2=[2,5].
	originals := make([]Element, 6)
	for i := range originals {
		originals[i] = NewParagraph("Para", font.Helvetica, 12)
	}

	cols := BalancedColumns(3, 12, originals...)
	cols.PlanLayout(LayoutArea{Width: 468, Height: 500})

	// After redistribution, each column's elements should be a
	// contiguous slice of the original list.
	seen := 0
	for colIdx, elems := range cols.elements {
		for _, elem := range elems {
			found := false
			for i := seen; i < len(originals); i++ {
				if elem == originals[i] {
					seen = i + 1
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("column %d contains an element out of document order (seen=%d)", colIdx, seen)
			}
		}
	}
	if seen != len(originals) {
		t.Errorf("only %d of %d elements accounted for", seen, len(originals))
	}
}

func TestBalancedColumnsIdempotent(t *testing.T) {
	elements := make([]Element, 4)
	for i := range elements {
		elements[i] = NewParagraph("Paragraph text", font.Helvetica, 12)
	}

	cols := BalancedColumns(2, 12, elements...)
	plan1 := cols.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	plan2 := cols.PlanLayout(LayoutArea{Width: 400, Height: 1000})

	if plan1.Consumed != plan2.Consumed {
		t.Errorf("PlanLayout not idempotent: consumed %.2f then %.2f", plan1.Consumed, plan2.Consumed)
	}
	if len(plan1.Blocks) != len(plan2.Blocks) {
		t.Errorf("PlanLayout not idempotent: %d blocks then %d", len(plan1.Blocks), len(plan2.Blocks))
	}
}

func TestBalancedColumnsManyElements(t *testing.T) {
	// 10 elements across 2 columns. With equal-height paragraphs the
	// balanced algorithm should place 5 in each column.
	elements := make([]Element, 10)
	for i := range elements {
		elements[i] = NewParagraph("Same line", font.Helvetica, 12)
	}

	cols := BalancedColumns(2, 12, elements...)
	cols.PlanLayout(LayoutArea{Width: 400, Height: 1000})

	if len(cols.elements[0]) != 5 || len(cols.elements[1]) != 5 {
		t.Errorf("expected 5/5 split, got %d/%d", len(cols.elements[0]), len(cols.elements[1]))
	}
}

func TestBalancedColumnsVaryingHeights(t *testing.T) {
	// One very tall element followed by several short ones. The tall
	// element should land alone in one column; the short ones share
	// the other. This is the scenario where round-robin was worst.
	tall := NewParagraph(
		"This paragraph is intentionally very long so it produces many "+
			"wrapped lines when laid out in a narrow column, simulating a "+
			"realistic scenario where one element dominates the content "+
			"height and the balanced algorithm must avoid pairing it with "+
			"other elements that would make one column much taller.",
		font.Helvetica, 12)
	shorts := make([]Element, 4)
	for i := range shorts {
		shorts[i] = NewParagraph("Short.", font.Helvetica, 12)
	}
	all := append([]Element{tall}, shorts...)

	cols := BalancedColumns(2, 12, all...)
	cols.PlanLayout(LayoutArea{Width: 200, Height: 2000})

	colWidths := cols.resolveWidths(200)
	_, colHeights := cols.layoutColumns(colWidths, 1e9)

	taller := max(colHeights[0], colHeights[1])
	shorter := min(colHeights[0], colHeights[1])
	if shorter == 0 {
		t.Fatal("one column is empty")
	}

	// Log heights for visibility.
	t.Logf("column heights: %.1f and %.1f (ratio %.2f)", colHeights[0], colHeights[1], taller/shorter)

	// The tall paragraph alone is taller than all shorts combined, so
	// perfect balance is impossible. But the algorithm must at least
	// keep the short paragraphs together rather than mixing them with
	// the tall one. All shorts should be in the same column.
	if len(cols.elements[0]) == 1 {
		// Tall element alone in col 0, shorts in col 1.
		if len(cols.elements[1]) != 4 {
			t.Errorf("expected 1/4 split, got %d/%d", len(cols.elements[0]), len(cols.elements[1]))
		}
	} else if len(cols.elements[1]) == 1 {
		// Shorts in col 0, tall alone in col 1.
		if len(cols.elements[0]) != 4 {
			t.Errorf("expected 4/1 split, got %d/%d", len(cols.elements[0]), len(cols.elements[1]))
		}
	} else {
		t.Errorf("expected one column to have exactly 1 element (the tall paragraph), got %d/%d",
			len(cols.elements[0]), len(cols.elements[1]))
	}
}

func TestNonBalancedColumnsUnchanged(t *testing.T) {
	// Verify that direct Add() usage without SetBalanced(true) still
	// works and does NOT redistribute.
	cols := NewColumns(2).SetGap(12)
	p1 := NewParagraph("Left", font.Helvetica, 12)
	p2 := NewParagraph("Right", font.Helvetica, 12)
	cols.Add(0, p1)
	cols.Add(1, p2)

	cols.PlanLayout(LayoutArea{Width: 400, Height: 500})

	if len(cols.elements[0]) != 1 || cols.elements[0][0] != p1 {
		t.Error("column 0 should contain only p1")
	}
	if len(cols.elements[1]) != 1 || cols.elements[1][0] != p2 {
		t.Error("column 1 should contain only p2")
	}
}

func TestBalancedColumnsRendering(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	elements := make([]Element, 6)
	for i := range elements {
		elements[i] = NewParagraph("Column content", font.Helvetica, 12)
	}
	r.Add(BalancedColumns(2, 12, elements...))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "Tj") {
		t.Error("expected text content")
	}
}

func TestColumnsPlanLayoutFull(t *testing.T) {
	cols := NewColumns(2).SetGap(12)
	cols.Add(0, NewParagraph("Left column", font.Helvetica, 12))
	cols.Add(1, NewParagraph("Right column", font.Helvetica, 12))

	plan := cols.PlanLayout(LayoutArea{Width: 468, Height: 500})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
	if len(plan.Blocks) == 0 {
		t.Error("expected blocks")
	}
}

func TestColumnsPlanLayoutNoSpace(t *testing.T) {
	cols := NewColumns(2)
	cols.Add(0, NewParagraph("Left", font.Helvetica, 12))
	cols.Add(1, NewParagraph("Right", font.Helvetica, 12))

	plan := cols.PlanLayout(LayoutArea{Width: 468, Height: 1})
	if plan.Status != LayoutNothing {
		t.Errorf("expected LayoutNothing, got %d", plan.Status)
	}
}
