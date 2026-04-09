// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"reflect"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestIsCJKIdeograph(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
		name string
	}{
		{0x4E00, true, "first CJK Unified Ideograph"},
		{0x9FFF, true, "last CJK Unified Ideograph"},
		{0x4E2D, true, "CJK character: middle/center"},
		{0x3400, true, "first Extension A"},
		{0x4DBF, true, "last Extension A"},
		{0xF900, true, "first CJK Compatibility Ideograph"},
		{0xFAFF, true, "last CJK Compatibility Ideograph"},
		{0x20000, true, "first Extension B"},
		{0x2A6DF, true, "last Extension B"},
		{0x2A700, true, "first Extension C"},
		{0x2B73F, true, "last Extension C"},
		{0x2B740, true, "first Extension D"},
		{0x2B81F, true, "last Extension D"},
		{0x2B820, true, "first Extension E"},
		{0x2CEAF, true, "last Extension F"},
		{'A', false, "Latin letter"},
		{'1', false, "digit"},
		{0x3041, false, "hiragana a (not ideograph)"},
	}
	for _, tt := range tests {
		if got := isCJKIdeograph(tt.r); got != tt.want {
			t.Errorf("isCJKIdeograph(%U) [%s] = %v, want %v", tt.r, tt.name, got, tt.want)
		}
	}
}

func TestIsHiragana(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
		name string
	}{
		{0x3041, true, "hiragana small a"},
		{0x3042, true, "hiragana a"},
		{0x309F, true, "last hiragana"},
		{0x3040, true, "first hiragana block"},
		{0x30A0, false, "katakana (not hiragana)"},
		{'a', false, "Latin letter"},
	}
	for _, tt := range tests {
		if got := isHiragana(tt.r); got != tt.want {
			t.Errorf("isHiragana(%U) [%s] = %v, want %v", tt.r, tt.name, got, tt.want)
		}
	}
}

func TestIsKatakana(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
		name string
	}{
		{0x30A0, true, "first katakana block"},
		{0x30AB, true, "katakana ka"},
		{0x30FF, true, "last katakana"},
		{0x31F0, true, "first katakana phonetic ext"},
		{0x31FF, true, "last katakana phonetic ext"},
		{0xFF65, true, "first halfwidth katakana"},
		{0xFF9F, true, "last halfwidth katakana"},
		{0x3042, false, "hiragana (not katakana)"},
	}
	for _, tt := range tests {
		if got := isKatakana(tt.r); got != tt.want {
			t.Errorf("isKatakana(%U) [%s] = %v, want %v", tt.r, tt.name, got, tt.want)
		}
	}
}

func TestIsHangul(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
		name string
	}{
		{0xAC00, true, "first Hangul syllable (ga)"},
		{0xD7AF, true, "last Hangul syllable"},
		{0xD7A3, true, "common Hangul syllable (hih)"},
		{0x1100, true, "first Hangul Jamo"},
		{0x11FF, true, "last Hangul Jamo"},
		{0x3130, true, "first Hangul Compatibility Jamo"},
		{0x318F, true, "last Hangul Compatibility Jamo"},
		{0xA960, true, "first Hangul Jamo Extended-A"},
		{0xA97F, true, "last Hangul Jamo Extended-A"},
		{0xD7B0, true, "first Hangul Jamo Extended-B"},
		{0xD7FF, true, "last Hangul Jamo Extended-B"},
		{0x4E00, false, "CJK ideograph (not Hangul)"},
	}
	for _, tt := range tests {
		if got := isHangul(tt.r); got != tt.want {
			t.Errorf("isHangul(%U) [%s] = %v, want %v", tt.r, tt.name, got, tt.want)
		}
	}
}

func TestIsCJKSymbolOrPunct(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
		name string
	}{
		{0x3000, true, "ideographic space"},
		{0x3001, true, "ideographic comma"},
		{0x3002, true, "ideographic full stop"},
		{0x300C, true, "left corner bracket"},
		{0x300D, true, "right corner bracket"},
		{0x303F, true, "last CJK Symbols block"},
		{0xFF01, true, "fullwidth exclamation mark"},
		{0xFF08, true, "fullwidth left paren"},
		{0xFF09, true, "fullwidth right paren"},
		{0xFF60, true, "last fullwidth form"},
		{0xFE30, true, "first CJK compatibility form"},
		{0xFE4F, true, "last CJK compatibility form"},
		{',', false, "ASCII comma (not CJK)"},
		{'(', false, "ASCII paren (not CJK)"},
	}
	for _, tt := range tests {
		if got := isCJKSymbolOrPunct(tt.r); got != tt.want {
			t.Errorf("isCJKSymbolOrPunct(%U) [%s] = %v, want %v", tt.r, tt.name, got, tt.want)
		}
	}
}

func TestIsBopomofo(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
		name string
	}{
		{0x3100, true, "first Bopomofo"},
		{0x3105, true, "Bopomofo B"},
		{0x312F, true, "last Bopomofo"},
		{0x31A0, true, "first Bopomofo Extended"},
		{0x31BF, true, "last Bopomofo Extended"},
		{0x3042, false, "hiragana (not Bopomofo)"},
	}
	for _, tt := range tests {
		if got := isBopomofo(tt.r); got != tt.want {
			t.Errorf("isBopomofo(%U) [%s] = %v, want %v", tt.r, tt.name, got, tt.want)
		}
	}
}

func TestIsCJKRadical(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
		name string
	}{
		{0x2E80, true, "first CJK Radicals Supplement"},
		{0x2EFF, true, "last CJK Radicals Supplement"},
		{0x2F00, true, "first Kangxi Radical"},
		{0x2FDF, true, "last Kangxi Radical"},
		{0x4E00, false, "CJK ideograph (not radical block)"},
	}
	for _, tt := range tests {
		if got := isCJKRadical(tt.r); got != tt.want {
			t.Errorf("isCJKRadical(%U) [%s] = %v, want %v", tt.r, tt.name, got, tt.want)
		}
	}
}

func TestIsCJK(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
		name string
	}{
		{0x4E2D, true, "CJK ideograph"},
		{0x3042, true, "hiragana"},
		{0x30AB, true, "katakana"},
		{0xAC00, true, "hangul"},
		{0x3001, true, "CJK punctuation"},
		{0xFF01, true, "fullwidth form"},
		{0x3105, true, "bopomofo"},
		{0x2F00, true, "kangxi radical"},
		{'A', false, "Latin"},
		{' ', false, "space"},
		{0x0410, false, "Cyrillic"},
	}
	for _, tt := range tests {
		if got := isCJK(tt.r); got != tt.want {
			t.Errorf("isCJK(%U) [%s] = %v, want %v", tt.r, tt.name, got, tt.want)
		}
	}
}

func TestIsCJKOpeningPunct(t *testing.T) {
	openers := []rune{
		0x3008, 0x300A, 0x300C, 0x300E, 0x3010,
		0x3014, 0x3016, 0x3018, 0x301D,
		0xFF08, 0xFF3B, 0xFF5B,
	}
	for _, r := range openers {
		if !isCJKOpeningPunct(r) {
			t.Errorf("isCJKOpeningPunct(%U) = false, want true", r)
		}
	}
	nonOpeners := []rune{0x3009, 0x300D, 0xFF09, 0x4E00, 'A'}
	for _, r := range nonOpeners {
		if isCJKOpeningPunct(r) {
			t.Errorf("isCJKOpeningPunct(%U) = true, want false", r)
		}
	}
}

func TestIsCJKClosingPunct(t *testing.T) {
	closers := []rune{
		0x3001, 0x3002, 0x3009, 0x300B, 0x300D, 0x300F,
		0x3011, 0x3015, 0x3017, 0x3019, 0x301F,
		0xFF09, 0xFF0C, 0xFF0E, 0xFF1A, 0xFF1B,
		0xFF1F, 0xFF01, 0xFF3D, 0xFF5D,
	}
	for _, r := range closers {
		if !isCJKClosingPunct(r) {
			t.Errorf("isCJKClosingPunct(%U) = false, want true", r)
		}
	}
	nonClosers := []rune{0x300C, 0x3010, 0xFF08, 0x4E00, 'Z'}
	for _, r := range nonClosers {
		if isCJKClosingPunct(r) {
			t.Errorf("isCJKClosingPunct(%U) = true, want false", r)
		}
	}
}

func TestIsKinsokuNoStart(t *testing.T) {
	noStart := []rune{
		0x30FC, // prolonged sound mark
		0x3041, 0x3043, 0x3045, 0x3047, 0x3049, // small hiragana
		0x3063, 0x3083, 0x3085, 0x3087, 0x308E,
		0x30A1, 0x30A3, 0x30A5, 0x30A7, 0x30A9, // small katakana
		0x30C3, 0x30E3, 0x30E5, 0x30E7, 0x30EE,
		0x30F5, 0x30F6,
		0x309D, 0x309E, 0x30FD, 0x30FE, // iteration marks
	}
	for _, r := range noStart {
		if !isKinsokuNoStart(r) {
			t.Errorf("isKinsokuNoStart(%U) = false, want true", r)
		}
	}
	allowed := []rune{0x4E00, 0x3042, 0x30AB, 0x300C, 'A'}
	for _, r := range allowed {
		if isKinsokuNoStart(r) {
			t.Errorf("isKinsokuNoStart(%U) = true, want false", r)
		}
	}
}

func TestIsCJKBreakBefore(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
		name string
	}{
		{0x4E2D, true, "ideograph: break before allowed"},
		{0x3042, true, "hiragana: break before allowed"},
		{0x30AB, true, "katakana: break before allowed"},
		{0xAC00, true, "hangul: break before allowed"},
		{0x300C, true, "opening bracket: break before allowed"},
		{0x3001, false, "closing punct (comma): no break before"},
		{0x3002, false, "closing punct (period): no break before"},
		{0xFF09, false, "fullwidth right paren: no break before"},
		{0x30FC, false, "prolonged sound mark: no break before"},
		{0x3063, false, "small tsu: no break before"},
		{0x30C3, false, "katakana small tsu: no break before"},
		{0x309D, false, "hiragana iteration mark: no break before"},
	}
	for _, tt := range tests {
		if got := isCJKBreakBefore(tt.r); got != tt.want {
			t.Errorf("isCJKBreakBefore(%U) [%s] = %v, want %v", tt.r, tt.name, got, tt.want)
		}
	}
}

func TestIsCJKBreakAfter(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
		name string
	}{
		{0x4E2D, true, "ideograph: break after allowed"},
		{0x3042, true, "hiragana: break after allowed"},
		{0x30AB, true, "katakana: break after allowed"},
		{0xAC00, true, "hangul: break after allowed"},
		{0x3001, true, "closing punct (comma): break after allowed"},
		{0x3002, true, "closing punct (period): break after allowed"},
		{0x300C, false, "opening bracket: no break after"},
		{0x3010, false, "opening lenticular bracket: no break after"},
		{0xFF08, false, "fullwidth left paren: no break after"},
	}
	for _, tt := range tests {
		if got := isCJKBreakAfter(tt.r); got != tt.want {
			t.Errorf("isCJKBreakAfter(%U) [%s] = %v, want %v", tt.r, tt.name, got, tt.want)
		}
	}
}

func TestSplitCJKToken(t *testing.T) {
	tests := []struct {
		input string
		want  []string
		name  string
	}{
		{"hello", []string{"hello"}, "pure Latin"},
		{"\u4e16\u754c", []string{"\u4e16", "\u754c"}, "two CJK ideographs"},
		{
			"hello\u4e16\u754ctest",
			[]string{"hello", "\u4e16", "\u754c", "test"},
			"mixed Latin and CJK",
		},
		{
			"\u4e2d\u6587test\u65e5\u672c",
			[]string{"\u4e2d", "\u6587", "test", "\u65e5", "\u672c"},
			"CJK-Latin-CJK",
		},
		{
			"\u3053\u3093\u306b\u3061\u306f",
			[]string{"\u3053", "\u3093", "\u306b", "\u3061", "\u306f"},
			"hiragana: konnichiwa",
		},
		{
			"\u30ab\u30bf\u30ab\u30ca",
			[]string{"\u30ab", "\u30bf", "\u30ab", "\u30ca"},
			"katakana",
		},
		{
			"\ud55c\uad6d\uc5b4",
			[]string{"\ud55c", "\uad6d", "\uc5b4"},
			"hangul: Korean",
		},
		{"", nil, "empty string"},
		{
			"\u4ef7\u683c\uff1a\u00a5100",
			// U+FF1A (fullwidth colon) is closing punct: stays with preceding char.
			[]string{"\u4ef7", "\u683c\uff1a", "\u00a5100"},
			"CJK with fullwidth colon groups with preceding char",
		},
		{
			"\u300c\u4e16\u754c\u300d",
			// U+300C (left bracket) is opening punct: stays with following char.
			// U+300D (right bracket) is closing punct: stays with preceding char.
			[]string{"\u300c\u4e16", "\u754c\u300d"},
			"kinsoku: brackets group with adjacent chars",
		},
		{
			"\u300c\u300c\u4e16\u300d\u300d",
			// Consecutive opening brackets group together with the first ideograph.
			// Consecutive closing brackets group together with the preceding char.
			[]string{"\u300c\u300c\u4e16\u300d\u300d"},
			"kinsoku: nested brackets stay grouped",
		},
		{
			"\u4e16\u3002\u4e16",
			// U+3002 (period) is closing punct: no break before it.
			[]string{"\u4e16\u3002", "\u4e16"},
			"kinsoku: period stays with preceding char",
		},
		{
			"\u4e16\u3001\u4e16",
			// U+3001 (comma) is closing punct: no break before it.
			[]string{"\u4e16\u3001", "\u4e16"},
			"kinsoku: comma stays with preceding char",
		},
	}
	for _, tt := range tests {
		got := splitCJKToken(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitCJKToken(%q) [%s] = %v, want %v", tt.input, tt.name, got, tt.want)
		}
	}
}

func TestSplitWordsCJK(t *testing.T) {
	// splitWords does whitespace splitting only; CJK splitting happens
	// later in breakCJKWords after word measurement.
	tests := []struct {
		input string
		want  []string
		name  string
	}{
		{
			"\u4eca\u65e5\u306f\u4e16\u754c",
			[]string{"\u4eca\u65e5\u306f\u4e16\u754c"},
			"pure CJK stays as single token",
		},
		{
			"hello \u4e16\u754c",
			[]string{"hello", "\u4e16\u754c"},
			"Latin space CJK: two tokens",
		},
		{
			"hello world",
			[]string{"hello", "world"},
			"pure Latin unchanged",
		},
		{
			"\u4e16\u754c\nhello",
			[]string{"\u4e16\u754c", lineBreakMarker, "hello"},
			"CJK with newline",
		},
		{
			"test\u4e2d\u6587 end",
			[]string{"test\u4e2d\u6587", "end"},
			"mixed token stays whole",
		},
	}
	for _, tt := range tests {
		got := splitWords(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitWords(%q) [%s] = %v, want %v", tt.input, tt.name, got, tt.want)
		}
	}
}

func TestBreakCJKWords(t *testing.T) {
	// breakCJKWords splits measured Word entries containing CJK text at
	// character boundaries. Intermediate sub-tokens get SpaceAfter=0;
	// the last sub-token inherits the original word's SpaceAfter.
	w := Word{
		Text:       "\u4eca\u65e5\u306f\u4e16\u754c",
		Width:      50,
		Font:       font.Helvetica,
		FontSize:   12,
		SpaceAfter: 3.5,
	}
	result := breakCJKWords([]Word{w})
	if len(result) != 5 {
		t.Fatalf("expected 5 words, got %d", len(result))
	}
	// All intermediate words should have SpaceAfter=0.
	for i := 0; i < 4; i++ {
		if result[i].SpaceAfter != 0 {
			t.Errorf("word %d (%q): SpaceAfter=%v, want 0", i, result[i].Text, result[i].SpaceAfter)
		}
	}
	// Last word inherits original SpaceAfter.
	if result[4].SpaceAfter != 3.5 {
		t.Errorf("last word SpaceAfter=%v, want 3.5", result[4].SpaceAfter)
	}
	// Each word should be a single character.
	expected := []string{"\u4eca", "\u65e5", "\u306f", "\u4e16", "\u754c"}
	for i, e := range expected {
		if result[i].Text != e {
			t.Errorf("word %d: text=%q, want %q", i, result[i].Text, e)
		}
	}
}

func TestBreakCJKWordsKinsoku(t *testing.T) {
	// Opening bracket groups with following char, closing with preceding.
	w := Word{
		Text:       "\u300c\u4e16\u754c\u300d",
		Width:      50,
		Font:       font.Helvetica,
		FontSize:   12,
		SpaceAfter: 3.0,
	}
	result := breakCJKWords([]Word{w})
	if len(result) != 2 {
		t.Fatalf("expected 2 words for bracketed CJK, got %d: %v", len(result), wordsText(result))
	}
	if result[0].Text != "\u300c\u4e16" {
		t.Errorf("word 0: %q, want %q", result[0].Text, "\u300c\u4e16")
	}
	if result[1].Text != "\u754c\u300d" {
		t.Errorf("word 1: %q, want %q", result[1].Text, "\u754c\u300d")
	}
	if result[0].SpaceAfter != 0 {
		t.Errorf("word 0 SpaceAfter=%v, want 0", result[0].SpaceAfter)
	}
	if result[1].SpaceAfter != 3.0 {
		t.Errorf("word 1 SpaceAfter=%v, want 3.0", result[1].SpaceAfter)
	}
}

func TestBreakCJKWordsLatinUnchanged(t *testing.T) {
	w := Word{
		Text:       "hello",
		Width:      30,
		Font:       font.Helvetica,
		FontSize:   12,
		SpaceAfter: 3.5,
	}
	result := breakCJKWords([]Word{w})
	if len(result) != 1 {
		t.Fatalf("expected 1 word for Latin, got %d", len(result))
	}
	if result[0].Text != "hello" || result[0].SpaceAfter != 3.5 {
		t.Errorf("Latin word modified: %+v", result[0])
	}
}

func wordsText(words []Word) []string {
	var s []string
	for _, w := range words {
		s = append(s, w.Text)
	}
	return s
}

func TestSplitCJKTokenEdgeCases(t *testing.T) {
	tests := []struct {
		input string
		want  []string
		name  string
	}{
		{"\u4e16", []string{"\u4e16"}, "single CJK character"},
		{"a", []string{"a"}, "single Latin character"},
		{
			"\u4e16\u754c\u3002",
			// Period groups with preceding char.
			[]string{"\u4e16", "\u754c\u3002"},
			"CJK with trailing period groups with preceding",
		},
		{
			"\u300c\u4e16\u300d",
			// Opening bracket with next, closing bracket with prev.
			[]string{"\u300c\u4e16\u300d"},
			"single char in brackets stays as one token",
		},
	}
	for _, tt := range tests {
		got := splitCJKToken(tt.input)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("splitCJKToken(%q) [%s] = %v, want %v", tt.input, tt.name, got, tt.want)
		}
	}
}

func TestSplitWordsCJKFullwidthSpace(t *testing.T) {
	// U+3000 (ideographic space) is treated as whitespace by strings.Fields,
	// so CJK characters separated by it become separate whitespace-delimited tokens.
	got := splitWords("\u4e16\u3000\u754c")
	want := []string{"\u4e16", "\u754c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitWords with U+3000 = %v, want %v", got, want)
	}
}

func TestSplitWordsCJKConsecutiveSpaces(t *testing.T) {
	got := splitWords("\u4e16  \u754c  hello")
	want := []string{"\u4e16", "\u754c", "hello"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("splitWords with consecutive spaces = %v, want %v", got, want)
	}
}

func TestKeepAllSkipsCJKBreaking(t *testing.T) {
	// With word-break: keep-all, CJK text should NOT be split at character
	// boundaries. It stays as one word that only breaks at spaces.
	p := NewParagraph("\u4e16\u754c\u4f60\u597d", font.Helvetica, 12)
	p.SetWordBreak("keep-all")
	lines := p.Layout(20)
	// Even at a narrow width, the CJK text stays on one line (may overflow)
	// because keep-all prevents character-level breaking.
	if len(lines) != 1 {
		t.Errorf("keep-all: expected 1 line (no CJK breaking), got %d", len(lines))
	}
}

func TestCJKParagraphLayout(t *testing.T) {
	// Verify that CJK text wraps character-by-character in a narrow width.
	// With a standard font, CJK characters get the .notdef width (~278
	// units in Helvetica). At fontSize 12: ~3.34pt per character.
	// Use width=5 to force wrapping of 4 characters (~13.36pt total).
	p := NewParagraph("\u4e16\u754c\u4f60\u597d", font.Helvetica, 12)
	lines := p.Layout(5)
	if len(lines) < 2 {
		t.Fatalf("expected CJK text to wrap into multiple lines at width=5, got %d line(s)", len(lines))
	}
	// Verify each word is an individual CJK character (kinsoku may group
	// punctuation, but these are pure ideographs).
	for i, line := range lines {
		for _, w := range line.Words {
			runes := []rune(w.Text)
			if len(runes) != 1 {
				t.Errorf("line %d: expected single-character CJK words, got %q (%d runes)", i, w.Text, len(runes))
			}
		}
	}
}
