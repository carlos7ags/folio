// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"math"
	"regexp"
	"strings"
)

// Glyph assembly thresholds for text search matching.
const (
	// glyphLineThreshold is the vertical distance (as a multiple of glyph
	// width) beyond which two glyphs are considered on different lines.
	glyphLineThreshold = 2.0

	// glyphSpaceThreshold is the horizontal gap (as a fraction of glyph
	// width) beyond which a space is inserted between glyphs on the same line.
	glyphSpaceThreshold = 0.3

	// matchBoxPad is the padding (in points) added around each redaction
	// mark rectangle to ensure full coverage of the target characters.
	matchBoxPad = 0.5

	// lineGroupThreshold is the fraction of estimated line height within
	// which two glyphs are considered part of the same text line.
	lineGroupThreshold = 0.5
)

// findTextMarks locates all occurrences of target strings on a page and
// returns RedactionMark rectangles for each. Uses glyph-level extraction
// for character-precise bounding boxes.
func findTextMarks(r *PdfReader, pageIdx int, targets []string) ([]RedactionMark, error) {
	glyphs, err := pageGlyphs(r, pageIdx)
	if err != nil {
		return nil, err
	}
	if len(glyphs) == 0 {
		return nil, nil
	}

	// Build the full page text from glyphs.
	text, indices := assembleGlyphText(glyphs)

	var marks []RedactionMark
	lower := strings.ToLower(text)
	for _, target := range targets {
		tLower := strings.ToLower(target)
		start := 0
		for {
			idx := strings.Index(lower[start:], tLower)
			if idx < 0 {
				break
			}
			absIdx := start + idx
			rects := glyphsToRects(glyphs, indices, absIdx, absIdx+len(tLower))
			for _, rect := range rects {
				marks = append(marks, RedactionMark{Page: pageIdx, Rect: rect})
			}
			start = absIdx + len(tLower)
		}
	}
	return marks, nil
}

// findPatternMarks locates all regex matches on a page and returns
// RedactionMark rectangles.
func findPatternMarks(r *PdfReader, pageIdx int, pattern *regexp.Regexp) ([]RedactionMark, error) {
	glyphs, err := pageGlyphs(r, pageIdx)
	if err != nil {
		return nil, err
	}
	if len(glyphs) == 0 {
		return nil, nil
	}

	text, indices := assembleGlyphText(glyphs)

	var marks []RedactionMark
	locs := pattern.FindAllStringIndex(text, -1)
	for _, loc := range locs {
		rects := glyphsToRects(glyphs, indices, loc[0], loc[1])
		for _, rect := range rects {
			marks = append(marks, RedactionMark{Page: pageIdx, Rect: rect})
		}
	}
	return marks, nil
}

// pageGlyphs extracts per-glyph spans from a page.
func pageGlyphs(r *PdfReader, pageIdx int) ([]GlyphSpan, error) {
	page, err := r.Page(pageIdx)
	if err != nil {
		return nil, err
	}
	data, err := page.ContentStream()
	if err != nil {
		return nil, err
	}
	ops := ParseContentStream(data)

	resources, _ := page.Resources()
	fonts := BuildFontCache(resources, r.resolver)

	proc := NewContentProcessor(fonts)
	proc.SetExtractGlyphs(true)
	proc.Process(ops)
	return proc.Glyphs(), nil
}

// assembleGlyphText builds a string from glyph characters and returns a
// mapping from byte offset in the string to glyph index. Spaces are
// inserted between glyphs that are far apart horizontally.
func assembleGlyphText(glyphs []GlyphSpan) (string, []int) {
	var sb strings.Builder
	var indices []int // indices[byteOffset] = glyphIndex

	for i, g := range glyphs {
		// Insert a space between glyphs on the same line that have a gap.
		if i > 0 {
			prev := glyphs[i-1]
			sameLine := math.Abs(g.Y-prev.Y) < prev.Width*glyphLineThreshold
			gap := g.X - (prev.X + prev.Width)
			if sameLine && gap > prev.Width*glyphSpaceThreshold {
				for range len(" ") {
					indices = append(indices, i)
				}
				sb.WriteByte(' ')
			}
			// Different line — insert newline.
			if !sameLine {
				for range len("\n") {
					indices = append(indices, i)
				}
				sb.WriteByte('\n')
			}
		}
		s := string(g.Char)
		for range len(s) {
			indices = append(indices, i)
		}
		sb.WriteString(s)
	}
	return sb.String(), indices
}

// glyphsToRects converts a character range in the assembled text back
// to bounding box rectangles. If the range spans multiple lines, one
// rect per line is returned. Uses a fixed font height estimate based
// on the average glyph width (approximation since GlyphSpan doesn't
// carry font size directly).
func glyphsToRects(glyphs []GlyphSpan, indices []int, startByte, endByte int) []Box {
	if startByte >= len(indices) || endByte > len(indices) {
		return nil
	}

	firstGlyph := indices[startByte]
	lastGlyph := indices[endByte-1]

	// Estimate line height from average glyph width (typical font has
	// width ~= 0.5 * height, so height ~= 2 * avgWidth).
	avgW := 0.0
	count := 0
	for gi := firstGlyph; gi <= lastGlyph && gi < len(glyphs); gi++ {
		if glyphs[gi].Width > 0 {
			avgW += glyphs[gi].Width
			count++
		}
	}
	lineH := 12.0 // fallback
	if count > 0 {
		lineH = (avgW / float64(count)) * 2.0
	}

	// Group by line (Y coordinate).
	type lineRange struct {
		minX, maxX, y float64
	}
	var lines []lineRange

	for gi := firstGlyph; gi <= lastGlyph && gi < len(glyphs); gi++ {
		g := glyphs[gi]
		if len(lines) == 0 || math.Abs(g.Y-lines[len(lines)-1].y) > lineH*lineGroupThreshold {
			lines = append(lines, lineRange{
				minX: g.X, maxX: g.X + g.Width, y: g.Y,
			})
		} else {
			lr := &lines[len(lines)-1]
			if g.X < lr.minX {
				lr.minX = g.X
			}
			end := g.X + g.Width
			if end > lr.maxX {
				lr.maxX = end
			}
		}
	}

	// Convert to tight boxes. Match rects use reduced vertical extent
	// (cap-height + shallow descender) to avoid overlapping adjacent
	// lines in tightly-spaced paragraphs. The rewriter's charBounds uses
	// full typographic proportions for overlap testing.
	const (
		matchAscender  = 0.65 // cap-height fraction (smaller than full typAscender)
		matchDescender = 0.15 // shallow descender (just enough for baseline chars)
	)
	var rects []Box
	for _, lr := range lines {
		rects = append(rects, Box{
			X1: lr.minX - matchBoxPad,
			Y1: lr.y - lineH*matchDescender - matchBoxPad,
			X2: lr.maxX + matchBoxPad,
			Y2: lr.y + lineH*matchAscender + matchBoxPad,
		})
	}
	return rects
}
