// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/font"
)

// BorderStyle specifies how a border line is drawn.
type BorderStyle int

const (
	BorderSolid  BorderStyle = iota // continuous line (default)
	BorderDashed                    // repeating dash pattern
	BorderDotted                    // repeating dot pattern
	BorderDouble                    // two parallel lines
	BorderNone                      // no border (same as Width=0)
)

// Border defines the style for one side of a cell or container border.
type Border struct {
	Width float64     // line width in points (0 = no border)
	Color Color       // stroke color
	Style BorderStyle // line style (default: solid)
}

// CellBorders holds the four borders of a cell or container.
type CellBorders struct {
	Top    Border
	Right  Border
	Bottom Border
	Left   Border
}

// DefaultBorder returns a thin black solid border.
func DefaultBorder() Border {
	return Border{Width: 0.5, Color: ColorBlack, Style: BorderSolid}
}

// SolidBorder creates a solid border with the given width and color.
func SolidBorder(width float64, c Color) Border {
	return Border{Width: width, Color: c, Style: BorderSolid}
}

// DashedBorder creates a dashed border with the given width and color.
func DashedBorder(width float64, c Color) Border {
	return Border{Width: width, Color: c, Style: BorderDashed}
}

// DottedBorder creates a dotted border with the given width and color.
func DottedBorder(width float64, c Color) Border {
	return Border{Width: width, Color: c, Style: BorderDotted}
}

// DoubleBorder creates a double-line border with the given width and color.
// The total visual width is approximately 3x the line width.
func DoubleBorder(width float64, c Color) Border {
	return Border{Width: width, Color: c, Style: BorderDouble}
}

// AllBorders returns CellBorders with the same border on all sides.
func AllBorders(b Border) CellBorders {
	return CellBorders{Top: b, Right: b, Bottom: b, Left: b}
}

// NoBorders returns CellBorders with no borders.
func NoBorders() CellBorders {
	return CellBorders{}
}

// Cell represents a single cell in a table.
type Cell struct {
	text     string
	font     *font.Standard
	embedded *font.EmbeddedFont
	fontSize float64
	content  Element // rich content (if non-nil, overrides text/font)
	align    Align
	valign   VAlign
	padding  float64  // uniform padding (all sides)
	padSides *Padding // per-side padding (overrides uniform when set)
	borders  CellBorders
	colspan  int
	rowspan  int
	bgColor  *Color  // background fill color (nil = transparent)
	hintW    float64 // CSS width hint in points (0 = not set)
}

// SetWidthHint sets the CSS width hint for this cell (in points).
// Used by auto-sizing to influence column width allocation.
func (c *Cell) SetWidthHint(pts float64) *Cell {
	c.hintW = pts
	return c
}

// SetAlign sets the horizontal text alignment within the cell.
func (c *Cell) SetAlign(a Align) *Cell {
	c.align = a
	return c
}

// SetPadding sets uniform padding on all sides (in points).
func (c *Cell) SetPadding(p float64) *Cell {
	c.padding = p
	c.padSides = nil
	return c
}

// SetPaddingSides sets different padding for each side.
func (c *Cell) SetPaddingSides(p Padding) *Cell {
	c.padSides = &p
	return c
}

// padTop returns the top padding.
func (c *Cell) padTop() float64 {
	if c.padSides != nil {
		return c.padSides.Top
	}
	return c.padding
}

// padRight returns the right padding.
func (c *Cell) padRight() float64 {
	if c.padSides != nil {
		return c.padSides.Right
	}
	return c.padding
}

// padBottom returns the bottom padding.
func (c *Cell) padBottom() float64 {
	if c.padSides != nil {
		return c.padSides.Bottom
	}
	return c.padding
}

// padLeft returns the left padding.
func (c *Cell) padLeft() float64 {
	if c.padSides != nil {
		return c.padSides.Left
	}
	return c.padding
}

// SetBorders sets the cell borders.
func (c *Cell) SetBorders(b CellBorders) *Cell {
	c.borders = b
	return c
}

// SetVAlign sets the vertical alignment within the cell.
func (c *Cell) SetVAlign(v VAlign) *Cell {
	c.valign = v
	return c
}

// SetBackground sets the cell background fill color.
func (c *Cell) SetBackground(color Color) *Cell {
	c.bgColor = &color
	return c
}

// SetColspan sets the number of columns this cell spans.
func (c *Cell) SetColspan(n int) *Cell {
	if n < 1 {
		n = 1
	}
	c.colspan = n
	return c
}

// SetRowspan sets the number of rows this cell spans.
func (c *Cell) SetRowspan(n int) *Cell {
	if n < 1 {
		n = 1
	}
	c.rowspan = n
	return c
}

// Row represents a row in a table.
type Row struct {
	cells    []*Cell
	isHeader bool
	isFooter bool
}

// AddCell adds a cell with text using a standard font.
func (r *Row) AddCell(text string, f *font.Standard, fontSize float64) *Cell {
	c := &Cell{
		text:     text,
		font:     f,
		fontSize: fontSize,
		align:    AlignLeft,
		padding:  4,
		borders:  AllBorders(DefaultBorder()),
		colspan:  1,
		rowspan:  1,
	}
	r.cells = append(r.cells, c)
	return c
}

// AddCellEmbedded adds a cell with text using an embedded font.
func (r *Row) AddCellEmbedded(text string, ef *font.EmbeddedFont, fontSize float64) *Cell {
	c := &Cell{
		text:     text,
		embedded: ef,
		fontSize: fontSize,
		align:    AlignLeft,
		padding:  4,
		borders:  AllBorders(DefaultBorder()),
		colspan:  1,
		rowspan:  1,
	}
	r.cells = append(r.cells, c)
	return c
}

// AddCellElement adds a cell containing any layout Element (paragraph, table,
// list, image, etc.) instead of plain text.
func (r *Row) AddCellElement(elem Element) *Cell {
	c := &Cell{
		content: elem,
		align:   AlignLeft,
		padding: 4,
		borders: AllBorders(DefaultBorder()),
		colspan: 1,
		rowspan: 1,
	}
	r.cells = append(r.cells, c)
	return c
}

// Table is a layout element that renders a grid of cells with borders.
// Builder API backed by a flat grid internally.
type Table struct {
	rows           []*Row
	colWidths      []float64   // explicit column widths in points (nil = equal distribution)
	colUnitWidths  []UnitValue // unit-based column widths (overrides colWidths if set)
	autoWidths     bool        // if true, compute column widths from cell content
	borderCollapse bool        // if true, collapse adjacent cell borders
	minWidth       float64     // minimum total table width (0 = no minimum)
	minWidthUnit   *UnitValue  // lazy-resolved min-width (overrides minWidth when set)
	cellSpacingH   float64     // horizontal spacing between cells (CSS border-spacing)
	cellSpacingV   float64     // vertical spacing between cells (CSS border-spacing)
}

// NewTable creates a new empty table.
func NewTable() *Table {
	return &Table{}
}

// SetColumnWidths sets explicit column widths in points.
// If not set, columns are distributed equally within the available width.
func (t *Table) SetColumnWidths(widths []float64) *Table {
	t.colWidths = widths
	t.colUnitWidths = nil
	t.autoWidths = false
	return t
}

// SetBorderCollapse enables CSS-style border-collapse. When true,
// adjacent cell borders are merged so you don't get double borders.
// This removes the right border of each cell except the last column,
// and the bottom border of each cell except the last row.
func (t *Table) SetBorderCollapse(enabled bool) *Table {
	t.borderCollapse = enabled
	return t
}

// BorderCollapse reports whether border-collapse is enabled.
func (t *Table) BorderCollapse() bool {
	return t.borderCollapse
}

// SetCellSpacing sets horizontal and vertical spacing between cells,
// corresponding to the CSS border-spacing property. Spacing is added
// between adjacent cells and at the table edges. Ignored when
// border-collapse is enabled.
func (t *Table) SetCellSpacing(h, v float64) *Table {
	t.cellSpacingH = h
	t.cellSpacingV = v
	return t
}

// effectiveSpacingH returns the horizontal cell spacing, or 0 when
// border-collapse is active.
func (t *Table) effectiveSpacingH() float64 {
	if t.borderCollapse {
		return 0
	}
	return t.cellSpacingH
}

// effectiveSpacingV returns the vertical cell spacing, or 0 when
// border-collapse is active.
func (t *Table) effectiveSpacingV() float64 {
	if t.borderCollapse {
		return 0
	}
	return t.cellSpacingV
}

// totalSpacingH returns the total horizontal space consumed by cell spacing.
// For N columns there are N+1 gaps (left edge, between each pair, right edge).
func (t *Table) totalSpacingH(nCols int) float64 {
	sh := t.effectiveSpacingH()
	if sh == 0 || nCols == 0 {
		return 0
	}
	return float64(nCols+1) * sh
}

// SetMinWidth sets the minimum total table width in points.
// When auto-sizing, if the content is narrower than this, columns are
// expanded proportionally to fill the minimum width.
func (t *Table) SetMinWidth(pts float64) *Table {
	t.minWidth = pts
	return t
}

// SetMinWidthUnit sets the minimum table width as a UnitValue, resolved
// lazily at layout time. Use Pct(100) for width:100%.
func (t *Table) SetMinWidthUnit(u UnitValue) *Table {
	t.minWidthUnit = &u
	return t
}

// SetAutoColumnWidths enables automatic column width calculation based
// on cell content. Columns are sized proportionally to their content's
// natural width, constrained to fit within the available space.
// Each column gets at least its MinWidth (longest word) and at most
// its MaxWidth (full content width without wrapping).
func (t *Table) SetAutoColumnWidths() *Table {
	t.autoWidths = true
	t.colWidths = nil
	t.colUnitWidths = nil
	return t
}

// SetColumnUnitWidths sets column widths using UnitValues, allowing
// a mix of point and percentage widths.
//
// Example:
//
//	table.SetColumnUnitWidths([]UnitValue{Pct(30), Pct(70)})
//	table.SetColumnUnitWidths([]UnitValue{Pt(100), Pct(50), Pt(100)})
func (t *Table) SetColumnUnitWidths(widths []UnitValue) *Table {
	t.colUnitWidths = widths
	t.colWidths = nil
	t.autoWidths = false
	return t
}

// AddRow adds a new row to the table and returns it for adding cells.
func (t *Table) AddRow() *Row {
	r := &Row{}
	t.rows = append(t.rows, r)
	return r
}

// AddHeaderRow adds a row that will be repeated on each new page
// when the table breaks across pages.
func (t *Table) AddHeaderRow() *Row {
	r := &Row{isHeader: true}
	t.rows = append(t.rows, r)
	return r
}

// AddFooterRow adds a row that will be repeated at the bottom of each
// page when the table breaks across pages.
func (t *Table) AddFooterRow() *Row {
	r := &Row{isFooter: true}
	t.rows = append(t.rows, r)
	return r
}

// numCols returns the number of columns by examining all rows,
// accounting for colspan.
func (t *Table) numCols() int {
	maxCols := 0
	for _, row := range t.rows {
		cols := 0
		for _, c := range row.cells {
			cols += c.colspan
		}
		if cols > maxCols {
			maxCols = cols
		}
	}
	return maxCols
}

// resolveColWidths computes column widths.
// Priority: auto > UnitValue > point widths > equal distribution.
func (t *Table) resolveColWidths(maxWidth float64) []float64 {
	nCols := t.numCols()
	if nCols == 0 {
		return nil
	}

	// Subtract horizontal cell spacing so columns fill the remaining space.
	availW := maxWidth - t.totalSpacingH(nCols)
	if availW < 0 {
		availW = 0
	}

	// Auto-sizing from cell content.
	if t.autoWidths {
		return t.computeAutoWidths(nCols, availW)
	}

	// UnitValue widths (supports mixed point/percent).
	if len(t.colUnitWidths) >= nCols {
		return ResolveAll(t.colUnitWidths[:nCols], availW)
	}

	// Explicit point widths.
	if len(t.colWidths) >= nCols {
		return t.colWidths[:nCols]
	}

	// Equal distribution.
	w := availW / float64(nCols)
	widths := make([]float64, nCols)
	for i := range widths {
		widths[i] = w
	}
	return widths
}

// computeAutoWidths sizes columns based on cell content.
// Algorithm:
//  1. Measure each cell's MinWidth (longest word) and MaxWidth (single line).
//  2. For each column, take the max of all cells' widths in that column.
//  3. If total MaxWidth fits, use MaxWidths (no wrapping needed).
//  4. If total MinWidth exceeds available, use MinWidths (can't do better).
//  5. Otherwise, distribute the extra space proportionally to each column's
//     desire (MaxWidth - MinWidth).
func (t *Table) computeAutoWidths(nCols int, maxWidth float64) []float64 {
	colMin := make([]float64, nCols)
	colMax := make([]float64, nCols)

	// First pass: measure single-column cells.
	for _, row := range t.rows {
		col := 0
		for _, cell := range row.cells {
			if col >= nCols {
				break
			}
			if cell.colspan > 1 {
				col += cell.colspan
				continue
			}

			minW, maxW := cellIntrinsicWidths(cell)
			if minW > colMin[col] {
				colMin[col] = minW
			}
			if maxW > colMax[col] {
				colMax[col] = maxW
			}
			col++
		}
	}

	// Second pass: distribute colspan cell widths across spanned columns.
	for _, row := range t.rows {
		col := 0
		for _, cell := range row.cells {
			if col >= nCols {
				break
			}
			span := cell.colspan
			if span <= 1 {
				col++
				continue
			}
			if col+span > nCols {
				span = nCols - col
			}

			cellMin, cellMax := cellIntrinsicWidths(cell)

			// Sum current column sizes for the spanned range.
			spanMin, spanMax := 0.0, 0.0
			for c := col; c < col+span; c++ {
				spanMin += colMin[c]
				spanMax += colMax[c]
			}

			// If the colspan cell needs more space, distribute the deficit evenly.
			if cellMin > spanMin {
				deficit := cellMin - spanMin
				per := deficit / float64(span)
				for c := col; c < col+span; c++ {
					colMin[c] += per
				}
			}
			if cellMax > spanMax {
				deficit := cellMax - spanMax
				per := deficit / float64(span)
				for c := col; c < col+span; c++ {
					colMax[c] += per
				}
			}

			col += span
		}
	}

	// Sum up totals.
	totalMin := 0.0
	totalMax := 0.0
	for i := range nCols {
		totalMin += colMin[i]
		totalMax += colMax[i]
	}

	widths := make([]float64, nCols)

	// Resolve table-level minWidth (prefer lazy UnitValue).
	resolvedMinWidth := t.minWidth
	if t.minWidthUnit != nil {
		resolvedMinWidth = t.minWidthUnit.Resolve(maxWidth)
	}

	// Apply table-level minWidth: if content is narrower, expand proportionally.
	if resolvedMinWidth > 0 && totalMax < resolvedMinWidth && resolvedMinWidth <= maxWidth {
		// Scale columns proportionally so total = resolvedMinWidth.
		scale := resolvedMinWidth / totalMax
		for i := range nCols {
			widths[i] = colMax[i] * scale
		}
		return widths
	}

	if totalMax <= maxWidth {
		// Everything fits at max width — no wrapping.
		copy(widths, colMax)
	} else if totalMin >= maxWidth {
		// Can't even fit minimums — use them and accept overflow.
		copy(widths, colMin)
	} else {
		// Distribute extra space proportionally.
		extra := maxWidth - totalMin
		totalDesire := totalMax - totalMin
		for i := range nCols {
			desire := colMax[i] - colMin[i]
			if totalDesire > 0 {
				widths[i] = colMin[i] + extra*(desire/totalDesire)
			} else {
				widths[i] = colMin[i]
			}
		}
	}

	return widths
}

// cellIntrinsicWidths returns the min and max content widths for a cell,
// accounting for padding.
func cellIntrinsicWidths(cell *Cell) (minW, maxW float64) {
	pad := cell.padLeft() + cell.padRight()

	// CSS width hint on the cell overrides content measurement.
	if cell.hintW > 0 {
		return cell.hintW, cell.hintW
	}

	// Rich cell content.
	if cell.content != nil {
		if m, ok := cell.content.(Measurable); ok {
			return m.MinWidth() + pad, m.MaxWidth() + pad
		}
		return pad, pad
	}

	// Plain text cell.
	measurer := cellTextMeasurer(cell)
	if measurer == nil || cell.text == "" {
		return pad, pad
	}

	words := splitWords(cell.text)
	spaceW := measurer.MeasureString(" ", cell.fontSize)

	// MinWidth: longest single word.
	for _, w := range words {
		ww := measurer.MeasureString(w, cell.fontSize)
		if ww+pad > minW {
			minW = ww + pad
		}
	}

	// MaxWidth: all words on a single line.
	lineW := 0.0
	for i, w := range words {
		lineW += measurer.MeasureString(w, cell.fontSize)
		if i < len(words)-1 {
			lineW += spaceW
		}
	}
	maxW = lineW + pad

	return minW, maxW
}

// gridCell is a cell positioned in the flat grid.
type gridCell struct {
	cell      *Cell
	col       int     // starting column index
	spanWidth float64 // total width across spanned columns
}

// gridRow is a row in the flat grid with computed height.
type gridRow struct {
	cells    []gridCell
	height   float64
	isHeader bool
	isFooter bool
}

// buildGrid converts the builder rows into a flat grid with computed sizes.
func (t *Table) buildGrid(colWidths []float64) []gridRow {
	nCols := len(colWidths)
	// Track which cells in the grid are occupied (by rowspan from above).
	// occupied[row][col] = true if occupied by a spanning cell from a previous row.
	// We build this dynamically as we process rows.
	// colOccupied tracks how many more rows each column is occupied for.
	colOccupied := make([]int, nCols)

	var grid []gridRow

	for _, row := range t.rows {
		gr := gridRow{isHeader: row.isHeader, isFooter: row.isFooter}
		cellIdx := 0
		col := 0

		for col < nCols && cellIdx < len(row.cells) {
			// Skip columns occupied by rowspan from above.
			for col < nCols && colOccupied[col] > 0 {
				colOccupied[col]--
				col++
			}
			if col >= nCols {
				break
			}
			if cellIdx >= len(row.cells) {
				break
			}

			cell := row.cells[cellIdx]
			colspan := min(cell.colspan, nCols-col)

			// Compute span width.
			spanW := 0.0
			for c := col; c < col+colspan; c++ {
				spanW += colWidths[c]
			}

			gr.cells = append(gr.cells, gridCell{
				cell:      cell,
				col:       col,
				spanWidth: spanW,
			})

			// Mark rowspan occupancy for future rows.
			if cell.rowspan > 1 {
				for c := col; c < col+colspan; c++ {
					colOccupied[c] = cell.rowspan - 1
				}
			}

			col += colspan
			cellIdx++
		}

		// Decrement rowspan occupancy for any remaining columns not visited.
		for col < nCols {
			if colOccupied[col] > 0 {
				colOccupied[col]--
			}
			col++
		}

		// Compute row height: tallest cell content + padding.
		maxH := 0.0
		for i := range gr.cells {
			h := t.cellContentHeight(&gr.cells[i])
			if h > maxH {
				maxH = h
			}
		}
		gr.height = maxH
		grid = append(grid, gr)
	}

	return grid
}

// cellContentHeight computes the height needed for a cell's content.
// For Element cells, it also caches the laid-out lines in gc.
func (t *Table) cellContentHeight(gc *gridCell) float64 {
	cell := gc.cell
	padH := cell.padTop() + cell.padBottom()
	innerWidth := gc.spanWidth - cell.padLeft() - cell.padRight()
	if innerWidth <= 0 {
		return padH
	}

	// Element-based cell: delegate to the Element's PlanLayout.
	if cell.content != nil {
		return measureConsumed(cell.content, innerWidth) + padH
	}

	measurer := t.cellMeasurer(cell)
	if measurer == nil {
		return padH
	}

	// Count wrapped lines.
	words := strings.Fields(cell.text)
	if len(words) == 0 {
		return cell.fontSize*1.2 + padH
	}

	lines := 1
	lineWidth := measurer.MeasureString(words[0], cell.fontSize)
	spaceW := measurer.MeasureString(" ", cell.fontSize)

	for i := 1; i < len(words); i++ {
		wordW := measurer.MeasureString(words[i], cell.fontSize)
		if lineWidth+spaceW+wordW > innerWidth {
			lines++
			lineWidth = wordW
		} else {
			lineWidth += spaceW + wordW
		}
	}

	return float64(lines)*cell.fontSize*1.2 + padH
}

// cellMeasurer returns the text measurer for a cell's font, or nil if none is set.
func (t *Table) cellMeasurer(cell *Cell) font.TextMeasurer {
	if cell.embedded != nil {
		return cell.embedded
	}
	if cell.font != nil {
		return cell.font
	}
	return nil
}

// Layout implements the Element interface.
func (t *Table) Layout(maxWidth float64) []Line {
	// Tables don't use the Line-based layout. They render directly
	// via the tableLayout method. We return a single synthetic line
	// with the total table height so the renderer knows how much
	// vertical space is consumed.
	//
	// The actual rendering happens in RenderTable, called by the renderer.
	colWidths := t.resolveColWidths(maxWidth)
	grid := t.buildGrid(colWidths)
	if t.borderCollapse {
		collapseBorders(grid)
	}

	sv := t.effectiveSpacingV()

	// Return one "line" per grid row so the renderer can page-break between rows.
	// Each line's height includes the spacing gap before the row (and the
	// bottom-edge gap is added to the last row).
	lines := make([]Line, len(grid))
	for i, gr := range grid {
		h := gr.height + sv // gap before this row
		if i == len(grid)-1 {
			h += sv // bottom edge after last row
		}
		lines[i] = Line{
			Height: h,
			IsLast: i == len(grid)-1,
			Align:  AlignLeft,
			tableRow: &tableRowRef{
				table:     t,
				grid:      grid,
				rowIndex:  i,
				colWidths: colWidths,
				maxWidth:  maxWidth,
			},
		}
	}
	return lines
}

// PlanLayout implements Element. Tables split between rows, repeating
// header rows on each new page.
func (t *Table) PlanLayout(area LayoutArea) LayoutPlan {
	colWidths := t.resolveColWidths(area.Width)
	grid := t.buildGrid(colWidths)
	if len(grid) == 0 {
		return LayoutPlan{Status: LayoutFull}
	}

	// Apply border-collapse: remove duplicate borders between adjacent cells.
	if t.borderCollapse {
		collapseBorders(grid)
	}

	// Identify header rows (at start) and footer rows (at end).
	var headerHeight float64
	var headerRowCount int
	for _, gr := range grid {
		if gr.isHeader {
			headerHeight += gr.height
			headerRowCount++
		} else {
			break
		}
	}

	var footerHeight float64
	var footerRowCount int
	for i := len(grid) - 1; i >= 0; i-- {
		if grid[i].isFooter {
			footerHeight += grid[i].height
			footerRowCount++
		} else {
			break
		}
	}

	bodyEnd := len(grid) - footerRowCount // index where body rows end

	sv := t.effectiveSpacingV()

	// Build blocks row by row, checking height.
	var blocks []PlacedBlock
	curY := 0.0
	splitIdx := len(grid)

	for i, gr := range grid {
		// Skip footer rows in the main loop; they'll be appended at the end.
		if gr.isFooter {
			continue
		}

		// Add vertical spacing before this row.
		curY += sv

		// Check if this body row fits (reserve space for footer if splitting).
		needsFooter := footerRowCount > 0 && i > headerRowCount
		reserveH := 0.0
		if needsFooter {
			reserveH = footerHeight
		}
		if curY+gr.height+reserveH > area.Height && area.Height > 0 && i > headerRowCount {
			splitIdx = i
			break
		}

		capturedGrid := grid
		capturedRowIdx := i
		capturedColWidths := colWidths
		capturedMaxW := area.Width
		capturedTable := t

		blocks = append(blocks, PlacedBlock{
			X: 0, Y: curY, Width: area.Width, Height: gr.height,
			Tag: "TR",
			Draw: func(ctx DrawContext, absX, absTopY float64) {
				drawTableRowDirect(ctx, capturedTable, capturedGrid, capturedRowIdx, capturedColWidths, capturedMaxW, absX, absTopY)
			},
		})
		curY += gr.height
	}

	// Bottom edge spacing after last body row.
	curY += sv

	// Append footer rows at the bottom (whether splitting or not).
	if footerRowCount > 0 {
		for i := bodyEnd; i < len(grid); i++ {
			gr := grid[i]
			capturedGrid := grid
			capturedRowIdx := i
			capturedColWidths := colWidths
			capturedMaxW := area.Width
			capturedTable := t

			blocks = append(blocks, PlacedBlock{
				X: 0, Y: curY, Width: area.Width, Height: gr.height,
				Tag: "TR",
				Draw: func(ctx DrawContext, absX, absTopY float64) {
					drawTableRowDirect(ctx, capturedTable, capturedGrid, capturedRowIdx, capturedColWidths, capturedMaxW, absX, absTopY)
				},
			})
			curY += gr.height
		}
	}

	// Wrap all row blocks in a parent "Table" block for structure tree nesting.
	wrapBlocks := func(rowBlocks []PlacedBlock, height float64) []PlacedBlock {
		if len(rowBlocks) == 0 {
			return rowBlocks
		}
		return []PlacedBlock{{
			X: 0, Y: 0, Width: area.Width, Height: height,
			Tag:      "Table",
			Children: rowBlocks,
		}}
	}

	if splitIdx >= bodyEnd {
		return LayoutPlan{Status: LayoutFull, Consumed: curY, Blocks: wrapBlocks(blocks, curY)}
	}

	// Build overflow table with header + footer rows + remaining data rows.
	overflowTable := &Table{
		colWidths:      t.colWidths,
		colUnitWidths:  t.colUnitWidths,
		borderCollapse: t.borderCollapse,
		cellSpacingH:   t.cellSpacingH,
		cellSpacingV:   t.cellSpacingV,
	}
	// Re-add header rows.
	for _, row := range t.rows {
		if row.isHeader {
			overflowTable.rows = append(overflowTable.rows, row)
		}
	}
	// Add remaining data rows (skip headers/footers + already-rendered rows).
	dataRowIdx := 0
	for _, row := range t.rows {
		if row.isHeader || row.isFooter {
			continue
		}
		renderedDataRows := splitIdx - headerRowCount
		if dataRowIdx >= renderedDataRows {
			overflowTable.rows = append(overflowTable.rows, row)
		}
		dataRowIdx++
	}
	// Re-add footer rows.
	for _, row := range t.rows {
		if row.isFooter {
			overflowTable.rows = append(overflowTable.rows, row)
		}
	}

	return LayoutPlan{
		Status: LayoutPartial, Consumed: curY, Blocks: wrapBlocks(blocks, curY), Overflow: overflowTable,
	}
}

// tableRowRef is internal data attached to a Line so the renderer
// can call back into the table to render that specific row.
type tableRowRef struct {
	table     *Table
	grid      []gridRow
	rowIndex  int
	colWidths []float64
	maxWidth  float64
}

// collapseBorders removes duplicate borders between adjacent cells.
// For each cell: remove the right border (except last column) and
// the bottom border (except last row). The adjacent cell's left/top
// border serves as the shared border.
func collapseBorders(grid []gridRow) {
	nCols := 0
	for _, gr := range grid {
		cols := 0
		for _, gc := range gr.cells {
			cols += max(gc.cell.colspan, 1)
		}
		if cols > nCols {
			nCols = cols
		}
	}

	for rowIdx, gr := range grid {
		for cellIdx, gc := range gr.cells {
			// Remove right border unless this is the last cell in the row.
			isLastCol := cellIdx == len(gr.cells)-1
			if !isLastCol {
				gc.cell.borders.Right = Border{}
			}

			// Remove bottom border unless this is the last row.
			isLastRow := rowIdx == len(grid)-1
			if !isLastRow {
				gc.cell.borders.Bottom = Border{}
			}
		}
	}
}

// drawCellBorders draws the four borders of a cell.
func drawCellBorders(stream *content.Stream, borders CellBorders, x, y, w, h float64) {
	// Top border: from top-left to top-right
	drawStyledBorder(stream, borders.Top, x, y+h, x+w, y+h)
	// Bottom border: from bottom-left to bottom-right
	drawStyledBorder(stream, borders.Bottom, x, y, x+w, y)
	// Left border: from bottom-left to top-left
	drawStyledBorder(stream, borders.Left, x, y, x, y+h)
	// Right border: from bottom-right to top-right
	drawStyledBorder(stream, borders.Right, x+w, y, x+w, y+h)
}

// drawStyledBorder draws a single border line with the appropriate style.
func drawStyledBorder(stream *content.Stream, b Border, x1, y1, x2, y2 float64) {
	if b.Width <= 0 || b.Style == BorderNone {
		return
	}

	stream.SaveState()
	setStrokeColor(stream, b.Color)

	switch b.Style {
	case BorderDashed:
		stream.SetLineWidth(b.Width)
		dash := max(b.Width*3, 3.0)
		gap := max(b.Width*2, 2.0)
		stream.SetDashPattern([]float64{dash, gap}, 0)
		stream.MoveTo(x1, y1)
		stream.LineTo(x2, y2)
		stream.Stroke()

	case BorderDotted:
		stream.SetLineWidth(b.Width)
		stream.SetLineCap(1) // round cap makes dots circular
		dot := b.Width
		gap := max(b.Width*2, 2.0)
		stream.SetDashPattern([]float64{dot, gap}, 0)
		stream.MoveTo(x1, y1)
		stream.LineTo(x2, y2)
		stream.Stroke()

	case BorderDouble:
		// Draw two lines separated by a gap equal to the line width.
		// Total visual width = 3 * b.Width (line + gap + line).
		offset := b.Width
		stream.SetLineWidth(b.Width)
		// Determine direction for offset (perpendicular to the line).
		if x1 == x2 {
			// Vertical line: offset horizontally.
			stream.MoveTo(x1-offset, y1)
			stream.LineTo(x2-offset, y2)
			stream.Stroke()
			stream.MoveTo(x1+offset, y1)
			stream.LineTo(x2+offset, y2)
			stream.Stroke()
		} else {
			// Horizontal line: offset vertically.
			stream.MoveTo(x1, y1-offset)
			stream.LineTo(x2, y2-offset)
			stream.Stroke()
			stream.MoveTo(x1, y1+offset)
			stream.LineTo(x2, y2+offset)
			stream.Stroke()
		}

	default: // BorderSolid
		stream.SetLineWidth(b.Width)
		stream.MoveTo(x1, y1)
		stream.LineTo(x2, y2)
		stream.Stroke()
	}

	stream.RestoreState()
}

// cellTextMeasurer returns the text measurer for a cell's font, or nil if none is set.
func cellTextMeasurer(cell *Cell) font.TextMeasurer {
	if cell.embedded != nil {
		return cell.embedded
	}
	if cell.font != nil {
		return cell.font
	}
	return nil
}

// IsTable reports whether a Line was produced by a Table element.
func (l *Line) IsTable() bool {
	return l.tableRow != nil
}

// drawTableRowDirect renders a table row directly using draw.go functions,
// without going through the old Renderer emit methods.
func drawTableRowDirect(ctx DrawContext, tbl *Table, grid []gridRow, rowIndex int, colWidths []float64, maxWidth, x, topY float64) {
	gr := grid[rowIndex]

	sh := tbl.effectiveSpacingH()

	for _, gc := range gr.cells {
		// Start after the left-edge spacing gap, then advance past
		// each preceding column plus its inter-column gap.
		cellX := x + sh
		for c := range gc.col {
			cellX += colWidths[c] + sh
		}
		cellBottomY := topY - gr.height

		// Background fill.
		if gc.cell.bgColor != nil {
			drawBackground(ctx, *gc.cell.bgColor, cellX, topY, gc.spanWidth, gr.height)
		}

		// Borders.
		drawCellBorders(ctx.Stream, gc.cell.borders, cellX, cellBottomY, gc.spanWidth, gr.height)

		// Cell content.
		if gc.cell.content != nil {
			drawCellElementDirect(ctx, gc, cellX, topY, gr.height)
		} else {
			drawCellTextDirect(ctx, gc.cell, cellX, topY, gc.spanWidth, gr.height)
		}
	}
}

// drawCellTextDirect renders plain text cell content using draw.go functions.
func drawCellTextDirect(ctx DrawContext, cell *Cell, cellX, cellTopY, cellW, cellH float64) {
	if cell.text == "" {
		return
	}

	measurer := cellTextMeasurer(cell)
	if measurer == nil {
		return
	}

	innerW := cellW - cell.padLeft() - cell.padRight()
	if innerW <= 0 {
		return
	}

	words := strings.Fields(cell.text)
	if len(words) == 0 {
		return
	}

	// Word wrap.
	type textLine struct {
		words []Word
		width float64
	}

	spaceW := measurer.MeasureString(" ", cell.fontSize)
	var textLines []textLine
	var curWords []Word
	curWidth := 0.0

	for _, w := range words {
		wordW := measurer.MeasureString(w, cell.fontSize)
		word := Word{
			Text: w, Width: wordW, Font: cell.font,
			Embedded: cell.embedded, FontSize: cell.fontSize,
			SpaceAfter: spaceW,
		}
		if curWidth > 0 && curWidth+spaceW+wordW > innerW {
			textLines = append(textLines, textLine{curWords, curWidth})
			curWords = []Word{word}
			curWidth = wordW
		} else {
			if curWidth > 0 {
				curWidth += spaceW
			}
			curWords = append(curWords, word)
			curWidth += wordW
		}
	}
	if len(curWords) > 0 {
		textLines = append(textLines, textLine{curWords, curWidth})
	}

	lineHeight := cell.fontSize * 1.2

	// Vertical alignment.
	totalTextH := float64(len(textLines)) * lineHeight
	innerH := cellH - cell.padTop() - cell.padBottom()
	vOffset := 0.0
	switch cell.valign {
	case VAlignMiddle:
		vOffset = (innerH - totalTextH) / 2
	case VAlignBottom:
		vOffset = innerH - totalTextH
	}
	if vOffset < 0 {
		vOffset = 0
	}

	for i, tl := range textLines {
		baselineY := cellTopY - cell.padTop() - vOffset - float64(i+1)*lineHeight + (lineHeight-cell.fontSize)/2

		var textX float64
		switch cell.align {
		case AlignCenter:
			textX = cellX + cell.padLeft() + (innerW-tl.width)/2
		case AlignRight:
			textX = cellX + cell.padLeft() + innerW - tl.width
		default:
			textX = cellX + cell.padLeft()
		}

		drawTextLine(ctx, tl.words, textX, baselineY, innerW, cell.align, i == len(textLines)-1)
	}
}

// drawCellElementDirect renders a rich cell (Element content) using the plan system.
func drawCellElementDirect(ctx DrawContext, gc gridCell, cellX, topY, rowHeight float64) {
	cell := gc.cell
	innerW := gc.spanWidth - cell.padLeft() - cell.padRight()

	plan := cell.content.PlanLayout(LayoutArea{Width: innerW, Height: 1e9})

	totalH := plan.Consumed
	innerH := rowHeight - cell.padTop() - cell.padBottom()
	vOffset := 0.0
	switch cell.valign {
	case VAlignMiddle:
		vOffset = (innerH - totalH) / 2
	case VAlignBottom:
		vOffset = innerH - totalH
	}
	if vOffset < 0 {
		vOffset = 0
	}

	contentX := cellX + cell.padLeft()
	curY := topY - cell.padTop() - vOffset

	for _, block := range plan.Blocks {
		bx := contentX + block.X
		by := curY - block.Y
		if block.Draw != nil {
			block.Draw(ctx, bx, by)
		}
		for _, child := range block.Children {
			drawBlockRecursive(child, bx, by, ctx)
		}
	}
}

// drawBlockRecursive draws a PlacedBlock and its children.
func drawBlockRecursive(block PlacedBlock, baseX, topY float64, ctx DrawContext) {
	bx := baseX + block.X
	by := topY - block.Y
	if block.Draw != nil {
		block.Draw(ctx, bx, by)
	}
	for _, child := range block.Children {
		drawBlockRecursive(child, bx, by, ctx)
	}
	if block.PostDraw != nil {
		block.PostDraw(ctx, bx, by)
	}
}
