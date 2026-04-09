# Stress Tests

Layout and rendering stress tests for the Folio PDF engine. The
original four files (columns, flexbox, grid, svg) were contributed by
[David Richardson (@enquora)](https://github.com/enquora) in
[#126](https://github.com/carlos7ags/folio/issues/126). Additional
files were added in [#152](https://github.com/carlos7ags/folio/issues/152)
to cover tables, RTL text, pagination, nested containers, inline flow,
CSS cascade, images, and print CSS.

## Files

| File | Exercises |
|---|---|
| `cascade.html` | `!important` at all four cascade tiers, CSS custom properties with `var()` fallbacks, specificity (ID vs class vs type), nested var() |
| `columns.html` | `column-count`, `column-width`, `column-rule`, `column-span: all`, `break-inside: avoid`, `break-before: column`, nested blocks |
| `flexbox.html` | `flex-direction`, `flex-wrap`, `justify-content`, `align-items`, `align-self`, `order`, `flex-grow/shrink/basis`, `gap`, CSS custom properties |
| `grid.html` | `grid-template-columns/rows`, `grid-template-areas`, `grid-column/row` placement, `auto-flow: dense`, `minmax()`, `repeat()`, nested grids, `justify-items`, `align-items` |
| `images.html` | `object-fit` (all 5 modes), `aspect-ratio`, data-URI PNG/JPEG, SVG `<image>` with raster, `linear-gradient` background, `background-size` |
| `inline.html` | Inline `<img>`, inline `<svg>`, `display: inline-block`, `<sub>`/`<sup>`, `<br>` inside inline tags, `<mark>` highlight |
| `longform.html` | 50+ paragraphs, headings, lists, tables, `page-break-before/inside`, `orphans`/`widows`, `@page` margins, running headers |
| `nested.html` | Flex-in-grid, grid-in-flex, columns-in-flex, table-in-grid-cell, 5-level nesting with distinct borders/backgrounds |
| `print.html` | `@page` custom margins, `@page :first`, `@top-center`/`@bottom-right` margin boxes, multi-page content |
| `rtl.html` | Hebrew, Arabic (connected script), Farsi, mixed bidi, bracket mirroring, `dir` attribute, CSS `direction:rtl` on table |
| `svg.html` | Primitives, `<path>` grammar, `<text>` with anchors/tspan, `<image>` with data-URI PNG, `linearGradient`/`radialGradient`, inline `<svg>` in paragraph flow |
| `tables.html` | Multi-page tables, `<thead>` repeat, `colspan`/`rowspan`, `border-collapse` vs `separate`, percentage-width cells in flex, nested tables |

## Running

Render with the Folio CLI or the example harness:

```sh
go run ./examples/html-to-pdf examples/stress-tests/tables.html
```

Or render all:

```sh
for f in examples/stress-tests/*.html; do
  go run ./examples/html-to-pdf "$f"
done
```

Compare the output PDFs against browser renderings to spot gaps.

## Known gaps

- **Column fill order** (#145): Folio distributes children round-robin
  across columns instead of filling sequentially. Affects
  `columns.html` test 6.
- **RTL text** requires the RTL branch (PR #148) to be merged for
  Arabic shaping and bidi reordering to work correctly in `rtl.html`.
