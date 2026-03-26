// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Import-page demonstrates loading an existing PDF as a template and
// adding dynamic content on top — the standard workflow for invoices,
// receipts, certificates, and forms.
//
// The included template.pdf is a payment receipt designed in an external
// tool. This example loads it, fills in receipt data, and saves a new PDF.
//
// Features demonstrated:
//   - Loading a real external PDF as a template
//   - Convenience API (reader.ExtractPageImport) with full indirect-ref resolution
//   - Overlay text on imported pages
//   - Reusing the same template across multiple pages
//
// Usage:
//
//	go run ./examples/import-page
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/reader"
)

func main() {
	// --- Load the template PDF ---
	templatePath := findTemplate()
	templateBytes, err := os.ReadFile(templatePath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}

	r, err := reader.Parse(templateBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, "parse:", err)
		os.Exit(1)
	}

	// Extract the first page for importing.
	// ExtractPageImport resolves all indirect references (fonts, images,
	// color spaces) so the result is self-contained and independent of
	// the source PdfReader.
	imp, err := reader.ExtractPageImport(r, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, "extract:", err)
		os.Exit(1)
	}
	fmt.Printf("Template: %.0fx%.0f pt\n", imp.Width, imp.Height)

	// --- Fill in receipts from the template ---
	receipts := []struct{ number, date, from, amount, method, work, period string }{
		{"REC-001", "2026-03-15", "Apex Capital Partners", "$34,948.88", "Wire Transfer", "Q1 consulting services", "Jan — Mar 2026"},
		{"REC-002", "2026-03-16", "Meridian Dynamics", "$12,500.00", "Check", "Software license — annual", "Mar 2026 — Mar 2027"},
		{"REC-003", "2026-03-17", "Northwind Technologies", "$8,750.00", "ACH", "Cloud infrastructure", "March 2026"},
	}

	templateSize := document.PageSize{Width: imp.Width, Height: imp.Height}
	doc := document.NewDocument(templateSize)
	doc.Info.Title = "Payment Receipts"

	for _, rec := range receipts {
		p := doc.AddPage()

		// Import template as background (rendered as a Form XObject).
		p.ImportPage(imp.ContentStream, imp.Resources, imp.Width, imp.Height)

		// Fill in the receipt fields on top of the template.
		// Coordinates determined using a grid overlay (50pt grid).
		p.AddText(rec.date, font.Helvetica, 10, 450, 640)   // next to "Date"
		p.AddText(rec.number, font.Helvetica, 10, 450, 593) // below "Receipt No."
		p.AddText(rec.from, font.Helvetica, 10, 220, 527)   // on "I received from" line
		p.AddText(rec.amount, font.Helvetica, 10, 220, 492) // on "The sum of" line
		p.AddText(rec.method, font.Helvetica, 10, 450, 492) // on "Method of payment" line
		p.AddText(rec.work, font.Helvetica, 10, 265, 450)   // on "For the work done" line
		p.AddText(rec.period, font.Helvetica, 10, 310, 380) // on "Corresponding to the period of" line
	}

	if err := doc.Save("receipts.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Verify.
	data, _ := os.ReadFile("receipts.pdf")
	result, _ := reader.Parse(data)
	fmt.Printf("Created receipts.pdf — %d pages, %d bytes\n",
		result.PageCount(), len(data))
}

func findTemplate() string {
	candidates := []string{
		"examples/import-page/template.pdf",
		"template.pdf",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	fmt.Fprintln(os.Stderr, "template.pdf not found — run from the repository root")
	os.Exit(1)
	return ""
}
