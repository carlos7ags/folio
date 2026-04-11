// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/core"
)

// xrefEntry is a single entry in the cross-reference table.
type xrefEntry struct {
	offset     int64 // byte offset (type 1) or object stream number (type 2)
	generation int   // generation number (type 1) or index in object stream (type 2)
	inUse      bool  // true if in-use, false if free
	compressed bool  // true if this object is stored in an object stream (type 2)
}

// xrefTable maps object numbers to their xref entries.
type xrefTable struct {
	entries map[int]xrefEntry
	trailer *core.PdfDictionary
}

// findStartXref scans backwards from the end of the file to find the
// startxref offset. Returns the byte offset to the xref table.
func findStartXref(data []byte) (int64, error) {
	// Search backwards from EOF for "startxref".
	searchLen := min(len(data), 1024)
	tail := data[len(data)-searchLen:]

	// Find last occurrence of "startxref".
	marker := []byte("startxref")
	idx := -1
	for i := len(tail) - len(marker); i >= 0; i-- {
		if string(tail[i:i+len(marker)]) == string(marker) {
			idx = i
			break
		}
	}
	if idx < 0 {
		return 0, fmt.Errorf("reader: startxref not found in last %d bytes", searchLen)
	}

	// Parse the offset number after "startxref".
	after := string(tail[idx+len(marker):])
	after = strings.TrimSpace(after)
	// Take the first line (the offset).
	if nl := strings.IndexAny(after, "\r\n"); nl > 0 {
		after = after[:nl]
	}
	after = strings.TrimSpace(after)

	offset, err := strconv.ParseInt(after, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("reader: invalid startxref offset %q: %w", after, err)
	}

	return offset, nil
}

// parseXrefTable reads the classic xref table and trailer dictionary.
// Handles multiple xref sections (from incremental updates) by following /Prev.
func parseXrefTable(data []byte) (*xrefTable, error) {
	startOffset, err := findStartXref(data)
	if err != nil {
		return nil, err
	}

	table := &xrefTable{
		entries: make(map[int]xrefEntry),
	}

	// Follow the chain of xref sections (linked by /Prev in trailer).
	// Track visited offsets to detect circular /Prev references.
	visited := map[int64]bool{}
	offset := startOffset
	for offset >= 0 {
		if visited[offset] {
			break
		}
		visited[offset] = true
		tok := NewTokenizer(data)
		tok.SetPos(int(offset))

		// Check if this is a classic xref or an xref stream.
		firstTok := tok.Peek()
		if firstTok.Type == TokenKeyword && firstTok.Value == "xref" {
			trailer, prevOffset, err := parseOneXrefSection(tok, table, data)
			if err != nil {
				return nil, err
			}
			if table.trailer == nil {
				table.trailer = trailer
			}
			offset = prevOffset
		} else if firstTok.Type == TokenNumber {
			// Xref stream: an indirect object whose stream contains the xref data.
			// The object's dictionary serves as the trailer.
			trailer, prevOffset, err := parseXrefStream(data, int(offset), table)
			if err != nil {
				return nil, err
			}
			if table.trailer == nil {
				table.trailer = trailer
			}
			offset = prevOffset
		} else {
			break
		}
	}

	if table.trailer == nil {
		return nil, fmt.Errorf("reader: no trailer dictionary found")
	}

	return table, nil
}

// parseXrefStream reads an xref stream object at the given offset.
// Xref streams (PDF 1.5+) store the cross-reference table as compressed
// binary data inside a stream object. The stream's dictionary also
// serves as the trailer dictionary.
//
// Stream format (ISO 32000 §7.5.8):
//   - /Type /XRef
//   - /Size — total number of objects
//   - /W [w1 w2 w3] — byte widths for each field
//   - /Index [start count ...] — subsection ranges (optional, default [0 Size])
//   - Stream data: binary entries, each w1+w2+w3 bytes
//
// Entry types (field 1):
//   - 0: free object (field 2 = next free obj, field 3 = generation)
//   - 1: in-use object (field 2 = byte offset, field 3 = generation)
//   - 2: compressed object in object stream (field 2 = obj stream number, field 3 = index)
func parseXrefStream(data []byte, offset int, table *xrefTable) (*core.PdfDictionary, int64, error) {
	tok := NewTokenizer(data)
	tok.SetPos(offset)
	parser := NewParser(tok)

	_, _, obj, err := parser.ParseIndirectObject()
	if err != nil {
		return nil, -1, fmt.Errorf("reader: xref stream at offset %d: %w", offset, err)
	}

	stream, ok := obj.(*core.PdfStream)
	if !ok {
		return nil, -1, fmt.Errorf("reader: xref stream at offset %d is not a stream", offset)
	}

	dict := stream.Dict

	// Verify /Type /XRef.
	if typeObj := dict.Get("Type"); typeObj != nil {
		if name, ok := typeObj.(*core.PdfName); ok && name.Value != "XRef" {
			return nil, -1, fmt.Errorf("reader: xref stream has wrong /Type: %s", name.Value)
		}
	}

	// Get /W (field widths).
	wObj := dict.Get("W")
	if wObj == nil {
		return nil, -1, fmt.Errorf("reader: xref stream missing /W")
	}
	wArr, ok := wObj.(*core.PdfArray)
	if !ok || wArr.Len() != 3 {
		return nil, -1, fmt.Errorf("reader: xref stream /W must be array of 3 integers")
	}
	w := [3]int{
		pdfIntValue(wArr.At(0)),
		pdfIntValue(wArr.At(1)),
		pdfIntValue(wArr.At(2)),
	}
	entrySize := w[0] + w[1] + w[2]
	if entrySize == 0 {
		return nil, -1, fmt.Errorf("reader: xref stream entry size is 0")
	}

	// Get /Size.
	sizeVal := 0
	if sizeObj := dict.Get("Size"); sizeObj != nil {
		sizeVal = pdfIntValue(sizeObj)
	}

	// Get /Index (subsection ranges). Default: [0 Size].
	var subsections [][2]int
	if indexObj := dict.Get("Index"); indexObj != nil {
		if indexArr, ok := indexObj.(*core.PdfArray); ok {
			for i := 0; i+1 < indexArr.Len(); i += 2 {
				start := pdfIntValue(indexArr.At(i))
				count := pdfIntValue(indexArr.At(i + 1))
				subsections = append(subsections, [2]int{start, count})
			}
		}
	}
	if len(subsections) == 0 {
		subsections = [][2]int{{0, sizeVal}}
	}

	// Decompress stream data.
	// The stream was parsed by ParseIndirectObject which reads raw data.
	// We need to decompress it ourselves since the resolver isn't available yet.
	streamData, err := decompressXrefStream(data, offset, w, dict)
	if err != nil {
		return nil, -1, err
	}

	// Parse entries.
	pos := 0
	for _, sub := range subsections {
		startObj := sub[0]
		count := sub[1]
		for i := range count {
			if pos+entrySize > len(streamData) {
				break
			}
			field1 := readXrefField(streamData[pos:], w[0])
			field2 := readXrefField(streamData[pos+w[0]:], w[1])
			field3 := readXrefField(streamData[pos+w[0]+w[1]:], w[2])
			pos += entrySize

			objNum := startObj + i

			// Default type is 1 (in-use) when w[0] is 0.
			entryType := field1
			if w[0] == 0 {
				entryType = 1
			}

			switch entryType {
			case 0:
				// Free object.
				if _, exists := table.entries[objNum]; !exists {
					table.entries[objNum] = xrefEntry{
						offset:     int64(field2),
						generation: int(field3),
						inUse:      false,
					}
				}
			case 1:
				// In-use, uncompressed object.
				if _, exists := table.entries[objNum]; !exists {
					table.entries[objNum] = xrefEntry{
						offset:     int64(field2),
						generation: int(field3),
						inUse:      true,
					}
				}
			case 2:
				// Compressed object in object stream.
				// field2 = object stream number, field3 = index within stream.
				if _, exists := table.entries[objNum]; !exists {
					table.entries[objNum] = xrefEntry{
						offset:     int64(field2),
						generation: int(field3),
						inUse:      true,
						compressed: true,
					}
				}
			}
		}
	}

	// Check for /Prev.
	prevOffset := int64(-1)
	if prev := dict.Get("Prev"); prev != nil {
		prevOffset = int64(pdfIntValue(prev))
	}

	return dict, prevOffset, nil
}

// decompressXrefStream reads and decompresses the stream data from an xref stream
// object at the given file offset. This is needed before the resolver is
// available (since the resolver needs the xref to function).
func decompressXrefStream(data []byte, objOffset int, w [3]int, dict *core.PdfDictionary) ([]byte, error) {
	// Find "stream" keyword after the dictionary.
	tok := NewTokenizer(data)
	tok.SetPos(objOffset)

	for tok.pos < tok.len-6 {
		if string(tok.data[tok.pos:tok.pos+6]) == "stream" {
			tok.pos += 6
			if tok.pos < tok.len && tok.data[tok.pos] == '\r' {
				tok.pos++
			}
			if tok.pos < tok.len && tok.data[tok.pos] == '\n' {
				tok.pos++
			}
			break
		}
		tok.pos++
	}

	// Read /Length bytes.
	streamLen := 0
	if lengthObj := dict.Get("Length"); lengthObj != nil {
		streamLen = pdfIntValue(lengthObj)
	}

	if streamLen <= 0 || tok.pos+streamLen > tok.len {
		return nil, fmt.Errorf("reader: xref stream has invalid /Length %d", streamLen)
	}

	rawData := data[tok.pos : tok.pos+streamLen]

	// Decompress with xref-specific limit (default 32 MB).
	return decompressStreamWithLimit(rawData, dict, defaultMaxXrefSize)
}

// readXrefField reads a big-endian integer of the given byte width.
func readXrefField(data []byte, width int) int {
	if width == 0 {
		return 0
	}
	val := 0
	for i := range width {
		if i < len(data) {
			val = val<<8 | int(data[i])
		}
	}
	return val
}

// pdfIntValue extracts an integer from a PdfObject.
func pdfIntValue(obj core.PdfObject) int {
	if num, ok := obj.(*core.PdfNumber); ok {
		return num.IntValue()
	}
	return 0
}

// parseOneXrefSection reads one xref section and its trailer.
// Returns the trailer dict and the /Prev offset (-1 if none).
// data is the full file content, needed for hybrid xref support (/XRefStm).
func parseOneXrefSection(tok *Tokenizer, table *xrefTable, data []byte) (*core.PdfDictionary, int64, error) {
	// Skip "xref" keyword.
	line := tok.ReadLine()
	if strings.TrimSpace(line) != "xref" {
		return nil, -1, fmt.Errorf("reader: expected 'xref', got %q", line)
	}

	// Read subsections until we hit "trailer".
	for {
		tok.skipWhitespaceAndComments()
		if tok.pos >= tok.len {
			break
		}

		// Peek: is the next token "trailer"?
		peekPos := tok.pos
		peekTok := tok.Next()
		if peekTok.Type == TokenKeyword && peekTok.Value == "trailer" {
			break
		}
		// Rewind — we need to read the subsection header as a line.
		tok.pos = peekPos

		line := tok.ReadLine()
		trimmed := strings.TrimSpace(line)

		// Skip blank lines and comments.
		if trimmed == "" || strings.HasPrefix(trimmed, "%") {
			continue
		}

		parts := strings.Fields(trimmed)
		if len(parts) != 2 {
			return nil, -1, fmt.Errorf("reader: invalid xref subsection header %q", trimmed)
		}

		startObj, err := strconv.Atoi(parts[0])
		if err != nil {
			return nil, -1, fmt.Errorf("reader: invalid xref start object %q", parts[0])
		}
		if startObj < 0 {
			return nil, -1, fmt.Errorf("reader: negative xref start object %d", startObj)
		}
		count, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, -1, fmt.Errorf("reader: invalid xref count %q", parts[1])
		}
		if count < 0 {
			return nil, -1, fmt.Errorf("reader: negative xref count %d", count)
		}

		// Read entries.
		for i := range count {
			entryLine := tok.ReadLine()
			entry, err := parseXrefEntry(entryLine)
			if err != nil {
				return nil, -1, fmt.Errorf("reader: xref entry %d: %w", startObj+i, err)
			}
			objNum := startObj + i
			if _, exists := table.entries[objNum]; !exists {
				table.entries[objNum] = entry
			}
		}
	}

	// Parse trailer dictionary.
	parser := NewParser(tok)
	trailerObj, err := parser.ParseObject()
	if err != nil {
		return nil, -1, fmt.Errorf("reader: trailer: %w", err)
	}
	trailer, ok := trailerObj.(*core.PdfDictionary)
	if !ok {
		return nil, -1, fmt.Errorf("reader: trailer is not a dictionary")
	}

	// Hybrid xref: if /XRefStm is present, merge entries from the xref stream.
	// In a hybrid-reference file (ISO 32000 §7.5.8.4), a classic xref section
	// includes an /XRefStm entry pointing to an xref stream that contains
	// entries for objects stored in object streams. The classic table entries
	// take precedence (the existing "if !exists" guard in parseXrefStream
	// prevents stream entries from overwriting table entries already present).
	if xrefStm := trailer.Get("XRefStm"); xrefStm != nil {
		if num, ok := xrefStm.(*core.PdfNumber); ok {
			stmOffset := int64(num.IntValue())
			if stmOffset >= 0 && int(stmOffset) < len(data) {
				// Errors from the supplemental stream are non-fatal; the
				// classic table entries are sufficient for non-compressed objects.
				_, _, _ = parseXrefStream(data, int(stmOffset), table)
			}
		}
	}

	// Check for /Prev (previous xref section offset).
	prevOffset := int64(-1)
	if prev := trailer.Get("Prev"); prev != nil {
		if num, ok := prev.(*core.PdfNumber); ok {
			prevOffset = int64(num.IntValue())
		}
	}

	return trailer, prevOffset, nil
}

// parseXrefEntry parses one line of the xref table.
// Format: "0000000009 00000 n \n" (20 bytes: 10-digit offset, space,
// 5-digit generation, space, 'n' or 'f', space, EOL).
func parseXrefEntry(line string) (xrefEntry, error) {
	line = strings.TrimRight(line, "\r\n ")
	if len(line) < 18 {
		return xrefEntry{}, fmt.Errorf("xref entry too short (%d chars): %q", len(line), line)
	}

	// Bounds are guaranteed by the len(line) >= 18 check above, but we
	// validate explicitly to guard against future changes and make the
	// invariant clear to readers.
	if len(line) < 10 {
		return xrefEntry{}, fmt.Errorf("xref entry too short for offset field: %q", line)
	}
	offsetStr := strings.TrimSpace(line[0:10])
	if len(line) < 16 {
		return xrefEntry{}, fmt.Errorf("xref entry too short for generation field: %q", line)
	}
	genStr := strings.TrimSpace(line[11:16])
	if len(line) < 18 {
		return xrefEntry{}, fmt.Errorf("xref entry too short for type field: %q", line)
	}
	typeChar := line[17]

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil {
		return xrefEntry{}, fmt.Errorf("invalid offset %q", offsetStr)
	}
	if offset < 0 {
		return xrefEntry{}, fmt.Errorf("negative offset %d", offset)
	}
	gen, err := strconv.Atoi(genStr)
	if err != nil {
		return xrefEntry{}, fmt.Errorf("invalid generation %q", genStr)
	}
	if gen < 0 {
		return xrefEntry{}, fmt.Errorf("negative generation %d", gen)
	}

	return xrefEntry{
		offset:     offset,
		generation: gen,
		inUse:      typeChar == 'n',
	}, nil
}
