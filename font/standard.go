// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package font provides PDF font definitions and (later) font parsing.
package font

import "github.com/carlos7ags/folio/core"

// Standard represents one of the 14 standard PDF fonts that every
// conforming viewer must support (ISO 32000 §9.6.2.2).
// These fonts require no embedding — only a reference by name.
type Standard struct {
	name string // PDF BaseFont name (e.g. "Helvetica")
}

// Name returns the PDF BaseFont name.
func (f *Standard) Name() string {
	return f.name
}

// Dict returns the PDF font dictionary for this standard font.
//
//	<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>
func (f *Standard) Dict() *core.PdfDictionary {
	d := core.NewPdfDictionary()
	d.Set("Type", core.NewPdfName("Font"))
	d.Set("Subtype", core.NewPdfName("Type1"))
	d.Set("BaseFont", core.NewPdfName(f.name))
	return d
}

// The 14 standard PDF fonts.
var (
	Helvetica            = &Standard{"Helvetica"}
	HelveticaBold        = &Standard{"Helvetica-Bold"}
	HelveticaOblique     = &Standard{"Helvetica-Oblique"}
	HelveticaBoldOblique = &Standard{"Helvetica-BoldOblique"}

	TimesRoman      = &Standard{"Times-Roman"}
	TimesBold       = &Standard{"Times-Bold"}
	TimesItalic     = &Standard{"Times-Italic"}
	TimesBoldItalic = &Standard{"Times-BoldItalic"}

	Courier            = &Standard{"Courier"}
	CourierBold        = &Standard{"Courier-Bold"}
	CourierOblique     = &Standard{"Courier-Oblique"}
	CourierBoldOblique = &Standard{"Courier-BoldOblique"}

	Symbol       = &Standard{"Symbol"}
	ZapfDingbats = &Standard{"ZapfDingbats"}
)

// StandardFonts returns all 14 standard fonts.
func StandardFonts() []*Standard {
	return []*Standard{
		Helvetica, HelveticaBold, HelveticaOblique, HelveticaBoldOblique,
		TimesRoman, TimesBold, TimesItalic, TimesBoldItalic,
		Courier, CourierBold, CourierOblique, CourierBoldOblique,
		Symbol, ZapfDingbats,
	}
}
