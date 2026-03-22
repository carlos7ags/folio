// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"io"
	"testing"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

func BenchmarkBlankPage(b *testing.B) {
	for range b.N {
		doc := NewDocument(PageSizeLetter)
		doc.AddPage()
		_, _ = doc.WriteTo(io.Discard)
	}
}

func BenchmarkSingleParagraph(b *testing.B) {
	for range b.N {
		doc := NewDocument(PageSizeLetter)
		doc.Add(layout.NewParagraph("Hello World", font.Helvetica, 12))
		_, _ = doc.WriteTo(io.Discard)
	}
}

func BenchmarkTable10x3(b *testing.B) {
	for range b.N {
		doc := NewDocument(PageSizeLetter)
		tbl := layout.NewTable()
		for range 10 {
			r := tbl.AddRow()
			r.AddCell("Column A", font.Helvetica, 10)
			r.AddCell("Column B", font.Helvetica, 10)
			r.AddCell("Column C", font.Helvetica, 10)
		}
		doc.Add(tbl)
		_, _ = doc.WriteTo(io.Discard)
	}
}

func BenchmarkMultiPage50(b *testing.B) {
	for range b.N {
		doc := NewDocument(PageSizeLetter)
		for range 50 {
			doc.Add(layout.NewParagraph(
				"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris.",
				font.Helvetica, 12,
			))
		}
		_, _ = doc.WriteTo(io.Discard)
	}
}

func BenchmarkLayoutParagraph(b *testing.B) {
	p := layout.NewParagraph(
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
		font.Helvetica, 12,
	)
	area := layout.LayoutArea{Width: 468, Height: 648}
	b.ResetTimer()
	for range b.N {
		p.PlanLayout(area)
	}
}

func BenchmarkLayoutTable(b *testing.B) {
	tbl := layout.NewTable().SetAutoColumnWidths()
	for range 20 {
		r := tbl.AddRow()
		r.AddCell("Product name here", font.Helvetica, 10)
		r.AddCell("A longer description of the product", font.Helvetica, 10)
		r.AddCell("$99.99", font.Helvetica, 10)
	}
	area := layout.LayoutArea{Width: 468, Height: 648}
	b.ResetTimer()
	for range b.N {
		tbl.PlanLayout(area)
	}
}
