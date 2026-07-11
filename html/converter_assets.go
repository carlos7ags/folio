// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
)

// errNoBaseFS is returned by readAsset when a relative or root-anchored path is
// requested but the caller did not configure Options.BaseFS.
var errNoBaseFS = errors.New("html: no BaseFS configured for local asset resolution")

// readAsset resolves p through baseFS. Path normalisation:
//   - Backslashes are converted to forward slashes (fs.FS convention).
//   - A leading "/" is stripped (web-style root → BaseFS root).
//   - "./" prefix is dropped; the result is path.Clean'ed.
//   - The cleaned path must satisfy fs.ValidPath, so ".." traversal is
//     rejected before fs.ReadFile is consulted.
//
// Returns errNoBaseFS when baseFS is nil. Callers that need OS access should
// pass os.DirFS("/") (or a more specific root) explicitly.
func readAsset(baseFS fs.FS, p string) ([]byte, error) {
	if baseFS == nil {
		return nil, errNoBaseFS
	}
	fsPath, err := normaliseFSPath(p)
	if err != nil {
		return nil, err
	}
	return fs.ReadFile(baseFS, fsPath)
}

// normaliseFSPath converts a user-supplied src/href into an fs.FS-valid path.
// Errors when the path is empty, contains a backslash that produces an invalid
// path after conversion, or escapes the root via "..". Backslashes are
// converted unconditionally (filepath.ToSlash is a no-op on non-Windows
// hosts, so we fold "\" → "/" explicitly so Windows-authored paths behave
// the same on macOS / Linux).
func normaliseFSPath(p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("html: empty path")
	}
	fsPath := strings.ReplaceAll(p, `\`, "/")
	fsPath = strings.TrimPrefix(fsPath, "./")
	fsPath = strings.TrimPrefix(fsPath, "/")
	if fsPath == "" {
		return "", fmt.Errorf("html: empty path after normalisation: %q", p)
	}
	fsPath = path.Clean(fsPath)
	if !fs.ValidPath(fsPath) {
		return "", fmt.Errorf("html: invalid path for BaseFS: %q", p)
	}
	return fsPath, nil
}

// resolveLocalAsset returns raw bytes for a local or remote asset reference,
// applying the project's uniform resolution contract. Every consumer
// (`<img>`, SVG inline, `<link rel="stylesheet">`, `@font-face url()`,
// background-image, FallbackFontPath) routes through here so a behavior
// change in one resolver becomes a behavior change in all of them.
//
// Resolution order:
//
//  1. src is an http/https URL — fetched only when opts.AllowRemoteFetch
//     is set (otherwise [ErrRemoteFetchDisabled]), filtered by urlPolicy
//     (or [DenyInternalHosts] when nil), re-checked on every redirect hop.
//     A policy denial is wrapped with [ErrURLPolicyDenied].
//
//  2. origin is an http/https URL, src is relative — resolved as a URL
//     against origin's host/path and fetched under the same gate.
//
//  3. src is filepath.IsAbs and opts.BaseFS is nil — read from the OS
//     only when opts.AllowAbsolutePaths is set (otherwise
//     [ErrAbsolutePathDenied]), size-capped like the HTTP path. The read
//     covers document-supplied absolute references such as
//     `<img src="/abs">` or `@font-face url('/abs')`; the programmatic
//     Options.FallbackFontPath and system-font search use a separate
//     trusted path and are unaffected. When opts.BaseFS is set, an
//     absolute path is treated as web-style root of BaseFS instead —
//     joinFSPath strips the leading slash and reads from the BaseFS root,
//     matching how `<base href="/">` resolves in browsers.
//
//  4. Otherwise — resolved via joinFSPath relative to origin's directory
//     (or BaseFS root for inline contexts) and read through opts.BaseFS.
//     Returns errNoBaseFS when opts.BaseFS is nil and src is non-absolute.
//
// origin is the document or stylesheet path/URL containing src. Pass ""
// for inline contexts: `<style>` blocks, top-level document references,
// and programmatic options like FallbackFontPath. Relative src values
// resolve relative to origin's directory (or BaseFS root when origin is
// empty).
//
// src is expected pre-trimmed of surrounding whitespace; call sites
// (parseStyleBlocks for href, parseFontFaceSrc for url(), getAttr for
// img src) already trim before reaching the resolver.
//
// maxBytes caps HTTP downloads. 0 (or any non-positive value) means use
// the default 50MB; pass 10MB for stylesheets to match the historical
// CSS fetch limit. The cap is ignored for filesystem reads — those are
// bounded by the source data.
//
// budget bounds the aggregate bytes read across all assets in one
// conversion; the returned bytes are charged against it. A nil budget
// imposes no aggregate cap.
//
// data: URIs are NOT handled here; callers parse them inline because
// each asset type has its own metadata-aware decoder (font.Face from
// font/x-truetype, *image.Image from image/png, raw bytes for CSS).
func resolveLocalAsset(opts Options, urlPolicy URLPolicy, budget *assetBudget, origin, src string, maxBytes int64) ([]byte, error) {
	var data []byte
	var err error
	switch {
	case isURL(src):
		data, err = fetchRemote(opts, urlPolicy, src, maxBytes)
	case isURL(origin):
		data, err = fetchRemote(opts, urlPolicy, joinURL(origin, src), maxBytes)
	case opts.BaseFS == nil && filepath.IsAbs(src):
		if !opts.AllowAbsolutePaths {
			return nil, fmt.Errorf("%w: %s", ErrAbsolutePathDenied, src)
		}
		data, err = readFileLimited(src, maxBytes)
	default:
		data, err = readAsset(opts.BaseFS, joinFSPath(origin, src))
	}
	if err != nil {
		return nil, err
	}
	if err := budget.account(int64(len(data))); err != nil {
		return nil, err
	}
	return data, nil
}

// assetBudget bounds the total bytes read across every asset load in one
// conversion, capping the aggregate memory that many references to the
// per-asset limit could otherwise amplify. Safe for concurrent use.
type assetBudget struct {
	used  atomic.Int64
	limit int64
}

// newAssetBudget returns a budget with the given limit, substituting the
// generous default when limit is non-positive.
func newAssetBudget(limit int64) *assetBudget {
	if limit <= 0 {
		limit = defaultMaxTotalAssetBytes
	}
	return &assetBudget{limit: limit}
}

// account charges n bytes against the budget, returning
// ErrAssetBudgetExceeded once the running total crosses the cap. A nil
// budget (or non-positive limit) never rejects.
func (b *assetBudget) account(n int64) error {
	if b == nil || b.limit <= 0 {
		return nil
	}
	if b.used.Add(n) > b.limit {
		return fmt.Errorf("%w: read exceeds %d bytes total", ErrAssetBudgetExceeded, b.limit)
	}
	return nil
}

// fetchRemote enforces the AllowRemoteFetch gate, selects the effective
// URL policy (the caller's, or DenyInternalHosts when nil), and performs
// the fetch with that policy applied pre-flight and on every redirect hop.
func fetchRemote(opts Options, userPolicy URLPolicy, rawURL string, maxBytes int64) ([]byte, error) {
	if !opts.AllowRemoteFetch {
		return nil, fmt.Errorf("%w: %s", ErrRemoteFetchDisabled, rawURL)
	}
	policy := userPolicy
	if policy == nil {
		policy = DenyInternalHosts
	}
	return fetchHTTPBytes(httpClientOrDefault(opts.Client), policy, rawURL, maxBytes)
}

// fetchHTTPBytes consults urlPolicy first (wrapping denials with
// ErrURLPolicyDenied so reportAssetError can distinguish caller intent
// from genuine load failure), then performs the GET via httpGetBytes
// on a client copy whose CheckRedirect re-applies urlPolicy to every
// redirect hop. A maxBytes value of 0 falls back to the 50MB default,
// matching the historical fetchImage cap.
func fetchHTTPBytes(client *http.Client, urlPolicy URLPolicy, url string, maxBytes int64) ([]byte, error) {
	if urlPolicy != nil {
		if err := urlPolicy(url); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrURLPolicyDenied, err)
		}
	}
	if maxBytes <= 0 {
		maxBytes = 50 << 20
	}
	return httpGetBytes(clientWithRedirectPolicy(client, urlPolicy), url, maxBytes)
}

// clientWithRedirectPolicy returns a shallow copy of client whose
// CheckRedirect re-applies policy to every redirect target (wrapping a
// denial as ErrURLPolicyDenied) and caps the redirect chain at 10, so a
// public first hop cannot bounce the request to an internal host. The
// caller's client is never mutated. A nil policy yields the standard
// redirect behavior with the 10-hop cap.
func clientWithRedirectPolicy(client *http.Client, policy URLPolicy) *http.Client {
	c := *client
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("html: stopped after 10 redirects")
		}
		if policy != nil {
			if err := policy(req.URL.String()); err != nil {
				return fmt.Errorf("%w: %w", ErrURLPolicyDenied, err)
			}
		}
		return nil
	}
	return &c
}

// DenyInternalHosts is a URLPolicy that blocks non-http(s) schemes and
// targets that resolve to loopback, private (RFC1918 / ULA), link-local,
// unspecified, or multicast addresses. It is the default policy applied
// when AllowRemoteFetch is true and Options.URLPolicy is nil, and it is
// re-checked on every redirect hop. Callers may also set it explicitly.
//
// For a DNS name, every resolved address is checked; the fetch is denied
// if any resolves to an internal address. This leaves a narrow TOCTOU
// window (the http client re-resolves at dial time); closing it fully
// would require a dial-time guard, tracked as a follow-up.
func DenyInternalHosts(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("html: parse url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("html: scheme %q not allowed", u.Scheme)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("html: empty host")
	}
	if ip := net.ParseIP(host); ip != nil {
		if isInternalIP(ip) {
			return fmt.Errorf("html: host %s resolves to internal address", host)
		}
		return nil
	}
	addrs, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("html: resolve %s: %w", host, err)
	}
	for _, ip := range addrs {
		if isInternalIP(ip) {
			return fmt.Errorf("html: host %s resolves to internal address %s", host, ip)
		}
	}
	return nil
}

// isInternalIP reports whether ip is one an asset fetch must never reach.
func isInternalIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		// Carrier-grade NAT 100.64.0.0/10 (RFC 6598) is not covered by
		// IsPrivate; block it too.
		return v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127
	}
	// IPv6 forms that embed an IPv4 address route to it, so classify the
	// embedded address: IPv4-compatible "::a.b.c.d" and the NAT64
	// well-known prefix 64:ff9b::/96 (last four bytes), either of which
	// could otherwise smuggle e.g. 169.254.169.254.
	if v16 := ip.To16(); v16 != nil {
		if isNAT64(v16) || isIPv4Compatible(v16) {
			return isInternalIP(net.IP(v16[12:16]))
		}
	}
	return false
}

// isIPv4Compatible reports whether v16 is the deprecated "::a.b.c.d" form
// (11 leading zero bytes, byte 11 zero) that maps directly to an IPv4.
func isIPv4Compatible(v16 net.IP) bool {
	for _, b := range v16[:12] {
		if b != 0 {
			return false
		}
	}
	return true
}

// isNAT64 reports whether v16 carries the NAT64 well-known prefix
// 64:ff9b::/96, whose last four bytes are an embedded IPv4.
func isNAT64(v16 net.IP) bool {
	prefix := []byte{0x00, 0x64, 0xff, 0x9b, 0, 0, 0, 0, 0, 0, 0, 0}
	for i := 0; i < 12; i++ {
		if v16[i] != prefix[i] {
			return false
		}
	}
	return true
}

// readFileLimited reads path into memory, capping at maxBytes to bound
// worst-case memory for absolute-path document assets (mirrors the HTTP
// path's cap). maxBytes <= 0 falls back to the 50MB default used by the
// HTTP path.
func readFileLimited(path string, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		maxBytes = 50 << 20
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("html: file %s exceeds %d bytes", path, maxBytes)
	}
	return data, nil
}

// resolveLocalAsset is the converter-method wrapper around the package
// free function. Use this from any code that already has a *converter;
// pre-converter call sites (parseStyleBlocks) call the free function
// directly with the already-built Options + URLPolicy values.
func (c *converter) resolveLocalAsset(origin, src string, maxBytes int64) ([]byte, error) {
	return resolveLocalAsset(c.opts, c.urlPolicy, c.budget, origin, src, maxBytes)
}

// httpClientOrDefault returns the configured HTTP client or http.DefaultClient.
func httpClientOrDefault(c *http.Client) *http.Client {
	if c != nil {
		return c
	}
	return http.DefaultClient
}

// loggerOrDiscard returns the configured logger or a no-op logger.
func loggerOrDiscard(l *slog.Logger) *slog.Logger {
	if l != nil {
		return l
	}
	return slog.New(slog.DiscardHandler)
}

// reportAssetError records a single asset-load failure. The event is always
// logged at warn level through Options.Logger (or dropped when Logger is
// nil); when Options.StrictAssets is true and the error is not an
// ErrURLPolicyDenied (which represents the caller's intent, not a load
// failure) it is additionally appended to c.strictErrs for return at the
// end of the conversion. category is a short label like "@font-face",
// "image", "background-image", "stylesheet". The error wrapped into
// strictErrs preserves the underlying err with errors.Is. attrs follows
// slog's variadic key/value convention; it is forwarded to the logger
// (with "error", err appended last to match the historical attr order
// callers may grep against) and inlined into the strict error message
// for grep-ability without needing structured-tree traversal.
func (c *converter) reportAssetError(category string, err error, attrs ...any) {
	logArgs := append(attrs, "error", err)
	c.logger.Warn("folio/html: "+category+" load failed", logArgs...)
	if c.opts.StrictAssets && !errors.Is(err, ErrURLPolicyDenied) {
		c.strictErrs = append(c.strictErrs, formatAssetError(category, err, attrs))
	}
}

// defaultMaxTotalAssetBytes is the aggregate per-conversion asset cap used
// when Options.MaxTotalAssetBytes is 0. Generous enough for realistic
// documents (several embedded fonts plus images) while bounding the
// worst case where many maxFontBytes-sized reads would otherwise multiply.
const defaultMaxTotalAssetBytes = 512 << 20

// joinFSPath resolves a relative src against the directory of an origin
// stylesheet path. An absolute src (leading "/" or "\") or empty origin
// resolves from the BaseFS root. A "../" in src is left for normaliseFSPath /
// fs.ValidPath to reject upstream. Backslashes in either argument are
// converted to forward slashes so Windows-authored paths behave the same on
// every host (filepath.ToSlash is a no-op on non-Windows builds).
func joinFSPath(origin, src string) string {
	src = strings.ReplaceAll(src, `\`, "/")
	if strings.HasPrefix(src, "/") {
		return src
	}
	if origin == "" {
		return src
	}
	dir := path.Dir(strings.ReplaceAll(origin, `\`, "/"))
	if dir == "." || dir == "/" {
		return src
	}
	return dir + "/" + src
}

// joinURL resolves a relative src against an HTTP origin URL. Absolute URLs
// in src bypass the origin. Anchor-style "/" paths resolve against the
// origin's host.
func joinURL(originURL, src string) string {
	if isURL(src) {
		return src
	}
	slash := strings.Index(originURL, "://")
	if slash < 0 {
		return src
	}
	hostStart := slash + 3
	hostEnd := strings.IndexByte(originURL[hostStart:], '/')
	if hostEnd < 0 {
		// origin has no path component; treat root as the host itself.
		if strings.HasPrefix(src, "/") {
			return originURL + src
		}
		return originURL + "/" + src
	}
	pathStart := hostStart + hostEnd
	if strings.HasPrefix(src, "/") {
		return originURL[:pathStart] + src
	}
	dir := path.Dir(originURL[pathStart:])
	if dir == "." || dir == "/" {
		return originURL[:pathStart] + "/" + src
	}
	resolved := path.Join(dir, src)
	return originURL[:pathStart] + "/" + strings.TrimPrefix(resolved, "/")
}
