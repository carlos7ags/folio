// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

func TestArrayAddPanicsOnNil(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when adding nil to PdfArray")
		}
	}()
	a := NewPdfArray()
	a.Add(nil)
}
