// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"
)

// TestGradientStopAlphaOpaqueDefault verifies the backwards-compat contract:
// a GradientStop literal with Alpha=0 (the zero value) renders fully opaque.
// All call sites that predate the Alpha field rely on this.
func TestGradientStopAlphaOpaqueDefault(t *testing.T) {
	stop := GradientStop{Color: RGB(1, 0, 0), Position: 0}
	rgba := stopToRGBA(stop)
	if rgba.A != 255 {
		t.Errorf("GradientStop with Alpha=0 should render opaque (A=255), got A=%d", rgba.A)
	}
	if rgba.R != 255 || rgba.G != 0 || rgba.B != 0 {
		t.Errorf("RGB channels: got (%d,%d,%d), want (255,0,0)", rgba.R, rgba.G, rgba.B)
	}
}

// TestGradientStopAlphaHalfOpacity verifies that an explicit Alpha in (0,1)
// produces the expected image/color.RGBA alpha byte.
func TestGradientStopAlphaHalfOpacity(t *testing.T) {
	stop := GradientStop{Color: RGB(0, 1, 0), Position: 0.5, Alpha: 0.5}
	rgba := stopToRGBA(stop)
	// clamp01(0.5) * 255 = 127.5 → truncates to 127 (uint8 conversion).
	if rgba.A < 126 || rgba.A > 128 {
		t.Errorf("Alpha=0.5 should produce A≈127, got A=%d", rgba.A)
	}
}

// TestGradientInterpolatesAlphaBetweenStops verifies that a gradient between
// an opaque and a semi-transparent stop produces an interpolated alpha at
// the midpoint — the core behavior needed for SVG stop-opacity support.
func TestGradientInterpolatesAlphaBetweenStops(t *testing.T) {
	stops := []GradientStop{
		{Color: RGB(1, 0, 0), Position: 0, Alpha: 0},   // opaque red (default)
		{Color: RGB(0, 0, 1), Position: 1, Alpha: 0.2}, // 20% alpha blue
	}
	// Use a wide strip so the pixel-center projection is close enough to
	// the analytical endpoint for a tight tolerance. The rasterizer
	// projects pixel centers into a centered coordinate space, so pixel
	// x=width-1 sits at t ≈ (width-1)/width rather than exactly 1.
	const w = 200
	img := RenderLinearGradient(w, 1, 90, stops) // 90° = left-to-right

	// Midpoint should have alpha ≈ lerp(255, 51, 0.5) = 153.
	mid := img.RGBAAt(w/2, 0)
	if mid.A < 140 || mid.A > 166 {
		t.Errorf("midpoint alpha: got %d, want ~153", mid.A)
	}

	// Start pixel should stay near fully opaque.
	start := img.RGBAAt(0, 0)
	if start.A < 245 {
		t.Errorf("start alpha: got %d, want >=245", start.A)
	}

	// End pixel at x=w-1 sits at t ≈ (w-1)/w ≈ 0.995, interpolated alpha
	// ≈ lerp(255, 51, 0.995) ≈ 52. Tolerance is wide enough to absorb
	// rounding and the half-pixel projection offset.
	end := img.RGBAAt(w-1, 0)
	if end.A < 45 || end.A > 65 {
		t.Errorf("end alpha: got %d, want ~52", end.A)
	}
}

// TestColorToRGBARemainsOpaque is a regression guard for the retained
// colorToRGBA helper, which solid-color paths (not gradients) still use.
func TestColorToRGBARemainsOpaque(t *testing.T) {
	c := RGB(0.5, 0.5, 0.5)
	rgba := colorToRGBA(c)
	if rgba.A != 255 {
		t.Errorf("colorToRGBA should always be opaque, got A=%d", rgba.A)
	}
}
