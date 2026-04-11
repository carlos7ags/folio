// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// buildSampleDocument constructs a representative multi-page document
// used by the size-regression test and benchmark. The shape — repeated
// headings and paragraphs — exercises the page tree, content streams,
// and resource dictionaries that benefit from xref stream and object
// stream packing.
func buildSampleDocument(sections int) *Document {
	doc := NewDocument(PageSizeLetter)
	doc.Info.Title = "Optimization size test"
	for i := 1; i <= sections; i++ {
		doc.Add(layout.NewHeading(fmt.Sprintf("Section %d", i), layout.H1))
		doc.Add(layout.NewParagraph(
			"Lorem ipsum dolor sit amet, consectetur adipiscing elit. "+
				"Sed do eiusmod tempor incididunt ut labore et dolore magna "+
				"aliqua. Ut enim ad minim veniam, quis nostrud exercitation "+
				"ullamco laboris nisi ut aliquip ex ea commodo consequat.",
			font.Helvetica, 11,
		))
	}
	return doc
}

func TestOptimizerShrinksRealDocument(t *testing.T) {
	// End-to-end size assertion: a real Document built through the
	// public layout API must shrink when the optimizer options are
	// enabled. The threshold is intentionally modest (5 percent)
	// because content streams dominate text-heavy documents and are
	// already Flate-compressed; the win comes from the metadata, the
	// page tree, the resources, and the xref itself.
	const sections = 25
	const minSavingPct = 5.0

	tradBytes, err := buildSampleDocument(sections).ToBytes()
	if err != nil {
		t.Fatalf("default write: %v", err)
	}
	xstmBytes, err := buildSampleDocument(sections).ToBytesWithOptions(WriteOptions{
		UseXRefStream: true,
	})
	if err != nil {
		t.Fatalf("xref-stream write: %v", err)
	}
	optBytes, err := buildSampleDocument(sections).ToBytesWithOptions(WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	})
	if err != nil {
		t.Fatalf("optimizer write: %v", err)
	}

	if xstmBytes := len(xstmBytes); xstmBytes > len(tradBytes) {
		t.Errorf("xref stream output (%d bytes) larger than default (%d bytes)",
			xstmBytes, len(tradBytes))
	}
	if len(optBytes) > len(xstmBytes) {
		t.Errorf("optimizer output (%d bytes) larger than xref-stream-only (%d bytes)",
			len(optBytes), len(xstmBytes))
	}

	saved := len(tradBytes) - len(optBytes)
	pct := 100.0 * float64(saved) / float64(len(tradBytes))
	if pct < minSavingPct {
		t.Errorf("optimizer saved only %.1f%% (%d bytes), want at least %.1f%%",
			pct, saved, minSavingPct)
	}
	t.Logf("default=%d bytes, xref-stream=%d bytes, optimized=%d bytes, saved=%.1f%% (%d bytes)",
		len(tradBytes), len(xstmBytes), len(optBytes), pct, saved)
}

func TestOptimizerOutputIsValidPDF(t *testing.T) {
	// A defensive structural check on the optimized output for a real
	// Document: the file must start with %PDF-, end with %%EOF, and
	// contain a /Type /XRef stream.
	doc := buildSampleDocument(10)
	out, err := doc.ToBytesWithOptions(WriteOptions{
		UseXRefStream:    true,
		UseObjectStreams: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Error("missing %PDF- header")
	}
	if !bytes.HasSuffix(out, []byte("%%EOF\n")) {
		t.Error("missing EOF marker")
	}
	if !bytes.Contains(out, []byte("/Type /XRef")) {
		t.Error("missing /Type /XRef")
	}
}

// BenchmarkWriteOptimized50 measures the cost of writing a 50-section
// document with the optimizer options enabled. Reported alongside
// BenchmarkMultiPage50 in bench_test.go, this gives a side-by-side
// view of the time and allocation cost of the optimized path.
func BenchmarkWriteOptimized50(b *testing.B) {
	for range b.N {
		doc := buildSampleDocument(50)
		_, _ = doc.WriteToWithOptions(io.Discard, WriteOptions{
			UseXRefStream:    true,
			UseObjectStreams: true,
		})
	}
}
