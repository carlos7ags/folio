// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "github.com/carlos7ags/folio/font"

// Indic OpenType shaping engine.
//
// This file implements the Indic-branch shaper described in the
// Microsoft "Creating and supporting OpenType fonts for the Indic
// scripts" specification. References in the code use the short form
// "Indic spec §<section>" so reviewers can audit against the
// authoritative text. Only the spec text was consulted — no code
// from other implementations was read or copied.
//
// The pipeline itself is script-neutral: the five-phase structure
// (syllable scan, initial reorder, GSUB phase 3, final reorder,
// GSUB phase 5) comes from the spec and does not vary. What varies
// between Brahmic scripts is the per-codepoint category table and
// a handful of per-script policy flags (does this script form reph?
// does it have pre-base matras? does it have conjunct ligatures?).
// The shared code path reads those from an indicScriptConfig and
// dispatches based on the UAX #24 Unicode script of the input.
//
// Pipeline (shared across all supported scripts):
//
//	Phase 1: scanIndicSyllables identifies syllable cluster
//	         boundaries using the grammar from Indic spec §2
//	         (Syllables). Each syllable is typed as Consonant /
//	         Vowel / Standalone / Symbol / Number / Other / Broken.
//
//	Phase 2: assignIndicPositions walks a Consonant cluster and
//	         assigns positional categories (base, pre-base matra,
//	         reph, half forms, below-base, post-base). It does NOT
//	         move the pre-base matra yet — that happens in phase 4
//	         after the GSUB substitutions have run, matching Indic
//	         spec "Reordering" §4.
//
//	Phase 3: applyIndicPhase3 dispatches the phase-3 GSUB features
//	         in the spec's required order: nukt, akhn, rphf, rkrf,
//	         blwf, half, pstf, vatu, cjct. rphf is applied only to
//	         the reph span so it does not fire mid-syllable; every
//	         other feature runs over the whole cluster. Scripts
//	         with cfg.HasConjuncts == false (Tamil) skip phase-3
//	         entirely — the spec does not define halant-merge
//	         conjunct formation for Tamil.
//
//	Phase 4: reorderIndicVisual performs post-substitution
//	         reordering: the pre-base matra glyph moves to
//	         immediately before the base glyph, and the reph (a
//	         single glyph once rphf has fired) moves according to
//	         the per-script reph-position policy (after base for
//	         Devanagari / Bengali, end of syllable for Kannada /
//	         Telugu, etc.).
//
//	Phase 5: applyIndicPhase5 dispatches the phase-5 feature set:
//	         init, pres, abvs, blws, psts, haln, calt, clig, liga,
//	         rlig. This is the standard presentational pass.
//
// The script-specific entry points (ShapeDevanagari, ShapeBengali,
// etc.) are thin wrappers that select a config and delegate to
// ShapeIndic.

const (
	devaVirama    = 0x094D // halant
	devaNukta     = 0x093C
	devaRa        = 0x0930
	devaEyelashRa = 0x0931 // ra with middle diagonal stroke
	devaPreBaseMI = 0x093F // vowel sign I (pre-base)
	devaZWJ       = 0x200D
	devaZWNJ      = 0x200C
)

// devaCategory classifies a Brahmic codepoint by its OpenType
// shaping role. Categories derive from Indic spec §3 ("Character
// category" table) and from UAX #44 Indic_Syllabic_Category values
// where the spec refers back to them. The set is shared across all
// supported Brahmic scripts; the per-script config decides which
// category a given rune resolves to.
//
// The deva* prefix is retained on this type because the first
// generation of the shaper used it exclusively, and several unit
// tests in indic_test.go assert against it. The constants are
// reused unchanged for Bengali, Gujarati, Gurmukhi, Kannada,
// Malayalam, Oriya, Tamil, and Telugu.
type devaCategory uint8

const (
	devaCatOther devaCategory = iota
	devaCatConsonant
	devaCatConsonantRa   // the special Ra (or equivalent) that can form reph / rakar
	devaCatVowel         // independent vowel letter
	devaCatVowelSign     // dependent matra (non pre-base)
	devaCatPreBaseMatra  // pre-base vowel sign
	devaCatNukta         // combining nukta
	devaCatVirama        // halant / virama
	devaCatModifier      // anusvara, candrabindu, tippi, bindi
	devaCatVisarga       // visarga
	devaCatJoiner        // ZWJ
	devaCatNonJoiner     // ZWNJ
	devaCatNumber        // digit
	devaCatPunctuation   // danda, double danda, etc.
	devaCatIndependentVS // Vowel sign AA / AU / etc. (kept with vowel signs)
)

// rephPosition controls where the reph glyph moves during phase 4
// "Final reordering". Different Brahmic scripts place reph in
// different positions (Indic spec "Final reordering" §4.2).
type rephPosition uint8

const (
	// rephPosNone: the script does not form reph at all (Gurmukhi,
	// Tamil), so the reph detection rules do not fire.
	rephPosNone rephPosition = iota
	// rephPosAfterBase: reph moves to immediately after the base
	// consonant. Devanagari default.
	rephPosAfterBase
	// rephPosAfterPostBase: reph moves to immediately after any
	// post-base consonant or matra. Kannada / Telugu default.
	rephPosAfterPostBase
	// rephPosBeforePostBase: reph moves before the first post-base
	// matra (Malayalam).
	rephPosBeforePostBase
	// rephPosAfterMain: reph stays at the start and moves to the
	// end of the syllable after all other forms (Bengali variant).
	rephPosAfterMain
)

// indicScriptConfig bundles the per-script data the generic shaper
// needs. One config per supported script lives in the package-level
// map indicConfigs; the dispatcher picks the right one based on the
// UAX #24 script of the input text.
type indicScriptConfig struct {
	// BlockStart / BlockEnd delimit the main Unicode block for this
	// script. Runes outside the range are treated as devaCatOther.
	//
	// Several pieces of the categoriser (digits, consonants) treat
	// (BlockStart & 0xFF80) as the 128-rune block base and assume
	// the same intra-block layout that Devanagari uses: digits at
	// offset 0x66..0x6F, consonants at 0x15..0x39. Every supported
	// Brahmic block in Unicode follows this layout, but new scripts
	// added here MUST be checked against the relevant block chart
	// before relying on those default ranges. Scripts that deviate
	// must override the affected runes via CategoryOverrides or
	// extend ConsonantExtraRanges / IndependentVowelRanges.
	BlockStart rune
	BlockEnd   rune

	// Virama is the halant / virama codepoint. Used both for
	// category lookup and for the reph detection rule.
	Virama rune

	// Nukta is the combining nukta codepoint, or 0 if the script
	// does not use one.
	Nukta rune

	// RaLetter / RaAlternate are the codepoints whose (Ra + halant
	// + consonant) sequence triggers reph formation. RaAlternate
	// is 0 for scripts with a single Ra (most of them).
	RaLetter    rune
	RaAlternate rune

	// PreBaseMatras lists every codepoint that the script renders
	// visually before the base consonant (i.e. logically follows
	// the base in memory but visually precedes it). Most Brahmic
	// scripts have one such matra (Devanagari sign I U+093F); a
	// few have several (Malayalam U+0D46/U+0D47, Tamil
	// U+0BC6/U+0BC7/U+0BC8, Bengali U+09BF/U+09C7/U+09C8, Oriya
	// U+0B47/U+0B48). Empty / nil for scripts without pre-base
	// matras.
	//
	// TODO(#216): split matras (Bengali U+09CB/U+09CC, Malayalam
	// U+0D4A/U+0D4B/U+0D4C, Oriya U+0B4B/U+0B4C) contain a logical
	// pre-base part. They are deferred from this PR; current
	// behaviour treats them as composed inputs that pass through
	// without decomposition. See TestShape{Bengali,Malayalam}
	// SplitMatraNotDecomposed for the regression locks.
	PreBaseMatras []rune

	// RephPos is where a detected reph moves in phase 4.
	RephPos rephPosition

	// HasConjuncts reports whether the script forms conjunct
	// consonant clusters at all. Tamil is the notable exception:
	// it has no halant-merge conjuncts, so phase-3 features that
	// build conjunct shapes (vatu, akhn, half, blwf, pstf, cjct,
	// rphf, rkrf, pres) are skipped entirely. The phase-3 stack
	// short-circuits when this flag is false.
	HasConjuncts bool

	// CategoryOverrides maps specific codepoints to a category
	// that does not match the default block-position rule. The
	// default rules assume:
	//   - consonants occupy the first span of the block
	//   - dependent signs (matras) occupy a middle span
	//   - digits occupy U+*66..U+*6F
	//   - independent vowels occupy U+*04..U+*14 (+ vocalic L/LL)
	// Any script that deviates (Malayalam chillu runes, Oriya's
	// separate ra, Gurmukhi addak/tippi) lists exceptions here.
	CategoryOverrides map[rune]devaCategory

	// ConsonantExtraRanges is a list of [start, end] inclusive
	// codepoint ranges of "extra" consonants that sit outside the
	// main U+*15..U+*39 span (e.g. Devanagari's nukta-composed
	// consonants U+0958..U+095F).
	ConsonantExtraRanges [][2]rune

	// VowelSignRanges is a list of [start, end] inclusive ranges
	// of dependent vowel signs / matras. The generic category
	// lookup walks these ranges for anything that is not covered
	// by the explicit category map.
	VowelSignRanges [][2]rune

	// IndependentVowelRanges lists [start, end] inclusive ranges
	// of independent vowels.
	IndependentVowelRanges [][2]rune
}

// devanagariConfig is the reference configuration used by
// ShapeDevanagari (Indic spec Devanagari section).
var devanagariConfig = &indicScriptConfig{
	BlockStart:    0x0900,
	BlockEnd:      0x097F,
	Virama:        0x094D,
	Nukta:         0x093C,
	RaLetter:      0x0930,
	RaAlternate:   0x0931, // eyelash ra
	PreBaseMatras: []rune{0x093F},
	RephPos:       rephPosAfterBase,
	HasConjuncts:  true,
	CategoryOverrides: map[rune]devaCategory{
		0x0900: devaCatModifier, // inverted candrabindu
		0x0901: devaCatModifier, // candrabindu
		0x0902: devaCatModifier, // anusvara
		0x0903: devaCatVisarga,
		0x093A: devaCatVowelSign,
		0x093B: devaCatVowelSign,
		0x093C: devaCatNukta,
		0x093D: devaCatOther, // avagraha
		0x094D: devaCatVirama,
		0x0950: devaCatOther, // OM
		0x0951: devaCatOther,
		0x0952: devaCatOther,
		0x0953: devaCatOther,
		0x0954: devaCatOther,
		0x0962: devaCatVowelSign,
		0x0963: devaCatVowelSign,
		0x0964: devaCatPunctuation,
		0x0965: devaCatPunctuation,
		0x0970: devaCatPunctuation,
	},
	ConsonantExtraRanges: [][2]rune{
		{0x0958, 0x095F}, // nukta-composed consonants
	},
	VowelSignRanges: [][2]rune{
		{0x093E, 0x094C},
		{0x094E, 0x094F},
	},
	IndependentVowelRanges: [][2]rune{
		{0x0904, 0x0914},
		{0x0960, 0x0961},
	},
}

// bengaliConfig describes Bengali shaping. Bengali has its own Ra
// (U+09B0) and an alternate Ra with middle diagonal (U+09F0). It
// also uses ya-phalaa (handled by the font's GSUB; we do not
// synthesize it — see TestShapeBengaliYaPhalaa) and rakar through
// the standard rkrf feature.
//
// Bengali reph placement: the Microsoft Bengali spec places reph
// after the main consonant cluster and before any post-base
// matras / signs. We model that with rephPosAfterMain, which
// inserts the reph after the last non-SMVD slot in the syllable.
//
// TODO(#216): split matras U+09CB (O) and U+09CC (AU) decompose
// to a logical pre-base U+09C7 plus a post-base part. We currently
// pass them through unchanged; see TestShapeBengaliSplitMatraNotDecomposed.
var bengaliConfig = &indicScriptConfig{
	BlockStart:  0x0980,
	BlockEnd:    0x09FF,
	Virama:      0x09CD,
	Nukta:       0x09BC,
	RaLetter:    0x09B0,
	RaAlternate: 0x09F0,
	// Pre-base matras: sign I (U+09BF), sign E (U+09C7) and sign
	// AI (U+09C8) are all rendered visually before the base.
	PreBaseMatras: []rune{0x09BF, 0x09C7, 0x09C8},
	RephPos:       rephPosAfterMain,
	HasConjuncts:  true,
	CategoryOverrides: map[rune]devaCategory{
		0x0981: devaCatModifier, // candrabindu
		0x0982: devaCatModifier, // anusvara
		0x0983: devaCatVisarga,
		0x09BC: devaCatNukta,
		0x09BD: devaCatOther, // avagraha
		0x09CD: devaCatVirama,
		0x09D7: devaCatVowelSign, // au length mark
		0x0964: devaCatPunctuation,
		0x0965: devaCatPunctuation,
	},
	ConsonantExtraRanges: [][2]rune{
		{0x09DC, 0x09DD}, // nukta-composed ra / rha
		{0x09DF, 0x09DF}, // yya
	},
	VowelSignRanges: [][2]rune{
		{0x09BE, 0x09CC},
		{0x09E2, 0x09E3},
	},
	IndependentVowelRanges: [][2]rune{
		{0x0985, 0x0994},
		{0x09E0, 0x09E1},
	},
}

// gujaratiConfig describes Gujarati shaping. Gujarati is closely
// modelled on Devanagari: its block layout is the same offset
// structure and its matra positions match.
var gujaratiConfig = &indicScriptConfig{
	BlockStart:    0x0A80,
	BlockEnd:      0x0AFF,
	Virama:        0x0ACD,
	Nukta:         0x0ABC,
	RaLetter:      0x0AB0,
	PreBaseMatras: []rune{0x0ABF},
	RephPos:       rephPosAfterBase,
	HasConjuncts:  true,
	CategoryOverrides: map[rune]devaCategory{
		0x0A81: devaCatModifier,
		0x0A82: devaCatModifier,
		0x0A83: devaCatVisarga,
		0x0ABC: devaCatNukta,
		0x0ABD: devaCatOther,
		0x0ACD: devaCatVirama,
		0x0964: devaCatPunctuation,
		0x0965: devaCatPunctuation,
	},
	VowelSignRanges: [][2]rune{
		{0x0ABE, 0x0ACC},
		{0x0AE2, 0x0AE3},
	},
	IndependentVowelRanges: [][2]rune{
		{0x0A85, 0x0A94},
		{0x0AE0, 0x0AE1},
	},
}

// gurmukhiConfig describes Gurmukhi shaping. Gurmukhi has no reph
// formation (there is no equivalent "ra-halant-consonant" reph
// shape in Punjabi orthography); tippi (U+0A70) and bindi (U+0A02)
// are modifier marks and addak (U+0A71) is a gemination mark
// treated here as a modifier for category purposes.
var gurmukhiConfig = &indicScriptConfig{
	BlockStart:    0x0A00,
	BlockEnd:      0x0A7F,
	Virama:        0x0A4D,
	Nukta:         0x0A3C,
	RaLetter:      0x0A30,         // present but reph is disabled via RephPos
	PreBaseMatras: []rune{0x0A3F}, // sihari
	RephPos:       rephPosNone,
	HasConjuncts:  true,
	CategoryOverrides: map[rune]devaCategory{
		0x0A01: devaCatModifier, // adak bindi
		0x0A02: devaCatModifier, // bindi
		0x0A03: devaCatVisarga,
		0x0A3C: devaCatNukta,
		0x0A4D: devaCatVirama,
		0x0A70: devaCatModifier, // tippi
		0x0A71: devaCatModifier, // addak
		0x0A75: devaCatModifier, // yakash
		0x0964: devaCatPunctuation,
		0x0965: devaCatPunctuation,
	},
	ConsonantExtraRanges: [][2]rune{
		{0x0A59, 0x0A5E}, // nukta-composed consonants (gap at 0x0A5D)
	},
	VowelSignRanges: [][2]rune{
		{0x0A3E, 0x0A4C},
	},
	IndependentVowelRanges: [][2]rune{
		{0x0A05, 0x0A14},
	},
}

// kannadaConfig describes Kannada shaping. Kannada uses "vattu"
// post-base consonants and places reph after any post-base matra
// per Indic spec's Kannada section.
var kannadaConfig = &indicScriptConfig{
	BlockStart:    0x0C80,
	BlockEnd:      0x0CFF,
	Virama:        0x0CCD,
	RaLetter:      0x0CB0,
	PreBaseMatras: []rune{0x0CBF}, // sign I
	RephPos:       rephPosAfterPostBase,
	HasConjuncts:  true,
	CategoryOverrides: map[rune]devaCategory{
		0x0C80: devaCatOther,    // spacing candrabindu
		0x0C81: devaCatModifier, // candrabindu
		0x0C82: devaCatModifier, // anusvara
		0x0C83: devaCatVisarga,
		0x0CBC: devaCatNukta,
		0x0CBD: devaCatOther,
		0x0CCD: devaCatVirama,
		0x0CD5: devaCatVowelSign, // length mark
		0x0CD6: devaCatVowelSign, // ai length mark
	},
	VowelSignRanges: [][2]rune{
		{0x0CBE, 0x0CCC},
		{0x0CE2, 0x0CE3},
	},
	IndependentVowelRanges: [][2]rune{
		{0x0C85, 0x0C94},
		{0x0CE0, 0x0CE1},
	},
}

// malayalamConfig describes Malayalam shaping. Malayalam has chillu
// letters (atomic forms that are effectively "dead" consonants) and
// places the reph before any post-base matra. Chillus are precomposed
// in Unicode (U+0D7A..U+0D7F) and classified as consonants here so
// they can act as a syllable base.
//
// TODO(#216): split matras U+0D4A (O), U+0D4B (OO), U+0D4C (AU)
// decompose to a logical pre-base U+0D46/U+0D47 plus a post-base
// part. Currently passed through unchanged; see
// TestShapeMalayalamSplitMatraNotDecomposed.
//
// TODO(#216): chillu sequence formation (consonant + virama + ZWJ)
// is not synthesised in the shaper; we rely on the font's GSUB to
// substitute precomposed chillus. See
// TestShapeMalayalamChilluZWJSequence.
var malayalamConfig = &indicScriptConfig{
	BlockStart: 0x0D00,
	BlockEnd:   0x0D7F,
	Virama:     0x0D4D,
	RaLetter:   0x0D30,
	// Pre-base matras: sign E (U+0D46) and sign EE (U+0D47) are
	// rendered visually before the base. Sign AI (U+0D48) is also
	// pre-base in Malayalam orthography.
	PreBaseMatras: []rune{0x0D46, 0x0D47, 0x0D48},
	RephPos:       rephPosBeforePostBase,
	HasConjuncts:  true,
	CategoryOverrides: map[rune]devaCategory{
		0x0D01: devaCatModifier,
		0x0D02: devaCatModifier,
		0x0D03: devaCatVisarga,
		0x0D3D: devaCatOther, // avagraha
		0x0D4D: devaCatVirama,
		0x0D57: devaCatVowelSign, // au length mark
	},
	ConsonantExtraRanges: [][2]rune{
		{0x0D7A, 0x0D7F}, // chillu letters
	},
	VowelSignRanges: [][2]rune{
		{0x0D3E, 0x0D4C},
		{0x0D62, 0x0D63},
	},
	IndependentVowelRanges: [][2]rune{
		{0x0D05, 0x0D14},
		{0x0D60, 0x0D61},
	},
}

// oriyaConfig describes Oriya (Odia) shaping. Oriya has distinct
// pre-base matras and reph behaviour similar to Devanagari.
//
// TODO(#216): split matras U+0B4B (O) and U+0B4C (AU) decompose to
// a logical pre-base U+0B47 plus a post-base part. Currently passed
// through unchanged; same treatment as Bengali split matras.
var oriyaConfig = &indicScriptConfig{
	BlockStart: 0x0B00,
	BlockEnd:   0x0B7F,
	Virama:     0x0B4D,
	Nukta:      0x0B3C,
	RaLetter:   0x0B30,
	// Pre-base matras: sign E (U+0B47) and sign AI (U+0B48) both
	// render visually before the base.
	PreBaseMatras: []rune{0x0B47, 0x0B48},
	RephPos:       rephPosAfterBase,
	HasConjuncts:  true,
	CategoryOverrides: map[rune]devaCategory{
		0x0B01: devaCatModifier,
		0x0B02: devaCatModifier,
		0x0B03: devaCatVisarga,
		0x0B3C: devaCatNukta,
		0x0B3D: devaCatOther,
		0x0B4D: devaCatVirama,
		0x0B56: devaCatVowelSign, // ai length mark
		0x0B57: devaCatVowelSign, // au length mark
	},
	ConsonantExtraRanges: [][2]rune{
		{0x0B5C, 0x0B5D}, // nukta-composed rra / rha
		{0x0B5F, 0x0B5F}, // yya
		{0x0B71, 0x0B71}, // letter wa
	},
	VowelSignRanges: [][2]rune{
		{0x0B3E, 0x0B4C},
		{0x0B62, 0x0B63},
	},
	IndependentVowelRanges: [][2]rune{
		{0x0B05, 0x0B14},
		{0x0B60, 0x0B61},
	},
}

// tamilConfig describes Tamil shaping. Tamil does not form reph or
// halant-merge conjuncts: phase-3 features rphf / rkrf / blwf /
// half / pstf / vatu / cjct / pres do not apply. HasConjuncts is
// false and the phase-3 dispatcher short-circuits accordingly.
// Phase-5 presentational features still run because Tamil fonts
// rely on liga / clig / haln for cluster shaping.
//
// TODO(#216): even with HasConjuncts=false, the akhn feature could
// be allowed for SRI ligatures. Deferred until a Tamil font with
// that feature is exercised; see TestTamilPhase3FeaturesAreNoOps.
var tamilConfig = &indicScriptConfig{
	BlockStart: 0x0B80,
	BlockEnd:   0x0BFF,
	Virama:     0x0BCD,
	RaLetter:   0x0BB0,
	// Pre-base matras: sign E (U+0BC6), sign EE (U+0BC7) and sign
	// AI (U+0BC8) all render visually before the base in Tamil
	// orthography.
	PreBaseMatras: []rune{0x0BC6, 0x0BC7, 0x0BC8},
	RephPos:       rephPosNone,
	HasConjuncts:  false,
	CategoryOverrides: map[rune]devaCategory{
		0x0B82: devaCatModifier, // anusvara (Tamil uses "SIGN ANUSVARA" placement here)
		0x0B83: devaCatOther,    // visarga-like but treated as punctuation
		0x0BCD: devaCatVirama,
		0x0BD7: devaCatVowelSign, // au length mark
	},
	VowelSignRanges: [][2]rune{
		{0x0BBE, 0x0BCC},
	},
	IndependentVowelRanges: [][2]rune{
		{0x0B85, 0x0B94},
	},
}

// teluguConfig describes Telugu shaping. Telugu uses vattu post-base
// consonants and places reph after post-base matras.
var teluguConfig = &indicScriptConfig{
	BlockStart:    0x0C00,
	BlockEnd:      0x0C7F,
	Virama:        0x0C4D,
	RaLetter:      0x0C30,
	PreBaseMatras: []rune{0x0C3F}, // sign I
	RephPos:       rephPosAfterPostBase,
	HasConjuncts:  true,
	CategoryOverrides: map[rune]devaCategory{
		0x0C00: devaCatModifier, // combining candrabindu above
		0x0C01: devaCatModifier,
		0x0C02: devaCatModifier,
		0x0C03: devaCatVisarga,
		0x0C3D: devaCatOther,
		0x0C4D: devaCatVirama,
		0x0C55: devaCatVowelSign, // length mark
		0x0C56: devaCatVowelSign, // ai length mark
	},
	VowelSignRanges: [][2]rune{
		{0x0C3E, 0x0C4C},
		{0x0C62, 0x0C63},
	},
	IndependentVowelRanges: [][2]rune{
		{0x0C05, 0x0C14},
		{0x0C60, 0x0C61},
	},
}

// indicConfigFor returns the script config for the given UAX #24
// Script, or nil if no shaper is configured for it. Used by the
// public dispatch layer in paragraph.go.
func indicConfigFor(sc Script) *indicScriptConfig {
	switch sc {
	case ScriptDevanagari:
		return devanagariConfig
	case ScriptBengali:
		return bengaliConfig
	case ScriptGujarati:
		return gujaratiConfig
	case ScriptGurmukhi:
		return gurmukhiConfig
	case ScriptKannada:
		return kannadaConfig
	case ScriptMalayalam:
		return malayalamConfig
	case ScriptOriya:
		return oriyaConfig
	case ScriptTamil:
		return tamilConfig
	case ScriptTelugu:
		return teluguConfig
	}
	return nil
}

// categoryOf returns the devaCategory for r under the given script
// config. Runes outside the block return devaCatOther. ZWJ / ZWNJ
// are recognised globally.
func (cfg *indicScriptConfig) categoryOf(r rune) devaCategory {
	if r == devaZWJ {
		return devaCatJoiner
	}
	if r == devaZWNJ {
		return devaCatNonJoiner
	}
	if r < cfg.BlockStart || r > cfg.BlockEnd {
		// Accept Devanagari danda / double danda across scripts: the
		// punctuation signs U+0964 / U+0965 are shared.
		if r == 0x0964 || r == 0x0965 {
			return devaCatPunctuation
		}
		return devaCatOther
	}
	// Explicit per-rune overrides take precedence.
	if cat, ok := cfg.CategoryOverrides[r]; ok {
		return cat
	}
	// Digits: the Unicode block layout for Brahmic scripts reserves
	// U+*66..U+*6F for digits (e.g. U+0966..U+096F for Devanagari,
	// U+09E6..U+09EF for Bengali). Compute the block digit range
	// relative to BlockStart.
	blockBase := cfg.BlockStart & 0xFF80
	digitStart := blockBase | 0x66
	digitEnd := blockBase | 0x6F
	if r >= digitStart && r <= digitEnd {
		return devaCatNumber
	}
	// Independent vowels.
	for _, rng := range cfg.IndependentVowelRanges {
		if r >= rng[0] && r <= rng[1] {
			return devaCatVowel
		}
	}
	// Dependent vowel signs / matras.
	for _, rng := range cfg.VowelSignRanges {
		if r >= rng[0] && r <= rng[1] {
			// A pre-base matra is a dependent vowel sign with special
			// reordering semantics; flag it as such here so phase-2
			// can pick it up.
			for _, pre := range cfg.PreBaseMatras {
				if r == pre {
					return devaCatPreBaseMatra
				}
			}
			return devaCatVowelSign
		}
	}
	// Nukta and virama flagged via single codepoints.
	if cfg.Nukta != 0 && r == cfg.Nukta {
		return devaCatNukta
	}
	if r == cfg.Virama {
		return devaCatVirama
	}
	// Ra (and eyelash / alternate Ra) gets its own category so
	// reph / rakar logic can find it without a second table lookup.
	if r == cfg.RaLetter || (cfg.RaAlternate != 0 && r == cfg.RaAlternate) {
		return devaCatConsonantRa
	}
	// Consonants U+*15..U+*39 plus any extra ranges (e.g. nukta-
	// composed precomposed consonants).
	consStart := blockBase | 0x15
	consEnd := blockBase | 0x39
	if r >= consStart && r <= consEnd {
		return devaCatConsonant
	}
	for _, rng := range cfg.ConsonantExtraRanges {
		if r >= rng[0] && r <= rng[1] {
			return devaCatConsonant
		}
	}
	// Anything else in the block is rare and treated as neutral.
	return devaCatOther
}

// devaSyllableType tags the grammar type of a syllable cluster per
// Indic spec §2 "Syllables".
type devaSyllableType uint8

const (
	devaSylConsonant   devaSyllableType = iota // consonant-driven cluster
	devaSylVowel                               // independent-vowel cluster
	devaSylStandalone                          // nukta or halant at start (dotted circle in some fonts)
	devaSylSymbol                              // symbol characters
	devaSylNumber                              // digit run
	devaSylPunctuation                         // danda / double danda run
	devaSylOther                               // everything else
	devaSylBroken                              // grammar failure (spec's "Broken" type)
)

// devaSyllable is a contiguous rune range assigned to one syllable
// cluster by scanIndicSyllables.
type devaSyllable struct {
	StartRune int              // inclusive rune index
	EndRune   int              // exclusive rune index
	Type      devaSyllableType // Consonant / Vowel / ...
}

// scanIndicSyllables walks runes once and yields the syllable
// cluster boundaries under cfg. The grammar implemented here is a
// simplified form of Indic spec §2:
//
//	Consonant  := (C H)* C M? MOD? VIS?   // C = consonant, H = halant,
//	                                     // M = vowel sign, MOD = modifier,
//	                                     // VIS = visarga
//	Vowel      := V M? MOD? VIS?          // V = independent vowel
//	Standalone := (N | H) + stray marks   // leading nukta or halant
//	Number     := D+                      // digit run
//	Punct      := P                       // danda / double danda
//	Other      := any other rune
//
// The scanner is greedy and never looks back: once a cluster
// boundary is decided it is not revisited.
func scanIndicSyllables(runes []rune, cfg *indicScriptConfig) []devaSyllable {
	if len(runes) == 0 {
		return nil
	}
	var out []devaSyllable
	i := 0
	for i < len(runes) {
		start := i
		cat := cfg.categoryOf(runes[i])
		switch cat {
		case devaCatNumber:
			for i < len(runes) && cfg.categoryOf(runes[i]) == devaCatNumber {
				i++
			}
			out = append(out, devaSyllable{start, i, devaSylNumber})
		case devaCatPunctuation:
			i++
			out = append(out, devaSyllable{start, i, devaSylPunctuation})
		case devaCatVowel:
			i++
			i = consumeIndicTail(runes, i, cfg)
			out = append(out, devaSyllable{start, i, devaSylVowel})
		case devaCatConsonant, devaCatConsonantRa:
			i = consumeIndicConsonantCluster(runes, i, cfg)
			i = consumeIndicTail(runes, i, cfg)
			out = append(out, devaSyllable{start, i, devaSylConsonant})
		case devaCatVirama, devaCatNukta:
			i++
			for i < len(runes) {
				c := cfg.categoryOf(runes[i])
				if c == devaCatVirama || c == devaCatNukta || c == devaCatVowelSign ||
					c == devaCatPreBaseMatra || c == devaCatModifier || c == devaCatVisarga {
					i++
					continue
				}
				break
			}
			out = append(out, devaSyllable{start, i, devaSylStandalone})
		case devaCatJoiner, devaCatNonJoiner:
			i++
			out = append(out, devaSyllable{start, i, devaSylOther})
		default:
			i++
			for i < len(runes) {
				c := cfg.categoryOf(runes[i])
				if c == devaCatConsonant || c == devaCatConsonantRa ||
					c == devaCatVowel || c == devaCatNumber || c == devaCatPunctuation {
					break
				}
				i++
			}
			out = append(out, devaSyllable{start, i, devaSylOther})
		}
	}
	return out
}

// consumeIndicConsonantCluster walks the (C N? H)* C N? prefix of
// a Consonant syllable starting at runes[i] and returns the new
// index positioned just past the base consonant (optionally
// followed by its nukta).
func consumeIndicConsonantCluster(runes []rune, i int, cfg *indicScriptConfig) int {
	for i < len(runes) {
		c := cfg.categoryOf(runes[i])
		if c != devaCatConsonant && c != devaCatConsonantRa {
			break
		}
		i++
		// Optional nukta immediately after the consonant.
		if i < len(runes) && cfg.categoryOf(runes[i]) == devaCatNukta {
			i++
		}
		// If the next char is a halant, continue the loop; otherwise
		// the current consonant is the base and we are done.
		if i < len(runes) && cfg.categoryOf(runes[i]) == devaCatVirama {
			i++
			if i < len(runes) {
				c2 := cfg.categoryOf(runes[i])
				if c2 == devaCatJoiner || c2 == devaCatNonJoiner {
					i++
				}
			}
			continue
		}
		break
	}
	return i
}

// consumeIndicTail walks the trailing matra / modifier / visarga /
// halant run after a base consonant or independent vowel.
func consumeIndicTail(runes []rune, i int, cfg *indicScriptConfig) int {
	for i < len(runes) {
		c := cfg.categoryOf(runes[i])
		if c == devaCatVowelSign || c == devaCatPreBaseMatra ||
			c == devaCatNukta || c == devaCatModifier ||
			c == devaCatVisarga || c == devaCatVirama {
			i++
			continue
		}
		break
	}
	return i
}

// devaGlyph is the per-position payload used during cluster
// shaping: the current GID plus the phase-2 positional metadata
// needed by the phase-4 visual reordering pass. Glyphs produced by
// GSUB that collapse several inputs (ligatures, rphf) preserve the
// metadata of the input slot that survived.
type devaGlyph struct {
	GID uint16

	// Positional category assigned during phase 2. Once set it
	// carries through GSUB so reorder can find the base and the
	// pre-base matra even after Single-substitution renaming.
	Pos devaGlyphPos
}

// devaGlyphPos is the positional role assigned to a glyph during
// phase 2 ("Initial reordering"). See Indic spec §4.
type devaGlyphPos uint8

const (
	devaPosNone       devaGlyphPos = iota
	devaPosBase                    // base consonant (or rphf-substituted reph that will move)
	devaPosPreBase                 // half form or other pre-base consonant
	devaPosPreBaseM                // pre-base matra before substitution
	devaPosAboveBase               // above-base sign (matras, modifiers)
	devaPosBelowBase               // below-base form consonant
	devaPosPostBase                // post-base form
	devaPosRephBase                // the leading Ra that becomes reph
	devaPosRephHalant              // the halant after the leading Ra
	devaPosSMVD                    // modifier / visarga (stay after base)
)

// ShapeIndic runs the full five-phase Indic shaping pipeline on s
// under the given script config. It is the script-neutral entry
// point; the script-specific wrappers (ShapeDevanagari, ShapeBengali,
// etc.) simply select a config and delegate here.
//
// When gsub is nil or the face has no features for this script, the
// function still runs the scanner and phase-2/phase-4 reordering so
// that the returned GID stream is in visual order using the base
// codepoint GIDs. This is the "no-GSUB fallback" and it renders
// passably for fonts that have glyphs but no shaping tables.
func ShapeIndic(s string, face font.Face, gsub *font.GSUBSubstitutions, cfg *indicScriptConfig) []uint16 {
	runes := []rune(s)
	if len(runes) == 0 || face == nil || cfg == nil {
		return nil
	}
	syllables := scanIndicSyllables(runes, cfg)
	if len(syllables) == 0 {
		return nil
	}
	var out []uint16
	for _, syl := range syllables {
		shaped := shapeIndicSyllable(runes[syl.StartRune:syl.EndRune], syl.Type, face, gsub, cfg)
		out = append(out, shaped...)
	}
	return out
}

// ShapeDevanagari is the Devanagari-specific entry point, kept for
// backwards compatibility with the original API. It forwards to
// ShapeIndic with devanagariConfig.
func ShapeDevanagari(s string, face font.Face, gsub *font.GSUBSubstitutions) []uint16 {
	return ShapeIndic(s, face, gsub, devanagariConfig)
}

// shapeIndicSyllable runs the per-syllable pipeline: build the
// initial glyph stream, assign phase-2 categories, apply phase-3
// GSUB, do phase-4 visual reordering, apply phase-5 GSUB, and
// return the final GIDs.
func shapeIndicSyllable(runes []rune, typ devaSyllableType, face font.Face, gsub *font.GSUBSubstitutions, cfg *indicScriptConfig) []uint16 {
	// Fast path for non-cluster syllables: map rune-to-GID and
	// return. Phase 3/4/5 features don't apply to these types.
	if typ != devaSylConsonant && typ != devaSylVowel {
		out := make([]uint16, 0, len(runes))
		for _, r := range runes {
			out = append(out, face.GlyphIndex(r))
		}
		return out
	}
	glyphs := make([]devaGlyph, len(runes))
	for i, r := range runes {
		glyphs[i] = devaGlyph{GID: face.GlyphIndex(r)}
	}
	// Phase 2: assign positional categories so later phases can
	// reorder and dispatch features correctly.
	assignIndicPositions(runes, glyphs, typ, cfg)

	// Phase 3 GSUB features (Indic spec §5 "Basic shaping features").
	glyphs = applyIndicPhase3(glyphs, gsub, cfg)

	// Phase 4 visual reordering (Indic spec §6 "Final reordering").
	glyphs = reorderIndicVisual(glyphs, cfg)

	// Phase 5 presentational features (Indic spec §7 "Final features").
	glyphs = applyIndicPhase5(glyphs, gsub)

	out := make([]uint16, len(glyphs))
	for i, g := range glyphs {
		out[i] = g.GID
	}
	return out
}

// assignIndicPositions walks a Consonant syllable and labels each
// glyph slot with its phase-2 positional category. The rules
// implemented here are a direct reading of Indic spec §4 "Initial
// reordering":
//
//  1. If the syllable starts with Ra + halant followed by another
//     consonant, mark those two slots as RephBase / RephHalant.
//     Scripts whose RephPos is rephPosNone skip this step (Tamil,
//     Gurmukhi).
//  2. Find the base consonant: the last consonant in the syllable
//     that is not followed by a halant, unless the syllable ends
//     in a halant cluster in which case it is the last consonant
//     before the trailing halant.
//  3. Consonants before the base that are followed by halant are
//     pre-base half forms.
//  4. The pre-base matra(s) are tagged PreBaseM so phase 4 can
//     find them and move them before the base glyph.
//  5. Modifiers / visarga are tagged SMVD (stay-after-base).
func assignIndicPositions(runes []rune, glyphs []devaGlyph, typ devaSyllableType, cfg *indicScriptConfig) {
	if typ != devaSylConsonant || len(runes) == 0 {
		return
	}

	// Rule 1: detect reph. A leading Ra + halant + consonant
	// triggers reph formation (rphf feature). The trailing
	// consonant must exist for reph to apply; a bare "Ra + halant"
	// at end of syllable is just a dead consonant, not a reph.
	hasReph := false
	if cfg.RephPos != rephPosNone && len(runes) >= 3 &&
		cfg.categoryOf(runes[0]) == devaCatConsonantRa &&
		cfg.categoryOf(runes[1]) == devaCatVirama {
		if cfg.categoryOf(runes[2]) == devaCatConsonant ||
			cfg.categoryOf(runes[2]) == devaCatConsonantRa {
			hasReph = true
			glyphs[0].Pos = devaPosRephBase
			glyphs[1].Pos = devaPosRephHalant
		}
	}

	// Rule 2: find the base consonant.
	baseIdx := -1
	for i := len(runes) - 1; i >= 0; i-- {
		c := cfg.categoryOf(runes[i])
		if c == devaCatConsonant || c == devaCatConsonantRa {
			if hasReph && i == 0 {
				continue
			}
			baseIdx = i
			break
		}
	}
	if baseIdx < 0 {
		return
	}
	glyphs[baseIdx].Pos = devaPosBase

	// Rule 3: consonants before the base that are followed by a
	// halant become pre-base half forms. Consonants after the base
	// (only possible via halant + consonant, i.e. post-base
	// conjunct) become post-base forms.
	startCons := 0
	if hasReph {
		startCons = 2
	}
	for i := startCons; i < baseIdx; i++ {
		c := cfg.categoryOf(runes[i])
		if c == devaCatConsonant || c == devaCatConsonantRa {
			if glyphs[i].Pos == devaPosNone {
				glyphs[i].Pos = devaPosPreBase
			}
		}
	}
	for i := baseIdx + 1; i < len(runes); i++ {
		c := cfg.categoryOf(runes[i])
		switch c {
		case devaCatConsonant, devaCatConsonantRa:
			if glyphs[i].Pos == devaPosNone {
				glyphs[i].Pos = devaPosPostBase
			}
		case devaCatPreBaseMatra:
			glyphs[i].Pos = devaPosPreBaseM
		case devaCatVowelSign:
			glyphs[i].Pos = devaPosAboveBase
		case devaCatModifier, devaCatVisarga:
			glyphs[i].Pos = devaPosSMVD
		}
	}
	// A pre-base matra can also appear between the base and a
	// post-base consonant in exotic inputs; catch it across the
	// whole syllable for robustness.
	for i := 0; i < len(runes); i++ {
		if cfg.categoryOf(runes[i]) == devaCatPreBaseMatra && glyphs[i].Pos == devaPosNone {
			glyphs[i].Pos = devaPosPreBaseM
		}
	}
}

// indicPhase3Features is the ordered list of GSUB features applied
// during phase 3 of Indic shaping (Indic spec §5). Order is
// load-bearing: each feature sees the output of all earlier ones.
// Every supported script uses the same order; scripts that do not
// use a given feature (e.g. Tamil's missing cjct) will see empty
// tables and the pass becomes a no-op.
var indicPhase3Features = [...]font.GSUBFeature{
	font.GSUBNukt,
	font.GSUBAkhn,
	font.GSUBRphf,
	font.GSUBRkrf,
	font.GSUBBlwf,
	font.GSUBHalf,
	font.GSUBPstf,
	font.GSUBVatu,
	font.GSUBCjct,
}

// indicPhase5Features is the ordered list of GSUB features applied
// during phase 5 (Indic spec §7). init is the Indic "init" form
// feature, distinct in semantics from the Arabic init: it marks a
// word-initial form but reuses the same tag name.
var indicPhase5Features = [...]font.GSUBFeature{
	font.GSUBInit,
	font.GSUBPres,
	font.GSUBAbvs,
	font.GSUBBlws,
	font.GSUBPsts,
	font.GSUBHaln,
	font.GSUBCalt,
	font.GSUBClig,
	font.GSUBLiga,
	font.GSUBRlig,
}

// applyIndicPhase3 runs the phase-3 feature stack over the glyph
// list. Each feature is applied only if the GSUB table has entries
// for it, and the three lookup types (Single, Ligature,
// ChainContext) are dispatched independently. Positional metadata
// is preserved across Single substitutions; after a Ligature or
// ChainContext pass that changes the stream length we rebuild
// metadata from the surviving slot positions using a best-effort
// "base slot wins" policy.
//
// Scripts with cfg.HasConjuncts == false (Tamil) skip the entire
// phase-3 pass: the spec states that Tamil does not form
// halant-merge conjuncts, so the conjunct-building features
// (rphf, rkrf, blwf, half, pstf, vatu, akhn, cjct, nukt) do not
// apply. Phase-5 presentational features still run.
func applyIndicPhase3(glyphs []devaGlyph, gsub *font.GSUBSubstitutions, cfg *indicScriptConfig) []devaGlyph {
	if gsub == nil || len(glyphs) == 0 {
		return glyphs
	}
	if cfg != nil && !cfg.HasConjuncts {
		return glyphs
	}
	for _, feat := range indicPhase3Features {
		if table, ok := gsub.Single[feat]; ok && len(table) > 0 {
			for i := range glyphs {
				// The rphf feature is special-cased: it only
				// applies to the RephBase slot.
				if feat == font.GSUBRphf && glyphs[i].Pos != devaPosRephBase {
					continue
				}
				if newGID, found := table[glyphs[i].GID]; found {
					glyphs[i].GID = newGID
				}
			}
		}
		if ligs, ok := gsub.Ligature[feat]; ok && len(ligs) > 0 {
			glyphs = applyIndicLigatureFeature(glyphs, ligs)
		}
		if chain, ok := gsub.ChainContext[feat]; ok && len(chain) > 0 {
			glyphs = applyIndicChainContextFeature(glyphs, gsub, feat)
		}
	}
	return glyphs
}

// applyIndicPhase5 mirrors applyIndicPhase3 but runs the phase-5
// feature stack.
func applyIndicPhase5(glyphs []devaGlyph, gsub *font.GSUBSubstitutions) []devaGlyph {
	if gsub == nil || len(glyphs) == 0 {
		return glyphs
	}
	for _, feat := range indicPhase5Features {
		if table, ok := gsub.Single[feat]; ok && len(table) > 0 {
			for i := range glyphs {
				if newGID, found := table[glyphs[i].GID]; found {
					glyphs[i].GID = newGID
				}
			}
		}
		if ligs, ok := gsub.Ligature[feat]; ok && len(ligs) > 0 {
			glyphs = applyIndicLigatureFeature(glyphs, ligs)
		}
		if chain, ok := gsub.ChainContext[feat]; ok && len(chain) > 0 {
			glyphs = applyIndicChainContextFeature(glyphs, gsub, feat)
		}
	}
	return glyphs
}

// applyIndicLigatureFeature runs a ligature feature across the
// glyph stream. When a ligature fires it consumes N input slots and
// emits one output slot; the surviving slot inherits the first input
// slot's positional metadata so phase-4 reorder can still find the
// base.
func applyIndicLigatureFeature(glyphs []devaGlyph, table map[uint16][]font.LigatureSubst) []devaGlyph {
	if len(glyphs) == 0 {
		return glyphs
	}
	out := make([]devaGlyph, 0, len(glyphs))
	i := 0
	for i < len(glyphs) {
		candidates := table[glyphs[i].GID]
		fired := false
		for _, cand := range candidates {
			need := len(cand.Components)
			if i+1+need > len(glyphs) {
				continue
			}
			matched := true
			for j := 0; j < need; j++ {
				if glyphs[i+1+j].GID != cand.Components[j] {
					matched = false
					break
				}
			}
			if !matched {
				continue
			}
			pos := glyphs[i].Pos
			for j := 0; j <= need; j++ {
				if glyphs[i+j].Pos == devaPosBase {
					pos = devaPosBase
					break
				}
			}
			out = append(out, devaGlyph{GID: cand.LigatureGID, Pos: pos})
			i += 1 + need
			fired = true
			break
		}
		if !fired {
			out = append(out, glyphs[i])
			i++
		}
	}
	return out
}

// applyIndicChainContextFeature runs a chaining contextual feature
// by delegating to the GSUBSubstitutions helper that the core GSUB
// code already exposes for Arabic shaping.
func applyIndicChainContextFeature(glyphs []devaGlyph, gsub *font.GSUBSubstitutions, feat font.GSUBFeature) []devaGlyph {
	gids := make([]uint16, len(glyphs))
	for i, g := range glyphs {
		gids[i] = g.GID
	}
	after := gsub.ApplyChainContext(gids, feat)
	if len(after) == len(gids) {
		for i := range glyphs {
			glyphs[i].GID = after[i]
		}
		return glyphs
	}
	out := make([]devaGlyph, len(after))
	for i, gid := range after {
		out[i] = devaGlyph{GID: gid}
	}
	return out
}

// reorderIndicVisual performs the phase-4 reordering (Indic spec §6):
//
//  1. If the syllable has a pre-base matra (PosPreBaseM), move it
//     to immediately before the base glyph.
//  2. If the syllable had a reph (the leading Ra + halant that the
//     rphf feature collapsed into one glyph), move that glyph
//     according to the per-script reph-position policy. Devanagari
//     puts reph immediately after the base; Kannada / Telugu put
//     it after the last post-base form; Malayalam puts it before
//     the first post-base matra; Bengali (rephPosAfterMain) places
//     it after the last non-modifier slot in the syllable, which
//     keeps reph after any post-base matras while staying before
//     trailing visarga / candrabindu.
//  3. Everything else stays in logical order.
func reorderIndicVisual(glyphs []devaGlyph, cfg *indicScriptConfig) []devaGlyph {
	baseIdx := -1
	for i, g := range glyphs {
		if g.Pos == devaPosBase {
			baseIdx = i
			break
		}
	}
	if baseIdx < 0 {
		return glyphs
	}

	// Locate every pre-base matra (there can be more than one in
	// Malayalam) and the reph glyph (if any).
	var preMatraIdxs []int
	rephIdx := -1
	rephHalantIdx := -1
	for i, g := range glyphs {
		switch g.Pos {
		case devaPosPreBaseM:
			preMatraIdxs = append(preMatraIdxs, i)
		case devaPosRephBase:
			rephIdx = i
		case devaPosRephHalant:
			rephHalantIdx = i
		}
	}

	// Compute the insertion position for the reph based on the
	// per-script policy.
	rephInsertAfter := baseIdx
	switch cfg.RephPos {
	case rephPosAfterBase:
		rephInsertAfter = baseIdx
	case rephPosAfterPostBase:
		rephInsertAfter = baseIdx
		for i := baseIdx + 1; i < len(glyphs); i++ {
			if glyphs[i].Pos == devaPosPostBase || glyphs[i].Pos == devaPosBelowBase {
				rephInsertAfter = i
			}
		}
	case rephPosBeforePostBase:
		// Insert reph right after the base but before any post-base
		// matra (AboveBase) — since post-base matras follow the base
		// in logical order, "after base, before first above-base"
		// translates to inserting right after the base in our stream.
		rephInsertAfter = baseIdx
	case rephPosAfterMain:
		// End-of-syllable placement: insert after the last non-SMVD
		// slot so that SMVD modifiers still trail the reph.
		rephInsertAfter = len(glyphs) - 1
		for i := len(glyphs) - 1; i >= 0; i-- {
			if glyphs[i].Pos != devaPosSMVD {
				rephInsertAfter = i
				break
			}
		}
	}

	// Build the output in the target visual order.
	result := make([]devaGlyph, 0, len(glyphs))
	skip := func(i int) bool {
		if i == rephIdx || i == rephHalantIdx {
			return true
		}
		for _, p := range preMatraIdxs {
			if i == p {
				return true
			}
		}
		return false
	}
	emitReph := func() {
		if rephIdx < 0 {
			return
		}
		result = append(result, glyphs[rephIdx])
		if rephHalantIdx >= 0 {
			result = append(result, glyphs[rephHalantIdx])
		}
	}
	for i := 0; i < len(glyphs); i++ {
		if skip(i) {
			continue
		}
		if i == baseIdx {
			for _, p := range preMatraIdxs {
				result = append(result, glyphs[p])
			}
			result = append(result, glyphs[i])
			if cfg.RephPos == rephPosAfterBase {
				emitReph()
			}
			if cfg.RephPos == rephPosBeforePostBase {
				emitReph()
			}
			// AfterPostBase / AfterMain: if the chosen insertion point
			// IS the base (no post-base slots in the syllable), emit
			// reph here — otherwise it will be emitted after the
			// post-base slot below.
			if (cfg.RephPos == rephPosAfterPostBase || cfg.RephPos == rephPosAfterMain) &&
				rephInsertAfter == baseIdx {
				emitReph()
			}
			continue
		}
		result = append(result, glyphs[i])
		if (cfg.RephPos == rephPosAfterPostBase || cfg.RephPos == rephPosAfterMain) &&
			i == rephInsertAfter && rephInsertAfter != baseIdx {
			emitReph()
		}
	}
	return result
}

// ShapeIndicWithEmbedded is the script-dispatching convenience
// wrapper used by paragraph layout. It picks the right config based
// on the UAX #24 script of the input, pulls GSUB tables from the
// embedded font, and runs the shaper. Returns (gids, true) on
// success; (nil, false) if the script is not Indic, the font is
// nil, or the input is empty.
func ShapeIndicWithEmbedded(s string, ef *font.EmbeddedFont, sc Script) ([]uint16, bool) {
	if ef == nil {
		return nil, false
	}
	cfg := indicConfigFor(sc)
	if cfg == nil {
		return nil, false
	}
	face := ef.Face()
	if face == nil {
		return nil, false
	}
	gsub := face.GSUB()
	gids := ShapeIndic(s, face, gsub, cfg)
	if len(gids) == 0 {
		return nil, false
	}
	return gids, true
}

// ShapeDevanagariWithEmbedded is retained for backwards
// compatibility. New callers should use ShapeIndicWithEmbedded with
// the resolved script.
func ShapeDevanagariWithEmbedded(s string, ef *font.EmbeddedFont) ([]uint16, bool) {
	return ShapeIndicWithEmbedded(s, ef, ScriptDevanagari)
}

// isIndicScript reports whether sc is one of the scripts the Indic
// shaper can handle.
func isIndicScript(sc Script) bool {
	return indicConfigFor(sc) != nil
}

// indicScriptOfWord returns the first Indic script seen in s, or
// ScriptCommon if none is present. Since splitMixedBidiWord has
// already segmented at script transitions before this runs, any
// Indic rune in s means the whole word is that script.
func indicScriptOfWord(s string) Script {
	for _, r := range s {
		sc := ScriptOf(r)
		if isIndicScript(sc) {
			return sc
		}
	}
	return ScriptCommon
}
