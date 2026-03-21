// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
)

var benchPDF []byte

func init() {
	doc := document.NewDocument(document.PageSizeLetter)
	doc.Info.Title = "Bench"
	p := doc.AddPage()
	p.AddText("Hello World benchmark text for testing PDF parsing performance", font.Helvetica, 12, 72, 700)
	p2 := doc.AddPage()
	p2.AddText("Second page content", font.Helvetica, 12, 72, 700)
	var buf bytes.Buffer
	_, _ = doc.WriteTo(&buf)
	benchPDF = buf.Bytes()
}

func BenchmarkParse(b *testing.B) {
	for range b.N {
		_, _ = Parse(benchPDF)
	}
}

func BenchmarkTokenizer(b *testing.B) {
	for range b.N {
		tok := NewTokenizer(benchPDF)
		for {
			t := tok.Next()
			if t.Type == TokenEOF {
				break
			}
		}
	}
}

func BenchmarkExtractText(b *testing.B) {
	r, _ := Parse(benchPDF)
	page, _ := r.Page(0)
	content, _ := page.ContentStream()
	b.ResetTimer()
	for range b.N {
		ExtractText(content)
	}
}

func BenchmarkMerge(b *testing.B) {
	r1, _ := Parse(benchPDF)
	r2, _ := Parse(benchPDF)
	b.ResetTimer()
	for range b.N {
		m, _ := Merge(r1, r2)
		var buf bytes.Buffer
		_, _ = m.WriteTo(&buf)
	}
}
