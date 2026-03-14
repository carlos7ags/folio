// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package barcode generates 1D and 2D barcodes that can be rendered
// directly into a PDF content stream as vector graphics (no images).
package barcode

import "github.com/carlos7ags/folio/content"

// Barcode is a generated barcode ready to be drawn into a PDF.
type Barcode struct {
	modules [][]bool // 2D grid of modules (true = dark, false = light)
	width   int      // number of modules wide
	height  int      // number of modules tall
}

// Width returns the number of modules wide.
func (b *Barcode) Width() int { return b.width }

// Height returns the number of modules tall.
func (b *Barcode) Height() int { return b.height }

// Draw renders the barcode into a PDF content stream as filled rectangles.
// x, y is the bottom-left corner; w, h are the total dimensions in points.
func (b *Barcode) Draw(stream *content.Stream, x, y, w, h float64) {
	if b.width == 0 || b.height == 0 {
		return
	}

	modW := w / float64(b.width)
	modH := h / float64(b.height)

	stream.SaveState()
	stream.SetFillColorRGB(0, 0, 0) // black modules

	for row := range b.height {
		for col := range b.width {
			if b.modules[row][col] {
				mx := x + float64(col)*modW
				// PDF y-axis: row 0 is top of barcode, so flip.
				my := y + h - float64(row+1)*modH
				stream.Rectangle(mx, my, modW, modH)
			}
		}
	}
	stream.Fill()
	stream.RestoreState()
}

// new1D creates a Barcode from a 1D bit pattern (single row of modules).
func new1D(modules []bool, barHeight int) *Barcode {
	grid := make([][]bool, barHeight)
	for i := range grid {
		row := make([]bool, len(modules))
		copy(row, modules)
		grid[i] = row
	}
	return &Barcode{
		modules: grid,
		width:   len(modules),
		height:  barHeight,
	}
}
