// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "slices"

// SplitAfterLine wraps the paragraph at maxWidth and splits it after
// the first n rendered lines. Returns:
//   - head: a paragraph containing the first min(n, total) lines.
//   - tail: a paragraph containing the remaining lines, or nil if no
//     overflow (n >= total).
//
// If n <= 0, head is nil and tail is the entire paragraph.
//
// Both returned paragraphs are clones — the receiver is unchanged.
// Styling (fonts, colors, links, decorations, inline elements, shaped
// scripts) is preserved across the split via the same word-grouping
// rules used for page-break splits.
//
// Spacing ownership: SpaceBefore stays with head only (the tail picks
// up where the head left off mid-paragraph; spaceBefore would double).
// SpaceAfter stays with tail only (symmetric reason). FirstLineIndent
// is dropped from both halves — re-laying the head at a different
// width re-applies indent to its first line, and the tail by definition
// is not the start of a paragraph.
//
// Safe to call concurrently with other read methods on the receiver.
//
// Re-laying the returned head/tail at a different maxWidth than the
// split is supported but may change line counts (the words are real
// — not pre-wrapped). Callers that want stable line counts should
// re-lay at the same width they passed to SplitAfterLine.
func (p *Paragraph) SplitAfterLine(n int, maxWidth float64) (head, tail *Paragraph) {
	lines := p.Layout(maxWidth)
	if n <= 0 {
		// Tail is the entire paragraph — preserves both spacings,
		// since cloneWithWords already propagates spaceAfter and we
		// restore spaceBefore explicitly here.
		t := p.cloneWithWords(flattenLineWords(lines))
		t.spaceBefore = p.spaceBefore
		return nil, t
	}
	if n >= len(lines) {
		// Head is the entire paragraph — preserve both spacings.
		h := p.cloneWithWords(flattenLineWords(lines))
		h.spaceBefore = p.spaceBefore
		return h, nil
	}

	headWords := flattenLineWords(lines[:n])
	tailWords := flattenLineWords(lines[n:])

	head = p.cloneWithWords(headWords)
	tail = p.cloneWithWords(tailWords)

	// cloneWithWords does not propagate spaceBefore (it's used for
	// page-split tails where spaceBefore is unwanted). For SplitAfterLine
	// the head IS the same paragraph just with fewer lines, so propagate
	// it explicitly. SpaceAfter belongs to tail; SpaceBefore belongs to
	// head — never both.
	head.spaceBefore = p.spaceBefore
	head.spaceAfter = 0
	tail.spaceBefore = 0
	return head, tail
}

// flattenLineWords concatenates words across lines, marking the first
// word of every line after the first with LineBreak=true so cloneWithWords
// reconstructs explicit \n separators. This preserves the wrap exactly
// as Layout produced it, even after a round-trip through cloneWithWords.
//
// Word fields (including pointer fields like *Color and *TextShadow)
// are copied by value here; the cloned paragraph shares those pointers
// with the source. Standard Go semantics — caller mutation of the
// shared color/shadow would affect both halves.
func flattenLineWords(lines []Line) []Word {
	total := 0
	for _, line := range lines {
		total += len(line.Words)
	}
	words := make([]Word, 0, total)
	for i, line := range lines {
		for j, w := range line.Words {
			if i > 0 && j == 0 {
				w.LineBreak = true
			}
			words = append(words, w)
		}
	}
	return words
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

	lineHeight := resolveLineHeight(p.leading, maxFontSize, p.runs)
	wordLines := p.wrapWords(measured, area.Width)

	// Bidi reordering: resolve the paragraph's base direction and
	// reorder each line's words into visual order for correct
	// left-to-right rendering of RTL and mixed-direction text.
	resolvedDir := DirectionLTR
	for i, wl := range wordLines {
		reordered, dir := resolveLineBidi(wl, p.direction)
		wordLines[i] = reordered
		if i == 0 {
			resolvedDir = dir
		}
	}
	// When direction is explicitly set (CSS direction:rtl or HTML
	// dir="rtl"), use it for alignment even if bidi resolved LTR.
	if p.direction != DirectionAuto {
		resolvedDir = p.direction
	}

	// Apply ellipsis truncation: mirrors Paragraph.Layout so the render
	// path (PlanLayout) and the measure/height path (Layout) truncate
	// identically. Firing this collapses wordLines to a single line, so
	// an ellipsis paragraph can never produce Overflow below.
	// applyEllipsisWords decides whether truncation actually applies — it
	// fires for multi-line overflow *and* for a single line wider than the
	// area (the white-space:nowrap case, which always yields one line, so
	// this cannot be gated on line count here).
	if p.ellipsis && len(wordLines) > 0 {
		widths := make([]float64, len(wordLines))
		for i, wl := range wordLines {
			widths[i] = lineWidth(wl)
		}
		wordLines, _, _ = applyEllipsisWords(wordLines, widths, area.Width)
	}

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

		// bgX is the line box's left edge (first-line indent applied,
		// no alignment offset). The text rendering origin adds the
		// alignment offset on top of bgX. Separating these keeps the
		// paragraph background pinned to the content-box width
		// instead of riding along with center/right-aligned text —
		// otherwise the background leaks past the line's right edge
		// by the alignment offset (CSS Backgrounds & Borders L3 §2.1).
		bgX := 0.0
		lineMaxW := area.Width
		if i == 0 && p.firstIndent != 0 {
			bgX += p.firstIndent
			lineMaxW -= p.firstIndent
		}
		effectiveAlign := p.align
		// RTL paragraphs default to right-aligned unless the caller
		// explicitly called SetAlign. This matches CSS behavior where
		// `direction: rtl` flips the default text-align to "right".
		if resolvedDir == DirectionRTL && !p.alignSet {
			effectiveAlign = AlignRight
		}
		if p.textAlignLastSet && (info.isLast || i == splitIdx-1) {
			effectiveAlign = p.textAlignLast
		}
		alignOffset := 0.0
		switch effectiveAlign {
		case AlignCenter:
			alignOffset = (lineMaxW - info.width) / 2
		case AlignRight:
			alignOffset = lineMaxW - info.width
		}
		x := bgX + alignOffset

		// Capture for closure.
		capturedWords := slices.Clone(info.words)
		capturedIsLast := info.isLast || i == splitIdx-1
		capturedWidth := lineMaxW
		capturedAlign := effectiveAlign
		capturedBg := p.background
		capturedLineH := lineHeights[i]
		capturedAlignOffset := alignOffset

		// Build child blocks for inline-block words. Positions are
		// line-relative (starting at 0); the parent PlacedBlock's X
		// already carries the alignment offset, and the renderer adds
		// parent X to child X when drawing (render_plans.go).
		var inlineChildren []PlacedBlock
		inlineX := 0.0
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
			inlineX += w.Width + w.SpaceAfter
		}

		block := PlacedBlock{
			X:      x,
			Y:      curY,
			Width:  info.width,
			Height: capturedLineH,
			Tag:    "P",
			Draw: func(ctx DrawContext, absX, absTopY float64) {
				if capturedBg != nil {
					drawBackground(ctx, *capturedBg, absX-capturedAlignOffset, absTopY, capturedWidth, capturedLineH)
				}
				baseline := computeBaseline(capturedWords, capturedLineH)
				drawTextLine(ctx, capturedWords, absX, absTopY-baseline, capturedWidth, capturedAlign, capturedIsLast)
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

// wordToRun converts a Word back to a TextRun, preserving all styling fields.
//
// For shaped scripts (Arabic, Indic), Word.Text holds post-shape codepoints
// (Presentation Forms-B for Arabic, post-reorder logical text for Indic)
// while Word.OriginalText holds the pre-shape input. The reconstructed
// TextRun must use the pre-shape text so that re-laying the cloned
// paragraph re-runs the shaper from scratch and re-populates OriginalText
// for /ActualText marked-content recovery (ISO 32000-2 §14.9.4). Without
// this, accessibility and copy/paste fidelity silently regress on any
// paragraph that crosses a page-split boundary.
//
// Word.InlineBlock holds the inline element (image, SVG, etc.) and round-
// trips to TextRun.InlineElement. Word.InlineWidth and Word.InlineHeight
// are cached measurements that re-compute during Layout; they do not need
// to round-trip. Word.GIDs are shaper output; TextRun has no GIDs field,
// so the shaper regenerates them on re-layout from the pre-shape input
// in TextRun.Text.
//
// Note: hyphenated words produced by hyphenateWord do not enter this
// path — hyphenation runs only inside Layout(), not in the wrapWords
// path used by PlanLayout. If hyphenation is ever wired into PlanLayout,
// the trailing "-" word and its rest pair will need a join-without-space
// fix in cloneWithWords (see issue tracking breakLongWords field loss).
func wordToRun(w Word) TextRun {
	text := w.Text
	if w.OriginalText != "" {
		text = w.OriginalText
	}
	return TextRun{
		Text:            text,
		Font:            w.Font,
		Embedded:        w.Embedded,
		FontSize:        w.FontSize,
		Color:           w.Color,
		Decoration:      w.Decoration,
		DecorationColor: w.DecorationColor,
		DecorationStyle: w.DecorationStyle,
		BaselineShift:   w.BaselineShift,
		LetterSpacing:   w.LetterSpacing,
		WordSpacing:     w.WordSpacing,
		LinkURI:         w.LinkURI,
		TextShadow:      w.TextShadow,
		BackgroundColor: w.BackgroundColor,
		InlineElement:   w.InlineBlock,
	}
}

// cloneWithWords creates a new Paragraph with the same style but different words.
// Used to create overflow paragraphs during splitting.
func (p *Paragraph) cloneWithWords(words []Word) *Paragraph {
	// Reconstruct runs from words. Group consecutive words with identical
	// styling into a single run. All Word-level styling fields must be
	// compared and preserved to avoid losing baseline shift, letter spacing,
	// links, highlights, etc. on page-split paragraphs.
	var runs []TextRun
	if len(words) > 0 {
		cur := wordToRun(words[0])
		// If the first word is a blank line (from \n\n), start text with
		// "\n" so splitWords produces a lineBreakMarker for the empty line.
		if words[0].Text == "" && words[0].LineBreak {
			cur.Text = "\n"
		}
		// Track the SpaceAfter of the most recently appended word so we
		// can decide whether to insert a separator before the next word
		// in the same run. The discriminator is SpaceAfter==0 vs >0,
		// not script — glueAdjacentRuns zeros SpaceAfter on Latin words
		// at run boundaries, breakCJKWords zeros it between CJK chars,
		// and breakLongWords does the same for character-broken words.
		// Re-measurement on the cloned paragraph regenerates the actual
		// space width for the destination font/size, so we only need
		// to faithfully encode "was there a space here" as a bool.
		prevSpaceAfter := words[0].SpaceAfter
		for _, w := range words[1:] {
			// Blank words (from consecutive \n\n) represent empty lines.
			// Serialize as "\n" so splitWords regenerates lineBreakMarkers.
			// Blank words have no visible text so style doesn't matter.
			//
			// Exclude inline elements: they also have Text="" but they
			// are NOT blank-line markers. SplitAtLine's flattenLineWords
			// sets LineBreak=true on the first word of every line after
			// the first, which can land on an inline element — without
			// this guard the inline would be silently dropped.
			if w.Text == "" && w.LineBreak && w.InlineBlock == nil {
				cur.Text += "\n"
				// A blank line resets the join state — whatever
				// follows starts a fresh visual line and should not
				// inherit the pre-blank-line word's SpaceAfter.
				prevSpaceAfter = 0
				continue
			}
			// Inline elements never merge with text or with another inline
			// element — each one needs its own TextRun so the renderer can
			// place the embedded element at the right point in the run list.
			isInline := w.InlineBlock != nil
			curInline := cur.InlineElement != nil
			sameRun := !isInline && !curInline &&
				w.Font == cur.Font && w.Embedded == cur.Embedded &&
				w.FontSize == cur.FontSize && w.Color == cur.Color &&
				w.Decoration == cur.Decoration && w.BaselineShift == cur.BaselineShift &&
				w.LetterSpacing == cur.LetterSpacing && w.WordSpacing == cur.WordSpacing &&
				w.LinkURI == cur.LinkURI && w.BackgroundColor == cur.BackgroundColor
			// A word with LineBreak=true had a forced \n before it.
			if w.LineBreak {
				if sameRun {
					cur.Text += "\n" + w.Text
				} else {
					// Style changes at line break: put \n at end of
					// current run, flush it, start new run for w.
					cur.Text += "\n"
					runs = append(runs, cur)
					cur = wordToRun(w)
				}
				prevSpaceAfter = w.SpaceAfter
				continue
			}
			if sameRun {
				if prevSpaceAfter == 0 {
					cur.Text += w.Text
				} else {
					cur.Text += " " + w.Text
				}
			} else {
				runs = append(runs, cur)
				cur = wordToRun(w)
			}
			prevSpaceAfter = w.SpaceAfter
		}
		runs = append(runs, cur)
	}

	return &Paragraph{
		runs:             runs,
		leading:          p.leading,
		noWrap:           p.noWrap,
		align:            p.align,
		alignSet:         p.alignSet,
		direction:        p.direction,
		spaceAfter:       p.spaceAfter,
		background:       p.background,
		wordBreak:        p.wordBreak,
		hyphens:          p.hyphens,
		textAlignLast:    p.textAlignLast,
		textAlignLastSet: p.textAlignLastSet,
		ellipsis:         p.ellipsis,
		// firstIndent is NOT propagated — it only applies to the first line
		orphans: p.orphans,
		widows:  p.widows,
	}
}
