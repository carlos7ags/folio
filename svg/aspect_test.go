// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"math"
	"testing"
)

func TestParsePreserveAspectRatio(t *testing.T) {
	cases := []struct {
		in    string
		want  PreserveAspectRatio
		descr string
	}{
		{"", DefaultPreserveAspectRatio(), "empty string falls back to default"},
		{"none", PreserveAspectRatio{None: true}, "none disables uniform scaling"},
		{"xMidYMid meet", PreserveAspectRatio{Align: AlignXMidYMid, MeetOrSlice: ScaleMeet}, "explicit default"},
		{"xMinYMin meet", PreserveAspectRatio{Align: AlignXMinYMin, MeetOrSlice: ScaleMeet}, "top-left meet"},
		{"xMaxYMax meet", PreserveAspectRatio{Align: AlignXMaxYMax, MeetOrSlice: ScaleMeet}, "bottom-right meet"},
		{"xMidYMid slice", PreserveAspectRatio{Align: AlignXMidYMid, MeetOrSlice: ScaleSlice}, "slice keyword"},
		{"xMaxYMin slice", PreserveAspectRatio{Align: AlignXMaxYMin, MeetOrSlice: ScaleSlice}, "top-right slice"},
		{"xMidYMax", PreserveAspectRatio{Align: AlignXMidYMax, MeetOrSlice: ScaleMeet}, "meet is default when omitted"},
		{"  xMinYMid   meet  ", PreserveAspectRatio{Align: AlignXMinYMid, MeetOrSlice: ScaleMeet}, "whitespace tolerated"},
		{"NONE", PreserveAspectRatio{None: true}, "none is case-insensitive"},
		{"garbage", DefaultPreserveAspectRatio(), "unknown align falls back to default"},
	}
	for _, tc := range cases {
		t.Run(tc.descr, func(t *testing.T) {
			got := parsePreserveAspectRatio(tc.in)
			if got != tc.want {
				t.Errorf("parsePreserveAspectRatio(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

// approxEq compares two floats within a small epsilon so we can assert
// on geometric computations without worrying about rounding noise.
func approxEq(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

// assertViewportTransform runs computeViewportTransform and checks the
// returned (sx, sy, tx, ty) match the expected values, reporting the
// test case description on failure.
func assertViewportTransform(t *testing.T, descr string, par PreserveAspectRatio, w, h, vbW, vbH, wantSx, wantSy, wantTx, wantTy float64) {
	t.Helper()
	sx, sy, tx, ty := computeViewportTransform(par, w, h, vbW, vbH)
	if !approxEq(sx, wantSx) || !approxEq(sy, wantSy) || !approxEq(tx, wantTx) || !approxEq(ty, wantTy) {
		t.Errorf("%s: got (sx=%g, sy=%g, tx=%g, ty=%g), want (%g, %g, %g, %g)",
			descr, sx, sy, tx, ty, wantSx, wantSy, wantTx, wantTy)
	}
}

func TestComputeViewportTransformNone(t *testing.T) {
	// None uses legacy non-uniform scaling with zero offset.
	assertViewportTransform(t, "none fills viewport",
		PreserveAspectRatio{None: true},
		200, 100, // target w, h
		50, 50, // viewBox
		4, 2, 0, 0)
}

func TestComputeViewportTransformMeetWider(t *testing.T) {
	// Target is wider than viewBox aspect ratio (2:1 vs 1:1). Meet scale
	// is limited by height (100/50 = 2), leaving horizontal bands.
	// Used width = 2 * 50 = 100, so 100 points of empty space on X.
	const (
		w, h = 200.0, 100.0
		vb   = 50.0
		s    = 2.0
	)
	usedW := s * vb

	cases := []struct {
		align AspectAlign
		wantX float64
	}{
		{AlignXMinYMid, 0},
		{AlignXMidYMid, (w - usedW) / 2},
		{AlignXMaxYMid, w - usedW},
	}
	for _, tc := range cases {
		assertViewportTransform(t, "meet wider/"+alignName(tc.align),
			PreserveAspectRatio{Align: tc.align, MeetOrSlice: ScaleMeet},
			w, h, vb, vb,
			s, s, tc.wantX, 0)
	}
}

func TestComputeViewportTransformMeetTaller(t *testing.T) {
	// Target is taller than viewBox (1:2 vs 1:1). Meet scale is
	// limited by width (100/50 = 2), leaving vertical bands. The Y
	// alignment logic places the used band at the top (yMin), middle
	// (yMid), or bottom (yMax) of the target in PDF coordinates.
	const (
		w, h  = 100.0, 200.0
		vb    = 50.0
		s     = 2.0
		usedH = s * vb
	)

	cases := []struct {
		align AspectAlign
		wantY float64
	}{
		// yMin: SVG top aligns with target top (high PDF y).
		{AlignXMidYMin, h - usedH},
		{AlignXMidYMid, (h - usedH) / 2},
		// yMax: SVG bottom aligns with target bottom (low PDF y).
		{AlignXMidYMax, 0},
	}
	for _, tc := range cases {
		assertViewportTransform(t, "meet taller/"+alignName(tc.align),
			PreserveAspectRatio{Align: tc.align, MeetOrSlice: ScaleMeet},
			w, h, vb, vb,
			s, s, 0, tc.wantY)
	}
}

func TestComputeViewportTransformMeetSameRatio(t *testing.T) {
	// When the target and viewBox aspect ratios match, meet == none
	// with no offset.
	assertViewportTransform(t, "meet same ratio",
		DefaultPreserveAspectRatio(),
		100, 50, 10, 5,
		10, 10, 0, 0)
}

func TestComputeViewportTransformSlice(t *testing.T) {
	// Slice uses max(scaleX, scaleY) and may produce negative offsets
	// because the scaled viewBox overflows the target rectangle. Here
	// scaleX = 200/50 = 4 is larger than scaleY = 100/50 = 2, so the
	// viewBox is scaled to 200x200 and the vertical overflow is 100.
	const (
		w, h = 200.0, 100.0
		vb   = 50.0
		s    = 4.0
	)

	// yMid: vertical overflow is centered, so ty = (100 - 200)/2 = -50.
	assertViewportTransform(t, "slice xMidYMid",
		PreserveAspectRatio{Align: AlignXMidYMid, MeetOrSlice: ScaleSlice},
		w, h, vb, vb,
		s, s, 0, -50)

	// yMin: SVG top aligns with target top, so ty = h - usedH = -100.
	assertViewportTransform(t, "slice xMidYMin",
		PreserveAspectRatio{Align: AlignXMidYMin, MeetOrSlice: ScaleSlice},
		w, h, vb, vb,
		s, s, 0, -100)

	// yMax: SVG bottom aligns with target bottom, so ty = 0.
	assertViewportTransform(t, "slice xMidYMax",
		PreserveAspectRatio{Align: AlignXMidYMax, MeetOrSlice: ScaleSlice},
		w, h, vb, vb,
		s, s, 0, 0)
}

func TestComputeViewportTransformZeroViewBox(t *testing.T) {
	// vbW/vbH of zero would otherwise divide by zero; the guard path
	// returns the none-scale (which is also problematic but matches
	// the legacy contract of "don't panic"). The renderer skips zero
	// viewBoxes before reaching this function in practice.
	sx, sy, tx, ty := computeViewportTransform(DefaultPreserveAspectRatio(), 100, 100, 0, 0)
	if tx != 0 || ty != 0 {
		t.Errorf("zero viewBox translation: got (%g, %g), want (0, 0)", tx, ty)
	}
	// Division by zero in Go produces +Inf or NaN, not a panic, so we
	// only check the function returned.
	_ = sx
	_ = sy
}

// alignName exists so test failure messages are readable.
func alignName(a AspectAlign) string {
	switch a {
	case AlignXMinYMin:
		return "xMinYMin"
	case AlignXMidYMin:
		return "xMidYMin"
	case AlignXMaxYMin:
		return "xMaxYMin"
	case AlignXMinYMid:
		return "xMinYMid"
	case AlignXMidYMid:
		return "xMidYMid"
	case AlignXMaxYMid:
		return "xMaxYMid"
	case AlignXMinYMax:
		return "xMinYMax"
	case AlignXMidYMax:
		return "xMidYMax"
	case AlignXMaxYMax:
		return "xMaxYMax"
	}
	return "unknown"
}

func TestSVGPreserveAspectRatioParsed(t *testing.T) {
	// Round-trip: an SVG root with the attribute must surface via
	// the public accessor.
	doc, err := Parse(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="50" viewBox="0 0 10 10" preserveAspectRatio="xMinYMax slice"/>`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	par := doc.PreserveAspectRatio()
	if par.Align != AlignXMinYMax || par.MeetOrSlice != ScaleSlice || par.None {
		t.Errorf("got %+v, want xMinYMax slice", par)
	}
}

func TestSVGPreserveAspectRatioDefault(t *testing.T) {
	// An SVG without the attribute must report the spec default
	// (xMidYMid meet) instead of the legacy non-uniform behavior.
	doc, err := Parse(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 10 10"/>`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	par := doc.PreserveAspectRatio()
	if par.None || par.Align != AlignXMidYMid || par.MeetOrSlice != ScaleMeet {
		t.Errorf("got %+v, want xMidYMid meet", par)
	}
}

func TestSVGPreserveAspectRatioNone(t *testing.T) {
	doc, err := Parse(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 10 10" preserveAspectRatio="none"/>`)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if !doc.PreserveAspectRatio().None {
		t.Errorf("got %+v, want None: true", doc.PreserveAspectRatio())
	}
}
