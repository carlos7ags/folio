// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"

	"github.com/carlos7ags/folio/font"
)

// HeadingLevel represents H1 through H6.
type HeadingLevel int

const (
	H1 HeadingLevel = iota + 1
	H2
	H3
	H4
	H5
	H6
)

// headingSizes maps heading level to default font size in points.
var headingSizes = [7]float64{
	0,    // unused (index 0)
	28,   // H1
	24,   // H2
	20,   // H3
	16,   // H4
	13.3, // H5
	10.7, // H6
}

// Heading is a block-level text element with a preset size based on its level.
// It renders as a bold paragraph with spacing proportional to its level.
type Heading struct {
	para  *Paragraph
	level HeadingLevel
}

// NewHeading creates a heading using a standard font.
// Uses HelveticaBold by default. The font size is determined by the heading level.
func NewHeading(text string, level HeadingLevel) *Heading {
	size := headingSize(level)
	return &Heading{
		para:  NewParagraph(text, font.HelveticaBold, size),
		level: level,
	}
}

// NewHeadingWithFont creates a heading with a specific standard font.
func NewHeadingWithFont(text string, level HeadingLevel, f *font.Standard, fontSize float64) *Heading {
	return &Heading{
		para:  NewParagraph(text, f, fontSize),
		level: level,
	}
}

// NewHeadingEmbedded creates a heading using an embedded font.
func NewHeadingEmbedded(text string, level HeadingLevel, ef *font.EmbeddedFont) *Heading {
	size := headingSize(level)
	return &Heading{
		para:  NewParagraphEmbedded(text, ef, size),
		level: level,
	}
}

// SetRuns replaces the heading's paragraph runs with the given styled runs.
func (h *Heading) SetRuns(runs []TextRun) *Heading {
	h.para.runs = runs
	return h
}

// SetAlign sets the horizontal alignment.
func (h *Heading) SetAlign(a Align) *Heading {
	h.para.SetAlign(a)
	return h
}

// Layout implements Element. Returns the heading lines with spacing.
func (h *Heading) Layout(maxWidth float64) []Line {
	lines := h.para.Layout(maxWidth)
	if len(lines) == 0 {
		return nil
	}

	// Add spacing above the heading (half the font size).
	spacing := headingSize(h.level) * 0.5
	lines[0].SpaceBefore += spacing

	// Keep the last heading line with the next element (don't orphan headings).
	lines[len(lines)-1].KeepWithNext = true

	// Set structure tag hint for tagged PDF.
	hintTag := headingTag(h.level)
	for i := range lines {
		lines[i].HintTag = hintTag
	}

	return lines
}

func headingSize(level HeadingLevel) float64 {
	if level >= H1 && level <= H6 {
		return headingSizes[level]
	}
	return headingSizes[H1]
}

// headingTag returns the PDF structure tag for a heading level.
func headingTag(level HeadingLevel) string {
	tags := [7]string{"", "H1", "H2", "H3", "H4", "H5", "H6"}
	if level >= H1 && level <= H6 {
		return tags[level]
	}
	return "H1"
}

// text returns the heading's plain text content.
func (h *Heading) text() string {
	var parts []string
	for _, run := range h.para.runs {
		if run.Text != "" {
			parts = append(parts, run.Text)
		}
	}
	return strings.Join(parts, " ")
}

// MinWidth implements Measurable by delegating to the inner Paragraph.
func (h *Heading) MinWidth() float64 { return h.para.MinWidth() }

// MaxWidth implements Measurable by delegating to the inner Paragraph.
func (h *Heading) MaxWidth() float64 { return h.para.MaxWidth() }

// PlanLayout implements Element by delegating to the inner Paragraph
// and overriding the structure tag.
func (h *Heading) PlanLayout(area LayoutArea) LayoutPlan {
	plan := h.para.PlanLayout(area)

	// Override structure tags from P to H1-H6 and set heading text.
	tag := headingTag(h.level)
	headingText := h.text()
	for i := range plan.Blocks {
		plan.Blocks[i].Tag = tag
		if i == 0 {
			plan.Blocks[i].HeadingText = headingText
		}
	}

	// Apply heading spacing on the first block.
	spacing := headingSize(h.level) * 0.5
	if len(plan.Blocks) > 0 {
		plan.Blocks[0].Y += spacing
		plan.Consumed += spacing
	}

	return plan
}
