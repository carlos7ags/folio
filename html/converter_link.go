// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
)

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
