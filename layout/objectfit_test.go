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

// makeTestImage creates a test JPEG with given dimensions.
func makeTestImage(t *testing.T, w, h int) *folioimage.Image {
	t.Helper()
	img := goimage.NewRGBA(goimage.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode JPEG: %v", err)
	}
	fimg, err := folioimage.NewJPEG(buf.Bytes())
	if err != nil {
		t.Fatalf("create Image: %v", err)
	}
	return fimg
}

func TestObjectFitContain(t *testing.T) {
	// 200x100 image in a 100x100 box with contain: should fit within box.
	fimg := makeTestImage(t, 200, 100) // aspect ratio 2.0
	ie := NewImageElement(fimg)
	ie.SetSize(100, 100) // square box
	ie.SetObjectFit("contain")

	lines := ie.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	w := lines[0].imageRef.width
	h := lines[0].imageRef.height
	// contain: should be 100x50 (fit within 100x100 preserving 2:1 ratio)
	if math.Abs(w-100) > 1 {
		t.Errorf("contain width = %.1f, want 100", w)
	}
	if math.Abs(h-50) > 1 {
		t.Errorf("contain height = %.1f, want 50", h)
	}
}

func TestObjectFitCover(t *testing.T) {
	// 200x100 image in a 100x100 box with cover: should fill the box.
	fimg := makeTestImage(t, 200, 100)
	ie := NewImageElement(fimg)
	ie.SetSize(100, 100)
	ie.SetObjectFit("cover")

	lines := ie.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	w := lines[0].imageRef.width
	h := lines[0].imageRef.height
	// cover: scale up so height fills 100, width = 200
	if math.Abs(h-100) > 1 {
		t.Errorf("cover height = %.1f, want 100", h)
	}
	if math.Abs(w-200) > 1 {
		t.Errorf("cover width = %.1f, want 200", w)
	}
}

func TestObjectFitFill(t *testing.T) {
	// fill: stretch to exact box dimensions.
	fimg := makeTestImage(t, 200, 100)
	ie := NewImageElement(fimg)
	ie.SetSize(100, 100)
	ie.SetObjectFit("fill")

	lines := ie.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	w := lines[0].imageRef.width
	h := lines[0].imageRef.height
	if math.Abs(w-100) > 1 {
		t.Errorf("fill width = %.1f, want 100", w)
	}
	if math.Abs(h-100) > 1 {
		t.Errorf("fill height = %.1f, want 100", h)
	}
}

func TestObjectFitNone(t *testing.T) {
	// none: use natural dimensions (pixels * 0.75 for pt).
	fimg := makeTestImage(t, 200, 100)
	ie := NewImageElement(fimg)
	ie.SetSize(50, 50)
	ie.SetObjectFit("none")

	lines := ie.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	w := lines[0].imageRef.width
	h := lines[0].imageRef.height
	// Natural size: 200px * 0.75 = 150pt, 100px * 0.75 = 75pt.
	if math.Abs(w-150) > 1 {
		t.Errorf("none width = %.1f, want 150", w)
	}
	if math.Abs(h-75) > 1 {
		t.Errorf("none height = %.1f, want 75", h)
	}
}

func TestObjectFitScaleDown(t *testing.T) {
	// scale-down with large box: should use natural size (smaller).
	fimg := makeTestImage(t, 100, 50)
	ie := NewImageElement(fimg)
	ie.SetSize(500, 500) // box bigger than image
	ie.SetObjectFit("scale-down")

	lines := ie.Layout(1000)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	w := lines[0].imageRef.width
	h := lines[0].imageRef.height
	// Natural: 100*0.75=75pt, 50*0.75=37.5pt — smaller than contain(500,250)
	if math.Abs(w-75) > 1 {
		t.Errorf("scale-down width = %.1f, want 75", w)
	}
	if math.Abs(h-37.5) > 1 {
		t.Errorf("scale-down height = %.1f, want 37.5", h)
	}
}

func TestObjectFitScaleDownLargeImage(t *testing.T) {
	// scale-down with small box: should use contain behavior.
	fimg := makeTestImage(t, 400, 200)
	ie := NewImageElement(fimg)
	ie.SetSize(100, 100)
	ie.SetObjectFit("scale-down")

	lines := ie.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	w := lines[0].imageRef.width
	h := lines[0].imageRef.height
	// contain: 100x50 (natural 300x150 is bigger, so scale down)
	if math.Abs(w-100) > 1 {
		t.Errorf("scale-down width = %.1f, want 100", w)
	}
	if math.Abs(h-50) > 1 {
		t.Errorf("scale-down height = %.1f, want 50", h)
	}
}

func TestObjectFitNotSetPreservesDefault(t *testing.T) {
	// Without object-fit, default behavior preserves aspect ratio.
	fimg := makeTestImage(t, 200, 100)
	ie := NewImageElement(fimg)
	ie.SetSize(100, 0) // only width set

	lines := ie.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	w := lines[0].imageRef.width
	h := lines[0].imageRef.height
	if math.Abs(w-100) > 1 {
		t.Errorf("default width = %.1f, want 100", w)
	}
	if math.Abs(h-50) > 1 {
		t.Errorf("default height = %.1f, want 50", h)
	}
}
