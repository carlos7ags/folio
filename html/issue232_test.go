// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"bytes"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"

	"github.com/carlos7ags/folio/layout"
)

// --- @font-face escalation ---

// TestStrictAssetsBadFontFaceReturnsError verifies that a missing @font-face
// asset is returned as an error from Convert when StrictAssets is true.
// Asserts on both the error message (for human-readable grep) and the
// underlying cause (for errors.Is callers).
func TestStrictAssetsBadFontFaceReturnsError(t *testing.T) {
	src := `<html><head><style>
		@font-face { font-family: 'X'; src: url('missing/font.ttf'); }
		p { font-family: 'X'; }
	</style></head><body><p>hello</p></body></html>`

	fsys := fstest.MapFS{}
	_, err := Convert(src, &Options{BaseFS: fsys, StrictAssets: true})
	if err == nil {
		t.Fatal("expected error from StrictAssets, got nil")
	}
	if !strings.Contains(err.Error(), "@font-face") {
		t.Errorf("error should reference @font-face: %v", err)
	}
	if !strings.Contains(err.Error(), "missing/font.ttf") {
		t.Errorf("error should reference the failing src: %v", err)
	}
	// fstest.MapFS returns fs.ErrNotExist for absent paths; that cause must
	// survive errors.Join + fmt.Errorf %w wrapping.
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected errors.Is(err, fs.ErrNotExist), got: %v", err)
	}
}

// TestStrictAssetsDefaultFalsePreservesOldBehavior locks the no-break
// guarantee for @font-face: a missing url() does not produce an error
// when StrictAssets is unset (zero value).
func TestStrictAssetsDefaultFalsePreservesOldBehavior(t *testing.T) {
	src := `<html><head><style>
		@font-face { font-family: 'X'; src: url('missing/font.ttf'); }
		p { font-family: 'X'; }
	</style></head><body><p>hello</p></body></html>`

	fsys := fstest.MapFS{}
	elems, err := Convert(src, &Options{BaseFS: fsys})
	if err != nil {
		t.Errorf("non-strict mode should not return error: %v", err)
	}
	if len(elems) == 0 {
		t.Error("expected paragraph element in partial result")
	}
}

// --- Stylesheet escalation (pre-converter path) ---

// TestStrictAssetsBadStylesheetReturnsError verifies stylesheet load
// failures are escalated. parseStyleBlocks runs before the converter is
// constructed, so this exercises the separate stylesheet-error path that
// collects into a local slice and is attached to the converter at
// construction time.
func TestStrictAssetsBadStylesheetReturnsError(t *testing.T) {
	src := `<html><head><link rel="stylesheet" href="missing.css"/></head><body><p>hi</p></body></html>`
	fsys := fstest.MapFS{}
	_, err := Convert(src, &Options{BaseFS: fsys, StrictAssets: true})
	if err == nil {
		t.Fatal("expected error from StrictAssets, got nil")
	}
	if !strings.Contains(err.Error(), "stylesheet") {
		t.Errorf("error should reference stylesheet: %v", err)
	}
	if !strings.Contains(err.Error(), "missing.css") {
		t.Errorf("error should reference the failing href: %v", err)
	}
}

// TestStrictAssetsDefaultFalseStylesheetPath mirrors the @font-face
// default-false test for the stylesheet path. Without this, a regression
// that always escalates stylesheet errors regardless of StrictAssets
// would ship green.
func TestStrictAssetsDefaultFalseStylesheetPath(t *testing.T) {
	src := `<html><head><link rel="stylesheet" href="missing.css"/></head><body><p>hi</p></body></html>`
	fsys := fstest.MapFS{}
	if _, err := Convert(src, &Options{BaseFS: fsys}); err != nil {
		t.Errorf("non-strict mode should not return stylesheet error: %v", err)
	}
}

// --- <img> escalation ---

// TestStrictAssetsBadImageReturnsError verifies <img src> load failures
// are escalated.
func TestStrictAssetsBadImageReturnsError(t *testing.T) {
	src := `<html><body><img src="missing.png" alt="alt text"/></body></html>`
	fsys := fstest.MapFS{}
	_, err := Convert(src, &Options{BaseFS: fsys, StrictAssets: true})
	if err == nil {
		t.Fatal("expected error from StrictAssets, got nil")
	}
	if !strings.Contains(err.Error(), "image") {
		t.Errorf("error should reference image: %v", err)
	}
	if !strings.Contains(err.Error(), "missing.png") {
		t.Errorf("error should reference the failing src: %v", err)
	}
}

// --- SVG image escalation ---

// TestStrictAssetsBadSVGImageEscalates covers the converter_image.go
// SVG-image branch that was converted to reportAssetError but otherwise
// uncovered by tests.
func TestStrictAssetsBadSVGImageEscalates(t *testing.T) {
	src := `<html><body><img src="missing.svg" alt="alt text"/></body></html>`
	fsys := fstest.MapFS{}
	_, err := Convert(src, &Options{BaseFS: fsys, StrictAssets: true})
	if err == nil {
		t.Fatal("expected error from StrictAssets, got nil")
	}
	if !strings.Contains(err.Error(), "missing.svg") {
		t.Errorf("error should reference the failing src: %v", err)
	}
}

// --- background-image escalation ---

// TestStrictAssetsBadBackgroundImageEscalates covers both helper sites
// in converter_helpers.go (fetch and load branches).
func TestStrictAssetsBadBackgroundImageEscalates(t *testing.T) {
	src := `<html><head><style>
		div { background-image: url('missing-bg.png'); width: 100px; height: 100px; }
	</style></head><body><div>x</div></body></html>`
	fsys := fstest.MapFS{}
	_, err := Convert(src, &Options{BaseFS: fsys, StrictAssets: true})
	if err == nil {
		t.Fatal("expected error from StrictAssets, got nil")
	}
	if !strings.Contains(err.Error(), "background-image") {
		t.Errorf("error should reference background-image: %v", err)
	}
	if !strings.Contains(err.Error(), "missing-bg.png") {
		t.Errorf("error should reference the failing src: %v", err)
	}
}

// --- FallbackFontPath escalation ---

// TestStrictAssetsFallbackFontPathFailureEscalates covers the
// FallbackFontPath site in getFallbackFont. Includes a non-WinAnsi
// character in the text so the fallback path is actually exercised.
func TestStrictAssetsFallbackFontPathFailureEscalates(t *testing.T) {
	src := `<html><body><p>hello 中</p></body></html>`
	fsys := fstest.MapFS{}
	_, err := Convert(src, &Options{
		BaseFS:           fsys,
		FallbackFontPath: "missing/fallback.ttf",
		StrictAssets:     true,
	})
	if err == nil {
		t.Fatal("expected FallbackFontPath load to escalate")
	}
	if !strings.Contains(err.Error(), "FallbackFontPath") {
		t.Errorf("error should reference FallbackFontPath: %v", err)
	}
}

// --- Remote HTTP failure ---

// TestStrictAssetsRemoteFontHTTPFailureEscalates verifies that an HTTP
// 5xx response (not a URLPolicy denial) escalates under StrictAssets.
// Distinct from URLPolicy denial; this path is genuine fetch failure.
func TestStrictAssetsRemoteFontHTTPFailureEscalates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	src := `<html><head><style>
		@font-face { font-family: 'X'; src: url('` + srv.URL + `/font.ttf'); }
	</style></head><body><p>hi</p></body></html>`
	_, err := Convert(src, &Options{AllowRemoteFetch: true, URLPolicy: allowAllURLs, StrictAssets: true})
	if err == nil {
		t.Fatal("expected HTTP 500 to escalate")
	}
	if !strings.Contains(err.Error(), "HTTP 500") {
		t.Errorf("error should contain status code: %v", err)
	}
	if errors.Is(err, ErrURLPolicyDenied) {
		t.Errorf("HTTP failure should not be tagged as URLPolicy denial: %v", err)
	}
}

// --- URLPolicy denial NOT escalated ---

// TestStrictAssetsURLPolicyDenialNotEscalated commits to the field-doc
// contract: a URLPolicy denial is wrapped with ErrURLPolicyDenied and
// excluded from the joined return error. The warn-log still fires so
// observability is unchanged. This is a hard pin, not a t.Skip — the
// claim is part of the API.
func TestStrictAssetsURLPolicyDenialNotEscalated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not a font but the policy blocks first"))
	}))
	defer srv.Close()

	denyAll := func(string) error { return errors.New("blocked by policy") }
	src := `<html><head><style>
		@font-face { font-family: 'X'; src: url('` + srv.URL + `/font.ttf'); }
	</style></head><body><p>hi</p></body></html>`

	_, err := Convert(src, &Options{
		AllowRemoteFetch: true,
		StrictAssets:     true,
		URLPolicy:        denyAll,
	})
	if err != nil {
		t.Fatalf("URLPolicy denial must not escalate under StrictAssets: %v", err)
	}
}

// --- Multiple failures join ---

// TestStrictAssetsCollectsMultipleFailures verifies that a document with
// multiple broken assets produces a joined error covering each failure.
// Asserts via errors.Join's Unwrap() []error semantics that each failure
// is a separate child, not a single concatenated string.
func TestStrictAssetsCollectsMultipleFailures(t *testing.T) {
	src := `<html><head><style>
		@font-face { font-family: 'A'; src: url('a.ttf'); }
		@font-face { font-family: 'B'; src: url('b.ttf'); }
	</style></head><body>
		<img src="bad.png"/>
	</body></html>`
	fsys := fstest.MapFS{}
	_, err := Convert(src, &Options{BaseFS: fsys, StrictAssets: true})
	if err == nil {
		t.Fatal("expected joined error, got nil")
	}

	type unwrapper interface{ Unwrap() []error }
	u, ok := err.(unwrapper)
	if !ok {
		t.Fatalf("err must implement Unwrap() []error (errors.Join), got %T", err)
	}
	children := u.Unwrap()
	if len(children) != 3 {
		t.Errorf("expected 3 child errors, got %d: %v", len(children), children)
	}

	for _, want := range []string{"a.ttf", "b.ttf", "bad.png"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("joined error missing %q: %v", want, err)
		}
	}
}

// --- Determinism ---

// TestStrictAssetsErrorOrderingDeterministic asserts that the joined
// error is byte-identical across runs given byte-identical input — a
// guarantee documented in the StrictAssets field doc.
func TestStrictAssetsErrorOrderingDeterministic(t *testing.T) {
	src := `<html><head>
		<link rel="stylesheet" href="missing-1.css"/>
		<style>
			@font-face { font-family: 'A'; src: url('font-a.ttf'); }
			@font-face { font-family: 'B'; src: url('font-b.ttf'); }
		</style>
	</head><body>
		<img src="img-1.png"/>
		<img src="img-2.png"/>
	</body></html>`
	fsys := fstest.MapFS{}

	var first, second string
	for i, dst := range []*string{&first, &second} {
		_, err := Convert(src, &Options{BaseFS: fsys, StrictAssets: true})
		if err == nil {
			t.Fatalf("run %d: expected error", i)
		}
		*dst = err.Error()
	}
	if first != second {
		t.Errorf("error message not deterministic across runs:\nrun 1: %s\nrun 2: %s", first, second)
	}
}

// --- Logging coexistence ---

// TestStrictAssetsStillLogsWhenStrict verifies the design contract: when
// StrictAssets is true, failures are BOTH logged via Logger AND returned
// as an error. The previous warn channel must not be silenced.
func TestStrictAssetsStillLogsWhenStrict(t *testing.T) {
	var buf bytes.Buffer
	var mu sync.Mutex
	handler := slog.NewTextHandler(syncWriter{w: &buf, mu: &mu}, &slog.HandlerOptions{Level: slog.LevelWarn})

	src := `<html><head><style>
		@font-face { font-family: 'X'; src: url('missing.ttf'); }
	</style></head><body><p>hi</p></body></html>`
	fsys := fstest.MapFS{}

	_, err := Convert(src, &Options{
		BaseFS:       fsys,
		StrictAssets: true,
		Logger:       slog.New(handler),
	})
	if err == nil {
		t.Fatal("expected error")
	}
	logs := buf.String()
	if !strings.Contains(logs, "@font-face load failed") {
		t.Errorf("expected warn log alongside returned error; logs: %q", logs)
	}
}

type syncWriter struct {
	w  *bytes.Buffer
	mu *sync.Mutex
}

func (s syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}

// --- Partial result inspection ---

// TestStrictAssetsReturnsPartialResultAlongsideError documents that
// Convert returns whatever elements it managed to build alongside the
// error, with enough fidelity for callers to render a degraded preview.
// Asserts on the actual paragraph text, not just len(elems) > 0.
func TestStrictAssetsReturnsPartialResultAlongsideError(t *testing.T) {
	src := `<html><head><style>
		@font-face { font-family: 'X'; src: url('missing.ttf'); }
	</style></head><body>
		<p>this paragraph is fine</p>
	</body></html>`
	fsys := fstest.MapFS{}
	elems, err := Convert(src, &Options{BaseFS: fsys, StrictAssets: true})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(elems) == 0 {
		t.Fatal("expected partial elements alongside error, got empty slice")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *layout.Paragraph, got %T", elems[0])
	}
	lines := p.Layout(500)
	if len(lines) == 0 || len(lines[0].Words) == 0 {
		t.Fatal("paragraph rendered no words in partial result")
	}
	var got strings.Builder
	for _, w := range lines[0].Words {
		got.WriteString(w.Text)
		got.WriteString(" ")
	}
	if !strings.Contains(got.String(), "this paragraph is fine") {
		t.Errorf("partial paragraph text mismatch: %q", got.String())
	}
}

// --- ConvertFull parity ---

// TestStrictAssetsConvertFullSurfacesErrors verifies the same behavior
// holds for ConvertFull (the full-result variant).
func TestStrictAssetsConvertFullSurfacesErrors(t *testing.T) {
	src := `<html><head><style>
		@font-face { font-family: 'X'; src: url('missing.ttf'); }
	</style></head><body><p>hi</p></body></html>`
	fsys := fstest.MapFS{}
	result, err := ConvertFull(src, &Options{BaseFS: fsys, StrictAssets: true})
	if err == nil {
		t.Fatal("expected error from ConvertFull, got nil")
	}
	if result == nil {
		t.Fatal("ConvertFull should return partial result alongside error")
	}
	if len(result.Elements) == 0 {
		t.Error("ConvertFull partial result has no elements")
	}
}

// --- formatAssetError unit tests ---

// TestFormatAssetErrorPercentInValueIsSafe pins the fix for the Q1
// blocker raised in code review: a path containing % must not be
// interpreted as a format verb when the message is later printed or
// re-wrapped. Without the fix, the resulting error message would be
// garbled (%!b(MISSING) etc) and the trailing %w would mis-position.
func TestFormatAssetErrorPercentInValueIsSafe(t *testing.T) {
	cause := errors.New("file not found")
	err := formatAssetError("@font-face", cause, []any{"path", `C:\Users\foo%bar.ttf`})

	msg := err.Error()
	if !strings.Contains(msg, `C:\Users\foo%bar.ttf`) {
		t.Errorf("path with %% mangled: %q", msg)
	}
	if strings.Contains(msg, "MISSING") || strings.Contains(msg, "BADWIDTH") {
		t.Errorf("format verb leak: %q", msg)
	}
	if !errors.Is(err, cause) {
		t.Errorf("errors.Is must still find the cause despite %% in attr value")
	}
}

// TestFormatAssetErrorOddAttrs pins the chosen behavior for an unpaired
// trailing attr (a programmer error in our own callers): record the
// orphan key with a !BADKEY placeholder, slog-style.
func TestFormatAssetErrorOddAttrs(t *testing.T) {
	cause := errors.New("boom")
	err := formatAssetError("image", cause, []any{"src", "x.png", "trailing"})
	msg := err.Error()
	if !strings.Contains(msg, "trailing=!BADKEY") {
		t.Errorf("expected !BADKEY placeholder for unpaired attr, got: %q", msg)
	}
}

// TestFormatAssetErrorEmptyAttrs verifies the no-attrs path produces a
// clean message without trailing whitespace or stray separators.
func TestFormatAssetErrorEmptyAttrs(t *testing.T) {
	cause := errors.New("boom")
	err := formatAssetError("stylesheet", cause, nil)
	got := err.Error()
	want := "folio/html: stylesheet load failed: boom"
	if got != want {
		t.Errorf("formatAssetError empty attrs:\n  got:  %q\n  want: %q", got, want)
	}
}

// --- Errors.Is unwrap correctness ---

// TestStrictAssetsErrorsUnwrapToFsErrNotExist verifies that joined
// errors preserve errors.Is on the underlying fs.ErrNotExist that
// fstest.MapFS produces. This catches a regression where the %w chain
// is broken (e.g. someone replaces fmt.Errorf with concat).
func TestStrictAssetsErrorsUnwrapToFsErrNotExist(t *testing.T) {
	src := `<html><head><style>
		@font-face { font-family: 'X'; src: url('missing.ttf'); }
	</style></head><body><p>hi</p></body></html>`
	fsys := fstest.MapFS{}
	_, err := Convert(src, &Options{BaseFS: fsys, StrictAssets: true})
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("expected errors.Is(err, fs.ErrNotExist), got %v", err)
	}
}
