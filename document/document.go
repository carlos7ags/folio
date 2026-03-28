// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/font"
	"github.com/carlos7ags/folio/layout"
)

// PageContext provides page information to header/footer decorators.
type PageContext struct {
	PageIndex  int // 0-based page index
	TotalPages int // total number of pages in the document
}

// PageDecorator is a callback that draws content on every page.
// It receives the page context and a Page to draw on.
// Decorators run after layout, so TotalPages is accurate.
type PageDecorator func(ctx PageContext, page *Page)

// ElementDecorator is a callback that returns a layout element for each page.
// Unlike PageDecorator (which draws at absolute coordinates and does not
// reserve space), ElementDecorator automatically measures the element's height
// and increases the corresponding margin (top for headers, bottom for footers)
// so content never overlaps the header/footer.
//
// Use ElementDecorator (via SetHeaderElement/SetFooterElement) when you want
// the layout engine to account for the header/footer height. Use PageDecorator
// (via SetHeader/SetFooter) when you need full control over absolute positioning.
//
// The returned element is laid out and rendered on each page. Return nil to
// skip a particular page.
type ElementDecorator func(ctx PageContext) layout.Element

// NamedDest is a named destination within the document.
type NamedDest struct {
	Name      string // destination name
	PageIndex int    // 0-based page index
	// Fit type: "Fit" (fit page), "FitH" (fit width at y), "XYZ" (position + zoom)
	FitType string
	Top     float64 // y position for FitH/XYZ
	Left    float64 // x position for XYZ
	Zoom    float64 // zoom level for XYZ (0 = unchanged)
}

// absoluteElement is a layout element placed at fixed coordinates.
type absoluteElement struct {
	elem         layout.Element
	x, y         float64
	width        float64
	pageIndex    int // -1 = last page
	rightAligned bool
	zIndex       int
}

// Document is the top-level API for building a PDF.
type Document struct {
	pages            []*Page
	pageSize         PageSize
	margins          layout.Margins
	firstMargins     *layout.Margins             // @page :first
	leftMargins      *layout.Margins             // @page :left
	rightMargins     *layout.Margins             // @page :right
	marginBoxes      map[string]layout.MarginBox // default margin boxes
	firstMarginBoxes map[string]layout.MarginBox // first-page margin boxes
	elements         []layout.Element
	absolutes        []absoluteElement
	Info             Info        // document metadata (Title, Author, etc.)
	outlines         []Outline   // bookmarks / outline tree
	namedDests       []NamedDest // named destinations
	header           PageDecorator
	footer           PageDecorator
	headerElem       ElementDecorator
	footerElem       ElementDecorator
	watermark        *WatermarkConfig
	encryption       *EncryptionConfig
	tagged           bool        // if true, produce tagged PDF with structure tree
	pdfA             *PdfAConfig // if non-nil, produce PDF/A conformant output
	acroForm         interface {
		Build(func(core.PdfObject) *core.PdfIndirectReference, []*core.PdfIndirectReference) (*core.PdfIndirectReference, map[int][]*core.PdfIndirectReference)
	}
	viewerPrefs   *ViewerPreferences
	pageLabels    []PageLabelRange
	autoBookmarks bool // if true, generate outlines from layout headings
	attachments   []FileAttachment
}

// NewDocument creates a new PDF document with the given page size.
func NewDocument(ps PageSize) *Document {
	return &Document{
		pageSize: ps,
		margins:  layout.Margins{Top: 72, Right: 72, Bottom: 72, Left: 72},
	}
}

// SetTagged enables tagged PDF output (PDF/UA foundation).
// When enabled, the document includes a structure tree with semantic tags
// (P, H1-H6, Table, Figure, etc.) and marked content operators in the
// content streams. This enables screen readers, text extraction, and
// accessibility compliance (Section 508, EN 301 549).
func (d *Document) SetTagged(enabled bool) {
	d.tagged = enabled
}

// SetAcroForm attaches an interactive form to the document.
// The form object must implement the Build method (typically *forms.AcroForm).
func (d *Document) SetAcroForm(form interface {
	Build(func(core.PdfObject) *core.PdfIndirectReference, []*core.PdfIndirectReference) (*core.PdfIndirectReference, map[int][]*core.PdfIndirectReference)
}) {
	d.acroForm = form
}

// SetAutoBookmarks enables automatic bookmark/outline generation from
// layout headings (H1-H6). When enabled, each Heading element in the
// document flow produces a bookmark entry. Headings are nested by level:
// H2 under H1, H3 under H2, etc.
func (d *Document) SetAutoBookmarks(enabled bool) {
	d.autoBookmarks = enabled
}

// SetMargins sets the page margins used by the layout engine (in PDF points).
// Default is 72pt (1 inch) on all sides.
func (d *Document) SetMargins(m layout.Margins) {
	d.margins = m
}

// SetFirstMargins sets margins for the first page only (@page :first).
func (d *Document) SetFirstMargins(m layout.Margins) {
	d.firstMargins = &m
}

// SetLeftMargins sets margins for left (even) pages (@page :left).
func (d *Document) SetLeftMargins(m layout.Margins) {
	d.leftMargins = &m
}

// SetRightMargins sets margins for right (odd) pages (@page :right).
func (d *Document) SetRightMargins(m layout.Margins) {
	d.rightMargins = &m
}

// SetMarginBoxes sets default margin box content for all pages.
func (d *Document) SetMarginBoxes(boxes map[string]layout.MarginBox) {
	d.marginBoxes = boxes
}

// SetFirstMarginBoxes sets margin box content for the first page only.
func (d *Document) SetFirstMarginBoxes(boxes map[string]layout.MarginBox) {
	d.firstMarginBoxes = boxes
}

// SetHeader sets a decorator that draws on every page (e.g. title, logo).
// The decorator runs after layout, so PageContext.TotalPages is accurate.
func (d *Document) SetHeader(fn PageDecorator) {
	d.header = fn
}

// SetFooter sets a decorator that draws on every page (e.g. page numbers).
// The decorator runs after layout, so PageContext.TotalPages is accurate.
func (d *Document) SetFooter(fn PageDecorator) {
	d.footer = fn
}

// SetHeaderElement sets an element-based header that automatically reserves
// space. The element is measured once to determine its height, and the top
// margin is increased accordingly so content never overlaps the header.
//
// The decorator is called for each page with the page context. Return nil
// to skip a page. For per-page variation, use ctx.PageIndex.
//
//	doc.SetHeaderElement(func(ctx document.PageContext) layout.Element {
//	    return layout.NewParagraph("My Report", font.HelveticaBold, 12)
//	})
func (d *Document) SetHeaderElement(fn ElementDecorator) {
	d.headerElem = fn
}

// SetFooterElement sets an element-based footer that automatically reserves
// space at the bottom of each page.
//
//	doc.SetFooterElement(func(ctx document.PageContext) layout.Element {
//	    text := fmt.Sprintf("Page %d of %d", ctx.PageIndex+1, ctx.TotalPages)
//	    return layout.NewParagraph(text, font.Helvetica, 9).
//	        SetAlign(layout.AlignCenter)
//	})
func (d *Document) SetFooterElement(fn ElementDecorator) {
	d.footerElem = fn
}

// SetHeaderText is a convenience that sets a simple text header on every page.
// The text is drawn with the given font, size, and alignment. The header
// automatically reserves space so content doesn't overlap.
//
// The text may contain {page} and {pages} placeholders that are replaced
// with the current page number and total page count.
func (d *Document) SetHeaderText(text string, f *font.Standard, size float64, align layout.Align) {
	d.headerElem = func(ctx PageContext) layout.Element {
		s := replacePagePlaceholders(text, ctx)
		return layout.NewParagraph(s, f, size).SetAlign(align)
	}
}

// SetFooterText is a convenience that sets a simple text footer on every page.
// See SetHeaderText for placeholder support.
func (d *Document) SetFooterText(text string, f *font.Standard, size float64, align layout.Align) {
	d.footerElem = func(ctx PageContext) layout.Element {
		s := replacePagePlaceholders(text, ctx)
		return layout.NewParagraph(s, f, size).SetAlign(align)
	}
}

// replacePagePlaceholders replaces {page} and {pages} in text.
func replacePagePlaceholders(text string, ctx PageContext) string {
	s := text
	s = strings.ReplaceAll(s, "{page}", strconv.Itoa(ctx.PageIndex+1))
	s = strings.ReplaceAll(s, "{pages}", strconv.Itoa(ctx.TotalPages))
	return s
}

// AddNamedDest registers a named destination within the document.
// Named destinations can be used as targets for internal links.
func (d *Document) AddNamedDest(dest NamedDest) {
	if dest.FitType == "" {
		dest.FitType = "Fit"
	}
	d.namedDests = append(d.namedDests, dest)
}

// Add appends a layout element (e.g. Paragraph) to the document.
// Elements are laid out automatically with word wrapping and page breaks
// when WriteTo/Save is called.
func (d *Document) Add(e layout.Element) {
	d.elements = append(d.elements, e)
}

// AddAbsolute places a layout element at fixed (x, y) coordinates on the
// last page produced by the layout engine. The element does not participate
// in normal vertical flow. Coordinates are in PDF points from the bottom-left.
// Width sets the layout width for word-wrapping; 0 uses full content width.
func (d *Document) AddAbsolute(e layout.Element, x, y, width float64) {
	d.absolutes = append(d.absolutes, absoluteElement{
		elem: e, x: x, y: y, width: width, pageIndex: -1,
	})
}

// AddAbsoluteRight places a layout element whose right edge is at
// (pageWidth - rightOffset) on the last page. The X coordinate is adjusted
// after layout to account for the element's actual width.
func (d *Document) AddAbsoluteRight(e layout.Element, rightOffset, y, width float64) {
	d.absolutes = append(d.absolutes, absoluteElement{
		elem: e, x: rightOffset, y: y, width: width, pageIndex: -1, rightAligned: true,
	})
}

// AddAbsoluteWithOpts places an element with full positioning control.
func (d *Document) AddAbsoluteWithOpts(e layout.Element, x, y, width float64, opts layout.AbsoluteOpts) {
	d.absolutes = append(d.absolutes, absoluteElement{
		elem: e, x: x, y: y, width: width,
		pageIndex: opts.PageIndex, rightAligned: opts.RightAligned, zIndex: opts.ZIndex,
	})
}

// AddAbsoluteOnPage places a layout element at (x, y) on a specific
// layout page (0-indexed). If the page doesn't exist, the element is ignored.
func (d *Document) AddAbsoluteOnPage(e layout.Element, x, y, width float64, pageIndex int) {
	d.absolutes = append(d.absolutes, absoluteElement{
		elem: e, x: x, y: y, width: width, pageIndex: pageIndex,
	})
}

// AddPage adds a blank page to the document and returns it.
func (d *Document) AddPage() *Page {
	p := &Page{size: d.pageSize}
	d.pages = append(d.pages, p)
	return p
}

// PageCount returns the number of pages in the document.
func (d *Document) PageCount() int {
	return len(d.pages)
}

// Page returns the page at the given zero-based index.
// Returns an error if the index is out of range.
func (d *Document) Page(index int) (*Page, error) {
	if index < 0 || index >= len(d.pages) {
		return nil, fmt.Errorf("page index %d out of range [0, %d)", index, len(d.pages))
	}
	return d.pages[index], nil
}

// RemovePage removes the page at the given zero-based index.
// Returns an error if the index is out of range.
func (d *Document) RemovePage(index int) error {
	if index < 0 || index >= len(d.pages) {
		return fmt.Errorf("page index %d out of range [0, %d)", index, len(d.pages))
	}
	d.pages = slices.Delete(d.pages, index, index+1)
	return nil
}

// buildAllPages returns all pages: manually created pages first,
// then any pages generated by the layout engine from elements.
// If tagging is enabled, structTags is populated with the structure tags.
func (d *Document) buildAllPages() (all []*Page, structTags []layout.StructTagInfo) {
	// Start with manually created pages.
	all = make([]*Page, len(d.pages))
	copy(all, d.pages)
	manualPageCount := len(all)

	// Measure element-based header/footer heights and adjust margins
	// so the layout engine reserves space for them automatically.
	margins := d.margins
	var headerHeight, footerHeight float64
	if d.headerElem != nil {
		headerHeight = d.measureElementDecorator(d.headerElem)
		margins.Top += headerHeight
	}
	if d.footerElem != nil {
		footerHeight = d.measureElementDecorator(d.footerElem)
		margins.Bottom += footerHeight
	}

	// Run layout engine if there are elements.
	if len(d.elements) > 0 || len(d.absolutes) > 0 {
		r := layout.NewRenderer(d.pageSize.Width, d.pageSize.Height, margins)
		if d.firstMargins != nil {
			r.SetFirstMargins(*d.firstMargins)
		}
		if d.leftMargins != nil {
			r.SetLeftMargins(*d.leftMargins)
		}
		if d.rightMargins != nil {
			r.SetRightMargins(*d.rightMargins)
		}
		if d.marginBoxes != nil {
			r.SetMarginBoxes(d.marginBoxes)
		}
		if d.firstMarginBoxes != nil {
			r.SetFirstMarginBoxes(d.firstMarginBoxes)
		}
		if d.tagged {
			r.SetTagged(true)
		}
		for _, e := range d.elements {
			r.Add(e)
		}
		for _, a := range d.absolutes {
			r.AddAbsoluteWithOpts(a.elem, a.x, a.y, a.width, layout.AbsoluteOpts{
				RightAligned: a.rightAligned,
				ZIndex:       a.zIndex,
				PageIndex:    a.pageIndex,
			})
		}
		results := r.Render()
		for _, res := range results {
			ps := d.pageSize
			if res.PageHeight > 0 {
				ps.Height = res.PageHeight
			}
			p := &Page{size: ps, stream: res.Stream}
			for _, f := range res.Fonts {
				p.fonts = append(p.fonts, fontResource{
					name:     f.Name,
					standard: f.Standard,
					embedded: f.Embedded,
				})
			}
			for _, im := range res.Images {
				p.images = append(p.images, imageResource{
					name:  im.Name,
					image: im.Image,
				})
			}
			for _, gs := range res.ExtGStates {
				gsDict := core.NewPdfDictionary()
				gsDict.Set("Type", core.NewPdfName("ExtGState"))
				gsDict.Set("ca", core.NewPdfReal(gs.Opacity))
				gsDict.Set("CA", core.NewPdfReal(gs.Opacity))
				p.extGStates = append(p.extGStates, extGStateResource{
					name: gs.Name,
					dict: gsDict,
				})
			}
			// Convert layout link areas into PDF link annotations.
			for _, link := range res.Links {
				ann := Annotation{
					subtype:  "Link",
					rect:     [4]float64{link.X, link.Y, link.X + link.W, link.Y + link.H},
					uri:      link.URI,
					dest:     link.DestName,
					destPage: -1,
				}
				p.annotations = append(p.annotations, ann)
			}
			all = append(all, p)
		}

		// Adjust struct tag page indices to account for manual pages.
		if d.tagged {
			tags := r.StructTags()
			for i := range tags {
				tags[i].PageIndex += manualPageCount
			}
			structTags = tags
		}

		// Auto-generate bookmarks from layout headings.
		if d.autoBookmarks && len(d.outlines) == 0 {
			d.outlines = buildAutoBookmarks(results, manualPageCount)
		}
	}

	// Apply header/footer decorators to all pages.
	if d.header != nil || d.footer != nil {
		total := len(all)
		for i, p := range all {
			ctx := PageContext{PageIndex: i, TotalPages: total}
			if d.header != nil {
				d.header(ctx, p)
			}
			if d.footer != nil {
				d.footer(ctx, p)
			}
		}
	}

	// Apply element-based header/footer decorators to all pages.
	if d.headerElem != nil || d.footerElem != nil {
		total := len(all)
		for i, p := range all {
			ctx := PageContext{PageIndex: i, TotalPages: total}
			ps := p.effectiveSize()
			if d.headerElem != nil {
				d.renderElementDecorator(d.headerElem, ctx, p, d.margins.Left, ps.Height-d.margins.Top, headerHeight)
			}
			if d.footerElem != nil {
				d.renderElementDecorator(d.footerElem, ctx, p, d.margins.Left, d.margins.Bottom+footerHeight, footerHeight)
			}
		}
	}

	// Apply watermark to all pages (drawn before page content).
	if d.watermark != nil {
		for _, p := range all {
			d.applyWatermark(p)
		}
	}

	return all, structTags
}

// buildAutoBookmarks generates an outline tree from heading info in rendered pages.
// Headings are nested by level: H2 under the preceding H1, H3 under H2, etc.
func buildAutoBookmarks(results []layout.PageResult, pageOffset int) []Outline {
	var outlines []Outline
	// Stack tracks the current nesting: stack[0] = H1 parent, stack[1] = H2, etc.
	type stackEntry struct {
		level   int
		outline *Outline
	}
	var stack []stackEntry

	for pageIdx, res := range results {
		actualPage := pageOffset + pageIdx
		for _, h := range res.Headings {
			dest := XYZDest(actualPage, 0, h.Y, 0)
			entry := Outline{Title: h.Text, Dest: dest}

			// Pop stack entries that are at the same or deeper level.
			for len(stack) > 0 && stack[len(stack)-1].level >= h.Level {
				stack = stack[:len(stack)-1]
			}

			if len(stack) == 0 {
				// Top-level heading.
				outlines = append(outlines, entry)
				stack = append(stack, stackEntry{
					level:   h.Level,
					outline: &outlines[len(outlines)-1],
				})
			} else {
				// Nested under the current parent.
				parent := stack[len(stack)-1].outline
				parent.Children = append(parent.Children, entry)
				stack = append(stack, stackEntry{
					level:   h.Level,
					outline: &parent.Children[len(parent.Children)-1],
				})
			}
		}
	}

	return outlines
}

// Save writes the document to a file at the given path.
func (d *Document) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err = d.WriteTo(f); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

// WriteTo serializes the complete PDF document to w.
func (d *Document) WriteTo(w io.Writer) (int64, error) {
	allPages, structTags := d.buildAllPages()

	// Second pass: replace ##TOTAL_PAGES## placeholder with actual count.
	totalStr := strconv.Itoa(len(allPages))
	for _, p := range allPages {
		if p.stream != nil {
			p.stream.ReplaceInBytes("##TOTAL_PAGES##", totalStr)
		}
	}

	version := "1.7"
	if d.pdfA != nil {
		version = pdfVersionForPdfA(d.pdfA.Level)
	}
	writer := NewWriter(version)

	catalog := core.NewPdfDictionary()
	catalog.Set("Type", core.NewPdfName("Catalog"))

	pagesDict := core.NewPdfDictionary()
	pagesDict.Set("Type", core.NewPdfName("Pages"))
	pagesDict.Set("Count", core.NewPdfInteger(len(allPages)))

	catalogRef := writer.AddObject(catalog)
	pagesRef := writer.AddObject(pagesDict)
	catalog.Set("Pages", pagesRef)

	kids := core.NewPdfArray()
	pageRefs := make([]*core.PdfIndirectReference, 0, len(allPages))
	pageDicts := make([]*core.PdfDictionary, 0, len(allPages))

	// First pass: build page objects.
	for pageIdx, page := range allPages {
		pageDict := core.NewPdfDictionary()
		pageDict.Set("Type", core.NewPdfName("Page"))
		pageDict.Set("Parent", pagesRef)

		// Tagged PDF: set StructParents for the ParentTree mapping.
		if d.tagged && len(structTags) > 0 {
			pageDict.Set("StructParents", core.NewPdfInteger(pageIdx))
		}
		ps := page.effectiveSize()
		pageDict.Set("MediaBox", core.NewPdfArray(
			core.NewPdfInteger(0),
			core.NewPdfInteger(0),
			core.NewPdfReal(ps.Width),
			core.NewPdfReal(ps.Height),
		))

		// Optional page geometry boxes.
		if page.cropBox != nil {
			pageDict.Set("CropBox", boxToArray(page.cropBox))
		}
		if page.bleedBox != nil {
			pageDict.Set("BleedBox", boxToArray(page.bleedBox))
		}
		if page.trimBox != nil {
			pageDict.Set("TrimBox", boxToArray(page.trimBox))
		}
		if page.artBox != nil {
			pageDict.Set("ArtBox", boxToArray(page.artBox))
		}

		// Build /Resources dictionary.
		resources := core.NewPdfDictionary()
		if len(page.fonts) > 0 {
			fontDict := core.NewPdfDictionary()
			for _, entry := range page.fonts {
				if entry.standard != nil {
					fontObjRef := writer.AddObject(entry.standard.Dict())
					fontDict.Set(entry.name, fontObjRef)
				} else if entry.embedded != nil {
					type0 := entry.embedded.BuildObjects(writer.AddObject)
					fontObjRef := writer.AddObject(type0)
					fontDict.Set(entry.name, fontObjRef)
				}
			}
			resources.Set("Font", fontDict)
		}
		if len(page.images) > 0 || len(page.formXObjects) > 0 {
			xobjDict := core.NewPdfDictionary()
			for _, entry := range page.images {
				imgRef, _ := entry.image.BuildXObject(writer.AddObject)
				xobjDict.Set(entry.name, imgRef)
			}
			for _, entry := range page.formXObjects {
				// Hoist nested PdfStream objects (font programs, images, etc.)
				// to indirect objects. PDF streams cannot appear as direct
				// objects inside a dictionary — they must be indirect refs.
				hoistStreams(entry.stream.Dict, writer.AddObject)
				formRef := writer.AddObject(entry.stream)
				xobjDict.Set(entry.name, formRef)
			}
			resources.Set("XObject", xobjDict)
		}
		if len(page.extGStates) > 0 {
			gsDict := core.NewPdfDictionary()
			for _, gs := range page.extGStates {
				gsRef := writer.AddObject(gs.dict)
				gsDict.Set(gs.name, gsRef)
			}
			resources.Set("ExtGState", gsDict)
		}
		pageDict.Set("Resources", resources)

		if page.rotate != 0 {
			pageDict.Set("Rotate", core.NewPdfInteger(page.rotate))
		}

		if page.stream != nil {
			contentStream := page.stream.ToPdfStream()
			contentRef := writer.AddObject(contentStream)
			pageDict.Set("Contents", contentRef)
		}

		pageRef := writer.AddObject(pageDict)
		pageRefs = append(pageRefs, pageRef)
		pageDicts = append(pageDicts, pageDict)
		kids.Add(pageRef)
	}
	pagesDict.Set("Kids", kids)

	// Second pass: annotations (need all pageRefs to exist for page links).
	for i, page := range allPages {
		if len(page.annotations) == 0 {
			continue
		}
		annots := core.NewPdfArray()
		for _, ann := range page.annotations {
			annotDict := core.NewPdfDictionary()
			annotDict.Set("Type", core.NewPdfName("Annot"))
			annotDict.Set("Subtype", core.NewPdfName(ann.subtype))
			annotDict.Set("Rect", core.NewPdfArray(
				core.NewPdfReal(ann.rect[0]),
				core.NewPdfReal(ann.rect[1]),
				core.NewPdfReal(ann.rect[2]),
				core.NewPdfReal(ann.rect[3]),
			))
			annotDict.Set("Border", core.NewPdfArray(
				core.NewPdfReal(ann.border[0]),
				core.NewPdfReal(ann.border[1]),
				core.NewPdfReal(ann.border[2]),
			))

			// Annotation color (/C array).
			if ann.color != nil {
				annotDict.Set("C", core.NewPdfArray(
					core.NewPdfReal(ann.color[0]),
					core.NewPdfReal(ann.color[1]),
					core.NewPdfReal(ann.color[2]),
				))
			}

			switch ann.subtype {
			case "Link":
				if ann.uri != "" {
					action := core.NewPdfDictionary()
					action.Set("Type", core.NewPdfName("Action"))
					action.Set("S", core.NewPdfName("URI"))
					action.Set("URI", core.NewPdfLiteralString(ann.uri))
					annotDict.Set("A", action)
				} else if ann.dest != "" {
					// Resolve named destination to a direct page reference
					// for maximum viewer compatibility (macOS Preview does
					// not reliably follow string-based GoTo destinations).
					resolved := false
					for _, nd := range d.namedDests {
						if nd.Name == ann.dest && nd.PageIndex >= 0 && nd.PageIndex < len(pageRefs) {
							annotDict.Set("Dest", core.NewPdfArray(
								pageRefs[nd.PageIndex],
								core.NewPdfName("Fit"),
							))
							resolved = true
							break
						}
					}
					if !resolved {
						action := core.NewPdfDictionary()
						action.Set("Type", core.NewPdfName("Action"))
						action.Set("S", core.NewPdfName("GoTo"))
						action.Set("D", core.NewPdfLiteralString(ann.dest))
						annotDict.Set("A", action)
					}
				} else if ann.destPage >= 0 && ann.destPage < len(pageRefs) {
					annotDict.Set("Dest", core.NewPdfArray(
						pageRefs[ann.destPage],
						core.NewPdfName("Fit"),
					))
				}

			case "Text":
				// Sticky note annotation (ISO 32000 §12.5.6.4).
				if ann.contents != "" {
					annotDict.Set("Contents", core.NewPdfLiteralString(ann.contents))
				}
				if ann.name != "" {
					annotDict.Set("Name", core.NewPdfName(ann.name))
				}
				if ann.open {
					annotDict.Set("Open", core.NewPdfBoolean(true))
				}

			case "Highlight", "Underline", "Squiggly", "StrikeOut":
				// Text markup annotations (ISO 32000 §12.5.6.10).
				if ann.contents != "" {
					annotDict.Set("Contents", core.NewPdfLiteralString(ann.contents))
				}
				// QuadPoints: required for text markup annotations.
				qp := ann.quadPoints
				if len(qp) == 0 {
					// Default: single quad matching the rect.
					qp = [][8]float64{{
						ann.rect[0], ann.rect[1],
						ann.rect[2], ann.rect[1],
						ann.rect[0], ann.rect[3],
						ann.rect[2], ann.rect[3],
					}}
				}
				qpArray := core.NewPdfArray()
				for _, quad := range qp {
					for _, v := range quad {
						qpArray.Add(core.NewPdfReal(v))
					}
				}
				annotDict.Set("QuadPoints", qpArray)
			}

			annotRef := writer.AddObject(annotDict)
			annots.Add(annotRef)
		}
		// PdfDictionary is a pointer — mutation is reflected in the registered object.
		pageDicts[i].Set("Annots", annots)
	}

	// Named destinations.
	if len(d.namedDests) > 0 {
		destsDict := core.NewPdfDictionary()
		for _, nd := range d.namedDests {
			if nd.PageIndex < 0 || nd.PageIndex >= len(pageRefs) {
				continue
			}
			var destArray *core.PdfArray
			switch nd.FitType {
			case "FitH":
				destArray = core.NewPdfArray(
					pageRefs[nd.PageIndex],
					core.NewPdfName("FitH"),
					core.NewPdfReal(nd.Top),
				)
			case "XYZ":
				destArray = core.NewPdfArray(
					pageRefs[nd.PageIndex],
					core.NewPdfName("XYZ"),
					core.NewPdfReal(nd.Left),
					core.NewPdfReal(nd.Top),
					core.NewPdfReal(nd.Zoom),
				)
			default: // "Fit"
				destArray = core.NewPdfArray(
					pageRefs[nd.PageIndex],
					core.NewPdfName("Fit"),
				)
			}
			destsDict.Set(nd.Name, destArray)
		}
		destsRef := writer.AddObject(destsDict)
		catalog.Set("Dests", destsRef)
	}

	// Outlines / bookmarks.
	if len(d.outlines) > 0 {
		outlinesRef := buildOutlineTree(d.outlines, pageRefs, writer.AddObject)
		if outlinesRef != nil {
			catalog.Set("Outlines", outlinesRef)
		}
	}

	// AcroForm: interactive form fields.
	if d.acroForm != nil {
		formRef, pageWidgets := d.acroForm.Build(writer.AddObject, pageRefs)
		if formRef != nil {
			catalog.Set("AcroForm", formRef)

			// Add widget annotations to each page's /Annots array.
			for pageIdx, widgetRefs := range pageWidgets {
				if pageIdx < len(pageDicts) {
					annots := pageDicts[pageIdx].Get("Annots")
					var annotsArr *core.PdfArray
					if annots != nil {
						if arr, ok := annots.(*core.PdfArray); ok {
							annotsArr = arr
						}
					}
					if annotsArr == nil {
						annotsArr = core.NewPdfArray()
					}
					for _, wRef := range widgetRefs {
						annotsArr.Add(wRef)
					}
					pageDicts[pageIdx].Set("Annots", annotsArr)
				}
			}
		}
	}

	// Tagged PDF: build structure tree with proper nesting.
	if d.tagged && len(structTags) > 0 {
		st := newStructTree()
		// Build nested tree using ParentIndex.
		nodes := make([]*structNode, len(structTags))
		for i, tag := range structTags {
			var node *structNode
			if tag.ParentIndex < 0 || tag.ParentIndex >= i {
				// Top-level element.
				node = st.addElement(StructTag(tag.Tag))
			} else {
				// Nested under parent.
				node = st.addChild(nodes[tag.ParentIndex], StructTag(tag.Tag))
			}
			if tag.AltText != "" {
				node.altText = tag.AltText
			}
			st.markContent(node, tag.PageIndex)
			nodes[i] = node
		}
		stRef := st.buildPdfObjects(pageRefs, writer.AddObject)
		if stRef != nil {
			catalog.Set("StructTreeRoot", stRef)
		}
		// MarkInfo dictionary signals this is a tagged PDF.
		markInfo := core.NewPdfDictionary()
		markInfo.Set("Marked", core.NewPdfBoolean(true))
		catalog.Set("MarkInfo", markInfo)
	}

	// PDF/A: validate, add XMP metadata and output intent.
	if d.pdfA != nil {
		if err := d.validatePdfA(allPages); err != nil {
			return 0, err
		}

		// /ID in the trailer is required for all PDF/A levels (ISO 19005 §6.1.3).
		if err := writer.GenerateFileID(); err != nil {
			return 0, err
		}

		// XMP metadata stream (required for PDF/A).
		xmpRef := buildXMPMetadata(d.Info, d.pdfA.Level, d.pdfA.XMPSchemas, d.pdfA.XMPProperties, writer.AddObject)
		catalog.Set("Metadata", xmpRef)

		// Output intent (required for PDF/A).
		intentRef := buildOutputIntent(d.pdfA, writer.AddObject)
		catalog.Set("OutputIntents", core.NewPdfArray(intentRef))
	}

	// File attachments (PDF/A-3B only; validated in validatePdfA).
	if len(d.attachments) > 0 {
		buildAttachments(d.attachments, catalog, writer.AddObject, d.Info.CreationDate)
	}

	// Viewer preferences.
	buildViewerPreferences(d.viewerPrefs, catalog)

	// Page labels.
	if len(d.pageLabels) > 0 {
		buildPageLabels(d.pageLabels, catalog, writer.AddObject)
	}

	writer.SetRoot(catalogRef)

	// Document metadata.
	if !d.Info.isEmpty() {
		infoRef := writer.AddObject(d.Info.toDict())
		writer.SetInfo(infoRef)
	}

	// Encryption: create encryptor and wire into the writer.
	if d.encryption != nil {
		rev := revisionFromAlgorithm(d.encryption.Algorithm)
		enc, err := core.NewEncryptor(rev,
			d.encryption.UserPassword,
			d.encryption.OwnerPassword,
			d.encryption.Permissions,
		)
		if err != nil {
			return 0, fmt.Errorf("document: encryption setup: %w", err)
		}
		writer.SetEncryption(enc)
	}

	return writer.WriteTo(w)
}

// hoistStreams walks a PdfDictionary tree and replaces any PdfStream
// values with indirect references. PDF streams cannot be direct objects
// inside a dictionary (ISO 32000 §7.3.8) — they must be indirect.
// This is needed for imported page resources where resolveDeep produces
// a fully direct object tree that may contain font programs and other
// streams as direct values.
func hoistStreams(dict *core.PdfDictionary, addObj func(core.PdfObject) *core.PdfIndirectReference) {
	for i, entry := range dict.Entries {
		switch v := entry.Value.(type) {
		case *core.PdfStream:
			// Recursively hoist any streams nested in this stream's dict.
			hoistStreams(v.Dict, addObj)
			dict.Entries[i].Value = addObj(v)
		case *core.PdfDictionary:
			hoistStreams(v, addObj)
		case *core.PdfArray:
			hoistStreamArray(v, addObj)
		}
	}
}

// hoistStreamArray walks a PdfArray and replaces any PdfStream elements
// with indirect references.
func hoistStreamArray(arr *core.PdfArray, addObj func(core.PdfObject) *core.PdfIndirectReference) {
	for i, elem := range arr.Elements {
		switch v := elem.(type) {
		case *core.PdfStream:
			hoistStreams(v.Dict, addObj)
			arr.Elements[i] = addObj(v)
		case *core.PdfDictionary:
			hoistStreams(v, addObj)
		case *core.PdfArray:
			hoistStreamArray(v, addObj)
		}
	}
}

// measureElementDecorator measures the height of an element decorator by
// laying out a sample element in the available content width.
func (d *Document) measureElementDecorator(fn ElementDecorator) float64 {
	// Use a dummy context for measurement.
	elem := fn(PageContext{PageIndex: 0, TotalPages: 1})
	if elem == nil {
		return 0
	}
	contentWidth := d.pageSize.Width - d.margins.Left - d.margins.Right
	plan := elem.PlanLayout(layout.LayoutArea{
		Width:  contentWidth,
		Height: d.pageSize.Height, // generous height for measurement
	})
	return plan.Consumed
}

// renderElementDecorator renders an element decorator on a page at the given
// position. It creates a mini layout renderer, renders the element, and
// merges the resulting content stream and resources into the page.
func (d *Document) renderElementDecorator(fn ElementDecorator, ctx PageContext, p *Page, x, y, height float64) {
	elem := fn(ctx)
	if elem == nil {
		return
	}
	contentWidth := d.pageSize.Width - d.margins.Left - d.margins.Right

	// Use a single-page renderer to lay out the element.
	r := layout.NewRenderer(contentWidth, height, layout.Margins{})
	r.Add(elem)
	results := r.Render()
	if len(results) == 0 {
		return
	}
	res := results[0]

	// Merge the rendered content into the page at the correct position.
	// The renderer produces content at (0, height) origin; we need to
	// translate it to (x, y-height) in page coordinates.
	p.ensureStream()

	// Remap font names to avoid collisions with the page's own fonts.
	// The mini-renderer assigns names like "F1", "F2" which may conflict
	// with fonts already registered on the page.
	nameMap := make(map[string]string) // old name → new name
	for _, f := range res.Fonts {
		newName := fmt.Sprintf("HF%d", len(p.fonts)+1)
		nameMap[f.Name] = newName
		p.fonts = append(p.fonts, fontResource{
			name:     newName,
			standard: f.Standard,
			embedded: f.Embedded,
		})
	}
	for _, im := range res.Images {
		newName := fmt.Sprintf("HI%d", len(p.images)+1)
		nameMap[im.Name] = newName
		p.images = append(p.images, imageResource{
			name:  newName,
			image: im.Image,
		})
	}

	// Rewrite font/image references in the content stream.
	streamStr := string(res.Stream.Bytes())
	for oldName, newName := range nameMap {
		streamStr = strings.ReplaceAll(streamStr, "/"+oldName+" ", "/"+newName+" ")
	}

	// Translate the content stream to the correct page position.
	// The renderer draws from top-left of its area; we offset with cm.
	translated := fmt.Sprintf("q 1 0 0 1 %s %s cm\n%s\nQ\n", formatPt(x), formatPt(y-height), streamStr)
	p.stream.AppendBytes([]byte(translated))
}

// formatPt formats a float for PDF (no trailing zeros, max 4 decimal places).
func formatPt(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	// Trim trailing zeros.
	if strings.Contains(s, ".") {
		s = strings.TrimRight(s, "0")
		s = strings.TrimRight(s, ".")
	}
	return s
}
