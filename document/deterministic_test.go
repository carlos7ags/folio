// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

// writeOpts renders a freshly built document with the given options and
// returns the serialized bytes, failing the test on any write error.
func writeOpts(t *testing.T, build func() *Document, opts WriteOptions) []byte {
	t.Helper()
	var buf bytes.Buffer
	if _, err := build().WriteToWithOptions(&buf, opts); err != nil {
		t.Fatalf("WriteToWithOptions failed: %v", err)
	}
	return buf.Bytes()
}

// newPdfADoc builds a minimal PDF/A-2B document. PDF/A is the path that
// carries both nondeterminism sources folio had: a random trailer /ID and
// wall-clock XMP timestamps.
func newPdfADoc() *Document {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Deterministic Test"
	doc.Info.Author = "Folio"
	doc.SetPdfA(PdfAConfig{Level: PdfA2B})
	return doc
}

// TestPdfANondeterministicByDefault documents the baseline: without the
// Deterministic option, two renders of identical input differ because the
// trailer /ID is randomly generated. This is the behavior the option fixes.
func TestPdfANondeterministicByDefault(t *testing.T) {
	a := writeOpts(t, newPdfADoc, WriteOptions{})
	b := writeOpts(t, newPdfADoc, WriteOptions{})
	if bytes.Equal(a, b) {
		t.Fatal("expected default PDF/A output to differ between renders (random /ID); got byte-identical output")
	}
}

// TestDeterministicPdfAByteIdentical is the core guarantee: identical input
// with Deterministic set yields byte-identical output, even with no
// Info.CreationDate (the XMP timestamp falls back to the fixed zero time
// rather than time.Now).
func TestDeterministicPdfAByteIdentical(t *testing.T) {
	a := writeOpts(t, newPdfADoc, WriteOptions{Deterministic: true})
	b := writeOpts(t, newPdfADoc, WriteOptions{Deterministic: true})
	if !bytes.Equal(a, b) {
		t.Fatalf("expected byte-identical PDF/A output with Deterministic; lengths %d vs %d", len(a), len(b))
	}
}

// TestDeterministicPdfAWithCreationDate confirms determinism holds when the
// caller supplies Info.CreationDate, which is the recommended usage.
func TestDeterministicPdfAWithCreationDate(t *testing.T) {
	build := func() *Document {
		doc := newPdfADoc()
		doc.Info.CreationDate = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
		doc.Info.ModDate = doc.Info.CreationDate
		return doc
	}
	a := writeOpts(t, build, WriteOptions{Deterministic: true})
	b := writeOpts(t, build, WriteOptions{Deterministic: true})
	if !bytes.Equal(a, b) {
		t.Fatal("expected byte-identical output with Deterministic and a set CreationDate")
	}
}

// TestDeterministicFileIDIsContentSensitive verifies the derived /ID is a
// content digest, not a constant: documents that differ produce different
// identifiers (and therefore different bytes).
func TestDeterministicFileIDIsContentSensitive(t *testing.T) {
	other := func() *Document {
		doc := newPdfADoc()
		doc.Info.Title = "A Different Title"
		return doc
	}
	a := writeOpts(t, newPdfADoc, WriteOptions{Deterministic: true})
	b := writeOpts(t, other, WriteOptions{Deterministic: true})
	if bytes.Equal(a, b) {
		t.Fatal("expected different documents to produce different deterministic output")
	}
}

// TestSetFileIDWritesTrailerID checks that SetFileID gives a plain (non-PDF/A)
// document a trailer /ID it would otherwise lack, written as the uppercase
// hex of the supplied bytes, and that the result is reproducible.
func TestSetFileIDWritesTrailerID(t *testing.T) {
	id := make([]byte, 16)
	for i := range id {
		id[i] = byte(i + 1)
	}
	build := func() *Document {
		doc := NewDocument(PageSizeLetter)
		doc.SetFileID(id)
		return doc
	}
	out := writeOpts(t, build, WriteOptions{})

	wantHex := fmt.Sprintf("<%X>", id) // e.g. <0102...10>
	want := "/ID [" + wantHex + " " + wantHex + "]"
	if !bytes.Contains(out, []byte(want)) {
		t.Fatalf("expected trailer to contain %q; output did not", want)
	}

	again := writeOpts(t, build, WriteOptions{})
	if !bytes.Equal(out, again) {
		t.Fatal("expected SetFileID output to be reproducible across renders")
	}
}

// TestSetFileIDOverridesDeterministic verifies an explicit /ID wins over the
// digest derivation when both Deterministic and SetFileID are in play.
func TestSetFileIDOverridesDeterministic(t *testing.T) {
	id := bytes.Repeat([]byte{0xAB}, 16)
	build := func() *Document {
		doc := newPdfADoc()
		doc.SetFileID(id)
		return doc
	}
	out := writeOpts(t, build, WriteOptions{Deterministic: true})

	wantHex := fmt.Sprintf("<%X>", id)
	if !bytes.Contains(out, []byte("/ID ["+wantHex+" "+wantHex+"]")) {
		t.Fatalf("expected explicit /ID %s to override the derived digest", wantHex)
	}
}

// TestDeterministicNonPdfAGainsID confirms a plain document, which normally
// has no /ID, gains a stable one under Deterministic and renders identically.
func TestDeterministicNonPdfAGainsID(t *testing.T) {
	plain := func() *Document { return NewDocument(PageSizeLetter) }

	def := writeOpts(t, plain, WriteOptions{})
	if bytes.Contains(def, []byte("/ID")) {
		t.Fatal("precondition: a plain document should carry no /ID by default")
	}

	a := writeOpts(t, plain, WriteOptions{Deterministic: true})
	if !bytes.Contains(a, []byte("/ID")) {
		t.Fatal("expected Deterministic to add a trailer /ID to a plain document")
	}
	b := writeOpts(t, plain, WriteOptions{Deterministic: true})
	if !bytes.Equal(a, b) {
		t.Fatal("expected deterministic plain-document output to be reproducible")
	}
}

// TestDeterministicTeeAcrossOptionSets verifies the /ID tee produces
// byte-stable output for Deterministic with and without UseXRefStream —
// the two code paths that call finishID from different trailer writers.
func TestDeterministicTeeAcrossOptionSets(t *testing.T) {
	cases := []struct {
		name string
		opts WriteOptions
	}{
		{"traditional trailer", WriteOptions{Deterministic: true}},
		{"xref stream trailer", WriteOptions{Deterministic: true, UseXRefStream: true}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := writeOpts(t, newPdfADoc, c.opts)
			b := writeOpts(t, newPdfADoc, c.opts)
			if !bytes.Equal(a, b) {
				t.Fatalf("expected byte-identical output for %s", c.name)
			}
		})
	}
}

// TestDeterministicTeeObjectStreamsPath verifies the /ID tee also covers
// the object-stream writer path (writeXRefStreamWithObjStms), which has
// its own finishID call site.
func TestDeterministicTeeObjectStreamsPath(t *testing.T) {
	opts := WriteOptions{Deterministic: true, UseXRefStream: true, UseObjectStreams: true}
	a := writeOpts(t, newPdfADoc, opts)
	b := writeOpts(t, newPdfADoc, opts)
	if !bytes.Equal(a, b) {
		t.Fatal("expected byte-identical output on the object-stream path")
	}
	if !bytes.Contains(a, []byte("/ID")) {
		t.Fatal("expected trailer to contain /ID on the object-stream path")
	}
}

// TestSetFileIDOverridesDeterministicObjectStreamsPath is
// TestSetFileIDOverridesDeterministic's counterpart for the object-stream
// writer path: an explicit /ID must still win over the tee-derived digest.
func TestSetFileIDOverridesDeterministicObjectStreamsPath(t *testing.T) {
	id := bytes.Repeat([]byte{0xCD}, 16)
	build := func() *Document {
		doc := newPdfADoc()
		doc.SetFileID(id)
		return doc
	}
	out := writeOpts(t, build, WriteOptions{Deterministic: true, UseXRefStream: true, UseObjectStreams: true})

	wantHex := fmt.Sprintf("<%X>", id)
	if !bytes.Contains(out, []byte("/ID ["+wantHex+" "+wantHex+"]")) {
		t.Fatalf("expected explicit /ID %s to override the derived digest on the object-stream path", wantHex)
	}
}
