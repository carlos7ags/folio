// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package export provides a C ABI for the Folio PDF library, enabling
// use from C, Python, Ruby, Java (JNI), .NET (P/Invoke), and any
// language with a C FFI.
//
// All Go objects are stored in an opaque handle table; C callers
// receive uint64 handles and never see raw Go pointers. Functions
// follow a consistent convention: return int32 (0 = success, negative
// = error code) with detailed messages retrievable via folio_last_error.
//
// The exported functions are organized by module:
//
//   - cabi_document.go  — Document creation, pages, metadata
//   - cabi_layout.go    — Layout elements (paragraphs, tables, etc.)
//   - cabi_font.go      — Font loading and embedding
//   - cabi_image.go     — Image loading
//   - cabi_reader.go    — PDF reading and text extraction
//   - cabi_sign.go      — Digital signatures
//   - cabi_forms.go     — AcroForm fields
//   - cabi_redact.go    — Text redaction
//   - cabi_import.go    — Page import
//
// Build with -buildmode=c-shared to produce a shared library.
package main
