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

func TestPageDebugMediaBox(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	p.SetDebugMediaBox([3]float64{1, 0, 0}, 2)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	cs := decompressedContentStreams(t, buf.Bytes())

	for _, want := range []string{"2 w", "1 0 0 RG", "0 0 612 792 re", "S"} {
		if !strings.Contains(cs, want) {
			t.Errorf("expected content stream to contain %q, got:\n%s", want, cs)
		}
	}
}

func TestPageDebugMediaBoxOverridesDocument(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.SetDebugMediaBox([3]float64{0, 0, 1}, 1) // document default: blue
	p := doc.AddPage()
	p.SetDebugMediaBox([3]float64{1, 0, 0}, 2) // page override: red

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	cs := decompressedContentStreams(t, buf.Bytes())

	if strings.Contains(cs, "0 0 1 RG") {
		t.Error("page-level SetDebugMediaBox should override the document default, but blue stroke color was found")
	}
	if !strings.Contains(cs, "1 0 0 RG") {
		t.Error("expected page-level red stroke color in content stream")
	}
}

func TestPageDebugMediaBoxNegativeLineWidthPanics(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	defer func() {
		if recover() == nil {
			t.Error("expected panic for negative line width")
		}
	}()
	p.SetDebugMediaBox([3]float64{0, 0, 0}, -1)
}
