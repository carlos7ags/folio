// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"
)

// makeWords creates a slice of Words with the given texts and a fixed width
// per character (6pt) for testing. Only the Text and Width fields are set.
func makeWords(texts ...string) []Word {
	words := make([]Word, len(texts))
	for i, t := range texts {
		words[i] = Word{Text: t, Width: float64(len(t)) * 6}
	}
	return words
}

// reverseTestRunes reverses runes for test assertions.
func reverseTestRunes(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// wordTexts extracts the Text field from each Word for easy comparison.
func wordTexts(words []Word) []string {
	out := make([]string, len(words))
	for i, w := range words {
		out[i] = w.Text
	}
	return out
}

func TestBidiPureHebrew_RTL(t *testing.T) {
	// Two Hebrew words in logical order. Visual order (left-to-right
	// on the page) should be reversed for an RTL paragraph, and each
	// word's glyphs reversed for PDF operators.
	words := makeWords("\u05E9\u05DC\u05D5\u05DD", "\u05E2\u05D5\u05DC\u05DD") // שלום עולם
	visual, dir := resolveLineBidi(words, DirectionAuto)
	if dir != DirectionRTL {
		t.Errorf("direction: got %v, want RTL", dir)
	}
	got := wordTexts(visual)
	// Visual order: second word first (עולם reversed, then שלום reversed).
	want0 := reverseTestRunes(words[1].Text)
	want1 := reverseTestRunes(words[0].Text)
	if got[0] != want0 || got[1] != want1 {
		t.Errorf("visual order: got %v, want [%q, %q]", got, want0, want1)
	}
}

func TestBidiPureEnglish_LTR(t *testing.T) {
	words := makeWords("Hello", "world")
	visual, dir := resolveLineBidi(words, DirectionAuto)
	if dir != DirectionLTR {
		t.Errorf("direction: got %v, want LTR", dir)
	}
	got := wordTexts(visual)
	if got[0] != "Hello" || got[1] != "world" {
		t.Errorf("visual order: got %v, want [Hello, world]", got)
	}
}

func TestBidiMixed_LTRBase(t *testing.T) {
	// "Hello שלום world" with LTR base. The Hebrew word sits visually
	// between the two English words with its glyphs visually reversed.
	words := makeWords("Hello", "\u05E9\u05DC\u05D5\u05DD", "world")
	visual, dir := resolveLineBidi(words, DirectionLTR)
	if dir != DirectionLTR {
		t.Errorf("direction: got %v, want LTR", dir)
	}
	got := wordTexts(visual)
	wantHebrew := reverseTestRunes(words[1].Text)
	if got[0] != "Hello" || got[1] != wantHebrew || got[2] != "world" {
		t.Errorf("visual order: got %v, want [Hello, %q, world]", got, wantHebrew)
	}
}

func TestBidiMixed_RTLBase(t *testing.T) {
	// "שלום Hello עולם" with RTL base (first strong char is Hebrew).
	// Visual order (left-to-right on page): עולם (reversed) Hello שלום (reversed)
	words := makeWords("\u05E9\u05DC\u05D5\u05DD", "Hello", "\u05E2\u05D5\u05DC\u05DD")
	visual, dir := resolveLineBidi(words, DirectionRTL)
	if dir != DirectionRTL {
		t.Errorf("direction: got %v, want RTL", dir)
	}
	got := wordTexts(visual)
	want0 := reverseTestRunes(words[2].Text)
	want2 := reverseTestRunes(words[0].Text)
	if got[0] != want0 || got[1] != "Hello" || got[2] != want2 {
		t.Errorf("visual order: got %v, want [%q, Hello, %q]", got, want0, want2)
	}
}

func TestBidiExplicitLTR_OnHebrew(t *testing.T) {
	// Hebrew text with explicit LTR direction. The first strong char
	// is Hebrew (RTL), but LTR default means the paragraph fallback is
	// LTR. Since Hebrew chars are strong RTL, they still resolve to RTL
	// embedding — so the words still reverse within the RTL run.
	words := makeWords("\u05E9\u05DC\u05D5\u05DD", "\u05E2\u05D5\u05DC\u05DD")
	_, dir := resolveLineBidi(words, DirectionLTR)
	// First strong char is RTL, so resolved direction is RTL regardless
	// of the LTR fallback.
	if dir != DirectionRTL {
		t.Errorf("direction: got %v, want RTL (first strong is Hebrew)", dir)
	}
}

func TestBidiEmpty(t *testing.T) {
	visual, dir := resolveLineBidi(nil, DirectionAuto)
	if len(visual) != 0 {
		t.Errorf("expected empty, got %d words", len(visual))
	}
	if dir != DirectionLTR {
		t.Errorf("empty direction: got %v, want LTR", dir)
	}
}

func TestBidiSingleWord(t *testing.T) {
	words := makeWords("\u05E9\u05DC\u05D5\u05DD") // שלום
	visual, dir := resolveLineBidi(words, DirectionAuto)
	if dir != DirectionRTL {
		t.Errorf("direction: got %v, want RTL", dir)
	}
	want := reverseTestRunes(words[0].Text)
	if len(visual) != 1 || visual[0].Text != want {
		t.Errorf("single word visual text: got %q, want %q", visual[0].Text, want)
	}
}

func TestMirrorBrackets(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"(hello)", ")hello("},
		{"[test]", "]test["},
		{"no brackets", "no brackets"},
		{"", ""},
		{"(a[b]c)", ")a]b[c("},
	}
	for _, tt := range tests {
		got := mirrorBrackets(tt.in)
		if got != tt.want {
			t.Errorf("mirrorBrackets(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestMirrorBracketsAppliedToRTLWords(t *testing.T) {
	// A Hebrew "word" containing parentheses should have them mirrored
	// and reversed after bidi reordering, per UAX #9 rule L4.
	words := makeWords("(\u05E9\u05DC\u05D5\u05DD)") // (שלום)
	visual, _ := resolveLineBidi(words, DirectionRTL)
	if len(visual) != 1 {
		t.Fatalf("expected 1 word, got %d", len(visual))
	}
	want := "(" + reverseTestRunes("\u05E9\u05DC\u05D5\u05DD") + ")"
	if visual[0].Text != want {
		t.Errorf("brackets mirrored and reversed: got %q, want %q", visual[0].Text, want)
	}
}

func TestBidiInlineBlockPreserved(t *testing.T) {
	// An InlineBlock word (e.g. an inline image) between two Hebrew words
	// must not be dropped during bidi reordering. It should appear in the
	// visual output between the two reversed text words.
	words := []Word{
		{Text: "\u05E9\u05DC\u05D5\u05DD", Width: 40}, // שלום
		{Text: "", Width: 12, InlineBlock: &Div{}},    // inline image
		{Text: "\u05E2\u05D5\u05DC\u05DD", Width: 40}, // עולם
	}
	visual, _ := resolveLineBidi(words, DirectionRTL)
	if len(visual) != 3 {
		t.Fatalf("expected 3 words (including inline), got %d", len(visual))
	}
	// All three words should be present — the critical assertion is that
	// the InlineBlock was not silently dropped.
	foundInline := false
	foundShalom := false
	foundOlam := false
	wantShalom := reverseTestRunes("\u05E9\u05DC\u05D5\u05DD")
	wantOlam := reverseTestRunes("\u05E2\u05D5\u05DC\u05DD")
	for _, w := range visual {
		if w.InlineBlock != nil {
			foundInline = true
		}
		if w.Text == wantShalom {
			foundShalom = true
		}
		if w.Text == wantOlam {
			foundOlam = true
		}
	}
	if !foundInline {
		t.Error("InlineBlock word was dropped during bidi reordering")
	}
	if !foundShalom || !foundOlam {
		t.Error("text words were dropped during bidi reordering")
	}
	// Both text words should be present, and the inline should sit between them.
	// The exact order depends on the splicing algorithm; the key invariant is
	// all three are present.
}

func TestBidiWhitespaceOnlyRespectsBase(t *testing.T) {
	// A line with only whitespace words should respect the base direction
	// hint rather than always returning LTR.
	words := makeWords(" ", " ")
	_, dir := resolveLineBidi(words, DirectionRTL)
	if dir != DirectionRTL {
		t.Errorf("whitespace-only with RTL base: got %v, want RTL", dir)
	}
}

func TestSplitMixedBidiWord(t *testing.T) {
	// "מחיר42" has Hebrew + digits → should split at the transition.
	w := Word{Text: "\u05DE\u05D7\u05D9\u05E842", Width: 60, SpaceAfter: 5}
	subs := splitMixedBidiWord(w)
	if subs == nil {
		t.Fatal("expected split, got nil")
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 sub-words, got %d", len(subs))
	}
	if subs[0].Text != "\u05DE\u05D7\u05D9\u05E8" {
		t.Errorf("sub[0]: got %q, want Hebrew part", subs[0].Text)
	}
	if subs[1].Text != "42" {
		t.Errorf("sub[1]: got %q, want '42'", subs[1].Text)
	}
	// SpaceAfter should be on the last sub-word only.
	if subs[0].SpaceAfter != 0 {
		t.Errorf("sub[0].SpaceAfter: got %v, want 0", subs[0].SpaceAfter)
	}
	if subs[1].SpaceAfter != 5 {
		t.Errorf("sub[1].SpaceAfter: got %v, want 5", subs[1].SpaceAfter)
	}
}

func TestSplitMixedBidiWordNoSplit(t *testing.T) {
	// Pure Hebrew — no transition, should return nil.
	w := Word{Text: "\u05E9\u05DC\u05D5\u05DD", Width: 40}
	if subs := splitMixedBidiWord(w); subs != nil {
		t.Errorf("pure Hebrew should not split, got %d sub-words", len(subs))
	}
	// Pure English — no transition.
	w2 := Word{Text: "Hello", Width: 30}
	if subs := splitMixedBidiWord(w2); subs != nil {
		t.Errorf("pure English should not split, got %d sub-words", len(subs))
	}
}

func TestSplitMixedBidiWordInlineBlock(t *testing.T) {
	// InlineBlock words should never split.
	w := Word{Text: "", InlineBlock: &Div{}}
	if subs := splitMixedBidiWord(w); subs != nil {
		t.Error("InlineBlock should not split")
	}
}

// TestSplitMixedBidiWordScriptChange verifies that two characters from
// different scripts at the same bidi level (e.g. Arabic alef + Devanagari
// ka, both bidi-strong but mutually incompatible scripts) split into two
// sub-words. This is the UAX #24 script-segmentation case and is the
// behaviour introduced alongside the ScriptOf / SegmentByScript helpers.
func TestSplitMixedBidiWordScriptChange(t *testing.T) {
	// Arabic alef followed by Devanagari ka.
	w := Word{Text: "\u0627\u0915", Width: 20, SpaceAfter: 4}
	subs := splitMixedBidiWord(w)
	if subs == nil {
		t.Fatal("expected split on script transition, got nil")
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 sub-words, got %d", len(subs))
	}
	if subs[0].Text != "\u0627" {
		t.Errorf("sub[0]: got %q, want Arabic alef", subs[0].Text)
	}
	if subs[1].Text != "\u0915" {
		t.Errorf("sub[1]: got %q, want Devanagari ka", subs[1].Text)
	}
	if subs[0].SpaceAfter != 0 {
		t.Errorf("sub[0].SpaceAfter: got %v, want 0", subs[0].SpaceAfter)
	}
	if subs[1].SpaceAfter != 4 {
		t.Errorf("sub[1].SpaceAfter: got %v, want 4", subs[1].SpaceAfter)
	}
}

// TestSplitMixedBidiWordLatinDevanagari verifies a same-direction
// (both LTR) script change also splits — Latin "test" + Devanagari ka.
func TestSplitMixedBidiWordLatinDevanagari(t *testing.T) {
	w := Word{Text: "test\u0915", Width: 30}
	subs := splitMixedBidiWord(w)
	if subs == nil {
		t.Fatal("expected split, got nil")
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 sub-words, got %d", len(subs))
	}
	if subs[0].Text != "test" {
		t.Errorf("sub[0]: got %q, want 'test'", subs[0].Text)
	}
	if subs[1].Text != "\u0915" {
		t.Errorf("sub[1]: got %q, want Devanagari ka", subs[1].Text)
	}
}

// TestSplitMixedBidiWordPureCJK ensures pure Han text passes through
// without splitting (script is uniform, bidi is uniform).
func TestSplitMixedBidiWordPureCJK(t *testing.T) {
	w := Word{Text: "\u4E2D\u6587", Width: 20} // 中文
	if subs := splitMixedBidiWord(w); subs != nil {
		t.Errorf("pure Han should not split, got %d sub-words", len(subs))
	}
}

// TestSplitMixedBidiWordPureArabic ensures pure Arabic passes through
// unchanged (no script transition, no bidi transition).
func TestSplitMixedBidiWordPureArabic(t *testing.T) {
	w := Word{Text: "\u0645\u0631\u062D\u0628\u0627", Width: 30} // مرحبا
	if subs := splitMixedBidiWord(w); subs != nil {
		t.Errorf("pure Arabic should not split, got %d sub-words", len(subs))
	}
}

// TestSplitMixedBidiWordAccentedLatin ensures combining marks on Latin
// text (Inherited script per UAX #24) attach to the Latin run.
func TestSplitMixedBidiWordAccentedLatin(t *testing.T) {
	w := Word{Text: "cafe\u0301", Width: 30}
	if subs := splitMixedBidiWord(w); subs != nil {
		t.Errorf("Latin with combining mark should not split, got %d sub-words", len(subs))
	}
}

// TestSplitMixedBidiWordClusterSnap verifies that a script transition
// that would otherwise land inside a grapheme cluster is suppressed,
// so every emitted sub-word starts on a UAX #29 cluster boundary.
//
// Input: Latin "test" followed by a Devanagari vowel sign (U+093E).
// U+093E is classified as ScriptDevanagari, so the per-rune walker
// would normally split between 't' and the vowel sign. But the vowel
// sign is a SpacingMark (GB9a): it must cluster with the preceding
// base. The splitter has to suppress that split and keep the whole
// word as one sub-word.
//
// Adding a Devanagari base afterward ("test\u093E\u0915") reinstates
// a real cluster-aligned transition: the vowel sign absorbs into the
// preceding Latin sub-word and the split lands at \u0915, which is a
// cluster boundary.
func TestSplitMixedBidiWordClusterSnap(t *testing.T) {
	// Case 1: orphan SpacingMark should suppress the split entirely.
	w := Word{Text: "test\u093E", Width: 30}
	if subs := splitMixedBidiWord(w); subs != nil {
		t.Errorf("orphan SpacingMark should not split, got %d sub-words: %v", len(subs), subs)
	}

	// Case 2: split must land at the real cluster boundary (\u0915),
	// not mid-cluster at \u093E. Every sub-word's text must begin at
	// a grapheme cluster boundary of the original word.
	w2 := Word{Text: "test\u093E\u0915", Width: 30}
	subs := splitMixedBidiWord(w2)
	if subs == nil {
		t.Fatal("expected split at cluster-aligned transition, got nil")
	}
	if len(subs) != 2 {
		t.Fatalf("expected 2 sub-words, got %d: %v", len(subs), subs)
	}
	if subs[0].Text != "test\u093E" {
		t.Errorf("sub[0]: got %q, want \"test\u093E\"", subs[0].Text)
	}
	if subs[1].Text != "\u0915" {
		t.Errorf("sub[1]: got %q, want \"\u0915\"", subs[1].Text)
	}
	// Verify every sub-word boundary aligns with a grapheme cluster
	// break in the original string.
	breaks := GraphemeBreaks(w2.Text)
	breakSet := make(map[int]bool, len(breaks))
	for _, b := range breaks {
		breakSet[b] = true
	}
	offset := 0
	for _, s := range subs {
		if !breakSet[offset] {
			t.Errorf("sub-word boundary at byte %d is not a cluster break (%v)", offset, breaks)
		}
		offset += len(s.Text)
	}
}

func TestBidiNumbersInRTL(t *testing.T) {
	// "שלום 42 עולם" — numbers in an RTL paragraph stay LTR.
	// Visual order (left-to-right): עולם (reversed) 42 שלום (reversed)
	words := makeWords("\u05E9\u05DC\u05D5\u05DD", "42", "\u05E2\u05D5\u05DC\u05DD")
	visual, dir := resolveLineBidi(words, DirectionRTL)
	if dir != DirectionRTL {
		t.Errorf("direction: got %v, want RTL", dir)
	}
	got := wordTexts(visual)
	want0 := reverseTestRunes(words[2].Text)
	want2 := reverseTestRunes(words[0].Text)
	if got[0] != want0 || got[1] != "42" || got[2] != want2 {
		t.Errorf("visual order: got %v, want [%q, 42, %q]", got, want0, want2)
	}
}

// TestBidiRTLRunReversesVisualGlyphsAndPreservesOriginalText verifies that
// pure Arabic and Hebrew words have their visual glyph runes reversed in an RTL run,
// while Word.OriginalText is preserved completely untouched for /ActualText recovery.
func TestBidiRTLRunReversesVisualGlyphsAndPreservesOriginalText(t *testing.T) {
	// Arabic: "سلام"
	origArabic := "\u0633\u0644\u0627\u0645"
	shapedArabic := ShapeArabic(origArabic)
	wordArabic := Word{
		Text:         shapedArabic,
		OriginalText: origArabic,
		Width:        30,
	}

	// Hebrew: "שלום"
	origHebrew := "\u05E9\u05DC\u05D5\u05DD"
	wordHebrew := Word{
		Text:         origHebrew,
		OriginalText: origHebrew,
		Width:        30,
	}

	visual, dir := resolveLineBidi([]Word{wordArabic, wordHebrew}, DirectionRTL)
	if dir != DirectionRTL {
		t.Fatalf("expected DirectionRTL, got %v", dir)
	}
	if len(visual) != 2 {
		t.Fatalf("expected 2 words, got %d", len(visual))
	}

	// Visual order in RTL: second word (Hebrew) first, first word (Arabic) second.
	// Hebrew word check:
	wantHebrewVisual := reverseTestRunes(origHebrew)
	if visual[0].Text != wantHebrewVisual {
		t.Errorf("Hebrew visual Text: got %q, want %q", visual[0].Text, wantHebrewVisual)
	}
	if visual[0].OriginalText != origHebrew {
		t.Errorf("Hebrew OriginalText: got %q, want untouched %q", visual[0].OriginalText, origHebrew)
	}

	// Arabic word check:
	wantArabicVisual := reverseTestRunes(shapedArabic)
	if visual[1].Text != wantArabicVisual {
		t.Errorf("Arabic visual Text: got %q, want %q", visual[1].Text, wantArabicVisual)
	}
	if visual[1].OriginalText != origArabic {
		t.Errorf("Arabic OriginalText: got %q, want untouched %q", visual[1].OriginalText, origArabic)
	}
}

// TestBidiMixedScriptBoundaryTokens verifies that boundary tokens containing
// neutral punctuation and numbers adjacent to RTL words (e.g. "(RC):", "19%")
// do not get their Latin characters or digits flipped.
func TestBidiMixedScriptBoundaryTokens(t *testing.T) {
	// Standalone boundary tokens: "(RC):", "19%", "(Business)"
	tokens := []string{"(RC):", "19%", "(Business)"}
	for _, tok := range tokens {
		w := Word{Text: tok, Width: 20}
		visual, _ := resolveLineBidi([]Word{w}, DirectionRTL)
		if len(visual) != 1 {
			t.Fatalf("%s: expected 1 word, got %d", tok, len(visual))
		}
		if visual[0].Text != tok {
			t.Errorf("%s: Latin/digit characters must not be flipped; got %q", tok, visual[0].Text)
		}
	}

	// Mixed token: "שלום(RC):" pre-split by splitMixedBidiWord
	combined := Word{Text: "\u05E9\u05DC\u05D5\u05DD(RC):", Width: 50}
	subs := splitMixedBidiWord(combined)
	if len(subs) < 2 {
		t.Fatalf("expected splitMixedBidiWord to split %q, got %v", combined.Text, subs)
	}
	visual, _ := resolveLineBidi(subs, DirectionRTL)
	if len(visual) != len(subs) {
		t.Fatalf("expected %d visual words, got %d", len(subs), len(visual))
	}
	foundRC := false
	for _, w := range visual {
		if strings.Contains(w.Text, "RC") {
			foundRC = true
			if strings.Contains(w.Text, "CR") {
				t.Errorf("Latin characters 'RC' flipped to 'CR' in %q", w.Text)
			}
		}
	}
	if !foundRC {
		t.Errorf("RC token not found in visual output: %v", wordTexts(visual))
	}
}
