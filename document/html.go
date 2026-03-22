// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	foliohtml "github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

// AddHTML parses an HTML string and adds the resulting layout elements to the
// document. It also extracts metadata from <title> and <meta> tags and applies
// them to the document's Info fields (Title, Author, Subject, Keywords,
// Creator). Existing non-empty Info fields are not overwritten.
//
// If the HTML contains @page CSS rules, the page size and margins are applied
// to the document.
//
// opts may be nil for default settings.
func (d *Document) AddHTML(htmlStr string, opts *foliohtml.Options) error {
	result, err := foliohtml.ConvertFull(htmlStr, opts)
	if err != nil {
		return err
	}

	// Apply metadata from <title> and <meta> tags.
	m := result.Metadata
	if d.Info.Title == "" && m.Title != "" {
		d.Info.Title = m.Title
	}
	if d.Info.Author == "" && m.Author != "" {
		d.Info.Author = m.Author
	}
	if d.Info.Subject == "" && m.Subject != "" {
		d.Info.Subject = m.Subject
	}
	if d.Info.Keywords == "" && m.Keywords != "" {
		d.Info.Keywords = m.Keywords
	}
	if d.Info.Creator == "" && m.Creator != "" {
		d.Info.Creator = m.Creator
	}

	// Apply @page configuration if present.
	if pc := result.PageConfig; pc != nil {
		if pc.Width > 0 && (pc.Height > 0 || pc.AutoHeight) {
			// parsePageSize already swaps width/height for landscape,
			// so we use the dimensions as-is. AutoHeight passes Height=0
			// to trigger content-sized pages.
			d.pageSize = PageSize{Width: pc.Width, Height: pc.Height}
		}
		if pc.HasMargins {
			d.margins.Top = pc.MarginTop
			d.margins.Right = pc.MarginRight
			d.margins.Bottom = pc.MarginBottom
			d.margins.Left = pc.MarginLeft
		}
		if pc.First != nil && pc.First.HasMargins {
			d.SetFirstMargins(layout.Margins{
				Top: pc.First.Top, Right: pc.First.Right,
				Bottom: pc.First.Bottom, Left: pc.First.Left,
			})
		}
		if pc.Left != nil && pc.Left.HasMargins {
			d.SetLeftMargins(layout.Margins{
				Top: pc.Left.Top, Right: pc.Left.Right,
				Bottom: pc.Left.Bottom, Left: pc.Left.Left,
			})
		}
		if pc.Right != nil && pc.Right.HasMargins {
			d.SetRightMargins(layout.Margins{
				Top: pc.Right.Top, Right: pc.Right.Right,
				Bottom: pc.Right.Bottom, Left: pc.Right.Left,
			})
		}
	}

	// Apply margin boxes from @page rules.
	if pc := result.PageConfig; pc != nil {
		if len(pc.MarginBoxes) > 0 {
			boxes := make(map[string]layout.MarginBox)
			for name, mbc := range pc.MarginBoxes {
				boxes[name] = layout.MarginBox{Content: mbc.Content, FontSize: mbc.FontSize, Color: mbc.Color}
			}
			d.SetMarginBoxes(boxes)
		}
		if pc.First != nil && len(pc.First.MarginBoxes) > 0 {
			boxes := make(map[string]layout.MarginBox)
			for name, mbc := range pc.First.MarginBoxes {
				boxes[name] = layout.MarginBox{Content: mbc.Content, FontSize: mbc.FontSize, Color: mbc.Color}
			}
			d.SetFirstMarginBoxes(boxes)
		}
	}

	// Add all normal-flow elements.
	d.elements = append(d.elements, result.Elements...)

	// Add absolutely positioned elements.
	for _, abs := range result.Absolutes {
		d.absolutes = append(d.absolutes, absoluteElement{
			elem:         abs.Element,
			x:            abs.X,
			y:            abs.Y,
			width:        abs.Width,
			pageIndex:    -1,
			rightAligned: abs.RightAligned,
			zIndex:       abs.ZIndex,
		})
	}

	return nil
}
