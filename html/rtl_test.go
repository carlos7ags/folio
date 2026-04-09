// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// TestHTMLDirRTLAttribute verifies that dir="rtl" on an HTML element
// produces a right-aligned paragraph with reversed word order.
func TestHTMLDirRTLAttribute(t *testing.T) {
	src := `<p dir="rtl">Hello world</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}
	// dir="rtl" forces RTL direction on the paragraph.
	if p.Direction() != layout.DirectionRTL {
		t.Errorf("direction: got %v, want DirectionRTL", p.Direction())
	}
	plan := p.PlanLayout(layout.LayoutArea{Width: 500, Height: 200})
	if plan.Status != layout.LayoutFull || len(plan.Blocks) == 0 {
		t.Fatal("layout failed")
	}
	// RTL paragraphs should right-align by default → X > 0.
	if plan.Blocks[0].X <= 0 {
		t.Errorf("dir=rtl paragraph should right-align; X=%v", plan.Blocks[0].X)
	}
}

// TestCSSDirectionRTL verifies that CSS direction:rtl works.
func TestCSSDirectionRTL(t *testing.T) {
	src := `<html><head><style>
		.rtl { direction: rtl; }
	</style></head><body>
		<p class="rtl">Hello world</p>
	</body></html>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}
	if p.Direction() != layout.DirectionRTL {
		t.Errorf("direction: got %v, want DirectionRTL", p.Direction())
	}
}

// TestDirInheritance verifies that dir="rtl" on a parent div is
// inherited by child paragraphs.
func TestDirInheritance(t *testing.T) {
	src := `<div dir="rtl"><p>Hello</p><p>World</p></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Both paragraphs inside the RTL div should inherit the direction.
	for i, e := range elems {
		p, ok := e.(*layout.Paragraph)
		if !ok {
			continue
		}
		if p.Direction() != layout.DirectionRTL {
			t.Errorf("paragraph %d: direction=%v, want DirectionRTL", i, p.Direction())
		}
	}
}

// TestCSSDirectionOverridesDirAttribute verifies that an explicit CSS
// direction:ltr declaration overrides the HTML dir="rtl" attribute.
func TestCSSDirectionOverridesDirAttribute(t *testing.T) {
	src := `<html><head><style>
		p { direction: ltr; }
	</style></head><body>
		<p dir="rtl">Hello</p>
	</body></html>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}
	// CSS direction:ltr should win over dir="rtl".
	if p.Direction() != layout.DirectionLTR {
		t.Errorf("CSS should override dir attr: got %v, want DirectionLTR", p.Direction())
	}
}

// TestDirAutoAttribute verifies dir="auto" (auto-detect from content).
func TestDirAutoAttribute(t *testing.T) {
	src := `<p dir="auto">Hello</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}
	// dir="auto" should not set a forced direction — let the bidi
	// algorithm auto-detect from the text content.
	if p.Direction() != layout.DirectionAuto {
		t.Errorf("dir=auto: got %v, want DirectionAuto", p.Direction())
	}
}

// TestDefaultDirectionIsAuto verifies that paragraphs without any dir
// or direction declaration use DirectionAuto (no forced direction).
func TestDefaultDirectionIsAuto(t *testing.T) {
	src := `<p>Hello</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}
	if p.Direction() != layout.DirectionAuto {
		t.Errorf("default direction: got %v, want DirectionAuto", p.Direction())
	}
}
