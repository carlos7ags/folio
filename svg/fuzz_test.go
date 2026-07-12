// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package svg

import "testing"

// FuzzPathData tests that the path-data parser never panics on arbitrary input.
func FuzzPathData(f *testing.F) {
	f.Add("M0 0 L10 10 Z")
	f.Add("M0 0 C 1 2 3 4 5 6 S 7 8 9 10")
	f.Add("m5,5 a2,2 0 1 0 4,0")
	f.Add("M0 0 A 5 5 0 1")
	f.Add("M 1e999 0")
	f.Add("-")
	f.Fuzz(func(t *testing.T, d string) {
		_, _ = parsePathData(d) // must not panic; errors are fine
	})
}

// FuzzSVGParse tests that SVG parsing and the read-only accessors never
// panic on arbitrary input. Mirrors reader.FuzzParsePDF: if parsing
// succeeds, basic operations must not panic either.
func FuzzSVGParse(f *testing.F) {
	f.Add(`<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100"><circle cx="50" cy="50" r="40"/></svg>`)
	f.Add(`<svg viewBox="0 0 10 10"><g transform="rotate(45"><path d="M0 0 L"/></g></svg>`)
	f.Add(`<svg><defs><linearGradient id="g"><stop offset="bad%"/></linearGradient></defs></svg>`)
	f.Add(`<svg`)
	f.Fuzz(func(t *testing.T, data string) {
		s, err := Parse(data)
		if err != nil || s == nil {
			return
		}
		_ = s.Width()
		_ = s.Height()
		_ = s.ViewBox()
		_ = s.AspectRatio()
		_ = s.PreserveAspectRatio()
		for _, n := range s.Defs() {
			_ = n.LinearGradient()
			_ = n.RadialGradient()
		}
	})
}
