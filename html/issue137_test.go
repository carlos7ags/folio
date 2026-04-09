// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// Regression tests for issue #137: `!important` cascade tiers for author
// declarations must follow the CSS spec order:
//
//	tier 0: stylesheet normal
//	tier 1: inline normal
//	tier 2: stylesheet !important
//	tier 3: inline !important
//
// Before the fix, inline declarations were unconditionally appended after
// stylesheet declarations, so a non-important inline rule would override a
// stylesheet !important rule (backwards from the spec).

// paragraphColor runs Convert on the given HTML, asserts the first element
// is a Paragraph, and returns the color of its first text run. Bail out on
// any structural mismatch so the tier-specific tests stay concise.
func paragraphColor(t *testing.T, src string) layout.Color {
	t.Helper()
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(elems) == 0 {
		t.Fatal("no elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}
	lines := p.Layout(500)
	if len(lines) == 0 || len(lines[0].Words) == 0 {
		t.Fatal("paragraph has no words")
	}
	return lines[0].Words[0].Color
}

// TestIssue137_StylesheetImportantBeatsInlineNormal — the canonical repro
// from the issue. A stylesheet rule marked `!important` must win over a
// non-important inline declaration.
func TestIssue137_StylesheetImportantBeatsInlineNormal(t *testing.T) {
	src := `<html><head><style>
		p { color: red !important; }
	</style></head><body>
		<p style="color: blue">hello</p>
	</body></html>`
	got := paragraphColor(t, src)
	want := layout.RGB(1, 0, 0) // red
	if got != want {
		t.Errorf("stylesheet !important should beat inline normal: got %+v, want %+v", got, want)
	}
}

// TestIssue137_InlineImportantBeatsStylesheetImportant — tier 3 (inline
// !important) must beat tier 2 (stylesheet !important). Uses hex colors
// to sidestep minor rounding differences between named-color tables.
func TestIssue137_InlineImportantBeatsStylesheetImportant(t *testing.T) {
	src := `<html><head><style>
		p { color: #ff0000 !important; }
	</style></head><body>
		<p style="color: #00ff00 !important">hello</p>
	</body></html>`
	got := paragraphColor(t, src)
	want := layout.RGB(0, 1, 0)
	if got != want {
		t.Errorf("inline !important should beat stylesheet !important: got %+v, want %+v", got, want)
	}
}

// TestIssue137_InlineNormalBeatsStylesheetNormal — tier 1 beats tier 0.
// Baseline behavior that must not regress with the cascade rework.
func TestIssue137_InlineNormalBeatsStylesheetNormal(t *testing.T) {
	src := `<html><head><style>
		p { color: red; }
	</style></head><body>
		<p style="color: blue">hello</p>
	</body></html>`
	got := paragraphColor(t, src)
	want := layout.RGB(0, 0, 1) // blue
	if got != want {
		t.Errorf("inline normal should beat stylesheet normal: got %+v, want %+v", got, want)
	}
}

// TestIssue137_StylesheetImportantBeatsStylesheetNormal — tier 2 beats tier 0.
// Existing specificity-based behavior, kept as a cascade-regression guard.
func TestIssue137_StylesheetImportantBeatsStylesheetNormal(t *testing.T) {
	src := `<html><head><style>
		.a { color: red !important; }
		p  { color: blue; }
	</style></head><body>
		<p class="a">hello</p>
	</body></html>`
	got := paragraphColor(t, src)
	want := layout.RGB(1, 0, 0)
	if got != want {
		t.Errorf("stylesheet !important should beat stylesheet normal: got %+v, want %+v", got, want)
	}
}

// TestIssue137_InlineImportantStripsFromValue — the `!important` suffix
// must be stripped from the value before the property parser sees it, so
// a color like "blue !important" resolves to "blue" (not rejected as
// malformed).
func TestIssue137_InlineImportantStripsFromValue(t *testing.T) {
	src := `<p style="color: blue !important">hi</p>`
	got := paragraphColor(t, src)
	want := layout.RGB(0, 0, 1)
	if got != want {
		t.Errorf("inline !important value must strip the suffix before parsing: got %+v, want %+v", got, want)
	}
}

// TestIssue137_CascadeOrderAcrossAllFourTiers — mixes all four tiers on the
// same property and verifies tier 3 wins. Guards against any future
// regression that re-orders the cascade.
func TestIssue137_CascadeOrderAcrossAllFourTiers(t *testing.T) {
	src := `<html><head><style>
		p { color: red; }            /* tier 0 */
		p { color: orange !important; } /* tier 2 */
	</style></head><body>
		<p style="color: yellow; color: lime !important">hi</p>
	</body></html>`
	// tier 1 = yellow (inline normal — loses to tier 2)
	// tier 3 = lime   (inline important — wins)
	got := paragraphColor(t, src)
	want := layout.RGB(0, 1, 0) // CSS "lime" = #00ff00
	if got != want {
		t.Errorf("tier 3 (inline !important) should win: got %+v, want %+v", got, want)
	}
}
