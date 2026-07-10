// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"testing"

	"github.com/carlos7ags/folio/font"
)

func TestResolveMeasurer(t *testing.T) {
	embedded := font.NewEmbeddedFont(newFakeFace('a'))
	std := font.Helvetica

	t.Run("embedded and std set returns embedded", func(t *testing.T) {
		got := resolveMeasurer(embedded, std, nil)
		if got != font.TextMeasurer(embedded) {
			t.Errorf("got %v, want embedded", got)
		}
	})

	t.Run("embedded set, std nil returns embedded", func(t *testing.T) {
		got := resolveMeasurer(embedded, nil, nil)
		if got != font.TextMeasurer(embedded) {
			t.Errorf("got %v, want embedded", got)
		}
	})

	t.Run("embedded nil, std set returns std", func(t *testing.T) {
		got := resolveMeasurer(nil, std, nil)
		if got != font.TextMeasurer(std) {
			t.Errorf("got %v, want std", got)
		}
	})

	t.Run("both nil falls back to Helvetica", func(t *testing.T) {
		got := resolveMeasurer(nil, nil, font.Helvetica)
		if got != font.TextMeasurer(font.Helvetica) {
			t.Errorf("got %v, want font.Helvetica", got)
		}
	})

	t.Run("both nil with untyped nil fallback is nil", func(t *testing.T) {
		got := resolveMeasurer(nil, nil, nil)
		if got != nil {
			t.Errorf("got %v, want untyped nil", got)
		}
	})

	t.Run("both nil with typed nil fallback is not nil", func(t *testing.T) {
		var nilStd *font.Standard
		got := resolveMeasurer(nil, nil, nilStd)
		if got == nil {
			t.Error("got untyped nil, want typed-nil interface (regression: nil-standard-field fallback)")
		}
	})
}
