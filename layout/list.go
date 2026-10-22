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
	direction      Direction          // text direction for list items
	markerColor    *Color             // optional override color for markers
	markerFontSize float64            // optional override font size for markers (0 = use list fontSize)
	markerFont     *font.Standard     // optional override standard font for markers (nil = use list font)
	markerEmbedded *font.EmbeddedFont // optional override embedded font for markers (nil = use list font)
	markerInside   bool               // CSS list-style-position: inside (marker flows inline)

	// start is the ordinal offset for marker numbering. The marker for the
	// item at slice index i is numbered (i + 1 + start). It defaults to 0 so
	// numbering begins at 1. SetStart adjusts it for <ol start="N">, and
	// overflowFrom threads the count of items already emitted on prior pages
	// into the continuation List so numbering continues across page breaks
	// instead of restarting at 1.
	start int
}

// listItem is a single entry in a list, optionally containing a nested sub-list.
// When element is non-nil, the item renders that rich element (block-level
// children, a styled box, etc.) and the marker is aligned to the first text
// line of the element. Otherwise, when runs is non-nil the item renders as a
// styled paragraph (supporting links, mixed fonts, etc.); otherwise it uses
// the plain text field.
type listItem struct {
	text    string
	runs    []TextRun // styled runs (nil = use plain text)
	element Element   // rich content; overrides text/runs when set
	subList *List     // optional nested list

	// suppressMarker is set on a continuation fragment produced when an
	// element item (or a plain runs item, see contPara) splits across pages.
	// The marker (bullet/number) is drawn only on the first fragment, so
	// continuation fragments suppress it.
	suppressMarker bool

	// contPara holds a pre-wrapped continuation paragraph for a plain
	// (text/runs) item that was split across a page break. When non-nil it is
	// used verbatim as the item's paragraph (overriding text/runs) so the tail
	// lines that did not fit on the prior page render on the next one without
	// re-deriving them from text.
	contPara *Paragraph

	// markerText/markerSet hold a verbatim marker string supplied by CSS
	// (li::marker { content: ... }). When markerSet is true the item's marker
	// is exactly markerText, overriding the style-derived marker entirely —
	// including ListNone — and an empty markerText suppresses the marker.
	markerText string
	markerSet  bool
}

// listLayoutRef carries list-specific rendering info on a Line.
type listLayoutRef struct {
	markerWords []Word  // words for the bullet/number (first line only)
	indent      float64 // left indent for the item text

	// element item fields (set when the item renders a rich Element)
	element       Element // non-nil for rich-content items
	elementWidth  float64 // layout width for the element (content column)
	markerOffsetY float64 // baseline of the first text line, from the line top
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

// SetStart sets the ordinal of the first item's marker for ordered lists,
// matching the HTML <ol start="N"> attribute. The first item is numbered n,
// the second n+1, and so on. Values below 1 are clamped to 1.
func (l *List) SetStart(n int) *List {
	if n < 1 {
		n = 1
	}
	l.start = n - 1
	return l
}

// Start returns the ordinal of the first item's marker for ordered lists,
// i.e. the value set via SetStart (or the HTML <ol start="N"> attribute).
// It defaults to 1 when no start was configured.
func (l *List) Start() int {
	return l.start + 1
}

// SetIndent sets the left indent for item text (default 18pt).
func (l *List) SetIndent(indent float64) *List {
	l.indent = indent
	return l
}

// Indent returns the left indent for item text. Item content (text runs or a
// rich element) is laid out in the column from this indent to the available
// width, so percentage box-model values on an element item must resolve
// against (availableWidth - Indent).
func (l *List) Indent() float64 {
	return l.indent
}

// SetLeading sets the line height multiplier.
func (l *List) SetLeading(leading float64) *List {
	l.leading = leading
	return l
}

// SetDirection sets the text direction for list items. When RTL, markers
// are positioned on the right side and item text is indented from the
// right margin. Item paragraphs inherit this direction for bidi reordering.
func (l *List) SetDirection(d Direction) *List {
	l.direction = d
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

// SetMarkerFont sets an override font for list markers, independent of the
// list's body font (used for CSS `::marker { font-style: italic }`).
// Exactly one of std/emb should be non-nil, mirroring how the list itself
// carries either a standard or an embedded font.
func (l *List) SetMarkerFont(std *font.Standard, emb *font.EmbeddedFont) *List {
	l.markerFont = std
	l.markerEmbedded = emb
	return l
}

// SetMarkerInside controls marker placement. When inside is true (CSS
// list-style-position: inside) the marker flows inline with the first content
// line and wrapped lines align under the marker. The default (false) is
// outside: the marker sits in the left gutter with a hanging indent.
func (l *List) SetMarkerInside(inside bool) *List {
	l.markerInside = inside
	return l
}

// AddItem adds a text item to the list.
func (l *List) AddItem(text string) *List {
	l.items = append(l.items, listItem{text: normalizeText(text)})
	return l
}

// AddItemRuns adds an item with styled text runs, supporting links,
// mixed fonts, and other inline formatting within the list item.
//
// The Text field of every run is NFC-normalized into a freshly
// allocated slice so List measurement (MinWidth, MaxWidth) sees the
// same canonical form as Layout, and the caller's slice is not
// mutated.
func (l *List) AddItemRuns(runs []TextRun) *List {
	l.items = append(l.items, listItem{runs: normalizeRuns(runs)})
	return l
}

// AddItemRunsWithSubList adds a styled-runs item and returns a nested
// sub-list under it.
//
// The run slice is copied and NFC-normalized so the caller's slice is
// not mutated and List measurement sees canonical text.
func (l *List) AddItemRunsWithSubList(runs []TextRun) *List {
	sub := &List{
		style:    ListUnordered,
		font:     l.font,
		embedded: l.embedded,
		fontSize: l.fontSize,
		indent:   l.indent,
		leading:  l.leading,
	}
	l.items = append(l.items, listItem{runs: normalizeRuns(runs), subList: sub})
	return sub
}

// AddItemElement adds an item whose content is a rich Element (for
// block-level <li> children or a styled <li> box). The list marker is
// aligned to the first text line of the element rather than rendered
// inline with text runs. The element is laid out within the item's
// content column (to the right of the marker).
func (l *List) AddItemElement(elem Element) *List {
	l.items = append(l.items, listItem{element: elem})
	return l
}

// AddItemElementWithSubList adds a rich-element item and returns a nested
// sub-list under it. The sub-list inherits the parent's font and font size.
func (l *List) AddItemElementWithSubList(elem Element) *List {
	sub := &List{
		style:    ListUnordered,
		font:     l.font,
		embedded: l.embedded,
		fontSize: l.fontSize,
		indent:   l.indent,
		leading:  l.leading,
	}
	l.items = append(l.items, listItem{element: elem, subList: sub})
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
	l.items = append(l.items, listItem{text: normalizeText(text), subList: sub})
	return sub
}

// SetLastItemMarker sets a verbatim CSS marker string on the most recently
// appended item (from li::marker { content }), overriding the style-derived
// marker. It is a no-op when the list has no items.
func (l *List) SetLastItemMarker(text string) *List {
	if len(l.items) == 0 {
		return l
	}
	l.items[len(l.items)-1].markerText = text
	l.items[len(l.items)-1].markerSet = true
	return l
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
	effIndent := l.effectiveIndent()
	totalIndent := baseIndent + effIndent
	itemWidth := maxWidth - totalIndent

	for i, item := range l.items {
		// Element items: lay the element out in the content column and
		// align the marker to the first text line of the element.
		if item.element != nil {
			allLines = append(allLines, l.layoutElementItem(item, i, maxWidth, totalIndent, itemWidth)...)
			if item.subList != nil {
				allLines = append(allLines, item.subList.layoutAt(maxWidth, totalIndent)...)
			}
			continue
		}

		markerPara := l.markerParagraph(i)
		markerLines := markerPara.Layout(l.markerLayoutWidth(i))

		// Create a paragraph for the item text.
		textPara := l.itemParagraph(item)
		textPara.SetLeading(l.leading)
		if l.direction != DirectionAuto {
			textPara.SetDirection(l.direction)
		}
		textLines := textPara.Layout(itemWidth)

		// Combine: the first line has both marker and text side by side.
		// (This Layout path feeds measurement/Columns; the marker is drawn by
		// the paginated planAt path, which carries inside/gutter placement.)
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
				line.listRef = &listLayoutRef{markerWords: markerLines[0].Words, indent: totalIndent}
			} else {
				line.listRef = &listLayoutRef{indent: totalIndent}
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

// layoutElementItem produces the lines for a rich-element list item. The
// element is laid out in the content column (width = itemWidth) and emitted
// as a single synthetic line carrying the element; the marker is aligned to
// the first text line of the element via markerOffsetY.
func (l *List) layoutElementItem(item listItem, index int, maxWidth, totalIndent, itemWidth float64) []Line {
	plan := item.element.PlanLayout(LayoutArea{Width: itemWidth, Height: 1e9})

	markerPara := l.markerParagraph(index)
	markerLines := markerPara.Layout(l.markerLayoutWidth(index))
	var markerWords []Word
	if len(markerLines) > 0 {
		markerWords = markerLines[0].Words
	}

	// Align the marker baseline to the first text line of the element.
	firstY, firstH, ok := firstLeafLine(plan.Blocks)
	markerOffsetY := 0.0
	if ok {
		markerOffsetY = firstY + computeBaseline(markerWords, firstH)
	} else {
		markerOffsetY = computeBaseline(markerWords, l.fontSize*l.leading)
	}

	line := Line{
		Height: plan.Consumed,
		IsLast: true,
		listRef: &listLayoutRef{
			markerWords:   markerWords,
			indent:        totalIndent,
			element:       item.element,
			elementWidth:  itemWidth,
			markerOffsetY: markerOffsetY,
		},
	}
	return []Line{line}
}

// MinWidth implements Measurable. Returns indent + widest word.
func (l *List) MinWidth() float64 {
	maxW := 0.0
	measurer := l.measurer()
	for _, item := range l.items {
		if item.element != nil {
			if m, ok := item.element.(Measurable); ok {
				if w := m.MinWidth(); w > maxW {
					maxW = w
				}
			}
			continue
		}
		text := l.itemText(item)
		for _, w := range splitWords(text) {
			ww := measurer.MeasureString(w, l.fontSize)
			if ww > maxW {
				maxW = ww
			}
		}
	}
	return l.effectiveIndent() + maxW
}

// MaxWidth implements Measurable. Returns indent + widest item line.
func (l *List) MaxWidth() float64 {
	maxW := 0.0
	measurer := l.measurer()
	for _, item := range l.items {
		if item.element != nil {
			if m, ok := item.element.(Measurable); ok {
				if w := m.MaxWidth(); w > maxW {
					maxW = w
				}
			}
			continue
		}
		text := l.itemText(item)
		ww := measurer.MeasureString(text, l.fontSize)
		if ww > maxW {
			maxW = ww
		}
	}
	return l.effectiveIndent() + maxW
}

// itemParagraph creates a Paragraph for a list item's text content.
// Uses styled runs when available, falling back to plain text.
func (l *List) itemParagraph(item listItem) *Paragraph {
	if item.contPara != nil {
		return item.contPara
	}
	if len(item.runs) > 0 {
		return NewStyledParagraph(item.runs...)
	}
	if l.embedded != nil {
		return NewParagraphEmbedded(item.text, l.embedded, l.fontSize)
	}
	return NewParagraph(item.text, l.font, l.fontSize)
}

// effectiveIndent returns the left indent that defines the marker gutter (for
// outside) or content offset (for inside). For inside it is l.indent unchanged.
// For outside it grows only when the widest marker would overflow the default
// indent, so a long custom marker does not overlap the item text; short-marker
// lists are unaffected and stay byte-identical to before.
func (l *List) effectiveIndent() float64 {
	if l.markerInside {
		return l.indent
	}
	widest := 0.0
	for i := range l.items {
		if l.items[i].suppressMarker {
			continue
		}
		if w := l.markerWidth(i); w > widest {
			widest = w
		}
	}
	// Only grow when a marker is wider than the gutter (i.e. it would overlap
	// the item text). Markers that already fit leave the indent untouched, so
	// short-marker lists stay byte-identical. When growing, add a space gap so
	// the marker and text are visibly separated.
	if widest > l.indent {
		return widest + l.spaceWidth()
	}
	return l.indent
}

// markerWidth returns the unbroken width of the item's marker (its widest
// single-line extent), used to size the outside gutter so a long marker does
// not overlap the item text. Zero when the marker is empty/suppressed.
func (l *List) markerWidth(index int) float64 {
	words, _ := l.markerParagraph(index).measureWords(1e9)
	return wordsWidth(words)
}

// wordsWidth sums word widths plus the inter-word SpaceAfter gaps (the trailing
// word's SpaceAfter is excluded, matching the rendered content extent).
func wordsWidth(words []Word) float64 {
	w := 0.0
	for i := range words {
		w += words[i].Width
		if i < len(words)-1 {
			w += words[i].SpaceAfter
		}
	}
	return w
}

// spaceWidth returns the width of a single space in the list font at the
// list font size — the gap between an inline/gutter marker and the item text.
func (l *List) spaceWidth() float64 {
	return l.measurer().MeasureString(" ", l.fontSize)
}

// markerLayoutWidth returns the width to lay the marker paragraph out at.
// Default (style-derived) markers wrap at the indent gutter as before. A custom
// CSS marker (markerSet) may legitimately exceed the gutter, so it is laid out
// at a width wide enough to keep it on a single line; the gutter is unchanged in
// this phase, so such a marker simply extends past it.
func (l *List) markerLayoutWidth(index int) float64 {
	if l.items[index].markerSet {
		return 1e9
	}
	return l.indent
}

// markerParagraph builds the marker paragraph for the item at index,
// applying the list's marker font size and color overrides.
func (l *List) markerParagraph(index int) *Paragraph {
	marker := l.marker(index)
	markerSize := l.fontSize
	if l.markerFontSize > 0 {
		markerSize = l.markerFontSize
	}
	stdFont, embFont := l.font, l.embedded
	if l.markerFont != nil || l.markerEmbedded != nil {
		stdFont, embFont = l.markerFont, l.markerEmbedded
	}
	var markerPara *Paragraph
	if embFont != nil {
		markerPara = NewParagraphEmbedded(marker, embFont, markerSize)
	} else {
		markerPara = NewParagraph(marker, stdFont, markerSize)
	}
	if l.markerColor != nil {
		markerPara.runs[0].Color = *l.markerColor
	}
	markerPara.SetLeading(l.leading)
	return markerPara
}

// firstLeafLine descends a placed-block tree and returns the relative Y
// offset and height of the first (topmost, then leftmost) leaf block that
// carries content (non-zero height). The Y is accumulated through container
// blocks so it is relative to the top of the supplied block slice. The
// boolean is false when no content leaf is found (e.g. an empty element).
func firstLeafLine(blocks []PlacedBlock) (y, height float64, ok bool) {
	bestSet := false
	var bestY, bestH float64
	var walk func(bs []PlacedBlock, offY float64)
	walk = func(bs []PlacedBlock, offY float64) {
		for _, b := range bs {
			absY := offY + b.Y
			if len(b.Children) > 0 {
				walk(b.Children, absY)
				continue
			}
			if b.Height <= 0 {
				continue
			}
			if !bestSet || absY < bestY {
				bestSet = true
				bestY = absY
				bestH = b.Height
			}
		}
	}
	walk(blocks, 0)
	return bestY, bestH, bestSet
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
// Fallback l.font preserves the historical typed-nil return when unset.
func (l *List) measurer() font.TextMeasurer {
	return resolveMeasurer(l.embedded, l.font, l.font)
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

	totalIndent := baseIndent + l.effectiveIndent()
	itemWidth := area.Width - totalIndent

	var blocks []PlacedBlock
	curY := 0.0
	allFit := true

	for i, item := range l.items {
		// Element items: lay the element out in the content column and
		// align the marker to the first text line of the element.
		if item.element != nil {
			elemBlocks, consumed, status, elemOverflow := l.planElementItem(item, i, area, totalIndent, itemWidth, curY)

			// The element could not place anything in the remaining area.
			// If other content already sits on this page, defer the whole
			// item (and the rest of the list) to the next page. Otherwise we
			// are at the top of an empty area: fall through to place whatever
			// the element can produce (LayoutNothing yields no blocks) so we
			// never loop forever on a too-tall first item.
			if status == LayoutNothing && len(blocks) > 0 {
				return LayoutPlan{
					Status: LayoutPartial, Consumed: curY,
					Blocks: wrapListBlocks(blocks, area.Width, curY), Overflow: l.overflowFrom(i),
				}
			}

			// The element partially fit: emit the part that fit (marker on
			// this fragment only) and continue the remaining list with the
			// element replaced by its overflow, marker suppressed.
			if status == LayoutPartial {
				// Guard against an infinite loop: if nothing actually fit
				// (consumed == 0) and there is already content on the page,
				// move the whole item to a fresh page instead of emitting an
				// empty fragment. At the top of an empty page we fall through
				// and accept the partial fragment to make progress.
				if consumed == 0 && len(blocks) > 0 {
					return LayoutPlan{
						Status: LayoutPartial, Consumed: curY,
						Blocks: wrapListBlocks(blocks, area.Width, curY), Overflow: l.overflowFrom(i),
					}
				}
				blocks = append(blocks, elemBlocks...)
				curY += consumed
				return LayoutPlan{
					Status: LayoutPartial, Consumed: curY,
					Blocks:   wrapListBlocks(blocks, area.Width, curY),
					Overflow: l.overflowContinuingElement(i, elemOverflow),
				}
			}

			blocks = append(blocks, elemBlocks...)
			curY += consumed

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
			continue
		}

		// Continuation fragments (a runs item split from a prior page)
		// suppress the marker so the bullet/number is drawn only once.
		var markerWords []Word
		markerW := 0.0
		if !item.suppressMarker {
			markerPara := l.markerParagraph(i)
			markerWords, _ = markerPara.measureWords(l.indent)
			// markerW is the drawn marker's content width (sum of the same
			// words), so inside text starts exactly where the marker ends.
			markerW = wordsWidth(markerWords)
		}

		// inside places the marker inline as the leading content of the first
		// line (RTL still uses the outside gutter path; see drawTextLine below).
		inside := l.markerInside && l.direction != DirectionRTL

		// Measure and wrap item text directly. For inside, the first line is
		// narrowed by the marker width so wrapped lines align under the marker.
		textPara := l.itemParagraph(item)
		textPara.SetLeading(l.leading)
		if l.direction != DirectionAuto {
			textPara.SetDirection(l.direction)
		}
		if inside && markerW > 0 {
			textPara.SetFirstLineIndent(markerW)
		}
		textWords, maxFS := textPara.measureWords(itemWidth)
		lineHeight := maxFS * l.leading
		wordLines := textPara.wrapWords(textWords, itemWidth)

		// Build PlacedBlocks for each text line. fitCount tracks how many of
		// this item's lines fit on the current page; if not all fit, the item
		// is split (or deferred whole) so it is never dropped.
		fitCount := 0
		for j, wl := range wordLines {
			if curY+lineHeight > area.Height && len(blocks) > 0 {
				allFit = false
				break
			}
			fitCount = j + 1

			capturedWords := wl
			capturedHeight := lineHeight
			capturedMaxW := area.Width
			capturedIndent := totalIndent
			// Indent the marker by the accumulated parent indent so each
			// nesting level's marker steps right under its parent's content
			// rather than stacking at the container's left edge (issue #358).
			// (RTL already steps via its depth-dependent x term below.)
			capturedBase := baseIndent
			capturedIsLast := j == len(wordLines)-1
			capturedRTL := l.direction == DirectionRTL
			capturedInside := inside
			// For inside, the marker leads the first line inline at the content
			// edge; the text then starts markerW further right on that line.
			capturedMarkerW := 0.0
			var capturedMarker []Word
			if j == 0 {
				capturedMarker = markerWords
				if capturedInside {
					capturedMarkerW = markerW
				}
			}

			block := PlacedBlock{
				X: 0, Y: curY, Width: lineWidth(wl), Height: lineHeight,
				Tag:   "LI",
				Links: linkSpans(wl),
				Draw: func(ctx DrawContext, absX, absTopY float64) {
					baselineY := absTopY - computeBaseline(capturedWords, capturedHeight)
					switch {
					case capturedRTL:
						// RTL: marker on the right, text indented from the right.
						// drawTextLine only consults align for justification —
						// every other alignment has to arrive already applied to
						// x, the way paragraph layout does it. Passing absX with
						// AlignRight left every list item stranded against the
						// left margin with its bullet a column away.
						textBoxW := capturedMaxW - capturedIndent
						if len(capturedMarker) > 0 {
							// Right-align the marker inside its gutter so a gap
							// separates it from the text, as a browser renders it.
							markerX := absX + capturedMaxW - lineWidth(capturedMarker)
							drawTextLine(ctx, capturedMarker, markerX, baselineY, capturedIndent, AlignRight, true)
						}
						textX := absX + textBoxW - lineWidth(capturedWords)
						if textX < absX {
							textX = absX
						}
						drawTextLine(ctx, capturedWords, textX, baselineY, textBoxW, AlignRight, capturedIsLast)
					case capturedInside:
						// Inside: marker is inline at the content edge; the first
						// line's text follows it, wrapped lines align under it.
						contentX := absX + capturedIndent
						if len(capturedMarker) > 0 {
							drawTextLine(ctx, capturedMarker, contentX, baselineY, capturedMarkerW, AlignLeft, true)
						}
						drawTextLine(ctx, capturedWords, contentX+capturedMarkerW, baselineY, capturedMaxW-capturedIndent-capturedMarkerW, AlignLeft, capturedIsLast)
					default:
						if len(capturedMarker) > 0 {
							drawTextLine(ctx, capturedMarker, absX+capturedBase, baselineY, capturedIndent-capturedBase, AlignLeft, true)
						}
						drawTextLine(ctx, capturedWords, absX+capturedIndent, baselineY, capturedMaxW-capturedIndent, AlignLeft, capturedIsLast)
					}
				},
			}
			// Offset link annotation x-coords. Text starts after the gutter
			// (outside) or after the inline marker on the first line (inside).
			linkBase := capturedIndent + capturedMarkerW
			for k := range block.Links {
				block.Links[k].X += linkBase
			}
			blocks = append(blocks, block)
			curY += lineHeight
		}

		if !allFit {
			// The item at i did not fully fit. Never skip it: if none of its
			// lines fit, defer the whole item to the next page (overflow starts
			// at i, marker intact). If some lines fit, split the item — emit the
			// fitting head here and continue the tail on the next page with the
			// marker suppressed. Either way the continuation List continues the
			// ordinal sequence (overflowFrom threads the start offset).
			var overflowList *List
			if fitCount == 0 {
				overflowList = l.overflowFrom(i)
			} else {
				_, tail := textPara.SplitAfterLine(fitCount, itemWidth)
				overflowList = l.overflowSplitting(i, tail)
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

// overflowFrom builds a List carrying the items from index from onward,
// preserving the list's rendering attributes for page-break continuation.
// The continuation's start offset is advanced by from so ordered-list markers
// continue the sequence across the page break instead of restarting at 1: the
// item now at continuation slice index 0 was originally at index from, so its
// marker (0 + 1 + cont.start) equals (from + 1 + l.start), its true ordinal.
func (l *List) overflowFrom(from int) *List {
	cont := l.cloneWithItems(l.items[from:])
	cont.start = l.start + from
	return cont
}

// overflowSplitting builds the continuation List for a plain (text/runs) item
// at index i that was split across a page break. The item at i is replaced by a
// continuation carrying the tail paragraph (the lines that did not fit) with the
// marker suppressed; remaining items i+1... follow unchanged. The start offset
// is advanced by i so the suppressed item still "occupies" its ordinal and the
// following items continue numbering correctly.
func (l *List) overflowSplitting(i int, tail *Paragraph) *List {
	cont := l.items[i]
	cont.contPara = tail
	cont.suppressMarker = true
	cont.text = ""
	cont.runs = nil

	items := make([]listItem, 0, len(l.items)-i)
	items = append(items, cont)
	items = append(items, l.items[i+1:]...)

	out := l.cloneWithItems(items)
	out.start = l.start + i
	return out
}

// overflowContinuingElement builds the continuation List for a partially-laid-out
// element item at index i. The item at i is replaced by a continuation whose
// element is the element's overflow and whose marker is suppressed (the
// bullet/number is drawn only on the first fragment). Remaining items i+1...
// follow unchanged. The nested sub-list (if any) is preserved on the
// continuation so it still renders after the element's tail content.
func (l *List) overflowContinuingElement(i int, elemOverflow Element) *List {
	cont := l.items[i]
	cont.element = elemOverflow
	cont.suppressMarker = true

	items := make([]listItem, 0, len(l.items)-i)
	items = append(items, cont)
	items = append(items, l.items[i+1:]...)
	out := l.cloneWithItems(items)
	out.start = l.start + i
	return out
}

// cloneWithItems returns a List with the supplied items and this list's
// rendering attributes, for page-break continuation.
func (l *List) cloneWithItems(items []listItem) *List {
	return &List{
		items:          items,
		style:          l.style,
		font:           l.font,
		embedded:       l.embedded,
		fontSize:       l.fontSize,
		indent:         l.indent,
		leading:        l.leading,
		direction:      l.direction,
		markerColor:    l.markerColor,
		markerFontSize: l.markerFontSize,
		markerFont:     l.markerFont,
		markerEmbedded: l.markerEmbedded,
		markerInside:   l.markerInside,
	}
}

// planElementItem lays out a rich-element list item into placed blocks. The
// element occupies the content column (offset by totalIndent); the marker is
// drawn aligned to the baseline of the element's first text line (unless the
// item is a continuation fragment, in which case the marker is suppressed).
// Returns the blocks (Y offsets relative to the list, already shifted by curY),
// the height consumed, and the element's own LayoutPlan status and overflow so
// the caller can split the item across pages.
func (l *List) planElementItem(item listItem, index int, area LayoutArea, totalIndent, itemWidth, curY float64) (blocks []PlacedBlock, consumed float64, status LayoutStatus, overflow Element) {
	plan := item.element.PlanLayout(LayoutArea{Width: itemWidth, Height: area.Height - curY})
	if plan.Status == LayoutNothing {
		return nil, 0, LayoutNothing, nil
	}

	// inside is honored for text/runs items only; element items keep the
	// gutter (outside) marker placement even when inside is requested.
	var markerWords []Word
	if !item.suppressMarker {
		markerPara := l.markerParagraph(index)
		markerWords, _ = markerPara.measureWords(l.indent)
	}

	// Marker baseline: align to the first text line of the element.
	firstY, firstH, ok := firstLeafLine(plan.Blocks)
	markerBaseline := 0.0
	if ok {
		markerBaseline = firstY + computeBaseline(markerWords, firstH)
	} else {
		markerBaseline = computeBaseline(markerWords, l.fontSize*l.leading)
	}

	// Shift the element's blocks into the content column at curY.
	contentBlocks := offsetBlocks(plan.Blocks, totalIndent, curY)

	out := make([]PlacedBlock, 0, len(contentBlocks)+1)

	// Only emit a marker block when there is a marker to draw. Continuation
	// fragments suppress the marker (markerWords is nil), so they carry no
	// marker block and the bullet/number is never repeated on later pages.
	if len(markerWords) > 0 {
		capturedMarker := markerWords
		capturedIndent := totalIndent
		capturedMaxW := area.Width
		capturedBaseline := markerBaseline
		capturedRTL := l.direction == DirectionRTL
		// Indent the marker by the accumulated parent indent so nested element
		// items step right under their parent's content, matching the text-item
		// path in planAt (issue #358).
		capturedBase := totalIndent - l.effectiveIndent()

		// The marker is drawn relative to its block's top, which sits at the
		// element's top (curY). markerBaseline is measured from that top.
		markerBlock := PlacedBlock{
			X: 0, Y: curY, Width: l.indent, Height: firstH,
			Tag: "LI",
			Draw: func(ctx DrawContext, absX, absTopY float64) {
				baselineY := absTopY - capturedBaseline
				if capturedRTL {
					drawTextLine(ctx, capturedMarker, absX+capturedMaxW-capturedIndent, baselineY, capturedIndent, AlignRight, true)
				} else {
					drawTextLine(ctx, capturedMarker, absX+capturedBase, baselineY, capturedIndent-capturedBase, AlignLeft, true)
				}
			},
		}
		out = append(out, markerBlock)
	}

	out = append(out, contentBlocks...)
	return out, plan.Consumed, plan.Status, plan.Overflow
}

// offsetBlocks returns a deep-enough copy of blocks shifted by (dx, dy) at the
// top level. Child blocks keep their relative offsets.
func offsetBlocks(blocks []PlacedBlock, dx, dy float64) []PlacedBlock {
	out := make([]PlacedBlock, len(blocks))
	for i, b := range blocks {
		b.X += dx
		b.Y += dy
		out[i] = b
	}
	return out
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
	if l.items[index].markerSet {
		return l.items[index].markerText
	}
	n := index + 1 + l.start
	switch l.style {
	case ListNone:
		return ""
	case ListOrdered:
		return fmt.Sprintf("%d.", n)
	case ListOrderedRoman:
		return ToRoman(n, false) + "."
	case ListOrderedRomanUp:
		return ToRoman(n, true) + "."
	case ListOrderedAlpha:
		return ToAlpha(n, 'a') + "."
	case ListOrderedAlphaUp:
		return ToAlpha(n, 'A') + "."
	default:
		return "\u2022" // bullet character •
	}
}

// ToRoman converts n to a Roman numeral string (lower-case, or upper-case when
// upper is true). Values outside 1..3999 fall back to decimal.
func ToRoman(n int, upper bool) string {
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

// ToAlpha converts n to alphabetic numbering using base as the first letter
// ('a' or 'A'): 1=a, 2=b, ..., 26=z, 27=aa. Non-positive n falls back to decimal.
func ToAlpha(n int, base byte) string {
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
