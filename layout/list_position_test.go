// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/font"
)

// reTdShow matches a `x y Td` followed (after intervening ops) by a `(text) Tj`
// on the next show. It is used to recover the draw x of a piece of text.
var reTd = regexp.MustCompile(`(-?\d+(?:\.\d+)?)\s+(-?\d+(?:\.\d+)?)\s+Td`)
var reTj = regexp.MustCompile(`\((.*?)\) Tj`)

// drawnText is one (x, text) draw recovered from a content stream.
type drawnText struct {
	x    float64
	text string
}

// drawnLines renders the leaf text-line blocks of a list plan, one slice of
// (x, text) pieces per block in document order. Each word is emitted as
// `... x y Td (word) Tj` by drawTextLine, so we pair each Td with the next Tj.
func drawnLines(t *testing.T, plan LayoutPlan) [][]drawnText {
	t.Helper()
	var leaves []PlacedBlock
	var walk func(bs []PlacedBlock)
	walk = func(bs []PlacedBlock) {
		for _, b := range bs {
			if len(b.Children) > 0 {
				walk(b.Children)
				continue
			}
			if b.Draw != nil {
				leaves = append(leaves, b)
			}
		}
	}
	walk(plan.Blocks)

	var out [][]drawnText
	for _, b := range leaves {
		s := content.NewStream()
		ctx := DrawContext{Stream: s, Page: &PageResult{}}
		b.Draw(ctx, b.X, b.Y+b.Height)
		stream := string(s.Bytes())
		var line []drawnText
		tds := reTd.FindAllStringSubmatchIndex(stream, -1)
		for _, td := range tds {
			x, _ := strconv.ParseFloat(stream[td[2]:td[3]], 64)
			rest := stream[td[1]:]
			if m := reTj.FindStringSubmatch(rest); m != nil {
				line = append(line, drawnText{x: x, text: m[1]})
			}
		}
		if len(line) > 0 {
			out = append(out, line)
		}
	}
	return out
}

// TestListInsideVsOutsideGeometry asserts the defining geometric difference
// between list-style-position inside and outside for a wrapping item:
//
//   - outside: the marker sits in the left gutter; the first text word and the
//     wrapped continuation word both start at the same content edge (hanging).
//   - inside: the marker leads the first line inline at the content edge, so the
//     first text word starts to the RIGHT of the marker, while the wrapped
//     continuation word aligns back at the content edge (under the marker).
func TestListInsideVsOutsideGeometry(t *testing.T) {
	const areaW = 120.0
	const indent = 18.0
	// Short words that wrap cleanly (no character-breaking) at the narrow width.
	mk := func(inside bool) [][]drawnText {
		l := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
		l.SetMarkerInside(inside)
		l.AddItem("one two ten six nine four ate")
		plan := l.PlanLayout(LayoutArea{Width: areaW, Height: 1000})
		if plan.Status != LayoutFull {
			t.Fatalf("inside=%v: expected LayoutFull, got %v", inside, plan.Status)
		}
		lines := drawnLines(t, plan)
		if len(lines) < 2 {
			t.Fatalf("inside=%v: expected wrapping to >=2 lines, got %d: %v", inside, len(lines), lines)
		}
		return lines
	}

	outside := mk(false)
	// Outside line 0: marker in the gutter at x=0, first text word at the indent.
	if outside[0][0].text != "1." || outside[0][0].x != 0 {
		t.Errorf("outside: expected marker '1.' at x=0, got %+v", outside[0][0])
	}
	outFirstText := outside[0][1]
	if !approxEqual(outFirstText.x, indent, 0.01) {
		t.Errorf("outside: first text word x = %v, want indent %v", outFirstText.x, indent)
	}
	// Outside hanging indent: continuation line also starts at the indent.
	if !approxEqual(outside[1][0].x, indent, 0.01) {
		t.Errorf("outside: continuation x = %v, want indent %v", outside[1][0].x, indent)
	}

	inside := mk(true)
	// Inside line 0: marker leads inline at the content edge (the indent), and
	// the first text word follows it shifted right by the marker width.
	inMarker := inside[0][0]
	if inMarker.text != "1." || !approxEqual(inMarker.x, indent, 0.01) {
		t.Errorf("inside: expected marker '1.' at content edge %v, got %+v", indent, inMarker)
	}
	inFirstText := inside[0][1]
	if inFirstText.x <= inMarker.x {
		t.Errorf("inside: first text word x (%v) should be right of inline marker (%v)", inFirstText.x, inMarker.x)
	}
	// Inside continuation aligns back under the marker (the content edge).
	if !approxEqual(inside[1][0].x, indent, 0.01) {
		t.Errorf("inside: continuation x = %v, want content edge %v", inside[1][0].x, indent)
	}
	// Defining first-line difference: inside shifts the first text word right of
	// where outside places it (by the marker width).
	if inFirstText.x <= outFirstText.x+1 {
		t.Errorf("inside first text x (%v) should be markerWidth right of outside (%v)", inFirstText.x, outFirstText.x)
	}
}

// TestListGutterAutoSizeWideMarker verifies the outside gutter grows so a
// marker wider than the default 18pt indent does not overlap the item text,
// while a plain short-marker list keeps the 18pt indent (no regression).
func TestListGutterAutoSizeWideMarker(t *testing.T) {
	// Wide custom marker via ::marker content.
	wide := NewList(font.Helvetica, 12)
	wide.AddItem("Body text of the clause.")
	wide.SetLastItemMarker("Section 12: ")

	markerW := wide.markerWidth(0)
	if markerW <= 18 {
		t.Fatalf("test precondition: marker width %v should exceed default 18pt indent", markerW)
	}
	eff := wide.effectiveIndent()
	if eff <= 18 {
		t.Errorf("wide marker: effectiveIndent %v did not grow past 18", eff)
	}

	// Intrinsic widths must include the grown gutter, else a list nested in a
	// table/flex would be under-sized and the marker would overlap the text.
	if mn := wide.MinWidth(); mn < eff {
		t.Errorf("wide marker: MinWidth %v < grown gutter %v", mn, eff)
	}
	if mx := wide.MaxWidth(); mx < eff {
		t.Errorf("wide marker: MaxWidth %v < grown gutter %v", mx, eff)
	}

	plan := wide.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	lines := drawnLines(t, plan)
	if len(lines) == 0 {
		t.Fatal("wide marker: no lines drawn")
	}
	// Line 0 holds the gutter marker (at x=0) then the body text. The body must
	// start at or beyond the indent so it never overlaps the marker.
	first := lines[0]
	markerX := first[0].x
	if markerX != 0 {
		t.Errorf("wide marker: marker should be at gutter x=0, got %v", markerX)
	}
	// The body text is the first piece whose x is at/after the content edge.
	var bodyX float64 = -1
	for _, p := range first {
		if p.x >= eff-0.01 {
			bodyX = p.x
			break
		}
	}
	if bodyX < 0 {
		t.Fatalf("wide marker: no body text at/after content edge %v; line=%+v", eff, first)
	}
	if bodyX < markerX+markerW-0.01 {
		t.Errorf("body text x (%v) overlaps marker [%v, %v]", bodyX, markerX, markerX+markerW)
	}

	// Regression: a plain short-marker list keeps the default 18pt indent.
	short := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
	short.AddItem("Item one")
	if got := short.effectiveIndent(); got != 18 {
		t.Errorf("short-marker list effectiveIndent = %v, want 18 (unchanged)", got)
	}
	sl := drawnLines(t, short.PlanLayout(LayoutArea{Width: 400, Height: 1000}))
	if len(sl) == 0 || len(sl[0]) < 2 {
		t.Fatalf("short-marker: expected marker+text on line 0, got %+v", sl)
	}
	if x := sl[0][1].x; !approxEqual(x, 18, 0.01) {
		t.Errorf("short-marker text x = %v, want 18", x)
	}
}

// TestNestedMarkerIndentsPerLevel verifies that nested-list markers step right
// per nesting level (each level's marker aligns under its parent's content
// edge) rather than all stacking at the container's left margin (issue #358).
// With short markers the gutter stays at the default 18pt, so successive
// levels indent by 18pt: 0, 18, 36.
func TestNestedMarkerIndentsPerLevel(t *testing.T) {
	l := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
	a := l.AddItemWithSubList("Level one")
	a.SetStyle(ListOrdered)
	b := a.AddItemWithSubList("Level two")
	b.SetStyle(ListOrdered)
	b.AddItem("Level three")

	plan := l.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	lines := drawnLines(t, plan)

	// The first token of each list line is its marker; in document order the
	// three "1." markers are level 1, 2, 3.
	var markerX []float64
	for _, ln := range lines {
		if len(ln) > 0 && ln[0].text == "1." {
			markerX = append(markerX, ln[0].x)
		}
	}
	if len(markerX) != 3 {
		t.Fatalf("expected 3 nested '1.' markers, got %d: %+v", len(markerX), markerX)
	}
	want := []float64{0, 18, 36}
	for i, x := range markerX {
		if !approxEqual(x, want[i], 0.01) {
			t.Errorf("level %d marker x = %v, want %v (markers must indent per level)", i+1, x, want[i])
		}
	}
	// Guard against regressing to the old behavior (all markers at x=0).
	if markerX[1] == 0 || markerX[2] == 0 {
		t.Errorf("nested markers stacked at the left margin: %+v", markerX)
	}
}

// TestNestedElementMarkerIndentsPerLevel covers the element-item path
// (planElementItem, used for <li> with block children): its marker must also
// indent per nesting level, matching the text-item path.
func TestNestedElementMarkerIndentsPerLevel(t *testing.T) {
	mkDiv := func(s string) Element {
		d := NewDiv()
		d.Add(NewParagraph(s, font.Helvetica, 12))
		return d
	}
	l := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
	a := l.AddItemElementWithSubList(mkDiv("Level one"))
	a.SetStyle(ListOrdered)
	b := a.AddItemElementWithSubList(mkDiv("Level two"))
	b.SetStyle(ListOrdered)
	b.AddItemElement(mkDiv("Level three"))

	plan := l.PlanLayout(LayoutArea{Width: 400, Height: 1000})
	lines := drawnLines(t, plan)

	var markerX []float64
	for _, ln := range lines {
		if len(ln) == 1 && ln[0].text == "1." { // element marker draws alone
			markerX = append(markerX, ln[0].x)
		}
	}
	if len(markerX) != 3 {
		t.Fatalf("expected 3 element-item '1.' markers, got %d: %+v", len(markerX), markerX)
	}
	for i, want := range []float64{0, 18, 36} {
		if !approxEqual(markerX[i], want, 0.01) {
			t.Errorf("element level %d marker x = %v, want %v", i+1, markerX[i], want)
		}
	}
}

// TestNestedMarkerFlush verifies SetMarkerFlush keeps every nesting level's
// marker in the container's left column (x=0) while body text still cascades,
// the inverse of the per-level indent default (#358 follow-up).
func TestNestedMarkerFlush(t *testing.T) {
	l := NewList(font.Helvetica, 12).SetStyle(ListOrdered)
	l.SetMarkerFlush(true)
	a := l.AddItemWithSubList("Level one")
	a.SetStyle(ListOrdered)
	b := a.AddItemWithSubList("Level two")
	b.SetStyle(ListOrdered)
	b.AddItem("Level three")

	plan := l.PlanLayout(LayoutArea{Width: 400, Height: 1000})

	// Collect every marker token (the leading "1." of each level). In flush
	// mode all three must share the container's left column (x=0), whereas the
	// nested default (TestNestedMarkerIndentsPerLevel) puts them at 0/18/36.
	var markerX []float64
	for _, ln := range drawnLines(t, plan) {
		if len(ln) > 0 && ln[0].text == "1." {
			markerX = append(markerX, ln[0].x)
		}
	}
	if len(markerX) != 3 {
		t.Fatalf("expected 3 '1.' markers, got %d: %+v", len(markerX), markerX)
	}
	for i, x := range markerX {
		if !approxEqual(x, 0, 0.01) {
			t.Errorf("flush: level %d marker x = %v, want 0 (all markers share the left column)", i+1, x)
		}
	}
}
