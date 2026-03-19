// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"unicode"
)

// Hyphenator implements the Liang-Knuth hyphenation algorithm used in TeX.
// It uses a set of patterns to determine valid hyphenation points in words.
type Hyphenator struct {
	patterns map[string][]int
}

// NewHyphenator creates a Hyphenator from a list of TeX-format patterns.
// Each pattern is a string of letters interspersed with digits indicating
// hyphenation priorities (e.g., ".hy1p" means priority 1 between y and p
// after word start).
func NewHyphenator(patterns []string) *Hyphenator {
	h := &Hyphenator{
		patterns: make(map[string][]int, len(patterns)),
	}
	for _, p := range patterns {
		letters, values := parsePattern(p)
		h.patterns[letters] = values
	}
	return h
}

// parsePattern extracts the letter key and the digit values from a TeX
// hyphenation pattern. For example, ".hy1p" yields letters=".hyp" and
// values=[0,0,0,1,0]. The values slice has len(letters)+1 entries,
// one for each inter-letter position (including before and after).
func parsePattern(pat string) (string, []int) {
	var letters []rune
	var values []int

	for _, r := range pat {
		if r >= '0' && r <= '9' {
			// Digit: set the value at the current position.
			for len(values) <= len(letters) {
				values = append(values, 0)
			}
			values[len(letters)] = int(r - '0')
		} else {
			letters = append(letters, r)
		}
	}
	// Ensure values has len(letters)+1 entries.
	for len(values) <= len(letters) {
		values = append(values, 0)
	}
	return string(letters), values
}

// Hyphenate returns the valid hyphenation break indices for a word.
// Each index i means a hyphen can be inserted after word[i-1] (i.e.,
// the word can be split into word[:i] and word[i:]).
// Short words (fewer than 4 characters) return no break points.
func (h *Hyphenator) Hyphenate(word string) []int {
	runes := []rune(strings.ToLower(word))
	if len(runes) < 4 {
		return nil
	}

	// Wrap word in dots: ".word."
	wrapped := make([]rune, 0, len(runes)+2)
	wrapped = append(wrapped, '.')
	wrapped = append(wrapped, runes...)
	wrapped = append(wrapped, '.')

	// Levels array: one entry per inter-character position in wrapped.
	levels := make([]int, len(wrapped)+1)

	// For each starting position in wrapped, try all substrings.
	for i := 0; i < len(wrapped); i++ {
		for j := i + 1; j <= len(wrapped); j++ {
			sub := string(wrapped[i:j])
			if vals, ok := h.patterns[sub]; ok {
				for k, v := range vals {
					pos := i + k
					if pos < len(levels) && v > levels[pos] {
						levels[pos] = v
					}
				}
			}
		}
	}

	// Extract break points. The levels correspond to positions in
	// the wrapped word ".word.". Position 0 is before '.', position 1
	// is between '.' and first letter, etc. Valid break positions in
	// the original word are from index 2 (after at least 2 chars) to
	// len(runes) (before at least 2 chars from end), mapped from
	// wrapped positions 2..len(runes).
	// An odd level means a break is allowed.
	var breaks []int
	for i := 2; i < len(runes); i++ {
		// Position in wrapped is i+1 (offset by the leading dot).
		pos := i + 1
		if pos < len(levels) && levels[pos]%2 == 1 {
			// Enforce minimum 2 chars from each end.
			if i >= 2 && i <= len(runes)-2 {
				breaks = append(breaks, i)
			}
		}
	}
	return breaks
}

// DefaultHyphenator returns the shared US English hyphenator instance.
// It is initialized lazily on first call.
func DefaultHyphenator() *Hyphenator {
	initDefaultHyphenator()
	return defaultHyphenator
}

// isAlphaWord checks if a string contains only letters (no digits,
// punctuation, etc.). Used to decide if hyphenation should be attempted.
func isAlphaWord(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}
