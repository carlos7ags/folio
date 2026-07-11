// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

// fakeFace is a stub font.Face that resolves rune coverage from an
// explicit allowlist. It exists so the fallback tests can probe
// segmentation behaviour without depending on system fonts (which would
// make tests skip on machines without Hebrew or Devanagari coverage).
type fakeFace struct {
	covers map[rune]uint16
}

func newFakeFace(runes ...rune) *fakeFace {
	covers := make(map[rune]uint16, len(runes))
	for i, r := range runes {
		covers[r] = uint16(i + 1)
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
func (f *fakeFace) GSUB() *font.GSUBSubstitutions { return nil }
func (f *fakeFace) GIDToUnicode() map[uint16]rune { return nil }
func (f *fakeFace) GPOS() *font.GPOSAdjustments   { return nil }

func TestNewParagraphFallbackPanicsOnNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewParagraphFallback with nil fallback should panic")
		}
	}()
	_ = NewParagraphFallback("hi", nil, 12)
}

func TestNewParagraphFallbackPanicsOnFontSize(t *testing.T) {
	ef := font.NewEmbeddedFont(newFakeFace('a'))
	fb := font.NewFallback(ef)
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewParagraphFallback with fontSize<=0 should panic")
		}
	}()
	_ = NewParagraphFallback("hi", fb, 0)
}

func TestNewParagraphFallbackEmptyText(t *testing.T) {
	// Empty input must still emit a single run so the rest of the layout
	// pipeline (which assumes every paragraph has at least one run)
	// doesn't have to special-case empty paragraphs.
	ef := font.NewEmbeddedFont(newFakeFace('a'))
	fb := font.NewFallback(ef)
	p := NewParagraphFallback("", fb, 12)
	runs := p.Runs()
	if len(runs) != 1 {
		t.Fatalf("expected 1 run for empty text, got %d", len(runs))
	}
	if runs[0].Embedded != ef {
		t.Errorf("empty-text run face = %p, want %p", runs[0].Embedded, ef)
	}
	if runs[0].Text != "" {
		t.Errorf("empty-text run text = %q, want empty", runs[0].Text)
	}
}

func TestNewParagraphFallbackSingleScript(t *testing.T) {
	// Pure-Latin input must collapse to one run on the Latin face even
	// though both faces are in the chain. Multiple runs here would
	// pessimise wrapping and inflate the resource map.
	latin := font.NewEmbeddedFont(newFakeFace('H', 'e', 'l', 'o'))
	hebrew := font.NewEmbeddedFont(newFakeFace('\u05E9'))
	fb := font.NewFallback(latin, hebrew)

	p := NewParagraphFallback("Hello", fb, 12)
	runs := p.Runs()
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Embedded != latin {
		t.Errorf("run face = %p, want latin %p", runs[0].Embedded, latin)
	}
	if runs[0].Text != "Hello" {
		t.Errorf("run text = %q, want %q", runs[0].Text, "Hello")
	}
}

func TestNewParagraphFallbackMixedScripts(t *testing.T) {
	// Latin + Hebrew + Arabic in one input should produce three runs
	// each bound to its own face, in source order. Coverage misses
	// (the leading-space ASCII codepoint resolved to Hebrew via the
	// "Common runs inherit from neighbours" rule in SegmentByScript)
	// must not split a script run.
	latin := font.NewEmbeddedFont(newFakeFace('H', 'e', 'l', 'o', ' '))
	hebrew := font.NewEmbeddedFont(newFakeFace('\u05E9', '\u05DC', '\u05D5', '\u05DD', ' '))
	arabic := font.NewEmbeddedFont(newFakeFace('\u0628', '\u0633', '\u0645', ' '))
	fb := font.NewFallback(latin, hebrew, arabic)

	// "Hello שלום بسم"
	p := NewParagraphFallback("Hello \u05E9\u05DC\u05D5\u05DD \u0628\u0633\u0645", fb, 12)
	runs := p.Runs()
	if len(runs) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(runs))
	}
	if runs[0].Embedded != latin {
		t.Errorf("run[0] face = %p, want latin %p", runs[0].Embedded, latin)
	}
	if runs[1].Embedded != hebrew {
		t.Errorf("run[1] face = %p, want hebrew %p", runs[1].Embedded, hebrew)
	}
	if runs[2].Embedded != arabic {
		t.Errorf("run[2] face = %p, want arabic %p", runs[2].Embedded, arabic)
	}
}

func TestNewParagraphFallbackCoalescesAdjacentSameFace(t *testing.T) {
	// Latin -> Common (digits) -> Latin should resolve to a single Latin
	// run after script segmentation merges Commons into their neighbours,
	// not three runs.
	latin := font.NewEmbeddedFont(newFakeFace('H', 'i', '4', '2'))
	hebrew := font.NewEmbeddedFont(newFakeFace('\u05E9'))
	fb := font.NewFallback(latin, hebrew)

	p := NewParagraphFallback("Hi42Hi", fb, 12)
	runs := p.Runs()
	if len(runs) != 1 {
		t.Fatalf("expected 1 coalesced run, got %d", len(runs))
	}
	if runs[0].Embedded != latin {
		t.Errorf("run face = %p, want latin %p", runs[0].Embedded, latin)
	}
}

func TestNewParagraphFallbackPointerReuseAcrossRuns(t *testing.T) {
	// The Hebrew face pointer must appear identically in every run
	// generated for Hebrew script content. Page-level resource dedupe
	// hashes by pointer; if NewParagraphFallback ever copies or wraps
	// faces, every paragraph would emit a fresh Type0 dict.
	latin := font.NewEmbeddedFont(newFakeFace('H', 'i'))
	hebrew := font.NewEmbeddedFont(newFakeFace('\u05E9', '\u05DC'))
	fb := font.NewFallback(latin, hebrew)

	p1 := NewParagraphFallback("Hi \u05E9\u05DC", fb, 12)
	p2 := NewParagraphFallback("\u05E9\u05DC Hi", fb, 12)

	runs1 := p1.Runs()
	runs2 := p2.Runs()
	if runs1[1].Embedded != hebrew {
		t.Error("p1 hebrew run does not point to original hebrew face")
	}
	if runs2[0].Embedded != hebrew {
		t.Error("p2 hebrew run does not point to original hebrew face")
	}
	if runs1[0].Embedded != latin {
		t.Error("p1 latin run does not point to original latin face")
	}
	if runs2[1].Embedded != latin {
		t.Error("p2 latin run does not point to original latin face")
	}
}

func TestNewParagraphFallbackProbeSkipsLeadingCommon(t *testing.T) {
	// SegmentByScript promotes leading Common runes (space, digits,
	// punctuation) into the script of the next real-script rune via
	// its reverse sweep. If face selection probed the literal first
	// rune of the segment, a Hebrew run that begins with whitespace
	// or a digit would route to whatever face covers that Common
	// rune first -- typically the Latin face -- and the entire
	// Hebrew substring would render as .notdef tofu.
	//
	// The fake Hebrew face below intentionally does NOT cover the
	// space, so the test fails immediately if the probe falls on the
	// Common rune.
	latin := font.NewEmbeddedFont(newFakeFace('A', ' ', '4', '2', '('))
	hebrew := font.NewEmbeddedFont(newFakeFace('\u05E9', '\u05DC', '\u05D5', '\u05DD'))
	fb := font.NewFallback(latin, hebrew)

	tests := []struct {
		name string
		text string
		want *font.EmbeddedFont
	}{
		{"leading space", " \u05E9\u05DC\u05D5\u05DD", hebrew},
		{"leading digits", "42 \u05E9\u05DC\u05D5\u05DD", hebrew},
		{"leading paren", "(\u05E9\u05DC\u05D5\u05DD)", hebrew},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParagraphFallback(tc.text, fb, 12)
			runs := p.Runs()
			if len(runs) != 1 {
				t.Fatalf("expected 1 coalesced run, got %d", len(runs))
			}
			if runs[0].Embedded != tc.want {
				t.Errorf("face = %p, want hebrew %p (probe should skip leading Common)",
					runs[0].Embedded, tc.want)
			}
		})
	}
}

func TestNewParagraphFallbackAllCommonInput(t *testing.T) {
	// Pure-Common input has no real-script rune to probe on. Falling
	// back to the first rune is the documented behaviour and routes
	// to whichever face covers that rune first; this guards the
	// pure-Common short-circuit in probeRune.
	latin := font.NewEmbeddedFont(newFakeFace('.', ',', '!'))
	other := font.NewEmbeddedFont(newFakeFace('.'))
	fb := font.NewFallback(latin, other)

	p := NewParagraphFallback("...,,,!!!", fb, 12)
	runs := p.Runs()
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Embedded != latin {
		t.Errorf("all-Common input should land on first face %p, got %p", latin, runs[0].Embedded)
	}
}

func TestNewParagraphFallbackWhitespaceOnly(t *testing.T) {
	// Whitespace-only input is a degenerate but valid case; it must
	// not panic and must produce one run on the first face that
	// covers space.
	latin := font.NewEmbeddedFont(newFakeFace(' ', '\t'))
	fb := font.NewFallback(latin)

	p := NewParagraphFallback("   ", fb, 12)
	runs := p.Runs()
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Embedded != latin {
		t.Errorf("face = %p, want latin %p", runs[0].Embedded, latin)
	}
	if runs[0].Text != "   " {
		t.Errorf("text = %q, want %q", runs[0].Text, "   ")
	}
}

func TestNewParagraphFallbackUncoveredRuneFallsBackToFirstFace(t *testing.T) {
	// When no face in the chain covers a script's base rune, the first
	// face is used so something still draws. The acceptance criterion
	// is "all glyphs render correctly when the chain includes a face
	// per script"; this case (no face for Han) is the unhappy path and
	// must not crash or skip the segment.
	latin := font.NewEmbeddedFont(newFakeFace('H'))
	hebrew := font.NewEmbeddedFont(newFakeFace('\u05E9'))
	fb := font.NewFallback(latin, hebrew)

	p := NewParagraphFallback("\u4E2D", fb, 12) // Han: neither face covers
	runs := p.Runs()
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Embedded != latin {
		t.Errorf("uncovered rune should land on first face %p, got %p", latin, runs[0].Embedded)
	}
}
