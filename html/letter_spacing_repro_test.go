// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// tdXPositions extracts the x coordinate of every "x y Td" operator in a
// content stream, in order of appearance.
func tdXPositions(t *testing.T, stream string) []float64 {
	t.Helper()
	re := regexp.MustCompile(`(?m)^([\d.+-]+) [\d.+-]+ Td$`)
	var xs []float64
	for _, m := range re.FindAllStringSubmatch(stream, -1) {
		x, err := strconv.ParseFloat(m[1], 64)
		if err != nil {
			t.Fatalf("bad Td x %q: %v", m[1], err)
		}
		xs = append(xs, x)
	}
	return xs
}

// Regression test for issue #429: CSS letter-spacing must affect both the
// drawn output (Tc character-spacing operator) and word placement (the
// measured word widths and inter-word gap used by line layout).
func TestLetterSpacingAffectsOutput(t *testing.T) {
	render := func(src string) string {
		elems, err := Convert(src, nil)
		if err != nil {
			t.Fatal(err)
		}
		return renderStreamText(t, elems)
	}

	plain := render(`<div>Hello World</div>`)
	spaced := render(`<div style="letter-spacing: 10px;">Hello World</div>`)

	// 10px = 7.5pt must be emitted as PDF character spacing.
	if !strings.Contains(spaced, "7.5 Tc") {
		t.Fatalf("spaced stream missing '7.5 Tc':\n%s", spaced)
	}
	if strings.Contains(plain, "Tc") {
		t.Fatalf("plain stream unexpectedly sets character spacing:\n%s", plain)
	}

	// Word placement must use the spaced widths: the gap from "Hello" to
	// "World" grows by 4 intra-word gaps ("H-e", "e-l", "l-l", "l-o") plus
	// the 2 boundaries around the space = 6 * 7.5pt = 45pt.
	px, sx := tdXPositions(t, plain), tdXPositions(t, spaced)
	if len(px) < 2 || len(sx) < 2 {
		t.Fatalf("expected two positioned words, got %d plain / %d spaced", len(px), len(sx))
	}
	plainGap := px[1] - px[0]
	spacedGap := sx[1] - sx[0]
	const want = 6 * 7.5
	if got := spacedGap - plainGap; got < want-0.01 || got > want+0.01 {
		t.Fatalf("advance Hello->World grew by %.3f pt, want %.1f (plain %.3f, spaced %.3f)",
			got, float64(want), plainGap, spacedGap)
	}
}

// Line breaking must use letter-spaced word widths: at a width that fits
// "aa bb" only without spacing, adding letter-spacing must wrap onto two
// lines (two distinct baseline y positions).
func TestLetterSpacingAffectsLineBreaking(t *testing.T) {
	countLines := func(src string) int {
		elems, err := Convert(src, nil)
		if err != nil {
			t.Fatal(err)
		}
		stream := renderStreamText(t, elems)
		re := regexp.MustCompile(`(?m)^[\d.+-]+ ([\d.+-]+) Td$`)
		ys := map[string]bool{}
		for _, m := range re.FindAllStringSubmatch(stream, -1) {
			ys[m[1]] = true
		}
		return len(ys)
	}

	if n := countLines(`<div style="width: 40px">aa bb</div>`); n != 1 {
		t.Fatalf("plain text should fit one line, got %d", n)
	}
	if n := countLines(`<div style="width: 40px; letter-spacing: 8px">aa bb</div>`); n != 2 {
		t.Fatalf("letter-spaced text should wrap to two lines, got %d", n)
	}
}

// List item text inherits letter-spacing (previously dropped by
// collectListItemRuns building bare TextRuns).
func TestLetterSpacingListItems(t *testing.T) {
	elems, err := Convert(`<ul style="letter-spacing: 4px"><li>Hello</li></ul>`, nil)
	if err != nil {
		t.Fatal(err)
	}
	stream := renderStreamText(t, elems)
	if !strings.Contains(stream, "3 Tc") { // 4px = 3pt
		t.Fatalf("list item stream missing '3 Tc':\n%s", stream)
	}
}
