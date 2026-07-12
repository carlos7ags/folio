// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Table-border-collapse demonstrates CSS 2.1 §17.6.2 conflict resolution
// for border-collapse: collapse tables: the wider border on a shared edge
// wins, border-style: hidden always suppresses the edge, and style
// priority (double beats solid) breaks a tie at equal width.
//
// Usage:
//
//	go run ./examples/table-border-collapse
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

func main() {
	if err := buildDocument().Save("table-border-collapse.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created table-border-collapse.pdf")
}

// buildDocument assembles three minimal tables, each isolating one tier of
// the CSS2.1 §17.6.2 conflict-resolution algorithm. Extracted from main()
// so the example test can build the same document against an in-memory
// buffer.
func buildDocument() *document.Document {
	doc := document.NewDocument(document.PageSizeA4)
	doc.Info.Title = "Table Border-Collapse Conflict Resolution"
	doc.Info.Author = "Folio"

	doc.Add(layout.NewHeading("Border-collapse conflict resolution", layout.H1))
	doc.Add(layout.NewParagraph(
		"CSS 2.1 section 17.6.2 resolves competing borders declared on both "+
			"sides of a shared cell edge: hidden always wins, otherwise the "+
			"wider border wins, then higher style priority, then source "+
			"order. Before this fix, folio's collapse code ignored width and "+
			"style entirely and always kept whichever cell came later in "+
			"row/column order — so a narrower or undeclared border could "+
			"silently beat a wider, explicitly declared one.",
		font.Helvetica, 11,
	))

	doc.Add(layout.NewHeading("Wider border wins", layout.H2))
	doc.Add(layout.NewParagraph(
		"The header declares a .25pt bottom border; the body declares an "+
			"explicit 0pt top border (folio has no distinct representation "+
			"for a declared-but-zero border, so this is simply no border). "+
			"Old behavior: the body's row came later, so its empty border "+
			"won and nothing was drawn. Fixed behavior: the header's wider, "+
			"nonzero border wins.",
		font.Helvetica, 11,
	))
	widerWins := layout.NewTable().SetBorderCollapse(true)
	widerWins.SetColumnWidths([]float64{200})
	h1 := widerWins.AddHeaderRow()
	h1.AddCell("Header (.25pt bottom)", font.HelveticaBold, 10).
		SetBorders(layout.CellBorders{Bottom: layout.SolidBorder(0.25, layout.ColorBlack)})
	b1 := widerWins.AddRow()
	b1.AddCell("Body (explicit 0pt top)", font.Helvetica, 10).
		SetBorders(layout.CellBorders{})
	doc.Add(widerWins)

	doc.Add(layout.NewHeading("Hidden always wins", layout.H2))
	doc.Add(layout.NewParagraph(
		"The left cell declares border-style: hidden on its right side; the "+
			"right cell declares a 10pt solid border on its left side. Old "+
			"behavior: whichever cell was later in column order controlled "+
			"the visible width, ignoring hidden entirely. Fixed behavior: "+
			"hidden unconditionally suppresses the edge, even against a "+
			"much wider border.",
		font.Helvetica, 11,
	))
	hiddenWins := layout.NewTable().SetBorderCollapse(true)
	hiddenWins.SetColumnWidths([]float64{100, 100})
	hr := hiddenWins.AddRow()
	hr.AddCell("Hidden", font.Helvetica, 10).
		SetBorders(layout.CellBorders{Right: layout.Border{Style: layout.BorderHidden}})
	hr.AddCell("10pt solid", font.Helvetica, 10).
		SetBorders(layout.CellBorders{Left: layout.SolidBorder(10, layout.ColorBlack)})
	doc.Add(hiddenWins)

	doc.Add(layout.NewHeading("Equal width: style priority", layout.H2))
	doc.Add(layout.NewParagraph(
		"Both rows declare a 2pt border on their shared edge, but one is "+
			"double and the other solid. Old behavior: the later row's "+
			"style won regardless of priority. Fixed behavior: double "+
			"outranks solid at equal width, so it wins.",
		font.Helvetica, 11,
	))
	styleTie := layout.NewTable().SetBorderCollapse(true)
	styleTie.SetColumnWidths([]float64{200})
	t1 := styleTie.AddRow()
	t1.AddCell("Double (2pt)", font.Helvetica, 10).
		SetBorders(layout.CellBorders{Bottom: layout.DoubleBorder(2, layout.ColorBlack)})
	t2 := styleTie.AddRow()
	t2.AddCell("Solid (2pt)", font.Helvetica, 10).
		SetBorders(layout.CellBorders{Top: layout.SolidBorder(2, layout.ColorBlack)})
	doc.Add(styleTie)

	return doc
}
