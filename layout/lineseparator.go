// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// LineSeparator is a layout element that draws a horizontal rule
// across the available width. Used to visually separate sections.
type LineSeparator struct {
	width       float64     // line width in points (default 0.5)
	color       Color       // stroke color (default black)
	style       BorderStyle // line style (default solid)
	spaceBefore float64     // vertical space above
	spaceAfter  float64     // vertical space below
	fraction    float64     // fraction of available width to draw (0-1, default 1.0)
	align       Align       // alignment when fraction < 1.0
}

// NewLineSeparator creates a thin black horizontal rule.
func NewLineSeparator() *LineSeparator {
	return &LineSeparator{
		width:    0.5,
		color:    ColorBlack,
		style:    BorderSolid,
		fraction: 1.0,
		align:    AlignLeft,
	}
}

// SetWidth sets the line width in points.
func (ls *LineSeparator) SetWidth(w float64) *LineSeparator {
	ls.width = w
	return ls
}

// SetColor sets the stroke color.
func (ls *LineSeparator) SetColor(c Color) *LineSeparator {
	ls.color = c
	return ls
}

// SetStyle sets the line style (solid, dashed, dotted).
func (ls *LineSeparator) SetStyle(s BorderStyle) *LineSeparator {
	ls.style = s
	return ls
}

// SetSpaceBefore sets vertical space above the separator.
func (ls *LineSeparator) SetSpaceBefore(pts float64) *LineSeparator {
	ls.spaceBefore = pts
	return ls
}

// SetSpaceAfter sets vertical space below the separator.
func (ls *LineSeparator) SetSpaceAfter(pts float64) *LineSeparator {
	ls.spaceAfter = pts
	return ls
}

// SetFraction sets what fraction of available width to draw (0–1).
// Default is 1.0 (full width). Use with SetAlign to position shorter rules.
func (ls *LineSeparator) SetFraction(f float64) *LineSeparator {
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	ls.fraction = f
	return ls
}

// SetAlign sets alignment when fraction < 1.0.
func (ls *LineSeparator) SetAlign(a Align) *LineSeparator {
	ls.align = a
	return ls
}

// Layout implements Element.
func (ls *LineSeparator) Layout(maxWidth float64) []Line {
	lineW := maxWidth * ls.fraction
	return []Line{{
		Width:       lineW,
		Height:      ls.width, // visual height is the line thickness
		Align:       ls.align,
		IsLast:      true,
		SpaceBefore: ls.spaceBefore,
		SpaceAfterV: ls.spaceAfter,
		separatorRef: &separatorLayoutRef{
			lineWidth: ls.width,
			color:     ls.color,
			style:     ls.style,
			drawWidth: lineW,
		},
	}}
}

// separatorLayoutRef carries rendering data for a LineSeparator.
type separatorLayoutRef struct {
	lineWidth float64
	color     Color
	style     BorderStyle
	drawWidth float64
}

// MinWidth implements Measurable. A separator has no minimum width.
func (ls *LineSeparator) MinWidth() float64 { return 0 }

// MaxWidth implements Measurable. A separator fills available width.
func (ls *LineSeparator) MaxWidth() float64 { return 0 }

// PlanLayout implements Element. A separator never splits — it's FULL or NOTHING.
func (ls *LineSeparator) PlanLayout(area LayoutArea) LayoutPlan {
	totalH := ls.spaceBefore + ls.width + ls.spaceAfter
	if totalH > area.Height && area.Height > 0 {
		return LayoutPlan{Status: LayoutNothing}
	}

	drawW := area.Width * ls.fraction
	x := 0.0
	switch ls.align {
	case AlignCenter:
		x = (area.Width - drawW) / 2
	case AlignRight:
		x = area.Width - drawW
	}

	capturedLS := *ls
	return LayoutPlan{
		Status:   LayoutFull,
		Consumed: totalH,
		Blocks: []PlacedBlock{{
			X:      x,
			Y:      ls.spaceBefore,
			Width:  drawW,
			Height: ls.width,
			Draw: func(ctx DrawContext, absX, absTopY float64) {
				b := Border{Width: capturedLS.width, Color: capturedLS.color, Style: capturedLS.style}
				cy := absTopY - capturedLS.width/2
				drawStyledBorder(ctx.Stream, b, absX, cy, absX+drawW, cy)
			},
		}},
	}
}
