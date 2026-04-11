// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"fmt"
)

// ObjStmEntry is a single object packed into an object stream. Object
// streams (ISO 32000-1 §7.5.7) carry only direct objects belonging to
// generation zero, so this struct intentionally records no generation
// number — the value is implicitly zero. Stream objects are not eligible
// for inclusion and the builder rejects them.
type ObjStmEntry struct {
	ObjectNumber int
	Object       PdfObject
}

// ObjStmPlacement reports where an entry ended up after BuildObjStm:
// the object number of the containing object stream and the entry's
// index within that stream. The writer uses this mapping to emit a
// type-2 cross-reference entry per ISO 32000-1 §7.5.8.3 Table 18.
//
// Index is the index within the object stream's header table (0-based),
// not a byte offset.
type ObjStmPlacement struct {
	ObjectNumber  int // the original object number (gen 0 implied)
	ObjStmObjNum  int // the object number that will be assigned to the containing /ObjStm
	IndexInObjStm int
}

// BuildObjStm assembles an /ObjStm indirect object per ISO 32000-1 §7.5.7
// from the supplied entries.
//
// The decoded stream consists of a header — N pairs of "objNum offset"
// integers, where offset is the byte offset of the corresponding object
// body relative to /First — followed by the N object bodies in the same
// order, each rendered with the standard PdfObject.WriteTo. /First is set
// to the length of the header so the bodies begin at exactly that offset
// after decompression.
//
// Entries are emitted in the order supplied. Callers that need a
// canonical layout must sort their entries before calling; the builder
// does not reorder so that callers retain control over the layout.
//
// The builder rejects:
//
//   - empty entry lists (a /ObjStm must compress at least one object);
//   - entries whose object number is <= 0 (object number 0 is the free
//     list head and cannot be assigned to a real object);
//   - duplicate object numbers within a single object stream (the
//     parser would resolve only one of them);
//   - stream objects (§7.5.7: streams cannot be compressed inside
//     another stream).
//
// FlateDecode is applied via the existing compressed-stream path. /Length
// is set on serialization. The returned stream's dictionary has /Type,
// /N, and /First populated; the caller is free to add /Extends or
// other entries before serialization, but must not overwrite the
// builder-owned keys.
func BuildObjStm(entries []ObjStmEntry) (*PdfStream, error) {
	if len(entries) == 0 {
		return nil, fmt.Errorf("objstm: no entries")
	}

	seen := make(map[int]struct{}, len(entries))
	var bodies bytes.Buffer
	offsets := make([]int, len(entries))

	for i, e := range entries {
		if e.Object == nil {
			return nil, fmt.Errorf("objstm: entry %d: nil object", i)
		}
		if e.ObjectNumber <= 0 {
			return nil, fmt.Errorf("objstm: entry %d: object number must be positive, got %d", i, e.ObjectNumber)
		}
		if _, dup := seen[e.ObjectNumber]; dup {
			return nil, fmt.Errorf("objstm: duplicate object number %d", e.ObjectNumber)
		}
		seen[e.ObjectNumber] = struct{}{}

		if e.Object.Type() == ObjectTypeStream {
			return nil, fmt.Errorf("objstm: entry %d (object %d): stream objects cannot be compressed", i, e.ObjectNumber)
		}

		if i > 0 {
			// Single LF separator between bodies. §7.5.7 allows any
			// whitespace; LF is the minimal deterministic choice.
			if err := bodies.WriteByte('\n'); err != nil {
				return nil, err
			}
		}
		offsets[i] = bodies.Len()
		if _, err := e.Object.WriteTo(&bodies); err != nil {
			return nil, fmt.Errorf("objstm: entry %d (object %d): serialize body: %w", i, e.ObjectNumber, err)
		}
	}

	var header bytes.Buffer
	for i, e := range entries {
		// One pair per line: "objNum SP offset LF". Easier to debug than
		// run-on whitespace and equally valid per §7.5.7.
		if _, err := fmt.Fprintf(&header, "%d %d\n", e.ObjectNumber, offsets[i]); err != nil {
			return nil, err
		}
	}

	first := header.Len()

	data := make([]byte, 0, header.Len()+bodies.Len())
	data = append(data, header.Bytes()...)
	data = append(data, bodies.Bytes()...)

	stream := NewPdfStreamCompressed(data)
	dict := stream.Dict
	dict.Set("Type", NewPdfName("ObjStm"))
	dict.Set("N", NewPdfInteger(len(entries)))
	dict.Set("First", NewPdfInteger(first))

	return stream, nil
}
