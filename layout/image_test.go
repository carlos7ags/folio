// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"bytes"
	goimage "image"
	"image/jpeg"
	"math"
	"testing"

	folioimage "github.com/carlos7ags/folio/image"
)

func createTestImage(t *testing.T) *folioimage.Image {
	t.Helper()
	// create a small JPEG in memory (200x100 pixels, aspect ratio 2.0)
	img := goimage.NewRGBA(goimage.Rect(0, 0, 200, 100))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("failed to encode test JPEG: %v", err)
	}
	fimg, err := folioimage.NewJPEG(buf.Bytes())
	if err != nil {
		t.Fatalf("failed to create folio Image: %v", err)
	}
	return fimg
}

func TestImageElementLayout(t *testing.T) {
	fimg := createTestImage(t)
	ie := NewImageElement(fimg)
	lines := ie.Layout(400)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	line := lines[0]
	if line.imageRef == nil {
		t.Fatal("expected imageRef to be set")
	}
	if !line.IsLast {
		t.Error("expected IsLast to be true")
	}
	if line.Align != AlignLeft {
		t.Errorf("expected AlignLeft, got %d", line.Align)
	}
	// Default: scale to fit available width (400), height = 400/2.0 = 200
	if math.Abs(line.Width-400) > 0.01 {
		t.Errorf("expected width 400, got %f", line.Width)
	}
	if math.Abs(line.Height-200) > 0.01 {
		t.Errorf("expected height 200, got %f", line.Height)
	}
}

func TestImageElementSetSize(t *testing.T) {
	fimg := createTestImage(t)
	ie := NewImageElement(fimg).SetSize(100, 50)

	lines := ie.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if math.Abs(lines[0].Width-100) > 0.01 {
		t.Errorf("expected width 100, got %f", lines[0].Width)
	}
	if math.Abs(lines[0].Height-50) > 0.01 {
		t.Errorf("expected height 50, got %f", lines[0].Height)
	}
}

func TestImageElementSetAlign(t *testing.T) {
	fimg := createTestImage(t)
	ie := NewImageElement(fimg).SetAlign(AlignCenter)

	lines := ie.Layout(400)
	if lines[0].Align != AlignCenter {
		t.Errorf("expected AlignCenter, got %d", lines[0].Align)
	}

	ie2 := NewImageElement(fimg).SetAlign(AlignRight)
	lines2 := ie2.Layout(400)
	if lines2[0].Align != AlignRight {
		t.Errorf("expected AlignRight, got %d", lines2[0].Align)
	}
}

func TestImageElementAutoHeight(t *testing.T) {
	// SetSize with width only (height=0): height computed from aspect ratio
	fimg := createTestImage(t) // 200x100, aspect ratio = 2.0
	ie := NewImageElement(fimg).SetSize(100, 0)

	lines := ie.Layout(400)
	// height = width / ar = 100 / 2.0 = 50
	if math.Abs(lines[0].Width-100) > 0.01 {
		t.Errorf("expected width 100, got %f", lines[0].Width)
	}
	if math.Abs(lines[0].Height-50) > 0.01 {
		t.Errorf("expected height 50, got %f", lines[0].Height)
	}
}

func TestImageElementAutoWidth(t *testing.T) {
	// SetSize with height only (width=0): width computed from aspect ratio
	fimg := createTestImage(t) // 200x100, aspect ratio = 2.0
	ie := NewImageElement(fimg).SetSize(0, 100)

	lines := ie.Layout(400)
	// width = height * ar = 100 * 2.0 = 200
	if math.Abs(lines[0].Width-200) > 0.01 {
		t.Errorf("expected width 200, got %f", lines[0].Width)
	}
	if math.Abs(lines[0].Height-100) > 0.01 {
		t.Errorf("expected height 100, got %f", lines[0].Height)
	}
}

func TestImageElementFitToWidth(t *testing.T) {
	// When explicit size exceeds maxWidth, image is clamped to maxWidth
	fimg := createTestImage(t) // 200x100, aspect ratio = 2.0
	ie := NewImageElement(fimg).SetSize(600, 300)

	lines := ie.Layout(400)
	// Clamped: width = 400, height = 400 / 2.0 = 200
	if math.Abs(lines[0].Width-400) > 0.01 {
		t.Errorf("expected width clamped to 400, got %f", lines[0].Width)
	}
	if math.Abs(lines[0].Height-200) > 0.01 {
		t.Errorf("expected height 200 after clamping, got %f", lines[0].Height)
	}
}

func TestImageElementImplementsElement(t *testing.T) {
	fimg := createTestImage(t)
	var _ Element = NewImageElement(fimg)
}

func TestImageResName(t *testing.T) {
	if imageResName(0) != "Im1" {
		t.Errorf("expected Im1, got %s", imageResName(0))
	}
	if imageResName(2) != "Im3" {
		t.Errorf("expected Im3, got %s", imageResName(2))
	}
}

func TestImageElementZeroSizeImage(t *testing.T) {
	// Create a zero-size Go image and convert it.
	zeroImg := goimage.NewRGBA(goimage.Rect(0, 0, 0, 0))
	fimg := folioimage.NewFromGoImage(zeroImg)
	if fimg != nil {
		t.Fatal("expected nil for zero-size image")
	}
}

func TestImageElementNilImage(t *testing.T) {
	// NewFromGoImage with nil should return nil.
	fimg := folioimage.NewFromGoImage(nil)
	if fimg != nil {
		t.Fatal("expected nil for nil input")
	}
}

func TestImageElementZeroSizeInLayout(t *testing.T) {
	// Create a 1x1 image (smallest valid), then set size to 0,0.
	// The resolveSize should use maxWidth and aspect ratio.
	img := goimage.NewRGBA(goimage.Rect(0, 0, 1, 1))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("failed to encode test JPEG: %v", err)
	}
	fimg, err := folioimage.NewJPEG(buf.Bytes())
	if err != nil {
		t.Fatalf("failed to create folio Image: %v", err)
	}
	ie := NewImageElement(fimg).SetSize(0, 0)
	lines := ie.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// Should not panic, and should produce positive dimensions.
	if lines[0].Width <= 0 || lines[0].Height <= 0 {
		t.Errorf("expected positive dimensions, got width=%f height=%f", lines[0].Width, lines[0].Height)
	}
}
