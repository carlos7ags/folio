// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"

	"github.com/carlos7ags/folio/layout"
)

// allowAllURLs is a URLPolicy that permits every host. Tests that hit a
// loopback httptest server need it because the built-in DenyInternalHosts
// default blocks 127.0.0.1.
func allowAllURLs(string) error { return nil }

// makePNGBytes returns a small encoded PNG for tests (no binary fixture).
func makePNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, color.RGBA{R: 10, G: 120, B: 220, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// errRoundTripper always errors without touching the network, and counts
// its invocations so a test can prove no request was attempted.
type errRoundTripper struct{ calls *int }

func (rt errRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	*rt.calls++
	return nil, fmt.Errorf("transport should not be reached")
}

// capturingHandler records the "error" attr value of each log record so a
// test can errors.Is against it (asset denials are logged, not escalated
// under StrictAssets).
type capturingHandler struct {
	mu   sync.Mutex
	errs []error
}

func (h *capturingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *capturingHandler) Handle(_ context.Context, r slog.Record) error {
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "error" {
			if err, ok := a.Value.Any().(error); ok {
				h.mu.Lock()
				h.errs = append(h.errs, err)
				h.mu.Unlock()
			}
		}
		return true
	})
	return nil
}

func (h *capturingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *capturingHandler) WithGroup(string) slog.Handler      { return h }

func (h *capturingHandler) has(target error) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, e := range h.errs {
		if errors.Is(e, target) {
			return true
		}
	}
	return false
}

// TestDenyInternalHosts pins the default policy's allow/deny decisions
// using IP literals only, so the table is deterministic (no DNS).
func TestDenyInternalHosts(t *testing.T) {
	cases := []struct {
		url  string
		deny bool
	}{
		{"http://93.184.216.34/x", false},                 // public IPv4 literal
		{"http://127.0.0.1/x", true},                      // loopback
		{"http://10.0.0.1/x", true},                       // RFC1918
		{"http://192.168.1.1/x", true},                    // RFC1918
		{"http://169.254.169.254/latest/meta-data", true}, // link-local (cloud metadata)
		{"http://[::1]/x", true},                          // IPv6 loopback
		{"http://[fe80::1]/x", true},                      // IPv6 link-local
		{"http://0.0.0.0/x", true},                        // unspecified
		{"http://100.64.0.1/x", true},                     // carrier-grade NAT
		{"file:///etc/passwd", true},                      // non-http scheme
		{"https://93.184.216.34/x", false},                // public over https
		{"http://[::127.0.0.1]/x", true},                  // IPv4-compatible IPv6 loopback
		{"http://[64:ff9b::a9fe:a9fe]/x", true},           // NAT64-embedded link-local (169.254.169.254)
	}
	for _, tc := range cases {
		t.Run(tc.url, func(t *testing.T) {
			err := DenyInternalHosts(tc.url)
			if tc.deny && err == nil {
				t.Errorf("DenyInternalHosts(%q) = nil, want denial", tc.url)
			}
			if !tc.deny && err != nil {
				t.Errorf("DenyInternalHosts(%q) = %v, want allow", tc.url, err)
			}
		})
	}
}

// TestRemoteFetchDisabledByDefault verifies that with no opt-in, an
// http(s) reference is not fetched: the transport is never reached and,
// under StrictAssets, ErrRemoteFetchDisabled is returned.
func TestRemoteFetchDisabledByDefault(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: errRoundTripper{calls: &calls}}

	_, err := Convert(`<img src="http://127.0.0.1:1/x.png"/>`, &Options{
		Client:       client,
		StrictAssets: true,
	})
	if err == nil {
		t.Fatal("expected ErrRemoteFetchDisabled under StrictAssets")
	}
	if !errors.Is(err, ErrRemoteFetchDisabled) {
		t.Errorf("error = %v, want ErrRemoteFetchDisabled", err)
	}
	if calls != 0 {
		t.Errorf("transport was reached %d times; no request should be made", calls)
	}
}

// TestLoopbackBlockedByDefaultPolicy verifies that with AllowRemoteFetch
// enabled but no URLPolicy, the built-in DenyInternalHosts blocks a
// loopback httptest server. The denial is logged (not escalated under
// StrictAssets), so we capture it through the Logger.
func TestLoopbackBlockedByDefaultPolicy(t *testing.T) {
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit.Store(true)
		_, _ = w.Write(makePNGBytes(t))
	}))
	defer srv.Close()

	h := &capturingHandler{}
	src := fmt.Sprintf(`<img src="%s/x.png"/>`, srv.URL)
	if _, err := Convert(src, &Options{
		AllowRemoteFetch: true,
		StrictAssets:     true,
		Logger:           slog.New(h),
	}); err != nil {
		t.Fatalf("URLPolicy denial must not escalate under StrictAssets: %v", err)
	}
	if !h.has(ErrURLPolicyDenied) {
		t.Error("expected a logged ErrURLPolicyDenied for the loopback target")
	}
	if hit.Load() {
		t.Error("loopback server was reached despite the default deny policy")
	}
}

// TestAllowAllOverrideFetches verifies that an explicit allow-all
// URLPolicy lets a loopback fetch succeed and decode into an image.
func TestAllowAllOverrideFetches(t *testing.T) {
	pngBytes := makePNGBytes(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(pngBytes)
	}))
	defer srv.Close()

	h := &capturingHandler{}
	src := fmt.Sprintf(`<img src="%s/photo.png"/>`, srv.URL)
	elems, err := Convert(src, &Options{
		AllowRemoteFetch: true,
		URLPolicy:        allowAllURLs,
		Logger:           slog.New(h),
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(elems) == 0 {
		t.Fatal("expected an element")
	}
	if _, ok := elems[0].(*layout.ImageElement); !ok {
		t.Fatalf("expected ImageElement, got %T (fetch may have failed)", elems[0])
	}
	if len(h.errs) != 0 {
		t.Errorf("expected no asset errors, got %v", h.errs)
	}
}

// TestRedirectHopRechecked verifies that DenyInternalHosts-style per-hop
// checking blocks a redirect from an allowed host to a denied one. Server
// A allows through, redirects to B; the policy denies B. The denial is
// logged (not escalated), so we capture it through the Logger.
func TestRedirectHopRechecked(t *testing.T) {
	srvB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(makePNGBytes(t))
	}))
	defer srvB.Close()

	srvA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, srvB.URL+"/b.png", http.StatusFound)
	}))
	defer srvA.Close()

	policy := func(u string) error {
		if u == srvB.URL+"/b.png" {
			return fmt.Errorf("host B denied")
		}
		return nil
	}

	h := &capturingHandler{}
	src := fmt.Sprintf(`<img src="%s/a.png"/>`, srvA.URL)
	if _, err := Convert(src, &Options{
		AllowRemoteFetch: true,
		StrictAssets:     true,
		URLPolicy:        policy,
		Logger:           slog.New(h),
	}); err != nil {
		t.Fatalf("URLPolicy denial must not escalate under StrictAssets: %v", err)
	}
	if !h.has(ErrURLPolicyDenied) {
		t.Error("expected the redirect to B to be blocked and logged as ErrURLPolicyDenied")
	}
}

// TestAbsolutePathDeniedByDefault verifies that an absolute filesystem
// path in document content is denied when BaseFS is nil and
// AllowAbsolutePaths is unset.
func TestAbsolutePathDeniedByDefault(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "photo.png")
	if err := os.WriteFile(abs, makePNGBytes(t), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Convert(fmt.Sprintf(`<img src="%s"/>`, abs), &Options{StrictAssets: true})
	if err == nil {
		t.Fatal("expected ErrAbsolutePathDenied under StrictAssets")
	}
	if !errors.Is(err, ErrAbsolutePathDenied) {
		t.Errorf("error = %v, want ErrAbsolutePathDenied", err)
	}
}

// TestAbsolutePathAllowedAndCapped verifies that with AllowAbsolutePaths
// an absolute PNG loads, and that readFileLimited enforces the byte cap.
func TestAbsolutePathAllowedAndCapped(t *testing.T) {
	dir := t.TempDir()
	abs := filepath.Join(dir, "photo.png")
	if err := os.WriteFile(abs, makePNGBytes(t), 0o644); err != nil {
		t.Fatal(err)
	}

	elems, err := Convert(fmt.Sprintf(`<img src="%s"/>`, abs), &Options{
		AllowAbsolutePaths: true,
		StrictAssets:       true,
	})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if _, ok := elems[0].(*layout.ImageElement); !ok {
		t.Fatalf("expected ImageElement, got %T", elems[0])
	}

	t.Run("readFileLimited", func(t *testing.T) {
		big := filepath.Join(t.TempDir(), "big.bin")
		if err := os.WriteFile(big, make([]byte, 100), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := readFileLimited(big, 10); err == nil {
			t.Error("expected error when file exceeds cap")
		}
		data, err := readFileLimited(big, 1000)
		if err != nil {
			t.Fatalf("within-cap read failed: %v", err)
		}
		if len(data) != 100 {
			t.Errorf("read %d bytes, want 100", len(data))
		}
	})
}

// TestSystemFontEscapeHatchUnaffected verifies that Options.FallbackFontPath
// still loads an absolute font path even when AllowAbsolutePaths is false:
// the programmatic font loader bypasses the gated resolver, so no
// ErrAbsolutePathDenied is produced for the font.
func TestSystemFontEscapeHatchUnaffected(t *testing.T) {
	abs, err := filepath.Abs("../font/testdata/synthetic_cjk.ttf")
	if err != nil {
		t.Fatalf("resolve fixture path: %v", err)
	}

	h := &capturingHandler{}
	// A non-WinAnsi character forces the fallback-font path to run.
	_, err = Convert(`<p>日</p>`, &Options{
		AllowAbsolutePaths: false,
		FallbackFontPath:   abs,
		StrictAssets:       true,
		Logger:             slog.New(h),
	})
	if err != nil {
		t.Fatalf("FallbackFontPath must load despite AllowAbsolutePaths=false: %v", err)
	}
	if h.has(ErrAbsolutePathDenied) {
		t.Error("FallbackFontPath was wrongly routed through the gated resolver")
	}
}

// TestAssetBudgetAccount unit-tests the aggregate counter directly.
func TestAssetBudgetAccount(t *testing.T) {
	if err := (*assetBudget)(nil).account(1 << 30); err != nil {
		t.Errorf("nil budget must never reject: %v", err)
	}
	b := newAssetBudget(100)
	if err := b.account(60); err != nil {
		t.Errorf("first read within cap: %v", err)
	}
	if err := b.account(40); err != nil {
		t.Errorf("read reaching the cap exactly must pass: %v", err)
	}
	if err := b.account(1); !errors.Is(err, ErrAssetBudgetExceeded) {
		t.Errorf("crossing the cap = %v, want ErrAssetBudgetExceeded", err)
	}
}

// TestAssetBudgetRejectsOnceExceeded verifies that several assets each
// under the per-asset cap but together over the aggregate cap are
// rejected once the running total crosses the budget.
func TestAssetBudgetRejectsOnceExceeded(t *testing.T) {
	pngBytes := makePNGBytes(t)
	s := int64(len(pngBytes))
	fsys := fstest.MapFS{
		"a.png": {Data: pngBytes},
		"b.png": {Data: pngBytes},
		"c.png": {Data: pngBytes},
	}

	// Budget admits exactly two images; the third crosses it.
	src := `<img src="a.png"/><img src="b.png"/><img src="c.png"/>`
	elems, err := Convert(src, &Options{
		BaseFS:             fsys,
		MaxTotalAssetBytes: 2 * s,
		StrictAssets:       true,
	})
	if err == nil {
		t.Fatal("expected ErrAssetBudgetExceeded once the budget is crossed")
	}
	if !errors.Is(err, ErrAssetBudgetExceeded) {
		t.Errorf("error = %v, want ErrAssetBudgetExceeded", err)
	}
	// The first two images fit and rendered; only the third was rejected.
	imgs := 0
	for _, e := range elems {
		if _, ok := e.(*layout.ImageElement); ok {
			imgs++
		}
	}
	if imgs != 2 {
		t.Errorf("rendered %d images, want 2 within budget", imgs)
	}
}

// TestAssetBudgetUnaffectedUnderCap verifies a normal document under the
// budget loads every asset without error.
func TestAssetBudgetUnaffectedUnderCap(t *testing.T) {
	pngBytes := makePNGBytes(t)
	fsys := fstest.MapFS{
		"a.png": {Data: pngBytes},
		"b.png": {Data: pngBytes},
	}
	h := &capturingHandler{}
	src := `<img src="a.png"/><img src="b.png"/>`
	elems, err := Convert(src, &Options{
		BaseFS:       fsys,
		StrictAssets: true, // default budget applies (512 MiB)
		Logger:       slog.New(h),
	})
	if err != nil {
		t.Fatalf("Convert under budget: %v", err)
	}
	if h.has(ErrAssetBudgetExceeded) {
		t.Error("budget rejected an asset under a normal document")
	}
	imgs := 0
	for _, e := range elems {
		if _, ok := e.(*layout.ImageElement); ok {
			imgs++
		}
	}
	if imgs != 2 {
		t.Errorf("rendered %d images, want 2", imgs)
	}
}
