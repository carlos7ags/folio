// Copyright 2026 Tamás Gulácsi and the Folio Authors
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
						b, err := io.ReadAll(rc)
						rc.Close()
						if err != nil {
							return err
						}
						if !yield(file{Data: b, Name: nm + "/" + f.Name}) {
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

func TestCorrupted(t *testing.T) {
	for f := range downloadPDFs(t) {
		rdr, err := Parse(f.Data)
		if err != nil {
			t.Log(f.Name, err)
			return
		}
		t.Log(f.Name, "count:", rdr.PageCount())
	}
}

func FuzzRealPDFTokenizer(f *testing.F) {
	for file := range downloadPDFs(f) {
		f.Add(file.Name, file.Data)
	}
	f.Fuzz(func(t *testing.T, nm string, data []byte) {
		tkz := NewTokenizer(data)
		var pos int
	Loop:
		for !tkz.AtEnd() {
			if token := tkz.Next(); token.Type == TokenEOF {
				break
			}
			if pos == tkz.pos {
				for range 3 {
					if token := tkz.Next(); token.Type == TokenEOF {
						break Loop
					} else if token.Pos != int64(pos) {
						break
					}
				}

				if pos == tkz.pos {
					t.Fatalf("%s: didn't advance from %d", nm, pos)
				}
			}
			pos = tkz.pos
		}
	})

}

func FuzzRealPDFParser(f *testing.F) {
	for file := range downloadPDFs(f) {
		f.Add(file.Name, file.Data)
	}
	f.Fuzz(func(t *testing.T, nm string, data []byte) {
		tok := NewTokenizer(data)
		parser := NewParser(tok)
		// Try to parse — must not panic (errors are fine).
		_, _ = parser.ParseObject()
	})
}
