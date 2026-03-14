// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"strings"
	"testing"
)

func TestSetFillColorCMYK(t *testing.T) {
	s := NewStream()
	s.SetFillColorCMYK(1, 0, 0.5, 0.2)
	got := string(s.Bytes())
	if !strings.Contains(got, "1 0 0.5 0.2 k") {
		t.Errorf("got %q, want CMYK fill operator", got)
	}
}

func TestSetStrokeColorCMYK(t *testing.T) {
	s := NewStream()
	s.SetStrokeColorCMYK(0, 1, 1, 0)
	got := string(s.Bytes())
	if !strings.Contains(got, "0 1 1 0 K") {
		t.Errorf("got %q, want CMYK stroke operator", got)
	}
}
