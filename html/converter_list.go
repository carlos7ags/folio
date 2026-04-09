// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
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

	// Propagate text direction to the list so markers position correctly
	// and item paragraphs inherit the direction for bidi reordering.
	if style.Direction != layout.DirectionAuto {
		list.SetDirection(style.Direction)
	}

	c.populateList(n, list, style)

	return []layout.Element{list}
}

// populateList fills a list with items from <li> children, handling nesting.
// Uses collectRuns (instead of collectDirectText) so inline elements like
// <a href="..."> are preserved as styled TextRuns with LinkURI.
func (c *converter) populateList(n *html.Node, list *layout.List, style computedStyle) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode || child.DataAtom != atom.Li {
			continue
		}

		runs := c.collectListItemRuns(child, style)
		nestedList := findNestedList(child)

		if nestedList != nil {
			if len(runs) == 0 {
				runs = []layout.TextRun{{Text: " ", Font: font.Helvetica, FontSize: style.FontSize}}
			}
			sub := list.AddItemRunsWithSubList(runs)
			if nestedList.DataAtom == atom.Ol {
				sub.SetStyle(layout.ListOrdered)
			}
			c.populateList(nestedList, sub, style)
		} else {
			if len(runs) > 0 {
				list.AddItemRuns(runs)
			}
		}
	}
}
