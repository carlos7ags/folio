// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/barcode"
	"github.com/carlos7ags/folio/font"
)

func TestBarcodeElementCode128(t *testing.T) {
	bc, err := barcode.NewCode128("Hello123")
	if err != nil {
		t.Fatal(err)
	}

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Barcode below:", font.Helvetica, 12))
	r.Add(NewBarcodeElement(bc, 200))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	content := string(pages[0].Stream.Bytes())
	// Barcode draws filled rectangles.
	if !strings.Contains(content, "re") {
		t.Error("expected rectangle operators from barcode")
	}
}

func TestBarcodeElementQR(t *testing.T) {
	bc, err := barcode.NewQR("https://example.com")
	if err != nil {
		t.Fatal(err)
	}

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewBarcodeElement(bc, 150).SetAlign(AlignCenter))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestBarcodeElementEAN13(t *testing.T) {
	bc, err := barcode.NewEAN13("590123412345")
	if err != nil {
		t.Fatal(err)
	}

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewBarcodeElement(bc, 180).SetHeight(40))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestBarcodeElementPlanLayout(t *testing.T) {
	bc, err := barcode.NewCode128("Test")
	if err != nil {
		t.Fatal(err)
	}

	be := NewBarcodeElement(bc, 200)
	plan := be.PlanLayout(LayoutArea{Width: 468, Height: 500})

	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
	if len(plan.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(plan.Blocks))
	}
	if plan.Blocks[0].Tag != "Figure" {
		t.Errorf("tag = %q, want Figure", plan.Blocks[0].Tag)
	}
}

func TestBarcodeElementNoSpace(t *testing.T) {
	bc, err := barcode.NewCode128("Test")
	if err != nil {
		t.Fatal(err)
	}

	be := NewBarcodeElement(bc, 200)
	plan := be.PlanLayout(LayoutArea{Width: 468, Height: 1})

	if plan.Status != LayoutNothing {
		t.Errorf("expected LayoutNothing for tiny height, got %d", plan.Status)
	}
}

func TestBarcodeElementMeasurable(t *testing.T) {
	bc, err := barcode.NewCode128("Test")
	if err != nil {
		t.Fatal(err)
	}

	be := NewBarcodeElement(bc, 200)
	if be.MinWidth() != 200 {
		t.Errorf("MinWidth = %.1f, want 200", be.MinWidth())
	}
	if be.MaxWidth() != 200 {
		t.Errorf("MaxWidth = %.1f, want 200", be.MaxWidth())
	}
}
