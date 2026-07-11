// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"archive/zip"
	"errors"
	"io"
	"iter"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const maxPDFSize = 10 << 20

func testTokenizer(t testing.TB, name string, data []byte) {
	tok := NewTokenizer(data)
	// Consume all tokens — must not panic, and must advance.
	var pos int
	for {
		token := tok.Next()
		if token.Type == TokenEOF {
			break
		}
		if pos == tok.pos {
			if token = tok.Next(); token.Type == TokenEOF {
				break
			}
			if pos == tok.pos {
				t.Fatalf("%s didn't advance from %d", name, pos)
			}
		}
		pos = tok.pos
	}
}

func TestTokenizer(t *testing.T) {
	for file := range downloadPDFs(t) {
		t.Run(file.Name, func(t *testing.T) {
			testTokenizer(t, file.Name, file.Data)
		})
	}
}

// FuzzTokenizer tests that the tokenizer never panics on arbitrary input.
func FuzzTokenizer(f *testing.F) {
	// Seed corpus with valid PDF fragments.
	f.Add("numbers", []byte("42 3.14 -7"))
	f.Add("string", []byte("(Hello World)"))
	f.Add("word", []byte("<48656C6C6F>"))
	f.Add("/", []byte("/Type /Pages"))
	f.Add("bools", []byte("true false null"))
	f.Add("list", []byte("<< /Type /Catalog /Pages 2 0 R >>"))
	f.Add("arr", []byte("[1 2 3 (hello) /Name]"))
	f.Add("comment", []byte("% this is a comment\n42"))
	f.Add("obj", []byte("1 0 obj\n<< /Type /Page >>\nendobj"))
	f.Add("nested_parens", []byte("(nested (parens) string)"))
	f.Add("escape", []byte(`(escape \n \r \t \\ \( \))`))
	f.Add("stream", []byte("<< /Length 0 >>\nstream\nendstream"))

	for file := range downloadPDFs(f) {
		f.Add(file.Name, file.Data)
	}

	f.Fuzz(func(t *testing.T, name string, data []byte) {
		testTokenizer(t, name, data)
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

	for file := range downloadPDFs(f) {
		f.Add(file.Data)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		tok := NewTokenizer(data)
		parser := NewParser(tok)
		// Try to parse — must not panic (errors are fine).
		_, _ = parser.ParseObject()
	})
}

// FuzzParsePDF tests that Parse never panics on arbitrary input.
func FuzzParsePDF(f *testing.F) {
	// Minimal valid PDF.
	f.Add([]byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]>>endobj\nxref\n0 4\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \ntrailer<</Size 4/Root 1 0 R>>\nstartxref\n190\n%%EOF"))
	// Xref stream with a negative /W field width — regression seed.
	f.Add(buildPDFWithXrefStreamW("[-5 3 1]"))

	for file := range downloadPDFs(f) {
		f.Add(file.Data)
	}

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
				_, _ = p.ContentStream()
				_, _ = p.ExtractText()
			}
		}
	})
}

func downloadZIP(zipFn, url string) (*zip.ReadCloser, error) {
	zr, err := zip.OpenReader(zipFn)
	if err == nil && zr != nil {
		return zr, err
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	fh, err := os.Create(zipFn)
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(fh, resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if err = fh.Close(); err != nil {
		return nil, err
	}
	return zip.OpenReader(zipFn)
}

type file struct {
	Name string
	Data []byte
}

func downloadPDFs(t testing.TB) iter.Seq[file] {
	cacheDir, _ := os.UserCacheDir()
	if cacheDir == "" {
		cacheDir = t.TempDir()
	} else {
		cacheDir = filepath.Join(cacheDir, "folio")
	}
	os.MkdirAll(cacheDir, 0775)

	return func(yield func(file) bool) {
		errExit := errors.New("exit")
		for nm, url := range map[string]string{
			"poppler":     "https://gitlab.freedesktop.org/poppler/test/-/archive/master/test-master.zip?ref_type=heads&path=tests",
			"klausnitzer": "https://github.com/klausnitzer/pentest-pdf-collection/archive/refs/heads/main.zip",
			"pdfcpu":      "https://github.com/pdfcpu/pdfcpu/archive/refs/heads/master.zip",
		} {
			if err := func() error {
				zr, err := downloadZIP(filepath.Join(cacheDir, nm+".zip"), url)
				if err != nil {
					return err
				}
				defer zr.Close()
				for _, f := range zr.File {
					if !strings.HasSuffix(f.Name, ".pdf") {
						continue
					}
					if err := func() error {
						rc, err := f.Open()
						if err != nil {
							return err
						}
						var b []byte
						if maxPDFSize == 0 {
							b, err = io.ReadAll(rc)
						} else {
							b, err = io.ReadAll(io.LimitReader(rc, int64(maxPDFSize)+1))
							if len(b) > maxPDFSize {
								return nil
							}
						}
						rc.Close()
						if err != nil {
							return err
						}
						if !yield(file{Data: b, Name: nm + ":" + strings.ReplaceAll(f.Name, "/", "%2F")}) {
							return errExit
						}
						return nil
					}(); err != nil {
						if errors.Is(err, errExit) {
							return errExit
						}
						t.Error(err)
					}
				}
				return nil
			}(); err != nil {
				if errors.Is(err, errExit) {
					return
				}
				t.Error(err)
				continue
			}
		}
	}
}
