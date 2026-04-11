// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Optimize compares the default writer with the optimized writer
// (cross-reference streams per ISO 32000-1 §7.5.8 plus object streams
// per ISO 32000-1 §7.5.7) across several document shapes and reports
// the byte-size delta for each.
//
// The compression ratio depends heavily on what the document contains:
// content streams are already Flate-compressed and ineligible for
// object stream packing, so text-heavy documents save less than
// metadata-heavy documents. The fixture set in this example is chosen
// to surface that difference, so callers can decide whether the
// optimizer is worth turning on for their workload.
//
// Usage:
//
//	go run ./examples/optimize
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// fixture is one row of the comparison table.
type fixture struct {
	name  string
	build func() *document.Document
}

func textHeavy() *document.Document {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Text-heavy fixture"
	for i := 1; i <= 25; i++ {
		doc.Add(layout.NewHeading(fmt.Sprintf("Section %d", i), layout.H1))
		doc.Add(layout.NewParagraph(
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do "+
				"eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut "+
				"enim ad minim veniam, quis nostrud exercitation ullamco laboris.",
			font.Helvetica, 11,
		))
	}
	return doc
}

func manyPages() *document.Document {
	// Page-tree-heavy: many empty pages produce many small dictionaries
	// (one page object plus its resources per page) and almost no
	// content stream bytes. This is the shape where the optimizer wins
	// the most because nearly every object is eligible for packing.
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Many empty pages fixture"
	for range 50 {
		doc.AddPage()
	}
	return doc
}

func tableHeavy() *document.Document {
	// One large table with many rows. Tables register multiple resource
	// dictionaries and per-cell styling, so they exercise the resource
	// path that the optimizer compresses well.
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Table-heavy fixture"
	tbl := layout.NewTable().SetAutoColumnWidths()
	header := tbl.AddRow()
	header.AddCell("SKU", font.Helvetica, 10)
	header.AddCell("Description", font.Helvetica, 10)
	header.AddCell("Quantity", font.Helvetica, 10)
	header.AddCell("Unit price", font.Helvetica, 10)
	header.AddCell("Line total", font.Helvetica, 10)
	for i := 1; i <= 60; i++ {
		row := tbl.AddRow()
		row.AddCell(fmt.Sprintf("SKU-%04d", i), font.Helvetica, 10)
		row.AddCell(fmt.Sprintf("Item description %d", i), font.Helvetica, 10)
		row.AddCell(fmt.Sprintf("%d", i), font.Helvetica, 10)
		row.AddCell(fmt.Sprintf("$%d.99", i*5), font.Helvetica, 10)
		row.AddCell(fmt.Sprintf("$%d.45", i*i*5), font.Helvetica, 10)
	}
	doc.Add(tbl)
	return doc
}

func writeBoth(f fixture) (defaultBytes, optimizedBytes []byte, err error) {
	defaultBytes, err = f.build().ToBytes()
	if err != nil {
		return nil, nil, fmt.Errorf("%s default: %w", f.name, err)
	}
	optimizedBytes, err = f.build().ToBytesWithOptions(document.WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("%s optimized: %w", f.name, err)
	}
	return defaultBytes, optimizedBytes, nil
}

func main() {
	fixtures := []fixture{
		{name: "text-heavy", build: textHeavy},
		{name: "many empty pages", build: manyPages},
		{name: "table-heavy", build: tableHeavy},
	}

	fmt.Printf("%-20s %12s %12s %10s\n", "fixture", "default", "optimized", "saved")
	fmt.Println("-------------------- ------------ ------------ ----------")

	for _, f := range fixtures {
		def, opt, err := writeBoth(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		saved := len(def) - len(opt)
		pct := 100.0 * float64(saved) / float64(len(def))
		fmt.Printf("%-20s %10d B %10d B %8.1f %%\n",
			f.name, len(def), len(opt), pct)
	}

	// Write the text-heavy fixture to disk so the user has a concrete
	// pair of files to inspect with qpdf or any PDF viewer.
	def, opt, err := writeBoth(fixtures[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("optimize-default.pdf", def, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write default file: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("optimize-compressed.pdf", opt, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write optimized file: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
	fmt.Println("wrote optimize-default.pdf and optimize-compressed.pdf (text-heavy fixture)")
}
