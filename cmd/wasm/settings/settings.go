// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package settings parses the renderSettings JSON accepted by the wasm
// entrypoint (cmd/wasm). It is a separate, build-tag-free package so the
// parsing logic can be unit-tested on the host toolchain.
package settings

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
)

// Margins holds page margins in PDF points. It unmarshals from either a
// single JSON number (applied to all four sides) or an object with any of
// top/right/bottom/left.
type Margins struct {
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}

// UnmarshalJSON accepts 36 or {"top":36,"left":18,...}.
func (m *Margins) UnmarshalJSON(b []byte) error {
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		m.Top, m.Right, m.Bottom, m.Left = n, n, n, n
		return nil
	}
	type plain Margins // avoid recursion
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return fmt.Errorf("margins must be a number or an object with top/right/bottom/left: %w", err)
	}
	*m = Margins(p)
	return nil
}

// Settings is the renderSettings JSON accepted by folioRender.
type Settings struct {
	PageSize    string   `json:"pageSize"`    // named size, e.g. "a4", "letter" (default letter)
	Orientation string   `json:"orientation"` // "portrait" (default) or "landscape"
	Margins     *Margins `json:"margins,omitempty"`

	MediaType            string `json:"mediaType"`
	PdfProfile           string `json:"pdfProfile"`
	PdfTitle             string `json:"pdfTitle"`
	PdfAuthor            string `json:"pdfAuthor,omitempty"`
	PdfSubject           string `json:"pdfSubject,omitempty"`
	PdfKeywords          string `json:"pdfKeywords,omitempty"`
	IgnoreResourceErrors bool   `json:"ignoreResourceErrors"`
	CssDpi               int    `json:"cssDpi"`
	Watermark            string `json:"watermark,omitempty"`  // optional watermark text
	HeaderHTML           string `json:"headerHtml,omitempty"` // HTML for page header
	FooterHTML           string `json:"footerHtml,omitempty"` // HTML for page footer

	// Asset / font resolution (issue #426).
	AllowAbsolutePaths bool   `json:"allowAbsolutePaths"`         // permit absolute OS paths in the document (default false)
	BaseDir            string `json:"baseDir,omitempty"`          // directory used as BaseFS root for relative asset paths
	FallbackFontPath   string `json:"fallbackFontPath,omitempty"` // Unicode fallback font path (BaseFS-relative or absolute)
	FallbackFontData   string `json:"fallbackFontData,omitempty"` // base64-encoded TTF/OTF used as the Unicode fallback font
}

// Parse decodes the renderSettings JSON.
func Parse(jsonStr string) (Settings, error) {
	var s Settings
	if err := json.Unmarshal([]byte(jsonStr), &s); err != nil {
		return Settings{}, err
	}
	return s, nil
}

var pageSizes = map[string]document.PageSize{
	"a0":        document.PageSizeA0,
	"a1":        document.PageSizeA1,
	"a2":        document.PageSizeA2,
	"a3":        document.PageSizeA3,
	"a4":        document.PageSizeA4,
	"a5":        document.PageSizeA5,
	"a6":        document.PageSizeA6,
	"b4":        document.PageSizeB4,
	"b5":        document.PageSizeB5,
	"letter":    document.PageSizeLetter,
	"legal":     document.PageSizeLegal,
	"tabloid":   document.PageSizeTabloid,
	"ledger":    document.PageSizeLedger,
	"executive": document.PageSizeExecutive,
}

// ResolvePageSize maps the named page size (defaulting to US Letter) and
// applies the orientation, swapping dimensions for landscape.
func (s Settings) ResolvePageSize() document.PageSize {
	ps, ok := pageSizes[strings.ToLower(s.PageSize)]
	if !ok {
		ps = document.PageSizeLetter
	}
	switch strings.ToLower(s.Orientation) {
	case "landscape":
		if ps.Height > ps.Width {
			ps.Width, ps.Height = ps.Height, ps.Width
		}
	case "portrait":
		if ps.Width > ps.Height {
			ps.Width, ps.Height = ps.Height, ps.Width
		}
	}
	return ps
}

// fallbackFontName is the synthetic BaseFS path used when the fallback font
// is supplied inline via fallbackFontData.
const fallbackFontName = "__folio_fallback_font__"

// ApplyOptions wires the asset-resolution settings into html.Options:
// baseDir becomes BaseFS (via os.DirFS), allowAbsolutePaths is passed
// through, and the fallback font is set from fallbackFontData (base64,
// takes precedence) or fallbackFontPath. Defaults stay safe: with no
// settings, BaseFS is nil and absolute paths are denied.
func (s Settings) ApplyOptions(opts *html.Options) error {
	opts.AllowAbsolutePaths = s.AllowAbsolutePaths
	if s.BaseDir != "" {
		opts.BaseFS = os.DirFS(s.BaseDir)
	}
	if s.FallbackFontPath != "" {
		opts.FallbackFontPath = s.FallbackFontPath
	}
	if s.FallbackFontData != "" {
		data, err := base64.StdEncoding.DecodeString(s.FallbackFontData)
		if err != nil {
			return fmt.Errorf("invalid fallbackFontData: %w", err)
		}
		opts.BaseFS = overlayFS{
			extra: map[string][]byte{fallbackFontName: data},
			base:  opts.BaseFS,
		}
		opts.FallbackFontPath = fallbackFontName
	}
	return nil
}

// overlayFS serves a small in-memory file set, falling back to base (which
// may be nil) for everything else.
type overlayFS struct {
	extra map[string][]byte
	base  fs.FS
}

func (o overlayFS) Open(name string) (fs.File, error) {
	if data, ok := o.extra[name]; ok {
		return &memFile{name: name, r: bytes.NewReader(data), size: int64(len(data))}, nil
	}
	if o.base != nil {
		return o.base.Open(name)
	}
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

type memFile struct {
	name string
	r    *bytes.Reader
	size int64
}

func (f *memFile) Read(p []byte) (int, error) { return f.r.Read(p) }
func (f *memFile) Close() error               { return nil }
func (f *memFile) Stat() (fs.FileInfo, error) { return memInfo{name: f.name, size: f.size}, nil }

type memInfo struct {
	name string
	size int64
}

func (i memInfo) Name() string       { return i.name }
func (i memInfo) Size() int64        { return i.size }
func (i memInfo) Mode() fs.FileMode  { return 0o444 }
func (i memInfo) ModTime() time.Time { return time.Time{} }
func (i memInfo) IsDir() bool        { return false }
func (i memInfo) Sys() any           { return nil }
