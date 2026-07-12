// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"sort"
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
)

// convertFlex converts a display:flex container into a layout.Flex.
func (c *converter) convertFlex(n *html.Node, style computedStyle) []layout.Element {
	// Save the parent's containing-block width before narrowing.
	// Margin / padding percentages on the flex container itself
	// resolve against the containing block (CSS 2.1 §8.4), not
	// against the flex's own post-narrow content box.
	parentContainerWidth := c.containerWidth
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

	// Map align-content (cross-axis distribution for wrapped lines).
	switch style.AlignContent {
	case "flex-end", "end":
		flex.SetAlignContent(layout.JustifyFlexEnd)
	case "center":
		flex.SetAlignContent(layout.JustifyCenter)
	case "space-between":
		flex.SetAlignContent(layout.JustifySpaceBetween)
	case "space-around":
		flex.SetAlignContent(layout.JustifySpaceAround)
	case "space-evenly":
		flex.SetAlignContent(layout.JustifySpaceEvenly)
	}

	if style.Gap > 0 {
		flex.SetGap(style.Gap)
	}
	if style.ColumnGap > 0 && style.Gap == 0 {
		flex.SetColumnGap(style.ColumnGap)
	}

	if style.hasPadding() {
		flex.SetPaddingAll(layout.Padding{
			Top:    style.PaddingTopAt(parentContainerWidth),
			Right:  style.PaddingRightAt(parentContainerWidth),
			Bottom: style.PaddingBottomAt(parentContainerWidth),
			Left:   style.PaddingLeftAt(parentContainerWidth),
		})
	}
	if style.hasBorder() {
		flex.SetBorders(buildCellBorders(style))
	}
	if style.BackgroundColor != nil {
		flex.SetBackground(*style.BackgroundColor)
	}
	if mt := style.MarginTopAt(parentContainerWidth); mt > 0 {
		flex.SetSpaceBefore(mt)
	}
	if mb := style.MarginBottomAt(parentContainerWidth); mb > 0 {
		flex.SetSpaceAfter(mb)
	}

	// Add children as flex items.
	// Each direct HTML child becomes exactly one flex item, even if
	// convertNode returns multiple layout elements (e.g. text with <br>).
	//
	// Children are collected first and then stable-sorted by the CSS
	// `order` property before being added to the Flex container. The
	// stable sort preserves DOM order for equal `order` values, which
	// matches CSS Flexbox spec behavior. Children without `order`
	// have the default value 0.
	type pendingChild struct {
		elem  layout.Element   // non-nil if this child is added as a plain element
		item  *layout.FlexItem // non-nil if this child needs FlexItem wrapping
		order int
	}
	var pending []pendingChild
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		// Skip whitespace-only text nodes inside flex containers (CSS spec:
		// whitespace-only text in flex containers does not generate flex items).
		if child.Type == html.TextNode {
			if strings.TrimSpace(child.Data) == "" {
				continue
			}
		}

		childStyle := style // default
		if child.Type == html.ElementNode {
			childStyle = c.computeElementStyle(child, style)
		} else {
			// Text-node children don't carry their own CSS, but they
			// must not inherit non-inherited properties from the flex
			// container (notably Order, which would otherwise leak and
			// reorder text-node siblings alongside element siblings).
			childStyle.Order = 0
		}

		// A flex item is blockified (CSS Flexbox §4). When an inline element
		// (e.g. a bare <span>) carries a visible box (background, border,
		// and/or border-radius), the bare inline path drops its padding,
		// border, and radius. Route it through convertBlock so it renders its
		// full box model identically to display:inline-block (issue #329).
		// No-op for spans without a box, which stay bare inline Paragraphs.
		childElems, blockified := c.blockifyInlineBoxChild(child, childStyle)
		if !blockified {
			childElems = c.convertNode(child, style)
		}
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

		// Resolve the child's margins against the flex container's
		// content-box width (the narrowed c.containerWidth). The
		// child's containing block is the flex's content box per
		// CSS Flexbox §4.
		childMT := childStyle.MarginTopAt(c.containerWidth)
		childMR := childStyle.MarginRightAt(c.containerWidth)
		childMB := childStyle.MarginBottomAt(c.containerWidth)
		childML := childStyle.MarginLeftAt(c.containerWidth)

		// Check if child has any margin (including negative) that needs FlexItem handling.
		hasMargins := childMT != 0 || childMB != 0 || childML != 0 || childMR != 0

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
				item.SetMargins(childMT, childMR, childMB, childML)
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
			pending = append(pending, pendingChild{order: childStyle.Order, item: item})
		} else {
			pending = append(pending, pendingChild{order: childStyle.Order, elem: elem})
		}
	}

	// Stable sort by order so DOM order is preserved for equal values.
	sort.SliceStable(pending, func(i, j int) bool {
		return pending[i].order < pending[j].order
	})
	for _, p := range pending {
		if p.item != nil {
			flex.AddItem(p.item)
		} else {
			flex.Add(p.elem)
		}
	}

	// Wrap in a Div if the flex container has box-model properties
	// that the Flex type doesn't support (border-radius, opacity, etc.).
	hasExtraVisuals := style.BorderRadius > 0 || style.BorderRadiusTL > 0 || style.BorderRadiusTR > 0 || style.BorderRadiusBR > 0 || style.BorderRadiusBL > 0 ||
		(style.Opacity > 0 && style.Opacity < 1) ||
		style.Overflow == "hidden" ||
		len(style.BoxShadows) > 0 ||
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
		// applyDivStyles needs the parent's containing-block width
		// for margin/padding percent resolution, not the post-narrow
		// flex content box.
		applyDivStyles(div, style, parentContainerWidth)
		return []layout.Element{div}
	}

	return []layout.Element{flex}
}
