// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html_test

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/reader"
)

// linkAnnot is a parsed /Link annotation, reduced to the parts that decide
// whether a viewer can follow it.
type linkAnnot struct {
	uri      string // /A << /S /URI /URI (...) >>
	destName string // /A << /S /GoTo /D (name) >>
	hasDest  bool   // /Dest [ pageRef /XYZ ... ] — a resolved direct destination
	destPage core.PdfObject
}

// followable reports whether the annotation actually navigates somewhere:
// either a direct /Dest array or a /GoTo naming a destination.
func (a linkAnnot) followable() bool {
	return a.hasDest || a.destName != "" || (a.uri != "" && !strings.HasPrefix(a.uri, "#"))
}

// collectLinkAnnots walks the page tree and returns every /Link annotation,
// in page order. Assertions run against parsed objects rather than a byte
// scan so a link that is present but points nowhere cannot pass.
func collectLinkAnnots(t *testing.T, r *reader.PdfReader) []linkAnnot {
	t.Helper()
	var out []linkAnnot
	cat := r.Catalog()
	if cat == nil {
		t.Fatal("no catalog in parsed PDF")
	}
	pagesObj, err := r.ResolveObject(cat.Get("Pages"))
	if err != nil {
		t.Fatalf("resolve /Pages: %v", err)
	}
	pages, ok := pagesObj.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("/Pages is %T, want *core.PdfDictionary", pagesObj)
	}
	kidsObj, err := r.ResolveObject(pages.Get("Kids"))
	if err != nil {
		t.Fatalf("resolve /Kids: %v", err)
	}
	kids, ok := kidsObj.(*core.PdfArray)
	if !ok {
		t.Fatalf("/Kids is %T, want *core.PdfArray", kidsObj)
	}
	for i := range kids.Len() {
		pageObj, err := r.ResolveObject(kids.At(i))
		if err != nil {
			t.Fatalf("resolve page %d: %v", i, err)
		}
		page, ok := pageObj.(*core.PdfDictionary)
		if !ok {
			continue
		}
		annotsObj, err := r.ResolveObject(page.Get("Annots"))
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
			la := linkAnnot{}
			if dest, ok := ann.Get("Dest").(*core.PdfArray); ok && dest.Len() > 0 {
				la.hasDest = true
				la.destPage = dest.At(0)
			}
			if actObj, err := r.ResolveObject(ann.Get("A")); err == nil {
				if act, ok := actObj.(*core.PdfDictionary); ok {
					s, _ := act.Get("S").(*core.PdfName)
					switch {
					case s != nil && s.Value == "URI":
						if v, ok := act.Get("URI").(*core.PdfString); ok {
							la.uri = v.Text()
						}
					case s != nil && s.Value == "GoTo":
						if v, ok := act.Get("D").(*core.PdfString); ok {
							la.destName = v.Text()
						}
					}
				}
			}
			out = append(out, la)
		}
	}
	return out
}

// assertNoFragmentURI fails if any annotation carries a /URI action whose
// value is a bare fragment. Such an action resolves to nothing in every
// viewer, so the link is dead even though it looks present.
func assertNoFragmentURI(t *testing.T, annots []linkAnnot) {
	t.Helper()
	for i, a := range annots {
		if strings.HasPrefix(a.uri, "#") {
			t.Errorf("annotation %d: /URI (%s) — a bare fragment is not a URI; "+
				"an internal anchor must emit a destination, not a /URI action", i, a.uri)
		}
	}
}

// TestInlineInternalAnchorEmitsDestination is the regression for internal
// anchors that flow inline. <a href="#id"> is display:inline by default, so
// in a heading, a paragraph, a list item or a table cell it is converted as
// part of the surrounding text rather than as its own element. Those paths
// used to copy the raw href onto the text run's link URI, producing a
// /URI (#id) action that navigates nowhere. Every one of them must resolve
// to the registered destination instead.
func TestInlineInternalAnchorEmitsDestination(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"heading", `<h2>Section <a href="#section-two">jump link</a> here</h2>`},
		{"paragraph", `<p>Lead text <a href="#section-two">jump link</a> trailing text.</p>`},
		{"list item", `<ul><li>Item with <a href="#section-two">jump link</a>.</li></ul>`},
		{"table cell", `<table><tr><td>Cell with <a href="#section-two">jump link</a></td></tr></table>`},
		// The control: an <a> forced to display:block is dispatched as its
		// own element and has always worked. It proves the boundary.
		{"block control", `<a href="#section-two" style="display:block">jump link</a>`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			htmlStr := `<html><body>` + tc.body + `
				<h2 id="section-two">Second Section</h2>
				<p>Body text under the second section.</p>
			</body></html>`

			pdf, r := htmlRoundtrip(t, htmlStr, document.PageSizeLetter)

			annots := collectLinkAnnots(t, r)
			assertNoFragmentURI(t, annots)

			if len(annots) == 0 {
				t.Fatal("no /Link annotation emitted — the anchor produced no clickable region")
			}
			for i, a := range annots {
				if !a.followable() {
					t.Errorf("annotation %d navigates nowhere (uri=%q destName=%q hasDest=%v)",
						i, a.uri, a.destName, a.hasDest)
				}
			}

			// The destination must be resolvable, not merely referenced.
			if !strings.Contains(string(pdf), "/Dests") {
				t.Fatal("output has no /Dests dictionary — the id was not registered")
			}
			dest := resolveNamedDest(t, r, "section-two")
			if dest.Len() < 2 {
				t.Fatalf("destination array too short: %d elements", dest.Len())
			}
			if fit, ok := dest.At(1).(*core.PdfName); !ok || fit.Value != "XYZ" {
				t.Errorf("destination fit = %v, want XYZ", dest.At(1))
			}

			// A direct /Dest must point at a page object, and a string /GoTo
			// must name the destination the catalog defines.
			for i, a := range annots {
				if a.hasDest && a.destPage == nil {
					t.Errorf("annotation %d has an empty /Dest array", i)
				}
				if a.destName != "" && a.destName != "section-two" {
					t.Errorf("annotation %d /GoTo names %q, want section-two", i, a.destName)
				}
			}
		})
	}
}

// TestInlineExternalLinkStillEmitsURI guards the fix from over-reaching: a
// real external href must keep its /URI action.
func TestInlineExternalLinkStillEmitsURI(t *testing.T) {
	const htmlStr = `<html><body>
		<p>Go <a href="https://example.com/docs">outside</a> the document.</p>
	</body></html>`

	_, r := htmlRoundtrip(t, htmlStr, document.PageSizeLetter)
	annots := collectLinkAnnots(t, r)
	if len(annots) != 1 {
		t.Fatalf("got %d link annotations, want 1", len(annots))
	}
	if annots[0].uri != "https://example.com/docs" {
		t.Errorf("uri = %q, want https://example.com/docs", annots[0].uri)
	}
}
