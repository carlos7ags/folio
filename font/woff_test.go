// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package font

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"os"
	"sort"
	"testing"
)

// buildWOFF creates a WOFF1 file from raw TTF data for testing purposes.
func buildWOFF(t *testing.T, ttfData []byte) []byte {
	t.Helper()

	if len(ttfData) < 12 {
		t.Fatal("TTF data too short")
	}

	flavor := binary.BigEndian.Uint32(ttfData[0:4])
	numTables := int(binary.BigEndian.Uint16(ttfData[4:6]))

	if len(ttfData) < 12+numTables*16 {
		t.Fatal("TTF data too short for table directory")
	}

	type tableInfo struct {
		tag      uint32
		checksum uint32
		data     []byte
	}

	var tables []tableInfo
	for i := range numTables {
		recOff := 12 + i*16
		tag := binary.BigEndian.Uint32(ttfData[recOff : recOff+4])
		checksum := binary.BigEndian.Uint32(ttfData[recOff+4 : recOff+8])
		offset := binary.BigEndian.Uint32(ttfData[recOff+8 : recOff+12])
		length := binary.BigEndian.Uint32(ttfData[recOff+12 : recOff+16])
		if int(offset+length) > len(ttfData) {
			t.Fatalf("table %d extends beyond TTF data", i)
		}
		tables = append(tables, tableInfo{
			tag:      tag,
			checksum: checksum,
			data:     ttfData[offset : offset+length],
		})
	}

	// Sort tables by tag (WOFF requires this).
	sort.Slice(tables, func(a, b int) bool {
		return tables[a].tag < tables[b].tag
	})

	// Build WOFF file.
	// Header: 44 bytes, table directory: numTables * 20 bytes, then table data.
	woffDirEnd := woffHeaderSize + len(tables)*woffTableDirEntrySize

	var buf bytes.Buffer
	buf.Grow(woffDirEnd + len(ttfData)) // rough estimate

	// Reserve space for header + directory (will fill in later).
	buf.Write(make([]byte, woffDirEnd))

	// Write table data (compress each table with zlib).
	type woffEntry struct {
		tag      uint32
		offset   uint32
		compLen  uint32
		origLen  uint32
		checksum uint32
	}
	woffEntries := make([]woffEntry, len(tables))

	for i, tbl := range tables {
		// Pad to 4-byte boundary before each table.
		for buf.Len()%4 != 0 {
			buf.WriteByte(0)
		}

		tableOffset := uint32(buf.Len())

		// Compress with zlib.
		var compressed bytes.Buffer
		w, err := zlib.NewWriterLevel(&compressed, zlib.BestCompression)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(tbl.data)
		_ = w.Close()

		compData := compressed.Bytes()
		compLen := uint32(len(compData))
		origLen := uint32(len(tbl.data))

		// Only use compression if it actually shrinks the data.
		if compLen < origLen {
			buf.Write(compData)
		} else {
			buf.Write(tbl.data)
			compLen = origLen
		}

		woffEntries[i] = woffEntry{
			tag:      tbl.tag,
			offset:   tableOffset,
			compLen:  compLen,
			origLen:  origLen,
			checksum: tbl.checksum,
		}
	}

	// Pad final to 4-byte boundary.
	for buf.Len()%4 != 0 {
		buf.WriteByte(0)
	}

	woffBytes := buf.Bytes()
	totalLength := uint32(len(woffBytes))

	// Calculate totalSfntSize.
	totalSfntSize := uint32(12 + numTables*16)
	for _, e := range woffEntries {
		totalSfntSize += (e.origLen + 3) &^ 3
	}

	// Fill in WOFF header.
	binary.BigEndian.PutUint32(woffBytes[0:4], woffMagic)
	binary.BigEndian.PutUint32(woffBytes[4:8], flavor)
	binary.BigEndian.PutUint32(woffBytes[8:12], totalLength)
	binary.BigEndian.PutUint16(woffBytes[12:14], uint16(numTables))
	binary.BigEndian.PutUint16(woffBytes[14:16], 0) // reserved
	binary.BigEndian.PutUint32(woffBytes[16:20], totalSfntSize)
	binary.BigEndian.PutUint16(woffBytes[20:22], 1) // majorVersion
	binary.BigEndian.PutUint16(woffBytes[22:24], 0) // minorVersion
	binary.BigEndian.PutUint32(woffBytes[24:28], 0) // metaOffset
	binary.BigEndian.PutUint32(woffBytes[28:32], 0) // metaLength
	binary.BigEndian.PutUint32(woffBytes[32:36], 0) // metaOrigLength
	binary.BigEndian.PutUint32(woffBytes[36:40], 0) // privOffset
	binary.BigEndian.PutUint32(woffBytes[40:44], 0) // privLength

	// Fill in table directory.
	for i, e := range woffEntries {
		off := woffHeaderSize + i*woffTableDirEntrySize
		binary.BigEndian.PutUint32(woffBytes[off:off+4], e.tag)
		binary.BigEndian.PutUint32(woffBytes[off+4:off+8], e.offset)
		binary.BigEndian.PutUint32(woffBytes[off+8:off+12], e.compLen)
		binary.BigEndian.PutUint32(woffBytes[off+12:off+16], e.origLen)
		binary.BigEndian.PutUint32(woffBytes[off+16:off+20], e.checksum)
	}

	return woffBytes
}

func TestDecodeWOFF(t *testing.T) {
	path := testFontPath(t)
	ttfData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	woffData := buildWOFF(t, ttfData)

	// Verify it starts with WOFF magic.
	sig := binary.BigEndian.Uint32(woffData[0:4])
	if sig != woffMagic {
		t.Fatalf("expected WOFF magic, got 0x%08X", sig)
	}

	// Decode WOFF back to TTF.
	decoded, err := decodeWOFF(woffData)
	if err != nil {
		t.Fatalf("decodeWOFF failed: %v", err)
	}

	// The decoded TTF should be parseable.
	face, err := ParseTTF(decoded)
	if err != nil {
		t.Fatalf("ParseTTF on decoded WOFF failed: %v", err)
	}

	// Verify it has the same properties as the original.
	origFace, err := ParseTTF(ttfData)
	if err != nil {
		t.Fatalf("ParseTTF on original failed: %v", err)
	}

	if face.PostScriptName() != origFace.PostScriptName() {
		t.Errorf("PostScriptName mismatch: got %q, want %q", face.PostScriptName(), origFace.PostScriptName())
	}
	if face.UnitsPerEm() != origFace.UnitsPerEm() {
		t.Errorf("UnitsPerEm mismatch: got %d, want %d", face.UnitsPerEm(), origFace.UnitsPerEm())
	}
	if face.NumGlyphs() != origFace.NumGlyphs() {
		t.Errorf("NumGlyphs mismatch: got %d, want %d", face.NumGlyphs(), origFace.NumGlyphs())
	}
}

func TestParseFontDispatch(t *testing.T) {
	path := testFontPath(t)
	ttfData, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// ParseFont with TTF data should work.
	face1, err := ParseFont(ttfData)
	if err != nil {
		t.Fatalf("ParseFont(TTF) failed: %v", err)
	}

	// ParseFont with WOFF data should also work.
	woffData := buildWOFF(t, ttfData)
	face2, err := ParseFont(woffData)
	if err != nil {
		t.Fatalf("ParseFont(WOFF) failed: %v", err)
	}

	if face1.PostScriptName() != face2.PostScriptName() {
		t.Errorf("PostScriptName mismatch: TTF=%q, WOFF=%q", face1.PostScriptName(), face2.PostScriptName())
	}
}

func TestDecodeWOFF_Errors(t *testing.T) {
	t.Run("too short", func(t *testing.T) {
		_, err := decodeWOFF([]byte{0x77, 0x4F, 0x46})
		if err == nil {
			t.Fatal("expected error for truncated data")
		}
	})

	t.Run("bad magic", func(t *testing.T) {
		data := make([]byte, woffHeaderSize)
		binary.BigEndian.PutUint32(data[0:4], 0xDEADBEEF)
		_, err := decodeWOFF(data)
		if err == nil {
			t.Fatal("expected error for bad magic")
		}
	})

	t.Run("truncated table directory", func(t *testing.T) {
		data := make([]byte, woffHeaderSize)
		binary.BigEndian.PutUint32(data[0:4], woffMagic)
		binary.BigEndian.PutUint16(data[12:14], 5) // 5 tables but no directory space
		_, err := decodeWOFF(data)
		if err == nil {
			t.Fatal("expected error for truncated table directory")
		}
	})

	t.Run("table extends beyond file", func(t *testing.T) {
		// 1 table, directory fits, but table data offset is out of bounds.
		size := woffHeaderSize + woffTableDirEntrySize
		data := make([]byte, size)
		binary.BigEndian.PutUint32(data[0:4], woffMagic)
		binary.BigEndian.PutUint16(data[12:14], 1)
		// Table entry: offset pointing way beyond file.
		entryOff := woffHeaderSize
		binary.BigEndian.PutUint32(data[entryOff+4:entryOff+8], 9999)  // offset
		binary.BigEndian.PutUint32(data[entryOff+8:entryOff+12], 100)  // compLength
		binary.BigEndian.PutUint32(data[entryOff+12:entryOff+16], 100) // origLength
		_, err := decodeWOFF(data)
		if err == nil {
			t.Fatal("expected error for table extending beyond file")
		}
	})

	t.Run("no tables", func(t *testing.T) {
		data := make([]byte, woffHeaderSize)
		binary.BigEndian.PutUint32(data[0:4], woffMagic)
		binary.BigEndian.PutUint16(data[12:14], 0)
		_, err := decodeWOFF(data)
		if err == nil {
			t.Fatal("expected error for zero tables")
		}
	})
}

func TestParseFont_TooShort(t *testing.T) {
	_, err := ParseFont([]byte{0, 1})
	if err == nil {
		t.Fatal("expected error for data too short")
	}
}
