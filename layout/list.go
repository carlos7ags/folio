// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"

	"github.com/carlos7ags/folio/font"
)

// ListStyle determines the marker style for list items.
type ListStyle int

const (
	ListUnordered      ListStyle = iota // bullet points: •
	ListOrdered                         // decimal: 1. 2. 3.
	ListOrderedRoman                    // lower roman: i. ii. iii. iv.
	ListOrderedRomanUp                  // upper roman: I. II. III. IV.
	ListOrderedAlpha                    // lower alpha: a. b. c.
	ListOrderedAlphaUp                  // upper alpha: A. B. C.
	ListNone                            // no marker
)

// List is a block-level element that renders ordered or unordered items.
type List struct {
	items          []listItem
	style          ListStyle
	font           *font.Standard
	embedded       *font.EmbeddedFont
	fontSize       float64
	indent         float64 // left indent for item text (points)
	leading        float64
	markerColor    *Color  // optional override color for markers
	markerFontSize float64 // optional override font size for markers (0 = use list fontSize)
}

// listItem is a single entry in a list, optionally containing a nested sub-list.
// When runs is non-nil, the item renders as a styled paragraph (supporting
// links, mixed fonts, etc.); otherwise it uses the plain text field.
type listItem struct {
	text    string
	runs    []TextRun // styled runs (nil = use plain text)
	subList *List     // optional nested list
}

// listLayoutRef carries list-specific rendering info on a Line.
type listLayoutRef struct {
	markerWords []Word  // words for the bullet/number (first line only)
	indent      float64 // left indent for the item text
}

// NewList creates an unordered list with a standard font.
func NewList(f *font.Standard, fontSize float64) *List {
	return &List{
		style:    ListUnordered,
		font:     f,
		fontSize: fontSize,
		indent:   18, // default indent
		leading:  1.2,
	}
}

// NewListEmbedded creates an unordered list with an embedded font.
func NewListEmbedded(ef *font.EmbeddedFont, fontSize float64) *List {
	return &List{
		style:    ListUnordered,
		embedded: ef,
		fontSize: fontSize,
		indent:   18,
		leading:  1.2,
	}
}

// SetStyle sets the list marker style (bullet, decimal, roman, alpha, or none).
func (l *List) SetStyle(s ListStyle) *List {
	l.style = s
	return l
}

// SetIndent sets the left indent for item text (default 18pt).
func (l *List) SetIndent(indent float64) *List {
	l.indent = indent
	return l
}

// SetLeading sets the line height multiplier.
func (l *List) SetLeading(leading float64) *List {
	l.leading = leading
	return l
}

// SetMarkerColor sets an override color for list markers.
func (l *List) SetMarkerColor(c Color) *List {
	l.markerColor = &c
	return l
}

// SetMarkerFontSize sets an override font size for list markers.
func (l *List) SetMarkerFontSize(size float64) *List {
	l.markerFontSize = size
	return l
}

// AddItem adds a text item to the list.
func (l *List) AddItem(text string) *List {
	l.items = append(l.items, listItem{text: text})
	return l
}

// AddItemRuns adds an item with styled text runs, supporting links,
// mixed fonts, and other inline formatting within the list item.
func (l *List) AddItemRuns(runs []TextRun) *List {
	l.items = append(l.items, listItem{runs: runs})
	return l
}

// AddItemRunsWithSubList adds a styled-runs item and returns a nested
// sub-list under it.
func (l *List) AddItemRunsWithSubList(runs []TextRun) *List {
	sub := &List{
		style:    ListUnordered,
		font:     l.font,
		embedded: l.embedded,
		fontSize: l.fontSize,
		indent:   l.indent,
		leading:  l.leading,
	}
	l.items = append(l.items, listItem{runs: runs, subList: sub})
	return sub
}

// AddItemWithSubList adds a text item and returns a nested sub-list
// under that item. The sub-list inherits the parent's font and font size.
func (l *List) AddItemWithSubList(text string) *List {
	sub := &List{
		style:    ListUnordered,
		font:     l.font,
		embedded: l.embedded,
		fontSize: l.fontSize,
		indent:   l.indent,
		leading:  l.leading,
	}
	l.items = append(l.items, listItem{text: text, subList: sub})
	return sub
}

// Layout implements Element. Each item is rendered as a paragraph
// with a bullet or number prefix, indented from the left margin.
func (l *List) Layout(maxWidth float64) []Line {
	return l.layoutAt(maxWidth, 0)
}

// layoutAt renders the list with an additional baseIndent accumulated
// from parent lists (for nesting).
func (l *List) layoutAt(maxWidth float64, baseIndent float64) []Line {
	var allLines []Line
	totalIndent := baseIndent + l.indent
	itemWidth := maxWidth - totalIndent

	for i, item := range l.items {
		marker := l.marker(i)

		// Create a paragraph for the marker.
		markerSize := l.fontSize
		if l.markerFontSize > 0 {
			markerSize = l.markerFontSize
		}
		var markerPara *Paragraph
		if l.embedded != nil {
			markerPara = NewParagraphEmbedded(marker, l.embedded, markerSize)
		} else {
			markerPara = NewParagraph(marker, l.font, markerSize)
		}
		if l.markerColor != nil {
			markerPara.runs[0].Color = *l.markerColor
		}
		markerPara.SetLeading(l.leading)
		markerLines := markerPara.Layout(l.indent)

		// Create a paragraph for the item text.
		textPara := l.itemParagraph(item)
		textPara.SetLeading(l.leading)
		textLines := textPara.Layout(itemWidth)

		// Combine: the first line has both marker and text side by side.
		for j, tl := range textLines {
			line := Line{
				Words:  make([]Word, 0, len(tl.Words)),
				Width:  tl.Width,
				Height: tl.Height,
				SpaceW: tl.SpaceW,
				Align:  tl.Align,
				IsLast: tl.IsLast,
			}

			if j == 0 && len(markerLines) > 0 {
				line.listRef = &listLayoutRef{
					markerWords: markerLines[0].Words,
					indent:      totalIndent,
				}
			} else {
				line.listRef = &listLayoutRef{
					indent: totalIndent,
				}
			}

			line.Words = append(line.Words, tl.Words...)
			allLines = append(allLines, line)
		}

		// Recurse into sub-list if present.
		if item.subList != nil {
			subLines := item.subList.layoutAt(maxWidth, totalIndent)
			allLines = append(allLines, subLines...)
		}
	}

	return allLines
}

// MinWidth implements Measurable. Returns indent + widest word.
func (l *List) MinWidth() float64 {
	maxW := 0.0
	for _, item := range l.items {
		text := l.itemText(item)
		measurer := l.measurer()
		for _, w := range splitWords(text) {
			ww := measurer.MeasureString(w, l.fontSize)
			if ww > maxW {
				maxW = ww
			}
		}
	}
	return l.indent + maxW
}

// MaxWidth implements Measurable. Returns indent + widest item line.
func (l *List) MaxWidth() float64 {
	maxW := 0.0
	measurer := l.measurer()
	for _, item := range l.items {
		text := l.itemText(item)
		ww := measurer.MeasureString(text, l.fontSize)
		if ww > maxW {
			maxW = ww
		}
	}
	return l.indent + maxW
}

// itemParagraph creates a Paragraph for a list item's text content.
// Uses styled runs when available, falling back to plain text.
func (l *List) itemParagraph(item listItem) *Paragraph {
	if len(item.runs) > 0 {
		return NewStyledParagraph(item.runs...)
	}
	if l.embedded != nil {
		return NewParagraphEmbedded(item.text, l.embedded, l.fontSize)
	}
	return NewParagraph(item.text, l.font, l.fontSize)
}

// itemText returns the plain text of a list item for measurement.
func (l *List) itemText(item listItem) string {
	if len(item.runs) > 0 {
		var s string
		for _, r := range item.runs {
			s += r.Text
		}
		return s
	}
	return item.text
}

// measurer returns the text measurer for this list's font.
func (l *List) measurer() font.TextMeasurer {
	if l.embedded != nil {
		return l.embedded
	}
	return l.font
}

// PlanLayout implements Element. Lists split between items.
func (l *List) PlanLayout(area LayoutArea) LayoutPlan {
	return l.planAt(area, 0)
}

// planAt produces a LayoutPlan with baseIndent for nesting.
func (l *List) planAt(area LayoutArea, baseIndent float64) LayoutPlan {
	if len(l.items) == 0 {
		return LayoutPlan{Status: LayoutFull}
	}
	if area.Height <= 0 {
		return LayoutPlan{Status: LayoutNothing}
	}

	totalIndent := baseIndent + l.indent
	itemWidth := area.Width - totalIndent

	var blocks []PlacedBlock
	curY := 0.0
	allFit := true

	for i, item := range l.items {
		marker := l.marker(i)

		// Measure marker words.
		markerSize := l.fontSize
		if l.markerFontSize > 0 {
			markerSize = l.markerFontSize
		}
		var markerPara *Paragraph
		if l.embedded != nil {
			markerPara = NewParagraphEmbedded(marker, l.embedded, markerSize)
		} else {
			markerPara = NewParagraph(marker, l.font, markerSize)
		}
		if l.markerColor != nil {
			markerPara.runs[0].Color = *l.markerColor
		}
		markerPara.SetLeading(l.leading)
		markerWords, _ := markerPara.measureWords(l.indent)

		// Measure and wrap item text directly.
		textPara := l.itemParagraph(item)
		textPara.SetLeading(l.leading)
		textWords, maxFS := textPara.measureWords(itemWidth)
		lineHeight := maxFS * l.leading
		wordLines := textPara.wrapWords(textWords, itemWidth)

		// Build PlacedBlocks for each text line.
		for j, wl := range wordLines {
			if curY+lineHeight > area.Height && len(blocks) > 0 {
				allFit = false
				break
			}

			capturedWords := wl
			capturedHeight := lineHeight
			capturedMaxW := area.Width
			capturedIndent := totalIndent
			capturedIsLast := j == len(wordLines)-1
			var capturedMarker []Word
			if j == 0 {
				capturedMarker = markerWords
			}

			block := PlacedBlock{
				X: 0, Y: curY, Width: lineWidth(wl), Height: lineHeight,
				Tag:   "LI",
				Links: linkSpans(wl),
				Draw: func(ctx DrawContext, absX, absTopY float64) {
					baselineY := absTopY - capturedHeight
					if len(capturedMarker) > 0 {
						drawTextLine(ctx, capturedMarker, absX, baselineY, capturedIndent, AlignLeft, true)
					}
					drawTextLine(ctx, capturedWords, absX+capturedIndent, baselineY, capturedMaxW-capturedIndent, AlignLeft, capturedIsLast)
				},
			}
			// Offset link annotation x-coords by the indent since the
			// text starts after the marker column.
			for k := range block.Links {
				block.Links[k].X += capturedIndent
			}
			blocks = append(blocks, block)
			curY += lineHeight
		}

		if !allFit {
			// Build overflow with remaining items.
			overflowList := &List{
				items:    l.items[i+1:],
				style:    l.style,
				font:     l.font,
				embedded: l.embedded,
				fontSize: l.fontSize,
				indent:   l.indent,
				leading:  l.leading,
			}
			return LayoutPlan{
				Status: LayoutPartial, Consumed: curY,
				Blocks: wrapListBlocks(blocks, area.Width, curY), Overflow: overflowList,
			}
		}

		// Recurse into sub-list.
		if item.subList != nil {
			subPlan := item.subList.planAt(
				LayoutArea{Width: area.Width, Height: area.Height - curY},
				totalIndent,
			)
			for _, b := range subPlan.Blocks {
				b.Y += curY
				blocks = append(blocks, b)
			}
			curY += subPlan.Consumed
		}
	}

	return LayoutPlan{Status: LayoutFull, Consumed: curY, Blocks: wrapListBlocks(blocks, area.Width, curY)}
}

// wrapListBlocks wraps list item blocks in a parent "L" block for structure tree nesting.
func wrapListBlocks(blocks []PlacedBlock, width, height float64) []PlacedBlock {
	if len(blocks) == 0 {
		return blocks
	}
	return []PlacedBlock{{
		X: 0, Y: 0, Width: width, Height: height,
		Tag:      "L",
		Children: blocks,
	}}
}

// marker returns the marker string (bullet, number, letter) for the item at index.
func (l *List) marker(index int) string {
	n := index + 1
	switch l.style {
	case ListNone:
		return ""
	case ListOrdered:
		return fmt.Sprintf("%d.", n)
	case ListOrderedRoman:
		return toRoman(n, false) + "."
	case ListOrderedRomanUp:
		return toRoman(n, true) + "."
	case ListOrderedAlpha:
		return toAlpha(n, 'a') + "."
	case ListOrderedAlphaUp:
		return toAlpha(n, 'A') + "."
	default:
		return "\u2022" // bullet character •
	}
}

// toRoman converts n to a Roman numeral string.
func toRoman(n int, upper bool) string {
	if n <= 0 || n > 3999 {
		return fmt.Sprintf("%d", n)
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"m", "cm", "d", "cd", "c", "xc", "l", "xl", "x", "ix", "v", "iv", "i"}
	var result string
	for i, v := range vals {
		for n >= v {
			result += syms[i]
			n -= v
		}
	}
	if upper {
		b := []byte(result)
		for i, c := range b {
			if c >= 'a' && c <= 'z' {
				b[i] = c - 32
			}
		}
		return string(b)
	}
	return result
}

// toAlpha converts n to alphabetic numbering (1=a, 2=b, ..., 26=z, 27=aa).
func toAlpha(n int, base byte) string {
	if n <= 0 {
		return fmt.Sprintf("%d", n)
	}
	var result []byte
	for n > 0 {
		n--
		result = append([]byte{base + byte(n%26)}, result...)
		n /= 26
	}
	return string(result)
}
