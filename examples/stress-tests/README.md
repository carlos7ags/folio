# Stress Tests

Layout and rendering stress tests contributed by
[David Richardson (@enquora)](https://github.com/enquora) in
[#126](https://github.com/carlos7ags/folio/issues/126). Each file
renders faithfully in Gecko, Blink, and WebKit and exercises a specific
CSS layout mode end-to-end.

## Files

| File | Exercises |
|---|---|
| `columns.html` | `column-count`, `column-width`, `column-rule`, `column-span: all`, `break-inside: avoid`, `break-before: column`, nested blocks |
| `flexbox.html` | `flex-direction`, `flex-wrap`, `justify-content`, `align-items`, `align-self`, `order`, `flex-grow/shrink/basis`, `gap`, CSS custom properties |
| `grid.html` | `grid-template-columns/rows`, `grid-template-areas`, `grid-column/row` placement, `auto-flow: dense`, `minmax()`, `repeat()`, nested grids, `justify-items`, `align-items` |
| `svg.html` | Primitives (rect, circle, ellipse, line, polyline, polygon), `<path>` grammar, `<text>` with anchors/tspan, `<image>` with data-URI PNG, `linearGradient`/`radialGradient`, inline `<svg>` in paragraph flow |

## Running

Render with the Folio CLI or the example harness:

```sh
go run ./examples/html-to-pdf examples/stress-tests/columns.html
```

Or render all four:

```sh
for f in examples/stress-tests/*.html; do
  go run ./examples/html-to-pdf "$f"
done
```

Compare the output PDFs against browser renderings to spot gaps.

## Known gaps

All rendering issues identified in the original #126 audit have been
fixed (#127, #128, #129, #130) except for one behavioral divergence:

- **Column fill order** (#145): Folio distributes children round-robin
  across columns instead of filling sequentially. This affects
  `columns.html` test 6.
