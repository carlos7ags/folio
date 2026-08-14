// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"encoding/base64"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/carlos7ags/folio/html"
)

func TestResolvePageSizeOrientation(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		width, height float64
	}{
		{"default letter", `{}`, 612, 792},
		{"a4 portrait", `{"pageSize":"a4","orientation":"portrait"}`, 595.28, 841.89},
		{"a4 landscape", `{"pageSize":"a4","orientation":"landscape"}`, 841.89, 595.28},
		{"letter landscape", `{"pageSize":"letter","orientation":"landscape"}`, 792, 612},
		{"ledger already landscape", `{"pageSize":"ledger","orientation":"landscape"}`, 1224, 792},
		{"ledger forced portrait", `{"pageSize":"ledger","orientation":"portrait"}`, 792, 1224},
		{"tabloid", `{"pageSize":"tabloid"}`, 792, 1224},
		{"case-insensitive", `{"pageSize":"A4","orientation":"Landscape"}`, 841.89, 595.28},
		{"unknown size falls back to letter", `{"pageSize":"bogus"}`, 612, 792},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := Parse(tt.json)
			if err != nil {
				t.Fatal(err)
			}
			ps := s.ResolvePageSize()
			if ps.Width != tt.width || ps.Height != tt.height {
				t.Errorf("got %gx%g, want %gx%g", ps.Width, ps.Height, tt.width, tt.height)
			}
		})
	}
}

func TestMarginsNumber(t *testing.T) {
	s, err := Parse(`{"margins":36}`)
	if err != nil {
		t.Fatal(err)
	}
	m := s.Margins
	if m == nil || m.Top != 36 || m.Right != 36 || m.Bottom != 36 || m.Left != 36 {
		t.Errorf("got %+v, want uniform 36", m)
	}
}

func TestMarginsObject(t *testing.T) {
	s, err := Parse(`{"margins":{"top":10,"right":20,"bottom":30,"left":40}}`)
	if err != nil {
		t.Fatal(err)
	}
	m := s.Margins
	if m == nil || m.Top != 10 || m.Right != 20 || m.Bottom != 30 || m.Left != 40 {
		t.Errorf("got %+v", m)
	}
}

func TestMarginsPartialObject(t *testing.T) {
	s, err := Parse(`{"margins":{"left":18}}`)
	if err != nil {
		t.Fatal(err)
	}
	m := s.Margins
	if m == nil || m.Left != 18 || m.Top != 0 {
		t.Errorf("got %+v", m)
	}
}

func TestMarginsOmitted(t *testing.T) {
	s, err := Parse(`{}`)
	if err != nil {
		t.Fatal(err)
	}
	if s.Margins != nil {
		t.Errorf("expected nil margins, got %+v", s.Margins)
	}
}

func TestMarginsInvalid(t *testing.T) {
	if _, err := Parse(`{"margins":"big"}`); err == nil {
		t.Error("expected error for string margins")
	}
}

func TestApplyOptionsDefaultsAreSafe(t *testing.T) {
	s, _ := Parse(`{}`)
	var opts html.Options
	if err := s.ApplyOptions(&opts); err != nil {
		t.Fatal(err)
	}
	if opts.BaseFS != nil || opts.AllowAbsolutePaths || opts.FallbackFontPath != "" {
		t.Errorf("defaults not safe: %+v", opts)
	}
}

func TestApplyOptionsBaseDirAndFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Parse(`{"baseDir":` + jsonStr(dir) + `,"allowAbsolutePaths":true,"fallbackFontPath":"fonts/f.ttf"}`)
	if err != nil {
		t.Fatal(err)
	}
	var opts html.Options
	if err := s.ApplyOptions(&opts); err != nil {
		t.Fatal(err)
	}
	if !opts.AllowAbsolutePaths {
		t.Error("allowAbsolutePaths not applied")
	}
	if opts.FallbackFontPath != "fonts/f.ttf" {
		t.Errorf("fallbackFontPath = %q", opts.FallbackFontPath)
	}
	data, err := fs.ReadFile(opts.BaseFS, "x.txt")
	if err != nil || string(data) != "hi" {
		t.Errorf("BaseFS read: %q, %v", data, err)
	}
}

func TestApplyOptionsFallbackFontData(t *testing.T) {
	fontBytes := []byte{0, 1, 0, 0, 0xde, 0xad}
	s := Settings{FallbackFontData: base64.StdEncoding.EncodeToString(fontBytes)}
	var opts html.Options
	if err := s.ApplyOptions(&opts); err != nil {
		t.Fatal(err)
	}
	if opts.FallbackFontPath != fallbackFontName {
		t.Errorf("FallbackFontPath = %q", opts.FallbackFontPath)
	}
	got, err := fs.ReadFile(opts.BaseFS, fallbackFontName)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(fontBytes) {
		t.Errorf("font bytes mismatch")
	}
	// Anything else is not found (no base FS).
	if _, err := opts.BaseFS.Open("other"); err == nil {
		t.Error("expected not-exist for other files")
	}
}

func TestApplyOptionsFontDataOverlaysBaseDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "img.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := Settings{
		BaseDir:          dir,
		FallbackFontData: base64.StdEncoding.EncodeToString([]byte("font")),
	}
	var opts html.Options
	if err := s.ApplyOptions(&opts); err != nil {
		t.Fatal(err)
	}
	if b, err := fs.ReadFile(opts.BaseFS, fallbackFontName); err != nil || string(b) != "font" {
		t.Errorf("overlay font: %q, %v", b, err)
	}
	if b, err := fs.ReadFile(opts.BaseFS, "img.png"); err != nil || string(b) != "png" {
		t.Errorf("base passthrough: %q, %v", b, err)
	}
}

func TestApplyOptionsBadFontData(t *testing.T) {
	s := Settings{FallbackFontData: "not base64!!!"}
	var opts html.Options
	if err := s.ApplyOptions(&opts); err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestParseMetadataFields(t *testing.T) {
	s, err := Parse(`{"pdfTitle":"T","pdfAuthor":"A","pdfSubject":"S","pdfKeywords":"k1,k2"}`)
	if err != nil {
		t.Fatal(err)
	}
	if s.PdfTitle != "T" || s.PdfAuthor != "A" || s.PdfSubject != "S" || s.PdfKeywords != "k1,k2" {
		t.Errorf("got %+v", s)
	}
}

// jsonStr encodes a Go string as a JSON string literal (handles backslashes
// in Windows-style temp paths).
func jsonStr(s string) string {
	b := []byte{'"'}
	for _, r := range s {
		switch r {
		case '"', '\\':
			b = append(b, '\\', byte(r))
		default:
			b = append(b, string(r)...)
		}
	}
	return string(append(b, '"'))
}
