// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package image

import (
	"encoding/binary"
	"fmt"
)

// JPEG marker constants.
const (
	markerSOI   = 0xFFD8 // Start of Image
	markerSOF0  = 0xFFC0 // Baseline DCT
	markerSOF1  = 0xFFC1 // Extended sequential DCT
	markerSOF2  = 0xFFC2 // Progressive DCT
	markerAPP14 = 0xFFEE // Application segment 14 (Adobe)
)

// maxJPEGSegments bounds the number of segments [parseJPEGHeader] is
// willing to walk before concluding the file is malformed. Real JPEGs
// rarely contain more than a few dozen segments; anything close to this
// limit is adversarial.
const maxJPEGSegments = 10000

// NewJPEG creates an Image from raw JPEG data. It parses the JPEG header
// to extract dimensions and color space, rejecting dimensions that exceed
// the package limits ([MaxDimension], [MaxPixels]).
//
// When the JPEG is 4-component CMYK with an Adobe APP14 marker, the image
// is flagged for inverted-CMYK decoding: Photoshop-exported CMYK JPEGs
// store values in a convention opposite to what PDF viewers expect, so the
// resulting XObject is emitted with a /Decode array that flips every
// channel. This matches the behavior of poppler, mupdf, and Chrome.
func NewJPEG(data []byte) (*Image, error) {
	w, h, ncomp, hasAdobe, err := parseJPEGHeader(data)
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
		adobeCMYK:  ncomp == 4 && hasAdobe,
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

// parseJPEGHeader reads the JPEG header to find dimensions, component
// count, and whether an Adobe APP14 marker is present. It scans for SOF0,
// SOF1, or SOF2 markers and bounds the number of segments walked via
// [maxJPEGSegments] to guard against crafted files that would otherwise
// loop slowly through pathological segment sequences.
//
// hasAdobe is true when the stream contains an APP14 segment (marker
// 0xFFEE) whose payload starts with the "Adobe" identifier. Photoshop
// always emits this marker when writing CMYK, so callers use it to detect
// the inverted-CMYK convention and emit a PDF /Decode array accordingly.
func parseJPEGHeader(data []byte) (width, height, numComponents int, hasAdobe bool, err error) {
	if len(data) < 2 || binary.BigEndian.Uint16(data[0:2]) != markerSOI {
		return 0, 0, 0, false, fmt.Errorf("not a JPEG file")
	}

	pos := 2
	for segments := 0; pos < len(data)-1; segments++ {
		if segments > maxJPEGSegments {
			return 0, 0, 0, false, fmt.Errorf("too many segments (>%d)", maxJPEGSegments)
		}

		// Find marker (0xFF followed by non-zero byte).
		if data[pos] != 0xFF {
			return 0, 0, 0, false, fmt.Errorf("expected marker at offset %d", pos)
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
				return 0, 0, 0, false, fmt.Errorf("truncated SOF segment")
			}
			height = int(binary.BigEndian.Uint16(data[pos+3 : pos+5]))
			width = int(binary.BigEndian.Uint16(data[pos+5 : pos+7]))
			numComponents = int(data[pos+7])
			return width, height, numComponents, hasAdobe, nil
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
			return 0, 0, 0, false, fmt.Errorf("invalid segment length %d at offset %d", segLen, pos)
		}

		// APP14 Adobe marker: 2-byte length, then "Adobe" + 7-byte body.
		// Only the identifier string is required to flag the file; the
		// DCTEncodeVersion / flags / ColorTransform fields aren't used by
		// the inversion heuristic.
		if marker == markerAPP14 && segLen >= 7 && pos+segLen <= len(data) {
			payload := data[pos+2 : pos+segLen]
			if len(payload) >= 5 && string(payload[0:5]) == "Adobe" {
				hasAdobe = true
			}
		}

		pos += segLen
	}

	return 0, 0, 0, false, fmt.Errorf("no SOF marker found")
}
