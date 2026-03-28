// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

// Package image decodes raster images and builds PDF image XObjects
// (ISO 32000 §8.8.5) for embedding in documents.
//
// Supported formats:
//
//   - JPEG — raw passthrough via DCTDecode (§7.4.8)
//   - PNG  — decompressed to FlateDecode (§7.4.4)
//   - TIFF — via golang.org/x/image/tiff
//   - WebP — via golang.org/x/image/webp
//   - GIF  — first frame only, via stdlib image/gif
//
// Each format has a [Load*] function (from file path) and a [New*]
// function (from []byte). [NewFromGoImage] accepts a Go [image.RGBA]
// directly. All constructors return an [Image] which can produce a
// PDF XObject via [Image.BuildXObject], including soft-mask (SMask)
// generation for images with transparency.
package image
