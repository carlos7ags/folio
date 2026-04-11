# Folio

A modern PDF library for Go — layout engine, HTML to PDF, redaction,
forms, digital signatures, barcodes, page import, and PDF/A compliance.

[![Go Reference](https://pkg.go.dev/badge/github.com/carlos7ags/folio.svg)](https://pkg.go.dev/github.com/carlos7ags/folio)
[![CI](https://github.com/carlos7ags/folio/actions/workflows/ci.yml/badge.svg)](https://github.com/carlos7ags/folio/actions)
[![Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

**[Try it live in your browser](https://playground.foliopdf.dev/)**

![Folio Playground](assets/playground.png)

---

## Install

```bash
go get github.com/carlos7ags/folio
```

Requires Go 1.25+. Two external dependencies: `golang.org/x/image` and `golang.org/x/net`.

## Language SDKs

| Language | Package | Status |
|----------|---------|--------|
| **Go** | `go get github.com/carlos7ags/folio` | This repo |
| **Java** | [`dev.foliopdf:folio-java`](https://central.sonatype.com/artifact/dev.foliopdf/folio-java) | [folio-java](https://github.com/carlos7ags/folio-java) |
| **WASM** | [Playground](https://playground.foliopdf.dev) | Built-in |

---

## Quick Start

```go
package main

import (
    "github.com/carlos7ags/folio/document"
    "github.com/carlos7ags/folio/font"
    "github.com/carlos7ags/folio/layout"
)

func main() {
    doc := document.NewDocument(document.PageSizeA4)
    doc.Info.Title = "Hello World"
    doc.SetAutoBookmarks(true)

    doc.Add(layout.NewHeading("Hello, Folio!", layout.H1))
    doc.Add(layout.NewParagraph(
        "Generated with Folio — the modern PDF library for Go.",
        font.Helvetica, 12,
    ))

    doc.Save("hello.pdf")
}
```

---

## HTML to PDF

The fastest way to generate PDFs — paste any HTML template and get a PDF.
No Chrome, no Puppeteer, no server required.

```go
import (
    "github.com/carlos7ags/folio/document"
    "github.com/carlos7ags/folio/html"
)

doc := document.NewDocument(document.PageSizeLetter)
elems, _ := html.Convert(`
    <h1>Invoice #1042</h1>
    <p>Bill to: <strong>Acme Corp</strong></p>
    <table border="1">
        <tr><th>Item</th><th>Amount</th></tr>
        <tr><td>Consulting</td><td>$1,200</td></tr>
    </table>
`, nil)
for _, e := range elems {
    doc.Add(e)
}
doc.Save("invoice.pdf")
```

Supports 40+ HTML elements, inline and `<style>` block CSS, flexbox, CSS grid,
SVG, named/hex/rgb colors, `@page` rules, and tables with colspan.

**[Try HTML to PDF live in your browser](https://playground.foliopdf.dev/)**

---

## Layout Engine

Folio uses a plan-based layout engine — layout is a pure function with no
mutation during rendering. Elements can be laid out multiple times safely,
which makes page break splitting clean and predictable.

```go
doc := document.NewDocument(document.PageSizeLetter)
doc.Info.Title = "Quarterly Report"
doc.Info.Author = "Finance Team"
doc.SetAutoBookmarks(true)

doc.Add(layout.NewHeading("Q3 Revenue Report", layout.H1))

doc.Add(layout.NewParagraph("Revenue grew 23% year over year.",
    font.Helvetica, 12).
    SetAlign(layout.AlignJustify).
    SetSpaceAfter(10))

tbl := layout.NewTable().SetAutoColumnWidths()
h := tbl.AddHeaderRow()
h.AddCell("Product", font.HelveticaBold, 10)
h.AddCell("Units", font.HelveticaBold, 10)
h.AddCell("Revenue", font.HelveticaBold, 10)

r := tbl.AddRow()
r.AddCell("Widget A", font.Helvetica, 10)
r.AddCell("1,200", font.Helvetica, 10)
r.AddCell("$48,000", font.Helvetica, 10)
doc.Add(tbl)

doc.Save("report.pdf")
```

### Layout Elements

| Element | Description |
|---|---|
| `Paragraph` | Word-wrapped text with alignment, leading, orphans/widows |
| `Heading` | H1-H6 with preset sizes, spacing, and auto-bookmarks |
| `Table` | Borders, colspan, rowspan, header repetition, auto-column widths |
| `List` | Bullet, numbered, Roman, alpha, nested |
| `Div` | Container with borders, background, padding |
| `Flex` | Flexbox layout with direction, wrap, alignment |
| `Image` | JPEG, PNG, TIFF with aspect ratio preservation |
| `LineSeparator` | Horizontal rule (solid, dashed, dotted) |
| `TabbedLine` | Tab stops with dot leaders (for TOCs) |
| `Link` | Clickable text with URL or internal destination |
| `Float` | Left/right floating with text wrap |
| `Columns` | Multi-column layout with automatic balancing |
| `AreaBreak` | Explicit page break |
| `BarcodeElement` | Code128, QR, EAN-13 inline in layout |

---

## Styled Text

```go
p := layout.NewStyledParagraph(
    layout.NewRun("Normal text ", font.Helvetica, 12),
    layout.NewRun("bold ", font.HelveticaBold, 12),
    layout.NewRun("colored and underlined", font.Helvetica, 12).
        WithColor(layout.ColorRed).
        WithUnderline(),
)
doc.Add(p)
```

---

## Tables

```go
tbl := layout.NewTable().SetAutoColumnWidths()
// Or explicit widths:
tbl.SetColumnUnitWidths([]layout.UnitValue{
    layout.Pct(30), layout.Pct(70),
})

// Header rows repeat automatically on page breaks
h := tbl.AddHeaderRow()
h.AddCell("Name", font.HelveticaBold, 10)
h.AddCell("Value", font.HelveticaBold, 10)

r := tbl.AddRow()
cell := r.AddCell("Styled cell", font.Helvetica, 10)
cell.SetBorders(layout.AllBorders(layout.DashedBorder(1, layout.ColorBlue)))
cell.SetBackground(layout.ColorLightGray)
cell.SetVAlign(layout.VAlignMiddle)
```

---

## Barcodes

```go
import "github.com/carlos7ags/folio/barcode"

qr, _ := barcode.NewQR("https://example.com")
doc.Add(layout.NewBarcodeElement(qr, 100).SetAlign(layout.AlignCenter))

bc, _ := barcode.NewCode128("SHIP-2024-001")
doc.Add(layout.NewBarcodeElement(bc, 200))

ean, _ := barcode.NewEAN13("590123412345")
doc.Add(layout.NewBarcodeElement(ean, 150))
```

---

## Interactive Forms

```go
import "github.com/carlos7ags/folio/forms"

form := forms.NewAcroForm()
form.Add(forms.NewTextField("name", [4]float64{72, 700, 300, 720}, 0))
form.Add(forms.NewCheckbox("agree", [4]float64{72, 670, 92, 690}, 0, false))
form.Add(forms.NewDropdown("role", [4]float64{72, 640, 250, 660}, 0,
    []string{"Developer", "Designer", "Manager"}))

doc.SetAcroForm(form)
doc.Save("form.pdf")
```

---

## Digital Signatures

```go
import "github.com/carlos7ags/folio/sign"

signer, _ := sign.NewLocalSigner(privateKey, []*x509.Certificate{cert})
signed, _ := sign.SignPDF(pdfBytes, sign.Options{
    Signer:   signer,
    Level:    sign.LevelBB,
    Reason:   "Approved",
    Location: "New York",
})
os.WriteFile("signed.pdf", signed, 0644)
```

Supports PAdES B-B, B-T (timestamped), and B-LT (long-term validation with
embedded OCSP responses and CRLs). Also supports external signers (HSM, KMS)
via the `Signer` interface. Uses Go stdlib crypto.

---

## Reading and Merging PDFs

```go
import "github.com/carlos7ags/folio/reader"

// Read
r, _ := reader.Load("document.pdf")
fmt.Println("Pages:", r.PageCount())
page, _ := r.Page(0)
text, _ := page.ExtractText()

// Merge
r1, _ := reader.Load("doc1.pdf")
r2, _ := reader.Load("doc2.pdf")
m, _ := reader.Merge(r1, r2)
m.SaveTo("merged.pdf")
```

---

## Redaction

Permanently remove sensitive text from PDFs — not just a visual overlay,
but actual removal of text operators from content streams.

```go
// By text search
m, _ := reader.RedactText(r, []string{"John Doe", "555-12-3456"}, nil)
m.SaveTo("redacted.pdf")

// By regex (e.g. SSNs)
re := regexp.MustCompile(`\d{3}-\d{2}-\d{4}`)
m, _ := reader.RedactPattern(r, re, &reader.RedactOptions{
    OverlayText: "REDACTED",
    StripMetadata: true,
})
m.SaveTo("redacted.pdf")
```

Character-level precision — partial words within a line are removed without
affecting adjacent text. See [`examples/redact/`](examples/redact/) for a full demo.

---

## Page Import

Load existing PDFs as templates and add dynamic content on top — the standard
workflow for invoices, receipts, certificates, and letterheads.

```go
r, _ := reader.Load("template.pdf")
imp, _ := reader.ExtractPageImport(r, 0)

doc := document.NewDocument(document.PageSizeLetter)
p := doc.AddPage()
p.ImportPage(imp.ContentStream, imp.Resources, imp.Width, imp.Height)
p.AddText("Invoice #1042", font.HelveticaBold, 14, 72, 700)
doc.Save("filled.pdf")
```

All resources (fonts, images, color spaces) are fully resolved and
self-contained — works with PDFs from any source.
See [`examples/import-page/`](examples/import-page/) for a receipt-filling demo.

---

## Headers, Footers, Watermarks

```go
doc.SetFooter(func(ctx document.PageContext, page *document.Page) {
    text := fmt.Sprintf("Page %d of %d", ctx.PageIndex+1, ctx.TotalPages)
    page.AddText(text, font.Helvetica, 9, 280, 30)
})

doc.SetWatermarkConfig(document.WatermarkConfig{
    Text:     "DRAFT",
    FontSize: 72,
    Opacity:  0.15,
    Angle:    45,
})
```

---

## Standards and Compliance

```go
doc.SetTagged(true)   // PDF/UA — screen readers, text extraction

doc.SetPdfA(document.PdfAConfig{Level: document.PdfA2B}) // archival

doc.SetAutoBookmarks(true) // auto-generate from headings

doc.SetPageLabels(
    document.PageLabelRange{PageIndex: 0, Style: document.LabelRomanLower},
    document.PageLabelRange{PageIndex: 4, Style: document.LabelDecimal},
)
```

---

## Colors

```go
layout.ColorRed               // 16 named colors
layout.RGB(0.2, 0.4, 0.8)    // RGB
layout.CMYK(1, 0, 0, 0)      // CMYK for print
layout.Hex("#FF8800")         // hex string
layout.Gray(0.5)              // grayscale
```

---

## CLI

```bash
go install github.com/carlos7ags/folio/cmd/folio@latest

folio merge -o combined.pdf doc1.pdf doc2.pdf
folio info document.pdf
folio text document.pdf
folio blank -o empty.pdf -size a4 -pages 5
```

---

## C Shared Library

Folio exports a C ABI (`libfolio.so` / `.dylib` / `.dll`) with 372 functions,
usable from Python, Ruby, C#, Java, or any language with FFI support.

```bash
CGO_ENABLED=1 go build -buildmode=c-shared -o libfolio.so ./export/
```

```c
#include "folio.h"

uint64_t doc = folio_document_new(595.28, 841.89);
uint64_t page = folio_document_add_page(doc);
folio_page_add_text(page, "Hello from C", folio_font_helvetica(), 24, 72, 750);
folio_document_save(doc, "hello.pdf");
folio_document_free(doc);
```

Pre-built binaries for Linux, macOS, and Windows are attached to each
[GitHub release](https://github.com/carlos7ags/folio/releases).

---

## Performance

All benchmarks run the full pipeline: HTML parsing, CSS cascade, layout,
and PDF serialization. Each document uses real styling: grid, flexbox,
border-radius, alternating rows, and page breaks. No headless browser,
no external process.

Benchmarks on Apple M1 Pro (`go test -bench`):

| Benchmark | What it generates | Time |
|-----------|-------------------|------|
| HTMLInvoice | Styled invoice: CSS Grid, flexbox, cards, 3-row table | 1.1 ms |
| HTMLReport | 2-page quarterly report: KPI cards, 3 tables, page break | 7.9 ms |
| HTMLTableHeavy100 | 100-row, 5-column data table with alternating rows | 11.3 ms |
| BlankPage | Empty page + PDF serialization | 5.4 µs |
| SingleParagraph | One paragraph, end-to-end | 130 µs |
| Table10x3 | 10-row table via layout API | 400 µs |

Reproduce locally:

```bash
go test -run='^$' -bench=. -benchmem ./document/
```

### Output size

The writer can emit cross-reference streams (ISO 32000-1 §7.5.8) and pack
eligible indirect objects into compressed object streams (ISO 32000-1
§7.5.7). Both modes are opt-in via `document.WriteOptions`:

```go
doc.SaveWithOptions("out.pdf", document.WriteOptions{
    UseXRefStream:    true,
    UseObjectStreams: true,
})
```

Existing callers of `Save`, `WriteTo`, and `ToBytes` see byte-identical
output. The savings depend on the document shape — content streams are
already Flate-compressed and ineligible for object stream packing, so
text-heavy documents save less than metadata-heavy ones:

| Fixture | Default | Optimized | Saved |
|---|---:|---:|---:|
| Text-heavy report (25 sections) | 6913 B | 5621 B | 18.7% |
| 50 empty pages | 6317 B | 932 B | 85.2% |
| 60-row data table | 8063 B | 7785 B | 3.4% |

Run [`examples/optimize`](examples/optimize/) to reproduce these numbers.

---

## Architecture

```
Element.PlanLayout(area) -> LayoutPlan (immutable)
PlacedBlock.Draw(ctx, x, y) -> PDF operators
```

- **No mutation** during layout — elements can be laid out multiple times safely
- **Content splitting** across pages via overflow elements
- **Intrinsic sizing** via MinWidth/MaxWidth for auto-column tables
- **Deterministic output** — byte-for-byte reproducible PDFs
- **Two external dependencies** — `golang.org/x/image` and `golang.org/x/net`

---

## Package Structure

```
folio/
  core/       PDF object model
  content/    Content stream builder
  document/   Document API (pages, outlines, PDF/A, watermarks, page import)
  font/       Standard 14 + TrueType embedding + subsetting
  image/      JPEG, PNG, TIFF
  layout/     Layout engine (all elements + rendering)
  barcode/    Code128, QR, EAN-13
  forms/      AcroForms (text, checkbox, radio, dropdown, signature)
  html/       HTML + CSS to PDF conversion
  svg/        SVG to PDF rendering
  sign/       Digital signatures (PAdES, CMS, timestamps)
  reader/     PDF parser, text extraction, merge, redaction, page import
  export/     C shared library (372 exported functions)
  cmd/folio/  CLI tool
```

---

## Examples

Each [`examples/`](examples/) subdirectory is a self-contained `go run` demo:

| Example | What it shows |
|---|---|
| [`hello`](examples/hello/) | Minimal one-page PDF |
| [`fonts`](examples/fonts/) | Standard, custom, and Unicode fonts (CJK, Cyrillic) |
| [`links`](examples/links/) | Hyperlinks, bookmarks, internal navigation |
| [`forms`](examples/forms/) | Interactive AcroForm fields |
| [`html-to-pdf`](examples/html-to-pdf/) | Rich HTML+CSS report with flexbox and tables |
| [`import-page`](examples/import-page/) | Load existing PDF as template, fill in data |
| [`merge`](examples/merge/) | Parse, merge, and extract text |
| [`optimize`](examples/optimize/) | Cross-reference and object stream output, side-by-side size comparison |
| [`redact`](examples/redact/) | Permanently remove sensitive text |
| [`report`](examples/report/) | Multi-page report with layout API |
| [`sign`](examples/sign/) | PAdES digital signature |
| [`zugferd`](examples/zugferd/) | PDF/A-3B invoice with Factur-X XML |

---

## Roadmap

- [ ] Template library — invoice, report, certificate, resume
- [ ] Hosted cloud API — POST HTML, get PDF
- [ ] Java SDK via Panama FFI
- [ ] .NET SDK via P/Invoke

---

## Contributing

Contributions welcome. Please open an issue before submitting large PRs.

```bash
git clone https://github.com/carlos7ags/folio
cd folio
go test ./...
```

---

## License

Apache License 2.0 — see [LICENSE](LICENSE).
