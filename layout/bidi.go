// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/carlos7ags/folio/unicode/grapheme"
	"golang.org/x/text/unicode/bidi"
)

// resolveLineBidi takes a sequence of measured words belonging to a single
// line (in logical / reading order) and a base paragraph direction. It runs
// the Unicode Bidirectional Algorithm (UAX #9) on the concatenated line
// text, then reorders the words into visual order — the order they should
// be painted left-to-right on the page.
//
// The returned direction is the resolved base direction of the paragraph
// (LTR or RTL), which callers use for the default alignment decision.
//
// Reordering operates at word granularity: every rune's embedding level
// is recovered (runeBidiLevels), each word is given the level of its
// first strong character (pickWordLevel), and rule L2 reverses the word
// sequence level by level. A word whose characters sit at different
// levels (e.g. "ב-2026" when it was not pre-split) has its own glyphs
// reordered per segment by visualWordText, so only its RTL parts are
// reversed and mirrored. Reordered words keep their logical text in
// OriginalText (when not already set by a shaper) and logicalText, and
// inter-word gaps are re-attached to the logical boundary they belong to.
//
// The input slice is not modified. Callers that need the line in logical
// order afterwards (page-split overflow) must keep their own reference.
//
// If the line has no right-to-left level at all, the words are returned
// unchanged (fast path).
func resolveLineBidi(words []Word, base Direction) ([]Word, Direction) {
	if len(words) == 0 {
		return words, DirectionLTR
	}

	// Skip bidi processing if all words are empty or contain only
	// whitespace/control characters (e.g. lineBreakMarker). The bidi
	// library panics on Order().Direction() for content-free strings.
	// Respect the base direction parameter so that the caller gets
	// RTL alignment even for whitespace-only lines in an RTL paragraph.
	hasContent := false
	for _, w := range words {
		for _, r := range w.Text {
			if r > ' ' {
				hasContent = true
				break
			}
		}
		if hasContent {
			break
		}
	}
	if !hasContent {
		fallback := DirectionLTR
		if base == DirectionRTL {
			fallback = DirectionRTL
		}
		return words, fallback
	}

	// Separate inline-block words (Text=="") from text words. Inline
	// blocks don't participate in bidi — they have no directional text.
	// We record their original indices so we can splice them back into
	// the visual output at the correct positions after reordering.
	type span struct{ start, end int }
	spans := make([]span, len(words))
	inlineAt := make(map[int]bool) // word indices that are inline blocks
	var sb strings.Builder
	runePos := 0
	textWordCount := 0
	for i, w := range words {
		if w.InlineBlock != nil || w.Text == "" {
			// Inline-block or empty word: assign a zero-width span at the
			// current rune position. It won't overlap any bidi run but we
			// handle it in the splice pass below.
			inlineAt[i] = true
			spans[i] = span{runePos, runePos}
			continue
		}
		if textWordCount > 0 {
			sb.WriteByte(' ')
			runePos++
		}
		spans[i].start = runePos
		sb.WriteString(w.Text)
		runePos += utf8.RuneCountInString(w.Text)
		spans[i].end = runePos
		textWordCount++
	}
	lineText := sb.String()

	// Run the bidi algorithm.
	var p bidi.Paragraph
	var opts []bidi.Option
	switch base {
	case DirectionRTL:
		opts = append(opts, bidi.DefaultDirection(bidi.RightToLeft))
	case DirectionLTR:
		opts = append(opts, bidi.DefaultDirection(bidi.LeftToRight))
		// DirectionAuto: no option → auto-detect, LTR fallback.
	}
	if _, err := p.SetString(lineText, opts...); err != nil {
		return words, DirectionLTR
	}

	ord, err := p.Order()
	if err != nil {
		return words, DirectionLTR
	}

	// Resolve the paragraph embedding level. An explicit RTL base forces
	// level 1 (bidi.DefaultDirection(RightToLeft) makes the library use
	// paragraph level 1 rather than auto-detecting it), so the resolved
	// direction must follow the base too: Ordering.Direction() only
	// reports the first run's direction, which for "abc שלום def" under
	// dir=rtl is LTR even though the paragraph is RTL. Otherwise the
	// level comes from the first strong character (UAX #9 P2/P3), which
	// is exactly what the library computes when no level is forced.
	paraLevel := int8(0)
	if base == DirectionRTL || firstStrongIsRTL(lineText) {
		paraLevel = 1
	}
	resolved := DirectionLTR
	if paraLevel == 1 {
		resolved = DirectionRTL
	}

	levels := runeBidiLevels(&ord, lineText, paraLevel)

	// Fast path: no odd (RTL) level anywhere on the line. Levels 0 and 2
	// cancel out under rule L2, so logical order is visual order.
	hasOdd := false
	for _, lv := range levels {
		if lv%2 == 1 {
			hasOdd = true
			break
		}
	}
	if !hasOdd {
		return words, resolved
	}

	// Assign each word one embedding level that decides its position.
	// A text word takes the level of its first strong (L, R, AL)
	// character, so "ב-2026" travels with its Hebrew letter and "(RC):"
	// with its Latin letters; digit-only words take their digits' level;
	// purely neutral words take their lowest level. Inline blocks and
	// empty words are neutrals (UAX #9 N1/N2): when the resolved
	// directions on both sides agree they take that direction — where
	// numbers (EN/AN) count as R, except an EN that W7 turned into L —
	// else the paragraph's embedding direction.
	n := len(words)
	wordLevel := make([]int8, n)
	wordRTL := make([]bool, n) // resolved direction of each text word
	// lastStrongRTL tracks the previous strong type in logical order for
	// W7, starting from sos (the paragraph direction).
	lastStrongRTL := paraLevel%2 == 1
	for i := range words {
		if inlineAt[i] {
			continue
		}
		lv := levels[spans[i].start:spans[i].end]
		wordLevel[i] = pickWordLevel(words[i].Text, lv)
		switch cls, ok := firstBidiClass(words[i].Text); {
		case ok && cls == bidi.L:
			wordRTL[i] = false
			lastStrongRTL = false
		case ok && (cls == bidi.R || cls == bidi.AL):
			wordRTL[i] = true
			lastStrongRTL = true
		case ok && cls == bidi.AN:
			wordRTL[i] = true
		case ok && cls == bidi.EN:
			// W7: an EN preceded by strong L (or an L sos) is L;
			// otherwise it stays a number, which N1 treats as R.
			wordRTL[i] = lastStrongRTL
		default:
			// Purely neutral word: the direction its level resolved to.
			wordRTL[i] = wordLevel[i]%2 == 1
		}
	}
	for i := range words {
		if !inlineAt[i] {
			continue
		}
		paraRTL := paraLevel%2 == 1
		prev, next := paraRTL, paraRTL
		for j := i - 1; j >= 0; j-- {
			if !inlineAt[j] {
				prev = wordRTL[j]
				break
			}
		}
		for j := i + 1; j < n; j++ {
			if !inlineAt[j] {
				next = wordRTL[j]
				break
			}
		}
		dirRTL := paraRTL
		if prev == next {
			dirRTL = prev
		}
		// The lowest level >= the paragraph level with that direction.
		wordLevel[i] = paraLevel
		if (paraLevel%2 == 1) != dirRTL {
			wordLevel[i] = paraLevel + 1
		}
	}

	// UAX #9 rule L2 at word granularity: from the highest level down to
	// the lowest odd level (1), reverse every maximal sequence of words at
	// that level or higher. order[v] is the logical index of the word
	// painted at visual position v (left to right).
	order := make([]int, n)
	for i := range order {
		order[i] = i
	}
	reverseByLevel(order, func(i int) int8 { return wordLevel[i] })

	visual := make([]Word, n)
	for v, li := range order {
		w := words[li]
		if !inlineAt[li] {
			text, changed, wholeRTL := visualWordText(w.Text, levels[spans[li].start:spans[li].end], wordLevel[li])
			if changed {
				// Keep the logical text: the kashida pass needs it, and
				// /ActualText (and text extraction) should report the
				// reading-order string rather than the reversed glyphs.
				// A shaped word already carries its pre-shape text.
				if w.OriginalText == "" {
					w.OriginalText = w.Text
				}
				w.logicalText = w.Text
				w.visualRTL = wholeRTL
				w.Text = text
			}
		}
		visual[v] = w
	}

	// Inter-word gaps follow the logical boundary between two visually
	// adjacent words, not whichever word happens to land on the left:
	// for a split token like "ב-"+"2026" (no space between) the zero gap
	// must stay between those two pieces after reordering. Pairs that are
	// not logical neighbours keep the left word's own gap.
	for v := 0; v < n-1; v++ {
		a, b := order[v], order[v+1]
		switch {
		case b == a+1:
			visual[v].SpaceAfter = words[a].SpaceAfter
		case a == b+1:
			visual[v].SpaceAfter = words[b].SpaceAfter
		}
	}
	visual[n-1].SpaceAfter = words[n-1].SpaceAfter

	return visual, resolved
}

// splitMixedBidiWord checks if a word contains characters at different
// bidi embedding levels (e.g. Hebrew letters mixed with digits or Latin
// characters in a single whitespace-delimited token) OR characters from
// different Unicode scripts per UAX #24 (e.g. Arabic mixed with
// Devanagari). If either kind of transition is found, it splits the word
// into sub-words at the transition points so each emitted sub-word is
// uniform in both bidi bucket and script. Each sub-word inherits all
// styling from the original word; only Text and Width differ. The caller
// must re-measure the sub-words' widths.
//
// Script inheritance follows the left-neighbour rule: Common and
// Inherited runes (including digits, punctuation, combining marks) attach
// to the preceding real script, matching how bidi-neutral runes attach
// to the preceding strong bidi bucket. Leading Common runs attach to the
// first following real script.
//
// Returns nil if the word has uniform bidi level AND uniform script (no
// split needed) or if it has no text content (inline block, empty).
func splitMixedBidiWord(w Word) []Word {
	if w.Text == "" || w.InlineBlock != nil {
		return nil
	}
	runes := []rune(w.Text)
	if len(runes) <= 1 {
		return nil
	}

	// Classify each rune's bidi class quickly. We only care about the
	// distinction between strong-RTL (R, AL, AN) and everything else.
	// If all runes have the same "directionality bucket", no bidi split.
	type bucket int
	const (
		bucketLTR bucket = iota
		bucketRTL
		bucketNeutral
	)
	classifyBidi := func(r rune) bucket {
		props, _ := bidi.LookupRune(r)
		switch props.Class() {
		case bidi.R, bidi.AL, bidi.AN:
			return bucketRTL
		case bidi.L, bidi.EN:
			return bucketLTR
		default:
			return bucketNeutral
		}
	}

	// Walk runes once to detect any transition that would force a split:
	// a change in the strong bidi bucket, or a change in the resolved
	// script. Neutrals / Common runes never trigger a transition on
	// their own — they inherit from the left.
	prevStrong := bucketNeutral
	prevScript := ScriptCommon
	hasTransition := false
	for _, r := range runes {
		b := classifyBidi(r)
		if b != bucketNeutral {
			if prevStrong != bucketNeutral && b != prevStrong {
				hasTransition = true
				break
			}
			prevStrong = b
		}
		sc := ScriptOf(r)
		if sc != ScriptCommon {
			if prevScript != ScriptCommon && sc != prevScript {
				hasTransition = true
				break
			}
			prevScript = sc
		}
	}
	if !hasTransition {
		return nil
	}

	// Split at strong-direction or script transitions. Neutral / Common
	// characters attach to the preceding strong run (or the first strong
	// run if they lead). This produces the smallest number of sub-words
	// while keeping each sub-word uniform in both bidi and script.
	//
	// A split position must additionally land on a grapheme cluster
	// boundary (UAX #29 §3.1.1): a base + combining mark, or any other
	// extended-cluster sequence, must never be torn apart by this pass.
	// If a transition would land mid-cluster (e.g. a Devanagari vowel
	// sign at rune index i after a Latin base), the split is suppressed
	// for that rune and the offending codepoint is absorbed into the
	// preceding sub-word; the tracking state still advances so the next
	// real transition is detected correctly.
	var parts []string
	start := 0
	currentStrong := bucketNeutral
	currentScript := ScriptCommon
	// Map each rune index to its byte offset in w.Text so we can test
	// cluster-boundary alignment against GraphemeBreaks. Rune index N
	// is at byteOffsets[N]; len(runes) is appended so the final slice
	// math works out.
	byteOffsets := make([]int, len(runes)+1)
	{
		bi := 0
		for ri, r := range runes {
			byteOffsets[ri] = bi
			bi += utf8.RuneLen(r)
		}
		byteOffsets[len(runes)] = bi
	}
	for i, r := range runes {
		b := classifyBidi(r)
		sc := ScriptOf(r)
		bidiChange := false
		scriptChange := false
		if b != bucketNeutral {
			if currentStrong == bucketNeutral {
				currentStrong = b
			} else if b != currentStrong {
				bidiChange = true
			}
		}
		if sc != ScriptCommon {
			if currentScript == ScriptCommon {
				currentScript = sc
			} else if sc != currentScript {
				scriptChange = true
			}
		}
		if bidiChange || scriptChange {
			// Only emit the split if it lands on a grapheme cluster
			// boundary. A mid-cluster split would leave a mark or
			// other non-starter character at the head of the next
			// sub-word, which is incorrect per UAX #29. When the
			// split is suppressed the offending codepoint is folded
			// into the preceding sub-word, and the tracking state
			// stays on the preceding sub-word's strong bucket /
			// script so the next real cluster-aligned transition is
			// still detected.
			if isGraphemeBoundary(w.Text, byteOffsets[i]) {
				parts = append(parts, string(runes[start:i]))
				start = i
				if b != bucketNeutral {
					currentStrong = b
				} else {
					currentStrong = bucketNeutral
				}
				if sc != ScriptCommon {
					currentScript = sc
				} else {
					currentScript = ScriptCommon
				}
			}
		}
	}
	parts = append(parts, string(runes[start:]))
	if len(parts) <= 1 {
		return nil
	}

	// Build sub-words inheriting all styling from the original.
	subs := make([]Word, len(parts))
	for i, p := range parts {
		subs[i] = w               // copy all fields
		subs[i].Text = p          // override text
		subs[i].Width = 0         // caller must re-measure
		subs[i].SpaceAfter = 0    // only the last sub-word gets inter-word space
		subs[i].LineBreak = false // only the first inherits the original's LineBreak
	}
	subs[0].LineBreak = w.LineBreak
	subs[len(subs)-1].SpaceAfter = w.SpaceAfter
	return subs
}

// bidiMirrorMap maps opening brackets to closing and vice versa for
// UAX #9 rule L4 (mirrored characters). Only the commonly-used pairs
// are included; the full BidiMirroring.txt has ~550 entries but the
// vast majority are obscure mathematical symbols that rarely appear
// in production documents.
var bidiMirrorMap = map[rune]rune{
	'(':      ')',
	')':      '(',
	'[':      ']',
	']':      '[',
	'{':      '}',
	'}':      '{',
	'<':      '>',
	'>':      '<',
	'\u00AB': '\u00BB', // « → »
	'\u00BB': '\u00AB', // » → «
	'\u2018': '\u2019', // ' → '
	'\u2019': '\u2018', // ' → '
	'\u201C': '\u201D', // " → "
	'\u201D': '\u201C', // " → "
	'\u2039': '\u203A', // ‹ → ›
	'\u203A': '\u2039', // › → ‹
}

// mirrorBrackets substitutes mirrored bracket characters in s per
// UAX #9 rule L4. Called on words within RTL runs so that e.g. "("
// renders as ")" when the visual direction is right-to-left.
func mirrorBrackets(s string) string {
	// Fast path: check if any rune needs mirroring.
	needsMirror := false
	for _, r := range s {
		if _, ok := bidiMirrorMap[r]; ok {
			needsMirror = true
			break
		}
	}
	if !needsMirror {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		if m, ok := bidiMirrorMap[r]; ok {
			sb.WriteRune(m)
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// reverseVisualRTL reverses the visual characters of an RTL word so that
// the LTR drawing advance of PDF text operators (Tj, TJ, ShowTextHex) paints
// them in correct right-to-left visual reading order. Grapheme cluster
// boundaries are respected so that combining marks (Arabic harakat, Hebrew
// niqqud) stay attached to their base character.
func reverseVisualRTL(s string) string {
	breaks := grapheme.Breaks(s)
	numClusters := len(breaks) - 1
	if numClusters <= 1 {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	for i := numClusters - 1; i >= 0; i-- {
		sb.WriteString(s[breaks[i]:breaks[i+1]])
	}
	return sb.String()
}

// firstStrongIsRTL reports whether the first strong character of s
// (UAX #9 rule P2) is right-to-left (R or AL). Text with no strong
// character, or whose first strong character is L, reports false.
func firstStrongIsRTL(s string) bool {
	for _, r := range s {
		props, _ := bidi.LookupRune(r)
		switch props.Class() {
		case bidi.L:
			return false
		case bidi.R, bidi.AL:
			return true
		}
	}
	return false
}

// runeBidiLevels returns the resolved embedding level of every rune of
// lineText. The bidi library exposes only each run's direction (level
// parity), so the levels are rebuilt from it: in an RTL paragraph
// (level 1) every rune is either level 1 (RTL) or level 2 (LTR text and
// numbers, raised by rule I2). In an LTR paragraph RTL runes are level
// 1, and LTR-parity runes are level 0 except numbers that resolved to
// EN/AN — European digits after an R/AL letter (W7 leaves them EN) or
// Arabic-Indic digits — which rule I1 raises to level 2, together with
// the separators and terminators that rules W4/W5 bind to them. That
// distinction matters: "abc שלום 123 עולם" paints the Hebrew words and
// the number as one reversed group, not as two independent RTL runs.
func runeBidiLevels(ord *bidi.Ordering, lineText string, paraLevel int8) []int8 {
	runes := []rune(lineText)
	n := len(runes)
	levels := make([]int8, n)
	odd := make([]bool, n)
	for ri := range ord.NumRuns() {
		run := ord.Run(ri)
		start, end := run.Pos()
		isOdd := run.Direction() == bidi.RightToLeft
		for k := max(start, 0); k <= end && k < n; k++ {
			odd[k] = isOdd
		}
	}
	if paraLevel == 1 {
		for i := range levels {
			if odd[i] {
				levels[i] = 1
			} else {
				levels[i] = 2
			}
		}
		return levels
	}

	classes := make([]bidi.Class, n)
	lastStrongRTL := false
	for i, r := range runes {
		props, _ := bidi.LookupRune(r)
		c := props.Class()
		classes[i] = c
		switch c {
		case bidi.L:
			lastStrongRTL = false
		case bidi.R, bidi.AL:
			lastStrongRTL = true
		}
		switch {
		case odd[i]:
			levels[i] = 1
		case c == bidi.AN || (c == bidi.EN && lastStrongRTL):
			levels[i] = 2
		}
	}
	// W4/W5: a separator between two such numbers, or a terminator run
	// adjacent to one, belongs to the number.
	isJoiner := func(c bidi.Class) bool {
		switch c {
		case bidi.ES, bidi.CS, bidi.ET, bidi.NSM, bidi.BN:
			return true
		}
		return false
	}
	for i := 0; i < n; {
		if odd[i] || levels[i] != 0 || !isJoiner(classes[i]) {
			i++
			continue
		}
		j := i
		onlyET := true
		for j < n && !odd[j] && levels[j] == 0 && isJoiner(classes[j]) {
			if classes[j] == bidi.ES || classes[j] == bidi.CS {
				onlyET = false
			}
			j++
		}
		prev2 := i > 0 && levels[i-1] == 2
		next2 := j < n && levels[j] == 2
		if (prev2 && next2) || (onlyET && (prev2 || next2)) {
			for k := i; k < j; k++ {
				levels[k] = 2
			}
		}
		i = j
	}
	return levels
}

// pickWordLevel chooses the single embedding level that positions a
// word during word-granular reordering: the level of its first strong
// character (L, R or AL), else of its first number (EN/AN), else its
// lowest level. lv holds the word's per-rune levels.
func pickWordLevel(text string, lv []int8) int8 {
	if len(lv) == 0 {
		return 0
	}
	firstNum := -1
	ri := 0
	for _, r := range text {
		if ri >= len(lv) {
			break
		}
		props, _ := bidi.LookupRune(r)
		switch props.Class() {
		case bidi.L, bidi.R, bidi.AL:
			return lv[ri]
		case bidi.EN, bidi.AN:
			if firstNum < 0 {
				firstNum = ri
			}
		}
		ri++
	}
	if firstNum >= 0 {
		return lv[firstNum]
	}
	return slices.Min(lv)
}

// firstBidiClass returns the bidi class of the first strong (L, R, AL)
// character of text, or failing that of its first number (EN, AN). ok is
// false for a purely neutral word.
func firstBidiClass(text string) (cls bidi.Class, ok bool) {
	num, haveNum := bidi.Class(0), false
	for _, r := range text {
		props, _ := bidi.LookupRune(r)
		switch c := props.Class(); c {
		case bidi.L, bidi.R, bidi.AL:
			return c, true
		case bidi.EN, bidi.AN:
			if !haveNum {
				num, haveNum = c, true
			}
		}
	}
	return num, haveNum
}

// reverseByLevel applies UAX #9 rule L2 to order, a permutation of item
// indices: from the highest level down to level 1, every maximal
// contiguous sequence of items at that level or higher is reversed.
func reverseByLevel(order []int, level func(int) int8) {
	var maxL int8
	for _, i := range order {
		maxL = max(maxL, level(i))
	}
	for k := maxL; k >= 1; k-- {
		for i := 0; i < len(order); {
			if level(order[i]) < k {
				i++
				continue
			}
			j := i
			for j < len(order) && level(order[j]) >= k {
				j++
			}
			slices.Reverse(order[i:j])
			i = j
		}
	}
}

// visualWordText returns the left-to-right paint order of a word's
// characters given each rune's embedding level (lv) and the level that
// positions the word (wordLvl). changed reports whether the result
// differs from text; wholeRTL reports that the result is exactly the
// whole word reversed and mirrored (a uniformly right-to-left word),
// which the kashida pass can undo.
//
// Words without R/AL letters keep the long-standing rule: a word with
// LTR letters or digits is painted as-is (UAX #9 L2 keeps an LTR
// sequence's internal order), and a purely neutral word inside an RTL
// run is reversed and mirrored. A word with RTL letters is reordered per
// grapheme cluster by rule L2 over its own levels, reversing and
// mirroring only its RTL segments — so "ב-2026" paints as "2026-ב"
// rather than reversing the digits along with the letter.
func visualWordText(text string, lv []int8, wordLvl int8) (visual string, changed, wholeRTL bool) {
	hasRTL, hasLTR := false, false
	for _, r := range text {
		props, _ := bidi.LookupRune(r)
		switch props.Class() {
		case bidi.R, bidi.AL:
			hasRTL = true
		case bidi.L, bidi.EN, bidi.AN:
			hasLTR = true
		}
	}
	if !hasRTL {
		if hasLTR || wordLvl%2 == 0 {
			return text, false, false
		}
		v := reverseVisualRTL(mirrorBrackets(text))
		return v, v != text, true
	}

	breaks := grapheme.Breaks(text)
	nc := len(breaks) - 1
	if nc < 1 {
		return text, false, false
	}
	clusterLv := make([]int8, nc)
	ri := 0
	uniform := true
	for k := range nc {
		if ri < len(lv) {
			clusterLv[k] = lv[ri]
		} else if k > 0 {
			clusterLv[k] = clusterLv[k-1]
		} else {
			clusterLv[k] = wordLvl
		}
		if clusterLv[k] != clusterLv[0] {
			uniform = false
		}
		ri += utf8.RuneCountInString(text[breaks[k]:breaks[k+1]])
	}
	if uniform {
		if clusterLv[0]%2 == 0 {
			return text, false, false
		}
		v := reverseVisualRTL(mirrorBrackets(text))
		return v, v != text, true
	}
	order := make([]int, nc)
	for i := range order {
		order[i] = i
	}
	reverseByLevel(order, func(i int) int8 { return clusterLv[i] })
	var sb strings.Builder
	sb.Grow(len(text))
	for _, k := range order {
		c := text[breaks[k]:breaks[k+1]]
		if clusterLv[k]%2 == 1 {
			c = mirrorBrackets(c)
		}
		sb.WriteString(c)
	}
	v := sb.String()
	return v, v != text, false
}
