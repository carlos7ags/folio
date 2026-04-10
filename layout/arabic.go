// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "github.com/carlos7ags/folio/font"

// Arabic contextual shaping via Unicode Presentation Forms-B substitution.
//
// Each Arabic letter has up to 4 visual forms depending on its position
// in the connected script:
//   - Isolated: the letter stands alone (no joining on either side)
//   - Final:    the letter joins to its right neighbor
//   - Initial:  the letter joins to its left neighbor
//   - Medial:   the letter joins on both sides
//
// The algorithm walks a string, determines each letter's joining context
// from its neighbors' joining types, and substitutes the base codepoint
// with the appropriate Presentation Forms-B codepoint. This is NOT full
// OpenType GSUB shaping — it covers the standard Arabic and Farsi
// alphabets using the pre-composed forms that most Arabic fonts include.
//
// Joining type classification (from ArabicShaping.txt):
//   R = Right-joining: only joins to the right (final form exists, no initial/medial)
//   D = Dual-joining:  joins on both sides (all 4 forms exist)
//   U = Non-joining:   does not join (isolated only)
//   T = Transparent:   does not affect joining context (diacritics)
//
// Reference: Unicode Standard Annex #9 §3, ArabicShaping.txt,
//            Unicode Character Database (Presentation Forms-B block U+FE70–U+FEFF).

// joiningType classifies an Arabic codepoint's joining behavior.
type joiningType int

const (
	jtNone        joiningType = iota // non-Arabic or non-joining
	jtRight                          // R: joins to the right only (alef, dal, ra, waw, etc.)
	jtDual                           // D: joins on both sides
	jtTransparent                    // T: transparent (diacritics, does not break joining)
)

// arabicForms holds the Presentation Forms-B codepoints for the four
// positional variants of an Arabic letter. A zero value means the form
// does not exist (use the base codepoint as fallback).
type arabicForms struct {
	isolated rune
	final    rune
	initial  rune
	medial   rune
}

// getJoiningType returns the joining type for a rune. Only Arabic block
// characters (U+0600–U+06FF) and a few Farsi additions are classified;
// everything else returns jtNone.
func getJoiningType(r rune) joiningType {
	// Arabic diacritics (tashkeel) are transparent.
	if r >= 0x0610 && r <= 0x061A || r >= 0x064B && r <= 0x065F ||
		r == 0x0670 || r >= 0x06D6 && r <= 0x06ED {
		return jtTransparent
	}

	// Check the forms table: if a letter has forms, it's either D or R.
	if f, ok := arabicFormsTable[r]; ok {
		if f.initial != 0 {
			return jtDual // has initial form → dual-joining
		}
		return jtRight // only isolated+final → right-joining
	}

	// Known right-joining characters not in the forms table.
	switch r {
	case 0x0622, 0x0623, 0x0624, 0x0625, 0x0627, // alef variants + waw
		0x0629,                         // teh marbuta
		0x062F, 0x0630, 0x0631, 0x0632, // dal, dhal, ra, zain
		0x0648, // waw
		0x0698: // Farsi zhe
		return jtRight
	}

	// Lam-alef ligature codepoints (Presentation Forms-B) function as
	// right-joining so the preceding character gets its initial/medial form.
	switch r {
	case 0xFEF5, 0xFEF6, 0xFEF7, 0xFEF8, 0xFEF9, 0xFEFA, 0xFEFB, 0xFEFC:
		return jtRight
	}

	// Remaining Arabic block letters that are dual-joining.
	if r >= 0x0620 && r <= 0x064A {
		return jtDual
	}
	// Farsi additions.
	if r == 0x067E || r == 0x0686 || r == 0x06A9 || r == 0x06AF || r == 0x06CC {
		return jtDual
	}

	return jtNone
}

// arabicFormsTable maps base Arabic codepoints to their Presentation Forms-B
// positional variants. Built from the Unicode Presentation Forms-B block
// (U+FE70–U+FEFF). Only letters that actually have presentation forms are
// listed; letters not in this table but with a joining type of D/R will
// pass through unshaped (the font's default glyph is used).
//
// The table covers the 28 standard Arabic letters plus common Farsi/Urdu
// additions (peh, cheh, zhe, gaf, farsi yeh).
var arabicFormsTable = map[rune]arabicForms{
	// Hamza
	0x0621: {0xFE80, 0, 0, 0},
	// Alef with Madda Above
	0x0622: {0xFE81, 0xFE82, 0, 0},
	// Alef with Hamza Above
	0x0623: {0xFE83, 0xFE84, 0, 0},
	// Waw with Hamza Above
	0x0624: {0xFE85, 0xFE86, 0, 0},
	// Alef with Hamza Below
	0x0625: {0xFE87, 0xFE88, 0, 0},
	// Yeh with Hamza Above
	0x0626: {0xFE89, 0xFE8A, 0xFE8B, 0xFE8C},
	// Alef
	0x0627: {0xFE8D, 0xFE8E, 0, 0},
	// Beh
	0x0628: {0xFE8F, 0xFE90, 0xFE91, 0xFE92},
	// Teh Marbuta
	0x0629: {0xFE93, 0xFE94, 0, 0},
	// Teh
	0x062A: {0xFE95, 0xFE96, 0xFE97, 0xFE98},
	// Theh
	0x062B: {0xFE99, 0xFE9A, 0xFE9B, 0xFE9C},
	// Jeem
	0x062C: {0xFE9D, 0xFE9E, 0xFE9F, 0xFEA0},
	// Hah
	0x062D: {0xFEA1, 0xFEA2, 0xFEA3, 0xFEA4},
	// Khah
	0x062E: {0xFEA5, 0xFEA6, 0xFEA7, 0xFEA8},
	// Dal
	0x062F: {0xFEA9, 0xFEAA, 0, 0},
	// Dhal
	0x0630: {0xFEAB, 0xFEAC, 0, 0},
	// Ra
	0x0631: {0xFEAD, 0xFEAE, 0, 0},
	// Zain
	0x0632: {0xFEAF, 0xFEB0, 0, 0},
	// Seen
	0x0633: {0xFEB1, 0xFEB2, 0xFEB3, 0xFEB4},
	// Sheen
	0x0634: {0xFEB5, 0xFEB6, 0xFEB7, 0xFEB8},
	// Sad
	0x0635: {0xFEB9, 0xFEBA, 0xFEBB, 0xFEBC},
	// Dad
	0x0636: {0xFEBD, 0xFEBE, 0xFEBF, 0xFEC0},
	// Tah
	0x0637: {0xFEC1, 0xFEC2, 0xFEC3, 0xFEC4},
	// Zah
	0x0638: {0xFEC5, 0xFEC6, 0xFEC7, 0xFEC8},
	// Ain
	0x0639: {0xFEC9, 0xFECA, 0xFECB, 0xFECC},
	// Ghain
	0x063A: {0xFECD, 0xFECE, 0xFECF, 0xFED0},
	// Feh
	0x0641: {0xFED1, 0xFED2, 0xFED3, 0xFED4},
	// Qaf
	0x0642: {0xFED5, 0xFED6, 0xFED7, 0xFED8},
	// Kaf
	0x0643: {0xFED9, 0xFEDA, 0xFEDB, 0xFEDC},
	// Lam
	0x0644: {0xFEDD, 0xFEDE, 0xFEDF, 0xFEE0},
	// Meem
	0x0645: {0xFEE1, 0xFEE2, 0xFEE3, 0xFEE4},
	// Noon
	0x0646: {0xFEE5, 0xFEE6, 0xFEE7, 0xFEE8},
	// Heh
	0x0647: {0xFEE9, 0xFEEA, 0xFEEB, 0xFEEC},
	// Waw
	0x0648: {0xFEED, 0xFEEE, 0, 0},
	// Alef Maksura
	0x0649: {0xFEEF, 0xFEF0, 0, 0},
	// Yeh
	0x064A: {0xFEF1, 0xFEF2, 0xFEF3, 0xFEF4},

	// --- Farsi / Urdu additions ---

	// Peh (U+067E)
	0x067E: {0xFB56, 0xFB57, 0xFB58, 0xFB59},
	// Cheh (U+0686)
	0x0686: {0xFB7A, 0xFB7B, 0xFB7C, 0xFB7D},
	// Zhe (U+0698) — right-joining only
	0x0698: {0xFB8A, 0xFB8B, 0, 0},
	// Gaf (U+06AF)
	0x06AF: {0xFB92, 0xFB93, 0xFB94, 0xFB95},
	// Kaf (U+06A9) — Persian kaf
	0x06A9: {0xFB8E, 0xFB8F, 0xFB90, 0xFB91},
	// Farsi Yeh (U+06CC)
	0x06CC: {0xFBFC, 0xFBFD, 0xFBFE, 0xFBFF},
}

// Lam-Alef ligatures: when lam (U+0644) is followed by an alef variant,
// the pair is replaced by a single ligature codepoint. This is the most
// important Arabic ligature — text without it looks visibly wrong.
var lamAlefLigatures = map[rune]rune{
	0x0622: 0xFEF5, // lam + alef with madda above → isolated
	0x0623: 0xFEF7, // lam + alef with hamza above → isolated
	0x0625: 0xFEF9, // lam + alef with hamza below → isolated
	0x0627: 0xFEFB, // lam + alef → isolated
}

// lamAlefFinalLigatures are the final forms (when the lam joins to its right).
var lamAlefFinalLigatures = map[rune]rune{
	0x0622: 0xFEF6, // lam + alef with madda above → final
	0x0623: 0xFEF8, // lam + alef with hamza above → final
	0x0625: 0xFEFA, // lam + alef with hamza below → final
	0x0627: 0xFEFC, // lam + alef → final
}

// ShapeArabic applies Arabic contextual shaping to a string by substituting
// base Arabic codepoints with their Presentation Forms-B positional variants.
// Non-Arabic characters pass through unchanged. The function also handles
// lam-alef ligature formation.
//
// This function should be called on each word's text BEFORE width measurement,
// since the shaped codepoints may have different glyph widths in the font.
// Spaces break the joining context, so word-by-word shaping produces correct
// results for whitespace-delimited Arabic text.
func ShapeArabic(s string) string {
	return shapeArabicWithFont(s, nil, nil, nil)
}

// ShapeArabicWithFont applies Arabic contextual shaping using the font's
// OpenType GSUB tables when available, falling back to the Presentation
// Forms-B table when the font has no GSUB or for characters not covered
// by the font's substitutions.
//
// The face parameter is the EmbeddedFont's underlying Face; pass nil to
// use the Presentation Forms-B table unconditionally.
func ShapeArabicWithFont(s string, face font.Face) string {
	if face == nil {
		return shapeArabicWithFont(s, nil, nil, nil)
	}
	gp, ok := face.(font.GSUBProvider)
	if !ok {
		return shapeArabicWithFont(s, nil, nil, nil)
	}
	gsub := gp.GSUB()
	var gidReverse map[uint16]rune
	if gsub != nil {
		gidReverse = gp.GIDToUnicode()
	}
	return shapeArabicWithFont(s, gsub, face, gidReverse)
}

func shapeArabicWithFont(s string, gsub font.GSUBSubstitutions, face font.Face, gidReverse map[uint16]rune) string {
	runes := []rune(s)
	if len(runes) == 0 {
		return s
	}

	// Fast path: skip shaping if no Arabic characters are present.
	hasArabic := false
	for _, r := range runes {
		if r >= 0x0600 && r <= 0x06FF || r >= 0x0750 && r <= 0x077F ||
			r >= 0xFB50 && r <= 0xFDFF || r >= 0xFE70 && r <= 0xFEFF {
			hasArabic = true
			break
		}
	}
	if !hasArabic {
		return s
	}

	// Phase 1: apply lam-alef ligatures. This merges pairs of runes
	// into single ligature runes, reducing the slice length.
	runes = applyLamAlef(runes)

	// Phase 2: determine each character's positional form.
	result := make([]rune, 0, len(runes))
	for i, r := range runes {
		jt := getJoiningType(r)
		if jt == jtNone || jt == jtTransparent {
			result = append(result, r)
			continue
		}

		// Look at non-transparent neighbors to determine joining context.
		joinsRight := prevJoinsLeft(runes, i)
		joinsLeft := nextJoinsRight(runes, i)

		// Determine the target feature for this position, capped by
		// the character's joining type. Right-joining (R) characters
		// can only take isolated or final form, never initial/medial.
		var targetFeature font.GSUBFeature
		switch {
		case joinsRight && joinsLeft && jt == jtDual:
			targetFeature = font.GSUBMedi
		case joinsRight:
			targetFeature = font.GSUBFina
		case joinsLeft && jt == jtDual:
			targetFeature = font.GSUBInit
		default:
			targetFeature = font.GSUBIsol
		}

		// Try GSUB font-driven substitution first. This uses the font's
		// own glyph substitution tables instead of the hardcoded PFB map.
		// The pipeline: rune → GID (via cmap) → GSUB substitution →
		// replacement GID → rune (via reverse cmap). Falls through to
		// PFB if any step fails.
		if gsub != nil && face != nil && gidReverse != nil {
			if table, ok := gsub[targetFeature]; ok {
				gid := face.GlyphIndex(r)
				if gid != 0 {
					if subGID, found := table[gid]; found {
						if subRune, hasRune := gidReverse[subGID]; hasRune {
							result = append(result, subRune)
							continue
						}
					}
				}
			}
		}

		// Presentation Forms-B substitution (codepoint-based fallback).
		forms, hasForms := arabicFormsTable[r]
		if !hasForms {
			result = append(result, r)
			continue
		}

		var shaped rune
		switch targetFeature {
		case font.GSUBMedi:
			if forms.medial != 0 {
				shaped = forms.medial
			}
		case font.GSUBFina:
			if forms.final != 0 {
				shaped = forms.final
			}
		case font.GSUBInit:
			if forms.initial != 0 {
				shaped = forms.initial
			}
		default:
			if forms.isolated != 0 {
				shaped = forms.isolated
			}
		}
		if shaped == 0 {
			shaped = r
		}
		result = append(result, shaped)
	}

	return string(result)
}

// applyLamAlef scans for lam (U+0644) followed by an alef variant and
// replaces the pair with the appropriate ligature codepoint.
func applyLamAlef(runes []rune) []rune {
	out := make([]rune, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		if runes[i] == 0x0644 && i+1 < len(runes) {
			// Check if the next non-transparent rune is an alef variant.
			next := i + 1
			for next < len(runes) && getJoiningType(runes[next]) == jtTransparent {
				next++
			}
			if next < len(runes) {
				// Check if lam joins to its right (i.e., previous character
				// can join left → lam takes final form of the ligature).
				useFinal := prevJoinsLeft(runes, i)
				if useFinal {
					if lig, ok := lamAlefFinalLigatures[runes[next]]; ok {
						out = append(out, lig)
						// Skip transparent chars between lam and alef, then skip alef.
						i = next
						continue
					}
				} else {
					if lig, ok := lamAlefLigatures[runes[next]]; ok {
						out = append(out, lig)
						i = next
						continue
					}
				}
			}
		}
		out = append(out, runes[i])
	}
	return out
}

// prevJoinsLeft checks if the previous non-transparent character has a
// joining type that allows it to join to the left (i.e., it is D or L).
// "Joins left" means the character's left edge connects to the next
// character — for dual-joining (D) characters this is always true.
func prevJoinsLeft(runes []rune, i int) bool {
	for j := i - 1; j >= 0; j-- {
		jt := getJoiningType(runes[j])
		if jt == jtTransparent {
			continue
		}
		return jt == jtDual // only dual-joining characters connect on the left
	}
	return false
}

// nextJoinsRight checks if the next non-transparent character has a
// joining type that allows it to join to the right (i.e., it is D or R).
func nextJoinsRight(runes []rune, i int) bool {
	for j := i + 1; j < len(runes); j++ {
		jt := getJoiningType(runes[j])
		if jt == jtTransparent {
			continue
		}
		return jt == jtDual || jt == jtRight
	}
	return false
}
