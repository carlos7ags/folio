// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"unicode/utf8"
)

// Kashida (tatweel, U+0640) is the Arabic baseline-extension character.
// In Arabic typography, justification is achieved by elongating the
// connectors between letters rather than by widening inter-word spaces.
// Tatweel is dual-joining: inserted between two glyphs that already
// connect on both sides, it stretches the connection without changing
// the visual identity of either neighbour.
//
// This file implements two operations:
//
//   - FindKashidaCandidates scans a string and returns every rune-boundary
//     where a tatweel could legally be inserted under the simplified rule
//     "the left neighbour can join on its left side AND the right neighbour
//     can join on its right side".
//
//   - InsertKashidas takes a target count and inserts that many tatweels
//     at the highest-priority sites returned by FindKashidaCandidates.
//
// The candidate finder works on both raw Arabic codepoints (U+0600..U+06FF)
// and on already-shaped Presentation Forms-B glyphs (U+FE70..U+FEFF), so it
// can be applied either before or after the Arabic shaper has run. This is
// important: the layout engine shapes a word once at measurement time, and
// kashida insertion happens during justification, after shaping.
//
// Reference: Unicode Standard, "Arabic" block; ArabicShaping.txt; the
// semantics of U+0640 ARABIC TATWEEL. The Unicode Standard does not
// prescribe a specific kashida placement algorithm — only the elongation
// character itself. The priority heuristic used here is intentionally
// simple; full typographic kashida placement is a follow-up topic.

// kashidaTatweel is the rune for ARABIC TATWEEL (U+0640).
const kashidaTatweel = '\u0640'

// KashidaCandidate marks one rune-boundary position in a string where
// inserting a tatweel (U+0640) is legal under the simplified Arabic
// joining rules. Position is the byte offset of the boundary in the
// original string — i.e. the byte index at which a U+0640 would be
// inserted, NOT the byte offset of either neighbour.
type KashidaCandidate struct {
	// Position is the byte offset where a tatweel would be inserted.
	Position int
	// Priority indicates how preferred this site is. Higher values are
	// preferred. The priority table is documented above this type.
	Priority uint8
}

// Priority constants for kashida candidate sites. Higher = more preferred.
//
// The priority order is intentionally simple for a first iteration:
//
//   - kashidaPriorityAfterSeen: after a seen-family letter (seen, sheen,
//     sad, dad). These letters carry a long horizontal baseline and accept
//     elongation gracefully.
//   - kashidaPriorityAfterFinal: after a letter that is in its final form
//     and itself follows a connection. Reserved for ligature-aware setups;
//     not currently distinguished from priority 1 in this implementation.
//   - kashidaPriorityMedialPair: between two dual-joining letters that are
//     both already medial (i.e. word-internal connections far from edges).
//   - kashidaPriorityBasic: any other legal insertion point.
const (
	kashidaPriorityBasic      uint8 = 1
	kashidaPriorityMedialPair uint8 = 2
	kashidaPriorityAfterFinal uint8 = 3
	kashidaPriorityAfterSeen  uint8 = 4
)

// joinSide describes which side(s) of a glyph can connect to a neighbour.
// A glyph that can join on its left side may have a tatweel inserted to
// the right of it (on its left edge). A glyph that can join on its right
// side may have a tatweel inserted to the left of it (on its right edge).
type joinSide uint8

const (
	joinNone  joinSide = 0
	joinLeft  joinSide = 1 << 0 // left side connects (dual or initial form)
	joinRight joinSide = 1 << 1 // right side connects (dual, final, medial form)
	joinBoth           = joinLeft | joinRight
)

// pfbJoinSide is a reverse-lookup table built from arabicFormsTable that
// maps each Presentation Forms-B codepoint to the side(s) it joins on.
// Initialised once in init() so subsequent calls are O(1) lookups.
var pfbJoinSide map[rune]joinSide

func init() {
	pfbJoinSide = make(map[rune]joinSide, len(arabicFormsTable)*4)
	for _, forms := range arabicFormsTable {
		// Isolated form: stands alone, no joining.
		if forms.isolated != 0 {
			pfbJoinSide[forms.isolated] = joinNone
		}
		// Final form: connects on its right side to a preceding glyph.
		if forms.final != 0 {
			pfbJoinSide[forms.final] = joinRight
		}
		// Initial form: connects on its left side to a following glyph.
		if forms.initial != 0 {
			pfbJoinSide[forms.initial] = joinLeft
		}
		// Medial form: connects on both sides.
		if forms.medial != 0 {
			pfbJoinSide[forms.medial] = joinBoth
		}
	}
	// Lam-alef ligatures behave like a final form for the leading lam:
	// they connect on the right side to a preceding glyph but never on
	// the left (alef breaks the connection). The "final" variants of
	// each lam-alef ligature carry that connection.
	for _, lig := range []rune{0xFEF6, 0xFEF8, 0xFEFA, 0xFEFC} {
		pfbJoinSide[lig] = joinRight
	}
	// Tatweel itself connects on both sides — the very property that
	// makes it usable as an elongation glyph.
	pfbJoinSide[kashidaTatweel] = joinBoth
}

// joinSideOf returns the join sides for a rune. It handles both base
// Arabic codepoints (via the existing joining-type classifier) and
// Presentation Forms-B glyphs (via pfbJoinSide). Non-Arabic runes return
// joinNone.
func joinSideOf(r rune) joinSide {
	if side, ok := pfbJoinSide[r]; ok {
		return side
	}
	switch getJoiningType(r) {
	case jtDual:
		return joinBoth
	case jtRight:
		return joinRight
	default:
		return joinNone
	}
}

// isSeenFamily reports whether r is one of seen/sheen/sad/dad in any of
// its forms (base codepoint or any of the four Presentation Forms-B
// positional variants). These letters carry a long horizontal baseline
// and are the canonical preferred sites for kashida insertion.
func isSeenFamily(r rune) bool {
	switch r {
	case 0x0633, 0x0634, 0x0635, 0x0636: // seen, sheen, sad, dad (base)
		return true
	// seen forms
	case 0xFEB1, 0xFEB2, 0xFEB3, 0xFEB4:
		return true
	// sheen forms
	case 0xFEB5, 0xFEB6, 0xFEB7, 0xFEB8:
		return true
	// sad forms
	case 0xFEB9, 0xFEBA, 0xFEBB, 0xFEBC:
		return true
	// dad forms
	case 0xFEBD, 0xFEBE, 0xFEBF, 0xFEC0:
		return true
	}
	return false
}

// FindKashidaCandidates returns every legal tatweel insertion point in s,
// in left-to-right (logical) byte order. A site is legal when the rune to
// the left of the boundary can join on its left side AND the rune to the
// right can join on its right side. Each candidate carries a priority
// derived from the simple heuristic described in the priority constants.
//
// The function operates on both raw Arabic codepoints and on already-
// shaped Presentation Forms-B glyphs, so callers can apply it either
// before or after Arabic shaping.
func FindKashidaCandidates(s string) []KashidaCandidate {
	if s == "" {
		return nil
	}
	var out []KashidaCandidate
	var prev rune
	hasPrev := false
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if hasPrev {
			// The boundary is between prev and r at byte offset i.
			leftSides := joinSideOf(prev)
			rightSides := joinSideOf(r)
			if leftSides&joinLeft != 0 && rightSides&joinRight != 0 {
				priority := kashidaPriorityBasic
				if isSeenFamily(prev) {
					priority = kashidaPriorityAfterSeen
				} else if leftSides == joinBoth && rightSides == joinBoth {
					priority = kashidaPriorityMedialPair
				}
				out = append(out, KashidaCandidate{
					Position: i,
					Priority: priority,
				})
			}
		}
		prev = r
		hasPrev = true
		i += size
	}
	return out
}

// InsertKashidas returns a copy of s with up to count tatweel characters
// (U+0640) inserted at the highest-priority candidate sites. If count is
// zero or there are no legal sites, s is returned unchanged. If count
// exceeds the number of legal sites, only the available sites are used
// and the remaining slack is left for the caller to consume by other
// means (whitespace stretching).
//
// Insertion strategy: candidates are ranked by priority (descending),
// with ties broken by position (ascending in logical order). At most one
// tatweel is inserted at each candidate site — duplicates would compound
// at a single connection and produce a visually wrong stair-stepped
// result. Callers wanting more elongation than there are sites should
// fall back to whitespace stretching.
//
// Precondition: count >= 0. Negative counts are treated as zero.
func InsertKashidas(s string, count int) string {
	if count <= 0 || s == "" {
		return s
	}
	candidates := FindKashidaCandidates(s)
	if len(candidates) == 0 {
		return s
	}
	if count > len(candidates) {
		count = len(candidates)
	}

	// Select the top-priority sites. We pick by priority descending,
	// breaking ties by spreading across the word: among sites of the same
	// priority we prefer the earliest, then the latest, then the next
	// earliest, and so on. This avoids piling all tatweels into one
	// adjacent cluster when the word has many equal-priority sites.
	picked := pickKashidaSites(candidates, count)
	if len(picked) == 0 {
		return s
	}

	// Sort picked offsets ascending and rebuild the string in one pass.
	insertSortAscending(picked)
	var sb strings.Builder
	sb.Grow(len(s) + count*len(string(kashidaTatweel)))
	cursor := 0
	for _, pos := range picked {
		sb.WriteString(s[cursor:pos])
		sb.WriteRune(kashidaTatweel)
		cursor = pos
	}
	sb.WriteString(s[cursor:])
	return sb.String()
}

// pickKashidaSites selects count positions from candidates, preferring
// higher priority and spreading equal-priority sites across the word.
func pickKashidaSites(candidates []KashidaCandidate, count int) []int {
	if count <= 0 || len(candidates) == 0 {
		return nil
	}
	// Group candidates by priority (highest first). We don't need a full
	// sort because the priority space is tiny (1..4): bucket once.
	const maxPriority = 4
	buckets := make([][]int, maxPriority+1)
	for _, c := range candidates {
		p := c.Priority
		if p > maxPriority {
			p = maxPriority
		}
		buckets[p] = append(buckets[p], c.Position)
	}
	picked := make([]int, 0, count)
	for p := maxPriority; p >= 1 && len(picked) < count; p-- {
		bucket := buckets[p]
		if len(bucket) == 0 {
			continue
		}
		need := count - len(picked)
		if need >= len(bucket) {
			picked = append(picked, bucket...)
			continue
		}
		// Spread selection across the bucket: alternate from the ends
		// inward so the chosen sites are not adjacent.
		picked = append(picked, spreadPick(bucket, need)...)
	}
	return picked
}

// spreadPick selects n positions from sorted-ascending bucket by
// alternating from the front and back of the bucket. With n=1 it returns
// the front. With n=2 it returns front and back. With n>=3 it interleaves
// front, back, front+1, back-1, ... until n positions are gathered.
func spreadPick(bucket []int, n int) []int {
	if n <= 0 || len(bucket) == 0 {
		return nil
	}
	if n >= len(bucket) {
		out := make([]int, len(bucket))
		copy(out, bucket)
		return out
	}
	out := make([]int, 0, n)
	lo, hi := 0, len(bucket)-1
	front := true
	for len(out) < n && lo <= hi {
		if front {
			out = append(out, bucket[lo])
			lo++
		} else {
			out = append(out, bucket[hi])
			hi--
		}
		front = !front
	}
	return out
}

// applyKashidaJustification mutates words in place to consume up to slack
// extra width by inserting tatweels into Arabic words. It returns the
// number of points actually consumed; the caller subtracts that from the
// remaining slack and distributes whatever is left via whitespace
// stretching.
//
// Eligibility rules:
//   - The word must use an embedded font that contains a real tatweel
//     glyph. Standard PDF fonts (Helvetica, Times, ...) cover only Latin
//     and have no Arabic glyphs, so they are skipped.
//   - The word's text must contain at least one legal kashida candidate
//     after Arabic shaping has produced its Presentation Forms-B output.
//
// Distribution strategy: total slack is divided evenly across the eligible
// Arabic words by kashida-glyph count. Each word gets roughly slack/N/adv
// tatweels where adv is the tatweel advance in the word's font. Words
// with too few legal sites take what they can; the residual rolls to the
// next word and ultimately back to the caller.
//
// After insertion, each modified word's Width and Text are updated. The
// re-measure path uses the same TextMeasurer the original measurement
// used, so kerning behaves identically.
func applyKashidaJustification(words []Word, slack float64) float64 {
	if slack <= 0 || len(words) == 0 {
		return 0
	}
	// Identify eligible word indices and their tatweel advances. We need
	// to know per-word advance because mixed-font lines may have different
	// tatweel glyph widths.
	type eligible struct {
		idx          int
		tatweelAdv   float64
		candidateCnt int
	}
	var elig []eligible
	for i := range words {
		w := words[i]
		if w.InlineBlock != nil || w.Embedded == nil {
			continue
		}
		if !wordIsArabic(w.Text) {
			continue
		}
		face := w.Embedded.Face()
		if face.GlyphIndex(kashidaTatweel) == 0 {
			continue
		}
		adv := w.Embedded.MeasureString(string(kashidaTatweel), w.FontSize)
		if adv <= 0 {
			continue
		}
		cnt := len(FindKashidaCandidates(w.Text))
		if cnt == 0 {
			continue
		}
		elig = append(elig, eligible{idx: i, tatweelAdv: adv, candidateCnt: cnt})
	}
	if len(elig) == 0 {
		return 0
	}

	consumed := 0.0
	remaining := slack
	// Pass over eligible words, allotting an even share of remaining slack
	// to each. We iterate left-to-right; words that can't absorb their
	// share roll the residual to the next word.
	for k, e := range elig {
		if remaining <= 0 {
			break
		}
		share := remaining / float64(len(elig)-k)
		count := int(share / e.tatweelAdv)
		if count <= 0 {
			continue
		}
		if count > e.candidateCnt {
			count = e.candidateCnt
		}
		newText := InsertKashidas(words[e.idx].Text, count)
		// Re-measure with the same TextMeasurer used at layout time.
		newWidth := words[e.idx].Embedded.MeasureString(newText, words[e.idx].FontSize)
		if words[e.idx].LetterSpacing != 0 {
			n := len([]rune(newText))
			if n > 1 {
				newWidth += words[e.idx].LetterSpacing * float64(n-1)
			}
		}
		delta := newWidth - words[e.idx].Width
		if delta <= 0 {
			continue
		}
		words[e.idx].Text = newText
		words[e.idx].Width = newWidth
		consumed += delta
		remaining -= delta
	}
	return consumed
}

// wordIsArabic reports whether s contains any Arabic-script codepoint
// (base block, supplement, Presentation Forms-A or Presentation Forms-B).
// The check is intentionally a single rune scan that exits at the first
// match — Arabic words usually have an Arabic letter very early.
func wordIsArabic(s string) bool {
	for _, r := range s {
		if r >= 0x0600 && r <= 0x06FF {
			return true
		}
		if r >= 0x0750 && r <= 0x077F {
			return true
		}
		if r >= 0xFB50 && r <= 0xFDFF {
			return true
		}
		if r >= 0xFE70 && r <= 0xFEFF {
			return true
		}
	}
	return false
}

// insertSortAscending sorts a small slice of ints in place. The slices
// here are tiny (one tatweel count per Arabic word, typically <16) so an
// insertion sort is fine and avoids pulling in sort for a hot path.
func insertSortAscending(a []int) {
	for i := 1; i < len(a); i++ {
		v := a[i]
		j := i - 1
		for j >= 0 && a[j] > v {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = v
	}
}
