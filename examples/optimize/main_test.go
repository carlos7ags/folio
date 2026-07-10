// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

// TestOptimizeExampleFixturesProduceValidPDFs runs the example's
// writeAll pipeline (the same one main() drives) against every
// fixture and asserts each of the five comparison outputs is a valid,
// parseable PDF. A regression in any WriteOptions combination would
// surface here rather than only in the printed byte-count table a
// human has to eyeball.
func TestOptimizeExampleFixturesProduceValidPDFs(t *testing.T) {
	fixtures := []fixture{
		{name: "text-heavy", build: textHeavy},
		{name: "many empty pages", build: manyPages},
		{name: "table-heavy", build: tableHeavy},
		{name: "imported text-heavy", build: importedTextHeavy},
	}

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			def, packed, swept, recompress, full, err := writeAll(f)
			if err != nil {
				t.Fatalf("writeAll: %v", err)
			}
			for label, pdf := range map[string][]byte{
				"default":     def,
				"xref+obj":    packed,
				"+sweep":      swept,
				"+recompress": recompress,
				"+full":       full,
			} {
				if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
					t.Errorf("%s: output does not start with %%PDF- header (got %q)", label, pdf[:min(8, len(pdf))])
				}
				if _, err := reader.Parse(pdf); err != nil {
					t.Errorf("%s: reader.Parse failed: %v", label, err)
				}
			}
		})
	}
}

// TestOptimizeExampleFullNotLargerThanDefault pins the headline claim
// the example's printed table exists to demonstrate: every lossless
// toggle enabled (+full) must never produce a larger file than the
// traditional writer (default). A regression that made optimization
// pessimal for some fixture shape would invert this inequality.
func TestOptimizeExampleFullNotLargerThanDefault(t *testing.T) {
	fixtures := []fixture{
		{name: "text-heavy", build: textHeavy},
		{name: "many empty pages", build: manyPages},
		{name: "table-heavy", build: tableHeavy},
		{name: "imported text-heavy", build: importedTextHeavy},
	}

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			def, _, _, _, full, err := writeAll(f)
			if err != nil {
				t.Fatalf("writeAll: %v", err)
			}
			if len(full) > len(def) {
				t.Errorf("+full (%d bytes) is larger than default (%d bytes); optimization regressed for %q", len(full), len(def), f.name)
			}
		})
	}
}

// TestOptimizeExampleImportedFixtureRecompressionWins pins the
// specific claim in the example's doc comment: imported documents
// carry raw (uncompressed) content streams, so +recompress should
// shrink them relative to the default writer. This is the scenario
// where the optimizer's win is most visible, and a regression that
// silently disabled recompression for imported pages would slip past
// the more lenient "not larger" check in the sibling test.
func TestOptimizeExampleImportedFixtureRecompressionWins(t *testing.T) {
	def, _, _, recompress, _, err := writeAll(fixture{name: "imported text-heavy", build: importedTextHeavy})
	if err != nil {
		t.Fatalf("writeAll: %v", err)
	}
	if len(recompress) >= len(def) {
		t.Errorf("+recompress (%d bytes) is not smaller than default (%d bytes) for the imported fixture; expected Flate recompression to shrink raw content streams", len(recompress), len(def))
	}
}

// TestOptimizeExampleManyPagesFixtureHasFiftyPages sanity-checks the
// manyPages fixture builder itself: a regression that changed the
// loop bound or broke AddPage would silently alter the shape of the
// page-tree-heavy comparison row.
func TestOptimizeExampleManyPagesFixtureHasFiftyPages(t *testing.T) {
	pdf, err := manyPages().ToBytes()
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	r, err := reader.Parse(pdf)
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	if got := r.PageCount(); got != 50 {
		t.Errorf("PageCount = %d, want 50", got)
	}
}
