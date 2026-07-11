// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

// TestResolverMatrixAbsolutePathNoBaseFS pins the centralized contract's
// rule #3 (absolute path + BaseFS nil → OS read) across every
// document-supplied resolver. Before #229 only Options.FallbackFontPath
// honored this rule; the centralization extended it to <img>, <link>,
// @font-face, and background-image. Without this matrix test, a
// regression where (say) the @font-face branch drops back to errNoBaseFS
// would slip through because no per-resolver test exercises rule #3.
//
// The test writes the requested asset to a temp file at an absolute
// path, references it from the document with no BaseFS configured, and
// verifies the load succeeds (StrictAssets surfaces failures as
// errors so a regression manifests as Convert returning an error).
func TestResolverMatrixAbsolutePathNoBaseFS(t *testing.T) {
	dir := t.TempDir()
	jpegPath := filepath.Join(dir, "img.jpg")
	if err := os.WriteFile(jpegPath, makeJPEGBytes(t), 0o644); err != nil {
		t.Fatal(err)
	}
	cssPath := filepath.Join(dir, "site.css")
	if err := os.WriteFile(cssPath, []byte("/* empty */"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		html string
	}{
		{
			name: "img src",
			html: fmt.Sprintf(`<html><body><img src="%s"/></body></html>`, jpegPath),
		},
		{
			name: "link rel=stylesheet href",
			html: fmt.Sprintf(`<html><head><link rel="stylesheet" href="%s"/></head><body><p>x</p></body></html>`, cssPath),
		},
		{
			name: "background-image url",
			html: fmt.Sprintf(`<html><head><style>div { background-image: url('%s'); width: 10px; height: 10px; }</style></head><body><div>x</div></body></html>`, jpegPath),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Convert(tc.html, &Options{AllowAbsolutePaths: true, StrictAssets: true}); err != nil {
				t.Errorf("absolute path with BaseFS=nil should resolve via OS: %v", err)
			}
		})
	}

	// @font-face requires a valid font file to fully succeed; rather than
	// shipping a font fixture, exercise rule #3 indirectly with a missing
	// path and assert the error class. A regression where rule #3 doesn't
	// fire would surface as errNoBaseFS ("no BaseFS configured") instead
	// of an OS-style "no such file" error, since the code would fall to
	// the default BaseFS-relative branch.
	t.Run("@font-face url", func(t *testing.T) {
		missing := filepath.Join(dir, "does-not-exist.ttf")
		src := fmt.Sprintf(`<html><head><style>
			@font-face { font-family: 'X'; src: url('%s'); }
		</style></head><body><p>x</p></body></html>`, missing)
		_, err := Convert(src, &Options{AllowAbsolutePaths: true, StrictAssets: true})
		if err == nil {
			t.Fatal("expected error for missing absolute font path")
		}
		if strings.Contains(err.Error(), "no BaseFS configured") {
			t.Errorf("rule 3 (absolute + BaseFS nil → OS) was bypassed; error: %v", err)
		}
		if !strings.Contains(err.Error(), "no such file") && !strings.Contains(err.Error(), "cannot find") {
			t.Errorf("expected OS-style not-found error, got: %v", err)
		}
	})
}

// TestDocumentAbsolutePathUsesBaseFSWhenSet pins the trust-boundary
// distinction between document-supplied references and the
// FallbackFontPath programmatic carve-out. With BaseFS set, an
// `<img src="/foo.jpg">` in the document must read foo.jpg at the
// BaseFS root — NOT escape to the OS — even though FallbackFontPath
// would bypass BaseFS for an absolute path. Documented in
// loadFallbackFont's comment in html/converter.go.
func TestDocumentAbsolutePathUsesBaseFSWhenSet(t *testing.T) {
	jpegData := makeJPEGBytes(t)
	fsys := &countingFS{inner: fstest.MapFS{
		"foo.jpg": {Data: jpegData},
	}}

	src := `<html><body><img src="/foo.jpg"/></body></html>`
	if _, err := Convert(src, &Options{BaseFS: fsys, StrictAssets: true}); err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if got := fsys.count("foo.jpg"); got == 0 {
		t.Errorf("absolute /foo.jpg should resolve at BaseFS root; opens = %v", fsys.snapshot())
	}
}

// TestSVGImgSrcGoesThroughURLPolicy pins the new behavior the
// centralization introduced as a deliberate side effect: SVG referenced
// from <img> via http(s):// now flows through Options.URLPolicy
// uniformly with raster images. Before #229 the SVG branch in
// convertSVGImage called readAsset directly with no URL handling at
// all, so URLPolicy never saw SVG URLs.
func TestSVGImgSrcGoesThroughURLPolicy(t *testing.T) {
	srvHit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		srvHit = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"></svg>`))
	}))
	defer srv.Close()

	policyCalled := false
	deny := func(string) error {
		policyCalled = true
		return errors.New("blocked by policy")
	}

	src := fmt.Sprintf(`<html><body><img src="%s/icon.svg"/></body></html>`, srv.URL)
	if _, err := Convert(src, &Options{AllowRemoteFetch: true, URLPolicy: deny}); err != nil {
		t.Fatalf("Convert: %v", err)
	}

	if !policyCalled {
		t.Error("URLPolicy was not consulted for SVG <img> URL")
	}
	if srvHit {
		t.Error("URLPolicy returned an error but the network was still hit")
	}
}

// TestImageFormatDetectionPrefersExtensionOverContentType pins the
// acknowledged narrow behavior change in PR #239: the old fetchImage
// consulted Content-Type before falling back to URL extension; the new
// path uses URL extension + magic-byte sniffing. A server returning
// `Content-Type: image/png` for a URL that ends `.jpg` and contains
// JPEG bytes must now decode successfully (extension says JPEG, the
// JPEG decoder accepts the bytes) rather than silently routing to the
// PNG decoder and failing.
func TestImageFormatDetectionPrefersExtensionOverContentType(t *testing.T) {
	jpegBytes := makeJPEGBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png") // intentionally wrong
		_, _ = w.Write(jpegBytes)
	}))
	defer srv.Close()

	src := fmt.Sprintf(`<html><body><img src="%s/photo.jpg"/></body></html>`, srv.URL)
	_, err := Convert(src, &Options{AllowRemoteFetch: true, URLPolicy: allowAllURLs, StrictAssets: true})
	if err != nil {
		t.Errorf("image with mismatched Content-Type but valid extension+bytes should decode: %v", err)
	}
}

// TestStrictAssetsRemoteFontHTTPFailureEscalatesNotPolicyTagged is a
// follow-up safety check: an HTTP 500 must be reported as a load
// failure, not as a URLPolicy denial. The centralized fetchHTTPBytes
// only wraps with ErrURLPolicyDenied when the policy callback errors;
// network failures and HTTP non-200 responses must surface raw.
func TestStrictAssetsRemoteFontHTTPFailureEscalatesNotPolicyTagged(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	src := fmt.Sprintf(`<html><head><style>
		@font-face { font-family: 'X'; src: url('%s/font.ttf'); }
	</style></head><body><p>hi</p></body></html>`, srv.URL)
	_, err := Convert(src, &Options{AllowRemoteFetch: true, URLPolicy: allowAllURLs, StrictAssets: true})
	if err == nil {
		t.Fatal("expected error from HTTP 500")
	}
	if errors.Is(err, ErrURLPolicyDenied) {
		t.Errorf("HTTP 500 should not be tagged as URLPolicy denial: %v", err)
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("error should mention HTTP 500: %v", err)
	}
}
