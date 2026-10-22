//go:build js && wasm

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"syscall/js"

	"github.com/carlos7ags/folio/cmd/wasm/settings"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

func renderHTML(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return map[string]any{"error": "expected 2 arguments: html, settingsJSON"}
	}

	htmlStr := args[0].String()
	settingsJSON := args[1].String()

	set, err := settings.Parse(settingsJSON)
	if err != nil {
		return map[string]any{"error": "invalid settings JSON: " + err.Error()}
	}

	// Named page size plus orientation (landscape swaps the dimensions).
	pageSize := set.ResolvePageSize()

	opts := &html.Options{
		PageWidth:  pageSize.Width,
		PageHeight: pageSize.Height,
	}
	// Asset resolution: baseDir → BaseFS, allowAbsolutePaths, fallback font
	// from path or inline base64 data.
	if err := set.ApplyOptions(opts); err != nil {
		return map[string]any{"error": err.Error()}
	}

	result, err := html.ConvertFull(htmlStr, opts)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	// Apply @page CSS rules to page size via the shared helper so this path
	// honors orientation-only keywords and resolves margin percentages /
	// calc identically to AddHTML (B-1). AutoHeight (size: <w> 0) yields a
	// content-sized page (height 0).
	if pc := result.PageConfig; pc != nil {
		w, h, _ := pc.Resolve(pageSize.Width, pageSize.Height)
		pageSize = document.PageSize{Width: w, Height: h}
	}

	doc := document.NewDocument(pageSize)

	// Apply @page margins (must be after NewDocument to override defaults).
	if pc := result.PageConfig; pc != nil {
		if pc.HasMargins {
			doc.SetMargins(layout.Margins{
				Top:    pc.MarginTop,
				Right:  pc.MarginRight,
				Bottom: pc.MarginBottom,
				Left:   pc.MarginLeft,
			})
		}
		if pc.First != nil && pc.First.HasMargins {
			doc.SetFirstMargins(layout.Margins{
				Top: pc.First.Top, Right: pc.First.Right,
				Bottom: pc.First.Bottom, Left: pc.First.Left,
			})
		}
		if pc.Left != nil && pc.Left.HasMargins {
			doc.SetLeftMargins(layout.Margins{
				Top: pc.Left.Top, Right: pc.Left.Right,
				Bottom: pc.Left.Bottom, Left: pc.Left.Left,
			})
		}
		if pc.Right != nil && pc.Right.HasMargins {
			doc.SetRightMargins(layout.Margins{
				Top: pc.Right.Top, Right: pc.Right.Right,
				Bottom: pc.Right.Bottom, Left: pc.Right.Left,
			})
		}
	}

	// Explicit margins from renderSettings (points) override @page margins.
	if set.Margins != nil {
		doc.SetMargins(layout.Margins{
			Top:    set.Margins.Top,
			Right:  set.Margins.Right,
			Bottom: set.Margins.Bottom,
			Left:   set.Margins.Left,
		})
	}

	// Apply margin boxes from @page rules (e.g. page numbers).
	if result.MarginBoxes != nil {
		doc.SetMarginBoxes(result.MarginBoxes)
	}
	if result.FirstMarginBoxes != nil {
		doc.SetFirstMarginBoxes(result.FirstMarginBoxes)
	}
	if result.LeftMarginBoxes != nil {
		doc.SetLeftMarginBoxes(result.LeftMarginBoxes)
	}
	if result.RightMarginBoxes != nil {
		doc.SetRightMarginBoxes(result.RightMarginBoxes)
	}

	// Apply metadata from HTML <title>/<meta> tags
	if result.Metadata.Title != "" {
		doc.Info.Title = result.Metadata.Title
	}
	if result.Metadata.Author != "" {
		doc.Info.Author = result.Metadata.Author
	}

	// Overrides / additions from renderSettings.
	if set.PdfTitle != "" {
		doc.Info.Title = set.PdfTitle
	}
	if set.PdfAuthor != "" {
		doc.Info.Author = set.PdfAuthor
	}
	if set.PdfSubject != "" {
		doc.Info.Subject = set.PdfSubject
	}
	if set.PdfKeywords != "" {
		doc.Info.Keywords = set.PdfKeywords
	}

	for _, e := range result.Elements {
		doc.Add(e)
	}

	// Add absolutely positioned elements (position: absolute/fixed).
	for _, abs := range result.Absolutes {
		doc.AddAbsoluteWithOpts(abs.Element, abs.X, abs.Y, abs.Width, layout.AbsoluteOpts{
			RightAligned: abs.RightAligned,
			ZIndex:       abs.ZIndex,
			PageIndex:    -1,
			Fixed:        abs.Fixed,
		})
	}

	// Optional watermark (requested by caller, e.g. playground).
	if set.Watermark != "" {
		doc.SetWatermarkConfig(document.WatermarkConfig{
			Text:     set.Watermark,
			FontSize: 54,
			ColorR:   0.001,
			ColorG:   0.001,
			ColorB:   0.001,
			Angle:    -35,
			Opacity:  0.04,
		})
	}

	// Header/footer from HTML.
	if set.HeaderHTML != "" {
		headerElems, err := html.Convert(set.HeaderHTML, nil)
		if err == nil && len(headerElems) > 0 {
			doc.SetHeaderElement(func(_ document.PageContext) layout.Element {
				if len(headerElems) == 1 {
					return headerElems[0]
				}
				div := layout.NewDiv()
				for _, e := range headerElems {
					div.Add(e)
				}
				return div
			})
		}
	}
	if set.FooterHTML != "" {
		footerElems, err := html.Convert(set.FooterHTML, nil)
		if err == nil && len(footerElems) > 0 {
			doc.SetFooterElement(func(_ document.PageContext) layout.Element {
				if len(footerElems) == 1 {
					return footerElems[0]
				}
				div := layout.NewDiv()
				for _, e := range footerElems {
					div.Add(e)
				}
				return div
			})
		}
	}

	// PDF profiles
	switch set.PdfProfile {
	case "pdfa1b":
		doc.SetPdfA(document.PdfAConfig{Level: document.PdfA1B})
	case "pdfa1a":
		doc.SetPdfA(document.PdfAConfig{Level: document.PdfA1A})
	case "pdfa2b":
		doc.SetPdfA(document.PdfAConfig{Level: document.PdfA2B})
	case "pdfa2u":
		doc.SetPdfA(document.PdfAConfig{Level: document.PdfA2U})
	case "pdfa2a":
		doc.SetPdfA(document.PdfAConfig{Level: document.PdfA2A})
	case "pdfa3b":
		doc.SetPdfA(document.PdfAConfig{Level: document.PdfA3B})
	case "pdfa3a":
		doc.SetPdfA(document.PdfAConfig{Level: document.PdfA3A})
	case "pdfa4":
		doc.SetPdfA(document.PdfAConfig{Level: document.PdfA4})
	case "pdfa4f":
		doc.SetPdfA(document.PdfAConfig{Level: document.PdfA4F})
	case "pdfa4e":
		doc.SetPdfA(document.PdfAConfig{Level: document.PdfA4E})
	case "pdfua1":
		doc.SetTagged(true)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		return map[string]any{"error": err.Error()}
	}

	return map[string]any{
		"pdf":    base64.StdEncoding.EncodeToString(buf.Bytes()),
		"pages":  doc.PageCount(),
		"size":   buf.Len(),
		"width":  pageSize.Width,
		"height": pageSize.Height,
	}
}

func main() {
	js.Global().Set("folioRender", js.FuncOf(renderHTML))
	select {}
}
