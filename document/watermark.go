// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"fmt"
	"math"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/font"
)

// WatermarkConfig holds parameters for a diagonal text watermark
// rendered behind content on every page.
type WatermarkConfig struct {
	Text     string  // the watermark text (required)
	FontSize float64 // font size in points (default 60)
	ColorR   float64 // red component 0-1 (default 0.85)
	ColorG   float64 // green component 0-1 (default 0.85)
	ColorB   float64 // blue component 0-1 (default 0.85)
	Angle    float64 // rotation angle in degrees (default 45)
	Opacity  float64 // fill opacity 0-1 (default 0.3)
}

// SetWatermark sets a diagonal text watermark rendered behind content on every page.
// Uses default styling: 60pt light gray Helvetica at 45 degrees, 0.3 opacity.
func (d *Document) SetWatermark(text string) {
	d.watermark = &WatermarkConfig{
		Text:     text,
		FontSize: 60,
		ColorR:   0.85,
		ColorG:   0.85,
		ColorB:   0.85,
		Angle:    45,
		Opacity:  0.3,
	}
}

// SetWatermarkConfig sets a watermark with full control over styling.
// Zero-value fields are replaced with defaults: FontSize=60, Color=(0.85,0.85,0.85),
// Angle=45, Opacity=0.3.
func (d *Document) SetWatermarkConfig(cfg WatermarkConfig) {
	if cfg.FontSize == 0 {
		cfg.FontSize = 60
	}
	if cfg.ColorR == 0 && cfg.ColorG == 0 && cfg.ColorB == 0 {
		cfg.ColorR = 0.85
		cfg.ColorG = 0.85
		cfg.ColorB = 0.85
	}
	if cfg.Angle == 0 {
		cfg.Angle = 45
	}
	if cfg.Opacity == 0 {
		cfg.Opacity = 0.3
	}
	d.watermark = &cfg
}

// applyWatermark appends watermark drawing commands to a page's content stream
// and registers the required font and ExtGState resources on the page.
func (d *Document) applyWatermark(p *Page) {
	wm := d.watermark
	ps := p.effectiveSize()

	// Register Helvetica font resource for the watermark.
	wmFontName := p.standardFontResource(font.Helvetica)

	// Register ExtGState for opacity.
	gsDict := core.NewPdfDictionary()
	gsDict.Set("Type", core.NewPdfName("ExtGState"))
	gsDict.Set("ca", core.NewPdfReal(wm.Opacity))
	gsDict.Set("CA", core.NewPdfReal(wm.Opacity))
	wmGSName := fmt.Sprintf("GS%d", len(p.extGStates)+1)
	p.extGStates = append(p.extGStates, extGStateResource{name: wmGSName, dict: gsDict})

	// Build watermark content stream commands.
	wmStream := content.NewStream()
	wmStream.SaveState()
	wmStream.SetExtGState(wmGSName)
	wmStream.BeginText()
	wmStream.SetFont(wmFontName, wm.FontSize)
	wmStream.SetFillColorRGB(wm.ColorR, wm.ColorG, wm.ColorB)

	// Compute rotation matrix centered on page.
	// Offset by half the text width so the text is visually centered.
	rad := wm.Angle * math.Pi / 180.0
	cosA := math.Cos(rad)
	sinA := math.Sin(rad)
	textWidth := font.Helvetica.MeasureString(wm.Text, wm.FontSize)
	cx := ps.Width/2 - (textWidth/2)*cosA
	cy := ps.Height/2 - (textWidth/2)*sinA
	wmStream.SetTextMatrix(cosA, sinA, -sinA, cosA, cx, cy)

	wmStream.ShowText(font.WinAnsiEncode(wm.Text))
	wmStream.EndText()
	wmStream.RestoreState()

	// Append watermark commands after existing page content so it renders
	// on top — visible regardless of background color.
	p.ensureStream()
	p.stream.AppendBytes(wmStream.Bytes())
}
