// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package content provides a builder for PDF content streams — the
// sequences of operators that render text, graphics, and images on a
// page (ISO 32000 §7.8.2).
//
// The [Stream] type accumulates operators into bytes via type-safe
// methods covering:
//
//   - Text rendering: BeginText, SetFont, ShowText, ShowTextArray (§9.4)
//   - Path construction: MoveTo, LineTo, CurveTo, Rectangle, ClosePath (§8.5)
//   - Graphics state: SaveState, RestoreState, SetLineWidth, SetDashPattern (§8.4)
//   - Color: SetFillColorRGB, SetStrokeColorCMYK, SetFillColorGray (§8.6)
//   - XObjects: Do (§8.8)
//   - Marked content: BeginMarkedContent, EndMarkedContent (§14.6)
//
// Convenience methods such as [Stream.Circle], [Stream.Ellipse], and
// [Stream.RoundedRectPerCorner] build complex paths from primitives.
// Call [Stream.Bytes] or [Stream.ToPdfStream] to obtain the result.
package content
