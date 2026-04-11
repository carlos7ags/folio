// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"errors"
	goimage "image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// --- checkDimensions ---

func TestCheckDimensionsOK(t *testing.T) {
	cases := []struct {
		name string
		w, h int
	}{
		{"1x1", 1, 1},
		{"100x100", 100, 100},
		{"max dimension square", 1000, 1000},
		{"tall but thin", 1, 10000},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := checkDimensions(tt.w, tt.h); err != nil {
				t.Errorf("checkDimensions(%d, %d) returned %v, want nil",
					tt.w, tt.h, err)
			}
		})
	}
}

func TestCheckDimensionsNonPositive(t *testing.T) {
	cases := []struct{ w, h int }{
		{0, 100},
		{100, 0},
		{-1, 100},
		{100, -1},
		{0, 0},
	}
	for _, tt := range cases {
		err := checkDimensions(tt.w, tt.h)
		if !errors.Is(err, ErrDimensionInvalid) {
			t.Errorf("checkDimensions(%d, %d) = %v, want ErrDimensionInvalid",
				tt.w, tt.h, err)
		}
	}
}

func TestCheckDimensionsTooLarge(t *testing.T) {
	err := checkDimensions(MaxDimension+1, 10)
	if !errors.Is(err, ErrDimensionTooLarge) {
		t.Errorf("expected ErrDimensionTooLarge, got %v", err)
	}
	err = checkDimensions(10, MaxDimension+1)
	if !errors.Is(err, ErrDimensionTooLarge) {
		t.Errorf("expected ErrDimensionTooLarge for tall, got %v", err)
	}
}

func TestCheckDimensionsPixelCountOverflow(t *testing.T) {
	// 12000*12000 = 144M > MaxPixels (100M) but each axis is within MaxDimension.
	err := checkDimensions(12000, 12000)
	if !errors.Is(err, ErrPixelCountTooLarge) {
		t.Errorf("expected ErrPixelCountTooLarge, got %v", err)
	}
}

// TestCheckDimensionsBoundaries locks in the exact inclusive/exclusive
// semantics of [checkDimensions]: exactly MaxDimension and exactly
// MaxPixels must pass, one more in any direction must fail.
func TestCheckDimensionsBoundaries(t *testing.T) {
	// Individual axes: exactly MaxDimension passes, +1 fails.
	if err := checkDimensions(MaxDimension, 1); err != nil {
		t.Errorf("checkDimensions(MaxDimension, 1): got %v, want nil", err)
	}
	if err := checkDimensions(1, MaxDimension); err != nil {
		t.Errorf("checkDimensions(1, MaxDimension): got %v, want nil", err)
	}
	if err := checkDimensions(MaxDimension+1, 1); !errors.Is(err, ErrDimensionTooLarge) {
		t.Errorf("MaxDimension+1 width should fail, got %v", err)
	}
	if err := checkDimensions(1, MaxDimension+1); !errors.Is(err, ErrDimensionTooLarge) {
		t.Errorf("MaxDimension+1 height should fail, got %v", err)
	}

	// Pixel product: exactly MaxPixels passes, one more fails.
	// 10000 × 10000 = 100,000,000 = MaxPixels exactly.
	if err := checkDimensions(10000, 10000); err != nil {
		t.Errorf("checkDimensions at exactly MaxPixels: got %v, want nil", err)
	}
	if err := checkDimensions(10001, 10000); !errors.Is(err, ErrPixelCountTooLarge) {
		t.Errorf("one pixel over MaxPixels should fail, got %v", err)
	}
	if err := checkDimensions(10000, 10001); !errors.Is(err, ErrPixelCountTooLarge) {
		t.Errorf("one pixel over MaxPixels (tall) should fail, got %v", err)
	}
}

// --- readLimited / file size enforcement ---

func TestReadLimitedSmallFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.bin")
	content := []byte("hello world")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	data, err := readLimited(path)
	if err != nil {
		t.Fatalf("readLimited: %v", err)
	}
	if !bytes.Equal(data, content) {
		t.Errorf("readLimited returned %q, want %q", data, content)
	}
}

func TestReadLimitedRejectsOversizedFile(t *testing.T) {
	// Write a sparse file bigger than MaxFileSize using Truncate,
	// avoiding a multi-hundred-megabyte on-disk write.
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(MaxFileSize + 1); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = readLimited(path)
	if !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("expected ErrFileTooLarge, got %v", err)
	}
}

// TestLoadRejectsOversizedFile exercises the MaxFileSize guard on every
// Load* constructor, not just LoadJPEG. The file is sparse (Truncate) so
// no real disk space is consumed.
func TestLoadRejectsOversizedFile(t *testing.T) {
	cases := []struct {
		name   string
		ext    string
		loader func(string) (*Image, error)
	}{
		{"jpeg", ".jpg", LoadJPEG},
		{"png", ".png", LoadPNG},
		{"tiff", ".tiff", LoadTIFF},
		{"webp", ".webp", LoadWebP},
		{"gif", ".gif", LoadGIF},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "big"+tc.ext)
			f, err := os.Create(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := f.Truncate(MaxFileSize + 1); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			if _, err := tc.loader(path); !errors.Is(err, ErrFileTooLarge) {
				t.Errorf("Load%s: expected ErrFileTooLarge, got %v", tc.name, err)
			}
		})
	}
}

// --- decompression bombs ---

func TestNewJPEGRejectsBombDimensions(t *testing.T) {
	// Synthetic JPEG with SOF0 declaring a 65535x65535 image. We never
	// decode pixels (JPEG is passthrough), so parseJPEGHeader will
	// succeed but checkDimensions must reject it.
	data := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xC0, // SOF0
		0x00, 0x11, // length = 17
		0x08,       // precision
		0xFF, 0xFF, // height = 65535
		0xFF, 0xFF, // width = 65535
		0x03, // ncomp = 3 (RGB)
		0x01, 0x11, 0x00,
		0x02, 0x11, 0x00,
		0x03, 0x11, 0x00,
	}
	_, err := NewJPEG(data)
	if err == nil {
		t.Fatal("expected error for bomb JPEG, got nil")
	}
	if !errors.Is(err, ErrDimensionTooLarge) && !errors.Is(err, ErrPixelCountTooLarge) {
		t.Errorf("expected dimension/pixel-count error, got %v", err)
	}
}

func TestNewJPEGRejectsZeroDimensions(t *testing.T) {
	// A crafted JPEG with a 0x0 SOF0 should be rejected.
	data := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xC0, // SOF0
		0x00, 0x11, // length = 17
		0x08,       // precision
		0x00, 0x00, // height = 0
		0x00, 0x00, // width = 0
		0x03,
		0x01, 0x11, 0x00,
		0x02, 0x11, 0x00,
		0x03, 0x11, 0x00,
	}
	_, err := NewJPEG(data)
	if !errors.Is(err, ErrDimensionInvalid) {
		t.Errorf("expected ErrDimensionInvalid, got %v", err)
	}
}

// TestParseJPEGHeaderTruncatedSOF is a regression test for a one-off
// index check in parseJPEGHeader: the bounds test allowed pos+7 == len(data)
// but the code then accessed data[pos+7], which is out of range. Found by
// FuzzNewJPEG after its seed corpus was strengthened in this PR.
func TestParseJPEGHeaderTruncatedSOF(t *testing.T) {
	// SOI + SOF2 marker + 7 bytes (length field + precision + height +
	// width but no ncomp byte). parseJPEGHeader must return an error,
	// not panic.
	data := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xC2, // SOF2
		0x00, 0x11, // length = 17
		0x08,                   // precision
		0x00, 0x01, 0x00, 0x01, // height=1, width=1 — but no ncomp byte
	}
	// Must not panic.
	_, _, _, _, err := parseJPEGHeader(data)
	if err == nil {
		t.Error("expected error for truncated SOF segment, got nil")
	}
}

func TestParseJPEGHeaderSegmentCap(t *testing.T) {
	// Craft a JPEG with many trivial APP0 segments but no SOF. The
	// parser should bail after maxJPEGSegments rather than walking the
	// whole file.
	var buf bytes.Buffer
	buf.Write([]byte{0xFF, 0xD8}) // SOI
	// Write maxJPEGSegments+1 APP0 segments, each with length 4
	// (length field + 2 data bytes).
	app0 := []byte{0xFF, 0xE0, 0x00, 0x04, 0x00, 0x00}
	for range maxJPEGSegments + 10 {
		buf.Write(app0)
	}
	_, _, _, _, err := parseJPEGHeader(buf.Bytes())
	if err == nil || !strings.Contains(err.Error(), "too many segments") {
		t.Errorf("expected 'too many segments' error, got %v", err)
	}
}

// --- buildRGBMaybeAlpha: each switch case exercised with position-
//     specific pixel assertions so that swaps, colour channel mistakes,
//     or un-premultiplication bugs would actually be caught. ---

// pixelAt returns the RGB triplet and alpha byte at (x, y) in the flat
// data/smask buffers produced by buildRGBMaybeAlpha. Tests use this to
// express expectations as "pixel (0,0) is [255,0,0] with alpha 128".
func pixelAt(img *Image, x, y int) (r, g, b, a byte) {
	off := (y*img.width + x) * 3
	r, g, b = img.data[off], img.data[off+1], img.data[off+2]
	if len(img.smask) > 0 {
		a = img.smask[y*img.smaskW+x]
	} else {
		a = 255
	}
	return
}

func assertPixel(t *testing.T, img *Image, x, y int, wantR, wantG, wantB, wantA byte) {
	t.Helper()
	r, g, b, a := pixelAt(img, x, y)
	if r != wantR || g != wantG || b != wantB || a != wantA {
		t.Errorf("pixel (%d,%d): got RGBA [%d,%d,%d,%d], want [%d,%d,%d,%d]",
			x, y, r, g, b, a, wantR, wantG, wantB, wantA)
	}
}

// TestBuildRGBMaybeAlphaGenericPath feeds a 2x2 Paletted image through
// the generic default branch of buildRGBMaybeAlpha and verifies every
// pixel's RGB triplet AND alpha at its position. The previous version of
// this test only asserted that "alpha 128 and 255 both appear somewhere",
// which would have passed even if RGB channels were swapped or zeroed.
func TestBuildRGBMaybeAlphaGenericPath(t *testing.T) {
	// Palette: index 0 = unused, 1 = semi-transparent red, 2 = opaque
	// green, 3 = opaque blue.
	pal := color.Palette{
		color.NRGBA{0, 0, 0, 255},
		color.NRGBA{255, 0, 0, 128},
		color.NRGBA{0, 255, 0, 255},
		color.NRGBA{0, 0, 255, 255},
	}
	img := goimage.NewPaletted(goimage.Rect(0, 0, 2, 2), pal)
	img.SetColorIndex(0, 0, 1) // semi-transparent red
	img.SetColorIndex(1, 0, 2) // opaque green
	img.SetColorIndex(0, 1, 3) // opaque blue
	img.SetColorIndex(1, 1, 2) // opaque green

	out, err := buildRGBMaybeAlpha(img, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if out.colorSpace != "DeviceRGB" {
		t.Errorf("colorSpace = %q, want DeviceRGB", out.colorSpace)
	}
	if len(out.data) != 2*2*3 {
		t.Fatalf("data len = %d, want %d", len(out.data), 2*2*3)
	}
	if len(out.smask) != 2*2 {
		t.Fatalf("smask len = %d, want %d", len(out.smask), 2*2)
	}

	// Position-specific checks: each pixel's RGB and alpha.
	assertPixel(t, out, 0, 0, 255, 0, 0, 128) // semi-transparent red
	assertPixel(t, out, 1, 0, 0, 255, 0, 255) // opaque green
	assertPixel(t, out, 0, 1, 0, 0, 255, 255) // opaque blue
	assertPixel(t, out, 1, 1, 0, 255, 0, 255) // opaque green
}

// TestBuildRGBMaybeAlphaNRGBAFastPath exercises the *goimage.NRGBA
// branch of the type switch with position-specific assertions. NRGBA
// stores straight alpha, so RGB bytes should pass through unchanged.
func TestBuildRGBMaybeAlphaNRGBAFastPath(t *testing.T) {
	img := goimage.NewNRGBA(goimage.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 200, G: 100, B: 50, A: 200})
	img.SetNRGBA(1, 0, color.NRGBA{R: 0, G: 0, B: 0, A: 255})
	img.SetNRGBA(0, 1, color.NRGBA{R: 255, G: 255, B: 255, A: 0})
	img.SetNRGBA(1, 1, color.NRGBA{R: 128, G: 64, B: 32, A: 128})

	out, err := buildRGBMaybeAlpha(img, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.smask) == 0 {
		t.Fatal("expected smask for partially transparent NRGBA input")
	}
	assertPixel(t, out, 0, 0, 200, 100, 50, 200)
	assertPixel(t, out, 1, 0, 0, 0, 0, 255)
	// NRGBA stores straight values, so even fully-transparent pixels
	// keep their stored RGB bytes.
	assertPixel(t, out, 0, 1, 255, 255, 255, 0)
	assertPixel(t, out, 1, 1, 128, 64, 32, 128)
}

// TestBuildRGBMaybeAlphaRGBAFastPath exercises the *goimage.RGBA branch
// of the type switch. RGBA stores premultiplied values, so the decoder
// must un-premultiply to recover straight alpha for PDF output. This
// branch was completely untested before this follow-up: PNG decode
// returns NRGBA for alpha PNGs, so in practice this code path only
// runs if a caller constructs an *goimage.RGBA directly.
func TestBuildRGBMaybeAlphaRGBAFastPath(t *testing.T) {
	// Semi-transparent red at A=128: premultiplied R = round(255*128/255)
	// = 128. Decoder should un-premultiply back to straight R=255.
	img := goimage.NewRGBA(goimage.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{R: 128, G: 0, B: 0, A: 128})  // → straight [255,0,0,128]
	img.SetRGBA(1, 0, color.RGBA{R: 0, G: 255, B: 0, A: 255})  // fully opaque
	img.SetRGBA(0, 1, color.RGBA{R: 0, G: 0, B: 0, A: 0})      // fully transparent
	img.SetRGBA(1, 1, color.RGBA{R: 64, G: 32, B: 16, A: 128}) // → straight [127,63,31,128]

	out, err := buildRGBMaybeAlpha(img, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.smask) == 0 {
		t.Fatal("expected smask for partially transparent RGBA input")
	}
	assertPixel(t, out, 0, 0, 255, 0, 0, 128)
	assertPixel(t, out, 1, 0, 0, 255, 0, 255)
	// Fully transparent pixels are zeroed by the decoder.
	assertPixel(t, out, 0, 1, 0, 0, 0, 0)
	// Un-premultiplication uses integer division truncation:
	// 64 * 255 / 128 = 127, 32 * 255 / 128 = 63, 16 * 255 / 128 = 31.
	assertPixel(t, out, 1, 1, 127, 63, 31, 128)
}

// TestBuildRGBMaybeAlphaAllOpaqueDropsSMask confirms that when every
// pixel is opaque, the single-pass path discards the transient alpha
// buffer and produces an Image with no soft mask.
func TestBuildRGBMaybeAlphaAllOpaqueDropsSMask(t *testing.T) {
	img := goimage.NewNRGBA(goimage.Rect(0, 0, 3, 2))
	for y := range 2 {
		for x := range 3 {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 80), G: uint8(y * 120), B: 0, A: 255})
		}
	}
	out, err := buildRGBMaybeAlpha(img, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	if out.smask != nil || out.smaskW != 0 || out.smaskH != 0 {
		t.Errorf("opaque image should produce no smask, got smask len=%d smaskW=%d smaskH=%d",
			len(out.smask), out.smaskW, out.smaskH)
	}
	// Position-specific RGB spot-check.
	assertPixel(t, out, 0, 0, 0, 0, 0, 255)
	assertPixel(t, out, 1, 0, 80, 0, 0, 255)
	assertPixel(t, out, 2, 1, 160, 120, 0, 255)
}

// --- fuzz targets (TIFF, WebP, GIF; JPEG/PNG are in fuzz_test.go) ---
//
// Each target seeds with three inputs: empty bytes, a format magic
// header, and a minimal but valid encoded image. The valid seed gives
// the fuzzer a realistic starting point whose mutations are much more
// likely to exercise decoder internals than magic-header-only seeds.

func FuzzNewTIFF(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0x49, 0x49, 0x2A, 0x00}) // TIFF little-endian magic
	if seed := fuzzSeedTIFF(); len(seed) > 0 {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("NewTIFF panicked: %v", r)
			}
		}()
		_, _ = NewTIFF(data)
	})
}

func FuzzNewWebP(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("RIFF\x00\x00\x00\x00WEBP"))
	// minimalWebP is defined in webp_test.go and is a valid 1x1 VP8L
	// lossless WebP — a richer starting point than a bare RIFF header.
	f.Add(minimalWebP)
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("NewWebP panicked: %v", r)
			}
		}()
		_, _ = NewWebP(data)
	})
}

func FuzzNewGIF(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte("GIF89a"))
	if seed := fuzzSeedGIF(); len(seed) > 0 {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("NewGIF panicked: %v", r)
			}
		}()
		_, _ = NewGIF(data)
	})
}

// --- NewFromGoImage dimension limit ---

func TestNewFromGoImageRejectsTooLarge(t *testing.T) {
	// Can't actually allocate a (MaxDimension+1)-sized image.RGBA because
	// it would consume ~1 GB. Instead, construct a small RGBA and mutate
	// the Rect bounds so the implicit Dx/Dy exceed the limit. Stride is
	// then too small for those bounds and we'd hit stride rejection,
	// which is also a valid reject path. To specifically test the
	// dimension path, use an RGBA with exactly MaxDimension+1 x 1 which
	// needs (MaxDimension+1)*4 bytes of Pix — fits comfortably.
	w, h := MaxDimension+1, 1
	rgba := &goimage.RGBA{
		Pix:    make([]byte, w*h*4),
		Stride: w * 4,
		Rect:   goimage.Rect(0, 0, w, h),
	}
	if got := NewFromGoImage(rgba); got != nil {
		t.Error("expected nil for oversized image, got non-nil")
	}
}

// TestRoundTripPNGBuildXObjectOpaque decodes a small opaque PNG and then
// calls BuildXObject, asserting the structure of the resulting indirect
// objects. This replaces a prior version of the test that was named
// "RoundTripPNGBuildXObject" but never actually called BuildXObject.
func TestRoundTripPNGBuildXObjectOpaque(t *testing.T) {
	const w, h = 7, 11
	rgba := goimage.NewRGBA(goimage.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			rgba.SetRGBA(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err != nil {
		t.Fatal(err)
	}
	img, err := NewPNG(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	var added []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		added = append(added, obj)
		return core.NewPdfIndirectReference(len(added), 0)
	}

	imgRef, smaskRef := img.BuildXObject(addObject)
	if imgRef == nil {
		t.Fatal("expected non-nil image reference")
	}
	if smaskRef != nil {
		t.Errorf("opaque PNG should not produce an SMask, got %v", smaskRef)
	}
	if len(added) != 1 {
		t.Fatalf("expected 1 object added, got %d", len(added))
	}
	stream, ok := added[0].(*core.PdfStream)
	if !ok {
		t.Fatalf("expected *core.PdfStream, got %T", added[0])
	}
	assertStreamDictEntry(t, stream.Dict, "Type", "XObject")
	assertStreamDictEntry(t, stream.Dict, "Subtype", "Image")
	assertStreamDictEntry(t, stream.Dict, "ColorSpace", "DeviceRGB")
	assertStreamDictInt(t, stream.Dict, "Width", w)
	assertStreamDictInt(t, stream.Dict, "Height", h)
	assertStreamDictInt(t, stream.Dict, "BitsPerComponent", 8)
}

// TestRoundTripPNGBuildXObjectAlpha decodes a semi-transparent PNG and
// verifies that BuildXObject produces the expected image + SMask pair
// with correct ordering and dict entries.
func TestRoundTripPNGBuildXObjectAlpha(t *testing.T) {
	const w, h = 4, 3
	src := goimage.NewNRGBA(goimage.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			src.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 255, A: 128})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	img, err := NewPNG(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	var added []core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		added = append(added, obj)
		return core.NewPdfIndirectReference(len(added), 0)
	}

	imgRef, smaskRef := img.BuildXObject(addObject)
	if imgRef == nil || smaskRef == nil {
		t.Fatalf("expected both image and smask refs, got img=%v smask=%v", imgRef, smaskRef)
	}
	if len(added) != 2 {
		t.Fatalf("expected 2 objects added, got %d", len(added))
	}
	// SMask must be added first so its reference is available when the
	// main image stream's dict is built.
	if smaskRef.Num() != 1 {
		t.Errorf("SMask should be object 1, got %d", smaskRef.Num())
	}
	if imgRef.Num() != 2 {
		t.Errorf("Image should be object 2, got %d", imgRef.Num())
	}

	smaskStream := added[0].(*core.PdfStream)
	assertStreamDictEntry(t, smaskStream.Dict, "Type", "XObject")
	assertStreamDictEntry(t, smaskStream.Dict, "Subtype", "Image")
	assertStreamDictEntry(t, smaskStream.Dict, "ColorSpace", "DeviceGray")
	assertStreamDictInt(t, smaskStream.Dict, "Width", w)
	assertStreamDictInt(t, smaskStream.Dict, "Height", h)
	assertStreamDictInt(t, smaskStream.Dict, "BitsPerComponent", 8)

	imgStream := added[1].(*core.PdfStream)
	assertStreamDictEntry(t, imgStream.Dict, "ColorSpace", "DeviceRGB")
	assertStreamDictInt(t, imgStream.Dict, "Width", w)
	assertStreamDictInt(t, imgStream.Dict, "Height", h)
	// Image dict should reference the SMask by indirect reference.
	smaskEntry := imgStream.Dict.Get("SMask")
	if smaskEntry == nil {
		t.Fatal("image stream dict missing /SMask entry")
	}
	smaskRefActual, ok := smaskEntry.(*core.PdfIndirectReference)
	if !ok {
		t.Fatalf("/SMask should be *PdfIndirectReference, got %T", smaskEntry)
	}
	if smaskRefActual.Num() != smaskRef.Num() {
		t.Errorf("/SMask ref = %d, want %d", smaskRefActual.Num(), smaskRef.Num())
	}
}

// --- Small helpers for the BuildXObject round-trip tests. ---

func assertStreamDictEntry(t *testing.T, dict *core.PdfDictionary, key, wantName string) {
	t.Helper()
	obj := dict.Get(key)
	if obj == nil {
		t.Errorf("dict missing /%s", key)
		return
	}
	name, ok := obj.(*core.PdfName)
	if !ok {
		t.Errorf("/%s: expected *PdfName, got %T", key, obj)
		return
	}
	if name.Value != wantName {
		t.Errorf("/%s = %q, want %q", key, name.Value, wantName)
	}
}

func assertStreamDictInt(t *testing.T, dict *core.PdfDictionary, key string, want int) {
	t.Helper()
	obj := dict.Get(key)
	if obj == nil {
		t.Errorf("dict missing /%s", key)
		return
	}
	num, ok := obj.(*core.PdfNumber)
	if !ok {
		t.Errorf("/%s: expected *PdfNumber, got %T", key, obj)
		return
	}
	if got := num.IntValue(); got != want {
		t.Errorf("/%s = %d, want %d", key, got, want)
	}
}
