// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"sync"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

// examplePDFBytes runs the example's shaping pipeline once per test
// process and caches the result.
var examplePDFBytes = sync.OnceValue(func() []byte {
	doc := buildDocument()
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		panic("WriteTo: " + err.Error())
	}
	return buf.Bytes()
})

func examplePDFReader(t *testing.T) *reader.PdfReader {
	t.Helper()
	r, err := reader.Parse(examplePDFBytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	return r
}

// TestIndicExampleProducesValidPDF asserts the example produces a
// well-formed PDF of plausible size. This needs no system font: the
// intro paragraph renders in Helvetica regardless of what
// loadDevanagariFont finds.
func TestIndicExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 1500 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >1.5 KB", len(pdf))
	}
}

// TestIndicExampleMetadataInInfoDict verifies the /Info dictionary
// carries the Title/Author main() sets on the Document.
func TestIndicExampleMetadataInInfoDict(t *testing.T) {
	r := examplePDFReader(t)
	title, author, _, _, _ := r.Info()
	if title != "Indic Text Shaping" {
		t.Errorf("/Info Title = %q, want %q", title, "Indic Text Shaping")
	}
	if author != "Folio" {
		t.Errorf("/Info Author = %q, want %q", author, "Folio")
	}
}

// TestIndicExampleDevanagariFontEmbedded mirrors the cjk example's
// pattern: when loadDevanagariFont finds a usable system font, pin
// that its PostScript name appears as a subset-prefixed /BaseFont in
// the output — proof the shaping pipeline actually ran with an
// embedded Devanagari-capable face, not just that the section's
// static English fallback paragraph rendered. Skips (rather than
// passing vacuously) when no such font exists on the host.
func TestIndicExampleDevanagariFontEmbedded(t *testing.T) {
	ef := loadDevanagariFont()
	if ef == nil {
		t.Skip("no Devanagari font found on this host; skipping")
	}
	ps := ef.Face().PostScriptName()
	if ps == "" {
		t.Fatal("Devanagari font has empty PostScriptName; cannot pin embedding")
	}

	pdf := examplePDFBytes()
	needle := []byte("+" + ps)
	if !bytes.Contains(pdf, needle) {
		t.Errorf("output PDF does not embed the Devanagari font: looking for %q (subset of PostScriptName %q)", needle, ps)
	}
}
