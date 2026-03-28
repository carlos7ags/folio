// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"fmt"
	"strings"

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
	PageHeight float64          // actual page height (non-zero only for auto-sized pages)
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
	elem         Element
	x, y         float64
	width        float64 // layout width; 0 means use full page content width
	pageIndex    int     // -1 means "current page at time of rendering"
	rightAligned bool    // x is a right-edge offset; final X = pageWidth - x - elementWidth
	zIndex       int     // negative = render behind normal flow content
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
	pageWidth        float64
	pageHeight       float64
	margins          Margins
	firstMargins     *Margins             // @page :first margins (nil = use default)
	leftMargins      *Margins             // @page :left margins for even pages (nil = use default)
	rightMargins     *Margins             // @page :right margins for odd pages (nil = use default)
	marginBoxes      map[string]MarginBox // default margin boxes
	firstMarginBoxes map[string]MarginBox // first-page margin boxes
	elements         []Element
	absolutes        []absoluteItem
	tagged           bool            // if true, emit BDC/EMC marked content
	structTags       []StructTagInfo // collected during rendering

	// Running string values for CSS string-set / string() support.
	// Maps string name → current value. Updated during pagination as
	// elements with StringSets are placed on pages.
	runningStrings map[string]string
	// Per-page snapshot of running string values at the end of each page.
	pageStrings []map[string]string
}

// MarginBox holds the content template for a CSS margin box.
// Content may contain placeholders like {counter(page)} and {counter(pages)}.
type MarginBox struct {
	Content  string     // template string with placeholders
	FontSize float64    // font size in points (0 = default 9pt)
	Color    [3]float64 // RGB color (0-1 each; all zero = default gray)
}

// MarginBoxSet holds margin boxes for a page variant.
type MarginBoxSet struct {
	Boxes map[string]MarginBox // e.g. "top-center" → content
}

// SetMarginBoxes sets default margin box content.
func (r *Renderer) SetMarginBoxes(boxes map[string]MarginBox) {
	r.marginBoxes = boxes
}

// SetFirstMarginBoxes sets margin boxes for the first page only.
func (r *Renderer) SetFirstMarginBoxes(boxes map[string]MarginBox) {
	r.firstMarginBoxes = boxes
}

// marginsForPage returns the margins to use for the given page index (0-based).
// Priority: :first (page 0) > :left/:right > default.
func (r *Renderer) marginsForPage(pageIdx int) Margins {
	if pageIdx == 0 && r.firstMargins != nil {
		return *r.firstMargins
	}
	if pageIdx%2 == 0 && r.rightMargins != nil {
		// Page 0 = first right page (odd page number 1), page 2 = page number 3, etc.
		return *r.rightMargins
	}
	if pageIdx%2 == 1 && r.leftMargins != nil {
		// Page 1 = page number 2 (left/even), page 3 = page number 4, etc.
		return *r.leftMargins
	}
	return r.margins
}

// NewRenderer creates a renderer for the given page dimensions and margins.
func NewRenderer(pageWidth, pageHeight float64, margins Margins) *Renderer {
	return &Renderer{
		pageWidth:  pageWidth,
		pageHeight: pageHeight,
		margins:    margins,
	}
}

// SetFirstMargins sets margins for the first page only (@page :first).
func (r *Renderer) SetFirstMargins(m Margins) {
	r.firstMargins = &m
}

// SetLeftMargins sets margins for left (even-numbered) pages (@page :left).
func (r *Renderer) SetLeftMargins(m Margins) {
	r.leftMargins = &m
}

// SetRightMargins sets margins for right (odd-numbered) pages (@page :right).
func (r *Renderer) SetRightMargins(m Margins) {
	r.rightMargins = &m
}

// drawMarginBoxes renders margin box content (headers/footers) on the current page.
func (r *Renderer) drawMarginBoxes(ctx *DrawContext, pageIdx int, margins Margins) {
	// Select margin boxes for this page.
	boxes := r.marginBoxes
	if pageIdx == 0 && r.firstMarginBoxes != nil {
		// Merge: first-page boxes override defaults.
		merged := make(map[string]MarginBox)
		for k, v := range r.marginBoxes {
			merged[k] = v
		}
		for k, v := range r.firstMarginBoxes {
			merged[k] = v
		}
		boxes = merged
	}
	if len(boxes) == 0 {
		return
	}

	f := font.Helvetica
	contentWidth := r.pageWidth - margins.Left - margins.Right

	for name, box := range boxes {
		text := box.Content
		if text == "" {
			continue
		}
		// Resolve {counter(page)} and {counter(pages)} placeholders.
		text = strings.ReplaceAll(text, "{counter(page)}", fmt.Sprintf("%d", pageIdx+1))
		text = strings.ReplaceAll(text, "{counter(pages)}", "##TOTAL_PAGES##")

		// Resolve {string(name)} placeholders from CSS string-set.
		text = r.resolveStringRefs(text, pageIdx)

		// Use box-specific font size, or default to 9pt.
		fontSize := box.FontSize
		if fontSize <= 0 {
			fontSize = 9.0
		}

		// Use box-specific color, or default to gray.
		textColor := Color{R: 0.4, G: 0.4, B: 0.4}
		if box.Color != [3]float64{0, 0, 0} {
			textColor = Color{R: box.Color[0], G: box.Color[1], B: box.Color[2]}
		}

		resName := registerFont(ctx.Page, Word{Font: f, FontSize: fontSize})
		textWidth := f.MeasureString(text, fontSize)

		var x, y float64
		switch name {
		case "top-left":
			x = margins.Left
			y = r.pageHeight - margins.Top/2 - fontSize/2
		case "top-center":
			x = margins.Left + (contentWidth-textWidth)/2
			y = r.pageHeight - margins.Top/2 - fontSize/2
		case "top-right":
			x = r.pageWidth - margins.Right - textWidth
			y = r.pageHeight - margins.Top/2 - fontSize/2
		case "bottom-left":
			x = margins.Left
			y = margins.Bottom/2 - fontSize/2
		case "bottom-center":
			x = margins.Left + (contentWidth-textWidth)/2
			y = margins.Bottom/2 - fontSize/2
		case "bottom-right":
			x = r.pageWidth - margins.Right - textWidth
			y = margins.Bottom/2 - fontSize/2
		default:
			continue
		}

		setFillColor(ctx.Stream, textColor)
		ctx.Stream.BeginText()
		ctx.Stream.SetFont(resName, fontSize)
		ctx.Stream.MoveText(x, y)
		ctx.Stream.ShowText(text)
		ctx.Stream.EndText()
	}
}

// captureStringSets extracts string-set values from a block tree and
// updates the renderer's running string state.
func (r *Renderer) captureStringSets(blocks []PlacedBlock) {
	for _, block := range blocks {
		for name, value := range block.StringSets {
			if r.runningStrings == nil {
				r.runningStrings = make(map[string]string)
			}
			r.runningStrings[name] = value
		}
		if len(block.Children) > 0 {
			r.captureStringSets(block.Children)
		}
	}
}

// snapshotStrings saves the current running string state for a page.
func (r *Renderer) snapshotStrings() {
	snapshot := make(map[string]string, len(r.runningStrings))
	for k, v := range r.runningStrings {
		snapshot[k] = v
	}
	r.pageStrings = append(r.pageStrings, snapshot)
}

// resolveStringRefs replaces {string(name)} placeholders in text with
// the running string value for the given page.
func (r *Renderer) resolveStringRefs(text string, pageIdx int) string {
	// Find all {string(...)} occurrences.
	for {
		start := strings.Index(text, "{string(")
		if start < 0 {
			break
		}
		end := strings.Index(text[start:], ")}")
		if end < 0 {
			break
		}
		end += start + 2 // include ")}"

		// Extract the string name.
		nameStart := start + len("{string(")
		nameEnd := end - 2 // before ")}"
		name := strings.TrimSpace(text[nameStart:nameEnd])

		// Look up the value for this page.
		value := ""
		if pageIdx < len(r.pageStrings) {
			value = r.pageStrings[pageIdx][name]
		}

		text = text[:start] + value + text[end:]
	}
	return text
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

// Add appends an element to the layout queue.
func (r *Renderer) Add(e Element) {
	r.elements = append(r.elements, e)
}

// AbsoluteOpts configures an absolutely positioned element.
type AbsoluteOpts struct {
	RightAligned bool // x is a right-edge offset
	ZIndex       int  // negative = render behind normal flow
	PageIndex    int  // -1 = last page
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

// AddAbsoluteWithOpts places an element with full positioning control.
func (r *Renderer) AddAbsoluteWithOpts(e Element, x, y, width float64, opts AbsoluteOpts) {
	r.absolutes = append(r.absolutes, absoluteItem{
		elem: e, x: x, y: y, width: width,
		pageIndex: opts.PageIndex, rightAligned: opts.RightAligned, zIndex: opts.ZIndex,
	})
}

// AddAbsoluteRight places an element whose right edge is offset from the
// right page edge. The final X is computed after layout: pageWidth - rightOffset - elementWidth.
func (r *Renderer) AddAbsoluteRight(e Element, rightOffset, y, width float64) {
	r.absolutes = append(r.absolutes, absoluteItem{
		elem: e, x: rightOffset, y: y, width: width, pageIndex: -1, rightAligned: true,
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
