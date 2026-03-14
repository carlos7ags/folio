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
		return &core.PdfIndirectReference{ObjectNumber: objCount, GenerationNumber: 0}
	}

	imgRef, smaskRef := img.BuildXObject(addObject)
	if imgRef == nil {
		t.Fatal("expected non-nil image reference")
	}
	if imgRef.ObjectNumber != 1 {
		t.Errorf("expected object number 1, got %d", imgRef.ObjectNumber)
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
		return &core.PdfIndirectReference{ObjectNumber: 1, GenerationNumber: 0}
	}

	imgRef, smaskRef := img.BuildXObject(addObject)
	if imgRef == nil {
		t.Fatal("expected non-nil image reference")
	}
	if smaskRef != nil {
		t.Error("expected nil SMask for grayscale JPEG")
	}
}
