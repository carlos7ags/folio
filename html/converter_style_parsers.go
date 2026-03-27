// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Standalone CSS value parsers extracted from converter_style.go.
// These functions parse CSS property values (transforms, shadows, angles)
// and do not depend on the converter struct.

package html

import (
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/layout"
)

func parseTransform(val string) []layout.TransformOp {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "none" || val == "" {
		return nil
	}

	var ops []layout.TransformOp
	// Match function calls: name(args)
	for val != "" {
		// Find the next function name.
		parenIdx := strings.Index(val, "(")
		if parenIdx < 0 {
			break
		}
		fname := strings.TrimSpace(val[:parenIdx])
		closeIdx := strings.Index(val[parenIdx:], ")")
		if closeIdx < 0 {
			break
		}
		argsStr := val[parenIdx+1 : parenIdx+closeIdx]
		val = strings.TrimSpace(val[parenIdx+closeIdx+1:])

		// Parse arguments (comma or space separated).
		argsStr = strings.ReplaceAll(argsStr, ",", " ")
		parts := strings.Fields(argsStr)

		switch fname {
		case "rotate":
			if len(parts) >= 1 {
				deg := parseAngle(parts[0])
				ops = append(ops, layout.TransformOp{Type: "rotate", Values: [2]float64{deg, 0}})
			}
		case "scale":
			if len(parts) >= 2 {
				sx := parseNumericVal(parts[0])
				sy := parseNumericVal(parts[1])
				ops = append(ops, layout.TransformOp{Type: "scale", Values: [2]float64{sx, sy}})
			} else if len(parts) >= 1 {
				s := parseNumericVal(parts[0])
				ops = append(ops, layout.TransformOp{Type: "scale", Values: [2]float64{s, s}})
			}
		case "scalex":
			if len(parts) >= 1 {
				s := parseNumericVal(parts[0])
				ops = append(ops, layout.TransformOp{Type: "scale", Values: [2]float64{s, 1}})
			}
		case "scaley":
			if len(parts) >= 1 {
				s := parseNumericVal(parts[0])
				ops = append(ops, layout.TransformOp{Type: "scale", Values: [2]float64{1, s}})
			}
		case "translate":
			if len(parts) >= 2 {
				tx := parseLengthPx(parts[0])
				ty := parseLengthPx(parts[1])
				ops = append(ops, layout.TransformOp{Type: "translate", Values: [2]float64{tx, -ty}})
			} else if len(parts) >= 1 {
				tx := parseLengthPx(parts[0])
				ops = append(ops, layout.TransformOp{Type: "translate", Values: [2]float64{tx, 0}})
			}
		case "translatex":
			if len(parts) >= 1 {
				tx := parseLengthPx(parts[0])
				ops = append(ops, layout.TransformOp{Type: "translate", Values: [2]float64{tx, 0}})
			}
		case "translatey":
			if len(parts) >= 1 {
				ty := parseLengthPx(parts[0])
				ops = append(ops, layout.TransformOp{Type: "translate", Values: [2]float64{0, -ty}})
			}
		case "skew":
			if len(parts) >= 2 {
				ax := parseAngle(parts[0])
				ay := parseAngle(parts[1])
				ops = append(ops, layout.TransformOp{Type: "skewX", Values: [2]float64{ax, 0}})
				ops = append(ops, layout.TransformOp{Type: "skewY", Values: [2]float64{ay, 0}})
			} else if len(parts) >= 1 {
				ax := parseAngle(parts[0])
				ops = append(ops, layout.TransformOp{Type: "skewX", Values: [2]float64{ax, 0}})
			}
		case "skewx":
			if len(parts) >= 1 {
				a := parseAngle(parts[0])
				ops = append(ops, layout.TransformOp{Type: "skewX", Values: [2]float64{a, 0}})
			}
		case "skewy":
			if len(parts) >= 1 {
				a := parseAngle(parts[0])
				ops = append(ops, layout.TransformOp{Type: "skewY", Values: [2]float64{a, 0}})
			}
		}
	}
	return ops
}

// parseAngle parses a CSS angle value like "45deg", "1.5rad", or "100grad".
// Returns degrees.
func parseAngle(s string) float64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "deg") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "deg"), 64)
		return v
	}
	if strings.HasSuffix(s, "rad") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "rad"), 64)
		return v * 180 / 3.14159265358979323846
	}
	if strings.HasSuffix(s, "grad") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "grad"), 64)
		return v * 0.9 // 400grad = 360deg
	}
	if strings.HasSuffix(s, "turn") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "turn"), 64)
		return v * 360
	}
	// Bare number — assume degrees.
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseNumericVal parses a bare numeric value (no unit).
func parseNumericVal(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// parseLengthPx parses a CSS length for use in transforms (px → pt conversion).
func parseLengthPx(s string) float64 {
	l := parseLength(s)
	if l != nil {
		return l.toPoints(0, 12) // default font size context
	}
	// Bare number — treat as px.
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v * 0.75
}

// parseTransformOrigin parses a CSS transform-origin value like
// "center center", "top left", "50% 50%" into point coordinates
// relative to the element's top-left corner.
func parseTransformOrigin(val string, width, height, fontSize float64) (float64, float64) {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "" {
		// Default: center center.
		return width / 2, height / 2
	}

	parts := strings.Fields(val)
	if len(parts) == 1 {
		// Single value: applies to X, Y defaults to center.
		x := resolveOriginComponent(parts[0], width, fontSize)
		return x, height / 2
	}
	x := resolveOriginComponent(parts[0], width, fontSize)
	y := resolveOriginComponent(parts[1], height, fontSize)
	return x, y
}

// resolveOriginComponent resolves a single transform-origin keyword or length
// to a point value relative to the given dimension.
func resolveOriginComponent(s string, dimension, fontSize float64) float64 {
	switch s {
	case "left", "top":
		return 0
	case "center":
		return dimension / 2
	case "right", "bottom":
		return dimension
	default:
		if l := parseLength(s); l != nil {
			return l.toPoints(dimension, fontSize)
		}
		return dimension / 2
	}
}

// parseBoxShadow parses a CSS box-shadow value.
// Format: "offsetX offsetY blur spread color" or "none".
func parseBoxShadow(val string, fontSize float64) *boxShadow {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "none" || val == "" {
		return nil
	}

	// Remove "inset" keyword if present.
	inset := false
	if strings.Contains(val, "inset") {
		inset = true
		val = strings.ReplaceAll(val, "inset", "")
		val = strings.TrimSpace(val)
	}

	parts := strings.Fields(val)
	if len(parts) < 2 {
		return nil
	}

	bs := &boxShadow{Inset: inset}

	// Parse lengths (up to 4) and the remaining token as color.
	var lengths []float64
	var colorToken string
	for _, p := range parts {
		if l := parseLength(p); l != nil {
			lengths = append(lengths, l.toPoints(0, fontSize))
		} else {
			// Accumulate as potential color token.
			if colorToken == "" {
				colorToken = p
			} else {
				colorToken += " " + p
			}
		}
	}

	if len(lengths) >= 2 {
		bs.OffsetX = lengths[0]
		bs.OffsetY = lengths[1]
	}
	if len(lengths) >= 3 {
		bs.Blur = lengths[2]
	}
	if len(lengths) >= 4 {
		bs.Spread = lengths[3]
	}

	if colorToken != "" {
		if c, ok := parseColor(colorToken); ok {
			bs.Color = c
		} else {
			bs.Color = layout.ColorBlack
		}
	} else {
		bs.Color = layout.ColorBlack
	}

	return bs
}

// parseBoxShadows parses a CSS box-shadow value that may contain multiple
// comma-separated shadows. Commas inside function calls (e.g. rgba()) are
// not treated as separators.
func parseBoxShadows(val string, fontSize float64) []boxShadow {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "none" || val == "" {
		return nil
	}

	// Split on commas that are not inside parentheses.
	parts := splitTopLevelCommas(val)
	var shadows []boxShadow
	for _, part := range parts {
		if bs := parseBoxShadow(strings.TrimSpace(part), fontSize); bs != nil {
			shadows = append(shadows, *bs)
		}
	}
	return shadows
}

// splitTopLevelCommas splits a string on commas that are not inside
// parentheses. This handles cases like "rgba(0,0,0,0.5) 2px 2px, 0 0 5px red".
func splitTopLevelCommas(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}
