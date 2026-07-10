// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/layout"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// convertTable converts a <table> element into a layout.Table.
func (c *converter) convertTable(n *html.Node, style computedStyle) []layout.Element {
	// Save parent containerWidth for resolving the table's own width properties.
	parentContainerWidth := c.containerWidth
	restore := c.narrowContainerWidth(style)
	defer restore()

	var elems []layout.Element
	tbl := layout.NewTable()

	// Parse border attribute (HTML4 style).
	borderWidth := 0.0
	if attr := getAttr(n, "border"); attr != "" && attr != "0" {
		borderWidth = 0.5
	}

	// Check for CSS border on the table style.
	if style.hasBorder() {
		borderWidth = style.BorderTopWidth
		if borderWidth == 0 {
			borderWidth = 0.5
		}
	}

	// border-collapse: collapse removes duplicate borders between cells.
	collapse := style.BorderCollapse == "collapse"
	if collapse {
		tbl.SetBorderCollapse(true)
	}
	if style.BorderSpacingH > 0 || style.BorderSpacingV > 0 {
		tbl.SetCellSpacing(style.BorderSpacingH, style.BorderSpacingV)
	}
	if style.Direction == layout.DirectionRTL {
		tbl.SetDirection(layout.DirectionRTL)
	}

	// Collect <col> widths from <colgroup>/<col> elements.
	var colWidths []layout.UnitValue

	// Walk children: <caption>, <colgroup>, <col>, <thead>, <tbody>, <tfoot>, or direct <tr>.
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		switch child.DataAtom {
		case atom.Caption:
			// Render caption as a centered paragraph before the table.
			text := collectText(child)
			if text != "" {
				f := resolveFont(style)
				p := layout.NewParagraph(text, f, style.FontSize)
				p.SetAlign(layout.AlignCenter)
				p.SetSpaceAfter(4)
				elems = append(elems, p)
			}
		case atom.Colgroup:
			for col := child.FirstChild; col != nil; col = col.NextSibling {
				if col.Type == html.ElementNode && col.DataAtom == atom.Col {
					colWidths = append(colWidths, c.parseColWidth(col, style)...)
				}
			}
		case atom.Col:
			colWidths = append(colWidths, c.parseColWidth(child, style)...)
		case atom.Thead:
			c.convertTableRows(child, tbl, style, borderWidth, true)
		case atom.Tbody:
			c.convertTableRows(child, tbl, style, borderWidth, false)
		case atom.Tfoot:
			c.convertTableFooterRows(child, tbl, style, borderWidth)
		case atom.Tr:
			c.convertTableRow(child, tbl, style, borderWidth, false)
		}
	}

	if len(colWidths) > 0 {
		tbl.SetColumnUnitWidths(colWidths)
	} else {
		tbl.SetAutoColumnWidths()
	}
	// Apply CSS width as table minimum width so auto-sizing expands to fill.
	// Use lazy UnitValue so percentages resolve at layout time against area.Width.
	if style.Width != nil {
		tbl.SetMinWidthUnit(cssLengthToUnitValue(style.Width, parentContainerWidth, style.FontSize))
	}

	// Apply table-level margin/background/width via Div wrapper.
	mt := style.MarginTopAt(parentContainerWidth)
	mb := style.MarginBottomAt(parentContainerWidth)
	hasTableMargin := mt > 0 || mb > 0
	hasTableWidth := style.MaxWidth != nil
	if hasTableMargin || style.BackgroundColor != nil || hasTableWidth || style.hasBorderRadius() {
		div := layout.NewDiv()
		div.Add(tbl)
		if mt > 0 {
			div.SetSpaceBefore(mt)
		}
		if mb > 0 {
			div.SetSpaceAfter(mb)
		}
		if style.BackgroundColor != nil {
			div.SetBackground(*style.BackgroundColor)
		}
		if style.MaxWidth != nil {
			div.SetMaxWidth(style.MaxWidth.toPoints(parentContainerWidth, style.FontSize))
		}
		// Apply the table's CSS border-radius to the wrapper Div so a rounded
		// table background is honored (issue #329). The child is a Table, not a
		// matching-bg Paragraph, so no overpaint clearing is needed.
		applyBorderRadiusToDiv(div, style)
		// Caption elements come before the table wrapper.
		elems = append(elems, div)
		return elems
	}

	elems = append(elems, tbl)
	return elems
}

// convertCSSTable handles elements with display:table — builds a layout.Table
// from children with display:table-row and display:table-cell.
func (c *converter) convertCSSTable(n *html.Node, style computedStyle) []layout.Element {
	tbl := layout.NewTable()
	tbl.SetAutoColumnWidths()

	if style.BorderCollapse == "collapse" {
		tbl.SetBorderCollapse(true)
	}
	if style.BorderSpacingH > 0 || style.BorderSpacingV > 0 {
		tbl.SetCellSpacing(style.BorderSpacingH, style.BorderSpacingV)
	}
	if style.Direction == layout.DirectionRTL {
		tbl.SetDirection(layout.DirectionRTL)
	}

	// Apply CSS width as table minimum width.
	if style.Width != nil {
		tbl.SetMinWidthUnit(cssLengthToUnitValue(style.Width, c.containerWidth, style.FontSize))
	}

	// anonRow accumulates consecutive bare table-cell children into a
	// single CSS anonymous table-row box. Reset to nil on any non-cell
	// child so only consecutive cells share a row (per CSS table fixup).
	var anonRow *layout.Row
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		childStyle := c.computeElementStyle(child, style)

		switch childStyle.Display {
		case "table-row":
			anonRow = nil
			row := tbl.AddRow()
			for cell := child.FirstChild; cell != nil; cell = cell.NextSibling {
				if cell.Type != html.ElementNode {
					continue
				}
				cellStyle := c.computeElementStyle(cell, childStyle)
				c.addCSSTableCell(tbl, row, cell, cellStyle)
			}
		case "table-cell":
			// Bare table-cell with no table-row parent: wrap consecutive
			// cells in an anonymous table-row box (CSS table fixup).
			if anonRow == nil {
				anonRow = tbl.AddRow()
			}
			c.addCSSTableCell(tbl, anonRow, child, childStyle)
		default:
			anonRow = nil
			// Non-row children — treat as a single-cell row.
			childElems := c.convertNode(child, style)
			if len(childElems) > 0 {
				row := tbl.AddRow()
				div := layout.NewDiv()
				for _, e := range childElems {
					div.Add(e)
				}
				row.AddCellElement(div)
			}
		}
	}

	// Wrap in Div for margin.
	mt := style.MarginTopAt(c.containerWidth)
	mb := style.MarginBottomAt(c.containerWidth)
	if mt > 0 || mb > 0 {
		div := layout.NewDiv()
		div.Add(tbl)
		if mt > 0 {
			div.SetSpaceBefore(mt)
		}
		if mb > 0 {
			div.SetSpaceAfter(mb)
		}
		return []layout.Element{div}
	}

	return []layout.Element{tbl}
}

// addCSSTableCell builds a single display:table-cell into row, applying the
// cell's content, alignment, padding, background, borders, and border-radius.
func (c *converter) addCSSTableCell(tbl *layout.Table, row *layout.Row, cell *html.Node, cellStyle computedStyle) {
	cellElems := c.walkChildren(cell, cellStyle)

	var layoutCell *layout.Cell
	if len(cellElems) == 0 {
		f := resolveFont(cellStyle)
		layoutCell = row.AddCell(" ", f, cellStyle.FontSize)
	} else if len(cellElems) == 1 {
		layoutCell = row.AddCellElement(cellElems[0])
	} else {
		div := layout.NewDiv()
		for _, e := range cellElems {
			div.Add(e)
		}
		layoutCell = row.AddCellElement(div)
	}
	layoutCell.SetAlign(resolveTextAlign(cellStyle))
	if cellStyle.hasPadding() {
		layoutCell.SetPaddingSides(layout.Padding{
			Top:    cellStyle.PaddingTopAt(c.containerWidth),
			Right:  cellStyle.PaddingRightAt(c.containerWidth),
			Bottom: cellStyle.PaddingBottomAt(c.containerWidth),
			Left:   cellStyle.PaddingLeftAt(c.containerWidth),
		})
	}
	if cellStyle.BackgroundColor != nil {
		layoutCell.SetBackground(*cellStyle.BackgroundColor)
		// The cell owns and draws the rounded box fill; clear the
		// same block-level background on its content Paragraph(s)
		// to avoid a square-cornered overdraw (issue #329).
		clearCellParagraphBackground(layoutCell, *cellStyle.BackgroundColor)
	}
	if cellHasBorderDeclaration(cellStyle) {
		layoutCell.SetBorders(buildCellBorders(cellStyle))
	}
	if !tbl.BorderCollapse() {
		if cellStyle.BorderRadiusTL > 0 || cellStyle.BorderRadiusTR > 0 ||
			cellStyle.BorderRadiusBR > 0 || cellStyle.BorderRadiusBL > 0 {
			layoutCell.SetBorderRadiusPerCorner(
				cellStyle.BorderRadiusTL, cellStyle.BorderRadiusTR,
				cellStyle.BorderRadiusBR, cellStyle.BorderRadiusBL)
		} else if cellStyle.BorderRadius > 0 {
			layoutCell.SetBorderRadius(cellStyle.BorderRadius)
		}
	}
}

// cellHasBorderDeclaration reports whether style declares any border a
// table cell needs to pass to buildCellBorders — hasBorder()'s width>0
// check plus border-style: hidden, which folio always zeros the computed
// width for (see parseBorderFull) but which must still reach the
// border-collapse conflict resolver as a distinct, always-winning value.
func cellHasBorderDeclaration(style computedStyle) bool {
	return style.hasBorder() ||
		style.BorderTopStyle == "hidden" || style.BorderRightStyle == "hidden" ||
		style.BorderBottomStyle == "hidden" || style.BorderLeftStyle == "hidden"
}

// parseColWidth extracts the width from a <col> element, respecting the span attribute.
func (c *converter) parseColWidth(col *html.Node, style computedStyle) []layout.UnitValue {
	span := 1
	if s := getAttr(col, "span"); s != "" {
		if v := parseInt(s); v > 1 {
			span = v
		}
	}

	colStyle := c.computeElementStyle(col, style)
	var uv layout.UnitValue
	if colStyle.Width != nil {
		if colStyle.Width.Unit == "%" {
			uv = layout.Pct(colStyle.Width.Value)
		} else {
			uv = layout.Pt(colStyle.Width.toPoints(0, style.FontSize))
		}
	} else if w := getAttr(col, "width"); w != "" {
		if strings.HasSuffix(w, "%") {
			if num, err := strconv.ParseFloat(strings.TrimSuffix(w, "%"), 64); err == nil {
				uv = layout.Pct(num)
			}
		} else {
			if num := parseAttrFloat(w); num > 0 {
				uv = layout.Pt(num * 0.75) // px to pt
			}
		}
	}

	var result []layout.UnitValue
	for i := 0; i < span; i++ {
		result = append(result, uv)
	}
	return result
}

// convertTableRows processes <tr> children within a <thead>/<tbody>/<tfoot>.
func (c *converter) convertTableRows(n *html.Node, tbl *layout.Table, style computedStyle, borderWidth float64, isHeader bool) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.DataAtom == atom.Tr {
			c.convertTableRow(child, tbl, style, borderWidth, isHeader)
		}
	}
}

// convertTableFooterRows processes <tr> children within a <tfoot>.
func (c *converter) convertTableFooterRows(n *html.Node, tbl *layout.Table, style computedStyle, borderWidth float64) {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.DataAtom == atom.Tr {
			c.convertTableRowKind(child, tbl, style, borderWidth, "footer")
		}
	}
}

// convertTableRow processes a single <tr> and its <td>/<th> cells.
func (c *converter) convertTableRow(n *html.Node, tbl *layout.Table, parentStyle computedStyle, borderWidth float64, isHeader bool) {
	kind := "body"
	if isHeader {
		kind = "header"
	}
	c.convertTableRowKind(n, tbl, parentStyle, borderWidth, kind)
}

// convertTableRowKind processes a single <tr>. kind is "header", "footer", or "body".
func (c *converter) convertTableRowKind(n *html.Node, tbl *layout.Table, parentStyle computedStyle, borderWidth float64, kind string) {
	var row *layout.Row
	switch kind {
	case "header":
		row = tbl.AddHeaderRow()
	case "footer":
		row = tbl.AddFooterRow()
	default:
		row = tbl.AddRow()
	}

	rowStyle := c.computeElementStyle(n, parentStyle)

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		if child.DataAtom != atom.Td && child.DataAtom != atom.Th {
			continue
		}

		cellStyle := c.computeElementStyle(child, rowStyle)

		// For <th>, default to bold when the cascaded weight is at
		// or below normal (400). This matches the historical browser
		// rendering of <th> (User-Agent stylesheet sets font-weight:
		// bold) and the pre-fix Folio behaviour. Limitation: a weight
		// of 0 (unset) and an explicit `font-weight: 100` are both
		// treated as "needs bolding"; distinguishing "author chose
		// light" from "default normal" would require a FontWeightSet
		// sentinel on computedStyle, tracked separately.
		if child.DataAtom == atom.Th {
			if cellStyle.FontWeight <= 400 {
				cellStyle.FontWeight = 700
			}
			// Default <th> alignment is center when the author hasn't
			// explicitly chosen left. Use resolveTextAlign so the
			// direction-relative `start`/`end` keywords are honoured
			// before the equality check (otherwise an RTL document
			// with `text-align: start` would resolve to AlignLeft and
			// then get unexpectedly re-aligned to center).
			if !cellStyle.TextAlignSet && resolveTextAlign(cellStyle) == layout.AlignLeft {
				cellStyle.TextAlign = layout.AlignCenter
				cellStyle.TextAlignKeyword = ""
			}
		}

		cellElems := c.walkChildren(child, cellStyle)

		var cell *layout.Cell
		switch len(cellElems) {
		case 0:
			f := resolveFont(cellStyle)
			cell = row.AddCell(" ", f, cellStyle.FontSize)
		case 1:
			cell = row.AddCellElement(cellElems[0])
		default:
			div := layout.NewDiv()
			for _, e := range cellElems {
				div.Add(e)
			}
			cell = row.AddCellElement(div)
		}

		cell.SetAlign(resolveTextAlign(cellStyle))

		// Per-side cell padding (default 4pt uniform).
		if cellStyle.hasPadding() {
			cell.SetPaddingSides(layout.Padding{
				Top:    cellStyle.PaddingTopAt(c.containerWidth),
				Right:  cellStyle.PaddingRightAt(c.containerWidth),
				Bottom: cellStyle.PaddingBottomAt(c.containerWidth),
				Left:   cellStyle.PaddingLeftAt(c.containerWidth),
			})
		} else {
			cell.SetPadding(4)
		}

		// Vertical alignment.
		switch cellStyle.VerticalAlign {
		case "middle":
			cell.SetVAlign(layout.VAlignMiddle)
		case "bottom":
			cell.SetVAlign(layout.VAlignBottom)
		}

		// Background color: cell CSS > row CSS.
		// The cell now owns and draws the box fill (honoring border-radius);
		// clear the same block-level background on the cell's content
		// Paragraph(s) to avoid a square-cornered overdraw (issue #329).
		if cellStyle.BackgroundColor != nil {
			cell.SetBackground(*cellStyle.BackgroundColor)
			clearCellParagraphBackground(cell, *cellStyle.BackgroundColor)
		} else if rowStyle.BackgroundColor != nil {
			cell.SetBackground(*rowStyle.BackgroundColor)
			clearCellParagraphBackground(cell, *rowStyle.BackgroundColor)
		}

		// Cell borders: prefer per-cell CSS borders, fall back to table border,
		// or remove default borders if table has no border.
		if cellHasBorderDeclaration(cellStyle) {
			cell.SetBorders(buildCellBorders(cellStyle))
		} else if borderWidth > 0 {
			cell.SetBorders(layout.AllBorders(layout.SolidBorder(borderWidth, layout.ColorBlack)))
		} else {
			// No cell border and no table border — clear the default borders.
			cell.SetBorders(layout.CellBorders{})
		}

		// Cell border-radius (CSS Backgrounds Level 3 §5.3).
		// Per spec, border-radius has no effect in border-collapse: collapse mode.
		if !tbl.BorderCollapse() {
			if cellStyle.BorderRadiusTL > 0 || cellStyle.BorderRadiusTR > 0 ||
				cellStyle.BorderRadiusBR > 0 || cellStyle.BorderRadiusBL > 0 {
				cell.SetBorderRadiusPerCorner(
					cellStyle.BorderRadiusTL, cellStyle.BorderRadiusTR,
					cellStyle.BorderRadiusBR, cellStyle.BorderRadiusBL)
			} else if cellStyle.BorderRadius > 0 {
				cell.SetBorderRadius(cellStyle.BorderRadius)
			}
		}

		if cs := getAttr(child, "colspan"); cs != "" {
			if v := parseInt(cs); v > 1 {
				cell.SetColspan(v)
			}
		}
		if rs := getAttr(child, "rowspan"); rs != "" {
			if v := parseInt(rs); v > 1 {
				cell.SetRowspan(v)
			}
		}

		// CSS width on the cell → column width hint for auto-sizing.
		// Percentage widths are stored as lazy UnitValues so they
		// resolve against the table's actual maxWidth at layout time
		// (see Cell.SetWidthHintUnit). Without lazy resolution, a
		// cell inside a narrow flex column would resolve its 50%
		// against c.containerWidth (the outer page width), producing
		// an absurdly large hint that overflows the column on render.
		if cellStyle.Width != nil {
			if cellStyle.Width.Unit == "%" {
				cell.SetWidthHintUnit(layout.Pct(cellStyle.Width.Value))
			} else {
				w := cellStyle.Width.toPoints(c.containerWidth, cellStyle.FontSize)
				if w > 0 {
					cell.SetWidthHint(w)
				}
			}
		}
	}
}
