// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package csscolor

import (
	"math"
	"testing"
)

const eps = 1e-9

func approxEq(a, b float64) bool {
	return math.Abs(a-b) < eps
}

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Color
		ok    bool
	}{
		// Named colors, full sRGB precision (x/255, not a decimal literal).
		{"aliceblue", "aliceblue", Color{R: 240.0 / 255, G: 248.0 / 255, B: 255.0 / 255, A: 1}, true},
		{"crimson", "crimson", Color{R: 220.0 / 255, G: 20.0 / 255, B: 60.0 / 255, A: 1}, true},
		{"blueviolet", "blueviolet", Color{R: 138.0 / 255, G: 43.0 / 255, B: 226.0 / 255, A: 1}, true},
		{"unknown named color", "notacolor", Color{}, false},

		// Hex: 3, 4, 6, 8 digit.
		{"hex-3", "#f00", Color{R: 1, G: 0, B: 0, A: 1}, true},
		{"hex-4", "#f008", Color{R: 1, G: 0, B: 0, A: float64(0x88) / 255}, true},
		{"hex-6", "#ff0000", Color{R: 1, G: 0, B: 0, A: 1}, true},
		{"hex-8", "#ff000080", Color{R: 1, G: 0, B: 0, A: float64(0x80) / 255}, true},
		{"hex-invalid-length", "#12345", Color{}, false},

		// rgb()/rgba(): comma form.
		{"rgb-comma", "rgb(255, 0, 0)", Color{R: 1, G: 0, B: 0, A: 1}, true},
		{"rgba-comma", "rgba(0, 0, 255, 0.5)", Color{R: 0, G: 0, B: 1, A: 0.5}, true},
		{"rgb-comma-percent", "rgb(100%, 0%, 0%)", Color{R: 1, G: 0, B: 0, A: 1}, true},
		{"rgb-comma-too-few-args", "rgb(1,2)", Color{}, false},

		// rgb()/rgba(): CSS Color 4 space-separated form.
		{"rgb-space", "rgb(255 0 0)", Color{R: 1, G: 0, B: 0, A: 1}, true},
		{"rgb-space-alpha", "rgb(255 0 0 / 0.5)", Color{R: 1, G: 0, B: 0, A: 0.5}, true},
		{"rgb-space-percent-alpha", "rgb(100% 0% 0% / 50%)", Color{R: 1, G: 0, B: 0, A: 0.5}, true},

		// hsl()/hsla(): comma form.
		{"hsl-comma-red", "hsl(0, 100%, 50%)", Color{R: 1, G: 0, B: 0, A: 1}, true},
		{"hsla-comma", "hsla(0, 100%, 50%, 0.3)", Color{R: 1, G: 0, B: 0, A: 0.3}, true},

		// hsl()/hsla(): space form.
		{"hsl-space", "hsl(0 100% 50%)", Color{R: 1, G: 0, B: 0, A: 1}, true},
		{"hsl-space-alpha", "hsl(0 100% 50% / 0.3)", Color{R: 1, G: 0, B: 0, A: 0.3}, true},

		// cmyk()/device-cmyk().
		{"cmyk", "cmyk(0, 1, 1, 0)", Color{A: 1, CMYK: &[4]float64{0, 1, 1, 0}}, true},
		{"device-cmyk", "device-cmyk(0, 0, 0, 1)", Color{A: 1, CMYK: &[4]float64{0, 0, 0, 1}}, true},

		// Component clamping.
		{"rgb-component-over", "rgb(300, -50, 128)", Color{R: 1, G: 0, B: 128.0 / 255, A: 1}, true},
		{"rgb-percent-over", "rgb(150%, -10%, 50%)", Color{R: 1, G: 0, B: 0.5, A: 1}, true},

		// Invalid input.
		{"empty", "", Color{}, false},
		{"garbage", "notacolor", Color{}, false},

		// Hex garbage: non-hex digits must fail, not silently become 0.
		{"hex-zzz", "#zzz", Color{}, false},
		{"hex-gg0000", "#gg0000", Color{}, false},
		{"hex-12345g", "#12345g", Color{}, false},
		{"hex-12g45678", "#12g45678", Color{}, false},

		// Malformed components.
		{"rgb-comma-garbage", "rgb(foo, 0, 0)", Color{}, false},
		{"rgb-space-garbage", "rgb(25x 0 0)", Color{}, false},
		{"hsl-comma-garbage-hue", "hsl(abc, 50%, 50%)", Color{}, false},
		{"hsl-comma-garbage-sat", "hsl(120, x%, 50%)", Color{}, false},
		{"cmyk-garbage", "cmyk(a, 0, 0, 0)", Color{}, false},

		// Non-finite (NaN/Inf) must fail closed, not render as black.
		{"rgb-comma-nan", "rgb(nan, 0, 0)", Color{}, false},
		{"rgb-space-inf", "rgb(0 0 inf)", Color{}, false},
		{"hsl-comma-nan-hue", "hsl(nan, 100%, 50%)", Color{}, false},
		{"rgba-comma-nan-alpha", "rgba(0, 0, 0, nan)", Color{}, false},
		{"rgb-space-inf-alpha", "rgb(0 0 0 / inf)", Color{}, false},
		{"cmyk-inf", "cmyk(inf, 0, 0, 0)", Color{}, false},
		{"device-cmyk-nan", "device-cmyk(0, nan, 0, 0)", Color{}, false},

		// Positive controls: valid input must remain unaffected.
		{"hex-3-abc", "#abc", Color{R: float64(0xaa) / 255, G: float64(0xbb) / 255, B: float64(0xcc) / 255, A: 1}, true},
		{"hex-6-AABBCC", "#AABBCC", Color{R: float64(0xaa) / 255, G: float64(0xbb) / 255, B: float64(0xcc) / 255, A: 1}, true},
		{"rgb-255-0-0", "rgb(255, 0, 0)", Color{R: 1, G: 0, B: 0, A: 1}, true},
		{"rgb-space-percent-alpha-half", "rgb(100% 0% 50% / 0.5)", Color{R: 1, G: 0, B: 0.5, A: 0.5}, true},
		{"hsl-120-100-50", "hsl(120, 100%, 50%)", Color{R: 0, G: 1, B: 0, A: 1}, true},
		{"cmyk-black", "cmyk(0, 0, 0, 1)", Color{A: 1, CMYK: &[4]float64{0, 0, 0, 1}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Parse(tt.input)
			if ok != tt.ok {
				t.Fatalf("Parse(%q) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if !tt.ok {
				return
			}
			if !approxEq(got.R, tt.want.R) || !approxEq(got.G, tt.want.G) || !approxEq(got.B, tt.want.B) || !approxEq(got.A, tt.want.A) {
				t.Errorf("Parse(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
			if (tt.want.CMYK == nil) != (got.CMYK == nil) {
				t.Fatalf("Parse(%q) CMYK presence = %v, want %v", tt.input, got.CMYK != nil, tt.want.CMYK != nil)
			}
			if tt.want.CMYK != nil {
				for i := range tt.want.CMYK {
					if !approxEq(got.CMYK[i], tt.want.CMYK[i]) {
						t.Errorf("Parse(%q) CMYK[%d] = %v, want %v", tt.input, i, got.CMYK[i], tt.want.CMYK[i])
					}
				}
			}
		})
	}
}

func TestLookupAllNames(t *testing.T) {
	all := AllNames()
	if len(all) != 148 {
		t.Fatalf("AllNames() returned %d entries, want 148", len(all))
	}
	for _, name := range all {
		if _, _, _, ok := Lookup(name); !ok {
			t.Errorf("Lookup(%q) failed for a name returned by AllNames()", name)
		}
	}
}

func TestLookupUnknown(t *testing.T) {
	if _, _, _, ok := Lookup("notacolor"); ok {
		t.Error("Lookup(\"notacolor\") should return ok=false")
	}
}
