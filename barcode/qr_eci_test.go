// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package barcode

import "testing"

// qrDataBytes reconstructs the data codewords (before error correction) from a
// symbol's de-interleaved blocks, in block order.
func qrDataBytes(version int, level ECCLevel, blocks [][]byte) []byte {
	bi := qrBlockTable[version][level]
	var out []byte
	b := 0
	for ; b < bi.group1Blocks; b++ {
		out = append(out, blocks[b][:bi.group1DataCW]...)
	}
	for k := range bi.group2Blocks {
		out = append(out, blocks[b+k][:bi.group2DataCW]...)
	}
	return out
}

// decodeQRBytePayload reads a byte-mode symbol back to its text and the ECI
// assignment number that preceded the byte segment (-1 if there was none).
func decodeQRBytePayload(t *testing.T, bc *Barcode) (string, int) {
	t.Helper()
	size := bc.width
	version := (size - 17) / 4

	level, mask, ok := readFormat(bc.modules, size)
	if !ok {
		t.Fatalf("format info invalid")
	}
	reserved := qrFunctionMap(version, size)
	stream := extractCodewordStream(bc.modules, reserved, size, mask)
	blocks := deinterleaveBlocks(stream[:qrTotalCodewords[version]], version, level)
	data := qrDataBytes(version, level, blocks)

	bits := make([]bool, 0, len(data)*8)
	for _, b := range data {
		for i := 7; i >= 0; i-- {
			bits = append(bits, (b>>i)&1 == 1)
		}
	}
	pos := 0
	read := func(n int) int {
		v := 0
		for range n {
			v <<= 1
			if bits[pos] {
				v |= 1
			}
			pos++
		}
		return v
	}

	eciNum := -1
	mode := read(4)
	if mode == 7 { // ECI mode indicator
		eciNum = read(8)
		mode = read(4)
	}
	if mode != 4 { // byte mode indicator
		t.Fatalf("expected byte mode, got mode %d", mode)
	}
	count := read(charCountBits(qrModeByte, version))
	out := make([]byte, count)
	for i := range count {
		out[i] = byte(read(8))
	}
	return string(out), eciNum
}

func TestQRByteModeECI(t *testing.T) {
	cases := []struct {
		data    string
		wantECI bool
	}{
		{"https://example.com/folio", false}, // ASCII byte mode, no ECI
		{"Hello, World!", false},
		{"Café", true}, // non-ASCII, declared UTF-8
		{"naïve résumé", true},
		{"日本語のテスト", true},
		{"emoji and 日本語 mixed", true},
	}
	for _, level := range []ECCLevel{ECCLevelL, ECCLevelM, ECCLevelQ, ECCLevelH} {
		for _, c := range cases {
			bc, err := NewQRWithECC(c.data, level)
			if err != nil {
				t.Fatalf("NewQRWithECC(%q, %d): %v", c.data, level, err)
			}
			got, eciNum := decodeQRBytePayload(t, bc)
			if got != c.data {
				t.Errorf("%q L%d: decoded %q", c.data, level, got)
			}
			// Non-ASCII data must carry the UTF-8 ECI assignment number (26);
			// ASCII data must carry none (-1).
			wantNum := -1
			if c.wantECI {
				wantNum = 26
			}
			if eciNum != wantNum {
				t.Errorf("%q L%d: ECI number = %d, want %d", c.data, level, eciNum, wantNum)
			}
		}
	}
}
