// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "testing"

// drawSpy is an unsplittable box (LayoutNothing when it does not fit, like
// atomicElement) whose Draw records the page and position it was drawn at,
// so a test can assert every block is drawn exactly once and on the page.
type drawSpy struct {
	id     int
	width  float64
	height float64
	draws  *[]spyDraw
}

type spyDraw struct {
	id     int
	page   int
	topY   float64
	height float64
}

func (s *drawSpy) PlanLayout(area LayoutArea) LayoutPlan {
	if s.height > area.Height && area.Height > 0 {
		return LayoutPlan{Status: LayoutNothing}
	}
	id, h, draws := s.id, s.height, s.draws
	return LayoutPlan{
		Status:   LayoutFull,
		Consumed: s.height,
		Blocks: []PlacedBlock{{
			Width:  s.width,
			Height: s.height,
			Draw: func(ctx DrawContext, _, topY float64) {
				*draws = append(*draws, spyDraw{id: id, page: ctx.PageIdx, topY: topY, height: h})
			},
		}},
	}
}

const (
	ovPageH  = 800.0
	ovBlocks = 20
	ovBlockH = 100.0
	ovFloat  = -1 // id of the floated icon
	ovTall   = -2 // id of the oversized atomic child
)

// oversizedContainer builds <div>[float] tall + 20×100pt blocks</div>. tall is
// the oversized child (taller than a whole page).
func oversizedContainer(draws *[]spyDraw, withFloat bool, tall Element) *Div {
	d := NewDiv()
	if withFloat {
		d.Add(NewFloat(FloatLeft, &drawSpy{id: ovFloat, width: 15, height: 15, draws: draws}))
	}
	d.Add(tall)
	for i := 0; i < ovBlocks; i++ {
		d.Add(&drawSpy{id: i, width: 100, height: ovBlockH, draws: draws})
	}
	return d
}

// assertEachBlockOnce checks every numbered block is drawn exactly once and
// lies within its page, and the float (if any) is drawn exactly once.
func assertEachBlockOnce(t *testing.T, draws []spyDraw, withFloat bool) {
	t.Helper()
	seen := map[int]int{}
	for _, d := range draws {
		seen[d.id]++
		if d.id < 0 {
			continue
		}
		if d.topY > ovPageH+0.01 || d.topY-d.height < -0.01 {
			t.Errorf("block %d drawn off page %d (top=%.1f, bottom=%.1f)", d.id, d.page, d.topY, d.topY-d.height)
		}
	}
	for i := 0; i < ovBlocks; i++ {
		if seen[i] != 1 {
			t.Errorf("block %d drawn %d times, want 1", i, seen[i])
		}
	}
	if withFloat && seen[ovFloat] != 1 {
		t.Errorf("float drawn %d times, want 1", seen[ovFloat])
	}
}

// TestOversizedChildAfterFloatPaginatesSiblings is the regression test for a
// container holding a float plus a child taller than a whole page. The float
// counted as progress, so the oversized child was deferred with it; deferWith
// rolled the float back, the container placed nothing and returned
// LayoutNothing, and at the page top the renderer force-placed the whole
// container at unbounded height — everything after the oversized child was
// drawn off the bottom of a single page. At the top of a page the container
// now places the oversized child itself and paginates the rest.
func TestOversizedChildAfterFloatPaginatesSiblings(t *testing.T) {
	for _, tc := range []struct {
		name      string
		withFloat bool
		lead      float64 // height of a top-level block before the container (0 = none)
		atomic    bool    // oversized child is atomic (LayoutNothing) vs a definite-height div (LayoutFull)
		wantPages int
	}{
		{"definite-height div, no float", false, 0, false, 4},
		{"definite-height div, float", true, 0, false, 4},
		{"atomic, no float", false, 0, true, 4},
		{"atomic, float", true, 0, true, 4},
		{"definite-height div, float, mid-page", true, 300, false, 5},
		{"atomic, float, mid-page", true, 300, true, 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var draws []spyDraw
			var tall Element = NewDiv().SetHeightUnit(Pt(1000))
			if tc.atomic {
				tall = &drawSpy{id: ovTall, width: 100, height: 1000, draws: &draws}
			}
			r := NewRenderer(500, ovPageH, Margins{})
			if tc.lead > 0 {
				r.Add(&drawSpy{id: -3, width: 100, height: tc.lead, draws: &draws})
			}
			r.Add(oversizedContainer(&draws, tc.withFloat, tall))
			pages := r.Render()
			if len(pages) != tc.wantPages {
				t.Errorf("got %d pages, want %d", len(pages), tc.wantPages)
			}
			assertEachBlockOnce(t, draws, tc.withFloat)
			if tc.atomic {
				n := 0
				for _, d := range draws {
					if d.id == ovTall {
						n++
					}
				}
				if n != 1 {
					t.Errorf("oversized child drawn %d times, want 1", n)
				}
			}
		})
	}
}

// TestDivAtFragmentTopPlacesOversizedChild checks the planning contract
// directly: mid-page the float-and-oversized-child container defers whole
// (LayoutNothing, so the renderer moves it to a fresh page), while at the
// top of a page it places the float and the oversized child and hands the
// remaining siblings to its overflow.
func TestDivAtFragmentTopPlacesOversizedChild(t *testing.T) {
	var draws []spyDraw
	build := func() *Div {
		return oversizedContainer(&draws, true, NewDiv().SetHeightUnit(Pt(1000)))
	}

	mid := build().PlanLayout(LayoutArea{Width: 400, Height: ovPageH})
	if mid.Status != LayoutNothing {
		t.Errorf("mid-page status = %v, want LayoutNothing (defer with the float)", mid.Status)
	}

	top := build().PlanLayout(LayoutArea{Width: 400, Height: ovPageH, AtFragmentTop: true})
	if top.Status != LayoutPartial {
		t.Fatalf("at-top status = %v, want LayoutPartial", top.Status)
	}
	if top.Overflow == nil {
		t.Fatal("at-top plan has no overflow: the 20 blocks after the oversized child were lost")
	}
	if len(top.Blocks) != 1 || countFloatBlocks(top.Blocks[0].Children) != 1 {
		t.Errorf("at-top plan should place the float on this page")
	}
}

// TestDivPaddingTallerThanPageNoPagePerChild is the regression test for a
// container whose top padding alone exceeds the page: at the top of a fresh
// page its inner height is 0, so the at-top force-place took one child per
// page and every continuation (which repeats the padding) did the same —
// 40 children gave 40 pages. Such a container is placed in one go instead.
func TestDivPaddingTallerThanPageNoPagePerChild(t *testing.T) {
	var draws []spyDraw
	d := NewDiv().SetPaddingAll(Padding{Top: 760}).SetBackground(RGB(0.9, 0.9, 0.9))
	const n = 40
	for i := 0; i < n; i++ {
		d.Add(&drawSpy{id: i, width: 100, height: 14, draws: &draws})
	}
	r := NewRenderer(500, 720, Margins{})
	r.Add(d)
	pages := r.Render()
	if len(pages) > 2 {
		t.Errorf("got %d pages, want at most 2 (no page per child)", len(pages))
	}
	seen := map[int]int{}
	for _, dr := range draws {
		seen[dr.id]++
	}
	for i := 0; i < n; i++ {
		if seen[i] != 1 {
			t.Errorf("block %d drawn %d times, want 1", i, seen[i])
		}
	}
}
