// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestListUnorderedBasic(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		AddItem("First item").
		AddItem("Second item")

	lines := l.Layout(400)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	// First line of first item should have marker words (bullet)
	if lines[0].listRef == nil {
		t.Fatal("expected listRef on line")
	}
	if len(lines[0].listRef.markerWords) == 0 {
		t.Error("first line should have marker words")
	}
}

func TestListOrderedMarkers(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		SetStyle(ListOrdered).
		AddItem("Alpha").
		AddItem("Beta").
		AddItem("Gamma")

	lines := l.Layout(400)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// Check markers are "1.", "2.", "3."
	markers := []string{"1.", "2.", "3."}
	lineIdx := 0
	for i, expected := range markers {
		if lineIdx >= len(lines) {
			t.Fatalf("ran out of lines at item %d", i)
		}
		ref := lines[lineIdx].listRef
		if ref == nil {
			t.Fatalf("line %d missing listRef", lineIdx)
		}
		if len(ref.markerWords) == 0 {
			t.Errorf("item %d: no marker words", i)
		} else if ref.markerWords[0].Text != expected {
			t.Errorf("item %d: expected marker %q, got %q", i, expected, ref.markerWords[0].Text)
		}
		// Skip to next item's first line
		lineIdx++
		for lineIdx < len(lines) && (lines[lineIdx].listRef == nil || len(lines[lineIdx].listRef.markerWords) > 0) {
			if lines[lineIdx].listRef != nil && len(lines[lineIdx].listRef.markerWords) > 0 {
				break
			}
			lineIdx++
		}
	}
}

func TestListIndent(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		SetIndent(36).
		AddItem("Indented item")

	lines := l.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	if lines[0].listRef.indent != 36 {
		t.Errorf("expected indent 36, got %f", lines[0].listRef.indent)
	}
}

func TestListWordWrap(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		AddItem("This is a longer list item that should wrap to multiple lines when the available width is narrow")

	lines := l.Layout(200)
	if len(lines) < 2 {
		t.Errorf("expected word-wrapped lines, got %d", len(lines))
	}
	// Second line should be indented but have no marker
	if len(lines) >= 2 {
		if lines[1].listRef == nil {
			t.Error("second line should have listRef for indent")
		} else if len(lines[1].listRef.markerWords) > 0 {
			t.Error("second line should not have marker words")
		}
	}
}

func TestListEmpty(t *testing.T) {
	l := NewList(font.Helvetica, 12)
	lines := l.Layout(400)
	if len(lines) != 0 {
		t.Errorf("empty list should produce no lines, got %d", len(lines))
	}
}

func TestListChaining(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		SetStyle(ListOrdered).
		SetIndent(24).
		SetLeading(1.5).
		AddItem("One").
		AddItem("Two")

	lines := l.Layout(400)
	if len(lines) < 2 {
		t.Errorf("expected at least 2 lines, got %d", len(lines))
	}
}

func TestListImplementsElement(t *testing.T) {
	var _ Element = NewList(font.Helvetica, 12)
}

func TestHeadingImplementsElement(t *testing.T) {
	var _ Element = NewHeading("Title", H1)
}

// --- Nested lists ---

func TestNestedListBasic(t *testing.T) {
	l := NewList(font.Helvetica, 12)
	sub := l.AddItemWithSubList("Parent item")
	sub.AddItem("Child A")
	sub.AddItem("Child B")

	lines := l.Layout(400)
	// 1 parent line + 2 child lines = 3
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// Parent line indent should be 18 (default)
	if lines[0].listRef.indent != 18 {
		t.Errorf("parent indent: expected 18, got %f", lines[0].listRef.indent)
	}

	// Child lines indent should be 36 (18 + 18)
	if lines[1].listRef.indent != 36 {
		t.Errorf("child indent: expected 36, got %f", lines[1].listRef.indent)
	}
}

func TestNestedListThreeLevels(t *testing.T) {
	l := NewList(font.Helvetica, 10)
	sub := l.AddItemWithSubList("Level 1")
	subsub := sub.AddItemWithSubList("Level 2")
	subsub.AddItem("Level 3")

	lines := l.Layout(400)
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d", len(lines))
	}

	// Level 1: indent 18, Level 2: indent 36, Level 3: indent 54
	if lines[0].listRef.indent != 18 {
		t.Errorf("level 1 indent: expected 18, got %f", lines[0].listRef.indent)
	}
	if lines[1].listRef.indent != 36 {
		t.Errorf("level 2 indent: expected 36, got %f", lines[1].listRef.indent)
	}
	if lines[2].listRef.indent != 54 {
		t.Errorf("level 3 indent: expected 54, got %f", lines[2].listRef.indent)
	}
}

func TestNestedListOrdered(t *testing.T) {
	l := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
	sub := l.AddItemWithSubList("First")
	sub.SetStyle(ListOrdered)
	sub.AddItem("Sub-first")
	sub.AddItem("Sub-second")
	l.AddItem("Second")

	lines := l.Layout(400)
	// "First" + "Sub-first" + "Sub-second" + "Second" = 4 lines
	if len(lines) < 4 {
		t.Fatalf("expected at least 4 lines, got %d", len(lines))
	}

	// Parent markers: "1." and "2." (numbering restarts in sub-list)
	if lines[0].listRef.markerWords[0].Text != "1." {
		t.Errorf("expected '1.' marker, got %q", lines[0].listRef.markerWords[0].Text)
	}
	// Sub-list starts at 1.
	if lines[1].listRef.markerWords[0].Text != "1." {
		t.Errorf("expected sub '1.' marker, got %q", lines[1].listRef.markerWords[0].Text)
	}
	// Last item is parent's second
	lastLine := lines[len(lines)-1]
	if lastLine.listRef.markerWords[0].Text != "2." {
		t.Errorf("expected '2.' marker, got %q", lastLine.listRef.markerWords[0].Text)
	}
}

func TestNestedListInheritsFont(t *testing.T) {
	l := NewList(font.HelveticaBold, 14)
	sub := l.AddItemWithSubList("Parent")
	sub.AddItem("Child")

	if sub.font != font.HelveticaBold {
		t.Error("sub-list should inherit parent font")
	}
	if sub.fontSize != 14 {
		t.Error("sub-list should inherit parent font size")
	}
}

func TestNestedListEmptySubList(t *testing.T) {
	l := NewList(font.Helvetica, 12)
	l.AddItemWithSubList("Parent with empty sub-list")
	l.AddItem("Next item")

	lines := l.Layout(400)
	// Should have 2 lines: parent + next item. Empty sub-list adds nothing.
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
}

func TestNestedListWordWrap(t *testing.T) {
	l := NewList(font.Helvetica, 12)
	sub := l.AddItemWithSubList("Parent")
	sub.AddItem("This is a very long nested list item that should wrap to multiple lines in a narrow column width")

	lines := l.Layout(200)
	// Parent + multiple wrapped child lines
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines (parent + wrapped child), got %d", len(lines))
	}
	// All child lines should have the nested indent (36 = 18+18)
	for i := 1; i < len(lines); i++ {
		if lines[i].listRef == nil {
			t.Fatalf("line %d missing listRef", i)
		}
		if lines[i].listRef.indent != 36 {
			t.Errorf("line %d indent: expected 36, got %f", i, lines[i].listRef.indent)
		}
	}
}

func TestNestedListDeep(t *testing.T) {
	l := NewList(font.Helvetica, 10)
	cur := l
	for range 5 {
		cur = cur.AddItemWithSubList("Level")
	}
	cur.AddItem("Leaf")

	lines := l.Layout(400)
	// 5 "Level" items + 1 "Leaf" = 6 lines
	if len(lines) < 6 {
		t.Fatalf("expected at least 6 lines, got %d", len(lines))
	}
	// Last line (deepest) should have indent = 6 * 18 = 108
	last := lines[len(lines)-1]
	if last.listRef.indent != 108 {
		t.Errorf("deepest indent: expected 108, got %f", last.listRef.indent)
	}
}

func TestListBulletCharacter(t *testing.T) {
	l := NewList(font.Helvetica, 12).
		AddItem("Item")

	lines := l.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
	ref := lines[0].listRef
	if ref == nil {
		t.Fatal("expected listRef on first line")
	}
	if len(ref.markerWords) == 0 {
		t.Fatal("expected marker words on first line")
	}
	bullet := ref.markerWords[0].Text
	if bullet != "\u2022" {
		t.Errorf("expected bullet character U+2022 (%q), got %q", "\u2022", bullet)
	}
}
