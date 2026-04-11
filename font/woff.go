// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"sort"
)

// woffMagic is the WOFF1 file signature: "wOFF" (0x774F4646).
const woffMagic = 0x774F4646

// woffHeaderSize is the size of the WOFF file header in bytes.
const woffHeaderSize = 44

// woffTableDirEntrySize is the size of each WOFF table directory entry.
const woffTableDirEntrySize = 20

// woffHeader represents the parsed WOFF1 file header.
// Only the fields actually consumed by the decoder are kept;
// length, reserved, and totalSfntSize are defined by the WOFF1 spec
// but are not needed to assemble the output TTF.
type woffHeader struct {
	signature uint32
	flavor    uint32
	numTables uint16
}

// woffTableEntry represents a single table directory entry in a WOFF file.
type woffTableEntry struct {
	tag          uint32
	offset       uint32
	compLength   uint32
	origLength   uint32
	origChecksum uint32
}

// decodeWOFF decodes a WOFF1 font file into raw TTF/OTF bytes.
func decodeWOFF(data []byte) ([]byte, error) {
	if len(data) < woffHeaderSize {
		return nil, fmt.Errorf("woff: data too short for header: %w", ErrTruncated)
	}

	// Parse header.
	var hdr woffHeader
	hdr.signature = binary.BigEndian.Uint32(data[0:4])
	if hdr.signature != woffMagic {
		return nil, fmt.Errorf("woff: invalid signature 0x%08X: %w", hdr.signature, ErrUnknownFormat)
	}
	hdr.flavor = binary.BigEndian.Uint32(data[4:8])
	hdr.numTables = binary.BigEndian.Uint16(data[12:14])

	if hdr.numTables == 0 {
		return nil, fmt.Errorf("woff: no tables: %w", ErrCorruptTable)
	}

	// Parse table directory.
	dirEnd := woffHeaderSize + int(hdr.numTables)*woffTableDirEntrySize
	if len(data) < dirEnd {
		return nil, fmt.Errorf("woff: data too short for table directory: %w", ErrTruncated)
	}

	entries := make([]woffTableEntry, hdr.numTables)
	for i := range entries {
		off := woffHeaderSize + i*woffTableDirEntrySize
		entries[i] = woffTableEntry{
			tag:          binary.BigEndian.Uint32(data[off : off+4]),
			offset:       binary.BigEndian.Uint32(data[off+4 : off+8]),
			compLength:   binary.BigEndian.Uint32(data[off+8 : off+12]),
			origLength:   binary.BigEndian.Uint32(data[off+12 : off+16]),
			origChecksum: binary.BigEndian.Uint32(data[off+16 : off+20]),
		}
	}

	// Decompress each table.
	tables := make([][]byte, len(entries))
	for i, e := range entries {
		if int(e.offset)+int(e.compLength) > len(data) {
			return nil, fmt.Errorf("woff: table %d extends beyond file: %w", i, ErrTruncated)
		}
		tableData := data[e.offset : e.offset+e.compLength]

		if e.compLength < e.origLength {
			// zlib-compressed table.
			r, err := zlib.NewReader(bytes.NewReader(tableData))
			if err != nil {
				return nil, fmt.Errorf("woff: zlib init for table %d: %v: %w", i, err, ErrCorruptTable)
			}
			decompressed, err := io.ReadAll(r)
			_ = r.Close()
			if err != nil {
				return nil, fmt.Errorf("woff: zlib decompress table %d: %v: %w", i, err, ErrCorruptTable)
			}
			if uint32(len(decompressed)) != e.origLength {
				return nil, fmt.Errorf("woff: table %d decompressed size mismatch: got %d, want %d: %w", i, len(decompressed), e.origLength, ErrCorruptTable)
			}
			tables[i] = decompressed
		} else {
			// Uncompressed table (compLength == origLength).
			tables[i] = tableData
		}
	}

	// Sort entries by tag for the output TTF table directory.
	type indexedEntry struct {
		idx   int
		entry woffTableEntry
	}
	sorted := make([]indexedEntry, len(entries))
	for i, e := range entries {
		sorted[i] = indexedEntry{idx: i, entry: e}
	}
	sort.Slice(sorted, func(a, b int) bool {
		return sorted[a].entry.tag < sorted[b].entry.tag
	})

	// Build the output TTF file.
	numTables := int(hdr.numTables)

	// Offset table: 12 bytes.
	// Table records: numTables * 16 bytes.
	headerSize := 12 + numTables*16

	// Calculate total size: header + all table data (each padded to 4 bytes).
	totalSize := headerSize
	for i := range sorted {
		tLen := int(sorted[i].entry.origLength)
		totalSize += (tLen + 3) &^ 3 // pad to 4-byte boundary
	}

	out := make([]byte, totalSize)

	// Write offset table header.
	binary.BigEndian.PutUint32(out[0:4], hdr.flavor) // sfVersion
	binary.BigEndian.PutUint16(out[4:6], uint16(numTables))

	// Compute searchRange, entrySelector, rangeShift.
	highPow2 := 1 << bits.Len(uint(numTables)>>1) // highest power of 2 <= numTables
	if highPow2 == 0 {
		highPow2 = 1
	}
	searchRange := highPow2 * 16
	entrySelector := bits.Len(uint(highPow2)) - 1
	rangeShift := numTables*16 - searchRange

	binary.BigEndian.PutUint16(out[6:8], uint16(searchRange))
	binary.BigEndian.PutUint16(out[8:10], uint16(entrySelector))
	binary.BigEndian.PutUint16(out[10:12], uint16(rangeShift))

	// Write table records and data.
	dataOffset := headerSize
	for i, se := range sorted {
		recOff := 12 + i*16
		tData := tables[se.idx]

		binary.BigEndian.PutUint32(out[recOff:recOff+4], se.entry.tag)
		binary.BigEndian.PutUint32(out[recOff+4:recOff+8], se.entry.origChecksum)
		binary.BigEndian.PutUint32(out[recOff+8:recOff+12], uint32(dataOffset))
		binary.BigEndian.PutUint32(out[recOff+12:recOff+16], se.entry.origLength)

		copy(out[dataOffset:], tData)
		dataOffset += (len(tData) + 3) &^ 3 // advance past padded data
	}

	return out, nil
}
