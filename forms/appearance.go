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

	// "Yes" appearance: border + checkmark.
	yesStream := core.NewPdfStream([]byte(
		fmt.Sprintf("q\n0.6 0.6 0.6 RG\n0.5 w\n0 0 %s %s re S\n"+
			"0 0 0 rg\nBT\n/ZaDb %s Tf\n%s %s Td\n(4) Tj\nET\nQ",
			fmtNum(w), fmtNum(h),
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

	// "Off" appearance: border only.
	offStream := core.NewPdfStream([]byte(
		fmt.Sprintf("q\n0.6 0.6 0.6 RG\n0.5 w\n0 0 %s %s re S\nQ", fmtNum(w), fmtNum(h)),
	))
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
func buildWidgetDict(child *Field, parentRef *core.PdfIndirectReference, pageRefs []*core.PdfIndirectReference, addObject func(core.PdfObject) *core.PdfIndirectReference) *core.PdfDictionary {
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

	// Radio button appearance: each widget needs its own /AP with a
	// unique export value key so the viewer can select them independently.
	exportVal := child.ExportValue
	if exportVal == "" {
		exportVal = "Yes"
	}
	rw := child.Rect[2] - child.Rect[0]
	rh := child.Rect[3] - child.Rect[1]
	ap := buildRadioAppearance(exportVal, rw, rh, addObject)
	w.Set("AP", ap)

	return w
}

// buildRadioAppearance creates an /AP dictionary for a radio button widget.
// The /N sub-dictionary contains an appearance for the export value (filled
// circle) and /Off (empty circle).
func buildRadioAppearance(exportVal string, w, h float64, addObject func(core.PdfObject) *core.PdfIndirectReference) *core.PdfDictionary {
	cx := w / 2
	cy := h / 2
	r := min(w, h) / 2 * 0.8

	// "On" appearance: outlined box with a filled dot in the center.
	dotR := r * 0.5
	onContent := fmt.Sprintf(
		"q\n0.6 0.6 0.6 RG\n0.5 w\n%s %s %s %s re S\n"+
			"0 0 0 rg\n%s %s %s %s re f\nQ",
		fmtNum(cx-r), fmtNum(cy-r), fmtNum(r*2), fmtNum(r*2),
		fmtNum(cx-dotR), fmtNum(cy-dotR), fmtNum(dotR*2), fmtNum(dotR*2),
	)
	onStream := core.NewPdfStream([]byte(onContent))
	onStream.Dict.Set("Type", core.NewPdfName("XObject"))
	onStream.Dict.Set("Subtype", core.NewPdfName("Form"))
	onStream.Dict.Set("BBox", core.NewPdfArray(
		core.NewPdfInteger(0), core.NewPdfInteger(0),
		core.NewPdfReal(w), core.NewPdfReal(h),
	))
	onRef := addObject(onStream)

	// "Off" appearance: empty box.
	offContent := fmt.Sprintf(
		"q\n0.6 0.6 0.6 RG\n0.5 w\n%s %s %s %s re S\nQ",
		fmtNum(cx-r), fmtNum(cy-r), fmtNum(r*2), fmtNum(r*2),
	)
	offStream := core.NewPdfStream([]byte(offContent))
	offStream.Dict.Set("Type", core.NewPdfName("XObject"))
	offStream.Dict.Set("Subtype", core.NewPdfName("Form"))
	offStream.Dict.Set("BBox", core.NewPdfArray(
		core.NewPdfInteger(0), core.NewPdfInteger(0),
		core.NewPdfReal(w), core.NewPdfReal(h),
	))
	offRef := addObject(offStream)

	nDict := core.NewPdfDictionary()
	nDict.Set(exportVal, onRef)
	nDict.Set("Off", offRef)

	ap := core.NewPdfDictionary()
	ap.Set("N", nDict)
	return ap
}

// fmtNum formats a float64 as a compact string for PDF content streams.
// Integers are rendered without a decimal point; others use two decimal places.
func fmtNum(v float64) string {
	if v == float64(int(v)) {
		return fmt.Sprintf("%d", int(v))
	}
	return fmt.Sprintf("%.2f", v)
}
