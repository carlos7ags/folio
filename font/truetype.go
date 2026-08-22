// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sync"
)

// sfntFace is the Face implementation backed by Folio's in-tree
// TrueType / OpenType table parsers. Lazy caches (gsubResult,
// gidToUnicodeMap, kernPairs) are each guarded by their own sync.Once,
// so a single sfntFace may be shared and read concurrently across
// goroutines. This is an internal implementation — callers use the
// Face interface.
//
// The name "sfntFace" is retained for source compatibility with the
// previous `golang.org/x/image/font/sfnt`-backed implementation; the
// dependency itself is gone from the metric path (issue #260).
type sfntFace struct {
	pf      *parsedFont
	rawData []byte

	// Descriptor metrics decoded once at construction (immutable after
	// ParseTTF): italicAngle and fixedPitch from the raw post table,
	// stemV and serif from the raw OS/2 table.
	italicAngle float64
	stemV       int
	fixedPitch  bool
	serif       bool

	// Cached GSUB substitution tables. gsubOnce guards the single parse,
	// covering both "parsed and empty" (gsubResult nil) and populated.
	gsubResult *GSUBSubstitutions
	gsubOnce   sync.Once

	// Cached GID→Unicode reverse map (nil = not yet built).
	gidToUnicodeMap  map[uint16]rune
	gidToUnicodeOnce sync.Once

	// Cached kern pairs: (leftGID, rightGID) → FUnit value. Populated on
	// the first Kern() call. A nil map after parsing means the font has
	// no kern table or no supported subtables; kernOnce guards re-parsing.
	kernPairs map[[2]uint16]int16
	kernOnce  sync.Once

	// Cached GPOS adjustments. gposOnce guards the single parse, covering
	// both "parsed and empty" (gposResult nil) and populated.
	gposResult *GPOSAdjustments
	gposOnce   sync.Once
}

// ParseTTF parses a TrueType (.ttf) or OpenType (.otf) font from raw bytes.
// Returns a Face that can be used for PDF embedding.
//
// The metric tables (head, hhea, maxp, hmtx, OS/2, name, cmap) are
// decoded eagerly via Folio's in-tree parsers; opaque tables (glyf,
// loca, post, kern, GSUB, GPOS, CFF) are kept as raw byte slices and
// decoded on first use by their respective callers.
func ParseTTF(data []byte) (Face, error) {
	pf, err := parseAllTables(data)
	if err != nil {
		return nil, fmt.Errorf("font: parse font: %w", err)
	}
	post := pf.rawTables["post"]
	os2 := pf.rawTables["OS/2"]
	return &sfntFace{
		pf:          pf,
		rawData:     data,
		italicAngle: postItalicAngle(post),
		stemV:       os2StemV(os2),
		fixedPitch:  postIsFixedPitch(post),
		serif:       os2IsSerif(os2),
	}, nil
}

// postItalicAngle parses the post table's Fixed 16.16 italic angle
// field at offset 4. Returns 0 if post is missing or too short.
func postItalicAngle(post []byte) float64 {
	if len(post) < 8 {
		return 0
	}
	raw := binary.BigEndian.Uint32(post[4:8])
	intPart := int16(raw >> 16)
	fracPart := float64(raw&0xFFFF) / 65536.0
	return float64(intPart) + fracPart
}

// postIsFixedPitch checks the post table isFixedPitch field (offset 12).
func postIsFixedPitch(post []byte) bool {
	if len(post) < 16 {
		return false
	}
	return binary.BigEndian.Uint32(post[12:16]) != 0
}

// os2StemV derives the dominant vertical stem width from the OS/2
// usWeightClass using the formula 10 + 220*(weightClass-50)/900,
// clamped to a minimum of 10. Returns 80 as a fallback if OS/2 is
// missing.
func os2StemV(os2 []byte) int {
	if len(os2) < 6 {
		return 80
	}
	weightClass := int(binary.BigEndian.Uint16(os2[4:6]))
	stemV := int(math.Round(10 + 220*float64(weightClass-50)/900))
	return max(stemV, 10)
}

// os2IsSerif checks the OS/2 sFamilyClass field (offset 30-31).
// Family classes 1-5 and 7 indicate serif fonts.
func os2IsSerif(os2 []byte) bool {
	if len(os2) < 32 {
		return false
	}
	class := int(int16(binary.BigEndian.Uint16(os2[30:32]))) >> 8 // high byte is class ID
	return class >= 1 && class <= 5 || class == 7
}

// LoadTTF reads and parses a TrueType font file from disk.
func LoadTTF(path string) (Face, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("font: read font file: %w", err)
	}
	return ParseTTF(data)
}

// PostScriptName returns the PostScript name (NameID 6), falling back
// to the full name (NameID 4) if the PostScript name entry is missing
// or empty.
func (f *sfntFace) PostScriptName() string {
	if f.pf.postScriptName != "" {
		return f.pf.postScriptName
	}
	return f.pf.fullName
}

// UnitsPerEm returns the font's design units per em (head.unitsPerEm).
func (f *sfntFace) UnitsPerEm() int {
	return int(f.pf.head.unitsPerEm)
}

// GlyphIndex returns the glyph ID for r, or 0 if the rune is not in the font.
func (f *sfntFace) GlyphIndex(r rune) uint16 {
	return f.pf.cmap[r]
}

// GlyphAdvance returns the advance width in font design units, or 0 if
// the glyph ID is out of range.
func (f *sfntFace) GlyphAdvance(glyphID uint16) int {
	if int(glyphID) >= len(f.pf.advances) {
		return 0
	}
	return int(f.pf.advances[glyphID])
}

// Ascent returns the typographic ascent in font design units.
//
// Selection follows the OpenType USE_TYPO_METRICS convention
// (OS/2 fsSelection bit 7): when set, the foundry has explicitly
// requested the typo metrics, so we use sTypoAscender; otherwise we
// match golang.org/x/image/font/sfnt and use the hhea ascender.
// Fonts without an OS/2 table fall back to hhea unconditionally.
//
// This preserves byte-identical FontDescriptor /Ascent values for
// every font where USE_TYPO_METRICS is unset (the majority — and
// every font sfnt v0.39.0 produced metrics for) while honoring the
// foundry's explicit override on the small set of fonts that opt in.
func (f *sfntFace) Ascent() int {
	if f.pf.os2 != nil && f.pf.os2.useTypoMetrics() {
		return int(f.pf.os2.sTypoAscender)
	}
	return int(f.pf.hhea.ascender)
}

// Descent returns the typographic descent as a negative value in font
// design units. PDF's font descriptor expects a negative number per
// ISO 32000 Table 122; both sTypoDescender (OS/2) and the hhea
// descender are already stored as signed values, so a font that ships
// a positive descent (rare but legal) round-trips that sign here.
//
// Selection mirrors [Ascent]: USE_TYPO_METRICS gates the choice
// between sTypoDescender and hhea.descender, preserving sfnt parity
// for fonts where the bit is unset.
func (f *sfntFace) Descent() int {
	if f.pf.os2 != nil && f.pf.os2.useTypoMetrics() {
		return int(f.pf.os2.sTypoDescender)
	}
	return int(f.pf.hhea.descender)
}

// LineGap returns the font's recommended extra line spacing (beyond
// ascent+descent) in font design units, used to derive CSS
// `line-height: normal`. Selection mirrors [Ascent]/[Descent]:
// USE_TYPO_METRICS gates the choice between sTypoLineGap and
// hhea.lineGap.
func (f *sfntFace) LineGap() int {
	if f.pf.os2 != nil && f.pf.os2.useTypoMetrics() {
		return int(f.pf.os2.sTypoLineGap)
	}
	return int(f.pf.hhea.lineGap)
}

// BBox returns the font bounding box from head.{xMin,yMin,xMax,yMax}
// in font design units. The head table already stores the bbox in
// PDF's Y-up coordinate system, so no axis flip is required.
func (f *sfntFace) BBox() [4]int {
	return [4]int{
		int(f.pf.head.xMin),
		int(f.pf.head.yMin),
		int(f.pf.head.xMax),
		int(f.pf.head.yMax),
	}
}

// ItalicAngle returns the italic angle, decoded at construction from
// the post table's Fixed 16.16 field at offset 4 (0 if post was
// missing or too short).
func (f *sfntFace) ItalicAngle() float64 {
	return f.italicAngle
}

// CapHeight returns the cap height from the OS/2 table. Requires
// OS/2 version >= 2; returns 0 otherwise.
func (f *sfntFace) CapHeight() int {
	if f.pf.os2 == nil {
		return 0
	}
	return int(f.pf.os2.sCapHeight)
}

// StemV returns the dominant vertical stem width, decoded at
// construction from the OS/2 usWeightClass using the formula
// 10 + 220*(weightClass-50)/900, clamped to a minimum of 10 (80 as a
// fallback if OS/2 was missing).
func (f *sfntFace) StemV() int {
	return f.stemV
}

// Kern returns the kerning adjustment between two glyphs. GPOS
// LookupType 2 ("kern" feature) takes precedence over the legacy kern
// table when a pair is present in both, per Microsoft OpenType guidance
// on GPOS being the canonical source of pair positioning in modern
// fonts. Returns 0 when neither source carries an adjustment.
func (f *sfntFace) Kern(left, right uint16) int {
	if g := f.GPOS(); g != nil {
		if v := g.PairAdjust(left, right); v != 0 {
			return int(v)
		}
	}
	f.kernOnce.Do(func() {
		if kern, ok := f.pf.rawTables["kern"]; ok {
			f.kernPairs = ParseKern(kern)
		}
	})
	if f.kernPairs == nil {
		return 0
	}
	return int(f.kernPairs[[2]uint16{left, right}])
}

// Flags returns the PDF font descriptor flags per ISO 32000 Table 123.
// Bits are computed from font metadata: FixedPitch (bit 0), Serif (bit 1),
// Symbolic (bit 2), Nonsymbolic (bit 5), Italic (bit 6).
func (f *sfntFace) Flags() uint32 {
	var flags uint32

	// Bit 0 (1): FixedPitch — check post table isFixedPitch field.
	if f.isFixedPitch() {
		flags |= 1
	}

	// Bit 1 (2): Serif — check OS/2 sFamilyClass.
	if f.isSerif() {
		flags |= 2
	}

	// Bit 2 (4) vs Bit 5 (32): Symbolic vs Nonsymbolic (mutually exclusive).
	// A font with a Unicode cmap that can map 'A' is Nonsymbolic.
	if f.GlyphIndex('A') != 0 {
		flags |= 32 // Nonsymbolic
	} else {
		flags |= 4 // Symbolic
	}

	// Bit 6 (64): Italic — check italic angle or OS/2 fsSelection.
	if f.ItalicAngle() != 0 || f.isItalicFromOS2() {
		flags |= 64
	}

	return flags
}

// isFixedPitch reports the post table isFixedPitch field, decoded at
// construction.
func (f *sfntFace) isFixedPitch() bool {
	return f.fixedPitch
}

// isSerif reports the OS/2 sFamilyClass classification, decoded at
// construction.
func (f *sfntFace) isSerif() bool {
	return f.serif
}

// isItalicFromOS2 checks OS/2 fsSelection bit 0 (Italic).
func (f *sfntFace) isItalicFromOS2() bool {
	if f.pf.os2 == nil {
		return false
	}
	return f.pf.os2.fsSelection&1 != 0
}

// RawData returns the complete, unmodified font file bytes.
func (f *sfntFace) RawData() []byte {
	return f.rawData
}

// NumGlyphs returns the total number of glyphs in the font.
func (f *sfntFace) NumGlyphs() int {
	return int(f.pf.maxp.numGlyphs)
}

// IsCFF reports whether this face uses CFF v1 outlines (a `CFF ` table)
// rather than TrueType outlines (glyf/loca). The check is intentionally
// narrow — `CFF2` variable-CFF (a distinct table tag) returns false so
// it cannot inadvertently take the v1 embed path, which would emit the
// wrong /FontFile3 stream subtype.
func (f *sfntFace) IsCFF() bool {
	_, ok := f.pf.rawTables["CFF "]
	return ok
}

// IsCFF2 reports whether this face uses CFF2 outlines (a `CFF2` table,
// CFF variable-font extension). Folio does not yet have a CFF2 embed
// path; the dispatcher uses this only to keep CFF2 fonts off the v1
// path so they fall back to the legacy embed semantics.
func (f *sfntFace) IsCFF2() bool {
	_, ok := f.pf.rawTables["CFF2"]
	return ok
}

// CFFData returns the raw bytes of the `CFF ` table when present.
// The PDF FontFile3 stream expects raw CFF bytes — not the surrounding
// sfnt wrapper — so this is the input both to CFF subsetting and to
// the unmodified-embed fallback. Returns nil for non-CFF faces.
func (f *sfntFace) CFFData() []byte {
	return f.pf.rawTables["CFF "]
}

// GSUB returns the parsed GSUB substitution tables. The result is cached
// after the first call; a nil return means the font has no GSUB tables
// for any of the recognized features.
func (f *sfntFace) GSUB() *GSUBSubstitutions {
	f.gsubOnce.Do(func() { f.gsubResult = ParseGSUB(f.rawData) })
	return f.gsubResult
}

// GPOS returns the parsed GPOS positioning tables. The result is cached
// after the first call; a nil return means the font has no recognized
// GPOS data (no "kern"/"mark" features, or only unsupported lookup
// types).
func (f *sfntFace) GPOS() *GPOSAdjustments {
	f.gposOnce.Do(func() { f.gposResult = ParseGPOS(f.rawData) })
	return f.gposResult
}

// GIDToUnicode returns a reverse mapping from glyph ID to Unicode codepoint.
// Built lazily from the font's parsed cmap. Used to convert
// GSUB-substituted GIDs back to codepoints for the text rendering pipeline.
func (f *sfntFace) GIDToUnicode() map[uint16]rune {
	f.gidToUnicodeOnce.Do(func() { f.gidToUnicodeMap = buildGIDToUnicodeFromCmap(f.pf.cmap) })
	return f.gidToUnicodeMap
}

// BuildGIDToUnicode parses a TrueType/OpenType font and builds a map
// from glyph ID to Unicode code point by scanning the font's cmap table.
// This is used as a fallback for CIDFont text extraction when no
// ToUnicode CMap is provided.
//
// First rune wins if multiple runes map to the same GID — matching
// the previous sfnt-backed behavior, which scanned the BMP in
// ascending order.
//
// Returns nil if parsing fails or the cmap yields no entries.
func BuildGIDToUnicode(fontData []byte) map[uint16]rune {
	pf, err := parseAllTables(fontData)
	if err != nil {
		return nil
	}
	return buildGIDToUnicodeFromCmap(pf.cmap)
}

// buildGIDToUnicodeFromCmap inverts a parsed cmap into a GID→rune
// map. When multiple runes target the same GID, the lowest rune wins
// — this matches the previous sfnt-backed scan over U+0000..U+FFFF in
// ascending order, which the text-extraction tests rely on for stable
// mappings of common Latin characters.
func buildGIDToUnicodeFromCmap(cmap cmapTable) map[uint16]rune {
	if len(cmap) == 0 {
		return nil
	}
	out := make(map[uint16]rune, len(cmap))
	for r, gid := range cmap {
		if gid == 0 {
			continue
		}
		if existing, ok := out[gid]; !ok || r < existing {
			out[gid] = r
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
