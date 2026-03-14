// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
)

// Margins defines the page margins in PDF points.
type Margins struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// PageResult holds the content stream and font/image resources for one rendered page.
type PageResult struct {
	Stream     *content.Stream
	Fonts      []FontEntry
	Images     []ImageEntry
	Links      []LinkArea       // clickable link annotations produced by Link elements
	ExtGStates []ExtGStateEntry // graphics state dictionaries (opacity, etc.)
	Headings   []HeadingInfo    // headings found on this page (for auto-bookmarks)
}

// HeadingInfo records a heading found during rendering.
type HeadingInfo struct {
	Text  string  // heading text
	Level int     // 1-6 (H1-H6)
	Y     float64 // y position in PDF coordinates (top of heading)
}

// ExtGStateEntry is a named graphics state dictionary registered on a page.
type ExtGStateEntry struct {
	Name    string  // resource name (e.g. "GS1")
	Opacity float64 // ca / CA value (0..1)
}

// LinkArea describes a clickable region on a rendered page.
type LinkArea struct {
	X, Y, W, H float64 // bounding box in PDF points (bottom-left origin)
	URI        string  // external URL (empty if internal link)
	DestName   string  // internal named destination (empty if external)
}

// ImageEntry is an image registered on a rendered page.
type ImageEntry struct {
	Name  string
	Image *folioimage.Image
}

// FontEntry is a font registered on a rendered page.
type FontEntry struct {
	Name     string
	Standard *font.Standard
	Embedded *font.EmbeddedFont
}

// absoluteItem is an element placed at fixed coordinates, outside normal flow.
type absoluteItem struct {
	elem      Element
	x, y      float64
	width     float64 // layout width; 0 means use full page content width
	pageIndex int     // -1 means "current page at time of rendering"
}

// StructTagInfo records a structure tag emitted during rendering.
// The Document layer uses these to build the PDF structure tree.
type StructTagInfo struct {
	Tag         string // structure type (e.g. "P", "H1", "Table")
	MCID        int    // marked content ID on this page
	PageIndex   int    // which page this tag is on
	AltText     string // alternative text (for Figure tags)
	ParentIndex int    // index of parent tag in the StructTags slice (-1 = top-level)
}

// Renderer lays out a sequence of elements into pages,
// handling page breaks automatically.
type Renderer struct {
	pageWidth  float64
	pageHeight float64
	margins    Margins
	elements   []Element
	absolutes  []absoluteItem
	tagged     bool            // if true, emit BDC/EMC marked content
	structTags []StructTagInfo // collected during rendering
	mcidCount  []int           // per-page MCID counter
}

// NewRenderer creates a renderer for the given page dimensions and margins.
func NewRenderer(pageWidth, pageHeight float64, margins Margins) *Renderer {
	return &Renderer{
		pageWidth:  pageWidth,
		pageHeight: pageHeight,
		margins:    margins,
	}
}

// SetTagged enables tagged PDF output. When true, the renderer wraps
// content in BDC/EMC marked content sequences and collects StructTagInfo
// for the document layer to build the structure tree.
func (r *Renderer) SetTagged(enabled bool) {
	r.tagged = enabled
}

// StructTags returns the structure tags collected during rendering.
func (r *Renderer) StructTags() []StructTagInfo {
	return r.structTags
}

// allocMCID allocates the next MCID for the given page index.
func (r *Renderer) allocMCID(pageIndex int) int {
	for len(r.mcidCount) <= pageIndex {
		r.mcidCount = append(r.mcidCount, 0)
	}
	mcid := r.mcidCount[pageIndex]
	r.mcidCount[pageIndex]++
	return mcid
}

// tagLine assigns a structure tag and MCID to a line if tagging is enabled.
func (r *Renderer) tagLine(line *Line, tag string, pageIndex int) {
	if !r.tagged || tag == "" {
		return
	}
	mcid := r.allocMCID(pageIndex)
	line.Tagged = true
	line.StructTag = tag
	line.MCID = mcid
	r.structTags = append(r.structTags, StructTagInfo{
		Tag:         tag,
		MCID:        mcid,
		PageIndex:   pageIndex,
		ParentIndex: -1, // top-level (line-based path doesn't support nesting)
	})
}

// Add appends an element to the layout queue.
func (r *Renderer) Add(e Element) {
	r.elements = append(r.elements, e)
}

// AddAbsolute places an element at the given (x, y) coordinates on the
// last page produced by the normal flow. The element does not participate
// in normal vertical stacking — it is rendered on top of flow content.
// Coordinates are in PDF points from the bottom-left corner of the page.
// Width sets the layout width for the element (e.g. for word-wrapping);
// pass 0 to use the full page content width.
func (r *Renderer) AddAbsolute(e Element, x, y, width float64) {
	r.absolutes = append(r.absolutes, absoluteItem{
		elem: e, x: x, y: y, width: width, pageIndex: -1,
	})
}

// AddAbsoluteOnPage places an element at (x, y) on a specific page
// (0-indexed). If the page index exceeds the number of pages produced
// by normal flow, the element is silently ignored.
func (r *Renderer) AddAbsoluteOnPage(e Element, x, y, width float64, pageIndex int) {
	r.absolutes = append(r.absolutes, absoluteItem{
		elem: e, x: x, y: y, width: width, pageIndex: pageIndex,
	})
}

// Render lays out elements into pages. Each Element provides a PlanLayout
// method for height-aware layout with content splitting across pages.
func (r *Renderer) Render() []PageResult {
	return r.renderWithPlans()
}
