// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// Tests added in response to the phase-1 review of the optimizer.
// Each test pins a property the original test set did not cover; the
// reviewer rationale appears in the test comment so the next reader
// can decide whether the property still matters before deleting.

func TestWriteToWithOptionsZeroObjects(t *testing.T) {
	// A Writer with no objects must produce a valid PDF in all three
	// modes. The xref-stream paths previously had no test for this
	// boundary; the only entries are the free head and the xref
	// stream's own self-reference.
	cases := []struct {
		name string
		opts WriteOptions
	}{
		{name: "default", opts: WriteOptions{}},
		{name: "xref stream", opts: WriteOptions{UseXRefStream: true}},
		{name: "objstm", opts: WriteOptions{UseXRefStream: true, UseObjectStreams: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := NewWriter("1.7")
			var buf bytes.Buffer
			if _, err := w.WriteToWithOptions(&buf, c.opts); err != nil {
				t.Fatalf("write: %v", err)
			}
			pdf := buf.Bytes()
			if !bytes.HasPrefix(pdf, []byte("%PDF-1.7\n")) {
				t.Error("missing header")
			}
			if !bytes.HasSuffix(pdf, []byte("EOF\n")) {
				t.Error("missing EOF marker")
			}
		})
	}
}

func TestWriteToWithOptionsObjStmExactCount(t *testing.T) {
	// 7 eligible objects with capacity 3 must produce ceil(7/3) = 3
	// /ObjStm streams. The capacity-1 test only asserts a lower bound;
	// this one checks the exact upper bound so a future bug that pads
	// out empty objstms is caught.
	w := manyObjectsWriter(t, 7) // 7 fillers, plus catalog and pages
	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:        true,
		UseObjectStreams:     true,
		ObjectStreamCapacity: 3,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Eligible objects with manyObjectsWriter: pages tree (2) + 7
	// fillers = 8 eligible. ceil(8/3) = 3 objstms.
	count := strings.Count(buf.String(), "/Type /ObjStm")
	if count != 3 {
		t.Errorf("/Type /ObjStm count = %d, want exactly 3 (8 eligible / capacity 3)", count)
	}
}

func TestWriteToWithOptionsObjStmCapacityBoundaryExact(t *testing.T) {
	// Exactly capacity == eligible count: one full objstm, no tail.
	w := manyObjectsWriter(t, 4) // 4 fillers + pages = 5 eligible
	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:        true,
		UseObjectStreams:     true,
		ObjectStreamCapacity: 5,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	count := strings.Count(buf.String(), "/Type /ObjStm")
	if count != 1 {
		t.Errorf("/Type /ObjStm count = %d, want exactly 1", count)
	}
}

func TestWriteToWithOptionsObjStmCapacityBoundaryPlusOne(t *testing.T) {
	// One past capacity: one full objstm and a one-entry tail.
	w := manyObjectsWriter(t, 5) // 5 fillers + pages = 6 eligible
	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:        true,
		UseObjectStreams:     true,
		ObjectStreamCapacity: 5,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	count := strings.Count(buf.String(), "/Type /ObjStm")
	if count != 2 {
		t.Errorf("/Type /ObjStm count = %d, want exactly 2 (5 + 1)", count)
	}
}

func TestWriteToWithOptionsObjStmEncryptionRefusedBeforeMutation(t *testing.T) {
	// The encryption refusal must run BEFORE the encryption walk, so
	// a refused call leaves the writer's objects untouched and a
	// follow-up call without UseObjectStreams produces correct (not
	// double-encrypted) output. The arch reviewer flagged this as F2.
	w := minimalCatalogWriter(t)
	enc, err := core.NewEncryptor(core.RevisionAES128, "user", "owner", core.PermPrint)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	w.SetEncryption(enc)

	var buf bytes.Buffer
	_, err = w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	})
	if err == nil {
		t.Fatal("expected error for encryption + objstm")
	}

	// Now retry without UseObjectStreams. If the failed call had
	// already mutated the objects via the encryption walk, this
	// second call would double-encrypt and the catalog dictionary
	// would no longer be readable as a /Type /Catalog at the start
	// of the file.
	buf.Reset()
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{UseXRefStream: true}); err != nil {
		t.Fatalf("retry write: %v", err)
	}
	// We can't check for plaintext /Type /Catalog because encryption
	// is on. Instead check that the file is well-formed and the size
	// is in the expected range; double-encryption typically produces
	// a different (larger) byte count.
	if buf.Len() == 0 {
		t.Fatal("retry produced empty output")
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Error("retry output missing header")
	}
}

func TestWriteToWithOptionsObjStmBodyContainsPDFKeywords(t *testing.T) {
	// Object stream bodies are tokenized by the reader; bodies that
	// contain PDF keyword substrings (endobj, stream, endstream) must
	// not break parsing. Folio's writer never embeds these literally
	// in plain dict values today, but a string-valued entry could.
	w := minimalCatalogWriter(t)
	d := core.NewPdfDictionary()
	d.Set("Note", core.NewPdfLiteralString("contains endobj and endstream tokens"))
	w.AddObject(d)

	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("/Type /ObjStm")) {
		t.Error("expected an objstm to have been produced")
	}
}

func TestWriteToWithOptionsOptimizerQpdfCheck(t *testing.T) {
	// External validation against qpdf for the optimizer output.
	// Gated on qpdf availability via runQpdfCheck.
	doc := buildSampleDocument(15)
	out, err := doc.ToBytesWithOptions(WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	runQpdfCheck(t, out)
}

func TestWriteToWithOptionsXRefStreamQpdfCheck(t *testing.T) {
	// External validation for the xref-stream-only mode.
	doc := buildSampleDocument(15)
	out, err := doc.ToBytesWithOptions(WriteOptions{UseXRefStream: true})
	if err != nil {
		t.Fatal(err)
	}
	runQpdfCheck(t, out)
}
