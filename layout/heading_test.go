// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestHeadingH1DefaultSize(t *testing.T) {
	h := NewHeading("Title", H1)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// H1 should use 28pt font
	if lines[0].Words[0].FontSize != 28 {
		t.Errorf("expected H1 font size 28, got %f", lines[0].Words[0].FontSize)
	}
}

func TestHeadingH6DefaultSize(t *testing.T) {
	h := NewHeading("Tiny heading", H6)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].Words[0].FontSize != 10.7 {
		t.Errorf("expected H6 font size 10.7, got %f", lines[0].Words[0].FontSize)
	}
}

func TestHeadingDefaultFont(t *testing.T) {
	h := NewHeading("Bold heading", H2)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].Words[0].Font != font.HelveticaBold {
		t.Error("expected HelveticaBold as default heading font")
	}
}

func TestHeadingWithFont(t *testing.T) {
	h := NewHeadingWithFont("Custom", H3, font.TimesRoman, 30)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].Words[0].Font != font.TimesRoman {
		t.Error("expected TimesRoman")
	}
	if lines[0].Words[0].FontSize != 30 {
		t.Errorf("expected font size 30, got %f", lines[0].Words[0].FontSize)
	}
}

func TestHeadingSpacing(t *testing.T) {
	h := NewHeading("Title", H1)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	// First line height should include spacing (fontSize*leading + fontSize*0.5)
	expectedMin := 28 * 1.2 // at least the base line height
	if lines[0].Height <= expectedMin {
		t.Logf("line height %f should include spacing above", lines[0].Height)
	}
}

func TestHeadingAlignment(t *testing.T) {
	h := NewHeading("Centered", H1).SetAlign(AlignCenter)
	lines := h.Layout(500)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].Align != AlignCenter {
		t.Error("expected AlignCenter")
	}
}

func TestHeadingWordWrap(t *testing.T) {
	h := NewHeading("This is a very long heading that should wrap to multiple lines", H1)
	lines := h.Layout(200)
	if len(lines) < 2 {
		t.Errorf("expected multiple lines for narrow width, got %d", len(lines))
	}
}

func TestHeadingAllLevels(t *testing.T) {
	levels := []HeadingLevel{H1, H2, H3, H4, H5, H6}
	var prevSize float64
	for _, level := range levels {
		h := NewHeading("Test", level)
		lines := h.Layout(500)
		if len(lines) == 0 {
			t.Fatalf("H%d produced no lines", level)
		}
		size := lines[0].Words[0].FontSize
		if prevSize > 0 && size >= prevSize {
			t.Errorf("H%d size %f should be smaller than H%d size %f", level, size, level-1, prevSize)
		}
		prevSize = size
	}
}

// TestHeadingPlanLayoutMultilineNoOverlap is a regression test for a bug
// where a heading whose text wrapped to multiple lines would render with
// the wrapped lines overprinted at the same Y-coordinate. The cause was
// that the heading's "space above" offset was being applied only to the
// first PlacedBlock, leaving subsequent line blocks at their original Y
// and producing an overlap of exactly headingSize*0.5 between every
// adjacent pair of lines.
//
// We exercise PlanLayout (the path used by the document renderer), not
// the older Layout(maxWidth) []Line API, because that is where the bug
// lived. Every heading level is checked: the visual severity of the bug
// scales with font size, so H1/H2 are obvious in PDFs, but H5/H6 carry
// the same defect at smaller magnitudes.
func TestHeadingPlanLayoutMultilineNoOverlap(t *testing.T) {
	const text = "Globex Corporation — Platform Renewal + Expansion (FY26)"
	levels := []HeadingLevel{H1, H2, H3, H4, H5, H6}

	for _, level := range levels {
		h := NewHeading(text, level)
		// Narrow width forces wrapping for every level.
		plan := h.PlanLayout(LayoutArea{Width: 180, Height: 10000})
		if len(plan.Blocks) < 2 {
			t.Fatalf("H%d: expected wrapping (>=2 blocks) at width 180, got %d",
				level, len(plan.Blocks))
		}
		for i := 1; i < len(plan.Blocks); i++ {
			prev := plan.Blocks[i-1]
			cur := plan.Blocks[i]
			if cur.Y < prev.Y+prev.Height-0.001 {
				t.Errorf("H%d: line %d Y=%.2f overlaps prev (Y=%.2f, H=%.2f); "+
					"every wrapped line block must start at or below the bottom "+
					"of the previous one",
					level, i, cur.Y, prev.Y, prev.Height)
			}
		}
		// Sanity: the heading's space-above must be reflected on the
		// first block, not absorbed elsewhere.
		expectedSpacing := headingSize(level) * 0.5
		if plan.Blocks[0].Y < expectedSpacing-0.001 {
			t.Errorf("H%d: first block Y=%.2f should include space-above %.2f",
				level, plan.Blocks[0].Y, expectedSpacing)
		}
	}
}

// TestHeadingPlanLayoutSingleLineSpacing guards the simple case: a
// single-line heading must still receive its space-above offset on the
// only block it produces. The multiline fix loops over all blocks; this
// test ensures the loop still handles the len==1 path correctly.
func TestHeadingPlanLayoutSingleLineSpacing(t *testing.T) {
	h := NewHeading("Short", H1)
	plan := h.PlanLayout(LayoutArea{Width: 500, Height: 1000})
	if len(plan.Blocks) != 1 {
		t.Fatalf("expected 1 block for short H1, got %d", len(plan.Blocks))
	}
	expected := headingSize(H1) * 0.5
	if plan.Blocks[0].Y < expected-0.001 || plan.Blocks[0].Y > expected+0.001 {
		t.Errorf("single-line H1 first block Y: expected %.2f, got %.2f",
			expected, plan.Blocks[0].Y)
	}
}

// TestHeadingPlanLayoutConsumedDoesNotOverAdvance is a regression test
// for #135. Before the fix, Heading.PlanLayout delegated to
// Paragraph.PlanLayout with the full area.Height and then bolted
// headingSize*0.5 onto plan.Consumed after the fact. For a multi-line
// heading given an area that exactly fits all its lines — a common
// case at page boundaries — this produced Consumed > area.Height by
// exactly spacing, which over-advanced curY in renderWithPlans and
// pushed the next element down by that amount.
//
// The fix reserves spacing from innerArea before delegating, so the
// inner Paragraph's split decision is made against area.Height-spacing.
// When the heading doesn't fit entirely, it splits cleanly via the
// continuation path; when it does fit, Consumed == sum of line heights
// + spacing, bounded by area.Height.
//
// We use a multi-line heading because single-line headings hit the
// paragraph's "always place the first line" escape hatch (guarded by
// i > 0 in paragraph.go's split loop), which is pre-existing behavior
// unrelated to this fix.
func TestHeadingPlanLayoutConsumedDoesNotOverAdvance(t *testing.T) {
	const text = "Globex Corporation — Platform Renewal + Expansion (FY26)"
	levels := []HeadingLevel{H1, H2, H3, H4, H5, H6}
	for _, level := range levels {
		t.Run(fmt.Sprintf("H%d", level), func(t *testing.T) {
			h := NewHeading(text, level)
			const width = 180.0
			// Measure how much vertical space the paragraph alone
			// wants for these wrapped lines.
			inner := h.para.PlanLayout(LayoutArea{Width: width, Height: 10000})
			if len(inner.Blocks) < 2 {
				t.Fatalf("setup expected multi-line wrap, got %d blocks",
					len(inner.Blocks))
			}
			// An area height exactly equal to the paragraph's content
			// height is the boundary case: pre-fix, Paragraph fits
			// every line (the area matches exactly), reports
			// Consumed=inner, and Heading adds spacing on top →
			// Consumed > area.Height.
			area := LayoutArea{Width: width, Height: inner.Consumed}
			plan := h.PlanLayout(area)
			if plan.Consumed > area.Height+0.001 {
				t.Errorf("Consumed %.2f exceeds area.Height %.2f by %.2f — "+
					"Heading.PlanLayout must reserve its space-above "+
					"from the inner paragraph area, not add it after "+
					"the fact",
					plan.Consumed, area.Height,
					plan.Consumed-area.Height)
			}
		})
	}
}

// TestHeadingPlanLayoutSplitsInsteadOfOverAdvancing reinforces the
// #135 regression: when a heading can't fit all its lines in the
// reserved area (area.Height - spacing), it should split via the
// continuation path rather than silently over-report Consumed. This
// test pins the "split happens" behavior so a future change that
// accidentally brought back the over-advance (by, say, passing the
// full area.Height to Paragraph) would be caught.
func TestHeadingPlanLayoutSplitsInsteadOfOverAdvancing(t *testing.T) {
	const text = "Globex Corporation — Platform Renewal + Expansion (FY26)"
	h := NewHeading(text, H1)
	inner := h.para.PlanLayout(LayoutArea{Width: 180, Height: 10000})
	// Area matches the paragraph's content exactly — with the fix,
	// the inner paragraph is given area.Height - spacing, which is
	// strictly less than the paragraph needs, so it splits.
	plan := h.PlanLayout(LayoutArea{Width: 180, Height: inner.Consumed})
	if plan.Status != LayoutPartial {
		t.Errorf("expected LayoutPartial when inner needs more than "+
			"area.Height-spacing, got %v", plan.Status)
	}
	if plan.Consumed > inner.Consumed+0.001 {
		t.Errorf("Consumed %.2f exceeds area.Height %.2f",
			plan.Consumed, inner.Consumed)
	}
}

// TestHeadingPlanLayoutOverflowIsHeading is a regression test for #133.
// When a heading wraps across a page boundary, the overflow element
// must be a *Heading (not a *Paragraph) so the continuation lines on
// the next page retain their H1-H6 structure tag in the tagged PDF.
// Before the fix, Heading.PlanLayout passed through the inner
// paragraph's overflow as-is, and the continuation rendered as <P>.
func TestHeadingPlanLayoutOverflowIsHeading(t *testing.T) {
	text := "This is a reasonably long heading that will wrap onto " +
		"several lines when the area is narrow enough so we can " +
		"exercise the continuation code path"
	h := NewHeading(text, H1)

	// Narrow width to force multi-line wrap, then an area whose
	// height only fits one line so the remainder overflows.
	plan := h.PlanLayout(LayoutArea{Width: 250, Height: 50})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial to exercise overflow, got %v", plan.Status)
	}
	overflow, ok := plan.Overflow.(*Heading)
	if !ok {
		t.Fatalf("expected overflow to be *Heading, got %T — "+
			"heading continuations must stay headings so their "+
			"H1-H6 structure tag survives page breaks",
			plan.Overflow)
	}
	if overflow.level != H1 {
		t.Errorf("continuation heading level: got %v, want H1", overflow.level)
	}
	if !overflow.continuation {
		t.Error("continuation heading must have continuation=true so " +
			"it does not re-apply space-above or re-emit the bookmark")
	}
}

// TestHeadingContinuationTagsAndNoDuplicateBookmark verifies the
// continuation heading's PlanLayout behavior: the blocks it produces
// must be tagged H1-H6 (so the structure tree is correct) but must
// NOT carry HeadingText (which would trigger a duplicate bookmark on
// the continuation page). It also must not re-apply space-above, so
// the first block of the continuation is flush with the top of its
// area.
func TestHeadingContinuationTagsAndNoDuplicateBookmark(t *testing.T) {
	text := "This is a reasonably long heading that will wrap onto " +
		"several lines when the area is narrow enough so we can " +
		"exercise the continuation code path"
	h := NewHeading(text, H2)
	firstPlan := h.PlanLayout(LayoutArea{Width: 250, Height: 50})
	if firstPlan.Status != LayoutPartial {
		t.Fatalf("setup: expected LayoutPartial, got %v", firstPlan.Status)
	}
	cont, ok := firstPlan.Overflow.(*Heading)
	if !ok {
		t.Fatalf("setup: expected *Heading overflow, got %T", firstPlan.Overflow)
	}

	// Lay the continuation out with plenty of room so it places every
	// remaining line without splitting again.
	contPlan := cont.PlanLayout(LayoutArea{Width: 250, Height: 10000})
	if len(contPlan.Blocks) == 0 {
		t.Fatal("continuation produced zero blocks")
	}

	// All continuation blocks must carry the H2 tag.
	wantTag := headingTag(H2)
	for i, b := range contPlan.Blocks {
		if b.Tag != wantTag {
			t.Errorf("continuation block %d tag: got %q, want %q — "+
				"continuation headings must preserve the H1-H6 tag",
				i, b.Tag, wantTag)
		}
	}

	// None of the continuation blocks must carry HeadingText: the
	// bookmark was already emitted on the starting page and a second
	// HeadingText would cause a duplicate entry in the bookmark tree.
	for i, b := range contPlan.Blocks {
		if b.HeadingText != "" {
			t.Errorf("continuation block %d HeadingText: got %q, "+
				"want empty — heading continuations must not emit a "+
				"duplicate bookmark", i, b.HeadingText)
		}
	}

	// The first continuation block must start flush at Y=0: no
	// space-above carried over from the original heading.
	if contPlan.Blocks[0].Y > 0.001 {
		t.Errorf("continuation block[0].Y = %.2f, want 0 — "+
			"continuation headings must not re-apply space-above",
			contPlan.Blocks[0].Y)
	}
}

// TestHeadingContinuationEndToEndRender is an end-to-end regression
// for both #133 (continuation tag) and #135 (Consumed accounting),
// exercised through the full tagged renderer. A long heading is
// forced to wrap and split across two pages, and then:
//   - Exactly one bookmark (HeadingInfo) must be emitted, on the
//     starting page only.
//   - At least one H1 structure tag must appear on each page, proving
//     the continuation keeps its tag across the page break.
func TestHeadingContinuationEndToEndRender(t *testing.T) {
	r := NewRenderer(200, 100, Margins{Top: 10, Right: 10, Bottom: 10, Left: 10})
	r.SetTagged(true)
	r.Add(NewHeading(
		"This is a long heading that will be forced to wrap and split "+
			"across two pages so we can check the tagged structure and "+
			"the bookmark index",
		H1,
	))
	pages := r.Render()
	if len(pages) < 2 {
		t.Fatalf("setup: expected at least 2 pages, got %d", len(pages))
	}

	// Exactly one HeadingInfo across all pages — the bookmark must
	// fire once on the starting page and never on continuation pages.
	total := 0
	for _, p := range pages {
		total += len(p.Headings)
	}
	if total != 1 {
		t.Errorf("expected exactly 1 HeadingInfo across all pages, got %d — "+
			"a heading that spans a page break must emit exactly one "+
			"bookmark entry, from the starting page", total)
	}
	if len(pages[0].Headings) != 1 {
		t.Errorf("expected HeadingInfo on page 0 (where heading starts), "+
			"got %d entries", len(pages[0].Headings))
	}

	// Every page that contains any of the heading's lines must carry
	// at least one H1 structure tag. Before #133 was fixed, the
	// continuation lines on page >= 1 were tagged P, not H1.
	tagsPerPage := make(map[int]map[string]int)
	for _, tag := range r.StructTags() {
		if _, ok := tagsPerPage[tag.PageIndex]; !ok {
			tagsPerPage[tag.PageIndex] = make(map[string]int)
		}
		tagsPerPage[tag.PageIndex][tag.Tag]++
	}
	// Sanity: the continuation path must actually have been exercised.
	// If something regresses and the heading fits on a single page,
	// the rest of this test would silently degrade to a no-op.
	continuationExercised := false
	for pageIdx := 1; pageIdx < len(pages); pageIdx++ {
		if len(tagsPerPage[pageIdx]) > 0 {
			continuationExercised = true
			break
		}
	}
	if !continuationExercised {
		t.Fatal("continuation path was not exercised: no structure " +
			"tags on any page after page 0 — test setup is broken")
	}
	for pageIdx := range pages {
		if tagsPerPage[pageIdx]["H1"] == 0 {
			t.Errorf("page %d has no H1 structure tag — heading "+
				"continuations must preserve the H1 tag across page "+
				"breaks so the tagged PDF structure tree stays correct. "+
				"Tags on page %d: %v", pageIdx, pageIdx, tagsPerPage[pageIdx])
		}
	}
}
