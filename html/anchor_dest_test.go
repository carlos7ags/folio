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

// TestInternalAnchorRegistersNamedDest is the end-to-end regression for
// internal-link navigation: an element with an id must auto-register a
// PDF named destination so <a href="#id"> resolves to it. Before the
// fix, the id was consumed only for CSS matching, so the link emitted a
// dangling /GoTo pointing at a destination that was never defined.
func TestInternalAnchorRegistersNamedDest(t *testing.T) {
	const htmlStr = `<html><body>
		<a href="#section-two">Jump to section two</a>
		<h2 id="section-two">Second Section</h2>
		<p>Body text under the second section.</p>
	</body></html>`

	pdf, r := htmlRoundtrip(t, htmlStr, document.PageSizeLetter)
	got := string(pdf)

	// The catalog must carry a /Dests dictionary defining the id.
	if !strings.Contains(got, "/Dests") {
		t.Error("output has no /Dests dictionary — the id was not registered")
	}
	if !strings.Contains(got, "section-two") {
		t.Error("destination name section-two missing from output")
	}
	// The resolved link should use XYZ (jump to the section's y while
	// retaining the current zoom, matching auto-bookmarks), and no dangling
	// string GoTo action should remain for a resolvable dest.
	if !strings.Contains(got, "/XYZ") {
		t.Error("resolved destination should use /XYZ to land on the section without clobbering zoom")
	}
	if strings.Contains(got, "/GoTo") {
		t.Error("output still contains a /GoTo action — the internal link did not resolve to a direct /Dest")
	}

	// Walk the catalog name → destination the way a viewer would, so we
	// prove the destination is *defined*, not merely referenced.
	dest := resolveNamedDest(t, r, "section-two")
	if dest.Len() < 2 {
		t.Fatalf("destination array too short: %d elements", dest.Len())
	}
	fit, ok := dest.At(1).(*core.PdfName)
	if !ok || fit.Value != "XYZ" {
		t.Errorf("destination fit = %v, want XYZ", dest.At(1))
	}
}

// resolveNamedDest looks up name in the catalog's /Dests dictionary and
// returns its destination array, resolving indirect references.
func resolveNamedDest(t *testing.T, r *reader.PdfReader, name string) *core.PdfArray {
	t.Helper()
	cat := r.Catalog()
	if cat == nil {
		t.Fatal("no catalog in parsed PDF")
	}
	destsObj, err := r.ResolveObject(cat.Get("Dests"))
	if err != nil {
		t.Fatalf("resolve /Dests: %v", err)
	}
	dests, ok := destsObj.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("/Dests is %T, want *core.PdfDictionary", destsObj)
	}
	arrObj, err := r.ResolveObject(dests.Get(name))
	if err != nil {
		t.Fatalf("resolve dest %q: %v", name, err)
	}
	arr, ok := arrObj.(*core.PdfArray)
	if !ok {
		t.Fatalf("destination %q is %T, want *core.PdfArray (name not defined)", name, arrObj)
	}
	return arr
}
