// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestParagraphTextAlignLastCenter(t *testing.T) {
	// Create a justified paragraph with text-align-last: center.
	p := NewParagraph(
		"This is a long paragraph with enough text to wrap to multiple lines "+
			"so we can verify that the last line has a different alignment from the others.",
		font.Helvetica, 12,
	)
	p.SetAlign(AlignJustify)
	p.SetTextAlignLast(AlignCenter)

	lines := p.Layout(150)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}

	// Non-last lines should be justify.
	for i := 0; i < len(lines)-1; i++ {
		if lines[i].Align != AlignJustify {
			t.Errorf("line %d: expected AlignJustify, got %v", i, lines[i].Align)
		}
	}
	// Last line should be center.
	last := lines[len(lines)-1]
	if last.Align != AlignCenter {
		t.Errorf("last line: expected AlignCenter, got %v", last.Align)
	}
}

func TestParagraphTextAlignLastRight(t *testing.T) {
	p := NewParagraph(
		"Another paragraph with enough text to wrap to multiple lines for testing right alignment on the last line.",
		font.Helvetica, 12,
	)
	p.SetAlign(AlignJustify)
	p.SetTextAlignLast(AlignRight)

	lines := p.Layout(150)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	last := lines[len(lines)-1]
	if last.Align != AlignRight {
		t.Errorf("last line: expected AlignRight, got %v", last.Align)
	}
}

func TestParagraphTextAlignLastJustify(t *testing.T) {
	// All lines should be justified when text-align-last: justify.
	p := NewParagraph(
		"Yet another paragraph that needs to be long enough to wrap across multiple lines for testing purposes.",
		font.Helvetica, 12,
	)
	p.SetAlign(AlignJustify)
	p.SetTextAlignLast(AlignJustify)

	lines := p.Layout(150)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	for i, line := range lines {
		if line.Align != AlignJustify {
			t.Errorf("line %d: expected AlignJustify, got %v", i, line.Align)
		}
	}
}

func TestParagraphTextAlignLastNotSet(t *testing.T) {
	// Without SetTextAlignLast, all lines use the paragraph alignment.
	p := NewParagraph(
		"A paragraph without text-align-last set, all lines should use the same alignment value.",
		font.Helvetica, 12,
	)
	p.SetAlign(AlignJustify)

	lines := p.Layout(150)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
	// All lines including last should be justify.
	for i, line := range lines {
		if line.Align != AlignJustify {
			t.Errorf("line %d: expected AlignJustify, got %v", i, line.Align)
		}
	}
}

func TestParagraphTextAlignLastSingleLine(t *testing.T) {
	// Single-line paragraph: text-align-last should still apply.
	p := NewParagraph("Short text", font.Helvetica, 12)
	p.SetAlign(AlignLeft)
	p.SetTextAlignLast(AlignCenter)

	lines := p.Layout(500)
	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if lines[0].Align != AlignCenter {
		t.Errorf("single line: expected AlignCenter, got %v", lines[0].Align)
	}
}
