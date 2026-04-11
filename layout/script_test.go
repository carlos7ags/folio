// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "testing"

func TestScriptOfSamples(t *testing.T) {
	cases := []struct {
		name string
		r    rune
		want Script
	}{
		{"Latin ASCII", 'a', ScriptLatin},
		{"Latin upper", 'Z', ScriptLatin},
		{"Arabic alef", '\u0627', ScriptArabic},
		{"Hebrew shin", '\u05E9', ScriptHebrew},
		{"Devanagari ka", '\u0915', ScriptDevanagari},
		{"Bengali ka", '\u0995', ScriptBengali},
		{"Tamil ka", '\u0B95', ScriptTamil},
		{"Thai ko kai", '\u0E01', ScriptThai},
		{"Han Chinese", '\u4E2D', ScriptHan},
		{"Hiragana a", '\u3042', ScriptHiragana},
		{"Katakana a", '\u30A2', ScriptKatakana},
		{"Hangul han", '\uD55C', ScriptHangul},
		{"Cyrillic a", '\u0430', ScriptCyrillic},
		{"Greek alpha", '\u03B1', ScriptGreek},
		{"ASCII digit", '1', ScriptCommon},
		{"ASCII space", ' ', ScriptCommon},
		{"ASCII punct", '.', ScriptCommon},
		{"combining acute", '\u0301', ScriptCommon},
		{"Latin-1 e-acute precomposed", '\u00E9', ScriptLatin},
	}
	for _, tc := range cases {
		if got := ScriptOf(tc.r); got != tc.want {
			t.Errorf("%s: ScriptOf(%U) = %d, want %d", tc.name, tc.r, got, tc.want)
		}
	}
}

func TestSegmentByScriptEmpty(t *testing.T) {
	if runs := SegmentByScript(""); len(runs) != 0 {
		t.Errorf("empty input: got %d runs, want 0", len(runs))
	}
}

func TestSegmentByScriptPureLatin(t *testing.T) {
	runs := SegmentByScript("hello")
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].Script != ScriptLatin {
		t.Errorf("script: got %d, want ScriptLatin", runs[0].Script)
	}
	if runs[0].Start != 0 || runs[0].End != len("hello") {
		t.Errorf("range: got [%d,%d), want [0,5)", runs[0].Start, runs[0].End)
	}
}

func TestSegmentByScriptLatinThenArabic(t *testing.T) {
	// "hello" followed directly by Arabic "مرحبا" (no space between).
	s := "hello\u0645\u0631\u062D\u0628\u0627"
	runs := SegmentByScript(s)
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d: %+v", len(runs), runs)
	}
	if runs[0].Script != ScriptLatin {
		t.Errorf("run[0] script: got %d, want Latin", runs[0].Script)
	}
	if runs[1].Script != ScriptArabic {
		t.Errorf("run[1] script: got %d, want Arabic", runs[1].Script)
	}
	if runs[0].End != 5 {
		t.Errorf("run[0] end: got %d, want 5 (after 'hello')", runs[0].End)
	}
	if runs[1].Start != 5 {
		t.Errorf("run[1] start: got %d, want 5", runs[1].Start)
	}
	if runs[1].End != len(s) {
		t.Errorf("run[1] end: got %d, want %d", runs[1].End, len(s))
	}
}

func TestSegmentByScriptLatinSpaceArabic(t *testing.T) {
	// "hello مرحبا" — the space is Common and should inherit from its
	// left neighbour (Latin), placing it inside the Latin run.
	s := "hello \u0645\u0631\u062D\u0628\u0627"
	runs := SegmentByScript(s)
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d: %+v", len(runs), runs)
	}
	if runs[0].Script != ScriptLatin {
		t.Errorf("run[0] script: got %d, want Latin", runs[0].Script)
	}
	// Latin run includes the trailing space (offset 6, past the space).
	if runs[0].End != 6 {
		t.Errorf("run[0] end: got %d, want 6 (Latin absorbs the space)", runs[0].End)
	}
	if runs[1].Script != ScriptArabic || runs[1].Start != 6 {
		t.Errorf("run[1]: got %+v, want Arabic starting at 6", runs[1])
	}
}

func TestSegmentByScriptCombiningMark(t *testing.T) {
	// "cafe\u0301" — combining acute is Inherited/Common and attaches
	// to the preceding Latin 'e', yielding one Latin run.
	s := "cafe\u0301"
	runs := SegmentByScript(s)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d: %+v", len(runs), runs)
	}
	if runs[0].Script != ScriptLatin {
		t.Errorf("script: got %d, want Latin", runs[0].Script)
	}
	if runs[0].End != len(s) {
		t.Errorf("end: got %d, want %d (full string)", runs[0].End, len(s))
	}
}

func TestSegmentByScriptLeadingCommon(t *testing.T) {
	// " hello" — leading space has no left neighbour, so it inherits
	// from the first real script to the right (Latin).
	s := " hello"
	runs := SegmentByScript(s)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d: %+v", len(runs), runs)
	}
	if runs[0].Script != ScriptLatin {
		t.Errorf("script: got %d, want Latin", runs[0].Script)
	}
	if runs[0].Start != 0 || runs[0].End != len(s) {
		t.Errorf("range: got [%d,%d), want full string", runs[0].Start, runs[0].End)
	}
}

func TestSegmentByScriptAllCommon(t *testing.T) {
	// A whole-Common string (digits + punctuation + spaces) has no real
	// script anywhere, so it emits a single ScriptCommon run.
	s := "123 ."
	runs := SegmentByScript(s)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d: %+v", len(runs), runs)
	}
	if runs[0].Script != ScriptCommon {
		t.Errorf("script: got %d, want Common", runs[0].Script)
	}
	if runs[0].End != len(s) {
		t.Errorf("end: got %d, want %d", runs[0].End, len(s))
	}
}

func TestSegmentByScriptArabicDevanagari(t *testing.T) {
	// Arabic alef followed directly by Devanagari ka. Two runs.
	s := "\u0627\u0915"
	runs := SegmentByScript(s)
	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d: %+v", len(runs), runs)
	}
	if runs[0].Script != ScriptArabic {
		t.Errorf("run[0]: got %d, want Arabic", runs[0].Script)
	}
	if runs[1].Script != ScriptDevanagari {
		t.Errorf("run[1]: got %d, want Devanagari", runs[1].Script)
	}
}
