// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"strconv"
	"strings"
)

// Style holds resolved visual properties for an SVG node.
type Style struct {
	Fill             *Color    // nil means default (black for shapes)
	FillOpacity      float64   // 0-1, default 1
	FillRule         string    // "nonzero" or "evenodd"
	Stroke           *Color    // nil means none
	StrokeOpacity    float64   // 0-1, default 1
	StrokeWidth      float64   // default 1
	StrokeLineCap    string    // "butt", "round", "square"
	StrokeLineJoin   string    // "miter", "round", "bevel"
	StrokeMiterLimit float64   // default 4
	StrokeDashArray  []float64 // dash pattern
	StrokeDashOffset float64   // dash offset
	Opacity          float64   // group/element opacity, default 1
	Display          string    // "none" hides the element
	Visibility       string    // "hidden" hides but preserves space
	FontFamily       string
	FontSize         float64
	FontWeight       string // "bold", "normal", etc.
	FontStyle        string // "italic", "normal"
}

// DefaultStyle returns a Style with default values.
func DefaultStyle() Style {
	return Style{
		FillOpacity:      1,
		FillRule:         "nonzero",
		StrokeOpacity:    1,
		StrokeWidth:      1,
		StrokeLineCap:    "butt",
		StrokeLineJoin:   "miter",
		StrokeMiterLimit: 4,
		Opacity:          1,
		Display:          "inline",
		Visibility:       "visible",
		FontFamily:       "sans-serif",
		FontSize:         16,
		FontWeight:       "normal",
		FontStyle:        "normal",
	}
}

// ResolveStyle resolves style from a node's attributes and inline style,
// inheriting from the parent style where appropriate.
// SVG inherits: fill, fill-opacity, fill-rule, stroke, stroke-opacity,
// stroke-width, stroke-linecap, stroke-linejoin, stroke-miterlimit,
// font-family, font-size, font-weight, font-style, visibility.
// Non-inherited (reset to defaults): opacity, display, stroke-dasharray,
// stroke-dashoffset, transform.
func ResolveStyle(node *Node, parent Style) Style {
	// Start with inherited properties from parent.
	s := Style{
		Fill:             parent.Fill,
		FillOpacity:      parent.FillOpacity,
		FillRule:         parent.FillRule,
		Stroke:           parent.Stroke,
		StrokeOpacity:    parent.StrokeOpacity,
		StrokeWidth:      parent.StrokeWidth,
		StrokeLineCap:    parent.StrokeLineCap,
		StrokeLineJoin:   parent.StrokeLineJoin,
		StrokeMiterLimit: parent.StrokeMiterLimit,
		Visibility:       parent.Visibility,
		FontFamily:       parent.FontFamily,
		FontSize:         parent.FontSize,
		FontWeight:       parent.FontWeight,
		FontStyle:        parent.FontStyle,
	}

	// Non-inherited properties get defaults.
	s.Opacity = 1
	s.Display = "inline"
	s.StrokeDashArray = nil
	s.StrokeDashOffset = 0

	if node == nil || node.Attrs == nil {
		return s
	}

	// Apply presentation attributes (lower priority).
	applyProperties(&s, node.Attrs)

	// Apply inline style attribute (higher priority).
	if styleAttr, ok := node.Attrs["style"]; ok {
		props := parseInlineStyle(styleAttr)
		applyProperties(&s, props)
	}

	return s
}

// applyProperties applies a map of SVG property name -> value to a Style.
func applyProperties(s *Style, props map[string]string) {
	for key, val := range props {
		val = strings.TrimSpace(val)
		switch key {
		case "fill":
			if val == "none" {
				// explicit none: no fill
				s.Fill = nil
			} else if c, ok := ParseColor(val); ok {
				cp := c
				s.Fill = &cp
			}
		case "fill-opacity":
			if v, err := strconv.ParseFloat(val, 64); err == nil {
				s.FillOpacity = clamp01(v)
			}
		case "fill-rule":
			if val == "nonzero" || val == "evenodd" {
				s.FillRule = val
			}
		case "stroke":
			if val == "none" {
				s.Stroke = nil
			} else if c, ok := ParseColor(val); ok {
				cp := c
				s.Stroke = &cp
			}
		case "stroke-opacity":
			if v, err := strconv.ParseFloat(val, 64); err == nil {
				s.StrokeOpacity = clamp01(v)
			}
		case "stroke-width":
			if v, err := strconv.ParseFloat(val, 64); err == nil {
				s.StrokeWidth = v
			}
		case "stroke-linecap":
			if val == "butt" || val == "round" || val == "square" {
				s.StrokeLineCap = val
			}
		case "stroke-linejoin":
			if val == "miter" || val == "round" || val == "bevel" {
				s.StrokeLineJoin = val
			}
		case "stroke-miterlimit":
			if v, err := strconv.ParseFloat(val, 64); err == nil {
				s.StrokeMiterLimit = v
			}
		case "stroke-dasharray":
			if val == "none" {
				s.StrokeDashArray = nil
			} else {
				s.StrokeDashArray = parseDashArray(val)
			}
		case "stroke-dashoffset":
			if v, err := strconv.ParseFloat(val, 64); err == nil {
				s.StrokeDashOffset = v
			}
		case "opacity":
			if v, err := strconv.ParseFloat(val, 64); err == nil {
				s.Opacity = clamp01(v)
			}
		case "display":
			s.Display = val
		case "visibility":
			if val == "visible" || val == "hidden" || val == "collapse" {
				s.Visibility = val
			}
		case "font-family":
			s.FontFamily = val
		case "font-size":
			if v, err := strconv.ParseFloat(strings.TrimSuffix(val, "px"), 64); err == nil {
				s.FontSize = v
			}
		case "font-weight":
			s.FontWeight = val
		case "font-style":
			s.FontStyle = val
		}
	}
}

// parseInlineStyle parses a CSS inline style string like "fill:red; stroke-width:2"
// into a property map.
func parseInlineStyle(s string) map[string]string {
	result := make(map[string]string)
	declarations := strings.Split(s, ";")
	for _, decl := range declarations {
		decl = strings.TrimSpace(decl)
		if decl == "" {
			continue
		}
		idx := strings.Index(decl, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(decl[:idx])
		val := strings.TrimSpace(decl[idx+1:])
		if key != "" {
			result[key] = val
		}
	}
	return result
}

// parseDashArray parses a stroke-dasharray value like "5,3,2" or "5 3 2".
func parseDashArray(s string) []float64 {
	s = strings.ReplaceAll(s, ",", " ")
	parts := strings.Fields(s)
	var result []float64
	for _, p := range parts {
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil
		}
		result = append(result, v)
	}
	return result
}
