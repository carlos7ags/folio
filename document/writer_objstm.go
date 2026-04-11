// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"fmt"

	"github.com/carlos7ags/folio/core"
)

// defaultObjectStreamCapacity is the cap on the number of objects packed
// into a single /ObjStm when WriteOptions.ObjectStreamCapacity is zero.
//
// 100 is a middle-ground default chosen so that:
//
//   - decode cost stays bounded — a reader resolving any single object
//     pays for at most 100 object bodies, not the whole file;
//   - flate compression is amortized — at one or two hundred bytes per
//     compressed object body, a 100-entry stream comfortably exceeds the
//     ~150 byte stream-dictionary overhead;
//   - the reader-side /N heuristic in resolver.go (which caps N at
//     streamData/2) is satisfied by a wide margin even on small payloads.
//
// Callers can override via WriteOptions.ObjectStreamCapacity.
const defaultObjectStreamCapacity = 100

// packedObjStm bundles an /ObjStm indirect object with the placement
// records for the compressed entries it contains. The placement records
// drive the type-2 xref entries emitted by writeXRefStreamWithObjStms.
type packedObjStm struct {
	objNum     int
	stream     *core.PdfStream
	placements []core.ObjStmPlacement
}

// writeXRefStreamWithObjStms is the writer path for
// WriteOptions{UseXRefStream: true, UseObjectStreams: true}. It is the
// only place that mixes type-1 and type-2 xref entries; the xref-stream-
// only path lives in writeXRefStreamTrailer.
//
// Algorithm:
//
//  1. Partition the user objects into "eligible for compression" and
//     "must stay inline" using objStmEligible.
//  2. Greedy-pack the eligible objects into one or more /ObjStm objects
//     of capacity opts.ObjectStreamCapacity (default 100). Each /ObjStm
//     receives a fresh object number appended after the original user
//     objects, so original numbers are preserved.
//  3. Write header + binary comment.
//  4. Write each ineligible user object inline, in original order,
//     recording byte offsets for the type-1 xref entries.
//  5. Write each /ObjStm as a normal indirect object, recording byte
//     offsets.
//  6. Build the cross-reference stream entry array: type-2 for compressed
//     objects, type-1 for everything else (including the /ObjStm objects
//     and the xref stream's own entry).
//  7. Build the xref stream via BuildXRefStream and append it as the
//     final indirect object, then write startxref + EOF.
//
// Phase 1 refuses encryption: §7.5.7 forbids per-object encryption of
// objects inside an /ObjStm, and the safe interaction with the standard
// security handler is large enough to defer.
func (w *Writer) writeXRefStreamWithObjStms(cw *countingWriter, opts WriteOptions) error {
	if w.encryptor != nil {
		return fmt.Errorf("writer: object streams are not supported with encryption in phase 1")
	}

	cap := opts.ObjectStreamCapacity
	if cap <= 0 {
		cap = defaultObjectStreamCapacity
	}

	eligibleIdx := make([]int, 0, len(w.objects))
	for i := range w.objects {
		if w.objStmEligible(w.objects[i]) {
			eligibleIdx = append(eligibleIdx, i)
		}
	}

	// Greedy-pack into object streams. Object numbers for the streams
	// themselves come after the original user objects so the originals
	// keep their numbers.
	nextObjNum := len(w.objects) + 1
	var objstms []packedObjStm
	for start := 0; start < len(eligibleIdx); start += cap {
		end := start + cap
		if end > len(eligibleIdx) {
			end = len(eligibleIdx)
		}
		chunk := eligibleIdx[start:end]

		entries := make([]core.ObjStmEntry, len(chunk))
		for i, idx := range chunk {
			entries[i] = core.ObjStmEntry{
				ObjectNumber: w.objects[idx].ObjectNumber,
				Object:       w.objects[idx].Object,
			}
		}
		stream, err := core.BuildObjStm(entries)
		if err != nil {
			return fmt.Errorf("build object stream: %w", err)
		}

		thisObjStmNum := nextObjNum
		nextObjNum++

		placements := make([]core.ObjStmPlacement, len(chunk))
		for i, idx := range chunk {
			placements[i] = core.ObjStmPlacement{
				ObjectNumber:  w.objects[idx].ObjectNumber,
				ObjStmObjNum:  thisObjStmNum,
				IndexInObjStm: i,
			}
		}
		objstms = append(objstms, packedObjStm{
			objNum:     thisObjStmNum,
			stream:     stream,
			placements: placements,
		})
	}
	xrefStreamObjNum := nextObjNum

	// Header.
	if err := writeHeader(cw, w.version); err != nil {
		return err
	}

	// Build a quick lookup for compressed object numbers so we can skip
	// them in the inline-write loop.
	compressed := make(map[int]struct{}, len(eligibleIdx))
	for _, idx := range eligibleIdx {
		compressed[w.objects[idx].ObjectNumber] = struct{}{}
	}

	// Write ineligible objects inline, recording offsets.
	inlineOffsets := make(map[int]int64, len(w.objects)-len(eligibleIdx))
	for _, obj := range w.objects {
		if _, isCompressed := compressed[obj.ObjectNumber]; isCompressed {
			continue
		}
		inlineOffsets[obj.ObjectNumber] = cw.n
		if _, err := fmt.Fprintf(cw, "%d %d obj\n", obj.ObjectNumber, obj.GenerationNumber); err != nil {
			return err
		}
		if _, err := obj.Object.WriteTo(cw); err != nil {
			return err
		}
		if _, err := fmt.Fprint(cw, "\nendobj\n"); err != nil {
			return err
		}
	}

	// Write the /ObjStm objects, recording offsets.
	objstmOffsets := make(map[int]int64, len(objstms))
	for _, os := range objstms {
		objstmOffsets[os.objNum] = cw.n
		if _, err := fmt.Fprintf(cw, "%d 0 obj\n", os.objNum); err != nil {
			return err
		}
		if _, err := os.stream.WriteTo(cw); err != nil {
			return err
		}
		if _, err := fmt.Fprint(cw, "\nendobj\n"); err != nil {
			return err
		}
	}

	// Now we know all offsets. Build the xref entry array.
	xrefStreamOffset := cw.n
	size := xrefStreamObjNum + 1
	entries := make([]core.XRefStreamEntry, size)
	entries[0] = core.XRefStreamEntry{Type: core.XRefEntryFree, Field2: 0, Field3: 65535}

	placementByObj := make(map[int]core.ObjStmPlacement, len(eligibleIdx))
	for _, os := range objstms {
		for _, p := range os.placements {
			placementByObj[p.ObjectNumber] = p
		}
	}

	for i := 1; i <= len(w.objects); i++ {
		if p, ok := placementByObj[i]; ok {
			entries[i] = core.XRefStreamEntry{
				Type:   core.XRefEntryCompressed,
				Field2: uint64(p.ObjStmObjNum),
				Field3: uint64(p.IndexInObjStm),
			}
			continue
		}
		off, ok := inlineOffsets[i]
		if !ok {
			return fmt.Errorf("writer: object %d is neither inline nor compressed", i)
		}
		entries[i] = core.XRefStreamEntry{
			Type:   core.XRefEntryInUse,
			Field2: uint64(off),
		}
	}
	for _, os := range objstms {
		entries[os.objNum] = core.XRefStreamEntry{
			Type:   core.XRefEntryInUse,
			Field2: uint64(objstmOffsets[os.objNum]),
		}
	}
	entries[xrefStreamObjNum] = core.XRefStreamEntry{
		Type:   core.XRefEntryInUse,
		Field2: uint64(xrefStreamOffset),
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

// objStmEligible reports whether an indirect object can be packed into
// an /ObjStm. ISO 32000-1 §7.5.7 forbids:
//
//   - stream objects (a stream cannot live inside another stream);
//   - objects with a generation number other than zero (the type-2 xref
//     entry has no generation field);
//   - the document encryption dictionary;
//   - any object whose value is the /Length of a stream (the parser
//     needs /Length before it can decompress the surrounding stream).
//
// Phase 1 adds two conservative restrictions on top of the spec:
//
//   - the document catalog (/Root) is kept inline;
//   - the document information dictionary (/Info) is kept inline.
//
// Both could legally be compressed in an unencrypted document, but
// keeping them inline removes a class of edge cases around legacy
// readers and hybrid xref handling. The restriction can be relaxed in
// a later commit once the rest of the optimizer is in place and we
// have broader reader-compatibility coverage.
//
// The /Length-of-a-stream rule is enforced implicitly: folio always
// inlines /Length as a direct integer (core/stream.go), so no indirect
// object ever serves as a /Length value. If that ever changes, the
// invariant test in this file will catch the regression.
func (w *Writer) objStmEligible(obj IndirectObject) bool {
	if obj.GenerationNumber != 0 {
		return false
	}
	if obj.Object == nil {
		return false
	}
	if obj.Object.Type() == core.ObjectTypeStream {
		return false
	}
	if w.encryptRef != nil && obj.ObjectNumber == w.encryptRef.Num() {
		return false
	}
	if w.root != nil && obj.ObjectNumber == w.root.Num() {
		return false
	}
	if w.info != nil && obj.ObjectNumber == w.info.Num() {
		return false
	}
	return true
}
