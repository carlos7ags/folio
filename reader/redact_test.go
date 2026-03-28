// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// createTestPDF generates a simple PDF with known text for redaction testing.
func createTestPDF(t *testing.T, texts ...string) []byte {
	t.Helper()
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Redaction Test"
	doc.Info.Author = "Test Author"
	for _, text := range texts {
		doc.Add(layout.NewParagraph(text, font.Helvetica, 12))
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("create test PDF: %v", err)
	}
	return buf.Bytes()
}

// TestSerializeContentOpsRoundTrip verifies that parsing then serializing
// a content stream produces functionally equivalent output.
func TestSerializeContentOpsRoundTrip(t *testing.T) {
	input := []byte("BT\n/F1 12 Tf\n100 700 Td\n(Hello World) Tj\nET\n")
	ops := ParseContentStream(input)
	if len(ops) == 0 {
		t.Fatal("expected parsed ops")
	}
	output := serializeContentOps(ops)
	// Re-parse the output and verify same operators.
	ops2 := ParseContentStream(output)
	if len(ops2) != len(ops) {
		t.Fatalf("round-trip op count: got %d, want %d", len(ops2), len(ops))
	}
	for i := range ops {
		if ops[i].Operator != ops2[i].Operator {
			t.Errorf("op %d: got %q, want %q", i, ops2[i].Operator, ops[i].Operator)
		}
	}
}

// TestRectsOverlap verifies rectangle overlap detection.
func TestRectsOverlap(t *testing.T) {
	tests := []struct {
		name string
		a, b Box
		want bool
	}{
		{"overlapping", Box{0, 0, 10, 10}, Box{5, 5, 15, 15}, true},
		{"adjacent", Box{0, 0, 10, 10}, Box{10, 0, 20, 10}, false},
		{"contained", Box{0, 0, 20, 20}, Box{5, 5, 15, 15}, true},
		{"separate", Box{0, 0, 5, 5}, Box{10, 10, 20, 20}, false},
		{"zero box", Box{}, Box{5, 5, 15, 15}, false},
	}
	for _, tt := range tests {
		if got := rectsOverlap(tt.a, tt.b); got != tt.want {
			t.Errorf("%s: rectsOverlap = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// TestRedactRegions verifies that region-based redaction removes text
// from the specified area and the output is a valid PDF.
func TestRedactRegions(t *testing.T) {
	pdf := createTestPDF(t, "Secret data here", "Public information")
	r, err := Parse(pdf)
	if err != nil {
		t.Fatal(err)
	}

	// Redact the top of the page where "Secret data here" is rendered.
	marks := []RedactionMark{{
		Page: 0,
		Rect: Box{X1: 0, Y1: 680, X2: 612, Y2: 800},
	}}

	m, err := RedactRegions(r, marks, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// Re-parse and verify.
	result, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("parse redacted PDF: %v", err)
	}
	if result.PageCount() != 1 {
		t.Errorf("page count: got %d, want 1", result.PageCount())
	}

	// Extract text — "Secret data here" should be gone.
	page, _ := result.Page(0)
	text, _ := page.ExtractText()
	if strings.Contains(text, "Secret") {
		t.Error("redacted text 'Secret' still extractable from output")
	}
}

// TestRedactText verifies text-search-based redaction.
func TestRedactText(t *testing.T) {
	pdf := createTestPDF(t, "My SSN is 123-45-6789 and my name is John.")
	r, err := Parse(pdf)
	if err != nil {
		t.Fatal(err)
	}

	m, err := RedactText(r, []string{"123-45-6789"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	result, _ := Parse(buf.Bytes())
	page, _ := result.Page(0)
	text, _ := page.ExtractText()

	if strings.Contains(text, "123-45-6789") {
		t.Error("redacted SSN still extractable")
	}
	if !strings.Contains(text, "name") {
		t.Error("non-redacted text 'name' should be preserved")
	}
}

// TestRedactPattern verifies regex-based redaction.
func TestRedactPattern(t *testing.T) {
	pdf := createTestPDF(t, "Contact: 555-123-4567 or 555-987-6543")
	r, err := Parse(pdf)
	if err != nil {
		t.Fatal(err)
	}

	pattern := regexp.MustCompile(`\d{3}-\d{3}-\d{4}`)
	m, err := RedactPattern(r, pattern, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	result, _ := Parse(buf.Bytes())
	page, _ := result.Page(0)
	text, _ := page.ExtractText()

	if strings.Contains(text, "555-123-4567") {
		t.Error("first phone number still extractable")
	}
	if strings.Contains(text, "555-987-6543") {
		t.Error("second phone number still extractable")
	}
}

// TestRedactWithOverlay verifies that overlay text appears in the output.
func TestRedactWithOverlay(t *testing.T) {
	pdf := createTestPDF(t, "Classified information")
	r, _ := Parse(pdf)

	m, err := RedactRegions(r, []RedactionMark{{
		Page: 0, Rect: Box{X1: 0, Y1: 680, X2: 612, Y2: 800},
	}}, &RedactOptions{
		OverlayText: "REDACTED",
	})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// The overlay text should be extractable from the output.
	result, _ := Parse(buf.Bytes())
	page, _ := result.Page(0)
	text, _ := page.ExtractText()
	if !strings.Contains(text, "REDACTED") {
		t.Errorf("overlay text 'REDACTED' not found in extracted text: %q", text)
	}
}

// TestRedactStripMetadata verifies metadata removal.
func TestRedactStripMetadata(t *testing.T) {
	pdf := createTestPDF(t, "Content")
	r, _ := Parse(pdf)

	m, err := RedactRegions(r, nil, &RedactOptions{
		StripMetadata: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	result, _ := Parse(buf.Bytes())
	title, author, _, _, _ := result.Info()
	if title != "" {
		t.Errorf("expected empty title after metadata strip, got %q", title)
	}
	if author != "" {
		t.Errorf("expected empty author after metadata strip, got %q", author)
	}
}

// TestRedactEmptyMarks verifies that redacting with no marks produces
// a valid copy of the original PDF.
func TestRedactEmptyMarks(t *testing.T) {
	pdf := createTestPDF(t, "Unchanged text")
	r, _ := Parse(pdf)

	m, err := RedactRegions(r, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	result, _ := Parse(buf.Bytes())
	if result.PageCount() != 1 {
		t.Errorf("page count: got %d, want 1", result.PageCount())
	}
}

// TestRedactMultiPage verifies redaction across multiple pages.
func TestRedactMultiPage(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Add(layout.NewParagraph("Page 1 secret", font.Helvetica, 12))
	doc.Add(layout.NewAreaBreak())
	doc.Add(layout.NewParagraph("Page 2 secret", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	r, _ := Parse(buf.Bytes())

	// Redact top of both pages.
	marks := []RedactionMark{
		{Page: 0, Rect: Box{X1: 0, Y1: 680, X2: 612, Y2: 800}},
		{Page: 1, Rect: Box{X1: 0, Y1: 680, X2: 612, Y2: 800}},
	}

	m, err := RedactRegions(r, marks, nil)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if _, err := m.WriteTo(&out); err != nil {
		t.Fatal(err)
	}
	result, _ := Parse(out.Bytes())
	if result.PageCount() != 2 {
		t.Errorf("page count: got %d, want 2", result.PageCount())
	}
}

// TestRedactTextPreservesAdjacentContent verifies that text on the same
// line as the redaction target is preserved when using character-level
// splitting. Only the targeted characters should be removed.
func TestRedactTextPreservesAdjacentContent(t *testing.T) {
	pdf := createTestPDF(t,
		"Public report for Q4 2026.",
		"SSN: 123-45-6789 is confidential.",
		"Revenue reached $28.3M this quarter.",
	)
	r, _ := Parse(pdf)

	m, err := RedactText(r, []string{"123-45-6789"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	result, _ := Parse(buf.Bytes())
	page, _ := result.Page(0)
	text, _ := page.ExtractText()

	// Redacted text must be gone.
	if strings.Contains(text, "123-45-6789") {
		t.Error("SSN still extractable")
	}
	// Text on OTHER lines must survive.
	for _, want := range []string{"Public", "Revenue", "$28.3M"} {
		if !strings.Contains(text, want) {
			t.Errorf("preserved text %q not found in output", want)
		}
	}
}

// TestRedactTextSameLineSurvives verifies that non-targeted text on the
// same Tj operator as the target is preserved via character-level splitting.
func TestRedactTextSameLineSurvives(t *testing.T) {
	pdf := createTestPDF(t, "Contact: SSN 123-45-6789 end of line.")
	r, _ := Parse(pdf)

	m, err := RedactText(r, []string{"123-45-6789"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	result, _ := Parse(buf.Bytes())
	page, _ := result.Page(0)
	text, _ := page.ExtractText()

	if strings.Contains(text, "123-45-6789") {
		t.Error("SSN still extractable")
	}
	// "Contact" should survive — it's on the same line but before the SSN.
	if !strings.Contains(text, "Contact") {
		t.Error("text before redaction target should be preserved")
	}
}

// TestRedactTextCaseInsensitive verifies that text search is case-insensitive.
func TestRedactTextCaseInsensitive(t *testing.T) {
	pdf := createTestPDF(t, "CONFIDENTIAL report from the board.")
	r, _ := Parse(pdf)

	m, err := RedactText(r, []string{"confidential"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	result, _ := Parse(buf.Bytes())
	page, _ := result.Page(0)
	text, _ := page.ExtractText()
	lower := strings.ToLower(text)

	if strings.Contains(lower, "confidential") {
		t.Error("case-insensitive match 'confidential' still extractable")
	}
	if !strings.Contains(lower, "board") {
		t.Error("non-redacted text 'board' should be preserved")
	}
}

// TestRedactPatternSSN verifies realistic SSN pattern redaction.
func TestRedactPatternSSN(t *testing.T) {
	pdf := createTestPDF(t,
		"Employee: Jane Doe, SSN: 987-65-4321",
		"Employer ID: 12-3456789",
	)
	r, _ := Parse(pdf)

	// SSN pattern: ###-##-####
	ssnPattern := regexp.MustCompile(`\d{3}-\d{2}-\d{4}`)
	m, err := RedactPattern(r, ssnPattern, &RedactOptions{
		FillColor:   [3]float64{0, 0, 0},
		OverlayText: "[SSN REDACTED]",
	})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	result, _ := Parse(buf.Bytes())
	page, _ := result.Page(0)
	text, _ := page.ExtractText()

	if strings.Contains(text, "987-65-4321") {
		t.Error("SSN still extractable")
	}
	// The EIN (12-3456789) should NOT match the SSN pattern.
	if !strings.Contains(text, "Employer") {
		t.Error("non-matching text should be preserved")
	}
	// Overlay text should appear.
	if !strings.Contains(text, "[SSN REDACTED]") {
		t.Error("overlay text not found")
	}
}

// TestRedactWithCustomColor verifies custom fill color.
func TestRedactWithCustomColor(t *testing.T) {
	pdf := createTestPDF(t, "Redact me")
	r, _ := Parse(pdf)

	// Use white fill (unusual but valid).
	m, err := RedactRegions(r, []RedactionMark{{
		Page: 0, Rect: Box{X1: 0, Y1: 680, X2: 612, Y2: 800},
	}}, &RedactOptions{
		FillColor: [3]float64{1, 1, 1},
	})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// Should produce a valid PDF.
	result, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}
	if result.PageCount() != 1 {
		t.Error("expected 1 page")
	}
}

// TestRedactOutOfRangePageIgnored verifies that marks referencing
// non-existent pages are silently skipped.
func TestRedactOutOfRangePageIgnored(t *testing.T) {
	pdf := createTestPDF(t, "Content")
	r, _ := Parse(pdf)

	m, err := RedactRegions(r, []RedactionMark{{
		Page: 99, Rect: Box{X1: 0, Y1: 0, X2: 100, Y2: 100},
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	result, _ := Parse(buf.Bytes())
	if result.PageCount() != 1 {
		t.Error("expected 1 page")
	}
}

// TestRedactNilPattern verifies that a nil regex produces a copy.
func TestRedactNilPattern(t *testing.T) {
	pdf := createTestPDF(t, "Content")
	r, _ := Parse(pdf)

	m, err := RedactPattern(r, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := m.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	result, _ := Parse(buf.Bytes())
	if result.PageCount() != 1 {
		t.Error("expected 1 page")
	}
}
