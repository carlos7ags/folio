// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"fmt"
	"io"
	"runtime"
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// longTable builds a table of rows body rows that wraps across many pages:
// a repeated header row, a footer row, and per row two element cells holding
// wrapped paragraphs plus a plain text cell, the mix the HTML converter and
// hand-built reports produce.
func longTable(rows int) *layout.Table {
	const text = "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore."
	tbl := layout.NewTable()
	tbl.SetColumnWidths([]float64{200, 200, 112})
	h := tbl.AddHeaderRow()
	h.AddCell("Item", font.HelveticaBold, 10)
	h.AddCell("Description", font.HelveticaBold, 10)
	h.AddCell("Amount", font.HelveticaBold, 10)
	for i := range rows {
		r := tbl.AddRow()
		r.AddCellElement(layout.NewParagraph(fmt.Sprintf("Row %d. %s", i, text), font.Helvetica, 10))
		r.AddCellElement(layout.NewParagraph(text, font.Helvetica, 10))
		r.AddCell(fmt.Sprintf("%d.00", i), font.Helvetica, 10)
	}
	f := tbl.AddFooterRow()
	f.AddCell("Total", font.HelveticaBold, 10)
	f.AddCell("", font.Helvetica, 10)
	f.AddCell("-", font.HelveticaBold, 10)
	return tbl
}

func writeLongTable(rows int) error {
	doc := NewDocument(PageSizeLetter)
	doc.Add(longTable(rows))
	_, err := doc.WriteTo(io.Discard)
	return err
}

// BenchmarkLongTable lays out and writes one table spanning many pages, at n
// and 2n rows. Layout cost should grow linearly with the row count: doubling
// the rows should about double the allocated bytes.
func BenchmarkLongTable(b *testing.B) {
	for _, n := range []int{200, 400, 800} {
		b.Run(fmt.Sprintf("rows=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				if err := writeLongTable(n); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestLongTableAllocScaling guards against quadratic table pagination: each
// page used to re-measure every remaining row, and each drawn row retained
// the grid of all of them, so doubling the rows of a multi-page table
// roughly tripled the bytes allocated. Linear layout keeps the ratio near 2.
func TestLongTableAllocScaling(t *testing.T) {
	if testing.Short() {
		t.Skip("allocation measurement")
	}
	const n = 120
	allocated := func(rows int) uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		if err := writeLongTable(rows); err != nil {
			t.Fatal(err)
		}
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}
	allocated(n) // warm font and encoder caches
	small, large := allocated(n), allocated(2*n)
	ratio := float64(large) / float64(small)
	t.Logf("%d rows: %d bytes, %d rows: %d bytes, ratio %.2f", n, small, 2*n, large, ratio)
	if ratio > 2.5 {
		t.Errorf("doubling the rows multiplied allocated bytes by %.2f, want at most 2.5", ratio)
	}
}
