// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Optimize compares the default writer with the optimized writer
// (cross-reference streams per ISO 32000-1 §7.5.8 plus object streams
// per ISO 32000-1 §7.5.7, orphan sweep over §7.5.4 reachability, and
// Flate recompression of eligible payloads per §7.4.4) across several
// document shapes and reports the byte-size delta for each.
//
// The compression ratio depends heavily on what the document contains:
// content streams produced by Folio's writer are already at
// zlib.BestCompression and cannot benefit from re-deflate, so layout-
// built documents see most of their gains from object-stream packing
// and the orphan sweep. Imported documents (built by parsing a source
// PDF and copying pages into a new Document) carry their content
// streams in raw plaintext form — that is where recompression wins big.
//
// Usage:
//
//	go run ./examples/optimize
package main

import (
	"bytes"
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
)

// fixture is one row of the comparison table.
type fixture struct {
	build func() *document.Document
	name  string
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

// importedTextHeavy builds the text-heavy document, writes it, parses
// the result, and imports every page into a fresh Document. The output
// document carries content streams in raw form — exactly the shape
// where Flate recompression on write produces a large win.
func importedTextHeavy() *document.Document {
	src := textHeavy()
	var buf bytes.Buffer
	if _, err := src.WriteTo(&buf); err != nil {
		panic(fmt.Errorf("source write: %w", err))
	}
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		panic(fmt.Errorf("source parse: %w", err))
	}
	out := document.NewDocument(document.PageSizeLetter)
	out.Info.Title = "Imported text-heavy fixture"
	for i := 0; i < r.PageCount(); i++ {
		srcPage, _ := r.Page(i)
		cs, _ := srcPage.ContentStream()
		res, _ := srcPage.Resources()
		p := out.AddPage()
		p.ImportPage(cs, res, srcPage.Width, srcPage.Height)
	}
	return out
}

// writeAll serializes the fixture in each comparison mode:
//
//   - default: traditional xref table and trailer (§7.5.4 / §7.5.5).
//   - xref+obj: cross-reference stream (§7.5.8) plus object streams (§7.5.7).
//   - +sweep: also drops indirect objects unreachable from /Root, /Info,
//     and /Encrypt before serialization.
//   - +recompress: also re-Flate-compresses eligible stream payloads
//     (§7.4.4) at zlib.BestCompression.
//   - +full: every lossless toggle on — adds content-stream operator
//     cleanup (§7.8) and structural deduplication of byte-identical
//     indirect objects.
func writeAll(f fixture) (defaultBytes, packedBytes, sweptBytes, recompressBytes, fullBytes []byte, err error) {
	defaultBytes, err = f.build().ToBytes()
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("%s default: %w", f.name, err)
	}
	packedBytes, err = f.build().ToBytesWithOptions(document.WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	})
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("%s xref+obj: %w", f.name, err)
	}
	sweptBytes, err = f.build().ToBytesWithOptions(document.WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
		OrphanSweep:      true,
	})
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("%s +sweep: %w", f.name, err)
	}
	recompressBytes, err = f.build().ToBytesWithOptions(document.WriteOptions{
		UseXRefStream:     true,
		UseObjectStreams:  true,
		OrphanSweep:       true,
		RecompressStreams: true,
	})
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("%s +recompress: %w", f.name, err)
	}
	fullBytes, err = f.build().ToBytesWithOptions(document.WriteOptions{
		UseXRefStream:       true,
		UseObjectStreams:    true,
		OrphanSweep:         true,
		CleanContentStreams: true,
		DeduplicateObjects:  true,
		RecompressStreams:   true,
	})
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("%s +full: %w", f.name, err)
	}
	return defaultBytes, packedBytes, sweptBytes, recompressBytes, fullBytes, nil
}

func main() {
	fixtures := []fixture{
		{name: "text-heavy", build: textHeavy},
		{name: "many empty pages", build: manyPages},
		{name: "table-heavy", build: tableHeavy},
		{name: "imported text-heavy", build: importedTextHeavy},
	}

	fmt.Printf("%-22s %12s %12s %12s %12s %12s %10s\n",
		"fixture", "default", "xref+obj", "+sweep", "+recompress", "+full", "saved")
	fmt.Println("---------------------- ------------ ------------ ------------ ------------ ------------ ----------")

	for _, f := range fixtures {
		def, packed, swept, recompress, full, err := writeAll(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		saved := len(def) - len(full)
		pct := 100.0 * float64(saved) / float64(len(def))
		fmt.Printf("%-22s %10d B %10d B %10d B %10d B %10d B %8.1f %%\n",
			f.name, len(def), len(packed), len(swept), len(recompress), len(full), pct)
	}

	// Write the imported fixture to disk so the user has concrete files
	// to inspect with qpdf or any PDF viewer. The imported case is
	// where the optimizer's win is most visible.
	def, _, _, _, full, err := writeAll(fixtures[len(fixtures)-1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("optimize-default.pdf", def, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write default file: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("optimize-compressed.pdf", full, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write optimized file: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
	fmt.Println("wrote optimize-default.pdf and optimize-compressed.pdf (imported text-heavy fixture)")
}
