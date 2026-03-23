# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.4.2] - 2026-03-22

### Added
- **Clickable links in PDFs** — `<a href="...">` inside paragraphs, headings, and list items now produce PDF link annotations (#23, #26, #27)
- **Multiple links per line** — paragraphs with several inline links each get their own precise annotation rectangle
- **Internal document links** — `layout.NewInternalLink` resolves to direct page references for macOS Preview compatibility
- **Layout API link support** — `TextRun.WithLinkURI()` and `WithDecoration()` for building linked text programmatically; `List.AddItemRuns()` for linked list items
- **Links example** (`examples/links/`) showcasing external, inline, multi-line, styled, heading, and list item links plus bookmarks and internal navigation
- **Fonts example** (`examples/fonts/`) demonstrating custom `@font-face` with Unicode (CJK, Cyrillic, Japanese)

### Fixed
- **Custom `@font-face` family names ignored** — `parseFontFamily` was mapping all names to standard fonts; now preserves custom names for embedded font matching (#16)
- **`page-break-after` ignored when body has `width: 100%`** — `AreaBreak` elements trapped inside Div wrappers are now hoisted out so the renderer can act on them (#21)
- **CSS class selectors case-insensitive** — `.myClass` now matches `class="myClass"` regardless of case (#28)
- **Punctuation spacing at run boundaries** — period/comma after a styled span (e.g. `<b>word</b>.`) no longer gets an extra inter-word space (#25)
- **Underline continuous across multi-word links** — decoration extends through trailing spaces between consecutive linked/decorated words
- **`@font-face` family name case mismatch** — font-face names are now lowercased consistently so CSS lookup matches

### Changed
- **Split `html/converter.go`** into 11 focused files by responsibility (paragraph, table, block, flex, forms, image, list, heading, link, style, helpers) — no behavior changes (#34)

## [0.4.1] - 2026-03-22

### Added
- **Comprehensive GoDoc comments** across all 14 packages — every exported and unexported symbol now has an accurate doc comment following Go conventions
- **Package-level doc comments** added to `layout`, `html`, `svg`, and consolidated in `core` (had two conflicting comments)
- **`ARCHITECTURE.md`** documenting design principles, package responsibilities, layering rules, dependency policy, and non-goals
- **Examples directory** with hello-world sample

### Fixed
- Stale/inaccurate doc comments: watermark "prepends" → "appends", `dss.Build` return description, `PdfObjects` → `BuildObjects`, merged interface docs in layout, and others
- `cmd/folio printUsage` referenced nonexistent "region" extraction strategy
- `svg/doc.go` listed nonexistent `clipPath` support, missing `radialGradient`
- `layout/doc.go` referenced wrong type names (`Tab` → `TabbedLine`, nonexistent `Transform` element)
- `font/standard.go` package doc was stale ("and later font parsing" — parsing is fully implemented)

### Changed
- Removed committed `folio.wasm` binary (7.2MB) from the repository; the release workflow already builds it fresh per tagged version
- Added `*.wasm` to `.gitignore`
- golangci-lint issues fixed and linter added to CI
- Apache 2.0 license replaced with verbatim text; missing license headers added

## [0.4.0] - 2026-03-19

### Added
- **WOFF1 font decoding** — `@font-face` now supports `.woff` files via automatic format detection (`font.LoadFont`)
- **CSS custom properties (variables)** — `--name: value` declarations with `var(--name, fallback)` resolution, inheritance, and nesting
- **CSS counters** — `counter-reset`, `counter-increment`, `counter()` and `counters()` in `::before`/`::after` content
- **CSS `clear` property** — `clear: left/right/both` advances past active floats before placing elements
- **CSS `border-spacing`** — horizontal and vertical cell spacing for tables in separate border model
- **HTTP background images** — `background-image: url('https://...')` fetches remote images in non-WASM builds
- **Inline-block in text flow** — `display: inline-block` elements flow within paragraphs as "big words" with correct line-breaking and height expansion
- **Containing-block absolute positioning** — `position: absolute` resolves against nearest positioned ancestor via overlay children, not just the page
- **Liang-Knuth hyphenation** — 4938 TeX US English patterns for linguistically correct syllable breaks, replacing geometric character splitting
- **C ABI export layer** — `export/` package for FFI from Python, Ruby, Swift, etc.
- **Full sRGB ICC profile** and PDF/A-1b compliance support
- **QR code v1-40** with numeric/alphanumeric encoding modes
- **Symbol and ZapfDingbats** font width tables

### Changed
- `font.LoadTTF` calls in HTML converter replaced with `font.LoadFont` (auto-detects TTF/OTF/WOFF)
- `hyphenateWord()` uses pattern-based breaks first, falls back to character splitting

## [0.3.0] - 2026-03-18

### Added
- **CSS Grid layout** — `display: grid` with `grid-template-columns`, `grid-template-rows`, `grid-template-areas`, named areas, `auto-rows`, alignment (`justify-items`, `align-items`), and page break support
- **Absolute positioning with z-index** — `position: absolute` elements ordered by `z-index`
- **`margin-left: auto` right-alignment** for inline-block SVGs in flex containers

### Fixed
- Flex width double-resolution when both `widthUnit` and percentage were set
- Inline-block SVGs disappearing due to missing width propagation

## [0.2.0] - 2026-03-17

### Added
- **Auto-height pages** via CSS `@page { size: 80mm 0; }` — page height sizes to content (receipts, flyers)
- **Negative margin support** for flex column children — enables CSS patterns like `margin: -10px -14px` to break out of parent padding
- **`margin-left: auto`** on flex row items — pushes items to the right edge (e.g., seat box alignment)
- **`margin-top: auto`** on flex column items — pushes items to the bottom (e.g., footer positioning)
- **Cross-axis stretch** (W3C Flexbox §9.4) — flex row items stretch to match tallest sibling, with or without definite container height
- **Flex column `flex-grow`** — items with `flex: 1` now grow to fill remaining space in column direction
- **Flex column `justify-content`** — space-between, center, flex-end, space-around, space-evenly in column direction
- **`hasDefiniteCrossSize` flag** on Flex — enables stretch when Flex is wrapped in a height-constrained Div
- **Watermark support** in WASM render API via `watermark` parameter
- **Automatic Unicode font embedding** for non-WinAnsi characters (CIDFont with embedded cmap)
- **CIDFont fallback decoding** from embedded font cmap tables
- **Font caching** for repeated font resolution
- **Form XObject resolution** in PDF reader
- **Tagged PDF extraction** improvements
- **Full text matrix tracking** and font-aware space detection in reader
- **Xref cycle detection**, hybrid xref support, stream length correction
- **SVG enhancements**: text-anchor, tspan, defs/use, gradient support

### Fixed
- **Percentage heights** now resolve against parent container's explicit height, not the page — fixes vertical bar charts overflowing their containers
- **`box-sizing: border-box`** no longer double-subtracts padding from width/height — only border is subtracted since the Div handles padding internally
- **Double-padding on wrapped flex containers** — when a Flex has CSS width/height, visual properties (padding, borders, margins) are cleared from the Flex and applied only to the wrapper Div
- **`letter-spacing` in width measurement** — `Paragraph.MinWidth()` and `MaxWidth()` now include letter-spacing, preventing flex items from being measured too narrow
- **Floating-point overflow in `margin-top: auto`** — added 0.01pt epsilon tolerance to prevent items from silently overflowing due to float rounding
- **`margin-top: auto` phase consistency** — `neededBelow` calculation now includes `marginBottom` of subsequent items in both phase 1 and phase 3
- **SpaceBefore/SpaceAfter doubling** on flex items — element margins are cleared when FlexItem margins take over (Div, Flex, and Paragraph)
- **Background preserved on wrapped Flex** — `min-height` backgrounds now fill the full height (kept on both Div wrapper and inner Flex)
- **`parseFloat` negative numbers** — CSS parser now correctly handles negative values like `-10px`
- **Flex children splitting** into separate items instead of grouping per HTML child
- **SVG shapes invisible** and text mirrored
- **Sequential elements overlapping** by tracking cumulative Y offset in renderer
- **`<br>` tags in paragraphs** and CSS width as flex-basis
- **WASM binary size** halved by excluding `net/http` from js builds

### Changed
- Cross-axis stretch now fires for all flex row items (not just when container has definite height)
- `planColumn` refactored into 3-phase layout: measure, grow, position

## [0.1.1] - 2026-03-16

### Added
- CLI `extract` command with pluggable strategies (simple, location)
- CLI `sign` command for PAdES digital signatures
- Table `SetBorderCollapse(true)` for CSS-style collapsed borders
- CSS `calc()` support in HTML-to-PDF (e.g., `width: calc(100% - 40px)`)
- CSS `@page` rule parsing (size, margins) in HTML-to-PDF
- CSS `orphans`/`widows` properties in HTML-to-PDF
- CSS `break-before`/`break-after`/`break-inside` modern syntax
- Remote image loading (`<img src="https://...">`) in HTML-to-PDF
- Data URI image support (`<img src="data:image/png;base64,...">`)
- PDF metadata extraction from HTML `<title>` and `<meta>` tags
- Content stream processor with full graphics state (CTM, color, font)
- Pluggable text extraction strategies (Simple, Location, Region)
- Path and image extraction from content streams
- Per-glyph span extraction (opt-in)
- Text rendering mode awareness (invisible text filtering)
- Marked content tag tracking (BMC/BDC/EMC)
- Form XObject recursion in content processing
- Actual glyph widths from font metrics (replaces estimation)
- Auto-bookmarks from layout headings
- Viewer preferences (page layout, mode, UI options)
- Page labels (decimal, Roman, alpha)
- Page geometry boxes (CropBox, BleedBox, TrimBox, ArtBox)
- SVG package in README

### Changed
- CLI version bumped to 0.1.1
- README updated with extract and sign commands, border-collapse, SVG package

### Fixed
- Table border-collapse: adjacent cells no longer draw double borders
- Tables section in README had undefined variable

## [0.1.0] - 2026-03-15

### Added
- Initial release
- PDF generation with layout engine (Paragraph, Heading, Table, List, Div, Image, Float, Flex, Columns)
- PDF reader with tokenizer, parser, xref streams, object streams
- PDF merge and modify
- HTML-to-PDF conversion with CSS support
- Digital signatures (PAdES B-B, B-T, B-LT)
- Interactive forms (AcroForms)
- Barcodes (Code128, QR, EAN-13)
- Tagged PDF and PDF/A compliance
- SVG rendering
- CLI tool (merge, info, pages, text, create, blank)
- Font embedding and subsetting (TrueType)
- JPEG, PNG, TIFF image support
- Encryption (AES-256, AES-128, RC4)
