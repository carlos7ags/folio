// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

// TestResolveColWidthsCacheHit asserts resolveColWidths returns equal
// (but independently-owned) results when called twice at the same
// maxWidth, and that a row-adding mutator invalidates the cache so
// subsequent calls recompute.
func TestResolveColWidthsCacheHit(t *testing.T) {
	tbl := NewTable()
	tbl.SetAutoColumnWidths()
	row := tbl.AddRow()
	row.AddCell("short", font.Helvetica, 10)
	row.AddCell("another cell", font.Helvetica, 10)

	first := tbl.resolveColWidths(400)
	second := tbl.resolveColWidths(400)
	if len(first) != len(second) {
		t.Fatalf("column count mismatch: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Errorf("column %d: %v vs %v (cache should be stable)", i, first[i], second[i])
		}
	}

	// Mutate through the returned slice; the cache must not alias it.
	if len(second) > 0 {
		second[0] = -1
	}
	third := tbl.resolveColWidths(400)
	if len(third) > 0 && third[0] == -1 {
		t.Error("resolveColWidths returned an aliased slice; caller mutation leaked into the cache")
	}

	// Adding a row with much wider content should change auto-sized widths
	// after invalidation.
	row2 := tbl.AddRow()
	row2.AddCell("short", font.Helvetica, 10)
	row2.AddCell("a very much longer cell content that widens the column", font.Helvetica, 10)

	fourth := tbl.resolveColWidths(400)
	changed := false
	for i := range fourth {
		if i < len(third) && fourth[i] != third[i] {
			changed = true
		}
	}
	if !changed {
		t.Error("expected widths to change after AddRow invalidated the cache")
	}
}

// TestResolveColWidthsCacheKeyedByWidth asserts the memo is keyed by
// maxWidth: querying at a different width must not return the width
// cached for a previous call.
func TestResolveColWidthsCacheKeyedByWidth(t *testing.T) {
	tbl := NewTable()
	tbl.SetAutoColumnWidths()
	row := tbl.AddRow()
	row.AddCell("a fairly long cell of text content", font.Helvetica, 10)
	row.AddCell("another fairly long cell of text content", font.Helvetica, 10)

	at300 := tbl.resolveColWidths(150)
	at600 := tbl.resolveColWidths(1000)

	sum := func(ws []float64) float64 {
		s := 0.0
		for _, w := range ws {
			s += w
		}
		return s
	}
	if sum(at300) == sum(at600) {
		t.Error("expected different total widths for different maxWidth values")
	}

	// Re-querying 150 after having cached 1000 must recompute (miss), not
	// return the stale 1000 result.
	at300Again := tbl.resolveColWidths(150)
	if sum(at300Again) != sum(at300) {
		t.Errorf("width-keyed cache miss returned wrong result: got %v, want %v", sum(at300Again), sum(at300))
	}
}

// TestTableOverflowReusesColWidthsCache is the multi-page identity guard:
// a continuation table produced by PlanLayout's page-break path must
// report the same resolveColWidths result as the source table at the
// same width — the caching mechanism's whole point is that this no
// longer requires re-measuring every remaining row's cells.
func TestTableOverflowReusesColWidthsCache(t *testing.T) {
	tbl := NewTable()
	tbl.SetAutoColumnWidths()
	for range 20 {
		row := tbl.AddRow()
		row.AddCell("Widget", font.Helvetica, 10)
		row.AddCell("1", font.Helvetica, 10)
		row.AddCell("$10.00", font.Helvetica, 10)
	}

	const areaWidth = 400.0
	plan := tbl.PlanLayout(LayoutArea{Width: areaWidth, Height: 60})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial to exercise the overflow path, got %v", plan.Status)
	}
	overflow, ok := plan.Overflow.(*Table)
	if !ok {
		t.Fatalf("expected overflow to be *Table, got %T", plan.Overflow)
	}

	if !overflow.colWidthsValid {
		t.Error("expected cloneForOverflow to carry a valid colWidths cache forward")
	}
	want := tbl.resolveColWidths(areaWidth)
	got := overflow.resolveColWidths(areaWidth)
	if len(want) != len(got) {
		t.Fatalf("column count: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if want[i] != got[i] {
			t.Errorf("column %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

// TestParagraphMeasureCacheInvalidation asserts AddRun invalidates the
// memoized MaxWidth/MinWidth so subsequent calls reflect the new text.
func TestParagraphMeasureCacheInvalidation(t *testing.T) {
	p := NewParagraph("short", font.Helvetica, 12)
	before := p.MaxWidth()

	p.AddRun(NewRun(" much longer additional text", font.Helvetica, 12))
	after := p.MaxWidth()

	if after <= before {
		t.Errorf("expected MaxWidth to grow after AddRun: before=%v after=%v", before, after)
	}
}

// TestParagraphMeasureCacheStable asserts repeated MinWidth/MaxWidth calls
// on an unmutated paragraph return identical results (the cached path).
func TestParagraphMeasureCacheStable(t *testing.T) {
	p := NewParagraph("The quick brown fox jumps over the lazy dog", font.Helvetica, 12)

	min1, max1 := p.MinWidth(), p.MaxWidth()
	min2, max2 := p.MinWidth(), p.MaxWidth()

	if min1 != min2 {
		t.Errorf("MinWidth not stable across cached calls: %v vs %v", min1, min2)
	}
	if max1 != max2 {
		t.Errorf("MaxWidth not stable across cached calls: %v vs %v", max1, max2)
	}
}
