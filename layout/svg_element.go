// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"encoding/base64"
	"strings"

	"github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
	"github.com/carlos7ags/folio/svg"
)

// SVGElement is a layout element that renders an SVG graphic in the document flow.
type SVGElement struct {
	svg     *svg.SVG
	width   float64 // explicit width in points (0 = auto from SVG/viewBox)
	height  float64 // explicit height in points (0 = auto)
	align   Align
	altText string // alternative text for accessibility (PDF/UA)
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

// SetAltText sets alternative text for accessibility (PDF/UA).
func (se *SVGElement) SetAltText(text string) *SVGElement {
	se.altText = text
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
			Tag:     "Figure",
			AltText: se.altText,
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
					RegisterImage: func(href string) (string, float64, float64) {
						img := decodeSVGImageHref(href)
						if img == nil {
							return "", 0, 0
						}
						name := registerImage(ctx.Page, img)
						return name, float64(img.Width()), float64(img.Height())
					},
					RegisterGradient: func(node *svg.Node, bbox svg.BBox) string {
						img := rasterizeSVGGradient(node, bbox)
						if img == nil {
							return ""
						}
						return registerImage(ctx.Page, img)
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

// decodeSVGImageHref decodes an SVG <image> href into a registered folio
// image. Only data: URIs are supported here — HTTP URLs and relative file
// paths are intentionally left to higher-level code that has the necessary
// context (base path, fetch policy). Returns nil on any failure so the
// caller can silently skip the element, matching how other SVG paint paths
// fail-soft on malformed input.
func decodeSVGImageHref(href string) *folioimage.Image {
	if !strings.HasPrefix(href, "data:") {
		return nil
	}
	rest := strings.TrimPrefix(href, "data:")
	commaIdx := strings.IndexByte(rest, ',')
	if commaIdx < 0 {
		return nil
	}
	meta := rest[:commaIdx]
	encoded := rest[commaIdx+1:]

	var data []byte
	if strings.Contains(meta, ";base64") {
		// Strip whitespace that sometimes appears in wrapped data URIs.
		encoded = strings.Map(func(r rune) rune {
			if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
				return -1
			}
			return r
		}, encoded)
		dec, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil
		}
		data = dec
	} else {
		data = []byte(encoded)
	}

	// Dispatch by media type with content-sniff fallback.
	switch {
	case strings.Contains(meta, "image/jpeg"), strings.Contains(meta, "image/jpg"):
		if img, err := folioimage.NewJPEG(data); err == nil {
			return img
		}
	case strings.Contains(meta, "image/png"):
		if img, err := folioimage.NewPNG(data); err == nil {
			return img
		}
	case strings.Contains(meta, "image/webp"):
		if img, err := folioimage.NewWebP(data); err == nil {
			return img
		}
	case strings.Contains(meta, "image/gif"):
		if img, err := folioimage.NewGIF(data); err == nil {
			return img
		}
	default:
		// Sniff by magic bytes.
		if len(data) >= 4 && string(data[:4]) == "\x89PNG" {
			if img, err := folioimage.NewPNG(data); err == nil {
				return img
			}
		}
		if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
			if img, err := folioimage.NewJPEG(data); err == nil {
				return img
			}
		}
	}
	return nil
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
