// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
	"os"
)

// ParseFont parses a font from raw bytes, auto-detecting the format.
// Supports TTF, OTF, and WOFF1 fonts.
//
// Errors returned by this function wrap one of the sentinel errors
// [ErrUnknownFormat], [ErrTruncated], or [ErrCorruptTable] so callers
// can match failure modes with errors.Is.
func ParseFont(data []byte) (Face, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("font data too short to determine format: %w", ErrTruncated)
	}
	sig := binary.BigEndian.Uint32(data[0:4])
	switch sig {
	case woffMagic:
		ttfData, err := decodeWOFF(data)
		if err != nil {
			return nil, fmt.Errorf("decode WOFF: %w", err)
		}
		return ParseTTF(ttfData)
	case 0x00010000, // TrueType
		0x4F54544F, // "OTTO" (OpenType/CFF)
		0x74727565, // "true"
		0x74797031, // "typ1"
		0x74746366: // "ttcf" (TrueType Collection)
		return ParseTTF(data)
	}
	return nil, fmt.Errorf("unknown font magic 0x%08X: %w", sig, ErrUnknownFormat)
}

// LoadFont reads and parses a font file from disk, auto-detecting the format.
// Supports TTF, OTF, and WOFF1 fonts.
//
// Errors returned by this function wrap one of the sentinel errors
// [ErrUnknownFormat], [ErrTruncated], or [ErrCorruptTable] so callers
// can match failure modes with errors.Is.
func LoadFont(path string) (Face, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read font file: %w", err)
	}
	return ParseFont(data)
}
