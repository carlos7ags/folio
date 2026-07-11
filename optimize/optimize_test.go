// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package optimize

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
)

// buildSample constructs a multi-page, multi-section document that
// re-uses the same font and heading text across pages — the shape
// where dedup and recompression both have something to work with.
func buildSample(sections int) *document.Document {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Optimize sample"
	for i := 1; i <= sections; i++ {
		doc.Add(layout.NewHeading(fmt.Sprintf("Section %d", i), layout.H1))
		doc.Add(layout.NewParagraph(
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do "+
				"eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut "+
				"enim ad minim veniam, quis nostrud exercitation ullamco laboris.",
			font.Helvetica, 11,
		))
	}
	return doc
}

// TestBytesRoundTripEquivalence proves the rewrite is content-preserving:
// the optimized PDF re-parses, has the same page count and per-page
// geometry, the same extracted text on every page, and is not larger
// than the input.
func TestBytesRoundTripEquivalence(t *testing.T) {
	src, err := buildSample(20).ToBytes()
	if err != nil {
		t.Fatalf("build source: %v", err)
	}

	out, stats, err := Bytes(src)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if len(out) > len(src) {
		t.Errorf("optimized output (%d bytes) larger than input (%d bytes)", len(out), len(src))
	}
	if stats.BytesIn != len(src) || stats.BytesOut != len(out) {
		t.Errorf("stats = %+v, want BytesIn=%d BytesOut=%d", stats, len(src), len(out))
	}
	if stats.SavedBytes() < 0 {
		t.Errorf("SavedBytes() = %d, want >= 0", stats.SavedBytes())
	}

	before, err := reader.Parse(src)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	after, err := reader.Parse(out)
	if err != nil {
		t.Fatalf("parse optimized output: %v", err)
	}

	if before.PageCount() != after.PageCount() {
		t.Fatalf("page count changed: before=%d after=%d", before.PageCount(), after.PageCount())
	}

	for i := range before.PageCount() {
		bp, err := before.Page(i)
		if err != nil {
			t.Fatalf("page %d (before): %v", i, err)
		}
		ap, err := after.Page(i)
		if err != nil {
			t.Fatalf("page %d (after): %v", i, err)
		}

		if bp.VisibleBox() != ap.VisibleBox() {
			t.Errorf("page %d: geometry changed: before=%v after=%v", i, bp.VisibleBox(), ap.VisibleBox())
		}

		bText, err := bp.ExtractText()
		if err != nil {
			t.Fatalf("page %d ExtractText (before): %v", i, err)
		}
		aText, err := ap.ExtractText()
		if err != nil {
			t.Fatalf("page %d ExtractText (after): %v", i, err)
		}
		if bText != aText {
			t.Errorf("page %d: extracted text changed:\nbefore: %q\nafter:  %q", i, bText, aText)
		}
	}

	t.Logf("input=%d bytes, output=%d bytes, saved=%.1f%% (%d bytes)",
		stats.BytesIn, stats.BytesOut, stats.SavedPercent(), stats.SavedBytes())
}

// TestBytesShrinksRepresentativeDocument records a representative size
// reduction on a larger fixture, mirroring examples/optimize's
// comparison table.
func TestBytesShrinksRepresentativeDocument(t *testing.T) {
	const sections = 40
	const minSavingPct = 5.0

	src, err := buildSample(sections).ToBytes()
	if err != nil {
		t.Fatalf("build source: %v", err)
	}

	out, stats, err := Bytes(src)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if len(out) > len(src) {
		t.Fatalf("optimized output (%d bytes) larger than input (%d bytes)", len(out), len(src))
	}
	if stats.SavedPercent() < minSavingPct {
		t.Errorf("saved only %.1f%% (%d bytes), want at least %.1f%%",
			stats.SavedPercent(), stats.SavedBytes(), minSavingPct)
	}
	t.Logf("sections=%d input=%d bytes output=%d bytes saved=%.1f%%",
		sections, stats.BytesIn, stats.BytesOut, stats.SavedPercent())
}

// TestBytesFallsBackWhenRewriteWouldGrow exercises the size guard
// directly on a document too small for the rewrite's fixed overhead
// (xref stream dictionary, object stream headers) to pay off — Bytes
// must hand back the original bytes rather than a larger file.
func TestBytesFallsBackWhenRewriteWouldGrow(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.AddPage()
	src, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("build source: %v", err)
	}

	out, stats, err := Bytes(src)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	if !bytes.Equal(out, src) {
		t.Fatalf("expected fallback to original bytes when rewrite does not shrink")
	}
	if stats.BytesIn != len(src) || stats.BytesOut != len(src) {
		t.Errorf("stats = %+v, want BytesIn=BytesOut=%d", stats, len(src))
	}
}

// TestBytesEncryptedInputRefused confirms Bytes reports ErrEncrypted
// instead of attempting to rewrite ciphertext it cannot interpret.
func TestBytesEncryptedInputRefused(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetEncryption(document.EncryptionConfig{
		Algorithm:    document.EncryptAES256,
		UserPassword: "secret",
	})
	doc.Add(layout.NewParagraph("Hello, encrypted world!", font.Helvetica, 12))

	src, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("build encrypted source: %v", err)
	}

	_, _, err = Bytes(src)
	if !errors.Is(err, ErrEncrypted) {
		t.Fatalf("Bytes error = %v, want ErrEncrypted", err)
	}
}

// TestBytesEncryptedEmptyPasswordRefused confirms Bytes still reports
// ErrEncrypted for a document that decrypts successfully under the
// empty password (an owner-password-only PDF, the common "restrict
// permissions" case) rather than silently stripping its encryption.
func TestBytesEncryptedEmptyPasswordRefused(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.SetEncryption(document.EncryptionConfig{
		Algorithm:     document.EncryptAES256,
		OwnerPassword: "admin",
	})
	doc.Add(layout.NewParagraph("Hello, encrypted world!", font.Helvetica, 12))

	src, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("build encrypted source: %v", err)
	}

	if _, err := reader.Parse(src); err != nil {
		t.Fatalf("sanity check: reader.Parse should open with empty password, got: %v", err)
	}

	_, _, err = Bytes(src)
	if !errors.Is(err, ErrEncrypted) {
		t.Fatalf("Bytes error = %v, want ErrEncrypted", err)
	}
}

// TestBytesInvalidInput confirms malformed input produces a plain
// parse error, not ErrEncrypted.
func TestBytesInvalidInput(t *testing.T) {
	_, _, err := Bytes([]byte("not a pdf"))
	if err == nil {
		t.Fatal("expected an error for non-PDF input")
	}
	if errors.Is(err, ErrEncrypted) {
		t.Fatalf("unexpected ErrEncrypted for malformed input: %v", err)
	}
}

// TestStatsHelpers exercises the Stats derived-value helpers directly.
func TestStatsHelpers(t *testing.T) {
	s := Stats{BytesIn: 1000, BytesOut: 750}
	if got := s.SavedBytes(); got != 250 {
		t.Errorf("SavedBytes() = %d, want 250", got)
	}
	if got := s.SavedPercent(); got != 25 {
		t.Errorf("SavedPercent() = %v, want 25", got)
	}

	zero := Stats{}
	if got := zero.SavedPercent(); got != 0 {
		t.Errorf("SavedPercent() on zero Stats = %v, want 0", got)
	}
}

// TestBytesDedupesSharedResources builds a document via document.Add
// where every page shares the same Helvetica font resource, then
// confirms the rewrite still parses and preserves every page after
// dedup and object-stream packing run.
func TestBytesDedupesSharedResources(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	for i := 0; i < 10; i++ {
		p := doc.AddPage()
		p.AddText(fmt.Sprintf("Page %d", i), font.Helvetica, 12, 72, 700)
	}
	src, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("build source: %v", err)
	}

	out, _, err := Bytes(src)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}

	r, err := reader.Parse(out)
	if err != nil {
		t.Fatalf("parse optimized output: %v", err)
	}
	if r.PageCount() != 10 {
		t.Fatalf("PageCount = %d, want 10", r.PageCount())
	}
	for i := range r.PageCount() {
		page, err := r.Page(i)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		res, err := page.Resources()
		if err != nil {
			t.Fatalf("page %d Resources: %v", i, err)
		}
		if res.Get("Font") == nil {
			t.Errorf("page %d: missing Font resources after rewrite", i)
		}
	}
}
