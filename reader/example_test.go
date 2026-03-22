// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader_test

import (
	"bytes"
	"fmt"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/reader"
)

func ExampleParse() {
	// Generate a PDF in memory.
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Example"
	p := doc.AddPage()
	p.AddText("Hello World", font.Helvetica, 12, 72, 700)
	var buf bytes.Buffer
	_, _ = doc.WriteTo(&buf)

	// Parse it back.
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Println("Version:", r.Version())
	fmt.Println("Pages:", r.PageCount())
	title, _, _, _, _ := r.Info()
	fmt.Println("Title:", title)

	// Output:
	// Version: 1.7
	// Pages: 1
	// Title: Example
}

func ExampleMerge() {
	// Create two PDFs.
	makePDF := func(title string) []byte {
		doc := document.NewDocument(document.PageSizeLetter)
		doc.Info.Title = title
		doc.AddPage()
		var buf bytes.Buffer
		_, _ = doc.WriteTo(&buf)
		return buf.Bytes()
	}

	r1, _ := reader.Parse(makePDF("Doc A"))
	r2, _ := reader.Parse(makePDF("Doc B"))

	m, _ := reader.Merge(r1, r2)
	m.SetInfo("Combined", "Folio")

	var out bytes.Buffer
	_, _ = m.WriteTo(&out)

	result, _ := reader.Parse(out.Bytes())
	fmt.Println("Merged pages:", result.PageCount())

	// Output:
	// Merged pages: 2
}
