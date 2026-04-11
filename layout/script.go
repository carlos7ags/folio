// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"unicode"
	"unicode/utf8"
)

// Script identifies a Unicode script per UAX #24. The set of values is
// intentionally limited to the scripts the layout engine can currently
// distinguish for shaping decisions. All other codepoints (including the
// real UAX #24 Common and Inherited pseudo-scripts, plus any unrecognised
// script) map to ScriptCommon and are resolved against their neighbours.
type Script uint8

const (
	// ScriptCommon covers UAX #24 "Common" and "Inherited" codepoints, plus
	// anything outside the recognised set. Runs of Common characters are
	// promoted to a neighbouring real script during segmentation.
	ScriptCommon Script = iota
	ScriptLatin
	ScriptArabic
	ScriptHebrew
	ScriptDevanagari
	ScriptBengali
	ScriptTamil
	ScriptThai
	ScriptHan
	ScriptHiragana
	ScriptKatakana
	ScriptHangul
	ScriptCyrillic
	ScriptGreek
)

// ScriptOf returns the Script that r belongs to. ASCII letters take a fast
// path; everything else falls through an ordered list of unicode.RangeTable
// checks arranged roughly by real-world frequency so the common cases exit
// early. Characters outside the recognised scripts (digits, punctuation,
// symbols, combining marks, scripts we do not yet handle) return
// ScriptCommon; callers are expected to resolve these against neighbours.
func ScriptOf(r rune) Script {
	// ASCII fast path: letters are Latin, everything else is Common.
	if r < 0x80 {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			return ScriptLatin
		}
		return ScriptCommon
	}
	switch {
	case unicode.Is(unicode.Latin, r):
		return ScriptLatin
	case unicode.Is(unicode.Han, r):
		return ScriptHan
	case unicode.Is(unicode.Hiragana, r):
		return ScriptHiragana
	case unicode.Is(unicode.Katakana, r):
		return ScriptKatakana
	case unicode.Is(unicode.Hangul, r):
		return ScriptHangul
	case unicode.Is(unicode.Arabic, r):
		return ScriptArabic
	case unicode.Is(unicode.Hebrew, r):
		return ScriptHebrew
	case unicode.Is(unicode.Cyrillic, r):
		return ScriptCyrillic
	case unicode.Is(unicode.Greek, r):
		return ScriptGreek
	case unicode.Is(unicode.Devanagari, r):
		return ScriptDevanagari
	case unicode.Is(unicode.Bengali, r):
		return ScriptBengali
	case unicode.Is(unicode.Tamil, r):
		return ScriptTamil
	case unicode.Is(unicode.Thai, r):
		return ScriptThai
	}
	return ScriptCommon
}

// ScriptRun describes a contiguous byte range of a string whose characters
// all resolve to the same Script per UAX #24 segmentation. Start and End
// are byte offsets into the original string; End is exclusive.
type ScriptRun struct {
	Start  int
	End    int
	Script Script
}

// SegmentByScript partitions s into runs where each rune resolves to the
// same Script. Common and Inherited runes inherit from their left
// neighbour; leading Common runs fall back to the nearest real script on
// the right, and a whole-Common input emits a single ScriptCommon run.
//
// This is a linear pass: one walk to collect raw rune segments, a
// two-sweep resolution pass that promotes Commons against neighbours,
// then a coalescing emit. It implements the "inherit-from-neighbour"
// simplification of UAX #24; full Script_Extensions support can land in
// a later change.
func SegmentByScript(s string) []ScriptRun {
	if s == "" {
		return nil
	}

	// Phase 1: walk runes once, recording each rune's script and byte
	// span. We keep per-script granularity here (adjacent runes of the
	// same script are merged immediately) so the resolution pass below
	// can promote whole Common groups in one step.
	type rawRun struct {
		start, end int
		script     Script
	}
	raws := make([]rawRun, 0, 8)
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		sc := ScriptOf(r)
		if n := len(raws); n > 0 && raws[n-1].script == sc {
			raws[n-1].end = i + size
		} else {
			raws = append(raws, rawRun{start: i, end: i + size, script: sc})
		}
		i += size
	}

	// Phase 2: resolve Common runs.
	// Left-neighbour wins: a Common group takes the script of the
	// preceding real group. Leading Common groups have no left neighbour,
	// so a reverse sweep promotes them from the first real script to
	// their right. If no real script exists anywhere in s, every group
	// stays ScriptCommon and the whole string emits as one Common run.
	for idx := 0; idx < len(raws); idx++ {
		if raws[idx].script != ScriptCommon {
			continue
		}
		if idx > 0 && raws[idx-1].script != ScriptCommon {
			raws[idx].script = raws[idx-1].script
		}
	}
	for idx := len(raws) - 1; idx >= 0; idx-- {
		if raws[idx].script != ScriptCommon {
			continue
		}
		if idx+1 < len(raws) && raws[idx+1].script != ScriptCommon {
			raws[idx].script = raws[idx+1].script
		}
	}

	// Phase 3: coalesce adjacent groups that now share a script.
	out := make([]ScriptRun, 0, len(raws))
	for _, r := range raws {
		if n := len(out); n > 0 && out[n-1].Script == r.script {
			out[n-1].End = r.end
			continue
		}
		out = append(out, ScriptRun{Start: r.start, End: r.end, Script: r.script})
	}
	return out
}
