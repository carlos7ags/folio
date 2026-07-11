// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"encoding/binary"
	"errors"
	goimage "image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/tiff"
)

// craftTIFFHeader builds a minimal little-endian TIFF with a single IFD
// declaring ImageWidth and ImageLength as LONG values. No strip data follows,
// so a full decode would fail; the IFD alone lets DecodeConfig report the
// dimensions before any raster is allocated.
func craftTIFFHeader(w, h uint32) []byte {
	var b bytes.Buffer
	b.Write([]byte{'I', 'I'})     // little-endian byte order
	b.Write([]byte{0x2A, 0x00})   // magic 42
	writeU32LE(&b, 8)             // offset of first IFD
	writeU16LE(&b, 2)             // entry count
	writeTIFFEntry(&b, 256, 4, w) // ImageWidth (LONG)
	writeTIFFEntry(&b, 257, 4, h) // ImageLength (LONG)
	writeU32LE(&b, 0)             // next IFD offset (none)
	return b.Bytes()
}

func writeU16LE(b *bytes.Buffer, v uint16) {
	buf := make([]byte, 2)
	binary.LittleEndian.PutUint16(buf, v)
	b.Write(buf)
}

func writeU32LE(b *bytes.Buffer, v uint32) {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, v)
	b.Write(buf)
}

func writeTIFFEntry(b *bytes.Buffer, tag, typ uint16, value uint32) {
	writeU16LE(b, tag)
	writeU16LE(b, typ)
	writeU32LE(b, 1) // count
	writeU32LE(b, value)
}

// TestNewTIFFOversizedHeader verifies the pixel-count limit is enforced from
// the IFD dimensions, before the raster is decoded.
func TestNewTIFFOversizedHeader(t *testing.T) {
	_, err := NewTIFF(craftTIFFHeader(16000, 16000))
	if !errors.Is(err, ErrPixelCountTooLarge) {
		t.Fatalf("expected ErrPixelCountTooLarge, got %v", err)
	}
}

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
