// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import "github.com/carlos7ags/folio/core"

// LabelStyle specifies the numbering style for page labels.
type LabelStyle string

const (
	LabelDecimal    LabelStyle = "D" // 1, 2, 3, ...
	LabelRomanUpper LabelStyle = "R" // I, II, III, ...
	LabelRomanLower LabelStyle = "r" // i, ii, iii, ...
	LabelAlphaUpper LabelStyle = "A" // A, B, C, ...
	LabelAlphaLower LabelStyle = "a" // a, b, c, ...
	LabelNone       LabelStyle = ""  // no numbering (prefix only)
)

// PageLabelRange defines a page label range starting at a given page index.
// All pages from PageIndex until the next range use this label style.
type PageLabelRange struct {
	PageIndex int        // 0-based page index where this range starts
	Style     LabelStyle // numbering style
	Prefix    string     // prefix before the number (e.g. "A-")
	Start     int        // starting number (default 1)
}

// SetPageLabels configures page label ranges for the document.
// Each range applies from its PageIndex until the next range starts.
//
// Example — front matter in Roman numerals, body in decimal:
//
//	doc.SetPageLabels(
//	    document.PageLabelRange{PageIndex: 0, Style: document.LabelRomanLower, Start: 1},
//	    document.PageLabelRange{PageIndex: 4, Style: document.LabelDecimal, Start: 1},
//	)
func (d *Document) SetPageLabels(ranges ...PageLabelRange) {
	d.pageLabels = ranges
}

// buildPageLabels creates the /PageLabels number tree on the catalog.
func buildPageLabels(ranges []PageLabelRange, catalog *core.PdfDictionary, addObject func(core.PdfObject) *core.PdfIndirectReference) {
	nums := core.NewPdfArray()

	for _, r := range ranges {
		nums.Add(core.NewPdfInteger(r.PageIndex))

		labelDict := core.NewPdfDictionary()
		if r.Style != "" {
			labelDict.Set("S", core.NewPdfName(string(r.Style)))
		}
		if r.Prefix != "" {
			labelDict.Set("P", core.NewPdfLiteralString(r.Prefix))
		}
		if r.Start > 0 && r.Start != 1 {
			labelDict.Set("St", core.NewPdfInteger(r.Start))
		}
		nums.Add(labelDict)
	}

	labelsDict := core.NewPdfDictionary()
	labelsDict.Set("Nums", nums)

	labelsRef := addObject(labelsDict)
	catalog.Set("PageLabels", labelsRef)
}
