// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"reflect"
	"testing"

	foliohtml "github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

// sampleHTML exercises the parts of a ConvertResult a naive ConvertFull+Add
// pipeline drops: @page geometry/margins plus base, :first, :left, and :right
// margin boxes, and a position:fixed element.
const sampleHTML = `<!DOCTYPE html><html><head>
<title>Sample</title>
<meta name="author" content="Tester">
<meta name="subject" content="Subj">
<meta name="keywords" content="k1, k2">
<meta name="generator" content="Gen">
<style>
  @page { size: A4; margin: 40px; @bottom-center { content: counter(page); } }
  @page :first { margin: 20px; @top-center { content: "First"; } }
  @page :left { margin-left: 60px; @bottom-left { content: "L"; color: red; } }
  @page :right { margin-right: 60px; @bottom-right { content: "R"; color: blue; } }
  .wm { position: fixed; top: 10px; left: 10px; }
</style></head><body>
<div class="wm">WATERMARK</div>
<h1>Heading</h1>
<p>Body paragraph.</p>
</body></html>`

// TestAddConvertResultMatchesAddHTML asserts AddHTML and
// ConvertFull+AddConvertResult leave the document in the same state — the
// contract that lets a caller use the raw result without losing anything.
func TestAddConvertResultMatchesAddHTML(t *testing.T) {
	viaAddHTML := NewDocument(PageSizeLetter)
	if err := viaAddHTML.AddHTML(sampleHTML, nil); err != nil {
		t.Fatalf("AddHTML: %v", err)
	}

	viaResult := NewDocument(PageSizeLetter)
	result, err := foliohtml.ConvertFull(sampleHTML, nil)
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	if err := viaResult.AddConvertResult(result); err != nil {
		t.Fatalf("AddConvertResult: %v", err)
	}

	if viaResult.pageSize != viaAddHTML.pageSize {
		t.Errorf("pageSize: AddConvertResult=%v AddHTML=%v", viaResult.pageSize, viaAddHTML.pageSize)
	}
	if viaResult.margins != viaAddHTML.margins {
		t.Errorf("margins: AddConvertResult=%v AddHTML=%v", viaResult.margins, viaAddHTML.margins)
	}
	assertMarginsEq(t, "firstMargins", viaResult.firstMargins, viaAddHTML.firstMargins)
	assertMarginsEq(t, "leftMargins", viaResult.leftMargins, viaAddHTML.leftMargins)
	assertMarginsEq(t, "rightMargins", viaResult.rightMargins, viaAddHTML.rightMargins)
	if len(viaResult.elements) != len(viaAddHTML.elements) {
		t.Errorf("elements: AddConvertResult=%d AddHTML=%d", len(viaResult.elements), len(viaAddHTML.elements))
	}
	if len(viaResult.absolutes) != len(viaAddHTML.absolutes) {
		t.Errorf("absolutes: AddConvertResult=%d AddHTML=%d", len(viaResult.absolutes), len(viaAddHTML.absolutes))
	}
	for _, mb := range []struct {
		name string
		a, b map[string]layout.MarginBox
	}{
		{"marginBoxes", viaResult.marginBoxes, viaAddHTML.marginBoxes},
		{"firstMarginBoxes", viaResult.firstMarginBoxes, viaAddHTML.firstMarginBoxes},
		{"leftMarginBoxes", viaResult.leftMarginBoxes, viaAddHTML.leftMarginBoxes},
		{"rightMarginBoxes", viaResult.rightMarginBoxes, viaAddHTML.rightMarginBoxes},
	} {
		if len(mb.a) == 0 {
			t.Errorf("%s: fixture produced none; the :first/:left/:right path is unexercised", mb.name)
		}
		if !reflect.DeepEqual(mb.a, mb.b) {
			t.Errorf("%s mismatch:\n AddConvertResult=%+v\n AddHTML=%+v", mb.name, mb.a, mb.b)
		}
	}
	if viaResult.Info != viaAddHTML.Info {
		t.Errorf("metadata mismatch:\n AddConvertResult=%+v\n AddHTML=%+v", viaResult.Info, viaAddHTML.Info)
	}
}

func assertMarginsEq(t *testing.T, name string, a, b *layout.Margins) {
	t.Helper()
	if (a == nil) != (b == nil) {
		t.Errorf("%s: presence differs AddConvertResult=%v AddHTML=%v", name, a, b)
		return
	}
	if a != nil && *a != *b {
		t.Errorf("%s: AddConvertResult=%v AddHTML=%v", name, *a, *b)
	}
}

// TestAddConvertResultCarriesAbsolutesAndPageConfig is the motivating
// regression: a position:fixed element and @page margin boxes must survive
// (the naive Elements-only loop drops both).
func TestAddConvertResultCarriesAbsolutesAndPageConfig(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	result, err := foliohtml.ConvertFull(sampleHTML, nil)
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	if len(result.Absolutes) == 0 {
		t.Fatal("test fixture produced no absolutes; sampleHTML must contain a position:fixed element")
	}
	if err := doc.AddConvertResult(result); err != nil {
		t.Fatalf("AddConvertResult: %v", err)
	}
	if len(doc.absolutes) != len(result.Absolutes) {
		t.Errorf("fixed/absolute elements dropped: doc has %d, result had %d", len(doc.absolutes), len(result.Absolutes))
	}
	if len(doc.marginBoxes) == 0 {
		t.Error("@page margin box (page numbering) was not applied")
	}
	// The @page { margin: 40px } overrides the 72pt default on all sides.
	if doc.margins.Top == 72 {
		t.Error("@page margins were not applied (still at the 72pt default)")
	}
	// The :left/:right reconstruction must preserve every field (notably the
	// color set on those boxes), matching the converter's own LeftMarginBoxes
	// /RightMarginBoxes. Dropping HasColor/Embedded would diverge here.
	if !reflect.DeepEqual(doc.leftMarginBoxes, result.LeftMarginBoxes) {
		t.Errorf("left margin boxes not faithfully reconstructed:\n doc=%+v\n result=%+v", doc.leftMarginBoxes, result.LeftMarginBoxes)
	}
	if !reflect.DeepEqual(doc.rightMarginBoxes, result.RightMarginBoxes) {
		t.Errorf("right margin boxes not faithfully reconstructed:\n doc=%+v\n result=%+v", doc.rightMarginBoxes, result.RightMarginBoxes)
	}
}

func TestAddConvertResultNil(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	if err := doc.AddConvertResult(nil); err == nil {
		t.Error("AddConvertResult(nil) should return an error")
	}
}

func TestSetAndGetPageSize(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	if got := doc.PageSize(); got != PageSizeLetter {
		t.Errorf("PageSize() = %v, want %v (from NewDocument)", got, PageSizeLetter)
	}
	doc.SetPageSize(PageSizeA4)
	if got := doc.PageSize(); got != PageSizeA4 {
		t.Errorf("after SetPageSize: PageSize() = %v, want %v", got, PageSizeA4)
	}
}
