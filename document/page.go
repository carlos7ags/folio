// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package document

import (
	"fmt"
	"math"

	"github.com/carlos7ags/folio/content"
	"github.com/carlos7ags/folio/core"
	"github.com/carlos7ags/folio/font"
	folioimage "github.com/carlos7ags/folio/image"
)

// PageSize defines the dimensions of a page in PDF points (1 point = 1/72 inch).
type PageSize struct {
	Width  float64
	Height float64
}

// Standard page sizes (dimensions in PDF points, 1 point = 1/72 inch).
var (
	// ISO A series
	PageSizeA0 = PageSize{Width: 2383.94, Height: 3370.39}
	PageSizeA1 = PageSize{Width: 1683.78, Height: 2383.94}
	PageSizeA2 = PageSize{Width: 1190.55, Height: 1683.78}
	PageSizeA3 = PageSize{Width: 841.89, Height: 1190.55}
	PageSizeA4 = PageSize{Width: 595.28, Height: 841.89}
	PageSizeA5 = PageSize{Width: 419.53, Height: 595.28}
	PageSizeA6 = PageSize{Width: 297.64, Height: 419.53}

	// ISO B series
	PageSizeB4 = PageSize{Width: 708.66, Height: 1000.63}
	PageSizeB5 = PageSize{Width: 498.90, Height: 708.66}

	// North American sizes
	PageSizeLetter    = PageSize{Width: 612, Height: 792}
	PageSizeLegal     = PageSize{Width: 612, Height: 1008}
	PageSizeTabloid   = PageSize{Width: 792, Height: 1224}
	PageSizeLedger    = PageSize{Width: 1224, Height: 792}
	PageSizeExecutive = PageSize{Width: 522, Height: 756}
)

// Landscape returns a copy of the page size with width and height swapped.
func (ps PageSize) Landscape() PageSize {
	return PageSize{Width: ps.Height, Height: ps.Width}
}

// fontResource is a font registered on a page, either standard or embedded.
type fontResource struct {
	name     string             // resource name (e.g. "F1")
	standard *font.Standard     // non-nil for standard fonts
	embedded *font.EmbeddedFont // non-nil for embedded fonts
}

// imageResource is an image registered on a page.
type imageResource struct {
	name  string // resource name (e.g. "Im1")
	image *folioimage.Image
}

// MarkupType identifies the kind of text markup annotation.
type MarkupType int

const (
	MarkupHighlight MarkupType = iota // yellow highlight
	MarkupUnderline                   // red underline
	MarkupSquiggly                    // wavy underline
	MarkupStrikeOut                   // strikethrough
)

// Annotation represents a PDF annotation on a page.
type Annotation struct {
	subtype  string      // e.g. "Link", "Text", "Highlight", "Underline", "Squiggly", "StrikeOut"
	rect     [4]float64  // [x1, y1, x2, y2] in PDF points
	uri      string      // for /URI actions (external links)
	dest     string      // for /GoTo actions (named destination)
	destPage int         // for /GoTo actions (page index, -1 = use named dest)
	color    *[3]float64 // annotation color (nil = default)
	border   [3]float64  // [hCorner, vCorner, width] — default [0 0 0] = no border

	// Text annotation (sticky note) fields.
	contents string // text content of the annotation
	name     string // icon name: "Comment", "Note", "Help", "Insert", etc.
	open     bool   // whether the popup is initially open

	// Text markup fields.
	quadPoints [][8]float64 // quadrilateral regions for text markup annotations
}

// extGStateResource is a graphics state parameter dictionary registered on a page.
type extGStateResource struct {
	name string              // resource name (e.g. "GS1")
	dict *core.PdfDictionary // the ExtGState dictionary
}

// Page represents a single page in a document.
// It tracks font resources, image resources, and content stream operations.
type Page struct {
	size        PageSize
	customSize  *PageSize           // per-page size override (nil = inherit from Document)
	fonts       []fontResource      // ordered font resources
	images      []imageResource     // ordered image resources
	extGStates  []extGStateResource // ordered ExtGState resources
	stream      *content.Stream     // content stream builder
	annotations []Annotation        // page annotations (links, etc.)
	rotate      int                 // page rotation in degrees (0, 90, 180, 270)

	// Optional page geometry boxes (ISO 32000 §14.11.2).
	// If nil, the box is not written. MediaBox is always derived from size.
	cropBox  *[4]float64 // visible region (default = MediaBox)
	bleedBox *[4]float64 // bleed region for production
	trimBox  *[4]float64 // intended finished page dimensions
	artBox   *[4]float64 // meaningful content area
}

// boxToArray converts a [4]float64 to a PdfArray.
func boxToArray(rect *[4]float64) *core.PdfArray {
	return core.NewPdfArray(
		core.NewPdfReal(rect[0]),
		core.NewPdfReal(rect[1]),
		core.NewPdfReal(rect[2]),
		core.NewPdfReal(rect[3]),
	)
}

// SetCropBox sets the visible region of the page. Content outside this box
// is clipped by the viewer. If not set, defaults to MediaBox.
// rect is [x1, y1, x2, y2] in PDF points.
func (p *Page) SetCropBox(rect [4]float64) *Page {
	p.cropBox = &rect
	return p
}

// SetBleedBox sets the bleed region for production (area to which page
// content should extend for trimming). Defaults to CropBox.
func (p *Page) SetBleedBox(rect [4]float64) *Page {
	p.bleedBox = &rect
	return p
}

// SetTrimBox sets the intended finished page dimensions after trimming.
// Defaults to CropBox. Important for print production.
func (p *Page) SetTrimBox(rect [4]float64) *Page {
	p.trimBox = &rect
	return p
}

// SetArtBox sets the meaningful content area of the page.
// Defaults to CropBox.
func (p *Page) SetArtBox(rect [4]float64) *Page {
	p.artBox = &rect
	return p
}

// SetSize overrides the page size for this page, allowing mixed page sizes
// within a document (e.g., a landscape insert in a portrait document).
// Pass a PageSize value; use standard sizes like PageSizeA4 or PageSizeLetter,
// or define a custom PageSize{Width, Height}.
func (p *Page) SetSize(ps PageSize) *Page {
	p.customSize = &ps
	return p
}

// effectiveSize returns the page size to use for this page.
// If a custom size has been set via SetSize, it is used; otherwise
// the default size inherited from the Document is returned.
func (p *Page) effectiveSize() PageSize {
	if p.customSize != nil {
		return *p.customSize
	}
	return p.size
}

// AddText draws text at the given (x, y) position using a standard font.
// Coordinates are in PDF points with origin at bottom-left.
// Panics if f is nil or size is negative.
func (p *Page) AddText(text string, f *font.Standard, size, x, y float64) {
	if f == nil {
		panic("folio: AddText called with nil font")
	}
	if size < 0 {
		panic("folio: AddText called with negative font size")
	}
	resName := p.standardFontResource(f)
	p.ensureStream()
	p.stream.BeginText()
	p.stream.SetFont(resName, size)
	p.stream.MoveText(x, y)
	p.stream.ShowText(text)
	p.stream.EndText()
}

// AddTextEmbedded draws text at the given (x, y) position using an embedded font.
// The text is encoded as glyph IDs for CIDFont rendering.
// Panics if ef is nil or size is negative.
func (p *Page) AddTextEmbedded(text string, ef *font.EmbeddedFont, size, x, y float64) {
	if ef == nil {
		panic("folio: AddTextEmbedded called with nil font")
	}
	if size < 0 {
		panic("folio: AddTextEmbedded called with negative font size")
	}
	resName := p.embeddedFontResource(ef)
	encoded := ef.EncodeString(text)
	p.ensureStream()
	p.stream.BeginText()
	p.stream.SetFont(resName, size)
	p.stream.MoveText(x, y)
	p.stream.ShowTextHex(encoded)
	p.stream.EndText()
}

// AddImage draws an image at (x, y) with the given width and height in PDF points.
// If width or height is 0, it is calculated from the other dimension
// preserving the aspect ratio. If both are 0, the image is placed at
// its natural size (1 pixel = 1 point).
// Panics if img is nil or width/height is negative.
func (p *Page) AddImage(img *folioimage.Image, x, y, width, height float64) {
	if img == nil {
		panic("folio: AddImage called with nil image")
	}
	if width < 0 || height < 0 {
		panic("folio: AddImage called with negative dimensions")
	}
	if width == 0 && height == 0 {
		width = float64(img.Width())
		height = float64(img.Height())
	} else if width == 0 {
		width = height * img.AspectRatio()
	} else if height == 0 {
		height = width / img.AspectRatio()
	}

	resName := p.imageResource(img)
	p.ensureStream()
	p.stream.SaveState()
	p.stream.ConcatMatrix(width, 0, 0, height, x, y)
	p.stream.Do(resName)
	p.stream.RestoreState()
}

// imageResource returns the resource name for an image,
// registering it if not already present.
func (p *Page) imageResource(img *folioimage.Image) string {
	for _, entry := range p.images {
		if entry.image == img {
			return entry.name
		}
	}
	name := fmt.Sprintf("Im%d", len(p.images)+1)
	p.images = append(p.images, imageResource{name: name, image: img})
	return name
}

// AddLink adds a clickable link annotation to the page.
// rect is [x1, y1, x2, y2] in PDF points (bottom-left origin).
// uri is the URL to open when clicked.
func (p *Page) AddLink(rect [4]float64, uri string) {
	p.annotations = append(p.annotations, Annotation{
		subtype:  "Link",
		rect:     rect,
		uri:      uri,
		destPage: -1,
	})
}

// AddInternalLink adds a clickable link to a named destination within the document.
func (p *Page) AddInternalLink(rect [4]float64, destName string) {
	p.annotations = append(p.annotations, Annotation{
		subtype:  "Link",
		rect:     rect,
		dest:     destName,
		destPage: -1,
	})
}

// AddPageLink adds a clickable link to a specific page in the document (0-based index).
func (p *Page) AddPageLink(rect [4]float64, pageIndex int) {
	p.annotations = append(p.annotations, Annotation{
		subtype:  "Link",
		rect:     rect,
		destPage: pageIndex,
	})
}

// AddTextAnnotation adds a sticky note (text annotation) at the given position.
// The note appears as an icon that reveals the text when clicked.
// rect is [x1, y1, x2, y2] defining the icon position.
// icon is one of: "Comment", "Note", "Help", "Insert", "Key",
// "NewParagraph", "Paragraph" (empty string defaults to "Note").
func (p *Page) AddTextAnnotation(rect [4]float64, text, icon string) {
	if icon == "" {
		icon = "Note"
	}
	p.annotations = append(p.annotations, Annotation{
		subtype:  "Text",
		rect:     rect,
		contents: text,
		name:     icon,
		destPage: -1,
	})
}

// AddTextAnnotationOpen adds a sticky note that is initially open (popup visible).
func (p *Page) AddTextAnnotationOpen(rect [4]float64, text, icon string) {
	if icon == "" {
		icon = "Note"
	}
	p.annotations = append(p.annotations, Annotation{
		subtype:  "Text",
		rect:     rect,
		contents: text,
		name:     icon,
		open:     true,
		destPage: -1,
	})
}

// AddHighlight adds a highlight annotation over a text region.
// rect defines the bounding box. color is [R, G, B] in [0, 1]
// (e.g. [1, 1, 0] for yellow). quadPoints defines the exact text
// regions as quadrilaterals — each quad is 8 floats:
// [x1,y1, x2,y2, x3,y3, x4,y4] (bottom-left origin, counterclockwise).
// If quadPoints is nil, a single quad matching rect is used.
func (p *Page) AddHighlight(rect [4]float64, color [3]float64, quadPoints [][8]float64) {
	c := color
	p.annotations = append(p.annotations, Annotation{
		subtype:    "Highlight",
		rect:       rect,
		color:      &c,
		quadPoints: quadPoints,
		destPage:   -1,
	})
}

// AddUnderline adds an underline annotation over a text region.
func (p *Page) AddUnderline(rect [4]float64, color [3]float64, quadPoints [][8]float64) {
	c := color
	p.annotations = append(p.annotations, Annotation{
		subtype:    "Underline",
		rect:       rect,
		color:      &c,
		quadPoints: quadPoints,
		destPage:   -1,
	})
}

// AddSquiggly adds a squiggly underline annotation over a text region.
func (p *Page) AddSquiggly(rect [4]float64, color [3]float64, quadPoints [][8]float64) {
	c := color
	p.annotations = append(p.annotations, Annotation{
		subtype:    "Squiggly",
		rect:       rect,
		color:      &c,
		quadPoints: quadPoints,
		destPage:   -1,
	})
}

// AddStrikeOut adds a strikeout annotation over a text region.
func (p *Page) AddStrikeOut(rect [4]float64, color [3]float64, quadPoints [][8]float64) {
	c := color
	p.annotations = append(p.annotations, Annotation{
		subtype:    "StrikeOut",
		rect:       rect,
		color:      &c,
		quadPoints: quadPoints,
		destPage:   -1,
	})
}

// AddTextMarkup adds a text markup annotation of the given type.
// This is a generic version of AddHighlight/AddUnderline/AddSquiggly/AddStrikeOut.
func (p *Page) AddTextMarkup(markupType MarkupType, rect [4]float64, color [3]float64, quadPoints [][8]float64) {
	subtypes := [...]string{"Highlight", "Underline", "Squiggly", "StrikeOut"}
	subtype := "Highlight"
	if int(markupType) < len(subtypes) {
		subtype = subtypes[markupType]
	}
	c := color
	p.annotations = append(p.annotations, Annotation{
		subtype:    subtype,
		rect:       rect,
		color:      &c,
		quadPoints: quadPoints,
		destPage:   -1,
	})
}

// SetOpacity sets the fill and stroke opacity for subsequent drawing operations.
// alpha is in [0, 1] where 0 = fully transparent and 1 = fully opaque.
// Returns the ExtGState resource name (e.g. "GS1") so you can also call
// stream.SetExtGState(name) directly.
func (p *Page) SetOpacity(alpha float64) string {
	return p.SetOpacityFillStroke(alpha, alpha)
}

// SetOpacityFillStroke sets separate fill and stroke opacity.
// fillAlpha controls non-stroke operations (text fill, shape fill).
// strokeAlpha controls stroke operations (outlines, borders).
// Values are clamped to [0, 1]. NaN is treated as 1.0 (fully opaque).
func (p *Page) SetOpacityFillStroke(fillAlpha, strokeAlpha float64) string {
	fillAlpha = clampAlpha(fillAlpha)
	strokeAlpha = clampAlpha(strokeAlpha)

	gsDict := core.NewPdfDictionary()
	gsDict.Set("Type", core.NewPdfName("ExtGState"))
	gsDict.Set("ca", core.NewPdfReal(fillAlpha))   // fill opacity
	gsDict.Set("CA", core.NewPdfReal(strokeAlpha)) // stroke opacity

	name := fmt.Sprintf("GS%d", len(p.extGStates)+1)
	p.extGStates = append(p.extGStates, extGStateResource{name: name, dict: gsDict})
	p.ensureStream()
	p.stream.SetExtGState(name)
	return name
}

// clampAlpha clamps a value to [0, 1]. NaN and Inf are treated as 1.0.
func clampAlpha(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 1.0
	}
	return max(0, min(1, v))
}

// SetRotate sets the page rotation in degrees. Must be a multiple of 90.
// Common values: 0 (default), 90 (landscape), 180 (upside down), 270.
// Panics if degrees is not a multiple of 90.
func (p *Page) SetRotate(degrees int) {
	if degrees%90 != 0 {
		panic(fmt.Sprintf("document.Page.SetRotate: degrees must be a multiple of 90, got %d", degrees))
	}
	p.rotate = degrees
}

// ContentStream returns the raw content stream builder for advanced usage.
// Returns nil if no content has been added.
func (p *Page) ContentStream() *content.Stream {
	return p.stream
}

func (p *Page) ensureStream() {
	if p.stream == nil {
		p.stream = content.NewStream()
	}
}

// standardFontResource returns the resource name for a standard font,
// registering it if not already present.
func (p *Page) standardFontResource(f *font.Standard) string {
	for _, entry := range p.fonts {
		if entry.standard != nil && entry.standard.Name() == f.Name() {
			return entry.name
		}
	}
	name := fmt.Sprintf("F%d", len(p.fonts)+1)
	p.fonts = append(p.fonts, fontResource{name: name, standard: f})
	return name
}

// embeddedFontResource returns the resource name for an embedded font,
// registering it if not already present.
func (p *Page) embeddedFontResource(ef *font.EmbeddedFont) string {
	for _, entry := range p.fonts {
		if entry.embedded == ef {
			return entry.name
		}
	}
	name := fmt.Sprintf("F%d", len(p.fonts)+1)
	p.fonts = append(p.fonts, fontResource{name: name, embedded: ef})
	return name
}
