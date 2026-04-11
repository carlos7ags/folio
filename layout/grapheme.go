// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "github.com/carlos7ags/folio/unicode/grapheme"

// This file is a thin re-export shim. The extended grapheme cluster
// algorithm (UAX #29 §3.1.1, rules GB1–GB13) lives in the
// github.com/carlos7ags/folio/unicode/grapheme package so that lower
// layers — font metrics in particular — can use cluster boundaries
// without taking a dependency on layout. See that package for the full
// implementation, rule references, and the scope of the
// Extended_Pictographic approximation used by GB11.
//
// The legacy layout-level names (GraphemeBreaks / NextGraphemeBreak /
// GraphemeCount) remain available so existing callers in layout and
// bidi code compile unchanged. New callers should prefer the
// grapheme.Breaks / grapheme.NextBreak / grapheme.Count names from the
// package directly.

// GraphemeBreaks returns the byte offsets of grapheme cluster
// boundaries in s, including 0 and len(s). See
// grapheme.Breaks for the underlying implementation.
func GraphemeBreaks(s string) []int { return grapheme.Breaks(s) }

// NextGraphemeBreak returns the byte offset of the next cluster
// boundary strictly after start, or len(s) if start is already in the
// final cluster. See grapheme.NextBreak.
func NextGraphemeBreak(s string, start int) int { return grapheme.NextBreak(s, start) }

// GraphemeCount returns the number of extended grapheme clusters in s.
// See grapheme.Count.
func GraphemeCount(s string) int { return grapheme.Count(s) }

// isGraphemeBoundary reports whether byte offset pos in s is a
// grapheme cluster boundary. Used by splitMixedBidiWord to snap
// candidate split points to the nearest cluster boundary.
func isGraphemeBoundary(s string, pos int) bool { return grapheme.IsBoundary(s, pos) }
