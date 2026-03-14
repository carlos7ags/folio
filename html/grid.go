// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
)

// convertGrid converts a display:grid container into a layout.Grid.
func (c *converter) convertGrid(n *html.Node, style computedStyle) []layout.Element {
	grid := layout.NewGrid()

	// Parse track definitions.
	if style.GridTemplateColumns != "" {
		grid.SetTemplateColumns(parseGridTracks(style.GridTemplateColumns, style.FontSize))
	}
	if style.GridTemplateRows != "" {
		grid.SetTemplateRows(parseGridTracks(style.GridTemplateRows, style.FontSize))
	}

	// Gap.
	if style.Gap > 0 {
		grid.SetGap(style.Gap, style.Gap)
	}

	// Container styling.
	if style.hasPadding() {
		grid.SetPaddingAll(layout.Padding{
			Top:    style.PaddingTop,
			Right:  style.PaddingRight,
			Bottom: style.PaddingBottom,
			Left:   style.PaddingLeft,
		})
	}
	if style.hasBorder() {
		grid.SetBorders(buildCellBorders(style))
	}
	if style.BackgroundColor != nil {
		grid.SetBackground(*style.BackgroundColor)
	}

	// Add children and their placements.
	childIdx := 0
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		childElems := c.convertNode(child, style)
		if len(childElems) == 0 {
			continue
		}

		childStyle := style
		if child.Type == html.ElementNode {
			childStyle = c.computeElementStyle(child, style)
		}

		for _, elem := range childElems {
			grid.AddChild(elem)

			// Set placement if the child has explicit grid positioning.
			if childStyle.GridColumnStart != 0 || childStyle.GridColumnEnd != 0 ||
				childStyle.GridRowStart != 0 || childStyle.GridRowEnd != 0 {
				grid.SetPlacement(childIdx, layout.GridPlacement{
					ColStart: childStyle.GridColumnStart,
					ColEnd:   childStyle.GridColumnEnd,
					RowStart: childStyle.GridRowStart,
					RowEnd:   childStyle.GridRowEnd,
				})
			}
			childIdx++
		}
	}

	return []layout.Element{grid}
}

// parseGridTracks parses a CSS grid-template-columns/rows value into GridTrack slices.
// Supports: px, %, fr, auto, repeat(N, track).
func parseGridTracks(val string, fontSize float64) []layout.GridTrack {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil
	}

	var tracks []layout.GridTrack

	// Expand repeat() functions first.
	expanded := expandRepeat(val)

	tokens := tokenizeGridTemplate(expanded)
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		tracks = append(tracks, parseSingleGridTrack(tok, fontSize))
	}

	return tracks
}

// expandRepeat expands repeat(N, track) patterns in a grid template string.
// e.g. "repeat(3, 1fr)" -> "1fr 1fr 1fr"
// e.g. "200px repeat(2, 1fr) auto" -> "200px 1fr 1fr auto"
func expandRepeat(val string) string {
	result := val
	for {
		idx := strings.Index(strings.ToLower(result), "repeat(")
		if idx < 0 {
			break
		}
		// Find matching closing paren.
		depth := 0
		endIdx := -1
		for i := idx + 7; i < len(result); i++ {
			if result[i] == '(' {
				depth++
			} else if result[i] == ')' {
				if depth == 0 {
					endIdx = i
					break
				}
				depth--
			}
		}
		if endIdx < 0 {
			break
		}

		inner := result[idx+7 : endIdx]
		// Split on first comma: "N, track-list"
		commaIdx := strings.Index(inner, ",")
		if commaIdx < 0 {
			break
		}

		countStr := strings.TrimSpace(inner[:commaIdx])
		trackStr := strings.TrimSpace(inner[commaIdx+1:])

		count, err := strconv.Atoi(countStr)
		if err != nil || count <= 0 {
			break
		}

		var expanded []string
		for i := 0; i < count; i++ {
			expanded = append(expanded, trackStr)
		}

		result = result[:idx] + strings.Join(expanded, " ") + result[endIdx+1:]
	}
	return result
}

// tokenizeGridTemplate splits a grid template value into tokens,
// respecting parentheses (for minmax() etc.).
func tokenizeGridTemplate(val string) []string {
	var tokens []string
	var current strings.Builder
	depth := 0

	for _, ch := range val {
		switch {
		case ch == '(':
			depth++
			current.WriteRune(ch)
		case ch == ')':
			depth--
			current.WriteRune(ch)
		case ch == ' ' && depth == 0:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// parseSingleGridTrack parses a single track token like "1fr", "200px", "auto", "50%",
// or "minmax(min, max)" (simplified: uses the max value).
func parseSingleGridTrack(tok string, fontSize float64) layout.GridTrack {
	tok = strings.TrimSpace(strings.ToLower(tok))

	if tok == "auto" {
		return layout.GridTrack{Type: layout.GridTrackAuto}
	}

	// Handle minmax() — simplified: use the max value.
	if strings.HasPrefix(tok, "minmax(") && strings.HasSuffix(tok, ")") {
		inner := tok[7 : len(tok)-1]
		parts := strings.SplitN(inner, ",", 2)
		if len(parts) == 2 {
			return parseSingleGridTrack(strings.TrimSpace(parts[1]), fontSize)
		}
	}

	// fr unit.
	if strings.HasSuffix(tok, "fr") {
		numStr := strings.TrimSuffix(tok, "fr")
		if v, err := strconv.ParseFloat(numStr, 64); err == nil {
			return layout.GridTrack{Type: layout.GridTrackFr, Value: v}
		}
	}

	// Percentage.
	if strings.HasSuffix(tok, "%") {
		numStr := strings.TrimSuffix(tok, "%")
		if v, err := strconv.ParseFloat(numStr, 64); err == nil {
			return layout.GridTrack{Type: layout.GridTrackPercent, Value: v}
		}
	}

	// px, pt, em, rem — use parseLength for conversion to points.
	if l := parseLength(tok); l != nil {
		return layout.GridTrack{Type: layout.GridTrackPx, Value: l.toPoints(0, fontSize)}
	}

	// Fallback: try as a plain number (treat as px).
	if v, err := strconv.ParseFloat(tok, 64); err == nil {
		return layout.GridTrack{Type: layout.GridTrackPx, Value: v * 0.75}
	}

	return layout.GridTrack{Type: layout.GridTrackAuto}
}

// parseGridLine parses a CSS grid-column or grid-row value.
// Supported formats:
//   - "2"           -> start=2, end=0 (auto)
//   - "1 / 3"       -> start=1, end=3
//   - "1 / span 2"  -> start=1, end=3 (1 + 2)
//   - "span 2"      -> start=0 (auto), end=2 (stored as span count for auto-placement)
func parseGridLine(val string) (start, end int) {
	val = strings.TrimSpace(val)
	parts := strings.Split(val, "/")

	if len(parts) == 1 {
		// Single value: either a line number or "span N".
		tok := strings.TrimSpace(parts[0])
		if strings.HasPrefix(strings.ToLower(tok), "span") {
			numStr := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(tok), "span"))
			if n, err := strconv.Atoi(numStr); err == nil {
				return 0, n // span stored in end for auto-placement
			}
		}
		if n, err := strconv.Atoi(tok); err == nil {
			return n, 0
		}
		return 0, 0
	}

	// Two parts: "start / end" or "start / span N".
	startTok := strings.TrimSpace(parts[0])
	endTok := strings.TrimSpace(parts[1])

	if n, err := strconv.Atoi(startTok); err == nil {
		start = n
	}

	if strings.HasPrefix(strings.ToLower(endTok), "span") {
		numStr := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(endTok), "span"))
		if n, err := strconv.Atoi(numStr); err == nil {
			end = start + n
		}
	} else if n, err := strconv.Atoi(endTok); err == nil {
		end = n
	}

	return start, end
}
