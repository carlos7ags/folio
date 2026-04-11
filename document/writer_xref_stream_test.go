// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// minimalCatalogWriter builds a Writer holding a minimal but valid
// document: a catalog and an empty pages tree. Several xref-stream
// tests share this fixture.
func minimalCatalogWriter(t *testing.T) *Writer {
	t.Helper()
	w := NewWriter("1.7")

	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))

	pages := core.NewPdfDictionary()
	pages.Set("Type", core.NewPdfName("Pages"))
	pages.Set("Kids", core.NewPdfArray())
	pages.Set("Count", core.NewPdfInteger(0))

	catalogRef := w.AddObject(catalog)
	pagesRef := w.AddObject(pages)
	catalog.Set("Pages", pagesRef)
	w.SetRoot(catalogRef)
	return w
}

// manyObjectsWriter builds a Writer with N+2 objects: a catalog, an
// empty pages tree, and N dummy dictionaries. Used for size-comparison
// and dense-subsection tests that need enough objects for the xref
// stream's overhead to amortize.
func manyObjectsWriter(t *testing.T, n int) *Writer {
	t.Helper()
	w := minimalCatalogWriter(t)
	for i := 0; i < n; i++ {
		d := core.NewPdfDictionary()
		d.Set("Type", core.NewPdfName("Filler"))
		d.Set("Index", core.NewPdfInteger(i))
		w.AddObject(d)
	}
	return w
}

func TestWriteToWithOptionsZeroValueMatchesWriteTo(t *testing.T) {
	// The zero-value WriteOptions must produce the historical default
	// output. Otherwise existing callers would observe a behavior
	// change after the refactor.
	wA := minimalCatalogWriter(t)
	wB := minimalCatalogWriter(t)

	var bufA, bufB bytes.Buffer
	if _, err := wA.WriteTo(&bufA); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if _, err := wB.WriteToWithOptions(&bufB, WriteOptions{}); err != nil {
		t.Fatalf("WriteToWithOptions zero: %v", err)
	}
	if !bytes.Equal(bufA.Bytes(), bufB.Bytes()) {
		t.Errorf("zero-options output diverges from WriteTo")
	}
}

func TestWriteToWithOptionsXRefStreamStructure(t *testing.T) {
	w := minimalCatalogWriter(t)

	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{UseXRefStream: true}); err != nil {
		t.Fatalf("WriteToWithOptions: %v", err)
	}
	pdf := buf.String()

	if !strings.HasPrefix(pdf, "%PDF-1.7\n") {
		t.Error("missing PDF header")
	}
	if !strings.HasSuffix(pdf, "%%EOF\n") {
		t.Error("missing EOF marker")
	}
	if !strings.Contains(pdf, "startxref") {
		t.Error("missing startxref")
	}
	// xref-stream mode must NOT emit the traditional 'xref' or 'trailer'
	// keywords as standalone lines. Tokenize loosely by line.
	for _, line := range strings.Split(pdf, "\n") {
		if line == "xref" {
			t.Error("xref-stream mode produced a traditional 'xref' keyword line")
		}
		if line == "trailer" {
			t.Error("xref-stream mode produced a 'trailer' keyword line")
		}
	}
	if !strings.Contains(pdf, "/Type /XRef") {
		t.Error("xref stream missing /Type /XRef")
	}
	// /W and /Size are mandatory on xref streams (§7.5.8.2 Table 17).
	if !strings.Contains(pdf, "/W ") {
		t.Error("xref stream missing /W")
	}
	if !strings.Contains(pdf, "/Size ") {
		t.Error("xref stream missing /Size")
	}
	if !strings.Contains(pdf, "/Root ") {
		t.Error("xref stream missing /Root")
	}
}

func TestWriteToWithOptionsXRefStreamStartxrefPointsAtStream(t *testing.T) {
	// startxref must point at the byte offset of the xref stream's
	// "N 0 obj" header, not at a 'xref' keyword. Verify by parsing the
	// startxref value and checking that the bytes at that offset begin
	// with "<num> 0 obj".
	w := minimalCatalogWriter(t)
	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{UseXRefStream: true}); err != nil {
		t.Fatal(err)
	}
	pdf := buf.Bytes()

	idx := bytes.Index(pdf, []byte("startxref\n"))
	if idx < 0 {
		t.Fatal("missing startxref")
	}
	rest := pdf[idx+len("startxref\n"):]
	nl := bytes.IndexByte(rest, '\n')
	if nl < 0 {
		t.Fatal("malformed startxref")
	}
	offsetStr := string(rest[:nl])
	var offset int
	for _, c := range offsetStr {
		if c < '0' || c > '9' {
			t.Fatalf("non-numeric startxref offset: %q", offsetStr)
		}
		offset = offset*10 + int(c-'0')
	}
	if offset >= len(pdf) {
		t.Fatalf("startxref offset %d out of bounds (file %d bytes)", offset, len(pdf))
	}
	at := pdf[offset:]
	// Expect "<num> 0 obj\n"
	sp := bytes.IndexByte(at, ' ')
	if sp < 0 {
		t.Fatalf("startxref offset %d does not point at an object header: %q", offset, at[:min(40, len(at))])
	}
	if !bytes.HasPrefix(at[sp:], []byte(" 0 obj\n")) {
		t.Errorf("startxref offset %d does not point at an obj header, found: %q",
			offset, at[:min(40, len(at))])
	}
}

func TestWriteToWithOptionsXRefStreamDeterministic(t *testing.T) {
	wA := minimalCatalogWriter(t)
	wB := minimalCatalogWriter(t)

	var bufA, bufB bytes.Buffer
	if _, err := wA.WriteToWithOptions(&bufA, WriteOptions{UseXRefStream: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := wB.WriteToWithOptions(&bufB, WriteOptions{UseXRefStream: true}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bufA.Bytes(), bufB.Bytes()) {
		t.Errorf("xref-stream output is non-deterministic")
	}
}

func TestWriteToWithOptionsXRefStreamSmallerOnLargeDoc(t *testing.T) {
	// On a doc with enough objects to amortize the xref stream's own
	// overhead, the xref stream output must be no larger than the
	// traditional output. Use 30 objects to comfortably exceed the
	// breakeven point.
	wA := manyObjectsWriter(t, 30)
	wB := manyObjectsWriter(t, 30)

	var trad, xstm bytes.Buffer
	if _, err := wA.WriteTo(&trad); err != nil {
		t.Fatal(err)
	}
	if _, err := wB.WriteToWithOptions(&xstm, WriteOptions{UseXRefStream: true}); err != nil {
		t.Fatal(err)
	}
	if xstm.Len() > trad.Len() {
		t.Errorf("xref stream output (%d bytes) larger than traditional (%d bytes)",
			xstm.Len(), trad.Len())
	}
	t.Logf("traditional=%d bytes, xref-stream=%d bytes, delta=%d",
		trad.Len(), xstm.Len(), trad.Len()-xstm.Len())
}

func TestWriteToWithOptionsObjectStreamsRequiresXRefStream(t *testing.T) {
	// Type-2 xref entries (compressed objects) require an xref stream
	// to express; the combination must be rejected rather than silently
	// upgraded.
	w := minimalCatalogWriter(t)
	var buf bytes.Buffer
	_, err := w.WriteToWithOptions(&buf, WriteOptions{UseObjectStreams: true})
	if err == nil {
		t.Error("expected error: UseObjectStreams without UseXRefStream")
	}
}
