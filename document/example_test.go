// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

func ExampleDocument_simple() {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Hello World"
	doc.Info.Author = "Folio"

	doc.Add(layout.NewHeading("Hello, Folio!", layout.H1))
	doc.Add(layout.NewParagraph(
		"This is a paragraph of text created with the Folio PDF library.",
		font.Helvetica, 12,
	))

	if err := doc.Save("hello.pdf"); err != nil {
		fmt.Println("error:", err)
		return
	}
	os.Remove("hello.pdf")
	fmt.Println("Created hello.pdf")
	// Output:
	// Created hello.pdf
}

func ExampleDocument_table() {
	doc := document.NewDocument(document.PageSizeA4)
	doc.Info.Title = "Invoice"

	doc.Add(layout.NewHeading("Invoice #1234", layout.H1))

	tbl := layout.NewTable().SetAutoColumnWidths()
	h := tbl.AddHeaderRow()
	h.AddCell("Item", font.HelveticaBold, 10)
	h.AddCell("Qty", font.HelveticaBold, 10)
	h.AddCell("Price", font.HelveticaBold, 10)

	r1 := tbl.AddRow()
	r1.AddCell("Widget", font.Helvetica, 10)
	r1.AddCell("5", font.Helvetica, 10)
	r1.AddCell("$250.00", font.Helvetica, 10)

	doc.Add(tbl)
	doc.Save("invoice.pdf")
	os.Remove("invoice.pdf")
	fmt.Println("Created invoice")
	// Output:
	// Created invoice
}

func ExampleDocument_taggedPdfA() {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Accessible Report"
	doc.Info.Author = "Folio"

	doc.SetTagged(true)
	doc.SetPdfA(document.PdfAConfig{Level: document.PdfA2B})

	doc.AddPage() // blank page — PDF/A requires embedded fonts for text

	doc.Save("accessible.pdf")
	os.Remove("accessible.pdf")
	fmt.Println("Created accessible PDF/A")
	// Output:
	// Created accessible PDF/A
}
