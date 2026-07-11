// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"

	htmlparse "golang.org/x/net/html"
)

func TestConvertSimpleParagraph(t *testing.T) {
	elems, err := Convert("<p>Hello World</p>", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed height, got %f", plan.Consumed)
	}
}

func TestConvertHeadings(t *testing.T) {
	html := `<h1>Title</h1><h2>Subtitle</h2><p>Body text.</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elems))
	}
	for i, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
		if plan.Consumed <= 0 {
			t.Errorf("element %d: expected positive consumed, got %f", i, plan.Consumed)
		}
	}
}

// TestConvertHeadingWithLink verifies that <a href> inside headings
// produces clickable link annotations. Regression test for #26.
func TestConvertHeadingWithLink(t *testing.T) {
	htmlStr := `<h2><a href="https://example.com">Linked Heading</a></h2>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if !planContainsLink(plan, "https://example.com") {
		t.Error("expected link annotation with URI 'https://example.com' on heading")
	}
}

// TestConvertHeadingMixedTextAndLink verifies that a heading with both
// plain text and an inline link produces a link only for the linked part.
func TestConvertHeadingMixedTextAndLink(t *testing.T) {
	htmlStr := `<h3>Read the <a href="https://example.com/docs">documentation</a> first</h3>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if !planContainsLink(plan, "https://example.com/docs") {
		t.Error("expected link annotation for 'https://example.com/docs' in heading")
	}
}

// planContainsLink recursively searches a LayoutPlan's blocks (including
// children) for a link annotation with the given URI.
func planContainsLink(plan layout.LayoutPlan, uri string) bool {
	for _, b := range plan.Blocks {
		if blockContainsLink(b, uri) {
			return true
		}
	}
	return false
}

func blockContainsLink(b layout.PlacedBlock, uri string) bool {
	for _, link := range b.Links {
		if link.URI == uri {
			return true
		}
	}
	for _, child := range b.Children {
		if blockContainsLink(child, uri) {
			return true
		}
	}
	return false
}

func TestConvertInlineStyles(t *testing.T) {
	html := `<p>Normal <strong>bold</strong> <em>italic</em> text.</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestConvertUnorderedList(t *testing.T) {
	html := `<ul><li>First</li><li>Second</li><li>Third</li></ul>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestConvertOrderedList(t *testing.T) {
	html := `<ol><li>First</li><li>Second</li></ol>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

// TestConvertListItemWithLink verifies that <a href> inside <li> produces
// clickable link annotations. Regression test for #27.
func TestConvertListItemWithLink(t *testing.T) {
	htmlStr := `<ul><li><a href="https://example.com">Linked item</a></li></ul>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if !planContainsLink(plan, "https://example.com") {
		t.Error("expected link annotation with URI 'https://example.com' in list item")
	}
}

// TestConvertListItemMixedTextAndLink verifies inline links within list
// item text produce link annotations.
func TestConvertListItemMixedTextAndLink(t *testing.T) {
	htmlStr := `<ul><li>Visit <a href="https://example.com">our site</a> today</li></ul>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if !planContainsLink(plan, "https://example.com") {
		t.Error("expected link annotation in list item with mixed text")
	}
}

func TestConvertDiv(t *testing.T) {
	html := `<div style="padding: 10px; background-color: #f0f0f0"><p>Inside div</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestConvertInlineStyle(t *testing.T) {
	html := `<p style="color: red; font-size: 18px; text-align: center">Styled</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestConvertDisplayNone(t *testing.T) {
	html := `<p>Visible</p><div style="display: none">Hidden</div><p>Also visible</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 2 {
		t.Fatalf("expected 2 elements (hidden div skipped), got %d", len(elems))
	}
}

func TestConvertBr(t *testing.T) {
	html := `<p>Line one</p><br><p>Line two</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elems))
	}
}

func TestConvertEmptyHTML(t *testing.T) {
	elems, err := Convert("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 0 {
		t.Fatalf("expected 0 elements, got %d", len(elems))
	}
}

func TestConvertOptions(t *testing.T) {
	elems, err := Convert("<p>Big text</p>", &Options{DefaultFontSize: 24})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestParseColor(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"red", true},
		{"#ff0000", true},
		{"#f00", true},
		{"#f00a", true},     // #RGBA
		{"#ff000080", true}, // #RRGGBBAA
		{"rgb(255, 0, 0)", true},
		{"rgba(255, 0, 0, 0.5)", true},
		{"rgba(100%, 0%, 0%, 1)", true},
		{"hsl(0, 100%, 50%)", true},
		{"hsla(120, 100%, 50%, 0.3)", true},
		{"hsl(240, 100%, 50%)", true},
		{"transparent", false},
		{"", false},
		{"notacolor", false},
	}
	for _, tt := range tests {
		_, ok := parseColor(tt.input)
		if ok != tt.ok {
			t.Errorf("parseColor(%q): got ok=%v, want %v", tt.input, ok, tt.ok)
		}
	}
}

func TestParseColorComponentClamping(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"0", 0},
		{"255", 1.0},
		{"128", 128.0 / 255},
		{"300", 1.0}, // clamped to 1.0
		{"-50", 0},   // clamped to 0
		{"100%", 1.0},
		{"0%", 0},
		{"150%", 1.0}, // clamped to 1.0
		{"-10%", 0},   // clamped to 0
	}
	for _, tt := range tests {
		c, ok := parseColor("rgb(" + tt.input + ", 0, 0)")
		if !ok {
			t.Fatalf("parseColor(rgb(%s, 0, 0)) failed to parse", tt.input)
		}
		if diff := c.R - tt.want; diff > 0.001 || diff < -0.001 {
			t.Errorf("parseColor(rgb(%s, 0, 0)).R = %f, want %f", tt.input, c.R, tt.want)
		}
	}
}

func TestParseColorOutOfRange(t *testing.T) {
	// rgb(300, -50, 128) should clamp to valid range.
	c, ok := parseColor("rgb(300, -50, 128)")
	if !ok {
		t.Fatal("expected ok=true for rgb with out-of-range values")
	}
	if c.R != 1.0 {
		t.Errorf("expected R=1.0 (clamped from 300), got %f", c.R)
	}
	if c.G != 0 {
		t.Errorf("expected G=0.0 (clamped from -50), got %f", c.G)
	}
}

func TestParseColorAlphaValues(t *testing.T) {
	tests := []struct {
		input string
		alpha float64
	}{
		{"red", 1.0},
		{"#ff0000", 1.0},
		{"#ff000080", 128.0 / 255},
		{"rgba(255, 0, 0, 0.5)", 0.5},
		{"rgba(255, 0, 0, 0)", 0},
		{"rgba(255, 0, 0, 1)", 1.0},
		{"hsla(0, 100%, 50%, 0.3)", 0.3},
	}
	for _, tt := range tests {
		_, alpha, ok := parseColorAlpha(tt.input)
		if !ok {
			t.Errorf("parseColorAlpha(%q): expected ok=true", tt.input)
			continue
		}
		if diff := alpha - tt.alpha; diff > 0.01 || diff < -0.01 {
			t.Errorf("parseColorAlpha(%q): alpha=%f, want %f", tt.input, alpha, tt.alpha)
		}
	}
}

func TestRGBSpaceSeparated(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
		r, g  float64 // expected R, G (approximate)
		alpha float64
	}{
		// Space-separated without alpha.
		{"rgb(255 0 0)", true, 1, 0, 1},
		{"rgb(0 128 255)", true, 0, 128.0 / 255, 1},
		// Space-separated with / alpha.
		{"rgb(255 0 0 / 0.5)", true, 1, 0, 0.5},
		{"rgb(255 0 0 / 1)", true, 1, 0, 1},
		{"rgb(255 0 0 / 0)", true, 1, 0, 0},
		// Percentage form.
		{"rgb(100% 0% 50%)", true, 1, 0, 1},
		{"rgb(100% 0% 50% / 0.8)", true, 1, 0, 0.8},
		// Percentage alpha.
		{"rgb(255 0 0 / 50%)", true, 1, 0, 0.5},
		// rgba() with space-separated form.
		{"rgba(255 0 0 / 0.3)", true, 1, 0, 0.3},
		// Legacy comma form still works.
		{"rgb(255, 0, 0)", true, 1, 0, 1},
		{"rgba(255, 0, 0, 0.5)", true, 1, 0, 0.5},
	}
	for _, tt := range tests {
		c, alpha, ok := parseColorAlpha(tt.input)
		if ok != tt.ok {
			t.Errorf("parseColorAlpha(%q): ok=%v, want %v", tt.input, ok, tt.ok)
			continue
		}
		if !ok {
			continue
		}
		if diff := c.R - tt.r; diff > 0.01 || diff < -0.01 {
			t.Errorf("parseColorAlpha(%q): R=%f, want %f", tt.input, c.R, tt.r)
		}
		if diff := c.G - tt.g; diff > 0.01 || diff < -0.01 {
			t.Errorf("parseColorAlpha(%q): G=%f, want %f", tt.input, c.G, tt.g)
		}
		if diff := alpha - tt.alpha; diff > 0.01 || diff < -0.01 {
			t.Errorf("parseColorAlpha(%q): alpha=%f, want %f", tt.input, alpha, tt.alpha)
		}
	}
}

func TestHSLSpaceSeparated(t *testing.T) {
	// hsl(120 100% 50%) = pure green, hsl(120 100% 50% / 0.5) = green with alpha.
	tests := []struct {
		input string
		ok    bool
		alpha float64
	}{
		{"hsl(120 100% 50%)", true, 1},
		{"hsl(120 100% 50% / 0.5)", true, 0.5},
		{"hsl(120 100% 50% / 50%)", true, 0.5},
		{"hsla(120 100% 50% / 0.3)", true, 0.3},
		{"hsl(0, 100%, 50%)", true, 1},
	}
	for _, tt := range tests {
		_, alpha, ok := parseColorAlpha(tt.input)
		if ok != tt.ok {
			t.Errorf("parseColorAlpha(%q): ok=%v, want %v", tt.input, ok, tt.ok)
			continue
		}
		if diff := alpha - tt.alpha; diff > 0.01 || diff < -0.01 {
			t.Errorf("parseColorAlpha(%q): alpha=%f, want %f", tt.input, alpha, tt.alpha)
		}
	}
}

func TestHSLColors(t *testing.T) {
	tests := []struct {
		input   string
		r, g, b float64 // expected RGB 0-1
	}{
		{"hsl(0, 100%, 50%)", 1, 0, 0},     // red
		{"hsl(120, 100%, 50%)", 0, 1, 0},   // green
		{"hsl(240, 100%, 50%)", 0, 0, 1},   // blue
		{"hsl(0, 0%, 50%)", 0.5, 0.5, 0.5}, // gray
		{"hsl(0, 0%, 0%)", 0, 0, 0},        // black
		{"hsl(0, 0%, 100%)", 1, 1, 1},      // white
	}
	for _, tt := range tests {
		c, ok := parseColor(tt.input)
		if !ok {
			t.Errorf("parseColor(%q): expected ok=true", tt.input)
			continue
		}
		if diff := c.R - tt.r; diff > 0.02 || diff < -0.02 {
			t.Errorf("parseColor(%q): R=%f, want %f", tt.input, c.R, tt.r)
		}
		if diff := c.G - tt.g; diff > 0.02 || diff < -0.02 {
			t.Errorf("parseColor(%q): G=%f, want %f", tt.input, c.G, tt.g)
		}
		if diff := c.B - tt.b; diff > 0.02 || diff < -0.02 {
			t.Errorf("parseColor(%q): B=%f, want %f", tt.input, c.B, tt.b)
		}
	}
}

func TestParseBoxShadowSingle(t *testing.T) {
	shadows := parseBoxShadows("2px 4px 6px rgba(0, 0, 0, 0.3)", 12)
	if len(shadows) != 1 {
		t.Fatalf("expected 1 shadow, got %d", len(shadows))
	}
	if shadows[0].OffsetX == 0 && shadows[0].OffsetY == 0 {
		t.Error("expected non-zero offsets")
	}
}

func TestParseBoxShadowMultiple(t *testing.T) {
	// Two shadows separated by comma — commas inside rgba() must not split.
	shadows := parseBoxShadows("2px 2px 4px rgba(0, 0, 0, 0.5), 0 0 10px rgba(255, 0, 0, 0.3)", 12)
	if len(shadows) != 2 {
		t.Fatalf("expected 2 shadows, got %d", len(shadows))
	}
}

func TestParseBoxShadowThree(t *testing.T) {
	shadows := parseBoxShadows("1px 1px 2px black, 0 0 5px red, -1px -1px 3px blue", 12)
	if len(shadows) != 3 {
		t.Fatalf("expected 3 shadows, got %d", len(shadows))
	}
}

func TestParseBoxShadowNone(t *testing.T) {
	shadows := parseBoxShadows("none", 12)
	if len(shadows) != 0 {
		t.Errorf("expected 0 shadows for 'none', got %d", len(shadows))
	}
}

func TestParseBoxShadowInset(t *testing.T) {
	shadows := parseBoxShadows("inset 0 2px 4px rgba(0,0,0,0.2)", 12)
	if len(shadows) != 1 {
		t.Fatalf("expected 1 shadow, got %d", len(shadows))
	}
	if !shadows[0].Inset {
		t.Error("expected inset shadow")
	}
}

// TestParseBoxShadowWithCalc is a regression test for the same
// strings.Fields tokenization bug fixed for `flex:` (#236),
// `margin:`/`padding:` (#237), `font:` (#240), `border:` (#242),
// `background-size:` (#244), `@page size` (#247), `gap:` (#249),
// and `border-radius:` (#252) — applied here to `box-shadow`. Pre-fix
// `box-shadow: calc(2px + 2px) 4px black` became 5 tokens
// ["calc(2px", "+", "2px)", "4px", "black"]; the calc fragments all
// failed parseLength and were accumulated into the colorToken,
// leaving only one length (4px) — len(lengths) < 2 → both offsets
// stayed 0 → the shadow was effectively invisible.
//
// rgb()/rgba()/hsl() with internal whitespace happened to work
// pre-fix by coincidence: the comma-separated fragments all failed
// parseLength and were re-joined with " " into colorToken, which
// parseColor then accepted. Post-fix it works directly via a single
// token — same observable behavior, cleaner code path.
// TestParseBoxShadowsWithCalcMultiple exercises the parseBoxShadows
// (plural) wrapper with calc inside one of the comma-separated entries.
// splitTopLevelCommas already keeps commas inside parens intact (so
// rgba(255, 0, 0, 0.5) survives), and parseBoxShadow now keeps calc
// lengths intact too — they should compose without interference.
func TestParseBoxShadowsWithCalcMultiple(t *testing.T) {
	shadows := parseBoxShadows("calc(2px + 2px) 4px red, 0 0 4px rgba(0, 0, 255, 0.5)", 12)
	if len(shadows) != 2 {
		t.Fatalf("expected 2 shadows, got %d", len(shadows))
	}
	// First: calc offset → 3pt; 4px → 3pt; red.
	if math.Abs(shadows[0].OffsetX-3) > 0.01 || math.Abs(shadows[0].OffsetY-3) > 0.01 {
		t.Errorf("shadows[0] offsets = (%.4f, %.4f), want (3, 3)",
			shadows[0].OffsetX, shadows[0].OffsetY)
	}
	if math.Abs(shadows[0].Color.R-1) > 0.01 {
		t.Errorf("shadows[0].Color.R = %.4f, want 1", shadows[0].Color.R)
	}
	// Second: 0/0/4px=3pt blur, blue.
	if math.Abs(shadows[1].Blur-3) > 0.01 {
		t.Errorf("shadows[1].Blur = %.4f, want 3", shadows[1].Blur)
	}
	if math.Abs(shadows[1].Color.B-1) > 0.01 {
		t.Errorf("shadows[1].Color.B = %.4f, want 1 (blue)", shadows[1].Color.B)
	}
}

func TestParseBoxShadowWithCalc(t *testing.T) {
	tests := []struct {
		name                string
		val                 string
		wantX               float64 // in pt
		wantY               float64
		wantBlur            float64
		wantSpread          float64
		wantR, wantG, wantB float64
		wantInset           bool
	}{
		{
			name: "canonical regression: calc(2px + 2px) 4px black",
			// The exact pre-fix regression case. Pre-fix the calc was
			// shredded into ["calc(2px","+","2px)"], all three failed
			// parseLength and were accumulated into colorToken — leaving
			// only one length (4px), so OffsetX/Y stayed 0 and the
			// shadow was effectively invisible.
			val:   "calc(2px + 2px) 4px black",
			wantX: 3, wantY: 3,
		},
		{
			name: "calc offsetX, plain offsetY, plain color",
			val:  "calc(2px + 2px) 6px black",
			// 4px = 3pt; 6px = 4.5pt.
			wantX: 3, wantY: 4.5,
		},
		{
			name: "hex color with calc lengths",
			val:  "calc(2px + 2px) 4px #ff0000",
			// Hex color is single-token; covers the parseColor branch
			// alongside named/rgb/rgba/hsl.
			wantX: 3, wantY: 3,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name: "inset + calc + rgba",
			// Exercises all three accumulators (inset, calc length, rgba
			// color) in one shot.
			val:   "inset calc(2px + 2px) 4px rgba(255, 0, 0, 0.5)",
			wantX: 3, wantY: 3,
			wantR: 1, wantG: 0, wantB: 0,
			wantInset: true,
		},
		{
			name: "plain offsets, calc blur",
			val:  "2px 4px calc(8px + 4px) red",
			// X=1.5pt, Y=3pt, blur=12px=9pt; red.
			wantX: 1.5, wantY: 3, wantBlur: 9,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name: "calc on all four length slots",
			val:  "calc(2px + 0px) calc(4px + 0px) calc(8px + 0px) calc(2px + 0px) red",
			// X=2px=1.5pt; Y=4px=3pt; blur=8px=6pt; spread=2px=1.5pt.
			wantX: 1.5, wantY: 3, wantBlur: 6, wantSpread: 1.5,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name: "min() and max() in length slots",
			val:  "min(2px, 4px) max(4px, 8px) red",
			// X=min(2px,4px)=1.5pt; Y=max(4px,8px)=6pt.
			wantX: 1.5, wantY: 6,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name: "clamp() in length slot",
			val:  "clamp(1px, 4px, 8px) 4px red",
			// X=clamp middle=4px=3pt.
			wantX: 3, wantY: 3,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name: "calc with subtraction",
			val:  "calc(8px - 4px) 4px red",
			// X=4px=3pt.
			wantX: 3, wantY: 3,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name: "calc with multiplication",
			val:  "calc(2px * 2) 4px red",
			// X=4px=3pt.
			wantX: 3, wantY: 3,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name: "calc with division",
			val:  "calc(8px / 2) 4px red",
			// X=4px=3pt.
			wantX: 3, wantY: 3,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name:  "calc + rgb color (both compound)",
			val:   "calc(2px + 2px) 4px rgb(255, 0, 0)",
			wantX: 3, wantY: 3,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name: "calc + rgba color (both compound)",
			val:  "calc(2px + 2px) 4px rgba(255, 0, 0, 0.5)",
			// Color alpha is not tracked on layout.Color; assert RGB only.
			wantX: 3, wantY: 3,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name: "calc + hsl color (both compound)",
			val:  "calc(2px + 2px) 4px hsl(0, 100%, 50%)",
			// hsl(0, 100%, 50%) is pure red.
			wantX: 3, wantY: 3,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name:  "inset + calc",
			val:   "inset calc(2px + 2px) 4px red",
			wantX: 3, wantY: 3,
			wantR: 1, wantG: 0, wantB: 0,
			wantInset: true,
		},
		{
			name:  "tab and newline separators",
			val:   "calc(2px + 2px)\t4px\nred",
			wantX: 3, wantY: 3,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name: "fewer than 2 lengths: shadow constructed with zeros",
			// Parser only checks parts >= 2, NOT lengths >= 2. With one
			// length + one color, bs is non-nil but OffsetX/OffsetY stay
			// at zero defaults. Documents the existing contract.
			val:   "calc(2px + 2px) red",
			wantX: 0, wantY: 0,
			wantR: 1, wantG: 0, wantB: 0,
		},
		{
			name: "unbalanced calc paren: nil shadow",
			// splitTopLevelFields keeps the unbalanced calc + trailing
			// chars as one giant token. parts == 1 → early return nil.
			val:   "calc(2px + 2px 4px red",
			wantR: -1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := parseBoxShadow(tt.val, 12)
			if tt.wantR == -1 {
				if bs != nil {
					t.Errorf("expected nil shadow, got %+v", bs)
				}
				return
			}
			if bs == nil {
				t.Fatalf("got nil shadow")
			}
			if math.Abs(bs.OffsetX-tt.wantX) > 0.01 {
				t.Errorf("OffsetX = %.4f, want %.4f", bs.OffsetX, tt.wantX)
			}
			if math.Abs(bs.OffsetY-tt.wantY) > 0.01 {
				t.Errorf("OffsetY = %.4f, want %.4f", bs.OffsetY, tt.wantY)
			}
			if math.Abs(bs.Blur-tt.wantBlur) > 0.01 {
				t.Errorf("Blur = %.4f, want %.4f", bs.Blur, tt.wantBlur)
			}
			if math.Abs(bs.Spread-tt.wantSpread) > 0.01 {
				t.Errorf("Spread = %.4f, want %.4f", bs.Spread, tt.wantSpread)
			}
			if math.Abs(bs.Color.R-tt.wantR) > 0.01 ||
				math.Abs(bs.Color.G-tt.wantG) > 0.01 ||
				math.Abs(bs.Color.B-tt.wantB) > 0.01 {
				t.Errorf("Color = %+v, want R=%.2f G=%.2f B=%.2f",
					bs.Color, tt.wantR, tt.wantG, tt.wantB)
			}
			if bs.Inset != tt.wantInset {
				t.Errorf("Inset = %v, want %v", bs.Inset, tt.wantInset)
			}
		})
	}
}

func TestBoxShadowHTMLMultiple(t *testing.T) {
	src := `<div style="box-shadow: 2px 2px 4px rgba(0,0,0,0.5), 0 0 10px red; padding: 10px;">
		<p>Shadowed content</p>
	</div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	// Verify it produces a valid layout.
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	if plan.Status != layout.LayoutFull {
		t.Error("expected LayoutFull")
	}
}

func TestSplitTopLevelCommas(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"a, b, c", 3},
		{"rgba(0,0,0,0.5) 2px, red 1px", 2},
		{"no commas here", 1},
		{"rgba(1,2,3), rgba(4,5,6)", 2},
		{"a(b,c), d(e,f), g", 3},
	}
	for _, tt := range tests {
		parts := splitTopLevelCommas(tt.input)
		if len(parts) != tt.want {
			t.Errorf("splitTopLevelCommas(%q): got %d parts, want %d", tt.input, len(parts), tt.want)
		}
	}
}

func TestSplitTopLevelFields(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"0 0 auto", []string{"0", "0", "auto"}},
		{"0 0 calc(50% - 8px)", []string{"0", "0", "calc(50% - 8px)"}},
		{"1 1 min(100px, 50%)", []string{"1", "1", "min(100px, 50%)"}},
		{"  spaced   words  ", []string{"spaced", "words"}},
		{"clamp(10px, 5%, 20px) auto", []string{"clamp(10px, 5%, 20px)", "auto"}},
		{"calc(1px + calc(2px - 1px))", []string{"calc(1px + calc(2px - 1px))"}},
		{"a((b c) d) e", []string{"a((b c) d)", "e"}},
		{"a\tb\nc\rd", []string{"a", "b", "c", "d"}},
		{"  calc(50% - 8px)  ", []string{"calc(50% - 8px)"}},
		// Unbalanced parens: '(' opens depth that never closes — the
		// remainder is consumed as a single token (mirrors splitTopLevelCommas).
		{"a (b c", []string{"a", "(b c"}},
		// Stray ')' is treated as a literal character; depth stays at 0.
		{"a) b", []string{"a)", "b"}},
		{"   ", nil},
		{"", nil},
	}
	for _, tt := range tests {
		got := splitTopLevelFields(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("splitTopLevelFields(%q): got %v (%d), want %v (%d)",
				tt.input, got, len(got), tt.want, len(tt.want))
			continue
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Errorf("splitTopLevelFields(%q)[%d] = %q, want %q",
					tt.input, i, got[i], tt.want[i])
			}
		}
	}
}

func TestIndexByteAtTopLevel(t *testing.T) {
	tests := []struct {
		input string
		b     byte
		want  int
	}{
		// Basic finds.
		{"a/b", '/', 1},
		{"hello/world", '/', 5},
		// Byte at index 0.
		{"/abc", '/', 0},
		// Slash inside parens is skipped.
		{"calc(2em / 2)", '/', -1},
		{"calc(1em + 2px)/1.5", '/', 15},
		{"calc(1em + 2px)/calc(1em * 1.5)", '/', 15},
		// Slash immediately after a closing paren is at depth 0 again.
		{"calc(1)/2", '/', 7},
		// "First top-level match wins" combined with "skip inside parens":
		// the slash at index 5 is inside the calc; the next at index 9
		// is at depth 0.
		{"calc(1/2)/x", '/', 9},
		// Plain "first wins" with no parens.
		{"a/b/c", '/', 1},
		// Slash inside nested parens is skipped.
		{"min(calc(1/2), 3)", '/', -1},
		// Unbalanced opener consumes the rest at depth>0.
		{"calc(1/2", '/', -1},
		// Unbalanced closer is a literal character; depth stays at 0.
		{"a)b/c", '/', 3},
		// Stray opener followed by balanced sub-expression: depth never
		// returns to 0, so a `/` after the inner close paren is still
		// at depth 1 and gets skipped.
		{"((a)/b", '/', -1},
		// Char never appears.
		{"abc", '/', -1},
		// Empty / whitespace-only input.
		{"", '/', -1},
		{"   ", '/', -1},
		// Other separator chars.
		{"a,b", ',', 1},
		{"rgb(0, 0, 0)", ',', -1},
	}
	for _, tt := range tests {
		got := indexByteAtTopLevel(tt.input, tt.b)
		if got != tt.want {
			t.Errorf("indexByteAtTopLevel(%q, %q) = %d, want %d",
				tt.input, tt.b, got, tt.want)
		}
	}
}

func TestParseFlexShorthandWithCalc(t *testing.T) {
	// Regression: `flex: 0 0 calc(50% - 8px)` was split on whitespace,
	// yielding 5 tokens; no shorthand case matched and FlexBasis stayed nil.
	// Cover all shorthand arities (1/2/3) and operator variants.
	cases := []struct {
		name        string
		input       string
		wantGrow    float64
		wantShrink  float64 // initial value 1 unless the shorthand sets it
		wantBasisAt float64 // expected toPoints(relativeTo=200, fontSize=12)
	}{
		// Three-token forms set all three.
		{"three: 0 0 calc(50% - 8px)", "0 0 calc(50% - 8px)", 0, 0, 200*0.5 - 6},
		{"three: 1 0 calc(100% / 2)", "1 0 calc(100% / 2)", 1, 0, 200 / 2},
		{"three: 1 1 calc(50% + 10px)", "1 1 calc(50% + 10px)", 1, 1, 200*0.5 + 10*0.75},
		{"three: 1 1 calc(2 * 25%)", "1 1 calc(2 * 25%)", 1, 1, 2 * 200 * 0.25},
		// Two-token form: `<flex-grow> <flex-basis>`. The parser sets grow
		// from parts[0] but does not touch FlexShrink — it stays at 1.
		{"two: 1 calc(50% - 8px)", "1 calc(50% - 8px)", 1, 1, 200*0.5 - 6},
		// Single-token form: a non-numeric token is parsed as the basis;
		// FlexGrow and FlexShrink keep their initial values (0, 1).
		// Pre-fix this hit case 3 because strings.Fields produced 3 tokens.
		{"one: calc(50% - 8px)", "calc(50% - 8px)", 0, 1, 200*0.5 - 6},
	}
	for _, tc := range cases {
		s := computedStyle{FlexGrow: 0, FlexShrink: 1}
		parseFlexShorthand(tc.input, &s)
		if s.FlexGrow != tc.wantGrow {
			t.Errorf("%s: FlexGrow = %v, want %v", tc.name, s.FlexGrow, tc.wantGrow)
		}
		if s.FlexShrink != tc.wantShrink {
			t.Errorf("%s: FlexShrink = %v, want %v", tc.name, s.FlexShrink, tc.wantShrink)
		}
		if s.FlexBasis == nil {
			t.Errorf("%s: FlexBasis is nil for %q", tc.name, tc.input)
			continue
		}
		got := s.FlexBasis.toPoints(200, 12)
		if math.Abs(got-tc.wantBasisAt) > 0.01 {
			t.Errorf("%s: FlexBasis.toPoints(200, 12) = %.4f, want %.4f",
				tc.name, got, tc.wantBasisAt)
		}
	}
}

func TestParseFlexShorthandWithMinMaxBasis(t *testing.T) {
	cases := []struct {
		input         string
		wantGrow      float64
		wantShrink    float64
		wantBasisAt   float64 // toPoints(relativeTo=200, fontSize=12)
		wantBasisDesc string
	}{
		// min(100px, 50%) → min(75pt, 100pt) at relativeTo=200 → 75pt.
		{"1 1 min(100px, 50%)", 1, 1, 75, "min picks smaller"},
		// max(100px, 30%) → max(75pt, 60pt) at relativeTo=200 → 75pt.
		{"1 0 max(100px, 30%)", 1, 0, 75, "max picks larger"},
		// clamp(50px, 20%, 200px) → clamp(37.5pt, 40pt, 150pt) at 200 → 40pt.
		{"0 0 clamp(50px, 20%, 200px)", 0, 0, 40, "clamp middle wins"},
	}
	for _, tc := range cases {
		s := computedStyle{FlexGrow: 0, FlexShrink: 1}
		parseFlexShorthand(tc.input, &s)
		if s.FlexGrow != tc.wantGrow {
			t.Errorf("%q: FlexGrow = %v, want %v", tc.input, s.FlexGrow, tc.wantGrow)
		}
		if s.FlexShrink != tc.wantShrink {
			t.Errorf("%q: FlexShrink = %v, want %v", tc.input, s.FlexShrink, tc.wantShrink)
		}
		if s.FlexBasis == nil {
			t.Errorf("%q: FlexBasis is nil", tc.input)
			continue
		}
		got := s.FlexBasis.toPoints(200, 12)
		if math.Abs(got-tc.wantBasisAt) > 0.01 {
			t.Errorf("%q (%s): FlexBasis.toPoints(200, 12) = %.4f, want %.4f",
				tc.input, tc.wantBasisDesc, got, tc.wantBasisAt)
		}
	}
}

// TestCSSLengthToUnitValueBranches asserts each branch of the
// length→UnitValue mapping. The lazy CalcUnit branch is the load-bearing
// fix for percentages-inside-calc resolving against the wrong container.
func TestCSSLengthToUnitValueBranches(t *testing.T) {
	const convertContainer = 1000.0 // converter-time width
	const layoutAvailable = 200.0   // layout-time width (intentionally different)

	t.Run("absolute px → eager Pt", func(t *testing.T) {
		l := parseLength("12px")
		uv := cssLengthToUnitValue(l, convertContainer, 12)
		if uv.Unit != layout.UnitPoint {
			t.Errorf("Unit = %d, want UnitPoint", uv.Unit)
		}
		// Resolved at convert time; layout-time available is ignored.
		if got := uv.Resolve(layoutAvailable); math.Abs(got-9) > 0.01 {
			t.Errorf("Resolve(%g) = %g, want 9 (12px = 9pt)", layoutAvailable, got)
		}
	})

	t.Run("plain percent → lazy Pct", func(t *testing.T) {
		l := parseLength("50%")
		uv := cssLengthToUnitValue(l, convertContainer, 12)
		if uv.Unit != layout.UnitPercent {
			t.Errorf("Unit = %d, want UnitPercent", uv.Unit)
		}
		// Pct resolves against the layout-time available, not convertContainer.
		if got := uv.Resolve(layoutAvailable); math.Abs(got-100) > 0.01 {
			t.Errorf("Resolve(%g) = %g, want 100 (50%% of %g)", layoutAvailable, got, layoutAvailable)
		}
	})

	t.Run("calc with percent → lazy CalcUnit", func(t *testing.T) {
		l := parseLength("calc(50% - 8px)")
		uv := cssLengthToUnitValue(l, convertContainer, 12)
		if uv.Unit != layout.UnitCalc {
			t.Errorf("Unit = %d, want UnitCalc (calc with percent must defer)", uv.Unit)
		}
		if uv.Calc == nil {
			t.Fatal("Calc closure is nil")
		}
		// Closure must use layoutAvailable, not convertContainer.
		// 50% of 200 - 8px(=6pt) = 94pt.
		got := uv.Resolve(layoutAvailable)
		if math.Abs(got-94) > 0.01 {
			t.Errorf("Resolve(%g) = %g, want 94 (= 50%%·200 - 6pt). "+
				"If you got 494 the closure captured the convert-time container.", layoutAvailable, got)
		}
		// And resolving against a different available recomputes correctly.
		if got := uv.Resolve(400); math.Abs(got-194) > 0.01 {
			t.Errorf("Resolve(400) = %g, want 194", got)
		}
	})

	t.Run("calc without percent → eager Pt", func(t *testing.T) {
		l := parseLength("calc(10px + 20px)")
		uv := cssLengthToUnitValue(l, convertContainer, 12)
		if uv.Unit != layout.UnitPoint {
			t.Errorf("Unit = %d, want UnitPoint (no percent → no need to defer)", uv.Unit)
		}
		// 30px = 22.5pt at convert time.
		if got := uv.Resolve(layoutAvailable); math.Abs(got-22.5) > 0.01 {
			t.Errorf("Resolve = %g, want 22.5", got)
		}
	})

	t.Run("min(100px, 50%) → lazy CalcUnit", func(t *testing.T) {
		l := parseLength("min(100px, 50%)")
		uv := cssLengthToUnitValue(l, convertContainer, 12)
		if uv.Unit != layout.UnitCalc {
			t.Errorf("Unit = %d, want UnitCalc", uv.Unit)
		}
		// At available=200: min(75pt, 100pt) = 75pt.
		if got := uv.Resolve(200); math.Abs(got-75) > 0.01 {
			t.Errorf("Resolve(200) = %g, want 75", got)
		}
		// At available=100: min(75pt, 50pt) = 50pt — proves the closure re-resolves.
		if got := uv.Resolve(100); math.Abs(got-50) > 0.01 {
			t.Errorf("Resolve(100) = %g, want 50", got)
		}
	})

	t.Run("min without percent → eager Pt", func(t *testing.T) {
		l := parseLength("min(100px, 50px)")
		uv := cssLengthToUnitValue(l, convertContainer, 12)
		if uv.Unit != layout.UnitPoint {
			t.Errorf("Unit = %d, want UnitPoint", uv.Unit)
		}
		if got := uv.Resolve(0); math.Abs(got-37.5) > 0.01 {
			t.Errorf("Resolve = %g, want 37.5 (50px)", got)
		}
	})

	t.Run("nil cssLength → Pt(0)", func(t *testing.T) {
		uv := cssLengthToUnitValue(nil, convertContainer, 12)
		if uv.Unit != layout.UnitPoint || uv.Value != 0 {
			t.Errorf("nil cssLength → %+v, want Pt(0)", uv)
		}
	})
}

// TestDependsOnPercent covers the recursive percent detection over
// calc/min/max/clamp trees, including nested combinations.
func TestDependsOnPercent(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// Plain values: dependsOnPercent only applies to compound expressions,
		// so a plain "%" returns false (the % path is handled separately).
		{"50%", false},
		{"12px", false},
		// calc() variants.
		{"calc(50% - 8px)", true},
		{"calc(10px + 20px)", false},
		{"calc(2 * 25%)", true},
		{"calc(2 * 25px)", false},
		// min/max/clamp variants.
		{"min(100px, 50%)", true},
		{"min(100px, 50px)", false},
		{"max(50%, 100px)", true},
		{"clamp(50px, 20%, 200px)", true},
		{"clamp(50px, 100px, 200px)", false},
		// Function inside function: percent buried under min(calc(...)).
		{"min(100px, calc(2 * 25%))", true},
		{"min(100px, calc(2 * 25px))", false},
	}
	for _, tt := range tests {
		l := parseLength(tt.input)
		if l == nil {
			t.Errorf("%q: parseLength returned nil", tt.input)
			continue
		}
		if got := l.dependsOnPercent(); got != tt.want {
			t.Errorf("%q.dependsOnPercent() = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseLength(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"10px", 7.5},
		{"12pt", 12},
		{"1em", 12},
		{"50%", 6},
		{"auto", 0},
	}
	for _, tt := range tests {
		l := parseLength(tt.input)
		if tt.want == 0 {
			if l != nil {
				t.Errorf("parseLength(%q): expected nil", tt.input)
			}
			continue
		}
		if l == nil {
			t.Errorf("parseLength(%q): got nil", tt.input)
			continue
		}
		got := l.toPoints(12, 12)
		if got < tt.want-0.1 || got > tt.want+0.1 {
			t.Errorf("parseLength(%q).toPoints(12, 12) = %f, want ~%f", tt.input, got, tt.want)
		}
	}
}

func TestParseFontWeight(t *testing.T) {
	if got := parseFontWeight("bold", 400); got != 700 {
		t.Errorf("parseFontWeight(bold) = %d, want 700", got)
	}
	if got := parseFontWeight("700", 400); got != 700 {
		t.Errorf("parseFontWeight(700) = %d, want 700", got)
	}
	if got := parseFontWeight("normal", 400); got != 400 {
		t.Errorf("parseFontWeight(normal) = %d, want 400", got)
	}
	if got := parseFontWeight("400", 400); got != 400 {
		t.Errorf("parseFontWeight(400) = %d, want 400", got)
	}
}

func TestParseMarginShorthand(t *testing.T) {
	top, right, bottom, left := parseMarginShorthand("10px 20px", 12)
	if top != 7.5 || bottom != 7.5 {
		t.Errorf("top/bottom: got %f/%f, want 7.5", top, bottom)
	}
	if right != 15 || left != 15 {
		t.Errorf("right/left: got %f/%f, want 15", right, left)
	}
}

// TestParseMarginShorthandWithCalc is a regression test for the same
// strings.Fields tokenization bug fixed in #236 for `flex:` — applied
// here to `margin:` and `padding:` (both share parseMarginShorthand).
// Pre-fix: `margin: calc(10px + 20px) 0` was split into 4 tokens
// ["calc(10px", "+", "20px)", "0"] and case 4 ran parseLength on
// "calc(10px" (unbalanced), yielding 0 for every side.
//
// Margins/paddings store as float64, so percent-only values like
// `margin: 50%` still resolve eagerly against relativeTo=0 (= 0pt).
// That is a separate, pre-existing limitation of the data model and
// is not addressed here. This PR only fixes the tokenization so calc/
// min/max/clamp values that resolve to a stable point value work.
func TestParseMarginShorthandWithCalc(t *testing.T) {
	tests := []struct {
		name                       string
		input                      string
		fontSize                   float64
		wantT, wantR, wantB, wantL float64
	}{
		{
			name:  "1-side: calc(10px + 20px)",
			input: "calc(10px + 20px)", fontSize: 12,
			// 30px = 22.5pt, applied to all four sides.
			wantT: 22.5, wantR: 22.5, wantB: 22.5, wantL: 22.5,
		},
		{
			name:  "2-side: calc(...) 0",
			input: "calc(10px + 20px) 0", fontSize: 12,
			// top/bottom = 22.5pt, left/right = 0.
			wantT: 22.5, wantR: 0, wantB: 22.5, wantL: 0,
		},
		{
			name:  "3-side: 8px calc(...) 4px",
			input: "8px calc(2px + 6px) 4px", fontSize: 12,
			// top=6, lr=6, bottom=3.
			wantT: 6, wantR: 6, wantB: 3, wantL: 6,
		},
		{
			name:  "4-side: distinct calcs",
			input: "calc(10px + 0px) 8px calc(20px - 4px) 12px", fontSize: 12,
			// 10px=7.5, 8px=6, 16px=12, 12px=9.
			wantT: 7.5, wantR: 6, wantB: 12, wantL: 9,
		},
		{
			name:  "calc with multiplication",
			input: "calc(10px * 2)", fontSize: 12,
			// 20px = 15pt.
			wantT: 15, wantR: 15, wantB: 15, wantL: 15,
		},
		{
			name:  "calc with division",
			input: "calc(20px / 2)", fontSize: 12,
			// 10px = 7.5pt.
			wantT: 7.5, wantR: 7.5, wantB: 7.5, wantL: 7.5,
		},
		{
			name:  "min() basis",
			input: "min(10px, 20px)", fontSize: 12,
			// min picks 10px = 7.5pt, applied uniformly.
			wantT: 7.5, wantR: 7.5, wantB: 7.5, wantL: 7.5,
		},
		{
			name:  "max() basis",
			input: "max(8px, 12px)", fontSize: 12,
			// max picks 12px = 9pt, applied uniformly.
			wantT: 9, wantR: 9, wantB: 9, wantL: 9,
		},
		{
			name:  "clamp() basis",
			input: "clamp(8px, 12px, 20px)", fontSize: 12,
			// middle wins: 12px = 9pt, applied uniformly.
			wantT: 9, wantR: 9, wantB: 9, wantL: 9,
		},
		{
			name:  "tabs and newlines as separators",
			input: "8px\tcalc(2px + 2px)\n4px 12px", fontSize: 12,
			wantT: 6, wantR: 3, wantB: 3, wantL: 9,
		},
		{
			name:  "5+ tokens hit the default branch (returns zeros)",
			input: "1px 2px 3px 4px 5px", fontSize: 12,
			wantT: 0, wantR: 0, wantB: 0, wantL: 0,
		},
		{
			name: "unbalanced calc paren does not crash",
			// splitTopLevelFields keeps the unbalanced calc as a single
			// token, parseLength rejects it as nil, parseBoxSide returns 0.
			input: "calc(10px + 20px", fontSize: 12,
			wantT: 0, wantR: 0, wantB: 0, wantL: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotT, gotR, gotB, gotL := parseMarginShorthand(tt.input, tt.fontSize)
			if math.Abs(gotT-tt.wantT) > 0.01 ||
				math.Abs(gotR-tt.wantR) > 0.01 ||
				math.Abs(gotB-tt.wantB) > 0.01 ||
				math.Abs(gotL-tt.wantL) > 0.01 {
				t.Errorf("parseMarginShorthand(%q) = (%.2f, %.2f, %.2f, %.2f), want (%.2f, %.2f, %.2f, %.2f)",
					tt.input,
					gotT, gotR, gotB, gotL,
					tt.wantT, tt.wantR, tt.wantB, tt.wantL)
			}
		})
	}
}

// TestMarginAutoFlagsWithCalc is a regression test for the auxiliary
// auto-flag scan in `applyProperty("margin", ...)`. parseMarginShorthand
// itself was migrated to splitTopLevelFields in #237, but this scan still
// used strings.Fields — so a calc()/min()/max()/clamp() value with
// internal whitespace would inflate len(parts), shifting the auto-flag
// positions relative to the parsed margin values.
//
// Example pre-fix: `margin: calc(10px + 20px) auto`. parseMarginShorthand
// (post-#237) correctly tokenizes 2 parts → top/bottom=22.5pt, left/right=
// auto. But the auxiliary scan strings.Fields → 5 tokens
// ["calc(10px","+","20px)","auto"] (the trailing 4th value), yielding
// autoFlags = [false,false,false,true,false]; with len(parts)==4 it
// hit case 4 (4-side margin) and only flagged left=auto, missing right.
//
// Post-fix the auxiliary scan uses the same paren-aware tokenizer as
// parseMarginShorthand, so auto-flag positions stay aligned.
func TestMarginAutoFlagsWithCalc(t *testing.T) {
	tests := []struct {
		name          string
		val           string
		wantTopAuto   bool
		wantLeftAuto  bool
		wantRightAuto bool
	}{
		{
			name: "calc top/bottom + auto left/right (centering)",
			// CSS classic: `margin: <length> auto` centers horizontally.
			val:           "calc(10px + 20px) auto",
			wantTopAuto:   false,
			wantLeftAuto:  true,
			wantRightAuto: true,
		},
		{
			name:          "auto top + calc right/left",
			val:           "auto calc(10px + 20px)",
			wantTopAuto:   true,
			wantLeftAuto:  false,
			wantRightAuto: false,
		},
		{
			name: "3-side: auto top + calc lr + plain bottom",
			// CSS 3-value margin grammar: top, left+right, bottom.
			// position 1 (calc) covers BOTH left and right, so only
			// MarginTopAuto is set here.
			val:           "auto calc(10px + 5px) 8px",
			wantTopAuto:   true,
			wantLeftAuto:  false,
			wantRightAuto: false,
		},
		{
			name: "3-side: plain top + auto lr + calc bottom",
			// Position 1 = auto → both Left and Right auto.
			val:           "8px auto calc(10px + 5px)",
			wantTopAuto:   false,
			wantLeftAuto:  true,
			wantRightAuto: true,
		},
		{
			name:          "calc top + auto right + calc bottom + auto left (4-side)",
			val:           "calc(10px + 0px) auto calc(20px - 4px) auto",
			wantTopAuto:   false,
			wantLeftAuto:  true,
			wantRightAuto: true,
		},
		{
			name:          "single auto applies to all sides",
			val:           "auto",
			wantTopAuto:   true,
			wantLeftAuto:  true,
			wantRightAuto: true,
		},
		{
			name:          "single calc value: no auto flags",
			val:           "calc(10px + 20px)",
			wantTopAuto:   false,
			wantLeftAuto:  false,
			wantRightAuto: false,
		},
		{
			name:          "min() top + auto lr (centering with min)",
			val:           "min(10px, 20px) auto",
			wantTopAuto:   false,
			wantLeftAuto:  true,
			wantRightAuto: true,
		},
		{
			name:          "max() top + auto lr",
			val:           "max(10px, 20px) auto",
			wantTopAuto:   false,
			wantLeftAuto:  true,
			wantRightAuto: true,
		},
		{
			name:          "clamp() top + auto lr",
			val:           "clamp(10px, 16px, 24px) auto",
			wantTopAuto:   false,
			wantLeftAuto:  true,
			wantRightAuto: true,
		},
		{
			name:          "tab separator preserves auto positions",
			val:           "calc(10px + 20px)\tauto",
			wantTopAuto:   false,
			wantLeftAuto:  true,
			wantRightAuto: true,
		},
		{
			name:          "newline separator preserves auto positions",
			val:           "calc(10px + 20px)\nauto",
			wantTopAuto:   false,
			wantLeftAuto:  true,
			wantRightAuto: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &converter{}
			style := &computedStyle{FontSize: 12}
			c.applyProperty("margin", tt.val, style)
			if style.MarginTopAuto != tt.wantTopAuto {
				t.Errorf("applyProperty(%q): MarginTopAuto = %v, want %v",
					tt.val, style.MarginTopAuto, tt.wantTopAuto)
			}
			if style.MarginLeftAuto != tt.wantLeftAuto {
				t.Errorf("applyProperty(%q): MarginLeftAuto = %v, want %v",
					tt.val, style.MarginLeftAuto, tt.wantLeftAuto)
			}
			if style.MarginRightAuto != tt.wantRightAuto {
				t.Errorf("applyProperty(%q): MarginRightAuto = %v, want %v",
					tt.val, style.MarginRightAuto, tt.wantRightAuto)
			}
		})
	}
}

// TestPaddingShorthandWithCalcEndToEnd verifies the fix flows through the
// HTML converter for `padding:` (not just `margin:`) — both share
// parseMarginShorthand. Pre-fix, all four padding sides became 0 because
// the calc value was shredded by strings.Fields.
func TestPaddingShorthandWithCalcEndToEnd(t *testing.T) {
	htmlDoc := `<!DOCTYPE html><html><body>
<div style="padding: calc(10px + 20px) 8px;">content</div>
</body></html>`

	elems, err := Convert(htmlDoc, &Options{PageWidth: 600, PageHeight: 800})
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if len(elems) == 0 {
		t.Fatal("no elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 1e9})
	// Walk for the styled Div (the converter wraps body in a Div).
	// The padded Div will have height >= 22.5pt (top) + line height + 22.5pt (bottom).
	var found bool
	var walk func(blocks []layout.PlacedBlock)
	walk = func(blocks []layout.PlacedBlock) {
		for _, b := range blocks {
			// 30px top + 30px bottom = 45pt of vertical padding alone.
			if b.Height >= 45 && b.Width > 0 {
				found = true
				return
			}
			walk(b.Children)
			if found {
				return
			}
		}
	}
	walk(plan.Blocks)
	if !found {
		t.Errorf("expected a block with at least 45pt height (padding: 30px top + 30px bottom resolved); pre-fix this was 0")
	}
}

// --- Table tests ---

func TestConvertSimpleTable(t *testing.T) {
	html := `<table>
		<tr><td>A1</td><td>B1</td></tr>
		<tr><td>A2</td><td>B2</td></tr>
	</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element (table), got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status == layout.LayoutNothing {
		t.Error("table got LayoutNothing")
	}
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestConvertTableWithHeader(t *testing.T) {
	html := `<table border="1">
		<thead><tr><th>Name</th><th>Value</th></tr></thead>
		<tbody>
			<tr><td>Alpha</td><td>100</td></tr>
			<tr><td>Beta</td><td>200</td></tr>
		</tbody>
	</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestConvertTableColspan(t *testing.T) {
	html := `<table>
		<tr><td colspan="2">Spanning</td></tr>
		<tr><td>A</td><td>B</td></tr>
	</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status == layout.LayoutNothing {
		t.Error("table got LayoutNothing")
	}
}

// --- Link tests ---

func TestConvertExternalLink(t *testing.T) {
	html := `<a href="https://example.com">Click here</a>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestConvertInternalLink(t *testing.T) {
	html := `<a href="#section1">Go to section 1</a>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestConvertLinkInParagraph(t *testing.T) {
	html := `<p>Visit <a href="https://example.com">our site</a> for more.</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatalf("expected at least 1 element, got %d", len(elems))
	}
}

// TestConvertLinkInParagraphProducesAnnotation verifies that <a href="...">
// inside a <p> produces a PlacedBlock with a Link annotation, so the
// document layer can create a clickable PDF annotation. Regression test for #23.
func TestConvertLinkInParagraphProducesAnnotation(t *testing.T) {
	htmlStr := `<p><a href="https://example.com">Click here</a></p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	found := false
	for _, b := range plan.Blocks {
		for _, link := range b.Links {
			if link.URI == "https://example.com" {
				found = true
			}
		}
	}
	if !found {
		t.Error("expected a PlacedBlock with Links containing URI 'https://example.com'")
	}
}

// TestConvertMixedTextAndLinkInParagraph verifies that a paragraph with
// both plain text and a link produces link annotations only for the linked part.
func TestConvertMixedTextAndLinkInParagraph(t *testing.T) {
	htmlStr := `<p>Visit <a href="https://example.com">our site</a> for more.</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	linkCount := 0
	for _, b := range plan.Blocks {
		linkCount += len(b.Links)
	}
	if linkCount == 0 {
		t.Error("expected at least one block with a Link annotation")
	}
}

// TestConvertMultipleLinksOnSameLine verifies that multiple distinct links
// on the same line each get their own annotation. Regression test for the
// per-line single-link limitation.
func TestConvertMultipleLinksOnSameLine(t *testing.T) {
	htmlStr := `<p><a href="https://a.com">A</a> and <a href="https://b.com">B</a> and <a href="https://c.com">C</a></p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	uris := make(map[string]bool)
	for _, b := range plan.Blocks {
		for _, link := range b.Links {
			uris[link.URI] = true
		}
	}
	for _, want := range []string{"https://a.com", "https://b.com", "https://c.com"} {
		if !uris[want] {
			t.Errorf("missing link annotation for %s (found: %v)", want, uris)
		}
	}
}

func TestConvertTableWithCSS(t *testing.T) {
	html := `<table style="border: 1px solid black">
		<tr><td style="padding: 8px; background-color: #eee">Styled cell</td></tr>
	</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestConvertDocumentWithTableAndLinks(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body>
  <h1>Invoice</h1>
  <p>See <a href="https://example.com/terms">terms</a>.</p>
  <table border="1">
    <thead><tr><th>Item</th><th>Qty</th><th>Price</th></tr></thead>
    <tbody>
      <tr><td>Widget A</td><td>10</td><td>$50</td></tr>
      <tr><td>Widget B</td><td>5</td><td>$30</td></tr>
    </tbody>
  </table>
</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 3 {
		t.Fatalf("expected at least 3 elements, got %d", len(elems))
	}
	for i, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 500, Height: 2000})
		if plan.Status == layout.LayoutNothing {
			t.Errorf("element %d: got LayoutNothing", i)
		}
	}
}

func TestParseBorderShorthand(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"1px solid black", 0.75},
		{"2px dashed red", 1.5},
		{"thin solid gray", 0.75},
		{"thick double blue", 3.75},
	}
	for _, tt := range tests {
		got := parseBorderShorthand(tt.input, 12)
		if got < tt.want-0.1 || got > tt.want+0.1 {
			t.Errorf("parseBorderShorthand(%q) = %f, want ~%f", tt.input, got, tt.want)
		}
	}
}

// --- Style block tests ---

func TestConvertStyleBlock(t *testing.T) {
	html := `<html><head><style>
		p { color: red }
		h1 { font-size: 36px }
	</style></head><body>
		<h1>Big</h1>
		<p>Red text</p>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements, got %d", len(elems))
	}
}

func TestConvertStyleBlockClass(t *testing.T) {
	html := `<html><head><style>
		.highlight { background-color: yellow }
	</style></head><body>
		<p class="highlight">Highlighted</p>
		<p>Normal</p>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements, got %d", len(elems))
	}
}

func TestConvertStyleBlockID(t *testing.T) {
	html := `<html><head><style>
		#title { font-size: 24px }
	</style></head><body>
		<p id="title">Title</p>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestConvertStyleBlockMultipleSelectors(t *testing.T) {
	html := `<html><head><style>
		h1, h2, h3 { color: navy }
	</style></head><body>
		<h1>One</h1>
		<h2>Two</h2>
		<h3>Three</h3>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elems))
	}
}

func TestConvertStyleBlockDescendant(t *testing.T) {
	html := `<html><head><style>
		div p { font-style: italic }
	</style></head><body>
		<div><p>Inside div - italic</p></div>
		<p>Outside div - normal</p>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements, got %d", len(elems))
	}
}

func TestStyleBlockInlineOverride(t *testing.T) {
	html := `<html><head><style>
		p { color: red }
	</style></head><body>
		<p style="color: blue">Blue wins</p>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

// --- Flexbox tests ---

func TestConvertFlexRow(t *testing.T) {
	html := `<div style="display: flex"><div>A</div><div>B</div><div>C</div></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestConvertFlexColumn(t *testing.T) {
	html := `<div style="display: flex; flex-direction: column">
		<p>Row 1</p><p>Row 2</p>
	</div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestConvertFlexWithGap(t *testing.T) {
	html := `<div style="display: flex; gap: 10px">
		<div>A</div><div>B</div>
	</div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

// TestGapShorthandWithCalc is a regression test for the same
// strings.Fields tokenization bug fixed for `flex:` (#236),
// `margin:`/`padding:` (#237), `font:` (#240), `border:` (#242),
// `background-size:` (#244), and `@page size` (#247) — applied here
// to the `gap`/`grid-gap` shorthand. Pre-fix `gap: calc(10px + 5px) 8px`
// became 4 tokens ["calc(10px", "+", "5px)", "8px"]; parts[0] = "calc(10px"
// failed parseBoxSide → row-gap stayed 0; parts[1] = "+" failed too →
// column-gap stayed 0; both gaps silently lost.
func TestGapShorthandWithCalc(t *testing.T) {
	tests := []struct {
		name              string
		val               string
		wantRowGap        float64 // in pt
		wantGap           float64 // flex compat: equals row-gap
		wantGridColumnGap float64
	}{
		{
			name: "single calc value applies to both axes",
			val:  "calc(10px + 5px)",
			// 15px = 11.25pt.
			wantRowGap: 11.25, wantGap: 11.25, wantGridColumnGap: 11.25,
		},
		{
			name: "two values: calc row-gap, plain column-gap",
			val:  "calc(10px + 5px) 8px",
			// row = 11.25pt; col = 6pt.
			wantRowGap: 11.25, wantGap: 11.25, wantGridColumnGap: 6,
		},
		{
			name: "two values: plain row-gap, calc column-gap",
			val:  "12px calc(2px * 4)",
			// row = 9pt; col = 8px = 6pt.
			wantRowGap: 9, wantGap: 9, wantGridColumnGap: 6,
		},
		{
			name: "two calcs",
			val:  "calc(10px + 5px) calc(20px / 2)",
			// row = 11.25pt; col = 10px = 7.5pt.
			wantRowGap: 11.25, wantGap: 11.25, wantGridColumnGap: 7.5,
		},
		{
			name: "calc with subtraction",
			val:  "calc(20px - 5px) 8px",
			// row = 15px = 11.25pt; col = 6pt.
			wantRowGap: 11.25, wantGap: 11.25, wantGridColumnGap: 6,
		},
		{
			name: "min() row, max() column",
			val:  "min(10px, 20px) max(8px, 16px)",
			// row = 10px = 7.5pt; col = 16px = 12pt.
			wantRowGap: 7.5, wantGap: 7.5, wantGridColumnGap: 12,
		},
		{
			name: "clamp() single value",
			val:  "clamp(8px, 16px, 24px)",
			// 16px = 12pt, applied to both axes.
			wantRowGap: 12, wantGap: 12, wantGridColumnGap: 12,
		},
		{
			name:       "tab separator",
			val:        "calc(10px + 5px)\t8px",
			wantRowGap: 11.25, wantGap: 11.25, wantGridColumnGap: 6,
		},
		{
			name:       "newline separator",
			val:        "calc(10px + 5px)\n8px",
			wantRowGap: 11.25, wantGap: 11.25, wantGridColumnGap: 6,
		},
		{
			name: "4+ tokens: parser uses only first two",
			val:  "10px 8px 5px 3px",
			// row = 7.5pt; col = 6pt; tokens 3 and 4 ignored.
			wantRowGap: 7.5, wantGap: 7.5, wantGridColumnGap: 6,
		},
		{
			name: "unbalanced calc paren: gaps stay 0",
			// splitTopLevelFields keeps the unbalanced calc + trailing
			// characters as one token; parseBoxSide → parseLength fails
			// → 0 for the single-value branch.
			val:        "calc(10px + 5px",
			wantRowGap: 0, wantGap: 0, wantGridColumnGap: 0,
		},
		{
			name: "empty value: gaps stay 0",
			// splitTopLevelFields returns no parts → neither branch
			// runs → gaps stay at zero default.
			val:        "",
			wantRowGap: 0, wantGap: 0, wantGridColumnGap: 0,
		},
		{
			name:       "whitespace-only value: gaps stay 0",
			val:        "   \t\n   ",
			wantRowGap: 0, wantGap: 0, wantGridColumnGap: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &converter{}
			style := &computedStyle{FontSize: 12}
			c.applyProperty("gap", tt.val, style)
			if math.Abs(style.RowGap-tt.wantRowGap) > 0.01 {
				t.Errorf("RowGap = %.4f, want %.4f", style.RowGap, tt.wantRowGap)
			}
			if math.Abs(style.Gap-tt.wantGap) > 0.01 {
				t.Errorf("Gap = %.4f, want %.4f", style.Gap, tt.wantGap)
			}
			if math.Abs(style.GridColumnGap-tt.wantGridColumnGap) > 0.01 {
				t.Errorf("GridColumnGap = %.4f, want %.4f",
					style.GridColumnGap, tt.wantGridColumnGap)
			}
		})
	}
}

// TestGridGapAliasMatchesGap pins that `grid-gap` (legacy alias for `gap`)
// routes through the same code path. Pre-fix this would silently drop
// calc values the same way `gap` did.
func TestGridGapAliasMatchesGap(t *testing.T) {
	c := &converter{}
	style := &computedStyle{FontSize: 12}
	c.applyProperty("grid-gap", "calc(10px + 5px) 8px", style)
	if math.Abs(style.RowGap-11.25) > 0.01 {
		t.Errorf("RowGap = %.4f, want 11.25", style.RowGap)
	}
	if math.Abs(style.GridColumnGap-6) > 0.01 {
		t.Errorf("GridColumnGap = %.4f, want 6", style.GridColumnGap)
	}
}

func TestConvertFlexJustifyCenter(t *testing.T) {
	html := `<div style="display: flex; justify-content: center">
		<p>Centered</p>
	</div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

// findLeafBlock returns the Y offset of the deepest descendant block
// (where deepest means: follows the first-child chain until a block has
// no children). Used by flex alignment tests to assert vertical position.
func findLeafBlock(blocks []layout.PlacedBlock) (layout.PlacedBlock, bool) {
	if len(blocks) == 0 {
		return layout.PlacedBlock{}, false
	}
	cur := blocks[0]
	for len(cur.Children) > 0 {
		cur = cur.Children[0]
	}
	return cur, true
}

func TestConvertFlexAlignItemsCenter(t *testing.T) {
	// align-items:center on a flex container with definite cross-size
	// must vertically center the child in the container.
	htmlStr := `<div style="display: flex; height: 60px; align-items: center; border: 1px dashed gray">` +
		`<div>ITEM</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	leaf, ok := findLeafBlock(plan.Blocks)
	if !ok {
		t.Fatal("no blocks produced")
	}
	// 60px container = 45pt. A small item should be centered ~ Y=15pt.
	// Top-aligned would be Y≈0; bottom-aligned would be Y≈30.
	if leaf.Y < 8 || leaf.Y > 22 {
		t.Errorf("centered child Y = %.2f, want ~15 (centered in 45pt container)", leaf.Y)
	}
}

func TestConvertFlexAlignItemsViaCustomProperty(t *testing.T) {
	// Regression for the CSS var() cascade-order bug: a stylesheet rule
	// like `align-items: var(--ai)` must resolve against an inline
	// `--ai: center` declared on the same element, even though inline
	// declarations come AFTER stylesheet rules in apply order. The
	// computeElementStyle two-pass apply makes custom properties
	// available to var() at apply-time regardless of source location.
	htmlStr := `<html><head><style>
.row { display: flex; height: 60px; align-items: var(--ai); border: 1px dashed gray; }
</style></head><body>
<div class="row" style="--ai: center"><div>ITEM</div></div>
</body></html>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	leaf, ok := findLeafBlock(plan.Blocks)
	if !ok {
		t.Fatal("no blocks produced")
	}
	// With the cascade bug, var(--ai) would resolve to nothing,
	// align-items would default to stretch, and the leaf would be
	// at Y=0 (stretched to fill). With the fix, it must center.
	if leaf.Y < 8 || leaf.Y > 22 {
		t.Errorf("centered child Y = %.2f, want ~15 (var(--ai) should resolve to center)", leaf.Y)
	}
}

func TestConvertFlexAlignItemsViaCustomPropertyAllValues(t *testing.T) {
	// Exercise all four align-items keyword values via var(), as the
	// flexbox.html stress test does. Each variant must resolve to the
	// expected vertical position.
	cases := []struct {
		name    string
		ai      string
		minY    float64
		maxY    float64
		comment string
	}{
		{"flex-start", "flex-start", -0.1, 4, "top of container"},
		{"center", "center", 8, 22, "vertical center of container"},
		{"flex-end", "flex-end", 26, 32, "bottom of container"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			htmlStr := `<html><head><style>` +
				`.row { display: flex; height: 60px; align-items: var(--ai); border: 1px dashed gray; }` +
				`</style></head><body>` +
				`<div class="row" style="--ai: ` + tc.ai + `"><div>ITEM</div></div>` +
				`</body></html>`
			elems, err := Convert(htmlStr, nil)
			if err != nil {
				t.Fatal(err)
			}
			plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
			leaf, ok := findLeafBlock(plan.Blocks)
			if !ok {
				t.Fatal("no blocks produced")
			}
			if leaf.Y < tc.minY || leaf.Y > tc.maxY {
				t.Errorf("var(--ai)=%s: leaf Y = %.2f, want %.1f..%.1f (%s)",
					tc.ai, leaf.Y, tc.minY, tc.maxY, tc.comment)
			}
		})
	}
}

// flexItemWidths returns the widths of a flex container's immediate
// child blocks, in visual (left-to-right) order. Each child in the
// test fixtures uses a distinct width-hint so its identity can be
// recovered from its laid-out width.
func flexItemWidths(flexBlock layout.PlacedBlock) []float64 {
	out := make([]float64, len(flexBlock.Children))
	for i, child := range flexBlock.Children {
		out[i] = child.Width
	}
	return out
}

func TestConvertFlexOrderProperty(t *testing.T) {
	// Children with explicit order values must be visually reordered.
	// DOM order: A(order:3, w:50px) B(order:1, w:100px) C(order:2, w:150px)
	// Expected visual order: B, C, A → widths in visual order: 100, 150, 50.
	// Each child uses width:Npx as its distinct identifier (consumed
	// as flex-basis with shrink:0 so it isn't resized).
	htmlStr := `<div style="display: flex">` +
		`<div style="order: 3; width: 50px; flex-shrink: 0">A</div>` +
		`<div style="order: 1; width: 100px; flex-shrink: 0">B</div>` +
		`<div style="order: 2; width: 150px; flex-shrink: 0">C</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 800})
	flexBlock := plan.Blocks[0]
	if len(flexBlock.Children) != 3 {
		t.Fatalf("expected 3 child blocks, got %d", len(flexBlock.Children))
	}
	widths := flexItemWidths(flexBlock)
	// Expected widths in visual order, in points (px*0.75): 75, 112.5, 37.5.
	wantWidths := []float64{75, 112.5, 37.5}
	for i, w := range wantWidths {
		if widths[i] < w-2 || widths[i] > w+2 {
			t.Errorf("visual slot %d: width = %.2f, want ~%.1f (full: %v)", i, widths[i], w, widths)
		}
	}
}

func TestConvertFlexOrderReordersFront(t *testing.T) {
	// A single child with order:-1 should jump to the front (leftmost).
	// DOM: A(w:50) B(w:100) C(order:-1, w:150) → visual: C, A, B.
	htmlStr := `<div style="display: flex">` +
		`<div style="width: 50px; flex-shrink: 0">A</div>` +
		`<div style="width: 100px; flex-shrink: 0">B</div>` +
		`<div style="order: -1; width: 150px; flex-shrink: 0">C</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 800})
	flexBlock := plan.Blocks[0]
	if len(flexBlock.Children) != 3 {
		t.Fatalf("expected 3 child blocks, got %d", len(flexBlock.Children))
	}
	widths := flexItemWidths(flexBlock)
	// Expected: C (112.5), A (37.5), B (75).
	wantWidths := []float64{112.5, 37.5, 75}
	for i, w := range wantWidths {
		if widths[i] < w-2 || widths[i] > w+2 {
			t.Errorf("visual slot %d: width = %.2f, want ~%.1f (full: %v)", i, widths[i], w, widths)
		}
	}
}

func TestConvertFlexOrderStableSortPreservesDOMOrderForTies(t *testing.T) {
	// Children with the same order value must remain in DOM order.
	// Mix tied and untied children so the test actually exercises
	// stability: a non-stable sort would permute the two order:1
	// children relative to each other.
	//
	// DOM: A(order:1, w:50) B(order:1, w:100) C(order:0, w:150)
	// Expected visual order by order value: C (order:0), A (order:1),
	// B (order:1) — A and B tied, A must come before B per DOM order.
	// In widths: 112.5 (C), 37.5 (A), 75 (B).
	htmlStr := `<div style="display: flex">` +
		`<div style="order: 1; width: 50px; flex-shrink: 0">A</div>` +
		`<div style="order: 1; width: 100px; flex-shrink: 0">B</div>` +
		`<div style="order: 0; width: 150px; flex-shrink: 0">C</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 800})
	flexBlock := plan.Blocks[0]
	widths := flexItemWidths(flexBlock)
	wantWidths := []float64{112.5, 37.5, 75}
	for i, w := range wantWidths {
		if widths[i] < w-2 || widths[i] > w+2 {
			t.Errorf("tie slot %d: width = %.2f, want ~%.1f (full: %v)", i, widths[i], w, widths)
		}
	}
}

func TestConvertFlexOrderColumnDirection(t *testing.T) {
	// The order property must also work in column direction. Children
	// are distinguished by their Y offset instead of X.
	//
	// DOM: A(order:2, h:30px) B(order:1, h:60px) C(order:0, h:90px)
	// Expected visual (top-to-bottom) order: C, B, A.
	htmlStr := `<div style="display: flex; flex-direction: column">` +
		`<div style="order: 2; height: 30px">A</div>` +
		`<div style="order: 1; height: 60px">B</div>` +
		`<div style="order: 0; height: 90px">C</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	flexBlock := plan.Blocks[0]
	if len(flexBlock.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(flexBlock.Children))
	}
	// Visual slot 0 (topmost) should be C (height 67.5pt = 90px).
	// Visual slot 1 should be B (height 45pt = 60px).
	// Visual slot 2 should be A (height 22.5pt = 30px).
	wantHeights := []float64{67.5, 45, 22.5}
	for i, h := range wantHeights {
		got := flexBlock.Children[i].Height
		if got < h-2 || got > h+2 {
			t.Errorf("visual slot %d: height = %.2f, want ~%.1f", i, got, h)
		}
	}
}

func TestConvertFlexOrderWithAlignSelf(t *testing.T) {
	// A flex child with both `order` and `align-self` set: the order
	// must reshuffle the children AND align-self must still be applied
	// (the `needsItem` branch wraps in a FlexItem).
	//
	// DOM:
	//   A (order:2, w:50, align-self unset)
	//   B (order:0, w:100, align-self:center)
	// Expected: B is first (order:0) and is vertically centered; A is
	// second (order:2) and is top-aligned (default for row).
	htmlStr := `<div style="display: flex; height: 60px">` +
		`<div style="order: 2; width: 50px; flex-shrink: 0">A</div>` +
		`<div style="order: 0; width: 100px; flex-shrink: 0; align-self: center">B</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	// Container is wrapped in Div because of height.
	div := plan.Blocks[0]
	if len(div.Children) == 0 {
		t.Fatal("no flex block")
	}
	flexBlock := div.Children[0]
	if len(flexBlock.Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(flexBlock.Children))
	}
	// Visual slot 0 is B (100px = 75pt), slot 1 is A (50px = 37.5pt).
	if w := flexBlock.Children[0].Width; w < 73 || w > 77 {
		t.Errorf("visual slot 0 width = %.2f, want ~75 (B with order:0)", w)
	}
	if w := flexBlock.Children[1].Width; w < 35.5 || w > 39.5 {
		t.Errorf("visual slot 1 width = %.2f, want ~37.5 (A with order:2)", w)
	}
	// align-self: center on B: its Y should be in the middle of 45pt.
	// B's rendered content is small, so Y should be > 0 if centered.
	if y := flexBlock.Children[0].Y; y < 5 {
		t.Errorf("B (align-self: center) Y = %.2f, want > 5 (centered, not top)", y)
	}
}

func TestCSSVarFallbackForUndeclaredProperty(t *testing.T) {
	// var() with a fallback when the referenced custom property is
	// undeclared should use the fallback. The fix must not break
	// this.
	htmlStr := `<div style="color: var(--undefined, #ff0000)">hello</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	// If the fallback resolved correctly, color becomes red and the
	// element has a Paragraph run with red color — we just verify
	// conversion succeeds and produces elements.
	if len(elems) == 0 {
		t.Fatal("expected at least one element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestCSSVarCustomPropertyForwardReference(t *testing.T) {
	// Regression for audit finding #2: a custom property that
	// references another custom property declared LATER in the
	// cascade must still resolve.
	//
	//   --b: var(--a);
	//   --a: 150px;
	//   width: var(--b);
	//
	// With eager resolution (the old behavior), --b would freeze to
	// the empty fallback because --a wasn't declared yet at apply
	// time. With lazy custom-prop resolution, var(--b) expands to
	// var(--a) which expands to 150px.
	htmlStr := `<div style="--b: var(--a); --a: 150px; width: var(--b); height: 20px; background: #ddd"></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 800})
	if len(plan.Blocks) == 0 {
		t.Fatal("no blocks produced")
	}
	box := plan.Blocks[0]
	// 150px = 112.5pt.
	if box.Width < 110 || box.Width > 115 {
		t.Errorf("box width = %.2f, want ~112.5 (var(--b) → var(--a) → 150px)", box.Width)
	}
}

func TestCSSVarCascadeOrderInlineOverridesParent(t *testing.T) {
	// General regression test for the CSS var() cascade-order fix:
	// a stylesheet rule that uses var() must read the custom property
	// from the inline style of the SAME element, even though inline
	// styles apply later in the cascade than stylesheet rules. This
	// is the same fix that makes align-items: var(--ai) work.
	//
	// We exercise it through a Width assertion: a stylesheet rule sets
	// width: var(--w), and the inline style sets --w. Without the
	// cascade fix, width is unset (var doesn't resolve at apply-time).
	// With the fix, width is honored.
	htmlStr := `<html><head><style>
.box { width: var(--w); height: 20px; background: #ddd; }
</style></head><body>
<div class="box" style="--w: 200px"></div>
</body></html>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 800})
	if len(plan.Blocks) == 0 {
		t.Fatal("no blocks produced")
	}
	box := plan.Blocks[0]
	// 200px = 150pt. Without the fix, var(--w) doesn't resolve and the
	// box takes the full container width.
	if box.Width < 148 || box.Width > 152 {
		t.Errorf("box width = %.2f, want ~150 (var(--w) should resolve to 200px = 150pt)", box.Width)
	}
}

// --- Nested list tests ---

func TestConvertNestedList(t *testing.T) {
	html := `<ul>
		<li>Item 1
			<ul>
				<li>Sub-item 1a</li>
				<li>Sub-item 1b</li>
			</ul>
		</li>
		<li>Item 2</li>
	</ul>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestConvertNestedOrderedInUnordered(t *testing.T) {
	html := `<ul>
		<li>Parent
			<ol>
				<li>Child 1</li>
				<li>Child 2</li>
			</ol>
		</li>
	</ul>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestConvertDeeplyNestedList(t *testing.T) {
	html := `<ul>
		<li>Level 1
			<ul>
				<li>Level 2
					<ul>
						<li>Level 3</li>
					</ul>
				</li>
			</ul>
		</li>
	</ul>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

// --- Div border tests ---

func TestConvertDivBorder(t *testing.T) {
	html := `<div style="border: 1px solid black"><p>Bordered</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestConvertDivDashedBorder(t *testing.T) {
	html := `<div style="border: 2px dashed red"><p>Dashed</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestConvertDivPartialBorder(t *testing.T) {
	html := `<div style="border-top: 1px solid blue; border-bottom: 1px solid blue"><p>Partial</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

// --- Code/Pre tests ---

func TestConvertInlineCode(t *testing.T) {
	html := `<p>Use <code>fmt.Println</code> to print</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestConvertPreBlock(t *testing.T) {
	html := "<pre>line 1\nline 2\nline 3</pre>"
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestConvertPreCode(t *testing.T) {
	html := "<pre><code>function foo() {\n  return 42;\n}</code></pre>"
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

// --- Image tests ---

func TestConvertImageMissing(t *testing.T) {
	html := `<img src="nonexistent.jpg" alt="Missing image">`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element (alt text fallback), got %d", len(elems))
	}
}

func TestConvertImageNoSrc(t *testing.T) {
	html := `<img alt="No source">`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element (alt text), got %d", len(elems))
	}
}

func TestConvertImageNoSrcNoAlt(t *testing.T) {
	html := `<img>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 0 {
		t.Fatalf("expected 0 elements, got %d", len(elems))
	}
}

// --- Font family tests ---

func TestParseFontFamily(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Courier", "courier"},
		{"'Courier New', monospace", "courier new"},
		{"monospace", "monospace"},
		{"Times New Roman", "times new roman"},
		{"serif", "serif"},
		{"Arial", "arial"},
		{"sans-serif", "sans-serif"},
		{"Helvetica", "helvetica"},
		{`"CustomFont"`, "customfont"},
		{`'Noto Sans', sans-serif`, "noto sans"},
		{`  "My Font"  `, "my font"},
	}
	for _, tt := range tests {
		got := parseFontFamily(tt.input)
		if got != tt.want {
			t.Errorf("parseFontFamily(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMapToStandardFamily(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"courier", "courier"},
		{"courier new", "courier"},
		{"monospace", "courier"},
		{"mono", "courier"},
		{"times new roman", "times"},
		{"times", "times"},
		{"serif", "times"},
		{"arial", "helvetica"},
		{"sans-serif", "helvetica"},
		{"helvetica", "helvetica"},
		{"noto sans", "helvetica"},
		{"customfont", "helvetica"},
	}
	for _, tt := range tests {
		got := mapToStandardFamily(tt.input)
		if got != tt.want {
			t.Errorf("mapToStandardFamily(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestConvertFontFamily(t *testing.T) {
	html := `<p style="font-family: 'Courier New', monospace">Mono text</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

// --- CSS parser tests ---

func TestCSSParseBasic(t *testing.T) {
	ss := &styleSheet{}
	ss.parseCSS("p { color: red; font-size: 14px }", "")
	if len(ss.rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(ss.rules))
	}
	if len(ss.rules[0].declarations) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(ss.rules[0].declarations))
	}
}

func TestCSSParseMultipleRules(t *testing.T) {
	ss := &styleSheet{}
	ss.parseCSS("h1 { color: blue } p { margin: 10px }", "")
	if len(ss.rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(ss.rules))
	}
}

func TestCSSParseComments(t *testing.T) {
	ss := &styleSheet{}
	ss.parseCSS("/* comment */ p { color: red } /* another */", "")
	if len(ss.rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(ss.rules))
	}
}

func TestCSSSelectorMatch(t *testing.T) {
	sel := parseSelector("p")
	if len(sel.parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(sel.parts))
	}
	if sel.parts[0].tag != "p" {
		t.Errorf("expected tag 'p', got %q", sel.parts[0].tag)
	}
}

func TestCSSSelectorClass(t *testing.T) {
	sel := parseSelector(".highlight")
	if sel.parts[0].class != "highlight" {
		t.Errorf("expected class 'highlight', got %q", sel.parts[0].class)
	}
}

// TestCSSClassCaseInsensitive verifies that CSS class selectors match
// HTML class attributes case-insensitively. Regression test for #28.
func TestCSSClassCaseInsensitive(t *testing.T) {
	htmlStr := `<html><head><style>
.myClass { color: red; }
.UPPER { font-weight: bold; }
</style></head><body>
<p class="myClass">Should be red</p>
<p class="upper">Should be bold</p>
<p class="MyClass">Mixed case should also match</p>
</body></html>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	// All three paragraphs should produce elements (if the class didn't
	// match, the text would still render but with default styling — we
	// can't easily check color here, so just verify no crash and the
	// converter produces elements).
	if len(elems) < 3 {
		t.Errorf("expected at least 3 elements, got %d", len(elems))
	}
}

// TestCSSClassCaseInsensitiveMatching directly tests containsClass.
func TestCSSClassCaseInsensitiveMatching(t *testing.T) {
	tests := []struct {
		classes []string
		name    string
		want    bool
	}{
		{[]string{"myClass"}, "myclass", true},
		{[]string{"myClass"}, "MYCLASS", true},
		{[]string{"myClass"}, "myClass", true},
		{[]string{"foo", "Bar"}, "bar", true},
		{[]string{"foo", "Bar"}, "BAR", true},
		{[]string{"foo"}, "baz", false},
		{nil, "foo", false},
	}
	for _, tt := range tests {
		got := containsClass(tt.classes, tt.name)
		if got != tt.want {
			t.Errorf("containsClass(%v, %q) = %v, want %v",
				tt.classes, tt.name, got, tt.want)
		}
	}
}

func TestCSSSelectorID(t *testing.T) {
	sel := parseSelector("#main")
	if sel.parts[0].id != "main" {
		t.Errorf("expected id 'main', got %q", sel.parts[0].id)
	}
}

func TestCSSSelectorDescendant(t *testing.T) {
	sel := parseSelector("div p")
	if len(sel.parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(sel.parts))
	}
	if sel.parts[0].tag != "div" || sel.parts[1].tag != "p" {
		t.Error("wrong descendant parts")
	}
}

func TestParseBorderFull(t *testing.T) {
	w, s, _ := parseBorderFull("2px dashed red", 12)
	if w < 1.4 || w > 1.6 {
		t.Errorf("width: got %f, want ~1.5", w)
	}
	if s != "dashed" {
		t.Errorf("style: got %q, want 'dashed'", s)
	}
}

// TestParseBorderFullWithCalc is a regression test for the same
// strings.Fields tokenization bug fixed for `flex:` (#236),
// `margin:`/`padding:` (#237), and `font:` (#240) — applied here to
// `border:`. The parser iterates each whitespace-delimited token and
// classifies as keyword, length, or color. Pre-fix, calc/min/max/clamp
// values for the width and rgb()/rgba()/hsl() values for the color
// (when written with internal whitespace) were shredded into fragments
// that matched none of those classifiers, so the width fell back to
// the default thin (0.75pt) and the color stayed black.
func TestParseBorderFullWithCalc(t *testing.T) {
	red := layout.Color{R: 1, G: 0, B: 0}
	tests := []struct {
		name      string
		input     string
		wantWidth float64
		wantStyle string
		wantColor layout.Color
	}{
		{
			name:  "calc width",
			input: "calc(1px + 1px) solid red",
			// 2px = 1.5pt.
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:  "min() width",
			input: "min(2px, 4px) solid red",
			// min picks 2px = 1.5pt.
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:  "max() width",
			input: "max(1px, 3px) dotted red",
			// max picks 3px = 2.25pt.
			wantWidth: 2.25, wantStyle: "dotted", wantColor: red,
		},
		{
			name:  "clamp() width",
			input: "clamp(1px, 2px, 4px) dashed red",
			// clamp middle = 2px = 1.5pt.
			wantWidth: 1.5, wantStyle: "dashed", wantColor: red,
		},
		{
			name:  "calc with subtraction",
			input: "calc(4px - 2px) solid red",
			// 2px = 1.5pt.
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:  "calc with multiplication",
			input: "calc(2px * 2) solid red",
			// 4px = 3pt.
			wantWidth: 3, wantStyle: "solid", wantColor: red,
		},
		{
			name:  "calc with division",
			input: "calc(8px / 2) solid red",
			// 4px = 3pt.
			wantWidth: 3, wantStyle: "solid", wantColor: red,
		},
		{
			name:  "rgb() color with internal whitespace",
			input: "1px solid rgb(255, 0, 0)",
			// Pre-fix the rgb was shredded to "rgb(255," / "0," / "0)"
			// and color stayed at default black.
			wantWidth: 0.75, wantStyle: "solid", wantColor: red,
		},
		{
			name:  "rgba() color with internal whitespace",
			input: "2px dashed rgba(255, 0, 0, 0.5)",
			// Color alpha is not tracked on layout.Color; assert RGB only.
			wantWidth: 1.5, wantStyle: "dashed", wantColor: red,
		},
		{
			name:  "hsl() color with internal whitespace",
			input: "1px solid hsl(0, 100%, 50%)",
			// hsl(0, 100%, 50%) is pure red. Pre-fix the hsl was
			// shredded the same way as rgb and color stayed black.
			wantWidth: 0.75, wantStyle: "solid", wantColor: red,
		},
		{
			name:      "calc width with rgb color (both compound)",
			input:     "calc(1px + 1px) solid rgb(255, 0, 0)",
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:      "hex color",
			input:     "1px solid #ff0000",
			wantWidth: 0.75, wantStyle: "solid", wantColor: red,
		},
		{
			name:      "tabs and newlines as separators",
			input:     "calc(1px + 1px)\tsolid\nred",
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:  "thick keyword still works",
			input: "thick solid red",
			// "thick" → 3.75pt.
			wantWidth: 3.75, wantStyle: "solid", wantColor: red,
		},
		{
			name:  "reverse order: color style width",
			input: "red solid 2px",
			// Parser is order-agnostic — each token is classified
			// independently, so reversed order resolves identically.
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:  "style only (no width or color)",
			input: "dashed",
			// Width defaults to thin (0.75pt), color to black.
			wantWidth: 0.75, wantStyle: "dashed", wantColor: layout.ColorBlack,
		},
		{
			name:  "lone width (no style or color)",
			input: "2px",
			// Style defaults to "solid", color to black.
			wantWidth: 1.5, wantStyle: "solid", wantColor: layout.ColorBlack,
		},
		{
			name:  "5+ tokens — extras are silently classified",
			input: "1px solid red foo bar",
			// "foo"/"bar" don't classify as keyword/length/color, so
			// they're ignored. Width/style/color come from the first
			// three valid tokens.
			wantWidth: 0.75, wantStyle: "solid", wantColor: red,
		},
		{
			name:  "none style zeros the width even if calc set",
			input: "calc(2px + 2px) none red",
			// "none" forces width to 0.
			wantWidth: 0, wantStyle: "none", wantColor: red,
		},
		{
			name: "empty input returns defaults",
			// parseBorderFull short-circuits on empty input, returning
			// width=0, style="none" (NOT the "solid" default used for
			// non-empty inputs), color=black.
			input:     "",
			wantWidth: 0, wantStyle: "none", wantColor: layout.ColorBlack,
		},
		{
			name: "unbalanced calc paren swallows everything to defaults",
			// splitTopLevelFields keeps the unbalanced calc + trailing
			// characters as one token (depth never returns to 0). That
			// single token doesn't classify as keyword, length, or color
			// — so width, style, and color all keep their non-empty-input
			// defaults (0.75pt thin, "solid", black). Note "solid" here
			// is the default style, NOT a recognized second token.
			// Documents the no-crash invariant on malformed input.
			input:     "calc(1px + 1px solid red",
			wantWidth: 0.75, wantStyle: "solid", wantColor: layout.ColorBlack,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, s, c := parseBorderFull(tt.input, 12)
			if math.Abs(w-tt.wantWidth) > 0.01 {
				t.Errorf("width = %.4f, want %.4f", w, tt.wantWidth)
			}
			if s != tt.wantStyle {
				t.Errorf("style = %q, want %q", s, tt.wantStyle)
			}
			if math.Abs(c.R-tt.wantColor.R) > 0.01 ||
				math.Abs(c.G-tt.wantColor.G) > 0.01 ||
				math.Abs(c.B-tt.wantColor.B) > 0.01 {
				t.Errorf("color RGB = %+v, want %+v", c, tt.wantColor)
			}
		})
	}
}

// --- Full document test ---

func TestConvertFullDocument(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Test</title></head>
<body>
  <h1>Hello World</h1>
  <p>This is a <strong>test</strong> document.</p>
  <ul>
    <li>Item one</li>
    <li>Item two</li>
  </ul>
</body>
</html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 3 {
		t.Fatalf("expected at least 3 elements, got %d", len(elems))
	}

	for i, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 500, Height: 2000})
		if plan.Status == layout.LayoutNothing {
			t.Errorf("element %d: got LayoutNothing", i)
		}
	}
}

// --- Blockquote ---

func TestConvertBlockquote(t *testing.T) {
	html := `<blockquote><p>To be or not to be.</p></blockquote>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element (div), got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestConvertBlockquoteWithCSS(t *testing.T) {
	html := `<style>blockquote { border-left: 4px solid red; }</style>
<blockquote><p>Styled quote.</p></blockquote>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- Page Break ---

func TestConvertPageBreakBefore(t *testing.T) {
	html := `<p>First</p><div style="page-break-before: always"><p>Second</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range elems {
		if _, ok := e.(*layout.AreaBreak); ok {
			found = true
		}
	}
	if !found {
		t.Error("expected an AreaBreak element for page-break-before: always")
	}
}

func TestConvertPageBreakAfter(t *testing.T) {
	html := `<div style="page-break-after: always"><p>Content</p></div><p>Next page</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range elems {
		if _, ok := e.(*layout.AreaBreak); ok {
			found = true
		}
	}
	if !found {
		t.Error("expected an AreaBreak element for page-break-after: always")
	}
}

// TestPageBreakInsideBodyWithWidth verifies that page-break-after works
// when <body> has width: 100%, which causes convertBlock to wrap children
// in a Div. AreaBreak elements must be hoisted out of the Div so the
// renderer can see them. Regression test for #21.
func TestPageBreakInsideBodyWithWidth(t *testing.T) {
	htmlStr := `<!DOCTYPE html><head><style>
.pagebreak { page-break-after: always; }
html, body { width: 100%; margin: 0; padding: 0; }
</style></head><body>
<div class="pagebreak"><p>Page 1</p></div>
<div class="pagebreak"><p>Page 2</p></div>
<div class="pagebreak"><p>Page 3</p></div>
</body>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	breakCount := 0
	for _, e := range elems {
		if _, ok := e.(*layout.AreaBreak); ok {
			breakCount++
		}
	}
	if breakCount < 3 {
		t.Errorf("expected at least 3 AreaBreaks, got %d (elements: %d)", breakCount, len(elems))
		for i, e := range elems {
			t.Logf("  [%d] %T", i, e)
		}
	}
}

// TestPageBreakInsideDivWrapper verifies that page-break-after works
// even when the parent has box-model properties that trigger a Div wrapper.
func TestPageBreakInsideDivWrapper(t *testing.T) {
	htmlStr := `<div style="padding: 10px; background-color: #eee">
<div style="page-break-after: always"><p>Section 1</p></div>
<div style="page-break-after: always"><p>Section 2</p></div>
</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	breakCount := 0
	for _, e := range elems {
		if _, ok := e.(*layout.AreaBreak); ok {
			breakCount++
		}
	}
	if breakCount < 2 {
		t.Errorf("expected at least 2 AreaBreaks hoisted from Div, got %d", breakCount)
		for i, e := range elems {
			t.Logf("  [%d] %T", i, e)
		}
	}
}

func TestPageBreakInsideAvoid(t *testing.T) {
	// A div with page-break-inside: avoid and box-model properties
	// (to ensure a Div is produced) should have KeepTogether set.
	htmlStr := `<div style="page-break-inside: avoid; padding: 5px; background: #eee">
		<p>Keep me together</p>
		<p>Second paragraph</p>
	</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range elems {
		if div, ok := e.(*layout.Div); ok {
			if div.KeepTogether() {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected Div with KeepTogether=true for page-break-inside: avoid")
		for i, e := range elems {
			t.Logf("  [%d] %T", i, e)
		}
	}
}

func TestPageBreakInsideAvoidRenderer(t *testing.T) {
	// A tall div with page-break-inside: avoid on a small page should
	// move to the next page instead of splitting.
	// Use paragraphs with enough text to take space, then a Div.
	p1 := layout.NewParagraph("First paragraph with enough content to take up space on the page. "+
		"This should use a significant portion of the available area.", font.Helvetica, 12)

	div2 := layout.NewDiv()
	div2.SetBackground(layout.RGB(1, 0, 0))
	div2.SetPadding(10)
	div2.SetKeepTogether(true)
	// Add several child paragraphs to make the div tall enough to split.
	for i := range 10 {
		_ = i
		div2.Add(layout.NewParagraph("Line of text in the keep-together div. "+
			"This needs to be long enough that the div exceeds remaining space.", font.Helvetica, 12))
	}

	margins := layout.Margins{Top: 20, Bottom: 20, Left: 20, Right: 20}
	r := layout.NewRenderer(612, 200, margins) // very short page (160pt content area)
	r.Add(p1)
	r.Add(div2)
	pages := r.Render()

	// p1 takes some space, then div2 doesn't fit and has KeepTogether.
	// It should move to page 2 rather than splitting.
	if len(pages) < 2 {
		t.Fatalf("expected at least 2 pages, got %d", len(pages))
	}
}

// --- aspect-ratio ---

func TestAspectRatio(t *testing.T) {
	htmlStr := `<div style="aspect-ratio: 16 / 9; width: 320px; background: #000"></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	// 320px = 240pt, 240 / (16/9) = 135pt
	if plan.Consumed < 130 || plan.Consumed > 140 {
		t.Errorf("expected ~135pt for 16:9 on 320px, got %f", plan.Consumed)
	}
}

func TestAspectRatioSingleNumber(t *testing.T) {
	htmlStr := `<div style="aspect-ratio: 2; width: 200px; background: #eee"></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	// 200px = 150pt, 150 / 2 = 75pt
	if plan.Consumed < 70 || plan.Consumed > 80 {
		t.Errorf("expected ~75pt for ratio 2 on 200px, got %f", plan.Consumed)
	}
}

func TestAspectRatioAuto(t *testing.T) {
	// "auto" and "none" should produce no aspect constraint.
	for _, val := range []string{"auto", "none", ""} {
		htmlStr := `<div style="aspect-ratio: ` + val + `; width: 200px; padding: 5px; background: #eee"><p>Hi</p></div>`
		elems, err := Convert(htmlStr, nil)
		if err != nil {
			t.Fatalf("aspect-ratio: %q: %v", val, err)
		}
		if len(elems) == 0 {
			continue // no visual wrapper for empty/no-constraint
		}
		plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
		// Should be content-based height, not width/ratio.
		if plan.Consumed > 100 {
			t.Errorf("aspect-ratio: %q should be content-based, got %f", val, plan.Consumed)
		}
	}
}

func TestAspectRatioAutoCompound(t *testing.T) {
	// "auto 16 / 9" compound form — the ratio part should apply.
	htmlStr := `<div style="aspect-ratio: auto 16 / 9; width: 320px; background: #000"></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	// 320px = 240pt, 240 / (16/9) = 135pt
	if plan.Consumed < 130 || plan.Consumed > 140 {
		t.Errorf("expected ~135pt for auto 16/9, got %f", plan.Consumed)
	}
}

func TestAspectRatioNegativeIgnored(t *testing.T) {
	// Negative ratio should be ignored (treated as auto).
	htmlStr := `<div style="aspect-ratio: -2 / 1; width: 200px; padding: 5px; background: #eee"><p>Hi</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	// Negative ratio → no constraint, content-based height.
	if plan.Consumed > 100 {
		t.Errorf("negative aspect-ratio should be ignored, got %f", plan.Consumed)
	}
}

func TestParseAspectRatioValues(t *testing.T) {
	tests := []struct {
		input string
		want  float64
	}{
		{"16 / 9", 16.0 / 9.0},
		{"16/9", 16.0 / 9.0},
		{"4 / 3", 4.0 / 3.0},
		{"1", 1},
		{"2", 2},
		{"1.778", 1.778},
		{"auto", 0},
		{"none", 0},
		{"", 0},
		{"auto 16 / 9", 16.0 / 9.0},
		{"-2 / 1", 0}, // negative rejected
		{"2 / -1", 0}, // negative rejected
		{"0 / 1", 0},  // zero rejected
		{"1 / 0", 0},  // zero divisor rejected
		{"garbage", 0},
	}
	for _, tt := range tests {
		got := parseAspectRatio(tt.input)
		diff := got - tt.want
		if diff > 0.01 || diff < -0.01 {
			t.Errorf("parseAspectRatio(%q) = %f, want %f", tt.input, got, tt.want)
		}
	}
}

// --- Div wrapper regression tests ---

func TestDivWrapperSkippedForPlainDiv(t *testing.T) {
	// A plain <div> with no box-model properties should NOT produce a
	// layout.Div wrapper — children should be returned directly.
	htmlStr := `<div><p>First</p><p>Second</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should produce 2 Paragraphs, not 1 Div containing them.
	for _, e := range elems {
		if _, ok := e.(*layout.Div); ok {
			t.Error("plain <div> without box-model should not create a layout.Div wrapper")
		}
	}
}

func TestDivWrapperCreatedForPadding(t *testing.T) {
	htmlStr := `<div style="padding: 10px"><p>Padded</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range elems {
		if _, ok := e.(*layout.Div); ok {
			found = true
		}
	}
	if !found {
		t.Error("div with padding should create a Div wrapper")
	}
}

func TestDivWrapperCreatedForWidth(t *testing.T) {
	htmlStr := `<div style="width: 200px"><p>Fixed width</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range elems {
		if _, ok := e.(*layout.Div); ok {
			found = true
		}
	}
	if !found {
		t.Error("div with width should create a Div wrapper")
	}
}

func TestDivWrapperCreatedForBackground(t *testing.T) {
	htmlStr := `<div style="background-color: #eee"><p>With bg</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range elems {
		if _, ok := e.(*layout.Div); ok {
			found = true
		}
	}
	if !found {
		t.Error("div with background should create a Div wrapper")
	}
}

func TestDivWrapperCreatedForAspectRatio(t *testing.T) {
	htmlStr := `<div style="aspect-ratio: 2; background: #eee"><p>Ratio</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range elems {
		if _, ok := e.(*layout.Div); ok {
			found = true
		}
	}
	if !found {
		t.Error("div with aspect-ratio should create a Div wrapper")
	}
}

// --- !important ---

func TestCSSImportant(t *testing.T) {
	html := `<style>
p { color: red; }
p { color: blue !important; }
</style>
<p>Important test</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSImportantOverridesHigherSpecificity(t *testing.T) {
	html := `<style>
#main { color: red; }
p { color: blue !important; }
</style>
<p id="main">Important wins</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- Pseudo-classes ---

func TestCSSFirstChild(t *testing.T) {
	html := `<style>
li:first-child { font-weight: bold; }
</style>
<ul><li>First</li><li>Second</li></ul>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSNthChild(t *testing.T) {
	html := `<style>
tr:nth-child(2) { background-color: #eee; }
</style>
<table><tr><td>Row 1</td></tr><tr><td>Row 2</td></tr><tr><td>Row 3</td></tr></table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSNthChildOddEven(t *testing.T) {
	html := `<style>
p:nth-child(odd) { color: red; }
p:nth-child(even) { color: blue; }
</style>
<div><p>One</p><p>Two</p><p>Three</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSColumnRule(t *testing.T) {
	src := `<div style="column-count: 3; column-gap: 20px; column-rule: 1px solid gray;">
		<p>First column content.</p>
		<p>Second column content.</p>
		<p>Third column content.</p>
	</div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 800})
	if plan.Status != layout.LayoutFull {
		t.Error("expected LayoutFull")
	}
}

func TestParseColumnRuleShorthand(t *testing.T) {
	tests := []struct {
		input string
		width float64 // in points (1px = 0.75pt)
		style string
	}{
		{"1px solid gray", 0.75, "solid"},
		{"2px dashed red", 1.5, "dashed"},
		{"dotted", 0, "dotted"},
		{"3px", 2.25, "solid"},
		{"none", 0, "none"},
	}
	for _, tt := range tests {
		w, s, _ := parseColumnRule(tt.input, 12)
		if abs(w-tt.width) > 0.01 {
			t.Errorf("parseColumnRule(%q): width=%f, want %f", tt.input, w, tt.width)
		}
		if s != tt.style {
			t.Errorf("parseColumnRule(%q): style=%q, want %q", tt.input, s, tt.style)
		}
	}
}

// TestParseColumnRuleWithCalc is a regression test for the same
// strings.Fields tokenization bug fixed for `flex:` (#236),
// `margin:`/`padding:` (#237), `font:` (#240), `border:` (#242),
// `background-size:` (#244), `@page size` (#247), `gap:` (#249),
// `border-radius:` (#252), `box-shadow:` (#254),
// `transform-origin:` (#257), `border-spacing:` (#258), and
// `columns:` (#259) — applied here to the `column-rule` shorthand.
// Pre-fix `column-rule: calc(1px + 1px) solid red` became 5 tokens
// ["calc(1px", "+", "1px)", "solid", "red"]; the calc fragments all
// failed both parseColor and parseLength → width stayed 0 (default).
//
// Same hazard for rgb()/rgba()/hsl() colors with internal whitespace.
func TestParseColumnRuleWithCalc(t *testing.T) {
	red := layout.Color{R: 1, G: 0, B: 0}
	tests := []struct {
		name      string
		input     string
		wantWidth float64
		wantStyle string
		wantColor layout.Color
	}{
		{
			name:  "calc width",
			input: "calc(1px + 1px) solid red",
			// 2px = 1.5pt.
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:      "min() width",
			input:     "min(2px, 4px) solid red",
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:  "max() width",
			input: "max(1px, 3px) dotted red",
			// 3px = 2.25pt.
			wantWidth: 2.25, wantStyle: "dotted", wantColor: red,
		},
		{
			name:  "clamp() width",
			input: "clamp(1px, 2px, 4px) dashed red",
			// clamp middle = 2px = 1.5pt.
			wantWidth: 1.5, wantStyle: "dashed", wantColor: red,
		},
		{
			name:      "calc with subtraction",
			input:     "calc(4px - 2px) solid red",
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:      "calc with multiplication",
			input:     "calc(1px * 2) solid red",
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:      "calc with division",
			input:     "calc(4px / 2) solid red",
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:      "rgb() color with internal whitespace",
			input:     "1px solid rgb(255, 0, 0)",
			wantWidth: 0.75, wantStyle: "solid", wantColor: red,
		},
		{
			name:  "rgba() color with internal whitespace",
			input: "2px dashed rgba(255, 0, 0, 0.5)",
			// Color alpha not tracked on layout.Color; assert RGB only.
			wantWidth: 1.5, wantStyle: "dashed", wantColor: red,
		},
		{
			name:  "hsl() color with internal whitespace",
			input: "1px solid hsl(0, 100%, 50%)",
			// hsl(0, 100%, 50%) is pure red.
			wantWidth: 0.75, wantStyle: "solid", wantColor: red,
		},
		{
			name:      "calc width + rgb color (both compound)",
			input:     "calc(1px + 1px) solid rgb(255, 0, 0)",
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:      "calc width + hex color",
			input:     "calc(1px + 1px) solid #ff0000",
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:      "tab and newline separators",
			input:     "calc(1px + 1px)\tsolid\nred",
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name:      "reverse order: color style width (parser is order-agnostic)",
			input:     "red solid calc(1px + 1px)",
			wantWidth: 1.5, wantStyle: "solid", wantColor: red,
		},
		{
			name: "none style zeros the width even if calc set",
			// Per CSS Multi-column Layout L1, column-rule-style: none
			// computes column-rule-width to 0 (same rule as the border
			// shorthand). This was inconsistent pre-fix (parseColumnRule
			// returned the parsed width regardless); aligned with
			// parseBorderFull in this PR.
			input:     "calc(2px + 2px) none red",
			wantWidth: 0, wantStyle: "none", wantColor: red,
		},
		{
			name:      "hidden style also zeros the width",
			input:     "2px hidden red",
			wantWidth: 0, wantStyle: "hidden", wantColor: red,
		},
		{
			name:      "empty input returns defaults",
			input:     "",
			wantWidth: 0, wantStyle: "solid", wantColor: layout.ColorBlack,
		},
		{
			name:      "whitespace-only input returns defaults",
			input:     "   \t\n   ",
			wantWidth: 0, wantStyle: "solid", wantColor: layout.ColorBlack,
		},
		{
			name: "unbalanced calc paren: stays at defaults",
			// splitTopLevelFields keeps the unbalanced calc + trailing
			// chars as one token; not a keyword, parseColor and
			// parseLength both fail → no width/style/color set →
			// defaults (width=0, style="solid", color=black) preserved.
			input:     "calc(1px + 1px solid red",
			wantWidth: 0, wantStyle: "solid", wantColor: layout.ColorBlack,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, s, c := parseColumnRule(tt.input, 12)
			if math.Abs(w-tt.wantWidth) > 0.01 {
				t.Errorf("width = %.4f, want %.4f", w, tt.wantWidth)
			}
			if s != tt.wantStyle {
				t.Errorf("style = %q, want %q", s, tt.wantStyle)
			}
			if math.Abs(c.R-tt.wantColor.R) > 0.01 ||
				math.Abs(c.G-tt.wantColor.G) > 0.01 ||
				math.Abs(c.B-tt.wantColor.B) > 0.01 {
				t.Errorf("color RGB = %+v, want %+v", c, tt.wantColor)
			}
		})
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestCSSNthOfType(t *testing.T) {
	src := `<style>
tr:nth-of-type(even) { background-color: #f9f9f9; }
tr:nth-of-type(odd) { background-color: #fff; }
</style>
<table>
<tr><td>Row 1</td></tr>
<tr><td>Row 2</td></tr>
<tr><td>Row 3</td></tr>
<tr><td>Row 4</td></tr>
</table>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSFirstLastOfType(t *testing.T) {
	src := `<style>
p:first-of-type { font-weight: bold; }
p:last-of-type { font-style: italic; }
</style>
<div>
<h2>Title</h2>
<p>First paragraph</p>
<p>Second paragraph</p>
<p>Last paragraph</p>
</div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSNthAnPlusB(t *testing.T) {
	// Test An+B expressions: 3n+1 matches positions 1, 4, 7, ...
	src := `<style>
li:nth-of-type(3n+1) { color: red; }
</style>
<ul><li>1</li><li>2</li><li>3</li><li>4</li><li>5</li><li>6</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- :empty pseudo-class (CSS Selectors Level 3 semantics) ---

// findElements returns every element node in doc with the given tag name.
func findElements(doc *htmlparse.Node, tag string) []*htmlparse.Node {
	var out []*htmlparse.Node
	var walk func(n *htmlparse.Node)
	walk = func(n *htmlparse.Node) {
		if n.Type == htmlparse.ElementNode && n.Data == tag {
			out = append(out, n)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return out
}

// emptyMatchCases drives :empty unit tests. CSS Selectors Level 3 says
// :empty matches an element with no children at all (text, including
// whitespace, counts as content; comments do NOT).
func TestCSSEmptyPseudoMatchesElement(t *testing.T) {
	tests := []struct {
		name string
		html string
		want bool
	}{
		{"truly empty", `<div><p></p></div>`, true},
		{"text content", `<div><p>x</p></div>`, false},
		{"whitespace text", `<div><p>   </p></div>`, false},
		{"element child", `<div><p><span></span></p></div>`, false},
		{"comment only matches per CSS3", `<div><p><!--c--></p></div>`, true},
		{"void element child", `<div><p><br></p></div>`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := htmlparse.Parse(strings.NewReader(tt.html))
			if err != nil {
				t.Fatal(err)
			}
			paras := findElements(doc, "p")
			if len(paras) != 1 {
				t.Fatalf("expected 1 <p>, got %d", len(paras))
			}
			got := pseudoMatches("empty", paras[0])
			if got != tt.want {
				t.Errorf("pseudoMatches(empty) = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCSSEmptyPseudoEndToEnd(t *testing.T) {
	// Smoke test: stylesheet referencing :empty must not break Convert.
	src := `<style>
p:empty { color: red; }
</style>
<div><p></p><p>not empty</p></div>`
	if _, err := Convert(src, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCSSEmptyWithCombinator(t *testing.T) {
	// :empty composed with an adjacent-sibling combinator must reach
	// the right cells. The second <td> (which follows an empty <td>)
	// is the only one that should match `td:empty + td`.
	doc, err := htmlparse.Parse(strings.NewReader(
		`<table><tr><td></td><td>second</td><td>third</td></tr></table>`))
	if err != nil {
		t.Fatal(err)
	}
	tds := findElements(doc, "td")
	if len(tds) != 3 {
		t.Fatalf("expected 3 <td>, got %d", len(tds))
	}
	sel := parseSelector("td:empty + td")
	want := []bool{false, true, false}
	for i, td := range tds {
		got := selectorMatches(sel, td)
		if got != want[i] {
			t.Errorf("td[%d]: selectorMatches(td:empty + td) = %v, want %v", i, got, want[i])
		}
	}
}

// --- ::placeholder pseudo-element ---

// colorApprox returns true if two colors are equal within 1/255 in each channel.
// CSS hex parsing produces 0.50196..., not 0.5, so layout.ColorGreen != #008000
// even though they render identically.
func colorApprox(a, b layout.Color) bool {
	return abs(a.R-b.R) < 0.005 && abs(a.G-b.G) < 0.005 && abs(a.B-b.B) < 0.005
}

// findFirstParagraphRun walks elems looking for a Div whose first child
// is a *layout.Paragraph and returns that paragraph's first run.
func findFirstParagraphRun(t *testing.T, elems []layout.Element) layout.TextRun {
	t.Helper()
	for _, e := range elems {
		div, ok := e.(*layout.Div)
		if !ok {
			continue
		}
		for _, c := range div.Children() {
			if p, ok := c.(*layout.Paragraph); ok {
				runs := p.Runs()
				if len(runs) == 0 {
					t.Fatal("paragraph has no runs")
				}
				return runs[0]
			}
		}
	}
	t.Fatal("no Div containing a Paragraph found")
	return layout.TextRun{}
}

func TestCSSPlaceholderColorAppliedInput(t *testing.T) {
	src := `<style>input::placeholder { color: #ff0000; }</style>
<input type="text" placeholder="Search...">`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	run := findFirstParagraphRun(t, elems)
	if run.Color != layout.ColorRed {
		t.Errorf("placeholder color = %+v, want ColorRed (%+v)", run.Color, layout.ColorRed)
	}
	if run.Text != "Search..." {
		t.Errorf("placeholder text = %q, want %q", run.Text, "Search...")
	}
}

func TestCSSPlaceholderColorAppliedTextarea(t *testing.T) {
	src := `<style>textarea::placeholder { color: #008000; }</style>
<textarea placeholder="Enter notes..."></textarea>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	run := findFirstParagraphRun(t, elems)
	if !colorApprox(run.Color, layout.ColorGreen) {
		t.Errorf("placeholder color = %+v, want ColorGreen (%+v)", run.Color, layout.ColorGreen)
	}
}

func TestCSSPlaceholderItalicResolvesItalicFont(t *testing.T) {
	src := `<style>input::placeholder { font-style: italic; }</style>
<input type="text" placeholder="Search...">`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	run := findFirstParagraphRun(t, elems)
	if run.Font != font.HelveticaOblique {
		t.Errorf("placeholder font = %v, want HelveticaOblique", run.Font)
	}
}

func TestCSSPlaceholderNumericFontWeightResolvesBold(t *testing.T) {
	// font-weight: 700 must be normalized to "bold" via parseFontWeight,
	// not stored verbatim (which would silently fail to bold the font).
	src := `<style>input::placeholder { font-weight: 700; }</style>
<input type="text" placeholder="Search...">`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	run := findFirstParagraphRun(t, elems)
	if run.Font != font.HelveticaBold {
		t.Errorf("placeholder font = %v, want HelveticaBold", run.Font)
	}
}

func TestCSSPlaceholderSkippedWhenInputHasValue(t *testing.T) {
	// Value is shown; the ::placeholder color must NOT be applied.
	src := `<style>
input { color: #000080; }
input::placeholder { color: #ff0000; }
</style>
<input type="text" value="hello" placeholder="Search...">`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	run := findFirstParagraphRun(t, elems)
	if run.Text != "hello" {
		t.Errorf("text = %q, want %q", run.Text, "hello")
	}
	if run.Color == layout.ColorRed {
		t.Error("value text was incorrectly styled with ::placeholder color")
	}
	if !colorApprox(run.Color, layout.ColorNavy) {
		t.Errorf("value color = %+v, want ColorNavy (%+v)", run.Color, layout.ColorNavy)
	}
}

func TestCSSPlaceholderSkippedWhenTextareaHasContent(t *testing.T) {
	src := `<style>
textarea::placeholder { color: #ff0000; }
</style>
<textarea>real content</textarea>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	run := findFirstParagraphRun(t, elems)
	if run.Color == layout.ColorRed {
		t.Error("textarea body was incorrectly styled with ::placeholder color")
	}
}

func TestCSSPlaceholderAttributeSelector(t *testing.T) {
	// Only the email input should pick up the rule.
	src := `<style>
input[type=email]::placeholder { color: #ff0000; }
</style>
<input type="text" placeholder="text-ph">
<input type="email" placeholder="email-ph">`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	var divs []*layout.Div
	for _, e := range elems {
		if d, ok := e.(*layout.Div); ok {
			divs = append(divs, d)
		}
	}
	if len(divs) < 2 {
		t.Fatalf("expected at least 2 input divs, got %d", len(divs))
	}
	getRun := func(d *layout.Div) layout.TextRun {
		for _, c := range d.Children() {
			if p, ok := c.(*layout.Paragraph); ok {
				return p.Runs()[0]
			}
		}
		t.Fatal("no paragraph in div")
		return layout.TextRun{}
	}
	textRun := getRun(divs[0])
	emailRun := getRun(divs[1])
	if textRun.Color == layout.ColorRed {
		t.Error("text input placeholder picked up email-only rule")
	}
	if emailRun.Color != layout.ColorRed {
		t.Errorf("email input placeholder color = %+v, want ColorRed", emailRun.Color)
	}
}

func TestCSSPlaceholderDoesNotLeakToSiblings(t *testing.T) {
	src := `<style>
input::placeholder { color: #ff0000; }
</style>
<input type="text" placeholder="ph">
<p>sibling paragraph</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Find the sibling paragraph (not inside the input Div).
	var siblingPara *layout.Paragraph
	for _, e := range elems {
		if p, ok := e.(*layout.Paragraph); ok {
			siblingPara = p
			break
		}
	}
	if siblingPara == nil {
		t.Fatal("sibling paragraph not found")
	}
	runs := siblingPara.Runs()
	if len(runs) == 0 {
		t.Fatal("sibling paragraph has no runs")
	}
	if runs[0].Color == layout.ColorRed {
		t.Error("sibling <p> incorrectly received ::placeholder color")
	}
}

// --- @media print ---

func TestCSSMediaPrint(t *testing.T) {
	html := `<style>
@media print {
    p { font-size: 14px; }
}
@media screen {
    p { font-size: 20px; }
}
</style>
<p>Print styles</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSMediaPrintNested(t *testing.T) {
	html := `<style>
p { color: black; }
@media print {
    .print-only { display: block; }
    .screen-only { display: none; }
}
</style>
<p class="print-only">Visible</p>
<p class="screen-only">Hidden</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	if len(elems) != 1 {
		t.Errorf("expected 1 visible element (screen-only hidden), got %d", len(elems))
	}
}

// --- Table caption ---

func TestConvertTableCaption(t *testing.T) {
	html := `<table>
<caption>Table 1: Sales Data</caption>
<tr><td>A</td><td>B</td></tr>
</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements (caption + table), got %d", len(elems))
	}
}

// --- Table col widths ---

func TestConvertTableColWidths(t *testing.T) {
	html := `<table>
<colgroup>
<col style="width: 30%%">
<col style="width: 70%%">
</colgroup>
<tr><td>Narrow</td><td>Wide</td></tr>
</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestConvertTableColSpan(t *testing.T) {
	html := `<table>
<col span="2" style="width: 50%%">
<tr><td>A</td><td>B</td></tr>
</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- Border collapse ---

func TestConvertTableBorderCollapse(t *testing.T) {
	html := `<table style="border-collapse: collapse; border: 1px solid black">
<tr><td>A</td><td>B</td></tr>
</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func findTable(elems []layout.Element) *layout.Table {
	for _, e := range elems {
		if tbl, ok := e.(*layout.Table); ok {
			return tbl
		}
		if div, ok := e.(*layout.Div); ok {
			if tbl := findTable(div.Children()); tbl != nil {
				return tbl
			}
		}
	}
	return nil
}

func TestConvertTableBorderCollapseDefault(t *testing.T) {
	// Per CSS 2.1 §17.6, the initial value for border-collapse is "separate".
	html := `<table border="1">
<tr><td>A</td><td>B</td></tr>
<tr><td>C</td><td>D</td></tr>
</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	tbl := findTable(elems)
	if tbl == nil {
		t.Fatal("expected a Table element")
	}
	if tbl.BorderCollapse() {
		t.Error("table should default to border-collapse: separate (CSS 2.1 §17.6)")
	}
}

func TestConvertTableBorderCollapseSeparateOverride(t *testing.T) {
	// Explicit border-collapse: separate should override the default.
	html := `<table style="border-collapse: separate; border: 1px solid black">
<tr><td>A</td><td>B</td></tr>
</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	tbl := findTable(elems)
	if tbl == nil {
		t.Fatal("expected a Table element")
	}
	if tbl.BorderCollapse() {
		t.Error("table with explicit border-collapse: separate should not collapse")
	}
}

// --- Font shorthand ---

func TestCSSFontShorthand(t *testing.T) {
	html := `<p style="font: bold 18px/1.5 courier">Styled text</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSFontShorthandItalic(t *testing.T) {
	html := `<p style="font: italic bold 14pt times">Italic bold</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- list-style-type ---

func TestCSSListStyleType(t *testing.T) {
	html := `<ul style="list-style-type: circle"><li>Item</li></ul>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- Border side shorthands ---

func TestCSSBorderSideShorthands(t *testing.T) {
	html := `<div style="border-top: 2px solid red; border-bottom: 1px dashed blue"><p>Bordered</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- Child combinator (>) ---

func TestCSSChildCombinator(t *testing.T) {
	html := `<style>
div > p { color: red; }
</style>
<div><p>Direct child</p><span><p>Nested (not direct)</p></span></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSChildCombinatorNoSpace(t *testing.T) {
	// Test "div>p" without spaces around >
	html := `<style>
div>p { font-weight: bold; }
</style>
<div><p>Bold</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- Adjacent sibling combinator (+) ---

func TestCSSAdjacentSiblingCombinator(t *testing.T) {
	html := `<style>
h1 + p { font-size: 20px; }
</style>
<h1>Title</h1><p>First para (styled)</p><p>Second para (not styled)</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements, got %d", len(elems))
	}
}

// --- General sibling combinator (~) ---

func TestCSSGeneralSiblingCombinator(t *testing.T) {
	html := `<style>
h1 ~ p { color: blue; }
</style>
<h1>Title</h1><p>Para 1</p><p>Para 2</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements, got %d", len(elems))
	}
}

// --- Universal selector (*) ---

func TestCSSUniversalSelector(t *testing.T) {
	html := `<style>
* { color: navy; }
</style>
<p>Universal</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSUniversalWithClass(t *testing.T) {
	html := `<style>
*.highlight { font-weight: bold; }
</style>
<p class="highlight">Highlighted</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- text-transform ---

func TestCSSTextTransformUppercase(t *testing.T) {
	html := `<p style="text-transform: uppercase">hello world</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSTextTransformLowercase(t *testing.T) {
	html := `<p style="text-transform: lowercase">HELLO WORLD</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSTextTransformCapitalize(t *testing.T) {
	html := `<p style="text-transform: capitalize">hello world foo</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSTextTransformInheritance(t *testing.T) {
	html := `<style>
div { text-transform: uppercase; }
</style>
<div><p>should be upper</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- white-space ---

func TestCSSWhiteSpacePre(t *testing.T) {
	html := `<p style="white-space: pre">hello    world
second line</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSWhiteSpaceNormal(t *testing.T) {
	html := `<p style="white-space: normal">hello    world
same line</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- Combined combinator tests ---

func TestCSSMixedCombinators(t *testing.T) {
	html := `<style>
div > ul > li:first-child { font-weight: bold; }
</style>
<div><ul><li>First</li><li>Second</li></ul></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSCombinatorWithDescendant(t *testing.T) {
	// div > p span — child then descendant
	html := `<style>
div > p span { color: red; }
</style>
<div><p>Normal <span>Red text</span></p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- Unit tests for helpers ---

func TestApplyTextTransform(t *testing.T) {
	tests := []struct {
		input     string
		transform string
		want      string
	}{
		{"hello world", "uppercase", "HELLO WORLD"},
		{"HELLO WORLD", "lowercase", "hello world"},
		{"hello world foo", "capitalize", "Hello World Foo"},
		{"hello", "none", "hello"},
		{"hello", "", "hello"},
	}
	for _, tt := range tests {
		got := applyTextTransform(tt.input, tt.transform)
		if got != tt.want {
			t.Errorf("applyTextTransform(%q, %q) = %q, want %q", tt.input, tt.transform, got, tt.want)
		}
	}
}

func TestProcessWhitespace(t *testing.T) {
	tests := []struct {
		input string
		ws    string
		want  string
	}{
		{"hello    world", "normal", "hello world"},
		{"hello    world\nsecond", "normal", "hello world second"},
		{"hello    world", "pre", "hello    world"},
		{"hello    world\nsecond", "pre", "hello    world\nsecond"},
		{"hello    world\nsecond", "pre-line", "hello world\nsecond"},
	}
	for _, tt := range tests {
		got := processWhitespace(tt.input, tt.ws)
		if got != tt.want {
			t.Errorf("processWhitespace(%q, %q) = %q, want %q", tt.input, tt.ws, got, tt.want)
		}
	}
}

func TestNormalizeCombinators(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"div>p", "div > p"},
		{"div > p", "div  >  p"},
		{"h1+p", "h1 + p"},
		{"p:nth-child(2n+1)", "p:nth-child(2n+1)"},
		{"div>p:nth-child(2n+1)", "div > p:nth-child(2n+1)"},
	}
	for _, tt := range tests {
		got := normalizeCombinators(tt.input)
		if got != tt.want {
			t.Errorf("normalizeCombinators(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestParseSelectorCombinators(t *testing.T) {
	// "div > p" should have 2 parts with child combinator
	sel := parseSelector("div > p")
	if len(sel.parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(sel.parts))
	}
	if sel.parts[0].tag != "div" {
		t.Errorf("part 0 tag = %q, want 'div'", sel.parts[0].tag)
	}
	if sel.parts[1].tag != "p" {
		t.Errorf("part 1 tag = %q, want 'p'", sel.parts[1].tag)
	}
	if sel.parts[1].combinator != ">" {
		t.Errorf("part 1 combinator = %q, want '>'", sel.parts[1].combinator)
	}
}

func TestParseSelectorAdjacentSibling(t *testing.T) {
	sel := parseSelector("h1 + p")
	if len(sel.parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(sel.parts))
	}
	if sel.parts[1].combinator != "+" {
		t.Errorf("combinator = %q, want '+'", sel.parts[1].combinator)
	}
}

func TestParseSelectorUniversal(t *testing.T) {
	sel := parseSelector("*")
	if len(sel.parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(sel.parts))
	}
	if sel.parts[0].tag != "*" {
		t.Errorf("tag = %q, want '*'", sel.parts[0].tag)
	}
	// Universal selector adds 0 specificity
	if sel.specificity != 0 {
		t.Errorf("specificity = %d, want 0", sel.specificity)
	}
}

func TestParseSelectorUniversalWithClass(t *testing.T) {
	sel := parseSelector("*.highlight")
	if len(sel.parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(sel.parts))
	}
	if sel.parts[0].tag != "*" {
		t.Errorf("tag = %q, want '*'", sel.parts[0].tag)
	}
	if sel.parts[0].class != "highlight" {
		t.Errorf("class = %q, want 'highlight'", sel.parts[0].class)
	}
	if sel.specificity != 10 {
		t.Errorf("specificity = %d, want 10", sel.specificity)
	}
}

// --- <hr> as LineSeparator ---

func TestConvertHrLineSeparator(t *testing.T) {
	elems, err := Convert("<hr>", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	// Should be a LineSeparator, not a paragraph.
	if _, ok := elems[0].(*layout.LineSeparator); !ok {
		t.Errorf("expected *layout.LineSeparator, got %T", elems[0])
	}
}

func TestConvertHrStyledCSS(t *testing.T) {
	html := `<style>hr { border-top: 2px solid red; }</style><hr>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	if _, ok := elems[0].(*layout.LineSeparator); !ok {
		t.Errorf("expected *layout.LineSeparator, got %T", elems[0])
	}
}

func TestConvertHrLayout(t *testing.T) {
	elems, err := Convert("<hr>", nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

// --- <dl>/<dt>/<dd> definition lists ---

func TestConvertDefinitionList(t *testing.T) {
	html := `<dl>
<dt>Term 1</dt>
<dd>Definition 1</dd>
<dt>Term 2</dt>
<dd>Definition 2</dd>
</dl>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element (div wrapper), got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestConvertDefinitionListStyled(t *testing.T) {
	html := `<style>
dt { color: navy; }
dd { font-style: italic; }
</style>
<dl><dt>Key</dt><dd>Value</dd></dl>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestConvertDefinitionListEmpty(t *testing.T) {
	html := `<dl></dl>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Empty dl produces a div with no children — may produce 1 empty element
	// Just verify no crash.
	_ = elems
}

// --- <figure>/<figcaption> ---

func TestConvertFigure(t *testing.T) {
	html := `<figure>
<p>Some content here</p>
<figcaption>Figure 1: Description</figcaption>
</figure>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestConvertFigureWithImage(t *testing.T) {
	// Image won't load but should fallback to alt text.
	html := `<figure>
<img src="photo.jpg" alt="A photo">
<figcaption>Photo caption</figcaption>
</figure>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestConvertFigcaptionItalic(t *testing.T) {
	html := `<figure><figcaption>Caption text</figcaption></figure>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- <sub>/<sup> font size reduction ---

func TestConvertSubFontSize(t *testing.T) {
	html := `<p>H<sub>2</sub>O</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestConvertSupFontSize(t *testing.T) {
	html := `<p>E=mc<sup>2</sup></p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestConvertSubSupInline(t *testing.T) {
	html := `<p>x<sub>i</sub> + y<sup>2</sup></p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	// Verify it lays out.
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

// --- Multiple <hr> in sequence ---

func TestConvertMultipleHr(t *testing.T) {
	html := `<p>Above</p><hr><hr><p>Below</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	separators := 0
	for _, e := range elems {
		if _, ok := e.(*layout.LineSeparator); ok {
			separators++
		}
	}
	if separators != 2 {
		t.Errorf("expected 2 LineSeparators, got %d", separators)
	}
}

// --- <dl> with text-transform ---

func TestConvertDefinitionListTextTransform(t *testing.T) {
	html := `<style>dt { text-transform: uppercase; }</style>
<dl><dt>term</dt><dd>definition</dd></dl>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestLetterSpacing(t *testing.T) {
	html := `<p style="letter-spacing: 2px">Spaced</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed height, got %f", plan.Consumed)
	}
}

func TestLetterSpacingInheritance(t *testing.T) {
	html := `<div style="letter-spacing: 3px"><p>Inherited spacing</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestWordSpacing(t *testing.T) {
	html := `<p style="word-spacing: 5px">Word spaced text here</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestTextIndent(t *testing.T) {
	html := `<p style="text-indent: 30px">Indented paragraph text that should have the first line indented.</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestTextIndentEmUnit(t *testing.T) {
	html := `<p style="text-indent: 2em">Indented by 2em</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestMaxWidth(t *testing.T) {
	html := `<div style="max-width: 200px; padding: 10px"><p>Constrained width</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	// Layout in a wide area — the div should not exceed max-width.
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestMinWidth(t *testing.T) {
	html := `<div style="min-width: 300px; padding: 5px"><p>Minimum width</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestLetterSpacingNormal(t *testing.T) {
	html := `<p style="letter-spacing: normal">Normal spacing</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestWordSpacingNormal(t *testing.T) {
	html := `<p style="word-spacing: normal">Normal word spacing</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestStyleBlockSpacing(t *testing.T) {
	html := `<html><head><style>
		.spaced { letter-spacing: 1px; word-spacing: 3px; text-indent: 20px; }
	</style></head><body>
	<p class="spaced">Styled with letter and word spacing</p>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestMaxWidthMinWidthStyleBlock(t *testing.T) {
	html := `<html><head><style>
		.container { max-width: 300px; min-width: 100px; padding: 8px; border: 1px solid black; }
	</style></head><body>
	<div class="container"><p>Box with constraints</p></div>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- Table Advanced Tests ---

func TestTableVerticalAlign(t *testing.T) {
	html := `<table border="1">
<tr>
<td style="vertical-align: middle; height: 60px">Middle</td>
<td style="vertical-align: bottom">Bottom</td>
<td>Top (default)</td>
</tr></table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	for _, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
		if plan.Status == layout.LayoutNothing {
			t.Error("unexpected LayoutNothing")
		}
	}
}

func TestTablePerCellBorders(t *testing.T) {
	html := `<table>
<tr>
<td style="border: 2px solid red">Red border</td>
<td style="border-bottom: 1px dashed blue">Dashed bottom</td>
<td>No border</td>
</tr></table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestTablePerSidePadding(t *testing.T) {
	html := `<table border="1">
<tr>
<td style="padding: 10px 20px 5px 15px">Different padding each side</td>
<td style="padding-left: 30px">Left padded</td>
</tr></table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	for _, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
		if plan.Status == layout.LayoutNothing {
			t.Error("unexpected LayoutNothing")
		}
	}
}

func TestTableFooterRows(t *testing.T) {
	html := `<table border="1">
<thead><tr><th>Header</th></tr></thead>
<tfoot><tr><td>Footer</td></tr></tfoot>
<tbody>
<tr><td>Row 1</td></tr>
<tr><td>Row 2</td></tr>
<tr><td>Row 3</td></tr>
</tbody>
</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	for _, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
		if plan.Status == layout.LayoutNothing {
			t.Error("unexpected LayoutNothing")
		}
	}
}

func TestTableFooterRepeatOnSplit(t *testing.T) {
	rows := ""
	for i := 0; i < 50; i++ {
		rows += `<tr><td>Data</td><td>Value</td></tr>`
	}
	html := `<table border="1">
<thead><tr><th>Col A</th><th>Col B</th></tr></thead>
<tfoot><tr><td>Total A</td><td>Total B</td></tr></tfoot>
<tbody>` + rows + `</tbody></table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	for _, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 400, Height: 200})
		if plan.Status == layout.LayoutNothing {
			t.Error("unexpected LayoutNothing")
		}
		if plan.Status == layout.LayoutPartial && plan.Overflow == nil {
			t.Error("expected overflow on partial layout")
		}
	}
}

func TestTableRowBackground(t *testing.T) {
	html := `<table border="1">
<tr style="background-color: #f0f0f0">
<td>Cell in gray row</td>
<td>Another cell</td>
</tr>
<tr>
<td>Cell in default row</td>
<td style="background-color: yellow">Yellow cell</td>
</tr>
</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestTableWithMaxWidth(t *testing.T) {
	html := `<table border="1" style="max-width: 300px">
<tr><td>Constrained table</td><td>Width limited</td></tr>
</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestTableCSSStyledBorders(t *testing.T) {
	html := `<html><head><style>
		table { border-collapse: collapse; }
		th { border-bottom: 2px solid black; padding: 8px; }
		td { border: 1px solid #ccc; padding: 6px 12px; }
		td.highlight { border: 2px solid red; background-color: #fff3f3; }
	</style></head><body>
	<table>
	<thead><tr><th>Name</th><th>Value</th></tr></thead>
	<tbody>
	<tr><td>Alpha</td><td class="highlight">100</td></tr>
	<tr><td>Beta</td><td>200</td></tr>
	</tbody>
	</table>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	for _, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
		if plan.Status == layout.LayoutNothing {
			t.Error("unexpected LayoutNothing")
		}
	}
}

func TestTableCellBorderRadiusSeparate(t *testing.T) {
	// Default table mode is now "separate" (CSS 2.1 §17.6).
	// border-radius on cells should render.
	htmlStr := `<table>
		<tr>
			<th style="background:#4f46e5; color:white; padding:8px; border-top-left-radius:8px">Desc</th>
			<th style="background:#4f46e5; color:white; padding:8px; border-top-right-radius:8px">Amt</th>
		</tr>
		<tr><td style="padding:8px">A</td><td style="padding:8px">$10</td></tr>
	</table>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	// Render through full pipeline — rounded corners should appear.
	r := layout.NewRenderer(612, 792, layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	for _, e := range elems {
		r.Add(e)
	}
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}
}

func TestTableCellBorderRadiusExplicitSeparate(t *testing.T) {
	// Explicitly set border-collapse: separate — radius should work.
	htmlStr := `<style>
		table { border-collapse: separate; }
		th:first-child { border-top-left-radius: 8px; }
		th:last-child { border-top-right-radius: 8px; }
		th { background: #4f46e5; color: white; padding: 8px; }
	</style>
	<table><tr><th>Description</th><th>Amount</th></tr>
	<tr><td>Item</td><td>$10</td></tr></table>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	for _, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
		if plan.Status == layout.LayoutNothing {
			t.Error("unexpected LayoutNothing")
		}
	}
}

func TestTableCellBorderRadiusCollapse(t *testing.T) {
	// Explicit border-collapse: collapse — radius should be ignored per
	// CSS Backgrounds Level 3 §5.3.
	htmlStr := `<style>
		table { border-collapse: collapse; }
		th { border-radius: 8px; border: 1px solid black; padding: 8px; }
	</style>
	<table><tr><th>A</th><th>B</th></tr></table>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
		if plan.Status == layout.LayoutNothing {
			t.Error("unexpected LayoutNothing")
		}
	}
}

func TestTableCellBorderRadiusCollapseStillCollapses(t *testing.T) {
	// Regression: explicit collapse mode should still collapse borders
	// (remove interior right/bottom borders).
	htmlStr := `<style>
		table { border-collapse: collapse; }
		td { border: 2px solid black; padding: 8px; }
	</style>
	<table><tr><td>A</td><td>B</td></tr><tr><td>C</td><td>D</td></tr></table>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Render — should not panic or produce visual artifacts.
	r := layout.NewRenderer(612, 792, layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	for _, e := range elems {
		r.Add(e)
	}
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}
}

func TestTableCellBorderRadiusAPI(t *testing.T) {
	// Verify layout API: SetBorderRadiusPerCorner on cells renders correctly.
	tbl := layout.NewTable()
	tbl.SetColumnWidths([]float64{200, 200})
	row := tbl.AddRow()
	c1 := row.AddCell("A", font.Helvetica, 12)
	c1.SetBorderRadiusPerCorner(8, 0, 0, 0)
	c1.SetBackground(layout.RGB(0.3, 0.3, 0.9))
	c1.SetBorders(layout.AllBorders(layout.SolidBorder(1, layout.ColorBlack)))
	c2 := row.AddCell("B", font.Helvetica, 12)
	c2.SetBorderRadiusPerCorner(0, 8, 0, 0)
	c2.SetBackground(layout.RGB(0.3, 0.3, 0.9))
	c2.SetBorders(layout.AllBorders(layout.SolidBorder(1, layout.ColorBlack)))

	plan := tbl.PlanLayout(layout.LayoutArea{Width: 400, Height: 500})
	if plan.Status == layout.LayoutNothing {
		t.Fatal("expected layout output")
	}

	// Render through full pipeline.
	r := layout.NewRenderer(612, 792, layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(tbl)
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}
}

func TestTableStripedRows(t *testing.T) {
	html := `<html><head><style>
		tr:nth-child(even) { background-color: #f2f2f2; }
		td { padding: 8px; }
	</style></head><body>
	<table border="1">
	<tr><td>Row 1</td></tr>
	<tr><td>Row 2</td></tr>
	<tr><td>Row 3</td></tr>
	<tr><td>Row 4</td></tr>
	</table>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestTableNestedTable(t *testing.T) {
	html := `<table border="1">
<tr>
<td>
  <table border="1">
  <tr><td>Inner A</td><td>Inner B</td></tr>
  </table>
</td>
<td>Outer cell</td>
</tr>
</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	for _, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
		if plan.Status == layout.LayoutNothing {
			t.Error("unexpected LayoutNothing")
		}
		if plan.Consumed <= 0 {
			t.Error("expected positive consumed height")
		}
	}
}

// --- Form Element Tests ---

func TestInputText(t *testing.T) {
	html := `<input type="text" value="Hello World">`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 300, Height: 500})
	if plan.Status == layout.LayoutNothing {
		t.Error("unexpected LayoutNothing")
	}
}

func TestInputTextPlaceholder(t *testing.T) {
	html := `<input type="text" placeholder="Enter name...">`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestInputPassword(t *testing.T) {
	html := `<input type="password" value="secret">`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestInputCheckbox(t *testing.T) {
	html := `<input type="checkbox" checked> <input type="checkbox">`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element for checkbox")
	}
}

func TestInputRadio(t *testing.T) {
	html := `<input type="radio" checked> <input type="radio">`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element for radio")
	}
}

func TestInputSubmit(t *testing.T) {
	html := `<input type="submit" value="Send">`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestInputSubmitDefault(t *testing.T) {
	html := `<input type="submit">`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestInputHidden(t *testing.T) {
	html := `<input type="hidden" name="token" value="abc">`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 0 {
		t.Fatalf("hidden input should produce no elements, got %d", len(elems))
	}
}

func TestButton(t *testing.T) {
	html := `<button>Click Me</button>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestSelect(t *testing.T) {
	html := `<select>
<option>Apple</option>
<option selected>Banana</option>
<option>Cherry</option>
</select>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestSelectNoSelected(t *testing.T) {
	html := `<select><option>First</option><option>Second</option></select>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestTextarea(t *testing.T) {
	html := `<textarea>Some multi-line text content here</textarea>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestTextareaPlaceholder(t *testing.T) {
	html := `<textarea placeholder="Write here..."></textarea>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestFieldset(t *testing.T) {
	html := `<fieldset>
<legend>Personal Info</legend>
<p>Name: <input type="text" value="John"></p>
<p>Email: <input type="text" value="john@example.com"></p>
</fieldset>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status == layout.LayoutNothing {
		t.Error("unexpected LayoutNothing")
	}
}

func TestLabel(t *testing.T) {
	html := `<label>Username:</label> <input type="text">`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestFormComplete(t *testing.T) {
	html := `<form>
<fieldset>
<legend>Registration</legend>
<p><label>Name:</label> <input type="text" value="Jane"></p>
<p><label>Password:</label> <input type="password" value="pass123"></p>
<p><label>Gender:</label>
  <input type="radio" checked> Male
  <input type="radio"> Female</p>
<p><label>Country:</label>
  <select><option>USA</option><option selected>UK</option></select></p>
<p><label>Bio:</label></p>
<textarea placeholder="Tell us about yourself"></textarea>
<p><input type="checkbox" checked> I agree to terms</p>
<button>Register</button>
</fieldset>
</form>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	for _, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 500, Height: 2000})
		if plan.Status == layout.LayoutNothing {
			t.Error("unexpected LayoutNothing")
		}
	}
}

// --- CSS Layout Polish Tests ---

func TestBorderRadius(t *testing.T) {
	html := `<div style="border: 1px solid black; border-radius: 10px; padding: 8px">
<p>Rounded corners</p>
</div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status == layout.LayoutNothing {
		t.Error("unexpected LayoutNothing")
	}
}

// TestBorderRadiusShorthandWithCalc is a regression test for the same
// strings.Fields tokenization bug fixed for `flex:` (#236),
// `margin:`/`padding:` (#237), `font:` (#240), `border:` (#242),
// `background-size:` (#244), `@page size` (#247), and `gap:` (#249) —
// applied here to the `border-radius` shorthand. Pre-fix
// `border-radius: calc(4px + 2px) 8px` became 4 tokens
// ["calc(4px", "+", "2px)", "8px"], which fell into the 4-arity case
// and parsed each fragment as its own corner, yielding 0 for the calc
// fragments and silently shifting the corner mapping.
//
// CSS border-radius arity → corner mapping (TL, TR, BR, BL):
//
//	1 value:  v       → all four corners = v
//	2 values: a b     → TL=BR=a, TR=BL=b
//	3 values: a b c   → TL=a, TR=BL=b, BR=c
//	4 values: a b c d → TL=a, TR=b, BR=c, BL=d
func TestBorderRadiusShorthandWithCalc(t *testing.T) {
	tests := []struct {
		name                           string
		val                            string
		wantTL, wantTR, wantBR, wantBL float64
	}{
		{
			name: "1-value calc applies to all four corners",
			val:  "calc(4px + 2px)",
			// 6px = 4.5pt.
			wantTL: 4.5, wantTR: 4.5, wantBR: 4.5, wantBL: 4.5,
		},
		{
			name: "2-value: calc TL/BR, plain TR/BL",
			val:  "calc(4px + 2px) 8px",
			// TL=BR=6px=4.5pt; TR=BL=8px=6pt.
			wantTL: 4.5, wantTR: 6, wantBR: 4.5, wantBL: 6,
		},
		{
			name: "2-value: plain TL/BR, calc TR/BL",
			val:  "8px calc(2px * 2)",
			// TL=BR=6pt; TR=BL=4px=3pt.
			wantTL: 6, wantTR: 3, wantBR: 6, wantBL: 3,
		},
		{
			name: "3-value with calc in TL position",
			val:  "calc(4px + 4px) 6px 12px",
			// TL=8px=6pt; TR=BL=6px=4.5pt; BR=12px=9pt.
			wantTL: 6, wantTR: 4.5, wantBR: 9, wantBL: 4.5,
		},
		{
			name: "3-value with calc in middle (TR=BL position)",
			// Locks the TR/BL shared slot — a regression that broke the
			// shared assignment (TR=parts[1], BL=parts[1]) would diverge.
			val: "8px calc(2px + 2px) 12px",
			// TL=8px=6pt; TR=BL=4px=3pt; BR=12px=9pt.
			wantTL: 6, wantTR: 3, wantBR: 9, wantBL: 3,
		},
		{
			name: "4-value with four distinct corner values",
			// Each corner gets a different value so a regression that
			// swapped any two corners (e.g. TR↔BR or BR↔BL) would fail
			// clearly on the assertion for the affected corner.
			val: "calc(2px + 2px) calc(4px + 4px) calc(8px + 4px) calc(8px + 8px)",
			// TL=4px=3pt, TR=8px=6pt, BR=12px=9pt, BL=16px=12pt.
			wantTL: 3, wantTR: 6, wantBR: 9, wantBL: 12,
		},
		{
			name: "min() single value",
			val:  "min(4px, 8px)",
			// min picks 4px = 3pt.
			wantTL: 3, wantTR: 3, wantBR: 3, wantBL: 3,
		},
		{
			name: "max() single value",
			val:  "max(4px, 8px)",
			// max picks 8px = 6pt.
			wantTL: 6, wantTR: 6, wantBR: 6, wantBL: 6,
		},
		{
			name: "clamp() single value",
			val:  "clamp(2px, 8px, 16px)",
			// middle wins: 8px = 6pt.
			wantTL: 6, wantTR: 6, wantBR: 6, wantBL: 6,
		},
		{
			name:   "tab separator",
			val:    "calc(4px + 2px)\t8px",
			wantTL: 4.5, wantTR: 6, wantBR: 4.5, wantBL: 6,
		},
		{
			name:   "newline separator",
			val:    "calc(4px + 2px)\n8px",
			wantTL: 4.5, wantTR: 6, wantBR: 4.5, wantBL: 6,
		},
		{
			name: "5+ tokens hit the default branch (no-op)",
			// Switch has no `case 5`/`default` body, so all four corners
			// stay at zero default. Documents the contract.
			val:    "1px 2px 3px 4px 5px",
			wantTL: 0, wantTR: 0, wantBR: 0, wantBL: 0,
		},
		{
			name: "unbalanced calc paren: corners stay 0",
			// splitTopLevelFields keeps the unbalanced calc + trailing
			// chars as one token; parseLength fails → 1-value branch
			// assigns 0 to all corners. No crash.
			val:    "calc(4px + 2px",
			wantTL: 0, wantTR: 0, wantBR: 0, wantBL: 0,
		},
		{
			name:   "empty value: corners stay 0",
			val:    "",
			wantTL: 0, wantTR: 0, wantBR: 0, wantBL: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &converter{}
			style := &computedStyle{FontSize: 12}
			c.applyProperty("border-radius", tt.val, style)
			if math.Abs(style.BorderRadiusTL-tt.wantTL) > 0.01 {
				t.Errorf("BorderRadiusTL = %.4f, want %.4f", style.BorderRadiusTL, tt.wantTL)
			}
			if math.Abs(style.BorderRadiusTR-tt.wantTR) > 0.01 {
				t.Errorf("BorderRadiusTR = %.4f, want %.4f", style.BorderRadiusTR, tt.wantTR)
			}
			if math.Abs(style.BorderRadiusBR-tt.wantBR) > 0.01 {
				t.Errorf("BorderRadiusBR = %.4f, want %.4f", style.BorderRadiusBR, tt.wantBR)
			}
			if math.Abs(style.BorderRadiusBL-tt.wantBL) > 0.01 {
				t.Errorf("BorderRadiusBL = %.4f, want %.4f", style.BorderRadiusBL, tt.wantBL)
			}
		})
	}
}

func TestOpacity(t *testing.T) {
	html := `<div style="opacity: 0.5; background-color: blue; padding: 10px">
<p>Semi-transparent</p>
</div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestOverflowHidden(t *testing.T) {
	html := `<div style="overflow: hidden; border: 1px solid black; padding: 5px">
<p>Content that might overflow</p>
</div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestBorderRadiusStyleBlock(t *testing.T) {
	html := `<html><head><style>
		.card { border: 1px solid #ddd; border-radius: 8px; padding: 16px; background: #f9f9f9; }
	</style></head><body>
	<div class="card"><p>Card content</p></div>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestOpacityValues(t *testing.T) {
	// Test edge cases: 0, 1, and a negative value should be clamped.
	html := `<div style="opacity: 0; padding: 1px"><p>Invisible</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCSSPolishCombined(t *testing.T) {
	html := `<html><head><style>
		.fancy {
			border: 2px solid navy;
			border-radius: 12px;
			opacity: 0.8;
			overflow: hidden;
			padding: 12px;
			background-color: #eef;
		}
	</style></head><body>
	<div class="fancy">
	<p>All CSS polish features combined</p>
	</div>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	for _, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
		if plan.Status == layout.LayoutNothing {
			t.Error("unexpected LayoutNothing")
		}
	}
}

func TestFormWithCSSStyling(t *testing.T) {
	html := `<html><head><style>
		input[type="text"] { border: 1px solid #ccc; border-radius: 4px; padding: 6px 10px; }
		button { background-color: #007bff; color: white; border-radius: 4px; padding: 8px 16px; border: none; }
		fieldset { border: 1px solid #ddd; border-radius: 8px; }
	</style></head><body>
	<fieldset>
	<legend>Login</legend>
	<p><label>Email:</label> <input type="text" placeholder="you@example.com"></p>
	<p><label>Password:</label> <input type="password"></p>
	<button>Sign In</button>
	</fieldset>
	</body></html>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	for _, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 500, Height: 2000})
		if plan.Status == layout.LayoutNothing {
			t.Error("unexpected LayoutNothing")
		}
	}
}

// --- Feature 1: External CSS (<link rel="stylesheet">) ---

func TestConvertExternalCSS(t *testing.T) {
	dir := t.TempDir()
	cssPath := filepath.Join(dir, "style.css")
	_ = os.WriteFile(cssPath, []byte("p { color: red; font-size: 24px; }"), 0644)

	htmlStr := `<html><head><link rel="stylesheet" href="style.css"></head><body><p>Styled</p></body></html>`
	elems, err := Convert(htmlStr, &Options{BaseFS: os.DirFS(dir)})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements from externally styled HTML")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status == layout.LayoutNothing {
		t.Error("unexpected LayoutNothing")
	}
}

func TestConvertExternalCSSMissingFile(t *testing.T) {
	htmlStr := `<html><head><link rel="stylesheet" href="missing.css"></head><body><p>OK</p></body></html>`
	elems, err := Convert(htmlStr, &Options{BaseFS: os.DirFS(t.TempDir())})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("should still produce elements even if CSS file is missing")
	}
}

func TestConvertExternalCSSOverriddenByStyle(t *testing.T) {
	dir := t.TempDir()
	cssPath := filepath.Join(dir, "base.css")
	_ = os.WriteFile(cssPath, []byte("p { font-size: 10px; }"), 0644)

	htmlStr := `<html><head>
		<link rel="stylesheet" href="base.css">
		<style>p { font-size: 20px; }</style>
	</head><body><p>Text</p></body></html>`
	elems, err := Convert(htmlStr, &Options{BaseFS: os.DirFS(dir)})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

// --- Feature 2: CSS float ---

func TestConvertCSSFloatLeft(t *testing.T) {
	htmlStr := `<div style="float: left; width: 100px"><p>Sidebar</p></div><p>Main content</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements (float + paragraph), got %d", len(elems))
	}
	if _, ok := elems[0].(*layout.Float); !ok {
		t.Errorf("expected first element to be *layout.Float, got %T", elems[0])
	}
}

func TestConvertCSSFloatRight(t *testing.T) {
	htmlStr := `<div style="float: right"><p>Right</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	if _, ok := elems[0].(*layout.Float); !ok {
		t.Errorf("expected *layout.Float, got %T", elems[0])
	}
}

func TestConvertCSSFloatNone(t *testing.T) {
	htmlStr := `<div style="float: none"><p>Normal</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	if _, ok := elems[0].(*layout.Float); ok {
		t.Error("float:none should not produce a Float element")
	}
}

// --- Feature 3: @font-face ---

func TestConvertFontFaceParsing(t *testing.T) {
	htmlStr := `<html><head><style>
		@font-face {
			font-family: "CustomFont";
			src: url("nonexistent.ttf");
		}
		p { font-family: "CustomFont"; }
	</style></head><body><p>Text</p></body></html>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements even with missing font")
	}
}

// TestCustomFontFamilyResolution verifies that a custom @font-face family
// name is preserved through CSS parsing and matched against embedded fonts
// during resolution, rather than being mapped to "helvetica". This is the
// regression test for https://github.com/carlos7ags/folio/issues/16.
func TestCustomFontFamilyResolution(t *testing.T) {
	// Construct a converter with a mock embedded font entry keyed as
	// "noto|400|normal" — simulating a loaded @font-face with
	// font-family: "Noto" and the default font-weight: normal (400).
	mockEF := font.NewEmbeddedFont(nil)
	c := &converter{
		embeddedFonts: map[string]*font.EmbeddedFont{
			"noto|400|normal": mockEF,
		},
	}

	// Simulate the CSS pipeline: parseFontFamily normalizes "Noto" to "noto",
	// then resolveFontPair should match the embedded font.
	style := defaultStyle()
	style.FontFamily = parseFontFamily(`"Noto"`)

	if style.FontFamily != "noto" {
		t.Fatalf("parseFontFamily(%q) = %q, want %q", `"Noto"`, style.FontFamily, "noto")
	}

	std, ef := c.resolveFontPair(style)
	if ef != mockEF {
		t.Errorf("expected embedded font for family %q, got standard font %v", style.FontFamily, std)
	}
	if std != nil {
		t.Errorf("expected nil standard font when embedded font matches, got %v", std)
	}
}

// TestCustomFontFamilyFallback verifies that an unknown family name that
// does not match any @font-face still falls back to a standard font.
func TestCustomFontFamilyFallback(t *testing.T) {
	c := &converter{
		embeddedFonts: make(map[string]*font.EmbeddedFont),
	}

	style := defaultStyle()
	style.FontFamily = parseFontFamily(`"UnknownFont"`)

	std, ef := c.resolveFontPair(style)
	if ef != nil {
		t.Error("expected nil embedded font for unknown family")
	}
	if std != font.Helvetica {
		t.Errorf("expected Helvetica fallback, got %v", std)
	}
}

// TestCustomFontFamilyWithFontShorthand verifies that the font shorthand
// property also preserves custom family names.
func TestCustomFontFamilyWithFontShorthand(t *testing.T) {
	_, _, _, _, family := parseFontShorthand("12px CustomFont", 12)
	if family != "customfont" {
		t.Errorf("parseFontShorthand font-family = %q, want %q", family, "customfont")
	}

	_, _, _, _, family = parseFontShorthand("bold 16px 'Noto Sans', sans-serif", 12)
	if family != "noto sans" {
		t.Errorf("parseFontShorthand font-family = %q, want %q", family, "noto sans")
	}
}

// TestParseFontShorthandWithCalc is a regression test for the same
// strings.Fields tokenization bug fixed for `flex:` (#236) and
// `margin:`/`padding:` (#237) — applied here to `font:`. Pre-fix,
// `font: calc(1em + 2px) sans-serif` was split into 4 tokens
// ["calc(1em", "+", "2px)", "sans-serif"], the size position got
// "calc(1em" (unbalanced calc → parseLength nil → size 0), and the
// remaining tokens were mis-routed into font-family.
//
// Pre-existing limitation NOT addressed here:
//   - Whitespace around the slash (e.g. `font: 12px / 1.5 sans`)
//     makes `/` its own token and it ends up consumed as font-family.
//
// The paren-blind slash detector — which previously misread `/` inside
// calc as the line-height separator — was fixed in a follow-up: see the
// `calc with division in size` and `calc on both sides of slash` cases
// below, which now pass thanks to indexByteAtTopLevel().
func TestParseFontShorthandWithCalc(t *testing.T) {
	const parentSize = 12.0
	tests := []struct {
		name           string
		input          string
		wantStyle      string
		wantWeight     int
		wantSize       float64 // in pt
		wantLineHeight float64
		wantFamily     string
	}{
		{
			name:  "calc in size, single family",
			input: "calc(1em + 2px) sans-serif",
			// 1em at parent=12pt → 12pt; +2px(=1.5pt) → 13.5pt.
			wantSize:   13.5,
			wantFamily: "sans-serif",
		},
		{
			name:  "min() in size",
			input: "min(14px, 1.2em) sans-serif",
			// min(10.5pt, 14.4pt) = 10.5pt.
			wantSize:   10.5,
			wantFamily: "sans-serif",
		},
		{
			name:  "max() in size",
			input: "max(8px, 14px) sans-serif",
			// max(6pt, 10.5pt) = 10.5pt.
			wantSize:   10.5,
			wantFamily: "sans-serif",
		},
		{
			name:  "clamp() in size",
			input: "clamp(8px, 16px, 24px) sans-serif",
			// middle wins: 16px = 12pt.
			wantSize:   12,
			wantFamily: "sans-serif",
		},
		{
			name:  "calc in size with /line-height (no surrounding spaces)",
			input: "calc(1em + 2px)/1.5 sans-serif",
			// Size resolves to 13.5pt; line-height is the unitless multiplier 1.5.
			wantSize:       13.5,
			wantLineHeight: 1.5,
			wantFamily:     "sans-serif",
		},
		{
			name:       "italic + calc + family",
			input:      "italic calc(1em + 2px) sans-serif",
			wantStyle:  "italic",
			wantSize:   13.5,
			wantFamily: "sans-serif",
		},
		{
			name:       "bold + calc + multi-word family",
			input:      "bold calc(1em + 2px) Helvetica Neue",
			wantWeight: 700,
			wantSize:   13.5,
			wantFamily: "helvetica neue",
		},
		{
			name:  "calc with subtraction",
			input: "calc(2em - 4px) sans-serif",
			// 2em at 12pt = 24pt; -4px(=3pt) = 21pt.
			wantSize:   21,
			wantFamily: "sans-serif",
		},
		{
			name:  "calc with multiplication",
			input: "calc(1em * 1.5) sans-serif",
			// 12pt * 1.5 = 18pt.
			wantSize:   18,
			wantFamily: "sans-serif",
		},
		{
			name: "calc with division in size — paren-aware slash detector",
			// Pre-fix the paren-blind strings.IndexByte('/') would split
			// "calc(3em / 2)" mid-expression into "calc(3em " and " 2)";
			// parseFontSize("calc(3em ") fails and falls back to
			// parentSize=12pt. Post-fix indexByteAtTopLevel skips `/`
			// inside parens and the calc resolves to 3*12/2 = 18pt.
			// The pre-fix fallback (12) and post-fix value (18) differ,
			// so a regression to paren-blind splitting fails clearly.
			input:      "calc(3em / 2) sans-serif",
			wantSize:   18,
			wantFamily: "sans-serif",
		},
		{
			name: "calc on both sides of slash",
			// Size and line-height are both calc expressions. The outer
			// `/` is at depth 0; the inner `*` and `+` are inside parens.
			input: "calc(1em + 2px)/calc(1em * 1.5) sans-serif",
			// Size: 12 + 1.5 = 13.5pt. Line-height calc resolves
			// at the size's fontSize (13.5pt as relativeTo): 1em *
			// 1.5 with fontSize=13.5 → 13.5 * 1.5 = 20.25pt; the
			// returned multiplier = 20.25 / 13.5 = 1.5.
			wantSize:       13.5,
			wantLineHeight: 1.5,
			wantFamily:     "sans-serif",
		},
		{
			name: "line-height as calc with em (length-form)",
			// parseLineHeight resolves a length-form calc by dividing
			// the resulting points by fontSize. calc(1.5em) at fontSize
			// 9pt = 13.5pt → multiplier = 13.5/9 = 1.5.
			input:          "12px/calc(1.5em) sans-serif",
			wantSize:       9,
			wantLineHeight: 1.5,
			wantFamily:     "sans-serif",
		},
		{
			name: "line-height as dimensionless calc (#275)",
			// Closes #275. A calc whose leaves are all dimensionless
			// numbers is itself a multiplier, not a length. Pre-fix
			// parseLineHeight passed `calc(1.2 * 1.5) = 1.8` through
			// `pts := l.toPoints(fontSize, fontSize)` (returning 1.8 for
			// the dimensionless case) and then divided by fontSize=9pt,
			// yielding 0.2 — a 9× compression. Post-fix
			// `cssLength.isDimensionless()` short-circuits the divide.
			input:          "12px/calc(1.2 * 1.5) sans-serif",
			wantSize:       9,
			wantLineHeight: 1.8,
			wantFamily:     "sans-serif",
		},
		{
			name:  "calc size with keyword line-height",
			input: "calc(1em + 2px)/normal sans-serif",
			// "normal" → multiplier 1.2.
			wantSize:       13.5,
			wantLineHeight: 1.2,
			wantFamily:     "sans-serif",
		},
		{
			name:  "calc size with quoted multi-word family",
			input: `calc(1em + 2px) "Times New Roman", serif`,
			// parseFontFamily picks the first family from the comma list,
			// strips quotes, lowercases.
			wantSize:   13.5,
			wantFamily: "times new roman",
		},
		{
			name:  "empty input keeps parentSize default",
			input: "",
			// Parser short-circuits and returns parentSize.
			wantSize: parentSize,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style, weight, size, lh, family :=
				parseFontShorthand(tt.input, parentSize)
			if style != tt.wantStyle {
				t.Errorf("style = %q, want %q", style, tt.wantStyle)
			}
			if weight != tt.wantWeight {
				t.Errorf("weight = %d, want %d", weight, tt.wantWeight)
			}
			if math.Abs(size-tt.wantSize) > 0.01 {
				t.Errorf("size = %.4f, want %.4f", size, tt.wantSize)
			}
			if math.Abs(lh-tt.wantLineHeight) > 0.01 {
				t.Errorf("lineHeight = %.4f, want %.4f", lh, tt.wantLineHeight)
			}
			if family != tt.wantFamily {
				t.Errorf("family = %q, want %q", family, tt.wantFamily)
			}
		})
	}
}

// TestStandardFontFamilyStillWorks verifies that standard font names
// (courier, times, helvetica) still resolve correctly after the refactor.
func TestStandardFontFamilyStillWorks(t *testing.T) {
	c := &converter{
		embeddedFonts: make(map[string]*font.EmbeddedFont),
	}

	tests := []struct {
		family string
		want   *font.Standard
	}{
		{"courier", font.Courier},
		{"courier new", font.Courier},
		{"monospace", font.Courier},
		{"times", font.TimesRoman},
		{"times new roman", font.TimesRoman},
		{"serif", font.TimesRoman},
		{"helvetica", font.Helvetica},
		{"arial", font.Helvetica},
		{"sans-serif", font.Helvetica},
	}
	for _, tt := range tests {
		style := defaultStyle()
		style.FontFamily = tt.family
		std, ef := c.resolveFontPair(style)
		if ef != nil {
			t.Errorf("family %q: expected nil embedded font", tt.family)
		}
		if std != tt.want {
			t.Errorf("family %q: got %v, want %v", tt.family, std.Name(), tt.want.Name())
		}
	}
}

func TestConvertFontFaceSrcParsing(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`url("font.ttf")`, "font.ttf"},
		{`url('font.ttf')`, "font.ttf"},
		{`url(font.ttf)`, "font.ttf"},
		{`url("path/to/font.woff") format("woff")`, "path/to/font.woff"},
		{`local("Arial")`, ""},
		{``, ""},
	}
	for _, tc := range tests {
		got := parseFontFaceSrc(tc.input)
		if got != tc.want {
			t.Errorf("parseFontFaceSrc(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// --- Feature 4: position: absolute/fixed ---

func TestConvertPositionAbsolute(t *testing.T) {
	htmlStr := `<div style="position: absolute; top: 50px; left: 100px; width: 200px"><p>Positioned</p></div><p>Normal flow</p>`
	result, err := ConvertFull(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Absolutes) == 0 {
		t.Fatal("expected at least 1 absolute item")
	}
	if result.Absolutes[0].Fixed {
		t.Error("position:absolute should not be Fixed")
	}
	if len(result.Elements) == 0 {
		t.Fatal("expected normal-flow elements")
	}
}

func TestConvertPositionFixed(t *testing.T) {
	htmlStr := `<div style="position: fixed; top: 0; left: 0"><p>Header</p></div>`
	result, err := ConvertFull(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Absolutes) == 0 {
		t.Fatal("expected absolute item for position:fixed")
	}
	if !result.Absolutes[0].Fixed {
		t.Error("position:fixed should have Fixed=true")
	}
}

func TestConvertPositionRelativePercentTop(t *testing.T) {
	// A relative top percentage must resolve against the page height, not 0.
	// Page height defaults to 792pt, so top:50% should yield a ~396pt offset.
	htmlStr := `<div style="position: relative; top: 50%"><p>Shifted</p></div>`
	result, err := ConvertFull(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Elements) == 0 {
		t.Fatal("expected a relatively positioned element")
	}
	div, ok := result.Elements[0].(*layout.Div)
	if !ok {
		t.Fatalf("expected *layout.Div, got %T", result.Elements[0])
	}
	plan := div.PlanLayout(layout.LayoutArea{Width: 468, Height: 720})
	if len(plan.Blocks) == 0 {
		t.Fatal("expected at least one placed block")
	}
	gotY := plan.Blocks[0].Y
	if gotY <= 0 {
		t.Fatalf("relative top:50%% should produce a non-zero vertical offset, got %v", gotY)
	}
	const want = 396.0 // 50% of default 792pt page height
	if gotY < want-1 || gotY > want+1 {
		t.Errorf("relative top:50%% offset = %v, want ~%v", gotY, want)
	}
}

func TestConvertPositionRelativePercentBottom(t *testing.T) {
	// A relative bottom percentage shifts the element upward (negative offset).
	htmlStr := `<div style="position: relative; bottom: 25%"><p>Shifted</p></div>`
	result, err := ConvertFull(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Elements) == 0 {
		t.Fatal("expected a relatively positioned element")
	}
	div, ok := result.Elements[0].(*layout.Div)
	if !ok {
		t.Fatalf("expected *layout.Div, got %T", result.Elements[0])
	}
	plan := div.PlanLayout(layout.LayoutArea{Width: 468, Height: 720})
	if len(plan.Blocks) == 0 {
		t.Fatal("expected at least one placed block")
	}
	gotY := plan.Blocks[0].Y
	const want = -198.0 // -25% of default 792pt page height
	if gotY > want+1 || gotY < want-1 {
		t.Errorf("relative bottom:25%% offset = %v, want ~%v", gotY, want)
	}
}

func TestConvertPositionStatic(t *testing.T) {
	htmlStr := `<div style="position: static"><p>Normal</p></div>`
	result, err := ConvertFull(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Absolutes) != 0 {
		t.Error("position:static should not produce absolute items")
	}
	if len(result.Elements) == 0 {
		t.Fatal("expected normal-flow elements")
	}
}

func TestConvertPositionCoordinates(t *testing.T) {
	htmlStr := `<div style="position: absolute; top: 100px; left: 50px; width: 200px"><p>At coordinates</p></div>`
	result, err := ConvertFull(htmlStr, &Options{PageWidth: 612, PageHeight: 792})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Absolutes) == 0 {
		t.Fatal("expected absolute item")
	}
	item := result.Absolutes[0]
	// left: 50px → 37.5pt
	if item.X < 37 || item.X > 38 {
		t.Errorf("expected X ~37.5, got %f", item.X)
	}
	// top: 100px → 75pt, PDF Y = 792 - 75 = 717
	if item.Y < 716 || item.Y > 718 {
		t.Errorf("expected Y ~717, got %f", item.Y)
	}
	// width: 200px → 150pt
	if item.Width < 149 || item.Width > 151 {
		t.Errorf("expected Width ~150, got %f", item.Width)
	}
}

func TestConvertBackwardCompatibility(t *testing.T) {
	htmlStr := `<div style="position: absolute; top: 10px"><p>Hidden</p></div><p>Visible</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected normal-flow elements")
	}
}

// --- Feature 1: ::before / ::after pseudo-elements ---

func TestPseudoElementBefore(t *testing.T) {
	htmlStr := `<style>p::before { content: "PREFIX "; }</style><p>Hello</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should produce elements: the ::before text prepended + the paragraph.
	if len(elems) == 0 {
		t.Fatal("expected at least one element")
	}
	// With ::before, the paragraph should have the prefix prepended as a separate element.
	// convertElement returns [beforeElem, paragraph].
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements (::before + paragraph), got %d", len(elems))
	}
}

func TestPseudoElementAfter(t *testing.T) {
	htmlStr := `<style>p::after { content: " SUFFIX"; }</style><p>Hello</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements (paragraph + ::after), got %d", len(elems))
	}
}

func TestPseudoElementContentNone(t *testing.T) {
	htmlStr := `<style>p::before { content: none; }</style><p>Hello</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	// content:none should not generate a pseudo-element.
	if len(elems) != 1 {
		t.Fatalf("expected 1 element (no ::before with content:none), got %d", len(elems))
	}
}

func TestPseudoElementBeforeAndAfter(t *testing.T) {
	htmlStr := `<style>
		p::before { content: "["; }
		p::after { content: "]"; }
	</style><p>Hello</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 3 {
		t.Fatalf("expected at least 3 elements (::before + paragraph + ::after), got %d", len(elems))
	}
}

// --- Feature 2: box-sizing: border-box ---

func TestBoxSizingBorderBox(t *testing.T) {
	htmlStr := `<div style="box-sizing: border-box; width: 200px; padding: 20px; border: 5px solid black"><p>Content</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least one element")
	}
	// The element should still render without errors.
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestBoxSizingContentBox(t *testing.T) {
	// content-box is the default — width should not subtract padding/border.
	htmlStr := `<div style="box-sizing: content-box; width: 200px; padding: 20px"><p>Content</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least one element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

// --- Feature 3: visibility: hidden ---

func TestVisibilityHidden(t *testing.T) {
	htmlStr := `<p style="visibility: hidden">Invisible</p><p>Visible</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Both elements should be present (visibility:hidden preserves space).
	if len(elems) != 2 {
		t.Fatalf("expected 2 elements (hidden + visible), got %d", len(elems))
	}
	// Both should take up space.
	for i, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
		if plan.Consumed <= 0 {
			t.Errorf("element %d: expected positive consumed, got %f", i, plan.Consumed)
		}
	}
}

func TestVisibilityHiddenVsDisplayNone(t *testing.T) {
	// display:none removes element entirely; visibility:hidden keeps it.
	htmlHidden := `<p style="visibility: hidden">Hidden</p>`
	htmlNone := `<p style="display: none">None</p>`

	elemsHidden, err := Convert(htmlHidden, nil)
	if err != nil {
		t.Fatal(err)
	}
	elemsNone, err := Convert(htmlNone, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(elemsHidden) != 1 {
		t.Fatalf("visibility:hidden should produce 1 element, got %d", len(elemsHidden))
	}
	if len(elemsNone) != 0 {
		t.Fatalf("display:none should produce 0 elements, got %d", len(elemsNone))
	}
}

func TestVisibilityInherited(t *testing.T) {
	htmlStr := `<div style="visibility: hidden"><p>Child</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The div and its child should still be present (visibility is inherited).
	if len(elems) == 0 {
		t.Fatal("expected elements (visibility:hidden preserves layout)")
	}
}

// --- Feature 4: min-height / max-height ---

func TestMinHeight(t *testing.T) {
	htmlStr := `<div style="min-height: 100px; background-color: #eee"><p>Short</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least one element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	// min-height: 100px → 75pt. The div should be at least 75pt tall.
	if plan.Consumed < 75 {
		t.Errorf("expected consumed >= 75pt (min-height: 100px), got %f", plan.Consumed)
	}
}

func TestMaxHeight(t *testing.T) {
	// Create a div with lots of content but max-height restricting it.
	htmlStr := `<div style="max-height: 50px; background-color: #eee"><p>Line 1</p><p>Line 2</p><p>Line 3</p><p>Line 4</p><p>Line 5</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least one element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	// max-height: 50px → 37.5pt. The consumed may include spaceBefore/After.
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

// --- Feature 5: Attribute selectors ---

func TestAttrSelectorPresence(t *testing.T) {
	htmlStr := `<style>[data-highlight] { font-weight: bold; }</style><p data-highlight>Bold</p><p>Normal</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

func TestAttrSelectorExact(t *testing.T) {
	htmlStr := `<style>[type="email"] { color: red; }</style><input type="email" value="test@example.com">`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

func TestAttrSelectorStartsWith(t *testing.T) {
	htmlStr := `<style>[href^="https"] { color: green; }</style><a href="https://example.com">Link</a>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

func TestAttrSelectorEndsWith(t *testing.T) {
	htmlStr := `<style>[href$=".pdf"] { color: blue; }</style><a href="doc.pdf">PDF</a>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

func TestAttrSelectorContains(t *testing.T) {
	htmlStr := `<style>[class*="warn"] { color: orange; }</style><p class="warning-msg">Warning</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

func TestAttrSelectorWordList(t *testing.T) {
	htmlStr := `<style>[class~="active"] { font-weight: bold; }</style><p class="btn active large">Active</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

func TestAttrSelectorDashPrefix(t *testing.T) {
	htmlStr := `<style>[lang|="en"] { font-style: italic; }</style><p lang="en-US">English</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

func TestAttrSelectorCaseInsensitive(t *testing.T) {
	// HTML attribute name is mixed case, CSS selector is lowercase.
	// The selector should still match (HTML attributes are case-insensitive).
	htmlStr := `<style>[data-value="test"] { font-weight: bold; }</style><p DATA-VALUE="test">Hello</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	// If the selector matched, the paragraph should use bold font.
	// This is a smoke test — the selector matching is the key behavior.
}

func TestAttrSelectorPresenceCaseInsensitive(t *testing.T) {
	// Presence selector [attr] should match regardless of case.
	htmlStr := `<style>[HIDDEN] { color: red; }</style><p hidden>Hidden text</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

func TestCSSEscapeSequences(t *testing.T) {
	// A class name containing a literal dot, escaped in the CSS selector.
	htmlStr := `<style>.my\.class { font-weight: bold; }</style><p class="my.class">Escaped</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements — escaped selector should match")
	}
}

func TestCSSEscapeInID(t *testing.T) {
	// An ID containing a literal colon, escaped in the CSS selector.
	htmlStr := `<style>#my\:id { color: red; }</style><p id="my:id">Escaped</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements — escaped ID selector should match")
	}
}

// --- Feature 6: :not() pseudo-class ---

func TestNotPseudoClass(t *testing.T) {
	htmlStr := `<style>p:not(.skip) { font-weight: bold; }</style><p class="skip">Skipped</p><p>Included</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 2 {
		t.Fatalf("expected 2 elements, got %d", len(elems))
	}
}

func TestNotPseudoClassTag(t *testing.T) {
	// :not(h1) should match paragraphs but not h1.
	htmlStr := `<style>:not(h1) { color: red; }</style><h1>Heading</h1><p>Paragraph</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements, got %d", len(elems))
	}
}

func TestNotPseudoClassID(t *testing.T) {
	htmlStr := `<style>p:not(#special) { font-style: italic; }</style><p id="special">Special</p><p>Normal</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 2 {
		t.Fatalf("expected 2 elements, got %d", len(elems))
	}
}

func TestNotPseudoClassAttribute(t *testing.T) {
	// :not([hidden]) should match elements WITHOUT the hidden attribute
	// and NOT match elements WITH the hidden attribute.
	// Verify by applying different colors to :not([hidden]) vs [hidden].
	htmlStr := `<style>
		p:not([hidden]) { color: red; }
		p[hidden] { color: blue; }
		.wrap { padding: 1px; }
	</style>
	<div class="wrap">
		<p>Visible</p>
		<p hidden>Hidden</p>
	</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	// Both paragraphs should render (hidden only affects browsers, not PDF).
	// The key assertion: :not([hidden]) matched the first <p> and [hidden]
	// matched the second. Verify layout succeeds with both rules applied.
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Status == layout.LayoutNothing {
		t.Error("unexpected LayoutNothing")
	}
	if len(plan.Blocks) == 0 {
		t.Error("expected blocks from div with two paragraphs")
	}
}

func TestNotWithAttrSiblingCombinator(t *testing.T) {
	// CSS pattern: > :not([hidden]) ~ :not([hidden]) { margin-top: ... }
	// Combines child combinator (>), :not() with attribute selector,
	// and general sibling combinator (~).
	// Used by CSS frameworks for spacing between visible siblings.
	htmlWith := `<style>
		.container > :not([hidden]) ~ :not([hidden]) { margin-top: 30px; }
		.container { padding: 1px; }
	</style>
	<div class="container">
		<p>First</p>
		<p>Second</p>
		<p>Third</p>
	</div>`

	htmlWithout := `<style>
		.container { padding: 1px; }
	</style>
	<div class="container">
		<p>First</p>
		<p>Second</p>
		<p>Third</p>
	</div>`

	elemsWith, err := Convert(htmlWith, nil)
	if err != nil {
		t.Fatal(err)
	}
	elemsWithout, err := Convert(htmlWithout, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elemsWith) == 0 || len(elemsWithout) == 0 {
		t.Fatal("expected elements from both versions")
	}

	planWith := elemsWith[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	planWithout := elemsWithout[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})

	// The version with margins should consume more height.
	if planWith.Consumed <= planWithout.Consumed {
		t.Errorf(":not([attr]) ~ :not([attr]) margin not applied: with=%f, without=%f",
			planWith.Consumed, planWithout.Consumed)
	}
}

func TestBoxShadow(t *testing.T) {
	htmlStr := `<div style="box-shadow: 5px 5px 10px 2px gray; padding: 10px"><p>Shadow box</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed height, got %f", plan.Consumed)
	}
}

func TestBoxShadowNone(t *testing.T) {
	htmlStr := `<div style="box-shadow: none; padding: 10px"><p>No shadow</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestTextOverflowEllipsis(t *testing.T) {
	htmlStr := `<div style="width: 50px; overflow: hidden"><p style="text-overflow: ellipsis; overflow: hidden">This is a very long paragraph that should be truncated with an ellipsis</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestOutline(t *testing.T) {
	htmlStr := `<div style="outline: 2px solid red; outline-offset: 3px; padding: 10px"><p>Outlined box</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestOutlineLonghand(t *testing.T) {
	htmlStr := `<div style="outline-width: 1px; outline-style: dashed; outline-color: blue; padding: 5px"><p>Dashed outline</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestCSSColumns(t *testing.T) {
	htmlStr := `<div style="column-count: 3; column-gap: 20px"><p>Column one</p><p>Column two</p><p>Column three</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed height, got %f", plan.Consumed)
	}
}

func TestColumnsShorthand(t *testing.T) {
	htmlStr := `<div style="columns: 2 15px"><p>First</p><p>Second</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

// TestColumnsShorthandWithCalc is a regression test for the same
// strings.Fields tokenization bug fixed for `flex:` (#236),
// `margin:`/`padding:` (#237), `font:` (#240), `border:` (#242),
// `background-size:` (#244), `@page size` (#247), `gap:` (#249),
// `border-radius:` (#252), `box-shadow:` (#254),
// `transform-origin:` (#257), and `border-spacing:` (#258) — applied
// here to the `columns` shorthand. Pre-fix `columns: calc(8px + 2px) 3`
// became 4 tokens ["calc(8px", "+", "2px)", "3"]; the calc fragments
// all failed both strconv.Atoi (not integers) and parseLength
// (unbalanced) → ColumnWidth stayed 0; only the integer "3" was
// recognized, so ColumnCount was set but the requested column-width
// was silently lost.
func TestColumnsShorthandWithCalc(t *testing.T) {
	tests := []struct {
		name      string
		val       string
		wantCount int
		wantWidth float64 // in pt
	}{
		{
			name: "calc width + integer count",
			val:  "calc(8px + 2px) 3",
			// 10px = 7.5pt; count = 3.
			wantWidth: 7.5, wantCount: 3,
		},
		{
			name: "integer count + calc width (order reversed)",
			val:  "3 calc(8px + 2px)",
			// Parser is order-agnostic.
			wantWidth: 7.5, wantCount: 3,
		},
		{
			name: "calc width only",
			val:  "calc(8px + 2px)",
			// Width set; count stays at zero default.
			wantWidth: 7.5, wantCount: 0,
		},
		{
			name: "count only (no width)",
			val:  "3",
			// Count set; width stays at zero default.
			wantWidth: 0, wantCount: 3,
		},
		{
			name: "min() width",
			val:  "min(8px, 16px) 2",
			// min picks 8px = 6pt; count = 2.
			wantWidth: 6, wantCount: 2,
		},
		{
			name: "max() width",
			val:  "max(8px, 16px) 4",
			// max picks 16px = 12pt; count = 4.
			wantWidth: 12, wantCount: 4,
		},
		{
			name: "clamp() width",
			val:  "clamp(4px, 8px, 16px) 3",
			// clamp middle = 8px = 6pt.
			wantWidth: 6, wantCount: 3,
		},
		{
			name: "calc with subtraction",
			val:  "calc(16px - 6px) 2",
			// 10px = 7.5pt.
			wantWidth: 7.5, wantCount: 2,
		},
		{
			name: "calc with multiplication",
			val:  "calc(5px * 2) 2",
			// 10px = 7.5pt.
			wantWidth: 7.5, wantCount: 2,
		},
		{
			name: "calc with division",
			val:  "calc(20px / 2) 2",
			// 10px = 7.5pt.
			wantWidth: 7.5, wantCount: 2,
		},
		{
			name:      "tab separator",
			val:       "calc(8px + 2px)\t3",
			wantWidth: 7.5, wantCount: 3,
		},
		{
			name:      "newline separator",
			val:       "calc(8px + 2px)\n3",
			wantWidth: 7.5, wantCount: 3,
		},
		{
			name: "extra unrecognized tokens silently ignored",
			// The loop iterates all tokens; non-integer non-length
			// tokens are simply skipped.
			val:       "auto 2 calc(8px + 2px) garbage",
			wantWidth: 7.5, wantCount: 2,
		},
		{
			name: "unbalanced calc paren: width stays 0, no crash",
			// splitTopLevelFields keeps the unbalanced calc + trailing
			// chars as one token; not an integer, parseLength fails
			// → token silently ignored. No crash.
			val:       "calc(8px + 2px",
			wantWidth: 0, wantCount: 0,
		},
		{
			name: "empty value: width and count stay 0",
			val:  "",
			// parts has length 0 → loop body never runs.
			wantWidth: 0, wantCount: 0,
		},
		{
			name:      "whitespace-only value: width and count stay 0",
			val:       "   \t\n   ",
			wantWidth: 0, wantCount: 0,
		},
		{
			name: "negative integer is rejected (count requires > 0)",
			// strconv.Atoi("-3") succeeds but the > 0 guard rejects it;
			// "-3" then falls to parseLength which interprets it as
			// "-3px" (bare number → px), so ColumnWidth = -3px = -2.25pt.
			// Documents the existing contract; nobody should write this.
			val:       "-3 4",
			wantWidth: -2.25, wantCount: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &converter{}
			style := &computedStyle{FontSize: 12}
			c.applyProperty("columns", tt.val, style)
			if math.Abs(style.ColumnWidth-tt.wantWidth) > 0.01 {
				t.Errorf("applyProperty(%q): ColumnWidth = %.4f, want %.4f",
					tt.val, style.ColumnWidth, tt.wantWidth)
			}
			if style.ColumnCount != tt.wantCount {
				t.Errorf("applyProperty(%q): ColumnCount = %d, want %d",
					tt.val, style.ColumnCount, tt.wantCount)
			}
		})
	}
}

func TestCSSColumnSpanAll(t *testing.T) {
	// A multi-column container with a column-span: all child should split
	// into three siblings: a Columns segment for the content before the
	// spanning child, the spanning child itself as a full-width element,
	// and a second Columns segment for the content after.
	htmlStr := `<div style="column-count: 2; column-gap: 20px;">` +
		`<p>Before paragraph one.</p>` +
		`<p>Before paragraph two.</p>` +
		`<h3 style="column-span: all">Spanning heading</h3>` +
		`<p>After paragraph one.</p>` +
		`<p>After paragraph two.</p>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 3 {
		t.Fatalf("expected 3 segmented elements (columns, span, columns), got %d", len(elems))
	}
	if _, ok := elems[0].(*layout.Columns); !ok {
		t.Errorf("expected elems[0] to be *layout.Columns, got %T", elems[0])
	}
	if _, ok := elems[2].(*layout.Columns); !ok {
		t.Errorf("expected elems[2] to be *layout.Columns, got %T", elems[2])
	}
	// The middle element must NOT be a Columns — wrapping the spanning
	// element in a single-item Columns would mean the column-span is
	// silently being ignored.
	if _, ok := elems[1].(*layout.Columns); ok {
		t.Errorf("expected elems[1] to be a full-width spanning element, got *layout.Columns")
	}
	// And it must not be nil — a regression that drops the spanning
	// child entirely should not slip through.
	if elems[1] == nil {
		t.Errorf("expected elems[1] to be a non-nil spanning element")
	}
	// Sanity: everything lays out.
	for i, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 600, Height: 2000})
		if plan.Status != layout.LayoutFull {
			t.Errorf("elems[%d]: expected LayoutFull, got %v", i, plan.Status)
		}
	}
}

func TestCSSColumnSpanAllLeading(t *testing.T) {
	// Spanning element as the first child: there should be no leading
	// Columns segment; the result is the span followed by one Columns.
	htmlStr := `<div style="column-count: 2">` +
		`<h3 style="column-span: all">Heading</h3>` +
		`<p>One.</p><p>Two.</p>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(elems))
	}
	// First element must NOT be a Columns — a regression that wraps
	// the spanning heading in a single-item Columns would still produce
	// 2 elements but be incorrect.
	if _, ok := elems[0].(*layout.Columns); ok {
		t.Errorf("expected elems[0] to be a full-width spanning element, got *layout.Columns")
	}
	if _, ok := elems[1].(*layout.Columns); !ok {
		t.Errorf("expected trailing Columns segment, got %T", elems[1])
	}
}

func TestCSSColumnSpanAllConsecutive(t *testing.T) {
	// Two consecutive column-span: all children should produce two
	// adjacent spanning elements with no empty Columns segment between
	// them. This exercises flushSegment being called when the segment
	// is already empty.
	htmlStr := `<div style="column-count: 2">` +
		`<p>Before.</p>` +
		`<h3 style="column-span: all">First span</h3>` +
		`<h3 style="column-span: all">Second span</h3>` +
		`<p>After.</p>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Expected sequence: Columns, span1, span2, Columns. No empty
	// Columns between the two spans.
	if len(elems) != 4 {
		t.Fatalf("expected 4 elements (cols, span, span, cols), got %d", len(elems))
	}
	if _, ok := elems[0].(*layout.Columns); !ok {
		t.Errorf("expected elems[0] Columns, got %T", elems[0])
	}
	if _, ok := elems[1].(*layout.Columns); ok {
		t.Errorf("elems[1] should be a spanning element, got *layout.Columns")
	}
	if _, ok := elems[2].(*layout.Columns); ok {
		t.Errorf("elems[2] should be a spanning element, got *layout.Columns")
	}
	if _, ok := elems[3].(*layout.Columns); !ok {
		t.Errorf("expected elems[3] Columns, got %T", elems[3])
	}
}

func TestCSSColumnSpanAllConsecutiveAtBoundary(t *testing.T) {
	// Two consecutive column-span: all children at the very top of the
	// container, with no leading content. There should be no leading
	// empty Columns.
	htmlStr := `<div style="column-count: 2">` +
		`<h3 style="column-span: all">First</h3>` +
		`<h3 style="column-span: all">Second</h3>` +
		`<p>Trailing.</p>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 3 {
		t.Fatalf("expected 3 elements (span, span, cols), got %d", len(elems))
	}
	if _, ok := elems[0].(*layout.Columns); ok {
		t.Errorf("elems[0] should be a spanning element, got *layout.Columns")
	}
	if _, ok := elems[1].(*layout.Columns); ok {
		t.Errorf("elems[1] should be a spanning element, got *layout.Columns")
	}
	if _, ok := elems[2].(*layout.Columns); !ok {
		t.Errorf("expected elems[2] Columns, got %T", elems[2])
	}
}

func TestCSSColumnSpanAllAutoColumnCount(t *testing.T) {
	// column-span: all should also work when the column count is
	// derived from column-width rather than declared explicitly.
	htmlStr := `<div style="column-width: 80px">` +
		`<p>One.</p><p>Two.</p>` +
		`<h3 style="column-span: all">Span</h3>` +
		`<p>Three.</p>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elems))
	}
	if _, ok := elems[1].(*layout.Columns); ok {
		t.Errorf("elems[1] should be the spanning element, got *layout.Columns")
	}
}

func TestCSSColumnSpanAllOnNonDirectDescendant(t *testing.T) {
	// Per spec, column-span: all only takes effect on direct children
	// of the multicol container. A column-span declaration on a deeper
	// descendant should be ignored — the wrapper section becomes a
	// regular member of a single Columns segment.
	htmlStr := `<div style="column-count: 2">` +
		`<section>` +
		`<h3 style="column-span: all">Buried heading</h3>` +
		`<p>Body.</p>` +
		`</section>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The whole flow should be a single Columns — no segmentation,
	// because the spanning child is not a direct child of the multicol
	// container.
	if len(elems) != 1 {
		t.Fatalf("expected 1 element (single Columns), got %d", len(elems))
	}
	if _, ok := elems[0].(*layout.Columns); !ok {
		t.Errorf("expected single *layout.Columns, got %T", elems[0])
	}
}

func TestCSSColumnSpanAllDisplayNone(t *testing.T) {
	// A column-span: all child that is also display: none renders to
	// nothing and must NOT cause a spurious segment flush. The result
	// should be a single Columns containing the surrounding paragraphs.
	htmlStr := `<div style="column-count: 2">` +
		`<p>One.</p>` +
		`<h3 style="column-span: all; display: none">Hidden span</h3>` +
		`<p>Two.</p>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element (single Columns), got %d", len(elems))
	}
	if _, ok := elems[0].(*layout.Columns); !ok {
		t.Errorf("expected *layout.Columns, got %T", elems[0])
	}
}

func TestCSSColumnSpanAllWithColumnRule(t *testing.T) {
	// Smoke test: column-rule on a multicol container with a spanning
	// child should not error. Each segment carries the rule; the
	// spanning element does not.
	htmlStr := `<div style="column-count: 2; column-rule: 1px solid gray">` +
		`<p>Before.</p>` +
		`<h3 style="column-span: all">Span</h3>` +
		`<p>After.</p>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elems))
	}
	for i, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 600, Height: 2000})
		if plan.Status != layout.LayoutFull {
			t.Errorf("elems[%d]: expected LayoutFull, got %v", i, plan.Status)
		}
	}
}

func TestCSSColumnSpanEmptyMulticolDoesNotDoubleWalk(t *testing.T) {
	// Regression for the empty fall-through bug: when a multi-column
	// container's children all render to nothing in normal flow, the
	// converter must not re-walk them via walkChildren after
	// buildMulticolSegments has already run convertNode on each one.
	// Re-walking would double-fire side effects on c.absolutes,
	// counters, fonts, etc.
	//
	// Here the only child is position:absolute, which appends to
	// c.absolutes once and returns nil from convertElement. A re-walk
	// would push the same element a second time.
	htmlStr := `<div style="column-count: 2; border: 1px solid">` +
		`<div style="position: absolute; top: 10px; left: 10px; width: 50px">abs</div>` +
		`</div>`
	result, err := ConvertFull(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Absolutes) != 1 {
		t.Errorf("expected exactly 1 absolute element, got %d (a re-walk would produce 2)", len(result.Absolutes))
	}
	// The container has a border, so a single empty Div should be
	// returned in the normal flow.
	if len(result.Elements) != 1 {
		t.Errorf("expected 1 flow element (the bordered empty Div), got %d", len(result.Elements))
	}
}

// TestCSSColumnsSequentialBalanced verifies that multi-column children are
// distributed by measured height (column-fill: balance) rather than
// round-robin by index. Regression test for
// https://github.com/carlos7ags/folio/issues/145.
func TestCSSColumnsSequentialBalanced(t *testing.T) {
	// Three paragraphs with very different lengths. With round-robin
	// the long paragraph and a short paragraph share column 0 while
	// column 1 gets only one short paragraph — wildly unbalanced.
	// With balanced distribution the two short paragraphs should share
	// one column and the long paragraph should get the other, producing
	// much more even heights.
	htmlStr := `<div style="column-count: 2; column-gap: 12px">` +
		`<p>Short one.</p>` +
		`<p>Short two.</p>` +
		`<p>This paragraph is intentionally much longer than the other ` +
		`two so that the balanced algorithm places it alone in one ` +
		`column while the shorter paragraphs share the other, ` +
		`producing approximately equal column heights instead of the ` +
		`wildly unbalanced round-robin result.</p>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}

	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 300, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Fatalf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Fatal("expected positive consumed height")
	}

	// With balanced distribution, both columns contribute content.
	// Verify the top-level block has children from both columns
	// (children at X=0 AND children at X>0).
	if len(plan.Blocks) == 0 || len(plan.Blocks[0].Children) == 0 {
		t.Fatal("expected children in plan blocks")
	}
	hasLeft, hasRight := false, false
	for _, child := range plan.Blocks[0].Children {
		if child.X < 1 {
			hasLeft = true
		} else {
			hasRight = true
		}
	}
	if !hasLeft || !hasRight {
		t.Error("expected content in both columns; one column is empty")
	}
}

func TestCSSColumnSpanAllTrailing(t *testing.T) {
	// Spanning element as the last child: there should be no trailing
	// Columns segment.
	htmlStr := `<div style="column-count: 2">` +
		`<p>One.</p><p>Two.</p>` +
		`<h3 style="column-span: all">Heading</h3>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(elems))
	}
	if _, ok := elems[0].(*layout.Columns); !ok {
		t.Errorf("expected leading Columns segment, got %T", elems[0])
	}
	// Trailing element must NOT be a Columns — symmetric to the
	// leading case.
	if _, ok := elems[1].(*layout.Columns); ok {
		t.Errorf("expected elems[1] to be a full-width spanning element, got *layout.Columns")
	}
}

func TestTextDecorationColorAndStyle(t *testing.T) {
	htmlStr := `<p style="text-decoration: underline; text-decoration-color: red; text-decoration-style: dashed">Decorated text</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestTextDecorationStyleWavy(t *testing.T) {
	htmlStr := `<p style="text-decoration: underline; text-decoration-style: wavy">Wavy underline</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestTextDecorationStyleDouble(t *testing.T) {
	htmlStr := `<p style="text-decoration: underline; text-decoration-style: double; text-decoration-color: blue">Double underline</p>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
}

func TestParseBoxShadow(t *testing.T) {
	tests := []struct {
		input   string
		wantNil bool
	}{
		{"5px 5px 10px 2px red", false},
		{"2px 2px gray", false},
		{"0px 0px 5px black", false},
		{"none", true},
		{"", true},
	}
	for _, tt := range tests {
		bs := parseBoxShadow(tt.input, 12)
		if tt.wantNil && bs != nil {
			t.Errorf("parseBoxShadow(%q): expected nil, got %+v", tt.input, bs)
		}
		if !tt.wantNil && bs == nil {
			t.Errorf("parseBoxShadow(%q): expected non-nil", tt.input)
		}
	}
}

func TestTransformRotate(t *testing.T) {
	h := `<div style="transform: rotate(45deg); padding: 10px;"><p>Rotated</p></div>`
	elems, err := Convert(h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed, got %f", plan.Consumed)
	}
}

func TestTransformScale(t *testing.T) {
	h := `<div style="transform: scale(1.5); padding: 5px;"><p>Scaled</p></div>`
	elems, err := Convert(h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestTransformTranslate(t *testing.T) {
	h := `<div style="transform: translate(10px, 20px); padding: 5px;"><p>Translated</p></div>`
	elems, err := Convert(h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestTransformMultiple(t *testing.T) {
	h := `<div style="transform: rotate(45deg) scale(0.8); padding: 5px;"><p>Multi</p></div>`
	elems, err := Convert(h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestTransformOrigin(t *testing.T) {
	h := `<div style="transform: rotate(30deg); transform-origin: top left; padding: 5px;"><p>Origin</p></div>`
	elems, err := Convert(h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

// TestParseTransformOriginWithCalc is a regression test for the same
// strings.Fields tokenization bug fixed for `flex:` (#236),
// `margin:`/`padding:` (#237), `font:` (#240), `border:` (#242),
// `background-size:` (#244), `@page size` (#247), `gap:` (#249),
// `border-radius:` (#252), and `box-shadow:` (#254) — applied here
// to `transform-origin`. Pre-fix `transform-origin: calc(50% - 10px) center`
// became 5 tokens ["calc(50%", "-", "10px)", "center"]; parts[0] =
// "calc(50%" was an unbalanced calc → parseLength returned nil → the
// fallback in resolveOriginComponent used dimension/2, silently losing
// the requested origin.
func TestParseTransformOriginWithCalc(t *testing.T) {
	const w, h, fs = 200.0, 100.0, 12.0
	tests := []struct {
		name  string
		val   string
		wantX float64
		wantY float64
	}{
		{
			name: "calc x, keyword y",
			val:  "calc(50% - 10px) center",
			// 50% × 200 - 10px(=7.5pt) = 92.5pt; center of 100 = 50.
			wantX: 92.5, wantY: 50,
		},
		{
			name: "keyword x, calc y",
			val:  "left calc(50% + 10px)",
			// left = 0; 50% × 100 + 10px(=7.5pt) = 57.5pt.
			wantX: 0, wantY: 57.5,
		},
		{
			name: "calc on both axes",
			val:  "calc(50% - 10px) calc(25% + 5px)",
			// X: 100 - 7.5 = 92.5pt; Y: 25 + 3.75 = 28.75pt.
			wantX: 92.5, wantY: 28.75,
		},
		{
			name: "single calc value (Y defaults to center)",
			val:  "calc(50% - 10px)",
			// X = 92.5pt; Y = h/2 = 50pt.
			wantX: 92.5, wantY: 50,
		},
		{
			name: "calc with addition",
			val:  "calc(10px + 10px) calc(20px + 0px)",
			// X = 20px = 15pt; Y = 20px = 15pt.
			wantX: 15, wantY: 15,
		},
		{
			name:  "calc with multiplication",
			val:   "calc(10px * 2) calc(20px * 1)",
			wantX: 15, wantY: 15,
		},
		{
			name: "calc with division",
			val:  "calc(40px / 2) calc(20px / 2)",
			// X = 20px = 15pt; Y = 10px = 7.5pt.
			wantX: 15, wantY: 7.5,
		},
		{
			name: "min() x, max() y",
			val:  "min(20px, 40px) max(10px, 30px)",
			// X = min(20px, 40px) = 15pt; Y = max(10px, 30px) = 22.5pt.
			wantX: 15, wantY: 22.5,
		},
		{
			name: "clamp() x",
			val:  "clamp(10px, 20px, 40px) center",
			// X = clamp middle = 20px = 15pt; Y = h/2 = 50.
			wantX: 15, wantY: 50,
		},
		{
			name:  "tab separator",
			val:   "calc(50% - 10px)\tcenter",
			wantX: 92.5, wantY: 50,
		},
		{
			name:  "newline separator",
			val:   "calc(50% - 10px)\ncenter",
			wantX: 92.5, wantY: 50,
		},
		{
			name: "plain percent x, plain percent y (no calc, regression-safe)",
			val:  "50% 25%",
			// X = 100; Y = 25.
			wantX: 100, wantY: 25,
		},
		{
			name: "empty value defaults to center center",
			val:  "",
			// X = w/2 = 100; Y = h/2 = 50.
			wantX: 100, wantY: 50,
		},
		{
			name: "unbalanced calc paren falls back to dimension/2",
			// resolveOriginComponent's parseLength fails → returns
			// dimension/2 as the fallback. splitTopLevelFields keeps
			// the unclosed-paren run as one token, so len(parts)==1 →
			// Y also defaults to height/2.
			val:   "calc(50% - 10px",
			wantX: 100, wantY: 50,
		},
		{
			name: "keyword-only: top left",
			// Locks the keyword-resolution branch alongside the calc
			// branch. left → 0; top → 0.
			val:   "top left",
			wantX: 0, wantY: 0,
		},
		{
			name: "keyword-only: bottom right",
			// right → w (200); bottom → h (100).
			val:   "right bottom",
			wantX: 200, wantY: 100,
		},
		{
			name:  "keyword-only: center center",
			val:   "center center",
			wantX: 100, wantY: 50,
		},
		{
			name: "3-value form: Z component is silently dropped",
			// CSS 3D transform-origin allows a Z component, but
			// parseTransformOrigin only reads parts[0] and parts[1].
			// Locks current contract: extras ignored, X/Y still
			// parsed correctly.
			val:   "50% 50% 10px",
			wantX: 100, wantY: 50,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotX, gotY := parseTransformOrigin(tt.val, w, h, fs)
			if math.Abs(gotX-tt.wantX) > 0.01 {
				t.Errorf("parseTransformOrigin(%q): X = %.4f, want %.4f",
					tt.val, gotX, tt.wantX)
			}
			if math.Abs(gotY-tt.wantY) > 0.01 {
				t.Errorf("parseTransformOrigin(%q): Y = %.4f, want %.4f",
					tt.val, gotY, tt.wantY)
			}
		})
	}
}

func TestTransformNone(t *testing.T) {
	h := `<div style="transform: none; padding: 5px;"><p>No transform</p></div>`
	elems, err := Convert(h, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

// TestParseTransformWithCalc closes #264. Pre-fix `parseTransform` had
// two paren-blind bugs that compounded:
//
//  1. The outer function-call extractor used
//     `strings.Index(val[parenIdx:], ")")` which finds the FIRST close
//     paren. For `translate(calc(50% - 10px), 10px)` it truncated the
//     args at the calc's inner `)`, so argsStr became `"calc(50% - 10px"`
//     (unbalanced) and the trailing `, 10px)` was mis-parsed as a
//     separate (nameless) function call.
//
//  2. The inner split did `strings.ReplaceAll(args, ",", " ")` followed
//     by `strings.Fields`. Even with the outer fix, the destructive
//     comma replacement also stripped commas from nested function calls
//     like `min(10px, 20px)`, breaking parseLength on the nested arg.
//
// Both are fixed by walking depth-aware: the outer loop finds the
// matching close paren respecting nesting, and the inner split uses
// splitTopLevelCommas (with a splitTopLevelFields fallback for the
// legacy space-separated form like `translate(10px 20px)`).
//
// parseLengthPx already routes through parseLength (calc-aware), so
// once the args are extracted intact, calc inside translate/translateX/
// translateY resolves correctly. parseAngle and parseNumericVal don't
// understand calc — calc inside rotate/scale/skew is therefore still
// silently 0; that gap is separate from the structural #264 fix.
func TestParseTransformWithCalc(t *testing.T) {
	tests := []struct {
		name    string
		val     string
		wantOps []layout.TransformOp
	}{
		{
			name: "translate with calc x and plain y",
			val:  "translate(calc(50% - 10px), 10px)",
			// CSS px → pt: 10px = 7.5pt. calc(50% - 10px) at relativeTo=0
			// resolves the % to 0, then - 7.5pt → -7.5pt. Y is negated
			// (CSS y-axis is flipped vs PDF).
			wantOps: []layout.TransformOp{
				{Type: "translate", Values: [2]float64{-7.5, -7.5}},
			},
		},
		{
			name: "translate with calc on both axes",
			val:  "translate(calc(20px + 10px), calc(40px - 10px))",
			// 30px = 22.5pt; 30px = 22.5pt; y negated.
			wantOps: []layout.TransformOp{
				{Type: "translate", Values: [2]float64{22.5, -22.5}},
			},
		},
		{
			name: "translate with min() and max()",
			val:  "translate(min(20px, 40px), max(10px, 20px))",
			// min picks 20px = 15pt; max picks 20px = 15pt; y negated.
			wantOps: []layout.TransformOp{
				{Type: "translate", Values: [2]float64{15, -15}},
			},
		},
		{
			name: "translate with nested function (preserves inner commas)",
			val:  "translate(min(10px, 20px), 0)",
			// Pre-fix the destructive comma→space replacement would have
			// turned this into `min(10px  20px)` and broken parseLength.
			// Post-fix nested commas survive.
			wantOps: []layout.TransformOp{
				{Type: "translate", Values: [2]float64{7.5, 0}},
			},
		},
		{
			name: "translateX with calc",
			val:  "translateX(calc(20px + 10px))",
			wantOps: []layout.TransformOp{
				{Type: "translate", Values: [2]float64{22.5, 0}},
			},
		},
		{
			name: "translateY with calc",
			val:  "translateY(calc(20px + 10px))",
			// Y is negated for CSS↔PDF axis flip.
			wantOps: []layout.TransformOp{
				{Type: "translate", Values: [2]float64{0, -22.5}},
			},
		},
		{
			name: "two functions in sequence: rotate + translate(calc)",
			val:  "rotate(45deg) translate(calc(10px + 10px), 0)",
			// Pre-fix the outer extractor would truncate the calc, leaving
			// dangling `, 0)` mis-parsed. Post-fix both functions parse.
			wantOps: []layout.TransformOp{
				{Type: "rotate", Values: [2]float64{45, 0}},
				{Type: "translate", Values: [2]float64{15, 0}},
			},
		},
		{
			name: "legacy space-separated args still work",
			// Browsers accept this form; the splitTopLevelFields fallback
			// kicks in when splitTopLevelCommas yields a single token.
			val: "translate(10px 20px)",
			wantOps: []layout.TransformOp{
				{Type: "translate", Values: [2]float64{7.5, -15}},
			},
		},
		{
			name: "single-arg translate (Y defaults to 0)",
			val:  "translate(calc(10px + 5px))",
			wantOps: []layout.TransformOp{
				{Type: "translate", Values: [2]float64{11.25, 0}},
			},
		},
		{
			name: "translate with clamp() — stresses splitTopLevelCommas",
			// clamp() has TWO internal commas. Pre-fix the destructive
			// `,` → ` ` replacement at the call site would have flattened
			// these to spaces, breaking parseLength on the clamp arg.
			// Post-fix splitTopLevelCommas keeps the whole clamp as a
			// single top-level token.
			val: "translate(clamp(5px, 10px, 20px), 0)",
			// clamp middle = 10px = 7.5pt; second arg = 0.
			wantOps: []layout.TransformOp{
				{Type: "translate", Values: [2]float64{7.5, 0}},
			},
		},
		{
			name: "translateX with calc multiplication",
			val:  "translateX(calc(10px * 2))",
			// 20px = 15pt.
			wantOps: []layout.TransformOp{
				{Type: "translate", Values: [2]float64{15, 0}},
			},
		},
		{
			name: "translateY with calc division",
			val:  "translateY(calc(40px / 2))",
			// 20px = 15pt; y negated.
			wantOps: []layout.TransformOp{
				{Type: "translate", Values: [2]float64{0, -15}},
			},
		},
		{
			name: "deep nesting (3 levels): translate(min(calc(...), ...), ...)",
			// Exercises the depth counter past 2 — outer translate paren,
			// min paren, calc paren. All three must close at the right
			// time for closeAbs to land on the outer translate's `)`.
			val: "translate(min(calc(10px + 0px), 20px), 0)",
			// min picks 10px = 7.5pt; second arg = 0.
			wantOps: []layout.TransformOp{
				{Type: "translate", Values: [2]float64{7.5, 0}},
			},
		},
		{
			name: "rotate(calc(45deg + 45deg)) resolves to 90deg",
			// Closes #274. Pre-fix parseAngle was calc-blind and this
			// resolved to 0; the new calc-aware parser walks the
			// expression tree with angle-native leaves.
			val: "rotate(calc(45deg + 45deg))",
			wantOps: []layout.TransformOp{
				{Type: "rotate", Values: [2]float64{90, 0}},
			},
		},
		{
			name: "unbalanced calc paren: function silently dropped, no crash",
			// The outer extractor's depth tracking never returns to 0;
			// closeAbs stays -1; the loop breaks. No ops produced.
			val:     "translate(calc(10px + 5px, 0)",
			wantOps: nil,
		},
		{
			name: "whitespace-only argument: emits {0,0} translate",
			// Documents current behavior: parts after splitTopLevelCommas
			// are [""], fallback to splitTopLevelFields("   ") returns
			// nil → len(parts)==0 → no branch taken → no op emitted.
			val:     "translate(   )",
			wantOps: nil,
		},
		{
			name:    "empty input produces no ops",
			val:     "",
			wantOps: nil,
		},
		{
			name:    "none keyword produces no ops",
			val:     "none",
			wantOps: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTransform(tt.val)
			if len(got) != len(tt.wantOps) {
				t.Fatalf("parseTransform(%q): got %d ops, want %d (%+v vs %+v)",
					tt.val, len(got), len(tt.wantOps), got, tt.wantOps)
			}
			for i, op := range got {
				if op.Type != tt.wantOps[i].Type {
					t.Errorf("ops[%d].Type = %q, want %q", i, op.Type, tt.wantOps[i].Type)
				}
				if math.Abs(op.Values[0]-tt.wantOps[i].Values[0]) > 0.01 ||
					math.Abs(op.Values[1]-tt.wantOps[i].Values[1]) > 0.01 {
					t.Errorf("ops[%d].Values = %v, want %v", i, op.Values, tt.wantOps[i].Values)
				}
			}
		})
	}
}

// --- CSS Grid tests ---

func TestCSSGridBasic(t *testing.T) {
	htmlStr := `<div style="display: grid; grid-template-columns: 1fr 1fr 1fr">
		<div><p>Cell 1</p></div>
		<div><p>Cell 2</p></div>
		<div><p>Cell 3</p></div>
	</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 grid element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed height, got %f", plan.Consumed)
	}
	// Should have a single container block with children.
	if len(plan.Blocks) != 1 {
		t.Fatalf("expected 1 container block, got %d", len(plan.Blocks))
	}
	if len(plan.Blocks[0].Children) != 3 {
		t.Errorf("expected 3 child blocks, got %d", len(plan.Blocks[0].Children))
	}
}

func TestCSSGridFixedAndFr(t *testing.T) {
	htmlStr := `<div style="display: grid; grid-template-columns: 200px 1fr 2fr">
		<div><p>Fixed</p></div>
		<div><p>Small flex</p></div>
		<div><p>Large flex</p></div>
	</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 grid element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed height, got %f", plan.Consumed)
	}
	// The first column should be 200px (150pt), and the remaining space
	// split 1:2 between the other two columns.
	if len(plan.Blocks) != 1 || len(plan.Blocks[0].Children) != 3 {
		t.Fatalf("expected 1 container with 3 children, got %d blocks", len(plan.Blocks))
	}
}

func TestCSSGridGap(t *testing.T) {
	htmlStr := `<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 20px">
		<div><p>A</p></div>
		<div><p>B</p></div>
		<div><p>C</p></div>
		<div><p>D</p></div>
	</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	// 4 items in a 2-column grid = 2 rows. The gap should add space.
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed height, got %f", plan.Consumed)
	}
	// Verify we have 4 child blocks.
	if len(plan.Blocks) == 1 && len(plan.Blocks[0].Children) != 4 {
		t.Errorf("expected 4 child blocks, got %d", len(plan.Blocks[0].Children))
	}
}

func TestCSSGridExplicitPlacement(t *testing.T) {
	htmlStr := `<div style="display: grid; grid-template-columns: 1fr 1fr 1fr">
		<div style="grid-column: 1 / 3"><p>Spans 2 cols</p></div>
		<div><p>Cell 2</p></div>
		<div><p>Cell 3</p></div>
	</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Errorf("expected positive consumed height, got %f", plan.Consumed)
	}
}

func TestCSSGridRepeat(t *testing.T) {
	htmlStr := `<div style="display: grid; grid-template-columns: repeat(4, 1fr)">
		<div><p>1</p></div>
		<div><p>2</p></div>
		<div><p>3</p></div>
		<div><p>4</p></div>
	</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 800, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	// 4 items in 4 columns = 1 row.
	if len(plan.Blocks) == 1 && len(plan.Blocks[0].Children) != 4 {
		t.Errorf("expected 4 child blocks (single row), got %d", len(plan.Blocks[0].Children))
	}
}

func TestCSSGridAutoRows(t *testing.T) {
	// Grid with columns defined but no explicit row template.
	// Rows should be auto-sized based on content.
	htmlStr := `<div style="display: grid; grid-template-columns: 1fr 1fr">
		<div><p>Row 1, Col 1</p></div>
		<div><p>Row 1, Col 2</p></div>
		<div><p>Row 2, Col 1</p></div>
		<div><p>Row 2, Col 2</p></div>
		<div><p>Row 3, Col 1</p></div>
	</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	// 5 items in 2-column grid = 3 rows (last row has 1 item).
	if len(plan.Blocks) == 1 && len(plan.Blocks[0].Children) != 5 {
		t.Errorf("expected 5 child blocks, got %d", len(plan.Blocks[0].Children))
	}
}

func TestCSSGridExplicitHeightIsHonored(t *testing.T) {
	// Regression for issue #129: a grid container with height: 120px
	// must be forced to that height, not grow with content. 120px = 90pt.
	htmlStr := `<div style="display: grid; grid-template-columns: 1fr 1fr; height: 120px">` +
		`<div>A</div><div>B</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	if len(plan.Blocks) == 0 {
		t.Fatal("no blocks")
	}
	h := plan.Blocks[0].Height
	if h < 88 || h > 92 {
		t.Errorf("grid height = %.2f, want ~90 (120px honored)", h)
	}
}

func TestCSSGridJustifyItemsCenter(t *testing.T) {
	// Regression for issue #129: justify-items: center on a grid
	// container should shrink items horizontally to their content
	// width and center them within the cell. The child divs use
	// border + padding so they are real Divs (not unwrapped to plain
	// paragraphs), which means they would otherwise fill the cell
	// width at layout time.
	htmlStr := `<div style="display: grid; grid-template-columns: 1fr 1fr; justify-items: center">` +
		`<div style="border: 1px solid #000; padding: 2px 6px">A</div>` +
		`<div style="border: 1px solid #000; padding: 2px 6px">B</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	gridBlock := plan.Blocks[0]
	if len(gridBlock.Children) < 2 {
		t.Fatalf("expected 2 children, got %d", len(gridBlock.Children))
	}
	// Each cell is 200pt wide. Without the fix, bordered Divs fill
	// the full 200pt. With the fix, they take their content width
	// (well under 100pt) and are centered within the 200pt cell.
	for i, child := range gridBlock.Children {
		if child.Width > 100 {
			t.Errorf("child %d: width = %.2f, want < 100 (shrunk to content)", i, child.Width)
		}
		cellLeftEdge := float64(i) * 200
		inCellX := child.X - cellLeftEdge
		if inCellX < 20 {
			t.Errorf("child %d: X = %.2f, cell-relative X = %.2f, want > 20 (centered horizontally)", i, child.X, inCellX)
		}
	}
}

func TestCSSGridAlignItemsCenterWithHeight(t *testing.T) {
	// Regression for issue #129: align-items: center with a definite
	// container height should distribute rows to fill the container
	// and vertically center items within their cells.
	// Grid inner height = 90pt. 1 row stretched to 90pt. Text content
	// ≈ 14.4pt (12pt font × 1.2 line height). Expected Y ≈ (90 −
	// 14.4) / 2 = 37.8pt. Tight ±3pt band catches off-by-one
	// regressions.
	htmlStr := `<div style="display: grid; grid-template-columns: 1fr 1fr; height: 120px; align-items: center">` +
		`<div>A</div><div>B</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	gridBlock := plan.Blocks[0]
	if len(gridBlock.Children) < 2 {
		t.Fatalf("expected 2 children, got %d", len(gridBlock.Children))
	}
	for i, child := range gridBlock.Children {
		if child.Y < 35 || child.Y > 40 {
			t.Errorf("child %d: Y = %.2f, want 35..40 (centered, ~37.8)", i, child.Y)
		}
	}
}

func TestCSSGridJustifyItemsEnd(t *testing.T) {
	// justify-items: end should place items at the right edge of the
	// cell, shrunk to their content width.
	htmlStr := `<div style="display: grid; grid-template-columns: 1fr 1fr; justify-items: end">` +
		`<div style="border: 1px solid #000; padding: 2px 6px">A</div>` +
		`<div style="border: 1px solid #000; padding: 2px 6px">B</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	gridBlock := plan.Blocks[0]
	// Each cell is 200pt wide. With justify-items: end, each item's
	// right edge should touch the cell's right edge.
	for i, child := range gridBlock.Children {
		cellRight := float64(i+1) * 200
		itemRight := child.X + child.Width
		if itemRight < cellRight-2 || itemRight > cellRight+2 {
			t.Errorf("child %d: right edge %.2f, want ~%.0f (end-aligned)", i, itemRight, cellRight)
		}
	}
}

func TestCSSGridJustifyItemsStart(t *testing.T) {
	// justify-items: start should leave items at the left edge of
	// the cell (no offset), still shrunk to content width.
	htmlStr := `<div style="display: grid; grid-template-columns: 1fr 1fr; justify-items: start">` +
		`<div style="border: 1px solid #000; padding: 2px 6px">A</div>` +
		`<div style="border: 1px solid #000; padding: 2px 6px">B</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	gridBlock := plan.Blocks[0]
	for i, child := range gridBlock.Children {
		cellLeft := float64(i) * 200
		if child.X < cellLeft-1 || child.X > cellLeft+1 {
			t.Errorf("child %d: X = %.2f, want ~%.0f (start-aligned)", i, child.X, cellLeft)
		}
		// Still shrunk to content.
		if child.Width > 100 {
			t.Errorf("child %d: width = %.2f, want < 100", i, child.Width)
		}
	}
}

func TestCSSGridExplicitRowTrackDoesNotStretch(t *testing.T) {
	// Regression for audit blocker 1: a row with an explicit px track
	// must NOT be stretched by the implicit row-stretching pass.
	// Grid template: 60px auto; height: 200px = 150pt.
	// With correct behavior: row 0 stays at 60px = 45pt, row 1 gets
	// the remaining ~105pt.
	// With buggy behavior: both rows split the leftover evenly.
	htmlStr := `<div style="display: grid; grid-template-columns: 1fr; grid-template-rows: 60px auto; height: 200px">` +
		`<div>A</div>` +
		`<div>B</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	gridBlock := plan.Blocks[0]
	if len(gridBlock.Children) < 2 {
		t.Fatalf("expected 2 children, got %d", len(gridBlock.Children))
	}
	// Row 0 Y = 0, row 1 Y = 45 (60px). If row 0 were incorrectly
	// stretched, row 1's Y would be greater than 45.
	row1Y := gridBlock.Children[1].Y
	if row1Y < 44 || row1Y > 46 {
		t.Errorf("row 1 Y = %.2f, want ~45 (row 0 must not stretch past 60px)", row1Y)
	}
}

func TestCSSGridExplicitAlignContentFlexStartDoesNotStretch(t *testing.T) {
	// Regression for audit blocker 2: `align-content: flex-start` is
	// an explicit CSS value that packs rows to the top and leaves
	// leftover space at the bottom. It must be distinguishable from
	// the CSS initial "normal" (which stretches).
	//
	// With explicit flex-start on a 120px grid and small items, rows
	// stay at natural height and items sit near the top.
	htmlStr := `<div style="display: grid; grid-template-columns: 1fr 1fr; height: 120px; align-content: flex-start">` +
		`<div>A</div><div>B</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	gridBlock := plan.Blocks[0]
	for i, child := range gridBlock.Children {
		// Natural height ~14.4pt, no stretching, no align-items.
		// Y should be near 0, not near 37 (stretched center).
		if child.Y > 2 {
			t.Errorf("child %d: Y = %.2f, want near 0 (align-content: flex-start, no stretch)", i, child.Y)
		}
	}
}

func TestCSSGridEmptyWithExplicitHeight(t *testing.T) {
	// Regression for audit nice-to-have #12: a grid with zero children
	// and an explicit height must render at its declared height (not
	// collapse to just padding).
	htmlStr := `<div style="display: grid; grid-template-columns: 1fr 1fr; height: 120px; border: 1px solid #000"></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 800})
	if len(plan.Blocks) == 0 {
		t.Fatal("expected at least one block for empty grid")
	}
	h := plan.Blocks[0].Height
	if h < 88 || h > 92 {
		t.Errorf("empty grid height = %.2f, want ~90 (120px honored)", h)
	}
}

func TestCSSGridJustifyAndAlignItemsCenterWithHeight(t *testing.T) {
	// The full #126 Test 8 scenario: justify-items, align-items, and
	// a definite container height combined. Bordered item Divs shrink
	// to content and center horizontally and vertically.
	htmlStr := `<div style="display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; height: 120px; justify-items: center; align-items: center; padding: 8px">` +
		`<div style="border: 1px solid #000; padding: 2px 6px">Alpha</div>` +
		`<div style="border: 1px solid #000; padding: 2px 6px">Beta</div>` +
		`<div style="border: 1px solid #000; padding: 2px 6px">Gamma</div>` +
		`<div style="border: 1px solid #000; padding: 2px 6px">Delta</div>` +
		`<div style="border: 1px solid #000; padding: 2px 6px">Epsilon</div>` +
		`<div style="border: 1px solid #000; padding: 2px 6px">Zeta</div>` +
		`</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 1000})
	gridBlock := plan.Blocks[0]
	if len(gridBlock.Children) != 6 {
		t.Fatalf("expected 6 children, got %d", len(gridBlock.Children))
	}
	for i, child := range gridBlock.Children {
		if child.Width > 100 {
			t.Errorf("child %d: width = %.2f, want < 100 (shrunk to content)", i, child.Width)
		}
		// No child should be pinned to the top-left of its cell.
		if child.X < 10 {
			t.Errorf("child %d: X = %.2f, want > 10 (not pinned to left)", i, child.X)
		}
		if child.Y < 10 {
			t.Errorf("child %d: Y = %.2f, want > 10 (not pinned to top)", i, child.Y)
		}
	}
}

// createTestJPEG creates a small test JPEG file and returns its path.
func createTestJPEG(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.jpg")
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestBackgroundImageURL(t *testing.T) {
	imgPath := createTestJPEG(t)
	dir := filepath.Dir(imgPath)
	htmlStr := `<div style="background-image: url('test.jpg'); width: 100px; height: 100px;"><p>Hello</p></div>`
	elems, err := Convert(htmlStr, &Options{BaseFS: os.DirFS(dir)})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

func TestBackgroundLinearGradient(t *testing.T) {
	htmlStr := `<div style="background-image: linear-gradient(to right, red, blue); padding: 10px;"><p>Gradient</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

func TestBackgroundRadialGradient(t *testing.T) {
	htmlStr := `<div style="background-image: radial-gradient(red, blue); padding: 10px;"><p>Radial</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestBackgroundSize(t *testing.T) {
	htmlStr := `<div style="background-image: linear-gradient(to right, red, blue); background-size: cover; padding: 10px;"><p>Cover</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

// TestBackgroundSizeWithCalc is a regression test for the same
// strings.Fields tokenization bug fixed for `flex:` (#236),
// `margin:`/`padding:` (#237), `font:` (#240), and `border:` (#242),
// applied here to `background-size:`. Pre-fix, a value like
// `background-size: calc(50px + 50px) 100px` was split into 5 tokens
// ["calc(50px", "+", "50px)", "100px", ...]; parts[0] = "calc(50px"
// (unbalanced) parsed nil → SizeW stayed 0; parts[1] = "+" parsed nil
// → SizeH stayed 0; the explicit size was silently lost.
//
// Pre-existing limitation NOT addressed here: SizeW/SizeH are stored
// as float64 and the parser passes relativeTo=0 to toPoints, so plain
// percent values (`background-size: 50%`) resolve to 0pt. Mirrors the
// margin/padding limitation called out in #237. Out of scope.
func TestBackgroundSizeWithCalc(t *testing.T) {
	tests := []struct {
		name      string
		size      string
		wantSizeW float64 // in pt
		wantSizeH float64 // 0 means unset
	}{
		{
			name: "calc width, plain px height",
			size: "calc(50px + 50px) 100px",
			// 100px = 75pt, 100px = 75pt.
			wantSizeW: 75, wantSizeH: 75,
		},
		{
			name: "calc width only",
			size: "calc(50px + 50px)",
			// Single value → only SizeW set.
			wantSizeW: 75, wantSizeH: 0,
		},
		{
			name: "calc on both axes",
			size: "calc(50px + 50px) calc(20px * 2)",
			// 100px = 75pt; 40px = 30pt.
			wantSizeW: 75, wantSizeH: 30,
		},
		{
			name: "min() width, max() height",
			size: "min(80px, 100px) max(40px, 60px)",
			// min picks 80px = 60pt; max picks 60px = 45pt.
			wantSizeW: 60, wantSizeH: 45,
		},
		{
			name: "clamp() width",
			size: "clamp(50px, 100px, 200px) 80px",
			// clamp middle = 100px = 75pt; 80px = 60pt.
			wantSizeW: 75, wantSizeH: 60,
		},
		{
			name: "calc with subtraction",
			size: "calc(200px - 100px) 50px",
			// 100px = 75pt, 50px = 37.5pt.
			wantSizeW: 75, wantSizeH: 37.5,
		},
		{
			name: "calc with multiplication",
			size: "calc(50px * 2) 50px",
			// 100px = 75pt, 50px = 37.5pt.
			wantSizeW: 75, wantSizeH: 37.5,
		},
		{
			name: "calc with division",
			size: "calc(200px / 2) 50px",
			// 100px = 75pt.
			wantSizeW: 75, wantSizeH: 37.5,
		},
		{
			name:      "tab as separator",
			size:      "calc(50px + 50px)\t100px",
			wantSizeW: 75, wantSizeH: 75,
		},
		{
			name:      "newline as separator",
			size:      "calc(50px + 50px)\n100px",
			wantSizeW: 75, wantSizeH: 75,
		},
		{
			name:      "plain two-value (no calc) still works post-swap",
			size:      "100px 50px",
			wantSizeW: 75, wantSizeH: 37.5,
		},
		{
			name: "3+ tokens: extras silently dropped",
			// CSS background-size accepts 1 or 2 values. The parser only
			// reads parts[0] and parts[1]; any extras are ignored. Locks
			// that contract.
			size:      "100px 50px 25px",
			wantSizeW: 75, wantSizeH: 37.5,
		},
		{
			name: "auto keyword short-circuits before token parsing",
			// "auto" hits the early-exit guard before splitTopLevelFields
			// is reached; SizeW/SizeH stay at 0. Sanity check that the
			// guard wasn't accidentally dropped by this PR.
			size:      "auto",
			wantSizeW: 0, wantSizeH: 0,
		},
		{
			name: "unbalanced calc paren: SizeW and SizeH stay 0",
			// splitTopLevelFields keeps the unbalanced calc + trailing
			// characters as one single token (depth never returns to 0).
			// So len(parts) == 1: parseLength rejects the lone token →
			// SizeW stays 0; SizeH stays 0 because there's no parts[1] —
			// not because parseLength rejected anything. Documents the
			// no-crash invariant on malformed input.
			size:      "calc(50px + 50px 100px",
			wantSizeW: 0, wantSizeH: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &converter{
				embeddedFonts: make(map[string]*font.EmbeddedFont),
			}
			style := computedStyle{
				BackgroundImage: "linear-gradient(red, blue)",
				BackgroundSize:  tt.size,
				FontSize:        12,
			}
			bgImg := c.resolveBackgroundImage(style)
			if bgImg == nil {
				t.Fatal("resolveBackgroundImage returned nil")
			}
			if math.Abs(bgImg.SizeW-tt.wantSizeW) > 0.01 {
				t.Errorf("SizeW = %.4f, want %.4f", bgImg.SizeW, tt.wantSizeW)
			}
			if math.Abs(bgImg.SizeH-tt.wantSizeH) > 0.01 {
				t.Errorf("SizeH = %.4f, want %.4f", bgImg.SizeH, tt.wantSizeH)
			}
		})
	}
}

func TestBackgroundPosition(t *testing.T) {
	htmlStr := `<div style="background-image: linear-gradient(to right, red, blue); background-position: center; padding: 10px;"><p>Center</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestBackgroundRepeatNoRepeat(t *testing.T) {
	htmlStr := `<div style="background-image: linear-gradient(to right, red, blue); background-repeat: no-repeat; padding: 10px;"><p>No Repeat</p></div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestBackgroundShorthandWithImage(t *testing.T) {
	imgPath := createTestJPEG(t)
	dir := filepath.Dir(imgPath)
	htmlStr := `<div style="background: url('test.jpg') no-repeat center; padding: 10px;"><p>Shorthand</p></div>`
	elems, err := Convert(htmlStr, &Options{BaseFS: os.DirFS(dir)})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestParseBackgroundImage(t *testing.T) {
	tests := []struct {
		input    string
		wantKind string
		wantVal  string
	}{
		{`url("test.jpg")`, "url", "test.jpg"},
		{`url('test.png')`, "url", "test.png"},
		{`linear-gradient(to right, red, blue)`, "linear-gradient", "to right, red, blue"},
		{`radial-gradient(red, blue)`, "radial-gradient", "red, blue"},
		{`none`, "", "none"},
	}
	for _, tt := range tests {
		kind, val := parseBackgroundImage(tt.input)
		if kind != tt.wantKind {
			t.Errorf("parseBackgroundImage(%q): kind = %q, want %q", tt.input, kind, tt.wantKind)
		}
		if val != tt.wantVal {
			t.Errorf("parseBackgroundImage(%q): val = %q, want %q", tt.input, val, tt.wantVal)
		}
	}
}

func TestParseLinearGradient(t *testing.T) {
	tests := []struct {
		args      string
		wantAngle float64
		wantStops int
	}{
		{"to right, red, blue", 90, 2},
		{"to bottom, red, green, blue", 180, 3},
		{"45deg, #ff0000 0%, #0000ff 100%", 45, 2},
		{"red, blue", 180, 2},
	}
	for _, tt := range tests {
		angle, stops := parseLinearGradient(tt.args)
		if angle != tt.wantAngle {
			t.Errorf("parseLinearGradient(%q): angle = %v, want %v", tt.args, angle, tt.wantAngle)
		}
		if len(stops) != tt.wantStops {
			t.Errorf("parseLinearGradient(%q): %d stops, want %d", tt.args, len(stops), tt.wantStops)
		}
	}
}

func TestParseRadialGradient(t *testing.T) {
	stops := parseRadialGradient("red, blue")
	if len(stops) != 2 {
		t.Errorf("expected 2 stops, got %d", len(stops))
	}

	stops = parseRadialGradient("circle, #ff0000 0%, #0000ff 100%")
	if len(stops) != 2 {
		t.Errorf("expected 2 stops, got %d", len(stops))
	}
}

// TestParseBgPosition was migrated from the original [2]float64 fraction
// assertion to the new [2]layout.ResolvableLength shape. The expected
// fractions are recovered by Resolve(100, 0) on each axis — percent
// leaves multiply Value/100 by 100, so a 50% axis resolves to 50 and
// the test reads back the original 0.5 fraction by dividing by 100.
// Plain-length axes (none of the inputs below use them) would not
// round-trip through this helper; the dedicated bg_position_lazy_test
// covers those cases.
func TestParseBgPosition(t *testing.T) {
	tests := []struct {
		input string
		wantX float64
		wantY float64
	}{
		{"center", 0.5, 0.5},
		{"top left", 0, 0},
		{"bottom right", 1, 1},
		{"left", 0, 0.5},
		{"50% 50%", 0.5, 0.5},
		{"", 0, 0},
	}
	for _, tt := range tests {
		pos := parseBgPosition(tt.input)
		gotX := pos[0].Resolve(100, 0) / 100
		gotY := pos[1].Resolve(100, 0) / 100
		if math.Abs(gotX-tt.wantX) > 1e-9 || math.Abs(gotY-tt.wantY) > 1e-9 {
			t.Errorf("parseBgPosition(%q) = [%v, %v], want [%v, %v]",
				tt.input, gotX, gotY, tt.wantX, tt.wantY)
		}
	}
}

func TestConvertTableBorderSpacing(t *testing.T) {
	// border-spacing should be parsed and applied to the table.
	html := `<table style="border-collapse: separate; border-spacing: 10px">
<tr><td>A</td><td>B</td></tr>
</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	tbl := findTable(elems)
	if tbl == nil {
		t.Fatal("expected a Table element")
	}
	if tbl.BorderCollapse() {
		t.Error("table should not be collapsed")
	}
	// Column widths should be reduced by horizontal spacing.
	// 2 columns, 3 gaps of 10px*0.75=7.5pt each = 22.5pt consumed.
	widths := tbl.Layout(400)
	totalW := 0.0
	for _, l := range widths {
		_ = l // just ensure no panic
	}
	_ = totalW
}

// TestConvertTableCellBorderStyleHiddenOnly covers a <td> that declares
// only border-style: hidden (no width, no other side). hasBorder()'s
// width>0 check alone would miss this — the border-collapse resolver
// needs "hidden" to reach buildCellBorders as a distinguishable value
// (see cellHasBorderDeclaration) rather than being treated the same as no
// border at all. This test only guards against a crash in the full
// pipeline; the hidden-always-wins conflict rule itself is exercised in
// layout/bordercollapse_test.go and html/issue378_bordercollapse_test.go.
func TestConvertTableCellBorderStyleHiddenOnly(t *testing.T) {
	src := `<table style="border-collapse: collapse">
<tr><td style="border-style: hidden">A</td><td>B</td></tr>
</table>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := layout.NewRenderer(612, 792, layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	for _, e := range elems {
		r.Add(e)
	}
	pages := r.Render()
	if len(pages) == 0 {
		t.Fatal("expected at least 1 page")
	}
}

func TestConvertTableBorderSpacingTwoValues(t *testing.T) {
	// Two-value border-spacing: horizontal vertical.
	html := `<table style="border-collapse: separate; border-spacing: 5px 10px">
<tr><td>A</td><td>B</td></tr>
</table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	tbl := findTable(elems)
	if tbl == nil {
		t.Fatal("expected a Table element")
	}
	// Should not panic and should produce valid layout.
	plan := tbl.PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
}

// TestBorderSpacingShorthandWithCalc is a regression test for the same
// strings.Fields tokenization bug fixed for `flex:` (#236),
// `margin:`/`padding:` (#237), `font:` (#240), `border:` (#242),
// `background-size:` (#244), `@page size` (#247), `gap:` (#249),
// `border-radius:` (#252), `box-shadow:` (#254), and
// `transform-origin:` (#257) — applied here to `border-spacing`.
// Pre-fix `border-spacing: calc(2px + 2px) 8px` became 4 tokens
// ["calc(2px", "+", "2px)", "8px"]; parts[0] = "calc(2px" failed
// parseLength → BorderSpacingH stayed 0; parts[1] = "+" failed too →
// BorderSpacingV stayed 0; both axes silently lost.
func TestBorderSpacingShorthandWithCalc(t *testing.T) {
	tests := []struct {
		name  string
		val   string
		wantH float64 // in pt
		wantV float64
	}{
		{
			name: "single calc applies to both axes",
			val:  "calc(2px + 2px)",
			// 4px = 3pt.
			wantH: 3, wantV: 3,
		},
		{
			name: "two values: calc h, plain v",
			val:  "calc(2px + 2px) 8px",
			// h = 3pt; v = 6pt.
			wantH: 3, wantV: 6,
		},
		{
			name: "two values: plain h, calc v",
			val:  "8px calc(2px * 2)",
			// h = 6pt; v = 4px = 3pt.
			wantH: 6, wantV: 3,
		},
		{
			name: "two calcs",
			val:  "calc(4px - 2px) calc(8px / 2)",
			// h = 2px = 1.5pt; v = 4px = 3pt.
			wantH: 1.5, wantV: 3,
		},
		{
			name: "calc with addition",
			val:  "calc(4px + 2px) 8px",
			// h = 6px = 4.5pt.
			wantH: 4.5, wantV: 6,
		},
		{
			name: "calc with subtraction",
			val:  "calc(8px - 4px) 8px",
			// h = 4px = 3pt.
			wantH: 3, wantV: 6,
		},
		{
			name: "calc with multiplication",
			val:  "calc(2px * 2) 8px",
			// h = 4px = 3pt.
			wantH: 3, wantV: 6,
		},
		{
			name: "calc with division",
			val:  "calc(8px / 2) 8px",
			// h = 4px = 3pt.
			wantH: 3, wantV: 6,
		},
		{
			name: "min() h, max() v",
			val:  "min(4px, 8px) max(8px, 4px)",
			// h = min = 4px = 3pt; v = max = 8px = 6pt.
			wantH: 3, wantV: 6,
		},
		{
			name: "clamp() single value",
			val:  "clamp(2px, 4px, 8px)",
			// middle wins: 4px = 3pt, applied to both axes.
			wantH: 3, wantV: 3,
		},
		{
			name:  "tab separator",
			val:   "calc(2px + 2px)\t8px",
			wantH: 3, wantV: 6,
		},
		{
			name:  "newline separator",
			val:   "calc(2px + 2px)\n8px",
			wantH: 3, wantV: 6,
		},
		{
			name: "3+ tokens: parser reads only first two",
			val:  "5px 10px 15px",
			// h = 5px = 3.75pt; v = 10px = 7.5pt; third ignored.
			wantH: 3.75, wantV: 7.5,
		},
		{
			name: "unbalanced calc paren: spacing stays 0",
			// splitTopLevelFields keeps the unbalanced calc + trailing
			// chars as one token (parts length 1) → single-value branch
			// runs → parseLength fails → both H and V are assigned the
			// same parsed-zero value.
			val:   "calc(2px + 2px",
			wantH: 0, wantV: 0,
		},
		{
			name: "empty value: spacing stays 0",
			// parts has length 0 → neither branch runs → defaults stay.
			val:   "",
			wantH: 0, wantV: 0,
		},
		{
			name:  "whitespace-only value: spacing stays 0",
			val:   "   \t\n   ",
			wantH: 0, wantV: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &converter{}
			style := &computedStyle{FontSize: 12}
			c.applyProperty("border-spacing", tt.val, style)
			if math.Abs(style.BorderSpacingH-tt.wantH) > 0.01 {
				t.Errorf("applyProperty(%q): BorderSpacingH = %.4f, want %.4f",
					tt.val, style.BorderSpacingH, tt.wantH)
			}
			if math.Abs(style.BorderSpacingV-tt.wantV) > 0.01 {
				t.Errorf("applyProperty(%q): BorderSpacingV = %.4f, want %.4f",
					tt.val, style.BorderSpacingV, tt.wantV)
			}
		})
	}
}

func TestConvertCSSTableBorderSpacing(t *testing.T) {
	// CSS display:table with border-spacing.
	html := `<div style="display: table; border-spacing: 8px">
<div style="display: table-row">
<div style="display: table-cell">A</div>
<div style="display: table-cell">B</div>
</div>
</div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	tbl := findTable(elems)
	if tbl == nil {
		t.Fatal("expected a Table element")
	}
}

func TestBackgroundImageHTTPURL(t *testing.T) {
	// Create a test HTTP server that serves a PNG image.
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 0, G: 0, B: 255, A: 255})
		}
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_ = png.Encode(w, img)
	}))
	defer srv.Close()

	htmlStr := fmt.Sprintf(
		`<div style="background-image: url('%s/bg.png'); width: 100px; height: 100px;"><p>Hello</p></div>`,
		srv.URL,
	)
	elems, err := Convert(htmlStr, &Options{AllowRemoteFetch: true, URLPolicy: allowAllURLs})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

func TestConvertInlineContainerWithBr(t *testing.T) {
	// Regression: <br> tags inside an inline container (e.g. a <div> with
	// inline children like <span>) used to panic with:
	//   "layout.NewStyledParagraph: run 2 has nil Font and nil Embedded"
	// because collectRuns inserts IsLineBreak marker runs for <br> elements,
	// which were passed unsanitised to NewStyledParagraph.
	htmlInput := `<div>
			<br>
			<span><b>Title</b></span><br><br>
			<span>Subtitle</span>
			<span>Another <br> subtitle</span>
		      </div>`

	elems, err := Convert(htmlInput, nil)
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	if len(elems) == 0 {
		t.Fatal("expected at least one element")
	}
	// Elements from bare <br> tags are empty paragraphs with zero height —
	// that is expected. Only check that elements with text have positive height.
	nonEmpty := 0
	for i, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
		if plan.Consumed > 0 {
			nonEmpty++
		} else if plan.Status != layout.LayoutFull {
			t.Errorf("element %d: zero-height element has unexpected status %v", i, plan.Status)
		}
	}
	if nonEmpty == 0 {
		t.Error("expected at least one element with positive consumed height")
	}
}

func TestConvertInlineContainerWithBrProducesMultipleElements(t *testing.T) {
	// Each <br> inside an inline container should split content into a
	// separate paragraph element, so "before" and "after" end up in
	// distinct layout elements.
	htmlInput := `<div>before<br>after</div>`

	elems, err := Convert(htmlInput, nil)
	if err != nil {
		t.Fatalf("Convert returned error: %v", err)
	}
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements (one per line), got %d", len(elems))
	}
}

// --- text-align-last tests ---

func TestTextAlignLastCenter(t *testing.T) {
	src := `<p style="text-align: justify; text-align-last: center;">
		The quick brown fox jumps over the lazy dog repeatedly until the text wraps.
	</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 200, Height: 500})
	if plan.Status != layout.LayoutFull {
		t.Fatal("expected LayoutFull")
	}
}

func TestTextAlignLastRight(t *testing.T) {
	src := `<p style="text-align: justify; text-align-last: right;">
		Some longer text that will wrap to multiple lines for testing.
	</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 200, Height: 500})
	if plan.Status != layout.LayoutFull {
		t.Fatal("expected LayoutFull")
	}
}

func TestTextAlignLastNotSet(t *testing.T) {
	// Without text-align-last, justified text should default last line to left.
	src := `<p style="text-align: justify;">Some text here.</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestTextAlignLastJustify(t *testing.T) {
	// text-align-last: justify should justify even the last line.
	src := `<p style="text-align: justify; text-align-last: justify;">
		All lines should be justified including the very last line.
	</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestTextAlignLastInherited(t *testing.T) {
	// text-align-last should inherit from parent.
	src := `<div style="text-align: justify; text-align-last: center;">
		<p>This paragraph inherits text-align-last from the div.</p>
	</div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- CMYK color tests ---

func TestCMYKColorParsing(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"cmyk(0, 1, 1, 0)", true},            // red
		{"cmyk(0, 0, 0, 1)", true},            // black
		{"cmyk(0%, 100%, 100%, 0%)", true},    // red with percentages
		{"device-cmyk(0.5, 0.3, 0, 0)", true}, // device-cmyk variant
		{"cmyk(0, 0, 0, 0)", true},            // white
		{"cmyk(0, 0)", false},                 // too few args
		{"cmyk()", false},
	}
	for _, tt := range tests {
		_, ok := parseColor(tt.input)
		if ok != tt.ok {
			t.Errorf("parseColor(%q): got ok=%v, want %v", tt.input, ok, tt.ok)
		}
	}
}

func TestCMYKColorSpace(t *testing.T) {
	c, ok := parseColor("cmyk(0, 1, 1, 0)")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if c.Space != layout.ColorSpaceCMYK {
		t.Errorf("expected CMYK color space, got %v", c.Space)
	}
}

func TestCMYKInHTML(t *testing.T) {
	src := `<p style="color: cmyk(0, 0, 0, 1);">Black text in CMYK</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestDeviceCMYKInHTML(t *testing.T) {
	src := `<p style="color: device-cmyk(1, 0, 0, 0);">Cyan text</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestCMYKComponentClamping(t *testing.T) {
	// Values > 1 should be clamped.
	c, ok := parseColor("cmyk(2, -0.5, 0.5, 0)")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if c.C != 1.0 {
		t.Errorf("expected C clamped to 1.0, got %f", c.C)
	}
	if c.M != 0.0 {
		t.Errorf("expected M clamped to 0.0, got %f", c.M)
	}
}

// --- ::marker pseudo-element tests ---

func TestMarkerColor(t *testing.T) {
	src := `<style>li::marker { color: red; }</style>
	<ul><li>Item 1</li><li>Item 2</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 500})
	if plan.Status != layout.LayoutFull {
		t.Fatal("expected LayoutFull")
	}
}

func TestMarkerFontSize(t *testing.T) {
	src := `<style>li::marker { font-size: 20px; }</style>
	<ol><li>Item 1</li><li>Item 2</li></ol>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestMarkerNoEffect(t *testing.T) {
	// Without ::marker styles, default behavior should be unchanged.
	src := `<ul><li>Item 1</li><li>Item 2</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- object-fit tests ---

func TestObjectFitCover(t *testing.T) {
	src := `<img style="width: 100px; height: 100px; object-fit: cover;" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVQI12NgAAIABQABNjN9GQAAAABJRu5ErkJggg==">`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 500})
	if plan.Status != layout.LayoutFull {
		t.Fatal("expected LayoutFull")
	}
}

func TestObjectFitContain(t *testing.T) {
	src := `<img style="width: 200px; height: 50px; object-fit: contain;" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVQI12NgAAIABQABNjN9GQAAAABJRu5ErkJggg==">`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestObjectFitFill(t *testing.T) {
	src := `<img style="width: 200px; height: 50px; object-fit: fill;" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVQI12NgAAIABQABNjN9GQAAAABJRu5ErkJggg==">`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestObjectFitNone(t *testing.T) {
	src := `<img style="width: 200px; height: 200px; object-fit: none;" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVQI12NgAAIABQABNjN9GQAAAABJRu5ErkJggg==">`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestObjectFitNotSet(t *testing.T) {
	// Default: no object-fit, image should use its natural aspect ratio.
	src := `<img style="width: 200px;" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVQI12NgAAIABQABNjN9GQAAAABJRu5ErkJggg==">`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

// --- @supports tests ---

func TestSupportsKnownProperty(t *testing.T) {
	src := `<style>
	@supports (display: flex) { .box { color: red; } }
	</style>
	<div class="box"><p>Supported</p></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestSupportsUnknownProperty(t *testing.T) {
	src := `<style>
	@supports (unknown-property: value) { .box { color: blue; } }
	</style>
	<div class="box"><p>Not supported</p></div>`
	_, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSupportsNot(t *testing.T) {
	src := `<style>
	@supports not (display: flex) { .box { color: green; } }
	</style>
	<div class="box"><p>Should not apply</p></div>`
	_, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSupportsAnd(t *testing.T) {
	src := `<style>
	@supports (display: flex) and (color: red) { .box { font-size: 20px; } }
	</style>
	<div class="box"><p>Both supported</p></div>`
	_, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestSupportsOr(t *testing.T) {
	src := `<style>
	@supports (unknown: val) or (display: flex) { .box { font-weight: bold; } }
	</style>
	<div class="box"><p>One supported</p></div>`
	_, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEvaluateSupports(t *testing.T) {
	tests := []struct {
		condition string
		want      bool
	}{
		{"(display: flex)", true},
		{"(color: red)", true},
		{"(unknown-prop: val)", false},
		{"not (display: flex)", false},
		{"not (unknown-prop: val)", true},
		{"(display: flex) and (color: red)", true},
		{"(display: flex) and (unknown: val)", false},
		{"(unknown: a) or (display: flex)", true},
		{"(unknown: a) or (other-unknown: b)", false},
	}
	for _, tt := range tests {
		got := evaluateSupports(tt.condition)
		if got != tt.want {
			t.Errorf("evaluateSupports(%q) = %v, want %v", tt.condition, got, tt.want)
		}
	}
}

// --- min()/max()/clamp() tests ---

func TestMinFunction(t *testing.T) {
	l := parseLength("min(200px, 100px)")
	if l == nil {
		t.Fatal("expected non-nil length for min()")
	}
	pts := l.toPoints(0, 12)
	if abs(pts-75) > 0.1 {
		t.Errorf("min(200px, 100px) = %fpt, want 75pt", pts)
	}
}

func TestMaxFunction(t *testing.T) {
	l := parseLength("max(50px, 100px)")
	if l == nil {
		t.Fatal("expected non-nil length for max()")
	}
	pts := l.toPoints(0, 12)
	if abs(pts-75) > 0.1 {
		t.Errorf("max(50px, 100px) = %fpt, want 75pt", pts)
	}
}

func TestClampFunction(t *testing.T) {
	l := parseLength("clamp(50px, 200px, 100px)")
	if l == nil {
		t.Fatal("expected non-nil length for clamp()")
	}
	pts := l.toPoints(0, 12)
	if abs(pts-75) > 0.1 {
		t.Errorf("clamp(50px, 200px, 100px) = %fpt, want 75pt", pts)
	}
}

func TestMinWithPercentage(t *testing.T) {
	l := parseLength("min(200px, 50%)")
	if l == nil {
		t.Fatal("expected non-nil length")
	}
	pts := l.toPoints(400, 12)
	if abs(pts-150) > 0.1 {
		t.Errorf("min(200px, 50%%) at 400pt = %fpt, want 150pt", pts)
	}
}

func TestClampMiddleInRange(t *testing.T) {
	l := parseLength("clamp(50px, 80px, 200px)")
	if l == nil {
		t.Fatal("expected non-nil")
	}
	pts := l.toPoints(0, 12)
	if abs(pts-60) > 0.1 {
		t.Errorf("clamp(50px, 80px, 200px) = %fpt, want 60pt", pts)
	}
}

func TestMinInCSS(t *testing.T) {
	src := `<div style="width: min(200px, 300px);"><p>Sized div</p></div>`
	_, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestMinTooFewArgs(t *testing.T) {
	if parseLength("min(100px)") != nil {
		t.Error("min() with 1 arg should return nil")
	}
}

func TestMaxTooFewArgs(t *testing.T) {
	if parseLength("max(100px)") != nil {
		t.Error("max() with 1 arg should return nil")
	}
}

func TestClampWrongArgCount(t *testing.T) {
	if parseLength("clamp(100px, 200px)") != nil {
		t.Error("clamp() with 2 args should return nil")
	}
}

// --- CSS bookmark tests ---

func TestBookmarkLevelOverride(t *testing.T) {
	src := `<h3 style="bookmark-level: 1;">Promoted heading</h3>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 500})
	// The heading may be wrapped in a Div; find the H-tagged block.
	tag := findBlockTag(plan.Blocks)
	if tag != "H1" {
		t.Errorf("expected tag H1, got %q", tag)
	}
}

func TestBookmarkLabelOverride(t *testing.T) {
	src := `<h2 style="bookmark-label: 'Custom Label';">Original text</h2>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 500})
	text := findBlockHeadingText(plan.Blocks)
	if text != "Custom Label" {
		t.Errorf("expected 'Custom Label', got %q", text)
	}
}

func TestBookmarkDefaultBehavior(t *testing.T) {
	src := `<h2>Normal heading</h2>`
	elems, _ := Convert(src, nil)
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 500})
	tag := findBlockTag(plan.Blocks)
	if tag != "H2" {
		t.Errorf("expected tag H2, got %q", tag)
	}
}

// findBlockTag recursively finds the first H-tagged block in a tree.
func findBlockTag(blocks []layout.PlacedBlock) string {
	for _, b := range blocks {
		if strings.HasPrefix(b.Tag, "H") {
			return b.Tag
		}
		if tag := findBlockTag(b.Children); tag != "" {
			return tag
		}
	}
	return ""
}

// findBlockHeadingText recursively finds the first HeadingText in a block tree.
func findBlockHeadingText(blocks []layout.PlacedBlock) string {
	for _, b := range blocks {
		if b.HeadingText != "" {
			return b.HeadingText
		}
		if text := findBlockHeadingText(b.Children); text != "" {
			return text
		}
	}
	return ""
}

func TestBookmarkLevelZero(t *testing.T) {
	src := `<h2 style="bookmark-level: 0;">Hidden from bookmarks</h2>`
	_, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// --- :is() and :where() selector tests ---

func TestIsSelector(t *testing.T) {
	src := `<style>
	:is(h1, h2, h3) { color: red; }
	</style>
	<h1>Title</h1><h2>Subtitle</h2><p>Normal</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 3 {
		t.Fatalf("expected at least 3 elements, got %d", len(elems))
	}
}

func TestWhereSelector(t *testing.T) {
	src := `<style>
	:where(.a, .b) { font-weight: bold; }
	</style>
	<p class="a">A</p><p class="b">B</p><p class="c">C</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 3 {
		t.Fatalf("expected at least 3 elements, got %d", len(elems))
	}
}

func TestIsSelectorNoMatch(t *testing.T) {
	// :is() with no matching selectors should not apply.
	src := `<style>
	:is(.x, .y) { color: blue; }
	</style>
	<p class="a">No match</p>`
	_, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func TestIsSelectorNested(t *testing.T) {
	// :is() used in a compound selector: div :is(p, span)
	src := `<style>
	div :is(p, span) { font-style: italic; }
	</style>
	<div><p>Italic</p><span>Also italic</span></div>`
	_, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// --- repeating gradient tests ---

func TestRepeatingLinearGradient(t *testing.T) {
	src := `<div style="background: repeating-linear-gradient(45deg, red, blue 20px); height: 100px; width: 200px;">
		<p>Striped background</p>
	</div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestRepeatingRadialGradient(t *testing.T) {
	src := `<div style="background: repeating-radial-gradient(circle, red, blue 20px); height: 100px; width: 200px;">
		<p>Radial</p>
	</div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
}

func TestRepeatingGradientInBackgroundImage(t *testing.T) {
	src := `<div style="background-image: repeating-linear-gradient(red, blue); height: 50px;">
		<p>BG image</p>
	</div>`
	_, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// --- column-width tests ---

func TestColumnWidthAutoCount(t *testing.T) {
	// column-width: 150px with a 500px container should produce ~3 columns.
	src := `<div style="column-width: 150px;">
		<p>A</p><p>B</p><p>C</p><p>D</p><p>E</p><p>F</p>
	</div>`
	elems, err := Convert(src, &Options{PageWidth: 500})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) < 1 {
		t.Fatal("expected at least 1 element")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 800})
	if plan.Status != layout.LayoutFull {
		t.Fatal("expected LayoutFull")
	}
}

func TestColumnWidthNarrowContainer(t *testing.T) {
	// column-width wider than container should produce 1 column.
	src := `<div style="column-width: 500px;">
		<p>Only one column possible.</p>
	</div>`
	_, err := Convert(src, &Options{PageWidth: 300})
	if err != nil {
		t.Fatal(err)
	}
}

func TestColumnsShorthandWithWidth(t *testing.T) {
	// "columns: 200px" sets column-width, not column-count.
	src := `<div style="columns: 200px;">
		<p>A</p><p>B</p><p>C</p>
	</div>`
	_, err := Convert(src, &Options{PageWidth: 600})
	if err != nil {
		t.Fatal(err)
	}
}

func TestColumnsShorthandCountAndWidth(t *testing.T) {
	// "columns: 3 200px" sets both.
	src := `<div style="columns: 3 200px;">
		<p>A</p><p>B</p><p>C</p>
	</div>`
	_, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
}

// TestInlineStrongInDivWrappedAsAnonymousBlock is a regression test for
// a bug where mixing text with inline <strong>/<em>/<span>/<a> inside a
// <div> (or any block container that's not <p>) produced one Paragraph
// per sibling node — a text node, then an inline element, then another
// text node became three stacked paragraphs instead of one wrapped
// paragraph with the emphasis inline. This broke roughly every
// letter-style template that weaved dynamic data into prose: the
// punctuation after the inline element (the "." in "...offer to
// <strong>X</strong>. Following...") was orphaned at the start of the
// third "line" because it was its own paragraph.
//
// CSS 2.1 §9.2.1.1 specifies that when a block container has mixed
// inline and block children, consecutive inline siblings are wrapped
// into an anonymous block box. walkChildren now implements this.
func TestInlineStrongInDivWrappedAsAnonymousBlock(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{
			"strong inside div",
			`<div>We're pleased to extend this offer to <strong>Globex Corporation</strong>. Following our discussions...</div>`,
		},
		{
			"em inside div",
			`<div>The value is <em>flexible</em> and can be adjusted later.</div>`,
		},
		{
			"span inside div",
			`<div>Part one <span>middle</span> part three.</div>`,
		},
		{
			"a inside div",
			`<div>Visit <a href="https://example.com">our site</a> for details.</div>`,
		},
		{
			"no wrapping element (implicit body)",
			`Plain text <strong>bold</strong> more plain text.`,
		},
		{
			"multiple inline elements mixed",
			`<div>First <strong>bold</strong> then <em>italic</em> then plain.</div>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			elems, err := Convert(tc.src, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(elems) != 1 {
				t.Errorf("expected 1 top-level element (one anonymous "+
					"block paragraph), got %d — inline siblings of a "+
					"block container must be wrapped together, not "+
					"split into separate paragraphs",
					len(elems))
				for i, e := range elems {
					t.Logf("  [%d] %T", i, e)
				}
			}
		})
	}
}

// TestBlockChildrenStillStaySeparate guards against the anonymous-block
// fix from over-reaching: actual block siblings inside a div must remain
// separate elements, not be collapsed into a single paragraph.
func TestBlockChildrenStillStaySeparate(t *testing.T) {
	src := `<div><p>First para</p><p>Second para</p><p>Third para</p></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 3 {
		t.Errorf("expected 3 paragraphs, got %d", len(elems))
	}
}

// TestMixedBlockAndInlineChildrenInDiv exercises the combined case: a
// div containing inline text, a block element, then more inline text
// must produce three elements — an anonymous paragraph, the block, and
// another anonymous paragraph. The inline runs on either side of the
// block must not be merged across the block boundary.
func TestMixedBlockAndInlineChildrenInDiv(t *testing.T) {
	src := `<div>Before text <strong>bold</strong>.<p>Middle block paragraph</p>After text <em>emphasized</em>.</div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 3 {
		t.Fatalf("expected 3 elements (anon-before + p + anon-after), got %d", len(elems))
	}
}

// TestTablePercentageCellWidthInsideConstrainedFlex is a regression
// test for a bug where <td style="width:50%"> inside a narrow flex
// column overflowed the column horizontally. The converter was
// resolving the percentage against the outer containerWidth at convert
// time (e.g. 50% of 612pt = 306pt), producing a width hint much larger
// than the table's actual layout width. The content rendered far off
// the right edge of the page.
//
// The fix stores percentage widths as lazy UnitValues
// (Cell.SetWidthHintUnit) which resolve against the table's actual
// maxWidth at layout time.
func TestTablePercentageCellWidthInsideConstrainedFlex(t *testing.T) {
	src := `<div style="display:flex;">
  <div style="flex:1;">MAIN</div>
  <div style="flex:0 0 280px;padding-left:32px;">
    <table style="width:100%;border-collapse:collapse;">
      <tr>
        <td style="width:50%;">Engagement type</td>
        <td>Existing Business</td>
      </tr>
    </table>
  </div>
</div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := layout.NewRenderer(612, 792, layout.Margins{Top: 36, Right: 36, Bottom: 36, Left: 36})
	for _, e := range elems {
		r.Add(e)
	}
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	// Extract all text X coordinates from the content stream.
	// Page right margin is at X = 612 - 36 = 576pt. Nothing should
	// render past that. Pre-fix, the right cell started at X=688,
	// 112pt past the margin.
	stream := string(pages[0].Stream.Bytes())
	maxX := 0.0
	i := 0
	for i < len(stream) {
		if i+3 <= len(stream) && stream[i:i+3] == " Td" {
			j := i - 1
			for j >= 0 && stream[j] != '\n' {
				j--
			}
			line := stream[j+1 : i]
			var x, y float64
			_, _ = fmt.Sscanf(line, "%f %f", &x, &y)
			if x > maxX {
				maxX = x
			}
		}
		i++
	}
	if maxX > 576.01 {
		t.Errorf("content rendered past right margin: maxX=%.1f, "+
			"right margin=576 — <td style=\"width:50%%\"> must resolve "+
			"against the table's actual width, not the outer container",
			maxX)
	}
}
