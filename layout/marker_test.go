// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestListMarkerColor(t *testing.T) {
	l := NewList(font.Helvetica, 12)
	red := ColorRed
	l.SetMarkerColor(red)
	l.AddItem("Item one")
	l.AddItem("Item two")

	lines := l.Layout(300)
	if len(lines) == 0 {
		t.Fatal("expected lines")
	}
	// Verify the list renders without error.
	plan := l.PlanLayout(LayoutArea{Width: 300, Height: 500})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestListMarkerFontSize(t *testing.T) {
	l := NewList(font.Helvetica, 12)
	l.SetMarkerFontSize(20)
	l.AddItem("Item one")
	l.AddItem("Item two")

	lines := l.Layout(300)
	if len(lines) == 0 {
		t.Fatal("expected lines")
	}
}

func TestListMarkerColorAndSize(t *testing.T) {
	l := NewList(font.Helvetica, 12)
	l.SetMarkerColor(ColorBlue)
	l.SetMarkerFontSize(16)
	l.SetStyle(ListOrdered)
	l.AddItem("First")
	l.AddItem("Second")
	l.AddItem("Third")

	plan := l.PlanLayout(LayoutArea{Width: 300, Height: 500})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

func TestListMarkerDefaultUnchanged(t *testing.T) {
	// Without marker overrides, behavior unchanged.
	l := NewList(font.Helvetica, 12)
	l.AddItem("Item")

	plan := l.PlanLayout(LayoutArea{Width: 300, Height: 500})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}
