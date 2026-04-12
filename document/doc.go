// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package document is the top-level API for building PDF files.
//
// A [Document] coordinates page creation, layout-engine integration,
// font and image embedding, headers/footers, watermarks, bookmarks,
// named destinations, page labels, metadata, encryption, digital
// signatures, and standards compliance (PDF/A, PDF/UA).
//
// Typical usage:
//
//	doc := document.NewDocument(document.PageSizeA4)
//	doc.Add(paragraph)
//	doc.WriteTo(writer)
//
// For HTML-to-PDF conversion, use [Document.AddHTML] which delegates
// to the [github.com/carlos7ags/folio/html] converter and feeds the
// resulting layout elements into the document.
package document
