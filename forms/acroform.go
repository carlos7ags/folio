// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"github.com/carlos7ags/folio/core"
)

// AcroForm manages the interactive form fields for a PDF document.
type AcroForm struct {
	fields []*Field
}

// NewAcroForm creates an empty form.
func NewAcroForm() *AcroForm {
	return &AcroForm{}
}

// Add adds a field to the form.
func (af *AcroForm) Add(f *Field) *AcroForm {
	af.fields = append(af.fields, f)
	return af
}

// Fields returns all fields in the form.
func (af *AcroForm) Fields() []*Field {
	return af.fields
}

// Build creates the /AcroForm dictionary and all field/widget objects.
// Returns the AcroForm dictionary reference and a map of pageIndex → widget refs
// (so the document layer can add them to each page's /Annots array).
func (af *AcroForm) Build(
	addObject func(core.PdfObject) *core.PdfIndirectReference,
	pageRefs []*core.PdfIndirectReference,
) (*core.PdfIndirectReference, map[int][]*core.PdfIndirectReference) {
	if len(af.fields) == 0 {
		return nil, nil
	}

	fieldsArr := core.NewPdfArray()
	pageWidgets := make(map[int][]*core.PdfIndirectReference)

	for _, field := range af.fields {
		fieldRef, widgets := field.ToDict(addObject, pageRefs)
		fieldsArr.Add(fieldRef)

		for _, w := range widgets {
			pageWidgets[w.pageIndex] = append(pageWidgets[w.pageIndex], w.ref)
		}
	}

	// Build the AcroForm dictionary.
	formDict := core.NewPdfDictionary()
	formDict.Set("Fields", fieldsArr)

	// Default resources: standard fonts for field rendering.
	dr := core.NewPdfDictionary()
	fontDict := core.NewPdfDictionary()

	// Helvetica for text fields.
	helvetica := core.NewPdfDictionary()
	helvetica.Set("Type", core.NewPdfName("Font"))
	helvetica.Set("Subtype", core.NewPdfName("Type1"))
	helvetica.Set("BaseFont", core.NewPdfName("Helvetica"))
	helvRef := addObject(helvetica)
	fontDict.Set("Helv", helvRef)

	// ZapfDingbats for checkboxes.
	zapf := core.NewPdfDictionary()
	zapf.Set("Type", core.NewPdfName("Font"))
	zapf.Set("Subtype", core.NewPdfName("Type1"))
	zapf.Set("BaseFont", core.NewPdfName("ZapfDingbats"))
	zapfRef := addObject(zapf)
	fontDict.Set("ZaDb", zapfRef)

	dr.Set("Font", fontDict)
	formDict.Set("DR", dr)

	// Default appearance for text fields.
	formDict.Set("DA", core.NewPdfLiteralString("/Helv 0 Tf 0 0 0 rg"))

	// NeedAppearances: tells the viewer to generate appearances for
	// fields that don't have explicit /AP dictionaries.
	formDict.Set("NeedAppearances", core.NewPdfBoolean(true))

	return addObject(formDict), pageWidgets
}
