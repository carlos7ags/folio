// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// Decoding limits protect against decompression bombs and malicious input.
// These are conservative defaults intended to handle realistic document
// images while bounding worst-case memory use. All bounds are enforced by
// the Load* and New* constructors before pixel buffers are allocated.
const (
	// MaxFileSize is the maximum size in bytes accepted by the Load*
	// functions. Files larger than this are rejected without allocating
	// a full-file buffer.
	MaxFileSize int64 = 200 * 1024 * 1024 // 200 MB

	// MaxDimension is the maximum width or height accepted by any
	// decoder. Images declaring larger dimensions are rejected before
	// pixel buffers are allocated.
	MaxDimension = 16384

	// MaxPixels is the maximum total pixel count (width × height). Even a
	// 16384×16384 image would allocate roughly 800 MB of RGB data; this
	// tighter bound keeps worst-case memory usage reasonable and guards
	// against int overflow on 32-bit platforms (e.g. WASM).
	MaxPixels = 100 * 1000 * 1000 // 100M pixels
)

// Sentinel errors for input validation. Tests and callers can use
// [errors.Is] to distinguish these from decoder-produced errors.
var (
	ErrFileTooLarge       = errors.New("image: file exceeds maximum size")
	ErrDimensionTooLarge  = errors.New("image: dimensions exceed maximum")
	ErrPixelCountTooLarge = errors.New("image: pixel count exceeds maximum")
	ErrDimensionInvalid   = errors.New("image: invalid dimensions")
)

// readLimited reads a file at path into memory, rejecting files larger
// than [MaxFileSize] without buffering the whole thing. It returns
// [ErrFileTooLarge] wrapped with size information when the limit is hit.
func readLimited(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// Stat-based early rejection for regular files. For pipes/devices
	// Size() returns 0 and we fall through to the streaming limit below.
	if fi, err := f.Stat(); err == nil && fi.Mode().IsRegular() {
		if fi.Size() > MaxFileSize {
			return nil, fmt.Errorf("%w: %d bytes > %d", ErrFileTooLarge, fi.Size(), MaxFileSize)
		}
	}

	// Read up to MaxFileSize+1 so we can detect overruns from streams
	// whose Stat reports 0.
	data, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > MaxFileSize {
		return nil, fmt.Errorf("%w: >%d bytes", ErrFileTooLarge, MaxFileSize)
	}
	return data, nil
}

// checkDimensions validates width and height against package limits. It
// returns [ErrDimensionInvalid] for non-positive values,
// [ErrDimensionTooLarge] for individual axes exceeding [MaxDimension],
// and [ErrPixelCountTooLarge] when the product exceeds [MaxPixels].
//
// The pixel count is computed in int64 to avoid int overflow on 32-bit
// platforms.
func checkDimensions(w, h int) error {
	if w <= 0 || h <= 0 {
		return fmt.Errorf("%w: %dx%d", ErrDimensionInvalid, w, h)
	}
	if w > MaxDimension || h > MaxDimension {
		return fmt.Errorf("%w: %dx%d (max %d)", ErrDimensionTooLarge, w, h, MaxDimension)
	}
	if int64(w)*int64(h) > MaxPixels {
		return fmt.Errorf("%w: %d pixels (max %d)", ErrPixelCountTooLarge, int64(w)*int64(h), MaxPixels)
	}
	return nil
}
