// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"strconv"
	"strings"
)

// BBox is an axis-aligned bounding box in SVG local user-space coordinates.
// Used by the gradient pipeline to size and position a rasterized gradient
// fill for a shape.
type BBox struct {
	X, Y, W, H float64
}

// Stop represents a single <stop> element in a gradient definition.
// Exported so RegisterGradient callbacks can drive an external rasterizer
// without reparsing SVG syntax themselves.
type Stop struct {
	// Offset is the stop position in [0, 1].
	Offset float64
	// Color is the resolved stop-color with stop-opacity baked into A.
	Color Color
}

// LinearGradientInfo is the parsed form of a <linearGradient> element.
// The X1/Y1/X2/Y2 endpoints are interpreted according to Units.
type LinearGradientInfo struct {
	X1, Y1, X2, Y2 float64
	// Units is "objectBoundingBox" (default) or "userSpaceOnUse".
	Units string
	Stops []Stop
}

// RadialGradientInfo is the parsed form of a <radialGradient> element.
type RadialGradientInfo struct {
	CX, CY, R float64
	// Units is "objectBoundingBox" (default) or "userSpaceOnUse".
	Units string
	Stops []Stop
}

// LinearGradient returns the parsed linear gradient definition rooted at n,
// or nil if n is not a <linearGradient> element or has no usable stops.
func (n *Node) LinearGradient() *LinearGradientInfo {
	if n == nil || n.Tag != "linearGradient" {
		return nil
	}
	info := &LinearGradientInfo{
		// SVG defaults for linear gradients with objectBoundingBox units.
		X1:    0,
		Y1:    0,
		X2:    1,
		Y2:    0,
		Units: "objectBoundingBox",
	}
	if v, ok := n.Attrs["gradientUnits"]; ok {
		info.Units = strings.TrimSpace(v)
	}
	info.X1 = parseGradientCoord(n.Attrs["x1"], info.X1)
	info.Y1 = parseGradientCoord(n.Attrs["y1"], info.Y1)
	info.X2 = parseGradientCoord(n.Attrs["x2"], info.X2)
	info.Y2 = parseGradientCoord(n.Attrs["y2"], info.Y2)
	info.Stops = parseStops(n)
	if len(info.Stops) < 2 {
		return nil
	}
	return info
}

// RadialGradient returns the parsed radial gradient definition rooted at n,
// or nil if n is not a <radialGradient> element or has no usable stops.
func (n *Node) RadialGradient() *RadialGradientInfo {
	if n == nil || n.Tag != "radialGradient" {
		return nil
	}
	info := &RadialGradientInfo{
		// SVG defaults for radial gradients with objectBoundingBox units.
		CX:    0.5,
		CY:    0.5,
		R:     0.5,
		Units: "objectBoundingBox",
	}
	if v, ok := n.Attrs["gradientUnits"]; ok {
		info.Units = strings.TrimSpace(v)
	}
	info.CX = parseGradientCoord(n.Attrs["cx"], info.CX)
	info.CY = parseGradientCoord(n.Attrs["cy"], info.CY)
	info.R = parseGradientCoord(n.Attrs["r"], info.R)
	info.Stops = parseStops(n)
	if len(info.Stops) < 2 {
		return nil
	}
	return info
}

// parseGradientCoord parses a gradient coordinate value which may be a plain
// number, a percentage ("50%"), or empty. Returns def for empty/invalid.
// Percentages are returned as fractions in [0, 1].
func parseGradientCoord(s string, def float64) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	if strings.HasSuffix(s, "%") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64); err == nil {
			return v / 100
		}
		return def
	}
	// Strip unit suffix if present.
	s = strings.TrimSuffix(s, "px")
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return v
	}
	return def
}

// parseStops walks the child <stop> elements of a gradient node and returns
// them as Stop values with stop-color, stop-opacity, and offset resolved.
// Offsets default to evenly distributed positions if none are specified, so
// a minimal <stop stop-color="red"/> / <stop stop-color="blue"/> pair still
// interpolates correctly.
func parseStops(n *Node) []Stop {
	var stops []Stop
	for _, child := range n.Children {
		if child.Tag != "stop" {
			continue
		}
		stop := Stop{}
		// Offset (numeric or percentage). -1 is a sentinel for "not set",
		// normalized below once the total stop count is known.
		stop.Offset = parseGradientCoord(child.Attrs["offset"], -1)

		// Color from attribute or inline style.
		colorStr := child.Attrs["stop-color"]
		opacityStr := child.Attrs["stop-opacity"]
		if styleAttr, ok := child.Attrs["style"]; ok {
			props := parseInlineStyle(styleAttr)
			if colorStr == "" {
				colorStr = props["stop-color"]
			}
			if opacityStr == "" {
				opacityStr = props["stop-opacity"]
			}
		}
		// Default stop color per SVG spec is black.
		if colorStr == "" {
			colorStr = "black"
		}
		c, ok := parseColor(colorStr)
		if !ok {
			continue
		}
		c.A = 1
		if opacityStr != "" {
			if v, err := strconv.ParseFloat(opacityStr, 64); err == nil {
				c.A = clamp01(v)
			}
		}
		stop.Color = c
		stops = append(stops, stop)
	}
	if len(stops) == 0 {
		return nil
	}
	// Normalize missing offsets: spread across [0, 1] by index.
	unset := true
	for _, s := range stops {
		if s.Offset >= 0 {
			unset = false
			break
		}
	}
	if unset {
		if len(stops) == 1 {
			stops[0].Offset = 0
		} else {
			for i := range stops {
				stops[i].Offset = float64(i) / float64(len(stops)-1)
			}
		}
	} else {
		// Replace any -1 offsets with neighboring interpolation: simplest
		// fallback is to snap to the previous stop's offset (monotonic
		// non-decreasing). This handles partially-specified stop lists.
		prev := 0.0
		for i := range stops {
			if stops[i].Offset < 0 {
				stops[i].Offset = prev
			}
			prev = stops[i].Offset
		}
	}
	return stops
}
