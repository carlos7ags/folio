// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// bookmarkAnchor wraps any block-level Element so that its first
// PlacedBlock carries CSS bookmark-* metadata for outline emission.
// Headings already attach this metadata themselves; bookmarkAnchor is
// the path for non-heading targets such as <figure> or <div> with an
// explicit bookmark-level rule.
//
// The wrapper is a thin pass-through: layout, measurement, and overflow
// are delegated to the inner element. It only decorates the first block
// of the produced plan with the bookmark fields, leaving the structure
// tag (Figure, Div, ...) untouched. That preserves accessibility
// semantics — a figure-as-bookmark stays a figure in the structure
// tree.
type bookmarkAnchor struct {
	inner  Element
	level  int // CSS bookmark-level: >= 1, or -1 for none/skip
	label  string
	closed bool
}

// measurableBookmarkAnchor is the variant returned when the inner
// element implements Measurable. It exposes MinWidth/MaxWidth that
// delegate to the inner so containers that consult Measurable (flex,
// table-cell shrink-to-fit) get the inner's natural sizing instead of
// collapsing to zero.
type measurableBookmarkAnchor struct {
	bookmarkAnchor
}

// NewBookmarkAnchor wraps inner so that its first PlacedBlock records
// the given bookmark level/label/closed state. The returned Element
// implements Measurable iff inner does, so wrapping does not change a
// caller's measurement contract.
//
// An empty label is allowed when level == -1 (a "skip" anchor that
// suppresses outline emission on a continuation page); a level >= 1
// with an empty label still records BookmarkLevel and BookmarkClosed
// so render_plans can decide what to do, but won't write HeadingText
// — render_plans drops empty-text entries, so the outline node is
// effectively suppressed without losing the closed/level intent for
// downstream debugging.
func NewBookmarkAnchor(inner Element, level int, label string, closed bool) Element {
	a := bookmarkAnchor{
		inner:  inner,
		level:  level,
		label:  normalizeText(label),
		closed: closed,
	}
	if _, ok := inner.(Measurable); ok {
		return &measurableBookmarkAnchor{bookmarkAnchor: a}
	}
	return &a
}

func (b *bookmarkAnchor) PlanLayout(area LayoutArea) LayoutPlan {
	plan := b.inner.PlanLayout(area)
	if len(plan.Blocks) == 0 {
		return plan
	}
	if b.level >= 1 || b.level == -1 {
		plan.Blocks[0].BookmarkLevel = b.level
		plan.Blocks[0].BookmarkClosed = b.closed
		if b.label != "" {
			plan.Blocks[0].HeadingText = b.label
		}
	}
	if plan.Status == LayoutPartial && plan.Overflow != nil {
		plan.Overflow = wrapBookmarkOverflow(plan.Overflow)
	}
	return plan
}

// wrapBookmarkOverflow wraps a continuation Element with a "skip"
// anchor so the second piece does not re-emit the bookmark on the next
// page. It preserves Measurable conformance via NewBookmarkAnchor.
func wrapBookmarkOverflow(inner Element) Element {
	return NewBookmarkAnchor(inner, -1, "", false)
}

// unwrap returns the wrapped element so layout code can type-assert the
// inner element's optional interfaces (see baseElement), rather than having
// the bookmark wrapper mask Clearable / KeepTogether / HeightSettable and the
// like on a decorated target.
func (b *bookmarkAnchor) unwrap() Element { return b.inner }

func (b *measurableBookmarkAnchor) MinWidth() float64 {
	return b.inner.(Measurable).MinWidth()
}

func (b *measurableBookmarkAnchor) MaxWidth() float64 {
	return b.inner.(Measurable).MaxWidth()
}
