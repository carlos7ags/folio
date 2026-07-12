// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"encoding/base64"
	"math"
	"os"
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

// TestPagePseudoSelectorCaseInsensitive verifies that page pseudo names are
// matched case-insensitively (CSS page pseudo names are case-insensitive), so
// `@page :First` is honoured exactly like `@page :first`.
//
// Fail-before/pass-after: before the fix `sel` was compared against lowercase
// literals without lowercasing, so `:First` fell into the default case and was
// dropped — pc.First would be nil and its margins lost.
func TestPagePseudoSelectorCaseInsensitive(t *testing.T) {
	html := `<html><head><style>
		@page { margin: 2cm; }
		@page :First { margin-top: 4cm; }
		@page :LEFT { margin-left: 3cm; }
		@page :Right { margin-right: 3cm; }
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc := result.PageConfig
	if pc == nil {
		t.Fatal("expected PageConfig")
	}

	// :First (capitalized) must be honoured.
	if pc.First == nil {
		t.Fatal("expected First page margins from @page :First")
	}
	// 4cm ≈ 113.39pt
	if math.Abs(pc.First.Top-113.39) > 1 {
		t.Errorf("First.Top = %.2f, want ~113.39 (4cm)", pc.First.Top)
	}

	// :LEFT must be honoured.
	if pc.Left == nil {
		t.Fatal("expected Left page margins from @page :LEFT")
	}
	if math.Abs(pc.Left.Left-85.04) > 1 {
		t.Errorf("Left.Left = %.2f, want ~85.04 (3cm)", pc.Left.Left)
	}

	// :Right must be honoured.
	if pc.Right == nil {
		t.Fatal("expected Right page margins from @page :Right")
	}
	if math.Abs(pc.Right.Right-85.04) > 1 {
		t.Errorf("Right.Right = %.2f, want ~85.04 (3cm)", pc.Right.Right)
	}
}

// TestPageFirstMarginsMergeOverBase is the Defect B regression: a partial
// `@page :first { margin-top: 4cm }` over a base `@page { margin: 2cm }` must
// yield top=4cm AND right/bottom/left=2cm (inherited base), not 0.
func TestPageFirstMarginsMergeOverBase(t *testing.T) {
	html := `<html><head><style>
		@page { margin: 2cm; }
		@page :first { margin-top: 4cm; }
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc := result.PageConfig
	if pc == nil || pc.First == nil {
		t.Fatal("expected PageConfig with First margins")
	}
	// 4cm ≈ 113.39pt, 2cm ≈ 56.69pt.
	if math.Abs(pc.First.Top-113.39) > 1 {
		t.Errorf("First.Top = %.2f, want ~113.39 (4cm)", pc.First.Top)
	}
	if math.Abs(pc.First.Right-56.69) > 1 {
		t.Errorf("First.Right = %.2f, want ~56.69 (2cm inherited from base, not 0)", pc.First.Right)
	}
	if math.Abs(pc.First.Bottom-56.69) > 1 {
		t.Errorf("First.Bottom = %.2f, want ~56.69 (2cm inherited from base, not 0)", pc.First.Bottom)
	}
	if math.Abs(pc.First.Left-56.69) > 1 {
		t.Errorf("First.Left = %.2f, want ~56.69 (2cm inherited from base, not 0)", pc.First.Left)
	}
}

// TestPageLeftRightMarginsMergeOverBase verifies :left/:right partial margins
// also inherit the base @page {} margins for unspecified sides.
func TestPageLeftRightMarginsMergeOverBase(t *testing.T) {
	html := `<html><head><style>
		@page { margin: 2cm; }
		@page :left { margin-left: 3cm; }
		@page :right { margin-right: 3cm; }
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc := result.PageConfig
	if pc == nil || pc.Left == nil || pc.Right == nil {
		t.Fatal("expected PageConfig with Left and Right margins")
	}
	// :left margin-left = 3cm (~85.04), other sides inherit 2cm (~56.69).
	if math.Abs(pc.Left.Left-85.04) > 1 {
		t.Errorf("Left.Left = %.2f, want ~85.04 (3cm)", pc.Left.Left)
	}
	if math.Abs(pc.Left.Top-56.69) > 1 {
		t.Errorf("Left.Top = %.2f, want ~56.69 (2cm inherited)", pc.Left.Top)
	}
	if math.Abs(pc.Left.Right-56.69) > 1 {
		t.Errorf("Left.Right = %.2f, want ~56.69 (2cm inherited)", pc.Left.Right)
	}
	// :right margin-right = 3cm, other sides inherit 2cm.
	if math.Abs(pc.Right.Right-85.04) > 1 {
		t.Errorf("Right.Right = %.2f, want ~85.04 (3cm)", pc.Right.Right)
	}
	if math.Abs(pc.Right.Left-56.69) > 1 {
		t.Errorf("Right.Left = %.2f, want ~56.69 (2cm inherited)", pc.Right.Left)
	}
}

// TestPagePseudoBeforeBaseStillMerges verifies the seeding works even when the
// :first rule appears in source BEFORE the base @page {} rule (two-pass fix).
func TestPagePseudoBeforeBaseStillMerges(t *testing.T) {
	html := `<html><head><style>
		@page :first { margin-top: 4cm; }
		@page { margin: 2cm; }
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc := result.PageConfig
	if pc == nil || pc.First == nil {
		t.Fatal("expected PageConfig with First margins")
	}
	if math.Abs(pc.First.Top-113.39) > 1 {
		t.Errorf("First.Top = %.2f, want ~113.39 (4cm)", pc.First.Top)
	}
	if math.Abs(pc.First.Left-56.69) > 1 {
		t.Errorf("First.Left = %.2f, want ~56.69 (2cm inherited, source order independent)", pc.First.Left)
	}
}

// TestPageNamedDoesNotPolluteDefault is the Defect C regression: a NAMED page
// (`@page narrow { margin: 5cm }`) must NOT become the global default rule.
func TestPageNamedDoesNotPolluteDefault(t *testing.T) {
	html := `<html><head><style>
		@page { margin: 2cm; }
		@page narrow { margin: 5cm; }
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc := result.PageConfig
	if pc == nil {
		t.Fatal("expected PageConfig")
	}
	// 2cm ≈ 56.69pt; must NOT be 5cm (~141.73pt) from the named page.
	if math.Abs(pc.MarginTop-56.69) > 1 {
		t.Errorf("default MarginTop = %.2f, want ~56.69 (named page must not pollute default)", pc.MarginTop)
	}
}

// TestPageNamedAloneNoDefaultMargins verifies a named page on its own does not
// set any default margins (HasMargins stays false).
func TestPageNamedAloneNoDefaultMargins(t *testing.T) {
	html := `<html><head><style>
		@page narrow { margin: 5cm; }
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageConfig != nil && result.PageConfig.HasMargins {
		t.Errorf("named-only page set default HasMargins=true with MarginTop=%.2f, want no default margins", result.PageConfig.MarginTop)
	}
}

// TestPageBlankDoesNotPolluteDefault verifies an unsupported pseudo (:blank)
// does not become the global default rule.
func TestPageBlankDoesNotPolluteDefault(t *testing.T) {
	html := `<html><head><style>
		@page { margin: 2cm; }
		@page :blank { margin: 5cm; }
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc := result.PageConfig
	if pc == nil {
		t.Fatal("expected PageConfig")
	}
	if math.Abs(pc.MarginTop-56.69) > 1 {
		t.Errorf("default MarginTop = %.2f, want ~56.69 (:blank must not pollute default)", pc.MarginTop)
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

// TestOrderedListStartAttribute verifies that <ol start="N"> is honored: the
// converted list renders and carries the start offset. Cross-page numbering
// continuation for a started list is covered at the layout level by
// TestOrderedListMultiPageRespectsStart.
func TestOrderedListStartAttribute(t *testing.T) {
	html := `<ol start="5"><li>First</li><li>Second</li><li>Third</li></ol>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected list elements")
	}
	list, ok := elems[0].(*layout.List)
	if !ok {
		t.Fatalf("expected first element to be *layout.List, got %T", elems[0])
	}
	// The wiring must carry the start="5" through to the list so the first
	// marker is numbered 5 rather than restarting at 1.
	if got := list.Start(); got != 5 {
		t.Errorf("ordered list start = %d, want 5 (from <ol start=\"5\">)", got)
	}
	plan := list.PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("ordered list with start should render")
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

// --- white-space: nowrap ---

// TestWhiteSpaceNowrapDoesNotBreakLongWord is the regression test asserting
// that an unbreakable word under white-space:nowrap renders as a single
// overflowing line, not character-broken across multiple lines the way a
// normal (wrapping) long word would be.
func TestWhiteSpaceNowrapDoesNotBreakLongWord(t *testing.T) {
	html := `<style>p { white-space: nowrap; }</style>
	<p>Superlongwordthatwouldnormallyneverbreak</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 50, Height: 1000})
	if len(plan.Blocks) != 1 {
		t.Errorf("white-space:nowrap should keep the word on one overflowing line, got %d blocks", len(plan.Blocks))
	}
}

// TestWhiteSpaceNowrapDoesNotSoftWrapAtSpaces asserts nowrap also suppresses
// ordinary space-boundary wrapping, not just character-breaking of a single
// unbreakable token — a multi-word nowrap run must stay on one line.
func TestWhiteSpaceNowrapDoesNotSoftWrapAtSpaces(t *testing.T) {
	html := `<style>p { white-space: nowrap; }</style>
	<p>hello world this is nowrap text</p>`
	elems, err := Convert(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 50, Height: 1000})
	if len(plan.Blocks) != 1 {
		t.Errorf("white-space:nowrap should keep the whole run on one overflowing line, got %d blocks", len(plan.Blocks))
	}
}

// TestWhiteSpaceNowrapAcrossFonts is a fast (no Chrome/poppler needed)
// regression guard across multiple font families and both TTF/glyf and
// OTF/CFF outlines, so the nowrap fix isn't accidentally coupled to Poppins
// specifically. No box-model properties are set on the element (no width,
// no background) so it stays a bare Paragraph rather than being wrapped in
// a Div — a Div's PlanLayout reports 1 top-level block regardless of how
// many lines its inner paragraph wrapped into, which would make this
// assertion pass even without the fix.
func TestWhiteSpaceNowrapAcrossFonts(t *testing.T) {
	files := []string{
		"NimbusSans-Bold.otf",
		"Inter-Bold.otf",
		"NotoSans-Bold.ttf",
		"OpenSans-Bold.ttf",
		"Roboto-Bold.ttf",
	}
	for _, name := range files {
		t.Run(name, func(t *testing.T) {
			ttfData, err := os.ReadFile("../font/testdata/" + name)
			if err != nil {
				t.Fatal(err)
			}
			b64 := base64.StdEncoding.EncodeToString(ttfData)

			src := `<html><head><style>
			@font-face {
				font-family: 'TestFont';
				src: url(data:font/truetype;base64,` + b64 + `) format('truetype');
				font-weight: bold;
			}
			p { font-family: 'TestFont'; font-weight: bold; font-size: 16px; white-space: nowrap; }
		</style></head><body>
			<p>ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789</p>
		</body></html>`

			elems, err := Convert(src, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(elems) == 0 {
				t.Fatal("expected elements")
			}
			// The unbroken token measures ~270-295pt across these fonts at
			// this font-size; constrain to well below that so it must
			// genuinely overflow (not just fit) to exercise the nowrap
			// guard — a wider area would pass even with nowrap reverted.
			plan := elems[0].PlanLayout(layout.LayoutArea{Width: 150, Height: 1000})
			if len(plan.Blocks) != 1 {
				t.Errorf("white-space:nowrap should keep the token on one overflowing line, got %d blocks", len(plan.Blocks))
			}
		})
	}
}

// --- line-height: normal ---

// TestLineHeightNormalUsesFontMetrics asserts that `line-height: normal` on
// an element using an embedded @font-face resolves from that font's own
// vertical metrics, not a flat fontSize*1.2. Poppins declares an unusually
// large line-gap, so its resolved leading is well above the old flat
// default.
func TestLineHeightNormalUsesFontMetrics(t *testing.T) {
	ttfData, err := os.ReadFile("../font/testdata/Poppins-Bold.ttf")
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.StdEncoding.EncodeToString(ttfData)

	src := `<html><head><style>
		@font-face {
			font-family: 'Poppins';
			src: url(data:font/truetype;base64,` + b64 + `) format('truetype');
		}
		p { font-family: 'Poppins'; font-size: 11pt; line-height: normal; }
	</style></head><body>
		<p>Hello Poppins</p>
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
	lines := p.Layout(500)
	if len(lines) == 0 || len(lines[0].Words) == 0 {
		t.Fatal("no words")
	}
	ef := lines[0].Words[0].Embedded
	if ef == nil {
		t.Fatal("expected embedded font (Poppins @font-face was not loaded)")
	}

	const fontSize = 11.0
	want := ef.NormalLineHeight(fontSize)
	flatDefault := fontSize * 1.2
	if want <= flatDefault {
		t.Fatalf("test fixture assumption broken: Poppins's normal leading (%v) should exceed the flat default (%v)", want, flatDefault)
	}
	if math.Abs(lines[0].Height-want) > 0.001 {
		t.Errorf("expected font-metric-derived height %.3f, got %.3f (flat 1.2 default would be %.3f)", want, lines[0].Height, flatDefault)
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
	src := `<p>Normal <span style="vertical-align: super; font-size: 8pt">TM</span> text</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	var foundTM, foundNormal bool
	for _, w := range lines[0].Words {
		if w.Text == "TM" {
			foundTM = true
			if w.BaselineShift <= 0 {
				t.Errorf("'TM' BaselineShift = %.2f, want positive (super)", w.BaselineShift)
			}
			if w.FontSize != 8 {
				t.Errorf("'TM' FontSize = %.1f, want 8", w.FontSize)
			}
		}
		if w.Text == "Normal" {
			foundNormal = true
			if w.BaselineShift != 0 {
				t.Errorf("'Normal' BaselineShift = %.2f, want 0", w.BaselineShift)
			}
		}
	}
	if !foundTM {
		t.Error("expected word 'TM'")
	}
	if !foundNormal {
		t.Error("expected word 'Normal'")
	}
}

func TestVerticalAlignSub(t *testing.T) {
	src := `<p>H<sub style="vertical-align: sub">2</sub>O</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	for _, w := range lines[0].Words {
		if w.Text == "2" {
			if w.BaselineShift >= 0 {
				t.Errorf("'2' BaselineShift = %.2f, want negative (sub)", w.BaselineShift)
			}
			if w.FontSize >= 12 {
				t.Errorf("'2' FontSize = %.1f, want smaller than 12 (sub reduces size)", w.FontSize)
			}
		}
		if w.Text == "H" && w.BaselineShift != 0 {
			t.Errorf("'H' BaselineShift = %.2f, want 0", w.BaselineShift)
		}
		if w.Text == "O" && w.BaselineShift != 0 {
			t.Errorf("'O' BaselineShift = %.2f, want 0", w.BaselineShift)
		}
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

// TestPageFirstEmptyMarginBoxPreserved is the parse-side regression guard for
// the bug where @page :first { @bottom-center { content: "" } } was dropped at
// parse, so it never reached the renderer and could not blank the first-page
// footer. An explicitly-declared empty-content box must be preserved (with
// HasContent set) so the renderer can OVERRIDE the inherited default for that
// page/slot. The base @page rule keeps dropping empty boxes (nothing to draw,
// nothing to override).
//
// Fail-before/pass-after: before the fix pc.First.MarginBoxes had no
// bottom-center entry, so the default footer leaked onto page 1.
func TestPageFirstEmptyMarginBoxPreserved(t *testing.T) {
	html := `<html><head><style>
		@page {
			margin: 36pt;
			@bottom-center { content: "Page " counter(page); }
		}
		@page :first {
			@bottom-center { content: ""; }
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
	// The default base box must still be present and non-empty.
	if mb, ok := pc.MarginBoxes["bottom-center"]; !ok {
		t.Error("expected default bottom-center margin box")
	} else if !strings.Contains(mb.Content, "Page ") {
		t.Errorf("default bottom-center content = %q, should contain 'Page '", mb.Content)
	}
	// The :first empty box must be preserved so it can override the default.
	if pc.First == nil {
		t.Fatal("expected First page config")
	}
	mb, ok := pc.First.MarginBoxes["bottom-center"]
	if !ok {
		t.Fatal("empty :first bottom-center box was dropped; it must be preserved to override the default")
	}
	if mb.Content != "" {
		t.Errorf(":first bottom-center content = %q, want empty", mb.Content)
	}
	if !mb.HasContent {
		t.Error(":first bottom-center HasContent should be true (content was explicitly declared)")
	}
}

// TestPageBaseEmptyMarginBoxDropped confirms an empty-content box on the BASE
// @page rule is still dropped (it has nothing to draw and nothing to override),
// preserving the pre-existing behaviour for the non-pseudo case.
func TestPageBaseEmptyMarginBoxDropped(t *testing.T) {
	html := `<html><head><style>
		@page {
			margin: 36pt;
			@bottom-center { content: ""; }
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
	if _, ok := pc.MarginBoxes["bottom-center"]; ok {
		t.Error("empty base bottom-center box should be dropped, but it was kept")
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

// N2: margin-box at-rule names are case-insensitive; @TOP-CENTER and
// @Bottom-Center must be stored under their lower-case keys so the
// renderer (which switches on lower-case literals) finds them.
func TestMarginBoxNameCaseInsensitive(t *testing.T) {
	html := `<html><head><style>
		@page {
			margin: 2cm;
			@TOP-CENTER { content: "Header"; }
			@Bottom-Center { content: "Footer"; }
		}
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	pc := result.PageConfig
	if pc == nil {
		t.Fatal("expected PageConfig")
	}
	if mb, ok := pc.MarginBoxes["top-center"]; !ok || mb.Content != "Header" {
		t.Errorf("top-center = %+v ok=%v, want content %q", mb, ok, "Header")
	}
	if mb, ok := pc.MarginBoxes["bottom-center"]; !ok || mb.Content != "Footer" {
		t.Errorf("bottom-center = %+v ok=%v, want content %q", mb, ok, "Footer")
	}
}

// B1: counter(page, <style>) must thread the list-style through to the
// placeholder and never leak the literal "{counter(page, upper-roman)}"
// call text into the rendered content.
func TestCounterStyleArgumentNoLeak(t *testing.T) {
	html := `<html><head><style>
		@page {
			margin: 2cm;
			@bottom-center { content: counter(page, upper-roman); }
			@top-center { content: "p. " counter(page, lower-alpha) " of " counter(pages, decimal); }
		}
	</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	bottom := result.PageConfig.MarginBoxes["bottom-center"].Content
	if want := layout.CounterPlaceholder("page", "upper-roman"); bottom != want {
		t.Errorf("bottom-center = %q, want %q", bottom, want)
	}
	// The stored value must be the opaque placeholder, not the raw CSS
	// call text (which carried a space after the comma). A raw-call leak
	// would still contain the original "counter(page, upper-roman)" with
	// its space; the normalized placeholder has no space.
	if strings.Contains(bottom, "counter(page, ") {
		t.Errorf("raw counter() call text leaked into content: %q", bottom)
	}
	top := result.PageConfig.MarginBoxes["top-center"].Content
	wantTop := "p. " + layout.CounterPlaceholder("page", "lower-alpha") +
		" of " + layout.CounterPlaceholder("pages", "decimal")
	if top != wantTop {
		t.Errorf("top-center = %q, want %q", top, wantTop)
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

func TestInlineBlockShrinkToFitFlowsInline(t *testing.T) {
	// Issue #330: two display:inline-block spans (no explicit width) should
	// shrink to fit their content and flow on the same line, rather than each
	// becoming a full-width block on its own line.
	src := `<p><span style="display:inline-block; background:#eee;">One</span> <span style="display:inline-block; background:#eee;">Two</span></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *layout.Paragraph, got %T", elems[0])
	}

	const areaWidth = 400.0
	lines := p.Layout(areaWidth)

	// Collect the inline-block words across all lines.
	var inlineWords []layout.Word
	for _, line := range lines {
		for _, w := range line.Words {
			if w.InlineBlock != nil {
				inlineWords = append(inlineWords, w)
			}
		}
	}
	if len(inlineWords) != 2 {
		t.Fatalf("expected 2 inline-block words, got %d", len(inlineWords))
	}

	// Both inline-blocks should land on a single line (shrink-to-fit), so the
	// paragraph should not have split them onto separate lines.
	if len(lines) != 1 {
		t.Errorf("expected inline-blocks to flow on a single line, got %d lines", len(lines))
	}

	// Each inline-block must be far narrower than the full area width: a
	// full-width regression would make each one ~areaWidth.
	for i, w := range inlineWords {
		if w.InlineWidth >= areaWidth {
			t.Errorf("inline-block %d width = %.1f, should shrink below area width %.0f", i, w.InlineWidth, areaWidth)
		}
		if w.InlineWidth <= 0 {
			t.Errorf("inline-block %d width = %.1f, should be positive", i, w.InlineWidth)
		}
	}
}

func TestInlineBlockNoWrapperFlowsInline(t *testing.T) {
	// Issue #330: a display:inline-block span with NO background/padding/border
	// converts to a Paragraph (not a Div) and is inlined as an InlineElement.
	// It must still flow inline on one line and hug its content (the inline
	// element width is well below the full area width).
	src := `<p>Before <span style="display:inline-block">chip</span> after</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *layout.Paragraph, got %T", elems[0])
	}

	const areaWidth = 400.0
	lines := p.Layout(areaWidth)
	if len(lines) != 1 {
		t.Errorf("expected content to flow on a single line, got %d lines", len(lines))
	}

	var inlineWords []layout.Word
	for _, line := range lines {
		for _, w := range line.Words {
			if w.InlineBlock != nil {
				inlineWords = append(inlineWords, w)
			}
		}
	}
	if len(inlineWords) != 1 {
		t.Fatalf("expected 1 inline-block word, got %d", len(inlineWords))
	}
	w := inlineWords[0]
	if w.InlineWidth <= 0 {
		t.Errorf("inline-block width = %.1f, should be positive", w.InlineWidth)
	}
	// "chip" is short; it must hug its content well below the 400pt area.
	if w.InlineWidth >= areaWidth/2 {
		t.Errorf("inline-block width = %.1f, should hug content (< %.0f)", w.InlineWidth, areaWidth/2)
	}
}

func TestInlineBlockChipsWrapAcrossLines(t *testing.T) {
	// Issue #330: several inline-block chips (with background, so each is a
	// shrink-to-fit Div) whose combined width exceeds the area must wrap onto
	// multiple lines, and each chip's width must stay below the area width.
	src := `<p>` +
		`<span style="display:inline-block; background:#eee;">Chip One</span> ` +
		`<span style="display:inline-block; background:#eee;">Chip Two</span> ` +
		`<span style="display:inline-block; background:#eee;">Chip Three</span> ` +
		`<span style="display:inline-block; background:#eee;">Chip Four</span>` +
		`</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *layout.Paragraph, got %T", elems[0])
	}

	// Narrow area forces wrapping across multiple lines.
	const areaWidth = 120.0
	lines := p.Layout(areaWidth)
	if len(lines) < 2 {
		t.Errorf("expected chips to wrap onto multiple lines at width %.0f, got %d", areaWidth, len(lines))
	}

	chipCount := 0
	for _, line := range lines {
		for _, w := range line.Words {
			if w.InlineBlock == nil {
				continue
			}
			chipCount++
			if w.InlineWidth >= areaWidth {
				t.Errorf("chip width = %.1f, should be < area width %.0f", w.InlineWidth, areaWidth)
			}
			if w.InlineWidth <= 0 {
				t.Errorf("chip width = %.1f, should be positive", w.InlineWidth)
			}
		}
	}
	if chipCount != 4 {
		t.Errorf("expected 4 inline-block chips, got %d", chipCount)
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

func TestInlineBlockSVGInDivRendersMedia(t *testing.T) {
	// An SVG with display:inline-block inside a wrapper div (not a
	// paragraph) should still render as actual SVG media, not an empty
	// block container. This tests the top-level dispatch: replaced
	// elements must use their specialized converters regardless of
	// display value.
	src := `<div><svg style="display:inline-block" width="100" height="50" viewBox="0 0 100 50"><rect width="100" height="50" fill="#333"/></svg></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Consumed < 40 {
		t.Errorf("consumed = %.1f, expected >= 40 (SVG should render as 50pt-tall media, not empty block)", plan.Consumed)
	}
}

func TestInlineBlockImageInFlexRendersMedia(t *testing.T) {
	// An image with display:inline-block inside a flex container should
	// render as actual image media.
	src := `<div style="display:flex"><img style="display:inline-block" src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==" width="80" height="40"></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Consumed < 30 {
		t.Errorf("consumed = %.1f, expected >= 30 (image should render as media, not empty block)", plan.Consumed)
	}
}

func TestInlineBlockSVGInFlexRendersMedia(t *testing.T) {
	// SVG with display:inline-block inside a flex container with centering
	// — the common pattern for centered images in editors.
	src := `<div style="display:flex;justify-content:center"><svg style="display:inline-block;width:120px;height:60px" xmlns="http://www.w3.org/2000/svg" width="120" height="60" viewBox="0 0 120 60"><rect width="120" height="60" fill="#111827"/></svg></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements for flex-wrapped inline-block SVG")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 1000})
	if plan.Consumed < 40 {
		t.Errorf("consumed = %.1f, expected >= 40 (SVG should render as media in flex container)", plan.Consumed)
	}
}

// --- Sub/Sup baseline shift ---

func TestSubBaselineShiftValue(t *testing.T) {
	// <sub> should produce a negative BaselineShift (shifted down).
	src := `<p>H<sub>2</sub>O</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// Find the "2" word — it should have negative BaselineShift.
	var found bool
	for _, w := range lines[0].Words {
		if w.Text == "2" {
			found = true
			if w.BaselineShift >= 0 {
				t.Errorf("sub word BaselineShift = %.2f, want negative", w.BaselineShift)
			}
		}
	}
	if !found {
		t.Error("expected a word with text '2'")
	}
}

func TestSupBaselineShiftValue(t *testing.T) {
	// <sup> should produce a positive BaselineShift (shifted up).
	src := `<p>E=mc<sup>2</sup></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// "E=mc" should have shift=0, "2" should have positive shift.
	for _, w := range lines[0].Words {
		if w.Text == "2" {
			if w.BaselineShift <= 0 {
				t.Errorf("sup word BaselineShift = %.2f, want positive", w.BaselineShift)
			}
		}
		if w.Text == "E=mc" {
			if w.BaselineShift != 0 {
				t.Errorf("normal word BaselineShift = %.2f, want 0", w.BaselineShift)
			}
		}
	}
}

func TestSubAdjacentSpacing(t *testing.T) {
	// H<sub>2</sub>O — "H" and "2" should be glued (SpaceAfter=0).
	src := `<p>H<sub>2</sub>O</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	for i, w := range lines[0].Words {
		if w.Text == "H" && i < len(lines[0].Words)-1 {
			if w.SpaceAfter != 0 {
				t.Errorf("'H' SpaceAfter = %.2f, want 0 (should be glued to subscript)", w.SpaceAfter)
			}
		}
		if w.Text == "2" && i < len(lines[0].Words)-1 {
			if w.SpaceAfter != 0 {
				t.Errorf("'2' SpaceAfter = %.2f, want 0 (should be glued to 'O')", w.SpaceAfter)
			}
		}
	}
}

func TestSupInHeading(t *testing.T) {
	src := `<h2>E=mc<sup>2</sup></h2>`
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
	// Should be a single block (one line), not split into multiple lines.
	if len(plan.Blocks) != 1 {
		t.Errorf("expected 1 block (single line heading), got %d", len(plan.Blocks))
	}
}

func TestCaffeineFormula(t *testing.T) {
	src := `<p>C<sub>8</sub>H<sub>10</sub>N<sub>4</sub>O<sub>2</sub></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// All adjacent word pairs should have SpaceAfter=0 (no gaps in formula).
	words := lines[0].Words
	for i := 0; i < len(words)-1; i++ {
		if words[i].SpaceAfter != 0 {
			t.Errorf("word[%d] %q SpaceAfter = %.2f, want 0", i, words[i].Text, words[i].SpaceAfter)
		}
	}
}

func TestSpaceBetweenStyledInlineElements(t *testing.T) {
	// "<b>bold</b> <i>italic</i>" — the space text node between elements
	// must be preserved so "bold" and "italic" don't merge.
	src := `<p><b>bold</b> <i>italic</i></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// Should have two separate words with space between them.
	var foundBold, foundItalic bool
	for _, w := range lines[0].Words {
		if w.Text == "bold" {
			foundBold = true
			if w.SpaceAfter == 0 {
				t.Error("'bold' SpaceAfter should be non-zero (space before italic)")
			}
		}
		if w.Text == "italic" {
			foundItalic = true
		}
	}
	if !foundBold || !foundItalic {
		t.Errorf("expected words 'bold' and 'italic', got %v", lines[0].Words)
	}
}

func TestNoSpaceBetweenAdjacentStyledElements(t *testing.T) {
	// "<i>italic</i><b>bold</b>" — no whitespace between elements,
	// should render flush.
	src := `<p><i>italic</i><b>bold</b></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	for _, w := range lines[0].Words {
		if w.Text == "italic" && w.SpaceAfter != 0 {
			t.Errorf("'italic' SpaceAfter = %.2f, want 0 (no space before bold)", w.SpaceAfter)
		}
	}
}

func TestSupFollowedByPunctuation(t *testing.T) {
	// "7<sup>th</sup>! works" — the "!" should NOT inherit superscript styling.
	src := `<p>April 7<sup>th</sup>! works</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	for _, w := range lines[0].Words {
		if w.Text == "!" || w.Text == "!works" || w.Text == "th!" {
			if w.BaselineShift != 0 {
				t.Errorf("word %q has BaselineShift=%.2f, want 0 (should not be superscript)", w.Text, w.BaselineShift)
			}
			if w.FontSize != 12 {
				t.Errorf("word %q has FontSize=%.1f, want 12 (should not inherit sup size)", w.Text, w.FontSize)
			}
		}
	}
	// "works" should be a separate word with a space before it.
	var foundWorks bool
	for _, w := range lines[0].Words {
		if w.Text == "works" {
			foundWorks = true
		}
	}
	if !foundWorks {
		t.Error("expected 'works' as a separate word")
	}
}

func TestCommaBetweenSubscripts(t *testing.T) {
	// x<sub>i</sub>,<sub>j</sub> — comma should NOT inherit subscript styling.
	src := `<p>x<sub>i</sub>,<sub>j</sub></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	for _, w := range lines[0].Words {
		if w.Text == "," {
			if w.BaselineShift != 0 {
				t.Errorf("comma BaselineShift = %.2f, want 0", w.BaselineShift)
			}
			if w.FontSize != 12 {
				t.Errorf("comma FontSize = %.1f, want 12", w.FontSize)
			}
		}
	}
}

func TestBaselineShiftCSSSuper(t *testing.T) {
	src := `<p>normal<span style="baseline-shift: super; font-size: 75%">super</span></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	for _, w := range lines[0].Words {
		if w.Text == "super" && w.BaselineShift <= 0 {
			t.Errorf("baseline-shift:super word shift = %.2f, want positive", w.BaselineShift)
		}
	}
}

func TestBaselineShiftCSSLength(t *testing.T) {
	src := `<p>normal<span style="baseline-shift: 5pt; font-size: 75%">shifted</span></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	for _, w := range lines[0].Words {
		if w.Text == "shifted" {
			if w.BaselineShift < 4.9 || w.BaselineShift > 5.1 {
				t.Errorf("baseline-shift:5pt word shift = %.2f, want ~5.0", w.BaselineShift)
			}
		}
	}
}

func TestBaselineShiftCSSNegative(t *testing.T) {
	src := `<p>normal<span style="baseline-shift: -3pt">dropped</span></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	for _, w := range lines[0].Words {
		if w.Text == "dropped" {
			if w.BaselineShift > -2.9 || w.BaselineShift < -3.1 {
				t.Errorf("baseline-shift:-3pt word shift = %.2f, want ~-3.0", w.BaselineShift)
			}
		}
	}
}

func TestVerticalAlignLength(t *testing.T) {
	src := `<p>normal<span style="vertical-align: 4pt; font-size: 75%">raised</span></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	for _, w := range lines[0].Words {
		if w.Text == "raised" {
			if w.BaselineShift < 3.9 || w.BaselineShift > 4.1 {
				t.Errorf("vertical-align:4pt shift = %.2f, want ~4.0", w.BaselineShift)
			}
		}
	}
}

func TestVerticalAlignOverridesBaselineShift(t *testing.T) {
	src := `<p>normal<span style="baseline-shift: 10pt; vertical-align: sub">text</span></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	for _, w := range lines[0].Words {
		if w.Text == "text" {
			if w.BaselineShift >= 0 {
				t.Errorf("vertical-align:sub should override baseline-shift:10pt, got shift=%.2f", w.BaselineShift)
			}
		}
	}
}

func TestBaselineShiftCSSSub(t *testing.T) {
	src := `<p>normal<span style="baseline-shift: sub; font-size: 75%">sub</span></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	for _, w := range lines[0].Words {
		if w.Text == "sub" && w.BaselineShift >= 0 {
			t.Errorf("baseline-shift:sub shift = %.2f, want negative", w.BaselineShift)
		}
	}
}

func TestBaselineShiftZero(t *testing.T) {
	// Explicit zero should override a tag default like <sup>.
	src := `<p>normal<sup style="baseline-shift: 0">flat</sup></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	for _, w := range lines[0].Words {
		if w.Text == "flat" && w.BaselineShift != 0 {
			t.Errorf("baseline-shift:0 on <sup> shift = %.2f, want 0", w.BaselineShift)
		}
	}
}

func TestBaselineShiftEmUnit(t *testing.T) {
	src := `<p>normal<span style="baseline-shift: 0.5em">half</span></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	for _, w := range lines[0].Words {
		if w.Text == "half" {
			// 0.5em at 12pt = 6pt
			if w.BaselineShift < 5.9 || w.BaselineShift > 6.1 {
				t.Errorf("baseline-shift:0.5em shift = %.2f, want ~6.0", w.BaselineShift)
			}
		}
	}
}

func TestBaselineShiftInvalid(t *testing.T) {
	src := `<p>normal<span style="baseline-shift: garbage">text</span></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	for _, w := range lines[0].Words {
		if w.Text == "text" && w.BaselineShift != 0 {
			t.Errorf("invalid baseline-shift should produce 0, got %.2f", w.BaselineShift)
		}
	}
}

func TestBaselineShiftThenVerticalAlign(t *testing.T) {
	// Reverse order: numeric then keyword — keyword should win.
	src := `<p><span style="vertical-align: sub; baseline-shift: 5pt">text</span></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	p := elems[0].(*layout.Paragraph)
	lines := p.Layout(400)
	for _, w := range lines[0].Words {
		if w.Text == "text" {
			// baseline-shift: 5pt is declared AFTER vertical-align: sub,
			// so the numeric 5pt should win.
			if w.BaselineShift < 4.9 || w.BaselineShift > 5.1 {
				t.Errorf("baseline-shift after vertical-align: shift = %.2f, want ~5.0", w.BaselineShift)
			}
		}
	}
}

// --- border-radius percentage (issue #329) ---

// findDivWithRadius walks the element tree and returns the first *layout.Div
// that has a non-zero border-radius (absolute or percentage). Returns nil if
// none is found.
func findDivWithRadius(elems []layout.Element) *layout.Div {
	for _, e := range elems {
		if d, ok := e.(*layout.Div); ok {
			pct := d.BorderRadiusPercent()
			if pct[0] > 0 || pct[1] > 0 || pct[2] > 0 || pct[3] > 0 {
				return d
			}
			if c := findDivWithRadius(d.Children()); c != nil {
				return c
			}
		}
	}
	return nil
}

// border-radius: 50% on a 28x28 box must reach the layout Div as a 0.5
// fraction per corner (not collapse to 0 the way the pre-fix eager parse did).
func TestBorderRadiusPercentReachesDiv(t *testing.T) {
	src := `<div style="width:28px;height:28px;background:#130048;border-radius:50%"></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	d := findDivWithRadius(elems)
	if d == nil {
		t.Fatal("expected a Div carrying a percentage border-radius")
	}
	pct := d.BorderRadiusPercent()
	for i, p := range pct {
		if p != 0.5 {
			t.Errorf("corner %d: border-radius percent = %v, want 0.5", i, p)
		}
	}
}

// A length border-radius must NOT set any percentage fraction — the control
// case that distinguishes the percentage path from the absolute path.
func TestBorderRadiusLengthHasNoPercent(t *testing.T) {
	src := `<div style="width:28px;height:28px;background:#130048;border-radius:14pt"></div>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	// findDivWithRadius only matches percentage corners; a length-only radius
	// must therefore not be found.
	if d := findDivWithRadius(elems); d != nil {
		t.Errorf("length border-radius should not carry a percentage fraction, got %v", d.BorderRadiusPercent())
	}
}

// --- PageConfig.Resolve shared helper (B-1 / S-1) ---

// helperPageConfig parses the given @page CSS and returns the PageConfig,
// failing the test if none is produced.
func helperPageConfig(t *testing.T, css string) *PageConfig {
	t.Helper()
	html := `<html><head><style>` + css + `</style></head><body><p>X</p></body></html>`
	result, err := ConvertFull(html, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	return result.PageConfig
}

func TestResolvePercentMarginsAgainstPageBox(t *testing.T) {
	// margin: 10% must resolve against the resolved page box (A4), NOT the
	// zero basis the eager floats carry. Left/right vs width, top/bottom vs
	// height (CSS Paged Media).
	pc := helperPageConfig(t, `@page { size: a4; margin: 10%; }`)
	// Eager floats are 0 before Resolve (percent has no absolute value).
	if pc.MarginTop != 0 {
		t.Fatalf("pre-Resolve MarginTop = %.2f, want 0 (zero basis)", pc.MarginTop)
	}
	w, h, auto := pc.Resolve(595.28, 841.89) // A4 default; @page also A4
	if auto {
		t.Error("autoHeight should be false")
	}
	if math.Abs(w-595.28) > 0.5 || math.Abs(h-841.89) > 0.5 {
		t.Errorf("size = %.2f x %.2f, want A4 595.28 x 841.89", w, h)
	}
	if math.Abs(pc.MarginLeft-59.528) > 0.1 || math.Abs(pc.MarginRight-59.528) > 0.1 {
		t.Errorf("L/R margin = %.3f / %.3f, want ~59.528 (10%% of width)", pc.MarginLeft, pc.MarginRight)
	}
	if math.Abs(pc.MarginTop-84.189) > 0.1 || math.Abs(pc.MarginBottom-84.189) > 0.1 {
		t.Errorf("T/B margin = %.3f / %.3f, want ~84.189 (10%% of height)", pc.MarginTop, pc.MarginBottom)
	}
}

func TestResolveCalcLonghandMargin(t *testing.T) {
	// margin-top: calc(1in + 2px) must resolve to 72 + 2 = 74pt.
	pc := helperPageConfig(t, `@page { margin-top: calc(1in + 2px); }`)
	pc.Resolve(595.28, 841.89)
	if math.Abs(pc.MarginTop-74) > 0.5 {
		t.Errorf("MarginTop = %.2f, want ~74 (1in + 2px)", pc.MarginTop)
	}
}

func TestResolveFirstPagefPercentMargin(t *testing.T) {
	// Percentages in :first margins must also resolve against the page box.
	pc := helperPageConfig(t, `@page { margin: 1cm; } @page :first { margin-top: 20%; }`)
	if pc.First == nil {
		t.Fatal("expected First margins")
	}
	pc.Resolve(595.28, 841.89)
	if math.Abs(pc.First.Top-168.378) > 0.2 {
		t.Errorf("First.Top = %.3f, want ~168.378 (20%% of A4 height)", pc.First.Top)
	}
}

func TestResolveOrientationOnlyRotatesDefault(t *testing.T) {
	// @page { size: landscape } with no explicit dims rotates the document
	// default (A4 portrait) to landscape.
	pc := helperPageConfig(t, `@page { size: landscape; }`)
	if !pc.OrientationOnly {
		t.Fatal("expected OrientationOnly")
	}
	w, h, _ := pc.Resolve(595.28, 841.89)
	if w <= h {
		t.Errorf("landscape size = %.2f x %.2f, want width > height", w, h)
	}
	if math.Abs(w-841.89) > 0.5 || math.Abs(h-595.28) > 0.5 {
		t.Errorf("size = %.2f x %.2f, want A4 rotated 841.89 x 595.28", w, h)
	}
}

func TestResolveOrientationOnlyOverridesExplicitDims(t *testing.T) {
	// S-1: a later size:landscape must rotate an earlier explicit named
	// size rather than being ignored. @page{size:A4} (portrait) followed by
	// @page{size:landscape} must yield A4 landscape.
	pc := helperPageConfig(t, `@page { size: a4; } @page { size: landscape; }`)
	if !pc.OrientationOnly {
		t.Fatal("expected OrientationOnly to be set by the later rule")
	}
	w, h, _ := pc.Resolve(595.28, 841.89)
	if w <= h {
		t.Errorf("S-1: size = %.2f x %.2f, want landscape (width > height)", w, h)
	}
	if math.Abs(w-841.89) > 0.5 || math.Abs(h-595.28) > 0.5 {
		t.Errorf("S-1: size = %.2f x %.2f, want A4 rotated 841.89 x 595.28", w, h)
	}
}
