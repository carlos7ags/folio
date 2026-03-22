// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package svg parses SVG markup and renders it into PDF content streams
// as native vector graphics — no rasterization is involved.
//
// Supported SVG elements include path, rect, circle, ellipse, line,
// polyline, polygon, text, use, g, defs, linearGradient, and radialGradient.
// Style attributes (fill, stroke, opacity, transform) are mapped to
// the equivalent PDF content-stream operators.
//
// Usage:
//
//	doc, _ := svg.Parse(svgString)
//	doc.Draw(stream, x, y, width, height)
package svg
