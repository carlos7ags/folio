// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"reflect"
	"testing"
)

// TestGraphemeBreaksASCII checks the ASCII fast path: every printable
// character is its own cluster, so the break slice is [0, 1, 2, ...].
func TestGraphemeBreaksASCII(t *testing.T) {
	got := GraphemeBreaks("hello")
	want := []int{0, 1, 2, 3, 4, 5}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GraphemeBreaks(%q): got %v, want %v", "hello", got, want)
	}
}

// TestGraphemeBreaksCombiningMark covers GB9 (× Extend): a base
// followed by a non-spacing mark forms a single cluster.
func TestGraphemeBreaksCombiningMark(t *testing.T) {
	got := GraphemeBreaks("e\u0301f")
	want := []int{0, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GraphemeBreaks(%q): got %v, want %v", "e\u0301f", got, want)
	}
}

// TestGraphemeBreaksMultipleMarks verifies GB9 applies repeatedly:
// several combining marks on the same base collapse into one cluster.
func TestGraphemeBreaksMultipleMarks(t *testing.T) {
	got := GraphemeBreaks("a\u0301\u0302b")
	want := []int{0, 5, 6}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GraphemeBreaks(%q): got %v, want %v", "a\u0301\u0302b", got, want)
	}
}

// TestGraphemeBreaksCRLF covers GB3: CR and LF stay in one cluster.
func TestGraphemeBreaksCRLF(t *testing.T) {
	got := GraphemeBreaks("a\r\nb")
	want := []int{0, 1, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GraphemeBreaks(%q): got %v, want %v", "a\r\nb", got, want)
	}
}

// TestGraphemeBreaksHangulLV covers GB6: a leading jamo (L) followed
// by a vowel jamo (V) clusters as an LV syllable.
func TestGraphemeBreaksHangulLV(t *testing.T) {
	got := GraphemeBreaks("\u1100\u1161")
	want := []int{0, 6}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GraphemeBreaks Hangul LV: got %v, want %v", got, want)
	}
}

// TestGraphemeBreaksHangulLVT covers GB6+GB7: L, V, and T jamo chain
// into a single LVT cluster.
func TestGraphemeBreaksHangulLVT(t *testing.T) {
	got := GraphemeBreaks("\u1100\u1161\u11A8")
	want := []int{0, 9}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GraphemeBreaks Hangul LVT: got %v, want %v", got, want)
	}
}

// TestGraphemeBreaksRIPair covers GB12/GB13: two Regional_Indicator
// codepoints form a single flag cluster.
func TestGraphemeBreaksRIPair(t *testing.T) {
	got := GraphemeBreaks("\U0001F1E6\U0001F1FA")
	want := []int{0, 8}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GraphemeBreaks RI pair: got %v, want %v", got, want)
	}
}

// TestGraphemeBreaksRITriple confirms GB12/GB13 pair only even-indexed
// RIs: three consecutive RIs cluster as the first pair plus a lone
// third RI, not as a triple.
func TestGraphemeBreaksRITriple(t *testing.T) {
	got := GraphemeBreaks("\U0001F1E6\U0001F1FA\U0001F1E6")
	want := []int{0, 8, 12}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GraphemeBreaks RI triple: got %v, want %v", got, want)
	}
}

// TestGraphemeBreaksZWJEmoji covers GB11: Extended_Pictographic +
// ZWJ + Extended_Pictographic forms a single cluster. The minimal
// Extended_Pictographic table in grapheme.go includes U+1F468 and
// U+1F469 via the 0x1F100..0x1F64F range, so this sequence joins.
func TestGraphemeBreaksZWJEmoji(t *testing.T) {
	got := GraphemeBreaks("\U0001F468\u200D\U0001F469")
	want := []int{0, 11}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GraphemeBreaks ZWJ emoji: got %v, want %v", got, want)
	}
}

// TestGraphemeBreaksEmpty handles the degenerate case: the empty
// string has a single boundary at offset 0 (GB1 with no GB2 follow-up
// because there is no characters between).
func TestGraphemeBreaksEmpty(t *testing.T) {
	got := GraphemeBreaks("")
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GraphemeBreaks empty: got %v, want %v", got, want)
	}
}

// TestGraphemeBreaksSpacingMark covers GB9a: Devanagari base + vowel
// sign clusters together. U+0915 (ka) is 3 UTF-8 bytes; U+093E
// (vowel sign aa) is 3 UTF-8 bytes.
func TestGraphemeBreaksSpacingMark(t *testing.T) {
	got := GraphemeBreaks("\u0915\u093E")
	want := []int{0, 6}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GraphemeBreaks Devanagari SpacingMark: got %v, want %v", got, want)
	}
}

// TestGraphemeCount exercises the streaming counter against the same
// inputs as the boundary-slice tests: the count equals
// len(GraphemeBreaks) - 1 for non-empty strings and zero for the
// empty string.
func TestGraphemeCount(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"hello", 5},
		{"e\u0301f", 2},
		{"a\u0301\u0302b", 2},
		{"a\r\nb", 3},
		{"\u1100\u1161", 1},
		{"\u1100\u1161\u11A8", 1},
		{"\U0001F1E6\U0001F1FA", 1},
		{"\U0001F1E6\U0001F1FA\U0001F1E6", 2},
		{"\U0001F468\u200D\U0001F469", 1},
		{"\u0915\u093E", 1},
	}
	for _, c := range cases {
		if got := GraphemeCount(c.in); got != c.want {
			t.Errorf("GraphemeCount(%q): got %d, want %d", c.in, got, c.want)
		}
	}
}

// TestNextGraphemeBreak walks each sample string cluster by cluster
// using the streaming helper and verifies the sequence of boundaries
// matches GraphemeBreaks exactly.
func TestNextGraphemeBreak(t *testing.T) {
	inputs := []string{
		"hello",
		"e\u0301f",
		"a\u0301\u0302b",
		"a\r\nb",
		"\u1100\u1161",
		"\u1100\u1161\u11A8",
		"\U0001F1E6\U0001F1FA",
		"\U0001F1E6\U0001F1FA\U0001F1E6",
		"\U0001F468\u200D\U0001F469",
		"\u0915\u093E",
	}
	for _, s := range inputs {
		want := GraphemeBreaks(s)
		// Walk boundaries via NextGraphemeBreak starting at each
		// previous boundary. The first entry in want is 0 (GB1);
		// subsequent entries come from the streaming helper.
		got := []int{0}
		for i := 0; i < len(s); {
			i = NextGraphemeBreak(s, i)
			got = append(got, i)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("NextGraphemeBreak walk(%q): got %v, want %v", s, got, want)
		}
	}
}
