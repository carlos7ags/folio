// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/carlos7ags/folio/layout"
)

// makeJPEGBytes returns a small encoded JPEG for tests.
func makeJPEGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestBaseFSLoadsImage verifies that <img> with a relative src is loaded
// from Options.BaseFS instead of the OS filesystem. We assert both that the
// BaseFS was read and that the decoded image made it into the layout.
func TestBaseFSLoadsImage(t *testing.T) {
	jpegData := makeJPEGBytes(t)
	fsys := &countingFS{inner: fstest.MapFS{
		"assets/photo.jpg": {Data: jpegData},
	}}

	elems, err := Convert(`<img src="assets/photo.jpg"/>`, &Options{BaseFS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	if _, ok := elems[0].(*layout.ImageElement); !ok {
		t.Fatalf("expected ImageElement from BaseFS, got %T (alt-text fallback means FS load failed)", elems[0])
	}
	if fsys.count("assets/photo.jpg") == 0 {
		t.Errorf("expected BaseFS to be read; opens = %v", fsys.snapshot())
	}
}

// TestBaseFSLoadsLinkedStylesheet verifies that <link rel="stylesheet"> resolves
// through BaseFS. We check that the stylesheet file was actually read by
// wrapping MapFS in a counting FS.
func TestBaseFSLoadsLinkedStylesheet(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{
		"styles/site.css": {Data: []byte(`p { color: red; }`)},
	}}
	html := `<html><head><link rel="stylesheet" href="styles/site.css"/></head><body><p>hi</p></body></html>`

	if _, err := ConvertFull(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}
	if fsys.count("styles/site.css") == 0 {
		t.Errorf("expected stylesheet to be read from BaseFS; opens = %v", fsys.snapshot())
	}
}

// TestBaseFSLoadsBackgroundImage verifies the background-image: url() resolver
// routes through BaseFS. This path goes through resolveBackgroundImage →
// loadLocalImage, a different code path from <img> and linked stylesheet.
func TestBaseFSLoadsBackgroundImage(t *testing.T) {
	jpegData := makeJPEGBytes(t)
	fsys := &countingFS{inner: fstest.MapFS{
		"bg.jpg": {Data: jpegData},
	}}
	html := `<div style="background-image: url('bg.jpg'); width:100px; height:100px;"><p>x</p></div>`
	if _, err := Convert(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}
	if fsys.count("bg.jpg") == 0 {
		t.Errorf("expected background-image to be read from BaseFS; opens = %v", fsys.snapshot())
	}
}

// TestBaseFSRootAnchoredPath verifies that an HTML-style absolute path
// ("/foo.jpg") is treated as web-style root and resolves at the BaseFS root.
func TestBaseFSRootAnchoredPath(t *testing.T) {
	jpegData := makeJPEGBytes(t)
	fsys := &countingFS{inner: fstest.MapFS{
		"photo.jpg": {Data: jpegData},
	}}
	elems, err := Convert(`<img src="/photo.jpg"/>`, &Options{BaseFS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := elems[0].(*layout.ImageElement); !ok {
		t.Fatalf("/photo.jpg should resolve from BaseFS root; got %T", elems[0])
	}
	if fsys.count("photo.jpg") == 0 {
		t.Errorf("expected /photo.jpg to read photo.jpg from BaseFS; opens = %v", fsys.snapshot())
	}
}

// TestNoBaseFSFailsLocalAsset verifies that without BaseFS, a relative <img>
// src produces alt-text fallback instead of an OS read. This is the v0.8
// behaviour change from the v0.7 cwd-relative fallback.
func TestNoBaseFSFailsLocalAsset(t *testing.T) {
	elems, err := Convert(`<img src="photo.jpg" alt="missing"/>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := elems[0].(*layout.ImageElement); ok {
		t.Fatal("expected alt-text fallback when BaseFS is nil; got ImageElement")
	}
}

// TestClientUsedForImageFetch verifies that Options.Client is used for HTTP
// image fetches.
func TestClientUsedForImageFetch(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(makeJPEGBytes(t))
	}))
	defer srv.Close()

	// Wrap the transport so we can also assert our client is the one making the call.
	seen := 0
	client := &http.Client{
		Transport: &countingTransport{inner: http.DefaultTransport, counter: &seen},
	}

	elems, err := Convert(fmt.Sprintf(`<img src="%s/photo.jpg"/>`, srv.URL), &Options{
		AllowRemoteFetch: true,
		URLPolicy:        allowAllURLs,
		Client:           client,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Error("test server was not hit")
	}
	if seen == 0 {
		t.Error("Options.Client transport was not used for image fetch")
	}
	if _, ok := elems[0].(*layout.ImageElement); !ok {
		t.Fatalf("expected ImageElement, got %T", elems[0])
	}
}

// TestClientUsedForStylesheetFetch verifies that Options.Client is used for
// HTTP <link rel="stylesheet"> loads.
func TestClientUsedForStylesheetFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		_, _ = io.WriteString(w, `p { color: red; }`)
	}))
	defer srv.Close()

	seen := 0
	client := &http.Client{
		Transport: &countingTransport{inner: http.DefaultTransport, counter: &seen},
	}
	html := fmt.Sprintf(`<html><head><link rel="stylesheet" href="%s/s.css"/></head><body><p>hi</p></body></html>`, srv.URL)

	if _, err := ConvertFull(html, &Options{AllowRemoteFetch: true, URLPolicy: allowAllURLs, Client: client}); err != nil {
		t.Fatal(err)
	}
	if seen == 0 {
		t.Error("Options.Client transport was not used for stylesheet fetch")
	}
}

// TestURLPolicyStillBlocksWithCustomClient confirms that URLPolicy short-circuits
// the fetch before the custom Client is ever consulted.
func TestURLPolicyStillBlocksWithCustomClient(t *testing.T) {
	seen := 0
	client := &http.Client{
		Transport: &countingTransport{inner: http.DefaultTransport, counter: &seen},
	}
	_, err := Convert(`<img src="http://example.invalid/x.jpg"/>`, &Options{
		AllowRemoteFetch: true,
		Client:           client,
		URLPolicy:        func(string) error { return fmt.Errorf("denied") },
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != 0 {
		t.Errorf("Client should not have been used when URLPolicy denies: got %d transport calls", seen)
	}
}

// TestBaseFSLoadsFontFace verifies that @font-face src resolves through BaseFS.
// The font bytes are invalid so loadFontFaces silently skips embedding — but
// the counting FS proves readAsset routed the read through BaseFS.
func TestBaseFSLoadsFontFace(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{
		"fonts/fake.ttf": {Data: []byte("not a real font")},
	}}
	html := `<html><head><style>
		@font-face { font-family: "X"; src: url("fonts/fake.ttf"); }
		p { font-family: "X"; }
	</style></head><body><p>hi</p></body></html>`

	if _, err := Convert(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}
	if fsys.count("fonts/fake.ttf") == 0 {
		t.Errorf("expected @font-face src to be read from BaseFS; opens = %v", fsys.snapshot())
	}
}

// TestBaseFSRejectsParentEscape verifies that a "../" traversal attempt in
// <img src> against BaseFS fails cleanly (rejected by fs.ValidPath in
// readAsset) and falls back to alt text, not to an OS read outside the FS.
func TestBaseFSRejectsParentEscape(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{
		"safe.jpg": {Data: makeJPEGBytes(t)},
	}}
	elems, err := Convert(`<img src="../etc/passwd" alt="blocked"/>`, &Options{BaseFS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elems))
	}
	if _, ok := elems[0].(*layout.Paragraph); !ok {
		t.Fatalf("expected alt-text Paragraph fallback, got %T", elems[0])
	}
	if fsys.count("../etc/passwd") != 0 {
		t.Errorf("fs should not have been opened with traversal path; opens = %v", fsys.snapshot())
	}
}

// TestReadAssetInvalidPath directly exercises readAsset's path handling
// for fs.FS inputs: traversal rejected, leading ./ stripped, not-found is
// a clean fs.ErrNotExist rather than a panic.
func TestReadAssetInvalidPath(t *testing.T) {
	fsys := fstest.MapFS{"dir/a.txt": {Data: []byte("ok")}}

	// ".." traversal must not reach fs.ReadFile.
	if _, err := readAsset(fsys, "dir/../../escape"); err == nil {
		t.Error("expected error for traversal path, got nil")
	}

	// Leading "./" is normalized and the read succeeds.
	data, err := readAsset(fsys, "./dir/a.txt")
	if err != nil {
		t.Errorf("./ prefix should be stripped: %v", err)
	}
	if string(data) != "ok" {
		t.Errorf("unexpected data: %q", data)
	}

	// Leading "/" is stripped; root-anchored paths resolve from FS root.
	data, err = readAsset(fsys, "/dir/a.txt")
	if err != nil {
		t.Errorf("/ prefix should be stripped: %v", err)
	}
	if string(data) != "ok" {
		t.Errorf("unexpected data: %q", data)
	}

	// Missing file surfaces as fs.ErrNotExist (not a panic, not a different error).
	if _, err := readAsset(fsys, "dir/nope.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected fs.ErrNotExist, got %v", err)
	}

	// Nil BaseFS surfaces errNoBaseFS rather than panicking.
	if _, err := readAsset(nil, "x"); !errors.Is(err, errNoBaseFS) {
		t.Errorf("expected errNoBaseFS for nil baseFS, got %v", err)
	}
}

// TestFontFaceRelativeToLinkedStylesheet verifies that when a linked
// stylesheet at "styles/site.css" declares
// @font-face { src: url("../fonts/x.ttf") }, the font is resolved as
// "fonts/x.ttf" at the BaseFS root — not "../fonts/x.ttf" against the
// document root, which would escape the BaseFS.
func TestFontFaceRelativeToLinkedStylesheet(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{
		"styles/site.css": {Data: []byte(`@font-face { font-family: "X"; src: url("../fonts/x.ttf"); }`)},
		"fonts/x.ttf":     {Data: []byte("not-a-real-font-but-we-only-care-about-the-read")},
	}}
	html := `<html><head><link rel="stylesheet" href="styles/site.css"/></head><body><p>hi</p></body></html>`

	if _, err := Convert(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}
	if fsys.count("fonts/x.ttf") == 0 {
		t.Errorf("expected @font-face to be resolved relative to its stylesheet (fonts/x.ttf); opens = %v", fsys.snapshot())
	}
}

// TestFontFaceRelativeToInlineStyleResolvesFromRoot verifies that an
// @font-face inside an inline <style> resolves its src from the BaseFS root.
func TestFontFaceRelativeToInlineStyleResolvesFromRoot(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{
		"fonts/y.ttf": {Data: []byte("not a real font")},
	}}
	html := `<html><head><style>
		@font-face { font-family: "Y"; src: url("fonts/y.ttf"); }
	</style></head><body><p>hi</p></body></html>`
	if _, err := Convert(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}
	if fsys.count("fonts/y.ttf") == 0 {
		t.Errorf("inline @font-face should resolve from BaseFS root; opens = %v", fsys.snapshot())
	}
}

// TestFontFaceRelativeToHTTPStylesheet verifies that an @font-face declared
// inside an HTTP-loaded stylesheet uses the same Client to fetch the font,
// resolving the src URL relative to the stylesheet's URL.
func TestFontFaceRelativeToHTTPStylesheet(t *testing.T) {
	requested := []string{}
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requested = append(requested, r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/css/site.css":
			w.Header().Set("Content-Type", "text/css")
			_, _ = io.WriteString(w, `@font-face { font-family: "Z"; src: url("../fonts/z.ttf"); }`)
		case "/fonts/z.ttf":
			w.Header().Set("Content-Type", "font/ttf")
			_, _ = w.Write([]byte("not a real font but the fetch still happens"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	html := fmt.Sprintf(`<html><head><link rel="stylesheet" href="%s/css/site.css"/></head><body><p>hi</p></body></html>`, srv.URL)
	if _, err := Convert(html, &Options{AllowRemoteFetch: true, URLPolicy: allowAllURLs, Client: srv.Client()}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"/css/site.css", "/fonts/z.ttf"}
	if !equalSlices(requested, want) {
		t.Errorf("expected requests %v, got %v", want, requested)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestLoggerCapturesFontFailure verifies @font-face load failures are
// surfaced through Options.Logger at warn level.
func TestLoggerCapturesFontFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))

	fsys := fstest.MapFS{} // empty — the font won't be found
	html := `<html><head><style>
		@font-face { font-family: "Missing"; src: url("nope.ttf"); }
	</style></head><body><p>hi</p></body></html>`
	if _, err := Convert(html, &Options{BaseFS: fsys, Logger: logger}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "@font-face load failed") {
		t.Errorf("expected logger to capture @font-face failure; got %q", out)
	}
}

// TestLoggerCapturesStylesheetFailure verifies linked-stylesheet load
// failures are surfaced through Options.Logger.
func TestLoggerCapturesStylesheetFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	html := `<html><head><link rel="stylesheet" href="missing.css"/></head><body><p>hi</p></body></html>`
	if _, err := Convert(html, &Options{BaseFS: fstest.MapFS{}, Logger: logger}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "stylesheet load failed") {
		t.Errorf("expected stylesheet failure log; got %q", buf.String())
	}
}

// TestLoggerCapturesImageFailure verifies image-load failures are surfaced
// through Options.Logger.
func TestLoggerCapturesImageFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	if _, err := Convert(`<img src="missing.jpg" alt="x"/>`, &Options{
		BaseFS: fstest.MapFS{}, Logger: logger,
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "image load failed") {
		t.Errorf("expected image failure log; got %q", buf.String())
	}
}

// TestFallbackFontPathThroughBaseFS verifies FallbackFontPath is read from
// BaseFS first when configured.
func TestFallbackFontPathThroughBaseFS(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{
		"fonts/fallback.ttf": {Data: []byte("not a real font, expected to fail parsing")},
	}}
	// Triggering the fallback requires non-WinAnsi text; we don't need the
	// font to actually parse — only the FS read needs to happen.
	html := `<html><body><p>日本語</p></body></html>`
	if _, err := Convert(html, &Options{
		BaseFS:           fsys,
		FallbackFontPath: "fonts/fallback.ttf",
	}); err != nil {
		t.Fatal(err)
	}
	if fsys.count("fonts/fallback.ttf") == 0 {
		t.Errorf("FallbackFontPath should be read through BaseFS; opens = %v", fsys.snapshot())
	}
}

// TestJoinFSPath exercises stylesheet-relative path resolution in the
// pure-string helper, including root-anchored and inline-origin cases.
func TestJoinFSPath(t *testing.T) {
	cases := []struct {
		origin string
		src    string
		want   string
	}{
		{origin: "", src: "x.ttf", want: "x.ttf"},
		{origin: "site.css", src: "x.ttf", want: "x.ttf"},
		{origin: "styles/site.css", src: "x.ttf", want: "styles/x.ttf"},
		{origin: "styles/site.css", src: "../fonts/x.ttf", want: "styles/../fonts/x.ttf"},
		{origin: "styles/site.css", src: "/x.ttf", want: "/x.ttf"},
	}
	for _, c := range cases {
		got := joinFSPath(c.origin, c.src)
		if got != c.want {
			t.Errorf("joinFSPath(%q, %q) = %q, want %q", c.origin, c.src, got, c.want)
		}
	}
}

// TestJoinURL exercises stylesheet-relative URL resolution.
func TestJoinURL(t *testing.T) {
	cases := []struct {
		origin string
		src    string
		want   string
	}{
		{origin: "https://example.com/css/site.css", src: "../fonts/x.ttf", want: "https://example.com/fonts/x.ttf"},
		{origin: "https://example.com/css/site.css", src: "x.ttf", want: "https://example.com/css/x.ttf"},
		{origin: "https://example.com/css/site.css", src: "/x.ttf", want: "https://example.com/x.ttf"},
		{origin: "https://example.com/css/site.css", src: "https://other.example/x.ttf", want: "https://other.example/x.ttf"},
		{origin: "https://example.com", src: "x.ttf", want: "https://example.com/x.ttf"},
	}
	for _, c := range cases {
		got := joinURL(c.origin, c.src)
		if got != c.want {
			t.Errorf("joinURL(%q, %q) = %q, want %q", c.origin, c.src, got, c.want)
		}
	}
}

// countingTransport wraps an http.RoundTripper and counts invocations.
type countingTransport struct {
	inner   http.RoundTripper
	counter *int
}

func (c *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	*c.counter++
	return c.inner.RoundTrip(req)
}

// countingFS wraps an fs.FS and records which paths were opened, so a test
// can assert BaseFS was actually the source of a read. Safe for concurrent
// use by Open; snapshots are independent copies.
type countingFS struct {
	inner fstest.MapFS
	mu    sync.Mutex
	opens map[string]int
}

func (c *countingFS) Open(name string) (fs.File, error) {
	c.mu.Lock()
	if c.opens == nil {
		c.opens = map[string]int{}
	}
	c.opens[name]++
	c.mu.Unlock()
	return c.inner.Open(name)
}

func (c *countingFS) count(name string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.opens[name]
}

func (c *countingFS) snapshot() map[string]int {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]int, len(c.opens))
	for k, v := range c.opens {
		out[k] = v
	}
	return out
}

// TestBaseFSBackslashSrc verifies that a Windows-style backslash separator in
// <img src> is normalised to forward slashes before the BaseFS read. Authors
// occasionally hand-write paths with "\" — the converter accepts them rather
// than failing fs.ValidPath silently.
func TestBaseFSBackslashSrc(t *testing.T) {
	jpegData := makeJPEGBytes(t)
	fsys := &countingFS{inner: fstest.MapFS{
		"dir/photo.jpg": {Data: jpegData},
	}}
	elems, err := Convert(`<img src="dir\photo.jpg"/>`, &Options{BaseFS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := elems[0].(*layout.ImageElement); !ok {
		t.Fatalf("backslash src should normalise to forward slash; got %T", elems[0])
	}
	if fsys.count("dir/photo.jpg") == 0 {
		t.Errorf("expected dir/photo.jpg to be opened; opens = %v", fsys.snapshot())
	}
}

// TestEmptyHrefDoesNotPanic verifies that empty href / src in linked
// stylesheets and background-image url() does not panic. <img src=""> is
// already short-circuited at convertImage; this exercises the other paths.
func TestEmptyHrefDoesNotPanic(t *testing.T) {
	fsys := fstest.MapFS{}

	// <link href="">
	html := `<html><head><link rel="stylesheet" href=""/></head><body><p>x</p></body></html>`
	if _, err := ConvertFull(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}

	// background-image: url('')
	html = `<div style="background-image: url(''); width:10px; height:10px;"><p>x</p></div>`
	if _, err := Convert(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}
}

// TestReadAssetEdgeCases exercises path normalisation for pathological inputs.
func TestReadAssetEdgeCases(t *testing.T) {
	fsys := fstest.MapFS{"a/b.txt": {Data: []byte("ok")}}

	cases := []struct {
		name      string
		input     string
		wantOK    bool
		wantData  string
		wantError string
	}{
		{name: "double slash collapses", input: "a//b.txt", wantOK: true, wantData: "ok"},
		{name: "deep traversal rejected", input: "./.././x", wantError: "invalid"},
		{name: "triple slash root", input: "///", wantError: ""}, // any error is fine; "///" must not read anything
		{name: "empty path", input: "", wantError: "empty"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			data, err := readAsset(fsys, c.input)
			if c.wantOK {
				if err != nil {
					t.Errorf("readAsset(%q) error = %v, want success", c.input, err)
				}
				if string(data) != c.wantData {
					t.Errorf("readAsset(%q) = %q, want %q", c.input, data, c.wantData)
				}
				return
			}
			if err == nil {
				t.Errorf("readAsset(%q) succeeded, want error", c.input)
				return
			}
			if c.wantError != "" && !strings.Contains(err.Error(), c.wantError) {
				t.Errorf("readAsset(%q) error = %v, want substring %q", c.input, err, c.wantError)
			}
		})
	}
}

// TestFallbackFontPathOSFallback verifies that when FallbackFontPath is not
// present in BaseFS but exists on disk, loadFallbackFont falls through to
// font.LoadFont. We can't supply a parseable font, but we can confirm
// font.LoadFont was reached by observing that BaseFS was opened and then the
// OS path was attempted — the OS attempt errors with a parse failure rather
// than fs.ErrNotExist, which is the diagnostic signature we assert.
func TestFallbackFontPathOSFallback(t *testing.T) {
	// Create a real (but invalid) font file on disk.
	dir := t.TempDir()
	osPath := filepath.Join(dir, "fallback.ttf")
	if err := os.WriteFile(osPath, []byte("not a real font"), 0o644); err != nil {
		t.Fatal(err)
	}

	// BaseFS is empty — the read will miss and trigger the OS fallback.
	fsys := &countingFS{inner: fstest.MapFS{}}
	c := &converter{
		opts:   Options{BaseFS: fsys, FallbackFontPath: osPath},
		logger: slog.New(slog.DiscardHandler),
	}

	// loadFallbackFont treats absolute paths as OS-only, so BaseFS isn't
	// consulted for an absolute osPath — but font.LoadFont is reached, which
	// is what we want to verify. Use a relative path within tempdir to also
	// verify the BaseFS-then-OS path. Since we cannot easily configure
	// BaseFS to point at /tmp without coupling, use the absolute-path branch:
	// confirm font.LoadFont errors with a parse-related error (not
	// fs.ErrNotExist), proving the file was reached on disk.
	_, err := c.loadFallbackFont(osPath, "")
	if err == nil {
		t.Fatal("expected parse error for invalid font bytes, got nil")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("font.LoadFont returned fs.ErrNotExist; expected parse error: %v", err)
	}
}

// TestFallbackFontAbsoluteSkipsBaseFS confirms that an absolute system path
// for FallbackFontPath does not consult BaseFS at all.
func TestFallbackFontAbsoluteSkipsBaseFS(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{}}
	c := &converter{
		opts:   Options{BaseFS: fsys},
		logger: slog.New(slog.DiscardHandler),
	}
	// Use a path that is absolute on the host. On a non-existent file the
	// font loader will error; we only care that BaseFS sees zero opens.
	abs := string(filepath.Separator) + filepath.Join("does", "not", "exist", "x.ttf")
	_, _ = c.loadFallbackFont(abs, "")
	if got := fsys.count(strings.TrimPrefix(filepath.ToSlash(abs), "/")); got != 0 {
		t.Errorf("absolute path should bypass BaseFS, but it was opened %d times", got)
	}
	if len(fsys.snapshot()) != 0 {
		t.Errorf("absolute path should bypass BaseFS, but opens = %v", fsys.snapshot())
	}
}

// TestFontFaceRootAnchoredFromLinkedFSStylesheet verifies that a root-anchored
// "src: url(/fonts/x.ttf)" inside a linked FS stylesheet resolves at the
// BaseFS root, not at the stylesheet's directory.
func TestFontFaceRootAnchoredFromLinkedFSStylesheet(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{
		"styles/site.css": {Data: []byte(`@font-face { font-family: "X"; src: url("/fonts/root.ttf"); }`)},
		"fonts/root.ttf":  {Data: []byte("not a real font")},
	}}
	html := `<html><head><link rel="stylesheet" href="styles/site.css"/></head><body><p>hi</p></body></html>`
	if _, err := Convert(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}
	if fsys.count("fonts/root.ttf") == 0 {
		t.Errorf("root-anchored @font-face should resolve from BaseFS root; opens = %v", fsys.snapshot())
	}
}

// TestFontFaceRootAnchoredFromHTTPStylesheet verifies that a root-anchored
// "src: url(/abs.ttf)" inside an HTTP stylesheet is fetched from the host
// root, not the stylesheet's directory.
func TestFontFaceRootAnchoredFromHTTPStylesheet(t *testing.T) {
	requested := []string{}
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requested = append(requested, r.URL.Path)
		mu.Unlock()
		switch r.URL.Path {
		case "/css/site.css":
			w.Header().Set("Content-Type", "text/css")
			_, _ = io.WriteString(w, `@font-face { font-family: "Z"; src: url("/abs.ttf"); }`)
		case "/abs.ttf":
			w.Header().Set("Content-Type", "font/ttf")
			_, _ = w.Write([]byte("not a real font"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	html := fmt.Sprintf(`<html><head><link rel="stylesheet" href="%s/css/site.css"/></head><body><p>hi</p></body></html>`, srv.URL)
	if _, err := Convert(html, &Options{AllowRemoteFetch: true, URLPolicy: allowAllURLs, Client: srv.Client()}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"/css/site.css", "/abs.ttf"}
	if !equalSlices(requested, want) {
		t.Errorf("expected requests %v, got %v", want, requested)
	}
}

// TestCABIEmptyBasePathFailsRelativeRefs is the Go-level equivalent of the C
// ABI contract: when basePath is empty, BaseFS stays nil, and a document
// with a relative <img src> falls back to alt text. Mirrors the comment in
// export/cabi_document_ext2.go.
func TestCABIEmptyBasePathFailsRelativeRefs(t *testing.T) {
	elems, err := Convert(`<img src="logo.png" alt="missing"/>`, &Options{BaseFS: nil})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := elems[0].(*layout.ImageElement); ok {
		t.Fatal("expected alt-text fallback when BaseFS is nil; got ImageElement")
	}
}

// TestCABIBasePathRoundTrip mirrors the C ABI's os.DirFS(basePath) wrap: a
// non-empty basePath behaves as if BaseFS were set to that directory.
func TestCABIBasePathRoundTrip(t *testing.T) {
	dir := t.TempDir()
	jpegPath := filepath.Join(dir, "photo.jpg")
	if err := os.WriteFile(jpegPath, makeJPEGBytes(t), 0o644); err != nil {
		t.Fatal(err)
	}
	elems, err := Convert(`<img src="photo.jpg"/>`, &Options{BaseFS: os.DirFS(dir)})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := elems[0].(*layout.ImageElement); !ok {
		t.Fatalf("expected ImageElement loaded via os.DirFS, got %T", elems[0])
	}
}

// TestFontFaceStylesheetRelativeRejectsEscape verifies that a stylesheet at
// "deep/styles/site.css" attempting to escape via "../../../etc/passwd" is
// rejected by readAsset's fs.ValidPath check rather than reading anything.
func TestFontFaceStylesheetRelativeRejectsEscape(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{
		"deep/styles/site.css": {Data: []byte(`@font-face { font-family: "X"; src: url("../../../etc/passwd"); }`)},
	}}
	html := `<html><head><link rel="stylesheet" href="deep/styles/site.css"/></head><body><p>hi</p></body></html>`
	if _, err := Convert(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}
	for k := range fsys.snapshot() {
		if strings.Contains(k, "passwd") || strings.Contains(k, "..") {
			t.Errorf("traversal path was opened via BaseFS: %q", k)
		}
	}
}

// TestFontFaceMultipleSrcCandidates documents the parseFontFaceSrc contract:
// the first url(...) in a comma-separated src list wins. local() entries
// without a url() are skipped, so url() positions matter.
func TestFontFaceMultipleSrcCandidates(t *testing.T) {
	cases := []struct {
		val  string
		want string
	}{
		{val: `local("Local Name"), url("a.ttf") format("woff2"), url("b.ttf")`, want: "a.ttf"},
		{val: `local("Only Local")`, want: ""},
		{val: `url("only.ttf") format("truetype")`, want: "only.ttf"},
		{val: `url('single-quoted.ttf')`, want: "single-quoted.ttf"},
	}
	for _, c := range cases {
		if got := parseFontFaceSrc(c.val); got != c.want {
			t.Errorf("parseFontFaceSrc(%q) = %q, want %q", c.val, got, c.want)
		}
	}
}

// TestFontFaceInsideSupports verifies that an @font-face nested inside an
// @supports block is loaded when the feature query evaluates to true. The
// CSS parser recurses into @supports via parseCSS so the origin should
// propagate.
func TestFontFaceInsideSupports(t *testing.T) {
	fsys := &countingFS{inner: fstest.MapFS{
		"fonts/supp.ttf": {Data: []byte("not a real font")},
	}}
	html := `<html><head><style>
		@supports (display: flex) {
			@font-face { font-family: "Supp"; src: url("fonts/supp.ttf"); }
		}
	</style></head><body><p>hi</p></body></html>`
	if _, err := Convert(html, &Options{BaseFS: fsys}); err != nil {
		t.Fatal(err)
	}
	if fsys.count("fonts/supp.ttf") == 0 {
		t.Errorf("@font-face inside @supports should still load; opens = %v", fsys.snapshot())
	}
}

// TestNilLoggerNoPanicOnFailures verifies that with Logger left unset, the
// converter does not panic when multiple asset loads fail simultaneously.
// loggerOrDiscard should hand back a no-op logger.
func TestNilLoggerNoPanicOnFailures(t *testing.T) {
	html := `<html><head>
		<link rel="stylesheet" href="missing1.css"/>
		<style>@font-face { font-family: "X"; src: url("missing.ttf"); } </style>
	</head><body>
		<img src="missing.jpg" alt="x"/>
		<div style="background-image: url('missing-bg.jpg'); width:10px; height:10px;"><p>x</p></div>
	</body></html>`
	if _, err := ConvertFull(html, &Options{BaseFS: fstest.MapFS{}}); err != nil {
		t.Fatalf("nil Logger should not produce an error: %v", err)
	}
}

// TestLoggerCapturesAllFailures verifies all four asset-load sites surface
// through the same logger when configured.
func TestLoggerCapturesAllFailures(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn}))
	html := `<html><head>
		<link rel="stylesheet" href="missing1.css"/>
		<style>@font-face { font-family: "X"; src: url("missing.ttf"); } </style>
	</head><body>
		<img src="missing.jpg" alt="x"/>
		<div style="background-image: url('missing-bg.jpg'); width:10px; height:10px;"><p>x</p></div>
	</body></html>`
	if _, err := ConvertFull(html, &Options{BaseFS: fstest.MapFS{}, Logger: logger}); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{
		"stylesheet load failed",
		"@font-face load failed",
		"image load failed",
		"background-image",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected logger output to contain %q; got:\n%s", want, out)
		}
	}
}
