// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
)

// SubsetCFF returns a CID-keyed CFF v1 blob containing only the
// charstrings referenced in usedGlyphs and the subroutines they
// transitively reach. Unused charstrings are replaced with a single
// `endchar` byte (0x0E); unreached global and per-FD local
// subroutines are replaced with a single `return` byte (0x0B). GID 0
// (.notdef) is always retained.
//
// The output preserves source-byte layout for sections that are
// already minimal: Header, Name INDEX, String INDEX, Charset, and
// FDSelect are copied verbatim. The Top DICT, FDArray font dicts,
// and per-FD Private DICTs are reassembled so their offset operands
// can be rewritten — all such operands use the fixed 5-byte longint
// encoding (29 + int32 big-endian) so section sizes are independent
// of the absolute offset values and the assembly can be done in a
// single forward pass.
//
// Errors wrap one of the package sentinels (ErrTruncated,
// ErrCorruptTable, ErrUnknownFormat). On any error the caller should
// fall back to embedding the full font; the subset never partially
// succeeds.
func SubsetCFF(cffBytes []byte, usedGlyphs map[uint16]rune) ([]byte, error) {
	src, err := parseCFF(cffBytes)
	if err != nil {
		return nil, fmt.Errorf("font: subset cff: %w", err)
	}

	keepGlyph := make([]bool, src.numGlyphs)
	keepGlyph[0] = true
	for gid := range usedGlyphs {
		if int(gid) < src.numGlyphs {
			keepGlyph[gid] = true
		}
	}

	// Reachability for global and per-FD local subroutines.
	lsubrs := make([]*cffIndex, len(src.fds))
	for i, fd := range src.fds {
		lsubrs[i] = fd.localSubrs
	}
	walker := newCharstringWalker(src.gsubrIndex, lsubrs)
	for gid := range src.numGlyphs {
		if !keepGlyph[gid] {
			continue
		}
		fd, err := src.fdForGlyph(gid)
		if err != nil {
			return nil, fmt.Errorf("font: subset cff: gid %d: %w", gid, err)
		}
		walker.Trace(src.charStringsIndex.Object(gid), fd)
	}

	// Build new CharStrings INDEX: kept GIDs verbatim, unkept GIDs
	// replaced with a single endchar byte. The GID count is
	// preserved so CID == GID semantics survive.
	newCharStrings := make([][]byte, src.numGlyphs)
	for i := range src.numGlyphs {
		if keepGlyph[i] {
			newCharStrings[i] = src.charStringsIndex.Object(i)
		} else {
			newCharStrings[i] = []byte{0x0E}
		}
	}
	newCharStringsBytes := writeCFFIndex(newCharStrings)

	// Build new Global Subr INDEX: unreached subrs become `return`.
	var newGsubrObjects [][]byte
	if src.gsubrIndex != nil && src.gsubrIndex.count > 0 {
		newGsubrObjects = make([][]byte, src.gsubrIndex.count)
		for i := range src.gsubrIndex.count {
			if walker.GsubrReached(i) {
				newGsubrObjects[i] = src.gsubrIndex.Object(i)
			} else {
				newGsubrObjects[i] = []byte{0x0B}
			}
		}
	}
	newGsubrBytes := writeCFFIndex(newGsubrObjects)

	// Per-FD: rebuild font dict, Private DICT (Subrs operand becomes
	// a placeholder), and Local Subr INDEX (with unreached payload
	// replaced by `return`).
	type fdBuild struct {
		fontDict       []byte
		privateDict    []byte
		localSubrsData []byte
		fontDictPatch  fontDictPatch // size + offset patch positions
		subrsPatch     int           // byte position of Subrs operand placeholder; -1 if none
		hasSubrs       bool          // mirror of subrsPatch != -1, for clarity
	}
	builds := make([]fdBuild, len(src.fds))
	for i, fd := range src.fds {
		// Always serialize a Local Subr INDEX section (even when
		// empty) for every FD whose Private DICT carries a Subrs
		// operator. The Subrs operator is a relative offset that
		// must land on a valid INDEX header — without emitting at
		// least 2 sentinel bytes, the offset would point into the
		// next FD's Private DICT and the re-parser would mistake
		// those bytes for an INDEX header.
		if fd.localSubrs != nil {
			subrs := make([][]byte, fd.localSubrs.count)
			for j := range fd.localSubrs.count {
				if walker.LsubrReached(i, j) {
					subrs[j] = fd.localSubrs.Object(j)
				} else {
					subrs[j] = []byte{0x0B}
				}
			}
			builds[i].localSubrsData = writeCFFIndex(subrs)
		}

		var subrsPatch int
		builds[i].privateDict, subrsPatch = rewritePrivateDict(fd.privateBytes, fd.privateDict)
		builds[i].subrsPatch = subrsPatch
		builds[i].hasSubrs = subrsPatch >= 0

		builds[i].fontDict, builds[i].fontDictPatch = rewriteFontDict(fd.fontDictBytes, fd.fontDict)
	}

	// Build FDArray INDEX.
	fdArrayObjects := make([][]byte, len(builds))
	for i, b := range builds {
		fdArrayObjects[i] = b.fontDict
	}
	newFDArrayBytes := writeCFFIndex(fdArrayObjects)

	// Rewrite Top DICT (placeholder operands for charset, CharStrings,
	// FDArray, FDSelect).
	newTopDict, topPatch := rewriteTopDict(src.topDictIndex.Object(0), src.topDict)
	newTopDictIndexBytes := writeCFFIndex([][]byte{newTopDict})

	// Section byte sources (some verbatim from src.raw, some new).
	headerBytes := src.header
	nameBytes := src.raw[src.nameIndex.rawStart:src.nameIndex.rawEnd]
	stringBytes := src.raw[src.stringIndex.rawStart:src.stringIndex.rawEnd]
	charsetBytes := src.raw[src.charsetOffset : src.charsetOffset+src.charsetSize]
	fdSelectBytes := src.raw[src.fdSelectOffset : src.fdSelectOffset+src.fdSelectSize]

	// Layout pass: compute absolute offsets for each section in the
	// fixed emission order. The Top DICT INDEX size is independent of
	// its placeholder values, which is why fixed 5-byte longints are
	// load-bearing for this single-pass layout to work.
	pos := 0
	pos += len(headerBytes)
	pos += len(nameBytes)
	pos += len(newTopDictIndexBytes)
	pos += len(stringBytes)
	pos += len(newGsubrBytes)
	offCharset := pos
	pos += len(charsetBytes)
	offFDSelect := pos
	pos += len(fdSelectBytes)
	offCharStrings := pos
	pos += len(newCharStringsBytes)
	offFDArray := pos
	pos += len(newFDArrayBytes)
	fdPrivateOff := make([]int, len(builds))
	for i := range builds {
		fdPrivateOff[i] = pos
		pos += len(builds[i].privateDict)
		pos += len(builds[i].localSubrsData)
	}
	finalSize := pos

	// Patch Top DICT operand placeholders.
	patchInt32(newTopDict, topPatch.charset, int32(offCharset))
	patchInt32(newTopDict, topPatch.charStrings, int32(offCharStrings))
	patchInt32(newTopDict, topPatch.fdArray, int32(offFDArray))
	patchInt32(newTopDict, topPatch.fdSelect, int32(offFDSelect))
	// The re-serialize below is REQUIRED, not an optimization choice:
	// writeCFFIndex above copied newTopDict's bytes into its output
	// (see appendOff + the final payload append), so the previous
	// newTopDictIndexBytes still carries the placeholder zeros even
	// though newTopDict itself has been patched. Refresh the wrapper
	// from the patched object bytes. Object length is unchanged
	// because every placeholder used the same fixed 5-byte longint
	// encoding, so the layout pass that already consumed
	// len(newTopDictIndexBytes) above remains accurate.
	newTopDictIndexBytes = writeCFFIndex([][]byte{newTopDict})

	// Patch each FDArray font dict's Private (size, offset) operands.
	for i, b := range builds {
		patchInt32(b.fontDict, b.fontDictPatch.size, int32(len(b.privateDict)))
		patchInt32(b.fontDict, b.fontDictPatch.offset, int32(fdPrivateOff[i]))
	}
	// Refresh FDArray INDEX bytes for the same reason as Top DICT
	// above: writeCFFIndex copied each font dict's bytes into the
	// INDEX payload before we patched the per-FD Private operands,
	// so the previous newFDArrayBytes carries stale zero
	// placeholders. Object byte lengths are unchanged by patchInt32,
	// so the cached len(newFDArrayBytes) from the layout pass is
	// still accurate.
	newFDArrayBytes = writeCFFIndex(fdArrayObjects)

	// Patch each Private DICT's Subrs operand. The offset is relative
	// to the Private DICT's start; since the Local Subr INDEX is
	// emitted immediately after, the value is exactly len(privateDict).
	for _, b := range builds {
		if !b.hasSubrs {
			continue
		}
		patchInt32(b.privateDict, b.subrsPatch, int32(len(b.privateDict)))
	}

	// Emit the assembled blob.
	out := make([]byte, 0, finalSize)
	out = append(out, headerBytes...)
	out = append(out, nameBytes...)
	out = append(out, newTopDictIndexBytes...)
	out = append(out, stringBytes...)
	out = append(out, newGsubrBytes...)
	out = append(out, charsetBytes...)
	out = append(out, fdSelectBytes...)
	out = append(out, newCharStringsBytes...)
	out = append(out, newFDArrayBytes...)
	for _, b := range builds {
		out = append(out, b.privateDict...)
		out = append(out, b.localSubrsData...)
	}
	if len(out) != finalSize {
		return nil, fmt.Errorf("font: subset cff: assembled %d bytes, expected %d: %w", len(out), finalSize, ErrCorruptTable)
	}
	return out, nil
}

// fdForGlyph reads the FDSelect table to find which FD owns gid. Only
// formats 0 and 3 are supported, matching computeFDSelectSize. Returns
// an error wrapping ErrCorruptTable if gid is out of range or the
// format is unknown.
func (cff *cffFont) fdForGlyph(gid int) (int, error) {
	if gid < 0 || gid >= cff.numGlyphs {
		return 0, fmt.Errorf("font: cff: gid %d out of range [0,%d): %w", gid, cff.numGlyphs, ErrCorruptTable)
	}
	off := cff.fdSelectOffset
	switch cff.raw[off] {
	case 0:
		return int(cff.raw[off+1+gid]), nil
	case 3:
		// Linear scan over ranges. A binary search would be faster
		// for large fonts; defer until profiling says it matters.
		nRanges := int(binary.BigEndian.Uint16(cff.raw[off+1 : off+3]))
		base := off + 3
		for i := range nRanges {
			first := int(binary.BigEndian.Uint16(cff.raw[base+i*3 : base+i*3+2]))
			fd := int(cff.raw[base+i*3+2])
			// The "first" of range i+1 (or the sentinel for the
			// final range) bounds the current range from above.
			nextFirst := int(binary.BigEndian.Uint16(cff.raw[base+nRanges*3 : base+nRanges*3+2]))
			if i+1 < nRanges {
				nextFirst = int(binary.BigEndian.Uint16(cff.raw[base+(i+1)*3 : base+(i+1)*3+2]))
			}
			if gid >= first && gid < nextFirst {
				return fd, nil
			}
		}
		return 0, fmt.Errorf("font: cff: gid %d not covered by fdselect format 3: %w", gid, ErrCorruptTable)
	}
	return 0, fmt.Errorf("font: cff: fdselect format %d unsupported: %w", cff.raw[off], ErrCorruptTable)
}

// writeCFFIndex serializes a list of object payloads as a CFF INDEX
// (TN #5176 §5). offSize is the smallest value that can encode the
// total payload size.
func writeCFFIndex(objects [][]byte) []byte {
	count := len(objects)
	if count == 0 {
		return []byte{0x00, 0x00}
	}
	totalPayload := 0
	for _, o := range objects {
		totalPayload += len(o)
	}
	// offsets are 1-indexed; the last offset equals totalPayload + 1.
	last := totalPayload + 1
	var offSize int
	switch {
	case last <= 0xFF:
		offSize = 1
	case last <= 0xFFFF:
		offSize = 2
	case last <= 0xFFFFFF:
		offSize = 3
	default:
		offSize = 4
	}

	headerLen := 3 + (count+1)*offSize
	out := make([]byte, 0, headerLen+totalPayload)
	out = append(out, byte(count>>8), byte(count&0xFF), byte(offSize))
	cumOff := 1
	out = appendOff(out, cumOff, offSize)
	for _, o := range objects {
		cumOff += len(o)
		out = appendOff(out, cumOff, offSize)
	}
	for _, o := range objects {
		out = append(out, o...)
	}
	return out
}

// appendOff appends a CFF INDEX offset of width n bytes (1..4) in
// big-endian order.
func appendOff(b []byte, v, n int) []byte {
	switch n {
	case 1:
		return append(b, byte(v))
	case 2:
		return append(b, byte(v>>8), byte(v))
	case 3:
		return append(b, byte(v>>16), byte(v>>8), byte(v))
	case 4:
		return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
	}
	return b
}

// topDictPatch records the byte offsets within a rebuilt Top DICT
// where the 4-byte int32 operand of each rewritable operator lives.
// Each value is the position of the most-significant byte of the
// longint operand (i.e. one past the 0x1D prefix byte). -1 means the
// operator was absent in the source and no patching is needed.
type topDictPatch struct {
	charset     int
	charStrings int
	fdArray     int
	fdSelect    int
}

// fontDictPatch records patch positions for the Private operator's
// two operands in a rebuilt FDArray font dict.
type fontDictPatch struct {
	size   int
	offset int
}

// rewriteTopDict emits a new Top DICT byte stream where the offset
// operands of `charset`, `CharStrings`, `FDArray`, and `FDSelect` are
// 5-byte longint placeholders. All other operators (ROS, CIDCount,
// version, etc.) are copied verbatim. Returns the new bytes and a
// struct giving the byte position of each placeholder's int32 payload.
func rewriteTopDict(src []byte, entries cffDict) ([]byte, topDictPatch) {
	patch := topDictPatch{charset: -1, charStrings: -1, fdArray: -1, fdSelect: -1}
	var out []byte
	for _, e := range entries {
		switch e.operator {
		case cffOpCharset:
			patch.charset = len(out) + 1
			out = appendLongInt(out, 0)
			out = append(out, byte(cffOpCharset))
		case cffOpCharStrings:
			patch.charStrings = len(out) + 1
			out = appendLongInt(out, 0)
			out = append(out, byte(cffOpCharStrings))
		case cffOp2FDArray:
			patch.fdArray = len(out) + 1
			out = appendLongInt(out, 0)
			out = append(out, cffOpEscape, 36)
		case cffOp2FDSelect:
			patch.fdSelect = len(out) + 1
			out = appendLongInt(out, 0)
			out = append(out, cffOpEscape, 37)
		default:
			out = appendDictEntryVerbatim(out, src, e)
		}
	}
	return out, patch
}

// rewriteFontDict emits a new FDArray font dict where the Private
// operator's two operands are 5-byte longint placeholders. Other
// operators (rare in font dicts but spec-legal — FontName, etc.) are
// copied verbatim.
func rewriteFontDict(src []byte, entries cffDict) ([]byte, fontDictPatch) {
	patch := fontDictPatch{size: -1, offset: -1}
	var out []byte
	for _, e := range entries {
		if e.operator == cffOpPrivate && len(e.intOperands) == 2 {
			patch.size = len(out) + 1
			out = appendLongInt(out, 0)
			patch.offset = len(out) + 1
			out = appendLongInt(out, 0)
			out = append(out, byte(cffOpPrivate))
		} else {
			out = appendDictEntryVerbatim(out, src, e)
		}
	}
	return out, patch
}

// rewritePrivateDict emits a new Private DICT where the Subrs operand
// (if present) is a 5-byte longint placeholder. Other operators
// (BlueValues, defaultWidthX, etc.) are copied verbatim. Returns the
// new bytes and the byte position of the Subrs placeholder, or -1
// when the source had no Subrs operator.
func rewritePrivateDict(src []byte, entries cffDict) ([]byte, int) {
	patch := -1
	var out []byte
	for _, e := range entries {
		if e.operator == cffOpSubrs {
			patch = len(out) + 1
			out = appendLongInt(out, 0)
			out = append(out, byte(cffOpSubrs))
		} else {
			out = appendDictEntryVerbatim(out, src, e)
		}
	}
	return out, patch
}

// appendDictEntryVerbatim copies the operand and operator bytes of e
// from src onto out. Used by the DICT rewriters for every operator
// they do not specifically replace.
func appendDictEntryVerbatim(out, src []byte, e cffDictEntry) []byte {
	out = append(out, src[e.operandStart:e.operandEnd]...)
	if e.operator > 0xFF {
		out = append(out, byte(e.operator>>8), byte(e.operator&0xFF))
	} else {
		out = append(out, byte(e.operator))
	}
	return out
}

// appendLongInt appends a Type-1 longint encoding (29 + int32 big-
// endian). The output is always exactly 5 bytes so DICT-level offset
// arithmetic stays stable when the operand value changes.
func appendLongInt(b []byte, v int32) []byte {
	return append(b, 29, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

// patchInt32 overwrites the four bytes at pos with v in big-endian
// order. pos must point at the int32 payload of a longint operand,
// i.e. one byte after the 0x1D prefix. A negative pos is a no-op so
// callers can pass topDictPatch fields that record "operator absent"
// as -1 without an explicit guard.
func patchInt32(buf []byte, pos int, v int32) {
	if pos < 0 {
		return
	}
	binary.BigEndian.PutUint32(buf[pos:pos+4], uint32(v))
}
