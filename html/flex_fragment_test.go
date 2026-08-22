// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html_test

import (
	"fmt"
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

// rowWrapperDoc builds a document whose content is pushed down by fillPt
// points of empty space and then continues with `rows` row wrappers. Each
// wrapper holds a short fixed-width marker plus a column of stacked tables —
// the shape a report generator uses for a numbered or gutter row. wrap decides
// whether the wrapper is a flex row or a plain block. Every cell carries a
// unique token so the extracted text can be checked for completeness.
func rowWrapperDoc(fillPt float64, rows int, wrap string) (htmlStr string, tokens []string) {
	var b strings.Builder
	b.WriteString("<html><body>")
	// px -> pt is a 0.75 scale in folio; the filler carries no text of its own.
	fmt.Fprintf(&b, `<div style="height:%.0fpx"></div>`, fillPt/0.75)
	for r := 0; r < rows; r++ {
		switch wrap {
		case "flex":
			fmt.Fprintf(&b, `<div style="display:flex;align-items:flex-start">`+
				`<div style="width:60px">M%02d</div><div style="flex:1">`, r)
		default:
			fmt.Fprintf(&b, `<div><div style="width:60px">M%02d</div><div>`, r)
		}
		tokens = append(tokens, fmt.Sprintf("M%02d", r))
		for tb := 0; tb < 2; tb++ {
			b.WriteString("<table>")
			for i := 0; i < 4; i++ {
				tok := fmt.Sprintf("R%02dT%dC%d", r, tb, i)
				tokens = append(tokens, tok)
				fmt.Fprintf(&b, "<tr><td>%s</td></tr>", tok)
			}
			b.WriteString("</table>")
		}
		b.WriteString("</div></div>")
	}
	b.WriteString("</body></html>")
	return b.String(), tokens
}

// missingTokens renders one document and reports which unique tokens never
// reached the PDF text — content loss measured as content, not as page count.
func missingTokens(t *testing.T, htmlStr string, tokens []string) []string {
	t.Helper()
	_, r := htmlRoundtrip(t, htmlStr, document.PageSizeLetter)
	text := normalizeWhitespace(extractAllText(t, r))
	var missing []string
	for _, tok := range tokens {
		if !strings.Contains(text, tok) {
			missing = append(missing, tok)
		}
	}
	return missing
}

// sweepRowWrapper renders the row-wrapper document at a range of fill heights,
// so the page boundary lands at every offset within the row wrapper. The window
// that loses content is only a few points wide — wide enough for the short
// marker, too narrow for the column's first table row — so a single fixed
// offset would sail past the bug. Returns the worst offset and its loss.
func sweepRowWrapper(t *testing.T, wrap string) (worstFill float64, worstMissing, total int) {
	t.Helper()
	for fill := 600.0; fill <= 780.0; fill += 2 {
		htmlStr, tokens := rowWrapperDoc(fill, 3, wrap)
		total = len(tokens)
		if n := len(missingTokens(t, htmlStr, tokens)); n > worstMissing {
			worstFill, worstMissing = fill, n
		}
	}
	return
}

// TestFlexRowWrapperStraddlingPageBoundaryKeepsAllContent is the end-to-end
// regression for the content-loss half of flex fragmentation. A flex row used
// as a row wrapper — short fixed-width marker plus a tall block column — that
// straddles a page boundary used to be laid out with the column silently
// dropped whenever the column could not place even its first table row in the
// space left. The result was a well-formed, complete-looking, SHORTER document
// with the tail content missing, which is why this asserts on the text that
// survives and not on the page count.
func TestFlexRowWrapperStraddlingPageBoundaryKeepsAllContent(t *testing.T) {
	fill, missing, total := sweepRowWrapper(t, "flex")
	if missing > 0 {
		t.Errorf("content loss: %d of %d tokens never made it into the PDF (worst at %.0fpt of leading fill)",
			missing, total, fill)
	}
}

// TestBlockRowWrapperStraddlingPageBoundaryKeepsAllContent is the passing
// control: the same content in a plain block wrapper, which fragments
// correctly at every offset. It pins the boundary of the bug — the loss came
// from the flex row path, not from the tables or the column itself.
func TestBlockRowWrapperStraddlingPageBoundaryKeepsAllContent(t *testing.T) {
	fill, missing, total := sweepRowWrapper(t, "block")
	if missing > 0 {
		t.Errorf("control case lost content: %d of %d tokens missing (worst at %.0fpt of leading fill) — "+
			"the plain block wrapper must fragment without dropping anything",
			missing, total, fill)
	}
}

// TestFlexRowWrapperContentMatchesBlockWrapper compares the two wrappers
// directly at the same offsets: converting a block row wrapper into a flex row
// must not change how much content reaches the PDF. On the document that
// exposed this, the flex version lost roughly a tenth of the extractable text
// while also reporting FEWER pages than the block version — which is why page
// count is not the signal being compared here.
func TestFlexRowWrapperContentMatchesBlockWrapper(t *testing.T) {
	for fill := 600.0; fill <= 780.0; fill += 6 {
		flexHTML, tokens := rowWrapperDoc(fill, 3, "flex")
		blockHTML, _ := rowWrapperDoc(fill, 3, "block")
		flexMissing := len(missingTokens(t, flexHTML, tokens))
		blockMissing := len(missingTokens(t, blockHTML, tokens))
		if flexMissing != blockMissing {
			t.Errorf("at %.0fpt of leading fill the flex row wrapper is missing %d of %d tokens and the block wrapper %d — "+
				"wrapping a column in a flex row must not drop content", fill, flexMissing, len(tokens), blockMissing)
		}
	}
}
