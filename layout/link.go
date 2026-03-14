// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "github.com/carlos7ags/folio/font"

// Link is a layout element that renders clickable styled text inline.
// It produces text words (like a Paragraph) but also carries a URI
// or named destination so the renderer can create a link annotation.
//
// Usage:
//
//	doc.Add(layout.NewLink("Click here", "https://example.com", font.Helvetica, 12))
type Link struct {
	para     *Paragraph
	uri      string // external URL (mutually exclusive with destName)
	destName string // internal named destination
}

// NewLink creates a link element with an external URI.
// The text is rendered as a styled paragraph and the entire
// area becomes a clickable link annotation.
func NewLink(text, uri string, f *font.Standard, fontSize float64) *Link {
	p := NewParagraph(text, f, fontSize)
	return &Link{para: p, uri: uri}
}

// NewLinkEmbedded creates a link with an embedded font.
func NewLinkEmbedded(text, uri string, ef *font.EmbeddedFont, fontSize float64) *Link {
	p := NewParagraphEmbedded(text, ef, fontSize)
	return &Link{para: p, uri: uri}
}

// NewInternalLink creates a link to a named destination within the document.
func NewInternalLink(text, destName string, f *font.Standard, fontSize float64) *Link {
	p := NewParagraph(text, f, fontSize)
	return &Link{para: p, destName: destName}
}

// SetColor sets the text color (default is inherited from the paragraph).
func (l *Link) SetColor(c Color) *Link {
	for i := range l.para.runs {
		l.para.runs[i].Color = c
	}
	return l
}

// SetUnderline adds underline decoration to the link text.
func (l *Link) SetUnderline() *Link {
	for i := range l.para.runs {
		l.para.runs[i].Decoration |= DecorationUnderline
	}
	return l
}

// SetAlign sets horizontal alignment.
func (l *Link) SetAlign(a Align) *Link {
	l.para.SetAlign(a)
	return l
}

// Layout implements Element. Returns the paragraph's lines with link
// metadata attached so the renderer can create annotations.
func (l *Link) Layout(maxWidth float64) []Line {
	lines := l.para.Layout(maxWidth)
	for i := range lines {
		lines[i].linkRef = &linkLayoutRef{
			uri:      l.uri,
			destName: l.destName,
		}
	}
	return lines
}

// linkLayoutRef carries link metadata on a Line for annotation creation.
type linkLayoutRef struct {
	uri      string // external URL
	destName string // internal named destination
}

// MinWidth implements Measurable by delegating to the inner Paragraph.
func (l *Link) MinWidth() float64 { return l.para.MinWidth() }

// MaxWidth implements Measurable by delegating to the inner Paragraph.
func (l *Link) MaxWidth() float64 { return l.para.MaxWidth() }

// PlanLayout implements Element by delegating to the inner Paragraph
// and attaching link metadata to each placed block.
func (l *Link) PlanLayout(area LayoutArea) LayoutPlan {
	plan := l.para.PlanLayout(area)

	// Attach link metadata to every block.
	for i := range plan.Blocks {
		plan.Blocks[i].Tag = "Link"
		plan.Blocks[i].Link = &LinkArea{
			URI:      l.uri,
			DestName: l.destName,
		}
	}

	// If there's overflow, wrap it as a Link too.
	if plan.Overflow != nil {
		if overflowPara, ok := plan.Overflow.(*Paragraph); ok {
			plan.Overflow = &Link{para: overflowPara, uri: l.uri, destName: l.destName}
		}
	}

	return plan
}
