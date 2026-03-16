// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/svg"
)

// SVGElement is a layout element that renders an SVG graphic in the document flow.
type SVGElement struct {
	svg    *svg.SVG
	width  float64 // explicit width in points (0 = auto from SVG/viewBox)
	height float64 // explicit height in points (0 = auto)
	align  Align
}

// NewSVGElement creates a layout element from a parsed SVG.
// By default the element uses the SVG's intrinsic dimensions and left alignment.
func NewSVGElement(s *svg.SVG) *SVGElement {
	return &SVGElement{
		svg:   s,
		align: AlignLeft,
	}
}

// SetSize sets explicit display dimensions. If either is 0 the value is
// computed from the SVG aspect ratio.
func (se *SVGElement) SetSize(width, height float64) *SVGElement {
	se.width = width
	se.height = height
	return se
}

// SetAlign sets horizontal alignment.
func (se *SVGElement) SetAlign(a Align) *SVGElement {
	se.align = a
	return se
}

// resolveSize computes the rendered width and height within maxWidth.
func (se *SVGElement) resolveSize(maxWidth float64) (float64, float64) {
	if se.svg == nil {
		return 0, 0
	}

	w := se.width
	h := se.height
	ar := se.svg.AspectRatio()

	if w == 0 && h == 0 {
		// Use SVG intrinsic dimensions.
		w = se.svg.Width()
		h = se.svg.Height()
		if w == 0 && h == 0 {
			// No dimensions at all — use available width, square fallback.
			w = maxWidth
			if ar > 0 {
				h = w / ar
			} else {
				h = w
			}
		}
	}

	if w == 0 && h > 0 {
		if ar > 0 {
			w = h * ar
		} else {
			w = h
		}
	} else if h == 0 && w > 0 {
		if ar > 0 {
			h = w / ar
		} else {
			h = w
		}
	}

	// Clamp to available width.
	if w > maxWidth {
		ratio := maxWidth / w
		w = maxWidth
		h *= ratio
	}

	return w, h
}

// PlanLayout implements Element. An SVG never splits — FULL or NOTHING.
func (se *SVGElement) PlanLayout(area LayoutArea) LayoutPlan {
	w, h := se.resolveSize(area.Width)

	if h > area.Height && area.Height > 0 {
		return LayoutPlan{Status: LayoutNothing}
	}

	x := 0.0
	switch se.align {
	case AlignCenter:
		x = (area.Width - w) / 2
	case AlignRight:
		x = area.Width - w
	}

	capturedSVG := se.svg
	capturedW, capturedH := w, h

	return LayoutPlan{
		Status:   LayoutFull,
		Consumed: h,
		Blocks: []PlacedBlock{{
			X: x, Y: 0, Width: w, Height: h,
			Tag: "Figure",
			Draw: func(ctx DrawContext, absX, absTopY float64) {
				opts := svg.RenderOptions{
					RegisterOpacity: func(opacity float64) string {
						return registerOpacity(ctx.Page, opacity)
					},
					RegisterFont: func(family, weight, style string, size float64) string {
						f := resolveSVGFont(family, weight, style)
						return registerFontStandard(ctx.Page, f)
					},
					MeasureText: func(family, weight, style string, size float64, text string) float64 {
						f := resolveSVGFont(family, weight, style)
						return f.MeasureString(text, size)
					},
				}
				capturedSVG.DrawWithOptions(ctx.Stream, absX, absTopY-capturedH, capturedW, capturedH, opts)
			},
		}},
	}
}

// MinWidth implements Measurable. Returns the explicit width or SVG intrinsic width.
func (se *SVGElement) MinWidth() float64 {
	if se.width > 0 {
		return se.width
	}
	if se.svg == nil {
		return 1
	}
	w := se.svg.Width()
	if w > 0 {
		return w
	}
	return 1 // minimum 1pt
}

// MaxWidth implements Measurable. Returns the explicit width or SVG intrinsic width.
func (se *SVGElement) MaxWidth() float64 {
	if se.width > 0 {
		return se.width
	}
	if se.svg == nil {
		return 1
	}
	w := se.svg.Width()
	if w > 0 {
		return w
	}
	return 1
}

// resolveSVGFont maps SVG font-family, font-weight, and font-style to a
// standard PDF font. This keeps SVG text rendering simple without requiring
// embedded font support.
func resolveSVGFont(family, weight, style string) *font.Standard {
	family = strings.ToLower(strings.TrimSpace(family))
	isBold := weight == "bold" || weight == "700" || weight == "800" || weight == "900"
	isItalic := style == "italic" || style == "oblique"

	switch {
	case strings.Contains(family, "courier") || strings.Contains(family, "monospace"):
		switch {
		case isBold && isItalic:
			return font.CourierBoldOblique
		case isBold:
			return font.CourierBold
		case isItalic:
			return font.CourierOblique
		default:
			return font.Courier
		}
	case strings.Contains(family, "times") || strings.Contains(family, "serif"):
		switch {
		case isBold && isItalic:
			return font.TimesBoldItalic
		case isBold:
			return font.TimesBold
		case isItalic:
			return font.TimesItalic
		default:
			return font.TimesRoman
		}
	default:
		// Default to Helvetica (sans-serif).
		switch {
		case isBold && isItalic:
			return font.HelveticaBoldOblique
		case isBold:
			return font.HelveticaBold
		case isItalic:
			return font.HelveticaOblique
		default:
			return font.Helvetica
		}
	}
}
