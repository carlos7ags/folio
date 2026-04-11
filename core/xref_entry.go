// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import "fmt"

// XRefEntryType is the value of the first field of a cross-reference
// stream entry (ISO 32000-1 §7.5.8.3 Table 18).
type XRefEntryType uint8

const (
	// XRefEntryFree marks an entry as a free object (linked-list head/body).
	// Field 2 is the object number of the next free object; field 3 is the
	// generation number to use if the slot is reused. The traditional xref
	// table uses 65535 as the head sentinel; xref streams use the same
	// convention (§7.5.8.3).
	XRefEntryFree XRefEntryType = 0

	// XRefEntryInUse marks an entry as an in-use object stored at a byte
	// offset from the start of the file. Field 2 is the offset; field 3 is
	// the generation number.
	XRefEntryInUse XRefEntryType = 1

	// XRefEntryCompressed marks an entry as an in-use object stored inside
	// an object stream (§7.5.7). Field 2 is the object number of the
	// containing object stream; field 3 is the index of the object within
	// that stream. The generation number is implicitly zero.
	XRefEntryCompressed XRefEntryType = 2
)

// XRefStreamEntry is one row of a cross-reference stream's binary payload.
// It is intentionally an unencoded value type — the byte layout is decided
// at encode time once all entries are known and the field widths are
// chosen.
type XRefStreamEntry struct {
	Type   XRefEntryType
	Field2 uint64
	Field3 uint64
}

// EncodeXRefStreamEntry writes one entry into dst using the given field
// widths. dst must have length widths[0]+widths[1]+widths[2]. Each field
// is written big-endian and zero-padded to its assigned width. Returns an
// error if any field value does not fit in its assigned width.
func EncodeXRefStreamEntry(dst []byte, e XRefStreamEntry, widths [3]int) error {
	total := widths[0] + widths[1] + widths[2]
	if len(dst) != total {
		return fmt.Errorf("xref entry: dst length %d, want %d", len(dst), total)
	}
	if err := putUintBE(dst[:widths[0]], uint64(e.Type)); err != nil {
		return fmt.Errorf("xref entry field 1: %w", err)
	}
	if err := putUintBE(dst[widths[0]:widths[0]+widths[1]], e.Field2); err != nil {
		return fmt.Errorf("xref entry field 2: %w", err)
	}
	if err := putUintBE(dst[widths[0]+widths[1]:], e.Field3); err != nil {
		return fmt.Errorf("xref entry field 3: %w", err)
	}
	return nil
}

// putUintBE writes v into dst as a big-endian unsigned integer. Returns an
// error if v does not fit in len(dst) bytes. A zero-length dst accepts
// only v == 0 (which encodes as the empty byte sequence) — see §7.5.8.2
// on a width of zero meaning "use the default value", which for any field
// is zero.
func putUintBE(dst []byte, v uint64) error {
	if len(dst) == 0 {
		if v != 0 {
			return fmt.Errorf("value %d does not fit in 0 bytes", v)
		}
		return nil
	}
	if len(dst) < 8 {
		limit := uint64(1) << (uint(len(dst)) * 8)
		if v >= limit {
			return fmt.Errorf("value %d does not fit in %d bytes", v, len(dst))
		}
	}
	for i := len(dst) - 1; i >= 0; i-- {
		dst[i] = byte(v)
		v >>= 8
	}
	return nil
}
