// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// iconDataURI returns a small valid PNG as a data URI. (onePxPNG in
// img_max_min_test.go fails to decode — its zlib checksum is wrong — so an
// <img> using it renders as alt text, which cannot test image placement.)
func iconDataURI(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := range 4 {
		for x := range 4 {
			img.Set(x, y, color.RGBA{G: 160, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

// countDoOps counts XObject draw operators (" Do") in a page content stream.
// PageResult.Images de-duplicates resources per page, so it undercounts when
// the same image is drawn twice; the operator count is what a viewer paints.
func countDoOps(stream string) int {
	n := 0
	for _, line := range strings.Split(stream, "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), " Do") {
			n++
		}
	}
	return n
}

// floatRecordHTML is the reported markup reduced to its essentials: a block
// whose first child is a floated icon, followed by a flex header and a table.
// The header has a fixed height (28px = 21pt) so the break thresholds are
// deterministic; the square icon resolves to 15x15pt. spacerPx pushes the record
// toward the page bottom (0 = no spacer).
func floatRecordHTML(icon string, spacerPx float64) string {
	spacer := ""
	if spacerPx > 0 {
		spacer = fmt.Sprintf(`<div style="height:%.0fpx"></div>`, spacerPx)
	}
	return fmt.Sprintf(`%s
<div style="padding-left:12px">
  <img style="float:left; width:20px" src="%s" alt="icon">
  <div style="display:flex; height:28px"><span>Header</span></div>
  <table><tr><td>Row one</td></tr><tr><td>Row two</td></tr></table>
</div>`, spacer, icon)
}

// headerX returns the x of the text-positioning operator that draws "Header"
// on a page, and whether it was found.
func headerX(stream string) (float64, bool) {
	lines := strings.Split(stream, "\n")
	lastX, haveX := 0.0, false
	for _, line := range lines {
		f := strings.Fields(line)
		if len(f) >= 3 && f[len(f)-1] == "Td" {
			if _, err := fmt.Sscanf(f[0], "%g", &lastX); err == nil {
				haveX = true
			}
		}
		if haveX && strings.Contains(line, "Header") {
			return lastX, true
		}
	}
	return 0, false
}

// TestFloatAtPageBreakIsNotDropped is the regression test for a floated icon
// lost when its block moves to the next page. With less room left than the
// icon's height, the float was skipped by the container (never placed, never
// carried to the overflow) so it was drawn on neither page, and the header on
// the next page lost the float's offset and shifted left.
func TestFloatAtPageBreakIsNotDropped(t *testing.T) {
	for _, tc := range []struct {
		name string
		left float64 // pt of room left at the page bottom
	}{
		{"less room than the icon", 10},
		{"room for the icon but not the header", 18},
	} {
		t.Run(tc.name, func(t *testing.T) {
			icon := iconDataURI(t)
			_, control := biRender(t, floatRecordHTML(icon, 0))
			wantX, ok := headerX(string(control[0].Stream.Bytes()))
			if !ok {
				t.Fatal("control: header not found")
			}
			if n := countDoOps(string(control[0].Stream.Bytes())); n != 1 {
				t.Fatalf("control draws %d images, want 1 (icon must load)", n)
			}

			_, pages := biRender(t, floatRecordHTML(icon, (biUsable-tc.left)/0.75))
			if len(pages) != 2 {
				t.Fatalf("got %d pages, want 2", len(pages))
			}
			biAssertNoOverflow(t, pages)

			p1, p2 := string(pages[0].Stream.Bytes()), string(pages[1].Stream.Bytes())
			if n := countDoOps(p1); n != 0 {
				t.Errorf("page 1 draws %d images, want 0 (the icon belongs with its header on page 2)", n)
			}
			if n := countDoOps(p2); n != 1 {
				t.Errorf("page 2 draws %d images, want 1 (the floated icon)", n)
			}
			if _, onP1 := headerX(p1); onP1 {
				t.Error("header drawn on page 1, want it on page 2")
			}
			gotX, ok := headerX(p2)
			if !ok {
				t.Fatal("header not found on page 2")
			}
			if diff := gotX - wantX; diff > 0.01 || diff < -0.01 {
				t.Errorf("header x on page 2 = %.2f, want %.2f (same offset beside the float as an unbroken record)", gotX, wantX)
			}
		})
	}
}
