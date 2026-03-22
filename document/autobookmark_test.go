// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

func TestAutoBookmarks(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetAutoBookmarks(true)

	doc.Add(layout.NewHeading("Chapter 1", layout.H1))
	doc.Add(layout.NewParagraph("Content.", font.Helvetica, 12))
	doc.Add(layout.NewHeading("Section 1.1", layout.H2))
	doc.Add(layout.NewParagraph("More content.", font.Helvetica, 12))
	doc.Add(layout.NewHeading("Chapter 2", layout.H1))
	doc.Add(layout.NewParagraph("Content.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Outlines") {
		t.Error("expected /Outlines in catalog")
	}
	if !strings.Contains(pdf, "Chapter 1") {
		t.Error("expected 'Chapter 1' bookmark")
	}
	if !strings.Contains(pdf, "Section 1.1") {
		t.Error("expected 'Section 1.1' bookmark")
	}
	if !strings.Contains(pdf, "Chapter 2") {
		t.Error("expected 'Chapter 2' bookmark")
	}
}

func TestAutoBookmarksNesting(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetAutoBookmarks(true)

	doc.Add(layout.NewHeading("H1", layout.H1))
	doc.Add(layout.NewHeading("H2 under H1", layout.H2))
	doc.Add(layout.NewHeading("H3 under H2", layout.H3))
	doc.Add(layout.NewHeading("Another H2", layout.H2))
	doc.Add(layout.NewHeading("Another H1", layout.H1))

	var buf bytes.Buffer
	_, _ = doc.WriteTo(&buf)

	pdf := buf.String()

	// All headings should appear as bookmarks.
	for _, text := range []string{"H1", "H2 under H1", "H3 under H2", "Another H2", "Another H1"} {
		if !strings.Contains(pdf, text) {
			t.Errorf("missing bookmark: %s", text)
		}
	}
}

func TestAutoBookmarksDisabledByDefault(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	// autoBookmarks not enabled.
	doc.Add(layout.NewHeading("Title", layout.H1))

	var buf bytes.Buffer
	_, _ = doc.WriteTo(&buf)

	pdf := buf.String()
	if strings.Contains(pdf, "/Outlines") {
		t.Error("should not have outlines when autoBookmarks is disabled")
	}
}

func TestAutoBookmarksDoesNotOverrideManual(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetAutoBookmarks(true)

	// Add manual bookmarks first.
	doc.AddOutline("Manual Bookmark", FitDest(0))

	doc.Add(layout.NewHeading("Auto Heading", layout.H1))

	var buf bytes.Buffer
	_, _ = doc.WriteTo(&buf)

	pdf := buf.String()
	// Manual bookmark should be present.
	if !strings.Contains(pdf, "Manual Bookmark") {
		t.Error("missing manual bookmark")
	}
	// Auto bookmark should NOT override manual ones.
	// (autoBookmarks only runs when len(outlines) == 0)
}

func TestAutoBookmarksQpdf(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetAutoBookmarks(true)
	doc.Info.Title = "Bookmarked Doc"

	doc.Add(layout.NewHeading("Introduction", layout.H1))
	doc.Add(layout.NewParagraph("Content here.", font.Helvetica, 12))
	doc.Add(layout.NewHeading("Methods", layout.H1))
	doc.Add(layout.NewHeading("Data Collection", layout.H2))
	doc.Add(layout.NewParagraph("More content.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	runQpdfCheck(t, buf.Bytes())
}
