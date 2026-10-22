// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// drawnYBand scans a page content stream for the minimum and maximum
// y-coordinate touched by text-positioning (Td) and vector (re/m/l)
// operators — a proxy for how far the page's content extends toward the
// bottom (min) and top (max) in absolute PDF page coordinates (origin at the
// page's bottom-left). Returns found=false when the page drew nothing.
func drawnYBand(stream string) (minY, maxY float64, found bool) {
	minY, maxY = 1e18, -1e18
	consider := func(y float64) {
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
		found = true
	}
	for _, line := range strings.Split(stream, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		switch f[len(f)-1] {
		case "Td", "m", "l":
			var y float64
			if _, err := fmt.Sscanf(f[len(f)-2], "%g", &y); err == nil {
				consider(y)
			}
		case "re": // x y w h re
			if len(f) >= 5 {
				var y, h float64
				_, errY := fmt.Sscanf(f[len(f)-4], "%g", &y)
				_, errH := fmt.Sscanf(f[len(f)-2], "%g", &h)
				if errY == nil && errH == nil {
					consider(y)
					consider(y + h)
				}
			}
		}
	}
	return
}

func tableColumn(label string, rows int) *Div {
	d := NewDiv()
	tbl := NewTable().SetAutoColumnWidths()
	for i := 0; i < rows; i++ {
		tbl.AddRow().AddCell(fmt.Sprintf("%s row %d", label, i), font.Helvetica, 10)
	}
	d.Add(tbl)
	return d
}

// TestFragmentedContentStaysWithinPageMargins is the regression test for a
// fragmenter overrunning the page's bottom margin. A tall two-column flex row
// wrapped in a section Div (the shape a real report uses) is fragmented across
// several pages; every page's drawn content must stay within the content band
// [bottomMargin, pageHeight-topMargin] — no rows in the bottom margin, nothing
// past the page edge.
//
// Two defects produced the overrun, both in Div.PlanLayout's partial-height
// accounting: (1) the LayoutPartial child branch never advanced curY, so the
// container reported Consumed ≈ 0 while having placed a full page of content;
// (2) innerHeight omitted the box's own spaceBefore/spaceAfter, so children
// filled the whole area and the spacing then pushed the total past the height
// the box was given. Pre-fix the worst page's content reached ~12pt into the
// 40pt bottom margin here (and far worse on deeper real documents).
func TestFragmentedContentStaysWithinPageMargins(t *testing.T) {
	const (
		pageW, pageH   = 595.0, 842.0
		top, bottom    = 40.0, 40.0
		tol            = 2.0 // text baselines sit a few pt above the glyph bottom
		contentTop     = pageH - top
		contentBottomY = bottom
	)

	r := NewRenderer(pageW, pageH, Margins{Top: top, Right: 40, Bottom: bottom, Left: 40})
	r.Add(NewParagraph("Report header before the tables", font.Helvetica, 12))

	left := tableColumn("Informacion solicitudes", 50)
	right := tableColumn("Actuaciones", 50)
	row := NewFlex().
		AddItem(NewFlexItem(left).SetBasis(250)).
		AddItem(NewFlexItem(right).SetBasis(250))
	section := NewDiv()
	section.SetSpaceBefore(20) // a section margin — part of what triggered the overrun
	section.Add(row)
	r.Add(section)

	pages := r.Render()
	if len(pages) < 2 {
		t.Fatalf("expected the tall row to fragment across multiple pages, got %d", len(pages))
	}

	for i, pg := range pages {
		minY, maxY, ok := drawnYBand(string(pg.Stream.Bytes()))
		if !ok {
			continue
		}
		if minY < contentBottomY-tol {
			t.Errorf("page %d: content reaches y=%.1f, below the bottom margin (%.1f) — overruns into/past the margin", i, minY, contentBottomY)
		}
		if maxY > contentTop+tol {
			t.Errorf("page %d: content reaches y=%.1f, above the top-margin line (%.1f)", i, maxY, contentTop)
		}
	}
}

// TestNonFragmentedPageRespectsMarginsUnchanged is the control: content that
// fits on one page is untouched by the fragmentation-path fixes and still sits
// within the margins.
func TestNonFragmentedPageRespectsMarginsUnchanged(t *testing.T) {
	const (
		pageW, pageH = 595.0, 842.0
		top, bottom  = 40.0, 40.0
		tol          = 2.0
	)
	r := NewRenderer(pageW, pageH, Margins{Top: top, Right: 40, Bottom: bottom, Left: 40})
	section := NewDiv()
	section.SetSpaceBefore(20)
	section.Add(tableColumn("Short", 5))
	r.Add(section)

	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected a single page for short content, got %d", len(pages))
	}
	minY, maxY, ok := drawnYBand(string(pages[0].Stream.Bytes()))
	if !ok {
		t.Fatal("page drew no content")
	}
	if minY < bottom-tol {
		t.Errorf("content reaches y=%.1f, below the bottom margin (%.1f)", minY, bottom)
	}
	if maxY > pageH-top+tol {
		t.Errorf("content reaches y=%.1f, above the top-margin line (%.1f)", maxY, pageH-top)
	}
}

// TestFragmentingDivReportsConsumedMatchingContent pins the direct invariant
// behind the first defect: a Div that fragments a child across pages must
// report a Consumed height equal to the content it actually placed, not ~0.
// A container that under-reports its height desynchronizes the renderer's page
// accounting (the root of the margin overrun above).
func TestFragmentingDivReportsConsumedMatchingContent(t *testing.T) {
	build := func() *Div {
		d := NewDiv()
		tbl := NewTable().SetAutoColumnWidths()
		for i := 0; i < 40; i++ {
			tbl.AddRow().AddCell(fmt.Sprintf("row %d", i), font.Helvetica, 10)
		}
		d.Add(tbl)
		return d
	}

	var elem Element = build()
	for page := 0; page < 20 && elem != nil; page++ {
		plan := elem.PlanLayout(LayoutArea{Width: 300, Height: 100})
		content := maxLeafBottomFPM(plan.Blocks)
		if content > 0 && plan.Consumed < content-0.5 {
			t.Errorf("page %d: Div reported Consumed=%.1f but placed content down to %.1f — under-reported height", page, plan.Consumed, content)
		}
		if plan.Status == LayoutPartial && plan.Overflow != nil {
			elem = plan.Overflow
		} else {
			elem = nil
		}
	}
}

func maxLeafBottomFPM(blocks []PlacedBlock) float64 {
	m := 0.0
	for i := range blocks {
		b := blocks[i]
		if len(b.Children) == 0 {
			if bot := b.Y + b.Height; bot > m {
				m = bot
			}
		} else if bot := maxLeafBottomFPM(b.Children); bot > m {
			m = bot
		}
	}
	return m
}
