// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/svg"
)

// TestSVGGradientStopOpacityReachesLayoutStops verifies that an SVG
// gradient `<stop stop-opacity="0.5">` propagates into the
// layout.GradientStop.Alpha field via parseSVGGradientStops. Regression
// test for the v0.6.2 documented gap where stop-opacity was silently
// dropped on the way from the svg package into the layout rasterizer.
func TestSVGGradientStopOpacityReachesLayoutStops(t *testing.T) {
	svgXML := `<svg>
		<defs>
			<linearGradient id="g">
				<stop offset="0" stop-color="red"/>
				<stop offset="1" stop-color="blue" stop-opacity="0.5"/>
			</linearGradient>
		</defs>
	</svg>`
	s, err := svg.Parse(svgXML)
	if err != nil {
		t.Fatalf("svg.Parse: %v", err)
	}
	var gradient *svg.Node
	var find func(n *svg.Node)
	find = func(n *svg.Node) {
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
	find(s.Root())
	if gradient == nil {
		t.Fatal("linearGradient not found")
	}

	stops := parseSVGGradientStops(gradient)
	if len(stops) != 2 {
		t.Fatalf("stops: got %d, want 2", len(stops))
	}

	// First stop: opaque (no stop-opacity). Alpha should be 0 sentinel.
	if stops[0].Alpha != 0 {
		t.Errorf("first stop Alpha: got %v, want 0 (opaque sentinel)", stops[0].Alpha)
	}

	// Second stop: stop-opacity="0.5". Alpha should be 0.5.
	if stops[1].Alpha < 0.49 || stops[1].Alpha > 0.51 {
		t.Errorf("second stop Alpha: got %v, want ~0.5", stops[1].Alpha)
	}
}

// TestSVGGradientFullyTransparentStopMapsToEpsilon verifies that
// stop-opacity="0" becomes a near-zero alpha (1/255) rather than the
// opaque sentinel — so a fade-to-transparent gradient still shows the
// transparency at the endpoint instead of being collapsed to opaque.
func TestSVGGradientFullyTransparentStopMapsToEpsilon(t *testing.T) {
	svgXML := `<svg>
		<defs>
			<linearGradient id="g">
				<stop offset="0" stop-color="red"/>
				<stop offset="1" stop-color="red" stop-opacity="0"/>
			</linearGradient>
		</defs>
	</svg>`
	s, err := svg.Parse(svgXML)
	if err != nil {
		t.Fatalf("svg.Parse: %v", err)
	}
	var gradient *svg.Node
	var find func(n *svg.Node)
	find = func(n *svg.Node) {
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
	find(s.Root())

	stops := parseSVGGradientStops(gradient)
	if len(stops) != 2 {
		t.Fatalf("stops: got %d, want 2", len(stops))
	}
	if stops[1].Alpha == 0 {
		t.Errorf("second stop alpha was 0 (treated as opaque); expected near-zero epsilon")
	}
	if stops[1].Alpha >= 0.01 {
		t.Errorf("second stop alpha %v too high — expected near-zero (1/255)", stops[1].Alpha)
	}
}
