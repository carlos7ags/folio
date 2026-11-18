// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// cellMeasureKey identifies one measurement of a cell's content: the same
// cell can be measured at several inner widths (a table planned at two
// widths, or auto column sizing), each with its own height.
type cellMeasureKey struct {
	cell  *Cell
	width float64
}

// tableMeasureCache memoizes cell measurements for a table and the
// continuation tables split from it: an element cell's laid-out height, a
// text cell's line count. Without it, every page of a table that spans
// many pages re-measured every remaining row, which made pagination
// quadratic in the row count.
type tableMeasureCache struct {
	heights map[cellMeasureKey]float64
	lines   map[cellMeasureKey]int
}

// elementHeight returns the height of an element cell's content laid out
// at innerWidth, padding excluded, laying it out on first use.
func (c *tableMeasureCache) elementHeight(cell *Cell, innerWidth float64) float64 {
	key := cellMeasureKey{cell: cell, width: innerWidth}
	if h, ok := c.heights[key]; ok {
		return h
	}
	h := measureConsumed(cell.content, innerWidth)
	if c.heights == nil {
		c.heights = make(map[cellMeasureKey]float64)
	}
	c.heights[key] = h
	return h
}

// textLines returns countCellTextLines(cell, innerWidth), counting on
// first use.
func (c *tableMeasureCache) textLines(cell *Cell, innerWidth float64) int {
	key := cellMeasureKey{cell: cell, width: innerWidth}
	if n, ok := c.lines[key]; ok {
		return n
	}
	n := countCellTextLines(cell, innerWidth)
	if c.lines == nil {
		c.lines = make(map[cellMeasureKey]int)
	}
	c.lines[key] = n
	return n
}

// pageRows is the grid Table.PlanLayout paginates: header rows, then the
// body rows still to place, then footer rows. row returns a row with its
// final height; groupStart returns the first row of the rowspan group
// holding row i (see spanGroupStarts).
type pageRows interface {
	len() int
	row(i int) *gridRow
	groupStart(i int) int
}

// gridPageRows is a fully built and measured grid.
type gridPageRows struct {
	grid   []gridRow
	starts []int
}

func (p gridPageRows) len() int             { return len(p.grid) }
func (p gridPageRows) row(i int) *gridRow   { return &p.grid[i] }
func (p gridPageRows) groupStart(i int) int { return p.starts[i] }
