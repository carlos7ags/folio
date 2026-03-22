// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "github.com/carlos7ags/folio/barcode"

// BarcodeElement is a layout element that renders a barcode in the document flow.
type BarcodeElement struct {
	bc     *barcode.Barcode
	width  float64 // display width in points
	height float64 // display height in points (0 = auto from aspect ratio)
	align  Align
}

// NewBarcodeElement creates a layout element from a generated barcode.
// Width sets the display width in points. Height is computed from the
// barcode's aspect ratio if 0.
func NewBarcodeElement(bc *barcode.Barcode, width float64) *BarcodeElement {
	return &BarcodeElement{
		bc:    bc,
		width: width,
		align: AlignLeft,
	}
}

// SetHeight sets an explicit display height. If 0, height is computed
// from the barcode's module aspect ratio.
func (be *BarcodeElement) SetHeight(h float64) *BarcodeElement {
	be.height = h
	return be
}

// SetAlign sets horizontal alignment.
func (be *BarcodeElement) SetAlign(a Align) *BarcodeElement {
	be.align = a
	return be
}

// resolveHeight returns the display height, computing it from the barcode
// aspect ratio if no explicit height was set.
func (be *BarcodeElement) resolveHeight() float64 {
	if be.height > 0 {
		return be.height
	}
	if be.bc == nil || be.bc.Width() == 0 {
		return be.width
	}
	return be.width * float64(be.bc.Height()) / float64(be.bc.Width())
}

// Layout implements Element.
func (be *BarcodeElement) Layout(maxWidth float64) []Line {
	w := be.width
	if w > maxWidth {
		w = maxWidth
	}
	h := be.resolveHeight()
	return []Line{{
		Width: w, Height: h, Align: be.align, IsLast: true,
	}}
}

// PlanLayout implements Element.
func (be *BarcodeElement) PlanLayout(area LayoutArea) LayoutPlan {
	w := be.width
	if w > area.Width {
		w = area.Width
	}
	h := be.resolveHeight()

	if h > area.Height && area.Height > 0 {
		return LayoutPlan{Status: LayoutNothing}
	}

	x := 0.0
	switch be.align {
	case AlignCenter:
		x = (area.Width - w) / 2
	case AlignRight:
		x = area.Width - w
	}

	capturedBC := be.bc
	capturedW, capturedH := w, h

	return LayoutPlan{
		Status:   LayoutFull,
		Consumed: h,
		Blocks: []PlacedBlock{{
			X: x, Y: 0, Width: w, Height: h,
			Tag: "Figure",
			Draw: func(ctx DrawContext, absX, absTopY float64) {
				capturedBC.Draw(ctx.Stream, absX, absTopY-capturedH, capturedW, capturedH)
			},
		}},
	}
}

// MinWidth implements Measurable.
func (be *BarcodeElement) MinWidth() float64 { return be.width }

// MaxWidth implements Measurable.
func (be *BarcodeElement) MaxWidth() float64 { return be.width }
