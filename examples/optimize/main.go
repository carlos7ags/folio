// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Optimize writes the same document twice — once with the historical
// default writer and once with cross-reference streams (ISO 32000-1
// §7.5.8) and object streams (ISO 32000-1 §7.5.7) enabled — and
// reports the byte-size difference.
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

func buildDocument() *document.Document {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Optimization demo"
	doc.Info.Author = "Folio"

	for i := 1; i <= 25; i++ {
		doc.Add(layout.NewHeading(fmt.Sprintf("Section %d", i), layout.H1))
		doc.Add(layout.NewParagraph(
			"This document exists to demonstrate the byte-size impact of the "+
				"cross-reference stream and object stream output modes. Each section "+
				"adds a few indirect objects (page tree node, content stream, "+
				"resources dictionary), so the savings grow with the number of "+
				"sections in the document.",
			font.Helvetica, 11,
		))
	}
	return doc
}

func main() {
	tradBytes, err := buildDocument().ToBytes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "default write failed: %v\n", err)
		os.Exit(1)
	}

	optBytes, err := buildDocument().ToBytesWithOptions(document.WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "optimized write failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile("optimize-default.pdf", tradBytes, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write default file: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile("optimize-compressed.pdf", optBytes, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "write optimized file: %v\n", err)
		os.Exit(1)
	}

	delta := len(tradBytes) - len(optBytes)
	pct := 100.0 * float64(delta) / float64(len(tradBytes))
	fmt.Printf("default     : %d bytes (optimize-default.pdf)\n", len(tradBytes))
	fmt.Printf("optimized   : %d bytes (optimize-compressed.pdf)\n", len(optBytes))
	fmt.Printf("saved       : %d bytes (%.1f%%)\n", delta, pct)
}
