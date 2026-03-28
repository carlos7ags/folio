// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package barcode

import (
	"fmt"
	"testing"

	"github.com/carlos7ags/folio/content"
)

// --- Code 128 tests ---

func TestCode128Basic(t *testing.T) {
	bc, err := NewCode128("Hello")
	if err != nil {
		t.Fatalf("Code128 failed: %v", err)
	}
	if bc.Width() == 0 || bc.Height() == 0 {
		t.Error("barcode should have non-zero dimensions")
	}
}

func TestCode128Digits(t *testing.T) {
	bc, err := NewCode128("1234567890")
	if err != nil {
		t.Fatalf("Code128 failed: %v", err)
	}
	if bc.Width() == 0 {
		t.Error("barcode should have non-zero width")
	}
}

func TestCode128Empty(t *testing.T) {
	_, err := NewCode128("")
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestCode128InvalidChar(t *testing.T) {
	_, err := NewCode128("Hello\x01World")
	if err == nil {
		t.Error("expected error for control character")
	}
}

func TestCode128Draw(t *testing.T) {
	bc, err := NewCode128("Test123")
	if err != nil {
		t.Fatal(err)
	}
	stream := content.NewStream()
	bc.Draw(stream, 0, 0, 200, 50)
	bytes := stream.Bytes()
	if len(bytes) == 0 {
		t.Error("expected content stream output")
	}
}

func TestCode128AllPrintable(t *testing.T) {
	// Test all printable ASCII characters.
	data := ""
	for ch := byte(32); ch < 127; ch++ {
		data += string(ch)
	}
	bc, err := NewCode128(data)
	if err != nil {
		t.Fatalf("Code128 with all printable ASCII failed: %v", err)
	}
	if bc.Width() == 0 {
		t.Error("expected non-zero width")
	}
}

// --- Code 128 pattern validation tests ---

func TestCode128PatternsLength(t *testing.T) {
	for i, p := range code128Patterns {
		if len(p) != 11 {
			t.Errorf("pattern %d has %d bools, want 11", i, len(p))
		}
	}
}

func TestCode128PatternsStartWithBar(t *testing.T) {
	for i, p := range code128Patterns {
		if len(p) > 0 && !p[0] {
			t.Errorf("pattern %d starts with space (false), want bar (true)", i)
		}
	}
}

func TestCode128PatternsUnique(t *testing.T) {
	seen := make(map[string]int)
	for i, p := range code128Patterns {
		key := fmt.Sprintf("%v", p)
		if prev, ok := seen[key]; ok {
			t.Errorf("pattern %d is a duplicate of pattern %d", i, prev)
		}
		seen[key] = i
	}
}

func TestCode128StopPattern(t *testing.T) {
	if len(code128Stop) != 13 {
		t.Errorf("stop pattern has %d bools, want 13", len(code128Stop))
	}
	if !code128Stop[0] {
		t.Error("stop pattern should start with bar (true)")
	}
}

// --- QR Code tests ---

func TestQRBasic(t *testing.T) {
	bc, err := NewQR("Hello World")
	if err != nil {
		t.Fatalf("QR failed: %v", err)
	}
	if bc.Width() != bc.Height() {
		t.Errorf("QR should be square, got %dx%d", bc.Width(), bc.Height())
	}
	// Version 1 QR is 21x21.
	if bc.Width() < 21 {
		t.Errorf("QR size = %d, expected >= 21", bc.Width())
	}
}

func TestQRURL(t *testing.T) {
	bc, err := NewQR("https://example.com/folio")
	if err != nil {
		t.Fatalf("QR failed: %v", err)
	}
	if bc.Width() == 0 {
		t.Error("expected non-zero dimensions")
	}
}

func TestQREmpty(t *testing.T) {
	_, err := NewQR("")
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestQRLongData(t *testing.T) {
	// Test near-capacity data for version 10.
	data := make([]byte, 200)
	for i := range data {
		data[i] = 'A' + byte(i%26)
	}
	bc, err := NewQR(string(data))
	if err != nil {
		t.Fatalf("QR with 200 bytes failed: %v", err)
	}
	if bc.Width() == 0 {
		t.Error("expected non-zero dimensions")
	}
}

func TestQRTooLong(t *testing.T) {
	data := make([]byte, 2400) // exceeds version 40 level M byte capacity (2331)
	for i := range data {
		data[i] = 'x' // lowercase forces byte mode
	}
	_, err := NewQR(string(data))
	if err == nil {
		t.Error("expected error for data exceeding capacity")
	}
}

func TestQRDraw(t *testing.T) {
	bc, err := NewQR("Test")
	if err != nil {
		t.Fatal(err)
	}
	stream := content.NewStream()
	bc.Draw(stream, 10, 10, 100, 100)
	if len(stream.Bytes()) == 0 {
		t.Error("expected content stream output")
	}
}

func TestQRVersionSelection(t *testing.T) {
	// Short data → version 1 (21x21).
	bc1, _ := NewQR("Hi")
	if bc1.Width() != 21 {
		t.Errorf("short data: size = %d, want 21 (version 1)", bc1.Width())
	}

	// Medium data → larger version.
	bc2, _ := NewQR("This is a longer string that needs more capacity")
	if bc2.Width() <= 21 {
		t.Errorf("medium data: size = %d, should be > 21", bc2.Width())
	}
}

func TestQRNumericMode(t *testing.T) {
	bc, err := NewQR("1234567890")
	if err != nil {
		t.Fatalf("QR numeric failed: %v", err)
	}
	// Numeric mode is more efficient, so "1234567890" (10 digits) should fit in version 1.
	// Version 1 at level M supports 34 numeric characters.
	if bc.Width() != 21 {
		t.Errorf("numeric data: size = %d, want 21 (version 1)", bc.Width())
	}
}

func TestQRAlphanumericMode(t *testing.T) {
	bc, err := NewQR("HELLO WORLD")
	if err != nil {
		t.Fatalf("QR alphanumeric failed: %v", err)
	}
	// "HELLO WORLD" is 11 chars, version 1 level M supports 20 alphanumeric chars.
	if bc.Width() != 21 {
		t.Errorf("alphanumeric data: size = %d, want 21 (version 1)", bc.Width())
	}
}

func TestQRModeDetection(t *testing.T) {
	if m := detectMode("12345"); m != qrModeNumeric {
		t.Errorf("expected numeric mode for digits, got %d", m)
	}
	if m := detectMode("HELLO 123"); m != qrModeAlphanumeric {
		t.Errorf("expected alphanumeric mode for uppercase+digits+space, got %d", m)
	}
	if m := detectMode("hello"); m != qrModeByte {
		t.Errorf("expected byte mode for lowercase, got %d", m)
	}
}

func TestQRVersion21Plus(t *testing.T) {
	// 700 bytes of lowercase forces byte mode; v20 max is 666 bytes at level M.
	data := make([]byte, 700)
	for i := range data {
		data[i] = 'a' + byte(i%26) // lowercase forces byte mode
	}
	bc, err := NewQR(string(data))
	if err != nil {
		t.Fatalf("QR with 700 bytes failed: %v", err)
	}
	// Version 21 size = 17 + 21*4 = 101.
	if bc.Width() < 101 {
		t.Errorf("700 bytes should need version 21+, got size %d", bc.Width())
	}
}

func TestQRVersion40(t *testing.T) {
	// Test near max capacity at level L (2953 bytes for v40).
	data := make([]byte, 2900)
	for i := range data {
		data[i] = byte(32 + i%95)
	}
	bc, err := NewQRWithECC(string(data), ECCLevelL)
	if err != nil {
		t.Fatalf("QR version 40 level L failed: %v", err)
	}
	// Version 40 size = 17 + 40*4 = 177.
	if bc.Width() != 177 {
		t.Errorf("expected version 40 (177x177), got %dx%d", bc.Width(), bc.Height())
	}
}

func TestQRNumericLargeCapacity(t *testing.T) {
	// Numeric mode at level L version 40 can hold ~7089 digits.
	// Test with 5000 digits to verify large numeric encoding works.
	data := make([]byte, 5000)
	for i := range data {
		data[i] = '0' + byte(i%10)
	}
	bc, err := NewQRWithECC(string(data), ECCLevelL)
	if err != nil {
		t.Fatalf("QR with 5000 digits failed: %v", err)
	}
	if bc.Width() == 0 {
		t.Error("expected non-zero dimensions")
	}
}

func TestQRAlphanumericHelpers(t *testing.T) {
	if !isNumeric("0123456789") {
		t.Error("expected isNumeric true for all digits")
	}
	if isNumeric("123A") {
		t.Error("expected isNumeric false with letter")
	}
	if !isAlphanumeric("HELLO WORLD 123 $%*+-./:") {
		t.Error("expected isAlphanumeric true for valid chars")
	}
	if isAlphanumeric("hello") {
		t.Error("expected isAlphanumeric false for lowercase")
	}
}

// --- EAN-13 tests ---

func TestEAN13Valid(t *testing.T) {
	bc, err := NewEAN13("5901234123457")
	if err != nil {
		t.Fatalf("EAN13 failed: %v", err)
	}
	// EAN-13 is 95 modules + 18 quiet zone = 113 modules wide.
	expectedWidth := 113
	if bc.Width() != expectedWidth {
		t.Errorf("width = %d, want %d", bc.Width(), expectedWidth)
	}
}

func TestEAN13AutoCheckDigit(t *testing.T) {
	// Provide 12 digits, check digit should be computed.
	bc, err := NewEAN13("590123412345")
	if err != nil {
		t.Fatalf("EAN13 with 12 digits failed: %v", err)
	}
	if bc.Width() == 0 {
		t.Error("expected non-zero width")
	}
}

func TestEAN13WrongCheckDigit(t *testing.T) {
	_, err := NewEAN13("5901234123450") // correct is 7
	if err == nil {
		t.Error("expected error for wrong check digit")
	}
}

func TestEAN13WrongLength(t *testing.T) {
	_, err := NewEAN13("12345")
	if err == nil {
		t.Error("expected error for wrong length")
	}
}

func TestEAN13NonNumeric(t *testing.T) {
	_, err := NewEAN13("590123412345A")
	if err == nil {
		t.Error("expected error for non-numeric")
	}
}

func TestEAN13Draw(t *testing.T) {
	bc, err := NewEAN13("590123412345")
	if err != nil {
		t.Fatal(err)
	}
	stream := content.NewStream()
	bc.Draw(stream, 0, 0, 150, 40)
	if len(stream.Bytes()) == 0 {
		t.Error("expected content stream output")
	}
}

func TestEAN13CheckDigitComputation(t *testing.T) {
	tests := []struct {
		input string
		check int
	}{
		{"590123412345", 7},
		{"400638133393", 1},
		{"012345678901", 2},
	}
	for _, tt := range tests {
		got := ean13CheckDigit(tt.input)
		if got != tt.check {
			t.Errorf("checkDigit(%q) = %d, want %d", tt.input, got, tt.check)
		}
	}
}
