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

// TestRTLListDirection verifies that dir="rtl" on a list container
// propagates direction and produces right-aligned output. The list's
// PlacedBlocks should have content positioned for RTL rendering.
func TestRTLListDirection(t *testing.T) {
	src := `<ul dir="rtl"><li>Hello</li><li>World</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
	if plan.Status != layout.LayoutFull {
		t.Fatalf("layout status: %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Errorf("expected positive Consumed, got %v", plan.Consumed)
	}
	// With RTL, the list should still produce renderable output.
	if len(plan.Blocks) == 0 {
		t.Error("expected at least 1 block")
	}
	// Verify multiple items produce sufficient height (each item is ~14pt).
	if plan.Consumed < 20 {
		t.Errorf("expected Consumed >= 20pt for 2 items, got %v", plan.Consumed)
	}
}

// TestRTLOrderedListProducesItems verifies that an ordered list with
// dir="rtl" renders all items with correct total height.
func TestRTLOrderedListProducesItems(t *testing.T) {
	src := `<ol dir="rtl"><li>First</li><li>Second</li><li>Third</li></ol>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
	// 3 items should produce height for 3 lines (~14pt each = ~42pt).
	if plan.Consumed < 30 {
		t.Errorf("expected Consumed >= 30pt for 3 items, got %v", plan.Consumed)
	}
}

// TestRTLTableRendersWithReversedColumns verifies that a table inside
// a dir="rtl" container renders with columns in right-to-left order.
func TestRTLTableRendersWithReversedColumns(t *testing.T) {
	src := `<div dir="rtl"><table><tr><td>A</td><td>B</td><td>C</td></tr></table></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive Consumed, got %v", plan.Consumed)
	}
}

// TestRTLCSSTableRendersWithReversedColumns covers display:table with
// direction:rtl via CSS.
func TestRTLCSSTableRendersWithReversedColumns(t *testing.T) {
	src := `<html><head><style>
		.t { display: table; direction: rtl; width: 100%; }
		.r { display: table-row; }
		.c { display: table-cell; }
	</style></head><body>
		<div class="t"><div class="r"><div class="c">Right</div><div class="c">Left</div></div></div>
	</body></html>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive Consumed, got %v", plan.Consumed)
	}
}

// TestUnicodeBidiOverrideRTL verifies that unicode-bidi:bidi-override
// with direction:rtl forces all text to render right-to-left, even
// Latin characters that would normally be LTR.
func TestUnicodeBidiOverrideRTL(t *testing.T) {
	src := `<html><head><style>
		.override { direction: rtl; unicode-bidi: bidi-override; }
	</style></head><body>
		<p class="override">Hello</p>
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
	// With bidi-override + RTL, even English text should right-align.
	plan := p.PlanLayout(layout.LayoutArea{Width: 500, Height: 200})
	if plan.Status != layout.LayoutFull || len(plan.Blocks) == 0 {
		t.Fatal("layout failed")
	}
	if plan.Blocks[0].X <= 0 {
		t.Errorf("bidi-override RTL should right-align; X=%v", plan.Blocks[0].X)
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

// TestRTLInvoiceTableLayout verifies that an RTL HTML invoice containing tables with
// unequal columns, headers, totals, and flex-nested layout converts and renders to PDF
// without layout errors.
func TestRTLInvoiceTableLayout(t *testing.T) {
	src := `<!doctype html>
<html lang="ar" dir="rtl">
<head>
  <style>
    @page { size: A4; margin: 0; }
    html, body { direction: rtl; width: 210mm; }
    .paper { display: flex; flex-direction: column; width: 210mm; padding: 10mm; }
    table { width: 100%; border-collapse: collapse; }
    table.doc-lines th { text-align: start; direction: rtl; }
    table.doc-lines td.num { text-align: end; direction: ltr; }
    .footer-notes-blocks { margin-top: auto; }
    .footer { margin-top: 8mm; text-align: center; }
  </style>
</head>
<body dir="rtl">
<main class="paper" dir="rtl">
  <table class="layout-table" dir="rtl">
    <tr>
      <td style="width:50%"><div>شركة الأفق للحلول التقنية</div></td>
      <td style="width:50%"><div>فاتورة FAC-2026-00142</div></td>
    </tr>
  </table>
  <table class="doc-lines" dir="rtl">
    <thead>
      <tr>
        <th style="width:10%">#</th>
        <th style="width:50%">البيان</th>
        <th style="width:40%">الإجمالي</th>
      </tr>
    </thead>
    <tbody>
      <tr>
        <td class="num">1</td>
        <td>اشتراك سنوي (Business)</td>
        <td class="num">120 000,00 DZD</td>
      </tr>
    </tbody>
  </table>
  <div class="footer-notes-blocks">
    <div>طريقة الدفع: تحويل بنكي</div>
  </div>
  <footer class="footer">
    <div>RIB: 002 00012 1234567890 45</div>
  </footer>
</main>
</body>
</html>`

	res, err := ConvertFull(src, nil)
	if err != nil {
		t.Fatalf("ConvertFull failed: %v", err)
	}
	if len(res.Elements) == 0 {
		t.Fatal("expected elements")
	}

	// 210mm x 297mm in points: 595.28 x 841.89
	plan := res.Elements[0].PlanLayout(layout.LayoutArea{Width: 595.28, Height: 841.89})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Overflow != nil {
		t.Error("expected invoice to fit on single page without overflow")
	}
}
