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
// Reordering is done at word granularity: each word is assigned the bidi
// level of its first character, and words are placed into the visual
// sequence according to the runs returned by bidi.Ordering. Within an
// RTL run, the words appear in reverse logical order. Character-level
// reordering within a single word (e.g. digits inside a Hebrew word) is
// a known limitation of this approach — it requires splitting words at
// level transitions, which is deferred to a follow-up.
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
		return words, DirectionLTR
	}

	// Build the concatenated line text and record each word's rune range.
	// Run.Pos() returns rune offsets (not byte offsets), so the spans
	// must also be in rune units to make the overlap check correct.
	type span struct{ start, end int }
	spans := make([]span, len(words))
	var sb strings.Builder
	runePos := 0
	for i, w := range words {
		if i > 0 {
			sb.WriteByte(' ')
			runePos++
		}
		spans[i].start = runePos
		sb.WriteString(w.Text)
		runePos += len([]rune(w.Text))
		spans[i].end = runePos
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

	// Map visual runs back to words. Each run covers a rune range in
	// lineText; we find which words overlap that range and collect them
	// in visual order. Within an RTL run the overlapping words are
	// appended in reverse logical order (last overlapping word first).
	//
	// The bidi library's Order() returns runs in reading order: for an
	// LTR paragraph that is left-to-right (Run 0 = leftmost), but for
	// an RTL paragraph it is right-to-left (Run 0 = rightmost). Since
	// the layout engine always places words at increasing X from the
	// left, we traverse runs in reverse for RTL paragraphs so that the
	// first collected word lands at the page's left edge.
	numRuns := ord.NumRuns()
	visual := make([]Word, 0, len(words))

	runStart, runEnd, runStep := 0, numRuns, 1
	if resolved == DirectionRTL {
		runStart, runEnd, runStep = numRuns-1, -1, -1
	}

	for ri := runStart; ri != runEnd; ri += runStep {
		run := ord.Run(ri)
		rStart, rEnd := run.Pos()
		runDir := run.Direction()

		// Collect indices of words that overlap this run's byte range.
		var indices []int
		for wi, sp := range spans {
			// A word overlaps the run if its byte range intersects.
			if sp.end > rStart && sp.start < rEnd {
				indices = append(indices, wi)
			}
		}

		if runDir == bidi.RightToLeft {
			// Reverse: last overlapping word first in visual order.
			for j := len(indices) - 1; j >= 0; j-- {
				w := words[indices[j]]
				w.Text = mirrorBrackets(w.Text)
				visual = append(visual, w)
			}
		} else {
			for _, wi := range indices {
				visual = append(visual, words[wi])
			}
		}
	}

	return visual, resolved
}

// bidiMirrorMap maps opening brackets to closing and vice versa for
// UAX #9 rule L4 (mirrored characters). Only the commonly-used pairs
// are included; the full BidiMirroring.txt has ~550 entries but the
// vast majority are obscure mathematical symbols that rarely appear
// in production documents.
var bidiMirrorMap = map[rune]rune{
	'(':    ')',
	')':    '(',
	'[':    ']',
	']':    '[',
	'{':    '}',
	'}':    '{',
	'<':    '>',
	'>':    '<',
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
