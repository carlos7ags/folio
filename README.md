# Folio

A modern PDF library for Go.

Generate, read, merge, sign, and manipulate PDF documents with a clean, idiomatic API.

[![Go Reference](https://pkg.go.dev/badge/github.com/carlos7ags/folio.svg)](https://pkg.go.dev/github.com/carlos7ags/folio)
[![CI](https://github.com/carlos7ags/folio/actions/workflows/ci.yml/badge.svg)](https://github.com/carlos7ags/folio/actions)

## Install

```bash
go get github.com/carlos7ags/folio
```

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
        "This PDF was generated with Folio, a modern PDF library for Go.",
        font.Helvetica, 12,
    ))

    doc.Save("hello.pdf")
}
```

## PDF Generation

Create documents with a high-level layout engine that handles word wrapping,
page breaks, and content splitting automatically.

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

## Layout Elements

| Element | Description |
|---------|-------------|
| `Paragraph` | Word-wrapped text with alignment, leading, orphans/widows |
| `Heading` | H1-H6 with preset sizes, spacing, and auto-bookmarks |
| `Table` | Borders, colspan, rowspan, header repetition, auto-column widths |
| `List` | Bullet, numbered, Roman, alpha, nested |
| `Div` | Generic container with borders, background, padding |
| `Image` | JPEG, PNG, TIFF with aspect ratio preservation |
| `LineSeparator` | Horizontal rule (solid, dashed, dotted) |
| `TabbedLine` | Tab stops with dot leaders (for TOCs) |
| `Link` | Clickable text with URL or internal destination |
| `Float` | Left/right floating with text wrap |
| `Flex` | Flexbox layout with direction, wrap, alignment |
| `Columns` | Multi-column layout with balancing |
| `AreaBreak` | Explicit page break |
| `BarcodeElement` | Code128, QR, EAN-13 inline in layout |

## Styled Text

```go
p := layout.NewStyledParagraph(
    layout.Run("Normal text ", font.Helvetica, 12),
    layout.Run("bold text ", font.HelveticaBold, 12),
    layout.Run("colored text", font.Helvetica, 12).
        WithColor(layout.ColorRed).
        WithUnderline(),
)
doc.Add(p)
```

## Containers and Flexbox

```go
box := layout.NewDiv().
    SetPadding(12).
    SetBorder(layout.SolidBorder(1, layout.ColorBlack)).
    SetBackground(layout.ColorLightGray).
    Add(layout.NewHeading("Notice", layout.H2)).
    Add(layout.NewParagraph("Important information.", font.Helvetica, 12))
doc.Add(box)
```

## Tables

```go
tbl := layout.NewTable().SetAutoColumnWidths()

// Or explicit widths with units
tbl.SetColumnUnitWidths([]layout.UnitValue{layout.Pct(30), layout.Pct(70)})

// Header rows repeat on page breaks
h := tbl.AddHeaderRow()
h.AddCell("Name", font.HelveticaBold, 10)
h.AddCell("Value", font.HelveticaBold, 10)

// Borders: solid, dashed, dotted, double
cell := r.AddCell("Styled", font.Helvetica, 10)
cell.SetBorders(layout.AllBorders(layout.DashedBorder(1, layout.ColorBlue)))
cell.SetBackground(layout.ColorLightGray)
cell.SetVAlign(layout.VAlignMiddle)
```

## HTML to PDF

```go
import "github.com/carlos7ags/folio/html"

elems, _ := html.Convert(`
    <h1>Invoice #1042</h1>
    <p>Bill to: <strong>Acme Corp</strong></p>
    <table border="1">
        <tr><th>Item</th><th>Price</th></tr>
        <tr><td>Widget</td><td>$50</td></tr>
    </table>
    <ul>
        <li>Payment due in 30 days</li>
    </ul>
`, nil)

doc := document.NewDocument(document.PageSizeLetter)
for _, e := range elems {
    doc.Add(e)
}
doc.Save("invoice.pdf")
```

Supports 40+ HTML elements, inline and `<style>` block CSS, named/hex/rgb colors,
flexbox, forms, images, and tables with colspan.

## Barcodes

```go
import "github.com/carlos7ags/folio/barcode"

qr, _ := barcode.QR("https://example.com")
doc.Add(layout.NewBarcodeElement(qr, 100).SetAlign(layout.AlignCenter))

bc, _ := barcode.Code128("SHIP-2024-001")
doc.Add(layout.NewBarcodeElement(bc, 200))

ean, _ := barcode.EAN13("590123412345")
doc.Add(layout.NewBarcodeElement(ean, 150))
```

## Interactive Forms

```go
import "github.com/carlos7ags/folio/forms"

form := forms.NewAcroForm()
form.Add(forms.TextField("name", [4]float64{72, 700, 300, 720}, 0))
form.Add(forms.Checkbox("agree", [4]float64{72, 670, 92, 690}, 0, false))
form.Add(forms.Dropdown("role", [4]float64{72, 640, 250, 660}, 0,
    []string{"Developer", "Designer", "Manager"}))

doc.SetAcroForm(form)
doc.Save("form.pdf")
```

## Digital Signatures

```go
import "github.com/carlos7ags/folio/sign"

signed, _ := sign.SignPDF(pdfBytes, sign.Options{
    Certificate: cert,
    PrivateKey:  key,
    Level:       sign.PAdES_B_LT,
    Reason:      "Approved",
    Location:    "New York",
})
os.WriteFile("signed.pdf", signed, 0644)
```

Supports PAdES B-B, B-T (timestamped), and B-LT (long-term validation with
embedded OCSP responses and CRLs). Uses Go stdlib crypto.

## Reading PDFs

```go
import "github.com/carlos7ags/folio/reader"

r, _ := reader.Open("document.pdf")

fmt.Println("Pages:", r.PageCount())
title, author, _, _, _ := r.Info()
fmt.Println("Title:", title)

page, _ := r.Page(0)
fmt.Println("Size:", page.Width, "x", page.Height)

text, _ := page.ExtractText()
fmt.Println(text)
```

## Merging PDFs

```go
r1, _ := reader.Open("doc1.pdf")
r2, _ := reader.Open("doc2.pdf")

m, _ := reader.Merge(r1, r2)
m.SetInfo("Combined Report", "Folio")
m.SaveTo("merged.pdf")
```

## Colors

```go
layout.ColorRed                // 16 named colors
layout.RGB(0.2, 0.4, 0.8)     // RGB
layout.CMYK(1, 0, 0, 0)       // CMYK for print
layout.Hex("#FF8800")          // hex string
layout.Gray(0.5)               // grayscale
```

## Headers, Footers, and Watermarks

```go
doc.SetFooter(func(ctx document.PageContext, page *document.Page) {
    text := fmt.Sprintf("Page %d of %d", ctx.PageIndex+1, ctx.TotalPages)
    page.AddText(text, font.Helvetica, 9, 280, 30)
})

doc.SetWatermark(document.WatermarkConfig{
    Text:     "DRAFT",
    FontSize: 72,
    Opacity:  0.15,
    Rotation: 45,
})
```

## Standards and Accessibility

```go
// Tagged PDF (screen readers, text extraction)
doc.SetTagged(true)

// PDF/A for archival (A-2b, A-2u, A-2a, A-3b)
doc.SetPdfA(document.PdfAConfig{Level: document.PdfA2B})

// Auto-generate bookmarks from headings
doc.SetAutoBookmarks(true)

// Viewer preferences
doc.SetViewerPreferences(document.ViewerPreferences{
    FitWindow:       true,
    DisplayDocTitle: true,
    PageLayout:      document.LayoutSinglePage,
})

// Page labels (Roman numerals for front matter)
doc.SetPageLabels(
    document.PageLabelRange{PageIndex: 0, Style: document.LabelRomanLower},
    document.PageLabelRange{PageIndex: 4, Style: document.LabelDecimal},
)
```

## Page Geometry

```go
// 14 standard sizes + landscape
document.PageSizeA4
document.PageSizeLetter
document.PageSizeA3.Landscape()

// Per-page size override
page := doc.AddPage()
page.SetSize(document.PageSizeLegal)

// Page boxes for print production
page.SetCropBox([4]float64{36, 36, 576, 756})
page.SetTrimBox([4]float64{36, 36, 576, 756})
page.SetBleedBox([4]float64{18, 18, 594, 774})
```

## CLI

```bash
go install github.com/carlos7ags/folio/cmd/folio@latest

folio merge -o combined.pdf doc1.pdf doc2.pdf
folio info document.pdf
folio pages document.pdf
folio text document.pdf
folio create -o hello.pdf -title "Hello" -text "World"
folio blank -o empty.pdf -size a4 -pages 5
```

## Architecture

Folio uses a plan-based layout engine where layout is a pure function:

```
Element.PlanLayout(area) -> LayoutPlan (immutable)
PlacedBlock.Draw(ctx, x, y) -> PDF operators
```

- **No mutation** during layout. Elements can be laid out multiple times safely.
- **Content splitting** across pages via overflow elements.
- **Intrinsic sizing** via MinWidth/MaxWidth for auto-column tables.
- **Deterministic output**. Ordered dictionaries produce byte-for-byte reproducible PDFs.

## Packages

```
folio/
  core/        PDF object model
  content/     Content stream builder
  document/    Document API (pages, outlines, PDF/A, watermarks)
  font/        Standard 14 + TrueType embedding + subsetting
  image/       JPEG, PNG, TIFF
  layout/      Layout engine (all elements + flexbox + rendering)
  barcode/     Code128, QR, EAN-13
  forms/       AcroForms (text, checkbox, radio, dropdown, signature)
  html/        HTML + CSS to PDF conversion
  sign/        Digital signatures (PAdES, CMS, timestamps)
  reader/      PDF parser (read, extract, merge)
  cmd/folio/   CLI tool
```

## License

Apache License 2.0. See [LICENSE](LICENSE).
