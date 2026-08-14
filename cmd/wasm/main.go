//go:build js && wasm

// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"syscall/js"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/layout"
)

type marginsSettings struct {
	Top    float64 `json:"top"`
	Right  float64 `json:"right"`
	Bottom float64 `json:"bottom"`
	Left   float64 `json:"left"`
}

type renderSettings struct {
	PageSize             string          `json:"pageSize"`
	Orientation          string          `json:"orientation"`           // "portrait" (default) or "landscape"
	Margins              *marginsSettings `json:"margins,omitempty"`    // optional page margins in points
	BasePath             string          `json:"basePath,omitempty"`    // local asset root (maps to html.Options.BaseFS)
	FallbackFontPath     string          `json:"fallbackFontPath"`      // Unicode-capable TTF/OTF for non-WinAnsi chars
	AllowAbsolutePaths   bool            `json:"allowAbsolutePaths"`    // allow reading absolute paths when BaseFS is nil
	MediaType            string          `json:"mediaType"`
	PdfProfile           string          `json:"pdfProfile"`
	PdfTitle             string          `json:"pdfTitle"`
	IgnoreResourceErrors bool            `json:"ignoreResourceErrors"`
	CssDpi               int             `json:"cssDpi"`
	Watermark            string          `json:"watermark,omitempty"`  // optional watermark text
	HeaderHTML           string          `json:"headerHtml,omitempty"` // HTML for page header
	FooterHTML           string          `json:"footerHtml,omitempty"` // HTML for page footer
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

	if settings.Orientation == "landscape" {
		pageSize = pageSize.Landscape()
	}

	opts := &html.Options{
		PageWidth:           pageSize.Width,
		PageHeight:          pageSize.Height,
		FallbackFontPath:    settings.FallbackFontPath,
		AllowAbsolutePaths:  settings.AllowAbsolutePaths,
	}
	if settings.BasePath != "" {
		opts.BaseFS = os.DirFS(settings.BasePath)
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

	// Apply user-specified margins (overrides @page defaults).
	if settings.Margins != nil {
		doc.SetMargins(layout.Margins{
			Top:    settings.Margins.Top,
			Right:  settings.Margins.Right,
			Bottom: settings.Margins.Bottom,
			Left:   settings.Margins.Left,
		})
	}

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
			Fixed:        abs.Fixed,
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

	// Header/footer from HTML.
	if settings.HeaderHTML != "" {
		headerElems, err := html.Convert(settings.HeaderHTML, nil)
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
	if settings.FooterHTML != "" {
		footerElems, err := html.Convert(settings.FooterHTML, nil)
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
	switch settings.PdfProfile {
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
