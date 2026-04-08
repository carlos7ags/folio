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
	para          *Paragraph
	level         HeadingLevel
	bookmarkLevel int               // CSS bookmark-level override (0 = use level)
	bookmarkLabel string            // CSS bookmark-label override (empty = use text)
	stringSets    map[string]string // CSS string-set values to capture
	continuation  bool              // true for the overflow half of a split heading
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

// SetBookmarkLevel overrides the auto-detected heading level for bookmark
// generation. Level 0 means use the default heading level. This corresponds
// to the CSS bookmark-level property.
func (h *Heading) SetBookmarkLevel(level int) *Heading {
	h.bookmarkLevel = level
	return h
}

// SetBookmarkLabel overrides the heading text used in the bookmark tree.
// Empty string means use the heading's text content.
func (h *Heading) SetBookmarkLabel(label string) *Heading {
	h.bookmarkLabel = label
	return h
}

// SetStringSet attaches a CSS string-set value to this heading.
// When the heading is placed during layout, the string value is captured
// and made available to margin boxes via the string() function.
func (h *Heading) SetStringSet(name, value string) *Heading {
	if h.stringSets == nil {
		h.stringSets = make(map[string]string)
	}
	h.stringSets[name] = value
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

// headingSize returns the default font size in points for the given heading level.
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
// and overriding the structure tag. When the heading wraps across a
// page break, the overflow is wrapped in a continuation Heading so
// the remaining lines on the next page keep their H1-H6 structure tag
// (see the continuation field).
func (h *Heading) PlanLayout(area LayoutArea) LayoutPlan {
	// Reserve the half-em "space above" up front so the inner Paragraph
	// packs into the available height minus the spacing. Without this
	// reservation, a heading that exactly fills the remaining area
	// would report Consumed = area.Height + spacing and over-advance
	// the renderer's curY by spacing after the heading — a bug that
	// was previously masked by the multi-line overlap fixed in #132
	// and is now exposed.
	//
	// Continuation headings inherit no space above: the spacing belongs
	// to the original heading on the starting page, not to the wrapped
	// lines on subsequent pages.
	spacing := 0.0
	if !h.continuation {
		spacing = headingSize(h.level) * 0.5
	}

	// Note: the Consumed <= area.Height bound is only tight for
	// multi-line headings. For a single-line heading whose line height
	// exceeds area.Height - spacing, Paragraph.PlanLayout's `i > 0`
	// escape hatch (see the split loop in paragraph.go) still places
	// the line, so Consumed can exceed area.Height by the overshoot.
	// That's a pre-existing degenerate case not addressed by this fix.
	innerArea := area
	innerArea.Height -= spacing
	if innerArea.Height <= 0 {
		return LayoutPlan{Status: LayoutNothing}
	}

	plan := h.para.PlanLayout(innerArea)
	if plan.Status == LayoutNothing {
		return plan
	}

	// Override structure tags from P to H1-H6. The tag must be applied
	// to every block (including continuation headings) so the tagged
	// PDF structure tree remains correct when a heading wraps across
	// pages. HeadingText and stringSets, on the other hand, are only
	// attached to the first block of the *original* heading — they
	// drive auto-bookmarks and the CSS string() capture, both of which
	// must fire exactly once per heading regardless of how many page
	// breaks the heading spans.
	effectiveLevel := h.level
	if h.bookmarkLevel > 0 && h.bookmarkLevel <= 6 {
		effectiveLevel = HeadingLevel(h.bookmarkLevel)
	}
	tag := headingTag(effectiveLevel)
	for i := range plan.Blocks {
		plan.Blocks[i].Tag = tag
	}
	if !h.continuation && len(plan.Blocks) > 0 {
		headingText := h.text()
		if h.bookmarkLabel != "" {
			headingText = h.bookmarkLabel
		}
		plan.Blocks[0].HeadingText = headingText
		if len(h.stringSets) > 0 {
			plan.Blocks[0].StringSets = h.stringSets
		}
	}

	// Shift every block down by the reserved spacing. This must apply
	// to all blocks, not just the first — a heading that wraps to
	// multiple lines produces one PlacedBlock per line, and shifting
	// only Blocks[0] would push line 0 down into line 1's space,
	// causing the wrapped lines to overprint each other (#132).
	// At a page top, render_plans normalizes the leading offset away
	// uniformly so the heading still snaps flush to the top margin.
	if spacing > 0 && len(plan.Blocks) > 0 {
		for i := range plan.Blocks {
			plan.Blocks[i].Y += spacing
		}
		plan.Consumed += spacing
	}

	// If the inner paragraph split, wrap its overflow Paragraph in a
	// continuation Heading so the remaining lines on the next page
	// keep the H1-H6 tag. The continuation intentionally inherits
	// level / bookmarkLevel / bookmarkLabel (so the tag override still
	// resolves correctly) but NOT stringSets: those are captured once
	// on the starting page. HeadingText is suppressed on continuation
	// blocks by the !h.continuation guard above, so no duplicate
	// bookmark is emitted on the continuation page.
	if plan.Status == LayoutPartial {
		if overflowPara, ok := plan.Overflow.(*Paragraph); ok {
			plan.Overflow = &Heading{
				para:          overflowPara,
				level:         h.level,
				bookmarkLevel: h.bookmarkLevel,
				bookmarkLabel: h.bookmarkLabel,
				continuation:  true,
			}
		}
	}

	return plan
}
