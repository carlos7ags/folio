// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"bytes"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
)

// TestParseFolioXRefStreamWriter is the round-trip check for the
// xref-stream writer mode added in phase 1 of the optimizer. The
// reader already supports parsing /Type /XRef streams (§7.5.8) for
// real-world inputs; this test pins the contract that folio's own
// writer output is consumable by folio's own reader.
//
// It lives in the reader package to break the document → reader cycle
// that prevents document tests from importing reader.
func TestParseFolioXRefStreamWriter(t *testing.T) {
	w := document.NewWriter("1.7")

	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))

	pages := core.NewPdfDictionary()
	pages.Set("Type", core.NewPdfName("Pages"))
	pages.Set("Kids", core.NewPdfArray())
	pages.Set("Count", core.NewPdfInteger(0))

	catalogRef := w.AddObject(catalog)
	pagesRef := w.AddObject(pages)
	catalog.Set("Pages", pagesRef)
	w.SetRoot(catalogRef)

	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, document.WriteOptions{UseXRefStream: true}); err != nil {
		t.Fatalf("write: %v", err)
	}

	r, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if r.PageCount() != 0 {
		t.Errorf("page count = %d, want 0", r.PageCount())
	}
	cat := r.Catalog()
	if cat == nil {
		t.Fatal("nil catalog")
	}
	if name, ok := cat.Get("Type").(*core.PdfName); !ok || name.Value != "Catalog" {
		t.Errorf("catalog /Type = %v, want /Catalog", cat.Get("Type"))
	}
	tr := r.Trailer()
	if tr == nil {
		t.Fatal("nil trailer (xref stream dict)")
	}
	if name, ok := tr.Get("Type").(*core.PdfName); !ok || name.Value != "XRef" {
		t.Errorf("trailer /Type = %v, want /XRef", tr.Get("Type"))
	}
}

func TestParseFolioXRefStreamWriterMultiObject(t *testing.T) {
	// Exercise the dense-subsection path with enough objects to push
	// field 2 width past one byte (offsets > 255).
	w := document.NewWriter("1.7")
	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))
	pages := core.NewPdfDictionary()
	pages.Set("Type", core.NewPdfName("Pages"))
	pages.Set("Kids", core.NewPdfArray())
	pages.Set("Count", core.NewPdfInteger(0))
	catalogRef := w.AddObject(catalog)
	pagesRef := w.AddObject(pages)
	catalog.Set("Pages", pagesRef)
	w.SetRoot(catalogRef)
	for i := 0; i < 25; i++ {
		d := core.NewPdfDictionary()
		d.Set("Index", core.NewPdfInteger(i))
		w.AddObject(d)
	}

	var buf bytes.Buffer
	if _, err := w.WriteToWithOptions(&buf, document.WriteOptions{UseXRefStream: true}); err != nil {
		t.Fatalf("write: %v", err)
	}

	r, err := Parse(buf.Bytes())
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// MaxObjectNumber should equal the total user objects + the xref stream
	// itself = 2 (catalog + pages) + 25 fillers + 1 xref stream = 28.
	if got := r.MaxObjectNumber(); got != 28 {
		t.Errorf("MaxObjectNumber = %d, want 28", got)
	}
}
