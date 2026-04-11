// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

func TestWriteToWithOptionsObjStmStructure(t *testing.T) {
	w := manyObjectsWriter(t, 5)
	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	pdf := buf.String()

	if !strings.HasPrefix(pdf, "%PDF-1.7\n") {
		t.Error("missing PDF header")
	}
	if !strings.HasSuffix(pdf, "%%EOF\n") {
		t.Error("missing EOF marker")
	}
	if !strings.Contains(pdf, "/Type /ObjStm") {
		t.Error("expected at least one /Type /ObjStm")
	}
	if !strings.Contains(pdf, "/Type /XRef") {
		t.Error("expected /Type /XRef on the trailing xref stream")
	}
	for _, line := range strings.Split(pdf, "\n") {
		if line == "xref" || line == "trailer" {
			t.Errorf("objstm mode produced legacy keyword line %q", line)
		}
	}
}

func TestWriteToWithOptionsObjStmCatalogStaysInline(t *testing.T) {
	// Phase 1 keeps /Root inline even though §7.5.7 would technically
	// permit compressing it in an unencrypted document. The first
	// object in minimalCatalogWriter is the catalog, so the file must
	// contain a "\n1 0 obj\n" header (anchored with a leading LF so
	// the substring does not collide with later object numbers like
	// "11" or "21").
	w := manyObjectsWriter(t, 3)
	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	pdf := buf.String()
	if !strings.Contains(pdf, "\n1 0 obj\n") {
		t.Error("catalog (object 1) is not present as an inline indirect object")
	}
}

func TestWriteToWithOptionsObjStmDeterministic(t *testing.T) {
	wA := manyObjectsWriter(t, 8)
	wB := manyObjectsWriter(t, 8)
	var bufA, bufB bytes.Buffer
	if _, err := wA.WriteToWithOptions(&bufA, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := wB.WriteToWithOptions(&bufB, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(bufA.Bytes(), bufB.Bytes()) {
		t.Error("objstm output is non-deterministic")
	}
}

func TestWriteToWithOptionsObjStmSmallerThanXRefStream(t *testing.T) {
	// On a doc with many small dictionaries, packing into an object
	// stream must be no larger than the xref-stream-only output. This
	// is the headline win of phase 1b.
	wA := manyObjectsWriter(t, 50)
	wB := manyObjectsWriter(t, 50)

	var xstm, objstm bytes.Buffer
	if _, err := wA.WriteToWithOptions(&xstm, WriteOptions{UseXRefStream: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := wB.WriteToWithOptions(&objstm, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	}); err != nil {
		t.Fatal(err)
	}
	if objstm.Len() > xstm.Len() {
		t.Errorf("objstm output (%d bytes) larger than xref-stream-only (%d bytes)",
			objstm.Len(), xstm.Len())
	}
	t.Logf("xref-stream=%d bytes, objstm=%d bytes, delta=%d",
		xstm.Len(), objstm.Len(), xstm.Len()-objstm.Len())
}

func TestWriteToWithOptionsObjStmCapacityOne(t *testing.T) {
	// Capacity 1 produces one /ObjStm per eligible object. With 5
	// eligible fillers we expect at least 5 /Type /ObjStm occurrences.
	w := manyObjectsWriter(t, 5)
	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:        true,
		UseObjectStreams:     true,
		ObjectStreamCapacity: 1,
	}); err != nil {
		t.Fatal(err)
	}
	count := strings.Count(buf.String(), "/Type /ObjStm")
	if count < 5 {
		t.Errorf("/Type /ObjStm count = %d, want at least 5", count)
	}
}

func TestWriteToWithOptionsObjStmRejectsEncryption(t *testing.T) {
	// Phase 1 refuses object streams when encryption is configured.
	// The interaction between the standard security handler and
	// /ObjStm requires careful handling deferred to a later phase.
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
		t.Error("expected error for encryption + object streams")
	}
}

func TestWriteToWithOptionsObjStmEmptyEligibleList(t *testing.T) {
	// When there are no eligible objects (here: only the catalog plus
	// a stream object, which is ineligible per §7.5.7), no /ObjStm is
	// produced. The output must still be a valid xref-stream PDF.
	w := NewWriter("1.7")
	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))
	catRef := w.AddObject(catalog)
	w.SetRoot(catRef)
	// One stream object — ineligible.
	w.AddObject(core.NewPdfStream([]byte("hello")))

	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	}); err != nil {
		t.Fatalf("write: %v", err)
	}
	pdf := buf.String()
	if strings.Contains(pdf, "/Type /ObjStm") {
		t.Error("did not expect /Type /ObjStm when no eligible objects")
	}
	if !strings.Contains(pdf, "/Type /XRef") {
		t.Error("expected /Type /XRef even with no objstms")
	}
}

func TestObjStmEligibleRules(t *testing.T) {
	// Direct unit test of the eligibility predicate. Streams, the
	// catalog, the info dict, and non-zero generations are all
	// rejected; everything else passes.
	w := NewWriter("1.7")

	d := core.NewPdfDictionary()
	d.Set("Type", core.NewPdfName("Catalog"))
	catRef := w.AddObject(d)
	w.SetRoot(catRef)

	info := core.NewPdfDictionary()
	infoRef := w.AddObject(info)
	w.SetInfo(infoRef)

	plain := core.NewPdfDictionary()
	plainRef := w.AddObject(plain)

	stream := core.NewPdfStream([]byte("data"))
	streamRef := w.AddObject(stream)

	gen2 := IndirectObject{
		ObjectNumber:     99,
		GenerationNumber: 2,
		Object:           core.NewPdfDictionary(),
	}

	if w.objStmEligible(w.objects[catRef.Num()-1]) {
		t.Error("catalog must be ineligible")
	}
	if w.objStmEligible(w.objects[infoRef.Num()-1]) {
		t.Error("info dict must be ineligible")
	}
	if !w.objStmEligible(w.objects[plainRef.Num()-1]) {
		t.Error("plain dict must be eligible")
	}
	if w.objStmEligible(w.objects[streamRef.Num()-1]) {
		t.Error("stream object must be ineligible")
	}
	if w.objStmEligible(gen2) {
		t.Error("generation > 0 must be ineligible")
	}
}

func TestWriteToWithOptionsObjStmSparseFileLayout(t *testing.T) {
	// Verify that compressed objects are NOT written inline. With
	// capacity 100, all 10 fillers (objects 3..12) go into one
	// objstm. The file must not contain "\nN 0 obj\n" anchored at a
	// line start for any of those object numbers.
	//
	// The leading \n anchor matters: without it, "3 0 obj\n" is a
	// substring of "13 0 obj\n" (which is the objstm's own header)
	// and the test gets a false positive.
	w := manyObjectsWriter(t, 10)
	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	}); err != nil {
		t.Fatal(err)
	}
	pdf := buf.String()
	for i := 3; i <= 12; i++ {
		needle := "\n" + intToStr(i) + " 0 obj\n"
		if strings.Contains(pdf, needle) {
			t.Errorf("filler object %d appears inline; should be in /ObjStm", i)
		}
	}
}

func intToStr(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
