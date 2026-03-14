// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package barcode

import "fmt"

// EAN13 generates an EAN-13 barcode from a 13-digit string.
// If 12 digits are provided, the check digit is computed automatically.
// Returns an error if the input is not 12 or 13 digits.
func EAN13(data string) (*Barcode, error) {
	// Validate input.
	for _, ch := range data {
		if ch < '0' || ch > '9' {
			return nil, fmt.Errorf("barcode: EAN-13 data must be numeric, got %q", string(ch))
		}
	}

	if len(data) == 12 {
		data += string(rune('0' + ean13CheckDigit(data)))
	} else if len(data) != 13 {
		return nil, fmt.Errorf("barcode: EAN-13 requires 12 or 13 digits, got %d", len(data))
	}

	// Verify check digit.
	expected := ean13CheckDigit(data[:12])
	actual := int(data[12] - '0')
	if actual != expected {
		return nil, fmt.Errorf("barcode: EAN-13 check digit mismatch: expected %d, got %d", expected, actual)
	}

	// Build module pattern.
	var modules []bool

	// Quiet zone (9 modules).
	for range 9 {
		modules = append(modules, false)
	}

	// Start guard: 101.
	modules = append(modules, true, false, true)

	// Left half (digits 2-7, encoded based on first digit parity).
	firstDigit := int(data[0] - '0')
	parity := ean13Parity[firstDigit]

	for i := 1; i <= 6; i++ {
		digit := int(data[i] - '0')
		if parity[i-1] == 'O' {
			modules = append(modules, eanLEncoding[digit]...)
		} else {
			modules = append(modules, eanGEncoding[digit]...)
		}
	}

	// Center guard: 01010.
	modules = append(modules, false, true, false, true, false)

	// Right half (digits 8-13, R-encoding).
	for i := 7; i <= 12; i++ {
		digit := int(data[i] - '0')
		modules = append(modules, eanREncoding[digit]...)
	}

	// End guard: 101.
	modules = append(modules, true, false, true)

	// Quiet zone.
	for range 9 {
		modules = append(modules, false)
	}

	return new1D(modules, 60), nil
}

// ean13CheckDigit computes the EAN-13 check digit for a 12-digit string.
func ean13CheckDigit(data string) int {
	sum := 0
	for i := range 12 {
		d := int(data[i] - '0')
		if i%2 == 0 {
			sum += d
		} else {
			sum += d * 3
		}
	}
	return (10 - sum%10) % 10
}

// ean13Parity defines the L/G encoding pattern for the left half
// based on the first digit. 'O' = L-encoding (odd), 'E' = G-encoding (even).
var ean13Parity = [10]string{
	"OOOOOO", // 0
	"OOEOEE", // 1
	"OOEEOE", // 2
	"OOEEEO", // 3
	"OEOOEE", // 4
	"OEEOOE", // 5
	"OEEEOO", // 6
	"OEOEOE", // 7
	"OEOEEO", // 8
	"OEEOEO", // 9
}

// eanLEncoding is the L-code (odd parity) for digits 0-9.
// Each digit is 7 modules wide.
var eanLEncoding = [10][]bool{
	{false, false, false, true, true, false, true}, // 0
	{false, false, true, true, false, false, true}, // 1
	{false, false, true, false, false, true, true}, // 2
	{false, true, true, true, true, false, true},   // 3
	{false, true, false, false, false, true, true}, // 4
	{false, true, true, false, false, false, true}, // 5
	{false, true, false, true, true, true, true},   // 6
	{false, true, true, true, false, true, true},   // 7
	{false, true, true, false, true, true, true},   // 8
	{false, false, false, true, false, true, true}, // 9
}

// eanGEncoding is the G-code (even parity) for digits 0-9.
// Mirror of R-code.
var eanGEncoding = [10][]bool{
	{false, true, false, false, true, true, true},   // 0
	{false, true, true, false, false, true, true},   // 1
	{false, false, true, true, false, true, true},   // 2
	{false, true, false, false, false, false, true}, // 3
	{false, false, true, true, true, false, true},   // 4
	{false, true, true, true, false, false, true},   // 5
	{false, false, false, false, true, false, true}, // 6
	{false, false, true, false, false, false, true}, // 7
	{false, false, false, true, false, false, true}, // 8
	{false, false, true, false, true, true, true},   // 9
}

// eanREncoding is the R-code for digits 0-9 (complement of L-code).
var eanREncoding = [10][]bool{
	{true, true, true, false, false, true, false},   // 0
	{true, true, false, false, true, true, false},   // 1
	{true, true, false, true, true, false, false},   // 2
	{true, false, false, false, false, true, false}, // 3
	{true, false, true, true, true, false, false},   // 4
	{true, false, false, true, true, true, false},   // 5
	{true, false, true, false, false, false, false}, // 6
	{true, false, false, false, true, false, false}, // 7
	{true, false, false, true, false, false, false}, // 8
	{true, true, true, false, true, false, false},   // 9
}
