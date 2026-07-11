// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"encoding/binary"
	"errors"
	goimage "image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"os"
	"path/filepath"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// craftGIFHeader builds a GIF89a signature and logical screen descriptor
// declaring the given canvas dimensions, with no global color table. This is
// enough for DecodeConfig to report the size before any frame is decoded.
func craftGIFHeader(w, h uint16) []byte {
	b := append([]byte("GIF89a"), 0, 0, 0, 0, 0, 0, 0)
	binary.LittleEndian.PutUint16(b[6:8], w)
	binary.LittleEndian.PutUint16(b[8:10], h)
	return b
}

// TestNewGIFOversizedHeader verifies the dimension limit is enforced from the
// logical-screen descriptor, before the frame buffer is allocated.
func TestNewGIFOversizedHeader(t *testing.T) {
	_, err := NewGIF(craftGIFHeader(60000, 100))
	if !errors.Is(err, ErrDimensionTooLarge) {
		t.Fatalf("expected ErrDimensionTooLarge, got %v", err)
	}
}

// createTestGIF generates a small GIF image in memory.
func createTestGIF(t *testing.T, w, h int) []byte {
	t.Helper()
	img := goimage.NewPaletted(goimage.Rect(0, 0, w, h), palette.Plan9)
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	var buf bytes.Buffer
	err := gif.Encode(&buf, img, nil)
	if err != nil {
		t.Fatalf("gif.Encode: %v", err)
	}
	return buf.Bytes()
}

func TestNewGIF(t *testing.T) {
	data := createTestGIF(t, 20, 15)
	img, err := NewGIF(data)
	if err != nil {
		t.Fatalf("NewGIF: %v", err)
	}
	if img.Width() != 20 {
		t.Errorf("expected width 20, got %d", img.Width())
	}
	if img.Height() != 15 {
		t.Errorf("expected height 15, got %d", img.Height())
	}
	if img.colorSpace != "DeviceRGB" {
		t.Errorf("expected DeviceRGB, got %s", img.colorSpace)
	}
	if img.filter != "FlateDecode" {
		t.Errorf("expected FlateDecode, got %s", img.filter)
	}
}

func TestLoadGIF(t *testing.T) {
	data := createTestGIF(t, 10, 10)
	dir := t.TempDir()
	path := filepath.Join(dir, "test.gif")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	img, err := LoadGIF(path)
	if err != nil {
		t.Fatalf("LoadGIF: %v", err)
	}
	if img.Width() != 10 {
		t.Errorf("expected width 10, got %d", img.Width())
	}
}

func TestLoadGIFNotFound(t *testing.T) {
	_, err := LoadGIF("/nonexistent/test.gif")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestNewGIFInvalid(t *testing.T) {
	_, err := NewGIF([]byte{0, 1, 2, 3})
	if err == nil {
		t.Error("expected error for invalid GIF data")
	}
}

func TestGIFBuildXObject(t *testing.T) {
	data := createTestGIF(t, 5, 5)
	img, err := NewGIF(data)
	if err != nil {
		t.Fatalf("NewGIF: %v", err)
	}
	objCount := 0
	addObject := func(obj core.PdfObject) *core.PdfIndirectReference {
		objCount++
		return core.NewPdfIndirectReference(objCount, 0)
	}
	ref, _ := img.BuildXObject(addObject)
	if ref == nil {
		t.Fatal("expected non-nil image reference")
	}
}
