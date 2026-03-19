// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package barcode

import "fmt"

// Code128 generates a Code 128 barcode from a string.
// Supports the full ASCII character set (Code B).
// Returns an error if the input contains characters outside ASCII 0-127.
func Code128(data string) (*Barcode, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("barcode: empty data")
	}

	// Encode using Code B (ASCII 32-127).
	values, err := encodeCode128B(data)
	if err != nil {
		return nil, err
	}

	// Build the module pattern.
	var modules []bool

	// Quiet zone (10 modules of white).
	for range 10 {
		modules = append(modules, false)
	}

	// Start code B.
	modules = append(modules, code128Patterns[104]...)

	// Data characters.
	checksum := 104 // start code B value
	for i, v := range values {
		modules = append(modules, code128Patterns[v]...)
		checksum += v * (i + 1)
	}

	// Checksum character.
	checksum %= 103
	modules = append(modules, code128Patterns[checksum]...)

	// Stop pattern (13 modules: 2331112).
	modules = append(modules, code128Stop...)

	// Quiet zone.
	for range 10 {
		modules = append(modules, false)
	}

	return new1D(modules, 50), nil
}

// encodeCode128B converts ASCII text to Code 128 Code B values.
func encodeCode128B(data string) ([]int, error) {
	values := make([]int, len(data))
	for i, ch := range data {
		if ch < 32 || ch > 127 {
			return nil, fmt.Errorf("barcode: Code 128B does not support character %d at position %d", ch, i)
		}
		values[i] = int(ch) - 32
	}
	return values, nil
}

// code128Patterns contains the bar/space patterns for Code 128.
// Each pattern is 11 modules (6 alternating bars and spaces).
// Index 0-105 = data/control characters.
// Patterns sourced from ISO/IEC 15417 (Code 128 specification).
var code128Patterns = [106][]bool{
	{true, true, false, true, true, false, false, true, true, false, false},   // 0
	{true, true, false, false, true, true, false, true, true, false, false},   // 1
	{true, true, false, false, true, true, false, false, true, true, false},   // 2
	{true, false, false, true, false, false, true, true, false, false, false}, // 3
	{true, false, false, true, false, false, false, true, true, false, false}, // 4
	{true, false, false, false, true, false, false, true, true, false, false}, // 5
	{true, false, false, true, true, false, false, true, false, false, false}, // 6
	{true, false, false, true, true, false, false, false, true, false, false}, // 7
	{true, false, false, false, true, true, false, false, true, false, false}, // 8
	{true, true, false, false, true, false, false, true, false, false, false}, // 9
	{true, true, false, false, true, false, false, false, true, false, false}, // 10
	{true, true, false, false, false, true, false, false, true, false, false}, // 11
	{true, false, true, true, false, false, true, true, true, false, false},   // 12
	{true, false, false, true, true, false, true, true, true, false, false},   // 13
	{true, false, false, true, true, false, false, true, true, true, false},   // 14
	{true, false, true, true, true, false, false, true, true, false, false},   // 15
	{true, false, false, true, true, true, false, true, true, false, false},   // 16
	{true, false, false, true, true, true, false, false, true, true, false},   // 17
	{true, true, false, false, true, true, true, false, false, true, false},   // 18
	{true, true, false, false, true, false, true, true, true, false, false},   // 19
	{true, true, false, false, true, false, false, true, true, true, false},   // 20
	{true, true, false, true, true, true, false, false, true, false, false},   // 21
	{true, true, false, false, true, true, true, false, true, false, false},   // 22
	{true, true, true, false, true, true, false, true, true, true, false},     // 23
	{true, true, true, false, true, false, false, true, true, false, false},   // 24
	{true, true, true, false, false, true, false, true, true, false, false},   // 25
	{true, true, true, false, false, true, false, false, true, true, false},   // 26
	{true, true, true, false, true, true, false, false, true, false, false},   // 27
	{true, true, true, false, false, true, true, false, true, false, false},   // 28
	{true, true, true, false, false, true, true, false, false, true, false},   // 29
	{true, true, false, true, true, false, true, true, false, false, false},   // 30
	{true, true, false, true, true, false, false, false, true, true, false},   // 31
	{true, true, false, false, false, true, true, false, true, true, false},   // 32
	{true, false, true, false, false, false, true, true, false, false, false}, // 33
	{true, false, false, false, true, false, true, true, false, false, false}, // 34
	{true, false, false, false, true, false, false, false, true, true, false}, // 35
	{true, false, true, true, false, false, false, true, false, false, false}, // 36
	{true, false, false, false, true, true, false, true, false, false, false}, // 37
	{true, false, false, false, true, true, false, false, false, true, false}, // 38
	{true, true, false, true, false, false, false, true, false, false, false}, // 39
	{true, true, false, false, false, true, false, true, false, false, false}, // 40
	{true, true, false, false, false, true, false, false, false, true, false}, // 41
	{true, false, true, true, false, true, true, true, false, false, false},   // 42
	{true, false, true, true, false, false, false, true, true, true, false},   // 43
	{true, false, false, false, true, true, false, true, true, true, false},   // 44
	{true, false, true, true, true, false, true, true, false, false, false},   // 45
	{true, false, true, true, true, false, false, false, true, true, false},   // 46
	{true, false, false, false, true, true, true, false, true, true, false},   // 47
	{true, true, true, false, true, true, true, false, true, true, false},     // 48
	{true, true, false, true, false, false, false, true, true, true, false},   // 49
	{true, true, false, false, false, true, false, true, true, true, false},   // 50
	{true, true, false, true, true, true, false, true, false, false, false},   // 51
	{true, true, false, true, true, true, false, false, false, true, false},   // 52
	{true, true, false, true, true, true, false, true, true, true, false},     // 53
	{true, true, true, false, true, false, true, true, false, false, false},   // 54
	{true, true, true, false, true, false, false, false, true, true, false},   // 55
	{true, true, true, false, false, false, true, false, true, true, false},   // 56
	{true, true, true, false, true, true, false, true, false, false, false},   // 57
	{true, true, true, false, true, true, false, false, false, true, false},   // 58
	{true, true, true, false, false, false, true, true, false, true, false},   // 59
	{true, true, true, false, true, true, true, true, false, true, false},     // 60
	{true, true, false, false, true, false, false, false, false, true, false}, // 61
	{true, true, true, true, false, false, false, true, false, true, false},   // 62
	{true, false, true, false, false, true, true, false, false, false, false}, // 63
	{true, false, true, false, false, false, false, true, true, false, false}, // 64
	{true, false, false, true, false, true, true, false, false, false, false}, // 65
	{true, false, false, true, false, false, false, false, true, true, false}, // 66
	{true, false, false, false, false, true, false, true, true, false, false}, // 67
	{true, false, false, false, false, true, false, false, true, true, false}, // 68
	{true, false, true, true, false, false, true, false, false, false, false}, // 69
	{true, false, true, true, false, false, false, false, true, false, false}, // 70
	{true, false, false, true, true, false, true, false, false, false, false}, // 71
	{true, false, false, true, true, false, false, false, false, true, false}, // 72
	{true, false, false, false, false, true, true, false, true, false, false}, // 73
	{true, false, false, false, false, true, true, false, false, true, false}, // 74
	{true, true, false, false, false, false, true, false, false, true, false}, // 75
	{true, true, false, false, true, false, true, false, false, false, false}, // 76
	{true, true, true, true, false, true, true, true, false, true, false},     // 77
	{true, true, false, false, false, false, true, false, true, false, false}, // 78
	{true, false, false, false, true, true, true, true, false, true, false},   // 79
	{true, false, true, false, false, true, true, true, true, false, false},   // 80
	{true, false, false, true, false, true, true, true, true, false, false},   // 81
	{true, false, false, true, false, false, true, true, true, true, false},   // 82
	{true, false, true, true, true, true, false, false, true, false, false},   // 83
	{true, false, false, true, true, true, true, false, true, false, false},   // 84
	{true, false, false, true, true, true, true, false, false, true, false},   // 85
	{true, true, true, true, false, true, false, false, true, false, false},   // 86
	{true, true, true, true, false, false, true, false, true, false, false},   // 87
	{true, true, true, true, false, false, true, false, false, true, false},   // 88
	{true, true, false, true, true, false, true, true, true, true, false},     // 89
	{true, true, false, true, true, true, true, false, true, true, false},     // 90
	{true, true, true, true, false, true, true, false, true, true, false},     // 91
	{true, false, true, false, true, true, true, true, false, false, false},   // 92
	{true, false, true, false, false, false, true, true, true, true, false},   // 93
	{true, false, false, false, true, false, true, true, true, true, false},   // 94
	{true, false, true, true, true, true, false, true, false, false, false},   // 95
	{true, false, true, true, true, true, false, false, false, true, false},   // 96
	{true, true, true, true, false, true, false, true, false, false, false},   // 97
	{true, true, true, true, false, true, false, false, false, true, false},   // 98
	{true, false, true, true, true, false, true, true, true, true, false},     // 99
	{true, false, true, true, true, true, false, true, true, true, false},     // 100
	{true, true, true, false, true, false, true, true, true, true, false},     // 101
	{true, true, true, true, false, true, false, true, true, true, false},     // 102
	{true, true, false, true, false, false, false, false, true, false, false}, // 103: Start A
	{true, true, false, true, false, false, true, false, false, false, false}, // 104: Start B
	{true, true, false, true, false, false, true, true, true, false, false},   // 105: Start C
}

// code128Stop is the stop pattern (13 modules: 2331112).
var code128Stop = []bool{
	true, true, false, false, false, true, true, true, false, true, false, true, true,
}
