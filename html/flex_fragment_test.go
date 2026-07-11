// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html_test

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
)

// TestFlexRowTallColumnFragmentsAcrossPages is the end-to-end regression for
// flex line fragmentation: a non-wrapping flex row whose column is taller
// than one page must fragment across pages. Before the fix the whole flex
// line was laid out on the first page and everything past the page bottom was
// clipped and silently dropped, so the document was a single page with
// missing content. After the fix the tall column continues onto further pages.
func TestFlexRowTallColumnFragmentsAcrossPages(t *testing.T) {
	var tall strings.Builder
	for i := 0; i < 120; i++ {
		tall.WriteString("<p>Body paragraph line inside the tall left column.</p>")
	}

	htmlStr := `<html><body>
		<div style="display:flex">
			<div style="width:48%">` + tall.String() + `</div>
			<div style="width:48%"><p>Short right column.</p></div>
		</div>
	</body></html>`

	_, r := htmlRoundtrip(t, htmlStr, document.PageSizeLetter)

	if got := r.PageCount(); got < 2 {
		t.Errorf("expected the tall flex column to span multiple pages, got %d page(s) — "+
			"content past page one was dropped instead of fragmented", got)
	}
}

// TestFlexRowFixedHeightBoxesFragment mirrors the real-world shape that first
// exposed the gap: a flex column of unsplittable fixed-height boxes (rather
// than reflowable paragraphs) whose stack is taller than one page. It guards
// both halves of the fix together — the Div must break between boxes and the
// flex row must continue the column onto the next page.
func TestFlexRowFixedHeightBoxesFragment(t *testing.T) {
	var boxes strings.Builder
	for i := 0; i < 30; i++ {
		boxes.WriteString(`<div style="height:60px">Box</div>`)
	}

	htmlStr := `<html><body>
		<div style="display:flex">
			<div style="flex:1">` + boxes.String() + `</div>
		</div>
	</body></html>`

	_, r := htmlRoundtrip(t, htmlStr, document.PageSizeLetter)

	if got := r.PageCount(); got < 2 {
		t.Errorf("expected a column of fixed-height boxes to span multiple pages, got %d page(s) — "+
			"the boxes past page one were clipped instead of continued", got)
	}
}
