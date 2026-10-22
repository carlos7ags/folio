// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	goimage "image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// craftPNGHeader builds a valid 8-byte signature plus an IHDR chunk (with
// correct CRC) declaring the given dimensions. No pixel data follows, so a
// full decode would fail; the header alone is enough for DecodeConfig.
func craftPNGHeader(w, h uint32) []byte {
	sig := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], w)
	binary.BigEndian.PutUint32(ihdr[4:8], h)
	ihdr[8] = 8 // bit depth
	ihdr[9] = 2 // color type: truecolor
	// ihdr[10..12] = 0 (compression, filter, interlace)
	chunk := append([]byte("IHDR"), ihdr...)
	crc := crc32.ChecksumIEEE(chunk)
	out := append([]byte{}, sig...)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(ihdr)))
	out = append(out, lenBuf...)
	out = append(out, chunk...)
	crcBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBuf, crc)
	return append(out, crcBuf...)
}

// TestNewPNGOversizedHeader verifies the pixel-count limit is enforced from
// the header, before the full raster is decoded and allocated.
func TestNewPNGOversizedHeader(t *testing.T) {
	_, err := NewPNG(craftPNGHeader(16000, 16000))
	if !errors.Is(err, ErrPixelCountTooLarge) {
		t.Fatalf("expected ErrPixelCountTooLarge, got %v", err)
	}
}

func TestNewPNGOversizedDimension(t *testing.T) {
	_, err := NewPNG(craftPNGHeader(60000, 10))
	if !errors.Is(err, ErrDimensionTooLarge) {
		t.Fatalf("expected ErrDimensionTooLarge, got %v", err)
	}
}

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
		return core.NewPdfIndirectReference(objCount, 0)
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
		return core.NewPdfIndirectReference(objCount, 0)
	}

	imgRef, smaskRef := img.BuildXObject(addObject)
	if imgRef == nil {
		t.Fatal("expected non-nil image reference")
	}
	if smaskRef == nil {
		t.Fatal("expected non-nil SMask reference for PNG with alpha")
	}
	// SMask should be added first (object 1), then the image (object 2).
	if smaskRef.Num() != 1 {
		t.Errorf("expected SMask object number 1, got %d", smaskRef.Num())
	}
	if imgRef.Num() != 2 {
		t.Errorf("expected image object number 2, got %d", imgRef.Num())
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
		return core.NewPdfIndirectReference(1, 0)
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

// varyingGrayPNG builds an opaque grayscale PNG whose pixel values vary
// enough (gradient plus noise) that the encoder is likely to choose more
// than one row filter.
func varyingGrayPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	src := goimage.NewGray(goimage.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := byte((x*7 + y*13) % 256)
			src.SetGray(x, y, color.Gray{Y: v})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

// varyingRGBPNG builds an opaque truecolor PNG whose pixel values vary
// enough (gradient plus noise) that the encoder is likely to choose more
// than one row filter.
func varyingRGBPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	src := goimage.NewRGBA(goimage.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			src.Set(x, y, color.RGBA{
				R: byte((x*7 + y*3) % 256),
				G: byte((x*11 + y*17) % 256),
				B: byte((x*5 + y*23) % 256),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
	return buf.Bytes()
}

// findIDATOffset returns the file offset of the first byte of the first
// IDAT chunk's payload.
func findIDATOffset(t *testing.T, data []byte) int {
	t.Helper()
	off := 8
	for off+8 <= len(data) {
		length := binary.BigEndian.Uint32(data[off : off+4])
		typeStart := off + 4
		payloadStart := typeStart + 4
		chunkType := string(data[typeStart:payloadStart])
		if chunkType == "IDAT" {
			return payloadStart
		}
		off = payloadStart + int(length) + 4
	}
	t.Fatal("no IDAT chunk found")
	return -1
}

// spliceChunk inserts a well-formed chunk of the given type and payload
// immediately before the first occurrence of beforeType in data.
func spliceChunk(t *testing.T, data []byte, chunkType string, payload []byte, beforeType string) []byte {
	t.Helper()
	off := 8
	for off+8 <= len(data) {
		length := binary.BigEndian.Uint32(data[off : off+4])
		typeStart := off + 4
		payloadStart := typeStart + 4
		ct := string(data[typeStart:payloadStart])
		if ct == beforeType {
			lenBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(lenBuf, uint32(len(payload)))
			chunk := append([]byte(chunkType), payload...)
			crc := crc32.ChecksumIEEE(chunk)
			crcBuf := make([]byte, 4)
			binary.BigEndian.PutUint32(crcBuf, crc)
			out := append([]byte{}, data[:off]...)
			out = append(out, lenBuf...)
			out = append(out, chunk...)
			out = append(out, crcBuf...)
			out = append(out, data[off:]...)
			return out
		}
		off = payloadStart + int(length) + 4
	}
	t.Fatalf("chunk type %s not found", beforeType)
	return nil
}

func TestPNGPassthroughEligibleGray(t *testing.T) {
	data := varyingGrayPNG(t, 12, 9)
	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG: %v", err)
	}
	if !img.preCompressed {
		t.Fatal("expected preCompressed for eligible grayscale PNG")
	}
	if img.predictorColors != 1 {
		t.Errorf("expected predictorColors 1, got %d", img.predictorColors)
	}
	zr, err := zlib.NewReader(bytes.NewReader(img.data))
	if err != nil {
		t.Fatalf("zlib.NewReader: %v", err)
	}
	inflated, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("inflate: %v", err)
	}
	rowBytes := img.width * 1
	if len(inflated) != img.height*(1+rowBytes) {
		t.Fatalf("expected %d inflated bytes, got %d", img.height*(1+rowBytes), len(inflated))
	}
	for y := range img.height {
		f := inflated[y*(1+rowBytes)]
		if f > 4 {
			t.Errorf("row %d: filter byte %d out of range 0..4", y, f)
		}
	}
}

func TestPNGPassthroughEligibleRGB(t *testing.T) {
	data := varyingRGBPNG(t, 12, 9)
	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG: %v", err)
	}
	if !img.preCompressed {
		t.Fatal("expected preCompressed for eligible truecolor PNG")
	}
	if img.predictorColors != 3 {
		t.Errorf("expected predictorColors 3, got %d", img.predictorColors)
	}
	zr, err := zlib.NewReader(bytes.NewReader(img.data))
	if err != nil {
		t.Fatalf("zlib.NewReader: %v", err)
	}
	inflated, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("inflate: %v", err)
	}
	rowBytes := img.width * 3
	if len(inflated) != img.height*(1+rowBytes) {
		t.Fatalf("expected %d inflated bytes, got %d", img.height*(1+rowBytes), len(inflated))
	}
	for y := range img.height {
		f := inflated[y*(1+rowBytes)]
		if f > 4 {
			t.Errorf("row %d: filter byte %d out of range 0..4", y, f)
		}
	}
}

func TestPNGPassthroughXObjectDict(t *testing.T) {
	data := varyingRGBPNG(t, 6, 5)
	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG: %v", err)
	}
	if !img.preCompressed {
		t.Fatal("expected preCompressed image")
	}

	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		return core.NewPdfIndirectReference(1, 0)
	}
	imgRef, _ := img.BuildXObject(addObject)
	if imgRef == nil {
		t.Fatal("expected non-nil image reference")
	}
}

func TestPNGPassthroughFallbackTable(t *testing.T) {
	t.Run("RGBA", func(t *testing.T) {
		src := goimage.NewNRGBA(goimage.Rect(0, 0, 4, 4))
		for y := range 4 {
			for x := range 4 {
				src.SetNRGBA(x, y, color.NRGBA{R: 10, G: 20, B: 30, A: 200})
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
		if img.preCompressed {
			t.Error("expected fallback for RGBA (colour type 6)")
		}
	})

	t.Run("GrayAlpha", func(t *testing.T) {
		grayAlpha := goimage.NewNRGBA(goimage.Rect(0, 0, 4, 4))
		for y := range 4 {
			for x := range 4 {
				grayAlpha.SetNRGBA(x, y, color.NRGBA{R: 50, G: 50, B: 50, A: 100})
			}
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, grayAlpha); err != nil {
			t.Fatalf("png.Encode: %v", err)
		}
		img, err := NewPNG(buf.Bytes())
		if err != nil {
			t.Fatalf("NewPNG: %v", err)
		}
		if img.preCompressed {
			t.Error("expected fallback for non-opaque image")
		}
	})

	t.Run("SixteenBit", func(t *testing.T) {
		src := goimage.NewGray16(goimage.Rect(0, 0, 4, 4))
		for y := range 4 {
			for x := range 4 {
				src.SetGray16(x, y, color.Gray16{Y: uint16((x + y) * 1000)})
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
		if img.preCompressed {
			t.Error("expected fallback for 16-bit PNG")
		}
	})

	t.Run("Interlaced", func(t *testing.T) {
		data := varyingRGBPNG(t, 4, 4)
		off := 8
		length := binary.BigEndian.Uint32(data[off : off+4])
		payloadStart := off + 8
		if string(data[off+4:payloadStart]) != "IHDR" {
			t.Fatal("expected IHDR as first chunk")
		}
		payload := append([]byte{}, data[payloadStart:payloadStart+int(length)]...)
		payload[12] = 1 // interlace method: Adam7
		chunk := append([]byte("IHDR"), payload...)
		crc := crc32.ChecksumIEEE(chunk)
		crcBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(crcBuf, crc)
		out := append([]byte{}, data[:payloadStart]...)
		out = append(out, payload...)
		out = append(out, crcBuf...)
		out = append(out, data[payloadStart+int(length)+4:]...)

		img := pngPassthrough(out)
		if img != nil {
			t.Error("expected nil for interlaced PNG")
		}
	})

	t.Run("TRNS", func(t *testing.T) {
		data := varyingRGBPNG(t, 4, 4)
		spliced := spliceChunk(t, data, "tRNS", []byte{0, 0, 0, 0, 0, 0}, "IDAT")
		img, err := NewPNG(spliced)
		if err != nil {
			t.Fatalf("NewPNG: %v", err)
		}
		if img.preCompressed {
			t.Error("expected fallback for truecolor PNG carrying tRNS")
		}
	})
}

func TestPNGPassthroughCorruptIDATFallsBackToError(t *testing.T) {
	data := varyingRGBPNG(t, 8, 8)
	idatOff := findIDATOffset(t, data)
	corrupt := append([]byte{}, data...)
	corrupt[idatOff+3] ^= 0xFF // flip a bit inside the IDAT payload; CRC now stale

	if _, decodeErr := png.Decode(bytes.NewReader(corrupt)); decodeErr == nil {
		t.Fatal("test setup invalid: expected stdlib decode to fail on corrupted IDAT")
	}

	if _, err := NewPNG(corrupt); err == nil {
		t.Fatal("expected NewPNG to return an error on corrupted IDAT, not a silent passthrough")
	}
}

// buildChunk assembles a length-prefixed, CRC-terminated PNG chunk.
func buildChunk(chunkType string, payload []byte) []byte {
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(payload)))
	body := append([]byte(chunkType), payload...)
	crc := crc32.ChecksumIEEE(body)
	crcBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBuf, crc)
	out := append([]byte{}, lenBuf...)
	out = append(out, body...)
	out = append(out, crcBuf...)
	return out
}

// craftPalettedPNG hand-builds a colour-type-3 PNG at the given bit depth
// with filter type 0 (None) on every scanline, so the caller controls the
// exact packed-index layout.
func craftPalettedPNG(t *testing.T, w, h int, bitDepth int, palette []byte, indices [][]int) []byte {
	t.Helper()
	rowBytes := (w*bitDepth + 7) / 8
	var raw []byte
	for y := range h {
		row := make([]byte, rowBytes)
		for x := range w {
			idx := byte(indices[y][x])
			bitOff := x * bitDepth
			byteOff := bitOff / 8
			shift := 8 - bitDepth - (bitOff % 8)
			row[byteOff] |= idx << shift
		}
		raw = append(raw, 0) // filter type 0
		raw = append(raw, row...)
	}

	var zbuf bytes.Buffer
	zw := zlib.NewWriter(&zbuf)
	if _, err := zw.Write(raw); err != nil {
		t.Fatalf("zlib write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zlib close: %v", err)
	}

	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:4], uint32(w))
	binary.BigEndian.PutUint32(ihdr[4:8], uint32(h))
	ihdr[8] = byte(bitDepth)
	ihdr[9] = 3 // colour type: palette

	out := append([]byte{}, pngSignature[:]...)
	out = append(out, buildChunk("IHDR", ihdr)...)
	out = append(out, buildChunk("PLTE", palette)...)
	out = append(out, buildChunk("IDAT", zbuf.Bytes())...)
	out = append(out, buildChunk("IEND", nil)...)
	return out
}

func TestPNGPassthroughPaletteRouting(t *testing.T) {
	for _, bitDepth := range []int{1, 2, 4, 8} {
		t.Run(fmt.Sprintf("depth%d", bitDepth), func(t *testing.T) {
			maxIdx := 1 << bitDepth
			if maxIdx > 256 {
				maxIdx = 256
			}
			palette := make([]byte, 0, maxIdx*3)
			for i := range maxIdx {
				palette = append(palette, byte(i), byte(i*2), byte(i*3))
			}
			w, h := 6, 5
			indices := make([][]int, h)
			for y := range h {
				indices[y] = make([]int, w)
				for x := range w {
					indices[y][x] = (x + y) % maxIdx
				}
			}
			data := craftPalettedPNG(t, w, h, bitDepth, palette, indices)

			img, err := NewPNG(data)
			if err != nil {
				t.Fatalf("NewPNG: %v", err)
			}
			if !img.preCompressed {
				t.Fatal("expected preCompressed for eligible paletted PNG")
			}
			if img.predictorColors != 1 {
				t.Errorf("expected predictorColors 1, got %d", img.predictorColors)
			}
			if img.bpc != bitDepth {
				t.Errorf("expected bpc %d, got %d", bitDepth, img.bpc)
			}

			zr, err := zlib.NewReader(bytes.NewReader(img.data))
			if err != nil {
				t.Fatalf("zlib.NewReader: %v", err)
			}
			inflated, err := io.ReadAll(zr)
			if err != nil {
				t.Fatalf("inflate: %v", err)
			}
			rowBytes := (w*bitDepth + 7) / 8
			wantLen := h * (1 + rowBytes)
			if len(inflated) != wantLen {
				t.Fatalf("expected %d inflated bytes, got %d", wantLen, len(inflated))
			}
		})
	}
}

func TestPNGPassthroughPaletteXObjectDict(t *testing.T) {
	palette := []byte{255, 0, 0, 0, 255, 0, 0, 0, 255, 128, 128, 128}
	w, h := 4, 4
	indices := make([][]int, h)
	for y := range h {
		indices[y] = make([]int, w)
		for x := range w {
			indices[y][x] = (x + y) % 4
		}
	}
	data := craftPalettedPNG(t, w, h, 8, palette, indices)

	img, err := NewPNG(data)
	if err != nil {
		t.Fatalf("NewPNG: %v", err)
	}
	if !img.preCompressed {
		t.Fatal("expected preCompressed image")
	}
	arr, ok := img.colorSpaceObj.(*core.PdfArray)
	if !ok {
		t.Fatalf("expected colorSpaceObj to be a *core.PdfArray, got %T", img.colorSpaceObj)
	}
	if arr.Len() != 4 {
		t.Fatalf("expected 4-element colour space array, got %d", arr.Len())
	}
	name0, ok := arr.At(0).(*core.PdfName)
	if !ok || name0.Value != "Indexed" {
		t.Errorf("expected element 0 to be /Indexed, got %v", arr.At(0))
	}
	name1, ok := arr.At(1).(*core.PdfName)
	if !ok || name1.Value != "DeviceRGB" {
		t.Errorf("expected element 1 to be /DeviceRGB, got %v", arr.At(1))
	}
	hival, ok := arr.At(2).(*core.PdfNumber)
	if !ok || hival.IntValue() != len(palette)/3-1 {
		t.Errorf("expected hival %d, got %v", len(palette)/3-1, arr.At(2))
	}
	lookup, ok := arr.At(3).(*core.PdfString)
	if !ok || lookup.Text() != string(palette) {
		t.Errorf("expected lookup table to equal PLTE payload")
	}
	if img.bpc != 8 {
		t.Errorf("expected BitsPerComponent 8, got %d", img.bpc)
	}

	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		return core.NewPdfIndirectReference(1, 0)
	}
	imgRef, _ := img.BuildXObject(addObject)
	if imgRef == nil {
		t.Fatal("expected non-nil image reference")
	}
}

func TestPNGPassthroughPaletteFallback(t *testing.T) {
	basePalette := []byte{255, 0, 0, 0, 255, 0, 0, 0, 255, 128, 128, 128}
	w, h := 4, 4
	indices := make([][]int, h)
	for y := range h {
		indices[y] = make([]int, w)
		for x := range w {
			indices[y][x] = (x + y) % 4
		}
	}

	t.Run("TRNS", func(t *testing.T) {
		data := craftPalettedPNG(t, w, h, 8, basePalette, indices)
		spliced := spliceChunk(t, data, "tRNS", []byte{255, 255, 255, 0}, "IDAT")
		img, err := NewPNG(spliced)
		if err != nil {
			t.Fatalf("NewPNG: %v", err)
		}
		if img.preCompressed {
			t.Error("expected fallback for paletted PNG carrying tRNS")
		}
	})

	t.Run("Interlaced", func(t *testing.T) {
		data := craftPalettedPNG(t, w, h, 8, basePalette, indices)
		// IHDR interlace byte is the 13th (last) byte of the 13-byte
		// payload, immediately preceding its CRC.
		ihdrPayloadEnd := 8 + 8 + 13 // signature + (len+type) + payload
		data[ihdrPayloadEnd-1] = 1
		// Recompute the IHDR CRC so the chunk walk's CRC check does not
		// reject the file before eligibility is even considered.
		chunkStart := 8 + 4 // signature + length field
		crc := crc32.ChecksumIEEE(data[chunkStart:ihdrPayloadEnd])
		binary.BigEndian.PutUint32(data[ihdrPayloadEnd:ihdrPayloadEnd+4], crc)

		img := pngPassthrough(data)
		if img != nil {
			t.Error("expected nil for interlaced paletted PNG")
		}
	})

	t.Run("OversizedPLTE", func(t *testing.T) {
		// bitDepth 1 permits at most 2 palette entries; supply 4.
		oversizedPalette := []byte{
			255, 0, 0,
			0, 255, 0,
			0, 0, 255,
			255, 255, 255,
		}
		oneBitIndices := make([][]int, h)
		for y := range h {
			oneBitIndices[y] = make([]int, w)
			for x := range w {
				oneBitIndices[y][x] = (x + y) % 2
			}
		}
		data := craftPalettedPNG(t, w, h, 1, oversizedPalette, oneBitIndices)
		img := pngPassthrough(data)
		if img != nil {
			t.Error("expected nil for PLTE larger than 1<<bitDepth entries")
		}
	})
}
