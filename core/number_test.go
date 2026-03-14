// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"math"
	"testing"
)

func TestFormatRealNaN(t *testing.T) {
	got := serialize(t, NewPdfReal(math.NaN()))
	if got != "0.0" {
		t.Errorf("expected %q for NaN, got %q", "0.0", got)
	}
}

func TestFormatRealPosInf(t *testing.T) {
	got := serialize(t, NewPdfReal(math.Inf(1)))
	if got != "0.0" {
		t.Errorf("expected %q for +Inf, got %q", "0.0", got)
	}
}

func TestFormatRealNegInf(t *testing.T) {
	got := serialize(t, NewPdfReal(math.Inf(-1)))
	if got != "0.0" {
		t.Errorf("expected %q for -Inf, got %q", "0.0", got)
	}
}
