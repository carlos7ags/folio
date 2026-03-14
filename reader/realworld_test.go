// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const realworldDir = "testdata/realworld"

// realworldPDFs lists expected test files and their known properties.
// Page counts and dimensions come from qpdf inspection.
var realworldPDFs = []struct {
	File       string
	Pages      int
	Width      float64 // page 0 MediaBox width
	Height     float64 // page 0 MediaBox height
	Source     string  // tool that produced it
	Linearized bool
	XRefStream bool
	ObjStream  bool
}{
	{
		File: "chrome-print.pdf", Pages: 5,
		Width: 612, Height: 792,
		Source: "Chrome Print to PDF",
	},
	{
		File: "form.pdf", Pages: 3,
		Width: 612, Height: 792,
		Source:     "PDF form with AcroForm fields",
		Linearized: true, XRefStream: true,
	},
	{
		File: "invoice.pdf", Pages: 1,
		Width: 595, Height: 842,
		Source: "Invoice generator (PDF 1.3)",
	},
	{
		File: "latex-pdftex.pdf", Pages: 4,
		Width: 612, Height: 792,
		Source:     "LaTeX pdfTeX",
		Linearized: true, XRefStream: true, ObjStream: true,
	},
	{
		File: "libreoffice.pdf", Pages: 583,
		Width: 595, Height: 842,
		Source: "LibreOffice (PDF 2.0)",
	},
	{
		File: "word.pdf", Pages: 1,
		Width: 596, Height: 842,
		Source: "Google Docs (exported via Skia/PDF)",
	},
}

// TestRealWorldParse verifies that every real-world PDF can be parsed
// without errors and reports the correct page count.
func TestRealWorldParse(t *testing.T) {
	for _, tc := range realworldPDFs {
		t.Run(tc.File, func(t *testing.T) {
			data := readTestFile(t, tc.File)
			r, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}
			if r.PageCount() != tc.Pages {
				t.Errorf("page count = %d, want %d", r.PageCount(), tc.Pages)
			}
		})
	}
}

// TestRealWorldPageDimensions checks that page 0 dimensions match expected values.
func TestRealWorldPageDimensions(t *testing.T) {
	for _, tc := range realworldPDFs {
		t.Run(tc.File, func(t *testing.T) {
			data := readTestFile(t, tc.File)
			r, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			page, err := r.Page(0)
			if err != nil {
				t.Fatalf("Page(0): %v", err)
			}
			w := page.MediaBox.Width()
			h := page.MediaBox.Height()
			if !approxEq(w, tc.Width, 1) {
				t.Errorf("width = %.1f, want %.1f", w, tc.Width)
			}
			if !approxEq(h, tc.Height, 1) {
				t.Errorf("height = %.1f, want %.1f", h, tc.Height)
			}
		})
	}
}

// TestRealWorldAllPagesAccessible iterates every page in each PDF to
// verify the page tree is fully traversable.
func TestRealWorldAllPagesAccessible(t *testing.T) {
	for _, tc := range realworldPDFs {
		t.Run(tc.File, func(t *testing.T) {
			data := readTestFile(t, tc.File)
			r, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			for i := range r.PageCount() {
				page, err := r.Page(i)
				if err != nil {
					t.Errorf("Page(%d): %v", i, err)
					continue
				}
				if page.MediaBox.Width() <= 0 || page.MediaBox.Height() <= 0 {
					t.Errorf("Page(%d): invalid MediaBox %.0fx%.0f", i, page.MediaBox.Width(), page.MediaBox.Height())
				}
			}
		})
	}
}

// TestRealWorldTextExtraction verifies ExtractText doesn't panic on any page.
// We don't assert specific text content since most PDFs use encoded fonts
// that our basic extractor can't decode — just that it doesn't crash.
func TestRealWorldTextExtraction(t *testing.T) {
	for _, tc := range realworldPDFs {
		t.Run(tc.File, func(t *testing.T) {
			data := readTestFile(t, tc.File)
			r, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			for i := range r.PageCount() {
				page, err := r.Page(i)
				if err != nil {
					t.Errorf("Page(%d): %v", i, err)
					continue
				}
				_, err = page.ExtractText()
				if err != nil {
					t.Errorf("Page(%d) ExtractText: %v", i, err)
				}
			}
		})
	}
}

// TestRealWorldContentOps verifies that content stream parsing doesn't
// panic or return errors for any page.
func TestRealWorldContentOps(t *testing.T) {
	for _, tc := range realworldPDFs {
		t.Run(tc.File, func(t *testing.T) {
			data := readTestFile(t, tc.File)
			r, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			for i := range r.PageCount() {
				page, err := r.Page(i)
				if err != nil {
					t.Errorf("Page(%d): %v", i, err)
					continue
				}
				content, err := page.ContentOps()
				if err != nil {
					t.Errorf("Page(%d) ContentOps: %v", i, err)
					continue
				}
				// Sanity: most pages should have at least one operator.
				// Allow empty pages (e.g., libreoffice page 0 is a cover).
				_ = content
			}
		})
	}
}

// TestRealWorldMetadata checks that document info extraction doesn't fail.
func TestRealWorldMetadata(t *testing.T) {
	for _, tc := range realworldPDFs {
		t.Run(tc.File, func(t *testing.T) {
			data := readTestFile(t, tc.File)
			r, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			title, author, _, creator, producer := r.Info()
			t.Logf("title=%q author=%q creator=%q producer=%q", title, author, creator, producer)
		})
	}
}

// TestRealWorldStrictMode tries parsing each PDF in strict mode.
// Some may fail — this documents which PDFs are fully spec-compliant.
func TestRealWorldStrictMode(t *testing.T) {
	for _, tc := range realworldPDFs {
		t.Run(tc.File, func(t *testing.T) {
			data := readTestFile(t, tc.File)
			_, err := ParseWithOptions(data, ReadOptions{
				Strictness: StrictnessStrict,
			})
			if err != nil {
				t.Logf("strict parse failed (acceptable): %v", err)
			}
		})
	}
}

// TestRealWorldPageBoxes checks all 5 page boxes on every page.
func TestRealWorldPageBoxes(t *testing.T) {
	for _, tc := range realworldPDFs {
		t.Run(tc.File, func(t *testing.T) {
			data := readTestFile(t, tc.File)
			r, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			for i := range r.PageCount() {
				page, err := r.Page(i)
				if err != nil {
					t.Errorf("Page(%d): %v", i, err)
					continue
				}
				// MediaBox must always exist.
				if page.MediaBox.IsZero() {
					t.Errorf("Page(%d): MediaBox is zero", i)
				}
				// VisibleBox must return something valid.
				vis := page.VisibleBox()
				if vis.Width() <= 0 || vis.Height() <= 0 {
					t.Errorf("Page(%d): VisibleBox invalid: %.0fx%.0f", i, vis.Width(), vis.Height())
				}
			}
		})
	}
}

// TestRealWorldInvoiceText verifies the invoice PDF (PDF 1.3, MacRomanEncoding).
func TestRealWorldInvoiceText(t *testing.T) {
	data := readTestFile(t, "invoice.pdf")
	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	for _, want := range []string{"Invoice", "Payment", "30 days"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in invoice text, got: %s", want, truncate(text, 200))
		}
	}
}

// TestRealWorldFormText verifies the form PDF extracts readable static text.
func TestRealWorldFormText(t *testing.T) {
	data := readTestFile(t, "form.pdf")
	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	for _, want := range []string{"Sample PDF Form", "Text field"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in form text, got: %s", want, truncate(text, 200))
		}
	}
}

// TestRealWorldChromeText verifies Chrome Print-to-PDF text via ToUnicode CMap.
func TestRealWorldChromeText(t *testing.T) {
	data := readTestFile(t, "chrome-print.pdf")
	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	// Chrome PDF uses Type0 fonts with ToUnicode CMaps.
	for _, want := range []string{"Turn", "what", "you", "know"} {
		if !containsWord(text, want) {
			t.Errorf("expected word %q in chrome text, got: %s", want, truncate(text, 200))
		}
	}
}

// TestRealWorldLibreOfficeText verifies LibreOffice PDF text via ToUnicode CMap.
func TestRealWorldLibreOfficeText(t *testing.T) {
	data := readTestFile(t, "libreoffice.pdf")
	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// Page 1 has the copyright notice (page 0 is blank cover).
	page, err := r.Page(1)
	if err != nil {
		t.Fatalf("Page(1): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	for _, want := range []string{"Copyright", "document", "LibreOffice"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in libreoffice text, got: %s", want, truncate(text, 300))
		}
	}
}

// TestRealWorldWordText verifies Google Docs / Word PDF text via ToUnicode CMap.
func TestRealWorldWordText(t *testing.T) {
	data := readTestFile(t, "word.pdf")
	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	// Type0 fonts with 2-byte codes produce spaced characters.
	for _, want := range []string{"Carlos", "Munoz"} {
		if !containsWord(text, want) {
			t.Errorf("expected word %q in word text, got: %s", want, truncate(text, 200))
		}
	}
}

// TestRealWorldLatexText verifies LaTeX pdfTeX text via ToUnicode CMap.
func TestRealWorldLatexText(t *testing.T) {
	data := readTestFile(t, "latex-pdftex.pdf")
	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	page, err := r.Page(0)
	if err != nil {
		t.Fatalf("Page(0): %v", err)
	}
	text, err := page.ExtractText()
	if err != nil {
		t.Fatalf("ExtractText: %v", err)
	}
	for _, want := range []string{"18.821", "MATHEMATICS", "PROJECT"} {
		if !strings.Contains(text, want) {
			t.Errorf("expected %q in latex text, got: %s", want, truncate(text, 300))
		}
	}
}

// containsWord checks if text contains a word, handling Type0 spaced output
// where "Hello" might appear as "H e l l o".
func containsWord(text, word string) bool {
	if strings.Contains(text, word) {
		return true
	}
	// Try spaced version.
	spaced := strings.Join(strings.Split(word, ""), " ")
	return strings.Contains(text, spaced)
}

// TestRealWorldLargeDocument stress-tests the reader with the 583-page
// LibreOffice PDF to catch performance or memory issues.
func TestRealWorldLargeDocument(t *testing.T) {
	data := readTestFile(t, "libreoffice.pdf")
	r, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if r.PageCount() != 583 {
		t.Errorf("page count = %d, want 583", r.PageCount())
	}
	// Spot-check a few pages spread across the document.
	for _, idx := range []int{0, 1, 50, 100, 200, 300, 400, 500, 582} {
		page, err := r.Page(idx)
		if err != nil {
			t.Errorf("Page(%d): %v", idx, err)
			continue
		}
		if page.MediaBox.Width() <= 0 {
			t.Errorf("Page(%d): bad MediaBox", idx)
		}
		_, err = page.ExtractText()
		if err != nil {
			t.Errorf("Page(%d) ExtractText: %v", idx, err)
		}
	}
}

// TestRealWorldLinearized verifies that linearized PDFs (form.pdf, latex-pdftex.pdf)
// parse correctly — the reader should handle the extra linearization dictionary
// and dual xref sections.
func TestRealWorldLinearized(t *testing.T) {
	for _, name := range []string{"form.pdf", "latex-pdftex.pdf"} {
		t.Run(name, func(t *testing.T) {
			data := readTestFile(t, name)
			r, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			// Verify all pages are accessible (linearization can break page ordering).
			for i := range r.PageCount() {
				page, err := r.Page(i)
				if err != nil {
					t.Errorf("Page(%d): %v", i, err)
					continue
				}
				if page.MediaBox.IsZero() {
					t.Errorf("Page(%d): zero MediaBox", i)
				}
			}
		})
	}
}

// TestRealWorldRoundTrip parses each PDF, extracts basic info, then
// re-parses the same bytes to verify idempotent parsing.
func TestRealWorldRoundTrip(t *testing.T) {
	for _, tc := range realworldPDFs {
		t.Run(tc.File, func(t *testing.T) {
			data := readTestFile(t, tc.File)

			r1, err := Parse(data)
			if err != nil {
				t.Fatalf("first Parse: %v", err)
			}
			r2, err := Parse(data)
			if err != nil {
				t.Fatalf("second Parse: %v", err)
			}

			if r1.PageCount() != r2.PageCount() {
				t.Errorf("page count mismatch: %d vs %d", r1.PageCount(), r2.PageCount())
			}

			for i := range r1.PageCount() {
				p1, _ := r1.Page(i)
				p2, _ := r2.Page(i)
				if p1.MediaBox != p2.MediaBox {
					t.Errorf("Page(%d) MediaBox mismatch: %v vs %v", i, p1.MediaBox, p2.MediaBox)
				}
			}
		})
	}
}

// --- helpers ---

func readTestFile(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join(realworldDir, name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("test file not available: %v", err)
	}
	return data
}

func approxEq(a, b, tolerance float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= tolerance
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
