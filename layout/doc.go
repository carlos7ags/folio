// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package layout implements a high-level, CSS-like element model for
// building PDF pages from composable building blocks.
//
// Elements are positioned through a two-phase process: layout then
// render. During layout, each [Element] is given a [LayoutArea] and
// returns a [LayoutPlan] describing the placed content and any
// overflow that did not fit. During render, [PlacedBlock] draw
// closures emit PDF content-stream operators.
//
// Available elements:
//
//   - [Paragraph]: text with word wrapping, alignment, and rich-text spans
//   - [Div]: block container with padding, borders, and background
//   - [Table]: row/column grid with cell spanning and border collapse
//   - [List]: ordered and unordered lists with nested sub-lists
//   - [ImageElement]: raster image placement with aspect-ratio control
//   - [BarcodeElement]: inline barcode (QR, Code 128, EAN-13)
//   - [Heading]: section heading for auto-generated bookmarks
//   - [Flex]: flexbox-style row/column layout
//   - [Grid]: CSS Grid-style two-dimensional layout
//   - [Columns]: multi-column text flow with balanced filling
//   - [Float]: float-left/right content wrapping
//   - [LineSeparator]: horizontal rule
//   - [AreaBreak]: force a new page or column
//   - [Link]: clickable hyperlink annotation
//   - [TabbedLine]: tabular alignment with tab stops
//   - [SVGElement]: inline SVG rendering
//
// Affine transformations (rotate, scale, translate) are supported via
// [Div.SetTransform] using [TransformOp] values computed by
// [ComputeTransformMatrix].
//
// The [Renderer] drives the layout loop: it feeds elements into pages,
// handles page breaks and overflow, and collects [PageResult] values
// containing content streams and resource references.
package layout
