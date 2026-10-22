// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"math"
	"testing"
)

// TestNormalLineHeightMetricSelection pins CSS line-height:normal resolution
// against the synthetic_metrics_*.ttf fixtures (see
// testdata/build_metrics_fixture.go), which share hhea 1050/-350/100 and OS/2
// typo 900/-250/150 at upem 1000 and differ only in the OS/2
// USE_TYPO_METRICS bit. The hhea metrics must be used unless the bit is set:
//
//	hhea: (1050 + 350 + 100) / 1000 = 1.5em
//	typo: ( 900 + 250 + 150) / 1000 = 1.3em
//
// usWinAscent/usWinDescent (1200/400) must never be consulted.
func TestNormalLineHeightMetricSelection(t *testing.T) {
	tests := []struct {
		file                     string
		ascent, descent, lineGap int
		wantEm                   float64
	}{
		{"testdata/synthetic_metrics_hhea.ttf", 1050, -350, 100, 1.5},
		{"testdata/synthetic_metrics_typo.ttf", 900, -250, 150, 1.3},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			face, err := LoadFont(tc.file)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			if got := face.Ascent(); got != tc.ascent {
				t.Errorf("Ascent = %d, want %d", got, tc.ascent)
			}
			if got := face.Descent(); got != tc.descent {
				t.Errorf("Descent = %d, want %d", got, tc.descent)
			}
			lg, ok := face.(lineGapper)
			if !ok {
				t.Fatalf("%T does not expose LineGap", face)
			}
			if got := lg.LineGap(); got != tc.lineGap {
				t.Errorf("LineGap = %d, want %d", got, tc.lineGap)
			}

			ef := NewEmbeddedFont(face)
			for _, size := range []float64{10, 11, 22} {
				want := tc.wantEm * size
				if got := ef.NormalLineHeight(size); math.Abs(got-want) > 1e-9 {
					t.Errorf("NormalLineHeight(%v) = %v, want %v", size, got, want)
				}
			}
		})
	}
}
