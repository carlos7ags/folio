package folio

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

	"github.com/carlos7ags/folio/reader"
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

func downloadPDFs(t testing.TB) iter.Seq[[]byte] {
	cacheDir, _ := os.UserCacheDir()
	if cacheDir == "" {
		cacheDir = t.TempDir()
	} else {
		cacheDir = filepath.Join(cacheDir, "folio")
	}
	os.MkdirAll(cacheDir, 0775)

	return func(yield func([]byte) bool) {
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
						if !yield(b) {
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
	for b := range downloadPDFs(t) {
		rdr, err := reader.Parse(b)
		if err != nil {
			t.Log(err)
			return
		}
		t.Log("count:", rdr.PageCount())
	}
}

func FuzzReader(f *testing.F) {
	for b := range downloadPDFs(f) {
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		rdr, err := reader.Parse(b)
		if err != nil {
			t.Error(err)
		}
		t.Log("count:", rdr.PageCount())
	})

}
