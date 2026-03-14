// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"github.com/carlos7ags/folio/content"
)

// renderWithPlans lays out elements into pages using PlanLayout.
// Each Element computes a height-aware LayoutPlan that supports
// content splitting across pages via Overflow.
func (r *Renderer) renderWithPlans() []PageResult {
	maxWidth := r.pageWidth - r.margins.Left - r.margins.Right
	usableHeight := r.pageHeight - r.margins.Top - r.margins.Bottom

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
			drawBlock(block, r.margins.Left, r.pageHeight-r.margins.Top, &ctx, r.tagged, &r.structTags, pageIdx)
		}

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
		remainingHeight = usableHeight
		pageIdx++
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
			floats = nil
			pageIdx++
			atPageTop = true
			continue
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
			curBlocks = append(curBlocks, plan.Blocks...)
			remainingHeight -= plan.Consumed
			if !isFloat {
				consumeFloatHeight(plan.Consumed)
			}
			atPageTop = false

		case LayoutPartial:
			if atPageTop && len(plan.Blocks) > 0 {
				plan.Blocks[0].Y = 0
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
				curBlocks = append(curBlocks, forcePlan.Blocks...)
				remainingHeight = 0
				atPageTop = false
				if forcePlan.Overflow != nil {
					queue = append([]Element{forcePlan.Overflow}, queue...)
				}
			}
		}
	}

	// Flush the last page.
	if len(curBlocks) > 0 {
		flushPage()
	} else if len(pages) == 0 {
		// Ensure at least one page.
		pages = append(pages, PageResult{Stream: content.NewStream()})
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

	// Record link annotation.
	if block.Link != nil {
		ctx.Page.Links = append(ctx.Page.Links, LinkArea{
			X:        pdfX,
			Y:        pdfY - block.Height,
			W:        block.Width,
			H:        block.Height,
			URI:      block.Link.URI,
			DestName: block.Link.DestName,
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
// onto the appropriate pages using the V2 system.
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

		ctx := DrawContext{Stream: page.Stream, Page: page}
		for _, block := range plan.Blocks {
			drawBlock(block, item.x, item.y, &ctx, r.tagged, &r.structTags, pageIdx)
		}
	}
}
