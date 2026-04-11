// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package barcode

import (
	"testing"
)

// FuzzNewCode128 exercises the Code 128 encoder with arbitrary byte
// inputs. The encoder is expected to accept ASCII 32-127 and reject
// everything else (including empty input); the contract we enforce
// here is that it must never panic and must always either produce a
// Barcode with positive dimensions or return an error.
func FuzzNewCode128(f *testing.F) {
	seeds := []string{
		"",
		"A",
		"Hello",
		"1234567890",
		"SKU-12345",
		"ISBN 978-0-596-00712-6",
		string([]byte{0x00}), // NUL (out of Code B range)
		string([]byte{0xFF}), // high-byte (out of ASCII)
		"\t\n\r",             // control whitespace (out of range)
		"é",                  // UTF-8 multi-byte (out of range)
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		bc, err := NewCode128(data)
		if err != nil {
			return // rejected inputs are fine
		}
		if bc == nil {
			t.Fatalf("NewCode128(%q): got nil barcode with nil error", data)
		}
		if bc.Width() <= 0 || bc.Height() <= 0 {
			t.Errorf("NewCode128(%q): got dimensions %dx%d", data, bc.Width(), bc.Height())
		}
	})
}

// FuzzNewEAN13 feeds arbitrary bytes to the EAN-13 encoder. Valid EAN
// is 12-13 digits; everything else must be rejected cleanly.
func FuzzNewEAN13(f *testing.F) {
	seeds := []string{
		"",
		"1234567890123", // 13 digits, valid check
		"123456789012",  // 12 digits, auto check
		"123456789012X", // non-digit
		"000000000000",  // 12 zeros
		"999999999999",
		"12",               // too short
		"1234567890123456", // too long
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		bc, err := NewEAN13(data)
		if err != nil {
			return
		}
		if bc == nil {
			t.Fatalf("NewEAN13(%q): got nil barcode with nil error", data)
		}
		if bc.Width() <= 0 || bc.Height() <= 0 {
			t.Errorf("NewEAN13(%q): got dimensions %dx%d", data, bc.Width(), bc.Height())
		}
	})
}

// FuzzNewQR feeds arbitrary bytes to the QR encoder. QR can encode any
// byte string up to its capacity limit, so the acceptable outcomes are
// a valid module grid or an error ("data too long"). The fuzzer runs
// at EC level M (the default) to match the most common code path.
func FuzzNewQR(f *testing.F) {
	seeds := []string{
		"",
		"A",
		"Hello, World!",
		"12345",                // numeric mode
		"HELLO",                // alphanumeric mode
		"https://example.com",  // URL (byte mode)
		"日本語",                  // CJK (UTF-8 multi-byte -> byte mode)
		"\x00\x01\x02\x03\xFF", // raw bytes
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, data string) {
		bc, err := NewQR(data)
		if err != nil {
			return
		}
		if bc == nil {
			t.Fatalf("NewQR(%q): got nil barcode with nil error", data)
		}
		w := bc.Width()
		h := bc.Height()
		// QR codes are square: versions 1-40 produce 21-177 modules
		// per side. A non-square result indicates a row/column mixup.
		if w != h {
			t.Errorf("NewQR(%q): non-square grid %dx%d", data, w, h)
		}
		if w < 21 || w > 177 {
			t.Errorf("NewQR(%q): module count %d outside v1-v40 range", data, w)
		}
	})
}

// FuzzNewQRWithECC covers the three non-default ECC levels so a bug
// that only manifests at level L, Q, or H gets exercised.
func FuzzNewQRWithECC(f *testing.F) {
	seeds := []struct {
		data  string
		level ECCLevel
	}{
		{"", ECCLevelL},
		{"ABC", ECCLevelL},
		{"ABC", ECCLevelQ},
		{"ABC", ECCLevelH},
		{"01234567890123456789", ECCLevelH},
	}
	for _, s := range seeds {
		f.Add(s.data, int(s.level))
	}

	f.Fuzz(func(t *testing.T, data string, levelInt int) {
		// Clamp to the 4 defined levels; any other value must be
		// treated as invalid by the encoder.
		if levelInt < int(ECCLevelL) || levelInt > int(ECCLevelH) {
			return
		}
		bc, err := NewQRWithECC(data, ECCLevel(levelInt))
		if err != nil {
			return
		}
		if bc == nil {
			t.Fatalf("NewQRWithECC(%q, %d): nil barcode with nil error", data, levelInt)
		}
		if bc.Width() != bc.Height() {
			t.Errorf("NewQRWithECC(%q, %d): non-square %dx%d",
				data, levelInt, bc.Width(), bc.Height())
		}
	})
}
