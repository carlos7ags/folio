// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	goimage "image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/tiff"
)

// createTestTIFF generates a small TIFF image in memory.
func createTestTIFF(t *testing.T, w, h int) []byte {
	t.Helper()
	img := goimage.NewRGBA(goimage.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, img, nil); err != nil {
		t.Fatalf("tiff.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestNewTIFF(t *testing.T) {
	data := createTestTIFF(t, 80, 60)
	img, err := NewTIFF(data)
	if err != nil {
		t.Fatalf("NewTIFF failed: %v", err)
	}
	if img.Width() != 80 {
		t.Errorf("expected width 80, got %d", img.Width())
	}
	if img.Height() != 60 {
		t.Errorf("expected height 60, got %d", img.Height())
	}
	if img.colorSpace != "DeviceRGB" {
		t.Errorf("expected DeviceRGB, got %s", img.colorSpace)
	}
	if img.filter != "FlateDecode" {
		t.Errorf("expected FlateDecode, got %s", img.filter)
	}
}

func TestLoadTIFF(t *testing.T) {
	data := createTestTIFF(t, 30, 20)
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.tiff")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	img, err := LoadTIFF(path)
	if err != nil {
		t.Fatalf("LoadTIFF failed: %v", err)
	}
	if img.Width() != 30 {
		t.Errorf("expected width 30, got %d", img.Width())
	}
	if img.Height() != 20 {
		t.Errorf("expected height 20, got %d", img.Height())
	}
}

func TestTIFFGrayscale(t *testing.T) {
	gray := goimage.NewGray(goimage.Rect(0, 0, 20, 20))
	for y := range 20 {
		for x := range 20 {
			gray.SetGray(x, y, color.Gray{Y: 200})
		}
	}
	var buf bytes.Buffer
	if err := tiff.Encode(&buf, gray, nil); err != nil {
		t.Fatalf("tiff.Encode: %v", err)
	}

	img, err := NewTIFF(buf.Bytes())
	if err != nil {
		t.Fatalf("NewTIFF failed: %v", err)
	}
	if img.colorSpace != "DeviceGray" {
		t.Errorf("expected DeviceGray, got %s", img.colorSpace)
	}
}

func TestNewTIFFInvalid(t *testing.T) {
	_, err := NewTIFF([]byte{0, 1, 2, 3})
	if err == nil {
		t.Error("expected error for invalid TIFF data")
	}
}

func TestLoadTIFFNotFound(t *testing.T) {
	_, err := LoadTIFF("/nonexistent/path/test.tiff")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}
