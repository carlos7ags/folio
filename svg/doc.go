// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package svg parses SVG markup and renders it into PDF content streams
// as native vector graphics — no rasterization is involved.
//
// # Supported elements
//
// Shape and container elements: svg, g, defs, use, path, rect, circle,
// ellipse, line, polyline, polygon. Path data supports the full SVG 1.1
// command set (M/L/H/V/C/S/Q/T/A/Z and their relative variants). Text
// elements (text, tspan) support single x/y positioning, dx/dy offsets,
// text-anchor (start/middle/end), and dominant-baseline. Gradients
// (linearGradient, radialGradient) are parsed and handed to a caller-
// supplied rasterization callback via [RenderOptions.RegisterGradient].
// Images are parsed (href / xlink:href) and handed to a caller-supplied
// decoder via [RenderOptions.RegisterImage]; this package never fetches
// external URLs on its own.
//
// Style attributes (fill, stroke, opacity, fill-opacity, stroke-opacity,
// stroke-width, stroke-linecap, stroke-linejoin, stroke-dasharray,
// stroke-miterlimit, font-family, font-size, font-weight, font-style,
// transform) are mapped to the equivalent PDF content-stream operators.
// CSS inheritance uses a prototype chain built from presentation
// attributes and inline style="..." declarations.
//
// Coordinate mapping honors preserveAspectRatio. The default when the
// attribute is absent is xMidYMid meet (spec default); values "none",
// all nine align keywords, and the meet/slice selector are all
// recognized. See [PreserveAspectRatio] for the parsed representation.
//
// # Unsupported
//
// The following SVG 1.1 elements are silently skipped by the renderer:
// style (no CSS parsing), symbol, marker, pattern, clipPath, mask,
// filter, switch, foreignObject. Per-character text positioning via
// x/y/dx/dy attribute lists is not supported; text rotate, textPath,
// and lengthAdjust are not supported. Stroke gradients collapse to the
// first stop color. Slice mode under preserveAspectRatio scales content
// correctly but this package does not emit a PDF clip path, so callers
// that need strict viewport clipping with slice should clip externally.
//
// # Usage
//
//	doc, _ := svg.Parse(svgString)
//	doc.Draw(stream, x, y, width, height)
package svg
