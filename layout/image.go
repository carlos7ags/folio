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
	altText        string  // alternative text for accessibility (PDF/UA)
	objectFit      string  // "contain", "cover", "fill", "none", "scale-down"
	objectPosition string  // e.g. "center", "top left"
	width          float64 // explicit width (0 = auto)
	height         float64 // explicit height (0 = auto)
	cssMaxWidth    float64 // CSS max-width upper bound (0 = unbounded)
	cssMaxHeight   float64 // CSS max-height upper bound (0 = unbounded)
	cssMinWidth    float64 // CSS min-width lower bound (0 = no minimum)
	cssMinHeight   float64 // CSS min-height lower bound (0 = no minimum)
	align          Align
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

// SetMaxSize sets CSS-style max-width and max-height upper bounds in
// PDF points. A value of 0 means "unbounded" on that axis. Bounds
// apply after the explicit / natural / aspect-ratio size is resolved
// — they shrink the rendered image while preserving aspect ratio.
// Pass 0/0 to clear any previous bound.
//
// Per CSS 2.1 §10.7, max-width / max-height apply to replaced
// elements like <img>: they cap the rendered size without forcing
// a specific dimension. They never upscale a smaller intrinsic image.
func (ie *ImageElement) SetMaxSize(maxWidth, maxHeight float64) *ImageElement {
	ie.cssMaxWidth = maxWidth
	ie.cssMaxHeight = maxHeight
	return ie
}

// SetMinSize sets CSS-style min-width and min-height lower bounds in
// PDF points. A value of 0 means "no minimum" on that axis. Bounds
// apply after the explicit / natural / aspect-ratio size is resolved
// and after max-* clamps — min-* wins over max-* when they conflict,
// per CSS 2.1 §10.7.
func (ie *ImageElement) SetMinSize(minWidth, minHeight float64) *ImageElement {
	ie.cssMinWidth = minWidth
	ie.cssMinHeight = minHeight
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

	// Apply CSS max-* / min-* clamps to the explicit (w, h) BEFORE the
	// object-fit branch dispatches. Per CSS Images L3 §3.2 + CSS 2.1
	// §10.7, max-width / max-height constrain the content box first,
	// then object-fit fits the image inside the clamped box. Without
	// this early clamp, `<img width=200 height=200 style="max-width:
	// 50px; object-fit: cover">` would keep the 200×200 box because
	// the object-fit branch returns before the post-resolution clamps
	// at the bottom of this function fire.
	w, h = ie.applyMaxMinClamps(w, h, ar)

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

	// Apply max/min clamps again after the auto-resolution. The
	// earlier call (before the object-fit branch) only mattered when
	// w and h were already set on entry; the auto-auto branch above
	// produced new values that need their own clamping.
	w, h = ie.applyMaxMinClamps(w, h, ar)

	// Clamp to available width — the container is the absolute outer
	// bound for replaced elements per CSS 2.1 §10.4. This wins even
	// over min-width / min-height: a min-height that would push the
	// image past the container width is silently dropped (the
	// width-aspect recompute resets the height accordingly). This
	// matches browser behaviour: a replaced element does not extend
	// past its containing block on the width axis.
	if w > maxWidth {
		w = maxWidth
		h = w / ar
	}

	return w, h
}

// applyMaxMinClamps applies the CSS max-* / min-* constraints to a
// (w, h) pair while preserving aspect ratio. Order matters: max
// clamps run first (shrink) so that a subsequent min clamp can
// upscale past them when the spec's "min wins over max" conflict
// rule kicks in. CSS 2.1 §10.7 specifies the combined rule as
// `min(max-width, max(min-width, used))` — equivalent to applying
// max then min in order on a value already in [min, max].
//
// Each axis clamp aspect-preserves: clamping w recomputes h via
// ar, and vice versa. A subsequent clamp on the other axis may
// fire if the recompute violates that axis's bound.
func (ie *ImageElement) applyMaxMinClamps(w, h, ar float64) (float64, float64) {
	if ie.cssMaxWidth > 0 && w > ie.cssMaxWidth {
		w = ie.cssMaxWidth
		h = w / ar
	}
	if ie.cssMaxHeight > 0 && h > ie.cssMaxHeight {
		h = ie.cssMaxHeight
		w = h * ar
	}
	if ie.cssMinWidth > 0 && w < ie.cssMinWidth {
		w = ie.cssMinWidth
		h = w / ar
	}
	if ie.cssMinHeight > 0 && h < ie.cssMinHeight {
		h = ie.cssMinHeight
		w = h * ar
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
