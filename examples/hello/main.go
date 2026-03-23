// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Hello creates a one-page PDF with a heading and a paragraph.
//
// Usage:
//
//	go run ./examples/hello
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

func main() {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Hello World"
	doc.Info.Author = "Folio"

	doc.Add(layout.NewHeading("Hello, Folio!", layout.H1))
	doc.Add(layout.NewParagraph(
		"This is a simple PDF created with the Folio library. "+
			"Folio is a pure-Go library for creating, reading, and signing PDF documents.",
		font.Helvetica, 12,
	))

	if err := doc.Save("hello.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created hello.pdf")
}
