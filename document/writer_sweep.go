// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"github.com/carlos7ags/folio/core"
)

// sweepOrphans drops any indirect object that is not reachable from the
// document roots and renumbers the survivors contiguously starting at 1.
//
// The reachability roots are the trailer entries: /Root, /Info, and
// /Encrypt (ISO 32000-1 §7.5.5). The walk follows every PdfIndirectReference
// nested inside a reachable object's value graph (dictionaries, arrays,
// stream dictionaries). Reaching an object number that does not correspond
// to a registered indirect object is treated as a dangling reference —
// allowed by §7.3.10 ("a reference to an unknown object is a reference
// to the null object") — and ignored.
//
// The sweep mutates Writer state in place:
//
//   - w.objects is replaced with the kept slice in original order, with
//     ObjectNumber rewritten to the new contiguous numbering.
//   - Every PdfIndirectReference inside a kept object's body is rewritten
//     so its ObjectNumber matches the new numbering.
//   - The root references themselves (w.root, w.info, w.encryptRef) are
//     rewritten in place. Because *PdfIndirectReference is shared by
//     pointer between the Writer's trailer slot and the embedding object's
//     value graph, a single rewrite updates both observers.
//
// The sweep is a no-op when every registered object is reachable. It is
// idempotent: calling it twice produces the same w.objects as calling it
// once. It is unconditionally lossless on size — the kept set is a subset
// of the input set and individual object bodies are not modified.
//
// Encrypted documents are refused upstream in WriteToWithOptions because
// the standard security handler keys each object on its number
// (§7.6.3.3); renumbering would invalidate every encryption key.
func (w *Writer) sweepOrphans() {
	if len(w.objects) == 0 {
		return
	}
	if w.encryptor != nil {
		// Defensive: WriteToWithOptions refuses OrphanSweep on
		// encrypted documents because per-object keys derive from the
		// object number (§7.6.3.3). A future caller that reaches this
		// method without the WriteOptions guard would silently
		// invalidate every encryption key. Returning here matches the
		// "lossless and safe" contract on the doc-comment above.
		return
	}

	// Reachability walk. Build an objNum → object body lookup so the
	// queue can dereference targets in O(1) without scanning w.objects.
	// Indexed by object number; the lookup is only consulted to follow
	// references, so collisions on duplicate numbers (a writer bug) are
	// detected as a refusal-to-renumber below rather than silent loss.
	byNum := make(map[int]core.PdfObject, len(w.objects))
	for _, obj := range w.objects {
		if _, dup := byNum[obj.ObjectNumber]; dup {
			// Two slots claim the same object number. The renumber map
			// is keyed by old number, so we cannot disambiguate which
			// of the two collisions a reference targeted. Refuse the
			// sweep rather than silently corrupt object identity.
			return
		}
		byNum[obj.ObjectNumber] = obj.Object
	}

	reachable := make(map[int]bool, len(w.objects))
	queue := make([]*core.PdfIndirectReference, 0, len(w.objects))

	enqueue := func(ref *core.PdfIndirectReference) {
		if ref == nil {
			return
		}
		if reachable[ref.Num()] {
			return
		}
		reachable[ref.Num()] = true
		queue = append(queue, ref)
	}

	enqueue(w.root)
	enqueue(w.info)
	enqueue(w.encryptRef)

	for len(queue) > 0 {
		ref := queue[0]
		queue = queue[1:]
		body, ok := byNum[ref.Num()]
		if !ok {
			// Dangling root reference; nothing to descend into.
			continue
		}
		walkReferences(body, enqueue)
	}

	if len(reachable) == len(w.objects) {
		// Every object is live — nothing to drop and nothing to renumber.
		return
	}

	// Build the kept slice and the renumber map in one pass, preserving
	// original write order so previously-asserted layout (e.g., catalog
	// before pages) is not disturbed beyond the renumbering.
	kept := make([]IndirectObject, 0, len(reachable))
	renumber := make(map[int]int, len(reachable))
	for _, obj := range w.objects {
		if !reachable[obj.ObjectNumber] {
			continue
		}
		newNum := len(kept) + 1
		renumber[obj.ObjectNumber] = newNum
		kept = append(kept, IndirectObject{
			ObjectNumber:     newNum,
			GenerationNumber: obj.GenerationNumber,
			Object:           obj.Object,
		})
	}

	// Rewrite every reference inside the surviving bodies. This also
	// visits trailer-root references that happen to be embedded inside
	// a kept object's value graph (e.g., catalog → /Pages), so the
	// trailer-slot rewrites below are belt-and-braces for refs that
	// were never embedded.
	// A reference pointer can be shared by several bodies (reader.Copier
	// hands out one per source object); renumber each pointer once.
	rewritten := make(map[*core.PdfIndirectReference]bool)
	rewrite := func(ref *core.PdfIndirectReference) {
		if ref == nil || rewritten[ref] {
			return
		}
		rewritten[ref] = true
		if newNum, ok := renumber[ref.Num()]; ok {
			ref.SetNum(newNum)
		}
		// Refs whose target is missing from the renumber map point at
		// objects we did not reach. Per §7.3.10 they resolve to null;
		// leaving the stale number lets the existing serialization
		// path emit it as a "N 0 R" that any conformant reader will
		// treat as the null object.
	}
	for _, obj := range kept {
		walkReferences(obj.Object, rewrite)
	}
	rewrite(w.root)
	rewrite(w.info)
	rewrite(w.encryptRef)

	w.objects = kept
}

// walkReferences invokes visit on every *PdfIndirectReference reachable
// from obj without descending into the referenced indirect object — a
// reference is yielded but not followed. The walk descends through
// PdfDictionary, PdfArray, and PdfStream (via the stream's dictionary).
// Other PdfObject kinds carry no references and are leaves.
//
// The walk is finite because direct PDF objects form a tree (an indirect
// reference is the only edge that can produce a cycle, and we never
// follow one).
func walkReferences(obj core.PdfObject, visit func(*core.PdfIndirectReference)) {
	switch v := obj.(type) {
	case *core.PdfIndirectReference:
		visit(v)
	case *core.PdfDictionary:
		for _, value := range v.All() {
			walkReferences(value, visit)
		}
	case *core.PdfArray:
		for _, elem := range v.All() {
			walkReferences(elem, visit)
		}
	case *core.PdfStream:
		// Stream payload bytes do not participate in object graph
		// reachability (PDF stream data is opaque to the cross-reference
		// table). Only the stream dictionary is walked.
		if v.Dict != nil {
			walkReferences(v.Dict, visit)
		}
	}
}
