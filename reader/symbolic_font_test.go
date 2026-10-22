// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
)

// TestSymbolicStandardFontRoundTrip writes Symbol and ZapfDingbats text
// (which folio emits without /Encoding) and reads it back: extraction must
// use the built-in encodings and widths, not WinAnsi.
func TestSymbolicStandardFontRoundTrip(t *testing.T) {
	doc := document.NewDocument(document.PageSizeLetter)
	p := doc.AddPage()
	p.AddText("\u03B1", font.Symbol, 10, 72, 700)
	p.AddText("\u2714", font.ZapfDingbats, 10, 72, 600)
	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}

	r, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	if !strings.Contains(text, "\u03B1") || !strings.Contains(text, "\u2714") {
		t.Errorf("ExtractText = %q, want \u03B1 and \u2714", text)
	}

	spans, err := page.TextSpans()
	if err != nil {
		t.Fatalf("TextSpans: %v", err)
	}
	// Widths come from the built-in code: α 631, ✔ 846 (not the 250/278
	// defaults a WinAnsi-indexed lookup of 'a'/'4' produced).
	want := map[string]float64{"\u03B1": 6.31, "\u2714": 8.46}
	for _, s := range spans {
		w, ok := want[s.Text]
		if !ok {
			continue
		}
		if math.Abs(s.Width-w) > 0.01 {
			t.Errorf("span %q width = %v, want %v", s.Text, s.Width, w)
		}
		delete(want, s.Text)
	}
	if len(want) != 0 {
		t.Errorf("missing spans for %v in %+v", want, spans)
	}
}

// TestSymbolicFontDifferencesUseBuiltinBase pins that a Symbol font whose
// /Encoding dictionary has /Differences but no /BaseEncoding decodes the
// remaining codes through Symbol's built-in encoding, not
// StandardEncoding.
func TestSymbolicFontDifferencesUseBuiltinBase(t *testing.T) {
	enc := core.NewPdfDictionary()
	enc.Set("Differences", core.NewPdfArray(core.NewPdfInteger(0x41), core.NewPdfName("bullet")))
	fd := core.NewPdfDictionary()
	fd.Set("Type", core.NewPdfName("Font"))
	fd.Set("Subtype", core.NewPdfName("Type1"))
	fd.Set("BaseFont", core.NewPdfName("Symbol"))
	fd.Set("Encoding", enc)

	fe := parseFontEntry(fd, nil)
	if fe == nil {
		t.Fatal("parseFontEntry returned nil")
	}
	if got := fe.Decode([]byte{0x41, 0x62}); got != "\u2022\u03B2" {
		t.Errorf("Decode = %q, want %q", got, "\u2022\u03B2")
	}
	if got := fe.CharWidth(0x62); got != 549 {
		t.Errorf("CharWidth(0x62) = %d, want 549 (beta)", got)
	}
}
