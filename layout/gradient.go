// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"image"
	"image/color"
	"math"
)

// GradientStop defines a color and position within a gradient.
type GradientStop struct {
	Color    Color
	Position float64 // 0-1
}

// RenderLinearGradient creates a gradient image with the given dimensions,
// angle (in degrees, 0 = to top, 90 = to right), and color stops.
// Returns an RGBA image suitable for embedding as a PDF XObject.
func RenderLinearGradient(width, height int, angle float64, stops []GradientStop) *image.RGBA {
	if width <= 0 || height <= 0 || len(stops) < 2 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}

	// Normalize stops: distribute evenly if positions are all zero.
	stops = normalizeStops(stops)

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Convert angle to radians. CSS: 0deg = to top, 90deg = to right.
	rad := angle * math.Pi / 180

	// Direction vector (in screen coords where y increases downward).
	dx := math.Sin(rad)
	dy := -math.Cos(rad)

	// Compute the length of the gradient line through the rectangle.
	// Project all four corners onto the gradient direction and find min/max.
	fw := float64(width)
	fh := float64(height)
	cx := fw / 2
	cy := fh / 2

	corners := [][2]float64{{0, 0}, {fw, 0}, {0, fh}, {fw, fh}}
	minProj := math.MaxFloat64
	maxProj := -math.MaxFloat64
	for _, c := range corners {
		proj := (c[0]-cx)*dx + (c[1]-cy)*dy
		if proj < minProj {
			minProj = proj
		}
		if proj > maxProj {
			maxProj = proj
		}
	}
	gradLen := maxProj - minProj
	if gradLen <= 0 {
		gradLen = 1
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Project pixel onto gradient line.
			proj := (float64(x)-cx)*dx + (float64(y)-cy)*dy
			t := (proj - minProj) / gradLen

			// Clamp.
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}

			c := interpolateStops(stops, t)
			img.SetRGBA(x, y, c)
		}
	}

	return img
}

// RenderRadialGradient creates a radial gradient image from center outward.
// Returns an RGBA image suitable for embedding as a PDF XObject.
func RenderRadialGradient(width, height int, stops []GradientStop) *image.RGBA {
	if width <= 0 || height <= 0 || len(stops) < 2 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}

	stops = normalizeStops(stops)

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	cx := float64(width) / 2
	cy := float64(height) / 2
	// Radius: farthest corner distance (CSS default for "farthest-corner").
	maxR := math.Sqrt(cx*cx + cy*cy)
	if maxR <= 0 {
		maxR = 1
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Sqrt(dx*dx + dy*dy)
			t := dist / maxR
			if t > 1 {
				t = 1
			}

			c := interpolateStops(stops, t)
			img.SetRGBA(x, y, c)
		}
	}

	return img
}

// normalizeStops ensures stops have evenly distributed positions
// when they are all zero (i.e., no explicit positions were provided).
func normalizeStops(stops []GradientStop) []GradientStop {
	out := make([]GradientStop, len(stops))
	copy(out, stops)

	// Check if any position is explicitly set.
	allZero := true
	for i, s := range out {
		if i > 0 && s.Position != 0 {
			allZero = false
			break
		}
	}

	if allZero || (out[0].Position == 0 && out[len(out)-1].Position == 0) {
		// Distribute evenly.
		for i := range out {
			out[i].Position = float64(i) / float64(len(out)-1)
		}
	}

	return out
}

// interpolateStops finds the two surrounding stops for position t and
// linearly interpolates between them.
func interpolateStops(stops []GradientStop, t float64) color.RGBA {
	if t <= stops[0].Position {
		return colorToRGBA(stops[0].Color)
	}
	if t >= stops[len(stops)-1].Position {
		return colorToRGBA(stops[len(stops)-1].Color)
	}

	for i := 1; i < len(stops); i++ {
		if t <= stops[i].Position {
			s0 := stops[i-1]
			s1 := stops[i]
			span := s1.Position - s0.Position
			if span <= 0 {
				return colorToRGBA(s1.Color)
			}
			f := (t - s0.Position) / span
			return lerpColor(colorToRGBA(s0.Color), colorToRGBA(s1.Color), f)
		}
	}

	return colorToRGBA(stops[len(stops)-1].Color)
}

// colorToRGBA converts a layout.Color to an image/color.RGBA.
func colorToRGBA(c Color) color.RGBA {
	return color.RGBA{
		R: uint8(clamp01(c.R) * 255),
		G: uint8(clamp01(c.G) * 255),
		B: uint8(clamp01(c.B) * 255),
		A: 255,
	}
}

// lerpColor linearly interpolates between two RGBA colors.
func lerpColor(a, b color.RGBA, t float64) color.RGBA {
	return color.RGBA{
		R: uint8(float64(a.R)*(1-t) + float64(b.R)*t),
		G: uint8(float64(a.G)*(1-t) + float64(b.G)*t),
		B: uint8(float64(a.B)*(1-t) + float64(b.B)*t),
		A: uint8(float64(a.A)*(1-t) + float64(b.A)*t),
	}
}

// clamp01 clamps a value to [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
