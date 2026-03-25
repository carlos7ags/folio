//go:build js && wasm

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"syscall/js"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

type renderSettings struct {
	PageSize             string `json:"pageSize"`
	MediaType            string `json:"mediaType"`
	PdfProfile           string `json:"pdfProfile"`
	PdfTitle             string `json:"pdfTitle"`
	IgnoreResourceErrors bool   `json:"ignoreResourceErrors"`
	CssDpi               int    `json:"cssDpi"`
	Watermark            string `json:"watermark,omitempty"` // optional watermark text
}

func renderHTML(_ js.Value, args []js.Value) any {
	if len(args) < 2 {
		return map[string]any{"error": "expected 2 arguments: html, settingsJSON"}
	}

	htmlStr := args[0].String()
	settingsJSON := args[1].String()

	var settings renderSettings
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return map[string]any{"error": "invalid settings JSON: " + err.Error()}
	}

	pageSize := document.PageSizeLetter
	switch settings.PageSize {
	case "a4":
		pageSize = document.PageSizeA4
	case "legal":
		pageSize = document.PageSizeLegal
	case "a3":
		pageSize = document.PageSizeA3
	}

	opts := &html.Options{
		PageWidth:  pageSize.Width,
		PageHeight: pageSize.Height,
	}

	result, err := html.ConvertFull(htmlStr, opts)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}

	// Apply @page CSS rules to page size.
	// AutoHeight means height explicitly set to 0 (size page to content).
	if pc := result.PageConfig; pc != nil {
		if pc.Width > 0 && pc.Height > 0 {
			pageSize = document.PageSize{Width: pc.Width, Height: pc.Height}
		} else if pc.Width > 0 && pc.AutoHeight {
			pageSize = document.PageSize{Width: pc.Width, Height: 0}
		}
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

	// Apply margin boxes from @page rules (e.g. page numbers).
	if result.MarginBoxes != nil {
		doc.SetMarginBoxes(result.MarginBoxes)
	}
	if result.FirstMarginBoxes != nil {
		doc.SetFirstMarginBoxes(result.FirstMarginBoxes)
	}

	// Apply metadata from HTML <title>/<meta> tags
	if result.Metadata.Title != "" {
		doc.Info.Title = result.Metadata.Title
	}
	if result.Metadata.Author != "" {
		doc.Info.Author = result.Metadata.Author
	}

	// Override title if user specified one
	if settings.PdfTitle != "" {
		doc.Info.Title = settings.PdfTitle
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
		})
	}

	// Optional watermark (requested by caller, e.g. playground).
	if settings.Watermark != "" {
		doc.SetWatermarkConfig(document.WatermarkConfig{
			Text:     settings.Watermark,
			FontSize: 54,
			ColorR:   0.001,
			ColorG:   0.001,
			ColorB:   0.001,
			Angle:    -35,
			Opacity:  0.04,
		})
	}

	// PDF profiles
	switch settings.PdfProfile {
	case "pdfa1b":
		doc.SetPdfA(document.PdfAConfig{Level: document.PdfA1B})
	case "pdfa2b":
		doc.SetPdfA(document.PdfAConfig{Level: document.PdfA2B})
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
