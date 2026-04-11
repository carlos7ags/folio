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

// decodeObjStm extracts the decompressed payload from an /ObjStm stream
// returned by BuildObjStm. Used to verify the byte-level layout.
func decodeObjStm(t *testing.T, s *PdfStream) []byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := s.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.Bytes()
	si := bytes.Index(out, []byte("\nstream\n"))
	ei := bytes.Index(out, []byte("\nendstream"))
	if si < 0 || ei < 0 {
		t.Fatalf("could not locate stream payload in %q", out)
	}
	compressed := out[si+len("\nstream\n") : ei]
	zr, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("zlib reader: %v", err)
	}
	decoded, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("zlib read: %v", err)
	}
	return decoded
}

func TestBuildObjStmRejectsEmpty(t *testing.T) {
	if _, err := BuildObjStm(nil); err == nil {
		t.Error("expected error for nil entries")
	}
	if _, err := BuildObjStm([]ObjStmEntry{}); err == nil {
		t.Error("expected error for empty entries")
	}
}

func TestBuildObjStmRejectsBadObjectNumber(t *testing.T) {
	cases := []int{0, -1}
	for _, n := range cases {
		_, err := BuildObjStm([]ObjStmEntry{{ObjectNumber: n, Object: NewPdfInteger(1)}})
		if err == nil {
			t.Errorf("expected error for object number %d", n)
		}
	}
}

func TestBuildObjStmRejectsNilObject(t *testing.T) {
	_, err := BuildObjStm([]ObjStmEntry{{ObjectNumber: 1, Object: nil}})
	if err == nil {
		t.Error("expected error for nil object")
	}
}

func TestBuildObjStmRejectsDuplicateObjectNumber(t *testing.T) {
	_, err := BuildObjStm([]ObjStmEntry{
		{ObjectNumber: 5, Object: NewPdfInteger(1)},
		{ObjectNumber: 5, Object: NewPdfInteger(2)},
	})
	if err == nil {
		t.Error("expected error for duplicate object number")
	}
}

func TestBuildObjStmRejectsStreamEntries(t *testing.T) {
	// §7.5.7: stream objects cannot be compressed inside another stream.
	innerStream := NewPdfStream([]byte("hello"))
	_, err := BuildObjStm([]ObjStmEntry{
		{ObjectNumber: 1, Object: innerStream},
	})
	if err == nil {
		t.Error("expected error for stream entry")
	}
}

func TestBuildObjStmSetsMandatoryDictEntries(t *testing.T) {
	entries := []ObjStmEntry{
		{ObjectNumber: 3, Object: NewPdfInteger(42)},
		{ObjectNumber: 4, Object: NewPdfName("Foo")},
	}
	stream, err := BuildObjStm(entries)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if name, ok := stream.Dict.Get("Type").(*PdfName); !ok || name.Value != "ObjStm" {
		t.Errorf("/Type = %v, want /ObjStm", stream.Dict.Get("Type"))
	}
	if n, ok := stream.Dict.Get("N").(*PdfNumber); !ok || n.IntValue() != 2 {
		t.Errorf("/N = %v, want 2", stream.Dict.Get("N"))
	}
	if stream.Dict.Get("First") == nil {
		t.Error("/First not set")
	}
}

func TestBuildObjStmHeaderLayout(t *testing.T) {
	// Verify the decoded payload has the documented (objNum offset)\n
	// layout for the header, /First points at the body block, and the
	// bodies appear in entry order separated by a single LF.
	entries := []ObjStmEntry{
		{ObjectNumber: 10, Object: NewPdfInteger(42)},      // body "42"
		{ObjectNumber: 11, Object: NewPdfName("Foo")},      // body "/Foo"
		{ObjectNumber: 12, Object: NewPdfInteger(1234567)}, // body "1234567"
	}
	stream, err := BuildObjStm(entries)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	decoded := decodeObjStm(t, stream)

	first := stream.Dict.Get("First").(*PdfNumber).IntValue()
	if first <= 0 || first > len(decoded) {
		t.Fatalf("/First = %d, decoded length = %d", first, len(decoded))
	}

	header := string(decoded[:first])
	// Header lines describe each entry as "objNum SP byteOffset LF".
	// Body offsets are relative to the start of the body block (after /First).
	wantLines := []string{
		"10 0\n",
		"11 3\n", // body "42" plus LF separator before "/Foo" → offset 3
		"12 8\n", // "/Foo" is 4 bytes, plus LF → offset 3+4+1 = 8
	}
	wantHeader := strings.Join(wantLines, "")
	if header != wantHeader {
		t.Errorf("header = %q, want %q", header, wantHeader)
	}

	bodies := string(decoded[first:])
	wantBodies := "42\n/Foo\n1234567"
	if bodies != wantBodies {
		t.Errorf("bodies = %q, want %q", bodies, wantBodies)
	}
}

func TestBuildObjStmSingleEntry(t *testing.T) {
	stream, err := BuildObjStm([]ObjStmEntry{
		{ObjectNumber: 1, Object: NewPdfInteger(7)},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	decoded := decodeObjStm(t, stream)
	first := stream.Dict.Get("First").(*PdfNumber).IntValue()

	if string(decoded[:first]) != "1 0\n" {
		t.Errorf("header = %q, want %q", decoded[:first], "1 0\n")
	}
	if string(decoded[first:]) != "7" {
		t.Errorf("body = %q, want %q", decoded[first:], "7")
	}
}

func TestBuildObjStmDictionaryEntry(t *testing.T) {
	// Dictionaries are valid direct objects and serialize via their own
	// WriteTo. Verify a dict body parses correctly out of the payload.
	d := NewPdfDictionary()
	d.Set("Type", NewPdfName("Catalog"))
	d.Set("Pages", NewPdfIndirectReference(2, 0))

	stream, err := BuildObjStm([]ObjStmEntry{{ObjectNumber: 1, Object: d}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	decoded := decodeObjStm(t, stream)
	first := stream.Dict.Get("First").(*PdfNumber).IntValue()
	body := string(decoded[first:])
	if !strings.Contains(body, "/Type /Catalog") {
		t.Errorf("body missing /Type /Catalog: %q", body)
	}
	if !strings.Contains(body, "/Pages 2 0 R") {
		t.Errorf("body missing /Pages 2 0 R: %q", body)
	}
}

func TestBuildObjStmDeterministic(t *testing.T) {
	build := func() *PdfStream {
		s, err := BuildObjStm([]ObjStmEntry{
			{ObjectNumber: 1, Object: NewPdfInteger(1)},
			{ObjectNumber: 2, Object: NewPdfName("A")},
			{ObjectNumber: 3, Object: NewPdfInteger(99)},
		})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	a := build()
	b := build()
	var bufA, bufB bytes.Buffer
	if _, err := a.WriteTo(&bufA); err != nil {
		t.Fatal(err)
	}
	if _, err := b.WriteTo(&bufB); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bufA.Bytes(), bufB.Bytes()) {
		t.Errorf("two builds produced different bytes:\nA=%x\nB=%x", bufA.Bytes(), bufB.Bytes())
	}
}
