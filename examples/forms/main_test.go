// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

// examplePDFBytes runs the example's field-building pipeline once per
// test process and caches the result.
var examplePDFBytes = sync.OnceValue(func() []byte {
	doc := buildDocument()
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		panic("WriteTo: " + err.Error())
	}
	return buf.Bytes()
})

// TestFormsExampleProducesValidPDF asserts the example produces a
// well-formed PDF of plausible size. The size floor catches the
// regression where AcroForm assembly silently drops fields and writes
// a near-empty file.
func TestFormsExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 3000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >3 KB", len(pdf))
	}
}

// TestFormsExampleHasAcroForm asserts the interactive form dictionary
// is present. Without it, the fields would just be inert annotations
// with no AcroForm root — the whole point of this example.
func TestFormsExampleHasAcroForm(t *testing.T) {
	pdf := string(examplePDFBytes())
	if !strings.Contains(pdf, "/AcroForm") {
		t.Fatal("no /AcroForm dictionary; doc.SetAcroForm(form) should register the interactive form root")
	}
}

// TestFormsExampleFieldNamesPresent locks in every field name the
// example registers via forms.New*Field. A regression that drops a
// field from the AcroForm (e.g. a missed form.Add call) surfaces here
// as a missing name, rather than only as a visual gap in the PDF.
func TestFormsExampleFieldNamesPresent(t *testing.T) {
	pdf := string(examplePDFBytes())
	names := []string{
		"fullName", "email", "password", "readonly", "comments",
		"agreeTerms", "subscribe", "preference", "country", "languages",
		"signature", "blue", "green",
	}
	for _, name := range names {
		if !strings.Contains(pdf, "("+name+")") {
			t.Errorf("field name %q not found in PDF; expected it in a /T (field name) entry", name)
		}
	}
}

// TestFormsExampleDistinctFieldTypesPresent asserts the three AcroForm
// field-type families the example demonstrates (text, button/checkbox,
// choice) all appear. This catches a regression where one whole field
// category (e.g. all checkboxes) is silently mistyped or dropped.
func TestFormsExampleDistinctFieldTypesPresent(t *testing.T) {
	pdf := string(examplePDFBytes())
	types := map[string]string{
		"/FT /Tx":  "text fields (name, email, password, comments, ...)",
		"/FT /Btn": "button fields (checkboxes, radio group)",
		"/FT /Ch":  "choice fields (dropdown, list box)",
	}
	for marker, desc := range types {
		if !strings.Contains(pdf, marker) {
			t.Errorf("no %q field type marker found (%s)", marker, desc)
		}
	}
}
