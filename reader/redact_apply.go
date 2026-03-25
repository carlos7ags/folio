// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"fmt"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/core"
)

// buildRedactionOverlay creates a content stream that draws opaque
// rectangles over the redacted regions and optional overlay text.
func buildRedactionOverlay(rects []Box, opts *RedactOptions) *core.PdfStream {
	s := content.NewStream()
	s.SaveState()

	r, g, b := opts.fillColor()
	s.SetFillColorRGB(r, g, b)

	for _, rect := range rects {
		w := rect.X2 - rect.X1
		h := rect.Y2 - rect.Y1
		s.Rectangle(rect.X1, rect.Y1, w, h)
	}
	s.Fill()

	// Overlay text on each rect.
	if opts.OverlayText != "" {
		or, og, ob := opts.overlayColor()
		s.SetFillColorRGB(or, og, ob)
		fontSize := opts.overlayFontSize()

		for _, rect := range rects {
			w := rect.X2 - rect.X1
			h := rect.Y2 - rect.Y1
			// Center the text in the box. Average character width is ~50%
			// of font size for standard Latin fonts (Helvetica).
			const avgCharWidthRatio = 0.5
			textW := float64(len(opts.OverlayText)) * fontSize * avgCharWidthRatio
			tx := rect.X1 + (w-textW)/2
			if tx < rect.X1 {
				tx = rect.X1
			}
			ty := rect.Y1 + (h-fontSize)/2
			s.BeginText()
			s.SetFont("Helv", fontSize)
			s.MoveText(tx, ty)
			s.ShowText(opts.OverlayText)
			s.EndText()
		}
	}

	s.RestoreState()
	return s.ToPdfStream()
}

// applyRedaction replaces a page's content stream with the rewritten
// content and adds the redaction overlay.
func applyRedaction(m *Modifier, pageIdx int, rewritten []byte, overlay *core.PdfStream, opts *RedactOptions) error {
	if pageIdx < 0 || pageIdx >= len(m.pageDicts) {
		return fmt.Errorf("redact: page index %d out of range", pageIdx)
	}
	pageDict := m.pageDicts[pageIdx]

	// Create rewritten content stream.
	rewrittenStream := core.NewPdfStreamCompressed(rewritten)
	rewrittenRef := m.writer.AddObject(rewrittenStream)

	// Create overlay stream.
	overlayRef := m.writer.AddObject(overlay)

	// Set /Contents to an array: [rewritten, overlay].
	pageDict.Set("Contents", core.NewPdfArray(rewrittenRef, overlayRef))

	// Register Helvetica font resource if overlay text is used.
	if opts.OverlayText != "" {
		ensureHelvFont(pageDict, m.writer)
	}

	return nil
}

// ensureHelvFont adds a Helvetica font entry to the page's Resources
// under the name "Helv" if not already present.
func ensureHelvFont(pageDict *core.PdfDictionary, w interface{ AddObject(core.PdfObject) *core.PdfIndirectReference }) {
	resources := pageDict.Get("Resources")
	var resDict *core.PdfDictionary
	if resources != nil {
		if d, ok := resources.(*core.PdfDictionary); ok {
			resDict = d
		}
	}
	if resDict == nil {
		resDict = core.NewPdfDictionary()
		pageDict.Set("Resources", resDict)
	}

	fonts := resDict.Get("Font")
	var fontDict *core.PdfDictionary
	if fonts != nil {
		if d, ok := fonts.(*core.PdfDictionary); ok {
			fontDict = d
		}
	}
	if fontDict == nil {
		fontDict = core.NewPdfDictionary()
		resDict.Set("Font", fontDict)
	}

	// Only add if not already present.
	if fontDict.Get("Helv") != nil {
		return
	}
	helv := core.NewPdfDictionary()
	helv.Set("Type", core.NewPdfName("Font"))
	helv.Set("Subtype", core.NewPdfName("Type1"))
	helv.Set("BaseFont", core.NewPdfName("Helvetica"))
	helv.Set("Encoding", core.NewPdfName("WinAnsiEncoding"))
	helvRef := w.AddObject(helv)
	fontDict.Set("Helv", helvRef)
}

// stripDocumentMetadata removes /Info and /Metadata from the document.
func stripDocumentMetadata(m *Modifier) {
	// Remove /Info dictionary.
	m.info = nil

	// Remove /Metadata from catalog.
	if m.catalog != nil {
		m.catalog.Remove("Metadata")
		m.catalog.Remove("PieceInfo")
	}

	// Remove per-page metadata.
	for _, pageDict := range m.pageDicts {
		pageDict.Remove("Metadata")
		pageDict.Remove("PieceInfo")
	}
}
