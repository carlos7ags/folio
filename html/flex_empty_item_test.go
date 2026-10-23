// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html_test

import (
	"math"
	"testing"

	"github.com/carlos7ags/folio/document"
)

// flexBoxTextLine renders src and returns the rendered line holding the
// box label "QR", failing the test when it is absent.
func flexBoxTextLine(t *testing.T, src string) textLine {
	t.Helper()
	_, r := htmlRoundtrip(t, src, document.PageSizeLetter)
	for _, l := range renderedLines(t, r) {
		if l.text == "QR" {
			return l
		}
	}
	t.Fatalf("no rendered QR line in output")
	return textLine{}
}

// flexEmptyItemDoc builds a flex container whose first child is an empty
// element (or whatever `first` holds) followed by a 96pt box labeled QR.
func flexEmptyItemDoc(containerCSS, first string) string {
	return `<html><head><style>` +
		`*{margin:0;padding:0}` +
		`.g{display:flex;` + containerCSS + `}` +
		`.q{width:96pt;height:96pt;border:1px solid red}` +
		`</style></head><body>` +
		`<div class="g">` + first + `<div class="q">QR</div></div>` +
		`</body></html>`
}

// TestFlexEmptyItemParticipatesInJustifyContent locks in that an empty
// element child of a flex container is still a flex item (CSS Flexbox
// §4). With `justify-content: space-between`, an empty first item and a
// sized second item, every browser pushes the second item to the end of
// the line — a common "right-align via an empty spacer" pattern (issue
// #447). Folio dropped the empty <div> because it converted to no layout
// element, leaving a single item that space-between puts at the start.
//
// Positions are compared against the same box placed with flex-start and
// flex-end, so the assertions do not depend on page margins or the exact
// glyph offset inside the box.
func TestFlexEmptyItemParticipatesInJustifyContent(t *testing.T) {
	const tol = 0.5
	start := flexBoxTextLine(t, flexEmptyItemDoc("justify-content:flex-start", "")).x
	end := flexBoxTextLine(t, flexEmptyItemDoc("justify-content:flex-end", "")).x
	free := end - start
	if free < 100 {
		t.Fatalf("flex-end moved box only %.2fpt from flex-start; want most of the line", free)
	}

	cases := []struct {
		name  string
		css   string
		first string
		want  float64 // expected box offset from the flex-start position
	}{
		{"space-between empty div", "justify-content:space-between", `<div></div>`, free},
		{"space-between empty div with class", "justify-content:space-between", `<div class=""></div>`, free},
		{"space-between empty span", "justify-content:space-between", `<span></span>`, free},
		{"space-between whitespace around empty div", "justify-content:space-between", "\n  <div></div>\n  ", free},
		// Two items (0 and 96 wide) share `free` of leftover space.
		// space-around: free/4 on each side of every item → box at 3/4.
		{"space-around empty div", "justify-content:space-around", `<div></div>`, free * 3 / 4},
		// space-evenly: three equal gaps of free/3 → box at 2/3.
		{"space-evenly empty div", "justify-content:space-evenly", `<div></div>`, free * 2 / 3},
		// Children that generate no box are not flex items: the box is
		// the only item and space-between keeps it at the start.
		{"space-between display none", "justify-content:space-between", `<div style="display:none"></div>`, 0},
		{"space-between absolute", "justify-content:space-between", `<div style="position:absolute"></div>`, 0},
		{"space-between whitespace only", "justify-content:space-between", "\n   \n", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := flexBoxTextLine(t, flexEmptyItemDoc(tc.css, tc.first)).x - start
			if math.Abs(got-tc.want) > tol {
				t.Errorf("box offset from flex-start = %.2fpt, want %.2fpt (free space %.2fpt)", got, tc.want, free)
			}
		})
	}
}

// TestFlexEmptyItemColumnSpaceBetween checks the column axis: in a
// fixed-height column container, an empty first item with space-between
// pushes the box to the bottom.
func TestFlexEmptyItemColumnSpaceBetween(t *testing.T) {
	const tol = 0.5
	col := func(justify, first string) float64 {
		return flexBoxTextLine(t, flexEmptyItemDoc("flex-direction:column;height:400pt;"+justify, first)).y
	}
	start := col("justify-content:flex-start", "")
	end := col("justify-content:flex-end", "")
	free := start - end // PDF y grows upward
	if free < 100 {
		t.Fatalf("column flex-end moved box only %.2fpt; want most of the container height", free)
	}
	got := start - col("justify-content:space-between", `<div></div>`)
	if math.Abs(got-free) > tol {
		t.Errorf("column space-between box offset = %.2fpt, want %.2fpt", got, free)
	}
}
