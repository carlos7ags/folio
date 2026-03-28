// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// baselineShiftFromStyle computes the vertical baseline offset for
// CSS vertical-align values like "super", "sub", "text-top", "text-bottom".
// textShadowFromStyle converts a CSS text-shadow to a layout.TextShadow.
func textShadowFromStyle(style computedStyle) *layout.TextShadow {
	if style.TextShadow == nil {
		return nil
	}
	return &layout.TextShadow{
		OffsetX: style.TextShadow.OffsetX,
		OffsetY: style.TextShadow.OffsetY,
		Blur:    style.TextShadow.Blur,
		Color:   style.TextShadow.Color,
	}
}

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
	// Auto-calculate column count from column-width if column-count is not set.
	if style.ColumnCount <= 1 && style.ColumnWidth > 0 && c.containerWidth > 0 {
		gap := style.ColumnGap
		if gap == 0 {
			gap = 12 // default column gap
		}
		style.ColumnCount = int((c.containerWidth + gap) / (style.ColumnWidth + gap))
		if style.ColumnCount < 1 {
			style.ColumnCount = 1
		}
	}
	if style.ColumnCount > 1 && len(children) > 0 {
		return c.buildColumns(children, style)
	}

	// If no box-model properties, skip the Div wrapper.
	hasWidthConstraints := style.Width != nil || style.MaxWidth != nil || style.MinWidth != nil
	hasHeightConstraints := style.Height != nil || style.MinHeight != nil || style.MaxHeight != nil
	hasVisualEffects := style.BorderRadius > 0 || style.BorderRadiusTL > 0 || style.BorderRadiusTR > 0 || style.BorderRadiusBR > 0 || style.BorderRadiusBL > 0 || (style.Opacity > 0 && style.Opacity < 1) || style.Overflow == "hidden"
	hasBoxShadow := len(style.BoxShadows) > 0
	hasOutline := style.OutlineWidth > 0
	hasTransform := style.Transform != "" && strings.ToLower(strings.TrimSpace(style.Transform)) != "none"
	hasBgImage := style.BackgroundImage != ""
	if !style.hasPadding() && !style.hasBorder() && !style.hasMargin() && style.BackgroundColor == nil && !hasWidthConstraints && !hasHeightConstraints && !hasVisualEffects && !hasBoxShadow && !hasOutline && !hasTransform && !hasBgImage {
		return children
	}

	// If any child is an AreaBreak, split into multiple Divs separated
	// by the breaks. AreaBreak only works at the top level (the renderer
	// checks for it by type assertion), so burying one inside a Div
	// would silently suppress the page break.
	if containsAreaBreak(children) {
		return c.splitOnAreaBreaks(children, style)
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

// containsAreaBreak reports whether any element in the slice is an AreaBreak.
func containsAreaBreak(elems []layout.Element) bool {
	for _, e := range elems {
		if _, ok := e.(*layout.AreaBreak); ok {
			return true
		}
	}
	return false
}

// splitOnAreaBreaks produces a sequence of Divs separated by AreaBreak
// elements. Each Div gets the same styles applied. This ensures AreaBreak
// elements appear at the top level where the renderer can act on them.
func (c *converter) splitOnAreaBreaks(children []layout.Element, style computedStyle) []layout.Element {
	var result []layout.Element
	var group []layout.Element

	flush := func() {
		if len(group) == 0 {
			return
		}
		div := layout.NewDiv()
		for _, child := range group {
			div.Add(child)
		}
		applyDivStyles(div, style, c.containerWidth)
		if bgImg := c.resolveBackgroundImage(style); bgImg != nil {
			div.SetBackgroundImage(bgImg)
		}
		result = append(result, div)
		group = nil
	}

	for _, child := range children {
		if _, ok := child.(*layout.AreaBreak); ok {
			flush()
			result = append(result, child)
		} else {
			group = append(group, child)
		}
	}
	flush()

	return result
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
	if style.BorderRadiusTL > 0 || style.BorderRadiusTR > 0 || style.BorderRadiusBR > 0 || style.BorderRadiusBL > 0 {
		div.SetBorderRadiusPerCorner(style.BorderRadiusTL, style.BorderRadiusTR, style.BorderRadiusBR, style.BorderRadiusBL)
	} else if style.BorderRadius > 0 {
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
	for _, bs := range style.BoxShadows {
		div.AddBoxShadow(layout.BoxShadow{
			OffsetX: bs.OffsetX,
			OffsetY: bs.OffsetY,
			Blur:    bs.Blur,
			Spread:  bs.Spread,
			Color:   bs.Color,
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
	if style.PageBreakInside == "avoid" {
		div.SetKeepTogether(true)
	}
}

// buildColumns creates a layout.Columns element from children and style.
func (c *converter) buildColumns(children []layout.Element, style computedStyle) []layout.Element {
	cols := layout.NewColumns(style.ColumnCount)
	if style.ColumnGap > 0 {
		cols.SetGap(style.ColumnGap)
	}
	if style.ColumnRuleWidth > 0 {
		cols.SetColumnRule(layout.ColumnRule{
			Width: style.ColumnRuleWidth,
			Color: style.ColumnRuleColor,
			Style: style.ColumnRuleStyle,
		})
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
	if style.hasPadding() {
		div.SetPaddingAll(layout.Padding{
			Top:    style.PaddingTop,
			Right:  style.PaddingRight,
			Bottom: style.PaddingBottom,
			Left:   style.PaddingLeft,
		})
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
