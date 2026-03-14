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
