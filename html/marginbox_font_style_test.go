// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import "testing"

// TestMarginBoxItalicResolvesObliqueFont verifies that font-style: italic
// on a margin box resolves to the standard oblique variant (issue #378
// gap A), not the plain hard-coded Helvetica.
func TestMarginBoxItalicResolvesObliqueFont(t *testing.T) {
	src := `<!DOCTYPE html><html><head><style>
@page { @bottom-right { content: "X"; font-style: italic; } }
</style></head><body><p>Hi</p></body></html>`

	result, err := ConvertFull(src, nil)
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	box, ok := result.MarginBoxes["bottom-right"]
	if !ok {
		t.Fatalf("no bottom-right margin box; got %v", result.MarginBoxes)
	}
	if box.Font == nil {
		t.Fatal("Font is nil, want Helvetica-Oblique")
	}
	if box.Font.Name() != "Helvetica-Oblique" {
		t.Errorf("Font.Name() = %q, want Helvetica-Oblique", box.Font.Name())
	}
	if box.Embedded != nil {
		t.Error("Embedded is non-nil, want nil for a document with no @font-face")
	}
}

// TestMarginBoxBoldResolvesBoldFont verifies font-weight: bold resolves to
// the standard bold variant.
func TestMarginBoxBoldResolvesBoldFont(t *testing.T) {
	src := `<!DOCTYPE html><html><head><style>
@page { @bottom-right { content: "X"; font-weight: bold; } }
</style></head><body><p>Hi</p></body></html>`

	result, err := ConvertFull(src, nil)
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	box, ok := result.MarginBoxes["bottom-right"]
	if !ok {
		t.Fatalf("no bottom-right margin box; got %v", result.MarginBoxes)
	}
	if box.Font == nil || box.Font.Name() != "Helvetica-Bold" {
		t.Errorf("Font = %v, want Helvetica-Bold", box.Font)
	}
}

// TestMarginBoxBoldItalicResolvesBoldObliqueFont exercises the issue's
// exact repro shape: font-style, font-weight, color, and font-size
// declared together on the same margin box must all be honored.
func TestMarginBoxBoldItalicResolvesBoldObliqueFont(t *testing.T) {
	src := `<!DOCTYPE html><html><head><style>
@page { @bottom-right {
	content: "CONFIDENTIAL";
	font-size: 6pt;
	font-style: italic;
	font-weight: bold;
	color: red;
} }
</style></head><body><p>Hi</p></body></html>`

	result, err := ConvertFull(src, nil)
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	box, ok := result.MarginBoxes["bottom-right"]
	if !ok {
		t.Fatalf("no bottom-right margin box; got %v", result.MarginBoxes)
	}
	if box.Font == nil || box.Font.Name() != "Helvetica-BoldOblique" {
		t.Errorf("Font = %v, want Helvetica-BoldOblique", box.Font)
	}
	if !box.HasColor {
		t.Error("HasColor = false, want true")
	}
	if box.Color != [3]float64{1, 0, 0} {
		t.Errorf("Color = %v, want red {1,0,0}", box.Color)
	}
	if box.FontSize != 6.0 {
		t.Errorf("FontSize = %v, want 6.0", box.FontSize)
	}
}

// TestMarginBoxFontFamilyOverridesStandardFamily verifies font-family on
// a margin box maps to the corresponding standard PDF family when no
// matching @font-face is registered.
func TestMarginBoxFontFamilyOverridesStandardFamily(t *testing.T) {
	src := `<!DOCTYPE html><html><head><style>
@page { @bottom-right { content: "X"; font-family: 'Courier New'; } }
</style></head><body><p>Hi</p></body></html>`

	result, err := ConvertFull(src, nil)
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	box, ok := result.MarginBoxes["bottom-right"]
	if !ok {
		t.Fatalf("no bottom-right margin box; got %v", result.MarginBoxes)
	}
	if box.Font == nil || box.Font.Name() != "Courier" {
		t.Errorf("Font = %v, want Courier", box.Font)
	}
}

// TestMarginBoxNoFontDeclarationKeepsPlainHelvetica pins the "undeclared
// box is unaffected" scope boundary: a box with only content/font-size/
// color set must leave Font nil so the renderer falls back to plain
// Helvetica, exactly as before this fix.
func TestMarginBoxNoFontDeclarationKeepsPlainHelvetica(t *testing.T) {
	src := `<!DOCTYPE html><html><head><style>
@page { @bottom-right { content: "X"; font-size: 8pt; color: blue; } }
</style></head><body><p>Hi</p></body></html>`

	result, err := ConvertFull(src, nil)
	if err != nil {
		t.Fatalf("ConvertFull: %v", err)
	}
	box, ok := result.MarginBoxes["bottom-right"]
	if !ok {
		t.Fatalf("no bottom-right margin box; got %v", result.MarginBoxes)
	}
	if box.Font != nil {
		t.Errorf("Font = %v, want nil (falls back to plain Helvetica)", box.Font)
	}
}
