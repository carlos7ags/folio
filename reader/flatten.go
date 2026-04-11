// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"fmt"

	"github.com/carlos7ags/folio/core"
)

// FlattenForms renders all form field appearances into the page content
// and removes the interactive AcroForm, making the PDF non-editable.
//
// Each widget annotation's normal appearance (/AP /N) is painted into the
// page's content stream at the widget's /Rect position. The widget is then
// removed from the page's /Annots array. Finally, /AcroForm is removed
// from the catalog.
func (m *Modifier) FlattenForms() error {
	// Process each page.
	for i, pageDict := range m.pageDicts {
		if pageDict == nil {
			continue
		}
		if err := m.flattenPage(pageDict, i); err != nil {
			return fmt.Errorf("flatten page %d: %w", i, err)
		}
	}

	// Remove /AcroForm from catalog.
	removeEntry(m.catalog, "AcroForm")
	return nil
}

// flattenPage processes a single page's annotations.
func (m *Modifier) flattenPage(pageDict *core.PdfDictionary, pageIndex int) error {
	annotsObj := pageDict.Get("Annots")
	if annotsObj == nil {
		return nil
	}

	annots, ok := annotsObj.(*core.PdfArray)
	if !ok {
		return nil
	}

	// Collect XObjects to add to resources and content stream operations.
	var contentOps []byte
	xobjects := core.NewPdfDictionary()
	xobjCount := 0
	var keepAnnots []core.PdfObject

	for _, annotObj := range annots.All() {
		annotDict, ok := annotObj.(*core.PdfDictionary)
		if !ok {
			// Could be an indirect ref — try to use as-is.
			keepAnnots = append(keepAnnots, annotObj)
			continue
		}

		// Check if this is a widget annotation.
		subtype := annotDict.Get("Subtype")
		if subtype == nil {
			keepAnnots = append(keepAnnots, annotObj)
			continue
		}
		subtypeName, ok := subtype.(*core.PdfName)
		if !ok || subtypeName.Value != "Widget" {
			keepAnnots = append(keepAnnots, annotObj)
			continue
		}

		// Get the appearance stream.
		apObj := annotDict.Get("AP")
		if apObj == nil {
			// No appearance — skip (remove widget but nothing to render).
			continue
		}
		apDict, ok := apObj.(*core.PdfDictionary)
		if !ok {
			continue
		}

		// Get /N (normal appearance).
		nObj := apDict.Get("N")
		if nObj == nil {
			continue
		}

		// /N can be a stream (single appearance) or a dict of streams
		// (keyed by appearance state like "Yes"/"Off").
		var apStream core.PdfObject
		switch n := nObj.(type) {
		case *core.PdfStream:
			apStream = n
		case *core.PdfDictionary:
			// For checkboxes/radios, use the current /AS state.
			asObj := annotDict.Get("AS")
			if asObj != nil {
				if asName, ok := asObj.(*core.PdfName); ok {
					apStream = n.Get(asName.Value)
				}
			}
			// If no AS or AS key not found, try "Yes" or first non-Off key.
			if apStream == nil {
				for key, value := range n.All() {
					if key != "Off" {
						apStream = value
						break
					}
				}
			}
		}

		if apStream == nil {
			continue
		}

		// Get the widget's /Rect.
		rectObj := annotDict.Get("Rect")
		if rectObj == nil {
			continue
		}
		rectArr, ok := rectObj.(*core.PdfArray)
		if !ok || rectArr.Len() < 4 {
			continue
		}

		x1 := pdfFloat(rectArr.At(0))
		y1 := pdfFloat(rectArr.At(1))
		x2 := pdfFloat(rectArr.At(2))
		y2 := pdfFloat(rectArr.At(3))
		w := x2 - x1
		h := y2 - y1

		if w <= 0 || h <= 0 {
			continue
		}

		// Register the appearance stream as an XObject resource.
		xobjCount++
		xobjName := fmt.Sprintf("FlatAP%d", xobjCount)
		apRef := m.writer.AddObject(apStream)
		xobjects.Set(xobjName, apRef)

		// Build content stream to paint the XObject at the rect position.
		// q [w 0 0 h x1 y1] cm /Name Do Q
		op := fmt.Sprintf("q %.4f 0 0 %.4f %.4f %.4f cm /%s Do Q\n", w, h, x1, y1, xobjName)
		contentOps = append(contentOps, []byte(op)...)
	}

	if xobjCount == 0 {
		// No widgets flattened — just clean up annotations if any were removed.
		if len(keepAnnots) != annots.Len() {
			if len(keepAnnots) == 0 {
				removeEntry(pageDict, "Annots")
			} else {
				pageDict.Set("Annots", core.NewPdfArray(keepAnnots...))
			}
		}
		return nil
	}

	// Merge new XObjects into page resources.
	resObj := pageDict.Get("Resources")
	var resDict *core.PdfDictionary
	if resObj != nil {
		resDict, _ = resObj.(*core.PdfDictionary)
	}
	if resDict == nil {
		resDict = core.NewPdfDictionary()
		pageDict.Set("Resources", resDict)
	}

	existingXObj := resDict.Get("XObject")
	if existingXObj != nil {
		if existingDict, ok := existingXObj.(*core.PdfDictionary); ok {
			for key, value := range xobjects.All() {
				existingDict.Set(key, value)
			}
		}
	} else {
		resDict.Set("XObject", xobjects)
	}

	// Append flattening content stream to page's /Contents.
	flatStream := core.NewPdfStream(contentOps)
	flatRef := m.writer.AddObject(flatStream)

	contentsObj := pageDict.Get("Contents")
	if contentsObj == nil {
		pageDict.Set("Contents", flatRef)
	} else {
		// Wrap existing contents + new stream in an array.
		switch c := contentsObj.(type) {
		case *core.PdfArray:
			c.Add(flatRef)
		default:
			pageDict.Set("Contents", core.NewPdfArray(contentsObj, flatRef))
		}
	}

	// Update annotations — keep non-widget annotations only.
	if len(keepAnnots) == 0 {
		removeEntry(pageDict, "Annots")
	} else {
		pageDict.Set("Annots", &core.PdfArray{Elements: keepAnnots})
	}

	return nil
}

// pdfFloat extracts a float64 from a PDF numeric object.
func pdfFloat(obj core.PdfObject) float64 {
	if n, ok := obj.(*core.PdfNumber); ok {
		return n.FloatValue()
	}
	return 0
}
