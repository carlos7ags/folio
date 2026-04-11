// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "github.com/carlos7ags/folio/font"

// Devanagari OpenType shaping engine.
//
// This file implements the Devanagari-branch shaper described in the
// Microsoft "Creating and supporting OpenType fonts for the Indic
// scripts" specification, Devanagari section. References in the code
// use the short form "Indic spec §<section>" so reviewers can audit
// against the authoritative text. Only the spec text was consulted —
// no code from other implementations was read or copied.
//
// Pipeline:
//
//	Phase 1: scanDevanagariSyllables identifies syllable cluster
//	         boundaries using the regex grammar from Indic spec §2
//	         (Syllables). Each syllable is typed as Consonant / Vowel /
//	         Standalone / Symbol / Number / Other / Broken.
//
//	Phase 2: reorderDevanagariCluster walks a Consonant cluster and
//	         assigns positional categories (base, pre-base matra, reph,
//	         half forms, below-base, post-base). It does NOT move the
//	         pre-base matra yet — that happens in phase 4 after the
//	         GSUB substitutions have run, matching Indic spec
//	         "Reordering" §4.
//
//	Phase 3: applyDevanagariFeatures3 dispatches the phase-3 GSUB
//	         features in the spec's required order: nukt, akhn, rphf,
//	         rkrf, blwf, half, pstf, vatu, cjct. rphf is applied only
//	         to the reph span so it does not fire mid-syllable; every
//	         other feature runs over the whole cluster.
//
//	Phase 4: reorderDevanagariVisual performs post-substitution
//	         reordering: the pre-base matra glyph moves to immediately
//	         before the base glyph, and the reph (a single glyph once
//	         rphf has fired) moves to immediately after the base per
//	         the Indic spec "Final reordering" rules.
//
//	Phase 5: applyDevanagariFeatures5 dispatches the phase-5 feature
//	         set: init, pres, abvs, blws, psts, haln, calt, clig, liga,
//	         rlig. This is the standard presentational pass.
//
// The entry point ShapeDevanagari runs all five phases and returns a
// uint16 glyph stream suitable for the draw path.

// Devanagari block constants.
const (
	devaBlockStart = 0x0900
	devaBlockEnd   = 0x097F

	devaVirama    = 0x094D // halant
	devaNukta     = 0x093C
	devaRa        = 0x0930
	devaEyelashRa = 0x0931 // ra with middle diagonal stroke
	devaPreBaseMI = 0x093F // vowel sign I (pre-base)
	devaZWJ       = 0x200D
	devaZWNJ      = 0x200C
)

// devaCategory classifies a Devanagari codepoint by its OpenType
// shaping role. Categories derive from Indic spec §3 ("Character
// category" table) and from UAX #44 Indic_Syllabic_Category values
// where the spec refers back to them. Only the runes this shaper
// needs to distinguish are listed; everything else collapses to
// devaCatOther and is carried through untouched.
type devaCategory uint8

const (
	devaCatOther devaCategory = iota
	devaCatConsonant
	devaCatConsonantRa   // the special Ra that can form reph / rakar
	devaCatVowel         // independent vowel letter
	devaCatVowelSign     // dependent matra (non pre-base)
	devaCatPreBaseMatra  // pre-base vowel sign (U+093F specifically)
	devaCatNukta         // combining nukta
	devaCatVirama        // halant / virama
	devaCatModifier      // anusvara, candrabindu
	devaCatVisarga       // visarga
	devaCatJoiner        // ZWJ
	devaCatNonJoiner     // ZWNJ
	devaCatNumber        // digit
	devaCatPunctuation   // danda, double danda, etc.
	devaCatIndependentVS // Vowel sign AA / AU / etc. (kept with vowel signs)
)

// devaCategoryOf returns the devaCategory for r. Runes outside the
// Devanagari block return devaCatOther.
//
// The table below is short (~130 assigned codepoints in the main
// Devanagari block). It is encoded as a flat switch on rune literals
// so the compiler can turn it into a binary search or a jump table;
// at ~150 entries the cost is negligible and it keeps the per-rune
// cost at O(log n) without building a runtime map. Entries follow
// Unicode 15.0 Devanagari block assignments.
func devaCategoryOf(r rune) devaCategory {
	// ZWJ / ZWNJ are in the general-punctuation block but matter for
	// Indic cluster semantics; check them before the block test.
	if r == devaZWJ {
		return devaCatJoiner
	}
	if r == devaZWNJ {
		return devaCatNonJoiner
	}
	if r < devaBlockStart || r > devaBlockEnd {
		return devaCatOther
	}
	switch r {
	// Signs and marks first (the non-consonant assignments).
	case 0x0900, 0x0901: // inverted candrabindu, candrabindu
		return devaCatModifier
	case 0x0902: // anusvara
		return devaCatModifier
	case 0x0903: // visarga
		return devaCatVisarga
	case 0x093A: // vowel sign oe
		return devaCatVowelSign
	case 0x093B: // vowel sign ooe
		return devaCatVowelSign
	case devaNukta:
		return devaCatNukta
	case 0x093D: // avagraha
		return devaCatOther
	case 0x093E: // sign AA
		return devaCatVowelSign
	case devaPreBaseMI:
		return devaCatPreBaseMatra
	case 0x0940: // sign II
		return devaCatVowelSign
	case 0x0941, 0x0942, 0x0943, 0x0944: // U, UU, vocalic R, vocalic RR
		return devaCatVowelSign
	case 0x0945, 0x0946, 0x0947, 0x0948: // candra E, short E, E, AI
		return devaCatVowelSign
	case 0x0949, 0x094A, 0x094B, 0x094C: // candra O, short O, O, AU
		return devaCatVowelSign
	case devaVirama:
		return devaCatVirama
	case 0x094E, 0x094F: // prishthamatra E, AW
		return devaCatVowelSign
	case 0x0950: // OM
		return devaCatOther
	case 0x0951, 0x0952, 0x0953, 0x0954: // stress/accent marks
		return devaCatOther
	case 0x0962, 0x0963: // vowel sign vocalic L / LL
		return devaCatVowelSign
	case 0x0964, 0x0965, 0x0970: // danda, double danda, abbreviation sign
		return devaCatPunctuation
	}
	// Digits U+0966..U+096F.
	if r >= 0x0966 && r <= 0x096F {
		return devaCatNumber
	}
	// Independent vowels U+0904..U+0914, plus U+0960..U+0961.
	if (r >= 0x0904 && r <= 0x0914) || r == 0x0960 || r == 0x0961 {
		return devaCatVowel
	}
	// Consonant Ra gets its own category so reph/rakar logic can find
	// it without a second table lookup.
	if r == devaRa || r == devaEyelashRa {
		return devaCatConsonantRa
	}
	// Consonants U+0915..U+0939 (excluding the ra carve-outs above) and
	// U+0958..U+095F (nukta-bearing precomposed consonants).
	if r >= 0x0915 && r <= 0x0939 {
		return devaCatConsonant
	}
	if r >= 0x0958 && r <= 0x095F {
		return devaCatConsonant
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
// cluster by scanDevanagariSyllables.
type devaSyllable struct {
	StartRune int              // inclusive rune index
	EndRune   int              // exclusive rune index
	Type      devaSyllableType // Consonant / Vowel / ...
}

// scanDevanagariSyllables walks runes once and yields the syllable
// cluster boundaries. The grammar implemented here is a simplified
// form of Indic spec §2:
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
// The scanner is greedy and never looks back: once a cluster boundary
// is decided it is not revisited. Complex scripts that need backtracking
// (Malayalam chillus, Tamil sri) are out of scope for this PR.
func scanDevanagariSyllables(runes []rune) []devaSyllable {
	if len(runes) == 0 {
		return nil
	}
	var out []devaSyllable
	i := 0
	for i < len(runes) {
		start := i
		cat := devaCategoryOf(runes[i])
		switch cat {
		case devaCatNumber:
			for i < len(runes) && devaCategoryOf(runes[i]) == devaCatNumber {
				i++
			}
			out = append(out, devaSyllable{start, i, devaSylNumber})
		case devaCatPunctuation:
			i++
			out = append(out, devaSyllable{start, i, devaSylPunctuation})
		case devaCatVowel:
			// V (N? VS?) M* MOD? VIS? — consume optional nukta, then
			// matra runs and trailing modifiers.
			i++
			i = consumeDevaTail(runes, i)
			out = append(out, devaSyllable{start, i, devaSylVowel})
		case devaCatConsonant, devaCatConsonantRa:
			// (C N? H)* C N? M* MOD? VIS?
			i = consumeDevaConsonantCluster(runes, i)
			i = consumeDevaTail(runes, i)
			out = append(out, devaSyllable{start, i, devaSylConsonant})
		case devaCatVirama, devaCatNukta:
			// A leading halant or nukta starts a "Standalone" cluster:
			// the spec inserts a dotted-circle base. We still emit a
			// single syllable so the caller sees the run.
			i++
			for i < len(runes) {
				c := devaCategoryOf(runes[i])
				if c == devaCatVirama || c == devaCatNukta || c == devaCatVowelSign ||
					c == devaCatPreBaseMatra || c == devaCatModifier || c == devaCatVisarga {
					i++
					continue
				}
				break
			}
			out = append(out, devaSyllable{start, i, devaSylStandalone})
		case devaCatJoiner, devaCatNonJoiner:
			// Stray ZWJ/ZWNJ — emit as Other so the caller passes it
			// through verbatim.
			i++
			out = append(out, devaSyllable{start, i, devaSylOther})
		default:
			// Independent marks, modifiers, symbols, or anything
			// uncategorised. Coalesce a run of non-cluster-starting
			// material into one Other syllable.
			i++
			for i < len(runes) {
				c := devaCategoryOf(runes[i])
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

// consumeDevaConsonantCluster walks the (C N? H)* C N? prefix of a
// Consonant syllable starting at runes[i] and returns the new index
// positioned just past the base consonant (optionally followed by its
// nukta). The caller then walks the matra tail separately.
func consumeDevaConsonantCluster(runes []rune, i int) int {
	for i < len(runes) {
		c := devaCategoryOf(runes[i])
		if c != devaCatConsonant && c != devaCatConsonantRa {
			break
		}
		i++
		// Optional nukta immediately after the consonant.
		if i < len(runes) && devaCategoryOf(runes[i]) == devaCatNukta {
			i++
		}
		// If the next char is a halant, continue the loop; otherwise
		// the current consonant is the base and we are done.
		if i < len(runes) && devaCategoryOf(runes[i]) == devaCatVirama {
			i++
			// ZWJ/ZWNJ immediately after halant controls whether a
			// half-form is forced; we consume them with the halant so
			// they travel inside the cluster.
			if i < len(runes) {
				c2 := devaCategoryOf(runes[i])
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

// consumeDevaTail walks the trailing matra / modifier / visarga /
// halant run after a base consonant or independent vowel.
func consumeDevaTail(runes []rune, i int) int {
	for i < len(runes) {
		c := devaCategoryOf(runes[i])
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

// devaGlyph is the per-position payload used during cluster shaping:
// the current GID plus the phase-2 positional metadata needed by the
// phase-4 visual reordering pass. Glyphs produced by GSUB that collapse
// several inputs (ligatures, rphf) preserve the metadata of the input
// slot that survived.
type devaGlyph struct {
	GID uint16

	// Positional category assigned during phase 2. Once set it carries
	// through GSUB so reorder can find the base and the pre-base matra
	// even after Single-substitution renaming.
	Pos devaGlyphPos
}

// devaGlyphPos is the positional role assigned to a glyph during
// phase 2 ("Initial reordering"). See Indic spec §4.
type devaGlyphPos uint8

const (
	devaPosNone       devaGlyphPos = iota
	devaPosBase                    // base consonant (or rphf-substituted reph that will move)
	devaPosPreBase                 // half form or other pre-base consonant
	devaPosPreBaseM                // pre-base matra (U+093F before substitution)
	devaPosAboveBase               // above-base sign (matras, modifiers)
	devaPosBelowBase               // below-base form consonant
	devaPosPostBase                // post-base form
	devaPosRephBase                // the leading Ra that becomes reph
	devaPosRephHalant              // the halant after the leading Ra
	devaPosSMVD                    // modifier / visarga (stay after base)
)

// ShapeDevanagari is the entry point used by layout. It runs the full
// five-phase Indic shaping pipeline on s and returns a glyph ID stream
// in visual order, ready for measurement and draw.
//
// When gsub is nil or the face has no Devanagari features, the function
// still runs the scanner and phase-2/phase-4 reordering so that the
// returned GID stream is in visual order using the base codepoint GIDs.
// This is the "no-GSUB fallback" and it renders passably for fonts that
// have Devanagari glyphs but no shaping tables.
func ShapeDevanagari(s string, face font.Face, gsub *font.GSUBSubstitutions) []uint16 {
	runes := []rune(s)
	if len(runes) == 0 || face == nil {
		return nil
	}
	syllables := scanDevanagariSyllables(runes)
	if len(syllables) == 0 {
		return nil
	}
	var out []uint16
	for _, syl := range syllables {
		shaped := shapeDevanagariSyllable(runes[syl.StartRune:syl.EndRune], syl.Type, face, gsub)
		out = append(out, shaped...)
	}
	return out
}

// shapeDevanagariSyllable runs the per-syllable pipeline: build the
// initial glyph stream, assign phase-2 categories, apply phase-3 GSUB,
// do phase-4 visual reordering, apply phase-5 GSUB, and return the
// final GIDs.
func shapeDevanagariSyllable(runes []rune, typ devaSyllableType, face font.Face, gsub *font.GSUBSubstitutions) []uint16 {
	// Fast path for non-cluster syllables: map rune-to-GID and return.
	// Phase 3/4/5 features don't apply to these types.
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
	assignDevaPositions(runes, glyphs, typ)

	// Phase 3 GSUB features (Indic spec §5 "Basic shaping features").
	glyphs = applyDevaPhase3(glyphs, gsub)

	// Phase 4 visual reordering (Indic spec §6 "Final reordering").
	glyphs = reorderDevaVisual(glyphs)

	// Phase 5 presentational features (Indic spec §7 "Final features").
	glyphs = applyDevaPhase5(glyphs, gsub)

	out := make([]uint16, len(glyphs))
	for i, g := range glyphs {
		out[i] = g.GID
	}
	return out
}

// assignDevaPositions walks a Consonant syllable and labels each glyph
// slot with its phase-2 positional category. The rules implemented
// here are a direct reading of Indic spec §4 "Initial reordering":
//
//  1. If the syllable starts with Ra + halant followed by another
//     consonant, mark those two slots as RephBase / RephHalant.
//  2. Find the base consonant: the last consonant in the syllable that
//     is not followed by a halant, unless the syllable ends in a
//     halant cluster in which case it is the last consonant before
//     the trailing halant.
//  3. Consonants before the base that are followed by halant are
//     pre-base half forms.
//  4. The pre-base matra U+093F is tagged PreBaseM so phase 4 can
//     find it and move it before the base glyph.
//  5. Modifiers / visarga are tagged SMVD (stay-after-base).
func assignDevaPositions(runes []rune, glyphs []devaGlyph, typ devaSyllableType) {
	if typ != devaSylConsonant || len(runes) == 0 {
		return
	}

	// Rule 1: detect reph. A leading Ra + halant + consonant triggers
	// reph formation (rphf feature). The trailing consonant must
	// exist for reph to apply; a bare "Ra + halant" at end of syllable
	// is just a dead consonant, not a reph.
	hasReph := false
	if len(runes) >= 3 &&
		devaCategoryOf(runes[0]) == devaCatConsonantRa &&
		devaCategoryOf(runes[1]) == devaCatVirama {
		// The third rune must be a consonant (not ZWJ/ZWNJ); ZWJ after
		// a halant forces a half form and suppresses reph.
		if devaCategoryOf(runes[2]) == devaCatConsonant ||
			devaCategoryOf(runes[2]) == devaCatConsonantRa {
			hasReph = true
			glyphs[0].Pos = devaPosRephBase
			glyphs[1].Pos = devaPosRephHalant
		}
	}

	// Rule 2: find the base consonant. Start from the end and walk
	// backwards past matras and trailing marks; the first consonant we
	// hit (that is not part of a reph) is the base.
	baseIdx := -1
	for i := len(runes) - 1; i >= 0; i-- {
		c := devaCategoryOf(runes[i])
		if c == devaCatConsonant || c == devaCatConsonantRa {
			// Skip the reph slot if present — it is not the base.
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
	// (only possible via halant + consonant, i.e. post-base conjunct)
	// become post-base forms.
	startCons := 0
	if hasReph {
		startCons = 2 // skip Ra + halant
	}
	for i := startCons; i < baseIdx; i++ {
		c := devaCategoryOf(runes[i])
		if c == devaCatConsonant || c == devaCatConsonantRa {
			if glyphs[i].Pos == devaPosNone {
				glyphs[i].Pos = devaPosPreBase
			}
		}
	}
	for i := baseIdx + 1; i < len(runes); i++ {
		c := devaCategoryOf(runes[i])
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
	// post-base consonant in exotic inputs; catch it across the whole
	// syllable for robustness.
	for i := 0; i < len(runes); i++ {
		if devaCategoryOf(runes[i]) == devaCatPreBaseMatra && glyphs[i].Pos == devaPosNone {
			glyphs[i].Pos = devaPosPreBaseM
		}
	}
}

// devaPhase3Features is the ordered list of GSUB features applied
// during phase 3 of Indic shaping (Indic spec §5). Order is
// load-bearing: each feature sees the output of all earlier ones.
var devaPhase3Features = [...]font.GSUBFeature{
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

// devaPhase5Features is the ordered list of GSUB features applied
// during phase 5 (Indic spec §7). init is the Indic "init" form
// feature, distinct in semantics from the Arabic init: it marks a
// word-initial form but reuses the same tag name.
var devaPhase5Features = [...]font.GSUBFeature{
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

// applyDevaPhase3 runs the phase-3 feature stack over the glyph list.
// Each feature is applied only if the GSUB table has entries for it,
// and the three lookup types (Single, Ligature, ChainContext) are
// dispatched independently. Positional metadata is preserved across
// Single substitutions; after a Ligature or ChainContext pass that
// changes the stream length we rebuild metadata from the surviving
// slot positions using a best-effort "base slot wins" policy — the
// phase-4 reorder only needs to know where the base, the pre-base
// matra, and the reph live, and each of those survives single-slot
// substitutions unchanged.
func applyDevaPhase3(glyphs []devaGlyph, gsub *font.GSUBSubstitutions) []devaGlyph {
	if gsub == nil || len(glyphs) == 0 {
		return glyphs
	}
	for _, feat := range devaPhase3Features {
		// Single substitutions: in-place rename, preserves positions.
		if table, ok := gsub.Single[feat]; ok && len(table) > 0 {
			for i := range glyphs {
				// The rphf feature is special-cased: it only applies
				// to the RephBase slot, so the spec guarantees the
				// substitution fires at position 0 (our leading Ra).
				// We still let Single entries fire anywhere they
				// match; this is a no-op for well-formed fonts.
				if feat == font.GSUBRphf && glyphs[i].Pos != devaPosRephBase {
					continue
				}
				if newGID, found := table[glyphs[i].GID]; found {
					glyphs[i].GID = newGID
				}
			}
		}
		// Ligatures: may collapse multiple slots into one.
		if ligs, ok := gsub.Ligature[feat]; ok && len(ligs) > 0 {
			glyphs = applyDevaLigatureFeature(glyphs, ligs)
		}
		// Chain contextual: dispatch through the merged GSUB helper.
		if chain, ok := gsub.ChainContext[feat]; ok && len(chain) > 0 {
			glyphs = applyDevaChainContextFeature(glyphs, gsub, feat)
		}
	}
	// After rphf: if a RephBase+RephHalant pair collapsed into one
	// surviving slot, the spec expects the combined glyph to behave as
	// a single "reph" glyph for phase 4 reordering. We tag that slot as
	// RephBase so reorder can move it.
	return glyphs
}

// applyDevaPhase5 mirrors applyDevaPhase3 but runs the phase-5 feature
// stack. No positional semantics remain at this point — the reorder
// pass has already moved the pre-base matra and the reph — so we do
// not need to rebuild metadata after multi-slot substitutions.
func applyDevaPhase5(glyphs []devaGlyph, gsub *font.GSUBSubstitutions) []devaGlyph {
	if gsub == nil || len(glyphs) == 0 {
		return glyphs
	}
	for _, feat := range devaPhase5Features {
		if table, ok := gsub.Single[feat]; ok && len(table) > 0 {
			for i := range glyphs {
				if newGID, found := table[glyphs[i].GID]; found {
					glyphs[i].GID = newGID
				}
			}
		}
		if ligs, ok := gsub.Ligature[feat]; ok && len(ligs) > 0 {
			glyphs = applyDevaLigatureFeature(glyphs, ligs)
		}
		if chain, ok := gsub.ChainContext[feat]; ok && len(chain) > 0 {
			glyphs = applyDevaChainContextFeature(glyphs, gsub, feat)
		}
	}
	return glyphs
}

// applyDevaLigatureFeature runs a ligature feature across the glyph
// stream. When a ligature fires it consumes N input slots and emits
// one output slot; the surviving slot inherits the first input slot's
// positional metadata so phase-4 reorder can still find the base.
func applyDevaLigatureFeature(glyphs []devaGlyph, table map[uint16][]font.LigatureSubst) []devaGlyph {
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
			// Preserve the first slot's position, but if any consumed
			// slot was the base, the combined glyph becomes the base.
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

// applyDevaChainContextFeature runs a chaining contextual feature by
// delegating to the GSUBSubstitutions helper that the core GSUB code
// already exposes for Arabic shaping. We extract GIDs, run the
// feature's ApplyChainContext, and merge the result back into the
// positional glyph stream. Chain context lookups typically do not
// change the stream length — they rewrite GIDs in place via their
// Actions — but when they do, positional metadata on affected slots
// may desynchronise; the Indic spec's phase-3 chain context features
// are used for contextual shaping of an already-identified base
// consonant, so a length-preserving in-place rewrite is the common
// case and the only one we implement robustly here.
func applyDevaChainContextFeature(glyphs []devaGlyph, gsub *font.GSUBSubstitutions, feat font.GSUBFeature) []devaGlyph {
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
	// Length changed: rebuild the stream from scratch with no
	// positional metadata. Phase-4 reorder will no-op on these slots
	// which is acceptable: a chain-context rewrite that collapses
	// glyphs has already decided the visual order on its own.
	out := make([]devaGlyph, len(after))
	for i, gid := range after {
		out[i] = devaGlyph{GID: gid}
	}
	return out
}

// reorderDevaVisual performs the phase-4 reordering (Indic spec §6):
//
//  1. If the syllable has a pre-base matra (PosPreBaseM), move it to
//     immediately before the base glyph.
//  2. If the syllable had a reph (the leading Ra + halant that the
//     rphf feature collapsed into one glyph), move that glyph to
//     immediately after the base. Note: some fonts place reph at the
//     end of the syllable rather than right after the base; this
//     shaper uses "right after the base" per the core spec text.
//     Fonts that want a different reph position handle it via the
//     phase-5 presentation features instead.
//  3. Everything else stays in logical order.
//
// Glyphs with no position (PosNone) travel with the slot they were
// adjacent to, preserving the relative order of matras, modifiers,
// and trailing marks.
func reorderDevaVisual(glyphs []devaGlyph) []devaGlyph {
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

	// Locate the pre-base matra and the reph glyph (if any).
	preMatraIdx := -1
	rephIdx := -1
	rephHalantIdx := -1
	for i, g := range glyphs {
		switch g.Pos {
		case devaPosPreBaseM:
			preMatraIdx = i
		case devaPosRephBase:
			rephIdx = i
		case devaPosRephHalant:
			rephHalantIdx = i
		}
	}

	// Build the output in the target visual order. We emit slots by
	// walking the original in logical order and re-emitting them
	// under the new rules: pre-base matra slot is skipped where it
	// lives and inserted before the base; reph slot(s) are skipped
	// where they live and inserted right after the base.
	result := make([]devaGlyph, 0, len(glyphs))
	skip := func(i int) bool {
		return i == preMatraIdx || i == rephIdx || i == rephHalantIdx
	}
	for i := 0; i < len(glyphs); i++ {
		if skip(i) {
			continue
		}
		if i == baseIdx {
			// Insert pre-base matra immediately before the base.
			if preMatraIdx >= 0 {
				result = append(result, glyphs[preMatraIdx])
			}
			// Emit the base itself.
			result = append(result, glyphs[i])
			// Insert reph immediately after the base. If the rphf
			// feature collapsed Ra+halant into one glyph, rephIdx is
			// set and rephHalantIdx is -1 (because the halant slot
			// was consumed); if both are set, we emit both so a font
			// without rphf still renders in visual order.
			if rephIdx >= 0 {
				result = append(result, glyphs[rephIdx])
			}
			if rephHalantIdx >= 0 {
				result = append(result, glyphs[rephHalantIdx])
			}
			continue
		}
		result = append(result, glyphs[i])
	}
	return result
}

// ShapeDevanagariWithEmbedded is a convenience wrapper that pulls GSUB
// tables from an EmbeddedFont's underlying Face and runs the shaper.
// Returns (gids, true) on success; (nil, false) if the face is nil or
// has no Devanagari shaping support at all (in which case the caller
// should fall back to the unshaped text path).
//
// This wrapper is used by the layout pipeline in paragraph.go as the
// direct analogue of ShapeArabicWithFont: it takes an EmbeddedFont,
// returns a GID stream, and leaves the caller to decide how to splice
// that stream onto a Word.
func ShapeDevanagariWithEmbedded(s string, ef *font.EmbeddedFont) ([]uint16, bool) {
	if ef == nil {
		return nil, false
	}
	face := ef.Face()
	if face == nil {
		return nil, false
	}
	var gsub *font.GSUBSubstitutions
	if gp, ok := face.(font.GSUBProvider); ok {
		gsub = gp.GSUB()
	}
	gids := ShapeDevanagari(s, face, gsub)
	if len(gids) == 0 {
		return nil, false
	}
	return gids, true
}
