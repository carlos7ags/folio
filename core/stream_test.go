// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"compress/zlib"
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

// TestDeflateStreamDataLevel verifies compressed output at a non-default
// level still inflates back to the original bytes, and that the default
// wrapper's output is byte-identical to explicitly requesting
// zlib.BestCompression.
func TestDeflateStreamDataLevel(t *testing.T) {
	payload := bytes.Repeat([]byte("hello world, compress me please "), 50)

	fast, err := DeflateStreamDataLevel(payload, zlib.BestSpeed)
	if err != nil {
		t.Fatalf("DeflateStreamDataLevel(BestSpeed): %v", err)
	}
	back, err := InflateStreamData(fast)
	if err != nil {
		t.Fatalf("InflateStreamData: %v", err)
	}
	if !bytes.Equal(back, payload) {
		t.Error("BestSpeed round-trip did not reproduce the original payload")
	}

	viaDefault, err := DeflateStreamData(payload)
	if err != nil {
		t.Fatalf("DeflateStreamData: %v", err)
	}
	viaExplicit, err := DeflateStreamDataLevel(payload, zlib.BestCompression)
	if err != nil {
		t.Fatalf("DeflateStreamDataLevel(BestCompression): %v", err)
	}
	if !bytes.Equal(viaDefault, viaExplicit) {
		t.Error("DeflateStreamData diverges from DeflateStreamDataLevel(_, zlib.BestCompression)")
	}
}

// TestPdfStreamSetCompressLevel verifies SetCompressLevel changes the
// level WriteTo uses, including the zlib.NoCompression edge case (0,
// which collides with the PdfStream zero value and must not fall back
// to BestCompression once SetCompressLevel has been called explicitly).
func TestPdfStreamSetCompressLevel(t *testing.T) {
	payload := bytes.Repeat([]byte("round trip through NoCompression "), 50)

	s := NewPdfStreamCompressed(payload)
	s.SetCompressLevel(zlib.NoCompression)

	var buf bytes.Buffer
	if _, err := s.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	// Re-inflate the stream body between "stream\n" and "\nendstream".
	const marker = "\nstream\n"
	idx := bytes.Index(buf.Bytes(), []byte(marker))
	if idx < 0 {
		t.Fatal("missing stream marker in output")
	}
	body := buf.Bytes()[idx+len(marker):]
	body = bytes.TrimSuffix(body, []byte("\nendstream"))

	back, err := InflateStreamData(body)
	if err != nil {
		t.Fatalf("InflateStreamData: %v", err)
	}
	if !bytes.Equal(back, payload) {
		t.Error("NoCompression round-trip did not reproduce the original payload")
	}

	// Default (no SetCompressLevel call) must still behave as before:
	// byte-identical to the historical BestCompression path.
	plain := NewPdfStreamCompressed(payload)
	var plainBuf bytes.Buffer
	if _, err := plain.WriteTo(&plainBuf); err != nil {
		t.Fatalf("WriteTo (default): %v", err)
	}
	explicit := NewPdfStreamCompressed(payload)
	explicit.SetCompressLevel(zlib.BestCompression)
	var explicitBuf bytes.Buffer
	if _, err := explicit.WriteTo(&explicitBuf); err != nil {
		t.Fatalf("WriteTo (explicit BestCompression): %v", err)
	}
	if !bytes.Equal(plainBuf.Bytes(), explicitBuf.Bytes()) {
		t.Error("default level diverges from explicit zlib.BestCompression")
	}
}
