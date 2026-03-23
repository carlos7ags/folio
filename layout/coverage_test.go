// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"bytes"
	goimage "image"
	"image/jpeg"
	"os"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
)

// --- Helpers ---

func loadTestTTF(t *testing.T) *font.EmbeddedFont {
	t.Helper()
	paths := []string{
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			face, err := font.LoadTTF(p)
			if err != nil {
				t.Fatalf("LoadTTF failed: %v", err)
			}
			return font.NewEmbeddedFont(face)
		}
	}
	t.Skip("no system TTF font found")
	return nil
}

func testJPEG(t *testing.T) *folioimage.Image {
	t.Helper()
	img := goimage.NewRGBA(goimage.Rect(0, 0, 100, 80))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatal(err)
	}
	fimg, err := folioimage.NewJPEG(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return fimg
}

// --- Embedded font tests ---

func TestParagraphEmbeddedLayout(t *testing.T) {
	ef := loadTestTTF(t)
	p := NewParagraphEmbedded("Hello embedded world", ef, 12)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected at least 1 line")
	}
	if len(lines[0].Words) == 0 {
		t.Fatal("expected words")
	}
}

func TestParagraphEmbeddedPlanLayout(t *testing.T) {
	ef := loadTestTTF(t)
	p := NewParagraphEmbedded("Hello embedded world", ef, 12)
	plan := p.PlanLayout(LayoutArea{Width: 400, Height: 100})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
	if len(plan.Blocks) == 0 {
		t.Fatal("expected blocks")
	}
}

func TestParagraphEmbeddedRendering(t *testing.T) {
	ef := loadTestTTF(t)
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraphEmbedded("Embedded font text", ef, 14))
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "Tj") {
		t.Error("expected text operator")
	}
}

func TestHeadingEmbedded(t *testing.T) {
	ef := loadTestTTF(t)
	h := NewHeadingEmbedded("Embedded Heading", H1, ef)
	lines := h.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected lines")
	}
	plan := h.PlanLayout(LayoutArea{Width: 400, Height: 200})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
}

func TestLinkEmbedded(t *testing.T) {
	ef := loadTestTTF(t)
	l := NewLinkEmbedded("Click me", "https://example.com", ef, 12)
	plan := l.PlanLayout(LayoutArea{Width: 400, Height: 100})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
	if len(plan.Blocks) == 0 || len(plan.Blocks[0].Links) == 0 {
		t.Error("expected link metadata")
	}
}

func TestListEmbedded(t *testing.T) {
	ef := loadTestTTF(t)
	l := NewListEmbedded(ef, 12).AddItem("First").AddItem("Second")
	lines := l.Layout(400)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d", len(lines))
	}
}

func TestTabbedLineEmbedded(t *testing.T) {
	ef := loadTestTTF(t)
	tl := NewTabbedLineEmbedded(ef, 12,
		TabStop{Position: 300, Align: TabAlignRight},
	).SetSegments("Label", "Value")
	plan := tl.PlanLayout(LayoutArea{Width: 468, Height: 100})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
}

func TestTableCellEmbedded(t *testing.T) {
	ef := loadTestTTF(t)
	tbl := NewTable()
	row := tbl.AddRow()
	row.AddCellEmbedded("Embedded cell", ef, 10)
	row.AddCellEmbedded("Another cell", ef, 10)

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(tbl)
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
}

func TestRunEmbedded(t *testing.T) {
	ef := loadTestTTF(t)
	run := RunEmbedded("Embedded text", ef, 12)
	if run.Embedded != ef {
		t.Error("expected embedded font on run")
	}
	p := NewStyledParagraph(run)
	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected lines")
	}
}

func TestParagraphAddRun(t *testing.T) {
	p := NewParagraph("Hello", font.Helvetica, 12)
	p.AddRun(Run("World", font.HelveticaBold, 14))

	lines := p.Layout(400)
	if len(lines) == 0 {
		t.Fatal("expected lines")
	}
	// Should have words from both runs.
	totalWords := 0
	for _, l := range lines {
		totalWords += len(l.Words)
	}
	if totalWords < 2 {
		t.Errorf("expected at least 2 words, got %d", totalWords)
	}
}

// --- PlanLayout tests ---

func TestPlanLayoutBasic(t *testing.T) {
	p := NewParagraph("Custom element", font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 400, Height: 100})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
	if len(plan.Blocks) == 0 {
		t.Fatal("expected blocks")
	}
}

func TestPlanLayoutRendering(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewParagraph("Custom element rendered", font.Helvetica, 12))
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "Tj") {
		t.Error("expected text from element")
	}
}

func TestPlanLayoutSplit(t *testing.T) {
	longText := strings.Repeat("Word ", 500)
	p := NewParagraph(longText, font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 468, Height: 200})
	if plan.Status != LayoutPartial {
		t.Errorf("expected LayoutPartial for overflowing content, got %d", plan.Status)
	}
	if plan.Overflow == nil {
		t.Error("expected overflow element")
	}
}

func TestPlanLayoutNoSpace(t *testing.T) {
	p := NewParagraph("Text", font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 400, Height: 0})
	if plan.Status != LayoutNothing {
		t.Errorf("expected LayoutNothing with zero height, got %d", plan.Status)
	}
}

// --- Measurable tests for remaining elements ---

func TestLineSeparatorMeasurable(t *testing.T) {
	ls := NewLineSeparator()
	if ls.MinWidth() != 0 {
		t.Errorf("LineSeparator MinWidth = %.1f, want 0", ls.MinWidth())
	}
	if ls.MaxWidth() != 0 {
		t.Errorf("LineSeparator MaxWidth = %.1f, want 0", ls.MaxWidth())
	}
}

func TestLinkMeasurable(t *testing.T) {
	l := NewLink("Click here", "https://example.com", font.Helvetica, 12)
	if l.MinWidth() <= 0 {
		t.Error("Link MinWidth should be positive")
	}
	if l.MaxWidth() < l.MinWidth() {
		t.Error("Link MaxWidth should be >= MinWidth")
	}
}

func TestTabbedLineMeasurable(t *testing.T) {
	tl := NewTabbedLine(font.Helvetica, 12,
		TabStop{Position: 200, Align: TabAlignLeft},
		TabStop{Position: 400, Align: TabAlignRight},
	).SetSegments("A", "B", "C")
	if tl.MinWidth() != 400 {
		t.Errorf("TabbedLine MinWidth = %.1f, want 400 (rightmost stop)", tl.MinWidth())
	}
	if tl.MaxWidth() != 400 {
		t.Errorf("TabbedLine MaxWidth = %.1f, want 400", tl.MaxWidth())
	}
}

// --- Setter methods ---

func TestDivSetPaddingAll(t *testing.T) {
	d := NewDiv().SetPaddingAll(Padding{Top: 10, Right: 20, Bottom: 30, Left: 40})
	if d.padding.Top != 10 || d.padding.Right != 20 || d.padding.Bottom != 30 || d.padding.Left != 40 {
		t.Errorf("padding = %+v, want {10 20 30 40}", d.padding)
	}
}

func TestDivSetBorders(t *testing.T) {
	b := AllBorders(DashedBorder(2, ColorRed))
	d := NewDiv().SetBorders(b)
	if d.borders.Top.Style != BorderDashed {
		t.Error("expected dashed border")
	}
}

func TestLinkSetAlign(t *testing.T) {
	l := NewLink("Text", "https://example.com", font.Helvetica, 12).SetAlign(AlignCenter)
	lines := l.Layout(400)
	if lines[0].Align != AlignCenter {
		t.Error("expected center alignment")
	}
}

func TestTabbedLineSetLeading(t *testing.T) {
	tl := NewTabbedLine(font.Helvetica, 12).SetLeading(1.5).SetSegments("Text")
	lines := tl.Layout(400)
	expected := 12.0 * 1.5
	if lines[0].Height != expected {
		t.Errorf("height = %.1f, want %.1f (leading 1.5)", lines[0].Height, expected)
	}
}

// --- Image rendering through PlanLayout ---

func TestImagePlanLayout(t *testing.T) {
	img := testJPEG(t)
	ie := NewImageElement(img)
	plan := ie.PlanLayout(LayoutArea{Width: 468, Height: 500})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
	if len(plan.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(plan.Blocks))
	}
	if plan.Blocks[0].Tag != "Figure" {
		t.Errorf("tag = %q, want Figure", plan.Blocks[0].Tag)
	}
}

func TestImagePlanLayoutNoSpace(t *testing.T) {
	img := testJPEG(t)
	ie := NewImageElement(img).SetSize(100, 200)
	plan := ie.PlanLayout(LayoutArea{Width: 468, Height: 50})
	if plan.Status != LayoutNothing {
		t.Errorf("expected LayoutNothing for tall image, got %d", plan.Status)
	}
}

func TestImageRendering(t *testing.T) {
	img := testJPEG(t)
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(NewImageElement(img).SetSize(200, 0))
	pages := r.Render()
	if len(pages) != 1 {
		t.Fatalf("expected 1 page, got %d", len(pages))
	}
	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "Do") {
		t.Error("expected Do operator for image")
	}
	if len(pages[0].Images) == 0 {
		t.Error("expected image registered on page")
	}
}

// --- Tagged rendering ---

func TestTaggedRendering(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.SetTagged(true)
	r.Add(NewHeading("Title", H1))
	r.Add(NewParagraph("Body text.", font.Helvetica, 12))
	r.Add(NewLineSeparator())

	pages := r.Render()
	tags := r.StructTags()

	if len(tags) < 2 {
		t.Fatalf("expected at least 2 tags, got %d", len(tags))
	}

	hasH1 := false
	hasP := false
	for _, tag := range tags {
		if tag.Tag == "H1" {
			hasH1 = true
		}
		if tag.Tag == "P" {
			hasP = true
		}
	}
	if !hasH1 {
		t.Error("missing H1 tag")
	}
	if !hasP {
		t.Error("missing P tag")
	}

	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "BDC") {
		t.Error("expected BDC operator")
	}
	if !strings.Contains(content, "EMC") {
		t.Error("expected EMC operator")
	}
}

// --- Div page split ---

func TestDivPageSplit(t *testing.T) {
	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})

	d := NewDiv().SetPadding(10).SetBorder(DefaultBorder())
	for range 50 {
		d.Add(NewParagraph("Line of text inside a div that should eventually overflow.", font.Helvetica, 12))
	}
	r.Add(d)

	pages := r.Render()
	if len(pages) < 2 {
		t.Errorf("expected div to split across pages, got %d page(s)", len(pages))
	}
}

// --- Table rich cell rendering ---

func TestTableRichCellRendering(t *testing.T) {
	tbl := NewTable()
	row := tbl.AddRow()
	row.AddCell("Plain", font.Helvetica, 10)
	row.AddCellElement(NewParagraph("Rich content", font.HelveticaBold, 10))

	r := NewRenderer(612, 792, Margins{Top: 72, Right: 72, Bottom: 72, Left: 72})
	r.Add(tbl)
	pages := r.Render()

	content := string(pages[0].Stream.Bytes())
	if !strings.Contains(content, "Tj") {
		t.Error("expected text from rich cell")
	}
}

// --- LineSeparator PlanLayout ---

func TestLineSeparatorPlanLayout(t *testing.T) {
	ls := NewLineSeparator().SetWidth(1).SetColor(ColorRed).SetStyle(BorderDashed)
	plan := ls.PlanLayout(LayoutArea{Width: 468, Height: 100})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull, got %d", plan.Status)
	}
	if len(plan.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(plan.Blocks))
	}
}

func TestLineSeparatorPlanLayoutNoSpace(t *testing.T) {
	ls := NewLineSeparator().SetSpaceBefore(50).SetSpaceAfter(50)
	plan := ls.PlanLayout(LayoutArea{Width: 468, Height: 10})
	if plan.Status != LayoutNothing {
		t.Errorf("expected LayoutNothing, got %d", plan.Status)
	}
}

// --- Paragraph split via PlanLayout ---

func TestParagraphPlanLayoutPartialSplit(t *testing.T) {
	longText := strings.Repeat("Word ", 200)
	p := NewParagraph(longText, font.Helvetica, 12)

	plan := p.PlanLayout(LayoutArea{Width: 468, Height: 50})
	if plan.Status != LayoutPartial {
		t.Errorf("expected LayoutPartial, got %d", plan.Status)
	}
	if plan.Overflow == nil {
		t.Error("expected overflow paragraph")
	}
	if len(plan.Blocks) == 0 {
		t.Error("expected some blocks that fit")
	}

	// Overflow should also be a Paragraph.
	if _, ok := plan.Overflow.(*Paragraph); !ok {
		t.Errorf("overflow type = %T, want *Paragraph", plan.Overflow)
	}
}

func TestParagraphPlanLayoutNoHeight(t *testing.T) {
	p := NewParagraph("Hello", font.Helvetica, 12)
	plan := p.PlanLayout(LayoutArea{Width: 400, Height: 0})
	if plan.Status != LayoutNothing {
		t.Errorf("expected LayoutNothing with zero height, got %d", plan.Status)
	}
}

func TestSVGElementNilSVG(t *testing.T) {
	// NewSVGElement with nil SVG should not panic on layout operations.
	se := NewSVGElement(nil)

	// resolveSize should return 0,0.
	w, h := se.resolveSize(400)
	if w != 0 || h != 0 {
		t.Errorf("expected 0,0 for nil SVG, got %f,%f", w, h)
	}

	// MinWidth/MaxWidth should not panic.
	if se.MinWidth() != 1 {
		t.Errorf("expected MinWidth=1, got %f", se.MinWidth())
	}
	if se.MaxWidth() != 1 {
		t.Errorf("expected MaxWidth=1, got %f", se.MaxWidth())
	}

	// PlanLayout should return zero-height full layout.
	plan := se.PlanLayout(LayoutArea{Width: 400, Height: 500})
	if plan.Status != LayoutFull {
		t.Errorf("expected LayoutFull for nil SVG, got %d", plan.Status)
	}
}
