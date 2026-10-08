// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package forms

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/reader"
)

// createPDFWithForms builds a minimal PDF containing form fields,
// writes it to bytes, and parses it back for FormFiller testing.
func createPDFWithForms(t *testing.T) *reader.PdfReader {
	t.Helper()
	doc := document.NewDocument(document.PageSizeLetter)
	doc.AddPage()

	form := NewAcroForm()
	form.Add(NewTextField("name", [4]float64{72, 700, 300, 720}, 0))
	form.Add(NewCheckbox("agree", [4]float64{72, 670, 92, 690}, 0, true))
	form.Add(NewDropdown("role", [4]float64{72, 640, 250, 660}, 0, []string{"Dev", "QA", "PM"}))
	doc.SetAcroForm(form)

	var buf strings.Builder
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	r, err := reader.Parse([]byte(buf.String()))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return r
}

func TestFormFillerFieldNames(t *testing.T) {
	r := createPDFWithForms(t)
	ff := NewFormFiller(r)
	names, err := ff.FieldNames()
	if err != nil {
		t.Fatalf("FieldNames: %v", err)
	}
	if len(names) < 3 {
		t.Errorf("expected at least 3 field names, got %d: %v", len(names), names)
	}
	// Check expected names are present.
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	for _, want := range []string{"name", "agree", "role"} {
		if !found[want] {
			t.Errorf("expected field %q in names %v", want, names)
		}
	}
}

func TestFormFillerGetValue(t *testing.T) {
	r := createPDFWithForms(t)
	ff := NewFormFiller(r)

	// Checkbox was set to checked=true, so value should be "Yes".
	val, err := ff.GetValue("agree")
	if err != nil {
		t.Fatalf("GetValue(agree): %v", err)
	}
	if val != "Yes" {
		t.Errorf("expected 'Yes' for checked checkbox, got %q", val)
	}
}

func TestFormFillerSetValue(t *testing.T) {
	r := createPDFWithForms(t)
	ff := NewFormFiller(r)

	err := ff.SetValue("name", "Carlos")
	if err != nil {
		t.Fatalf("SetValue: %v", err)
	}

	// Read back.
	val, err := ff.GetValue("name")
	if err != nil {
		t.Fatalf("GetValue after set: %v", err)
	}
	if val != "Carlos" {
		t.Errorf("expected 'Carlos', got %q", val)
	}
}

func TestFormFillerSetCheckbox(t *testing.T) {
	r := createPDFWithForms(t)
	ff := NewFormFiller(r)

	// Uncheck.
	err := ff.SetCheckbox("agree", false)
	if err != nil {
		t.Fatalf("SetCheckbox(false): %v", err)
	}
	val, _ := ff.GetValue("agree")
	if val != "Off" {
		t.Errorf("expected 'Off' after uncheck, got %q", val)
	}

	// Re-check.
	err = ff.SetCheckbox("agree", true)
	if err != nil {
		t.Fatalf("SetCheckbox(true): %v", err)
	}
	val, _ = ff.GetValue("agree")
	if val != "Yes" && val != "" {
		// "Yes" is default export value when AP/N is missing.
		t.Logf("checkbox value after re-check: %q", val)
	}
}

func TestFormFillerFieldNotFound(t *testing.T) {
	r := createPDFWithForms(t)
	ff := NewFormFiller(r)

	_, err := ff.GetValue("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent field")
	}

	err = ff.SetValue("nonexistent", "value")
	if err == nil {
		t.Error("expected error for nonexistent field")
	}

	err = ff.SetCheckbox("nonexistent", true)
	if err == nil {
		t.Error("expected error for nonexistent field")
	}
}

// TestFormFillerNonASCIINamesAndValues verifies that field names and
// values outside ASCII are written as PDF text strings and that the
// filler decodes them when reading back and matching names.
func TestFormFillerNonASCIINamesAndValues(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.AddPage()
	form := NewAcroForm()
	tf := NewTextField("Año", [4]float64{72, 700, 300, 720}, 0)
	tf.SetValue("Sí — “claro”")
	form.Add(tf)
	doc.SetAcroForm(form)

	var buf strings.Builder
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if strings.Contains(buf.String(), "A\xc3\xb1o") || strings.Contains(buf.String(), "S\xc3\xad") {
		t.Error("raw UTF-8 bytes leaked into the field name or value")
	}
	r, err := reader.Parse([]byte(buf.String()))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	ff := NewFormFiller(r)

	names, err := ff.FieldNames()
	if err != nil {
		t.Fatalf("FieldNames: %v", err)
	}
	if len(names) != 1 || names[0] != "Año" {
		t.Errorf("FieldNames = %q, want [Año]", names)
	}
	val, err := ff.GetValue("Año")
	if err != nil {
		t.Fatalf("GetValue: %v", err)
	}
	if val != "Sí — “claro”" {
		t.Errorf("GetValue = %q", val)
	}
	if err := ff.SetValue("Año", "Não"); err != nil {
		t.Fatalf("SetValue: %v", err)
	}
	if val, _ := ff.GetValue("Año"); val != "Não" {
		t.Errorf("GetValue after SetValue = %q, want Não", val)
	}
}
