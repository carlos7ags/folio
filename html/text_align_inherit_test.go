// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html_test

import (
	"math"
	"sort"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/reader"
)

// textLine is one rendered line of text, reassembled from the individual
// positioned spans a content stream emits. Kerning splits a single word
// across several Tj operators, so no one span carries the line's real
// left edge — only the minimum X over the whole line does.
type textLine struct {
	text string
	x    float64 // leftmost pen position on the line
	y    float64
}

// renderedLines groups a page's text spans into lines by baseline and
// returns them in reading order (top of the page first). Assertions run
// against the geometry the viewer actually paints rather than against the
// computed style, so a style that is right but never reaches the
// paragraph cannot pass.
func renderedLines(t *testing.T, r *reader.PdfReader) []textLine {
	t.Helper()
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	spans, err := page.TextSpans()
	if err != nil {
		t.Fatalf("TextSpans: %v", err)
	}

	// Bucket spans by baseline. Exact equality is too strict once a line
	// carries mixed font sizes, so bucket within half a point.
	const sameLine = 0.5
	var lines []textLine
	for _, s := range spans {
		if strings.TrimSpace(s.Text) == "" {
			continue
		}
		placed := false
		for i := range lines {
			if math.Abs(lines[i].y-s.Y) <= sameLine {
				lines[i].text += s.Text
				lines[i].x = math.Min(lines[i].x, s.X)
				placed = true
				break
			}
		}
		if !placed {
			lines = append(lines, textLine{text: s.Text, x: s.X, y: s.Y})
		}
	}
	sort.Slice(lines, func(i, j int) bool { return lines[i].y > lines[j].y })
	return lines
}

// TestTextAlignInheritsToChildBlocks locks in that `text-align` declared on
// a container reaches the block children that inherit it.
//
// text-align is an inherited property, so every browser right-aligns all
// three children of the div below. Folio right-aligned only the heading:
// computedStyle.inherit() copied the alignment VALUE to the child but not
// the TextAlignSet flag that converter_paragraph.go guards on, so each
// paragraph held the right value behind a false flag and kept the default
// left alignment.
//
// The three paragraphs carry identical text so their geometry is directly
// comparable: an inheriting paragraph must land exactly where the same
// paragraph lands when it declares the alignment itself.
func TestTextAlignInheritsToChildBlocks(t *testing.T) {
	const sample = "Right aligned sample text"
	src := `<html><body>
<div style="text-align: right">
  <h1>Heading</h1>
  <p>` + sample + `</p>
  <p>` + sample + `</p>
</div>
<p style="text-align: right">` + sample + `</p>
<p>` + sample + `</p>
</body></html>`

	_, r := htmlRoundtrip(t, src, document.PageSizeA4)
	lines := renderedLines(t, r)
	if len(lines) != 5 {
		for i, l := range lines {
			t.Logf("line %d: x=%.2f y=%.2f %q", i, l.x, l.y, l.text)
		}
		t.Fatalf("got %d rendered lines, want 5", len(lines))
	}

	heading := lines[0]
	inherited1, inherited2 := lines[1], lines[2]
	declared := lines[3] // passing control: declares text-align itself
	undeclared := lines[4]

	// Tolerance covers only sub-point rounding in the emitted pen
	// positions; identical text under identical alignment must land in
	// the same place.
	const tol = 0.5

	// The heading already worked before the fix — keep it asserted so a
	// regression there is caught too.
	if heading.x < 300 {
		t.Errorf("heading x = %.2f, want right-aligned (well right of the left margin)", heading.x)
	}

	// The control proves the alignment is reachable at all: if this fails,
	// the test is measuring something other than the inheritance bug.
	if declared.x < 300 {
		t.Fatalf("control paragraph with its own text-align: right has x = %.2f, want right-aligned", declared.x)
	}

	for i, got := range []textLine{inherited1, inherited2} {
		if math.Abs(got.x-declared.x) > tol {
			t.Errorf("inheriting paragraph %d: x = %.2f, want %.2f (same as the paragraph that declares text-align: right); text-align was not inherited",
				i+1, got.x, declared.x)
		}
	}

	// An element whose ancestors never declared text-align must keep the
	// default left alignment: inheriting the flag must not turn "default"
	// into "author declared it".
	if math.Abs(undeclared.x-declared.x) <= tol {
		t.Errorf("paragraph outside the aligned container: x = %.2f, want the default left alignment, not %.2f", undeclared.x, declared.x)
	}
	if undeclared.x > 300 {
		t.Errorf("paragraph outside the aligned container: x = %.2f, want left-aligned", undeclared.x)
	}
}

// TestTextAlignInheritDoesNotOverrideUADefaults guards the boundary the
// inheritance fix has to respect: <caption> and <th> default to centered,
// and a UA default outranks an alignment that was merely INHERITED — only a
// declaration on the element itself replaces it.
//
// Propagating TextAlignSet through inherit() made every descendant of a
// `text-align: left` container look like it had declared left alignment,
// which silently left-aligned captions that must stay centered. The self-only
// TextAlignSelfSet flag keeps the two cases apart; this test fails if they are
// ever collapsed back together.
func TestTextAlignInheritDoesNotOverrideUADefaults(t *testing.T) {
	src := `<html><body>
<div style="text-align: left">
<table><caption>Inherited</caption><tr><td>Cell</td></tr></table>
</div>
<table><caption>Baseline</caption><tr><td>Cell</td></tr></table>
<table><caption style="text-align: left">Declared</caption><tr><td>Cell</td></tr></table>
</body></html>`

	_, r := htmlRoundtrip(t, src, document.PageSizeA4)
	byText := map[string]textLine{}
	for _, l := range renderedLines(t, r) {
		byText[l.text] = l
	}

	baseline, ok := byText["Baseline"]
	if !ok {
		t.Fatalf("caption %q not rendered; got lines %v", "Baseline", byText)
	}
	if baseline.x < 200 {
		t.Fatalf("caption outside any aligned container has x = %.2f, want centered; the control itself is broken", baseline.x)
	}

	// A caption whose ancestor declared text-align must still centre.
	inherited, ok := byText["Inherited"]
	if !ok {
		t.Fatalf("caption %q not rendered", "Inherited")
	}
	if math.Abs(inherited.x-baseline.x) > 0.5 {
		t.Errorf("caption inside a text-align: left container: x = %.2f, want %.2f (still centered) — an inherited alignment must not displace the UA default",
			inherited.x, baseline.x)
	}

	// A caption that declares the alignment itself does override the default.
	declared, ok := byText["Declared"]
	if !ok {
		t.Fatalf("caption %q not rendered", "Declared")
	}
	if declared.x > 200 {
		t.Errorf("caption with its own text-align: left: x = %.2f, want left-aligned", declared.x)
	}
}
