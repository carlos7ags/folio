package html

import (
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// TestIssue130_Gap3_InlineSVGInParagraph asserts that an inline <svg> placed
// between words inside a <p> participates in paragraph layout. Regression
// test for issue #130 gap 3: previously the <svg> inherited Display="block"
// from its parent <p>, and collectRuns silently dropped block-level children
// from inline flow.
func TestIssue130_Gap3_InlineSVGInParagraph(t *testing.T) {
	src := `<p>This paragraph contains text before the SVG element, then an inline SVG icon <svg width="12" height="12" viewBox="0 0 12 12"><circle cx="6" cy="6" r="5" fill="red"/></svg> and then text continues after it.</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}

	lines := p.Layout(500)
	foundInline := false
	for _, line := range lines {
		for _, w := range line.Words {
			if w.InlineBlock != nil {
				foundInline = true
			}
		}
	}
	if !foundInline {
		t.Errorf("no inline element word found in paragraph; inline <svg> was dropped")
	}
}

// TestIssue130_Gap3_InlineImgInParagraph covers the same bug for <img>
// since the fix applies to both replaced elements.
func TestIssue130_Gap3_InlineImgInParagraph(t *testing.T) {
	// 1×1 transparent PNG.
	src := `<p>before <img src="data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==" width="8" height="8"> after</p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *Paragraph, got %T", elems[0])
	}
	lines := p.Layout(500)
	foundInline := false
	for _, line := range lines {
		for _, w := range line.Words {
			if w.InlineBlock != nil {
				foundInline = true
			}
		}
	}
	if !foundInline {
		t.Errorf("no inline element word found in paragraph; inline <img> was dropped")
	}
}
