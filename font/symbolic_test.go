// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"math"
	"testing"
)

func TestStandardEncodeSymbolic(t *testing.T) {
	tests := []struct {
		name string
		f    *Standard
		in   string
		want string
	}{
		// Symbol: α is code 0x61; Latin 'a' has no Symbol glyph and
		// becomes '?' (which Symbol has, at 0x3F).
		{"symbol alpha", Symbol, "\u03B1", "\x61"},
		{"symbol unmapped", Symbol, "\u4E2D\u03B1", "?\x61"},
		// A raw built-in code passes through: 'a' (0x61) draws α.
		{"symbol raw code", Symbol, "a\u03B1", "\x61\x61"},
		{"symbol digits and space", Symbol, "1 + 2", "1 + 2"},
		{"symbol Delta and increment", Symbol, "\u0394\u2206", "\x44\x44"},
		{"symbol Omega and ohm", Symbol, "\u03A9\u2126", "\x57\x57"},
		{"symbol mu and micro", Symbol, "\u03BC\u00B5", "\x6D\x6D"},
		{"symbol minus and hyphen", Symbol, "\u2212-", "\x2D\x2D"},
		{"symbol high codes", Symbol, "\u2264\u221E\u2211", "\xA3\xA5\xE5"},
		// ZapfDingbats: ✔ is code 0x34; a raw code such as '4' passes
		// through, and text with no glyph becomes a space (ZapfDingbats
		// has no '?').
		{"zapf check", ZapfDingbats, "\u2714", "\x34"},
		{"zapf raw code", ZapfDingbats, "4a", "4a"},
		{"zapf unmapped", ZapfDingbats, "\u4E2D\u4E16", "  "},
		{"zapf star and circled one", ZapfDingbats, "\u2605\u2460", "\x48\xAC"},
		{"zapf ornament", ZapfDingbats, "\u2768", "\x80"},
		// Latin fonts are unchanged: WinAnsiEncoding.
		{"helvetica", Helvetica, "a\u00E9\u20AC\u03B1", "a\xE9\x80?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.f.Encode(tt.in); got != tt.want {
				t.Errorf("%s.Encode(%q) = %q, want %q", tt.f.Name(), tt.in, got, tt.want)
			}
		})
	}
}

func TestStandardCanEncode(t *testing.T) {
	tests := []struct {
		f    *Standard
		in   string
		want bool
	}{
		{Symbol, "\u03B1\u03B2 = \u03C0", true},
		{Symbol, "a", true},
		{Symbol, "\u4E2D", false},
		{ZapfDingbats, "\u2714 \u2717", true},
		{ZapfDingbats, "x", true},
		{ZapfDingbats, "\u4E2D", false},
		{Helvetica, "caf\u00E9", true},
		{Helvetica, "\u03B1", false},
	}
	for _, tt := range tests {
		if got := tt.f.CanEncode(tt.in); got != tt.want {
			t.Errorf("%s.CanEncode(%q) = %v, want %v", tt.f.Name(), tt.in, got, tt.want)
		}
		for _, r := range tt.in {
			if !tt.f.CanEncodeRune(r) && tt.want {
				t.Errorf("%s.CanEncodeRune(%q) = false inside encodable %q", tt.f.Name(), r, tt.in)
			}
		}
	}
	var nilFont *Standard
	if !nilFont.CanEncode("abc") || nilFont.CanEncodeRune('\u03B1') {
		t.Error("nil *Standard should fall back to WinAnsi encodability")
	}
}

// TestSymbolicBuiltinRoundTrip checks every glyph of both built-in
// encodings: its Unicode value encodes back to its code, and measures as
// the code's AFM width.
func TestSymbolicBuiltinRoundTrip(t *testing.T) {
	for _, f := range []*Standard{Symbol, ZapfDingbats} {
		table := StandardFontBuiltinEncoding(f.Name())
		widths := StandardFontByteWidths(f.Name())
		n := 0
		for c, r := range table {
			if r == 0 {
				continue
			}
			n++
			s := string(r)
			if got := f.Encode(s); got != string([]byte{byte(c)}) {
				t.Errorf("%s: Encode(%U) = %q, want code 0x%02X", f.Name(), r, got, c)
			}
			if !f.CanEncodeRune(r) {
				t.Errorf("%s: CanEncodeRune(%U) = false", f.Name(), r)
			}
			if got, want := f.MeasureString(s, 1000), float64(widths[c]); math.Abs(got-want) > 1e-9 {
				t.Errorf("%s: MeasureString(%U) = %v, want %v", f.Name(), r, got, want)
			}
		}
		// Symbol encodes 189 glyphs (its "apple" is unencoded);
		// ZapfDingbats encodes 202.
		want := map[string]int{"Symbol": 189, "ZapfDingbats": 202}[f.Name()]
		if n != want {
			t.Errorf("%s: %d encoded glyphs, want %d", f.Name(), n, want)
		}
	}
}

// TestSymbolicWidthsAgreeWithRuneTables cross-checks the generated
// per-code widths against the hand-maintained rune-keyed width tables, so
// the two sources cannot drift apart.
func TestSymbolicWidthsAgreeWithRuneTables(t *testing.T) {
	for _, f := range []*Standard{Symbol, ZapfDingbats} {
		e := f.symbolic()
		runeWidths := standardWidths[f.Name()]
		for r, w := range runeWidths {
			if r == 0 {
				continue
			}
			c, ok := e.fromRune[r]
			if !ok {
				continue // unencoded glyph, e.g. Symbol's apple (U+F8FF)
			}
			if e.widths[c] != w {
				t.Errorf("%s: %U width %d in rune table, %d at code 0x%02X", f.Name(), r, w, e.widths[c], c)
			}
		}
	}
}

func TestSymbolicMeasureMatchesEncodedGlyphs(t *testing.T) {
	// ✔ (code 0x34) is 846 units wide, not the 278 of the '4' the code
	// would be in WinAnsi.
	if got := ZapfDingbats.MeasureString("\u2714", 10); math.Abs(got-8.46) > 1e-9 {
		t.Errorf("ZapfDingbats \u2714 width = %v, want 8.46", got)
	}
	// "中α" draws "?α": 444 + 631.
	if got := Symbol.MeasureString("\u4E2D\u03B1", 10); math.Abs(got-10.75) > 1e-9 {
		t.Errorf("Symbol \"\u4E2D\u03B1\" width = %v, want 10.75", got)
	}
	// Unencodable ZapfDingbats text draws (and measures as) spaces.
	if got := ZapfDingbats.MeasureString("\u4E2D\u4E16", 10); math.Abs(got-5.56) > 1e-9 {
		t.Errorf("ZapfDingbats \"\u4E2D\u4E16\" width = %v, want 5.56", got)
	}
	// Prefix and fit measurements agree with MeasureString.
	text := "\u03B1a\u03B2"
	prefixes := Symbol.MeasureStringPrefixes(text, 10)
	if got, want := prefixes[len(prefixes)-1], Symbol.MeasureString(text, 10); got != want {
		t.Errorf("MeasureStringPrefixes total = %v, want %v", got, want)
	}
	if _, runes, w := Symbol.PrefixFit(text, 10, 1000); runes != 3 || w != prefixes[3] {
		t.Errorf("PrefixFit = (%d runes, %v), want (3, %v)", runes, w, prefixes[3])
	}
}

func TestStandardFontBuiltinEncoding(t *testing.T) {
	if StandardFontBuiltinEncoding("Helvetica") != nil {
		t.Error("Helvetica should have no symbolic built-in encoding")
	}
	sym := StandardFontBuiltinEncoding("Symbol")
	if len(sym) != 256 || sym[0x61] != '\u03B1' || sym[0x44] != '\u0394' {
		t.Fatalf("Symbol built-in encoding wrong: len %d", len(sym))
	}
	sym[0x61] = 'x' // the result is a copy
	if StandardFontBuiltinEncoding("Symbol")[0x61] != '\u03B1' {
		t.Error("StandardFontBuiltinEncoding returned shared storage")
	}
	if z := StandardFontBuiltinEncoding("ZapfDingbats"); z[0x34] != '\u2714' {
		t.Errorf("ZapfDingbats 0x34 = %U, want U+2714", z[0x34])
	}
}
