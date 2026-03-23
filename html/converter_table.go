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
	hasTableMargin := style.MarginTop > 0 || style.MarginBottom > 0
	hasTableWidth := style.MaxWidth != nil
	if hasTableMargin || style.BackgroundColor != nil || hasTableWidth {
		div := layout.NewDiv()
		div.Add(tbl)
		if style.MarginTop > 0 {
			div.SetSpaceBefore(style.MarginTop)
		}
		if style.MarginBottom > 0 {
			div.SetSpaceAfter(style.MarginBottom)
		}
		if style.BackgroundColor != nil {
			div.SetBackground(*style.BackgroundColor)
		}
		if style.MaxWidth != nil {
			div.SetMaxWidth(style.MaxWidth.toPoints(parentContainerWidth, style.FontSize))
		}
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

	// Apply CSS width as table minimum width.
	if style.Width != nil {
		tbl.SetMinWidthUnit(cssLengthToUnitValue(style.Width, c.containerWidth, style.FontSize))
	}

	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			continue
		}
		childStyle := c.computeElementStyle(child, style)

		if childStyle.Display == "table-row" {
			row := tbl.AddRow()
			for cell := child.FirstChild; cell != nil; cell = cell.NextSibling {
				if cell.Type != html.ElementNode {
					continue
				}
				cellStyle := c.computeElementStyle(cell, childStyle)
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
				layoutCell.SetAlign(cellStyle.TextAlign)
				if cellStyle.hasPadding() {
					layoutCell.SetPaddingSides(layout.Padding{
						Top:    cellStyle.PaddingTop,
						Right:  cellStyle.PaddingRight,
						Bottom: cellStyle.PaddingBottom,
						Left:   cellStyle.PaddingLeft,
					})
				}
				if cellStyle.BackgroundColor != nil {
					layoutCell.SetBackground(*cellStyle.BackgroundColor)
				}
				if cellStyle.hasBorder() {
					layoutCell.SetBorders(buildCellBorders(cellStyle))
				}
			}
		} else {
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
	if style.MarginTop > 0 || style.MarginBottom > 0 {
		div := layout.NewDiv()
		div.Add(tbl)
		if style.MarginTop > 0 {
			div.SetSpaceBefore(style.MarginTop)
		}
		if style.MarginBottom > 0 {
			div.SetSpaceAfter(style.MarginBottom)
		}
		return []layout.Element{div}
	}

	return []layout.Element{tbl}
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

		// For <th>, default to bold.
		if child.DataAtom == atom.Th {
			if cellStyle.FontWeight == "normal" {
				cellStyle.FontWeight = "bold"
			}
			if cellStyle.TextAlign == layout.AlignLeft {
				cellStyle.TextAlign = layout.AlignCenter
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

		cell.SetAlign(cellStyle.TextAlign)

		// Per-side cell padding (default 4pt uniform).
		if cellStyle.hasPadding() {
			cell.SetPaddingSides(layout.Padding{
				Top:    cellStyle.PaddingTop,
				Right:  cellStyle.PaddingRight,
				Bottom: cellStyle.PaddingBottom,
				Left:   cellStyle.PaddingLeft,
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
		if cellStyle.BackgroundColor != nil {
			cell.SetBackground(*cellStyle.BackgroundColor)
		} else if rowStyle.BackgroundColor != nil {
			cell.SetBackground(*rowStyle.BackgroundColor)
		}

		// Cell borders: prefer per-cell CSS borders, fall back to table border,
		// or remove default borders if table has no border.
		if cellStyle.hasBorder() {
			cell.SetBorders(buildCellBorders(cellStyle))
		} else if borderWidth > 0 {
			cell.SetBorders(layout.AllBorders(layout.SolidBorder(borderWidth, layout.ColorBlack)))
		} else {
			// No cell border and no table border — clear the default borders.
			cell.SetBorders(layout.CellBorders{})
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
		if cellStyle.Width != nil {
			w := cellStyle.Width.toPoints(c.containerWidth, cellStyle.FontSize)
			if w > 0 {
				cell.SetWidthHint(w)
			}
		}
	}
}
