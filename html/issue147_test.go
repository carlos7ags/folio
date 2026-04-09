// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// TestIssue147_BrInsideStrongInList reproduces the exact crash from #147:
// a <br> inside a <strong> inside an <li> produces a TextRun{Text:"\n"}
// with nil Font, which panics in NewStyledParagraph.
func TestIssue147_BrInsideStrongInList(t *testing.T) {
	src := `<ol>
   <li>This is a <strong>test<br />to see</strong> if it breaks</li>
</ol>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
	if plan.Status == layout.LayoutNothing {
		t.Error("expected renderable output, got LayoutNothing")
	}
	if plan.Consumed <= 0 {
		t.Errorf("expected positive Consumed, got %v", plan.Consumed)
	}
}

// TestIssue147_BrInsideEmInParagraph covers the same pattern in a <p>.
func TestIssue147_BrInsideEmInParagraph(t *testing.T) {
	src := `<p>Hello <em>world<br/>!</em></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive Consumed, got %v", plan.Consumed)
	}
}

// TestIssue147_BrInsideAnchor covers <br> inside <a>.
func TestIssue147_BrInsideAnchor(t *testing.T) {
	src := `<p>Click <a href="#">here<br/>now</a></p>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive Consumed, got %v", plan.Consumed)
	}
}

// TestIssue10_BrInsideSpan reproduces the crash from the earlier #10:
// same root cause as #147, different inline element (<span> vs <strong>).
func TestIssue10_BrInsideSpan(t *testing.T) {
	src := `<span>xxxxxxxxxxx<br>xxxxxxxxxxxxxxxx</span>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
	if plan.Consumed <= 0 {
		t.Errorf("expected positive Consumed, got %v", plan.Consumed)
	}
}

// TestIssue147_BrAsFirstChild covers <br> as the first child of an inline
// element inside a list item.
func TestIssue147_BrAsFirstChild(t *testing.T) {
	src := `<ol><li><strong><br/>text</strong></li></ol>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 500, Height: 500})
	if plan.Status == layout.LayoutNothing {
		t.Error("expected renderable output")
	}
}

// TestIssue147_BrAsLastChild covers <br> as the last child.
func TestIssue147_BrAsLastChild(t *testing.T) {
	src := `<ol><li><strong>text<br/></strong></li></ol>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

// TestIssue147_MultipleBrInStrong covers multiple consecutive <br> inside
// a styled inline element.
func TestIssue147_MultipleBrInStrong(t *testing.T) {
	src := `<ol><li><strong>text<br/><br/>more</strong></li></ol>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}

// TestIssue147_NestedInlineWithBr covers deeply nested inline elements.
func TestIssue147_NestedInlineWithBr(t *testing.T) {
	src := `<ol><li><strong><em><br/></em></strong></li></ol>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
}
