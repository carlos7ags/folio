// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

func TestMarkerPseudoElementColor(t *testing.T) {
	src := `<style>li::marker { color: red; }</style>
	<ul><li>Item one</li><li>Item two</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
	if plan.Consumed <= 0 {
		t.Error("expected positive consumed height")
	}
}

func TestMarkerPseudoElementFontSize(t *testing.T) {
	src := `<style>li::marker { font-size: 20px; }</style>
	<ul><li>Item one</li><li>Item two</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestMarkerPseudoElementColorAndSize(t *testing.T) {
	src := `<style>li::marker { color: #00ff00; font-size: 18px; }</style>
	<ol><li>First</li><li>Second</li><li>Third</li></ol>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

func TestMarkerPseudoElementNoEffect(t *testing.T) {
	// Without ::marker, list should render normally.
	src := `<ul><li>Item one</li><li>Item two</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}

// font-style: italic on ::marker resolves through the same resolveFontPair
// path used for regular text, giving the marker a distinct font resource
// (the standard-font italic fallback, since no @font-face is registered)
// from the body text's font.
func TestMarkerPseudoElementFontStyleItalic(t *testing.T) {
	src := `<style>li::marker { font-style: italic; content: "* "; }</style>
	<ul><li>Alpha</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)

	markerFont := fontResourceBefore(t, stream, "(*) Tj")
	bodyFont := fontResourceBefore(t, stream, "(Alpha) Tj")
	if markerFont == "" || bodyFont == "" {
		t.Fatalf("could not locate font resources in stream:\n%s", stream)
	}
	if markerFont == bodyFont {
		t.Errorf("expected marker font-style: italic to use a different font resource than the body text, both used %q\nstream:\n%s", markerFont, stream)
	}
}

// An unrecognized font-style value (anything but "italic") is a no-op: no
// panic, and the marker keeps rendering with the list's default font.
func TestMarkerPseudoElementFontStyleUnrecognizedIsNoop(t *testing.T) {
	src := `<style>li::marker { font-style: oblique; content: "* "; }</style>
	<ul><li>Alpha</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}

	stream := renderStreamText(t, elems)
	markerFont := fontResourceBefore(t, stream, "(*) Tj")
	bodyFont := fontResourceBefore(t, stream, "(Alpha) Tj")
	if markerFont != bodyFont {
		t.Errorf("unrecognized font-style should be a no-op (marker keeps the list's default font), got marker=%q body=%q\nstream:\n%s", markerFont, bodyFont, stream)
	}
}

// fontResourceBefore returns the PDF font resource name (e.g. "/F1") set by
// the nearest preceding "Tf" operator before the given Tj text token.
func fontResourceBefore(t *testing.T, stream, tjToken string) string {
	t.Helper()
	idx := strings.Index(stream, tjToken)
	if idx < 0 {
		t.Fatalf("token %q not found in stream", tjToken)
	}
	prefix := stream[:idx]
	tfIdx := strings.LastIndex(prefix, "Tf")
	if tfIdx < 0 {
		return ""
	}
	lineStart := strings.LastIndex(prefix[:tfIdx], "\n") + 1
	fields := strings.Fields(prefix[lineStart:tfIdx])
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func TestMarkerPseudoElementTextUnaffected(t *testing.T) {
	// ::marker styling should not affect text color of items.
	src := `<style>li::marker { color: blue; } li { color: black; }</style>
	<ul><li>Item text should be black</li></ul>`
	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	plan := elems[0].PlanLayout(layout.LayoutArea{Width: 400, Height: 1000})
	if plan.Status != layout.LayoutFull {
		t.Errorf("expected LayoutFull, got %v", plan.Status)
	}
}
