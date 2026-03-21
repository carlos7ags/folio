// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"strings"
	"testing"
)

func TestPageCropBox(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.SetCropBox([4]float64{36, 36, 576, 756})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/CropBox") {
		t.Error("expected /CropBox in page dict")
	}
}

func TestPageAllBoxes(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.SetCropBox([4]float64{0, 0, 612, 792})
	p.SetBleedBox([4]float64{-18, -18, 630, 810})
	p.SetTrimBox([4]float64{0, 0, 612, 792})
	p.SetArtBox([4]float64{36, 36, 576, 756})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	pdf := buf.String()
	for _, box := range []string{"/CropBox", "/BleedBox", "/TrimBox", "/ArtBox"} {
		if !strings.Contains(pdf, box) {
			t.Errorf("expected %s in page dict", box)
		}
	}
}

func TestPageNoBoxesDefault(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()

	var buf bytes.Buffer
	_, _ = doc.WriteTo(&buf)

	pdf := buf.String()
	// Without explicit boxes, only MediaBox should appear.
	if !strings.Contains(pdf, "/MediaBox") {
		t.Error("expected /MediaBox")
	}
	if strings.Contains(pdf, "/CropBox") {
		t.Error("unexpected /CropBox on default page")
	}
}

func TestBoxQpdfCheck(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.SetCropBox([4]float64{36, 36, 576, 756})
	p.SetTrimBox([4]float64{36, 36, 576, 756})

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	runQpdfCheck(t, buf.Bytes())
}
