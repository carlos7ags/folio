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
	runs             []TextRun
	leading          float64 // line height multiplier (e.g. 1.2 means 120% of fontSize)
	align            Align
	spaceBefore      float64           // extra space before the paragraph (points)
	spaceAfter       float64           // extra space after the paragraph (points)
	background       *Color            // background fill color (nil = transparent)
	firstIndent      float64           // first-line indent (points, from CSS text-indent)
	orphans          int               // min lines at bottom of page before break (0 = disabled)
	widows           int               // min lines at top of page after break (0 = disabled)
	ellipsis         bool              // if true, truncate overflowing text with "..."
	wordBreak        string            // "normal" (default), "break-all" (allow break within words)
	hyphens          string            // "none", "manual" (default), "auto" (automatic hyphenation)
	textAlignLast    Align             // alignment for the last line (0 = use default)
	textAlignLastSet bool              // true if textAlignLast was explicitly set
	stringSets       map[string]string // CSS string-set values to capture
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
// Panics if any text run has both Font and Embedded nil.
// Runs with InlineElement set are exempt from the font requirement.
func NewStyledParagraph(runs ...TextRun) *Paragraph {
	for i, r := range runs {
		if r.InlineElement != nil {
			continue // inline elements don't need fonts
		}
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
// Panics if the run has both Font and Embedded nil (unless InlineElement is set).
func (p *Paragraph) AddRun(r TextRun) *Paragraph {
	if r.InlineElement == nil && r.Font == nil && r.Embedded == nil {
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

// GetSpaceBefore returns the extra vertical space before the paragraph.
func (p *Paragraph) GetSpaceBefore() float64 { return p.spaceBefore }

// GetSpaceAfter returns the extra vertical space after the paragraph.
func (p *Paragraph) GetSpaceAfter() float64 { return p.spaceAfter }

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

// SetWordBreak sets the word-break behavior. "break-all" allows breaking
// within any word at character boundaries. Default is "normal" (break at spaces only).
func (p *Paragraph) SetWordBreak(wb string) *Paragraph {
	p.wordBreak = wb
	return p
}

// SetHyphens sets the hyphenation mode. "auto" enables automatic hyphenation
// at syllable boundaries. "none" disables all hyphenation. "manual" (default)
// only breaks at soft hyphens (&shy;).
func (p *Paragraph) SetHyphens(h string) *Paragraph {
	p.hyphens = h
	return p
}

// SetTextAlignLast sets the alignment for the last line of the paragraph.
// This is used to override the normal alignment (e.g. justify) for just
// the final line.
func (p *Paragraph) SetTextAlignLast(a Align) *Paragraph {
	p.textAlignLast = a
	p.textAlignLastSet = true
	return p
}

// SetStringSet attaches a CSS string-set value to this paragraph.
func (p *Paragraph) SetStringSet(name, value string) *Paragraph {
	if p.stringSets == nil {
		p.stringSets = make(map[string]string)
	}
	p.stringSets[name] = value
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

	for i, run := range p.runs {
		// Inline element runs produce a single InlineBlock word.
		if run.InlineElement != nil {
			plan := run.InlineElement.PlanLayout(LayoutArea{
				Width: maxWidth, Height: 1e6,
			})
			var iw, ih float64
			if plan.Status != LayoutNothing && len(plan.Blocks) > 0 {
				iw = plan.Blocks[0].Width
				ih = plan.Consumed
			}
			measured = append(measured, Word{
				InlineBlock:  run.InlineElement,
				InlineWidth:  iw,
				InlineHeight: ih,
				Width:        iw,
				SpaceAfter:   inlineSpaceAfter(measured, p.runs, i),
			})
			continue
		}

		measurer := runMeasurer(run)
		spaceW := measurer.MeasureString(" ", run.FontSize) + run.WordSpacing
		words := splitWords(run.Text)

		nextLineBreak := false
		for _, w := range words {
			if w == lineBreakMarker {
				nextLineBreak = true
				continue
			}
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
				LineBreak:       nextLineBreak,
				DecorationColor: run.DecorationColor,
				DecorationStyle: run.DecorationStyle,
				SpaceAfter:      spaceW,
				LetterSpacing:   run.LetterSpacing,
				WordSpacing:     run.WordSpacing,
				BaselineShift:   run.BaselineShift,
				LinkURI:         run.LinkURI,
				TextShadow:      run.TextShadow,
				BackgroundColor: run.BackgroundColor,
			})
			nextLineBreak = false
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

	// Break words that don't fit. With word-break:break-all, break ALL words
	// at character boundaries to fill lines maximally.
	if p.wordBreak == "break-all" {
		measured = breakAllWords(measured, maxWidth)
	} else {
		measured = breakLongWords(measured, maxWidth)
	}

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
		// Forced line break from \n.
		if measured[i].LineBreak {
			lines = append(lines, Line{
				Words: slices.Clone(measured[lineStart:i]),
				Width: lineWidth, Height: lineHeight, SpaceW: measured[lineStart].SpaceAfter,
			})
			lineStart = i
			lineWidth = measured[i].Width
			effectiveMax = maxWidth
			continue
		}
		spaceW := measured[i-1].SpaceAfter
		candidate := lineWidth + spaceW + measured[i].Width
		if candidate > effectiveMax && lineStart < i {
			// Try hyphenation: if enabled, attempt to break the next word
			// and fit part of it on this line with a hyphen.
			if p.hyphens == "auto" {
				remaining := effectiveMax - lineWidth - spaceW
				if part, rest, ok := hyphenateWord(measured[i], remaining); ok {
					// Fit the first part on this line.
					lineWords := make([]Word, i-lineStart+1)
					copy(lineWords, measured[lineStart:i])
					lineWords[len(lineWords)-1] = part
					lw := lineWidth + spaceW + part.Width
					lines = append(lines, buildLine(lineWords, lw, lineHeight, p.align, false))
					measured[i] = rest
					lineStart = i
					lineWidth = rest.Width
					effectiveMax = maxWidth
					continue
				}
			}
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

	// Apply text-align-last: override alignment on the last line.
	if p.textAlignLastSet && len(lines) > 0 {
		lines[len(lines)-1].Align = p.textAlignLast
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

// inlineSpaceAfter computes the space-after width for an inline element word.
// It uses surrounding text metrics when available: first from already-measured
// words (preceding context), then by scanning forward in the runs for the
// next text run (following context). When no text context exists at all
// (e.g. a paragraph of only inline elements), it returns 0 so the elements
// sit flush against each other.
func inlineSpaceAfter(measured []Word, runs []TextRun, currentIdx int) float64 {
	// Preceding word has known metrics — inherit its spacing.
	if len(measured) > 0 {
		return measured[len(measured)-1].SpaceAfter
	}
	// Look ahead for the next text run to derive font-based spacing.
	for j := currentIdx + 1; j < len(runs); j++ {
		r := runs[j]
		if r.InlineElement != nil {
			continue
		}
		m := runMeasurer(r)
		if m == nil {
			continue
		}
		return m.MeasureString(" ", r.FontSize) + r.WordSpacing
	}
	// No text context at all — flush spacing.
	return 0
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

// hyphenateWord attempts to split a word to fit within `available` points
// using the Liang-Knuth hyphenation algorithm for linguistically correct
// syllable breaks. Returns the first part (with trailing hyphen) and the
// remainder. Returns ok=false if no valid split point is found.
func hyphenateWord(w Word, available float64) (part, rest Word, ok bool) {
	runes := []rune(w.Text)
	if len(runes) < 4 {
		return Word{}, Word{}, false
	}

	var measure func(string) float64
	if w.Embedded != nil {
		measure = func(s string) float64 { return w.Embedded.MeasureString(s, w.FontSize) }
	} else if w.Font != nil {
		measure = func(s string) float64 { return w.Font.MeasureString(s, w.FontSize) }
	} else {
		return Word{}, Word{}, false
	}

	hyphenW := measure("-")

	// Get linguistically valid break points from the hyphenator.
	// Only attempt pattern-based hyphenation for pure-alpha words;
	// fall back to character-boundary splitting for others.
	var breakPoints []int
	if isAlphaWord(w.Text) {
		breakPoints = DefaultHyphenator().Hyphenate(w.Text)
	}

	// Find the latest valid break point that fits.
	bestSplit := -1
	if len(breakPoints) > 0 {
		for i := len(breakPoints) - 1; i >= 0; i-- {
			bp := breakPoints[i]
			prefix := string(runes[:bp])
			pw := measure(prefix) + hyphenW
			if pw <= available {
				bestSplit = bp
				break
			}
		}
	}

	// Fallback: if no pattern-based break fits, try character boundaries
	// (at least 2 chars from each end) for very long words.
	if bestSplit < 0 {
		for i := len(runes) - 2; i >= 2; i-- {
			prefix := string(runes[:i])
			pw := measure(prefix) + hyphenW
			if pw <= available {
				bestSplit = i
				break
			}
		}
	}

	if bestSplit < 0 {
		return Word{}, Word{}, false
	}

	prefixText := string(runes[:bestSplit]) + "-"
	suffixText := string(runes[bestSplit:])

	part = w
	part.Text = prefixText
	part.Width = measure(prefixText)
	part.SpaceAfter = 0

	rest = w
	rest.Text = suffixText
	rest.Width = measure(suffixText)

	return part, rest, true
}

// breakAllWords breaks every word into individual characters, allowing
// the word-wrap algorithm to break within any word (word-break: break-all).
func breakAllWords(words []Word, maxWidth float64) []Word {
	var result []Word
	for _, w := range words {
		runes := []rune(w.Text)
		if len(runes) <= 1 {
			result = append(result, w)
			continue
		}
		measurer := w.Font
		emb := w.Embedded
		var measure func(string) float64
		if emb != nil {
			measure = func(s string) float64 { return emb.MeasureString(s, w.FontSize) }
		} else if measurer != nil {
			measure = func(s string) float64 { return measurer.MeasureString(s, w.FontSize) }
		} else {
			result = append(result, w)
			continue
		}
		// Split into individual characters as separate "words".
		for j, r := range runes {
			ch := string(r)
			cw := Word{
				Text:          ch,
				Width:         measure(ch),
				Font:          w.Font,
				Embedded:      w.Embedded,
				FontSize:      w.FontSize,
				Color:         w.Color,
				Decoration:    w.Decoration,
				LetterSpacing: w.LetterSpacing,
				WordSpacing:   w.WordSpacing,
				SpaceAfter:    0, // no space between characters of same word
			}
			if j == len(runes)-1 {
				cw.SpaceAfter = w.SpaceAfter // preserve inter-word space on last char
			}
			result = append(result, cw)
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
			if run.LetterSpacing != 0 && len([]rune(w)) > 1 {
				ww += run.LetterSpacing * float64(len([]rune(w))-1)
			}
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
			ww := measurer.MeasureString(w, run.FontSize)
			if run.LetterSpacing != 0 && len([]rune(w)) > 1 {
				ww += run.LetterSpacing * float64(len([]rune(w))-1)
			}
			total += ww
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
// splitWords splits text into words, preserving \n as a lineBreakMarker
// sentinel that forces a line break during word-wrapping.
func splitWords(text string) []string {
	// Normalize \r\n and bare \r to \n, then split on newlines.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	lines := strings.Split(text, "\n")
	var result []string
	for i, line := range lines {
		if i > 0 {
			result = append(result, lineBreakMarker)
		}
		result = append(result, strings.Fields(line)...)
	}
	return result
}

// lineBreakMarker is a sentinel value in the word list that signals a
// forced line break from a \n character in the source text.
const lineBreakMarker = "\x00linebreak"

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

	// Compute per-line height: max of text line height and any inline-block heights.
	lineHeights := make([]float64, len(infos))
	for i, info := range infos {
		lh := lineHeight
		for _, w := range info.words {
			if w.InlineBlock != nil && w.InlineHeight > lh {
				lh = w.InlineHeight
			}
		}
		lineHeights[i] = lh
	}

	// Determine how many lines fit.
	// area.Height <= 0 means no space left — nothing fits.
	if area.Height <= 0 {
		return LayoutPlan{Status: LayoutNothing}
	}
	totalH := 0.0
	splitIdx := len(infos)
	for i, info := range infos {
		h := lineHeights[i]
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
		effectiveAlign := p.align
		if p.textAlignLastSet && (info.isLast || i == splitIdx-1) {
			effectiveAlign = p.textAlignLast
		}
		switch effectiveAlign {
		case AlignCenter:
			x += (lineMaxW - info.width) / 2
		case AlignRight:
			x += lineMaxW - info.width
		}

		// Capture for closure.
		capturedWords := slices.Clone(info.words)
		capturedIsLast := info.isLast || i == splitIdx-1
		capturedWidth := lineMaxW
		capturedAlign := effectiveAlign
		capturedBg := p.background
		capturedLineH := lineHeights[i]

		// Build child blocks for inline-block words.
		var inlineChildren []PlacedBlock
		inlineX := x
		for _, w := range info.words {
			if w.InlineBlock != nil {
				ibPlan := w.InlineBlock.PlanLayout(LayoutArea{
					Width: w.InlineWidth, Height: w.InlineHeight,
				})
				for _, ib := range ibPlan.Blocks {
					ib.X += inlineX
					ib.Y += capturedLineH - w.InlineHeight
					inlineChildren = append(inlineChildren, ib)
				}
			}
			inlineX += w.Width
			if w.InlineBlock != nil {
				inlineX += w.SpaceAfter
			} else {
				inlineX += w.SpaceAfter
			}
		}

		block := PlacedBlock{
			X:      x,
			Y:      curY,
			Width:  info.width,
			Height: capturedLineH,
			Tag:    "P",
			Draw: func(ctx DrawContext, absX, absTopY float64) {
				if capturedBg != nil {
					drawBackground(ctx, *capturedBg, absX, absTopY, capturedWidth, capturedLineH)
				}
				drawTextLine(ctx, capturedWords, absX, absTopY-capturedLineH, capturedWidth, capturedAlign, capturedIsLast)
			},
			Children: inlineChildren,
		}
		// Compute precise link annotations for every distinct link URI
		// in this line. Each linked span gets its own annotation rect.
		block.Links = linkSpans(info.words)
		// Attach string-set values on the first block.
		if i == 0 && len(p.stringSets) > 0 {
			block.StringSets = p.stringSets
		}
		blocks = append(blocks, block)
		curY += capturedLineH
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
//
// When a run's text starts with punctuation and no leading whitespace
// (e.g. ". Then" after a bold run), the leading punctuation characters
// are appended to the last word of the previous run. This produces
// "here." as one word instead of "here" + "." as two, matching standard
// typographic behavior at style boundaries.
func (p *Paragraph) measureWords(maxWidth float64) ([]Word, float64) {
	var measured []Word
	var maxFontSize float64

	for i, run := range p.runs {
		// Inline element runs produce a single InlineBlock word.
		if run.InlineElement != nil {
			plan := run.InlineElement.PlanLayout(LayoutArea{
				Width: maxWidth, Height: 1e6,
			})
			var iw, ih float64
			if plan.Status != LayoutNothing && len(plan.Blocks) > 0 {
				iw = plan.Blocks[0].Width
				ih = plan.Consumed
			}
			measured = append(measured, Word{
				InlineBlock:  run.InlineElement,
				InlineWidth:  iw,
				InlineHeight: ih,
				Width:        iw,
				SpaceAfter:   inlineSpaceAfter(measured, p.runs, i),
			})
			continue
		}

		measurer := runMeasurer(run)
		spaceW := measurer.MeasureString(" ", run.FontSize) + run.WordSpacing
		text := run.Text

		// If the run starts with punctuation (no leading space) and we
		// already have words, append the punctuation to the previous word.
		// The punctuation renders in the previous word's font, which is
		// visually correct — "here." should look like one word.
		if len(measured) > 0 && len(text) > 0 && !isSpace(rune(text[0])) {
			punct, rest := splitLeadingPunct(text)
			if punct != "" {
				prev := &measured[len(measured)-1]
				prev.Text += punct
				prevMeasurer := wordMeasurer(*prev)
				prev.Width = prevMeasurer.MeasureString(prev.Text, prev.FontSize)
				if prev.LetterSpacing != 0 {
					prev.Width += prev.LetterSpacing * float64(len([]rune(prev.Text))-1)
				}
				text = rest
			}
		}

		words := splitWords(text)
		nextLineBreak := false
		for _, w := range words {
			if w == lineBreakMarker {
				nextLineBreak = true
				continue
			}
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
				BaselineShift:   run.BaselineShift,
				LinkURI:         run.LinkURI,
				TextShadow:      run.TextShadow,
				BackgroundColor: run.BackgroundColor,
				LineBreak:       nextLineBreak,
			})
			nextLineBreak = false
		}
		if run.FontSize > maxFontSize {
			maxFontSize = run.FontSize
		}
	}

	measured = breakLongWords(measured, maxWidth)
	return measured, maxFontSize
}

// splitLeadingPunct splits a string into a leading punctuation prefix
// and the remainder. Returns ("", s) if s does not start with punctuation.
func splitLeadingPunct(s string) (punct, rest string) {
	i := 0
	for _, r := range s {
		if isPunctuation(r) {
			i += len(string(r))
		} else {
			break
		}
	}
	if i == 0 {
		return "", s
	}
	return s[:i], s[i:]
}

// isPunctuation reports whether r is a punctuation character that should
// attach to the preceding word rather than stand alone.
func isPunctuation(r rune) bool {
	switch r {
	case '.', ',', ';', ':', '!', '?', ')', ']', '}', '"', '\'',
		'\u2019', '\u201D': // right single/double quotes
		return true
	}
	return false
}

// isSpace reports whether r is a whitespace character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// wordMeasurer returns a TextMeasurer for the given word's font.
func wordMeasurer(w Word) font.TextMeasurer {
	if w.Embedded != nil {
		return w.Embedded
	}
	return w.Font
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
		// Forced line break from \n in source text.
		if words[i].LineBreak {
			lines = append(lines, slices.Clone(words[lineStart:i]))
			lineStart = i
			lw = words[i].Width
			effectiveWidth = maxWidth
			continue
		}
		spaceW := words[i-1].SpaceAfter
		candidate := lw + spaceW + words[i].Width
		if candidate > effectiveWidth && lineStart < i {
			lines = append(lines, slices.Clone(words[lineStart:i]))
			lineStart = i
			lw = words[i].Width
			effectiveWidth = maxWidth
		} else {
			lw = candidate
		}
	}
	lines = append(lines, slices.Clone(words[lineStart:]))
	return lines
}

// linkSpans computes a LinkArea for every contiguous run of words that
// share the same non-empty LinkURI. Each span's X and W are relative to
// the line's starting x position. This supports multiple distinct links
// on the same line (e.g. "Visit GitHub or GitLab").
func linkSpans(words []Word) []LinkArea {
	var spans []LinkArea
	cx := 0.0
	i := 0
	for i < len(words) {
		uri := words[i].LinkURI
		if uri == "" {
			if i < len(words)-1 {
				cx += words[i].Width + words[i].SpaceAfter
			}
			i++
			continue
		}
		// Start of a linked span.
		startX := cx
		endX := cx + words[i].Width
		j := i + 1
		for j < len(words) && words[j].LinkURI == uri {
			// Extend through the space before this word.
			endX = cx
			for k := i; k < j; k++ {
				endX += words[k].Width + words[k].SpaceAfter
			}
			endX += words[j].Width
			j++
		}
		spans = append(spans, LinkArea{
			URI: uri,
			X:   startX,
			W:   endX - startX,
		})
		// Advance cx past all words in this span.
		for i < j {
			if i < len(words)-1 {
				cx += words[i].Width + words[i].SpaceAfter
			}
			i++
		}
	}
	return spans
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
