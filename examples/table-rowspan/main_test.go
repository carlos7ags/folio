// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

var examplePDFBytes = sync.OnceValue(func() []byte {
	var buf bytes.Buffer
	if _, err := buildDocument().WriteTo(&buf); err != nil {
		panic("WriteTo: " + err.Error())
	}
	return buf.Bytes()
})

func examplePDFReader(t *testing.T) *reader.PdfReader {
	t.Helper()
	r, err := reader.Parse(examplePDFBytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	return r
}

func TestRowspanExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 1000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >1 KB", len(pdf))
	}
}

// TestRowspanExampleMultiPage checks the document now spans multiple
// pages: the roster section is long enough to force a page break, and
// (per TestRowspanExampleGroupCrossesPageBreak) that break lands right
// before a rowspan group.
func TestRowspanExampleMultiPage(t *testing.T) {
	r := examplePDFReader(t)
	if got := r.PageCount(); got < 2 {
		t.Errorf("PageCount = %d, want >= 2", got)
	}
}

// TestRowspanExampleContent confirms every cell — including the cells
// in rows that sit beside a rowspanning cell — is emitted. Before the
// rowspan geometry fix the spanning cells still rendered; the failure
// mode was purely visual (drawn one row tall), so this guards content
// rather than geometry. The geometry assertions live in the layout
// package's TestTableRowspan* tests.
func TestRowspanExampleContent(t *testing.T) {
	r := examplePDFReader(t)
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	for _, want := range []string{
		"Span", "B1", "B2",
		"Morning", "Registration", "Keynote", "Workshop intro", "All day: help desk",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("page text missing %q; extracted:\n%s", want, text)
		}
	}
}

// TestRowspanExampleGroupCrossesPageBreak is the issue #362 regression:
// the roster's rowspanning "Platform Team" cell and the three rows under
// it must land together on the page after the split, not be torn between
// the page where the split occurs and the next one.
func TestRowspanExampleGroupCrossesPageBreak(t *testing.T) {
	r := examplePDFReader(t)
	n := r.PageCount()
	if n < 2 {
		t.Fatalf("expected >= 2 pages, got %d", n)
	}

	pageText := func(i int) string {
		page, err := r.Page(i)
		if err != nil {
			t.Fatalf("Page(%d): %v", i, err)
		}
		text, err := page.ExtractText()
		if err != nil {
			t.Fatalf("Page(%d).ExtractText: %v", i, err)
		}
		return text
	}

	// Find the page where the group's members appear; the group cell
	// text and all of its rows must be on that same page.
	groupMembers := []string{"Alice", "Bilal", "Chen", "Divya"}
	groupPage := -1
	for i := 0; i < n; i++ {
		text := pageText(i)
		if strings.Contains(text, "Alice") {
			groupPage = i
			break
		}
	}
	if groupPage == -1 {
		t.Fatal("could not find the page containing the rowspan group")
	}
	text := pageText(groupPage)
	if !strings.Contains(text, "Platform Team") {
		t.Errorf("page %d has the group's first member but not the spanning cell text", groupPage)
	}
	for _, member := range groupMembers {
		if !strings.Contains(text, member) {
			t.Errorf("page %d missing group member %q — the group was split across pages", groupPage, member)
		}
	}
	if groupPage == 0 {
		t.Error("expected the rowspan group on a later page than the first (it should have been pushed past the natural split)")
	}
}
