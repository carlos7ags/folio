// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package core implements the fundamental PDF object types defined in
// ISO 32000 §7.3 — Boolean, Number, String, Name, Array, Dictionary,
// Stream, and Null — plus indirect references (§7.3.10) and standard
// security handler encryption (§7.6).
//
// Every object type satisfies the [PdfObject] interface, which provides
// WriteTo for serialization and Type for runtime type discrimination.
// Composite types (Array, Dictionary, Stream) preserve insertion order
// to produce deterministic output.
//
// Encryption covers three standard security handler revisions:
//
//   - RC4-128  (V=2, R=3): legacy, widely compatible
//   - AES-128  (V=4, R=4): recommended minimum
//   - AES-256  (V=5, R=6): strongest, PDF 2.0
package core
