// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html_test

import (
	"math"
	"testing"

	"github.com/carlos7ags/folio/document"
)

// TestHeadingSpaceAboveIsMarginTop locks in that, between other blocks,
// the space above an <h1>-<h6> is exactly its CSS margin-top, as in a
// browser, and that no extra gap is added above a heading that starts
// the page.
//
// Folio used to add a fixed half-of-the-level-size gap above every
// heading on top of the CSS margin, so `margin-top: 0` still left a
// visible gap and an explicit margin-top rendered larger than declared.
//
// At the top of a page the layout engine drops the leading margin of
// the first block, heading or div alike. That truncation is separate,
// pre-existing behavior; the page-top case only checks that the
// heading's built-in gap is not added inside the wrapper either.
//
// Each case renders a heading and a control: a plain <div> with the
// same font-size, font-weight and margins the heading resolves to. The
// two boxes must lay out identically, so every rendered baseline must
// coincide. Comparing against a control rather than hard-coded
// coordinates keeps the test independent of font metrics.
func TestHeadingSpaceAboveIsMarginTop(t *testing.T) {
	const css = `<style>body { margin: 0; font-size: 12px } p { margin: 0 }</style>`
	// h2 resolves to 18pt bold; its UA margins are 0.83em = 14.94pt.
	const h2Box = `font-size: 18pt; font-weight: bold;`

	cases := []struct {
		name    string
		heading string
		control string
	}{
		{
			name:    "margin-top zero",
			heading: `<p>AAA</p><h2 style="margin-top: 0; margin-bottom: 0">BBB</h2><p>CCC</p>`,
			control: `<p>AAA</p><div style="` + h2Box + ` margin: 0">BBB</div><p>CCC</p>`,
		},
		{
			name:    "explicit margin-top",
			heading: `<p>AAA</p><h2 style="margin-top: 40px; margin-bottom: 0">BBB</h2><p>CCC</p>`,
			control: `<p>AAA</p><div style="` + h2Box + ` margin: 30pt 0 0 0">BBB</div><p>CCC</p>`,
		},
		{
			name:    "UA default margins",
			heading: `<p>AAA</p><h2>BBB</h2><p>CCC</p>`,
			control: `<p>AAA</p><div style="` + h2Box + ` margin: 14.94pt 0">BBB</div><p>CCC</p>`,
		},
		{
			name:    "h1 UA default margins",
			heading: `<p>AAA</p><h1>BBB</h1><p>CCC</p>`,
			control: `<p>AAA</p><div style="font-size: 24pt; font-weight: bold; margin: 16.08pt 0">BBB</div><p>CCC</p>`,
		},
		{
			// Both boxes lose their 14.94pt margin-top at the page
			// top; the heading must not add its own gap inside.
			name:    "page top no built-in gap",
			heading: `<h2>BBB</h2><p>CCC</p>`,
			control: `<div style="` + h2Box + ` margin: 14.94pt 0">BBB</div><p>CCC</p>`,
		},
	}

	const tol = 0.05
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, hr := htmlRoundtrip(t, `<html><head>`+css+`</head><body>`+tc.heading+`</body></html>`, document.PageSizeA4)
			_, cr := htmlRoundtrip(t, `<html><head>`+css+`</head><body>`+tc.control+`</body></html>`, document.PageSizeA4)
			got := renderedLines(t, hr)
			want := renderedLines(t, cr)
			if len(got) != len(want) {
				t.Fatalf("heading render has %d lines, control has %d", len(got), len(want))
			}
			for i := range got {
				if got[i].text != want[i].text {
					t.Fatalf("line %d: heading text %q, control text %q", i, got[i].text, want[i].text)
				}
				if math.Abs(got[i].y-want[i].y) > tol {
					t.Errorf("line %d %q: baseline y = %.2f, want %.2f (same as an equivalent div); off by %.2fpt",
						i, got[i].text, got[i].y, want[i].y, want[i].y-got[i].y)
				}
			}
		})
	}
}
