// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"math"
	"strings"
	"testing"
)

func TestContentProcessorBasic(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n100 700 Td\n(Hello World) Tj\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}
	if spans[0].Text != "Hello World" {
		t.Errorf("text = %q, want %q", spans[0].Text, "Hello World")
	}
	if spans[0].X != 100 {
		t.Errorf("X = %.1f, want 100", spans[0].X)
	}
	if spans[0].Y != 700 {
		t.Errorf("Y = %.1f, want 700", spans[0].Y)
	}
	if spans[0].Font != "F1" {
		t.Errorf("Font = %q, want F1", spans[0].Font)
	}
	if spans[0].Height != 12 {
		t.Errorf("Height = %.1f, want 12", spans[0].Height)
	}
}

func TestContentProcessorColor(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n1 0 0 rg\n72 700 Td\n(Red text) Tj\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	if len(spans) == 0 {
		t.Fatal("expected span")
	}
	if spans[0].Color != [3]float64{1, 0, 0} {
		t.Errorf("Color = %v, want [1 0 0]", spans[0].Color)
	}
}

func TestContentProcessorGrayColor(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n0.5 g\n72 700 Td\n(Gray) Tj\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	if len(spans) == 0 {
		t.Fatal("expected span")
	}
	if spans[0].Color != [3]float64{0.5, 0.5, 0.5} {
		t.Errorf("Color = %v, want [0.5 0.5 0.5]", spans[0].Color)
	}
}

func TestContentProcessorCTM(t *testing.T) {
	// Scale 2x, then draw text at 50,350 → user space 100,700
	data := []byte("2 0 0 2 0 0 cm\nBT\n/F1 12 Tf\n50 350 Td\n(Scaled) Tj\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	if len(spans) == 0 {
		t.Fatal("expected span")
	}
	if math.Abs(spans[0].X-100) > 0.1 {
		t.Errorf("X = %.1f, want 100 (50 * 2)", spans[0].X)
	}
	if math.Abs(spans[0].Y-700) > 0.1 {
		t.Errorf("Y = %.1f, want 700 (350 * 2)", spans[0].Y)
	}
	// Font size should be scaled too.
	if math.Abs(spans[0].Height-24) > 0.1 {
		t.Errorf("Height = %.1f, want 24 (12 * 2)", spans[0].Height)
	}
}

func TestContentProcessorSaveRestore(t *testing.T) {
	data := []byte("q\n1 0 0 rg\nBT\n/F1 12 Tf\n72 700 Td\n(Red) Tj\nET\nQ\nBT\n/F1 12 Tf\n72 680 Td\n(Black) Tj\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	if len(spans) < 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	// First span: red (set inside q...Q).
	if spans[0].Color != [3]float64{1, 0, 0} {
		t.Errorf("span 0 color = %v, want [1 0 0]", spans[0].Color)
	}
	// Second span: black (restored after Q).
	if spans[1].Color != [3]float64{0, 0, 0} {
		t.Errorf("span 1 color = %v, want [0 0 0]", spans[1].Color)
	}
}

func TestContentProcessorMultipleSpans(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n72 700 Td\n(Line 1) Tj\n0 -14.4 Td\n(Line 2) Tj\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	if spans[0].Text != "Line 1" {
		t.Errorf("span 0 = %q", spans[0].Text)
	}
	if spans[1].Text != "Line 2" {
		t.Errorf("span 1 = %q", spans[1].Text)
	}
	// Line 2 should be below line 1.
	if spans[1].Y >= spans[0].Y {
		t.Error("line 2 should have lower Y than line 1")
	}
}

func TestContentProcessorTJKerning(t *testing.T) {
	// TJ with kerning adjustments.
	data := []byte("BT\n/F1 12 Tf\n72 700 Td\n[(H) -80 (ello)] TJ\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	// Should produce two spans: "H" and "ello".
	if len(spans) < 2 {
		t.Fatalf("expected at least 2 spans, got %d", len(spans))
	}
	combined := ""
	for _, s := range spans {
		combined += s.Text
	}
	if combined != "Hello" {
		t.Errorf("combined = %q, want Hello", combined)
	}
}

// --- Strategy tests ---

func TestSimpleStrategy(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n72 700 Td\n(Hello World) Tj\nET")
	result := ExtractWithStrategy(data, nil, &SimpleStrategy{})
	if result != "Hello World" {
		t.Errorf("result = %q, want %q", result, "Hello World")
	}
}

func TestSimpleStrategyMultiLine(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n72 700 Td\n(Line 1) Tj\n0 -20 Td\n(Line 2) Tj\nET")
	result := ExtractWithStrategy(data, nil, &SimpleStrategy{})
	if !strings.Contains(result, "Line 1") || !strings.Contains(result, "Line 2") {
		t.Errorf("result = %q, expected both lines", result)
	}
	if !strings.Contains(result, "\n") {
		t.Error("expected newline between lines")
	}
}

func TestLocationStrategy(t *testing.T) {
	// Draw text in reverse order — LocationStrategy should sort it.
	data := []byte("BT\n/F1 12 Tf\n72 680 Td\n(Second) Tj\nET\nBT\n/F1 12 Tf\n72 700 Td\n(First) Tj\nET")
	result := ExtractWithStrategy(data, nil, &LocationStrategy{})
	firstIdx := strings.Index(result, "First")
	secondIdx := strings.Index(result, "Second")
	if firstIdx < 0 || secondIdx < 0 {
		t.Fatalf("result = %q, expected both words", result)
	}
	if firstIdx > secondIdx {
		t.Errorf("LocationStrategy should put 'First' (higher Y) before 'Second'")
	}
}

func TestRegionStrategy(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n72 700 Td\n(Inside) Tj\nET\nBT\n/F1 12 Tf\n400 700 Td\n(Outside) Tj\nET")
	inner := &SimpleStrategy{}
	region := NewRegionStrategy(50, 690, 200, 30, inner)
	result := ExtractWithStrategy(data, nil, region)

	if !strings.Contains(result, "Inside") {
		t.Errorf("result = %q, should contain 'Inside'", result)
	}
	if strings.Contains(result, "Outside") {
		t.Errorf("result = %q, should NOT contain 'Outside'", result)
	}
}

func TestExtractWithStrategyEmpty(t *testing.T) {
	result := ExtractWithStrategy([]byte(""), nil, &SimpleStrategy{})
	if result != "" {
		t.Errorf("empty input should produce empty result, got %q", result)
	}
}

// --- Path extraction ---

func TestPathExtraction(t *testing.T) {
	data := []byte("1 0 0 RG\n2 w\n72 700 m\n200 700 l\nS")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	proc.Process(ops)

	paths := proc.Paths()
	if len(paths) < 2 {
		t.Fatalf("expected at least 2 path ops (move + line), got %d", len(paths))
	}

	// Should have stroke color.
	found := false
	for _, p := range paths {
		if p.Painted == PaintStroke {
			found = true
			if p.StrokeColor != [3]float64{1, 0, 0} {
				t.Errorf("stroke color = %v, want [1 0 0]", p.StrokeColor)
			}
		}
	}
	if !found {
		t.Error("expected a stroked path")
	}
}

func TestRectanglePath(t *testing.T) {
	data := []byte("72 700 200 50 re\nf")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	proc.Process(ops)

	paths := proc.Paths()
	hasRect := false
	for _, p := range paths {
		if p.Type == PathRect {
			hasRect = true
		}
	}
	if !hasRect {
		t.Error("expected rectangle path")
	}
}

func TestPathFillAndStroke(t *testing.T) {
	data := []byte("72 700 m\n200 700 l\n200 750 l\nB")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	proc.Process(ops)

	paths := proc.Paths()
	hasFillStroke := false
	for _, p := range paths {
		if p.Painted == PaintFillStroke {
			hasFillStroke = true
		}
	}
	if !hasFillStroke {
		t.Error("expected fill+stroke paint")
	}
}

// --- Image extraction ---

func TestImageExtraction(t *testing.T) {
	// q cm(200, 0, 0, 100, 72, 600) /Im1 Do Q
	data := []byte("q\n200 0 0 100 72 600 cm\n/Im1 Do\nQ")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	proc.Process(ops)

	images := proc.Images()
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	if images[0].Name != "Im1" {
		t.Errorf("name = %q, want Im1", images[0].Name)
	}
	if math.Abs(images[0].X-72) > 0.1 {
		t.Errorf("X = %.1f, want 72", images[0].X)
	}
	if math.Abs(images[0].Y-600) > 0.1 {
		t.Errorf("Y = %.1f, want 600", images[0].Y)
	}
}

func TestMultipleImages(t *testing.T) {
	data := []byte("q\n100 0 0 50 72 700 cm\n/Im1 Do\nQ\nq\n80 0 0 40 300 700 cm\n/Im2 Do\nQ")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	proc.Process(ops)

	if len(proc.Images()) != 2 {
		t.Errorf("expected 2 images, got %d", len(proc.Images()))
	}
}

// --- Glyph extraction ---

func TestGlyphExtraction(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n72 700 Td\n(Hello) Tj\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	proc.SetExtractGlyphs(true)
	proc.Process(ops)

	glyphs := proc.Glyphs()
	if len(glyphs) != 5 {
		t.Fatalf("expected 5 glyphs for 'Hello', got %d", len(glyphs))
	}
	if glyphs[0].Char != 'H' {
		t.Errorf("glyph 0 = %c, want H", glyphs[0].Char)
	}
	if glyphs[4].Char != 'o' {
		t.Errorf("glyph 4 = %c, want o", glyphs[4].Char)
	}
	// Each glyph should have increasing X.
	for i := 1; i < len(glyphs); i++ {
		if glyphs[i].X <= glyphs[i-1].X {
			t.Errorf("glyph %d X (%.1f) should be > glyph %d X (%.1f)",
				i, glyphs[i].X, i-1, glyphs[i-1].X)
		}
	}
}

func TestGlyphsDisabledByDefault(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n72 700 Td\n(Hello) Tj\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	proc.Process(ops) // no SetExtractGlyphs(true)

	if len(proc.Glyphs()) != 0 {
		t.Error("glyphs should be empty when not enabled")
	}
}

// --- Combined extraction ---

// --- Invisible text (Tr mode 3) ---

func TestInvisibleTextFiltered(t *testing.T) {
	// Tr 3 = invisible text (used for searchable OCR layers).
	data := []byte("BT\n/F1 12 Tf\n3 Tr\n72 700 Td\n(Hidden) Tj\nET\nBT\n/F1 12 Tf\n0 Tr\n72 680 Td\n(Visible) Tj\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
	if spans[0].Visible {
		t.Error("span 0 should be invisible (Tr=3)")
	}
	if !spans[1].Visible {
		t.Error("span 1 should be visible (Tr=0)")
	}

	// SimpleStrategy should skip invisible text.
	result := ExtractWithStrategy(data, nil, &SimpleStrategy{})
	if strings.Contains(result, "Hidden") {
		t.Error("invisible text should not appear in extraction")
	}
	if !strings.Contains(result, "Visible") {
		t.Error("visible text should appear")
	}
}

func TestInvisibleTextInSpans(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n3 Tr\n72 700 Td\n(OCR layer) Tj\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	if spans[0].Visible {
		t.Error("span should be invisible")
	}
	if spans[0].Text != "OCR layer" {
		t.Errorf("text = %q, want 'OCR layer'", spans[0].Text)
	}
}

// --- Marked content tags ---

func TestMarkedContentTags(t *testing.T) {
	data := []byte("/P BMC\nBT\n/F1 12 Tf\n72 700 Td\n(Tagged text) Tj\nET\nEMC")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	if len(spans) == 0 {
		t.Fatal("expected spans")
	}
	if spans[0].Tag != "P" {
		t.Errorf("tag = %q, want P", spans[0].Tag)
	}
}

func TestNestedMarkedContent(t *testing.T) {
	data := []byte("/Document BMC\n/P BMC\nBT\n/F1 12 Tf\n72 700 Td\n(Nested) Tj\nET\nEMC\nEMC")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	if len(spans) == 0 {
		t.Fatal("expected spans")
	}
	// Innermost tag should be "P".
	if spans[0].Tag != "P" {
		t.Errorf("tag = %q, want P (innermost)", spans[0].Tag)
	}
}

func TestBDCMarkedContent(t *testing.T) {
	data := []byte("/H1 <</MCID 0>> BDC\nBT\n/F1 18 Tf\n72 700 Td\n(Heading) Tj\nET\nEMC")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	if len(spans) == 0 {
		t.Fatal("expected spans")
	}
	if spans[0].Tag != "H1" {
		t.Errorf("tag = %q, want H1", spans[0].Tag)
	}
}

func TestUntaggedContent(t *testing.T) {
	data := []byte("BT\n/F1 12 Tf\n72 700 Td\n(No tag) Tj\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	spans := proc.Process(ops)

	if len(spans) == 0 {
		t.Fatal("expected spans")
	}
	if spans[0].Tag != "" {
		t.Errorf("tag = %q, want empty", spans[0].Tag)
	}
}

// --- Form XObject recursion ---

func TestFormXObjectRecursion(t *testing.T) {
	// Main content references /Fm1 which contains text.
	mainData := []byte("BT\n/F1 12 Tf\n72 700 Td\n(Main) Tj\nET\n/Fm1 Do")
	formData := []byte("BT\n/F1 10 Tf\n72 600 Td\n(Form content) Tj\nET")

	ops := ParseContentStream(mainData)
	proc := NewContentProcessor(nil)
	proc.SetFormResolver(func(name string) []ContentOp {
		if name == "Fm1" {
			return ParseContentStream(formData)
		}
		return nil
	})
	spans := proc.Process(ops)

	if len(spans) < 2 {
		t.Fatalf("expected at least 2 spans (main + form), got %d", len(spans))
	}
	texts := ""
	for _, s := range spans {
		texts += s.Text + " "
	}
	if !strings.Contains(texts, "Main") {
		t.Error("missing main text")
	}
	if !strings.Contains(texts, "Form content") {
		t.Error("missing form XObject text")
	}
}

func TestFormXObjectNoResolver(t *testing.T) {
	// Without a resolver, Do just records the image ref.
	data := []byte("/Fm1 Do")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	proc.Process(ops)

	if len(proc.Images()) != 1 {
		t.Errorf("expected 1 image ref, got %d", len(proc.Images()))
	}
}

func TestCombinedExtraction(t *testing.T) {
	// Content with text, a path, and an image.
	data := []byte("q\n200 0 0 100 72 600 cm\n/Im1 Do\nQ\n1 0 0 RG\n72 500 m\n200 500 l\nS\nBT\n/F1 12 Tf\n72 400 Td\n(Caption) Tj\nET")
	ops := ParseContentStream(data)
	proc := NewContentProcessor(nil)
	proc.Process(ops)

	if len(proc.Spans()) == 0 {
		t.Error("expected text spans")
	}
	if len(proc.Images()) == 0 {
		t.Error("expected images")
	}
	if len(proc.Paths()) == 0 {
		t.Error("expected paths")
	}
}
