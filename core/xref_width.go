// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

// XRefStreamWidths returns the byte widths of the three fields of a
// cross-reference stream entry, sized to fit the largest value each field
// must hold (ISO 32000-1 §7.5.8.2).
//
//   - field 1 is the entry type: 0 (free), 1 (in-use uncompressed), or
//     2 (in-use compressed in an object stream). Always 1 byte.
//   - field 2 holds either a byte offset (type 1) or the object number of
//     the containing object stream (type 2). It must be wide enough for
//     the larger of maxOffset and maxObjStmNum.
//   - field 3 holds either the generation number (type 1) or the index of
//     the object within its containing object stream (type 2). It must be
//     wide enough for the larger of maxGen and maxIndex.
//
// Per §7.5.8.2 a width of zero is permitted and means "the field is not
// present and a default value is used", but the default for field 2 is 0
// and for field 3 is 0 — both meaningless for any non-trivial document —
// so this function returns a minimum width of 1 byte for fields 2 and 3
// even when the maximum value is 0. The spec allows this; it costs at
// most two bytes per entry and keeps the encoder simple.
//
// All inputs must be non-negative.
func XRefStreamWidths(maxOffset, maxGen, maxObjStmNum, maxIndex int) [3]int {
	field2Max := max(maxOffset, maxObjStmNum)
	field3Max := max(maxGen, maxIndex)
	return [3]int{1, byteWidth(field2Max), byteWidth(field3Max)}
}

// byteWidth returns the smallest number of bytes (>=1) needed to represent
// v as an unsigned big-endian integer. Negative inputs are treated as 0.
func byteWidth(v int) int {
	if v <= 0 {
		return 1
	}
	w := 0
	for v > 0 {
		w++
		v >>= 8
	}
	return w
}
