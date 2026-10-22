// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"context"
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
	Anchors    []AnchorInfo     // fragment ids found on this page (for named destinations)
	PageHeight float64          // actual page height (non-zero only for auto-sized pages)
}

// AnchorInfo records a fragment identifier (an element's HTML id) found
// during rendering, with the y position of its target block's top. The
// document layer registers these as PDF named destinations.
type AnchorInfo struct {
	Name string  // fragment id (the value of href="#name")
	Y    float64 // y position in PDF coordinates (top of the target block)
}

// HeadingInfo records a heading found during rendering.
//
// Despite the name, this collects every block participating in the
// outline tree — both <h1>-<h6> and any non-heading element with an
// explicit CSS bookmark-level. Closed reflects bookmark-state: closed
// and is forwarded to the PDF Outline entry's /Count sign.
type HeadingInfo struct {
	Text   string  // heading text / bookmark label
	Level  int     // 1-6 (H1-H6 or explicit bookmark-level)
	Y      float64 // y position in PDF coordinates (top of heading)
	Closed bool    // bookmark-state: closed
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
	fixed        bool    // position: fixed — draw on every page
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
	firstMarginBoxes map[string]MarginBox // first-page margin boxes (@page :first)
	leftMarginBoxes  map[string]MarginBox // left-page margin boxes (@page :left)
	rightMarginBoxes map[string]MarginBox // right-page margin boxes (@page :right)
	elements         []Element
	absolutes        []absoluteItem
	tagged           bool            // if true, emit BDC/EMC marked content
	actualText       bool            // if true, emit ISO 32000-2 §14.9.4 /ActualText for shaped Arabic
	structTags       []StructTagInfo // collected during rendering

	// Running string values for CSS string-set / string() support.
	// Maps string name → current value. Updated during pagination as
	// elements with StringSets are placed on pages.
	runningStrings map[string]string
	// Per-page snapshot of running string values at the end of each page.
	pageStrings []map[string]string

	// ctx bounds the layout pass when set via RenderContext. It is stored
	// on this short-lived, per-render struct (never shared or persisted) so
	// the pagination loops can check it at page and element boundaries. nil
	// means no cancellation. ctxErr records the first ctx.Err() seen and
	// aborts the remaining layout; RenderContext returns it.
	ctx    context.Context
	ctxErr error
}

// marginBoxOrder fixes the drawing order of margin boxes so content-stream
// bytes and font resource names are deterministic (map iteration order
// must never reach an output path).
var marginBoxOrder = [...]string{
	"top-left", "top-center", "top-right",
	"bottom-left", "bottom-center", "bottom-right",
}

// MarginBox holds the content template for a CSS margin box.
// Content may contain placeholders like {counter(page)} and {counter(pages)}.
type MarginBox struct {
	Content  string     // template string with placeholders
	FontSize float64    // font size in points (0 = default 9pt)
	Color    [3]float64 // RGB color (0-1 each)
	// HasColor is true only when the source CSS explicitly set a `color`
	// declaration. It distinguishes an explicit `color: black` ({0,0,0})
	// from an unset color: when false the renderer applies the default
	// gray; when true it honours Color verbatim (including pure black).
	HasColor bool
	// Font is the standard PDF font to draw with when Embedded is nil,
	// resolved from the margin box's own font-style/font-weight/
	// font-family declarations. nil falls back to plain Helvetica — the
	// pre-fix behavior for a box with no font declarations of its own.
	Font *font.Standard
	// Embedded is the document's default body font, used to draw the
	// margin-box text. When non-nil the renderer emits the text with this
	// embedded font (Identity-H GID encoding), so the glyphs are subset
	// and embedded — required for PDF/A. When nil the renderer falls back
	// to Font, or plain Helvetica if Font is also nil.
	Embedded *font.EmbeddedFont
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

// SetLeftMarginBoxes sets margin boxes for left (even-numbered) pages (@page :left).
func (r *Renderer) SetLeftMarginBoxes(boxes map[string]MarginBox) {
	r.leftMarginBoxes = boxes
}

// SetRightMarginBoxes sets margin boxes for right (odd-numbered) pages (@page :right).
func (r *Renderer) SetRightMarginBoxes(boxes map[string]MarginBox) {
	r.rightMarginBoxes = boxes
}

// marginsForPage returns the margins to use for the given page index (0-based).
//
// Page parity follows CSS paged media for LEFT-TO-RIGHT documents: page 0 (the
// first page, page number 1) is a :right page, page 1 (page number 2) is a
// :left page, then alternating. Right-to-left progression is not modelled.
//
// Precedence (highest first): :first (page 0 only) > :left/:right by parity >
// default. :first wins over :right on page 0 because CSS gives :first higher
// specificity. Each pseudo set already inherits the base @page {} margins for
// sides it does not specify (the cascade merge happens in html/page.go), so the
// returned Margins are fully resolved.
func (r *Renderer) marginsForPage(pageIdx int) Margins {
	if pageIdx == 0 && r.firstMargins != nil {
		return *r.firstMargins
	}
	if pageIdx%2 == 0 && r.rightMargins != nil {
		// Even index = right page (odd page number: 1, 3, 5, ...).
		return *r.rightMargins
	}
	if pageIdx%2 == 1 && r.leftMargins != nil {
		// Odd index = left page (even page number: 2, 4, 6, ...).
		return *r.leftMargins
	}
	return r.margins
}

// marginBoxesForPage selects the margin-box set for the given page index,
// merged over the default set. Parity and precedence mirror marginsForPage:
// :first (page 0) > :left/:right by parity > default. A parity-specific or
// first-page box overrides the same-named default box; sides absent from the
// parity set fall back to the default. LTR progression is assumed (see
// marginsForPage).
func (r *Renderer) marginBoxesForPage(pageIdx int) map[string]MarginBox {
	var override map[string]MarginBox
	switch {
	case pageIdx == 0 && r.firstMarginBoxes != nil:
		override = r.firstMarginBoxes
	case pageIdx%2 == 0 && r.rightMarginBoxes != nil:
		override = r.rightMarginBoxes
	case pageIdx%2 == 1 && r.leftMarginBoxes != nil:
		override = r.leftMarginBoxes
	}
	if override == nil {
		return r.marginBoxes
	}
	merged := make(map[string]MarginBox, len(r.marginBoxes)+len(override))
	for k, v := range r.marginBoxes {
		merged[k] = v
	}
	for k, v := range override {
		merged[k] = v
	}
	return merged
}

// NewRenderer creates a renderer for the given page dimensions and margins.
// /ActualText emission for shaped Arabic words is enabled by default; call
// SetActualText(false) to opt out (e.g. to minimise content stream size).
func NewRenderer(pageWidth, pageHeight float64, margins Margins) *Renderer {
	return &Renderer{
		pageWidth:  pageWidth,
		pageHeight: pageHeight,
		margins:    margins,
		actualText: true,
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
	// Select margin boxes for this page (parity-aware, merged over defaults).
	boxes := r.marginBoxesForPage(pageIdx)
	if len(boxes) == 0 {
		return
	}

	contentWidth := r.pageWidth - margins.Left - margins.Right

	for _, name := range marginBoxOrder {
		box, ok := boxes[name]
		if !ok {
			continue
		}
		text := box.Content
		if text == "" {
			continue
		}
		// Resolve {counter(page)} / {counter(pages)} placeholders,
		// including styled variants like {counter(page,upper-roman)}.
		// pageIdx is 0-based; counter(page) is 1-based per CSS GCPM.
		// counter(pages) uses ctx.TotalPages, set during the emission
		// pass once pagination is final.
		text = substituteCounters(text, pageIdx, ctx.TotalPages)

		// Resolve {string(name)} placeholders from CSS string-set.
		text = r.resolveStringRefs(text, pageIdx)

		// Use box-specific font size, or default to 9pt.
		fontSize := box.FontSize
		if fontSize <= 0 {
			fontSize = 9.0
		}

		// Use box-specific color when the CSS explicitly set one (HasColor),
		// otherwise default to gray. The HasColor flag is required so an
		// explicit `color: black` ({0,0,0}) is honoured rather than being
		// mistaken for "unset" and forced to gray.
		textColor := Color{R: 0.4, G: 0.4, B: 0.4}
		if box.HasColor {
			textColor = Color{R: box.Color[0], G: box.Color[1], B: box.Color[2]}
		}

		// Choose the drawing word: an embedded font (PDF/A-safe, Identity-H
		// GID encoding) when one was stamped on the box, otherwise the
		// box's own resolved standard font, falling back to plain
		// Helvetica. Width is measured with the same font so alignment
		// math is consistent.
		word := Word{FontSize: fontSize, OriginalText: text, Text: text}
		var textWidth float64
		if box.Embedded != nil {
			word.Embedded = box.Embedded
			textWidth = box.Embedded.MeasureString(text, fontSize)
		} else {
			f := box.Font
			if f == nil {
				f = font.Helvetica
			}
			word.Font = f
			textWidth = f.MeasureString(text, fontSize)
		}
		resName := registerFont(ctx.Page, word)

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

		// Running headers/footers are pagination artifacts, not document
		// content. In a tagged PDF they must be marked as /Artifact so they
		// stay out of the structure tree (PDF/UA, ISO 14289-1 §7.1).
		if r.tagged {
			ctx.Stream.BeginArtifact()
		}
		setFillColor(ctx.Stream, textColor)
		ctx.Stream.BeginText()
		ctx.Stream.SetFont(resName, fontSize)
		ctx.Stream.MoveText(x, y)
		if box.Embedded != nil {
			// Embedded fonts require Identity-H GID encoding; route through
			// the shared embedded-word drawer so glyph subsetting is recorded
			// via EncodeString. Raw ShowText is only valid for standard fonts.
			drawWordEmbedded(ctx.Stream, word)
		} else {
			ctx.Stream.ShowText(text)
		}
		ctx.Stream.EndText()
		if r.tagged {
			ctx.Stream.EndMarkedContent()
		}
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

// SetActualText controls whether the renderer emits ISO 32000-2 §14.9.4
// /Span /ActualText marked-content sequences around words whose text was
// substituted by the Arabic shaper. When enabled (the default), copy/paste
// and accessibility consumers see the original Unicode codepoints rather
// than the Presentation Forms-B substitutions. Disabling shaves a few
// dozen bytes per shaped Arabic word and is appropriate for size-sensitive
// documents that do not require text round-tripping.
func (r *Renderer) SetActualText(enabled bool) {
	r.actualText = enabled
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
	Fixed        bool // position: fixed — draw on every page
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
		pageIndex: opts.PageIndex, rightAligned: opts.RightAligned, zIndex: opts.ZIndex, fixed: opts.Fixed,
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

// RenderContext is the context-aware variant of Render. It checks ctx at
// page and element boundaries during layout and returns ctx.Err()
// (context.Canceled or context.DeadlineExceeded) if the context is done,
// with a nil page slice in that case. With a background context it behaves
// exactly like Render.
func (r *Renderer) RenderContext(ctx context.Context) ([]PageResult, error) {
	r.ctx = ctx
	pages := r.renderWithPlans()
	if r.ctxErr != nil {
		return nil, r.ctxErr
	}
	return pages, nil
}

// cancelled reports whether the bound context (if any) is done, recording
// the first error so the in-progress layout can unwind. It is nil-safe:
// with no context set it always returns false, so Render incurs no context
// overhead.
func (r *Renderer) cancelled() bool {
	if r.ctxErr != nil {
		return true
	}
	if r.ctx == nil {
		return false
	}
	if err := r.ctx.Err(); err != nil {
		r.ctxErr = err
		return true
	}
	return false
}
