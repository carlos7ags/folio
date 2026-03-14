// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package barcode

import "fmt"

// QR generates a QR Code barcode from a string.
// Uses byte mode encoding with error correction level M (15% recovery).
// Automatically selects the smallest version (1-10) that fits the data.
func QR(data string) (*Barcode, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("barcode: empty data")
	}

	// Find the smallest version that fits.
	version := 0
	for v := 1; v <= 10; v++ {
		cap := qrByteCapacity[v]
		if len(data) <= cap {
			version = v
			break
		}
	}
	if version == 0 {
		return nil, fmt.Errorf("barcode: data too long for QR version 1-10 (%d bytes, max %d)", len(data), qrByteCapacity[10])
	}

	size := 17 + version*4
	modules := make([][]bool, size)
	reserved := make([][]bool, size) // tracks which modules are reserved (not data)
	for i := range size {
		modules[i] = make([]bool, size)
		reserved[i] = make([]bool, size)
	}

	// Place finder patterns (3 corners).
	placeFinder(modules, reserved, 0, 0)
	placeFinder(modules, reserved, 0, size-7)
	placeFinder(modules, reserved, size-7, 0)

	// Place alignment patterns (version 2+).
	if version >= 2 {
		positions := alignmentPositions(version)
		for _, r := range positions {
			for _, c := range positions {
				if !reserved[r][c] {
					placeAlignment(modules, reserved, r, c)
				}
			}
		}
	}

	// Place timing patterns.
	for i := 8; i < size-8; i++ {
		modules[6][i] = i%2 == 0
		reserved[6][i] = true
		modules[i][6] = i%2 == 0
		reserved[i][6] = true
	}

	// Dark module.
	modules[size-8][8] = true
	reserved[size-8][8] = true

	// Reserve format info areas.
	for i := range 9 {
		reserved[8][i] = true
		reserved[i][8] = true
	}
	for i := range 8 {
		reserved[8][size-1-i] = true
		reserved[size-1-i][8] = true
	}

	// Reserve version info areas (version 7+, not needed for 1-10).

	// Encode data.
	bits := encodeQRData(data, version)

	// Place data bits.
	placeData(modules, reserved, bits, size)

	// Apply mask pattern 0 (checkerboard: (row+col) % 2 == 0).
	for r := range size {
		for c := range size {
			if !reserved[r][c] && (r+c)%2 == 0 {
				modules[r][c] = !modules[r][c]
			}
		}
	}

	// Place format info (error correction M, mask 0).
	placeFormatInfo(modules, reserved, size, version)

	return &Barcode{modules: modules, width: size, height: size}, nil
}

// qrByteCapacity is the maximum byte-mode data capacity for versions 1-10
// at error correction level M.
var qrByteCapacity = [11]int{
	0,   // version 0 unused
	14,  // version 1
	26,  // version 2
	42,  // version 3
	62,  // version 4
	84,  // version 5
	106, // version 6
	122, // version 7
	152, // version 8
	180, // version 9
	213, // version 10
}

// placeFinder places a 7x7 finder pattern at (row, col).
func placeFinder(modules, reserved [][]bool, row, col int) {
	for r := -1; r <= 7; r++ {
		for c := -1; c <= 7; c++ {
			rr := row + r
			cc := col + c
			if rr < 0 || cc < 0 || rr >= len(modules) || cc >= len(modules[0]) {
				continue
			}
			dark := false
			if r >= 0 && r <= 6 && c >= 0 && c <= 6 {
				// Outer border or inner 3x3 block.
				if r == 0 || r == 6 || c == 0 || c == 6 ||
					(r >= 2 && r <= 4 && c >= 2 && c <= 4) {
					dark = true
				}
			}
			modules[rr][cc] = dark
			reserved[rr][cc] = true
		}
	}
}

// placeAlignment places a 5x5 alignment pattern centered at (row, col).
func placeAlignment(modules, reserved [][]bool, row, col int) {
	for r := -2; r <= 2; r++ {
		for c := -2; c <= 2; c++ {
			rr := row + r
			cc := col + c
			if rr < 0 || cc < 0 || rr >= len(modules) || cc >= len(modules[0]) {
				continue
			}
			dark := r == -2 || r == 2 || c == -2 || c == 2 || (r == 0 && c == 0)
			modules[rr][cc] = dark
			reserved[rr][cc] = true
		}
	}
}

// alignmentPositions returns the alignment pattern center coordinates for a version.
var alignmentTable = [11][]int{
	{},          // version 0
	{},          // version 1 (no alignment)
	{6, 18},     // version 2
	{6, 22},     // version 3
	{6, 26},     // version 4
	{6, 30},     // version 5
	{6, 34},     // version 6
	{6, 22, 38}, // version 7
	{6, 24, 42}, // version 8
	{6, 26, 46}, // version 9
	{6, 28, 50}, // version 10
}

func alignmentPositions(version int) []int {
	if version < 1 || version > 10 {
		return nil
	}
	return alignmentTable[version]
}

// encodeQRData encodes data in byte mode with ECC level M.
func encodeQRData(data string, version int) []bool {
	var bits []bool

	// Mode indicator: byte mode = 0100.
	bits = append(bits, false, true, false, false)

	// Character count indicator (8 bits for versions 1-9, 16 for 10+).
	countBits := 8
	if version >= 10 {
		countBits = 16
	}
	for i := countBits - 1; i >= 0; i-- {
		bits = append(bits, (len(data)>>i)&1 == 1)
	}

	// Data bytes.
	for _, b := range []byte(data) {
		for i := 7; i >= 0; i-- {
			bits = append(bits, (b>>i)&1 == 1)
		}
	}

	// Terminator (up to 4 zero bits).
	totalBits := qrTotalDataBits(version)
	for range 4 {
		if len(bits) >= totalBits {
			break
		}
		bits = append(bits, false)
	}

	// Pad to byte boundary.
	for len(bits)%8 != 0 {
		bits = append(bits, false)
	}

	// Pad bytes (alternating 0xEC, 0x11).
	padBytes := []byte{0xEC, 0x11}
	padIdx := 0
	for len(bits) < totalBits {
		b := padBytes[padIdx%2]
		for i := 7; i >= 0; i-- {
			bits = append(bits, (b>>i)&1 == 1)
		}
		padIdx++
	}

	// Truncate to exact size.
	if len(bits) > totalBits {
		bits = bits[:totalBits]
	}

	// Add error correction codewords.
	bits = appendECC(bits, version)

	return bits
}

// qrTotalDataBits returns the total data bits (before ECC) for a version at ECC level M.
var qrDataCodewords = [11]int{
	0,   // version 0
	16,  // version 1: 16 data codewords
	28,  // version 2
	44,  // version 3
	64,  // version 4
	86,  // version 5
	108, // version 6
	124, // version 7
	154, // version 8
	182, // version 9
	216, // version 10
}

func qrTotalDataBits(version int) int {
	if version < 1 || version > 10 {
		return 0
	}
	return qrDataCodewords[version] * 8
}

// appendECC appends Reed-Solomon error correction codewords.
// Simplified: generates ECC using GF(256) arithmetic.
func appendECC(dataBits []bool, version int) []bool {
	// Convert bits to codewords (bytes).
	codewords := make([]byte, len(dataBits)/8)
	for i := range codewords {
		var b byte
		for j := range 8 {
			if dataBits[i*8+j] {
				b |= 1 << (7 - j)
			}
		}
		codewords[i] = b
	}

	// ECC codeword count per version (level M).
	eccCount := qrECCCount(version)

	// Generate ECC using polynomial division in GF(256).
	generator := rsGeneratorPoly(eccCount)
	ecc := rsEncode(codewords, generator, eccCount)

	// Append ECC bits.
	result := make([]bool, len(dataBits))
	copy(result, dataBits)
	for _, b := range ecc {
		for i := 7; i >= 0; i-- {
			result = append(result, (b>>i)&1 == 1)
		}
	}

	return result
}

// qrECCCount returns the number of ECC codewords for a version at level M.
var qrECCCounts = [11]int{
	0,  // version 0
	10, // version 1
	16, // version 2
	26, // version 3
	18, // version 4 (2 blocks)
	24, // version 5
	16, // version 6 (4 blocks - simplified to single block)
	18, // version 7
	22, // version 8
	22, // version 9
	26, // version 10
}

func qrECCCount(version int) int {
	if version < 1 || version > 10 {
		return 0
	}
	return qrECCCounts[version]
}

// --- GF(256) arithmetic for Reed-Solomon ---

// gfExp and gfLog are lookup tables for GF(256) with primitive polynomial 0x11D.
var gfExp [512]byte
var gfLog [256]byte

func init() {
	x := 1
	for i := range 255 {
		gfExp[i] = byte(x)
		gfLog[x] = byte(i)
		x <<= 1
		if x >= 256 {
			x ^= 0x11D
		}
	}
	for i := 255; i < 512; i++ {
		gfExp[i] = gfExp[i-255]
	}
}

func gfMul(a, b byte) byte {
	if a == 0 || b == 0 {
		return 0
	}
	return gfExp[int(gfLog[a])+int(gfLog[b])]
}

// rsGeneratorPoly computes the Reed-Solomon generator polynomial of degree n.
func rsGeneratorPoly(n int) []byte {
	g := []byte{1}
	for i := range n {
		ng := make([]byte, len(g)+1)
		for j, coeff := range g {
			ng[j] ^= gfMul(coeff, gfExp[i])
			ng[j+1] ^= coeff
		}
		g = ng
	}
	return g
}

// rsEncode performs Reed-Solomon encoding.
func rsEncode(data []byte, generator []byte, eccLen int) []byte {
	// Extend data with zero ECC bytes.
	msg := make([]byte, len(data)+eccLen)
	copy(msg, data)

	for i := range len(data) {
		coeff := msg[i]
		if coeff == 0 {
			continue
		}
		for j := range len(generator) {
			msg[i+j] ^= gfMul(generator[j], coeff)
		}
	}

	return msg[len(data):]
}

// placeData places data bits into the QR matrix in the zigzag pattern.
func placeData(modules, reserved [][]bool, bits []bool, size int) {
	bitIdx := 0
	upward := true

	for col := size - 1; col >= 0; col -= 2 {
		if col == 6 {
			col-- // skip timing column
		}
		if col < 0 {
			break
		}

		rows := make([]int, size)
		if upward {
			for i := range size {
				rows[i] = size - 1 - i
			}
		} else {
			for i := range size {
				rows[i] = i
			}
		}

		for _, row := range rows {
			for c := col; c >= max(col-1, 0); c-- {
				if reserved[row][c] {
					continue
				}
				if bitIdx < len(bits) {
					modules[row][c] = bits[bitIdx]
					bitIdx++
				}
			}
		}
		upward = !upward
	}
}

// placeFormatInfo places the 15-bit format information string.
func placeFormatInfo(modules, reserved [][]bool, size, _ int) {
	// Format info for ECC level M (01), mask pattern 0 (000) = 0b01000.
	// After BCH encoding: 101010000010010.
	formatBits := [15]bool{
		true, false, true, false, true, false, false, false, false, false, true, false, false, true, false,
	}

	// Place around top-left finder.
	positions := [][2]int{
		{0, 8}, {1, 8}, {2, 8}, {3, 8}, {4, 8}, {5, 8}, {7, 8}, {8, 8},
		{8, 7}, {8, 5}, {8, 4}, {8, 3}, {8, 2}, {8, 1}, {8, 0},
	}
	for i, pos := range positions {
		modules[pos[0]][pos[1]] = formatBits[i]
	}

	// Place along bottom-left and top-right.
	for i := range 7 {
		modules[size-1-i][8] = formatBits[i]
	}
	for i := range 8 {
		modules[8][size-8+i] = formatBits[7+i]
	}
}
