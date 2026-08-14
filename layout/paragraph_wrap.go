// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "slices"

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

// applyEllipsisWords collapses multiple word-lines into a single truncated
// line when ellipsis is enabled and the text wraps to more than one line.
// It mirrors the firing condition in Paragraph.Layout (len(lines) > 1) and
// reuses truncateWithEllipsis so both the Layout and PlanLayout render
// paths share the exact same truncation logic. Returns the (possibly
// collapsed) word-lines, their widths, and whether truncation fired.
func applyEllipsisWords(wordLines [][]Word, widths []float64, maxWidth float64) ([][]Word, []float64, bool) {
	if len(wordLines) <= 1 {
		return wordLines, widths, false
	}
	first := Line{Words: wordLines[0], Width: widths[0]}
	truncated := truncateWithEllipsis(first, maxWidth)
	return [][]Word{truncated.Words}, []float64{truncated.Width}, true
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
