// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "testing"

const fcEps = 0.001

// fcEq compares two floats within the float-context test tolerance.
func fcEq(a, b float64) bool { return approxEq(a, b, fcEps) }

// TestFloatContextSingleLeft: one left float reduces available width and sets
// leftOffset to its width while it is active; returns full width below it.
func TestFloatContextSingleLeft(t *testing.T) {
	var fc floatContext
	shift := fc.place(FloatLeft, 100, 50, 0)
	if !fcEq(shift, 0) {
		t.Errorf("first float shift = %.3f, want 0", shift)
	}

	// At Y=0 (inside the float's extent [0,50]): width reduced, leftOffset=100.
	w, off := fc.available(300, 0)
	if !fcEq(w, 200) {
		t.Errorf("available width at Y=0 = %.3f, want 200", w)
	}
	if !fcEq(off, 100) {
		t.Errorf("leftOffset at Y=0 = %.3f, want 100", off)
	}

	// At Y=49 (still inside): same reduction.
	w, off = fc.available(300, 49)
	if !fcEq(w, 200) || !fcEq(off, 100) {
		t.Errorf("available at Y=49 = (%.3f,%.3f), want (200,100)", w, off)
	}

	// At Y=50 (bottom, exclusive): float no longer occupies — full width.
	w, off = fc.available(300, 50)
	if !fcEq(w, 300) || !fcEq(off, 0) {
		t.Errorf("available at Y=50 = (%.3f,%.3f), want (300,0)", w, off)
	}

	if lb := fc.lowestBottom(); !fcEq(lb, 50) {
		t.Errorf("lowestBottom = %.3f, want 50", lb)
	}
}

// TestFloatContextSingleRight: one right float reduces width but leftOffset
// stays 0 (right floats do not push in-flow content rightward).
func TestFloatContextSingleRight(t *testing.T) {
	var fc floatContext
	fc.place(FloatRight, 120, 40, 0)

	w, off := fc.available(300, 0)
	if !fcEq(w, 180) {
		t.Errorf("available width = %.3f, want 180", w)
	}
	if !fcEq(off, 0) {
		t.Errorf("leftOffset = %.3f, want 0 (right float)", off)
	}

	// Below the float: full width again.
	w, off = fc.available(300, 40)
	if !fcEq(w, 300) || !fcEq(off, 0) {
		t.Errorf("available below right float = (%.3f,%.3f), want (300,0)", w, off)
	}
}

// TestFloatContextTwoSameSideStack: two left floats stack — the second's shift
// equals the first's width, and combined they reduce available width by both.
func TestFloatContextTwoSameSideStack(t *testing.T) {
	var fc floatContext
	s1 := fc.place(FloatLeft, 100, 60, 0)
	if !fcEq(s1, 0) {
		t.Errorf("first left shift = %.3f, want 0", s1)
	}
	s2 := fc.place(FloatLeft, 80, 60, 0)
	if !fcEq(s2, 100) {
		t.Errorf("second left shift = %.3f, want 100 (stacks past first)", s2)
	}

	w, off := fc.available(400, 0)
	if !fcEq(w, 220) {
		t.Errorf("available with two left floats = %.3f, want 220", w)
	}
	if !fcEq(off, 180) {
		t.Errorf("leftOffset with two left floats = %.3f, want 180", off)
	}

	// sameSideWidth reflects both.
	if ssw := fc.sameSideWidth(FloatLeft, 0); !fcEq(ssw, 180) {
		t.Errorf("sameSideWidth(left,0) = %.3f, want 180", ssw)
	}
	if ssw := fc.sameSideWidth(FloatRight, 0); !fcEq(ssw, 0) {
		t.Errorf("sameSideWidth(right,0) = %.3f, want 0", ssw)
	}
}

// TestFloatContextTwoSameSideStackStaggered: a same-side float whose start Y is
// below an earlier float's bottom does not stack against the expired one.
func TestFloatContextStackStaggered(t *testing.T) {
	var fc floatContext
	fc.place(FloatLeft, 100, 30, 0) // occupies [0,30]
	// At Y=40 the first float is expired; the new one starts fresh at shift 0.
	s := fc.place(FloatLeft, 90, 30, 40)
	if !fcEq(s, 0) {
		t.Errorf("staggered left shift = %.3f, want 0 (first float expired)", s)
	}
}

// TestFloatContextOppositeSides: left + right floats reduce width by both, but
// leftOffset only counts the left float.
func TestFloatContextOppositeSides(t *testing.T) {
	var fc floatContext
	fc.place(FloatLeft, 100, 50, 0)
	rs := fc.place(FloatRight, 120, 50, 0)
	if !fcEq(rs, 0) {
		t.Errorf("right shift with a left float present = %.3f, want 0 (opposite side)", rs)
	}

	w, off := fc.available(400, 0)
	if !fcEq(w, 180) {
		t.Errorf("available with L+R floats = %.3f, want 180", w)
	}
	if !fcEq(off, 100) {
		t.Errorf("leftOffset with L+R floats = %.3f, want 100", off)
	}
}

// TestFloatContextClear: clear advances to the bottom of the matching active
// floats only.
func TestFloatContextClear(t *testing.T) {
	var fc floatContext
	fc.place(FloatLeft, 100, 50, 0)  // [0,50]
	fc.place(FloatRight, 120, 80, 0) // [0,80]

	if y := fc.clear("left", 0); !fcEq(y, 50) {
		t.Errorf("clear(left,0) = %.3f, want 50", y)
	}
	if y := fc.clear("right", 0); !fcEq(y, 80) {
		t.Errorf("clear(right,0) = %.3f, want 80", y)
	}
	if y := fc.clear("both", 0); !fcEq(y, 80) {
		t.Errorf("clear(both,0) = %.3f, want 80 (max of both)", y)
	}

	// Clearing at a Y past all floats is a no-op.
	if y := fc.clear("both", 90); !fcEq(y, 90) {
		t.Errorf("clear(both,90) = %.3f, want 90 (no active floats)", y)
	}

	// clear at a Y where only the right float is still active.
	if y := fc.clear("left", 60); !fcEq(y, 60) {
		t.Errorf("clear(left,60) = %.3f, want 60 (left float expired)", y)
	}
	if y := fc.clear("both", 60); !fcEq(y, 80) {
		t.Errorf("clear(both,60) = %.3f, want 80", y)
	}
}

// TestFloatContextDropBelowAll: dropBelowAll returns the max bottom of active
// floats; no-op when none are active.
func TestFloatContextDropBelowAll(t *testing.T) {
	var fc floatContext
	fc.place(FloatLeft, 100, 50, 0)  // [0,50]
	fc.place(FloatRight, 120, 90, 0) // [0,90]

	if y := fc.dropBelowAll(0); !fcEq(y, 90) {
		t.Errorf("dropBelowAll(0) = %.3f, want 90", y)
	}
	// At Y=60 only the right float is active.
	if y := fc.dropBelowAll(60); !fcEq(y, 90) {
		t.Errorf("dropBelowAll(60) = %.3f, want 90", y)
	}
	// Past all floats: no-op.
	if y := fc.dropBelowAll(100); !fcEq(y, 100) {
		t.Errorf("dropBelowAll(100) = %.3f, want 100", y)
	}
}

// TestFloatContextLowestBottom: max bottom across all placed floats; 0 when
// empty; ignores expiry (it spans all floats ever placed).
func TestFloatContextLowestBottom(t *testing.T) {
	var fc floatContext
	if lb := fc.lowestBottom(); !fcEq(lb, 0) {
		t.Errorf("empty lowestBottom = %.3f, want 0", lb)
	}
	fc.place(FloatLeft, 100, 50, 0)   // [0,50]
	fc.place(FloatRight, 120, 30, 40) // [40,70]
	if lb := fc.lowestBottom(); !fcEq(lb, 70) {
		t.Errorf("lowestBottom = %.3f, want 70", lb)
	}
}

// TestFloatContextEmptyAvailable: with no floats, available returns the full
// container width and zero offset at any Y.
func TestFloatContextEmptyAvailable(t *testing.T) {
	var fc floatContext
	w, off := fc.available(250, 0)
	if !fcEq(w, 250) || !fcEq(off, 0) {
		t.Errorf("empty available = (%.3f,%.3f), want (250,0)", w, off)
	}
	w, off = fc.available(250, 999)
	if !fcEq(w, 250) || !fcEq(off, 0) {
		t.Errorf("empty available at high Y = (%.3f,%.3f), want (250,0)", w, off)
	}
}

// TestFloatContextWidthClampsAtZero: floats wider than the container clamp the
// available width at zero rather than going negative.
func TestFloatContextWidthClampsAtZero(t *testing.T) {
	var fc floatContext
	fc.place(FloatLeft, 200, 50, 0)
	fc.place(FloatRight, 200, 50, 0)
	w, _ := fc.available(300, 0)
	if !fcEq(w, 0) {
		t.Errorf("available clamped = %.3f, want 0", w)
	}
}

// TestFloatContextLeftOffsetClampsAtContainer: a left float wider than the
// container clamps both the available width at zero and the leftOffset at the
// container width, so content is never pushed past the right edge.
func TestFloatContextLeftOffsetClampsAtContainer(t *testing.T) {
	var fc floatContext
	fc.place(FloatLeft, 140, 50, 0)
	w, off := fc.available(100, 0)
	if !fcEq(w, 0) {
		t.Errorf("available width = %.3f, want 0 (left float wider than container)", w)
	}
	if off > 100 {
		t.Errorf("leftOffset = %.3f, want <= 100 (container width)", off)
	}
}
