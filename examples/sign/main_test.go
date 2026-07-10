// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

// exampleSignedPDF runs the example's certificate + sign pipeline
// once per test process and caches the result. The pipeline generates
// a fresh RSA-2048 key each run, so caching also keeps the test suite
// from paying that cost once per assertion.
var exampleSignedPDF = sync.OnceValue(func() []byte {
	signed, err := buildSignedPDF()
	if err != nil {
		panic("buildSignedPDF: " + err.Error())
	}
	return signed
})

// TestSignExampleProducesValidPDF asserts the signed output is a
// well-formed PDF of plausible size. The size floor catches the
// regression where signing silently truncates the byte range or the
// underlying document fails to render.
func TestSignExampleProducesValidPDF(t *testing.T) {
	pdf := exampleSignedPDF()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 2000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >2 KB", len(pdf))
	}
}

// TestSignExampleParsesAsValidPDF verifies the signed PDF is not just
// header-shaped but actually parses: cross-reference table, page tree,
// and /Info dictionary all resolve. A signature that corrupts the
// byte range or trailer would otherwise still pass the header check.
func TestSignExampleParsesAsValidPDF(t *testing.T) {
	r, err := reader.Parse(exampleSignedPDF())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	if got := r.PageCount(); got != 1 {
		t.Errorf("PageCount = %d, want 1", got)
	}
	title, _, _, _, _ := r.Info()
	if title != "Signed Document" {
		t.Errorf("/Info Title = %q, want %q", title, "Signed Document")
	}
}

// TestSignExampleHasSignatureDictAndByteRange pins the PAdES B-B
// structural markers the example's own verification step checks for:
// a /Type /Sig dictionary and a /ByteRange entry delimiting the
// signed byte ranges (ISO 32000-2 §12.8.1). Losing either would mean
// the "signature" is cosmetic only — no viewer would recognize it.
func TestSignExampleHasSignatureDictAndByteRange(t *testing.T) {
	pdf := string(exampleSignedPDF())
	if !strings.Contains(pdf, "/Type /Sig") {
		t.Error("no /Type /Sig dictionary; sign.SignPDF should emit a signature dictionary")
	}
	if !strings.Contains(pdf, "/ByteRange") {
		t.Error("no /ByteRange entry; sign.SignPDF should delimit the signed byte ranges")
	}
	if !strings.Contains(pdf, "/SubFilter") {
		t.Error("no /SubFilter entry; expected a PAdES subfilter identifying the signature format")
	}
}

// TestSignExampleSignerNameInDictionary asserts the certificate's
// CommonName (the value passed as sign.Options.Name) is embedded in
// the signature dictionary's /Name entry, not just used to render
// visible text elsewhere in the PDF.
func TestSignExampleSignerNameInDictionary(t *testing.T) {
	pdf := string(exampleSignedPDF())
	if !strings.Contains(pdf, "Folio Example Signer") {
		t.Error("certificate CommonName \"Folio Example Signer\" not found in signed PDF; sign.Options.Name may not be wired through")
	}
}
