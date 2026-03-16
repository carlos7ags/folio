// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package reader

import "math"

// TextSpan is a positioned piece of text extracted from a content stream.
// It carries full rendering context: position, size, font, color, and
// the current transformation matrix at the time of rendering.
type TextSpan struct {
	Text       string     // decoded Unicode text
	X, Y       float64    // baseline position in user space (after CTM)
	Width      float64    // text width in user space (from glyph metrics or estimate)
	Height     float64    // font size in user space
	Font       string     // font resource name (e.g. "F1")
	Color      [3]float64 // fill color (RGB, 0-1)
	Matrix     [6]float64 // full CTM at time of rendering [a b c d e f]
	Tag        string     // innermost marked content tag (e.g. "P", "H1", "Span"), empty if untagged
	Visible    bool       // false if text rendering mode is invisible (Tr=3)
	SpaceWidth float64    // width of space character in user space (0 if unavailable)
	MCID       int        // marked content identifier (-1 if not inside marked content)
}

// PathOp represents a graphics path operation extracted from a content stream.
type PathOp struct {
	Type        PathType     // move, line, curve, rect, close
	Points      [][2]float64 // control/end points in user space
	StrokeColor [3]float64
	FillColor   [3]float64
	LineWidth   float64
	Painted     PaintOp // how the path was painted (stroke, fill, both)
}

// PathType identifies the kind of path segment.
type PathType int

const (
	PathMove  PathType = iota // moveto
	PathLine                  // lineto
	PathCurve                 // cubic bezier
	PathRect                  // rectangle
	PathClose                 // close subpath
)

// PaintOp describes how a path was painted.
type PaintOp int

const (
	PaintNone       PaintOp = iota
	PaintStroke             // S
	PaintFill               // f
	PaintFillStroke         // B
	PaintClip               // W
)

// ImageRef represents an image reference found in the content stream.
type ImageRef struct {
	Name   string     // XObject resource name (e.g. "Im1")
	X, Y   float64    // position in user space (bottom-left of image)
	Width  float64    // display width in user space
	Height float64    // display height in user space
	Matrix [6]float64 // full CTM at time of rendering
	Inline bool       // true if inline image (BI/ID/EI)
}

// GlyphSpan is a single glyph with its individual position and width.
// Produced when glyph-level extraction is enabled.
type GlyphSpan struct {
	Char  rune
	X, Y  float64 // baseline position in user space
	Width float64 // glyph width in user space
	Font  string
	Color [3]float64
}

// graphicsState holds the mutable PDF graphics state tracked during parsing.
type graphicsState struct {
	ctm       [6]float64 // current transformation matrix
	fillColor [3]float64 // current fill color (RGB)
	fontName  string
	fontSize  float64

	// Text state (within BT...ET)
	textMatrix     [6]float64 // current text matrix (Tm)
	textLineMatrix [6]float64 // line start matrix (set by Td/TD/Tm, used by T*)
	leading        float64
	textRenderMode int // Tr: 0=fill, 1=stroke, 2=fill+stroke, 3=invisible, 4-7=clip variants

	// Marked content tag stack (BMC/BDC ... EMC).
	tagStack []string // current tag nesting, e.g. ["Document", "P"]

	// Marked content identifier from BDC properties (/MCID).
	currentMCID int   // -1 if not inside marked content with MCID
	mcidStack   []int // stack of MCID values for nested BDC/BMC...EMC

	// Clipping (simplified: bounding rect)
	clipX, clipY, clipW, clipH float64
	hasClip                    bool
}

// newGraphicsState returns the default graphics state.
func newGraphicsState() graphicsState {
	return graphicsState{
		ctm:         [6]float64{1, 0, 0, 1, 0, 0}, // identity
		fontSize:    12,
		currentMCID: -1,
	}
}

// ContentProcessor walks a sequence of ContentOps, maintains full graphics
// state (CTM, color, font, clipping), and produces typed results:
// TextSpans, PathOps, ImageRefs, and optionally GlyphSpans.
type ContentProcessor struct {
	fonts  FontCache
	state  graphicsState
	stack  []graphicsState // q/Q save/restore stack
	spans  []TextSpan
	paths  []PathOp
	images []ImageRef
	glyphs []GlyphSpan

	// Current path being constructed (between m/l/c/re and S/f/B).
	curPath     []pathSegment
	lineWidth   float64
	strokeColor [3]float64

	// Options.
	extractGlyphs bool // if true, emit per-glyph GlyphSpans

	// FormXObject resolver: given a resource name (e.g. "Fm1"), returns
	// the parsed content ops of the Form XObject, or nil if not a form.
	// Set via SetFormResolver to enable recursive Form XObject processing.
	formResolver func(name string) []ContentOp
	depth        int // recursion depth (0 = top-level call)
}

type pathSegment struct {
	typ    PathType
	points [][2]float64
}

// SetFormResolver sets a callback that resolves Form XObject names to
// their parsed content ops. When set, the processor recursively processes
// Form XObjects encountered via the Do operator.
//
// Example:
//
//	proc.SetFormResolver(func(name string) []ContentOp {
//	    // Look up XObject in page resources, check /Subtype /Form,
//	    // decompress stream, parse content ops.
//	    return parseFormXObject(resources, name, resolver)
//	})
func (p *ContentProcessor) SetFormResolver(fn func(name string) []ContentOp) {
	p.formResolver = fn
}

// SetExtractGlyphs enables per-glyph span extraction.
// When true, Process() also populates Glyphs().
func (p *ContentProcessor) SetExtractGlyphs(enabled bool) {
	p.extractGlyphs = enabled
}

// Paths returns path operations extracted during Process().
func (p *ContentProcessor) Paths() []PathOp { return p.paths }

// Images returns image references extracted during Process().
func (p *ContentProcessor) Images() []ImageRef { return p.images }

// Glyphs returns per-glyph spans (only if SetExtractGlyphs(true) was called).
func (p *ContentProcessor) Glyphs() []GlyphSpan { return p.glyphs }

// NewContentProcessor creates a processor with the given font cache.
// Pass nil for fonts if font decoding is not needed.
func NewContentProcessor(fonts FontCache) *ContentProcessor {
	return &ContentProcessor{
		fonts: fonts,
		state: newGraphicsState(),
	}
}

// Process walks the content ops and extracts TextSpans with full positioning.
func (p *ContentProcessor) Process(ops []ContentOp) []TextSpan {
	// Only reset on top-level call, not recursive Form XObject calls.
	if p.depth == 0 {
		p.spans = nil
		p.paths = nil
		p.images = nil
		p.glyphs = nil
		p.curPath = nil
	}
	// Guard against excessive recursion from circular Form XObject references.
	const maxFormDepth = 50
	if p.depth >= maxFormDepth {
		return p.spans
	}
	p.depth++
	defer func() { p.depth-- }()

	for _, op := range ops {
		switch op.Operator {

		// --- Graphics state ---
		case "q":
			p.stack = append(p.stack, p.state)
		case "Q":
			if len(p.stack) > 0 {
				p.state = p.stack[len(p.stack)-1]
				p.stack = p.stack[:len(p.stack)-1]
			}
		case "cm":
			if len(op.Operands) >= 6 {
				m := [6]float64{
					tokenFloat(op.Operands[0]), tokenFloat(op.Operands[1]),
					tokenFloat(op.Operands[2]), tokenFloat(op.Operands[3]),
					tokenFloat(op.Operands[4]), tokenFloat(op.Operands[5]),
				}
				p.state.ctm = multiplyMatrix(m, p.state.ctm)
			}

		// --- Color ---
		case "rg": // fill color RGB
			if len(op.Operands) >= 3 {
				p.state.fillColor = [3]float64{
					tokenFloat(op.Operands[0]),
					tokenFloat(op.Operands[1]),
					tokenFloat(op.Operands[2]),
				}
			}
		case "g": // fill color gray
			if len(op.Operands) >= 1 {
				v := tokenFloat(op.Operands[0])
				p.state.fillColor = [3]float64{v, v, v}
			}
		case "k": // fill color CMYK → approximate RGB
			if len(op.Operands) >= 4 {
				c := tokenFloat(op.Operands[0])
				m := tokenFloat(op.Operands[1])
				y := tokenFloat(op.Operands[2])
				k := tokenFloat(op.Operands[3])
				p.state.fillColor = cmykToRGB(c, m, y, k)
			}
		case "cs", "CS":
			// Set color space — we don't track color space state, but the
			// subsequent sc/SC/scn/SCN operators may carry color values we
			// can interpret as DeviceRGB when 3 operands are given.

		case "sc", "scn":
			// Set fill color in current color space. When 3 operands are
			// given, interpret as DeviceRGB (the most common case).
			if len(op.Operands) == 3 {
				p.state.fillColor = [3]float64{
					tokenFloat(op.Operands[0]),
					tokenFloat(op.Operands[1]),
					tokenFloat(op.Operands[2]),
				}
			}
		case "SC", "SCN":
			// Set stroke color in current color space. When 3 operands are
			// given, interpret as DeviceRGB.
			if len(op.Operands) == 3 {
				p.strokeColor = [3]float64{
					tokenFloat(op.Operands[0]),
					tokenFloat(op.Operands[1]),
					tokenFloat(op.Operands[2]),
				}
			}

		// --- Text state ---
		case "BT":
			p.state.textMatrix = identityMatrix
			p.state.textLineMatrix = identityMatrix
		case "ET":
			// End text — nothing to do.

		case "Tf":
			if len(op.Operands) >= 2 {
				if op.Operands[0].Type == TokenName {
					p.state.fontName = op.Operands[0].Value
				}
				p.state.fontSize = absFloat(tokenFloat(op.Operands[1]))
			}
		case "TL":
			if len(op.Operands) >= 1 {
				p.state.leading = tokenFloat(op.Operands[0])
			}
		case "Tr":
			if len(op.Operands) >= 1 {
				p.state.textRenderMode = int(tokenFloat(op.Operands[0]))
			}

		// --- Marked content (structure tags) ---
		case "BMC":
			if len(op.Operands) >= 1 && op.Operands[0].Type == TokenName {
				p.state.tagStack = append(p.state.tagStack, op.Operands[0].Value)
			}
			// BMC has no properties dict, so push -1 for MCID.
			p.state.mcidStack = append(p.state.mcidStack, p.state.currentMCID)
			// currentMCID stays unchanged (no MCID in BMC).
		case "BDC":
			if len(op.Operands) >= 1 && op.Operands[0].Type == TokenName {
				p.state.tagStack = append(p.state.tagStack, op.Operands[0].Value)
			}
			// Save the current MCID on the stack.
			p.state.mcidStack = append(p.state.mcidStack, p.state.currentMCID)
			// Extract /MCID from the inline properties dictionary.
			// BDC operands: /Tag <<dict>> or /Tag /PropertiesName
			// When inline, the dict tokens appear as:
			//   TokenDictOpen, /MCID, number, ..., TokenDictClose
			mcid := extractMCIDFromOperands(op.Operands)
			if mcid >= 0 {
				p.state.currentMCID = mcid
			}
		case "EMC":
			if len(p.state.tagStack) > 0 {
				p.state.tagStack = p.state.tagStack[:len(p.state.tagStack)-1]
			}
			// Restore the previous MCID from the stack.
			if len(p.state.mcidStack) > 0 {
				p.state.currentMCID = p.state.mcidStack[len(p.state.mcidStack)-1]
				p.state.mcidStack = p.state.mcidStack[:len(p.state.mcidStack)-1]
			} else {
				p.state.currentMCID = -1
			}

		// --- Text positioning ---
		case "Tm":
			if len(op.Operands) >= 6 {
				m := [6]float64{
					tokenFloat(op.Operands[0]), tokenFloat(op.Operands[1]),
					tokenFloat(op.Operands[2]), tokenFloat(op.Operands[3]),
					tokenFloat(op.Operands[4]), tokenFloat(op.Operands[5]),
				}
				p.state.textMatrix = m
				p.state.textLineMatrix = m
			}
		case "Td":
			if len(op.Operands) >= 2 {
				tx := tokenFloat(op.Operands[0])
				ty := tokenFloat(op.Operands[1])
				p.state.textLineMatrix = multiplyMatrix([6]float64{1, 0, 0, 1, tx, ty}, p.state.textLineMatrix)
				p.state.textMatrix = p.state.textLineMatrix
			}
		case "TD":
			if len(op.Operands) >= 2 {
				tx := tokenFloat(op.Operands[0])
				ty := tokenFloat(op.Operands[1])
				p.state.leading = -ty
				p.state.textLineMatrix = multiplyMatrix([6]float64{1, 0, 0, 1, tx, ty}, p.state.textLineMatrix)
				p.state.textMatrix = p.state.textLineMatrix
			}
		case "T*":
			p.state.textLineMatrix = multiplyMatrix([6]float64{1, 0, 0, 1, 0, -p.state.leading}, p.state.textLineMatrix)
			p.state.textMatrix = p.state.textLineMatrix

		// --- Text showing ---
		case "Tj":
			if len(op.Operands) > 0 {
				p.emitText(op.Operands[0])
			}
		case "'":
			p.state.textLineMatrix = multiplyMatrix([6]float64{1, 0, 0, 1, 0, -p.state.leading}, p.state.textLineMatrix)
			p.state.textMatrix = p.state.textLineMatrix
			if len(op.Operands) > 0 {
				p.emitText(op.Operands[0])
			}
		case "\"":
			if len(op.Operands) >= 3 {
				p.state.textLineMatrix = multiplyMatrix([6]float64{1, 0, 0, 1, 0, -p.state.leading}, p.state.textLineMatrix)
				p.state.textMatrix = p.state.textLineMatrix
				p.emitText(op.Operands[2])
			}
		case "TJ":
			for _, operand := range op.Operands {
				if operand.Type == TokenString || operand.Type == TokenHexString {
					p.emitText(operand)
				} else if operand.Type == TokenNumber {
					adj := tokenFloat(operand)
					p.state.textMatrix[4] -= adj / 1000 * p.state.fontSize
				}
			}

		// --- Line width ---
		case "w":
			if len(op.Operands) >= 1 {
				p.lineWidth = tokenFloat(op.Operands[0])
			}

		// --- Stroke color ---
		case "RG":
			if len(op.Operands) >= 3 {
				p.strokeColor = [3]float64{
					tokenFloat(op.Operands[0]),
					tokenFloat(op.Operands[1]),
					tokenFloat(op.Operands[2]),
				}
			}
		case "G":
			if len(op.Operands) >= 1 {
				v := tokenFloat(op.Operands[0])
				p.strokeColor = [3]float64{v, v, v}
			}
		case "K":
			if len(op.Operands) >= 4 {
				p.strokeColor = cmykToRGB(
					tokenFloat(op.Operands[0]), tokenFloat(op.Operands[1]),
					tokenFloat(op.Operands[2]), tokenFloat(op.Operands[3]),
				)
			}

		// --- Path construction ---
		case "m": // moveto
			if len(op.Operands) >= 2 {
				x, y := tokenFloat(op.Operands[0]), tokenFloat(op.Operands[1])
				ux, uy := transformPoint(p.state.ctm, x, y)
				p.curPath = append(p.curPath, pathSegment{PathMove, [][2]float64{{ux, uy}}})
			}
		case "l": // lineto
			if len(op.Operands) >= 2 {
				x, y := tokenFloat(op.Operands[0]), tokenFloat(op.Operands[1])
				ux, uy := transformPoint(p.state.ctm, x, y)
				p.curPath = append(p.curPath, pathSegment{PathLine, [][2]float64{{ux, uy}}})
			}
		case "c": // cubic bezier
			if len(op.Operands) >= 6 {
				pts := make([][2]float64, 3)
				for i := 0; i < 3; i++ {
					x, y := tokenFloat(op.Operands[i*2]), tokenFloat(op.Operands[i*2+1])
					pts[i][0], pts[i][1] = transformPoint(p.state.ctm, x, y)
				}
				p.curPath = append(p.curPath, pathSegment{PathCurve, pts})
			}
		case "re": // rectangle
			if len(op.Operands) >= 4 {
				x, y := tokenFloat(op.Operands[0]), tokenFloat(op.Operands[1])
				w, h := tokenFloat(op.Operands[2]), tokenFloat(op.Operands[3])
				ux, uy := transformPoint(p.state.ctm, x, y)
				uw, uh := w*matrixScale(p.state.ctm), h*matrixScale(p.state.ctm)
				p.curPath = append(p.curPath, pathSegment{PathRect, [][2]float64{{ux, uy}, {uw, uh}}})
			}
		case "h": // close path
			p.curPath = append(p.curPath, pathSegment{typ: PathClose})

		// --- Path painting ---
		case "S": // stroke
			p.emitPath(PaintStroke)
		case "f", "F": // fill
			p.emitPath(PaintFill)
		case "B", "b": // fill + stroke
			p.emitPath(PaintFillStroke)
		case "n": // end path (no paint)
			p.curPath = nil
		case "W", "W*": // clip
			p.emitPath(PaintClip)

		// --- XObject (images and forms) ---
		case "Do":
			if len(op.Operands) >= 1 && op.Operands[0].Type == TokenName {
				name := op.Operands[0].Value
				// Record as image reference (caller can check if it's actually a Form).
				p.images = append(p.images, ImageRef{
					Name:   name,
					X:      p.state.ctm[4],
					Y:      p.state.ctm[5],
					Width:  matrixScale(p.state.ctm),
					Height: math.Sqrt(p.state.ctm[2]*p.state.ctm[2] + p.state.ctm[3]*p.state.ctm[3]),
					Matrix: p.state.ctm,
				})

				// If we have a FormXObject resolver, recurse into Form XObjects.
				if p.formResolver != nil {
					if formOps := p.formResolver(name); formOps != nil {
						// Save state, process form content, restore.
						saved := p.state
						p.Process(formOps)
						p.state = saved
					}
				}
			}
		}
	}

	return p.spans
}

// Spans returns the collected text spans from the last Process call.
func (p *ContentProcessor) Spans() []TextSpan {
	return p.spans
}

// emitText decodes a text operand and adds a TextSpan.
func (p *ContentProcessor) emitText(tok Token) {
	raw := []byte(tok.Value)
	fe := p.fontEntry()

	var text string
	if fe != nil {
		text = fe.Decode(raw)
	} else {
		text = string(raw)
	}

	if text == "" {
		return
	}

	// Compute text width from font metrics or estimation.
	var widthTextSpace float64

	if fe != nil {
		tw := fe.TextWidth(raw)
		if tw > 0 {
			// tw is in 1/1000 of text space. Convert to text space units.
			widthTextSpace = float64(tw) / 1000 * p.state.fontSize
		}
	}
	if widthTextSpace == 0 {
		// Fallback: estimate from character count.
		charCount := len([]rune(text))
		widthTextSpace = float64(charCount) * p.state.fontSize * 0.6
	}

	// Compute render matrix: textMatrix x CTM.
	rm := multiplyMatrix(p.state.textMatrix, p.state.ctm)
	ux, uy := rm[4], rm[5]
	rmScale := matrixScale(rm)
	widthUserSpace := widthTextSpace * matrixScale(p.state.textMatrix) * matrixScale(p.state.ctm)

	// Determine visibility and tag.
	visible := p.state.textRenderMode != 3 // mode 3 = invisible
	tag := ""
	if len(p.state.tagStack) > 0 {
		tag = p.state.tagStack[len(p.state.tagStack)-1]
	}

	// Compute space width in user space from font metrics.
	var spaceWidth float64
	if fe != nil {
		sw := fe.SpaceWidth()
		if sw > 0 {
			spaceWidth = float64(sw) / 1000.0 * p.state.fontSize * rmScale
		}
	}

	span := TextSpan{
		Text:       text,
		X:          ux,
		Y:          uy,
		Width:      widthUserSpace,
		Height:     p.state.fontSize * rmScale,
		Font:       p.state.fontName,
		Color:      p.state.fillColor,
		Matrix:     p.state.ctm,
		Tag:        tag,
		Visible:    visible,
		SpaceWidth: spaceWidth,
		MCID:       p.state.currentMCID,
	}

	p.spans = append(p.spans, span)

	// Emit per-glyph spans if enabled.
	p.emitGlyphs(text, ux, uy, p.state.fontSize)

	// Advance text matrix in text space.
	p.state.textMatrix[4] += widthTextSpace
}

// emitGlyphs produces per-glyph GlyphSpans from a text string.
func (p *ContentProcessor) emitGlyphs(text string, startX, y, fontSize float64) {
	if !p.extractGlyphs {
		return
	}
	scale := matrixScale(p.state.ctm)
	fe := p.fontEntry()
	x := startX
	for _, ch := range text {
		var glyphW float64
		if fe != nil {
			// Get width for this character code.
			w := fe.CharWidth(int(ch))
			if w > 0 {
				glyphW = float64(w) / 1000 * fontSize * scale
			}
		}
		if glyphW == 0 {
			glyphW = fontSize * 0.6 * scale
		}

		p.glyphs = append(p.glyphs, GlyphSpan{
			Char:  ch,
			X:     x,
			Y:     y,
			Width: glyphW,
			Font:  p.state.fontName,
			Color: p.state.fillColor,
		})
		x += glyphW
	}
}

// emitPath finishes the current path and records it.
func (p *ContentProcessor) emitPath(paint PaintOp) {
	if len(p.curPath) == 0 {
		return
	}
	for _, seg := range p.curPath {
		p.paths = append(p.paths, PathOp{
			Type:        seg.typ,
			Points:      seg.points,
			StrokeColor: p.strokeColor,
			FillColor:   p.state.fillColor,
			LineWidth:   p.lineWidth * matrixScale(p.state.ctm),
			Painted:     paint,
		})
	}
	p.curPath = nil
}

// fontEntry returns the current font entry from the cache.
func (p *ContentProcessor) fontEntry() *FontEntry {
	if p.fonts == nil {
		return nil
	}
	return p.fonts[p.state.fontName]
}

// extractMCIDFromOperands scans BDC operands for an /MCID integer value.
// Returns -1 if no MCID is found.
//
// In a content stream, BDC operands appear as flat tokens:
//
//	/Tag << /MCID 0 >> BDC
//
// The tokenizer produces: [TokenName("Tag"), TokenDictOpen, TokenName("MCID"),
// TokenNumber(0), TokenDictClose]. We scan for the /MCID name followed by a number.
func extractMCIDFromOperands(operands []Token) int {
	for i := 0; i < len(operands)-1; i++ {
		if operands[i].Type == TokenName && operands[i].Value == "MCID" {
			if operands[i+1].Type == TokenNumber {
				return int(operands[i+1].Int)
			}
		}
	}
	return -1
}

// --- Matrix math ---

// identityMatrix is the 2D affine identity matrix [1 0 0 1 0 0].
var identityMatrix = [6]float64{1, 0, 0, 1, 0, 0}

// multiplyMatrix multiplies two 3x3 matrices stored as [a b c d e f]
// where the matrix is:  [a b 0]
//
//	[c d 0]
//	[e f 1]
func multiplyMatrix(a, b [6]float64) [6]float64 {
	return [6]float64{
		a[0]*b[0] + a[1]*b[2],
		a[0]*b[1] + a[1]*b[3],
		a[2]*b[0] + a[3]*b[2],
		a[2]*b[1] + a[3]*b[3],
		a[4]*b[0] + a[5]*b[2] + b[4],
		a[4]*b[1] + a[5]*b[3] + b[5],
	}
}

// transformPoint applies a CTM to a point.
func transformPoint(ctm [6]float64, x, y float64) (float64, float64) {
	return ctm[0]*x + ctm[2]*y + ctm[4],
		ctm[1]*x + ctm[3]*y + ctm[5]
}

// matrixScale returns the uniform scale factor of a CTM.
// For a simple scale/translate matrix [sx 0 0 sy tx ty], returns sx.
// For a rotated matrix, returns sqrt(a^2 + b^2).
func matrixScale(ctm [6]float64) float64 {
	s := math.Sqrt(ctm[0]*ctm[0] + ctm[1]*ctm[1])
	if s == 0 {
		return 1
	}
	return s
}

// absFloat returns the absolute value of a float.
func absFloat(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// cmykToRGB does a basic CMYK→RGB conversion.
func cmykToRGB(c, m, y, k float64) [3]float64 {
	return [3]float64{
		(1 - c) * (1 - k),
		(1 - m) * (1 - k),
		(1 - y) * (1 - k),
	}
}
