// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"testing"
)

func TestParseCMapBfChar(t *testing.T) {
	cmap := `
/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CMapType 2 def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
3 beginbfchar
<0003> <0020>
<0008> <0041>
<0009> <0042>
endbfchar
endcmap
`
	cm := ParseCMap([]byte(cmap))

	if cm.CodeBytes() != 2 {
		t.Errorf("CodeBytes = %d, want 2", cm.CodeBytes())
	}
	if s, ok := cm.lookupCode(0x0003); !ok || s != " " {
		t.Errorf("code 3 = %q, want space", s)
	}
	if s, ok := cm.lookupCode(0x0008); !ok || s != "A" {
		t.Errorf("code 8 = %q, want A", s)
	}
	if s, ok := cm.lookupCode(0x0009); !ok || s != "B" {
		t.Errorf("code 9 = %q, want B", s)
	}
	if _, ok := cm.lookupCode(0x0010); ok {
		t.Error("code 0x10 should not be mapped")
	}
}

func TestParseCMapBfRange(t *testing.T) {
	cmap := `
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfrange
<0041> <0043> <0061>
endbfrange
`
	cm := ParseCMap([]byte(cmap))

	// 0x41 → 'a', 0x42 → 'b', 0x43 → 'c'
	tests := []struct {
		code uint32
		want string
	}{
		{0x0041, "a"},
		{0x0042, "b"},
		{0x0043, "c"},
	}
	for _, tc := range tests {
		s, ok := cm.lookupCode(tc.code)
		if !ok || s != tc.want {
			t.Errorf("code 0x%04X = %q, want %q", tc.code, s, tc.want)
		}
	}
}

func TestParseCMapSingleByte(t *testing.T) {
	cmap := `
1 begincodespacerange
<00> <FF>
endcodespacerange
3 beginbfchar
<01> <0048>
<02> <0069>
<03> <0021>
endbfchar
`
	cm := ParseCMap([]byte(cmap))

	if cm.CodeBytes() != 1 {
		t.Errorf("CodeBytes = %d, want 1", cm.CodeBytes())
	}

	got := cm.Decode([]byte{0x01, 0x02, 0x03})
	if got != "Hi!" {
		t.Errorf("Decode = %q, want %q", got, "Hi!")
	}
}

func TestParseCMapTwoByteDecode(t *testing.T) {
	cmap := `
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
3 beginbfchar
<0003> <0020>
<0025> <0041>
<0045> <0061>
endbfchar
1 beginbfrange
<0046> <0048> <0062>
endbfrange
`
	cm := ParseCMap([]byte(cmap))

	// "\x00\x25\x00\x45\x00\x46\x00\x48" → "Abbd"
	input := []byte{0x00, 0x25, 0x00, 0x45, 0x00, 0x46, 0x00, 0x48}
	got := cm.Decode(input)
	if got != "Aabd" {
		t.Errorf("Decode = %q, want %q", got, "Aabd")
	}
}

func TestParseCMapMultipleSections(t *testing.T) {
	cmap := `
1 begincodespacerange
<00> <FF>
endcodespacerange
2 beginbfchar
<01> <0041>
<02> <0042>
endbfchar
2 beginbfchar
<03> <0043>
<04> <0044>
endbfchar
`
	cm := ParseCMap([]byte(cmap))

	got := cm.Decode([]byte{0x01, 0x02, 0x03, 0x04})
	if got != "ABCD" {
		t.Errorf("Decode = %q, want %q", got, "ABCD")
	}
}

func TestParseCMapEmpty(t *testing.T) {
	cm := ParseCMap([]byte(""))
	got := cm.Decode([]byte("Hello"))
	if got != "Hello" {
		t.Errorf("empty CMap Decode = %q, want %q", got, "Hello")
	}
}

func TestParseCMapInferCodeSpace(t *testing.T) {
	// No codespacerange but has 2-byte codes.
	cmap := `
1 beginbfchar
<0100> <0041>
endbfchar
`
	cm := ParseCMap([]byte(cmap))
	if cm.CodeBytes() != 2 {
		t.Errorf("inferred CodeBytes = %d, want 2", cm.CodeBytes())
	}
}

func TestParseCMapRealChrome(t *testing.T) {
	// Subset of a real Chrome-generated CMap.
	cmap := `
/CIDInit /ProcSet findresource begin
12 dict begin
begincmap
/CIDSystemInfo
<<  /Registry (Adobe)
/Ordering (UCS)
/Supplement 0
>> def
/CMapName /Adobe-Identity-UCS def
/CMapType 2 def
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
3 beginbfchar
<0004> <0020>
<0025> <0041>
<0045> <0061>
endbfchar
2 beginbfrange
<0026> <0027> <0042>
<0046> <0048> <0062>
endbfrange
endcmap
CMapName currentdict /CMap defineresource pop
end
end
`
	cm := ParseCMap([]byte(cmap))

	// Decode " ABC abc"
	input := []byte{
		0x00, 0x04, // space
		0x00, 0x25, // A
		0x00, 0x26, // B
		0x00, 0x27, // C
		0x00, 0x04, // space
		0x00, 0x45, // a
		0x00, 0x46, // b
		0x00, 0x47, // c
		0x00, 0x48, // d
	}
	got := cm.Decode(input)
	if got != " ABC abcd" {
		t.Errorf("Decode = %q, want %q", got, " ABC abcd")
	}
}

func TestDecodeUnicodeHexSurrogatePair(t *testing.T) {
	// U+1F600 (😀) = D83D DE00 in UTF-16
	got := decodeUnicodeHex("D83DDE00")
	if got != "😀" {
		t.Errorf("surrogate pair = %q, want 😀", got)
	}
}

func TestDecodeUnicodeHexBMP(t *testing.T) {
	got := decodeUnicodeHex("0041")
	if got != "A" {
		t.Errorf("BMP = %q, want A", got)
	}
}

func TestDecodeHexCode(t *testing.T) {
	tests := []struct {
		hex  string
		code uint32
		n    int
	}{
		{"00", 0, 1},
		{"FF", 255, 1},
		{"0041", 65, 2},
		{"FFFF", 65535, 2},
	}
	for _, tc := range tests {
		code, n := decodeHexCode(tc.hex)
		if code != tc.code || n != tc.n {
			t.Errorf("decodeHexCode(%q) = (%d, %d), want (%d, %d)", tc.hex, code, n, tc.code, tc.n)
		}
	}
}

func TestCMapNilDecode(t *testing.T) {
	var cm *CMap
	got := cm.Decode([]byte("test"))
	if got != "test" {
		t.Errorf("nil CMap Decode = %q, want %q", got, "test")
	}
}

func TestParseCMapBfRangeArrayForm(t *testing.T) {
	cmap := `
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfrange
<0041> <0043> [<0061> <0062> <0063>]
endbfrange
`
	cm := ParseCMap([]byte(cmap))

	tests := []struct {
		code uint32
		want string
	}{
		{0x0041, "a"},
		{0x0042, "b"},
		{0x0043, "c"},
	}
	for _, tc := range tests {
		s, ok := cm.lookupCode(tc.code)
		if !ok || s != tc.want {
			t.Errorf("code 0x%04X = %q, want %q", tc.code, s, tc.want)
		}
	}
}

func TestParseCMapBfRangeArrayFormMisalignment(t *testing.T) {
	// Regression: an array-form entry used to be flattened by
	// extractHexTokens, misaligning every entry that followed it in the
	// block.
	cmap := `
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
2 beginbfrange
<0041> <0043> [<0061> <0062> <0063>]
<0050> <0051> <0070>
endbfrange
`
	cm := ParseCMap([]byte(cmap))

	tests := []struct {
		code uint32
		want string
	}{
		{0x0041, "a"},
		{0x0042, "b"},
		{0x0043, "c"},
		{0x0050, "p"},
		{0x0051, "q"},
	}
	for _, tc := range tests {
		s, ok := cm.lookupCode(tc.code)
		if !ok || s != tc.want {
			t.Errorf("code 0x%04X = %q, want %q", tc.code, s, tc.want)
		}
	}
}

func TestParseCMapBfRangeArrayFormLigature(t *testing.T) {
	// Multi-rune destination inside an array (fi ligature).
	cmap := `
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfrange
<FB01> <FB01> [<00660069>]
endbfrange
`
	cm := ParseCMap([]byte(cmap))
	s, ok := cm.lookupCode(0xFB01)
	if !ok || s != "fi" {
		t.Errorf("code 0xFB01 = %q, want %q", s, "fi")
	}
}

func TestParseCMapBfRangeArrayFormSurrogatePair(t *testing.T) {
	// Surrogate-pair destination inside an array: D835 DC00 → U+1D400
	// (MATHEMATICAL BOLD CAPITAL A).
	cmap := `
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfrange
<0001> <0001> [<D835DC00>]
endbfrange
`
	cm := ParseCMap([]byte(cmap))
	s, ok := cm.lookupCode(0x0001)
	if !ok || s != "\U0001D400" {
		t.Errorf("code 0x0001 = %q (%U), want U+1D400", s, []rune(s))
	}
}

func TestParseCMapBfRangeArrayShorterThanRange(t *testing.T) {
	cmap := `
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
1 beginbfrange
<0041> <0045> [<0061> <0062>]
endbfrange
`
	cm := ParseCMap([]byte(cmap))

	if s, ok := cm.lookupCode(0x0041); !ok || s != "a" {
		t.Errorf("code 0x41 = %q, want %q", s, "a")
	}
	if s, ok := cm.lookupCode(0x0042); !ok || s != "b" {
		t.Errorf("code 0x42 = %q, want %q", s, "b")
	}
	// Codes past the array's length are unmapped; Decode falls back to
	// the raw rune.
	for _, code := range []uint32{0x0043, 0x0044, 0x0045} {
		if _, ok := cm.lookupCode(code); ok {
			t.Errorf("code 0x%04X should be unmapped", code)
		}
	}
	got := cm.Decode([]byte{0x00, 0x43, 0x00, 0x44, 0x00, 0x45})
	want := "CDE"
	if got != want {
		t.Errorf("Decode = %q, want %q", got, want)
	}
}

func TestParseCMapBfRangeArrayLongerThanRange(t *testing.T) {
	cmap := `
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
2 beginbfrange
<0041> <0041> [<0061> <0062>]
<0050> <0051> <0070>
endbfrange
`
	cm := ParseCMap([]byte(cmap))

	if s, ok := cm.lookupCode(0x0041); !ok || s != "a" {
		t.Errorf("code 0x41 = %q, want %q", s, "a")
	}
	if _, ok := cm.lookupCode(0x0042); ok {
		t.Error("code 0x42 should be unmapped; extra array destination must not leak")
	}
	// The extra destination must not shift the following entry.
	if s, ok := cm.lookupCode(0x0050); !ok || s != "p" {
		t.Errorf("code 0x50 = %q, want %q", s, "p")
	}
	if s, ok := cm.lookupCode(0x0051); !ok || s != "q" {
		t.Errorf("code 0x51 = %q, want %q", s, "q")
	}
}

func TestParseCMapBfRangeArrayUnterminated(t *testing.T) {
	// Malformed: unterminated array at end of block. Must not panic, and
	// entries before it are unaffected.
	cmap := `
1 begincodespacerange
<0000> <FFFF>
endcodespacerange
2 beginbfrange
<0050> <0051> <0070>
<0041> <0042> [<0061>
endbfrange
`
	cm := ParseCMap([]byte(cmap))

	if s, ok := cm.lookupCode(0x0050); !ok || s != "p" {
		t.Errorf("code 0x50 = %q, want %q", s, "p")
	}
	if s, ok := cm.lookupCode(0x0051); !ok || s != "q" {
		t.Errorf("code 0x51 = %q, want %q", s, "q")
	}
}

func TestExtractHexTokens(t *testing.T) {
	tokens := extractHexTokens("<0041> <0042>")
	if len(tokens) != 2 || tokens[0] != "0041" || tokens[1] != "0042" {
		t.Errorf("tokens = %v", tokens)
	}
}

func TestCMapBfCharLigature(t *testing.T) {
	// U+FB01 is the fi ligature — a single Unicode character.
	cmap := `
1 begincodespacerange
<00> <FF>
endcodespacerange
1 beginbfchar
<0C> <FB01>
endbfchar
`
	cm := ParseCMap([]byte(cmap))
	got := cm.Decode([]byte{0x0C})
	if got != "\uFB01" {
		t.Errorf("ligature = %q (%U), want U+FB01", got, []rune(got))
	}
}
