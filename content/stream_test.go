// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"math"
	"strings"
	"testing"
)

func TestBeginEndText(t *testing.T) {
	s := NewStream()
	s.BeginText()
	s.EndText()
	got := string(s.Bytes())
	if got != "BT\nET" {
		t.Errorf("expected %q, got %q", "BT\nET", got)
	}
}

func TestSetFont(t *testing.T) {
	s := NewStream()
	s.SetFont("F1", 12)
	got := string(s.Bytes())
	if got != "/F1 12 Tf" {
		t.Errorf("expected %q, got %q", "/F1 12 Tf", got)
	}
}

func TestSetFontFractional(t *testing.T) {
	s := NewStream()
	s.SetFont("F1", 10.5)
	got := string(s.Bytes())
	if got != "/F1 10.5 Tf" {
		t.Errorf("expected %q, got %q", "/F1 10.5 Tf", got)
	}
}

func TestMoveText(t *testing.T) {
	s := NewStream()
	s.MoveText(100, 700)
	got := string(s.Bytes())
	if got != "100 700 Td" {
		t.Errorf("expected %q, got %q", "100 700 Td", got)
	}
}

func TestMoveTextFractional(t *testing.T) {
	s := NewStream()
	s.MoveText(72.5, 300.25)
	got := string(s.Bytes())
	if got != "72.5 300.25 Td" {
		t.Errorf("expected %q, got %q", "72.5 300.25 Td", got)
	}
}

func TestShowText(t *testing.T) {
	s := NewStream()
	s.ShowText("Hello World")
	got := string(s.Bytes())
	if got != "(Hello World) Tj" {
		t.Errorf("expected %q, got %q", "(Hello World) Tj", got)
	}
}

func TestShowTextEscaping(t *testing.T) {
	s := NewStream()
	s.ShowText(`a\b(c)d`)
	got := string(s.Bytes())
	if got != `(a\\b\(c\)d) Tj` {
		t.Errorf("expected %q, got %q", `(a\\b\(c\)d) Tj`, got)
	}
}

func TestSetLeading(t *testing.T) {
	s := NewStream()
	s.SetLeading(14)
	got := string(s.Bytes())
	if got != "14 TL" {
		t.Errorf("expected %q, got %q", "14 TL", got)
	}
}

func TestMoveToNextLine(t *testing.T) {
	s := NewStream()
	s.MoveToNextLine()
	got := string(s.Bytes())
	if got != "T*" {
		t.Errorf("expected %q, got %q", "T*", got)
	}
}

func TestShowTextNextLine(t *testing.T) {
	s := NewStream()
	s.ShowTextNextLine("Second line")
	got := string(s.Bytes())
	if got != "(Second line) '" {
		t.Errorf("expected %q, got %q", "(Second line) '", got)
	}
}

func TestFullTextBlock(t *testing.T) {
	s := NewStream()
	s.BeginText()
	s.SetFont("F1", 12)
	s.MoveText(100, 700)
	s.ShowText("Hello World")
	s.EndText()

	expected := strings.Join([]string{
		"BT",
		"/F1 12 Tf",
		"100 700 Td",
		"(Hello World) Tj",
		"ET",
	}, "\n")

	got := string(s.Bytes())
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestMultiLineText(t *testing.T) {
	s := NewStream()
	s.BeginText()
	s.SetFont("F1", 12)
	s.SetLeading(14)
	s.MoveText(72, 720)
	s.ShowText("Line one")
	s.ShowTextNextLine("Line two")
	s.ShowTextNextLine("Line three")
	s.EndText()

	got := string(s.Bytes())
	if !strings.Contains(got, "(Line one) Tj") {
		t.Error("missing first line")
	}
	if !strings.Contains(got, "(Line two) '") {
		t.Error("missing second line")
	}
	if !strings.Contains(got, "(Line three) '") {
		t.Error("missing third line")
	}
}

func TestToPdfStream(t *testing.T) {
	s := NewStream()
	s.BeginText()
	s.ShowText("test")
	s.EndText()

	ps := s.ToPdfStream()
	if ps == nil {
		t.Fatal("ToPdfStream returned nil")
	}
	if len(ps.Data) == 0 {
		t.Error("stream data is empty")
	}
	// /Length should be set during WriteTo, so just check Data matches
	expected := "BT\n(test) Tj\nET"
	if string(ps.Data) != expected {
		t.Errorf("expected data %q, got %q", expected, string(ps.Data))
	}
}

func TestEmptyStream(t *testing.T) {
	s := NewStream()
	if len(s.Bytes()) != 0 {
		t.Errorf("expected empty bytes, got %q", string(s.Bytes()))
	}
}

// --- Graphics operator tests ---

func TestSaveRestoreState(t *testing.T) {
	s := NewStream()
	s.SaveState()
	s.RestoreState()
	got := string(s.Bytes())
	if got != "q\nQ" {
		t.Errorf("expected %q, got %q", "q\nQ", got)
	}
}

func TestSetLineWidth(t *testing.T) {
	s := NewStream()
	s.SetLineWidth(0.5)
	got := string(s.Bytes())
	if got != "0.5 w" {
		t.Errorf("expected %q, got %q", "0.5 w", got)
	}
}

func TestMoveTo(t *testing.T) {
	s := NewStream()
	s.MoveTo(100, 200)
	got := string(s.Bytes())
	if got != "100 200 m" {
		t.Errorf("expected %q, got %q", "100 200 m", got)
	}
}

func TestLineTo(t *testing.T) {
	s := NewStream()
	s.LineTo(300, 400)
	got := string(s.Bytes())
	if got != "300 400 l" {
		t.Errorf("expected %q, got %q", "300 400 l", got)
	}
}

func TestRectangle(t *testing.T) {
	s := NewStream()
	s.Rectangle(72, 720, 468, 50)
	got := string(s.Bytes())
	if got != "72 720 468 50 re" {
		t.Errorf("expected %q, got %q", "72 720 468 50 re", got)
	}
}

func TestStroke(t *testing.T) {
	s := NewStream()
	s.Stroke()
	if string(s.Bytes()) != "S" {
		t.Errorf("expected 'S', got %q", string(s.Bytes()))
	}
}

func TestFill(t *testing.T) {
	s := NewStream()
	s.Fill()
	if string(s.Bytes()) != "f" {
		t.Errorf("expected 'f', got %q", string(s.Bytes()))
	}
}

func TestSetStrokeColorRGB(t *testing.T) {
	s := NewStream()
	s.SetStrokeColorRGB(1, 0, 0)
	got := string(s.Bytes())
	if got != "1 0 0 RG" {
		t.Errorf("expected %q, got %q", "1 0 0 RG", got)
	}
}

func TestSetFillColorRGB(t *testing.T) {
	s := NewStream()
	s.SetFillColorRGB(0, 0.5, 1)
	got := string(s.Bytes())
	if got != "0 0.5 1 rg" {
		t.Errorf("expected %q, got %q", "0 0.5 1 rg", got)
	}
}

func TestSetStrokeColorGray(t *testing.T) {
	s := NewStream()
	s.SetStrokeColorGray(0.5)
	if string(s.Bytes()) != "0.5 G" {
		t.Errorf("expected '0.5 G', got %q", string(s.Bytes()))
	}
}

func TestSetFillColorGray(t *testing.T) {
	s := NewStream()
	s.SetFillColorGray(0)
	if string(s.Bytes()) != "0 g" {
		t.Errorf("expected '0 g', got %q", string(s.Bytes()))
	}
}

func TestDrawLineSequence(t *testing.T) {
	s := NewStream()
	s.SaveState()
	s.SetLineWidth(1)
	s.SetStrokeColorGray(0)
	s.MoveTo(72, 720)
	s.LineTo(540, 720)
	s.Stroke()
	s.RestoreState()

	expected := strings.Join([]string{
		"q", "1 w", "0 G",
		"72 720 m", "540 720 l", "S", "Q",
	}, "\n")
	got := string(s.Bytes())
	if got != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, got)
	}
}

func TestShowTextHex(t *testing.T) {
	s := NewStream()
	s.ShowTextHex([]byte{0x00, 0x48, 0x00, 0x69})
	got := string(s.Bytes())
	expected := "<00480069> Tj"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestConcatMatrix(t *testing.T) {
	s := NewStream()
	s.ConcatMatrix(100, 0, 0, 200, 50, 60)
	got := string(s.Bytes())
	expected := "100 0 0 200 50 60 cm"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestFillAndStroke(t *testing.T) {
	s := NewStream()
	s.FillAndStroke()
	got := string(s.Bytes())
	if got != "B" {
		t.Errorf("expected %q, got %q", "B", got)
	}
}

func TestClosePath(t *testing.T) {
	s := NewStream()
	s.ClosePath()
	got := string(s.Bytes())
	if got != "h" {
		t.Errorf("expected %q, got %q", "h", got)
	}
}

func TestDo(t *testing.T) {
	s := NewStream()
	s.Do("Im1")
	got := string(s.Bytes())
	expected := "/Im1 Do"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// --- Sprint A: New text operators ---

func TestSetCharSpacing(t *testing.T) {
	s := NewStream()
	s.SetCharSpacing(0.5)
	got := string(s.Bytes())
	if got != "0.5 Tc" {
		t.Errorf("expected %q, got %q", "0.5 Tc", got)
	}
}

func TestSetWordSpacing(t *testing.T) {
	s := NewStream()
	s.SetWordSpacing(2.5)
	got := string(s.Bytes())
	if got != "2.5 Tw" {
		t.Errorf("expected %q, got %q", "2.5 Tw", got)
	}
}

func TestSetTextRise(t *testing.T) {
	s := NewStream()
	s.SetTextRise(5)
	got := string(s.Bytes())
	if got != "5 Ts" {
		t.Errorf("expected %q, got %q", "5 Ts", got)
	}
}

func TestSetTextRiseNegative(t *testing.T) {
	s := NewStream()
	s.SetTextRise(-3)
	got := string(s.Bytes())
	if got != "-3 Ts" {
		t.Errorf("expected %q, got %q", "-3 Ts", got)
	}
}

func TestSetTextRenderingMode(t *testing.T) {
	tests := []struct {
		mode     int
		expected string
	}{
		{TextRenderFill, "0 Tr"},
		{TextRenderStroke, "1 Tr"},
		{TextRenderFillStroke, "2 Tr"},
		{TextRenderInvisible, "3 Tr"},
	}
	for _, tc := range tests {
		s := NewStream()
		s.SetTextRenderingMode(tc.mode)
		got := string(s.Bytes())
		if got != tc.expected {
			t.Errorf("mode %d: expected %q, got %q", tc.mode, tc.expected, got)
		}
	}
}

func TestSetTextMatrix(t *testing.T) {
	s := NewStream()
	s.SetTextMatrix(1, 0, 0, 1, 72, 700)
	got := string(s.Bytes())
	if got != "1 0 0 1 72 700 Tm" {
		t.Errorf("expected %q, got %q", "1 0 0 1 72 700 Tm", got)
	}
}

// --- Sprint A: New graphics state operators ---

func TestSetLineCap(t *testing.T) {
	s := NewStream()
	s.SetLineCap(LineCapRound)
	got := string(s.Bytes())
	if got != "1 J" {
		t.Errorf("expected %q, got %q", "1 J", got)
	}
}

func TestSetLineJoin(t *testing.T) {
	s := NewStream()
	s.SetLineJoin(LineJoinBevel)
	got := string(s.Bytes())
	if got != "2 j" {
		t.Errorf("expected %q, got %q", "2 j", got)
	}
}

func assertPanics(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("%s: expected panic, got none", name)
		}
	}()
	fn()
}

func TestSetLineCapInvalid(t *testing.T) {
	s := NewStream()
	// Valid values should not panic.
	for _, v := range []int{0, 1, 2} {
		s.SetLineCap(v)
	}
	// Invalid values should panic.
	assertPanics(t, "SetLineCap(-1)", func() { s.SetLineCap(-1) })
	assertPanics(t, "SetLineCap(3)", func() { s.SetLineCap(3) })
}

func TestSetLineJoinInvalid(t *testing.T) {
	s := NewStream()
	for _, v := range []int{0, 1, 2} {
		s.SetLineJoin(v)
	}
	assertPanics(t, "SetLineJoin(-1)", func() { s.SetLineJoin(-1) })
	assertPanics(t, "SetLineJoin(3)", func() { s.SetLineJoin(3) })
}

func TestSetTextRenderingModeInvalid(t *testing.T) {
	s := NewStream()
	for v := range 8 {
		s.SetTextRenderingMode(v)
	}
	assertPanics(t, "SetTextRenderingMode(-1)", func() { s.SetTextRenderingMode(-1) })
	assertPanics(t, "SetTextRenderingMode(8)", func() { s.SetTextRenderingMode(8) })
}

func TestSetMiterLimit(t *testing.T) {
	s := NewStream()
	s.SetMiterLimit(10)
	got := string(s.Bytes())
	if got != "10 M" {
		t.Errorf("expected %q, got %q", "10 M", got)
	}
}

func TestSetDashPattern(t *testing.T) {
	s := NewStream()
	s.SetDashPattern([]float64{3, 2}, 0)
	got := string(s.Bytes())
	if got != "[3 2] 0 d" {
		t.Errorf("expected %q, got %q", "[3 2] 0 d", got)
	}
}

func TestSetDashPatternSolid(t *testing.T) {
	s := NewStream()
	s.SetDashPattern(nil, 0)
	got := string(s.Bytes())
	if got != "[] 0 d" {
		t.Errorf("expected %q, got %q", "[] 0 d", got)
	}
}

func TestSetExtGState(t *testing.T) {
	s := NewStream()
	s.SetExtGState("GS1")
	got := string(s.Bytes())
	if got != "/GS1 gs" {
		t.Errorf("expected %q, got %q", "/GS1 gs", got)
	}
}

// --- Sprint A: Bézier curves ---

func TestCurveTo(t *testing.T) {
	s := NewStream()
	s.CurveTo(10, 20, 30, 40, 50, 60)
	got := string(s.Bytes())
	if got != "10 20 30 40 50 60 c" {
		t.Errorf("expected %q, got %q", "10 20 30 40 50 60 c", got)
	}
}

func TestCurveToV(t *testing.T) {
	s := NewStream()
	s.CurveToV(30, 40, 50, 60)
	got := string(s.Bytes())
	if got != "30 40 50 60 v" {
		t.Errorf("expected %q, got %q", "30 40 50 60 v", got)
	}
}

func TestCurveToY(t *testing.T) {
	s := NewStream()
	s.CurveToY(10, 20, 50, 60)
	got := string(s.Bytes())
	if got != "10 20 50 60 y" {
		t.Errorf("expected %q, got %q", "10 20 50 60 y", got)
	}
}

func TestClipNonZero(t *testing.T) {
	s := NewStream()
	s.ClipNonZero()
	if string(s.Bytes()) != "W" {
		t.Errorf("expected 'W', got %q", string(s.Bytes()))
	}
}

func TestClipEvenOdd(t *testing.T) {
	s := NewStream()
	s.ClipEvenOdd()
	if string(s.Bytes()) != "W*" {
		t.Errorf("expected 'W*', got %q", string(s.Bytes()))
	}
}

func TestEndPath(t *testing.T) {
	s := NewStream()
	s.EndPath()
	if string(s.Bytes()) != "n" {
		t.Errorf("expected 'n', got %q", string(s.Bytes()))
	}
}

func TestFillEvenOdd(t *testing.T) {
	s := NewStream()
	s.FillEvenOdd()
	if string(s.Bytes()) != "f*" {
		t.Errorf("expected 'f*', got %q", string(s.Bytes()))
	}
}

func TestClosePathStroke(t *testing.T) {
	s := NewStream()
	s.ClosePathStroke()
	if string(s.Bytes()) != "s" {
		t.Errorf("expected 's', got %q", string(s.Bytes()))
	}
}

func TestClosePathFillAndStroke(t *testing.T) {
	s := NewStream()
	s.ClosePathFillAndStroke()
	if string(s.Bytes()) != "b" {
		t.Errorf("expected 'b', got %q", string(s.Bytes()))
	}
}

// --- Sprint A: Convenience helpers ---

func TestCircle(t *testing.T) {
	s := NewStream()
	s.Circle(100, 200, 50)
	got := string(s.Bytes())
	// Should start with MoveTo at (cx+r, cy)
	if !strings.Contains(got, "150 200 m") {
		t.Error("circle should start at (cx+r, cy)")
	}
	// Should contain 4 Bézier curves (c operator)
	count := strings.Count(got, " c\n") + strings.Count(got, " c")
	// The last curve may not have a trailing newline before 'h'
	if count < 4 {
		t.Errorf("expected 4 Bézier curves, got %d", count)
	}
	// Should close path
	if !strings.Contains(got, "\nh") {
		t.Error("circle should close path")
	}
}

func TestEllipse(t *testing.T) {
	s := NewStream()
	s.Ellipse(100, 200, 60, 30)
	got := string(s.Bytes())
	// Should start at (cx+rx, cy) = (160, 200)
	if !strings.Contains(got, "160 200 m") {
		t.Error("ellipse should start at (cx+rx, cy)")
	}
}

func TestRoundedRect(t *testing.T) {
	s := NewStream()
	s.RoundedRect(10, 20, 100, 50, 5)
	got := string(s.Bytes())
	// Should start with MoveTo at (x+r, y) = (15, 20)
	if !strings.Contains(got, "15 20 m") {
		t.Error("rounded rect should start at (x+r, y)")
	}
	// 4 lines + 4 curves + close
	if !strings.Contains(got, " l\n") {
		t.Error("rounded rect should contain line segments")
	}
	if strings.Count(got, " c\n")+strings.Count(got, " c") < 4 {
		t.Error("rounded rect should contain 4 Bézier curves for corners")
	}
}

func TestRoundedRectClampedRadius(t *testing.T) {
	s := NewStream()
	// radius 100 is larger than half the height (25), should clamp to 25
	s.RoundedRect(0, 0, 100, 50, 100)
	got := string(s.Bytes())
	// Should start at (x+r, y) = (25, 0) since r is clamped to 25
	if !strings.Contains(got, "25 0 m") {
		t.Errorf("expected clamped radius, got:\n%s", got)
	}
}

func TestShowTextArray(t *testing.T) {
	s := NewStream()
	s.ShowTextArray([]TextArrayElement{
		{Text: "H"},
		{Adjustment: -80, IsAdjustment: true},
		{Text: "ello"},
	})
	got := string(s.Bytes())
	if !strings.Contains(got, "TJ") {
		t.Error("missing TJ operator")
	}
	if !strings.Contains(got, "(H)") {
		t.Error("missing text segment 'H'")
	}
	if !strings.Contains(got, "-80") {
		t.Error("missing kern adjustment")
	}
	if !strings.Contains(got, "(ello)") {
		t.Error("missing text segment 'ello'")
	}
}

func TestShowTextArrayNoKerning(t *testing.T) {
	s := NewStream()
	s.ShowTextArray([]TextArrayElement{
		{Text: "Hello"},
	})
	got := string(s.Bytes())
	expected := "[(Hello) ] TJ"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestShowTextArrayHex(t *testing.T) {
	s := NewStream()
	s.ShowTextArrayHex([]TextArrayElement{
		{HexData: []byte{0x00, 0x48}},
		{Adjustment: -50, IsAdjustment: true},
		{HexData: []byte{0x00, 0x65}},
	})
	got := string(s.Bytes())
	if !strings.Contains(got, "TJ") {
		t.Error("missing TJ operator")
	}
	if !strings.Contains(got, "<0048>") {
		t.Error("missing hex segment")
	}
	if !strings.Contains(got, "-50") {
		t.Error("missing kern adjustment")
	}
}

func TestShowTextArrayEscaping(t *testing.T) {
	s := NewStream()
	s.ShowTextArray([]TextArrayElement{
		{Text: "a(b)"},
	})
	got := string(s.Bytes())
	if !strings.Contains(got, `a\(b\)`) {
		t.Errorf("expected escaped parens, got %q", got)
	}
}

// --- formatNum edge cases ---

func TestFormatNumNaNAndInf(t *testing.T) {
	tests := []struct {
		name  string
		input float64
	}{
		{"NaN", math.NaN()},
		{"+Inf", math.Inf(1)},
		{"-Inf", math.Inf(-1)},
	}
	for _, tc := range tests {
		got := formatNum(tc.input)
		if got != "0" {
			t.Errorf("formatNum(%s): expected %q, got %q", tc.name, "0", got)
		}
	}
}

func TestFormatNumPrecision(t *testing.T) {
	tests := []struct {
		input    float64
		expected string
	}{
		{0.000001, "0.000001"},
		{1.123456, "1.123456"},
		{0.1, "0.1"},
		{0.00001, "0.00001"},
	}
	for _, tc := range tests {
		got := formatNum(tc.input)
		if got != tc.expected {
			t.Errorf("formatNum(%v): expected %q, got %q", tc.input, tc.expected, got)
		}
	}
}
