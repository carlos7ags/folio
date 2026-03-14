// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"slices"
	"strings"

	"github.com/carlos7ags/folio/font"
)

// Paragraph is a block of text that word-wraps within the available width.
// It is composed of one or more TextRuns, each with its own font, size,
// and color. All runs flow together as a single word-wrapped unit.
type Paragraph struct {
	runs        []TextRun
	leading     float64 // line height multiplier (e.g. 1.2 means 120% of fontSize)
	align       Align
	spaceBefore float64 // extra space before the paragraph (points)
	spaceAfter  float64 // extra space after the paragraph (points)
	background  *Color  // background fill color (nil = transparent)
	firstIndent float64 // first-line indent (points, from CSS text-indent)
	orphans     int     // min lines at bottom of page before break (0 = disabled)
	widows      int     // min lines at top of page after break (0 = disabled)
	ellipsis    bool    // if true, truncate overflowing text with "..."
}

// NewParagraph creates a paragraph with a single run using a standard PDF font.
// Panics if f is nil or fontSize is not positive.
func NewParagraph(text string, f *font.Standard, fontSize float64) *Paragraph {
	if f == nil {
		panic("layout.NewParagraph: nil font")
	}
	if fontSize <= 0 {
		panic("layout.NewParagraph: fontSize must be positive")
	}
	return &Paragraph{
		runs:    []TextRun{{Text: text, Font: f, FontSize: fontSize}},
		leading: 1.2,
		align:   AlignLeft,
	}
}

// NewParagraphEmbedded creates a paragraph with a single run using an embedded font.
// Panics if ef is nil or fontSize is not positive.
func NewParagraphEmbedded(text string, ef *font.EmbeddedFont, fontSize float64) *Paragraph {
	if ef == nil {
		panic("layout.NewParagraphEmbedded: nil font")
	}
	if fontSize <= 0 {
		panic("layout.NewParagraphEmbedded: fontSize must be positive")
	}
	return &Paragraph{
		runs:    []TextRun{{Text: text, Embedded: ef, FontSize: fontSize}},
		leading: 1.2,
		align:   AlignLeft,
	}
}

// NewStyledParagraph creates a paragraph from multiple styled runs.
// Runs are concatenated and word-wrapped as a single flowing text.
// Panics if any run has both Font and Embedded nil.
func NewStyledParagraph(runs ...TextRun) *Paragraph {
	for i, r := range runs {
		if r.Font == nil && r.Embedded == nil {
			panic(fmt.Sprintf("layout.NewStyledParagraph: run %d has nil Font and nil Embedded", i))
		}
	}
	return &Paragraph{
		runs:    runs,
		leading: 1.2,
		align:   AlignLeft,
	}
}

// AddRun appends a styled run to the paragraph.
// Panics if the run has both Font and Embedded nil.
func (p *Paragraph) AddRun(r TextRun) *Paragraph {
	if r.Font == nil && r.Embedded == nil {
		panic("layout.Paragraph.AddRun: run has nil Font and nil Embedded")
	}
	p.runs = append(p.runs, r)
	return p
}

// SetLeading sets the line height multiplier (default 1.2).
func (p *Paragraph) SetLeading(l float64) *Paragraph {
	p.leading = l
	return p
}

// SetAlign sets the horizontal text alignment.
func (p *Paragraph) SetAlign(a Align) *Paragraph {
	p.align = a
	return p
}

// SetSpaceBefore sets extra vertical space before the paragraph (in points).
func (p *Paragraph) SetSpaceBefore(pts float64) *Paragraph {
	p.spaceBefore = pts
	return p
}

// SetSpaceAfter sets extra vertical space after the paragraph (in points).
func (p *Paragraph) SetSpaceAfter(pts float64) *Paragraph {
	p.spaceAfter = pts
	return p
}

// SetBackground sets a background fill color for the paragraph.
func (p *Paragraph) SetBackground(c Color) *Paragraph {
	p.background = &c
	return p
}

// SetFirstLineIndent sets the indentation for the first line (in points).
// This corresponds to the CSS text-indent property.
func (p *Paragraph) SetFirstLineIndent(pts float64) *Paragraph {
	p.firstIndent = pts
	return p
}

// SetOrphans sets the minimum number of lines that must remain at the
// bottom of a page before a page break. If fewer lines would remain,
// the entire paragraph is pushed to the next page (via KeepWithNext).
// Default is 0 (disabled). Typical value: 2.
func (p *Paragraph) SetOrphans(n int) *Paragraph {
	p.orphans = n
	return p
}

// SetEllipsis enables or disables text truncation with "..." when text
// overflows the available width. Typically used with overflow:hidden and
// a fixed width container.
func (p *Paragraph) SetEllipsis(v bool) *Paragraph {
	p.ellipsis = v
	return p
}

// SetWidows sets the minimum number of lines that must appear at the
// top of a page after a page break. If fewer lines would appear,
// additional lines are pulled from the previous page. Implemented
// by setting KeepWithNext on trailing lines.
// Default is 0 (disabled). Typical value: 2.
func (p *Paragraph) SetWidows(n int) *Paragraph {
	p.widows = n
	return p
}

// Layout implements Element. It splits the paragraph text into wrapped lines
// that fit within maxWidth. Words from different runs carry their own styling.
func (p *Paragraph) Layout(maxWidth float64) []Line {
	// Flatten all runs into a single word list, each word carrying
	// the styling of the run it came from.
	var measured []Word
	var maxFontSize float64

	for _, run := range p.runs {
		measurer := runMeasurer(run)
		spaceW := measurer.MeasureString(" ", run.FontSize) + run.WordSpacing
		words := splitWords(run.Text)

		for _, w := range words {
			wordW := measurer.MeasureString(w, run.FontSize)
			// Account for letter-spacing: adds extra space after each character except last.
			if run.LetterSpacing != 0 && len([]rune(w)) > 1 {
				wordW += run.LetterSpacing * float64(len([]rune(w))-1)
			}
			measured = append(measured, Word{
				Text:            w,
				Width:           wordW,
				Font:            run.Font,
				Embedded:        run.Embedded,
				FontSize:        run.FontSize,
				Color:           run.Color,
				Decoration:      run.Decoration,
				DecorationColor: run.DecorationColor,
				DecorationStyle: run.DecorationStyle,
				SpaceAfter:      spaceW,
				LetterSpacing:   run.LetterSpacing,
				WordSpacing:     run.WordSpacing,
			})
		}
		if run.FontSize > maxFontSize {
			maxFontSize = run.FontSize
		}
	}

	if len(measured) == 0 {
		// Empty paragraph: still emit spacing if spaceBefore/spaceAfter is set.
		if p.spaceBefore > 0 || p.spaceAfter > 0 {
			return []Line{{
				Height:      0,
				SpaceBefore: p.spaceBefore,
				SpaceAfterV: p.spaceAfter,
				IsLast:      true,
			}}
		}
		return nil
	}

	// Break any single word that exceeds maxWidth into character-level chunks.
	measured = breakLongWords(measured, maxWidth)

	lineHeight := maxFontSize * p.leading

	// Greedy word-wrap.
	// Space between words uses the preceding word's SpaceAfter.
	var lines []Line
	lineStart := 0
	lineWidth := measured[0].Width
	effectiveMax := maxWidth
	if p.firstIndent != 0 {
		effectiveMax = maxWidth - p.firstIndent
	}

	for i := 1; i < len(measured); i++ {
		spaceW := measured[i-1].SpaceAfter
		candidate := lineWidth + spaceW + measured[i].Width
		if candidate > effectiveMax && lineStart < i {
			lines = append(lines, buildLine(measured[lineStart:i], lineWidth, lineHeight, p.align, false))
			lineStart = i
			lineWidth = measured[i].Width
			effectiveMax = maxWidth // subsequent lines use full width
		} else {
			lineWidth = candidate
		}
	}
	// Last line.
	lines = append(lines, buildLine(measured[lineStart:], lineWidth, lineHeight, p.align, true))

	// Apply ellipsis truncation: if enabled, keep only the first line
	// and replace trailing text with "..." if it overflows.
	if p.ellipsis && len(lines) > 1 {
		lines = lines[:1]
		lines[0].IsLast = true
		// Truncate words to fit within maxWidth and append ellipsis.
		lines[0] = truncateWithEllipsis(lines[0], maxWidth)
	}

	// Apply paragraph-level properties to the first/last lines.
	if len(lines) > 0 {
		if p.spaceBefore > 0 {
			lines[0].SpaceBefore = p.spaceBefore
		}
		if p.spaceAfter > 0 {
			lines[len(lines)-1].SpaceAfterV = p.spaceAfter
		}
		if p.background != nil {
			for i := range lines {
				lines[i].Background = p.background
			}
		}

		// Orphans: if the paragraph has more lines than the orphan
		// threshold, mark the first N lines with KeepWithNext so the
		// renderer won't break after fewer than N lines at the bottom.
		if p.orphans > 0 && len(lines) > p.orphans {
			for i := range min(p.orphans, len(lines)-1) {
				lines[i].KeepWithNext = true
			}
		}

		// Widows: if the paragraph has more lines than the widow
		// threshold, mark lines near the end so the renderer pulls
		// enough lines to the next page. We set KeepWithNext on
		// lines starting from (total - widows) to ensure at least
		// `widows` lines land on the next page after any break.
		if p.widows > 0 && len(lines) > p.widows {
			start := max(len(lines)-p.widows-1, 0)
			for i := start; i < len(lines)-1; i++ {
				lines[i].KeepWithNext = true
			}
		}
	}

	return lines
}

// runMeasurer returns the text measurer for a run.
func runMeasurer(run TextRun) font.TextMeasurer {
	if run.Embedded != nil {
		return run.Embedded
	}
	return run.Font
}

// buildLine creates a Line from a slice of words.
func buildLine(words []Word, width, height float64, align Align, isLast bool) Line {
	// SpaceW: use the first word's SpaceAfter as default (for single-font compatibility).
	spaceW := 0.0
	if len(words) > 0 {
		spaceW = words[0].SpaceAfter
	}
	return Line{
		Words:  slices.Clone(words),
		Width:  width,
		Height: height,
		SpaceW: spaceW,
		Align:  align,
		IsLast: isLast,
	}
}

// breakLongWords splits any word exceeding maxWidth into character-level chunks
// so that the word-wrap algorithm can handle them. Words that fit are unchanged.
func breakLongWords(words []Word, maxWidth float64) []Word {
	var result []Word
	for _, w := range words {
		if w.Width <= maxWidth {
			result = append(result, w)
			continue
		}
		// Break by characters. Build chunks that fit within maxWidth.
		runes := []rune(w.Text)
		measurer := w.Font
		var emb *font.EmbeddedFont
		if w.Embedded != nil {
			emb = w.Embedded
		}
		var measure func(string) float64
		if emb != nil {
			measure = func(s string) float64 { return emb.MeasureString(s, w.FontSize) }
		} else if measurer != nil {
			measure = func(s string) float64 { return measurer.MeasureString(s, w.FontSize) }
		} else {
			result = append(result, w)
			continue
		}

		start := 0
		for start < len(runes) {
			end := start + 1
			for end < len(runes) {
				candidate := string(runes[start : end+1])
				if measure(candidate) > maxWidth {
					break
				}
				end++
			}
			chunk := string(runes[start:end])
			result = append(result, Word{
				Text:          chunk,
				Width:         measure(chunk),
				Font:          w.Font,
				Embedded:      w.Embedded,
				FontSize:      w.FontSize,
				Color:         w.Color,
				Decoration:    w.Decoration,
				SpaceAfter:    0, // no inter-word space within a broken word
				LetterSpacing: w.LetterSpacing,
				WordSpacing:   w.WordSpacing,
			})
			start = end
		}
	}
	return result
}

// MinWidth implements Measurable. Returns the width of the longest word
// (the narrowest the paragraph can be without clipping).
func (p *Paragraph) MinWidth() float64 {
	maxWordW := 0.0
	for _, run := range p.runs {
		measurer := runMeasurer(run)
		for _, w := range splitWords(run.Text) {
			ww := measurer.MeasureString(w, run.FontSize)
			if ww > maxWordW {
				maxWordW = ww
			}
		}
	}
	return maxWordW
}

// MaxWidth implements Measurable. Returns the width of all text on a single
// line (the natural width without wrapping).
func (p *Paragraph) MaxWidth() float64 {
	total := 0.0
	for _, run := range p.runs {
		measurer := runMeasurer(run)
		words := splitWords(run.Text)
		spaceW := measurer.MeasureString(" ", run.FontSize)
		for i, w := range words {
			total += measurer.MeasureString(w, run.FontSize)
			if i < len(words)-1 {
				total += spaceW
			}
		}
		// Add a space between runs.
		if len(words) > 0 {
			total += spaceW
		}
	}
	return total
}

// splitWords splits text on whitespace, collapsing consecutive whitespace.
// Newlines are treated as word separators (same as HTML/CSS normal whitespace).
func splitWords(text string) []string {
	return strings.Fields(text)
}

// PlanLayout implements Element. It computes word-wrapped lines that fit
// within the available area. If the paragraph doesn't fit entirely, it
// returns LayoutPartial with the remaining words as the Overflow element.
func (p *Paragraph) PlanLayout(area LayoutArea) LayoutPlan {
	measured, maxFontSize := p.measureWords(area.Width)

	if len(measured) == 0 {
		consumed := p.spaceBefore + p.spaceAfter
		return LayoutPlan{Status: LayoutFull, Consumed: consumed}
	}

	lineHeight := maxFontSize * p.leading
	wordLines := p.wrapWords(measured, area.Width)

	// Compute heights and split at available height.
	type lineInfo struct {
		words       []Word
		width       float64
		isLast      bool
		spaceBefore float64
		spaceAfter  float64
	}

	infos := make([]lineInfo, len(wordLines))
	for i, wl := range wordLines {
		w := 0.0
		for _, word := range wl {
			w += word.Width
			if i > 0 || len(wl) > 1 {
				w += word.SpaceAfter
			}
		}
		// Recompute width properly.
		w = lineWidth(wl)
		infos[i] = lineInfo{
			words:  wl,
			width:  w,
			isLast: i == len(wordLines)-1,
		}
	}
	if len(infos) > 0 {
		infos[0].spaceBefore = p.spaceBefore
		infos[len(infos)-1].spaceAfter = p.spaceAfter
	}

	// Determine how many lines fit.
	// area.Height <= 0 means no space left — nothing fits.
	if area.Height <= 0 {
		return LayoutPlan{Status: LayoutNothing}
	}
	totalH := 0.0
	splitIdx := len(infos)
	for i, info := range infos {
		h := lineHeight
		if i == 0 {
			h += info.spaceBefore
		}
		if i == len(infos)-1 {
			h += info.spaceAfter
		}
		if totalH+h > area.Height && i > 0 {
			splitIdx = i
			break
		}
		totalH += h
	}

	// Build placed blocks for fitted lines.
	blocks := make([]PlacedBlock, 0, splitIdx)
	curY := 0.0

	for i := range splitIdx {
		info := infos[i]
		if i == 0 {
			curY += info.spaceBefore
		}

		x := 0.0
		lineMaxW := area.Width
		// Apply first-line indent to the first line only.
		if i == 0 && p.firstIndent != 0 {
			x += p.firstIndent
			lineMaxW -= p.firstIndent
		}
		switch p.align {
		case AlignCenter:
			x += (lineMaxW - info.width) / 2
		case AlignRight:
			x += lineMaxW - info.width
		}

		// Capture for closure.
		capturedWords := slices.Clone(info.words)
		capturedIsLast := info.isLast || i == splitIdx-1
		capturedWidth := lineMaxW
		capturedAlign := p.align
		capturedBg := p.background
		capturedLineH := lineHeight

		block := PlacedBlock{
			X:      x,
			Y:      curY,
			Width:  info.width,
			Height: lineHeight,
			Tag:    "P",
			Draw: func(ctx DrawContext, absX, absTopY float64) {
				if capturedBg != nil {
					drawBackground(ctx, *capturedBg, absX, absTopY, capturedWidth, capturedLineH)
				}
				drawTextLine(ctx, capturedWords, absX, absTopY-capturedLineH, capturedWidth, capturedAlign, capturedIsLast)
			},
		}
		blocks = append(blocks, block)
		curY += lineHeight
		if i == splitIdx-1 {
			curY += info.spaceAfter
		}
	}

	if splitIdx >= len(infos) {
		return LayoutPlan{
			Status:   LayoutFull,
			Consumed: curY,
			Blocks:   blocks,
		}
	}

	// Build overflow paragraph from remaining words.
	var overflowWords []Word
	for i := splitIdx; i < len(infos); i++ {
		overflowWords = append(overflowWords, infos[i].words...)
	}
	overflow := p.cloneWithWords(overflowWords)
	overflow.spaceBefore = 0 // no spaceBefore on continuation

	return LayoutPlan{
		Status:   LayoutPartial,
		Consumed: curY,
		Blocks:   blocks,
		Overflow: overflow,
	}
}

// measureWords flattens all runs into measured words.
func (p *Paragraph) measureWords(maxWidth float64) ([]Word, float64) {
	var measured []Word
	var maxFontSize float64

	for _, run := range p.runs {
		measurer := runMeasurer(run)
		spaceW := measurer.MeasureString(" ", run.FontSize) + run.WordSpacing
		words := splitWords(run.Text)
		for _, w := range words {
			wordW := measurer.MeasureString(w, run.FontSize)
			if run.LetterSpacing != 0 && len([]rune(w)) > 1 {
				wordW += run.LetterSpacing * float64(len([]rune(w))-1)
			}
			measured = append(measured, Word{
				Text:            w,
				Width:           wordW,
				Font:            run.Font,
				Embedded:        run.Embedded,
				FontSize:        run.FontSize,
				Color:           run.Color,
				Decoration:      run.Decoration,
				DecorationColor: run.DecorationColor,
				DecorationStyle: run.DecorationStyle,
				SpaceAfter:      spaceW,
				LetterSpacing:   run.LetterSpacing,
				WordSpacing:     run.WordSpacing,
			})
		}
		if run.FontSize > maxFontSize {
			maxFontSize = run.FontSize
		}
	}

	measured = breakLongWords(measured, maxWidth)
	return measured, maxFontSize
}

// wrapWords performs greedy word-wrapping, returning groups of words per line.
func (p *Paragraph) wrapWords(words []Word, maxWidth float64) [][]Word {
	if len(words) == 0 {
		return nil
	}
	var lines [][]Word
	lineStart := 0
	// First line has reduced width for text-indent.
	effectiveWidth := maxWidth
	if p.firstIndent != 0 {
		effectiveWidth = maxWidth - p.firstIndent
	}
	lw := words[0].Width

	for i := 1; i < len(words); i++ {
		spaceW := words[i-1].SpaceAfter
		candidate := lw + spaceW + words[i].Width
		if candidate > effectiveWidth && lineStart < i {
			lines = append(lines, slices.Clone(words[lineStart:i]))
			lineStart = i
			lw = words[i].Width
			effectiveWidth = maxWidth // subsequent lines use full width
		} else {
			lw = candidate
		}
	}
	lines = append(lines, slices.Clone(words[lineStart:]))
	return lines
}

// lineWidth computes the content width of a word slice.
func lineWidth(words []Word) float64 {
	if len(words) == 0 {
		return 0
	}
	w := 0.0
	for i, word := range words {
		w += word.Width
		if i < len(words)-1 {
			w += word.SpaceAfter
		}
	}
	return w
}

// truncateWithEllipsis truncates a line's words so the total width plus "..."
// fits within maxWidth. The ellipsis is appended to the last visible word's text.
func truncateWithEllipsis(line Line, maxWidth float64) Line {
	if len(line.Words) == 0 {
		return line
	}

	// Measure "..." using the last word's font metrics.
	lastWord := line.Words[len(line.Words)-1]
	var ellipsisW float64
	if lastWord.Embedded != nil {
		ellipsisW = lastWord.Embedded.MeasureString("...", lastWord.FontSize)
	} else if lastWord.Font != nil {
		ellipsisW = lastWord.Font.MeasureString("...", lastWord.FontSize)
	} else {
		// Approximate: 3 dots at ~0.3em each.
		ellipsisW = lastWord.FontSize * 0.9
	}

	// Find how many words fit with room for "...".
	w := 0.0
	cutIdx := len(line.Words)
	for i, word := range line.Words {
		nextW := w + word.Width
		if i > 0 {
			nextW += line.Words[i-1].SpaceAfter
		}
		if nextW+ellipsisW > maxWidth && i > 0 {
			cutIdx = i
			break
		}
		w = nextW
	}

	if cutIdx >= len(line.Words) {
		// Everything fits — just append ellipsis to the last word.
		words := slices.Clone(line.Words)
		last := &words[len(words)-1]
		last.Text += "..."
		last.Width += ellipsisW
		line.Words = words
		line.Width = lineWidth(words)
		return line
	}

	words := slices.Clone(line.Words[:cutIdx])
	if len(words) > 0 {
		last := &words[len(words)-1]
		last.Text += "..."
		last.Width += ellipsisW
	}
	line.Words = words
	line.Width = lineWidth(words)
	return line
}

// cloneWithWords creates a new Paragraph with the same style but different words.
// Used to create overflow paragraphs during splitting.
func (p *Paragraph) cloneWithWords(words []Word) *Paragraph {
	// Reconstruct runs from words (simplified: one run per word group with same styling).
	var runs []TextRun
	if len(words) > 0 {
		// Group consecutive words with the same font/color into runs.
		cur := TextRun{
			Text:            words[0].Text,
			Font:            words[0].Font,
			Embedded:        words[0].Embedded,
			FontSize:        words[0].FontSize,
			Color:           words[0].Color,
			Decoration:      words[0].Decoration,
			DecorationColor: words[0].DecorationColor,
			DecorationStyle: words[0].DecorationStyle,
		}
		for _, w := range words[1:] {
			if w.Font == cur.Font && w.Embedded == cur.Embedded &&
				w.FontSize == cur.FontSize && w.Color == cur.Color &&
				w.Decoration == cur.Decoration {
				cur.Text += " " + w.Text
			} else {
				runs = append(runs, cur)
				cur = TextRun{
					Text:            w.Text,
					Font:            w.Font,
					Embedded:        w.Embedded,
					FontSize:        w.FontSize,
					Color:           w.Color,
					Decoration:      w.Decoration,
					DecorationColor: w.DecorationColor,
					DecorationStyle: w.DecorationStyle,
				}
			}
		}
		runs = append(runs, cur)
	}

	return &Paragraph{
		runs:       runs,
		leading:    p.leading,
		align:      p.align,
		spaceAfter: p.spaceAfter,
		background: p.background,
		// firstIndent is NOT propagated — it only applies to the first line
		orphans: p.orphans,
		widows:  p.widows,
	}
}
