// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "slices"

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

// tablePaging is the state of one pagination chain shared, by pointer, by a
// table and every continuation table split from it: the rows the chain
// started from, and a master grid of them per layout width. Continuation
// tables view the masters from their first body row (Table.masterStart);
// a master's row indices do not depend on its width, so a continuation
// laid out at another width (pages with different margins) views that
// width's master from the same row.
type tablePaging struct {
	rows    []*Row
	masters []*tableMaster
}

// tableMaster is the grid of a table at one layout width, shared by the
// table and every continuation table split from it. A continuation used to rebuild and
// re-measure the grid of all its remaining rows on every page, and each
// drawn row kept that whole grid alive until the document was written:
// both quadratic in the row count. The master is built once, without
// measuring, and each rowspan group is measured when pagination first
// reaches it. A continuation views the master from its first body row
// (Table.masterStart), and each page draws from a compact copy of just the
// rows it needs (pageGridFor).
//
// It is used only for tables whose header rows all lead, whose footer
// rows all trail, whose header rows do not span into the body, and in
// which no rowspan group is entered with a column still marked occupied.
// (buildGridGeometry can carry a column's occupancy one row past a
// rowspan when a colspan skips over that column; rebuilding a
// continuation's grid drops what was carried into its first row, which a
// shared master would not.) For those tables, the grid of any
// continuation is exactly the master's header rows, its body rows from
// masterStart on (always the start of a rowspan group), and its footer
// rows, so the master yields the same rows, heights and groups that
// rebuilding the continuation's grid would.
type tableMaster struct {
	width     float64   // layout width the master was built for
	colWidths []float64 // resolved column widths at that width

	grid        []gridRow // one row per table row
	groupStarts []int     // spanGroupStarts(grid)
	measured    []bool    // rows whose height is final

	headerRows int // leading header rows
	bodyEnd    int // index of the first footer row (len(grid) if none)
	tailStart  int // first row of the rowspan group holding the last body row
	irregular  bool
}

// pagingMaster returns the master grid for laying t out at width, and the
// master index of t's first body row, building the master on first use.
// It returns nil when t's layout is not regular (see tableMaster); the
// caller then builds t's grid in full.
func (t *Table) pagingMaster(width float64, colWidths []float64) (*tableMaster, int) {
	p := t.paging
	if p == nil {
		p = &tablePaging{rows: t.rows}
		t.paging = p
		t.masterStart = -1
	}
	var m *tableMaster
	for _, pm := range p.masters {
		if pm.width == width && slices.Equal(pm.colWidths, colWidths) {
			m = pm
			break
		}
	}
	if m == nil {
		m = t.newTableMaster(p.rows, width, colWidths)
		p.masters = append(p.masters, m)
	}
	if t.masterStart < 0 {
		t.masterStart = m.headerRows
	}
	if m.irregular {
		return nil, 0
	}
	return m, t.masterStart
}

func (t *Table) newTableMaster(rows []*Row, width float64, colWidths []float64) *tableMaster {
	grid, carried := t.buildGridGeometry(rows, colWidths)
	m := &tableMaster{
		width:       width,
		colWidths:   slices.Clone(colWidths),
		grid:        grid,
		groupStarts: spanGroupStarts(grid),
		measured:    make([]bool, len(grid)),
	}
	for m.headerRows < len(grid) && grid[m.headerRows].isHeader {
		m.headerRows++
	}
	m.bodyEnd = len(grid)
	for m.bodyEnd > m.headerRows && grid[m.bodyEnd-1].isFooter {
		m.bodyEnd--
	}
	for i := m.headerRows; i < m.bodyEnd; i++ {
		if grid[i].isHeader || grid[i].isFooter {
			m.irregular = true // a header or footer row among the body rows
		}
	}
	if m.headerRows < len(grid) && m.groupStarts[m.headerRows] != m.headerRows {
		m.irregular = true // a header row spans into the rows below it
	}
	for r := range grid {
		if carried[r] && m.groupStarts[r] == r {
			m.irregular = true // a group entered with a column still occupied
		}
	}
	m.tailStart = m.bodyEnd
	if m.bodyEnd > m.headerRows {
		m.tailStart = m.groupStarts[m.bodyEnd-1]
	}
	return m
}

// groupEnd returns the end (exclusive) of the rowspan group holding row r.
func (m *tableMaster) groupEnd(r int) int {
	gs := m.groupStarts[r]
	end := r + 1
	for end < len(m.grid) && m.groupStarts[end] == gs {
		end++
	}
	return end
}

// ensure gives row r its final height, measuring its whole rowspan group
// on first use: a rowspanning cell can grow the last row it spans.
func (m *tableMaster) ensure(t *Table, r int) {
	if m.measured[r] {
		return
	}
	gs, ge := m.groupStarts[r], m.groupEnd(r)
	for k := gs; k < ge; k++ {
		t.measureGridRow(&m.grid[k])
	}
	// The group is closed under rowspan, so resolving it alone gives the
	// heights resolving the whole grid would.
	t.resolveRowspanHeights(m.grid[gs:ge])
	for k := gs; k < ge; k++ {
		m.measured[k] = true
	}
}

// masterPageRows views a master as the grid of the table whose first body
// row is the master's row start.
type masterPageRows struct {
	t     *Table
	m     *tableMaster
	start int
}

// master maps an index of this view to the master's.
func (p masterPageRows) master(i int) int {
	if i < p.m.headerRows {
		return i
	}
	return p.start + i - p.m.headerRows
}

func (p masterPageRows) len() int { return p.m.headerRows + len(p.m.grid) - p.start }

func (p masterPageRows) row(i int) *gridRow {
	mi := p.master(i)
	p.m.ensure(p.t, mi)
	return &p.m.grid[mi]
}

func (p masterPageRows) groupStart(i int) int {
	mi := p.master(i)
	gs := p.m.groupStarts[mi]
	if mi < p.m.headerRows {
		return gs
	}
	return p.m.headerRows + gs - p.start
}

// pageGridFor returns the grid the page split at splitIdx draws from, for
// the table viewing the master from start (indices as in masterPageRows),
// and a function mapping a footer row's view index to its pageGrid index.
// Header rows and the body rows placed on the page keep their view index.
//
// The page grid holds the header rows; the body rows from start through
// the end of the rowspan group holding the split row, and one group more;
// and, when the table has footer rows, the group holding the last body row
// and the footer rows. So every edge drawn on the page meets the same
// neighbour across it as in the full grid, under border-collapse as
// before: the last placed row meets the first unplaced one, a spanning
// cell drawn whole on the page while its oversize group is split (only
// the group's first row caps its span) meets the row below the group, and
// the footer meets the table's last body row. Rows past a cut may meet a
// different neighbour there, but none of them is drawn on this page, and
// no rowspan crosses a cut. Cells are copied, with
// their row index rebased, so resolveBorders can write the page's resolved
// borders into them and Draw closures keep only this page's rows alive.
func (m *tableMaster) pageGridFor(t *Table, start, splitIdx, bodyEnd int, hasFooter bool) ([]gridRow, func(int) int) {
	p := masterPageRows{t: t, m: m, start: start}
	identity := func(i int) int { return i }

	end := len(m.grid)
	footerIdx := identity
	if splitIdx < bodyEnd {
		end = m.groupEnd(p.master(splitIdx))
		if end < len(m.grid) {
			end = m.groupEnd(end)
		}
		if hasFooter && end < m.tailStart {
			cut := m.headerRows + end - start
			footerIdx = func(i int) int { return cut + p.master(i) - m.tailStart }
		} else if hasFooter {
			end = len(m.grid)
		}
	}

	var src []int
	for r := range m.headerRows {
		src = append(src, r)
	}
	for r := start; r < end; r++ {
		src = append(src, r)
	}
	if end < len(m.grid) && hasFooter {
		for r := m.tailStart; r < len(m.grid); r++ {
			src = append(src, r)
		}
	}

	page := make([]gridRow, len(src))
	for i, r := range src {
		m.ensure(t, r)
		page[i] = m.grid[r]
		page[i].cells = slices.Clone(m.grid[r].cells)
		for c := range page[i].cells {
			page[i].cells[c].row = i
		}
	}
	return page, footerIdx
}
