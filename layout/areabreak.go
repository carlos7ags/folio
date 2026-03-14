// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

// AreaBreak is a layout element that forces a page break.
// When the renderer encounters an AreaBreak, it starts a new page
// and continues laying out subsequent elements there.
type AreaBreak struct{}

// NewAreaBreak creates an element that forces a page break.
func NewAreaBreak() *AreaBreak {
	return &AreaBreak{}
}

// Layout returns a single line with a page break marker.
// Retained for internal use; the renderer handles AreaBreak by type assertion.
func (ab *AreaBreak) Layout(maxWidth float64) []Line {
	return []Line{{
		Height:    0,
		IsLast:    true,
		areaBreak: true,
	}}
}

// PlanLayout implements Element. AreaBreak is handled by type assertion
// in the renderer, so this returns an empty plan.
func (ab *AreaBreak) PlanLayout(area LayoutArea) LayoutPlan {
	return LayoutPlan{Status: LayoutFull, Consumed: 0}
}
