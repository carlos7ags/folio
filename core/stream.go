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

// Type returns ObjectTypeStream.
func (s *PdfStream) Type() ObjectType { return ObjectTypeStream }

// WriteTo serializes the stream dictionary and data bytes to w.
func (s *PdfStream) WriteTo(w io.Writer) (int64, error) {
	cw := &countingWriter{w: w}

	// Determine the actual bytes to write (compressed or raw)
	streamData := s.Data
	if s.compress {
		compressed, err := deflate(s.Data)
		if err != nil {
			return 0, fmt.Errorf("flate compress: %w", err)
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

// deflate compresses data using zlib (RFC 1950), which is what PDF's
// FlateDecode filter expects.
func deflate(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
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
