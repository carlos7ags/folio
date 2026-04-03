# Migrating from v0.5.x to v0.6.0

This guide covers only what you need to change in your code.
For the full list of new features and fixes, see [CHANGELOG.md](CHANGELOG.md).

---

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
