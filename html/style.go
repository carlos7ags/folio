// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import "github.com/carlos7ags/folio/layout"

// computedStyle holds the resolved CSS properties for a single HTML node.
type computedStyle struct {
	// Text
	FontFamily           string // "helvetica", "courier", "times"
	FontSize             float64
	FontWeight           int    // CSS Fonts L4 numeric ladder (100..900). 400 = normal, 700 = bold. 0 means "not set" (treated as 400).
	FontStyle            string // "normal", "italic"
	Color                layout.Color
	TextAlign            layout.Align
	TextAlignSet         bool         // true if text-align was declared on this element or inherited from an ancestor that declared it
	TextAlignSelfSet     bool         // true only if text-align was declared on THIS element; not inherited. Distinguishes an author's own choice from an inherited one for the UA defaults that a direct declaration outranks (<th> centering, <caption> centering).
	TextAlignKeyword     string       // "start" or "end" when the author used the direction-relative keyword; otherwise "". Resolved at consumer time via resolveTextAlign(style).
	TextAlignLast        layout.Align // text-align-last override for the last line
	TextAlignLastSet     bool         // true if text-align-last was explicitly set
	TextAlignLastKeyword string       // "start"/"end" for direction-relative text-align-last; resolve via resolveTextAlignLast(style)
	TextDecoration       layout.TextDecoration
	TextTransform        string // "none", "uppercase", "lowercase", "capitalize"
	WhiteSpace           string // "normal", "nowrap", "pre", "pre-wrap", "pre-line"
	LineHeight           float64
	LetterSpacing        float64 // extra space between characters (points)
	WordSpacing          float64 // extra space between words (points)
	TextIndent           float64 // first-line indent (points)
	WordBreak            string  // "normal", "break-all"
	Hyphens              string  // "none", "manual", "auto"

	// Box model
	MarginTopAuto   bool // true if margin-top: auto (for flex layout)
	MarginLeftAuto  bool // true if margin-left: auto (for centering)
	MarginRightAuto bool // true if margin-right: auto (for centering)

	// Margin / padding are stored as unresolved *cssLength values so that
	// percent / calc / min / max / clamp expressions can be resolved
	// against the containing-block width at consumer time (#269). The
	// parser cannot eagerly resolve them — containing-block width is
	// unknown when CSS is applied — so the helpers MarginTopAt /
	// PaddingTopAt below take a container width argument.
	//
	// Phase 1+2 stored both an eagerly-resolved float64 sibling and the
	// *cssLength here. Phase 4 dropped the float64 fields; the helpers
	// are now the only read path. A nil pointer means the side was not
	// declared.
	MarginTopLength    *cssLength
	MarginRightLength  *cssLength
	MarginBottomLength *cssLength
	MarginLeftLength   *cssLength

	PaddingTopLength    *cssLength
	PaddingRightLength  *cssLength
	PaddingBottomLength *cssLength
	PaddingLeftLength   *cssLength

	// Borders
	BorderTopWidth    float64
	BorderRightWidth  float64
	BorderBottomWidth float64
	BorderLeftWidth   float64

	BorderTopColor    layout.Color
	BorderRightColor  layout.Color
	BorderBottomColor layout.Color
	BorderLeftColor   layout.Color

	BorderTopStyle    string
	BorderRightStyle  string
	BorderBottomStyle string
	BorderLeftStyle   string

	// Text direction (bidi)
	Direction   layout.Direction // DirectionAuto (default), DirectionLTR, DirectionRTL
	UnicodeBidi string           // "normal", "embed", "bidi-override", "isolate"

	// Layout
	Display         string // "block", "inline", "flex", "none", "table", etc.
	Float           string // "left", "right", "none"
	Clear           string // "left", "right", "both", "none"
	Width           *cssLength
	Height          *cssLength
	MaxWidth        *cssLength
	MinWidth        *cssLength
	AspectRatio     float64 // width/height ratio (0 = not set); e.g. 16/9 = 1.778
	BackgroundColor *layout.Color

	// Background image
	BackgroundImage    string // "url(...)" or "linear-gradient(...)" or "radial-gradient(...)"
	BackgroundSize     string // "auto", "cover", "contain", "Xpx Ypx"
	BackgroundPosition string // "center", "top left", "X% Y%"
	BackgroundRepeat   string // "repeat", "no-repeat", "repeat-x", "repeat-y"

	// Object fit/position (for images)
	ObjectFit      string // "contain", "cover", "fill", "none", "scale-down"
	ObjectPosition string // e.g. "center", "top left", "50% 50%"

	// Positioning
	Position  string // "static", "relative", "absolute", "fixed"
	Top       *cssLength
	Left      *cssLength
	Right     *cssLength
	Bottom    *cssLength
	ZIndex    int  // z-index (default 0; negative = behind normal flow)
	ZIndexSet bool // true if z-index was explicitly set

	// Flex
	FlexDirection  string
	JustifyContent string
	AlignItems     string
	FlexWrap       string
	FlexGrow       float64
	FlexShrink     float64
	FlexBasis      *cssLength
	AlignSelf      string // "auto", "flex-start", "flex-end", "center", "stretch"
	AlignContent   string // "flex-start", "flex-end", "center", "space-between", "space-around", "stretch"
	JustifyItems   string // "start", "end", "center", "stretch" (grid only)
	Gap            float64
	Order          int // CSS order property; ascending sort key for flex children, ties broken by DOM order

	// Grid
	GridTemplateColumns string     // raw CSS value e.g. "1fr 1fr 1fr", "200px 1fr 2fr"
	GridTemplateRows    string     // raw CSS value
	GridColumnStart     int        // 1-based line number, 0 = auto
	GridColumnEnd       int        // 1-based line number, 0 = auto
	GridRowStart        int        // 1-based line number, 0 = auto
	GridRowEnd          int        // 1-based line number, 0 = auto
	GridAutoFlow        string     // "row" (default)
	GridAutoRows        string     // raw CSS value for implicit row sizing
	GridTemplateAreas   [][]string // parsed grid-template-areas, e.g. [["header","header"],["sidebar","content"]]
	GridArea            string     // grid-area name for placement
	RowGap              float64    // row-gap (takes priority over Gap for grid)
	GridColumnGap       float64    // column-gap for grid (takes priority over Gap for grid)

	// List
	ListStyleType      string
	ListStylePosition  string        // "inside" or "outside" (empty = outside default)
	ListMarkerColor    *layout.Color // marker color from ::marker pseudo-element
	ListMarkerFontSize float64       // marker font size from ::marker (0 = use default)

	// CSS string-set for running headers (e.g. string-set: chapter content())
	StringSetName  string // name of the string (e.g. "chapter")
	StringSetValue string // "content()" or literal string

	// Page break
	PageBreakBefore string // "auto", "always", "avoid"
	PageBreakAfter  string // "auto", "always", "avoid"
	PageBreakInside string // "auto", "avoid"

	// Orphans and widows (paged media)
	Orphans int // minimum lines at bottom of page (0 = not set)
	Widows  int // minimum lines at top of page (0 = not set)

	// CSS bookmark properties
	BookmarkLevel    int    // bookmark-level override (0 = use heading level, -1 = "none" / skip)
	BookmarkLevelSet bool   // true if bookmark-level was explicitly set
	BookmarkLabel    string // bookmark-label override (empty = use heading text)
	BookmarkState    string // bookmark-state: "open" (default) or "closed"

	// Table
	BorderCollapse     string  // "separate", "collapse"
	BorderSpacingH     float64 // horizontal border-spacing (points)
	BorderSpacingV     float64 // vertical border-spacing (points)
	VerticalAlign      string  // "top", "middle", "bottom", "super", "sub" (for table cells and inline)
	BaselineShiftValue float64 // explicit baseline-shift in points (from CSS baseline-shift property)
	BaselineShiftSet   bool    // true if baseline-shift was explicitly set via CSS

	// Visual effects
	BorderRadius   float64 // uniform corner radius (points, 0 = sharp)
	BorderRadiusTL float64 // per-corner: top-left
	BorderRadiusTR float64 // per-corner: top-right
	BorderRadiusBR float64 // per-corner: bottom-right
	BorderRadiusBL float64 // per-corner: bottom-left
	// Per-corner percentage radii (fraction 0..1; 0 = not a percentage).
	// CSS resolves a percentage radius against the box width (horizontal axis)
	// and height (vertical axis) at layout time, yielding elliptical corners.
	BorderRadiusTLPct float64
	BorderRadiusTRPct float64
	BorderRadiusBRPct float64
	BorderRadiusBLPct float64
	Opacity           float64 // 0..1 (0 = default, meaning "not set")
	Overflow          string  // "visible", "hidden"

	// Box shadow (multiple shadows supported, drawn bottom-to-top)
	BoxShadows []boxShadow

	// Text shadow
	TextShadow *boxShadow // reuses boxShadow struct (same fields minus Inset)

	// Text overflow
	TextOverflow string // "clip" (default), "ellipsis"

	// Outline
	OutlineWidth  float64
	OutlineStyle  string
	OutlineColor  layout.Color
	OutlineOffset float64

	// Columns
	ColumnCount     int
	ColumnWidth     float64 // CSS column-width in points (0 = auto)
	ColumnGap       float64
	ColumnRuleWidth float64
	ColumnRuleStyle string // "solid", "dashed", "dotted"
	ColumnRuleColor layout.Color
	ColumnSpan      string // "none" (default), "all"

	// Text decoration extensions
	TextDecorationColor *layout.Color
	TextDecorationStyle string // "solid", "dashed", "dotted", "double", "wavy"

	// Box sizing
	BoxSizing string // "content-box" (default), "border-box"

	// Visibility
	Visibility string // "visible" (default), "hidden", "collapse"

	// Height constraints
	MinHeight *cssLength
	MaxHeight *cssLength

	// CSS transforms
	Transform       string // raw CSS transform value, e.g. "rotate(45deg) scale(1.5)"
	TransformOrigin string // e.g. "center center", "top left", "50% 50%"

	// CSS custom properties (variables)
	CustomProperties map[string]string

	// CSS counters
	CounterReset     []counterEntry // counter-reset declarations
	CounterIncrement []counterEntry // counter-increment declarations
}

// counterEntry represents a single counter name/value pair in
// counter-reset or counter-increment declarations.
type counterEntry struct {
	Name  string
	Value int
}

// boxShadow represents a parsed CSS box-shadow value.
type boxShadow struct {
	OffsetX float64
	OffsetY float64
	Blur    float64
	Spread  float64
	Color   layout.Color
	Inset   bool
}

// cssLength represents a CSS length value, including calc() expressions.
type cssLength struct {
	Value float64
	Unit  string // "px", "pt", "em", "%", "rem"

	// calc expression: if non-nil, this length is a calc() and
	// Value/Unit are ignored. Resolved at toPoints() time.
	calc *calcExpr

	// Math functions: min(), max(), clamp().
	// If minArgs or maxArgs is non-nil, this is a min()/max() function.
	// For clamp(min, preferred, max), clampArgs holds [3]*cssLength.
	minArgs []*cssLength // min(a, b, ...)
	maxArgs []*cssLength // max(a, b, ...)
}

// calcOp is an operator in a calc expression.
type calcOp int

const (
	calcAdd calcOp = iota // calcAdd represents addition (+).
	calcSub               // calcSub represents subtraction (-).
	calcMul               // calcMul represents multiplication (*).
	calcDiv               // calcDiv represents division (/).
)

// calcExpr represents a parsed calc() expression tree.
// Supports: +, -, *, / with length and number operands.
type calcExpr struct {
	// Leaf: a single length value.
	leaf *cssLength // non-nil for leaf nodes

	// Branch: left op right.
	left  *calcExpr
	op    calcOp
	right *calcExpr
}

// dependsOnPercent reports whether the resolved value would change with
// available width — i.e. a percentage appears somewhere in a calc, min,
// max, or clamp tree. Plain Unit == "%" lengths take the Pct(...) path
// in cssLengthToUnitValue and don't need this check.
func (l *cssLength) dependsOnPercent() bool {
	if l == nil {
		return false
	}
	if l.calc != nil {
		return l.calc.dependsOnPercent()
	}
	for _, a := range l.minArgs {
		if a.Unit == "%" || a.dependsOnPercent() {
			return true
		}
	}
	for _, a := range l.maxArgs {
		if a.Unit == "%" || a.dependsOnPercent() {
			return true
		}
	}
	return false
}

func (e *calcExpr) dependsOnPercent() bool {
	if e == nil {
		return false
	}
	if e.leaf != nil {
		if e.leaf.Unit == "%" {
			return true
		}
		return e.leaf.dependsOnPercent()
	}
	if e.left == nil || e.right == nil {
		return false
	}
	return e.left.dependsOnPercent() || e.right.dependsOnPercent()
}

// isDimensionless reports whether the resolved value carries no length
// dimension — i.e. every leaf is a bare number stored with Unit "num".
// Used by parseLineHeight to distinguish a dimensionless calc
// (e.g. `calc(1.2 * 1.5)` → multiplier 1.8) from a length-typed calc
// (e.g. `calc(1em + 4px)` → length, divide by fontSize for the
// multiplier). A dimensionless cssLength can be resolved with
// toPoints(0, 0) since the only leaf branch consulted is the "num"
// case, which returns Value as-is.
func (l *cssLength) isDimensionless() bool {
	if l == nil {
		return false
	}
	if l.calc != nil {
		return l.calc.isDimensionless()
	}
	if len(l.minArgs) > 0 {
		for _, a := range l.minArgs {
			if !a.isDimensionless() {
				return false
			}
		}
		return true
	}
	if len(l.maxArgs) > 0 {
		for _, a := range l.maxArgs {
			if !a.isDimensionless() {
				return false
			}
		}
		return true
	}
	return l.Unit == "num"
}

func (e *calcExpr) isDimensionless() bool {
	if e == nil {
		return false
	}
	if e.leaf != nil {
		return e.leaf.isDimensionless()
	}
	if e.left == nil || e.right == nil {
		return false
	}
	return e.left.isDimensionless() && e.right.isDimensionless()
}

// resolve evaluates a calcExpr to points.
func (e *calcExpr) resolve(relativeTo, fontSize float64) float64 {
	if e.leaf != nil {
		return e.leaf.toPoints(relativeTo, fontSize)
	}
	l := e.left.resolve(relativeTo, fontSize)
	r := e.right.resolve(relativeTo, fontSize)
	switch e.op {
	case calcAdd:
		return l + r
	case calcSub:
		return l - r
	case calcMul:
		return l * r
	case calcDiv:
		if r != 0 {
			return l / r
		}
		return 0
	}
	return 0
}

// Resolve satisfies layout.ResolvableLength so cssLength values can feed
// background-position lazy resolution at draw time. It is a thin alias
// for toPoints; container maps to the percent-resolution dimension and
// fontSize feeds em / rem leaves. See layout.ResolvableLength for the
// background-position semantic (container is (box - image) on each axis).
func (l *cssLength) Resolve(container, fontSize float64) float64 {
	return l.toPoints(container, fontSize)
}

// toPoints converts a CSS length to PDF points.
// relativeTo is used for percentage values.
func (l *cssLength) toPoints(relativeTo, fontSize float64) float64 {
	if l == nil {
		return 0
	}
	if l.calc != nil {
		return l.calc.resolve(relativeTo, fontSize)
	}
	if len(l.minArgs) > 0 {
		result := l.minArgs[0].toPoints(relativeTo, fontSize)
		for _, arg := range l.minArgs[1:] {
			if v := arg.toPoints(relativeTo, fontSize); v < result {
				result = v
			}
		}
		return result
	}
	if len(l.maxArgs) > 0 {
		result := l.maxArgs[0].toPoints(relativeTo, fontSize)
		for _, arg := range l.maxArgs[1:] {
			if v := arg.toPoints(relativeTo, fontSize); v > result {
				result = v
			}
		}
		return result
	}
	switch l.Unit {
	case "pt":
		return l.Value
	case "px":
		return l.Value * 0.75 // 96dpi screen → 72dpi print
	case "em":
		return l.Value * fontSize
	case "rem":
		return l.Value * 16 * 0.75 // assume 16px root
	case "%":
		return l.Value / 100 * relativeTo
	case "mm":
		return l.Value * 72 / 25.4
	case "cm":
		return l.Value * 72 / 2.54
	case "in":
		return l.Value * 72
	case "num":
		return l.Value // dimensionless number (used in calc * and /)
	default:
		return l.Value * 0.75 // default to px
	}
}

// defaultStyle returns browser-like defaults.
func defaultStyle() computedStyle {
	return computedStyle{
		FontFamily:     "helvetica",
		FontSize:       12, // 16px * 0.75 = 12pt
		FontWeight:     400,
		FontStyle:      "normal",
		Color:          layout.ColorBlack,
		TextAlign:      layout.AlignLeft,
		TextDecoration: layout.DecorationNone,
		LineHeight:     1.2,
		Display:        "block",
		FlexDirection:  "row",
		JustifyContent: "flex-start",
		AlignItems:     "stretch",
		FlexWrap:       "nowrap",
		FlexShrink:     1,
		ListStyleType:  "disc",
	}
}

// inherit creates a child style that inherits text properties from the parent.
//
// Every inherited property must carry its "was declared" flag alongside its
// value: consumers guard on the flag, so a value that arrives without it is
// held but never applied. TextAlignSelfSet is the one deliberate exception —
// it means "declared on THIS element" and must NOT propagate.
func (s *computedStyle) inherit() computedStyle {
	child := computedStyle{
		FontFamily:           s.FontFamily,
		FontSize:             s.FontSize,
		FontWeight:           s.FontWeight,
		FontStyle:            s.FontStyle,
		Color:                s.Color,
		TextAlign:            s.TextAlign,
		TextAlignSet:         s.TextAlignSet,
		TextAlignKeyword:     s.TextAlignKeyword,
		TextAlignLast:        s.TextAlignLast,
		TextAlignLastSet:     s.TextAlignLastSet,
		TextAlignLastKeyword: s.TextAlignLastKeyword,
		TextDecoration:       s.TextDecoration,
		TextTransform:        s.TextTransform,
		WhiteSpace:           s.WhiteSpace,
		LineHeight:           s.LineHeight,
		LetterSpacing:        s.LetterSpacing,
		WordSpacing:          s.WordSpacing,
		TextIndent:           s.TextIndent,
		Direction:            s.Direction, // CSS direction is inherited
		Display:              "block",
		FlexDirection:        "row",
		JustifyContent:       "flex-start",
		AlignItems:           "stretch",
		FlexWrap:             "nowrap",
		FlexShrink:           1,
		ListStyleType:        s.ListStyleType,
		Visibility:           s.Visibility,
		WordBreak:            s.WordBreak,
		Hyphens:              s.Hyphens,
	}
	// CSS custom properties inherit: deep-copy the map.
	if len(s.CustomProperties) > 0 {
		child.CustomProperties = make(map[string]string, len(s.CustomProperties))
		for k, v := range s.CustomProperties {
			child.CustomProperties[k] = v
		}
	}
	return child
}

// hasPadding returns true if any padding side is declared with a
// meaningful (non-zero, or container-dependent) value. Returns false
// for an absent declaration AND for explicit-zero declarations like
// `padding: 0` or `padding: 0%`, which add no layout effect and
// should not trigger wrapper-emission code paths.
func (s *computedStyle) hasPadding() bool {
	return hasMeaningfulLength(s.PaddingTopLength) ||
		hasMeaningfulLength(s.PaddingRightLength) ||
		hasMeaningfulLength(s.PaddingBottomLength) ||
		hasMeaningfulLength(s.PaddingLeftLength)
}

// hasBorder returns true if any border is set.
func (s *computedStyle) hasBorder() bool {
	return s.BorderTopWidth > 0 || s.BorderRightWidth > 0 || s.BorderBottomWidth > 0 || s.BorderLeftWidth > 0
}

// hasMargin returns true if any margin side is declared with a
// meaningful (non-zero, or container-dependent) value. See hasPadding
// for the non-zero-declaration rationale.
func (s *computedStyle) hasMargin() bool {
	return hasMeaningfulLength(s.MarginTopLength) ||
		hasMeaningfulLength(s.MarginRightLength) ||
		hasMeaningfulLength(s.MarginBottomLength) ||
		hasMeaningfulLength(s.MarginLeftLength)
}

// hasMeaningfulLength reports whether a *cssLength sibling carries a
// declaration that would resolve to a non-zero value for some
// container width — distinct from "no declaration" (nil) and from
// "explicit zero declaration" (`0` / `0%` / `calc(0)`).
//
// A plain length leaf is meaningful when Value != 0. A calc / min /
// max / clamp tree is meaningful conservatively — we cannot determine
// at parse time whether the expression evaluates to non-zero, so we
// assume it does. Callers gate wrapper emission on this, and a
// spurious wrapper is preferable to a missing one.
func hasMeaningfulLength(l *cssLength) bool {
	if l == nil {
		return false
	}
	if l.calc != nil || l.minArgs != nil || l.maxArgs != nil {
		return true
	}
	return l.Value != 0
}

// MarginTopAt resolves the top margin against containerWidth. The
// stored *cssLength carries the unresolved percent / calc / min /
// max / clamp tree from the parser; resolution against the actual
// containing block happens here. A nil sibling means the side was
// not declared and the helper returns 0.
//
// containerWidth is the width of the containing block, in points.
// Consumers that don't know the container yet (e.g. probe paths
// pre-flow) may pass 0 and accept that percent declarations resolve
// to 0pt.
func (s *computedStyle) MarginTopAt(containerWidth float64) float64 {
	return resolveBoxSide(s.MarginTopLength, containerWidth, s.FontSize)
}

// MarginRightAt — see MarginTopAt.
func (s *computedStyle) MarginRightAt(containerWidth float64) float64 {
	return resolveBoxSide(s.MarginRightLength, containerWidth, s.FontSize)
}

// MarginBottomAt — see MarginTopAt.
func (s *computedStyle) MarginBottomAt(containerWidth float64) float64 {
	return resolveBoxSide(s.MarginBottomLength, containerWidth, s.FontSize)
}

// MarginLeftAt — see MarginTopAt.
func (s *computedStyle) MarginLeftAt(containerWidth float64) float64 {
	return resolveBoxSide(s.MarginLeftLength, containerWidth, s.FontSize)
}

// PaddingTopAt — see MarginTopAt. Same contract for padding.
func (s *computedStyle) PaddingTopAt(containerWidth float64) float64 {
	return resolveBoxSide(s.PaddingTopLength, containerWidth, s.FontSize)
}

// PaddingRightAt — see MarginTopAt.
func (s *computedStyle) PaddingRightAt(containerWidth float64) float64 {
	return resolveBoxSide(s.PaddingRightLength, containerWidth, s.FontSize)
}

// PaddingBottomAt — see MarginTopAt.
func (s *computedStyle) PaddingBottomAt(containerWidth float64) float64 {
	return resolveBoxSide(s.PaddingBottomLength, containerWidth, s.FontSize)
}

// PaddingLeftAt — see MarginTopAt.
func (s *computedStyle) PaddingLeftAt(containerWidth float64) float64 {
	return resolveBoxSide(s.PaddingLeftLength, containerWidth, s.FontSize)
}

// resolveBoxSide resolves a margin / padding side against the
// supplied containing-block width. A nil length means the side was
// not declared and resolves to 0.
func resolveBoxSide(length *cssLength, containerWidth, fontSize float64) float64 {
	if length == nil {
		return 0
	}
	return length.toPoints(containerWidth, fontSize)
}
