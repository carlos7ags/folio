// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import "testing"

func FuzzParseTTF(f *testing.F) {
	// Seed with empty bytes.
	f.Add([]byte{})
	// Seed with the TrueType magic number (scalar type 1).
	f.Add([]byte{0x00, 0x01, 0x00, 0x00})
	// Seed with the "true" tag used by some TrueType fonts.
	f.Add([]byte("true"))
	// Seed with the OpenType magic number.
	f.Add([]byte("OTTO"))

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseTTF panicked: %v", r)
			}
		}()
		// Errors are expected for random input; only panics are failures.
		ParseTTF(data)
	})
}
