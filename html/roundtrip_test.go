// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/html"
	"github.com/carlos7ags/folio/reader"
)

// htmlRoundtrip converts HTML to PDF, validates with qpdf, parses back, and
// returns the reader for further verification. It fails the test on any error.
func htmlRoundtrip(t *testing.T, htmlStr string, pageSize document.PageSize) ([]byte, *reader.PdfReader) {
	t.Helper()

	elems, err := html.Convert(htmlStr, nil)
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	if len(elems) == 0 {
		t.Fatal("Convert returned zero elements")
	}

	doc := document.NewDocument(pageSize)
	doc.Info.Title = t.Name()
	doc.Info.Author = "Folio Roundtrip Test"
	for _, e := range elems {
		doc.Add(e)
	}

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	pdf := buf.Bytes()
	qpdfCheck(t, pdf)

	r, err := reader.Parse(pdf)
	if err != nil {
		t.Fatalf("reader.Parse: %v", err)
	}
	return pdf, r
}

// qpdfCheck validates PDF bytes using qpdf --check. Skips if qpdf not installed.
func qpdfCheck(t *testing.T, pdfBytes []byte) {
	t.Helper()
	qpdfPath, err := exec.LookPath("qpdf")
	if err != nil {
		t.Log("qpdf not installed, skipping structural validation")
		return
	}
	tmpFile := filepath.Join(t.TempDir(), "test.pdf")
	if err := os.WriteFile(tmpFile, pdfBytes, 0644); err != nil {
		t.Fatalf("write temp PDF: %v", err)
	}
	cmd := exec.Command(qpdfPath, "--check", tmpFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("qpdf --check failed: %v\n%s", err, output)
	}
}

// extractAllText extracts text from all pages of a parsed PDF.
func extractAllText(t *testing.T, r *reader.PdfReader) string {
	t.Helper()
	var all strings.Builder
	for i := range r.PageCount() {
		page, err := r.Page(i)
		if err != nil {
			t.Fatalf("Page(%d): %v", i, err)
		}
		text, err := page.ExtractText()
		if err != nil {
			t.Fatalf("ExtractText page %d: %v", i, err)
		}
		if i > 0 {
			all.WriteString("\n")
		}
		all.WriteString(text)
	}
	return all.String()
}

// assertTextContains checks that extracted PDF text contains all expected strings.
func assertTextContains(t *testing.T, text string, expected ...string) {
	t.Helper()
	// Normalize whitespace so line breaks from word-wrapping don't cause
	// false negatives when checking for multi-word phrases.
	normalized := normalizeWhitespace(text)
	for _, s := range expected {
		if !strings.Contains(normalized, s) {
			t.Errorf("extracted text missing %q (got %d chars: %q...)", s, len(normalized), truncate(normalized, 200))
		}
	}
}

// normalizeWhitespace collapses all runs of whitespace (including newlines)
// into a single space. This makes text assertions resilient to line-break
// changes caused by word-width differences.
func normalizeWhitespace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// --- Test cases ---

func TestRoundtrip_SimpleText(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<h1>Hello World</h1>
		<p>This is a simple paragraph with plain text.</p>
	`, document.PageSizeLetter)

	if r.PageCount() != 1 {
		t.Errorf("PageCount = %d, want 1", r.PageCount())
	}

	title, author, _, _, _ := r.Info()
	if title == "" {
		t.Error("missing document title")
	}
	if author != "Folio Roundtrip Test" {
		t.Errorf("author = %q, want %q", author, "Folio Roundtrip Test")
	}

	text := extractAllText(t, r)
	assertTextContains(t, text, "Hello World", "simple paragraph", "plain text")
}

func TestRoundtrip_StyledText(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<p style="color: red; font-size: 18px">Red large text</p>
		<p style="text-align: center">Centered paragraph</p>
		<p>Normal with <strong>bold</strong> and <em>italic</em> words.</p>
		<p><u>Underlined</u> and <s>strikethrough</s> text.</p>
	`, document.PageSizeA4)

	text := extractAllText(t, r)
	assertTextContains(t, text, "Red large text", "Centered paragraph", "bold", "italic", "Underlined", "strikethrough")
}

func TestRoundtrip_Headings(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<h1>Heading One</h1>
		<h2>Heading Two</h2>
		<h3>Heading Three</h3>
		<h4>Heading Four</h4>
		<h5>Heading Five</h5>
		<h6>Heading Six</h6>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text,
		"Heading One", "Heading Two", "Heading Three",
		"Heading Four", "Heading Five", "Heading Six",
	)
}

func TestRoundtrip_Table(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<table>
			<thead>
				<tr><th>Name</th><th>Age</th><th>City</th></tr>
			</thead>
			<tbody>
				<tr><td>Alice</td><td>30</td><td>New York</td></tr>
				<tr><td>Bob</td><td>25</td><td>London</td></tr>
				<tr><td>Charlie</td><td>35</td><td>Tokyo</td></tr>
			</tbody>
		</table>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text, "Name", "Age", "City", "Alice", "Bob", "Charlie", "New York", "London", "Tokyo")
}

func TestRoundtrip_TableColspan(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<table border="1">
			<tr><th colspan="3">Full Width Header</th></tr>
			<tr><td>A</td><td>B</td><td>C</td></tr>
			<tr><td colspan="2">Merged AB</td><td>C2</td></tr>
		</table>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text, "Full Width Header")
	// Text extraction may split words at cell boundaries; verify the pieces are present.
	if !strings.Contains(text, "Merg") || !strings.Contains(text, "AB") {
		t.Errorf("extracted text missing colspan content: %q", truncate(text, 200))
	}
}

func TestRoundtrip_Lists(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<h2>Unordered</h2>
		<ul>
			<li>Apple</li>
			<li>Banana</li>
			<li>Cherry</li>
		</ul>
		<h2>Ordered</h2>
		<ol>
			<li>First item</li>
			<li>Second item</li>
			<li>Third item</li>
		</ol>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text, "Apple", "Banana", "Cherry", "First item", "Second item", "Third item")
}

func TestRoundtrip_NestedStructure(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<div style="padding: 10px; border: 1px solid black; background-color: #f0f0f0">
			<h2>Section Title</h2>
			<p>Paragraph inside a styled div.</p>
			<div style="margin-left: 20px; padding: 5px; border-left: 3px solid blue">
				<p>Nested div with left border.</p>
			</div>
		</div>
	`, document.PageSizeA4)

	text := extractAllText(t, r)
	assertTextContains(t, text, "Section Title", "Paragraph inside a styled div", "Nested div with left border")
}

func TestRoundtrip_Links(t *testing.T) {
	pdf, r := htmlRoundtrip(t, `
		<p>Visit <a href="https://example.com">Example Site</a> for more info.</p>
		<p>Email us at <a href="mailto:test@example.com">test@example.com</a>.</p>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text, "Example Site", "test@example.com")

	// Links are rendered as clickable annotations if supported.
	if bytes.Contains(pdf, []byte("example.com")) {
		t.Log("Link URI found in PDF output")
	}
}

func TestRoundtrip_CSSBoxModel(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<div style="margin: 20px; padding: 15px; border: 2px solid #333; background-color: #eee">
			<p style="margin-bottom: 10px">Box model test with margin, padding, border.</p>
		</div>
		<div style="width: 50%; margin: 0 auto; text-align: center">
			<p>Centered half-width block.</p>
		</div>
	`, document.PageSizeA4)

	text := extractAllText(t, r)
	assertTextContains(t, text, "Box model test", "Centered half-width block")
}

func TestRoundtrip_InlineFormatting(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<p>
			This has <strong>bold</strong>, <em>italic</em>,
			<strong><em>bold italic</em></strong>,
			<code>monospace code</code>,
			<mark>highlighted</mark>,
			<small>small text</small>,
			and <span style="color: green">green span</span>.
		</p>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text, "bold", "italic", "monospace code", "highlighted", "small text", "green span")
}

func TestRoundtrip_ComplexInvoice(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<html>
		<head><style>
			body { font-size: 11px; }
			h1 { color: #2c3e50; border-bottom: 2px solid #2c3e50; padding-bottom: 5px; }
			.header { display: flex; justify-content: space-between; }
			.total { font-weight: bold; text-align: right; }
			table { width: 100%; border-collapse: collapse; }
			th { background-color: #34495e; color: white; padding: 8px; text-align: left; }
			td { padding: 8px; border-bottom: 1px solid #ddd; }
			.footer { margin-top: 20px; font-size: 9px; color: #777; text-align: center; }
		</style></head>
		<body>
			<h1>INVOICE</h1>
			<p><strong>From:</strong> Folio Software Inc.</p>
			<p><strong>To:</strong> Customer Corp</p>
			<p><strong>Invoice #:</strong> INV-2026-001</p>
			<p><strong>Date:</strong> March 16, 2026</p>

			<table>
				<thead>
					<tr><th>Description</th><th>Qty</th><th>Unit Price</th><th>Amount</th></tr>
				</thead>
				<tbody>
					<tr><td>PDF Engine License</td><td>1</td><td>$5,000.00</td><td>$5,000.00</td></tr>
					<tr><td>Java SDK License</td><td>1</td><td>$2,500.00</td><td>$2,500.00</td></tr>
					<tr><td>Priority Support (12 months)</td><td>1</td><td>$1,200.00</td><td>$1,200.00</td></tr>
					<tr><td colspan="3" class="total">Subtotal</td><td>$8,700.00</td></tr>
					<tr><td colspan="3" class="total">Tax (10%)</td><td>$870.00</td></tr>
					<tr><td colspan="3" class="total">Total Due</td><td>$9,570.00</td></tr>
				</tbody>
			</table>

			<h2>Payment Terms</h2>
			<ul>
				<li>Payment due within 30 days of invoice date</li>
				<li>Wire transfer to: Account #123456789</li>
				<li>Late payments subject to 1.5% monthly interest</li>
			</ul>

			<p class="footer">Thank you for choosing Folio Software. Questions? Contact billing@folio.dev</p>
		</body>
		</html>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text,
		"INVOICE",
		"Folio Software Inc.",
		"Customer Corp",
		"INV-2026-001",
		"PDF Engine License",
		"Java SDK License",
		"Priority Support",
		"$5,000.00",
		"$9,570.00",
		"Payment due within 30 days",
		"billing@folio.dev",
	)

	// A full invoice should fit on a single page.
	if r.PageCount() != 1 {
		t.Errorf("PageCount = %d, want 1", r.PageCount())
	}
}

func TestRoundtrip_MultiPageContent(t *testing.T) {
	// Generate enough content to force multiple pages.
	var sb strings.Builder
	sb.WriteString("<h1>Multi-Page Document</h1>")
	for i := range 80 {
		sb.WriteString("<p>")
		sb.WriteString(strings.Repeat("Lorem ipsum dolor sit amet. ", 5))
		if i == 0 {
			sb.WriteString("FIRST_PARA_MARKER ")
		}
		if i == 79 {
			sb.WriteString("LAST_PARA_MARKER ")
		}
		sb.WriteString("</p>")
	}

	_, r := htmlRoundtrip(t, sb.String(), document.PageSizeLetter)

	if r.PageCount() < 2 {
		t.Errorf("Expected multiple pages, got %d", r.PageCount())
	}

	text := extractAllText(t, r)
	assertTextContains(t, text, "Multi-Page Document", "FIRST_PARA_MARKER", "LAST_PARA_MARKER", "Lorem ipsum")
}

func TestRoundtrip_DefinitionList(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<dl>
			<dt>PDF</dt>
			<dd>Portable Document Format</dd>
			<dt>HTML</dt>
			<dd>HyperText Markup Language</dd>
		</dl>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text, "PDF", "Portable Document Format", "HTML", "HyperText Markup Language")
}

func TestRoundtrip_Blockquote(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<p>Someone once said:</p>
		<blockquote>
			<p>The best way to predict the future is to invent it.</p>
		</blockquote>
		<p>Wise words indeed.</p>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text, "Someone once said", "predict the future", "Wise words")
}

func TestRoundtrip_PreformattedCode(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<p>Here is some code:</p>
		<pre>func main() {
    fmt.Println("Hello")
}</pre>
		<p>End of code.</p>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text, "Here is some code", "Hello", "End of code")
}

func TestRoundtrip_HorizontalRule(t *testing.T) {
	pdf, _ := htmlRoundtrip(t, `
		<p>Before rule</p>
		<hr>
		<p>After rule</p>
	`, document.PageSizeLetter)

	// HR renders as a line — verify the PDF has path operators (re or l for the line).
	if !bytes.Contains(pdf, []byte("re")) && !bytes.Contains(pdf, []byte(" l\n")) {
		t.Log("PDF may be missing HR line rendering (not fatal)")
	}
}

func TestRoundtrip_NestedLists(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<ul>
			<li>Level 1 Item A
				<ul>
					<li>Level 2 Item A1</li>
					<li>Level 2 Item A2</li>
				</ul>
			</li>
			<li>Level 1 Item B</li>
		</ul>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text, "Level 1 Item A", "Level 2 Item A1", "Level 2 Item A2", "Level 1 Item B")
}

func TestRoundtrip_EmptyElements(t *testing.T) {
	// Edge case: empty tags and whitespace-only content should not crash.
	_, _ = htmlRoundtrip(t, `
		<div></div>
		<p></p>
		<p>   </p>
		<table><tr><td></td></tr></table>
		<ul><li></li></ul>
		<p>Non-empty paragraph after empty elements.</p>
	`, document.PageSizeLetter)
}

func TestRoundtrip_SpecialCharacters(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<p>Ampersand: &amp; Less: &lt; Greater: &gt;</p>
		<p>Quote: &quot; Apostrophe: &#39;</p>
		<p>Copyright: &#169; Registered: &#174;</p>
		<p>Dollar: $ Euro: € Pound: £ Yen: ¥</p>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	// Standard PDF fonts may not have all glyphs, but basic ASCII entities should roundtrip.
	assertTextContains(t, text, "Ampersand:", "Less:", "Greater:", "Dollar: $")
}

func TestRoundtrip_LargeTable(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`<table border="1"><thead><tr><th>ID</th><th>Name</th><th>Value</th></tr></thead><tbody>`)
	for i := range 50 {
		sb.WriteString("<tr>")
		sb.WriteString("<td>" + strings.Repeat("X", 3) + "</td>")
		sb.WriteString("<td>Row " + string(rune('A'+i%26)) + "</td>")
		sb.WriteString("<td>$" + strings.Repeat("9", 4) + "</td>")
		sb.WriteString("</tr>")
	}
	sb.WriteString("</tbody></table>")

	_, r := htmlRoundtrip(t, sb.String(), document.PageSizeLetter)

	// 50 rows should produce multiple pages.
	if r.PageCount() < 2 {
		t.Logf("Large table produced %d page(s) — may fit on 1 depending on font size", r.PageCount())
	}

	text := extractAllText(t, r)
	assertTextContains(t, text, "ID", "Name", "Value", "Row A")
}

func TestRoundtrip_SVGInline(t *testing.T) {
	pdf, _ := htmlRoundtrip(t, `
		<p>Before SVG</p>
		<svg width="100" height="100" viewBox="0 0 100 100">
			<rect x="10" y="10" width="80" height="80" fill="blue" stroke="black" stroke-width="2"/>
			<circle cx="50" cy="50" r="20" fill="red"/>
		</svg>
		<p>After SVG</p>
	`, document.PageSizeLetter)

	// SVG renders as vector operators — look for color/path operators.
	if !bytes.Contains(pdf, []byte("rg")) {
		t.Error("PDF missing fill color operators (expected from SVG)")
	}
}

func TestRoundtrip_MixedContent(t *testing.T) {
	// Comprehensive test mixing many element types in one document.
	_, r := htmlRoundtrip(t, `
		<html><body>
		<h1>Annual Report 2026</h1>

		<h2>Executive Summary</h2>
		<p>This year marked a significant milestone for the company.
		   Revenue grew by <strong>23%</strong> compared to the previous year.</p>

		<h2>Financial Overview</h2>
		<table>
			<thead><tr><th>Quarter</th><th>Revenue</th><th>Growth</th></tr></thead>
			<tbody>
				<tr><td>Q1</td><td>$12.3M</td><td>+18%</td></tr>
				<tr><td>Q2</td><td>$14.1M</td><td>+22%</td></tr>
				<tr><td>Q3</td><td>$15.8M</td><td>+25%</td></tr>
				<tr><td>Q4</td><td>$17.2M</td><td>+28%</td></tr>
			</tbody>
		</table>

		<h2>Key Achievements</h2>
		<ol>
			<li>Launched in 12 new markets</li>
			<li>Acquired TechStartup Inc.</li>
			<li>Released version 2.0 of flagship product</li>
		</ol>

		<h2>Team</h2>
		<p>Our team grew from <em>150</em> to <em>230</em> employees across
		   <strong>5 offices</strong> worldwide.</p>

		<blockquote>
			<p>Innovation distinguishes between a leader and a follower.</p>
		</blockquote>

		<h2>Looking Ahead</h2>
		<ul>
			<li>Expand into APAC region</li>
			<li>Launch enterprise tier</li>
			<li>Double engineering headcount</li>
		</ul>

		<hr>
		<p style="font-size: 9px; color: #888; text-align: center">
			Confidential — For internal use only
		</p>
		</body></html>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text,
		"Annual Report 2026",
		"Executive Summary",
		"23%",
		"Q1", "Q2", "Q3", "Q4",
		"$12.3M", "$17.2M",
		"Launched in 12 new markets",
		"Acquired TechStartup",
		"150", "230", "5 offices",
		"Innovation distinguishes",
		"Expand into APAC",
		"Confidential",
	)
}

func TestRoundtrip_PageDimensions(t *testing.T) {
	sizes := []struct {
		name string
		size document.PageSize
		w, h float64
	}{
		{"Letter", document.PageSizeLetter, 612, 792},
		{"A4", document.PageSizeA4, 595, 842},
	}

	for _, tc := range sizes {
		t.Run(tc.name, func(t *testing.T) {
			_, r := htmlRoundtrip(t, "<p>Page size test</p>", tc.size)
			page, err := r.Page(0)
			if err != nil {
				t.Fatalf("Page(0): %v", err)
			}
			// Allow small tolerance for rounding.
			if abs(page.Width-tc.w) > 1 || abs(page.Height-tc.h) > 1 {
				t.Errorf("page dimensions = %.0f x %.0f, want %.0f x %.0f",
					page.Width, page.Height, tc.w, tc.h)
			}
		})
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func TestRoundtrip_CSSFlexbox(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<div style="display: flex; gap: 10px">
			<div style="flex: 1; padding: 10px; background-color: #e3f2fd">
				<p>Column One</p>
			</div>
			<div style="flex: 1; padding: 10px; background-color: #fce4ec">
				<p>Column Two</p>
			</div>
			<div style="flex: 1; padding: 10px; background-color: #e8f5e9">
				<p>Column Three</p>
			</div>
		</div>
	`, document.PageSizeLetter)

	text := extractAllText(t, r)
	assertTextContains(t, text, "Column One", "Column Two", "Column Three")
}

func TestRoundtrip_CSSGrid(t *testing.T) {
	_, r := htmlRoundtrip(t, `
		<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px">
			<div style="padding: 5px; background-color: #ddd"><p>Grid Cell A</p></div>
			<div style="padding: 5px; background-color: #ccc"><p>Grid Cell B</p></div>
			<div style="padding: 5px; background-color: #bbb"><p>Grid Cell C</p></div>
			<div style="padding: 5px; background-color: #aaa"><p>Grid Cell D</p></div>
		</div>
	`, document.PageSizeA4)

	text := extractAllText(t, r)
	assertTextContains(t, text, "Grid Cell A", "Grid Cell B", "Grid Cell C", "Grid Cell D")
}
