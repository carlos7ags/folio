// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Table-rowspan creates a multi-page PDF demonstrating cells that span
// multiple rows (rowspan) alongside multi-column spans (colspan), including
// a long table whose rowspan group is pushed intact across a page break.
//
// Usage:
//
//	go run ./examples/table-rowspan
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

func main() {
	if err := buildDocument().Save("table-rowspan.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created table-rowspan.pdf")
}

// buildDocument assembles three sections: the minimal rowspan case from
// issue #357, a richer schedule grid where a cell spans three rows, and a
// long roster table that crosses a page break with a rowspan group intact
// (issue #362). Extracted from main() so the example test can build the
// same document against an in-memory buffer.
func buildDocument() *document.Document {
	doc := document.NewDocument(document.PageSizeA4)
	doc.Info.Title = "Table Rowspan"
	doc.Info.Author = "Folio"

	doc.Add(layout.NewHeading("Rowspan", layout.H1))
	doc.Add(layout.NewParagraph(
		"The first column spans both rows; the second column has one cell per row.",
		font.Helvetica, 11,
	))

	basic := layout.NewTable()
	r1 := basic.AddRow()
	r1.AddCell("Span", font.HelveticaBold, 10).SetRowspan(2).SetVAlign(layout.VAlignMiddle)
	r1.AddCell("B1", font.Helvetica, 10)
	r2 := basic.AddRow()
	r2.AddCell("B2", font.Helvetica, 10)
	doc.Add(basic)

	doc.Add(layout.NewHeading("Schedule grid", layout.H2))
	doc.Add(layout.NewParagraph(
		"\"Morning\" spans three time slots; \"All day\" spans the whole column.",
		font.Helvetica, 11,
	))

	sched := layout.NewTable()
	sched.SetColumnWidths([]float64{90, 160, 160})

	h := sched.AddHeaderRow()
	h.AddCell("Block", font.HelveticaBold, 10)
	h.AddCell("Track A", font.HelveticaBold, 10)
	h.AddCell("Track B", font.HelveticaBold, 10)

	row1 := sched.AddRow()
	row1.AddCell("Morning", font.HelveticaBold, 10).SetRowspan(3).SetVAlign(layout.VAlignMiddle)
	row1.AddCell("Registration", font.Helvetica, 10)
	row1.AddCell("All day: help desk", font.Helvetica, 10).SetRowspan(3).SetVAlign(layout.VAlignMiddle)

	row2 := sched.AddRow()
	row2.AddCell("Keynote", font.Helvetica, 10)

	row3 := sched.AddRow()
	row3.AddCell("Workshop intro", font.Helvetica, 10)

	doc.Add(sched)

	doc.Add(layout.NewHeading("Rowspan across a page break", layout.H2))
	doc.Add(layout.NewParagraph(
		"A long roster where a rowspanning \"Platform Team\" cell would normally "+
			"land right at the page boundary. The whole group is pushed to the "+
			"next page instead of being split mid-span.",
		font.Helvetica, 11,
	))

	roster := layout.NewTable()
	roster.SetColumnWidths([]float64{90, 160, 160})

	rh := roster.AddHeaderRow()
	rh.AddCell("Team", font.HelveticaBold, 10)
	rh.AddCell("Member", font.HelveticaBold, 10)
	rh.AddCell("Role", font.HelveticaBold, 10)

	// Enough plain rows to push the page boundary right up against the
	// rowspan group below, then a 4-row group so the split can't land
	// inside it — the whole group moves to the next page.
	for i := 1; i <= 24; i++ {
		row := roster.AddRow()
		row.AddCell(fmt.Sprintf("Team %d", i), font.Helvetica, 10)
		row.AddCell(fmt.Sprintf("Member %d", i), font.Helvetica, 10)
		row.AddCell("Contributor", font.Helvetica, 10)
	}

	group := roster.AddRow()
	group.AddCell("Platform Team", font.HelveticaBold, 10).SetRowspan(4).SetVAlign(layout.VAlignMiddle)
	group.AddCell("Alice", font.Helvetica, 10)
	group.AddCell("Lead", font.Helvetica, 10)

	member2 := roster.AddRow()
	member2.AddCell("Bilal", font.Helvetica, 10)
	member2.AddCell("Engineer", font.Helvetica, 10)

	member3 := roster.AddRow()
	member3.AddCell("Chen", font.Helvetica, 10)
	member3.AddCell("Engineer", font.Helvetica, 10)

	member4 := roster.AddRow()
	member4.AddCell("Divya", font.Helvetica, 10)
	member4.AddCell("Engineer", font.Helvetica, 10)

	for i := 25; i <= 30; i++ {
		row := roster.AddRow()
		row.AddCell(fmt.Sprintf("Team %d", i), font.Helvetica, 10)
		row.AddCell(fmt.Sprintf("Member %d", i), font.Helvetica, 10)
		row.AddCell("Contributor", font.Helvetica, 10)
	}

	doc.Add(roster)

	return doc
}
