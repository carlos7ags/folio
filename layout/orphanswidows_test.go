// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestOrphansSetKeepWithNext(t *testing.T) {
	// Create a paragraph with enough text to produce many lines.
	text := strings.Repeat("Word ", 100)
	p := NewParagraph(text, font.Helvetica, 12).SetOrphans(2)

	lines := p.Layout(200) // narrow width forces many lines
	if len(lines) < 5 {
		t.Fatalf("expected many lines, got %d", len(lines))
	}

	// First 2 lines should have KeepWithNext (orphan threshold).
	if !lines[0].KeepWithNext {
		t.Error("line 0 should have KeepWithNext (orphans=2)")
	}
	if !lines[1].KeepWithNext {
		t.Error("line 1 should have KeepWithNext (orphans=2)")
	}
}

func TestWidowsSetKeepWithNext(t *testing.T) {
	text := strings.Repeat("Word ", 100)
	p := NewParagraph(text, font.Helvetica, 12).SetWidows(2)

	lines := p.Layout(200)
	n := len(lines)
	if n < 5 {
		t.Fatalf("expected many lines, got %d", n)
	}

	// Lines near the end should have KeepWithNext to prevent widows.
	// Specifically, lines[n-3] and lines[n-2] should have KeepWithNext
	// so that if a break happens, at least 2 lines land on the next page.
	if !lines[n-3].KeepWithNext {
		t.Errorf("line %d should have KeepWithNext (widows=2)", n-3)
	}
	if !lines[n-2].KeepWithNext {
		t.Errorf("line %d should have KeepWithNext (widows=2)", n-2)
	}
}

func TestOrphansDisabledByDefault(t *testing.T) {
	text := strings.Repeat("Word ", 50)
	p := NewParagraph(text, font.Helvetica, 12) // no SetOrphans

	lines := p.Layout(200)
	// Line 0 should NOT have KeepWithNext by default.
	if lines[0].KeepWithNext {
		t.Error("line 0 should not have KeepWithNext without SetOrphans")
	}
}

func TestWidowsDisabledByDefault(t *testing.T) {
	text := strings.Repeat("Word ", 50)
	p := NewParagraph(text, font.Helvetica, 12) // no SetWidows

	lines := p.Layout(200)
	n := len(lines)
	// Second-to-last line should NOT have KeepWithNext by default.
	if lines[n-2].KeepWithNext {
		t.Error("should not have KeepWithNext without SetWidows")
	}
}

func TestOrphansShortParagraph(t *testing.T) {
	// Paragraph with fewer lines than orphan count should not panic.
	p := NewParagraph("Short", font.Helvetica, 12).SetOrphans(5)
	lines := p.Layout(400)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// No KeepWithNext on a single-line paragraph (nothing to keep with).
}

func TestOrphansAndWidowsCombined(t *testing.T) {
	text := strings.Repeat("Word ", 100)
	p := NewParagraph(text, font.Helvetica, 12).SetOrphans(3).SetWidows(3)

	lines := p.Layout(200)
	n := len(lines)
	if n < 8 {
		t.Fatalf("expected many lines, got %d", n)
	}

	// First 3 lines: KeepWithNext (orphans).
	for i := range 3 {
		if !lines[i].KeepWithNext {
			t.Errorf("line %d should have KeepWithNext (orphans=3)", i)
		}
	}

	// Last 3 lines before the final: KeepWithNext (widows).
	for i := n - 4; i < n-1; i++ {
		if !lines[i].KeepWithNext {
			t.Errorf("line %d should have KeepWithNext (widows=3)", i)
		}
	}
}
