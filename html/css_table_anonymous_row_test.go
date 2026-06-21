// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// TestCSSTableAnonymousRow verifies that consecutive bare display:table-cell
// children of a display:table element (with no display:table-row wrapper) are
// grouped into a single CSS anonymous table-row box and laid out side by side,
// rather than each becoming its own one-cell row and stacking vertically.
func TestCSSTableAnonymousRow(t *testing.T) {
	src := `<div style="display:table; width:100%;">
  <div style="display:table-cell; width:50%;"><h2>Left column</h2><p>Left line one.</p><p>Left line two.</p></div>
  <div style="display:table-cell; width:50%;"><h2>Right column</h2><p>Right line one.</p></div>
</div>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least one element")
	}

	// The table may be elems[0] directly, or wrapped in a Div if margins were
	// applied. PlanLayout on whichever element we get resolves both cases.
	el := elems[0]
	plan := el.PlanLayout(layout.LayoutArea{Width: 500, Height: 1e9})
	if plan.Status != layout.LayoutFull {
		t.Fatalf("unexpected plan status %v", plan.Status)
	}
	if len(plan.Blocks) == 0 {
		t.Fatal("expected at least one placed block")
	}

	// Primary, metric-independent assertion: the two consecutive bare cells must
	// form exactly ONE anonymous table-row. The bug produced one row PER cell
	// (two TR blocks here), which stacked them vertically.
	if rows := countTaggedBlocks(plan.Blocks, "TR"); rows != 1 {
		t.Fatalf("expected 1 anonymous table-row for two bare table-cell children, got %d; "+
			"consecutive cells should share a single row", rows)
	}

	// Geometric corroboration: a single row is about as tall as the taller cell,
	// never the sum of both. Measure each cell at the ~250pt column width it gets
	// in the real 50/50 layout so the baseline matches (full-width baselines would
	// under-measure, since narrower columns reflow taller).
	const colW = 250.0
	leftH := measureCellHeight(t, `<div style="display:table; width:100%;">
  <div style="display:table-cell; width:100%;"><h2>Left column</h2><p>Left line one.</p><p>Left line two.</p></div>
</div>`, colW)
	rightH := measureCellHeight(t, `<div style="display:table; width:100%;">
  <div style="display:table-cell; width:100%;"><h2>Right column</h2><p>Right line one.</p></div>
</div>`, colW)

	taller := math.Max(leftH, rightH)
	stackedSum := leftH + rightH
	if plan.Consumed >= stackedSum-0.5 {
		t.Fatalf("cells appear stacked: Consumed=%.2f >= stackedSum=%.2f (leftH=%.2f rightH=%.2f)",
			plan.Consumed, stackedSum, leftH, rightH)
	}
	if math.Abs(plan.Consumed-taller) > 0.5*taller+2 {
		t.Fatalf("Consumed=%.2f not close to a single row height (taller cell=%.2f)", plan.Consumed, taller)
	}
}

// countTaggedBlocks counts blocks (recursively) whose Tag equals tag.
func countTaggedBlocks(blocks []layout.PlacedBlock, tag string) int {
	n := 0
	for _, b := range blocks {
		if b.Tag == tag {
			n++
		}
		n += countTaggedBlocks(b.Children, tag)
	}
	return n
}

// measureCellHeight lays out a single-cell display:table at the given column
// width and returns its consumed height, used as the per-cell baseline for the
// side-by-side check.
func measureCellHeight(t *testing.T, src string, width float64) float64 {
	t.Helper()
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least one element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: width, Height: 1e9})
	return plan.Consumed
}
