// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"io"
	"testing"
)

// TestStreamLengthIsDirect pins an invariant relied on by the
// object-stream packing path in document/writer_objstm.go.
//
// ISO 32000-1 §7.5.7 forbids placing an indirect object inside an
// /ObjStm if that object serves as the /Length value of any stream:
// the parser needs /Length before it can decompress the surrounding
// stream and cannot resolve a compressed object until it has finished
// parsing the xref. Folio satisfies this rule implicitly by always
// writing /Length as a direct integer.
//
// If a future refactor switches /Length to an indirect reference (for
// example, to share a length across multiple streams), this test will
// fail and the engineer making the change is forced to add an explicit
// eligibility check in writer_objstm.go before the optimizer can be
// trusted on the affected document.
func TestStreamLengthIsDirect(t *testing.T) {
	cases := []struct {
		name string
		s    *PdfStream
	}{
		{name: "uncompressed", s: NewPdfStream([]byte("hello"))},
		{name: "compressed", s: NewPdfStreamCompressed([]byte("hello world hello"))},
		{name: "empty", s: NewPdfStream(nil)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := c.s.WriteTo(io.Discard); err != nil {
				t.Fatalf("WriteTo: %v", err)
			}
			length := c.s.Dict.Get("Length")
			if length == nil {
				t.Fatal("/Length not set after WriteTo")
			}
			if _, ok := length.(*PdfNumber); !ok {
				t.Errorf("/Length is %T, want *PdfNumber (direct integer); "+
					"object stream eligibility in document/writer_objstm.go "+
					"depends on /Length never being indirect", length)
			}
		})
	}
}
