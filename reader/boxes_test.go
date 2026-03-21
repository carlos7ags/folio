// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"math"
	"testing"

	"github.com/carlos7ags/folio/document"
)

func TestReadPageBoxes(t *testing.T) {
	// Generate a PDF with all box types set.
	doc := document.NewDocument(document.PageSizeLetter)
	p := doc.AddPage()
	p.SetCropBox([4]float64{36, 36, 576, 756})
	p.SetBleedBox([4]float64{18, 18, 594, 774})
	p.SetTrimBox([4]float64{36, 36, 576, 756})
	p.SetArtBox([4]float64{72, 72, 540, 720})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	page, _ := r.Page(0)

	// MediaBox should be full letter size.
	if page.MediaBox.Width() != 612 || page.MediaBox.Height() != 792 {
		t.Errorf("MediaBox = %.0fx%.0f, want 612x792", page.MediaBox.Width(), page.MediaBox.Height())
	}

	// CropBox.
	if page.CropBox.IsZero() {
		t.Fatal("CropBox should not be zero")
	}
	if page.CropBox.X1 != 36 || page.CropBox.Y1 != 36 {
		t.Errorf("CropBox origin = (%.0f, %.0f), want (36, 36)", page.CropBox.X1, page.CropBox.Y1)
	}
	if page.CropBox.Width() != 540 || page.CropBox.Height() != 720 {
		t.Errorf("CropBox = %.0fx%.0f, want 540x720", page.CropBox.Width(), page.CropBox.Height())
	}

	// BleedBox.
	if page.BleedBox.IsZero() {
		t.Fatal("BleedBox should not be zero")
	}

	// TrimBox.
	if page.TrimBox.IsZero() {
		t.Fatal("TrimBox should not be zero")
	}

	// ArtBox.
	if page.ArtBox.IsZero() {
		t.Fatal("ArtBox should not be zero")
	}
	if page.ArtBox.X1 != 72 || page.ArtBox.Y1 != 72 {
		t.Errorf("ArtBox origin = (%.0f, %.0f), want (72, 72)", page.ArtBox.X1, page.ArtBox.Y1)
	}
}

func TestVisibleBoxUsesCropBox(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	p := doc.AddPage()
	p.SetCropBox([4]float64{36, 36, 576, 756})

	var buf bytes.Buffer
	_, _ = doc.WriteTo(&buf)

	r, _ := Parse(buf.Bytes())
	page, _ := r.Page(0)

	// Width/Height should come from CropBox, not MediaBox.
	visible := page.VisibleBox()
	if visible.Width() != 540 {
		t.Errorf("visible width = %.0f, want 540 (from CropBox)", visible.Width())
	}
	if page.Width != 540 {
		t.Errorf("page.Width = %.0f, want 540 (from CropBox)", page.Width)
	}
}

func TestVisibleBoxFallsBackToMediaBox(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.AddPage() // no CropBox

	var buf bytes.Buffer
	_, _ = doc.WriteTo(&buf)

	r, _ := Parse(buf.Bytes())
	page, _ := r.Page(0)

	if page.Width != 612 || page.Height != 792 {
		t.Errorf("page = %.0fx%.0f, want 612x792 (MediaBox)", page.Width, page.Height)
	}
	visible := page.VisibleBox()
	if visible.Width() != 612 {
		t.Errorf("visible = %.0fx%.0f, want 612x792", visible.Width(), visible.Height())
	}
}

func TestBoxZeroCheck(t *testing.T) {
	var b Box
	if !b.IsZero() {
		t.Error("zero Box should be zero")
	}
	b = Box{0, 0, 612, 792}
	if b.IsZero() {
		t.Error("non-zero Box should not be zero")
	}
}

func TestBoxWidthHeight(t *testing.T) {
	b := Box{36, 36, 576, 756}
	if math.Abs(b.Width()-540) > 0.01 {
		t.Errorf("Width = %.1f, want 540", b.Width())
	}
	if math.Abs(b.Height()-720) > 0.01 {
		t.Errorf("Height = %.1f, want 720", b.Height())
	}
}
