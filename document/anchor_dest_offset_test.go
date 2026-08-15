// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"bytes"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
)

// TestNamedDestPageIndexOffsetByManualPages verifies that an anchor produced by
// flow content is registered on its ABSOLUTE page index — offset past any
// manually-added pages (manualPageCount + i) — so an internal link lands on the
// right page in a document that mixes manual pages with laid-out flow content.
func TestNamedDestPageIndexOffsetByManualPages(t *testing.T) {
	d := document.NewDocument(document.PageSizeLetter)
	d.AddPage() // manual page 0
	d.AddPage() // manual page 1

	// Flow content: the anchored element becomes flow page 0 → absolute page 2.
	d.Add(layout.NewAnchor(layout.NewParagraph("Target section", font.Helvetica, 12), "target"))

	var buf bytes.Buffer
	if _, err := d.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	if got := r.PageCount(); got != 3 {
		t.Fatalf("page count = %d, want 3 (2 manual + 1 flow)", got)
	}

	// /Dests → target → [pageRef /XYZ ...].
	cat := r.Catalog()
	destsObj, err := r.ResolveObject(cat.Get("Dests"))
	if err != nil {
		t.Fatalf("resolve /Dests: %v", err)
	}
	dests, ok := destsObj.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("/Dests is %T, want *core.PdfDictionary", destsObj)
	}
	arrObj, err := r.ResolveObject(dests.Get("target"))
	if err != nil {
		t.Fatalf("resolve dest target: %v", err)
	}
	arr, ok := arrObj.(*core.PdfArray)
	if !ok || arr.Len() < 1 {
		t.Fatalf("dest target is %T/len<1, want a destination array", arrObj)
	}
	pageRef, ok := arr.At(0).(*core.PdfIndirectReference)
	if !ok {
		t.Fatalf("dest page target is %T, want *core.PdfIndirectReference", arr.At(0))
	}

	if idx := pageIndexInKids(t, r, pageRef.Num()); idx != 2 {
		t.Errorf("anchor destination page index = %d, want 2 (offset past the 2 manual pages)", idx)
	}
}

// TestNamedDestDuplicateNameFirstWins verifies that when a destination name is
// registered more than once (duplicate ids, or an id colliding with a caller's
// AddNamedDest), the /Dests dictionary keeps the FIRST registration — matching
// the link-annotation resolver, which stops at the first match, so both point
// at the same target.
func TestNamedDestDuplicateNameFirstWins(t *testing.T) {
	d := document.NewDocument(document.PageSizeLetter)
	d.AddPage() // page 0
	d.AddPage() // page 1

	d.AddNamedDest(document.NamedDest{Name: "dup", PageIndex: 0, FitType: "XYZ", Top: 700})
	d.AddNamedDest(document.NamedDest{Name: "dup", PageIndex: 1, FitType: "XYZ", Top: 700})

	var buf bytes.Buffer
	if _, err := d.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	r, err := reader.Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}

	destsObj, err := r.ResolveObject(r.Catalog().Get("Dests"))
	if err != nil {
		t.Fatalf("resolve /Dests: %v", err)
	}
	dests, ok := destsObj.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("/Dests is %T, want *core.PdfDictionary", destsObj)
	}
	arrObj, err := r.ResolveObject(dests.Get("dup"))
	if err != nil {
		t.Fatalf("resolve dest dup: %v", err)
	}
	arr, ok := arrObj.(*core.PdfArray)
	if !ok || arr.Len() < 1 {
		t.Fatalf("dest dup is %T/len<1, want a destination array", arrObj)
	}
	pageRef, ok := arr.At(0).(*core.PdfIndirectReference)
	if !ok {
		t.Fatalf("dest page target is %T, want *core.PdfIndirectReference", arr.At(0))
	}
	if idx := pageIndexInKids(t, r, pageRef.Num()); idx != 0 {
		t.Errorf("duplicate name resolved to page %d, want 0 (first registration must win)", idx)
	}
}

// pageIndexInKids returns the position of the page object objNum within the
// page tree's /Kids array.
func pageIndexInKids(t *testing.T, r *reader.PdfReader, objNum int) int {
	t.Helper()
	pagesObj, err := r.ResolveObject(r.Catalog().Get("Pages"))
	if err != nil {
		t.Fatalf("resolve /Pages: %v", err)
	}
	pages, ok := pagesObj.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("/Pages is %T, want *core.PdfDictionary", pagesObj)
	}
	kids, ok := pages.Get("Kids").(*core.PdfArray)
	if !ok {
		t.Fatal("/Pages has no /Kids array")
	}
	for i := 0; i < kids.Len(); i++ {
		if ref, ok := kids.At(i).(*core.PdfIndirectReference); ok && ref.Num() == objNum {
			return i
		}
	}
	t.Fatalf("page object %d not found in /Kids", objNum)
	return -1
}
