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
		r, err := Load(path)
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
	pageDicts []*core.PdfDictionary // parallel to kids.Elements
	info      *core.PdfDictionary
}

// newModifier creates an empty Modifier with a fresh PDF 1.7 writer,
// catalog, and page tree root.
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
		page, err := r.Page(i)
		if err != nil {
			return fmt.Errorf("merge page %d: %w", i, err)
		}

		// Deep-copy the page dictionary.
		copied, err := copier.CopyObject(page.pageDict)
		if err != nil {
			return fmt.Errorf("merge page %d: %w", i, err)
		}

		copiedDict, ok := copied.(*core.PdfDictionary)
		if !ok {
			return fmt.Errorf("merge page %d: copied object is not a dictionary", i)
		}

		// Remove /Parent from source, set our own.
		removeEntry(copiedDict, "Parent")
		copiedDict.Set("Parent", m.pagesRef)

		pageRef := m.writer.AddObject(copiedDict)
		m.kids.Add(pageRef)
		m.pageDicts = append(m.pageDicts, copiedDict)
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
	m.pageDicts = append(m.pageDicts, pageDict)
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
	m.pageDicts = append(m.pageDicts, pageDict)
	m.pageCount++
}

// PageCount returns the number of pages in the modifier.
func (m *Modifier) PageCount() int {
	return m.pageCount
}

// RemovePage removes the page at the given 0-based index.
func (m *Modifier) RemovePage(index int) error {
	if index < 0 || index >= m.pageCount {
		return fmt.Errorf("page index %d out of range [0, %d)", index, m.pageCount)
	}
	m.kids.Elements = append(m.kids.Elements[:index], m.kids.Elements[index+1:]...)
	m.pageDicts = append(m.pageDicts[:index], m.pageDicts[index+1:]...)
	m.pageCount--
	return nil
}

// RotatePage sets the rotation of the page at the given index.
// Degrees must be a multiple of 90 (0, 90, 180, 270).
func (m *Modifier) RotatePage(index int, degrees int) error {
	if index < 0 || index >= m.pageCount {
		return fmt.Errorf("page index %d out of range [0, %d)", index, m.pageCount)
	}
	if degrees%90 != 0 {
		return fmt.Errorf("rotation must be a multiple of 90, got %d", degrees)
	}
	if m.pageDicts[index] == nil {
		return fmt.Errorf("page %d has no accessible dictionary", index)
	}
	m.pageDicts[index].Set("Rotate", core.NewPdfInteger(degrees))
	return nil
}

// ReorderPages rearranges pages according to the given order.
// order must be a permutation of [0, pageCount).
func (m *Modifier) ReorderPages(order []int) error {
	if len(order) != m.pageCount {
		return fmt.Errorf("order length %d does not match page count %d", len(order), m.pageCount)
	}
	seen := make(map[int]bool, m.pageCount)
	for _, idx := range order {
		if idx < 0 || idx >= m.pageCount {
			return fmt.Errorf("invalid page index %d in order", idx)
		}
		if seen[idx] {
			return fmt.Errorf("duplicate page index %d in order", idx)
		}
		seen[idx] = true
	}
	newKids := make([]core.PdfObject, m.pageCount)
	newDicts := make([]*core.PdfDictionary, m.pageCount)
	for i, idx := range order {
		newKids[i] = m.kids.Elements[idx]
		newDicts[i] = m.pageDicts[idx]
	}
	m.kids.Elements = newKids
	m.pageDicts = newDicts
	return nil
}

// CropPage sets the CropBox on the page at the given index.
func (m *Modifier) CropPage(index int, rect [4]float64) error {
	if index < 0 || index >= m.pageCount {
		return fmt.Errorf("page index %d out of range [0, %d)", index, m.pageCount)
	}
	if m.pageDicts[index] == nil {
		return fmt.Errorf("page %d has no accessible dictionary", index)
	}
	m.pageDicts[index].Set("CropBox", core.NewPdfArray(
		core.NewPdfReal(rect[0]), core.NewPdfReal(rect[1]),
		core.NewPdfReal(rect[2]), core.NewPdfReal(rect[3]),
	))
	return nil
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
		_ = f.Close()
		return err
	}
	return f.Close()
}
