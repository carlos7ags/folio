// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Links demonstrates all link-related features in Folio:
//
//   - External hyperlinks (<a href="https://...">)
//   - Inline links within paragraphs (mixed text and links)
//   - Multi-word links with continuous underline
//   - Multi-line links that wrap across lines
//   - Styled links (custom colors, bold, no-underline)
//   - Bookmarks / outlines for PDF viewer sidebar navigation
//   - Internal links that jump between pages
//   - Links generated via the HTML converter
//   - Links generated via the layout API directly
//
// Usage:
//
//	go run ./examples/links
package main

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

func main() {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Folio Links Showcase"
	doc.Info.Author = "Folio"

	// ---------------------------------------------------------------
	// Page 1: HTML-generated links
	// ---------------------------------------------------------------
	htmlContent := `<html><head><style>
body { font-family: Helvetica; font-size: 10px; margin: 0; }
h1 { font-size: 18px; color: #1a1a2e; margin-bottom: 3px; }
h2 { font-size: 11px; color: #2c3e50; margin-top: 8px; margin-bottom: 2px; }
p { margin-bottom: 3px; line-height: 1.3; }
a { color: #2563eb; }
.subtle a { color: #6b7280; text-decoration: none; }
hr { margin: 6px 0; }
</style></head><body>

<h1>Folio Links Showcase</h1>
<p>This PDF demonstrates link features supported by the Folio library.</p>

<hr/>

<h2>External Hyperlinks</h2>
<p><a href="https://github.com/carlos7ags/folio">Folio on GitHub</a></p>
<p><a href="https://pkg.go.dev/github.com/carlos7ags/folio">Go Package Documentation (pkg.go.dev)</a></p>
<p><a href="https://www.example.com">Example.com</a></p>

<h2>Inline Links in Paragraphs</h2>
<p>Visit the <a href="https://golang.org">Go programming language</a> website for tutorials and documentation.</p>
<p>Read the latest news on the <a href="https://go.dev/blog">Go Blog</a>.</p>
<p>Browse packages at <a href="https://pkg.go.dev">pkg.go.dev</a>.</p>

<h2>Multi-Word Links</h2>
<p><a href="https://example.com/multi">Click this entire multi-word phrase</a> — the underline is continuous across all words.</p>

<h2>Multi-Line Wrapping Links</h2>
<p style="width: 220px"><a href="https://example.com/long">This long hyperlink wraps across multiple lines with separate annotations per line.</a></p>

<h2>Styled Links</h2>
<p><a href="https://example.com/red" style="color: #dc2626;">Red link</a> —
   <a href="https://example.com/green" style="color: #059669;">Green link</a> —
   <a href="https://example.com/purple" style="color: #7c3aed;">Purple link</a></p>
<p><b><a href="https://example.com/bold" style="color: #1e40af;">Bold blue link</a></b></p>
<p class="subtle">Subtle style: <a href="https://example.com/subtle">no underline, muted color</a></p>

<h2><a href="https://example.com/heading-link">Linked Heading (clickable)</a></h2>

<h2>Links in Lists</h2>
<ul>
<li><a href="https://example.com/li1">Linked item</a></li>
<li>Text with <a href="https://example.com/li2">inline link</a></li>
</ul>

<h2>Edge Cases</h2>
<p><a href="https://example.com/start">Link at start</a> then text, and text then <a href="https://example.com/end">link at end</a></p>
<p><a href="https://example.com/adj-a">Adjacent</a><a href="https://example.com/adj-b">Links</a> with no gap between them.</p>


</body></html>`

	result, err := html.ConvertFull(htmlContent, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "html convert: %v\n", err)
		os.Exit(1)
	}
	for _, e := range result.Elements {
		doc.Add(e)
	}

	// ---------------------------------------------------------------
	// Page 2: Layout API links + internal navigation
	// ---------------------------------------------------------------
	doc.Add(layout.NewAreaBreak())

	// Heading
	h := layout.NewHeading("Layout API Links", layout.H1)
	doc.Add(h)
	doc.Add(para("These links are built with the layout package — no HTML.", 9))

	// External links
	doc.Add(spacer(4))
	doc.Add(sectionHeading("External Links"))
	doc.Add(layout.NewLink(
		"Folio on GitHub",
		"https://github.com/carlos7ags/folio",
		font.Helvetica, 11,
	).SetColor(layout.RGB(0.15, 0.39, 0.92)).SetUnderline())
	doc.Add(spacer(2))
	doc.Add(layout.NewLink(
		"Go Standard Library (pkg.go.dev)",
		"https://pkg.go.dev/std",
		font.Helvetica, 11,
	).SetColor(layout.RGB(0.15, 0.39, 0.92)).SetUnderline())
	doc.Add(spacer(2))
	doc.Add(layout.NewLink(
		"Go Blog",
		"https://go.dev/blog",
		font.TimesRoman, 11,
	).SetColor(layout.RGB(0.02, 0.59, 0.40)).SetUnderline())

	// Rich text with inline links
	doc.Add(spacer(4))
	doc.Add(sectionHeading("Inline Links in Rich Text"))
	p := layout.NewStyledParagraph(
		layout.Run("Read the ", font.Helvetica, 11),
		layout.Run("installation guide", font.HelveticaBold, 11).
			WithColor(layout.RGB(0.15, 0.25, 0.70)).
			WithDecoration(layout.DecorationUnderline).
			WithLinkURI("https://go.dev/doc/install"),
		layout.Run(" to set up Go, then follow the ", font.Helvetica, 11),
		layout.Run("getting started tutorial.", font.HelveticaBold, 11).
			WithColor(layout.RGB(0.15, 0.25, 0.70)).
			WithDecoration(layout.DecorationUnderline).
			WithLinkURI("https://go.dev/doc/tutorial/getting-started"),
	)
	p.SetLeading(1.4)
	doc.Add(p)

	// Internal cross-page navigation
	doc.Add(spacer(4))
	doc.Add(sectionHeading("Internal Navigation"))
	doc.Add(para("Click below to jump back to page 1:", 9))
	doc.Add(layout.NewInternalLink(
		"Go to page 1 (HTML Links)",
		"page1",
		font.HelveticaBold, 11,
	).SetColor(layout.RGB(0.58, 0.17, 0.83)).SetUnderline())

	// Linked heading via layout API
	doc.Add(spacer(4))
	doc.Add(sectionHeading("Linked Heading"))
	h2 := layout.NewHeadingWithFont("Resources", layout.H3, font.HelveticaBold, 13)
	h2.SetRuns([]layout.TextRun{
		layout.Run("Resources", font.HelveticaBold, 13).
			WithColor(layout.RGB(0.15, 0.39, 0.92)).
			WithDecoration(layout.DecorationUnderline).
			WithLinkURI("https://example.com/resources"),
	})
	doc.Add(h2)

	// Linked list items via layout API
	doc.Add(spacer(2))
	doc.Add(sectionHeading("Linked List Items"))
	list := layout.NewList(font.Helvetica, 10)
	list.AddItemRuns([]layout.TextRun{
		layout.Run("Folio repository", font.Helvetica, 10).
			WithColor(layout.RGB(0.15, 0.39, 0.92)).
			WithDecoration(layout.DecorationUnderline).
			WithLinkURI("https://github.com/carlos7ags/folio"),
	})
	list.AddItemRuns([]layout.TextRun{
		layout.Run("See the ", font.Helvetica, 10),
		layout.Run("Go docs", font.HelveticaBold, 10).
			WithColor(layout.RGB(0.15, 0.39, 0.92)).
			WithDecoration(layout.DecorationUnderline).
			WithLinkURI("https://pkg.go.dev"),
		layout.Run(" for details.", font.Helvetica, 10),
	})
	list.AddItem("Plain item (no link)")
	doc.Add(list)

	// Multiple links on one line
	doc.Add(spacer(4))
	doc.Add(sectionHeading("Multiple Links Per Line"))
	doc.Add(layout.NewStyledParagraph(
		layout.Run("Compare ", font.Helvetica, 10),
		layout.Run("GitHub", font.HelveticaBold, 10).
			WithColor(layout.RGB(0.15, 0.39, 0.92)).
			WithDecoration(layout.DecorationUnderline).
			WithLinkURI("https://github.com"),
		layout.Run(" and ", font.Helvetica, 10),
		layout.Run("GitLab", font.HelveticaBold, 10).
			WithColor(layout.RGB(0.15, 0.39, 0.92)).
			WithDecoration(layout.DecorationUnderline).
			WithLinkURI("https://gitlab.com"),
		layout.Run(" for hosting.", font.Helvetica, 10),
	))

	// Register named destinations and bookmarks.
	doc.AddNamedDest(document.NamedDest{
		Name: "page1", PageIndex: 0, FitType: "Fit",
	})
	doc.AddNamedDest(document.NamedDest{
		Name: "page2", PageIndex: 1, FitType: "Fit",
	})

	sec1 := doc.AddOutline("HTML Links", document.Destination{
		PageIndex: 0, Type: document.DestFit,
	})
	sec1.AddChild("External Hyperlinks", document.Destination{
		PageIndex: 0, Type: document.DestFit,
	})
	sec1.AddChild("Inline Links", document.Destination{
		PageIndex: 0, Type: document.DestFit,
	})
	sec1.AddChild("Multi-Line Links", document.Destination{
		PageIndex: 0, Type: document.DestFit,
	})

	doc.AddOutline("Layout API Links", document.Destination{
		PageIndex: 1, Type: document.DestFit,
	})

	if err := doc.Save("links.pdf"); err != nil {
		fmt.Fprintf(os.Stderr, "save: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Created links.pdf")
}

// sectionHeading creates a small bold heading.
func sectionHeading(text string) *layout.Paragraph {
	p := layout.NewStyledParagraph(
		layout.Run(text, font.HelveticaBold, 12).
			WithColor(layout.RGB(0.17, 0.24, 0.31)),
	)
	p.SetSpaceAfter(4)
	return p
}

// para creates a plain paragraph with the given font size.
func para(text string, size float64) *layout.Paragraph {
	p := layout.NewParagraph(text, font.Helvetica, size)
	p.SetLeading(1.4)
	return p
}

// spacer creates vertical space.
func spacer(pts float64) *layout.Paragraph {
	p := layout.NewParagraph(" ", font.Helvetica, 1)
	p.SetSpaceBefore(pts)
	return p
}
