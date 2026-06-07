// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"context"
	"fmt"
	"html/template"

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
// opts may be nil for default settings. It is equivalent to
// AddHTMLWithContext with a background context.
func (d *Document) AddHTML(htmlStr string, opts *foliohtml.Options) error {
	return d.AddHTMLWithContext(context.Background(), htmlStr, opts)
}

// AddHTMLWithContext is the context-aware variant of AddHTML. It threads ctx
// through the HTML conversion, checking it at element boundaries during the
// tree walk and returning ctx.Err() if the context is done, so a runaway
// conversion can be bounded by a deadline.
func (d *Document) AddHTMLWithContext(ctx context.Context, htmlStr string, opts *foliohtml.Options) error {
	result, err := foliohtml.ConvertFullWithContext(ctx, htmlStr, opts)
	if err != nil {
		return err
	}
	return d.AddConvertResult(result)
}

// AddConvertResult feeds a raw [html.ConvertResult] into the document:
// elements, absolutes, @page geometry/margins, margin boxes, and metadata
// (existing non-empty Info fields are kept). AddHTML is exactly ConvertFull
// followed by AddConvertResult; call this directly only to inspect or modify
// the result first. Adding result.Elements by hand instead drops
// result.Absolutes and the @page configuration.
func (d *Document) AddConvertResult(result *foliohtml.ConvertResult) error {
	if result == nil {
		return fmt.Errorf("document: AddConvertResult: nil result")
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
		// Resolve page geometry + the orientation-only swap + all margin
		// percentages / calc through the single shared helper so every
		// Document-building consumer behaves identically (B-1). The default
		// page box is the document default; the helper applies any explicit
		// @page size or orientation-only keyword and resolves margins
		// against the final dimensions (left/right vs width, top/bottom vs
		// height, per CSS Paged Media).
		w, h, _ := pc.Resolve(d.pageSize.Width, d.pageSize.Height)
		d.pageSize = PageSize{Width: w, Height: h}

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
				boxes[name] = layout.MarginBox{Content: mbc.Content, FontSize: mbc.FontSize, Color: mbc.Color, HasColor: mbc.HasColor, Embedded: mbc.Embedded}
			}
			d.SetMarginBoxes(boxes)
		}
		if pc.First != nil && len(pc.First.MarginBoxes) > 0 {
			boxes := make(map[string]layout.MarginBox)
			for name, mbc := range pc.First.MarginBoxes {
				boxes[name] = layout.MarginBox{Content: mbc.Content, FontSize: mbc.FontSize, Color: mbc.Color, HasColor: mbc.HasColor, Embedded: mbc.Embedded}
			}
			d.SetFirstMarginBoxes(boxes)
		}
		if pc.Left != nil && len(pc.Left.MarginBoxes) > 0 {
			boxes := make(map[string]layout.MarginBox)
			for name, mbc := range pc.Left.MarginBoxes {
				boxes[name] = layout.MarginBox{Content: mbc.Content, FontSize: mbc.FontSize, Color: mbc.Color, HasColor: mbc.HasColor, Embedded: mbc.Embedded}
			}
			d.SetLeftMarginBoxes(boxes)
		}
		if pc.Right != nil && len(pc.Right.MarginBoxes) > 0 {
			boxes := make(map[string]layout.MarginBox)
			for name, mbc := range pc.Right.MarginBoxes {
				boxes[name] = layout.MarginBox{Content: mbc.Content, FontSize: mbc.FontSize, Color: mbc.Color, HasColor: mbc.HasColor, Embedded: mbc.Embedded}
			}
			d.SetRightMarginBoxes(boxes)
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
			fixed:        abs.Fixed,
		})
	}

	return nil
}

// AddHTMLTemplate executes a Go [html/template] with the given data and
// adds the resulting HTML to the document. This is a convenience for the
// common workflow of rendering a template with dynamic data to produce a
// PDF — e.g., invoices, reports, or letters.
//
// The template string is parsed with [html/template.New] and executed
// with data. The result is passed to [Document.AddHTML]. Template
// functions, CSS <style> blocks, and all HTML features supported by
// [AddHTML] work as expected.
//
//	doc.AddHTMLTemplate(`
//	  <h1>Invoice #{{.Number}}</h1>
//	  <table>
//	    {{range .Items}}
//	    <tr><td>{{.Name}}</td><td>{{.Price}}</td></tr>
//	    {{end}}
//	  </table>
//	`, invoiceData, nil)
func (d *Document) AddHTMLTemplate(tmplStr string, data any, opts *foliohtml.Options) error {
	t, err := template.New("folio").Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("document: parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("document: execute template: %w", err)
	}
	return d.AddHTML(buf.String(), opts)
}

// AddHTMLTemplateFuncs is like [AddHTMLTemplate] but accepts custom
// template functions via a [template.FuncMap].
func (d *Document) AddHTMLTemplateFuncs(tmplStr string, funcs template.FuncMap, data any, opts *foliohtml.Options) error {
	t, err := template.New("folio").Funcs(funcs).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("document: parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return fmt.Errorf("document: execute template: %w", err)
	}
	return d.AddHTML(buf.String(), opts)
}
