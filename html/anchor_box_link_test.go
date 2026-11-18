// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html_test

import (
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/reader"
)

// linkRects returns the /Rect of every /Link annotation on every page, in
// page order, as [llx lly urx ury].
func linkRects(t *testing.T, r *reader.PdfReader) [][4]float64 {
	t.Helper()
	var out [][4]float64
	for i := range r.PageCount() {
		page, err := r.Page(i)
		if err != nil {
			t.Fatalf("page %d: %v", i, err)
		}
		annotsObj, err := r.ResolveObject(page.Dict().Get("Annots"))
		if err != nil {
			continue
		}
		annots, ok := annotsObj.(*core.PdfArray)
		if !ok {
			continue
		}
		for j := range annots.Len() {
			annObj, err := r.ResolveObject(annots.At(j))
			if err != nil {
				continue
			}
			ann, ok := annObj.(*core.PdfDictionary)
			if !ok {
				continue
			}
			if sub, ok := ann.Get("Subtype").(*core.PdfName); !ok || sub.Value != "Link" {
				continue
			}
			rectObj, err := r.ResolveObject(ann.Get("Rect"))
			if err != nil {
				t.Fatalf("resolve /Rect: %v", err)
			}
			rect, ok := rectObj.(*core.PdfArray)
			if !ok || rect.Len() != 4 {
				t.Fatalf("/Rect is %v, want a 4-element array", rectObj)
			}
			var box [4]float64
			for k := range 4 {
				box[k] = pdfNumber(t, rect.At(k))
			}
			out = append(out, box)
		}
	}
	return out
}

func pdfNumber(t *testing.T, o core.PdfObject) float64 {
	t.Helper()
	switch v := o.(type) {
	case *core.PdfNumber:
		return v.FloatValue()
	}
	t.Fatalf("expected a number, got %T", o)
	return 0
}

// TestBoxAnchorEmitsLink is the regression for an <a> that generates a
// block-level box of its own — display:flex, display:grid, or
// display:inline-block. Those displays dispatch the anchor to the
// container converters instead of the link converter, which used to drop
// the href: the content rendered but nothing on the page was clickable.
// The anchor must produce exactly one link annotation that covers its box
// (padding included) and navigates to the target.
func TestBoxAnchorEmitsLink(t *testing.T) {
	cases := []struct {
		name      string
		display   string
		fullWidth bool // block-level boxes span the content width
	}{
		{"flex", "flex", true},
		{"grid", "grid", true},
		{"inline-block", "inline-block", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			htmlStr := `<html><head><style>
				body { margin: 0; font-size: 12px; }
				.box { display: ` + tc.display + `; padding: 10px; background: #d4f1e4; }
			</style></head><body>
				<a class="box" href="#target"><span>box anchor content</span></a>
				<h2 id="target">Target</h2>
			</body></html>`

			_, r := htmlRoundtrip(t, htmlStr, document.PageSizeLetter)
			annots := collectLinkAnnots(t, r)
			assertNoFragmentURI(t, annots)
			if len(annots) != 1 {
				t.Fatalf("got %d link annotations, want 1", len(annots))
			}
			a := annots[0]
			if !a.followable() {
				t.Fatalf("annotation navigates nowhere (uri=%q destName=%q hasDest=%v)",
					a.uri, a.destName, a.hasDest)
			}
			if a.destName != "" && a.destName != "target" {
				t.Errorf("/GoTo names %q, want target", a.destName)
			}
			resolveNamedDest(t, r, "target")

			rects := linkRects(t, r)
			if len(rects) != 1 {
				t.Fatalf("got %d link rects, want 1", len(rects))
			}
			box := rects[0]
			w, h := box[2]-box[0], box[3]-box[1]
			// 12px (9pt) text plus 10px (7.5pt) padding top and bottom.
			if h < 9+15 {
				t.Errorf("link rect height = %.1f, want >= 24 (padding box)", h)
			}
			if tc.fullWidth && w < 400 {
				t.Errorf("link rect width = %.1f, want the full content width", w)
			}
			if !tc.fullWidth && (w < 20 || w > 400) {
				t.Errorf("link rect width = %.1f, want the shrink-to-fit box width", w)
			}
		})
	}
}

// TestBoxAnchorExternalLink checks the same path keeps an external href as
// a /URI action.
func TestBoxAnchorExternalLink(t *testing.T) {
	_, r := htmlRoundtrip(t, `<html><body>
		<a href="https://example.com/x" style="display:flex"><span>out</span></a>
	</body></html>`, document.PageSizeLetter)
	annots := collectLinkAnnots(t, r)
	if len(annots) != 1 {
		t.Fatalf("got %d link annotations, want 1", len(annots))
	}
	if annots[0].uri != "https://example.com/x" {
		t.Errorf("uri = %q, want https://example.com/x", annots[0].uri)
	}
}

// TestBoxAnchorWithoutHrefHasNoLink guards against annotating an <a> that
// navigates nowhere.
func TestBoxAnchorWithoutHrefHasNoLink(t *testing.T) {
	_, r := htmlRoundtrip(t, `<html><body>
		<a style="display:flex"><span>no href</span></a>
		<a href="#" style="display:flex"><span>bare hash</span></a>
	</body></html>`, document.PageSizeLetter)
	if annots := collectLinkAnnots(t, r); len(annots) != 0 {
		t.Fatalf("got %d link annotations, want 0", len(annots))
	}
}

// TestInlineBlockAnchorInTextEmitsLink covers an inline-block <a> that
// flows inside a paragraph as an atomic inline box.
func TestInlineBlockAnchorInTextEmitsLink(t *testing.T) {
	_, r := htmlRoundtrip(t, `<html><body>
		<p>Before <a href="#target" style="display:inline-block; padding:4px; background:#eee">button</a> after.</p>
		<h2 id="target">Target</h2>
	</body></html>`, document.PageSizeLetter)
	annots := collectLinkAnnots(t, r)
	if len(annots) != 1 {
		t.Fatalf("got %d link annotations, want 1", len(annots))
	}
	if !annots[0].followable() {
		t.Fatal("annotation navigates nowhere")
	}

	// The rect must cover the inline box itself: right of the preceding
	// text, and only as wide as the box. The same box at the start of a
	// paragraph gives the reference left edge and width.
	_, ref := htmlRoundtrip(t, `<html><body>
		<p><a href="#target" style="display:inline-block; padding:4px; background:#eee">button</a> after.</p>
		<h2 id="target">Target</h2>
	</body></html>`, document.PageSizeLetter)
	got, want := linkRects(t, r), linkRects(t, ref)
	if len(got) != 1 || len(want) != 1 {
		t.Fatalf("got %d and %d link rects, want 1 each", len(got), len(want))
	}
	if got[0][0] < want[0][0]+10 {
		t.Errorf("link rect starts at x=%.1f, want right of the preceding text (box at paragraph start is at x=%.1f)",
			got[0][0], want[0][0])
	}
	gotW, wantW := got[0][2]-got[0][0], want[0][2]-want[0][0]
	if diff := gotW - wantW; diff < -0.5 || diff > 0.5 {
		t.Errorf("link rect width = %.1f, want the inline box width %.1f", gotW, wantW)
	}
	if gotW < 20 || gotW > 150 {
		t.Errorf("link rect width = %.1f, want the shrink-to-fit box width", gotW)
	}
}

// TestFlexItemAnchorWidthResolvedOnce is the regression for a box <a> that
// is itself a flex item with a percentage width. The width is consumed as
// the flex basis and must not be resolved again against the item's own
// allocated width: the link (and the box) must span 50% of the container,
// exactly as a <div> item would.
func TestFlexItemAnchorWidthResolvedOnce(t *testing.T) {
	_, r := htmlRoundtrip(t, `<html><body>
		<div style="display:flex; width:400px">
			<a href="#target" style="display:inline-block; width:50%; background:#eee">x</a>
			<div>y</div>
		</div>
		<h2 id="target">Target</h2>
	</body></html>`, document.PageSizeLetter)
	rects := linkRects(t, r)
	if len(rects) != 1 {
		t.Fatalf("got %d link rects, want 1", len(rects))
	}
	// 50% of 400px is 200px = 150pt.
	if w := rects[0][2] - rects[0][0]; w < 149.5 || w > 150.5 {
		t.Errorf("link rect width = %.1f, want 150 (the flex item width)", w)
	}

	// The <div> equivalent: a full-width link box inside a 50% <div> item
	// measures that item's width.
	_, ref := htmlRoundtrip(t, `<html><body>
		<div style="display:flex; width:400px">
			<div style="width:50%"><a href="#target" style="display:flex">x</a></div>
			<div>y</div>
		</div>
		<h2 id="target">Target</h2>
	</body></html>`, document.PageSizeLetter)
	refRects := linkRects(t, ref)
	if len(refRects) != 1 {
		t.Fatalf("reference: got %d link rects, want 1", len(refRects))
	}
	if got, want := rects[0][2]-rects[0][0], refRects[0][2]-refRects[0][0]; got-want < -0.5 || got-want > 0.5 {
		t.Errorf("anchor item width = %.1f, want the <div> item width %.1f", got, want)
	}
}

// followingLinkTop renders body and returns the top edge of the last link
// annotation, used as a probe for where the content after a flex row
// starts.
func followingLinkTop(t *testing.T, body string) float64 {
	t.Helper()
	_, r := htmlRoundtrip(t, `<html><body style="margin:0">`+body+`
		<a href="#target" style="display:flex">after</a>
		<h2 id="target">Target</h2>
	</body></html>`, document.PageSizeLetter)
	rects := linkRects(t, r)
	if len(rects) == 0 {
		t.Fatal("no link rects")
	}
	return rects[len(rects)-1][3]
}

// TestFlexItemMarginsAppliedOnce checks a flex item's margins are applied
// once even when the item is wrapped by a decorator: a box <a> (link
// wrapper) or an element with an id (anchor wrapper). The flex item owns
// the margins, so the element's own space before/after must be cleared
// through the wrapper; otherwise the row grows by the margins a second
// time and the content after it moves down.
func TestFlexItemMarginsAppliedOnce(t *testing.T) {
	want := followingLinkTop(t, `<div style="display:flex">
		<div style="display:flex; margin:20px">x</div>
	</div>`)
	cases := []struct{ name, body string }{
		{"anchor", `<div style="display:flex">
			<a href="#target" style="display:flex; margin:20px">x</a>
		</div>`},
		{"id", `<div style="display:flex">
			<div id="item" style="display:flex; margin:20px">x</div>
		</div>`},
		{"id-block", `<div style="display:flex">
			<div id="item" style="margin:20px">x</div>
		</div>`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := followingLinkTop(t, tc.body); got-want < -0.5 || got-want > 0.5 {
				t.Errorf("content after the row starts at y=%.1f, want %.1f (as with a plain <div>)", got, want)
			}
		})
	}
}

// TestBoxAnchorRectExcludesMargin checks the link covers the border box,
// not the margin around it.
func TestBoxAnchorRectExcludesMargin(t *testing.T) {
	_, r := htmlRoundtrip(t, `<html><body style="margin:0">
		<a href="#target" style="display:flex; margin:40px 0; padding:0; font-size:12px; line-height:20px"><span>x</span></a>
		<h2 id="target">Target</h2>
	</body></html>`, document.PageSizeLetter)
	rects := linkRects(t, r)
	if len(rects) != 1 {
		t.Fatalf("got %d link rects, want 1", len(rects))
	}
	// line-height 20px is 15pt; the 40px (30pt) margins must not be included.
	if h := rects[0][3] - rects[0][1]; h > 20 {
		t.Errorf("link rect height = %.1f, want about 15 (margins excluded)", h)
	}
}

// TestBoxAnchorAcrossPagesLinksEachFragment checks a flex <a> that breaks
// across a page stays clickable on every page it occupies.
func TestBoxAnchorAcrossPagesLinksEachFragment(t *testing.T) {
	_, r := htmlRoundtrip(t, `<html><body>
		<a href="#target" style="display:flex; flex-direction:column">
			<div style="height:500px">first</div>
			<div style="height:500px">second</div>
		</a>
		<h2 id="target">Target</h2>
	</body></html>`, document.PageSizeLetter)
	if r.PageCount() < 2 {
		t.Fatalf("got %d pages, want the anchor to span two", r.PageCount())
	}
	annots := collectLinkAnnots(t, r)
	if len(annots) != 2 {
		t.Fatalf("got %d link annotations, want one per page fragment (2)", len(annots))
	}
}
