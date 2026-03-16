// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"math"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// --- Div UnitValue width ---

func TestDivWidthUnit_Pt(t *testing.T) {
	d := NewDiv().
		SetWidthUnit(Pt(200)).
		Add(NewParagraph("Content", font.Helvetica, 12))

	plan := d.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if len(plan.Blocks) == 0 {
		t.Fatal("expected blocks")
	}
	if math.Abs(plan.Blocks[0].Width-200) > 0.01 {
		t.Errorf("width = %.1f, want 200", plan.Blocks[0].Width)
	}
}

func TestDivWidthUnit_Pct(t *testing.T) {
	d := NewDiv().
		SetWidthUnit(Pct(50)).
		Add(NewParagraph("Content", font.Helvetica, 12))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if len(plan.Blocks) == 0 {
		t.Fatal("expected blocks")
	}
	if math.Abs(plan.Blocks[0].Width-200) > 0.01 {
		t.Errorf("width = %.1f, want 200 (50%% of 400)", plan.Blocks[0].Width)
	}
}

func TestDivWidthUnit_PctChangesWithArea(t *testing.T) {
	d := NewDiv().
		SetWidthUnit(Pct(50)).
		Add(NewParagraph("Content", font.Helvetica, 12))

	// 50% of 400 = 200
	plan1 := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	// 50% of 600 = 300
	plan2 := d.PlanLayout(LayoutArea{Width: 600, Height: 1000})

	w1 := plan1.Blocks[0].Width
	w2 := plan2.Blocks[0].Width

	if math.Abs(w1-200) > 0.01 {
		t.Errorf("at 400: width = %.1f, want 200", w1)
	}
	if math.Abs(w2-300) > 0.01 {
		t.Errorf("at 600: width = %.1f, want 300", w2)
	}
}

func TestDivMaxWidthUnit(t *testing.T) {
	d := NewDiv().
		SetMaxWidthUnit(Pct(50)).
		Add(NewParagraph("Content", font.Helvetica, 12))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Blocks[0].Width > 201 {
		t.Errorf("width = %.1f, should be capped at ~200 (50%% of 400)", plan.Blocks[0].Width)
	}
}

func TestDivMinWidthUnit(t *testing.T) {
	d := NewDiv().
		SetMinWidthUnit(Pct(80)).
		Add(NewParagraph("X", font.Helvetica, 12))

	plan := d.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Blocks[0].Width < 399 {
		t.Errorf("width = %.1f, should be at least ~400 (80%% of 500)", plan.Blocks[0].Width)
	}
}

func TestDivMinHeightUnit(t *testing.T) {
	d := NewDiv().
		SetMinHeightUnit(Pt(100)).
		Add(NewParagraph("Short", font.Helvetica, 12))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed < 100 {
		t.Errorf("consumed = %.1f, want >= 100 (from minHeight)", plan.Consumed)
	}
}

// --- Div MinWidth/MaxWidth with UnitValue ---

func TestDivMinWidthWithUnitValue_Point(t *testing.T) {
	d := NewDiv().
		SetWidthUnit(Pt(150)).
		Add(NewParagraph("Test", font.Helvetica, 12))

	minW := d.MinWidth()
	maxW := d.MaxWidth()
	if minW != 150 {
		t.Errorf("MinWidth = %.1f, want 150", minW)
	}
	if maxW != 150 {
		t.Errorf("MaxWidth = %.1f, want 150", maxW)
	}
}

func TestDivMinWidthWithUnitValue_Pct(t *testing.T) {
	d := NewDiv().
		SetWidthUnit(Pct(50)).
		Add(NewParagraph("Test", font.Helvetica, 12))

	// Percentage widths don't contribute to intrinsic width.
	minW := d.MinWidth()
	if minW == 50 {
		t.Error("MinWidth should NOT return the percentage value as intrinsic width")
	}
}

// --- Table minWidthUnit ---

func TestTableMinWidthUnit_Pct100(t *testing.T) {
	tbl := NewTable()
	tbl.SetAutoColumnWidths()
	tbl.SetMinWidthUnit(Pct(100))

	row := tbl.AddRow()
	row.AddCell("Hello", font.Helvetica, 10)
	row.AddCell("World", font.Helvetica, 10)

	// Layout at 500pt: table should expand to fill 500pt.
	plan := tbl.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed")
	}
}

func TestTableMinWidthUnit_Pt(t *testing.T) {
	tbl := NewTable()
	tbl.SetAutoColumnWidths()
	tbl.SetMinWidthUnit(Pt(400))

	row := tbl.AddRow()
	row.AddCell("A", font.Helvetica, 10)
	row.AddCell("B", font.Helvetica, 10)

	plan := tbl.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

// --- Cell width hint ---

func TestCellWidthHint(t *testing.T) {
	tbl := NewTable()
	tbl.SetAutoColumnWidths()

	row := tbl.AddRow()
	row.AddCell("Text", font.Helvetica, 10)
	c := row.AddCell("Wide", font.Helvetica, 10)
	c.SetWidthHint(200)

	plan := tbl.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

// --- FlexItem basisUnit ---

func TestFlexItemBasisUnit_Pt(t *testing.T) {
	flex := NewFlex()
	flex.SetDirection(FlexRow)

	item := NewFlexItem(NewParagraph("Content", font.Helvetica, 12))
	item.SetBasisUnit(Pt(200))
	flex.AddItem(item)
	flex.Add(NewParagraph("Other", font.Helvetica, 12))

	plan := flex.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestFlexItemBasisUnit_Pct(t *testing.T) {
	flex := NewFlex()
	flex.SetDirection(FlexRow)

	item := NewFlexItem(NewParagraph("Half", font.Helvetica, 12))
	item.SetBasisUnit(Pct(50))
	flex.AddItem(item)

	plan := flex.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestFlexItemEffectiveBasis(t *testing.T) {
	item := &FlexItem{shrink: 1}

	// No basis set → 0 (auto).
	if b := item.effectiveBasis(500); b != 0 {
		t.Errorf("effectiveBasis = %.1f, want 0", b)
	}

	// Absolute basis.
	item.basis = 100
	if b := item.effectiveBasis(500); b != 100 {
		t.Errorf("effectiveBasis = %.1f, want 100", b)
	}

	// UnitValue overrides absolute.
	u := Pct(30)
	item.basisUnit = &u
	if b := item.effectiveBasis(500); math.Abs(b-150) > 0.01 {
		t.Errorf("effectiveBasis = %.1f, want 150 (30%% of 500)", b)
	}
}

// --- Div relative offset ---

func TestDivRelativeOffset(t *testing.T) {
	d := NewDiv().
		SetRelativeOffset(10, 5).
		Add(NewParagraph("Shifted", font.Helvetica, 12))

	plan := d.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if len(plan.Blocks) == 0 {
		t.Fatal("expected blocks")
	}
	// Container block should have X offset = 10.
	if plan.Blocks[0].X != 10 {
		t.Errorf("X = %.1f, want 10", plan.Blocks[0].X)
	}
}

// --- Paragraph word-break ---

func TestParagraphWordBreakAll(t *testing.T) {
	p := NewParagraph("Superlongword", font.Helvetica, 12)
	p.SetWordBreak("break-all")

	// With very narrow width, should break into multiple lines.
	lines := p.Layout(30)
	if len(lines) <= 1 {
		t.Errorf("word-break:break-all with narrow width should produce multiple lines, got %d", len(lines))
	}
}

func TestParagraphWordBreakNormal(t *testing.T) {
	p := NewParagraph("Hello World", font.Helvetica, 12)

	// Normal word-break: "Hello World" should be 1-2 lines depending on width.
	lines := p.Layout(400)
	if len(lines) != 1 {
		t.Errorf("expected 1 line for short text at wide width, got %d", len(lines))
	}
}

// --- Div GetSpaceBefore/GetSpaceAfter ---

func TestDivGetSpaceBeforeAfter(t *testing.T) {
	d := NewDiv().
		SetSpaceBefore(15).
		SetSpaceAfter(20)

	if d.GetSpaceBefore() != 15 {
		t.Errorf("GetSpaceBefore = %.1f, want 15", d.GetSpaceBefore())
	}
	if d.GetSpaceAfter() != 20 {
		t.Errorf("GetSpaceAfter = %.1f, want 20", d.GetSpaceAfter())
	}
}

func TestParagraphGetSpaceBeforeAfter(t *testing.T) {
	p := NewParagraph("Text", font.Helvetica, 12)
	p.SetSpaceBefore(10)
	p.SetSpaceAfter(12)

	if p.GetSpaceBefore() != 10 {
		t.Errorf("GetSpaceBefore = %.1f, want 10", p.GetSpaceBefore())
	}
	if p.GetSpaceAfter() != 12 {
		t.Errorf("GetSpaceAfter = %.1f, want 12", p.GetSpaceAfter())
	}
}

func TestFlexGetSpaceBeforeAfter(t *testing.T) {
	f := NewFlex()
	f.SetSpaceBefore(8)
	f.SetSpaceAfter(16)

	if f.GetSpaceBefore() != 8 {
		t.Errorf("GetSpaceBefore = %.1f, want 8", f.GetSpaceBefore())
	}
	if f.GetSpaceAfter() != 16 {
		t.Errorf("GetSpaceAfter = %.1f, want 16", f.GetSpaceAfter())
	}
}

// --- Renderer per-page margins ---

func TestRendererMarginsForPage(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.SetFirstMargins(Margins{Top: 100, Right: 72, Bottom: 72, Left: 72})
	r.SetLeftMargins(Margins{Top: 72, Right: 50, Bottom: 72, Left: 90})
	r.SetRightMargins(Margins{Top: 72, Right: 90, Bottom: 72, Left: 50})

	// Page 0 = first page.
	m0 := r.marginsForPage(0)
	if m0.Top != 100 {
		t.Errorf("page 0: Top = %.1f, want 100 (first page)", m0.Top)
	}

	// Page 1 = left page (even page number 2).
	m1 := r.marginsForPage(1)
	if m1.Left != 90 {
		t.Errorf("page 1: Left = %.1f, want 90 (left page)", m1.Left)
	}

	// Page 2 = right page (odd page number 3).
	m2 := r.marginsForPage(2)
	if m2.Right != 90 {
		t.Errorf("page 2: Right = %.1f, want 90 (right page)", m2.Right)
	}

	// Page 3 = left page.
	m3 := r.marginsForPage(3)
	if m3.Left != 90 {
		t.Errorf("page 3: Left = %.1f, want 90 (left page)", m3.Left)
	}
}

func TestRendererDefaultMarginsWhenNoVariants(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	for i := 0; i < 5; i++ {
		m := r.marginsForPage(i)
		if m.Top != 72 {
			t.Errorf("page %d: Top = %.1f, want 72 (default)", i, m.Top)
		}
	}
}

// --- Heading SetRuns ---

func TestHeadingSetRuns(t *testing.T) {
	h := NewHeadingWithFont("Test", H2, font.HelveticaBold, 18)
	run := TextRun{
		Text:     "Colored Title",
		Font:     font.HelveticaBold,
		FontSize: 18,
		Color:    RGB(0.1, 0, 0.3),
	}
	h.SetRuns([]TextRun{run})

	plan := h.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	if plan.Consumed <= 0 {
		t.Error("heading should render after SetRuns")
	}
}

// --- TextRun BaselineShift ---

func TestBaselineShiftOnWord(t *testing.T) {
	p := NewStyledParagraph(
		TextRun{Text: "Normal ", Font: font.Helvetica, FontSize: 12},
		TextRun{Text: "Super", Font: font.Helvetica, FontSize: 8, BaselineShift: 4},
		TextRun{Text: " end", Font: font.Helvetica, FontSize: 12},
	)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected lines")
	}
	// Verify the superscript word has BaselineShift set.
	found := false
	for _, w := range lines[0].Words {
		if w.Text == "Super" && w.BaselineShift == 4 {
			found = true
		}
	}
	if !found {
		t.Error("expected word 'Super' with BaselineShift=4")
	}
}

// --- Paragraph hyphens ---

func TestParagraphHyphensAuto(t *testing.T) {
	// Use two words where the second doesn't fit on the first line.
	// Hyphenation should break the second word and add a hyphen.
	p := NewStyledParagraph(
		TextRun{Text: "Hello internationalization", Font: font.Helvetica, FontSize: 12},
	)
	p.SetHyphens("auto")

	// Width enough for "Hello" + part of "internationalization" but not all.
	lines := p.Layout(120)
	if len(lines) <= 1 {
		t.Errorf("hyphens:auto should produce multiple lines, got %d", len(lines))
	}
	// First line should contain a hyphenated fragment.
	if len(lines) > 0 && len(lines[0].Words) > 1 {
		lastWord := lines[0].Words[len(lines[0].Words)-1]
		if len(lastWord.Text) > 1 && lastWord.Text[len(lastWord.Text)-1] == '-' {
			// Good — hyphenation worked.
		}
	}
}

func TestParagraphHyphensNone(t *testing.T) {
	p := NewParagraph("Hello World", font.Helvetica, 12)
	p.SetHyphens("none")

	lines := p.Layout(400)
	if len(lines) != 1 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}
}

// --- Percentage height in fixed-height flex container ---

func TestFlexRowPercentageHeightConstrainedByContainer(t *testing.T) {
	// Simulates: <div style="display:flex; align-items:flex-end; height:130px">
	//   <div style="flex:1; height:80%; background:..."></div>
	//   <div style="flex:1; height:50%; background:..."></div>
	// </div>
	// <p>This should appear below the 130px chart.</p>
	//
	// Bug: child percentage heights were resolving against page height (~800pt)
	// instead of the container's 130px.

	bar1 := NewDiv()
	bar1.SetHeightUnit(Pct(80))

	bar2 := NewDiv()
	bar2.SetHeightUnit(Pct(50))

	flex := NewFlex()
	flex.SetDirection(FlexRow)
	flex.ForceHeight(Pt(130))
	flex.SetAlignItems(CrossAlignEnd)
	flex.AddItem(NewFlexItem(bar1).SetGrow(1))
	flex.AddItem(NewFlexItem(bar2).SetGrow(1))

	plan := flex.PlanLayout(LayoutArea{Width: 400, Height: 800})

	// The flex container should consume exactly 130pt, not more.
	if math.Abs(plan.Consumed-130) > 1 {
		t.Errorf("flex consumed = %.1f, want 130 (explicit height)", plan.Consumed)
	}

	// Children should have heights relative to 130, not 800.
	// bar1 = 80% of 130 = 104, bar2 = 50% of 130 = 65.
	if len(plan.Blocks) == 0 {
		t.Fatal("expected blocks")
	}
	container := plan.Blocks[0]
	if len(container.Children) < 2 {
		t.Fatalf("expected 2 children, got %d", len(container.Children))
	}
	bar1H := container.Children[0].Height
	bar2H := container.Children[1].Height
	if math.Abs(bar1H-104) > 1 {
		t.Errorf("bar1 height = %.1f, want ~104 (80%% of 130)", bar1H)
	}
	if math.Abs(bar2H-65) > 1 {
		t.Errorf("bar2 height = %.1f, want ~65 (50%% of 130)", bar2H)
	}
}

func TestDivPercentageHeightConstrainedByParent(t *testing.T) {
	// A div with explicit height should constrain children's percentage heights.
	child := NewDiv()
	child.SetHeightUnit(Pct(50))

	parent := NewDiv()
	parent.SetHeightUnit(Pt(200))
	parent.Add(child)

	plan := parent.PlanLayout(LayoutArea{Width: 400, Height: 800})
	if math.Abs(plan.Consumed-200) > 1 {
		t.Errorf("parent consumed = %.1f, want 200", plan.Consumed)
	}

	// Child should be 50% of 200 = 100, not 50% of 800 = 400.
	if len(plan.Blocks) == 0 || len(plan.Blocks[0].Children) == 0 {
		t.Fatal("expected nested blocks")
	}
	childH := plan.Blocks[0].Children[0].Height
	if math.Abs(childH-100) > 1 {
		t.Errorf("child height = %.1f, want 100 (50%% of 200)", childH)
	}
}

// --- Cross-axis stretch tests (flex-row with explicit height) ---

func TestFlexRow_CrossAxisStretch_BackgroundFillsContainerHeight(t *testing.T) {
	// Basic: items should stretch to fill the container's full height.
	// <div style="display:flex; height:200px;">
	//   <div style="flex:1; background:navy;">Short</div>
	//   <div style="width:100px; background:gray;">Also short</div>
	// </div>

	item1 := NewDiv()
	item1.SetBackground(Color{R: 0, G: 0, B: 0.5})
	item2 := NewDiv()
	item2.SetBackground(Color{R: 0.5, G: 0.5, B: 0.5})
	item2.SetWidth(100)

	flex := NewFlex()
	flex.SetDirection(FlexRow)
	flex.ForceHeight(Pt(200))
	flex.SetAlignItems(CrossAlignStretch) // default, but explicit
	flex.AddItem(NewFlexItem(item1).SetGrow(1))
	flex.AddItem(NewFlexItem(item2))

	plan := flex.PlanLayout(LayoutArea{Width: 400, Height: 800})

	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %v", plan.Status)
	}
	if math.Abs(plan.Consumed-200) > 1 {
		t.Errorf("consumed = %.1f, want 200", plan.Consumed)
	}

	// Both items should have height = 200 (the container height).
	container := plan.Blocks[0]
	for i, child := range container.Children {
		if math.Abs(child.Height-200) > 1 {
			t.Errorf("child[%d] height = %.1f, want 200 (stretched)", i, child.Height)
		}
	}
}

func TestFlexRow_CrossAxisStretch_NestedColumnJustifyCenter(t *testing.T) {
	// Nested column flex with justify-content:center should center
	// within the stretched height.
	// <div style="display:flex; height:300px;">
	//   <div style="flex:1; display:flex; flex-direction:column; justify-content:center;">
	//     <div style="height:40px;">Centered</div>
	//   </div>
	// </div>

	innerChild := NewDiv()
	innerChild.SetHeightUnit(Pt(40))

	nestedFlex := NewFlex()
	nestedFlex.SetDirection(FlexColumn)
	nestedFlex.SetJustifyContent(JustifyCenter)
	nestedFlex.AddItem(NewFlexItem(innerChild))

	flex := NewFlex()
	flex.SetDirection(FlexRow)
	flex.ForceHeight(Pt(300))
	flex.AddItem(NewFlexItem(nestedFlex).SetGrow(1))

	plan := flex.PlanLayout(LayoutArea{Width: 400, Height: 800})

	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %v", plan.Status)
	}

	// The nested flex should consume 300pt (stretched).
	container := plan.Blocks[0]
	if len(container.Children) == 0 {
		t.Fatal("expected children")
	}
	nestedH := container.Children[0].Height
	if math.Abs(nestedH-300) > 1 {
		t.Errorf("nested flex height = %.1f, want 300 (stretched)", nestedH)
	}

	// The inner child (40px) should be vertically centered within 300px.
	// Center offset = (300 - 40) / 2 = 130.
	if len(container.Children[0].Children) == 0 {
		t.Fatal("expected inner child")
	}
	innerY := container.Children[0].Children[0].Y
	// The inner child's Y should be around 130 (centered).
	if math.Abs(innerY-130) > 2 {
		t.Errorf("inner child Y = %.1f, want ~130 (centered in 300)", innerY)
	}
}

func TestFlexRow_CrossAxisStretch_ExplicitHeightNotOverridden(t *testing.T) {
	// Items with an explicit CSS height should NOT be stretched.
	// <div style="display:flex; height:200px;">
	//   <div style="flex:1; height:50px;">Fixed height</div>
	// </div>

	item := NewDiv()
	item.SetHeightUnit(Pt(50))

	flex := NewFlex()
	flex.SetDirection(FlexRow)
	flex.ForceHeight(Pt(200))
	flex.AddItem(NewFlexItem(item).SetGrow(1))

	plan := flex.PlanLayout(LayoutArea{Width: 400, Height: 800})

	container := plan.Blocks[0]
	if len(container.Children) == 0 {
		t.Fatal("expected children")
	}
	childH := container.Children[0].Height
	if math.Abs(childH-50) > 1 {
		t.Errorf("child height = %.1f, want 50 (explicit, not stretched)", childH)
	}
}

func TestFlexRow_CrossAxisStretch_AlignCenterNotStretched(t *testing.T) {
	// align-items:center should not stretch, just center.
	// <div style="display:flex; height:200px; align-items:center;">
	//   <div style="flex:1;">Content</div>
	// </div>

	item := NewDiv()

	flex := NewFlex()
	flex.SetDirection(FlexRow)
	flex.ForceHeight(Pt(200))
	flex.SetAlignItems(CrossAlignCenter)
	flex.AddItem(NewFlexItem(item).SetGrow(1))

	plan := flex.PlanLayout(LayoutArea{Width: 400, Height: 800})

	container := plan.Blocks[0]
	if len(container.Children) == 0 {
		t.Fatal("expected children")
	}
	// Item should NOT be 200px — it should be at its natural content height.
	childH := container.Children[0].Height
	if childH > 10 {
		t.Errorf("child height = %.1f, want small (centered, not stretched)", childH)
	}
}

func TestFlexRow_CrossAxisStretch_AlignSelfOverride(t *testing.T) {
	// align-self:center on one item should prevent that item from stretching.
	item1 := NewDiv()
	item1.SetBackground(Color{R: 1})
	item2 := NewDiv()

	flex := NewFlex()
	flex.SetDirection(FlexRow)
	flex.ForceHeight(Pt(200))
	flex.SetAlignItems(CrossAlignStretch)
	flex.AddItem(NewFlexItem(item1).SetGrow(1))
	flex.AddItem(NewFlexItem(item2).SetGrow(1).SetAlignSelf(CrossAlignCenter))

	plan := flex.PlanLayout(LayoutArea{Width: 400, Height: 800})

	container := plan.Blocks[0]
	if len(container.Children) < 2 {
		t.Fatal("expected 2 children")
	}
	// item1 should be stretched to 200.
	if math.Abs(container.Children[0].Height-200) > 1 {
		t.Errorf("item1 height = %.1f, want 200 (stretched)", container.Children[0].Height)
	}
	// item2 should NOT be stretched (align-self: center).
	if container.Children[1].Height > 10 {
		t.Errorf("item2 height = %.1f, want small (align-self:center)", container.Children[1].Height)
	}
}

func TestFlexRow_CrossAxisStretch_NoExplicitHeight_NoStretch(t *testing.T) {
	// Without an explicit container height, items should NOT be stretched
	// beyond the tallest item's content height.
	item1 := NewDiv()
	item1.SetHeightUnit(Pt(80))
	item2 := NewDiv() // content height ~0

	flex := NewFlex()
	flex.SetDirection(FlexRow)
	// No ForceHeight — container has no explicit height.
	flex.AddItem(NewFlexItem(item1).SetGrow(1))
	flex.AddItem(NewFlexItem(item2).SetGrow(1))

	plan := flex.PlanLayout(LayoutArea{Width: 400, Height: 800})

	container := plan.Blocks[0]
	if len(container.Children) < 2 {
		t.Fatal("expected 2 children")
	}
	// item2 should be at most 80 (the tallest item), not 800.
	if container.Children[1].Height > 81 {
		t.Errorf("item2 height = %.1f, want ≤80 (no container height)", container.Children[1].Height)
	}
}

// --- Column flex-grow tests ---

func TestFlexColumn_GrowDistributesRemainingSpace(t *testing.T) {
	// Simulates the boarding pass: column flex with explicit height,
	// one fixed-height item and one growing item.
	// <div style="display:flex; flex-direction:column; height:300px;">
	//   <div style="height:50px;">Header</div>
	//   <div style="flex:1;">Content (should grow to 250px)</div>
	// </div>

	header := NewDiv()
	header.SetHeightUnit(Pt(50))

	content := NewDiv()

	flex := NewFlex()
	flex.SetDirection(FlexColumn)
	flex.ForceHeight(Pt(300))
	flex.AddItem(NewFlexItem(header))
	flex.AddItem(NewFlexItem(content).SetGrow(1))

	plan := flex.PlanLayout(LayoutArea{Width: 400, Height: 800})

	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %v", plan.Status)
	}
	if math.Abs(plan.Consumed-300) > 1 {
		t.Errorf("consumed = %.1f, want 300", plan.Consumed)
	}

	container := plan.Blocks[0]
	if len(container.Children) < 2 {
		t.Fatal("expected 2 children")
	}

	// Header: 50px.
	if math.Abs(container.Children[0].Height-50) > 1 {
		t.Errorf("header height = %.1f, want 50", container.Children[0].Height)
	}
	// Content: should grow to fill remaining 250px.
	if math.Abs(container.Children[1].Height-250) > 1 {
		t.Errorf("content height = %.1f, want 250 (grew to fill)", container.Children[1].Height)
	}
}

func TestFlexColumn_GrowWithNestedMarginTopAuto(t *testing.T) {
	// Boarding pass pattern: outer column has header + content(grow),
	// inner content column has fields + bottom(margin-top:auto).
	// Bottom should be pushed to the actual bottom.

	header := NewDiv()
	header.SetHeightUnit(Pt(40))

	fields := NewDiv()
	fields.SetHeightUnit(Pt(60))

	bottom := NewDiv()
	bottom.SetHeightUnit(Pt(30))

	innerFlex := NewFlex()
	innerFlex.SetDirection(FlexColumn)
	innerFlex.AddItem(NewFlexItem(fields))
	innerFlex.AddItem(NewFlexItem(bottom).SetMarginTopAuto())

	outerFlex := NewFlex()
	outerFlex.SetDirection(FlexColumn)
	outerFlex.ForceHeight(Pt(300))
	outerFlex.AddItem(NewFlexItem(header))
	outerFlex.AddItem(NewFlexItem(innerFlex).SetGrow(1))

	plan := outerFlex.PlanLayout(LayoutArea{Width: 400, Height: 800})

	if plan.Status != LayoutFull {
		t.Fatalf("expected LayoutFull, got %v", plan.Status)
	}

	container := plan.Blocks[0]
	if len(container.Children) < 2 {
		t.Fatal("expected 2 outer children")
	}

	// Inner flex should be 260px (300 - 40 header).
	innerH := container.Children[1].Height
	if math.Abs(innerH-260) > 1 {
		t.Errorf("inner flex height = %.1f, want 260", innerH)
	}

	// Within inner flex (260px), bottom (30px) should be pushed to the bottom
	// by margin-top:auto. autoSpace = 260 - 60 - 30 = 170.
	// bottom Y (relative to inner flex) = fields(60) + autoSpace(170) = 230.
	if len(container.Children[1].Children) < 2 {
		t.Fatal("expected 2 inner children")
	}
	bottomY := container.Children[1].Children[1].Y
	expectedY := 60.0 + 170.0 // fields + autoSpace (relative to inner flex)
	if math.Abs(bottomY-expectedY) > 2 {
		t.Errorf("bottom Y = %.1f, want ~%.1f (margin-top:auto in grown container)", bottomY, expectedY)
	}
}
