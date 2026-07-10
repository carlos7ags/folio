// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
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
	// (for <img src="http://...">, background-image: url(), etc.), and
	// again for each redirect hop. Return nil to allow the fetch, or an
	// error to block it. Only consulted when AllowRemoteFetch is true.
	// If nil while AllowRemoteFetch is true, the built-in DenyInternalHosts
	// policy is used; set URLPolicy to a function that always returns nil
	// to allow every host.
	URLPolicy URLPolicy
	// AllowRemoteFetch enables fetching http(s) assets referenced by the
	// document (<img src="http://...">, background-image: url(http://...),
	// linked stylesheets, @font-face url(http://...)). Default false: no
	// network request is made and such references fail with
	// ErrRemoteFetchDisabled (logged, and surfaced under StrictAssets).
	// When true, every fetch is filtered by URLPolicy; if URLPolicy is nil,
	// the built-in DenyInternalHosts policy is applied, which blocks
	// loopback, RFC1918, link-local, and non-http(s) targets and is
	// re-checked on every redirect hop.
	AllowRemoteFetch bool
	// AllowAbsolutePaths enables reading absolute filesystem paths named in
	// document content (<img src="/etc/..."> or @font-face url('/abs/...'))
	// when BaseFS is nil. Default false: such references fail with
	// ErrAbsolutePathDenied. When true, the read is capped at the same byte
	// limit as the HTTP path. This does NOT affect Options.FallbackFontPath
	// or the built-in system-font search, which load programmatic absolute
	// paths through a separate trusted code path.
	AllowAbsolutePaths bool
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

	// MaxTotalAssetBytes caps the aggregate bytes read across every asset
	// loaded during one conversion (images, fonts, linked stylesheets —
	// remote, BaseFS-relative, and absolute alike). 0 selects a generous
	// default (512 MiB) that comfortably fits realistic documents with
	// several embedded fonts and images while bounding the worst case,
	// where many references at the per-asset limit would otherwise
	// multiply into unbounded memory. Once the running total is crossed,
	// the offending and any later asset loads fail with
	// ErrAssetBudgetExceeded (logged, and surfaced under StrictAssets).
	// Set a large value to effectively disable, or a small one to tightly
	// bound untrusted input.
	MaxTotalAssetBytes int64
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
	out.AllowRemoteFetch = o.AllowRemoteFetch
	out.AllowAbsolutePaths = o.AllowAbsolutePaths
	out.Client = o.Client
	out.Logger = o.Logger
	out.StrictAssets = o.StrictAssets
	out.MaxElements = o.MaxElements
	out.MaxDepth = o.MaxDepth
	out.MaxTotalAssetBytes = o.MaxTotalAssetBytes
	return out
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
	budget := newAssetBudget(o.MaxTotalAssetBytes)
	ss := parseStyleBlocks(doc, o, budget, logStylesheetErr)

	c := &converter{opts: o, logger: logger, rootFontSize: o.DefaultFontSize, sheet: ss, embeddedFonts: make(map[string]*font.EmbeddedFont), containerWidth: o.PageWidth, counters: make(map[string][]int), urlPolicy: o.URLPolicy, budget: budget, strictErrs: stylesheetErrs, ctx: ctx}

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
	budget := newAssetBudget(o.MaxTotalAssetBytes)
	ss := parseStyleBlocks(doc, o, budget, logStylesheetErr)

	c := &converter{opts: o, logger: logger, rootFontSize: o.DefaultFontSize, sheet: ss, embeddedFonts: make(map[string]*font.EmbeddedFont), containerWidth: o.PageWidth, counters: make(map[string][]int), urlPolicy: o.URLPolicy, budget: budget, strictErrs: stylesheetErrs, ctx: ctx}

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

	// budget bounds the aggregate asset bytes read across the conversion.
	// Shared with the pre-converter stylesheet loads so every path counts
	// against one total. Nil in unit tests that build a converter directly.
	budget *assetBudget

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
