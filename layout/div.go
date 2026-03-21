// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import folioimage "github.com/carlos7ags/folio/image"

// BackgroundImage describes a background image for a Div container.
type BackgroundImage struct {
	Image    *folioimage.Image // the image to draw
	Size     string            // "auto", "cover", "contain"
	SizeW    float64           // explicit width (0 = auto)
	SizeH    float64           // explicit height (0 = auto)
	Position [2]float64        // x%, y% (0-1 each)
	Repeat   string            // "no-repeat", "repeat", "repeat-x", "repeat-y"
}

// Padding defines the padding on each side of a container.
type Padding struct {
	Top    float64
	Right  float64
	Bottom float64
	Left   float64
}

// UniformPadding creates Padding with the same value on all sides.
func UniformPadding(p float64) Padding {
	return Padding{Top: p, Right: p, Bottom: p, Left: p}
}

// BoxShadow represents a CSS box-shadow effect.
type BoxShadow struct {
	OffsetX float64 // horizontal offset (positive = right)
	OffsetY float64 // vertical offset (positive = down)
	Blur    float64 // blur radius (approximate)
	Spread  float64 // expand/contract shadow size
	Color   Color   // shadow color
}

// Div is a generic block container that holds child elements.
// It supports borders, background color, padding, and margin,
// similar to an HTML <div>. All child elements are laid out
// vertically within the container's padded area.
type Div struct {
	elements      []Element
	padding       Padding
	borders       CellBorders
	background    *Color
	spaceBefore   float64
	spaceAfter    float64
	width         float64    // explicit outer width in points (0 = auto/fill available)
	widthPct      float64    // explicit outer width as fraction 0..1 (0 = not set)
	maxWidth      float64    // maximum outer width (0 = no limit)
	minWidth      float64    // minimum outer width (0 = no minimum)
	minHeight     float64    // minimum outer height (0 = no minimum)
	maxHeight     float64    // maximum outer height (0 = no limit)
	widthUnit     *UnitValue // lazy-resolved width (overrides width/widthPct when set)
	maxWidthUnit  *UnitValue // lazy-resolved max-width
	minWidthUnit  *UnitValue // lazy-resolved min-width
	minHeightUnit *UnitValue // lazy-resolved min-height
	maxHeightUnit *UnitValue // lazy-resolved max-height
	heightUnit    *UnitValue // lazy-resolved explicit height (forces exact height)
	hCenter       bool       // true = horizontally center within parent (margin: auto)
	hRight        bool       // true = right-align within parent (margin-left: auto)
	borderRadius  [4]float64 // corner radii [TL, TR, BR, BL] (points, 0 = sharp)
	opacity       float64    // 0..1 (0 = default/opaque, meaning "not set")
	overflow      string     // "visible" (default), "hidden"
	boxShadows    []BoxShadow
	outlineWidth  float64
	outlineStyle  string
	outlineColor  Color
	outlineOffset float64
	bgImage       *BackgroundImage
	clear         string // CSS clear: "left", "right", "both"
	structTag     string // custom structure tag for PDF/UA (empty = default "Div")

	// CSS position:relative offsets (visual only, don't affect layout flow).
	relOffsetX float64
	relOffsetY float64

	// CSS transform support.
	transforms       []TransformOp
	transformOriginX float64 // in points, relative to element top-left
	transformOriginY float64

	// keepTogether prevents the Div from splitting across pages
	// (CSS page-break-inside: avoid). If true, the renderer moves
	// the entire Div to the next page rather than splitting it.
	keepTogether bool

	// Overlay children: absolutely positioned elements within this
	// containing block. They are laid out independently and placed at
	// fixed offsets (overlayX, overlayY) from the Div's top-left,
	// without affecting normal-flow layout.
	overlays []overlayChild
}

// overlayChild is an absolutely positioned child element within a Div.
type overlayChild struct {
	elem         Element
	x            float64 // offset from containing block left edge (CSS left)
	y            float64 // offset from containing block top edge (CSS top)
	width        float64 // layout width (0 = use containing block width)
	rightAligned bool    // true when positioned with CSS right
	zIndex       int     // z-index for ordering
}

// NewDiv creates an empty Div container.
func NewDiv() *Div {
	return &Div{}
}

// Children returns the child elements of the Div.
func (d *Div) Children() []Element {
	return d.elements
}

// Add appends a child element to the Div.
func (d *Div) Add(e Element) *Div {
	d.elements = append(d.elements, e)
	return d
}

// AddOverlay adds an absolutely positioned child element that will be
// rendered at (x, y) relative to the Div's content area, without
// participating in normal flow layout. This implements CSS position:absolute
// within a containing block.
func (d *Div) AddOverlay(elem Element, x, y, width float64, rightAligned bool, zIndex int) *Div {
	d.overlays = append(d.overlays, overlayChild{
		elem: elem, x: x, y: y, width: width,
		rightAligned: rightAligned, zIndex: zIndex,
	})
	return d
}

// SetPadding sets uniform padding on all sides.
func (d *Div) SetPadding(p float64) *Div {
	d.padding = UniformPadding(p)
	return d
}

// SetPaddingAll sets different padding for each side.
func (d *Div) SetPaddingAll(p Padding) *Div {
	d.padding = p
	return d
}

// SetBorders sets the borders around the Div.
func (d *Div) SetBorders(b CellBorders) *Div {
	d.borders = b
	return d
}

// SetBorder sets the same border on all four sides.
func (d *Div) SetBorder(b Border) *Div {
	d.borders = AllBorders(b)
	return d
}

// SetBackground sets the background fill color.
func (d *Div) SetBackground(c Color) *Div {
	d.background = &c
	return d
}

// SetSpaceBefore sets extra vertical space before the Div.
func (d *Div) SetSpaceBefore(pts float64) *Div {
	d.spaceBefore = pts
	return d
}

// SetSpaceAfter sets extra vertical space after the Div.
func (d *Div) SetSpaceAfter(pts float64) *Div {
	d.spaceAfter = pts
	return d
}

// GetSpaceBefore returns the extra vertical space before the Div.
func (d *Div) GetSpaceBefore() float64 { return d.spaceBefore }

// GetSpaceAfter returns the extra vertical space after the Div.
func (d *Div) GetSpaceAfter() float64 { return d.spaceAfter }

// SetWidth sets an explicit outer width for the Div (in points).
// When set, the Div will use exactly this width (clamped by maxWidth).
func (d *Div) SetWidth(pts float64) *Div {
	d.width = pts
	return d
}

// SetWidthPercent sets the outer width as a fraction of the available area width.
// pct is a value between 0 and 1 (e.g. 0.5 = 50%). Resolved at layout time.
func (d *Div) SetWidthPercent(pct float64) *Div {
	d.widthPct = pct
	return d
}

// SetMaxWidth sets the maximum outer width of the Div (in points).
// The Div will not exceed this width even if more space is available.
func (d *Div) SetMaxWidth(pts float64) *Div {
	d.maxWidth = pts
	return d
}

// SetMinWidth sets the minimum outer width of the Div (in points).
func (d *Div) SetMinWidth(pts float64) *Div {
	d.minWidth = pts
	return d
}

// SetMinHeight sets the minimum outer height of the Div (in points).
func (d *Div) SetMinHeight(pts float64) *Div {
	d.minHeight = pts
	return d
}

// SetMaxHeight sets the maximum outer height of the Div (in points).
func (d *Div) SetMaxHeight(pts float64) *Div {
	d.maxHeight = pts
	return d
}

// SetWidthUnit sets the width as a UnitValue, resolved lazily at layout time.
// Use Pt(v) for absolute points or Pct(v) for percentage of available width.
func (d *Div) SetWidthUnit(u UnitValue) *Div {
	d.widthUnit = &u
	return d
}

// ClearWidthUnit removes the explicit width, allowing the element to use the
// full available width from its parent layout area.
func (d *Div) ClearWidthUnit() {
	d.widthUnit = nil
	d.widthPct = 0
	d.width = 0
}

// SetMaxWidthUnit sets the max-width as a UnitValue, resolved at layout time.
func (d *Div) SetMaxWidthUnit(u UnitValue) *Div {
	d.maxWidthUnit = &u
	return d
}

// SetMinWidthUnit sets the min-width as a UnitValue, resolved at layout time.
func (d *Div) SetMinWidthUnit(u UnitValue) *Div {
	d.minWidthUnit = &u
	return d
}

// SetMinHeightUnit sets the min-height as a UnitValue, resolved at layout time.
func (d *Div) SetMinHeightUnit(u UnitValue) *Div {
	d.minHeightUnit = &u
	return d
}

// SetHeightUnit sets an explicit height as a UnitValue, resolved at layout time.
// Forces the Div to this exact height (used for CSS height property).
func (d *Div) SetHeightUnit(u UnitValue) *Div {
	d.heightUnit = &u
	return d
}

// SetMaxHeightUnit sets the max-height as a UnitValue, resolved at layout time.
func (d *Div) SetMaxHeightUnit(u UnitValue) *Div {
	d.maxHeightUnit = &u
	return d
}

// ForceHeight implements HeightSettable. Sets explicit height for cross-axis stretch.
func (d *Div) ForceHeight(u UnitValue) { d.heightUnit = &u }

// ClearHeightUnit removes the explicit height, reverting to content-based sizing.
func (d *Div) ClearHeightUnit() {
	d.heightUnit = nil
}

// HasExplicitHeight returns true if the Div has an explicit CSS height set.
func (d *Div) HasExplicitHeight() bool { return d.heightUnit != nil }

// SetClear sets the CSS clear property ("left", "right", "both").
func (d *Div) SetClear(v string) *Div { d.clear = v; return d }

// ClearValue returns the CSS clear value.
func (d *Div) ClearValue() string { return d.clear }

// SetHCenter enables horizontal centering (margin: 0 auto behavior).
func (d *Div) SetHCenter(enabled bool) *Div {
	d.hCenter = enabled
	return d
}

// SetHRight enables right-alignment (margin-left: auto behavior).
func (d *Div) SetHRight(enabled bool) *Div {
	d.hRight = enabled
	return d
}

// SetRelativeOffset sets position:relative offsets. The element is visually
// shifted by (dx, dy) without affecting layout flow.
func (d *Div) SetRelativeOffset(dx, dy float64) *Div {
	d.relOffsetX = dx
	d.relOffsetY = dy
	return d
}

// SetBorderRadius sets a uniform corner radius for all four corners (in points).
func (d *Div) SetBorderRadius(r float64) *Div {
	d.borderRadius = [4]float64{r, r, r, r}
	return d
}

// SetBorderRadiusPerCorner sets per-corner radii: top-left, top-right,
// bottom-right, bottom-left (matching CSS border-radius order).
func (d *Div) SetBorderRadiusPerCorner(tl, tr, br, bl float64) *Div {
	d.borderRadius = [4]float64{tl, tr, br, bl}
	return d
}

// hasRadius returns true if any corner has a non-zero radius.
func (d *Div) hasRadius() bool {
	return d.borderRadius[0] > 0 || d.borderRadius[1] > 0 || d.borderRadius[2] > 0 || d.borderRadius[3] > 0
}

// SetOpacity sets the opacity for the entire Div (0 = transparent, 1 = opaque).
func (d *Div) SetOpacity(o float64) *Div {
	d.opacity = o
	return d
}

// SetOverflow sets the overflow behavior ("visible" or "hidden").
// "hidden" clips child content to the Div's bounds.
func (d *Div) SetOverflow(v string) *Div {
	d.overflow = v
	return d
}

// SetKeepTogether prevents the Div from splitting across pages
// (CSS page-break-inside: avoid). If the Div doesn't fit on the
// current page, it moves to the next page instead of splitting.
func (d *Div) SetKeepTogether(v bool) *Div {
	d.keepTogether = v
	return d
}

// KeepTogether reports whether the Div should avoid splitting across pages.
func (d *Div) KeepTogether() bool {
	return d.keepTogether
}

// SetTag sets a custom PDF structure tag for accessibility (PDF/UA).
// Common tags: "Sect", "Art", "BlockQuote", "Caption", "Part".
// If empty, the default "Div" tag is used.
func (d *Div) SetTag(tag string) *Div {
	d.structTag = tag
	return d
}

// resolveTag returns the structure tag for this Div.
func (d *Div) resolveTag() string {
	if d.structTag != "" {
		return d.structTag
	}
	return "Div"
}

// SetBoxShadow sets a single box-shadow effect on the Div, replacing any
// previously set shadows. For multiple shadows, use AddBoxShadow.
func (d *Div) SetBoxShadow(shadow BoxShadow) *Div {
	d.boxShadows = []BoxShadow{shadow}
	return d
}

// AddBoxShadow appends a box-shadow effect. Multiple shadows are drawn
// in reverse order (last added is drawn first, appearing behind earlier ones),
// matching CSS stacking behavior.
func (d *Div) AddBoxShadow(shadow BoxShadow) *Div {
	d.boxShadows = append(d.boxShadows, shadow)
	return d
}

// SetOutline sets an outline around the Div (drawn outside the border edge).
func (d *Div) SetOutline(width float64, style string, color Color, offset float64) *Div {
	d.outlineWidth = width
	d.outlineStyle = style
	d.outlineColor = color
	d.outlineOffset = offset
	return d
}

// SetBackgroundImage sets a background image for the Div container.
func (d *Div) SetBackgroundImage(img *BackgroundImage) *Div {
	d.bgImage = img
	return d
}

// SetTransform sets the CSS transform operations for this Div.
func (d *Div) SetTransform(ops []TransformOp) *Div {
	d.transforms = ops
	return d
}

// SetTransformOrigin sets the transform origin point relative to the
// element's top-left corner (in points).
func (d *Div) SetTransformOrigin(x, y float64) *Div {
	d.transformOriginX = x
	d.transformOriginY = y
	return d
}

// Layout returns a single synthetic line representing the Div. It delegates
// to PlanLayout to compute dimensions.
func (d *Div) Layout(maxWidth float64) []Line {
	effectiveWidth := maxWidth
	if d.maxWidth > 0 && effectiveWidth > d.maxWidth {
		effectiveWidth = d.maxWidth
	}
	if d.minWidth > 0 && effectiveWidth < d.minWidth {
		effectiveWidth = d.minWidth
	}
	plan := d.PlanLayout(LayoutArea{Width: effectiveWidth, Height: 1e9})
	innerWidth := effectiveWidth - d.padding.Left - d.padding.Right
	totalHeight := plan.Consumed - d.spaceBefore - d.spaceAfter
	contentHeight := totalHeight - d.padding.Top - d.padding.Bottom

	return []Line{{
		Height:      totalHeight,
		IsLast:      true,
		SpaceBefore: d.spaceBefore,
		SpaceAfterV: d.spaceAfter,
		divRef: &divLayoutRef{
			div:           d,
			contentHeight: contentHeight,
			totalHeight:   totalHeight,
			innerWidth:    innerWidth,
			outerWidth:    effectiveWidth,
		},
	}}
}

// MinWidth implements Measurable. Returns padding + max child MinWidth,
// or the div's own explicit width if set.
func (d *Div) MinWidth() float64 {
	// UnitValue width (only absolute values are intrinsic; percentages return 0).
	if d.widthUnit != nil && d.widthUnit.Unit == UnitPoint {
		return d.widthUnit.Value
	}
	if d.width > 0 {
		return d.width
	}
	if d.minWidth > 0 {
		return d.minWidth
	}
	maxW := 0.0
	for _, elem := range d.elements {
		if m, ok := elem.(Measurable); ok {
			if w := m.MinWidth(); w > maxW {
				maxW = w
			}
		}
	}
	return maxW + d.padding.Left + d.padding.Right
}

// MaxWidth implements Measurable. Returns padding + max child MaxWidth,
// or the div's own explicit width if set.
func (d *Div) MaxWidth() float64 {
	if d.widthUnit != nil && d.widthUnit.Unit == UnitPoint {
		return d.widthUnit.Value
	}
	if d.width > 0 {
		return d.width
	}
	maxW := 0.0
	for _, elem := range d.elements {
		if m, ok := elem.(Measurable); ok {
			if w := m.MaxWidth(); w > maxW {
				maxW = w
			}
		}
	}
	return maxW + d.padding.Left + d.padding.Right
}

// PlanLayout implements Element. A Div splits its children across pages.
func (d *Div) PlanLayout(area LayoutArea) LayoutPlan {
	effectiveWidth := area.Width

	// Resolve width: prefer UnitValue (lazy), fall back to legacy fields.
	if d.widthUnit != nil {
		effectiveWidth = d.widthUnit.Resolve(area.Width)
	} else if d.widthPct > 0 {
		effectiveWidth = area.Width * d.widthPct
	} else if d.width > 0 {
		effectiveWidth = d.width
	}

	// Resolve max-width.
	maxW := d.maxWidth
	if d.maxWidthUnit != nil {
		maxW = d.maxWidthUnit.Resolve(area.Width)
	}
	if maxW > 0 && effectiveWidth > maxW {
		effectiveWidth = maxW
	}

	// Resolve min-width.
	minW := d.minWidth
	if d.minWidthUnit != nil {
		minW = d.minWidthUnit.Resolve(area.Width)
	}
	if minW > 0 && effectiveWidth < minW {
		effectiveWidth = minW
	}
	innerWidth := effectiveWidth - d.padding.Left - d.padding.Right
	innerHeight := area.Height - d.padding.Top - d.padding.Bottom
	if innerHeight < 0 {
		innerHeight = 0
	}

	// If the div has an explicit height, use it to constrain children
	// so that percentage heights resolve against the container, not the
	// remaining page height.
	if d.heightUnit != nil {
		resolvedH := d.heightUnit.Resolve(area.Height)
		innerHeight = resolvedH - d.padding.Top - d.padding.Bottom
		if innerHeight < 0 {
			innerHeight = 0
		}
	}

	// Lay out children within the inner area.
	var fittedBlocks []PlacedBlock
	var overflowElements []Element
	curY := d.padding.Top
	remaining := innerHeight

	allFit := true
	overflowStartIdx := -1
	for idx, elem := range d.elements {
		plan := elem.PlanLayout(LayoutArea{Width: innerWidth, Height: remaining})

		switch plan.Status {
		case LayoutFull:
			for _, block := range plan.Blocks {
				block.X += d.padding.Left
				block.Y += curY
				fittedBlocks = append(fittedBlocks, block)
			}
			curY += plan.Consumed
			remaining -= plan.Consumed

		case LayoutPartial:
			for _, block := range plan.Blocks {
				block.X += d.padding.Left
				block.Y += curY
				fittedBlocks = append(fittedBlocks, block)
			}
			allFit = false
			overflowStartIdx = idx
			if plan.Overflow != nil {
				overflowElements = append(overflowElements, plan.Overflow)
			}

		case LayoutNothing:
			allFit = false
			overflowStartIdx = idx
			overflowElements = append(overflowElements, elem)
		}

		if !allFit {
			break
		}
	}

	// Add remaining un-laid-out siblings to overflow.
	if overflowStartIdx >= 0 && overflowStartIdx+1 < len(d.elements) {
		overflowElements = append(overflowElements, d.elements[overflowStartIdx+1:]...)
	}

	totalH := curY + d.padding.Bottom

	// Apply explicit height if set (CSS height property).
	if d.heightUnit != nil {
		totalH = d.heightUnit.Resolve(area.Height)
	}

	// Apply min-height / max-height constraints (prefer UnitValue).
	mh := d.minHeight
	if d.minHeightUnit != nil {
		mh = d.minHeightUnit.Resolve(area.Height)
	}
	if mh > 0 && totalH < mh {
		totalH = mh
	}
	xh := d.maxHeight
	if d.maxHeightUnit != nil {
		xh = d.maxHeightUnit.Resolve(area.Height)
	}
	if xh > 0 && totalH > xh {
		totalH = xh
	}

	// Wrap fitted content in a container block with background + borders.
	capturedDiv := d
	capturedTotalH := totalH
	capturedOuterW := effectiveWidth

	// Horizontal alignment via auto margins.
	xPos := d.relOffsetX
	if d.hCenter && effectiveWidth < area.Width {
		xPos += (area.Width - effectiveWidth) / 2
	} else if d.hRight && effectiveWidth < area.Width {
		xPos += area.Width - effectiveWidth
	}

	containerBlock := PlacedBlock{
		X: xPos, Y: d.spaceBefore + d.relOffsetY, Width: effectiveWidth, Height: totalH,
		Tag: d.resolveTag(),
		Draw: func(ctx DrawContext, absX, absTopY float64) {
			bottomY := absTopY - capturedTotalH
			r := capturedDiv.borderRadius
			hasR := capturedDiv.hasRadius()

			// Apply CSS transform if set.
			if len(capturedDiv.transforms) > 0 {
				ctx.Stream.SaveState()
				// Transform-origin: translate to origin, apply transform, translate back.
				// Origin is relative to element top-left; convert to PDF coords.
				ox := absX + capturedDiv.transformOriginX
				oy := absTopY - capturedDiv.transformOriginY
				// 1. Translate to origin.
				ctx.Stream.ConcatMatrix(1, 0, 0, 1, ox, oy)
				// 2. Apply combined transform matrix.
				a, b, c, d, e, f := ComputeTransformMatrix(capturedDiv.transforms)
				ctx.Stream.ConcatMatrix(a, b, c, d, e, f)
				// 3. Translate back.
				ctx.Stream.ConcatMatrix(1, 0, 0, 1, -ox, -oy)
			}

			// Apply opacity via ExtGState if set.
			if capturedDiv.opacity > 0 && capturedDiv.opacity < 1 {
				gsName := registerOpacity(ctx.Page, capturedDiv.opacity)
				ctx.Stream.SaveState()
				ctx.Stream.SetExtGState(gsName)
			}

			// Draw box-shadows before background/content (reverse order = CSS stacking).
			for i := len(capturedDiv.boxShadows) - 1; i >= 0; i-- {
				drawBoxShadow(ctx, &capturedDiv.boxShadows[i], absX, bottomY, capturedOuterW, capturedTotalH)
			}

			// overflow:hidden — set clipping path.
			if capturedDiv.overflow == "hidden" {
				ctx.Stream.SaveState()
				if hasR {
					ctx.Stream.RoundedRectPerCorner(absX, bottomY, capturedOuterW, capturedTotalH, r[0], r[1], r[2], r[3])
				} else {
					ctx.Stream.Rectangle(absX, bottomY, capturedOuterW, capturedTotalH)
				}
				ctx.Stream.ClipNonZero()
				ctx.Stream.EndPath()
			}

			if capturedDiv.background != nil {
				ctx.Stream.SaveState()
				setFillColor(ctx.Stream, *capturedDiv.background)
				if hasR {
					ctx.Stream.RoundedRectPerCorner(absX, bottomY, capturedOuterW, capturedTotalH, r[0], r[1], r[2], r[3])
				} else {
					ctx.Stream.Rectangle(absX, bottomY, capturedOuterW, capturedTotalH)
				}
				ctx.Stream.Fill()
				ctx.Stream.RestoreState()
			}

			// Draw background image after background color, before borders.
			if capturedDiv.bgImage != nil && capturedDiv.bgImage.Image != nil {
				drawBackgroundImage(ctx, capturedDiv.bgImage, absX, bottomY, capturedOuterW, capturedTotalH, r[0])
			}

			if hasR {
				drawRoundedBorders(ctx.Stream, capturedDiv.borders, absX, bottomY, capturedOuterW, capturedTotalH, r[0])
			} else {
				drawCellBorders(ctx.Stream, capturedDiv.borders, absX, bottomY, capturedOuterW, capturedTotalH)
			}

			// Draw outline after borders.
			if capturedDiv.outlineWidth > 0 {
				drawOutline(ctx, capturedDiv.outlineWidth, capturedDiv.outlineStyle, capturedDiv.outlineColor, capturedDiv.outlineOffset, absX, bottomY, capturedOuterW, capturedTotalH)
			}
		},
		PostDraw: func(ctx DrawContext, absX, absTopY float64) {
			// Restore clipping state.
			if capturedDiv.overflow == "hidden" {
				ctx.Stream.RestoreState()
			}
			// Restore opacity state.
			if capturedDiv.opacity > 0 && capturedDiv.opacity < 1 {
				ctx.Stream.RestoreState()
			}
			// Restore transform state.
			if len(capturedDiv.transforms) > 0 {
				ctx.Stream.RestoreState()
			}
		},
		Children: fittedBlocks,
	}

	// Lay out overlay children (position:absolute within this containing block).
	// Overlays are positioned at fixed offsets and don't affect normal flow.
	for _, ov := range d.overlays {
		ovWidth := ov.width
		if ovWidth <= 0 {
			ovWidth = innerWidth
		}
		ovPlan := ov.elem.PlanLayout(LayoutArea{Width: ovWidth, Height: totalH})
		for _, block := range ovPlan.Blocks {
			if ov.rightAligned {
				// CSS right: position from the right edge of the containing block.
				elemWidth := block.Width
				if elemWidth <= 0 {
					elemWidth = ovWidth
				}
				block.X += effectiveWidth - ov.x - elemWidth
			} else {
				block.X += d.padding.Left + ov.x
			}
			block.Y += d.padding.Top + ov.y
			containerBlock.Children = append(containerBlock.Children, block)
		}
	}

	consumed := d.spaceBefore + totalH + d.spaceAfter
	blocks := []PlacedBlock{containerBlock}

	if allFit {
		return LayoutPlan{Status: LayoutFull, Consumed: consumed, Blocks: blocks}
	}

	// Create overflow Div with remaining children.
	overflowDiv := &Div{
		elements:   overflowElements,
		padding:    d.padding,
		borders:    d.borders,
		background: d.background,
		bgImage:    d.bgImage,
		spaceAfter: d.spaceAfter,
	}
	return LayoutPlan{
		Status: LayoutPartial, Consumed: consumed, Blocks: blocks, Overflow: overflowDiv,
	}
}

// divLayoutRef carries Div-specific rendering data on a Line.
type divLayoutRef struct {
	div           *Div
	contentHeight float64
	totalHeight   float64
	innerWidth    float64
	outerWidth    float64
}
