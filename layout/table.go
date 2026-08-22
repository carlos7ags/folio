// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"slices"
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
	BorderHidden                    // CSS2.1 §17.6.2: never draws, and always wins a border conflict
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
	text         string
	font         *font.Standard
	embedded     *font.EmbeddedFont
	fontSize     float64
	content      Element // rich content (if non-nil, overrides text/font)
	align        Align
	valign       VAlign
	padding      float64  // uniform padding (all sides)
	padSides     *Padding // per-side padding (overrides uniform when set)
	borders      CellBorders
	colspan      int
	rowspan      int
	bgColor      *Color     // background fill color (nil = transparent)
	hintW        float64    // CSS width hint in points (0 = not set)
	hintWUnit    *UnitValue // lazy width hint (overrides hintW when set; resolved at layout time)
	borderRadius [4]float64 // corner radii [TL, TR, BR, BL] (points, 0 = sharp)
}

// SetWidthHint sets the CSS width hint for this cell (in points).
// Used by auto-sizing to influence column width allocation.
func (c *Cell) SetWidthHint(pts float64) *Cell {
	c.hintW = pts
	c.hintWUnit = nil
	return c
}

// SetWidthHintUnit sets a lazy width hint resolved at layout time against
// the column track's available width. Use this instead of SetWidthHint for
// percentage widths (e.g. <td style="width:50%">) so the hint resolves
// against the table's actual maxWidth instead of whatever container width
// was current at convert time. Without lazy resolution, a cell inside a
// narrow flex column can resolve 50% against the outer page width and
// overflow the column on render.
func (c *Cell) SetWidthHintUnit(u UnitValue) *Cell {
	c.hintWUnit = &u
	c.hintW = 0
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

// SetBorderRadius sets a uniform corner radius on all four corners.
func (c *Cell) SetBorderRadius(r float64) *Cell {
	c.borderRadius = [4]float64{r, r, r, r}
	return c
}

// SetBorderRadiusPerCorner sets per-corner radii: top-left, top-right,
// bottom-right, bottom-left (matching CSS border-radius order).
func (c *Cell) SetBorderRadiusPerCorner(tl, tr, br, bl float64) *Cell {
	c.borderRadius = [4]float64{tl, tr, br, bl}
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

// Content returns the cell's rich content Element, or nil if the cell holds
// plain text instead. When the cell owns and draws the box fill (background +
// border-radius), callers use this to detect and clear a redundant
// block-level background on the content element (see issue #329).
func (c *Cell) Content() Element { return c.content }

// Background returns a copy of the cell's background fill color, or nil if the
// cell has no background. A copy is returned so callers cannot mutate the
// cell's internal fill through the pointer. Provided for testing.
func (c *Cell) Background() *Color {
	if c.bgColor == nil {
		return nil
	}
	col := *c.bgColor
	return &col
}

// BorderRadii returns the cell's per-corner radii in CSS order
// (top-left, top-right, bottom-right, bottom-left), in points. A value of 0
// means the corner is sharp. Provided for testing.
func (c *Cell) BorderRadii() [4]float64 { return c.borderRadius }

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
		text:     normalizeText(text),
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
		text:     normalizeText(text),
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
	direction      Direction   // text direction; RTL reverses column order

	// resolveColWidths memo, keyed by the maxWidth it was computed at.
	// A table spanning N pages would otherwise re-measure every cell's
	// text on every continuation page (each continuation table contains
	// ALL remaining rows), giving O(rows²) text measurement overall.
	// cloneForOverflow copies these fields (via whole-struct copy) so a
	// continuation table reuses the original's resolved widths — which is
	// also the correct behavior: columns must not shift between pages of
	// the same table. Invalidated by any mutator that can change sizing
	// (AddRow and friends, column-width/min-width setters).
	cachedColWidths   []float64
	cachedColWidthsAt float64
	colWidthsValid    bool
}

// NewTable creates a new empty table.
func NewTable() *Table {
	return &Table{}
}

// Rows returns the table's rows in order (header, body, and footer rows are
// all stored in this single slice). Provided for testing.
func (t *Table) Rows() []*Row { return t.rows }

// Cells returns the row's cells in order. Provided for testing.
func (r *Row) Cells() []*Cell { return r.cells }

// SetColumnWidths sets explicit column widths in points.
// If not set, columns are distributed equally within the available width.
func (t *Table) SetColumnWidths(widths []float64) *Table {
	t.colWidths = widths
	t.colUnitWidths = nil
	t.autoWidths = false
	t.colWidthsValid = false
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

// SetDirection sets the text direction for the table. When RTL, columns
// are rendered right-to-left: column 0 appears at the right edge of the
// table and the last column at the left edge. Cell content paragraphs
// also inherit the direction for bidi text reordering.
func (t *Table) SetDirection(d Direction) *Table {
	t.direction = d
	return t
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

// cloneForOverflow returns a shallow copy of t with no rows, used by
// PlanLayout to build the continuation table when a page break splits
// this table. Every sizing field (autoWidths, minWidth, minWidthUnit,
// colWidths, colUnitWidths, border-collapse, cell spacing) is inherited
// so column widths on the continuation page exactly match the first
// page. Without this, a table that relied on auto-sizing from per-cell
// width hints — notably `<th style="width:N%">` in the HTML converter,
// which calls SetAutoColumnWidths + SetWidthHint — would silently fall
// back to equal-distribution column widths after a page break and
// visibly shift between pages. Rows are rebuilt by the caller from
// header + remaining body + footer rows.
//
// When adding a new field to Table, no change is needed here: the
// whole struct is copied. Tests in TestTableOverflowPreserves* guard
// the invariant.
func (t *Table) cloneForOverflow() *Table {
	clone := *t
	clone.rows = nil
	return &clone
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
	t.colWidthsValid = false
	return t
}

// SetMinWidthUnit sets the minimum table width as a UnitValue, resolved
// lazily at layout time. Use Pct(100) for width:100%.
func (t *Table) SetMinWidthUnit(u UnitValue) *Table {
	t.minWidthUnit = &u
	t.colWidthsValid = false
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
	t.colWidthsValid = false
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
	t.colWidthsValid = false
	return t
}

// AddRow adds a new row to the table and returns it for adding cells.
func (t *Table) AddRow() *Row {
	r := &Row{}
	t.rows = append(t.rows, r)
	t.colWidthsValid = false
	return r
}

// AddHeaderRow adds a row that will be repeated on each new page
// when the table breaks across pages.
func (t *Table) AddHeaderRow() *Row {
	r := &Row{isHeader: true}
	t.rows = append(t.rows, r)
	t.colWidthsValid = false
	return r
}

// AddFooterRow adds a row that will be repeated at the bottom of each
// page when the table breaks across pages.
func (t *Table) AddFooterRow() *Row {
	r := &Row{isFooter: true}
	t.rows = append(t.rows, r)
	t.colWidthsValid = false
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
	if t.colWidthsValid && t.cachedColWidthsAt == maxWidth {
		// Return a copy: callers must not be able to mutate the memo
		// through the returned slice.
		return slices.Clone(t.cachedColWidths)
	}

	widths := t.resolveColWidthsUncached(maxWidth)

	t.cachedColWidths = slices.Clone(widths)
	t.cachedColWidthsAt = maxWidth
	t.colWidthsValid = true
	return widths
}

// resolveColWidthsUncached does the actual width-resolution work; see
// resolveColWidths for the memoization wrapper.
func (t *Table) resolveColWidthsUncached(maxWidth float64) []float64 {
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

			minW, maxW := cellIntrinsicWidths(cell, maxWidth)
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

			cellMin, cellMax := cellIntrinsicWidths(cell, maxWidth)

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
// accounting for padding. availWidth is the table's available width, used
// to resolve lazy UnitValue hints (percentages) against the actual table
// size rather than whatever container width was current when the cell
// was constructed.
func cellIntrinsicWidths(cell *Cell, availWidth float64) (minW, maxW float64) {
	pad := cell.padLeft() + cell.padRight()

	// Lazy UnitValue hint (e.g. percentage) resolves against the
	// table's actual maxWidth. This is the path used by the HTML
	// converter for `<td style="width:N%">`.
	if cell.hintWUnit != nil {
		resolved := cell.hintWUnit.Resolve(availWidth)
		if resolved > 0 {
			// Clip to available width so a badly-sized hint can't
			// push total column width past the table's maxWidth.
			if resolved > availWidth {
				resolved = availWidth
			}
			return resolved, resolved
		}
	}

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
	cell         *Cell
	col          int         // starting column index
	row          int         // starting row index (used by resolveBorders to index segment slices of a neighbor across a span)
	spanWidth    float64     // total width across spanned columns
	spanHeight   float64     // total height across spanned rows (0 = single row)
	colSpanCount int         // occupied columns, clipped to table width
	rowSpanCount int         // occupied rows, clipped to table row count (1 = single row)
	resolved     CellBorders // per-render resolved borders under border-collapse; never copied back into cell.borders

	// Per-segment resolved borders under border-collapse, populated by
	// resolveBorders alongside the whole-edge `resolved` fields above.
	// segTop/segBottom have length colSpanCount (one winner per spanned
	// column); segLeft/segRight have length rowSpanCount (one winner per
	// spanned row). For a single-row/single-column cell each slice has
	// exactly one element and mirrors `resolved`. Never copied into
	// cell.borders; recomputed fresh on every resolveBorders call.
	segTop, segBottom []Border
	segLeft, segRight []Border
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

	for rIdx, row := range t.rows {
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

			// Rowspan clipped to the table's actual row count, so a
			// spanning cell's true bottom edge (used by resolveBorders)
			// is never mistaken for an earlier row.
			rowspan := cell.rowspan
			if rowspan < 1 {
				rowspan = 1
			}
			rowSpanCount := min(rowspan, len(t.rows)-rIdx)

			// Compute span width.
			spanW := 0.0
			for c := col; c < col+colspan; c++ {
				spanW += colWidths[c]
			}

			gr.cells = append(gr.cells, gridCell{
				cell:         cell,
				col:          col,
				row:          rIdx,
				spanWidth:    spanW,
				colSpanCount: colspan,
				rowSpanCount: rowSpanCount,
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

		// Compute row height: tallest cell content + padding. Rowspanning
		// cells are excluded here so they don't inflate their starting row;
		// their height is distributed across the spanned rows below.
		maxH := 0.0
		for i := range gr.cells {
			h := t.cellContentHeight(&gr.cells[i])
			if gr.cells[i].cell.rowspan > 1 {
				continue
			}
			if h > maxH {
				maxH = h
			}
		}
		gr.height = maxH
		grid = append(grid, gr)
	}

	t.resolveRowspanHeights(grid)
	return grid
}

// resolveRowspanHeights computes spanHeight for every rowspanning cell once
// all natural row heights are known. If a spanning cell needs more height than
// its spanned rows provide, the deficit grows the last spanned row; spanHeight
// is then the total covered height (spanned row heights plus the spacing gaps
// between them).
//
// Note: PlanLayout keeps a rowspan and every row it covers together as one
// atomic group when splitting across a page break (issue #362); a group
// taller than a full page falls back to drawing past the page bottom, same
// as an oversized plain row.
func (t *Table) resolveRowspanHeights(grid []gridRow) {
	sv := t.effectiveSpacingV()

	// Pass 1: grow the last spanned row to cover any content deficit.
	for r := range grid {
		for i := range grid[r].cells {
			gc := &grid[r].cells[i]
			if gc.cell.rowspan <= 1 {
				continue
			}
			end := min(r+gc.cell.rowspan, len(grid))
			avail := float64(end-r-1) * sv
			for k := r; k < end; k++ {
				avail += grid[k].height
			}
			if need := t.cellContentHeight(gc); need > avail {
				grid[end-1].height += need - avail
			}
		}
	}

	// Pass 2: record each spanning cell's total height from the final row
	// heights (rows may have grown in pass 1).
	for r := range grid {
		for i := range grid[r].cells {
			gc := &grid[r].cells[i]
			if gc.cell.rowspan <= 1 {
				continue
			}
			end := min(r+gc.cell.rowspan, len(grid))
			h := float64(end-r-1) * sv
			for k := r; k < end; k++ {
				h += grid[k].height
			}
			gc.spanHeight = h
		}
	}
}

// spanGroupStarts returns, for each grid row, the index of the first row of
// the atomic span group containing it. A group is the transitive closure of
// rowspan coverage: any row covered by a span belongs to the span's starting
// row's group, and a span starting inside a group extends that group. Rows
// without span coverage are their own group.
func spanGroupStarts(grid []gridRow) []int {
	starts := make([]int, len(grid))
	i := 0
	for i < len(grid) {
		end := i + 1
		for j := i; j < end; j++ {
			for _, gc := range grid[j].cells {
				if gc.cell.rowspan > 1 {
					if e := min(j+gc.cell.rowspan, len(grid)); e > end {
						end = e
					}
				}
			}
		}
		for j := i; j < end; j++ {
			starts[j] = i
		}
		i = end
	}
	return starts
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

	measurer := cellTextMeasurer(cell)
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
		resolveBorders(grid, len(colWidths))
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

// PlanLayout implements Element. Tables split between rows — never inside a
// rowspan group — repeating header rows on each new page.
func (t *Table) PlanLayout(area LayoutArea) LayoutPlan {
	if area.Height <= 0 {
		return LayoutPlan{Status: LayoutNothing}
	}

	colWidths := t.resolveColWidths(area.Width)
	grid := t.buildGrid(colWidths)
	if len(grid) == 0 {
		return LayoutPlan{Status: LayoutFull}
	}

	// Apply border-collapse: resolve competing borders on shared cell edges.
	if t.borderCollapse {
		resolveBorders(grid, len(colWidths))
	}

	// groupStarts[i] is the first row of the rowspan group containing row
	// i; a split must never land strictly inside a group (issue #362).
	groupStarts := spanGroupStarts(grid)

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

	// oversizeGroupEnd tracks the (exclusive) grid end index of a rowspan
	// group that cannot fit even on a fresh, otherwise-empty page. While a
	// row index is inside [leader, oversizeGroupEnd) the #362 keep-together
	// check below is bypassed in favor of the pre-#362 per-row check, so
	// the group is split like plain rows instead of overflowing every
	// page. oversizeGroupLeader records the leader's row index so its
	// Draw closure can cap the spanning cell's rendered height to what
	// this page actually shows (see drawTableRowDirect's capRows param).
	oversizeGroupEnd := -1
	oversizeGroupLeader := -1

	for i, gr := range grid {
		// Skip footer rows in the main loop; they'll be appended at the end.
		if gr.isFooter {
			continue
		}

		// Add vertical spacing before this row.
		curY += sv

		// Reserve footer space from the first body row on (i >= headerRowCount):
		// the first body row must be fit-checked against the footer too, or a
		// table whose header + first row + footer don't fit together spills
		// past the page's bottom margin.
		needsFooter := footerRowCount > 0 && i >= headerRowCount
		reserveH := 0.0
		if needsFooter {
			reserveH = footerHeight
		}

		// Check if this row's span group fits (reserve space for footer if
		// splitting). A row that isn't its group's leader never triggers a
		// split — the group's space was reserved when the leader was
		// checked. For a span-free row the group is just the row itself,
		// so this reduces to the plain per-row check.
		if groupStarts[i] == i {
			groupH := gr.height
			groupEnd := i + 1
			for k := i + 1; k < len(grid) && groupStarts[k] == i; k++ {
				groupH += sv + grid[k].height
				groupEnd = k + 1
			}
			groupOverflows := curY+groupH+reserveH > area.Height && area.Height > 0
			if groupOverflows && i == headerRowCount && groupEnd > i+1 {
				// The group's leader is the first body row of this
				// layout pass — i.e. this pass is starting fresh — and
				// the multi-row group still doesn't fit. It can never fit
				// on any page; fall back to per-row splitting inside it
				// rather than overflowing every page forever (issue:
				// rowspan group taller than a page). Restricted to genuine
				// multi-row groups: for a span-free first body row this
				// path would render the leader unconditionally, which is
				// exactly the header-stranding the branch below prevents.
				oversizeGroupEnd = groupEnd
				oversizeGroupLeader = i
			} else if groupOverflows && i >= headerRowCount {
				// Fit-check body rows including the FIRST one
				// (i == headerRowCount): header rows are always placed
				// here, but the first body row must still fit alongside
				// them or the header is left stranded at the page bottom
				// with its row spilling into the bottom margin. When the
				// first body row doesn't fit, splitIdx lands on it and the
				// header-stranding guard after the loop defers the whole
				// table to the next page. Header rows themselves
				// (i < headerRowCount) never trigger a split.
				splitIdx = i
				break
			}
		}

		// Per-row split check (the pre-#362 behavior), applied only to
		// rows strictly after an oversize group's leader — the leader
		// itself always renders (guaranteeing progress even if its own
		// natural row height alone were to exceed the page).
		if i < oversizeGroupEnd && i > headerRowCount {
			if curY+gr.height+reserveH > area.Height && area.Height > 0 {
				splitIdx = i
				break
			}
		}

		capturedGrid := grid
		capturedRowIdx := i
		capturedColWidths := colWidths
		capturedMaxW := area.Width
		capturedTable := t
		capturedCapRows := 0
		if i == oversizeGroupLeader {
			// This leader's group will be split before oversizeGroupEnd
			// is reached (guaranteed: a group whose total height exceeds
			// the page eventually exceeds it row by row too). Cap the
			// spanning cell's rendered height to what actually renders on
			// this page; the real split point (splitIdx) isn't known yet
			// at capture time, so the closure recomputes it against the
			// grid rows actually captured below.
			capturedCapRows = -1 // resolved to (splitIdx-i) lazily below
		}

		blocks = append(blocks, PlacedBlock{
			X: 0, Y: curY, Width: area.Width, Height: gr.height,
			Tag: "TR",
			Draw: func(ctx DrawContext, absX, absTopY float64) {
				capRows := 0
				if capturedCapRows == -1 {
					capRows = splitIdx - capturedRowIdx
				}
				drawTableRowDirect(ctx, capturedTable, capturedGrid, capturedRowIdx, capturedColWidths, capturedMaxW, absX, absTopY, capRows)
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
					drawTableRowDirect(ctx, capturedTable, capturedGrid, capturedRowIdx, capturedColWidths, capturedMaxW, absX, absTopY, 0)
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

	// Orphan guard: if the split landed on the first body row, not even one
	// body row fit alongside the header in the available height. Placing the
	// header alone would strand it at the page bottom (and the overflow table
	// re-adds the header, duplicating it), so defer the WHOLE table — header
	// included — to the next page by reporting LayoutNothing. The renderer
	// relocates it to a fresh page where the full page height is available;
	// and when already at the top of a page (a header+row taller than a whole
	// page), the renderer force-places it, so pagination cannot loop. Guarded
	// on there being body rows so a header/footer-only table still lays out.
	if splitIdx <= headerRowCount && headerRowCount < bodyEnd {
		return LayoutPlan{Status: LayoutNothing}
	}

	if splitIdx >= bodyEnd {
		return LayoutPlan{Status: LayoutFull, Consumed: curY, Blocks: wrapBlocks(blocks, curY)}
	}

	// Build overflow table with header + footer rows + remaining data rows.
	// cloneForOverflow inherits every sizing field (see its doc comment);
	// rows are rebuilt below from header + remaining body + footer.
	overflowTable := t.cloneForOverflow()
	// Re-add header rows.
	for _, row := range t.rows {
		if row.isHeader {
			overflowTable.rows = append(overflowTable.rows, row)
		}
	}
	// Add remaining data rows (skip headers/footers + already-rendered rows).
	// When the split landed strictly inside an oversize rowspan group, the
	// first remaining row must re-carry a continuation of the spanning
	// cell(s) (reduced rowspan) instead of the plain original row, or the
	// occupied column would be lost and buildGrid would shift the row's
	// own cell(s) left on the overflow table.
	splitInsideOversizeGroup := oversizeGroupLeader >= 0 && splitIdx > oversizeGroupLeader && splitIdx < oversizeGroupEnd
	dataRowIdx := 0
	for _, row := range t.rows {
		if row.isHeader || row.isFooter {
			continue
		}
		renderedDataRows := splitIdx - headerRowCount
		if dataRowIdx == renderedDataRows && splitInsideOversizeGroup {
			overflowTable.rows = append(overflowTable.rows, buildContinuationRow(grid, len(colWidths), splitIdx, row))
		} else if dataRowIdx >= renderedDataRows {
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

// buildOwnership returns a (row x col) matrix pointing each grid position
// at the *gridCell that occupies it, accounting for colspan/rowspan. Used
// by resolveBorders to find the true neighbor across a shared edge — a
// gridCell's array position in gr.cells is NOT its grid column (colspan
// shifts things), and its starting row is NOT necessarily its ending row
// (rowspan). Returns nil at positions no cell logically reaches (shouldn't
// happen for a well-formed grid, but guarded defensively).
func buildOwnership(grid []gridRow, nCols int) [][]*gridCell {
	owner := make([][]*gridCell, len(grid))
	for r := range owner {
		owner[r] = make([]*gridCell, nCols)
	}
	for r, gr := range grid {
		for i := range gr.cells {
			gc := &gr.cells[i]
			for dr := 0; dr < gc.rowSpanCount && r+dr < len(grid); dr++ {
				for dc := 0; dc < gc.colSpanCount && gc.col+dc < nCols; dc++ {
					owner[r+dr][gc.col+dc] = gc
				}
			}
		}
	}
	return owner
}

// buildContinuationRow reconstructs the API-level *Row at grid index
// splitIdx for an overflow table, splicing in a continuation cell (empty
// content, reduced rowspan) for every rowspan that started on an EARLIER
// row and still covers splitIdx. Without this, the occupied column(s)
// would simply be missing from the row's cell list and buildGrid would
// shift the row's own remaining cells left on the fresh overflow table
// (which has no memory of the original span).
//
// The continuation cell is a styled placeholder — it carries the
// spanning cell's borders/background/sizing but not its text/content, to
// avoid duplicating content across the page break. Only geometry
// (nothing painted past the page bottom) and span continuation (no
// column shift) are guaranteed; true content-flow continuation of the
// spanning cell's own content across the split is a follow-up.
func buildContinuationRow(grid []gridRow, nCols int, splitIdx int, orig *Row) *Row {
	owner := buildOwnership(grid, nCols)
	newRow := &Row{isHeader: orig.isHeader, isFooter: orig.isFooter}
	origIdx := 0
	for col := 0; col < nCols; {
		if gc := owner[splitIdx][col]; gc != nil && gc.row < splitIdx {
			if col == gc.col {
				remaining := gc.row + gc.rowSpanCount - splitIdx
				clone := *gc.cell
				clone.text = ""
				clone.content = nil
				clone.rowspan = remaining
				newRow.cells = append(newRow.cells, &clone)
			}
			col = gc.col + gc.colSpanCount
			continue
		}
		if origIdx < len(orig.cells) {
			c := orig.cells[origIdx]
			newRow.cells = append(newRow.cells, c)
			col += c.colspan
			origIdx++
			continue
		}
		col++
	}
	return newRow
}

// borderPriority ranks a border's style for CSS2.1 §17.6.2 conflict
// resolution once widths are tied. A zero-width border (undeclared, or
// explicit width:0/style:none) always ranks lowest regardless of its Style
// field's zero value — Go's Border{} zero value is Style==BorderSolid (0),
// which must NOT be mistaken for a real, declared solid border.
// groove/ridge/inset/outset are not distinguished from solid — they are
// already flattened to BorderSolid upstream (html/converter_block.go).
func borderPriority(b Border) int {
	if b.Width <= 0 {
		return -1
	}
	switch b.Style {
	case BorderDouble:
		return 3
	case BorderDashed:
		return 1
	case BorderDotted:
		return 0
	default: // BorderSolid (covers groove/ridge/inset/outset)
		return 2
	}
}

// resolveEdge picks the winning border for one shared edge, given the two
// cells' own-side declarations (near = the earlier cell's right/bottom,
// far = the later cell's left/top). Per CSS2.1 §17.6.2:
//  1. hidden always wins, even against a wider border, and suppresses the
//     edge entirely if either side is hidden.
//  2. Otherwise the wider border wins.
//  3. Equal width: higher style-priority wins (see borderPriority).
//  4. Full tie: the later element (far) wins — this also preserves the
//     pre-existing behavior for the common case of identical borders on
//     every cell (folio's default new-cell border).
func resolveEdge(near, far Border) Border {
	if near.Style == BorderHidden || far.Style == BorderHidden {
		return Border{}
	}
	if near.Width != far.Width {
		if near.Width > far.Width {
			return near
		}
		return far
	}
	if p1, p2 := borderPriority(near), borderPriority(far); p1 != p2 {
		if p1 > p2 {
			return near
		}
		return far
	}
	return far
}

// resolveBorders implements border-collapse: collapse per CSS2.1 §17.6.2.
// It never mutates gc.cell.borders (the cell's own declared border) —
// Table.PlanLayout resolves borders once per Layout/PlanLayout call, and a
// header row that is reused verbatim across the overflow continuation table
// (cloneForOverflow) must be resolved fresh against its new neighbor each
// time, not against a possibly-zeroed leftover from a previous resolution.
// Each call resolves fresh from the untouched declarations, writing results
// only into gc.resolved.
func resolveBorders(grid []gridRow, nCols int) {
	owner := buildOwnership(grid, nCols)

	// Pass 1: initialize whole-edge and per-segment fields from each cell's
	// own declaration. Segment slices are read from gc.cell.borders (the
	// DECLARED side) throughout pass 2 below, never from a possibly-cleared
	// gc.resolved / prior segment value — so each segment's result never
	// depends on the iteration order of other segments or cells.
	for r := range grid {
		for i := range grid[r].cells {
			gc := &grid[r].cells[i]
			gc.resolved = gc.cell.borders       // start from the cell's own declaration
			gc.cell.borderRadius = [4]float64{} // border-radius has no effect under collapse (CSS Backgrounds L3 §5.3)

			gc.segTop = make([]Border, gc.colSpanCount)
			gc.segBottom = make([]Border, gc.colSpanCount)
			for dc := range gc.colSpanCount {
				gc.segTop[dc] = gc.cell.borders.Top
				gc.segBottom[dc] = gc.cell.borders.Bottom
			}
			gc.segLeft = make([]Border, gc.rowSpanCount)
			gc.segRight = make([]Border, gc.rowSpanCount)
			for dr := range gc.rowSpanCount {
				gc.segLeft[dr] = gc.cell.borders.Left
				gc.segRight[dr] = gc.cell.borders.Right
			}
		}
	}

	// Pass 2: resolve every interior segment along each cell's right and
	// bottom edges by walking the FULL span extent, not just the starting
	// row/column. Each interior segment is visited exactly once — from the
	// "near" (above/left) cell's side — since every interior edge has
	// exactly one near/far pair for the row (or column) it sits on.
	for r := range grid {
		for i := range grid[r].cells {
			near := &grid[r].cells[i]

			// Vertical edge: near's right side vs. the cell(s) starting at
			// its right-adjacent column, for every row the span covers.
			rightCol := near.col + near.colSpanCount
			if rightCol < nCols {
				for dr := range near.rowSpanCount {
					if r+dr >= len(grid) {
						break
					}
					far := owner[r+dr][rightCol]
					if far == nil || far == near {
						continue
					}
					winner := resolveEdge(near.cell.borders.Right, far.cell.borders.Left)
					near.segRight[dr] = Border{}
					far.segLeft[r+dr-far.row] = winner
				}
			} // else: table's right perimeter — near's own Right segments stand, untouched.

			// Horizontal edge: near's bottom side vs. the cell(s) starting
			// at its row-below-adjacent row, for every column the span
			// covers.
			belowRow := r + near.rowSpanCount
			if belowRow < len(grid) {
				for dc := range near.colSpanCount {
					col := near.col + dc
					if col >= nCols {
						break
					}
					far := owner[belowRow][col]
					if far == nil || far == near {
						continue
					}
					winner := resolveEdge(near.cell.borders.Bottom, far.cell.borders.Top)
					near.segBottom[dc] = Border{}
					far.segTop[col-far.col] = winner
				}
			} // else: table's bottom perimeter — near's own Bottom segments stand, untouched.
		}
	}

	// Keep the whole-edge `resolved` fields in sync for the common
	// (unspanned) case and for any remaining perimeter-only consumers:
	// mirror segment 0's outcome onto the aggregate field.
	for r := range grid {
		for i := range grid[r].cells {
			gc := &grid[r].cells[i]
			if len(gc.segTop) > 0 {
				gc.resolved.Top = gc.segTop[0]
			}
			if len(gc.segBottom) > 0 {
				gc.resolved.Bottom = gc.segBottom[len(gc.segBottom)-1]
			}
			if len(gc.segLeft) > 0 {
				gc.resolved.Left = gc.segLeft[0]
			}
			if len(gc.segRight) > 0 {
				gc.resolved.Right = gc.segRight[len(gc.segRight)-1]
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

// drawCellBordersCollapsed draws a cell's borders under border-collapse,
// one segment per grid unit the cell spans, using the per-segment winners
// resolveBorders computed (gc.segTop/segBottom/segLeft/segRight). This is
// the collapse-mode counterpart to drawCellBorders, which draws each side
// as a single full-length line and is used for the (non-collapse) default
// and by div/flex/grid callers — its signature is intentionally left
// unchanged.
//
// Convention: an interior segment is drawn exactly once, by whichever cell
// holds the non-zero winner after resolveBorders — the loser side was
// zeroed to Border{}, and drawStyledBorder is a no-op for a zero-width
// border, so no segment is ever double-drawn.
func drawCellBordersCollapsed(stream *content.Stream, tbl *Table, grid []gridRow, colWidths []float64, gc gridCell, rowIndex int, cellX, cellBottomY, topY float64, rowLimit int) {
	// rowLimit caps the vertical (left/right) segment loop below to the
	// rows actually rendered on this page (mirrors the cellH capping in
	// drawTableRowDirect for an oversize rowspan group split mid-span). 0
	// means "no cap" — draw every spanned row's segment as usual.
	rowSpanDrawn := gc.rowSpanCount
	if rowLimit > 0 && rowLimit < rowSpanDrawn {
		rowSpanDrawn = rowLimit
	}
	// Column (horizontal) segment geometry: x-start and width of each
	// spanned column, in logical order (index 0 = gc.col). RTL lays out
	// higher logical columns further left, so the accumulation direction
	// flips; LTR accumulates left to right from cellX.
	segColX := make([]float64, gc.colSpanCount)
	segColW := make([]float64, gc.colSpanCount)
	if tbl.direction == DirectionRTL {
		curX := cellX
		for dc := gc.colSpanCount - 1; dc >= 0; dc-- {
			w := colWidths[gc.col+dc]
			segColX[dc] = curX
			segColW[dc] = w
			curX += w
		}
	} else {
		curX := cellX
		for dc := 0; dc < gc.colSpanCount; dc++ {
			w := colWidths[gc.col+dc]
			segColX[dc] = curX
			segColW[dc] = w
			curX += w
		}
	}

	// Row (vertical) segment geometry: bottom-y and height of each spanned
	// row. Rows always flow top to bottom regardless of direction.
	segRowY := make([]float64, rowSpanDrawn)
	segRowH := make([]float64, rowSpanDrawn)
	sv := tbl.effectiveSpacingV()
	curY := topY
	for dr := 0; dr < rowSpanDrawn; dr++ {
		h := grid[rowIndex+dr].height
		segRowY[dr] = curY - h
		segRowH[dr] = h
		curY -= h + sv
	}

	// Top edge always covers the cell's own row(s); bottom edge only draws
	// at the true (uncapped) bottom of the span — a capped/split cell has
	// no declared border at its artificial page-bottom split, so drawing
	// nothing there is intentional (documented limitation: no border at
	// an oversize-group's mid-span page split).
	for dc := 0; dc < gc.colSpanCount; dc++ {
		drawStyledBorder(stream, gc.segTop[dc], segColX[dc], topY, segColX[dc]+segColW[dc], topY)
		if rowSpanDrawn == gc.rowSpanCount {
			drawStyledBorder(stream, gc.segBottom[dc], segColX[dc], cellBottomY, segColX[dc]+segColW[dc], cellBottomY)
		}
	}
	// Left and right edges: one vertical line per rendered spanned row.
	rightX := cellX + gc.spanWidth
	for dr := 0; dr < rowSpanDrawn; dr++ {
		drawStyledBorder(stream, gc.segLeft[dr], cellX, segRowY[dr], cellX, segRowY[dr]+segRowH[dr])
		drawStyledBorder(stream, gc.segRight[dr], rightX, segRowY[dr], rightX, segRowY[dr]+segRowH[dr])
	}
}

// drawBackgroundRounded fills a rounded rectangle background for a cell.
// x, y is bottom-left; w, h are dimensions; r is [TL, TR, BR, BL] radii.
func drawBackgroundRounded(ctx DrawContext, bg Color, x, y, w, h float64, r [4]float64) {
	ctx.Stream.SaveState()
	setFillColor(ctx.Stream, bg)
	ctx.Stream.RoundedRectPerCorner(x, y, w, h, r[0], r[1], r[2], r[3])
	ctx.Stream.Fill()
	ctx.Stream.RestoreState()
}

// drawCellBordersRounded draws cell borders with rounded corners.
// When all four borders are identical, draws a single rounded rect stroke.
// When borders differ, each side is drawn individually with corner arcs
// at endpoints where the radius is non-zero.
func drawCellBordersRounded(stream *content.Stream, borders CellBorders, x, y, w, h float64, r [4]float64) {
	// Fast path: all borders identical → single rounded rect stroke.
	if borders.Top.Width > 0 && borders.Top == borders.Right &&
		borders.Top == borders.Bottom && borders.Top == borders.Left {
		stream.SaveState()
		setStrokeColor(stream, borders.Top.Color)
		stream.SetLineWidth(borders.Top.Width)
		stream.RoundedRectPerCorner(x, y, w, h, r[0], r[1], r[2], r[3])
		stream.Stroke()
		stream.RestoreState()
		return
	}

	// Mixed borders: draw each side with its adjacent corner arcs.
	// r = [TL, TR, BR, BL], coordinates: (x,y) = bottom-left.
	const k = 0.5522847498 // Bézier approximation for circular arcs

	maxR := min(w, h) / 2
	rTL := min(r[0], maxR)
	rTR := min(r[1], maxR)
	rBR := min(r[2], maxR)
	rBL := min(r[3], maxR)

	// Bottom border: BL corner arc → bottom line → BR corner arc
	if borders.Bottom.Width > 0 {
		stream.SaveState()
		setStrokeColor(stream, borders.Bottom.Color)
		stream.SetLineWidth(borders.Bottom.Width)
		if rBL > 0 {
			kr := rBL * k
			stream.MoveTo(x, y+rBL)
			stream.CurveTo(x, y+rBL-kr, x+rBL-kr, y, x+rBL, y)
		} else {
			stream.MoveTo(x, y)
		}
		stream.LineTo(x+w-rBR, y)
		if rBR > 0 {
			kr := rBR * k
			stream.CurveTo(x+w-rBR+kr, y, x+w, y+rBR-kr, x+w, y+rBR)
		}
		stream.Stroke()
		stream.RestoreState()
	}

	// Right border: BR corner arc → right line → TR corner arc
	if borders.Right.Width > 0 {
		stream.SaveState()
		setStrokeColor(stream, borders.Right.Color)
		stream.SetLineWidth(borders.Right.Width)
		if rBR > 0 {
			kr := rBR * k
			stream.MoveTo(x+w-rBR, y)
			stream.CurveTo(x+w-rBR+kr, y, x+w, y+rBR-kr, x+w, y+rBR)
		} else {
			stream.MoveTo(x+w, y)
		}
		stream.LineTo(x+w, y+h-rTR)
		if rTR > 0 {
			kr := rTR * k
			stream.CurveTo(x+w, y+h-rTR+kr, x+w-rTR+kr, y+h, x+w-rTR, y+h)
		}
		stream.Stroke()
		stream.RestoreState()
	}

	// Top border: TR corner arc → top line → TL corner arc
	if borders.Top.Width > 0 {
		stream.SaveState()
		setStrokeColor(stream, borders.Top.Color)
		stream.SetLineWidth(borders.Top.Width)
		if rTR > 0 {
			kr := rTR * k
			stream.MoveTo(x+w, y+h-rTR)
			stream.CurveTo(x+w, y+h-rTR+kr, x+w-rTR+kr, y+h, x+w-rTR, y+h)
		} else {
			stream.MoveTo(x+w, y+h)
		}
		stream.LineTo(x+rTL, y+h)
		if rTL > 0 {
			kr := rTL * k
			stream.CurveTo(x+rTL-kr, y+h, x, y+h-rTL+kr, x, y+h-rTL)
		}
		stream.Stroke()
		stream.RestoreState()
	}

	// Left border: TL corner arc → left line → BL corner arc
	if borders.Left.Width > 0 {
		stream.SaveState()
		setStrokeColor(stream, borders.Left.Color)
		stream.SetLineWidth(borders.Left.Width)
		if rTL > 0 {
			kr := rTL * k
			stream.MoveTo(x+rTL, y+h)
			stream.CurveTo(x+rTL-kr, y+h, x, y+h-rTL+kr, x, y+h-rTL)
		} else {
			stream.MoveTo(x, y+h)
		}
		stream.LineTo(x, y+rBL)
		if rBL > 0 {
			kr := rBL * k
			stream.CurveTo(x, y+rBL-kr, x+rBL-kr, y, x+rBL, y)
		}
		stream.Stroke()
		stream.RestoreState()
	}
}

// drawStyledBorder draws a single border line with the appropriate style.
func drawStyledBorder(stream *content.Stream, b Border, x1, y1, x2, y2 float64) {
	if b.Width <= 0 || b.Style == BorderNone || b.Style == BorderHidden {
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
	return resolveMeasurer(cell.embedded, cell.font, nil)
}

// IsTable reports whether a Line was produced by a Table element.
func (l *Line) IsTable() bool {
	return l.tableRow != nil
}

// drawTableRowDirect renders a table row directly using draw.go functions,
// without going through the old Renderer emit methods.
//
// capRows is normally 0 (draw every spanning cell at its full natural
// spanHeight). When positive, it caps any cell starting at rowIndex whose
// span extends beyond capRows rows to the height actually covered by those
// capRows rows (natural row heights plus the vertical spacing gaps between
// them) — used when an oversize rowspan group is split mid-group across a
// page break, so the spanning cell's border/background box closes at the
// page bottom on this page instead of painting the whole (mostly
// off-page) span. The continuation of the cell is drawn separately by the
// overflow table (see PlanLayout's oversize-group handling).
func drawTableRowDirect(ctx DrawContext, tbl *Table, grid []gridRow, rowIndex int, colWidths []float64, maxWidth, x, topY float64, capRows int) {
	gr := grid[rowIndex]

	sh := tbl.effectiveSpacingH()

	// Compute total table width for RTL mirroring.
	totalW := sh // left-edge spacing
	for _, w := range colWidths {
		totalW += w + sh
	}

	for _, gc := range gr.cells {
		var cellX float64
		if tbl.direction == DirectionRTL {
			// RTL: column 0 at the right edge, last column at the left.
			// Start from the right and work leftward past each column.
			cellX = x + totalW - gc.spanWidth - sh
			for c := range gc.col {
				cellX -= colWidths[len(colWidths)-1-c] + sh
			}
		} else {
			// LTR: column 0 at the left edge (default).
			cellX = x + sh
			for c := range gc.col {
				cellX += colWidths[c] + sh
			}
		}
		// Rowspanning cells extend below their starting row; spanHeight
		// covers the full span (0 for single-row cells).
		cellH := gr.height
		if gc.spanHeight > 0 {
			cellH = gc.spanHeight
		}
		if capRows > 0 && gc.rowSpanCount > capRows {
			// This page only shows capRows rows of the span (the rest
			// splits to the overflow table) — close the cell's box at
			// what's actually rendered here instead of the full span.
			renderedH := 0.0
			for k := range capRows {
				if rowIndex+k >= len(grid) {
					break
				}
				if k > 0 {
					renderedH += tbl.effectiveSpacingV()
				}
				renderedH += grid[rowIndex+k].height
			}
			cellH = renderedH
		}
		cellBottomY := topY - cellH

		// Background fill (with optional rounded corners).
		r := gc.cell.borderRadius
		hasRadius := r[0] > 0 || r[1] > 0 || r[2] > 0 || r[3] > 0
		if gc.cell.bgColor != nil {
			if hasRadius {
				drawBackgroundRounded(ctx, *gc.cell.bgColor, cellX, cellBottomY, gc.spanWidth, cellH, r)
			} else {
				drawBackground(ctx, *gc.cell.bgColor, cellX, topY, gc.spanWidth, cellH)
			}
		}

		// Borders (with optional rounded corners). Under border-collapse,
		// gc.cell.borders holds each cell's own untouched declaration —
		// the resolved, deduplicated borders are drawn per grid segment
		// from gc.segTop/segBottom/segLeft/segRight instead (see
		// resolveBorders). borderRadius is zeroed under collapse
		// (resolveBorders), so hasRadius can never be true here.
		if tbl.borderCollapse {
			drawCellBordersCollapsed(ctx.Stream, tbl, grid, colWidths, gc, rowIndex, cellX, cellBottomY, topY, capRows)
		} else if hasRadius {
			drawCellBordersRounded(ctx.Stream, gc.cell.borders, cellX, cellBottomY, gc.spanWidth, cellH, r)
		} else {
			drawCellBorders(ctx.Stream, gc.cell.borders, cellX, cellBottomY, gc.spanWidth, cellH)
		}

		// Cell content.
		if gc.cell.content != nil {
			drawCellElementDirect(ctx, gc, cellX, topY, cellH)
		} else {
			drawCellTextDirect(ctx, gc.cell, cellX, topY, gc.spanWidth, cellH)
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
