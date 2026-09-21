// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
)

// splitLinkTarget classifies an href into an external URI and an internal
// named destination. A bare fragment ("#id") addresses a destination inside
// this document, not a resource on the network: emitting it as a PDF /URI
// action produces a link that resolves to nothing in every viewer, so it
// must become a destination instead.
//
// Both results are empty for an href that navigates nowhere — an absent
// href, or a lone "#" with no fragment name. Callers render such an <a> as
// plain text rather than attaching a link annotation with no action.
func splitLinkTarget(href string) (uri, destName string) {
	if strings.HasPrefix(href, "#") {
		return "", href[1:]
	}
	return href, ""
}

// convertLink converts an <a> element into a layout.Link or inline runs.
func (c *converter) convertLink(n *html.Node, style computedStyle) []layout.Element {
	href := getAttr(n, "href")
	uri, destName := splitLinkTarget(href)

	text := collectText(n)
	if text == "" {
		// An anchor need not contain text: an image or icon that navigates
		// is an <a> wrapping only an <img>. Converting the children as
		// inline runs keeps that content in the document and lets the
		// paragraph pipeline attach the annotation to it.
		return c.convertEmptyTextLink(n, style, uri, destName)
	}
	text = applyTextTransform(text, style.TextTransform)

	// Resolve the same font the surrounding text would use, so link text in
	// a document with an @font-face is set in that face instead of silently
	// falling back to a non-embedded standard PDF face.
	stdFont, embFont := c.resolveFontForText(style, text)

	var link *layout.Link
	switch {
	case destName == "" && uri == "":
		// Nothing to navigate to — render the text without an annotation.
		return c.convertInlineContainer(n, style)
	case destName != "" && embFont != nil:
		link = layout.NewInternalLinkEmbedded(text, destName, embFont, style.FontSize)
	case destName != "":
		link = layout.NewInternalLink(text, destName, stdFont, style.FontSize)
	case embFont != nil:
		link = layout.NewLinkEmbedded(text, uri, embFont, style.FontSize)
	default:
		link = layout.NewLink(text, uri, stdFont, style.FontSize)
	}
	link.SetColor(style.Color)
	link.SetUnderline()
	return []layout.Element{link}
}

// convertEmptyTextLink handles an <a> whose content carries no text —
// typically a single <img>. The children are collected as inline runs and
// tagged with the link target, so the paragraph pipeline both keeps the
// content and produces an annotation covering it.
func (c *converter) convertEmptyTextLink(n *html.Node, style computedStyle, uri, destName string) []layout.Element {
	runs := c.collectRuns(n, style)
	if len(runs) == 0 {
		return nil
	}
	for i := range runs {
		runs[i].LinkURI = uri
		runs[i].LinkDest = destName
	}
	return []layout.Element{layout.NewStyledParagraph(runs...)}
}
