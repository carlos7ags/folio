// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

// Face represents a parsed font file and provides metric, encoding, and
// glyph-indexing operations used when embedding the font in a PDF.
//
// Concurrency: Face implementations are not safe for concurrent use by
// multiple goroutines. Individual methods lazily populate internal caches
// (table data, GSUB tables, GID-to-Unicode maps), and these caches are
// not synchronized. A single Face may be reused across many pages in a
// document so long as page rendering is sequential, which is how folio's
// layout pipeline uses them. If you need a Face from multiple goroutines,
// give each goroutine its own instance via ParseFont or LoadFont.
//
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
}

// GSUBProvider is an optional interface that a Face may implement to
// expose parsed OpenType GSUB substitution tables for Arabic positional
// shaping features (init, medi, fina, isol). Callers should type-assert
// to check availability rather than requiring all Face implementations
// to support GSUB. This avoids breaking external Face implementers
// during v0.x.
//
// TODO: at v1.0, merge GSUB() back into Face. The type-assertion
// indirection adds no value once the API is stable.
type GSUBProvider interface {
	GSUB() GSUBSubstitutions
	// GIDToUnicode returns a reverse mapping from glyph ID to Unicode
	// codepoint, built from the font's cmap table. Used to convert
	// GSUB-substituted GIDs back to codepoints for the text pipeline.
	// The result is cached after the first call.
	GIDToUnicode() map[uint16]rune
}
