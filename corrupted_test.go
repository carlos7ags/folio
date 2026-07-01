package folio

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/reader"
)

// https://github.com/klausnitzer/pentest-pdf-collection/tree/main/pdf_files
func TestCorrupted(t *testing.T) {
	for nm, u := range map[string]string{
		"poppler":     "https://gitlab.freedesktop.org/poppler/test/-/archive/master/test-master.zip?ref_type=heads&path=tests",
		"klausnitzer": "https://github.com/klausnitzer/pentest-pdf-collection/archive/refs/heads/main.zip",
	} {
		t.Run(nm, func(t *testing.T) {
			resp, err := http.Get(u)
			if err != nil {
				t.Fatal(err)
			}
			b, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				t.Fatal(err)
			}
			zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range zr.File {
				if !strings.HasSuffix(f.Name, ".pdf") {
					continue
				}
				t.Run(f.Name, func(t *testing.T) {
					t.Log(f.Name)
					rc, err := f.Open()
					if err != nil {
						t.Fatal(err)
					}
					b, err := io.ReadAll(rc)
					rc.Close()
					if err != nil {
						t.Fatal(err)
					}

					rdr, err := reader.Parse(b)
					if err != nil {
						t.Log(err)
						return
					}
					t.Log("count:", rdr.PageCount())
				})
			}
		})
	}
}
