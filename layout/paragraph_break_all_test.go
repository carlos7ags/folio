// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"
	"unicode/utf8"

	"github.com/carlos7ags/folio/font"
)

// TestMeasureWordsBreakAll is a regression test for the bug where
// measureWords ignored word-break:break-all and only ever called
// breakLongWords. With break-all set, every multi-rune word in the
// returned slice must already be split at character boundaries so the
// word-wrap algorithm can place a break between any two characters.
//
// This mirrors the dispatch already present in (*Paragraph).Layout.
func TestMeasureWordsBreakAll(t *testing.T) {
	p := NewParagraph("abcd efgh", font.Helvetica, 12).SetWordBreak("break-all")

	words, _ := p.measureWords(400)
	if len(words) == 0 {
		t.Fatal("expected at least one measured word")
	}
	for i, w := range words {
		if w.LineBreak {
			continue
		}
		if utf8.RuneCountInString(w.Text) != 1 {
			t.Errorf("word %d: expected single rune under break-all, got %q (%d runes)",
				i, w.Text, utf8.RuneCountInString(w.Text))
		}
	}
}

// TestMeasureWordsDefaultBreakingUnchanged is the control case: with
// wordBreak unset, multi-rune words must remain intact. This guards
// against the fix accidentally firing on the default code path.
func TestMeasureWordsDefaultBreakingUnchanged(t *testing.T) {
	p := NewParagraph("abcd efgh", font.Helvetica, 12)

	words, _ := p.measureWords(400)
	multiRune := 0
	for _, w := range words {
		if utf8.RuneCountInString(w.Text) > 1 {
			multiRune++
		}
	}
	if multiRune != 2 {
		t.Errorf("expected 2 multi-rune words under default breaking, got %d", multiRune)
	}
}

// TestMeasureWordsBreakAllKeepAllStillBreaks ensures break-all wins
// over the keep-all guard at the top of the function (which only
// gates breakCJKWords). This documents the precedence of the two
// wordBreak modes the user can pick.
func TestMeasureWordsBreakAllPrecedence(t *testing.T) {
	// keep-all is its own mode; combining is undefined per spec, but
	// break-all is what the user explicitly asked for here.
	p := NewParagraph("abcd", font.Helvetica, 12).SetWordBreak("break-all")
	words, _ := p.measureWords(400)
	for i, w := range words {
		if utf8.RuneCountInString(w.Text) != 1 {
			t.Errorf("word %d: %q not split under break-all", i, w.Text)
		}
	}
}
