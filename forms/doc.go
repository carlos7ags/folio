// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package forms provides AcroForm support for creating and filling
// interactive PDF form fields (ISO 32000 §12.7).
//
// Supported field types:
//
//   - Text input — [TextField], [MultilineTextField], [PasswordField]
//   - Selection — [Checkbox], [RadioGroup], [Dropdown], [ListBox]
//   - Action — [PushButton]
//   - Signature — [SignatureField]
//
// Each factory function returns a [Field] that can be configured with
// flags (ReadOnly, Required, etc.) and appearance properties. Call
// [Field.ToDict] to produce the PDF dictionary and widget annotations
// for embedding in a document.
package forms
