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
			threshold := lineH * 0.25
			if span.SpaceWidth > 0 {
				threshold = span.SpaceWidth * 0.5
			}
			if gap > threshold {
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
		} else {
			threshold := lineH * 0.25
			if span.SpaceWidth > 0 {
				threshold = span.SpaceWidth * 0.5
			}
			if span.X-prevEndX > threshold {
				// Word gap on same line.
				if len(result) > 0 && result[len(result)-1] != ' ' {
					result = append(result, ' ')
				}
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

// --- TaggedStrategy ---

// TaggedStrategy extracts text in logical reading order using the PDF
// structure tree. Falls back to position-based ordering for untagged content.
type TaggedStrategy struct {
	tree    *StructureTree
	spans   []TextSpan
	pageNum int // 0-based page index for filtering spans
}

// NewTaggedStrategy creates a strategy that uses the structure tree to
// determine reading order. pageNum is the 0-based page index.
func NewTaggedStrategy(tree *StructureTree, pageNum int) *TaggedStrategy {
	return &TaggedStrategy{tree: tree, pageNum: pageNum}
}

func (s *TaggedStrategy) ProcessSpan(span TextSpan) {
	if !span.Visible {
		return
	}
	s.spans = append(s.spans, span)
}

// blockLevelTags contains structure types that should produce line breaks
// between them in the output text.
var blockLevelTags = map[string]bool{
	"P": true, "H": true, "H1": true, "H2": true, "H3": true,
	"H4": true, "H5": true, "H6": true, "Div": true,
	"Table": true, "TR": true, "TBody": true, "THead": true, "TFoot": true,
	"L": true, "LI": true, "LBody": true,
	"BlockQuote": true, "Caption": true, "TOC": true, "TOCI": true,
	"Part": true, "Sect": true, "Art": true,
}

func (s *TaggedStrategy) Result() string {
	if len(s.spans) == 0 {
		return ""
	}

	// Group spans by MCID.
	mcidSpans := map[int][]TextSpan{}
	var untagged []TextSpan
	for _, span := range s.spans {
		if span.MCID < 0 {
			untagged = append(untagged, span)
		} else {
			mcidSpans[span.MCID] = append(mcidSpans[span.MCID], span)
		}
	}

	var result []byte
	prevIsBlock := false

	// Walk the structure tree in document order and collect text.
	if s.tree != nil && s.tree.Root != nil {
		s.walkNode(s.tree.Root, mcidSpans, &result, &prevIsBlock)
	}

	// Append untagged spans at the end.
	if len(untagged) > 0 {
		if len(result) > 0 && result[len(result)-1] != '\n' {
			result = append(result, '\n')
		}
		for _, span := range untagged {
			result = append(result, span.Text...)
		}
	}

	return string(result)
}

// walkNode recursively walks the structure tree and appends text for
// leaf nodes (those with MCID >= 0) in document order.
func (s *TaggedStrategy) walkNode(node *StructNode, mcidSpans map[int][]TextSpan, result *[]byte, prevIsBlock *bool) {
	isBlock := blockLevelTags[node.Tag]

	// If this is a leaf node with an MCID, emit its text.
	if node.MCID >= 0 {
		spans, ok := mcidSpans[node.MCID]
		if ok && len(spans) > 0 {
			if isBlock && len(*result) > 0 && (*result)[len(*result)-1] != '\n' {
				*result = append(*result, '\n')
			} else if !isBlock && *prevIsBlock && len(*result) > 0 && (*result)[len(*result)-1] != '\n' {
				*result = append(*result, '\n')
			} else if !isBlock && len(*result) > 0 && (*result)[len(*result)-1] != '\n' && (*result)[len(*result)-1] != ' ' {
				*result = append(*result, ' ')
			}
			for _, span := range spans {
				*result = append(*result, span.Text...)
			}
			*prevIsBlock = isBlock
		}
		return
	}

	// Insert block separator before block-level elements.
	if isBlock && len(node.Children) > 0 && len(*result) > 0 && (*result)[len(*result)-1] != '\n' {
		*result = append(*result, '\n')
	}

	// Recurse into children.
	for _, child := range node.Children {
		s.walkNode(child, mcidSpans, result, prevIsBlock)
	}

	// Insert newline after block-level elements.
	if isBlock && len(*result) > 0 && (*result)[len(*result)-1] != '\n' {
		*result = append(*result, '\n')
	}

	*prevIsBlock = isBlock
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
