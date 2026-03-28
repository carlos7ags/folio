// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"testing"

	"github.com/carlos7ags/folio/layout"
)

func TestMarkerPseudoElementColor(t *testing.T) {
	src := `<style>li::marker { color: red; }</style>
	<ul><li>Item one</li><li>Item two</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

func TestMarkerPseudoElementFontSize(t *testing.T) {
	src := `<style>li::marker { font-size: 20px; }</style>
	<ul><li>Item one</li><li>Item two</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestMarkerPseudoElementColorAndSize(t *testing.T) {
	src := `<style>li::marker { color: #00ff00; font-size: 18px; }</style>
	<ol><li>First</li><li>Second</li><li>Third</li></ol>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestMarkerPseudoElementNoEffect(t *testing.T) {
	// Without ::marker, list should render normally.
	src := `<ul><li>Item one</li><li>Item two</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestMarkerPseudoElementTextUnaffected(t *testing.T) {
	// ::marker styling should not affect text color of items.
	src := `<style>li::marker { color: blue; } li { color: black; }</style>
	<ul><li>Item text should be black</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}
