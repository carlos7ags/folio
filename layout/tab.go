// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "github.com/carlos7ags/folio/font"

// TabAlign specifies how text aligns relative to a tab stop position.
type TabAlign int

const (
	TabAlignLeft   TabAlign = iota // text starts at the tab position (default)
	TabAlignRight                  // text ends at the tab position
	TabAlignCenter                 // text is centered on the tab position
)

// TabStop defines a position on a line where a tab character jumps to.
type TabStop struct {
	Position float64  // x position in points from the left edge of the content area
	Align    TabAlign // how text after the tab aligns to this position
	Leader   rune     // fill character between previous content and the tab (0 = none, '.' for dot leaders)
}

// TabbedLine is a layout element that renders a single line of text
// with tab stops. Unlike a Paragraph (which word-wraps), a TabbedLine
// always produces exactly one line with content positioned at tab stops.
//
// This is used for table of contents entries, invoice line items,
// form fields, and anywhere you need aligned columns without a full Table.
//
// Usage:
//
//	line := layout.NewTabbedLine(font.Helvetica, 12,
//	    layout.TabStop{Position: 400, Align: layout.TabAlignRight, Leader: '.'},
//	).SetSegments("Chapter 1", "15")
type TabbedLine struct {
	font     *font.Standard
	embedded *font.EmbeddedFont
	fontSize float64
	stops    []TabStop
	segments []string // text segments: segments[0] is before first tab, segments[1] after first tab, etc.
	color    Color
	leading  float64
}

// NewTabbedLine creates a tabbed line with the given font and tab stops.
func NewTabbedLine(f *font.Standard, fontSize float64, stops ...TabStop) *TabbedLine {
	return &TabbedLine{
		font:     f,
		fontSize: fontSize,
		stops:    stops,
		leading:  1.2,
	}
}

// NewTabbedLineEmbedded creates a tabbed line with an embedded font.
func NewTabbedLineEmbedded(ef *font.EmbeddedFont, fontSize float64, stops ...TabStop) *TabbedLine {
	return &TabbedLine{
		embedded: ef,
		fontSize: fontSize,
		stops:    stops,
		leading:  1.2,
	}
}

// SetSegments sets the text segments. The first segment is placed at x=0,
// subsequent segments are placed at the corresponding tab stop.
func (tl *TabbedLine) SetSegments(segments ...string) *TabbedLine {
	tl.segments = segments
	return tl
}

// SetColor sets the text color.
func (tl *TabbedLine) SetColor(c Color) *TabbedLine {
	tl.color = c
	return tl
}

// SetLeading sets the line height multiplier (default 1.2).
func (tl *TabbedLine) SetLeading(l float64) *TabbedLine {
	tl.leading = l
	return tl
}

// Layout returns a single line with tab-positioned words.
func (tl *TabbedLine) Layout(maxWidth float64) []Line {
	if len(tl.segments) == 0 {
		return nil
	}
	measurer := tl.measurer()
	words, totalWidth := tl.computeWords(measurer)
	return []Line{{
		Words:  words,
		Width:  totalWidth,
		Height: tl.fontSize * tl.leading,
		SpaceW: measurer.MeasureString(" ", tl.fontSize),
		Align:  AlignLeft,
		IsLast: true,
	}}
}

// computeWords measures all segments and returns positioned words.
func (tl *TabbedLine) computeWords(measurer font.TextMeasurer) ([]Word, float64) {
	var words []Word
	totalWidth := 0.0

	for i, seg := range tl.segments {
		if i == 0 {
			segWords := tl.measureSegment(seg, measurer)
			words = append(words, segWords...)
			for _, w := range segWords {
				totalWidth += w.Width + w.SpaceAfter
			}
		} else {
			stopIdx := i - 1
			if stopIdx >= len(tl.stops) {
				segWords := tl.measureSegment(seg, measurer)
				words = append(words, segWords...)
				for _, w := range segWords {
					totalWidth += w.Width + w.SpaceAfter
				}
				continue
			}

			stop := tl.stops[stopIdx]

			if stop.Leader != 0 {
				leaderWord := tl.buildLeader(stop.Leader, totalWidth, stop.Position, seg, stop.Align, measurer)
				if leaderWord.Width > 0 {
					words = append(words, leaderWord)
					totalWidth += leaderWord.Width + leaderWord.SpaceAfter
				}
			}

			segWidth := measurer.MeasureString(seg, tl.fontSize)
			tabX := tl.tabX(stop, segWidth)

			gap := tabX - totalWidth
			if gap > 0 && len(words) > 0 {
				words[len(words)-1].SpaceAfter = gap
			}

			segWords := tl.measureSegment(seg, measurer)
			words = append(words, segWords...)
			totalWidth = tabX + segWidth
		}
	}

	if len(words) > 0 {
		totalWidth -= words[len(words)-1].SpaceAfter
		words[len(words)-1].SpaceAfter = 0
	}

	return words, totalWidth
}

// tabX computes the x position where the segment text starts,
// given the tab stop and the segment's width.
func (tl *TabbedLine) tabX(stop TabStop, segWidth float64) float64 {
	switch stop.Align {
	case TabAlignRight:
		return stop.Position - segWidth
	case TabAlignCenter:
		return stop.Position - segWidth/2
	default: // TabAlignLeft
		return stop.Position
	}
}

// buildLeader creates a word filled with the leader character
// spanning from curX to the tab stop position.
func (tl *TabbedLine) buildLeader(leader rune, curX, stopPos float64, seg string, align TabAlign, measurer font.TextMeasurer) Word {
	segWidth := measurer.MeasureString(seg, tl.fontSize)
	targetX := tl.tabX(TabStop{Position: stopPos, Align: align}, segWidth)

	gap := targetX - curX
	if gap <= 0 {
		return Word{}
	}

	leaderStr := string(leader)
	leaderW := measurer.MeasureString(leaderStr, tl.fontSize)
	if leaderW <= 0 {
		return Word{}
	}

	// Compute how many leader characters fit, with spacing.
	spaceW := measurer.MeasureString(" ", tl.fontSize)
	leaderUnit := leaderW + spaceW*0.5 // leader char + half space
	count := int(gap / leaderUnit)
	if count <= 0 {
		return Word{}
	}

	var text string
	for range count {
		text += leaderStr + " "
	}

	return Word{
		Text:     text,
		Width:    measurer.MeasureString(text, tl.fontSize),
		Font:     tl.font,
		Embedded: tl.embedded,
		FontSize: tl.fontSize,
		Color:    tl.color,
	}
}

// measureSegment splits a segment into measured words.
func (tl *TabbedLine) measureSegment(text string, measurer font.TextMeasurer) []Word {
	raw := splitWords(text)
	spaceW := measurer.MeasureString(" ", tl.fontSize)
	words := make([]Word, len(raw))
	for i, w := range raw {
		words[i] = Word{
			Text:       w,
			Width:      measurer.MeasureString(w, tl.fontSize),
			Font:       tl.font,
			Embedded:   tl.embedded,
			FontSize:   tl.fontSize,
			Color:      tl.color,
			SpaceAfter: spaceW,
		}
	}
	return words
}

// measurer returns the text measurer for this tabbed line's font.
func (tl *TabbedLine) measurer() font.TextMeasurer {
	if tl.embedded != nil {
		return tl.embedded
	}
	return tl.font
}

// MinWidth implements Measurable. Returns the rightmost tab stop position
// (the narrowest width that shows all tab-positioned content).
func (tl *TabbedLine) MinWidth() float64 {
	maxPos := 0.0
	for _, stop := range tl.stops {
		if stop.Position > maxPos {
			maxPos = stop.Position
		}
	}
	return maxPos
}

// MaxWidth implements Measurable. Same as MinWidth for tabbed lines.
func (tl *TabbedLine) MaxWidth() float64 { return tl.MinWidth() }

// PlanLayout implements Element. A tabbed line never splits — FULL or NOTHING.
func (tl *TabbedLine) PlanLayout(area LayoutArea) LayoutPlan {
	if len(tl.segments) == 0 {
		return LayoutPlan{Status: LayoutFull}
	}

	measurer := tl.measurer()
	lineHeight := tl.fontSize * tl.leading
	words, totalWidth := tl.computeWords(measurer)

	if lineHeight > area.Height && area.Height > 0 {
		return LayoutPlan{Status: LayoutNothing}
	}

	capturedWords := words
	capturedWidth := area.Width

	return LayoutPlan{
		Status:   LayoutFull,
		Consumed: lineHeight,
		Blocks: []PlacedBlock{{
			X:      0,
			Y:      0,
			Width:  totalWidth,
			Height: lineHeight,
			Tag:    "P",
			Draw: func(ctx DrawContext, absX, absTopY float64) {
				baseline := computeBaseline(capturedWords, lineHeight)
				drawTextLine(ctx, capturedWords, absX, absTopY-baseline, capturedWidth, AlignLeft, true)
			},
		}},
	}
}
