// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

// Face is the abstraction over a parsed font file. It provides the
// data needed to embed a font in a PDF: glyph metrics, character
// mapping, and the raw font bytes.
//
// The current implementation uses golang.org/x/image/font/sfnt under
// the hood, but this interface is designed so we can replace sfnt
// with our own parser later without changing any calling code.
type Face interface {
	// PostScriptName returns the font's PostScript name (used as /BaseFont).
	PostScriptName() string

	// UnitsPerEm returns the font's design units per em.
	UnitsPerEm() int

	// GlyphIndex returns the glyph ID for a Unicode rune.
	// Returns 0 (the .notdef glyph) if the rune is not in the font.
	GlyphIndex(r rune) uint16

	// GlyphAdvance returns the advance width of a glyph in font design units.
	GlyphAdvance(glyphID uint16) int

	// Ascent returns the typographic ascent in font design units.
	Ascent() int

	// Descent returns the typographic descent in font design units (negative).
	Descent() int

	// BBox returns the font's bounding box in font design units:
	// [xMin, yMin, xMax, yMax].
	BBox() [4]int

	// ItalicAngle returns the italic angle in degrees (0 for upright fonts).
	// Parsed from the post table.
	ItalicAngle() float64

	// CapHeight returns the cap height in font design units.
	// Parsed from the OS/2 table (sCapHeight field).
	// Returns 0 if the OS/2 table is missing or too short.
	CapHeight() int

	// StemV returns the dominant vertical stem width for PDF FontDescriptor.
	// Derived from the OS/2 table usWeightClass.
	// Returns 0 if the OS/2 table is missing.
	StemV() int

	// Kern returns the kerning adjustment between two glyphs in font design units.
	// A negative value means the glyphs should be moved closer together.
	// Returns 0 if no kerning data is available or the pair has no adjustment.
	Kern(left, right uint16) int

	// Flags returns the PDF font flags (ISO 32000 §9.8.2, Table 123).
	Flags() uint32

	// RawData returns the complete, unmodified font file bytes.
	// Used for embedding the full font in the PDF.
	RawData() []byte

	// NumGlyphs returns the total number of glyphs in the font.
	NumGlyphs() int

	// GSUB returns the parsed OpenType GSUB substitution tables for
	// Arabic positional shaping features (init, medi, fina, isol).
	// Returns nil if the font has no GSUB table or no Arabic features.
	// The result is cached after the first call.
	GSUB() GSUBSubstitutions
}
