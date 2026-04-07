// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"path/filepath"
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
	// BasePath is the base directory for resolving relative image/font/CSS paths.
	BasePath string
	// PageWidth is the page width in points (default 612 = US Letter).
	PageWidth float64
	// PageHeight is the page height in points (default 792 = US Letter).
	PageHeight float64
	// FallbackFontPath is the path to a Unicode-capable TTF/OTF font used
	// when text contains characters outside WinAnsiEncoding (e.g. CJK, emoji).
	// If empty, the converter searches common system font locations.
	FallbackFontPath string
	// URLPolicy is called before the converter fetches a remote URL
	// (for <img src="http://...">, background-image: url(), etc.).
	// Return nil to allow the fetch, or an error to block it.
	// If nil, all URLs are allowed.
	URLPolicy URLPolicy
}

// URLPolicy controls whether the HTML converter may fetch a remote URL.
// It is called with the URL string before each HTTP request. Return nil
// to allow the fetch, or an error to block it and prevent the request.
type URLPolicy func(url string) error

// defaults returns a copy of Options with zero-value fields replaced by sensible defaults.
func (o *Options) defaults() Options {
	out := Options{DefaultFontSize: 12, PageWidth: 612, PageHeight: 792}
	if o != nil {
		if o.DefaultFontSize > 0 {
			out.DefaultFontSize = o.DefaultFontSize
		}
		out.BasePath = o.BasePath
		if o.PageWidth > 0 {
			out.PageWidth = o.PageWidth
		}
		if o.PageHeight > 0 {
			out.PageHeight = o.PageHeight
		}
		if o.URLPolicy != nil {
			out.URLPolicy = o.URLPolicy
		}
	}
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
}

// DocMetadata holds document metadata extracted from HTML head elements.
type DocMetadata struct {
	Title       string // from <title>
	Author      string // from <meta name="author">
	Description string // from <meta name="description">
	Keywords    string // from <meta name="keywords">
	Creator     string // from <meta name="generator">
	Subject     string // from <meta name="subject">
}

// MarginBoxContent holds the parsed content of a CSS margin box (e.g. @top-center).
type MarginBoxContent struct {
	Content  string     // resolved content string (after evaluating counter(), string literals, etc.)
	FontSize float64    // font size in points (0 = use default 9pt)
	Color    [3]float64 // RGB color (0-1 each; all zero = default gray)
}

// PageMargins holds the margin values and margin-box content for a
// page variant (e.g. :first, :left, :right) parsed from a CSS @page rule.
type PageMargins struct {
	Top, Right, Bottom, Left float64
	HasMargins               bool                        // true if any margin property was explicitly set (even to 0)
	MarginBoxes              map[string]MarginBoxContent // e.g. "top-center" → content
}

// PageConfig holds page dimensions and margins from CSS @page rules.
type PageConfig struct {
	Width      float64 // page width in points (0 = use default)
	Height     float64 // page height in points (0 = use default)
	AutoHeight bool    // true when @page size has explicit height of 0 (size to content)
	Landscape  bool

	// Default margins (from @page with no pseudo-selector).
	MarginTop    float64
	MarginRight  float64
	MarginBottom float64
	MarginLeft   float64
	HasMargins   bool // true if any margin property was explicitly set (even to 0)

	// Per-page-type margin overrides (nil = use default).
	First *PageMargins // @page :first
	Left  *PageMargins // @page :left (even pages in LTR)
	Right *PageMargins // @page :right (odd pages in LTR)

	// Default margin boxes (from @page with no pseudo-selector).
	MarginBoxes map[string]MarginBoxContent // e.g. "top-center" → content
}

// convertMarginBoxes converts html.MarginBoxContent to layout.MarginBox.
func convertMarginBoxes(src map[string]MarginBoxContent) map[string]layout.MarginBox {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]layout.MarginBox, len(src))
	for name, mbc := range src {
		out[name] = layout.MarginBox{
			Content:  mbc.Content,
			FontSize: mbc.FontSize,
			Color:    mbc.Color,
		}
	}
	return out
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
// and absolutely positioned items.
func ConvertFull(htmlStr string, opts *Options) (*ConvertResult, error) {
	o := opts.defaults()
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, err
	}

	style := defaultStyle()
	style.FontSize = o.DefaultFontSize

	ss := parseStyleBlocks(doc, o.BasePath, makeCSSFetcher(o.URLPolicy))

	c := &converter{opts: o, rootFontSize: o.DefaultFontSize, sheet: ss, embeddedFonts: make(map[string]*font.EmbeddedFont), containerWidth: o.PageWidth, counters: make(map[string][]int), urlPolicy: o.URLPolicy}

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

	// Load @font-face fonts.
	for _, ff := range ss.fontFaces {
		path := ff.src
		if !filepath.IsAbs(path) && o.BasePath != "" {
			path = filepath.Join(o.BasePath, path)
		}
		face, err := font.LoadFont(path)
		if err != nil {
			continue
		}
		ef := font.NewEmbeddedFont(face)
		key := ff.family + "|" + ff.weight + "|" + ff.style
		c.embeddedFonts[key] = ef
	}

	elems := c.walkChildren(doc, style)
	result := &ConvertResult{Elements: elems, Absolutes: c.absolutes, Metadata: c.metadata}
	result.PageConfig = pageConfig

	// Build ready-to-use margin box maps so callers can pass them
	// directly to doc.SetMarginBoxes without type conversion.
	if pageConfig != nil {
		result.MarginBoxes = convertMarginBoxes(pageConfig.MarginBoxes)
		if pageConfig.First != nil {
			result.FirstMarginBoxes = convertMarginBoxes(pageConfig.First.MarginBoxes)
		}
	}

	return result, nil
}

// Convert parses an HTML string and returns a slice of layout elements
// suitable for passing to a layout.Renderer. Only a subset of HTML is
// supported — see package documentation for details.
func Convert(htmlStr string, opts *Options) ([]layout.Element, error) {
	o := opts.defaults()
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return nil, err
	}

	style := defaultStyle()
	style.FontSize = o.DefaultFontSize

	ss := parseStyleBlocks(doc, o.BasePath, makeCSSFetcher(o.URLPolicy))

	c := &converter{opts: o, rootFontSize: o.DefaultFontSize, sheet: ss, embeddedFonts: make(map[string]*font.EmbeddedFont), containerWidth: o.PageWidth, counters: make(map[string][]int), urlPolicy: o.URLPolicy}

	// Update containerWidth if @page specifies a different page size.
	if len(ss.pageRules) > 0 {
		if pc := parsePageConfig(ss.pageRules, o.DefaultFontSize); pc != nil && pc.Width > 0 {
			c.containerWidth = pc.Width
			c.opts.PageWidth = pc.Width
			c.opts.PageHeight = pc.Height
		}
	}

	// Load @font-face fonts.
	for _, ff := range ss.fontFaces {
		path := ff.src
		if !filepath.IsAbs(path) && o.BasePath != "" {
			path = filepath.Join(o.BasePath, path)
		}
		face, err := font.LoadFont(path)
		if err != nil {
			continue // silently skip unloadable fonts
		}
		ef := font.NewEmbeddedFont(face)
		key := ff.family + "|" + ff.weight + "|" + ff.style
		c.embeddedFonts[key] = ef
	}

	return c.walkChildren(doc, style), nil
}

type converter struct {
	opts           Options
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

// getFallbackFont returns a Unicode-capable embedded font for text that
// can't be encoded in WinAnsiEncoding. The font is loaded lazily on first
// use. Returns nil if no suitable font is found.
func (c *converter) getFallbackFont() *font.EmbeddedFont {
	if c.fallbackFontLoaded {
		return c.fallbackFont
	}
	c.fallbackFontLoaded = true

	// Try user-specified path first.
	if c.opts.FallbackFontPath != "" {
		if face, err := font.LoadFont(c.opts.FallbackFontPath); err == nil {
			c.fallbackFont = font.NewEmbeddedFont(face)
			return c.fallbackFont
		}
	}

	// Search common system font locations for a Unicode-capable font.
	candidates := []string{
		// macOS
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/System/Library/Fonts/Helvetica.ttc",
		// Linux — Noto Sans has excellent Unicode coverage
		"/usr/share/fonts/truetype/noto/NotoSans-Regular.ttf",
		"/usr/share/fonts/noto/NotoSans-Regular.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
		"/usr/share/fonts/dejavu/DejaVuSans.ttf",
		// Windows
		`C:\Windows\Fonts\arial.ttf`,
		`C:\Windows\Fonts\segoeui.ttf`,
	}
	for _, path := range candidates {
		if face, err := font.LoadFont(path); err == nil {
			c.fallbackFont = font.NewEmbeddedFont(face)
			return c.fallbackFont
		}
	}

	return nil
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

// walkChildren processes all child nodes and collects layout elements.
// It applies CSS margin collapsing between adjacent block-level elements:
// when one element's margin-bottom is followed by the next element's margin-top,
// the margins collapse to the larger of the two instead of summing.
func (c *converter) walkChildren(n *html.Node, parentStyle computedStyle) []layout.Element {
	var elems []layout.Element
	var prevMarginBottom float64
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		childElems := c.convertNode(child, parentStyle)
		for _, e := range childElems {
			prevMarginBottom = collapseMargins(prevMarginBottom, e)
			elems = append(elems, e)
		}
	}
	return elems
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
		if style.Top != nil {
			dy = style.Top.toPoints(0, style.FontSize)
		} else if style.Bottom != nil {
			dy = -style.Bottom.toPoints(0, style.FontSize)
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
