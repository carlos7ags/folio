// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"bytes"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/font"
)

// TestAddTextSymbolicFontsUseBuiltinEncoding pins that Symbol and
// ZapfDingbats text is shown in the fonts' built-in encodings (they are
// written without /Encoding), not in WinAnsiEncoding.
func TestAddTextSymbolicFontsUseBuiltinEncoding(t *testing.T) {
	doc := NewDocument(PageSizeLetter)
	p := doc.AddPage()
	// α is code 0x61 ('a') in Symbol; 中 has no Symbol glyph and becomes
	// '?'. ✔ is code 0x34 ('4') in ZapfDingbats.
	p.AddText("\u4E2D\u03B1", font.Symbol, 12, 72, 700)
	p.AddText("\u2714", font.ZapfDingbats, 12, 72, 680)

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo failed: %v", err)
	}
	cs := decompressedContentStreams(t, buf.Bytes())
	if !strings.Contains(cs, "(?a) Tj") {
		t.Errorf("Symbol text not in built-in encoding; content:\n%s", cs)
	}
	if !strings.Contains(cs, "(4) Tj") {
		t.Errorf("ZapfDingbats text not in built-in encoding; content:\n%s", cs)
	}
}
