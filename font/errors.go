// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import "errors"

// Sentinel errors returned by font parsing and subsetting. Callers can
// match against these with errors.Is to distinguish failure modes
// without string matching.
var (
	// ErrUnknownFormat indicates that the input data does not match any
	// font format recognized by this package (TTF, OTF, WOFF1). The data
	// may be a different format entirely or simply not a font file.
	ErrUnknownFormat = errors.New("font: unknown font format")

	// ErrTruncated indicates that the input ends before a structure it
	// was in the middle of parsing. The data is shorter than its own
	// declared offsets require.
	ErrTruncated = errors.New("font: data truncated")

	// ErrMissingTable indicates that a table required for the requested
	// operation (typically subsetting) is absent from the font.
	ErrMissingTable = errors.New("font: required table missing")

	// ErrCorruptTable indicates that a table's contents could not be
	// parsed. The table is present but its internal layout is invalid.
	ErrCorruptTable = errors.New("font: corrupt table data")
)
