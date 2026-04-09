// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"

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
// Reordering operates at word granularity: each word is assigned the bidi
// level of its first character, and words are placed into the visual
// sequence according to the runs returned by bidi.Ordering. Within an
// RTL run, the words appear in reverse logical order. Words with mixed
// bidi levels (e.g. Hebrew+digits in a single token) are pre-split by
// splitMixedBidiWord before reaching this function, so each word here
// is directionally uniform.
//
// If the line contains only LTR text and the base direction is LTR, the
// words are returned unchanged (fast path).
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
		runePos += len([]rune(w.Text))
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

	// Resolve the base direction from the Ordering.
	resolved := DirectionLTR
	if ord.Direction() == bidi.RightToLeft {
		resolved = DirectionRTL
	}

	// Fast path: single LTR run covering the whole line — no reordering.
	if ord.NumRuns() == 1 {
		r := ord.Run(0)
		if r.Direction() == bidi.LeftToRight {
			return words, resolved
		}
	}

	// Map visual runs back to text words. Each run covers a rune range
	// in lineText; we find which text words overlap that range and
	// collect them in visual order. Within an RTL run the overlapping
	// words are appended in reverse logical order (last first).
	//
	// The bidi library's Order() returns runs in reading order: for an
	// LTR paragraph that is left-to-right (Run 0 = leftmost), but for
	// an RTL paragraph it is right-to-left (Run 0 = rightmost). Since
	// the layout engine always places words at increasing X from the
	// left, we traverse runs in reverse for RTL paragraphs so that the
	// first collected word lands at the page's left edge.
	numRuns := ord.NumRuns()
	visual := make([]Word, 0, len(words))
	placed := make(map[int]bool, len(words)) // tracks which word indices have been placed

	runStart, runEnd, runStep := 0, numRuns, 1
	if resolved == DirectionRTL {
		runStart, runEnd, runStep = numRuns-1, -1, -1
	}

	for ri := runStart; ri != runEnd; ri += runStep {
		run := ord.Run(ri)
		rStart, rEnd := run.Pos()
		runDir := run.Direction()

		// Collect indices of text words that overlap this run's rune range.
		var indices []int
		for wi, sp := range spans {
			if inlineAt[wi] {
				continue
			}
			if sp.end > rStart && sp.start < rEnd {
				indices = append(indices, wi)
			}
		}

		if runDir == bidi.RightToLeft {
			for j := len(indices) - 1; j >= 0; j-- {
				wi := indices[j]
				w := words[wi]
				w.Text = mirrorBrackets(w.Text)
				// Attach any inline-block words that immediately preceded
				// this text word in logical order (they travel with it).
				for ib := wi - 1; ib >= 0 && inlineAt[ib] && !placed[ib]; ib-- {
					visual = append(visual, words[ib])
					placed[ib] = true
				}
				visual = append(visual, w)
				placed[wi] = true
				// Attach trailing inline-blocks.
				for ib := wi + 1; ib < len(words) && inlineAt[ib] && !placed[ib]; ib++ {
					visual = append(visual, words[ib])
					placed[ib] = true
				}
			}
		} else {
			for _, wi := range indices {
				// Attach preceding inline-blocks.
				for ib := wi - 1; ib >= 0 && inlineAt[ib] && !placed[ib]; ib-- {
					visual = append(visual, words[ib])
					placed[ib] = true
				}
				visual = append(visual, words[wi])
				placed[wi] = true
				// Attach trailing inline-blocks.
				for ib := wi + 1; ib < len(words) && inlineAt[ib] && !placed[ib]; ib++ {
					visual = append(visual, words[ib])
					placed[ib] = true
				}
			}
		}
	}

	// Append any remaining inline-block words that weren't adjacent to
	// any text word (e.g., a line with only inline elements).
	for i, w := range words {
		if !placed[i] {
			visual = append(visual, w)
		}
	}

	return visual, resolved
}

// splitMixedBidiWord checks if a word contains characters at different
// bidi embedding levels (e.g. Hebrew letters mixed with digits or Latin
// characters in a single whitespace-delimited token). If level transitions
// are found, it splits the word into sub-words at the transition points.
// Each sub-word inherits all styling from the original word; only Text and
// Width differ. The caller must re-measure the sub-words' widths.
//
// Returns nil if the word has uniform bidi level (no split needed) or if
// it has no text content (inline block, empty).
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
	// If all runes have the same "directionality bucket", no split.
	type bucket int
	const (
		bucketLTR bucket = iota
		bucketRTL
		bucketNeutral
	)
	classify := func(r rune) bucket {
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

	// Walk runes, detect transitions between LTR and RTL (ignoring neutrals).
	prevStrong := bucketNeutral
	hasTransition := false
	for _, r := range runes {
		b := classify(r)
		if b == bucketNeutral {
			continue
		}
		if prevStrong != bucketNeutral && b != prevStrong {
			hasTransition = true
			break
		}
		prevStrong = b
	}
	if !hasTransition {
		return nil
	}

	// Split at strong-direction transitions. Neutral characters attach to
	// the preceding strong direction run (or the first strong run if they
	// lead). This produces the smallest number of sub-words while keeping
	// each sub-word directionally uniform.
	var parts []string
	start := 0
	currentStrong := bucketNeutral
	for i, r := range runes {
		b := classify(r)
		if b == bucketNeutral {
			continue
		}
		if currentStrong == bucketNeutral {
			currentStrong = b
			continue
		}
		if b != currentStrong {
			parts = append(parts, string(runes[start:i]))
			start = i
			currentStrong = b
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
