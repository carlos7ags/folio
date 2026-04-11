// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	goimage "image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// createTestJPEG generates a small JPEG image in memory.
func createTestJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := goimage.NewRGBA(goimage.Rect(0, 0, w, h))
	// Fill with red.
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestNewJPEG(t *testing.T) {
	data := createTestJPEG(t, 100, 50)
	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG failed: %v", err)
	}
	if img.Width() != 100 {
		t.Errorf("expected width 100, got %d", img.Width())
	}
	if img.Height() != 50 {
		t.Errorf("expected height 50, got %d", img.Height())
	}
}

func TestNewJPEGColorSpace(t *testing.T) {
	data := createTestJPEG(t, 10, 10)
	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG failed: %v", err)
	}
	if img.colorSpace != "DeviceRGB" {
		t.Errorf("expected DeviceRGB, got %s", img.colorSpace)
	}
}

func TestNewJPEGGrayscale(t *testing.T) {
	// Create a grayscale JPEG.
	gray := goimage.NewGray(goimage.Rect(0, 0, 20, 20))
	for y := range 20 {
		for x := range 20 {
			gray.SetGray(x, y, color.Gray{Y: 128})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, gray, nil); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}

	img, err := NewJPEG(buf.Bytes())
	if err != nil {
		t.Fatalf("NewJPEG failed: %v", err)
	}
	if img.colorSpace != "DeviceGray" {
		t.Errorf("expected DeviceGray, got %s", img.colorSpace)
	}
}

func TestNewJPEGInvalid(t *testing.T) {
	_, err := NewJPEG([]byte{0, 1, 2, 3})
	if err == nil {
		t.Error("expected error for invalid JPEG data")
	}
}

func TestNewJPEGTruncated(t *testing.T) {
	data := createTestJPEG(t, 10, 10)
	_, err := NewJPEG(data[:20]) // truncated
	if err == nil {
		t.Error("expected error for truncated JPEG")
	}
}

func TestJPEGAspectRatio(t *testing.T) {
	data := createTestJPEG(t, 200, 100)
	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG failed: %v", err)
	}
	if img.AspectRatio() != 2.0 {
		t.Errorf("expected aspect ratio 2.0, got %f", img.AspectRatio())
	}
}

func TestJPEGFilter(t *testing.T) {
	data := createTestJPEG(t, 10, 10)
	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG failed: %v", err)
	}
	if img.filter != "DCTDecode" {
		t.Errorf("expected DCTDecode, got %s", img.filter)
	}
}

func TestLoadJPEG(t *testing.T) {
	data := createTestJPEG(t, 40, 30)
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.jpg")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	img, err := LoadJPEG(path)
	if err != nil {
		t.Fatalf("LoadJPEG failed: %v", err)
	}
	if img.Width() != 40 {
		t.Errorf("expected width 40, got %d", img.Width())
	}
	if img.Height() != 30 {
		t.Errorf("expected height 30, got %d", img.Height())
	}
}

func TestLoadJPEGNotFound(t *testing.T) {
	_, err := LoadJPEG("/nonexistent/path/test.jpg")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestJPEGBuildXObject(t *testing.T) {
	data := createTestJPEG(t, 20, 10)
	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG failed: %v", err)
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
	if imgRef.Num() != 1 {
		t.Errorf("expected object number 1, got %d", imgRef.Num())
	}
	if smaskRef != nil {
		t.Error("expected nil SMask reference for JPEG")
	}
	if objCount != 1 {
		t.Errorf("expected 1 object added, got %d", objCount)
	}
}

func TestJPEGBuildXObjectColorSpace(t *testing.T) {
	// Test that a grayscale JPEG builds correctly through BuildXObject.
	gray := goimage.NewGray(goimage.Rect(0, 0, 10, 10))
	for y := range 10 {
		for x := range 10 {
			gray.SetGray(x, y, color.Gray{Y: 128})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, gray, nil); err != nil {
		t.Fatalf("jpeg.Encode: %v", err)
	}

	img, err := NewJPEG(buf.Bytes())
	if err != nil {
		t.Fatalf("NewJPEG failed: %v", err)
	}
	if img.colorSpace != "DeviceGray" {
		t.Errorf("expected DeviceGray, got %s", img.colorSpace)
	}

	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		return core.NewPdfIndirectReference(1, 0)
	}

	imgRef, smaskRef := img.BuildXObject(addObject)
	if imgRef == nil {
		t.Fatal("expected non-nil image reference")
	}
	if smaskRef != nil {
		t.Error("expected nil SMask for grayscale JPEG")
	}
}

func TestNewJPEGCMYK(t *testing.T) {
	// Craft a synthetic JPEG with 4 components (CMYK).
	// We only need SOI + SOF0 with ncomp=4 — parseJPEGHeader reads
	// dimensions and component count from the SOF marker.
	data := []byte{
		0xFF, 0xD8, // SOI
		0xFF, 0xC0, // SOF0 (Baseline DCT)
		0x00, 0x11, // segment length = 17 (header + 4 components * 3)
		0x08,       // precision = 8
		0x00, 0x01, // height = 1
		0x00, 0x01, // width = 1
		0x04, // ncomp = 4 (CMYK)
		// component specifications (4 * 3 bytes)
		0x01, 0x11, 0x00,
		0x02, 0x11, 0x00,
		0x03, 0x11, 0x00,
		0x04, 0x11, 0x00,
	}
	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG CMYK: %v", err)
	}
	if img.colorSpace != "DeviceCMYK" {
		t.Errorf("expected DeviceCMYK, got %s", img.colorSpace)
	}
	if img.Width() != 1 || img.Height() != 1 {
		t.Errorf("expected 1x1, got %dx%d", img.Width(), img.Height())
	}
	// No APP14 marker => no Adobe-inverted-CMYK flag.
	if img.adobeCMYK {
		t.Error("CMYK without APP14 should not set adobeCMYK")
	}
}

// app14AdobeSegment returns a complete APP14 Adobe marker segment with
// the given ColorTransform byte. The segment is 16 bytes total: marker
// (2) + length (2) + "Adobe" (5) + DCTEncodeVersion (2) + Flags0 (2) +
// Flags1 (2) + ColorTransform (1).
func app14AdobeSegment(colorTransform byte) []byte {
	return []byte{
		0xFF, 0xEE, // APP14 marker
		0x00, 0x0E, // segment length = 14 (includes length field)
		'A', 'd', 'o', 'b', 'e',
		0x00, 0x64, // DCTEncodeVersion = 100
		0x80, 0x00, // APP14Flags0 = 0x8000
		0x00, 0x00, // APP14Flags1
		colorTransform,
	}
}

// cmykSOF0 returns a minimal 1x1 CMYK SOF0 segment (SOF marker through
// the four component records).
func cmykSOF0() []byte {
	return []byte{
		0xFF, 0xC0, // SOF0
		0x00, 0x11, // length = 17
		0x08,       // precision
		0x00, 0x01, // height
		0x00, 0x01, // width
		0x04, // ncomp = 4
		0x01, 0x11, 0x00,
		0x02, 0x11, 0x00,
		0x03, 0x11, 0x00,
		0x04, 0x11, 0x00,
	}
}

func TestNewJPEGAdobeCMYK(t *testing.T) {
	// SOI + APP14 (Adobe) + CMYK SOF0. The APP14 marker flips the
	// adobeCMYK flag on the resulting Image.
	data := []byte{0xFF, 0xD8} // SOI
	data = append(data, app14AdobeSegment(0)...)
	data = append(data, cmykSOF0()...)

	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}
	if img.colorSpace != "DeviceCMYK" {
		t.Errorf("expected DeviceCMYK, got %s", img.colorSpace)
	}
	if !img.adobeCMYK {
		t.Error("APP14 + 4 components should set adobeCMYK")
	}
}

func TestNewJPEGAdobeRGBNotFlagged(t *testing.T) {
	// APP14 marker on a 3-component (RGB/YCbCr) JPEG must NOT flip the
	// adobeCMYK flag — the inversion hack only applies to 4-component
	// streams.
	rgbSOF := []byte{
		0xFF, 0xC0, // SOF0
		0x00, 0x11, // length = 17 (header + 3*3 + pad)
		0x08,
		0x00, 0x01,
		0x00, 0x01,
		0x03, // ncomp = 3
		0x01, 0x11, 0x00,
		0x02, 0x11, 0x01,
		0x03, 0x11, 0x01,
		0x00, 0x00, // padding to match length (length includes itself)
	}
	data := []byte{0xFF, 0xD8}
	data = append(data, app14AdobeSegment(1)...)
	data = append(data, rgbSOF...)

	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}
	if img.adobeCMYK {
		t.Error("3-component JPEG with APP14 should not set adobeCMYK")
	}
}

func TestJPEGBuildXObjectAdobeCMYKEmitsDecode(t *testing.T) {
	data := []byte{0xFF, 0xD8}
	data = append(data, app14AdobeSegment(0)...)
	data = append(data, cmykSOF0()...)

	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}

	var captured core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		captured = obj
		return core.NewPdfIndirectReference(1, 0)
	}
	img.BuildXObject(addObject)

	stream, ok := captured.(*core.PdfStream)
	if !ok {
		t.Fatalf("expected PdfStream, got %T", captured)
	}
	decode := stream.Dict.Get("Decode")
	if decode == nil {
		t.Fatal("expected Decode entry for Adobe CMYK JPEG")
	}
	arr, ok := decode.(*core.PdfArray)
	if !ok {
		t.Fatalf("Decode should be a PdfArray, got %T", decode)
	}
	// Decode array for inverted CMYK: [1 0 1 0 1 0 1 0] — eight integers.
	if arr.Len() != 8 {
		t.Fatalf("Decode array length = %d, want 8", arr.Len())
	}
	want := []int{1, 0, 1, 0, 1, 0, 1, 0}
	for i, w := range want {
		n, ok := arr.At(i).(*core.PdfNumber)
		if !ok {
			t.Errorf("Decode[%d] not a PdfNumber: %T", i, arr.At(i))
			continue
		}
		if n.IntValue() != w {
			t.Errorf("Decode[%d] = %d, want %d", i, n.IntValue(), w)
		}
	}
}

func TestJPEGBuildXObjectPlainRGBNoDecode(t *testing.T) {
	data := createTestJPEG(t, 4, 4)
	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}

	var captured core.PdfObject
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		captured = obj
		return core.NewPdfIndirectReference(1, 0)
	}
	img.BuildXObject(addObject)

	stream, ok := captured.(*core.PdfStream)
	if !ok {
		t.Fatalf("expected PdfStream, got %T", captured)
	}
	if stream.Dict.Get("Decode") != nil {
		t.Error("plain RGB JPEG must not emit a Decode array")
	}
}

// TestNewJPEGAdobeCMYKWithIntermediateSegments covers the realistic
// marker order used by Photoshop: SOI, APP14, DQT, SOF0. The parser must
// remember the APP14 observation across intermediate segments and still
// return the correct dimensions from the later SOF.
func TestNewJPEGAdobeCMYKWithIntermediateSegments(t *testing.T) {
	// DQT (0xFFDB) with a 65-byte quantization table (length 67 incl. length).
	dqt := []byte{0xFF, 0xDB, 0x00, 0x43, 0x00}
	dqt = append(dqt, make([]byte, 64)...)

	data := []byte{0xFF, 0xD8}
	data = append(data, app14AdobeSegment(2)...) // ColorTransform = YCCK
	data = append(data, dqt...)
	data = append(data, cmykSOF0()...)

	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}
	if !img.adobeCMYK {
		t.Error("APP14 before DQT+SOF must still set adobeCMYK")
	}
	if img.Width() != 1 || img.Height() != 1 {
		t.Errorf("dimensions lost: got %dx%d, want 1x1", img.Width(), img.Height())
	}
	if img.colorSpace != "DeviceCMYK" {
		t.Errorf("got %s, want DeviceCMYK", img.colorSpace)
	}
}

// TestNewJPEGBogusAPP14NotFlagged verifies that APP markers at 0xFFEE
// whose payload does not start with "Adobe" are ignored. Some encoders
// use APP14 for their own purposes and we must not mis-flag those.
func TestNewJPEGBogusAPP14NotFlagged(t *testing.T) {
	// APP14 length 14, but identifier is "Appli" instead of "Adobe".
	bogus := []byte{
		0xFF, 0xEE,
		0x00, 0x0E,
		'A', 'p', 'p', 'l', 'i',
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	}
	data := []byte{0xFF, 0xD8}
	data = append(data, bogus...)
	data = append(data, cmykSOF0()...)

	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}
	if img.adobeCMYK {
		t.Error("APP14 with non-Adobe identifier must not set adobeCMYK")
	}
}

// TestNewJPEGTruncatedAPP14NotFlagged covers a short APP14 segment
// whose payload can't hold the full "Adobe" identifier. Parsing must
// continue to SOF without panicking and without setting the flag.
func TestNewJPEGTruncatedAPP14NotFlagged(t *testing.T) {
	// APP14 with length 5: identifier field is only 3 bytes.
	trunc := []byte{
		0xFF, 0xEE,
		0x00, 0x05,
		'A', 'd', 'o',
	}
	data := []byte{0xFF, 0xD8}
	data = append(data, trunc...)
	data = append(data, cmykSOF0()...)

	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}
	if img.adobeCMYK {
		t.Error("truncated APP14 must not set adobeCMYK")
	}
}

// TestJPEGBuildXObjectAdobeCMYKSerialization confirms that the Decode
// array survives round-trip through PdfStream serialization — i.e. the
// bytes emitted to a PDF reader actually contain the /Decode entry.
func TestJPEGBuildXObjectAdobeCMYKSerialization(t *testing.T) {
	data := []byte{0xFF, 0xD8}
	data = append(data, app14AdobeSegment(0)...)
	data = append(data, cmykSOF0()...)

	img, err := NewJPEG(data)
	if err != nil {
		t.Fatalf("NewJPEG: %v", err)
	}

	var captured *core.PdfStream
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		if s, ok := obj.(*core.PdfStream); ok {
			captured = s
		}
		return core.NewPdfIndirectReference(1, 0)
	}
	img.BuildXObject(addObject)
	if captured == nil {
		t.Fatal("no stream captured")
	}

	var buf bytes.Buffer
	if _, err := captured.WriteTo(&buf); err != nil {
		t.Fatalf("stream.WriteTo: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "/Decode") {
		t.Errorf("serialized stream missing /Decode entry:\n%s", out)
	}
	if !strings.Contains(out, "[1 0 1 0 1 0 1 0]") {
		t.Errorf("serialized stream missing expected Decode array:\n%s", out)
	}
	// The raw JPEG bytes (including the APP14 marker) must pass through
	// unchanged — we do not re-encode, only flag.
	if !bytes.Contains(buf.Bytes(), []byte("Adobe")) {
		t.Error("serialized stream should contain the original APP14 Adobe payload")
	}
}
