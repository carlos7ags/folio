// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
)

// stubCmapBytes is a minimal-but-valid cmap table that golang.org/x/image/font/sfnt
// will accept without complaint: one Unicode-platform encoding record
// pointing at a format-6 subtable with zero entries. Total: 22 bytes.
//
// We write this stub when the real cmap exceeds sfnt's maxCmapSegments
// limit. sfnt parses the stub successfully; downstream Folio code calls
// [sfntFace.GlyphIndex] which reads from our own [cmapTable] parsed from
// the original raw bytes, so the stub's empty mapping is never consulted.
//
// Layout: ISO/IEC 14496-22 §5.2.1.3.6 (format 6, "Trimmed table mapping").
var stubCmapBytes = []byte{
	0x00, 0x00, // version
	0x00, 0x01, // numTables = 1
	// Encoding record:
	0x00, 0x00, // platformID = 0 (Unicode)
	0x00, 0x03, // encodingID = 3 (Unicode 2.0 BMP)
	0x00, 0x00, 0x00, 0x0C, // offset = 12 (just past this record)
	// Format 6 subtable:
	0x00, 0x06, // format = 6
	0x00, 0x0A, // length = 10 (header only, no glyphIdArray)
	0x00, 0x00, // language = 0
	0x00, 0x00, // firstCode = 0
	0x00, 0x00, // entryCount = 0
}

// appendStubCmap returns a new font byte stream where the cmap table's
// directory entry has been redirected to a minimal stub appended at the
// end of the file. The original cmap bytes are left in place but
// orphaned — sfnt only follows directory offsets and never sees them.
//
// This is the surgical workaround for sfnt's maxCmapSegments=20000
// limit: rather than re-encode the entire font with a smaller cmap (which
// would require shifting every subsequent table's offset and recomputing
// checksums), we leave all other tables untouched and add a stub the
// directory points at instead.
//
// Folio supplies the real cmap mapping via [sfntFace.cmap], populated by
// [parseCmapTable] from the ORIGINAL bytes before the stub substitution.
// The orphaned cmap data inflates the font by at most a few hundred
// kilobytes (CJK cmap tables are typically 200-500 KB), which is
// negligible against the multi-megabyte size of the fonts that hit this
// path.
func appendStubCmap(rawFont []byte) ([]byte, error) {
	if len(rawFont) < 12 {
		return nil, fmt.Errorf("stub cmap: font header too short: %w", ErrTruncated)
	}
	numTables := int(binary.BigEndian.Uint16(rawFont[4:6]))
	if len(rawFont) < 12+numTables*16 {
		return nil, fmt.Errorf("stub cmap: directory truncated: %w", ErrTruncated)
	}

	// Locate the cmap directory entry.
	cmapEntryOffset := -1
	for i := range numTables {
		entry := 12 + i*16
		if string(rawFont[entry:entry+4]) == "cmap" {
			cmapEntryOffset = entry
			break
		}
	}
	if cmapEntryOffset < 0 {
		return nil, fmt.Errorf("stub cmap: no cmap entry in directory: %w", ErrMissingTable)
	}

	// Append the stub aligned to 4 bytes (sfnt expects table offsets to
	// be 4-byte aligned; misaligned reads succeed in practice but the
	// spec requires it).
	srcLen := len(rawFont)
	pad := (4 - (srcLen % 4)) % 4
	newCmapOffset := srcLen + pad
	out := make([]byte, newCmapOffset+len(stubCmapBytes))
	copy(out, rawFont)
	copy(out[newCmapOffset:], stubCmapBytes)

	// Patch the directory entry to point at the stub. checksum is left
	// untouched — sfnt does not validate per-table checksums.
	binary.BigEndian.PutUint32(out[cmapEntryOffset+8:cmapEntryOffset+12], uint32(newCmapOffset))
	binary.BigEndian.PutUint32(out[cmapEntryOffset+12:cmapEntryOffset+16], uint32(len(stubCmapBytes)))

	return out, nil
}
