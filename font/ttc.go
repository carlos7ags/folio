// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
)

// ttcMagic is the 4-byte signature that identifies a TrueType Collection.
//
// TTC layout reference: ISO/IEC 14496-22 §5 (Open Font Format) and the
// Apple TrueType Reference, "Font Collections" — both define the
// 12-byte TTC header (tag, version, numFonts) followed by uint32 offsets
// to each font's offset table, with table directory offsets stated as
// absolute positions within the TTC file.
const ttcMagic = 0x74746366 // "ttcf"

// extractTTCFont returns a standalone TTF byte stream for the font at index
// in a TrueType Collection. Tables are copied in the order they appear in
// the source font's table directory, with offsets rewritten to be relative
// to the new single-font file. The TTC's shared-table layout is preserved
// only by value: each extracted font is fully self-contained, so callers
// can subset, embed, or hash it like any other TTF.
//
// Index selection follows the convention used by browsers and font tools
// for url() references without a `#` fragment: face 0 is the natural choice
// for whole-collection references. Higher indices are accepted in case a
// future caller (HTTP fragment, font-collection-index CSS extension, etc.)
// needs to select a specific face.
//
// All offset and length arithmetic on parsed uint32 fields is performed in
// uint64 against the buffer length to avoid the int wraparound that would
// occur on 32-bit hosts when a hostile collection declares offsets larger
// than 2^31. Bad input returns a wrapped sentinel error; the function
// never panics on malformed bytes.
func extractTTCFont(data []byte, index int) ([]byte, error) {
	dataLen := uint64(len(data))
	if dataLen < 12 {
		return nil, fmt.Errorf("ttc: header too short: %w", ErrTruncated)
	}
	if binary.BigEndian.Uint32(data[0:4]) != ttcMagic {
		return nil, fmt.Errorf("ttc: missing ttcf magic: %w", ErrUnknownFormat)
	}
	numFonts := uint64(binary.BigEndian.Uint32(data[8:12]))
	if numFonts < 1 {
		return nil, fmt.Errorf("ttc: empty collection: %w", ErrCorruptTable)
	}
	if index < 0 || uint64(index) >= numFonts {
		return nil, fmt.Errorf("ttc: font index %d out of range [0,%d): %w", index, numFonts, ErrCorruptTable)
	}
	if dataLen < 12+numFonts*4 {
		return nil, fmt.Errorf("ttc: offset table truncated: %w", ErrTruncated)
	}

	fontOffset := uint64(binary.BigEndian.Uint32(data[12+index*4:]))
	if fontOffset+12 > dataLen {
		return nil, fmt.Errorf("ttc: font %d offset out of range: %w", index, ErrTruncated)
	}

	// Offset Table (12 bytes): sfntVersion[4], numTables[2], searchRange[2],
	// entrySelector[2], rangeShift[2].
	numTables := uint64(binary.BigEndian.Uint16(data[fontOffset+4:]))
	dirSize := numTables * 16
	if fontOffset+12+dirSize > dataLen {
		return nil, fmt.Errorf("ttc: font %d directory truncated: %w", index, ErrTruncated)
	}

	// Snapshot each (tag, checksum, srcOffset, length) and accumulate the
	// total payload size so we can size the output buffer up-front.
	type entry struct {
		tag      [4]byte
		checksum uint32
		srcOff   uint64
		length   uint64
	}
	entries := make([]entry, numTables)
	var payload uint64
	for i := uint64(0); i < numTables; i++ {
		base := fontOffset + 12 + i*16
		var e entry
		copy(e.tag[:], data[base:base+4])
		e.checksum = binary.BigEndian.Uint32(data[base+4:])
		e.srcOff = uint64(binary.BigEndian.Uint32(data[base+8:]))
		e.length = uint64(binary.BigEndian.Uint32(data[base+12:]))
		if e.srcOff+e.length > dataLen {
			return nil, fmt.Errorf("ttc: table %s extends beyond data: %w", string(e.tag[:]), ErrTruncated)
		}
		entries[i] = e
		// Tables in TTF are 4-byte aligned in the file.
		payload += (e.length + 3) &^ 3
	}

	headerSize := 12 + dirSize
	totalSize := headerSize + payload
	if totalSize > uint64(^uint(0)>>1) {
		return nil, fmt.Errorf("ttc: extracted size %d exceeds platform int range: %w", totalSize, ErrCorruptTable)
	}
	out := make([]byte, totalSize)

	// Copy the offset table header (sfntVersion + counts) verbatim from the
	// source font's offset table — these fields are font-shape metadata, not
	// offsets, so they translate directly.
	copy(out[0:12], data[fontOffset:fontOffset+12])

	// Write directory entries with rewritten offsets, then copy table data.
	dst := headerSize
	for i, e := range entries {
		dirBase := 12 + uint64(i)*16
		copy(out[dirBase:dirBase+4], e.tag[:])
		binary.BigEndian.PutUint32(out[dirBase+4:], e.checksum)
		binary.BigEndian.PutUint32(out[dirBase+8:], uint32(dst))
		binary.BigEndian.PutUint32(out[dirBase+12:], uint32(e.length))
		copy(out[dst:dst+e.length], data[e.srcOff:e.srcOff+e.length])
		dst += (e.length + 3) &^ 3
	}

	return out, nil
}
