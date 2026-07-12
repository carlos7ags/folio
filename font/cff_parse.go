// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
)

// cffFont is the parsed, read-only view of a CID-keyed CFF v1 font.
// Phase 2 builds this structure; Phase 3+ consumes it to emit a
// subset CFF blob.
//
// Every section is exposed both as a byte range (for verbatim
// copy-through) and via decoded helpers where Phase 3 needs structured
// access. Offsets in this structure are absolute byte positions in raw
// unless explicitly marked relative.
//
// Spec reference: Adobe Technical Note #5176 (CFF v1).
type cffFont struct {
	nameIndex    *cffIndex // §5 INDEX format; one entry holds the PostScript name
	topDictIndex *cffIndex // single-Top-DICT INDEX (CID-keyed CFFs)
	stringIndex  *cffIndex // §10 — SID resolution; copy-through for subsetting
	gsubrIndex   *cffIndex // §16 Global subroutines

	charStringsIndex *cffIndex // §17 CharStrings INDEX (one entry per GID)
	fdArrayIndex     *cffIndex // §19 FDArray INDEX (entries are font DICTs)

	raw []byte // full source CFF bytes

	header []byte // header bytes (Header §6); length = hdrSize

	topDict cffDict // decoded operators of the single Top DICT

	fds []*cffFD // one per FDArray INDEX entry

	// Sections located via Top DICT operands. The byte ranges in raw
	// are computed defensively; sub-format parsing is deferred.
	charsetOffset  int // absolute offset in raw of charset bytes
	charsetSize    int // byte length of charset (incl. format byte)
	fdSelectOffset int
	fdSelectSize   int

	// Decoded Top DICT scalars Phase 3 needs without re-traversing.
	rosRegistry   int64
	rosOrdering   int64
	rosSupplement int64
	cidCount      int

	numGlyphs int // == charStringsIndex.count; mirrored for clarity

}

// cffFD holds the per-FD state of a CID-keyed CFF: font DICT bytes,
// the Private DICT that lives at an absolute offset, and the Local
// Subr INDEX rooted at a relative offset from the Private DICT start.
type cffFD struct {

	// localSubrs may be nil when the FD declares no local Subr INDEX
	// (the Private DICT lacks the Subrs operator). Spec-legal.
	localSubrs *cffIndex
	// fontDictBytes is the FDArray INDEX object i — the bytes of the
	// font DICT used for this FD. Subsetting copies and rewrites this.
	fontDictBytes []byte
	fontDict      cffDict // decoded entries

	privateBytes []byte  // alias to raw[privateOffset:privateOffset+privateSize]
	privateDict  cffDict // decoded entries

	privateOffset int // absolute offset in cffFont.raw
	privateSize   int // byte length
}

// cffIndex is a parsed INDEX structure (TN #5176 §5). Empty INDEXes
// (count == 0) are valid and serialize to two zero bytes; they expose
// no objects and have offSize == 0.
type cffIndex struct {
	offsets     []int
	raw         []byte // aliases cffFont.raw — do not mutate
	rawStart    int    // absolute offset where the INDEX begins in cffFont.raw
	rawEnd      int    // one past the last byte of the INDEX
	count       int
	offSize     int // 1..4 for non-empty INDEXes; 0 for empty
	objectsBase int // absolute offset where object payloads start
}

// Object returns the bytes of object i. The returned slice aliases the
// parent CFF raw buffer; callers must not mutate it. Out-of-range i,
// or any call on an empty INDEX, yields nil so callers can use the
// result with a length check.
func (idx *cffIndex) Object(i int) []byte {
	if idx == nil || i < 0 || i >= idx.count {
		return nil
	}
	start := idx.objectsBase + idx.offsets[i] - 1
	end := idx.objectsBase + idx.offsets[i+1] - 1
	return idx.raw[start:end]
}

// parseCFFIndex reads an INDEX starting at pos within raw. Returns the
// parsed structure (with rawEnd pointing one past the last byte
// consumed) or an error wrapping one of the package sentinels.
//
// The empty-INDEX shortcut (count == 0, 2 bytes total) is recognized:
// such INDEXes have no offSize byte and no offsets table. This matches
// real-world CFFs where, for example, the Encodings section can be
// represented as an empty INDEX in CID-keyed fonts.
func parseCFFIndex(raw []byte, pos int) (*cffIndex, error) {
	if pos < 0 || pos+2 > len(raw) {
		return nil, fmt.Errorf("font: cff: INDEX header truncated at offset %d: %w", pos, ErrTruncated)
	}
	count := int(binary.BigEndian.Uint16(raw[pos : pos+2]))
	if count == 0 {
		return &cffIndex{
			rawStart: pos,
			rawEnd:   pos + 2,
			raw:      raw,
		}, nil
	}
	if pos+3 > len(raw) {
		return nil, fmt.Errorf("font: cff: INDEX offSize truncated at offset %d: %w", pos, ErrTruncated)
	}
	offSize := int(raw[pos+2])
	if offSize < 1 || offSize > 4 {
		return nil, fmt.Errorf("font: cff: INDEX offSize %d out of range: %w", offSize, ErrCorruptTable)
	}
	offBase := pos + 3
	offBytes := (count + 1) * offSize
	if offBase+offBytes > len(raw) {
		return nil, fmt.Errorf("font: cff: INDEX offset table truncated: %w", ErrTruncated)
	}
	offsets := make([]int, count+1)
	for i := range count + 1 {
		offsets[i] = readOffset(raw[offBase+i*offSize:], offSize)
	}
	if offsets[0] != 1 {
		return nil, fmt.Errorf("font: cff: INDEX first offset %d != 1: %w", offsets[0], ErrCorruptTable)
	}
	for i := range count {
		if offsets[i+1] < offsets[i] {
			return nil, fmt.Errorf("font: cff: INDEX offsets non-monotonic at %d: %w", i, ErrCorruptTable)
		}
	}
	objectsBase := offBase + offBytes
	payloadSize := offsets[count] - 1
	end := objectsBase + payloadSize
	if end > len(raw) || end < objectsBase {
		return nil, fmt.Errorf("font: cff: INDEX payload truncated: %w", ErrTruncated)
	}
	return &cffIndex{
		rawStart:    pos,
		rawEnd:      end,
		count:       count,
		offSize:     offSize,
		objectsBase: objectsBase,
		offsets:     offsets,
		raw:         raw,
	}, nil
}

// CFF v1 DICT one-byte operator codes (TN #5176 Table 9).
const (
	cffOpVersion          = 0
	cffOpNotice           = 1
	cffOpFullName         = 2
	cffOpFamilyName       = 3
	cffOpWeight           = 4
	cffOpFontBBox         = 5
	cffOpBlueValues       = 6 // Private only
	cffOpOtherBlues       = 7 // Private only
	cffOpFamilyBlues      = 8 // Private only
	cffOpFamilyOtherBlues = 9 // Private only
	cffOpStdHW            = 10
	cffOpStdVW            = 11
	cffOpEscape           = 12 // 2-byte operator prefix
	cffOpUniqueID         = 13
	cffOpXUID             = 14
	cffOpCharset          = 15
	cffOpEncoding         = 16
	cffOpCharStrings      = 17
	cffOpPrivate          = 18
	cffOpSubrs            = 19 // Private only
	cffOpDefaultWidthX    = 20 // Private only
	cffOpNominalWidthX    = 21 // Private only
)

// CFF v1 DICT two-byte operator codes (TN #5176 Table 10). Encoded as
// (12 << 8) | secondByte so a single int identifies them uniquely.
const (
	cffOp2Copyright          = 12 << 8
	cffOp2IsFixedPitch       = 12<<8 | 1
	cffOp2ItalicAngle        = 12<<8 | 2
	cffOp2UnderlinePosition  = 12<<8 | 3
	cffOp2UnderlineThickness = 12<<8 | 4
	cffOp2PaintType          = 12<<8 | 5
	cffOp2CharstringType     = 12<<8 | 6
	cffOp2FontMatrix         = 12<<8 | 7
	cffOp2StrokeWidth        = 12<<8 | 8
	cffOp2ROS                = 12<<8 | 30
	cffOp2CIDFontVersion     = 12<<8 | 31
	cffOp2CIDFontRevision    = 12<<8 | 32
	cffOp2CIDFontType        = 12<<8 | 33
	cffOp2CIDCount           = 12<<8 | 34
	cffOp2UIDBase            = 12<<8 | 35
	cffOp2FDArray            = 12<<8 | 36
	cffOp2FDSelect           = 12<<8 | 37
	cffOp2FontName           = 12<<8 | 38
)

// cffDictEntry records one (operands, operator) pair parsed from a
// DICT. operator is the one-byte or escaped two-byte code.
//
// Phase 3+ rewrites individual operands of an entry (for example the
// offset half of `Private size offset`), so byte ranges are tracked at
// two granularities:
//
//   - operandStart and operandEnd bracket the run of all operand
//     bytes — operandEnd is the position of the operator byte.
//   - operandSpans[i] is the half-open byte range [start, end) of
//     operand i within the DICT byte stream, in the same order as
//     intOperands.
//
// intOperands holds decoded integer values in order. Slots that came
// from a BCD real operand hold zero; their original bytes can be
// recovered via operandSpans[i] when Phase 3 needs to reproduce the
// real verbatim. realIndices lists those slot indices so callers do
// not have to maintain a separate flag.
type cffDictEntry struct {
	intOperands  []int64
	operandSpans [][2]int
	realIndices  []int
	operator     int
	operandStart int
	operandEnd   int
}

// cffDict is the ordered list of entries parsed from a DICT.
type cffDict []cffDictEntry

// get returns the first entry matching op (1-byte or escaped 2-byte
// code) and ok=true, or zero-value and false if absent. CFF DICTs are
// usually small (a few dozen entries at most) so linear search is fine.
func (d cffDict) get(op int) (cffDictEntry, bool) {
	for _, e := range d {
		if e.operator == op {
			return e, true
		}
	}
	return cffDictEntry{}, false
}

// parseCFFDict decodes a DICT byte stream (TN #5176 §4) into a sequence
// of entries. Operand encodings handled:
//
//   - single-byte int  (b in 32..246)
//   - two-byte int     (b in 247..254)
//   - shortint         (b == 28)
//   - longint          (b == 29)
//   - BCD real         (b == 30)
//
// Operators are one-byte values 0..21 (excluding 22..27 reserved) or
// two-byte escape sequences (12 followed by 0..N). The reserved bytes
// 22..27, 31, 255 abort parsing with ErrCorruptTable.
func parseCFFDict(b []byte) (cffDict, error) {
	var out cffDict
	var operands []int64
	var spans [][2]int
	var realIdx []int
	operandStart := 0
	pos := 0
	for pos < len(b) {
		bv := b[pos]
		opStart := pos
		switch {
		case bv <= 21:
			// Operator. 12 is the 2-byte escape; everything else is
			// a single-byte operator.
			var op, opSize int
			if bv == cffOpEscape {
				if pos+1 >= len(b) {
					return nil, fmt.Errorf("font: cff: DICT 2-byte operator truncated: %w", ErrTruncated)
				}
				op = cffOpEscape<<8 | int(b[pos+1])
				opSize = 2
			} else {
				op = int(bv)
				opSize = 1
			}
			out = append(out, cffDictEntry{
				operator:     op,
				operandStart: operandStart,
				operandEnd:   pos,
				intOperands:  operands,
				operandSpans: spans,
				realIndices:  realIdx,
			})
			pos += opSize
			operands = nil
			spans = nil
			realIdx = nil
			operandStart = pos
		case bv == 28:
			if pos+3 > len(b) {
				return nil, fmt.Errorf("font: cff: DICT shortint truncated: %w", ErrTruncated)
			}
			v := int64(int16(binary.BigEndian.Uint16(b[pos+1 : pos+3])))
			operands = append(operands, v)
			pos += 3
			spans = append(spans, [2]int{opStart, pos})
		case bv == 29:
			if pos+5 > len(b) {
				return nil, fmt.Errorf("font: cff: DICT longint truncated: %w", ErrTruncated)
			}
			v := int64(int32(binary.BigEndian.Uint32(b[pos+1 : pos+5])))
			operands = append(operands, v)
			pos += 5
			spans = append(spans, [2]int{opStart, pos})
		case bv == 30:
			// BCD real. Walk bytes until a nibble equals 0xF (end).
			pos++
			ended := false
			for pos < len(b) {
				v := b[pos]
				pos++
				if v&0x0F == 0x0F || v>>4 == 0x0F {
					ended = true
					break
				}
			}
			if !ended {
				return nil, fmt.Errorf("font: cff: DICT BCD real unterminated: %w", ErrTruncated)
			}
			// Reserve a zero-valued slot in intOperands; the original
			// bytes live in operandSpans for Phase 3 to copy verbatim.
			realIdx = append(realIdx, len(operands))
			operands = append(operands, 0)
			spans = append(spans, [2]int{opStart, pos})
		case bv >= 32 && bv <= 246:
			operands = append(operands, int64(bv)-139)
			pos++
			spans = append(spans, [2]int{opStart, pos})
		case bv >= 247 && bv <= 250:
			if pos+1 >= len(b) {
				return nil, fmt.Errorf("font: cff: DICT 2-byte int truncated: %w", ErrTruncated)
			}
			v := int64(bv-247)*256 + int64(b[pos+1]) + 108
			operands = append(operands, v)
			pos += 2
			spans = append(spans, [2]int{opStart, pos})
		case bv >= 251 && bv <= 254:
			if pos+1 >= len(b) {
				return nil, fmt.Errorf("font: cff: DICT 2-byte int truncated: %w", ErrTruncated)
			}
			v := -int64(bv-251)*256 - int64(b[pos+1]) - 108
			operands = append(operands, v)
			pos += 2
			spans = append(spans, [2]int{opStart, pos})
		default:
			// 22..27, 31, 255 are reserved/invalid in DICT streams.
			return nil, fmt.Errorf("font: cff: DICT reserved byte 0x%02X at offset %d: %w", bv, pos, ErrCorruptTable)
		}
	}
	// Operands not followed by an operator are spec-illegal but we
	// silently drop them rather than fail — a DICT may legitimately
	// terminate after its last operator with no trailing data.
	return out, nil
}

// dictInt fetches the first integer operand of operator op, or
// (0, false) if op is absent or has no integer operands.
func dictInt(d cffDict, op int) (int64, bool) {
	e, ok := d.get(op)
	if !ok || len(e.intOperands) == 0 {
		return 0, false
	}
	return e.intOperands[0], true
}

// parseCFF parses CID-keyed CFF v1 bytes into a structured form. Plain
// (name-keyed) CFF, CFF font collections, and CFF2 are rejected with
// ErrCorruptTable or ErrUnknownFormat so Phase 3's subsetter and
// embedder can rely on the CID-keyed invariants (ROS triplet, FDArray,
// FDSelect, CIDCount).
//
// The function performs only the structural decoding needed by Phase 3+:
// charstring opcodes are not interpreted, charset/FDSelect formats are
// validated for size but not for content. Any structural inconsistency
// returns an error wrapping one of the package sentinels.
func parseCFF(raw []byte) (*cffFont, error) {
	if len(raw) < 4 {
		return nil, fmt.Errorf("font: cff: header truncated: %w", ErrTruncated)
	}
	if raw[0] != 1 {
		return nil, fmt.Errorf("font: cff: unsupported major %d: %w", raw[0], ErrUnknownFormat)
	}
	hdrSize := int(raw[2])
	if hdrSize < 4 || hdrSize > len(raw) {
		return nil, fmt.Errorf("font: cff: bad hdrSize %d: %w", hdrSize, ErrCorruptTable)
	}

	pos := hdrSize
	nameIdx, err := parseCFFIndex(raw, pos)
	if err != nil {
		return nil, fmt.Errorf("font: cff: name index: %w", err)
	}
	pos = nameIdx.rawEnd

	topDictIdx, err := parseCFFIndex(raw, pos)
	if err != nil {
		return nil, fmt.Errorf("font: cff: top dict index: %w", err)
	}
	if topDictIdx.count != 1 {
		return nil, fmt.Errorf("font: cff: top dict count %d != 1 (font collection unsupported): %w", topDictIdx.count, ErrCorruptTable)
	}
	pos = topDictIdx.rawEnd

	topDict, err := parseCFFDict(topDictIdx.Object(0))
	if err != nil {
		return nil, fmt.Errorf("font: cff: top dict: %w", err)
	}

	// Require ROS so callers can rely on CID-keyed semantics. Charset
	// and CharStrings and FDArray/FDSelect are also mandatory for CID-
	// keyed CFFs (TN #5176 §18); enforce them explicitly.
	rosEntry, ok := topDict.get(cffOp2ROS)
	if !ok || len(rosEntry.intOperands) < 3 {
		return nil, fmt.Errorf("font: cff: missing or malformed ROS operator: %w", ErrCorruptTable)
	}

	stringIdx, err := parseCFFIndex(raw, pos)
	if err != nil {
		return nil, fmt.Errorf("font: cff: string index: %w", err)
	}
	pos = stringIdx.rawEnd

	gsubrIdx, err := parseCFFIndex(raw, pos)
	if err != nil {
		return nil, fmt.Errorf("font: cff: gsubr index: %w", err)
	}

	charStringsOff, ok := dictInt(topDict, cffOpCharStrings)
	if !ok {
		return nil, fmt.Errorf("font: cff: missing CharStrings operator: %w", ErrCorruptTable)
	}
	fdArrayOff, ok := dictInt(topDict, cffOp2FDArray)
	if !ok {
		return nil, fmt.Errorf("font: cff: missing FDArray operator: %w", ErrCorruptTable)
	}
	fdSelectOff, ok := dictInt(topDict, cffOp2FDSelect)
	if !ok {
		return nil, fmt.Errorf("font: cff: missing FDSelect operator: %w", ErrCorruptTable)
	}
	// CID-keyed CFFs must declare a custom charset explicitly
	// (TN #5176 §18). The implicit-default values 0 (ISOAdobe), 1
	// (Expert), and 2 (ExpertSubset) are reserved for non-CID fonts;
	// here they would also be treated as absolute byte offsets and
	// computeCharsetSize would interpret CFF header bytes as a
	// charset format byte, yielding nonsense. Reject early.
	charsetOff, ok := dictInt(topDict, cffOpCharset)
	if !ok {
		return nil, fmt.Errorf("font: cff: CID-keyed font missing charset operator: %w", ErrCorruptTable)
	}
	if charsetOff < 3 {
		return nil, fmt.Errorf("font: cff: CID-keyed font using predefined charset %d (reserved for non-CID CFF): %w", charsetOff, ErrCorruptTable)
	}
	cidCountVal, _ := dictInt(topDict, cffOp2CIDCount)
	cidCount := int(cidCountVal)
	if cidCount == 0 {
		// Default per TN #5176 §18 Table 11.
		cidCount = 8720
	}

	charStringsIdx, err := parseCFFIndex(raw, int(charStringsOff))
	if err != nil {
		return nil, fmt.Errorf("font: cff: charstrings index: %w", err)
	}
	numGlyphs := charStringsIdx.count
	if numGlyphs == 0 {
		return nil, fmt.Errorf("font: cff: charstrings index empty: %w", ErrCorruptTable)
	}

	fdArrayIdx, err := parseCFFIndex(raw, int(fdArrayOff))
	if err != nil {
		return nil, fmt.Errorf("font: cff: fdarray index: %w", err)
	}
	if fdArrayIdx.count == 0 {
		return nil, fmt.Errorf("font: cff: empty FDArray INDEX: %w", ErrCorruptTable)
	}

	charsetSize, err := computeCharsetSize(raw, int(charsetOff), numGlyphs)
	if err != nil {
		return nil, fmt.Errorf("font: cff: charset: %w", err)
	}
	fdSelectSize, err := computeFDSelectSize(raw, int(fdSelectOff), numGlyphs)
	if err != nil {
		return nil, fmt.Errorf("font: cff: fdselect: %w", err)
	}

	fds := make([]*cffFD, fdArrayIdx.count)
	for i := range fdArrayIdx.count {
		fd, err := parseCFFFD(raw, fdArrayIdx, i)
		if err != nil {
			return nil, fmt.Errorf("font: cff: fd[%d]: %w", i, err)
		}
		fds[i] = fd
	}

	return &cffFont{
		raw:              raw,
		header:           raw[:hdrSize],
		nameIndex:        nameIdx,
		topDictIndex:     topDictIdx,
		stringIndex:      stringIdx,
		gsubrIndex:       gsubrIdx,
		topDict:          topDict,
		charsetOffset:    int(charsetOff),
		charsetSize:      charsetSize,
		fdSelectOffset:   int(fdSelectOff),
		fdSelectSize:     fdSelectSize,
		charStringsIndex: charStringsIdx,
		fdArrayIndex:     fdArrayIdx,
		rosRegistry:      rosEntry.intOperands[0],
		rosOrdering:      rosEntry.intOperands[1],
		rosSupplement:    rosEntry.intOperands[2],
		cidCount:         cidCount,
		numGlyphs:        numGlyphs,
		fds:              fds,
	}, nil
}

// parseCFFFD reads one entry from the FDArray INDEX as a font DICT,
// then follows its Private operator to the Private DICT (absolute
// offset) and the Private DICT's Subrs operator to the Local Subr
// INDEX (relative offset). Returns a cffFD ready for subsetting.
func parseCFFFD(raw []byte, fdArrayIdx *cffIndex, i int) (*cffFD, error) {
	fontDictBytes := fdArrayIdx.Object(i)
	if fontDictBytes == nil {
		return nil, fmt.Errorf("font: cff: fd %d missing: %w", i, ErrCorruptTable)
	}
	fontDict, err := parseCFFDict(fontDictBytes)
	if err != nil {
		return nil, fmt.Errorf("font: cff: fd %d font dict: %w", i, err)
	}
	privEntry, ok := fontDict.get(cffOpPrivate)
	if !ok || len(privEntry.intOperands) != 2 {
		return nil, fmt.Errorf("font: cff: fd %d Private operator missing or malformed: %w", i, ErrCorruptTable)
	}
	privSize := int(privEntry.intOperands[0])
	privOff := int(privEntry.intOperands[1])
	if privSize < 0 || privOff < 0 || privOff+privSize > len(raw) {
		return nil, fmt.Errorf("font: cff: fd %d Private range [%d:%d] out of raw bounds: %w", i, privOff, privOff+privSize, ErrCorruptTable)
	}
	privBytes := raw[privOff : privOff+privSize]
	privDict, err := parseCFFDict(privBytes)
	if err != nil {
		return nil, fmt.Errorf("font: cff: fd %d private dict: %w", i, err)
	}
	var localSubrs *cffIndex
	if subrsEntry, ok := privDict.get(cffOpSubrs); ok {
		if len(subrsEntry.intOperands) != 1 {
			return nil, fmt.Errorf("font: cff: fd %d Subrs operand count %d: %w", i, len(subrsEntry.intOperands), ErrCorruptTable)
		}
		subrsRel := int(subrsEntry.intOperands[0])
		localSubrs, err = parseCFFIndex(raw, privOff+subrsRel)
		if err != nil {
			return nil, fmt.Errorf("font: cff: fd %d local subr index: %w", i, err)
		}
	}
	return &cffFD{
		fontDictBytes: fontDictBytes,
		fontDict:      fontDict,
		privateOffset: privOff,
		privateSize:   privSize,
		privateBytes:  privBytes,
		privateDict:   privDict,
		localSubrs:    localSubrs,
	}, nil
}

// computeCharsetSize returns the byte length of a CID-keyed charset
// starting at off. TN #5176 §13 defines three formats:
//
//   - 0:  fmt(1) + (numGlyphs-1) * SID(2)
//   - 1:  fmt(1) + ranges each of (first SID(2), nLeft uint8(1))
//   - 2:  fmt(1) + ranges each of (first SID(2), nLeft uint16(2))
//
// Formats 1 and 2 enumerate ranges until cumulative glyphs covered
// reaches numGlyphs-1 (charset omits GID 0 — .notdef is implicit).
func computeCharsetSize(raw []byte, off, numGlyphs int) (int, error) {
	if off < 0 || off >= len(raw) {
		return 0, fmt.Errorf("font: cff: charset offset out of range: %w", ErrCorruptTable)
	}
	if numGlyphs < 1 {
		return 0, fmt.Errorf("font: cff: numGlyphs %d invalid: %w", numGlyphs, ErrCorruptTable)
	}
	format := raw[off]
	remaining := numGlyphs - 1 // glyphs to cover, excluding GID 0
	switch format {
	case 0:
		size := 1 + remaining*2
		if off+size > len(raw) {
			return 0, fmt.Errorf("font: cff: charset format 0 truncated: %w", ErrTruncated)
		}
		return size, nil
	case 1:
		pos := off + 1
		for remaining > 0 {
			if pos+3 > len(raw) {
				return 0, fmt.Errorf("font: cff: charset format 1 truncated: %w", ErrTruncated)
			}
			nLeft := int(raw[pos+2])
			remaining -= nLeft + 1
			pos += 3
		}
		if remaining < 0 {
			return 0, fmt.Errorf("font: cff: charset format 1 over-covers glyphs by %d: %w", -remaining, ErrCorruptTable)
		}
		return pos - off, nil
	case 2:
		pos := off + 1
		for remaining > 0 {
			if pos+4 > len(raw) {
				return 0, fmt.Errorf("font: cff: charset format 2 truncated: %w", ErrTruncated)
			}
			nLeft := int(binary.BigEndian.Uint16(raw[pos+2 : pos+4]))
			remaining -= nLeft + 1
			pos += 4
		}
		if remaining < 0 {
			return 0, fmt.Errorf("font: cff: charset format 2 over-covers glyphs by %d: %w", -remaining, ErrCorruptTable)
		}
		return pos - off, nil
	default:
		return 0, fmt.Errorf("font: cff: charset format %d unknown: %w", format, ErrCorruptTable)
	}
}

// computeFDSelectSize returns the byte length of an FDSelect table
// at off. TN #5176 §19 defines two formats:
//
//   - 0: fmt(1) + numGlyphs * fd(uint8)
//   - 3: fmt(1) + nRanges(uint16) + nRanges*(first uint16 + fd uint8) + sentinel uint16
func computeFDSelectSize(raw []byte, off, numGlyphs int) (int, error) {
	if off < 0 || off >= len(raw) {
		return 0, fmt.Errorf("font: cff: fdselect offset out of range: %w", ErrCorruptTable)
	}
	format := raw[off]
	switch format {
	case 0:
		size := 1 + numGlyphs
		if off+size > len(raw) {
			return 0, fmt.Errorf("font: cff: fdselect format 0 truncated: %w", ErrTruncated)
		}
		return size, nil
	case 3:
		if off+3 > len(raw) {
			return 0, fmt.Errorf("font: cff: fdselect format 3 header truncated: %w", ErrTruncated)
		}
		nRanges := int(binary.BigEndian.Uint16(raw[off+1 : off+3]))
		size := 1 + 2 + nRanges*3 + 2
		if off+size > len(raw) {
			return 0, fmt.Errorf("font: cff: fdselect format 3 body truncated: %w", ErrTruncated)
		}
		// Per TN #5176 §19, the trailing sentinel uint16 must equal
		// numGlyphs — it terminates the implied range that begins at
		// the last `first` field. A mismatch means either the table
		// was authored against a different glyph count or the bytes
		// are corrupt; either way Phase 3 cannot rely on the FD
		// assignment, so reject up-front.
		sentinel := int(binary.BigEndian.Uint16(raw[off+size-2 : off+size]))
		if sentinel != numGlyphs {
			return 0, fmt.Errorf("font: cff: fdselect format 3 sentinel %d != numGlyphs %d: %w", sentinel, numGlyphs, ErrCorruptTable)
		}
		return size, nil
	default:
		return 0, fmt.Errorf("font: cff: fdselect format %d unknown: %w", format, ErrCorruptTable)
	}
}
