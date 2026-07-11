// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import "fmt"

// debugMediaBoxConfig holds the stroke color and line width for a debug
// MediaBox outline.
type debugMediaBoxConfig struct {
	color     [3]float64
	lineWidth float64
}

// SetDebugMediaBox strokes a rectangle around this page's MediaBox — the
// exact page geometry PDF viewers use — for visual layout debugging.
// Overrides any document-wide SetDebugMediaBox for this page.
// color is [R, G, B] in [0, 1]. Panics if lineWidth is negative.
func (p *Page) SetDebugMediaBox(color [3]float64, lineWidth float64) *Page {
	if lineWidth < 0 {
		panic(fmt.Sprintf("document.Page.SetDebugMediaBox: negative line width %v", lineWidth))
	}
	p.debugMediaBox = &debugMediaBoxConfig{color: color, lineWidth: lineWidth}
	return p
}

// SetDebugMediaBox strokes a rectangle around every page's MediaBox for
// visual layout debugging. A page with its own Page.SetDebugMediaBox
// keeps that per-page setting instead. color is [R, G, B] in [0, 1].
// Panics if lineWidth is negative.
func (d *Document) SetDebugMediaBox(color [3]float64, lineWidth float64) {
	if lineWidth < 0 {
		panic(fmt.Sprintf("document.Document.SetDebugMediaBox: negative line width %v", lineWidth))
	}
	d.debugMediaBox = &debugMediaBoxConfig{color: color, lineWidth: lineWidth}
}

// applyDebugMediaBox draws the MediaBox outline for one page, if either the
// page or the document has a debug config set (page config wins). Must run
// after every other content mutation in buildAllPages so the outline is the
// last thing drawn — on top of headers, footers, absolutes, and watermark.
func (d *Document) applyDebugMediaBox(p *Page) {
	cfg := p.debugMediaBox
	if cfg == nil {
		cfg = d.debugMediaBox
	}
	if cfg == nil {
		return
	}
	ps := p.effectiveSize()
	p.AddRect(0, 0, ps.Width, ps.Height, cfg.lineWidth, cfg.color)
}
