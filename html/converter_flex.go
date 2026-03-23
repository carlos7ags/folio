// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
)

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
