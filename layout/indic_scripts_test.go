// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"reflect"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// mockIndicFace is a script-agnostic stand-in for a real Indic font.
// Each codepoint maps deterministically to GID = rune + 1000 unless
// an explicit override is supplied. GID 0 is reserved for "missing
// glyph" so that 0 in assertions always signals a real miss.
type mockIndicFace struct {
	overrides     map[rune]uint16
	substitutions *font.GSUBSubstitutions
}

func (m *mockIndicFace) PostScriptName() string { return "MockIndic" }
func (m *mockIndicFace) UnitsPerEm() int        { return 1000 }
func (m *mockIndicFace) GlyphIndex(r rune) uint16 {
	if gid, ok := m.overrides[r]; ok {
		return gid
	}
	return uint16(r) + 1000
}
func (m *mockIndicFace) GlyphAdvance(uint16) int       { return 500 }
func (m *mockIndicFace) Ascent() int                   { return 800 }
func (m *mockIndicFace) Descent() int                  { return -200 }
func (m *mockIndicFace) BBox() [4]int                  { return [4]int{0, -200, 1000, 800} }
func (m *mockIndicFace) ItalicAngle() float64          { return 0 }
func (m *mockIndicFace) CapHeight() int                { return 700 }
func (m *mockIndicFace) StemV() int                    { return 80 }
func (m *mockIndicFace) Kern(uint16, uint16) int       { return 0 }
func (m *mockIndicFace) Flags() uint32                 { return 0 }
func (m *mockIndicFace) RawData() []byte               { return nil }
func (m *mockIndicFace) NumGlyphs() int                { return 65535 }
func (m *mockIndicFace) GSUB() *font.GSUBSubstitutions { return m.substitutions }
func (m *mockIndicFace) GIDToUnicode() map[uint16]rune { return nil }
func (m *mockIndicFace) GPOS() *font.GPOSAdjustments   { return nil }

func gid(r rune) uint16 { return uint16(r) + 1000 }

// --- Category classification per script ------------------------------------

func TestIndicCategoryPerScript(t *testing.T) {
	type row struct {
		cfg  *indicScriptConfig
		r    rune
		want devaCategory
		name string
	}
	rows := []row{
		// Bengali
		{bengaliConfig, 0x0995, devaCatConsonant, "bengali ka"},
		{bengaliConfig, 0x09B0, devaCatConsonantRa, "bengali ra"},
		{bengaliConfig, 0x09CD, devaCatVirama, "bengali virama"},
		{bengaliConfig, 0x09BF, devaCatPreBaseMatra, "bengali i-matra"},
		{bengaliConfig, 0x09BE, devaCatVowelSign, "bengali aa-matra"},
		// Gujarati
		{gujaratiConfig, 0x0A95, devaCatConsonant, "gujarati ka"},
		{gujaratiConfig, 0x0AB0, devaCatConsonantRa, "gujarati ra"},
		{gujaratiConfig, 0x0ACD, devaCatVirama, "gujarati virama"},
		{gujaratiConfig, 0x0ABF, devaCatPreBaseMatra, "gujarati i-matra"},
		// Gurmukhi
		{gurmukhiConfig, 0x0A15, devaCatConsonant, "gurmukhi ka"},
		{gurmukhiConfig, 0x0A4D, devaCatVirama, "gurmukhi virama"},
		{gurmukhiConfig, 0x0A3F, devaCatPreBaseMatra, "gurmukhi sihari"},
		{gurmukhiConfig, 0x0A70, devaCatModifier, "gurmukhi tippi"},
		// Kannada
		{kannadaConfig, 0x0C95, devaCatConsonant, "kannada ka"},
		{kannadaConfig, 0x0CB0, devaCatConsonantRa, "kannada ra"},
		{kannadaConfig, 0x0CCD, devaCatVirama, "kannada virama"},
		{kannadaConfig, 0x0CBF, devaCatPreBaseMatra, "kannada i-matra"},
		// Malayalam
		{malayalamConfig, 0x0D15, devaCatConsonant, "malayalam ka"},
		{malayalamConfig, 0x0D30, devaCatConsonantRa, "malayalam ra"},
		{malayalamConfig, 0x0D4D, devaCatVirama, "malayalam virama"},
		{malayalamConfig, 0x0D46, devaCatPreBaseMatra, "malayalam e-matra"},
		{malayalamConfig, 0x0D7A, devaCatConsonant, "malayalam chillu nn"},
		// Oriya
		{oriyaConfig, 0x0B15, devaCatConsonant, "oriya ka"},
		{oriyaConfig, 0x0B30, devaCatConsonantRa, "oriya ra"},
		{oriyaConfig, 0x0B4D, devaCatVirama, "oriya virama"},
		{oriyaConfig, 0x0B47, devaCatPreBaseMatra, "oriya e-matra"},
		// Tamil
		{tamilConfig, 0x0B95, devaCatConsonant, "tamil ka"},
		{tamilConfig, 0x0BB0, devaCatConsonantRa, "tamil ra"},
		{tamilConfig, 0x0BCD, devaCatVirama, "tamil virama"},
		{tamilConfig, 0x0BC6, devaCatPreBaseMatra, "tamil e-matra"},
		// Telugu
		{teluguConfig, 0x0C15, devaCatConsonant, "telugu ka"},
		{teluguConfig, 0x0C30, devaCatConsonantRa, "telugu ra"},
		{teluguConfig, 0x0C4D, devaCatVirama, "telugu virama"},
		{teluguConfig, 0x0C3F, devaCatPreBaseMatra, "telugu i-matra"},
	}
	for _, tc := range rows {
		if got := tc.cfg.categoryOf(tc.r); got != tc.want {
			t.Errorf("%s (U+%04X): got %d, want %d", tc.name, tc.r, got, tc.want)
		}
	}
}

// --- Pre-base matra reordering per script ----------------------------------

// Each script has a single consonant + pre-base matra input; the
// shaper must move the matra GID in front of the consonant GID in
// the output stream. This is the minimal phase-4 behaviour for
// every script that has a pre-base matra.
func TestShapeIndicPreBaseMatraReorderAllScripts(t *testing.T) {
	cases := []struct {
		name    string
		script  Script
		cfg     *indicScriptConfig
		cons    rune
		matra   rune
		wantGID []uint16
	}{
		{"bengali ki (ka + i-matra)", ScriptBengali, bengaliConfig, 0x0995, 0x09BF,
			[]uint16{gid(0x09BF), gid(0x0995)}},
		{"gujarati ki (ka + i-matra)", ScriptGujarati, gujaratiConfig, 0x0A95, 0x0ABF,
			[]uint16{gid(0x0ABF), gid(0x0A95)}},
		{"gurmukhi sihari (ka + sihari)", ScriptGurmukhi, gurmukhiConfig, 0x0A15, 0x0A3F,
			[]uint16{gid(0x0A3F), gid(0x0A15)}},
		{"kannada ki (ka + i-matra)", ScriptKannada, kannadaConfig, 0x0C95, 0x0CBF,
			[]uint16{gid(0x0CBF), gid(0x0C95)}},
		{"malayalam ke (ka + e-matra)", ScriptMalayalam, malayalamConfig, 0x0D15, 0x0D46,
			[]uint16{gid(0x0D46), gid(0x0D15)}},
		{"oriya ke (ka + e-matra)", ScriptOriya, oriyaConfig, 0x0B15, 0x0B47,
			[]uint16{gid(0x0B47), gid(0x0B15)}},
		{"tamil ke (ka + e-matra)", ScriptTamil, tamilConfig, 0x0B95, 0x0BC6,
			[]uint16{gid(0x0BC6), gid(0x0B95)}},
		{"telugu ki (ka + i-matra)", ScriptTelugu, teluguConfig, 0x0C15, 0x0C3F,
			[]uint16{gid(0x0C3F), gid(0x0C15)}},
	}
	for _, tc := range cases {
		face := &mockIndicFace{}
		input := string([]rune{tc.cons, tc.matra})
		got := ShapeIndic(input, face, nil, tc.cfg)
		if !reflect.DeepEqual(got, tc.wantGID) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.wantGID)
		}
		// Also verify the script dispatcher picks the same config.
		if indicConfigFor(tc.script) != tc.cfg {
			t.Errorf("%s: indicConfigFor(%d) did not return the expected config",
				tc.name, tc.script)
		}
	}
}

// --- Reph formation per script ---------------------------------------------

// Scripts that have reph must detect Ra + virama + consonant at the
// start of a syllable, collapse the Ra glyph through a synthetic
// rphf Single substitution, and place the reph glyph at the
// script-appropriate position in the output.
func TestShapeIndicRephPlacement(t *testing.T) {
	cases := []struct {
		name   string
		cfg    *indicScriptConfig
		ra     rune
		virama rune
		base   rune
		// rphf maps the Ra codepoint's GID to this synthetic reph GID.
		rephGID uint16
		// Expected final stream (phase-4 output).
		want []uint16
	}{
		{
			name: "bengali reph + ka",
			cfg:  bengaliConfig, ra: 0x09B0, virama: 0x09CD, base: 0x0995,
			rephGID: 5000,
			// Bengali uses rephPosAfterMain (reph trails the main
			// cluster and any post-base matras, before trailing
			// SMVD modifiers). With no post-base slots the
			// insertion point collapses to "immediately after base".
			want: []uint16{gid(0x0995), 5000, gid(0x09CD)},
		},
		{
			name: "gujarati reph + ka",
			cfg:  gujaratiConfig, ra: 0x0AB0, virama: 0x0ACD, base: 0x0A95,
			rephGID: 5001,
			want:    []uint16{gid(0x0A95), 5001, gid(0x0ACD)},
		},
		{
			name: "kannada reph + ka",
			cfg:  kannadaConfig, ra: 0x0CB0, virama: 0x0CCD, base: 0x0C95,
			rephGID: 5002,
			// Kannada reph-pos is "after post-base"; with no post-base
			// consonants the insertion point collapses to "after base".
			want: []uint16{gid(0x0C95), 5002, gid(0x0CCD)},
		},
		{
			name: "malayalam reph + ka",
			cfg:  malayalamConfig, ra: 0x0D30, virama: 0x0D4D, base: 0x0D15,
			rephGID: 5003,
			// Malayalam reph-pos is "before post-base"; with no
			// post-base matras it collapses to "immediately after base".
			want: []uint16{gid(0x0D15), 5003, gid(0x0D4D)},
		},
		{
			name: "oriya reph + ka",
			cfg:  oriyaConfig, ra: 0x0B30, virama: 0x0B4D, base: 0x0B15,
			rephGID: 5004,
			want:    []uint16{gid(0x0B15), 5004, gid(0x0B4D)},
		},
		{
			name: "telugu reph + ka",
			cfg:  teluguConfig, ra: 0x0C30, virama: 0x0C4D, base: 0x0C15,
			rephGID: 5005,
			// Telugu is "after post-base"; no post-base here so same
			// as after-base.
			want: []uint16{gid(0x0C15), 5005, gid(0x0C4D)},
		},
	}
	for _, tc := range cases {
		face := &mockIndicFace{
			substitutions: &font.GSUBSubstitutions{
				Single: map[font.GSUBFeature]map[uint16]uint16{
					font.GSUBRphf: {gid(tc.ra): tc.rephGID},
				},
			},
		}
		input := string([]rune{tc.ra, tc.virama, tc.base})
		got := ShapeIndic(input, face, face.substitutions, tc.cfg)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// --- Scripts without reph: Gurmukhi and Tamil ------------------------------

// Gurmukhi has no reph (Punjabi orthography does not use it), so a
// leading Ra + virama + consonant must be treated as a plain
// halant-joined consonant cluster. No glyph moves to after the base.
func TestShapeGurmukhiNoReph(t *testing.T) {
	face := &mockIndicFace{}
	// U+0A30 (ra) + U+0A4D (virama) + U+0A15 (ka).
	input := "\u0A30\u0A4D\u0A15"
	got := ShapeIndic(input, face, nil, gurmukhiConfig)
	// Expected: pass-through in logical order; the Ra is not detected
	// as a reph trigger because gurmukhiConfig.RephPos == rephPosNone.
	want := []uint16{gid(0x0A30), gid(0x0A4D), gid(0x0A15)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("gurmukhi no-reph: got %v, want %v", got, want)
	}
}

// Tamil also has rephPosNone (and HasConjuncts=false in the config,
// though that flag is not yet enforced in the pipeline). A Ra +
// virama + consonant sequence therefore should NOT trigger reph
// movement.
func TestShapeTamilNoReph(t *testing.T) {
	face := &mockIndicFace{}
	// U+0BB0 (ra) + U+0BCD (virama) + U+0B95 (ka).
	input := "\u0BB0\u0BCD\u0B95"
	got := ShapeIndic(input, face, nil, tamilConfig)
	want := []uint16{gid(0x0BB0), gid(0x0BCD), gid(0x0B95)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tamil no-reph: got %v, want %v", got, want)
	}
}

// --- Malayalam chillu letter as base ---------------------------------------

// Malayalam chillu letters (U+0D7A..U+0D7F) are atomic dead-
// consonant forms. When a chillu appears alone in a syllable, the
// shaper should treat it as a consonant base and pass it through
// unchanged.
func TestShapeMalayalamChilluBase(t *testing.T) {
	face := &mockIndicFace{}
	// U+0D7B = chillu-n.
	input := "\u0D7B"
	got := ShapeIndic(input, face, nil, malayalamConfig)
	want := []uint16{gid(0x0D7B)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("malayalam chillu-n: got %v, want %v", got, want)
	}
}

// --- Syllable scanner per script -------------------------------------------

// The scanner must identify a single consonant syllable for each
// script's minimal "consonant + pre-base matra" input.
func TestScanIndicSyllablesAllScripts(t *testing.T) {
	cases := []struct {
		name  string
		cfg   *indicScriptConfig
		input string
		want  []devaSyllable
	}{
		{"bengali ka+i", bengaliConfig, "\u0995\u09BF",
			[]devaSyllable{{0, 2, devaSylConsonant}}},
		{"gujarati ka+i", gujaratiConfig, "\u0A95\u0ABF",
			[]devaSyllable{{0, 2, devaSylConsonant}}},
		{"gurmukhi ka+sihari", gurmukhiConfig, "\u0A15\u0A3F",
			[]devaSyllable{{0, 2, devaSylConsonant}}},
		{"kannada ka+i", kannadaConfig, "\u0C95\u0CBF",
			[]devaSyllable{{0, 2, devaSylConsonant}}},
		{"malayalam ka+e", malayalamConfig, "\u0D15\u0D46",
			[]devaSyllable{{0, 2, devaSylConsonant}}},
		{"oriya ka+e", oriyaConfig, "\u0B15\u0B47",
			[]devaSyllable{{0, 2, devaSylConsonant}}},
		{"tamil ka+e", tamilConfig, "\u0B95\u0BC6",
			[]devaSyllable{{0, 2, devaSylConsonant}}},
		{"telugu ka+i", teluguConfig, "\u0C15\u0C3F",
			[]devaSyllable{{0, 2, devaSylConsonant}}},
		{"bengali independent vowel a", bengaliConfig, "\u0985",
			[]devaSyllable{{0, 1, devaSylVowel}}},
		{"malayalam digit zero", malayalamConfig, "\u0D66",
			[]devaSyllable{{0, 1, devaSylNumber}}},
	}
	for _, tc := range cases {
		got := scanIndicSyllables([]rune(tc.input), tc.cfg)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

// --- Script dispatch via ScriptOf / indicScriptOfWord ----------------------

func TestIndicScriptOfWord(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  Script
	}{
		{"empty", "", ScriptCommon},
		{"bengali", "\u0995", ScriptBengali},
		{"gujarati", "\u0A95", ScriptGujarati},
		{"gurmukhi", "\u0A15", ScriptGurmukhi},
		{"kannada", "\u0C95", ScriptKannada},
		{"malayalam", "\u0D15", ScriptMalayalam},
		{"oriya", "\u0B15", ScriptOriya},
		{"tamil", "\u0B95", ScriptTamil},
		{"telugu", "\u0C15", ScriptTelugu},
		{"latin", "hello", ScriptCommon},
	}
	for _, tc := range cases {
		if got := indicScriptOfWord(tc.input); got != tc.want {
			t.Errorf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

// --- ShapeIndicWithEmbedded dispatch ---------------------------------------

// Verify that the EmbeddedFont wrapper pulls GSUB from the face and
// routes through the shaper for a non-Devanagari script.
func TestShapeIndicWithEmbeddedBengali(t *testing.T) {
	face := &mockIndicFace{}
	ef := font.NewEmbeddedFont(face)
	// Bengali ka + i-matra: expect [matra-gid, ka-gid].
	gids, ok := ShapeIndicWithEmbedded("\u0995\u09BF", ef, ScriptBengali)
	if !ok {
		t.Fatal("expected bengali shaping to succeed")
	}
	want := []uint16{gid(0x09BF), gid(0x0995)}
	if !reflect.DeepEqual(gids, want) {
		t.Errorf("bengali: got %v, want %v", gids, want)
	}
}

// Non-Indic scripts return (nil, false) so the caller falls back to
// the rune path. Covers Latin and two non-Indic non-Latin scripts to
// guard against accidental category leaks (Arabic combining marks,
// Han / CJK ideographs).
func TestShapeIndicWithEmbeddedRejectsNonIndic(t *testing.T) {
	face := &mockIndicFace{}
	ef := font.NewEmbeddedFont(face)
	cases := []struct {
		name   string
		text   string
		script Script
	}{
		{"latin", "hello", ScriptLatin},
		{"arabic alef", "\u0627", ScriptArabic},
		{"han ideograph", "\u4E2D", ScriptHan},
	}
	for _, tc := range cases {
		if gids, ok := ShapeIndicWithEmbedded(tc.text, ef, tc.script); ok {
			t.Errorf("%s: expected rejection, got gids=%v", tc.name, gids)
		}
	}
}

// --- T1: Pre-base matra alt (smell-test fix) -------------------------------

// TestShapeMalayalamPreBaseMatraAlt ensures Malayalam treats the
// non-default pre-base matra U+0D47 as pre-base. Removing it from
// PreBaseMatras should make this test fail.
func TestShapeMalayalamPreBaseMatraAlt(t *testing.T) {
	face := &mockIndicFace{}
	// U+0D15 (ka) + U+0D47 (sign EE).
	got := ShapeIndic("\u0D15\u0D47", face, nil, malayalamConfig)
	want := []uint16{gid(0x0D47), gid(0x0D15)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("malayalam ka+EE: got %v, want %v", got, want)
	}
}

// TestShapeTamilPreBaseMatraAlt covers Tamil's second and third
// pre-base matras (U+0BC7 EE and U+0BC8 AI). All three pre-base
// matras must reorder before the base.
func TestShapeTamilPreBaseMatraAlt(t *testing.T) {
	face := &mockIndicFace{}
	cases := []struct {
		name  string
		input string
		want  []uint16
	}{
		// Note: U+0B95 is Bengali ka, but the test mirrors the
		// PR audit's exact case which uses Tamil ka U+0B95 — Tamil
		// "ka" is actually U+0B95 too. Use Tamil block consonant
		// U+0B95 with Tamil pre-base matras.
		{"tamil ka+EE", "\u0B95\u0BC7", []uint16{gid(0x0BC7), gid(0x0B95)}},
		{"tamil ka+AI", "\u0B95\u0BC8", []uint16{gid(0x0BC8), gid(0x0B95)}},
	}
	for _, tc := range cases {
		got := ShapeIndic(tc.input, face, nil, tamilConfig)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// --- T2: Bengali conjunct cluster ------------------------------------------

// TestShapeBengaliConjunct covers a ka + virama + ba cluster. With
// no GSUB the shaper must scan it as a single consonant syllable
// and pass GIDs through in cluster order.
func TestShapeBengaliConjunct(t *testing.T) {
	face := &mockIndicFace{}
	// U+0995 (ka) + U+09CD (virama) + U+09AC (ba).
	input := "\u0995\u09CD\u09AC"
	syls := scanIndicSyllables([]rune(input), bengaliConfig)
	if len(syls) != 1 || syls[0].Type != devaSylConsonant ||
		syls[0].StartRune != 0 || syls[0].EndRune != 3 {
		t.Errorf("syllable scan: got %+v, want single consonant cluster [0,3)", syls)
	}
	got := ShapeIndic(input, face, nil, bengaliConfig)
	want := []uint16{gid(0x0995), gid(0x09CD), gid(0x09AC)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bengali ka+virama+ba: got %v, want %v", got, want)
	}
}

// --- T3: Bengali ya-phalaa pass-through -------------------------------------

// TestShapeBengaliYaPhalaa locks the contract that ya-phalaa is a
// font-supplied GSUB feature: the layout shaper does not synthesize
// it. With no GSUB tables the input must pass through in cluster
// order.
func TestShapeBengaliYaPhalaa(t *testing.T) {
	face := &mockIndicFace{}
	// U+0995 (ka) + U+09CD (virama) + U+09AF (ya).
	input := "\u0995\u09CD\u09AF"
	got := ShapeIndic(input, face, nil, bengaliConfig)
	want := []uint16{gid(0x0995), gid(0x09CD), gid(0x09AF)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bengali ya-phalaa pass-through: got %v, want %v", got, want)
	}
}

// --- T4: Deferred behaviour regression locks --------------------------------

// TestShapeBengaliSplitMatraNotDecomposed pins the current
// behaviour for Bengali split matra U+09CB (O = U+09C7 + U+09BE).
// We do NOT decompose it; the matra passes through unchanged.
func TestShapeBengaliSplitMatraNotDecomposed(t *testing.T) {
	face := &mockIndicFace{}
	// U+0995 (ka) + U+09CB (sign O, split matra).
	input := "\u0995\u09CB"
	got := ShapeIndic(input, face, nil, bengaliConfig)
	// U+09CB is a (non pre-base) vowel sign in our table, so it
	// stays after the base.
	want := []uint16{gid(0x0995), gid(0x09CB)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("bengali split-matra pass-through: got %v, want %v", got, want)
	}
}

// TestShapeMalayalamSplitMatraNotDecomposed pins Malayalam split
// matra U+0D4A (O = U+0D46 + U+0D3E). Pass-through.
func TestShapeMalayalamSplitMatraNotDecomposed(t *testing.T) {
	face := &mockIndicFace{}
	// U+0D15 (ka) + U+0D4A (sign O, split matra).
	input := "\u0D15\u0D4A"
	got := ShapeIndic(input, face, nil, malayalamConfig)
	want := []uint16{gid(0x0D15), gid(0x0D4A)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("malayalam split-matra pass-through: got %v, want %v", got, want)
	}
}

// TestShapeMalayalamChilluZWJSequence pins the consonant + virama
// + ZWJ chillu-formation sequence. We do not synthesize chillus;
// the ZWJ stays in the stream in logical order.
func TestShapeMalayalamChilluZWJSequence(t *testing.T) {
	face := &mockIndicFace{}
	// U+0D28 (na) + U+0D4D (virama) + U+200D (ZWJ).
	input := "\u0D28\u0D4D\u200D"
	got := ShapeIndic(input, face, nil, malayalamConfig)
	want := []uint16{gid(0x0D28), gid(0x0D4D), gid(0x200D)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("malayalam chillu zwj sequence: got %v, want %v", got, want)
	}
}

// --- T5: Tamil phase-3 short-circuit ---------------------------------------

// TestTamilPhase3FeaturesAreNoOps verifies that even with a synthetic
// blwf single-substitution in the GSUB table, Tamil shaping skips
// phase-3 entirely (HasConjuncts == false). The substitution must
// not fire.
func TestTamilPhase3FeaturesAreNoOps(t *testing.T) {
	face := &mockIndicFace{
		substitutions: &font.GSUBSubstitutions{
			Single: map[font.GSUBFeature]map[uint16]uint16{
				// Map Tamil ra GID to a synthetic post-base form.
				font.GSUBBlwf: {gid(0x0BB0): 7777},
				// Also a half feature that would otherwise fire.
				font.GSUBHalf: {gid(0x0B95): 7778},
			},
		},
	}
	// Tamil ka + virama + ra. With HasConjuncts=false the entire
	// phase-3 is skipped.
	input := "\u0B95\u0BCD\u0BB0"
	got := ShapeIndic(input, face, face.substitutions, tamilConfig)
	want := []uint16{gid(0x0B95), gid(0x0BCD), gid(0x0BB0)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tamil phase-3 short-circuit: got %v, want %v", got, want)
	}
	// And Devanagari, where HasConjuncts=true, must still run blwf.
	face2 := &mockIndicFace{
		substitutions: &font.GSUBSubstitutions{
			Single: map[font.GSUBFeature]map[uint16]uint16{
				font.GSUBBlwf: {gid(0x0930): 8888},
			},
		},
	}
	got2 := ShapeIndic("\u0915\u094D\u0930", face2, face2.substitutions, devanagariConfig)
	found := false
	for _, g := range got2 {
		if g == 8888 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("devanagari blwf must still fire when HasConjuncts=true; got %v", got2)
	}
}

// --- T6: Telugu / Kannada vattu post-base reordering ------------------------

// TestShapeTeluguVattu covers consonant + virama + ra. Without GSUB
// the vattu (ra-kara) sits in logical order; the syllable scanner
// must group all three runes into one consonant cluster, and phase-2
// must label ra as devaPosPostBase so that a real font's vatu
// feature could substitute it.
func TestShapeTeluguVattu(t *testing.T) {
	face := &mockIndicFace{}
	// U+0C15 (ka) + U+0C4D (virama) + U+0C30 (ra).
	input := "\u0C15\u0C4D\u0C30"
	syls := scanIndicSyllables([]rune(input), teluguConfig)
	if len(syls) != 1 || syls[0].Type != devaSylConsonant {
		t.Fatalf("expected single consonant syllable, got %+v", syls)
	}
	got := ShapeIndic(input, face, nil, teluguConfig)
	want := []uint16{gid(0x0C15), gid(0x0C4D), gid(0x0C30)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("telugu vattu pass-through: got %v, want %v", got, want)
	}
}

// TestShapeKannadaVattu mirrors TestShapeTeluguVattu for Kannada.
func TestShapeKannadaVattu(t *testing.T) {
	face := &mockIndicFace{}
	// U+0C95 (ka) + U+0CCD (virama) + U+0CB0 (ra).
	input := "\u0C95\u0CCD\u0CB0"
	syls := scanIndicSyllables([]rune(input), kannadaConfig)
	if len(syls) != 1 || syls[0].Type != devaSylConsonant {
		t.Fatalf("expected single consonant syllable, got %+v", syls)
	}
	got := ShapeIndic(input, face, nil, kannadaConfig)
	want := []uint16{gid(0x0C95), gid(0x0CCD), gid(0x0CB0)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("kannada vattu pass-through: got %v, want %v", got, want)
	}
}

// --- T7: Paragraph-level end-to-end for new scripts ------------------------

// TestBengaliEndToEndViaSplit drives a Bengali word through
// shapeAndMeasureWord and verifies the resulting Word carries a GID
// stream and a non-zero measured width.
func TestBengaliEndToEndViaSplit(t *testing.T) {
	face := &mockIndicFace{}
	ef := font.NewEmbeddedFont(face)
	run := TextRun{Embedded: ef, FontSize: 12}
	w := Word{
		Text:     "\u0995\u09BF", // Bengali ka + i-matra
		Embedded: ef,
		FontSize: 12,
	}
	shapeAndMeasureWord(&w, run, ef)
	if len(w.GIDs) == 0 {
		t.Fatalf("expected Bengali word to carry GIDs")
	}
	want := []uint16{gid(0x09BF), gid(0x0995)}
	if !reflect.DeepEqual(w.GIDs, want) {
		t.Errorf("GID stream: got %v, want %v", w.GIDs, want)
	}
	if w.Width <= 0 {
		t.Errorf("expected non-zero width, got %v", w.Width)
	}
	if w.OriginalText != "\u0995\u09BF" {
		t.Errorf("OriginalText: got %q, want bengali ka+i", w.OriginalText)
	}
}

// TestMixedLatinTeluguBidiSplit drives a mixed Latin / Telugu word
// through splitMixedBidiWord; the Telugu sub-word must receive a
// GID stream while the Latin halves do not.
func TestMixedLatinTeluguBidiSplit(t *testing.T) {
	face := &mockIndicFace{}
	ef := font.NewEmbeddedFont(face)
	run := TextRun{Embedded: ef, FontSize: 12}
	word := Word{
		Text:     "A\u0C15B", // Latin A, Telugu ka, Latin B
		Embedded: ef,
		FontSize: 12,
	}
	subs := splitMixedBidiWord(word)
	if len(subs) != 3 {
		t.Fatalf("expected 3 sub-words, got %d", len(subs))
	}
	for i := range subs {
		shapeAndMeasureWord(&subs[i], run, ef)
	}
	if subs[0].Text != "A" || len(subs[0].GIDs) != 0 {
		t.Errorf("sub 0: want Latin A, no GIDs; got %q GIDs=%v", subs[0].Text, subs[0].GIDs)
	}
	if subs[1].Text != "\u0C15" || len(subs[1].GIDs) == 0 {
		t.Errorf("sub 1: want Telugu ka with GIDs; got %q GIDs=%v", subs[1].Text, subs[1].GIDs)
	}
	if subs[2].Text != "B" || len(subs[2].GIDs) != 0 {
		t.Errorf("sub 2: want Latin B, no GIDs; got %q GIDs=%v", subs[2].Text, subs[2].GIDs)
	}
}

// --- T9: Reph placement actually distinguishes positions -------------------

// TestShapeKannadaRephAfterPostBase exercises rephPosAfterPostBase
// with a real post-base form. The cluster is ra + virama + ka +
// virama + ya: phase-2 places ka as base and ya as post-base.
// Bengali (rephPosAfterMain) and Kannada (rephPosAfterPostBase)
// should both place reph after ya here, but Devanagari (after-base)
// would place it immediately after ka. The test asserts Kannada
// puts the reph at the end of the consonant span.
func TestShapeKannadaRephAfterPostBase(t *testing.T) {
	// Use a synthetic rphf so we can identify the reph in the output.
	face := &mockIndicFace{
		substitutions: &font.GSUBSubstitutions{
			Single: map[font.GSUBFeature]map[uint16]uint16{
				font.GSUBRphf: {gid(0x0CB0): 6000},
			},
		},
	}
	// U+0CB0 (ra) + U+0CCD (virama) + U+0C95 (ka) + U+0CCD + U+0CAF (ya).
	input := "\u0CB0\u0CCD\u0C95\u0CCD\u0CAF"
	got := ShapeIndic(input, face, face.substitutions, kannadaConfig)
	// Logical input slots: [ra, vir, ka, vir, ya]. Phase 2:
	// reph(ra,vir), base=ka, post-base=ya. Phase 4 visual order
	// should be: ka (base), virama, ya (post-base), reph, halant.
	want := []uint16{gid(0x0C95), gid(0x0CCD), gid(0x0CAF), 6000, gid(0x0CCD)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("kannada reph after post-base: got %v, want %v", got, want)
	}
}

// --- T10: Multi-syllable scan for every script -----------------------------

// TestScanIndicSyllablesMultiSyllable verifies the scanner correctly
// segments two consonants joined by an independent vowel for each
// Brahmic script.
func TestScanIndicSyllablesMultiSyllable(t *testing.T) {
	cases := []struct {
		name  string
		cfg   *indicScriptConfig
		input string
		want  int // expected syllable count
	}{
		{"devanagari ka+a+ka", devanagariConfig, "\u0915\u0905\u0915", 3},
		{"bengali ka+a+ka", bengaliConfig, "\u0995\u0985\u0995", 3},
		{"gujarati ka+a+ka", gujaratiConfig, "\u0A95\u0A85\u0A95", 3},
		{"gurmukhi ka+a+ka", gurmukhiConfig, "\u0A15\u0A05\u0A15", 3},
		{"kannada ka+a+ka", kannadaConfig, "\u0C95\u0C85\u0C95", 3},
		{"malayalam ka+a+ka", malayalamConfig, "\u0D15\u0D05\u0D15", 3},
		{"oriya ka+a+ka", oriyaConfig, "\u0B15\u0B05\u0B15", 3},
		{"tamil ka+a+ka", tamilConfig, "\u0B95\u0B85\u0B95", 3},
		{"telugu ka+a+ka", teluguConfig, "\u0C15\u0C05\u0C15", 3},
	}
	for _, tc := range cases {
		got := scanIndicSyllables([]rune(tc.input), tc.cfg)
		if len(got) != tc.want {
			t.Errorf("%s: got %d syllables (%+v), want %d", tc.name, len(got), got, tc.want)
		}
	}
}
