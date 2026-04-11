// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"unicode"
	"unicode/utf8"
)

// Grapheme cluster boundary detection per Unicode UAX #29 §3.1.1,
// "Extended Grapheme Clusters" (the default in the standard). The
// algorithm assigns each codepoint a Grapheme_Cluster_Break property
// value and applies break rules GB1–GB13 pairwise. The public entry
// points (GraphemeBreaks, NextGraphemeBreak, GraphemeCount) produce
// byte offsets into a UTF-8 string so callers can walk clusters
// without materialising per-cluster substrings.
//
// Standard reference: Unicode UAX #29 §3.1.1 (rules GB1–GB13), using
// the Extended_Grapheme_Cluster variant. The rule numbers in the case
// branches below match that text exactly so a reviewer can audit each
// decision against the spec.
//
// Known limitation: the Extended_Pictographic table used for GB11
// (emoji ZWJ sequences) covers the main emoji ranges but is not the
// full Unicode Emoji Data list. Grapheme clustering of obscure emoji
// ZWJ sequences may therefore break between codepoints where a full
// table would join them. This is documented in the emoji range check
// below and is a follow-up item.

// gbProperty is the Grapheme_Cluster_Break property value assigned to
// each codepoint. Only the values that participate in the break rules
// are distinguished; every other codepoint is gbOther.
type gbProperty uint8

const (
	gbOther gbProperty = iota
	gbCR
	gbLF
	gbControl
	gbExtend
	gbZWJ
	gbRegionalIndicator
	gbPrepend
	gbSpacingMark
	gbL
	gbV
	gbT
	gbLV
	gbLVT
	gbExtendedPictographic
)

// gbPropertyOf returns the Grapheme_Cluster_Break property for r. For
// the buckets exposed by the stdlib unicode package (combining marks,
// control characters, format characters) we reuse the stdlib range
// tables; the remainder (Hangul jamo, Regional_Indicator,
// Extended_Pictographic, Prepend) are explicit codepoint ranges.
func gbPropertyOf(r rune) gbProperty {
	// GB3/GB4/GB5 prerequisites: CR, LF, and Control are the three
	// hard-break classes. CR and LF are distinguished because GB3
	// keeps them together as a single cluster.
	if r == '\r' {
		return gbCR
	}
	if r == '\n' {
		return gbLF
	}

	// ZWJ (U+200D) has its own property value because GB11 references
	// it directly. It must be checked before the Extend / Format paths
	// below, since ZWJ is also a format character.
	if r == 0x200D {
		return gbZWJ
	}

	// Regional_Indicator symbols U+1F1E6..U+1F1FF form flag emoji
	// pairs under GB12/GB13.
	if r >= 0x1F1E6 && r <= 0x1F1FF {
		return gbRegionalIndicator
	}

	// Hangul jamo blocks. The ranges below match the Hangul Jamo
	// block (U+1100..U+11FF) partitioned into L (leading consonants),
	// V (vowels), and T (trailing consonants), plus the precomposed
	// Hangul Syllables block (U+AC00..U+D7A3) where each syllable is
	// either LV or LVT depending on whether it has a trailing jamo.
	// GB6/GB7/GB8 keep jamo sequences in a single cluster.
	if r >= 0x1100 && r <= 0x11FF {
		switch {
		case r <= 0x115F:
			return gbL
		case r <= 0x11A7:
			return gbV
		default:
			return gbT
		}
	}
	// Hangul Jamo Extended-A (U+A960..U+A97F) is all L.
	if r >= 0xA960 && r <= 0xA97F {
		return gbL
	}
	// Hangul Jamo Extended-B (U+D7B0..U+D7FF) splits between V and T.
	if r >= 0xD7B0 && r <= 0xD7FF {
		if r <= 0xD7C6 {
			return gbV
		}
		return gbT
	}
	// Precomposed Hangul Syllables: LV if (syllable - base) % 28 == 0,
	// otherwise LVT. Each L has 21*28 = 588 syllables, and within each
	// L block every 28th syllable has no trailing consonant.
	if r >= 0xAC00 && r <= 0xD7A3 {
		if (r-0xAC00)%28 == 0 {
			return gbLV
		}
		return gbLVT
	}

	// Extended_Pictographic covers emoji that can participate in GB11
	// ZWJ sequences. The ranges below are the main emoji blocks; a
	// full Unicode Emoji Data integration is a follow-up (see the
	// package comment above). These ranges are sufficient for the
	// common cases (faces, people, symbols, flags's base characters,
	// miscellaneous pictographs).
	if isExtendedPictographic(r) {
		return gbExtendedPictographic
	}

	// Prepend: Indic prefixed marks. The canonical set is small and
	// lives in a few specific ranges. We include the ones that occur
	// in mainstream scripts so GB9b holds for typical text.
	if isPrepend(r) {
		return gbPrepend
	}

	// SpacingMark (GB9a): Indic vowel signs that take advance width
	// but combine with the preceding base. The stdlib unicode.Mc
	// (Mark_Spacing_Combining) covers this except for a handful of
	// codepoints UAX #29 explicitly excludes; for our purposes Mc is
	// a close enough match and captures the vast majority of cases.
	if unicode.Is(unicode.Mc, r) {
		return gbSpacingMark
	}

	// Extend (GB9): combining marks (Mn), enclosing marks (Me), and
	// a few format characters that behave as extenders. Mn and Me
	// together cover all non-spacing and enclosing combining marks.
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) {
		return gbExtend
	}

	// Control: the remaining Cc and Cf characters that are not CR,
	// LF, or ZWJ (ZWJ was caught above). GB4/GB5 break around these.
	// Line Separator (U+2028) and Paragraph Separator (U+2029) are
	// also Control per UAX #29.
	if r == 0x2028 || r == 0x2029 {
		return gbControl
	}
	if unicode.Is(unicode.Cc, r) || unicode.Is(unicode.Cf, r) {
		return gbControl
	}

	return gbOther
}

// isExtendedPictographic reports whether r is in the minimal
// Extended_Pictographic range used by GB11. This is a hand-coded
// approximation of the full Unicode Emoji Data table and covers the
// main emoji blocks. See the package-level comment for the scope of
// the limitation.
func isExtendedPictographic(r rune) bool {
	switch {
	case r == 0x00A9, r == 0x00AE: // copyright, registered
		return true
	case r == 0x203C, r == 0x2049:
		return true
	case r >= 0x2122 && r <= 0x2139:
		return true
	case r >= 0x2194 && r <= 0x2199:
		return true
	case r >= 0x21A9 && r <= 0x21AA:
		return true
	case r >= 0x231A && r <= 0x231B:
		return true
	case r == 0x2328:
		return true
	case r >= 0x23E9 && r <= 0x23F3:
		return true
	case r >= 0x23F8 && r <= 0x23FA:
		return true
	case r == 0x24C2:
		return true
	case r >= 0x25AA && r <= 0x25AB:
		return true
	case r == 0x25B6, r == 0x25C0:
		return true
	case r >= 0x25FB && r <= 0x25FE:
		return true
	case r >= 0x2600 && r <= 0x27BF:
		// Miscellaneous Symbols + Dingbats: broad emoji range.
		return true
	case r == 0x2B05, r == 0x2B06, r == 0x2B07, r == 0x2B1B, r == 0x2B1C, r == 0x2B50, r == 0x2B55:
		return true
	case r == 0x3030, r == 0x303D:
		return true
	case r == 0x3297, r == 0x3299:
		return true
	case r >= 0x1F000 && r <= 0x1F02F:
		return true
	case r >= 0x1F0A0 && r <= 0x1F0FF:
		return true
	case r >= 0x1F100 && r <= 0x1F64F:
		// Enclosed Alphanumeric Supplement through Emoticons.
		return true
	case r >= 0x1F680 && r <= 0x1F6FF:
		// Transport and Map Symbols.
		return true
	case r >= 0x1F700 && r <= 0x1F77F:
		return true
	case r >= 0x1F780 && r <= 0x1F7FF:
		return true
	case r >= 0x1F800 && r <= 0x1F8FF:
		return true
	case r >= 0x1F900 && r <= 0x1F9FF:
		// Supplemental Symbols and Pictographs.
		return true
	case r >= 0x1FA00 && r <= 0x1FAFF:
		return true
	}
	return false
}

// isPrepend reports whether r is a Prepend codepoint under GB9b. The
// canonical Prepend set is small (Arabic number signs, a handful of
// Indic prefixed marks); we enumerate the known ranges explicitly
// rather than pulling in a full property table.
func isPrepend(r rune) bool {
	switch {
	case r >= 0x0600 && r <= 0x0605: // Arabic number signs
		return true
	case r == 0x06DD: // Arabic end of ayah
		return true
	case r == 0x070F: // Syriac abbreviation mark
		return true
	case r == 0x0890, r == 0x0891: // Arabic pound / piastre signs
		return true
	case r == 0x08E2: // Arabic disputed end of ayah
		return true
	case r == 0x0D4E: // Malayalam letter dot reph
		return true
	case r == 0x110BD, r == 0x110CD: // Kaithi number sign / letter number sign
		return true
	case r >= 0x111C2 && r <= 0x111C3: // Sharada sign jihvamuliya / upadhmaniya
		return true
	case r == 0x1193F, r == 0x11941: // Dives Akuru prefixed nasal / initial ra
		return true
	case r == 0x11A3A: // Zanabazar Square cluster-initial letter ra
		return true
	case r >= 0x11A84 && r <= 0x11A89: // Soyombo cluster-initial letters
		return true
	case r == 0x11D46: // Masaram Gondi repha
		return true
	}
	return false
}

// shouldBreakBetween applies UAX #29 rules GB3–GB13 (and the implicit
// GB999 default) to decide whether a grapheme cluster boundary exists
// between two adjacent codepoints with the given properties. The
// caller also supplies the two pieces of state that pair-wise rules
// cannot capture on their own:
//
//   - oddRI: true if the count of contiguous Regional_Indicator
//     codepoints ending at prev is odd. Combined with GB12/GB13 this
//     is enough to pair flag emoji without over-clustering a run of
//     three or more RIs.
//   - zwjAfterPict: true if prev is ZWJ and the cluster before that
//     ZWJ ended in an Extended_Pictographic (possibly followed by
//     Extend characters). GB11 only joins a ZWJ to a following
//     Extended_Pictographic when this is true.
//
// The return value is true when a cluster break exists between prev
// and curr, false when they belong to the same cluster.
func shouldBreakBetween(prev, curr gbProperty, oddRI, zwjAfterPict bool) bool {
	// GB3: CR × LF — no break between CR and LF. Checked first so it
	// wins over GB4/GB5 which would otherwise break around CR and LF.
	if prev == gbCR && curr == gbLF {
		return false
	}
	// GB4: (Control | CR | LF) ÷ — always break after a controller.
	if prev == gbControl || prev == gbCR || prev == gbLF {
		return true
	}
	// GB5: ÷ (Control | CR | LF) — always break before a controller.
	if curr == gbControl || curr == gbCR || curr == gbLF {
		return true
	}
	// GB6: L × (L | V | LV | LVT).
	if prev == gbL && (curr == gbL || curr == gbV || curr == gbLV || curr == gbLVT) {
		return false
	}
	// GB7: (LV | V) × (V | T).
	if (prev == gbLV || prev == gbV) && (curr == gbV || curr == gbT) {
		return false
	}
	// GB8: (LVT | T) × T.
	if (prev == gbLVT || prev == gbT) && curr == gbT {
		return false
	}
	// GB9: × (Extend | ZWJ) — never break before an extender.
	if curr == gbExtend || curr == gbZWJ {
		return false
	}
	// GB9a: × SpacingMark — never break before a spacing mark.
	if curr == gbSpacingMark {
		return false
	}
	// GB9b: Prepend × — never break after a Prepend.
	if prev == gbPrepend {
		return false
	}
	// GB11: \p{Extended_Pictographic} Extend* ZWJ × \p{Extended_Pictographic}.
	// The caller has already tracked whether prev is a ZWJ that
	// extends an Extended_Pictographic; we just need curr to be an
	// Extended_Pictographic to complete the join.
	if zwjAfterPict && curr == gbExtendedPictographic {
		return false
	}
	// GB12: sot (RI RI)* RI × RI.
	// GB13: [^RI] (RI RI)* RI × RI.
	// Both rules collapse to: an RI pairs with a preceding RI only
	// when the count of consecutive RIs ending at prev is odd.
	if prev == gbRegionalIndicator && curr == gbRegionalIndicator && oddRI {
		return false
	}
	// GB999: Any ÷ Any — default is to break.
	return true
}

// GraphemeBreaks returns the byte offsets of grapheme cluster
// boundaries in s, including 0 (start of string) and len(s) (end of
// string). Boundaries follow the extended grapheme cluster rules from
// Unicode UAX #29 §3.1.1 (GB1–GB13). The returned slice always starts
// with 0 and ends with len(s), so len(result) - 1 equals the number of
// clusters in the string. For the empty string it returns [0].
//
// Example: GraphemeBreaks("e\u0301f") returns [0, 3, 4] — the
// combining acute (2 UTF-8 bytes) is part of the same cluster as 'e'.
func GraphemeBreaks(s string) []int {
	// GB1: sot ÷ Any — always break at the start. The empty string
	// still has the start boundary; GB2 (end boundary) then lands on
	// the same offset and the caller gets [0].
	out := make([]int, 0, len(s)/2+2)
	out = append(out, 0)
	if s == "" {
		return out
	}

	// ASCII fast path: runs of ASCII printables (0x20..0x7E except
	// 0x7F) all map to gbOther, and gbOther × gbOther always breaks
	// under GB999. The tricky bit is the final ASCII character in
	// the run: we cannot emit a boundary after it until we know the
	// next codepoint, because the next codepoint might be an Extend
	// or ZWJ (GB9) which would join back into the ASCII character's
	// cluster. So we emit boundaries only for completed pairs — i.e.
	// up through the second-to-last ASCII byte — and leave the final
	// ASCII byte for the main loop to handle.
	i := 0
	for i+1 < len(s) {
		c := s[i]
		next := s[i+1]
		if c < 0x20 || c == 0x7F || c >= 0x80 {
			break
		}
		if next < 0x20 || next == 0x7F || next >= 0x80 {
			break
		}
		// Both c and next are ASCII printables. gbOther × gbOther is
		// a break under GB999, so emit the boundary after c.
		i++
		out = append(out, i)
	}
	if i == len(s) {
		return out
	}

	// Full UAX #29 walk for the remainder. We re-decode the first
	// post-ASCII rune so it joins the state machine cleanly.
	prevProp := gbOther
	havePrev := false
	// Track the two pieces of cross-pair state required by GB11 and
	// GB12/GB13. riParity is true when the count of consecutive
	// Regional_Indicator codepoints ending at prev is odd; pictActive
	// is true when prev is ZWJ and the cluster before that ZWJ ended
	// in an Extended_Pictographic (with any number of Extend chars in
	// between, per GB11's Extend* clause).
	riParity := false
	pictActive := false
	// pictRun tracks whether the current cluster's last non-Extend
	// character was an Extended_Pictographic. GB11 uses this to
	// decide whether a following ZWJ counts as the "ZWJ after
	// pictographic" case.
	pictRun := false

	for i < len(s) {
		r, size := utf8.DecodeRuneInString(s[i:])
		curr := gbPropertyOf(r)

		if havePrev {
			if shouldBreakBetween(prevProp, curr, riParity, pictActive) {
				out = append(out, i)
				// A break resets the GB11 picto run: the new cluster
				// starts fresh, so the picto state is whatever the
				// current codepoint contributes.
				pictRun = curr == gbExtendedPictographic
			} else {
				// No break: the current codepoint joins the previous
				// cluster. Update the picto run: Extend and ZWJ keep
				// the existing picto state (GB11 Extend* clause); any
				// other non-breaking continuation resets it to whether
				// this codepoint itself is an Extended_Pictographic.
				if curr != gbExtend && curr != gbZWJ {
					pictRun = curr == gbExtendedPictographic
				}
			}
		} else {
			pictRun = curr == gbExtendedPictographic
		}

		// Update RI parity: consecutive RIs toggle, anything else
		// resets. This implements the "odd number of preceding RIs"
		// side of GB12/GB13 without tracking the full run length.
		if curr == gbRegionalIndicator {
			if prevProp == gbRegionalIndicator && havePrev {
				riParity = !riParity
			} else {
				riParity = true
			}
		} else {
			riParity = false
		}

		// pictActive is "prev is ZWJ and the cluster ending at that
		// ZWJ was picto-active". We compute it for the next iteration:
		// after processing curr, the ZWJ-after-pict state for the
		// pair (curr, next) is true iff curr is ZWJ and pictRun was
		// set before this codepoint's contribution erased it.
		if curr == gbZWJ && pictRun {
			pictActive = true
		} else {
			pictActive = false
		}

		prevProp = curr
		havePrev = true
		i += size
	}

	// GB2: Any ÷ eot — always break at the end.
	if out[len(out)-1] != len(s) {
		out = append(out, len(s))
	}
	return out
}

// NextGraphemeBreak returns the byte offset of the next cluster
// boundary strictly after start, or len(s) if start is already in the
// final cluster of s. It walks the rules incrementally without
// materialising the full break slice, so streaming consumers that
// only need one boundary at a time avoid the allocation.
//
// The caller is responsible for passing a start offset that lies on a
// valid UTF-8 rune boundary in s. Behaviour for offsets in the middle
// of a multi-byte rune is unspecified (and would indicate a bug in
// the caller since all folio APIs operate on rune boundaries).
func NextGraphemeBreak(s string, start int) int {
	if start >= len(s) {
		return len(s)
	}
	// Decode the starting rune so we have an initial property value
	// to compare against the next codepoint. If there is no next
	// codepoint, the only boundary after start is len(s).
	r, size := utf8.DecodeRuneInString(s[start:])
	prev := gbPropertyOf(r)
	pictRun := prev == gbExtendedPictographic
	riParity := prev == gbRegionalIndicator
	i := start + size

	for i < len(s) {
		r2, sz := utf8.DecodeRuneInString(s[i:])
		curr := gbPropertyOf(r2)
		pictActive := prev == gbZWJ && pictRun
		if shouldBreakBetween(prev, curr, riParity, pictActive) {
			return i
		}
		// No break: update the picto-run state per GB11's Extend*.
		if curr != gbExtend && curr != gbZWJ {
			pictRun = curr == gbExtendedPictographic
		}
		if curr == gbRegionalIndicator {
			if prev == gbRegionalIndicator {
				riParity = !riParity
			} else {
				riParity = true
			}
		} else {
			riParity = false
		}
		prev = curr
		i += sz
	}
	return len(s)
}

// GraphemeCount returns the number of extended grapheme clusters in
// s. The empty string has zero clusters; any non-empty string has at
// least one.
func GraphemeCount(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for i := 0; i < len(s); {
		i = NextGraphemeBreak(s, i)
		n++
	}
	return n
}

// isGraphemeBoundary reports whether byte offset pos in s is a
// grapheme cluster boundary. pos must be on a valid UTF-8 rune
// boundary; offsets of 0 and len(s) are always boundaries per
// GB1/GB2. This helper drives splitMixedBidiWord's "snap to cluster"
// adjustment.
func isGraphemeBoundary(s string, pos int) bool {
	if pos <= 0 || pos >= len(s) {
		return true
	}
	// Walk clusters from the start until we pass pos; if we land on
	// pos exactly, it is a boundary. A full scan is O(n) but the
	// caller only invokes this on words (short strings) so the cost
	// is bounded by word length.
	for i := 0; i < len(s); {
		if i == pos {
			return true
		}
		if i > pos {
			return false
		}
		i = NextGraphemeBreak(s, i)
	}
	return pos == len(s)
}
