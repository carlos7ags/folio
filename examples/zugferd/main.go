package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// here one would embed the ttf font files
var regularTTF []byte
var boldTTF []byte

var (
	fontRegular *font.EmbeddedFont
	fontBold    *font.EmbeddedFont
)

func init() {
	r, err := font.ParseFont(regularTTF)
	if err != nil {
		panic(err)
	}
	b, err := font.ParseFont(boldTTF)
	if err != nil {
		panic(err)
	}
	fontRegular = font.NewEmbeddedFont(r)
	fontBold = font.NewEmbeddedFont(b)
}

func main() {
	doc := document.NewDocument(document.PageSizeA4)
	doc.SetMargins(layout.Margins{Top: 40, Right: 40, Bottom: 40, Left: 40})
	doc.Info.Title = "Example Document"
	doc.Info.Author = "folio"

	// --- Title ---
	doc.Add(
		layout.NewStyledParagraph(
			layout.RunEmbedded("Hello from folio!", fontBold, 24).
				WithColor(layout.Hex("E8720C")),
		).SetAlign(layout.AlignCenter).SetLeading(1.2),
	)

	// --- Divider ---
	doc.Add(
		layout.NewLineSeparator().
			SetWidth(2).
			SetColor(layout.Hex("E8720C")).
			SetSpaceBefore(12).
			SetSpaceAfter(18),
	)

	// --- Body text ---
	doc.Add(
		layout.NewParagraphEmbedded(
			"This is a minimal example showing how to create a PDF with embedded fonts, "+
				"styled text, tables, and a line separator using the folio library.",
			fontRegular, 11,
		).SetLeading(1.5),
	)

	// --- Simple table ---
	tbl := layout.NewTable().
		SetColumnUnitWidths([]layout.UnitValue{layout.Pct(40), layout.Pct(30), layout.Pct(30)}).
		SetBorderCollapse(true)

	border := layout.CellBorders{
		Bottom: layout.Border{Width: 1, Color: layout.Hex("dddddd"), Style: layout.BorderSolid},
	}

	// Header row
	hr := tbl.AddHeaderRow()
	for _, h := range []string{"Item", "Qty", "Price"} {
		c := hr.AddCellElement(
			layout.NewStyledParagraph(
				layout.RunEmbedded(h, fontBold, 10).WithColor(layout.Hex("E8720C")),
			),
		)
		c.SetBorders(layout.CellBorders{
			Bottom: layout.Border{Width: 2, Color: layout.Hex("E8720C"), Style: layout.BorderSolid},
		}).SetPaddingSides(layout.Padding{Top: 6, Right: 8, Bottom: 6, Left: 8})
	}

	// Data rows
	rows := [][]string{
		{"Widget A", "10", "€ 5,00"},
		{"Widget B", "3", "€ 12,50"},
		{"Service C", "1", "€ 250,00"},
	}
	for _, row := range rows {
		r := tbl.AddRow()
		for _, text := range row {
			c := r.AddCellEmbedded(text, fontRegular, 9)
			c.SetBorders(border).
				SetPaddingSides(layout.Padding{Top: 6, Right: 8, Bottom: 6, Left: 8})
		}
	}

	wrapper := layout.NewDiv().SetSpaceBefore(14)
	wrapper.Add(tbl)
	doc.Add(wrapper)

	// --- Footer ---
	doc.SetFooter(func(ctx document.PageContext, page *document.Page) {
		page.AddTextEmbedded(
			fmt.Sprintf("Page %d / %d", ctx.PageIndex+1, ctx.TotalPages),
			fontRegular, 8, 40, 20,
		)
	})

	// --- Write PDF ---
	f, err := os.Create("example.pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()

	if _, err := doc.WriteTo(f); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote example.pdf")
}
