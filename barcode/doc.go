// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package barcode generates 1D and 2D barcodes and renders them
// directly into PDF content streams as vector graphics.
//
// Supported formats:
//
//   - QR Code (ISO/IEC 18004) — via [QR] and [QRWithECC]
//   - Code 128 (ISO/IEC 15417) — via [Code128]
//   - EAN-13 (GS1 General Specifications) — via [EAN13]
//
// Each constructor returns a [Barcode] holding a 2D module grid.
// Call [Barcode.Draw] to render the barcode onto a [content.Stream]
// at any position and size. All output is vector (filled rectangles),
// so barcodes remain sharp at any zoom level.
package barcode
