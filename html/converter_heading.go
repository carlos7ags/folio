// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
)

// convertHeading creates a layout.Heading from an <h1>-<h6> element.
func (c *converter) convertHeading(n *html.Node, style computedStyle, level layout.HeadingLevel) []layout.Element {
	// Use collectRuns instead of collectText so that inline elements
	// like <a href="..."> are preserved as styled TextRuns with LinkURI.
	runs := c.collectRuns(n, style)
	if len(runs) == 0 {
		return nil
	}

	// Apply text-transform to each run.
	for i := range runs {
		runs[i].Text = applyTextTransform(runs[i].Text, style.TextTransform)
	}

	text := collectText(n)
	stdFont, embFont := c.resolveFontPair(style)
	var h *layout.Heading
	if embFont != nil {
		h = layout.NewHeadingEmbedded(text, level, embFont)
	} else {
		h = layout.NewHeadingWithFont(text, level, stdFont, style.FontSize)
	}
	// Replace the default run with the fully styled runs from collectRuns.
	h.SetRuns(runs)
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
