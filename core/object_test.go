// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"compress/zlib"
	"io"
	"strings"
	"testing"
)

// helper: serialize a PdfObject to a string.
func serialize(t *testing.T, obj PdfObject) string {
	t.Helper()
	var buf bytes.Buffer
	n, err := obj.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	if int64(buf.Len()) != n {
		t.Fatalf("WriteTo returned n=%d but buffer has %d bytes", n, buf.Len())
	}
	return buf.String()
}

// --- PdfBoolean ---

func TestBooleanTrue(t *testing.T) {
	got := serialize(t, NewPdfBoolean(true))
	if got != "true" {
		t.Errorf("expected %q, got %q", "true", got)
	}
}

func TestBooleanFalse(t *testing.T) {
	got := serialize(t, NewPdfBoolean(false))
	if got != "false" {
		t.Errorf("expected %q, got %q", "false", got)
	}
}

func TestBooleanType(t *testing.T) {
	b := NewPdfBoolean(true)
	if b.Type() != ObjectTypeBoolean {
		t.Errorf("expected ObjectTypeBoolean, got %v", b.Type())
	}
}

// --- PdfNumber ---

func TestIntegerPositive(t *testing.T) {
	got := serialize(t, NewPdfInteger(42))
	if got != "42" {
		t.Errorf("expected %q, got %q", "42", got)
	}
}

func TestIntegerZero(t *testing.T) {
	got := serialize(t, NewPdfInteger(0))
	if got != "0" {
		t.Errorf("expected %q, got %q", "0", got)
	}
}

func TestIntegerNegative(t *testing.T) {
	got := serialize(t, NewPdfInteger(-17))
	if got != "-17" {
		t.Errorf("expected %q, got %q", "-17", got)
	}
}

func TestRealSimple(t *testing.T) {
	got := serialize(t, NewPdfReal(3.14))
	if got != "3.14" {
		t.Errorf("expected %q, got %q", "3.14", got)
	}
}

func TestRealWholeNumber(t *testing.T) {
	// A real that happens to be a whole number should still show a decimal
	got := serialize(t, NewPdfReal(5.0))
	if got != "5.0" {
		t.Errorf("expected %q, got %q", "5.0", got)
	}
}

func TestRealNegative(t *testing.T) {
	got := serialize(t, NewPdfReal(-0.5))
	if got != "-0.5" {
		t.Errorf("expected %q, got %q", "-0.5", got)
	}
}

func TestNumberType(t *testing.T) {
	if NewPdfInteger(1).Type() != ObjectTypeNumber {
		t.Error("integer should have ObjectTypeNumber")
	}
	if NewPdfReal(1.0).Type() != ObjectTypeNumber {
		t.Error("real should have ObjectTypeNumber")
	}
}

func TestNumberIsInteger(t *testing.T) {
	if !NewPdfInteger(5).IsInteger() {
		t.Error("expected IsInteger true")
	}
	if NewPdfReal(5.0).IsInteger() {
		t.Error("expected IsInteger false for real")
	}
}

func TestNumberAccessors(t *testing.T) {
	n := NewPdfInteger(42)
	if n.IntValue() != 42 {
		t.Errorf("IntValue: expected 42, got %d", n.IntValue())
	}
	if n.FloatValue() != 42.0 {
		t.Errorf("FloatValue: expected 42.0, got %f", n.FloatValue())
	}
}

// --- PdfString ---

func TestLiteralStringSimple(t *testing.T) {
	got := serialize(t, NewPdfLiteralString("Hello"))
	if got != "(Hello)" {
		t.Errorf("expected %q, got %q", "(Hello)", got)
	}
}

func TestLiteralStringEmpty(t *testing.T) {
	got := serialize(t, NewPdfLiteralString(""))
	if got != "()" {
		t.Errorf("expected %q, got %q", "()", got)
	}
}

func TestLiteralStringEscaping(t *testing.T) {
	got := serialize(t, NewPdfLiteralString(`a\b(c)d`))
	if got != `(a\\b\(c\)d)` {
		t.Errorf("expected %q, got %q", `(a\\b\(c\)d)`, got)
	}
}

func TestLiteralStringNewlines(t *testing.T) {
	got := serialize(t, NewPdfLiteralString("line1\nline2\r"))
	if got != `(line1\nline2\r)` {
		t.Errorf("expected %q, got %q", `(line1\nline2\r)`, got)
	}
}

func TestHexStringSimple(t *testing.T) {
	got := serialize(t, NewPdfHexString("Hello"))
	if got != "<48656C6C6F>" {
		t.Errorf("expected %q, got %q", "<48656C6C6F>", got)
	}
}

func TestHexStringEmpty(t *testing.T) {
	got := serialize(t, NewPdfHexString(""))
	if got != "<>" {
		t.Errorf("expected %q, got %q", "<>", got)
	}
}

func TestStringType(t *testing.T) {
	if NewPdfLiteralString("x").Type() != ObjectTypeString {
		t.Error("literal string should have ObjectTypeString")
	}
	if NewPdfHexString("x").Type() != ObjectTypeString {
		t.Error("hex string should have ObjectTypeString")
	}
}

// --- PdfName ---

func TestNameSimple(t *testing.T) {
	got := serialize(t, NewPdfName("Type"))
	if got != "/Type" {
		t.Errorf("expected %q, got %q", "/Type", got)
	}
}

func TestNameWithSpace(t *testing.T) {
	got := serialize(t, NewPdfName("A B"))
	if got != "/A#20B" {
		t.Errorf("expected %q, got %q", "/A#20B", got)
	}
}

func TestNameWithHash(t *testing.T) {
	got := serialize(t, NewPdfName("A#B"))
	if got != "/A#23B" {
		t.Errorf("expected %q, got %q", "/A#23B", got)
	}
}

func TestNameWithDelimiters(t *testing.T) {
	got := serialize(t, NewPdfName("A(B"))
	if got != "/A#28B" {
		t.Errorf("expected %q, got %q", "/A#28B", got)
	}
}

func TestNameType(t *testing.T) {
	if NewPdfName("X").Type() != ObjectTypeName {
		t.Error("expected ObjectTypeName")
	}
}

// --- PdfArray ---

func TestArrayEmpty(t *testing.T) {
	got := serialize(t, NewPdfArray())
	if got != "[]" {
		t.Errorf("expected %q, got %q", "[]", got)
	}
}

func TestArraySingleElement(t *testing.T) {
	got := serialize(t, NewPdfArray(NewPdfInteger(42)))
	if got != "[42]" {
		t.Errorf("expected %q, got %q", "[42]", got)
	}
}

func TestArrayMultipleElements(t *testing.T) {
	a := NewPdfArray(
		NewPdfInteger(1),
		NewPdfReal(2.5),
		NewPdfLiteralString("hi"),
	)
	got := serialize(t, a)
	if got != "[1 2.5 (hi)]" {
		t.Errorf("expected %q, got %q", "[1 2.5 (hi)]", got)
	}
}

func TestArrayNested(t *testing.T) {
	inner := NewPdfArray(NewPdfInteger(1), NewPdfInteger(2))
	outer := NewPdfArray(inner, NewPdfInteger(3))
	got := serialize(t, outer)
	if got != "[[1 2] 3]" {
		t.Errorf("expected %q, got %q", "[[1 2] 3]", got)
	}
}

func TestArrayAdd(t *testing.T) {
	a := NewPdfArray()
	a.Add(NewPdfInteger(1))
	a.Add(NewPdfInteger(2))
	if a.Len() != 2 {
		t.Errorf("expected len 2, got %d", a.Len())
	}
	got := serialize(t, a)
	if got != "[1 2]" {
		t.Errorf("expected %q, got %q", "[1 2]", got)
	}
}

func TestArrayType(t *testing.T) {
	if NewPdfArray().Type() != ObjectTypeArray {
		t.Error("expected ObjectTypeArray")
	}
}

// --- PdfDictionary ---

func TestDictionaryEmpty(t *testing.T) {
	got := serialize(t, NewPdfDictionary())
	if got != "<< >>" {
		t.Errorf("expected %q, got %q", "<< >>", got)
	}
}

func TestDictionarySingleEntry(t *testing.T) {
	d := NewPdfDictionary()
	d.Set("Type", NewPdfName("Catalog"))
	got := serialize(t, d)
	if got != "<< /Type /Catalog >>" {
		t.Errorf("expected %q, got %q", "<< /Type /Catalog >>", got)
	}
}

func TestDictionaryMultipleEntries(t *testing.T) {
	d := NewPdfDictionary()
	d.Set("Type", NewPdfName("Page"))
	d.Set("Parent", NewPdfIndirectReference(2, 0))
	got := serialize(t, d)
	if got != "<< /Type /Page /Parent 2 0 R >>" {
		t.Errorf("expected %q, got %q", "<< /Type /Page /Parent 2 0 R >>", got)
	}
}

func TestDictionarySetOverwrite(t *testing.T) {
	d := NewPdfDictionary()
	d.Set("Count", NewPdfInteger(1))
	d.Set("Count", NewPdfInteger(5))
	got := serialize(t, d)
	if got != "<< /Count 5 >>" {
		t.Errorf("expected %q, got %q", "<< /Count 5 >>", got)
	}
}

func TestDictionaryGet(t *testing.T) {
	d := NewPdfDictionary()
	d.Set("X", NewPdfInteger(42))
	obj := d.Get("X")
	if obj == nil {
		t.Fatal("expected non-nil")
	}
	num, ok := obj.(*PdfNumber)
	if !ok {
		t.Fatal("expected *PdfNumber")
	}
	if num.IntValue() != 42 {
		t.Errorf("expected 42, got %d", num.IntValue())
	}
}

func TestDictionaryGetMissing(t *testing.T) {
	d := NewPdfDictionary()
	if d.Get("Nope") != nil {
		t.Error("expected nil for missing key")
	}
}

func TestDictionaryPreservesOrder(t *testing.T) {
	d := NewPdfDictionary()
	d.Set("B", NewPdfInteger(2))
	d.Set("A", NewPdfInteger(1))
	got := serialize(t, d)
	// B was inserted first, so it should appear first
	if got != "<< /B 2 /A 1 >>" {
		t.Errorf("expected %q, got %q", "<< /B 2 /A 1 >>", got)
	}
}

func TestDictionaryType(t *testing.T) {
	if NewPdfDictionary().Type() != ObjectTypeDictionary {
		t.Error("expected ObjectTypeDictionary")
	}
}

// --- PdfStream ---

func TestStreamSimple(t *testing.T) {
	s := NewPdfStream([]byte("Hello"))
	got := serialize(t, s)
	expected := "<< /Length 5 >>\nstream\nHello\nendstream"
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestStreamEmpty(t *testing.T) {
	s := NewPdfStream([]byte{})
	got := serialize(t, s)
	expected := "<< /Length 0 >>\nstream\n\nendstream"
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestStreamWithExtraDict(t *testing.T) {
	s := NewPdfStream([]byte("data"))
	s.Dict.Set("Subtype", NewPdfName("Image"))
	got := serialize(t, s)
	expected := "<< /Subtype /Image /Length 4 >>\nstream\ndata\nendstream"
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestStreamType(t *testing.T) {
	if NewPdfStream(nil).Type() != ObjectTypeStream {
		t.Error("expected ObjectTypeStream")
	}
}

func TestStreamCompressed(t *testing.T) {
	data := []byte("Hello World, this is some text to compress")
	s := NewPdfStreamCompressed(data)

	var buf bytes.Buffer
	_, err := s.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "/Filter /FlateDecode") {
		t.Error("compressed stream should have /Filter /FlateDecode")
	}
	// Compressed data should be smaller than original for non-trivial input
	if strings.Contains(got, "Hello World") {
		t.Error("compressed stream should not contain raw text")
	}
}

func TestStreamCompressedDecompresses(t *testing.T) {
	original := []byte("The quick brown fox jumps over the lazy dog. Repeated text. Repeated text. Repeated text.")
	s := NewPdfStreamCompressed(original)

	var buf bytes.Buffer
	_, err := s.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}

	// Extract the compressed bytes between "stream\n" and "\nendstream"
	raw := buf.String()
	streamStart := strings.Index(raw, "stream\n") + len("stream\n")
	streamEnd := strings.LastIndex(raw, "\nendstream")
	compressed := []byte(raw[streamStart:streamEnd])

	// Decompress and verify
	r, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("zlib.NewReader failed: %v", err)
	}
	defer r.Close()
	decompressed, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("decompress failed: %v", err)
	}
	if string(decompressed) != string(original) {
		t.Errorf("decompressed data mismatch:\nexpected: %q\ngot: %q", original, decompressed)
	}
}

func TestStreamSetCompress(t *testing.T) {
	s := NewPdfStream([]byte("data"))
	s.SetCompress(true)

	var buf bytes.Buffer
	_, err := s.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	if !strings.Contains(buf.String(), "/Filter /FlateDecode") {
		t.Error("SetCompress(true) should enable FlateDecode")
	}
}

func TestStreamUncompressedByDefault(t *testing.T) {
	s := NewPdfStream([]byte("raw data"))

	var buf bytes.Buffer
	_, err := s.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	if strings.Contains(buf.String(), "FlateDecode") {
		t.Error("stream should not be compressed by default")
	}
	if !strings.Contains(buf.String(), "raw data") {
		t.Error("uncompressed stream should contain raw data")
	}
}

// --- PdfNull ---

func TestNull(t *testing.T) {
	got := serialize(t, NewPdfNull())
	if got != "null" {
		t.Errorf("expected %q, got %q", "null", got)
	}
}

func TestNullType(t *testing.T) {
	if NewPdfNull().Type() != ObjectTypeNull {
		t.Error("expected ObjectTypeNull")
	}
}

// --- PdfIndirectReference ---

func TestReferenceSimple(t *testing.T) {
	got := serialize(t, NewPdfIndirectReference(1, 0))
	if got != "1 0 R" {
		t.Errorf("expected %q, got %q", "1 0 R", got)
	}
}

func TestReferenceHighNumbers(t *testing.T) {
	got := serialize(t, NewPdfIndirectReference(100, 2))
	if got != "100 2 R" {
		t.Errorf("expected %q, got %q", "100 2 R", got)
	}
}

func TestReferenceType(t *testing.T) {
	if NewPdfIndirectReference(1, 0).Type() != ObjectTypeReference {
		t.Error("expected ObjectTypeReference")
	}
}

// --- Additional coverage tests ---

func TestEmptyArrayWriteTo(t *testing.T) {
	a := NewPdfArray()
	got := serialize(t, a)
	if got != "[]" {
		t.Errorf("expected %q, got %q", "[]", got)
	}
	// Verify the byte count is exactly 2 (for "[" and "]")
	var buf bytes.Buffer
	n, err := a.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	if n != 2 {
		t.Errorf("expected WriteTo to return 2, got %d", n)
	}
}

func TestEscapeLiteralStringCarriageReturn(t *testing.T) {
	got := EscapeLiteralString("before\rafter")
	expected := `before\rafter`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestEscapeLiteralStringTab(t *testing.T) {
	got := EscapeLiteralString("col1\tcol2")
	expected := `col1\tcol2`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestUncompressedStreamWriteTo(t *testing.T) {
	data := []byte("raw content here")
	s := NewPdfStream(data)
	var buf bytes.Buffer
	n, err := s.WriteTo(&buf)
	if err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	got := buf.String()
	expected := "<< /Length 16 >>\nstream\nraw content here\nendstream"
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
	if n != int64(len(expected)) {
		t.Errorf("expected n=%d, got n=%d", len(expected), n)
	}
	// Verify no filter is set
	if strings.Contains(got, "FlateDecode") {
		t.Error("uncompressed stream should not contain FlateDecode")
	}
}
