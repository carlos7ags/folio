// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"

	folioimage "github.com/carlos7ags/folio/image"
)

// ImageElement is a layout element that places an image in the document flow.
type ImageElement struct {
	img            *folioimage.Image
	width          float64 // explicit width (0 = auto)
	height         float64 // explicit height (0 = auto)
	align          Align
	altText        string // alternative text for accessibility (PDF/UA)
	objectFit      string // "contain", "cover", "fill", "none", "scale-down"
	objectPosition string // e.g. "center", "top left"
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

// SetObjectFit sets the object-fit CSS property for controlling how the image
// fills its content box when explicit width and height are both set.
// Valid values: "contain", "cover", "fill", "none", "scale-down".
func (ie *ImageElement) SetObjectFit(fit string) *ImageElement {
	ie.objectFit = fit
	return ie
}

// SetObjectPosition sets the object-position CSS property for controlling
// image placement within its content box.
func (ie *ImageElement) SetObjectPosition(pos string) *ImageElement {
	ie.objectPosition = pos
	return ie
}

// SetAltText sets alternative text for accessibility (PDF/UA).
// Screen readers use this to describe the image to visually impaired users.
func (ie *ImageElement) SetAltText(text string) *ImageElement {
	ie.altText = text
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

	// When both width and height are explicitly set and object-fit is specified,
	// compute the rendered image dimensions according to the fit mode.
	if w > 0 && h > 0 && ie.objectFit != "" {
		boxW, boxH := w, h
		// Clamp box to available width.
		if boxW > maxWidth {
			boxW = maxWidth
		}
		switch ie.objectFit {
		case "fill":
			// Stretch to fill the box exactly (ignore aspect ratio).
			return boxW, boxH
		case "contain":
			// Scale to fit entirely within the box, preserving aspect ratio.
			iw, ih := boxW, boxW/ar
			if ih > boxH {
				ih = boxH
				iw = ih * ar
			}
			return iw, ih
		case "cover":
			// Scale to fill the entire box, preserving aspect ratio (overflow cropped).
			// For PDF, we render the full cover dimensions since we can't clip
			// without a clip path. The image fills the box completely.
			iw, ih := boxW, boxW/ar
			if ih < boxH {
				ih = boxH
				iw = ih * ar
			}
			return iw, ih
		case "none":
			// No scaling: use natural pixel dimensions (converted to points at 72dpi).
			natW := float64(ie.img.Width()) * 0.75 // px to pt
			natH := float64(ie.img.Height()) * 0.75
			return natW, natH
		case "scale-down":
			// Like contain, but never scale up.
			natW := float64(ie.img.Width()) * 0.75
			natH := float64(ie.img.Height()) * 0.75
			iw, ih := boxW, boxW/ar
			if ih > boxH {
				ih = boxH
				iw = ih * ar
			}
			// If natural size is smaller, use natural size.
			if natW < iw && natH < ih {
				return natW, natH
			}
			return iw, ih
		}
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
			X:       x,
			Y:       0,
			Width:   w,
			Height:  h,
			Tag:     "Figure",
			AltText: ie.altText,
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
