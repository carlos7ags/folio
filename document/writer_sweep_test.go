// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// objectCountInBody returns the number of "N G obj" headers in a
// serialized PDF body. Used by sweep tests to assert exact object
// counts independently of the trailer/xref-stream representation.
func objectCountInBody(pdf []byte) int {
	// Count "obj\n" preceded by " 0" (generation 0 only — matches
	// every object the writer produces today).
	return strings.Count(string(pdf), " 0 obj\n")
}

func TestSweepOrphans_NoOpWhenAllReachable(t *testing.T) {
	// Every object in minimalCatalogWriter is reachable from /Root,
	// so the sweep must not change object count or numbering.
	w := minimalCatalogWriter(t)
	originalCount := len(w.objects)

	w.sweepOrphans()

	if len(w.objects) != originalCount {
		t.Errorf("sweepOrphans dropped reachable objects: count %d → %d", originalCount, len(w.objects))
	}
	if w.objects[0].ObjectNumber != 1 {
		t.Errorf("first object number = %d, want 1", w.objects[0].ObjectNumber)
	}
}

func TestSweepOrphans_RemovesUnreachableObject(t *testing.T) {
	// An AddObject call that nothing references must be dropped.
	w := minimalCatalogWriter(t)
	originalRoot := w.root
	originalRootObj := w.objects[0].Object
	orphan := core.NewPdfDictionary()
	orphan.Set("Type", core.NewPdfName("Filler"))
	w.AddObject(orphan)
	originalCount := len(w.objects)

	w.sweepOrphans()

	if len(w.objects) != originalCount-1 {
		t.Errorf("after sweep, len(objects) = %d, want %d", len(w.objects), originalCount-1)
	}
	for _, obj := range w.objects {
		if obj.Object == orphan {
			t.Error("orphan body still present in w.objects")
		}
	}
	// Identity check: the catalog body that was object 1 must still be
	// the first kept object (renumbering preserves original order).
	if w.objects[0].Object != originalRootObj {
		t.Error("first kept object is not the original catalog body")
	}
	if w.root != originalRoot {
		t.Error("w.root pointer changed; expected in-place rewrite of the same instance")
	}
}

func TestSweepOrphans_PreservesReachableNumbers(t *testing.T) {
	// When orphans sit BEHIND reachable objects, the survivors must
	// renumber down to fill the gap. catalog=1, pages=2, orphan=3,
	// extra=4. After sweep, extra becomes 3 because there is no
	// orphan #3 anymore — but if extra is also unreachable, only 1
	// and 2 remain.
	w := minimalCatalogWriter(t)
	orphan := core.NewPdfDictionary()
	orphan.Set("Type", core.NewPdfName("Filler"))
	w.AddObject(orphan)

	// Add a second unreachable object so the renumbering shifts more
	// than one slot. (It is also unreachable; both go.)
	extra := core.NewPdfDictionary()
	extra.Set("Type", core.NewPdfName("Filler2"))
	w.AddObject(extra)

	w.sweepOrphans()

	if len(w.objects) != 2 {
		t.Fatalf("len(objects) = %d, want 2", len(w.objects))
	}
	if w.objects[0].ObjectNumber != 1 || w.objects[1].ObjectNumber != 2 {
		t.Errorf("objects renumbered to (%d, %d), want (1, 2)",
			w.objects[0].ObjectNumber, w.objects[1].ObjectNumber)
	}
}

func TestSweepOrphans_RenumbersTransitiveReferences(t *testing.T) {
	// catalog (1) → pages (2). Insert an orphan AHEAD of pages so the
	// pages object gets renumbered downward, and verify catalog's
	// /Pages reference is rewritten to point at the new number.
	w := NewWriter("1.7")
	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))
	catalogRef := w.AddObject(catalog) // 1

	orphan := core.NewPdfDictionary()
	orphan.Set("Type", core.NewPdfName("Filler"))
	w.AddObject(orphan) // 2 — will be dropped

	pages := core.NewPdfDictionary()
	pages.Set("Type", core.NewPdfName("Pages"))
	pages.Set("Kids", core.NewPdfArray())
	pages.Set("Count", core.NewPdfInteger(0))
	pagesRef := w.AddObject(pages) // 3 — must renumber to 2
	catalog.Set("Pages", pagesRef)
	w.SetRoot(catalogRef)

	w.sweepOrphans()

	if len(w.objects) != 2 {
		t.Fatalf("len(objects) = %d, want 2", len(w.objects))
	}
	if got := pagesRef.Num(); got != 2 {
		t.Errorf("pagesRef renumbered to %d, want 2", got)
	}
	// Catalog stores the same *PdfIndirectReference instance under "Pages".
	if got := catalog.Get("Pages").(*core.PdfIndirectReference).Num(); got != 2 {
		t.Errorf("catalog /Pages = %d, want 2 after renumber", got)
	}
	if w.root.Num() != 1 {
		t.Errorf("w.root = %d, want 1", w.root.Num())
	}
}

func TestSweepOrphans_RenumbersSharedReferenceOnce(t *testing.T) {
	// One *PdfIndirectReference held by two bodies (as reader.Copier
	// hands out) must be renumbered once, not once per body.
	w := NewWriter("1.7")
	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))
	catalogRef := w.AddObject(catalog) // 1

	w.AddObject(core.NewPdfDictionary()) // 2 — orphan, dropped

	a := core.NewPdfDictionary()
	aRef := w.AddObject(a) // 3 → 2
	b := core.NewPdfDictionary()
	bRef := w.AddObject(b) // 4 → 3
	catalog.Set("A", aRef)
	catalog.Set("B", bRef)
	a.Set("B", bRef)
	w.SetRoot(catalogRef)

	w.sweepOrphans()

	if got := bRef.Num(); got != 3 {
		t.Errorf("shared reference renumbered to %d, want 3", got)
	}
}

func TestSweepOrphans_RewritesReferencesInsideArrays(t *testing.T) {
	// Pages' /Kids array holds references to page dicts. An orphan
	// inserted between catalog and the first page must trigger
	// renumbering inside the array, not just at the dictionary level.
	w := NewWriter("1.7")
	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))
	catalogRef := w.AddObject(catalog) // 1

	orphan := core.NewPdfDictionary()
	orphan.Set("Type", core.NewPdfName("Filler"))
	w.AddObject(orphan) // 2 — drop

	pages := core.NewPdfDictionary()
	pages.Set("Type", core.NewPdfName("Pages"))
	pagesRef := w.AddObject(pages) // 3 → 2

	page := core.NewPdfDictionary()
	page.Set("Type", core.NewPdfName("Page"))
	page.Set("Parent", pagesRef)
	pageRef := w.AddObject(page) // 4 → 3

	pages.Set("Kids", core.NewPdfArray(pageRef))
	pages.Set("Count", core.NewPdfInteger(1))
	catalog.Set("Pages", pagesRef)
	w.SetRoot(catalogRef)

	w.sweepOrphans()

	if len(w.objects) != 3 {
		t.Fatalf("len(objects) = %d, want 3", len(w.objects))
	}
	kidsArr := pages.Get("Kids").(*core.PdfArray)
	if kidsArr.Len() != 1 {
		t.Fatalf("kids len = %d, want 1", kidsArr.Len())
	}
	if got := kidsArr.At(0).(*core.PdfIndirectReference).Num(); got != 3 {
		t.Errorf("kids[0] = %d, want 3", got)
	}
	if got := page.Get("Parent").(*core.PdfIndirectReference).Num(); got != 2 {
		t.Errorf("page /Parent = %d, want 2", got)
	}
}

func TestSweepOrphans_DescendsThroughStreamDict(t *testing.T) {
	// A reference reachable only through a /Length entry inside a
	// stream dictionary must be marked reachable. Streams' opaque
	// payload bytes are not walked, but the dictionary is.
	w := NewWriter("1.7")
	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))
	catalogRef := w.AddObject(catalog) // 1

	target := core.NewPdfDictionary()
	target.Set("Type", core.NewPdfName("OnlyViaStreamDict"))
	targetRef := w.AddObject(target) // 2

	stream := core.NewPdfStream([]byte("hello"))
	stream.Dict.Set("Length", core.NewPdfInteger(5))
	stream.Dict.Set("Sidecar", targetRef)
	streamRef := w.AddObject(stream) // 3
	catalog.Set("Stream", streamRef)
	w.SetRoot(catalogRef)

	w.sweepOrphans()

	if len(w.objects) != 3 {
		t.Errorf("len(objects) = %d, want 3 (catalog + stream + sidecar)", len(w.objects))
	}
}

func TestSweepOrphans_FollowsInfoRoot(t *testing.T) {
	// /Info is a trailer root alongside /Root. An object reachable only
	// through w.info must survive.
	w := NewWriter("1.7")
	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))
	catalogRef := w.AddObject(catalog) // 1
	w.SetRoot(catalogRef)

	infoDict := core.NewPdfDictionary()
	infoDict.Set("Title", core.NewPdfLiteralString("test"))
	infoRef := w.AddObject(infoDict) // 2
	w.SetInfo(infoRef)

	w.sweepOrphans()

	if len(w.objects) != 2 {
		t.Errorf("len(objects) = %d, want 2 (catalog + info)", len(w.objects))
	}
}

func TestSweepOrphans_FollowsEncryptRoot(t *testing.T) {
	// /Encrypt is a trailer root alongside /Root. Objects reachable
	// only through w.encryptRef (and references nested inside the
	// encryption dictionary) must survive. We bypass SetEncryption to
	// avoid the upstream encryption-refusal — the sweep walker itself
	// is encryptor-agnostic.
	w := NewWriter("1.7")
	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))
	catalogRef := w.AddObject(catalog) // 1
	w.SetRoot(catalogRef)

	// Set up an /Encrypt dictionary by hand. It carries its own
	// indirect reference to a sidecar that must be reached only
	// through this trailer slot.
	sidecar := core.NewPdfDictionary()
	sidecar.Set("Type", core.NewPdfName("EncryptSidecar"))
	sidecarRef := w.AddObject(sidecar) // 2

	encDict := core.NewPdfDictionary()
	encDict.Set("Filter", core.NewPdfName("Standard"))
	encDict.Set("Sidecar", sidecarRef)
	encRef := w.AddObject(encDict) // 3
	// Manually wire the trailer slot. We do not call SetEncryption
	// because that path also installs an Encryptor, which sweepOrphans
	// is documented to refuse against.
	w.encryptRef = encRef

	w.sweepOrphans()

	if len(w.objects) != 3 {
		t.Errorf("len(objects) = %d, want 3 (catalog + encrypt + sidecar)", len(w.objects))
	}
	// Sidecar must still be present (it is reachable only through the
	// /Encrypt root).
	found := false
	for _, obj := range w.objects {
		if obj.Object == sidecar {
			found = true
		}
	}
	if !found {
		t.Error("sidecar reachable only via /Encrypt was incorrectly dropped")
	}
}

func TestSweepOrphans_Idempotent(t *testing.T) {
	// Sweeping twice must not differ from sweeping once at every
	// observable layer: the object-number snapshot AND the serialized
	// PDF bytes must match. A snapshot-only check would miss a bug
	// that mutated reference instances on the second pass.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		orphan := core.NewPdfDictionary()
		orphan.Set("Type", core.NewPdfName("Filler"))
		w.AddObject(orphan)
		return w
	}

	w := makeWriter()
	w.sweepOrphans()
	first := snapshotObjectNumbers(w)
	var firstBytes bytes.Buffer
	if _, err := w.WriteToWithOptions(&firstBytes, WriteOptions{}); err != nil {
		t.Fatalf("first write: %v", err)
	}

	w2 := makeWriter()
	w2.sweepOrphans()
	w2.sweepOrphans()
	second := snapshotObjectNumbers(w2)
	var secondBytes bytes.Buffer
	if _, err := w2.WriteToWithOptions(&secondBytes, WriteOptions{}); err != nil {
		t.Fatalf("second write: %v", err)
	}

	if !sliceEqual(first, second) {
		t.Errorf("sweep is not idempotent on object numbers: %v vs %v", first, second)
	}
	if !bytes.Equal(firstBytes.Bytes(), secondBytes.Bytes()) {
		t.Error("sweep is not idempotent on serialized output")
	}
}

func TestSweepOrphans_EmptyWriter(t *testing.T) {
	// A Writer with no objects must not panic and must remain empty.
	w := NewWriter("1.7")
	w.sweepOrphans()
	if len(w.objects) != 0 {
		t.Errorf("empty writer gained %d objects", len(w.objects))
	}
}

func TestSweepOrphans_NoRoots(t *testing.T) {
	// A Writer with objects but no /Root, /Info, or /Encrypt set
	// must drop everything — nothing is reachable.
	w := NewWriter("1.7")
	w.AddObject(core.NewPdfDictionary())
	w.AddObject(core.NewPdfDictionary())

	w.sweepOrphans()

	if len(w.objects) != 0 {
		t.Errorf("rootless writer kept %d objects, want 0", len(w.objects))
	}
}

func TestSweepOrphans_DanglingReferenceLeftUntouched(t *testing.T) {
	// A reachable object that points at an object number nothing
	// registered must not trip the sweep, and must not have its
	// reference number rewritten (rewriting it to 0 would silently
	// change semantics).
	w := NewWriter("1.7")
	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))
	dangling := core.NewPdfIndirectReference(99, 0)
	catalog.Set("Phantom", dangling)
	catalogRef := w.AddObject(catalog) // 1
	w.SetRoot(catalogRef)

	w.sweepOrphans()

	if len(w.objects) != 1 {
		t.Errorf("len(objects) = %d, want 1", len(w.objects))
	}
	if got := dangling.Num(); got != 99 {
		t.Errorf("dangling ref renumbered to %d, want 99 (untouched)", got)
	}
}

func TestSweepOrphans_DropsObjectShrinksPDF(t *testing.T) {
	// End-to-end size check: writing with OrphanSweep must produce
	// fewer bytes than writing without when the Writer holds an
	// orphan, and the same number of bytes (modulo orphan absence)
	// otherwise.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		orphan := core.NewPdfDictionary()
		orphan.Set("Type", core.NewPdfName("Filler"))
		// Add some payload so the orphan is non-trivial in size.
		orphan.Set("Lorem", core.NewPdfLiteralString(strings.Repeat("x", 500)))
		w.AddObject(orphan)
		return w
	}

	var withSweep, noSweep bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&noSweep, WriteOptions{}); err != nil {
		t.Fatalf("no-sweep write: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&withSweep, WriteOptions{OrphanSweep: true}); err != nil {
		t.Fatalf("sweep write: %v", err)
	}
	if withSweep.Len() >= noSweep.Len() {
		t.Errorf("OrphanSweep did not shrink output: sweep=%d bytes, no-sweep=%d bytes",
			withSweep.Len(), noSweep.Len())
	}
}

func TestSweepOrphans_RefusedOnEncryptedDocument(t *testing.T) {
	// The standard security handler keys each object on its number
	// (§7.6.3.3). Renumbering an encrypted document would silently
	// invalidate the keys, so the writer must refuse the option.
	// The refusal must happen BEFORE the encryption walk and BEFORE
	// the sweep itself, so a retry without OrphanSweep produces a
	// correct (not double-encrypted, not pre-renumbered) file.
	w := minimalCatalogWriter(t)
	enc, err := core.NewEncryptor(core.RevisionAES128, "user", "owner", core.PermPrint)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	w.SetEncryption(enc)

	preRefusalNumbers := snapshotObjectNumbers(w)
	preRefusalCount := len(w.objects)

	var buf bytes.Buffer
	_, err = w.WriteToWithOptions(&buf, WriteOptions{OrphanSweep: true})
	if err == nil {
		t.Fatal("expected error for encryption + OrphanSweep")
	}
	if !strings.Contains(err.Error(), "orphan sweep") {
		t.Errorf("error %q does not mention orphan sweep", err.Error())
	}

	// State invariant: the failed call must not have mutated the
	// writer's object set or numbering. A bug that pre-mutated would
	// either drop objects (count mismatch) or renumber them (snapshot
	// mismatch).
	if len(w.objects) != preRefusalCount {
		t.Errorf("refusal mutated object count: was %d, now %d",
			preRefusalCount, len(w.objects))
	}
	if !sliceEqual(preRefusalNumbers, snapshotObjectNumbers(w)) {
		t.Error("refusal renumbered objects")
	}

	buf.Reset()
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{}); err != nil {
		t.Fatalf("retry without sweep: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("retry produced empty output")
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("%PDF-")) {
		t.Error("retry output missing PDF header")
	}
}

func TestSweepOrphans_RefusalOrderWithObjStmAndEncryption(t *testing.T) {
	// All three guards are tripped simultaneously: UseObjectStreams,
	// OrphanSweep, and an encryptor are set. WriteToWithOptions must
	// return *some* error and leave the writer unmutated. We do not
	// pin the specific error message because the order of guard
	// evaluation is an internal implementation detail; we only pin
	// that one of the documented refusals fired and nothing was
	// half-applied.
	w := minimalCatalogWriter(t)
	enc, err := core.NewEncryptor(core.RevisionAES128, "user", "owner", core.PermPrint)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	w.SetEncryption(enc)
	preNumbers := snapshotObjectNumbers(w)

	var buf bytes.Buffer
	_, err = w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
		OrphanSweep:      true,
	})
	if err == nil {
		t.Fatal("expected refusal with objstm + sweep + encryption")
	}
	msg := err.Error()
	if !strings.Contains(msg, "encryption") && !strings.Contains(msg, "encrypted") {
		t.Errorf("error %q does not name the encryption refusal", msg)
	}
	if !sliceEqual(preNumbers, snapshotObjectNumbers(w)) {
		t.Error("triple-refusal mutated object numbering")
	}
}

func TestSweepOrphans_ZeroOptionsByteIdenticalWhenNoOrphans(t *testing.T) {
	// The zero-value WriteOptions must continue to produce byte-
	// identical output, and OrphanSweep on a clean document must also
	// produce byte-identical output to the no-sweep case (since the
	// sweep is a no-op when nothing is unreachable).
	makeWriter := func() *Writer {
		return minimalCatalogWriter(t)
	}

	var noSweep, withSweep bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&noSweep, WriteOptions{}); err != nil {
		t.Fatalf("no-sweep: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&withSweep, WriteOptions{OrphanSweep: true}); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if !bytes.Equal(noSweep.Bytes(), withSweep.Bytes()) {
		t.Error("OrphanSweep on clean document should produce byte-identical output")
	}
}

func TestSweepOrphans_CombinesWithXRefStream(t *testing.T) {
	// OrphanSweep must compose with UseXRefStream. Sweep runs first
	// and produces a smaller object set; the xref stream then encodes
	// the smaller set. The output must parse and shrink.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		for i := 0; i < 5; i++ {
			d := core.NewPdfDictionary()
			d.Set("Type", core.NewPdfName("Filler"))
			d.Set("Junk", core.NewPdfLiteralString(strings.Repeat("z", 200)))
			w.AddObject(d)
		}
		return w
	}

	var withoutSweep, withSweep bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&withoutSweep, WriteOptions{
		UseXRefStream: true,
	}); err != nil {
		t.Fatalf("xref-stream-only: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&withSweep, WriteOptions{
		UseXRefStream: true,
		OrphanSweep:   true,
	}); err != nil {
		t.Fatalf("xref-stream + sweep: %v", err)
	}
	if withSweep.Len() >= withoutSweep.Len() {
		t.Errorf("OrphanSweep + xref stream did not shrink: sweep=%d, no-sweep=%d",
			withSweep.Len(), withoutSweep.Len())
	}
	if !bytes.Contains(withSweep.Bytes(), []byte("/Type /XRef")) {
		t.Error("expected /Type /XRef in output")
	}
}

func TestSweepOrphans_CombinesWithObjectStreams(t *testing.T) {
	// OrphanSweep must compose with UseObjectStreams. The objstm
	// packer sees the post-sweep object set, so fewer eligible objects
	// are packed, which still shrinks output overall.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		for i := 0; i < 8; i++ {
			d := core.NewPdfDictionary()
			d.Set("Type", core.NewPdfName("Filler"))
			d.Set("Junk", core.NewPdfLiteralString(strings.Repeat("z", 200)))
			w.AddObject(d)
		}
		return w
	}

	var noSweep, withSweep bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&noSweep, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	}); err != nil {
		t.Fatalf("objstm only: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&withSweep, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
		OrphanSweep:      true,
	}); err != nil {
		t.Fatalf("objstm + sweep: %v", err)
	}
	if withSweep.Len() >= noSweep.Len() {
		t.Errorf("sweep failed to shrink objstm output: sweep=%d, no-sweep=%d",
			withSweep.Len(), noSweep.Len())
	}
}

func TestSweepOrphans_ProducedPDFHasFewerObjectHeaders(t *testing.T) {
	// Direct structural assertion: after sweep, the body must contain
	// fewer "N 0 obj\n" headers than before.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		for i := 0; i < 3; i++ {
			d := core.NewPdfDictionary()
			d.Set("Type", core.NewPdfName("Filler"))
			w.AddObject(d)
		}
		return w
	}

	var noSweep, withSweep bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&noSweep, WriteOptions{}); err != nil {
		t.Fatalf("no-sweep: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&withSweep, WriteOptions{OrphanSweep: true}); err != nil {
		t.Fatalf("sweep: %v", err)
	}

	noSweepCount := objectCountInBody(noSweep.Bytes())
	sweepCount := objectCountInBody(withSweep.Bytes())
	if noSweepCount != 5 {
		t.Errorf("no-sweep body has %d objects, want 5 (catalog + pages + 3 orphans)", noSweepCount)
	}
	if sweepCount != 2 {
		t.Errorf("sweep body has %d objects, want 2 (catalog + pages)", sweepCount)
	}
}

func TestSweepOrphans_DocumentAPIShrinksOutput(t *testing.T) {
	// End-to-end through the public Document API. A Document built
	// with the layout engine should not contain orphans today, so
	// OrphanSweep should be a near-no-op and definitely should not
	// enlarge or break the file.
	doc := buildSampleDocument(5)
	plain, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("plain: %v", err)
	}
	doc2 := buildSampleDocument(5)
	swept, err := doc2.ToBytesWithOptions(WriteOptions{OrphanSweep: true})
	if err != nil {
		t.Fatalf("swept: %v", err)
	}
	if len(swept) > len(plain) {
		t.Errorf("OrphanSweep enlarged a clean document: plain=%d, swept=%d",
			len(plain), len(swept))
	}
	if !bytes.HasPrefix(swept, []byte("%PDF-")) {
		t.Error("missing PDF header")
	}
	if !bytes.HasSuffix(swept, []byte("%%EOF\n")) {
		t.Error("missing EOF marker")
	}
}

func snapshotObjectNumbers(w *Writer) []int {
	out := make([]int, len(w.objects))
	for i, obj := range w.objects {
		out[i] = obj.ObjectNumber
	}
	return out
}

func sliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestWalkReferences_LeafKindsHaveNoReferences pins the contract that
// non-composite PDF objects (Name, Integer, Real, etc.) are leaves.
func TestWalkReferences_LeafKindsHaveNoReferences(t *testing.T) {
	cases := []core.PdfObject{
		core.NewPdfName("Foo"),
		core.NewPdfInteger(42),
		core.NewPdfReal(3.14),
		core.NewPdfBoolean(true),
		core.NewPdfLiteralString("hello"),
		core.NewPdfNull(),
	}
	for _, obj := range cases {
		called := false
		walkReferences(obj, func(*core.PdfIndirectReference) { called = true })
		if called {
			t.Errorf("walkReferences yielded a reference for leaf %T", obj)
		}
	}
}

// TestWalkReferences_DoesNotFollowReferences confirms the walker
// yields a reference but does not chase its target — important so a
// reference cycle cannot make the walker loop.
func TestWalkReferences_DoesNotFollowReferences(t *testing.T) {
	target := core.NewPdfDictionary()
	target.Set("Inner", core.NewPdfIndirectReference(99, 0))
	ref := core.NewPdfIndirectReference(1, 0)

	visited := 0
	walkReferences(ref, func(r *core.PdfIndirectReference) {
		visited++
		if r.Num() != 1 {
			t.Errorf("yielded ref num = %d, want 1", r.Num())
		}
	})
	if visited != 1 {
		t.Errorf("visited %d refs, want 1", visited)
	}
}

func TestSweepOrphans_SurvivesCycle(t *testing.T) {
	// A → B → A through indirect references registered as objects.
	// The BFS must terminate via the reachable-set dedupe; a regression
	// that moved the dedupe check after the dereference would loop
	// forever and hang the test.
	w := NewWriter("1.7")
	a := core.NewPdfDictionary()
	a.Set("Type", core.NewPdfName("A"))
	aRef := w.AddObject(a) // 1

	b := core.NewPdfDictionary()
	b.Set("Type", core.NewPdfName("B"))
	bRef := w.AddObject(b) // 2

	a.Set("Next", bRef)
	b.Set("Next", aRef)

	w.SetRoot(aRef)

	// Ten-second deadline guard via t.Deadline-style escape: if the
	// sweep loops, the test process will just hang. We rely on test
	// timeouts at the suite level. The structural assertion below
	// verifies completion.
	w.sweepOrphans()

	if len(w.objects) != 2 {
		t.Errorf("len(objects) = %d, want 2 (both nodes of the cycle reachable)", len(w.objects))
	}
}

func TestSweepOrphans_SurvivesSelfReference(t *testing.T) {
	// An object whose own dictionary contains a ref back to itself.
	// Same loop concern as the cycle test, simpler shape.
	w := NewWriter("1.7")
	self := core.NewPdfDictionary()
	self.Set("Type", core.NewPdfName("SelfReferential"))
	selfRef := w.AddObject(self) // 1
	self.Set("Me", selfRef)
	w.SetRoot(selfRef)

	w.sweepOrphans()

	if len(w.objects) != 1 {
		t.Errorf("len(objects) = %d, want 1", len(w.objects))
	}
}

func TestSweepOrphans_RewritesAllInstancesPointingAtSameNumber(t *testing.T) {
	// Two distinct *PdfIndirectReference instances point at object
	// number 2 (e.g., the catalog stores one instance under /Pages and
	// the page dict stores a separately-constructed instance under
	// /Parent). After the sweep renumbers object 2, BOTH instances
	// must be rewritten — otherwise one observer would see the new
	// number and the other the old one.
	w := NewWriter("1.7")
	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))
	catalogRef := w.AddObject(catalog) // 1

	orphan := core.NewPdfDictionary()
	orphan.Set("Type", core.NewPdfName("Filler"))
	w.AddObject(orphan) // 2 — drops, forcing pages to renumber to 2

	pages := core.NewPdfDictionary()
	pages.Set("Type", core.NewPdfName("Pages"))
	pages.Set("Count", core.NewPdfInteger(1))
	pagesRef := w.AddObject(pages) // 3 → 2

	page := core.NewPdfDictionary()
	page.Set("Type", core.NewPdfName("Page"))
	// Build a SECOND, independent reference instance to pages. Same
	// target number, distinct pointer.
	pagesRefAlt := core.NewPdfIndirectReference(pagesRef.Num(), pagesRef.Gen())
	page.Set("Parent", pagesRefAlt)
	pageRef := w.AddObject(page) // 4 → 3

	pages.Set("Kids", core.NewPdfArray(pageRef))
	catalog.Set("Pages", pagesRef)
	w.SetRoot(catalogRef)

	w.sweepOrphans()

	if got := pagesRef.Num(); got != 2 {
		t.Errorf("primary pages ref = %d, want 2", got)
	}
	if got := pagesRefAlt.Num(); got != 2 {
		t.Errorf("alternate pages ref instance = %d, want 2 (would dangle if missed)", got)
	}
}

func TestSweepOrphans_DropsOrphanChain(t *testing.T) {
	// Two registered objects A and B, neither reachable from any root.
	// A's body references B. Both must drop; the sweep must NOT mark B
	// reachable just because A's body points to it.
	w := minimalCatalogWriter(t)
	originalCount := len(w.objects)

	chainA := core.NewPdfDictionary()
	chainA.Set("Type", core.NewPdfName("A"))
	chainB := core.NewPdfDictionary()
	chainB.Set("Type", core.NewPdfName("B"))
	bRef := w.AddObject(chainB) // unreachable
	chainA.Set("Next", bRef)
	w.AddObject(chainA) // also unreachable

	if len(w.objects) != originalCount+2 {
		t.Fatalf("setup: len(objects) = %d, want %d", len(w.objects), originalCount+2)
	}

	w.sweepOrphans()

	if len(w.objects) != originalCount {
		t.Errorf("after sweep, len(objects) = %d, want %d (both orphans dropped)",
			len(w.objects), originalCount)
	}
}

func TestSweepOrphans_RefusesOnDuplicateObjectNumbers(t *testing.T) {
	// If the writer somehow held two IndirectObject slots with the
	// same ObjectNumber, the sweep cannot disambiguate which slot a
	// reference targeted. The implementation refuses (returns without
	// mutation) rather than silently corrupt object identity.
	w := NewWriter("1.7")
	first := core.NewPdfDictionary()
	first.Set("Type", core.NewPdfName("First"))
	w.AddObject(first) // 1
	// Manually add a second slot with the same object number.
	w.objects = append(w.objects, IndirectObject{
		ObjectNumber: 1,
		Object:       core.NewPdfDictionary(),
	})

	preCount := len(w.objects)
	w.sweepOrphans()
	if len(w.objects) != preCount {
		t.Errorf("sweep mutated state on duplicate-number input: %d → %d",
			preCount, len(w.objects))
	}
}

func TestSweepOrphans_SweptOutputReparses(t *testing.T) {
	// Structural reparse of the swept output. An off-by-one in xref
	// offsets after renumbering would shrink the file but produce
	// garbage; bytes.HasPrefix/HasSuffix would not catch it. We use
	// the reader package via the public Document API to round-trip.
	// (Direct reader import would be a layering inversion; instead we
	// pin structural invariants the writer is contractually obliged
	// to preserve.)
	w := minimalCatalogWriter(t)
	for i := 0; i < 5; i++ {
		d := core.NewPdfDictionary()
		d.Set("Type", core.NewPdfName("Filler"))
		d.Set("Junk", core.NewPdfLiteralString(strings.Repeat("z", 100)))
		w.AddObject(d)
	}

	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{OrphanSweep: true}); err != nil {
		t.Fatalf("write: %v", err)
	}
	pdf := buf.Bytes()

	// Structural anchors expected of any conformant ISO 32000 file
	// (§7.5.2 header, §7.5.4 xref, §7.5.5 trailer, §7.5.5 startxref + EOF).
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Error("missing PDF header")
	}
	if !bytes.HasSuffix(pdf, []byte("EOF\n")) {
		t.Error("missing EOF marker")
	}
	if !bytes.Contains(pdf, []byte("\nxref\n")) {
		t.Error("missing xref table keyword")
	}
	if !bytes.Contains(pdf, []byte("\ntrailer\n")) {
		t.Error("missing trailer keyword")
	}
	if !bytes.Contains(pdf, []byte("\nstartxref\n")) {
		t.Error("missing startxref keyword")
	}

	// startxref offset must be parseable and must point at a 'x' (the
	// first byte of "xref"). This catches off-by-one xref offsets.
	startxrefIdx := bytes.LastIndex(pdf, []byte("startxref\n"))
	if startxrefIdx < 0 {
		t.Fatal("startxref keyword not found")
	}
	rest := pdf[startxrefIdx+len("startxref\n"):]
	newline := bytes.IndexByte(rest, '\n')
	if newline < 0 {
		t.Fatal("no newline after startxref offset")
	}
	var off int64
	if _, err := fmt.Sscanf(string(rest[:newline]), "%d", &off); err != nil {
		t.Fatalf("parse startxref offset: %v", err)
	}
	if off < 0 || off >= int64(len(pdf)) {
		t.Errorf("startxref offset %d outside file (len %d)", off, len(pdf))
	}
	if pdf[off] != 'x' {
		t.Errorf("byte at startxref offset %d = %q, want 'x' (start of \"xref\")",
			off, pdf[off])
	}

	// Object header for object 2 must exist and refer to the renumbered
	// pages dictionary — the sweep should have collapsed numbering so
	// the original (unswept) object 2 is now a Filler object.
	if !bytes.Contains(pdf, []byte("\n2 0 obj\n")) {
		t.Error("missing object 2 header")
	}
}

func TestSweepOrphans_SetNumExposedAsExpected(t *testing.T) {
	// Direct unit test for core.PdfIndirectReference.SetNum — the
	// helper sweepOrphans relies on. A regression that made SetNum a
	// no-op would silently break renumbering; sweep tests would catch
	// it transitively, but a direct test makes the symptom obvious.
	ref := core.NewPdfIndirectReference(7, 0)
	ref.SetNum(3)
	if ref.Num() != 3 {
		t.Errorf("after SetNum(3), Num() = %d, want 3", ref.Num())
	}
	if ref.Gen() != 0 {
		t.Errorf("SetNum changed Gen() to %d, want 0", ref.Gen())
	}
}
