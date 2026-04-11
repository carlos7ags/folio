// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package content provides a builder for PDF content streams — the
// sequences of operators that render text, graphics, and images on a
// page (ISO 32000 §7.8.2).
//
// The [Stream] type accumulates operators into bytes via type-safe
// methods covering:
//
//   - Text rendering: BeginText, SetFont, ShowText, ShowTextArray, MoveTextWithLeading, SetHorizontalScaling (§9.4)
//   - Path construction: MoveTo, LineTo, CurveTo, Rectangle, ClosePath (§8.5)
//   - Graphics state: SaveState, RestoreState, SetLineWidth, SetDashPattern (§8.4)
//   - Color: SetFillColorRGB, SetStrokeColorCMYK, SetFillColorGray (§8.6)
//   - Shading: ShadingFill (§8.7.4)
//   - XObjects: Do (§8.8)
//   - Marked content: BeginMarkedContent, EndMarkedContent, MarkedPoint, MarkedPointWithID (§14.6)
//
// Convenience methods such as [Stream.Circle], [Stream.Ellipse], and
// [Stream.RoundedRectPerCorner] build complex paths from primitives.
// Call [Stream.Bytes] or [Stream.ToPdfStream] to obtain the result.
//
// # ISO 32000 operators intentionally not implemented
//
// A few operator families from ISO 32000 are deliberately omitted from
// this builder:
//
//   - Inline images (BI / ID / EI, §8.9.7) — inline images require
//     composing an inline image dictionary followed by raw sample data,
//     which does not fit a linear operator-by-operator builder. Use
//     [Stream.Do] with an image XObject instead (see the image package).
//   - Type 3 font glyph metrics (d0 / d1, §9.6.4) — these operators may
//     only appear inside a Type 3 font glyph description, which is out
//     of scope for a general page-content builder.
//   - Compatibility section markers (BX / EX, §14.9) — these delimit
//     regions where unknown operators must be tolerated by consumers;
//     they serve no purpose when producing content.
package content
