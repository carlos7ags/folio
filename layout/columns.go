// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// ColumnRule defines a visual separator drawn between columns.
type ColumnRule struct {
	Width float64 // line width in points (default 0 = no rule)
	Color Color   // line color (default black)
	Style string  // "solid" (default), "dashed", "dotted"
}

// Columns is a block-level element that arranges child elements
// side by side in equal-width (or custom-width) columns.
type Columns struct {
	cols     int         // number of columns
	gap      float64     // gap between columns (points)
	widths   []float64   // optional explicit column widths (fractions 0–1)
	elements [][]Element // elements[colIndex] = list of elements in that column
	rule     ColumnRule  // vertical rule drawn between columns

	// balanced enables height-based distribution. When true,
	// PlanLayout measures all elements in column 0 and then
	// redistributes them across columns to equalize heights.
	// Set via [BalancedColumns].
	balanced bool
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

// SetColumnRule sets the vertical rule drawn between columns.
// CSS equivalent: column-rule: 1px solid gray.
func (c *Columns) SetColumnRule(rule ColumnRule) *Columns {
	c.rule = rule
	return c
}

// SetColumnRuleWidth sets just the rule width (shorthand for a solid black rule).
func (c *Columns) SetColumnRuleWidth(width float64) *Columns {
	c.rule.Width = width
	if c.rule.Style == "" {
		c.rule.Style = "solid"
	}
	return c
}

// SetBalanced enables height-based redistribution at layout time. When
// true, PlanLayout measures all elements (regardless of which column
// they were added to) and redistributes them across columns to equalize
// heights. Elements are packed in document order using a greedy
// algorithm that targets (totalHeight / numColumns) per column. This
// is the CSS column-fill: balance behavior.
func (c *Columns) SetBalanced(b bool) *Columns {
	c.balanced = b
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
// and combine them. When balanced is set, elements are redistributed
// across columns by measured height before final layout.
func (c *Columns) PlanLayout(area LayoutArea) LayoutPlan {
	colWidths := c.resolveWidths(area.Width)

	if c.balanced {
		c.redistribute(colWidths)
	}

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

// redistribute measures all elements from column 0 (where BalancedColumns
// deposits them) and redistributes them across columns so that column
// heights are approximately equal. Uses a greedy algorithm: fill each
// column up to (totalHeight / numColumns), then spill to the next.
func (c *Columns) redistribute(colWidths []float64) {
	// Collect all elements from every column into a flat list.
	var all []Element
	for _, elems := range c.elements {
		all = append(all, elems...)
	}
	if len(all) == 0 {
		return
	}

	// Measure each element using the first column's width. All columns
	// in balanced mode are equal-width, so colWidths[0] is representative.
	measureWidth := colWidths[0]
	heights := make([]float64, len(all))
	totalH := 0.0
	for i, elem := range all {
		plan := elem.PlanLayout(LayoutArea{Width: measureWidth, Height: 1e9})
		heights[i] = plan.Consumed
		totalH += plan.Consumed
	}

	target := totalH / float64(c.cols)

	// Reset column slots.
	for i := range c.elements {
		c.elements[i] = nil
	}

	col := 0
	colH := 0.0
	for i, elem := range all {
		// If adding this element would exceed the target AND we haven't
		// reached the last column AND the column already has content,
		// advance to the next column.
		if colH+heights[i] > target && col < c.cols-1 && colH > 0 {
			col++
			colH = 0
		}
		c.elements[col] = append(c.elements[col], elem)
		colH += heights[i]
	}
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

	// Capture column rule drawing if configured.
	var drawFunc func(ctx DrawContext, x, topY float64)
	if c.rule.Width > 0 && c.cols > 1 {
		capturedWidths := make([]float64, len(colWidths))
		copy(capturedWidths, colWidths)
		capturedGap := c.gap
		capturedRule := c.rule
		capturedH := totalH

		drawFunc = func(ctx DrawContext, x, topY float64) {
			drawColumnRules(ctx, capturedWidths, capturedGap, capturedRule, x, topY, capturedH)
		}
	}

	return LayoutPlan{
		Status:   LayoutFull,
		Consumed: totalH,
		Blocks: []PlacedBlock{{
			X: 0, Y: 0, Width: areaWidth, Height: totalH,
			Draw:     drawFunc,
			Children: children,
		}},
	}
}

// drawColumnRules draws vertical rules between columns.
func drawColumnRules(ctx DrawContext, colWidths []float64, gap float64, rule ColumnRule, absX, topY, height float64) {
	ctx.Stream.SaveState()
	ctx.Stream.SetLineWidth(rule.Width)
	setStrokeColor(ctx.Stream, rule.Color)

	switch rule.Style {
	case "dashed":
		ctx.Stream.SetDashPattern([]float64{4, 2}, 0)
	case "dotted":
		ctx.Stream.SetDashPattern([]float64{1, 2}, 0)
	}

	xPos := absX
	for i := 0; i < len(colWidths)-1; i++ {
		xPos += colWidths[i]
		// Draw rule centered in the gap.
		ruleX := xPos + gap/2
		bottomY := topY - height
		ctx.Stream.MoveTo(ruleX, topY)
		ctx.Stream.LineTo(ruleX, bottomY)
		ctx.Stream.Stroke()
		xPos += gap
	}

	ctx.Stream.RestoreState()
}

// BalancedColumns creates a multi-column layout that equalizes column
// heights. All elements are placed into column 0 initially; at layout
// time [Columns.PlanLayout] measures each element and redistributes
// them across columns using a greedy packing algorithm that targets
// (totalHeight / numColumns) per column. This produces visually
// balanced output even when elements have different heights.
func BalancedColumns(cols int, gap float64, elements ...Element) *Columns {
	c := NewColumns(cols).SetGap(gap)
	c.balanced = true
	for _, elem := range elements {
		c.Add(0, elem)
	}
	return c
}
