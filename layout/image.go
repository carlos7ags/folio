// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"

	"github.com/carlos7ags/folio/content"
	folioimage "github.com/carlos7ags/folio/image"
)

// ImageElement is a layout element that places an image in the document flow.
type ImageElement struct {
	img    *folioimage.Image
	width  float64 // explicit width (0 = auto)
	height float64 // explicit height (0 = auto)
	align  Align
}

// NewImageElement creates a layout element from an Image.
// By default, the image scales to fit the available width
// while preserving aspect ratio.
func NewImageElement(img *folioimage.Image) *ImageElement {
	return &ImageElement{
		img:   img,
		align: AlignLeft,
	}
}

// SetSize sets explicit width and height in PDF points.
// If either is 0, it is calculated from the other preserving aspect ratio.
func (ie *ImageElement) SetSize(width, height float64) *ImageElement {
	ie.width = width
	ie.height = height
	return ie
}

// SetAlign sets horizontal alignment of the image.
func (ie *ImageElement) SetAlign(a Align) *ImageElement {
	ie.align = a
	return ie
}

// Layout implements Element. Returns a single Line representing the image.
func (ie *ImageElement) Layout(maxWidth float64) []Line {
	w, h := ie.resolveSize(maxWidth)

	return []Line{{
		Width:    w,
		Height:   h,
		Align:    ie.align,
		IsLast:   true,
		imageRef: &imageLayoutRef{img: ie.img, width: w, height: h},
	}}
}

// resolveSize computes the rendered width and height.
func (ie *ImageElement) resolveSize(maxWidth float64) (float64, float64) {
	if ie.img == nil {
		return 0, 0
	}

	w := ie.width
	h := ie.height
	ar := ie.img.AspectRatio()

	// Guard against zero or negative aspect ratio to prevent division by zero.
	if ar <= 0 {
		ar = 1
	}

	if w == 0 && h == 0 {
		// Scale to fit available width.
		w = maxWidth
		h = w / ar
	} else if w == 0 {
		w = h * ar
	} else if h == 0 {
		h = w / ar
	}

	// Clamp to available width.
	if w > maxWidth {
		w = maxWidth
		h = w / ar
	}

	return w, h
}

// imageLayoutRef holds data for the renderer to emit an image.
type imageLayoutRef struct {
	img    *folioimage.Image
	width  float64
	height float64
}

// renderImage draws the image into the content stream at (x, y).
// y is the top of the image area (PDF top-down layout coords).
func (ref *imageLayoutRef) renderImage(stream *content.Stream, resName string, x, topY float64) {
	// PDF image placement: cm(width, 0, 0, height, x, bottomY)
	bottomY := topY - ref.height
	stream.SaveState()
	stream.ConcatMatrix(ref.width, 0, 0, ref.height, x, bottomY)
	stream.Do(resName)
	stream.RestoreState()
}

// imageResName generates a resource name for images on a page.
func imageResName(index int) string {
	return fmt.Sprintf("Im%d", index+1)
}

// MinWidth implements Measurable. Returns the explicit width or 0 (auto).
func (ie *ImageElement) MinWidth() float64 {
	if ie.width > 0 {
		return ie.width
	}
	return 1 // minimum 1pt
}

// MaxWidth implements Measurable. Returns the explicit width or natural pixel width.
func (ie *ImageElement) MaxWidth() float64 {
	if ie.width > 0 {
		return ie.width
	}
	if ie.img == nil {
		return 1
	}
	return float64(ie.img.Width())
}

// PlanLayout implements Element. An image never splits — FULL or NOTHING.
func (ie *ImageElement) PlanLayout(area LayoutArea) LayoutPlan {
	w, h := ie.resolveSize(area.Width)

	if h > area.Height && area.Height > 0 {
		return LayoutPlan{Status: LayoutNothing}
	}

	x := 0.0
	switch ie.align {
	case AlignCenter:
		x = (area.Width - w) / 2
	case AlignRight:
		x = area.Width - w
	}

	capturedImg := ie.img
	capturedW, capturedH := w, h
	return LayoutPlan{
		Status:   LayoutFull,
		Consumed: h,
		Blocks: []PlacedBlock{{
			X:      x,
			Y:      0,
			Width:  w,
			Height: h,
			Tag:    "Figure",
			Draw: func(ctx DrawContext, absX, absTopY float64) {
				resName := registerImage(ctx.Page, capturedImg)
				bottomY := absTopY - capturedH
				ctx.Stream.SaveState()
				ctx.Stream.ConcatMatrix(capturedW, 0, 0, capturedH, absX, bottomY)
				ctx.Stream.Do(resName)
				ctx.Stream.RestoreState()
			},
		}},
	}
}
