// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"fmt"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// bareFloatHTML is the issue #372 markup: a floated block with no box-model
// properties (no width, background, border, padding, or margin) holding more
// than one child, followed by in-flow text that wraps beside it and a second
// paragraph that lands below the float's bottom edge.
const bareFloatHTML = `<div style="float:left"><h2>Heading</h2><p>Paragraph</p></div>
<p>Beside text that flows around the floated box and keeps going for long
enough that it wraps onto several lines, more lines than the floated box is
tall, so that the final lines of this paragraph fall below the bottom edge of
the float and return to the left margin of the page content area. Some more
words are added here to make certain that this happens on every platform and
with every font metric variation that the renderer might reasonably apply.
The paragraph continues with a few further sentences of filler so that the
wrapped lines comfortably outrun the floated heading and paragraph, leaving
no doubt that the remaining text is laid out at the full content width once
the float has ended.</p>
<p>Afterward</p>`

// TestBareFloatedBlockIsOneFloat is the regression test for issue #372: a
// floated element whose block has no box-model properties used to be returned
// as its flat children, and each child was wrapped in its own Float. The
// element must float as a single box.
func TestBareFloatedBlockIsOneFloat(t *testing.T) {
	for _, side := range []string{"left", "right"} {
		t.Run(side, func(t *testing.T) {
			htmlStr := fmt.Sprintf(`<div style="float:%s"><h2>Heading</h2><p>Paragraph</p></div><p>After</p>`, side)
			elems, err := Convert(htmlStr, nil)
			if err != nil {
				t.Fatal(err)
			}
			floats := 0
			for _, e := range elems {
				if _, ok := e.(*layout.Float); ok {
					floats++
				}
			}
			if floats != 1 {
				var types []string
				for _, e := range elems {
					types = append(types, fmt.Sprintf("%T", e))
				}
				t.Fatalf("got %d floats, want exactly 1; elements: %v", floats, types)
			}
		})
	}
}

// TestBareAbsoluteBlockIsOneBox covers the same flattening for
// position:absolute: each child of a bare absolutely positioned block used to
// become its own absolute item at the same offset, drawing them on top of
// each other instead of stacking them inside one box.
func TestBareAbsoluteBlockIsOneBox(t *testing.T) {
	res, err := ConvertFull(`<div style="position:absolute;top:10px;left:10px"><h2>Heading</h2><p>Paragraph</p></div>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Absolutes) != 1 {
		t.Fatalf("got %d absolute items, want exactly 1", len(res.Absolutes))
	}
}

// textRun is a positioned word from a page content stream.
type textRun struct {
	x, y float64
	text string
}

// textRuns extracts each Td-positioned text-show operation from a content
// stream, joining TJ array fragments into a single string.
func textRuns(stream string) []textRun {
	var runs []textRun
	var x, y float64
	for _, line := range strings.Split(stream, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasSuffix(line, " Td"):
			_, _ = fmt.Sscanf(line, "%g %g Td", &x, &y)
		case strings.HasSuffix(line, " Tj"), strings.HasSuffix(line, " TJ"):
			var b strings.Builder
			in := false
			for _, r := range line {
				switch {
				case r == '(':
					in = true
				case r == ')':
					in = false
				case in:
					b.WriteRune(r)
				}
			}
			runs = append(runs, textRun{x: x, y: y, text: b.String()})
		}
	}
	return runs
}

// TestBareFloatedBlockRendersAsOneBox checks the rendered geometry for issue
// #372: the heading and paragraph stack vertically inside the single floated
// box, the in-flow text beside it starts to the right of that box (which is
// shrink-to-fit, not full width), and content below the float returns to the
// left margin.
func TestBareFloatedBlockRendersAsOneBox(t *testing.T) {
	_, pages := biRender(t, bareFloatHTML)
	if len(pages) == 0 {
		t.Fatal("no pages rendered")
	}
	runs := textRuns(string(pages[0].Stream.Bytes()))

	find := func(text string) textRun {
		t.Helper()
		for _, r := range runs {
			if r.text == text {
				return r
			}
		}
		t.Fatalf("text %q not found in content stream", text)
		return textRun{}
	}
	heading := find("Heading")
	para := find("Paragraph")
	beside := find("Beside")

	const tol = 0.5
	if d := para.x - heading.x; d < -tol || d > tol {
		t.Errorf("paragraph x = %.2f, heading x = %.2f: want both at the float's left edge (stacked)", para.x, heading.x)
	}
	if para.y >= heading.y {
		t.Errorf("paragraph y = %.2f, heading y = %.2f: want paragraph below heading", para.y, heading.y)
	}
	if beside.x <= heading.x+tol {
		t.Errorf("beside text x = %.2f: want it right of the float (float left edge %.2f)", beside.x, heading.x)
	}
	// The float is shrink-to-fit: the wrapped text must start well short of
	// the content box's right half, not below a full-width float.
	if beside.x > biPageW/2 {
		t.Errorf("beside text x = %.2f: float is not shrink-to-fit (expected it to hug its content)", beside.x)
	}
	if beside.y < para.y {
		t.Errorf("beside text y = %.2f: want it to start beside the float, not below it (paragraph y %.2f)", beside.y, para.y)
	}

	// A block that starts below the float's bottom edge is no longer
	// narrowed by it and sits at the left margin.
	after := find("Afterward")
	if after.y >= para.y {
		t.Errorf("following paragraph y = %.2f: want it below the float (paragraph y %.2f)", after.y, para.y)
	}
	if d := after.x - heading.x; d < -tol || d > tol {
		t.Errorf("following paragraph x = %.2f: want it at the left margin %.2f", after.x, heading.x)
	}
}
