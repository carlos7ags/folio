// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/reader"
)

// textStringTitles covers an ASCII control, Latin-1 accents, and
// characters outside Latin-1 (em dash, curly quotes). The test adds a
// nested title with an astral-plane code point.
var textStringTitles = []string{
	"Reporte de consulta",
	"Información del Perfil",
	"Categorías de información",
	"Pontuação",
	"Résumé — naïve “quotes”",
}

// TestTextStringsRoundTrip verifies that outline titles and Info entries
// are written as PDF text strings (ISO 32000-1 §7.9.2.2) and decode back
// to the original Unicode text, instead of raw UTF-8 bytes that viewers
// misread as PDFDocEncoding ("InformaciÃ³n").
func TestTextStringsRoundTrip(t *testing.T) {
	doc := document.NewDocument(document.PageSizeA4)
	for range textStringTitles {
		doc.AddPage()
	}
	for i, title := range textStringTitles {
		o := doc.AddOutline(title, document.FitDest(i))
		if i == 1 {
			o.AddChild("Sección anidada 😀", document.FitDest(i))
		}
	}
	info := document.Info{
		Title:    "Informe — “prueba”",
		Author:   "José Núñez",
		Subject:  "Pontuação",
		Keywords: "consulta, información",
		Creator:  "Créateur",
		Producer: "folio ✓",
		Language: "es",
	}
	doc.Info = info

	var buf bytes.Buffer
	if _, err := doc.WriteTo(&buf); err != nil {
		t.Fatalf("WriteTo: %v", err)
	}
	pdf := buf.Bytes()

	if bytes.Contains(pdf, []byte("Informaci\xc3\xb3n")) {
		t.Error("raw UTF-8 bytes leaked into a PDF text string")
	}
	if !bytes.Contains(pdf, []byte("/Title (Reporte de consulta)")) {
		t.Error("ASCII outline title should stay a plain literal string")
	}

	r, err := reader.Parse(pdf)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	want := []string{
		textStringTitles[0],
		textStringTitles[1], "Sección anidada 😀",
		textStringTitles[2], textStringTitles[3], textStringTitles[4],
	}
	if got := outlineTitles(t, r); !slices.Equal(got, want) {
		t.Errorf("outline titles:\n got %q\nwant %q", got, want)
	}

	title, author, subject, creator, producer := r.Info()
	for _, c := range []struct{ key, got, want string }{
		{"Title", title, info.Title},
		{"Author", author, info.Author},
		{"Subject", subject, info.Subject},
		{"Creator", creator, info.Creator},
		{"Producer", producer, info.Producer},
	} {
		if c.got != c.want {
			t.Errorf("Info /%s = %q, want %q", c.key, c.got, c.want)
		}
	}
	infoDict := resolveDict(t, r, r.Trailer().Get("Info"))
	for key, want := range map[string]string{
		"Title": info.Title, "Author": info.Author, "Subject": info.Subject,
		"Keywords": info.Keywords, "Creator": info.Creator, "Producer": info.Producer,
	} {
		s, ok := infoDict.Get(key).(*core.PdfString)
		if !ok {
			t.Errorf("Info /%s missing or not a string", key)
			continue
		}
		if got := viewerDecode(s); got != want {
			t.Errorf("Info /%s decodes to %q in a viewer, want %q", key, got, want)
		}
	}
	if !bytes.Contains(pdf, []byte("/Lang (es)")) {
		t.Error("ASCII /Lang should stay a plain literal string")
	}

	qpdfCheck(t, pdf)
}

// outlineTitles walks the outline tree depth-first and returns each
// item's decoded /Title.
func outlineTitles(t *testing.T, r *reader.PdfReader) []string {
	t.Helper()
	root := resolveDict(t, r, r.Catalog().Get("Outlines"))
	var out []string
	var walk func(first core.PdfObject)
	walk = func(first core.PdfObject) {
		for item := first; item != nil; {
			d := resolveDict(t, r, item)
			s, ok := d.Get("Title").(*core.PdfString)
			if !ok {
				t.Fatalf("outline item without a string /Title: %v", d.Get("Title"))
			}
			out = append(out, viewerDecode(s))
			if c := d.Get("First"); c != nil {
				walk(c)
			}
			item = d.Get("Next")
		}
	}
	walk(root.Get("First"))
	return out
}

func resolveDict(t *testing.T, r *reader.PdfReader, obj core.PdfObject) *core.PdfDictionary {
	t.Helper()
	if obj == nil {
		t.Fatal("missing object")
	}
	res, err := r.ResolveObject(obj)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	d, ok := res.(*core.PdfDictionary)
	if !ok {
		t.Fatalf("expected dictionary, got %T", res)
	}
	return d
}

// viewerDecode decodes a PDF text string the way a conforming viewer
// does: a value with a byte-order mark is Unicode, anything else is
// PDFDocEncoding, one byte per character. Latin-1 stands in for
// PDFDocEncoding here; they agree on every byte these tests produce, so
// raw UTF-8 decodes to mojibake ("InformaciÃ³n") exactly as in a viewer.
func viewerDecode(s *core.PdfString) string {
	raw := s.Text()
	if strings.HasPrefix(raw, "\xFE\xFF") || strings.HasPrefix(raw, "\xEF\xBB\xBF") {
		return s.TextString()
	}
	r := make([]rune, len(raw))
	for i := range len(raw) {
		r[i] = rune(raw[i])
	}
	return string(r)
}

func qpdfCheck(t *testing.T, pdf []byte) {
	t.Helper()
	qpdf, err := exec.LookPath("qpdf")
	if err != nil {
		t.Log("qpdf not installed, skipping qpdf --check")
		return
	}
	path := filepath.Join(t.TempDir(), "out.pdf")
	if err := os.WriteFile(path, pdf, 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(qpdf, "--check", path).CombinedOutput(); err != nil {
		t.Errorf("qpdf --check failed: %v\n%s", err, out)
	}
}
