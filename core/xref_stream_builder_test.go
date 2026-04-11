// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"compress/zlib"
	"io"
	"testing"
)

// minimalXRefSubsection builds a single dense subsection covering object
// numbers 0..n with the conventional free head and uncompressed entries
// at fabricated offsets. Used by multiple tests.
func minimalXRefSubsection(n int) XRefStreamSubsection {
	entries := make([]XRefStreamEntry, n+1)
	entries[0] = XRefStreamEntry{Type: XRefEntryFree, Field2: 0, Field3: 65535}
	for i := 1; i <= n; i++ {
		entries[i] = XRefStreamEntry{
			Type:   XRefEntryInUse,
			Field2: uint64(100 * i),
			Field3: 0,
		}
	}
	return XRefStreamSubsection{First: 0, Entries: entries}
}

func TestBuildXRefStreamRejectsEmpty(t *testing.T) {
	if _, err := BuildXRefStream(nil, 1, nil); err == nil {
		t.Error("expected error for empty subsections")
	}
}

func TestBuildXRefStreamRejectsZeroSize(t *testing.T) {
	subs := []XRefStreamSubsection{minimalXRefSubsection(0)}
	if _, err := BuildXRefStream(subs, 0, nil); err == nil {
		t.Error("expected error for zero size")
	}
}

func TestBuildXRefStreamRejectsOverflowingSubsection(t *testing.T) {
	subs := []XRefStreamSubsection{{
		First:   5,
		Entries: []XRefStreamEntry{{}, {}, {}}, // covers 5,6,7
	}}
	if _, err := BuildXRefStream(subs, 7, nil); err == nil {
		t.Error("expected error: subsection extends past size")
	}
}

func TestBuildXRefStreamSetsMandatoryDictEntries(t *testing.T) {
	subs := []XRefStreamSubsection{minimalXRefSubsection(2)}
	stream, err := BuildXRefStream(subs, 3, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if v := stream.Dict.Get("Type"); v == nil {
		t.Error("/Type not set")
	} else if name, ok := v.(*PdfName); !ok || name.Value != "XRef" {
		t.Errorf("/Type = %v, want /XRef", v)
	}
	if v := stream.Dict.Get("Size"); v == nil {
		t.Error("/Size not set")
	} else if n, ok := v.(*PdfNumber); !ok || n.IntValue() != 3 {
		t.Errorf("/Size = %v, want 3", v)
	}
	w := stream.Dict.Get("W")
	if w == nil {
		t.Fatal("/W not set")
	}
	arr, ok := w.(*PdfArray)
	if !ok || arr.Len() != 3 {
		t.Fatalf("/W = %v, want 3-element array", w)
	}
}

func TestBuildXRefStreamOmitsDefaultIndex(t *testing.T) {
	// Single subsection covering [0, size) — /Index is the default and
	// must not be written, per §7.5.8.2.
	subs := []XRefStreamSubsection{minimalXRefSubsection(2)}
	stream, err := BuildXRefStream(subs, 3, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if stream.Dict.Get("Index") != nil {
		t.Error("/Index should be omitted for default subsection layout")
	}
}

func TestBuildXRefStreamWritesNonDefaultIndex(t *testing.T) {
	// Sparse subsection: starts at 5, covers two objects, /Size is 10.
	subs := []XRefStreamSubsection{{
		First: 5,
		Entries: []XRefStreamEntry{
			{Type: XRefEntryInUse, Field2: 100},
			{Type: XRefEntryInUse, Field2: 200},
		},
	}}
	stream, err := BuildXRefStream(subs, 10, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	idx := stream.Dict.Get("Index")
	if idx == nil {
		t.Fatal("/Index not written for sparse layout")
	}
	arr, ok := idx.(*PdfArray)
	if !ok || arr.Len() != 2 {
		t.Fatalf("/Index = %v, want 2-element array", idx)
	}
	first, _ := arr.At(0).(*PdfNumber)
	count, _ := arr.At(1).(*PdfNumber)
	if first == nil || first.IntValue() != 5 {
		t.Errorf("/Index[0] = %v, want 5", arr.At(0))
	}
	if count == nil || count.IntValue() != 2 {
		t.Errorf("/Index[1] = %v, want 2", arr.At(1))
	}
}

func TestBuildXRefStreamPayloadRoundTrip(t *testing.T) {
	// Build a known-shaped xref stream, serialize it, decompress the
	// payload, and verify each entry decodes back to the input.
	entries := []XRefStreamEntry{
		{Type: XRefEntryFree, Field2: 0, Field3: 65535},
		{Type: XRefEntryInUse, Field2: 1234, Field3: 0},
		{Type: XRefEntryCompressed, Field2: 5, Field3: 0},
		{Type: XRefEntryCompressed, Field2: 5, Field3: 1},
	}
	subs := []XRefStreamSubsection{{First: 0, Entries: entries}}
	stream, err := BuildXRefStream(subs, 4, nil)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	// Serialize and pull out the compressed payload between "stream\n" and
	// "\nendstream".
	var buf bytes.Buffer
	if _, err := stream.WriteTo(&buf); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := buf.Bytes()
	startMarker := []byte("\nstream\n")
	endMarker := []byte("\nendstream")
	si := bytes.Index(out, startMarker)
	ei := bytes.Index(out, endMarker)
	if si < 0 || ei < 0 || ei <= si {
		t.Fatalf("could not locate stream payload in %q", out)
	}
	compressed := out[si+len(startMarker) : ei]

	zr, err := zlib.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("zlib reader: %v", err)
	}
	decoded, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("zlib read: %v", err)
	}

	wArr, _ := stream.Dict.Get("W").(*PdfArray)
	w0, _ := wArr.At(0).(*PdfNumber)
	w1, _ := wArr.At(1).(*PdfNumber)
	w2, _ := wArr.At(2).(*PdfNumber)
	widths := [3]int{w0.IntValue(), w1.IntValue(), w2.IntValue()}
	rowSize := widths[0] + widths[1] + widths[2]

	if len(decoded) != rowSize*len(entries) {
		t.Fatalf("decoded payload length %d, want %d", len(decoded), rowSize*len(entries))
	}

	for i, want := range entries {
		row := decoded[i*rowSize : (i+1)*rowSize]
		gotType := XRefEntryType(row[0])
		gotF2 := readBE(row[widths[0] : widths[0]+widths[1]])
		gotF3 := readBE(row[widths[0]+widths[1]:])
		if gotType != want.Type || gotF2 != want.Field2 || gotF3 != want.Field3 {
			t.Errorf("entry %d: got (%d,%d,%d), want (%d,%d,%d)",
				i, gotType, gotF2, gotF3, want.Type, want.Field2, want.Field3)
		}
	}
}

func TestBuildXRefStreamCopiesExtras(t *testing.T) {
	subs := []XRefStreamSubsection{minimalXRefSubsection(2)}
	extras := NewPdfDictionary()
	extras.Set("Root", NewPdfIndirectReference(2, 0))
	extras.Set("Info", NewPdfIndirectReference(3, 0))
	// Reserved keys must be ignored, even if the caller sets them.
	extras.Set("Type", NewPdfName("Should Not Win"))
	extras.Set("Filter", NewPdfName("Should Not Win"))
	extras.Set("Size", NewPdfInteger(99999))

	stream, err := BuildXRefStream(subs, 3, extras)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if root := stream.Dict.Get("Root"); root == nil {
		t.Error("/Root not copied from extras")
	}
	if info := stream.Dict.Get("Info"); info == nil {
		t.Error("/Info not copied from extras")
	}
	if name, ok := stream.Dict.Get("Type").(*PdfName); !ok || name.Value != "XRef" {
		t.Errorf("/Type = %v, must remain /XRef", stream.Dict.Get("Type"))
	}
	if n, ok := stream.Dict.Get("Size").(*PdfNumber); !ok || n.IntValue() != 3 {
		t.Errorf("/Size = %v, must remain 3 (caller override ignored)", stream.Dict.Get("Size"))
	}
}

func TestBuildXRefStreamDeterministic(t *testing.T) {
	subs := []XRefStreamSubsection{minimalXRefSubsection(5)}
	a, err := BuildXRefStream(subs, 6, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, err := BuildXRefStream(subs, 6, nil)
	if err != nil {
		t.Fatal(err)
	}
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

// readBE decodes a big-endian unsigned integer from a byte slice of
// arbitrary length up to 8.
func readBE(b []byte) uint64 {
	var v uint64
	for _, c := range b {
		v = v<<8 | uint64(c)
	}
	return v
}
