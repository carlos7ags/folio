// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package grapheme

import (
	"reflect"
	"testing"
)

// TestBreaksASCII checks the ASCII fast path: every printable character
// is its own cluster, so the break slice is [0, 1, 2, ...].
func TestBreaksASCII(t *testing.T) {
	got := Breaks("hello")
	want := []int{0, 1, 2, 3, 4, 5}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Breaks(%q): got %v, want %v", "hello", got, want)
	}
}

// TestBreaksCombiningMark covers GB9 (× Extend): a base followed by a
// non-spacing mark forms a single cluster.
func TestBreaksCombiningMark(t *testing.T) {
	got := Breaks("e\u0301f")
	want := []int{0, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Breaks(%q): got %v, want %v", "e\u0301f", got, want)
	}
}

// TestBreaksMultipleMarks verifies GB9 applies repeatedly: several
// combining marks on the same base collapse into one cluster.
func TestBreaksMultipleMarks(t *testing.T) {
	got := Breaks("a\u0301\u0302b")
	want := []int{0, 5, 6}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Breaks(%q): got %v, want %v", "a\u0301\u0302b", got, want)
	}
}

// TestBreaksCRLF covers GB3: CR and LF stay in one cluster.
func TestBreaksCRLF(t *testing.T) {
	got := Breaks("a\r\nb")
	want := []int{0, 1, 3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Breaks(%q): got %v, want %v", "a\r\nb", got, want)
	}
}

// TestBreaksHangulLV covers GB6: a leading jamo (L) followed by a
// vowel jamo (V) clusters as an LV syllable.
func TestBreaksHangulLV(t *testing.T) {
	got := Breaks("\u1100\u1161")
	want := []int{0, 6}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Breaks Hangul LV: got %v, want %v", got, want)
	}
}

// TestBreaksHangulLVT covers GB6+GB7: L, V, and T jamo chain into a
// single LVT cluster.
func TestBreaksHangulLVT(t *testing.T) {
	got := Breaks("\u1100\u1161\u11A8")
	want := []int{0, 9}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Breaks Hangul LVT: got %v, want %v", got, want)
	}
}

// TestBreaksRIPair covers GB12/GB13: two Regional_Indicator codepoints
// form a single flag cluster.
func TestBreaksRIPair(t *testing.T) {
	got := Breaks("\U0001F1E6\U0001F1FA")
	want := []int{0, 8}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Breaks RI pair: got %v, want %v", got, want)
	}
}

// TestBreaksRITriple confirms GB12/GB13 pair only even-indexed RIs:
// three consecutive RIs cluster as the first pair plus a lone third
// RI, not as a triple.
func TestBreaksRITriple(t *testing.T) {
	got := Breaks("\U0001F1E6\U0001F1FA\U0001F1E6")
	want := []int{0, 8, 12}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Breaks RI triple: got %v, want %v", got, want)
	}
}

// TestBreaksZWJEmoji covers GB11: Extended_Pictographic + ZWJ +
// Extended_Pictographic forms a single cluster.
func TestBreaksZWJEmoji(t *testing.T) {
	got := Breaks("\U0001F468\u200D\U0001F469")
	want := []int{0, 11}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Breaks ZWJ emoji: got %v, want %v", got, want)
	}
}

// TestBreaksEmpty handles the degenerate case: the empty string has a
// single boundary at offset 0 (GB1 with no GB2 follow-up because there
// are no characters between).
func TestBreaksEmpty(t *testing.T) {
	got := Breaks("")
	want := []int{0}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Breaks empty: got %v, want %v", got, want)
	}
}

// TestBreaksSpacingMark covers GB9a: Devanagari base + vowel sign
// clusters together. U+0915 (ka) is 3 UTF-8 bytes; U+093E (vowel sign
// aa) is 3 UTF-8 bytes.
func TestBreaksSpacingMark(t *testing.T) {
	got := Breaks("\u0915\u093E")
	want := []int{0, 6}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Breaks Devanagari SpacingMark: got %v, want %v", got, want)
	}
}

// TestCount exercises the streaming counter against the same inputs as
// the boundary-slice tests: the count equals len(Breaks) - 1 for
// non-empty strings and zero for the empty string.
func TestCount(t *testing.T) {
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
		if got := Count(c.in); got != c.want {
			t.Errorf("Count(%q): got %d, want %d", c.in, got, c.want)
		}
	}
}

// TestNextBreak walks each sample string cluster by cluster using the
// streaming helper and verifies the sequence of boundaries matches
// Breaks exactly.
func TestNextBreak(t *testing.T) {
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
		want := Breaks(s)
		got := []int{0}
		for i := 0; i < len(s); {
			i = NextBreak(s, i)
			got = append(got, i)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("NextBreak walk(%q): got %v, want %v", s, got, want)
		}
	}
}

// TestPropertyOfCoreClasses spot-checks the property classifier for
// the codepoints that the MeasureString cluster-advance rule relies
// on: combining marks must be PropExtend, ZWJ must be PropZWJ, and
// Devanagari vowel signs must be PropSpacingMark. These three classes
// drive whether a cluster member contributes advance width or not.
func TestPropertyOfCoreClasses(t *testing.T) {
	cases := []struct {
		r    rune
		want Property
		name string
	}{
		{'e', PropOther, "latin base"},
		{'\u0301', PropExtend, "combining acute"},
		{'\u0302', PropExtend, "combining circumflex"},
		{'\u200D', PropZWJ, "zero width joiner"},
		{'\u093E', PropSpacingMark, "Devanagari vowel sign aa"},
		{'\r', PropCR, "carriage return"},
		{'\n', PropLF, "line feed"},
	}
	for _, c := range cases {
		if got := PropertyOf(c.r); got != c.want {
			t.Errorf("PropertyOf(%s U+%04X): got %d, want %d", c.name, c.r, got, c.want)
		}
	}
}
