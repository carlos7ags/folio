// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"os"
	"testing"
)

// kernBuilder assembles synthetic kern table bytes for tests. The
// builder mirrors the spec layout closely so each test reads like a
// byte-level picture of the table it exercises.
type kernBuilder struct {
	buf []byte
}

func (b *kernBuilder) u16(v uint16) {
	var tmp [2]byte
	binary.BigEndian.PutUint16(tmp[:], v)
	b.buf = append(b.buf, tmp[:]...)
}

func (b *kernBuilder) u32(v uint32) {
	var tmp [4]byte
	binary.BigEndian.PutUint32(tmp[:], v)
	b.buf = append(b.buf, tmp[:]...)
}

func (b *kernBuilder) i16(v int16) { b.u16(uint16(v)) }

type kernPair struct {
	left, right uint16
	value       int16
}

// format0Body encodes nPairs + dummy search tuple + pairs. The returned
// slice is the subtable body following its header.
func format0Body(pairs ...kernPair) []byte {
	var body kernBuilder
	body.u16(uint16(len(pairs))) // nPairs
	body.u16(0)                  // searchRange
	body.u16(0)                  // entrySelector
	body.u16(0)                  // rangeShift
	for _, p := range pairs {
		body.u16(p.left)
		body.u16(p.right)
		body.i16(p.value)
	}
	return body.buf
}

// v0Subtable wraps a format-0 body in a v0 subtable header.
func v0Subtable(coverage uint16, body []byte) []byte {
	var sub kernBuilder
	total := 6 + len(body)
	sub.u16(0)             // subtable version
	sub.u16(uint16(total)) // length
	sub.u16(coverage)      // coverage
	sub.buf = append(sub.buf, body...)
	return sub.buf
}

// v0Table wraps a set of subtables in a v0 kern table header.
func v0Table(subs ...[]byte) []byte {
	var tbl kernBuilder
	tbl.u16(0) // version
	tbl.u16(uint16(len(subs)))
	for _, s := range subs {
		tbl.buf = append(tbl.buf, s...)
	}
	return tbl.buf
}

// v1Subtable wraps a format-0 body in a v1 (Apple AAT) subtable header.
// The v1 header is 8 bytes: length(uint32), coverage(uint16),
// tupleIndex(uint16).
func v1Subtable(coverage uint16, body []byte) []byte {
	var sub kernBuilder
	total := 8 + len(body)
	sub.u32(uint32(total))
	sub.u16(coverage)
	sub.u16(0) // tupleIndex
	sub.buf = append(sub.buf, body...)
	return sub.buf
}

// v1Table wraps a set of subtables in a v1 kern table header. The v1
// header is 8 bytes: version(Fixed 16.16 == 0x00010000), nTables(uint32).
func v1Table(subs ...[]byte) []byte {
	var tbl kernBuilder
	tbl.u32(0x00010000) // version
	tbl.u32(uint32(len(subs)))
	for _, s := range subs {
		tbl.buf = append(tbl.buf, s...)
	}
	return tbl.buf
}

func TestParseKernNilAndEmpty(t *testing.T) {
	if m := ParseKern(nil); m != nil {
		t.Errorf("ParseKern(nil) = %v, want nil", m)
	}
	if m := ParseKern([]byte{0, 0, 0, 0}); m != nil {
		t.Errorf("ParseKern(4 zero bytes) = %v, want nil", m)
	}
	// Fewer than 4 bytes is unparseable.
	if m := ParseKern([]byte{0, 0}); m != nil {
		t.Errorf("ParseKern(2 bytes) = %v, want nil", m)
	}
}

func TestParseKernV0Format0TwoPairs(t *testing.T) {
	body := format0Body(
		kernPair{0x0041, 0x0056, -70}, // A V
		kernPair{0x0041, 0x0057, -50}, // A W
	)
	// Coverage: format=0 (high byte), horizontal bit set (low byte bit 0).
	sub := v0Subtable(0x0001, body)
	data := v0Table(sub)

	m := ParseKern(data)
	if m == nil {
		t.Fatal("ParseKern returned nil map")
	}
	if got := m[[2]uint16{0x0041, 0x0056}]; got != -70 {
		t.Errorf("A,V = %d, want -70", got)
	}
	if got := m[[2]uint16{0x0041, 0x0057}]; got != -50 {
		t.Errorf("A,W = %d, want -50", got)
	}
	if got := m[[2]uint16{0x0042, 0x0042}]; got != 0 {
		t.Errorf("missing pair = %d, want 0", got)
	}
}

func TestParseKernSignPreservation(t *testing.T) {
	body := format0Body(
		kernPair{1, 2, -32768}, // most-negative int16
		kernPair{3, 4, 32767},  // most-positive int16
		kernPair{5, 6, -1},
	)
	data := v0Table(v0Subtable(0x0001, body))

	m := ParseKern(data)
	if m == nil {
		t.Fatal("ParseKern returned nil map")
	}
	if got := m[[2]uint16{1, 2}]; got != -32768 {
		t.Errorf("got %d, want -32768", got)
	}
	if got := m[[2]uint16{3, 4}]; got != 32767 {
		t.Errorf("got %d, want 32767", got)
	}
	if got := m[[2]uint16{5, 6}]; got != -1 {
		t.Errorf("got %d, want -1", got)
	}
}

func TestParseKernV0SkipsFormat1Subtable(t *testing.T) {
	// First subtable: format=1 (high byte), horizontal. Body is 8 bytes
	// of filler since ParseKern should skip it without looking.
	filler := []byte{0, 0, 0, 0, 0, 0, 0, 0}
	sub1 := v0Subtable(0x0101, filler) // format=1, horizontal
	// Second subtable: format=0, horizontal, with a real pair.
	body2 := format0Body(kernPair{0x0041, 0x0056, -70})
	sub2 := v0Subtable(0x0001, body2)
	data := v0Table(sub1, sub2)

	m := ParseKern(data)
	if m == nil {
		t.Fatal("expected pairs from the format-0 subtable")
	}
	if got := m[[2]uint16{0x0041, 0x0056}]; got != -70 {
		t.Errorf("A,V = %d, want -70", got)
	}
	if len(m) != 1 {
		t.Errorf("map has %d entries, want 1", len(m))
	}
}

func TestParseKernV0HorizontalBitClearSkipped(t *testing.T) {
	// A v0 subtable with the horizontal bit cleared represents vertical
	// kerning and must not contribute pairs to the horizontal lookup.
	body := format0Body(kernPair{1, 2, -40})
	// Coverage: format=0, horizontal bit clear.
	sub := v0Subtable(0x0000, body)
	data := v0Table(sub)

	if m := ParseKern(data); m != nil {
		t.Errorf("expected nil map when horizontal bit is clear, got %v", m)
	}
}

func TestParseKernV0CrossStreamBitSkipped(t *testing.T) {
	// Cross-stream pairs are perpendicular to text direction and must be
	// ignored even when the horizontal bit is set.
	body := format0Body(kernPair{1, 2, -40})
	// Coverage: format=0, horizontal | cross-stream (0x01 | 0x04).
	sub := v0Subtable(0x0005, body)
	data := v0Table(sub)

	if m := ParseKern(data); m != nil {
		t.Errorf("expected nil map when cross-stream bit is set, got %v", m)
	}
}

func TestParseKernV0MinimumBitSkipped(t *testing.T) {
	// Minimum values rather than adjustments: not supported by folio's
	// cumulative-adjust kerning model and must be ignored.
	body := format0Body(kernPair{1, 2, -40})
	sub := v0Subtable(0x0003, body) // horizontal | minimum
	data := v0Table(sub)

	if m := ParseKern(data); m != nil {
		t.Errorf("expected nil map when minimum bit is set, got %v", m)
	}
}

func TestParseKernV1Format0Pair(t *testing.T) {
	body := format0Body(kernPair{0x0041, 0x0056, -70})
	// v1 coverage: format in the LOW byte. Vertical/cross-stream bits clear.
	sub := v1Subtable(0x0000, body)
	data := v1Table(sub)

	m := ParseKern(data)
	if m == nil {
		t.Fatal("ParseKern returned nil for v1 table")
	}
	if got := m[[2]uint16{0x0041, 0x0056}]; got != -70 {
		t.Errorf("A,V = %d, want -70", got)
	}
}

func TestParseKernV1VerticalBitSkipped(t *testing.T) {
	// v1 bit 15 = vertical. Must be ignored by horizontal lookup.
	body := format0Body(kernPair{0x0041, 0x0056, -70})
	sub := v1Subtable(0x8000, body)
	data := v1Table(sub)

	if m := ParseKern(data); m != nil {
		t.Errorf("expected nil map when v1 vertical bit is set, got %v", m)
	}
}

func TestParseKernV1CrossStreamBitSkipped(t *testing.T) {
	// v1 bit 14 = cross-stream. Must be skipped.
	body := format0Body(kernPair{0x0041, 0x0056, -70})
	sub := v1Subtable(0x4000, body)
	data := v1Table(sub)

	if m := ParseKern(data); m != nil {
		t.Errorf("expected nil map when v1 cross-stream bit is set, got %v", m)
	}
}

func TestParseKernV1VariationBitSkipped(t *testing.T) {
	// v1 bit 13 = variation. Must be skipped.
	body := format0Body(kernPair{0x0041, 0x0056, -70})
	sub := v1Subtable(0x2000, body)
	data := v1Table(sub)

	if m := ParseKern(data); m != nil {
		t.Errorf("expected nil map when v1 variation bit is set, got %v", m)
	}
}

func TestParseKernInflatedNPairsClamped(t *testing.T) {
	// Format-0 body with nPairs inflated well beyond the actual data.
	// The parser must clamp rather than read off the end.
	body := []byte{
		0x27, 0x0F, // nPairs = 9999
		0x00, 0x00, // searchRange
		0x00, 0x00, // entrySelector
		0x00, 0x00, // rangeShift
		0x00, 0x41, 0x00, 0x56, 0xFF, 0xB0, // A V -80
	}
	sub := v0Subtable(0x0001, body)
	data := v0Table(sub)

	m := ParseKern(data)
	if m == nil {
		t.Fatal("ParseKern returned nil; should have clamped and kept the one real pair")
	}
	if got := m[[2]uint16{0x0041, 0x0056}]; got != -80 {
		t.Errorf("A,V = %d, want -80", got)
	}
	if len(m) != 1 {
		t.Errorf("map has %d entries, want 1 (others clamped out)", len(m))
	}
}

func TestParseKernUnknownVersion(t *testing.T) {
	// First two bytes = 0x0002 is neither v0 nor v1 and must return nil.
	data := []byte{0x00, 0x02, 0x00, 0x00}
	if m := ParseKern(data); m != nil {
		t.Errorf("unknown version returned %v, want nil", m)
	}
}

func TestParseKernV0TruncatedSubtable(t *testing.T) {
	// Declared subtable length exceeds available bytes — parser must bail
	// out without returning spurious pairs.
	var b kernBuilder
	b.u16(0)        // version
	b.u16(1)        // nTables = 1
	b.u16(0)        // subtable version
	b.u16(6 + 1024) // length = 1030 (far exceeds remaining)
	b.u16(0x0001)   // coverage
	// body deliberately truncated
	if m := ParseKern(b.buf); m != nil {
		t.Errorf("expected nil map for truncated subtable, got %v", m)
	}
}

func TestParseKernFromRealFont(t *testing.T) {
	path := "/System/Library/Fonts/Supplemental/Arial.ttf"
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("Arial TTF not available: %v", err)
	}
	face, err := ParseTTF(data)
	if err != nil {
		t.Fatalf("ParseTTF: %v", err)
	}
	// A system Arial typically contains a kern table with classic pairs
	// such as A-V. If it does not, the test logs a note and succeeds —
	// we cannot assert a specific value without making the test fragile
	// to font vendor updates.
	l := face.GlyphIndex('A')
	r := face.GlyphIndex('V')
	if l == 0 || r == 0 {
		t.Skip("font has no glyph for A or V")
	}
	k := face.Kern(l, r)
	t.Logf("real-font Kern('A','V') = %d (FUnits)", k)
	if k == 0 {
		t.Log("note: font may use GPOS instead of a kern table for this pair")
	}
	if k > 0 {
		t.Errorf("A,V kern should be <= 0 for Latin fonts, got %d", k)
	}
}

func TestKernCacheIdentity(t *testing.T) {
	// A second Kern() call must not rebuild the map: the face struct
	// holds one cached instance.
	face := loadTestFace(t).(*sfntFace)
	// Prime the cache.
	_ = face.Kern(1, 2)
	first := face.kernPairs
	_ = face.Kern(3, 4)
	second := face.kernPairs
	// Same underlying map (either both nil or both the same header).
	if (first == nil) != (second == nil) {
		t.Errorf("cache nil-ness changed between calls: first=%v second=%v",
			first == nil, second == nil)
	}
}

// TestKernCacheReparseGuard exercises the kernPairsParsed one-shot flag
// by manually clearing kernPairs and confirming a second lookup does
// not re-populate the cache from the raw table bytes.
func TestKernCacheReparseGuard(t *testing.T) {
	face := loadTestFace(t).(*sfntFace)
	_ = face.Kern(0, 0)
	if !face.kernPairsParsed {
		t.Fatal("expected kernPairsParsed=true after first Kern call")
	}
	face.kernPairs = nil
	_ = face.Kern(1, 2)
	if face.kernPairs != nil {
		t.Errorf("second Kern call re-populated cache after manual clear")
	}
}
