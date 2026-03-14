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
	maxWidth      float64 // maximum outer width (0 = no limit)
	minWidth      float64 // minimum outer width (0 = no minimum)
	minHeight     float64 // minimum outer height (0 = no minimum)
	maxHeight     float64 // maximum outer height (0 = no limit)
	borderRadius  float64 // corner radius (points, 0 = sharp corners)
	opacity       float64 // 0..1 (0 = default/opaque, meaning "not set")
	overflow      string  // "visible" (default), "hidden"
	boxShadow     *BoxShadow
	outlineWidth  float64
	outlineStyle  string
	outlineColor  Color
	outlineOffset float64
	bgImage       *BackgroundImage

	// CSS transform support.
	transforms       []TransformOp
	transformOriginX float64 // in points, relative to element top-left
	transformOriginY float64
}

// NewDiv creates an empty Div container.
func NewDiv() *Div {
	return &Div{}
}

// Add appends a child element to the Div.
func (d *Div) Add(e Element) *Div {
	d.elements = append(d.elements, e)
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

// SetBorderRadius sets the corner radius for rounded corners (in points).
func (d *Div) SetBorderRadius(r float64) *Div {
	d.borderRadius = r
	return d
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

// SetBoxShadow sets a box-shadow effect on the Div.
func (d *Div) SetBoxShadow(shadow BoxShadow) *Div {
	d.boxShadow = &shadow
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

// MinWidth implements Measurable. Returns padding + max child MinWidth.
func (d *Div) MinWidth() float64 {
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

// MaxWidth implements Measurable. Returns padding + max child MaxWidth.
func (d *Div) MaxWidth() float64 {
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
	if d.maxWidth > 0 && effectiveWidth > d.maxWidth {
		effectiveWidth = d.maxWidth
	}
	if d.minWidth > 0 && effectiveWidth < d.minWidth {
		effectiveWidth = d.minWidth
	}
	innerWidth := effectiveWidth - d.padding.Left - d.padding.Right
	innerHeight := area.Height - d.padding.Top - d.padding.Bottom
	if innerHeight < 0 {
		innerHeight = 0
	}

	// Lay out children within the inner area.
	var fittedBlocks []PlacedBlock
	var overflowElements []Element
	curY := d.padding.Top
	remaining := innerHeight

	allFit := true
	for _, elem := range d.elements {
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
			if plan.Overflow != nil {
				overflowElements = append(overflowElements, plan.Overflow)
			}
			break

		case LayoutNothing:
			allFit = false
			overflowElements = append(overflowElements, elem)
			break
		}

		if !allFit {
			break
		}
	}

	// Add remaining un-laid-out elements to overflow.
	if !allFit {
		for i, elem := range d.elements {
			// Find where we stopped.
			found := false
			for _, oe := range overflowElements {
				if oe == elem {
					found = true
					break
				}
			}
			if found {
				// Add all elements after this one.
				if i+1 < len(d.elements) {
					overflowElements = append(overflowElements, d.elements[i+1:]...)
				}
				break
			}
		}
	}

	totalH := curY + d.padding.Bottom

	// Apply min-height / max-height constraints.
	if d.minHeight > 0 && totalH < d.minHeight {
		totalH = d.minHeight
	}
	if d.maxHeight > 0 && totalH > d.maxHeight {
		totalH = d.maxHeight
	}

	// Wrap fitted content in a container block with background + borders.
	capturedDiv := d
	capturedTotalH := totalH
	capturedOuterW := effectiveWidth

	containerBlock := PlacedBlock{
		X: 0, Y: d.spaceBefore, Width: effectiveWidth, Height: totalH,
		Tag: "Div",
		Draw: func(ctx DrawContext, absX, absTopY float64) {
			bottomY := absTopY - capturedTotalH
			r := capturedDiv.borderRadius

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

			// Draw box-shadow before background/content.
			if capturedDiv.boxShadow != nil {
				drawBoxShadow(ctx, capturedDiv.boxShadow, absX, bottomY, capturedOuterW, capturedTotalH)
			}

			// overflow:hidden — set clipping path.
			if capturedDiv.overflow == "hidden" {
				ctx.Stream.SaveState()
				if r > 0 {
					ctx.Stream.RoundedRect(absX, bottomY, capturedOuterW, capturedTotalH, r)
				} else {
					ctx.Stream.Rectangle(absX, bottomY, capturedOuterW, capturedTotalH)
				}
				ctx.Stream.ClipNonZero()
				ctx.Stream.EndPath()
			}

			if capturedDiv.background != nil {
				ctx.Stream.SaveState()
				setFillColor(ctx.Stream, *capturedDiv.background)
				if r > 0 {
					ctx.Stream.RoundedRect(absX, bottomY, capturedOuterW, capturedTotalH, r)
				} else {
					ctx.Stream.Rectangle(absX, bottomY, capturedOuterW, capturedTotalH)
				}
				ctx.Stream.Fill()
				ctx.Stream.RestoreState()
			}

			// Draw background image after background color, before borders.
			if capturedDiv.bgImage != nil && capturedDiv.bgImage.Image != nil {
				drawBackgroundImage(ctx, capturedDiv.bgImage, absX, bottomY, capturedOuterW, capturedTotalH, r)
			}

			if r > 0 {
				drawRoundedBorders(ctx.Stream, capturedDiv.borders, absX, bottomY, capturedOuterW, capturedTotalH, r)
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
