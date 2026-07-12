// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// Name IDs Folio cares about. The full list lives in §5.2.5; we
// surface only the two needed for PDF /BaseFont selection.
const (
	nameIDFull       = 4
	nameIDPostScript = 6
)

// nameRecords holds the resolved name strings Folio reads from the
// `name` table. Empty strings indicate the corresponding NameID was
// absent or no supported encoding was available.
type nameRecords struct {
	postScript string
	full       string
}

// parseName decodes the `name` table from raw bytes and returns the
// name strings Folio needs. Format-1 tables (with language-tag
// records appended after the name records) are accepted; the trailing
// language-tag block is ignored because Folio does not consult
// language-specific names.
//
// Spec: ISO/IEC 14496-22 §5.2.5 — Naming Table layout.
//
// Header (6 bytes):
//   - format (uint16)       at 0  — 0 or 1
//   - count (uint16)        at 2  — number of name records
//   - stringOffset (Offset16) at 4 — offset (from table start) to the
//     string storage block
//
// Each name record is 12 bytes:
//   - platformID (uint16)
//   - encodingID (uint16)
//   - languageID (uint16)
//   - nameID (uint16)
//   - length (uint16)         — bytes within string storage
//   - stringOffset (Offset16) — offset from stringOffset header field
//
// Encoding selection priority for each NameID, picking the best
// available encoding:
//  1. Platform 3 (Windows) Encoding 1 (Unicode BMP) or 10 (UCS-4) →
//     UTF-16BE (most modern fonts)
//  2. Platform 0 (Unicode, any encoding)              → UTF-16BE
//  3. Platform 1 (Macintosh) Encoding 0 (Roman)       → MacRoman
func parseName(data []byte) (nameRecords, error) {
	dataLen := uint64(len(data))
	if dataLen < 6 {
		return nameRecords{}, fmt.Errorf("font: name: header truncated (%d < 6): %w", dataLen, ErrTruncated)
	}
	count := uint64(binary.BigEndian.Uint16(data[2:4]))
	stringOff := uint64(binary.BigEndian.Uint16(data[4:6]))
	recordsEnd := 6 + count*12
	if recordsEnd > dataLen {
		return nameRecords{}, fmt.Errorf("font: name: record array truncated (need %d, have %d): %w", recordsEnd, dataLen, ErrTruncated)
	}
	if stringOff > dataLen {
		return nameRecords{}, fmt.Errorf("font: name: stringOffset %d beyond table length %d: %w", stringOff, dataLen, ErrCorruptTable)
	}

	type candidate struct {
		dec   func([]byte) string
		data  []byte
		score int
	}
	best := map[uint16]candidate{}

	for i := uint64(0); i < count; i++ {
		off := 6 + i*12
		platformID := binary.BigEndian.Uint16(data[off : off+2])
		encodingID := binary.BigEndian.Uint16(data[off+2 : off+4])
		nameID := binary.BigEndian.Uint16(data[off+6 : off+8])
		length := uint64(binary.BigEndian.Uint16(data[off+8 : off+10]))
		recOff := uint64(binary.BigEndian.Uint16(data[off+10 : off+12]))
		if nameID != nameIDFull && nameID != nameIDPostScript {
			continue
		}
		score, dec := scoreNameEncoding(platformID, encodingID)
		if score == 0 {
			continue
		}
		strStart := stringOff + recOff
		strEnd := strStart + length
		if strEnd > dataLen {
			// Skip malformed record rather than failing the whole parse.
			continue
		}
		prev, ok := best[nameID]
		if !ok || score > prev.score {
			best[nameID] = candidate{score: score, data: data[strStart:strEnd], dec: dec}
		}
	}

	var nm nameRecords
	if c, ok := best[nameIDPostScript]; ok {
		nm.postScript = c.dec(c.data)
	}
	if c, ok := best[nameIDFull]; ok {
		nm.full = c.dec(c.data)
	}
	return nm, nil
}

// scoreNameEncoding returns a priority for the (platformID, encodingID)
// pair. Higher score wins; 0 means the pair is not supported. The
// returned decoder is paired with the score so the caller doesn't have
// to dispatch a second time.
func scoreNameEncoding(platformID, encodingID uint16) (int, func([]byte) string) {
	switch platformID {
	case 3: // Windows
		switch encodingID {
		case 1, 10: // Unicode BMP (UCS-2) or UCS-4 — both UTF-16BE on the wire
			return 30, decodeUTF16BE
		}
	case 0: // Unicode
		// All Unicode-platform encodings store strings as UTF-16BE.
		return 20, decodeUTF16BE
	case 1: // Macintosh
		if encodingID == 0 { // Roman
			return 10, decodeMacRoman
		}
	}
	return 0, nil
}

// decodeUTF16BE decodes a UTF-16 big-endian byte slice into a Go
// string, joining UTF-16 surrogate pairs into the corresponding
// supplementary-plane runes per Unicode §3.8. Odd-length slices have
// their trailing byte ignored — the spec mandates an even-byte count
// but a corrupt record is preferable to a parse failure.
func decodeUTF16BE(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	u := make([]uint16, len(b)/2)
	for i := range u {
		u[i] = binary.BigEndian.Uint16(b[i*2 : i*2+2])
	}
	return string(utf16.Decode(u))
}

// decodeMacRoman decodes a Mac OS Roman byte slice into a Go string
// using the Apple encoding table. Bytes 0x00..0x7F map to ASCII;
// 0x80..0xFF map to the runes in macRomanHigh below. The table is
// derived from the Unicode Consortium's ROMAN.TXT mapping (file dated
// 1995-04-15 by Unicode); see
// https://unicode.org/Public/MAPPINGS/VENDORS/APPLE/ROMAN.TXT.
func decodeMacRoman(b []byte) string {
	out := make([]rune, len(b))
	for i, c := range b {
		if c < 0x80 {
			out[i] = rune(c)
			continue
		}
		out[i] = macRomanHigh[c-0x80]
	}
	return string(out)
}

// macRomanHigh holds the 128 entries for Mac OS Roman bytes
// 0x80..0xFF. Source: Unicode Consortium ROMAN.TXT.
var macRomanHigh = [128]rune{
	0x00C4, 0x00C5, 0x00C7, 0x00C9, 0x00D1, 0x00D6, 0x00DC, 0x00E1,
	0x00E0, 0x00E2, 0x00E4, 0x00E3, 0x00E5, 0x00E7, 0x00E9, 0x00E8,
	0x00EA, 0x00EB, 0x00ED, 0x00EC, 0x00EE, 0x00EF, 0x00F1, 0x00F3,
	0x00F2, 0x00F4, 0x00F6, 0x00F5, 0x00FA, 0x00F9, 0x00FB, 0x00FC,
	0x2020, 0x00B0, 0x00A2, 0x00A3, 0x00A7, 0x2022, 0x00B6, 0x00DF,
	0x00AE, 0x00A9, 0x2122, 0x00B4, 0x00A8, 0x2260, 0x00C6, 0x00D8,
	0x221E, 0x00B1, 0x2264, 0x2265, 0x00A5, 0x00B5, 0x2202, 0x2211,
	0x220F, 0x03C0, 0x222B, 0x00AA, 0x00BA, 0x03A9, 0x00E6, 0x00F8,
	0x00BF, 0x00A1, 0x00AC, 0x221A, 0x0192, 0x2248, 0x2206, 0x00AB,
	0x00BB, 0x2026, 0x00A0, 0x00C0, 0x00C3, 0x00D5, 0x0152, 0x0153,
	0x2013, 0x2014, 0x201C, 0x201D, 0x2018, 0x2019, 0x00F7, 0x25CA,
	0x00FF, 0x0178, 0x2044, 0x20AC, 0x2039, 0x203A, 0xFB01, 0xFB02,
	0x2021, 0x00B7, 0x201A, 0x201E, 0x2030, 0x00C2, 0x00CA, 0x00C1,
	0x00CB, 0x00C8, 0x00CD, 0x00CE, 0x00CF, 0x00CC, 0x00D3, 0x00D4,
	0xF8FF, 0x00D2, 0x00DA, 0x00DB, 0x00D9, 0x0131, 0x02C6, 0x02DC,
	0x00AF, 0x02D8, 0x02D9, 0x02DA, 0x00B8, 0x02DD, 0x02DB, 0x02C7,
}
