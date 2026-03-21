// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestFormatPdfDate(t *testing.T) {
	// 2024-03-15 14:30:45 UTC
	ts := time.Date(2024, 3, 15, 14, 30, 45, 0, time.UTC)
	got := formatPdfDate(ts)
	expected := "D:20240315143045+00'00'"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatPdfDateNegativeOffset(t *testing.T) {
	loc := time.FixedZone("EST", -5*3600)
	ts := time.Date(2024, 1, 1, 12, 0, 0, 0, loc)
	got := formatPdfDate(ts)
	expected := "D:20240101120000-05'00'"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFormatPdfDatePositiveOffset(t *testing.T) {
	loc := time.FixedZone("IST", 5*3600+30*60)
	ts := time.Date(2024, 6, 15, 9, 0, 0, 0, loc)
	got := formatPdfDate(ts)
	expected := "D:20240615090000+05'30'"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestInfoToDict(t *testing.T) {
	info := &Info{
		Title:  "Test Document",
		Author: "Test Author",
	}
	var buf bytes.Buffer
	_, _ = info.toDict().WriteTo(&buf)
	s := buf.String()

	if !strings.Contains(s, "/Title (Test Document)") {
		t.Error("missing /Title")
	}
	if !strings.Contains(s, "/Author (Test Author)") {
		t.Error("missing /Author")
	}
}

func TestInfoToDictEmpty(t *testing.T) {
	info := &Info{}
	var buf bytes.Buffer
	_, _ = info.toDict().WriteTo(&buf)
	// Empty info dict should have no entries
	if buf.String() != "<< >>" {
		t.Errorf("expected empty dict, got %q", buf.String())
	}
}

func TestInfoIsEmpty(t *testing.T) {
	if !(&Info{}).isEmpty() {
		t.Error("default Info should be empty")
	}
	if (&Info{Title: "x"}).isEmpty() {
		t.Error("Info with Title should not be empty")
	}
}

func TestInfoToDictAllFields(t *testing.T) {
	ts := time.Date(2024, 3, 15, 14, 30, 0, 0, time.UTC)
	info := &Info{
		Title:        "My Title",
		Author:       "John Doe",
		Subject:      "Testing",
		Keywords:     "pdf, test",
		Creator:      "Folio Test",
		Producer:     "Folio",
		CreationDate: ts,
		ModDate:      ts,
	}
	var buf bytes.Buffer
	_, _ = info.toDict().WriteTo(&buf)
	s := buf.String()

	for _, key := range []string{"/Title", "/Author", "/Subject", "/Keywords",
		"/Creator", "/Producer", "/CreationDate", "/ModDate"} {
		if !strings.Contains(s, key) {
			t.Errorf("missing %s in info dict", key)
		}
	}
}

func TestDocumentMetadataInPDF(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Test Report"
	doc.Info.Author = "Folio"
	doc.Info.Producer = "Folio PDF Library"
	doc.AddPage()

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if !strings.Contains(pdf, "/Title (Test Report)") {
		t.Error("PDF missing /Title")
	}
	if !strings.Contains(pdf, "/Author (Folio)") {
		t.Error("PDF missing /Author")
	}
	if !strings.Contains(pdf, "/Info") {
		t.Error("trailer missing /Info reference")
	}
}

func TestDocumentNoMetadataOmitsInfo(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	doc.AddPage()

	var buf bytes.Buffer
	_, err := doc.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	pdf := buf.String()
	if strings.Contains(pdf, "/Info") {
		t.Error("trailer should not have /Info when metadata is empty")
	}
}
