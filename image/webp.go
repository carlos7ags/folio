// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"fmt"

	"golang.org/x/image/webp"
)

// NewWebP creates an Image from raw WebP data. Alpha is preserved as a
// soft mask if present. Dimensions are validated against [MaxDimension]
// and [MaxPixels] before the pixel buffer is allocated.
func NewWebP(data []byte) (*Image, error) {
	img, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("webp: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if err := checkDimensions(w, h); err != nil {
		return nil, fmt.Errorf("webp: %w", err)
	}

	// buildRGBMaybeAlpha walks the decoded image once: RGB bytes go
	// into the data buffer and alpha is collected alongside, then
	// dropped if every pixel was opaque. The generic path converts
	// pixels via color.NRGBAModel, so no draw.Draw copy is needed.
	return buildRGBMaybeAlpha(img, w, h)
}

// LoadWebP reads a WebP file from disk and creates an Image. Files
// larger than [MaxFileSize] are rejected with [ErrFileTooLarge].
func LoadWebP(path string) (*Image, error) {
	data, err := readLimited(path)
	if err != nil {
		return nil, err
	}
	return NewWebP(data)
}
