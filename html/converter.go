// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
	"github.com/carlos7ags/folio/layout"
	"github.com/carlos7ags/folio/svg"

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
}

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

	ss := parseStyleBlocks(doc, o.BasePath)

	c := &converter{opts: o, rootFontSize: o.DefaultFontSize, sheet: ss, embeddedFonts: make(map[string]*font.EmbeddedFont), containerWidth: o.PageWidth, counters: make(map[string][]int)}

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

	ss := parseStyleBlocks(doc, o.BasePath)

	c := &converter{opts: o, rootFontSize: o.DefaultFontSize, sheet: ss, embeddedFonts: make(map[string]*font.EmbeddedFont), containerWidth: o.PageWidth, counters: make(map[string][]int)}

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
			// Collapse margins: reduce this element's SpaceBefore if the
			// previous element's SpaceAfter overlaps.
			if prevMarginBottom > 0 {
				if sb, ok := e.(interface{ GetSpaceBefore() float64 }); ok {
					before := sb.GetSpaceBefore()
					if before > 0 {
						// Collapse: use max(prevBottom, thisBefore) instead of sum.
						collapsed := math.Max(prevMarginBottom, before)
						reduction := prevMarginBottom + before - collapsed
						if reduction > 0 {
							if setter, ok2 := e.(interface{ SetSpaceBefore(float64) }); ok2 {
								setter.SetSpaceBefore(before - reduction)
							}
						}
					}
				}
			}
			// Track this element's SpaceAfter for next iteration.
			prevMarginBottom = 0
			if sa, ok := e.(interface{ GetSpaceAfter() float64 }); ok {
				prevMarginBottom = sa.GetSpaceAfter()
			}
			elems = append(elems, e)
		}
	}
	return elems
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

// convertText handles bare text nodes.
func (c *converter) convertText(n *html.Node, style computedStyle) []layout.Element {
	text := processWhitespace(n.Data, style.WhiteSpace)
	if text == "" {
		return nil
	}
	text = applyTextTransform(text, style.TextTransform)
	stdFont, embFont := c.resolveFontForText(style, text)
	run := layout.TextRun{
		Text:            text,
		Font:            stdFont,
		Embedded:        embFont,
		FontSize:        style.FontSize,
		Color:           style.Color,
		Decoration:      style.TextDecoration,
		DecorationColor: style.TextDecorationColor,
		DecorationStyle: style.TextDecorationStyle,
		LetterSpacing:   style.LetterSpacing,
		WordSpacing:     style.WordSpacing,
		BaselineShift:   baselineShiftFromStyle(style),
	}
	p := layout.NewStyledParagraph(run)
	p.SetAlign(style.TextAlign)
	p.SetLeading(style.LineHeight)
	return []layout.Element{p}
}

// parseCounterEntries parses a counter-reset or counter-increment value.
// defaultVal is the default value when no integer follows a name (0 for reset, 1 for increment).
func parseCounterEntries(val string, defaultVal int) []counterEntry {
	parts := strings.Fields(val)
	var entries []counterEntry
	for i := 0; i < len(parts); i++ {
		name := parts[i]
		if name == "none" {
			return nil
		}
		value := defaultVal
		if i+1 < len(parts) {
			if v, err := strconv.Atoi(parts[i+1]); err == nil {
				value = v
				i++ // skip the number
			}
		}
		entries = append(entries, counterEntry{Name: name, Value: value})
	}
	return entries
}

// resetCounter pushes a new counter value onto the stack for the given name.
func (c *converter) resetCounter(name string, value int) {
	c.counters[name] = append(c.counters[name], value)
}

// popCounter removes the most recently pushed counter for the given name.
// Called when leaving an element that did counter-reset to restore nesting.
func (c *converter) popCounter(name string) {
	stack := c.counters[name]
	if len(stack) > 0 {
		c.counters[name] = stack[:len(stack)-1]
	}
}

// incrementCounter adds value to the innermost counter for the given name.
// If no counter exists, auto-instantiates one at the document root per CSS spec.
func (c *converter) incrementCounter(name string, value int) {
	stack := c.counters[name]
	if len(stack) == 0 {
		// Auto-instantiate at document root per CSS spec.
		c.counters[name] = []int{value}
		return
	}
	stack[len(stack)-1] += value
}

// getCounter returns the current (innermost) value of the named counter.
func (c *converter) getCounter(name string) int {
	stack := c.counters[name]
	if len(stack) == 0 {
		return 0
	}
	return stack[len(stack)-1]
}

// parsePseudoContent extracts the text from a CSS content property value.
// Supports quoted strings, counter(name), counters(name, separator), and
// concatenation of the above. Returns empty string for unsupported values.
func (c *converter) parsePseudoContent(decls []cssDecl) string {
	for _, d := range decls {
		if d.property == "content" {
			val := strings.TrimSpace(d.value)
			if val == "none" || val == "" {
				return ""
			}
			return c.resolveContentValue(val)
		}
	}
	return ""
}

// resolveContentValue parses a CSS content value, resolving quoted strings,
// counter() and counters() function calls.
func (c *converter) resolveContentValue(val string) string {
	var result strings.Builder
	remaining := val
	for len(remaining) > 0 {
		remaining = strings.TrimSpace(remaining)
		if len(remaining) == 0 {
			break
		}
		// Quoted string.
		if remaining[0] == '"' || remaining[0] == '\'' {
			quote := remaining[0]
			end := strings.IndexByte(remaining[1:], quote)
			if end >= 0 {
				result.WriteString(remaining[1 : end+1])
				remaining = remaining[end+2:]
				continue
			}
			// Malformed quote — treat rest as literal.
			result.WriteString(remaining[1:])
			break
		}
		// counters() function — must check before counter() to avoid prefix match.
		if strings.HasPrefix(remaining, "counters(") {
			closeIdx := strings.IndexByte(remaining, ')')
			if closeIdx >= 0 {
				inner := remaining[len("counters("):closeIdx]
				parts := strings.SplitN(inner, ",", 2)
				name := strings.TrimSpace(parts[0])
				sep := "."
				if len(parts) > 1 {
					sep = strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				}
				stack := c.counters[name]
				strs := make([]string, len(stack))
				for i, v := range stack {
					strs[i] = strconv.Itoa(v)
				}
				result.WriteString(strings.Join(strs, sep))
				remaining = remaining[closeIdx+1:]
				continue
			}
		}
		// counter() function.
		if strings.HasPrefix(remaining, "counter(") {
			closeIdx := strings.IndexByte(remaining, ')')
			if closeIdx >= 0 {
				name := strings.TrimSpace(remaining[len("counter("):closeIdx])
				result.WriteString(strconv.Itoa(c.getCounter(name)))
				remaining = remaining[closeIdx+1:]
				continue
			}
		}
		// Skip unknown token.
		spIdx := strings.IndexByte(remaining, ' ')
		if spIdx >= 0 {
			remaining = remaining[spIdx+1:]
		} else {
			break
		}
	}
	return result.String()
}

// generatePseudoElement creates a text element for ::before or ::after content.
func (c *converter) generatePseudoElement(text string, style computedStyle) layout.Element {
	stdFont, embFont := c.resolveFontPair(style)
	run := layout.TextRun{
		Text:            text,
		Font:            stdFont,
		Embedded:        embFont,
		FontSize:        style.FontSize,
		Color:           style.Color,
		Decoration:      style.TextDecoration,
		DecorationColor: style.TextDecorationColor,
		DecorationStyle: style.TextDecorationStyle,
		LetterSpacing:   style.LetterSpacing,
		WordSpacing:     style.WordSpacing,
		BaselineShift:   baselineShiftFromStyle(style),
	}
	p := layout.NewStyledParagraph(run)
	p.SetAlign(style.TextAlign)
	p.SetLeading(style.LineHeight)
	return p
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

	// Inline-block: renders as a block (Div) but participates in inline flow.
	// For PDF purposes, treat as a block with box-model support.
	// Special elements (SVG, IMG) fall through to their specific handlers below.
	if style.Display == "inline-block" && n.DataAtom != atom.Svg && n.DataAtom != atom.Img {
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
	case atom.Img:
		return c.convertImage(n, style)
	case atom.Svg:
		return c.convertSVG(n, style)
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

// convertHeading creates a layout.Heading from an <h1>-<h6> element.
func (c *converter) convertHeading(n *html.Node, style computedStyle, level layout.HeadingLevel) []layout.Element {
	text := collectText(n)
	if text == "" {
		return nil
	}
	text = applyTextTransform(text, style.TextTransform)

	stdFont, embFont := c.resolveFontPair(style)
	run := layout.TextRun{
		Text:            text,
		Font:            stdFont,
		Embedded:        embFont,
		FontSize:        style.FontSize,
		Color:           style.Color,
		Decoration:      style.TextDecoration,
		DecorationColor: style.TextDecorationColor,
		DecorationStyle: style.TextDecorationStyle,
		LetterSpacing:   style.LetterSpacing,
		WordSpacing:     style.WordSpacing,
		BaselineShift:   baselineShiftFromStyle(style),
	}
	var h *layout.Heading
	if embFont != nil {
		h = layout.NewHeadingEmbedded(text, level, embFont)
	} else {
		h = layout.NewHeadingWithFont(text, level, stdFont, style.FontSize)
	}
	// Replace the default run with the fully styled one.
	h.SetRuns([]layout.TextRun{run})
	h.SetAlign(style.TextAlign)

	// Wrap in a Div if the heading has box-model properties.
	needsWrapper := style.hasBorder() || style.hasPadding() || style.hasMargin() ||
		style.BackgroundColor != nil || style.Width != nil || style.MaxWidth != nil
	if needsWrapper {
		div := layout.NewDiv()
		div.Add(h)
		applyDivStyles(div, style, c.containerWidth)
		return []layout.Element{div}
	}

	return []layout.Element{h}
}

// convertParagraph creates a layout.Paragraph from a <p> element.
func (c *converter) convertParagraph(n *html.Node, style computedStyle) []layout.Element {
	runs := c.collectRuns(n, style)
	if len(runs) == 0 {
		return nil
	}

	// Split runs at <br> markers (TextRun with "\n") into line groups.
	groups := splitRunsAtBr(runs)

	var elems []layout.Element
	for i, group := range groups {
		if len(group) == 0 {
			continue
		}
		p := c.buildParagraphFromRuns(group, style)
		// Only apply top margin to first paragraph, bottom margin to last.
		if i == 0 && style.MarginTop > 0 {
			p.SetSpaceBefore(style.MarginTop)
		}
		if i == len(groups)-1 && style.MarginBottom > 0 {
			p.SetSpaceAfter(style.MarginBottom)
		}
		elems = append(elems, p)
	}

	// Wrap in a Div if the paragraph has box-model properties.
	needsWrapper := style.hasBorder() || style.hasPadding() || style.BackgroundColor != nil ||
		style.Width != nil || style.MaxWidth != nil
	if needsWrapper {
		div := layout.NewDiv()
		for _, e := range elems {
			div.Add(e)
		}
		applyDivStyles(div, style, c.containerWidth)
		return []layout.Element{div}
	}

	return elems
}

// splitRunsAtBr splits a flat slice of TextRuns into groups separated by
// newline markers (from <br> tags). Each group becomes a separate paragraph.
func splitRunsAtBr(runs []layout.TextRun) [][]layout.TextRun {
	var groups [][]layout.TextRun
	var current []layout.TextRun
	for _, r := range runs {
		if r.Text == "\n" && r.Font == nil && r.Embedded == nil {
			groups = append(groups, current)
			current = nil
			continue
		}
		current = append(current, r)
	}
	groups = append(groups, current)
	return groups
}

// buildParagraphFromRuns creates a styled paragraph from a slice of TextRuns.
func (c *converter) buildParagraphFromRuns(runs []layout.TextRun, style computedStyle) *layout.Paragraph {
	var p *layout.Paragraph
	if len(runs) == 1 && runs[0].Embedded == nil && runs[0].Color == (layout.Color{}) {
		// Simple case: single run, standard font, default color.
		p = layout.NewParagraph(runs[0].Text, runs[0].Font, runs[0].FontSize)
	} else {
		// Styled: multiple runs, embedded font, or custom color.
		p = layout.NewStyledParagraph(runs...)
	}

	p.SetAlign(style.TextAlign)
	p.SetLeading(style.LineHeight)
	if style.TextIndent != 0 {
		p.SetFirstLineIndent(style.TextIndent)
	}
	if style.BackgroundColor != nil {
		p.SetBackground(*style.BackgroundColor)
	}
	if style.TextOverflow == "ellipsis" && style.Overflow == "hidden" {
		p.SetEllipsis(true)
	}
	if style.WordBreak == "break-all" || style.WordBreak == "break-word" {
		p.SetWordBreak(style.WordBreak)
	}
	if style.Orphans > 0 {
		p.SetOrphans(style.Orphans)
	}
	if style.Widows > 0 {
		p.SetWidows(style.Widows)
	}
	switch style.Hyphens {
	case "auto":
		p.SetHyphens("auto")
	case "none":
		p.SetHyphens("none")
	}
	return p
}

// convertBr produces a small empty paragraph to create a line break.
func (c *converter) convertBr(style computedStyle) []layout.Element {
	stdFont, embFont := c.resolveFontPair(style)
	var p *layout.Paragraph
	if embFont != nil {
		p = layout.NewParagraphEmbedded(" ", embFont, style.FontSize)
	} else {
		p = layout.NewParagraph(" ", stdFont, style.FontSize)
	}
	p.SetLeading(style.LineHeight)
	return []layout.Element{p}
}

// convertHr creates a horizontal rule using layout.LineSeparator.
func (c *converter) convertHr(style computedStyle) []layout.Element {
	hr := layout.NewLineSeparator()
	hr.SetSpaceBefore(style.MarginTop)
	hr.SetSpaceAfter(style.MarginBottom)

	// Apply border color if set via CSS.
	if style.hasBorder() {
		hr.SetWidth(style.BorderTopWidth)
		hr.SetColor(style.BorderTopColor)
	}
	// Apply explicit color from CSS.
	if style.Color != (layout.Color{}) && style.Color != layout.ColorBlack {
		hr.SetColor(style.Color)
	}
	if style.BackgroundColor != nil {
		hr.SetColor(*style.BackgroundColor)
	}

	return []layout.Element{hr}
}

// convertPre handles <pre> elements, preserving whitespace and line breaks.
func (c *converter) convertPre(n *html.Node, style computedStyle) []layout.Element {
	raw := collectRawText(n)
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	f := resolveFont(style)
	lines := strings.Split(raw, "\n")

	// Strip leading/trailing empty lines.
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	div := layout.NewDiv()
	div.SetPadding(6)
	bg := layout.RGB(0.96, 0.96, 0.96)
	div.SetBackground(bg)

	for _, line := range lines {
		if line == "" {
			line = " " // preserve blank lines
		}
		// Replace tabs with spaces.
		line = strings.ReplaceAll(line, "\t", "    ")
		p := layout.NewParagraph(line, f, style.FontSize)
		p.SetLeading(1.4)
		div.Add(p)
	}

	return []layout.Element{div}
}

// convertImage handles <img> elements.
func (c *converter) convertImage(n *html.Node, style computedStyle) []layout.Element {
	src := getAttr(n, "src")
	alt := getAttr(n, "alt")

	if src == "" {
		if alt != "" {
			return c.altTextFallback(alt, style)
		}
		return nil
	}

	// Load image: data URI, HTTP URL, or local path.
	var img *folioimage.Image
	var err error

	if strings.HasPrefix(src, "data:") {
		img, err = decodeDataURI(src)
	} else if isURL(src) {
		img, err = fetchImage(src)
	} else {
		imgPath := src
		if !filepath.IsAbs(imgPath) && c.opts.BasePath != "" {
			imgPath = filepath.Join(c.opts.BasePath, imgPath)
		}
		img, err = loadImage(imgPath)
	}
	if err != nil {
		if alt != "" {
			return c.altTextFallback(alt, style)
		}
		return c.altTextFallback("[image: "+src+"]", style)
	}

	ie := layout.NewImageElement(img)

	// Parse width/height from attributes or CSS.
	w := parseAttrFloat(getAttr(n, "width"))
	h := parseAttrFloat(getAttr(n, "height"))
	if style.Width != nil {
		w = style.Width.toPoints(0, style.FontSize)
	}
	if style.Height != nil {
		h = style.Height.toPoints(0, style.FontSize)
	}
	if w > 0 || h > 0 {
		ie.SetSize(w, h)
	}

	return []layout.Element{ie}
}

// convertSVG handles inline <svg> elements.
func (c *converter) convertSVG(n *html.Node, style computedStyle) []layout.Element {
	// Serialize the <svg> HTML node back to markup so the SVG parser can read it.
	var buf bytes.Buffer
	if err := html.Render(&buf, n); err != nil {
		return nil
	}

	s, err := svg.Parse(buf.String())
	if err != nil {
		return nil // skip invalid SVG
	}

	el := layout.NewSVGElement(s)

	// Apply explicit size from CSS or SVG attributes.
	w := s.Width()
	h := s.Height()
	if style.Width != nil {
		w = style.Width.toPoints(0, style.FontSize)
	}
	if style.Height != nil {
		h = style.Height.toPoints(0, style.FontSize)
	}
	if w > 0 || h > 0 {
		el.SetSize(w, h)
	}

	return []layout.Element{el}
}

// altTextFallback returns a paragraph with alt text when an image can't be loaded.
func (c *converter) altTextFallback(alt string, style computedStyle) []layout.Element {
	stdFont, embFont := c.resolveFontPair(style)
	var p *layout.Paragraph
	if embFont != nil {
		p = layout.NewParagraphEmbedded(alt, embFont, style.FontSize)
	} else {
		p = layout.NewParagraph(alt, stdFont, style.FontSize)
	}
	return []layout.Element{p}
}

// decodeDataURI parses a data: URI and returns the image.
// Format: data:[<mediatype>][;base64],<data>
func decodeDataURI(uri string) (*folioimage.Image, error) {
	// Strip "data:" prefix.
	rest := strings.TrimPrefix(uri, "data:")

	// Split at comma: metadata,data
	commaIdx := strings.IndexByte(rest, ',')
	if commaIdx < 0 {
		return nil, fmt.Errorf("invalid data URI: no comma")
	}
	meta := rest[:commaIdx]
	encoded := rest[commaIdx+1:]

	// Decode data.
	var data []byte
	if strings.Contains(meta, ";base64") {
		var err error
		data, err = base64Decode(encoded)
		if err != nil {
			return nil, fmt.Errorf("data URI base64: %w", err)
		}
	} else {
		data = []byte(encoded)
	}

	// Detect format from media type.
	if strings.Contains(meta, "image/jpeg") || strings.Contains(meta, "image/jpg") {
		return folioimage.NewJPEG(data)
	}
	if strings.Contains(meta, "image/png") {
		return folioimage.NewPNG(data)
	}

	// Fallback: content sniffing.
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xD8 {
		return folioimage.NewJPEG(data)
	}
	if len(data) >= 4 && string(data[:4]) == "\x89PNG" {
		return folioimage.NewPNG(data)
	}
	if img, err := folioimage.NewJPEG(data); err == nil {
		return img, nil
	}
	return folioimage.NewPNG(data)
}

// base64Decode decodes standard base64.
func base64Decode(s string) ([]byte, error) {
	// Remove whitespace (common in data URIs).
	s = strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, s)

	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var lookup [256]byte
	for i := range lookup {
		lookup[i] = 0xFF
	}
	for i, c := range alphabet {
		lookup[c] = byte(i)
	}

	// Estimate output size.
	out := make([]byte, 0, len(s)*3/4)
	var buf uint32
	var bits int

	for _, c := range []byte(s) {
		if c == '=' {
			break
		}
		val := lookup[c]
		if val == 0xFF {
			continue // skip unknown chars
		}
		buf = buf<<6 | uint32(val)
		bits += 6
		if bits >= 8 {
			bits -= 8
			out = append(out, byte(buf>>bits))
			buf &= (1 << bits) - 1
		}
	}

	return out, nil
}

// isURL checks if a string is an HTTP(S) URL.
func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// fetchImage is implemented in fetch_image.go (with net/http)
// and fetch_image_wasm.go (stub for WASM builds).

// loadImage attempts to load an image file (JPEG, PNG, or TIFF).
func loadImage(path string) (*folioimage.Image, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return folioimage.LoadJPEG(path)
	case ".png":
		return folioimage.LoadPNG(path)
	case ".tif", ".tiff":
		return folioimage.LoadTIFF(path)
	default:
		// Try reading raw bytes and detecting format.
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		// Try JPEG first, then PNG.
		if img, err := folioimage.NewJPEG(data); err == nil {
			return img, nil
		}
		return folioimage.NewPNG(data)
	}
}

// baselineShiftFromStyle computes the vertical baseline offset for
// CSS vertical-align values like "super", "sub", "text-top", "text-bottom".
func baselineShiftFromStyle(style computedStyle) float64 {
	switch style.VerticalAlign {
	case "super":
		return style.FontSize * 0.35 // raise by ~35% of font size
	case "sub":
		return -style.FontSize * 0.2 // lower by ~20% of font size
	case "text-top":
		return style.FontSize * 0.25
	case "text-bottom":
		return -style.FontSize * 0.15
	default:
		return 0
	}
}

// cssLengthToUnitValue converts a cssLength to a layout.UnitValue.
// Percentage values are stored lazily (resolved at layout time).
// Absolute values are resolved immediately to points.
func cssLengthToUnitValue(l *cssLength, containerWidth, fontSize float64) layout.UnitValue {
	if l == nil {
		return layout.Pt(0)
	}
	if l.Unit == "%" {
		return layout.Pct(l.Value)
	}
	return layout.Pt(l.toPoints(containerWidth, fontSize))
}

// narrowContainerWidth saves the current containerWidth, narrows it based on
// the element's padding/border/width, and returns a restore function.
func (c *converter) narrowContainerWidth(style computedStyle) func() {
	prev := c.containerWidth
	if style.Width != nil {
		if w := style.Width.toPoints(c.containerWidth, style.FontSize); w > 0 {
			c.containerWidth = w
		}
	}
	if style.hasPadding() {
		c.containerWidth -= style.PaddingLeft + style.PaddingRight
	}
	if style.hasBorder() {
		c.containerWidth -= style.BorderLeftWidth + style.BorderRightWidth
	}
	if c.containerWidth < 0 {
		c.containerWidth = 0
	}
	return func() { c.containerWidth = prev }
}

// convertBlock wraps children in a Div container.
func (c *converter) convertBlock(n *html.Node, style computedStyle) []layout.Element {
	restore := c.narrowContainerWidth(style)
	children := c.walkChildren(n, style)
	restore()

	// Allow empty divs that have visual properties (height, background, border).
	hasVisualBox := style.Height != nil || style.BackgroundColor != nil ||
		style.hasBorder() || style.hasPadding()
	if len(children) == 0 && !hasVisualBox {
		return nil
	}

	// If column-count > 1, distribute children across columns.
	if style.ColumnCount > 1 && len(children) > 0 {
		return c.buildColumns(children, style)
	}

	// If no box-model properties, skip the Div wrapper.
	hasWidthConstraints := style.Width != nil || style.MaxWidth != nil || style.MinWidth != nil
	hasHeightConstraints := style.Height != nil || style.MinHeight != nil || style.MaxHeight != nil
	hasVisualEffects := style.BorderRadius > 0 || (style.Opacity > 0 && style.Opacity < 1) || style.Overflow == "hidden"
	hasBoxShadow := style.BoxShadow != nil
	hasOutline := style.OutlineWidth > 0
	hasTransform := style.Transform != "" && strings.ToLower(strings.TrimSpace(style.Transform)) != "none"
	hasBgImage := style.BackgroundImage != ""
	if !style.hasPadding() && !style.hasBorder() && !style.hasMargin() && style.BackgroundColor == nil && !hasWidthConstraints && !hasHeightConstraints && !hasVisualEffects && !hasBoxShadow && !hasOutline && !hasTransform && !hasBgImage {
		return children
	}

	div := layout.NewDiv()
	for _, child := range children {
		div.Add(child)
	}
	applyDivStyles(div, style, c.containerWidth)

	// Apply background image if set.
	if bgImg := c.resolveBackgroundImage(style); bgImg != nil {
		div.SetBackgroundImage(bgImg)
	}

	return []layout.Element{div}
}

// applyDivStyles applies common computed style properties to a layout.Div.
// containerWidth is the available width in points, used to resolve percentage values.
func applyDivStyles(div *layout.Div, style computedStyle, containerWidth float64) {
	if style.hasPadding() {
		div.SetPaddingAll(layout.Padding{
			Top:    style.PaddingTop,
			Right:  style.PaddingRight,
			Bottom: style.PaddingBottom,
			Left:   style.PaddingLeft,
		})
	}
	if style.hasBorder() {
		div.SetBorders(buildCellBorders(style))
	}
	if style.MarginTop > 0 {
		div.SetSpaceBefore(style.MarginTop)
	}
	if style.MarginBottom > 0 {
		div.SetSpaceAfter(style.MarginBottom)
	}
	// Horizontal alignment via auto margins.
	if style.MarginLeftAuto && style.MarginRightAuto {
		div.SetHCenter(true)
	} else if style.MarginLeftAuto && !style.MarginRightAuto {
		div.SetHRight(true)
	}
	if style.BackgroundColor != nil {
		div.SetBackground(*style.BackgroundColor)
	}
	if style.Width != nil {
		div.SetWidthUnit(cssLengthToUnitValue(style.Width, containerWidth, style.FontSize))
	}
	if style.MaxWidth != nil {
		div.SetMaxWidthUnit(cssLengthToUnitValue(style.MaxWidth, containerWidth, style.FontSize))
	}
	if style.MinWidth != nil {
		div.SetMinWidthUnit(cssLengthToUnitValue(style.MinWidth, containerWidth, style.FontSize))
	}
	if style.Height != nil {
		div.SetHeightUnit(cssLengthToUnitValue(style.Height, containerWidth, style.FontSize))
	}
	if style.MinHeight != nil {
		div.SetMinHeightUnit(cssLengthToUnitValue(style.MinHeight, containerWidth, style.FontSize))
	}
	if style.MaxHeight != nil {
		div.SetMaxHeightUnit(cssLengthToUnitValue(style.MaxHeight, containerWidth, style.FontSize))
	}
	if style.BorderRadius > 0 {
		div.SetBorderRadius(style.BorderRadius)
	}
	if style.Clear != "" && style.Clear != "none" {
		div.SetClear(style.Clear)
	}
	if style.Opacity > 0 && style.Opacity < 1 {
		div.SetOpacity(style.Opacity)
	}
	if style.Overflow == "hidden" {
		div.SetOverflow("hidden")
	}
	if style.BoxShadow != nil {
		div.SetBoxShadow(layout.BoxShadow{
			OffsetX: style.BoxShadow.OffsetX,
			OffsetY: style.BoxShadow.OffsetY,
			Blur:    style.BoxShadow.Blur,
			Spread:  style.BoxShadow.Spread,
			Color:   style.BoxShadow.Color,
		})
	}
	if style.OutlineWidth > 0 {
		div.SetOutline(style.OutlineWidth, style.OutlineStyle, style.OutlineColor, style.OutlineOffset)
	}
	if ops := parseTransform(style.Transform); len(ops) > 0 {
		div.SetTransform(ops)
		// Compute approximate element dimensions for transform-origin.
		// Use maxWidth/width hint if available; otherwise use a default.
		w := 0.0
		if style.Width != nil {
			w = style.Width.toPoints(containerWidth, style.FontSize)
		} else if style.MaxWidth != nil {
			w = style.MaxWidth.toPoints(containerWidth, style.FontSize)
		}
		h := 0.0
		if style.Height != nil {
			h = style.Height.toPoints(containerWidth, style.FontSize)
		} else if style.MinHeight != nil {
			h = style.MinHeight.toPoints(containerWidth, style.FontSize)
		}
		ox, oy := parseTransformOrigin(style.TransformOrigin, w, h, style.FontSize)
		div.SetTransformOrigin(ox, oy)
	}
}

// buildColumns creates a layout.Columns element from children and style.
func (c *converter) buildColumns(children []layout.Element, style computedStyle) []layout.Element {
	cols := layout.NewColumns(style.ColumnCount)
	if style.ColumnGap > 0 {
		cols.SetGap(style.ColumnGap)
	}

	// Distribute children round-robin across columns.
	for i, child := range children {
		cols.Add(i%style.ColumnCount, child)
	}

	return []layout.Element{cols}
}

// buildCellBorders creates layout.CellBorders from a computed style.
func buildCellBorders(style computedStyle) layout.CellBorders {
	return layout.CellBorders{
		Top:    buildBorder(style.BorderTopWidth, style.BorderTopStyle, style.BorderTopColor),
		Right:  buildBorder(style.BorderRightWidth, style.BorderRightStyle, style.BorderRightColor),
		Bottom: buildBorder(style.BorderBottomWidth, style.BorderBottomStyle, style.BorderBottomColor),
		Left:   buildBorder(style.BorderLeftWidth, style.BorderLeftStyle, style.BorderLeftColor),
	}
}

// buildBorder creates a single layout.Border from width, style, and color.
func buildBorder(width float64, style string, color layout.Color) layout.Border {
	if width <= 0 {
		return layout.Border{}
	}
	switch style {
	case "dashed":
		return layout.DashedBorder(width, color)
	case "dotted":
		return layout.DottedBorder(width, color)
	case "double":
		return layout.DoubleBorder(width, color)
	default:
		return layout.SolidBorder(width, color)
	}
}

// convertInlineContainer handles inline elements like <span>, <em>, <strong>.
// Collects text runs from children and wraps in a paragraph.
func (c *converter) convertInlineContainer(n *html.Node, style computedStyle) []layout.Element {
	runs := c.collectRuns(n, style)
	if len(runs) == 0 {
		return nil
	}
	p := layout.NewStyledParagraph(runs...)
	p.SetAlign(style.TextAlign)
	p.SetLeading(style.LineHeight)
	return []layout.Element{p}
}

// convertList handles <ul> and <ol> elements, including nested lists.
func (c *converter) convertList(n *html.Node, style computedStyle, ordered bool) []layout.Element {
	stdFont, embFont := c.resolveFontPair(style)
	var list *layout.List
	if embFont != nil {
		list = layout.NewListEmbedded(embFont, style.FontSize)
	} else {
		list = layout.NewList(stdFont, style.FontSize)
	}
	list.SetLeading(style.LineHeight)

	// Apply list-style-type from CSS, with fallback to ordered/unordered default.
	switch style.ListStyleType {
	case "disc", "":
		if ordered {
			list.SetStyle(layout.ListOrdered)
		} else {
			list.SetStyle(layout.ListUnordered)
		}
	case "circle", "square":
		list.SetStyle(layout.ListUnordered)
	case "decimal", "decimal-leading-zero":
		list.SetStyle(layout.ListOrdered)
	case "lower-roman":
		list.SetStyle(layout.ListOrderedRoman)
	case "upper-roman":
		list.SetStyle(layout.ListOrderedRomanUp)
	case "lower-alpha", "lower-latin":
		list.SetStyle(layout.ListOrderedAlpha)
	case "upper-alpha", "upper-latin":
		list.SetStyle(layout.ListOrderedAlphaUp)
	case "none":
		list.SetStyle(layout.ListNone)
	default:
		if ordered {
			list.SetStyle(layout.ListOrdered)
		}
	}

	c.populateList(n, list, style)

	return []layout.Element{list}
}

// populateList fills a list with items from <li> children, handling nesting.
func (c *converter) populateList(n *html.Node, list *layout.List, style computedStyle) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || child.DataAtom != atom.Li {
			continue
		}

		text := collectDirectText(child)
		nestedList := findNestedList(child)

		if nestedList != nil {
			if text == "" {
				text = " "
			}
			sub := list.AddItemWithSubList(text)
			if nestedList.DataAtom == atom.Ol {
				sub.SetStyle(layout.ListOrdered)
			}
			c.populateList(nestedList, sub, style)
		} else {
			if text != "" {
				list.AddItem(text)
			}
		}
	}
}

// convertBlockquote renders a <blockquote> as an indented block with a left border.
func (c *converter) convertBlockquote(n *html.Node, style computedStyle) []layout.Element {
	children := c.walkChildren(n, style)
	if len(children) == 0 {
		return nil
	}

	div := layout.NewDiv()
	for _, child := range children {
		div.Add(child)
	}

	// Left border: 3pt solid gray.
	gray := layout.RGB(0.6, 0.6, 0.6)
	div.SetBorders(layout.CellBorders{
		Left: layout.SolidBorder(3, gray),
	})
	div.SetPaddingAll(layout.Padding{
		Top:    3,
		Right:  6,
		Bottom: 3,
		Left:   15,
	})
	if style.MarginTop > 0 {
		div.SetSpaceBefore(style.MarginTop)
	}
	if style.MarginBottom > 0 {
		div.SetSpaceAfter(style.MarginBottom)
	}
	if style.BackgroundColor != nil {
		div.SetBackground(*style.BackgroundColor)
	}
	// Override with any explicit border/padding from CSS.
	if style.hasBorder() {
		div.SetBorders(buildCellBorders(style))
	}

	return []layout.Element{div}
}

// convertDefinitionList converts a <dl> element into a series of term/definition pairs.
func (c *converter) convertDefinitionList(n *html.Node, style computedStyle) []layout.Element {
	div := layout.NewDiv()
	if style.MarginTop > 0 {
		div.SetSpaceBefore(style.MarginTop)
	}
	if style.MarginBottom > 0 {
		div.SetSpaceAfter(style.MarginBottom)
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		childStyle := c.computeElementStyle(child, style)

		switch child.DataAtom {
		case atom.Dt:
			// Definition term: bold, no indent.
			text := collectText(child)
			if text == "" {
				continue
			}
			text = applyTextTransform(text, childStyle.TextTransform)
			f := resolveFont(childStyle)
			p := layout.NewParagraph(text, f, childStyle.FontSize)
			p.SetAlign(childStyle.TextAlign)
			p.SetLeading(childStyle.LineHeight)
			div.Add(p)

		case atom.Dd:
			// Definition description: indented.
			children := c.walkChildren(child, childStyle)
			if len(children) == 0 {
				continue
			}
			indent := layout.NewDiv()
			for _, ch := range children {
				indent.Add(ch)
			}
			indent.SetPaddingAll(layout.Padding{Left: childStyle.MarginLeft})
			div.Add(indent)

		default:
			// Process other children (e.g. nested <div>).
			elems := c.convertNode(child, style)
			for _, e := range elems {
				div.Add(e)
			}
		}
	}

	return []layout.Element{div}
}

// convertFigure converts a <figure> element, rendering <figcaption> as styled caption.
func (c *converter) convertFigure(n *html.Node, style computedStyle) []layout.Element {
	div := layout.NewDiv()
	if style.MarginTop > 0 {
		div.SetSpaceBefore(style.MarginTop)
	}
	if style.MarginBottom > 0 {
		div.SetSpaceAfter(style.MarginBottom)
	}
	if style.hasPadding() {
		div.SetPaddingAll(layout.Padding{
			Top:    style.PaddingTop,
			Right:  style.PaddingRight,
			Bottom: style.PaddingBottom,
			Left:   style.PaddingLeft,
		})
	}
	if style.hasBorder() {
		div.SetBorders(buildCellBorders(style))
	}
	if style.BackgroundColor != nil {
		div.SetBackground(*style.BackgroundColor)
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}

		childStyle := c.computeElementStyle(child, style)

		if child.DataAtom == atom.Figcaption {
			// Render figcaption as italic centered paragraph.
			text := collectText(child)
			if text == "" {
				continue
			}
			text = applyTextTransform(text, childStyle.TextTransform)
			f := resolveFont(childStyle)
			p := layout.NewParagraph(text, f, childStyle.FontSize)
			p.SetAlign(layout.AlignCenter)
			p.SetLeading(childStyle.LineHeight)
			p.SetSpaceBefore(4)
			div.Add(p)
		} else {
			// Other children (e.g. <img>, <pre>, <table>).
			elems := c.convertNode(child, style)
			for _, e := range elems {
				div.Add(e)
			}
		}
	}

	return []layout.Element{div}
}

// convertCSSTable handles elements with display:table — builds a layout.Table
// from children with display:table-row and display:table-cell.
func (c *converter) convertCSSTable(n *html.Node, style computedStyle) []layout.Element {
	tbl := layout.NewTable()
	tbl.SetAutoColumnWidths()

	if style.BorderCollapse == "collapse" {
		tbl.SetBorderCollapse(true)
	}
	if style.BorderSpacingH > 0 || style.BorderSpacingV > 0 {
		tbl.SetCellSpacing(style.BorderSpacingH, style.BorderSpacingV)
	}

	// Apply CSS width as table minimum width.
	if style.Width != nil {
		tbl.SetMinWidthUnit(cssLengthToUnitValue(style.Width, c.containerWidth, style.FontSize))
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		childStyle := c.computeElementStyle(child, style)

		if childStyle.Display == "table-row" {
			row := tbl.AddRow()
			for cell := child.FirstChild; cell != nil; cell = cell.NextSibling {
				if cell.Type != html.ElementNode {
					continue
				}
				cellStyle := c.computeElementStyle(cell, childStyle)
				cellElems := c.walkChildren(cell, cellStyle)

				var layoutCell *layout.Cell
				if len(cellElems) == 0 {
					f := resolveFont(cellStyle)
					layoutCell = row.AddCell(" ", f, cellStyle.FontSize)
				} else if len(cellElems) == 1 {
					layoutCell = row.AddCellElement(cellElems[0])
				} else {
					div := layout.NewDiv()
					for _, e := range cellElems {
						div.Add(e)
					}
					layoutCell = row.AddCellElement(div)
				}
				layoutCell.SetAlign(cellStyle.TextAlign)
				if cellStyle.hasPadding() {
					layoutCell.SetPaddingSides(layout.Padding{
						Top:    cellStyle.PaddingTop,
						Right:  cellStyle.PaddingRight,
						Bottom: cellStyle.PaddingBottom,
						Left:   cellStyle.PaddingLeft,
					})
				}
				if cellStyle.BackgroundColor != nil {
					layoutCell.SetBackground(*cellStyle.BackgroundColor)
				}
				if cellStyle.hasBorder() {
					layoutCell.SetBorders(buildCellBorders(cellStyle))
				}
			}
		} else {
			// Non-row children — treat as a single-cell row.
			childElems := c.convertNode(child, style)
			if len(childElems) > 0 {
				row := tbl.AddRow()
				div := layout.NewDiv()
				for _, e := range childElems {
					div.Add(e)
				}
				row.AddCellElement(div)
			}
		}
	}

	// Wrap in Div for margin.
	if style.MarginTop > 0 || style.MarginBottom > 0 {
		div := layout.NewDiv()
		div.Add(tbl)
		if style.MarginTop > 0 {
			div.SetSpaceBefore(style.MarginTop)
		}
		if style.MarginBottom > 0 {
			div.SetSpaceAfter(style.MarginBottom)
		}
		return []layout.Element{div}
	}

	return []layout.Element{tbl}
}

// convertTable converts a <table> element into a layout.Table.
func (c *converter) convertTable(n *html.Node, style computedStyle) []layout.Element {
	// Save parent containerWidth for resolving the table's own width properties.
	parentContainerWidth := c.containerWidth
	restore := c.narrowContainerWidth(style)
	defer restore()

	var elems []layout.Element
	tbl := layout.NewTable()

	// Parse border attribute (HTML4 style).
	borderWidth := 0.0
	if attr := getAttr(n, "border"); attr != "" && attr != "0" {
		borderWidth = 0.5
	}

	// Check for CSS border on the table style.
	if style.hasBorder() {
		borderWidth = style.BorderTopWidth
		if borderWidth == 0 {
			borderWidth = 0.5
		}
	}

	// border-collapse: collapse removes duplicate borders between cells.
	collapse := style.BorderCollapse == "collapse"
	if collapse {
		tbl.SetBorderCollapse(true)
	}
	if style.BorderSpacingH > 0 || style.BorderSpacingV > 0 {
		tbl.SetCellSpacing(style.BorderSpacingH, style.BorderSpacingV)
	}

	// Collect <col> widths from <colgroup>/<col> elements.
	var colWidths []layout.UnitValue

	// Walk children: <caption>, <colgroup>, <col>, <thead>, <tbody>, <tfoot>, or direct <tr>.
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		switch child.DataAtom {
		case atom.Caption:
			// Render caption as a centered paragraph before the table.
			text := collectText(child)
			if text != "" {
				f := resolveFont(style)
				p := layout.NewParagraph(text, f, style.FontSize)
				p.SetAlign(layout.AlignCenter)
				p.SetSpaceAfter(4)
				elems = append(elems, p)
			}
		case atom.Colgroup:
			for col := child.FirstChild; col != nil; col = col.NextSibling {
				if col.Type == html.ElementNode && col.DataAtom == atom.Col {
					colWidths = append(colWidths, c.parseColWidth(col, style)...)
				}
			}
		case atom.Col:
			colWidths = append(colWidths, c.parseColWidth(child, style)...)
		case atom.Thead:
			c.convertTableRows(child, tbl, style, borderWidth, true)
		case atom.Tbody:
			c.convertTableRows(child, tbl, style, borderWidth, false)
		case atom.Tfoot:
			c.convertTableFooterRows(child, tbl, style, borderWidth)
		case atom.Tr:
			c.convertTableRow(child, tbl, style, borderWidth, false)
		}
	}

	if len(colWidths) > 0 {
		tbl.SetColumnUnitWidths(colWidths)
	} else {
		tbl.SetAutoColumnWidths()
	}
	// Apply CSS width as table minimum width so auto-sizing expands to fill.
	// Use lazy UnitValue so percentages resolve at layout time against area.Width.
	if style.Width != nil {
		tbl.SetMinWidthUnit(cssLengthToUnitValue(style.Width, parentContainerWidth, style.FontSize))
	}

	// Apply table-level margin/background/width via Div wrapper.
	hasTableMargin := style.MarginTop > 0 || style.MarginBottom > 0
	hasTableWidth := style.MaxWidth != nil
	if hasTableMargin || style.BackgroundColor != nil || hasTableWidth {
		div := layout.NewDiv()
		div.Add(tbl)
		if style.MarginTop > 0 {
			div.SetSpaceBefore(style.MarginTop)
		}
		if style.MarginBottom > 0 {
			div.SetSpaceAfter(style.MarginBottom)
		}
		if style.BackgroundColor != nil {
			div.SetBackground(*style.BackgroundColor)
		}
		if style.MaxWidth != nil {
			div.SetMaxWidth(style.MaxWidth.toPoints(parentContainerWidth, style.FontSize))
		}
		// Caption elements come before the table wrapper.
		elems = append(elems, div)
		return elems
	}

	elems = append(elems, tbl)
	return elems
}

// parseColWidth extracts the width from a <col> element, respecting the span attribute.
func (c *converter) parseColWidth(col *html.Node, style computedStyle) []layout.UnitValue {
	span := 1
	if s := getAttr(col, "span"); s != "" {
		if v := parseInt(s); v > 1 {
			span = v
		}
	}

	colStyle := c.computeElementStyle(col, style)
	var uv layout.UnitValue
	if colStyle.Width != nil {
		if colStyle.Width.Unit == "%" {
			uv = layout.Pct(colStyle.Width.Value)
		} else {
			uv = layout.Pt(colStyle.Width.toPoints(0, style.FontSize))
		}
	} else if w := getAttr(col, "width"); w != "" {
		if strings.HasSuffix(w, "%") {
			if num, err := strconv.ParseFloat(strings.TrimSuffix(w, "%"), 64); err == nil {
				uv = layout.Pct(num)
			}
		} else {
			if num := parseAttrFloat(w); num > 0 {
				uv = layout.Pt(num * 0.75) // px to pt
			}
		}
	}

	var result []layout.UnitValue
	for i := 0; i < span; i++ {
		result = append(result, uv)
	}
	return result
}

// convertTableRows processes <tr> children within a <thead>/<tbody>/<tfoot>.
func (c *converter) convertTableRows(n *html.Node, tbl *layout.Table, style computedStyle, borderWidth float64, isHeader bool) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.DataAtom == atom.Tr {
			c.convertTableRow(child, tbl, style, borderWidth, isHeader)
		}
	}
}

// convertTableFooterRows processes <tr> children within a <tfoot>.
func (c *converter) convertTableFooterRows(n *html.Node, tbl *layout.Table, style computedStyle, borderWidth float64) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.DataAtom == atom.Tr {
			c.convertTableRowKind(child, tbl, style, borderWidth, "footer")
		}
	}
}

// convertTableRow processes a single <tr> and its <td>/<th> cells.
func (c *converter) convertTableRow(n *html.Node, tbl *layout.Table, parentStyle computedStyle, borderWidth float64, isHeader bool) {
	kind := "body"
	if isHeader {
		kind = "header"
	}
	c.convertTableRowKind(n, tbl, parentStyle, borderWidth, kind)
}

// convertTableRowKind processes a single <tr>. kind is "header", "footer", or "body".
func (c *converter) convertTableRowKind(n *html.Node, tbl *layout.Table, parentStyle computedStyle, borderWidth float64, kind string) {
	var row *layout.Row
	switch kind {
	case "header":
		row = tbl.AddHeaderRow()
	case "footer":
		row = tbl.AddFooterRow()
	default:
		row = tbl.AddRow()
	}

	rowStyle := c.computeElementStyle(n, parentStyle)

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		if child.DataAtom != atom.Td && child.DataAtom != atom.Th {
			continue
		}

		cellStyle := c.computeElementStyle(child, rowStyle)

		// For <th>, default to bold.
		if child.DataAtom == atom.Th {
			if cellStyle.FontWeight == "normal" {
				cellStyle.FontWeight = "bold"
			}
			if cellStyle.TextAlign == layout.AlignLeft {
				cellStyle.TextAlign = layout.AlignCenter
			}
		}

		cellElems := c.walkChildren(child, cellStyle)

		var cell *layout.Cell
		switch len(cellElems) {
		case 0:
			f := resolveFont(cellStyle)
			cell = row.AddCell(" ", f, cellStyle.FontSize)
		case 1:
			cell = row.AddCellElement(cellElems[0])
		default:
			div := layout.NewDiv()
			for _, e := range cellElems {
				div.Add(e)
			}
			cell = row.AddCellElement(div)
		}

		cell.SetAlign(cellStyle.TextAlign)

		// Per-side cell padding (default 4pt uniform).
		if cellStyle.hasPadding() {
			cell.SetPaddingSides(layout.Padding{
				Top:    cellStyle.PaddingTop,
				Right:  cellStyle.PaddingRight,
				Bottom: cellStyle.PaddingBottom,
				Left:   cellStyle.PaddingLeft,
			})
		} else {
			cell.SetPadding(4)
		}

		// Vertical alignment.
		switch cellStyle.VerticalAlign {
		case "middle":
			cell.SetVAlign(layout.VAlignMiddle)
		case "bottom":
			cell.SetVAlign(layout.VAlignBottom)
		}

		// Background color: cell CSS > row CSS.
		if cellStyle.BackgroundColor != nil {
			cell.SetBackground(*cellStyle.BackgroundColor)
		} else if rowStyle.BackgroundColor != nil {
			cell.SetBackground(*rowStyle.BackgroundColor)
		}

		// Cell borders: prefer per-cell CSS borders, fall back to table border,
		// or remove default borders if table has no border.
		if cellStyle.hasBorder() {
			cell.SetBorders(buildCellBorders(cellStyle))
		} else if borderWidth > 0 {
			cell.SetBorders(layout.AllBorders(layout.SolidBorder(borderWidth, layout.ColorBlack)))
		} else {
			// No cell border and no table border — clear the default borders.
			cell.SetBorders(layout.CellBorders{})
		}

		if cs := getAttr(child, "colspan"); cs != "" {
			if v := parseInt(cs); v > 1 {
				cell.SetColspan(v)
			}
		}
		if rs := getAttr(child, "rowspan"); rs != "" {
			if v := parseInt(rs); v > 1 {
				cell.SetRowspan(v)
			}
		}

		// CSS width on the cell → column width hint for auto-sizing.
		if cellStyle.Width != nil {
			w := cellStyle.Width.toPoints(c.containerWidth, cellStyle.FontSize)
			if w > 0 {
				cell.SetWidthHint(w)
			}
		}
	}
}

// convertLink converts an <a> element into a layout.Link or inline runs.
func (c *converter) convertLink(n *html.Node, style computedStyle) []layout.Element {
	href := getAttr(n, "href")
	text := collectText(n)
	if text == "" {
		return nil
	}
	text = applyTextTransform(text, style.TextTransform)

	f := resolveFont(style)

	if strings.HasPrefix(href, "#") {
		destName := href[1:]
		link := layout.NewInternalLink(text, destName, f, style.FontSize)
		link.SetColor(style.Color)
		link.SetUnderline()
		return []layout.Element{link}
	}

	link := layout.NewLink(text, href, f, style.FontSize)
	link.SetColor(style.Color)
	link.SetUnderline()
	return []layout.Element{link}
}

// --- Form element converters ---
// These render form controls as visual representations (not interactive).
// The goal is to produce a PDF that looks like the HTML form, not to create
// AcroForm fields.

// convertInput renders an <input> element as a visual representation.
func (c *converter) convertInput(n *html.Node, style computedStyle) []layout.Element {
	inputType := strings.ToLower(getAttr(n, "type"))
	if inputType == "" {
		inputType = "text"
	}

	switch inputType {
	case "hidden":
		return nil
	case "checkbox", "radio":
		return c.convertCheckboxRadio(n, style, inputType)
	case "submit", "reset", "button":
		return c.convertInputButton(n, style, inputType)
	default: // text, password, email, number, tel, url, search, date, etc.
		return c.convertInputText(n, style, inputType)
	}
}

// convertInputText renders a text-like input as a bordered box with value text.
func (c *converter) convertInputText(n *html.Node, style computedStyle, inputType string) []layout.Element {
	value := getAttr(n, "value")
	placeholder := getAttr(n, "placeholder")

	displayText := value
	textColor := style.Color
	if displayText == "" && placeholder != "" {
		displayText = placeholder
		textColor = layout.ColorGray
	}
	if displayText == "" {
		displayText = " " // ensure the box has content for sizing
	}
	if inputType == "password" && value != "" {
		displayText = strings.Repeat("●", len([]rune(value)))
	}

	f := resolveFont(style)
	p := layout.NewParagraph(displayText, f, style.FontSize)
	p.SetLeading(style.LineHeight)

	div := layout.NewDiv()
	div.Add(p)
	div.SetPaddingAll(layout.Padding{Top: 3, Right: 6, Bottom: 3, Left: 6})
	div.SetBorders(layout.AllBorders(layout.SolidBorder(0.75, layout.ColorGray)))
	div.SetBorderRadius(2)

	if style.BackgroundColor != nil {
		div.SetBackground(*style.BackgroundColor)
	} else {
		div.SetBackground(layout.ColorWhite)
	}

	run := layout.TextRun{Text: displayText, Font: f, FontSize: style.FontSize, Color: textColor}
	_ = run // we used NewParagraph above

	if style.hasBorder() {
		div.SetBorders(buildCellBorders(style))
	}

	return []layout.Element{div}
}

// convertCheckboxRadio renders a checkbox or radio button as a small box/circle with optional check.
func (c *converter) convertCheckboxRadio(n *html.Node, style computedStyle, inputType string) []layout.Element {
	checked := hasAttr(n, "checked")
	var symbol string
	if inputType == "checkbox" {
		if checked {
			symbol = "☑"
		} else {
			symbol = "☐"
		}
	} else { // radio
		if checked {
			symbol = "◉"
		} else {
			symbol = "○"
		}
	}

	f := resolveFont(style)
	p := layout.NewParagraph(symbol, f, style.FontSize)
	return []layout.Element{p}
}

// convertInputButton renders submit/reset/button inputs as a styled button box.
func (c *converter) convertInputButton(n *html.Node, style computedStyle, inputType string) []layout.Element {
	value := getAttr(n, "value")
	if value == "" {
		switch inputType {
		case "submit":
			value = "Submit"
		case "reset":
			value = "Reset"
		default:
			value = "Button"
		}
	}
	return c.buildButtonElement(value, style)
}

// convertButton renders a <button> element.
func (c *converter) convertButton(n *html.Node, style computedStyle) []layout.Element {
	text := collectText(n)
	if text == "" {
		text = "Button"
	}
	return c.buildButtonElement(text, style)
}

// buildButtonElement creates a styled button visual.
func (c *converter) buildButtonElement(text string, style computedStyle) []layout.Element {
	f := resolveFont(style)
	p := layout.NewParagraph(text, f, style.FontSize)
	p.SetAlign(layout.AlignCenter)

	div := layout.NewDiv()
	div.Add(p)
	div.SetPaddingAll(layout.Padding{Top: 4, Right: 12, Bottom: 4, Left: 12})
	div.SetBorderRadius(3)

	if style.BackgroundColor != nil {
		div.SetBackground(*style.BackgroundColor)
	} else {
		div.SetBackground(layout.Gray(0.9))
	}
	if style.hasBorder() {
		div.SetBorders(buildCellBorders(style))
	} else {
		div.SetBorders(layout.AllBorders(layout.SolidBorder(0.75, layout.ColorGray)))
	}

	return []layout.Element{div}
}

// convertSelect renders a <select> as a dropdown-like box showing the selected option.
func (c *converter) convertSelect(n *html.Node, style computedStyle) []layout.Element {
	// Find the selected option, or use the first option.
	var selectedText string
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.DataAtom == atom.Option {
			text := collectText(child)
			if selectedText == "" {
				selectedText = text // first option as fallback
			}
			if hasAttr(child, "selected") {
				selectedText = text
				break
			}
		}
		// Handle <optgroup>.
		if child.Type == html.ElementNode && child.DataAtom == atom.Optgroup {
			for opt := child.FirstChild; opt != nil; opt = opt.NextSibling {
				if opt.Type == html.ElementNode && opt.DataAtom == atom.Option {
					text := collectText(opt)
					if selectedText == "" {
						selectedText = text
					}
					if hasAttr(opt, "selected") {
						selectedText = text
						break
					}
				}
			}
		}
	}
	if selectedText == "" {
		selectedText = " "
	}

	// Render as text + dropdown arrow in a bordered box.
	displayText := selectedText + " ▾"
	f := resolveFont(style)
	p := layout.NewParagraph(displayText, f, style.FontSize)

	div := layout.NewDiv()
	div.Add(p)
	div.SetPaddingAll(layout.Padding{Top: 3, Right: 6, Bottom: 3, Left: 6})
	div.SetBorders(layout.AllBorders(layout.SolidBorder(0.75, layout.ColorGray)))
	div.SetBorderRadius(2)
	div.SetBackground(layout.ColorWhite)

	return []layout.Element{div}
}

// convertTextarea renders a <textarea> as a multi-line bordered box.
func (c *converter) convertTextarea(n *html.Node, style computedStyle) []layout.Element {
	text := collectText(n)
	placeholder := getAttr(n, "placeholder")

	displayText := text
	textColor := style.Color
	if displayText == "" && placeholder != "" {
		displayText = placeholder
		textColor = layout.ColorGray
	}
	if displayText == "" {
		displayText = " \n \n " // empty textarea placeholder (3 lines)
	}

	f := resolveFont(style)
	run := layout.TextRun{Text: displayText, Font: f, FontSize: style.FontSize, Color: textColor}
	p := layout.NewStyledParagraph(run)
	p.SetLeading(style.LineHeight)

	div := layout.NewDiv()
	div.Add(p)
	div.SetPaddingAll(layout.Padding{Top: 4, Right: 6, Bottom: 4, Left: 6})
	div.SetBorders(layout.AllBorders(layout.SolidBorder(0.75, layout.ColorGray)))
	div.SetBorderRadius(2)
	div.SetBackground(layout.ColorWhite)

	return []layout.Element{div}
}

// convertFieldset renders a <fieldset> as a bordered container.
// <legend> children are rendered as a bold header paragraph.
func (c *converter) convertFieldset(n *html.Node, style computedStyle) []layout.Element {
	div := layout.NewDiv()
	div.SetPaddingAll(layout.Padding{Top: 8, Right: 8, Bottom: 8, Left: 8})
	div.SetBorders(layout.AllBorders(layout.SolidBorder(0.75, layout.ColorGray)))
	div.SetBorderRadius(3)

	if style.MarginTop > 0 {
		div.SetSpaceBefore(style.MarginTop)
	}
	if style.MarginBottom > 0 {
		div.SetSpaceAfter(style.MarginBottom)
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		childStyle := c.computeElementStyle(child, style)
		if child.DataAtom == atom.Legend {
			text := collectText(child)
			if text != "" {
				f := resolveFont(childStyle)
				p := layout.NewParagraph(text, f, childStyle.FontSize)
				p.SetSpaceAfter(4)
				div.Add(p)
			}
		} else {
			elems := c.convertElement(child, style)
			for _, e := range elems {
				div.Add(e)
			}
		}
	}

	return []layout.Element{div}
}

// hasAttr returns true if the node has the given attribute (regardless of value).
func hasAttr(n *html.Node, key string) bool {
	for _, a := range n.Attr {
		if a.Key == key {
			return true
		}
	}
	return false
}

// convertFlex converts a display:flex container into a layout.Flex.
func (c *converter) convertFlex(n *html.Node, style computedStyle) []layout.Element {
	restore := c.narrowContainerWidth(style)
	defer restore()

	flex := layout.NewFlex()

	// Map direction.
	switch style.FlexDirection {
	case "column", "column-reverse":
		flex.SetDirection(layout.FlexColumn)
	default:
		flex.SetDirection(layout.FlexRow)
	}

	// Map justify-content.
	switch style.JustifyContent {
	case "flex-end":
		flex.SetJustifyContent(layout.JustifyFlexEnd)
	case "center":
		flex.SetJustifyContent(layout.JustifyCenter)
	case "space-between":
		flex.SetJustifyContent(layout.JustifySpaceBetween)
	case "space-around":
		flex.SetJustifyContent(layout.JustifySpaceAround)
	case "space-evenly":
		flex.SetJustifyContent(layout.JustifySpaceEvenly)
	default:
		flex.SetJustifyContent(layout.JustifyFlexStart)
	}

	// Map align-items.
	switch style.AlignItems {
	case "flex-start", "start":
		flex.SetAlignItems(layout.CrossAlignStart)
	case "flex-end", "end":
		flex.SetAlignItems(layout.CrossAlignEnd)
	case "center":
		flex.SetAlignItems(layout.CrossAlignCenter)
	default:
		flex.SetAlignItems(layout.CrossAlignStretch)
	}

	// Map wrap.
	switch style.FlexWrap {
	case "wrap", "wrap-reverse":
		flex.SetWrap(layout.FlexWrapOn)
	default:
		flex.SetWrap(layout.FlexNoWrap)
	}

	if style.Gap > 0 {
		flex.SetGap(style.Gap)
	}
	if style.ColumnGap > 0 && style.Gap == 0 {
		flex.SetColumnGap(style.ColumnGap)
	}

	if style.hasPadding() {
		flex.SetPaddingAll(layout.Padding{
			Top:    style.PaddingTop,
			Right:  style.PaddingRight,
			Bottom: style.PaddingBottom,
			Left:   style.PaddingLeft,
		})
	}
	if style.hasBorder() {
		flex.SetBorders(buildCellBorders(style))
	}
	if style.BackgroundColor != nil {
		flex.SetBackground(*style.BackgroundColor)
	}
	if style.MarginTop > 0 {
		flex.SetSpaceBefore(style.MarginTop)
	}
	if style.MarginBottom > 0 {
		flex.SetSpaceAfter(style.MarginBottom)
	}

	// Add children as flex items.
	// Each direct HTML child becomes exactly one flex item, even if
	// convertNode returns multiple layout elements (e.g. text with <br>).
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		// Skip whitespace-only text nodes inside flex containers (CSS spec:
		// whitespace-only text in flex containers does not generate flex items).
		if child.Type == html.TextNode {
			if strings.TrimSpace(child.Data) == "" {
				continue
			}
		}

		childElems := c.convertNode(child, style)
		if len(childElems) == 0 {
			continue
		}

		// Wrap multiple elements from a single HTML child into a Div
		// so they form one flex item (matching CSS flex behavior).
		var elem layout.Element
		if len(childElems) == 1 {
			elem = childElems[0]
		} else {
			wrapper := layout.NewDiv()
			for _, ce := range childElems {
				wrapper.Add(ce)
			}
			elem = wrapper
		}

		childStyle := style // default
		if child.Type == html.ElementNode {
			childStyle = c.computeElementStyle(child, style)
		}

		// CSS width on a flex child acts as flex-basis (when flex-basis is not set).
		effectiveBasis := childStyle.FlexBasis
		widthUsedAsBasis := false
		if effectiveBasis == nil && childStyle.Width != nil {
			effectiveBasis = childStyle.Width
			widthUsedAsBasis = true
		}

		// When CSS width is consumed as flex-basis, clear the Div's own width
		// to prevent double-resolution: the flex algorithm already allocates
		// the correct width, so the Div should fill its flex-allocated area
		// rather than re-resolving the percentage against that area.
		if widthUsedAsBasis {
			if d, ok := elem.(*layout.Div); ok {
				d.ClearWidthUnit()
			}
		}

		// Check if child has any margin (including negative) that needs FlexItem handling.
		hasMargins := childStyle.MarginTop != 0 || childStyle.MarginBottom != 0 ||
			childStyle.MarginLeft != 0 || childStyle.MarginRight != 0

		needsItem := childStyle.FlexGrow > 0 || childStyle.FlexShrink != 1 ||
			effectiveBasis != nil || (childStyle.AlignSelf != "" && childStyle.AlignSelf != "auto") ||
			childStyle.MarginTopAuto || childStyle.MarginLeftAuto || hasMargins
		if needsItem {
			item := layout.NewFlexItem(elem)
			if childStyle.FlexGrow > 0 {
				item.SetGrow(childStyle.FlexGrow)
			}
			if childStyle.FlexShrink != 1 {
				item.SetShrink(childStyle.FlexShrink)
			}
			if effectiveBasis != nil {
				item.SetBasisUnit(cssLengthToUnitValue(effectiveBasis, c.containerWidth, childStyle.FontSize))
			}
			switch childStyle.AlignSelf {
			case "flex-start", "start":
				item.SetAlignSelf(layout.CrossAlignStart)
			case "flex-end", "end":
				item.SetAlignSelf(layout.CrossAlignEnd)
			case "center":
				item.SetAlignSelf(layout.CrossAlignCenter)
			case "stretch":
				item.SetAlignSelf(layout.CrossAlignStretch)
			}
			if childStyle.MarginTopAuto {
				item.SetMarginTopAuto()
			}
			if childStyle.MarginLeftAuto {
				item.SetMarginLeftAuto()
			}
			if hasMargins {
				item.SetMargins(childStyle.MarginTop, childStyle.MarginRight,
					childStyle.MarginBottom, childStyle.MarginLeft)
				// Clear SpaceBefore/SpaceAfter on the element since the FlexItem's
				// margins handle vertical spacing — otherwise margins are doubled.
				if f, ok := elem.(*layout.Flex); ok {
					f.SetSpaceBefore(0)
					f.SetSpaceAfter(0)
				} else if d, ok := elem.(*layout.Div); ok {
					d.SetSpaceBefore(0)
					d.SetSpaceAfter(0)
				} else if p, ok := elem.(*layout.Paragraph); ok {
					p.SetSpaceBefore(0)
					p.SetSpaceAfter(0)
				}
			}
			flex.AddItem(item)
		} else {
			flex.Add(elem)
		}
	}

	// Wrap in a Div if the flex container has box-model properties
	// that the Flex type doesn't support (border-radius, opacity, etc.).
	hasExtraVisuals := style.BorderRadius > 0 ||
		(style.Opacity > 0 && style.Opacity < 1) ||
		style.Overflow == "hidden" ||
		style.BoxShadow != nil ||
		style.Width != nil || style.MaxWidth != nil || style.MinWidth != nil ||
		style.Height != nil || style.MinHeight != nil || style.MaxHeight != nil
	if hasExtraVisuals {
		div := layout.NewDiv()
		// Clear layout properties from the Flex — they'll be applied to the
		// wrapper Div instead. Without this, padding/borders/margins would be
		// applied twice (once on the Flex, once on the Div).
		// Background is kept on BOTH: the Div's background fills the full
		// height/min-height area, while the Flex's background covers content.
		// Since they're the same color, this ensures min-height backgrounds
		// work correctly (matching CSS behavior).
		flex.SetPaddingAll(layout.Padding{})
		flex.SetBorders(layout.CellBorders{})
		flex.SetSpaceBefore(0)
		flex.SetSpaceAfter(0)
		// If the wrapper Div has explicit height, tell the Flex its cross-axis
		// is definite so cross-axis stretching works correctly.
		if style.Height != nil {
			flex.SetDefiniteCrossSize(true)
		}
		div.Add(flex)
		applyDivStyles(div, style, c.containerWidth)
		return []layout.Element{div}
	}

	return []layout.Element{flex}
}

// collectRuns gathers inline content as TextRuns, recursing into inline children.
func (c *converter) collectRuns(n *html.Node, style computedStyle) []layout.TextRun {
	var runs []layout.TextRun
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case html.TextNode:
			text := processWhitespace(child.Data, style.WhiteSpace)
			if text == "" {
				continue
			}
			text = applyTextTransform(text, style.TextTransform)
			stdFont, embFont := c.resolveFontForText(style, text)
			run := layout.TextRun{
				Text:            text,
				Font:            stdFont,
				Embedded:        embFont,
				FontSize:        style.FontSize,
				Color:           style.Color,
				Decoration:      style.TextDecoration,
				DecorationColor: style.TextDecorationColor,
				DecorationStyle: style.TextDecorationStyle,
				LetterSpacing:   style.LetterSpacing,
				WordSpacing:     style.WordSpacing,
				BaselineShift:   baselineShiftFromStyle(style),
			}
			runs = append(runs, run)
		case html.ElementNode:
			if child.DataAtom == atom.Br {
				// Insert a newline marker that convertParagraph splits on.
				runs = append(runs, layout.TextRun{Text: "\n"})
				continue
			}
			childStyle := c.computeElementStyle(child, style)
			childRuns := c.collectRuns(child, childStyle)
			runs = append(runs, childRuns...)
		}
	}
	return runs
}

// computeElementStyle resolves the style for an element node.
func (c *converter) computeElementStyle(n *html.Node, parent computedStyle) computedStyle {
	style := parent.inherit()

	// Apply tag defaults.
	c.applyTagDefaults(n, &style)

	// Apply stylesheet rules.
	if c.sheet != nil {
		for _, decl := range c.sheet.matchingDeclarations(n) {
			c.applyProperty(decl.property, decl.value, &style)
		}
	}

	// Apply inline style attribute (highest specificity).
	if attr := getAttr(n, "style"); attr != "" {
		c.applyInlineStyle(attr, &style)
	}

	return style
}

// applyTagDefaults sets browser-like defaults for known HTML elements.
func (c *converter) applyTagDefaults(n *html.Node, style *computedStyle) {
	switch n.DataAtom {
	case atom.H1:
		style.FontSize = 24 // 32px * 0.75
		style.FontWeight = "bold"
		style.MarginTop = 16.08 // 0.67em at 32px → 32*0.67*0.75
		style.MarginBottom = 16.08
	case atom.H2:
		style.FontSize = 18 // 24px * 0.75
		style.FontWeight = "bold"
		style.MarginTop = 14.94 // 0.83em at 24px → 24*0.83*0.75
		style.MarginBottom = 14.94
	case atom.H3:
		style.FontSize = 14.04 // 18.72px * 0.75
		style.FontWeight = "bold"
		style.MarginTop = 14.04 // 1em at 18.72px → 18.72*0.75
		style.MarginBottom = 14.04
	case atom.H4:
		style.FontSize = 12 // 16px * 0.75
		style.FontWeight = "bold"
		style.MarginTop = 16.02 // 1.33em at 16px → 16*1.33*0.75
		style.MarginBottom = 16.02
	case atom.H5:
		style.FontSize = 9.96 // 13.28px * 0.75
		style.FontWeight = "bold"
		style.MarginTop = 16.60 // 1.67em at 13.28px → 13.28*1.67*0.75
		style.MarginBottom = 16.60
	case atom.H6:
		style.FontSize = 8.01 // 10.72px * 0.75
		style.FontWeight = "bold"
		style.MarginTop = 18.62 // 2.33em at 10.72px → 10.72*2.33*0.75
		style.MarginBottom = 18.62
	case atom.P:
		style.MarginTop = 12 // 1em at 16px → 16*0.75
		style.MarginBottom = 12
	case atom.Strong, atom.B:
		style.FontWeight = "bold"
	case atom.Em, atom.I:
		style.FontStyle = "italic"
	case atom.U:
		style.TextDecoration |= layout.DecorationUnderline
	case atom.S, atom.Del:
		style.TextDecoration |= layout.DecorationStrikethrough
	case atom.Small:
		style.FontSize = style.FontSize * 0.833
	case atom.Sub:
		style.FontSize = style.FontSize * 0.75
	case atom.Sup:
		style.FontSize = style.FontSize * 0.75
	case atom.Code:
		style.FontFamily = "courier"
	case atom.Pre:
		style.FontFamily = "courier"
		style.WhiteSpace = "pre"
		style.MarginTop = 12
		style.MarginBottom = 12
	case atom.Hr:
		style.MarginTop = 6
		style.MarginBottom = 6
	case atom.A:
		style.Color = layout.RGB(0, 0, 0.933) // default link blue
		style.TextDecoration |= layout.DecorationUnderline
	case atom.Table:
		style.MarginTop = 12
		style.MarginBottom = 12
		style.BorderCollapse = "collapse"
	case atom.Ul, atom.Ol:
		style.MarginTop = 12
		style.MarginBottom = 12
	case atom.Blockquote:
		style.MarginTop = 12
		style.MarginBottom = 12
	case atom.Dl:
		style.MarginTop = 12
		style.MarginBottom = 12
	case atom.Dt:
		style.FontWeight = "bold"
	case atom.Dd:
		style.MarginLeft = 30 // browser default ~40px → 30pt
	case atom.Figure:
		style.MarginTop = 12
		style.MarginBottom = 12
	case atom.Figcaption:
		style.FontStyle = "italic"
		style.FontSize = style.FontSize * 0.9
	case atom.Fieldset:
		style.MarginTop = 9 // ~12px * 0.75
		style.MarginBottom = 9
		style.Display = "block"
	case atom.Legend:
		style.FontWeight = "bold"
	case atom.Button:
		style.Display = "inline"
	case atom.Input, atom.Select, atom.Textarea:
		style.Display = "inline"
	case atom.Label:
		style.Display = "inline"
	}
}

// applyInlineStyle parses a CSS style attribute and applies it to the style.
func (c *converter) applyInlineStyle(attr string, style *computedStyle) {
	for _, decl := range splitDeclarations(attr) {
		prop, val := splitDeclaration(decl)
		if prop == "" || val == "" {
			continue
		}
		c.applyProperty(prop, val, style)
	}
}

// resolveVars replaces var(--name) and var(--name, fallback) references in a
// CSS value string using the element's custom properties. Handles nested var()
// calls and multiple var() references in a single value.
func resolveVars(value string, style *computedStyle) string {
	for {
		idx := strings.Index(value, "var(")
		if idx < 0 {
			return value
		}
		// Find matching closing paren, accounting for nested parens.
		depth := 0
		end := -1
		for i := idx + 4; i < len(value); i++ {
			if value[i] == '(' {
				depth++
			}
			if value[i] == ')' {
				if depth == 0 {
					end = i
					break
				}
				depth--
			}
		}
		if end < 0 {
			return value // malformed, bail out
		}

		inner := value[idx+4 : end]
		// Split on first comma for fallback.
		name, fallback := inner, ""
		if ci := strings.IndexByte(inner, ','); ci >= 0 {
			name = strings.TrimSpace(inner[:ci])
			fallback = strings.TrimSpace(inner[ci+1:])
		} else {
			name = strings.TrimSpace(name)
		}

		resolved := fallback
		if style.CustomProperties != nil {
			if v, ok := style.CustomProperties[name]; ok {
				resolved = v
			}
		}
		value = value[:idx] + resolved + value[end+1:]
	}
}

// applyProperty applies a single CSS property to a computed style.
func (c *converter) applyProperty(prop, val string, style *computedStyle) {
	// Resolve var() references before any processing.
	if strings.Contains(val, "var(") {
		val = resolveVars(val, style)
	}

	// Store custom properties (CSS variables).
	if strings.HasPrefix(prop, "--") {
		if style.CustomProperties == nil {
			style.CustomProperties = make(map[string]string)
		}
		style.CustomProperties[prop] = val
		return
	}

	switch prop {
	case "color":
		if c, ok := parseColor(val); ok {
			style.Color = c
		}
	case "background-color":
		if c, ok := parseColor(val); ok {
			style.BackgroundColor = &c
		}
	case "background":
		// Background shorthand: try parsing as a color (simple case).
		if clr, ok := parseColor(val); ok {
			style.BackgroundColor = &clr
		}
	case "background-image":
		style.BackgroundImage = strings.TrimSpace(val)
	case "background-size":
		style.BackgroundSize = strings.TrimSpace(strings.ToLower(val))
	case "background-position":
		style.BackgroundPosition = strings.TrimSpace(strings.ToLower(val))
	case "background-repeat":
		style.BackgroundRepeat = strings.TrimSpace(strings.ToLower(val))
	case "font-family":
		style.FontFamily = parseFontFamily(val)
	case "font-size":
		style.FontSize = parseFontSize(val, style.FontSize)
	case "font-weight":
		style.FontWeight = parseFontWeight(val)
	case "font-style":
		style.FontStyle = parseFontStyle(val)
	case "text-align":
		if a, ok := parseTextAlign(val); ok {
			style.TextAlign = a
		}
	case "text-decoration":
		style.TextDecoration = parseTextDecoration(val)
	case "text-transform":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "uppercase" || v == "lowercase" || v == "capitalize" || v == "none" {
			style.TextTransform = v
		}
	case "white-space":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "normal" || v == "nowrap" || v == "pre" || v == "pre-wrap" || v == "pre-line" {
			style.WhiteSpace = v
		}
	case "word-break":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "normal" || v == "break-all" || v == "keep-all" || v == "break-word" {
			style.WordBreak = v
		}
	case "hyphens", "-webkit-hyphens":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "none" || v == "manual" || v == "auto" {
			style.Hyphens = v
		}
	case "letter-spacing":
		if l := parseLength(val); l != nil {
			style.LetterSpacing = l.toPoints(0, style.FontSize)
		} else if strings.TrimSpace(strings.ToLower(val)) == "normal" {
			style.LetterSpacing = 0
		}
	case "word-spacing":
		if l := parseLength(val); l != nil {
			style.WordSpacing = l.toPoints(0, style.FontSize)
		} else if strings.TrimSpace(strings.ToLower(val)) == "normal" {
			style.WordSpacing = 0
		}
	case "text-indent":
		if l := parseLength(val); l != nil {
			style.TextIndent = l.toPoints(0, style.FontSize)
		}
	case "line-height":
		style.LineHeight = parseLineHeight(val, style.FontSize)
	case "display":
		style.Display = parseDisplay(val)
	case "margin":
		style.MarginTop, style.MarginRight, style.MarginBottom, style.MarginLeft =
			parseMarginShorthand(val, style.FontSize)
		// Detect auto keywords in margin shorthand.
		parts := strings.Fields(val)
		autoFlags := make([]bool, len(parts))
		for i, p := range parts {
			autoFlags[i] = strings.ToLower(p) == "auto"
		}
		switch len(parts) {
		case 1:
			if autoFlags[0] {
				style.MarginTopAuto = true
				style.MarginLeftAuto = true
				style.MarginRightAuto = true
			}
		case 2:
			if autoFlags[0] {
				style.MarginTopAuto = true
			}
			if autoFlags[1] {
				style.MarginLeftAuto = true
				style.MarginRightAuto = true
			}
		case 3:
			if autoFlags[0] {
				style.MarginTopAuto = true
			}
			if autoFlags[1] {
				style.MarginLeftAuto = true
				style.MarginRightAuto = true
			}
		case 4:
			if autoFlags[0] {
				style.MarginTopAuto = true
			}
			if autoFlags[1] {
				style.MarginRightAuto = true
			}
			if autoFlags[3] {
				style.MarginLeftAuto = true
			}
		}
	case "margin-top":
		if strings.TrimSpace(strings.ToLower(val)) == "auto" {
			style.MarginTopAuto = true
		} else {
			style.MarginTop = parseBoxSide(val, style.FontSize)
		}
	case "margin-right":
		if strings.TrimSpace(strings.ToLower(val)) == "auto" {
			style.MarginRightAuto = true
		} else {
			style.MarginRight = parseBoxSide(val, style.FontSize)
		}
	case "margin-bottom":
		style.MarginBottom = parseBoxSide(val, style.FontSize)
	case "margin-left":
		if strings.TrimSpace(strings.ToLower(val)) == "auto" {
			style.MarginLeftAuto = true
		} else {
			style.MarginLeft = parseBoxSide(val, style.FontSize)
		}
	case "padding":
		style.PaddingTop, style.PaddingRight, style.PaddingBottom, style.PaddingLeft =
			parseMarginShorthand(val, style.FontSize)
	case "padding-top":
		style.PaddingTop = parseBoxSide(val, style.FontSize)
	case "padding-right":
		style.PaddingRight = parseBoxSide(val, style.FontSize)
	case "padding-bottom":
		style.PaddingBottom = parseBoxSide(val, style.FontSize)
	case "padding-left":
		style.PaddingLeft = parseBoxSide(val, style.FontSize)
	case "width":
		style.Width = parseLength(val)
	case "max-width":
		style.MaxWidth = parseLength(val)
	case "min-width":
		style.MinWidth = parseLength(val)
	case "height":
		style.Height = parseLength(val)
	case "border":
		w, s, clr := parseBorderFull(val, style.FontSize)
		style.BorderTopWidth = w
		style.BorderRightWidth = w
		style.BorderBottomWidth = w
		style.BorderLeftWidth = w
		style.BorderTopStyle = s
		style.BorderRightStyle = s
		style.BorderBottomStyle = s
		style.BorderLeftStyle = s
		style.BorderTopColor = clr
		style.BorderRightColor = clr
		style.BorderBottomColor = clr
		style.BorderLeftColor = clr
	case "border-width":
		w := parseBoxSide(val, style.FontSize)
		style.BorderTopWidth = w
		style.BorderRightWidth = w
		style.BorderBottomWidth = w
		style.BorderLeftWidth = w
	case "border-top-width":
		style.BorderTopWidth = parseBoxSide(val, style.FontSize)
	case "border-right-width":
		style.BorderRightWidth = parseBoxSide(val, style.FontSize)
	case "border-bottom-width":
		style.BorderBottomWidth = parseBoxSide(val, style.FontSize)
	case "border-left-width":
		style.BorderLeftWidth = parseBoxSide(val, style.FontSize)
	case "border-color":
		if c, ok := parseColor(val); ok {
			style.BorderTopColor = c
			style.BorderRightColor = c
			style.BorderBottomColor = c
			style.BorderLeftColor = c
		}
	case "border-style":
		style.BorderTopStyle = val
		style.BorderRightStyle = val
		style.BorderBottomStyle = val
		style.BorderLeftStyle = val
	case "flex-direction":
		style.FlexDirection = strings.TrimSpace(strings.ToLower(val))
	case "justify-content":
		style.JustifyContent = strings.TrimSpace(strings.ToLower(val))
	case "align-items":
		style.AlignItems = strings.TrimSpace(strings.ToLower(val))
	case "align-self":
		style.AlignSelf = strings.TrimSpace(strings.ToLower(val))
	case "flex-wrap":
		style.FlexWrap = strings.TrimSpace(strings.ToLower(val))
	case "flex":
		parseFlexShorthand(val, style)
	case "flex-flow":
		parseFlexFlowShorthand(val, style)
	case "flex-grow":
		if v, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			style.FlexGrow = v
		}
	case "flex-shrink":
		if v, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			style.FlexShrink = v
		}
	case "flex-basis":
		style.FlexBasis = parseLength(val)
	case "gap", "grid-gap":
		parts := strings.Fields(strings.TrimSpace(val))
		if len(parts) == 1 {
			v := parseBoxSide(parts[0], style.FontSize)
			style.Gap = v
			style.RowGap = v
			style.GridColumnGap = v
		} else if len(parts) >= 2 {
			style.RowGap = parseBoxSide(parts[0], style.FontSize)
			style.GridColumnGap = parseBoxSide(parts[1], style.FontSize)
			style.Gap = style.RowGap // flex compat: use row-gap value
		}
	case "row-gap":
		style.RowGap = parseBoxSide(val, style.FontSize)
	case "grid-template-columns":
		style.GridTemplateColumns = strings.TrimSpace(val)
	case "grid-template-rows":
		style.GridTemplateRows = strings.TrimSpace(val)
	case "grid-column":
		style.GridColumnStart, style.GridColumnEnd = parseGridLine(val)
	case "grid-row":
		style.GridRowStart, style.GridRowEnd = parseGridLine(val)
	case "grid-column-start":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			style.GridColumnStart = v
		}
	case "grid-column-end":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			style.GridColumnEnd = v
		}
	case "grid-row-start":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			style.GridRowStart = v
		}
	case "grid-row-end":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			style.GridRowEnd = v
		}
	case "grid-auto-flow":
		style.GridAutoFlow = strings.TrimSpace(strings.ToLower(val))
	case "grid-auto-rows":
		style.GridAutoRows = strings.TrimSpace(val)
	case "grid-template-areas":
		style.GridTemplateAreas = parseGridTemplateAreas(val)
	case "grid-area":
		style.GridArea = strings.TrimSpace(val)
	case "align-content":
		style.AlignContent = strings.TrimSpace(strings.ToLower(val))
	case "justify-items":
		style.JustifyItems = strings.TrimSpace(strings.ToLower(val))
	case "page-break-before", "break-before":
		v := strings.TrimSpace(strings.ToLower(val))
		switch v {
		case "always", "page":
			style.PageBreakBefore = "always"
		case "avoid", "avoid-page":
			style.PageBreakBefore = "avoid"
		case "auto":
			style.PageBreakBefore = "auto"
		}
	case "page-break-after", "break-after":
		v := strings.TrimSpace(strings.ToLower(val))
		switch v {
		case "always", "page":
			style.PageBreakAfter = "always"
		case "avoid", "avoid-page":
			style.PageBreakAfter = "avoid"
		case "auto":
			style.PageBreakAfter = "auto"
		}
	case "page-break-inside", "break-inside":
		v := strings.TrimSpace(strings.ToLower(val))
		switch v {
		case "avoid", "avoid-page":
			style.PageBreakInside = "avoid"
		case "auto":
			style.PageBreakInside = "auto"
		}
	case "orphans":
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && n > 0 {
			style.Orphans = n
		}
	case "widows":
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && n > 0 {
			style.Widows = n
		}
	case "list-style-type", "list-style":
		v := strings.TrimSpace(strings.ToLower(val))
		// Extract type from shorthand (list-style: disc inside).
		if parts := strings.Fields(v); len(parts) > 0 {
			style.ListStyleType = parts[0]
		}
	case "border-collapse":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "collapse" || v == "separate" {
			style.BorderCollapse = v
		}
	case "border-spacing":
		// Supports: "5px" (both) or "5px 10px" (horizontal vertical).
		parts := strings.Fields(strings.TrimSpace(val))
		if len(parts) == 1 {
			if l := parseLength(parts[0]); l != nil {
				v := l.toPoints(0, style.FontSize)
				style.BorderSpacingH = v
				style.BorderSpacingV = v
			}
		} else if len(parts) >= 2 {
			if lh := parseLength(parts[0]); lh != nil {
				style.BorderSpacingH = lh.toPoints(0, style.FontSize)
			}
			if lv := parseLength(parts[1]); lv != nil {
				style.BorderSpacingV = lv.toPoints(0, style.FontSize)
			}
		}
	case "vertical-align":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "top" || v == "middle" || v == "bottom" || v == "super" || v == "sub" || v == "baseline" || v == "text-top" || v == "text-bottom" {
			style.VerticalAlign = v
		}
	case "border-top":
		w, s, clr := parseBorderFull(val, style.FontSize)
		style.BorderTopWidth = w
		style.BorderTopStyle = s
		style.BorderTopColor = clr
	case "border-right":
		w, s, clr := parseBorderFull(val, style.FontSize)
		style.BorderRightWidth = w
		style.BorderRightStyle = s
		style.BorderRightColor = clr
	case "border-bottom":
		w, s, clr := parseBorderFull(val, style.FontSize)
		style.BorderBottomWidth = w
		style.BorderBottomStyle = s
		style.BorderBottomColor = clr
	case "border-left":
		w, s, clr := parseBorderFull(val, style.FontSize)
		style.BorderLeftWidth = w
		style.BorderLeftStyle = s
		style.BorderLeftColor = clr
	case "font":
		fs, fw, sz, lh, ff := parseFontShorthand(val, style.FontSize)
		if fs != "" {
			style.FontStyle = fs
		}
		if fw != "" {
			style.FontWeight = fw
		}
		if sz > 0 {
			style.FontSize = sz
		}
		if lh > 0 {
			style.LineHeight = lh
		}
		if ff != "" {
			style.FontFamily = ff
		}
	case "border-radius":
		if l := parseLength(val); l != nil {
			style.BorderRadius = l.toPoints(0, style.FontSize)
		}
	case "opacity":
		if v, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
			style.Opacity = v
		}
	case "overflow":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "hidden" || v == "visible" || v == "auto" || v == "scroll" {
			style.Overflow = v
		}
	case "float":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "left" || v == "right" || v == "none" {
			style.Float = v
		}
	case "clear":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "left" || v == "right" || v == "both" || v == "none" {
			style.Clear = v
		}
	case "box-sizing":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "content-box" || v == "border-box" {
			style.BoxSizing = v
		}
	case "visibility":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "visible" || v == "hidden" || v == "collapse" {
			style.Visibility = v
		}
	case "min-height":
		style.MinHeight = parseLength(val)
	case "max-height":
		style.MaxHeight = parseLength(val)
	case "position":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "static" || v == "relative" || v == "absolute" || v == "fixed" {
			style.Position = v
		}
	case "top":
		style.Top = parseLength(val)
	case "left":
		style.Left = parseLength(val)
	case "right":
		style.Right = parseLength(val)
	case "bottom":
		style.Bottom = parseLength(val)
	case "z-index":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil {
			style.ZIndex = v
			style.ZIndexSet = true
		}

	// Box shadow
	case "box-shadow":
		style.BoxShadow = parseBoxShadow(val, style.FontSize)

	// Text overflow
	case "text-overflow":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "clip" || v == "ellipsis" {
			style.TextOverflow = v
		}

	// Outline
	case "outline":
		w, s, clr := parseBorderFull(val, style.FontSize)
		style.OutlineWidth = w
		style.OutlineStyle = s
		style.OutlineColor = clr
	case "outline-width":
		style.OutlineWidth = parseBoxSide(val, style.FontSize)
	case "outline-style":
		style.OutlineStyle = strings.TrimSpace(strings.ToLower(val))
	case "outline-color":
		if c, ok := parseColor(val); ok {
			style.OutlineColor = c
		}
	case "outline-offset":
		style.OutlineOffset = parseBoxSide(val, style.FontSize)

	// CSS Columns
	case "column-count":
		if v, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && v > 0 {
			style.ColumnCount = v
		}
	case "column-gap":
		v := parseBoxSide(val, style.FontSize)
		style.ColumnGap = v
		style.GridColumnGap = v
	case "columns":
		parts := strings.Fields(strings.TrimSpace(val))
		for _, p := range parts {
			if v, err := strconv.Atoi(p); err == nil && v > 0 {
				style.ColumnCount = v
			} else if l := parseLength(p); l != nil {
				style.ColumnGap = l.toPoints(0, style.FontSize)
			}
		}

	// CSS transforms
	case "transform":
		style.Transform = strings.TrimSpace(val)
	case "transform-origin":
		style.TransformOrigin = strings.TrimSpace(val)

	// Text decoration extensions
	case "text-decoration-color":
		if c, ok := parseColor(val); ok {
			style.TextDecorationColor = &c
		}
	case "text-decoration-style":
		v := strings.TrimSpace(strings.ToLower(val))
		if v == "solid" || v == "dashed" || v == "dotted" || v == "double" || v == "wavy" {
			style.TextDecorationStyle = v
		}

	// CSS counters
	case "counter-reset":
		style.CounterReset = parseCounterEntries(val, 0)
	case "counter-increment":
		style.CounterIncrement = parseCounterEntries(val, 1)
	}
}

// parseTransform parses a CSS transform value like "rotate(45deg) scale(1.5)"
// into a slice of layout.TransformOp.
func parseTransform(val string) []layout.TransformOp {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "none" || val == "" {
		return nil
	}

	var ops []layout.TransformOp
	// Match function calls: name(args)
	for val != "" {
		// Find the next function name.
		parenIdx := strings.Index(val, "(")
		if parenIdx < 0 {
			break
		}
		fname := strings.TrimSpace(val[:parenIdx])
		closeIdx := strings.Index(val[parenIdx:], ")")
		if closeIdx < 0 {
			break
		}
		argsStr := val[parenIdx+1 : parenIdx+closeIdx]
		val = strings.TrimSpace(val[parenIdx+closeIdx+1:])

		// Parse arguments (comma or space separated).
		argsStr = strings.ReplaceAll(argsStr, ",", " ")
		parts := strings.Fields(argsStr)

		switch fname {
		case "rotate":
			if len(parts) >= 1 {
				deg := parseAngle(parts[0])
				ops = append(ops, layout.TransformOp{Type: "rotate", Values: [2]float64{deg, 0}})
			}
		case "scale":
			if len(parts) >= 2 {
				sx := parseNumericVal(parts[0])
				sy := parseNumericVal(parts[1])
				ops = append(ops, layout.TransformOp{Type: "scale", Values: [2]float64{sx, sy}})
			} else if len(parts) >= 1 {
				s := parseNumericVal(parts[0])
				ops = append(ops, layout.TransformOp{Type: "scale", Values: [2]float64{s, s}})
			}
		case "scalex":
			if len(parts) >= 1 {
				s := parseNumericVal(parts[0])
				ops = append(ops, layout.TransformOp{Type: "scale", Values: [2]float64{s, 1}})
			}
		case "scaley":
			if len(parts) >= 1 {
				s := parseNumericVal(parts[0])
				ops = append(ops, layout.TransformOp{Type: "scale", Values: [2]float64{1, s}})
			}
		case "translate":
			if len(parts) >= 2 {
				tx := parseLengthPx(parts[0])
				ty := parseLengthPx(parts[1])
				ops = append(ops, layout.TransformOp{Type: "translate", Values: [2]float64{tx, -ty}})
			} else if len(parts) >= 1 {
				tx := parseLengthPx(parts[0])
				ops = append(ops, layout.TransformOp{Type: "translate", Values: [2]float64{tx, 0}})
			}
		case "translatex":
			if len(parts) >= 1 {
				tx := parseLengthPx(parts[0])
				ops = append(ops, layout.TransformOp{Type: "translate", Values: [2]float64{tx, 0}})
			}
		case "translatey":
			if len(parts) >= 1 {
				ty := parseLengthPx(parts[0])
				ops = append(ops, layout.TransformOp{Type: "translate", Values: [2]float64{0, -ty}})
			}
		case "skew":
			if len(parts) >= 2 {
				ax := parseAngle(parts[0])
				ay := parseAngle(parts[1])
				ops = append(ops, layout.TransformOp{Type: "skewX", Values: [2]float64{ax, 0}})
				ops = append(ops, layout.TransformOp{Type: "skewY", Values: [2]float64{ay, 0}})
			} else if len(parts) >= 1 {
				ax := parseAngle(parts[0])
				ops = append(ops, layout.TransformOp{Type: "skewX", Values: [2]float64{ax, 0}})
			}
		case "skewx":
			if len(parts) >= 1 {
				a := parseAngle(parts[0])
				ops = append(ops, layout.TransformOp{Type: "skewX", Values: [2]float64{a, 0}})
			}
		case "skewy":
			if len(parts) >= 1 {
				a := parseAngle(parts[0])
				ops = append(ops, layout.TransformOp{Type: "skewY", Values: [2]float64{a, 0}})
			}
		}
	}
	return ops
}

// parseAngle parses a CSS angle value like "45deg", "1.5rad", or "100grad".
// Returns degrees.
func parseAngle(s string) float64 {
	s = strings.TrimSpace(strings.ToLower(s))
	if strings.HasSuffix(s, "deg") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "deg"), 64)
		return v
	}
	if strings.HasSuffix(s, "rad") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "rad"), 64)
		return v * 180 / 3.14159265358979323846
	}
	if strings.HasSuffix(s, "grad") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "grad"), 64)
		return v * 0.9 // 400grad = 360deg
	}
	if strings.HasSuffix(s, "turn") {
		v, _ := strconv.ParseFloat(strings.TrimSuffix(s, "turn"), 64)
		return v * 360
	}
	// Bare number — assume degrees.
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseNumericVal parses a bare numeric value (no unit).
func parseNumericVal(s string) float64 {
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v
}

// parseLengthPx parses a CSS length for use in transforms (px → pt conversion).
func parseLengthPx(s string) float64 {
	l := parseLength(s)
	if l != nil {
		return l.toPoints(0, 12) // default font size context
	}
	// Bare number — treat as px.
	v, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return v * 0.75
}

// parseTransformOrigin parses a CSS transform-origin value like
// "center center", "top left", "50% 50%" into point coordinates
// relative to the element's top-left corner.
func parseTransformOrigin(val string, width, height, fontSize float64) (float64, float64) {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "" {
		// Default: center center.
		return width / 2, height / 2
	}

	parts := strings.Fields(val)
	if len(parts) == 1 {
		// Single value: applies to X, Y defaults to center.
		x := resolveOriginComponent(parts[0], width, fontSize)
		return x, height / 2
	}
	x := resolveOriginComponent(parts[0], width, fontSize)
	y := resolveOriginComponent(parts[1], height, fontSize)
	return x, y
}

// resolveOriginComponent resolves a single transform-origin keyword or length
// to a point value relative to the given dimension.
func resolveOriginComponent(s string, dimension, fontSize float64) float64 {
	switch s {
	case "left", "top":
		return 0
	case "center":
		return dimension / 2
	case "right", "bottom":
		return dimension
	default:
		if l := parseLength(s); l != nil {
			return l.toPoints(dimension, fontSize)
		}
		return dimension / 2
	}
}

// parseBoxShadow parses a CSS box-shadow value.
// Format: "offsetX offsetY blur spread color" or "none".
func parseBoxShadow(val string, fontSize float64) *boxShadow {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "none" || val == "" {
		return nil
	}

	// Remove "inset" keyword if present.
	inset := false
	if strings.Contains(val, "inset") {
		inset = true
		val = strings.ReplaceAll(val, "inset", "")
		val = strings.TrimSpace(val)
	}

	parts := strings.Fields(val)
	if len(parts) < 2 {
		return nil
	}

	bs := &boxShadow{Inset: inset}

	// Parse lengths (up to 4) and the remaining token as color.
	var lengths []float64
	var colorToken string
	for _, p := range parts {
		if l := parseLength(p); l != nil {
			lengths = append(lengths, l.toPoints(0, fontSize))
		} else {
			// Accumulate as potential color token.
			if colorToken == "" {
				colorToken = p
			} else {
				colorToken += " " + p
			}
		}
	}

	if len(lengths) >= 2 {
		bs.OffsetX = lengths[0]
		bs.OffsetY = lengths[1]
	}
	if len(lengths) >= 3 {
		bs.Blur = lengths[2]
	}
	if len(lengths) >= 4 {
		bs.Spread = lengths[3]
	}

	if colorToken != "" {
		if c, ok := parseColor(colorToken); ok {
			bs.Color = c
		} else {
			bs.Color = layout.ColorBlack
		}
	} else {
		bs.Color = layout.ColorBlack
	}

	return bs
}

// resolveFont maps a computedStyle's family/weight/style to a standard PDF font.
func resolveFont(style computedStyle) *font.Standard {
	bold := style.FontWeight == "bold"
	italic := style.FontStyle == "italic"

	switch style.FontFamily {
	case "courier":
		switch {
		case bold && italic:
			return font.CourierBoldOblique
		case bold:
			return font.CourierBold
		case italic:
			return font.CourierOblique
		default:
			return font.Courier
		}
	case "times":
		switch {
		case bold && italic:
			return font.TimesBoldItalic
		case bold:
			return font.TimesBold
		case italic:
			return font.TimesItalic
		default:
			return font.TimesRoman
		}
	default: // "helvetica"
		switch {
		case bold && italic:
			return font.HelveticaBoldOblique
		case bold:
			return font.HelveticaBold
		case italic:
			return font.HelveticaOblique
		default:
			return font.Helvetica
		}
	}
}

// resolveFontPair returns either a standard font or an embedded font for the
// given style. If the font family matches an @font-face rule, the embedded
// font is returned; otherwise the standard font is returned.
func (c *converter) resolveFontPair(style computedStyle) (*font.Standard, *font.EmbeddedFont) {
	if len(c.embeddedFonts) > 0 {
		family := strings.ToLower(style.FontFamily)
		key := family + "|" + style.FontWeight + "|" + style.FontStyle
		if ef, ok := c.embeddedFonts[key]; ok {
			return nil, ef
		}
		// Try without specific weight/style.
		keyBase := family + "|normal|normal"
		if ef, ok := c.embeddedFonts[keyBase]; ok {
			return nil, ef
		}
	}
	return resolveFont(style), nil
}

// collectText recursively collects all text content from a node.
func collectText(n *html.Node) string {
	var sb strings.Builder
	collectTextInto(n, &sb)
	return collapseWhitespace(sb.String())
}

// collectTextInto appends all text content from n and its descendants to sb.
func collectTextInto(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
		return
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		collectTextInto(child, sb)
	}
}

// collectRawText preserves whitespace (for <pre> elements).
func collectRawText(n *html.Node) string {
	var sb strings.Builder
	collectRawTextInto(n, &sb)
	return sb.String()
}

// collectRawTextInto appends raw text from n and its descendants to sb, preserving whitespace.
func collectRawTextInto(n *html.Node, sb *strings.Builder) {
	if n.Type == html.TextNode {
		sb.WriteString(n.Data)
		return
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		collectRawTextInto(child, sb)
	}
}

// collectDirectText collects text only from direct text node children,
// skipping nested <ul>/<ol> elements (for list item text extraction).
func collectDirectText(n *html.Node) string {
	var sb strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode {
			sb.WriteString(child.Data)
		} else if child.Type == html.ElementNode &&
			child.DataAtom != atom.Ul && child.DataAtom != atom.Ol {
			// Recurse into inline elements but not nested lists.
			collectTextInto(child, &sb)
		}
	}
	return collapseWhitespace(sb.String())
}

// findNestedList finds the first <ul> or <ol> child of a node.
func findNestedList(n *html.Node) *html.Node {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode &&
			(child.DataAtom == atom.Ul || child.DataAtom == atom.Ol) {
			return child
		}
	}
	return nil
}

// collapseWhitespace collapses runs of whitespace into single spaces and trims.
func collapseWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// applyTextTransform applies a CSS text-transform value to a string.
func applyTextTransform(s, transform string) string {
	switch transform {
	case "uppercase":
		return strings.ToUpper(s)
	case "lowercase":
		return strings.ToLower(s)
	case "capitalize":
		return capitalizeWords(s)
	default:
		return s
	}
}

// capitalizeWords capitalizes the first letter of each word.
func capitalizeWords(s string) string {
	var sb strings.Builder
	prevSpace := true
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			prevSpace = true
			sb.WriteRune(r)
		} else if prevSpace {
			sb.WriteRune(toUpperRune(r))
			prevSpace = false
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// toUpperRune converts a single rune to uppercase.
func toUpperRune(r rune) rune {
	s := strings.ToUpper(string(r))
	for _, c := range s {
		return c
	}
	return r
}

// processWhitespace handles whitespace according to the white-space CSS property.
func processWhitespace(s, whiteSpace string) string {
	switch whiteSpace {
	case "pre", "pre-wrap":
		// Preserve whitespace and line breaks.
		return s
	case "pre-line":
		// Collapse spaces/tabs but preserve line breaks.
		var sb strings.Builder
		lines := strings.Split(s, "\n")
		for i, line := range lines {
			if i > 0 {
				sb.WriteByte('\n')
			}
			sb.WriteString(strings.Join(strings.Fields(line), " "))
		}
		return strings.TrimSpace(sb.String())
	default: // "normal", "nowrap"
		return collapseWhitespace(s)
	}
}

// textContent returns the concatenated text of all descendant text nodes.
func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var s string
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		s += textContent(c)
	}
	return strings.TrimSpace(s)
}

// extractMeta extracts metadata from a <meta> element.
func (c *converter) extractMeta(n *html.Node) {
	name := strings.ToLower(getAttr(n, "name"))
	content := getAttr(n, "content")
	if content == "" {
		return
	}
	switch name {
	case "author":
		c.metadata.Author = content
	case "description":
		c.metadata.Description = content
	case "keywords":
		c.metadata.Keywords = content
	case "generator":
		c.metadata.Creator = content
	case "subject":
		c.metadata.Subject = content
	}
}

// getAttr returns the value of the named attribute on n, or the empty string.
func getAttr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

// splitDeclarations splits a CSS style string into individual declarations.
func splitDeclarations(style string) []string {
	return strings.Split(style, ";")
}

// splitDeclaration splits "property: value" into (property, value).
func splitDeclaration(decl string) (string, string) {
	idx := strings.IndexByte(decl, ':')
	if idx < 0 {
		return "", ""
	}
	prop := strings.TrimSpace(decl[:idx])
	val := strings.TrimSpace(decl[idx+1:])
	return strings.ToLower(prop), val
}

// parseInt parses a string to int, returning 0 on failure.
func parseInt(s string) int {
	v, _ := strconv.Atoi(strings.TrimSpace(s))
	return v
}

// parseAttrFloat parses an HTML attribute value as float64 (for width/height attrs).
func parseAttrFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

// parseBackgroundImage parses the CSS background-image value and returns
// the kind ("url", "linear-gradient", "radial-gradient") and the inner value.
func parseBackgroundImage(val string) (kind string, inner string) {
	val = strings.TrimSpace(val)
	lower := strings.ToLower(val)

	if strings.HasPrefix(lower, "url(") {
		inner := extractFunctionArgs(val)
		// Remove surrounding quotes.
		inner = strings.Trim(inner, `"'`)
		return "url", inner
	}
	if strings.HasPrefix(lower, "linear-gradient(") {
		return "linear-gradient", extractFunctionArgs(val)
	}
	if strings.HasPrefix(lower, "radial-gradient(") {
		return "radial-gradient", extractFunctionArgs(val)
	}
	return "", val
}

// extractFunctionArgs extracts the content between the outermost parentheses.
func extractFunctionArgs(val string) string {
	start := strings.IndexByte(val, '(')
	if start < 0 {
		return val
	}
	// Find matching close paren.
	depth := 0
	for i := start; i < len(val); i++ {
		switch val[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return val[start+1 : i]
			}
		}
	}
	return val[start+1:]
}

// parseLinearGradient parses CSS linear-gradient arguments.
// Returns the angle in degrees and the color stops.
func parseLinearGradient(args string) (float64, []layout.GradientStop) {
	// Split on commas, but respect nested parentheses (e.g., rgb()).
	parts := splitGradientArgs(args)
	if len(parts) < 2 {
		return 180, nil
	}

	angle := 180.0 // default: to bottom
	startIdx := 0

	// Check if first part is a direction.
	first := strings.TrimSpace(strings.ToLower(parts[0]))
	if strings.HasPrefix(first, "to ") {
		angle = parseGradientDirection(first)
		startIdx = 1
	} else if strings.HasSuffix(first, "deg") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(first, "deg"), 64); err == nil {
			angle = v
		}
		startIdx = 1
	} else if strings.HasSuffix(first, "rad") {
		if v, err := strconv.ParseFloat(strings.TrimSuffix(first, "rad"), 64); err == nil {
			angle = v * 180 / math.Pi
		}
		startIdx = 1
	}

	colorParts := parts[startIdx:]
	stops := parseGradientStops(colorParts)

	return angle, stops
}

// parseRadialGradient parses CSS radial-gradient arguments.
// Returns the color stops (center ellipse is assumed).
func parseRadialGradient(args string) []layout.GradientStop {
	parts := splitGradientArgs(args)
	if len(parts) < 2 {
		return nil
	}

	startIdx := 0
	// Skip shape/size keywords.
	first := strings.TrimSpace(strings.ToLower(parts[0]))
	if first == "circle" || first == "ellipse" ||
		strings.HasPrefix(first, "circle ") || strings.HasPrefix(first, "ellipse ") ||
		strings.Contains(first, "closest") || strings.Contains(first, "farthest") {
		startIdx = 1
	}

	return parseGradientStops(parts[startIdx:])
}

// splitGradientArgs splits a gradient argument string on commas,
// respecting nested parentheses (e.g., rgb(1,2,3)).
func splitGradientArgs(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	if start < len(s) {
		parts = append(parts, strings.TrimSpace(s[start:]))
	}
	return parts
}

// parseGradientDirection converts "to right", "to bottom left", etc. to degrees.
func parseGradientDirection(dir string) float64 {
	dir = strings.TrimPrefix(dir, "to ")
	dir = strings.TrimSpace(dir)
	switch dir {
	case "top":
		return 0
	case "right":
		return 90
	case "bottom":
		return 180
	case "left":
		return 270
	case "top right":
		return 45
	case "top left":
		return 315
	case "bottom right":
		return 135
	case "bottom left":
		return 225
	default:
		return 180
	}
}

// parseGradientStops parses a slice of "color [position]" strings into GradientStops.
func parseGradientStops(parts []string) []layout.GradientStop {
	var stops []layout.GradientStop
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		// Try to split into color + position.
		// The position is the last token if it ends with %.
		stop := layout.GradientStop{}
		tokens := strings.Fields(p)

		if len(tokens) >= 2 {
			last := tokens[len(tokens)-1]
			if strings.HasSuffix(last, "%") {
				if v, err := strconv.ParseFloat(strings.TrimSuffix(last, "%"), 64); err == nil {
					stop.Position = v / 100
				}
				colorStr := strings.Join(tokens[:len(tokens)-1], " ")
				if clr, ok := parseColor(colorStr); ok {
					stop.Color = clr
				}
			} else {
				// All tokens are the color.
				if clr, ok := parseColor(p); ok {
					stop.Color = clr
				}
			}
		} else {
			if clr, ok := parseColor(p); ok {
				stop.Color = clr
			}
		}

		stops = append(stops, stop)
	}

	return stops
}

// parseBackgroundPosition converts CSS background-position keywords to [x, y]
// fractions in [0, 1].
func parseBgPosition(val string) [2]float64 {
	val = strings.TrimSpace(strings.ToLower(val))
	if val == "" {
		return [2]float64{0, 0}
	}

	parts := strings.Fields(val)

	toFrac := func(s string) (float64, bool) {
		switch s {
		case "left":
			return 0, true
		case "center":
			return 0.5, true
		case "right":
			return 1, true
		case "top":
			return 0, true
		case "bottom":
			return 1, true
		}
		if strings.HasSuffix(s, "%") {
			if v, err := strconv.ParseFloat(strings.TrimSuffix(s, "%"), 64); err == nil {
				return v / 100, true
			}
		}
		return 0, false
	}

	if len(parts) == 1 {
		if parts[0] == "center" {
			return [2]float64{0.5, 0.5}
		}
		if f, ok := toFrac(parts[0]); ok {
			// Single keyword: "left" = 0, 0.5; "top" = 0.5, 0
			switch parts[0] {
			case "top", "bottom":
				return [2]float64{0.5, f}
			default:
				return [2]float64{f, 0.5}
			}
		}
		return [2]float64{0, 0}
	}

	x, y := 0.0, 0.0
	if f, ok := toFrac(parts[0]); ok {
		x = f
	}
	if f, ok := toFrac(parts[1]); ok {
		y = f
	}
	return [2]float64{x, y}
}

// resolveBackgroundImage resolves a background-image CSS value into a layout.BackgroundImage.
// Returns nil if the value cannot be resolved.
func (c *converter) resolveBackgroundImage(style computedStyle) *layout.BackgroundImage {
	if style.BackgroundImage == "" {
		return nil
	}

	kind, inner := parseBackgroundImage(style.BackgroundImage)
	var img *folioimage.Image

	switch kind {
	case "url":
		imgPath := inner
		if strings.HasPrefix(imgPath, "http://") || strings.HasPrefix(imgPath, "https://") {
			loaded, err := fetchImage(imgPath)
			if err != nil {
				return nil
			}
			img = loaded
		} else {
			if !filepath.IsAbs(imgPath) && c.opts.BasePath != "" {
				imgPath = filepath.Join(c.opts.BasePath, imgPath)
			}
			loaded, err := loadImage(imgPath)
			if err != nil {
				return nil
			}
			img = loaded
		}

	case "linear-gradient":
		angle, stops := parseLinearGradient(inner)
		if len(stops) < 2 {
			return nil
		}
		// Render at a reasonable resolution.
		w, h := 200, 200
		rgba := layout.RenderLinearGradient(w, h, angle, stops)
		img = folioimage.NewFromGoImage(rgba)

	case "radial-gradient":
		stops := parseRadialGradient(inner)
		if len(stops) < 2 {
			return nil
		}
		w, h := 200, 200
		rgba := layout.RenderRadialGradient(w, h, stops)
		img = folioimage.NewFromGoImage(rgba)

	default:
		return nil
	}

	if img == nil {
		return nil
	}

	bgImg := &layout.BackgroundImage{
		Image:    img,
		Size:     style.BackgroundSize,
		Position: parseBgPosition(style.BackgroundPosition),
		Repeat:   style.BackgroundRepeat,
	}

	// Parse explicit size values.
	if style.BackgroundSize != "" && style.BackgroundSize != "cover" && style.BackgroundSize != "contain" && style.BackgroundSize != "auto" {
		parts := strings.Fields(style.BackgroundSize)
		if len(parts) >= 1 {
			if l := parseLength(parts[0]); l != nil {
				bgImg.SizeW = l.toPoints(0, style.FontSize)
			}
		}
		if len(parts) >= 2 {
			if l := parseLength(parts[1]); l != nil {
				bgImg.SizeH = l.toPoints(0, style.FontSize)
			}
		}
	}

	if bgImg.Repeat == "" {
		bgImg.Repeat = "repeat"
	}

	return bgImg
}
