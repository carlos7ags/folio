// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Options configures the HTML → layout.Element conversion.
type Options struct {
	// DefaultFontSize is the root font size in points (default 12).
	DefaultFontSize float64
	// BaseFS resolves all local paths referenced by the document: images,
	// fonts, linked stylesheets, background-image url(), and FallbackFontPath.
	// Pass embed.FS for embedded assets, os.DirFS(dir) or (*os.Root).FS() for
	// sandboxed directory access, fstest.MapFS in tests.
	// Paths are normalised to fs.FS conventions: forward slashes only, no
	// leading slash, no ".." traversal — invalid paths are rejected before
	// the read. A leading "/" in src/href is treated as web-style root and
	// stripped (resolved from the BaseFS root). When BaseFS is nil, every
	// local-asset reference fails — the document is expected to inline its
	// assets via data: URIs.
	BaseFS fs.FS
	// PageWidth is the page width in points (default 612 = US Letter).
	PageWidth float64
	// PageHeight is the page height in points (default 792 = US Letter).
	PageHeight float64
	// FallbackFontPath is a Unicode-capable TTF/OTF font used when text
	// contains characters outside WinAnsiEncoding (e.g. CJK, emoji). When
	// BaseFS is set, the path is resolved through it first; otherwise the
	// converter searches common system font locations.
	FallbackFontPath string
	// URLPolicy is called before the converter fetches a remote URL
	// (for <img src="http://...">, background-image: url(), etc.).
	// Return nil to allow the fetch, or an error to block it.
	// If nil, all URLs are allowed.
	URLPolicy URLPolicy
	// Client is the HTTP client used to fetch remote images and stylesheets.
	// If nil, http.DefaultClient is used.
	Client *http.Client
	// Logger receives warn-level events when an asset fails to load: missing
	// fonts in @font-face, unreadable linked stylesheets, image fetch errors
	// that fall back to alt text. If nil, these events are dropped.
	Logger *slog.Logger
	// StrictAssets promotes asset-load failures from warn-and-continue to
	// returned errors. When true, Convert and ConvertFull collect every
	// failed @font-face url(), <img>, background-image, linked stylesheet,
	// SVG load, and FallbackFontPath, then return them joined via
	// errors.Join at the end of the conversion. The partial result (the
	// elements that did render) is returned alongside the error so callers
	// can inspect both. Errors are returned in document order: linked
	// stylesheets first, then @font-face rules, then asset references in
	// tree-walk order — stable across runs given byte-identical input.
	//
	// URLPolicy denials are wrapped with ErrURLPolicyDenied and excluded
	// from the joined error, since they represent the caller's intent
	// (the policy callback already returned the signal it was wired to
	// produce) rather than a load failure. The denial is still logged
	// through Options.Logger.
	//
	// When false (default), every asset failure is logged through Logger
	// (if set) and the conversion continues. This suits production where
	// missing assets should not abort a render. Use StrictAssets in
	// development and CI to surface broken paths in the local feedback
	// loop instead of letting them silently degrade the output.
	StrictAssets bool

	// FlushListMarkers draws every nested ordered/unordered list marker in the
	// container's left column (all numbers share one left edge) instead of
	// indenting each level's marker under its parent's content. Body text still
	// cascades per level. This is the "outline" style common in legal
	// documents. Default false (markers indent per nesting level).
	FlushListMarkers bool

	// MaxElements caps the number of HTML nodes converted into layout
	// elements. 0 (the default) means unlimited. It guards against
	// resource exhaustion from very large or programmatically-expanded
	// input (e.g. a small template rendered against a huge dataset): once
	// the cap is crossed, Convert/ConvertFull stop walking the tree and
	// return a *LimitError (Kind LimitElements) instead of continuing to
	// allocate. Recommended for any path that converts untrusted HTML.
	MaxElements int

	// MaxDepth caps the nesting depth of converted elements. 0 (the
	// default) means unlimited. It guards against pathologically nested
	// input that would otherwise grow the conversion recursion (and the
	// goroutine stack) without bound. Exceeding it returns a *LimitError
	// (Kind LimitDepth).
	MaxDepth int
}

// URLPolicy controls whether the HTML converter may fetch a remote URL.
// It is called with the URL string before each HTTP request. Return nil
// to allow the fetch, or an error to block it and prevent the request.
type URLPolicy func(url string) error

// defaults returns a copy of Options with zero-value fields replaced by sensible defaults.
func (o *Options) defaults() Options {
	out := Options{DefaultFontSize: 12, PageWidth: 612, PageHeight: 792}
	if o == nil {
		return out
	}
	if o.DefaultFontSize > 0 {
		out.DefaultFontSize = o.DefaultFontSize
	}
	if o.PageWidth > 0 {
		out.PageWidth = o.PageWidth
	}
	if o.PageHeight > 0 {
		out.PageHeight = o.PageHeight
	}
	out.BaseFS = o.BaseFS
	out.FallbackFontPath = o.FallbackFontPath
	out.URLPolicy = o.URLPolicy
	out.Client = o.Client
	out.Logger = o.Logger
	out.StrictAssets = o.StrictAssets
	out.FlushListMarkers = o.FlushListMarkers
	out.MaxElements = o.MaxElements
	out.MaxDepth = o.MaxDepth
	return out
}

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
//  1. src is an http/https URL — fetched via opts.Client subject to
//     urlPolicy. A policy denial is wrapped with [ErrURLPolicyDenied].
//
//  2. origin is an http/https URL, src is relative — resolved as a URL
//     against origin's host/path and fetched.
//
//  3. src is filepath.IsAbs and opts.BaseFS is nil — read directly from
//     the OS via os.ReadFile. Typically a system font path like
//     "/System/Library/Fonts/STHeiti Light.ttc" or
//     `C:\Windows\Fonts\msyh.ttc` referenced from programmatically-built
//     HTML where the caller chose not to configure a BaseFS. When
//     opts.BaseFS is set, an absolute path is treated as web-style root
//     of BaseFS instead — joinFSPath strips the leading slash and reads
//     from the BaseFS root, matching how `<base href="/">` resolves in
//     browsers.
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
// data: URIs are NOT handled here; callers parse them inline because
// each asset type has its own metadata-aware decoder (font.Face from
// font/x-truetype, *image.Image from image/png, raw bytes for CSS).
func resolveLocalAsset(opts Options, urlPolicy URLPolicy, origin, src string, maxBytes int64) ([]byte, error) {
	switch {
	case isURL(src):
		return fetchHTTPBytes(httpClientOrDefault(opts.Client), urlPolicy, src, maxBytes)
	case isURL(origin):
		return fetchHTTPBytes(httpClientOrDefault(opts.Client), urlPolicy, joinURL(origin, src), maxBytes)
	case opts.BaseFS == nil && filepath.IsAbs(src):
		return os.ReadFile(src)
	default:
		return readAsset(opts.BaseFS, joinFSPath(origin, src))
	}
}

// fetchHTTPBytes consults urlPolicy first (wrapping denials with
// ErrURLPolicyDenied so reportAssetError can distinguish caller intent
// from genuine load failure), then performs the GET via httpGetBytes
// with the supplied byte cap. A maxBytes value of 0 falls back to the
// 50MB default, matching the historical fetchImage cap.
func fetchHTTPBytes(client *http.Client, urlPolicy URLPolicy, url string, maxBytes int64) ([]byte, error) {
	if urlPolicy != nil {
		if err := urlPolicy(url); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrURLPolicyDenied, err)
		}
	}
	if maxBytes <= 0 {
		maxBytes = 50 << 20
	}
	return httpGetBytes(client, url, maxBytes)
}

// resolveLocalAsset is the converter-method wrapper around the package
// free function. Use this from any code that already has a *converter;
// pre-converter call sites (parseStyleBlocks) call the free function
// directly with the already-built Options + URLPolicy values.
func (c *converter) resolveLocalAsset(origin, src string, maxBytes int64) ([]byte, error) {
	return resolveLocalAsset(c.opts, c.urlPolicy, origin, src, maxBytes)
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

// ConvertResult holds the full result of an HTML → layout conversion,
// including both normal-flow elements and absolutely positioned items.
type ConvertResult struct {
	Elements   []layout.Element
	Absolutes  []AbsoluteItem
	PageConfig *PageConfig // page settings from @page rules (nil if none)
	Metadata   DocMetadata // extracted from <title> and <meta> tags

	// MarginBoxes are ready-to-use margin box definitions from @page rules
	// (e.g. page numbers via @bottom-center). Pass directly to
	// document.SetMarginBoxes. Nil if no margin boxes were declared.
	MarginBoxes map[string]layout.MarginBox

	// FirstMarginBoxes are margin boxes for @page :first only.
	// Pass to document.SetFirstMarginBoxes. Nil if not declared.
	FirstMarginBoxes map[string]layout.MarginBox

	// LeftMarginBoxes are margin boxes for @page :left (even pages, LTR).
	// Pass to document.SetLeftMarginBoxes. Nil if not declared.
	LeftMarginBoxes map[string]layout.MarginBox

	// RightMarginBoxes are margin boxes for @page :right (odd pages, LTR).
	// Pass to document.SetRightMarginBoxes. Nil if not declared.
	RightMarginBoxes map[string]layout.MarginBox
}

// DocMetadata holds document metadata extracted from HTML head elements.
type DocMetadata struct {
	Title       string // from <title>
	Author      string // from <meta name="author">
	Description string // from <meta name="description">
	Keywords    string // from <meta name="keywords">
	Creator     string // from <meta name="generator">
	Subject     string // from <meta name="subject">

	// Language is the BCP-47 tag from <html lang="..."> when present
	// (e.g. "zh-CN", "ja", "en-US"). It is currently consumed by the
	// @font-face loader to select the appropriate face from pan-CJK
	// TTCs via font.ParseFontForLanguage — a document declaring
	// lang="zh-CN" with a NotoSansCJK-Regular.ttc loads the SC face
	// instead of the JP face-0 default. Per-element lang attributes
	// (<p lang="ja">) are NOT yet honoured; the property is currently
	// document-level only (issue #280, deferred Phase 2).
	Language string
}

// MarginBoxContent holds the parsed content of a CSS margin box (e.g. @top-center).
type MarginBoxContent struct {
	Content string // resolved content string (after evaluating counter(), string literals, etc.)
	// HasContent is true when a `content` declaration was present in the
	// margin-box rule, even if it resolved to the empty string (e.g.
	// `content: ""` or `content: none`). It lets a pseudo-page box
	// (@page :first { @bottom-center { content: "" } }) be preserved so it
	// can OVERRIDE — and thereby blank — the inherited default box for that
	// page/slot, instead of being dropped and falling back to the default.
	HasContent bool
	FontSize   float64    // font size in points (0 = use default 9pt)
	Color      [3]float64 // RGB color (0-1 each)
	// HasColor is true only when a `color` declaration was present in the
	// margin-box rule. It lets the renderer distinguish an explicit
	// `color: black` from an unset color (which defaults to gray).
	HasColor bool
	// Embedded is the document's default body font, stamped during
	// conversion so the renderer can draw the margin box with an embedded
	// (PDF/A-safe) font instead of the non-embedded standard Helvetica.
	// Nil when the document uses no embedded fonts. Font-family declared
	// inside the margin box itself is not yet honoured (follow-up).
	Embedded *font.EmbeddedFont
}

// PageMargins holds the margin values and margin-box content for a
// page variant (e.g. :first, :left, :right) parsed from a CSS @page rule.
type PageMargins struct {
	Top, Right, Bottom, Left float64
	HasMargins               bool                        // true if any margin property was explicitly set (even to 0)
	MarginBoxes              map[string]MarginBoxContent // e.g. "top-center" → content

	// Unresolved length trees for each side, preserved so percent / calc
	// can be resolved against the page box at apply time (the float
	// fields above are eagerly resolved with a zero basis and are only
	// correct for absolute units). nil when the side was not set.
	topLen, rightLen, bottomLen, leftLen *cssLength

	// fontSize is the default font size captured at parse time, used to
	// resolve em / rem inside the deferred length trees.
	fontSize float64
}

// ResolveMargins resolves any deferred (percent / calc) margin lengths
// against the page box. Per CSS Paged Media, @page margin percentages
// resolve against the page dimensions: left/right against pageW,
// top/bottom against pageH. Sides without a deferred length keep their
// eagerly resolved float value.
func (m *PageMargins) ResolveMargins(pageW, pageH float64) {
	if m.topLen != nil {
		m.Top = m.topLen.toPoints(pageH, m.fontSize)
	}
	if m.bottomLen != nil {
		m.Bottom = m.bottomLen.toPoints(pageH, m.fontSize)
	}
	if m.rightLen != nil {
		m.Right = m.rightLen.toPoints(pageW, m.fontSize)
	}
	if m.leftLen != nil {
		m.Left = m.leftLen.toPoints(pageW, m.fontSize)
	}
}

// PageConfig holds page dimensions and margins from CSS @page rules.
type PageConfig struct {
	Width      float64 // page width in points (0 = use default)
	Height     float64 // page height in points (0 = use default)
	AutoHeight bool    // true when @page size has explicit height of 0 (size to content)
	Landscape  bool

	// OrientationOnly is true when @page { size: ... } gave only an
	// orientation keyword (landscape/portrait) with no named size or
	// explicit dimensions. Width/Height are then 0 and the orientation
	// must be applied to the document default page size at apply time.
	// Landscape distinguishes the two keywords (true = landscape).
	OrientationOnly bool

	// Default margins (from @page with no pseudo-selector).
	MarginTop    float64
	MarginRight  float64
	MarginBottom float64
	MarginLeft   float64
	HasMargins   bool // true if any margin property was explicitly set (even to 0)

	// Unresolved length trees for the default margins, preserved so
	// percent / calc resolve against the page box at apply time. The
	// float fields above are eagerly resolved with a zero basis and are
	// only correct for absolute units. nil when not set.
	marginTopLen, marginRightLen, marginBottomLen, marginLeftLen *cssLength

	// fontSize is the default font size captured at parse time, used to
	// resolve em / rem inside the deferred length trees.
	fontSize float64

	// Per-page-type margin overrides (nil = use default).
	First *PageMargins // @page :first
	Left  *PageMargins // @page :left (even pages in LTR)
	Right *PageMargins // @page :right (odd pages in LTR)

	// Default margin boxes (from @page with no pseudo-selector).
	MarginBoxes map[string]MarginBoxContent // e.g. "top-center" → content
}

// ResolveMargins resolves the default-margin deferred lengths against the
// page box, then resolves any per-page-type variant overrides
// (:first, :left, :right). See PageMargins.ResolveMargins for the
// percent basis rules. pageW / pageH are the resolved page dimensions in
// points (the @page size if given, otherwise the document default).
func (pc *PageConfig) ResolveMargins(pageW, pageH float64) {
	if pc.marginTopLen != nil {
		pc.MarginTop = pc.marginTopLen.toPoints(pageH, pc.fontSize)
	}
	if pc.marginBottomLen != nil {
		pc.MarginBottom = pc.marginBottomLen.toPoints(pageH, pc.fontSize)
	}
	if pc.marginRightLen != nil {
		pc.MarginRight = pc.marginRightLen.toPoints(pageW, pc.fontSize)
	}
	if pc.marginLeftLen != nil {
		pc.MarginLeft = pc.marginLeftLen.toPoints(pageW, pc.fontSize)
	}
	if pc.First != nil {
		pc.First.ResolveMargins(pageW, pageH)
	}
	if pc.Left != nil {
		pc.Left.ResolveMargins(pageW, pageH)
	}
	if pc.Right != nil {
		pc.Right.ResolveMargins(pageW, pageH)
	}
}

// Resolve computes the final page geometry for this @page config and
// resolves all margin lengths (default + :first / :left / :right) against
// that geometry. It is the single entry point every Document-building
// consumer must call so that page sizing, the orientation-only swap, and
// deferred percent / calc margin resolution behave identically across all
// code paths (AddHTML, the C ABI, WASM, the tmpl package, and examples).
//
// defaultW / defaultH are the document's default page dimensions in points,
// used when the @page rule supplied no explicit size (or only an
// orientation keyword). The returned width / height are the final page
// dimensions; autoHeight reports the CSS `size: <w> 0` content-sized case
// (height is then 0). Margins are resolved in place on pc and read by the
// caller from pc.MarginTop/… and pc.First/Left/Right after this returns.
//
// Precedence (S-1): an orientation-only keyword (size: landscape | portrait)
// rotates whatever size is otherwise in effect — the explicit @page dims if
// given, else the document default — rather than being ignored when explicit
// dims exist.
func (pc *PageConfig) Resolve(defaultW, defaultH float64) (width, height float64, autoHeight bool) {
	switch {
	case pc.Width > 0 && (pc.Height > 0 || pc.AutoHeight):
		// Explicit @page dimensions. parsePageSize already applied any
		// `landscape` keyword that accompanied a named/explicit size.
		width, height, autoHeight = pc.Width, pc.Height, pc.AutoHeight
	default:
		// No explicit @page size: start from the document default.
		width, height = defaultW, defaultH
	}

	// Orientation-only keyword (size: landscape | portrait) applies its
	// orientation to whatever dims are now in effect, swapping if needed.
	// AutoHeight pages have no meaningful orientation, so leave them.
	if pc.OrientationOnly && !autoHeight {
		if pc.Landscape && width < height {
			width, height = height, width
		} else if !pc.Landscape && width > height {
			width, height = height, width
		}
	}

	// Resolve @page margin percentages / calc against the final page box.
	// Per CSS Paged Media, left/right resolve against width and top/bottom
	// against height. AutoHeight uses defaultH as the percent basis since
	// the final height is content-driven and unknown here.
	basisH := height
	if autoHeight {
		basisH = defaultH
	}
	pc.ResolveMargins(width, basisH)

	return width, height, autoHeight
}

// convertMarginBoxes converts html.MarginBoxContent to layout.MarginBox,
// stamping the document's default body font (emb) onto each box so the
// renderer draws running headers/footers with an embedded, PDF/A-safe font
// instead of the non-embedded standard Helvetica (issue #328). emb may be
// nil when the document has no embedded fonts; the renderer then falls back
// to Helvetica, which is acceptable because such a document is not PDF/A.
func convertMarginBoxes(src map[string]MarginBoxContent, emb *font.EmbeddedFont) map[string]layout.MarginBox {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]layout.MarginBox, len(src))
	for name, mbc := range src {
		out[name] = layout.MarginBox{
			Content:  mbc.Content,
			FontSize: mbc.FontSize,
			Color:    mbc.Color,
			HasColor: mbc.HasColor,
			Embedded: emb,
		}
	}
	return out
}

// stampMarginBoxFont writes emb onto the Embedded field of every
// MarginBoxContent in src. It is used so the document.AddHTML path, which
// reconstructs layout.MarginBox from PageConfig rather than from the
// already-converted ConvertResult.MarginBoxes, embeds the same body font.
func stampMarginBoxFont(src map[string]MarginBoxContent, emb *font.EmbeddedFont) {
	for name, mbc := range src {
		mbc.Embedded = emb
		src[name] = mbc
	}
}

// defaultMarginBoxFont returns the embedded font that body text resolves to,
// for use as the default font of @page margin boxes. It computes the <body>
// element's cascaded style (so an author's `body { font-family: 'X' }` with a
// matching @font-face is honoured) and resolves that style to an embedded
// font. Returns nil when the document uses no embedded fonts (pure
// standard-font document), in which case the renderer keeps the Helvetica
// fallback. Font-family declared inside the margin box itself is not parsed
// yet (deferred follow-up); the body font is the default per CSS GCPM.
func (c *converter) defaultMarginBoxFont(doc *html.Node, root computedStyle) *font.EmbeddedFont {
	if len(c.embeddedFonts) == 0 {
		return nil
	}
	style := root
	if body := findBodyNode(doc); body != nil {
		style = c.computeElementStyle(body, root)
	}
	_, emb := c.resolveFontPair(style)
	return emb
}

// findBodyNode returns the first <body> element in the parsed tree, or nil.
func findBodyNode(doc *html.Node) *html.Node {
	var walk func(*html.Node) *html.Node
	walk = func(n *html.Node) *html.Node {
		if n == nil {
			return nil
		}
		if n.Type == html.ElementNode && n.DataAtom == atom.Body {
			return n
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			if found := walk(ch); found != nil {
				return found
			}
		}
		return nil
	}
	return walk(doc)
}

// AbsoluteItem represents an element removed from normal flow via
// position:absolute or position:fixed.
type AbsoluteItem struct {
	Element      layout.Element
	X, Y         float64 // X from left edge, Y from top in PDF coordinates (bottom-left origin)
	Width        float64
	Fixed        bool // position:fixed (render on every page)
	RightAligned bool // true when positioned with CSS right (X is right-edge offset)
	ZIndex       int  // z-index: negative = render behind normal flow
}

// ConvertFull parses an HTML string and returns both normal-flow elements
// and absolutely positioned items. It is equivalent to
// ConvertFullWithContext with a background context.
func ConvertFull(htmlStr string, opts *Options) (*ConvertResult, error) {
	return ConvertFullWithContext(context.Background(), htmlStr, opts)
}

// ConvertFullWithContext is the context-aware variant of ConvertFull. It
// checks ctx at element boundaries while walking the HTML tree and returns
// ctx.Err() (context.Canceled or context.DeadlineExceeded) if the context
// is done, letting callers bound the conversion of pathological input with
// a deadline or cancellation. A nil result is returned on cancellation.
func ConvertFullWithContext(ctx context.Context, htmlStr string, opts *Options) (*ConvertResult, error) {
	o := opts.defaults()
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, &ParseError{Err: err}
	}

	style := defaultStyle()
	style.FontSize = o.DefaultFontSize

	logger := loggerOrDiscard(o.Logger)
	var stylesheetErrs []error
	logStylesheetErr := func(href string, err error) {
		logger.Warn("folio/html: stylesheet load failed", "href", href, "error", err)
		if o.StrictAssets {
			stylesheetErrs = append(stylesheetErrs, formatAssetError("stylesheet", err, []any{"href", href}))
		}
	}
	ss := parseStyleBlocks(doc, o, logStylesheetErr)

	c := &converter{opts: o, logger: logger, rootFontSize: o.DefaultFontSize, sheet: ss, embeddedFonts: make(map[string]*font.EmbeddedFont), containerWidth: o.PageWidth, counters: make(map[string][]int), urlPolicy: o.URLPolicy, strictErrs: stylesheetErrs, ctx: ctx}

	// Parse @page config early so containerWidth reflects the actual page size
	// (e.g. landscape pages have a wider containerWidth).
	var pageConfig *PageConfig
	if len(ss.pageRules) > 0 {
		pageConfig = parsePageConfig(ss.pageRules, o.DefaultFontSize)
		if pageConfig != nil && pageConfig.Width > 0 {
			c.containerWidth = pageConfig.Width
			c.opts.PageWidth = pageConfig.Width
			c.opts.PageHeight = pageConfig.Height
		}
	}

	// Extract <html lang> before loading @font-face URLs so the
	// document-level language drives TTC face selection. Storing on
	// the metadata struct also exposes it to callers via
	// ConvertResult below.
	c.metadata.Language = findHTMLLang(doc)

	// Load @font-face fonts.
	c.loadFontFaces(ss.fontFaces)

	elems := c.walkChildren(doc, style)
	if c.ctxErr != nil {
		return nil, c.ctxErr
	}
	if c.limitErr != nil {
		return nil, c.limitErr
	}
	result := &ConvertResult{Elements: elems, Absolutes: c.absolutes, Metadata: c.metadata}
	result.PageConfig = pageConfig

	// Build ready-to-use margin box maps so callers can pass them
	// directly to doc.SetMarginBoxes without type conversion. The
	// document's default body font is stamped onto each box (issue #328)
	// and also onto the MarginBoxContent values in pageConfig so the
	// document.AddHTML path (which rebuilds layout.MarginBox from
	// pageConfig) embeds the same font.
	if pageConfig != nil {
		marginFont := c.defaultMarginBoxFont(doc, style)
		stampMarginBoxFont(pageConfig.MarginBoxes, marginFont)
		result.MarginBoxes = convertMarginBoxes(pageConfig.MarginBoxes, marginFont)
		if pageConfig.First != nil {
			stampMarginBoxFont(pageConfig.First.MarginBoxes, marginFont)
			result.FirstMarginBoxes = convertMarginBoxes(pageConfig.First.MarginBoxes, marginFont)
		}
		if pageConfig.Left != nil {
			stampMarginBoxFont(pageConfig.Left.MarginBoxes, marginFont)
			result.LeftMarginBoxes = convertMarginBoxes(pageConfig.Left.MarginBoxes, marginFont)
		}
		if pageConfig.Right != nil {
			stampMarginBoxFont(pageConfig.Right.MarginBoxes, marginFont)
			result.RightMarginBoxes = convertMarginBoxes(pageConfig.Right.MarginBoxes, marginFont)
		}
	}

	if len(c.strictErrs) > 0 {
		return result, errors.Join(c.strictErrs...)
	}
	return result, nil
}

// Convert parses an HTML string and returns a slice of layout elements
// suitable for passing to a layout.Renderer. Only a subset of HTML is
// supported — see package documentation for details. It is equivalent to
// ConvertWithContext with a background context.
func Convert(htmlStr string, opts *Options) ([]layout.Element, error) {
	return ConvertWithContext(context.Background(), htmlStr, opts)
}

// ConvertWithContext is the context-aware variant of Convert. It checks ctx
// at element boundaries while walking the HTML tree and returns ctx.Err()
// if the context is done.
func ConvertWithContext(ctx context.Context, htmlStr string, opts *Options) ([]layout.Element, error) {
	o := opts.defaults()
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, &ParseError{Err: err}
	}

	style := defaultStyle()
	style.FontSize = o.DefaultFontSize

	logger := loggerOrDiscard(o.Logger)
	var stylesheetErrs []error
	logStylesheetErr := func(href string, err error) {
		logger.Warn("folio/html: stylesheet load failed", "href", href, "error", err)
		if o.StrictAssets {
			stylesheetErrs = append(stylesheetErrs, formatAssetError("stylesheet", err, []any{"href", href}))
		}
	}
	ss := parseStyleBlocks(doc, o, logStylesheetErr)

	c := &converter{opts: o, logger: logger, rootFontSize: o.DefaultFontSize, sheet: ss, embeddedFonts: make(map[string]*font.EmbeddedFont), containerWidth: o.PageWidth, counters: make(map[string][]int), urlPolicy: o.URLPolicy, strictErrs: stylesheetErrs, ctx: ctx}

	// Update containerWidth if @page specifies a different page size.
	if len(ss.pageRules) > 0 {
		if pc := parsePageConfig(ss.pageRules, o.DefaultFontSize); pc != nil && pc.Width > 0 {
			c.containerWidth = pc.Width
			c.opts.PageWidth = pc.Width
			c.opts.PageHeight = pc.Height
		}
	}

	// Match ConvertFull: extract <html lang> before @font-face load
	// so the document-level language drives TTC face selection.
	c.metadata.Language = findHTMLLang(doc)

	// Load @font-face fonts.
	c.loadFontFaces(ss.fontFaces)

	elems := c.walkChildren(doc, style)
	if c.ctxErr != nil {
		return nil, c.ctxErr
	}
	if c.limitErr != nil {
		return nil, c.limitErr
	}
	if len(c.strictErrs) > 0 {
		return elems, errors.Join(c.strictErrs...)
	}
	return elems, nil
}

type converter struct {
	opts           Options
	logger         *slog.Logger
	rootFontSize   float64
	sheet          *styleSheet
	embeddedFonts  map[string]*font.EmbeddedFont // family+"|"+weight+"|"+style → embedded font
	absolutes      []AbsoluteItem
	metadata       DocMetadata
	containerWidth float64 // current container width in points for resolving % widths

	// Unicode fallback: lazily loaded when text contains non-WinAnsi characters.
	fallbackFont       *font.EmbeddedFont
	fallbackFontLoaded bool // true after first attempt (even if failed)

	// CSS counters: maps counter name → stack of values (for nesting).
	counters map[string][]int

	// Positioned ancestor stack for resolving position:absolute against the
	// nearest containing block (position:relative/absolute/fixed ancestor).
	positionedAncestors []containingBlock

	// urlPolicy is called before fetching remote URLs. Nil means allow all.
	urlPolicy URLPolicy

	// strictErrs accumulates asset-load failures when Options.StrictAssets
	// is true. Convert / ConvertFull return errors.Join(strictErrs...) at
	// the end of the run. When StrictAssets is false this slice is never
	// appended to — reportAssetError still calls Logger.Warn.
	strictErrs []error

	// Resource guards (Options.MaxElements / MaxDepth). nodeCount counts
	// the nodes converted so far; depth tracks the current nesting level
	// (incremented on convertNode entry, decremented on exit). limitErr is
	// set the first time a ceiling is crossed; once set, convertNode and
	// walkChildren unwind without further work and Convert / ConvertFull
	// return it. Both ceilings are disabled when their Option is 0.
	nodeCount int
	depth     int
	limitErr  error

	// ctx bounds the conversion walk. It is stored on this short-lived,
	// per-conversion worker (never shared or persisted) so convertNode can
	// check it at element boundaries without threading it through every
	// recursive signature. nil means no cancellation (Convert/ConvertFull
	// use context.Background). ctxErr records the first ctx.Err() seen and
	// aborts the remaining walk; Convert/ConvertFull return it.
	ctx    context.Context
	ctxErr error
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

// containingBlock tracks a positioned ancestor for absolute positioning resolution.
type containingBlock struct {
	width   float64          // resolved content width in points
	height  float64          // resolved content height in points (0 if unknown)
	pending []pendingOverlay // absolute children waiting to be attached to the Div
}

// pendingOverlay stores an absolute element waiting to be attached to its
// containing block's Div.
type pendingOverlay struct {
	elem         layout.Element
	x, y         float64
	width        float64
	rightAligned bool
	zIndex       int
}

// loadFontFaces loads @font-face fonts into the converter's embeddedFonts map.
// data: URIs are decoded inline; every other src — http(s) URL, BaseFS-
// relative path, and absolute filesystem path — flows through the unified
// [resolveLocalAsset] contract. The font is resolved relative to the
// stylesheet it was declared in (its origin), so url("../fonts/x.ttf")
// inside styles/site.css resolves to fonts/x.ttf at the BaseFS root.
// Inline <style> blocks resolve from the BaseFS root.
//
// Failures (missing data, invalid font bytes, fetch errors) are reported
// through Options.Logger at warn level and skipped — they never abort the
// conversion.
func (c *converter) loadFontFaces(faces []fontFaceRule) {
	// Document-level lang drives TTC face selection. Empty lang is
	// the no-op default — font.ParseFontForLanguage with "" picks
	// face 0, matching the legacy font.ParseFont behaviour for
	// back-compat. Per-element lang overrides (<p lang="ja">) are
	// not yet honoured (#280 Phase 2): a single face is loaded per
	// @font-face rule at converter setup time, so an element-level
	// override would need either eager multi-face loading or a
	// shape-time face selector — both deferred.
	lang := c.metadata.Language
	for _, ff := range faces {
		src := ff.src

		var face font.Face
		var data []byte
		var err error

		if strings.HasPrefix(src, "data:") {
			face, err = decodeFontDataURI(src)
		} else {
			data, err = c.resolveLocalAsset(ff.origin, src, 50<<20)
			if err == nil {
				face, err = font.ParseFontForLanguage(data, lang)
			}
		}

		if err != nil {
			c.reportAssetError("@font-face", err,
				"family", ff.family, "src", src, "origin", ff.origin)
			continue
		}
		ef := font.NewEmbeddedFont(face)
		// Key shape: family|<numeric weight>|style. The numeric weight
		// matches what computedStyle.FontWeight stores after
		// parseFontWeight, which lets resolveFontPair walk the available
		// weights for nearest-match selection.
		key := ff.family + "|" + strconv.Itoa(ff.weight) + "|" + ff.style
		c.embeddedFonts[key] = ef
	}
}

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

// decodeFontDataURI decodes a base64-encoded font from a data: URI.
// Supports data:font/truetype;base64,..., data:font/opentype;base64,...,
// data:application/x-font-ttf;base64,..., and similar media types.
func decodeFontDataURI(uri string) (font.Face, error) {
	rest := strings.TrimPrefix(uri, "data:")
	commaIdx := strings.IndexByte(rest, ',')
	if commaIdx < 0 {
		return nil, fmt.Errorf("html: invalid data URI: no comma")
	}
	meta := rest[:commaIdx]
	encoded := rest[commaIdx+1:]

	if !strings.Contains(meta, ";base64") {
		return nil, fmt.Errorf("html: font data URI must be base64-encoded")
	}

	data, err := base64Decode(encoded)
	if err != nil {
		return nil, fmt.Errorf("html: font data URI base64: %w", err)
	}

	return font.ParseTTF(data)
}

// getFallbackFont returns a Unicode-capable embedded font for text that
// can't be encoded in WinAnsiEncoding. The font is loaded lazily on first
// use. Returns nil if no suitable font is found.
//
// Lookup order: Options.FallbackFontPath via BaseFS, then via OS, then a
// hardcoded list of common system font locations.
func (c *converter) getFallbackFont() *font.EmbeddedFont {
	if c.fallbackFontLoaded {
		return c.fallbackFont
	}
	c.fallbackFontLoaded = true

	// Document lang reaches this lazy load because getFallbackFont
	// is only invoked from chooseFont during walkChildren, which
	// runs after findHTMLLang in ConvertFull/Convert. The same TTC
	// face-selection rules that govern @font-face apply: a doc with
	// lang="zh-CN" pulling NotoSansCJK-Regular.ttc as the system
	// fallback picks the SC face instead of JP face-0.
	lang := c.metadata.Language

	if c.opts.FallbackFontPath != "" {
		if face, err := c.loadFallbackFont(c.opts.FallbackFontPath, lang); err == nil {
			c.fallbackFont = font.NewEmbeddedFont(face)
			return c.fallbackFont
		} else {
			c.reportAssetError("FallbackFontPath", err,
				"path", c.opts.FallbackFontPath)
		}
	}

	// Search common system font locations for a Unicode-capable font.
	// CJK-specific fonts are listed first since they provide the widest
	// coverage for East Asian scripts while also covering Latin.
	candidates := []string{
		// macOS — CJK fonts
		"/Library/Fonts/Arial Unicode.ttf",
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		"/System/Library/Fonts/STHeiti Light.ttc",
		"/System/Library/Fonts/PingFang.ttc",
		"/System/Library/Fonts/Hiragino Sans GB.ttc",
		// macOS — general Unicode
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/System/Library/Fonts/Helvetica.ttc",
		// Linux — CJK fonts
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/google-noto-cjk/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		// Linux — general Unicode
		"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
		"/usr/share/fonts/noto/NotoSans-Regular.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/dejavu/DejaVuSans.ttf",
		// Windows — CJK fonts
		`C:\Windows\Fonts\msyh.ttc`,
		`C:\Windows\Fonts\msgothic.ttc`,
		`C:\Windows\Fonts\malgun.ttf`,
		`C:\Windows\Fonts\simsun.ttc`,
		// Windows — general Unicode
		`C:\Windows\Fonts\arial.ttf`,
		`C:\Windows\Fonts\segoeui.ttf`,
	}
	for _, path := range candidates {
		if face, err := font.LoadFontForLanguage(path, lang); err == nil {
			c.fallbackFont = font.NewEmbeddedFont(face)
			return c.fallbackFont
		}
	}

	return nil
}

// loadFallbackFont resolves the FallbackFontPath option with two
// programmatic-only carve-outs from the standard [resolveLocalAsset]
// contract that document-content resolvers do not get:
//
//  1. An absolute path always bypasses BaseFS to the OS, regardless of
//     whether BaseFS is set. FallbackFontPath is configured by the
//     embedding application, almost always at a system font location;
//     callers typically build BaseFS for their asset directory rather
//     than the system font tree. Document-supplied absolute paths
//     (`<img src="/abs">`, `@font-face url('/abs')`) follow the
//     centralized rule because the asset reference comes from
//     untrusted content.
//
//  2. A relative path that misses in BaseFS retries against the OS so
//     a typo in the option does not silently produce no-fallback text.
//     The retry is logged at debug level so the BaseFS attempt remains
//     observable when investigating which path the loader took.
func (c *converter) loadFallbackFont(p, lang string) (font.Face, error) {
	if filepath.IsAbs(p) {
		return font.LoadFontForLanguage(p, lang)
	}
	if c.opts.BaseFS != nil {
		data, baseErr := c.resolveLocalAsset("", p, 50<<20)
		if baseErr == nil {
			return font.ParseFontForLanguage(data, lang)
		}
		c.logger.Debug("folio/html: FallbackFontPath not in BaseFS, trying OS",
			"path", p, "error", baseErr)
	}
	return font.LoadFontForLanguage(p, lang)
}

// resolveFontForText returns the best font for the given text. If the text
// can be encoded in WinAnsiEncoding, returns the standard font. Otherwise,
// tries the embedded fonts from @font-face, then the system fallback font.
func (c *converter) resolveFontForText(style computedStyle, text string) (*font.Standard, *font.EmbeddedFont) {
	stdFont, embFont := c.resolveFontPair(style)

	// If already using an embedded font (from @font-face), it handles Unicode.
	if embFont != nil {
		return nil, embFont
	}

	// Standard font — check if text fits in WinAnsiEncoding.
	if font.CanEncodeWinAnsi(text) {
		return stdFont, nil
	}

	// Text has non-WinAnsi characters — try fallback.
	if fb := c.getFallbackFont(); fb != nil {
		return nil, fb
	}

	// No fallback available — use standard font (chars will become ?).
	return stdFont, nil
}

// applyUnicodeBidi wraps text in Unicode bidi control characters based on
// the computed CSS direction and unicode-bidi properties. This implements
// CSS Unicode Bidirectional Algorithm integration:
//
//   - bidi-override + rtl: wraps in RLO...PDF (U+202E...U+202C) to force
//     all characters to RTL visual order regardless of their bidi class.
//   - bidi-override + ltr: wraps in LRO...PDF (U+202D...U+202C).
//   - embed + rtl: wraps in RLE...PDF (U+202B...U+202C).
//   - embed + ltr: wraps in LRE...PDF (U+202A...U+202C).
//   - isolate + rtl: wraps in RLI...PDI (U+2067...U+2069).
//   - isolate + ltr: wraps in LRI...PDI (U+2066...U+2069).
//
// These characters are consumed by the bidi algorithm in x/text/unicode/bidi
// during resolveLineBidi, producing the correct embedding levels.
func applyUnicodeBidi(text string, style computedStyle) string {
	if style.UnicodeBidi == "" || style.UnicodeBidi == "normal" {
		return text
	}
	switch style.UnicodeBidi {
	case "bidi-override":
		if style.Direction == layout.DirectionRTL {
			return "\u202E" + text + "\u202C" // RLO + text + PDF
		}
		if style.Direction == layout.DirectionLTR {
			return "\u202D" + text + "\u202C" // LRO + text + PDF
		}
	case "embed":
		if style.Direction == layout.DirectionRTL {
			return "\u202B" + text + "\u202C" // RLE + text + PDF
		}
		if style.Direction == layout.DirectionLTR {
			return "\u202A" + text + "\u202C" // LRE + text + PDF
		}
	case "isolate":
		if style.Direction == layout.DirectionRTL {
			return "\u2067" + text + "\u2069" // RLI + text + PDI
		}
		if style.Direction == layout.DirectionLTR {
			return "\u2066" + text + "\u2069" // LRI + text + PDI
		}
	case "isolate-override":
		if style.Direction == layout.DirectionRTL {
			return "\u2067\u202E" + text + "\u202C\u2069"
		}
		if style.Direction == layout.DirectionLTR {
			return "\u2067\u202D" + text + "\u202C\u2069"
		}
	}
	return text
}

// splitTextByFont splits a text string into one or more TextRuns at script
// boundaries where the font needs to change. Characters encodable in
// WinAnsiEncoding use the standard font; characters that need a fallback
// (Hebrew, Arabic, CJK, etc.) use the embedded fallback font. This enables
// mixed-script text like "Hello שלום" to render correctly when the standard
// font lacks Hebrew glyphs but the fallback font covers both scripts.
//
// When the style already specifies an embedded font (via @font-face), or
// when no fallback font is available, the text is returned as a single run
// (no splitting needed — same behavior as before this function existed).
func (c *converter) splitTextByFont(text string, style computedStyle) []layout.TextRun {
	// Apply unicode-bidi overrides by wrapping text in Unicode bidi
	// control characters. This forces the base embedding level per
	// CSS Unicode Bidirectional Algorithm §2.2.
	text = applyUnicodeBidi(text, style)

	stdFont, embFont := c.resolveFontPair(style)

	// If already using an embedded font, it handles all Unicode — no split.
	if embFont != nil {
		return []layout.TextRun{c.makeTextRun(text, nil, embFont, style)}
	}

	// If all text fits in WinAnsi, use standard font — no split.
	if font.CanEncodeWinAnsi(text) {
		return []layout.TextRun{c.makeTextRun(text, stdFont, nil, style)}
	}

	// Get the fallback font. If unavailable, return single run with std font.
	fb := c.getFallbackFont()
	if fb == nil {
		return []layout.TextRun{c.makeTextRun(text, stdFont, nil, style)}
	}

	// Split at boundaries between WinAnsi-encodable and non-WinAnsi characters.
	// Consecutive characters that share the same "needs fallback" status are
	// grouped into a single run to minimize run count.
	runes := []rune(text)
	var runs []layout.TextRun
	start := 0
	startNeedsFallback := !font.CanEncodeWinAnsiRune(runes[0])

	for i := 1; i <= len(runes); i++ {
		needsFallback := false
		if i < len(runes) {
			needsFallback = !font.CanEncodeWinAnsiRune(runes[i])
		}
		// Emit a run at boundaries or at end of string.
		if i == len(runes) || needsFallback != startNeedsFallback {
			seg := string(runes[start:i])
			if startNeedsFallback {
				runs = append(runs, c.makeTextRun(seg, nil, fb, style))
			} else {
				runs = append(runs, c.makeTextRun(seg, stdFont, nil, style))
			}
			start = i
			startNeedsFallback = needsFallback
		}
	}

	return runs
}

// makeTextRun creates a TextRun with all styling fields from the computed style.
func (c *converter) makeTextRun(text string, std *font.Standard, emb *font.EmbeddedFont, style computedStyle) layout.TextRun {
	return layout.TextRun{
		Text:            text,
		Font:            std,
		Embedded:        emb,
		FontSize:        style.FontSize,
		Color:           style.Color,
		Decoration:      style.TextDecoration,
		DecorationColor: style.TextDecorationColor,
		DecorationStyle: style.TextDecorationStyle,
		LetterSpacing:   style.LetterSpacing,
		WordSpacing:     style.WordSpacing,
		BaselineShift:   baselineShiftFromStyle(style),
		TextShadow:      textShadowFromStyle(style),
		BackgroundColor: style.BackgroundColor,
	}
}

// walkChildren processes all child nodes and collects layout elements.
// It applies CSS margin collapsing between adjacent block-level elements:
// when one element's margin-bottom is followed by the next element's margin-top,
// the margins collapse to the larger of the two instead of summing.
//
// It also implements CSS 2.1 §9.2.1.1 anonymous block boxes: when a block
// container has mixed inline and block children, any run of consecutive
// inline content (text nodes and inline elements like <strong>, <em>,
// <span>, <a>) is wrapped into a single anonymous paragraph rather than
// being split into one paragraph per sibling node. Without this grouping,
// "We're pleased to offer <strong>Acme</strong>. Please..." would render
// as three paragraphs on three lines with the period orphaned at the start
// of line 3, instead of one wrapped paragraph with "Acme" bold inline.
func (c *converter) walkChildren(n *html.Node, parentStyle computedStyle) []layout.Element {
	var elems []layout.Element
	var prevMarginBottom float64
	var inlineBuf []*html.Node

	appendBlock := func(e layout.Element) {
		prevMarginBottom = collapseMargins(prevMarginBottom, e)
		elems = append(elems, e)
	}

	flushInline := func() {
		if len(inlineBuf) == 0 {
			return
		}
		var runs []layout.TextRun
		for _, node := range inlineBuf {
			runs = append(runs, c.collectRunsFromNode(node, parentStyle)...)
		}
		inlineBuf = inlineBuf[:0]
		if len(runs) == 0 {
			return
		}
		for _, group := range splitRunsAtBr(runs) {
			if len(group) == 0 {
				continue
			}
			p := c.buildParagraphFromRuns(group, parentStyle)
			appendBlock(p)
		}
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if c.limitErr != nil || c.ctxErr != nil {
			break
		}
		if c.isInlineFlowChild(child, parentStyle) {
			inlineBuf = append(inlineBuf, child)
			continue
		}
		flushInline()
		for _, e := range c.convertNode(child, parentStyle) {
			appendBlock(e)
		}
	}
	flushInline()
	return elems
}

// isInlineFlowChild reports whether a child node, when encountered inside
// a block container, should participate in inline flow (and therefore be
// grouped with its inline siblings into an anonymous block box) rather
// than be converted as a standalone block element.
//
// Text nodes are always inline. Whitespace-only text nodes between block
// siblings are deliberately NOT inline — they would cause spurious
// anonymous paragraphs containing nothing but a space between, say, two
// <div>s. Known text-level inline HTML tags (<span>, <strong>, <em>,
// <a>, etc.) are inline unless their computed style overrides display
// to block, flex, grid, or none.
//
// Replaced inline elements (<img>, <svg>) and form controls (<input>,
// <button>, <select>, <textarea>, <label>), and <br>, are intentionally
// NOT in the list. <img>/<svg> need standalone block handling for the
// top-level case (a bare <svg> as the whole document must become an
// SVGElement, not a paragraph wrapping an SVGElement) and mixing them
// inline with text is a pre-existing limitation — not worse than main.
// Form controls need their own element-level conversion (convertInput /
// convertButton / etc.) which collectRunsFromNode does not handle, so
// grouping them as inline flow would silently drop them. <br> between
// two blocks is historically emitted as a standalone spacer paragraph —
// buffering it as inline produces no output because splitRunsAtBr
// splits its lone "\n" into two empty groups. Mixing <br> inside a
// real inline run (e.g. "line1<br>line2" inside a <div>) still works
// correctly via the buffered text on either side.
func (c *converter) isInlineFlowChild(child *html.Node, parentStyle computedStyle) bool {
	switch child.Type {
	case html.TextNode:
		// Whitespace-only text between block siblings must not be
		// promoted to an anonymous paragraph.
		if strings.TrimSpace(child.Data) == "" {
			return false
		}
		return true
	case html.ElementNode:
		switch child.DataAtom {
		case atom.Span, atom.Em, atom.Strong, atom.B, atom.I, atom.U, atom.S,
			atom.Del, atom.Mark, atom.Small, atom.Sub, atom.Sup, atom.Code,
			atom.A:
			// Honor CSS display overrides — a <span style="display:block">
			// should still be treated as a block.
			style := c.computeElementStyle(child, parentStyle)
			if style.Display == "block" || style.Display == "flex" ||
				style.Display == "grid" || style.Display == "none" {
				return false
			}
			return true
		}
		return false
	}
	return false
}

// collapseMargins implements adjacent-sibling margin collapsing for
// block-level layout elements. Given the previous element's SpaceAfter,
// it reduces the next element's SpaceBefore so the gap between them is
// max(prevAfter, nextBefore) instead of their sum, then returns the
// SpaceAfter of e for use as prevAfter in the next iteration.
func collapseMargins(prevAfter float64, e layout.Element) float64 {
	if prevAfter > 0 {
		if sb, ok := e.(interface{ GetSpaceBefore() float64 }); ok {
			before := sb.GetSpaceBefore()
			if before > 0 {
				collapsed := math.Max(prevAfter, before)
				reduction := prevAfter + before - collapsed
				if reduction > 0 {
					if setter, ok2 := e.(interface{ SetSpaceBefore(float64) }); ok2 {
						setter.SetSpaceBefore(before - reduction)
					}
				}
			}
		}
	}
	if sa, ok := e.(interface{ GetSpaceAfter() float64 }); ok {
		return sa.GetSpaceAfter()
	}
	return 0
}

// convertNode converts a single HTML node into zero or more layout elements.
func (c *converter) convertNode(n *html.Node, parentStyle computedStyle) []layout.Element {
	// Boundary guards. convertNode is the single chokepoint every element
	// node flows through (walkChildren and the flex/grid/table child loops
	// all call it), so checking here bounds every conversion path. Once
	// either ctxErr or limitErr is set the walk unwinds without further work.
	if c.ctxErr != nil || c.limitErr != nil {
		return nil
	}
	// Cancellation check at this element boundary.
	if c.ctx != nil {
		if err := c.ctx.Err(); err != nil {
			c.ctxErr = err
			return nil
		}
	}
	// Resource guards (Options.MaxElements / MaxDepth).
	c.nodeCount++
	if c.opts.MaxElements > 0 && c.nodeCount > c.opts.MaxElements {
		c.limitErr = &LimitError{Kind: LimitElements, Limit: c.opts.MaxElements}
		return nil
	}
	c.depth++
	defer func() { c.depth-- }()
	if c.opts.MaxDepth > 0 && c.depth > c.opts.MaxDepth {
		c.limitErr = &LimitError{Kind: LimitDepth, Limit: c.opts.MaxDepth}
		return nil
	}

	switch n.Type {
	case html.TextNode:
		return c.convertText(n, parentStyle)
	case html.ElementNode:
		return c.convertElement(n, parentStyle)
	case html.DocumentNode:
		return c.walkChildren(n, parentStyle)
	default:
		return nil
	}
}

// convertElement dispatches on element tag.
func (c *converter) convertElement(n *html.Node, parentStyle computedStyle) []layout.Element {
	style := c.computeElementStyle(n, parentStyle)

	if style.Display == "none" {
		return nil
	}

	// Handle visibility: hidden — render as invisible (preserves space).
	if style.Visibility == "hidden" || style.Visibility == "collapse" {
		style.Opacity = 0.001 // nearly transparent — preserves layout space
		style.Color = layout.ColorWhite
		style.BackgroundColor = nil
		style.BorderTopWidth = 0
		style.BorderRightWidth = 0
		style.BorderBottomWidth = 0
		style.BorderLeftWidth = 0
	}

	// Apply CSS counter-reset: push new counter values onto the stack.
	for _, cr := range style.CounterReset {
		c.resetCounter(cr.Name, cr.Value)
	}
	// Apply CSS counter-increment: add to the innermost counter.
	for _, ci := range style.CounterIncrement {
		c.incrementCounter(ci.Name, ci.Value)
	}

	// Apply box-sizing: border-box adjustment.
	// CSS border-box means the declared width/height include padding and border.
	// Our layout Div treats widthUnit as the OUTER width (it subtracts padding
	// internally), so we only subtract border widths here — padding is handled
	// by the Div's own layout logic.
	if style.BoxSizing == "border-box" {
		if style.Width != nil {
			adjusted := *style.Width
			pts := adjusted.toPoints(0, style.FontSize)
			sub := style.BorderLeftWidth + style.BorderRightWidth
			if sub > 0 && pts-sub > 0 {
				adjusted = cssLength{Value: pts - sub, Unit: "pt"}
				style.Width = &adjusted
			}
		}
		if style.Height != nil {
			adjusted := *style.Height
			pts := adjusted.toPoints(0, style.FontSize)
			sub := style.BorderTopWidth + style.BorderBottomWidth
			if sub > 0 && pts-sub > 0 {
				adjusted = cssLength{Value: pts - sub, Unit: "pt"}
				style.Height = &adjusted
			}
		}
	}

	// Page break before.
	var before []layout.Element
	if style.PageBreakBefore == "always" {
		before = append(before, layout.NewAreaBreak())
	}

	// If this element establishes a containing block (position: relative,
	// absolute, or fixed), push it onto the positioned ancestor stack so
	// that descendant absolute elements resolve against it.
	isContainingBlock := style.Position == "relative" || style.Position == "absolute" || style.Position == "fixed"
	if isContainingBlock {
		cbWidth := c.containerWidth
		if style.Width != nil {
			if w := style.Width.toPoints(c.containerWidth, style.FontSize); w > 0 {
				cbWidth = w
			}
		}
		cbHeight := 0.0
		if style.Height != nil {
			cbHeight = style.Height.toPoints(c.opts.PageHeight, style.FontSize)
		}
		c.positionedAncestors = append(c.positionedAncestors, containingBlock{
			width:  cbWidth,
			height: cbHeight,
		})
	}

	elems := c.convertElementInner(n, style)

	// Apply CSS bookmark-level on non-heading elements. Headings carry
	// their own bookmark metadata via convertHeading → layout.Heading;
	// for other elements we wrap the produced Element so its first
	// PlacedBlock records the outline target. Skip the wrap when no
	// elements were produced or when the level is non-positive (0 is a
	// no-op, -1 / "none" is meaningful only on a heading where it
	// suppresses the default outline entry).
	if style.BookmarkLevelSet && style.BookmarkLevel >= 1 && !isHeadingNode(n) && len(elems) > 0 {
		text := collectText(n)
		label := resolveBookmarkLabel(style.BookmarkLabel, n, text)
		if label != "" {
			closed := style.BookmarkState == "closed"
			elems[0] = layout.NewBookmarkAnchor(elems[0], style.BookmarkLevel, label, closed)
		}
	}

	// ::before pseudo-element.
	if c.sheet != nil {
		beforeDecls := c.sheet.matchingPseudoElementDeclarations(n, "before")
		if text := c.parsePseudoContent(beforeDecls); text != "" {
			elem := c.generatePseudoElement(text, style)
			elems = append([]layout.Element{elem}, elems...)
		}
	}

	// ::after pseudo-element.
	if c.sheet != nil {
		afterDecls := c.sheet.matchingPseudoElementDeclarations(n, "after")
		if text := c.parsePseudoContent(afterDecls); text != "" {
			elem := c.generatePseudoElement(text, style)
			elems = append(elems, elem)
		}
	}

	// Pop the containing block and collect pending overlays.
	var pendingOverlays []pendingOverlay
	if isContainingBlock {
		top := c.positionedAncestors[len(c.positionedAncestors)-1]
		pendingOverlays = top.pending
		c.positionedAncestors = c.positionedAncestors[:len(c.positionedAncestors)-1]
	}

	// Wrap in float if CSS float is set.
	if style.Float == "left" || style.Float == "right" {
		side := layout.FloatLeft
		if style.Float == "right" {
			side = layout.FloatRight
		}
		var floated []layout.Element
		for _, e := range elems {
			floated = append(floated, layout.NewFloat(side, e))
		}
		elems = floated
	}

	// Handle position:absolute/fixed — remove from normal flow.
	if style.Position == "absolute" || style.Position == "fixed" {
		// Determine the containing block for resolving offsets.
		cbWidth := c.opts.PageWidth
		cbHeight := c.opts.PageHeight
		hasContainingBlock := len(c.positionedAncestors) > 0 && style.Position == "absolute"
		if hasContainingBlock {
			cb := &c.positionedAncestors[len(c.positionedAncestors)-1]
			cbWidth = cb.width
			if cb.height > 0 {
				cbHeight = cb.height
			}
		}

		for _, e := range elems {
			if hasContainingBlock {
				// Add as overlay on the nearest positioned ancestor.
				ov := pendingOverlay{elem: e, zIndex: style.ZIndex}
				if style.Left != nil {
					ov.x = style.Left.toPoints(cbWidth, style.FontSize)
				} else if style.Right != nil {
					ov.x = style.Right.toPoints(cbWidth, style.FontSize)
					ov.rightAligned = true
				}
				if style.Top != nil {
					ov.y = style.Top.toPoints(cbHeight, style.FontSize)
				} else if style.Bottom != nil {
					// CSS bottom in containing block: offset from the bottom edge.
					bottomVal := style.Bottom.toPoints(cbHeight, style.FontSize)
					if cbHeight > 0 {
						ov.y = cbHeight - bottomVal
					}
				}
				if style.Width != nil {
					ov.width = style.Width.toPoints(cbWidth, style.FontSize)
				}
				cb := &c.positionedAncestors[len(c.positionedAncestors)-1]
				cb.pending = append(cb.pending, ov)
			} else {
				// No positioned ancestor — fall back to page-level absolute.
				item := AbsoluteItem{
					Element: e,
					Fixed:   style.Position == "fixed",
				}
				if style.Left != nil {
					item.X = style.Left.toPoints(cbWidth, style.FontSize)
				} else if style.Right != nil {
					item.X = style.Right.toPoints(cbWidth, style.FontSize)
					item.RightAligned = true
				}
				if style.Top != nil {
					// CSS top → PDF y: page_height - top
					item.Y = cbHeight - style.Top.toPoints(cbHeight, style.FontSize)
				} else if style.Bottom != nil {
					item.Y = style.Bottom.toPoints(cbHeight, style.FontSize)
				}
				if style.Width != nil {
					item.Width = style.Width.toPoints(cbWidth, style.FontSize)
				}
				item.ZIndex = style.ZIndex
				c.absolutes = append(c.absolutes, item)
			}
		}
		// Attach any overlays from descendants of this absolute element
		// to the result elements (there are none to attach since we
		// return nil, but we still need to handle them if they were
		// collected). In practice, absolute children of absolute elements
		// are handled because the absolute element pushed/popped its own
		// containing block above.

		// Pop any counters that were reset by this element.
		for _, cr := range style.CounterReset {
			c.popCounter(cr.Name)
		}
		return nil // don't add to normal flow
	}

	// Attach pending overlay children (absolute descendants) to the
	// element's Div. If the element produced a single Div, attach
	// directly; otherwise wrap in a new Div to serve as the container.
	if len(pendingOverlays) > 0 {
		var targetDiv *layout.Div
		if len(elems) == 1 {
			targetDiv, _ = elems[0].(*layout.Div)
		}
		if targetDiv == nil {
			// Wrap in a new Div to serve as the containing block.
			targetDiv = layout.NewDiv()
			for _, e := range elems {
				targetDiv.Add(e)
			}
			elems = []layout.Element{targetDiv}
		}
		for _, ov := range pendingOverlays {
			targetDiv.AddOverlay(ov.elem, ov.x, ov.y, ov.width, ov.rightAligned, ov.zIndex)
		}
	}

	// Handle position:relative — offset visually without affecting flow.
	if style.Position == "relative" && (style.Top != nil || style.Left != nil || style.Right != nil || style.Bottom != nil) {
		dx := 0.0
		dy := 0.0
		if style.Left != nil {
			dx = style.Left.toPoints(c.containerWidth, style.FontSize)
		} else if style.Right != nil {
			dx = -style.Right.toPoints(c.containerWidth, style.FontSize)
		}
		// Per CSS, top/bottom percentages on a relatively positioned box
		// resolve against the height of its containing block. We don't track
		// the containing block height through normal flow here, so we
		// approximate with the page height (the nearest available basis),
		// mirroring how left/right use c.containerWidth. Absolute lengths
		// (px/pt/em) are unaffected since they ignore the percentage basis.
		if style.Top != nil {
			dy = style.Top.toPoints(c.opts.PageHeight, style.FontSize)
		} else if style.Bottom != nil {
			dy = -style.Bottom.toPoints(c.opts.PageHeight, style.FontSize)
		}
		if dx != 0 || dy != 0 {
			var result []layout.Element
			for _, e := range elems {
				div := layout.NewDiv()
				div.Add(e)
				div.SetRelativeOffset(dx, dy)
				result = append(result, div)
			}
			elems = result
		}
	}

	// Page break after.
	if style.PageBreakAfter == "always" {
		elems = append(elems, layout.NewAreaBreak())
	}

	// Pop any counters that were reset by this element (restore nesting).
	for _, cr := range style.CounterReset {
		c.popCounter(cr.Name)
	}

	if len(before) > 0 {
		elems = append(before, elems...)
	}
	return elems
}

// convertElementInner handles the actual element dispatch after page break handling.
func (c *converter) convertElementInner(n *html.Node, style computedStyle) []layout.Element {
	// Flex containers.
	if style.Display == "flex" {
		return c.convertFlex(n, style)
	}

	// Grid containers.
	if style.Display == "grid" {
		return c.convertGrid(n, style)
	}

	// CSS table layout: elements with display:table are rendered as tables.
	if style.Display == "table" {
		return c.convertCSSTable(n, style)
	}

	// Replaced elements (images, SVGs) must use their specialized converters
	// regardless of display value. CSS display on a replaced element affects
	// layout participation, not how the media itself is rendered. Without
	// this early dispatch, display:inline-block SVG/IMG would enter
	// convertBlock and produce an empty container instead of actual media.
	// (In paragraph-level inline flow, collectRuns handles these elements
	// via convertInlineElement before the display:inline-block branch.)
	switch n.DataAtom {
	case atom.Img:
		return c.convertImage(n, style)
	case atom.Svg:
		return c.convertSVG(n, style)
	}

	// Inline-block: renders as a block (Div) but participates in inline flow.
	// When inline-block elements appear inside a paragraph, collectRuns
	// handles them as inline element runs. At the top level (here), they
	// still render as blocks since there is no inline flow context.
	if style.Display == "inline-block" {
		return c.convertBlock(n, style)
	}

	switch n.DataAtom {
	case atom.H1:
		return c.convertHeading(n, style, layout.H1)
	case atom.H2:
		return c.convertHeading(n, style, layout.H2)
	case atom.H3:
		return c.convertHeading(n, style, layout.H3)
	case atom.H4:
		return c.convertHeading(n, style, layout.H4)
	case atom.H5:
		return c.convertHeading(n, style, layout.H5)
	case atom.H6:
		return c.convertHeading(n, style, layout.H6)
	case atom.P:
		return c.convertParagraph(n, style)
	case atom.Br:
		return c.convertBr(style)
	case atom.Hr:
		return c.convertHr(style)
	case atom.Pre:
		return c.convertPre(n, style)
	case atom.Div, atom.Section, atom.Article, atom.Main, atom.Header,
		atom.Footer, atom.Nav, atom.Aside:
		return c.convertBlock(n, style)
	case atom.Blockquote:
		return c.convertBlockquote(n, style)
	case atom.Dl:
		return c.convertDefinitionList(n, style)
	case atom.Figure:
		return c.convertFigure(n, style)
	case atom.Span, atom.Em, atom.Strong, atom.B, atom.I, atom.U, atom.S,
		atom.Del, atom.Mark, atom.Small, atom.Sub, atom.Sup, atom.Code:
		// A default-inline element set to display:block that carries a
		// block-level box (a border-radius with a background or border) must
		// own a rounded fill, like display:inline-block already does. The
		// inline path (convertInlineContainer) only produces a bare Paragraph
		// that drops the radius and paints a square background, so route these
		// through convertBlock, which builds a Div via applyDivStyles and
		// clears the redundant child-paragraph background (issue #329). Spans
		// without such a box stay on the inline path (no regression).
		if style.Display == "block" && style.hasBorderRadius() &&
			(style.BackgroundColor != nil || style.hasBorder()) {
			return c.convertBlock(n, style)
		}
		return c.convertInlineContainer(n, style)
	case atom.Table:
		return c.convertTable(n, style)
	case atom.A:
		return c.convertLink(n, style)
	case atom.Ul:
		return c.convertList(n, style, false)
	case atom.Ol:
		return c.convertList(n, style, true)
	case atom.Input:
		return c.convertInput(n, style)
	case atom.Select:
		return c.convertSelect(n, style)
	case atom.Textarea:
		return c.convertTextarea(n, style)
	case atom.Button:
		return c.convertButton(n, style)
	case atom.Form:
		return c.convertBlock(n, style)
	case atom.Label:
		return c.convertInlineContainer(n, style)
	case atom.Fieldset:
		return c.convertFieldset(n, style)
	case atom.Html, atom.Head:
		return c.walkChildren(n, style)
	case atom.Body:
		// Body is a normal block element (per CSS spec).
		// Its padding/border/background are additive with @page margins.
		return c.convertBlock(n, style)
	case atom.Title:
		c.metadata.Title = textContent(n)
		return nil
	case atom.Meta:
		c.extractMeta(n)
		return nil
	case atom.Style, atom.Script, atom.Link:
		return nil // skip non-visual elements
	default:
		// Unknown element — treat as block container.
		return c.convertBlock(n, style)
	}
}
