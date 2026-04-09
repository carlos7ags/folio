// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

// TestParagraphRTLAutoDetect verifies that a paragraph containing only
// Hebrew text auto-detects RTL and right-aligns by default.
func TestParagraphRTLAutoDetect(t *testing.T) {
	p := NewParagraph("\u05E9\u05DC\u05D5\u05DD \u05E2\u05D5\u05DC\u05DD", font.Helvetica, 12) // "שלום עולם"
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 200})
	if plan.Status != LayoutFull {
		t.Fatalf("status: %v", plan.Status)
	}
	if len(plan.Blocks) == 0 {
		t.Fatal("no blocks")
	}
	// With right-alignment on a 500pt-wide area, the block's X should be
	// positive (shifted to the right). A left-aligned block would have X=0.
	if plan.Blocks[0].X <= 0 {
		t.Errorf("Hebrew paragraph should auto-right-align; block X=%v (expected >0)", plan.Blocks[0].X)
	}
}

// TestParagraphRTLWordOrder verifies that Hebrew words appear in visual
// order (reversed from logical) on the line.
func TestParagraphRTLWordOrder(t *testing.T) {
	// "שלום עולם" → visual order left-to-right: עולם שלום
	p := NewParagraph("\u05E9\u05DC\u05D5\u05DD \u05E2\u05D5\u05DC\u05DD", font.Helvetica, 12)
	lines := p.Layout(500)
	if len(lines) == 0 {
		t.Fatal("no lines")
	}
	words := lines[0].Words
	if len(words) < 2 {
		t.Fatalf("expected 2 words, got %d", len(words))
	}
	// Visual first word should be the second logical word (עולם).
	if words[0].Text != "\u05E2\u05D5\u05DC\u05DD" {
		t.Errorf("first visual word: got %q, want %q", words[0].Text, "\u05E2\u05D5\u05DC\u05DD")
	}
	if words[1].Text != "\u05E9\u05DC\u05D5\u05DD" {
		t.Errorf("second visual word: got %q, want %q", words[1].Text, "\u05E9\u05DC\u05D5\u05DD")
	}
}

// TestParagraphExplicitAlignOverridesRTL verifies that SetAlign(AlignLeft)
// on a Hebrew paragraph keeps left-alignment even though the resolved
// direction is RTL.
func TestParagraphExplicitAlignOverridesRTL(t *testing.T) {
	p := NewParagraph("\u05E9\u05DC\u05D5\u05DD", font.Helvetica, 12)
	p.SetAlign(AlignLeft)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 200})
	if plan.Status != LayoutFull || len(plan.Blocks) == 0 {
		t.Fatal("layout failed")
	}
	if plan.Blocks[0].X != 0 {
		t.Errorf("explicit AlignLeft should keep X=0; got %v", plan.Blocks[0].X)
	}
}

// TestParagraphExplicitDirectionRTL_PunctOnly verifies that
// SetDirection(DirectionRTL) on text with no strong or weak directional
// characters (only punctuation) uses the RTL fallback and right-aligns.
// Note: numbers (EN) are weak-LTR and cause the bidi library to resolve
// LTR even with an RTL default, so we use punctuation-only text here.
func TestParagraphExplicitDirectionRTL_PunctOnly(t *testing.T) {
	p := NewParagraph("...", font.Helvetica, 12)
	p.SetDirection(DirectionRTL)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 200})
	if plan.Status != LayoutFull || len(plan.Blocks) == 0 {
		t.Fatal("layout failed")
	}
	if plan.Blocks[0].X <= 0 {
		t.Errorf("DirectionRTL hint on punctuation-only text should right-align; X=%v", plan.Blocks[0].X)
	}
}

// TestParagraphLTRUnchanged is a regression guard: plain English text
// with no direction set behaves exactly as before (left-aligned, words
// in logical order).
func TestParagraphLTRUnchanged(t *testing.T) {
	p := NewParagraph("Hello world", font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 200})
	if plan.Status != LayoutFull || len(plan.Blocks) == 0 {
		t.Fatal("layout failed")
	}
	if plan.Blocks[0].X != 0 {
		t.Errorf("English paragraph should be left-aligned (X=0); got %v", plan.Blocks[0].X)
	}
	lines := p.Layout(500)
	if len(lines) == 0 || len(lines[0].Words) < 2 {
		t.Fatal("expected 2 words")
	}
	if lines[0].Words[0].Text != "Hello" || lines[0].Words[1].Text != "world" {
		t.Errorf("English word order should be unchanged: got [%q, %q]",
			lines[0].Words[0].Text, lines[0].Words[1].Text)
	}
}

// TestParagraphMixedBidiInRTL verifies mixed Hebrew+English in an RTL
// paragraph: the English word stays LTR, Hebrew words reverse.
func TestParagraphMixedBidiInRTL(t *testing.T) {
	// "שלום Hello עולם" → visual: עולם Hello שלום
	text := "\u05E9\u05DC\u05D5\u05DD Hello \u05E2\u05D5\u05DC\u05DD"
	p := NewParagraph(text, font.Helvetica, 12)
	lines := p.Layout(500)
	if len(lines) == 0 {
		t.Fatal("no lines")
	}
	words := lines[0].Words
	if len(words) < 3 {
		t.Fatalf("expected 3 words, got %d", len(words))
	}
	if words[0].Text != "\u05E2\u05D5\u05DC\u05DD" {
		t.Errorf("first visual word: got %q, want עולם", words[0].Text)
	}
	if words[1].Text != "Hello" {
		t.Errorf("middle word: got %q, want Hello", words[1].Text)
	}
	if words[2].Text != "\u05E9\u05DC\u05D5\u05DD" {
		t.Errorf("last visual word: got %q, want שלום", words[2].Text)
	}
}

// TestParagraphRTLBracketsAreMirrored verifies that parentheses in an
// RTL paragraph are mirrored per UAX #9 rule L4.
func TestParagraphRTLBracketsAreMirrored(t *testing.T) {
	// "(שלום)" → visual: ")שלום(" — mirrored brackets.
	text := "(\u05E9\u05DC\u05D5\u05DD)"
	p := NewParagraph(text, font.Helvetica, 12)
	lines := p.Layout(500)
	if len(lines) == 0 || len(lines[0].Words) == 0 {
		t.Fatal("no words")
	}
	w := lines[0].Words[0].Text
	if len(w) == 0 || w[0] != ')' {
		t.Errorf("opening '(' should mirror to ')' in RTL: got %q", w)
	}
}

// TestParagraphDirectionPreservedOnOverflow verifies that direction and
// alignSet survive a page-break split (cloneWithWords).
func TestParagraphDirectionPreservedOnOverflow(t *testing.T) {
	// Build a long paragraph that forces overflow. Each Hebrew word
	// plus space is ~24pt wide in Helvetica 12, so 20 words in a
	// 200pt-wide area at 14.4pt line height with only 15pt available
	// height forces a split after the first line.
	var parts []string
	for i := 0; i < 20; i++ {
		parts = append(parts, "\u05E9\u05DC\u05D5\u05DD")
	}
	long := ""
	for i, p := range parts {
		if i > 0 {
			long += " "
		}
		long += p
	}
	p := NewParagraph(long, font.Helvetica, 12)
	p.SetDirection(DirectionRTL)

	plan := p.PlanLayout(LayoutArea{Width: 200, Height: 15})
	if plan.Status != LayoutPartial {
		t.Fatalf("expected LayoutPartial, got %v (need more words or tighter area)", plan.Status)
	}
	overflow, ok := plan.Overflow.(*Paragraph)
	if !ok {
		t.Fatalf("overflow is %T, want *Paragraph", plan.Overflow)
	}
	if overflow.direction != DirectionRTL {
		t.Errorf("overflow direction: got %v, want RTL", overflow.direction)
	}
	if overflow.alignSet != p.alignSet {
		t.Errorf("overflow alignSet: got %v, want %v", overflow.alignSet, p.alignSet)
	}
}

// TestParagraphEmptyRTLNoPanic verifies that an empty paragraph with
// SetDirection(RTL) does not panic and produces a valid layout.
func TestParagraphEmptyRTLNoPanic(t *testing.T) {
	p := NewParagraph("", font.Helvetica, 12)
	p.SetDirection(DirectionRTL)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 200})
	if plan.Status != LayoutFull {
		t.Errorf("empty RTL paragraph: status=%v, want LayoutFull", plan.Status)
	}
}

// TestParagraphWhitespaceOnlyRTLAlignment verifies that a whitespace-only
// paragraph with explicit RTL direction still right-aligns (the hasContent
// guard should respect the base direction rather than falling back to LTR).
func TestParagraphWhitespaceOnlyRTLAlignment(t *testing.T) {
	p := NewParagraph("   ", font.Helvetica, 12)
	p.SetDirection(DirectionRTL)
	plan := p.PlanLayout(LayoutArea{Width: 500, Height: 200})
	if plan.Status != LayoutFull || len(plan.Blocks) == 0 {
		t.Skipf("whitespace-only paragraph produced no blocks")
	}
	// With RTL direction, alignment should be right → X > 0.
	if plan.Blocks[0].X <= 0 {
		t.Errorf("whitespace-only RTL should right-align; X=%v", plan.Blocks[0].X)
	}
}
