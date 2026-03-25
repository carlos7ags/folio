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
- `reader` imports `document` for struct tree types and `font` for
  standard font metrics (used in text extraction width calculation).
- `forms` imports `reader` for form filling on existing PDFs.

These are intentional — they exist because reading an existing PDF
requires understanding the same structures that writing creates. If
these cross-layer imports grow, consider extracting shared types into
`core` or a new `model` package.

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

All other functionality (PDF parsing, encryption, signing, content
streams, subsetting, barcode generation, SVG rendering) is implemented
from scratch using only the Go standard library.

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
