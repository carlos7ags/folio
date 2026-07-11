// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "github.com/carlos7ags/folio/content"

// LayoutArea describes the available space for laying out an element.
type LayoutArea struct {
	Width  float64 // available width in PDF points
	Height float64 // remaining height on the current page
}

// LayoutStatus indicates how much of an element fit in the available area.
type LayoutStatus int

const (
	// LayoutFull means the element fit entirely within the area.
	LayoutFull LayoutStatus = iota

	// LayoutPartial means part of the element fit. The fitted portion
	// is in LayoutPlan.Blocks; the remainder is in LayoutPlan.Overflow.
	LayoutPartial

	// LayoutNothing means the element could not fit at all;
	// the area is too small even for the first line or row.
	LayoutNothing
)

// LayoutPlan is the immutable result of laying out an element.
// It contains positioned content blocks ready to draw, plus any
// overflow content that didn't fit in the available area.
//
// LayoutPlan is pure data — no methods, no mutation, no side effects.
// This enables caching, concurrent layout, and easy testing.
type LayoutPlan struct {
	Status   LayoutStatus  // how much fit
	Consumed float64       // total height consumed by fitted content
	Blocks   []PlacedBlock // positioned content ready to draw
	Overflow Element       // the part that didn't fit (nil if LayoutFull)
}

// PlacedBlock is a positioned piece of content within a LayoutPlan.
// It carries a Draw closure that emits PDF operators when called.
//
// PlacedBlocks form a tree: container elements (Div, Table) have
// children. Leaf elements (text lines, images) have no children.
type PlacedBlock struct {
	X      float64 // x position relative to the layout area's left edge
	Y      float64 // y offset from the top of the layout area (increases downward)
	Width  float64
	Height float64

	// Draw emits PDF content stream operators for this block.
	// Called during the rendering pass with the absolute PDF
	// coordinates (x = left edge, topY = top edge in PDF coords).
	// May be nil for structural containers that only hold children.
	Draw func(ctx DrawContext, x, topY float64)

	// PostDraw is called after all children are drawn. Used to restore
	// graphics state (e.g. after clipping or opacity changes).
	PostDraw func(ctx DrawContext, x, topY float64)

	// Tag is the PDF structure tag for accessibility (e.g. "P", "H1", "Figure").
	// Empty string means no tag (untagged content or structural wrapper).
	Tag string

	// AltText is alternative text for accessibility (used with Figure tags).
	AltText string

	// HeadingText is the plain text of a heading (for auto-bookmark generation).
	// Also used as the bookmark label for non-heading bookmark targets.
	HeadingText string

	// BookmarkLevel encodes the CSS bookmark-level override for this
	// block. 0 = use the level derived from Tag (H1-H6); -1 = explicit
	// "none" (skip from outline); 1-6 = explicit level. Non-heading
	// blocks set 1-6 to participate in the outline tree.
	BookmarkLevel int

	// BookmarkClosed corresponds to bookmark-state: closed. When true,
	// the outline subtree below this entry is collapsed by default in
	// PDF viewers (ISO 32000 §12.3.3 — emitted as a negative /Count).
	BookmarkClosed bool

	// AnchorName is the fragment identifier (an element's HTML id) that
	// resolves to this block's top position. The renderer surfaces it on
	// PageResult.Anchors and the document layer registers it as a PDF
	// named destination so <a href="#name"> links navigate here.
	AnchorName string

	// Children are nested content blocks (e.g. lines within a paragraph,
	// cells within a table row). The renderer draws them in order.
	Children []PlacedBlock

	// Links carries link annotations for this block. Each entry
	// describes a clickable region. A single line of text may contain
	// multiple links with different URIs.
	Links []LinkArea

	// StringSets holds CSS string-set values captured from this block.
	// Each entry maps a string name to its content value (e.g. "chapter" → "Chapter 3").
	// Used by running headers: string-set on an element captures text that
	// string() in margin boxes can reference.
	StringSets map[string]string

	// floatInfo carries float positioning data (nil if not a float).
	floatInfo *floatBlockInfo
}

// DrawContext provides the rendering target for PlacedBlock.Draw closures.
type DrawContext struct {
	Stream *content.Stream
	Page   *PageResult
	// ActualText, when true, allows drawTextLine to emit ISO 32000-2
	// §14.9.4 /Span /ActualText marked-content sequences around shaped
	// Arabic words so that copy/paste and accessibility tools recover the
	// original Unicode codepoints rather than the Presentation Forms-B
	// substitutions emitted by ShapeArabic. The Renderer sets this from
	// its actualText field; tests that build a DrawContext directly may
	// leave it false to suppress emission.
	ActualText bool
	// PageIdx is the 0-based index of the page this draw call targets.
	// Used to substitute CSS counter(page) placeholders in body-flow
	// text. Tests building a DrawContext for non-paged rendering may
	// leave this zero.
	PageIdx int
	// TotalPages is the final page count of the document, set during the
	// emission pass after pagination completes. Used to substitute
	// counter(pages) placeholders. Zero indicates the count is not yet
	// known; placeholders remain in the emitted text in that case.
	TotalPages int
}

// measureConsumed returns the height an element consumes at the given width.
func measureConsumed(e Element, width float64) float64 {
	plan := e.PlanLayout(LayoutArea{Width: width, Height: 1e9})
	return plan.Consumed
}

// measureNaturalWidth returns the natural width of an element by laying it
// out with generous width and finding the widest block.
func measureNaturalWidth(e Element, hintWidth float64) float64 {
	if m, ok := e.(Measurable); ok {
		return m.MaxWidth()
	}
	plan := e.PlanLayout(LayoutArea{Width: hintWidth, Height: 1e9})
	return maxBlockWidth(plan.Blocks)
}

// maxBlockWidth returns the maximum right edge (X + Width) across blocks.
func maxBlockWidth(blocks []PlacedBlock) float64 {
	w := 0.0
	for _, b := range blocks {
		if bw := b.X + b.Width; bw > w {
			w = bw
		}
	}
	return w
}

// Clearable is implemented by elements that support the CSS clear property.
type Clearable interface {
	ClearValue() string // "left", "right", "both", or ""
}

// elementWrapper is implemented by decorator elements (the anchor and
// bookmark markers) that wrap an inner element but cannot reproduce its
// optional layout interfaces (Clearable, HeightSettable, KeepTogether,
// layoutable, ...). Layout code that type-asserts an optional interface must
// do so against baseElement(e) so a decorated element still participates in
// clear, keep-together, cross-axis stretch, column layout, and the like.
type elementWrapper interface {
	unwrap() Element
}

// baseElement returns the innermost element, seeing through any decorator
// wrappers, so an optional-interface type assertion matches the real element
// rather than the wrapper (which would otherwise silently mask it).
func baseElement(e Element) Element {
	for {
		w, ok := e.(elementWrapper)
		if !ok {
			return e
		}
		e = w.unwrap()
	}
}

// HeightSettable is implemented by elements that can have their height
// forced before layout (used by flex cross-axis stretching).
type HeightSettable interface {
	ForceHeight(u UnitValue)
	ClearHeightUnit()
	HasExplicitHeight() bool // true if element has a CSS height set
}

// Measurable is an optional interface that elements can implement
// to report their intrinsic width constraints. This enables features
// like auto-sizing table columns based on cell content.
type Measurable interface {
	// MinWidth returns the narrowest width this element can be rendered
	// at without clipping or overflow. For text, this is typically the
	// width of the longest word. For images, the minimum display width.
	MinWidth() float64

	// MaxWidth returns the natural width this element wants if
	// unconstrained. For text, this is the width of the entire text
	// on a single line. For images, the natural pixel width.
	MaxWidth() float64
}
