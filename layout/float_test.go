// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestFloatLeftBasic(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	// Float a narrow element to the left.
	floatContent := NewParagraph("FLOAT", font.HelveticaBold, 12)
	r.Add(NewFloat(FloatLeft, floatContent).SetMargin(10))
	r.Add(NewParagraph("This text should wrap around the floated element on the right side.", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "Tj") {
		t.Error("expected text content")
	}
}

func TestFloatRightBasic(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	floatContent := NewParagraph("RIGHT", font.HelveticaBold, 12)
	r.Add(NewFloat(FloatRight, floatContent).SetMargin(10))
	r.Add(NewParagraph("This text should wrap on the left side of the right-floated element.", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestFloatDoesNotConsumeHeight(t *testing.T) {
	f := NewFloat(FloatLeft, NewParagraph("Float", font.Helvetica, 12))
	plan := f.PlanLayout(LayoutArea{Width: 468, Height: 500})
	if plan.Consumed != 0 {
		t.Errorf("Float should not consume vertical space, got %.1f", plan.Consumed)
	}
}

func TestFloatMeasurable(t *testing.T) {
	f := NewFloat(FloatLeft, NewParagraph("Hello World", font.Helvetica, 12))
	if f.MinWidth() <= 0 {
		t.Error("Float MinWidth should be positive")
	}
	if f.MaxWidth() < f.MinWidth() {
		t.Error("Float MaxWidth should be >= MinWidth")
	}
}

func TestFloatWithMultipleParagraphs(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	r.Add(NewFloat(FloatLeft, NewParagraph("SIDEBAR", font.HelveticaBold, 10)))
	r.Add(NewParagraph("First paragraph wrapping around float.", font.Helvetica, 12))
	r.Add(NewParagraph("Second paragraph also wrapping.", font.Helvetica, 12))
	r.Add(NewParagraph("Third paragraph, float may have cleared by now.", font.Helvetica, 12))

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestFloatRendersContent(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewFloat(FloatLeft, NewParagraph("Floated!", font.Helvetica, 12)))

	pages := r.Render()
	content := string(pages[0].Stream.Bytes())
	// The float's content should be drawn.
	if !strings.Contains(content, "Tj") {
		t.Error("expected text from float content")
	}
}
