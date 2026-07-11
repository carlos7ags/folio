// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"math"
	"sync"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

var examplePDFBytes = sync.OnceValue(func() []byte {
	var buf bytes.Buffer
	if _, err := buildDocument().WriteTo(&buf); err != nil {
		panic("WriteTo: " + err.Error())
	}
	return buf.Bytes()
})

func examplePDFReader(t *testing.T) *reader.PdfReader {
	t.Helper()
	r, err := reader.Parse(examplePDFBytes())
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	return r
}

// strokedLineWidths collects the LineWidth of every stroked line segment
// across every page, so assertions don't depend on exactly which page a
// table lands on. PathOps() emits one entry per path segment (the "moveto"
// that starts a line and the "lineto" that draws it are separate entries),
// so only PathLine segments are counted — otherwise every border line
// would be counted twice.
func strokedLineWidths(t *testing.T, r *reader.PdfReader) []float64 {
	t.Helper()
	var widths []float64
	for i := 0; i < r.PageCount(); i++ {
		page, err := r.Page(i)
		if err != nil {
			t.Fatalf("Page(%d): %v", i, err)
		}
		ops, err := page.PathOps()
		if err != nil {
			t.Fatalf("Page(%d).PathOps: %v", i, err)
		}
		for _, op := range ops {
			if op.Type != reader.PathLine {
				continue
			}
			if op.Painted == reader.PaintStroke || op.Painted == reader.PaintFillStroke {
				widths = append(widths, op.LineWidth)
			}
		}
	}
	return widths
}

func containsWidth(widths []float64, want float64) bool {
	for _, w := range widths {
		if math.Abs(w-want) < 1e-6 {
			return true
		}
	}
	return false
}

func TestBorderCollapseExampleProducesValidPDF(t *testing.T) {
	pdf := examplePDFBytes()
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Errorf("output does not start with %%PDF- header (got %q)", pdf[:min(8, len(pdf))])
	}
	if len(pdf) < 1000 {
		t.Errorf("output PDF is suspiciously small (%d bytes); expected >1 KB", len(pdf))
	}
}

// TestBorderCollapseExampleWiderWins pins the reduction's exact scenario
// (issue #378): the header's wider .25pt border must win over the body's
// explicit 0pt border on their shared edge.
func TestBorderCollapseExampleWiderWins(t *testing.T) {
	widths := strokedLineWidths(t, examplePDFReader(t))
	if !containsWidth(widths, 0.25) {
		t.Errorf("expected a stroked 0.25pt line (the header's winning border), got widths %v", widths)
	}
}

// TestBorderCollapseExampleHiddenSuppresses pins that a hidden border
// suppresses its shared edge even against a much wider (10pt) neighbor —
// no 10pt stroke should appear anywhere in the document.
func TestBorderCollapseExampleHiddenSuppresses(t *testing.T) {
	widths := strokedLineWidths(t, examplePDFReader(t))
	if containsWidth(widths, 10) {
		t.Errorf("expected no 10pt stroke (hidden should have suppressed it), got widths %v", widths)
	}
}

// TestBorderCollapseExampleStylePriorityAtEqualWidth pins that double
// outranks solid at equal width: exactly two 2pt strokes should be present
// for the shared edge (double draws two parallel lines; solid would draw
// only one).
func TestBorderCollapseExampleStylePriorityAtEqualWidth(t *testing.T) {
	widths := strokedLineWidths(t, examplePDFReader(t))
	count := 0
	for _, w := range widths {
		if math.Abs(w-2) < 1e-6 {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected exactly 2 strokes at 2pt width (double border wins), got %d; widths %v", count, widths)
	}
}
