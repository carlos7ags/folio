// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/reader"
)

const pagMargin = 72

func newPaginationDoc() *document.Document {
	d := document.NewDocument(document.PageSizeA4)
	d.SetMargins(layout.Margins{Top: pagMargin, Right: pagMargin, Bottom: pagMargin, Left: pagMargin})
	return d
}

func numberedWords(prefix string, n int) string {
	w := make([]string, n)
	for i := range w {
		w[i] = fmt.Sprintf("%s%d", prefix, i+1)
	}
	return strings.Join(w, " ")
}

// pageSpans returns the text spans of page i.
func pageSpans(t *testing.T, r *reader.PdfReader, i int) []reader.TextSpan {
	t.Helper()
	p, err := r.Page(i)
	if err != nil {
		t.Fatalf("Page(%d): %v", i, err)
	}
	spans, err := p.TextSpans()
	if err != nil {
		t.Fatalf("TextSpans(%d): %v", i, err)
	}
	return spans
}

// pageLines returns, for each page, the first three characters of each
// text line's text, top line first — enough to name the paragraph a line
// belongs to when every word starts with its paragraph id.
func pageLines(t *testing.T, r *reader.PdfReader) [][]string {
	t.Helper()
	pages := make([][]string, r.PageCount())
	for i := range pages {
		lines := map[float64]string{}
		for _, s := range pageSpans(t, r, i) {
			if len(s.Text) >= 3 {
				lines[math.Round(s.Y)] = s.Text[:3]
			}
		}
		ys := make([]float64, 0, len(lines))
		for y := range lines {
			ys = append(ys, y)
		}
		sort.Sort(sort.Reverse(sort.Float64Slice(ys)))
		for _, y := range ys {
			pages[i] = append(pages[i], lines[y])
		}
	}
	return pages
}

// TestIssue478FirstLineStaysAboveBottomMargin reproduces issue #478: a
// paragraph starting with less than a line's room left on the page must
// move to the next page instead of placing its first line inside the
// bottom margin.
func TestIssue478FirstLineStaysAboveBottomMargin(t *testing.T) {
	d := newPaginationDoc()
	n := 0
	for i := range 40 {
		var w []string
		for range 30 + (i*37)%90 {
			n++
			w = append(w, fmt.Sprintf("w%04d", n))
		}
		d.Add(layout.NewParagraph(strings.Join(w, " "), font.Helvetica, 10).SetSpaceAfter(6))
	}
	r := writeAndParse(t, d)
	for i := range r.PageCount() {
		low := math.Inf(1)
		for _, s := range pageSpans(t, r, i) {
			if strings.HasPrefix(s.Text, "w") {
				low = math.Min(low, s.Y)
			}
		}
		if low < pagMargin {
			t.Errorf("page %d: lowest baseline %.1f is inside the %dpt bottom margin", i+1, low, pagMargin)
		}
	}
}
