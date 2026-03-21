// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package barcode

import "fmt"

// ECCLevel represents the error correction level for QR codes.
type ECCLevel int

const (
	ECCLevelL ECCLevel = iota // 7% recovery
	ECCLevelM                 // 15% recovery
	ECCLevelQ                 // 25% recovery
	ECCLevelH                 // 30% recovery
)

// qrMode represents the encoding mode for QR code data.
type qrMode int

const (
	qrModeNumeric      qrMode = 1 // 0001
	qrModeAlphanumeric qrMode = 2 // 0010
	qrModeByte         qrMode = 4 // 0100
)

// alphanumericValues maps characters to their alphanumeric mode values.
var alphanumericValues = map[byte]int{
	'0': 0, '1': 1, '2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7, '8': 8, '9': 9,
	'A': 10, 'B': 11, 'C': 12, 'D': 13, 'E': 14, 'F': 15, 'G': 16, 'H': 17,
	'I': 18, 'J': 19, 'K': 20, 'L': 21, 'M': 22, 'N': 23, 'O': 24, 'P': 25,
	'Q': 26, 'R': 27, 'S': 28, 'T': 29, 'U': 30, 'V': 31, 'W': 32, 'X': 33,
	'Y': 34, 'Z': 35, ' ': 36, '$': 37, '%': 38, '*': 39, '+': 40, '-': 41,
	'.': 42, '/': 43, ':': 44,
}

// isNumeric returns true if data contains only digits 0-9.
func isNumeric(data string) bool {
	for i := range len(data) {
		if data[i] < '0' || data[i] > '9' {
			return false
		}
	}
	return true
}

// isAlphanumeric returns true if data contains only alphanumeric mode characters.
func isAlphanumeric(data string) bool {
	for i := range len(data) {
		if _, ok := alphanumericValues[data[i]]; !ok {
			return false
		}
	}
	return true
}

// detectMode returns the most efficient encoding mode for the data.
func detectMode(data string) qrMode {
	if isNumeric(data) {
		return qrModeNumeric
	}
	if isAlphanumeric(data) {
		return qrModeAlphanumeric
	}
	return qrModeByte
}

// charCountBits returns the number of character count indicator bits for the given mode and version.
func charCountBits(mode qrMode, version int) int {
	switch mode {
	case qrModeNumeric:
		if version <= 9 {
			return 10
		}
		if version <= 26 {
			return 12
		}
		return 14
	case qrModeAlphanumeric:
		if version <= 9 {
			return 9
		}
		if version <= 26 {
			return 11
		}
		return 13
	default: // byte mode
		if version <= 9 {
			return 8
		}
		return 16
	}
}

// dataCapacity returns the number of characters that fit in the given version/level/mode.
func dataCapacity(version int, level ECCLevel, mode qrMode) int {
	dataCW := qrDataCodewords[version][level]
	countBits := charCountBits(mode, version)
	totalDataBits := dataCW*8 - 4 - countBits // subtract mode indicator and count bits

	switch mode {
	case qrModeNumeric:
		// 3 digits → 10 bits, 2 digits → 7 bits, 1 digit → 4 bits
		fullGroups := totalDataBits / 10
		remaining := totalDataBits % 10
		chars := fullGroups * 3
		if remaining >= 7 {
			chars += 2
		} else if remaining >= 4 {
			chars += 1
		}
		return chars
	case qrModeAlphanumeric:
		// 2 chars → 11 bits, 1 char → 6 bits
		fullPairs := totalDataBits / 11
		remaining := totalDataBits % 11
		chars := fullPairs * 2
		if remaining >= 6 {
			chars += 1
		}
		return chars
	default: // byte mode
		return totalDataBits / 8
	}
}

// QR generates a QR Code barcode from a string.
// Uses the most efficient encoding mode with error correction level M (15% recovery).
// Automatically selects the smallest version (1-40) that fits the data.
func QR(data string) (*Barcode, error) {
	return QRWithECC(data, ECCLevelM)
}

// QRWithECC generates a QR Code barcode with the specified error correction level.
// Automatically selects the most efficient encoding mode and smallest version (1-40).
func QRWithECC(data string, level ECCLevel) (*Barcode, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("barcode: empty data")
	}
	if level < ECCLevelL || level > ECCLevelH {
		return nil, fmt.Errorf("barcode: invalid ECC level %d", level)
	}

	mode := detectMode(data)

	// Find the smallest version that fits.
	version := 0
	for v := 1; v <= 40; v++ {
		cap := dataCapacity(v, level, mode)
		if len(data) <= cap {
			version = v
			break
		}
	}
	if version == 0 {
		return nil, fmt.Errorf("barcode: data too long for QR version 1-40 at ECC level %d", level)
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

	// Reserve version info areas (version 7+).
	if version >= 7 {
		for i := range 6 {
			for j := range 3 {
				reserved[size-11+j][i] = true // bottom-left
				reserved[i][size-11+j] = true // top-right
			}
		}
	}

	// Encode data.
	bits := encodeQRData(data, version, level, mode)

	// Place data bits.
	placeData(modules, reserved, bits, size)

	// Evaluate all 8 mask patterns and pick the best one.
	bestMask, bestModules := evaluateMasks(modules, reserved, size)

	// Place format info.
	placeFormatInfo(bestModules, size, level, bestMask)

	// Place version info (version 7+).
	if version >= 7 {
		placeVersionInfo(bestModules, size, version)
	}

	return &Barcode{modules: bestModules, width: size, height: size}, nil
}

// qrByteCapacity holds the maximum number of data bytes for each QR version (1-40)
// and ECC level (L/M/Q/H). Index 0 is unused.
var qrByteCapacity = [41][4]int{
	{0, 0, 0, 0},             // version 0 unused
	{17, 14, 11, 7},          // version 1
	{32, 26, 20, 14},         // version 2
	{53, 42, 32, 24},         // version 3
	{78, 62, 46, 34},         // version 4
	{106, 84, 60, 44},        // version 5
	{134, 106, 74, 58},       // version 6
	{154, 122, 86, 64},       // version 7
	{192, 152, 108, 84},      // version 8
	{230, 180, 130, 98},      // version 9
	{271, 213, 151, 119},     // version 10
	{321, 251, 177, 137},     // version 11
	{367, 287, 203, 155},     // version 12
	{425, 331, 241, 177},     // version 13
	{458, 362, 258, 194},     // version 14
	{520, 412, 292, 220},     // version 15
	{586, 450, 322, 250},     // version 16
	{644, 504, 364, 280},     // version 17
	{718, 560, 394, 310},     // version 18
	{792, 624, 442, 338},     // version 19
	{858, 666, 482, 382},     // version 20
	{929, 711, 509, 403},     // version 21
	{1003, 779, 565, 439},    // version 22
	{1091, 857, 611, 461},    // version 23
	{1171, 911, 661, 511},    // version 24
	{1273, 997, 715, 535},    // version 25
	{1367, 1059, 751, 593},   // version 26
	{1465, 1125, 805, 625},   // version 27
	{1528, 1190, 868, 658},   // version 28
	{1628, 1264, 908, 698},   // version 29
	{1732, 1370, 982, 742},   // version 30
	{1840, 1452, 1030, 790},  // version 31
	{1952, 1538, 1112, 842},  // version 32
	{2068, 1628, 1168, 898},  // version 33
	{2188, 1722, 1228, 958},  // version 34
	{2303, 1809, 1283, 983},  // version 35
	{2431, 1911, 1351, 1051}, // version 36
	{2563, 1989, 1423, 1093}, // version 37
	{2699, 2099, 1499, 1139}, // version 38
	{2809, 2213, 1579, 1219}, // version 39
	{2953, 2331, 1663, 1273}, // version 40
}

// qrDataCodewords holds the number of data codewords for each QR version (1-40)
// and ECC level (L/M/Q/H). Index 0 is unused.
var qrDataCodewords = [41][4]int{
	{0, 0, 0, 0},             // version 0 unused
	{19, 16, 13, 9},          // version 1
	{34, 28, 22, 16},         // version 2
	{55, 44, 34, 26},         // version 3
	{80, 64, 48, 36},         // version 4
	{108, 86, 62, 46},        // version 5
	{136, 108, 76, 60},       // version 6
	{156, 124, 88, 66},       // version 7
	{194, 154, 110, 86},      // version 8
	{232, 182, 132, 100},     // version 9
	{274, 216, 154, 122},     // version 10
	{324, 254, 180, 140},     // version 11
	{370, 290, 206, 158},     // version 12
	{428, 334, 244, 180},     // version 13
	{461, 365, 261, 197},     // version 14
	{523, 415, 295, 223},     // version 15
	{589, 453, 325, 253},     // version 16
	{647, 507, 367, 283},     // version 17
	{721, 563, 397, 313},     // version 18
	{795, 627, 445, 341},     // version 19
	{861, 669, 485, 385},     // version 20
	{932, 714, 512, 406},     // version 21
	{1006, 782, 568, 442},    // version 22
	{1094, 860, 614, 464},    // version 23
	{1174, 914, 664, 514},    // version 24
	{1276, 1000, 718, 538},   // version 25
	{1370, 1062, 754, 596},   // version 26
	{1468, 1128, 808, 628},   // version 27
	{1531, 1193, 871, 661},   // version 28
	{1631, 1267, 911, 701},   // version 29
	{1735, 1373, 985, 745},   // version 30
	{1843, 1455, 1033, 793},  // version 31
	{1955, 1541, 1115, 845},  // version 32
	{2071, 1631, 1171, 901},  // version 33
	{2191, 1725, 1231, 961},  // version 34
	{2306, 1812, 1286, 986},  // version 35
	{2434, 1914, 1354, 1054}, // version 36
	{2566, 1992, 1426, 1096}, // version 37
	{2702, 2102, 1502, 1142}, // version 38
	{2812, 2216, 1582, 1222}, // version 39
	{2956, 2334, 1666, 1276}, // version 40
}

// qrECCPerBlock holds the number of error correction codewords per block for each
// QR version (1-40) and ECC level (L/M/Q/H). Index 0 is unused.
var qrECCPerBlock = [41][4]int{
	{0, 0, 0, 0},     // version 0 unused
	{7, 10, 13, 17},  // version 1
	{10, 16, 22, 28}, // version 2
	{15, 26, 18, 22}, // version 3
	{20, 18, 26, 16}, // version 4
	{26, 24, 18, 22}, // version 5
	{18, 16, 24, 28}, // version 6
	{20, 18, 18, 26}, // version 7
	{24, 22, 22, 26}, // version 8
	{30, 22, 20, 24}, // version 9
	{18, 26, 24, 28}, // version 10
	{20, 30, 28, 24}, // version 11
	{24, 22, 26, 28}, // version 12
	{26, 22, 24, 22}, // version 13
	{30, 24, 20, 24}, // version 14
	{22, 24, 30, 24}, // version 15
	{24, 28, 24, 30}, // version 16
	{28, 28, 28, 28}, // version 17
	{30, 26, 28, 28}, // version 18
	{28, 26, 26, 26}, // version 19
	{28, 26, 30, 28}, // version 20
	{28, 26, 28, 28}, // version 21
	{28, 28, 30, 28}, // version 22
	{30, 28, 30, 28}, // version 23
	{30, 28, 30, 28}, // version 24
	{26, 28, 30, 28}, // version 25
	{28, 28, 28, 28}, // version 26
	{30, 28, 30, 28}, // version 27
	{30, 28, 30, 28}, // version 28
	{30, 28, 30, 28}, // version 29
	{30, 28, 30, 28}, // version 30
	{30, 28, 30, 28}, // version 31
	{30, 28, 30, 28}, // version 32
	{30, 28, 30, 28}, // version 33
	{30, 28, 30, 28}, // version 34
	{30, 28, 30, 28}, // version 35
	{30, 28, 30, 28}, // version 36
	{30, 28, 30, 28}, // version 37
	{30, 28, 30, 28}, // version 38
	{30, 28, 30, 28}, // version 39
	{30, 28, 30, 28}, // version 40
}

// qrBlockInfo describes the block structure for a version/ECC combination.
type qrBlockInfo struct {
	group1Blocks int
	group1DataCW int // data codewords per block in group 1
	group2Blocks int
	group2DataCW int // data codewords per block in group 2 (= group1DataCW + 1, or 0)
}

// qrBlockTable holds the block structure for each QR version (1-40)
// and ECC level (L/M/Q/H). Index 0 is unused.
var qrBlockTable = [41][4]qrBlockInfo{
	{}, // version 0 unused
	// version 1
	{{1, 19, 0, 0}, {1, 16, 0, 0}, {1, 13, 0, 0}, {1, 9, 0, 0}},
	// version 2
	{{1, 34, 0, 0}, {1, 28, 0, 0}, {1, 22, 0, 0}, {1, 16, 0, 0}},
	// version 3
	{{1, 55, 0, 0}, {1, 44, 0, 0}, {2, 17, 0, 0}, {2, 13, 0, 0}},
	// version 4
	{{1, 80, 0, 0}, {2, 32, 0, 0}, {2, 24, 0, 0}, {4, 9, 0, 0}},
	// version 5
	{{1, 108, 0, 0}, {2, 43, 0, 0}, {2, 15, 2, 16}, {2, 11, 2, 12}},
	// version 6
	{{2, 68, 0, 0}, {4, 27, 0, 0}, {4, 19, 0, 0}, {4, 15, 0, 0}},
	// version 7
	{{2, 78, 0, 0}, {4, 31, 0, 0}, {2, 14, 4, 15}, {4, 13, 1, 14}},
	// version 8
	{{2, 97, 0, 0}, {2, 38, 2, 39}, {4, 18, 2, 19}, {4, 14, 2, 15}},
	// version 9
	{{2, 116, 0, 0}, {3, 36, 2, 37}, {4, 16, 4, 17}, {4, 12, 4, 13}},
	// version 10
	{{2, 68, 2, 69}, {4, 43, 1, 44}, {6, 19, 2, 20}, {6, 15, 2, 16}},
	// version 11
	{{4, 81, 0, 0}, {1, 50, 4, 51}, {4, 22, 4, 23}, {3, 12, 8, 13}},
	// version 12
	{{2, 92, 2, 93}, {6, 36, 2, 37}, {4, 20, 6, 21}, {7, 14, 4, 15}},
	// version 13
	{{4, 107, 0, 0}, {8, 37, 1, 38}, {8, 20, 4, 21}, {12, 11, 4, 12}},
	// version 14
	{{3, 115, 1, 116}, {4, 40, 5, 41}, {11, 16, 5, 17}, {11, 12, 5, 13}},
	// version 15
	{{5, 87, 1, 88}, {5, 41, 5, 42}, {5, 24, 7, 25}, {11, 12, 7, 13}},
	// version 16
	{{5, 98, 1, 99}, {7, 45, 3, 46}, {15, 19, 2, 20}, {3, 15, 13, 16}},
	// version 17
	{{1, 107, 5, 108}, {10, 46, 1, 47}, {1, 22, 15, 23}, {2, 14, 17, 15}},
	// version 18
	{{5, 120, 1, 121}, {9, 43, 4, 44}, {17, 22, 1, 23}, {2, 14, 19, 15}},
	// version 19
	{{3, 113, 4, 114}, {3, 44, 11, 45}, {17, 21, 4, 22}, {9, 13, 16, 14}},
	// version 20
	{{3, 107, 5, 108}, {3, 41, 13, 42}, {15, 24, 5, 25}, {15, 15, 10, 16}},
	// version 21
	{{4, 116, 4, 117}, {17, 42, 0, 0}, {17, 22, 6, 23}, {19, 16, 6, 17}},
	// version 22
	{{2, 111, 7, 112}, {17, 46, 0, 0}, {7, 24, 16, 25}, {34, 13, 0, 0}},
	// version 23
	{{4, 121, 5, 122}, {4, 47, 14, 48}, {11, 24, 14, 25}, {16, 15, 14, 16}},
	// version 24
	{{6, 117, 4, 118}, {6, 45, 14, 46}, {11, 24, 16, 25}, {30, 16, 2, 17}},
	// version 25
	{{8, 106, 4, 107}, {8, 47, 13, 48}, {7, 24, 22, 25}, {22, 15, 13, 16}},
	// version 26
	{{10, 114, 2, 115}, {19, 46, 4, 47}, {28, 22, 6, 23}, {33, 16, 4, 17}},
	// version 27
	{{8, 122, 4, 123}, {22, 45, 3, 46}, {8, 23, 26, 24}, {12, 15, 28, 16}},
	// version 28
	{{3, 117, 10, 118}, {3, 45, 23, 46}, {4, 24, 31, 25}, {11, 15, 31, 16}},
	// version 29
	{{7, 116, 7, 117}, {21, 45, 7, 46}, {1, 23, 37, 24}, {19, 15, 26, 16}},
	// version 30
	{{5, 115, 10, 116}, {19, 47, 10, 48}, {15, 24, 25, 25}, {23, 15, 25, 16}},
	// version 31
	{{13, 115, 3, 116}, {2, 46, 29, 47}, {42, 24, 1, 25}, {23, 15, 28, 16}},
	// version 32
	{{17, 115, 0, 0}, {10, 46, 23, 47}, {10, 24, 35, 25}, {19, 15, 35, 16}},
	// version 33
	{{17, 115, 1, 116}, {14, 46, 21, 47}, {29, 24, 19, 25}, {11, 15, 46, 16}},
	// version 34
	{{13, 115, 6, 116}, {14, 46, 23, 47}, {44, 24, 7, 25}, {59, 16, 1, 17}},
	// version 35
	{{12, 121, 7, 122}, {12, 47, 26, 48}, {39, 24, 14, 25}, {22, 15, 41, 16}},
	// version 36
	{{6, 121, 14, 122}, {6, 47, 34, 48}, {46, 24, 10, 25}, {2, 15, 64, 16}},
	// version 37
	{{17, 122, 4, 123}, {29, 46, 14, 47}, {49, 24, 10, 25}, {24, 15, 46, 16}},
	// version 38
	{{4, 122, 18, 123}, {13, 46, 32, 47}, {48, 24, 14, 25}, {42, 15, 32, 16}},
	// version 39
	{{20, 117, 4, 118}, {40, 47, 7, 48}, {43, 24, 22, 25}, {10, 15, 67, 16}},
	// version 40
	{{19, 118, 6, 119}, {18, 47, 31, 48}, {34, 24, 34, 25}, {20, 15, 61, 16}},
}

// alignmentTable holds the alignment pattern center coordinates for each QR
// version (1-40). Version 1 has no alignment patterns. Index 0 is unused.
var alignmentTable = [41][]int{
	{},                             // version 0
	{},                             // version 1 (no alignment)
	{6, 18},                        // version 2
	{6, 22},                        // version 3
	{6, 26},                        // version 4
	{6, 30},                        // version 5
	{6, 34},                        // version 6
	{6, 22, 38},                    // version 7
	{6, 24, 42},                    // version 8
	{6, 26, 46},                    // version 9
	{6, 28, 50},                    // version 10
	{6, 30, 54},                    // version 11
	{6, 32, 58},                    // version 12
	{6, 34, 62},                    // version 13
	{6, 26, 46, 66},                // version 14
	{6, 26, 48, 70},                // version 15
	{6, 26, 50, 74},                // version 16
	{6, 30, 54, 78},                // version 17
	{6, 30, 56, 82},                // version 18
	{6, 30, 58, 86},                // version 19
	{6, 34, 62, 90},                // version 20
	{6, 28, 50, 72, 94},            // version 21
	{6, 26, 50, 74, 98},            // version 22
	{6, 30, 54, 78, 102},           // version 23
	{6, 28, 54, 80, 106},           // version 24
	{6, 32, 58, 84, 110},           // version 25
	{6, 30, 58, 86, 114},           // version 26
	{6, 34, 62, 90, 118},           // version 27
	{6, 26, 50, 74, 98, 122},       // version 28
	{6, 30, 54, 78, 102, 126},      // version 29
	{6, 26, 52, 78, 104, 130},      // version 30
	{6, 30, 56, 82, 108, 134},      // version 31
	{6, 34, 60, 86, 112, 138},      // version 32
	{6, 30, 58, 86, 114, 142},      // version 33
	{6, 34, 62, 90, 118, 146},      // version 34
	{6, 30, 54, 78, 102, 126, 150}, // version 35
	{6, 24, 50, 76, 102, 128, 154}, // version 36
	{6, 28, 54, 80, 106, 132, 158}, // version 37
	{6, 32, 58, 84, 110, 136, 162}, // version 38
	{6, 26, 54, 82, 110, 138, 166}, // version 39
	{6, 30, 58, 86, 114, 142, 170}, // version 40
}

// alignmentPositions returns the alignment pattern center coordinates for the given version.
func alignmentPositions(version int) []int {
	if version < 1 || version > 40 {
		return nil
	}
	return alignmentTable[version]
}

// qrFormatInfo holds pre-computed BCH(15,5) format information strings for each
// ECC level and mask pattern combination, XORed with mask 0x5412.
var qrFormatInfo = [4][8]uint16{
	// ECCLevelL
	{0x77C4, 0x72F3, 0x7DAA, 0x789D, 0x662F, 0x6318, 0x6C41, 0x6976},
	// ECCLevelM
	{0x5412, 0x5125, 0x5E7C, 0x5B4B, 0x45F9, 0x40CE, 0x4F97, 0x4AA0},
	// ECCLevelQ
	{0x355F, 0x3068, 0x3F31, 0x3A06, 0x24B4, 0x2183, 0x2EDA, 0x2BED},
	// ECCLevelH
	{0x1689, 0x13BE, 0x1CE7, 0x19D0, 0x0762, 0x0255, 0x0D0C, 0x083B},
}

// qrVersionInfo holds BCH(18,6) encoded version information for versions 7-40,
// using generator polynomial 0x1F25. Versions 0-6 have no version info.
var qrVersionInfo = [41]uint32{
	0, 0, 0, 0, 0, 0, 0, // versions 0-6: no version info
	0x07C94, // version 7
	0x085BC, // version 8
	0x09A99, // version 9
	0x0A4D3, // version 10
	0x0BBF6, // version 11
	0x0C762, // version 12
	0x0D847, // version 13
	0x0E60D, // version 14
	0x0F928, // version 15
	0x10B78, // version 16
	0x1145D, // version 17
	0x12A17, // version 18
	0x13532, // version 19
	0x149A6, // version 20
	0x15683, // version 21
	0x168C9, // version 22
	0x177EC, // version 23
	0x18EC4, // version 24
	0x191E1, // version 25
	0x1AFAB, // version 26
	0x1B08E, // version 27
	0x1CC1A, // version 28
	0x1D33F, // version 29
	0x1ED75, // version 30
	0x1F250, // version 31
	0x209D5, // version 32
	0x216F0, // version 33
	0x228BA, // version 34
	0x2379F, // version 35
	0x24B0B, // version 36
	0x2542E, // version 37
	0x26A64, // version 38
	0x27541, // version 39
	0x28C69, // version 40
}

// placeFinder places a 7x7 finder pattern at (row, col) and reserves the
// surrounding one-module-wide separator zone.
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

// appendBitsN appends the lowest n bits of val (MSB first) to bits.
func appendBitsN(bits []bool, val, n int) []bool {
	for i := n - 1; i >= 0; i-- {
		bits = append(bits, (val>>i)&1 == 1)
	}
	return bits
}

// encodeNumericData encodes data in numeric mode.
func encodeNumericData(data string, version int) []bool {
	var bits []bool

	// Mode indicator: numeric = 0001.
	bits = append(bits, false, false, false, true)

	// Character count indicator.
	countBits := charCountBits(qrModeNumeric, version)
	bits = appendBitsN(bits, len(data), countBits)

	// Encode groups of 3 digits → 10 bits, 2 → 7 bits, 1 → 4 bits.
	i := 0
	for i+2 < len(data) {
		val := int(data[i]-'0')*100 + int(data[i+1]-'0')*10 + int(data[i+2]-'0')
		bits = appendBitsN(bits, val, 10)
		i += 3
	}
	switch remaining := len(data) - i; remaining {
	case 2:
		val := int(data[i]-'0')*10 + int(data[i+1]-'0')
		bits = appendBitsN(bits, val, 7)
	case 1:
		val := int(data[i] - '0')
		bits = appendBitsN(bits, val, 4)
	}

	return bits
}

// encodeAlphanumericData encodes data in alphanumeric mode.
func encodeAlphanumericData(data string, version int) []bool {
	var bits []bool

	// Mode indicator: alphanumeric = 0010.
	bits = append(bits, false, false, true, false)

	// Character count indicator.
	countBits := charCountBits(qrModeAlphanumeric, version)
	bits = appendBitsN(bits, len(data), countBits)

	// Encode pairs → 11 bits, remainder → 6 bits.
	i := 0
	for i+1 < len(data) {
		val := alphanumericValues[data[i]]*45 + alphanumericValues[data[i+1]]
		bits = appendBitsN(bits, val, 11)
		i += 2
	}
	if i < len(data) {
		bits = appendBitsN(bits, alphanumericValues[data[i]], 6)
	}

	return bits
}

// encodeByteData encodes data in byte mode.
func encodeByteData(data string, version int) []bool {
	var bits []bool

	// Mode indicator: byte mode = 0100.
	bits = append(bits, false, true, false, false)

	// Character count indicator (8 bits for versions 1-9, 16 for 10+).
	countBits := charCountBits(qrModeByte, version)
	bits = appendBitsN(bits, len(data), countBits)

	// Data bytes.
	for _, b := range []byte(data) {
		for i := 7; i >= 0; i-- {
			bits = append(bits, (b>>i)&1 == 1)
		}
	}

	return bits
}

// encodeQRData encodes data with the specified ECC level and encoding mode,
// returning the complete bitstream including interleaved error correction codewords.
func encodeQRData(data string, version int, level ECCLevel, mode qrMode) []bool {
	var bits []bool

	switch mode {
	case qrModeNumeric:
		bits = encodeNumericData(data, version)
	case qrModeAlphanumeric:
		bits = encodeAlphanumericData(data, version)
	default:
		bits = encodeByteData(data, version)
	}

	// Terminator (up to 4 zero bits).
	totalBits := qrDataCodewords[version][level] * 8
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

	// Add error correction codewords with multi-block interleaving.
	bits = appendECCInterleaved(bits, version, level)

	return bits
}

// appendECCInterleaved splits data into blocks, generates per-block ECC,
// and interleaves both data and ECC codewords per the QR spec.
func appendECCInterleaved(dataBits []bool, version int, level ECCLevel) []bool {
	// Convert bits to codewords (bytes).
	dataBytes := make([]byte, len(dataBits)/8)
	for i := range dataBytes {
		var b byte
		for j := range 8 {
			if dataBits[i*8+j] {
				b |= 1 << (7 - j)
			}
		}
		dataBytes[i] = b
	}

	bi := qrBlockTable[version][level]
	eccPerBlock := qrECCPerBlock[version][level]

	// Build blocks.
	totalBlocks := bi.group1Blocks + bi.group2Blocks
	blocks := make([][]byte, totalBlocks)
	offset := 0

	for i := range bi.group1Blocks {
		n := bi.group1DataCW
		blocks[i] = make([]byte, n)
		copy(blocks[i], dataBytes[offset:offset+n])
		offset += n
	}
	for i := range bi.group2Blocks {
		n := bi.group2DataCW
		blocks[bi.group1Blocks+i] = make([]byte, n)
		copy(blocks[bi.group1Blocks+i], dataBytes[offset:offset+n])
		offset += n
	}

	// Generate ECC for each block.
	generator := rsGeneratorPoly(eccPerBlock)
	eccBlocks := make([][]byte, totalBlocks)
	for i, block := range blocks {
		eccBlocks[i] = rsEncode(block, generator, eccPerBlock)
	}

	// Interleave data codewords.
	var interleaved []byte
	maxDataCW := bi.group1DataCW
	if bi.group2Blocks > 0 && bi.group2DataCW > maxDataCW {
		maxDataCW = bi.group2DataCW
	}
	for j := range maxDataCW {
		for i := range totalBlocks {
			if j < len(blocks[i]) {
				interleaved = append(interleaved, blocks[i][j])
			}
		}
	}

	// Interleave ECC codewords.
	for j := range eccPerBlock {
		for i := range totalBlocks {
			if j < len(eccBlocks[i]) {
				interleaved = append(interleaved, eccBlocks[i][j])
			}
		}
	}

	// Convert back to bits.
	result := make([]bool, 0, len(interleaved)*8)
	for _, b := range interleaved {
		for i := 7; i >= 0; i-- {
			result = append(result, (b>>i)&1 == 1)
		}
	}

	return result
}

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

// gfMul returns the product of a and b in GF(256).
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

// rsEncode performs Reed-Solomon encoding, returning eccLen error correction codewords.
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

// qrMaskFunc returns true if the module at (row, col) should be flipped for the given mask.
func qrMaskFunc(mask int, row, col int) bool {
	switch mask {
	case 0:
		return (row+col)%2 == 0
	case 1:
		return row%2 == 0
	case 2:
		return col%3 == 0
	case 3:
		return (row+col)%3 == 0
	case 4:
		return (row/2+col/3)%2 == 0
	case 5:
		return (row*col)%2+(row*col)%3 == 0
	case 6:
		return ((row*col)%2+(row*col)%3)%2 == 0
	case 7:
		return ((row*col)%3+(row+col)%2)%2 == 0
	}
	return false
}

// applyMask applies a mask pattern to a copy of the modules matrix.
// Only non-reserved modules are flipped.
func applyMask(modules, reserved [][]bool, size, mask int) [][]bool {
	result := make([][]bool, size)
	for r := range size {
		result[r] = make([]bool, size)
		copy(result[r], modules[r])
		for c := range size {
			if !reserved[r][c] && qrMaskFunc(mask, r, c) {
				result[r][c] = !result[r][c]
			}
		}
	}
	return result
}

// evaluateMasks tries all 8 mask patterns, computes penalty scores, and returns
// the best mask index and the masked modules matrix.
func evaluateMasks(modules, reserved [][]bool, size int) (int, [][]bool) {
	bestMask := 0
	bestPenalty := int(^uint(0) >> 1) // max int
	var bestModules [][]bool

	for mask := range 8 {
		masked := applyMask(modules, reserved, size, mask)
		penalty := penaltyScore(masked, size)
		if penalty < bestPenalty {
			bestPenalty = penalty
			bestMask = mask
			bestModules = masked
		}
	}

	return bestMask, bestModules
}

// penaltyScore computes the total penalty score for a masked QR matrix
// using all four rules from ISO 18004 section 7.8.3.
func penaltyScore(modules [][]bool, size int) int {
	return penaltyRule1(modules, size) +
		penaltyRule2(modules, size) +
		penaltyRule3(modules, size) +
		penaltyRule4(modules, size)
}

// penaltyRule1 scores adjacent same-colored modules in rows and columns.
// Each run of 5 or more adds a penalty of 3 + (run length - 5).
func penaltyRule1(modules [][]bool, size int) int {
	penalty := 0

	// Horizontal runs.
	for r := range size {
		count := 1
		for c := 1; c < size; c++ {
			if modules[r][c] == modules[r][c-1] {
				count++
			} else {
				if count >= 5 {
					penalty += 3 + (count - 5)
				}
				count = 1
			}
		}
		if count >= 5 {
			penalty += 3 + (count - 5)
		}
	}

	// Vertical runs.
	for c := range size {
		count := 1
		for r := 1; r < size; r++ {
			if modules[r][c] == modules[r-1][c] {
				count++
			} else {
				if count >= 5 {
					penalty += 3 + (count - 5)
				}
				count = 1
			}
		}
		if count >= 5 {
			penalty += 3 + (count - 5)
		}
	}

	return penalty
}

// penaltyRule2 scores 2x2 blocks of same-colored modules, adding 3 per block.
func penaltyRule2(modules [][]bool, size int) int {
	count := 0
	for r := range size - 1 {
		for c := range size - 1 {
			v := modules[r][c]
			if modules[r][c+1] == v && modules[r+1][c] == v && modules[r+1][c+1] == v {
				count++
			}
		}
	}
	return 3 * count
}

// penaltyRule3 scores finder-like patterns (1:1:3:1:1 ratio with a 4-module
// light zone on either side), adding 40 per occurrence.
func penaltyRule3(modules [][]bool, size int) int {
	count := 0
	// Pattern: dark, light, dark dark dark, light, dark, light light light light
	// or the reverse.
	p1 := [11]bool{true, false, true, true, true, false, true, false, false, false, false}
	p2 := [11]bool{false, false, false, false, true, false, true, true, true, false, true}

	for r := range size {
		for c := 0; c <= size-11; c++ {
			match1 := true
			match2 := true
			for k := range 11 {
				if modules[r][c+k] != p1[k] {
					match1 = false
				}
				if modules[r][c+k] != p2[k] {
					match2 = false
				}
				if !match1 && !match2 {
					break
				}
			}
			if match1 || match2 {
				count++
			}
		}
	}

	for c := range size {
		for r := 0; r <= size-11; r++ {
			match1 := true
			match2 := true
			for k := range 11 {
				if modules[r+k][c] != p1[k] {
					match1 = false
				}
				if modules[r+k][c] != p2[k] {
					match2 = false
				}
				if !match1 && !match2 {
					break
				}
			}
			if match1 || match2 {
				count++
			}
		}
	}

	return 40 * count
}

// penaltyRule4 scores color imbalance. The penalty is 10 * k where
// k = floor(|50 - percentDark| / 5) * 2.
func penaltyRule4(modules [][]bool, size int) int {
	dark := 0
	total := size * size
	for r := range size {
		for c := range size {
			if modules[r][c] {
				dark++
			}
		}
	}
	percent := dark * 100 / total
	diff := percent - 50
	if diff < 0 {
		diff = -diff
	}
	k := (diff / 5) * 2
	return 10 * k
}

// placeFormatInfo places the 15-bit format information string.
func placeFormatInfo(modules [][]bool, size int, level ECCLevel, mask int) {
	info := qrFormatInfo[level][mask]

	// Convert to 15-bit array (MSB first).
	var formatBits [15]bool
	for i := range 15 {
		formatBits[i] = (info>>(14-i))&1 == 1
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

// placeVersionInfo places the 18-bit version information for versions 7+.
// Two copies: bottom-left (6x3 block) and top-right (3x6 block).
func placeVersionInfo(modules [][]bool, size, version int) {
	if version < 7 {
		return
	}
	info := qrVersionInfo[version]

	for i := range 18 {
		bit := (info>>(17-i))&1 == 1
		// i maps to position: row = i/3, col = i%3
		// Bottom-left block: rows (size-11)..(size-9), cols 0..5
		r := i / 3
		c := i % 3
		modules[size-11+c][r] = bit // bottom-left
		modules[r][size-11+c] = bit // top-right
	}
}
