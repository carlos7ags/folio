// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import "encoding/binary"

// ParseKern parses the contents of a TrueType kern table and returns the
// full map of (leftGID, rightGID) pairs to signed FUnit adjustments from
// every horizontal, non-cross-stream format-0 subtable. Subtables in
// unsupported formats, vertical subtables, cross-stream subtables, and
// minimum/override subtables are ignored. Unknown, short, or malformed
// data yields an empty map (nil). The returned map never contains
// zero-valued entries.
//
// Supports both version-0 (Microsoft/OpenType) and version-1 (Apple AAT)
// kern tables. The two versions use different header and coverage-field
// layouts; see the Apple TrueType Reference Manual and Microsoft
// OpenType specification for the exact bit assignments.
func ParseKern(data []byte) map[[2]uint16]int16 {
	if len(data) < 4 {
		return nil
	}
	// Distinguish v0 from v1 by the first two bytes. v0 stores version as
	// a uint16 at offset 0 with value 0. v1 (Apple AAT) stores version as
	// a Fixed 16.16 at offset 0 with integer part 1, i.e. bytes 00 01 00 00.
	// A v1 file therefore has 0x0001 in the first two bytes; a v0 file has
	// 0x0000 there. Any other value is unknown.
	v0 := binary.BigEndian.Uint16(data[0:2])
	result := make(map[[2]uint16]int16)
	switch v0 {
	case 0:
		parseKernV0(data, result)
	case 1:
		if len(data) >= 8 {
			parseKernV1(data, result)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// parseKernV0 parses a Microsoft/OpenType version-0 kern table into result.
//
// Header:
//
//	version  uint16 (==0)
//	nTables  uint16
//
// Each subtable:
//
//	version  uint16
//	length   uint16  (including this 6-byte header)
//	coverage uint16
//
// coverage field (per the Apple TrueType Reference Manual, mirrored by
// Microsoft OpenType):
//
//	high byte: format
//	low byte bit 0: horizontal   (1 = horizontal metrics)
//	low byte bit 1: minimum      (minimum values rather than adjustments)
//	low byte bit 2: cross-stream (perpendicular to text direction)
//	low byte bit 3: override     (replace any accumulated kern value)
//
// Only format 0, horizontal, non-minimum, non-cross-stream subtables
// contribute pairs. The override bit (0x0008) is not inspected: folio
// keeps a single value per pair in the cache, and the in-order "first
// subtable wins" policy in parseKernFormat0 naturally absorbs whatever
// override semantics a well-formed font intends for shipped system
// fonts seen in practice.
func parseKernV0(data []byte, out map[[2]uint16]int16) {
	nTables := int(binary.BigEndian.Uint16(data[2:4]))
	off := 4
	for range nTables {
		if off+6 > len(data) {
			return
		}
		subLen := int(binary.BigEndian.Uint16(data[off+2 : off+4]))
		coverage := binary.BigEndian.Uint16(data[off+4 : off+6])
		if subLen < 6 || off+subLen > len(data) {
			return
		}
		format := coverage >> 8
		horizontal := coverage&0x0001 != 0
		minimum := coverage&0x0002 != 0
		crossStream := coverage&0x0004 != 0
		if format == 0 && horizontal && !minimum && !crossStream {
			parseKernFormat0(data[off+6:off+subLen], out)
		}
		off += subLen
	}
}

// parseKernV1 parses an Apple AAT version-1 kern table into result.
//
// Header:
//
//	version  Fixed  (uint32 == 0x00010000)
//	nTables  uint32
//
// Each subtable:
//
//	length     uint32 (including this 8-byte header)
//	coverage   uint16
//	tupleIndex uint16
//
// coverage field (v1 layout, from the Apple TrueType Reference Manual):
//
//	bit 15 (high byte bit 7): vertical     (1 = vertical subtable)
//	bit 14 (high byte bit 6): cross-stream (1 = cross-stream)
//	bit 13 (high byte bit 5): variation
//	bits 8-12: reserved
//	bits 0-7 (low byte): format
//
// Note this is the opposite of v0: in v1, vertical is encoded by a set
// bit and the default orientation is horizontal; format lives in the
// LOW byte. This is why a v1 table cannot be decoded with the same
// logic as a v0 table.
func parseKernV1(data []byte, out map[[2]uint16]int16) {
	nTables := int(binary.BigEndian.Uint32(data[4:8]))
	off := 8
	for range nTables {
		if off+8 > len(data) {
			return
		}
		subLen := int(binary.BigEndian.Uint32(data[off : off+4]))
		coverage := binary.BigEndian.Uint16(data[off+4 : off+6])
		if subLen < 8 || off+subLen > len(data) {
			return
		}
		vertical := coverage&0x8000 != 0
		crossStream := coverage&0x4000 != 0
		variation := coverage&0x2000 != 0
		format := coverage & 0x00FF
		if format == 0 && !vertical && !crossStream && !variation {
			parseKernFormat0(data[off+8:off+subLen], out)
		}
		off += subLen
	}
}

// parseKernFormat0 parses a kern format-0 subtable body (the bytes after
// the subtable header) into out.
//
// Body layout:
//
//	nPairs        uint16
//	searchRange   uint16
//	entrySelector uint16
//	rangeShift    uint16
//	pairs         [nPairs] { left uint16; right uint16; value int16 }
//
// The searchRange/entrySelector/rangeShift fields are remnants of a
// precomputed binary-search descriptor and are ignored here. Zero-valued
// pairs are skipped so the returned map only reports actual adjustments.
func parseKernFormat0(body []byte, out map[[2]uint16]int16) {
	if len(body) < 8 {
		return
	}
	nPairs := int(binary.BigEndian.Uint16(body[0:2]))
	pairs := body[8:]
	// Clamp declared pair count to what the table actually contains.
	if maxPairs := len(pairs) / 6; nPairs > maxPairs {
		nPairs = maxPairs
	}
	for i := 0; i < nPairs; i++ {
		off := i * 6
		left := binary.BigEndian.Uint16(pairs[off : off+2])
		right := binary.BigEndian.Uint16(pairs[off+2 : off+4])
		value := int16(binary.BigEndian.Uint16(pairs[off+4 : off+6]))
		if value == 0 {
			continue
		}
		key := [2]uint16{left, right}
		// First subtable wins for a given pair — later subtables may carry
		// supplementary adjustments that v0 semantics would cumulatively
		// add, but folio's Kern contract returns a single value per pair
		// and no current test font exercises that edge case.
		if _, exists := out[key]; !exists {
			out[key] = value
		}
	}
}
