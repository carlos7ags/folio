// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// A declared CSS-wide keyword (inherit, initial, unset) is a real cascaded
// value: it must override a less specific rule or a UA tag default, not be
// dropped as an unparseable value that leaves the earlier value in effect.

func TestColorInheritOverridesLessSpecificRule(t *testing.T) {
	src := `<html><head><style>
		a { color: #1a0dd7; }
		.black { color: #000000; }
		.black a.inherit { color: inherit; }
	</style></head><body>
		<div class="black"><a id="t" class="inherit" href="#">AAAA</a></div>
	</body></html>`
	got := styleByID(t, src, "t").Color
	if got != layout.ColorBlack {
		t.Errorf("color: inherit = %+v, want parent black %+v", got, layout.ColorBlack)
	}
}

func TestCSSWideKeywordsColor(t *testing.T) {
	red := layout.RGB(1, 0, 0)
	blue := layout.RGB(0, 0, 1)
	cases := []struct {
		value string
		want  layout.Color
	}{
		{"inherit", red},
		{"INHERIT", red},
		{"unset", red},        // color is inherited: unset == inherit
		{"currentColor", red}, // on color itself, currentColor == inherit
		{"initial", layout.ColorBlack},
		{"inherit !important", red}, // importance is stripped before apply
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			src := `<html><head><style>
				span { color: #0000ff; }
				.p { color: #ff0000; }
				.p span.k { color: ` + tc.value + `; }
			</style></head><body>
				<div class="p"><span id="t" class="k">x</span></div>
			</body></html>`
			got := styleByID(t, src, "t").Color
			if got == blue || got != tc.want {
				t.Errorf("color: %s = %+v, want %+v", tc.value, got, tc.want)
			}
		})
	}
}

func TestCSSWideKeywordsFontWeight(t *testing.T) {
	cases := []struct {
		value string
		want  int
	}{
		{"inherit", 300},
		{"unset", 300},
		{"initial", 400},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			// <b> carries a bold UA default; the keyword must beat it.
			src := `<html><head><style>
				.p { font-weight: 300; }
				.p b { font-weight: ` + tc.value + `; }
			</style></head><body>
				<div class="p"><b id="t">x</b></div>
			</body></html>`
			if got := styleByID(t, src, "t").FontWeight; got != tc.want {
				t.Errorf("font-weight: %s = %d, want %d", tc.value, got, tc.want)
			}
		})
	}
}

func TestCSSWideKeywordsFontSize(t *testing.T) {
	cases := []struct {
		value string
		want  float64
	}{
		{"inherit", 9},
		{"unset", 9},
		{"initial", 12}, // medium = 16px = 12pt
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			// <h1> carries a 24pt UA default; the keyword must beat it.
			src := `<html><head><style>
				.p { font-size: 12px; }
				.p h1 { font-size: ` + tc.value + `; }
			</style></head><body>
				<div class="p"><h1 id="t">x</h1></div>
			</body></html>`
			if got := styleByID(t, src, "t").FontSize; math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("font-size: %s = %v, want %v", tc.value, got, tc.want)
			}
		})
	}
}

func TestCSSWideKeywordsFontStyleAndFamily(t *testing.T) {
	// The parent is italic and a less specific rule sets the <em> back to
	// normal, so only a real inherit yields italic (a parser that coerces
	// the keyword to normal, or drops it, does not).
	src := `<html><head><style>
		.p { font-style: italic; font-family: Courier; }
		em { font-style: normal; font-family: Times; }
		.p em { font-style: inherit; font-family: inherit; }
	</style></head><body>
		<div class="p"><em id="t">x</em></div>
	</body></html>`
	s := styleByID(t, src, "t")
	if s.FontStyle != "italic" {
		t.Errorf("font-style: inherit = %q, want %q", s.FontStyle, "italic")
	}
	if s.FontFamily != "courier" {
		t.Errorf("font-family: inherit = %q, want %q", s.FontFamily, "courier")
	}
}

func TestCSSWideKeywordsBackgroundColor(t *testing.T) {
	red := layout.RGB(1, 0, 0)
	t.Run("inherit", func(t *testing.T) {
		src := `<html><head><style>
			span { background-color: #0000ff; }
			.p { background-color: #ff0000; }
			.p span { background-color: inherit; }
		</style></head><body>
			<div class="p"><span id="t">x</span></div>
		</body></html>`
		got := styleByID(t, src, "t").BackgroundColor
		if got == nil || *got != red {
			t.Errorf("background-color: inherit = %v, want %+v", got, red)
		}
	})
	for _, kw := range []string{"initial", "unset"} {
		t.Run(kw, func(t *testing.T) {
			// background-color is not inherited: unset == initial == transparent.
			src := `<html><head><style>
				span { background-color: #0000ff; }
				.p { background-color: #ff0000; }
				.p span { background-color: ` + kw + `; }
			</style></head><body>
				<div class="p"><span id="t">x</span></div>
			</body></html>`
			if got := styleByID(t, src, "t").BackgroundColor; got != nil {
				t.Errorf("background-color: %s = %+v, want transparent (nil)", kw, *got)
			}
		})
	}
}

func TestCSSWideKeywordsLineHeight(t *testing.T) {
	src := `<html><head><style>
		p { line-height: 3; }
		.p { line-height: 1.5; }
		.p p { line-height: inherit; }
	</style></head><body>
		<div class="p"><p id="t">x</p></div>
	</body></html>`
	if got := styleByID(t, src, "t").LineHeight; math.Abs(got-1.5) > 1e-9 {
		t.Errorf("line-height: inherit = %v, want 1.5", got)
	}
}

func TestCSSWideKeywordsTextAlign(t *testing.T) {
	src := `<html><head><style>
		p { text-align: center; }
		.p { text-align: right; }
		.p p { text-align: inherit; }
	</style></head><body>
		<div class="p"><p id="t">x</p></div>
	</body></html>`
	if got := styleByID(t, src, "t").TextAlign; got != layout.AlignRight {
		t.Errorf("text-align: inherit = %v, want right", got)
	}
}

// TestCSSWideKeywordsFontSizeInitialHonorsDefaultFontSize pins that
// `font-size: initial` resets to the configured root size
// (Options.DefaultFontSize, folio's `medium`), not a hard-coded 12pt.
func TestCSSWideKeywordsFontSizeInitialHonorsDefaultFontSize(t *testing.T) {
	src := `<html><head><style>
		.p { font-size: 30px; }
		.p h1 { font-size: initial; }
		.p h2 { font-size: unset; }
	</style></head><body>
		<div class="p"><h1 id="i">x</h1><h2 id="u">y</h2></div>
	</body></html>`
	opts := &Options{DefaultFontSize: 10}
	if got := styleByIDWithOptions(t, src, "i", opts).FontSize; math.Abs(got-10) > 1e-9 {
		t.Errorf("font-size: initial = %v, want DefaultFontSize 10", got)
	}
	// unset on an inherited property is inherit: 30px = 22.5pt.
	if got := styleByIDWithOptions(t, src, "u", opts).FontSize; math.Abs(got-22.5) > 1e-9 {
		t.Errorf("font-size: unset = %v, want inherited 22.5", got)
	}
}

func TestCSSWideKeywordsInlineStyle(t *testing.T) {
	src := `<html><head><style>
		a { color: #0000ff; }
	</style></head><body>
		<div style="color: #ff0000"><a id="t" href="#" style="color: inherit">x</a></div>
	</body></html>`
	if got, want := styleByID(t, src, "t").Color, layout.RGB(1, 0, 0); got != want {
		t.Errorf("inline color: inherit = %+v, want %+v", got, want)
	}
}

// An !important stylesheet keyword outranks a normal inline declaration.
func TestCSSWideKeywordsImportantBeatsInline(t *testing.T) {
	src := `<html><head><style>
		.p { color: #00ff00; }
		a { color: inherit !important; }
	</style></head><body>
		<div class="p"><a id="t" href="#" style="color: #ff0000">x</a></div>
	</body></html>`
	if got, want := styleByID(t, src, "t").Color, layout.RGB(0, 1, 0); got != want {
		t.Errorf("color: inherit !important vs inline red = %+v, want %+v", got, want)
	}
}

// A keyword produced by var() substitution, such as the fallback of
// `var(--undefined, inherit)`, is resolved like a declared one.
func TestCSSWideKeywordsViaVarFallback(t *testing.T) {
	src := `<html><head><style>
		span { color: #0000ff; }
		.p { color: #ff0000; }
		.p span { color: var(--undefined, inherit); }
	</style></head><body>
		<div class="p"><span id="t">x</span></div>
	</body></html>`
	if got, want := styleByID(t, src, "t").Color, layout.RGB(1, 0, 0); got != want {
		t.Errorf("color: var(--undefined, inherit) = %+v, want %+v", got, want)
	}
}
