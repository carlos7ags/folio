---
name: using-folio
description: How to generate PDFs with the folio Go library (HTML to PDF). Use when writing Go code that calls github.com/carlos7ags/folio to render HTML/CSS into a PDF, configure pages/fonts/PDF-A, or when a generated PDF is missing content (page numbers, headers/footers, fixed/absolute elements). Teaches the canonical AddHTML path and how to verify output.
---

# Using folio

folio is a pure-Go HTML-to-PDF library. This skill captures the stable usage
patterns that are easy to get wrong. It deliberately avoids enumerating which
CSS properties are supported (that list changes release to release) — for the
current feature surface, read `examples/` in the repo and the package docs.

## The one rule: use `Document.AddHTML`, not `html.ConvertFull`

`AddHTML` is the canonical entry point. It wires *everything* the converter
produces into the document:

- normal-flow elements
- **absolutely/fixed-positioned elements** (`position: absolute` / `fixed`)
- page geometry and margins from `@page` rules
- margin boxes for the base, `:first`, `:left`, and `:right` page slots
  (e.g. `@bottom-center { content: counter(page) }` page numbers)
- document metadata from `<title>` and `<meta>`

```go
import "github.com/carlos7ags/folio/document"

doc := document.NewDocument(document.PageSizeA4)
if err := doc.AddHTML(htmlString, nil); err != nil {
    return err
}
if err := doc.Save("out.pdf"); err != nil {
    return err
}
```

`opts` may be nil. `AddHTMLWithContext` is the deadline-aware variant.
`AddHTMLTemplate(tmpl, data, opts)` runs an `html/template` first (invoices,
reports).

### Why this matters

`html.ConvertFull(html, opts)` returns a raw `*ConvertResult`. If you feed it
into a document by hand, it is very easy to forward only `result.Elements` and
silently drop `result.Absolutes`, `result.PageConfig`, and the margin-box maps.
The symptom is a PDF that renders fine but is missing page numbers,
headers/footers, or every `position: fixed`/`absolute` element — with no error.
This is the single most common folio mistake.

**Only reach for `ConvertFull` when you genuinely need the raw result** (e.g.
to inspect or adjust elements before they reach the document). When you do, add
it back with `Document.AddConvertResult` — never wire `result.Elements` by hand:

```go
result, err := html.ConvertFull(htmlString, nil)
if err != nil { return err }

// ... inspect or modify result here ...

doc := document.NewDocument(document.PageSizeA4)
if err := doc.AddConvertResult(result); err != nil { return err }
```

`AddConvertResult` forwards everything (elements, absolutes, `@page`
geometry/margins, margin boxes, metadata) and sets the page size from any
`@page` rule, so you don't pre-resolve geometry. `AddHTML` is exactly
`ConvertFull` + `AddConvertResult`. Use `SetPageSize`/`PageSize()` if you need
to read or override the size afterward.

## Options worth knowing (`*html.Options`)

All optional; the zero value (nil) is "sensible defaults, all local assets must
be inlined as data: URIs".

- `BaseFS fs.FS` — resolves every local path (images, fonts, linked CSS,
  `background-image: url(...)`). Use `os.DirFS(dir)`, `embed.FS`, or
  `fstest.MapFS`. Without it, any local-asset reference fails.
- `FallbackFontPath string` — a Unicode TTF/OTF for glyphs outside
  WinAnsiEncoding (CJK, emoji, many accented scripts). Resolved via `BaseFS`
  when set.
- `StrictAssets bool` — turn asset-load failures (missing fonts, broken image
  paths, unreadable stylesheets) into a returned error instead of warn-and-
  continue. Turn it **on in dev/CI** to catch broken paths; leave off in prod.
- `Logger *slog.Logger` — receives warn events for failed assets when
  `StrictAssets` is off.
- `PageWidth` / `PageHeight`, `DefaultFontSize`, `URLPolicy`, `Client`,
  `MaxElements`.

## Fonts

- The standard PDF fonts (Helvetica, Times, Courier) need no setup but only
  cover WinAnsi. Anything else needs an embedded font.
- Embed via CSS `@font-face { font-family: 'X'; src: url('fonts/X.ttf') }` with
  `BaseFS` pointing at the asset root, then use `font-family: 'X'`.
- For stray out-of-range characters in otherwise-Latin text, set
  `Options.FallbackFontPath` rather than restyling everything.
- See `examples/fonts`, `examples/cjk`, `examples/indic`, `examples/rtl`.

## PDF/A (archival)

Set a config before saving; folio adds the XMP metadata, output intent, and
`/ID`, and validates conformance at write time:

```go
doc.SetPdfA(document.PdfAConfig{Level: document.PdfA2B})
```

Levels include `PdfA1B`, `PdfA2B`/`PdfA2U`/`PdfA2A`, `PdfA3B` (only level that
permits file attachments), and the `PdfA4*` family. **PDF/A requires all fonts
embedded** — the standard-14 fonts do not satisfy it, so supply real fonts via
`@font-face`/`FallbackFontPath`. For ZUGFeRD/Factur-X invoices (PDF/A-3B +
attachment + XMP schema), see `examples/zugferd`.

## Verifying output

A folio PDF that "renders" can still be wrong (dropped content, square corners
where rounding was intended, restarted list numbering, missing page numbers).
Verify with Poppler tools rather than trusting that no error means correct:

```sh
pdfinfo out.pdf                       # page count, size
pdftotext -layout out.pdf -           # text + positions; grep for expected content
pdftotext -layout -f 2 -l 2 out.pdf - # just page 2 — catch content lost at a page break
pdftoppm -png -r 200 out.pdf page     # rasterize to inspect visually (bump -r for detail)
```

To distinguish a *rounded* corner (Bézier `c` operators) from a *square* one
(`re` rectangle) that overpaints it, decompress the content stream and inspect
the operators:

```sh
python3 -c "import sys,zlib,re; d=open('out.pdf','rb').read(); \
print('\n'.join(s.decode('latin1') for s in re.findall(rb'stream\r?\n(.*?)\r?\nendstream', d, re.S) \
for s in [zlib.decompress(s)] ))" | grep -E ' re$| c$' | sort | uniq -c
```

When fixing a rendering bug, render **before** (current `main`) and **after**
(your branch) of the same minimal HTML and diff the two — structural unit tests
alone miss visual regressions.

## Pointers

- `examples/` — runnable demos: `hello`, `html-to-pdf`, `invoice`, `report`,
  `template`, `forms`, `links`, `merge`, `redact`, `optimize`, plus the font/
  script examples above. Each is `go run ./examples/<name>`.
- Page sizes: `document.PageSizeLetter`, `PageSizeA4`, `PageSizeLegal`,
  `PageSizeTabloid`, …; `.Landscape()` swaps width/height.
- Output: `doc.Save(path)`, `doc.WriteTo(w)`, and the `*WithOptions` variants
  for compression/linearization settings.
