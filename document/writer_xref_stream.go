// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"fmt"
	"io"

	"github.com/carlos7ags/folio/core"
)

// WriteOptions controls optional behavior of the PDF writer. The zero
// value reproduces the historical default: a traditional cross-reference
// table (ISO 32000-1 §7.5.4) and a separate trailer dictionary
// (§7.5.5), with no object-stream packing.
//
// Future fields will be added behind backward-compatible defaults; this
// struct is the single extension point for writer behavior.
type WriteOptions struct {
	// UseXRefStream replaces the traditional xref table and trailer with
	// a cross-reference stream object (ISO 32000-1 §7.5.8). The stream
	// dictionary carries the same /Root, /Info, /Encrypt, and /ID fields
	// the trailer would have, with /Type /XRef and Flate-compressed
	// entries. PDF readers from PDF 1.5 onward support this format.
	UseXRefStream bool

	// UseObjectStreams packs eligible indirect objects into compressed
	// object streams (ISO 32000-1 §7.5.7). It implies UseXRefStream
	// because compressed-object xref entries (type 2) require an xref
	// stream to express. Phase 1 of the optimizer does not implement
	// this option; setting it returns an error from WriteToWithOptions.
	UseObjectStreams bool

	// ObjectStreamCapacity caps the number of objects packed into a
	// single /ObjStm. Zero means "use the writer default". Ignored
	// unless UseObjectStreams is set.
	ObjectStreamCapacity int
}

// WriteToWithOptions is the option-aware variant of WriteTo. WriteTo is
// kept as a thin wrapper that calls this function with a zero-value
// options struct, so existing callers continue to receive the historical
// default output.
func (w *Writer) WriteToWithOptions(out io.Writer, opts WriteOptions) (int64, error) {
	if opts.UseObjectStreams && !opts.UseXRefStream {
		// §7.5.8.3: type-2 xref entries (compressed objects) require an
		// xref stream to express. Refuse the contradictory combination
		// instead of silently upgrading.
		return 0, fmt.Errorf("writer: UseObjectStreams requires UseXRefStream")
	}

	// Encrypt all user objects in place. Done before serialization so
	// the offsets we record reflect the encrypted bytes. Matches the
	// historical writer behavior.
	if w.encryptor != nil {
		for _, obj := range w.objects {
			if err := w.encryptor.EncryptObject(obj.Object, obj.ObjectNumber, obj.GenerationNumber); err != nil {
				return 0, fmt.Errorf("encrypt object %d: %w", obj.ObjectNumber, err)
			}
		}
	}

	cw := &countingWriter{w: out}

	if opts.UseObjectStreams {
		return cw.n, w.writeXRefStreamWithObjStms(cw, opts)
	}

	if err := writeHeader(cw, w.version); err != nil {
		return cw.n, err
	}

	offsets, err := w.writeObjectBodies(cw)
	if err != nil {
		return cw.n, err
	}

	if opts.UseXRefStream {
		return cw.n, w.writeXRefStreamTrailer(cw, offsets)
	}
	return cw.n, w.writeTraditionalTrailer(cw, offsets)
}

// writeHeader emits the PDF version header and the four-byte binary
// comment that signals to file-type detectors that the file contains
// non-ASCII data (ISO 32000-1 §7.5.2).
func writeHeader(cw *countingWriter, version string) error {
	if _, err := fmt.Fprintf(cw, "%%PDF-%s\n", version); err != nil {
		return err
	}
	_, err := fmt.Fprintf(cw, "%%\xe2\xe3\xcf\xd3\n")
	return err
}

// writeObjectBodies serializes every registered indirect object and
// returns the byte offset where each object's "N G obj" header begins.
// offsets[i] corresponds to w.objects[i].
func (w *Writer) writeObjectBodies(cw *countingWriter) ([]int64, error) {
	offsets := make([]int64, len(w.objects))
	for i, obj := range w.objects {
		offsets[i] = cw.n
		if _, err := fmt.Fprintf(cw, "%d %d obj\n", obj.ObjectNumber, obj.GenerationNumber); err != nil {
			return nil, err
		}
		if _, err := obj.Object.WriteTo(cw); err != nil {
			return nil, err
		}
		if _, err := fmt.Fprint(cw, "\nendobj\n"); err != nil {
			return nil, err
		}
	}
	return offsets, nil
}

// writeTraditionalTrailer emits a §7.5.4 cross-reference table and a
// §7.5.5 trailer dictionary followed by startxref and EOF.
func (w *Writer) writeTraditionalTrailer(cw *countingWriter, offsets []int64) error {
	xrefOffset := cw.n
	if _, err := fmt.Fprint(cw, "xref\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(cw, "0 %d\n", len(w.objects)+1); err != nil {
		return err
	}
	if _, err := fmt.Fprint(cw, "0000000000 65535 f \n"); err != nil {
		return err
	}
	for _, offset := range offsets {
		if _, err := fmt.Fprintf(cw, "%010d 00000 n \n", offset); err != nil {
			return err
		}
	}

	trailer := w.buildTrailerDict()
	trailer.Set("Size", core.NewPdfInteger(len(w.objects)+1))
	if _, err := fmt.Fprint(cw, "trailer\n"); err != nil {
		return err
	}
	if _, err := trailer.WriteTo(cw); err != nil {
		return err
	}
	if _, err := fmt.Fprint(cw, "\n"); err != nil {
		return err
	}
	_, err := fmt.Fprintf(cw, "startxref\n%d\n%%%%EOF\n", xrefOffset)
	return err
}

// writeXRefStreamTrailer appends the cross-reference stream as a final
// indirect object, then writes startxref and EOF. The xref stream is
// always the last object in the file, so its own offset is known
// before any compression happens and the field-width calculation can
// observe the maximum offset directly — no chicken-and-egg.
func (w *Writer) writeXRefStreamTrailer(cw *countingWriter, offsets []int64) error {
	xrefStreamObjNum := len(w.objects) + 1
	xrefStreamOffset := cw.n
	size := xrefStreamObjNum + 1 // covers object numbers 0..xrefStreamObjNum

	entries := make([]core.XRefStreamEntry, size)
	entries[0] = core.XRefStreamEntry{Type: core.XRefEntryFree, Field2: 0, Field3: 65535}
	for i, off := range offsets {
		entries[i+1] = core.XRefStreamEntry{
			Type:   core.XRefEntryInUse,
			Field2: uint64(off),
			Field3: 0,
		}
	}
	entries[xrefStreamObjNum] = core.XRefStreamEntry{
		Type:   core.XRefEntryInUse,
		Field2: uint64(xrefStreamOffset),
		Field3: 0,
	}

	extras := w.buildTrailerDict()
	subsections := []core.XRefStreamSubsection{{First: 0, Entries: entries}}
	stream, err := core.BuildXRefStream(subsections, size, extras)
	if err != nil {
		return fmt.Errorf("build xref stream: %w", err)
	}

	if _, err := fmt.Fprintf(cw, "%d 0 obj\n", xrefStreamObjNum); err != nil {
		return err
	}
	if _, err := stream.WriteTo(cw); err != nil {
		return err
	}
	if _, err := fmt.Fprint(cw, "\nendobj\n"); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cw, "startxref\n%d\n%%%%EOF\n", xrefStreamOffset)
	return err
}

// buildTrailerDict assembles /Root, /Info, /Encrypt, and /ID. /Size is
// set by the caller because the traditional and xref-stream paths use
// different values (the xref stream introduces one extra object).
func (w *Writer) buildTrailerDict() *core.PdfDictionary {
	d := core.NewPdfDictionary()
	if w.root != nil {
		d.Set("Root", w.root)
	}
	if w.info != nil {
		d.Set("Info", w.info)
	}
	if w.encryptor != nil {
		d.Set("Encrypt", w.encryptRef)
		id := core.NewPdfHexString(string(w.encryptor.FileID))
		d.Set("ID", core.NewPdfArray(id, id))
	} else if len(w.fileID) > 0 {
		id := core.NewPdfHexString(string(w.fileID))
		d.Set("ID", core.NewPdfArray(id, id))
	}
	return d
}
