// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	goimage "image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"golang.org/x/image/tiff"
)

// Fuzz seeds are generated once at fuzz-target setup time via the
// fuzzSeed* helpers below. Each helper produces a minimal but non-empty,
// format-valid encoding so the fuzzer starts from real structured input
// rather than only empty bytes or a magic-number header.

func fuzzSeedJPEG() []byte {
	img := goimage.NewRGBA(goimage.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 0, color.RGBA{G: 255, A: 255})
	img.Set(0, 1, color.RGBA{B: 255, A: 255})
	img.Set(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 75}); err != nil {
		return nil
	}
	return buf.Bytes()
}

func fuzzSeedPNGOpaque() []byte {
	img := goimage.NewRGBA(goimage.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 1, color.RGBA{G: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func fuzzSeedPNGAlpha() []byte {
	img := goimage.NewNRGBA(goimage.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 255, G: 0, B: 0, A: 128})
	img.SetNRGBA(1, 1, color.NRGBA{R: 0, G: 255, B: 0, A: 64})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func fuzzSeedTIFF() []byte {
	img := goimage.NewRGBA(goimage.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 1, color.RGBA{B: 255, A: 255})
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, img, nil); err != nil {
		return nil
	}
	return buf.Bytes()
}

func fuzzSeedGIF() []byte {
	img := goimage.NewPaletted(goimage.Rect(0, 0, 2, 2), palette.Plan9)
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	img.Set(1, 1, color.RGBA{G: 255, A: 255})
	var buf bytes.Buffer
	if err := gif.Encode(&buf, img, nil); err != nil {
		return nil
	}
	return buf.Bytes()
}

func FuzzNewJPEG(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0xFF, 0xD8, 0xFF, 0xE0})
	if seed := fuzzSeedJPEG(); len(seed) > 0 {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("NewJPEG panicked: %v", r)
			}
		}()
		// Errors are expected for random input; only panics are failures.
		_, _ = NewJPEG(data)
	})
}

func FuzzNewPNG(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
	if seed := fuzzSeedPNGOpaque(); len(seed) > 0 {
		f.Add(seed)
	}
	if seed := fuzzSeedPNGAlpha(); len(seed) > 0 {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("NewPNG panicked: %v", r)
			}
		}()
		// Errors are expected for random input; only panics are failures.
		_, _ = NewPNG(data)
	})
}
