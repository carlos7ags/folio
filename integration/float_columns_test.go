// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package integration

import (
	"math"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	foliohtml "github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/reader"
)

// TestFloatRightColumnPlacement renders a two-column float:left / float:right
// 50/50 layout and asserts the right column lands in the right half of the
// content box. The float:right column must resolve its 50% against the whole
// containing block, not the ~50% left over beside the float:left column, and
// must be positioned at the container's right edge.
func TestFloatRightColumnPlacement(t *testing.T) {
	const htmlStr = `<!DOCTYPE html><html><head><style>
.col-left  { float: left;  width: 50%; }
.col-right { float: right; width: 50%; }
</style></head><body>
<div class="col-left"><h2>Left column</h2><p>Left line one.</p></div>
<div class="col-right"><h2>Right column</h2><p>Right line one.</p></div>
</body></html>`

	doc := document.NewDocument(document.PageSizeA4)
	opts := &foliohtml.Options{
		PageWidth:  document.PageSizeA4.Width,
		PageHeight: document.PageSizeA4.Height,
	}
	if err := doc.AddHTML(htmlStr, opts); err != nil {
		t.Fatal(err)
	}
	pdf, err := doc.ToBytes()
	if err != nil {
		t.Fatal(err)
	}

	r, err := reader.Parse(pdf)
	if err != nil {
		t.Fatal(err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := page.ContentStream()
	if err != nil {
		t.Fatal(err)
	}
	spans := reader.NewContentProcessor(nil).Process(reader.ParseContentStream(data))
	if len(spans) == 0 {
		t.Fatal("no text spans extracted")
	}

	// Calibration: dump every span's text and horizontal position.
	for _, s := range spans {
		t.Logf("span: text=%q X=%.2f Y=%.2f W=%.2f", s.Text, s.X, s.Y, s.Width)
	}

	// Content box: A4 minus the document's default 72pt left/right margins.
	const margin = 72.0
	contentWidth := document.PageSizeA4.Width - 2*margin
	midX := margin + contentWidth/2

	leftX := math.Inf(1)
	rightX := math.Inf(1)
	for _, s := range spans {
		if strings.Contains(s.Text, "Left") && s.X < leftX {
			leftX = s.X
		}
		if strings.Contains(s.Text, "Right") && s.X < rightX {
			rightX = s.X
		}
	}
	if math.IsInf(leftX, 1) {
		t.Fatalf("no span containing %q found", "Left")
	}
	if math.IsInf(rightX, 1) {
		t.Fatalf("no span containing %q found", "Right")
	}

	t.Logf("leftX=%.2f rightX=%.2f contentWidth=%.2f midX=%.2f", leftX, rightX, contentWidth, midX)

	// The right column must sit ~half the content width to the right of the
	// left column (fix ≈ 0.5). The bug placed it ≈ 0.25 of the way; a 0.40
	// threshold cleanly separates the two regimes.
	delta := rightX - leftX
	if delta < 0.40*contentWidth {
		t.Errorf("right column not far enough right: rightX-leftX=%.2f, want >= %.2f (0.40*contentWidth, contentWidth=%.2f); leftX=%.2f rightX=%.2f",
			delta, 0.40*contentWidth, contentWidth, leftX, rightX)
	}

	// The right column must genuinely be in the right half of the content
	// box. Its content starts at the left edge of the right band, i.e. at the
	// content midpoint; allow a 1pt epsilon for rounding. The bug placed it
	// well left of the midpoint (≈ 0.25 of the content width).
	const eps = 1.0
	if rightX < midX-eps {
		t.Errorf("right column not in right half: rightX=%.2f, midX=%.2f (contentWidth=%.2f); leftX=%.2f",
			rightX, midX, contentWidth, leftX)
	}
}

// TestFloatLeftRightContentBelow renders two 50% float columns (left + right
// fill the whole width) followed by a normal in-flow block with NO clear. The
// after block cannot fit beside the floats, so it must drop BELOW both columns
// and render at full width — not squeeze into ~0 width beside them.
func TestFloatLeftRightContentBelow(t *testing.T) {
	const htmlStr = `<!DOCTYPE html><html><head><style>
.col-left  { float: left;  width: 50%; }
.col-right { float: right; width: 50%; }
</style></head><body>
<div class="col-left"><h2>Left column</h2><p>Left line one.</p></div>
<div class="col-right"><h2>Right column</h2><p>Right line one.</p></div>
<div class="after"><h2>Content after</h2><p>Should be below both columns.</p></div>
</body></html>`

	doc := document.NewDocument(document.PageSizeA4)
	opts := &foliohtml.Options{
		PageWidth:  document.PageSizeA4.Width,
		PageHeight: document.PageSizeA4.Height,
	}
	if err := doc.AddHTML(htmlStr, opts); err != nil {
		t.Fatal(err)
	}
	pdf, err := doc.ToBytes()
	if err != nil {
		t.Fatal(err)
	}

	r, err := reader.Parse(pdf)
	if err != nil {
		t.Fatal(err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := page.ContentStream()
	if err != nil {
		t.Fatal(err)
	}
	spans := reader.NewContentProcessor(nil).Process(reader.ParseContentStream(data))
	if len(spans) == 0 {
		t.Fatal("no text spans extracted")
	}

	// Calibration: dump every span's text and position.
	for _, s := range spans {
		t.Logf("span: text=%q X=%.2f Y=%.2f W=%.2f", s.Text, s.X, s.Y, s.Width)
	}

	// colBottomY: the lowest (smallest-Y) text belonging to either column.
	// afterTopY: the highest (largest-Y) text belonging to the after block,
	// afterLeftX: the X of that topmost after-block span.
	colBottomY := math.Inf(1)
	afterTopY := math.Inf(-1)
	afterLeftX := math.NaN()
	for _, s := range spans {
		isCol := strings.Contains(s.Text, "Left") || strings.Contains(s.Text, "Right") || strings.Contains(s.Text, "line")
		isAfter := strings.Contains(s.Text, "Content") || strings.Contains(s.Text, "after") ||
			strings.Contains(s.Text, "Should") || strings.Contains(s.Text, "below") ||
			strings.Contains(s.Text, "columns")
		if isCol && isAfter {
			t.Fatalf("span %q classified as both column and after; classifier keywords overlap", s.Text)
		}
		if isCol && s.Y < colBottomY {
			colBottomY = s.Y
		}
		if isAfter && s.Y > afterTopY {
			afterTopY = s.Y
			afterLeftX = s.X
		}
	}
	if math.IsInf(colBottomY, 1) {
		t.Fatal("no column spans found")
	}
	if math.IsInf(afterTopY, -1) {
		t.Fatal("no after-block spans found")
	}

	t.Logf("colBottomY=%.2f afterTopY=%.2f afterLeftX=%.2f", colBottomY, afterTopY, afterLeftX)

	// The after block must begin BELOW the lowest column text (smaller Y).
	if afterTopY >= colBottomY {
		t.Errorf("after block not below columns: afterTopY=%.2f, want < colBottomY=%.2f",
			afterTopY, colBottomY)
	}

	// The after block must render at full width, not squeezed into a ~0-width
	// vertical stack beside the floats. Its top line must start near the left
	// margin (≈ 72pt), not pushed right toward the float band.
	const margin = 72.0
	const leftEps = 5.0
	if afterLeftX > margin+leftEps {
		t.Errorf("after block pushed right (squeezed beside floats): afterLeftX=%.2f, want <= %.2f",
			afterLeftX, margin+leftEps)
	}
}

// TestFloatColumnsInsideRowDiv renders a .row block container that holds a
// float:left col, a float:right col, and an empty clear:both div, followed by
// an .after block. The floats must lay out side by side INSIDE the row, the
// row must grow to enclose them, and the .after block must paint BELOW the row
// at full width — not overlap the columns.
func TestFloatColumnsInsideRowDiv(t *testing.T) {
	const htmlStr = `<!DOCTYPE html><html><head><style>
.row{width:100%;} .col-left{float:left;width:50%;} .col-right{float:right;width:50%;} .clear{clear:both;}
</style></head><body>
<div class="row">
<div class="col-left"><h2>Left column</h2><p>Left line one.</p><p>Left line two.</p></div>
<div class="col-right"><h2>Right column</h2><p>Right line one.</p></div>
<div class="clear"></div>
</div>
<div class="after"><h2>Content after the row</h2><p>This must appear BELOW both columns.</p></div>
</body></html>`

	spans := renderHTMLSpans(t, htmlStr)

	for _, s := range spans {
		t.Logf("span: text=%q X=%.2f Y=%.2f W=%.2f", s.Text, s.X, s.Y, s.Width)
	}

	// colBottomY: the lowest (smallest-Y) column text. "Left line two." is the
	// lowest column line. afterTopY/afterLeftX: the highest after-block span.
	// "Left"/"Right"/"line" appear only in the columns. The after block shares
	// the words "column"/"columns"/"both", so those are deliberately excluded
	// from the column classifier to avoid cross-contaminating colBottomY.
	colBottomY := math.Inf(1)
	afterTopY := math.Inf(-1)
	afterLeftX := math.NaN()
	for _, s := range spans {
		isCol := strings.Contains(s.Text, "Left") || strings.Contains(s.Text, "Right") ||
			strings.Contains(s.Text, "line")
		isAfter := strings.Contains(s.Text, "Content") || strings.Contains(s.Text, "after") ||
			strings.Contains(s.Text, "BELOW") || strings.Contains(s.Text, "must") ||
			strings.Contains(s.Text, "appear")
		if isCol && isAfter {
			t.Fatalf("span %q classified as both column and after; classifier keywords overlap", s.Text)
		}
		if isCol && s.Y < colBottomY {
			colBottomY = s.Y
		}
		if isAfter && s.Y > afterTopY {
			afterTopY = s.Y
			afterLeftX = s.X
		}
	}
	if math.IsInf(colBottomY, 1) {
		t.Fatal("no column spans found")
	}
	if math.IsInf(afterTopY, -1) {
		t.Fatal("no after-block spans found")
	}

	t.Logf("colBottomY=%.2f afterTopY=%.2f afterLeftX=%.2f", colBottomY, afterTopY, afterLeftX)

	// The after block must begin BELOW the lowest column text (smaller Y).
	if afterTopY >= colBottomY {
		t.Errorf("after block not below row: afterTopY=%.2f, want < colBottomY=%.2f",
			afterTopY, colBottomY)
	}

	// The after block must render full-width: its top line starts near the
	// left margin (≈ 72pt), not pushed right toward the float band.
	const margin = 72.0
	const leftEps = 5.0
	if afterLeftX > margin+leftEps {
		t.Errorf("after block pushed right: afterLeftX=%.2f, want <= %.2f",
			afterLeftX, margin+leftEps)
	}
}

// TestFloatInsideBFCContainer renders an overflow:hidden .container that wraps
// two floated columns, followed by an .after block. The container must GROW to
// enclose both floats so they are NOT clipped away (this is the core bug), and
// the after block must paint below them.
func TestFloatInsideBFCContainer(t *testing.T) {
	const htmlStr = `<!DOCTYPE html><html><head><style>
.container{overflow:hidden;} .col-left{float:left;width:50%;} .col-right{float:right;width:50%;}
</style></head><body>
<div class="container">
<div class="col-left"><h2>Left column</h2><p>Left line one.</p></div>
<div class="col-right"><h2>Right column</h2><p>Right line one.</p></div>
</div>
<div class="after"><h2>Content after</h2><p>Should be below both columns.</p></div>
</body></html>`

	spans := renderHTMLSpans(t, htmlStr)

	for _, s := range spans {
		t.Logf("span: text=%q X=%.2f Y=%.2f W=%.2f", s.Text, s.X, s.Y, s.Width)
	}

	const margin = 72.0
	contentWidth := document.PageSizeA4.Width - 2*margin
	midX := margin + contentWidth/2

	// Core assertion: both columns' text survives (not clipped by the
	// collapsed overflow:hidden box). Track their positions too.
	var haveLeft, haveRight bool
	leftColX := math.Inf(1)
	rightColX := math.Inf(1)
	colBottomY := math.Inf(1)
	afterTopY := math.Inf(-1)
	for _, s := range spans {
		if strings.Contains(s.Text, "Left") {
			haveLeft = true
			if s.X < leftColX {
				leftColX = s.X
			}
		}
		if strings.Contains(s.Text, "Right") {
			haveRight = true
			if s.X < rightColX {
				rightColX = s.X
			}
		}
		isCol := strings.Contains(s.Text, "Left") || strings.Contains(s.Text, "Right") ||
			strings.Contains(s.Text, "line")
		isAfter := strings.Contains(s.Text, "Content") || strings.Contains(s.Text, "after") ||
			strings.Contains(s.Text, "Should")
		if isCol && isAfter {
			t.Fatalf("span %q classified as both column and after; classifier keywords overlap", s.Text)
		}
		if isCol && s.Y < colBottomY {
			colBottomY = s.Y
		}
		if isAfter && s.Y > afterTopY {
			afterTopY = s.Y
		}
	}

	if !haveLeft {
		t.Errorf("left column text missing — clipped away by collapsed container")
	}
	if !haveRight {
		t.Errorf("right column text missing — clipped away by collapsed container")
	}
	if math.IsInf(colBottomY, 1) {
		t.Fatal("no column spans found")
	}
	if math.IsInf(afterTopY, -1) {
		t.Fatal("no after-block spans found")
	}

	t.Logf("leftColX=%.2f rightColX=%.2f midX=%.2f colBottomY=%.2f afterTopY=%.2f",
		leftColX, rightColX, midX, colBottomY, afterTopY)

	// The right column must sit in the right half of the content box.
	const eps = 1.0
	if !math.IsInf(rightColX, 1) && rightColX < midX-eps {
		t.Errorf("right column not in right half: rightColX=%.2f, want >= %.2f (midX)",
			rightColX, midX)
	}

	// The after block must paint below both columns.
	if afterTopY >= colBottomY {
		t.Errorf("after block not below columns: afterTopY=%.2f, want < colBottomY=%.2f",
			afterTopY, colBottomY)
	}
}

// TestParagraphWrapsBesideFloatInDiv renders an overflow:hidden box holding a
// float:left side box and an in-flow paragraph. The paragraph must wrap in the
// reduced space to the RIGHT of the float (beside it, not under it), the box
// must grow to enclose the float, and the .after block must paint below.
func TestParagraphWrapsBesideFloatInDiv(t *testing.T) {
	const htmlStr = `<!DOCTYPE html><html><head><style>
.box{overflow:hidden;}
.side{float:left;width:120px;}
</style></head><body>
<div class="box">
<div class="side"><h2>Side</h2><p>Float text.</p></div>
<p>This paragraph should wrap in the reduced space to the right of the floated box, starting to the right of the float rather than under it, and the overflow:hidden container should grow to enclose the float.</p>
</div>
<div class="after"><h2>After</h2><p>Below the box.</p></div>
</body></html>`

	spans := renderHTMLSpans(t, htmlStr)

	for _, s := range spans {
		t.Logf("span: text=%q X=%.2f Y=%.2f W=%.2f", s.Text, s.X, s.Y, s.Width)
	}

	const margin = 72.0

	// floatX/floatBottomY: the leftmost X and lowest Y of the floated side box
	// content ("Side", "Float text."). besideTopY: the Y of the topmost
	// wrapping-paragraph line; besideX is the leftmost X on that line (its true
	// left edge), so it reflects where the paragraph actually starts. The
	// paragraph shares no words with the float or after block. afterTopY: the
	// highest after-block span.
	floatX := math.Inf(1)
	floatBottomY := math.Inf(1)
	var haveFloat bool
	besideTopY := math.Inf(-1)
	var haveBeside bool
	afterTopY := math.Inf(-1)
	var haveAfter bool
	classifyBeside := func(text string) bool {
		return strings.Contains(text, "paragraph") || strings.Contains(text, "wrap") ||
			strings.Contains(text, "reduced") || strings.Contains(text, "right") ||
			strings.Contains(text, "floated")
	}
	for _, s := range spans {
		isFloat := strings.Contains(s.Text, "Side") || strings.Contains(s.Text, "Float")
		isBeside := classifyBeside(s.Text)
		isAfter := strings.Contains(s.Text, "After") || strings.Contains(s.Text, "Below")
		if isFloat {
			haveFloat = true
			if s.X < floatX {
				floatX = s.X
			}
			if s.Y < floatBottomY {
				floatBottomY = s.Y
			}
		}
		if isBeside {
			haveBeside = true
			if s.Y > besideTopY {
				besideTopY = s.Y
			}
		}
		if isAfter {
			haveAfter = true
			if s.Y > afterTopY {
				afterTopY = s.Y
			}
		}
	}

	// besideX: the leftmost X among ALL spans on the topmost beside-line, not
	// just keyword-matching ones, so it captures the line's true left edge.
	besideX := math.Inf(1)
	for _, s := range spans {
		if math.Abs(s.Y-besideTopY) < 0.5 && s.X < besideX {
			besideX = s.X
		}
	}

	if !haveFloat {
		t.Fatal("floated side box text missing — clipped away by collapsed container")
	}
	if !haveBeside {
		t.Fatal("no wrapping-paragraph spans found")
	}
	if !haveAfter {
		t.Fatal("no after-block spans found")
	}

	t.Logf("floatX=%.2f floatBottomY=%.2f besideX=%.2f besideTopY=%.2f afterTopY=%.2f",
		floatX, floatBottomY, besideX, besideTopY, afterTopY)

	// The float must sit near the left margin (small X).
	const leftEps = 5.0
	if floatX > margin+leftEps {
		t.Errorf("floated box not near left margin: floatX=%.2f, want <= %.2f", floatX, margin+leftEps)
	}

	// The wrapping paragraph must start to the RIGHT of the float, past the
	// float band. The 120px float is 120*0.75=90pt wide (96dpi→72dpi), so its
	// right edge sits at margin+90 ≈ 162pt. The paragraph must start at or past
	// that, i.e. beside the float, not under it at the left margin.
	const floatBandRight = margin + 120.0*0.75
	if besideX < floatBandRight {
		t.Errorf("paragraph not beside float (under it instead): besideX=%.2f, want >= %.2f (margin+float width); floatX=%.2f",
			besideX, floatBandRight, floatX)
	}
	// Robust relative guard: the paragraph must be clearly right of the float,
	// by well over half the float band (90pt), not merely nudged right.
	if besideX-floatX < 80.0 {
		t.Errorf("paragraph not clearly right of float: besideX-floatX=%.2f, want >= 80; besideX=%.2f floatX=%.2f",
			besideX-floatX, besideX, floatX)
	}

	// The after block must paint below the float's content and below the
	// beside-text (smaller Y than both).
	if afterTopY >= floatBottomY {
		t.Errorf("after block not below float: afterTopY=%.2f, want < floatBottomY=%.2f", afterTopY, floatBottomY)
	}
	if afterTopY >= besideTopY {
		t.Errorf("after block not below beside-text: afterTopY=%.2f, want < besideTopY=%.2f", afterTopY, besideTopY)
	}
}

// renderHTMLSpans renders an HTML string to a single-page A4 PDF and returns
// the extracted text spans.
func renderHTMLSpans(t *testing.T, htmlStr string) []reader.TextSpan {
	t.Helper()
	doc := document.NewDocument(document.PageSizeA4)
	opts := &foliohtml.Options{
		PageWidth:  document.PageSizeA4.Width,
		PageHeight: document.PageSizeA4.Height,
	}
	if err := doc.AddHTML(htmlStr, opts); err != nil {
		t.Fatal(err)
	}
	pdf, err := doc.ToBytes()
	if err != nil {
		t.Fatal(err)
	}
	r, err := reader.Parse(pdf)
	if err != nil {
		t.Fatal(err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatal(err)
	}
	data, err := page.ContentStream()
	if err != nil {
		t.Fatal(err)
	}
	spans := reader.NewContentProcessor(nil).Process(reader.ParseContentStream(data))
	if len(spans) == 0 {
		t.Fatal("no text spans extracted")
	}
	return spans
}
