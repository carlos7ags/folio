// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// addPlainDict registers a small dictionary with the given name and
// returns its indirect reference. Reachable from /Root via a sentinel
// /Sample-N entry to keep the orphan sweep from touching it.
func addPlainDict(t *testing.T, w *Writer, slot, name string) *core.PdfIndirectReference {
	t.Helper()
	d := core.NewPdfDictionary()
	d.Set("Type", core.NewPdfName(name))
	ref := w.AddObject(d)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set(slot, ref)
	return ref
}

func TestDeduplicateObjects_NoOpEmptyWriter(t *testing.T) {
	w := NewWriter("1.7")
	w.deduplicateObjects() // must not panic on zero objects
	if len(w.objects) != 0 {
		t.Errorf("empty writer gained %d objects", len(w.objects))
	}
}

func TestDeduplicateObjects_NoOpWhenNoDuplicates(t *testing.T) {
	w := minimalCatalogWriter(t)
	a := core.NewPdfDictionary()
	a.Set("Type", core.NewPdfName("Foo"))
	b := core.NewPdfDictionary()
	b.Set("Type", core.NewPdfName("Bar"))
	w.AddObject(a)
	w.AddObject(b)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("A", core.NewPdfIndirectReference(3, 0))
	root.Set("B", core.NewPdfIndirectReference(4, 0))
	preCount := len(w.objects)

	w.deduplicateObjects()

	if len(w.objects) != preCount {
		t.Errorf("dedup dropped non-duplicates: %d → %d", preCount, len(w.objects))
	}
}

func TestDeduplicateObjects_MergesIdenticalDicts(t *testing.T) {
	// Two separately-constructed dictionaries with byte-identical
	// content. After dedup, only one survives and both observers
	// reach the canonical survivor.
	w := minimalCatalogWriter(t)
	refA := addPlainDict(t, w, "A", "Twin")
	refB := addPlainDict(t, w, "B", "Twin")

	if refA.Num() == refB.Num() {
		t.Fatalf("setup: refs share number %d", refA.Num())
	}

	w.deduplicateObjects()

	root := w.objects[0].Object.(*core.PdfDictionary)
	gotA := root.Get("A").(*core.PdfIndirectReference)
	gotB := root.Get("B").(*core.PdfIndirectReference)
	if gotA.Num() != gotB.Num() {
		t.Errorf("after dedup, A=%d B=%d — should point at the same survivor",
			gotA.Num(), gotB.Num())
	}
}

func TestDeduplicateObjects_MergesIdenticalStreams(t *testing.T) {
	// Identical raw stream payloads with identical dicts must merge.
	// This is the headline case for the dedup pass: imported pages
	// often share fonts or color spaces stored as separate stream
	// objects.
	w := minimalCatalogWriter(t)
	payload := bytes.Repeat([]byte("shared "), 100)

	makeStream := func(slot string) *core.PdfIndirectReference {
		s := core.NewPdfStream(payload)
		s.Dict.Set("Type", core.NewPdfName("XObject"))
		ref := w.AddObject(s)
		root := w.objects[0].Object.(*core.PdfDictionary)
		root.Set(slot, ref)
		return ref
	}

	refA := makeStream("A")
	refB := makeStream("B")
	refC := makeStream("C")

	preCount := len(w.objects)
	w.deduplicateObjects()

	if len(w.objects) != preCount-2 {
		t.Errorf("expected to drop 2 of 3 duplicate streams: pre=%d, post=%d",
			preCount, len(w.objects))
	}
	if refA.Num() != refB.Num() || refB.Num() != refC.Num() {
		t.Errorf("three duplicate streams not collapsed to one: A=%d B=%d C=%d",
			refA.Num(), refB.Num(), refC.Num())
	}
}

func TestDeduplicateObjects_SkipsCatalogInfoEncrypt(t *testing.T) {
	// The catalog (w.root), info, and /Encrypt are excluded from
	// dedup. Even if a duplicate body were registered with the same
	// content as the catalog, the catalog itself must not be merged
	// into a non-root slot. Here we construct a body byte-identical
	// to the catalog and verify the catalog survives at slot 0.
	w := minimalCatalogWriter(t)
	originalRootObj := w.objects[0].Object
	clone := core.NewPdfDictionary()
	clone.Set("Type", core.NewPdfName("Catalog"))
	clone.Set("Pages", w.objects[0].Object.(*core.PdfDictionary).Get("Pages"))
	cloneRef := w.AddObject(clone)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("Decoy", cloneRef)

	w.deduplicateObjects()

	// The catalog must still be the first object after dedup.
	if w.objects[0].Object != originalRootObj {
		t.Error("catalog body was replaced; trailer /Root would point at the wrong object")
	}
	// w.root must still be in w.objects (not pointing at a number
	// that no longer exists).
	rootFound := false
	for _, obj := range w.objects {
		if obj.ObjectNumber == w.root.Num() && obj.Object == originalRootObj {
			rootFound = true
		}
	}
	if !rootFound {
		t.Errorf("trailer /Root (%d) does not match any kept object body", w.root.Num())
	}
}

func TestDeduplicateObjects_RewritesReferencesInArrays(t *testing.T) {
	// References to merged duplicates living inside PdfArray elements
	// must also be rewritten to the canonical survivor.
	w := minimalCatalogWriter(t)
	d1 := core.NewPdfDictionary()
	d1.Set("Type", core.NewPdfName("Twin"))
	d2 := core.NewPdfDictionary()
	d2.Set("Type", core.NewPdfName("Twin"))
	r1 := w.AddObject(d1)
	r2 := w.AddObject(d2)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("Twins", core.NewPdfArray(r1, r2))

	w.deduplicateObjects()

	arr := root.Get("Twins").(*core.PdfArray)
	if arr.Len() != 2 {
		t.Fatalf("array len = %d, want 2", arr.Len())
	}
	a := arr.At(0).(*core.PdfIndirectReference)
	b := arr.At(1).(*core.PdfIndirectReference)
	if a.Num() != b.Num() {
		t.Errorf("array refs not collapsed: [%d, %d]", a.Num(), b.Num())
	}
}

func TestDeduplicateObjects_RewritesReferencesInsideStreamDict(t *testing.T) {
	// A reference stored inside a stream dictionary must be rewritten
	// when its target is merged. This is the same case as PdfDictionary
	// rewrites but exercises the stream-dict descent in walkReferences.
	w := minimalCatalogWriter(t)
	twin1 := core.NewPdfDictionary()
	twin1.Set("Type", core.NewPdfName("Twin"))
	twin2 := core.NewPdfDictionary()
	twin2.Set("Type", core.NewPdfName("Twin"))
	r1 := w.AddObject(twin1)
	r2 := w.AddObject(twin2)

	holder := core.NewPdfStream([]byte("data"))
	holder.Dict.Set("RefA", r1)
	holder.Dict.Set("RefB", r2)
	holderRef := w.AddObject(holder)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("Holder", holderRef)

	w.deduplicateObjects()

	gotA := holder.Dict.Get("RefA").(*core.PdfIndirectReference)
	gotB := holder.Dict.Get("RefB").(*core.PdfIndirectReference)
	if gotA.Num() != gotB.Num() {
		t.Errorf("stream-dict refs not collapsed: A=%d B=%d", gotA.Num(), gotB.Num())
	}
}

func TestDeduplicateObjects_RenumbersSurvivorsContiguously(t *testing.T) {
	// After dedup drops slots, surviving objects must be renumbered
	// contiguously starting at 1. Otherwise the xref table would
	// carry free entries for the gaps, costing bytes.
	w := minimalCatalogWriter(t)
	// Two twins.
	addPlainDict(t, w, "A", "Twin")
	addPlainDict(t, w, "B", "Twin")
	// One unique trailing object.
	addPlainDict(t, w, "C", "Unique")

	preCount := len(w.objects)
	w.deduplicateObjects()

	if len(w.objects) != preCount-1 {
		t.Fatalf("expected %d objects, got %d", preCount-1, len(w.objects))
	}
	for i, obj := range w.objects {
		if obj.ObjectNumber != i+1 {
			t.Errorf("object at index %d has number %d, want %d", i, obj.ObjectNumber, i+1)
		}
	}
}

func TestDeduplicateObjects_Idempotent(t *testing.T) {
	// Running dedup twice must produce byte-identical writer output:
	// the first pass collapses duplicates; the second pass finds
	// nothing to merge.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		addPlainDict(t, w, "A", "Twin")
		addPlainDict(t, w, "B", "Twin")
		addPlainDict(t, w, "C", "Twin")
		return w
	}
	w1 := makeWriter()
	w1.deduplicateObjects()
	var buf1 bytes.Buffer
	if _, err := w1.WriteToWithOptions(&buf1, WriteOptions{}); err != nil {
		t.Fatalf("first write: %v", err)
	}

	w2 := makeWriter()
	w2.deduplicateObjects()
	w2.deduplicateObjects()
	var buf2 bytes.Buffer
	if _, err := w2.WriteToWithOptions(&buf2, WriteOptions{}); err != nil {
		t.Fatalf("second write: %v", err)
	}

	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Errorf("dedup not idempotent: 1 pass %d bytes vs 2 passes %d bytes",
			buf1.Len(), buf2.Len())
	}
}

func TestDeduplicateObjects_ZeroOptionsByteIdentical(t *testing.T) {
	// A Writer with eligible duplicates must produce byte-identical
	// output between WriteOptions{} and WriteTo when DeduplicateObjects
	// is unset. Defends against the toggle silently activating.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		addPlainDict(t, w, "A", "Twin")
		addPlainDict(t, w, "B", "Twin")
		return w
	}
	var bufA, bufB bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&bufA, WriteOptions{}); err != nil {
		t.Fatalf("zero opts: %v", err)
	}
	if _, err := makeWriter().WriteTo(&bufB); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	if !bytes.Equal(bufA.Bytes(), bufB.Bytes()) {
		t.Error("zero-options output diverges from WriteTo")
	}
}

func TestDeduplicateObjects_RefusedOnEncryptedDocument(t *testing.T) {
	// Per-object encryption keys derive from the object number
	// (§7.6.3.3); renumbering after dedup would invalidate every
	// key. The refusal must run before any mutation so a retry
	// without the option produces correct output.
	w := minimalCatalogWriter(t)
	addPlainDict(t, w, "A", "Twin")
	addPlainDict(t, w, "B", "Twin")
	enc, err := core.NewEncryptor(core.RevisionAES128, "user", "owner", core.PermPrint)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	w.SetEncryption(enc)
	preCount := len(w.objects)
	preNumbers := snapshotObjectNumbers(w)

	var buf bytes.Buffer
	_, err = w.WriteToWithOptions(&buf, WriteOptions{DeduplicateObjects: true})
	if err == nil {
		t.Fatal("expected refusal with encryption + DeduplicateObjects")
	}
	if !strings.Contains(err.Error(), "deduplication") {
		t.Errorf("error %q does not mention deduplication", err.Error())
	}
	if len(w.objects) != preCount {
		t.Errorf("refusal mutated object count: %d → %d", preCount, len(w.objects))
	}
	if !sliceEqual(preNumbers, snapshotObjectNumbers(w)) {
		t.Error("refusal renumbered objects")
	}

	buf.Reset()
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{}); err != nil {
		t.Fatalf("retry without dedup: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("retry produced empty output")
	}
}

func TestDeduplicateObjects_ComposesWithOrphanSweep(t *testing.T) {
	// Sweep + dedup: sweep drops orphans first, then dedup merges
	// remaining duplicates. Result must be smaller than either alone.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		// Reachable twins.
		addPlainDict(t, w, "A", "Twin")
		addPlainDict(t, w, "B", "Twin")
		// Unreachable orphan.
		o := core.NewPdfDictionary()
		o.Set("Type", core.NewPdfName("Orphan"))
		o.Set("Junk", core.NewPdfLiteralString(strings.Repeat("x", 200)))
		w.AddObject(o)
		return w
	}
	var sweepOnly, dedupOnly, both bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&sweepOnly, WriteOptions{
		OrphanSweep: true,
	}); err != nil {
		t.Fatalf("sweep only: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&dedupOnly, WriteOptions{
		DeduplicateObjects: true,
	}); err != nil {
		t.Fatalf("dedup only: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&both, WriteOptions{
		OrphanSweep:        true,
		DeduplicateObjects: true,
	}); err != nil {
		t.Fatalf("both: %v", err)
	}
	if both.Len() >= sweepOnly.Len() {
		t.Errorf("sweep+dedup not smaller than sweep alone: both=%d, sweep=%d",
			both.Len(), sweepOnly.Len())
	}
	if both.Len() >= dedupOnly.Len() {
		t.Errorf("sweep+dedup not smaller than dedup alone: both=%d, dedup=%d",
			both.Len(), dedupOnly.Len())
	}
}

func TestDeduplicateObjects_ComposesWithRecompressAndObjStm(t *testing.T) {
	// Full optimizer stack on a writer with duplicates. Result must
	// reparse structurally and be the smallest of all combinations.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		payload := bytes.Repeat([]byte("dedup-then-compress "), 200)
		makeStream := func(slot string) {
			s := core.NewPdfStream(payload)
			ref := w.AddObject(s)
			root := w.objects[0].Object.(*core.PdfDictionary)
			root.Set(slot, ref)
		}
		makeStream("A")
		makeStream("B")
		makeStream("C")
		return w
	}
	var noDedup, withDedup bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&noDedup, WriteOptions{
		UseXRefStream:     true,
		UseObjectStreams:  true,
		RecompressStreams: true,
	}); err != nil {
		t.Fatalf("no dedup: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&withDedup, WriteOptions{
		UseXRefStream:      true,
		UseObjectStreams:   true,
		RecompressStreams:  true,
		DeduplicateObjects: true,
	}); err != nil {
		t.Fatalf("with dedup: %v", err)
	}
	if withDedup.Len() >= noDedup.Len() {
		t.Errorf("dedup did not shrink with full stack: with=%d, without=%d",
			withDedup.Len(), noDedup.Len())
	}
	if !bytes.Contains(withDedup.Bytes(), []byte("/Type /XRef")) {
		t.Error("missing /Type /XRef in output")
	}
	if !bytes.HasPrefix(withDedup.Bytes(), []byte("%PDF-")) {
		t.Error("missing PDF header")
	}
	if !bytes.HasSuffix(withDedup.Bytes(), []byte("EOF\n")) {
		t.Error("missing EOF marker")
	}
}

func TestDeduplicateObjects_DocumentAPIDoesNotEnlarge(t *testing.T) {
	// Layout-built documents typically have few or no duplicate
	// objects, so dedup is a near-no-op. Asserting "no enlargement"
	// + structural reparse is the right contract for the public API.
	doc := buildSampleDocument(5)
	plain, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("plain: %v", err)
	}
	doc2 := buildSampleDocument(5)
	deduped, err := doc2.ToBytesWithOptions(WriteOptions{DeduplicateObjects: true})
	if err != nil {
		t.Fatalf("deduped: %v", err)
	}
	if len(deduped) > len(plain) {
		t.Errorf("DeduplicateObjects enlarged a layout-built document: plain=%d, deduped=%d",
			len(plain), len(deduped))
	}
	if !bytes.HasPrefix(deduped, []byte("%PDF-")) {
		t.Error("missing PDF header")
	}
	if !bytes.HasSuffix(deduped, []byte("EOF\n")) {
		t.Error("missing EOF marker")
	}
}

func TestDeduplicateObjects_DistinctButSimilarBodiesNotMerged(t *testing.T) {
	// Two dictionaries whose serialized bytes differ by a single byte
	// must NOT be merged. Defends against a hash truncation or
	// equality-by-prefix bug.
	w := minimalCatalogWriter(t)
	d1 := core.NewPdfDictionary()
	d1.Set("Type", core.NewPdfName("Almost"))
	d1.Set("N", core.NewPdfInteger(1))
	d2 := core.NewPdfDictionary()
	d2.Set("Type", core.NewPdfName("Almost"))
	d2.Set("N", core.NewPdfInteger(2))
	r1 := w.AddObject(d1)
	r2 := w.AddObject(d2)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("A", r1)
	root.Set("B", r2)

	preCount := len(w.objects)
	w.deduplicateObjects()
	if len(w.objects) != preCount {
		t.Errorf("dedup merged distinct dicts: %d → %d", preCount, len(w.objects))
	}
	if r1.Num() == r2.Num() {
		t.Error("distinct dicts share an object number after dedup")
	}
}

func TestDeduplicateObjects_HashStableAcrossWriteToCalls(t *testing.T) {
	// The dedup pass hashes obj.WriteTo() bytes. Two consecutive
	// WriteTo calls on the same object must produce identical bytes,
	// otherwise the second hash would diverge and dedup would fail
	// to merge a previously-merged duplicate (silently ineffective).
	//
	// Streams are the interesting case: PdfStream.WriteTo mutates
	// s.Dict by setting /Length (and /Filter when compress=true).
	// A bug that bumped /Length to a fresh dict slot on each call,
	// or that re-Flated to non-deterministic bytes, would surface
	// here.
	cases := []struct {
		name  string
		build func() core.PdfObject
	}{
		{
			name: "raw stream",
			build: func() core.PdfObject {
				s := core.NewPdfStream(bytes.Repeat([]byte("hello "), 200))
				s.Dict.Set("Type", core.NewPdfName("Test"))
				return s
			},
		},
		{
			name: "compressed stream",
			build: func() core.PdfObject {
				return core.NewPdfStreamCompressed(bytes.Repeat([]byte("compressible "), 200))
			},
		},
		{
			name: "dictionary",
			build: func() core.PdfObject {
				d := core.NewPdfDictionary()
				d.Set("Type", core.NewPdfName("Test"))
				d.Set("N", core.NewPdfInteger(42))
				d.Set("Name", core.NewPdfLiteralString("hello"))
				return d
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			obj := c.build()
			var buf1, buf2 bytes.Buffer
			if _, err := obj.WriteTo(&buf1); err != nil {
				t.Fatalf("first WriteTo: %v", err)
			}
			if _, err := obj.WriteTo(&buf2); err != nil {
				t.Fatalf("second WriteTo: %v", err)
			}
			if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
				t.Errorf("WriteTo not idempotent — dedup hashes would diverge\n  call 1 (%d B): %q\n  call 2 (%d B): %q",
					buf1.Len(), buf1.Bytes(), buf2.Len(), buf2.Bytes())
			}
		})
	}
}

func TestDeduplicateObjects_SweepThenDedupKeepsCatalogIdentity(t *testing.T) {
	// Run sweep and dedup together. After the combined run, the
	// catalog body must still be the SAME object the writer started
	// with (pointer identity), and w.root must point at the slot
	// that holds it. A bug that merged the catalog into a duplicate,
	// or that pointed w.root at a stale number, would silently break
	// /Root in the trailer.
	w := minimalCatalogWriter(t)
	originalCatalogBody := w.objects[0].Object
	// Add an orphan that sweep will drop, plus a twin pair that dedup
	// will merge.
	o := core.NewPdfDictionary()
	o.Set("Type", core.NewPdfName("Orphan"))
	w.AddObject(o)
	addPlainDict(t, w, "T1", "Twin")
	addPlainDict(t, w, "T2", "Twin")

	w.sweepOrphans()
	w.deduplicateObjects()

	rootSlot := -1
	for i, obj := range w.objects {
		if obj.Object == originalCatalogBody {
			rootSlot = i
			break
		}
	}
	if rootSlot < 0 {
		t.Fatal("original catalog body lost after sweep+dedup")
	}
	if w.root.Num() != w.objects[rootSlot].ObjectNumber {
		t.Errorf("w.root = %d, but original catalog is at object number %d",
			w.root.Num(), w.objects[rootSlot].ObjectNumber)
	}
}

func TestDeduplicateObjects_RefusalSnapshotsPayloadBytes(t *testing.T) {
	// The encryption refusal must not mutate any stream payload
	// bytes. A snapshot/check defends against a bug that pre-hashed
	// streams (calling WriteTo, which mutates the stream dict) before
	// the refusal fired.
	w := minimalCatalogWriter(t)
	payload := bytes.Repeat([]byte("payload "), 200)
	s := core.NewPdfStream(payload)
	s.Dict.Set("Type", core.NewPdfName("X"))
	ref := w.AddObject(s)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("X", ref)
	originalData := append([]byte(nil), s.Data...)
	originalDictLen := s.Dict.Len()

	enc, err := core.NewEncryptor(core.RevisionAES128, "user", "owner", core.PermPrint)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	w.SetEncryption(enc)

	var buf bytes.Buffer
	_, err = w.WriteToWithOptions(&buf, WriteOptions{DeduplicateObjects: true})
	if err == nil {
		t.Fatal("expected refusal")
	}
	if !bytes.Equal(s.Data, originalData) {
		t.Error("refusal mutated stream payload")
	}
	if s.Dict.Len() != originalDictLen {
		t.Errorf("refusal added entries to stream dict: was %d, now %d",
			originalDictLen, s.Dict.Len())
	}
}

func TestDeduplicateObjects_TripleEncryptionRefusal(t *testing.T) {
	// All three optimizer toggles enabled simultaneously with an
	// encryptor present: the writer must refuse with SOME error and
	// leave the writer state unmutated. We do not pin which refusal
	// fires first because that ordering is internal.
	w := minimalCatalogWriter(t)
	payload := bytes.Repeat([]byte("data "), 100)
	s := core.NewPdfStream(payload)
	ref := w.AddObject(s)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("S", ref)
	originalPayload := append([]byte(nil), s.Data...)

	enc, err := core.NewEncryptor(core.RevisionAES128, "user", "owner", core.PermPrint)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	w.SetEncryption(enc)
	preNumbers := snapshotObjectNumbers(w)

	var buf bytes.Buffer
	_, err = w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:       true,
		UseObjectStreams:    true,
		OrphanSweep:         true,
		CleanContentStreams: true,
		DeduplicateObjects:  true,
		RecompressStreams:   true,
	})
	if err == nil {
		t.Fatal("expected refusal with all toggles + encryption")
	}
	if !sliceEqual(preNumbers, snapshotObjectNumbers(w)) {
		t.Error("triple-refusal renumbered objects")
	}
	if !bytes.Equal(s.Data, originalPayload) {
		t.Error("triple-refusal mutated stream payload")
	}
}

func TestDeduplicateObjects_DropsObjectShrinksSerializedOutput(t *testing.T) {
	// End-to-end: a writer with three identical large-ish payloads
	// must produce strictly smaller serialized output with
	// DeduplicateObjects than without.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		// Use streams with significant payload so dedup wins are
		// visible against the per-object overhead.
		payload := bytes.Repeat([]byte("repeated payload "), 200)
		makeStream := func(slot string) {
			s := core.NewPdfStream(payload)
			ref := w.AddObject(s)
			root := w.objects[0].Object.(*core.PdfDictionary)
			root.Set(slot, ref)
		}
		makeStream("A")
		makeStream("B")
		makeStream("C")
		return w
	}
	var noDedup, withDedup bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&noDedup, WriteOptions{}); err != nil {
		t.Fatalf("no dedup: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&withDedup, WriteOptions{
		DeduplicateObjects: true,
	}); err != nil {
		t.Fatalf("with dedup: %v", err)
	}
	if withDedup.Len() >= noDedup.Len() {
		t.Errorf("dedup did not shrink: with=%d, without=%d",
			withDedup.Len(), noDedup.Len())
	}
}

func TestDeduplicateObjects_SameDataDifferentDictNotMerged(t *testing.T) {
	// Two streams with identical raw Data but different Dict entries
	// (e.g. different /Subtype) must not be merged: their WriteTo
	// output would differ.
	w := minimalCatalogWriter(t)
	payload := bytes.Repeat([]byte("shared "), 100)

	s1 := core.NewPdfStream(payload)
	s1.Dict.Set("Subtype", core.NewPdfName("Image"))
	r1 := w.AddObject(s1)

	s2 := core.NewPdfStream(payload)
	s2.Dict.Set("Subtype", core.NewPdfName("Form"))
	r2 := w.AddObject(s2)

	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("A", r1)
	root.Set("B", r2)

	preCount := len(w.objects)
	w.deduplicateObjects()

	if len(w.objects) != preCount {
		t.Errorf("dedup merged streams with different dicts: %d → %d", preCount, len(w.objects))
	}
	if r1.Num() == r2.Num() {
		t.Error("streams with different dicts share an object number after dedup")
	}
}

func TestDeduplicateObjects_SameDataAndDictDifferentCompressNotMerged(t *testing.T) {
	// Same Data and same Dict entries, but one stream compresses
	// (NewPdfStreamCompressed) and the other does not (NewPdfStream).
	// Their WriteTo output differs (/Filter presence and payload
	// bytes), so they must not be merged.
	w := minimalCatalogWriter(t)
	payload := bytes.Repeat([]byte("compressible payload "), 100)

	s1 := core.NewPdfStream(payload)
	r1 := w.AddObject(s1)

	s2 := core.NewPdfStreamCompressed(payload)
	r2 := w.AddObject(s2)

	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("A", r1)
	root.Set("B", r2)

	preCount := len(w.objects)
	w.deduplicateObjects()

	if len(w.objects) != preCount {
		t.Errorf("dedup merged streams with different compress flags: %d → %d", preCount, len(w.objects))
	}
	if r1.Num() == r2.Num() {
		t.Error("streams with different compress flags share an object number after dedup")
	}
}

func TestDeduplicateObjects_EqualCompressedStreamsMerge(t *testing.T) {
	// Two NewPdfStreamCompressed streams with equal data and equal
	// dicts must still merge under the new identity scheme.
	w := minimalCatalogWriter(t)
	payload := bytes.Repeat([]byte("compressible payload "), 100)

	s1 := core.NewPdfStreamCompressed(payload)
	r1 := w.AddObject(s1)
	s2 := core.NewPdfStreamCompressed(payload)
	r2 := w.AddObject(s2)

	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("A", r1)
	root.Set("B", r2)

	preCount := len(w.objects)
	w.deduplicateObjects()

	if len(w.objects) != preCount-1 {
		t.Errorf("expected 1 object dropped, pre=%d post=%d", preCount, len(w.objects))
	}
	if r1.Num() != r2.Num() {
		t.Errorf("equal compressed streams not merged: A=%d B=%d", r1.Num(), r2.Num())
	}
}
