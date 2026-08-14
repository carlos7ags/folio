// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// anchorMarker wraps any block-level Element so that its first
// PlacedBlock carries a fragment identifier (an element's HTML id). The
// renderer surfaces these onto PageResult.Anchors and the document layer
// registers them as PDF named destinations, so <a href="#name"> links
// resolve to the wrapped element's top position.
//
// The wrapper is a thin pass-through: layout, measurement, and overflow
// are delegated to the inner element. It only decorates the first block
// of the produced plan with the anchor name, leaving structure tags and
// bookmark metadata untouched. The name is recorded once, on the first
// block; a continuation that overflows to the next page is not
// re-tagged, so the destination points at the element's start.
type anchorMarker struct {
	inner Element
	name  string
}

// measurableAnchorMarker is the variant returned when the inner element
// implements Measurable. It exposes MinWidth/MaxWidth that delegate to
// the inner so containers that consult Measurable (flex, table-cell
// shrink-to-fit) get the inner's natural sizing instead of collapsing to
// zero.
type measurableAnchorMarker struct {
	anchorMarker
}

// NewAnchor wraps inner so that its first PlacedBlock records name as a
// named-destination target. The returned Element implements Measurable
// iff inner does, so wrapping does not change a caller's measurement
// contract. An empty name is a no-op wrap.
func NewAnchor(inner Element, name string) Element {
	a := anchorMarker{inner: inner, name: name}
	if _, ok := inner.(Measurable); ok {
		return &measurableAnchorMarker{anchorMarker: a}
	}
	return &a
}

func (a *anchorMarker) PlanLayout(area LayoutArea) LayoutPlan {
	plan := a.inner.PlanLayout(area)
	if a.name == "" || len(plan.Blocks) == 0 {
		return plan
	}
	plan.Blocks[0].AnchorName = a.name
	return plan
}

// unwrap returns the wrapped element so layout code can type-assert the
// inner element's optional interfaces (see baseElement). Without this the
// wrapper would mask interfaces like Clearable / KeepTogether / HeightSettable
// and silently disable clear, page-break-inside: avoid, and cross-axis
// stretch on any element that carries an id.
func (a *anchorMarker) unwrap() Element { return a.inner }

func (a *measurableAnchorMarker) MinWidth() float64 {
	return a.inner.(Measurable).MinWidth()
}

func (a *measurableAnchorMarker) MaxWidth() float64 {
	return a.inner.(Measurable).MaxWidth()
}
