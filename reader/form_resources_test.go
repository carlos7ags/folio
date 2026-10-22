// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"fmt"
	"strings"
	"testing"
)

func formStream(dict, data string) string {
	return fmt.Sprintf("<< %s /Length %d >>\nstream\n%s\nendstream", dict, len(data), data)
}

// toUnicodeHi maps code 01 to "H" and 02 to "i", so text decodes only
// through the font that owns this CMap.
const toUnicodeHi = "/CIDInit /ProcSet findresource begin 12 dict begin begincmap " +
	"/CMapName /T def 1 begincodespacerange <00> <FF> endcodespacerange " +
	"2 beginbfchar <01> <0048> <02> <0069> endbfchar endcmap " +
	"CMapName currentdict /CMap defineresource pop end end"

func pageSpanText(t *testing.T, objs []string) string {
	t.Helper()
	r, err := Parse(buildPDFFromObjects(objs))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	spans, err := page.TextSpans()
	if err != nil {
		t.Fatalf("TextSpans: %v", err)
	}
	var texts []string
	for _, s := range spans {
		texts = append(texts, s.Text)
	}
	return strings.Join(texts, "|")
}

// A form's names resolve in its own /Resources (ISO 32000-1 §8.10.1): its
// fonts, and forms nested in it, are not in the page's resources.
func TestTextSpansFormOwnResources(t *testing.T) {
	got := pageSpanText(t, []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents 4 0 R /Resources << /XObject << /Fm1 5 0 R >> >> >>",
		formStream("", "/Fm1 Do"),
		formStream("/Type /XObject /Subtype /Form /BBox [0 0 200 200] /Resources << /Font << /F1 6 0 R >> /XObject << /Fm2 8 0 R >> >>",
			"BT /F1 12 Tf 10 100 Td <0102> Tj ET /Fm2 Do"),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /ToUnicode 7 0 R >>",
		formStream("", toUnicodeHi),
		formStream("/Type /XObject /Subtype /Form /BBox [0 0 200 200] /Resources << /Font << /F1 6 0 R >> >>",
			"BT /F1 12 Tf 10 50 Td <0201> Tj ET"),
	})
	if got != "Hi|iH" {
		t.Errorf("got %q, want %q", got, "Hi|iH")
	}
}

// A form without /Resources uses those of the content that draws it.
func TestTextSpansFormInheritsResources(t *testing.T) {
	got := pageSpanText(t, []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents 4 0 R /Resources << /Font << /F1 6 0 R >> /XObject << /Fm1 5 0 R >> >> >>",
		formStream("", "/Fm1 Do"),
		formStream("/Type /XObject /Subtype /Form /BBox [0 0 200 200]", "BT /F1 12 Tf 10 100 Td <0102> Tj ET"),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /ToUnicode 7 0 R >>",
		formStream("", toUnicodeHi),
	})
	if got != "Hi" {
		t.Errorf("got %q, want %q", got, "Hi")
	}
}

// The same name may denote different objects in the page and in a form;
// inside the form, the form's binding applies.
func TestTextSpansFormNameShadowsPage(t *testing.T) {
	got := pageSpanText(t, []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Contents 4 0 R /Resources << /XObject << /X 5 0 R >> >> >>",
		formStream("", "/X Do"),
		formStream("/Type /XObject /Subtype /Form /BBox [0 0 200 200] /Resources << /Font << /F1 7 0 R >> /XObject << /X 6 0 R >> >>",
			"BT /F1 12 Tf 10 100 Td (outer) Tj ET /X Do"),
		formStream("/Type /XObject /Subtype /Form /BBox [0 0 200 200] /Resources << /Font << /F1 7 0 R >> >>",
			"BT /F1 12 Tf 10 50 Td (inner) Tj ET"),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	})
	if got != "outer|inner" {
		t.Errorf("got %q, want %q", got, "outer|inner")
	}
}
