// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"testing"
)

// fakeFace is a minimal Face implementation that resolves rune coverage
// from an explicit allowlist. Used to test Fallback dispatch without
// requiring real font files on the system.
type fakeFace struct {
	covers map[rune]uint16 // rune -> non-zero gid; missing rune -> 0 (.notdef)
}

func newFakeFace(runes ...rune) *fakeFace {
	covers := make(map[rune]uint16, len(runes))
	for i, r := range runes {
		covers[r] = uint16(i + 1) // any non-zero gid suffices
	}
	return &fakeFace{covers: covers}
}

func (f *fakeFace) PostScriptName() string        { return "FakeFace" }
func (f *fakeFace) UnitsPerEm() int               { return 1000 }
func (f *fakeFace) GlyphIndex(r rune) uint16      { return f.covers[r] }
func (f *fakeFace) GlyphAdvance(_ uint16) int     { return 500 }
func (f *fakeFace) Ascent() int                   { return 800 }
func (f *fakeFace) Descent() int                  { return -200 }
func (f *fakeFace) BBox() [4]int                  { return [4]int{0, -200, 1000, 800} }
func (f *fakeFace) ItalicAngle() float64          { return 0 }
func (f *fakeFace) CapHeight() int                { return 700 }
func (f *fakeFace) StemV() int                    { return 80 }
func (f *fakeFace) Kern(_, _ uint16) int          { return 0 }
func (f *fakeFace) Flags() uint32                 { return 0 }
func (f *fakeFace) RawData() []byte               { return nil }
func (f *fakeFace) NumGlyphs() int                { return len(f.covers) + 1 }
func (f *fakeFace) GSUB() *GSUBSubstitutions      { return nil }
func (f *fakeFace) GIDToUnicode() map[uint16]rune { return nil }
func (f *fakeFace) GPOS() *GPOSAdjustments        { return nil }

func TestNewFallbackPanicsOnEmpty(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewFallback() with no faces should panic")
		}
	}()
	_ = NewFallback()
}

func TestNewFallbackPanicsOnNil(t *testing.T) {
	ef := NewEmbeddedFont(newFakeFace('a'))
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewFallback with nil entry should panic")
		}
	}()
	_ = NewFallback(ef, nil)
}

func TestFallbackPickFaceFirstHit(t *testing.T) {
	latin := NewEmbeddedFont(newFakeFace('H', 'e', 'l', 'o'))
	hebrew := NewEmbeddedFont(newFakeFace('\u05E9', '\u05DC', '\u05D5', '\u05DD'))
	fb := NewFallback(latin, hebrew)

	tests := []struct {
		name string
		r    rune
		want *EmbeddedFont
	}{
		{"latin H lands on first face", 'H', latin},
		{"hebrew shin lands on second face", '\u05E9', hebrew},
		{"missing rune falls back to first face", '\u4E2D', latin}, // Han, neither covers
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fb.PickFace(tc.r)
			if got != tc.want {
				t.Errorf("PickFace(%U): got face=%p, want %p", tc.r, got, tc.want)
			}
		})
	}
}

func TestFallbackFacesPreservesPointerOrder(t *testing.T) {
	a := NewEmbeddedFont(newFakeFace('a'))
	b := NewEmbeddedFont(newFakeFace('b'))
	c := NewEmbeddedFont(newFakeFace('c'))
	fb := NewFallback(a, b, c)

	got := fb.Faces()
	want := []*EmbeddedFont{a, b, c}
	if len(got) != len(want) {
		t.Fatalf("Faces() returned %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Faces()[%d] = %p, want %p", i, got[i], want[i])
		}
	}
}

func TestFallbackSubsetAggregatesUsedGlyphsOnSharedPointer(t *testing.T) {
	// Acceptance criterion #4 in #192: subsetting produces one font
	// resource per underlying face, each containing only the glyphs
	// used from that face. The mechanism is pointer dedupe: when the
	// same *EmbeddedFont is referenced from many TextRuns, every
	// EncodeString call accumulates onto the same usedGlyphs map, so
	// BuildObjects sees the union of glyphs across the document.
	//
	// This test simulates the layout layer's behaviour by encoding
	// distinct Hebrew strings through the same *EmbeddedFont returned
	// by the Fallback's PickFace, then asserting the aggregated glyph
	// set covers every input rune. If a future refactor wraps or
	// copies *EmbeddedFont inside Fallback, the encodings would
	// accumulate on disjoint maps and this test would fail.
	latin := NewEmbeddedFont(newFakeFace('a'))
	hebrew := NewEmbeddedFont(newFakeFace('\u05E9', '\u05DC', '\u05D5', '\u05DD'))
	fb := NewFallback(latin, hebrew)

	face1 := fb.PickFace('\u05E9')
	face1.EncodeString("\u05E9\u05DC")
	face2 := fb.PickFace('\u05D5')
	face2.EncodeString("\u05D5\u05DD")

	// Both encodings must have landed on the same hebrew pointer.
	if face1 != hebrew || face2 != hebrew {
		t.Fatalf("PickFace returned a non-identity pointer: face1=%p face2=%p hebrew=%p",
			face1, face2, hebrew)
	}
	want := []rune{'\u05E9', '\u05DC', '\u05D5', '\u05DD'}
	for _, r := range want {
		gid := hebrew.face.GlyphIndex(r)
		if _, ok := hebrew.usedGlyphs[gid]; !ok {
			t.Errorf("glyph for %U not aggregated on shared pointer", r)
		}
	}
}

func TestFallbackPointerIdentityShared(t *testing.T) {
	// Pointer dedupe is the load-bearing invariant for subsetting and
	// ToUnicode emission: PickFace must return the original *EmbeddedFont
	// stored on the Fallback, never a copy or a wrapper. If this ever
	// regresses, the page resource map will stop deduping and a document
	// that uses the Fallback in many paragraphs will balloon its font
	// dictionary count.
	latin := NewEmbeddedFont(newFakeFace('a', 'b'))
	hebrew := NewEmbeddedFont(newFakeFace('\u05E9'))
	fb := NewFallback(latin, hebrew)

	if fb.PickFace('a') != latin {
		t.Error("PickFace('a') returned a non-identical face pointer")
	}
	if fb.PickFace('\u05E9') != hebrew {
		t.Error("PickFace(shin) returned a non-identical face pointer")
	}
}
