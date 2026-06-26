// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"math"

	"github.com/carlos7ags/folio/content"
)

// renderWithPlans lays out elements into pages using PlanLayout.
// Each Element computes a height-aware LayoutPlan that supports
// content splitting across pages via Overflow.
func (r *Renderer) renderWithPlans() []PageResult {
	autoHeight := r.pageHeight == 0

	// Compute margins/dimensions for the current page.
	pageMarginsFor := func(idx int) (maxW, usableH float64, m Margins) {
		m = r.marginsForPage(idx)
		maxW = r.pageWidth - m.Left - m.Right
		usableH = r.pageHeight - m.Top - m.Bottom
		if autoHeight {
			usableH = math.MaxFloat64
		}
		return
	}

	curMargins := r.marginsForPage(0)
	maxWidth := r.pageWidth - curMargins.Left - curMargins.Right
	usableHeight := r.pageHeight - curMargins.Top - curMargins.Bottom
	if autoHeight {
		usableHeight = math.MaxFloat64
	}

	var pages []PageResult

	// Build the element queue.
	queue := make([]Element, len(r.elements))
	copy(queue, r.elements)

	// Per-page record captured during pagination. Drawing is deferred to
	// a second pass so DrawContext.TotalPages can carry the final page
	// count when CSS counter(pages) placeholders are substituted in body
	// flow text. This also lets margin boxes resolve counter(pages)
	// directly without relying on a post-stream byte replacement.
	type pageRecord struct {
		blocks  []PlacedBlock
		margins Margins
		idx     int
	}
	var records []pageRecord

	var curBlocks []PlacedBlock
	remainingHeight := usableHeight
	curY := 0.0
	pageIdx := 0
	atPageTop := true

	flushPage := func() {
		// Capture string-set values from placed blocks before drawing.
		// This updates running string state used by margin box string() refs.
		r.captureStringSets(curBlocks)
		r.snapshotStrings()

		records = append(records, pageRecord{
			blocks:  curBlocks,
			margins: curMargins,
			idx:     pageIdx,
		})
	}

	startNewPage := func() {
		if len(curBlocks) > 0 {
			flushPage()
		}
		curBlocks = nil
		pageIdx++
		// Recalculate margins for the new page.
		maxWidth, usableHeight, curMargins = pageMarginsFor(pageIdx)
		remainingHeight = usableHeight
		curY = 0
		atPageTop = true
	}

	// Float tracking: active floats reduce available width for subsequent elements.
	type activeFloat struct {
		side         FloatSide
		width        float64 // width consumed by the float (including margin)
		remainHeight float64 // how much vertical space the float still occupies
	}
	var floats []activeFloat

	// effectiveWidth returns the available width accounting for active floats.
	effectiveWidth := func() (width, leftOffset float64) {
		w := maxWidth
		off := 0.0
		for _, f := range floats {
			w -= f.width
			if f.side == FloatLeft {
				off += f.width
			}
		}
		if w < 0 {
			w = 0
		}
		return w, off
	}

	// consumeFloatHeight reduces float remaining heights after content is placed.
	consumeFloatHeight := func(h float64) {
		alive := floats[:0]
		for _, f := range floats {
			f.remainHeight -= h
			if f.remainHeight > 0 {
				alive = append(alive, f)
			}
		}
		floats = alive
	}

	// Initialize first page.
	_ = pageIdx // used in flushPage closure

	for len(queue) > 0 {
		// Cancellation check at the element boundary — the pagination loop
		// is where a runaway document burns CPU, so this is what lets a
		// caller's deadline actually abort the work.
		if r.cancelled() {
			return nil
		}
		elem := queue[0]
		queue = queue[1:]

		// Handle AreaBreak — always flush and start a new page.
		//
		// Unlike startNewPage (which skips empty pages), an explicit break
		// always advances the page even when the current page has no blocks.
		// Crucially it must recompute margins/dimensions for the new page via
		// pageMarginsFor(pageIdx): the per-page margin selection (@page :first
		// / :left / :right) keys off the cumulative pageIdx, so without this
		// recalc every break-delimited page would keep page 0's :first margins.
		if _, ok := elem.(*AreaBreak); ok {
			flushPage()
			curBlocks = nil
			pageIdx++
			maxWidth, usableHeight, curMargins = pageMarginsFor(pageIdx)
			remainingHeight = usableHeight
			curY = 0
			floats = nil
			atPageTop = true
			continue
		}

		// CSS clear: advance past active floats before placing this element.
		if cl, ok := elem.(Clearable); ok {
			cv := cl.ClearValue()
			if cv == "left" || cv == "right" || cv == "both" {
				maxRemain := 0.0
				for _, f := range floats {
					if cv == "both" || (cv == "left" && f.side == FloatLeft) || (cv == "right" && f.side == FloatRight) {
						if f.remainHeight > maxRemain {
							maxRemain = f.remainHeight
						}
					}
				}
				if maxRemain > 0 {
					curY += maxRemain
					remainingHeight -= maxRemain
					consumeFloatHeight(maxRemain)
				}
			}
		}

		availWidth, leftOffset := effectiveWidth()

		// A float resolves its width against the containing block, not the
		// space remaining beside sibling floats (CSS 2.1 10.3.5). In-flow
		// content uses the float-reduced width.
		_, isFloatElem := elem.(*Float)

		// No room beside the active floats: drop this in-flow block below
		// them (same mechanics as clear:both). A block whose minimum content
		// width exceeds the space left by the floats cannot sit beside them.
		if !isFloatElem && len(floats) > 0 {
			if m, ok := elem.(Measurable); ok && m.MinWidth() > availWidth+0.01 {
				maxRemain := 0.0
				for _, f := range floats {
					if f.remainHeight > maxRemain {
						maxRemain = f.remainHeight
					}
				}
				if maxRemain > 0 {
					curY += maxRemain
					remainingHeight -= maxRemain
					consumeFloatHeight(maxRemain)
					availWidth, leftOffset = effectiveWidth()
				}
			}
		}

		planWidth := availWidth
		if isFloatElem {
			planWidth = maxWidth
		}
		area := LayoutArea{
			Width:  planWidth,
			Height: remainingHeight,
		}

		plan := elem.PlanLayout(area)

		// If this element is a float, place it beside any floats already
		// active on the same side so multiple floats stack instead of
		// overlapping (left floats from the left edge, right from the right).
		isFloat := false
		for bi := range plan.Blocks {
			b := &plan.Blocks[bi]
			if b.floatInfo == nil {
				continue
			}
			isFloat = true
			var sameSide float64
			for _, f := range floats {
				if f.side == b.floatInfo.side {
					sameSide += f.width
				}
			}
			if b.floatInfo.side == FloatLeft {
				b.X += sameSide
			} else {
				b.X -= sameSide
			}
			floats = append(floats, activeFloat{
				side:         b.floatInfo.side,
				width:        b.floatInfo.floatWidth,
				remainHeight: b.floatInfo.height,
			})
		}

		// Offset in-flow blocks past active left floats.
		if leftOffset > 0 && !isFloat {
			for i := range plan.Blocks {
				plan.Blocks[i].X += leftOffset
			}
		}

		switch plan.Status {
		case LayoutFull:
			if atPageTop {
				stripLeadingOffset(&plan)
			}
			for i := range plan.Blocks {
				plan.Blocks[i].Y += curY
			}
			curBlocks = append(curBlocks, plan.Blocks...)
			curY += plan.Consumed
			remainingHeight -= plan.Consumed
			if !isFloat {
				consumeFloatHeight(plan.Consumed)
			}
			atPageTop = false

		case LayoutPartial:
			// page-break-inside: avoid — if the element wants to stay
			// together and we're not at the top of a fresh page, move
			// the whole element to the next page instead of splitting.
			if kt, ok := elem.(interface{ KeepTogether() bool }); ok && kt.KeepTogether() && !atPageTop {
				startNewPage()
				floats = nil
				queue = append([]Element{elem}, queue...)
				continue
			}

			if atPageTop {
				stripLeadingOffset(&plan)
			}
			for i := range plan.Blocks {
				plan.Blocks[i].Y += curY
			}
			curBlocks = append(curBlocks, plan.Blocks...)

			startNewPage()
			floats = nil
			if plan.Overflow != nil {
				queue = append([]Element{plan.Overflow}, queue...)
			}

		case LayoutNothing:
			if !atPageTop {
				startNewPage()
				floats = nil
				queue = append([]Element{elem}, queue...)
			} else {
				forcePlan := elem.PlanLayout(LayoutArea{Width: availWidth, Height: 1e9})
				for i := range forcePlan.Blocks {
					forcePlan.Blocks[i].Y += curY
				}
				curBlocks = append(curBlocks, forcePlan.Blocks...)
				curY += forcePlan.Consumed
				remainingHeight = 0
				atPageTop = false
				if forcePlan.Overflow != nil {
					queue = append([]Element{forcePlan.Overflow}, queue...)
				}
			}
		}
	}

	// For auto-height pages, compute the actual page height from content.
	// Auto-height only ever produces page 0, so use the margins resolved for
	// that page (which may come from @page :first) — matching the margins used
	// to position the content — rather than the default margin set.
	if autoHeight && len(curBlocks) > 0 {
		m0 := r.marginsForPage(0)
		r.pageHeight = curY + m0.Top + m0.Bottom
	}

	// Flush the last page.
	if len(curBlocks) > 0 {
		flushPage()
	} else if len(records) == 0 {
		// Ensure at least one page.
		if autoHeight {
			m0 := r.marginsForPage(0)
			r.pageHeight = m0.Top + m0.Bottom
		}
		records = append(records, pageRecord{idx: 0, margins: r.marginsForPage(0)})
	}

	// Emission pass: now that pagination is final, draw each page with
	// the resolved total page count available for counter(pages)
	// substitution.
	totalPages := len(records)
	for _, rec := range records {
		// Cancellation check at the page boundary during the emission pass.
		if r.cancelled() {
			return nil
		}
		stream := content.NewStream()
		page := &PageResult{Stream: stream}
		ctx := DrawContext{
			Stream:     stream,
			Page:       page,
			ActualText: r.actualText,
			PageIdx:    rec.idx,
			TotalPages: totalPages,
		}
		for _, block := range rec.blocks {
			drawBlock(block, rec.margins.Left, r.pageHeight-rec.margins.Top, &ctx, r.tagged, &r.structTags, rec.idx)
		}
		r.drawMarginBoxes(&ctx, rec.idx, rec.margins)
		pages = append(pages, PageResult{
			Stream:     stream,
			Fonts:      page.Fonts,
			Images:     page.Images,
			Links:      page.Links,
			ExtGStates: page.ExtGStates,
			Headings:   page.Headings,
		})
	}

	// Tag auto-height pages with their computed height.
	if autoHeight {
		for i := range pages {
			pages[i].PageHeight = r.pageHeight
		}
	}

	// Render absolutely positioned elements.
	r.renderAbsolutes(pages, maxWidth, totalPages)

	return pages
}

// stripLeadingOffset normalizes a plan that begins with leading vertical
// whitespace (e.g. a heading's space-above, or a paragraph's space-before)
// when the plan is being placed at the top of a fresh page. The first
// block's Y is treated as the leading offset and subtracted from every
// block, and from the plan's consumed height, so the element snaps flush
// to the top margin without collapsing the spacing between its own
// internal blocks (which would otherwise overlap or gap by the offset).
func stripLeadingOffset(plan *LayoutPlan) {
	if len(plan.Blocks) == 0 {
		return
	}
	offset := plan.Blocks[0].Y
	if offset <= 0 {
		return
	}
	for i := range plan.Blocks {
		plan.Blocks[i].Y -= offset
	}
	plan.Consumed -= offset
}

// drawBlock recursively draws a PlacedBlock and its children into the stream.
// baseX and topY define the coordinate origin for the block's position.
func drawBlock(block PlacedBlock, baseX, topY float64, ctx *DrawContext, tagged bool, tags *[]StructTagInfo, pageIdx int) {
	drawBlockNested(block, baseX, topY, ctx, tagged, tags, pageIdx, -1)
}

// drawBlockNested recursively draws a PlacedBlock and its children, tracking parent for nesting.
func drawBlockNested(block PlacedBlock, baseX, topY float64, ctx *DrawContext, tagged bool, tags *[]StructTagInfo, pageIdx int, parentIdx int) {
	// Compute PDF coordinates.
	pdfX := baseX + block.X
	pdfY := topY - block.Y

	// Emit marked content for tagged PDF.
	myIdx := -1
	if tagged && block.Tag != "" {
		mcid := len(*tags)
		myIdx = mcid
		ctx.Stream.BeginMarkedContentWithID(block.Tag, mcid)
		*tags = append(*tags, StructTagInfo{
			Tag:         block.Tag,
			MCID:        mcid,
			PageIndex:   pageIdx,
			AltText:     block.AltText,
			ParentIndex: parentIdx,
		})
		defer ctx.Stream.EndMarkedContent()
	}

	// Draw this block's content.
	if block.Draw != nil {
		block.Draw(*ctx, pdfX, pdfY)
	}

	// Record heading for auto-bookmarks. The block's BookmarkLevel takes
	// precedence over the level derived from its structure tag:
	//
	//   BookmarkLevel == -1 → CSS "bookmark-level: none" — skip.
	//   BookmarkLevel  >  0 → explicit override (or non-heading target).
	//   BookmarkLevel == 0  → fall back to the H1-H6 tag.
	if block.BookmarkLevel != -1 && block.HeadingText != "" {
		level := block.BookmarkLevel
		if level == 0 {
			level = headingLevel(block.Tag)
		}
		if level > 0 {
			ctx.Page.Headings = append(ctx.Page.Headings, HeadingInfo{
				Text:   block.HeadingText,
				Level:  level,
				Y:      pdfY,
				Closed: block.BookmarkClosed,
			})
		}
	}

	// Record link annotations.
	for _, link := range block.Links {
		// Use the precise link span if available, otherwise fall back to
		// the full block dimensions.
		linkX := pdfX
		linkW := block.Width
		if link.W > 0 {
			linkX = pdfX + link.X
			linkW = link.W
		}
		ctx.Page.Links = append(ctx.Page.Links, LinkArea{
			X:        linkX,
			Y:        pdfY - block.Height,
			W:        linkW,
			H:        block.Height,
			URI:      link.URI,
			DestName: link.DestName,
		})
	}

	// Draw children with nesting — parent is either this tagged block or inherited.
	childParent := parentIdx
	if myIdx >= 0 {
		childParent = myIdx
	}
	for _, child := range block.Children {
		drawBlockNested(child, pdfX, pdfY, ctx, tagged, tags, pageIdx, childParent)
	}

	// Post-draw cleanup (restore clipping, opacity, etc.).
	if block.PostDraw != nil {
		block.PostDraw(*ctx, pdfX, pdfY)
	}
}

// headingLevel returns the heading level (1-6) for a tag like "H1", "H2".
// Returns 0 if the tag is not a heading.
func headingLevel(tag string) int {
	if len(tag) == 2 && tag[0] == 'H' && tag[1] >= '1' && tag[1] <= '6' {
		return int(tag[1] - '0')
	}
	return 0
}

// renderAbsolutes lays out and draws absolutely positioned elements
// onto the appropriate pages. Elements with negative z-index are
// prepended (rendered behind normal flow); others are appended (on top).
// totalPages carries the document's final page count so that any CSS
// counter(pages) placeholders inside absolute-positioned text resolve
// to the same value used by body and margin-box content.
func (r *Renderer) renderAbsolutes(pages []PageResult, defaultWidth float64, totalPages int) {
	lastPage := len(pages) - 1

	for _, item := range r.absolutes {
		// A fixed element (position: fixed) is drawn on every page; its
		// geometry is page-relative (resolved against page width/height) so it
		// is laid out and placed identically on each page. Non-fixed elements
		// resolve to a single page: an explicit pageIndex, or -1 ⇒ last page.
		if item.fixed {
			for pageIdx := range pages {
				r.drawAbsoluteOnPage(pages, item, pageIdx, defaultWidth, totalPages)
			}
			continue
		}

		pageIdx := item.pageIndex
		if pageIdx < 0 {
			pageIdx = lastPage
		}
		if pageIdx < 0 || pageIdx >= len(pages) {
			continue
		}
		r.drawAbsoluteOnPage(pages, item, pageIdx, defaultWidth, totalPages)
	}
}

// drawAbsoluteOnPage lays out one absolute item against the given page and
// draws it, honoring right-alignment and z-index ordering.
func (r *Renderer) drawAbsoluteOnPage(pages []PageResult, item absoluteItem, pageIdx int, defaultWidth float64, totalPages int) {
	page := &pages[pageIdx]

	layoutWidth := item.width
	if layoutWidth <= 0 {
		layoutWidth = defaultWidth
	}

	area := LayoutArea{Width: layoutWidth, Height: r.pageHeight}
	plan := item.elem.PlanLayout(area)

	x := item.x
	if item.rightAligned {
		elemWidth := 0.0
		for _, block := range plan.Blocks {
			if w := block.X + block.Width; w > elemWidth {
				elemWidth = w
			}
		}
		x = r.pageWidth - item.x - elemWidth
	}

	if item.zIndex < 0 {
		// Render into a temporary stream and prepend to draw behind flow content.
		bgStream := content.NewStream()
		bgCtx := DrawContext{
			Stream:     bgStream,
			Page:       page,
			ActualText: r.actualText,
			PageIdx:    pageIdx,
			TotalPages: totalPages,
		}
		for _, block := range plan.Blocks {
			drawBlock(block, x, item.y, &bgCtx, r.tagged, &r.structTags, pageIdx)
		}
		page.Stream.PrependBytes(bgStream.Bytes())
	} else {
		ctx := DrawContext{
			Stream:     page.Stream,
			Page:       page,
			ActualText: r.actualText,
			PageIdx:    pageIdx,
			TotalPages: totalPages,
		}
		for _, block := range plan.Blocks {
			drawBlock(block, x, item.y, &ctx, r.tagged, &r.structTags, pageIdx)
		}
	}
}
