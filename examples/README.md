# Examples

Each subdirectory is a self-contained example that produces a PDF.

## Running

```bash
go run ./examples/hello
```

## Structure

```
examples/
├── hello/          # minimal one-page PDF
├── fonts/          # standard, custom, and Unicode fonts (CJK, Cyrillic)
├── links/          # hyperlinks, bookmarks, internal navigation
├── forms/          # interactive AcroForm fields (text, checkbox, radio, dropdown)
├── html-to-pdf/    # rich HTML+CSS report (flexbox, tables, page breaks)
├── merge/          # parse, merge, and extract text from PDFs
├── redact/         # permanently remove sensitive text (SSNs, emails, PII)
├── report/         # multi-page report with layout API (tables, lists, columns)
├── sign/           # PAdES digital signature with self-signed certificate
├── zugferd/        # PDF/A-3B invoice with Factur-X XML attachment
└── README.md
```

## Adding an example

1. Create a new directory under `examples/` (e.g., `examples/invoice/`).
2. Add a single `main.go` that produces one PDF.
3. Include any required data files in the same directory (XML, images, etc.).
4. Keep it self-contained — no shared code between examples.
5. Add a doc comment at the top of `main.go` describing what the example does and how to run it.
6. Add the directory to the structure list above.
