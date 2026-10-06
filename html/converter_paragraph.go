// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// convertParagraph creates a layout.Paragraph from a <p> element.
func (c *converter) convertParagraph(n *html.Node, style computedStyle) []layout.Element {
	runs := c.collectRuns(n, style)
	if len(runs) == 0 {
		return nil
	}

	// Split runs at <br> markers (TextRun with IsLineBreak) into line groups.
	groups := splitRunsAtBr(runs)

	var elems []layout.Element
	for i, group := range groups {
		if len(group) == 0 {
			continue
		}
		p := c.buildParagraphFromRuns(group, style)
		// Only apply top margin to first paragraph, bottom margin to last.
		if i == 0 {
			if mt := style.MarginTopAt(c.containerWidth); mt > 0 {
				p.SetSpaceBefore(mt)
			}
		}
		if i == len(groups)-1 {
			if mb := style.MarginBottomAt(c.containerWidth); mb > 0 {
				p.SetSpaceAfter(mb)
			}
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
		// The Div now owns and draws the box background (honoring
		// border-radius); clear the redundant paragraph-level background on
		// the wrapped children to avoid a square-cornered overdraw (issue #329).
		if style.BackgroundColor != nil {
			clearMatchingParagraphBackgrounds(div.Children(), *style.BackgroundColor)
		}
		return []layout.Element{div}
	}

	return elems
}

// splitRunsAtBr splits a flat slice of TextRuns into groups separated by
// line-break markers (from <br> tags). Each group becomes a separate paragraph.
func splitRunsAtBr(runs []layout.TextRun) [][]layout.TextRun {
	var groups [][]layout.TextRun
	var current []layout.TextRun
	for _, r := range runs {
		if r.IsLineBreak {
			groups = append(groups, current)
			current = nil
			continue
		}
		current = append(current, r)
	}
	groups = append(groups, current)
	return groups
}

// allRunsNoWrap reports whether every non-whitespace text run in the group
// carries the NoWrap flag (set when its source style was white-space:nowrap).
// Inline elements, line-break markers, and whitespace-only runs are ignored:
// whitespace between the block edge and a nowrap inline element (from
// pretty-printed markup, e.g. "<p>\n  <b>tok</b>\n</p>") collapses to a " "
// run that a browser trims at the line edge, so it must not veto the verdict.
// Returns false for a group with no non-whitespace text run, so an empty or
// element-only paragraph is not spuriously marked nowrap.
func allRunsNoWrap(runs []layout.TextRun) bool {
	sawText := false
	for _, r := range runs {
		if r.InlineElement != nil || r.IsLineBreak || strings.TrimSpace(r.Text) == "" {
			continue
		}
		sawText = true
		if !r.NoWrap {
			return false
		}
	}
	return sawText
}

// buildParagraphFromRuns creates a styled paragraph from a slice of TextRuns.
func (c *converter) buildParagraphFromRuns(runs []layout.TextRun, style computedStyle) *layout.Paragraph {
	// Always use NewStyledParagraph to preserve all TextRun fields
	// (BaselineShift, BackgroundColor, Decoration, etc.). The NewParagraph
	// fast path was a premature optimization that discarded per-run styling.
	p := layout.NewStyledParagraph(runs...)

	if style.TextAlignSet {
		p.SetAlign(resolveTextAlign(style))
	}
	if style.Direction != layout.DirectionAuto {
		p.SetDirection(style.Direction)
	}
	if style.TextAlignLastSet {
		p.SetTextAlignLast(resolveTextAlignLast(style))
	}
	p.SetLeading(style.LineHeight)
	if style.StringSetName != "" {
		value := style.StringSetValue
		if strings.Contains(value, "content()") {
			// Extract text from the runs (we don't have the HTML node here).
			var textParts []string
			for _, r := range runs {
				if r.Text != "" {
					textParts = append(textParts, r.Text)
				}
			}
			value = strings.ReplaceAll(value, "content()", strings.Join(textParts, ""))
		}
		value = strings.Trim(value, `"'`)
		p.SetStringSet(style.StringSetName, value)
	}
	if style.TextIndent != 0 {
		p.SetFirstLineIndent(style.TextIndent)
	}
	if style.BackgroundColor != nil {
		p.SetBackground(*style.BackgroundColor)
	}
	if style.TextOverflow == "ellipsis" && style.Overflow == "hidden" {
		p.SetEllipsis(true)
	}
	if style.WordBreak == "break-all" || style.WordBreak == "break-word" || style.WordBreak == "keep-all" {
		p.SetWordBreak(style.WordBreak)
	}
	// white-space:nowrap is an inherited property that can be set on the
	// block itself or on an inline element carrying the text (e.g.
	// <div><b style="white-space:nowrap">token</b></div>). The block style
	// covers the former; for the latter, treat the paragraph as nowrap when
	// all of its text runs come from a nowrap context. (A paragraph mixing
	// nowrap and wrapping runs keeps wrapping — folio's nowrap is a
	// paragraph-level flag, so per-run nowrap on part of a mixed line is a
	// known limitation.)
	if style.WhiteSpace == "nowrap" || allRunsNoWrap(runs) {
		p.SetNoWrap(true)
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
		TextShadow:      textShadowFromStyle(style),
	}
	p := layout.NewStyledParagraph(run)
	if style.TextAlignSet {
		p.SetAlign(resolveTextAlign(style))
	}
	if style.Direction != layout.DirectionAuto {
		p.SetDirection(style.Direction)
	}
	p.SetLeading(style.LineHeight)
	return []layout.Element{p}
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
	hr.SetSpaceBefore(style.MarginTopAt(c.containerWidth))
	hr.SetSpaceAfter(style.MarginBottomAt(c.containerWidth))

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

// convertInlineContainer handles inline elements like <span>, <em>, <strong>.
// Collects text runs from children and wraps in a paragraph.
func (c *converter) convertInlineContainer(n *html.Node, style computedStyle) []layout.Element {
	runs := c.collectRuns(n, style)
	if len(runs) == 0 {
		return nil
	}
	var elems []layout.Element
	for _, group := range splitRunsAtBr(runs) {
		if len(group) == 0 {
			continue
		}
		p := c.buildParagraphFromRuns(group, style)
		elems = append(elems, p)
	}
	return elems
}

// collectRuns gathers inline content as TextRuns, recursing into inline children.
// Images, SVGs, and elements with display:inline-block are converted to inline
// element runs that flow within the paragraph like words.
func (c *converter) collectRuns(n *html.Node, style computedStyle) []layout.TextRun {
	var runs []layout.TextRun
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		runs = append(runs, c.collectRunsFromNode(child, style)...)
	}
	return runs
}

// collectRunsFromNode converts a single sibling node (text or inline element)
// into zero or more TextRuns, using parentStyle as the enclosing style. This
// is the per-node body of collectRuns extracted so that walkChildren can
// group consecutive inline siblings of a block container into a single
// anonymous block box (CSS 2.1 §9.2.1.1) instead of producing one paragraph
// per text/inline element and losing text shaping, alignment, and wrap.
func (c *converter) collectRunsFromNode(child *html.Node, parentStyle computedStyle) []layout.TextRun {
	switch child.Type {
	case html.TextNode:
		// Use inline-aware whitespace collapsing that preserves
		// leading/trailing spaces per CSS Text Module Level 3 §4.1.1.
		// Block-level contexts use processWhitespace which strips them.
		var text string
		if parentStyle.WhiteSpace == "pre" || parentStyle.WhiteSpace == "pre-wrap" || parentStyle.WhiteSpace == "pre-line" {
			text = processWhitespace(child.Data, parentStyle.WhiteSpace)
		} else {
			text = collapseWhitespaceInline(child.Data)
		}
		if text == "" {
			return nil
		}
		text = applyTextTransform(text, parentStyle.TextTransform)
		return c.splitTextByFont(text, parentStyle)
	case html.ElementNode:
		if child.DataAtom == atom.Br {
			// Insert a line-break marker that splitRunsAtBr splits on.
			return []layout.TextRun{{IsLineBreak: true}}
		}
		childStyle := c.computeElementStyle(child, parentStyle)

		// Images and SVGs are inline by default in HTML (replaced
		// elements). Honor display overrides: "none" hides them,
		// "block" skips them in inline flow (they would need to be
		// handled as block-level elements outside the paragraph).
		// All other display values (default "", "inline",
		// "inline-block") are treated as inline.
		if child.DataAtom == atom.Img || child.DataAtom == atom.Svg {
			if childStyle.Display == "none" || childStyle.Display == "block" {
				return nil
			}
			if el := c.convertInlineElement(child, childStyle); el != nil {
				return []layout.TextRun{{InlineElement: el}}
			}
			return nil
		}

		// Elements with display:inline-block flow inline as atomic boxes.
		if childStyle.Display == "inline-block" {
			if el := c.convertInlineBlockElement(child, childStyle); el != nil {
				return []layout.TextRun{{InlineElement: el}}
			}
			return nil
		}

		childRuns := c.collectRuns(child, childStyle)
		// Propagate href from <a> elements to all child runs.
		if child.DataAtom == atom.A {
			href := getAttr(child, "href")
			if href != "" {
				for i := range childRuns {
					childRuns[i].LinkURI = href
				}
			}
		}
		return childRuns
	}
	return nil
}

// convertInlineElement converts an <img> or <svg> node into a single
// layout.Element suitable for inline placement within a paragraph.
func (c *converter) convertInlineElement(n *html.Node, style computedStyle) layout.Element {
	var elems []layout.Element
	switch n.DataAtom {
	case atom.Img:
		elems = c.convertImage(n, style)
	case atom.Svg:
		elems = c.convertSVG(n, style)
	}
	if len(elems) > 0 {
		return elems[0]
	}
	return nil
}

// convertInlineBlockElement converts a display:inline-block element into a
// layout.Element suitable for inline placement within a paragraph.
func (c *converter) convertInlineBlockElement(n *html.Node, style computedStyle) layout.Element {
	elems := c.convertBlock(n, style)
	if len(elems) == 0 {
		return nil
	}
	el := elems[0]
	// Enable shrink-to-fit (CSS fit-content) so the inline-block sizes to its
	// content and flows inline, instead of filling the whole paragraph width.
	// An explicit CSS width still wins (SetShrinkToFit only applies when no
	// explicit width is set, handled in Div.PlanLayout).
	if div, ok := el.(*layout.Div); ok {
		div.SetShrinkToFit(true)
	}
	return el
}

// collectListItemRuns collects styled TextRuns from a <li> element,
// skipping nested <ul>/<ol> elements (which are handled as sub-lists).
func (c *converter) collectListItemRuns(li *html.Node, style computedStyle) []layout.TextRun {
	var runs []layout.TextRun
	for child := li.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode &&
			(child.DataAtom == atom.Ul || child.DataAtom == atom.Ol) {
			continue // skip nested lists
		}
		switch child.Type {
		case html.TextNode:
			text := processWhitespace(child.Data, style.WhiteSpace)
			if text == "" {
				continue
			}
			stdFont, embFont := c.resolveFontForText(style, text)
			run := c.makeTextRun(text, stdFont, embFont, style)
			// The li's own background paints as a block background, not a
			// per-word text highlight.
			run.BackgroundColor = nil
			runs = append(runs, run)
		case html.ElementNode:
			childStyle := c.computeElementStyle(child, style)
			childRuns := c.collectRuns(child, childStyle)
			if child.DataAtom == atom.A {
				href := getAttr(child, "href")
				if href != "" {
					for i := range childRuns {
						childRuns[i].LinkURI = href
					}
				}
			}
			runs = append(runs, childRuns...)
		}
	}
	return runs
}
