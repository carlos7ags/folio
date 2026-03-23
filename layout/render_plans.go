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

	var curBlocks []PlacedBlock
	curPageStream := content.NewStream()
	curFonts := []FontEntry{}
	curImages := []ImageEntry{}
	curLinks := []LinkArea{}
	remainingHeight := usableHeight
	curY := 0.0
	pageIdx := 0
	atPageTop := true

	flushPage := func() {
		// Draw all placed blocks into the content stream.
		ctx := DrawContext{
			Stream: curPageStream,
			Page: &PageResult{
				Stream: curPageStream,
				Fonts:  curFonts,
				Images: curImages,
				Links:  curLinks,
			},
		}
		for _, block := range curBlocks {
			drawBlock(block, curMargins.Left, r.pageHeight-curMargins.Top, &ctx, r.tagged, &r.structTags, pageIdx)
		}

		// Draw margin boxes (headers/footers from @page CSS).
		totalPages := len(queue) // approximate; will be corrected in second pass
		_ = totalPages
		r.drawMarginBoxes(&ctx, pageIdx, curMargins)

		pages = append(pages, PageResult{
			Stream:     curPageStream,
			Fonts:      ctx.Page.Fonts,
			Images:     ctx.Page.Images,
			Links:      ctx.Page.Links,
			ExtGStates: ctx.Page.ExtGStates,
			Headings:   ctx.Page.Headings,
		})
	}

	startNewPage := func() {
		if len(curBlocks) > 0 || curPageStream.Bytes() != nil {
			flushPage()
		}
		curBlocks = nil
		curPageStream = content.NewStream()
		curFonts = nil
		curImages = nil
		curLinks = nil
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
		elem := queue[0]
		queue = queue[1:]

		// Handle AreaBreak — always flush and start a new page.
		if _, ok := elem.(*AreaBreak); ok {
			flushPage()
			curBlocks = nil
			curPageStream = content.NewStream()
			curFonts = nil
			curImages = nil
			curLinks = nil
			remainingHeight = usableHeight
			curY = 0
			floats = nil
			pageIdx++
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
		area := LayoutArea{
			Width:  availWidth,
			Height: remainingHeight,
		}

		plan := elem.PlanLayout(area)

		// Check if this element is a float.
		isFloat := false
		for _, b := range plan.Blocks {
			if b.floatInfo != nil {
				isFloat = true
				floats = append(floats, activeFloat{
					side:         b.floatInfo.side,
					width:        b.floatInfo.floatWidth,
					remainHeight: b.floatInfo.height,
				})
			}
		}

		// Offset blocks by float left margin.
		if leftOffset > 0 && !isFloat {
			for i := range plan.Blocks {
				plan.Blocks[i].X += leftOffset
			}
		}

		switch plan.Status {
		case LayoutFull:
			if atPageTop && len(plan.Blocks) > 0 {
				plan.Blocks[0].Y = 0
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
			if atPageTop && len(plan.Blocks) > 0 {
				plan.Blocks[0].Y = 0
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
	if autoHeight && len(curBlocks) > 0 {
		r.pageHeight = curY + r.margins.Top + r.margins.Bottom
	}

	// Flush the last page.
	if len(curBlocks) > 0 {
		flushPage()
	} else if len(pages) == 0 {
		// Ensure at least one page.
		if autoHeight {
			r.pageHeight = r.margins.Top + r.margins.Bottom
		}
		pages = append(pages, PageResult{Stream: content.NewStream()})
	}

	// Tag auto-height pages with their computed height.
	if autoHeight {
		for i := range pages {
			pages[i].PageHeight = r.pageHeight
		}
	}

	// Render absolutely positioned elements.
	r.renderAbsolutes(pages, maxWidth)

	return pages
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

	// Record heading for auto-bookmarks.
	if level := headingLevel(block.Tag); level > 0 && block.HeadingText != "" {
		ctx.Page.Headings = append(ctx.Page.Headings, HeadingInfo{
			Text:  block.HeadingText,
			Level: level,
			Y:     pdfY,
		})
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
func (r *Renderer) renderAbsolutes(pages []PageResult, defaultWidth float64) {
	lastPage := len(pages) - 1

	for _, item := range r.absolutes {
		pageIdx := item.pageIndex
		if pageIdx < 0 {
			pageIdx = lastPage
		}
		if pageIdx < 0 || pageIdx >= len(pages) {
			continue
		}
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
			bgCtx := DrawContext{Stream: bgStream, Page: page}
			for _, block := range plan.Blocks {
				drawBlock(block, x, item.y, &bgCtx, r.tagged, &r.structTags, pageIdx)
			}
			page.Stream.PrependBytes(bgStream.Bytes())
		} else {
			ctx := DrawContext{Stream: page.Stream, Page: page}
			for _, block := range plan.Blocks {
				drawBlock(block, x, item.y, &ctx, r.tagged, &r.structTags, pageIdx)
			}
		}
	}
}
