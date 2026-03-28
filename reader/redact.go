// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"fmt"
	"regexp"
)

// RedactOptions configures the redaction behavior.
type RedactOptions struct {
	// FillColor is the RGB color of the redaction box (0-1 each).
	// Default: black [0, 0, 0].
	FillColor [3]float64

	// OverlayText is optional text drawn centered on each redaction box
	// (e.g. "REDACTED", "[CONFIDENTIAL]").
	OverlayText string

	// OverlayFontSize is the font size for overlay text. Default: 8.
	OverlayFontSize float64

	// OverlayColor is the RGB color for overlay text. Default: white [1, 1, 1].
	OverlayColor [3]float64

	// StripMetadata removes document-level metadata (/Info dictionary,
	// /Metadata XMP stream, /PieceInfo) from the output.
	StripMetadata bool
}

// fillColor returns the fill color with defaults applied.
func (o *RedactOptions) fillColor() (r, g, b float64) {
	if o == nil {
		return 0, 0, 0
	}
	return o.FillColor[0], o.FillColor[1], o.FillColor[2]
}

// overlayColor returns the overlay text color with defaults applied.
func (o *RedactOptions) overlayColor() (r, g, b float64) {
	if o == nil {
		return 1, 1, 1
	}
	c := o.OverlayColor
	if c == [3]float64{} {
		return 1, 1, 1
	}
	return c[0], c[1], c[2]
}

// overlayFontSize returns the overlay font size with defaults applied.
func (o *RedactOptions) overlayFontSize() float64 {
	if o == nil || o.OverlayFontSize <= 0 {
		return 8
	}
	return o.OverlayFontSize
}

// RedactionMark identifies a rectangular area on a page to be redacted.
type RedactionMark struct {
	Page int // 0-based page index
	Rect Box // region in user-space coordinates (lower-left origin)
}

// RedactRegions redacts specified rectangular areas from a PDF. It removes
// text operators whose bounding boxes overlap any redaction mark, draws
// replacement boxes, and returns a Modifier with the sanitized output.
//
// The redacted content is permanently removed from the content streams —
// not merely hidden behind a visual overlay.
func RedactRegions(r *PdfReader, marks []RedactionMark, opts *RedactOptions) (*Modifier, error) {
	if len(marks) == 0 {
		return Merge(r)
	}
	if opts == nil {
		opts = &RedactOptions{}
	}

	// Create a writable copy of the input PDF.
	m, err := Merge(r)
	if err != nil {
		return nil, fmt.Errorf("redact: copy PDF: %w", err)
	}

	// Group marks by page.
	pageMarks := make(map[int][]Box)
	for _, mark := range marks {
		pageMarks[mark.Page] = append(pageMarks[mark.Page], mark.Rect)
	}

	// Process each affected page.
	for pageIdx, rects := range pageMarks {
		if pageIdx < 0 || pageIdx >= r.PageCount() {
			continue
		}

		// Get the page's content stream and font cache.
		page, err := r.Page(pageIdx)
		if err != nil {
			return nil, fmt.Errorf("redact: page %d: %w", pageIdx, err)
		}
		data, err := page.ContentStream()
		if err != nil {
			return nil, fmt.Errorf("redact: page %d content: %w", pageIdx, err)
		}
		resources, _ := page.Resources()
		fonts := buildFontCache(resources, r.resolver)

		// Rewrite the content stream (remove overlapping text ops).
		rewritten := rewriteContentStream(data, rects, fonts)

		// Build the overlay (opaque boxes + optional text).
		overlay := buildRedactionOverlay(rects, opts)

		// Apply to the modifier's page.
		if err := applyRedaction(m, pageIdx, rewritten, overlay, opts); err != nil {
			return nil, err
		}
	}

	// Strip metadata if requested.
	if opts.StripMetadata {
		stripDocumentMetadata(m)
	}

	return m, nil
}

// RedactText finds all occurrences of the given strings in the PDF and
// redacts them. It uses glyph-level positioning to determine precise
// bounding boxes for each match.
//
// The search is case-insensitive. Each occurrence across all pages is
// redacted with the same options.
func RedactText(r *PdfReader, targets []string, opts *RedactOptions) (*Modifier, error) {
	if len(targets) == 0 {
		return Merge(r)
	}

	var allMarks []RedactionMark
	for pageIdx := range r.PageCount() {
		marks, err := findTextMarks(r, pageIdx, targets)
		if err != nil {
			return nil, fmt.Errorf("redact: scan page %d: %w", pageIdx, err)
		}
		allMarks = append(allMarks, marks...)
	}

	return RedactRegions(r, allMarks, opts)
}

// RedactPattern finds all text matching a regular expression and redacts it.
// Uses glyph-level extraction for character-precise bounding boxes.
func RedactPattern(r *PdfReader, pattern *regexp.Regexp, opts *RedactOptions) (*Modifier, error) {
	if pattern == nil {
		return Merge(r)
	}

	var allMarks []RedactionMark
	for pageIdx := range r.PageCount() {
		marks, err := findPatternMarks(r, pageIdx, pattern)
		if err != nil {
			return nil, fmt.Errorf("redact: scan page %d: %w", pageIdx, err)
		}
		allMarks = append(allMarks, marks...)
	}

	return RedactRegions(r, allMarks, opts)
}
