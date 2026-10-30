// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// linkBox wraps a block-level Element so that the whole box it produces is
// a clickable link. It is the box-level counterpart of Link: Link turns a
// run of text into a link, while linkBox attaches a link annotation to an
// arbitrary element — a flex or grid container, an inline-block — that is
// itself the hyperlink.
//
// Like anchorMarker, the wrapper is a thin pass-through: layout,
// measurement and overflow are delegated to the inner element. Every
// top-level block of the produced plan gets one LinkArea with no explicit
// span, which the renderer expands to the block's full border box. When the
// inner element overflows onto another page the continuation is wrapped
// too, so each fragment of the box stays clickable.
type linkBox struct {
	inner    Element
	uri      string
	destName string
}

// measurableLinkBox is the variant returned when the inner element
// implements Measurable, so wrapping does not change how containers that
// consult MinWidth/MaxWidth size the element.
type measurableLinkBox struct {
	linkBox
}

// NewLinkBox wraps inner so that the area of every block it places is a
// link annotation. uri is an external URL; destName is a named destination
// inside the document. When both are empty, inner is returned unchanged.
// The returned Element implements Measurable iff inner does.
func NewLinkBox(inner Element, uri, destName string) Element {
	if inner == nil || (uri == "" && destName == "") {
		return inner
	}
	l := linkBox{inner: inner, uri: uri, destName: destName}
	if _, ok := inner.(Measurable); ok {
		return &measurableLinkBox{linkBox: l}
	}
	return &l
}

func (l *linkBox) PlanLayout(area LayoutArea) LayoutPlan {
	plan := l.inner.PlanLayout(area)
	// Unlike Link, the blocks keep the inner element's structure tag
	// (a flex or grid container is tagged Div). Retagging the box itself as
	// Link would drop that structure; a proper Link structure element would
	// have to wrap the box as a new parent block, which is not done here, so
	// in a tagged PDF the box is not exposed as a Link element.
	for i := range plan.Blocks {
		plan.Blocks[i].Links = append(plan.Blocks[i].Links, LinkArea{
			URI:      l.uri,
			DestName: l.destName,
		})
	}
	if plan.Overflow != nil {
		plan.Overflow = NewLinkBox(plan.Overflow, l.uri, l.destName)
	}
	return plan
}

// unwrap returns the wrapped element so layout code can type-assert the
// inner element's optional interfaces (see baseElement), exactly as for
// anchorMarker.
func (l *linkBox) unwrap() Element { return l.inner }

func (l *measurableLinkBox) MinWidth() float64 {
	return l.inner.(Measurable).MinWidth()
}

func (l *measurableLinkBox) MaxWidth() float64 {
	return l.inner.(Measurable).MaxWidth()
}
