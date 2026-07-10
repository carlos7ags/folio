// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package grapheme

import (
	"reflect"
	"strings"
	"testing"
)

// TestNextBreakMatchesBreaks checks that walking NextBreak from 0 to
// len(s) reproduces Breaks(s) exactly, for every corpus string. This is
// the safety net for switching allocation-heavy Breaks callers over to
// the incremental walker.
func TestNextBreakMatchesBreaks(t *testing.T) {
	corpus := []string{
		"",
		"a",
		"AVAILABLE To Watch",
		"éf",
		"काख",
		"\U0001F1FA\U0001F1F8\U0001F1E9\U0001F1EA",
		"\U0001F469\u200d\U0001F4BB",
		"؀٢",
		"ﺍﻠ",
		strings.Repeat("x", 65), // cross the ASCII fast path in Breaks
	}

	for _, s := range corpus {
		want := Breaks(s)

		walked := []int{0}
		for i := 0; i < len(s); {
			i = NextBreak(s, i)
			walked = append(walked, i)
		}

		if !reflect.DeepEqual(walked, want) {
			t.Errorf("NextBreak walk over %q: got %v, want %v (Breaks)", s, walked, want)
		}
	}
}
