# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

### Changed (breaking)

- **`font.Face` now includes `GSUB()`, `GPOS()`, and `GIDToUnicode()`** — the optional `font.GSUBProvider` and `font.GPOSProvider` interfaces are removed, and every `Face` implementation must provide the three methods directly. A `Face` with no shaping data for a given table returns nil rather than omitting the method; `GIDToUnicode` returns nil when the font has no cmap-derived reverse map. This drops the type-assertion indirection that callers previously used to probe for shaping support (`face.(font.GSUBProvider)`, `face.(font.GPOSProvider)`) now that the seam is no longer needed pre-1.0.
- **`html` no longer fetches remote or absolute-path assets by default** — a document's `http(s)` references (`<img>`, `background-image: url()`, linked stylesheets, `@font-face url()`) are only fetched when `Options.AllowRemoteFetch` is set, and absolute filesystem paths in document content are only read when `Options.AllowAbsolutePaths` is set (both default false). When remote fetch is enabled with no `Options.URLPolicy`, the built-in `html.DenyInternalHosts` policy blocks loopback, RFC1918, link-local, and non-http(s) targets and is re-checked on every redirect hop; absolute-path reads are size-capped like HTTP reads. Set `URLPolicy` to a function returning nil to allow every host. `Options.FallbackFontPath` and the built-in system-font search are unaffected.

### Added

- **`zugferd` package** — generates Factur-X/ZUGFeRD hybrid e-invoices: a typed `Invoice` renders EN 16931 UN/CEFACT Cross-Industry Invoice (CII) XML and `Invoice.Attach` embeds it into a `document.Document` as a PDF/A-3B associated file with the matching Factur-X XMP schema, keeping the attachment filename, `AFRelationship`, and XMP conformance level in sync. Covers the MINIMUM and BASIC Factur-X 1.0 profiles, with hand-written field-presence and totals-arithmetic validation (`Invoice.Validate`) and a fixed-point `Amount` type so money never touches a float. `examples/zugferd` is ported onto it. See `docs/design-zugferd.md` for profile coverage and known limitations.
- **`html.Options.AllowRemoteFetch`** — opt-in for fetching `http(s)` assets referenced by the document. Default false.
- **`html.Options.AllowAbsolutePaths`** — opt-in for reading absolute filesystem paths named in document content when `BaseFS` is nil. Default false; reads are size-capped like the HTTP path.
- **`html.DenyInternalHosts`** — a `URLPolicy` that blocks non-http(s) schemes and targets resolving to loopback, private (RFC1918 / ULA), link-local, unspecified, or multicast addresses (plus carrier-grade NAT, and IPv4-compatible / NAT64 IPv6 forms that embed such addresses). Applied by default when `AllowRemoteFetch` is true and `URLPolicy` is nil, and re-checked on every redirect hop.
- **`html.Options.MaxTotalAssetBytes`** — caps the aggregate bytes read across every asset loaded during one conversion (images, fonts, stylesheets — remote, `BaseFS`-relative, and absolute alike). 0 selects a generous 512 MiB default; loads that cross the cap fail with `html.ErrAssetBudgetExceeded`.
- **`html.ErrRemoteFetchDisabled`** — returned when a document references an `http(s)` asset but `AllowRemoteFetch` is false. Reported normally (logged, and surfaced under `StrictAssets`).
- **`html.ErrAbsolutePathDenied`** — returned when a document references an absolute filesystem path but `AllowAbsolutePaths` is false. Reported normally.
- **`html.ErrAssetBudgetExceeded`** — returned when a conversion's aggregate asset read size crosses `Options.MaxTotalAssetBytes`. Reported normally.

### Changed

- **CSS named colors now resolve to full-precision sRGB values in both `html` and `svg` output** — the two packages each carried an independent 148-entry named-color table stored as rounded decimal literals (3 decimals in `html`, 4 in `svg`), so the same keyword could render to different bytes depending on which package handled it. Both now share one table in `internal/csscolor` that computes each component as `x/255` at full `float64` precision, which changes the emitted bytes for named colors slightly versus the old decimal literals in both packages. `svg` color parsing also now accepts the full CSS color grammar (4/8-digit hex, CSS Color 4 space-separated `rgb()`, and `hsl()`/`hsla()`) that `html` already supported — previously-valid `svg` inputs are unaffected. As part of sharing the parser, malformed `svg` color literals (invalid hex digits, non-numeric `rgb()`/alpha components) are now treated leniently — the bad component defaults to 0 rather than rejecting the whole color, matching how `html` already handled them.

## [0.9.1] - 2026-06-10

A rendering-correctness release across `border-radius` (#329), CSS paged media
(`@page`, `position: fixed`, margin boxes — #327/#328), and lists
(#330/#339/#342/#347/#358), plus a from-scratch fix of the QR barcode encoder
(#341). Adds native multi-level list numbering via CSS counters (#356), table
rowspan geometry (#357), `Document.AddConvertResult`, and
`SetPageSize`/`PageSize` (#338). No breaking changes.

### Added

- **`document.Document.AddConvertResult(*html.ConvertResult) error`** — the wiring half of `AddHTML`, exported so callers who need the raw `ConvertResult` (to inspect or transform elements before rendering) can add it back with a single correct call. It forwards everything the converter produces — normal-flow elements, absolutely/`position:fixed` elements, `@page` size and margins, all four margin-box sets, and `<title>`/`<meta>` metadata — and sets the page size from any `@page` rule. `AddHTML` is now exactly `ConvertFull` + `AddConvertResult`, so the two paths cannot diverge. This closes the discoverability trap where a hand-written `for _, e := range result.Elements { doc.Add(e) }` loop silently dropped absolutes, margin boxes, and `@page` config with no error (#338)
- **`document.Document.SetPageSize(PageSize)` and `PageSize() PageSize`** — read and override the page dimensions after `NewDocument`. The size was previously fixed at construction even though `AddHTML` mutates it internally from `@page` rules; these expose the missing setter/getter. `AddConvertResult` derives the size from `@page` automatically, so the pre-`NewDocument` geometry-resolve dance is no longer required (#338)
- **Code 128 code sets A and C** — `barcode.NewCode128` now selects sets A, B, and C automatically instead of Code B only. Set C encodes digit runs as pairs (roughly halving the width of numeric payloads) and set A encodes the ASCII control characters (0–31); bytes above 127 still return an error. Round-trip and golden-pattern tests cover every code-set transition (#346)
- **Native multi-level list numbering via CSS counters and `::marker`** — `li::marker { content: ... }` now defines the marker string, so `counter-reset` / `counter-increment` with `counter()` / `counters()` produce true nested numbering (e.g. `1`, `1.1`, `1.1.1`) instead of a per-level restart. Adds `list-style-position` (`inside` / `outside`): `inside` renders the marker inline with the first content line and wraps subsequent lines under it; `outside` (default) places it in a gutter that auto-grows to fit a wide marker so multi-level ordinals no longer overlap the item text (short-marker lists stay byte-identical). The `list-style` shorthand now scans all tokens (position / type / image). `layout.List` gains `SetMarkerInside`. New `examples/legal-numbering` renders a nested Master Services Agreement with hanging indents. Phase 1 of #356
- **Table `rowspan` vertical geometry** — a `rowspan` cell previously reserved grid occupancy (following rows skipped its columns) but was always drawn one row tall. The spanning cell now spans the full height of its rows: `buildGrid` excludes rowspanning cells from their starting row's natural height, then resolves span heights from the summed spanned-row heights plus inter-row gaps (growing the last spanned row when content needs more room), and vertical alignment centers across the full span. A rowspan straddling a page break is not yet handled (#357)

### Fixed

#### `border-radius` (#329)

- **Percentage `border-radius` renders as a true rounded corner** — `border-radius: 50%` was resolving against a zero reference and collapsing to 0 (square corners). It now resolves per-axis: a circle on a square box, an ellipse on a rectangle, per CSS Backgrounds §5.5 corner clamping (#329)
- **`border-radius` survives on a text-bearing box** — a box with a direct text run drew its rounded background, then the text's paragraph re-painted the same color as a square rectangle on top, squaring the corners. The redundant fill (and any matching `TextRun.BackgroundColor` highlight) is now cleared, so the common "number inside a colored circle" badge works as a single rounded box (#329)
- **`border-radius` applied across the remaining container paths** — `flex: 0 0 auto` chips, grid items, `display: block` spans, `blockquote`, `figure`, and table wrappers each now round like `display: inline-block`. Previously only the inline-block path took the rounded-rect draw, so visually identical content-hugging boxes rounded or not depending on how they were sized (#329)

#### CSS paged media — `@page`, `position: fixed`, margin boxes (#327, #328)

- **`position: fixed` elements render on every page** — a fixed element (page watermark, pinned footer) was emitted once on the last flowed page instead of repeated on each page. It now draws on every page (#327)
- **`@page :first` / `:left` / `:right` margins inherit the base `@page` cascade** — a partial pseudo-page override (e.g. `@page :first { margin: 20px }`) now inherits the base `@page {}` margins for the sides it does not set, and left/right duplex margin boxes resolve correctly (#327)
- **`@page` parsing correctness bundle** — a batch of fixes to `@page` rule parsing across the converter, layout, and document layers (size/orientation resolution, margin percentage/`calc` resolution, and margin-box cascade), so every document-building entry point applies identical paged-media geometry (#327)
- **`@page` margin boxes embed the default font for PDF/A** — running headers/footers declared via `@bottom-center` etc. now carry the embedded body font, so a PDF/A document with page numbering no longer fails conformance on an unembedded font in the margin box (#328)

#### Lists and inline-block (#330, #339, #342, #347, #358)

- **`display: inline-block` shrinks to fit and flows inline** — an inline-block box now sizes to its content (fit-content width) and participates in inline flow instead of forcing a block break (#330)
- **Styled `<li>` lays out block children and honors its own box** — a list item containing block-level children (cards, nested divs) now lays them out correctly, and the `<li>`'s own background / border / `border-radius` is drawn via a per-item box that the list layout previously lacked (#339, #342)
- **`inline-block` / explicit-width `<li>` hugs its content and stops double-painting** — follow-up to the per-item box: a badge-style `<li>` now shrinks to content and clears the square background overpaint that squared its rounded corners (#342)
- **Ordered lists continue numbering and keep every item across a page break** — a long `<ol>` spanning pages restarted at `1.` on the second page and silently dropped the item straddling the boundary. Numbering now continues the sequence (honoring `<ol start="N">`), and a boundary item is split or deferred so no content is lost. Adds `layout.List.SetStart`/`Start` (#347)
- **Nested list markers indent per level** — a sub-list's marker was drawn at the container's left margin regardless of depth, so every nested ordinal sat in the same left column while only the body text indented. Each level's marker now steps right by the accumulated parent indent, landing under its parent's content edge (LTR and RTL; `list-style-position: inside` already drew relative to the content edge). Single-level lists are unaffected (#358)
- **Margin-box fidelity for `@page :left` / `:right`** — fixing the wiring around `AddConvertResult` surfaced that the left/right margin-box reconstruction dropped the `HasColor` flag and the embedded font that base and `:first` already carried; all four sets are now consistent (#338)
- **`examples/html-to-pdf` no longer drops absolutes** — the example wired the raw `ConvertFull` result by hand and forwarded only `result.Elements`, silently dropping `position: fixed`/`absolute` content. It now uses `doc.AddConvertResult(result)`, which forwards elements, absolutes, `@page` config, and margin boxes in one call (#338)

#### Barcodes (#341, #346)

- **QR codes are now scannable** — `barcode.NewQR` produced structurally valid but unscannable symbols due to four independent encoder defects: the Reed-Solomon generator polynomial was built constant-term-first (so every block's error-correction codewords were wrong), one of the two format-information copies was written bit-reversed, the version-information bits were written in reverse (breaking version 7+), and the level-H ECC-per-block table had wrong codeword counts for versions 21–40. All four are fixed and verified by an independent decoder (#341)
- **EAN-13 quiet zones** — corrected the leading/trailing quiet-zone widths so EAN-13 symbols meet the spec and scan reliably (#346)

### Tests

- **Independent barcode decode tests** — `qr_decode_test.go` reads generated QR symbols back with an independently-built GF(256) field, cross-checks the codeword-capacity tables, and reads format/version info from the matrix; each check was confirmed to fail when its corresponding fix is reverted. Adds Code 128 (A/B/C) and EAN-13 decode round-trips, and an `examples/barcodes` demo rendering QR (all four ECC levels), Code 128, and EAN-13 into one PDF for visual/scanner verification (#341, #346)
- **List pagination regression tests** — multi-page ordered-list numbering continuity, `<ol start>` continuation across a break, unordered no-content-loss, a single item taller than a page, and `SetStart` clamping (#347)
- **`AddConvertResult` equivalence + fidelity tests** — assert `AddHTML` and `ConvertFull` + `AddConvertResult` leave identical document state (page size, all margins, all four margin-box sets, elements, absolutes, metadata), that absolutes and `@page` config survive, and the `:left`/`:right` field-fidelity regression (#338)

### Documentation

- **`using-folio` Claude Code skill** — a thin, version-controlled skill (`.claude/skills/using-folio`) capturing the stable usage patterns: prefer `Document.AddHTML` (or `ConvertFull` + `AddConvertResult`) over hand-wiring, the asset-loading options (`BaseFS`, `FallbackFontPath`, `StrictAssets`), PDF/A setup, and a verification cookbook. The `html` package docs gain matching guidance steering callers to `AddHTML`/`AddConvertResult` (#338)

## [0.9.0] - 2026-05-31

### Added

- **`document.WriteOptions.Deterministic` + `document.Document.SetFileID([]byte)`** — opt-in byte-reproducible output. With `Deterministic` set, the trailer `/ID` is derived from an MD5 digest of the serialized object set (the content-based file-identifier scheme of ISO 32000-1 §14.4) instead of random bytes, and the XMP `CreateDate`/`ModifyDate` plus embedded-file timestamps fall back to the zero time rather than `time.Now` when `Info.CreationDate` is unset — so identical input documents produce byte-identical PDFs. Set `Info.CreationDate`/`ModDate` so the recorded dates are meaningful rather than the year-zero default. `SetFileID` pins the trailer `/ID` directly, overriding both the random PDF/A identifier and the derived digest; a non-PDF/A document that would otherwise carry no `/ID` gains one. Encrypted documents are excluded — the standard security handler mints a random file identifier and derives the encryption key from it (§7.6.3.3), so their output is never byte-stable
- **`html.ParseError` and `html.AssetError` error types** — a typed taxonomy so callers (e.g. a service rendering untrusted templates) can map folio failures onto a response status with `errors.As`. `*ParseError` wraps a failure to parse the input HTML into a document — an input fault; folio's lenient parser makes it rare, but it is distinct from an internal fault. `*AssetError` is the typed form of the failures collected under `StrictAssets`: it carries `Category` (`"image"`, `"@font-face"`, `"background-image"`, `"stylesheet"`, `"SVG image"`, `"FallbackFontPath"`) and `Ref` (the offending `src`/`href`/`url`/`path`), and `Unwrap`s to the underlying cause so `errors.Is` against `fs.ErrNotExist`, `ErrURLPolicyDenied`, network errors, etc. keeps working. The joined error returned by `Convert`/`ConvertFull` contains one `*AssetError` per failed reference. Any error that is neither type should be treated as an internal fault. `AssetError.Error()` is byte-identical to the previous untyped message, so existing log/grep expectations are unchanged
- **`html.Options.MaxElements` / `html.Options.MaxDepth` resource guards + `html.LimitError`** — bound the HTML→layout conversion so untrusted input cannot exhaust memory or the goroutine stack. `MaxElements` caps the number of HTML nodes converted into layout elements; `MaxDepth` caps the element nesting depth. Both default to 0 (unlimited), preserving existing behavior. When a ceiling is crossed, `Convert`/`ConvertFull` stop walking the tree and return a `*html.LimitError` (with `Kind` — `LimitElements` or `LimitDepth` — and the configured `Limit`), failing closed instead of continuing to allocate. The guard sits at the single `convertNode` chokepoint every element node flows through (the `<table>`/grid/flex child loops included), so all conversion paths are bounded. `LimitError` joins the `errors.As` taxonomy alongside `ParseError`/`AssetError` as an input-side fault. Page-count and output-byte ceilings live at the layout and writer layers respectively and are tracked separately
- **`context.Context` on the render entry points** — `html.ConvertWithContext` / `html.ConvertFullWithContext`, `(*document.Document).AddHTMLWithContext`, `(*document.Document).WriteToWithContext`, and `(*layout.Renderer).RenderContext`. Each is the context-aware variant of the existing call; the original (`Convert`, `ConvertFull`, `AddHTML`, `WriteToWithOptions`, `Render`) is preserved and now delegates with `context.Background()`, so this addition is non-breaking. The context is checked at element boundaries during the HTML tree walk (the `convertNode` chokepoint), at page boundaries during layout (the pagination and emission loops in `renderWithPlans`), and at object boundaries during PDF serialization (`writeObjectBodies`). When the context is done, conversion/layout/serialization stops and returns `ctx.Err()` (`context.Canceled` or `context.DeadlineExceeded`); partial output is discarded. This lets a caller bound a render with a real per-render deadline rather than abandoning a goroutine that keeps burning a core on a pathological document. The context is held on the short-lived per-conversion/per-render worker structs (never shared or persisted), avoiding a signature change across every internal recursive call
- **Root `folio` package façade** — a new root-level package re-exports the common document-construction surface so callers can use a single stable import path: `folio.NewDocument`, `folio.Document`, `folio.PageSize` (and the `PageSizeA0`…`PageSizeExecutive` presets), `folio.Info`, `folio.WriteOptions`, `folio.PdfAConfig`, `folio.EncryptionConfig`. The types are aliases and the constructor/presets are value re-exports, so they are fully interchangeable with the `document` package identifiers and carry the complete method set. Specialized APIs (HTML conversion, layout, fonts) remain in their own subpackages; the façade covers only document construction

## [0.8.0] - 2026-05-30

### Visual changes

- **GPOS cursive horizontal placement** — `drawShapedGIDsCursive` previously applied the entry/exit X delta as a `Td` shift on top of the glyph's natural advance, landing each joined glyph one extra advance past its predecessor. Per OpenType §6.3, in horizontal text the X component of the cursive join is already encoded by hmtx; the feature only aligns the join in Y. Documents using fonts with cursive joining (Arabic, several Indic scripts) tighten in horizontal direction. Cursive remains LTR-only; the RTL pass is gated on the LookupFlag `RIGHT_TO_LEFT` bit which is parsed but not yet honoured by the draw path (#220)
- **CJK paragraphs across page breaks no longer corrupt with spurious spaces** — `cloneWithWords` reconstructs the overflow paragraph during page-break splitting and previously joined every same-style word with a literal `" "` regardless of `SpaceAfter`. CJK ideographs (which `breakCJKWords` emits with `SpaceAfter=0`) crossing a page break became `"中 文 文 本"`, and the corrupted text re-wrapped to MORE lines than the original because each inserted space became a real `spaceW`-wide gap on re-measure. Affects any documents with CJK content that paginated in v0.7.x (#246)
- **Unicode NFC normalization is applied at every `layout` entry point** — `NewRun`/`NewRunEmbedded`/`NewParagraph*`/`Heading.SetRuns`/`Row.AddCell*`/`TabbedLine.SetSegments`. Documents that supplied canonically decomposed strings (e.g. `e` + combining acute `U+0301` instead of `é`) now measure and render against the precomposed form. Most input is already NFC, so for the majority of documents output is byte-identical; affected documents see corrected widths and shaping (font cmap tables that only cover precomposed codepoints stop falling through to `.notdef`) (#217)
- **Numeric `font-weight` against standard PDF-14 fonts now rounds at 600** — pre-fix, `parseFontWeight` collapsed every numeric value to the binary string `"normal"`/`"bold"` with the boundary at 700, so `font-weight: 600` against Helvetica picked Helvetica (Regular). Post-fix, weights ≥ 600 select the Bold variant per CSS Fonts L4 §5.2's synthetic-bolding guidance for missing weights. So `font-weight: 600` against Helvetica now picks Helvetica-Bold; `font-weight: 800` continues to pick Helvetica-Bold. Documents that wrote `font-weight: 600` against the standard families and expected Regular weight will see headings tighten visually. The fix also unblocks proper SemiBold/Medium rendering when the family has matching `@font-face` declarations — see Added (#286, #287)
- **`<th>` honours explicit `text-align: left` / `start` instead of silently re-centering** — pre-fix, the table-cell default-center heuristic gated on `cellStyle.TextAlign == AlignLeft`, so an author who wrote `th { text-align: left }` got their explicit choice overridden because the resolved alignment matched the re-center sentinel. Post-fix, the gate is `!cellStyle.TextAlignSet && resolveTextAlign(cellStyle) == AlignLeft`, which preserves explicit author intent (left, or `start`/`end` resolving to left under direction). Default `<th>` still centers when no author choice is set. The new semantic is more spec-correct (UA stylesheet center is a default, not an override) and matches every browser. Documents that wrote `th { text-align: left }` and expected the silent override will see headers shift from center to left (#288)

### Changed (breaking)

- **`html.Options.BasePath` removed; `BaseFS` is the single way to resolve local assets** — the v0.7.x compromise that kept both fields is gone. Callers pass any `fs.FS` (`embed.FS`, `os.DirFS(dir)`, `(*os.Root).FS()`, `fstest.MapFS`) and every local reference — `<img src>`, `<link href>`, `@font-face url()`, `background-image: url()`, and `Options.FallbackFontPath` — flows through it. Paths are normalised to `fs.FS` conventions before the read: forward slashes, no leading `/`, no `..` traversal. A leading `/` in document `src`/`href` is treated as web-style root-of-`BaseFS` (matching how `<base href="/">` works in browsers) instead of an absolute filesystem path. With `BaseFS` nil, every local-asset reference fails — the document is expected to inline its assets via `data:` URIs. The C ABI signature is unchanged: `folio_document_add_html_with_options` still takes a `basePath` C string and wraps it as `os.DirFS(basePath)` internally (#85)
- **`@font-face` URLs resolve relative to their containing stylesheet** — a linked `<link rel="stylesheet" href="css/site.css">` containing `@font-face { src: url(../fonts/Inter.ttf); }` now resolves to `fonts/Inter.ttf` from the `BaseFS` root, not from the document root. HTTP-origin stylesheets resolve relative URLs as HTTP, FS-origin stylesheets resolve them through `BaseFS`. Inline `<style>` blocks continue to resolve relative URLs from `BaseFS` root. Documents that previously relied on the root-anchored behavior need to use root-relative `/fonts/Inter.ttf` or move the `@font-face` rule into an inline `<style>`
- **`layout.UnitValue` is no longer comparable with `==`** — the struct now holds a func field for the new `UnitCalc` variant, so equality comparisons no longer compile. No in-tree consumer compared `UnitValue` for equality; out-of-tree consumers that did will see a compile error and need to switch to comparing the variant tag and resolved value separately (#236)
- **`layout.BackgroundImage.Position` is now `[2]layout.ResolvableLength`** — pre-change the field was `[2]float64`, eagerly resolved at parse time, which dropped plain lengths and mixed-unit calc on `background-position` to 0 because the background box dimensions are only known at draw time. Post-change the field carries a `ResolvableLength` per axis whose `Resolve(container, fontSize float64) float64` method evaluates at draw time. A new sibling `FontSize float64` field on the struct feeds em / rem resolution. Out-of-tree code that constructs `BackgroundImage` directly must implement the one-method `ResolvableLength` interface for each axis (see MIGRATING) (#266, #319)

### Added

- **`html.Options.Logger` (`*slog.Logger`)** — receives warn-level events when a local or remote asset fails to load: missing fonts in `@font-face`, unreadable linked stylesheets, image fetch errors that fall back to alt text. Defaults to nil (silent). Pair with `slog.NewTextHandler(os.Stderr, nil)` during development to surface what would otherwise be swallowed (#85)
- **`html.Options.Client` (`*http.Client`)** — HTTP client used for remote fetches (`<img>`, linked stylesheets, `@font-face url(http://...)`). Lets callers configure timeouts, transport, and proxies; mock the network in tests via `httptest.NewServer`; or share connection pools with surrounding code. Defaults to `http.DefaultClient` (#85)
- **`tmpl.RenderFile` / `tmpl.RenderFileTo` auto-populate `BaseFS`** — when the caller's `Options.BaseFS` is nil, the helpers default it to `os.DirFS(filepath.Dir(templatePath))` so that a template referencing `<img src="logo.png">` next to itself resolves without extra wiring
- **`html.Options.StrictAssets` (`bool`)** — promotes asset-load failures from warn-and-continue to returned errors. When true, `Convert` and `ConvertFull` collect every failed `@font-face url()`, `<img>`, `background-image: url()`, linked stylesheet, SVG load, and `FallbackFontPath`, then return them joined via `errors.Join` at the end of the conversion. The partial result (the elements that did render) is returned alongside the error so callers can inspect both. Errors are returned in document order — linked stylesheets, `@font-face` rules, then asset references in tree-walk order — and are byte-stable across runs given byte-identical input. Defaults to false — production keeps the warn-and-continue behavior. Use it in development and CI to surface broken asset paths in the local feedback loop instead of letting them silently degrade the output (#232)
- **`font.ParseFontForLanguage(data, lang string) (Face, error)`** — TTC face selection by BCP-47 language tag. Pan-CJK font collections (NotoSansCJK, Source Han Sans, Hiragino, PingFang, msgothic.ttc) ship with separate faces for Japanese, Korean, Simplified Chinese, and Traditional Chinese variants encoded in each face's NameID 1 FontFamily ("Noto Sans CJK JP", "Noto Sans CJK SC", etc.). The new entry point picks the face whose family name best matches the requested language: `"zh-CN"` / `"zh-Hans"` → SC, `"zh-TW"` / `"zh-Hant"` → TC, `"ja"` → JP, `"ko"` → KR. Empty `lang` (or any unrecognised hint) falls back to face 0, so `ParseFont` is now a thin wrapper around `ParseFontForLanguage(data, "")` and back-compat is preserved. Pinned by tests against synthetic 4-face TTCs and a real-system round-trip. `examples/cjk` updated to prefer SC-specific standalone fonts (`NotoSansSC-Regular.otf`, `simsun.ttc`) ahead of the pan-CJK TTC bundles whose default face is JP
- **`html.ErrURLPolicyDenied`** — sentinel returned when a `URLPolicy` callback rejects a fetch. The denial wraps it with `fmt.Errorf("%w: %w", ErrURLPolicyDenied, policyErr)` so callers can branch on `errors.Is(err, html.ErrURLPolicyDenied)` to distinguish "I told it to block" from "the asset broke." Under `StrictAssets`, `ErrURLPolicyDenied` is logged through `Options.Logger` but excluded from the joined return error — the caller already received the signal they wired the policy to produce (#232)
- **`font.Fallback` and `layout.NewParagraphFallback`** — Phase 1 of #192. `font.Fallback` wraps an ordered list of `*EmbeddedFont` and dispatches per script run via `PickFace(firstBaseRune)`. `NewParagraphFallback(text, fb, size)` walks `SegmentByScript`, picks one face per script run, coalesces same-face neighbours, and emits N `TextRun`s. Lets a single string mixing Latin / Hebrew / Arabic / CJK render without the caller pre-splitting into per-font runs. Within-script coverage gaps still render as `.notdef`; per-cluster intra-script dispatch is Phase 2. Pointer identity is preserved through the fallback chain so subsetting and the page-level font-resource map keep deduping by pointer — one Type0 dict per face per document regardless of how many paragraphs share the fallback (#225)
- **`layout.Paragraph.MeasureLines(maxWidth) int` and `MeasureHeight(maxWidth) float64`** — wrapped line count and rendered height (sum of line heights, excluding `SpaceBefore`/`SpaceAfter` so callers compose with their own pagination math) at a given width. Both are thin wrappers over the existing `Layout()` path, so the values always agree with what the renderer will produce. Unblocks clamp/truncate decisions before render (#238)
- **`layout.Paragraph.SplitAfterLine(n int, maxWidth float64) (head, tail *Paragraph)`** — splits a paragraph after the first `n` rendered lines at a given width. Both returned paragraphs are clones; the receiver is unchanged. `n <= 0` returns `(nil, full)`, `n >= total lines` returns `(full, nil)`. Spacing ownership: the half that owns the original boundary keeps that spacing; `FirstLineIndent` is dropped from both halves so re-laying re-applies it correctly. Designed for first-N-lines-plus-appendix flows. The `cloneWithWords` audit chain (#238/#241/#246/#250) shipped first so the resulting clones reproduce correctly across all script families, inline elements, links, and forced line breaks (#253)
- **`layout.Anchor` element + auto-registered PDF named destinations** — `<a href="#anchor">` now emits a real PDF link annotation with an internal `/Dest` rather than a `/URI` action containing the literal `"#anchor"`. The HTML walker prepends a zero-height `layout.Anchor` for any element with an `id` attribute; the renderer surfaces these onto `PageResult.Anchors`, and the document layer registers them as PDF named destinations automatically — no caller `doc.AddNamedDest()` calls required. `TextRun`/`Word` link plumbing splits into `LinkURI` vs `LinkDestName` so a single line can carry both kinds of links (#223)
- **`layout.UnitCalc` variant + `layout.CalcUnit(fn)` constructor** — a `UnitValue` whose value is computed lazily at layout time via a `Calc func(available float64) float64` closure. Introduced so CSS lengths that depend on a percentage in a calc expression (e.g. `flex: 0 0 calc(50% - 8px)`) can resolve against the actual layout area at render time rather than against the pre-margin container width. Pure absolute lengths still resolve eagerly via `Pt` — the new variant only kicks in when the parser detects percentage participation through `cssLength.dependsOnPercent()` / `calcExpr.dependsOnPercent()` (#236)
- **`document.Info.Language`** (BCP-47 / RFC 3066) — writes catalog `/Lang` per ISO 32000-2 §14.9.2 and `dc:language` in XMP. Required for any PDF/A Level A variant per ISO 19005-2/3 §6.7.2 (#214)
- **PDF/A-3a and PDF/A-4 family** — `PdfA3a` (ISO 19005-3:2012, Level A) for accessibility-conformance with attachments; `PdfA4` (base, PDF 2.0), `PdfA4F` (embedded files, Factur-X successor), `PdfA4E` (engineering, replaces PDF/E-1) for ISO 19005-4:2020. `pdfaid:rev = "2020"` emitted for part 4; `pdfaid:conformance` omitted for plain A-4 and present (`F` / `E`) for A-4f / A-4e. Embedded files now permitted in A-3 (a/b) and A-4 (f/e); the `pdfaf` extension schema is declared for every level allowing associated files (#214)
- **C ABI: `folio_document_set_language(doc, lang)` + `FOLIO_PDFA_3A` / `FOLIO_PDFA_4` / `FOLIO_PDFA_4F` / `FOLIO_PDFA_4E`** — companions to the new Go-side PDF/A surface. Existing `FOLIO_PDFA_*` numeric values are preserved (#214)
- **C ABI: `folio_paragraph_measure_lines` / `folio_paragraph_measure_height` / `folio_paragraph_split_after_line`** — surface the new `Paragraph` measurement and split helpers (#238, #253) to non-Go callers. `measure_lines` returns `int32_t` (`-1` on bad handle); `measure_height` returns `double` (`-1.0` on bad handle); `split_after_line` returns two paragraph handles via out-pointers, either of which may be `0` for the no-op halves (`n <= 0` → `(0, full)`, `n >= total` → `(full, 0)`). Receiver unchanged; caller frees non-zero halves (#283)
- **C ABI: `folio_font_parse_for_language(data, length, lang)`** — wraps `font.ParseFontForLanguage` so C consumers can pick the right TTC face by BCP-47 tag (`"zh-CN"`, `"ja"`, etc.) for pan-CJK font collections. NULL `lang` falls back to face 0, matching `folio_font_parse_ttf` semantics. The `font.Fallback` / `layout.NewParagraphFallback` surface (#225) is intentionally NOT exposed in v0.8.0 — per its PR description, C ABI exposure is Phase 2 work tied to promoting `*EmbeddedFont` to an interface (#283)
- **C ABI exports grow from 388 (v0.7.1) to 393 (+5)** — `scripts/audit-cabi.sh` reports Go `//export` directives, `export/folio.h` declarations, and built dylib symbols all in sync at 393. The header is updated in lockstep with each addition
- **WASM CLI exposes `pdfa3a` / `pdfa4` / `pdfa4f` / `pdfa4e`** — and fills in profile keys the Go API already supported but the playground did not (`pdfa1a`, `pdfa2u`, `pdfa2a`, `pdfa3b`) (#214)
- **CSS `:empty` pseudo-class** (Selectors L3 §9) — matches an element with no element children and no non-empty text children. Comments do not count as children, per spec (#215)
- **CSS `::placeholder` pseudo-element** (Pseudo-Elements L4) — applies to `<input>` and `<textarea>` carrying a `placeholder` attribute, only when the field has no `value` (input) or body text (textarea). Supported sub-properties: `color`, `font-style`, `font-weight`, `font-size`. Numeric font-weight values route through `parseFontWeight` so `font-weight: 700` / `bolder` resolve to the bold font variant (#215)
- **Indic shaping for eight Brahmic scripts** — Bengali, Gujarati, Gurmukhi, Kannada, Malayalam, Oriya, Tamil, Telugu. Generalises the previously Devanagari-only five-phase shaper around a per-script `indicScriptConfig`. Existing Devanagari behaviour and tests are preserved. `ScriptOf` / `Script` is extended to recognise the six new scripts (Bengali and Tamil were already mapped). Paragraph layout dispatches via `indicConfigFor(Script)`. Gurmukhi and Tamil use `rephPosNone` so a leading Ra + virama + consonant in those scripts is left as a plain halant-joined cluster (#216)
- **GPOS LookupType 6 (Mark-to-Mark Positioning)** — combining marks anchor to the previously placed mark instead of stacking on the same base anchor, unblocking Arabic shadda + harakat stacks, Hebrew dagesh + niqqud, Vietnamese tone-on-diacritic vowels (`ế`), and stacked IPA tone marks. New `MarkMarkOffset(mark1GID, mark2GID, feature)` accessor; parameter order follows the spec's mark1/mark2 (attaching/receiving) convention (#213)
- **GPOS LookupType 3 (Cursive Attachment)** — parses CursivePosFormat1 entry/exit anchors and applies them at draw time on shaper-produced GID streams. Each glyph emits its own `Tj`; the second through Nth are bracketed by `Td` shifts that align the entry anchor onto the previous exit. New `CursiveOffset(prevGID, currGID, feature)` accessor returning `prev.Exit - curr.Entry`. LTR-only: RTL handling is gated on the LookupFlag `RIGHT_TO_LEFT` bit, which is parsed and stored on the `CursiveRecord` but not yet honoured by the draw path (#218)
- **GPOS LookupType 5 (Mark-to-Ligature)** — parses MarkLigPosFormat1 with per-(component, mark-class) anchor grids and a parallel `Present` grid. New `MarkLigatureOffset(ligGID, componentIdx, markGID, feature)` accessor. Draw integration tries Type 5 before falling back to Type 4 mark-to-base when the cluster base is a ligature carrier. Component attribution uses a stop-gap heuristic (first Extend/ZWJ mark to component 0, subsequent marks to the last component) because the layout pipeline does not yet plumb cluster→component IDs from the shaper; ligatures with three or more components fall back to Type 4 to avoid silent middle-component misplacement. `TODO(#218)` left in `layout/draw.go` for the follow-up (#218)
- **CSS `counter(page)` and `counter(pages)` resolve in body-flow `::before` / `::after` `content`** — previously these only worked inside `@page` margin boxes. Two-pass emission: pagination is decided once, then a second pass draws each page with the final page total available, so `counter(pages)` is substituted directly during draw rather than via a post-stream byte replace. Layout-time width reservation for counter placeholders keeps line breaks stable across digit-count transitions (page 9 → 10, 99 → 100). `target-counter(url(#anchor), page)` for cross-references remains tracked as #222 (#221)
- **CSS GCPM bookmark properties for PDF outlines** — `bookmark-label` resolves `content()`, `attr(NAME)`, and literal strings during HTML conversion (missing attributes fall back to the element's text content); `bookmark-level: none` excludes an element from the outline (cover pages, suppressed headings); `bookmark-state: closed` marks the subtree collapsed in viewers via the negative `/Count` sign convention (ISO 32000 §12.3.3). Non-heading elements with an explicit `bookmark-level` join the outline through a new `BookmarkAnchor` wrapper that decorates the first `PlacedBlock` without retagging the structure tree (a `<figure>` stays a Figure in the structure tree). Level-skip rule documented: an entry whose level skips a parent (h1 → h3) nests under the nearest preceding lower-level entry (#224)
- **CSS shorthand parsers preserve `calc()` / `min()` / `max()` / `clamp()` values** — every shorthand below previously used `strings.Fields` on values that may legally contain functional values, which split on the function's internal whitespace and silently dropped the declaration. A new paren-aware `splitTopLevelFields` (sibling of `splitTopLevelCommas`) replaces it across: `flex` (#236), `margin`/`padding` (#237), `font` (#240), `border` (#242), `background-size` (#244), `@page size` (#247, plus the page-local length parser is now calc-aware via `parseLengthPt`), `gap`/`grid-gap` (#249), `border-radius` (#252), `box-shadow` (#254), `transform-origin` (#257), `border-spacing` (#258), `columns` (#259), and `column-rule` (#261). `border` and `column-rule` additionally regain `rgb()` / `rgba()` / `hsl()` color parsing (which had been shredded by the same tokenizer). For `flex` only: percentages inside calc resolve lazily through the new `layout.UnitCalc` variant against the actual layout area at render time (#236)
- **`<html lang>` propagated to `@font-face` TTC face selection** — `html.ConvertFull` now reads the document root's `lang` attribute and routes it through `font.ParseFontForLanguage`, so `<html lang="zh-CN">` + `@font-face url('NotoSansCJK-Regular.ttc')` picks the SC face instead of the JP face-0 default. The same hint also flows to `Options.FallbackFontPath` and the built-in system-font candidate list (loaded lazily, so the lang is available by the time the loader runs): a doc declaring `lang="zh-CN"` whose `<p>` falls back to the CJK system font gets the SC face there too. The extracted tag is also exposed on `ConvertResult.Metadata.Language` for callers that want to feed it into the PDF catalog's `/Lang` entry or other downstream consumers. Per-element `lang` overrides (`<p lang="ja">`) are NOT yet honoured — `@font-face` is loaded once at converter setup, so element-level overrides would need either eager multi-face loading or a shape-time face selector. Deferred as Phase 2 of #280 (#280)
- **`font.LoadFontForLanguage(path, lang string) (Face, error)`** — disk sibling to `ParseFontForLanguage`. The same BCP-47 face-selection rules apply; empty `lang` falls through to face-0 / single-face semantics identical to `LoadFont`
- **Numeric `font-weight` ladder + nearest-weight `@font-face` matching (CSS Fonts L4 §3.1 + §5.2)** — pre-fix `parseFontWeight` returned the binary string `"normal"`/`"bold"` and the `@font-face` resolver looked up faces by that string. Documents declaring four Inter weights at 400/500/600/700 and writing `font-weight: 600` silently picked Inter-Regular instead of Inter-SemiBold; same for 500 (Medium) collapsing to Regular. Post-fix `computedStyle.FontWeight` is `int` over the 100..900 ladder, `parseFontWeight` returns the spec ladder (with `bolder`/`lighter` resolving relative to the inherited weight), and `resolveFontPair` does §5.2 nearest-weight matching: exact match wins, then walks the ladder per the spec's per-window algorithm (`[400, 500]` ascending toward 500 first, `< 400` descending then ascending, `> 500` ascending then descending). Plus an italic-vs-normal style fallback so an author asking for italic Inter at a registered weight gets the regular-style face instead of falling through to Helvetica. `computedStyle.FontWeight` is package-internal so this is not a public API break (#286, #287)
- **CSS `text-align: start | end` direction-relative keywords (CSS Text L4 §7.1)** — pre-fix `parseTextAlign` returned `(AlignLeft, ok=false)` for both, so the registry's Apply silently dropped the declaration: `<p style="direction: rtl; text-align: start">שלום</p>` rendered left-aligned despite the author asking for the RTL start edge. Post-fix the parser preserves the keyword on `computedStyle.TextAlignKeyword` and a new `resolveTextAlign(style)` helper late-binds the resolution against `style.Direction` at consumer time (so source-order interactions like `text-align: start; direction: rtl` resolve correctly regardless of declaration ordering). Same treatment for `text-align-last` via `resolveTextAlignLast`. New public `layout.Paragraph.Align()` getter exposed for test inspection (#288)
- **CSS `text-decoration-line: overline` (CSS Text Decoration L4 §3.1)** — pre-fix `layout.TextDecoration` defined `Underline (1<<0)` and `Strikethrough (1<<1)` only; the third spec line type was silently dropped at parse time. `<p style="text-decoration: overline">x</p>` rendered plain. Post-fix `DecorationOverline = 1 << 2` joins the bitset, `parseTextDecoration` ORs it in for the `overline` keyword (combinations like `text-decoration: underline overline line-through` produce all three flags), and `drawDecorations` renders the stroke at `baselineY + ascent * 0.95` with `text-decoration-style` dispatch (solid / double / wavy / dashed / dotted) honoured. The `double` secondary stroke offsets DOWNWARD (toward text) so both lines stay inside the line box even at `line-height: 1`. Adjacent runs with mismatched decoration sets correctly mask the trailing-space extension to only their shared subset, fixing a latent bug surfaced by the new flag (#289)
- **CSS border styles `groove | ridge | inset | outset` (CSS Backgrounds L3 §4.1)** — pre-fix the parser stored the keyword string but `buildBorder`'s switch matched 3 keywords + default-solid, so `<div style="border: 4px ridge silver">` parsed and stored "ridge" but rendered as a flat solid line. Post-fix the parser whitelist accepts the four 3D bevel styles and `buildBorderForSide(side, ...)` dispatches per-side via new `beveledColor` / `lightenColor` / `darkenColor` helpers (fixed 30% sRGB shift; CMYK adjusts K). Per CSS §4.1: groove/inset → top+left dark, bottom+right light (sunken); ridge/outset → opposite (raised). Folio renders a single solid stroke per side with side-modulated color rather than the strict spec's two-half-width split bevel — visually indistinguishable for thin borders, less pronounced bevel for thick ones (#290)
- **CSS `max-width` / `max-height` / `min-width` / `min-height` on `<img>` (CSS 2.1 §10.7)** — pre-fix the CSS parser stored these properties on `computedStyle` but `html/converter_image.go::convertImage` never threaded them to `layout.ImageElement`, and `ImageElement` had no fields to receive them. So a 2000×500 source logo with `max-width: 180px` in a 540pt-wide page bled the entire header instead of clamping. Post-fix `layout.ImageElement` gains `cssMaxWidth` / `cssMaxHeight` / `cssMinWidth` / `cssMinHeight` fields plus `SetMaxSize(maxW, maxH)` and `SetMinSize(minW, minH)` setters; `resolveSize` applies the clamps in spec order (max first, then min, with min winning over max per the conflict rule), aspect-preserving on each axis. The `object-fit` branch's early-return path now applies the clamps before fitting, so `<img width=200 height=200 style="object-fit: cover; max-width: 50px">` correctly produces a 50×50 box with cover-fit content. The container width remains the absolute outer bound for replaced elements per §10.4 — a `min-height` that would push the image past the canvas is silently dropped to keep the image inside its containing block, matching browser behaviour. Percentage `max-width: 50%` still resolves to 0pt because the converter's CSS-to-points conversion passes `relativeTo=0`; pre-existing limitation tracked separately (#291, #292)
- **CFF v1 subsetter for CID-keyed OTF fonts** — Adobe-Japan1 / Adobe-GB1 / Adobe-Korea1 / Adobe-CNS1 fonts embedded through the new CFF read path (#260) previously shipped the full CFF Top DICT, CharStrings INDEX, and FDArray/FDSelect into the PDF, producing multi-megabyte output for documents that only used a handful of glyphs. The new subsetter walks the used-GID set from the layout pass, rewrites CharStrings, prunes FDArray entries, and emits a minimal CID-keyed CFF stream through `/FontFile3` + `/CIDFontType0`. On the test corpus a 1-page document quoting a single CJK ideograph from Source Han Sans dropped from 11.1 MB to 173 KB end-to-end. Pure-Type 1 (non-CID) CFF fonts continue to ship un-subset because the layout pipeline emits CIDs unconditionally; gating on CID-keyed-v1 keeps the subsetter conservative. Sharing of embedded and standard fonts across pages also lands in the same change, closing the duplicate-FontDescriptor accounting that #295 originally surfaced (#295, #299, #300)
- **`background-position` and gradient color stops accept percent-only `calc()` / `min()` / `max()` / `clamp()`** — pre-change the parsers only recognised bare percent / length leaves; expressions like `background-position: calc(50% + 10px) 0` or `linear-gradient(red, blue calc(50% + 1px))` silently fell back to 0. Post-change `parseBgPosition` and the gradient-stop parser accept any calc tree whose participation includes percent (per the rules in #236). The bg-position half then resolves lazily via the new `layout.ResolvableLength` machinery added in #319; gradient-stop calc remains eagerly resolved against the box width at parse time. Followup tests and a `splitTopLevelFields` doc gap close out in #314, and a `parseCalcExpr` depth-reset fix between operator scan passes lands in #315 (#266, #312, #314, #315)

### Changed

- **Microsoft Symbol fonts (Wingdings, Symbol, dingbat) now load and render correctly** — the cmap parser accepts the (platformID=3, encodingID=0) Symbol-encoding fallback when no Unicode subtable is present (matching HarfBuzz / go-text / pdf.js), and additionally mirrors PUA-keyed entries (0xF020..0xF0FF) to the corresponding ASCII codepoints (0x0020..0x00FF). Without the mirror these fonts loaded but `<p>A</p>` rendered as `.notdef` because the html-shaping path sends U+0041, not U+F041; HarfBuzz applies the same alias automatically. Existing ASCII-range entries in the source cmap are NOT overwritten — explicit values from the font win over synthesized aliases (#262)
- **`font` package no longer depends on `golang.org/x/image/font/sfnt` for metric reads** — `head`, `hhea`, `maxp`, `hmtx`, `OS/2`, `name`, and `cmap` are now parsed directly from raw font bytes via new files `font/{head,hhea,maxp,hmtx,os2,name,cmap}.go`. Closes #248 (sfnt's hardcoded `maxCmapSegments = 20000` blocked Microsoft YaHei, Noto Sans CJK, STHeiti) and fixes #227 (the CJK rendering bug those fonts caused) without the architectural debt of the closed PR #251 (recovery-path heuristic that substring-matched sfnt's unexported error message). `Ascent`/`Descent` follow the OpenType USE_TYPO_METRICS convention (OS/2 fsSelection bit 7): when set, the foundry has explicitly requested the typo metrics, so we use `sTypoAscender`/`sTypoDescender`; otherwise we use the hhea ascender/descender. This preserves byte-identical FontDescriptor `/Ascent` and `/Descent` output for every font where USE_TYPO_METRICS is unset (the majority — and every font sfnt v0.39.0 produced metrics for, since sfnt unconditionally used hhea). Fonts that opt into the bit see their values shift to match the foundry's intent. CFF/OTF outline support is unchanged: Folio's subset code already only handled TrueType `glyf`/`loca`, so CFF fonts continue to load and embed (raw CFF bytes opaque) but cannot be subset (#260)
- **`html` asset resolution centralized behind a single `resolveLocalAsset` contract** — `<img src>`, inline SVG, `<link rel="stylesheet" href>`, `@font-face url()`, `background-image: url()`, and `FallbackFontPath` previously each carried their own routing logic that disagreed on absolute paths, root-anchored paths, and HTTP origins. They now route through one method documented in `ARCHITECTURE.md` "Asset resolution". Behavior-preserving for every existing test (`go test ./...` green); the visible side-effects are that `<img src="https://...svg">` and inline-SVG `<img>` URLs now flow through `URLPolicy` enforcement uniformly, and `@font-face url('/abs/system/font.ttf')` succeeds with `BaseFS: nil` (closes the workaround originally proposed in PR #228). `FallbackFontPath` retains a documented programmatic-only carve-out: an absolute path always bypasses `BaseFS` to the OS, and a relative path that misses `BaseFS` retries against the OS — neither extends to document-supplied references because the trust boundary is different (#229)
- **HTML CSS parser refactored into a declarative property registry** — replaces the ~700-line `applyProperty` switch in `html/converter_style.go` with one `cssProperty` entry per CSS property (138 entries total) carrying its own `Apply` closure plus documentation fields. `applyProperty` shrinks to 7 lines: pre-dispatch logic preserved verbatim (custom-property `--` storage, then `var()` resolution, then registry dispatch). The same registry is the source of truth for `docs/CSS_SUPPORT.md`, auto-generated by `go generate ./html/...`. `applyTagDefaults` (browser HTML element defaults) is untouched. No public API change; behaviour-preserving across the full suite (#271)
- **Error message prefixes standardized to `<pkg>:` / `<pkg>: <subdomain>:` / `<pkg>.<Func>:`** — top-level errors get `<pkg>:`, sub-feature errors inside `font/` and `reader/` get `<pkg>: <subdomain>:`, and panics naming a public function get `<pkg>.<Func>:`. Adds package prefixes to ~130 previously unprefixed internal helper errors. Renames the message of `html.ErrURLPolicyDenied` from `"folio/html: ..."` to `"html: ..."`; the exported `var` identity is unchanged, so `errors.Is(err, html.ErrURLPolicyDenied)` still works. Callers that string-compared `err.Error()` against the old verbatim message — including the `folio/html:` prefix — must update their assertions (#313)
- **`computedStyle` margin/padding storage migrates to lazy `*cssLength` siblings** — the eight legacy `float64` fields (`MarginTop`/`Right`/`Bottom`/`Left`, `PaddingTop`/`Right`/`Bottom`/`Left`) are removed from `computedStyle`; every read goes through `MarginTopAt(width)` / `PaddingTopAt(width)` helpers backed by sibling `*cssLength` fields. Tag-default setters write `&cssLength{Value: N, Unit: "pt"}` so heading / paragraph margins are no longer baked-in floats. `computedStyle` is package-internal so this is not a public API break; the `PageConfig.MarginTop` field used by `page.go` is a separate type and stays as `float64`. A `TestHeadingDefaultMarginsSurviveMigration` regression pins the documented per-heading top/bottom margin values (16.08, 14.94, 14.04, 16.02, 16.60, 18.62) (#269, #309, #310, #311)

### Fixed

- **SVG `preserveAspectRatio` slice viewport clip** — when an SVG is drawn with `slice` meet-or-slice, the renderer now emits a PDF clip path on the target rectangle before the viewport transform. Previously the uniform scale was applied correctly but content outside the target rectangle leaked onto the page. Callers that already clipped externally will continue to work (#196)
- **TrueType Collection (`.ttc`) fonts now load** — `font.ParseFont` and `font.LoadFont` advertised TTC support but routed the bytes to `sfnt.Parse`, which rejects collections with `invalid single font (data is a font collection)`. The dispatch now extracts face 0 from the collection into a standalone single-font TTF (table directory rewritten to point at the new offsets) and parses that. Selection of face 0 matches browser behavior for `url()` references without a `#` fragment. Very large CJK collections may still hit `golang.org/x/image/font/sfnt`'s hardcoded `maxCmapSegments` limit — that is an upstream parser limit, orthogonal to TTC dispatch (#227)
- **`font.ParseFont` no longer falsely advertises PostScript Type 1 (`typ1`) support** — the magic was listed in the dispatch alongside TrueType variants, but `ParseTTF` routes it to `sfnt.Parse` which rejects Type 1 with a confusing `sfnt: invalid font` error. The entry is gone; callers feeding `typ1` bytes now receive a clear `ErrUnknownFormat`. Type 1 has been deprecated since PDF 2.0 and no modern font foundry ships in this format, so re-introducing support would require a full Type 1 parser, not just adding the magic back. New `TestParseFontDispatchSurface` audits every magic the dispatch claims to support to prevent the same false-advertisement shape recurring (#230)
- **GPOS Type 5 mark-to-ligature now falls back to Type 4 mark-to-base for ligatures with three or more components** — without per-cluster component attribution, middle-component anchors silently misplaced marks. The bail-out preserves correct rendering for the common two-component cases (Arabic lam-alef, Latin "fi" / "ffl") while avoiding the misplacement on longer ligatures (#220)
- **Inline elements and Arabic `OriginalText` survive paragraph page splits** — `cloneWithWords`'s `wordToRun` helper was dropping `Word.InlineBlock` (inline image / SVG / Div) so any inline element falling into the overflow half of a paragraph was silently lost, and was copying post-shape `Word.Text` instead of pre-shape `Word.OriginalText` so re-laying skipped repopulating `OriginalText` and broke `/ActualText` marked-content recovery (ISO 32000-2 §14.9.4) for Arabic. `wordToRun` now copies `InlineBlock` → `InlineElement` and uses `OriginalText` (when set) as `Text`; `cloneWithWords`'s `sameRun` guard rejects merges across inline elements (#241)
- **`font:` shorthand handles a `/` inside `calc()` correctly** — the size/line-height detector used `strings.IndexByte(sizeStr, '/')` which found any `/` including ones inside `calc(2em / 2)` and split mid-expression. Replaced with a paren-aware `indexByteAtTopLevel` helper. Same fix surface as the calc-tokenization series (#270)
- **`column-rule` width zeroes when `column-rule-style` is `none` or `hidden`** — matching CSS Multi-column Layout L1 (same rule as `border-style: none`). `parseBorderFull` already implemented this; `parseColumnRule` did not, so `column-rule: 4px none red` returned width = 3pt instead of 0pt. `hidden` is now also accepted as a style keyword (parity with `parseBorderFull`) (#270)
- **`transform()` parses `calc()` / `min()` / `max()` arguments correctly** — two compounding paren-blind bugs in `parseTransform`. The outer function-call extractor used `strings.Index(val[parenIdx:], ")")` and so truncated `translate(calc(50% - 10px), 0)` mid-calc; the inner argument splitter used `strings.ReplaceAll(args, ",", " ")` which destructively flattened nested commas (`min(10px, 20px)` → `min(10px  20px)`). The outer is now depth-tracking; the inner uses `splitTopLevelCommas` first with a `splitTopLevelFields` fallback for the legacy SVG space-separated form. Calc inside `rotate` / `scale` / `skew` is fixed in a follow-up via #284 (#272)
- **`parseAngle` and `parseNumericVal` understand `calc()` / `min()` / `max()` / `clamp()`** — closes the per-arg gap left after #272 surfaced calc to transform args. `transform: rotate(calc(45deg + 45deg))` now resolves to 90deg (was 0), `scale(calc(0.5 + 0.5))` to 1.0 (was 0, a degenerate zero-scale transform), `skew(calc(10deg * 2))` to 20deg. `parseAngle` gains an angle-native calc evaluator (`resolveAngleCalc`) that mirrors `parseCalcExpr`'s grammar but reads each leaf as an angle in degrees (or a dimensionless multiplier). `parseNumericVal` handles min/max/clamp directly and delegates calc to the existing dimensionless-leaf path. `parseAngleLeaf`'s suffix-detection order is fixed in passing — pre-fix `100grad` matched the `rad` suffix before `grad` and resolved to 0; the new ordering checks turn/grad/rad/deg (#274, #284)
- **`parseLineHeight` distinguishes dimensionless calc from length-form calc** — `font: 12px/calc(1.2 * 1.5) sans` now resolves to multiplier 1.8 as written. Pre-fix the dimensionless calc result (1.8) was passed through `pts := l.toPoints(fontSize, fontSize)` and then divided by fontSize=9pt, yielding 0.2 — a 9× compression of line spacing. New helpers `cssLength.isDimensionless()` and `calcExpr.isDimensionless()` walk the calc tree and report true iff every leaf is a bare number; `parseLineHeight` short-circuits via that check before the existing length-form path. The length-form calc (`calc(1.5em)`, `calc(1em + 4px)`) is unchanged (#275, #284)
- **`margin: <calc> auto` correctly flags both auto-sides** — the auxiliary auto-flag scan inside the `margin` case of `applyProperty` still used `strings.Fields` after #237 migrated `parseMarginShorthand` itself. The two tokenizers diverged on functional values, so `margin: calc(10px + 20px) auto` correctly resolved top/bottom = 22.5pt and left/right = auto, but only `MarginLeftAuto` was flagged — `MarginRightAuto` was missed. Both tokenizers now use `splitTopLevelFields` (#263)
- **Paragraph background stays pinned to the content box under `text-align: center` / `right`** — pre-fix the background rectangle was drawn from the line's measured start X, so a centred or right-aligned paragraph with `background-color` painted a stripe under each centred line rather than under the paragraph's content box. The renderer now anchors the background to the content box bounds regardless of per-line alignment, matching browser behaviour (#293)
- **`word-break: break-all` honoured in the second paragraph measurement pass** — `measureWords` was consulted twice during paragraph layout but only the first pass observed `word-break: break-all`, so a paragraph that needed re-measurement (page splits, container resizes, line-height re-evaluation) regressed to the default no-break behaviour mid-layout. The second pass now reads the same word-break state as the first, so long-word breaks remain stable across re-measurement boundaries (#294)
- **`breakLongWords` preserves `OriginalText` and shaped GIDs across chunk boundaries** — when a single word overflowed the available width and was split into multiple chunks, the helper kept the raw substring but dropped the pre-shape `OriginalText` and the post-shape `GIDs` slice. The downstream `/ActualText` marked-content emission (ISO 32000-2 §14.9.4) then lost the recovery hint for Arabic and complex-script words split across line boundaries, and the draw path re-shaped from the chunk's runes instead of reusing the already-shaped GIDs. Both fields are now propagated through every chunk, so accessibility recovery and shape-cache reuse survive the split (#255, #307)

### Deprecated

The four direct-field accesses deprecated in v0.7.0 remain fully functional in v0.8.0 and continue to work identically to their accessor-method equivalents. Removal target is v1.0, when the stable public API will be declared. Migrate before then to avoid a compile-error churn at the v1.0 boundary.

- **`core.PdfDictionary.Entries`** direct field access — use `All`, `Get`, `Set`, `Remove`. Direct slice mutation bypasses the lazy key index and can desync the dictionary's lookup table.
- **`core.PdfArray.Elements`** direct field access — use `All`, `At`, `Len`, `Add`, `Set`, `RemoveAt`, `Replace`.
- **`core.PdfIndirectReference.ObjectNumber`** — use `Num` for reads and `SetNum` for writes.
- **`core.PdfIndirectReference.GenerationNumber`** — use `Gen`.

`core.RevisionRC4128` (RC4 encryption) remains formally discouraged for new documents — RC4 is cryptographically broken — but is **NOT** scheduled for removal. It is kept indefinitely to support reading and writing legacy RC4-encrypted PDFs. Use `RevisionAES256` for new encrypted documents.

### Tests

- **`examples/cjk` is now CI-tested end-to-end** — adds `examples/cjk/main_test.go` that builds the example's HTML against a synthetic TTC fixture (constructed from any system TTF at test time), runs `html.ConvertFull` and `document.Save`, and asserts the resulting bytes start with `%PDF-` and embed the requested font's PostScript name (matching the `+<name>` subset prefix). Issue #227 — broken TTC dispatch on Windows / Linux — was the kind of regression this would have caught: the example documented working `msyh.ttc` and `NotoSansCJK-Regular.ttc` paths but neither was ever exercised end-to-end because nothing in CI compiled or ran the example. A second test (`TestCJKExampleFindCJKFontReturnsExistingPath`) verifies the example's `findCJKFont` candidate list does not drift away from on-disk reality. Phase 1 of #231; subsequent PRs roll the same pattern out to other examples (#231)
- **Hyphenation contract guard for `cloneWithWords`** — locks in the dormant-but-correct behaviour: `hyphenateWord` produces a part-and-rest pair where `part.Text` ends in `-` and `SpaceAfter=0`, and `cloneWithWords` joins same-style words. A naive join would emit `"linguis-tic"` instead of the original `"linguistic"`. Today the path is unreachable because `wrapWords` (which `PlanLayout` uses) does not call `hyphenateWord` — only `Paragraph.Layout` does. The new test exercises the page-split path with a long alpha word eligible for hyphenation under both Liang-Knuth and character-boundary fallback paths and asserts no `-` leaks into the overflow runs. If a future change wires `hyphenateWord` into `wrapWords`, this fails immediately and signals that `cloneWithWords` needs hyphen-aware join logic before that change can land. Closes the audit chain that produced #241 / #246 (#250)
- **Phase 2 of `#231` — end-to-end CI tests for the remaining `examples/` directory** — extends the same pattern Phase 1 of #231 introduced for `examples/cjk` to the rest of the example tree, so every documented example now compiles and runs in CI rather than rotting silently. New `main_test.go` files build the example's PDF and assert `%PDF-` prefix + key invariants per example: `examples/html-to-pdf` reads back through `pdf.Reader` and checks rendered text plus image presence (with cached bytes to amortize the build cost across assertions); `examples/report` builds with reader-based content assertions; `examples/{hello, links, template}` cover the small-surface examples in one batch; `examples/{merge, import-page, redact}` cover the mid-complexity reader-write round-trip examples in one batch (#231, #301, #302, #303, #304)
- **Synthetic CJK font fixture for integration tests** — replaces the previous "skip if NotoSansCJK is not on the test box" gate with a deterministic in-memory CJK font built at test setup time, so the CJK shaping and TTC dispatch tests now run on every CI machine regardless of installed fonts. The fixture builds a minimal CFF + cmap pair sufficient for the parser and shaper to traverse end-to-end (#281, #305)
- **Direct unit test for `parseLineHeight` dimensionless / length-form calc paths** — covers the helper introduced by #275 in isolation rather than only via the `font:` shorthand integration test. Locks in the `cssLength.isDimensionless()` / `calcExpr.isDimensionless()` walk so a future change to the calc grammar can't regress the 9× compression bug that #275 originally fixed (#275, #306)

### Documentation

- **`docs/CSS_SUPPORT.md` introduced** — auto-generated from the CSS property registry by `go generate ./html/...`. Covers all 138 supported CSS properties grouped by category (Typography / Color / Backgrounds / BoxModel / Borders / Layout / Flexbox / Grid / MultiColumn / Tables / Pagination / Lists / Effects / PDF), with per-property aliases / accepted values / notes. Adds a value-form glossary, a box-alignment cross-context callout, a "Known unsupported features" table with concrete workarounds (`oklch`/`color-mix` → precompute or use a CSS variable; `line-clamp` → `layout.Paragraph.SplitAfterLine`; `filter` / `backdrop-filter` → pre-bake into images; etc.), and a contributor guide for adding a new property. Linked from the README. `TestCSSDocsInSync` enforces drift detection between the registry and the on-disk file (#271, #273)
- **`docs/CSS_SUPPORT.md` extended with Selectors / At-rules / Functions** — three hand-written sections covering the surface area the property registry doesn't reach. Selectors: 4 combinators, 5 simple selectors, 7 attribute operators, 13 pseudo-classes, 4 pseudo-elements, with non-supported sets called out in each subsection (interaction-state pseudo-classes, single-colon pseudo-element legacy forms, attribute case-sensitivity flags). At-rules: `@font-face`, `@page` + selectors, `@page` margin boxes, `@supports`, `@media print`, plus the silently-ignored set (`@media` non-print, `@import`, `@keyframes`, `@counter-style`, `@namespace`, `@charset`, `@layer` / `@scope` / `@container` / `@property`). Functions: ~30 entries across math (`calc`/`min`/`max`/`clamp`), color (`rgb`/`rgba`/`hsl`/`hsla`/`cmyk`), gradients, content/counters, transforms, and `url`, with related issue numbers (#222, #265, #266, #274, #275) called out for known gaps. Three drift guards: `TestAtRulesDocCoverage` AST-greps `parseCSS` for `@`-prefixed string literals; `TestFunctionsDocCoverage` and `TestSelectorsDocCoverage` use static expected lists with behavioural smoke checks against the parsers (#162, #282)

## [0.7.1] - 2026-04-22

C ABI follow-up to v0.7.0. No Go-side behavior changes; every addition is in `export/`. C ABI exports grow from 372 to 388 (+16). The header `export/folio.h` is updated in lockstep — `scripts/audit-cabi.sh` reports Go and header in sync.

### Added

#### Writer optimizer (C ABI)

Handle-based builder so future toggles compose without renaming existing calls. The save and buffer entry points accept a zero handle as "use defaults", so callers that only want the optimizer for a single write do not need to allocate and free an options object.

- **`folio_write_options_new`** / **`folio_write_options_free`**
- **`folio_write_options_set_use_xref_stream`** — ISO 32000-1 §7.5.8
- **`folio_write_options_set_use_object_streams`** — §7.5.7
- **`folio_write_options_set_object_stream_capacity`**
- **`folio_write_options_set_orphan_sweep`**
- **`folio_write_options_set_clean_content_streams`** — §7.8
- **`folio_write_options_set_deduplicate_objects`**
- **`folio_write_options_set_recompress_streams`** — §7.4.4
- **`folio_document_save_with_options`**
- **`folio_document_write_to_buffer_with_options`**

#### Document and per-element setters (C ABI)

- **`folio_document_set_actual_text`** — toggles the `/Span /ActualText` emission for shaped Arabic words (ISO 32000-2 §14.9.4)
- **`folio_paragraph_set_direction`** — `FOLIO_DIR_AUTO` (0) / `FOLIO_DIR_LTR` (1) / `FOLIO_DIR_RTL` (2). Out-of-range values normalize to auto
- **`folio_list_set_direction`**
- **`folio_table_set_direction`**
- **`folio_columns_set_balanced`**

#### Header

- **`FOLIO_DIR_AUTO`, `FOLIO_DIR_LTR`, `FOLIO_DIR_RTL`** preprocessor constants in `export/folio.h` so C consumers do not encode magic numbers for the direction setters

### Test plan

`export/testdata/test_cabi.c` gains 21 new assertions covering: `WriteOptions` lifecycle, all seven setters (happy path and bad-handle rejection), zero-options default path, an end-to-end document write with every optimizer toggle on that confirms the output starts with `%PDF-`, `folio_document_set_actual_text` happy path plus bad-handle, direction setters on Paragraph / List / Table (happy path, normalization of out-of-range codes, bad handle), `folio_columns_set_balanced` happy path and bad-handle. Total `test_cabi` assertions: 390 (384 pre-existing + 6 new blocks).

### Not exposed

Internal-only v0.7.0 additions remain unexported from the C ABI: `core.PdfIndirectReference.SetNum` and `core.PdfStream.WillCompress` (writer-internal); `core.DeflateStreamData` / `core.InflateStreamData` (callers can use any host-language zlib); `core.PdfArray` / `PdfDictionary` / `PdfNumber` / `PdfBoolean` / `PdfString` Go-style accessors; `font.ParseGPOS` / `ParseGSUB` / `ParseKern` and `face.GSUB` / `GPOS` / `GIDToUnicode` (font parser internals); `font.CanEncodeWinAnsiRune`, `EmbeddedFont.EncodeGIDs` / `MeasureGIDs` (shaper-internal); `layout.ShapeArabic` / `ShapeArabicWithFont` / `ShapeDevanagari*`, `ScriptOf` / `SegmentByScript`, `GraphemeBreaks` / `NextGraphemeBreak` / `GraphemeCount`, `FindKashidaCandidates` / `InsertKashidas` (run inside the layout pipeline); `layout.GSUBProvider` / `GPOSProvider` (Go interfaces); `tmpl` package (Go-specific templating). RTL and shaping for HTML-driven workflows continue to flow through `folio_document_add_html` unchanged.
## [0.7.0] - 2026-04-21

No breaking API changes. Every new field, method, and package is additive; zero-value `WriteOptions` and existing constructors produce byte-identical output to v0.6.2. Several bug fixes change the visible output of affected documents — see Visual changes before regression-diffing PDFs.

### Visual changes

- **CSS multi-column fill order** — children are distributed sequentially with height-balanced packing instead of round-robin by index. Matches CSS Multi-column Layout `column-fill: balance` (the spec default). Documents using `column-count` will reflow (#145)
- **Arabic text shaping** — the layout engine uses the font's OpenType GSUB contextual substitutions (init/medi/fina/isol) when present and falls back to the legacy Arabic Presentation Forms-B substitutions only when the font lacks GSUB. v0.6.x rendered Arabic as disconnected isolated forms; v0.7.0 renders connected (#160)
- **TrueType kerning** — fonts whose `kern` table uses Apple-format v0 coverage (Arial and many others) previously returned zero kern for every pair due to a flipped coverage-byte decode. Pairs are now applied; spacing in affected documents tightens (#172)
- **CJK line-breaking** — Japanese, Chinese, and Korean paragraphs break per JIS X 4051 kinsoku shori rules instead of arbitrary character boundaries (#157)
- **Per-glyph font fallback** — runes outside the primary font's coverage previously rendered as `.notdef` boxes; the layout engine selects a fallback face per glyph cluster when one is configured
- **CSS `!important` cascade** at the inline/stylesheet boundary is now honored (#137)
- **`/ActualText` markers around shaped Arabic words** are emitted by default. Adds a small per-word byte cost and improves copy/paste fidelity in PDF readers. Opt out with `Document.SetActualText(false)`

### Deprecated

These remain fully functional in v0.7.0. Plan to migrate before v1.0, when the stable API will be declared and deprecated symbols removed.

- **`core.PdfDictionary.Entries`** direct field access — use `All`, `Get`, `Set`, `Remove`. Direct slice mutation bypasses the lazy key index
- **`core.PdfArray.Elements`** direct field access — use `All`, `At`, `Len`, `Add`, `Set`, `RemoveAt`, `Replace`
- **`core.PdfIndirectReference.ObjectNumber`** — use `Num` for reads and `SetNum` for writes
- **`core.PdfIndirectReference.GenerationNumber`** — use `Gen`

### Discouraged (security)

- **`core.RevisionRC4128`** — RC4 is cryptographically broken; use `RevisionAES256` for new documents. The constant remains supported for reading and writing legacy RC4-encrypted PDFs and is not scheduled for removal

### Added

#### Internationalization

- **Right-to-left text** — bidi paragraph layout per UAX #9 via `golang.org/x/text/unicode/bidi`, character-level bidi splitting, RTL list support (markers on right, text indented from right), HTML `dir` attribute and CSS `direction` property wiring (#37)
- **Arabic shaping** — presentation-forms shaper with full GSUB pipeline for init/medi/fina/isol features, GSUB ligature substitutions (`rlig`, `liga`), kashida (tatweel) justification, `/ActualText` markers (default on) for round-trip-safe copy/paste of shaped words (#37, #160, #179, #180)
- **Devanagari shaping** — Indic shaper for Hindi, Sanskrit, Marathi, and Nepali. Five-phase OpenType pipeline with reordering, half-form substitution, and conjunct formation (#186)
- **CJK line-breaking** — kinsoku shori rules per JIS X 4051 with leading and trailing prohibition sets (#157)
- **Unicode infrastructure** — UAX #29 grapheme clusters (`unicode/grapheme` package), UAX #24 script segmentation, cluster-aware `font.MeasureString` (#170, #176, #183)

#### OpenType infrastructure

- **GSUB LookupType 4** ligature substitutions (#171)
- **GSUB LookupType 6** chaining contextual substitution with depth-bounded action recursion (#174, #184)
- **GPOS LookupType 2** pair adjustment (Format 1 explicit pairs and Format 2 class-based pairs); `face.Kern` consults GPOS first and falls back to the legacy `kern` table (#175)
- **GPOS LookupType 4** mark-to-base anchoring with draw-pipeline wiring; correct diacritic placement for Arabic and Devanagari (#175, #185)
- **TrueType `kern` table** parser hardening: correct v0 / v1 coverage decode, per-face cache, kerning-aware `MeasureString` for both `Standard` and `EmbeddedFont` (#172)

#### Writer optimizer

The `WriteOptions` struct is the single extension point for opt-in writer behavior. Zero value preserves byte-identical output to v0.6.2. Pass dispatch order: orphan sweep → content-stream cleanup → object dedup → stream recompression → encryption → serialization.

- **`document.WriteOptions`** with `Writer.WriteToWithOptions`, `Document.WriteToWithOptions`, `Document.SaveWithOptions`, `Document.ToBytesWithOptions`
- **`UseXRefStream`** — cross-reference stream object (ISO 32000-1 §7.5.8) replacing the traditional xref table and trailer. The xref stream is always written as the last indirect object so its own offset is known before serialization
- **`UseObjectStreams`** — packs eligible indirect objects into compressed object streams (§7.5.7). Implies `UseXRefStream`. Refused on encrypted documents
- **`ObjectStreamCapacity`** — cap on objects packed per `/ObjStm` (default 100)
- **`OrphanSweep`** — drops indirect objects unreachable from `/Root`, `/Info`, `/Encrypt`; renumbers survivors contiguously
- **`RecompressStreams`** — re-Flates eligible payloads at `zlib.BestCompression`, gated by a size-regression guard. Skips DCT/JPX/CCITT/JBIG2 leaf filters, multi-filter chains, and FlateDecode streams carrying `/DecodeParms` (predictor handling per §7.4.4.4)
- **`DeduplicateObjects`** — merges byte-identical indirect objects via SHA-256 of canonical serialization; rewrites references to the canonical survivor; renumbers contiguously. Excludes catalog, `/Info`, `/Encrypt`
- **`CleanContentStreams`** — removes empty `q`...`Q` save/restore pairs and identity `1 0 0 1 0 0 cm` operators (§7.8) from page content streams. Includes a byte-level lexer respecting strings, hex strings, comments, names, arrays, dictionaries, and inline images (BI/ID/EI per §8.9.7)
- **`core.PdfIndirectReference.SetNum`** — object-number setter used by writer-side renumbering passes
- **`core.PdfStream.WillCompress`** — getter matching the existing `SetCompress` setter; lets writer-side passes skip streams the writer will already deflate
- **`core.DeflateStreamData` / `core.InflateStreamData`** — exported zlib codec helpers (formerly the unexported `deflate`)
- **`examples/optimize`** — runnable demo with four fixtures (text-heavy, many empty pages, table-heavy, imported text-heavy) reporting byte-size deltas across the optimizer toggles. Imported fixture: 40,569 → 6,071 bytes (85.0% saved) with the full stack

#### Templates and HTML

- **`tmpl` package** — `html/template` integration: parse a template, execute against caller data, feed the result to the HTML converter (#155)
- **`@font-face` with `src: url(data:font/...)`** base64 data URI fonts (#159)

#### Examples

- **`examples/indic`** — Hindi, Sanskrit, Marathi, Nepali samples (#191)
- **`examples/rtl`** — expanded with full Arabic shaping showcase (#191)
- **`examples/optimize`** — multi-fixture optimizer comparison (#177, #181)
- **Stress tests** for column, grid, flexbox, and SVG layouts (#126, #153)

### Fixed

- **`<br>` inside `<strong>`, `<em>`, `<a>`, and other inline elements** — previously panicked. All 14 inline tags now route line breaks correctly through the `TextRun.IsLineBreak` field (#147, #150)
- **Multi-line headings** no longer overprint wrapped lines (#132)
- **`column-span: all` inside multi-column containers** — correct break handling at leading, trailing, consecutive, and column-boundary positions (#127)
- **Heading overflow tag and `Consumed` accounting on page break** (#139)
- **Gradient `stop-opacity`** plumbed through `layout.GradientStop` to the rasterizer (#146)
- **JPEG parser out-of-bounds** found by fuzz testing (#169)
- **CSS Custom property `var()` references** in `align-items` (#128)

### Audits

- **`core` package** — correctness fixes, hardening, gradual API cleanup; sentinel errors for parser failures (#165)
- **`content` package** — operator validation against ISO 32000 operator set; coverage to 100% on operator emission paths (#166)
- **`image` package** — limits on dimension and component count; fuzz suite for JPEG, PNG, TIFF; CMYK JPEG via Adobe APP14 (#167, #169, #173)
- **`font` package** — sentinel errors, documented concurrency contract, coverage on subset and cmap paths (#168, #173)
- **SVG `preserveAspectRatio`** parsed and honored; barcode fuzz suite (#178)

### Changed

- **Go module dependencies**:
  - `golang.org/x/image` 0.38.0 → 0.39.0
  - `golang.org/x/net` 0.52.0 → 0.53.0
  - Added `golang.org/x/text/unicode/bidi` for UAX #9 bidirectional text
- **`softprops/action-gh-release`** v2 → v3 (release CI)
- **Internal**: `ARCHITECTURE.md` updated with the bidi dependency entry; `font/kern.go` extracted from `truetype.go`; GSUB extracted from the `Face` interface into an optional `GSUBProvider`; `unicode/grapheme` extracted as a leaf package
- **READMEs**: end-to-end HTML-to-PDF benchmarks and performance section; Language SDKs section pointing at Java and WASM ports

### Contributors

- **David Richardson** ([@enquora](https://github.com/enquora)) — stress-test contributions for column, grid, flexbox, SVG layouts (#126)

## [0.6.2] - 2026-04-08

No breaking API changes — all additions below are additive. `layout.Grid.SetAlignContent` keeps its signature; it now also records that the value was explicitly set so implicit "normal" stretching is preserved. The C ABI grows from 348 to 372 exports, all additive.

### Added

- **SVG `<image>` elements with data-URI raster sources** — `<image href="data:image/png;base64,...">` inside an `<svg>` now decodes and draws the embedded PNG/JPEG/WebP/GIF. Missing `width`/`height` fall back to the raster's intrinsic pixel dimensions (#130)
- **Real SVG gradients** — `linearGradient` and `radialGradient` referenced via `fill="url(#id)"` are now rasterized and drawn clipped to the shape instead of collapsing to the first stop color (#130)
- **`svg.RenderOptions.RegisterImage`** — new callback for wiring an external image decoder and XObject registrar from SVG `<image>` elements, keeping the svg package free of image-format dependencies (#130)
- **`svg.RenderOptions.RegisterGradient`** — new callback for rasterizing SVG gradients into PDF image XObjects; nil callback preserves the legacy first-stop fallback (#130)
- **`svg.Node.LinearGradient()` / `svg.Node.RadialGradient()`** — accessors returning parsed gradient definitions (`LinearGradientInfo` / `RadialGradientInfo`) with resolved `Stop` values, so callers can drive an external rasterizer without reparsing SVG syntax (#130)
- **`svg.BBox`** — new exported type representing an axis-aligned bounding box in SVG local coordinates, used as the shape bounding box passed to `RegisterGradient` (#130)
- **24 new C ABI exports** — `folio_document_to_bytes`, `folio_document_validate_pdfa`, Div extras (`set_aspect_ratio`, `set_keep_together`, `set_border_radius_per_corner`, `set_width_percent`, `set_hcenter`, `set_hright`, `set_clear`, `set_outline`, `add_box_shadow`), Cell extras (`set_border_radius`, `set_border_radius_per_corner`), Grid extras (`set_border`, `set_borders`, `set_template_areas`), Flex extras (`set_align_content`, `set_borders`), `paragraph_set_text_align_last`, Image extras (`set_object_fit`, `set_object_position`), `run_list_last_set_background_color`, `signer_new_pkcs12`, `parse_css_length`. Total C ABI: 372 exports, up from 348 (#143)

### Fixed

- **Inline `<svg>` / `<img>` dropped from paragraph flow** — replaced elements inside a `<p>` inherited `display: block` from the parent and were silently skipped by the inline collector. They now default to `display: inline` in `applyTagDefaults`, matching browser replaced-element behavior. Affects any `<p>` containing a bare `<svg>` or `<img>` without an explicit inline `display` override (#130)
- **Inline `<strong>` / `<em>` / `<span>` / `<a>` split paragraphs into three** — `<div>text <strong>bold</strong> more text</div>` produced three stacked paragraphs with the trailing punctuation orphaned on a new line. Two interacting bugs: text-emphasis tags (`strong`, `em`, `span`, `b`, `i`, `u`, `s`, `del`, `mark`, `small`, `sub`, `sup`, `code`, `a`) inherited `display: block`, and `walkChildren` didn't group consecutive inline siblings into anonymous block boxes per CSS 2.1 §9.2.1.1. `walkChildren` now buffers consecutive inline flow children and flushes them as a single paragraph (#142)
- **`<td style="width:50%">` overflow in narrow flex columns** — cell width percentages were resolved against the converter's outer container width instead of the table's actual layout width, producing absolute hints much larger than the table could hold and cells that ran off the page. Percentage widths are now deferred to layout time where the real table width is known (#142)
- **CSS Grid `align-items`, `justify-items`, and container height** — explicit `height` on a grid container is now honored so `align-items` / `align-content` have room to distribute; `justify-items` values (`start`/`end`/`center`/`stretch`) are applied; explicit `align-content: flex-start` no longer triggers row stretching (#129)
- **Flexbox `order` property** — children are stable-sorted by their CSS `order` value before layout; ties preserve DOM order per Flexbox spec. `align-items` now resolves `var(--custom-property)` references through the cascade (#128)
- **Table sizing preserved across page-break continuations** — `min-width`, auto-column widths with cell hints, and `border-collapse`/`border-spacing` now survive when a table splits across pages (#134)
- **Multi-line headings no longer overprint wrapped lines** — `Heading.PlanLayout` allocates per-line boxes so long headings that wrap at narrow widths render cleanly without glyph overlap (#132)
- **`column-span: all` inside multi-column containers** — elements with `column-span: all` now break the column flow correctly at leading, trailing, consecutive, and column-boundary positions, including when `column-rule` is set (#127)
- **Heading overflow tag + `Consumed` accounting on page break** — headings that split across pages carry the correct structure tag on continuation and no longer over-advance the available height (#139)

## [0.6.1] - 2026-04-05

### Added

- **CSS `aspect-ratio`** property on Div elements (CSS Sizing Level 4 §5.1) — derives height from width when no explicit height is set; supports `16 / 9`, `auto 16/9`, single number forms (#112)
- **CSS Color Level 4** space-separated `rgb()`/`hsl()` syntax — `rgb(255 0 0 / 0.5)`, percentage alpha, applies to `rgba()`/`hsla()` too (#108)
- **`html.ParseCSSLength`** public utility — converts CSS length strings (`"1in"`, `"16px"`, `"2em"`, `"50%"`, `calc()`) to PDF points (#109)
- **`Document.ToBytes`** convenience method — returns serialized PDF as `[]byte` for HTTP responses, base64 encoding, in-memory processing (#66)
- **Per-side border-radius on table cells** — `drawCellBordersRounded` now draws each border side independently with corner arcs, instead of requiring all four borders to be identical (#115)
- **WASM header/footer** — `folioRender` accepts `headerHtml`/`footerHtml` in settings JSON, rendered via `SetHeaderElement`/`SetFooterElement` (#102)
- **Invoice example** (`examples/invoice/`) — professional invoice PDF demonstrating rounded table headers, CSS Grid, Flexbox, and optional Tailwind CSS v2

### Fixed

- **Nil font panic** in `runMeasurer` — falls back to Helvetica when `TextRun` has no font set (#98)
- **Table default `border-collapse`** changed from `collapse` to `separate` per CSS 2.1 §17.6 — previously prevented cell border-radius from rendering (#114)
- **Table default margins** removed — browsers set zero margins on `<table>`; added `border-spacing: 2px` default per browser UA stylesheets (#117)
- **Div `drawRoundedBorders`** now uses per-corner radii (`RoundedRectPerCorner`) instead of uniform radius; previously only `r[0]` was used (#104)
- **`:not([hidden])` selector** — attribute selectors inside pseudo-class parens were incorrectly extracted, leaving empty `:not()` that always returned false; enables CSS framework `space-y-*` utilities (#101)
- **`rem` unit parsing** — `parsePlainLength` checked `"em"` suffix before `"rem"`, so `"1rem"` failed to parse (#111)
- **Table cell border-radius in HTML** — converter now skips radius wiring in `border-collapse: collapse` mode per CSS Backgrounds Level 3 §5.3 (#100)
- **README Go version** corrected from 1.21+ to 1.25+ to match go.mod (#124)

### Visual change

- **Table borders now render in `separate` mode by default** (previously `collapse`). Tables without explicit `border-collapse: collapse` in CSS will show individual cell borders instead of shared borders. This matches browser behavior per CSS 2.1 §17.6. To restore the old behavior, add `table { border-collapse: collapse; }` to your CSS.

### Changed

- **Layout test coverage** 70% → 77.9% — 40 new integration tests for draw functions, table rendering, Div features, Grid layout, Flex column, paragraph indent/ellipsis/orphans
- **Playground URL** updated to `playground.foliopdf.dev`

## [0.6.0] - 2026-04-03

**Breaking changes** — see [MIGRATING.md](MIGRATING.md) for upgrade steps.

### Breaking

- Renamed constructors to `New*`/`Load*`/`Parse*` across `reader`, `barcode`, `layout`, `sign`, `forms`
- `sign.LoadPKCS12` renamed to `sign.ParsePKCS12` (same signature, name-only change)
- `Document.Page(index)` returns `(*Page, error)` instead of panicking
- Unexported internal symbols in `reader` and `svg`
- Baseline positioning uses CSS half-leading with actual font metrics (visual change — text shifts up ~4pt for 12pt Helvetica)
- `vertical-align` accepts length/percentage values (previously ignored)

### Added

- Element-based headers/footers: `SetHeaderElement`, `SetFooterElement`, `SetHeaderText`, `SetFooterText`
- `AddHTMLTemplate` / `AddHTMLTemplateFuncs` for Go template → PDF
- `ValidatePdfA` for early PDF/A validation
- Per-run text highlight (`WithBackgroundColor`, `<mark>` in HTML)
- Inline elements in paragraphs (`<img>`, `<svg>`, `display:inline-block`)
- `<sub>` and `<sup>` rendering with baseline shift and correct spacing
- `baseline-shift` CSS property (keywords and lengths/percentages)
- `vertical-align` extended with length/percentage values per CSS 2.1
- Empty lines from consecutive `\n\n` in paragraphs
- `RunInline` for inline layout elements within paragraphs
- CSS: `text-align-last`, `::marker`, `cmyk()`, `object-fit`, `@supports`, `min()`/`max()`/`clamp()`, `:is()`/`:where()`, repeating gradients, `column-width`/`column-rule`, `string-set`/`string()`, `page-break-inside: avoid`, escape sequences in selectors, multiple `box-shadow`
- WebP and GIF image formats

### Fixed

- `<sub>`/`<sup>` baseline shift — previously only reduced font size (#86)
- Adjacent styled runs no longer insert spurious spaces; inline whitespace collapsing per CSS Text Level 3 §4.1.1 (#86)
- Punctuation after `</sup>`/`</sub>` keeps correct styling (#86)
- Punctuation at font boundaries keeps its own font (#30)
- Consecutive `\n\n` produce visible empty lines (#91)
- Blank lines preserved across page splits (#95)
- Paragraph baseline uses CSS half-leading `(lineH + ascent - descent) / 2` (#90)
- `cloneWithWords` preserves all Word styling + line breaks on page-split paragraphs
- Style changes at line break boundaries preserved during page splits
- Overflow handling includes following siblings in Div layout (#13)
- Table layout handles zero/negative height without panicking
- Inline-block SVG/IMG dispatch to correct converters (#71)
- Inline element alignment: line-relative child positions (#71)
- `buildParagraphFromRuns` always uses `NewStyledParagraph` (was dropping `BaselineShift`/`BackgroundColor`)
- RGB color components clamped to 0–1
- Case-insensitive attribute selector matching
- Alpha premultiplication fix for PNG
- Font descriptor flags from actual metadata
- Kern format 0 nPairs validated
- Encrypted PDFs detected with clear error
- Signatures preserved on multi-sign PDFs
- Highlight/underline/strikethrough use actual font metrics (#73)
- Predictor column count bounded to prevent allocation DoS

### Contributors

- **Ben Davidson** ([@bendavidsonku](https://github.com/bendavidsonku)) — inline elements in paragraphs, per-run text highlight background (#71, #72)
- **Jason Kulatunga** ([@AnalogJ](https://github.com/AnalogJ)) — table zero-height fix, overflow sibling handling (#13)

### Changed

- `golang.org/x/image` v0.37.0 → v0.38.0
- Internal: `html/converter.go` split into focused modules
- Internal: `ARCHITECTURE.md` with design principles, layering rules, naming conventions

## [0.5.2] - 2026-03-26

### Added
- **PDF redaction** — `RedactText`, `RedactPattern`, `RedactRegions` permanently remove text from content streams with character-level TJ splitting precision; configurable fill color, overlay text, and metadata stripping (#59)
- **Page import** — `Page.ImportPage` and `Page.ImportPageWithOpts` load existing PDF pages as Form XObjects (ISO 32000 §8.10) for template workflows; `reader.ExtractPageImport` convenience API with full indirect-ref resolution (#47)
- **Drawing primitives** — `Page.AddLine`, `Page.AddRect`, `Page.AddRectFilled` for low-level graphics on pages
- **PDF/UA accessibility** — alt text for images, custom structure tags, structure tree reading from existing PDFs (#60)
- **Paragraph `\n` line breaks** — `\n`, `\r\n`, and `\r` now produce forced line breaks in paragraphs and table cells (#61, #63)
- **C ABI expanded to 346 functions** (up from 330) — adds redaction, page import, drawing, digital signatures, encryption permissions, page manipulation, content extraction, form flattening, merge, TextRun builder, styled list/heading exports
- **Examples** — `merge/` (parse, merge, extract text), `sign/` (PAdES B-B digital signature), `report/` (multi-page layout API), `import-page/` (external PDF template filling), `redact/` (sensitive text removal)

### Fixed
- **Import page blank output** — `resolveDeep` recursively resolves all indirect references in imported resources; `hoistStreams` converts nested PdfStream objects to indirect refs, fixing blank/partial output with real-world PDFs
- **Paragraph `\n` collapsed to space** — `splitWords` now splits on newlines first and inserts forced line break markers

## [0.5.1] - 2026-03-25

### Fixed
- **Release workflow** — replaced deprecated `macos-13` runner with `macos-latest` for x86_64 builds
- **Fuzz test regex** — anchored `-fuzz='^FuzzParse$'` to avoid matching `FuzzParsePDF`

## [0.5.0] - 2026-03-25

### Contributors

- **Marc Ole Bulling** — `<br>` nil pointer fix (#10)
- **Moritz** ([@FrauElster](https://github.com/FrauElster)) — PDF/A-3b file attachments (#17)
- **Piotr Pawlak** ([@piotrxp](https://github.com/piotrxp)) — SSRF prevention for remote resources (#39)

### Added
- **C ABI expanded to 281 functions** (up from 115) — covers nearly all Go engine features
- **Barcode C ABI** — `folio_barcode_qr`, `qr_ecc`, `code128`, `ean13` + layout elements
- **SVG C ABI** — `folio_svg_parse`, `parse_bytes` + layout elements with size/align
- **Link C ABI** — hyperlink, embedded font, and internal link layout elements
- **Flex C ABI** — full flexbox container with items, direction, justify, align, wrap, gap, borders
- **Grid C ABI** — CSS Grid with template columns/rows, auto-rows, placement, justify/align items/content
- **Columns C ABI** — multi-column layout with gap and custom widths
- **Float C ABI** — left/right floating elements with margin
- **TabbedLine C ABI** — tab-stop text with dot leaders for TOC-style layouts
- **Form filling C ABI** — `folio_form_filler_new`, `set_value`, `set_checkbox`, `field_names`, `get_value`
- **Form field builder C ABI** — `folio_form_create_text_field`, `create_checkbox` + `set_value`, `set_read_only`, `set_required`, `set_background_color`, `set_border_color`, then `add_field`
- **Additional form fields** — multiline text, password, listbox, radio group
- **Document watermark** — `folio_document_set_watermark` and `set_watermark_config`
- **Outlines/bookmarks C ABI** — `folio_document_add_outline`, `add_outline_xyz`, `outline_add_child`
- **Named destinations** — `folio_document_add_named_dest`
- **Viewer preferences** — `folio_document_set_viewer_preferences`
- **Page labels** — `folio_document_add_page_label`
- **File attachments** — `folio_document_attach_file` for PDF/A-3b compliance
- **Inline HTML** — `folio_document_add_html` and `add_html_with_options`
- **Page-specific margins** — `folio_document_set_first_margins`, `set_left_margins`, `set_right_margins`
- **Absolute positioning** — `folio_document_add_absolute`
- **Page extensions** — art box, page size override, page-to-page links, text annotations, text markup annotations (highlight, underline, squiggly, strikeout), separate fill/stroke opacity
- **All 14 standard font accessors** — added `helvetica_oblique`, `helvetica_bold_oblique`, `times_italic`, `times_bold_italic`, `courier_bold`, `courier_oblique`, `courier_bold_oblique`, `symbol`, `zapf_dingbats`
- **Paragraph extensions** — orphans, widows, ellipsis, word-break, hyphens
- **Table extensions** — footer rows, cell spacing, auto column widths, min width
- **Cell extensions** — per-side padding, vertical alignment, borders, width hints
- **Div extensions** — border radius, opacity, overflow, max/min width, box shadow, max height
- **List extensions** — leading, nested sub-lists
- **Image extensions** — TIFF loading, element alignment
- **C ABI audit script** (`scripts/audit-cabi.sh`) — detects drift between Go exports, folio.h, and built symbols
- **C integration tests** — 258 tests covering all exported functions

### Changed
- **Library version injected at build time** via `-ldflags "-X main.version=..."` — `folio_version()` returns the git tag in releases, `git describe` in dev builds
- **CI** — added C ABI audit, shared library build, and C integration test steps
- **Release workflow** — builds native shared libraries for 5 platforms (linux-x86_64, linux-aarch64, macos-x86_64, macos-aarch64, windows-x86_64) with SHA256 checksums
- **Makefile** — added `audit`, `audit-build`, cross-compilation targets (`cross-linux-amd64`, etc.), OS-aware shared library extension detection

### Fixed
- **`<br>` nil pointer** — fixed nil pointer exception with `<br>` tags (#10)
- **PDF/A-3b file attachments** — proper embedded file streams with `/AF`, `/Names`, MIME types (#17)
- **ZUGFeRD lint** — deterministic timestamps, fixed example (#41)
- **Content stream compression** — FlateDecode for content streams, merge stream dict bug (#44)
- **SSRF prevention** — URL policy interceptor for blocking/modifying remote resource requests (#39)
- **Radio button / checkbox appearance** — fixed appearance stream generation for form fields
- **`:root` selector** — now correctly matches the `<html>` element
- **Gradient rendering** — fixed CSS gradient parsing and rendering
- **Page number CSS counter** — `counter(page)` and `counter(pages)` now work in margin boxes
- **MarginBox API** — exposed `MarginBoxes`/`FirstMarginBoxes` on `ConvertResult` for simpler programmatic access (#54)

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

### Contributors

- **Emrecan BATI** — Apache 2.0 license text cleanup

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
