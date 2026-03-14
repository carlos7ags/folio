// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"testing"
)

// FuzzTokenizer tests that the tokenizer never panics on arbitrary input.
func FuzzTokenizer(f *testing.F) {
	// Seed corpus with valid PDF fragments.
	f.Add([]byte("42 3.14 -7"))
	f.Add([]byte("(Hello World)"))
	f.Add([]byte("<48656C6C6F>"))
	f.Add([]byte("/Type /Pages"))
	f.Add([]byte("true false null"))
	f.Add([]byte("<< /Type /Catalog /Pages 2 0 R >>"))
	f.Add([]byte("[1 2 3 (hello) /Name]"))
	f.Add([]byte("% this is a comment\n42"))
	f.Add([]byte("1 0 obj\n<< /Type /Page >>\nendobj"))
	f.Add([]byte("(nested (parens) string)"))
	f.Add([]byte(`(escape \n \r \t \\ \( \))`))
	f.Add([]byte("<< /Length 0 >>\nstream\nendstream"))

	f.Fuzz(func(t *testing.T, data []byte) {
		tok := NewTokenizer(data)
		// Consume all tokens — must not panic.
		for i := 0; i < 10000; i++ {
			token := tok.Next()
			if token.Type == TokenEOF {
				break
			}
		}
	})
}

// FuzzParse tests that the parser never panics on arbitrary input.
func FuzzParse(f *testing.F) {
	f.Add([]byte("<< /Type /Catalog >>"))
	f.Add([]byte("[1 2 3]"))
	f.Add([]byte("5 0 R"))
	f.Add([]byte("1 0 obj\n42\nendobj"))
	f.Add([]byte("(Hello) /Name true null 3.14"))
	f.Add([]byte("<< /A << /B 1 >> >>"))
	f.Add([]byte("[<< /X 1 >> << /Y 2 >>]"))

	f.Fuzz(func(t *testing.T, data []byte) {
		tok := NewTokenizer(data)
		parser := NewParser(tok)
		// Try to parse — must not panic (errors are fine).
		parser.ParseObject()
	})
}

// FuzzParsePDF tests that Parse never panics on arbitrary input.
func FuzzParsePDF(f *testing.F) {
	// Minimal valid PDF.
	f.Add([]byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]>>endobj\nxref\n0 4\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \ntrailer<</Size 4/Root 1 0 R>>\nstartxref\n190\n%%EOF"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must not panic — errors are expected on random input.
		r, err := Parse(data)
		if err != nil {
			return
		}
		// If parse succeeds, basic operations must not panic.
		r.Version()
		r.PageCount()
		r.Info()
		if r.PageCount() > 0 {
			p, _ := r.Page(0)
			if p != nil {
				p.ContentStream()
				p.ExtractText()
			}
		}
	})
}
