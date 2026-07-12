// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// containingBlock tracks a positioned ancestor for absolute positioning resolution.
type containingBlock struct {
	pending []pendingOverlay // absolute children waiting to be attached to the Div
	width   float64          // resolved content width in points
	height  float64          // resolved content height in points (0 if unknown)
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

	// UA default (CSS Lists 3 §6.1): every list root implicitly resets the
	// "list-item" counter to 0, so counter(list-item) in ::marker numbers
	// items and nested lists restart at 1. An explicit author counter-reset
	// for "list-item" overrides this.
	if n.DataAtom == atom.Ul || n.DataAtom == atom.Ol {
		style.CounterReset = withImplicitListItemReset(style.CounterReset)
	}

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
