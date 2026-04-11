// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"fmt"

	"golang.org/x/image/tiff"
)

// NewTIFF creates an Image from raw TIFF data. It decodes the TIFF and
// re-encodes pixels as FlateDecode. Grayscale images are preserved as
// DeviceGray. Alpha channels are discarded because TIFF alpha is
// uncommon in PDF workflows. Dimensions are validated against
// [MaxDimension] and [MaxPixels] before the pixel buffer is allocated.
func NewTIFF(data []byte) (*Image, error) {
	img, err := tiff.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("tiff: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if err := checkDimensions(w, h); err != nil {
		return nil, fmt.Errorf("tiff: %w", err)
	}

	if isGrayscale(img) {
		return buildGray(img, w, h)
	}
	return buildRGBOnly(img, w, h)
}

// LoadTIFF reads a TIFF file from disk and creates an Image. Files
// larger than [MaxFileSize] are rejected with [ErrFileTooLarge].
func LoadTIFF(path string) (*Image, error) {
	data, err := readLimited(path)
	if err != nil {
		return nil, err
	}
	return NewTIFF(data)
}
