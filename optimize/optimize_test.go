// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package optimize

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
)

// buildPdfASample constructs a PDF/A-3B document with an XML file
// attachment (a Factur-X-style e-invoice), which writes /OutputIntents,
// /Metadata, /AF, /Names/EmbeddedFiles, and /MarkInfo onto the catalog —
// exactly the compliance data optimize.Bytes must refuse to drop
// silently.
func buildPdfASample(t *testing.T) []byte {
	t.Helper()
	doc := document.NewDocument(document.PageSizeA4)
	doc.Info.Title = "Invoice"
	doc.AddPage()
	doc.SetPdfA(document.PdfAConfig{Level: document.PdfA3B})
	doc.AttachFile(document.FileAttachment{
		FileName:       "factur-x.xml",
		MIMEType:       "application/xml",
		AFRelationship: "Alternative",
		Data:           []byte("<xml/>"),
	})
	src, err := doc.ToBytes()
	if err != nil {
		t.Fatalf("build PDF/A sample: %v", err)
	}
	return src
}

// TestBytesRefusesLossyInput confirms Bytes refuses to silently strip
// PDF/A-3B compliance data (output intent, XMP metadata, attachment,
// tagging) from a Factur-X-style document.
func TestBytesRefusesLossyInput(t *testing.T) {
	src := buildPdfASample(t)

	_, _, err := Bytes(src)
	if !errors.Is(err, ErrLossyInput) {
		t.Fatalf("Bytes error = %v, want ErrLossyInput", err)
	}
	if !strings.Contains(err.Error(), "OutputIntents") {
		t.Errorf("error %q does not mention OutputIntents", err.Error())
	}
}

// TestBytesLossyOptInPasses confirms Options{AllowLossy: true} lets the
// same compliance-bearing document through, at the cost of the dropped
// data, and that the result still round-trips.
func TestBytesLossyOptInPasses(t *testing.T) {
	src := buildPdfASample(t)

	out, stats, err := Bytes(src, Options{AllowLossy: true})
	if err != nil {
		t.Fatalf("Bytes with AllowLossy: %v", err)
	}
	if stats.BytesIn != len(src) {
		t.Errorf("stats.BytesIn = %d, want %d", stats.BytesIn, len(src))
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
		t.Errorf("page count changed: before=%d after=%d", before.PageCount(), after.PageCount())
	}
}

// TestBytesCleanInputUnaffected guards against false positives: a plain
// document with no PDF/A conformance or attachments optimizes without
// error and without passing Options.
func TestBytesCleanInputUnaffected(t *testing.T) {
	src, err := buildSample(5).ToBytes()
	if err != nil {
		t.Fatalf("build source: %v", err)
	}

	if _, _, err := Bytes(src); err != nil {
		t.Fatalf("Bytes on clean input: %v", err)
	}
}

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

// inheritedAttrsFixture is a hand-written raw PDF where the root Pages
// node carries MediaBox, Rotate, and Resources, and the first leaf page
// declares none of them (relying on inheritance per ISO 32000 §7.7.3.4).
// The second leaf page overrides MediaBox with its own value, to prove
// page-level values are never clobbered by ancestor ones.
//
// This carries a byte-exact classic xref table rather than relying on
// reader.Parse's tolerant-mode xref repair: repair scans for "N G obj"
// markers, but mistakes some "N G R" indirect references (of which this
// fixture — being small and reference-heavy — has several) for object
// headers, corrupting the offset table. A correct xref avoids the
// repair path entirely.
const inheritedAttrsFixture = "%PDF-1.7\n" +
	"1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
	"2 0 obj\n<< /Type /Pages /Kids [3 0 R 6 0 R] /Count 2 /MediaBox [0 0 612 792] /Rotate 90 /Resources << /Font << /F1 5 0 R >> >> >>\nendobj\n" +
	"3 0 obj\n<< /Type /Page /Parent 2 0 R /Contents 4 0 R >>\nendobj\n" +
	"4 0 obj\n<< /Length 23 >>\nstream\nBT /F1 12 Tf (Hi) Tj ET\nendstream\nendobj\n" +
	"5 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n" +
	"6 0 obj\n<< /Type /Page /Parent 2 0 R /Contents 4 0 R /MediaBox [0 0 300 300] >>\nendobj\n" +
	"xref\n0 7\n" +
	"0000000000 65535 f \n" +
	"0000000009 00000 n \n" +
	"0000000058 00000 n \n" +
	"0000000195 00000 n \n" +
	"0000000258 00000 n \n" +
	"0000000331 00000 n \n" +
	"0000000401 00000 n \n" +
	"trailer\n<< /Root 1 0 R /Size 7 >>\n" +
	"startxref\n488\n%%EOF\n"

// TestBytesInheritedPageAttributes proves the optimize rewrite
// materializes MediaBox, Rotate, and Resources inherited from an
// ancestor Pages node onto the leaf page dict it copies, and that a
// page which declares its own MediaBox keeps that value rather than
// being overwritten by the ancestor's.
func TestBytesInheritedPageAttributes(t *testing.T) {
	fixture := []byte(inheritedAttrsFixture)

	r, err := reader.Parse(fixture)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if r.PageCount() != 2 {
		t.Fatalf("PageCount = %d, want 2", r.PageCount())
	}
	page0, err := r.Page(0)
	if err != nil {
		t.Fatalf("page 0: %v", err)
	}
	if page0.Dict().Get("MediaBox") != nil {
		t.Fatalf("fixture sanity check failed: leaf page 0 should not declare its own MediaBox")
	}
	if page0.MediaBox.Width() != 612 || page0.MediaBox.Height() != 792 {
		t.Fatalf("fixture sanity check failed: page 0 MediaBox = %v, want 612x792 (inherited)", page0.MediaBox)
	}

	// rewrite is exercised directly (rather than through Bytes) because
	// Bytes falls back to the original input when the rewrite does not
	// shrink it, which this tiny fixture would trigger.
	out, err := rewrite(r)
	if err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	after, err := reader.Parse(out)
	if err != nil {
		t.Fatalf("parse rewritten output: %v", err)
	}
	if after.PageCount() != 2 {
		t.Fatalf("PageCount after rewrite = %d, want 2", after.PageCount())
	}

	p0, err := after.Page(0)
	if err != nil {
		t.Fatalf("page 0 after rewrite: %v", err)
	}
	if p0.MediaBox.Width() != 612 || p0.MediaBox.Height() != 792 {
		t.Errorf("page 0 MediaBox = %v, want 612x792", p0.MediaBox)
	}
	if p0.Rotate != 90 {
		t.Errorf("page 0 Rotate = %d, want 90", p0.Rotate)
	}
	res, err := p0.Resources()
	if err != nil {
		t.Fatalf("page 0 Resources: %v", err)
	}
	if res.Get("Font") == nil {
		t.Errorf("page 0 Resources missing Font after rewrite")
	}
	if p0.Width <= 0 || p0.Height <= 0 {
		t.Errorf("page 0 renders degenerately: width=%v height=%v", p0.Width, p0.Height)
	}

	// Guard case: page 1 declared its own MediaBox and must keep it,
	// not the ancestor's.
	p1, err := after.Page(1)
	if err != nil {
		t.Fatalf("page 1 after rewrite: %v", err)
	}
	if p1.MediaBox.Width() != 300 || p1.MediaBox.Height() != 300 {
		t.Errorf("page 1 MediaBox = %v, want 300x300 (own value, not inherited)", p1.MediaBox)
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
