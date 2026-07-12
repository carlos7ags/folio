// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"math"
	"testing"

	"github.com/carlos7ags/folio/layout"
)

// cellFontSize returns the resolved font-size (in points) of a table cell's
// text, assuming the cell holds a single plain-text run (as `addCSSTableCell`
// and `convertTableRowKind` both produce for a `<td>text</td>`/`<th>text</th>`
// with no child elements).
func cellFontSize(t *testing.T, cell *layout.Cell) float64 {
	t.Helper()
	p, ok := cell.Content().(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected cell content to be *layout.Paragraph, got %T", cell.Content())
	}
	runs := p.Runs()
	if len(runs) == 0 {
		t.Fatal("expected at least one text run in cell")
	}
	return runs[0].FontSize
}

// TestTbodyFontSizeCascadesToCells is the regression test for a bug where a
// `tbody { font-size: ... }` (or `thead`/`tfoot`) selector's declarations
// never reached descendant `td`/`th` text: convertTable passed the <table>
// element's own style straight through to <thead>/<tbody>/<tfoot> children
// instead of first computing each section element's own style (which is
// where a rule targeting `tbody`/`thead`/`tfoot` is matched and cascaded),
// so any CSS scoped to those selectors was silently dropped and cells fell
// back to the inherited/page-default font-size.
func TestTbodyFontSizeCascadesToCells(t *testing.T) {
	src := `<html><head><style>
		body { font-size: 16px; }
		thead { font-size: 14px; }
		tbody { font-size: 13px; }
	</style></head><body>
		<table>
			<thead><tr><th>Head</th></tr></thead>
			<tbody><tr><td>ACTIVA</td></tr></tbody>
		</table>
	</body></html>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	tbl, ok := elems[0].(*layout.Table)
	if !ok {
		t.Fatalf("expected *layout.Table, got %T", elems[0])
	}
	rows := tbl.Rows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (header + body), got %d", len(rows))
	}

	// thead: 14px -> 10.5pt.
	headSize := cellFontSize(t, rows[0].Cells()[0])
	wantHeadSize := 14.0 * 0.75
	if math.Abs(headSize-wantHeadSize) > 0.01 {
		t.Errorf("thead cell font-size = %.3f, want %.3f (14px)", headSize, wantHeadSize)
	}

	// tbody: 13px -> 9.75pt. This is the bug: pre-fix, this resolved to the
	// inherited body font-size (16px -> 12pt) instead.
	bodySize := cellFontSize(t, rows[1].Cells()[0])
	wantBodySize := 13.0 * 0.75
	if math.Abs(bodySize-wantBodySize) > 0.01 {
		t.Errorf("tbody cell font-size = %.3f, want %.3f (13px) — tbody{font-size} did not cascade to the cell", bodySize, wantBodySize)
	}

	// thead and tbody must resolve independently, not just both happen to
	// pick up the same (wrong) value.
	if math.Abs(headSize-bodySize) < 0.01 {
		t.Errorf("thead (%.3f) and tbody (%.3f) resolved to the same font-size; expected them to differ (14px vs 13px)", headSize, bodySize)
	}
}

// TestTfootFontSizeCascadesToCells covers the third table-section selector
// fixed alongside tbody/thead.
func TestTfootFontSizeCascadesToCells(t *testing.T) {
	src := `<html><head><style>
		body { font-size: 16px; }
		tfoot { font-size: 10px; }
	</style></head><body>
		<table>
			<tbody><tr><td>Row</td></tr></tbody>
			<tfoot><tr><td>Total</td></tr></tfoot>
		</table>
	</body></html>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := elems[0].(*layout.Table)
	if !ok {
		t.Fatalf("expected *layout.Table, got %T", elems[0])
	}
	rows := tbl.Rows()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows (body + footer), got %d", len(rows))
	}

	footSize := cellFontSize(t, rows[1].Cells()[0])
	wantFootSize := 10.0 * 0.75
	if math.Abs(footSize-wantFootSize) > 0.01 {
		t.Errorf("tfoot cell font-size = %.3f, want %.3f (10px)", footSize, wantFootSize)
	}
}

// TestNonTableFontSizeUnaffectedByTbodyFix is a control case: fixing the
// tbody/thead/tfoot style-cascade bug must not change font-size resolution
// for ordinary non-table text.
func TestNonTableFontSizeUnaffectedByTbodyFix(t *testing.T) {
	src := `<html><head><style>
		body { font-size: 16px; }
		p { font-size: 11px; }
	</style></head><body>
		<p>Registros</p>
	</body></html>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(elems) == 0 {
		t.Fatal("expected elements")
	}
	p, ok := elems[0].(*layout.Paragraph)
	if !ok {
		t.Fatalf("expected *layout.Paragraph, got %T", elems[0])
	}
	runs := p.Runs()
	if len(runs) == 0 {
		t.Fatal("expected at least one run")
	}
	want := 11.0 * 0.75
	if math.Abs(runs[0].FontSize-want) > 0.01 {
		t.Errorf("<p> font-size = %.3f, want %.3f (11px)", runs[0].FontSize, want)
	}
}

// TestImplicitTbodyFontSizeCascades covers the common real-world shape where
// the author writes <table><tr>...</tr></table> with no explicit <tbody> —
// the HTML5 parser synthesizes one, so a `tbody { font-size }` selector must
// still match it.
func TestImplicitTbodyFontSizeCascades(t *testing.T) {
	src := `<html><head><style>
		body { font-size: 16px; }
		tbody { font-size: 13px; }
	</style></head><body>
		<table><tr><td>ACTIVA</td></tr></table>
	</body></html>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := elems[0].(*layout.Table)
	if !ok {
		t.Fatalf("expected *layout.Table, got %T", elems[0])
	}
	rows := tbl.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	got := cellFontSize(t, rows[0].Cells()[0])
	want := 13.0 * 0.75
	if math.Abs(got-want) > 0.01 {
		t.Errorf("implicit-tbody cell font-size = %.3f, want %.3f (13px)", got, want)
	}
}

// TestTbodyDisplayNoneHidesRows is the regression test for a section-level
// `display: none` being ignored: pre-fix, thead/tbody/tfoot were rendered
// unconditionally regardless of their own computed style (since that style
// was never computed at all), so a hidden section's rows still rendered.
func TestTbodyDisplayNoneHidesRows(t *testing.T) {
	src := `<html><head><style>
		tbody.hidden { display: none; }
	</style></head><body>
		<table>
			<tbody><tr><td>Visible</td></tr></tbody>
			<tbody class="hidden"><tr><td>Hidden</td></tr></tbody>
		</table>
	</body></html>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	tbl, ok := elems[0].(*layout.Table)
	if !ok {
		t.Fatalf("expected *layout.Table, got %T", elems[0])
	}
	rows := tbl.Rows()
	if len(rows) != 1 {
		t.Fatalf("expected only the visible tbody's row, got %d rows", len(rows))
	}
}

// TestCaptionOwnStyleCascades is the regression test for <caption> having
// the same "table's style used directly instead of the element's own"
// bug as thead/tbody/tfoot, in the same convertTable switch.
func TestCaptionOwnStyleCascades(t *testing.T) {
	src := `<html><head><style>
		table { font-size: 12px; }
		caption { font-size: 20px; text-align: left; }
	</style></head><body>
		<table><caption>Totals</caption><tr><td>Row</td></tr></table>
	</body></html>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The caption paragraph is emitted before the table.
	var caption *layout.Paragraph
	for _, e := range elems {
		if p, ok := e.(*layout.Paragraph); ok {
			caption = p
			break
		}
	}
	if caption == nil {
		t.Fatal("expected a caption paragraph among the elements")
	}
	runs := caption.Runs()
	if len(runs) == 0 {
		t.Fatal("expected at least one run in the caption")
	}
	want := 20.0 * 0.75
	if math.Abs(runs[0].FontSize-want) > 0.01 {
		t.Errorf("caption font-size = %.3f, want %.3f (20px)", runs[0].FontSize, want)
	}
	if caption.Align() != layout.AlignLeft {
		t.Errorf("caption align = %v, want AlignLeft (explicit text-align:left override)", caption.Align())
	}
}

// TestCaptionDisplayNoneOmitsCaption covers the same display:none guard for
// <caption> added alongside thead/tbody/tfoot's.
func TestCaptionDisplayNoneOmitsCaption(t *testing.T) {
	src := `<html><head><style>
		caption { display: none; }
	</style></head><body>
		<table><caption>Hidden Caption</caption><tr><td>Row</td></tr></table>
	</body></html>`

	elems, err := Convert(src, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range elems {
		if _, ok := e.(*layout.Paragraph); ok {
			t.Fatal("expected no caption paragraph when caption has display:none")
		}
	}
}
