// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader_test

import (
	"bytes"
	goimage "image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	folioimage "github.com/carlos7ags/folio/image"
	"github.com/carlos7ags/folio/reader"
)

// buildGrayPNGRoundTrip builds an opaque grayscale PNG with varied pixel
// values (gradient plus noise), embeds it in a one-page document, and
// returns both the written PDF bytes and the flattened pixel bytes the
// pre-passthrough decode path would have produced.
func buildGrayPNGRoundTrip(t *testing.T, w, h int) (pdfBytes, wantPixels []byte) {
	t.Helper()
	src := goimage.NewGray(goimage.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			src.SetGray(x, y, color.Gray{Y: byte((x*7 + y*13) % 256)})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}

	want := make([]byte, 0, w*h)
	for y := range h {
		for x := range w {
			r, _, _, _ := src.At(x, y).RGBA()
			want = append(want, byte(r>>8))
		}
	}

	pdfBytes = embedInDocument(t, buf.Bytes())
	return pdfBytes, want
}

// buildRGBPNGRoundTrip is the truecolor counterpart of
// buildGrayPNGRoundTrip.
func buildRGBPNGRoundTrip(t *testing.T, w, h int) (pdfBytes, wantPixels []byte) {
	t.Helper()
	src := goimage.NewRGBA(goimage.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			src.Set(x, y, color.RGBA{
				R: byte((x*7 + y*3) % 256),
				G: byte((x*11 + y*17) % 256),
				B: byte((x*5 + y*23) % 256),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}

	want := make([]byte, 0, w*h*3)
	for y := range h {
		for x := range w {
			r, g, b, _ := src.At(x, y).RGBA()
			want = append(want, byte(r>>8), byte(g>>8), byte(b>>8))
		}
	}

	pdfBytes = embedInDocument(t, buf.Bytes())
	return pdfBytes, want
}

func embedInDocument(t *testing.T, pngBytes []byte) []byte {
	t.Helper()
	img, err := folioimage.NewPNG(pngBytes)
	if err != nil {
		t.Fatalf("folioimage.NewPNG: %v", err)
	}
	doc := document.NewDocument(document.PageSizeLetter)
	p := doc.AddPage()
	p.AddImage(img, 72, 600, float64(img.Width()), float64(img.Height()))

	var out bytes.Buffer
	if _, err := doc.WriteTo(&out); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	return out.Bytes()
}

// resolveImageXObject parses pdfBytes, finds the page's "Im1" XObject, and
// returns the resolved stream (Data already inflated and, if the stream
// carried a PNG predictor, un-predicted by the reader).
func resolveImageXObject(t *testing.T, pdfBytes []byte) *core.PdfStream {
	t.Helper()
	r, err := reader.Parse(pdfBytes)
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	resources, err := page.Resources()
	if err != nil {
		t.Fatalf("Resources: %v", err)
	}
	xobjDictObj, err := r.ResolveObject(resources.Get("XObject"))
	if err != nil {
		t.Fatalf("resolve XObject dict: %v", err)
	}
	xobjDict, ok := xobjDictObj.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("expected XObject to resolve to a dictionary, got %T", xobjDictObj)
	}
	imRef := xobjDict.Get("Im1")
	if imRef == nil {
		t.Fatal("expected /Im1 XObject entry")
	}
	imObj, err := r.ResolveObject(imRef)
	if err != nil {
		t.Fatalf("resolve Im1: %v", err)
	}
	stream, ok := imObj.(*core.PdfStream)
	if !ok {
		t.Fatalf("expected Im1 to resolve to a stream, got %T", imObj)
	}
	return stream
}

func TestPNGPassthroughRoundTripGray(t *testing.T) {
	pdfBytes, want := buildGrayPNGRoundTrip(t, 16, 11)

	pdf := string(pdfBytes)
	if !strings.Contains(pdf, "/Filter /FlateDecode") {
		t.Error("expected /Filter /FlateDecode in written PDF")
	}
	if !strings.Contains(pdf, "/DecodeParms") {
		t.Error("expected /DecodeParms in written PDF")
	}
	if !strings.Contains(pdf, "/Predictor 15") {
		t.Error("expected /Predictor 15 in written PDF")
	}
	if !strings.Contains(pdf, "/Colors 1") {
		t.Error("expected /Colors 1 in written PDF")
	}

	stream := resolveImageXObject(t, pdfBytes)
	if !bytes.Equal(stream.Data, want) {
		t.Fatalf("predictor-decoded bytes mismatch: got %d bytes, want %d bytes", len(stream.Data), len(want))
	}
}

func TestPNGPassthroughRoundTripRGB(t *testing.T) {
	pdfBytes, want := buildRGBPNGRoundTrip(t, 16, 11)

	pdf := string(pdfBytes)
	if !strings.Contains(pdf, "/Filter /FlateDecode") {
		t.Error("expected /Filter /FlateDecode in written PDF")
	}
	if !strings.Contains(pdf, "/DecodeParms") {
		t.Error("expected /DecodeParms in written PDF")
	}
	if !strings.Contains(pdf, "/Predictor 15") {
		t.Error("expected /Predictor 15 in written PDF")
	}
	if !strings.Contains(pdf, "/Colors 3") {
		t.Error("expected /Colors 3 in written PDF")
	}

	stream := resolveImageXObject(t, pdfBytes)
	if !bytes.Equal(stream.Data, want) {
		t.Fatalf("predictor-decoded bytes mismatch: got %d bytes, want %d bytes", len(stream.Data), len(want))
	}
}
