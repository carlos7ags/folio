// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Merge demonstrates reading, merging, and inspecting PDF documents:
//
//   - Creating PDFs with the document and layout APIs
//   - Parsing PDFs from bytes with reader.Parse
//   - Merging multiple PDFs into one with reader.Merge
//   - Setting metadata on the merged document
//   - Extracting text from individual pages
//   - Inspecting page count and dimensions
//   - Saving the merged result
//
// Usage:
//
//	go run ./examples/merge
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

func main() {
	// --- Create source PDFs ---
	fmt.Println("Creating source PDFs...")

	pdf1 := createReport("Q3 2026 Revenue Report", []string{
		"Total revenue reached $25.1M, up 18% year-over-year.",
		"Advisory services contributed $12.8M (51% of total).",
		"New client acquisitions increased by 23 accounts.",
	})

	pdf2 := createReport("Q4 2026 Revenue Report", []string{
		"Total revenue reached $28.3M, up 22% year-over-year.",
		"Operating margin improved to 30%, driven by cost optimization.",
		"Asia-Pacific region grew 31.7%, the fastest across all regions.",
	})

	pdf3 := createCoverPage("Annual Summary 2026", "Apex Capital Partners")

	// --- Parse PDFs ---
	r1, err := reader.Parse(pdf1)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse pdf1:", err)
		os.Exit(1)
	}
	r2, err := reader.Parse(pdf2)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse pdf2:", err)
		os.Exit(1)
	}
	r3, err := reader.Parse(pdf3)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse pdf3:", err)
		os.Exit(1)
	}

	// --- Inspect source PDFs ---
	fmt.Printf("  PDF 1: %d page(s), %d bytes\n", r1.PageCount(), len(pdf1))
	fmt.Printf("  PDF 2: %d page(s), %d bytes\n", r2.PageCount(), len(pdf2))
	fmt.Printf("  PDF 3: %d page(s), %d bytes\n", r3.PageCount(), len(pdf3))

	// --- Merge: cover + Q3 report + Q4 report ---
	fmt.Println("\nMerging...")
	merged, err := reader.Merge(r3, r1, r2)
	if err != nil {
		fmt.Fprintln(os.Stderr, "merge:", err)
		os.Exit(1)
	}
	merged.SetInfo("Annual Summary 2026", "Apex Capital Partners")

	// --- Save merged PDF ---
	if err := merged.SaveTo("merged.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, "save:", err)
		os.Exit(1)
	}

	// --- Read back and inspect ---
	mergedBytes, _ := os.ReadFile("merged.pdf")
	result, err := reader.Parse(mergedBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse merged:", err)
		os.Exit(1)
	}

	title, author, _, _, _ := result.Info()
	fmt.Printf("\nMerged PDF: %d pages, %d bytes\n", result.PageCount(), len(mergedBytes))
	fmt.Printf("  Title:  %s\n", title)
	fmt.Printf("  Author: %s\n", author)

	// --- Extract text from each page ---
	fmt.Println("\nExtracted text:")
	for i := range result.PageCount() {
		page, err := result.Page(i)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  page %d: %v\n", i+1, err)
			continue
		}
		text, err := page.ExtractText()
		if err != nil {
			fmt.Fprintf(os.Stderr, "  page %d text: %v\n", i+1, err)
			continue
		}
		// Show first 80 chars of each page.
		preview := text
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}
		box := page.VisibleBox()
		fmt.Printf("  Page %d (%.0fx%.0f pt): %s\n", i+1, box.Width(), box.Height(), preview)
	}

	fmt.Println("\nCreated merged.pdf")
}

// createReport generates a simple one-page report PDF.
func createReport(title string, bullets []string) []byte {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = title
	doc.Info.Author = "Apex Capital Partners"

	doc.Add(layout.NewParagraph(title, font.HelveticaBold, 18))

	doc.Add(layout.NewLineSeparator().
		SetWidth(1).
		SetColor(layout.RGB(0.7, 0.7, 0.7)).
		SetSpaceBefore(6).
		SetSpaceAfter(10))

	for _, b := range bullets {
		p := layout.NewStyledParagraph(
			layout.Run("• ", font.Helvetica, 11),
			layout.Run(b, font.Helvetica, 11),
		)
		p.SetLeading(1.4)
		p.SetSpaceAfter(4)
		doc.Add(p)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return buf.Bytes()
}

// createCoverPage generates a simple cover page PDF.
func createCoverPage(title, subtitle string) []byte {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = title

	p := doc.AddPage()
	p.AddText(title, font.HelveticaBold, 28, 72, 500)
	p.AddText(subtitle, font.Helvetica, 16, 72, 460)
	p.AddText("Confidential", font.Helvetica, 10, 72, 420)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return buf.Bytes()
}
