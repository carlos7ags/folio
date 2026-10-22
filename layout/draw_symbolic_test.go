// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import (
	"strings"
	"testing"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/font"
)

// TestDrawWordStandardSymbolicEncoding pins that standard-font words are
// shown in the font's own encoding: Symbol and ZapfDingbats use their
// built-in encodings, every other standard font WinAnsiEncoding.
func TestDrawWordStandardSymbolicEncoding(t *testing.T) {
	tests := []struct {
		f    *font.Standard
		text string
		want string
	}{
		{font.Symbol, "\u03B1\u03B2", "(ab) Tj"},
		{font.Symbol, "a", "(a) Tj"},
		{font.Symbol, "\u4E2D", "(?) Tj"},
		{font.ZapfDingbats, "\u2714", "(4) Tj"},
		{font.Helvetica, "\u00E9", "(\xE9) Tj"},
	}
	for _, tt := range tests {
		s := content.NewStream()
		drawWordStandard(s, Word{Text: tt.text, Font: tt.f, FontSize: 12})
		if got := string(s.Bytes()); !strings.Contains(got, tt.want) {
			t.Errorf("%s %q: got %q, want %q", tt.f.Name(), tt.text, got, tt.want)
		}
	}
}
