// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package reader parses existing PDF files and provides access to
// document structure, pages, and content.
//
// Use [Open] to read from a file path or [Parse] / [ParseWithOptions]
// to read from bytes. The returned [PdfReader] exposes page geometry
// (MediaBox, CropBox, BleedBox, TrimBox, ArtBox per §14.11.2),
// text extraction with multiple strategies, content-stream parsing,
// image references, and the document's structure tree (§14.7).
//
// Higher-level operations built on the reader:
//
//   - [Merge] / [MergeFiles] — combine multiple PDFs
//   - [RedactText] / [RedactPattern] / [RedactRegions] — permanent content removal
//   - [ExtractPageImport] — extract a page as a Form XObject for reuse
//   - Form flattening — bake interactive fields into static content
//
// The parser supports both tolerant and strict modes (see [Strictness])
// and includes security limits to prevent memory exhaustion on
// malformed input.
package reader
