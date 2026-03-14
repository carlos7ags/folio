// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import "testing"

func FuzzNewJPEG(f *testing.F) {
	// Seed with empty bytes.
	f.Add([]byte{})
	// Seed with the JPEG SOI marker.
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE0})

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("NewJPEG panicked: %v", r)
			}
		}()
		// Errors are expected for random input; only panics are failures.
		NewJPEG(data)
	})
}

func FuzzNewPNG(f *testing.F) {
	// Seed with empty bytes.
	f.Add([]byte{})
	// Seed with the PNG magic header.
	f.Add([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("NewPNG panicked: %v", r)
			}
		}()
		// Errors are expected for random input; only panics are failures.
		NewPNG(data)
	})
}
