// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// generateTestPDF creates a PDF in memory for round-trip testing.
func generateTestPDF(t *testing.T) []byte {
	t.Helper()
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Test Document"
	doc.Info.Author = "Folio Tests"

	p := doc.AddPage()
	p.AddText("Hello World", font.Helvetica, 12, 72, 700)

	p2 := doc.AddPage()
	p2.AddText("Page Two", font.Helvetica, 14, 72, 700)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("generate PDF: %v", err)
	}
	return buf.Bytes()
}

func generateLayoutPDF(t *testing.T) []byte {
	t.Helper()
	doc := document.NewDocument(document.PageSizeA4)
	doc.Info.Title = "Layout Test"
	doc.Info.Author = "Folio"
	doc.Info.Subject = "Round-trip test"

	doc.Add(layout.NewHeading("Chapter One", layout.H1))
	doc.Add(layout.NewParagraph("First paragraph of content.", font.Helvetica, 12))
	doc.Add(layout.NewParagraph("Second paragraph with different text.", font.Helvetica, 12))

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("generate PDF: %v", err)
	}
	return buf.Bytes()
}

// --- Tokenizer tests ---

func TestTokenizerNumbers(t *testing.T) {
	tok := NewTokenizer([]byte("42 3.14 -7 +0"))
	tests := []struct {
		isInt bool
		intV  int64
		realV float64
	}{
		{true, 42, 42},
		{false, 0, 3.14},
		{true, -7, -7},
		{true, 0, 0},
	}
	for i, tt := range tests {
		token := tok.Next()
		if token.Type != TokenNumber {
			t.Errorf("token %d: type = %d, want TokenNumber", i, token.Type)
		}
		if token.IsInt != tt.isInt {
			t.Errorf("token %d: IsInt = %v, want %v", i, token.IsInt, tt.isInt)
		}
		if tt.isInt && token.Int != tt.intV {
			t.Errorf("token %d: Int = %d, want %d", i, token.Int, tt.intV)
		}
	}
}

func TestTokenizerStrings(t *testing.T) {
	tok := NewTokenizer([]byte(`(Hello World) (nested (parens)) (escape\n\t\\)`))
	tests := []string{"Hello World", "nested (parens)", "escape\n\t\\"}
	for i, want := range tests {
		token := tok.Next()
		if token.Type != TokenString {
			t.Errorf("token %d: type = %d, want TokenString", i, token.Type)
		}
		if token.Value != want {
			t.Errorf("token %d: value = %q, want %q", i, token.Value, want)
		}
	}
}

func TestTokenizerHexStrings(t *testing.T) {
	tok := NewTokenizer([]byte("<48656C6C6F>"))
	token := tok.Next()
	if token.Type != TokenHexString {
		t.Fatalf("type = %d, want TokenHexString", token.Type)
	}
	if token.Value != "Hello" {
		t.Errorf("value = %q, want %q", token.Value, "Hello")
	}
}

func TestTokenizerNames(t *testing.T) {
	tok := NewTokenizer([]byte("/Type /Pages /Name#20With#20Spaces"))
	tests := []string{"Type", "Pages", "Name With Spaces"}
	for i, want := range tests {
		token := tok.Next()
		if token.Type != TokenName {
			t.Errorf("token %d: type = %d, want TokenName", i, token.Type)
		}
		if token.Value != want {
			t.Errorf("token %d: value = %q, want %q", i, token.Value, want)
		}
	}
}

func TestTokenizerBoolNull(t *testing.T) {
	tok := NewTokenizer([]byte("true false null"))
	t1 := tok.Next()
	if t1.Type != TokenBool || t1.Value != "true" {
		t.Errorf("expected true, got %+v", t1)
	}
	t2 := tok.Next()
	if t2.Type != TokenBool || t2.Value != "false" {
		t.Errorf("expected false, got %+v", t2)
	}
	t3 := tok.Next()
	if t3.Type != TokenNull {
		t.Errorf("expected null, got %+v", t3)
	}
}

func TestTokenizerDictAndArray(t *testing.T) {
	tok := NewTokenizer([]byte("<< /Type /Page /Kids [1 0 R] >>"))
	tests := []TokenType{TokenDictOpen, TokenName, TokenName, TokenName, TokenArrayOpen, TokenNumber, TokenNumber, TokenKeyword, TokenArrayClose, TokenDictClose}
	for i, want := range tests {
		token := tok.Next()
		if token.Type != want {
			t.Errorf("token %d: type = %d, want %d (value=%q)", i, token.Type, want, token.Value)
		}
	}
}

func TestTokenizerComments(t *testing.T) {
	tok := NewTokenizer([]byte("42 % this is a comment\n43"))
	t1 := tok.Next()
	if t1.Int != 42 {
		t.Errorf("expected 42, got %d", t1.Int)
	}
	t2 := tok.Next()
	if t2.Int != 43 {
		t.Errorf("expected 43, got %d", t2.Int)
	}
}

func TestTokenizerPeek(t *testing.T) {
	tok := NewTokenizer([]byte("42 43"))
	peek := tok.Peek()
	if peek.Int != 42 {
		t.Errorf("peek = %d, want 42", peek.Int)
	}
	next := tok.Next()
	if next.Int != 42 {
		t.Errorf("next after peek = %d, want 42", next.Int)
	}
}

// --- Parser tests ---

func TestParserDictionary(t *testing.T) {
	tok := NewTokenizer([]byte("<< /Type /Catalog /Pages 2 0 R >>"))
	p := NewParser(tok)
	obj, err := p.ParseObject()
	if err != nil {
		t.Fatal(err)
	}
	dict, ok := obj.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("expected PdfDictionary, got %T", obj)
	}
	typeObj := dict.Get("Type")
	if typeObj == nil {
		t.Fatal("missing /Type")
	}
	if name, ok := typeObj.(*core.PdfName); !ok || name.Value != "Catalog" {
		t.Errorf("/Type = %v, want /Catalog", typeObj)
	}
}

func TestParserArray(t *testing.T) {
	tok := NewTokenizer([]byte("[1 2 3 (hello) /Name]"))
	p := NewParser(tok)
	obj, err := p.ParseObject()
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := obj.(*core.PdfArray)
	if !ok {
		t.Fatalf("expected PdfArray, got %T", obj)
	}
	if arr.Len() != 5 {
		t.Errorf("array length = %d, want 5", arr.Len())
	}
}

func TestParserIndirectReference(t *testing.T) {
	tok := NewTokenizer([]byte("5 0 R"))
	p := NewParser(tok)
	obj, err := p.ParseObject()
	if err != nil {
		t.Fatal(err)
	}
	ref, ok := obj.(*core.PdfIndirectReference)
	if !ok {
		t.Fatalf("expected PdfIndirectReference, got %T", obj)
	}
	if ref.ObjectNumber != 5 {
		t.Errorf("objNum = %d, want 5", ref.ObjectNumber)
	}
}

func TestParserIndirectObject(t *testing.T) {
	tok := NewTokenizer([]byte("3 0 obj\n<< /Type /Page >>\nendobj"))
	p := NewParser(tok)
	objNum, genNum, obj, err := p.ParseIndirectObject()
	if err != nil {
		t.Fatal(err)
	}
	if objNum != 3 || genNum != 0 {
		t.Errorf("got %d %d, want 3 0", objNum, genNum)
	}
	if _, ok := obj.(*core.PdfDictionary); !ok {
		t.Errorf("expected dict, got %T", obj)
	}
}

// --- Round-trip tests ---

func TestRoundTripBasic(t *testing.T) {
	data := generateTestPDF(t)
	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if r.PageCount() != 2 {
		t.Errorf("PageCount = %d, want 2", r.PageCount())
	}
}

func TestRoundTripVersion(t *testing.T) {
	data := generateTestPDF(t)
	r, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	v := r.Version()
	if v != "1.7" {
		t.Errorf("Version = %q, want 1.7", v)
	}
}

func TestRoundTripInfo(t *testing.T) {
	data := generateTestPDF(t)
	r, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	title, author, _, _, _ := r.Info()
	if title != "Test Document" {
		t.Errorf("Title = %q, want %q", title, "Test Document")
	}
	if author != "Folio Tests" {
		t.Errorf("Author = %q, want %q", author, "Folio Tests")
	}
}

func TestRoundTripPageDimensions(t *testing.T) {
	data := generateTestPDF(t)
	r, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatal(err)
	}
	// Letter size: 612 x 792.
	if page.Width != 612 {
		t.Errorf("Width = %.1f, want 612", page.Width)
	}
	if page.Height != 792 {
		t.Errorf("Height = %.1f, want 792", page.Height)
	}
}

func TestRoundTripPageOutOfRange(t *testing.T) {
	data := generateTestPDF(t)
	r, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	_, err = r.Page(5)
	if err == nil {
		t.Error("expected error for out-of-range page")
	}
}

func TestRoundTripCatalog(t *testing.T) {
	data := generateTestPDF(t)
	r, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	cat := r.Catalog()
	if cat == nil {
		t.Fatal("Catalog is nil")
	}
	typeObj := cat.Get("Type")
	if typeObj == nil {
		t.Fatal("Catalog missing /Type")
	}
}

func TestRoundTripLayoutPDF(t *testing.T) {
	data := generateLayoutPDF(t)
	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse layout PDF failed: %v", err)
	}
	if r.PageCount() < 1 {
		t.Error("expected at least 1 page")
	}
	title, _, _, _, _ := r.Info()
	if title != "Layout Test" {
		t.Errorf("Title = %q, want %q", title, "Layout Test")
	}
}

func TestRoundTripContentStream(t *testing.T) {
	data := generateTestPDF(t)
	r, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := r.Page(0)
	content, err := page.ContentStream()
	if err != nil {
		t.Fatalf("ContentStream: %v", err)
	}
	if len(content) == 0 {
		t.Error("expected non-empty content stream")
	}
	// Should contain text operators.
	if !strings.Contains(string(content), "Tj") {
		t.Error("content stream should contain Tj operator")
	}
}

func TestRoundTripResources(t *testing.T) {
	data := generateTestPDF(t)
	r, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := r.Page(0)
	res, err := page.Resources()
	if err != nil {
		t.Fatalf("Resources: %v", err)
	}
	if res == nil {
		t.Fatal("Resources is nil")
	}
	// Should have a /Font dictionary.
	fontObj := res.Get("Font")
	if fontObj == nil {
		t.Error("expected /Font in resources")
	}
}

func TestRoundTripMultiPage(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Multi-page"
	for i := range 10 {
		p := doc.AddPage()
		p.AddText(strings.Repeat("X", i+1), font.Helvetica, 12, 72, 700)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if r.PageCount() != 10 {
		t.Errorf("PageCount = %d, want 10", r.PageCount())
	}
}

func TestRoundTripA4Size(t *testing.T) {
	doc := document.NewDocument(document.PageSizeA4)
	doc.AddPage()

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	r, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	page, _ := r.Page(0)

	// A4: 595.28 x 841.89
	if page.Width < 595 || page.Width > 596 {
		t.Errorf("A4 width = %.2f, want ~595.28", page.Width)
	}
	if page.Height < 841 || page.Height > 842 {
		t.Errorf("A4 height = %.2f, want ~841.89", page.Height)
	}
}

// --- Edge cases ---

func TestParseInvalidPDF(t *testing.T) {
	_, err := Parse([]byte("not a pdf"))
	if err == nil {
		t.Error("expected error for invalid PDF")
	}
}

func TestParseTooSmall(t *testing.T) {
	_, err := Parse([]byte("abc"))
	if err == nil {
		t.Error("expected error for tiny input")
	}
}

func TestTokenizerEOF(t *testing.T) {
	tok := NewTokenizer([]byte(""))
	token := tok.Next()
	if token.Type != TokenEOF {
		t.Errorf("expected EOF, got type %d", token.Type)
	}
}

// --- Xref stream tests ---

func TestRoundTripXrefStream(t *testing.T) {
	// Generate a PDF, convert with qpdf to use object/xref streams,
	// then read it back.
	qpdfPath, err := exec.LookPath("qpdf")
	if err != nil {
		t.Skip("qpdf not installed, skipping xref stream test")
	}

	// Generate a multi-page PDF.
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Xref Stream Test"
	for range 3 {
		p := doc.AddPage()
		p.AddText("Xref stream test page", font.Helvetica, 12, 72, 700)
	}
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// Write to temp file.
	tmpDir := t.TempDir()
	inputPath := tmpDir + "/input.pdf"
	outputPath := tmpDir + "/xrefstream.pdf"
	if err := os.WriteFile(inputPath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	// Convert with qpdf to force object streams (which use xref streams).
	cmd := exec.Command(qpdfPath, inputPath, outputPath, "--object-streams=generate")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("qpdf conversion failed: %v\n%s", err, out)
	}

	// Read the xref-stream PDF.
	xrefData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	r, err := Parse(xrefData)
	if err != nil {
		t.Fatalf("Parse xref stream PDF failed: %v", err)
	}

	if r.PageCount() != 3 {
		t.Errorf("PageCount = %d, want 3", r.PageCount())
	}

	title, _, _, _, _ := r.Info()
	if title != "Xref Stream Test" {
		t.Errorf("Title = %q, want %q", title, "Xref Stream Test")
	}

	page, _ := r.Page(0)
	if page.Width != 612 || page.Height != 792 {
		t.Errorf("page size = %.0fx%.0f, want 612x792", page.Width, page.Height)
	}
}

func TestRoundTripQdfFormat(t *testing.T) {
	// QDF is a human-readable PDF format produced by qpdf.
	// It uses classic xref (not streams), but tests our reader
	// against a different writer's output.
	qpdfPath, err := exec.LookPath("qpdf")
	if err != nil {
		t.Skip("qpdf not installed")
	}

	doc := document.NewDocument(document.PageSizeA4)
	doc.Info.Title = "QDF Test"
	p := doc.AddPage()
	p.AddText("Hello from QDF", font.Helvetica, 14, 72, 750)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	inputPath := tmpDir + "/input.pdf"
	qdfPath := tmpDir + "/output.qdf"
	if err := os.WriteFile(inputPath, buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(qpdfPath, "--qdf", "--object-streams=disable", inputPath, qdfPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("qpdf --qdf failed: %v\n%s", err, out)
	}

	qdfData, err := os.ReadFile(qdfPath)
	if err != nil {
		t.Fatal(err)
	}

	r, err := Parse(qdfData)
	if err != nil {
		t.Fatalf("Parse QDF failed: %v", err)
	}

	if r.PageCount() != 1 {
		t.Errorf("PageCount = %d, want 1", r.PageCount())
	}

	title, _, _, _, _ := r.Info()
	if title != "QDF Test" {
		t.Errorf("Title = %q, want %q", title, "QDF Test")
	}
}

// --- Content stream parser tests ---

func TestContentStreamParser(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n100 700 Td\n(Hello World) Tj\nET")
	ops := ParseContentStream(data)

	if len(ops) != 5 {
		t.Fatalf("expected 5 operators, got %d", len(ops))
	}

	if ops[0].Operator != "BT" {
		t.Errorf("op 0 = %q, want BT", ops[0].Operator)
	}
	if ops[1].Operator != "Tf" {
		t.Errorf("op 1 = %q, want Tf", ops[1].Operator)
	}
	if len(ops[1].Operands) != 2 {
		t.Errorf("Tf should have 2 operands, got %d", len(ops[1].Operands))
	}
	if ops[3].Operator != "Tj" {
		t.Errorf("op 3 = %q, want Tj", ops[3].Operator)
	}
}

func TestExtractTextBasic(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n100 700 Td\n(Hello World) Tj\nET")
	text := ExtractText(data)
	if text != "Hello World" {
		t.Errorf("text = %q, want %q", text, "Hello World")
	}
}

func TestExtractTextFromPDF(t *testing.T) {
	pdfData := generateTestPDF(t)
	r, err := Parse(pdfData)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := r.Page(0)
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(text, "Hello World") {
		t.Errorf("extracted text = %q, want to contain 'Hello World'", text)
	}
}

func TestContentOpsFromPDF(t *testing.T) {
	pdfData := generateTestPDF(t)
	r, err := Parse(pdfData)
	if err != nil {
		t.Fatal(err)
	}
	page, _ := r.Page(0)
	ops, err := page.ContentOps()
	if err != nil {
		t.Fatalf("ContentOps: %v", err)
	}
	if len(ops) == 0 {
		t.Error("expected at least one operator")
	}

	// Should have BT and ET.
	hasBT := false
	hasET := false
	for _, op := range ops {
		if op.Operator == "BT" {
			hasBT = true
		}
		if op.Operator == "ET" {
			hasET = true
		}
	}
	if !hasBT {
		t.Error("missing BT operator")
	}
	if !hasET {
		t.Error("missing ET operator")
	}
}

// --- Error recovery tests ---

func TestParseWithStrictness(t *testing.T) {
	data := generateTestPDF(t)
	r, err := ParseWithOptions(data, ReadOptions{Strictness: StrictnessStrict})
	if err != nil {
		t.Fatalf("strict parse failed: %v", err)
	}
	if r.PageCount() != 2 {
		t.Errorf("PageCount = %d, want 2", r.PageCount())
	}
}

func TestParseWithGarbageHeader(t *testing.T) {
	// Some PDFs have garbage bytes before the %PDF- header.
	data := generateTestPDF(t)
	garbage := append([]byte("GARBAGE_BYTES_"), data...)
	r, err := Parse(garbage)
	if err != nil {
		t.Fatalf("tolerant parse should handle garbage header: %v", err)
	}
	if r.PageCount() != 2 {
		t.Errorf("PageCount = %d, want 2", r.PageCount())
	}
}

// --- Cache eviction test ---

func TestCacheEviction(t *testing.T) {
	data := generateTestPDF(t)
	r, err := ParseWithOptions(data, ReadOptions{MaxCache: 5})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	// Should still work — eviction shouldn't break resolution.
	if r.PageCount() != 2 {
		t.Errorf("PageCount = %d, want 2", r.PageCount())
	}
}

// --- Memory limits tests ---

func TestMemoryLimitsDefault(t *testing.T) {
	// Default limits should allow normal PDFs to parse.
	data := generateTestPDF(t)
	r, err := ParseWithOptions(data, ReadOptions{})
	if err != nil {
		t.Fatalf("parse with default limits: %v", err)
	}
	if r.PageCount() != 2 {
		t.Errorf("PageCount = %d, want 2", r.PageCount())
	}
}

func TestMemoryLimitsDisabled(t *testing.T) {
	// Explicitly disabled limits (-1) should work.
	data := generateTestPDF(t)
	r, err := ParseWithOptions(data, ReadOptions{
		MemoryLimits: MemoryLimits{
			MaxStreamSize:  -1,
			MaxTotalAlloc:  -1,
			MaxXrefSize:    -1,
			MaxObjectCount: -1,
		},
	})
	if err != nil {
		t.Fatalf("parse with disabled limits: %v", err)
	}
	if r.PageCount() != 2 {
		t.Errorf("PageCount = %d, want 2", r.PageCount())
	}
}

func TestMemoryLimitsTinyStreamSize(t *testing.T) {
	// Use a layout PDF which has compressed content streams.
	data := generateLayoutPDF(t)

	// Set a very small stream limit — decompression should fail.
	r, err := ParseWithOptions(data, ReadOptions{
		MemoryLimits: MemoryLimits{
			MaxStreamSize: 10, // 10 bytes — way too small for any content stream
		},
	})
	if err != nil {
		// If parse itself fails, it should be a memory limit error.
		if !errors.Is(err, ErrMemoryLimitExceeded) {
			t.Fatalf("expected ErrMemoryLimitExceeded, got: %v", err)
		}
		return // expected failure — compressed streams exceeded limit
	}

	// If parse succeeded (classic xref, no compressed streams needed
	// during page tree walk), verify the reader still works.
	if r.PageCount() < 1 {
		t.Error("expected at least 1 page")
	}
}

func TestMemoryLimitsTinyTotalAlloc(t *testing.T) {
	// Create a PDF with multiple compressed streams.
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Alloc Limit Test"
	// Add enough content to create substantial compressed streams.
	for range 5 {
		doc.Add(layout.NewParagraph(
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit. "+
				"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
			font.Helvetica, 12))
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}

	// Parse with a generous per-stream limit but tiny total.
	_, err := ParseWithOptions(buf.Bytes(), ReadOptions{
		MemoryLimits: MemoryLimits{
			MaxStreamSize: 1 << 20, // 1 MB per stream — fine
			MaxTotalAlloc: 1,       // 1 byte total — way too small
		},
	})
	// Streams are decompressed during resolve, so this may or may not
	// trigger during parse depending on whether streams are accessed.
	// We just verify no panic occurs.
	_ = err
}

func TestMemoryLimitsMaxObjectCount(t *testing.T) {
	data := generateTestPDF(t)

	// Set max object count to 1 — should reject our PDF.
	_, err := ParseWithOptions(data, ReadOptions{
		MemoryLimits: MemoryLimits{
			MaxObjectCount: 1,
		},
	})
	if err == nil {
		t.Fatal("expected error for object count limit")
	}
	if !errors.Is(err, ErrMemoryLimitExceeded) {
		t.Fatalf("expected ErrMemoryLimitExceeded, got: %v", err)
	}
}

func TestMemoryLimitsReasonableValues(t *testing.T) {
	// Ensure custom but reasonable limits work.
	data := generateTestPDF(t)
	r, err := ParseWithOptions(data, ReadOptions{
		MemoryLimits: MemoryLimits{
			MaxStreamSize:  10 << 20, // 10 MB
			MaxTotalAlloc:  50 << 20, // 50 MB
			MaxObjectCount: 10000,
		},
	})
	if err != nil {
		t.Fatalf("parse with reasonable limits: %v", err)
	}
	if r.PageCount() != 2 {
		t.Errorf("PageCount = %d, want 2", r.PageCount())
	}
}

func TestLimitedReadAll(t *testing.T) {
	data := bytes.NewReader([]byte("hello world"))

	// Should succeed when limit is large enough.
	result, err := limitedReadAll(data, 100)
	if err != nil {
		t.Fatalf("limitedReadAll: %v", err)
	}
	if string(result) != "hello world" {
		t.Errorf("got %q, want %q", result, "hello world")
	}

	// Should fail when limit is too small.
	data.Reset([]byte("hello world"))
	_, err = limitedReadAll(data, 5)
	if err == nil {
		t.Fatal("expected error for small limit")
	}
	if !errors.Is(err, ErrMemoryLimitExceeded) {
		t.Fatalf("expected ErrMemoryLimitExceeded, got: %v", err)
	}

	// Should work with no limit (-1).
	data.Reset([]byte("hello world"))
	result, err = limitedReadAll(data, -1)
	if err != nil {
		t.Fatalf("limitedReadAll unlimited: %v", err)
	}
	if string(result) != "hello world" {
		t.Errorf("got %q, want %q", result, "hello world")
	}
}

func TestMemoryLimitsDefaults(t *testing.T) {
	ml := MemoryLimits{}
	if ml.effectiveMaxStreamSize() != defaultMaxStreamSize {
		t.Errorf("default stream size = %d, want %d", ml.effectiveMaxStreamSize(), defaultMaxStreamSize)
	}
	if ml.effectiveMaxTotalAlloc() != defaultMaxTotalAlloc {
		t.Errorf("default total alloc = %d, want %d", ml.effectiveMaxTotalAlloc(), defaultMaxTotalAlloc)
	}
	if ml.effectiveMaxXrefSize() != defaultMaxXrefSize {
		t.Errorf("default xref size = %d, want %d", ml.effectiveMaxXrefSize(), defaultMaxXrefSize)
	}
	if ml.effectiveMaxObjectCount() != defaultMaxObjectCount {
		t.Errorf("default object count = %d, want %d", ml.effectiveMaxObjectCount(), defaultMaxObjectCount)
	}
}

// Use core import to avoid unused import error.
var _ = core.NewPdfNull
