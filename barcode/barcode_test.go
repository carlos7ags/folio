// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package barcode

import (
	"testing"

	"github.com/carlos7ags/folio/content"
)

// --- Code 128 tests ---

func TestCode128Basic(t *testing.T) {
	bc, err := Code128("Hello")
	if err != nil {
		t.Fatalf("Code128 failed: %v", err)
	}
	if bc.Width() == 0 || bc.Height() == 0 {
		t.Error("barcode should have non-zero dimensions")
	}
}

func TestCode128Digits(t *testing.T) {
	bc, err := Code128("1234567890")
	if err != nil {
		t.Fatalf("Code128 failed: %v", err)
	}
	if bc.Width() == 0 {
		t.Error("barcode should have non-zero width")
	}
}

func TestCode128Empty(t *testing.T) {
	_, err := Code128("")
	if err == nil {
		t.Error("expected error for empty data")
	}
}

func TestCode128InvalidChar(t *testing.T) {
	_, err := Code128("Hello\x01World")
	if err == nil {
		t.Error("expected error for control character")
	}
}

func TestCode128Draw(t *testing.T) {
	bc, err := Code128("Test123")
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
	bc, err := Code128(data)
	if err != nil {
		t.Fatalf("Code128 with all printable ASCII failed: %v", err)
	}
	if bc.Width() == 0 {
		t.Error("expected non-zero width")
	}
}

// --- QR Code tests ---

func TestQRBasic(t *testing.T) {
	bc, err := QR("Hello World")
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
	bc, err := QR("https://example.com/folio")
	if err != nil {
		t.Fatalf("QR failed: %v", err)
	}
	if bc.Width() == 0 {
		t.Error("expected non-zero dimensions")
	}
}

func TestQREmpty(t *testing.T) {
	_, err := QR("")
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
	bc, err := QR(string(data))
	if err != nil {
		t.Fatalf("QR with 200 bytes failed: %v", err)
	}
	if bc.Width() == 0 {
		t.Error("expected non-zero dimensions")
	}
}

func TestQRTooLong(t *testing.T) {
	data := make([]byte, 300)
	for i := range data {
		data[i] = 'X'
	}
	_, err := QR(string(data))
	if err == nil {
		t.Error("expected error for data exceeding capacity")
	}
}

func TestQRDraw(t *testing.T) {
	bc, err := QR("Test")
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
	bc1, _ := QR("Hi")
	if bc1.Width() != 21 {
		t.Errorf("short data: size = %d, want 21 (version 1)", bc1.Width())
	}

	// Medium data → larger version.
	bc2, _ := QR("This is a longer string that needs more capacity")
	if bc2.Width() <= 21 {
		t.Errorf("medium data: size = %d, should be > 21", bc2.Width())
	}
}

// --- EAN-13 tests ---

func TestEAN13Valid(t *testing.T) {
	bc, err := EAN13("5901234123457")
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
	bc, err := EAN13("590123412345")
	if err != nil {
		t.Fatalf("EAN13 with 12 digits failed: %v", err)
	}
	if bc.Width() == 0 {
		t.Error("expected non-zero width")
	}
}

func TestEAN13WrongCheckDigit(t *testing.T) {
	_, err := EAN13("5901234123450") // correct is 7
	if err == nil {
		t.Error("expected error for wrong check digit")
	}
}

func TestEAN13WrongLength(t *testing.T) {
	_, err := EAN13("12345")
	if err == nil {
		t.Error("expected error for wrong length")
	}
}

func TestEAN13NonNumeric(t *testing.T) {
	_, err := EAN13("590123412345A")
	if err == nil {
		t.Error("expected error for non-numeric")
	}
}

func TestEAN13Draw(t *testing.T) {
	bc, err := EAN13("590123412345")
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
