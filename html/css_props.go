// Copyright 2026 Carlos Munoz and the Folio Authors
// SPDX-License-Identifier: Apache-2.0

package html

import (
	"strconv"
	"strings"

	"github.com/carlos7ags/folio/layout"
)

// cssProperty is a declarative descriptor for a single CSS property
// the parser knows about. The parser dispatches via cssPropByName when
// applying a declaration; the same registry is the source of truth for
// docs/CSS_SUPPORT.md (generated, see internal/gen-css-docs).
//
// All fields except Name and Apply are documentation-only — they have
// no effect on parsing.
//
// Carve-out: the `content` property is NOT a candidate for this
// registry. It is intercepted in pseudo-element generation
// (parsePseudoContent) where the html.Node is in scope; it never
// flows through applyProperty. Do not add a `content` entry here.
type cssProperty struct {
	Name     string                               // canonical CSS name, e.g. "letter-spacing"
	Aliases  []string                             // alternative names handled identically (e.g. "-webkit-hyphens", "grid-gap")
	Category string                               // one of: Typography, Color, Backgrounds, BoxModel, Borders, Layout, Flexbox, Grid, MultiColumn, Tables, Pagination, Lists, Effects, PDF
	Values   []string                             // accepted value forms, for docs (e.g. ["<length>", "normal"])
	Notes    string                               // human-readable caveat for docs
	Apply    func(s *computedStyle, value string) // mutate s based on value; silently ignore invalid input
}

// cssProperties is the registry of all CSS properties Folio's HTML
// converter knows about. To add a new property, append a cssProperty
// here and the parser will dispatch to it automatically. The order in
// this slice does not affect parsing; docs are sorted at generation
// time by Category then Name.
//
// Pilot batch (#260): 8 entries. Bulk migration in subsequent PRs.
var cssProperties = []cssProperty{
	{
		Name:     "color",
		Category: "Color",
		Values:   []string{"<named>", "<hex>", "rgb()", "rgba()", "hsl()", "hsla()", "cmyk()"},
		Notes:    "sRGB only. oklch() and color-mix() are not supported.",
		Apply: func(s *computedStyle, value string) {
			if c, ok := parseColor(value); ok {
				s.Color = c
			}
		},
	},
	{
		Name:     "letter-spacing",
		Category: "Typography",
		Values:   []string{"<length>", "normal"},
		Apply: func(s *computedStyle, value string) {
			if l := parseLength(value); l != nil {
				s.LetterSpacing = l.toPoints(0, s.FontSize)
			} else if strings.TrimSpace(strings.ToLower(value)) == "normal" {
				s.LetterSpacing = 0
			}
		},
	},
	{
		Name:     "word-spacing",
		Category: "Typography",
		Values:   []string{"<length>", "normal"},
		Apply: func(s *computedStyle, value string) {
			if l := parseLength(value); l != nil {
				s.WordSpacing = l.toPoints(0, s.FontSize)
			} else if strings.TrimSpace(strings.ToLower(value)) == "normal" {
				s.WordSpacing = 0
			}
		},
	},
	{
		Name:     "text-transform",
		Category: "Typography",
		Values:   []string{"uppercase", "lowercase", "capitalize", "none"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "uppercase" || v == "lowercase" || v == "capitalize" || v == "none" {
				s.TextTransform = v
			}
		},
	},
	{
		Name:     "text-align",
		Category: "Typography",
		Values:   []string{"left", "right", "center", "justify", "start", "end"},
		Apply: func(s *computedStyle, value string) {
			if a, kw, ok := parseTextAlign(value); ok {
				s.TextAlign = a
				s.TextAlignKeyword = kw
				s.TextAlignSet = true
				s.TextAlignSelfSet = true
			}
		},
	},
	{
		Name:     "white-space",
		Category: "Typography",
		Values:   []string{"normal", "nowrap", "pre", "pre-wrap", "pre-line"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "normal" || v == "nowrap" || v == "pre" || v == "pre-wrap" || v == "pre-line" {
				s.WhiteSpace = v
			}
		},
	},
	{
		Name:     "direction",
		Category: "Typography",
		Values:   []string{"ltr", "rtl"},
		Notes:    "Interacts with unicode-bidi; together they control bidi paragraph direction.",
		Apply: func(s *computedStyle, value string) {
			switch strings.TrimSpace(strings.ToLower(value)) {
			case "rtl":
				s.Direction = layout.DirectionRTL
			case "ltr":
				s.Direction = layout.DirectionLTR
			}
		},
	},
	{
		Name:     "opacity",
		Category: "Effects",
		Values:   []string{"<number 0..1>"},
		Notes:    "Values outside 0..1 are clamped.",
		Apply: func(s *computedStyle, value string) {
			v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil {
				return
			}
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
			s.Opacity = v
		},
	},

	// --- Typography (Phase 2 batch 1) -----------------------------------

	{
		Name: "font-family", Category: "Typography",
		Values: []string{"<family-name>", "<generic-family>"},
		Apply: func(s *computedStyle, value string) {
			s.FontFamily = parseFontFamily(value)
		},
	},
	{
		Name: "font-size", Category: "Typography",
		Values: []string{"<absolute-size>", "<relative-size>", "<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			s.FontSize = parseFontSize(value, s.FontSize)
		},
	},
	{
		Name: "font-weight", Category: "Typography",
		Values: []string{"normal", "bold", "bolder", "lighter", "<integer 100..900>"},
		Apply: func(s *computedStyle, value string) {
			// At property-application time s.FontWeight already holds
			// the inherited value (parent's resolved weight, or the
			// default 400). bolder/lighter resolve against it.
			s.FontWeight = parseFontWeight(value, s.FontWeight)
		},
	},
	{
		Name: "font-style", Category: "Typography",
		Values: []string{"normal", "italic", "oblique"},
		Apply: func(s *computedStyle, value string) {
			s.FontStyle = parseFontStyle(value)
		},
	},
	{
		Name: "text-decoration", Category: "Typography",
		Values: []string{"none", "underline", "overline", "line-through", "blink"},
		Apply: func(s *computedStyle, value string) {
			s.TextDecoration = parseTextDecoration(value)
		},
	},
	{
		Name: "text-decoration-color", Category: "Typography",
		Values: []string{"<color>"},
		Apply: func(s *computedStyle, value string) {
			if c, ok := parseColor(value); ok {
				s.TextDecorationColor = &c
			}
		},
	},
	{
		Name: "text-decoration-style", Category: "Typography",
		Values: []string{"solid", "dashed", "dotted", "double", "wavy"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "solid" || v == "dashed" || v == "dotted" || v == "double" || v == "wavy" {
				s.TextDecorationStyle = v
			}
		},
	},
	{
		Name: "text-indent", Category: "Typography",
		Values: []string{"<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			if l := parseLength(value); l != nil {
				s.TextIndent = l.toPoints(0, s.FontSize)
			}
		},
	},
	{
		Name: "text-shadow", Category: "Typography",
		Values: []string{"<offset-x> <offset-y> [<blur>] [<color>]", "none"},
		Apply: func(s *computedStyle, value string) {
			s.TextShadow = parseBoxShadow(strings.TrimSpace(strings.ToLower(value)), s.FontSize)
		},
	},
	{
		Name: "word-break", Category: "Typography",
		Values: []string{"normal", "break-all", "keep-all", "break-word"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "normal" || v == "break-all" || v == "keep-all" || v == "break-word" {
				s.WordBreak = v
			}
		},
	},
	{
		Name: "hyphens", Aliases: []string{"-webkit-hyphens"}, Category: "Typography",
		Values: []string{"none", "manual", "auto"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "none" || v == "manual" || v == "auto" {
				s.Hyphens = v
			}
		},
	},
	{
		Name: "unicode-bidi", Category: "Typography",
		Values: []string{"normal", "embed", "bidi-override", "isolate", "isolate-override", "plaintext"},
		Notes:  "Interacts with direction; together they control bidi paragraph layout.",
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "normal" || v == "embed" || v == "bidi-override" || v == "isolate" ||
				v == "isolate-override" || v == "plaintext" {
				s.UnicodeBidi = v
			}
		},
	},
	{
		Name: "line-height", Category: "Typography",
		Values: []string{"<number>", "<length>", "<percentage>", "normal"},
		Apply: func(s *computedStyle, value string) {
			s.LineHeight = parseLineHeight(value, s.FontSize)
		},
	},
	{
		Name: "text-align-last", Category: "Typography",
		Values: []string{"left", "right", "center", "justify", "start", "end"},
		Apply: func(s *computedStyle, value string) {
			if a, kw, ok := parseTextAlign(value); ok {
				s.TextAlignLast = a
				s.TextAlignLastKeyword = kw
				s.TextAlignLastSet = true
			}
		},
	},

	// --- BoxModel + Layout (Phase 2 batch 2) ----------------------------

	{
		Name: "display", Category: "Layout",
		Values: []string{"block", "inline", "inline-block", "flex", "grid", "table", "table-row", "table-cell", "list-item", "none"},
		Apply: func(s *computedStyle, value string) {
			s.Display = parseDisplay(value)
		},
	},
	{
		Name: "width", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>", "auto"},
		Apply: func(s *computedStyle, value string) {
			s.Width = parseLength(value)
		},
	},
	{
		Name: "height", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>", "auto"},
		Apply: func(s *computedStyle, value string) {
			s.Height = parseLength(value)
		},
	},
	{
		Name: "min-width", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			s.MinWidth = parseLength(value)
		},
	},
	{
		Name: "max-width", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>", "none"},
		Apply: func(s *computedStyle, value string) {
			s.MaxWidth = parseLength(value)
		},
	},
	{
		Name: "min-height", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			s.MinHeight = parseLength(value)
		},
	},
	{
		Name: "max-height", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>", "none"},
		Apply: func(s *computedStyle, value string) {
			s.MaxHeight = parseLength(value)
		},
	},
	{
		Name: "aspect-ratio", Category: "BoxModel",
		Values: []string{"<ratio>", "auto"},
		Apply: func(s *computedStyle, value string) {
			s.AspectRatio = parseAspectRatio(value)
		},
	},
	{
		Name: "padding-top", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			s.PaddingTopLength = parseBoxSideLength(value, s.FontSize)
		},
	},
	{
		Name: "padding-right", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			s.PaddingRightLength = parseBoxSideLength(value, s.FontSize)
		},
	},
	{
		Name: "padding-bottom", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			s.PaddingBottomLength = parseBoxSideLength(value, s.FontSize)
		},
	},
	{
		Name: "padding-left", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			s.PaddingLeftLength = parseBoxSideLength(value, s.FontSize)
		},
	},
	{
		Name: "margin-bottom", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>"},
		Notes:  "margin-top, margin-left, margin-right also accept `auto`; margin-bottom does not.",
		Apply: func(s *computedStyle, value string) {
			s.MarginBottomLength = parseBoxSideLength(value, s.FontSize)
		},
	},
	{
		Name: "position", Category: "Layout",
		Values: []string{"static", "relative", "absolute", "fixed"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "static" || v == "relative" || v == "absolute" || v == "fixed" {
				s.Position = v
			}
		},
	},
	{
		Name: "top", Category: "Layout",
		Values: []string{"<length>", "<percentage>", "auto"},
		Apply: func(s *computedStyle, value string) {
			s.Top = parseLength(value)
		},
	},
	{
		Name: "right", Category: "Layout",
		Values: []string{"<length>", "<percentage>", "auto"},
		Apply: func(s *computedStyle, value string) {
			s.Right = parseLength(value)
		},
	},
	{
		Name: "bottom", Category: "Layout",
		Values: []string{"<length>", "<percentage>", "auto"},
		Apply: func(s *computedStyle, value string) {
			s.Bottom = parseLength(value)
		},
	},
	{
		Name: "left", Category: "Layout",
		Values: []string{"<length>", "<percentage>", "auto"},
		Apply: func(s *computedStyle, value string) {
			s.Left = parseLength(value)
		},
	},
	{
		Name: "z-index", Category: "Layout",
		Values: []string{"<integer>", "auto"},
		Apply: func(s *computedStyle, value string) {
			if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				s.ZIndex = v
				s.ZIndexSet = true
			}
		},
	},
	{
		Name: "overflow", Category: "Layout",
		Values: []string{"hidden", "visible", "auto", "scroll"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "hidden" || v == "visible" || v == "auto" || v == "scroll" {
				s.Overflow = v
			}
		},
	},
	{
		Name: "float", Category: "Layout",
		Values: []string{"left", "right", "none"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "left" || v == "right" || v == "none" {
				s.Float = v
			}
		},
	},
	{
		Name: "clear", Category: "Layout",
		Values: []string{"left", "right", "both", "none"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "left" || v == "right" || v == "both" || v == "none" {
				s.Clear = v
			}
		},
	},
	{
		Name: "box-sizing", Category: "Layout",
		Values: []string{"content-box", "border-box"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "content-box" || v == "border-box" {
				s.BoxSizing = v
			}
		},
	},
	{
		Name: "visibility", Category: "Layout",
		Values: []string{"visible", "hidden", "collapse"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "visible" || v == "hidden" || v == "collapse" {
				s.Visibility = v
			}
		},
	},
	{
		Name: "text-overflow", Category: "Effects",
		Values: []string{"clip", "ellipsis"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "clip" || v == "ellipsis" {
				s.TextOverflow = v
			}
		},
	},

	// --- Backgrounds (Phase 2 batch 3) ----------------------------------

	{
		Name: "background-color", Category: "Backgrounds",
		Values: []string{"<color>"},
		Apply: func(s *computedStyle, value string) {
			if c, ok := parseColor(value); ok {
				s.BackgroundColor = &c
			}
		},
	},
	{
		Name: "background-image", Category: "Backgrounds",
		Values: []string{"<gradient>", "url(...)", "none"},
		Apply: func(s *computedStyle, value string) {
			s.BackgroundImage = strings.TrimSpace(value)
		},
	},
	{
		Name: "background-size", Category: "Backgrounds",
		Values: []string{"<length>", "<percentage>", "auto", "cover", "contain"},
		Apply: func(s *computedStyle, value string) {
			s.BackgroundSize = strings.TrimSpace(strings.ToLower(value))
		},
	},
	{
		Name: "background-position", Category: "Backgrounds",
		Values: []string{"<position>"},
		Apply: func(s *computedStyle, value string) {
			s.BackgroundPosition = strings.TrimSpace(strings.ToLower(value))
		},
	},
	{
		Name: "background-repeat", Category: "Backgrounds",
		Values: []string{"repeat", "repeat-x", "repeat-y", "no-repeat", "space", "round"},
		Apply: func(s *computedStyle, value string) {
			s.BackgroundRepeat = strings.TrimSpace(strings.ToLower(value))
		},
	},

	// --- Borders simple (Phase 2 batch 3) -------------------------------

	{
		Name: "border-top-width", Category: "Borders",
		Values: []string{"<length>", "thin", "medium", "thick"},
		Apply: func(s *computedStyle, value string) {
			s.BorderTopWidth = parseBoxSide(value, s.FontSize)
		},
	},
	{
		Name: "border-right-width", Category: "Borders",
		Values: []string{"<length>", "thin", "medium", "thick"},
		Apply: func(s *computedStyle, value string) {
			s.BorderRightWidth = parseBoxSide(value, s.FontSize)
		},
	},
	{
		Name: "border-bottom-width", Category: "Borders",
		Values: []string{"<length>", "thin", "medium", "thick"},
		Apply: func(s *computedStyle, value string) {
			s.BorderBottomWidth = parseBoxSide(value, s.FontSize)
		},
	},
	{
		Name: "border-left-width", Category: "Borders",
		Values: []string{"<length>", "thin", "medium", "thick"},
		Apply: func(s *computedStyle, value string) {
			s.BorderLeftWidth = parseBoxSide(value, s.FontSize)
		},
	},
	{
		Name: "border-top-left-radius", Category: "Borders",
		Values: []string{"<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			s.BorderRadiusTL, s.BorderRadiusTLPct = parseRadiusComponent(value, s.FontSize)
		},
	},
	{
		Name: "border-top-right-radius", Category: "Borders",
		Values: []string{"<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			s.BorderRadiusTR, s.BorderRadiusTRPct = parseRadiusComponent(value, s.FontSize)
		},
	},
	{
		Name: "border-bottom-right-radius", Category: "Borders",
		Values: []string{"<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			s.BorderRadiusBR, s.BorderRadiusBRPct = parseRadiusComponent(value, s.FontSize)
		},
	},
	{
		Name: "border-bottom-left-radius", Category: "Borders",
		Values: []string{"<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			s.BorderRadiusBL, s.BorderRadiusBLPct = parseRadiusComponent(value, s.FontSize)
		},
	},

	// --- Tables (Phase 2 batch 3) ---------------------------------------

	{
		Name: "border-collapse", Category: "Tables",
		Values: []string{"collapse", "separate"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "collapse" || v == "separate" {
				s.BorderCollapse = v
			}
		},
	},

	// --- Pagination (Phase 2 batch 3) -----------------------------------

	{
		Name: "page-break-before", Aliases: []string{"break-before"}, Category: "Pagination",
		Values: []string{"always", "page", "avoid", "avoid-page", "auto"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			switch v {
			case "always", "page":
				s.PageBreakBefore = "always"
			case "avoid", "avoid-page":
				s.PageBreakBefore = "avoid"
			case "auto":
				s.PageBreakBefore = "auto"
			}
		},
	},
	{
		Name: "page-break-after", Aliases: []string{"break-after"}, Category: "Pagination",
		Values: []string{"always", "page", "avoid", "avoid-page", "auto"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			switch v {
			case "always", "page":
				s.PageBreakAfter = "always"
			case "avoid", "avoid-page":
				s.PageBreakAfter = "avoid"
			case "auto":
				s.PageBreakAfter = "auto"
			}
		},
	},
	{
		Name: "page-break-inside", Aliases: []string{"break-inside"}, Category: "Pagination",
		Values: []string{"avoid", "avoid-page", "auto"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			switch v {
			case "avoid", "avoid-page":
				s.PageBreakInside = "avoid"
			case "auto":
				s.PageBreakInside = "auto"
			}
		},
	},
	{
		Name: "orphans", Category: "Pagination",
		Values: []string{"<integer>"},
		Apply: func(s *computedStyle, value string) {
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n > 0 {
				s.Orphans = n
			}
		},
	},
	{
		Name: "widows", Category: "Pagination",
		Values: []string{"<integer>"},
		Apply: func(s *computedStyle, value string) {
			if n, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && n > 0 {
				s.Widows = n
			}
		},
	},

	// --- Lists (Phase 2 batch 3) ----------------------------------------

	{
		Name: "list-style-type", Aliases: []string{"list-style"}, Category: "Lists",
		Values: []string{"disc", "circle", "square", "decimal", "lower-alpha", "upper-alpha", "lower-roman", "upper-roman", "none"},
		Notes:  "list-style is a shorthand; type and position tokens are extracted.",
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			parts := strings.Fields(v)
			typeSet := false
			for _, p := range parts {
				switch p {
				case "inside", "outside":
					s.ListStylePosition = p
				default:
					// First non-position, non-url token is the marker type.
					// A url(...) token is a marker image, not a type keyword.
					if !typeSet && !strings.HasPrefix(p, "url(") {
						s.ListStyleType = p
						typeSet = true
					}
				}
			}
		},
	},
	{
		Name: "list-style-position", Category: "Lists",
		Values: []string{"inside", "outside"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			switch v {
			case "inside", "outside":
				s.ListStylePosition = v
			}
		},
	},

	// --- Effects (Phase 2 batch 3) --------------------------------------

	{
		Name: "transform", Category: "Effects",
		Values: []string{"<transform-function>+", "none"},
		Apply: func(s *computedStyle, value string) {
			s.Transform = strings.TrimSpace(value)
		},
	},
	{
		Name: "transform-origin", Category: "Effects",
		Values: []string{"<position>"},
		Apply: func(s *computedStyle, value string) {
			s.TransformOrigin = strings.TrimSpace(value)
		},
	},
	{
		Name: "object-fit", Category: "Effects",
		Values: []string{"contain", "cover", "fill", "none", "scale-down"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			switch v {
			case "contain", "cover", "fill", "none", "scale-down":
				s.ObjectFit = v
			}
		},
	},
	{
		Name: "object-position", Category: "Effects",
		Values: []string{"<position>"},
		Apply: func(s *computedStyle, value string) {
			s.ObjectPosition = strings.TrimSpace(strings.ToLower(value))
		},
	},
	{
		Name: "outline-width", Category: "Effects",
		Values: []string{"<length>", "thin", "medium", "thick"},
		Apply: func(s *computedStyle, value string) {
			s.OutlineWidth = parseBoxSide(value, s.FontSize)
		},
	},
	{
		Name: "outline-style", Category: "Effects",
		Values: []string{"solid", "dashed", "dotted", "double", "none"},
		Apply: func(s *computedStyle, value string) {
			s.OutlineStyle = strings.TrimSpace(strings.ToLower(value))
		},
	},
	{
		Name: "outline-color", Category: "Effects",
		Values: []string{"<color>"},
		Apply: func(s *computedStyle, value string) {
			if c, ok := parseColor(value); ok {
				s.OutlineColor = c
			}
		},
	},
	{
		Name: "outline-offset", Category: "Effects",
		Values: []string{"<length>"},
		Apply: func(s *computedStyle, value string) {
			s.OutlineOffset = parseBoxSide(value, s.FontSize)
		},
	},
	{
		Name: "box-shadow", Category: "Effects",
		Values: []string{"<offset-x> <offset-y> [<blur>] [<spread>] [<color>] [inset]", "none"},
		Notes:  "Supports comma-separated multiple shadows.",
		Apply: func(s *computedStyle, value string) {
			s.BoxShadows = parseBoxShadows(value, s.FontSize)
		},
	},

	// --- PDF-specific (Phase 2 batch 3) ---------------------------------

	{
		Name: "counter-reset", Category: "PDF",
		Values: []string{"<identifier> [<integer>]+"},
		Apply: func(s *computedStyle, value string) {
			s.CounterReset = parseCounterEntries(value, 0)
		},
	},
	{
		Name: "counter-increment", Category: "PDF",
		Values: []string{"<identifier> [<integer>]+"},
		Apply: func(s *computedStyle, value string) {
			s.CounterIncrement = parseCounterEntries(value, 1)
		},
	},
	{
		Name: "bookmark-level", Category: "PDF",
		Values: []string{"<integer 1..6>", "none"},
		Notes:  "Per CSS GCPM. Levels are clamped to Folio's H1-H6 range.",
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "none" {
				s.BookmarkLevel = -1
				s.BookmarkLevelSet = true
			} else if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 6 {
				s.BookmarkLevel = n
				s.BookmarkLevelSet = true
			}
		},
	},
	{
		Name: "bookmark-label", Category: "PDF",
		Values: []string{"<string>", "content()", "attr(<identifier>)"},
		Notes:  "content() and attr() are resolved at element-conversion time.",
		Apply: func(s *computedStyle, value string) {
			s.BookmarkLabel = strings.TrimSpace(value)
		},
	},
	{
		Name: "bookmark-state", Category: "PDF",
		Values: []string{"open", "closed"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "open" || v == "closed" {
				s.BookmarkState = v
			}
		},
	},
	{
		Name: "string-set", Category: "PDF",
		Values: []string{"<identifier> <content-list>"},
		Notes:  "Used by @page margin boxes for running headers/footers.",
		Apply: func(s *computedStyle, value string) {
			parts := strings.Fields(strings.TrimSpace(value))
			if len(parts) >= 2 {
				s.StringSetName = parts[0]
				s.StringSetValue = strings.Join(parts[1:], " ")
			}
		},
	},

	// --- Flexbox (Phase 2 batch 4) --------------------------------------

	{
		Name: "flex-direction", Category: "Flexbox",
		Values: []string{"row", "row-reverse", "column", "column-reverse"},
		Apply: func(s *computedStyle, value string) {
			s.FlexDirection = strings.TrimSpace(strings.ToLower(value))
		},
	},
	{
		Name: "flex-wrap", Category: "Flexbox",
		Values: []string{"nowrap", "wrap", "wrap-reverse"},
		Apply: func(s *computedStyle, value string) {
			s.FlexWrap = strings.TrimSpace(strings.ToLower(value))
		},
	},
	{
		Name: "flex-grow", Category: "Flexbox",
		Values: []string{"<number>"},
		Apply: func(s *computedStyle, value string) {
			if v, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				s.FlexGrow = v
			}
		},
	},
	{
		Name: "flex-shrink", Category: "Flexbox",
		Values: []string{"<number>"},
		Apply: func(s *computedStyle, value string) {
			if v, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				s.FlexShrink = v
			}
		},
	},
	{
		Name: "flex-basis", Category: "Flexbox",
		Values: []string{"<length>", "<percentage>", "auto", "content"},
		Apply: func(s *computedStyle, value string) {
			s.FlexBasis = parseLength(value)
		},
	},
	{
		Name: "justify-content", Category: "Flexbox",
		Values: []string{"flex-start", "flex-end", "center", "space-between", "space-around", "space-evenly", "start", "end"},
		Apply: func(s *computedStyle, value string) {
			s.JustifyContent = strings.TrimSpace(strings.ToLower(value))
		},
	},
	{
		Name: "align-items", Category: "Flexbox",
		Values: []string{"stretch", "flex-start", "flex-end", "center", "baseline", "start", "end"},
		Apply: func(s *computedStyle, value string) {
			s.AlignItems = strings.TrimSpace(strings.ToLower(value))
		},
	},
	{
		Name: "align-self", Category: "Flexbox",
		Values: []string{"auto", "stretch", "flex-start", "flex-end", "center", "baseline"},
		Apply: func(s *computedStyle, value string) {
			s.AlignSelf = strings.TrimSpace(strings.ToLower(value))
		},
	},
	{
		Name: "order", Category: "Flexbox",
		Values: []string{"<integer>"},
		Apply: func(s *computedStyle, value string) {
			if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				s.Order = v
			}
		},
	},

	// --- Grid (Phase 2 batch 4) -----------------------------------------

	{
		Name: "row-gap", Category: "Grid",
		Values: []string{"<length>", "<percentage>", "normal"},
		Apply: func(s *computedStyle, value string) {
			s.RowGap = parseBoxSide(value, s.FontSize)
		},
	},
	{
		Name: "grid-template-columns", Category: "Grid",
		Values: []string{"<track-list>", "none"},
		Apply: func(s *computedStyle, value string) {
			s.GridTemplateColumns = strings.TrimSpace(value)
		},
	},
	{
		Name: "grid-template-rows", Category: "Grid",
		Values: []string{"<track-list>", "none"},
		Apply: func(s *computedStyle, value string) {
			s.GridTemplateRows = strings.TrimSpace(value)
		},
	},
	{
		Name: "grid-template-areas", Category: "Grid",
		Values: []string{"<string>+", "none"},
		Apply: func(s *computedStyle, value string) {
			s.GridTemplateAreas = parseGridTemplateAreas(value)
		},
	},
	{
		Name: "grid-area", Category: "Grid",
		Values: []string{"<grid-line> [/ <grid-line>]{0..3}"},
		Apply: func(s *computedStyle, value string) {
			s.GridArea = strings.TrimSpace(value)
		},
	},
	{
		Name: "grid-column", Category: "Grid",
		Values: []string{"<grid-line> [/ <grid-line>]?"},
		Apply: func(s *computedStyle, value string) {
			s.GridColumnStart, s.GridColumnEnd = parseGridLine(value)
		},
	},
	{
		Name: "grid-row", Category: "Grid",
		Values: []string{"<grid-line> [/ <grid-line>]?"},
		Apply: func(s *computedStyle, value string) {
			s.GridRowStart, s.GridRowEnd = parseGridLine(value)
		},
	},
	{
		Name: "grid-column-start", Category: "Grid",
		Values: []string{"<integer>"},
		Apply: func(s *computedStyle, value string) {
			if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				s.GridColumnStart = v
			}
		},
	},
	{
		Name: "grid-column-end", Category: "Grid",
		Values: []string{"<integer>"},
		Apply: func(s *computedStyle, value string) {
			if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				s.GridColumnEnd = v
			}
		},
	},
	{
		Name: "grid-row-start", Category: "Grid",
		Values: []string{"<integer>"},
		Apply: func(s *computedStyle, value string) {
			if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				s.GridRowStart = v
			}
		},
	},
	{
		Name: "grid-row-end", Category: "Grid",
		Values: []string{"<integer>"},
		Apply: func(s *computedStyle, value string) {
			if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil {
				s.GridRowEnd = v
			}
		},
	},
	{
		Name: "grid-auto-flow", Category: "Grid",
		Values: []string{"row", "column", "dense", "row dense", "column dense"},
		Apply: func(s *computedStyle, value string) {
			s.GridAutoFlow = strings.TrimSpace(strings.ToLower(value))
		},
	},
	{
		Name: "grid-auto-rows", Category: "Grid",
		Values: []string{"<track-size>"},
		Apply: func(s *computedStyle, value string) {
			s.GridAutoRows = strings.TrimSpace(value)
		},
	},
	{
		Name: "align-content", Category: "Grid",
		Values: []string{"normal", "stretch", "flex-start", "flex-end", "center", "space-between", "space-around", "space-evenly"},
		Apply: func(s *computedStyle, value string) {
			s.AlignContent = strings.TrimSpace(strings.ToLower(value))
		},
	},
	{
		Name: "justify-items", Category: "Grid",
		Values: []string{"start", "end", "center", "stretch"},
		Apply: func(s *computedStyle, value string) {
			s.JustifyItems = strings.TrimSpace(strings.ToLower(value))
		},
	},

	// --- MultiColumn (Phase 2 batch 4) ----------------------------------

	{
		Name: "column-count", Category: "MultiColumn",
		Values: []string{"<integer>", "auto"},
		Apply: func(s *computedStyle, value string) {
			if v, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && v > 0 {
				s.ColumnCount = v
			}
		},
	},
	{
		Name: "column-gap", Category: "MultiColumn",
		Values: []string{"<length>", "normal"},
		Apply: func(s *computedStyle, value string) {
			v := parseBoxSide(value, s.FontSize)
			s.ColumnGap = v
			s.GridColumnGap = v
		},
	},
	{
		Name: "column-width", Category: "MultiColumn",
		Values: []string{"<length>", "auto"},
		Apply: func(s *computedStyle, value string) {
			if l := parseLength(value); l != nil {
				s.ColumnWidth = l.toPoints(0, s.FontSize)
			}
		},
	},
	{
		Name: "column-rule-width", Category: "MultiColumn",
		Values: []string{"<length>", "thin", "medium", "thick"},
		Apply: func(s *computedStyle, value string) {
			if l := parseLength(value); l != nil {
				s.ColumnRuleWidth = l.toPoints(0, s.FontSize)
			}
		},
	},
	{
		Name: "column-rule-style", Category: "MultiColumn",
		Values: []string{"solid", "dashed", "dotted", "double", "none"},
		Apply: func(s *computedStyle, value string) {
			s.ColumnRuleStyle = strings.TrimSpace(strings.ToLower(value))
		},
	},
	{
		Name: "column-rule-color", Category: "MultiColumn",
		Values: []string{"<color>"},
		Apply: func(s *computedStyle, value string) {
			if c, ok := parseColor(value); ok {
				s.ColumnRuleColor = c
			}
		},
	},
	{
		Name: "column-span", Category: "MultiColumn",
		Values: []string{"none", "all"},
		Apply: func(s *computedStyle, value string) {
			switch strings.TrimSpace(strings.ToLower(value)) {
			case "all":
				s.ColumnSpan = "all"
			case "none":
				s.ColumnSpan = "none"
			}
		},
	},

	// --- Margins / paddings (Phase 3 batch A) ---------------------------

	{
		Name: "margin", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>", "auto", "<1-4 of these>"},
		Notes:  "auto keyword sets MarginTopAuto/LeftAuto/RightAuto per CSS shorthand position rules.",
		Apply: func(s *computedStyle, value string) {
			s.MarginTopLength, s.MarginRightLength, s.MarginBottomLength, s.MarginLeftLength =
				parseMarginShorthandLengths(value, s.FontSize)
			// Use splitTopLevelFields (paren-aware) so calc()/min()/max()/
			// clamp() values stay as single tokens — otherwise the
			// auto-flag positions could shift with respect to the parsed
			// margin values (#237).
			parts := splitTopLevelFields(value)
			autoFlags := make([]bool, len(parts))
			for i, p := range parts {
				autoFlags[i] = strings.ToLower(p) == "auto"
			}
			switch len(parts) {
			case 1:
				if autoFlags[0] {
					s.MarginTopAuto = true
					s.MarginLeftAuto = true
					s.MarginRightAuto = true
				}
			case 2:
				if autoFlags[0] {
					s.MarginTopAuto = true
				}
				if autoFlags[1] {
					s.MarginLeftAuto = true
					s.MarginRightAuto = true
				}
			case 3:
				if autoFlags[0] {
					s.MarginTopAuto = true
				}
				if autoFlags[1] {
					s.MarginLeftAuto = true
					s.MarginRightAuto = true
				}
			case 4:
				if autoFlags[0] {
					s.MarginTopAuto = true
				}
				if autoFlags[1] {
					s.MarginRightAuto = true
				}
				if autoFlags[3] {
					s.MarginLeftAuto = true
				}
			}
		},
	},
	{
		Name: "margin-top", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>", "auto"},
		Apply: func(s *computedStyle, value string) {
			if strings.TrimSpace(strings.ToLower(value)) == "auto" {
				s.MarginTopAuto = true
			} else {
				s.MarginTopLength = parseBoxSideLength(value, s.FontSize)
			}
		},
	},
	{
		Name: "margin-right", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>", "auto"},
		Apply: func(s *computedStyle, value string) {
			if strings.TrimSpace(strings.ToLower(value)) == "auto" {
				s.MarginRightAuto = true
			} else {
				s.MarginRightLength = parseBoxSideLength(value, s.FontSize)
			}
		},
	},
	{
		Name: "margin-left", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>", "auto"},
		Apply: func(s *computedStyle, value string) {
			if strings.TrimSpace(strings.ToLower(value)) == "auto" {
				s.MarginLeftAuto = true
			} else {
				s.MarginLeftLength = parseBoxSideLength(value, s.FontSize)
			}
		},
	},
	{
		Name: "padding", Category: "BoxModel",
		Values: []string{"<length>", "<percentage>", "<1-4 of these>"},
		Apply: func(s *computedStyle, value string) {
			s.PaddingTopLength, s.PaddingRightLength, s.PaddingBottomLength, s.PaddingLeftLength =
				parseMarginShorthandLengths(value, s.FontSize)
		},
	},

	// --- Backgrounds + Borders shorthands (Phase 3 batch B) -------------

	{
		Name: "background", Category: "Backgrounds",
		Values: []string{"<color>", "<gradient>", "url(...)"},
		Notes:  "Background shorthand: dispatches to BackgroundImage for gradient/url, BackgroundColor otherwise.",
		Apply: func(s *computedStyle, value string) {
			lower := strings.ToLower(strings.TrimSpace(value))
			if strings.HasPrefix(lower, "linear-gradient(") ||
				strings.HasPrefix(lower, "repeating-linear-gradient(") ||
				strings.HasPrefix(lower, "radial-gradient(") ||
				strings.HasPrefix(lower, "repeating-radial-gradient(") ||
				strings.HasPrefix(lower, "url(") {
				s.BackgroundImage = strings.TrimSpace(value)
			} else if clr, ok := parseColor(value); ok {
				s.BackgroundColor = &clr
			}
		},
	},
	{
		Name: "border", Category: "Borders",
		Values: []string{"<line-width> <line-style> <color>"},
		Notes:  "Sets all 12 fields (4 sides × {width, style, color}) at once.",
		Apply: func(s *computedStyle, value string) {
			w, st, clr := parseBorderFull(value, s.FontSize)
			s.BorderTopWidth = w
			s.BorderRightWidth = w
			s.BorderBottomWidth = w
			s.BorderLeftWidth = w
			s.BorderTopStyle = st
			s.BorderRightStyle = st
			s.BorderBottomStyle = st
			s.BorderLeftStyle = st
			s.BorderTopColor = clr
			s.BorderRightColor = clr
			s.BorderBottomColor = clr
			s.BorderLeftColor = clr
		},
	},
	{
		Name: "border-width", Category: "Borders",
		Values: []string{"<line-width>"},
		Apply: func(s *computedStyle, value string) {
			w := parseBoxSide(value, s.FontSize)
			s.BorderTopWidth = w
			s.BorderRightWidth = w
			s.BorderBottomWidth = w
			s.BorderLeftWidth = w
		},
	},
	{
		Name: "border-color", Category: "Borders",
		Values: []string{"<color>"},
		Apply: func(s *computedStyle, value string) {
			if c, ok := parseColor(value); ok {
				s.BorderTopColor = c
				s.BorderRightColor = c
				s.BorderBottomColor = c
				s.BorderLeftColor = c
			}
		},
	},
	{
		Name: "border-style", Category: "Borders",
		Values: []string{"solid", "dashed", "dotted", "double", "none", "hidden", "groove", "ridge", "inset", "outset"},
		Notes:  "groove/ridge/inset/outset are rendered as a single solid stroke per side with the spec's per-side dark/light color modulation, rather than the strict two-half-width split bevel.",
		Apply: func(s *computedStyle, value string) {
			s.BorderTopStyle = value
			s.BorderRightStyle = value
			s.BorderBottomStyle = value
			s.BorderLeftStyle = value
		},
	},
	{
		Name: "border-top", Category: "Borders",
		Values: []string{"<line-width> <line-style> <color>"},
		Apply: func(s *computedStyle, value string) {
			w, st, clr := parseBorderFull(value, s.FontSize)
			s.BorderTopWidth = w
			s.BorderTopStyle = st
			s.BorderTopColor = clr
		},
	},
	{
		Name: "border-right", Category: "Borders",
		Values: []string{"<line-width> <line-style> <color>"},
		Apply: func(s *computedStyle, value string) {
			w, st, clr := parseBorderFull(value, s.FontSize)
			s.BorderRightWidth = w
			s.BorderRightStyle = st
			s.BorderRightColor = clr
		},
	},
	{
		Name: "border-bottom", Category: "Borders",
		Values: []string{"<line-width> <line-style> <color>"},
		Apply: func(s *computedStyle, value string) {
			w, st, clr := parseBorderFull(value, s.FontSize)
			s.BorderBottomWidth = w
			s.BorderBottomStyle = st
			s.BorderBottomColor = clr
		},
	},
	{
		Name: "border-left", Category: "Borders",
		Values: []string{"<line-width> <line-style> <color>"},
		Apply: func(s *computedStyle, value string) {
			w, st, clr := parseBorderFull(value, s.FontSize)
			s.BorderLeftWidth = w
			s.BorderLeftStyle = st
			s.BorderLeftColor = clr
		},
	},
	{
		Name: "border-radius", Category: "Borders",
		Values: []string{"<length>", "<percentage>", "<1-4 of these>"},
		Notes:  "splitTopLevelFields preserves calc()/min()/max()/clamp() as single tokens.",
		Apply: func(s *computedStyle, value string) {
			// The `border-radius: <h> / <v>` slash syntax (independent
			// horizontal/vertical radii) is deferred follow-up work; only the
			// leading (horizontal) set is consumed here, and a single value per
			// corner drives both axes. parseRadiusComponent preserves
			// percentages so they resolve against the box at layout time.
			if slash := indexByteAtTopLevel(value, '/'); slash >= 0 {
				value = strings.TrimSpace(value[:slash])
			}
			parts := splitTopLevelFields(value)
			var (
				tlA, tlP float64
				trA, trP float64
				brA, brP float64
				blA, blP float64
			)
			switch len(parts) {
			case 1:
				tlA, tlP = parseRadiusComponent(parts[0], s.FontSize)
				trA, trP = tlA, tlP
				brA, brP = tlA, tlP
				blA, blP = tlA, tlP
			case 2:
				// parts[0] -> TL & BR (diagonal), parts[1] -> TR & BL.
				tlA, tlP = parseRadiusComponent(parts[0], s.FontSize)
				trA, trP = parseRadiusComponent(parts[1], s.FontSize)
				brA, brP = tlA, tlP
				blA, blP = trA, trP
			case 3:
				// parts[0] -> TL, parts[1] -> TR & BL, parts[2] -> BR.
				tlA, tlP = parseRadiusComponent(parts[0], s.FontSize)
				trA, trP = parseRadiusComponent(parts[1], s.FontSize)
				brA, brP = parseRadiusComponent(parts[2], s.FontSize)
				blA, blP = trA, trP
			case 4:
				tlA, tlP = parseRadiusComponent(parts[0], s.FontSize)
				trA, trP = parseRadiusComponent(parts[1], s.FontSize)
				brA, brP = parseRadiusComponent(parts[2], s.FontSize)
				blA, blP = parseRadiusComponent(parts[3], s.FontSize)
			default:
				return
			}
			s.BorderRadiusTL, s.BorderRadiusTLPct = tlA, tlP
			s.BorderRadiusTR, s.BorderRadiusTRPct = trA, trP
			s.BorderRadiusBR, s.BorderRadiusBRPct = brA, brP
			s.BorderRadiusBL, s.BorderRadiusBLPct = blA, blP
			s.BorderRadius = tlA
		},
	},
	{
		Name: "border-spacing", Category: "Tables",
		Values: []string{"<length>", "<length> <length>"},
		Apply: func(s *computedStyle, value string) {
			parts := splitTopLevelFields(strings.TrimSpace(value))
			if len(parts) == 1 {
				if l := parseLength(parts[0]); l != nil {
					v := l.toPoints(0, s.FontSize)
					s.BorderSpacingH = v
					s.BorderSpacingV = v
				}
			} else if len(parts) >= 2 {
				if lh := parseLength(parts[0]); lh != nil {
					s.BorderSpacingH = lh.toPoints(0, s.FontSize)
				}
				if lv := parseLength(parts[1]); lv != nil {
					s.BorderSpacingV = lv.toPoints(0, s.FontSize)
				}
			}
		},
	},

	// --- Flex / gap shorthands (Phase 3 batch B) ------------------------

	{
		Name: "flex", Category: "Flexbox",
		Values: []string{"<flex-grow> <flex-shrink>? <flex-basis>?", "none", "auto"},
		Apply: func(s *computedStyle, value string) {
			parseFlexShorthand(value, s)
		},
	},
	{
		Name: "flex-flow", Category: "Flexbox",
		Values: []string{"<flex-direction> <flex-wrap>"},
		Apply: func(s *computedStyle, value string) {
			parseFlexFlowShorthand(value, s)
		},
	},
	{
		Name: "gap", Aliases: []string{"grid-gap"}, Category: "Grid",
		Values: []string{"<row-gap>", "<row-gap> <column-gap>"},
		Notes:  "Sets RowGap and GridColumnGap; in flex contexts, also Gap.",
		Apply: func(s *computedStyle, value string) {
			parts := splitTopLevelFields(strings.TrimSpace(value))
			if len(parts) == 1 {
				v := parseBoxSide(parts[0], s.FontSize)
				s.Gap = v
				s.RowGap = v
				s.GridColumnGap = v
			} else if len(parts) >= 2 {
				s.RowGap = parseBoxSide(parts[0], s.FontSize)
				s.GridColumnGap = parseBoxSide(parts[1], s.FontSize)
				s.Gap = s.RowGap // flex compat: use row-gap value
			}
		},
	},

	// --- vertical-align / baseline-shift (Phase 3 batch B) --------------

	{
		Name: "vertical-align", Category: "Typography",
		Values: []string{"top", "middle", "bottom", "super", "sub", "baseline", "text-top", "text-bottom", "<length>", "<percentage>"},
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			if v == "top" || v == "middle" || v == "bottom" || v == "super" || v == "sub" || v == "baseline" || v == "text-top" || v == "text-bottom" {
				s.VerticalAlign = v
				s.BaselineShiftSet = false
			} else if l := parseCSSLengthWithUnit(v); l != nil {
				lineH := s.FontSize * s.LineHeight
				s.BaselineShiftValue = l.toPoints(lineH, s.FontSize)
				s.BaselineShiftSet = true
			}
		},
	},
	{
		Name: "baseline-shift", Category: "Typography",
		Values: []string{"super", "sub", "baseline", "<length>", "<percentage>"},
		Notes:  "CSS Inline Layout Module Level 3 §4.3 — percentages resolve against line-height.",
		Apply: func(s *computedStyle, value string) {
			v := strings.TrimSpace(strings.ToLower(value))
			switch v {
			case "super":
				s.VerticalAlign = "super"
				s.BaselineShiftSet = false
			case "sub":
				s.VerticalAlign = "sub"
				s.BaselineShiftSet = false
			case "baseline":
				s.VerticalAlign = ""
				s.BaselineShiftSet = false
			default:
				if l := parseCSSLengthWithUnit(v); l != nil {
					lineH := s.FontSize * s.LineHeight
					s.BaselineShiftValue = l.toPoints(lineH, s.FontSize)
					s.BaselineShiftSet = true
				}
			}
		},
	},

	// --- font / outline / columns shorthands (Phase 3 batch B) ----------

	{
		Name: "font", Category: "Typography",
		Values: []string{"<font-style>? <font-weight>? <font-size>[/<line-height>]? <font-family>"},
		Apply: func(s *computedStyle, value string) {
			fs, fw, sz, lh, ff := parseFontShorthand(value, s.FontSize)
			if fs != "" {
				s.FontStyle = fs
			}
			if fw != 0 {
				s.FontWeight = fw
			}
			if sz > 0 {
				s.FontSize = sz
			}
			if lh > 0 {
				s.LineHeight = lh
			}
			if ff != "" {
				s.FontFamily = ff
			}
		},
	},
	{
		Name: "outline", Category: "Effects",
		Values: []string{"<line-width> <line-style> <color>"},
		Apply: func(s *computedStyle, value string) {
			w, st, clr := parseBorderFull(value, s.FontSize)
			s.OutlineWidth = w
			s.OutlineStyle = st
			s.OutlineColor = clr
		},
	},
	{
		Name: "columns", Category: "MultiColumn",
		Values: []string{"<column-count>", "<column-width>", "<column-count> <column-width>"},
		Apply: func(s *computedStyle, value string) {
			parts := splitTopLevelFields(strings.TrimSpace(value))
			for _, p := range parts {
				if v, err := strconv.Atoi(p); err == nil && v > 0 {
					s.ColumnCount = v
				} else if l := parseLength(p); l != nil {
					s.ColumnWidth = l.toPoints(0, s.FontSize)
				}
			}
		},
	},
	{
		Name: "column-rule", Category: "MultiColumn",
		Values: []string{"<line-width> <line-style> <color>"},
		Apply: func(s *computedStyle, value string) {
			s.ColumnRuleWidth, s.ColumnRuleStyle, s.ColumnRuleColor = parseColumnRule(value, s.FontSize)
		},
	},
}

// cssPropByName indexes cssProperties by canonical name and aliases.
// Built once at package init. Lookups are O(1).
var cssPropByName = func() map[string]*cssProperty {
	m := make(map[string]*cssProperty, len(cssProperties)*2)
	for i := range cssProperties {
		p := &cssProperties[i]
		m[p.Name] = p
		for _, alias := range p.Aliases {
			m[alias] = p
		}
	}
	return m
}()
