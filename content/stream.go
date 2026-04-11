// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package content

import (
	"bytes"
	"fmt"
	"math"
	"strings"

	"github.com/carlos7ags/folio/core"
)

// Stream builds a PDF content stream from a sequence of operators.
// The resulting bytes go inside a PdfStream on a page's /Contents.
type Stream struct {
	buf bytes.Buffer
}

// NewStream creates an empty content stream builder.
func NewStream() *Stream {
	return &Stream{}
}

// --- Text operators (ISO 32000 §9.4) ---

// BeginText writes the BT (begin text object) operator.
func (s *Stream) BeginText() {
	s.writeln("BT")
}

// EndText writes the ET (end text object) operator.
func (s *Stream) EndText() {
	s.writeln("ET")
}

// SetFont writes the Tf operator: set font and size.
// fontName is the resource name (e.g. "F1"), size is in points.
func (s *Stream) SetFont(fontName string, size float64) {
	s.writeln(fmt.Sprintf("/%s %s Tf", fontName, formatNum(size)))
}

// MoveText writes the Td operator: move to (tx, ty) relative to
// the start of the current line.
func (s *Stream) MoveText(tx, ty float64) {
	s.writeln(fmt.Sprintf("%s %s Td", formatNum(tx), formatNum(ty)))
}

// MoveTextWithLeading writes the TD operator: move to (tx, ty) relative
// to the start of the current line and set the leading to -ty.
// Equivalent to calling SetLeading(-ty) followed by MoveText(tx, ty),
// but in a single operator.
func (s *Stream) MoveTextWithLeading(tx, ty float64) {
	s.writeln(fmt.Sprintf("%s %s TD", formatNum(tx), formatNum(ty)))
}

// ShowText writes the Tj operator: show a text string.
func (s *Stream) ShowText(text string) {
	s.writeln(fmt.Sprintf("(%s) Tj", core.EscapeLiteralString(text)))
}

// MoveToNextLine writes the T* operator: move to the start of the next line.
func (s *Stream) MoveToNextLine() {
	s.writeln("T*")
}

// SetLeading writes the TL operator: set the text leading (line spacing).
func (s *Stream) SetLeading(leading float64) {
	s.writeln(fmt.Sprintf("%s TL", formatNum(leading)))
}

// ShowTextHex writes the Tj operator with a hex-encoded string.
// Used for CIDFont text where each glyph ID is encoded as a
// big-endian uint16 pair.
func (s *Stream) ShowTextHex(data []byte) {
	s.writeln(fmt.Sprintf("<%X> Tj", data))
}

// ShowTextArray writes the TJ operator: show text with per-glyph positioning.
// Each element in the array is either a string (shown as text) or a numeric
// adjustment (in thousandths of a unit of text space — negative values move right).
// This is used for kerning: [(H) -80 (ello)] TJ shifts "ello" 80 units right.
func (s *Stream) ShowTextArray(elements []TextArrayElement) {
	var b strings.Builder
	b.WriteByte('[')
	for _, e := range elements {
		if e.IsAdjustment {
			b.WriteString(formatNum(e.Adjustment))
			b.WriteByte(' ')
		} else {
			fmt.Fprintf(&b, "(%s) ", core.EscapeLiteralString(e.Text))
		}
	}
	b.WriteString("] TJ")
	s.writeln(b.String())
}

// ShowTextArrayHex writes the TJ operator with hex-encoded strings.
// Used for CIDFont text with kerning adjustments.
func (s *Stream) ShowTextArrayHex(elements []TextArrayElement) {
	var b strings.Builder
	b.WriteByte('[')
	for _, e := range elements {
		if e.IsAdjustment {
			b.WriteString(formatNum(e.Adjustment))
			b.WriteByte(' ')
		} else {
			fmt.Fprintf(&b, "<%X> ", e.HexData)
		}
	}
	b.WriteString("] TJ")
	s.writeln(b.String())
}

// TextArrayElement is a single element in a TJ text array.
// Either a text string or a numeric positioning adjustment.
type TextArrayElement struct {
	Text         string  // non-empty for text segments
	HexData      []byte  // non-nil for hex-encoded segments (CIDFont)
	Adjustment   float64 // kerning adjustment (thousandths of text space unit)
	IsAdjustment bool    // true if this is a numeric adjustment, not text
}

// SetCharSpacing writes the Tc operator: set character spacing.
// charSpace is extra spacing added between each character, in text space units.
func (s *Stream) SetCharSpacing(charSpace float64) {
	s.writeln(fmt.Sprintf("%s Tc", formatNum(charSpace)))
}

// SetWordSpacing writes the Tw operator: set word spacing.
// wordSpace is extra spacing added after each ASCII space character (0x20),
// in text space units.
func (s *Stream) SetWordSpacing(wordSpace float64) {
	s.writeln(fmt.Sprintf("%s Tw", formatNum(wordSpace)))
}

// SetHorizontalScaling writes the Tz operator: set the horizontal scaling,
// as a percentage of normal width. 100 means normal width; 50 means half
// width (condensed); 200 means double width (expanded).
func (s *Stream) SetHorizontalScaling(scale float64) {
	s.writeln(fmt.Sprintf("%s Tz", formatNum(scale)))
}

// SetTextRise writes the Ts operator: set text rise.
// rise shifts text up (positive) or down (negative) from the baseline,
// in text space units. Used for superscript and subscript.
func (s *Stream) SetTextRise(rise float64) {
	s.writeln(fmt.Sprintf("%s Ts", formatNum(rise)))
}

// TextRenderingMode constants (ISO 32000 §9.3.6).
const (
	TextRenderFill           = 0 // Fill text (default)
	TextRenderStroke         = 1 // Stroke text
	TextRenderFillStroke     = 2 // Fill then stroke text
	TextRenderInvisible      = 3 // Invisible text (for searchable OCR layers)
	TextRenderFillClip       = 4 // Fill text and add to clipping path
	TextRenderStrokeClip     = 5 // Stroke text and add to clipping path
	TextRenderFillStrokeClip = 6 // Fill, stroke, and clip
	TextRenderClip           = 7 // Add text to clipping path only
)

// SetTextRenderingMode writes the Tr operator: set text rendering mode.
// mode is one of the TextRender* constants (0-7).
func (s *Stream) SetTextRenderingMode(mode int) {
	if mode < 0 || mode > 7 {
		panic(fmt.Sprintf("content: SetTextRenderingMode: invalid mode %d (must be 0-7)", mode))
	}
	s.writeln(fmt.Sprintf("%d Tr", mode))
}

// SetTextMatrix writes the Tm operator: set the text matrix.
// The six values [a b c d e f] define the text position and transformation.
// Common use: Tm(1, 0, 0, 1, x, y) positions text at (x, y).
// For rotated text: Tm(cos, sin, -sin, cos, x, y).
func (s *Stream) SetTextMatrix(a, b, c, d, e, f float64) {
	s.writeln(fmt.Sprintf("%s %s %s %s %s %s Tm",
		formatNum(a), formatNum(b), formatNum(c),
		formatNum(d), formatNum(e), formatNum(f)))
}

// ShowTextNextLine writes the ' operator: move to next line and show text.
func (s *Stream) ShowTextNextLine(text string) {
	s.writeln(fmt.Sprintf("(%s) '", core.EscapeLiteralString(text)))
}

// ShowTextWithSpacing writes the " operator: set word spacing, set character
// spacing, move to the next line, and show the given text. Equivalent to
// SetWordSpacing(wordSpace) + SetCharSpacing(charSpace) + ShowTextNextLine(text).
func (s *Stream) ShowTextWithSpacing(wordSpace, charSpace float64, text string) {
	s.writeln(fmt.Sprintf("%s %s (%s) \"",
		formatNum(wordSpace), formatNum(charSpace),
		core.EscapeLiteralString(text)))
}

// --- Graphics state operators (ISO 32000 §8.4) ---

// SaveState writes the q operator: save the current graphics state.
func (s *Stream) SaveState() {
	s.writeln("q")
}

// RestoreState writes the Q operator: restore the graphics state.
func (s *Stream) RestoreState() {
	s.writeln("Q")
}

// ConcatMatrix writes the cm operator: concatenate a transformation matrix.
// The six values define the matrix [a b c d e f].
// Common use: cm(width, 0, 0, height, x, y) to place an image.
func (s *Stream) ConcatMatrix(a, b, c, d, e, f float64) {
	s.writeln(fmt.Sprintf("%s %s %s %s %s %s cm",
		formatNum(a), formatNum(b), formatNum(c),
		formatNum(d), formatNum(e), formatNum(f)))
}

// SetLineWidth writes the w operator: set the line width.
// width must be >= 0; a value of 0 denotes the thinnest line the
// output device can render.
func (s *Stream) SetLineWidth(width float64) {
	if width < 0 {
		panic(fmt.Sprintf("content: SetLineWidth: invalid width %v (must be >= 0)", width))
	}
	s.writeln(fmt.Sprintf("%s w", formatNum(width)))
}

// Line cap style constants (ISO 32000 §8.4.3.3).
const (
	LineCapButt   = 0 // Butt cap (default) — stroke ends at endpoint
	LineCapRound  = 1 // Round cap — semicircle at endpoint
	LineCapSquare = 2 // Square cap — half-square at endpoint
)

// SetLineCap writes the J operator: set line cap style (0-2).
func (s *Stream) SetLineCap(style int) {
	if style < 0 || style > 2 {
		panic(fmt.Sprintf("content: SetLineCap: invalid style %d (must be 0-2)", style))
	}
	s.writeln(fmt.Sprintf("%d J", style))
}

// Line join style constants (ISO 32000 §8.4.3.4).
const (
	LineJoinMiter = 0 // Miter join (default) — sharp corners
	LineJoinRound = 1 // Round join — rounded corners
	LineJoinBevel = 2 // Bevel join — flat corners
)

// SetLineJoin writes the j operator: set line join style (0-2).
func (s *Stream) SetLineJoin(style int) {
	if style < 0 || style > 2 {
		panic(fmt.Sprintf("content: SetLineJoin: invalid style %d (must be 0-2)", style))
	}
	s.writeln(fmt.Sprintf("%d j", style))
}

// SetMiterLimit writes the M operator: set miter limit for line joins.
// limit must be >= 1 (ISO 32000 §8.4.3.5).
func (s *Stream) SetMiterLimit(limit float64) {
	if limit < 1 {
		panic(fmt.Sprintf("content: SetMiterLimit: invalid limit %v (must be >= 1)", limit))
	}
	s.writeln(fmt.Sprintf("%s M", formatNum(limit)))
}

// SetDashPattern writes the d operator: set line dash pattern.
// dashArray defines the pattern (e.g. [3 2] for 3-on, 2-off).
// phase is the offset into the pattern to start at.
func (s *Stream) SetDashPattern(dashArray []float64, phase float64) {
	var b strings.Builder
	b.WriteByte('[')
	for i, v := range dashArray {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(formatNum(v))
	}
	b.WriteString("] ")
	b.WriteString(formatNum(phase))
	b.WriteString(" d")
	s.writeln(b.String())
}

// SetExtGState writes the gs operator: set graphics state from an
// ExtGState resource dictionary. name is the resource name (e.g. "GS1").
func (s *Stream) SetExtGState(name string) {
	s.writeln(fmt.Sprintf("/%s gs", name))
}

// SetRenderingIntent writes the ri operator: set the color rendering intent.
// Standard values are "AbsoluteColorimetric", "RelativeColorimetric",
// "Saturation", and "Perceptual".
func (s *Stream) SetRenderingIntent(name string) {
	s.writeln(fmt.Sprintf("/%s ri", name))
}

// SetFlatness writes the i operator: set the flatness tolerance for curve
// rendering. Valid range is 0 to 100; 0 uses the output device's default.
// Higher values allow coarser curve approximation (faster but less accurate).
func (s *Stream) SetFlatness(tolerance float64) {
	s.writeln(fmt.Sprintf("%s i", formatNum(tolerance)))
}

// --- Path construction operators (ISO 32000 §8.5.2) ---

// MoveTo writes the m operator: begin a new subpath at (x, y).
func (s *Stream) MoveTo(x, y float64) {
	s.writeln(fmt.Sprintf("%s %s m", formatNum(x), formatNum(y)))
}

// LineTo writes the l operator: append a straight line to (x, y).
func (s *Stream) LineTo(x, y float64) {
	s.writeln(fmt.Sprintf("%s %s l", formatNum(x), formatNum(y)))
}

// Rectangle writes the re operator: append a rectangle.
// (x, y) is the lower-left corner; w and h are width and height.
func (s *Stream) Rectangle(x, y, w, h float64) {
	s.writeln(fmt.Sprintf("%s %s %s %s re", formatNum(x), formatNum(y), formatNum(w), formatNum(h)))
}

// CurveTo writes the c operator: append a cubic Bézier curve.
// (x1, y1) and (x2, y2) are control points; (x3, y3) is the endpoint.
func (s *Stream) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	s.writeln(fmt.Sprintf("%s %s %s %s %s %s c",
		formatNum(x1), formatNum(y1), formatNum(x2), formatNum(y2),
		formatNum(x3), formatNum(y3)))
}

// CurveToV writes the v operator: append a cubic Bézier curve
// with the first control point at the current point.
// (x2, y2) is the second control point; (x3, y3) is the endpoint.
func (s *Stream) CurveToV(x2, y2, x3, y3 float64) {
	s.writeln(fmt.Sprintf("%s %s %s %s v",
		formatNum(x2), formatNum(y2), formatNum(x3), formatNum(y3)))
}

// CurveToY writes the y operator: append a cubic Bézier curve
// with the second control point at the endpoint.
// (x1, y1) is the first control point; (x3, y3) is the endpoint.
func (s *Stream) CurveToY(x1, y1, x3, y3 float64) {
	s.writeln(fmt.Sprintf("%s %s %s %s y",
		formatNum(x1), formatNum(y1), formatNum(x3), formatNum(y3)))
}

// ClipNonZero writes the W operator: set the clipping path using non-zero winding rule.
// Must be followed by a path painting operator (n for no-op paint).
func (s *Stream) ClipNonZero() {
	s.writeln("W")
}

// ClipEvenOdd writes the W* operator: set the clipping path using even-odd rule.
func (s *Stream) ClipEvenOdd() {
	s.writeln("W*")
}

// EndPath writes the n operator: end the path without filling or stroking.
// Often used after clipping operators.
func (s *Stream) EndPath() {
	s.writeln("n")
}

// --- Path painting operators (ISO 32000 §8.5.3) ---

// Stroke writes the S operator: stroke the current path.
func (s *Stream) Stroke() {
	s.writeln("S")
}

// Fill writes the f operator: fill the current path (non-zero winding).
func (s *Stream) Fill() {
	s.writeln("f")
}

// FillEvenOdd writes the f* operator: fill using even-odd rule.
func (s *Stream) FillEvenOdd() {
	s.writeln("f*")
}

// FillAndStroke writes the B operator: fill and stroke the current path.
func (s *Stream) FillAndStroke() {
	s.writeln("B")
}

// FillEvenOddAndStroke writes the B* operator: fill the current path using
// the even-odd rule, then stroke it.
func (s *Stream) FillEvenOddAndStroke() {
	s.writeln("B*")
}

// ClosePathStroke writes the s operator: close path and stroke.
func (s *Stream) ClosePathStroke() {
	s.writeln("s")
}

// ClosePathFillAndStroke writes the b operator: close path, fill, and stroke.
func (s *Stream) ClosePathFillAndStroke() {
	s.writeln("b")
}

// ClosePathFillEvenOddAndStroke writes the b* operator: close the path,
// fill it using the even-odd rule, and stroke it.
func (s *Stream) ClosePathFillEvenOddAndStroke() {
	s.writeln("b*")
}

// ClosePath writes the h operator: close the current subpath.
func (s *Stream) ClosePath() {
	s.writeln("h")
}

// --- Color operators (ISO 32000 §8.6.8) ---

// SetStrokeColorSpace writes the CS operator: set the current color space
// for stroking. name is a device color space ("DeviceRGB", "DeviceGray",
// "DeviceCMYK", "Pattern") or the resource name of a color space in the
// current resource dictionary.
func (s *Stream) SetStrokeColorSpace(name string) {
	s.writeln(fmt.Sprintf("/%s CS", name))
}

// SetFillColorSpace writes the cs operator: set the current color space
// for filling and non-stroking paint operations.
func (s *Stream) SetFillColorSpace(name string) {
	s.writeln(fmt.Sprintf("/%s cs", name))
}

// SetStrokeColor writes the SC operator: set the stroke color using the
// current stroke color space. The number of components depends on the
// color space (1 for Gray, 3 for RGB, 4 for CMYK). For ICCBased or
// Pattern spaces use SetStrokeColorPattern.
func (s *Stream) SetStrokeColor(components ...float64) {
	var b strings.Builder
	for i, c := range components {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(formatNum(c))
	}
	b.WriteString(" SC")
	s.writeln(b.String())
}

// SetFillColor writes the sc operator: set the fill color using the
// current fill color space. The number of components depends on the
// color space (1 for Gray, 3 for RGB, 4 for CMYK). For ICCBased or
// Pattern spaces use SetFillColorPattern.
func (s *Stream) SetFillColor(components ...float64) {
	var b strings.Builder
	for i, c := range components {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(formatNum(c))
	}
	b.WriteString(" sc")
	s.writeln(b.String())
}

// SetStrokeColorPattern writes the SCN operator: set the stroke color using
// a pattern from a Pattern color space, optionally preceded by tint components
// for uncolored patterns. Pass empty patternName to emit SCN with components only
// (useful for DeviceN / ICCBased / Lab color spaces that SC does not support).
func (s *Stream) SetStrokeColorPattern(patternName string, components ...float64) {
	var b strings.Builder
	for i, c := range components {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(formatNum(c))
	}
	if patternName != "" {
		if len(components) > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('/')
		b.WriteString(patternName)
	}
	b.WriteString(" SCN")
	s.writeln(b.String())
}

// SetFillColorPattern writes the scn operator: set the fill color using
// a pattern from a Pattern color space, optionally preceded by tint components
// for uncolored patterns. Pass empty patternName to emit scn with components only
// (useful for DeviceN / ICCBased / Lab color spaces that sc does not support).
func (s *Stream) SetFillColorPattern(patternName string, components ...float64) {
	var b strings.Builder
	for i, c := range components {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(formatNum(c))
	}
	if patternName != "" {
		if len(components) > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('/')
		b.WriteString(patternName)
	}
	b.WriteString(" scn")
	s.writeln(b.String())
}

// SetStrokeColorRGB writes the RG operator: set stroke color in DeviceRGB.
// r, g, b are in [0, 1].
func (s *Stream) SetStrokeColorRGB(r, g, b float64) {
	s.writeln(fmt.Sprintf("%s %s %s RG", formatNum(r), formatNum(g), formatNum(b)))
}

// SetFillColorRGB writes the rg operator: set fill color in DeviceRGB.
// r, g, b are in [0, 1].
func (s *Stream) SetFillColorRGB(r, g, b float64) {
	s.writeln(fmt.Sprintf("%s %s %s rg", formatNum(r), formatNum(g), formatNum(b)))
}

// SetStrokeColorGray writes the G operator: set stroke color in DeviceGray.
// gray is in [0, 1] where 0=black, 1=white.
func (s *Stream) SetStrokeColorGray(gray float64) {
	s.writeln(fmt.Sprintf("%s G", formatNum(gray)))
}

// SetFillColorGray writes the g operator: set fill color in DeviceGray.
func (s *Stream) SetFillColorGray(gray float64) {
	s.writeln(fmt.Sprintf("%s g", formatNum(gray)))
}

// SetStrokeColorCMYK writes the K operator: set stroke color in DeviceCMYK.
// c, m, y, k are in [0, 1].
func (s *Stream) SetStrokeColorCMYK(c, m, y, k float64) {
	s.writeln(fmt.Sprintf("%s %s %s %s K", formatNum(c), formatNum(m), formatNum(y), formatNum(k)))
}

// SetFillColorCMYK writes the k operator: set fill color in DeviceCMYK.
// c, m, y, k are in [0, 1].
func (s *Stream) SetFillColorCMYK(c, m, y, k float64) {
	s.writeln(fmt.Sprintf("%s %s %s %s k", formatNum(c), formatNum(m), formatNum(y), formatNum(k)))
}

// --- Shading operators (ISO 32000 §8.7.4) ---

// ShadingFill writes the sh operator: paint the shape and color defined by
// a shading dictionary within the current clipping region. name is the
// resource name of a shading in the current Pattern resource dictionary.
func (s *Stream) ShadingFill(name string) {
	s.writeln(fmt.Sprintf("/%s sh", name))
}

// --- XObject operators (ISO 32000 §8.8) ---

// Do writes the Do operator: paint the named XObject.
// name is the resource name (e.g. "Im1").
func (s *Stream) Do(name string) {
	s.writeln(fmt.Sprintf("/%s Do", name))
}

// --- Marked content operators (ISO 32000 §14.6) ---

// BeginMarkedContent writes the BMC operator: begin a marked content sequence.
// tag is the structure type (e.g. "P", "Span", "Figure").
func (s *Stream) BeginMarkedContent(tag string) {
	s.writeln(fmt.Sprintf("/%s BMC", tag))
}

// BeginMarkedContentWithID writes the BDC operator: begin a marked content
// sequence with a property dictionary containing an MCID.
// tag is the structure type; mcid links this content to the structure tree.
func (s *Stream) BeginMarkedContentWithID(tag string, mcid int) {
	s.writeln(fmt.Sprintf("/%s <</MCID %d>> BDC", tag, mcid))
}

// MarkedPoint writes the MP operator: designate a marked-content point.
// tag is the marked-content tag (structure type).
func (s *Stream) MarkedPoint(tag string) {
	s.writeln(fmt.Sprintf("/%s MP", tag))
}

// MarkedPointWithID writes the DP operator: designate a marked-content
// point with an MCID property that links to the structure tree.
func (s *Stream) MarkedPointWithID(tag string, mcid int) {
	s.writeln(fmt.Sprintf("/%s <</MCID %d>> DP", tag, mcid))
}

// EndMarkedContent writes the EMC operator: end the current marked content sequence.
func (s *Stream) EndMarkedContent() {
	s.writeln("EMC")
}

// --- Convenience path helpers ---

// Circle appends a circular path centered at (cx, cy) with the given radius.
// Uses four cubic Bézier curves (the standard approximation).
// Does NOT stroke or fill — call Stroke(), Fill(), etc. after.
func (s *Stream) Circle(cx, cy, r float64) {
	s.Ellipse(cx, cy, r, r)
}

// Ellipse appends an elliptical path centered at (cx, cy) with radii rx, ry.
// Uses four cubic Bézier curves.
func (s *Stream) Ellipse(cx, cy, rx, ry float64) {
	// Magic number for circular Bézier approximation: 4*(√2 - 1)/3 ≈ 0.5523
	const k = 0.5522847498
	kx := rx * k
	ky := ry * k

	s.MoveTo(cx+rx, cy)
	s.CurveTo(cx+rx, cy+ky, cx+kx, cy+ry, cx, cy+ry) // top
	s.CurveTo(cx-kx, cy+ry, cx-rx, cy+ky, cx-rx, cy) // left
	s.CurveTo(cx-rx, cy-ky, cx-kx, cy-ry, cx, cy-ry) // bottom
	s.CurveTo(cx+kx, cy-ry, cx+rx, cy-ky, cx+rx, cy) // right
	s.ClosePath()
}

// RoundedRect appends a rounded rectangle path.
// (x, y) is the lower-left corner; w and h are width and height;
// r is the corner radius (clamped to half the smaller dimension).
func (s *Stream) RoundedRect(x, y, w, h, r float64) {
	maxR := min(w, h) / 2
	if r > maxR {
		r = maxR
	}
	const k = 0.5522847498
	kr := r * k

	// Start at bottom-left, just past the corner radius.
	s.MoveTo(x+r, y)
	// Bottom edge → bottom-right corner
	s.LineTo(x+w-r, y)
	s.CurveTo(x+w-r+kr, y, x+w, y+r-kr, x+w, y+r)
	// Right edge → top-right corner
	s.LineTo(x+w, y+h-r)
	s.CurveTo(x+w, y+h-r+kr, x+w-r+kr, y+h, x+w-r, y+h)
	// Top edge → top-left corner
	s.LineTo(x+r, y+h)
	s.CurveTo(x+r-kr, y+h, x, y+h-r+kr, x, y+h-r)
	// Left edge → bottom-left corner
	s.LineTo(x, y+r)
	s.CurveTo(x, y+r-kr, x+r-kr, y, x+r, y)
	s.ClosePath()
}

// RoundedRectPerCorner draws a rounded rectangle with different radii per corner.
// The radii are rTL (top-left), rTR (top-right), rBR (bottom-right), rBL (bottom-left).
// In PDF coordinates y increases upward, so (x, y) is the bottom-left of the rect.
//
// Radii are proportionally reduced so that no edge is over-subscribed by its
// two adjacent corners (the CSS border-radius algorithm, CSS Backgrounds and
// Borders Module Level 3 §5.5). Negative radii are treated as 0.
func (s *Stream) RoundedRectPerCorner(x, y, w, h, rTL, rTR, rBR, rBL float64) {
	if rTL < 0 {
		rTL = 0
	}
	if rTR < 0 {
		rTR = 0
	}
	if rBR < 0 {
		rBR = 0
	}
	if rBL < 0 {
		rBL = 0
	}
	f := 1.0
	if s := rTL + rTR; s > 0 {
		f = min(f, w/s)
	}
	if s := rTR + rBR; s > 0 {
		f = min(f, h/s)
	}
	if s := rBL + rBR; s > 0 {
		f = min(f, w/s)
	}
	if s := rTL + rBL; s > 0 {
		f = min(f, h/s)
	}
	if f < 1 {
		rTL *= f
		rTR *= f
		rBR *= f
		rBL *= f
	}
	const k = 0.5522847498

	// Start at bottom-left, just past the BL corner radius.
	s.MoveTo(x+rBL, y)
	// Bottom edge → bottom-right corner
	s.LineTo(x+w-rBR, y)
	if rBR > 0 {
		kr := rBR * k
		s.CurveTo(x+w-rBR+kr, y, x+w, y+rBR-kr, x+w, y+rBR)
	}
	// Right edge → top-right corner
	s.LineTo(x+w, y+h-rTR)
	if rTR > 0 {
		kr := rTR * k
		s.CurveTo(x+w, y+h-rTR+kr, x+w-rTR+kr, y+h, x+w-rTR, y+h)
	}
	// Top edge → top-left corner
	s.LineTo(x+rTL, y+h)
	if rTL > 0 {
		kr := rTL * k
		s.CurveTo(x+rTL-kr, y+h, x, y+h-rTL+kr, x, y+h-rTL)
	}
	// Left edge → bottom-left corner
	s.LineTo(x, y+rBL)
	if rBL > 0 {
		kr := rBL * k
		s.CurveTo(x, y+rBL-kr, x+rBL-kr, y, x+rBL, y)
	}
	s.ClosePath()
}

// --- Output ---

// PrependBytes inserts raw content stream bytes before any existing
// content. This is used for watermarks and other background elements
// that must be drawn before the page's main content.
func (s *Stream) PrependBytes(data []byte) {
	if len(data) == 0 {
		return
	}
	existing := make([]byte, s.buf.Len())
	copy(existing, s.buf.Bytes())
	s.buf.Reset()
	s.buf.Write(data)
	if len(existing) > 0 {
		s.buf.WriteByte('\n')
		s.buf.Write(existing)
	}
}

// ReplaceInBytes performs a byte-level replacement in the content stream.
// Used for second-pass substitutions like total page count placeholders.
func (s *Stream) ReplaceInBytes(old, new string) {
	data := bytes.ReplaceAll(s.buf.Bytes(), []byte(old), []byte(new))
	s.buf.Reset()
	s.buf.Write(data)
}

// AppendBytes appends raw content stream bytes after the existing content.
func (s *Stream) AppendBytes(data []byte) {
	if len(data) == 0 {
		return
	}
	if s.buf.Len() > 0 {
		s.buf.WriteByte('\n')
	}
	s.buf.Write(data)
}

// Bytes returns the content stream as raw bytes, suitable for
// embedding in a PdfStream.
func (s *Stream) Bytes() []byte {
	return s.buf.Bytes()
}

// ToPdfStream wraps the content stream bytes in a compressed core.PdfStream.
// FlateDecode compression typically reduces content stream size by 60-80%.
func (s *Stream) ToPdfStream() *core.PdfStream {
	return core.NewPdfStreamCompressed(s.Bytes())
}

// --- Internals ---

// writeln appends a single operator line to the content stream,
// inserting a newline separator if the buffer is non-empty.
func (s *Stream) writeln(line string) {
	if s.buf.Len() > 0 {
		s.buf.WriteByte('\n')
	}
	s.buf.WriteString(line)
}

// formatNum formats a number for PDF content streams.
// Integers are written without decimal points. NaN and Inf are
// replaced with 0 to avoid producing invalid PDF tokens.
// Fractional values are formatted with up to 6 decimal places,
// so magnitudes smaller than 1e-6 round to 0. Values with
// |v| >= 1e15 fall through to float formatting to avoid
// int64 precision loss.
func formatNum(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	if v == float64(int64(v)) && math.Abs(v) < 1e15 {
		return fmt.Sprintf("%d", int64(v))
	}
	s := fmt.Sprintf("%.6f", v)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}
