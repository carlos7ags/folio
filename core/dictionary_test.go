// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package core

import "testing"

func TestDictionarySetPanicsOnNilValue(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when setting nil value in PdfDictionary")
		}
	}()
	d := NewPdfDictionary()
	d.Set("Key", nil)
}
