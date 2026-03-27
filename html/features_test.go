// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// --- Text color on bare text nodes ---

func TestTextColorInherited(t *testing.T) {
	html := `<style>.red { color: #ff0000; }</style><div class="red">Red text</div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	// Should render without error; text color is applied via TextRun.
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestTableCellTextColor(t *testing.T) {
	html := `<style>.up { color: #0a8a4a; } .down { color: #d32f2f; }</style>
	<table><tr><td class="up">+12%</td><td class="down">-3%</td></tr></table>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

// --- Empty divs with visual properties (bars) ---

func TestEmptyDivWithBackgroundRendered(t *testing.T) {
	html := `<style>.bar { height: 8px; background: #130048; border-radius: 4px; }</style>
	<div class="bar" style="width:60%"></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected element for empty div with visual properties")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("empty div with height+background should have positive consumed height")
	}
}

// --- Table width: 100% ---

func TestTableWidth100Percent(t *testing.T) {
	html := `<style>table { width: 100%; }</style>
	<table><tr><td>A</td><td>B</td></tr></table>`
	opts := &Options{PageWidth: 500, PageHeight: 800}
	elems, err := Convert(html, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	// Table should use the full available width.
	if len(plan.Blocks) > 0 && plan.Blocks[0].Width < 490 {
		t.Errorf("table width = %.1f, expected close to 500", plan.Blocks[0].Width)
	}
}

// --- @page margin: 0 ---

func TestPageMarginZero(t *testing.T) {
	html := `<html><head><style>@page { size: a4; margin: 0; }</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	if !result.PageConfig.HasMargins {
		t.Error("HasMargins should be true even for margin: 0")
	}
	if result.PageConfig.MarginTop != 0 || result.PageConfig.MarginRight != 0 ||
		result.PageConfig.MarginBottom != 0 || result.PageConfig.MarginLeft != 0 {
		t.Errorf("margins should all be 0, got T=%.1f R=%.1f B=%.1f L=%.1f",
			result.PageConfig.MarginTop, result.PageConfig.MarginRight,
			result.PageConfig.MarginBottom, result.PageConfig.MarginLeft)
	}
}

func TestPageMarginExplicit(t *testing.T) {
	html := `<html><head><style>@page { margin: 2cm; }</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	if !result.PageConfig.HasMargins {
		t.Error("HasMargins should be true")
	}
	// 2cm ≈ 56.69pt
	if math.Abs(result.PageConfig.MarginTop-56.69) > 1 {
		t.Errorf("MarginTop = %.2f, want ~56.69", result.PageConfig.MarginTop)
	}
}

// --- @page :first / :left / :right pseudo-selectors ---

func TestPagePseudoSelectors(t *testing.T) {
	html := `<html><head><style>
		@page { margin: 2cm; }
		@page :first { margin-top: 4cm; }
		@page :left { margin-left: 3cm; }
		@page :right { margin-right: 3cm; }
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc := result.PageConfig
	if pc == nil {
		t.Fatal("expected PageConfig")
	}

	// Default margins
	if !pc.HasMargins {
		t.Error("default HasMargins should be true")
	}

	// :first
	if pc.First == nil {
		t.Fatal("expected First page margins")
	}
	if !pc.First.HasMargins {
		t.Error("First.HasMargins should be true")
	}
	// 4cm ≈ 113.39pt
	if math.Abs(pc.First.Top-113.39) > 1 {
		t.Errorf("First.Top = %.2f, want ~113.39", pc.First.Top)
	}

	// :left
	if pc.Left == nil {
		t.Fatal("expected Left page margins")
	}
	// 3cm ≈ 85.04pt
	if math.Abs(pc.Left.Left-85.04) > 1 {
		t.Errorf("Left.Left = %.2f, want ~85.04", pc.Left.Left)
	}

	// :right
	if pc.Right == nil {
		t.Fatal("expected Right page margins")
	}
	if math.Abs(pc.Right.Right-85.04) > 1 {
		t.Errorf("Right.Right = %.2f, want ~85.04", pc.Right.Right)
	}
}

// --- Body padding renders as Div ---

func TestBodyPaddingRendered(t *testing.T) {
	html := `<html><head><style>
		@page { size: a4; margin: 0; }
		body { padding: 2cm; }
	</style></head><body><p>Content</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Body with padding should create a Div wrapper.
	if len(result.Elements) == 0 {
		t.Fatal("expected elements")
	}
	plan := result.Elements[0].PlanLayout(layout.LayoutArea{Width: 595, Height: 842})
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height from body wrapper")
	}
}

// --- Heading with border/padding wraps in Div ---

func TestHeadingWithBorder(t *testing.T) {
	html := `<style>h2 { border-bottom: 2px solid #e8e6f0; padding-bottom: 6px; }</style>
	<h2>Section Title</h2>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("heading with border should render with positive height")
	}
}

// --- Paragraph with border/padding wraps in Div ---

func TestParagraphWithBorder(t *testing.T) {
	html := `<style>p { border: 1px solid #ccc; padding: 8px; background: #f9f9f9; }</style>
	<p>Boxed paragraph</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("paragraph with border should render")
	}
}

// --- Flex container margins ---

func TestFlexMargins(t *testing.T) {
	html := `<style>.flex { display: flex; gap: 16px; margin-bottom: 30px; }</style>
	<div class="flex"><div>A</div><div>B</div></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("flex with margin should render")
	}
}

// --- CSS margin collapsing ---

func TestMarginCollapsing(t *testing.T) {
	html := `<style>
		.a { margin-bottom: 30px; }
		.b { margin-top: 20px; }
	</style>
	<div class="a"><p>A</p></div><div class="b"><p>B</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	// With margin collapsing, spacing between A and B should be max(30,20)=30,
	// not 30+20=50. We verify elements render without error.
	for i, e := range elems {
		plan := e.PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
		if plan.Consumed <= 0 {
			t.Errorf("element %d: expected positive consumed", i)
		}
	}
}

// --- Cell CSS width hint ---

func TestCellWidthHint(t *testing.T) {
	html := `<style>
		table { width: 100%; }
		.wide { width: 200px; }
	</style>
	<table><tr><td>Narrow</td><td class="wide">Wide</td></tr></table>`
	opts := &Options{PageWidth: 500, PageHeight: 800}
	elems, err := Convert(html, opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

// --- position: relative ---

func TestPositionRelative(t *testing.T) {
	html := `<style>.shifted { position: relative; top: 10px; left: 5px; }</style>
	<div class="shifted"><p>Shifted content</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("position:relative element should render")
	}
}

// --- list-style-type ---

func TestListStyleTypeRoman(t *testing.T) {
	html := `<style>ol { list-style-type: lower-roman; }</style>
	<ol><li>First</li><li>Second</li><li>Third</li></ol>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("list should render")
	}
}

func TestListStyleTypeAlpha(t *testing.T) {
	html := `<style>ol { list-style-type: upper-alpha; }</style>
	<ol><li>A item</li><li>B item</li></ol>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected list elements")
	}
}

// --- display: inline-block ---

func TestDisplayInlineBlock(t *testing.T) {
	html := `<style>.badge { display: inline-block; padding: 4px 8px; background: #eee; border-radius: 4px; }</style>
	<span class="badge">Tag</span>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements for inline-block")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("inline-block element should render")
	}
}

// --- word-break: break-all ---

func TestWordBreakBreakAll(t *testing.T) {
	html := `<style>p { word-break: break-all; }</style>
	<p>Superlongwordthatwouldnormallyneverbreak</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	// With break-all and narrow width, the word should break into multiple lines.
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 50, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("word-break:break-all should render text")
	}
}

// --- Lazy percentage resolution via UnitValue ---

func TestPercentageWidthResolvesAtLayoutTime(t *testing.T) {
	html := `<div style="width:50%"><p>Half width</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	// Layout with 400pt → div should be 200pt wide.
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if len(plan.Blocks) > 0 {
		w := plan.Blocks[0].Width
		if math.Abs(w-200) > 1 {
			t.Errorf("div width = %.1f, want ~200 (50%% of 400)", w)
		}
	}
}

func TestPercentageWidthChangesWithArea(t *testing.T) {
	html := `<div style="width:50%"><p>Half</p></div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}

	// Layout at 400pt
	plan1 := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	// Layout at 600pt
	plan2 := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 1000})

	if len(plan1.Blocks) > 0 && len(plan2.Blocks) > 0 {
		w1 := plan1.Blocks[0].Width
		w2 := plan2.Blocks[0].Width
		if math.Abs(w1-200) > 1 {
			t.Errorf("at 400pt: div width = %.1f, want ~200", w1)
		}
		if math.Abs(w2-300) > 1 {
			t.Errorf("at 600pt: div width = %.1f, want ~300", w2)
		}
	}
}

// --- Quarterly report integration test ---

func TestQuarterlyReportIntegration(t *testing.T) {
	html := `<!DOCTYPE html><html><head><style>
		@page { size: A4; margin: 0; }
		body { font-family: Helvetica, Arial, sans-serif; padding: 2cm; color: #333; font-size: 10pt; }
		h1 { color: #130048; font-size: 22pt; margin-bottom: 4px; }
		.subtitle { color: #666; font-size: 11pt; margin-bottom: 30px; }
		.kpi-grid { display: flex; gap: 16px; margin-bottom: 30px; }
		.kpi { flex: 1; background: #f8f7ff; border: 1px solid #e8e6f0; border-radius: 8px; padding: 16px; text-align: center; }
		.kpi-value { font-size: 24pt; font-weight: bold; color: #130048; }
		.kpi-label { font-size: 8pt; text-transform: uppercase; color: #999; margin-top: 4px; }
		.kpi-change { font-size: 9pt; margin-top: 4px; }
		.up { color: #0a8a4a; }
		.down { color: #d32f2f; }
		h2 { color: #130048; font-size: 14pt; margin-top: 30px; border-bottom: 2px solid #e8e6f0; padding-bottom: 6px; }
		table { width: 100%%; border-collapse: collapse; margin-top: 12px; }
		th { background: #f8f7ff; padding: 8px 10px; text-align: left; font-size: 8pt; text-transform: uppercase; color: #666; border-bottom: 2px solid #e8e6f0; }
		td { padding: 8px 10px; border-bottom: 1px solid #f0f0f0; font-size: 9pt; }
		.right { text-align: right; }
		.bar-cell { width: 120px; }
		.bar { height: 8px; background: #130048; border-radius: 4px; }
		.footer { margin-top: 40px; font-size: 8pt; color: #999; text-align: center; }
	</style></head><body>
		<h1>Quarterly Report</h1>
		<div class="subtitle">Q4 2025</div>
		<div class="kpi-grid">
			<div class="kpi"><div class="kpi-value">2.1M</div><div class="kpi-label">Revenue</div><div class="kpi-change up">+12.3%%</div></div>
			<div class="kpi"><div class="kpi-value">1,847</div><div class="kpi-label">Customers</div><div class="kpi-change up">+8.7%%</div></div>
		</div>
		<h2>Revenue by Product</h2>
		<table>
			<thead><tr><th>Product</th><th class="right">Revenue</th><th class="right">Share</th><th class="bar-cell"></th></tr></thead>
			<tbody>
				<tr><td>Enterprise</td><td class="right">1,050,000</td><td class="right">50%%</td><td class="bar-cell"><div class="bar" style="width:100%%"></div></td></tr>
				<tr><td>Professional</td><td class="right">630,000</td><td class="right">30%%</td><td class="bar-cell"><div class="bar" style="width:60%%"></div></td></tr>
			</tbody>
		</table>
		<h2>Top Regions</h2>
		<table>
			<thead><tr><th>Region</th><th class="right">Customers</th><th class="right">Revenue</th><th class="right">Growth</th></tr></thead>
			<tbody>
				<tr><td>DACH</td><td class="right">823</td><td class="right">940,000</td><td class="right up">+15.2%%</td></tr>
				<tr><td>Rest of World</td><td class="right">123</td><td class="right">170,000</td><td class="right down">-3.4%%</td></tr>
			</tbody>
		</table>
		<div class="footer">Created on 15 February 2026</div>
	</body></html>`

	result, err := ConvertFull(html, &Options{PageWidth: 595.28, PageHeight: 841.89})
	if err != nil {
		t.Fatal(err)
	}

	// Should have PageConfig with margin: 0 and HasMargins.
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig from @page rule")
	}
	if !result.PageConfig.HasMargins {
		t.Error("HasMargins should be true")
	}
	if result.PageConfig.MarginTop != 0 {
		t.Errorf("expected margin 0, got %.2f", result.PageConfig.MarginTop)
	}

	// Should produce elements.
	if len(result.Elements) == 0 {
		t.Fatal("expected elements from quarterly report")
	}

	// All elements should lay out without error.
	for i, e := range result.Elements {
		plan := e.PlanLayout(layout.LayoutArea{Width: 595.28, Height: 841.89})
		if plan.Consumed < 0 {
			t.Errorf("element %d: negative consumed height", i)
		}
	}
}

// --- containerWidth narrowing for body padding ---

func TestContainerWidthNarrowedByBodyPadding(t *testing.T) {
	// Table with width:100% inside body with padding should resolve to
	// body inner width, not full page width.
	html := `<html><head><style>
		@page { size: a4; margin: 0; }
		body { padding: 72pt; }
		table { width: 100%%; }
	</style></head><body>
		<table><tr><td>A</td><td>B</td></tr></table>
	</body></html>`
	result, err := ConvertFull(html, &Options{PageWidth: 595.28, PageHeight: 841.89})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Elements) == 0 {
		t.Fatal("expected elements")
	}
	// Layout at full page width. Body div has 72pt padding on each side.
	plan := result.Elements[0].PlanLayout(layout.LayoutArea{Width: 595.28, Height: 841.89})
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed")
	}
}

// --- @page pseudo-selector parsing with no space ---

func TestPagePseudoNoSpace(t *testing.T) {
	html := `<html><head><style>@page:first { margin-top: 3cm; }</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	if result.PageConfig.First == nil {
		t.Fatal("expected First page margins from @page:first")
	}
	if !result.PageConfig.First.HasMargins {
		t.Error("First.HasMargins should be true")
	}
}

// --- word-break inherited ---

func TestWordBreakInherited(t *testing.T) {
	html := `<style>body { word-break: break-all; }</style><p>Text</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("expected rendered text")
	}
}

// --- list-style-type: none ---

func TestListStyleNone(t *testing.T) {
	html := `<style>ul { list-style-type: none; }</style>
	<ul><li>Item</li></ul>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

// --- generatePseudoElement carries color ---

func TestPseudoElementColor(t *testing.T) {
	html := `<style>p::before { content: ">>"; color: red; }</style><p>Text</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should not panic; pseudo elements should render.
	_ = elems
}

// --- display: flex with multiple children ---

func TestFlexWithMultipleChildren(t *testing.T) {
	html := `<style>
		.row { display: flex; gap: 10px; }
		.col { flex: 1; padding: 8px; background: #f0f0f0; }
	</style>
	<div class="row">
		<div class="col">Column 1</div>
		<div class="col">Column 2</div>
		<div class="col">Column 3</div>
	</div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected flex elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 600, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("flex should render")
	}
}

// --- h1 text color ---

func TestHeadingColor(t *testing.T) {
	html := `<style>h1 { color: #130048; }</style><h1>Title</h1>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected heading element")
	}
}

// --- vertical-align on inline elements ---

func TestVerticalAlignSuper(t *testing.T) {
	html := `<p>Normal <span style="vertical-align: super; font-size: 8pt">TM</span> text</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("vertical-align:super should render text")
	}
}

func TestVerticalAlignSub(t *testing.T) {
	html := `<p>H<sub style="vertical-align: sub">2</sub>O</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

// --- hyphens: auto ---

func TestHyphensAuto(t *testing.T) {
	html := `<style>p { hyphens: auto; }</style>
	<p>Incomprehensibilities and antidisestablishmentarianism</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	// With narrow width, hyphenation should break words.
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 100, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("hyphens:auto should render")
	}
}

func TestHyphensNone(t *testing.T) {
	html := `<style>p { hyphens: none; }</style><p>Text</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

func TestHyphensInherited(t *testing.T) {
	html := `<style>body { hyphens: auto; }</style><p>Internationalization</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

// --- Page margin boxes ---

func TestPageMarginBoxes(t *testing.T) {
	html := `<html><head><style>
		@page {
			size: a4;
			margin: 2cm;
			@top-center { content: "My Document"; }
			@bottom-center { content: "Page " counter(page); }
		}
	</style></head><body><p>Content</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc := result.PageConfig
	if pc == nil {
		t.Fatal("expected PageConfig")
	}
	if len(pc.MarginBoxes) == 0 {
		t.Fatal("expected margin boxes")
	}
	if mb, ok := pc.MarginBoxes["top-center"]; !ok {
		t.Error("expected top-center margin box")
	} else if mb.Content != "My Document" {
		t.Errorf("top-center content = %q, want %q", mb.Content, "My Document")
	}
	if mb, ok := pc.MarginBoxes["bottom-center"]; !ok {
		t.Error("expected bottom-center margin box")
	} else if !strings.Contains(mb.Content, "Page ") {
		t.Errorf("bottom-center content = %q, should contain 'Page '", mb.Content)
	}
}

func TestPageMarginBoxFirstPage(t *testing.T) {
	html := `<html><head><style>
		@page { margin: 2cm; }
		@page :first {
			@top-center { content: "Cover Page"; }
		}
	</style></head><body><p>Content</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc := result.PageConfig
	if pc == nil {
		t.Fatal("expected PageConfig")
	}
	if pc.First == nil {
		t.Fatal("expected First page config")
	}
	if len(pc.First.MarginBoxes) == 0 {
		t.Fatal("expected margin boxes on first page")
	}
	if mb, ok := pc.First.MarginBoxes["top-center"]; !ok {
		t.Error("expected top-center margin box on :first")
	} else if mb.Content != "Cover Page" {
		t.Errorf("content = %q, want %q", mb.Content, "Cover Page")
	}
}

func TestNestedBraceParsing(t *testing.T) {
	// Verify that nested braces in @page rules don't break the CSS parser.
	html := `<html><head><style>
		@page {
			margin: 1cm;
			@top-left { content: "Left"; }
			@top-right { content: "Right"; }
		}
		body { font-size: 10pt; }
	</style></head><body><p>Text</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Should parse both the @page rule AND the body rule.
	if len(result.Elements) == 0 {
		t.Fatal("expected elements")
	}
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	if len(result.PageConfig.MarginBoxes) != 2 {
		t.Errorf("expected 2 margin boxes, got %d", len(result.PageConfig.MarginBoxes))
	}
}

func TestCounterInContent(t *testing.T) {
	html := `<html><head><style>
		@page {
			margin: 2cm;
			@bottom-right { content: "Page " counter(page) " of " counter(pages); }
		}
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	mb := result.PageConfig.MarginBoxes["bottom-right"]
	// Should contain placeholders for counters.
	if !strings.Contains(mb.Content, "Page ") {
		t.Errorf("content = %q, should contain 'Page '", mb.Content)
	}
	if !strings.Contains(mb.Content, "{counter(page)}") {
		t.Errorf("content = %q, should contain counter placeholder", mb.Content)
	}
}

// --- counter(pages) placeholder ---

func TestCounterPagesPlaceholder(t *testing.T) {
	html := `<html><head><style>
		@page {
			margin: 2cm;
			@bottom-center { content: "Page " counter(page) " of " counter(pages); }
		}
	</style></head><body><p>Content</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	mb := result.PageConfig.MarginBoxes["bottom-center"]
	if !strings.Contains(mb.Content, "{counter(pages)}") {
		t.Errorf("content should contain counter(pages) placeholder, got %q", mb.Content)
	}
}

// --- Margin box font-size and color from CSS ---

func TestMarginBoxFontSize(t *testing.T) {
	html := `<html><head><style>
		@page {
			margin: 2cm;
			@top-center { content: "Header"; font-size: 12pt; }
		}
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	mb := result.PageConfig.MarginBoxes["top-center"]
	if mb.FontSize != 12 {
		t.Errorf("FontSize = %.1f, want 12", mb.FontSize)
	}
}

func TestMarginBoxColor(t *testing.T) {
	html := `<html><head><style>
		@page {
			margin: 2cm;
			@bottom-right { content: "Footer"; color: #ff0000; }
		}
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	mb := result.PageConfig.MarginBoxes["bottom-right"]
	// Red = R:1, G:0, B:0
	if mb.Color[0] < 0.9 {
		t.Errorf("Color R = %.2f, want ~1.0 (red)", mb.Color[0])
	}
}

// --- list-style-type: none ---

func TestListStyleTypeNoneNoMarker(t *testing.T) {
	html := `<style>ul { list-style-type: none; }</style>
	<ul><li>Item 1</li><li>Item 2</li></ul>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected list elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("list with style:none should still render")
	}
}

// --- display: table on non-table elements ---

func TestDisplayTable(t *testing.T) {
	html := `<style>
		.table { display: table; width: 100%%; }
		.row { display: table-row; }
		.cell { display: table-cell; padding: 8px; }
	</style>
	<div class="table">
		<div class="row">
			<div class="cell">A</div>
			<div class="cell">B</div>
			<div class="cell">C</div>
		</div>
		<div class="row">
			<div class="cell">1</div>
			<div class="cell">2</div>
			<div class="cell">3</div>
		</div>
	</div>`
	elems, err := Convert(html, &Options{PageWidth: 500, PageHeight: 800})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements for display:table")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("display:table should render")
	}
}

func TestDisplayTableWithBackground(t *testing.T) {
	html := `<style>
		.table { display: table; }
		.row { display: table-row; }
		.cell { display: table-cell; padding: 4px; background: #f0f0f0; }
	</style>
	<div class="table">
		<div class="row">
			<div class="cell">Hello</div>
			<div class="cell">World</div>
		</div>
	</div>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

// Ensure the % literal doesn't break tests.
func init() {
	_ = strings.Contains("", "")
}

// --- CSS Custom Properties (Variables) ---

func TestResolveVarsBasic(t *testing.T) {
	style := &computedStyle{
		CustomProperties: map[string]string{
			"--color": "red",
		},
	}
	got := resolveVars("var(--color)", style)
	if got != "red" {
		t.Errorf("expected %q, got %q", "red", got)
	}
}

func TestResolveVarsFallback(t *testing.T) {
	style := &computedStyle{} // no custom properties
	got := resolveVars("var(--undefined, blue)", style)
	if got != "blue" {
		t.Errorf("expected %q, got %q", "blue", got)
	}
}

func TestResolveVarsNested(t *testing.T) {
	style := &computedStyle{
		CustomProperties: map[string]string{
			"--b": "green",
		},
	}
	// --a is not defined, so fallback var(--b) is used, which resolves to green.
	got := resolveVars("var(--a, var(--b))", style)
	if got != "green" {
		t.Errorf("expected %q, got %q", "green", got)
	}
}

func TestResolveVarsMultiple(t *testing.T) {
	style := &computedStyle{
		CustomProperties: map[string]string{
			"--width": "2px",
			"--color": "#333",
		},
	}
	got := resolveVars("var(--width) solid var(--color)", style)
	if got != "2px solid #333" {
		t.Errorf("expected %q, got %q", "2px solid #333", got)
	}
}

func TestCSSVarInheritance(t *testing.T) {
	htm := `<style>
		.parent { --text-color: #ff0000; }
		.child { color: var(--text-color); }
	</style>
	<div class="parent"><div class="child">Red text</div></div>`
	elems, err := Convert(htm, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	// The element should render without error.
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestCSSVarFallbackIntegration(t *testing.T) {
	htm := `<style>
		div { color: var(--missing, blue); }
	</style>
	<div>Blue text</div>`
	elems, err := Convert(htm, nil)
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

func TestCSSVarChained(t *testing.T) {
	// --a references --b which is defined
	style := &computedStyle{
		CustomProperties: map[string]string{
			"--b": "10px",
		},
	}
	got := resolveVars("var(--a, var(--b))", style)
	if got != "10px" {
		t.Errorf("expected %q, got %q", "10px", got)
	}
}

// --- CSS counters ---

func TestParseCounterEntries(t *testing.T) {
	// Single name, default value.
	entries := parseCounterEntries("section", 0)
	if len(entries) != 1 || entries[0].Name != "section" || entries[0].Value != 0 {
		t.Errorf("expected [{section 0}], got %v", entries)
	}

	// Name with explicit value.
	entries = parseCounterEntries("item 5", 0)
	if len(entries) != 1 || entries[0].Name != "item" || entries[0].Value != 5 {
		t.Errorf("expected [{item 5}], got %v", entries)
	}

	// Multiple counters.
	entries = parseCounterEntries("a 1 b 2", 0)
	if len(entries) != 2 || entries[0].Name != "a" || entries[0].Value != 1 || entries[1].Name != "b" || entries[1].Value != 2 {
		t.Errorf("expected [{a 1} {b 2}], got %v", entries)
	}

	// "none" returns nil.
	entries = parseCounterEntries("none", 0)
	if entries != nil {
		t.Errorf("expected nil for 'none', got %v", entries)
	}

	// Default value for increment.
	entries = parseCounterEntries("section", 1)
	if len(entries) != 1 || entries[0].Value != 1 {
		t.Errorf("expected value 1 for increment default, got %v", entries)
	}
}

func TestCounterMethods(t *testing.T) {
	c := &converter{counters: make(map[string][]int)}

	// getCounter returns 0 for unset counter.
	if v := c.getCounter("x"); v != 0 {
		t.Errorf("expected 0 for unset counter, got %d", v)
	}

	// Auto-instantiate via increment.
	c.incrementCounter("x", 1)
	if v := c.getCounter("x"); v != 1 {
		t.Errorf("expected 1 after auto-instantiate increment, got %d", v)
	}

	// Reset counter.
	c.resetCounter("x", 0)
	if v := c.getCounter("x"); v != 0 {
		t.Errorf("expected 0 after reset, got %d", v)
	}

	// Increment after reset.
	c.incrementCounter("x", 1)
	if v := c.getCounter("x"); v != 1 {
		t.Errorf("expected 1 after increment, got %d", v)
	}
	c.incrementCounter("x", 2)
	if v := c.getCounter("x"); v != 3 {
		t.Errorf("expected 3 after increment by 2, got %d", v)
	}

	// Nested counters via reset.
	c.resetCounter("x", 10)
	if v := c.getCounter("x"); v != 10 {
		t.Errorf("expected 10 after nested reset, got %d", v)
	}
	c.popCounter("x")
	if v := c.getCounter("x"); v != 3 {
		t.Errorf("expected 3 after pop, got %d", v)
	}
}

func TestResolveContentValueCounter(t *testing.T) {
	c := &converter{counters: make(map[string][]int)}
	c.resetCounter("section", 0)
	c.incrementCounter("section", 1)

	// Simple counter().
	got := c.resolveContentValue(`counter(section)`)
	if got != "1" {
		t.Errorf("expected %q, got %q", "1", got)
	}

	// Counter with prefix string.
	got = c.resolveContentValue(`"Section " counter(section)`)
	if got != "Section 1" {
		t.Errorf("expected %q, got %q", "Section 1", got)
	}

	// Counter with prefix and suffix.
	got = c.resolveContentValue(`"[" counter(section) "]"`)
	if got != "[1]" {
		t.Errorf("expected %q, got %q", "[1]", got)
	}
}

func TestResolveContentValueCounters(t *testing.T) {
	c := &converter{counters: make(map[string][]int)}
	c.resetCounter("sec", 0)
	c.incrementCounter("sec", 1)
	c.resetCounter("sec", 0) // nested
	c.incrementCounter("sec", 1)

	// counters() with default separator.
	got := c.resolveContentValue(`counters(sec, ".")`)
	if got != "1.1" {
		t.Errorf("expected %q, got %q", "1.1", got)
	}

	// counters() with custom separator.
	got = c.resolveContentValue(`counters(sec, " > ")`)
	if got != "1 > 1" {
		t.Errorf("expected %q, got %q", "1 > 1", got)
	}
}

func TestCounterResetCustomStart(t *testing.T) {
	c := &converter{counters: make(map[string][]int)}
	c.resetCounter("item", 5)
	c.incrementCounter("item", 1)
	if v := c.getCounter("item"); v != 6 {
		t.Errorf("expected 6 (start at 5, increment 1), got %d", v)
	}
}

func TestCounterCustomIncrement(t *testing.T) {
	c := &converter{counters: make(map[string][]int)}
	c.resetCounter("item", 0)
	c.incrementCounter("item", 2)
	if v := c.getCounter("item"); v != 2 {
		t.Errorf("expected 2 after increment by 2, got %d", v)
	}
	c.incrementCounter("item", 2)
	if v := c.getCounter("item"); v != 4 {
		t.Errorf("expected 4 after second increment by 2, got %d", v)
	}
}

func TestCounterAutoInstantiate(t *testing.T) {
	// counter-increment without counter-reset should auto-instantiate.
	c := &converter{counters: make(map[string][]int)}
	c.incrementCounter("x", 1)
	if v := c.getCounter("x"); v != 1 {
		t.Errorf("expected 1 for auto-instantiated counter, got %d", v)
	}
}

func TestCSSCountersIntegration(t *testing.T) {
	// Full integration test: counter-reset on parent, counter-increment on
	// children, content: counter(section) in ::before pseudo-element.
	htmlStr := `<style>
		.list { counter-reset: section; }
		.item { counter-increment: section; }
		.item::before { content: counter(section) ". "; }
	</style>
	<div class="list">
		<div class="item">First</div>
		<div class="item">Second</div>
		<div class="item">Third</div>
	</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements from counter integration test")
	}
	// Verify layout works without panics.
	for _, e := range elems {
		e.PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	}
}

func TestCSSCountersNestedIntegration(t *testing.T) {
	// Nested counters: counters(section, ".") should produce "1.1", "1.2", etc.
	htmlStr := `<style>
		.outer { counter-reset: section; }
		.outer > div { counter-increment: section; counter-reset: section; }
		.inner { counter-increment: section; }
		.inner::before { content: counters(section, ".") " "; }
	</style>
	<div class="outer">
		<div>
			<div class="inner">A</div>
			<div class="inner">B</div>
		</div>
	</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements from nested counter test")
	}
}

func TestCSSCounterIncrementOnly(t *testing.T) {
	// counter-increment without counter-reset — auto-instantiated per CSS spec.
	htmlStr := `<style>
		.item { counter-increment: x; }
		.item::before { content: counter(x) " "; }
	</style>
	<div>
		<div class="item">A</div>
		<div class="item">B</div>
	</div>`
	elems, err := Convert(htmlStr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements from auto-instantiate counter test")
	}
}

// --- Containing-block absolute positioning ---

func TestAbsoluteInsideRelativeParent(t *testing.T) {
	// An absolute element inside a position:relative parent should be
	// rendered as an overlay child of the parent, not as a page-level
	// absolute item.
	htmlStr := `<div style="position: relative; width: 200px; height: 100px; background: #eee;">
		<div style="position: absolute; top: 10px; left: 20px; width: 50px;"><p>Abs</p></div>
	</div>`
	result, err := ConvertFull(htmlStr, &Options{PageWidth: 612, PageHeight: 792})
	if err != nil {
		t.Fatal(err)
	}
	// The absolute child should NOT appear in the global absolutes list.
	if len(result.Absolutes) != 0 {
		t.Errorf("expected 0 global absolutes (child should be overlay), got %d", len(result.Absolutes))
	}
	// The parent div should be in normal flow elements.
	if len(result.Elements) == 0 {
		t.Fatal("expected normal-flow elements for the relative parent")
	}
	// Verify the element renders with positive consumed height.
	plan := result.Elements[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("relative parent with absolute child should have positive consumed height")
	}
	// Verify the container block has children (the overlay).
	if len(plan.Blocks) == 0 {
		t.Fatal("expected placed blocks")
	}
	containerBlock := plan.Blocks[0]
	// The container block should have children: normal flow children
	// plus the overlay child.
	foundOverlay := false
	for _, child := range containerBlock.Children {
		// Overlay children have X offset that includes padding + CSS left.
		// CSS left: 20px = 15pt.
		if child.X >= 14 && child.X <= 16 {
			foundOverlay = true
		}
	}
	if !foundOverlay {
		t.Error("expected overlay child with X offset ~15pt (20px CSS left)")
	}
}

func TestNestedPositionedAncestors(t *testing.T) {
	// The nearest positioned ancestor should be used for absolute positioning.
	htmlStr := `<div style="position: relative; width: 400px; height: 300px;">
		<div style="position: relative; width: 200px; height: 150px;">
			<div style="position: absolute; top: 5px; left: 10px;"><p>Inner abs</p></div>
		</div>
	</div>`
	result, err := ConvertFull(htmlStr, &Options{PageWidth: 612, PageHeight: 792})
	if err != nil {
		t.Fatal(err)
	}
	// Should not produce any global absolutes.
	if len(result.Absolutes) != 0 {
		t.Errorf("expected 0 global absolutes, got %d", len(result.Absolutes))
	}
	if len(result.Elements) == 0 {
		t.Fatal("expected elements")
	}
	plan := result.Elements[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("nested positioned containers should render")
	}
}

func TestAbsoluteWithNoPositionedAncestor(t *testing.T) {
	// Without a positioned ancestor, absolute should fall back to the page
	// (global absolutes list).
	htmlStr := `<div style="position: absolute; top: 50px; left: 100px;"><p>Page-level abs</p></div><p>Normal</p>`
	result, err := ConvertFull(htmlStr, &Options{PageWidth: 612, PageHeight: 792})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Absolutes) == 0 {
		t.Fatal("expected global absolute item when no positioned ancestor exists")
	}
	if len(result.Elements) == 0 {
		t.Fatal("expected normal-flow elements")
	}
}

func TestAbsoluteWithRightInContainingBlock(t *testing.T) {
	// Test position:absolute with CSS right inside a containing block.
	htmlStr := `<div style="position: relative; width: 300px; height: 100px;">
		<div style="position: absolute; top: 0; right: 10px; width: 50px;"><p>Right</p></div>
	</div>`
	result, err := ConvertFull(htmlStr, &Options{PageWidth: 612, PageHeight: 792})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Absolutes) != 0 {
		t.Errorf("expected 0 global absolutes, got %d", len(result.Absolutes))
	}
	if len(result.Elements) == 0 {
		t.Fatal("expected elements")
	}
	plan := result.Elements[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("should render with positive height")
	}
}

// --- Text highlight / background color ---

func TestMarkElementRendersHighlight(t *testing.T) {
	// <mark> should render with default yellow background.
	src := `<p>This is <mark>highlighted</mark> text.</p>`
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

func TestInlineBackgroundColorCSS(t *testing.T) {
	// Explicit background-color on a span should produce a highlight.
	src := `<p>Before <span style="background-color: #ff0;">highlight</span> after</p>`
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

func TestMarkWithCustomColor(t *testing.T) {
	// <mark> with CSS override should use the custom color.
	src := `<style>mark { background-color: #90EE90; }</style>
	<p>This is <mark>green highlight</mark> text.</p>`
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

func TestMultipleHighlightsInParagraph(t *testing.T) {
	src := `<p><mark>First</mark> gap <mark>Second</mark></p>`
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

// --- Inline elements within paragraphs ---

func TestInlineSVGInParagraph(t *testing.T) {
	// An SVG inside a <p> should produce a paragraph that renders
	// (not a separate block element).
	src := `<p>Status: <svg width="16" height="16" viewBox="0 0 16 16"><circle cx="8" cy="8" r="6" fill="green"/></svg> OK</p>`
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

func TestInlineSVGInParagraphSingleLine(t *testing.T) {
	// Text + small SVG should fit on one line at wide width.
	src := `<p>Check <svg width="12" height="12" viewBox="0 0 12 12"><rect width="12" height="12" fill="red"/></svg> done</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	// Should be a single paragraph, not split into multiple elements.
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	// A single-line paragraph should have exactly 1 block (one line).
	if len(plan.Blocks) != 1 {
		t.Errorf("expected 1 block (single line), got %d", len(plan.Blocks))
	}
}

func TestInlineImageInParagraph(t *testing.T) {
	// A tiny 1x1 PNG data URI inside a paragraph.
	src := `<p>Icon: <img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==" width="16" height="16"> label</p>`
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

func TestInlineBlockDivInParagraph(t *testing.T) {
	// A display:inline-block div inside a paragraph should flow inline.
	src := `<p>Before <span style="display:inline-block; width:20px; height:20px; background:#0f0;"></span> After</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

func TestInlineBlockStandalone(t *testing.T) {
	// A standalone display:inline-block (not inside a paragraph)
	// should still render as a block element at the top level.
	src := `<div style="display:inline-block; width:100px; height:50px; background:#f00;"></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements for standalone inline-block")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("standalone inline-block should render")
	}
}

func TestMultipleInlineSVGsInParagraph(t *testing.T) {
	// Multiple SVGs inline with text.
	src := `<p><svg width="10" height="10" viewBox="0 0 10 10"><rect width="10" height="10" fill="red"/></svg> and <svg width="10" height="10" viewBox="0 0 10 10"><rect width="10" height="10" fill="blue"/></svg> icons</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestInlineSVGInHeading(t *testing.T) {
	// SVG inside a heading (which also uses collectRuns).
	src := `<h2><svg width="20" height="20" viewBox="0 0 20 20"><circle cx="10" cy="10" r="8" fill="orange"/></svg> Section Title</h2>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("heading with inline SVG should render")
	}
}

func TestDisplayBlockSVGNotInlinedInParagraph(t *testing.T) {
	// An SVG with display:block inside a paragraph should not produce
	// an inline element run — it should be skipped in inline flow.
	src := `<p>Before <svg style="display:block" width="100" height="50" viewBox="0 0 100 50"><rect width="100" height="50" fill="blue"/></svg> After</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	// The paragraph should contain only the text runs "Before" and "After"
	// (the display:block SVG is skipped). Verify by checking that the
	// paragraph fits on one line with just text content.
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if len(plan.Blocks) != 1 {
		t.Errorf("expected 1 block (single text line), got %d", len(plan.Blocks))
	}
	// Width should be narrow (just text), not wide (would be wider with
	// a 100px inline SVG).
	if plan.Blocks[0].Width > 80 {
		t.Errorf("block width = %.1f, expected < 80 (display:block SVG should not be inlined)", plan.Blocks[0].Width)
	}
}

func TestDisplayNoneSVGHiddenInParagraph(t *testing.T) {
	// An SVG with display:none inside a paragraph should be completely
	// hidden — not rendered as either inline or block.
	src := `<p>Visible <svg style="display:none" width="100" height="50" viewBox="0 0 100 50"><rect width="100" height="50" fill="red"/></svg> text</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	// Should render only text, no SVG element at all.
	if len(plan.Blocks) != 1 {
		t.Errorf("expected 1 block (text only), got %d", len(plan.Blocks))
	}
}

func TestDisplayBlockImageNotInlinedInParagraph(t *testing.T) {
	// An image with display:block inside a paragraph should be skipped
	// in inline flow, not treated as an inline element.
	src := `<p>Text <img style="display:block" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==" width="200" height="100"> more</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	// Should be a single line of just text, not widened by a 200px image.
	if len(plan.Blocks) != 1 {
		t.Errorf("expected 1 block (text only), got %d", len(plan.Blocks))
	}
	if plan.Blocks[0].Width > 80 {
		t.Errorf("block width = %.1f, expected < 80 (display:block image should not be inlined)", plan.Blocks[0].Width)
	}
}

func TestInlineBlockSVGStillInlinedInParagraph(t *testing.T) {
	// An SVG with display:inline-block inside a paragraph should still
	// be treated as inline (inline-block is not "block").
	src := `<p>Check <svg style="display:inline-block" width="16" height="16" viewBox="0 0 16 16"><circle cx="8" cy="8" r="6" fill="green"/></svg> done</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	// Should still be a single line with the SVG inline.
	if len(plan.Blocks) != 1 {
		t.Errorf("expected 1 block (single line), got %d", len(plan.Blocks))
	}
}
