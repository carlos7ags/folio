// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout_test

import (
	"fmt"

	"github.com/carlos7ags/folio/barcode"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

func ExampleParagraph() {
	p := layout.NewParagraph("Hello, World!", font.Helvetica, 12).
		SetAlign(layout.AlignCenter).
		SetSpaceBefore(10)

	plan := p.PlanLayout(layout.LayoutArea{Width: 468, Height: 500})
	fmt.Println("Status:", plan.Status == layout.LayoutFull)
	fmt.Println("Blocks:", len(plan.Blocks))

	// Output:
	// Status: true
	// Blocks: 1
}

func ExampleTable_autoColumns() {
	tbl := layout.NewTable().SetAutoColumnWidths()

	h := tbl.AddHeaderRow()
	h.AddCell("Product", font.HelveticaBold, 10)
	h.AddCell("Description", font.HelveticaBold, 10)
	h.AddCell("Price", font.HelveticaBold, 10)

	r := tbl.AddRow()
	r.AddCell("Widget", font.Helvetica, 10)
	r.AddCell("A wonderful widget that does amazing things", font.Helvetica, 10)
	r.AddCell("$9.99", font.Helvetica, 10)

	plan := tbl.PlanLayout(layout.LayoutArea{Width: 468, Height: 500})
	fmt.Println("Table fits:", plan.Status == layout.LayoutFull)

	// Output:
	// Table fits: true
}

func ExampleDiv() {
	box := layout.NewDiv().
		SetPadding(12).
		SetBorder(layout.SolidBorder(1, layout.ColorBlack)).
		SetBackground(layout.ColorLightGray).
		Add(layout.NewHeading("Notice", layout.H2)).
		Add(layout.NewParagraph("Important information goes here.", font.Helvetica, 12))

	plan := box.PlanLayout(layout.LayoutArea{Width: 468, Height: 500})
	fmt.Println("Div fits:", plan.Status == layout.LayoutFull)

	// Output:
	// Div fits: true
}

func ExampleNewBarcodeElement() {
	bc, _ := barcode.QR("https://example.com")
	elem := layout.NewBarcodeElement(bc, 100).SetAlign(layout.AlignCenter)

	plan := elem.PlanLayout(layout.LayoutArea{Width: 468, Height: 500})
	fmt.Println("QR fits:", plan.Status == layout.LayoutFull)

	// Output:
	// QR fits: true
}
