// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"bytes"
	"fmt"
	"os"

	"golang.org/x/image/tiff"
)

// NewTIFF creates an Image from raw TIFF data.
// It decodes the TIFF to extract pixel data, then uses FlateDecode for PDF
// embedding. Grayscale images are preserved as DeviceGray. Alpha channels
// are discarded because TIFF alpha is uncommon in PDF workflows.
func NewTIFF(data []byte) (*Image, error) {
	img, err := tiff.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("tiff: %w", err)
	}

	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	return buildRGB(img, w, h)
}

// LoadTIFF reads a TIFF file and creates an Image.
func LoadTIFF(path string) (*Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return NewTIFF(data)
}
