// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Report demonstrates building a multi-page business document using
// the Go layout API directly — no HTML involved.
//
// Layout features exercised:
//   - Headings (H1–H3) with auto-generated PDF bookmarks
//   - Rich text paragraphs (mixed fonts, colors, sizes)
//   - Tables with styled headers, borders, and right-aligned columns
//   - Ordered and unordered lists
//   - Divs with background color, borders, padding, and border-radius
//   - Line separators
//   - Page breaks (AreaBreak)
//   - Headers and footers with page numbers
//
// Usage:
//
//	go run ./examples/report
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

var (
	navy = layout.RGB(0.06, 0.09, 0.16)
	teal = layout.RGB(0.05, 0.61, 0.53)
	gray = layout.RGB(0.39, 0.43, 0.47)
)

func main() {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetMargins(layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	doc.Info.Title = "Annual Report 2026"
	doc.Info.Author = "Apex Capital Partners"
	doc.SetAutoBookmarks(true)

	// --- Header / Footer ---
	doc.SetFooter(func(ctx document.PageContext, p *document.Page) {
		if ctx.PageIndex == 0 {
			return
		}
		p.AddText(
			fmt.Sprintf("Page %d of %d", ctx.PageIndex+1, ctx.TotalPages),
			font.Helvetica, 8, 480, 40,
		)
		p.AddText("Apex Capital Partners — Confidential",
			font.Helvetica, 8, 72, 40,
		)
	})

	// ==================== PAGE 1: Cover ====================
	doc.Add(spacer(160))
	doc.Add(layout.NewParagraph("Annual Report", font.HelveticaBold, 36).
		SetAlign(layout.AlignCenter))
	doc.Add(spacer(4))
	doc.Add(layout.NewParagraph("2026", font.HelveticaBold, 48).
		SetAlign(layout.AlignCenter))
	doc.Add(sep())
	doc.Add(layout.NewParagraph("Apex Capital Partners", font.Helvetica, 14).
		SetAlign(layout.AlignCenter))

	// ==================== PAGE 2: Executive Summary ====================
	doc.Add(layout.NewAreaBreak())
	doc.Add(layout.NewHeading("Executive Summary", layout.H1))
	doc.Add(body(
		"Fiscal year 2026 marked a transformational period for Apex Capital Partners. "+
			"Revenue grew 22% year-over-year to $28.3M, driven by strong performance "+
			"in advisory services and a strategic expansion into the Asia-Pacific region."))
	doc.Add(body(
		"Operating margin improved to 30%, reflecting disciplined cost management "+
			"and technology-driven efficiency gains. Client retention remained strong "+
			"at 97.2%, and we welcomed 23 new institutional clients during the year."))

	// Callout box
	callout := layout.NewDiv().
		SetPaddingAll(layout.Padding{Top: 10, Right: 14, Bottom: 10, Left: 14}).
		SetBorders(layout.CellBorders{
			Left: layout.Border{Width: 3, Color: teal, Style: layout.BorderSolid},
		}).
		SetBackground(layout.RGB(0.97, 0.98, 1.0)).
		SetSpaceBefore(8).
		SetSpaceAfter(12)
	callout.Add(layout.NewStyledParagraph(
		layout.Run("Highlight: ", font.HelveticaBold, 10).WithColor(navy),
		layout.Run("Named \"Top Advisory Firm\" by the Financial Times for the third consecutive year.", font.Helvetica, 10),
	))
	doc.Add(callout)

	// --- Financial Highlights ---
	doc.Add(layout.NewHeading("Financial Highlights", layout.H2))
	doc.Add(financialTable())

	// ==================== PAGE 3: Segments + Strategy ====================
	doc.Add(layout.NewAreaBreak())
	doc.Add(layout.NewHeading("Revenue by Segment", layout.H2))
	doc.Add(body("Revenue is diversified across four business segments."))
	doc.Add(revenueTable())

	doc.Add(layout.NewHeading("Strategic Initiatives", layout.H2))
	doc.Add(body("Key accomplishments during 2026:"))

	list := layout.NewList(font.Helvetica, 10)
	list.SetLeading(1.5)
	list.AddItem("Launched digital asset custody platform ($2.1B AUM in first month)")
	list.AddItem("Opened Singapore office with 12 analysts for Asia-Pacific expansion")
	list.AddItem("Signed cross-border payment partnership with Deutsche Bank")
	list.AddItem("Published inaugural ESG Impact Report")
	list.AddItem("Achieved 97% client satisfaction in annual survey")
	doc.Add(list)

	doc.Add(spacer(12))
	callout2 := layout.NewDiv().
		SetPaddingAll(layout.Padding{Top: 10, Right: 14, Bottom: 10, Left: 14}).
		SetBorders(layout.CellBorders{
			Left: layout.Border{Width: 3, Color: layout.RGB(0.96, 0.62, 0.04), Style: layout.BorderSolid},
		}).
		SetBackground(layout.RGB(1.0, 0.99, 0.94))
	callout2.Add(layout.NewStyledParagraph(
		layout.Run("Outlook: ", font.HelveticaBold, 10).WithColor(navy),
		layout.Run("Board approval pending for Series C fund ($500M target). Q1 2027 earnings call scheduled for April 15.", font.Helvetica, 10),
	))
	doc.Add(callout2)

	// ==================== PAGE 4: Team ====================
	doc.Add(layout.NewAreaBreak())
	doc.Add(layout.NewHeading("Leadership Team", layout.H1))
	doc.Add(spacer(4))

	team := []struct{ name, title, bio string }{
		{"Sarah Chen", "Chief Financial Officer",
			"20+ years in investment banking. Previously VP at Goldman Sachs. MBA from Wharton."},
		{"Michael Torres", "Managing Director",
			"Leads client advisory and M&A. Former McKinsey partner. CFA charterholder."},
		{"Priya Patel", "Head of Research",
			"PhD Economics, MIT. Published author on emerging market strategy."},
		{"James Wu", "Chief Technology Officer",
			"Led digital transformation at two Fortune 500 firms. MS Computer Science, Stanford."},
	}

	for _, t := range team {
		card := layout.NewDiv().
			SetPaddingAll(layout.Padding{Top: 8, Right: 12, Bottom: 8, Left: 12}).
			SetBorders(layout.CellBorders{
				Bottom: layout.SolidBorder(1, layout.RGB(0.9, 0.9, 0.9)),
			}).
			SetSpaceAfter(2)

		card.Add(layout.NewStyledParagraph(
			layout.Run(t.name, font.HelveticaBold, 11).WithColor(navy),
			layout.Run("  —  ", font.Helvetica, 10).WithColor(gray),
			layout.Run(t.title, font.Helvetica, 10).WithColor(teal),
		))
		card.Add(layout.NewStyledParagraph(
			layout.Run(t.bio, font.Helvetica, 9).WithColor(gray),
		).SetLeading(1.4).SetSpaceBefore(2))

		doc.Add(card)
	}

	// --- Save ---
	if err := doc.Save("report.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Created report.pdf")
}

func body(text string) *layout.Paragraph {
	return layout.NewParagraph(text, font.Helvetica, 10).
		SetLeading(1.5).
		SetSpaceAfter(6)
}

func spacer(pts float64) *layout.Paragraph {
	p := layout.NewParagraph(" ", font.Helvetica, 1)
	p.SetSpaceBefore(pts)
	return p
}

func sep() *layout.LineSeparator {
	return layout.NewLineSeparator().
		SetWidth(1).
		SetColor(layout.RGB(0.75, 0.75, 0.75)).
		SetSpaceBefore(12).
		SetSpaceAfter(12)
}

func financialTable() *layout.Table {
	tbl := layout.NewTable().
		SetColumnUnitWidths([]layout.UnitValue{
			layout.Pct(40), layout.Pct(20), layout.Pct(20), layout.Pct(20),
		}).
		SetBorderCollapse(true)

	hBorder := layout.CellBorders{Bottom: layout.SolidBorder(2, navy)}
	rBorder := layout.CellBorders{Bottom: layout.SolidBorder(0.5, layout.RGB(0.85, 0.85, 0.85))}
	pad := layout.Padding{Top: 6, Right: 8, Bottom: 6, Left: 8}

	hr := tbl.AddHeaderRow()
	for _, h := range []string{"Metric", "2026", "2025", "Change"} {
		hr.AddCell(h, font.HelveticaBold, 9).SetBorders(hBorder).SetPaddingSides(pad)
	}

	data := [][]string{
		{"Revenue", "$28.3M", "$23.2M", "+22.0%"},
		{"Gross Profit", "$17.0M", "$13.3M", "+27.8%"},
		{"Operating Expenses", "$8.5M", "$7.2M", "+18.1%"},
		{"Net Income", "$6.1M", "$5.2M", "+17.3%"},
		{"Operating Margin", "30.0%", "26.3%", "+3.7pp"},
	}
	for _, row := range data {
		r := tbl.AddRow()
		for j, cell := range row {
			c := r.AddCell(cell, font.Helvetica, 9).SetBorders(rBorder).SetPaddingSides(pad)
			if j > 0 {
				c.SetAlign(layout.AlignRight)
			}
		}
	}
	return tbl
}

func revenueTable() *layout.Table {
	tbl := layout.NewTable().
		SetColumnUnitWidths([]layout.UnitValue{
			layout.Pct(40), layout.Pct(30), layout.Pct(30),
		}).
		SetBorderCollapse(true)

	hBorder := layout.CellBorders{Bottom: layout.SolidBorder(2, navy)}
	rBorder := layout.CellBorders{Bottom: layout.SolidBorder(0.5, layout.RGB(0.85, 0.85, 0.85))}
	pad := layout.Padding{Top: 6, Right: 8, Bottom: 6, Left: 8}

	hr := tbl.AddHeaderRow()
	for _, h := range []string{"Segment", "Revenue", "% of Total"} {
		hr.AddCell(h, font.HelveticaBold, 9).SetBorders(hBorder).SetPaddingSides(pad)
	}

	data := [][]string{
		{"Advisory Services", "$14.2M", "50%"},
		{"Asset Management", "$8.5M", "30%"},
		{"Research & Analytics", "$4.2M", "15%"},
		{"Other", "$1.4M", "5%"},
	}
	for _, row := range data {
		r := tbl.AddRow()
		for j, cell := range row {
			c := r.AddCell(cell, font.Helvetica, 9).SetBorders(rBorder).SetPaddingSides(pad)
			if j > 0 {
				c.SetAlign(layout.AlignRight)
			}
		}
	}
	return tbl
}
