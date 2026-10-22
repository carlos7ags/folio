// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"fmt"
	"strings"
	"testing"
)

// TestFloatWithOversizedBlockPaginatesSiblings is the HTML-level regression
// test for a container holding a floated icon plus a block taller than a
// whole page. The float counted as progress, so the tall block was deferred
// with it, leaving the container empty (LayoutNothing); the renderer then
// force-placed the whole container at unbounded height on one page, drawing
// every block after the tall one off the bottom of the page. Without the
// float the same markup paginated correctly, so both must give the same
// page count with every block drawn exactly once, inside the page.
func TestFloatWithOversizedBlockPaginatesSiblings(t *testing.T) {
	const n = 20
	build := func(withFloat bool) string {
		var b strings.Builder
		// The background keeps the wrapper a real container (a bare <div>
		// is flattened into its parent's flow).
		b.WriteString(`<div style="background:#eee">`)
		if withFloat {
			fmt.Fprintf(&b, `<img style="float:left; width:20px" src="%s" alt="icon">`, iconDataURI(t))
		}
		b.WriteString(`<div style="height:1000pt"></div>`)
		for i := range n {
			fmt.Fprintf(&b, `<div style="height:100pt">Q%02dZ</div>`, i)
		}
		b.WriteString("</div>")
		return b.String()
	}

	// 785pt usable: the 1000pt block alone on page 1 (overflowing, as an
	// unsplittable box must), then 7 of the 100pt blocks per page.
	const wantPages = 4
	for _, withFloat := range []bool{false, true} {
		t.Run(fmt.Sprintf("float=%v", withFloat), func(t *testing.T) {
			_, pages := biRender(t, build(withFloat))
			if len(pages) != wantPages {
				t.Errorf("got %d pages, want %d", len(pages), wantPages)
			}
			for i := range n {
				label := fmt.Sprintf("Q%02dZ", i)
				count := 0
				for _, pg := range pages {
					count += strings.Count(string(pg.Stream.Bytes()), label)
				}
				if count != 1 {
					t.Errorf("%s drawn %d times, want 1", label, count)
				}
			}
			// The pages after the first hold only the 100pt blocks, which
			// must sit inside the margins.
			if len(pages) > 1 {
				biAssertNoOverflow(t, pages[1:])
			}
			if withFloat {
				imgs := 0
				for _, pg := range pages {
					imgs += countDoOps(string(pg.Stream.Bytes()))
				}
				if imgs != 1 {
					t.Errorf("icon drawn %d times, want 1", imgs)
				}
			}
		})
	}
}

// TestPaddingTallerThanPageNoPagePerChild is the HTML-level regression test
// for a container whose padding (or border) alone exceeds the usable page
// height: it must not fragment into one page per child paragraph.
func TestPaddingTallerThanPageNoPagePerChild(t *testing.T) {
	for _, box := range []string{"padding-top:900pt", "border-top:900pt solid #000"} {
		t.Run(box, func(t *testing.T) {
			var b strings.Builder
			fmt.Fprintf(&b, `<div style="%s;background:#eee">`, box)
			for i := range 40 {
				fmt.Fprintf(&b, `<p>P%02dZ</p>`, i)
			}
			b.WriteString("</div>")
			_, pages := biRender(t, b.String())
			if len(pages) > 3 {
				t.Errorf("got %d pages, want at most 3 (no page per child)", len(pages))
			}
			for i := range 40 {
				label := fmt.Sprintf("P%02dZ", i)
				count := 0
				for _, pg := range pages {
					count += strings.Count(string(pg.Stream.Bytes()), label)
				}
				if count != 1 {
					t.Errorf("%s drawn %d times, want 1", label, count)
				}
			}
		})
	}
}
