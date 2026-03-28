// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package font handles font loading, parsing, metrics, subsetting,
// and PDF embedding for TrueType, OpenType, and the 14 standard PDF
// fonts (ISO 32000 §9.6.2.2).
//
// The [Face] interface abstracts over font formats, providing glyph
// metrics, character mapping, kerning, and raw font bytes needed for
// PDF font descriptors (§9.8). Use [ParseTTF] to parse from bytes or
// [LoadTTF] to load from a file path.
//
// For the 14 standard fonts, pre-defined variables ([Helvetica],
// [TimesRoman], [Courier], etc.) provide [Standard] values with
// hardcoded width tables and metrics from the PDF specification
// (Appendix D). Both [Standard] and [EmbeddedFont] implement the
// [TextMeasurer] interface for layout-engine integration.
//
// Font subsetting via [Subset] produces minimal TrueType programs
// containing only the glyphs used in a document, reducing file size.
package font
