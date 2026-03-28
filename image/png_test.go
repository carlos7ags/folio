// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	goimage "image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// createTestPNG generates a small PNG image in memory.
func createTestPNG(t *testing.T, w, h int, withAlpha bool) []byte {
	t.Helper()
	var img goimage.Image
	if withAlpha {
		rgba := goimage.NewNRGBA(goimage.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				rgba.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 255, A: 128})
			}
		}
		img = rgba
	} else {
		rgb := goimage.NewRGBA(goimage.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				rgb.Set(x, y, color.RGBA{R: 0, G: 255, B: 0, A: 255})
			}
		}
		img = rgb
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestNewPNG(t *testing.T) {
	data := createTestPNG(t, 80, 60, false)
	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG failed: %v", err)
	}
	if img.Width() != 80 {
		t.Errorf("expected width 80, got %d", img.Width())
	}
	if img.Height() != 60 {
		t.Errorf("expected height 60, got %d", img.Height())
	}
}

func TestNewPNGColorSpace(t *testing.T) {
	data := createTestPNG(t, 10, 10, false)
	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG failed: %v", err)
	}
	if img.colorSpace != "DeviceRGB" {
		t.Errorf("expected DeviceRGB, got %s", img.colorSpace)
	}
}

func TestNewPNGWithAlpha(t *testing.T) {
	data := createTestPNG(t, 10, 10, true)
	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG failed: %v", err)
	}
	if len(img.smask) == 0 {
		t.Error("expected SMask for PNG with alpha")
	}
	if img.smaskW != 10 || img.smaskH != 10 {
		t.Errorf("expected smask 10x10, got %dx%d", img.smaskW, img.smaskH)
	}
}

func TestNewPNGNoAlpha(t *testing.T) {
	data := createTestPNG(t, 10, 10, false)
	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG failed: %v", err)
	}
	if len(img.smask) != 0 {
		t.Error("expected no SMask for opaque PNG")
	}
}

func TestNewPNGGrayscale(t *testing.T) {
	gray := goimage.NewGray(goimage.Rect(0, 0, 20, 20))
	for y := range 20 {
		for x := range 20 {
			gray.SetGray(x, y, color.Gray{Y: 200})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, gray); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}

	img, err := NewPNG(buf.Bytes())
	if err != nil {
		t.Fatalf("NewPNG failed: %v", err)
	}
	if img.colorSpace != "DeviceGray" {
		t.Errorf("expected DeviceGray, got %s", img.colorSpace)
	}
}

func TestNewPNGInvalid(t *testing.T) {
	_, err := NewPNG([]byte{0, 1, 2, 3})
	if err == nil {
		t.Error("expected error for invalid PNG data")
	}
}

func TestPNGFilter(t *testing.T) {
	data := createTestPNG(t, 10, 10, false)
	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG failed: %v", err)
	}
	if img.filter != "FlateDecode" {
		t.Errorf("expected FlateDecode, got %s", img.filter)
	}
}

func TestPNGAspectRatio(t *testing.T) {
	data := createTestPNG(t, 200, 100, false)
	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG failed: %v", err)
	}
	if img.AspectRatio() != 2.0 {
		t.Errorf("expected aspect ratio 2.0, got %f", img.AspectRatio())
	}
}

func TestPNGAspectRatioSquare(t *testing.T) {
	data := createTestPNG(t, 50, 50, false)
	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG failed: %v", err)
	}
	if img.AspectRatio() != 1.0 {
		t.Errorf("expected aspect ratio 1.0, got %f", img.AspectRatio())
	}
}

func TestAspectRatioZeroHeight(t *testing.T) {
	// Edge case: zero height should return 1 (not panic with division by zero).
	img := &Image{width: 100, height: 0}
	if img.AspectRatio() != 1.0 {
		t.Errorf("expected aspect ratio 1.0 for zero height, got %f", img.AspectRatio())
	}
}

func TestLoadPNG(t *testing.T) {
	data := createTestPNG(t, 30, 20, false)
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.png")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	img, err := LoadPNG(path)
	if err != nil {
		t.Fatalf("LoadPNG failed: %v", err)
	}
	if img.Width() != 30 {
		t.Errorf("expected width 30, got %d", img.Width())
	}
	if img.Height() != 20 {
		t.Errorf("expected height 20, got %d", img.Height())
	}
}

func TestLoadPNGNotFound(t *testing.T) {
	_, err := LoadPNG("/nonexistent/path/test.png")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestPNGBuildXObject(t *testing.T) {
	data := createTestPNG(t, 15, 10, false)
	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG failed: %v", err)
	}

	objCount := 0
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		objCount++
		return &core.PdfIndirectReference{ObjectNumber: objCount, GenerationNumber: 0}
	}

	imgRef, smaskRef := img.BuildXObject(addObject)
	if imgRef == nil {
		t.Fatal("expected non-nil image reference")
	}
	if smaskRef != nil {
		t.Error("expected nil SMask reference for opaque PNG")
	}
	if objCount != 1 {
		t.Errorf("expected 1 object added, got %d", objCount)
	}
}

func TestPNGBuildXObjectWithAlpha(t *testing.T) {
	data := createTestPNG(t, 15, 10, true)
	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG failed: %v", err)
	}

	objCount := 0
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		objCount++
		return &core.PdfIndirectReference{ObjectNumber: objCount, GenerationNumber: 0}
	}

	imgRef, smaskRef := img.BuildXObject(addObject)
	if imgRef == nil {
		t.Fatal("expected non-nil image reference")
	}
	if smaskRef == nil {
		t.Fatal("expected non-nil SMask reference for PNG with alpha")
	}
	// SMask should be added first (object 1), then the image (object 2).
	if smaskRef.ObjectNumber != 1 {
		t.Errorf("expected SMask object number 1, got %d", smaskRef.ObjectNumber)
	}
	if imgRef.ObjectNumber != 2 {
		t.Errorf("expected image object number 2, got %d", imgRef.ObjectNumber)
	}
	if objCount != 2 {
		t.Errorf("expected 2 objects added, got %d", objCount)
	}
}

func TestPNGAlphaStraightColor(t *testing.T) {
	// Create a 2x2 NRGBA image with semi-transparent red.
	// Straight alpha: R=255, G=0, B=0, A=128.
	// The PDF RGB data must contain [255, 0, 0] (non-premultiplied),
	// NOT [128, 0, 0] (which would be the premultiplied value).
	src := goimage.NewNRGBA(goimage.Rect(0, 0, 2, 2))
	for y := range 2 {
		for x := range 2 {
			src.SetNRGBA(x, y, color.NRGBA{R: 255, G: 0, B: 0, A: 128})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}

	img, err := NewPNG(buf.Bytes())
	if err != nil {
		t.Fatalf("NewPNG: %v", err)
	}

	// Check RGB bytes: each pixel should be [255, 0, 0].
	if len(img.data) != 2*2*3 {
		t.Fatalf("expected %d RGB bytes, got %d", 2*2*3, len(img.data))
	}
	for i := 0; i < len(img.data); i += 3 {
		r, g, b := img.data[i], img.data[i+1], img.data[i+2]
		if r != 255 || g != 0 || b != 0 {
			t.Errorf("pixel %d: expected RGB [255, 0, 0], got [%d, %d, %d]", i/3, r, g, b)
		}
	}

	// Check alpha bytes: each should be 128.
	if len(img.smask) != 2*2 {
		t.Fatalf("expected %d alpha bytes, got %d", 2*2, len(img.smask))
	}
	for i, a := range img.smask {
		if a != 128 {
			t.Errorf("alpha pixel %d: expected 128, got %d", i, a)
		}
	}
}

func TestNewFromGoImageAlphaStraight(t *testing.T) {
	// image.RGBA stores premultiplied values.
	// Semi-transparent red: premultiplied R=128 with A=128 means straight R=255.
	src := goimage.NewRGBA(goimage.Rect(0, 0, 1, 1))
	src.SetRGBA(0, 0, color.RGBA{R: 128, G: 0, B: 0, A: 128})

	img := NewFromGoImage(src)
	if img == nil {
		t.Fatal("NewFromGoImage returned nil")
	}

	// RGB data should be un-premultiplied: R = 128 * 255 / 128 = 255.
	if len(img.data) != 3 {
		t.Fatalf("expected 3 RGB bytes, got %d", len(img.data))
	}
	if img.data[0] != 255 {
		t.Errorf("expected R=255 (un-premultiplied), got %d", img.data[0])
	}
	if img.data[1] != 0 {
		t.Errorf("expected G=0, got %d", img.data[1])
	}
	if img.data[2] != 0 {
		t.Errorf("expected B=0, got %d", img.data[2])
	}

	// Alpha should be 128.
	if len(img.smask) != 1 || img.smask[0] != 128 {
		t.Errorf("expected alpha=128, got %v", img.smask)
	}
}

func TestPNGBuildXObjectGrayscale(t *testing.T) {
	gray := goimage.NewGray(goimage.Rect(0, 0, 10, 10))
	for y := range 10 {
		for x := range 10 {
			gray.SetGray(x, y, color.Gray{Y: 200})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, gray); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}

	img, err := NewPNG(buf.Bytes())
	if err != nil {
		t.Fatalf("NewPNG failed: %v", err)
	}

	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		return &core.PdfIndirectReference{ObjectNumber: 1, GenerationNumber: 0}
	}

	imgRef, smaskRef := img.BuildXObject(addObject)
	if imgRef == nil {
		t.Fatal("expected non-nil image reference")
	}
	if smaskRef != nil {
		t.Error("expected nil SMask for grayscale PNG")
	}
}

func TestNewFromGoImageNil(t *testing.T) {
	img := NewFromGoImage(nil)
	if img != nil {
		t.Error("expected nil for nil input")
	}
}

func TestNewFromGoImageZeroSize(t *testing.T) {
	rgba := goimage.NewRGBA(goimage.Rect(0, 0, 0, 0))
	img := NewFromGoImage(rgba)
	if img != nil {
		t.Error("expected nil for zero-size image")
	}
}

func TestNewFromGoImageZeroWidth(t *testing.T) {
	rgba := goimage.NewRGBA(goimage.Rect(0, 0, 0, 10))
	img := NewFromGoImage(rgba)
	if img != nil {
		t.Error("expected nil for zero-width image")
	}
}

func TestNewFromGoImageZeroHeight(t *testing.T) {
	rgba := goimage.NewRGBA(goimage.Rect(0, 0, 10, 0))
	img := NewFromGoImage(rgba)
	if img != nil {
		t.Error("expected nil for zero-height image")
	}
}

func TestNewFromGoImageValid(t *testing.T) {
	rgba := goimage.NewRGBA(goimage.Rect(0, 0, 2, 2))
	// Set some pixels to non-opaque to trigger alpha.
	rgba.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 128})
	rgba.SetRGBA(1, 1, color.RGBA{R: 0, G: 255, B: 0, A: 255})

	img := NewFromGoImage(rgba)
	if img == nil {
		t.Fatal("expected non-nil image")
	}
	if img.Width() != 2 || img.Height() != 2 {
		t.Errorf("expected 2x2, got %dx%d", img.Width(), img.Height())
	}
	if len(img.smask) == 0 {
		t.Error("expected smask due to semi-transparent pixel")
	}
}

func TestNewFromGoImageInvalidStride(t *testing.T) {
	// Create an RGBA image then tamper with its Stride to be too small.
	rgba := goimage.NewRGBA(goimage.Rect(0, 0, 10, 10))
	rgba.Stride = 1 // way too small for 10 pixels wide
	img := NewFromGoImage(rgba)
	if img != nil {
		t.Error("expected nil for invalid stride")
	}
}
