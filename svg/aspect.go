// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import "strings"

// AspectAlign controls how the viewBox is positioned within the viewport
// when preserveAspectRatio performs uniform scaling. The enum values match
// the SVG 1.1 spec (§7.8 preserveAspectRatio attribute).
type AspectAlign int

const (
	// AlignXMidYMid centers the viewBox on both axes. This is the SVG
	// default and is what [DefaultPreserveAspectRatio] returns.
	AlignXMidYMid AspectAlign = iota
	AlignXMinYMin
	AlignXMidYMin
	AlignXMaxYMin
	AlignXMinYMid
	AlignXMaxYMid
	AlignXMinYMax
	AlignXMidYMax
	AlignXMaxYMax
)

// AspectMeetOrSlice selects between the two uniform-scaling strategies.
type AspectMeetOrSlice int

const (
	// ScaleMeet scales uniformly so the entire viewBox fits inside the
	// viewport. Empty bands may appear on one axis when the viewport
	// aspect ratio differs from the viewBox aspect ratio.
	ScaleMeet AspectMeetOrSlice = iota
	// ScaleSlice scales uniformly so the viewport is completely covered
	// by the viewBox; content outside the viewport is not clipped by
	// this package, so callers should clip externally when using slice.
	ScaleSlice
)

// PreserveAspectRatio captures the parsed preserveAspectRatio attribute.
// An SVG element that does not specify the attribute defaults to
// [AlignXMidYMid] + [ScaleMeet] per the SVG 1.1 spec. When None is true
// the renderer uses non-uniform scaling to fill the viewport, ignoring
// Align and MeetOrSlice (this is how the attribute value "none" is
// represented internally).
type PreserveAspectRatio struct {
	Align       AspectAlign
	MeetOrSlice AspectMeetOrSlice
	None        bool
}

// DefaultPreserveAspectRatio returns the SVG 1.1 default: xMidYMid meet.
func DefaultPreserveAspectRatio() PreserveAspectRatio {
	return PreserveAspectRatio{Align: AlignXMidYMid, MeetOrSlice: ScaleMeet}
}

// parsePreserveAspectRatio parses the attribute value into a
// PreserveAspectRatio. Unrecognized tokens fall back to the default
// (xMidYMid meet) rather than failing, matching SVG viewer behavior.
func parsePreserveAspectRatio(s string) PreserveAspectRatio {
	fields := strings.Fields(strings.TrimSpace(s))
	if len(fields) == 0 {
		return DefaultPreserveAspectRatio()
	}

	// First token is either "none" or an align keyword.
	if strings.EqualFold(fields[0], "none") {
		return PreserveAspectRatio{None: true}
	}

	result := DefaultPreserveAspectRatio()
	switch fields[0] {
	case "xMinYMin":
		result.Align = AlignXMinYMin
	case "xMidYMin":
		result.Align = AlignXMidYMin
	case "xMaxYMin":
		result.Align = AlignXMaxYMin
	case "xMinYMid":
		result.Align = AlignXMinYMid
	case "xMidYMid":
		result.Align = AlignXMidYMid
	case "xMaxYMid":
		result.Align = AlignXMaxYMid
	case "xMinYMax":
		result.Align = AlignXMinYMax
	case "xMidYMax":
		result.Align = AlignXMidYMax
	case "xMaxYMax":
		result.Align = AlignXMaxYMax
	}

	// Optional second token: "meet" or "slice" (default meet).
	if len(fields) > 1 {
		if strings.EqualFold(fields[1], "slice") {
			result.MeetOrSlice = ScaleSlice
		}
	}
	return result
}

// computeViewportTransform maps a viewBox (vbW × vbH) into a target
// rectangle (w × h) according to the PreserveAspectRatio rules. It
// returns the uniform scale factors for x and y (equal unless None is
// set) and the translation offset inside the target rectangle.
//
// The scale and offset are intended to be applied as a single affine
// transform: points in the viewBox space are multiplied by (sx, sy)
// and then translated by (tx, ty) inside the target rectangle's local
// frame (i.e. with the target's bottom-left already at the origin).
func computeViewportTransform(par PreserveAspectRatio, w, h, vbW, vbH float64) (sx, sy, tx, ty float64) {
	if par.None || vbW == 0 || vbH == 0 {
		return w / vbW, h / vbH, 0, 0
	}

	scaleX := w / vbW
	scaleY := h / vbH
	var scale float64
	if par.MeetOrSlice == ScaleSlice {
		scale = max(scaleX, scaleY)
	} else {
		scale = min(scaleX, scaleY)
	}

	usedW := scale * vbW
	usedH := scale * vbH

	switch par.Align {
	case AlignXMinYMin, AlignXMinYMid, AlignXMinYMax:
		tx = 0
	case AlignXMaxYMin, AlignXMaxYMid, AlignXMaxYMax:
		tx = w - usedW
	default: // xMid*
		tx = (w - usedW) / 2
	}

	switch par.Align {
	case AlignXMinYMin, AlignXMidYMin, AlignXMaxYMin:
		ty = h - usedH
	case AlignXMinYMax, AlignXMidYMax, AlignXMaxYMax:
		ty = 0
	default: // *YMid
		ty = (h - usedH) / 2
	}

	return scale, scale, tx, ty
}
