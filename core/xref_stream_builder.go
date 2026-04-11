// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import "fmt"

// XRefStreamSubsection is one contiguous run of cross-reference stream
// entries describing object numbers First, First+1, ..., First+len(Entries)-1.
//
// A complete xref stream is built from one or more subsections. The common
// case is a single subsection starting at 0 and covering every object in
// the file; that is what BuildXRefStream emits when given a single
// [0, N] subsection. Sparse subsections are supported because incremental
// updates need them, even though phase 1 of the optimizer does not.
type XRefStreamSubsection struct {
	First   int
	Entries []XRefStreamEntry
}

// BuildXRefStream assembles a cross-reference stream object per
// ISO 32000-1 §7.5.8.
//
// subsections describes the entries in object-number order; size is the
// value of the /Size entry, which must equal one more than the highest
// object number ever written to the file (§7.5.8.2 Table 17). extras
// supplies the entries that would otherwise live in the file trailer
// (/Root, /Info, /Encrypt, /ID, /Prev). The builder owns /Type, /Size,
// /W, /Index, /Filter, and /Length.
//
// The returned stream is configured with FlateDecode and writes its
// /Length on serialization. Phase 1 does not apply a PNG predictor
// (§7.5.8 Table 17 /DecodeParms): predictors save bytes on row-similar
// payloads but add a clean-room implementation burden, so the trade-off
// is deferred until the rest of the optimizer is in place.
func BuildXRefStream(subsections []XRefStreamSubsection, size int, extras *PdfDictionary) (*PdfStream, error) {
	if len(subsections) == 0 {
		return nil, fmt.Errorf("xref stream: at least one subsection required")
	}
	if size <= 0 {
		return nil, fmt.Errorf("xref stream: size must be positive, got %d", size)
	}

	var maxField2, maxField3 uint64
	totalEntries := 0
	for _, sub := range subsections {
		if sub.First < 0 {
			return nil, fmt.Errorf("xref stream: subsection First must be non-negative, got %d", sub.First)
		}
		if sub.First+len(sub.Entries) > size {
			return nil, fmt.Errorf("xref stream: subsection [%d,%d) extends past size %d",
				sub.First, sub.First+len(sub.Entries), size)
		}
		totalEntries += len(sub.Entries)
		for _, e := range sub.Entries {
			if e.Field2 > maxField2 {
				maxField2 = e.Field2
			}
			if e.Field3 > maxField3 {
				maxField3 = e.Field3
			}
		}
	}

	widths := XRefStreamWidths(int(maxField2), 0, 0, int(maxField3))
	rowSize := widths[0] + widths[1] + widths[2]

	payload := make([]byte, totalEntries*rowSize)
	pos := 0
	for _, sub := range subsections {
		for _, entry := range sub.Entries {
			if err := EncodeXRefStreamEntry(payload[pos:pos+rowSize], entry, widths); err != nil {
				return nil, err
			}
			pos += rowSize
		}
	}

	stream := NewPdfStreamCompressed(payload)
	dict := stream.Dict
	dict.Set("Type", NewPdfName("XRef"))
	dict.Set("Size", NewPdfInteger(size))

	wArr := NewPdfArray(
		NewPdfInteger(widths[0]),
		NewPdfInteger(widths[1]),
		NewPdfInteger(widths[2]),
	)
	dict.Set("W", wArr)

	if !isDefaultIndex(subsections, size) {
		idxArr := &PdfArray{}
		for _, sub := range subsections {
			idxArr.Add(NewPdfInteger(sub.First))
			idxArr.Add(NewPdfInteger(len(sub.Entries)))
		}
		dict.Set("Index", idxArr)
	}

	if extras != nil {
		for k, v := range extras.All() {
			// Reserved keys are owned by the builder.
			switch k {
			case "Type", "Size", "W", "Index", "Filter", "Length", "DecodeParms":
				continue
			}
			dict.Set(k, v)
		}
	}

	return stream, nil
}

// isDefaultIndex reports whether the subsection list is exactly the
// implicit default of a single subsection [0, size]. When true the
// builder omits /Index per §7.5.8.2, which permits the default.
func isDefaultIndex(subs []XRefStreamSubsection, size int) bool {
	return len(subs) == 1 && subs[0].First == 0 && len(subs[0].Entries) == size
}
