// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

// Face represents a parsed font file and provides metric, encoding, and
// glyph-indexing operations used when embedding the font in a PDF.
//
// Concurrency: Faces returned by ParseTTF/LoadTTF synchronize their
// internal lazy caches (table data, GSUB tables, GID-to-Unicode maps),
// so their methods are safe for concurrent use by multiple goroutines
// and a single Face may be shared across concurrently-rendered
// documents. EmbeddedFont, which wraps a Face, tracks per-document
// glyph usage and remains not safe for concurrent use — do not share
// one EmbeddedFont across concurrently-rendered documents.
//
// Face is the abstraction over a parsed font file. It provides the
// data needed to embed a font in a PDF: glyph metrics, character
// mapping, and the raw font bytes.
//
// The metric path is implemented in-tree (head, hhea, maxp, hmtx,
// OS/2, name, cmap). GSUB/GPOS/kern/post are parsed lazily by their
// respective callers from the raw table directory. The interface
// remains small so a future replacement parser can swap in without
// breaking callers.
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
	GSUB() *GSUBSubstitutions
	// GIDToUnicode returns a reverse mapping from glyph ID to Unicode
	// codepoint, built from the font's cmap table. Used to convert
	// GSUB-substituted GIDs back to codepoints for the text pipeline.
	// The result is cached after the first call.
	GIDToUnicode() map[uint16]rune
}

// GPOSProvider is an optional interface that a Face may implement to
// expose parsed OpenType GPOS positioning tables. GPOS() returns nil
// when the font has no recognized positioning data. See GSUBProvider
// for the rationale behind the optional-interface pattern during v0.x.
//
// TODO: at v1.0, merge GPOS() back into Face.
type GPOSProvider interface {
	GPOS() *GPOSAdjustments
}

// cffFace is a package-private interface for accessing CFF table data
// on faces whose underlying implementation parses sfnt tables. CFF
// embedding takes a different PDF object-graph path from TrueType, so
// the embedder needs to detect CFF and pull out the raw `CFF ` table
// bytes; this interface keeps that coupling out of the public Face
// surface during v0.x.
type cffFace interface {
	IsCFF() bool
	IsCFF2() bool
	CFFData() []byte
}

// faceCFFData returns (cffBytes, true) when the face is a CID-keyed
// CFF v1 font — the only outline format Folio currently has a working
// /FontFile3 + /CIDFontType0 embed path for. Plain (name-keyed) CFF v1
// needs a /Type1C stream and a non-composite font dictionary; CFF2 has
// a different stream subtype and FVAR-driven instancing. Both are
// kept off the CID-keyed path and fall back to the legacy embed
// semantics until a dedicated path lands. Returns (nil, false)
// otherwise.
func faceCFFData(f Face) ([]byte, bool) {
	cf, ok := f.(cffFace)
	if !ok || !cf.IsCFF() || cf.IsCFF2() {
		return nil, false
	}
	data := cf.CFFData()
	if !isCIDKeyedCFFv1(data) {
		return nil, false
	}
	return data, true
}
