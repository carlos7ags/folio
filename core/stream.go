// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

// PdfStream represents a PDF stream object (ISO 32000 §7.3.8).
// A stream consists of a dictionary followed by a sequence of bytes
// enclosed between "stream" and "endstream" keywords.
type PdfStream struct {
	Dict     *PdfDictionary
	Data     []byte
	compress bool // if true, apply FlateDecode on WriteTo
	levelSet bool // whether SetCompressLevel was called
	level    int  // zlib level to use when levelSet is true
}

// NewPdfStream creates a stream with the given data (uncompressed).
// The /Length entry is managed automatically during serialization.
func NewPdfStream(data []byte) *PdfStream {
	return &PdfStream{
		Dict: NewPdfDictionary(),
		Data: data,
	}
}

// NewPdfStreamCompressed creates a stream that will be compressed
// with FlateDecode (zlib) when written. The Data field holds the
// uncompressed bytes; compression happens during WriteTo.
func NewPdfStreamCompressed(data []byte) *PdfStream {
	return &PdfStream{
		Dict:     NewPdfDictionary(),
		Data:     data,
		compress: true,
	}
}

// SetCompress enables or disables FlateDecode compression for this stream.
func (s *PdfStream) SetCompress(enabled bool) {
	s.compress = enabled
}

// SetCompressLevel selects the zlib level WriteTo uses when compressing
// this stream (see compress/zlib level constants, e.g. zlib.BestSpeed,
// zlib.DefaultCompression, zlib.NoCompression, zlib.HuffmanOnly). It has
// no effect unless the stream is also set to compress (SetCompress(true)
// or NewPdfStreamCompressed). Without a call to SetCompressLevel, WriteTo
// uses zlib.BestCompression, matching historical behavior.
func (s *PdfStream) SetCompressLevel(level int) {
	s.levelSet = true
	s.level = level
}

// effectiveLevel returns the zlib level WriteTo should use: the level set
// via SetCompressLevel, or zlib.BestCompression by default. levelSet
// (rather than a zero-value check) is required because 0 is itself a
// valid zlib level (zlib.NoCompression).
func (s *PdfStream) effectiveLevel() int {
	if s.levelSet {
		return s.level
	}
	return zlib.BestCompression
}

// WillCompress reports whether WriteTo will Flate-compress this stream's
// payload before serializing. Useful for writer-side passes that want to
// skip streams the writer is already going to compress at BestCompression.
func (s *PdfStream) WillCompress() bool {
	return s.compress
}

// Type returns ObjectTypeStream.
func (s *PdfStream) Type() ObjectType { return ObjectTypeStream }

// WriteTo serializes the stream dictionary and data bytes to w.
func (s *PdfStream) WriteTo(w io.Writer) (int64, error) {
	cw := &countingWriter{w: w}

	// Determine the actual bytes to write (compressed or raw)
	streamData := s.Data
	if s.compress {
		compressed, err := DeflateStreamDataLevel(s.Data, s.effectiveLevel())
		if err != nil {
			return 0, fmt.Errorf("core: flate compress: %w", err)
		}
		streamData = compressed
		s.Dict.Set("Filter", NewPdfName("FlateDecode"))
	}

	s.Dict.Set("Length", NewPdfInteger(len(streamData)))

	if _, err := s.Dict.WriteTo(cw); err != nil {
		return cw.n, err
	}

	// Per spec: "stream" keyword followed by a single EOL (LF or CR+LF),
	// then the data, then EOL + "endstream".
	if _, err := fmt.Fprint(cw, "\nstream\n"); err != nil {
		return cw.n, err
	}
	if _, err := cw.Write(streamData); err != nil {
		return cw.n, err
	}
	if _, err := fmt.Fprint(cw, "\nendstream"); err != nil {
		return cw.n, err
	}

	return cw.n, nil
}

// DeflateStreamData compresses data using zlib (RFC 1950) at
// zlib.BestCompression — the encoding PDF's FlateDecode filter
// (ISO 32000-1 §7.4.4) expects. Exposed as a writer-side primitive
// for passes that need to produce FlateDecode payloads independently
// of [PdfStream.WriteTo].
func DeflateStreamData(data []byte) ([]byte, error) {
	return DeflateStreamDataLevel(data, zlib.BestCompression)
}

// DeflateStreamDataLevel compresses data using zlib (RFC 1950) at the
// given level (see compress/zlib level constants). It is the level-aware
// primitive behind [DeflateStreamData] and [PdfStream.SetCompressLevel].
func DeflateStreamDataLevel(data []byte, level int) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, level)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// InflateStreamData is the inverse of [DeflateStreamData]: it
// decompresses zlib-framed bytes (PDF FlateDecode payload). It is the
// writer-side counterpart used by the recompression pass; the reader
// has its own size-bounded inflate that defends against zip bombs at
// parse time. Streams reaching the writer are already in memory, so
// this helper does not impose a size cap — callers control input size.
func InflateStreamData(data []byte) ([]byte, error) {
	r, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("core: inflate: %w", err)
	}
	defer func() { _ = r.Close() }()
	out, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("core: inflate: %w", err)
	}
	return out, nil
}
