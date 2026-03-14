// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// FloatSide specifies which side a floating element is placed on.
type FloatSide int

const (
	FloatLeft  FloatSide = iota // element floats to the left, text wraps on the right
	FloatRight                  // element floats to the right, text wraps on the left
)

// Float is a layout element that places content to the left or right
// of the flow, allowing subsequent elements to wrap around it.
//
// Usage:
//
//	doc.Add(layout.NewFloat(layout.FloatLeft,
//	    layout.NewImageElement(img).SetSize(150, 0),
//	))
//	doc.Add(layout.NewParagraph("Text wraps around the image...", font.Helvetica, 12))
//
// The Float element produces a PlacedBlock with the floated content
// and modifies the available width for subsequent siblings.
type Float struct {
	side    FloatSide
	content Element
	margin  float64 // gap between float and wrapped text (default 8pt)
}

// NewFloat creates a floating element.
func NewFloat(side FloatSide, content Element) *Float {
	return &Float{
		side:    side,
		content: content,
		margin:  8,
	}
}

// SetMargin sets the gap between the float and surrounding text (default 8pt).
func (f *Float) SetMargin(m float64) *Float {
	f.margin = m
	return f
}

// Layout returns a single line representing the float's dimensions.
func (f *Float) Layout(maxWidth float64) []Line {
	plan := f.content.PlanLayout(LayoutArea{Width: maxWidth, Height: 1e9})
	w := maxBlockWidth(plan.Blocks)
	return []Line{{
		Width:  w,
		Height: plan.Consumed,
		IsLast: true,
	}}
}

// PlanLayout implements Element.
func (f *Float) PlanLayout(area LayoutArea) LayoutPlan {
	plan := f.content.PlanLayout(area)

	if plan.Status == LayoutNothing {
		return plan
	}

	// Compute the float's width from its content blocks.
	floatWidth := 0.0
	floatHeight := plan.Consumed
	for _, b := range plan.Blocks {
		if b.X+b.Width > floatWidth {
			floatWidth = b.X + b.Width
		}
	}
	floatWidth += f.margin

	// Position the content based on float side.
	x := 0.0
	if f.side == FloatRight {
		x = area.Width - floatWidth + f.margin
	}

	block := PlacedBlock{
		X:        x,
		Y:        0,
		Width:    floatWidth,
		Height:   floatHeight,
		Children: plan.Blocks,
		floatInfo: &floatBlockInfo{
			side:       f.side,
			floatWidth: floatWidth,
			height:     floatHeight,
		},
	}

	return LayoutPlan{
		Status:   plan.Status,
		Consumed: 0, // float doesn't consume vertical space in the normal flow
		Blocks:   []PlacedBlock{block},
		Overflow: plan.Overflow,
	}
}

// MinWidth implements Measurable.
func (f *Float) MinWidth() float64 {
	if m, ok := f.content.(Measurable); ok {
		return m.MinWidth() + f.margin
	}
	return f.margin
}

// MaxWidth implements Measurable.
func (f *Float) MaxWidth() float64 {
	if m, ok := f.content.(Measurable); ok {
		return m.MaxWidth() + f.margin
	}
	return f.margin
}

// floatBlockInfo carries float metadata on a PlacedBlock so the render
// loop can adjust available width for subsequent elements.
type floatBlockInfo struct {
	side       FloatSide
	floatWidth float64
	height     float64 // how far down the float extends
}
