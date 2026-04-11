// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"fmt"
	"io"
	"math"
)

// PdfNumber represents a PDF numeric object — either integer or real
// (ISO 32000 §7.3.3). It tracks whether the value is integral so that
// integers serialize without a decimal point.
type PdfNumber struct {
	value     float64
	isInteger bool
}

// NewPdfInteger creates an integer PdfNumber.
func NewPdfInteger(v int) *PdfNumber {
	return &PdfNumber{value: float64(v), isInteger: true}
}

// NewPdfReal creates a real (floating-point) PdfNumber.
func NewPdfReal(v float64) *PdfNumber {
	return &PdfNumber{value: v, isInteger: false}
}

// Type returns ObjectTypeNumber.
func (n *PdfNumber) Type() ObjectType { return ObjectTypeNumber }

// IntValue returns the integer value, truncating reals toward zero.
// NaN and infinite values return 0 so malformed PDFs cannot propagate
// undefined conversions into the rest of the pipeline.
//
// Note: this method silently truncates real values and can overflow on
// 32-bit platforms for very large reals. Use [PdfNumber.IntValueChecked]
// when you need to distinguish between an integer, a truncated real,
// and an out-of-range value.
func (n *PdfNumber) IntValue() int {
	if math.IsNaN(n.value) || math.IsInf(n.value, 0) {
		return 0
	}
	return int(n.value)
}

// IntValueChecked returns the integer value and a boolean indicating whether
// the conversion was exact. ok is false if the underlying value is a real
// with a fractional part, is NaN or Inf, or overflows the platform int range.
func (n *PdfNumber) IntValueChecked() (value int, ok bool) {
	v := n.value
	if v != v { // NaN
		return 0, false
	}
	if v > float64(maxInt) || v < float64(minInt) {
		return 0, false
	}
	if !n.isInteger && v != float64(int64(v)) {
		return int(v), false
	}
	return int(v), true
}

// Platform int bounds for IntValueChecked overflow detection.
const (
	maxInt = int(^uint(0) >> 1)
	minInt = -maxInt - 1
)

// FloatValue returns the float64 value.
func (n *PdfNumber) FloatValue() float64 {
	return n.value
}

// IsInteger reports whether this number was created as an integer.
func (n *PdfNumber) IsInteger() bool {
	return n.isInteger
}

// WriteTo serializes the number as an integer or real to w.
func (n *PdfNumber) WriteTo(w io.Writer) (int64, error) {
	var s string
	if n.isInteger {
		s = fmt.Sprintf("%d", int64(n.value))
	} else {
		// Use %g to avoid trailing zeros, but ensure at least one decimal
		// place so it's clear this is a real number. PDF readers accept
		// both forms, but %g gives the most compact representation.
		s = formatReal(n.value)
	}
	written, err := fmt.Fprint(w, s)
	return int64(written), err
}

// formatReal formats a float64 for PDF output.
// NaN and Inf are replaced with 0 to avoid invalid PDF tokens.
func formatReal(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0.0"
	}
	// Handle special case of integer-valued reals
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return fmt.Sprintf("%.1f", v)
	}
	// Trim trailing zeros from fixed-point representation
	s := fmt.Sprintf("%.6f", v)
	// Remove trailing zeros after decimal point
	i := len(s) - 1
	for i > 0 && s[i] == '0' {
		i--
	}
	// Keep at least one digit after decimal point
	if s[i] == '.' {
		i++
	}
	return s[:i+1]
}
