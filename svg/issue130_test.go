// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/content"
)

// TestImageElementCallsRegisterImage verifies that an <image> element in an
// SVG triggers the RegisterImage callback with the href value and emits a Do
// operator for the returned XObject name.
func TestImageElementCallsRegisterImage(t *testing.T) {
	svgXML := `<svg viewBox="0 0 100 100">
		<image x="10" y="20" width="40" height="30" href="data:image/png;base64,AAA"/>
	</svg>`
	s, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	var gotHref string
	stream := content.NewStream()
	s.DrawWithOptions(stream, 0, 0, 100, 100, RenderOptions{
		RegisterImage: func(href string) (string, float64, float64) {
			gotHref = href
			return "Im1", 200, 150
		},
	})

	if gotHref != "data:image/png;base64,AAA" {
		t.Errorf("RegisterImage got href=%q, want data:image/png;base64,AAA", gotHref)
	}
	out := string(stream.Bytes())
	if !strings.Contains(out, "/Im1 Do") {
		t.Errorf("expected '/Im1 Do' in output, got:\n%s", out)
	}
}

// TestImageElementSkippedWhenRegisterImageNil verifies that <image> elements
// are silently skipped when no RegisterImage callback is provided.
func TestImageElementSkippedWhenRegisterImageNil(t *testing.T) {
	svgXML := `<svg viewBox="0 0 100 100">
		<image x="0" y="0" width="50" height="50" href="data:image/png;base64,AAA"/>
	</svg>`
	s, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	s.DrawWithOptions(stream, 0, 0, 100, 100, RenderOptions{})
	out := string(stream.Bytes())
	if strings.Contains(out, "Do") {
		t.Errorf("expected no Do operator when RegisterImage is nil, got:\n%s", out)
	}
}

// TestImageElementSkippedOnEmptyName verifies that returning an empty name
// from RegisterImage skips the element (used for decode failures).
func TestImageElementSkippedOnEmptyName(t *testing.T) {
	svgXML := `<svg viewBox="0 0 100 100">
		<image x="0" y="0" width="50" height="50" href="data:bad"/>
	</svg>`
	s, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	called := false
	s.DrawWithOptions(stream, 0, 0, 100, 100, RenderOptions{
		RegisterImage: func(href string) (string, float64, float64) {
			called = true
			return "", 0, 0
		},
	})
	if !called {
		t.Error("RegisterImage was not called")
	}
	out := string(stream.Bytes())
	if strings.Contains(out, "Do") {
		t.Errorf("expected no Do operator when RegisterImage returns empty name, got:\n%s", out)
	}
}

// TestImageElementXlinkHref verifies that xlink:href is accepted in addition
// to the unprefixed href attribute.
func TestImageElementXlinkHref(t *testing.T) {
	svgXML := `<svg viewBox="0 0 100 100" xmlns:xlink="http://www.w3.org/1999/xlink">
		<image x="0" y="0" width="50" height="50" xlink:href="data:image/png;base64,BBB"/>
	</svg>`
	s, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	var gotHref string
	stream := content.NewStream()
	s.DrawWithOptions(stream, 0, 0, 100, 100, RenderOptions{
		RegisterImage: func(href string) (string, float64, float64) {
			gotHref = href
			return "Im1", 10, 10
		},
	})
	if gotHref != "data:image/png;base64,BBB" {
		t.Errorf("xlink:href not honored, got %q", gotHref)
	}
	out := string(stream.Bytes())
	if !strings.Contains(out, "/Im1 Do") {
		t.Errorf("expected '/Im1 Do' in output for xlink:href, got:\n%s", out)
	}
}

// TestLinearGradientInvokesRegisterGradient verifies that a rect with a
// fill="url(#id)" reference to a linearGradient triggers the
// RegisterGradient callback and emits a Do operator for the returned
// XObject (rather than falling back to a solid fill).
func TestLinearGradientInvokesRegisterGradient(t *testing.T) {
	svgXML := `<svg viewBox="0 0 200 80">
		<defs>
			<linearGradient id="g1" x1="0" y1="0" x2="1" y2="0">
				<stop offset="0" stop-color="#0f172a"/>
				<stop offset="1" stop-color="#0d9488"/>
			</linearGradient>
		</defs>
		<rect width="200" height="80" fill="url(#g1)"/>
	</svg>`
	s, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	var gotNode *Node
	var gotBBox BBox
	s.DrawWithOptions(stream, 0, 0, 200, 80, RenderOptions{
		RegisterGradient: func(node *Node, bbox BBox) string {
			gotNode = node
			gotBBox = bbox
			return "Im1"
		},
	})
	if gotNode == nil {
		t.Fatal("RegisterGradient was not called")
	}
	if gotNode.Tag != "linearGradient" {
		t.Errorf("expected linearGradient, got %q", gotNode.Tag)
	}
	if gotBBox.W != 200 || gotBBox.H != 80 {
		t.Errorf("bbox mismatch: got %+v, want W=200 H=80", gotBBox)
	}
	info := gotNode.LinearGradient()
	if info == nil {
		t.Fatal("LinearGradient() returned nil")
	}
	if len(info.Stops) != 2 {
		t.Fatalf("expected 2 stops, got %d", len(info.Stops))
	}
	if info.Stops[0].Color.R != 0x0f/255.0 {
		t.Errorf("first stop R=%.3f, want %.3f", info.Stops[0].Color.R, 0x0f/255.0)
	}

	out := string(stream.Bytes())
	if !strings.Contains(out, "/Im1 Do") {
		t.Errorf("expected /Im1 Do in output, got:\n%s", out)
	}
	// The clip path should be present too ("W" followed by "n").
	if !strings.Contains(out, "W\nn") && !strings.Contains(out, "W\r\nn") && !strings.Contains(out, "W n") {
		t.Errorf("expected clip+no-op (W / n) before gradient draw, got:\n%s", out)
	}
}

// TestRadialGradientInvokesRegisterGradient mirrors the linearGradient
// test for radialGradient references.
func TestRadialGradientInvokesRegisterGradient(t *testing.T) {
	svgXML := `<svg viewBox="0 0 100 100">
		<defs>
			<radialGradient id="g2">
				<stop offset="0" stop-color="white"/>
				<stop offset="1" stop-color="black"/>
			</radialGradient>
		</defs>
		<circle cx="50" cy="50" r="40" fill="url(#g2)"/>
	</svg>`
	s, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	var gotTag string
	s.DrawWithOptions(stream, 0, 0, 100, 100, RenderOptions{
		RegisterGradient: func(node *Node, bbox BBox) string {
			gotTag = node.Tag
			return "Im1"
		},
	})
	if gotTag != "radialGradient" {
		t.Errorf("expected radialGradient, got %q", gotTag)
	}
	out := string(stream.Bytes())
	if !strings.Contains(out, "/Im1 Do") {
		t.Errorf("expected /Im1 Do, got:\n%s", out)
	}
}

// TestGradientFallbackToFirstStopWhenCallbackEmpty verifies that returning
// an empty string from RegisterGradient falls back to the first-stop color
// rather than leaving the shape unpainted.
func TestGradientFallbackToFirstStopWhenCallbackEmpty(t *testing.T) {
	svgXML := `<svg viewBox="0 0 10 10">
		<defs>
			<linearGradient id="g">
				<stop offset="0" stop-color="#ff0000"/>
				<stop offset="1" stop-color="#0000ff"/>
			</linearGradient>
		</defs>
		<rect width="10" height="10" fill="url(#g)"/>
	</svg>`
	s, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	s.DrawWithOptions(stream, 0, 0, 10, 10, RenderOptions{
		RegisterGradient: func(node *Node, bbox BBox) string { return "" },
	})
	out := string(stream.Bytes())
	// Should see a fill op (f or B) and the red color (1 0 0 rg).
	if !strings.Contains(out, "1 0 0 rg") {
		t.Errorf("expected first-stop red fallback (1 0 0 rg), got:\n%s", out)
	}
	if strings.Contains(out, "Do") {
		t.Errorf("did not expect Do operator in fallback path, got:\n%s", out)
	}
}

// TestGradientFallbackWhenNoCallback verifies legacy behavior — without a
// RegisterGradient callback, the renderer still collapses gradient fills
// to the first stop color so existing callers aren't broken.
func TestGradientFallbackWhenNoCallback(t *testing.T) {
	svgXML := `<svg viewBox="0 0 10 10">
		<defs>
			<linearGradient id="g">
				<stop offset="0" stop-color="#ff0000"/>
				<stop offset="1" stop-color="#0000ff"/>
			</linearGradient>
		</defs>
		<rect width="10" height="10" fill="url(#g)"/>
	</svg>`
	s, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	s.DrawWithOptions(stream, 0, 0, 10, 10, RenderOptions{})
	out := string(stream.Bytes())
	if !strings.Contains(out, "1 0 0 rg") {
		t.Errorf("expected first-stop red (legacy fallback), got:\n%s", out)
	}
}

// TestLinearGradientStopParsing exercises the Node.LinearGradient accessor
// directly to make sure stop order, offsets, and colors round-trip.
func TestLinearGradientStopParsing(t *testing.T) {
	svgXML := `<svg>
		<defs>
			<linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
				<stop offset="0%" stop-color="red"/>
				<stop offset="50%" stop-color="lime" stop-opacity="0.5"/>
				<stop offset="100%" stop-color="blue"/>
			</linearGradient>
		</defs>
	</svg>`
	s, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	root := s.Root()
	var gradient *Node
	var find func(n *Node)
	find = func(n *Node) {
		if gradient != nil {
			return
		}
		if n.Tag == "linearGradient" {
			gradient = n
			return
		}
		for _, c := range n.Children {
			find(c)
		}
	}
	find(root)
	if gradient == nil {
		t.Fatal("linearGradient node not found")
	}
	info := gradient.LinearGradient()
	if info == nil {
		t.Fatal("LinearGradient() returned nil")
	}
	if info.X1 != 0 || info.Y1 != 0 || info.X2 != 1 || info.Y2 != 1 {
		t.Errorf("endpoints: got (%v,%v)→(%v,%v), want (0,0)→(1,1)",
			info.X1, info.Y1, info.X2, info.Y2)
	}
	if len(info.Stops) != 3 {
		t.Fatalf("stops: got %d, want 3", len(info.Stops))
	}
	if info.Stops[0].Offset != 0 || info.Stops[1].Offset != 0.5 || info.Stops[2].Offset != 1 {
		t.Errorf("offsets: got %v %v %v, want 0 0.5 1",
			info.Stops[0].Offset, info.Stops[1].Offset, info.Stops[2].Offset)
	}
	if info.Stops[1].Color.A != 0.5 {
		t.Errorf("middle stop alpha: got %v, want 0.5", info.Stops[1].Color.A)
	}
}

// TestImageElementIntrinsicFallback verifies that missing width/height on
// <image> fall back to the intrinsic dimensions reported by RegisterImage.
func TestImageElementIntrinsicFallback(t *testing.T) {
	svgXML := `<svg viewBox="0 0 400 400">
		<image x="0" y="0" href="data:image/png;base64,CCC"/>
	</svg>`
	s, err := Parse(svgXML)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	stream := content.NewStream()
	s.DrawWithOptions(stream, 0, 0, 400, 400, RenderOptions{
		RegisterImage: func(href string) (string, float64, float64) {
			return "Im1", 123, 45
		},
	})
	out := string(stream.Bytes())
	// The CTM should use intrinsic dims: matrix = [123 0 0 -45 0 45]
	// We look for "123 0 0 -45" prefix to be resilient to the y+h component.
	if !strings.Contains(out, "123 0 0 -45") {
		t.Errorf("expected intrinsic dimensions in CTM, got:\n%s", out)
	}
}
