// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"strings"
	"testing"
)

func TestHyphenateHyphenation(t *testing.T) {
	h := DefaultHyphenator()
	breaks := h.Hyphenate("hyphenation")
	// "hyphenation" should break as hy-phen-ation or similar.
	// We verify known break points exist.
	if len(breaks) == 0 {
		t.Fatal("expected break points for 'hyphenation', got none")
	}
	t.Logf("hyphenation breaks: %v", breaks)
	// At minimum, there should be a break between "hy" and "phen" (index 2).
	found2 := false
	for _, b := range breaks {
		if b == 2 {
			found2 = true
		}
	}
	if !found2 {
		t.Errorf("expected break at index 2 (hy-phenation), got %v", breaks)
	}
}

func TestHyphenateAlgorithm(t *testing.T) {
	h := DefaultHyphenator()
	breaks := h.Hyphenate("algorithm")
	if len(breaks) == 0 {
		t.Fatal("expected break points for 'algorithm', got none")
	}
	t.Logf("algorithm breaks: %v", breaks)
	// "algorithm" should have at least one valid break (e.g., al-go-rithm).
	// Verify all break points are within valid range.
	runes := []rune("algorithm")
	for _, b := range breaks {
		if b < 2 || b > len(runes)-2 {
			t.Errorf("break point %d out of valid range [2, %d]", b, len(runes)-2)
		}
	}
}

func TestHyphenateShortWords(t *testing.T) {
	h := DefaultHyphenator()
	for _, word := range []string{"a", "to", "the", "an", "it"} {
		breaks := h.Hyphenate(word)
		if len(breaks) != 0 {
			t.Errorf("short word %q should have no breaks, got %v", word, breaks)
		}
	}
}

func TestHyphenateUnknownWords(t *testing.T) {
	h := DefaultHyphenator()
	// Made-up words should still get reasonable behavior (no panics,
	// break points within valid range if any).
	for _, word := range []string{"zxywvuts", "abcdefgh", "qrstuvwx"} {
		breaks := h.Hyphenate(word)
		runes := []rune(word)
		for _, b := range breaks {
			if b < 2 || b > len(runes)-2 {
				t.Errorf("word %q: break point %d out of valid range", word, b)
			}
		}
	}
}

func TestHyphenateCommonWords(t *testing.T) {
	h := DefaultHyphenator()
	// Test several common English words that have well-known hyphenation.
	tests := []struct {
		word     string
		minBreak int // at least this many break points expected
	}{
		{"computer", 1},     // com-put-er
		{"information", 2},  // in-for-ma-tion
		{"programming", 1},  // pro-gram-ming
		{"typography", 2},   // ty-pog-ra-phy
		{"celebration", 2},  // cel-e-bra-tion
		{"understanding", 2}, // un-der-stand-ing
	}
	for _, tt := range tests {
		breaks := h.Hyphenate(tt.word)
		if len(breaks) < tt.minBreak {
			t.Errorf("%q: expected at least %d break(s), got %d: %v",
				tt.word, tt.minBreak, len(breaks), breaks)
		}
	}
}

func TestParsePattern(t *testing.T) {
	letters, values := parsePattern(".hy1p")
	if letters != ".hyp" {
		t.Errorf("expected letters '.hyp', got %q", letters)
	}
	// Values: positions 0(.),1(h),2(y),3(p) → [0,0,0,1,0]
	// The '1' is between y and p, which is position 3.
	expected := []int{0, 0, 0, 1, 0}
	if len(values) != len(expected) {
		t.Fatalf("expected %d values, got %d: %v", len(expected), len(values), values)
	}
	for i, v := range expected {
		if values[i] != v {
			t.Errorf("values[%d]: expected %d, got %d", i, v, values[i])
		}
	}
}

func TestParsePatternMultiDigit(t *testing.T) {
	letters, values := parsePattern("a2b3c")
	if letters != "abc" {
		t.Errorf("expected letters 'abc', got %q", letters)
	}
	// a 2 b 3 c → positions: [0, 2, 3, 0]
	expected := []int{0, 2, 3, 0}
	if len(values) != len(expected) {
		t.Fatalf("expected %d values, got %d: %v", len(expected), len(values), values)
	}
	for i, v := range expected {
		if values[i] != v {
			t.Errorf("values[%d]: expected %d, got %d", i, v, values[i])
		}
	}
}

func TestHyphenatorVisual(t *testing.T) {
	// Helper to visualize hyphenation for debugging.
	h := DefaultHyphenator()
	words := []string{"hyphenation", "algorithm", "computer", "programming", "typography"}
	for _, word := range words {
		breaks := h.Hyphenate(word)
		runes := []rune(word)
		var parts []string
		prev := 0
		for _, b := range breaks {
			parts = append(parts, string(runes[prev:b]))
			prev = b
		}
		parts = append(parts, string(runes[prev:]))
		t.Logf("%s → %s", word, strings.Join(parts, "-"))
	}
}

func TestDefaultHyphenatorNotNil(t *testing.T) {
	h := DefaultHyphenator()
	if h == nil {
		t.Fatal("DefaultHyphenator() returned nil")
	}
	if len(h.patterns) == 0 {
		t.Fatal("DefaultHyphenator() has no patterns loaded")
	}
	t.Logf("loaded %d patterns", len(h.patterns))
}

func TestIsAlphaWord(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"hello", true},
		{"Hello", true},
		{"hello-world", false},
		{"don't", false},
		{"123", false},
		{"abc123", false},
		{"", true},
	}
	for _, tt := range tests {
		got := isAlphaWord(tt.input)
		if got != tt.want {
			t.Errorf("isAlphaWord(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func BenchmarkHyphenate(b *testing.B) {
	h := DefaultHyphenator()
	words := []string{"hyphenation", "algorithm", "computer", "programming", "typography"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h.Hyphenate(words[i%len(words)])
	}
}

func BenchmarkDefaultHyphenatorInit(b *testing.B) {
	// Benchmark pattern parsing (re-create each time).
	lines := splitPatternLines(enUSPatterns)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewHyphenator(lines)
	}
}

func ExampleHyphenator_Hyphenate() {
	h := DefaultHyphenator()
	breaks := h.Hyphenate("hyphenation")
	runes := []rune("hyphenation")
	var parts []string
	prev := 0
	for _, b := range breaks {
		parts = append(parts, string(runes[prev:b]))
		prev = b
	}
	parts = append(parts, string(runes[prev:]))
	fmt.Println(strings.Join(parts, "-"))
}
