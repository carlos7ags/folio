// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"
)

func TestPageSizeA4(t *testing.T) {
	result, err := ConvertFull(`<html><head><style>@page { size: a4; }</style></head><body><p>Text</p></body></html>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig from @page rule")
	}
	// A4: 595.28 x 841.89
	if math.Abs(result.PageConfig.Width-595.28) > 1 {
		t.Errorf("width = %.2f, want ~595.28", result.PageConfig.Width)
	}
	if math.Abs(result.PageConfig.Height-841.89) > 1 {
		t.Errorf("height = %.2f, want ~841.89", result.PageConfig.Height)
	}
}

func TestPageSizeLetter(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { size: letter; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	if result.PageConfig.Width != 612 || result.PageConfig.Height != 792 {
		t.Errorf("size = %.0fx%.0f, want 612x792", result.PageConfig.Width, result.PageConfig.Height)
	}
}

func TestPageSizeLandscape(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { size: a4 landscape; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	if !result.PageConfig.Landscape {
		t.Error("expected landscape flag")
	}
	// Landscape A4: width > height
	if result.PageConfig.Width <= result.PageConfig.Height {
		t.Errorf("landscape should have width > height, got %.0f x %.0f",
			result.PageConfig.Width, result.PageConfig.Height)
	}
}

func TestPageSizeCustomDimensions(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { size: 8.5in 11in; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	// 8.5in = 612pt, 11in = 792pt
	if math.Abs(result.PageConfig.Width-612) > 1 {
		t.Errorf("width = %.2f, want 612", result.PageConfig.Width)
	}
	if math.Abs(result.PageConfig.Height-792) > 1 {
		t.Errorf("height = %.2f, want 792", result.PageConfig.Height)
	}
}

func TestPageSizeMillimeters(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { size: 210mm 297mm; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	// 210mm ≈ 595.28pt, 297mm ≈ 841.89pt (A4)
	if math.Abs(result.PageConfig.Width-595.28) > 1 {
		t.Errorf("width = %.2f, want ~595.28", result.PageConfig.Width)
	}
}

func TestPageMargins(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { margin: 1in; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	for _, m := range []float64{result.PageConfig.MarginTop, result.PageConfig.MarginRight, result.PageConfig.MarginBottom, result.PageConfig.MarginLeft} {
		if math.Abs(m-72) > 1 {
			t.Errorf("margin = %.2f, want 72 (1in)", m)
		}
	}
}

func TestPageMarginsIndividual(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { margin-top: 2cm; margin-right: 1cm; margin-bottom: 2cm; margin-left: 1cm; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	// 2cm ≈ 56.69pt, 1cm ≈ 28.35pt
	if math.Abs(result.PageConfig.MarginTop-56.69) > 1 {
		t.Errorf("margin-top = %.2f, want ~56.69", result.PageConfig.MarginTop)
	}
	if math.Abs(result.PageConfig.MarginRight-28.35) > 1 {
		t.Errorf("margin-right = %.2f, want ~28.35", result.PageConfig.MarginRight)
	}
}

func TestPageSizeAndMargins(t *testing.T) {
	result, _ := ConvertFull(`<html><head><style>@page { size: a4; margin: 72pt; }</style></head><body><p>X</p></body></html>`, nil)
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	if math.Abs(result.PageConfig.Width-595.28) > 1 {
		t.Errorf("width = %.2f, want ~595.28", result.PageConfig.Width)
	}
	if result.PageConfig.MarginTop != 72 {
		t.Errorf("margin-top = %.2f, want 72", result.PageConfig.MarginTop)
	}
}

func TestPageSizeAutoHeight(t *testing.T) {
	// @page { size: 80mm 0; } should set width and AutoHeight=true.
	result, err := ConvertFull(`<html><head><style>@page { size: 80mm 0; margin: 0; }</style></head><body><p>Receipt</p></body></html>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig from @page rule")
	}
	// 80mm ≈ 226.77pt
	if math.Abs(result.PageConfig.Width-226.77) > 1 {
		t.Errorf("width = %.2f, want ~226.77", result.PageConfig.Width)
	}
	if result.PageConfig.Height != 0 {
		t.Errorf("height = %.2f, want 0 (auto-height)", result.PageConfig.Height)
	}
	if !result.PageConfig.AutoHeight {
		t.Error("expected AutoHeight=true for size: 80mm 0")
	}
}

func TestPageSizeAutoHeight210mm(t *testing.T) {
	// @page { size: 210mm 0; } — flyer-style auto-height.
	result, err := ConvertFull(`<html><head><style>@page { size: 210mm 0; margin: 0; }</style></head><body><h1>Hello</h1></body></html>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.PageConfig == nil {
		t.Fatal("expected PageConfig")
	}
	if math.Abs(result.PageConfig.Width-595.28) > 1 {
		t.Errorf("width = %.2f, want ~595.28", result.PageConfig.Width)
	}
	if !result.PageConfig.AutoHeight {
		t.Error("expected AutoHeight=true")
	}
}

func TestNoPageRule(t *testing.T) {
	result, _ := ConvertFull(`<p>No page rule</p>`, nil)
	if result.PageConfig != nil {
		t.Error("expected nil PageConfig when no @page rule")
	}
}

func TestBreakBeforeModernSyntax(t *testing.T) {
	elems, _ := Convert(`<div style="break-before: page">After break</div>`, nil)
	// Should have an AreaBreak before the div content.
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements (AreaBreak + content), got %d", len(elems))
	}
}

func TestBreakAfterModernSyntax(t *testing.T) {
	elems, _ := Convert(`<div style="break-after: page">Before break</div><p>After</p>`, nil)
	// Should have content + AreaBreak + content.
	if len(elems) < 2 {
		t.Fatalf("expected at least 2 elements, got %d", len(elems))
	}
}

func TestOrphansWidowsCSS(t *testing.T) {
	// Orphans/widows are parsed and applied to paragraphs.
	// We can't easily test the visual effect, but verify parsing doesn't error.
	elems, err := Convert(`<p style="orphans: 3; widows: 2">Text content here.</p>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Error("expected elements")
	}
}

func TestBreakInsideAvoid(t *testing.T) {
	elems, err := Convert(`<div style="break-inside: avoid"><p>Keep together</p></div>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Error("expected elements")
	}
}
