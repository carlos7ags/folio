// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

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

	// Apply ::marker pseudo-element styles from <li> children.
	// Check the first <li> for ::marker declarations and apply to the list.
	if c.sheet != nil {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.ElementNode && child.DataAtom == atom.Li {
				markerDecls := c.sheet.matchingPseudoElementDeclarations(child, "marker")
				for _, d := range markerDecls {
					switch d.property {
					case "color":
						if clr, ok := parseColor(d.value); ok {
							list.SetMarkerColor(clr)
						}
					case "font-size":
						fs := parseFontSize(d.value, style.FontSize)
						if fs > 0 {
							list.SetMarkerFontSize(fs)
						}
					}
				}
				break // only need to check the first <li>
			}
		}
	}

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

	// Honor the <ol start="N"> attribute so ordered-list numbering begins at
	// N (and, via List.overflowFrom, continues correctly across page breaks).
	// SetStart clamps values below 1 to 1: zero/negative ordinals (which the
	// HTML spec technically permits) are intentionally not rendered, since the
	// alpha/roman marker styles have no representation for them.
	// <reversed> is not yet supported.
	if ordered {
		if s := getAttr(n, "start"); s != "" {
			if start, err := strconv.Atoi(strings.TrimSpace(s)); err == nil {
				list.SetStart(start)
			}
		}
	}

	// Propagate text direction to the list so markers position correctly
	// and item paragraphs inherit the direction for bidi reordering.
	if style.Direction != layout.DirectionAuto {
		list.SetDirection(style.Direction)
	}

	// list-style-position: inside flows the marker inline with the first
	// content line; outside (the default) keeps it in the left gutter.
	if style.ListStylePosition == "inside" {
		list.SetMarkerInside(true)
	}

	// FlushListMarkers (document-wide option) draws all nested markers in one
	// left column instead of indenting per level. Sub-lists inherit it via the
	// AddItem*WithSubList constructors.
	if c.opts.FlushListMarkers {
		list.SetMarkerFlush(true)
	}

	c.populateList(n, list, style)

	return []layout.Element{list}
}

// populateList fills a list with items from <li> children, handling nesting.
//
// Each <li> takes one of two paths:
//
//   - Fast (inline) path: the <li> has no box-model styles and contains only
//     inline content (optionally followed by a nested <ul>/<ol>). The item
//     renders as styled TextRuns with the marker inline, exactly as before.
//     Uses collectListItemRuns so inline elements like <a href="..."> are
//     preserved as styled TextRuns with LinkURI.
//
//   - Element path: the <li> has block-level flow children (<div>, <p>, <br>,
//     display:block) and/or its own box-model styles. The <li>'s children are
//     run through the normal block-flow converter (walkChildren) and wrapped
//     in a Div, so block children flow onto separate lines and any
//     background/border/border-radius/padding on the <li> is painted. The
//     marker is aligned to the first text line of the element.
func (c *converter) populateList(n *html.Node, list *layout.List, style computedStyle) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || child.DataAtom != atom.Li {
			continue
		}

		liStyle := c.computeElementStyle(child, style)
		hasBox := liHasBoxModel(liStyle)
		nestedList := findNestedList(child)

		// Apply the <li>'s own counters before its content is resolved so a
		// counter-increment fires for each item and any counter-reset scopes
		// the subtree. convertElement does this for the element path's block
		// children, but the fast path and the li itself bypass it, so we mirror
		// the reset-then-increment ordering here.
		for _, cr := range liStyle.CounterReset {
			c.resetCounter(cr.Name, cr.Value)
		}
		for _, ci := range liStyle.CounterIncrement {
			c.incrementCounter(ci.Name, ci.Value)
		}

		// Resolve the li::marker { content } string now that this item's
		// counters reflect counter-reset/increment, so counter()/counters() in
		// the marker render the right values. Applied per path below once the
		// item has been appended.
		markerText, hasMarker := c.resolveMarkerContent(child)

		// Fast path: plain inline item (optionally with a nested list as a
		// sub-list) and no box-model styles. Preserves existing rendering
		// and indentation for the common case. Use if/else (not continue) so
		// the shared counter pop below runs for both paths.
		if !hasBox && !liHasBlockFlowChildren(c, child, liStyle) {
			runs := c.collectListItemRuns(child, style)
			if nestedList != nil {
				if len(runs) == 0 {
					runs = []layout.TextRun{{Text: " ", Font: font.Helvetica, FontSize: style.FontSize}}
				}
				sub := list.AddItemRunsWithSubList(runs)
				if hasMarker {
					list.SetLastItemMarker(markerText)
				}
				if nestedList.DataAtom == atom.Ol {
					sub.SetStyle(layout.ListOrdered)
				}
				// The fast path bypasses convertElement for the nested
				// list, so apply the nested list's own counter-reset and
				// counter-increment around the recursion. The element path's
				// nested list goes through convertElement, which already
				// handles both.
				nestedStyle := c.computeElementStyle(nestedList, style)
				for _, cr := range nestedStyle.CounterReset {
					c.resetCounter(cr.Name, cr.Value)
				}
				for _, ci := range nestedStyle.CounterIncrement {
					c.incrementCounter(ci.Name, ci.Value)
				}
				c.populateList(nestedList, sub, style)
				for _, cr := range nestedStyle.CounterReset {
					c.popCounter(cr.Name)
				}
			} else if len(runs) > 0 {
				list.AddItemRuns(runs)
				if hasMarker {
					list.SetLastItemMarker(markerText)
				}
			}
		} else {
			// Element path: convert the <li>'s children via the normal
			// block-flow path so block elements, <br>, and nested lists lay
			// out correctly. Apply the <li>'s own box styles when present.
			c.addElementListItem(child, list, liStyle, hasBox)
			if hasMarker {
				list.SetLastItemMarker(markerText)
			}
		}

		// Pop the li's own counter-reset after its subtree is processed so
		// sibling items see the restored nesting (runs on both paths).
		for _, cr := range liStyle.CounterReset {
			c.popCounter(cr.Name)
		}
	}
}

// resolveMarkerContent reads the li::marker { content } declaration for an
// <li> and resolves it (counter()/counters()/strings). The bool reports whether
// a content declaration exists at all: an explicit `content: none` (or empty)
// returns ("", true) to suppress the marker, while the absence of any content
// declaration returns ("", false) so the default style-derived marker stands.
func (c *converter) resolveMarkerContent(li *html.Node) (string, bool) {
	if c.sheet == nil {
		return "", false
	}
	// matchingPseudoElementDeclarations sorts ascending by specificity (stable
	// for ties), so the last matching `content` declaration is the cascade
	// winner. Take it — matching the last-wins color/font-size handling in
	// convertList rather than returning on the first (lowest-specificity) decl.
	decls := c.sheet.matchingPseudoElementDeclarations(li, "marker")
	text, found := "", false
	for _, d := range decls {
		if d.property != "content" {
			continue
		}
		found = true
		if val := strings.TrimSpace(d.value); val == "none" || val == "" {
			text = ""
		} else {
			text = c.resolveContentValue(val)
		}
	}
	return text, found
}

// addElementListItem converts an <li> with block-level children and/or
// box-model styles into a rich-element list item.
func (c *converter) addElementListItem(li *html.Node, list *layout.List, liStyle computedStyle, hasBox bool) {
	children := c.walkChildren(li, liStyle)
	if len(children) == 0 {
		// Nothing renderable; still emit an (empty) marker for an empty <li>.
		list.AddItemElement(layout.NewDiv())
		return
	}

	div := layout.NewDiv()
	if hasBox {
		// The element item is laid out in the content column (to the right of
		// the marker), i.e. the available width minus the list indent. Resolve
		// percentage box-model values (padding, border, border-radius) against
		// that narrower width so they match where the box is actually placed
		// rather than overflowing against the full container width.
		contentWidth := c.containerWidth - list.Indent()
		if contentWidth < 0 {
			contentWidth = 0
		}
		applyDivStyles(div, liStyle, contentWidth)

		// The Div now owns and draws the box background (honoring border-radius).
		// Clear the redundant block-level/run background that the <li>'s
		// background propagated onto the inner content paragraph and its text
		// runs, so it does not re-draw a square fill over the rounded corners
		// (same double-paint #340 fixed for convertBlock/blockquote/cells). The
		// content may sit directly in the Div (a badge's single Paragraph) or
		// nested inside an inner wrapper Div (block children), so clear at every
		// depth. The children are cleared before being added below.
		if liStyle.BackgroundColor != nil {
			clearMatchingBackgroundsRecursive(children, *liStyle.BackgroundColor)
		}
	}
	// An inline-block <li> (e.g. a badge "circle around a number") should HUG
	// its content rather than fill the content column. Enable shrink-to-fit
	// (CSS fit-content); an explicit CSS width still wins, since
	// Div.PlanLayout resolves the width unit before applying shrink-to-fit
	// (and applyDivStyles already set the width unit from liStyle.Width).
	if liStyle.Display == "inline-block" {
		div.SetShrinkToFit(true)
	}
	for _, e := range children {
		div.Add(e)
	}
	list.AddItemElement(div)
}

// liHasBoxModel reports whether an <li>'s computed style carries box-model
// decoration that requires a Div wrapper to render (background, border,
// border-radius, or padding).
func liHasBoxModel(s computedStyle) bool {
	return s.hasPadding() || s.hasBorder() || s.hasBorderRadius() ||
		s.BackgroundColor != nil
}

// liHasBlockFlowChildren reports whether an <li> contains any child that
// participates in block flow (block element, <br>, or display:block), which
// requires the block-flow conversion path rather than inline text runs. A
// nested <ul>/<ol> alone does not count: it is handled by the sub-list fast
// path.
func liHasBlockFlowChildren(c *converter, li *html.Node, liStyle computedStyle) bool {
	for child := li.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		// A nested list is handled by the sub-list fast path.
		if child.DataAtom == atom.Ul || child.DataAtom == atom.Ol {
			continue
		}
		if child.DataAtom == atom.Br {
			return true
		}
		if !c.isInlineFlowChild(child, liStyle) {
			return true
		}
	}
	return false
}
