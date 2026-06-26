// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// floatContext tracks floats placed within a single block container's content
// coordinate space (Y increases downward, origin at the content-box top-left).
// It mirrors the body-flow float bookkeeping in render_plans.go (activeFloat /
// effectiveWidth / consumeFloatHeight) but records each float's absolute
// vertical extent [top, bottom] instead of a remaining height, so queries can
// be answered at any Y. This lets a Div lay out in-flow children beside its own
// floats and grow to enclose them (a pragmatic display:flow-root model).
type floatContext struct {
	floats []placedFloat
}

// placedFloat is one float occupying [top, bottom] on a given side.
type placedFloat struct {
	side   FloatSide
	width  float64
	top    float64
	bottom float64
}

// place registers a float of the given side/width/height starting at atY and
// returns the horizontal shift needed so same-side floats stack instead of
// overlap (left floats accumulate from the left, right floats from the right).
// The shift is the total width of same-side floats still occupying atY.
func (fc *floatContext) place(side FloatSide, width, height, atY float64) float64 {
	shift := fc.sameSideWidth(side, atY)
	fc.floats = append(fc.floats, placedFloat{
		side:   side,
		width:  width,
		top:    atY,
		bottom: atY + height,
	})
	return shift
}

// sameSideWidth returns the combined width of floats on the given side that
// still occupy atY (bottom > atY). Used for the stacking shift.
func (fc *floatContext) sameSideWidth(side FloatSide, atY float64) float64 {
	w := 0.0
	for _, f := range fc.floats {
		if f.side == side && f.bottom > atY {
			w += f.width
		}
	}
	return w
}

// available returns the width remaining for in-flow content at vertical
// position atY, accounting only for floats still occupying space there
// (bottom > atY), plus the total width of such LEFT floats as leftOffset. When
// no float occupies atY it returns (containerWidth, 0).
func (fc *floatContext) available(containerWidth, atY float64) (width, leftOffset float64) {
	w := containerWidth
	off := 0.0
	for _, f := range fc.floats {
		if f.bottom <= atY {
			continue
		}
		w -= f.width
		if f.side == FloatLeft {
			off += f.width
		}
	}
	if w < 0 {
		w = 0
	}
	// Clamp leftOffset so it never pushes content past the container's right
	// edge when left floats are wider than the container.
	if off > containerWidth {
		off = containerWidth
	}
	return w, off
}

// clear returns the Y below the matching floats still active at atY for a CSS
// clear value ("left", "right", "both"); the max bottom of those floats, or
// atY when none match.
func (fc *floatContext) clear(value string, atY float64) float64 {
	newY := atY
	for _, f := range fc.floats {
		if f.bottom <= atY {
			continue
		}
		match := value == "both" ||
			(value == "left" && f.side == FloatLeft) ||
			(value == "right" && f.side == FloatRight)
		if match && f.bottom > newY {
			newY = f.bottom
		}
	}
	return newY
}

// dropBelowAll returns the Y below ALL floats active at atY (max bottom), or
// atY when none are active. Used when an in-flow block cannot fit beside the
// floats.
func (fc *floatContext) dropBelowAll(atY float64) float64 {
	newY := atY
	for _, f := range fc.floats {
		if f.bottom <= atY {
			continue
		}
		if f.bottom > newY {
			newY = f.bottom
		}
	}
	return newY
}

// lowestBottom returns the maximum bottom across ALL placed floats, or 0 when
// empty. Used to grow the container to enclose its floats.
func (fc *floatContext) lowestBottom() float64 {
	low := 0.0
	for _, f := range fc.floats {
		if f.bottom > low {
			low = f.bottom
		}
	}
	return low
}
