// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Redact demonstrates permanently removing sensitive content from PDFs.
//
// Unlike drawing a black box (which leaves text selectable and recoverable),
// true redaction removes text operators from the PDF content stream. The
// redacted content cannot be extracted, searched, or recovered.
//
// Features demonstrated:
//   - Text search redaction (find and remove all occurrences)
//   - Regex pattern redaction (e.g. SSN, phone numbers)
//   - Region-based redaction (specify coordinates)
//   - Custom overlay text on redaction boxes
//   - Metadata stripping
//   - Character-level precision (adjacent text preserved)
//
// Usage:
//
//	go run ./examples/redact
package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
)

func main() {
	// --- Step 1: Create a PDF with sensitive content ---
	fmt.Println("Creating PDF with sensitive content...")
	pdf := createSensitivePDF()

	r, err := reader.Parse(pdf)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Show original text.
	fmt.Println("\nOriginal text:")
	printPageText(r, 0)

	// --- Step 2: Redact by text search ---
	fmt.Println("\n--- Text Search Redaction ---")
	m, err := reader.RedactText(r, []string{
		"Jane Doe",
		"987-65-4321",
		"4111-1111-1111-1111",
	}, &reader.RedactOptions{
		OverlayText: "REDACTED",
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "redact text:", err)
		os.Exit(1)
	}
	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	r2, _ := reader.Parse(buf.Bytes())
	fmt.Println("After text redaction:")
	printPageText(r2, 0)

	// --- Step 3: Redact by regex pattern ---
	fmt.Println("\n--- Regex Pattern Redaction ---")
	r3, _ := reader.Parse(pdf)
	emailPattern := regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
	m2, err := reader.RedactPattern(r3, emailPattern, &reader.RedactOptions{
		FillColor:   [3]float64{0.5, 0, 0},
		OverlayText: "[EMAIL]",
		OverlayColor: [3]float64{1, 1, 1},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "redact pattern:", err)
		os.Exit(1)
	}
	var buf2 bytes.Buffer
	if _, err := m2.WriteTo(&buf2); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	r4, _ := reader.Parse(buf2.Bytes())
	fmt.Println("After email redaction:")
	printPageText(r4, 0)

	// --- Step 4: Full redaction with metadata strip ---
	fmt.Println("\n--- Full Redaction (text + regex + metadata strip) ---")
	r5, _ := reader.Parse(pdf)
	ssnPattern := regexp.MustCompile(`\d{3}-\d{2}-\d{4}`)
	m3, err := reader.RedactPattern(r5, ssnPattern, &reader.RedactOptions{
		FillColor:     [3]float64{0, 0, 0},
		OverlayText:   "[SSN]",
		StripMetadata: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := m3.SaveTo("redacted.pdf"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// Verify the output.
	redactedBytes, _ := os.ReadFile("redacted.pdf")
	rFinal, _ := reader.Parse(redactedBytes)
	title, author, _, _, _ := rFinal.Info()
	fmt.Printf("Output: %d bytes, %d pages\n", len(redactedBytes), rFinal.PageCount())
	fmt.Printf("Metadata stripped — Title: %q, Author: %q\n", title, author)
	fmt.Println("Text:")
	printPageText(rFinal, 0)

	// Verify SSN is truly gone from raw bytes.
	if strings.Contains(string(redactedBytes), "987-65-4321") {
		fmt.Println("\nWARNING: SSN found in raw PDF bytes!")
	} else {
		fmt.Println("\nVerified: SSN not present in raw PDF bytes.")
	}

	fmt.Println("\nCreated redacted.pdf")
}

func createSensitivePDF() []byte {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Employee Record"
	doc.Info.Author = "HR Department"

	doc.Add(layout.NewParagraph("Employee Record", font.HelveticaBold, 18))
	doc.Add(layout.NewLineSeparator().SetWidth(1).SetSpaceBefore(4).SetSpaceAfter(8))

	lines := []string{
		"Name: Jane Doe",
		"SSN: 987-65-4321",
		"Email: jane.doe@example.com",
		"Phone: 555-123-4567",
		"Credit Card: 4111-1111-1111-1111",
		"Department: Engineering",
		"Salary: $145,000",
	}
	for _, line := range lines {
		doc.Add(layout.NewParagraph(line, font.Helvetica, 11).SetLeading(1.5))
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return buf.Bytes()
}

func printPageText(r *reader.PdfReader, pageIdx int) {
	page, _ := r.Page(pageIdx)
	text, _ := page.ExtractText()
	for _, line := range strings.Split(text, "\n") {
		if line != "" {
			fmt.Printf("  %s\n", line)
		}
	}
}
