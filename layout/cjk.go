// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "slices"

// CJK character classification functions for line-breaking support.
//
// These functions identify characters from CJK scripts (Chinese, Japanese,
// Korean) and determine where line breaks are permitted within CJK text.
// The Unicode ranges are derived from the Unicode Standard character block
// definitions (https://www.unicode.org/charts/).

// isCJKIdeograph reports whether r is a CJK Unified Ideograph or a
// compatibility ideograph. This covers the core Han character set used
// across Chinese, Japanese, and Korean writing systems.
func isCJKIdeograph(r rune) bool {
	return (r >= 0x3400 && r <= 0x4DBF) || // CJK Unified Ideographs Extension A
		(r >= 0x4E00 && r <= 0x9FFF) || // CJK Unified Ideographs
		(r >= 0xF900 && r <= 0xFAFF) || // CJK Compatibility Ideographs
		(r >= 0x20000 && r <= 0x2A6DF) || // CJK Unified Ideographs Extension B
		(r >= 0x2A700 && r <= 0x2B73F) || // CJK Unified Ideographs Extension C
		(r >= 0x2B740 && r <= 0x2B81F) || // CJK Unified Ideographs Extension D
		(r >= 0x2B820 && r <= 0x2CEAF) || // CJK Unified Ideographs Extension E
		(r >= 0x2CEB0 && r <= 0x2EBEF) || // CJK Unified Ideographs Extension F
		(r >= 0x2F800 && r <= 0x2FA1F) // CJK Compatibility Ideographs Supplement
}

// isHiragana reports whether r is a Hiragana character.
func isHiragana(r rune) bool {
	return r >= 0x3040 && r <= 0x309F
}

// isKatakana reports whether r is a Katakana character, including
// Katakana Phonetic Extensions and halfwidth Katakana forms.
func isKatakana(r rune) bool {
	return (r >= 0x30A0 && r <= 0x30FF) || // Katakana
		(r >= 0x31F0 && r <= 0x31FF) || // Katakana Phonetic Extensions
		(r >= 0xFF65 && r <= 0xFF9F) // Halfwidth Katakana Forms
}

// isHangul reports whether r is a Hangul (Korean) character, including
// precomposed syllables, Jamo, and compatibility Jamo.
func isHangul(r rune) bool {
	return (r >= 0xAC00 && r <= 0xD7AF) || // Hangul Syllables
		(r >= 0x1100 && r <= 0x11FF) || // Hangul Jamo
		(r >= 0x3130 && r <= 0x318F) || // Hangul Compatibility Jamo
		(r >= 0xA960 && r <= 0xA97F) || // Hangul Jamo Extended-A
		(r >= 0xD7B0 && r <= 0xD7FF) // Hangul Jamo Extended-B
}

// isCJKSymbolOrPunct reports whether r is a CJK symbol, punctuation mark,
// or fullwidth form character.
func isCJKSymbolOrPunct(r rune) bool {
	return (r >= 0x3000 && r <= 0x303F) || // CJK Symbols and Punctuation
		(r >= 0xFF01 && r <= 0xFF60) || // Fullwidth Forms (ASCII variants)
		(r >= 0xFE30 && r <= 0xFE4F) // CJK Compatibility Forms
}

// isBopomofo reports whether r is a Bopomofo (Zhuyin) character, used
// as the phonetic annotation system for Traditional Chinese.
func isBopomofo(r rune) bool {
	return (r >= 0x3100 && r <= 0x312F) || // Bopomofo
		(r >= 0x31A0 && r <= 0x31BF) // Bopomofo Extended
}

// isCJKRadical reports whether r is a CJK radical character. These are
// standalone radical forms used in dictionaries and educational materials.
func isCJKRadical(r rune) bool {
	return (r >= 0x2E80 && r <= 0x2EFF) || // CJK Radicals Supplement
		(r >= 0x2F00 && r <= 0x2FDF) // Kangxi Radicals
}

// isCJK reports whether r belongs to any CJK script or is a CJK-related
// symbol or punctuation character.
func isCJK(r rune) bool {
	return isCJKIdeograph(r) || isHiragana(r) || isKatakana(r) ||
		isHangul(r) || isCJKSymbolOrPunct(r) || isBopomofo(r) ||
		isCJKRadical(r)
}

// CJK opening punctuation: a break is allowed before these characters
// (they start a bracketed group) but NOT after them.
func isCJKOpeningPunct(r rune) bool {
	switch r {
	case 0x3008, // LEFT ANGLE BRACKET
		0x300A, // LEFT DOUBLE ANGLE BRACKET
		0x300C, // LEFT CORNER BRACKET
		0x300E, // LEFT WHITE CORNER BRACKET
		0x3010, // LEFT BLACK LENTICULAR BRACKET
		0x3014, // LEFT TORTOISE SHELL BRACKET
		0x3016, // LEFT WHITE LENTICULAR BRACKET
		0x3018, // LEFT WHITE TORTOISE SHELL BRACKET
		0x301D, // REVERSED DOUBLE PRIME QUOTATION MARK
		0xFF08, // FULLWIDTH LEFT PARENTHESIS
		0xFF3B, // FULLWIDTH LEFT SQUARE BRACKET
		0xFF5B: // FULLWIDTH LEFT CURLY BRACKET
		return true
	}
	return false
}

// CJK closing punctuation: a break is allowed after these characters
// (they end a bracketed group) but NOT before them.
func isCJKClosingPunct(r rune) bool {
	switch r {
	case 0x3001, // IDEOGRAPHIC COMMA
		0x3002, // IDEOGRAPHIC FULL STOP
		0x3009, // RIGHT ANGLE BRACKET
		0x300B, // RIGHT DOUBLE ANGLE BRACKET
		0x300D, // RIGHT CORNER BRACKET
		0x300F, // RIGHT WHITE CORNER BRACKET
		0x3011, // RIGHT BLACK LENTICULAR BRACKET
		0x3015, // RIGHT TORTOISE SHELL BRACKET
		0x3017, // RIGHT WHITE LENTICULAR BRACKET
		0x3019, // RIGHT WHITE TORTOISE SHELL BRACKET
		0x301F, // LOW DOUBLE PRIME QUOTATION MARK
		0xFF09, // FULLWIDTH RIGHT PARENTHESIS
		0xFF0C, // FULLWIDTH COMMA
		0xFF0E, // FULLWIDTH FULL STOP
		0xFF1A, // FULLWIDTH COLON
		0xFF1B, // FULLWIDTH SEMICOLON
		0xFF1F, // FULLWIDTH QUESTION MARK
		0xFF01, // FULLWIDTH EXCLAMATION MARK
		0xFF3D, // FULLWIDTH RIGHT SQUARE BRACKET
		0xFF5D: // FULLWIDTH RIGHT CURLY BRACKET
		return true
	}
	return false
}

// isKinsokuNoStart reports whether r must NOT start a line per kinsoku
// shori rules (JIS X 4051 / W3C JLREQ). This includes small kana,
// prolonged sound marks, and iteration marks, in addition to closing
// punctuation (handled separately by isCJKClosingPunct).
func isKinsokuNoStart(r rune) bool {
	switch r {
	case
		// Prolonged sound mark
		0x30FC, // KATAKANA-HIRAGANA PROLONGED SOUND MARK
		// Small hiragana
		0x3041, // SMALL A
		0x3043, // SMALL I
		0x3045, // SMALL U
		0x3047, // SMALL E
		0x3049, // SMALL O
		0x3063, // SMALL TU
		0x3083, // SMALL YA
		0x3085, // SMALL YU
		0x3087, // SMALL YO
		0x308E, // SMALL WA
		// Small katakana
		0x30A1, // SMALL A
		0x30A3, // SMALL I
		0x30A5, // SMALL U
		0x30A7, // SMALL E
		0x30A9, // SMALL O
		0x30C3, // SMALL TU
		0x30E3, // SMALL YA
		0x30E5, // SMALL YU
		0x30E7, // SMALL YO
		0x30EE, // SMALL WA
		0x30F5, // SMALL KA
		0x30F6, // SMALL KE
		// Iteration marks
		0x309D, // HIRAGANA ITERATION MARK
		0x309E, // HIRAGANA VOICED ITERATION MARK
		0x30FD, // KATAKANA ITERATION MARK
		0x30FE: // KATAKANA VOICED ITERATION MARK
		return true
	}
	return false
}

// isCJKBreakBefore reports whether a line break is permitted immediately
// before r. In CJK text, breaks are generally allowed before ideographs,
// kana, hangul, bopomofo, radicals, and opening punctuation.
// Breaks are NOT allowed before closing punctuation, small kana,
// prolonged sound marks, or iteration marks (kinsoku line-start
// prohibitions).
func isCJKBreakBefore(r rune) bool {
	if isCJKClosingPunct(r) || isKinsokuNoStart(r) {
		return false
	}
	return isCJKIdeograph(r) || isHiragana(r) || isKatakana(r) ||
		isHangul(r) || isCJKOpeningPunct(r) || isBopomofo(r) ||
		isCJKRadical(r)
}

// isCJKBreakAfter reports whether a line break is permitted immediately
// after r. In CJK text, breaks are generally allowed after ideographs,
// kana, hangul, bopomofo, radicals, and closing punctuation.
// Breaks are NOT allowed after opening punctuation (it must stay with
// the following character on the same line).
func isCJKBreakAfter(r rune) bool {
	if isCJKOpeningPunct(r) {
		return false
	}
	return isCJKIdeograph(r) || isHiragana(r) || isKatakana(r) ||
		isHangul(r) || isCJKClosingPunct(r) || isBopomofo(r) ||
		isCJKRadical(r)
}

// splitCJKToken splits a whitespace-free token into sub-tokens at CJK
// break opportunities. Non-CJK runs (e.g. Latin words embedded in CJK
// text) are kept as single tokens. CJK characters are split so the
// word-wrap algorithm can break between them, but kinsoku rules are
// respected: opening punctuation stays grouped with the following
// character, and closing punctuation stays grouped with the preceding
// character. If the token contains no CJK characters, it is returned
// unchanged as a single-element slice.
//
// For example:
//
//	"hello世界test"  -> ["hello", "世", "界", "test"]
//	"「世界」"        -> ["「世", "界」"]
//	"价格：¥100"     -> ["价", "格：", "¥100"]
func splitCJKToken(token string) []string {
	runes := []rune(token)
	if len(runes) == 0 {
		return nil
	}

	// Fast path: no CJK means no splitting needed.
	if !slices.ContainsFunc(runes, isCJK) {
		return []string{token}
	}

	// Walk rune-by-rune. A break is inserted between runes[i-1] and
	// runes[i] when:
	//   - transitioning from non-CJK to CJK (flush Latin run)
	//   - transitioning from CJK to non-CJK (flush CJK run)
	//   - both are CJK and isCJKBreakAfter(prev) && isCJKBreakBefore(curr)
	var result []string
	start := 0

	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		curr := runes[i]
		prevCJK := isCJK(prev)
		currCJK := isCJK(curr)

		shouldBreak := false
		switch {
		case prevCJK && currCJK:
			shouldBreak = isCJKBreakAfter(prev) && isCJKBreakBefore(curr)
		case !prevCJK && currCJK:
			shouldBreak = isCJKBreakBefore(curr)
		case prevCJK && !currCJK:
			shouldBreak = isCJKBreakAfter(prev)
		}

		if shouldBreak {
			result = append(result, string(runes[start:i]))
			start = i
		}
	}
	result = append(result, string(runes[start:]))
	return result
}
