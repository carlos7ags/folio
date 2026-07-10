# Migrating

For the full list of new features and fixes, see [CHANGELOG.md](CHANGELOG.md).

---

## Upgrading from v0.9.x to v0.10.0

### `font.Face` gains `GSUB()`, `GPOS()`, and `GIDToUnicode()`; `GSUBProvider`/`GPOSProvider` are removed

Shaping support used to be an optional side-interface a `Face` could
implement, discovered by callers via type assertion. Both provider
interfaces are gone; `Face` now declares the three methods directly, so
every implementation must provide them. Return nil to mean "no data for
this font" — that was already the nil-return contract for the removed
interfaces.

Migration for external `Face` implementers:

```go
// before
type myFace struct{ /* ... */ }

// (GSUB/GPOS/GIDToUnicode only needed to satisfy the optional
// GSUBProvider/GPOSProvider interfaces)

// after
type myFace struct{ /* ... */ }

func (f *myFace) GSUB() *font.GSUBSubstitutions { return nil }
func (f *myFace) GIDToUnicode() map[uint16]rune { return nil }
func (f *myFace) GPOS() *font.GPOSAdjustments   { return nil }
```

Migration for callers that probed for shaping support:

```go
// before
gp, ok := face.(font.GSUBProvider)
if !ok {
    // no shaping data
}
gsub := gp.GSUB()

// after
gsub := face.GSUB()
if gsub == nil {
    // no shaping data
}
```

### Remote and absolute-path assets are now opt-in

`html.Convert` / `html.ConvertFull` no longer fetch remote or
absolute-path assets by default. A document's `http(s)` references
(`<img>`, `background-image: url()`, linked stylesheets,
`@font-face url()`) are fetched only when `Options.AllowRemoteFetch` is
set, and absolute filesystem paths in document content are read only when
`Options.AllowAbsolutePaths` is set. A blocked reference fails with
`ErrRemoteFetchDisabled` or `ErrAbsolutePathDenied` — logged, and
surfaced through the joined error under `StrictAssets`.

When remote fetch is enabled with no `URLPolicy`, the built-in
`DenyInternalHosts` policy blocks loopback, RFC1918, link-local, and
non-http(s) targets and is re-checked on every redirect hop. Set
`URLPolicy` to a function returning nil to allow every host.

```go
// before — remote images fetched with no filtering
opts := &html.Options{}

// after — enable remote fetch; internal hosts blocked by default
opts := &html.Options{AllowRemoteFetch: true}

// after — allow every host, including internal ones
opts := &html.Options{
    AllowRemoteFetch: true,
    URLPolicy:        func(string) error { return nil },
}
```

`Options.FallbackFontPath` and the built-in system-font search load
absolute paths through a separate trusted code path and are unaffected.

A conversion now also enforces an aggregate asset-size budget via the new
`Options.MaxTotalAssetBytes` (default 512 MiB when 0), covering every
asset it reads — images, fonts, and stylesheets alike. Documents within
the default are unaffected; raise it for asset-heavy documents (many
large embedded fonts) or lower it to tightly bound untrusted input. A
crossed budget fails the offending load with `ErrAssetBudgetExceeded`.

### C ABI: `folio_last_error` is now caller-owned

`folio_last_error` previously returned a `char*` pointing at
library-internal storage, valid only until the next C ABI call; callers
were expected to treat it as borrowed and never free it. It now returns
a fresh copy that the caller must release with the new
`folio_string_free`.

A caller that keeps the old assumption does not crash — it leaks one
short error-message string per call to `folio_last_error` until it
adopts `folio_string_free`.

```c
/* before */
const char *msg = folio_last_error();
printf("%s\n", msg);
/* msg was borrowed; nothing to free */

/* after */
char *msg = folio_last_error();
printf("%s\n", msg);
folio_string_free(msg); /* NULL is a no-op */
```

`folio_version`'s return value is unaffected and must still not be
passed to `folio_string_free`.

## Upgrading from v0.7.x to v0.8.0

### `html.Options.BasePath` is removed

`BaseFS` is now the single way to resolve every local asset referenced
by an HTML document — `<img src>`, `<link href>`, `@font-face url()`,
`background-image: url()`, and `Options.FallbackFontPath`.

Migration:

```go
// before
opts := &html.Options{BasePath: "./assets"}

// after
opts := &html.Options{BaseFS: os.DirFS("./assets")}
```

Any `fs.FS` works:

| Source                | Example                                  |
|-----------------------|------------------------------------------|
| Embedded assets       | `BaseFS: assetsFS // //go:embed assets`  |
| On-disk directory     | `BaseFS: os.DirFS("./assets")`           |
| Sandboxed directory   | `r, _ := os.OpenRoot("./assets"); BaseFS: r.FS()` |
| In-memory (tests)     | `BaseFS: fstest.MapFS{...}`              |

### Path semantics changed

Paths in the document are normalised to `fs.FS` conventions before the
read: forward slashes only, no leading `/`, no `..` traversal — invalid
paths are rejected before the open.

A leading `/` in `src`/`href` is now treated as web-style root of the
`BaseFS` (matching how `<base href="/">` works in browsers) instead of
an absolute filesystem path. If you previously relied on absolute
filesystem paths, mount the directory containing them as `BaseFS` and
use root-relative `/`-prefixed paths in the document.

When `BaseFS` is nil, every local-asset reference fails — the document
must inline its assets via `data:` URIs.

### `@font-face` resolves relative to its containing stylesheet

A linked stylesheet at `css/site.css` containing
`@font-face { src: url(../fonts/Inter.ttf); }` now resolves to
`fonts/Inter.ttf` from the `BaseFS` root — matching browser behavior.
Previously the URL resolved from the document root regardless of where
the stylesheet lived.

If your `@font-face` rules currently rely on the old root-anchored
behavior, either:

- use a root-relative path (`url(/fonts/Inter.ttf)`), or
- move the rule into an inline `<style>` block in the document.

HTTP-origin stylesheets resolve relative URLs as HTTP, FS-origin
stylesheets resolve them through `BaseFS`. Inline `<style>` blocks
continue to resolve relative URLs from the `BaseFS` root.

### Surfacing asset-load errors

A new optional `Options.Logger` (`*slog.Logger`) receives warn-level
events when an asset fails to load: missing `@font-face` files,
unreadable linked stylesheets, image fetches that fall back to alt
text. Previously these were silently swallowed. To surface them during
development:

```go
opts := &html.Options{
    BaseFS: os.DirFS("./assets"),
    Logger: slog.New(slog.NewTextHandler(os.Stderr, nil)),
}
```

The default remains silent, so existing callers see no behavior change
unless they opt in.

### `tmpl.RenderFile` and `tmpl.RenderFileTo`

These helpers now auto-populate `BaseFS` to `os.DirFS(filepath.Dir(templatePath))`
when the caller leaves it nil, so a template referencing `<img src="logo.png">`
next to itself resolves without extra wiring. Callers that already pass a
`BaseFS` keep their value.

### C ABI

The signature of `folio_document_add_html_with_options` is unchanged —
it still takes a `basePath` C string and wraps it as
`os.DirFS(basePath)` internally at the boundary. C consumers see no
breaking change to existing exports.

The C ABI surface grows from 388 to 393 (+5):

- **`folio_document_set_language(doc, lang)`** — BCP-47 / RFC 3066 tag.
  Required for any PDF/A Level A variant per ISO 19005-2/3 §6.7.2.
- **`FOLIO_PDFA_3A`, `FOLIO_PDFA_4`, `FOLIO_PDFA_4F`, `FOLIO_PDFA_4E`** —
  new PDF/A profile constants. Existing `FOLIO_PDFA_*` numeric values
  are preserved.
- **`folio_paragraph_measure_lines(p, max_width)`** /
  **`folio_paragraph_measure_height(p, max_width)`** — wrap the new
  `Paragraph.MeasureLines` / `MeasureHeight` helpers. Useful for
  clamp/truncate decisions before rendering.
- **`folio_paragraph_split_after_line(p, n, max_width, *head, *tail)`** —
  splits a paragraph after the first `n` lines at `max_width`. Returns
  two handles via out-pointers; either may be `0` (no-op halves at
  `n <= 0` or `n >= total`). Receiver is unchanged. Caller frees the
  non-zero halves via `folio_paragraph_free`.
- **`folio_font_parse_for_language(data, length, lang)`** — TTC face
  selection by BCP-47 tag for pan-CJK font collections (NotoSansCJK,
  Source Han Sans, msgothic, etc.). NULL `lang` falls back to face 0,
  matching `folio_font_parse_ttf` semantics.

`scripts/audit-cabi.sh` reports Go `//export` directives, header
declarations, and built dylib symbols all in sync at 393.

### `layout.UnitValue` is no longer comparable with `==`

`UnitValue` now holds a func field for the new `UnitCalc` variant —
the closure that resolves CSS `calc()` lengths against the actual
layout area at render time. As a consequence the struct is no longer
comparable with `==`. Out-of-tree code that compared `UnitValue`
values directly will see a compile error.

```go
// before
if a == b { ... }

// after — compare the variant tag and the resolved value separately
if a.Type == b.Type && a.Pt(width) == b.Pt(width) { ... }
```

No in-tree consumer compared `UnitValue` for equality, so the change
is contained to consumers that did so deliberately.

### `layout.BackgroundImage.Position` is now `[2]layout.ResolvableLength`

The field was `[2]float64`, resolved eagerly at parse time. It is now
`[2]layout.ResolvableLength`, resolved at draw time against the
background box. The struct also gains a sibling `FontSize float64`
field so `em` / `rem` leaves in `background-position` resolve against
the originating element's font size.

`ResolvableLength` is a one-method interface:

```go
type ResolvableLength interface {
    Resolve(container, fontSize float64) float64
}
```

Out-of-tree code that constructs `BackgroundImage` directly must
implement it for each axis. A pt-typed wrapper is the smallest viable
shape:

```go
type ptLen float64

func (p ptLen) Resolve(_, _ float64) float64 { return float64(p) }

// before
bg := &layout.BackgroundImage{
    Image:    img,
    Position: [2]float64{10, 20},
}

// after
bg := &layout.BackgroundImage{
    Image:    img,
    Position: [2]layout.ResolvableLength{ptLen(10), ptLen(20)},
    FontSize: 12, // only consulted by em / rem leaves
}
```

The HTML converter constructs `BackgroundImage` internally and routes
its own parsed `cssLength` through `Position`, so callers that go
through `html.Convert` / `html.ConvertFull` see no source change.

### `html.ErrURLPolicyDenied` message text changed

The exported `var` identity is unchanged, so `errors.Is(err,
html.ErrURLPolicyDenied)` continues to work. The wrapped error message
changed from `"folio/html: ..."` to `"html: ..."` as part of the
package-prefix standardization. Callers that compared `err.Error()`
verbatim against the old string need to update their assertion. The
same standardization touches ~130 internal helper errors across the
tree; none are exported, so the impact is limited to test fixtures
that string-matched against the previous messages.

### Visual changes

Several v0.8.0 fixes change the visible output of affected documents.
Review these before regression-diffing PDFs against a v0.7.x baseline:

- **GPOS cursive horizontal placement** — Fonts with cursive joining
  (Arabic and several Indic scripts) tighten in horizontal direction.
  The previous draw path applied the entry/exit X delta as a `Td`
  shift on top of the glyph's natural advance, landing each joined
  glyph one extra advance past its predecessor. Per OpenType §6.3,
  in horizontal text the X component of the cursive join is already
  encoded by hmtx; the feature only aligns the join in Y. Cursive
  remains LTR-only (#220).
- **CJK paragraphs across page breaks** — Continuation pages of a
  CJK paragraph that paginated in v0.7.x rendered every ideograph
  with a spurious ASCII space (`"中文文本"` → `"中 文 文 本"`) and
  re-wrapped to MORE lines than the original. v0.8.0 honours the
  `SpaceAfter=0` boundary that `breakCJKWords` emits, so the
  continuation matches the source. Affected documents will reflow
  on the continuation pages (#246).
- **GPOS Type 5 mark-to-ligature fallback** — Ligatures with three
  or more components now fall back to Type 4 mark-to-base instead of
  applying middle-component anchors blindly. Without per-cluster
  component attribution from the shaper, the middle-component path
  silently misplaced marks. Two-component ligatures (the common case
  for Arabic lam-alef and Latin "fi" / "ffl") are unaffected (#220).
- **Unicode NFC normalization** — Strings that arrived in canonically
  decomposed form (e.g. `e` + combining acute `U+0301` instead of
  `é`) are now normalised to NFC at every `layout` entry point.
  Most input is already NFC, so the majority of documents are
  byte-identical. Affected documents see corrected widths and
  shaping — font cmap tables that only cover precomposed codepoints
  stop falling through to `.notdef` (#217).
- **`font-weight: 600` against PDF-14 standard fonts now picks Bold** —
  pre-fix, the binary-string parser collapsed every numeric weight
  to `"normal"` or `"bold"` with the boundary at 700, so
  `font-weight: 600` against Helvetica rendered as Helvetica
  (Regular). Post-fix, weights ≥ 600 round to the Bold variant per
  CSS Fonts L4 §5.2's synthetic-bolding guidance; standard fonts
  ship Regular and Bold variants only, so 600 → Bold is the only
  reasonable rounding. Documents that wrote `font-weight: 600`
  against the standard families and expected Regular weight will
  see headings tighten visually. The fix also unblocks proper
  SemiBold/Medium rendering when the family has matching
  `@font-face` declarations — a document declaring four Inter
  weights at 400/500/600/700 and writing `font-weight: 600`
  previously silently picked Inter-Regular and now correctly picks
  Inter-SemiBold (#286).
- **`<th>` honours explicit `text-align: left` instead of silently
  re-centering** — pre-fix, the table-cell default-center heuristic
  gated on `cellStyle.TextAlign == AlignLeft`, so `th { text-align:
  left }` got the explicit choice overridden because the resolved
  alignment matched the re-center sentinel. Post-fix, the gate
  also requires `!cellStyle.TextAlignSet`, so explicit author
  choice (left, or `start` / `end` resolving to left under the
  current direction) is preserved. Default `<th>` still centers.
  Documents that wrote `th { text-align: left }` and expected the
  silent override will see headers shift from center to left (#288).
- **CSS `border: ... ridge|groove|inset|outset` now renders the 3D
  bevel instead of a flat solid line** — pre-fix the four bevel
  keywords parsed and stored on the computed style but the renderer
  fell through to `solid`, dropping the modulation entirely. Post-fix
  each side renders with a per-side dark/light color shift (groove/
  inset → top+left dark, bottom+right light; ridge/outset → opposite)
  per CSS Backgrounds L3 §4.1. Documents that used these styles will
  see the bevel they originally intended; documents that relied on
  the silent flatten will see colored modulation. The renderer uses
  a single solid stroke per side with side-uniform modulation rather
  than the strict spec's two-half-width split bevel — visually
  indistinguishable for thin borders, less pronounced for thick
  ones (#290).

### Asset resolution side-effects

`<img src="https://...svg">` and inline-SVG `<img>` URLs now flow
through `Options.URLPolicy` enforcement uniformly with all other HTTP
asset fetches (#229). If you have a `URLPolicy` set, double-check that
it permits the SVG hosts your documents reference; previously these
two routes bypassed the policy.

`@font-face url('/abs/system/font.ttf')` now succeeds with `BaseFS:
nil` through the centralised `resolveLocalAsset` contract. Documents
that rely on absolute system font paths no longer need a `BaseFS`
mount around `/`.

`Options.FallbackFontPath` retains a documented programmatic-only
carve-out: an absolute path always bypasses `BaseFS` to the OS, and a
relative path that misses `BaseFS` retries against the OS. Neither
extends to document-supplied references, since the trust boundary is
different — only programmatic, caller-supplied paths get the OS
retry.

---

## Upgrading from v0.6.x to v0.7.0

No code changes are required. Every new API is additive and the
zero-value `document.WriteOptions` reproduces byte-identical output to
v0.6.2.

Several bug fixes change the visible output of affected documents.
Review these before regression-diffing PDFs against a v0.6.x baseline:

- **CSS multi-column fill order** — sequential balanced fill replaces
  round-robin distribution (#145).
- **Arabic text shaping** — OpenType GSUB contextual shaping replaces
  the isolated-forms fallback when the font has GSUB positional
  features (#160).
- **TrueType kerning** — Apple-format v0 coverage (Arial and others)
  now returns non-zero kern pairs; spacing tightens (#172).
- **CJK line-breaking** — JIS X 4051 kinsoku shori rules (#157).
- **Per-glyph font fallback** — runes outside the primary font's
  coverage route to a configured fallback face.
- **CSS `!important` cascade** at the inline/stylesheet boundary
  is now honored (#137).
- **`/ActualText` markers around shaped Arabic words** emitted by
  default; opt out with `Document.SetActualText(false)`.

Deprecated direct-field access on `core.PdfDictionary.Entries`,
`core.PdfArray.Elements`, and `core.PdfIndirectReference.ObjectNumber` /
`.GenerationNumber` remains supported in v0.7.0 and is scheduled for
removal at v1.0. Migration targets are listed in the Deprecated section
of the CHANGELOG.

---

## Upgrading from v0.5.x to v0.6.0

## 1. Rename constructors

All constructors now follow `New*` / `Load*` / `Parse*` conventions.

Run this to fix all renames automatically:

```bash
find . -name '*.go' -exec sed -i '' \
  -e 's/reader\.Open(/reader.Load(/g' \
  -e 's/barcode\.QRWithECC(/barcode.NewQRWithECC(/g' \
  -e 's/barcode\.QR(/barcode.NewQR(/g' \
  -e 's/barcode\.Code128(/barcode.NewCode128(/g' \
  -e 's/barcode\.EAN13(/barcode.NewEAN13(/g' \
  -e 's/layout\.RunEmbedded(/layout.NewRunEmbedded(/g' \
  -e 's/layout\.Run(/layout.NewRun(/g' \
  -e 's/sign\.LoadPKCS12(/sign.ParsePKCS12(/g' \
  -e 's/forms\.MultilineTextField(/forms.NewMultilineTextField(/g' \
  -e 's/forms\.PasswordField(/forms.NewPasswordField(/g' \
  -e 's/forms\.SignatureField(/forms.NewSignatureField(/g' \
  -e 's/forms\.TextField(/forms.NewTextField(/g' \
  -e 's/forms\.Checkbox(/forms.NewCheckbox(/g' \
  -e 's/forms\.Dropdown(/forms.NewDropdown(/g' \
  -e 's/forms\.ListBox(/forms.NewListBox(/g' \
  -e 's/forms\.RadioGroup(/forms.NewRadioGroup(/g' \
  {} +
```

Full rename table:

| Old | New |
|-----|-----|
| `reader.Open(path)` | `reader.Load(path)` |
| `barcode.QR(data)` | `barcode.NewQR(data)` |
| `barcode.Code128(data)` | `barcode.NewCode128(data)` |
| `barcode.EAN13(data)` | `barcode.NewEAN13(data)` |
| `barcode.QRWithECC(data, level)` | `barcode.NewQRWithECC(data, level)` |
| `layout.Run(text, font, size)` | `layout.NewRun(text, font, size)` |
| `layout.RunEmbedded(text, ef, size)` | `layout.NewRunEmbedded(text, ef, size)` |
| `sign.LoadPKCS12(data, password)` | `sign.ParsePKCS12(data, password)` |
| `forms.TextField(...)` | `forms.NewTextField(...)` |
| `forms.Checkbox(...)` | `forms.NewCheckbox(...)` |
| `forms.Dropdown(...)` | `forms.NewDropdown(...)` |
| `forms.ListBox(...)` | `forms.NewListBox(...)` |
| `forms.RadioGroup(...)` | `forms.NewRadioGroup(...)` |
| `forms.PasswordField(...)` | `forms.NewPasswordField(...)` |
| `forms.MultilineTextField(...)` | `forms.NewMultilineTextField(...)` |
| `forms.SignatureField(...)` | `forms.NewSignatureField(...)` |

## 2. Rename `sign.LoadPKCS12` → `sign.ParsePKCS12`

The function was renamed to match the `Parse*` convention (it takes
`[]byte`, not a file path). The signature is unchanged — only the name.

```go
// Before
signer, err := sign.LoadPKCS12(data, "password")

// After
signer, err := sign.ParsePKCS12(data, "password")
```

## 3. Handle `Document.Page` error return

```go
// Before
page := doc.Page(0)

// After
page, err := doc.Page(0)
if err != nil {
    // handle out-of-range index
}
```

## 4. Remove references to unexported symbols

These were internal and are now unexported. If your code referenced
them directly, switch to the public API equivalents.

**reader:** `buildFontCache`, `parseStructureTree`, `glyphToRune`,
`serializeContentOps`, `winAnsiEncoding`, `macRomanEncoding`,
`standardEncoding`

**svg:** `parseColor`, `parseTransform`, `arcToCubics`, `parsePathData`,
`defaultStyle`, `resolveStyle`, `identity`

## 5. Visual review — baseline positioning changed

Text baselines now use CSS half-leading with actual font metrics.
For Helvetica 12pt at 1.2 leading, text moves **up ~4pt** within
line boxes. Background colors now cover the full line height.

**All generated PDFs will look different.** The change is correct
per CSS 2.1 §10.8.1, but documents that relied on the old positioning
should be visually reviewed.

## 6. Check for `vertical-align` with length values

`vertical-align: 5pt` was previously silently ignored. It now
produces a baseline shift. If your HTML has accidental length values
on `vertical-align`, they will now take effect.
