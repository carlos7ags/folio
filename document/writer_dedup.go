// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"crypto/sha256"

	"github.com/carlos7ags/folio/core"
)

// deduplicateObjects merges byte-identical indirect objects so the
// serialized output stores each unique payload exactly once.
//
// The pass works in three steps:
//
//  1. For every eligible indirect object, compute the SHA-256 of its
//     canonical serialization (the bytes that PdfObject.WriteTo would
//     emit for that object's body). Two objects whose serializations
//     match are equivalent under PDF semantics (§7.3) provided their
//     embedded references resolve to the same targets — and they do,
//     because we run after sweep/cleanup but before encryption, on a
//     graph that has not been mutated since the previous pass.
//
//  2. Group hashes. For each group with more than one member, the
//     first slot in original write order becomes the canonical
//     survivor and every other slot is marked for elimination. Build
//     a redirect map: duplicateObjNum → canonicalObjNum.
//
//  3. Rewrite every PdfIndirectReference inside every kept object's
//     value graph (and the trailer-slot roots) so duplicates point
//     at their canonical survivor. Drop the eliminated slots from
//     w.objects, then renumber survivors contiguously starting at 1.
//
// Eligibility: the catalog (w.root), the document info dictionary
// (w.info), and the encryption dictionary (w.encryptRef) are excluded.
// They are reachability roots; merging them would require relocating
// the trailer-side bookkeeping, and they are unique in practice.
//
// Encrypted documents are refused upstream in WriteToWithOptions
// because per-object keys derive from the object number (§7.6.3.3)
// and renumbering would invalidate them. The defensive guard at the
// top of this method matches the sweep contract so a future caller
// bypassing WriteToWithOptions cannot corrupt encrypted payloads.
//
// The pass never enlarges output, never changes object content, and
// never invalidates a reference. It composes safely with
// OrphanSweep, CleanContentStreams, RecompressStreams,
// UseXRefStream, and UseObjectStreams.
func (w *Writer) deduplicateObjects() {
	if len(w.objects) == 0 {
		return
	}
	if w.encryptor != nil {
		return
	}

	// Identify the trailer-root object numbers that must not be merged.
	skip := make(map[int]bool, 3)
	if w.root != nil {
		skip[w.root.Num()] = true
	}
	if w.info != nil {
		skip[w.info.Num()] = true
	}
	if w.encryptRef != nil {
		skip[w.encryptRef.Num()] = true
	}

	// Step 1: hash each eligible object's canonical bytes. Skipped
	// objects are still kept; their hashes are set to a sentinel that
	// cannot collide with any real hash so they never group with
	// anything else.
	//
	// Side effect: hashing a *PdfStream calls its WriteTo, which sets
	// /Length on the stream's dictionary (and /Filter when the stream
	// was constructed with NewPdfStreamCompressed). Both are inserted
	// at deterministic positions and re-running WriteTo produces
	// byte-identical output, so the hash is stable across calls and
	// the writer's later serialization sees the same dictionary state.
	type hashedSlot struct {
		hash string // hex SHA-256 of canonical bytes; "" means "do not dedup"
		idx  int    // index into w.objects
	}
	hashed := make([]hashedSlot, len(w.objects))
	var buf bytes.Buffer
	for i, obj := range w.objects {
		hashed[i].idx = i
		if skip[obj.ObjectNumber] {
			continue
		}
		buf.Reset()
		if _, err := obj.Object.WriteTo(&buf); err != nil {
			// Hashing failed; treat as un-dedupable. Better to keep
			// the object than to fail the whole write.
			continue
		}
		sum := sha256.Sum256(buf.Bytes())
		hashed[i].hash = string(sum[:])
	}

	// Step 2: group by hash; first slot wins.
	canonical := make(map[string]int) // hash → ObjectNumber of canonical survivor
	redirect := make(map[int]int)     // duplicate ObjectNumber → canonical ObjectNumber
	for _, h := range hashed {
		if h.hash == "" {
			continue
		}
		objNum := w.objects[h.idx].ObjectNumber
		if canon, ok := canonical[h.hash]; ok {
			redirect[objNum] = canon
		} else {
			canonical[h.hash] = objNum
		}
	}

	if len(redirect) == 0 {
		return
	}

	// Step 3a: rewrite references to merged duplicates. A reference to
	// a duplicate becomes a reference to its canonical survivor.
	rewriteToCanonical := func(ref *core.PdfIndirectReference) {
		if ref == nil {
			return
		}
		if canon, ok := redirect[ref.Num()]; ok {
			ref.SetNum(canon)
		}
	}
	for _, obj := range w.objects {
		walkReferences(obj.Object, rewriteToCanonical)
	}
	rewriteToCanonical(w.root)
	rewriteToCanonical(w.info)
	rewriteToCanonical(w.encryptRef)

	// Step 3b: drop the duplicates from w.objects.
	kept := make([]IndirectObject, 0, len(w.objects)-len(redirect))
	for _, obj := range w.objects {
		if _, isDup := redirect[obj.ObjectNumber]; isDup {
			continue
		}
		kept = append(kept, obj)
	}

	// Step 3c: renumber survivors contiguously. We could leave gaps
	// (xref free entries handle them per §7.5.4) but contiguous
	// numbering keeps the xref subsection compact and matches the
	// post-sweep convention.
	renumber := make(map[int]int, len(kept))
	for i := range kept {
		newNum := i + 1
		if kept[i].ObjectNumber != newNum {
			renumber[kept[i].ObjectNumber] = newNum
		}
	}
	if len(renumber) > 0 {
		rewriteForRenumber := func(ref *core.PdfIndirectReference) {
			if ref == nil {
				return
			}
			if newNum, ok := renumber[ref.Num()]; ok {
				ref.SetNum(newNum)
			}
		}
		for i := range kept {
			walkReferences(kept[i].Object, rewriteForRenumber)
			kept[i].ObjectNumber = i + 1
		}
		rewriteForRenumber(w.root)
		rewriteForRenumber(w.info)
		rewriteForRenumber(w.encryptRef)
	}

	w.objects = kept
}
