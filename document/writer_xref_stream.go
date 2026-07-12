// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"crypto/md5"
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

	// ObjectStreamCapacity caps the number of objects packed into a
	// single /ObjStm. Zero means "use the writer default". Ignored
	// unless UseObjectStreams is set.
	ObjectStreamCapacity int

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

	// OrphanSweep removes indirect objects unreachable from the
	// document roots before serialization. The reachable set is
	// computed by walking PdfIndirectReference values out of /Root,
	// /Info, and /Encrypt (ISO 32000-1 §7.5.4 requires every "in use"
	// xref entry to point at an object the document actually consumes;
	// §7.7.2 names the document catalog as the reachability entry
	// point). Surviving objects are renumbered contiguously starting
	// at 1. The sweep is lossless: it never alters object content, only
	// drops orphans and rewrites object numbers.
	//
	// Currently refused on encrypted documents because the per-object
	// encryption key derives from the object number (§7.6.3.3); a safe
	// interaction with the standard security handler has not yet been
	// implemented.
	OrphanSweep bool

	// RecompressStreams re-Flate-compresses eligible stream payloads
	// at zlib.BestCompression and commits the rewrite only when it
	// produces a strictly smaller payload (size-regression guarded).
	// Eligibility (per ISO 32000-1 §7.4):
	//
	//   - Streams with no /Filter — payload is raw bytes; Flate is
	//     applied and /Filter /FlateDecode is set.
	//   - Streams whose /Filter is exactly /FlateDecode — payload is
	//     inflated, re-deflated at higher effort, and committed only
	//     on shrink. Useful when the original producer used a lower
	//     zlib level than BestCompression.
	//
	// Streams whose filter chain contains /DCTDecode (§7.4.8),
	// /JPXDecode (§7.4.9), /CCITTFaxDecode (§7.4.7), or /JBIG2Decode
	// (§7.4.8) are skipped — those payloads are already specialized-
	// compressed and Flate would inflate them. Multi-filter chains
	// other than the eligible cases above are also skipped. FlateDecode
	// streams that carry a /DecodeParms entry (most commonly a PNG or
	// TIFF predictor per §7.4.4.4) are skipped because the inflated
	// payload is predictor-filtered bytes, not plaintext, and a safe
	// re-deflate would require re-applying the predictor inversion.
	//
	// Currently refused on encrypted documents: rewriting payload
	// bytes after the encryption walk would emit ciphertext that
	// decrypts to the wrong plaintext, and rewriting before the
	// walk requires the encryption code to revisit each rewritten
	// stream — both deferred until needed.
	//
	// Lossless and safe to combine with UseXRefStream,
	// UseObjectStreams, and OrphanSweep.
	RecompressStreams bool

	// DeduplicateObjects merges byte-identical indirect objects so the
	// serialized output stores each unique payload exactly once. For
	// every group of indirect objects whose canonical serialization
	// (per ISO 32000-1 §7.3) produces the same bytes, the first slot
	// is retained and every PdfIndirectReference targeting any of the
	// other slots is rewritten to point at the survivor. Surviving
	// objects are renumbered contiguously.
	//
	// The pass is most valuable on documents that re-use resources:
	// imported pages sharing fonts or color spaces, repeated logos
	// embedded as separate XObjects, identical ToUnicode CMaps
	// (§9.10.3) across font dictionaries, and per-page content streams
	// produced by templated workflows.
	//
	// Catalog, /Info, and /Encrypt indirect objects are excluded from
	// dedup — they are reachability roots that the trailer slots refer
	// to by pointer, and merging them would require relocating the
	// trailer-side bookkeeping. They are unique in practice anyway.
	//
	// Currently refused on encrypted documents for the same reason as
	// OrphanSweep: per-object encryption keys derive from the object
	// number (§7.6.3.3) and renumbering after dedup would invalidate
	// every key.
	DeduplicateObjects bool

	// CleanContentStreams removes redundant operators from page content
	// streams (ISO 32000-1 §7.8): empty `q ... Q` graphics-state
	// save/restore pairs that contain only whitespace, and identity
	// `1 0 0 1 0 0 cm` matrix concatenations. The cleanup operates on
	// the raw byte payload of each stream referenced from a Page
	// dictionary's /Contents entry; the size-regression guard reverts
	// any rewrite that fails to shrink.
	//
	// Currently refused on encrypted documents because rewriting
	// payload bytes after the encryption walk would emit ciphertext
	// that decrypts to the wrong plaintext.
	CleanContentStreams bool

	// Deterministic makes the writer produce byte-identical output for
	// identical input documents. It derives the trailer /ID from a digest
	// of the serialized object set instead of random bytes — matching the
	// content-based file-identifier scheme ISO 32000-1 §14.4 describes —
	// and routes every metadata timestamp that would otherwise default to
	// the wall clock (XMP CreateDate/ModifyDate, embedded-file dates)
	// through Document.Info.CreationDate, falling back to the zero time
	// rather than time.Now when CreationDate is unset. Setting
	// Document.Info.CreationDate (and ModDate) is therefore recommended so
	// the recorded dates are meaningful rather than the year-zero default.
	//
	// An explicit Document.SetFileID overrides the derived /ID. A non-PDF/A
	// document that would otherwise carry no /ID gains one. Encrypted
	// documents are excluded: the standard security handler mints a random
	// file identifier and derives the encryption key from it (§7.6.3.3), so
	// their output is never byte-stable; the /ID is left to the handler.
	Deterministic bool
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
		return 0, fmt.Errorf("document: writer: UseObjectStreams requires UseXRefStream")
	}
	if opts.UseObjectStreams && w.encryptor != nil {
		// Refuse before the encryption walk runs. EncryptObject mutates
		// objects in place, so a deferred refusal would leave the writer
		// in a half-encrypted state and a retry without UseObjectStreams
		// would double-encrypt. Phase 1 does not support per-objstm
		// encryption (§7.6.1 requires the entire object stream to be
		// encrypted as a unit, not the individual entries).
		return 0, fmt.Errorf("document: writer: object streams are not supported with encryption in phase 1")
	}
	if opts.OrphanSweep && w.encryptor != nil {
		// Same defense as the objstm refusal above: refuse before the
		// encryption walk so a retry without OrphanSweep is not
		// double-encrypted. The standard security handler keys each
		// object on its number (§7.6.3.3); a sweep that renumbers
		// objects in an encrypted document would silently invalidate
		// the keys for every renumbered entry.
		return 0, fmt.Errorf("document: writer: orphan sweep is not supported on encrypted documents")
	}
	if opts.RecompressStreams && w.encryptor != nil {
		// Same defense as the sweep refusal above: rewriting payload
		// bytes interacts with the encryption walk in subtle ways
		// (encrypted ciphertext decrypts only against the bytes that
		// were encrypted). Refuse before any mutation so a retry
		// without RecompressStreams is unaffected.
		return 0, fmt.Errorf("document: writer: stream recompression is not supported on encrypted documents")
	}
	if opts.DeduplicateObjects && w.encryptor != nil {
		// Same defense: dedup renumbers survivors, which would
		// invalidate per-object encryption keys (§7.6.3.3).
		return 0, fmt.Errorf("document: writer: object deduplication is not supported on encrypted documents")
	}
	if opts.CleanContentStreams && w.encryptor != nil {
		// Same defense as RecompressStreams: rewriting payload bytes
		// interacts badly with the encryption walk.
		return 0, fmt.Errorf("document: writer: content stream cleanup is not supported on encrypted documents")
	}
	// Order: sweep → cleanup → dedup → recompress → encrypt → serialize.
	// - Sweep first so later passes do not waste effort on orphans.
	// - Cleanup before dedup so two streams that become byte-identical
	//   after redundant-operator removal can be merged.
	// - Dedup before recompress so each canonical payload is recompressed
	//   exactly once.
	// - Encryption walk runs last because it mutates payload bytes
	//   (ciphertext) that no other pass should touch.
	if opts.OrphanSweep {
		w.sweepOrphans()
	}
	if opts.CleanContentStreams {
		w.cleanContentStreams()
	}
	if opts.DeduplicateObjects {
		w.deduplicateObjects()
	}
	if opts.RecompressStreams {
		w.recompressStreams()
	}

	// Encrypt all user objects in place. Done before serialization so
	// the offsets we record reflect the encrypted bytes. Matches the
	// historical writer behavior.
	if w.encryptor != nil {
		for _, obj := range w.objects {
			if err := w.encryptor.EncryptObject(obj.Object, obj.ObjectNumber, obj.GenerationNumber); err != nil {
				return 0, fmt.Errorf("document: encrypt object %d: %w", obj.ObjectNumber, err)
			}
		}
	}

	// Deterministic /ID: when requested and no explicit identifier was set
	// (via SetFileID), derive a stable /ID from the final object set — after
	// the optimization passes above have settled object content and numbers
	// — so identical inputs yield byte-identical output. Encryption supplies
	// its own random /ID and key, so deterministic output is unavailable for
	// encrypted documents and the derivation is skipped there.
	if opts.Deterministic && len(w.fileID) == 0 && w.encryptor == nil {
		w.fileID = w.deterministicFileID()
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
		// Cancellation check at the object boundary during serialization.
		if w.ctx != nil {
			if err := w.ctx.Err(); err != nil {
				return nil, err
			}
		}
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
		return fmt.Errorf("document: build xref stream: %w", err)
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

// deterministicFileID derives a stable 16-byte file identifier by hashing
// the serialized form of every registered object in order. Because the
// object set, object numbers, and each object's serialization are fully
// determined by the document content — PdfDictionary preserves insertion
// order, so there is no map-iteration nondeterminism — identical input
// documents produce an identical identifier. MD5 yields exactly the 16
// bytes a /ID entry expects (ISO 32000-1 §14.4); it is used here only as a
// content digest, not for any security property.
func (w *Writer) deterministicFileID() []byte {
	h := md5.New()
	for _, obj := range w.objects {
		_, _ = fmt.Fprintf(h, "%d %d obj\n", obj.ObjectNumber, obj.GenerationNumber)
		_, _ = obj.Object.WriteTo(h)
		_, _ = io.WriteString(h, "\nendobj\n")
	}
	return h.Sum(nil)
}
