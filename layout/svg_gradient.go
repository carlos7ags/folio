// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"bytes"
	"image"
	"image/png"
	"math"

	folioimage "github.com/carlos7ags/folio/image"
	"github.com/carlos7ags/folio/svg"
)

// rasterizeSVGGradient rasterizes an SVG <linearGradient> or <radialGradient>
// into a PNG-backed folio image that covers the given shape bounding box.
// The returned image is ready to be registered on a PageResult and drawn
// (clipped to the shape) by the svg renderer.
//
// Coordinate systems — SVG gradients are expressed in either
// objectBoundingBox units (default: gradient coords are fractions of the
// shape bbox) or userSpaceOnUse units (gradient coords are absolute SVG
// user-space). For v1 we compute angle from the direction vector alone,
// which is valid for objectBoundingBox and for userSpaceOnUse without a
// gradientTransform. gradientTransform, spreadMethod="reflect", and focal
// radial gradients are documented known gaps.
//
// Returns nil on any failure so the svg renderer can fall back to the
// first stop color.
func rasterizeSVGGradient(node *svg.Node, bbox svg.BBox) *folioimage.Image {
	if bbox.W <= 0 || bbox.H <= 0 {
		return nil
	}

	// Choose a raster resolution. The gradient will be drawn stretched to
	// cover bbox, so we want roughly 2× the display size in pixels for
	// reasonable smoothness without runaway memory. Clamped to a 64..512
	// range so tiny icons still look decent and huge shapes don't blow up.
	pxW := clampInt(int(math.Round(bbox.W*2)), 64, 512)
	pxH := clampInt(int(math.Round(bbox.H*2)), 64, 512)

	stops := parseSVGGradientStops(node)
	if len(stops) < 2 {
		return nil
	}

	var rgba *image.RGBA
	switch node.Tag {
	case "linearGradient":
		info := node.LinearGradient()
		if info == nil {
			return nil
		}
		rgba = RenderLinearGradient(pxW, pxH, linearGradientCSSAngle(info), stops)
	case "radialGradient":
		if node.RadialGradient() == nil {
			return nil
		}
		// For v1 the radial center is assumed to be bbox center and the
		// radius extends to the farthest corner — matching SVG's default
		// cx=0.5, cy=0.5, r=0.5 on objectBoundingBox units. Non-default
		// cx/cy/r and gradientTransform are documented known gaps.
		rgba = RenderRadialGradient(pxW, pxH, stops)
	default:
		return nil
	}
	if rgba == nil {
		return nil
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		return nil
	}
	img, err := folioimage.NewPNG(buf.Bytes())
	if err != nil {
		return nil
	}
	return img
}

// parseSVGGradientStops converts svg.Stop values into layout.GradientStop
// values. Returns nil if the gradient has fewer than two usable stops.
//
// stop-opacity is currently dropped: layout.GradientStop carries only RGB
// and the downstream rasterizer hardcodes full alpha. Supporting
// translucent stops would require widening layout.GradientStop and is a
// documented known gap tied to CSS gradients (which have the same
// limitation).
func parseSVGGradientStops(node *svg.Node) []GradientStop {
	var rawStops []svg.Stop
	if info := node.LinearGradient(); info != nil {
		rawStops = info.Stops
	} else if info := node.RadialGradient(); info != nil {
		rawStops = info.Stops
	}
	if len(rawStops) < 2 {
		return nil
	}
	out := make([]GradientStop, 0, len(rawStops))
	for _, s := range rawStops {
		out = append(out, GradientStop{
			Color:    Color{R: s.Color.R, G: s.Color.G, B: s.Color.B},
			Position: s.Offset,
		})
	}
	return out
}

// linearGradientCSSAngle converts an SVG linearGradient direction
// (x1,y1)→(x2,y2) into the CSS gradient-angle convention used by
// RenderLinearGradient (0° = to top, 90° = to right, 180° = to bottom).
//
// Derivation — SVG y-axis points down, CSS angle conventions are:
//
//	0°   → to top    (dy negative in SVG y-down)
//	90°  → to right  (dx positive)
//	180° → to bottom (dy positive in SVG y-down)
//	270° → to left   (dx negative)
//
// So angle = atan2(dx, -dy), normalized to [0, 360).
func linearGradientCSSAngle(info *svg.LinearGradientInfo) float64 {
	dx := info.X2 - info.X1
	dy := info.Y2 - info.Y1
	if dx == 0 && dy == 0 {
		return 90 // degenerate — default to horizontal
	}
	rad := math.Atan2(dx, -dy)
	deg := rad * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	return deg
}

// clampInt clamps n to [lo, hi].
func clampInt(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}
