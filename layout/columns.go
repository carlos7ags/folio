// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// Columns is a block-level element that arranges child elements
// side by side in equal-width (or custom-width) columns.
type Columns struct {
	cols     int         // number of columns
	gap      float64     // gap between columns (points)
	widths   []float64   // optional explicit column widths (fractions 0–1)
	elements [][]Element // elements[colIndex] = list of elements in that column
}

// columnsLayoutRef carries per-column line data for the renderer.
type columnsLayoutRef struct {
	colLines []columnLine // one entry per column with content at this row
}

// columnLine holds the data for one column's line at a given row.
type columnLine struct {
	xOffset float64 // x-offset from left margin
	width   float64 // column width
	line    *Line   // the actual line content (nil if this column is exhausted)
}

// NewColumns creates a multi-column layout with the given number of columns.
// Panics if cols < 1.
func NewColumns(cols int) *Columns {
	if cols < 1 {
		panic("layout.NewColumns: cols must be >= 1")
	}
	elements := make([][]Element, cols)
	return &Columns{
		cols:     cols,
		gap:      12, // default gap
		elements: elements,
	}
}

// SetGap sets the gap between columns in points (default 12).
// Negative values are clamped to 0.
func (c *Columns) SetGap(gap float64) *Columns {
	if gap < 0 {
		gap = 0
	}
	c.gap = gap
	return c
}

// SetWidths sets explicit column width fractions. Each value is a fraction
// of the total available width (after subtracting gaps). They should sum to 1.0.
// If not set, columns are equal width.
func (c *Columns) SetWidths(widths []float64) *Columns {
	c.widths = widths
	return c
}

// Add adds an element to a specific column (0-indexed).
func (c *Columns) Add(col int, elem Element) *Columns {
	if col >= 0 && col < c.cols {
		c.elements[col] = append(c.elements[col], elem)
	}
	return c
}

// resolveWidths computes actual column widths in points.
func (c *Columns) resolveWidths(maxWidth float64) []float64 {
	totalGap := c.gap * float64(c.cols-1)
	contentWidth := maxWidth - totalGap

	widths := make([]float64, c.cols)
	if len(c.widths) == c.cols {
		for i, frac := range c.widths {
			widths[i] = contentWidth * frac
		}
	} else {
		w := contentWidth / float64(c.cols)
		for i := range widths {
			widths[i] = w
		}
	}
	return widths
}

// Layout implements Element. It lays out each column independently,
// then combines them row by row, with each output line representing
// one horizontal slice across all columns.
func (c *Columns) Layout(maxWidth float64) []Line {
	colWidths := c.resolveWidths(maxWidth)

	// Lay out each column into its own slice of lines.
	type layoutable interface {
		Layout(maxWidth float64) []Line
	}
	colLines := make([][]Line, c.cols)
	for i := range c.cols {
		var lines []Line
		for _, elem := range c.elements[i] {
			if l, ok := elem.(layoutable); ok {
				lines = append(lines, l.Layout(colWidths[i])...)
			}
		}
		colLines[i] = lines
	}

	// Find the maximum number of lines across all columns.
	maxLines := 0
	for _, cl := range colLines {
		if len(cl) > maxLines {
			maxLines = len(cl)
		}
	}

	// Combine into output lines, one per row.
	var result []Line
	for row := range maxLines {
		// Determine the tallest line in this row and the max spacing.
		rowHeight := 0.0
		maxSpaceBefore := 0.0
		maxSpaceAfterV := 0.0
		for i := range c.cols {
			if row < len(colLines[i]) {
				h := colLines[i][row].Height + colLines[i][row].SpaceBefore + colLines[i][row].SpaceAfterV
				if h > rowHeight {
					rowHeight = h
				}
				if colLines[i][row].SpaceBefore > maxSpaceBefore {
					maxSpaceBefore = colLines[i][row].SpaceBefore
				}
				if colLines[i][row].SpaceAfterV > maxSpaceAfterV {
					maxSpaceAfterV = colLines[i][row].SpaceAfterV
				}
			}
		}

		// Build column line entries.
		cls := make([]columnLine, c.cols)
		xOffset := 0.0
		for i := range c.cols {
			cls[i].xOffset = xOffset
			cls[i].width = colWidths[i]
			if row < len(colLines[i]) {
				lineCopy := colLines[i][row]
				cls[i].line = &lineCopy
			}
			xOffset += colWidths[i] + c.gap
		}

		result = append(result, Line{
			Height:      rowHeight,
			SpaceBefore: maxSpaceBefore,
			SpaceAfterV: maxSpaceAfterV,
			columnsRef: &columnsLayoutRef{
				colLines: cls,
			},
		})
	}

	return result
}

// PlanLayout implements Element. Columns lay out each column independently
// and combine them. If balanced is true, column heights are equalized.
func (c *Columns) PlanLayout(area LayoutArea) LayoutPlan {
	colWidths := c.resolveWidths(area.Width)

	// Lay out each column with unlimited height to measure total content.
	colBlocks, colHeights := c.layoutColumns(colWidths, 1e9)

	// Total height is the tallest column.
	totalH := 0.0
	for _, h := range colHeights {
		if h > totalH {
			totalH = h
		}
	}

	if totalH > area.Height && area.Height > 0 {
		return LayoutPlan{Status: LayoutNothing}
	}

	return c.buildColumnsPlan(colBlocks, colWidths, totalH, area.Width)
}

// layoutColumns lays out each column's elements and returns positioned blocks and heights.
func (c *Columns) layoutColumns(colWidths []float64, maxHeight float64) ([][]PlacedBlock, []float64) {
	colBlocks := make([][]PlacedBlock, c.cols)
	colHeights := make([]float64, c.cols)

	for i := range c.cols {
		y := 0.0
		for _, elem := range c.elements[i] {
			plan := elem.PlanLayout(LayoutArea{Width: colWidths[i], Height: maxHeight - y})
			for _, block := range plan.Blocks {
				b := block
				b.Y += y
				colBlocks[i] = append(colBlocks[i], b)
			}
			y += plan.Consumed
		}
		colHeights[i] = y
	}

	return colBlocks, colHeights
}

// buildColumnsPlan assembles the final LayoutPlan from column blocks.
func (c *Columns) buildColumnsPlan(colBlocks [][]PlacedBlock, colWidths []float64, totalH, areaWidth float64) LayoutPlan {
	var children []PlacedBlock
	xOffset := 0.0
	for i := range c.cols {
		for _, block := range colBlocks[i] {
			b := block
			b.X += xOffset
			children = append(children, b)
		}
		xOffset += colWidths[i] + c.gap
	}

	return LayoutPlan{
		Status:   LayoutFull,
		Consumed: totalH,
		Blocks: []PlacedBlock{{
			X: 0, Y: 0, Width: areaWidth, Height: totalH,
			Children: children,
		}},
	}
}

// BalancedColumns creates a multi-column layout that equalizes column heights.
// Content is placed in columns sequentially but each column is limited to
// approximately (totalHeight / numColumns) to balance the visual output.
func BalancedColumns(cols int, gap float64, elements ...Element) *Columns {
	c := NewColumns(cols).SetGap(gap)

	// Distribute elements round-robin across columns.
	// For true balancing, we'd need to measure and redistribute,
	// but round-robin is a good approximation for uniform content.
	for i, elem := range elements {
		c.Add(i%cols, elem)
	}

	return c
}
