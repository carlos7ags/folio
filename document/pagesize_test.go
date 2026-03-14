// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"testing"
)

func TestPageSizeDimensions(t *testing.T) {
	sizes := []struct {
		name string
		ps   PageSize
	}{
		{"A0", PageSizeA0},
		{"A1", PageSizeA1},
		{"A2", PageSizeA2},
		{"A3", PageSizeA3},
		{"A4", PageSizeA4},
		{"A5", PageSizeA5},
		{"A6", PageSizeA6},
		{"B4", PageSizeB4},
		{"B5", PageSizeB5},
		{"Letter", PageSizeLetter},
		{"Legal", PageSizeLegal},
		{"Tabloid", PageSizeTabloid},
		{"Ledger", PageSizeLedger},
		{"Executive", PageSizeExecutive},
	}

	for _, tt := range sizes {
		if tt.ps.Width <= 0 || tt.ps.Height <= 0 {
			t.Errorf("%s: dimensions must be positive, got %.2f x %.2f", tt.name, tt.ps.Width, tt.ps.Height)
		}
		// All portrait sizes (except Ledger) should be taller than wide.
		if tt.name != "Ledger" && tt.ps.Height <= tt.ps.Width {
			t.Errorf("%s: expected portrait (height > width), got %.2f x %.2f", tt.name, tt.ps.Width, tt.ps.Height)
		}
	}

	// Ledger is landscape Tabloid.
	if PageSizeLedger.Width != PageSizeTabloid.Height || PageSizeLedger.Height != PageSizeTabloid.Width {
		t.Error("Ledger should be Tabloid rotated 90 degrees")
	}
}

func TestPageSizeLandscape(t *testing.T) {
	landscape := PageSizeA4.Landscape()
	if landscape.Width != PageSizeA4.Height || landscape.Height != PageSizeA4.Width {
		t.Errorf("Landscape() should swap width and height, got %.2f x %.2f", landscape.Width, landscape.Height)
	}

	// Landscape of landscape should return to portrait.
	portrait := landscape.Landscape()
	if portrait.Width != PageSizeA4.Width || portrait.Height != PageSizeA4.Height {
		t.Error("double Landscape() should return to original")
	}
}

func TestPageSizeASeriesDescending(t *testing.T) {
	// Each A-size should be roughly half the area of the previous.
	sizes := []PageSize{PageSizeA0, PageSizeA1, PageSizeA2, PageSizeA3, PageSizeA4, PageSizeA5, PageSizeA6}
	for i := 1; i < len(sizes); i++ {
		prevArea := sizes[i-1].Width * sizes[i-1].Height
		curArea := sizes[i].Width * sizes[i].Height
		ratio := prevArea / curArea
		// Should be approximately 2.0 (within 1% tolerance).
		if ratio < 1.98 || ratio > 2.02 {
			t.Errorf("A%d/A%d area ratio = %.3f, expected ~2.0", i-1, i, ratio)
		}
	}
}
