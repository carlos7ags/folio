// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"unicode/utf8"

	"github.com/carlos7ags/folio/font"
)

// runMeasurer returns the text measurer for a run, defaulting to
// Helvetica if no font is set (defensive).
func runMeasurer(run TextRun) font.TextMeasurer {
	return resolveMeasurer(run.Embedded, run.Font, font.Helvetica)
}

// computeBaseline returns the distance from the top of the line box to the
// text baseline using CSS half-leading (CSS 2.1 §10.8.1):
//
//	leading = lineH - (ascent + descent)
//	half-leading = leading / 2
//	baseline from top = half-leading + ascent = (lineH + ascent - descent) / 2
//
// When a line has mixed font sizes, the largest baseline wins so that all
// text on the line shares a common baseline position.
func computeBaseline(words []Word, lineH float64) float64 {
	baseline := 0.0
	for _, w := range words {
		if w.InlineBlock != nil {
			continue
		}
		var ascent, descent float64
		if w.Font != nil {
			ascent = w.Font.Ascent(w.FontSize)
			descent = w.Font.Descent(w.FontSize)
		} else if w.Embedded != nil {
			face := w.Embedded.Face()
			upem := float64(face.UnitsPerEm())
			ascent = float64(face.Ascent()) / upem * w.FontSize
			descent = -float64(face.Descent()) / upem * w.FontSize
		}
		if ascent > 0 {
			wb := (lineH + ascent - descent) / 2
			if wb > baseline {
				baseline = wb
			}
		}
	}
	if baseline == 0 {
		baseline = lineH * 0.8
	}
	return baseline
}

// resolveLineHeight computes a paragraph's (uniform, whole-paragraph) line
// box height. When leading is [LeadingNormal], the height is derived from
// the vertical metrics of dominant (the run that set maxFontSize) instead
// of a flat multiplier — matching how browsers resolve CSS
// `line-height: normal` per the active font, including fonts that declare
// an unusually large line-gap. font.Standard (the base-14 PDF fonts) has no
// embedded vertical metrics to consult, so it falls back to the same 1.2
// approximation used before this existed.
func resolveLineHeight(leading, maxFontSize float64, dominant TextRun) float64 {
	if leading != LeadingNormal {
		return maxFontSize * leading
	}
	if dominant.Embedded != nil {
		return dominant.Embedded.NormalLineHeight(maxFontSize)
	}
	return maxFontSize * 1.2
}

// runsAdjacent returns true when run at index i directly abuts the previous
// run with no whitespace between them. This happens with inline elements like
// <sub>/<sup> where "C<sub>8</sub>" produces runs ["C", "8"] with no space.
// When true, the last word of the previous run should have SpaceAfter = 0
// so the words render flush against each other.
func runsAdjacent(runs []TextRun, i int) bool {
	if i <= 0 || len(runs) <= i {
		return false
	}
	cur := runs[i]
	prev := runs[i-1]
	// Skip inline element runs — they have their own spacing logic.
	if cur.InlineElement != nil || prev.InlineElement != nil {
		return false
	}
	if cur.Text == "" || prev.Text == "" {
		return false
	}
	// If previous run ends without whitespace and current starts without
	// whitespace, the runs are adjacent (no inter-word space).
	lastChar := prev.Text[len(prev.Text)-1]
	firstChar := cur.Text[0]
	return !isASCIISpace(lastChar) && !isASCIISpace(firstChar)
}

// isASCIISpace checks for ASCII whitespace. HTML parsers normalize most
// whitespace to ASCII, so this covers the practical cases. Non-ASCII
// whitespace (e.g. \u00A0 non-breaking space) is not treated as a
// separator, which matches browser behavior (NBSP doesn't break words).
func isASCIISpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

// glueAdjacentRuns zeroes SpaceAfter on the last measured word when the
// current run directly abuts the previous run with no whitespace.
func glueAdjacentRuns(measured []Word, runs []TextRun, runIdx int) {
	if runsAdjacent(runs, runIdx) && len(measured) > 0 {
		measured[len(measured)-1].SpaceAfter = 0
	}
}

// measureInlineElement measures an inline element run and returns a Word
// representing it as an inline-block in the paragraph's word stream.
// Both Layout() and measureWords() use this to keep measurement logic
// in one place.
func measureInlineElement(run TextRun, maxWidth float64, measured []Word, runs []TextRun, runIdx int) Word {
	plan := run.InlineElement.PlanLayout(LayoutArea{
		Width: maxWidth, Height: 1e6,
	})
	var iw, ih float64
	if plan.Status != LayoutNothing && len(plan.Blocks) > 0 {
		iw = plan.Blocks[0].Width
		ih = plan.Consumed
	}
	return Word{
		InlineBlock:  run.InlineElement,
		InlineWidth:  iw,
		InlineHeight: ih,
		Width:        iw,
		SpaceAfter:   inlineSpaceAfter(measured, runs, runIdx),
	}
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
		return runMeasurer(r).MeasureString(" ", r.FontSize) + r.WordSpacing
	}
	// No text context at all — flush spacing.
	return 0
}

// MinWidth implements Measurable. Returns the width of the longest word
// (the narrowest the paragraph can be without clipping).
func (p *Paragraph) MinWidth() float64 {
	maxWordW := 0.0
	for _, run := range p.runs {
		measurer := runMeasurer(run)
		for _, w := range splitWords(run.Text) {
			ww := measurer.MeasureString(w, run.FontSize)
			if run.LetterSpacing != 0 {
				if n := utf8.RuneCountInString(w); n > 1 {
					ww += run.LetterSpacing * float64(n-1)
				}
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
		// Inline elements (e.g. a nested inline-block) participate in the
		// same line as adjacent text. Treat them like a non-breaking word
		// that contributes its own max-content width plus the same
		// inter-run space the text branch adds below, so a fit-content
		// parent measures wide enough to contain the nested chip.
		if run.InlineElement != nil {
			total += inlineElementMaxWidth(run.InlineElement)
			// Add a space after the inline element (mirrors the per-run
			// trailing space added for text runs).
			total += inlineRunSpace(p.runs, run)
			continue
		}
		measurer := runMeasurer(run)
		words := splitWords(run.Text)
		spaceW := measurer.MeasureString(" ", run.FontSize)
		for i, w := range words {
			ww := measurer.MeasureString(w, run.FontSize)
			if run.LetterSpacing != 0 {
				if n := utf8.RuneCountInString(w); n > 1 {
					ww += run.LetterSpacing * float64(n-1)
				}
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

// inlineElementMaxWidth reports the max-content width of an inline element
// run. It prefers the element's own Measurable.MaxWidth(); otherwise it
// falls back to measuring via PlanLayout at a large width and reading the
// first block's width (mirroring how measureInlineElement extracts width).
func inlineElementMaxWidth(el Element) float64 {
	if m, ok := el.(Measurable); ok {
		return m.MaxWidth()
	}
	plan := el.PlanLayout(LayoutArea{Width: 1e6, Height: 1e6})
	if plan.Status != LayoutNothing && len(plan.Blocks) > 0 {
		return plan.Blocks[0].Width
	}
	return 0
}

// inlineRunSpace returns the inter-run trailing space to add after an inline
// element run, consistent with the per-run space the text branch of MaxWidth
// adds. It derives the space width from a measurer; inline element runs carry
// no font, so a fallback FontSize is used when none is present.
func inlineRunSpace(runs []TextRun, run TextRun) float64 {
	// An inline-element run typically has no font; derive the space width
	// from the run's own font metrics when available, otherwise fall back
	// to a neighboring text run's measurer.
	if run.Font != nil || run.Embedded != nil {
		return runMeasurer(run).MeasureString(" ", runFontSize(run))
	}
	for _, r := range runs {
		if r.InlineElement == nil && (r.Font != nil || r.Embedded != nil) {
			return runMeasurer(r).MeasureString(" ", r.FontSize)
		}
	}
	return 0
}

// runFontSize returns a non-zero font size for measuring, defaulting to a
// reasonable value when the run carries none.
func runFontSize(run TextRun) float64 {
	if run.FontSize > 0 {
		return run.FontSize
	}
	return 12
}

// MeasureLines reports how many wrapped lines this paragraph produces at
// maxWidth. Equivalent to len(p.Layout(maxWidth)). Useful for clamp
// decisions ("if MeasureLines > 8 then split") without inspecting words.
//
// Safe to call concurrently with other read methods (Layout, MeasureHeight)
// as long as no goroutine is mutating the paragraph through setters.
func (p *Paragraph) MeasureLines(maxWidth float64) int {
	return len(p.Layout(maxWidth))
}

// MeasureHeight reports the rendered height (in points) of this paragraph
// at maxWidth — the sum of each line's Height. Does NOT include
// SpaceBefore or SpaceAfter; the caller adds those if relevant.
//
// Safe to call concurrently with other read methods (Layout, MeasureLines)
// as long as no goroutine is mutating the paragraph through setters.
func (p *Paragraph) MeasureHeight(maxWidth float64) float64 {
	total := 0.0
	for _, line := range p.Layout(maxWidth) {
		total += line.Height
	}
	return total
}

// measureWords flattens all runs into measured words.
//
// When a run's text starts with punctuation and no leading whitespace
// (e.g. ". Then" after a bold run), the leading punctuation characters
// are appended to the last word of the previous run. This produces
// "here." as one word instead of "here" + "." as two, matching standard
// typographic behavior at style boundaries.
func (p *Paragraph) measureWords(maxWidth float64) ([]Word, float64, TextRun) {
	var measured []Word
	var maxFontSize float64
	var dominantRun TextRun

	nextLineBreakFromBr := false
	for i, run := range p.runs {
		if run.InlineElement != nil {
			glueAdjacentRuns(measured, p.runs, i)
			measured = append(measured, measureInlineElement(run, maxWidth, measured, p.runs, i))
			continue
		}
		if run.IsLineBreak {
			nextLineBreakFromBr = true
			continue
		}

		// Zero the previous word's SpaceAfter when this run abuts it
		// with no whitespace (e.g. "C" + "<sub>8</sub>").
		glueAdjacentRuns(measured, p.runs, i)

		measurer := runMeasurer(run)
		spaceW := measurer.MeasureString(" ", run.FontSize) + run.WordSpacing
		text := run.Text

		// If the run starts with punctuation (no leading space) and we
		// already have words, append the punctuation to the previous word.
		// The punctuation renders in the previous word's font, which is
		// visually correct — "here." should look like one word.
		// Skip merging when the previous word has different styling (font size
		// or baseline shift), as the punctuation should use the current run's
		// styling, not the previous word's. This prevents "th" (superscript)
		// from absorbing "!" (normal) in "7<sup>th</sup>! works".
		if len(measured) > 0 && len(text) > 0 && !isSpace(rune(text[0])) {
			prev := &measured[len(measured)-1]
			sameStyle := prev.Font == run.Font && prev.Embedded == run.Embedded &&
				prev.FontSize == run.FontSize && prev.BaselineShift == run.BaselineShift
			if sameStyle {
				punct, rest := splitLeadingPunct(text)
				if punct != "" {
					prev.Text += punct
					prevMeasurer := wordMeasurer(*prev)
					prevMeasureText := prev.Text
					if hasCounterPlaceholder(prevMeasureText) {
						prevMeasureText = expandCountersForMeasure(prevMeasureText)
					}
					prev.Width = prevMeasurer.MeasureString(prevMeasureText, prev.FontSize)
					if prev.LetterSpacing != 0 {
						prev.Width += prev.LetterSpacing * float64(utf8.RuneCountInString(prevMeasureText)-1)
					}
					text = rest
				}
			}
		}

		words := splitWords(text)
		nextLineBreak := nextLineBreakFromBr
		nextLineBreakFromBr = false
		for _, w := range words {
			if w == lineBreakMarker {
				if nextLineBreak {
					measured = append(measured, Word{
						Font:      run.Font,
						Embedded:  run.Embedded,
						FontSize:  run.FontSize,
						LineBreak: true,
					})
				}
				nextLineBreak = true
				continue
			}
			// Build the word from the unshaped text first so the bidi
			// split sees the original codepoints; each piece is then
			// shaped and its pre-shape text recorded for /ActualText.
			word := Word{
				Text:            w,
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
			}
			if subs := splitMixedBidiWord(word); subs != nil {
				for si, sub := range subs {
					shapeAndMeasureWord(&sub, run, measurer)
					if si == 0 {
						sub.LineBreak = nextLineBreak
					}
					measured = append(measured, sub)
				}
			} else {
				shapeAndMeasureWord(&word, run, measurer)
				measured = append(measured, word)
			}
			nextLineBreak = false
		}
		if run.FontSize > maxFontSize {
			maxFontSize = run.FontSize
			dominantRun = run
		}
	}

	if p.wordBreak != "keep-all" {
		measured = breakCJKWords(measured)
	}
	// Mirror the dispatch in measureWords (above): word-break:break-all
	// requires every word be split at character boundaries, not just the
	// over-long ones. Calling only breakLongWords here let break-all
	// silently degrade to default break behavior whenever re-measurement
	// was performed by this code path. white-space:nowrap leaves an
	// overlong word intact so it overflows instead of wrapping.
	if p.wordBreak == "break-all" {
		measured = breakAllWords(measured, maxWidth)
	} else if !p.noWrap {
		measured = breakLongWords(measured, maxWidth)
	}
	return measured, maxFontSize, dominantRun
}

// wordMeasurer returns a TextMeasurer for the given word's font.
// Fallback w.Font preserves the historical typed-nil return when unset.
func wordMeasurer(w Word) font.TextMeasurer {
	return resolveMeasurer(w.Embedded, w.Font, w.Font)
}
