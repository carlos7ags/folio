// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"fmt"

	"github.com/carlos7ags/folio/core"
)

// buildDA creates the default appearance string (DA) for a form field.
// The DA string contains content stream operators that set the font and color.
// Example: "/Helv 12 Tf 0 0 0 rg"
func buildDA(f *Field) string {
	fontSize := f.FontSize
	if fontSize == 0 {
		fontSize = 0 // 0 = auto-size in PDF viewers
	}
	fontName := f.FontName
	if fontName == "" {
		fontName = "Helv"
	}
	r, g, b := f.TextColor[0], f.TextColor[1], f.TextColor[2]
	return fmt.Sprintf("/%s %s Tf %s %s %s rg",
		fontName, fmtNum(fontSize),
		fmtNum(r), fmtNum(g), fmtNum(b))
}

// buildCheckboxAppearance creates the /AP dictionary for a checkbox field.
// The returned dictionary contains a /N (normal) sub-dictionary with /Yes and /Off appearance streams.
func buildCheckboxAppearance(f *Field, addObject func(core.PdfObject) *core.PdfIndirectReference) *core.PdfDictionary {
	w := f.Rect[2] - f.Rect[0]
	h := f.Rect[3] - f.Rect[1]

	// "Yes" appearance: checkmark.
	yesStream := core.NewPdfStream([]byte(
		fmt.Sprintf("q\n0 0 0 rg\nBT\n/ZaDb %s Tf\n%s %s Td\n(4) Tj\nET\nQ",
			fmtNum(h*0.8), fmtNum(w*0.15), fmtNum(h*0.15)),
	))
	yesStream.Dict.Set("Type", core.NewPdfName("XObject"))
	yesStream.Dict.Set("Subtype", core.NewPdfName("Form"))
	yesStream.Dict.Set("BBox", core.NewPdfArray(
		core.NewPdfInteger(0), core.NewPdfInteger(0),
		core.NewPdfReal(w), core.NewPdfReal(h),
	))
	// ZapfDingbats font resource for checkmark.
	fontDict := core.NewPdfDictionary()
	fontDict.Set("Type", core.NewPdfName("Font"))
	fontDict.Set("Subtype", core.NewPdfName("Type1"))
	fontDict.Set("BaseFont", core.NewPdfName("ZapfDingbats"))
	fontRef := addObject(fontDict)
	resDict := core.NewPdfDictionary()
	fontResDict := core.NewPdfDictionary()
	fontResDict.Set("ZaDb", fontRef)
	resDict.Set("Font", fontResDict)
	yesStream.Dict.Set("Resources", resDict)
	yesRef := addObject(yesStream)

	// "Off" appearance: empty box.
	offStream := core.NewPdfStream([]byte(""))
	offStream.Dict.Set("Type", core.NewPdfName("XObject"))
	offStream.Dict.Set("Subtype", core.NewPdfName("Form"))
	offStream.Dict.Set("BBox", core.NewPdfArray(
		core.NewPdfInteger(0), core.NewPdfInteger(0),
		core.NewPdfReal(w), core.NewPdfReal(h),
	))
	offRef := addObject(offStream)

	// Normal appearance dictionary.
	nDict := core.NewPdfDictionary()
	nDict.Set("Yes", yesRef)
	nDict.Set("Off", offRef)

	ap := core.NewPdfDictionary()
	ap.Set("N", nDict)
	return ap
}

// buildWidgetDict creates a standalone widget annotation dictionary for a radio button child.
// The widget is linked to its parent field and, when valid, to the target page.
func buildWidgetDict(child *Field, parentRef *core.PdfIndirectReference, pageRefs []*core.PdfIndirectReference) *core.PdfDictionary {
	w := core.NewPdfDictionary()
	w.Set("Type", core.NewPdfName("Annot"))
	w.Set("Subtype", core.NewPdfName("Widget"))
	w.Set("Rect", core.NewPdfArray(
		core.NewPdfReal(child.Rect[0]),
		core.NewPdfReal(child.Rect[1]),
		core.NewPdfReal(child.Rect[2]),
		core.NewPdfReal(child.Rect[3]),
	))
	w.Set("Parent", parentRef)

	if child.PageIndex >= 0 && child.PageIndex < len(pageRefs) {
		w.Set("P", pageRefs[child.PageIndex])
	}

	// Appearance state.
	w.Set("AS", core.NewPdfName("Off"))

	return w
}

// fmtNum formats a float64 as a compact string for PDF content streams.
// Integers are rendered without a decimal point; others use two decimal places.
func fmtNum(v float64) string {
	if v == float64(int(v)) {
		return fmt.Sprintf("%d", int(v))
	}
	return fmt.Sprintf("%.2f", v)
}
