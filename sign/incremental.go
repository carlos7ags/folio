// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package sign

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/core"
)

// incrementalObject is an indirect object to append in an incremental update.
type incrementalObject struct {
	ObjectNumber     int
	GenerationNumber int
	Object           core.PdfObject
}

// incrementalWriter appends new objects to an existing PDF via an incremental update.
// It writes a new xref section and trailer with /Prev pointing to the original xref.
type incrementalWriter struct {
	original    []byte              // original PDF bytes
	objects     []incrementalObject // new or modified objects
	prevXref    int64               // byte offset of the original xref
	prevTrailer *core.PdfDictionary // original trailer dictionary
}

// newIncrementalWriter creates an incremental writer for the given PDF bytes.
func newIncrementalWriter(pdfBytes []byte, prevXref int64, prevTrailer *core.PdfDictionary) *incrementalWriter {
	return &incrementalWriter{
		original:    pdfBytes,
		prevXref:    prevXref,
		prevTrailer: prevTrailer,
	}
}

// addObject registers a new object to be appended.
func (w *incrementalWriter) addObject(objNum int, obj core.PdfObject) {
	w.objects = append(w.objects, incrementalObject{
		ObjectNumber:     objNum,
		GenerationNumber: 0,
		Object:           obj,
	})
}

// write produces the complete PDF: original bytes + appended objects + new xref + new trailer.
func (w *incrementalWriter) write() ([]byte, error) {
	var buf bytes.Buffer

	// Copy original PDF bytes.
	buf.Write(w.original)

	// Ensure we start on a new line.
	if len(w.original) > 0 && w.original[len(w.original)-1] != '\n' {
		buf.WriteByte('\n')
	}

	// Write each new object, tracking offsets.
	type offsetEntry struct {
		objNum int
		offset int64
	}
	offsets := make([]offsetEntry, 0, len(w.objects))

	for _, obj := range w.objects {
		offset := int64(buf.Len())
		offsets = append(offsets, offsetEntry{objNum: obj.ObjectNumber, offset: offset})

		fmt.Fprintf(&buf, "%d %d obj\n", obj.ObjectNumber, obj.GenerationNumber)
		if _, err := obj.Object.WriteTo(&buf); err != nil {
			return nil, fmt.Errorf("sign: write object %d: %w", obj.ObjectNumber, err)
		}
		fmt.Fprint(&buf, "\nendobj\n")
	}

	// Write xref section (only the new objects).
	xrefOffset := int64(buf.Len())
	fmt.Fprint(&buf, "xref\n")

	// Group contiguous object numbers into subsections.
	// For simplicity (and because signature updates are small), write one subsection per object.
	for _, entry := range offsets {
		fmt.Fprintf(&buf, "%d 1\n", entry.objNum)
		fmt.Fprintf(&buf, "%010d 00000 n \n", entry.offset)
	}

	// Build trailer.
	trailer := core.NewPdfDictionary()

	// Copy /Root and /Info from previous trailer.
	if root := w.prevTrailer.Get("Root"); root != nil {
		trailer.Set("Root", root)
	}
	if info := w.prevTrailer.Get("Info"); info != nil {
		trailer.Set("Info", info)
	}
	if id := w.prevTrailer.Get("ID"); id != nil {
		trailer.Set("ID", id)
	}

	// /Size must be max object number + 1 across the entire file.
	maxObjNum := prevTrailerSize(w.prevTrailer)
	for _, obj := range w.objects {
		if obj.ObjectNumber+1 > maxObjNum {
			maxObjNum = obj.ObjectNumber + 1
		}
	}
	trailer.Set("Size", core.NewPdfInteger(maxObjNum))
	trailer.Set("Prev", core.NewPdfInteger(int(w.prevXref)))

	fmt.Fprint(&buf, "trailer\n")
	if _, err := trailer.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("sign: write trailer: %w", err)
	}
	fmt.Fprintf(&buf, "\nstartxref\n%d\n%%%%EOF\n", xrefOffset)

	return buf.Bytes(), nil
}

// prevTrailerSize extracts the /Size integer from a trailer dictionary.
func prevTrailerSize(trailer *core.PdfDictionary) int {
	if trailer == nil {
		return 0
	}
	sizeObj := trailer.Get("Size")
	if sizeObj == nil {
		return 0
	}
	if num, ok := sizeObj.(*core.PdfNumber); ok {
		return num.IntValue()
	}
	return 0
}

// findStartXref scans backwards from EOF to find the startxref byte offset.
func findStartXref(data []byte) (int64, error) {
	searchLen := min(1024, len(data))
	tail := data[len(data)-searchLen:]

	marker := []byte("startxref")
	idx := bytes.LastIndex(tail, marker)
	if idx < 0 {
		return 0, fmt.Errorf("sign: startxref not found in last %d bytes", searchLen)
	}

	after := string(tail[idx+len(marker):])
	after = strings.TrimSpace(after)
	if nl := strings.IndexAny(after, "\r\n"); nl > 0 {
		after = after[:nl]
	}
	after = strings.TrimSpace(after)

	offset, err := strconv.ParseInt(after, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("sign: invalid startxref offset %q: %w", after, err)
	}
	return offset, nil
}
