// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/layout"
)

// TestIssue130_Gap2_LinearGradientProducesImageXObject confirms that a
// full HTML→layout pipeline renders a linearGradient-filled rect as a
// real gradient (registered as an image XObject) rather than a flat
// first-stop color. Regression for issue #130 gap 2.
func TestIssue130_Gap2_LinearGradientProducesImageXObject(t *testing.T) {
	src := `<svg width="200" height="80" viewBox="0 0 200 80">
	  <defs>
	    <linearGradient id="g1" x1="0" y1="0" x2="1" y2="0">
	      <stop offset="0" stop-color="#0f172a"/>
	      <stop offset="1" stop-color="#0d9488"/>
	    </linearGradient>
	  </defs>
	  <rect width="200" height="80" fill="url(#g1)"/>
	</svg>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	svgEl, ok := elems[0].(*layout.SVGElement)
	if !ok {
		t.Fatalf("expected SVGElement, got %T", elems[0])
	}

	plan := svgEl.PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
	if plan.Status != layout.LayoutFull || len(plan.Blocks) == 0 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	page := &layout.PageResult{Stream: content.NewStream()}
	plan.Blocks[0].Draw(layout.DrawContext{Stream: page.Stream, Page: page}, 0, 100)

	if len(page.Images) == 0 {
		t.Fatal("expected a gradient image XObject registered on the page")
	}
	out := string(page.Stream.Bytes())
	if !strings.Contains(out, " Do") {
		t.Errorf("expected Do operator for gradient image, got:\n%s", out)
	}
}

// TestIssue130_Gap2_RadialGradientProducesImageXObject covers radial
// gradients via the full pipeline.
func TestIssue130_Gap2_RadialGradientProducesImageXObject(t *testing.T) {
	src := `<svg width="100" height="100" viewBox="0 0 100 100">
	  <defs>
	    <radialGradient id="g2">
	      <stop offset="0" stop-color="white"/>
	      <stop offset="1" stop-color="black"/>
	    </radialGradient>
	  </defs>
	  <circle cx="50" cy="50" r="40" fill="url(#g2)"/>
	</svg>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	svgEl, ok := elems[0].(*layout.SVGElement)
	if !ok {
		t.Fatalf("expected SVGElement, got %T", elems[0])
	}
	plan := svgEl.PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
	page := &layout.PageResult{Stream: content.NewStream()}
	plan.Blocks[0].Draw(layout.DrawContext{Stream: page.Stream, Page: page}, 0, 100)

	if len(page.Images) == 0 {
		t.Error("expected radial gradient image XObject on the page")
	}
}

// TestIssue130_Gap1_SVGImageDataURIRegisters confirms that an SVG containing
// an <image> element with a data-URI PNG produces a Do operator in the
// rendered content stream and registers an image XObject on the page.
func TestIssue130_Gap1_SVGImageDataURIRegisters(t *testing.T) {
	// 1×1 red PNG, base64.
	const redDot = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	src := `<svg width="50" height="50" viewBox="0 0 50 50"><image x="5" y="5" width="40" height="40" href="data:image/png;base64,` + redDot + `"/></svg>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least one element")
	}
	svgEl, ok := elems[0].(*layout.SVGElement)
	if !ok {
		t.Fatalf("expected *layout.SVGElement, got %T", elems[0])
	}

	// Exercise the Draw path directly through PlanLayout.
	plan := svgEl.PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
	if plan.Status != layout.LayoutFull {
		t.Fatalf("unexpected plan status %v", plan.Status)
	}
	if len(plan.Blocks) == 0 {
		t.Fatal("expected a placed block")
	}

	page := &layout.PageResult{Stream: content.NewStream()}
	ctx := layout.DrawContext{Stream: page.Stream, Page: page}
	plan.Blocks[0].Draw(ctx, 0, 100)

	if len(page.Images) == 0 {
		t.Fatalf("expected at least one image registered on the page")
	}
	out := string(page.Stream.Bytes())
	if !strings.Contains(out, " Do") {
		t.Errorf("expected a Do operator in the content stream, got:\n%s", out)
	}
}
