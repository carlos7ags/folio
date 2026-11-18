// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html_test

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
)

// TestContentSurvivesFirstLineDeferral collects regressions from a paragraph
// deferring a first line that does not fit when it is not at the top of a
// fragment. That is right in page flow, but a caller that lays content out in
// a bounded area which is not carried to the next page (an inline-block, a
// definite-height box, an absolutely positioned box, a list item) must not
// let it defer: the content was silently dropped. Every case must render on
// a single page with all of its text.
func TestContentSurvivesFirstLineDeferral(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []string
	}{
		// An inline-block is re-planned at the height it was measured with;
		// float rounding in its padding left the inner paragraph a hair
		// short, so the paragraph deferred its only line and the box drew
		// empty.
		{name: "inline-block span", body: `<p>See <span style="display:inline-block;padding:10px;background:#eee">ALPHA BETA</span> end</p>`, want: []string{"ALPHA BETA", "end"}},
		{name: "inline-block anchor", body: `<p>See <a href="https://example.com" style="display:inline-block;padding:10px">ALPHA BETA</a> end</p>`, want: []string{"ALPHA BETA", "end"}},
		{name: "nested inline-block", body: `<p>See <span style="display:inline-block;padding:7px"><span style="display:inline-block;padding:10px">ALPHA</span> BETA</span> end</p>`, want: []string{"ALPHA", "BETA", "end"}},
		{name: "inline-block in a table cell", body: `<table><tr><td>See <span style="display:inline-block;padding:10px">ALPHA</span> end</td></tr></table>`, want: []string{"ALPHA", "end"}},
		// A definite-height box contains its content instead of
		// fragmenting it: a child taller than the box overflows it rather
		// than being deferred (and dropped, since the box is not carried
		// to the next page).
		{name: "inline-block with explicit height", body: `<p>TOP <span style="display:inline-block;height:14px;padding:2px 6px;font-size:12px">ALPHA</span> OMEGA</p>`, want: []string{"TOP", "ALPHA", "OMEGA"}},
		{name: "block shorter than its line", body: `<p>TOP</p><div style="height:12px;font-size:12px">ALPHA</div><p>OMEGA</p>`, want: []string{"TOP", "ALPHA", "OMEGA"}},
		{name: "short block in a table cell", body: `<table><tr><td><div style="height:8px">ALPHA</div></td></tr></table>`, want: []string{"ALPHA"}},
		// Absolutely positioned content is out of flow and never
		// fragments, so it is not bounded by its containing block's
		// height.
		{name: "absolute box with padding and border", body: `<div style="position:relative">HOST<div style="position:absolute;top:0;right:0;padding:3px;border:1px solid">ALPHA</div></div>`, want: []string{"HOST", "ALPHA"}},
		{name: "absolute text in a short host", body: `<div style="position:relative;height:5px"><div style="position:absolute;top:0;left:0">ALPHA</div></div>`, want: []string{"ALPHA"}},
		// A definite-height flex row places an item taller than the
		// container whole, overflowing it.
		{name: "short flex row at page top", body: `<div style="display:flex;height:5px"><div>ALPHA</div><div>BETA</div></div>`, want: []string{"ALPHA", "BETA"}},
		{name: "short flex row below content", body: `<p>TOP</p><div style="display:flex;height:10px"><div>ALPHA</div><div>BETA</div></div>`, want: []string{"TOP", "ALPHA", "BETA"}},
		// A list's first element item sits at the fragment's top when
		// the list does, so its first line is forced instead of deferred.
		{name: "list item in a short box", body: `<div style="height:5px"><ul><li><div>ALPHA</div></li></ul></div>`, want: []string{"ALPHA"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			htmlStr := `<html><head><style>body { margin: 0; font-size: 12px; }</style></head><body>` +
				tc.body + `</body></html>`
			_, r := htmlRoundtrip(t, htmlStr, document.PageSizeLetter)
			if n := r.PageCount(); n != 1 {
				t.Errorf("got %d pages, want 1", n)
			}
			page, err := r.Page(0)
			if err != nil {
				t.Fatal(err)
			}
			txt, err := page.ExtractText()
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.want {
				if !strings.Contains(txt, w) {
					t.Errorf("%q missing from page text %q", w, txt)
				}
			}
		})
	}
}
