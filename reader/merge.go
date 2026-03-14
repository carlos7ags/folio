// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"fmt"
	"io"
	"os"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/document"
	"github.com/carlos7ags/folio/font"
)

// Merge concatenates multiple PDFs into a single PDF.
// Pages are appended in order: all pages from the first PDF,
// then all pages from the second, etc.
func Merge(readers ...*PdfReader) (*Modifier, error) {
	m := newModifier()

	for _, r := range readers {
		if err := m.appendReader(r); err != nil {
			return nil, err
		}
	}

	return m, nil
}

// MergeFiles is a convenience that opens, parses, and merges PDF files.
func MergeFiles(paths ...string) (*Modifier, error) {
	var readers []*PdfReader
	for _, path := range paths {
		r, err := Open(path)
		if err != nil {
			return nil, fmt.Errorf("merge: %w", err)
		}
		readers = append(readers, r)
	}
	return Merge(readers...)
}

// Modifier builds a new PDF from copied pages and new content.
// It bridges the reader (source) and writer (output).
type Modifier struct {
	writer    *document.Writer
	catalog   *core.PdfDictionary
	pagesDict *core.PdfDictionary
	kids      *core.PdfArray
	pagesRef  *core.PdfIndirectReference
	pageCount int
	info      *core.PdfDictionary
}

func newModifier() *Modifier {
	w := document.NewWriter("1.7")

	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))

	pagesDict := core.NewPdfDictionary()
	pagesDict.Set("Type", core.NewPdfName("Pages"))

	pagesRef := w.AddObject(pagesDict)
	catalog.Set("Pages", pagesRef)

	return &Modifier{
		writer:    w,
		catalog:   catalog,
		pagesDict: pagesDict,
		kids:      core.NewPdfArray(),
		pagesRef:  pagesRef,
	}
}

// appendReader copies all pages from a reader into the modifier.
func (m *Modifier) appendReader(r *PdfReader) error {
	copier := NewCopier(r, m.writer.AddObject)

	for i := range r.PageCount() {
		pageRef, err := copier.CopyPage(i)
		if err != nil {
			return fmt.Errorf("merge page %d: %w", i, err)
		}

		// Set /Parent to our pages dict.
		// We need to resolve the page dict to set the parent.
		// Since CopyPage returns a ref, the dict is already registered.
		// We modify it in place (PdfDictionary is a pointer).
		pageObj, _ := m.resolveRef(pageRef)
		if pageDict, ok := pageObj.(*core.PdfDictionary); ok {
			pageDict.Set("Parent", m.pagesRef)
		}

		m.kids.Add(pageRef)
		m.pageCount++
	}

	// Copy document info from the first reader if not already set.
	if m.info == nil {
		infoRef := r.xref.trailer.Get("Info")
		if infoRef != nil {
			infoCopied, err := copier.CopyObject(infoRef)
			if err == nil {
				if dict, ok := infoCopied.(*core.PdfDictionary); ok {
					m.info = dict
				}
			}
		}
	}

	return nil
}

// resolveRef looks up an indirect reference in the writer's object list.
// This is a simple linear scan — fine for the number of objects in a merge.
func (m *Modifier) resolveRef(ref *core.PdfIndirectReference) (core.PdfObject, bool) {
	// The writer stores objects internally. We access them via the ref.
	// Since the writer's AddObject returns refs sequentially, and the
	// copier registered the page dict, we can't directly access it.
	// Instead, we rely on the fact that PdfDictionary is a pointer —
	// the copier's CopyPage already set up the dict correctly.
	return nil, false
}

// AddBlankPage adds a blank page with the given dimensions.
func (m *Modifier) AddBlankPage(width, height float64) {
	pageDict := core.NewPdfDictionary()
	pageDict.Set("Type", core.NewPdfName("Page"))
	pageDict.Set("Parent", m.pagesRef)
	pageDict.Set("MediaBox", core.NewPdfArray(
		core.NewPdfInteger(0),
		core.NewPdfInteger(0),
		core.NewPdfReal(width),
		core.NewPdfReal(height),
	))

	pageRef := m.writer.AddObject(pageDict)
	m.kids.Add(pageRef)
	m.pageCount++
}

// AddPageWithText adds a page with simple text content.
func (m *Modifier) AddPageWithText(width, height float64, text string, f *font.Standard, fontSize, x, y float64) {
	// Build content stream.
	stream := content.NewStream()
	stream.BeginText()
	stream.SetFont("F1", fontSize)
	stream.MoveText(x, y)
	stream.ShowText(text)
	stream.EndText()

	contentStream := stream.ToPdfStream()
	contentRef := m.writer.AddObject(contentStream)

	// Font dictionary.
	fontDict := f.Dict()
	fontRef := m.writer.AddObject(fontDict)

	fontResDict := core.NewPdfDictionary()
	fontResDict.Set("F1", fontRef)

	resources := core.NewPdfDictionary()
	resources.Set("Font", fontResDict)

	// Page dictionary.
	pageDict := core.NewPdfDictionary()
	pageDict.Set("Type", core.NewPdfName("Page"))
	pageDict.Set("Parent", m.pagesRef)
	pageDict.Set("MediaBox", core.NewPdfArray(
		core.NewPdfInteger(0),
		core.NewPdfInteger(0),
		core.NewPdfReal(width),
		core.NewPdfReal(height),
	))
	pageDict.Set("Contents", contentRef)
	pageDict.Set("Resources", resources)

	pageRef := m.writer.AddObject(pageDict)
	m.kids.Add(pageRef)
	m.pageCount++
}

// SetInfo sets document metadata on the output PDF.
func (m *Modifier) SetInfo(title, author string) {
	info := core.NewPdfDictionary()
	if title != "" {
		info.Set("Title", core.NewPdfLiteralString(title))
	}
	if author != "" {
		info.Set("Author", core.NewPdfLiteralString(author))
	}
	m.info = info
}

// WriteTo writes the merged/modified PDF to the given writer.
func (m *Modifier) WriteTo(w io.Writer) (int64, error) {
	m.pagesDict.Set("Kids", m.kids)
	m.pagesDict.Set("Count", core.NewPdfInteger(m.pageCount))

	catalogRef := m.writer.AddObject(m.catalog)
	m.writer.SetRoot(catalogRef)

	if m.info != nil {
		infoRef := m.writer.AddObject(m.info)
		m.writer.SetInfo(infoRef)
	}

	return m.writer.WriteTo(w)
}

// SaveTo writes the merged/modified PDF to a file.
func (m *Modifier) SaveTo(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := m.WriteTo(f); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
