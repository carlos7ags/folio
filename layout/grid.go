// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// GridTrackType identifies the unit of a grid track size.
type GridTrackType int

const (
	// GridTrackPx is an absolute size in PDF points.
	GridTrackPx GridTrackType = iota
	// GridTrackPercent is a percentage of the container width.
	GridTrackPercent
	// GridTrackFr is a fractional unit that shares remaining space.
	GridTrackFr
	// GridTrackAuto sizes to fit the content.
	GridTrackAuto
)

// GridTrack defines a single column or row track in a CSS Grid.
type GridTrack struct {
	Type  GridTrackType
	Value float64
}

// GridPlacement specifies explicit placement of a grid item.
// Line numbers are 1-based (matching CSS grid-line numbering).
// A zero value means "auto" (no explicit placement on that axis).
type GridPlacement struct {
	ColStart int
	ColEnd   int
	RowStart int
	RowEnd   int
}

// Grid is a container that lays out children using CSS Grid semantics.
// It implements Element and Measurable.
type Grid struct {
	children     []Element
	templateCols []GridTrack
	templateRows []GridTrack
	rowGap       float64
	colGap       float64
	placements   []GridPlacement // per-child placement; index matches children
	padding      Padding
	borders      CellBorders
	background   *Color
	spaceBefore  float64
	spaceAfter   float64
}

// NewGrid creates an empty grid container.
func NewGrid() *Grid {
	return &Grid{}
}

// AddChild appends a child element to the grid.
func (g *Grid) AddChild(e Element) *Grid {
	g.children = append(g.children, e)
	// Extend placements with a zero-value (auto) placement.
	g.placements = append(g.placements, GridPlacement{})
	return g
}

// SetTemplateColumns sets the column track definitions.
func (g *Grid) SetTemplateColumns(tracks []GridTrack) *Grid {
	g.templateCols = tracks
	return g
}

// SetTemplateRows sets the row track definitions.
func (g *Grid) SetTemplateRows(tracks []GridTrack) *Grid {
	g.templateRows = tracks
	return g
}

// SetGap sets both row and column gaps.
func (g *Grid) SetGap(row, col float64) *Grid {
	g.rowGap = row
	g.colGap = col
	return g
}

// SetRowGap sets only the row gap.
func (g *Grid) SetRowGap(gap float64) *Grid { g.rowGap = gap; return g }

// SetColumnGap sets only the column gap.
func (g *Grid) SetColumnGap(gap float64) *Grid { g.colGap = gap; return g }

// SetPlacement sets explicit grid placement for a child by index.
func (g *Grid) SetPlacement(childIndex int, p GridPlacement) *Grid {
	for len(g.placements) <= childIndex {
		g.placements = append(g.placements, GridPlacement{})
	}
	g.placements[childIndex] = p
	return g
}

// SetPadding sets uniform padding on all sides.
func (g *Grid) SetPadding(p float64) *Grid { g.padding = UniformPadding(p); return g }

// SetPaddingAll sets per-side padding.
func (g *Grid) SetPaddingAll(p Padding) *Grid { g.padding = p; return g }

// SetBorders sets the borders around the container.
func (g *Grid) SetBorders(b CellBorders) *Grid { g.borders = b; return g }

// SetBorder sets the same border on all sides.
func (g *Grid) SetBorder(b Border) *Grid { g.borders = AllBorders(b); return g }

// SetBackground sets the background fill color.
func (g *Grid) SetBackground(c Color) *Grid { g.background = &c; return g }

// SetSpaceBefore sets extra vertical space before the container.
func (g *Grid) SetSpaceBefore(pts float64) *Grid { g.spaceBefore = pts; return g }

// SetSpaceAfter sets extra vertical space after the container.
func (g *Grid) SetSpaceAfter(pts float64) *Grid { g.spaceAfter = pts; return g }

// cssGridCell records a child's resolved position within the grid.
type cssGridCell struct {
	childIdx int
	colStart int // 0-based column index
	colEnd   int // exclusive
	rowStart int // 0-based row index
	rowEnd   int // exclusive
}

// Layout implements the Element interface via a synthetic line.
func (g *Grid) Layout(maxWidth float64) []Line {
	plan := g.PlanLayout(LayoutArea{Width: maxWidth, Height: 1e9})
	totalH := plan.Consumed
	return []Line{{
		Height:      totalH,
		IsLast:      true,
		SpaceBefore: g.spaceBefore,
		SpaceAfterV: g.spaceAfter,
		divRef: &divLayoutRef{
			div:           nil,
			contentHeight: totalH,
			totalHeight:   totalH,
			innerWidth:    maxWidth - g.padding.Left - g.padding.Right,
			outerWidth:    maxWidth,
		},
	}}
}

// MinWidth implements Measurable.
func (g *Grid) MinWidth() float64 {
	hPad := g.padding.Left + g.padding.Right
	// Sum of all non-fr column minimums.
	sum := 0.0
	numCols := len(g.templateCols)
	for _, t := range g.templateCols {
		switch t.Type {
		case GridTrackPx:
			sum += t.Value
		case GridTrackAuto, GridTrackFr:
			// Minimum is 0 for fr; for auto, ideally child min-width but
			// we approximate with 0 here for simplicity.
		}
	}
	if numCols > 1 {
		sum += g.colGap * float64(numCols-1)
	}
	return sum + hPad
}

// MaxWidth implements Measurable.
func (g *Grid) MaxWidth() float64 {
	hPad := g.padding.Left + g.padding.Right
	sum := 0.0
	numCols := len(g.templateCols)
	for _, t := range g.templateCols {
		switch t.Type {
		case GridTrackPx:
			sum += t.Value
		default:
			sum += 200 // rough estimate for auto/fr max
		}
	}
	if numCols > 1 {
		sum += g.colGap * float64(numCols-1)
	}
	return sum + hPad
}

// PlanLayout implements Element.
func (g *Grid) PlanLayout(area LayoutArea) LayoutPlan {
	if len(g.children) == 0 {
		consumed := g.spaceBefore + g.padding.Top + g.padding.Bottom + g.spaceAfter
		return LayoutPlan{Status: LayoutFull, Consumed: consumed}
	}

	innerWidth := area.Width - g.padding.Left - g.padding.Right

	numCols := len(g.templateCols)
	if numCols == 0 {
		numCols = 1
		g.templateCols = []GridTrack{{Type: GridTrackAuto}}
	}

	// Step 1: Resolve column widths.
	colWidths := g.resolveColumnWidths(innerWidth, numCols)

	// Step 2: Place items on the grid.
	cells := g.placeItems(numCols)

	// Determine number of rows.
	numRows := 0
	for _, cell := range cells {
		if cell.rowEnd > numRows {
			numRows = cell.rowEnd
		}
	}
	if numRows == 0 {
		numRows = 1
	}

	// Step 3: Lay out each cell and determine row heights.
	rowHeights := make([]float64, numRows)
	cellPlans := make([]LayoutPlan, len(cells))

	for i, cell := range cells {
		// Compute available width for this cell (sum of spanned columns + gaps).
		cellWidth := g.cellWidth(cell, colWidths)
		plan := g.children[cell.childIdx].PlanLayout(LayoutArea{Width: cellWidth, Height: 1e9})
		cellPlans[i] = plan
		// Distribute consumed height across spanned rows (use max per row).
		// For single-row spans, just update that row.
		if cell.rowEnd-cell.rowStart == 1 {
			if plan.Consumed > rowHeights[cell.rowStart] {
				rowHeights[cell.rowStart] = plan.Consumed
			}
		} else {
			// Multi-row span: we don't subdivide — just ensure total span height is sufficient.
			// We'll handle this after single-row items.
		}
	}

	// Second pass for multi-row spans: ensure row heights accommodate them.
	for i, cell := range cells {
		span := cell.rowEnd - cell.rowStart
		if span <= 1 {
			continue
		}
		needed := cellPlans[i].Consumed
		// Sum current heights + gaps for the spanned rows.
		have := 0.0
		for r := cell.rowStart; r < cell.rowEnd; r++ {
			have += rowHeights[r]
			if r > cell.rowStart {
				have += g.rowGap
			}
		}
		if needed > have {
			// Distribute extra evenly among spanned rows.
			extra := (needed - have) / float64(span)
			for r := cell.rowStart; r < cell.rowEnd; r++ {
				rowHeights[r] += extra
			}
		}
	}

	// Also apply explicit row template heights where specified.
	for i, t := range g.templateRows {
		if i >= numRows {
			break
		}
		switch t.Type {
		case GridTrackPx:
			if t.Value > rowHeights[i] {
				rowHeights[i] = t.Value
			}
		case GridTrackPercent:
			h := t.Value / 100 * area.Height
			if h > rowHeights[i] {
				rowHeights[i] = h
			}
		}
	}

	// Step 4: Compute row Y-positions.
	rowY := make([]float64, numRows)
	curY := g.padding.Top
	for r := 0; r < numRows; r++ {
		if r > 0 {
			curY += g.rowGap
		}
		rowY[r] = curY
		curY += rowHeights[r]
	}

	// Compute column X-positions.
	colX := make([]float64, numCols)
	curX := g.padding.Left
	for c := 0; c < numCols; c++ {
		if c > 0 {
			curX += g.colGap
		}
		colX[c] = curX
		curX += colWidths[c]
	}

	// Step 5: Position all cell blocks.
	var allChildren []PlacedBlock
	for i, cell := range cells {
		plan := cellPlans[i]
		x := colX[cell.colStart]
		y := rowY[cell.rowStart]
		for _, block := range plan.Blocks {
			b := block
			b.X += x
			b.Y += y
			allChildren = append(allChildren, b)
		}
	}

	totalH := curY + g.padding.Bottom
	consumed := g.spaceBefore + totalH + g.spaceAfter

	containerBlock := g.makeContainerBlock(allChildren, totalH, area.Width)

	return LayoutPlan{Status: LayoutFull, Consumed: consumed, Blocks: []PlacedBlock{containerBlock}}
}

// resolveColumnWidths converts track definitions to absolute widths.
func (g *Grid) resolveColumnWidths(innerWidth float64, numCols int) []float64 {
	widths := make([]float64, numCols)
	totalGap := 0.0
	if numCols > 1 {
		totalGap = g.colGap * float64(numCols-1)
	}
	available := innerWidth - totalGap

	// First pass: resolve px and % tracks.
	remaining := available
	totalFr := 0.0
	autoCount := 0

	for i, t := range g.templateCols {
		switch t.Type {
		case GridTrackPx:
			widths[i] = t.Value
			remaining -= t.Value
		case GridTrackPercent:
			w := t.Value / 100 * innerWidth
			widths[i] = w
			remaining -= w
		case GridTrackFr:
			totalFr += t.Value
		case GridTrackAuto:
			autoCount++
		}
	}

	if remaining < 0 {
		remaining = 0
	}

	// Second pass: measure auto columns for intrinsic width.
	if autoCount > 0 {
		autoWidth := 0.0
		if totalFr > 0 {
			// When fr units are present, auto columns get their max content width
			// but not more than a fair share.
			autoWidth = remaining / float64(autoCount+1) // rough share
		} else {
			autoWidth = remaining / float64(autoCount)
		}
		for i, t := range g.templateCols {
			if t.Type == GridTrackAuto {
				widths[i] = autoWidth
				remaining -= autoWidth
			}
		}
	}

	if remaining < 0 {
		remaining = 0
	}

	// Third pass: distribute remaining space among fr tracks.
	if totalFr > 0 {
		for i, t := range g.templateCols {
			if t.Type == GridTrackFr {
				widths[i] = remaining * (t.Value / totalFr)
			}
		}
	}

	return widths
}

// placeItems assigns each child to a grid cell using explicit placements
// and auto-flow (row by row) for unplaced items.
func (g *Grid) placeItems(numCols int) []cssGridCell {
	cells := make([]cssGridCell, len(g.children))

	// Track which grid positions are occupied.
	// occupied[row][col] = true if taken.
	occupied := make(map[[2]int]bool)

	markOccupied := func(c cssGridCell) {
		for r := c.rowStart; r < c.rowEnd; r++ {
			for col := c.colStart; col < c.colEnd; col++ {
				occupied[[2]int{r, col}] = true
			}
		}
	}

	// First pass: place items with explicit placement.
	for i := range g.children {
		p := GridPlacement{}
		if i < len(g.placements) {
			p = g.placements[i]
		}

		if p.ColStart > 0 || p.RowStart > 0 {
			cell := cssGridCell{childIdx: i}

			// Convert 1-based CSS lines to 0-based indices.
			if p.ColStart > 0 {
				cell.colStart = p.ColStart - 1
			}
			if p.ColEnd > 0 {
				cell.colEnd = p.ColEnd - 1
			} else if p.ColStart > 0 {
				cell.colEnd = cell.colStart + 1
			}

			if p.RowStart > 0 {
				cell.rowStart = p.RowStart - 1
			}
			if p.RowEnd > 0 {
				cell.rowEnd = p.RowEnd - 1
			} else if p.RowStart > 0 {
				cell.rowEnd = cell.rowStart + 1
			}

			// Clamp to grid bounds for columns.
			if cell.colEnd > numCols {
				cell.colEnd = numCols
			}
			if cell.colStart >= numCols {
				cell.colStart = numCols - 1
			}
			if cell.colEnd <= cell.colStart {
				cell.colEnd = cell.colStart + 1
			}
			if cell.rowEnd <= cell.rowStart {
				cell.rowEnd = cell.rowStart + 1
			}

			cells[i] = cell
			markOccupied(cell)
		}
	}

	// Second pass: auto-place remaining items row by row.
	autoRow, autoCol := 0, 0
	for i := range g.children {
		p := GridPlacement{}
		if i < len(g.placements) {
			p = g.placements[i]
		}
		if p.ColStart > 0 || p.RowStart > 0 {
			continue // already placed
		}

		// Determine span from placement.
		colSpan := 1
		if p.ColEnd > 0 && p.ColStart == 0 {
			// "span N" encoded as ColEnd = N (special convention from parser).
			colSpan = p.ColEnd
		}
		rowSpan := 1
		if p.RowEnd > 0 && p.RowStart == 0 {
			rowSpan = p.RowEnd
		}

		// Find next available slot.
		for {
			if autoCol+colSpan > numCols {
				autoCol = 0
				autoRow++
			}
			// Check if the slot is free.
			free := true
			for r := autoRow; r < autoRow+rowSpan && free; r++ {
				for c := autoCol; c < autoCol+colSpan && free; c++ {
					if occupied[[2]int{r, c}] {
						free = false
					}
				}
			}
			if free {
				break
			}
			autoCol++
		}

		cell := cssGridCell{
			childIdx: i,
			colStart: autoCol,
			colEnd:   autoCol + colSpan,
			rowStart: autoRow,
			rowEnd:   autoRow + rowSpan,
		}
		cells[i] = cell
		markOccupied(cell)

		autoCol += colSpan
	}

	return cells
}

// cellWidth returns the total width available for a cell spanning columns.
func (g *Grid) cellWidth(cell cssGridCell, colWidths []float64) float64 {
	w := 0.0
	for c := cell.colStart; c < cell.colEnd; c++ {
		if c < len(colWidths) {
			w += colWidths[c]
		}
	}
	// Add inter-column gaps within the span.
	gaps := cell.colEnd - cell.colStart - 1
	if gaps > 0 {
		w += g.colGap * float64(gaps)
	}
	return w
}

// makeContainerBlock creates the wrapper PlacedBlock with background and borders.
func (g *Grid) makeContainerBlock(children []PlacedBlock, totalH, outerWidth float64) PlacedBlock {
	capturedGrid := g
	capturedH := totalH
	capturedW := outerWidth
	return PlacedBlock{
		X: 0, Y: g.spaceBefore, Width: outerWidth, Height: totalH,
		Tag: "Div",
		Draw: func(ctx DrawContext, absX, absTopY float64) {
			bottomY := absTopY - capturedH
			if capturedGrid.background != nil {
				ctx.Stream.SaveState()
				setFillColor(ctx.Stream, *capturedGrid.background)
				ctx.Stream.Rectangle(absX, bottomY, capturedW, capturedH)
				ctx.Stream.Fill()
				ctx.Stream.RestoreState()
			}
			drawCellBorders(ctx.Stream, capturedGrid.borders, absX, bottomY, capturedW, capturedH)
		},
		Children: children,
	}
}
