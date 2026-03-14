// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import "sort"

// ExtractionStrategy assembles text from a sequence of TextSpans.
// Different strategies produce different output: simple concatenation,
// spatial layout preservation, or region-filtered extraction.
type ExtractionStrategy interface {
	// ProcessSpan receives a single TextSpan. Called in content stream order.
	ProcessSpan(span TextSpan)

	// Result returns the final assembled text.
	Result() string
}

// --- SimpleStrategy ---

// SimpleStrategy concatenates text in content stream order, inserting
// spaces for gaps and newlines for line changes. This matches our
// original ExtractText behavior.
type SimpleStrategy struct {
	result  []byte
	prevX   float64
	prevY   float64
	hadText bool
}

func (s *SimpleStrategy) ProcessSpan(span TextSpan) {
	// Skip invisible text (Tr mode 3).
	if !span.Visible {
		return
	}
	if s.hadText {
		dy := span.Y - s.prevY
		if dy < 0 {
			dy = -dy
		}
		lineH := span.Height
		if lineH <= 0 {
			lineH = 12
		}

		if dy > lineH*0.5 {
			// Line change.
			s.appendNewline()
		} else {
			// Same line — check for word gap.
			gap := span.X - s.prevX
			if gap > lineH*0.25 {
				s.appendSpace()
			}
		}
	}

	s.result = append(s.result, span.Text...)
	s.prevX = span.X + span.Width
	s.prevY = span.Y
	s.hadText = true
}

func (s *SimpleStrategy) Result() string {
	return string(s.result)
}

func (s *SimpleStrategy) appendSpace() {
	if len(s.result) > 0 && s.result[len(s.result)-1] != ' ' && s.result[len(s.result)-1] != '\n' {
		s.result = append(s.result, ' ')
	}
}

func (s *SimpleStrategy) appendNewline() {
	if len(s.result) > 0 && s.result[len(s.result)-1] != '\n' {
		s.result = append(s.result, '\n')
	}
}

// --- LocationStrategy ---

// LocationStrategy sorts text by position (top-to-bottom, left-to-right)
// to reconstruct the visual layout of the page. This handles PDFs where
// text is drawn in non-reading order.
type LocationStrategy struct {
	spans []TextSpan
}

func (l *LocationStrategy) ProcessSpan(span TextSpan) {
	if !span.Visible {
		return
	}
	l.spans = append(l.spans, span)
}

func (l *LocationStrategy) Result() string {
	if len(l.spans) == 0 {
		return ""
	}

	// Sort by Y descending (top of page first), then X ascending (left to right).
	sort.Slice(l.spans, func(i, j int) bool {
		a, b := l.spans[i], l.spans[j]
		// Group by line: spans within 0.5 * height are on the same line.
		lineH := a.Height
		if lineH <= 0 {
			lineH = 12
		}
		dy := a.Y - b.Y
		if dy < 0 {
			dy = -dy
		}
		if dy > lineH*0.5 {
			return a.Y > b.Y // higher Y = higher on page
		}
		return a.X < b.X // same line: left to right
	})

	var result []byte
	prevY := l.spans[0].Y
	prevEndX := 0.0

	for _, span := range l.spans {
		lineH := span.Height
		if lineH <= 0 {
			lineH = 12
		}
		dy := span.Y - prevY
		if dy < 0 {
			dy = -dy
		}

		if dy > lineH*0.5 {
			// New line.
			if len(result) > 0 && result[len(result)-1] != '\n' {
				result = append(result, '\n')
			}
			prevEndX = 0
		} else if span.X-prevEndX > lineH*0.25 {
			// Word gap on same line.
			if len(result) > 0 && result[len(result)-1] != ' ' {
				result = append(result, ' ')
			}
		}

		result = append(result, span.Text...)
		prevY = span.Y
		prevEndX = span.X + span.Width
	}

	return string(result)
}

// --- RegionStrategy ---

// RegionStrategy extracts text only from spans that fall within
// a specified rectangle. Useful for extracting text from a specific
// area of a page (e.g., a header, footer, or form field).
type RegionStrategy struct {
	x, y, w, h float64 // region in user space
	inner      ExtractionStrategy
}

// NewRegionStrategy creates a strategy that filters to a rectangle.
// (x, y) is the bottom-left corner; w and h are dimensions.
// The inner strategy assembles the filtered text.
func NewRegionStrategy(x, y, w, h float64, inner ExtractionStrategy) *RegionStrategy {
	return &RegionStrategy{x: x, y: y, w: w, h: h, inner: inner}
}

func (r *RegionStrategy) ProcessSpan(span TextSpan) {
	// Check if span overlaps the region.
	if span.X+span.Width < r.x || span.X > r.x+r.w {
		return // outside horizontally
	}
	if span.Y < r.y || span.Y > r.y+r.h {
		return // outside vertically
	}
	r.inner.ProcessSpan(span)
}

func (r *RegionStrategy) Result() string {
	return r.inner.Result()
}

// --- Convenience functions ---

// ExtractWithStrategy runs the ContentProcessor and feeds spans to a strategy.
func ExtractWithStrategy(data []byte, fonts FontCache, strategy ExtractionStrategy) string {
	ops := ParseContentStream(data)
	proc := NewContentProcessor(fonts)
	spans := proc.Process(ops)
	for _, span := range spans {
		strategy.ProcessSpan(span)
	}
	return strategy.Result()
}
