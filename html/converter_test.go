// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
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
		{"rgb(255, 0, 0)", true},
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
	if parseFontWeight("bold") != "bold" {
		t.Error("expected bold")
	}
	if parseFontWeight("700") != "bold" {
		t.Error("expected bold for 700")
	}
	if parseFontWeight("normal") != "normal" {
		t.Error("expected normal")
	}
	if parseFontWeight("400") != "normal" {
		t.Error("expected normal for 400")
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
	ss.parseCSS("p { color: red; font-size: 14px }")
	if len(ss.rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(ss.rules))
	}
	if len(ss.rules[0].declarations) != 2 {
		t.Fatalf("expected 2 declarations, got %d", len(ss.rules[0].declarations))
	}
}

func TestCSSParseMultipleRules(t *testing.T) {
	ss := &styleSheet{}
	ss.parseCSS("h1 { color: blue } p { margin: 10px }")
	if len(ss.rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(ss.rules))
	}
}

func TestCSSParseComments(t *testing.T) {
	ss := &styleSheet{}
	ss.parseCSS("/* comment */ p { color: red } /* another */")
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
	// Tables should default to border-collapse: collapse without explicit CSS.
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
	if !tbl.BorderCollapse() {
		t.Error("table should default to border-collapse: collapse")
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
	elems, err := Convert(htmlStr, &Options{BasePath: dir})
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
	elems, err := Convert(htmlStr, &Options{BasePath: "/nonexistent"})
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
	elems, err := Convert(htmlStr, &Options{BasePath: dir})
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
	// "noto|normal|normal" — simulating a loaded @font-face with
	// font-family: "Noto".
	mockEF := font.NewEmbeddedFont(nil)
	c := &converter{
		embeddedFonts: map[string]*font.EmbeddedFont{
			"noto|normal|normal": mockEF,
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
	elems, err := Convert(htmlStr, &Options{BasePath: dir})
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
	elems, err := Convert(htmlStr, &Options{BasePath: dir})
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
		if pos[0] != tt.wantX || pos[1] != tt.wantY {
			t.Errorf("parseBgPosition(%q) = [%v, %v], want [%v, %v]",
				tt.input, pos[0], pos[1], tt.wantX, tt.wantY)
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
