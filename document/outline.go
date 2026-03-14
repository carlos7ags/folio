// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"github.com/carlos7ags/folio/core"
)

// Destination describes where a bookmark links to.
type Destination struct {
	PageIndex int // zero-based page index
	Type      DestType
	Left      float64 // for XYZ: left coordinate (0 = use current)
	Top       float64 // for XYZ: top coordinate (0 = use current)
	Zoom      float64 // for XYZ: zoom factor (0 = use current)
}

// DestType is the type of PDF destination.
type DestType int

const (
	// DestFit displays the page with its contents magnified just enough
	// to fit the entire page within the window (ISO 32000 §12.3.2.2).
	DestFit DestType = iota

	// DestXYZ positions the page at (left, top) with the given zoom.
	// Zero values mean "keep current" (ISO 32000 §12.3.2.2).
	DestXYZ
)

// FitDest creates a Fit destination for the given page index.
func FitDest(pageIndex int) Destination {
	return Destination{PageIndex: pageIndex, Type: DestFit}
}

// XYZDest creates an XYZ destination for the given page, position, and zoom.
// Pass 0 for left, top, or zoom to keep the current value.
func XYZDest(pageIndex int, left, top, zoom float64) Destination {
	return Destination{
		PageIndex: pageIndex,
		Type:      DestXYZ,
		Left:      left,
		Top:       top,
		Zoom:      zoom,
	}
}

// Outline represents a bookmark entry in the PDF outline tree.
// Outlines can be nested (children form sub-bookmarks).
type Outline struct {
	Title    string
	Dest     Destination
	Children []Outline
}

// AddOutline adds a top-level bookmark to the document.
func (d *Document) AddOutline(title string, dest Destination) *Outline {
	d.outlines = append(d.outlines, Outline{Title: title, Dest: dest})
	return &d.outlines[len(d.outlines)-1]
}

// AddChild adds a child bookmark under this outline entry.
func (o *Outline) AddChild(title string, dest Destination) *Outline {
	o.Children = append(o.Children, Outline{Title: title, Dest: dest})
	return &o.Children[len(o.Children)-1]
}

// buildOutlineTree registers the outline tree as PDF objects and returns
// the /Outlines dict reference. Returns nil if there are no outlines.
//
// PDF outline structure (ISO 32000 §12.3.3):
//
//	/Outlines dict
//	  /First → first item
//	  /Last  → last item
//	  /Count → total visible items
//
//	Each item:
//	  /Title  → string
//	  /Dest   → [pageRef /Fit] or [pageRef /XYZ left top zoom]
//	  /Parent → parent ref
//	  /Next   → next sibling ref
//	  /Prev   → prev sibling ref
//	  /First  → first child ref (if has children)
//	  /Last   → last child ref (if has children)
//	  /Count  → number of visible descendants
func buildOutlineTree(
	outlines []Outline,
	pageRefs []*core.PdfIndirectReference,
	addObject func(core.PdfObject) *core.PdfIndirectReference,
) *core.PdfIndirectReference {
	if len(outlines) == 0 {
		return nil
	}

	// Create the root /Outlines dictionary
	outlinesDict := core.NewPdfDictionary()
	outlinesDict.Set("Type", core.NewPdfName("Outlines"))
	outlinesRef := addObject(outlinesDict)

	// Build the item linked list
	itemRefs := buildOutlineItems(outlines, outlinesRef, pageRefs, addObject)

	outlinesDict.Set("First", itemRefs[0])
	outlinesDict.Set("Last", itemRefs[len(itemRefs)-1])
	outlinesDict.Set("Count", core.NewPdfInteger(countOutlines(outlines)))

	return outlinesRef
}

// buildOutlineItems recursively builds outline items and returns their refs.
func buildOutlineItems(
	items []Outline,
	parentRef *core.PdfIndirectReference,
	pageRefs []*core.PdfIndirectReference,
	addObject func(core.PdfObject) *core.PdfIndirectReference,
) []*core.PdfIndirectReference {

	// First pass: create all dicts and register them to get refs
	dicts := make([]*core.PdfDictionary, len(items))
	refs := make([]*core.PdfIndirectReference, len(items))
	for i := range items {
		dicts[i] = core.NewPdfDictionary()
		refs[i] = addObject(dicts[i])
	}

	// Second pass: fill in all fields now that we have all refs
	for i, item := range items {
		d := dicts[i]
		d.Set("Title", core.NewPdfLiteralString(item.Title))
		d.Set("Parent", parentRef)

		// Destination
		if item.Dest.PageIndex >= 0 && item.Dest.PageIndex < len(pageRefs) {
			d.Set("Dest", buildDestArray(item.Dest, pageRefs[item.Dest.PageIndex]))
		}

		// Sibling links
		if i > 0 {
			d.Set("Prev", refs[i-1])
		}
		if i < len(items)-1 {
			d.Set("Next", refs[i+1])
		}

		// Children
		if len(item.Children) > 0 {
			childRefs := buildOutlineItems(item.Children, refs[i], pageRefs, addObject)
			d.Set("First", childRefs[0])
			d.Set("Last", childRefs[len(childRefs)-1])
			d.Set("Count", core.NewPdfInteger(countOutlines(item.Children)))
		}
	}

	return refs
}

// buildDestArray creates the PDF destination array for an outline item.
func buildDestArray(dest Destination, pageRef *core.PdfIndirectReference) *core.PdfArray {
	switch dest.Type {
	case DestXYZ:
		return core.NewPdfArray(
			pageRef,
			core.NewPdfName("XYZ"),
			pdfNumOrNull(dest.Left),
			pdfNumOrNull(dest.Top),
			pdfNumOrNull(dest.Zoom),
		)
	default: // DestFit
		return core.NewPdfArray(
			pageRef,
			core.NewPdfName("Fit"),
		)
	}
}

// pdfNumOrNull returns a PdfReal for non-zero values, PdfNull for zero
// (which means "keep current" in PDF destinations).
func pdfNumOrNull(v float64) core.PdfObject {
	if v == 0 {
		return core.NewPdfNull()
	}
	return core.NewPdfReal(v)
}

// countOutlines returns the total number of outline items (including children).
func countOutlines(items []Outline) int {
	n := len(items)
	for _, item := range items {
		n += countOutlines(item.Children)
	}
	return n
}
