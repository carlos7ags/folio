// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"bytes"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// renderStreams renders the elements and returns each page's content-stream
// bytes. The named-destination metadata an anchor records lives on
// PageResult.Anchors, never in the drawn stream, so wrapping an element in an
// anchor must leave these bytes byte-for-byte identical — any difference means
// the wrapper changed layout or pagination.
func renderStreams(elems ...Element) [][]byte {
	r := NewRenderer(612, 250, Margins{Top: 20, Bottom: 20, Left: 20, Right: 20})
	for _, e := range elems {
		r.Add(e)
	}
	pages := r.Render()
	out := make([][]byte, len(pages))
	for i, p := range pages {
		out[i] = p.Stream.Bytes()
	}
	return out
}

func assertSameStreams(t *testing.T, plain, wrapped [][]byte, what string) {
	t.Helper()
	if len(plain) != len(wrapped) {
		t.Fatalf("%s: page count changed by wrapping in an anchor: %d -> %d "+
			"(the id altered pagination)", what, len(plain), len(wrapped))
	}
	for i := range plain {
		if !bytes.Equal(plain[i], wrapped[i]) {
			t.Errorf("%s: page %d content stream changed when the element carried an id "+
				"— the anchor wrapper masked an optional layout interface", what, i)
		}
	}
}

// TestAnchorPreservesKeepTogether pins finding 1: wrapping a
// page-break-inside:avoid Div in an anchor (as convertElement does for any
// element with an id) must NOT disable keep-together. Before baseElement, the
// wrapper masked the KeepTogether interface, so the div split across the page
// boundary instead of moving whole to the next page.
func TestAnchorPreservesKeepTogether(t *testing.T) {
	build := func(wrap bool) []Element {
		filler := NewDiv().SetPadding(5)
		for range 5 {
			filler.Add(NewParagraph("Filler line taking up vertical space on page.", font.Helvetica, 12))
		}
		kt := NewDiv().SetPadding(10).SetKeepTogether(true)
		for range 10 {
			kt.Add(NewParagraph("Keep-together line that needs space on the page.", font.Helvetica, 12))
		}
		var target Element = kt
		if wrap {
			target = NewAnchor(kt, "section")
		}
		return []Element{filler, target}
	}

	plain := renderStreams(build(false)...)
	wrapped := renderStreams(build(true)...)
	assertSameStreams(t, plain, wrapped, "keep-together")
}

// TestAnchorPreservesOptionalInterfaces pins the mechanism behind finding 1:
// the layouter reaches an element's CSS clear (Clearable) and cross-axis
// stretch (HeightSettable) via optional-interface assertions, which the
// anchor wrapper used to mask. Through baseElement they must resolve to the
// inner element's values.
func TestAnchorPreservesOptionalInterfaces(t *testing.T) {
	// Clearable: a cleared div wrapped in an anchor still reports its clear.
	cleared := NewDiv().SetClear("both")
	if cl, ok := baseElement(NewAnchor(cleared, "x")).(Clearable); !ok {
		t.Error("anchor-wrapped Div no longer satisfies Clearable via baseElement")
	} else if cl.ClearValue() != "both" {
		t.Errorf("ClearValue through wrapper = %q, want %q", cl.ClearValue(), "both")
	}

	// HeightSettable: a div (which supports forced height for flex stretch)
	// wrapped in an anchor still exposes it.
	box := NewDiv()
	if _, ok := baseElement(NewAnchor(box, "x")).(HeightSettable); !ok {
		t.Error("anchor-wrapped Div no longer satisfies HeightSettable via baseElement")
	}
}

// TestBaseElementSeesThroughWrappers is the direct unit check for the fix:
// baseElement must recover the inner element through anchor and bookmark
// wrappers (and their nesting), so optional-interface assertions match it.
func TestBaseElementSeesThroughWrappers(t *testing.T) {
	inner := &fakeElement{width: 100, height: 10}

	if got := baseElement(NewAnchor(inner, "x")); got != Element(inner) {
		t.Errorf("baseElement through anchor = %p, want inner %p", got, inner)
	}
	if got := baseElement(NewBookmarkAnchor(inner, 2, "L", false)); got != Element(inner) {
		t.Errorf("baseElement through bookmark = %p, want inner %p", got, inner)
	}
	// Nested: bookmark(anchor(inner)) — convertElement can apply both.
	nested := NewBookmarkAnchor(NewAnchor(inner, "x"), 2, "L", false)
	if got := baseElement(nested); got != Element(inner) {
		t.Errorf("baseElement through nested wrappers = %p, want inner %p", got, inner)
	}

	// A Measurable inner must be recovered through the measurable variant too.
	m := &measurableFake{fakeElement: fakeElement{width: 100, height: 10}, min: 42, max: 84}
	if got := baseElement(NewAnchor(m, "x")); got != Element(m) {
		t.Errorf("baseElement through measurable anchor = %p, want inner %p", got, m)
	}
}
