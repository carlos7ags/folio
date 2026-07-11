// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	folioFont "github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/reader"
)

// examplePDFBytes runs the example's discovery + HTML + convert
// pipeline once per test process and caches the result.
var examplePDFBytes = sync.OnceValue(func() []byte {
	doc, err := buildDocument()
	if err != nil {
		panic("buildDocument: " + err.Error())
	}
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

// TestFontsExampleProducesValidPDF asserts the example produces a
// well-formed PDF of plausible size. This assertion needs no system
// fonts: the standard-14 section renders regardless of what custom
// fonts discoverFonts() finds.
func TestFontsExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 2000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >2 KB", len(pdf))
	}
}

// TestFontsExampleParsesAsValidPDF verifies the output is not just
// header-shaped but actually parses: cross-reference table and page
// tree both resolve.
func TestFontsExampleParsesAsValidPDF(t *testing.T) {
	r := examplePDFReader(t)
	if got := r.PageCount(); got < 1 {
		t.Errorf("PageCount = %d, want >= 1", got)
	}
	title, author, _, _, _ := r.Info()
	if title != "Folio Font Showcase" {
		t.Errorf("/Info Title = %q, want %q", title, "Folio Font Showcase")
	}
	if author != "Folio" {
		t.Errorf("/Info Author = %q, want %q", author, "Folio")
	}
}

// TestFontsExampleStandardFontsEmbedded asserts the three standard-14
// PDF fonts the example always exercises (Helvetica, Times, Courier)
// appear as /BaseFont entries. These embed without any font file on
// disk, so this must pass on every host, custom fonts or not.
func TestFontsExampleStandardFontsEmbedded(t *testing.T) {
	pdf := string(examplePDFBytes())
	for _, marker := range []string{"/BaseFont /Helvetica", "/BaseFont /Times", "/BaseFont /Courier"} {
		if !strings.Contains(pdf, marker) {
			t.Errorf("no %q entry; standard-14 font section should always embed regardless of system fonts", marker)
		}
	}
}

// TestFontsExampleCustomFontsEmbedded mirrors the cjk example's
// pattern: for each font discoverFonts() locates on this host, pin
// that its PostScript name appears as a subset-prefixed /BaseFont in
// the output. This is the assertion that would have caught the #391
// regression where the example's @font-face rules used absolute paths
// but did not opt into AllowAbsolutePaths, so no custom font embedded.
// When discoverFonts() finds nothing (no candidate path exists on
// this OS, e.g. Linux CI), the test skips rather than passing
// vacuously — there is nothing font-specific to verify.
func TestFontsExampleCustomFontsEmbedded(t *testing.T) {
	fonts := discoverFonts()
	if len(fonts) == 0 {
		t.Skip("no custom fonts found on this host; skipping")
	}

	pdf := examplePDFBytes()
	for _, f := range fonts {
		face, err := folioFont.LoadFont(f.path)
		if err != nil {
			t.Fatalf("font.LoadFont(%q): %v", f.path, err)
		}
		ps := face.PostScriptName()
		if ps == "" {
			t.Fatalf("font at %q has empty PostScriptName; cannot pin embedding", f.path)
		}
		needle := []byte("+" + ps)
		if !bytes.Contains(pdf, needle) {
			t.Errorf("output PDF does not embed discovered font %q: looking for %q (subset of PostScriptName %q)", f.name, needle, ps)
		}
	}
}
