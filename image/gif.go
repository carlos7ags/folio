// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"fmt"
	"image/gif"
)

// NewGIF creates an Image from raw GIF data. Only the first frame of an
// animation is used; subsequent frames are discarded. Alpha is preserved
// as a soft mask if present. Dimensions are validated against
// [MaxDimension] and [MaxPixels] before the pixel buffer is allocated.
func NewGIF(data []byte) (*Image, error) {
	img, err := gif.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gif: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if err := checkDimensions(w, h); err != nil {
		return nil, fmt.Errorf("gif: %w", err)
	}

	// GIF returns *goimage.Paletted; buildRGBMaybeAlpha's generic path
	// handles it via color.NRGBAModel.Convert, extracting straight alpha
	// in the same pass as the RGB bytes.
	return buildRGBMaybeAlpha(img, w, h)
}

// LoadGIF reads a GIF file from disk and creates an Image. Files larger
// than [MaxFileSize] are rejected with [ErrFileTooLarge].
func LoadGIF(path string) (*Image, error) {
	data, err := readLimited(path)
	if err != nil {
		return nil, err
	}
	return NewGIF(data)
}
