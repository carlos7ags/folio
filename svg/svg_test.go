// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"math"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/content"
)

const epsilon = 0.001

func approxEqual(a, b float64) bool {
	return math.Abs(a-b) < epsilon
}

func assertColor(t *testing.T, label string, got Color, wantR, wantG, wantB, wantA float64) {
	t.Helper()
	if !approxEqual(got.R, wantR) || !approxEqual(got.G, wantG) ||
		!approxEqual(got.B, wantB) || !approxEqual(got.A, wantA) {
		t.Errorf("%s: got Color{R:%.4f, G:%.4f, B:%.4f, A:%.4f}, want {R:%.4f, G:%.4f, B:%.4f, A:%.4f}",
			label, got.R, got.G, got.B, got.A, wantR, wantG, wantB, wantA)
	}
}

func assertMatrix(t *testing.T, label string, got Matrix, wantA, wantB, wantC, wantD, wantE, wantF float64) {
	t.Helper()
	if !approxEqual(got.A, wantA) || !approxEqual(got.B, wantB) ||
		!approxEqual(got.C, wantC) || !approxEqual(got.D, wantD) ||
		!approxEqual(got.E, wantE) || !approxEqual(got.F, wantF) {
		t.Errorf("%s: got Matrix{A:%.4f, B:%.4f, C:%.4f, D:%.4f, E:%.4f, F:%.4f}, want {A:%.4f, B:%.4f, C:%.4f, D:%.4f, E:%.4f, F:%.4f}",
			label, got.A, got.B, got.C, got.D, got.E, got.F, wantA, wantB, wantC, wantD, wantE, wantF)
	}
}

// ---------------------------------------------------------------------------
// Color parsing tests
// ---------------------------------------------------------------------------

func TestParseColor_NamedRed(t *testing.T) {
	c, ok := ParseColor("red")
	if !ok {
		t.Fatal("ParseColor(\"red\") returned ok=false")
	}
	assertColor(t, "red", c, 1, 0, 0, 1)
}

func TestParseColor_NamedBlue(t *testing.T) {
	c, ok := ParseColor("blue")
	if !ok {
		t.Fatal("ParseColor(\"blue\") returned ok=false")
	}
	assertColor(t, "blue", c, 0, 0, 1, 1)
}

func TestParseColor_HexFull(t *testing.T) {
	c, ok := ParseColor("#ff0000")
	if !ok {
		t.Fatal("ParseColor(\"#ff0000\") returned ok=false")
	}
	assertColor(t, "#ff0000", c, 1, 0, 0, 1)
}

func TestParseColor_HexShorthand(t *testing.T) {
	c, ok := ParseColor("#0f0")
	if !ok {
		t.Fatal("ParseColor(\"#0f0\") returned ok=false")
	}
	assertColor(t, "#0f0", c, 0, 1, 0, 1)
}

func TestParseColor_RGB(t *testing.T) {
	c, ok := ParseColor("rgb(255, 0, 0)")
	if !ok {
		t.Fatal("ParseColor(\"rgb(255, 0, 0)\") returned ok=false")
	}
	assertColor(t, "rgb(255,0,0)", c, 1, 0, 0, 1)
}

func TestParseColor_RGBA(t *testing.T) {
	c, ok := ParseColor("rgba(0, 0, 255, 0.5)")
	if !ok {
		t.Fatal("ParseColor(\"rgba(0, 0, 255, 0.5)\") returned ok=false")
	}
	assertColor(t, "rgba semi-blue", c, 0, 0, 1, 0.5)
}

func TestParseColor_None(t *testing.T) {
	_, ok := ParseColor("none")
	if ok {
		t.Error("ParseColor(\"none\") should return ok=false")
	}
}

func TestParseColor_CurrentColor(t *testing.T) {
	// Implementation returns black (0,0,0,1) with ok=true for currentColor.
	c, ok := ParseColor("currentColor")
	if !ok {
		t.Fatal("ParseColor(\"currentColor\") returned ok=false")
	}
	assertColor(t, "currentColor", c, 0, 0, 0, 1)
}

func TestParseColor_Invalid(t *testing.T) {
	_, ok := ParseColor("notacolor")
	if ok {
		t.Error("ParseColor(\"notacolor\") should return ok=false")
	}
}

func TestParseColor_EmptyString(t *testing.T) {
	_, ok := ParseColor("")
	if ok {
		t.Error("ParseColor(\"\") should return ok=false")
	}
}

func TestParseColor_HexBlue(t *testing.T) {
	c, ok := ParseColor("#0000ff")
	if !ok {
		t.Fatal("ParseColor(\"#0000ff\") returned ok=false")
	}
	assertColor(t, "#0000ff", c, 0, 0, 1, 1)
}

// ---------------------------------------------------------------------------
// Transform parsing tests
// ---------------------------------------------------------------------------

func TestIdentityMatrix(t *testing.T) {
	m := Identity()
	assertMatrix(t, "Identity", m, 1, 0, 0, 1, 0, 0)
}

func TestParseTransform_Translate(t *testing.T) {
	m := ParseTransform("translate(10, 20)")
	assertMatrix(t, "translate(10,20)", m, 1, 0, 0, 1, 10, 20)
}

func TestParseTransform_Scale(t *testing.T) {
	m := ParseTransform("scale(2)")
	assertMatrix(t, "scale(2)", m, 2, 0, 0, 2, 0, 0)
}

func TestParseTransform_Rotate90(t *testing.T) {
	m := ParseTransform("rotate(90)")
	// cos(90)=0, sin(90)=1 => A=0, B=1, C=-1, D=0
	assertMatrix(t, "rotate(90)", m, 0, 1, -1, 0, 0, 0)
}

func TestParseTransform_Combined(t *testing.T) {
	m := ParseTransform("translate(10,20) scale(2)")
	// translate(10,20) * scale(2) = {A:2, B:0, C:0, D:2, E:10, F:20}
	assertMatrix(t, "translate+scale", m, 2, 0, 0, 2, 10, 20)
}

func TestParseTransform_Matrix(t *testing.T) {
	m := ParseTransform("matrix(1,0,0,1,50,100)")
	assertMatrix(t, "matrix literal", m, 1, 0, 0, 1, 50, 100)
}

func TestParseTransform_SkewX(t *testing.T) {
	m := ParseTransform("skewX(45)")
	// tan(45deg) = 1
	assertMatrix(t, "skewX(45)", m, 1, 0, 1, 1, 0, 0)
}

func TestParseTransform_SkewY(t *testing.T) {
	m := ParseTransform("skewY(45)")
	assertMatrix(t, "skewY(45)", m, 1, 1, 0, 1, 0, 0)
}

func TestParseTransform_Empty(t *testing.T) {
	m := ParseTransform("")
	assertMatrix(t, "empty transform", m, 1, 0, 0, 1, 0, 0)
}

func TestParseTransform_NonUniformScale(t *testing.T) {
	m := ParseTransform("scale(3, 0.5)")
	assertMatrix(t, "scale(3,0.5)", m, 3, 0, 0, 0.5, 0, 0)
}

func TestMultiply(t *testing.T) {
	a := Translate(5, 10)
	b := Scale(2, 3)
	m := a.Multiply(b)
	// translate * scale: A=2, D=3, E=5, F=10
	assertMatrix(t, "Multiply", m, 2, 0, 0, 3, 5, 10)
}

// ---------------------------------------------------------------------------
// Path parsing tests
// ---------------------------------------------------------------------------

func TestParsePath_SimpleML(t *testing.T) {
	cmds, err := ParsePathData("M 0 0 L 10 10")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	if cmds[0].Type != 'M' {
		t.Errorf("cmd[0].Type = %c, want M", cmds[0].Type)
	}
	if cmds[1].Type != 'L' {
		t.Errorf("cmd[1].Type = %c, want L", cmds[1].Type)
	}
	if !approxEqual(cmds[1].Args[0], 10) || !approxEqual(cmds[1].Args[1], 10) {
		t.Errorf("cmd[1].Args = %v, want [10 10]", cmds[1].Args)
	}
}

func TestParsePath_Relative(t *testing.T) {
	cmds, err := ParsePathData("M 0 0 l 10 10")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	// Relative l 10 10 from M 0 0 => absolute L 10 10
	if cmds[1].Type != 'L' {
		t.Errorf("cmd[1].Type = %c, want L", cmds[1].Type)
	}
	if !approxEqual(cmds[1].Args[0], 10) || !approxEqual(cmds[1].Args[1], 10) {
		t.Errorf("cmd[1].Args = %v, want [10 10]", cmds[1].Args)
	}
}

func TestParsePath_ImplicitLinetoAfterM(t *testing.T) {
	cmds, err := ParsePathData("M 0 0 10 10 20 20")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	// M 0 0 => M, then 10 10 => implicit L, then 20 20 => implicit L
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
	if cmds[0].Type != 'M' {
		t.Errorf("cmd[0].Type = %c, want M", cmds[0].Type)
	}
	if cmds[1].Type != 'L' {
		t.Errorf("cmd[1].Type = %c, want L (implicit lineto)", cmds[1].Type)
	}
	if cmds[2].Type != 'L' {
		t.Errorf("cmd[2].Type = %c, want L (implicit lineto)", cmds[2].Type)
	}
}

func TestParsePath_HV(t *testing.T) {
	cmds, err := ParsePathData("M 0 0 H 10 V 20")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	// H 10 => L 10 0; V 20 => L 10 20
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
	// H converted to L
	if cmds[1].Type != 'L' {
		t.Errorf("H should convert to L, got %c", cmds[1].Type)
	}
	if !approxEqual(cmds[1].Args[0], 10) || !approxEqual(cmds[1].Args[1], 0) {
		t.Errorf("H 10: got args %v, want [10 0]", cmds[1].Args)
	}
	// V converted to L
	if cmds[2].Type != 'L' {
		t.Errorf("V should convert to L, got %c", cmds[2].Type)
	}
	if !approxEqual(cmds[2].Args[0], 10) || !approxEqual(cmds[2].Args[1], 20) {
		t.Errorf("V 20: got args %v, want [10 20]", cmds[2].Args)
	}
}

func TestParsePath_ClosePath(t *testing.T) {
	cmds, err := ParsePathData("M 0 0 L 10 10 Z")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
	if cmds[2].Type != 'Z' {
		t.Errorf("cmd[2].Type = %c, want Z", cmds[2].Type)
	}
}

func TestParsePath_Cubic(t *testing.T) {
	cmds, err := ParsePathData("M 0 0 C 1 2 3 4 5 6")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	if cmds[1].Type != 'C' {
		t.Errorf("cmd[1].Type = %c, want C", cmds[1].Type)
	}
	expected := []float64{1, 2, 3, 4, 5, 6}
	for i, v := range expected {
		if !approxEqual(cmds[1].Args[i], v) {
			t.Errorf("C arg[%d] = %.4f, want %.4f", i, cmds[1].Args[i], v)
		}
	}
}

func TestParsePath_Quadratic(t *testing.T) {
	cmds, err := ParsePathData("M 0 0 Q 5 10 10 0")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	if cmds[1].Type != 'Q' {
		t.Errorf("cmd[1].Type = %c, want Q", cmds[1].Type)
	}
	if !approxEqual(cmds[1].Args[0], 5) || !approxEqual(cmds[1].Args[1], 10) ||
		!approxEqual(cmds[1].Args[2], 10) || !approxEqual(cmds[1].Args[3], 0) {
		t.Errorf("Q args = %v, want [5 10 10 0]", cmds[1].Args)
	}
}

func TestParsePath_NoSpaceNegative(t *testing.T) {
	cmds, err := ParsePathData("M10-20L30-40")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	if len(cmds) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(cmds))
	}
	if !approxEqual(cmds[0].Args[0], 10) || !approxEqual(cmds[0].Args[1], -20) {
		t.Errorf("M args = %v, want [10 -20]", cmds[0].Args)
	}
	if !approxEqual(cmds[1].Args[0], 30) || !approxEqual(cmds[1].Args[1], -40) {
		t.Errorf("L args = %v, want [30 -40]", cmds[1].Args)
	}
}

func TestParsePath_Arc(t *testing.T) {
	cmds, err := ParsePathData("M 0 0 A 25 25 0 0 1 50 0")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	if len(cmds) < 2 {
		t.Fatalf("expected at least 2 commands, got %d", len(cmds))
	}
	// First is M, then A
	if cmds[0].Type != 'M' {
		t.Errorf("cmd[0].Type = %c, want M", cmds[0].Type)
	}
	if cmds[1].Type != 'A' {
		t.Errorf("cmd[1].Type = %c, want A", cmds[1].Type)
	}
	// Arc endpoint should be 50, 0
	if !approxEqual(cmds[1].Args[5], 50) || !approxEqual(cmds[1].Args[6], 0) {
		t.Errorf("A endpoint = (%.4f, %.4f), want (50, 0)", cmds[1].Args[5], cmds[1].Args[6])
	}
}

func TestParsePath_SmoothCubic(t *testing.T) {
	cmds, err := ParsePathData("M 0 0 C 1 2 3 4 5 6 S 8 9 10 11")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	// M, C, then S converted to C
	if len(cmds) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(cmds))
	}
	if cmds[2].Type != 'C' {
		t.Errorf("S should be converted to C, got %c", cmds[2].Type)
	}
}

func TestParsePath_EmptyString(t *testing.T) {
	cmds, err := ParsePathData("")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands for empty path, got %d", len(cmds))
	}
}

// ---------------------------------------------------------------------------
// ArcToCubics tests
// ---------------------------------------------------------------------------

func TestArcToCubics_Basic(t *testing.T) {
	cubics := ArcToCubics(0, 0, 25, 25, 0, false, true, 50, 0)
	if len(cubics) == 0 {
		t.Fatal("ArcToCubics returned no commands")
	}
	for _, c := range cubics {
		if c.Type != 'C' {
			t.Errorf("ArcToCubics returned command type %c, want C", c.Type)
		}
	}
	// The last cubic endpoint should approximate (50, 0).
	last := cubics[len(cubics)-1]
	endX := last.Args[4]
	endY := last.Args[5]
	if !approxEqual(endX, 50) || !approxEqual(endY, 0) {
		t.Errorf("Arc endpoint = (%.4f, %.4f), want (50, 0)", endX, endY)
	}
}

func TestArcToCubics_SamePoint(t *testing.T) {
	cubics := ArcToCubics(10, 10, 25, 25, 0, false, true, 10, 10)
	if len(cubics) != 0 {
		t.Errorf("ArcToCubics with same start/end should return nil, got %d commands", len(cubics))
	}
}

func TestArcToCubics_ZeroRadius(t *testing.T) {
	cubics := ArcToCubics(0, 0, 0, 25, 0, false, true, 50, 0)
	if len(cubics) != 1 {
		t.Fatalf("ArcToCubics with zero rx should return 1 degenerate cubic, got %d", len(cubics))
	}
	if cubics[0].Type != 'C' {
		t.Errorf("degenerate arc type = %c, want C", cubics[0].Type)
	}
}

// ---------------------------------------------------------------------------
// SVG parsing tests
// ---------------------------------------------------------------------------

func TestParse_Minimal(t *testing.T) {
	s, err := Parse(`<svg width="100" height="100"></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !approxEqual(s.Width(), 100) {
		t.Errorf("Width = %.4f, want 100", s.Width())
	}
	if !approxEqual(s.Height(), 100) {
		t.Errorf("Height = %.4f, want 100", s.Height())
	}
}

func TestParse_ViewBox(t *testing.T) {
	s, err := Parse(`<svg viewBox="0 0 200 200"></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	vb := s.ViewBox()
	if !vb.Valid {
		t.Fatal("ViewBox should be valid")
	}
	if !approxEqual(vb.Width, 200) || !approxEqual(vb.Height, 200) {
		t.Errorf("ViewBox = %+v, want 0 0 200 200", vb)
	}
	// Width/Height should fall back to viewBox when not explicitly set.
	if !approxEqual(s.Width(), 200) {
		t.Errorf("Width (fallback) = %.4f, want 200", s.Width())
	}
}

func TestParse_WithShapes(t *testing.T) {
	s, err := Parse(`<svg width="100" height="100"><rect width="50" height="50"/><circle r="10"/></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	root := s.Root()
	if root == nil {
		t.Fatal("Root is nil")
	}
	if len(root.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(root.Children))
	}
	if root.Children[0].Tag != "rect" {
		t.Errorf("child[0].Tag = %q, want \"rect\"", root.Children[0].Tag)
	}
	if root.Children[1].Tag != "circle" {
		t.Errorf("child[1].Tag = %q, want \"circle\"", root.Children[1].Tag)
	}
}

func TestParse_TextContent(t *testing.T) {
	s, err := Parse(`<svg xmlns="http://www.w3.org/2000/svg"><text>Hello</text></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	root := s.Root()
	if len(root.Children) == 0 {
		t.Fatal("expected at least one child")
	}
	textNode := root.Children[0]
	if textNode.Tag != "text" {
		t.Errorf("child[0].Tag = %q, want \"text\"", textNode.Tag)
	}
	if textNode.Text != "Hello" {
		t.Errorf("text content = %q, want \"Hello\"", textNode.Text)
	}
}

func TestParse_NestedGroups(t *testing.T) {
	s, err := Parse(`<svg width="100" height="100"><g><rect width="10" height="10"/></g></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	root := s.Root()
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child (g), got %d", len(root.Children))
	}
	g := root.Children[0]
	if g.Tag != "g" {
		t.Errorf("child[0].Tag = %q, want \"g\"", g.Tag)
	}
	if len(g.Children) != 1 {
		t.Fatalf("expected 1 child in g, got %d", len(g.Children))
	}
	if g.Children[0].Tag != "rect" {
		t.Errorf("g.child[0].Tag = %q, want \"rect\"", g.Children[0].Tag)
	}
}

func TestParse_AspectRatio(t *testing.T) {
	s, err := Parse(`<svg width="200" height="100"></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if !approxEqual(s.AspectRatio(), 2.0) {
		t.Errorf("AspectRatio = %.4f, want 2.0", s.AspectRatio())
	}
}

func TestParseBytes(t *testing.T) {
	data := []byte(`<svg width="50" height="50"></svg>`)
	s, err := ParseBytes(data)
	if err != nil {
		t.Fatalf("ParseBytes error: %v", err)
	}
	if !approxEqual(s.Width(), 50) {
		t.Errorf("Width = %.4f, want 50", s.Width())
	}
}

// ---------------------------------------------------------------------------
// Style resolution tests
// ---------------------------------------------------------------------------

func TestDefaultStyle(t *testing.T) {
	s := DefaultStyle()
	// Default fill is nil (meaning black for shapes, applied at render time).
	if s.Fill != nil {
		t.Error("DefaultStyle().Fill should be nil")
	}
	if !approxEqual(s.FillOpacity, 1) {
		t.Errorf("FillOpacity = %.4f, want 1", s.FillOpacity)
	}
	if !approxEqual(s.StrokeWidth, 1) {
		t.Errorf("StrokeWidth = %.4f, want 1", s.StrokeWidth)
	}
	if !approxEqual(s.Opacity, 1) {
		t.Errorf("Opacity = %.4f, want 1", s.Opacity)
	}
}

func TestResolveStyle_FillAttribute(t *testing.T) {
	node := &Node{
		Tag:   "rect",
		Attrs: map[string]string{"fill": "red"},
	}
	parent := DefaultStyle()
	s := ResolveStyle(node, parent)
	if s.Fill == nil {
		t.Fatal("Fill should not be nil after fill=\"red\"")
	}
	assertColor(t, "resolved fill", *s.Fill, 1, 0, 0, 1)
}

func TestResolveStyle_StrokeInheritance(t *testing.T) {
	// Parent has a stroke set.
	parentStroke := Color{R: 0, G: 0, B: 1, A: 1}
	parent := DefaultStyle()
	parent.Stroke = &parentStroke

	// Child does not override stroke.
	child := &Node{
		Tag:   "rect",
		Attrs: map[string]string{},
	}
	s := ResolveStyle(child, parent)
	if s.Stroke == nil {
		t.Fatal("Child should inherit parent stroke")
	}
	assertColor(t, "inherited stroke", *s.Stroke, 0, 0, 1, 1)
}

func TestResolveStyle_InlineStyleOverrides(t *testing.T) {
	node := &Node{
		Tag:   "rect",
		Attrs: map[string]string{"fill": "blue", "style": "fill:green"},
	}
	parent := DefaultStyle()
	s := ResolveStyle(node, parent)
	if s.Fill == nil {
		t.Fatal("Fill should not be nil")
	}
	// Inline style has higher priority than presentation attribute.
	// green = (0, 0.502, 0, 1)
	if !approxEqual(s.Fill.G, 0.502) || !approxEqual(s.Fill.R, 0) {
		t.Errorf("Fill should be green from inline style, got R=%.4f G=%.4f B=%.4f",
			s.Fill.R, s.Fill.G, s.Fill.B)
	}
}

func TestResolveStyle_OpacityNotInherited(t *testing.T) {
	parent := DefaultStyle()
	parent.Opacity = 0.5

	child := &Node{
		Tag:   "rect",
		Attrs: map[string]string{},
	}
	s := ResolveStyle(child, parent)
	// Opacity is non-inherited; child should get default 1.0.
	if !approxEqual(s.Opacity, 1.0) {
		t.Errorf("Opacity should not be inherited, got %.4f, want 1.0", s.Opacity)
	}
}

func TestResolveStyle_FillNone(t *testing.T) {
	node := &Node{
		Tag:   "rect",
		Attrs: map[string]string{"fill": "none"},
	}
	parentFill := Color{R: 1, G: 0, B: 0, A: 1}
	parent := DefaultStyle()
	parent.Fill = &parentFill

	s := ResolveStyle(node, parent)
	if s.Fill != nil {
		t.Error("fill=\"none\" should result in nil Fill")
	}
}

// ---------------------------------------------------------------------------
// Rendering integration tests
// ---------------------------------------------------------------------------

func TestRender_Rect(t *testing.T) {
	s, err := Parse(`<svg width="100" height="100"><rect x="10" y="10" width="50" height="30" fill="red"/></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 100, 100)
	out := string(stream.Bytes())

	// Should contain rectangle operator "re" and fill operator "f"
	if !strings.Contains(out, "re") {
		t.Error("rect rendering should contain \"re\" operator")
	}
	if !strings.Contains(out, "f") {
		t.Error("rect rendering with fill should contain \"f\" operator")
	}
	// Should contain save/restore state
	if !strings.Contains(out, "q") {
		t.Error("rendering should contain \"q\" (save state)")
	}
	if !strings.Contains(out, "Q") {
		t.Error("rendering should contain \"Q\" (restore state)")
	}
}

func TestRender_Circle(t *testing.T) {
	s, err := Parse(`<svg width="100" height="100"><circle cx="50" cy="50" r="25" fill="blue"/></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 100, 100)
	out := string(stream.Bytes())

	// Circle is approximated with cubic Bezier curves ("c" operator)
	if !strings.Contains(out, " c\n") && !strings.Contains(out, " c") {
		t.Error("circle rendering should contain \"c\" (curveTo) operator")
	}
	// Should contain fill
	if !strings.Contains(out, "f") {
		t.Error("circle rendering with fill should contain \"f\" operator")
	}
	// Should contain closepath "h"
	if !strings.Contains(out, "h") {
		t.Error("circle rendering should contain \"h\" (closePath) operator")
	}
}

func TestRender_ViewBoxScaling(t *testing.T) {
	s, err := Parse(`<svg viewBox="0 0 200 200"><rect x="0" y="0" width="200" height="200" fill="green"/></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	// Draw into a 100x100 box; viewBox is 200x200, so scale is 0.5
	s.Draw(stream, 0, 0, 100, 100)
	out := string(stream.Bytes())

	// Should contain cm (concat matrix) operator for the scaling transform.
	if !strings.Contains(out, "cm") {
		t.Error("viewBox rendering should contain \"cm\" (concat matrix) for scaling")
	}
	// The scale factor 0.5 should appear in the output.
	if !strings.Contains(out, "0.5") {
		t.Error("viewBox scaling of 200->100 should produce 0.5 scale factor in cm operator")
	}
}

func TestRender_StrokeAndFill(t *testing.T) {
	s, err := Parse(`<svg width="100" height="100"><rect x="5" y="5" width="90" height="90" fill="red" stroke="blue" stroke-width="2"/></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 100, 100)
	out := string(stream.Bytes())

	// Should contain both fill and stroke colors
	if !strings.Contains(out, "rg") {
		t.Error("fill+stroke rendering should contain \"rg\" (set fill color)")
	}
	if !strings.Contains(out, "RG") {
		t.Error("fill+stroke rendering should contain \"RG\" (set stroke color)")
	}
	// Should contain B (fill and stroke)
	if !strings.Contains(out, "B") {
		t.Error("fill+stroke rendering should contain \"B\" (fill and stroke)")
	}
}

func TestRender_EmptySVG(t *testing.T) {
	s, err := Parse(`<svg width="100" height="100"></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 100, 100)
	out := string(stream.Bytes())

	// Should still have save/restore state wrapper.
	if !strings.Contains(out, "q") || !strings.Contains(out, "Q") {
		t.Error("even empty SVG rendering should have save/restore state")
	}
}

// ---------------------------------------------------------------------------
// Edge case tests
// ---------------------------------------------------------------------------

func TestParse_EmptySVG(t *testing.T) {
	s, err := Parse(`<svg></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Should have zero width/height and no children.
	if s.Width() != 0 {
		t.Errorf("Width = %.4f, want 0", s.Width())
	}
	if s.Height() != 0 {
		t.Errorf("Height = %.4f, want 0", s.Height())
	}
	// Rendering should not panic.
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 100, 100)
}

func TestRender_ZeroSizeViewBox(t *testing.T) {
	s, err := Parse(`<svg viewBox="0 0 0 0"></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// viewBox with zero dimensions should be marked invalid.
	vb := s.ViewBox()
	if vb.Valid {
		t.Error("viewBox with zero dimensions should be invalid")
	}
	// Rendering must not panic (no divide-by-zero).
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 100, 100)
}

func TestRender_ZeroTargetDimensions(t *testing.T) {
	s, err := Parse(`<svg viewBox="0 0 100 100"><rect width="50" height="50" fill="red"/></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Zero width target: must not panic.
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 0, 100)
	// Zero height target: must not panic.
	stream2 := content.NewStream()
	s.Draw(stream2, 0, 0, 100, 0)
}

func TestParse_WhitespaceTextNodes(t *testing.T) {
	s, err := Parse(`<svg width="100" height="100">
		<g>
			<rect width="10" height="10"/>
		</g>
	</svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Should parse normally, whitespace-only text nodes are ignored.
	root := s.Root()
	if root == nil {
		t.Fatal("Root is nil")
	}
	// Rendering should not panic.
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 100, 100)
}

func TestParsePath_EmptyDAttribute(t *testing.T) {
	cmds, err := ParsePathData("")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands for empty d, got %d", len(cmds))
	}
}

func TestParsePath_WhitespaceOnly(t *testing.T) {
	cmds, err := ParsePathData("   \t\n  ")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands for whitespace-only path, got %d", len(cmds))
	}
}

func TestParsePath_ZAloneNoPriorMoveto(t *testing.T) {
	// A single Z without a prior moveto should not panic.
	cmds, err := ParsePathData("Z")
	if err != nil {
		t.Fatalf("ParsePathData error: %v", err)
	}
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}
	if cmds[0].Type != 'Z' {
		t.Errorf("cmd[0].Type = %c, want Z", cmds[0].Type)
	}
}

func TestParsePath_InvalidCommandLetter(t *testing.T) {
	_, err := ParsePathData("M 0 0 X 10 10")
	if err == nil {
		t.Error("expected error for invalid command letter 'X', got nil")
	}
}

func TestParse_DeeplyNestedGroups(t *testing.T) {
	// Build SVG with 15 levels of nested <g> elements.
	var sb strings.Builder
	sb.WriteString(`<svg width="100" height="100">`)
	depth := 15
	for range depth {
		sb.WriteString(`<g>`)
	}
	sb.WriteString(`<rect width="10" height="10" fill="red"/>`)
	for range depth {
		sb.WriteString(`</g>`)
	}
	sb.WriteString(`</svg>`)

	s, err := Parse(sb.String())
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Rendering deeply nested groups must not panic.
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 100, 100)

	out := string(stream.Bytes())
	// Should contain the rect operator.
	if !strings.Contains(out, "re") {
		t.Error("deeply nested group rendering should contain \"re\" operator for the rect")
	}
}

func TestRender_VeryLargeCoordinates(t *testing.T) {
	s, err := Parse(`<svg viewBox="0 0 1e10 1e10"><rect width="1e10" height="1e10" fill="blue"/></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	// Must not panic with very large coordinates.
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 100, 100)
	out := string(stream.Bytes())
	if !strings.Contains(out, "re") {
		t.Error("large coordinate rendering should still contain \"re\" operator")
	}
}

func TestParse_NegativeDimensions(t *testing.T) {
	s, err := Parse(`<svg viewBox="0 0 -100 -100"></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	vb := s.ViewBox()
	if vb.Valid {
		t.Error("viewBox with negative dimensions should be invalid")
	}
	// Rendering must not panic.
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 100, 100)
}

func TestResolveStyle_UnrecognizedProperties(t *testing.T) {
	// Unrecognized CSS properties in style attribute should not cause panic.
	node := &Node{
		Tag:   "rect",
		Attrs: map[string]string{"style": "fill:red; unknown-prop:value; another-thing:123"},
	}
	parent := DefaultStyle()
	s := ResolveStyle(node, parent)
	// Fill should still be parsed correctly.
	if s.Fill == nil {
		t.Fatal("Fill should not be nil after style with unrecognized properties")
	}
	assertColor(t, "fill with unrecognized props", *s.Fill, 1, 0, 0, 1)
}

func TestParseColor_RGBOverflow(t *testing.T) {
	// rgb(999,999,999) — values above 255 should be clamped to 1.0.
	c, ok := ParseColor("rgb(999,999,999)")
	if !ok {
		t.Fatal("ParseColor(\"rgb(999,999,999)\") returned ok=false")
	}
	// Each component is 999/255 clamped to 1.0.
	assertColor(t, "rgb overflow", c, 1.0, 1.0, 1.0, 1.0)
}

func TestParseColor_RGBNegative(t *testing.T) {
	// rgb(-10,-10,-10) — negative values should be clamped to 0.
	c, ok := ParseColor("rgb(-10,-10,-10)")
	if !ok {
		t.Fatal("ParseColor(\"rgb(-10,-10,-10)\") returned ok=false")
	}
	assertColor(t, "rgb negative", c, 0, 0, 0, 1)
}

func TestRender_PathEmptyD(t *testing.T) {
	// Path with empty d attribute should render without error.
	s, err := Parse(`<svg width="100" height="100"><path d="" fill="black"/></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 100, 100)
	// Should not crash; output should still have save/restore.
	out := string(stream.Bytes())
	if !strings.Contains(out, "q") {
		t.Error("empty path SVG should still have save state")
	}
}

func TestRender_PathElement(t *testing.T) {
	s, err := Parse(`<svg width="100" height="100"><path d="M 10 10 L 90 10 L 90 90 Z" fill="black"/></svg>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	s.Draw(stream, 0, 0, 100, 100)
	out := string(stream.Bytes())

	// Should contain moveTo, lineTo, closePath
	if !strings.Contains(out, " m\n") && !strings.Contains(out, " m") {
		t.Error("path rendering should contain \"m\" (moveTo)")
	}
	if !strings.Contains(out, " l\n") && !strings.Contains(out, " l") {
		t.Error("path rendering should contain \"l\" (lineTo)")
	}
	if !strings.Contains(out, "h") {
		t.Error("path rendering with Z should contain \"h\" (closePath)")
	}
}
