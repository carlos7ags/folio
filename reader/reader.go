// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import (
	"fmt"
	"os"

	"github.com/carlos7ags/folio/core"
)

// Strictness controls how the reader handles malformed PDFs.
type Strictness int

const (
	// StrictnessTolerant attempts to recover from common PDF errors.
	// This is the default and handles most real-world PDFs.
	StrictnessTolerant Strictness = iota

	// StrictnessStrict fails immediately on any spec violation.
	StrictnessStrict
)

// PdfReader opens and reads an existing PDF file.
type PdfReader struct {
	data       []byte
	xref       *xrefTable
	resolver   *resolver
	catalog    *core.PdfDictionary
	pages      []*PageInfo
	strictness Strictness
}

// Box represents a PDF rectangle: [x1, y1, x2, y2] in points.
// x1,y1 is the lower-left corner; x2,y2 is the upper-right corner.
type Box struct {
	X1, Y1, X2, Y2 float64
}

// Width returns the box width.
func (b Box) Width() float64 { return b.X2 - b.X1 }

// Height returns the box height.
func (b Box) Height() float64 { return b.Y2 - b.Y1 }

// IsZero reports whether the box is unset (all zeros).
func (b Box) IsZero() bool { return b == Box{} }

// PageInfo holds parsed information about a single page.
type PageInfo struct {
	Number int     // 1-based page number
	Width  float64 // page width in points (from effective visible box)
	Height float64 // page height in points (from effective visible box)
	Rotate int     // rotation in degrees (0, 90, 180, 270)

	// The 5 PDF page geometry boxes (ISO 32000 §14.11.2).
	// MediaBox is required; others are optional and inherit from MediaBox if absent.
	MediaBox Box // page boundaries — the full physical medium
	CropBox  Box // visible region (default = MediaBox)
	BleedBox Box // region for production bleed (default = CropBox)
	TrimBox  Box // intended finished page dimensions (default = CropBox)
	ArtBox   Box // meaningful content area (default = CropBox)

	pageDict           *core.PdfDictionary
	reader             *PdfReader
	inheritedResources core.PdfObject // /Resources inherited from ancestor Pages node
}

// VisibleBox returns the effective visible area of the page.
// This is the CropBox if set, otherwise the MediaBox.
func (p *PageInfo) VisibleBox() Box {
	if !p.CropBox.IsZero() {
		return p.CropBox
	}
	return p.MediaBox
}

// Open reads and parses a PDF file from disk.
func Open(path string) (*PdfReader, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reader: %w", err)
	}
	return Parse(data)
}

// ReadOptions configures the PDF reader.
type ReadOptions struct {
	Strictness   Strictness
	MaxCache     int          // max cached objects (0 = default 10000)
	MemoryLimits MemoryLimits // memory safety limits for decompression
}

// Parse reads and parses a PDF from a byte slice.
func Parse(data []byte) (*PdfReader, error) {
	return ParseWithOptions(data, ReadOptions{})
}

// ParseWithOptions reads and parses a PDF with custom options.
func ParseWithOptions(data []byte, opts ReadOptions) (*PdfReader, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("reader: file too small")
	}

	// Verify PDF header. Tolerant mode accepts offset headers (%PDF- not at byte 0).
	headerOff := findPDFHeader(data)
	if headerOff < 0 {
		return nil, fmt.Errorf("reader: not a PDF file (missing %%PDF- header)")
	}
	// If header is not at byte 0, trim the garbage prefix so xref
	// offsets align correctly.
	if headerOff > 0 {
		data = data[headerOff:]
	}

	// Parse xref table and trailer.
	xref, err := parseXrefTable(data)
	if err != nil {
		// Tolerant mode: try to repair xref by scanning for objects.
		if opts.Strictness == StrictnessTolerant {
			xref, err = repairXref(data)
		}
		if err != nil {
			return nil, err
		}
	}

	mem := newMemoryTracker(opts.MemoryLimits)

	// Validate xref object count.
	maxObj := opts.MemoryLimits.effectiveMaxObjectCount()
	if maxObj >= 0 && len(xref.entries) > maxObj {
		return nil, fmt.Errorf("%w: xref contains %d objects, limit is %d",
			ErrMemoryLimitExceeded, len(xref.entries), maxObj)
	}

	res := newResolver(data, xref, mem)
	if opts.MaxCache > 0 {
		res.SetMaxCache(opts.MaxCache)
	}

	r := &PdfReader{
		data:       data,
		xref:       xref,
		resolver:   res,
		strictness: opts.Strictness,
	}

	// Resolve the document catalog.
	rootRef := xref.trailer.Get("Root")
	if rootRef == nil {
		return nil, fmt.Errorf("reader: trailer has no /Root entry")
	}
	rootObj, err := r.resolver.ResolveDeep(rootRef)
	if err != nil {
		return nil, fmt.Errorf("reader: resolve catalog: %w", err)
	}
	catalog, ok := rootObj.(*core.PdfDictionary)
	if !ok {
		return nil, fmt.Errorf("reader: catalog is not a dictionary")
	}
	r.catalog = catalog

	// Parse the page tree.
	if err := r.parsePageTree(); err != nil {
		return nil, err
	}

	return r, nil
}

// RawBytes returns the raw PDF data that was parsed.
// This is needed for incremental save operations (e.g., digital signing).
func (r *PdfReader) RawBytes() []byte {
	return r.data
}

// Trailer returns the trailer dictionary from the most recent xref section.
func (r *PdfReader) Trailer() *core.PdfDictionary {
	if r.xref == nil {
		return nil
	}
	return r.xref.trailer
}

// MaxObjectNumber returns the highest object number in the xref table.
func (r *PdfReader) MaxObjectNumber() int {
	if r.xref == nil {
		return 0
	}
	maxNum := 0
	for num := range r.xref.entries {
		if num > maxNum {
			maxNum = num
		}
	}
	return maxNum
}

// findPDFHeader locates the %PDF- header, which may not be at byte 0
// (some PDFs have garbage bytes prepended).
func findPDFHeader(data []byte) int {
	searchLen := min(len(data), 1024)
	marker := []byte("%PDF-")
	for i := 0; i <= searchLen-len(marker); i++ {
		if string(data[i:i+len(marker)]) == string(marker) {
			return i
		}
	}
	return -1
}

// repairXref builds an xref table by scanning the file for object definitions.
// This is used as a fallback when the normal xref parsing fails.
func repairXref(data []byte) (*xrefTable, error) {
	table := &xrefTable{entries: make(map[int]xrefEntry)}

	// Scan for "N G obj" patterns.
	tok := NewTokenizer(data)
	for !tok.AtEnd() {
		pos := tok.Pos()
		t1 := tok.Next()
		if t1.Type != TokenNumber || !t1.IsInt || t1.Int < 0 {
			continue
		}
		t2 := tok.Next()
		if t2.Type != TokenNumber || !t2.IsInt {
			continue
		}
		t3 := tok.Next()
		if t3.Type != TokenKeyword && t3.Value != "obj" {
			continue
		}
		// Found "N G obj" at position pos.
		objNum := int(t1.Int)
		genNum := int(t2.Int)
		table.entries[objNum] = xrefEntry{
			offset:     int64(pos),
			generation: genNum,
			inUse:      true,
		}
	}

	if len(table.entries) == 0 {
		return nil, fmt.Errorf("reader: xref repair found no objects")
	}

	// Find trailer by scanning for "trailer" keyword.
	trailerIdx := -1
	for i := len(data) - 1; i > 0; i-- {
		if i+7 <= len(data) && string(data[i:i+7]) == "trailer" {
			trailerIdx = i + 7
			break
		}
	}
	if trailerIdx > 0 {
		rTok := NewTokenizer(data)
		rTok.SetPos(trailerIdx)
		parser := NewParser(rTok)
		obj, err := parser.ParseObject()
		if err == nil {
			if dict, ok := obj.(*core.PdfDictionary); ok {
				table.trailer = dict
			}
		}
	}

	if table.trailer == nil {
		return nil, fmt.Errorf("reader: xref repair could not find trailer")
	}

	return table, nil
}

// Version returns the PDF version from the header (e.g. "1.7").
func (r *PdfReader) Version() string {
	if len(r.data) < 8 {
		return ""
	}
	// %PDF-X.Y
	end := 8
	for end < len(r.data) && end < 12 && r.data[end] != '\n' && r.data[end] != '\r' {
		end++
	}
	return string(r.data[5:end])
}

// PageCount returns the number of pages in the document.
func (r *PdfReader) PageCount() int {
	return len(r.pages)
}

// Page returns information about the i-th page (0-based index).
func (r *PdfReader) Page(index int) (*PageInfo, error) {
	if index < 0 || index >= len(r.pages) {
		return nil, fmt.Errorf("reader: page index %d out of range [0, %d)", index, len(r.pages))
	}
	return r.pages[index], nil
}

// Info returns the document info dictionary values.
func (r *PdfReader) Info() (title, author, subject, creator, producer string) {
	infoRef := r.xref.trailer.Get("Info")
	if infoRef == nil {
		return
	}
	infoObj, err := r.resolver.ResolveDeep(infoRef)
	if err != nil {
		return
	}
	infoDict, ok := infoObj.(*core.PdfDictionary)
	if !ok {
		return
	}

	getString := func(key string) string {
		obj := infoDict.Get(key)
		if obj == nil {
			return ""
		}
		if s, ok := obj.(*core.PdfString); ok {
			return s.Value
		}
		return ""
	}

	title = getString("Title")
	author = getString("Author")
	subject = getString("Subject")
	creator = getString("Creator")
	producer = getString("Producer")
	return
}

// Catalog returns the document catalog dictionary.
func (r *PdfReader) Catalog() *core.PdfDictionary {
	return r.catalog
}

// ResolveObject resolves an indirect reference to its target object.
func (r *PdfReader) ResolveObject(obj core.PdfObject) (core.PdfObject, error) {
	return r.resolver.ResolveDeep(obj)
}

// parsePageTree traverses the page tree and collects all leaf pages.
func (r *PdfReader) parsePageTree() error {
	pagesRef := r.catalog.Get("Pages")
	if pagesRef == nil {
		return fmt.Errorf("reader: catalog has no /Pages entry")
	}
	pagesObj, err := r.resolver.ResolveDeep(pagesRef)
	if err != nil {
		return fmt.Errorf("reader: resolve Pages: %w", err)
	}
	pagesDict, ok := pagesObj.(*core.PdfDictionary)
	if !ok {
		return fmt.Errorf("reader: /Pages is not a dictionary")
	}

	r.pages = nil
	return r.collectPages(pagesDict, inherited{})
}

// inherited carries inheritable page tree attributes down the tree.
// Per ISO 32000 §7.7.3.4, these keys inherit through the page tree:
// MediaBox, CropBox, Resources, Rotate.
type inherited struct {
	mediaBox  *core.PdfArray
	cropBox   *core.PdfArray
	resources core.PdfObject // may be indirect ref — resolved lazily
	rotate    *core.PdfNumber
}

// collectPages recursively collects leaf pages from the page tree.
func (r *PdfReader) collectPages(node *core.PdfDictionary, inh inherited) error {
	// Update inheritable attributes from this node.
	if mb := node.Get("MediaBox"); mb != nil {
		if arr, ok := mb.(*core.PdfArray); ok {
			inh.mediaBox = arr
		}
	}
	if cb := node.Get("CropBox"); cb != nil {
		if arr, ok := cb.(*core.PdfArray); ok {
			inh.cropBox = arr
		}
	}
	if res := node.Get("Resources"); res != nil {
		inh.resources = res
	}
	if rot := node.Get("Rotate"); rot != nil {
		if num, ok := rot.(*core.PdfNumber); ok {
			inh.rotate = num
		}
	}

	typeObj := node.Get("Type")
	if typeObj != nil {
		if name, ok := typeObj.(*core.PdfName); ok && name.Value == "Page" {
			// Leaf page.
			page := r.buildPageInfo(node, inh)
			r.pages = append(r.pages, page)
			return nil
		}
	}

	// Pages node — recurse into /Kids.
	kidsObj := node.Get("Kids")
	if kidsObj == nil {
		return nil
	}
	kidsResolved, err := r.resolver.ResolveDeep(kidsObj)
	if err != nil {
		return fmt.Errorf("reader: resolve Kids: %w", err)
	}
	kids, ok := kidsResolved.(*core.PdfArray)
	if !ok {
		return fmt.Errorf("reader: /Kids is not an array")
	}

	for _, kidRef := range kids.Elements {
		kidObj, err := r.resolver.ResolveDeep(kidRef)
		if err != nil {
			return fmt.Errorf("reader: resolve page kid: %w", err)
		}
		kidDict, ok := kidObj.(*core.PdfDictionary)
		if !ok {
			continue
		}
		if err := r.collectPages(kidDict, inh); err != nil {
			return err
		}
	}

	return nil
}

// buildPageInfo creates a PageInfo from a page dictionary.
// inh carries attributes inherited from ancestor Pages nodes.
func (r *PdfReader) buildPageInfo(pageDict *core.PdfDictionary, inh inherited) *PageInfo {
	page := &PageInfo{
		Number:   len(r.pages) + 1,
		pageDict: pageDict,
		reader:   r,
	}

	// MediaBox: page-level overrides inherited.
	if inh.mediaBox != nil {
		page.MediaBox = arrayToBox(inh.mediaBox)
	}
	if mb := pageDict.Get("MediaBox"); mb != nil {
		if arr, ok := mb.(*core.PdfArray); ok {
			page.MediaBox = arrayToBox(arr)
		}
	}

	// CropBox: page-level overrides inherited (ISO 32000 §7.7.3.4).
	if inh.cropBox != nil {
		page.CropBox = arrayToBox(inh.cropBox)
	}
	if cb := pageDict.Get("CropBox"); cb != nil {
		if arr, ok := cb.(*core.PdfArray); ok {
			page.CropBox = arrayToBox(arr)
		}
	}

	// BleedBox, TrimBox, ArtBox are NOT inheritable per spec.
	page.BleedBox = parseDictBox(pageDict, "BleedBox")
	page.TrimBox = parseDictBox(pageDict, "TrimBox")
	page.ArtBox = parseDictBox(pageDict, "ArtBox")

	// Width/Height from the visible box (CropBox or MediaBox).
	visible := page.VisibleBox()
	page.Width = visible.Width()
	page.Height = visible.Height()

	// Rotate: page-level overrides inherited.
	if inh.rotate != nil {
		page.Rotate = inh.rotate.IntValue()
	}
	if rot := pageDict.Get("Rotate"); rot != nil {
		if num, ok := rot.(*core.PdfNumber); ok {
			page.Rotate = num.IntValue()
		}
	}

	// Resources: store inherited value for fallback in Resources().
	if inh.resources != nil {
		page.inheritedResources = inh.resources
	}

	return page
}

// arrayToBox converts a PdfArray [x1, y1, x2, y2] to a Box.
func arrayToBox(arr *core.PdfArray) Box {
	if arr == nil || arr.Len() < 4 {
		return Box{}
	}
	return Box{
		X1: pdfNumValue(arr.Elements[0]),
		Y1: pdfNumValue(arr.Elements[1]),
		X2: pdfNumValue(arr.Elements[2]),
		Y2: pdfNumValue(arr.Elements[3]),
	}
}

// parseDictBox reads a box entry from a dictionary, returns zero Box if absent.
func parseDictBox(dict *core.PdfDictionary, key string) Box {
	obj := dict.Get(key)
	if obj == nil {
		return Box{}
	}
	if arr, ok := obj.(*core.PdfArray); ok {
		return arrayToBox(arr)
	}
	return Box{}
}

// Dict returns the raw page dictionary.
func (p *PageInfo) Dict() *core.PdfDictionary {
	return p.pageDict
}

// Resources returns the page's resource dictionary, resolving indirect references.
// If the page has no /Resources entry, falls back to resources inherited from
// ancestor Pages nodes (per ISO 32000 §7.7.3.4).
func (p *PageInfo) Resources() (*core.PdfDictionary, error) {
	res := p.pageDict.Get("Resources")
	if res == nil {
		// Fall back to inherited resources from parent Pages node.
		res = p.inheritedResources
	}
	if res == nil {
		return core.NewPdfDictionary(), nil
	}
	resolved, err := p.reader.resolver.ResolveDeep(res)
	if err != nil {
		return nil, err
	}
	if dict, ok := resolved.(*core.PdfDictionary); ok {
		return dict, nil
	}
	return core.NewPdfDictionary(), nil
}

// ContentStream returns the decompressed content stream bytes for this page.
// If the page has multiple content streams, they are concatenated.
func (p *PageInfo) ContentStream() ([]byte, error) {
	contents := p.pageDict.Get("Contents")
	if contents == nil {
		return nil, nil
	}

	resolved, err := p.reader.resolver.ResolveDeep(contents)
	if err != nil {
		return nil, fmt.Errorf("reader: resolve Contents: %w", err)
	}

	switch v := resolved.(type) {
	case *core.PdfStream:
		return v.Data, nil
	case *core.PdfArray:
		// Multiple content streams — concatenate.
		var result []byte
		for _, elem := range v.Elements {
			streamObj, err := p.reader.resolver.ResolveDeep(elem)
			if err != nil {
				continue
			}
			if stream, ok := streamObj.(*core.PdfStream); ok {
				result = append(result, stream.Data...)
				result = append(result, '\n')
			}
		}
		return result, nil
	default:
		return nil, nil
	}
}

// ExtractText returns text extracted from the page content stream.
// It uses the page's font resources to decode character codes to Unicode
// via ToUnicode CMaps and standard encodings (WinAnsi, MacRoman).
func (p *PageInfo) ExtractText() (string, error) {
	data, err := p.ContentStream()
	if err != nil {
		return "", err
	}
	if data == nil {
		return "", nil
	}

	// Build font cache from page resources for proper text decoding.
	var fonts FontCache
	if p.reader != nil {
		res, resErr := p.Resources()
		if resErr == nil && res != nil {
			fonts = BuildFontCache(res, p.reader.resolver)
		}
	}

	return ExtractTextWithFonts(data, fonts), nil
}

// ContentOps parses the page's content stream into a sequence of operators.
func (p *PageInfo) ContentOps() ([]ContentOp, error) {
	data, err := p.ContentStream()
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	return ParseContentStream(data), nil
}

// TextSpans extracts all text spans from the page with full positioning,
// font, and color information. This is the richest extraction method.
func (p *PageInfo) TextSpans() ([]TextSpan, error) {
	data, err := p.ContentStream()
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}

	var fonts FontCache
	if p.reader != nil {
		res, resErr := p.Resources()
		if resErr == nil && res != nil {
			fonts = BuildFontCache(res, p.reader.resolver)
		}
	}

	ops := ParseContentStream(data)
	proc := NewContentProcessor(fonts)
	return proc.Process(ops), nil
}

// ExtractTextWithStrategy extracts text using a pluggable strategy.
func (p *PageInfo) ExtractTextWithStrategy(strategy ExtractionStrategy) (string, error) {
	spans, err := p.TextSpans()
	if err != nil {
		return "", err
	}
	for _, span := range spans {
		strategy.ProcessSpan(span)
	}
	return strategy.Result(), nil
}

// pdfNumValue extracts a float64 from a PdfNumber or PdfObject.
func pdfNumValue(obj core.PdfObject) float64 {
	if num, ok := obj.(*core.PdfNumber); ok {
		return num.FloatValue()
	}
	return 0
}
