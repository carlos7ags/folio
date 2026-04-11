// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"encoding/binary"
	"fmt"
)

// JPEG marker constants.
const (
	markerSOI  = 0xFFD8 // Start of Image
	markerSOF0 = 0xFFC0 // Baseline DCT
	markerSOF1 = 0xFFC1 // Extended sequential DCT
	markerSOF2 = 0xFFC2 // Progressive DCT
)

// maxJPEGSegments bounds the number of segments [parseJPEGHeader] is
// willing to walk before concluding the file is malformed. Real JPEGs
// rarely contain more than a few dozen segments; anything close to this
// limit is adversarial.
const maxJPEGSegments = 10000

// NewJPEG creates an Image from raw JPEG data. It parses the JPEG header
// to extract dimensions and color space, rejecting dimensions that exceed
// the package limits ([MaxDimension], [MaxPixels]).
func NewJPEG(data []byte) (*Image, error) {
	w, h, ncomp, err := parseJPEGHeader(data)
	if err != nil {
		return nil, fmt.Errorf("jpeg: %w", err)
	}
	if err := checkDimensions(w, h); err != nil {
		return nil, fmt.Errorf("jpeg: %w", err)
	}

	var cs string
	switch ncomp {
	case 1:
		cs = "DeviceGray"
	case 3:
		cs = "DeviceRGB"
	case 4:
		cs = "DeviceCMYK"
	default:
		return nil, fmt.Errorf("jpeg: unsupported component count %d", ncomp)
	}

	return &Image{
		data:       data,
		width:      w,
		height:     h,
		colorSpace: cs,
		bpc:        8,
		filter:     "DCTDecode",
	}, nil
}

// LoadJPEG reads a JPEG file from disk and creates an Image. Files larger
// than [MaxFileSize] are rejected with [ErrFileTooLarge] before being
// buffered into memory.
func LoadJPEG(path string) (*Image, error) {
	data, err := readLimited(path)
	if err != nil {
		return nil, err
	}
	return NewJPEG(data)
}

// parseJPEGHeader reads the JPEG header to find dimensions and component
// count. It scans for SOF0, SOF1, or SOF2 markers and bounds the number
// of segments walked via [maxJPEGSegments] to guard against crafted files
// that would otherwise loop slowly through pathological segment sequences.
func parseJPEGHeader(data []byte) (width, height, numComponents int, err error) {
	if len(data) < 2 || binary.BigEndian.Uint16(data[0:2]) != markerSOI {
		return 0, 0, 0, fmt.Errorf("not a JPEG file")
	}

	pos := 2
	for segments := 0; pos < len(data)-1; segments++ {
		if segments > maxJPEGSegments {
			return 0, 0, 0, fmt.Errorf("too many segments (>%d)", maxJPEGSegments)
		}

		// Find marker (0xFF followed by non-zero byte).
		if data[pos] != 0xFF {
			return 0, 0, 0, fmt.Errorf("expected marker at offset %d", pos)
		}

		// Skip padding 0xFF bytes.
		for pos < len(data)-1 && data[pos+1] == 0xFF {
			pos++
		}
		if pos >= len(data)-1 {
			break
		}

		marker := uint16(0xFF00) | uint16(data[pos+1])
		pos += 2

		// SOF markers contain the image dimensions.
		if marker == markerSOF0 || marker == markerSOF1 || marker == markerSOF2 {
			// SOF layout: length(2) + precision(1) + height(2) + width(2) + ncomp(1)
			// The ncomp byte lives at data[pos+7], so we need pos+8 ≤ len(data).
			if pos+8 > len(data) {
				return 0, 0, 0, fmt.Errorf("truncated SOF segment")
			}
			height = int(binary.BigEndian.Uint16(data[pos+3 : pos+5]))
			width = int(binary.BigEndian.Uint16(data[pos+5 : pos+7]))
			numComponents = int(data[pos+7])
			return width, height, numComponents, nil
		}

		// Skip non-SOF segments.
		if marker == 0xFFD9 { // EOI
			break
		}
		if marker >= 0xFFD0 && marker <= 0xFFD7 { // RST markers (no length)
			continue
		}
		if pos+1 >= len(data) {
			break
		}
		segLen := int(binary.BigEndian.Uint16(data[pos : pos+2]))
		if segLen < 2 {
			return 0, 0, 0, fmt.Errorf("invalid segment length %d at offset %d", segLen, pos)
		}
		pos += segLen
	}

	return 0, 0, 0, fmt.Errorf("no SOF marker found")
}
