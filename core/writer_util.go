// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"io"
)

// countingWriter wraps an io.Writer and tracks the total bytes written.
// Used by composite types (array, dictionary) to accurately report
// the number of bytes written across multiple WriteTo calls.
type countingWriter struct {
	w io.Writer
	n int64
}

func (cw *countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	cw.n += int64(n)
	return n, err
}

func (cw *countingWriter) WriteString(s string) (int, error) {
	n, err := io.WriteString(cw.w, s)
	cw.n += int64(n)
	return n, err
}
