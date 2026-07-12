// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"strings"

	"github.com/carlos7ags/folio/internal/csscolor"
)

// Color represents an RGBA color with components in [0,1].
type Color struct {
	R, G, B, A float64
}

// ParseColor parses an SVG color string.
// Supports: named colors, hex (#rgb, #rgba, #rrggbb, #rrggbbaa), rgb()/rgba()
// (comma and space-separated), hsl()/hsla(), "none", "currentColor".
// Returns ok=false for "none" or unparseable values.
func parseColor(s string) (Color, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Color{}, false
	}

	lower := strings.ToLower(s)

	if lower == "none" {
		return Color{}, false
	}

	if lower == "currentcolor" {
		// currentColor is context-dependent; return black as a sensible default.
		return Color{R: 0, G: 0, B: 0, A: 1}, true
	}

	c, ok := csscolor.Parse(lower)
	if !ok || c.CMYK != nil {
		return Color{}, false
	}
	return Color{R: c.R, G: c.G, B: c.B, A: c.A}, true
}

// clamp01 clamps v to the range [0, 1].
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
