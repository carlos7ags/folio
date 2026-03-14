// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package layout

import "github.com/carlos7ags/folio/font"

// Align specifies horizontal text alignment.
type Align int

const (
	AlignLeft Align = iota
	AlignCenter
	AlignRight
	AlignJustify
)

// ColorSpace identifies the color space of a Color value.
type ColorSpace int

const (
	ColorSpaceRGB  ColorSpace = iota // DeviceRGB (default)
	ColorSpaceCMYK                   // DeviceCMYK
)

// Color represents a color value. The zero value is RGB black.
// Use RGB(), CMYK(), Gray(), or Hex() constructors to create colors.
type Color struct {
	R, G, B    float64    // RGB components (0-1), used when Space == ColorSpaceRGB
	C, M, Y, K float64    // CMYK components (0-1), used when Space == ColorSpaceCMYK
	Space      ColorSpace // color space (default: RGB)
}

// Common color constants.
var (
	ColorBlack     = Color{R: 0, G: 0, B: 0}
	ColorWhite     = Color{R: 1, G: 1, B: 1}
	ColorRed       = Color{R: 1, G: 0, B: 0}
	ColorGreen     = Color{R: 0, G: 0.5, B: 0}
	ColorBlue      = Color{R: 0, G: 0, B: 1}
	ColorYellow    = Color{R: 1, G: 1, B: 0}
	ColorCyan      = Color{R: 0, G: 1, B: 1}
	ColorMagenta   = Color{R: 1, G: 0, B: 1}
	ColorGray      = Color{R: 0.5, G: 0.5, B: 0.5}
	ColorLightGray = Color{R: 0.75, G: 0.75, B: 0.75}
	ColorDarkGray  = Color{R: 0.25, G: 0.25, B: 0.25}
	ColorOrange    = Color{R: 1, G: 0.647, B: 0}
	ColorNavy      = Color{R: 0, G: 0, B: 0.5}
	ColorMaroon    = Color{R: 0.5, G: 0, B: 0}
	ColorPurple    = Color{R: 0.5, G: 0, B: 0.5}
	ColorTeal      = Color{R: 0, G: 0.5, B: 0.5}
)

// RGB creates a Color from values in [0, 1].
func RGB(r, g, b float64) Color {
	return Color{R: r, G: g, B: b}
}

// CMYK creates a DeviceCMYK color from values in [0, 1].
func CMYK(c, m, y, k float64) Color {
	return Color{C: c, M: m, Y: y, K: k, Space: ColorSpaceCMYK}
}

// Gray creates a grayscale Color from a single value in [0, 1]
// where 0 is black and 1 is white.
func Gray(v float64) Color {
	return Color{R: v, G: v, B: v}
}

// Hex creates a Color from a hex string like "#FF8800" or "FF8800".
func Hex(hex string) Color {
	if len(hex) > 0 && hex[0] == '#' {
		hex = hex[1:]
	}
	if len(hex) != 6 {
		return ColorBlack
	}
	r := hexByte(hex[0:2])
	g := hexByte(hex[2:4])
	b := hexByte(hex[4:6])
	return Color{R: float64(r) / 255, G: float64(g) / 255, B: float64(b) / 255}
}

func hexByte(s string) byte {
	var v byte
	for _, c := range []byte(s) {
		v <<= 4
		switch {
		case c >= '0' && c <= '9':
			v |= c - '0'
		case c >= 'a' && c <= 'f':
			v |= c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v |= c - 'A' + 10
		}
	}
	return v
}

// TextRun is a styled span of text within a paragraph.
// A paragraph is composed of one or more runs, each with its own
// font, size, and color. Runs flow together: the text of consecutive
// runs is concatenated and word-wrapped as a unit.
type TextRun struct {
	Text            string
	Font            *font.Standard
	Embedded        *font.EmbeddedFont
	FontSize        float64
	Color           Color
	Decoration      TextDecoration
	DecorationColor *Color  // if non-nil, decoration uses this color instead of text Color
	DecorationStyle string  // "solid" (default), "dashed", "dotted", "double", "wavy"
	LetterSpacing   float64 // extra space between characters (points, from CSS letter-spacing)
	WordSpacing     float64 // extra space between words (points, from CSS word-spacing)
}

// Run creates a TextRun with a standard font.
func Run(text string, f *font.Standard, fontSize float64) TextRun {
	return TextRun{Text: text, Font: f, FontSize: fontSize}
}

// RunEmbedded creates a TextRun with an embedded font.
func RunEmbedded(text string, ef *font.EmbeddedFont, fontSize float64) TextRun {
	return TextRun{Text: text, Embedded: ef, FontSize: fontSize}
}

// WithColor returns a copy of the run with the given color.
func (r TextRun) WithColor(c Color) TextRun {
	r.Color = c
	return r
}

// WithUnderline returns a copy of the run with underline decoration.
func (r TextRun) WithUnderline() TextRun {
	r.Decoration |= DecorationUnderline
	return r
}

// WithStrikethrough returns a copy of the run with strikethrough decoration.
func (r TextRun) WithStrikethrough() TextRun {
	r.Decoration |= DecorationStrikethrough
	return r
}

// Element is anything the layout engine can render into a page.
// It computes a height-aware layout plan within a given area,
// supporting content splitting across pages via overflow.
type Element interface {
	// PlanLayout computes the layout within the given area.
	// The implementation must not mutate the receiver.
	// If the element doesn't fit entirely, it returns LayoutPartial
	// with Overflow set to a new element containing the remainder.
	PlanLayout(area LayoutArea) LayoutPlan
}

// VAlign specifies vertical alignment within a container (e.g. table cell).
type VAlign int

const (
	VAlignTop VAlign = iota // default
	VAlignMiddle
	VAlignBottom
)

// TextDecoration specifies text decoration (underline, strikethrough).
type TextDecoration int

const (
	DecorationNone          TextDecoration = 0
	DecorationUnderline     TextDecoration = 1 << 0
	DecorationStrikethrough TextDecoration = 1 << 1
)

// Line is a single horizontal line produced by layout.
// It carries enough information for the renderer to emit PDF operators.
type Line struct {
	Words        []Word              // the words on this line (nil for table rows)
	Width        float64             // actual content width (sum of word widths + spaces)
	Height       float64             // line height (fontSize * leading)
	SpaceW       float64             // default space width (used for single-font lines)
	Align        Align               // alignment for this line
	IsLast       bool                // true if this is the last line of a paragraph (justify→left)
	tableRow     *tableRowRef        // non-nil if this line is a table row
	imageRef     *imageLayoutRef     // non-nil if this line is an image
	listRef      *listLayoutRef      // non-nil if this line is a list item
	columnsRef   *columnsLayoutRef   // non-nil if this line is a columns row
	divRef       *divLayoutRef       // non-nil if this line is a div container
	separatorRef *separatorLayoutRef // non-nil if this line is a horizontal rule
	linkRef      *linkLayoutRef      // non-nil if this line is a clickable link

	// Box model fields.
	SpaceBefore  float64 // extra vertical space before this line (points)
	SpaceAfterV  float64 // extra vertical space after this line (points)
	Background   *Color  // if non-nil, fill rect behind this line
	KeepWithNext bool    // don't page-break between this line and the next
	areaBreak    bool    // if true, force a new page before subsequent content

	// Tagged PDF fields.
	StructTag string // if non-empty, wrap content in BDC/EMC with this tag
	MCID      int    // marked content ID (-1 = unassigned; set by document layer)
	Tagged    bool   // whether this line should emit marked content operators
	HintTag   string // element-provided tag override (e.g. "H1" for headings)
}

// Word is a measured chunk of text (no spaces).
type Word struct {
	Text  string
	Width float64
	// Font info needed by the renderer to emit Tf/Tj operators.
	Font            *font.Standard
	Embedded        *font.EmbeddedFont
	FontSize        float64
	Color           Color
	Decoration      TextDecoration
	DecorationColor *Color // if non-nil, decoration uses this color
	DecorationStyle string // "solid", "dashed", "dotted", "double", "wavy"
	// SpaceAfter is the width of a space character in this word's font/size.
	// Used to compute the gap to the next word when fonts differ.
	SpaceAfter    float64
	LetterSpacing float64 // extra inter-character space (Tc operator)
	WordSpacing   float64 // extra inter-word space added to SpaceAfter
}
