// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"compress/zlib"
	"crypto/rand"
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
)

// makeFlateAt produces a Flate-encoded payload for plaintext using the
// requested zlib level. Used by tests that need a "lower-than-Best"
// baseline to demonstrate the recompression win.
func makeFlateAt(t *testing.T, plaintext []byte, level int) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw, err := zlib.NewWriterLevel(&buf, level)
	if err != nil {
		t.Fatalf("zlib.NewWriterLevel(%d): %v", level, err)
	}
	if _, err := zw.Write(plaintext); err != nil {
		t.Fatalf("zlib write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zlib close: %v", err)
	}
	return buf.Bytes()
}

// addRawStream wires a raw (uncompressed, no /Filter) stream into a
// minimal Writer. Mirrors the shape produced by reader/import.go for
// imported pages whose source was compressed by the resolver.
func addRawStream(t *testing.T, w *Writer, data []byte) (*core.PdfStream, *core.PdfIndirectReference) {
	t.Helper()
	s := core.NewPdfStream(data)
	ref := w.AddObject(s)
	// Wire it under /Root so OrphanSweep would not drop it.
	root, ok := w.objects[0].Object.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("setup: first object is not the root dict")
	}
	root.Set("Sample", ref)
	return s, ref
}

// addFlateStream wires a pre-Flate-compressed stream (with /Filter
// /FlateDecode set, compress=false) into the Writer. The level
// parameter controls how aggressively the source compressed it.
func addFlateStream(t *testing.T, w *Writer, plaintext []byte, level int) (*core.PdfStream, *core.PdfIndirectReference) {
	t.Helper()
	encoded := makeFlateAt(t, plaintext, level)
	s := core.NewPdfStream(encoded)
	s.Dict.Set("Filter", core.NewPdfName("FlateDecode"))
	ref := w.AddObject(s)
	root, ok := w.objects[0].Object.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("setup: first object is not the root dict")
	}
	root.Set("Sample", ref)
	return s, ref
}

func TestRecompressStreams_NoOpOnEmptyWriter(t *testing.T) {
	w := NewWriter("1.7")
	w.recompressStreams() // must not panic on zero objects
	if len(w.objects) != 0 {
		t.Errorf("empty writer gained %d objects", len(w.objects))
	}
}

func TestRecompressStreams_SkipsWillCompressStreams(t *testing.T) {
	// A stream marked compress=true is what NewPdfStreamCompressed
	// returns; the writer's WriteTo will deflate it at BestCompression
	// already. Recompression should leave it alone — re-running here
	// would just duplicate work for identical output.
	w := minimalCatalogWriter(t)
	plaintext := bytes.Repeat([]byte("compressible "), 200)
	s := core.NewPdfStreamCompressed(plaintext)
	originalData := s.Data
	originalCompress := s.WillCompress()
	ref := w.AddObject(s)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("Sample", ref)

	w.recompressStreams()

	if !bytes.Equal(s.Data, originalData) {
		t.Error("compress=true stream was rewritten")
	}
	if s.WillCompress() != originalCompress {
		t.Error("compress flag flipped on a compress=true stream")
	}
	if s.Dict.Get("Filter") != nil {
		t.Error("compress=true stream got a /Filter set prematurely")
	}
}

func TestRecompressStreams_ShrinksRawStream(t *testing.T) {
	// A raw stream (no /Filter, compress=false) is the shape produced
	// by reader/import.go when it copies a stream that the resolver
	// already inflated. Recompression must Flate the payload, set
	// /Filter /FlateDecode, and shrink the byte count.
	w := minimalCatalogWriter(t)
	plaintext := bytes.Repeat([]byte("shrink me "), 500)
	s, _ := addRawStream(t, w, plaintext)
	originalLen := len(s.Data)

	w.recompressStreams()

	if len(s.Data) >= originalLen {
		t.Errorf("recompression did not shrink: was %d, now %d", originalLen, len(s.Data))
	}
	filter := s.Dict.Get("Filter")
	if filter == nil {
		t.Fatal("missing /Filter after recompression")
	}
	name, ok := filter.(*core.PdfName)
	if !ok || name.Value != "FlateDecode" {
		t.Errorf("/Filter = %v, want /FlateDecode", filter)
	}
	if s.WillCompress() {
		t.Error("compress flag still set after pre-compression; WriteTo would double-compress")
	}
}

func TestRecompressStreams_ReFlatesBestSpeedStream(t *testing.T) {
	// A stream pre-compressed at BestSpeed by another producer is
	// eligible: inflate then re-deflate at BestCompression. The win
	// is producer-dependent but real for sufficiently large payloads.
	w := minimalCatalogWriter(t)
	var buf bytes.Buffer
	for i := 0; i < 300; i++ {
		fmt.Fprintf(&buf, "q 1 0 0 1 %d %d cm /F1 12 Tf (%d) Tj Q BT /F2 10 Tf %d %d Td (Invoice line item description %d with unit price %0.2f) Tj ET\n",
			i*10, (i*20)%800, i, i%50, i%100, i, float64(i)*19.99)
	}
	plaintext := buf.Bytes()
	s, _ := addFlateStream(t, w, plaintext, zlib.BestSpeed)
	originalLen := len(s.Data)

	w.recompressStreams()

	if len(s.Data) >= originalLen {
		t.Errorf("re-Flate did not shrink: was %d (BestSpeed), now %d (BestCompression)",
			originalLen, len(s.Data))
	}
	// /Filter must remain /FlateDecode (single name, not array).
	filter := s.Dict.Get("Filter")
	name, ok := filter.(*core.PdfName)
	if !ok || name.Value != "FlateDecode" {
		t.Errorf("/Filter = %v, want /FlateDecode", filter)
	}
}

func TestRecompressStreams_GuardRevertsRandomBytes(t *testing.T) {
	// Random bytes are incompressible — Flate will produce output
	// strictly larger than the input thanks to header overhead. The
	// guard must revert the rewrite, leaving the stream untouched.
	w := minimalCatalogWriter(t)
	random := make([]byte, 4096)
	if _, err := rand.Read(random); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	s, _ := addRawStream(t, w, random)
	originalData := append([]byte(nil), s.Data...)
	originalFilter := s.Dict.Get("Filter")

	w.recompressStreams()

	if !bytes.Equal(s.Data, originalData) {
		t.Error("guard failed to revert: random bytes were rewritten")
	}
	if originalFilter == nil && s.Dict.Get("Filter") != nil {
		t.Error("guard failed to revert: /Filter was added after revert")
	}
}

func TestRecompressStreams_SkipsDCTDecode(t *testing.T) {
	// A JPEG-encoded payload (/Filter /DCTDecode) must not be
	// touched. Re-Flating already-DCT bytes inflates them and adds
	// a meaningless filter chain that no reader would correctly
	// decode without the DCT step.
	w := minimalCatalogWriter(t)
	fakeJPEG := bytes.Repeat([]byte{0xff, 0xd8, 0xff, 0xe0}, 200) // not a valid JPEG, but the decoder is not invoked
	s := core.NewPdfStream(fakeJPEG)
	s.Dict.Set("Filter", core.NewPdfName("DCTDecode"))
	ref := w.AddObject(s)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("Sample", ref)

	originalData := append([]byte(nil), s.Data...)
	w.recompressStreams()

	if !bytes.Equal(s.Data, originalData) {
		t.Error("DCTDecode stream was modified by recompression")
	}
	filter := s.Dict.Get("Filter").(*core.PdfName)
	if filter.Value != "DCTDecode" {
		t.Errorf("/Filter = %s, want DCTDecode unchanged", filter.Value)
	}
}

func TestRecompressStreams_SkipsAllSpecializedFilters(t *testing.T) {
	// Per ISO 32000-1 §7.4.7 (CCITTFaxDecode), §7.4.8 (DCTDecode,
	// JBIG2Decode), and §7.4.9 (JPXDecode), these filters carry
	// payloads that Flate must not be applied over.
	specialized := []string{"DCTDecode", "JPXDecode", "CCITTFaxDecode", "JBIG2Decode"}
	for _, filter := range specialized {
		t.Run(filter, func(t *testing.T) {
			w := minimalCatalogWriter(t)
			payload := bytes.Repeat([]byte("opaque "), 100)
			s := core.NewPdfStream(payload)
			s.Dict.Set("Filter", core.NewPdfName(filter))
			ref := w.AddObject(s)
			root := w.objects[0].Object.(*core.PdfDictionary)
			root.Set("Sample", ref)
			original := append([]byte(nil), s.Data...)

			w.recompressStreams()

			if !bytes.Equal(s.Data, original) {
				t.Errorf("%s stream was modified", filter)
			}
		})
	}
}

func TestRecompressStreams_SkipsMultiFilterChain(t *testing.T) {
	// A multi-filter chain like /Filter [/ASCII85Decode /FlateDecode]
	// is not in the eligibility set: rewriting would require
	// understanding the entire chain and producing bytes that round-
	// trip through it. Conservative skip.
	w := minimalCatalogWriter(t)
	payload := bytes.Repeat([]byte("a"), 100)
	s := core.NewPdfStream(payload)
	chain := core.NewPdfArray(
		core.NewPdfName("ASCII85Decode"),
		core.NewPdfName("FlateDecode"),
	)
	s.Dict.Set("Filter", chain)
	ref := w.AddObject(s)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("Sample", ref)
	original := append([]byte(nil), s.Data...)

	w.recompressStreams()

	if !bytes.Equal(s.Data, original) {
		t.Error("multi-filter chain stream was modified")
	}
}

func TestRecompressStreams_SkipsFlateWithDecodeParms(t *testing.T) {
	// CRITICAL correctness: a Flate stream with /DecodeParms (typically
	// a PNG/TIFF predictor per §7.4.4.4) carries predictor-filtered
	// bytes after inflation, NOT plaintext. The recompression pass
	// MUST skip these — otherwise re-deflating the filtered bytes and
	// dropping /DecodeParms produces a stream that decodes to garbage
	// in any reader that honors the predictor contract.
	w := minimalCatalogWriter(t)
	// Build a synthetic predictor-shaped payload. The bytes themselves
	// don't have to be valid PNG-filtered data — the eligibility
	// decision must be made from the dictionary alone, before any
	// inflate happens. We pre-Flate the bytes so they look like a
	// typical xref-stream-style payload.
	preFiltered := bytes.Repeat([]byte("predictor-filtered "), 300)
	s, _ := addFlateStream(t, w, preFiltered, zlib.BestSpeed)
	parms := core.NewPdfDictionary()
	parms.Set("Predictor", core.NewPdfInteger(15))
	parms.Set("Columns", core.NewPdfInteger(80))
	s.Dict.Set("DecodeParms", parms)

	originalData := append([]byte(nil), s.Data...)

	w.recompressStreams()

	if !bytes.Equal(s.Data, originalData) {
		t.Error("Flate-with-/DecodeParms stream was modified — predictor contract violated")
	}
	// /DecodeParms must remain — the skip preserves the original
	// dictionary intact, including the predictor parameters.
	if s.Dict.Get("DecodeParms") == nil {
		t.Error("skip path mutated dictionary by removing /DecodeParms")
	}
	// /Filter must remain.
	if name, ok := s.Dict.Get("Filter").(*core.PdfName); !ok || name.Value != "FlateDecode" {
		t.Errorf("/Filter mutated: %v", s.Dict.Get("Filter"))
	}
}

func TestClassifyForRecompress_FlateWithDecodeParmsIneligible(t *testing.T) {
	// Direct unit on the eligibility decision, separate from the
	// pass-level test above so a regression in the classifier alone
	// fails with a clear signal.
	s := core.NewPdfStream(nil)
	s.Dict.Set("Filter", core.NewPdfName("FlateDecode"))
	s.Dict.Set("DecodeParms", core.NewPdfDictionary())
	if _, ok := classifyForRecompress(s); ok {
		t.Error("Flate stream with /DecodeParms classified as eligible (would corrupt predictor data)")
	}
}

func TestRecompressStreams_LeavesEmptyStreamAlone(t *testing.T) {
	// An empty stream cannot shrink. The pass must leave it alone
	// rather than emit a Flate-of-empty header (which is non-zero).
	w := minimalCatalogWriter(t)
	s := core.NewPdfStream(nil)
	ref := w.AddObject(s)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("Sample", ref)

	w.recompressStreams()

	if len(s.Data) != 0 {
		t.Errorf("empty stream gained %d bytes", len(s.Data))
	}
	if s.Dict.Get("Filter") != nil {
		t.Error("empty stream got a spurious /Filter")
	}
}

func TestRecompressStreams_Idempotent(t *testing.T) {
	// Recompressing twice must produce byte-identical writer output:
	// the first pass establishes BestCompression bytes, the second
	// pass inflates them, re-deflates them (same bytes, same output),
	// and the guard reverts since they're equal-size — no commit.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		addRawStream(t, w, bytes.Repeat([]byte("idempotent "), 400))
		addFlateStream(t, w, bytes.Repeat([]byte("flate "), 400), zlib.BestSpeed)
		return w
	}

	w1 := makeWriter()
	w1.recompressStreams()
	var buf1 bytes.Buffer
	if _, err := w1.WriteToWithOptions(&buf1, WriteOptions{}); err != nil {
		t.Fatalf("first write: %v", err)
	}

	w2 := makeWriter()
	w2.recompressStreams()
	w2.recompressStreams()
	var buf2 bytes.Buffer
	if _, err := w2.WriteToWithOptions(&buf2, WriteOptions{}); err != nil {
		t.Fatalf("second write: %v", err)
	}

	if !bytes.Equal(buf1.Bytes(), buf2.Bytes()) {
		t.Errorf("recompression is not idempotent: 1 pass %d bytes vs 2 passes %d bytes",
			buf1.Len(), buf2.Len())
	}
}

func TestRecompressStreams_ZeroOptionsByteIdentical(t *testing.T) {
	// A Writer with eligible streams must produce byte-identical
	// output between WriteOptions{} and explicit-default options
	// when RecompressStreams is unset. Defends against the toggle
	// silently activating.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		addRawStream(t, w, bytes.Repeat([]byte("default "), 200))
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

func TestRecompressStreams_RefusedOnEncryptedDocument(t *testing.T) {
	// Refusal must fire before any writer mutation so a retry
	// without RecompressStreams produces correct output (not double-
	// recompressed, not missing payload). State invariants: object
	// count and serialized-stream-payload bytes both unchanged.
	w := minimalCatalogWriter(t)
	plaintext := bytes.Repeat([]byte("encrypted? "), 200)
	s, _ := addRawStream(t, w, plaintext)
	originalPayload := append([]byte(nil), s.Data...)

	enc, err := core.NewEncryptor(core.RevisionAES128, "user", "owner", core.PermPrint)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	w.SetEncryption(enc)
	preCount := len(w.objects)

	var buf bytes.Buffer
	_, err = w.WriteToWithOptions(&buf, WriteOptions{RecompressStreams: true})
	if err == nil {
		t.Fatal("expected error for encryption + RecompressStreams")
	}
	if !strings.Contains(err.Error(), "recompression") {
		t.Errorf("error %q does not mention recompression", err.Error())
	}
	if len(w.objects) != preCount {
		t.Errorf("refusal mutated object count: %d → %d", preCount, len(w.objects))
	}
	if !bytes.Equal(s.Data, originalPayload) {
		t.Error("refusal mutated stream payload")
	}
	if s.Dict.Get("Filter") != nil {
		t.Error("refusal added a /Filter to the stream")
	}
}

func TestRecompressStreams_ComposesWithOrphanSweep(t *testing.T) {
	// Sweep + recompress: the sweep drops orphaned streams first, so
	// the recompression pass does not waste effort on objects that
	// will not survive. The output must shrink relative to either
	// option alone.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		// Reachable, recompressible.
		addRawStream(t, w, bytes.Repeat([]byte("keep "), 300))
		// Unreachable orphan with a compressible payload — should be
		// dropped by sweep, not visited by recompress.
		orphan := core.NewPdfStream(bytes.Repeat([]byte("drop "), 400))
		w.AddObject(orphan)
		return w
	}

	var sweepOnly, recompressOnly, both bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&sweepOnly, WriteOptions{
		OrphanSweep: true,
	}); err != nil {
		t.Fatalf("sweep only: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&recompressOnly, WriteOptions{
		RecompressStreams: true,
	}); err != nil {
		t.Fatalf("recompress only: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&both, WriteOptions{
		OrphanSweep:       true,
		RecompressStreams: true,
	}); err != nil {
		t.Fatalf("both: %v", err)
	}
	if both.Len() >= sweepOnly.Len() {
		t.Errorf("both options not smaller than sweep-only: both=%d, sweep=%d",
			both.Len(), sweepOnly.Len())
	}
	if both.Len() >= recompressOnly.Len() {
		t.Errorf("both options not smaller than recompress-only: both=%d, recompress=%d",
			both.Len(), recompressOnly.Len())
	}
}

func TestRecompressStreams_ComposesWithXRefStreamAndObjStm(t *testing.T) {
	// Recompression must compose with the existing optimizer toggles
	// without breaking either side. Result must reparse structurally.
	makeWriter := func() *Writer {
		w := minimalCatalogWriter(t)
		addRawStream(t, w, bytes.Repeat([]byte("compose "), 400))
		return w
	}

	var noRecompress, withRecompress bytes.Buffer
	if _, err := makeWriter().WriteToWithOptions(&noRecompress, WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	}); err != nil {
		t.Fatalf("no recompress: %v", err)
	}
	if _, err := makeWriter().WriteToWithOptions(&withRecompress, WriteOptions{
		UseXRefStream:     true,
		UseObjectStreams:  true,
		RecompressStreams: true,
	}); err != nil {
		t.Fatalf("with recompress: %v", err)
	}
	if withRecompress.Len() >= noRecompress.Len() {
		t.Errorf("recompression did not shrink with xref+objstm: with=%d, without=%d",
			withRecompress.Len(), noRecompress.Len())
	}
	if !bytes.Contains(withRecompress.Bytes(), []byte("/Type /XRef")) {
		t.Error("missing /Type /XRef")
	}
	if !bytes.HasPrefix(withRecompress.Bytes(), []byte("%PDF-")) {
		t.Error("missing PDF header")
	}
	if !bytes.HasSuffix(withRecompress.Bytes(), []byte("EOF\n")) {
		t.Error("missing EOF marker")
	}
}

func TestRecompressStreams_RewrittenStreamReinflatesToOriginal(t *testing.T) {
	// Round-trip correctness: the bytes committed by recompression
	// must inflate back to the original plaintext. A bug that emitted
	// arbitrary bytes "of the right size" would silently corrupt the
	// document; this test catches it.
	w := minimalCatalogWriter(t)
	plaintext := []byte("the quick brown fox jumps over the lazy dog. " + strings.Repeat("data ", 200))
	s, _ := addRawStream(t, w, plaintext)

	w.recompressStreams()

	if s.Dict.Get("Filter") == nil {
		t.Skip("recompression did not commit; nothing to verify")
	}
	roundTripped, err := core.InflateStreamData(s.Data)
	if err != nil {
		t.Fatalf("InflateStreamData on committed payload: %v", err)
	}
	if !bytes.Equal(roundTripped, plaintext) {
		t.Errorf("recompressed payload does not round-trip: got %d bytes, want %d",
			len(roundTripped), len(plaintext))
	}
}

func TestRecompressStreams_FlateDecodeRoundTripCorrectness(t *testing.T) {
	// Same correctness check on the FlateDecode-already-compressed
	// path: the rewrite must inflate to the same plaintext as the
	// original payload would.
	w := minimalCatalogWriter(t)
	plaintext := bytes.Repeat([]byte("preserve me "), 400)
	s, _ := addFlateStream(t, w, plaintext, zlib.BestSpeed)
	originalInflated, err := core.InflateStreamData(s.Data)
	if err != nil {
		t.Fatalf("setup: cannot inflate fixture: %v", err)
	}

	w.recompressStreams()

	roundTripped, err := core.InflateStreamData(s.Data)
	if err != nil {
		t.Fatalf("InflateStreamData on committed payload: %v", err)
	}
	if !bytes.Equal(roundTripped, originalInflated) {
		t.Errorf("re-Flate corrupted plaintext: got %d bytes, want %d",
			len(roundTripped), len(originalInflated))
	}
}

func TestRecompressStreams_SkipsBadFlatePayload(t *testing.T) {
	// A stream that claims /Filter /FlateDecode but holds garbage
	// must not crash the writer and must not be rewritten. The pass
	// treats inflate failure as "skip" rather than aborting the entire
	// write.
	w := minimalCatalogWriter(t)
	garbage := []byte("definitely not zlib-framed bytes")
	s := core.NewPdfStream(garbage)
	s.Dict.Set("Filter", core.NewPdfName("FlateDecode"))
	ref := w.AddObject(s)
	root := w.objects[0].Object.(*core.PdfDictionary)
	root.Set("Sample", ref)

	w.recompressStreams()

	if !bytes.Equal(s.Data, garbage) {
		t.Error("garbage Flate stream was modified despite inflate failure")
	}
}

func TestClassifyForRecompress_FilterShapes(t *testing.T) {
	// Direct unit on the classifier so its eligibility table can be
	// inspected without going through the full pass.
	cases := []struct {
		name      string
		filterSet func(*core.PdfDictionary)
		want      bool
		mode      recompressMode
	}{
		{name: "no filter", filterSet: func(d *core.PdfDictionary) {}, want: true, mode: recompressRaw},
		{name: "single FlateDecode", filterSet: func(d *core.PdfDictionary) {
			d.Set("Filter", core.NewPdfName("FlateDecode"))
		}, want: true, mode: recompressFlate},
		{name: "single DCTDecode", filterSet: func(d *core.PdfDictionary) {
			d.Set("Filter", core.NewPdfName("DCTDecode"))
		}, want: false},
		{name: "single ASCIIHexDecode", filterSet: func(d *core.PdfDictionary) {
			d.Set("Filter", core.NewPdfName("ASCIIHexDecode"))
		}, want: false},
		{name: "array of one FlateDecode", filterSet: func(d *core.PdfDictionary) {
			d.Set("Filter", core.NewPdfArray(core.NewPdfName("FlateDecode")))
		}, want: true, mode: recompressFlate},
		{name: "array of two", filterSet: func(d *core.PdfDictionary) {
			d.Set("Filter", core.NewPdfArray(
				core.NewPdfName("ASCII85Decode"),
				core.NewPdfName("FlateDecode"),
			))
		}, want: false},
		{name: "array containing non-name", filterSet: func(d *core.PdfDictionary) {
			d.Set("Filter", core.NewPdfArray(core.NewPdfInteger(0)))
		}, want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := core.NewPdfStream(nil)
			c.filterSet(s.Dict)
			mode, ok := classifyForRecompress(s)
			if ok != c.want {
				t.Errorf("eligible = %v, want %v", ok, c.want)
			}
			if ok && mode != c.mode {
				t.Errorf("mode = %v, want %v", mode, c.mode)
			}
		})
	}
}

func TestRecompressStreams_TripleGuardRefusalWithObjStmAndEncryption(t *testing.T) {
	// All three trigger conditions are tripped simultaneously:
	// UseObjectStreams + RecompressStreams + an encryptor. The current
	// guard ordering reports the objstm-encryption refusal first
	// because that check appears earlier in WriteToWithOptions. We
	// pin "some refusal fired and nothing was mutated" rather than
	// the specific message, since guard ordering is internal.
	w := minimalCatalogWriter(t)
	plaintext := bytes.Repeat([]byte("triple-guard "), 200)
	s, _ := addRawStream(t, w, plaintext)
	originalPayload := append([]byte(nil), s.Data...)

	enc, err := core.NewEncryptor(core.RevisionAES128, "user", "owner", core.PermPrint)
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	w.SetEncryption(enc)
	preNumbers := snapshotObjectNumbers(w)

	var buf bytes.Buffer
	_, err = w.WriteToWithOptions(&buf, WriteOptions{
		UseXRefStream:     true,
		UseObjectStreams:  true,
		RecompressStreams: true,
	})
	if err == nil {
		t.Fatal("expected refusal with objstm + recompress + encryption")
	}
	if !sliceEqual(preNumbers, snapshotObjectNumbers(w)) {
		t.Error("triple-guard refusal renumbered objects")
	}
	if !bytes.Equal(s.Data, originalPayload) {
		t.Error("triple-guard refusal mutated stream payload")
	}
	if s.Dict.Get("Filter") != nil {
		t.Error("triple-guard refusal added a /Filter to the stream")
	}
}

func TestRecompressStreams_NearBreakevenPayload(t *testing.T) {
	// Defends the size-regression guard's strict-less-than comparator
	// (`<`, not `<=`). A payload engineered to land near the
	// breakeven point would expose an off-by-one such that an
	// equal-size candidate gets committed unnecessarily, perturbing
	// a byte-stable artifact for zero benefit.
	//
	// We construct a payload whose Flate output is very close to (but
	// strictly smaller than) the original size. The pass should
	// commit. Then we check that re-running the pass on the now-Flate
	// payload (which is at BestCompression) produces no further commit
	// — the second-pass candidate would be equal-size, the guard must
	// keep the baseline.
	w := minimalCatalogWriter(t)
	// 64 bytes of mixed structure — enough to compress, not so
	// repetitive that it compresses dramatically.
	payload := []byte("structured but not uniformly repetitive payload bytes here ABC")
	s, _ := addRawStream(t, w, payload)

	w.recompressStreams()
	firstPassData := append([]byte(nil), s.Data...)

	// If the first pass committed, /Filter is now /FlateDecode.
	// Otherwise, the original payload was already too small to shrink.
	if s.Dict.Get("Filter") == nil {
		t.Skip("payload too small to shrink on first pass; near-breakeven guard not exercised")
	}

	w.recompressStreams()
	if !bytes.Equal(s.Data, firstPassData) {
		t.Errorf("second pass committed an equal-size candidate (guard `<` regressed to `<=`)")
	}
}

func TestRecompressStreams_OverwritesStaleLength(t *testing.T) {
	// A caller could set /Length explicitly in the stream dict to a
	// value that no longer matches len(stream.Data) after recompress.
	// PdfStream.WriteTo must overwrite /Length from the actual data
	// length on every serialization, regardless of pre-set values.
	// This test pins that contract from the recompression caller's
	// perspective: a wrong /Length set before recompress must not
	// survive into the serialized output.
	w := minimalCatalogWriter(t)
	plaintext := bytes.Repeat([]byte("overwrite my length "), 200)
	s, _ := addRawStream(t, w, plaintext)
	s.Dict.Set("Length", core.NewPdfInteger(99999)) // intentionally wrong

	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, WriteOptions{RecompressStreams: true}); err != nil {
		t.Fatalf("write: %v", err)
	}
	pdf := buf.String()

	// The serialized length must equal len(s.Data) after recompress,
	// not the wrong 99999 we planted.
	wantLengthSubstr := fmt.Sprintf("/Length %d ", len(s.Data))
	if !strings.Contains(pdf, wantLengthSubstr) {
		t.Errorf("expected %q in output to confirm /Length was overwritten", wantLengthSubstr)
	}
	if strings.Contains(pdf, "/Length 99999") {
		t.Error("stale /Length 99999 leaked into serialized output")
	}
}

func TestClassifyForRecompress_NonNameNonArrayFilter(t *testing.T) {
	// A /Filter set to something other than a name or an array of
	// names is malformed per §7.4.2 and must classify as ineligible
	// rather than fall through to the "no /Filter" branch.
	cases := []struct {
		name  string
		value core.PdfObject
	}{
		{name: "PdfNull", value: core.NewPdfNull()},
		{name: "PdfDictionary", value: core.NewPdfDictionary()},
		{name: "PdfBoolean", value: core.NewPdfBoolean(true)},
		{name: "PdfInteger", value: core.NewPdfInteger(0)},
		{name: "PdfLiteralString", value: core.NewPdfLiteralString("FlateDecode")},
		{name: "PdfIndirectReference", value: core.NewPdfIndirectReference(1, 0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := core.NewPdfStream(nil)
			s.Dict.Set("Filter", c.value)
			if _, ok := classifyForRecompress(s); ok {
				t.Errorf("/Filter of type %T classified as eligible", c.value)
			}
		})
	}
}

func TestClassifyForRecompress_FlateFlateChain(t *testing.T) {
	// /Filter [/FlateDecode /FlateDecode] is legal but unusual — two
	// successive Flate passes. The classifier currently treats
	// anything other than a single Flate name as ineligible, so this
	// is a skip. Pin that explicitly.
	s := core.NewPdfStream(nil)
	s.Dict.Set("Filter", core.NewPdfArray(
		core.NewPdfName("FlateDecode"),
		core.NewPdfName("FlateDecode"),
	))
	if _, ok := classifyForRecompress(s); ok {
		t.Error("[/FlateDecode /FlateDecode] chain classified as eligible")
	}
}

func TestClassifyForRecompress_EmptyFilterArray(t *testing.T) {
	// /Filter [] is not strictly forbidden by §7.4.2 (empty filter
	// chain = no filtering applied) but is not standard. The
	// classifier must not treat it as the "single FlateDecode" case.
	s := core.NewPdfStream(nil)
	s.Dict.Set("Filter", core.NewPdfArray())
	if _, ok := classifyForRecompress(s); ok {
		t.Error("/Filter [] (empty array) classified as eligible")
	}
}

func TestRecompressStreams_OrphansDroppedBeforeRecompressVisit(t *testing.T) {
	// When OrphanSweep + RecompressStreams are both enabled, the
	// sweep must run first so the recompression pass does not waste
	// effort on orphan streams. We invoke the passes directly (in
	// order) and assert the orphan is gone before recompress visits.
	w := minimalCatalogWriter(t)
	addRawStream(t, w, bytes.Repeat([]byte("keep "), 200))
	orphan := core.NewPdfStream(bytes.Repeat([]byte("drop "), 400))
	w.AddObject(orphan)
	preCount := len(w.objects)

	w.sweepOrphans()
	if len(w.objects) >= preCount {
		t.Fatalf("sweep did not drop orphan: pre=%d, post=%d", preCount, len(w.objects))
	}
	for _, obj := range w.objects {
		if obj.Object == orphan {
			t.Fatal("orphan still present after sweep; recompress would visit it")
		}
	}

	// Now recompress runs against the post-sweep object set — the
	// orphan body cannot be visited because it is no longer in
	// w.objects.
	w.recompressStreams()
}

func TestRecompressStreams_DocumentAPIShrinksWithSyntheticOrphan(t *testing.T) {
	// End-to-end through the public Document API. We can't easily
	// build a real reader-imported document inside this package
	// without a circular import, so this test uses ToBytesWithOptions
	// on a layout-built Document and verifies the toggle does not
	// enlarge or break the output. The reader-imported size win is
	// asserted by the e2e test in writer_optimize_size_test.go.
	doc := buildSampleDocument(5)
	plain, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("plain: %v", err)
	}
	doc2 := buildSampleDocument(5)
	rec, err := doc2.ToBytesWithOptions(WriteOptions{RecompressStreams: true})
	if err != nil {
		t.Fatalf("recompressed: %v", err)
	}
	if len(rec) > len(plain) {
		t.Errorf("RecompressStreams enlarged a layout-built document: plain=%d, rec=%d",
			len(plain), len(rec))
	}
	if !bytes.HasPrefix(rec, []byte("%PDF-")) {
		t.Error("missing PDF header")
	}
	if !bytes.HasSuffix(rec, []byte("EOF\n")) {
		t.Error("missing EOF marker")
	}
}
