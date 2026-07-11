// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"sync"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

// examplePDFBytes runs the example's font-discovery and layout
// pipeline once per test process and caches the result.
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

// TestRTLExampleProducesValidPDF asserts the example produces a
// well-formed PDF of plausible size. This needs no system fonts: the
// section intros render in Helvetica via makeParagraph's nil-EmbeddedFont
// fallback regardless of what Hebrew/Arabic fonts get discovered.
func TestRTLExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 3000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >3 KB", len(pdf))
	}
}

// TestRTLExampleMetadataInInfoDict verifies the /Info dictionary
// carries the Title/Author main() sets on the Document.
func TestRTLExampleMetadataInInfoDict(t *testing.T) {
	r := examplePDFReader(t)
	title, author, _, _, _ := r.Info()
	if title != "Right-To-Left Text" {
		t.Errorf("/Info Title = %q, want %q", title, "Right-To-Left Text")
	}
	if author != "Folio" {
		t.Errorf("/Info Author = %q, want %q", author, "Folio")
	}
}

// TestRTLExampleHebrewFontEmbedded mirrors the cjk example's pattern:
// when loadHebrewFont finds a usable system font, pin that its
// PostScript name appears as a subset-prefixed /BaseFont in the
// output — proof the Hebrew bidi sections actually rendered with an
// embedded face. Skips (rather than passing vacuously) when no
// Hebrew-capable font exists on the host.
func TestRTLExampleHebrewFontEmbedded(t *testing.T) {
	ef := loadHebrewFont()
	if ef == nil {
		t.Skip("no Hebrew font found on this host; skipping")
	}
	ps := ef.Face().PostScriptName()
	if ps == "" {
		t.Fatal("Hebrew font has empty PostScriptName; cannot pin embedding")
	}

	pdf := examplePDFBytes()
	needle := []byte("+" + ps)
	if !bytes.Contains(pdf, needle) {
		t.Errorf("output PDF does not embed the Hebrew font: looking for %q (subset of PostScriptName %q)", needle, ps)
	}
}

// TestRTLExampleArabicFontEmbedded is the Arabic counterpart of
// TestRTLExampleHebrewFontEmbedded, pinning the font that carries
// contextual shaping, ligatures, GPOS mark attachment, and kashida
// justification in sections 2-2c.
func TestRTLExampleArabicFontEmbedded(t *testing.T) {
	ef := loadArabicFont()
	if ef == nil {
		t.Skip("no Arabic font found on this host; skipping")
	}
	ps := ef.Face().PostScriptName()
	if ps == "" {
		t.Fatal("Arabic font has empty PostScriptName; cannot pin embedding")
	}

	pdf := examplePDFBytes()
	needle := []byte("+" + ps)
	if !bytes.Contains(pdf, needle) {
		t.Errorf("output PDF does not embed the Arabic font: looking for %q (subset of PostScriptName %q)", needle, ps)
	}
}
