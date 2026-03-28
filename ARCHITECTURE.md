# Architecture

This document captures the design principles, package contracts, and
ground rules for the Folio PDF library. Use it to evaluate whether a
proposed change (issue, PR, feature request) fits the project's
direction or would introduce a deviation that makes the library harder
to maintain.

---

## Design principles

1. **Spec-first, not viewer-first.** Implement what ISO 32000 says, not
   what a particular viewer happens to accept. When the spec is
   ambiguous, match the behavior of the major viewers (Adobe Acrobat,
   Chrome's PDFium, macOS Preview) and document the choice.

2. **Standard library cryptography only.** All encryption (AES, RC4)
   and signing (CMS/PKCS#7, RSA, ECDSA) use `crypto/*` and
   `encoding/asn1` from the Go standard library. No external
   cryptography dependencies — ever.

3. **Minimal external dependencies.** The only direct dependencies are
   `golang.org/x/image` (font parsing via sfnt, TIFF decoding) and
   `golang.org/x/net` (HTML parsing). A new external dependency
   requires a compelling justification and must come from the
   `golang.org/x` ecosystem or be similarly vetted. Convenience is
   not a compelling justification.

4. **Zero-allocation PDF output path.** The writer (`core` types →
   `io.Writer`) should not allocate beyond what the data requires. No
   intermediate `[]byte` buffers for the full document. Objects stream
   directly.

5. **Deterministic output.** Given the same inputs, produce
   byte-identical PDFs. This means ordered dictionaries (insertion
   order, not map iteration order), stable subset tags, and no
   timestamps unless the caller opts in. This enables testing by
   byte comparison.

6. **Immutable layout results.** The layout phase produces a
   `LayoutPlan` that is pure data — no closures that capture mutable
   state, no side effects. This enables caching, concurrent layout,
   and straightforward testing.

7. **Errors, not panics.** Public API functions return errors. Panics
   are reserved for true programming errors (nil where non-nil is
   required, violated internal invariants). Never panic on malformed
   PDF input.

---

## Package responsibilities

Each package owns one concern. If a change touches multiple packages,
check that it respects these boundaries.

| Package | Owns | Does NOT own |
|---|---|---|
| `core` | PDF object types (§7.3), serialization, encryption (§7.6) | Parsing, document structure, page layout |
| `content` | Content stream builder (operators → bytes) | Interpreting content streams, text extraction |
| `font` | Font loading, parsing, metrics, subsetting, PDF embedding | Text layout, line breaking, glyph rendering |
| `image` | Image decoding (JPEG/PNG/TIFF) and PDF XObject construction | Image manipulation, resizing, format conversion |
| `barcode` | Barcode generation (QR, Code 128, EAN-13) as module grids | Barcode scanning/reading |
| `layout` | Element model, box layout, pagination, rendering to content streams | HTML parsing, CSS parsing, document assembly |
| `forms` | AcroForm field creation and form filling | Field validation logic, JavaScript actions |
| `sign` | PAdES digital signatures (B-B through B-LTA), CMS, TSA, OCSP, DSS | Certificate management, key storage |
| `reader` | PDF parsing, object resolution, text extraction, page merging, content transforms (redaction, form flattening) | PDF modification in place (use incremental writes) |
| `html` | HTML+CSS → `layout.Element` conversion | Direct PDF generation (that's `document`'s job) |
| `svg` | SVG parsing and rendering to content stream operators | SVG creation/generation, raster export |
| `document` | Top-level document assembly: pages, fonts, images, outlines, metadata, PDF/A, tagged PDF | Low-level PDF object wiring (that's `core`) |
| `export` | C ABI for FFI consumers | Business logic — it delegates everything to other packages |
| `cmd/folio` | CLI tool for merge, info, text extraction, signing | Library API (it's a thin consumer) |
| `cmd/wasm` | WebAssembly entry point — exposes `folioRender` for browser use | Library API (thin consumer) |
| `cmd/gen-metrics` | Code generator — parses AFM files, emits Go width/kern tables | Runtime logic (build-time only tool) |
| `integration` | Cross-package integration tests (header/footer, import, running headers) | Production code — test-only package |

---

## Layering rules

Imports flow downward. A package must never import a package above it
or at the same level (no cycles).

```
             document
            /   |    \
         html layout  sign
        / |     |   \    \
      svg font image barcode
        \   \  |   /       \
         content  forms    reader
            \      /      /
              core
```

Exceptions:
- `document` imports `html` to offer HTML-to-PDF convenience methods
  (`AddHTML`) that delegate to the converter.
- `layout` imports `image` for image element rendering and background
  image support in divs.
- `reader` imports `document` for struct tree types and `font` for
  standard font metrics (used in text extraction width calculation).
- `forms` imports `reader` for form filling on existing PDFs.

These are intentional — they exist because the functionality requires
access to types defined in another layer. If these cross-layer imports
grow, consider extracting shared types into `core` or a new `model`
package.

### Known structural TODOs

These are deferred refactors that would improve the architecture but
have high churn cost relative to immediate benefit. Tackle them when a
natural trigger arises (e.g., a new package needs `Color`).

- **TODO(A4): Unify `layout.Color` and `svg.Color`.** Two separate
  Color types serve the same purpose. Extract a shared color type into
  `core` or a new `style` package to eliminate conversion friction.
  Trigger: when a third package (e.g., `markdown`) needs a Color type.

- **TODO(A5): Reduce `layout` package surface area.** At ~75 exported
  types, `layout` is the largest package. Primitives like `Color`,
  `Margins`, `Padding`, `UnitValue` could move to a shared package.
  Trigger: when `layout` grows further or when multiple packages need
  these primitives independently.

- **TODO(A6): Split `reader` into focused packages.** The `reader`
  package handles parsing, text extraction, merging, and redaction —
  four distinct concerns. Splitting into `reader`, `merge`, and
  `redact` would improve cohesion. Trigger: when any of these areas
  grows significantly or gains its own public API surface.

---

## Naming conventions

Constructor names follow a consistent pattern across packages:

| Pattern | Meaning | Example |
|---|---|---|
| `New*` | Construct from Go types or config | `NewDocument()`, `NewDiv()`, `NewEmbeddedFont(face)` |
| `Parse*` | Construct from raw bytes (`[]byte`) | `ParseTTF(data)`, `Parse(pdfBytes)` |
| `Load*` | Construct from a file path (`string`) | `LoadTTF(path)`, `LoadJPEG(path)` |

When a package offers both file and bytes constructors, prefer `Load*`
for paths and `Parse*` / `New*` for bytes. Value-type constructors
(borders, colors) may use descriptive names without a prefix
(e.g., `SolidBorder`, `DashedBorder`).

---

## Non-goals

These are things the library intentionally does not do. PRs that
introduce these should be redirected or declined.

- **PDF rendering / rasterization.** Folio creates and reads PDFs; it
  does not render them to pixels. Use a viewer or a tool like
  `mupdf`/`poppler` for that.
- **JavaScript / actions.** PDF supports embedded JavaScript. We don't
  and won't — it's a security surface with minimal value.
- **Multimedia annotations.** Sound, video, 3D, and rich media
  annotations are out of scope.
- **XFA forms.** XFA is deprecated in PDF 2.0. We support AcroForms
  only.
- **Full CSS compliance.** The HTML converter supports a practical
  subset of CSS for document generation. It is not a browser engine
  and does not aim to be one.
- **Image manipulation.** No resizing, cropping, color space
  conversion, or format transcoding. Accept what the caller provides.
- **Certificate / key management.** The signing package accepts keys
  and certificates; it does not generate, store, or manage them.

---

## Dependency policy

| Dependency | Justification | Replacement plan |
|---|---|---|
| `golang.org/x/image/font/sfnt` | TrueType/OpenType font parsing. Writing a full sfnt parser is substantial work with little upside. | The `font.Face` interface abstracts over sfnt. If we ever write our own parser, no calling code changes. |
| `golang.org/x/image/tiff` | TIFF decoding. Rarely used but required for completeness. | Could drop if TIFF support is removed. |
| `golang.org/x/net/html` | HTML tokenization and tree construction. Correct HTML parsing is hard and well-solved. | No replacement planned. |

The transitive dependency `golang.org/x/text` is pulled in by
`golang.org/x/image` and `golang.org/x/net`. It is not imported
directly by any Folio package.

All other functionality (PDF parsing, encryption, signing, content
streams, subsetting, barcode generation, SVG rendering) is implemented
from scratch using only the Go standard library.

---

## Error handling

Public API functions return `error`. Errors are wrapped with
`fmt.Errorf("package: context: %w", err)` so callers can use
`errors.Is` and `errors.Unwrap` to inspect causes. Each package
prefixes its errors with the package name (e.g., `"reader: ..."`,
`"sign: ..."`).

Sentinel errors are used sparingly — only when callers need to branch
on the error kind (e.g., `reader.ErrMemoryLimitExceeded`). Most
functions return wrapped standard errors.

Panics are used only for violated preconditions in constructors (nil
font, negative size, invalid enum) where the caller has a programming
error. They are never used for recoverable conditions or malformed
input.

---

## Concurrency

Individual Folio objects (documents, readers, layout elements) are
**not safe for concurrent use** from multiple goroutines. This matches
the standard Go convention: callers synchronize access if needed.

The layout phase is designed for concurrent-friendly use: `PlanLayout`
produces immutable `LayoutPlan` values with no shared mutable state,
so multiple elements can be laid out concurrently if the caller
manages goroutines.

The `reader` package is safe to use from multiple goroutines **after**
parsing completes (`Open`/`Parse` return), since the parsed state is
read-only. Concurrent calls to `ExtractText` on different pages are
safe.

---

## Versioning

The library follows [semantic versioning](https://semver.org/). While
on `v0.x`, breaking API changes may occur in minor releases. Each
release includes a changelog documenting additions, changes, and
breaking changes.

The goal is to reach `v1.0` with a stable public API. Until then,
breaking changes are batched into releases rather than sprinkled
across patches, and deprecated symbols are kept for at least one
minor release when feasible.

---

## Evaluating a proposal

When reviewing an issue or PR, ask:

1. **Does it respect package boundaries?** If it adds image resizing
   to `image` or CSS animation to `html`, it's out of scope.
2. **Does it add a dependency?** If yes, can the same thing be done
   with the standard library in reasonable effort?
3. **Does it produce deterministic output?** If the change introduces
   maps, random values, or timestamps without opt-in, it breaks
   reproducibility.
4. **Does it follow the layering rules?** Check that no new import
   cycles are introduced and that dependencies flow downward.
5. **Is it spec-grounded?** New PDF features should reference the
   relevant ISO 32000 section. "Adobe Reader does X" is not sufficient
   justification without a spec reference.
6. **Does it fit the non-goals?** If the feature is explicitly listed
   as a non-goal, the bar for inclusion is very high — it requires
   a change to this document first.
